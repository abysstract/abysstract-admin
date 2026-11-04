package service

import (
	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
)

// buildMenuTree is an internal auth/layout projection for MenusOf. It is not a
// MenuService CRUD path and does not implement the Menu Resource List RPC.
func buildMenuTree(list []*model.SysMenu) []*adminv1.Menu {
	nodes := make(map[int64]*adminv1.Menu, len(list))
	for _, m := range list {
		entity, err := resourcegen.MenuToEntity(m)
		if err != nil {
			continue
		}
		nodes[m.ID] = entity
	}
	var roots []*adminv1.Menu
	for _, m := range list {
		node := nodes[m.ID]
		if node == nil {
			continue
		}
		if parent, ok := nodes[m.ParentID]; ok && m.ParentID != 0 {
			parent.Children = append(parent.Children, node)
		} else {
			roots = append(roots, node)
		}
	}
	return roots
}
