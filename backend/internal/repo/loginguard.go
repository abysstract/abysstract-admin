package repo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/abysstract/abysstract-admin/backend/identity"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/query"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

// FailCount returns the consecutive failures of an account, or 0 when it has
// none or its lockout has expired.
func (s *LoginSecurity) FailCount(ctx context.Context, username string, now time.Time) (int32, error) {
	username = identity.ASCIICaseKey(username)
	if username == "" {
		return 0, nil
	}
	q := query.Use(s.db).SysLoginAttempt
	row, err := q.WithContext(ctx).Where(q.Username.Eq(username)).Take()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if row.LockedUntil != nil && !row.LockedUntil.After(resource.NormalizeTimestamp(now)) {
		return 0, nil
	}
	return row.FailCount, nil
}

// LoginIPGuard counts failed logins per source address in a fixed window
// (sys_login_ip_attempt). The first failure opens a window of the configured
// length; failures after it ends start a new one. A successful login does not
// clear the count, so one valid account cannot reset it for an attacker.
type LoginIPGuard struct {
	db     *gorm.DB
	window time.Duration
}

// NewLoginIPGuard returns a guard with a window of windowSecs seconds.
func NewLoginIPGuard(db *gorm.DB, windowSecs int) *LoginIPGuard {
	return &LoginIPGuard{db: db, window: time.Duration(windowSecs) * time.Second}
}

// Failures returns the failures of ip in its current window.
func (g *LoginIPGuard) Failures(ctx context.Context, ip string, now time.Time) (count int32, until time.Time, err error) {
	if ip == "" {
		return 0, time.Time{}, nil
	}
	q := query.Use(g.db).SysLoginIPAttempt
	row, err := q.WithContext(ctx).Where(q.IP.Eq(ip)).Take()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, time.Time{}, nil
	}
	if err != nil {
		return 0, time.Time{}, err
	}
	if !row.WindowUntil.After(resource.NormalizeTimestamp(now)) {
		return 0, time.Time{}, nil
	}
	return row.FailCount, row.WindowUntil, nil
}

// RecordFailure counts one failure of ip and returns the new count. It also
// removes rows whose window has ended.
func (g *LoginIPGuard) RecordFailure(ctx context.Context, ip string, now time.Time) (count int32, err error) {
	if ip == "" {
		return 0, nil
	}
	now = resource.NormalizeTimestamp(now)
	err = g.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		fresh := map[string]any{"ip": ip, "fail_count": 0, "window_until": now.Add(g.window)}
		if err := tx.Model(&model.SysLoginIPAttempt{}).Clauses(clause.OnConflict{DoNothing: true}).Create(fresh).Error; err != nil {
			return err
		}
		var row model.SysLoginIPAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("ip = ?", ip).Take(&row).Error; err != nil {
			return err
		}
		updates := map[string]any{"fail_count": row.FailCount + 1}
		count = row.FailCount + 1
		if !row.WindowUntil.After(now) {
			updates = map[string]any{"fail_count": 1, "window_until": now.Add(g.window)}
			count = 1
		}
		if err := tx.Model(&model.SysLoginIPAttempt{}).Where("ip = ?", ip).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Where("window_until < ? AND ip <> ?", now, ip).Delete(&model.SysLoginIPAttempt{}).Error
	})
	return count, err
}

// CaptchaUsedRepo remembers captcha challenges that were already answered
// (sys_captcha_used), so a solved challenge cannot be replayed.
type CaptchaUsedRepo struct {
	db *gorm.DB
}

// NewCaptchaUsedRepo returns a CaptchaUsedRepo.
func NewCaptchaUsedRepo(db *gorm.DB) *CaptchaUsedRepo {
	return &CaptchaUsedRepo{db: db}
}

// MarkUsed records id and reports whether it was unused. Entries past their
// expiry are removed on the way.
func (r *CaptchaUsedRepo) MarkUsed(ctx context.Context, id string, expiresAt, now time.Time) (fresh bool, err error) {
	now = resource.NormalizeTimestamp(now)
	expiresAt = resource.NormalizeTimestamp(expiresAt)
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at < ?", now).Delete(&model.SysCaptchaUsed{}).Error; err != nil {
			return err
		}
		res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.SysCaptchaUsed{ID: id, ExpiresAt: expiresAt})
		if res.Error != nil {
			return res.Error
		}
		fresh = res.RowsAffected == 1
		return nil
	})
	return fresh, err
}
