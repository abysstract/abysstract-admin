package repo

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// ProviderGoogle is the sys_user_identity provider for Google sign-in.
const ProviderGoogle = "google"

// UserByIdentity returns the undeleted user bound to the external account
// (provider, subject), or ErrNotFound.
func (r *Repo) UserByIdentity(ctx context.Context, provider, subject string) (*model.SysUser, error) {
	var userID int64
	err := r.db.WithContext(ctx).
		Table("sys_user_identity").
		Select("user_id").
		Where("provider = ? AND subject = ?", provider, subject).
		Take(&userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var u model.SysUser
	if err := r.db.WithContext(ctx).Where("id = ? AND del_flag = 0", userID).Take(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// UsersByEmail returns up to two undeleted users whose email equals email,
// ignoring case. Two results mean the address is ambiguous.
func (r *Repo) UsersByEmail(ctx context.Context, email string) ([]model.SysUser, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, nil
	}
	var users []model.SysUser
	err := r.db.WithContext(ctx).
		Where("LOWER(email) = ? AND del_flag = 0", email).
		Order("id").Limit(2).Find(&users).Error
	return users, err
}

// BindIdentity binds the external account to the user. It returns
// gorm.ErrDuplicatedKey when the account is already bound, or the user already
// has an account for the provider.
func (r *Repo) BindIdentity(ctx context.Context, userID int64, provider, subject, email string) error {
	return r.db.WithContext(ctx).Table("sys_user_identity").Create(map[string]any{
		"provider": provider,
		"subject":  subject,
		"user_id":  userID,
		"email":    email,
	}).Error
}
