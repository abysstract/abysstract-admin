//go:build integration

package adminapp

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/abysstract/abysstract-admin/backend/identity"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/module"
	"github.com/abysstract/abysstract-admin/backend/permission"
)

func TestConnectAndHTTPAccessPolicies(t *testing.T) {
	skipSystemTables(t)
	db := sqliteDB(t)
	seedAuthSchema(t, db)
	var businessCalls atomic.Int32
	var httpCalls atomic.Int32
	svc, procedure := probeService(t, "company.contract.v1", "ProbeService", func() { businessCalls.Add(1) })
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), DB: db, AllowedHTTPMethods: []string{"PUT"}, Modules: []module.Definition{{
		Name:     "system",
		Services: []module.ConnectService{svc},
		HTTP: []module.HTTPRoute{
			{Name: "public", Method: "GET", Path: "/hook/public", Access: module.AccessPublic},
			{Name: "auth", Method: "POST", Path: "/hook/auth", Access: module.AccessAuthenticated, MaxBytes: 64},
			{Name: "perm", Method: "PUT", Path: "/hook/perm", Access: module.AccessPermission, Permission: "biz:hook:write"},
		},
		Permissions: func(reg permission.Registrar) error {
			return reg.RegisterGenerated(permission.Entry{Procedure: procedure, Permission: "biz:hook:write"})
		},
		Create: func(context.Context, module.Runtime) (module.Instance, error) {
			return module.Instance{
				Services: []module.ConnectService{svc},
				HTTP: []module.HTTPHandler{
					{Name: "public", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						httpCalls.Add(1)
						w.WriteHeader(http.StatusNoContent)
					})},
					{Name: "auth", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						httpCalls.Add(1)
						actor, ok := identity.FromContext(r.Context())
						if !ok || actor.UserID != 1 {
							http.Error(w, "missing actor", 500)
							return
						}
						w.WriteHeader(http.StatusNoContent)
					})},
					{Name: "perm", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						httpCalls.Add(1)
						w.WriteHeader(http.StatusNoContent)
					})},
				},
			}, nil
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app)
	base := "http://" + app.ListenerAddr()

	// Anonymous public HTTP.
	resp, err := http.Get(base + "/hook/public")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("public = %d", resp.StatusCode)
	}

	// Anonymous protected HTTP -> 401, zero handler.
	httpCalls.Store(0)
	resp, err = http.Post(base+"/hook/auth", "application/json", strings.NewReader(`{"x":1}`))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("auth anonymous = %d %s", resp.StatusCode, body)
	}
	if httpCalls.Load() != 0 {
		t.Fatal("handler called for 401")
	}
	if bytes.Contains(body, []byte("Bearer")) || bytes.Contains(body, []byte("token")) {
		t.Fatalf("token leaked: %s", body)
	}

	client := connect.NewClient(connecthttp.NewTransport(http.DefaultClient, base))
	err = client.CallUnary(context.Background(), connect.Spec{Procedure: procedure}, &wrapperspb.StringValue{Value: "ping"}, &wrapperspb.StringValue{})
	if err == nil {
		t.Fatal("expected unauthenticated connect")
	}
	if businessCalls.Load() != 0 {
		t.Fatal("business handler called anonymously")
	}

	mgr := jwt.NewManager("test-secret-not-default", time.Hour)
	token, _, err := mgr.Generate(1, "alice", 0)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, base+"/hook/auth", strings.NewReader(`{"password":"secret"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("auth token = %d", resp.StatusCode)
	}

	huge := bytes.Repeat([]byte("a"), 128)
	req, _ = http.NewRequest(http.MethodPost, base+"/hook/auth", bytes.NewReader(huge))
	req.Header.Set("Authorization", "Bearer "+token)
	httpCalls.Store(0)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("max bytes = %d", resp.StatusCode)
	}
	if httpCalls.Load() != 0 {
		t.Fatal("handler called for 413")
	}

	req, _ = http.NewRequest(http.MethodPut, base+"/hook/perm", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("missing perm = %d", resp.StatusCode)
	}

	cancel()
	<-done
}

func TestHTTPPwdResetAndConnectOversize(t *testing.T) {
	skipSystemTables(t)
	db := sqliteDB(t)
	seedAuthSchema(t, db)
	var calls atomic.Int32
	svc, procedure := probeService(t, "company.contract.v1", "SizeService", func() { calls.Add(1) })
	cfg := testConfig(t)
	cfg.UploadMaxBytes = 64
	app, err := New(Options{Config: cfg, Logger: silentLogger(), DB: db, Modules: []module.Definition{{
		Name:        "system",
		Services:    []module.ConnectService{svc},
		HTTP:        []module.HTTPRoute{{Name: "auth", Method: "POST", Path: "/hook/auth", Access: module.AccessAuthenticated}},
		Permissions: generatedPermissions(procedure, permission.Authenticated),
		Create: func(context.Context, module.Runtime) (module.Instance, error) {
			return module.Instance{
				Services: []module.ConnectService{svc},
				HTTP: []module.HTTPHandler{{Name: "auth", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					w.WriteHeader(http.StatusNoContent)
				})}},
			}, nil
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app)
	base := "http://" + app.ListenerAddr()
	mgr := jwt.NewManager(cfg.JWTSecret, time.Hour)
	resetToken, _, err := mgr.Generate(2, "reset", 0)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, base+"/hook/auth", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+resetToken)
	calls.Store(0)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("pwd reset = %d", resp.StatusCode)
	}
	if calls.Load() != 0 {
		t.Fatal("handler called during pwd reset")
	}

	client := connect.NewClient(connecthttp.NewTransport(http.DefaultClient, base))
	err = client.CallUnary(context.Background(), connect.Spec{Procedure: procedure}, &wrapperspb.StringValue{Value: strings.Repeat("x", 4096)}, &wrapperspb.StringValue{})
	if err == nil {
		t.Fatal("expected oversized connect to fail")
	}
	if calls.Load() != 0 {
		t.Fatal("handler called for oversized connect")
	}
	cancel()
	<-done
}

func TestDataScopeDeniesAnonymous(t *testing.T) {
	skipSystemTables(t)
	db := sqliteDB(t)
	if err := db.Exec(`CREATE TABLE items (id INTEGER, dept_id INTEGER, create_by INTEGER)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO items (id, dept_id, create_by) VALUES (1, 9, 9)`).Error; err != nil {
		t.Fatal(err)
	}
	seen := make(chan int64, 1)
	app, err := New(Options{Config: testConfig(t), Logger: silentLogger(), DB: db, Modules: []module.Definition{{
		Name: "system",
		Create: func(_ context.Context, rt module.Runtime) (module.Instance, error) {
			var count int64
			scope := rt.DataScope.DataScope(context.Background(), "dept_id", "create_by")
			if err := scope.Apply(db.Table("items")).Count(&count).Error; err != nil {
				return module.Instance{}, err
			}
			seen <- count
			return module.Instance{}, nil
		},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app)
	if count := <-seen; count != 0 {
		t.Fatalf("anonymous data scope visible rows = %d", count)
	}
	cancel()
	<-done
}
