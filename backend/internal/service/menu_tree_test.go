package service

import (
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

// Menu Tree projection remains an internal auth/layout concern; Menu CRUD List
// itself returns flat rows and never calls this helper.
func TestMenuTreeProjectionForAuthLayout(t *testing.T) {
	now := time.Now()
	items := buildMenuTree([]*model.SysMenu{
		{ID: 1, ParentID: 0, MenuName: "系统", MenuType: "M", Status: 1, CreateTime: now, UpdateTime: now},
		{ID: 2, ParentID: 1, MenuName: "菜单", MenuType: "C", Status: 1, CreateTime: now, UpdateTime: now},
		{ID: 3, ParentID: 0, MenuName: "按钮", MenuType: "F", Status: 1, CreateTime: now, UpdateTime: now},
	})
	if len(items) != 2 || items[0].Id != 1 || len(items[0].Children) != 1 || items[0].Children[0].Id != 2 {
		t.Fatalf("Menu projection = %#v, want nested menu under system", items)
	}
	if items[1].Id != 3 || items[1].MenuType != "F" {
		t.Fatalf("Menu projection second root = %#v", items[1])
	}
}
