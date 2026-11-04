package repo

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm/clause"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/query"
)

// ListMenuRows is the injected Menu Resource List path. Menu has no soft-delete
// flag; the specialized repository exists for typed query semantics and the
// role_menu/delete transaction boundary, not for a second CRUD API.
func (r *Repo) ListMenuRows(
	ctx context.Context,
	page Page,
	keyword string,
	parentID *int64,
	menuType *string,
	status *int16,
	orders []Order,
	access AccessScope,
) ([]*model.SysMenu, int64, error) {
	page.Num, page.Size = normalizePage(page.Num, page.Size)
	q := access.Apply(r.db.WithContext(ctx)).Model(&model.SysMenu{})
	if search := Search([]Column{"menu_name", "path", "component", "perms"}, keyword); !search.IsNoop() {
		q = search.Apply(q)
	}
	if parentID != nil {
		q = Equal(Column("parent_id"), *parentID).Apply(q)
	}
	if menuType != nil {
		q = Equal(Column("menu_type"), *menuType).Apply(q)
	}
	if status != nil {
		q = Equal(Column("status"), *status).Apply(q)
	}
	for _, order := range StableOrders(orders, Column("id")) {
		q = OrderClause(order).Apply(q)
	}

	var total int64
	if err := wrapDBErr(q.Count(&total).Error); err != nil {
		return nil, 0, err
	}
	var list []*model.SysMenu
	if err := wrapDBErr(q.Offset((page.Num - 1) * page.Size).Limit(page.Size).Find(&list).Error); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *Repo) GetMenuResource(ctx context.Context, id int64, access AccessScope) (*model.SysMenu, error) {
	var m model.SysMenu
	if err := access.Apply(r.db.WithContext(ctx)).Where("id = ?", id).First(&m).Error; err != nil {
		return nil, wrapDBErr(err)
	}
	return &m, nil
}

func (r *Repo) GetMenuResourceForUpdate(ctx context.Context, id int64, access AccessScope) (*model.SysMenu, error) {
	var m model.SysMenu
	if err := access.Apply(r.db.WithContext(ctx)).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&m).Error; err != nil {
		return nil, wrapDBErr(err)
	}
	return &m, nil
}

func (r *Repo) GetManyMenuResourcesForUpdate(ctx context.Context, ids []int64, access AccessScope) ([]*model.SysMenu, error) {
	var list []*model.SysMenu
	if err := access.Apply(r.db.WithContext(ctx)).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", ids).Find(&list).Error; err != nil {
		return nil, wrapDBErr(err)
	}
	return list, nil
}

// PrepareMenuCreate keeps the legacy database defaults while validating a
// non-root parent before generated Repository.Create runs.
func (r *Repo) PrepareMenuCreate(ctx context.Context, m *model.SysMenu) error {
	if m.Status == 0 {
		m.Status = 1
	}
	if m.Visible == 0 {
		m.Visible = 1
	}
	return r.ValidateMenuParent(ctx, 0, m.ParentID)
}

// ValidateMenuParent rejects self-parenting and moving below a descendant.
func (r *Repo) ValidateMenuParent(ctx context.Context, id, parentID int64) error {
	if parentID == 0 {
		return nil
	}
	if parentID == id {
		return ErrCycle
	}
	current := parentID
	visited := map[int64]struct{}{}
	for current != 0 {
		if current == id {
			return ErrCycle
		}
		if _, ok := visited[current]; ok {
			return ErrCycle
		}
		visited[current] = struct{}{}
		parent, err := r.GetMenuResource(ctx, current, UnrestrictedAccessScope())
		if err != nil {
			return err
		}
		current = parent.ParentID
	}
	return nil
}

func (r *Repo) ValidateMenuDelete(ctx context.Context, id int64) error {
	var children int64
	if err := r.db.WithContext(ctx).Model(&model.SysMenu{}).
		Where("parent_id = ?", id).Count(&children).Error; err != nil {
		return wrapDBErr(err)
	}
	if children > 0 {
		return ErrHasChildren
	}
	return nil
}

// DeleteMenuResource hard-deletes a leaf and cleans role_menu in the same
// Controller-owned transaction. It deliberately does not open a nested tx.
func (r *Repo) DeleteMenuResource(ctx context.Context, id int64, access AccessScope) error {
	q := query.Use(r.db)
	if err := r.ValidateMenuDelete(ctx, id); err != nil {
		return err
	}
	if _, err := q.SysRoleMenu.WithContext(ctx).Where(q.SysRoleMenu.MenuID.Eq(id)).Delete(); err != nil {
		return wrapDBErr(err)
	}
	result := access.Apply(r.db.WithContext(ctx)).Where("id = ?", id).Delete(&model.SysMenu{})
	if err := wrapDBErr(result.Error); err != nil {
		return err
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) BatchDeleteMenuResources(ctx context.Context, ids []int64, access AccessScope) (int64, error) {
	q := query.Use(r.db)
	if len(ids) == 0 {
		return 0, fmt.Errorf("menu: BatchDelete ID 集合不能为空")
	}
	if _, err := q.SysRoleMenu.WithContext(ctx).Where(q.SysRoleMenu.MenuID.In(ids...)).Delete(); err != nil {
		return 0, wrapDBErr(err)
	}
	result := access.Apply(r.db.WithContext(ctx)).Where("id IN ?", ids).Delete(&model.SysMenu{})
	if err := wrapDBErr(result.Error); err != nil {
		return 0, err
	}
	if result.RowsAffected != int64(len(ids)) {
		return result.RowsAffected, ErrNotFound
	}
	return result.RowsAffected, nil
}

func ValidateMenuType(menuType string) error {
	switch strings.TrimSpace(menuType) {
	case "M", "C", "F":
		return nil
	default:
		return fmt.Errorf("menu_type %q 非法，必须是 M/C/F", menuType)
	}
}
