package repo

import (
	"context"
	"sort"

	"github.com/abysstract/abysstract-admin/backend/resource"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/query"
)

// ErrHasChildren 存在子节点，不允许删除。
var ErrHasChildren = resource.ErrHasChildren

// MenusOf 返回用户可见的目录/菜单（仅 M 目录、C 菜单，启用且可见），用于前端动态菜单与路由。
// admin 角色返回全部；其余按角色-菜单授权返回，并自动补齐缺失的祖先目录以保证树完整。
func (r *Repo) MenusOf(ctx context.Context, userID int64) ([]*model.SysMenu, error) {
	keys, err := r.RoleKeysOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	isAdmin := false
	for _, k := range keys {
		if k == SuperAdminRoleKey {
			isAdmin = true
			break
		}
	}

	var list []*model.SysMenu
	if isAdmin {
		if err = r.db.WithContext(ctx).Model(&model.SysMenu{}).
			Where("menu_type IN ?", []string{"M", "C"}).
			Where("status = 1 AND visible = 1").
			Find(&list).Error; err != nil {
			return nil, err
		}
		// admin 无需按授权补齐祖先，但仍需剪掉父级被停用/隐藏的孤儿子树。
		return pruneAndSortMenuTree(list), nil
	}

	q := query.Use(r.db)
	m, rm, ur, role := q.SysMenu, q.SysRoleMenu, q.SysUserRole, q.SysRole
	list, err = m.WithContext(ctx).
		Join(rm, rm.MenuID.EqCol(m.ID)).
		Join(ur, ur.RoleID.EqCol(rm.RoleID)).
		Join(role, role.ID.EqCol(ur.RoleID), role.Status.Eq(1), role.DelFlag.Eq(0)).
		Where(ur.UserID.Eq(userID), m.MenuType.In("M", "C"), m.Status.Eq(1), m.Visible.Eq(1)).
		Distinct(m.ALL).Find()
	if err != nil {
		return nil, err
	}

	// 补齐缺失的祖先目录（仅启用且可见），避免子菜单因父目录未授权而在树中丢失。
	included := make(map[int64]bool, len(list))
	for _, m := range list {
		included[m.ID] = true
	}
	var missing []int64
	for _, m := range list {
		if m.ParentID != 0 && !included[m.ParentID] {
			missing = append(missing, m.ParentID)
		}
	}
	for len(missing) > 0 {
		var parents []*model.SysMenu
		if err := r.db.WithContext(ctx).
			Where("id IN ? AND menu_type IN ?", missing, []string{"M", "C"}).
			Where("status = 1 AND visible = 1").
			Find(&parents).Error; err != nil {
			return nil, err
		}
		missing = missing[:0]
		for _, p := range parents {
			if included[p.ID] {
				continue
			}
			included[p.ID] = true
			list = append(list, p)
			if p.ParentID != 0 && !included[p.ParentID] {
				missing = append(missing, p.ParentID)
			}
		}
	}

	return pruneAndSortMenuTree(list), nil
}

// pruneAndSortMenuTree 剔除父级（非根）缺失的孤儿子树（祖先被停用/隐藏时整棵不可见），
// 并按 parent_id, order_num, id 稳定排序，保证树构建顺序确定。
func pruneAndSortMenuTree(list []*model.SysMenu) []*model.SysMenu {
	present := make(map[int64]bool, len(list))
	for _, m := range list {
		present[m.ID] = true
	}
	for {
		removed := false
		kept := list[:0]
		for _, m := range list {
			if m.ParentID != 0 && !present[m.ParentID] {
				delete(present, m.ID)
				removed = true
				continue
			}
			kept = append(kept, m)
		}
		list = kept
		if !removed {
			break
		}
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].ParentID != list[j].ParentID {
			return list[i].ParentID < list[j].ParentID
		}
		if list[i].OrderNum != list[j].OrderNum {
			return list[i].OrderNum < list[j].OrderNum
		}
		return list[i].ID < list[j].ID
	})
	return list
}
