//go:build integration

package service

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect/v2"

	resource "github.com/abysstract/abysstract-admin/backend/resource"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"

	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

func openMenuIntegration(t *testing.T) (*repo.Repo, func()) {
	t.Helper()
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return repo.New(gdb), func() {}
}

func TestGeneratedMenuDatabaseCRUDPresenceParentAndRoleCleanup(t *testing.T) {
	base, closeDB := openMenuIntegration(t)
	defer closeDB()
	handler := newMenuIntegrationHandler(t, base)
	ctx := context.Background()

	root := createIntegrationMenu(t, handler, 0, "menu-root", "M")
	child := createIntegrationMenu(t, handler, root.Id, "menu-child", "C")

	rollbackHooks := newMenuHooks()
	rollbackHooks.AfterUpdate = func(context.Context, resource.Tx, *model.SysMenu, *model.SysMenu, repo.ColumnSet) error {
		return errors.New("forced Menu update rollback")
	}
	rollbackHooks.AfterDelete = func(context.Context, resource.Tx, *model.SysMenu) error {
		return errors.New("forced Menu delete rollback")
	}
	rollbackHooks.AfterBatchDelete = func(context.Context, resource.Tx, []*model.SysMenu) error {
		return errors.New("forced Menu batch rollback")
	}
	rollbackController, err := resourcegen.NewMenuController(resourcegen.MenuControllerDeps{
		Store: testutil.Store(t, base), Hooks: rollbackHooks, Repository: newMenuRepository(),
	})
	if err != nil {
		t.Fatalf("NewMenuController rollback fixture: %v", err)
	}

	list, err := handler.ListMenus(ctx, &adminv1.ListMenusRequest{
		Page:    &adminv1.PageRequest{PageNum: 1, PageSize: 100},
		Keyword: "child",
	})
	if err != nil {
		t.Fatalf("ListMenus keyword: %v", err)
	}
	if list.Page == nil || list.Page.Total != 1 || len(list.Items) != 1 || list.Items[0].Id != child.Id {
		t.Fatalf("flat ListMenus = items=%v page=%v, want one flat child", list.Items, list.Page)
	}
	if len(list.Items[0].Children) != 0 {
		t.Fatalf("flat ListMenus unexpectedly filled children: %#v", list.Items[0])
	}

	before := model.SysMenu{}
	if err := base.DB().Where("id = ?", child.Id).First(&before).Error; err != nil {
		t.Fatalf("load child before update: %v", err)
	}
	newName := "menu-child-renamed"
	updated, err := handler.UpdateMenu(ctx, &adminv1.UpdateMenuRequest{Id: child.Id, MenuName: &newName})
	if err != nil {
		t.Fatalf("selective UpdateMenu: %v", err)
	}
	if updated.Menu == nil || updated.Menu.MenuName != newName || updated.Menu.ParentId != before.ParentID {
		t.Fatalf("selective UpdateMenu response = %#v", updated.Menu)
	}
	var after model.SysMenu
	if err := base.DB().Where("id = ?", child.Id).First(&after).Error; err != nil {
		t.Fatalf("load child after update: %v", err)
	}
	if after.MenuName != newName || !after.UpdateTime.After(before.UpdateTime) {
		t.Fatalf("selective Menu update persisted incorrectly: before=%+v after=%+v", before, after)
	}

	sameName := newName
	sameAt := after.UpdateTime
	same, err := handler.UpdateMenu(ctx, &adminv1.UpdateMenuRequest{Id: child.Id, MenuName: &sameName})
	if err != nil {
		t.Fatalf("same-value UpdateMenu: %v", err)
	}
	if same.Menu == nil || !same.Menu.UpdateTime.AsTime().Equal(sameAt) {
		t.Fatalf("same-value Menu update changed UpdatedAt: got=%v want=%s", same.Menu, sameAt)
	}
	if _, err := handler.UpdateMenu(ctx, &adminv1.UpdateMenuRequest{Id: child.Id}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("empty Menu patch code = %v err=%v, want InvalidArgument", connect.CodeOf(err), err)
	}

	invalidType := "X"
	if _, err := handler.UpdateMenu(ctx, &adminv1.UpdateMenuRequest{Id: child.Id, MenuType: &invalidType}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("invalid menu_type code = %v err=%v, want InvalidArgument", connect.CodeOf(err), err)
	}

	rootParent := child.Id
	if _, err := handler.UpdateMenu(ctx, &adminv1.UpdateMenuRequest{Id: root.Id, ParentId: &rootParent}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("Menu parent cycle code = %v err=%v, want InvalidArgument", connect.CodeOf(err), err)
	}

	if _, err := rollbackController.UpdateMenu(ctx, &adminv1.UpdateMenuRequest{Id: child.Id, ParentId: ptrInt64(0)}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("forced Menu update rollback code = %v err=%v, want Internal", connect.CodeOf(err), err)
	}
	var rollbackChild model.SysMenu
	if err := base.DB().Where("id = ?", child.Id).First(&rollbackChild).Error; err != nil {
		t.Fatalf("load child after update rollback: %v", err)
	}
	if rollbackChild.ParentID != root.Id || rollbackChild.MenuName != newName {
		t.Fatalf("Menu update rollback leaked state: %+v", rollbackChild)
	}

	if _, err := handler.DeleteMenu(ctx, &adminv1.DeleteMenuRequest{Id: root.Id}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("Menu parent delete code = %v err=%v, want FailedPrecondition", connect.CodeOf(err), err)
	}

	if err := base.DB().Exec("INSERT INTO sys_role_menu (role_id, menu_id) VALUES (?, ?)", 1, child.Id).Error; err != nil {
		t.Fatalf("seed role_menu: %v", err)
	}
	if _, err := rollbackController.DeleteMenu(ctx, &adminv1.DeleteMenuRequest{Id: child.Id}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("forced Menu delete rollback code = %v err=%v, want Internal", connect.CodeOf(err), err)
	}
	var relationCount int64
	if err := base.DB().Table("sys_role_menu").Where("menu_id = ?", child.Id).Count(&relationCount).Error; err != nil {
		t.Fatalf("count role_menu after rollback: %v", err)
	}
	if relationCount != 1 {
		t.Fatalf("Menu delete rollback removed role_menu relation: %d", relationCount)
	}
	if _, err := handler.DeleteMenu(ctx, &adminv1.DeleteMenuRequest{Id: child.Id}); err != nil {
		t.Fatalf("DeleteMenu leaf: %v", err)
	}
	if _, err := handler.GetMenu(ctx, &adminv1.GetMenuRequest{Id: child.Id}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("Get deleted Menu code = %v err=%v, want NotFound", connect.CodeOf(err), err)
	}
	if err := base.DB().Table("sys_role_menu").Where("menu_id = ?", child.Id).Count(&relationCount).Error; err != nil {
		t.Fatalf("count role_menu after delete: %v", err)
	}
	if relationCount != 0 {
		t.Fatalf("Menu Delete left role_menu relations: %d", relationCount)
	}

	batchOne := createIntegrationMenu(t, handler, 0, "menu-batch-one", "F")
	batchTwo := createIntegrationMenu(t, handler, 0, "menu-batch-two", "F")
	if _, err := rollbackController.BatchDeleteMenus(ctx, &adminv1.BatchDeleteMenusRequest{Ids: []int64{batchOne.Id, batchTwo.Id}}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("forced Menu batch rollback code = %v err=%v, want Internal", connect.CodeOf(err), err)
	}
	if _, err := handler.BatchDeleteMenus(ctx, &adminv1.BatchDeleteMenusRequest{Ids: []int64{batchOne.Id, batchTwo.Id, batchOne.Id}}); err != nil {
		t.Fatalf("BatchDeleteMenus: %v", err)
	}
	if _, err := handler.BatchDeleteMenus(ctx, &adminv1.BatchDeleteMenusRequest{Ids: []int64{999999999}}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("BatchDelete missing Menu code = %v err=%v, want NotFound", connect.CodeOf(err), err)
	}

	status := adminv1.Status_STATUS_ENABLED
	filtered, err := handler.ListMenus(ctx, &adminv1.ListMenusRequest{
		Page:   &adminv1.PageRequest{PageNum: 1, PageSize: 100},
		Status: &status,
	})
	if err != nil {
		t.Fatalf("ListMenus status filter: %v", err)
	}
	for _, item := range filtered.Items {
		if item.Status != adminv1.Status_STATUS_ENABLED {
			t.Fatalf("status filter returned %v: %#v", item.Status, item)
		}
	}

	if err := base.DB().Model(&model.SysMenu{}).Where("id = ?", root.Id).Update("status", 0).Error; err != nil {
		t.Fatalf("seed explicit-zero ListMenus status: %v", err)
	}
	unspecified := adminv1.Status_STATUS_UNSPECIFIED
	zeroFiltered, err := handler.ListMenus(ctx, &adminv1.ListMenusRequest{
		Page: &adminv1.PageRequest{PageNum: 1, PageSize: 100}, Keyword: "menu-root", Status: &unspecified,
	})
	if err != nil || zeroFiltered.Page == nil || zeroFiltered.Page.Total != 1 || len(zeroFiltered.Items) != 1 || zeroFiltered.Items[0].Id != root.Id {
		t.Fatalf("ListMenus explicit STATUS_UNSPECIFIED err=%v items=%v page=%v, want status=0 row", err, zeroFiltered.Items, zeroFiltered.Page)
	}

	if err := base.DB().Model(&model.SysMenu{}).Where("id = ?", root.Id).Update("status", 99).Error; err != nil {
		t.Fatalf("seed invalid Menu status: %v", err)
	}
	if _, err := handler.GetMenu(ctx, &adminv1.GetMenuRequest{Id: root.Id}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("unknown Menu status Get code = %v err=%v, want Internal", connect.CodeOf(err), err)
	}
}
