// Package repo 是基于 gorm + 生成模型的数据访问层。
package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/abysstract/abysstract-admin/backend/identity"

	"github.com/abysstract/abysstract-admin/backend/resource"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gen/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/query"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
)

// ErrNotFound 记录不存在。
var ErrNotFound = resource.ErrNotFound

// ErrEmptyPatch 表示 selective update 没有任何列需要更新（空 patch）。
var ErrEmptyPatch = resource.ErrEmptyPatch

// ErrInvalidUpdateColumns indicates a selective update received unknown or duplicate columns.
var ErrInvalidUpdateColumns = resource.ErrInvalidUpdateColumns

type Repo struct{ db *gorm.DB }

func New(db *gorm.DB) *Repo { return &Repo{db: db} }

// WithDB binds an existing GORM database handle to a child Repo. Callers use
// it only to preserve an already-open transaction; InTransaction owns the
// transaction lifecycle for business code.
func (r *Repo) WithDB(db *gorm.DB) *Repo { return &Repo{db: db} }

// InTransaction runs fn with a Repo bound to one database transaction. A
// callback error rolls the transaction back; nil commits it. Generated
// Resource controllers use this as their only transaction boundary.
func (r *Repo) InTransaction(ctx context.Context, fn func(txRepo *Repo) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(r.WithDB(tx))
	})
}

// Transaction preserves the legacy repository API. New Resource controller
// code must use InTransaction instead.
func (r *Repo) Transaction(ctx context.Context, fn func(tx *Repo) error) error {
	return r.InTransaction(ctx, fn)
}

// DB 返回底层 gorm.DB，供测试夹具直接构造数据（不对外暴露到 service 层）。
// 业务读写在 repo 方法内完成，不应绕过此方法拼业务 SQL。
func (r *Repo) DB() *gorm.DB { return r.db }

// currentUserID 从 context 取当前登录用户 ID；未登录返回 0。
// 与 service.currentUserID 同源；repo 层独立定义以避免反向依赖 service。
func currentUserID(ctx context.Context) int64 {
	if c, ok := interceptor.ClaimsFromContext(ctx); ok {
		return c.UserID
	}
	return 0
}

// VerifyPassword 校验登录名+密码（仅未删除用户）。
func (r *Repo) VerifyPassword(ctx context.Context, username, password string) (*model.SysUser, error) {
	var u model.SysUser
	if err := r.db.WithContext(ctx).
		Where("username = ? AND del_flag = 0", identity.ASCIICaseKey(username)).First(&u).Error; err != nil {
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) != nil {
		return nil, errors.New("密码错误")
	}
	return &u, nil
}

// TokenVersion 返回用户当前的 token_version。
// 用户不存在/已删除返回 ErrNotFound。
func (r *Repo) TokenVersion(ctx context.Context, userID int64) (int64, error) {
	var v int32
	err := r.db.WithContext(ctx).
		Table("sys_user").
		Select("token_version").
		Where("id = ? AND del_flag = 0", userID).
		Take(&v).Error
	if err != nil {
		return 0, err
	}
	return int64(v), nil
}

// IncrementTokenVersion 使用户当前 token_version +1，使既有 token 失效。
// 配合拦截器 ver 比对实现会话失效（需求 6.4/6.8）。
func (r *Repo) IncrementTokenVersion(ctx context.Context, userID int64) error {
	u := query.Use(r.db).SysUser
	res, err := u.WithContext(ctx).Where(u.ID.Eq(userID), u.DelFlag.Eq(0)).UpdateSimple(
		u.TokenVersion.SetCol(u.TokenVersion.Add(1)), u.UpdateTime.SetCol(field.NewUnsafeFieldRaw("CURRENT_TIMESTAMP")),
	)
	if err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// PwdResetRequired 返回用户的 pwd_reset_required 标志（true 表示需改密）。
func (r *Repo) PwdResetRequired(ctx context.Context, userID int64) (bool, error) {
	var v int16
	err := r.db.WithContext(ctx).
		Table("sys_user").
		Select("pwd_reset_required").
		Where("id = ? AND del_flag = 0", userID).
		Take(&v).Error
	if err != nil {
		return false, err
	}
	return v == 1, nil
}

// SetPwdResetRequired 显式设置用户的 pwd_reset_required 标志。
// 用于重置密码后强制首次登录改密。
func (r *Repo) SetPwdResetRequired(ctx context.Context, userID int64, required bool) error {
	v := 0
	if required {
		v = 1
	}
	return r.updateUserValues(ctx, userID, map[string]any{"pwd_reset_required": v, "update_time": gorm.Expr("CURRENT_TIMESTAMP")})
}

// ChangeOwnPassword 改密 + 清除首次改密标志 + 递增 token_version，事务内完成。
// oldHash 用于校验旧密码（在 service 层比对）；newHash 已 hash。
// 若用户不存在/已删除返回 ErrNotFound。
//
// 注意：本方法不吊销 refresh token。改密场景请改用 ChangeOwnPasswordAndRevokeRefresh，
// 否则旧 refresh token 可读取新 token_version 换取 access token，绕过会话失效。
func (r *Repo) ChangeOwnPassword(ctx context.Context, userID int64, newHash string) error {
	u := query.Use(r.db).SysUser
	res, err := u.WithContext(ctx).Where(u.ID.Eq(userID), u.DelFlag.Eq(0)).UpdateSimple(
		u.Password.Value(newHash), u.PwdResetRequired.Value(0),
		u.TokenVersion.SetCol(u.TokenVersion.Add(1)), u.UpdateTime.SetCol(field.NewUnsafeFieldRaw("CURRENT_TIMESTAMP")),
	)
	if err != nil {
		return err
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ChangeOwnPasswordAndRevokeRefresh 在单个事务内同时完成「改密 + 递增 token_version +
// 吊销该用户所有 refresh token」，保证两者原子成功或原子失败。
//
// 密码、会话版本和刷新令牌状态共同提交；任一步骤失败时整个事务回滚。
// RefreshToken RPC 同时核对令牌记录与用户的 token_version。
//
// refresh 可为 nil（未启用刷新令牌时），此时仅执行密码更新。
func (r *Repo) ChangeOwnPasswordAndRevokeRefresh(
	ctx context.Context, userID int64, newHash string, refresh *RefreshTokenRepo,
) error {
	return wrapDBErr(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		u := query.Use(tx).SysUser
		res, err := u.WithContext(ctx).Where(u.ID.Eq(userID), u.DelFlag.Eq(0)).UpdateSimple(
			u.Password.Value(newHash), u.PwdResetRequired.Value(0),
			u.TokenVersion.SetCol(u.TokenVersion.Add(1)), u.UpdateTime.SetCol(field.NewUnsafeFieldRaw("CURRENT_TIMESTAMP")),
		)
		if err != nil {
			return err
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		if refresh != nil {
			if err := revokeAllRefreshTokens(tx, userID); err != nil {
				return err
			}
		}
		return nil
	}))
}

// UpdateAvatar 更新当前用户的头像引用（需求 9.3）。
// 头像上传校验失败时 service 层不会调用此方法，从而保证原引用被保留。
func (r *Repo) UpdateAvatar(ctx context.Context, userID int64, avatar string) error {
	return r.updateUserValues(ctx, userID, map[string]any{"avatar": avatar, "update_time": NormalizeTimestamp(time.Now())})
}

// UpdateProfileOwn 更新当前用户自己的资料（nickname/email/phone），不改 dept_id 与角色（需求 9.13）。
// username 不允许自行修改，故不在此方法签名中。
func (r *Repo) UpdateProfileOwn(ctx context.Context, userID int64, nickname, email, phone string) error {
	return r.updateUserValues(ctx, userID, map[string]any{
		"nickname":    nickname,
		"email":       email,
		"phone":       phone,
		"update_time": NormalizeTimestamp(time.Now()),
	})
}

// DashboardStats 仪表盘统计（需求 9.14）。
// todayLoginCount 按 login_time 当天计数；recentLogins 取最近 10 条。
type DashboardStats struct {
	UserCount       int64
	RoleCount       int64
	TodayLoginCount int64
	RecentLogins    []LoginLogRow
}

// LoginLogRow 登录日志展示行。
type LoginLogRow struct {
	Username  string    `gorm:"column:username"`
	IP        string    `gorm:"column:ip"`
	LoginTime time.Time `gorm:"column:login_time"`
	Status    int32     `gorm:"column:status"`
}

// DashboardStats 查询统计（需求 9.14）。
func (r *Repo) DashboardStats(ctx context.Context) (*DashboardStats, error) {
	stats := &DashboardStats{}
	if err := r.db.WithContext(ctx).
		Table("sys_user").
		Where("del_flag = 0 AND status = 1").
		Count(&stats.UserCount).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).
		Table("sys_role").
		Where("del_flag = 0 AND status = 1").
		Count(&stats.RoleCount).Error; err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if err := r.db.WithContext(ctx).
		Table("sys_login_log").
		Where("login_time >= ?", today).
		Count(&stats.TodayLoginCount).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).
		Table("sys_login_log").
		Order("login_time DESC").
		Limit(10).
		Scan(&stats.RecentLogins).Error; err != nil {
		return nil, err
	}
	return stats, nil
}

// GetUserForUpdate keeps account state stable during token rotation and writes.
func (r *Repo) GetUserForUpdate(ctx context.Context, id int64) (*model.SysUser, error) {
	var user model.SysUser
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND del_flag = 0", id).Take(&user).Error; err != nil {
		return nil, wrapDBErr(err)
	}
	return &user, nil
}

func (r *Repo) GetUser(ctx context.Context, id int64) (*model.SysUser, error) {
	var u model.SysUser
	if err := r.db.WithContext(ctx).
		Where("id = ? AND del_flag = 0", id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// ListUsersForExport 按关键字与状态过滤，返回至多 limit+1 条记录，**不经过 normalizePage 的 100 上限**。
// 仅供 Excel 导出（需求 9.5）使用；标准分页列表由 generated User Resource 的 ListUserRows 提供。
// 查询 limit+1 条：service 层据此检测是否超过 maxExportRows 上限（返回长度 > limit 即超限），
// 使 ExportExcel 的 100,000 行上限检查可达。
func (r *Repo) ListUsersForExport(ctx context.Context, keyword string, status int16, limit int) ([]*model.SysUser, error) {
	q := r.db.WithContext(ctx).Model(&model.SysUser{}).Where("del_flag = 0")
	q = Search([]Column{"username", "nickname"}, keyword).Apply(q)
	if status != 0 {
		q = q.Where("status = ?", status)
	}
	// 数据权限作用域：与 generated ListUserRows 一致，导出也按当前用户可见范围过滤。
	q = q.Scopes(r.DataScope(ctx, "dept_id", "create_by"))

	var list []*model.SysUser
	// 多取 1 条用于超限检测；service 层负责在超限时拒绝。
	if err := q.Order("id").Limit(limit + 1).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// CreateUser 创建用户（hash 密码）并绑定角色，事务内完成。
// create_by 写入当前登录用户 ID（支撑「仅本人」data_scope）。
func (r *Repo) CreateUser(ctx context.Context, u *model.SysUser, plainPassword string, roleIDs []int64) error {
	u.Username = identity.ASCIICaseKey(u.Username)
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	now := NormalizeTimestamp(time.Now())
	u.Password = string(hash)
	u.CreateTime, u.UpdateTime = now, now
	u.CreateBy = currentUserID(ctx)
	if u.Status == 0 {
		u.Status = 1
	}

	return wrapDBErr(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(u).Error; err != nil {
			return err
		}
		if err := replaceUserRoles(tx, u.ID, roleIDs); err != nil {
			return err
		}
		return tx.Where("id = ?", u.ID).Take(u).Error
	}))
}

// ErrProtectedUserUpdate 表示非内置管理员本人试图修改内置管理员。
var ErrProtectedUserUpdate = errors.New("不能修改内置管理员")

// UpdateUser 更新用户资料并重置角色绑定（不改密码）。
// ID=1 只能由内置管理员本人修改；该保护在 repo 层重复校验，避免
// 任何绕过 UserService 的调用同时改写管理员资料和角色绑定。
func (r *Repo) UpdateUser(ctx context.Context, u *model.SysUser, roleIDs []int64) error {
	if u != nil && u.ID == 1 && currentUserID(ctx) != 1 {
		return ErrProtectedUserUpdate
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := r.WithDB(tx).GetUserForUpdate(ctx, u.ID); err != nil {
			return wrapDBErr(err)
		}
		res := tx.Model(&model.SysUser{}).
			Where("id = ? AND del_flag = 0", u.ID).
			Updates(map[string]any{
				"nickname":    u.Nickname,
				"email":       u.Email,
				"phone":       u.Phone,
				"dept_id":     u.DeptID,
				"remark":      u.Remark,
				"update_time": NormalizeTimestamp(time.Now()),
			})
		if res.Error != nil {
			return res.Error
		}
		if err := replaceUserRoles(tx, u.ID, roleIDs); err != nil {
			return err
		}
		return tx.Where("id = ?", u.ID).Take(u).Error
	})
}

// DeleteUser 软删除（del_flag=1），并清理用户-角色关联。
// 保护：禁止删除 ID=1 的内置管理员与当前登录用户，防止误操作使系统失去管理员入口。
func (r *Repo) DeleteUser(ctx context.Context, id int64) error {
	if err := guardProtectedUser(ctx, id); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.SysUser{}).
			Where("id = ? AND del_flag = 0", id).
			Updates(map[string]any{"del_flag": 1, "update_time": NormalizeTimestamp(time.Now())})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		q := query.Use(tx).SysUserRole
		_, err := q.WithContext(ctx).Where(q.UserID.Eq(id)).Delete()
		return err
	})
}

// errProtectedUser 试图删除受保护的用户（内置管理员 ID=1 或当前登录用户自身）。
var errProtectedUser = errors.New("不能删除内置管理员或当前登录用户")

// guardProtectedUser 校验目标 ID 不是内置管理员（1）也不是当前登录用户。
// service 层调用单条/批量删除前必须先调本函数。
func guardProtectedUser(ctx context.Context, id int64) error {
	if id == 1 {
		return errProtectedUser
	}
	if uid := currentUserID(ctx); uid != 0 && id == uid {
		return errProtectedUser
	}
	return nil
}

// BatchDeleteUsers 批量软删除用户并清理用户-角色关联（需求 9.7）。
// 单事务保证「全部成功或全部回滚」：先对 ID 去重并锁定全部 active rows，
// 要求每个 distinct ID 都存在且可见，再执行软删除和关系清理。这样请求中
// 混入不存在/已删除 ID 时不会出现部分删除。
// 保护：ids 中包含内置管理员（ID=1）或当前登录用户时直接拒绝整个请求。
func (r *Repo) BatchDeleteUsers(ctx context.Context, ids []int64) (int64, error) {
	distinct := distinctInt64(ids)
	if len(distinct) == 0 {
		return 0, nil
	}
	// 受保护用户检查：任意 id 命中即整体拒绝（不部分删除）。
	uid := currentUserID(ctx)
	for _, id := range distinct {
		if id == 1 || (uid != 0 && id == uid) {
			return 0, errProtectedUser
		}
	}
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var users []model.SysUser
		if err := tx.Model(&model.SysUser{}).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id IN ? AND del_flag = 0", distinct).
			Find(&users).Error; err != nil {
			return err
		}
		if len(users) != len(distinct) {
			return ErrNotFound
		}
		res := tx.Model(&model.SysUser{}).
			Where("id IN ? AND del_flag = 0", distinct).
			Updates(map[string]any{"del_flag": 1, "update_time": NormalizeTimestamp(time.Now())})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != int64(len(distinct)) {
			return fmt.Errorf("BatchDeleteUsers 影响行数 %d 与 distinct ID 数 %d 不一致", res.RowsAffected, len(distinct))
		}
		affected = res.RowsAffected
		q := query.Use(tx).SysUserRole
		_, err := q.WithContext(ctx).Where(q.UserID.In(distinct...)).Delete()
		return err
	})
	return affected, err
}

func (r *Repo) ChangeStatus(ctx context.Context, id int64, status int16) error {
	return r.updateUserValues(ctx, id, map[string]any{"status": status, "update_time": NormalizeTimestamp(time.Now())})
}

// ResetPassword 管理员重置目标用户密码。
// 同步递增 token_version（吊销目标用户所有现有会话）并设置 pwd_reset_required=1
// （强制目标用户首次登录后修改临时密码），与改密/会话失效流程一致。
//
// 注意：本方法不吊销 refresh token。重置密码场景请改用 ResetPasswordAndRevokeRefresh，
// 否则目标用户此前的 refresh token 仍可读取新 token_version 换取 access token。
func (r *Repo) ResetPassword(ctx context.Context, id int64, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	res := r.db.WithContext(ctx).Model(&model.SysUser{}).
		Where("id = ? AND del_flag = 0", id).
		Updates(map[string]any{
			"password":           string(hash),
			"token_version":      gorm.Expr("token_version + 1"),
			"pwd_reset_required": 1,
			"update_time":        NormalizeTimestamp(time.Now()),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ResetPasswordAndRevokeRefresh 在单个事务内同时完成「重置密码 + 递增 token_version +
// 设置 pwd_reset_required + 吊销目标用户所有 refresh token」，原子成功或失败。
//
// 密码重置、强制改密标记、会话版本和刷新令牌状态共同提交；任一步骤失败时回滚。
// RefreshToken RPC 同时核对令牌记录与用户的 token_version。
//
// refresh 可为 nil（未启用刷新令牌时），此时仅执行密码重置。
func (r *Repo) ResetPasswordAndRevokeRefresh(
	ctx context.Context, id int64, newPassword string, refresh *RefreshTokenRepo,
) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return wrapDBErr(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.SysUser{}).
			Where("id = ? AND del_flag = 0", id).
			Updates(map[string]any{
				"password":           string(hash),
				"token_version":      gorm.Expr("token_version + 1"),
				"pwd_reset_required": 1,
				"update_time":        NormalizeTimestamp(time.Now()),
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		if refresh != nil {
			if err := revokeAllRefreshTokens(tx, id); err != nil {
				return err
			}
		}
		return nil
	}))
}

// RoleIDsOf 返回用户绑定的角色 ID。
func (r *Repo) RoleIDsOf(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	q := query.Use(r.db).SysUserRole
	err := q.WithContext(ctx).Where(q.UserID.Eq(userID)).Pluck(q.RoleID, &ids)
	return ids, err
}

// RoleKeysOf 返回用户启用角色的标识（role_key）。
func (r *Repo) RoleKeysOf(ctx context.Context, userID int64) ([]string, error) {
	var keys []string
	q := query.Use(r.db)
	role, ur := q.SysRole, q.SysUserRole
	err := role.WithContext(ctx).Join(ur, ur.RoleID.EqCol(role.ID)).
		Where(ur.UserID.Eq(userID), role.Status.Eq(1), role.DelFlag.Eq(0)).Pluck(role.RoleKey, &keys)
	return keys, err
}

// PermsOf 返回用户的按钮/操作权限标识；含 admin 角色则返回通配 "*:*:*"。
func (r *Repo) PermsOf(ctx context.Context, userID int64) ([]string, error) {
	keys, err := r.RoleKeysOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, k := range keys {
		if k == SuperAdminRoleKey {
			return []string{"*:*:*"}, nil
		}
	}
	var perms []string
	q := query.Use(r.db)
	m, rm, ur, role := q.SysMenu, q.SysRoleMenu, q.SysUserRole, q.SysRole
	err = m.WithContext(ctx).
		Join(rm, rm.MenuID.EqCol(m.ID)).
		Join(ur, ur.RoleID.EqCol(rm.RoleID)).
		Join(role, role.ID.EqCol(ur.RoleID), role.Status.Eq(1), role.DelFlag.Eq(0)).
		Where(ur.UserID.Eq(userID), m.Perms.Neq(""), m.Status.Eq(1)).
		Distinct(m.Perms).Pluck(m.Perms, &perms)
	return perms, err
}

// replaceUserRoles 重置用户的角色绑定。
func replaceUserRoles(tx *gorm.DB, userID int64, roleIDs []int64) error {
	q := query.Use(tx).SysUserRole
	if _, err := q.WithContext(tx.Statement.Context).Where(q.UserID.Eq(userID)).Delete(); err != nil {
		return err
	}
	for _, rid := range roleIDs {
		if err := q.WithContext(tx.Statement.Context).Clauses(clause.OnConflict{DoNothing: true}).Create(&model.SysUserRole{UserID: userID, RoleID: rid}); err != nil {
			return err
		}
	}
	return nil
}

// updateUserValues resolves existence under the same lock as a selective write.
func (r *Repo) updateUserValues(ctx context.Context, id int64, values map[string]any) error {
	return r.InTransaction(ctx, func(tx *Repo) error {
		if _, err := tx.GetUserForUpdate(ctx, id); err != nil {
			return wrapDBErr(err)
		}
		return wrapDBErr(tx.db.Model(&model.SysUser{}).Where("id = ? AND del_flag = 0", id).Updates(values).Error)
	})
}
