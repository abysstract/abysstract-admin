package repo

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm/clause"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/query"
)

// SuperAdminRoleKey is the single immutable role identity used by RBAC and
// destructive-operation guards. The role may be disabled intentionally, but
// it must never be deleted.
const SuperAdminRoleKey = "admin"

var (
	ErrProtectedRole  = errors.New("超级管理员角色不可删除")
	ErrLastActiveRole = errors.New("不能停用当前登录用户的唯一有效角色")
)

// MenuIDsOfRole reads the authorization relation for a role. Callers that need
// an active-role boundary must first load the role through GetRoleResource.
func (r *Repo) MenuIDsOfRole(ctx context.Context, roleID int64) ([]int64, error) {
	var ids []int64
	q := query.Use(r.db).SysRoleMenu
	err := q.WithContext(ctx).Where(q.RoleID.Eq(roleID)).Order(q.MenuID).Pluck(q.MenuID, &ids)
	return ids, wrapDBErr(err)
}

// GetRoleResource loads an active role for generated standard operations.
func (r *Repo) GetRoleResource(ctx context.Context, id int64, access AccessScope) (*model.SysRole, error) {
	var role model.SysRole
	if err := access.Apply(r.db.WithContext(ctx)).
		Where("id = ? AND del_flag = 0", id).First(&role).Error; err != nil {
		return nil, wrapDBErr(err)
	}
	return &role, nil
}

// GetRoleResourceForUpdate locks an active role inside the Controller-owned
// transaction. Deleted/inaccessible roles collapse to ErrNotFound.
func (r *Repo) GetRoleResourceForUpdate(ctx context.Context, id int64, access AccessScope) (*model.SysRole, error) {
	var role model.SysRole
	if err := access.Apply(r.db.WithContext(ctx)).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND del_flag = 0", id).First(&role).Error; err != nil {
		return nil, wrapDBErr(err)
	}
	return &role, nil
}

func (r *Repo) GetManyRoleResourcesForUpdate(ctx context.Context, ids []int64, access AccessScope) ([]*model.SysRole, error) {
	var roles []*model.SysRole
	if err := access.Apply(r.db.WithContext(ctx)).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ? AND del_flag = 0", ids).Find(&roles).Error; err != nil {
		return nil, wrapDBErr(err)
	}
	sort.Slice(roles, func(i, j int) bool { return roles[i].ID < roles[j].ID })
	return roles, nil
}

// ListRoleRows is the injected Resource list path. Role itself has no row-level
// DataScope; only active roles are returned.
func (r *Repo) ListRoleRows(ctx context.Context, page Page, keyword string, status *int16, orders []Order, access AccessScope) ([]*model.SysRole, int64, error) {
	page.Num, page.Size = normalizePage(page.Num, page.Size)
	q := access.Apply(r.db.WithContext(ctx)).Model(&model.SysRole{}).Where("del_flag = 0")
	if search := Search([]Column{"role_name", "role_key"}, keyword); !search.IsNoop() {
		q = search.Apply(q)
	}
	if status != nil {
		q = Equal(Column("status"), *status).Apply(q)
	}
	if len(orders) == 0 {
		// Requirements 1a/4b define the generated List default as the resource
		// key ascending. Keep the old role_sort presentation order out of the
		// API path; callers must request it explicitly if it is ever exposed.
		orders = []Order{{Column: Column("id")}}
	} else {
		orders = StableOrders(orders, Column("id"))
	}
	for _, order := range orders {
		q = OrderClause(order).Apply(q)
	}
	var total int64
	if err := wrapDBErr(q.Count(&total).Error); err != nil {
		return nil, 0, err
	}
	var roles []*model.SysRole
	if err := wrapDBErr(q.Offset((page.Num - 1) * page.Size).Limit(page.Size).Find(&roles).Error); err != nil {
		return nil, 0, err
	}
	return roles, total, nil
}

// PrepareRoleCreate retains the existing status/data_scope defaults before the
// generated Repository.Create writes the model.
func (r *Repo) PrepareRoleCreate(role *model.SysRole) error {
	if role.Status == 0 {
		role.Status = 1
	}
	if role.DataScope == 0 {
		role.DataScope = 1
	}
	return nil
}

func ValidateRoleStatus(status int16) error {
	if status != 1 && status != 2 {
		return fmt.Errorf("status 必须为 1（启用）或 2（停用），当前为 %d", status)
	}
	return nil
}

func ValidateRoleDataScope(scope int16) error {
	if scope < 1 || scope > 5 {
		return fmt.Errorf("data_scope 必须在 1 到 5 之间，当前为 %d", scope)
	}
	return nil
}

func ValidateRoleDelete(role *model.SysRole) error {
	if role != nil && role.RoleKey == SuperAdminRoleKey {
		return ErrProtectedRole
	}
	return nil
}

func (r *Repo) ChangeRoleStatusResource(ctx context.Context, id int64, status int16, access AccessScope) error {
	if err := ValidateRoleStatus(status); err != nil {
		return err
	}
	return r.InTransaction(ctx, func(tx *Repo) error {
		if _, err := tx.GetRoleResourceForUpdate(ctx, id, access); err != nil {
			return err
		}
		if status == 2 {
			if userID := currentUserID(ctx); userID != 0 {
				activeRoleIDs, err := tx.activeRoleIDsForUpdate(ctx, userID)
				if err != nil {
					return err
				}
				if len(activeRoleIDs) == 1 && activeRoleIDs[0] == id {
					return ErrLastActiveRole
				}
			}
		}
		result := access.Apply(tx.db.WithContext(ctx)).Model(&model.SysRole{}).
			Where("id = ? AND del_flag = 0", id).
			Updates(map[string]any{"status": status, "update_time": NormalizeTimestamp(time.Now())})
		if err := wrapDBErr(result.Error); err != nil {
			return err
		}
		return nil
	})
}

// activeRoleIDsForUpdate locks the current user's enabled, non-deleted roles
// so the last-role check cannot race with another role status change.
func (r *Repo) activeRoleIDsForUpdate(ctx context.Context, userID int64) ([]int64, error) {
	var ids []int64
	err := r.db.WithContext(ctx).Model(&model.SysRole{}).
		Where("id IN (?)", r.db.Model(&model.SysUserRole{}).Select("role_id").Where("user_id = ?", userID)).
		Where("status = 1 AND del_flag = 0").Order("id").
		Clauses(clause.Locking{Strength: "UPDATE"}).Pluck("id", &ids).Error
	return ids, wrapDBErr(err)
}

// DeleteRoleResource soft-deletes an active role and clears both relation
// tables in the surrounding Controller transaction.
func (r *Repo) DeleteRoleResource(ctx context.Context, id int64, access AccessScope) error {
	q := query.Use(r.db)
	role, err := r.GetRoleResourceForUpdate(ctx, id, access)
	if err != nil {
		return err
	}
	if err := ValidateRoleDelete(role); err != nil {
		return err
	}
	result := access.Apply(r.db.WithContext(ctx)).Model(&model.SysRole{}).
		Where("id = ? AND del_flag = 0", id).
		Updates(map[string]any{"del_flag": 1, "update_time": NormalizeTimestamp(time.Now())})
	if err := wrapDBErr(result.Error); err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	if _, err := q.SysUserRole.WithContext(ctx).Where(q.SysUserRole.RoleID.Eq(id)).Delete(); err != nil {
		return wrapDBErr(err)
	}
	if _, err := q.SysRoleMenu.WithContext(ctx).Where(q.SysRoleMenu.RoleID.Eq(id)).Delete(); err != nil {
		return wrapDBErr(err)
	}
	_, err = q.SysRoleDept.WithContext(ctx).Where(q.SysRoleDept.RoleID.Eq(id)).Delete()
	return wrapDBErr(err)
}

func (r *Repo) BatchDeleteRoleResources(ctx context.Context, ids []int64, access AccessScope) (int64, error) {
	q := query.Use(r.db)
	if len(ids) == 0 {
		return 0, fmt.Errorf("role: BatchDelete ID 集合不能为空")
	}
	result := access.Apply(r.db.WithContext(ctx)).Model(&model.SysRole{}).
		Where("id IN ? AND del_flag = 0", ids).
		Updates(map[string]any{"del_flag": 1, "update_time": NormalizeTimestamp(time.Now())})
	if err := wrapDBErr(result.Error); err != nil {
		return 0, err
	}
	if result.RowsAffected != int64(len(ids)) {
		return result.RowsAffected, ErrNotFound
	}
	if _, err := q.SysUserRole.WithContext(ctx).Where(q.SysUserRole.RoleID.In(ids...)).Delete(); err != nil {
		return 0, wrapDBErr(err)
	}
	if _, err := q.SysRoleMenu.WithContext(ctx).Where(q.SysRoleMenu.RoleID.In(ids...)).Delete(); err != nil {
		return 0, wrapDBErr(err)
	}
	if _, err := q.SysRoleDept.WithContext(ctx).Where(q.SysRoleDept.RoleID.In(ids...)).Delete(); err != nil {
		return 0, wrapDBErr(err)
	}
	return result.RowsAffected, nil
}

// UpdateRoleMenusResource replaces one role's menu relation in the caller's
// transaction. It validates role and menu existence before deleting anything.
func (r *Repo) UpdateRoleMenusResource(ctx context.Context, roleID int64, menuIDs []int64, access AccessScope) error {
	q := query.Use(r.db)
	if _, err := r.GetRoleResourceForUpdate(ctx, roleID, access); err != nil {
		return err
	}
	distinct := distinctInt64(menuIDs)
	if len(distinct) > 0 {
		var count int64
		if err := wrapDBErr(r.db.WithContext(ctx).Model(&model.SysMenu{}).Where("id IN ?", distinct).Count(&count).Error); err != nil {
			return err
		}
		if count != int64(len(distinct)) {
			return ErrNotFound
		}
	}
	if _, err := q.SysRoleMenu.WithContext(ctx).Where(q.SysRoleMenu.RoleID.Eq(roleID)).Delete(); err != nil {
		return wrapDBErr(err)
	}
	for _, menuID := range distinct {
		if err := q.SysRoleMenu.WithContext(ctx).Create(&model.SysRoleMenu{RoleID: roleID, MenuID: menuID}); err != nil {
			return wrapDBErr(err)
		}
	}
	result := r.db.WithContext(ctx).Model(&model.SysRole{}).
		Where("id = ? AND del_flag = 0", roleID).
		Update("update_time", NormalizeTimestamp(time.Now()))
	if err := wrapDBErr(result.Error); err != nil {
		return err
	}
	return nil
}

func distinctInt64(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
