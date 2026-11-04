package repo

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// RefreshTokenRepo 封装 sys_refresh_token 表的访问（需求 6.10/6.11）。
//
// 仅在 cfg.RefreshTokenEnabled 时由 service 层使用。
// 存储的是 token 的 SHA-256 哈希，避免明文泄露。
type RefreshTokenRepo struct{ db *gorm.DB }

// NewRefreshTokenRepo 构造 repo。
func NewRefreshTokenRepo(db *gorm.DB) *RefreshTokenRepo { return &RefreshTokenRepo{db: db} }

// ErrRefreshTokenRevoked token 已被吊销（轮转或登出）。
var ErrRefreshTokenRevoked = errors.New("refresh token 已吊销")

// Create 写入一条新的 refresh token 记录。tokenHash = sha256hex(rawToken)。
// tokenVersion 绑定签发时的 user.token_version。
// RefreshToken RPC 校验 rec.token_version == user.token_version；
// 改密/重置密码递增 user.token_version 后，旧 refresh 即使未被批量吊销也会因版本不匹配被拒绝。
func (r *RefreshTokenRepo) Create(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time, tokenVersion int32) error {
	return r.db.WithContext(ctx).Create(&model.SysRefreshToken{
		UserID:       userID,
		TokenHash:    tokenHash,
		ExpiresAt:    NormalizeTimestamp(expiresAt),
		Revoked:      0,
		TokenVersion: tokenVersion,
	}).Error
}

// LookupActive 按 hash 查询未吊销且未过期的 token。
// 返回 (record, error)；已吊销/已过期/不存在返回各自错误。
func (r *RefreshTokenRepo) LookupActive(ctx context.Context, tokenHash string, now time.Time) (*model.SysRefreshToken, error) {
	var row model.SysRefreshToken
	err := r.db.WithContext(ctx).
		Where("token_hash = ?", tokenHash).
		Take(&row).Error
	if err != nil {
		return nil, err
	}
	if row.Revoked == 1 {
		return nil, ErrRefreshTokenRevoked
	}
	if !row.ExpiresAt.After(now) {
		return nil, errors.New("refresh token 已过期")
	}
	return &row, nil
}

// Revoke 吊销指定 hash 的 token。
func (r *RefreshTokenRepo) Revoke(ctx context.Context, tokenHash string) error {
	res := r.db.WithContext(ctx).
		Table("sys_refresh_token").
		Where("token_hash = ? AND revoked = 0", tokenHash).
		Update("revoked", 1)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrRefreshTokenRevoked
	}
	return nil
}

// RevokeAllForUser 吊销某用户的所有 refresh token（改密 / 登出 all-device）。
func (r *RefreshTokenRepo) RevokeAllForUser(ctx context.Context, userID int64) error {
	return revokeAllRefreshTokens(r.db.WithContext(ctx), userID)
}

// revokeAllRefreshTokens 在给定 gorm 会话（可为事务 tx）上执行吊销。
// 抽成独立函数便于在组合事务中复用：改密 + 吊销 refresh 必须原子完成，
// 否则吊销失败时旧 refresh token 仍可读取新 token_version 换取 access token。
func revokeAllRefreshTokens(db *gorm.DB, userID int64) error {
	res := db.Table("sys_refresh_token").
		Where("user_id = ?", userID).
		Update("revoked", 1)
	if res.Error != nil {
		return res.Error
	}
	return nil
}

// PurgeExpired 删除已过期的记录，减少表体积。返回删除行数。
func (r *RefreshTokenRepo) PurgeExpired(ctx context.Context, now time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("expires_at < ?", now).
		Delete(&model.SysRefreshToken{})
	return res.RowsAffected, res.Error
}
