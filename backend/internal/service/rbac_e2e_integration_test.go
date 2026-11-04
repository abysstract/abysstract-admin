//go:build integration

package service_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

// TestRBACEndToEnd 启动接入真实拦截器链（Auth → Permission → OperLog）的 in-process
// Connect 服务，覆盖「登录 → 携带 token 访问受限接口 → 按权限放行/拒绝」的完整路径。
// 满足 Requirement 3.7。
func TestRBACEndToEnd(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	r := repo.New(gdb)
	mgr := jwt.NewManager("e2e-rbac-secret", time.Hour)

	// 准备两类用户：
	//   - admin（seed）：拥有通配权限 "*:*:*"，能访问所有接口。
	//   - limited：拥有部分菜单权限（system:user:list）但缺少 system:user:add。
	limited, err := createUserForTest(r, "limited-user", "pw123456")
	if err != nil {
		t.Fatalf("create limited user: %v", err)
	}
	if err := assignLimitedPerms(t, r, limited.ID); err != nil {
		t.Fatalf("assign limited perms: %v", err)
	}

	// 启动带完整拦截器链的 in-process server。
	server := newRBACServer(t, r, mgr)
	defer server.Close()

	// 场景 1：未携带 token 访问受限接口 → Unauthenticated。
	t.Run("no_token_is_unauthenticated", func(t *testing.T) {
		client := adminv1connect.NewUserServiceClient(connect.NewClient(connecthttp.NewTransport(server.Client(), server.URL)))
		_, err := client.ListUsers(context.Background(),
			&adminv1.ListUsersRequest{})
		if err == nil {
			t.Fatal("expected Unauthenticated, got nil")
		}
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("code = %v, want %v", connect.CodeOf(err), connect.CodeUnauthenticated)
		}
	})

	// 场景 2：admin 登录取 token → 访问受限接口成功（通配 "*:*:*"）。
	t.Run("admin_token_grants_all_access", func(t *testing.T) {
		authClient := adminv1connect.NewAuthServiceClient(connect.NewClient(connecthttp.NewTransport(server.Client(), server.URL)))
		loginResp, err := authClient.Login(context.Background(),
			&adminv1.LoginRequest{
				Username: "admin",
				Password: "admin123",
			})
		if err != nil {
			t.Fatalf("admin login: %v", err)
		}
		token := loginResp.GetToken()
		if token == "" {
			t.Fatal("token empty")
		}

		userClient := adminv1connect.NewUserServiceClient(connect.NewClient(connecthttp.NewTransport(server.Client(), server.URL), bearerInterceptor(token)))
		resp, err := userClient.ListUsers(context.Background(),
			&adminv1.ListUsersRequest{})
		if err != nil {
			t.Fatalf("admin ListUsers should succeed: %v", err)
		}
		// admin data_scope=1（全部）应能看见至少 2 个用户（admin + limited-user）。
		if len(resp.GetItems()) < 2 {
			t.Errorf("admin should see all users, got %d items", len(resp.GetItems()))
		}
	})

	// 场景 3：limited 用户拥有 system:user:list 权限 → 可访问 ListUsers。
	t.Run("limited_user_can_list", func(t *testing.T) {
		token := loginAndGetToken(t, server.URL, server.Client(), "limited-user", "pw123456")

		userClient := adminv1connect.NewUserServiceClient(connect.NewClient(connecthttp.NewTransport(server.Client(), server.URL), bearerInterceptor(token)))
		_, err := userClient.ListUsers(context.Background(),
			&adminv1.ListUsersRequest{})
		if err != nil {
			t.Fatalf("limited user with system:user:list should be allowed: %v", err)
		}
	})

	// 场景 4：limited 用户缺 system:user:add 权限 → CreateUser 被拒绝。
	t.Run("limited_user_cannot_create", func(t *testing.T) {
		token := loginAndGetToken(t, server.URL, server.Client(), "limited-user", "pw123456")

		userClient := adminv1connect.NewUserServiceClient(connect.NewClient(connecthttp.NewTransport(server.Client(), server.URL), bearerInterceptor(token)))
		_, err := userClient.CreateUser(context.Background(),
			&adminv1.CreateUserRequest{
				Username: "forbidden-test",
				Password: "whatever1",
			})
		if err == nil {
			t.Fatal("limited user without system:user:add must be rejected")
		}
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("code = %v, want %v", connect.CodeOf(err), connect.CodePermissionDenied)
		}
	})

	// 场景 5：token 签名错误（伪造 token）→ Unauthenticated。
	t.Run("forged_token_is_rejected", func(t *testing.T) {
		forged, _, err := jwt.NewManager("wrong-secret", time.Hour).Generate(limited.ID, "limited-user", 0)
		if err != nil {
			t.Fatalf("forge token: %v", err)
		}

		userClient := adminv1connect.NewUserServiceClient(connect.NewClient(connecthttp.NewTransport(server.Client(), server.URL), bearerInterceptor(forged)))
		_, err = userClient.ListUsers(context.Background(),
			&adminv1.ListUsersRequest{})
		if err == nil {
			t.Fatal("forged token must be rejected")
		}
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("code = %v, want %v", connect.CodeOf(err), connect.CodeUnauthenticated)
		}
	})
}
