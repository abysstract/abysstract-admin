package repo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/abysstract/abysstract-admin/backend/identity"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/resource"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/query"
)

// LoginSecurity 封装登录失败计数与账号锁定逻辑（需求 6.6/6.7）。
//
// 策略：
//   - 失败 +1，达到阈值 → 写入 locked_until = now + lockoutSeconds。
//   - 登录成功 → 重置计数与 locked_until。
//   - 锁定到期 → 下次检查时懒清零（locked_until 在过去视为已过期）。
//
// 所有时间用 UTC，避免跨时区比较出错。
type LoginSecurity struct {
	db          *gorm.DB
	maxFailures int
	lockoutSecs int
}

// NewLoginSecurity 构造一个 LoginSecurity。maxFailures/lockoutSecs 来自 conf。
func NewLoginSecurity(db *gorm.DB, maxFailures, lockoutSecs int) *LoginSecurity {
	return &LoginSecurity{db: db, maxFailures: maxFailures, lockoutSecs: lockoutSecs}
}

// ErrAccountLocked 账号当前处于锁定状态。
var ErrAccountLocked = errors.New("账号已锁定，请稍后再试")

// lockUntil 返回锁定截止时刻 = now + lockoutSecs。
func (s *LoginSecurity) lockUntil(now time.Time) time.Time {
	return now.Add(time.Duration(s.lockoutSecs) * time.Second)
}

// IsLocked 报告账号是否处于锁定状态（locked_until 在未来）。
// 锁定已过期时懒清零 fail_count 并返回 false。
func (s *LoginSecurity) IsLocked(ctx context.Context, username string, now time.Time) (locked bool, until time.Time, err error) {
	username = identity.ASCIICaseKey(username)
	now = resource.NormalizeTimestamp(now)
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.SysLoginAttempt
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("username = ?", username).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if row.LockedUntil == nil {
			return nil
		}
		if row.LockedUntil.After(now) {
			locked, until = true, *row.LockedUntil
			return nil
		}
		return tx.Where("username = ?", username).Delete(&model.SysLoginAttempt{}).Error
	})
	return
}

// RecordFailure locks one canonical identity inside the caller's transaction.
func (s *LoginSecurity) RecordFailure(ctx context.Context, username string, now time.Time) (locked bool, count int32, err error) {
	username = identity.ASCIICaseKey(username)
	if username == "" {
		return false, 0, nil
	}
	now = resource.NormalizeTimestamp(now)
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.SysLoginAttempt
		if err := tx.Model(&row).Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"username": username}).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("username = ?", username).Take(&row).Error; err != nil {
			return err
		}
		updates := map[string]any{"fail_count": gorm.Expr("fail_count + ?", 1)}
		if row.FailCount+1 >= int32(s.maxFailures) {
			updates["locked_until"] = s.lockUntil(now)
		}
		if err := tx.Model(&model.SysLoginAttempt{}).Where("username = ?", username).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Where("username = ?", username).Take(&row).Error; err != nil {
			return err
		}
		count, locked = row.FailCount, row.FailCount >= int32(s.maxFailures)
		return nil
	})
	return
}

// Reset 清空该账号的失败计数与锁定状态。登录成功时调用。
func (s *LoginSecurity) Reset(ctx context.Context, username string) error {
	username = identity.ASCIICaseKey(username)
	if username == "" {
		return nil
	}
	q := query.Use(s.db).SysLoginAttempt
	_, err := q.WithContext(ctx).Where(q.Username.Eq(username)).Delete()
	return err
}
