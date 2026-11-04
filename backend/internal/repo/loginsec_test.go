//go:build integration

package repo

import (
	"context"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

// Property 10: 账号登录锁定（需求 6.6/6.7）
//
// 不变量：
//  1. 失败次数 < 阈值：IsLocked = false；不写入 locked_until。
//  2. 失败次数 >= 阈值（默认 5）：IsLocked = true；locked_until = now + lockoutSecs。
//  3. 锁定期间即使密码正确也拒绝（IsLocked 仍为 true）。
//  4. locked_until 过期后：IsLocked 懒清零为 false，并重置 fail_count。
//  5. 期间发生一次成功登录：Reset 清空 fail_count 与 locked_until。
func TestLoginSecurityLockout(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	ls := NewLoginSecurity(gdb, 5, 900)
	username := "lockout-test-user"
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)

	// 失败 1..4：未锁定。
	for i := 1; i <= 4; i++ {
		locked, until, err := ls.IsLocked(ctx, username, now)
		if err != nil {
			t.Fatalf("iter %d IsLocked: %v", i, err)
		}
		if locked {
			t.Fatalf("iter %d should not be locked", i)
		}
		_ = until

		lockedNow, fail, err := ls.RecordFailure(ctx, username, now)
		if err != nil {
			t.Fatalf("iter %d RecordFailure: %v", i, err)
		}
		if lockedNow {
			t.Fatalf("iter %d should not lock yet", i)
		}
		if fail != int32(i) {
			t.Errorf("iter %d fail = %d, want %d", i, fail, i)
		}
	}

	// 第 5 次失败：触发锁定，locked_until = now + 900s。
	locked, fail, err := ls.RecordFailure(ctx, username, now)
	if err != nil {
		t.Fatalf("5th RecordFailure: %v", err)
	}
	if !locked {
		t.Fatal("5th failure should trigger lockout")
	}
	if fail != 5 {
		t.Errorf("5th fail = %d, want 5", fail)
	}

	// IsLocked 应返回 true + 截止时刻 = now + 900s。
	locked, until, err := ls.IsLocked(ctx, username, now.Add(1*time.Second))
	if err != nil {
		t.Fatalf("IsLocked after lockout: %v", err)
	}
	if !locked {
		t.Fatal("should be locked immediately after 5 failures")
	}
	wantUntil := now.Add(900 * time.Second)
	if !until.Equal(wantUntil) {
		t.Errorf("until = %v, want %v", until, wantUntil)
	}

	// 锁定期间再失败：仍 locked（locked_until 不变，fail_count 继续递增）。
	locked2, _, err := ls.IsLocked(ctx, username, now.Add(10*time.Second))
	if err != nil {
		t.Fatalf("locked-period IsLocked: %v", err)
	}
	if !locked2 {
		t.Fatal("should remain locked within lockout window")
	}

	// 过期后：懒清零。
	expired := now.Add(901 * time.Second)
	locked3, _, err := ls.IsLocked(ctx, username, expired)
	if err != nil {
		t.Fatalf("expired IsLocked: %v", err)
	}
	if locked3 {
		t.Fatal("should be unlocked after lockout expiry")
	}

	// 懒清零后，下一次失败从 fail_count=1 开始（不是从 5 继续）。
	locked4, fail2, err := ls.RecordFailure(ctx, username, expired.Add(1*time.Second))
	if err != nil {
		t.Fatalf("post-expire RecordFailure: %v", err)
	}
	if locked4 {
		t.Fatal("should not lock on first failure after reset")
	}
	if fail2 != 1 {
		t.Errorf("post-expire fail = %d, want 1", fail2)
	}
}

// Property 10 扩展：Reset 清空计数（模拟登录成功）。
// 不变量：Reset 后 IsLocked 立即返回 false，下一次失败从 1 开始。
func TestLoginSecurityReset(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	ls := NewLoginSecurity(gdb, 3, 300)
	username := "reset-test"
	now := time.Now()

	// 3 次失败触发锁定。
	for i := 0; i < 3; i++ {
		_, _, err := ls.RecordFailure(ctx, username, now)
		if err != nil {
			t.Fatalf("RecordFailure %d: %v", i, err)
		}
	}
	locked, _, err := ls.IsLocked(ctx, username, now.Add(1*time.Second))
	if err != nil || !locked {
		t.Fatalf("should be locked; err=%v locked=%v", err, locked)
	}

	if err := ls.Reset(ctx, username); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	locked2, _, err := ls.IsLocked(ctx, username, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("IsLocked after Reset: %v", err)
	}
	if locked2 {
		t.Fatal("should not be locked after Reset")
	}

	// 下次失败应从 1 开始。
	_, fail, err := ls.RecordFailure(ctx, username, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("RecordFailure post-reset: %v", err)
	}
	if fail != 1 {
		t.Errorf("fail post-reset = %d, want 1", fail)
	}
}

// Property 10 边界：不存在该用户的失败记录时 IsLocked 返回 false（不报错）。
func TestLoginSecurityUnknownUser(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ls := NewLoginSecurity(gdb, 5, 900)
	locked, _, err := ls.IsLocked(context.Background(), "never-existed", time.Now())
	if err != nil {
		t.Fatalf("IsLocked on unknown user: %v", err)
	}
	if locked {
		t.Fatal("unknown user should not be locked")
	}
}
