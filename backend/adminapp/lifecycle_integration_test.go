//go:build integration

package adminapp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/module"
)

func TestRunCreateFailureClosesPreviousOnceAndKeepsBorrowedDB(t *testing.T) {
	skipSystemTables(t)
	db := sqliteDB(t)
	var closes atomic.Int32
	closeCh := make(chan struct{}, 1)
	want := errors.New("create boom")
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), DB: db, Modules: []module.Definition{
		{
			Name: "system",
			Create: func(context.Context, module.Runtime) (module.Instance, error) {
				return module.Instance{Close: func(context.Context) error {
					closes.Add(1)
					closeCh <- struct{}{}
					return nil
				}}, nil
			},
		},
		{
			Name:     "billing",
			Requires: []string{"system"},
			Create: func(context.Context, module.Runtime) (module.Instance, error) {
				return module.Instance{}, want
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(t.Context())
	if !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
	select {
	case <-closeCh:
	case <-time.After(2 * time.Second):
		t.Fatal("system Close not called")
	}
	if closes.Load() != 1 {
		t.Fatalf("closes = %d", closes.Load())
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("borrowed db closed: %v", err)
	}
}

func TestRunListenFailureClosesInstances(t *testing.T) {
	skipSystemTables(t)
	restore := OverrideListen(func(string, string) (net.Listener, error) {
		return nil, errors.New("listen boom")
	})
	defer restore()
	var closes atomic.Int32
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), DB: sqliteDB(t), Modules: []module.Definition{{
		Name: "system",
		Create: func(context.Context, module.Runtime) (module.Instance, error) {
			return module.Instance{Close: func(context.Context) error {
				closes.Add(1)
				return nil
			}}, nil
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(t.Context())
	if err == nil || !strings.Contains(err.Error(), "listen boom") {
		t.Fatalf("err = %v", err)
	}
	if closes.Load() != 1 {
		t.Fatalf("closes = %d", closes.Load())
	}
}

func TestDuplicateRunAndContextCancel(t *testing.T) {
	skipSystemTables(t)
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), DB: sqliteDB(t), Modules: []module.Definition{systemModule()}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app)
	second := app.Run(context.Background())
	if !errors.Is(second, ErrAlreadyStarted) {
		t.Fatalf("second run = %v", second)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run = %v", err)
	}
}

func TestShutdownOnceAndCallerTimeout(t *testing.T) {
	skipSystemTables(t)
	started := make(chan struct{})
	release := make(chan struct{})
	var closes atomic.Int32
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), DB: sqliteDB(t), Modules: []module.Definition{{
		Name: "system",
		Create: func(context.Context, module.Runtime) (module.Instance, error) {
			return module.Instance{Close: func(ctx context.Context) error {
				closes.Add(1)
				close(started)
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}}, nil
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	runErr := make(chan error, 1)
	go func() { runErr <- app.Run(context.Background()) }()
	waitListening(t, app)
	short, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := app.Shutdown(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("caller shutdown = %v", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("close not started")
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = app.Shutdown(context.Background())
	}()
	go func() {
		defer wg.Done()
		_ = app.Shutdown(context.Background())
	}()
	close(release)
	wg.Wait()
	if err := <-runErr; err != nil {
		t.Fatalf("run = %v", err)
	}
	if closes.Load() != 1 {
		t.Fatalf("closes = %d", closes.Load())
	}
}

func TestModuleCloseErrorsContinue(t *testing.T) {
	skipSystemTables(t)
	want := errors.New("close boom")
	order := make([]string, 0, 2)
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), DB: sqliteDB(t), Modules: []module.Definition{
		{
			Name: "system",
			Create: func(context.Context, module.Runtime) (module.Instance, error) {
				return module.Instance{Close: func(context.Context) error {
					order = append(order, "system")
					return nil
				}}, nil
			},
		},
		{
			Name:     "billing",
			Requires: []string{"system"},
			Create: func(context.Context, module.Runtime) (module.Instance, error) {
				return module.Instance{Close: func(context.Context) error {
					order = append(order, "billing")
					return want
				}}, nil
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app)
	cancel()
	err = <-done
	if !errors.Is(err, want) {
		t.Fatalf("err = %v", err)
	}
	if fmt.Sprint(order) != "[billing system]" {
		t.Fatalf("order = %v", order)
	}
}

func TestTwoAppsListenIndependently(t *testing.T) {
	skipSystemTables(t)
	start := func() *App {
		app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), DB: sqliteDB(t), Modules: []module.Definition{systemModule()}})
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- app.Run(context.Background()) }()
		t.Cleanup(func() {
			_ = app.Shutdown(context.Background())
			<-done
		})
		waitListening(t, app)
		return app
	}
	a := start()
	b := start()
	resp, err := httpGet("http://" + a.ListenerAddr() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("a health = %d", resp.StatusCode)
	}
	resp, err = httpGet("http://" + b.ListenerAddr() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("b health = %d", resp.StatusCode)
	}
}

func TestInFlightRequestKeepsDB(t *testing.T) {
	skipSystemTables(t)
	db := sqliteDB(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), DB: db, Modules: []module.Definition{{
		Name: "system",
		HTTP: []module.HTTPRoute{{Name: "wait", Method: "GET", Path: "/hook/wait", Access: module.AccessPublic}},
		Create: func(_ context.Context, rt module.Runtime) (module.Instance, error) {
			return module.Instance{HTTP: []module.HTTPHandler{{Name: "wait", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if rt.Context.Err() != nil {
					http.Error(w, "runtime cancelled", 500)
					return
				}
				close(entered)
				<-release
				w.WriteHeader(204)
			})}}}, nil
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app)
	reqDone := make(chan int, 1)
	go func() {
		resp, err := httpGet("http://" + app.ListenerAddr() + "/hook/wait")
		if err != nil {
			reqDone <- -1
			return
		}
		resp.Body.Close()
		reqDone <- resp.StatusCode
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("handler not entered")
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("db during request: %v", err)
	}
	close(release)
	if code := <-reqDone; code != 204 {
		t.Fatalf("status = %d", code)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run = %v", err)
	}
}

func TestDrainTimeoutForceCloses(t *testing.T) {
	skipSystemTables(t)
	cfg := testConfig(t)
	cfg.ShutdownTimeout = 1 * time.Second
	entered := make(chan struct{})
	app, err := New(Options{Config: cfg, Logger: silentLogger(), DB: sqliteDB(t), Modules: []module.Definition{{
		Name: "system",
		HTTP: []module.HTTPRoute{{Name: "wait", Method: "GET", Path: "/hook/wait", Access: module.AccessPublic}},
		Create: func(context.Context, module.Runtime) (module.Instance, error) {
			return module.Instance{HTTP: []module.HTTPHandler{{Name: "wait", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				time.Sleep(3 * time.Second)
				w.WriteHeader(204)
			})}}}, nil
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app)
	go func() { _, _ = httpGet("http://" + app.ListenerAddr() + "/hook/wait") }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("handler not entered")
	}
	cancel()
	err = <-done
	if !errors.Is(err, ErrShutdownTimeout) && (err == nil || !strings.Contains(err.Error(), "deadline")) {
		t.Fatalf("run = %v", err)
	}
}

func TestRunUncleanInjectedRouteReturnsErrorAndCleans(t *testing.T) {
	skipSystemTables(t)
	var closes atomic.Int32
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), DB: sqliteDB(t), Modules: []module.Definition{{
		Name: "system",
		Create: func(context.Context, module.Runtime) (module.Instance, error) {
			return module.Instance{Close: func(context.Context) error {
				closes.Add(1)
				return nil
			}}, nil
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	app.routes = append(app.routes, preparedRoute{
		Module:  "system",
		Pattern: "GET /policy//target",
		Route:   module.HTTPRoute{Name: "unclean", Method: "GET", Path: "/policy//target", Access: module.AccessPublic},
	})
	err = app.Run(context.Background())
	if err == nil {
		t.Fatal("expected unclean route to fail Run")
	}
	if !strings.Contains(err.Error(), "canonical") {
		t.Fatalf("err = %v", err)
	}
	if closes.Load() != 1 {
		t.Fatalf("closes = %d", closes.Load())
	}
}

func TestShutdownDuringCreateCancelsAndCleans(t *testing.T) {
	skipSystemTables(t)
	entered := make(chan struct{})
	var systemClosed atomic.Int32
	createCtxCanceled := make(chan bool, 1)
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), DB: sqliteDB(t), Modules: []module.Definition{
		{
			Name: "system",
			Create: func(context.Context, module.Runtime) (module.Instance, error) {
				return module.Instance{Close: func(context.Context) error {
					systemClosed.Add(1)
					return nil
				}}, nil
			},
		},
		{
			Name:     "billing",
			Requires: []string{"system"},
			Create: func(ctx context.Context, _ module.Runtime) (module.Instance, error) {
				close(entered)
				<-ctx.Done()
				createCtxCanceled <- errors.Is(ctx.Err(), context.Canceled)
				return module.Instance{}, ctx.Err()
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	runErr := make(chan error, 1)
	go func() { runErr <- app.Run(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Create did not start")
	}
	results := make(chan error, 2)
	go func() { results <- app.Shutdown(context.Background()) }()
	go func() { results <- app.Shutdown(context.Background()) }()
	errRun := <-runErr
	if !errors.Is(errRun, context.Canceled) {
		t.Fatalf("run = %v", errRun)
	}
	errA := <-results
	errB := <-results
	if errA != errB {
		t.Fatalf("shutdown results differ: %v %v", errA, errB)
	}
	select {
	case canceled := <-createCtxCanceled:
		if !canceled {
			t.Fatal("Create context was not canceled")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Create did not observe cancel")
	}
	if systemClosed.Load() != 1 {
		t.Fatalf("system closes = %d", systemClosed.Load())
	}
}
