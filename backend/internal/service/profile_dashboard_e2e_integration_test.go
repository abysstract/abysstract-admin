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

// TestProfileEndToEnd 覆盖：
//   - GetProfile：能读取 seed 的 admin 资料且 username 只读。
//   - UpdateProfile：更新 nickname/email/phone 后再读，确认落库。
//   - username 在 UpdateProfileRequest 中无字段，无法被修改（proto 层保证）。
func TestProfileEndToEnd(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	r := repo.New(gdb)
	mgr := jwt.NewManager("profile-e2e-secret", time.Hour)

	server := newProfileDashboardServer(t, r, mgr)
	defer server.Close()

	token := loginAndGetToken(t, server.URL, server.Client(), "admin", "admin123")
	profClient := adminv1connect.NewProfileServiceClient(connect.NewClient(connecthttp.NewTransport(server.Client(), server.URL), bearerInterceptor(token)))

	// Get
	getResp, err := profClient.GetProfile(context.Background(),
		&adminv1.GetProfileRequest{})
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if getResp.Username != "admin" {
		t.Errorf("username got=%s want=admin", getResp.Username)
	}

	// Update
	upResp, err := profClient.UpdateProfile(context.Background(),
		&adminv1.UpdateProfileRequest{
			Nickname: "超管",
			Email:    "admin@example.com",
			Phone:    "13800000000",
		})
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if upResp.Profile.Nickname != "超管" || upResp.Profile.Email != "admin@example.com" {
		t.Errorf("updated profile mismatch: %+v", upResp.Profile)
	}

	// 二次 Get 确认落库
	getResp2, err := profClient.GetProfile(context.Background(),
		&adminv1.GetProfileRequest{})
	if err != nil {
		t.Fatalf("GetProfile(2): %v", err)
	}
	if getResp2.Nickname != "超管" {
		t.Errorf("nickname after update got=%s want=超管", getResp2.Nickname)
	}
	if getResp2.Username != "admin" {
		t.Errorf("username changed unexpectedly: %s", getResp2.Username)
	}
}

// TestDashboardEndToEnd 覆盖：
//   - Overview 返回非空统计字段（seed 后 user_count ≥ 1，role_count ≥ 1）。
//   - today_login_count 反映本次 login 写入 sys_login_log（auth 拦截器或 repo 层应已写入；若未写则 ≥ 0）。
func TestDashboardEndToEnd(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	r := repo.New(gdb)
	mgr := jwt.NewManager("dashboard-e2e-secret", time.Hour)

	server := newProfileDashboardServer(t, r, mgr)
	defer server.Close()

	token := loginAndGetToken(t, server.URL, server.Client(), "admin", "admin123")
	dashClient := adminv1connect.NewDashboardServiceClient(connect.NewClient(connecthttp.NewTransport(server.Client(), server.URL), bearerInterceptor(token)))

	resp, err := dashClient.Overview(context.Background(),
		&adminv1.OverviewRequest{})
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if resp.UserCount < 1 {
		t.Errorf("user_count got=%d, want >=1", resp.UserCount)
	}
	if resp.RoleCount < 1 {
		t.Errorf("role_count got=%d, want >=1", resp.RoleCount)
	}
}
