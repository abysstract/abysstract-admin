package adminapp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/module"
)

// Options creates a public application. DB, when non-nil, is borrowed.
type Options struct {
	Config             adminconfig.Config
	Modules            []module.Definition
	DB                 *gorm.DB
	Logger             *slog.Logger
	AllowedHTTPMethods []string
}

type appState int

const (
	stateCreated appState = iota
	stateStarting
	stateRunning
	stateStopping
	stateStopped
)

// App is a validated, runnable application.
type App struct {
	cfg            adminconfig.Config
	modules        []module.Definition
	routes         []preparedRoute
	registry       *interceptor.PermRegistry
	allowedMethods []string
	borrowedDB     *gorm.DB
	logger         *slog.Logger

	mu          sync.Mutex
	state       appState
	stopOnce    sync.Once
	cleanupOnce sync.Once
	stopCh      chan struct{}
	stoppedCh   chan struct{}
	closeErr    error
	serveErrCh  chan error

	db            *gorm.DB
	closeDB       bool
	startCancel   context.CancelFunc
	startFinished bool
	runtimeCancel context.CancelFunc
	instances     []createdInstance
	listener      net.Listener
	server        *http.Server
}

// ListenerAddr returns the bound address after Run has started listening.
func (a *App) ListenerAddr() string {
	if a == nil {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.listener == nil {
		return ""
	}
	return a.listener.Addr().String()
}

// New copies and validates configuration and module declarations. It does not
// open a database, call Create, or listen.
func New(options Options) (*App, error) {
	cfg := options.Config.Clone()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.ValidateDatabase(); err != nil {
		return nil, err
	}
	defs := cloneDefinitions(options.Modules)
	ordered, err := sortAndValidateModules(defs)
	if err != nil {
		return nil, err
	}
	procedures, _, err := collectProcedures(ordered)
	if err != nil {
		return nil, err
	}
	allowed, err := normalizeAllowedHTTPMethods(options.AllowedHTTPMethods)
	if err != nil {
		return nil, err
	}
	routes, err := collectAndCheckRoutes(ordered, procedures, allowed, requestMaxBytes(cfg))
	if err != nil {
		return nil, err
	}
	registry, err := collectPermissions(ordered, procedures)
	if err != nil {
		return nil, err
	}
	return &App{
		cfg:            cfg,
		modules:        ordered,
		routes:         routes,
		registry:       registry,
		allowedMethods: allowed,
		borrowedDB:     options.DB,
		logger:         defaultLogger(options.Logger),
		stopCh:         make(chan struct{}),
		stoppedCh:      make(chan struct{}),
		serveErrCh:     make(chan error, 1),
	}, nil
}

// Start initializes the modules and returns their HTTP handler. The caller owns
// the HTTP server and calls Shutdown after requests have drained.
func (a *App) Start(ctx context.Context) (http.Handler, error) {
	return a.initialize(ctx, false)
}

// Run starts the default HTTP server and closes the application when it stops.
func (a *App) Run(ctx context.Context) error {
	if _, err := a.initialize(ctx, true); err != nil {
		return err
	}
	select {
	case err := <-a.serveErrCh:
		return errors.Join(err, a.completeStop(context.WithoutCancel(ctx), true))
	case <-ctx.Done():
		return a.completeStop(context.WithoutCancel(ctx), false)
	case <-a.stopCh:
		<-a.stoppedCh
		return a.closeErr
	}
}

func (a *App) initialize(ctx context.Context, serveHTTP bool) (http.Handler, error) {
	if ctx == nil {
		return nil, ErrNilContext
	}
	startCtx, startCancel := context.WithCancel(ctx)
	a.mu.Lock()
	if a.state != stateCreated {
		a.mu.Unlock()
		startCancel()
		return nil, ErrAlreadyStarted
	}
	a.state = stateStarting
	a.startCancel = startCancel
	a.mu.Unlock()
	defer startCancel()

	handler, err := a.start(startCtx)
	if err == nil && serveHTTP {
		err = a.startHTTP(handler)
	}
	a.mu.Lock()
	a.startFinished = true
	stopRequested := a.state == stateStopping
	if err == nil && a.state == stateStarting {
		a.state = stateRunning
		stopRequested = false
	}
	a.mu.Unlock()
	if err != nil {
		closeErr := a.completeStop(context.WithoutCancel(ctx), true)
		return nil, errors.Join(err, closeErr)
	}
	if stopRequested {
		return nil, errors.Join(context.Canceled, a.completeStop(context.WithoutCancel(ctx), false))
	}

	return handler, nil
}

// Shutdown requests a single cleanup. Callers may wait with their own deadline
// while the application timeout still governs the actual cleanup.
func (a *App) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	a.mu.Lock()
	switch a.state {
	case stateCreated:
		a.mu.Unlock()
		return ErrNotStarted
	case stateStopped:
		err := a.closeErr
		a.mu.Unlock()
		return err
	default:
		if a.state == stateStarting {
			a.state = stateStopping
		}
		cancel := a.startCancel
		startFinished := a.startFinished
		a.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if startFinished {
			go a.beginShutdown(false)
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.stoppedCh:
		return a.closeErr
	}
}

func (a *App) start(ctx context.Context) (http.Handler, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	gdb, closeDB, err := acquireDB(ctx, a.cfg, a.borrowedDB)
	if err != nil {
		return nil, err
	}
	a.db = gdb
	a.closeDB = closeDB
	if err := requireSystemTables(gdb); err != nil {
		return nil, err
	}
	store := repo.New(gdb)
	jwtMgr := jwt.NewManager(a.cfg.JWTSecret, a.cfg.JWTExpiresIn)
	runtimeCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	a.runtimeCancel = cancel
	runtime := module.Runtime{
		DB:        gdb,
		Logger:    a.logger,
		Config:    a.cfg.Clone(),
		Context:   runtimeCtx,
		DataScope: dataScopeAdapter{repo: store},
	}
	created := make([]createdInstance, 0, len(a.modules))
	for _, def := range a.modules {
		inst, err := def.Create(ctx, runtime)
		if err != nil {
			a.instances = created
			return nil, fmt.Errorf("adminapp: module %q create: %w", def.Name, err)
		}
		entry := createdInstance{name: def.Name, services: inst.Services, http: inst.HTTP, close: inst.Close}
		created = append(created, entry)
		if err := matchInstance(def, inst); err != nil {
			a.instances = created
			return nil, err
		}
	}
	a.instances = created
	sec := httpSecurity{
		manager:     jwtMgr,
		verLoader:   store.TokenVersion,
		resetLoader: store.PwdResetRequired,
		permLoader:  store.PermsOf,
		trustProxy:  a.cfg.TrustProxy,
		record: func(entry interceptor.OperLogEntry) {
			_ = store.AddOperLog(context.Background(), operLogModel(entry))
		},
	}
	server := connect.NewServer(
		interceptor.NewAuthInterceptor(a.registry, jwtMgr, store.TokenVersion, store.PwdResetRequired),
		interceptor.NewPermissionInterceptor(a.registry, store.PermsOf),
		interceptor.NewOperLogInterceptor(func(ctx context.Context, entry interceptor.OperLogEntry) {
			_ = store.AddOperLog(ctx, operLogModel(entry))
		}, a.cfg.TrustProxy),
	)
	maxBytes := int(requestMaxBytes(a.cfg))
	connectOpts := connecthttp.WithReadMaxBytes(maxBytes)
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	metricsReg := prometheus.NewRegistry()
	handler, err := buildHandler(a.cfg, a.logger, a.allowedMethods, a.routes, a.instances, server, connectOpts, sec, sqlDB, metricsReg, metricsReg)
	if err != nil {
		return nil, err
	}
	return handler, nil
}

func (a *App) startHTTP(handler http.Handler) error {
	listener, err := listen("tcp", a.cfg.Addr)
	if err != nil {
		return fmt.Errorf("adminapp: listen: %w", err)
	}
	server := &http.Server{Addr: a.cfg.Addr, Handler: handler}
	ApplyServerTimeouts(server)
	protocols := &http.Protocols{}
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	server.Protocols = protocols
	a.mu.Lock()
	a.listener = listener
	a.server = server
	a.mu.Unlock()
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.serveErrCh <- err
			return
		}
		a.serveErrCh <- nil
	}()
	return nil
}

func (a *App) beginShutdown(force bool) {
	a.signalStop()
	_, _ = force, a.completeStop(context.Background(), force)
}

func (a *App) signalStop() {
	a.stopOnce.Do(func() {
		close(a.stopCh)
	})
}

func (a *App) completeStop(parent context.Context, forceHTTP bool) error {
	a.signalStop()
	a.cleanupOnce.Do(func() {
		a.mu.Lock()
		a.state = stateStopping
		timeout := a.cfg.ShutdownTimeout
		a.mu.Unlock()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), timeout)
		defer cancel()
		err := a.cleanup(ctx, forceHTTP)
		a.mu.Lock()
		a.closeErr = err
		a.state = stateStopped
		close(a.stoppedCh)
		a.mu.Unlock()
	})
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.closeErr
}

func (a *App) cleanup(ctx context.Context, forceHTTP bool) error {
	var errs []error
	if a.server != nil {
		if forceHTTP {
			if err := a.server.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errs = append(errs, err)
			}
		} else if err := a.server.Shutdown(ctx); err != nil {
			errs = append(errs, err)
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				errs = append(errs, ErrShutdownTimeout)
				_ = a.server.Close()
			}
		}
	}
	if a.runtimeCancel != nil {
		a.runtimeCancel()
	}
	for i := len(a.instances) - 1; i >= 0; i-- {
		entry := a.instances[i]
		if entry.close == nil {
			continue
		}
		if err := entry.close(ctx); err != nil {
			errs = append(errs, fmt.Errorf("module %q close: %w", entry.name, err))
		}
	}
	if a.closeDB {
		if err := closeGorm(a.db); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func operLogModel(entry interceptor.OperLogEntry) *model.SysOperLog {
	return &model.SysOperLog{
		Title:     entry.Title,
		Method:    entry.Method,
		OperName:  entry.OperName,
		OperID:    entry.OperID,
		OperIP:    entry.OperIP,
		OperParam: entry.OperParam,
		Status:    entry.Status,
		ErrorMsg:  entry.ErrorMsg,
		CostTime:  entry.CostMs,
	}
}
