package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm/clause"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/internal/service"
)

// newRBACServer 启动一个 httptest.Server，挂载完整拦截器链 + 各 service handler。
// 与 cmd/server/main.go 同构，但不带 CORS（测试直连）。
func newRBACServer(t *testing.T, r *repo.Repo, mgr *jwt.Manager) *httptest.Server {
	t.Helper()
	authSvc := service.NewAuthService(r,
		repo.NewLoginSecurity(r.DB(), 5, 900),
		repo.NewRefreshTokenRepo(r.DB()),
		mgr, time.Hour, false, 7*24*time.Hour, false)
	userSvc, err := service.NewGeneratedUserService(r)
	if err != nil {
		t.Fatalf("new generated User service: %v", err)
	}

	permRegistry := interceptor.NewPermRegistry()
	if err := interceptor.RegisterBuiltinPermissions(permRegistry); err != nil {
		t.Fatalf("register builtin permissions: %v", err)
	}
	if err := resourcegen.RegisterUserPermissions(permRegistry); err != nil {
		t.Fatalf("register User permissions: %v", err)
	}
	if err := permRegistry.Freeze(); err != nil {
		t.Fatalf("freeze permission registry: %v", err)
	}

	operLogger := func(ctx context.Context, e interceptor.OperLogEntry) {
		_ = r.AddOperLog(ctx, &model.SysOperLog{
			Title:    e.Title,
			Method:   e.Method,
			OperName: e.OperName,
			OperID:   e.OperID,
			OperIP:   e.OperIP,
			Status:   e.Status,
			ErrorMsg: e.ErrorMsg,
			CostTime: e.CostMs,
		})
	}

	server := connect.NewServer(
		interceptor.NewAuthInterceptor(permRegistry, mgr, r.TokenVersion, r.PwdResetRequired),
		interceptor.NewPermissionInterceptor(permRegistry, r.PermsOf),
		interceptor.NewOperLogInterceptor(operLogger, false),
	)

	mux := http.NewServeMux()
	adminv1connect.RegisterAuthServiceHandler(server, authSvc)
	adminv1connect.RegisterUserServiceHandler(server, userSvc)
	connecthttp.Mount(mux, server, connecthttp.WithReadMaxBytes(0))

	// h2c 支持：Connect 客户端默认走 HTTP/2，无 TLS 需要 h2c 升级。
	srv := httptest.NewUnstartedServer(h2c.NewHandler(mux, &http2.Server{}))
	srv.EnableHTTP2 = true
	srv.Start()
	return srv
}

// bearerInterceptor 给 outgoing Connect 请求注入 Authorization: Bearer <token>。
func bearerInterceptor(token string) connect.ClientInterceptor {
	return func(next connect.ClientFunc) connect.ClientFunc {
		return func(ctx context.Context, spec connect.Spec) (connect.ClientStream, error) {
			ctx, info := connect.NewClientContext(ctx)
			info.RequestHeader().Set("Authorization", "Bearer "+token)
			return next(ctx, spec)
		}
	}
}

// loginAndGetToken 通过 HTTP 调用 /Login 并返回 token 字符串（e2e 不直接调 service）。
func loginAndGetToken(t *testing.T, baseURL string, client *http.Client, username, password string) string {
	t.Helper()
	authClient := adminv1connect.NewAuthServiceClient(connect.NewClient(connecthttp.NewTransport(client, baseURL, connecthttp.WithReadMaxBytes(0))))
	resp, err := authClient.Login(context.Background(),
		&adminv1.LoginRequest{Username: username, Password: password})
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return resp.GetToken()
}

// assignLimitedPerms 创建一个有限权限角色：只含 system:user:list + system:user:query，
// 绑定到 userID。覆盖走 repo 层的 SQL，避免依赖更多 service。
func assignLimitedPerms(t *testing.T, r *repo.Repo, userID int64) error {
	t.Helper()
	ctx := context.Background()
	// 1. 创建角色（data_scope=5「仅本人」，避免污染其他测试数据）。
	role := model.SysRole{RoleName: "受限测试角色", RoleKey: "limited-test", RoleSort: 99, DataScope: 5, Status: 1, Remark: "e2e only"}
	if err := r.DB().WithContext(ctx).Create(&role).Error; err != nil {
		return err
	}
	if err := r.DB().WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.SysUserRole{UserID: userID, RoleID: role.ID}).Error; err != nil {
		return err
	}
	return r.DB().WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&[]model.SysRoleMenu{{RoleID: role.ID, MenuID: 100}, {RoleID: role.ID, MenuID: 1001}}).Error
}
