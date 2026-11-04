//go:build integration

package system

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/abysstract/abysstract-admin/backend/adminapp"
	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/observ"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/module"
	"github.com/abysstract/abysstract-admin/backend/permission"
)

func TestSystemModuleCreateFailsOnEmptyDatabase(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	def, err := NewModule()
	if err != nil {
		t.Fatal(err)
	}
	_, err = def.Create(context.Background(), module.Runtime{DB: db, Config: testConfig(t), Logger: silentLogger()})
	if err != ErrMissingSystemTables {
		t.Fatalf("err = %v", err)
	}
}

func TestSystemModuleCreateSucceedsOnMigratedDatabase(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	def, err := NewModule()
	if err != nil {
		t.Fatal(err)
	}
	if err := testutil.Migrate(db); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t)
	cfg.UploadDir = t.TempDir() + "/uploads"
	inst, err := def.Create(context.Background(), module.Runtime{DB: db, Config: cfg, Logger: silentLogger()})
	if err != nil {
		t.Fatal(err)
	}
	if len(inst.Services) != 13 {
		t.Fatalf("services = %d", len(inst.Services))
	}
	for i, svc := range inst.Services {
		if svc.Descriptor == nil || svc.Register == nil {
			t.Fatalf("service %d incomplete", i)
		}
	}
	if inst.Close == nil {
		t.Fatal("Close is nil")
	}
	if err := inst.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSystemAppLoginAndBusinessProbe(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	systemDef, err := NewModule()
	if err != nil {
		t.Fatal(err)
	}
	if err := testutil.Migrate(db); err != nil {
		t.Fatal(err)
	}
	var probeCalls atomic.Int32
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	probe, procedure := probeService(t, "company.contract.v1", "ProbeService", func() { probeCalls.Add(1) })
	app, err := adminapp.New(adminapp.Options{
		Config: testConfig(t),
		Logger: logger,
		DB:     db,
		Modules: []module.Definition{
			systemDef,
			{
				Name:     "billing",
				Requires: []string{"system"},
				Services: []module.ConnectService{probe},
				Permissions: func(reg permission.Registrar) error {
					return reg.RegisterGenerated(permission.Entry{Procedure: procedure, Permission: "biz:contract:list"})
				},
				Create: func(context.Context, module.Runtime) (module.Instance, error) {
					return module.Instance{Services: []module.ConnectService{probe}}, nil
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app, done)
	base := "http://" + app.ListenerAddr()

	assertProbe := func(path, wantSubstring string, status int) {
		t.Helper()
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != status {
			t.Fatalf("%s status = %d body=%s", path, resp.StatusCode, body)
		}
		if wantSubstring != "" && !strings.Contains(string(body), wantSubstring) {
			t.Fatalf("%s body = %s, want %s", path, body, wantSubstring)
		}
	}
	assertProbe("/healthz", `"alive"`, http.StatusOK)
	assertJSONRequestLog(t, logBuf.String(), "GET", "/healthz", http.StatusOK)
	assertProbe("/readyz", `"ready"`, http.StatusOK)
	assertProbe("/version", "version", http.StatusOK)
	resp, err := http.Get(base + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics = %d", resp.StatusCode)
	}
	req, err := http.NewRequest(http.MethodGet, base+"/healthz", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "http://localhost:5173")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("cors origin = %q", got)
	}
	if resp.Header.Get(observ.HeaderRequestID) == "" {
		t.Fatal("missing request id")
	}
	static, err := http.Get(base + "/static/")
	if err != nil {
		t.Fatal(err)
	}
	static.Body.Close()
	if static.StatusCode != http.StatusOK && static.StatusCode != http.StatusMovedPermanently && static.StatusCode != http.StatusNotFound && static.StatusCode != http.StatusForbidden {
		t.Fatalf("static = %d", static.StatusCode)
	}

	auth := adminv1connect.NewAuthServiceClient(connect.NewClient(connecthttp.NewTransport(http.DefaultClient, base)))
	login, err := auth.Login(context.Background(), &adminv1.LoginRequest{
		Username: "admin",
		Password: "admin123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if login.GetToken() == "" {
		t.Fatal("empty token")
	}

	client := connect.NewClient(connecthttp.NewTransport(http.DefaultClient, base))
	err = client.CallUnary(context.Background(), connect.Spec{Procedure: procedure}, &wrapperspb.StringValue{Value: "ping"}, &wrapperspb.StringValue{})
	if err == nil {
		t.Fatal("anonymous probe succeeded")
	}
	if probeCalls.Load() != 0 {
		t.Fatal("anonymous probe invoked handler")
	}

	authed := &wrapperspb.StringValue{Value: "ping"}
	authedCtx, authedInfo := connect.NewClientContext(context.Background())
	authedInfo.RequestHeader().Set("Authorization", "Bearer "+login.GetToken())
	if err := client.CallUnary(authedCtx, connect.Spec{Procedure: procedure}, authed, &wrapperspb.StringValue{}); err != nil {
		t.Fatalf("admin probe: %v", err)
	}
	if probeCalls.Load() == 0 {
		t.Fatal("admin probe did not invoke handler")
	}

	if err := db.Exec(`
		INSERT INTO sys_user (dept_id, username, nickname, email, password, status, remark)
		VALUES (1, 'alice', 'Alice', 'alice@example.com', 'x', 1, 'tester')`).Error; err != nil {
		t.Fatal(err)
	}
	var aliceID int64
	if err := db.Raw("SELECT id FROM sys_user WHERE username = ?", "alice").Scan(&aliceID).Error; err != nil {
		t.Fatal(err)
	}
	var aliceVer int64
	if err := db.Raw("SELECT token_version FROM sys_user WHERE id = ?", aliceID).Scan(&aliceVer).Error; err != nil {
		t.Fatal(err)
	}
	aliceToken, _, err := jwt.NewManager("test-secret-not-default", time.Hour).Generate(aliceID, "alice", aliceVer)
	if err != nil {
		t.Fatal(err)
	}
	before := probeCalls.Load()
	denied := &wrapperspb.StringValue{Value: "ping"}
	deniedCtx, deniedInfo := connect.NewClientContext(context.Background())
	deniedInfo.RequestHeader().Set("Authorization", "Bearer "+aliceToken)
	if err := client.CallUnary(deniedCtx, connect.Spec{Procedure: procedure}, denied, &wrapperspb.StringValue{}); err == nil {
		t.Fatal("alice probe succeeded")
	}
	if probeCalls.Load() != before {
		t.Fatal("alice probe invoked handler")
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("borrowed db closed while running: %v", err)
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run = %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("borrowed db closed after shutdown: %v", err)
	}
}

func TestShutdownDuringSystemCreateCancels(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := testutil.Migrate(db); err != nil {
		t.Fatal(err)
	}
	systemDef, err := NewModule()
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	createCanceled := make(chan bool, 1)
	app, err := adminapp.New(adminapp.Options{
		Config: testConfig(t),
		Logger: silentLogger(),
		DB:     db,
		Modules: []module.Definition{
			systemDef,
			{
				Name:     "billing",
				Requires: []string{"system"},
				Create: func(ctx context.Context, _ module.Runtime) (module.Instance, error) {
					close(entered)
					<-ctx.Done()
					createCanceled <- errors.Is(ctx.Err(), context.Canceled)
					return module.Instance{}, ctx.Err()
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	runErr := make(chan error, 1)
	go func() { runErr <- app.Run(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(30 * time.Second):
		t.Fatal("billing Create did not start")
	}
	shutdownErr := make(chan error, 1)
	go func() { shutdownErr <- app.Shutdown(context.Background()) }()
	errRun := <-runErr
	if !errors.Is(errRun, context.Canceled) {
		t.Fatalf("run = %v", errRun)
	}
	if err := <-shutdownErr; err != nil && !errors.Is(err, context.Canceled) && err != errRun {
		t.Fatalf("shutdown = %v run = %v", err, errRun)
	}
	select {
	case canceled := <-createCanceled:
		if !canceled {
			t.Fatal("Create context was not canceled")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Create did not observe cancel")
	}
}
