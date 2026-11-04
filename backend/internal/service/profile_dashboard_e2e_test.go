package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/service"
)

// newProfileDashboardServer 启动一个仅含 Auth + Profile + Dashboard 的 in-process server，
// 复用 rbac_e2e_test 的 bearerInterceptor / loginAndGetToken。
func newProfileDashboardServer(t *testing.T, r *repo.Repo, mgr *jwt.Manager) *httptest.Server {
	t.Helper()
	loginSec := repo.NewLoginSecurity(r.DB(), 5, 900)
	refresh := repo.NewRefreshTokenRepo(r.DB())
	authSvc := service.NewAuthService(r, loginSec, refresh, mgr, time.Hour, false, 7*24*time.Hour, false)
	profSvc := service.NewProfileService(r)
	dashSvc := service.NewDashboardService(r)

	permRegistry := interceptor.NewPermRegistry()
	if err := interceptor.RegisterBuiltinPermissions(permRegistry); err != nil {
		t.Fatalf("register builtin permissions: %v", err)
	}
	if err := permRegistry.Freeze(); err != nil {
		t.Fatalf("freeze permission registry: %v", err)
	}

	operLogger := func(context.Context, interceptor.OperLogEntry) {}

	server := connect.NewServer(
		interceptor.NewAuthInterceptor(permRegistry, mgr, r.TokenVersion, r.PwdResetRequired),
		interceptor.NewPermissionInterceptor(permRegistry, r.PermsOf),
		interceptor.NewOperLogInterceptor(operLogger, false),
	)

	mux := http.NewServeMux()
	adminv1connect.RegisterAuthServiceHandler(server, authSvc)
	adminv1connect.RegisterProfileServiceHandler(server, profSvc)
	adminv1connect.RegisterDashboardServiceHandler(server, dashSvc)
	connecthttp.Mount(mux, server, connecthttp.WithReadMaxBytes(0))

	srv := httptest.NewUnstartedServer(h2c.NewHandler(mux, &http2.Server{}))
	srv.EnableHTTP2 = true
	srv.Start()
	return srv
}

// 防止 unused 警告（model 在 e2e 测试套件其他文件用到）。
var _ = model.SysUser{}
