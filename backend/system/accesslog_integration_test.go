//go:build integration

package system

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/abysstract/abysstract-admin/backend/accesslog"
	"github.com/abysstract/abysstract-admin/backend/adminapp"
	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/module"
)

// recordingSink is an application-provided sink passed through
// adminapp.Options.AccessLogSinks.
type recordingSink struct {
	mu      sync.Mutex
	entries []accesslog.Entry
	closed  bool
}

func (s *recordingSink) Name() string { return "recording" }

func (s *recordingSink) Write(_ context.Context, entries []accesslog.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entries...)
	return nil
}

func (s *recordingSink) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *recordingSink) snapshot() ([]accesslog.Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]accesslog.Entry(nil), s.entries...), s.closed
}

func startAccessLogApp(t *testing.T, sinks []string, extra ...accesslog.Sink) (*adminapp.App, string, func()) {
	t.Helper()
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := testutil.Migrate(db); err != nil {
		t.Fatal(err)
	}
	systemDef, err := NewModule()
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t)
	cfg.AccessLogSinks = sinks
	cfg.AccessLogFlushInterval = 20 * time.Millisecond
	app, err := adminapp.New(adminapp.Options{
		Config:         cfg,
		Logger:         silentLogger(),
		DB:             db,
		Modules:        []module.Definition{systemDef},
		AccessLogSinks: extra,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	waitListening(t, app, done)
	stop := func() {
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("run = %v", err)
		}
	}
	return app, "http://" + app.ListenerAddr(), stop
}

func bearer(token string) context.Context {
	ctx, info := connect.NewClientContext(context.Background())
	info.RequestHeader().Set("Authorization", "Bearer "+token)
	return ctx
}

func hasMenuPerm(menus []*adminv1.Menu, perm string) bool {
	for _, m := range menus {
		if m.GetPerms() == perm || hasMenuPerm(m.GetChildren(), perm) {
			return true
		}
	}
	return false
}

func TestAccessLogThroughApp(t *testing.T) {
	extra := &recordingSink{}
	app, base, stop := startAccessLogApp(t, []string{"db"}, extra)
	stopped := false
	defer func() {
		if !stopped {
			stop()
		}
	}()
	if app.AccessLog() == nil || app.AccessLog().Reader() == nil {
		t.Fatal("app access log must be readable with the db sink")
	}

	transport := connect.NewClient(connecthttp.NewTransport(http.DefaultClient, base))
	auth := adminv1connect.NewAuthServiceClient(transport)
	users := adminv1connect.NewUserServiceClient(transport)
	logs := adminv1connect.NewLogServiceClient(transport)
	since := time.Now().Add(-time.Minute)

	if _, err := auth.Login(context.Background(), &adminv1.LoginRequest{Username: "admin", Password: "wrong-password"}); err == nil {
		t.Fatal("wrong password accepted")
	}
	login, err := auth.Login(context.Background(), &adminv1.LoginRequest{Username: "admin", Password: "admin123"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	ctx := bearer(login.GetToken())
	info, err := auth.GetUserInfo(ctx, &adminv1.GetUserInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasMenuPerm(info.GetMenus(), "system:accesslog:list") {
		t.Fatal("request log menu missing for the super admin")
	}
	if _, err := users.ListUsers(ctx, &adminv1.ListUsersRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := users.ListUsers(context.Background(), &adminv1.ListUsersRequest{}); err == nil {
		t.Fatal("anonymous ListUsers succeeded")
	}
	resp, err := http.Get(base + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	list := func(req *adminv1.ListAccessLogsRequest) *adminv1.ListAccessLogsResponse {
		t.Helper()
		out, err := logs.ListAccessLogs(ctx, req)
		if err != nil {
			t.Fatalf("ListAccessLogs: %v", err)
		}
		return out
	}
	listUsers := adminv1connect.UserServiceListUsersProcedure
	var page *adminv1.ListAccessLogsResponse
	deadline := time.Now().Add(10 * time.Second)
	for {
		page = list(&adminv1.ListAccessLogsRequest{Procedure: listUsers, StartTime: timestamppb.New(since)})
		if len(page.GetItems()) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("ListUsers entries not written: %+v", page.GetItems())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if page.GetPage().GetTotal() != 2 {
		t.Fatalf("ListUsers entries = %d", page.GetPage().GetTotal())
	}
	anon, ok := page.GetItems()[0], page.GetItems()[1]
	if ok.GetPrincipalType() != "user" || ok.GetPrincipalName() != "admin" || ok.GetPrincipalId() == "" ||
		ok.GetCode() != "ok" || ok.GetStatus() != http.StatusOK || !ok.GetSuccess() || ok.GetKind() != "rpc" ||
		ok.GetService() != "abysstract.admin.v1.UserService" || ok.GetRequestId() == "" || ok.GetClientIp() == "" || ok.GetDurationUs() <= 0 {
		t.Fatalf("authorized entry = %+v", ok)
	}
	if anon.GetPrincipalType() != "" || anon.GetCode() != "unauthenticated" || anon.GetSuccess() || anon.GetStatus() != http.StatusUnauthorized {
		t.Fatalf("anonymous entry = %+v", anon)
	}

	logins := list(&adminv1.ListAccessLogsRequest{Procedure: adminv1connect.AuthServiceLoginProcedure})
	if logins.GetPage().GetTotal() != 2 {
		t.Fatalf("login entries = %+v", logins.GetItems())
	}
	if good := logins.GetItems()[0]; good.GetPrincipalName() != "admin" || !good.GetSuccess() {
		t.Fatalf("successful login entry = %+v", good)
	}
	failed := list(&adminv1.ListAccessLogsRequest{Result: "failure", Procedure: adminv1connect.AuthServiceLoginProcedure})
	if failed.GetPage().GetTotal() != 1 || failed.GetItems()[0].GetCode() != "unauthenticated" || failed.GetItems()[0].GetError() == "" {
		t.Fatalf("failed login filter = %+v", failed.GetItems())
	}
	byPrincipal := list(&adminv1.ListAccessLogsRequest{Principal: "admin", Procedure: listUsers})
	if byPrincipal.GetPage().GetTotal() != 1 {
		t.Fatalf("principal filter = %+v", byPrincipal.GetItems())
	}
	if probes := list(&adminv1.ListAccessLogsRequest{Procedure: "/healthz"}); probes.GetPage().GetTotal() != 0 {
		t.Fatalf("health probes must not be logged: %+v", probes.GetItems())
	}
	if _, err := logs.ListAccessLogs(ctx, &adminv1.ListAccessLogsRequest{Result: "maybe"}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("bad result filter = %v", err)
	}
	if _, err := logs.ListAccessLogs(context.Background(), &adminv1.ListAccessLogsRequest{}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anonymous ListAccessLogs = %v", err)
	}

	status, err := logs.GetAccessLogStatus(ctx, &adminv1.GetAccessLogStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !status.GetEnabled() || !status.GetReadable() || len(status.GetSinks()) != 2 ||
		status.GetSinks()[0].GetName() != "db" || !status.GetSinks()[0].GetReader() ||
		status.GetSinks()[1].GetName() != "recording" || status.GetEnqueued() == 0 || status.GetDropped() != 0 {
		t.Fatalf("status = %+v", status)
	}

	// The application sink receives the same entries and is flushed and
	// closed on shutdown.
	stop()
	stopped = true
	entries, closed := extra.snapshot()
	if !closed {
		t.Fatal("application sink not closed on shutdown")
	}
	var sawStatusCall bool
	for _, e := range entries {
		if e.Procedure == adminv1connect.LogServiceGetAccessLogStatusProcedure {
			sawStatusCall = true
			if e.RequestID == "" || e.Principal.Name != "admin" {
				t.Fatalf("status entry = %+v", e)
			}
		}
	}
	if !sawStatusCall {
		t.Fatal("last request was not flushed on shutdown")
	}
}

func TestAccessLogWithoutReaderHidesPage(t *testing.T) {
	_, base, stop := startAccessLogApp(t, []string{"stdout"})
	defer stop()
	transport := connect.NewClient(connecthttp.NewTransport(http.DefaultClient, base))
	auth := adminv1connect.NewAuthServiceClient(transport)
	logs := adminv1connect.NewLogServiceClient(transport)
	login, err := auth.Login(context.Background(), &adminv1.LoginRequest{Username: "admin", Password: "admin123"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := bearer(login.GetToken())
	info, err := auth.GetUserInfo(ctx, &adminv1.GetUserInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if hasMenuPerm(info.GetMenus(), "system:accesslog:list") {
		t.Fatal("request log menu must be hidden without a readable sink")
	}
	if !hasMenuPerm(info.GetMenus(), "system:operlog:list") {
		t.Fatal("other menus must stay visible")
	}
	_, err = logs.ListAccessLogs(ctx, &adminv1.ListAccessLogsRequest{})
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("ListAccessLogs = %v, want failed_precondition", err)
	}
	status, err := logs.GetAccessLogStatus(ctx, &adminv1.GetAccessLogStatusRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !status.GetEnabled() || status.GetReadable() {
		t.Fatalf("status = %+v", status)
	}
}
