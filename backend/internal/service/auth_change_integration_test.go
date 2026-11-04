//go:build integration

package service_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/service"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

// newAuthSvcForTest 构造一个全功能的 AuthService，refresh enabled=false。
// 返回 (svc, repo, jwtMgr) 供测试驱动。
func newAuthSvcForTest(t *testing.T, refreshEnabled bool) (*service.AuthService, *repo.Repo, *jwt.Manager) {
	t.Helper()
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := repo.New(gdb)
	mgr := jwt.NewManager("test-secret", time.Hour)
	svc := service.NewAuthService(r,
		repo.NewLoginSecurity(gdb, 5, 900),
		repo.NewRefreshTokenRepo(gdb),
		mgr, time.Hour,
		refreshEnabled, 7*24*time.Hour,
		false,
	)
	return svc, r, mgr
}

// TestChangePasswordSuccess 改密成功（需求 6.1/6.4）：
//   - 旧密码校验通过
//   - token_version 递增
//   - pwd_reset_required 清零
//   - 新密码可用、旧密码不可用
func TestChangePasswordSuccess(t *testing.T) {
	svc, r, _ := newAuthSvcForTest(t, false)
	const oldPass = "admin123"
	const newPass = "NewP@ssw0rd"

	_, err := svc.ChangePassword(context.Background(),
		&adminv1.ChangePasswordRequest{
			OldPassword: oldPass,
			NewPassword: newPass,
		})
	// 没带 claims 应拒绝。
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("without claims err=%v, want Unauthenticated", err)
	}

	// admin 初始 token_version=0，pwd_reset_required=0。
	_, err = svc.ChangePassword(ctxWithClaims(1, "admin", 0),
		&adminv1.ChangePasswordRequest{
			OldPassword: oldPass,
			NewPassword: newPass,
		})
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	// 验证 token_version 递增到 1。
	ver, err := r.TokenVersion(context.Background(), 1)
	if err != nil {
		t.Fatalf("TokenVersion: %v", err)
	}
	if ver != 1 {
		t.Errorf("token_version = %d, want 1", ver)
	}

	// 验证旧密码失效、新密码可用。
	if _, err := svc.Login(context.Background(),
		&adminv1.LoginRequest{Username: "admin", Password: oldPass}); err == nil {
		t.Error("old password should not work after change")
	}
	if _, err := svc.Login(context.Background(),
		&adminv1.LoginRequest{Username: "admin", Password: newPass}); err != nil {
		t.Errorf("new password should work, got %v", err)
	}
}

// TestChangePasswordFailures 改密失败的几种分支（需求 6.2/6.3）：
//   - 旧密码错误 → Unauthenticated
//   - 新密码不满足复杂度 → InvalidArgument
//   - 新旧相同 → InvalidArgument
func TestChangePasswordFailures(t *testing.T) {
	svc, _, _ := newAuthSvcForTest(t, false)
	ctx := ctxWithClaims(1, "admin", 0)

	cases := []struct {
		name    string
		oldP    string
		newP    string
		wantCmp connect.Code
	}{
		{"wrong_old", "wrong-old-pass", "NewP@ssw0rd", connect.CodeUnauthenticated},
		{"weak_new", "admin123", "123", connect.CodeInvalidArgument},
		{"single_class_new", "admin123", "aaaaaaaa", connect.CodeInvalidArgument},
		{"same_as_old", "admin123", "admin123", connect.CodeInvalidArgument},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.ChangePassword(ctx,
				&adminv1.ChangePasswordRequest{
					OldPassword: c.oldP,
					NewPassword: c.newP,
				})
			if connect.CodeOf(err) != c.wantCmp {
				t.Errorf("err=%v want code %v", err, c.wantCmp)
			}
		})
	}
}

// TestChangePasswordClearsResetFlag 改密清除 pwd_reset_required（需求 6.5）。
func TestChangePasswordClearsResetFlag(t *testing.T) {
	svc, r, _ := newAuthSvcForTest(t, false)
	// 手工把 admin 标记为需改密。
	if err := r.SetPwdResetRequired(context.Background(), 1, true); err != nil {
		t.Fatalf("SetPwdResetRequired: %v", err)
	}
	required, err := r.PwdResetRequired(context.Background(), 1)
	if err != nil || !required {
		t.Fatalf("setup: required=%v err=%v", required, err)
	}

	_, err = svc.ChangePassword(ctxWithClaims(1, "admin", 0),
		&adminv1.ChangePasswordRequest{
			OldPassword: "admin123",
			NewPassword: "NewP@ssw0rd",
		})
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	required2, err := r.PwdResetRequired(context.Background(), 1)
	if err != nil {
		t.Fatalf("PwdResetRequired: %v", err)
	}
	if required2 {
		t.Error("pwd_reset_required should be cleared after ChangePassword")
	}
}

// TestLoginLockoutViaAuthService 集成测试：登录失败 5 次触发锁定，期间正确密码也被拒。
func TestLoginLockoutViaAuthService(t *testing.T) {
	svc, r, _ := newAuthSvcForTest(t, false)
	// 用一个独立用户，密码已知。
	u := &model.SysUser{DeptID: new(int64(1)), Username: "lock-int", Nickname: "x", Status: 1}
	if err := r.CreateUser(context.Background(), u, "GoodPass1", nil); err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 5 次失败密码。
	for i := 0; i < 5; i++ {
		_, _ = svc.Login(context.Background(),
			&adminv1.LoginRequest{Username: "lock-int", Password: "wrong-pass"})
	}

	// 第 6 次：即使密码正确也应被锁定。
	_, err := svc.Login(context.Background(),
		&adminv1.LoginRequest{Username: "lock-int", Password: "GoodPass1"})
	if err == nil {
		t.Fatal("login should fail during lockout")
	}
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("err code = %v, want PermissionDenied (locked)", connect.CodeOf(err))
	}
}

// TestRefreshTokenIssueAndRotate 启用 refresh：Login 返回 refresh_token，RefreshToken 轮转。
// 旧 refresh 用第二次时被拒绝（已吊销）。
func TestRefreshTokenIssueAndRotate(t *testing.T) {
	svc, _, _ := newAuthSvcForTest(t, true)

	loginResp, err := svc.Login(context.Background(),
		&adminv1.LoginRequest{Username: "admin", Password: "admin123"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	rt := loginResp.GetRefreshToken()
	if rt == "" {
		t.Fatal("refresh_token must be issued when enabled")
	}

	// 第一次 RefreshToken：成功 + 轮转。
	resp1, err := svc.RefreshToken(context.Background(),
		&adminv1.RefreshTokenRequest{RefreshToken: rt})
	if err != nil {
		t.Fatalf("RefreshToken 1: %v", err)
	}
	if resp1.GetToken() == "" || resp1.GetRefreshToken() == "" {
		t.Error("response missing token/refresh_token")
	}
	if resp1.GetRefreshToken() == rt {
		t.Error("refresh_token should rotate")
	}

	// 旧 refresh 已被吊销 → 第二次用同一 rt 应失败。
	_, err = svc.RefreshToken(context.Background(),
		&adminv1.RefreshTokenRequest{RefreshToken: rt})
	if err == nil {
		t.Fatal("old refresh_token should be revoked after rotation")
	}
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("err code = %v, want Unauthenticated (revoked)", connect.CodeOf(err))
	}
}

// TestRefreshTokenDisabled 未启用时 RefreshToken 返回 Unimplemented。
func TestRefreshTokenDisabled(t *testing.T) {
	svc, _, _ := newAuthSvcForTest(t, false)

	_, err := svc.RefreshToken(context.Background(),
		&adminv1.RefreshTokenRequest{RefreshToken: "anything"})
	if connect.CodeOf(err) != connect.CodeUnimplemented {
		t.Errorf("err code = %v, want Unimplemented", connect.CodeOf(err))
	}
}
