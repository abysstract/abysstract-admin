package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// DataScopeAccessScope adapts the existing data-scope resolver to the opaque
// AccessScope contract consumed by generated Resource read controllers. The
// scope is evaluated against every query; it is never eligible for a global
// unrestricted cache.
func (r *Repo) DataScopeAccessScope(ctx context.Context, deptDBName, creatorDBName string) AccessScope {
	return NewAccessScope(func(db *gorm.DB) *gorm.DB {
		return db.Scopes(r.DataScope(ctx, deptDBName, creatorDBName))
	})
}

// GetUserResource loads one active User inside the caller's row-level scope.
// NotFound intentionally covers both a missing row and an out-of-scope row.
func (r *Repo) GetUserResource(ctx context.Context, id int64, access AccessScope) (*model.SysUser, error) {
	var user model.SysUser
	if err := access.Apply(r.db.WithContext(ctx)).
		Where("id = ? AND del_flag = 0", id).First(&user).Error; err != nil {
		return nil, wrapDBErr(err)
	}
	return &user, nil
}

// ListUserRows is the generated User List persistence boundary. It keeps the
// established active-row, keyword, status/dept filters and DataScope query
// semantics without exposing password or role relation columns.
func (r *Repo) ListUserRows(ctx context.Context, page Page, keyword string, status *int16, deptID *int64, orders []Order, access AccessScope) ([]*model.SysUser, int64, error) {
	page.Num, page.Size = normalizePage(page.Num, page.Size)
	q := access.Apply(r.db.WithContext(ctx)).Model(&model.SysUser{}).Where("del_flag = 0")
	if search := Search([]Column{"username", "nickname"}, keyword); !search.IsNoop() {
		q = search.Apply(q)
	}
	if status != nil {
		q = Equal(Column("status"), *status).Apply(q)
	}
	if deptID != nil {
		q = Equal(Column("dept_id"), *deptID).Apply(q)
	}
	if len(orders) == 0 {
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
	var users []*model.SysUser
	if err := wrapDBErr(q.Offset((page.Num - 1) * page.Size).Limit(page.Size).Find(&users).Error); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}
