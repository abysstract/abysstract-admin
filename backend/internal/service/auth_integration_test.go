//go:build integration

package service_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/service"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

// TestLoginSuccessAndFailure 覆盖 AuthService.Login 的三种分支：
//   - 凭证正确：签发可校验的 token（Requirement 3.3）
//   - 密码错误：不签发并返回 Unauthenticated（Requirement 3.3）
//   - 不存在的用户：返回 Unauthenticated
//
// 依赖真实 PostgreSQL（migrate 已建表 + seed 内置 admin/admin123）。
func TestLoginSuccessAndFailure(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	r := repo.New(gdb)
	mgr := jwt.NewManager("test-secret-login", time.Hour)
	svc := service.NewAuthService(r,
		repo.NewLoginSecurity(gdb, 5, 900),
		repo.NewRefreshTokenRepo(gdb),
		mgr, time.Hour, false, 7*24*time.Hour, false)

	t.Run("correct_credentials_issues_valid_token", func(t *testing.T) {
		resp, err := svc.Login(context.Background(),
			&adminv1.LoginRequest{
				Username: "admin",
				Password: "admin123",
			},
		)
		if err != nil {
			t.Fatalf("login should succeed for admin/admin123, got err: %v", err)
		}
		token := resp.GetToken()
		if token == "" {
			t.Fatal("token must not be empty on success")
		}
		if resp.GetExpiresIn() <= 0 {
			t.Errorf("expires_in = %d, want > 0", resp.GetExpiresIn())
		}

		// 签发的 token 必须能被同一 Manager 校验。
		claims, err := mgr.Parse(token)
		if err != nil {
			t.Fatalf("token must be parseable by same manager: %v", err)
		}
		if claims.Username != "admin" {
			t.Errorf("claims.Username = %q, want %q", claims.Username, "admin")
		}
		if claims.UserID != 1 {
			t.Errorf("claims.UserID = %d, want 1 (seeded admin)", claims.UserID)
		}
	})

	t.Run("wrong_password_does_not_issue_token", func(t *testing.T) {
		_, err := svc.Login(context.Background(),
			&adminv1.LoginRequest{
				Username: "admin",
				Password: "wrong-password",
			},
		)
		if err == nil {
			t.Fatal("login with wrong password must fail, got nil err")
		}
		// 错误码必须是 Unauthenticated（service.go:55）。
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("error code = %v, want %v", connect.CodeOf(err), connect.CodeUnauthenticated)
		}
	})

	t.Run("nonexistent_user_returns_unauthenticated", func(t *testing.T) {
		_, err := svc.Login(context.Background(),
			&adminv1.LoginRequest{
				Username: "ghost",
				Password: "whatever",
			},
		)
		if err == nil {
			t.Fatal("login with nonexistent user must fail")
		}
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("error code = %v, want %v", connect.CodeOf(err), connect.CodeUnauthenticated)
		}
	})
}

// TestLoginDisabledAccount 覆盖账号停用分支：用户存在且密码正确但 status=2。
// service.go:59 应返回 PermissionDenied。
func TestLoginDisabledAccount(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}

	r := repo.New(gdb)
	// 创建一个独立测试用户（status=1 启用），用 repo.CreateUser 完成 bcrypt 哈希。
	u, err := createUserForTest(r, "login-test", "pass123")
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	// 停用账号。
	if err := r.ChangeStatus(context.Background(), u.ID, 2); err != nil {
		t.Fatalf("disable user: %v", err)
	}

	mgr := jwt.NewManager("test-secret-disabled", time.Hour)
	svc := service.NewAuthService(r,
		repo.NewLoginSecurity(gdb, 5, 900),
		repo.NewRefreshTokenRepo(gdb),
		mgr, time.Hour, false, 7*24*time.Hour, false)

	_, err = svc.Login(context.Background(),
		&adminv1.LoginRequest{
			Username: "login-test",
			Password: "pass123",
		},
	)
	if err == nil {
		t.Fatal("login with disabled account must fail")
	}
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("error code = %v, want %v", connect.CodeOf(err), connect.CodePermissionDenied)
	}
}
