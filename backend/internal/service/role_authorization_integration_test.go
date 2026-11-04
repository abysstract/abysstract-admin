//go:build integration

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"gorm.io/gorm/clause"

	"connectrpc.com/connect/v2"

	resource "github.com/abysstract/abysstract-admin/backend/resource"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"

	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

func openRoleIntegration(t *testing.T) (*repo.Repo, func()) {
	t.Helper()
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return repo.New(gdb), func() {}
}

func TestGeneratedRoleDatabaseCRUDAuthorizationAndRollback(t *testing.T) {
	base, closeDB := openRoleIntegration(t)
	defer closeDB()
	handler := newRoleIntegrationHandler(t, base)
	ctx := context.Background()
	adminCtx := interceptor.WithClaims(ctx, &jwt.Claims{UserID: 1, Username: "admin"})

	var superAdmin model.SysRole
	if err := base.DB().Where("role_key = ?", repo.SuperAdminRoleKey).First(&superAdmin).Error; err != nil {
		t.Fatalf("load super-admin Role: %v", err)
	}
	if _, err := handler.DeleteRole(ctx, &adminv1.DeleteRoleRequest{Id: superAdmin.ID}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("DeleteRole super-admin code = %v err=%v, want FailedPrecondition", connect.CodeOf(err), err)
	}
	var protectedSuperAdmin model.SysRole
	if err := base.DB().Where("id = ?", superAdmin.ID).First(&protectedSuperAdmin).Error; err != nil {
		t.Fatalf("reload super-admin Role after delete guard: %v", err)
	}
	if protectedSuperAdmin.DelFlag != 0 || protectedSuperAdmin.RoleKey != repo.SuperAdminRoleKey {
		t.Fatalf("super-admin delete guard changed row: %+v", protectedSuperAdmin)
	}

	support := createIntegrationRole(t, handler, "辅助管理员", "support-admin")
	var originalAdminRoleIDs []int64
	if err := base.DB().Raw("SELECT role_id FROM sys_user_role WHERE user_id = ? ORDER BY role_id", 1).Scan(&originalAdminRoleIDs).Error; err != nil {
		t.Fatalf("load original admin role bindings: %v", err)
	}
	if err := base.DB().Clauses(clause.OnConflict{DoNothing: true}).Create(&model.SysUserRole{UserID: 1, RoleID: support.Id}).Error; err != nil {
		t.Fatalf("bind support Role to admin: %v", err)
	}
	if _, err := handler.ChangeRoleStatus(adminCtx, &adminv1.ChangeRoleStatusRequest{Id: superAdmin.ID, Status: adminv1.Status_STATUS_DISABLED}); err != nil {
		t.Fatalf("other valid admin disabling super-admin Role: %v", err)
	}
	var disabledSuperAdmin model.SysRole
	if err := base.DB().Where("id = ?", superAdmin.ID).First(&disabledSuperAdmin).Error; err != nil {
		t.Fatalf("load disabled super-admin Role: %v", err)
	}
	if disabledSuperAdmin.Status != 2 {
		t.Fatalf("super-admin Role status = %d, want disabled", disabledSuperAdmin.Status)
	}
	if _, err := handler.ChangeRoleStatus(adminCtx, &adminv1.ChangeRoleStatusRequest{Id: superAdmin.ID, Status: adminv1.Status_STATUS_ENABLED}); err != nil {
		t.Fatalf("restore super-admin Role status: %v", err)
	}
	if err := base.DB().Exec("DELETE FROM sys_user_role WHERE user_id = ? AND role_id = ?", 1, support.Id).Error; err != nil {
		t.Fatalf("unbind support Role from admin: %v", err)
	}

	sole := createIntegrationRole(t, handler, "唯一角色测试", "sole-role")
	if err := base.DB().Exec("DELETE FROM sys_user_role WHERE user_id = ?", 1).Error; err != nil {
		t.Fatalf("clear admin role bindings for last-role guard: %v", err)
	}
	if err := base.DB().Exec("INSERT INTO sys_user_role (user_id, role_id) VALUES (?, ?)", 1, sole.Id).Error; err != nil {
		t.Fatalf("bind sole Role to admin: %v", err)
	}
	if _, err := handler.ChangeRoleStatus(adminCtx, &adminv1.ChangeRoleStatusRequest{Id: sole.Id, Status: adminv1.Status_STATUS_DISABLED}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("ChangeRoleStatus sole active Role code = %v err=%v, want FailedPrecondition", connect.CodeOf(err), err)
	}
	var stillEnabledSole model.SysRole
	if err := base.DB().Where("id = ?", sole.Id).First(&stillEnabledSole).Error; err != nil {
		t.Fatalf("load sole Role after guard: %v", err)
	}
	if stillEnabledSole.Status != 1 {
		t.Fatalf("sole Role status changed after guard: %d", stillEnabledSole.Status)
	}
	if err := base.DB().Exec("DELETE FROM sys_user_role WHERE user_id = ?", 1).Error; err != nil {
		t.Fatalf("restore admin role bindings: %v", err)
	}
	for _, roleID := range originalAdminRoleIDs {
		if err := base.DB().Clauses(clause.OnConflict{DoNothing: true}).Create(&model.SysUserRole{UserID: 1, RoleID: roleID}).Error; err != nil {
			t.Fatalf("restore admin role binding %d: %v", roleID, err)
		}
	}

	role := createIntegrationRole(t, handler, "资源测试角色", "resource-role")
	if _, err := handler.BatchDeleteRoles(ctx, &adminv1.BatchDeleteRolesRequest{Ids: []int64{role.Id, superAdmin.ID, role.Id}}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("BatchDeleteRoles containing super-admin code = %v err=%v, want FailedPrecondition", connect.CodeOf(err), err)
	}
	var protectedBatchRole model.SysRole
	if err := base.DB().Where("id = ? AND del_flag = 0", role.Id).First(&protectedBatchRole).Error; err != nil {
		t.Fatalf("batch super-admin guard changed ordinary Role: %v", err)
	}
	if len(role.MenuIds) != 0 {
		t.Fatalf("standard CreateRole unexpectedly projected menu_ids: %v", role.MenuIds)
	}

	list, err := handler.ListRoles(ctx, &adminv1.ListRolesRequest{
		Page:    &adminv1.PageRequest{PageNum: 1, PageSize: 10},
		Keyword: "resource-role",
	})
	if err != nil {
		t.Fatalf("ListRoles keyword: %v", err)
	}
	if list.Page == nil || list.Page.Total != 1 || len(list.Items) != 1 || list.Items[0].Id != role.Id {
		t.Fatalf("ListRoles keyword = items=%v page=%v, want one role", list.Items, list.Page)
	}

	menus, err := handler.GetRoleMenus(ctx, &adminv1.GetRoleMenusRequest{RoleId: role.Id})
	if err != nil {
		t.Fatalf("GetRoleMenus initial: %v", err)
	}
	if len(menus.MenuIds) != 0 {
		t.Fatalf("new Role menu IDs = %v, want empty", menus.MenuIds)
	}
	if _, err := handler.UpdateRoleMenus(ctx, &adminv1.UpdateRoleMenusRequest{
		RoleId:  role.Id,
		MenuIds: []int64{100, 1, 100},
	}); err != nil {
		t.Fatalf("UpdateRoleMenus valid relation: %v", err)
	}
	menus, err = handler.GetRoleMenus(ctx, &adminv1.GetRoleMenusRequest{RoleId: role.Id})
	if err != nil {
		t.Fatalf("GetRoleMenus after update: %v", err)
	}
	if got, want := menus.MenuIds, []int64{1, 100}; !equalInt64s(got, want) {
		t.Fatalf("role menu IDs = %v, want deduplicated sorted %v", got, want)
	}
	beforeRelationUpdate := model.SysRole{}
	if err := base.DB().Where("id = ?", role.Id).First(&beforeRelationUpdate).Error; err != nil {
		t.Fatalf("load Role before invalid relation update: %v", err)
	}
	if _, err := handler.UpdateRoleMenus(ctx, &adminv1.UpdateRoleMenusRequest{
		RoleId:  role.Id,
		MenuIds: []int64{1, 999999999},
	}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("invalid menu relation code = %v err=%v, want NotFound", connect.CodeOf(err), err)
	}
	if got, want := roleMenuIDs(t, base, role.Id), []int64{1, 100}; !equalInt64s(got, want) {
		t.Fatalf("invalid menu relation changed existing IDs = %v, want %v", got, want)
	}
	var afterRelationUpdate model.SysRole
	if err := base.DB().Where("id = ?", role.Id).First(&afterRelationUpdate).Error; err != nil {
		t.Fatalf("load Role after invalid relation update: %v", err)
	}
	if !afterRelationUpdate.UpdateTime.Equal(beforeRelationUpdate.UpdateTime) {
		t.Fatalf("invalid menu relation changed UpdatedAt: before=%s after=%s", beforeRelationUpdate.UpdateTime, afterRelationUpdate.UpdateTime)
	}
	if _, err := handler.UpdateRoleMenus(ctx, &adminv1.UpdateRoleMenusRequest{RoleId: role.Id}); err != nil {
		t.Fatalf("empty UpdateRoleMenus should clear relation: %v", err)
	}
	if got := roleMenuIDs(t, base, role.Id); len(got) != 0 {
		t.Fatalf("empty UpdateRoleMenus left relations: %v", got)
	}
	if _, err := handler.UpdateRoleMenus(ctx, &adminv1.UpdateRoleMenusRequest{RoleId: role.Id, MenuIds: []int64{1}}); err != nil {
		t.Fatalf("set baseline Role relation: %v", err)
	}
	var relationUpdateBefore model.SysRole
	if err := base.DB().Where("id = ?", role.Id).First(&relationUpdateBefore).Error; err != nil {
		t.Fatalf("load Role before forced relation rollback: %v", err)
	}

	testutil.AddConstraint(t, base.DB(), "sys_role_menu", "reject_role_menu_test", fmt.Sprintf("CHECK (role_id <> %d OR menu_id <> 100)", role.Id))
	_, relationErr := handler.UpdateRoleMenus(ctx, &adminv1.UpdateRoleMenusRequest{RoleId: role.Id, MenuIds: []int64{1, 100}})
	if connect.CodeOf(relationErr) != connect.CodeInternal {
		t.Fatalf("forced Role relation rollback code = %v err=%v, want Internal", connect.CodeOf(relationErr), relationErr)
	}
	if err := base.DB().Migrator().DropConstraint(&model.SysRoleMenu{}, "reject_role_menu_test"); err != nil {
		t.Fatal(err)
	}
	if got, want := roleMenuIDs(t, base, role.Id), []int64{1}; !equalInt64s(got, want) {
		t.Fatalf("forced relation failure leaked role_menu changes = %v, want %v", got, want)
	}
	var relationUpdateAfter model.SysRole
	if err := base.DB().Where("id = ?", role.Id).First(&relationUpdateAfter).Error; err != nil {
		t.Fatalf("load Role after forced relation rollback: %v", err)
	}
	if !relationUpdateAfter.UpdateTime.Equal(relationUpdateBefore.UpdateTime) {
		t.Fatalf("forced relation failure changed UpdatedAt: before=%s after=%s", relationUpdateBefore.UpdateTime, relationUpdateAfter.UpdateTime)
	}
	if _, err := handler.UpdateRoleMenus(ctx, &adminv1.UpdateRoleMenusRequest{RoleId: role.Id, MenuIds: []int64{1, 100}}); err != nil {
		t.Fatalf("restore valid Role relations: %v", err)
	}

	before := model.SysRole{}
	if err := base.DB().Where("id = ?", role.Id).First(&before).Error; err != nil {
		t.Fatalf("load Role before selective update: %v", err)
	}
	newName := "资源测试角色-更新"
	newSort := int32(0)
	updated, err := handler.UpdateRole(ctx, &adminv1.UpdateRoleRequest{
		Id:       role.Id,
		RoleName: &newName,
		RoleSort: &newSort,
	})
	if err != nil {
		t.Fatalf("selective UpdateRole: %v", err)
	}
	if updated.Role == nil || updated.Role.RoleName != newName || updated.Role.RoleSort != 0 || updated.Role.RoleKey != before.RoleKey || updated.Role.DataScope != int32(before.DataScope) || updated.Role.Remark != before.Remark {
		t.Fatalf("selective UpdateRole response = %#v, omitted values were not preserved", updated.Role)
	}
	var after model.SysRole
	if err := base.DB().Where("id = ?", role.Id).First(&after).Error; err != nil {
		t.Fatalf("load Role after selective update: %v", err)
	}
	if after.RoleName != newName || after.RoleSort != 0 || after.RoleKey != before.RoleKey || after.DataScope != before.DataScope || !after.UpdateTime.After(before.UpdateTime) {
		t.Fatalf("selective UpdateRole persisted incorrectly: before=%+v after=%+v", before, after)
	}
	unchangedAt := after.UpdateTime
	sameRemark := after.Remark
	same, err := handler.UpdateRole(ctx, &adminv1.UpdateRoleRequest{Id: role.Id, Remark: &sameRemark})
	if err != nil {
		t.Fatalf("same-value Role patch: %v", err)
	}
	if same.Role == nil || !same.Role.UpdateTime.AsTime().Equal(unchangedAt) {
		t.Fatalf("same-value Role patch changed UpdatedAt: got=%v want=%s", same.Role, unchangedAt)
	}
	if _, err := handler.UpdateRole(ctx, &adminv1.UpdateRoleRequest{Id: role.Id}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("empty Role patch code = %v err=%v, want InvalidArgument", connect.CodeOf(err), err)
	}
	invalidScope := int32(6)
	if _, err := handler.UpdateRole(ctx, &adminv1.UpdateRoleRequest{Id: role.Id, DataScope: &invalidScope}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("invalid Role data_scope update code = %v err=%v, want InvalidArgument", connect.CodeOf(err), err)
	}
	if _, err := handler.CreateRole(ctx, &adminv1.CreateRoleRequest{RoleName: "无效范围", RoleKey: "invalid-scope", DataScope: invalidScope}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("invalid Role data_scope create code = %v err=%v, want InvalidArgument", connect.CodeOf(err), err)
	}
	if _, err := handler.CreateRole(ctx, &adminv1.CreateRoleRequest{RoleName: "重复角色", RoleKey: "resource-role"}); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("duplicate Role key code = %v err=%v, want AlreadyExists", connect.CodeOf(err), err)
	}

	beforeStatus := after.UpdateTime
	if _, err := handler.ChangeRoleStatus(ctx, &adminv1.ChangeRoleStatusRequest{Id: role.Id, Status: adminv1.Status_STATUS_DISABLED}); err != nil {
		t.Fatalf("ChangeRoleStatus disabled: %v", err)
	}
	var disabled model.SysRole
	if err := base.DB().Where("id = ?", role.Id).First(&disabled).Error; err != nil {
		t.Fatalf("load disabled Role: %v", err)
	}
	if disabled.Status != 2 || !disabled.UpdateTime.After(beforeStatus) {
		t.Fatalf("ChangeRoleStatus disabled persisted incorrectly: before=%s after=%+v", beforeStatus, disabled)
	}
	if _, err := handler.ChangeRoleStatus(ctx, &adminv1.ChangeRoleStatusRequest{Id: role.Id, Status: adminv1.Status(99)}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("unknown Role status code = %v err=%v, want InvalidArgument", connect.CodeOf(err), err)
	}
	if _, err := handler.ChangeRoleStatus(ctx, &adminv1.ChangeRoleStatusRequest{Id: role.Id, Status: adminv1.Status_STATUS_ENABLED}); err != nil {
		t.Fatalf("ChangeRoleStatus enabled: %v", err)
	}

	rollbackName := "回滚角色原值"
	rollbackRole := createIntegrationRole(t, handler, rollbackName, "rollback-role")
	rollbackHooks := newRoleHooks()
	rollbackHooks.AfterUpdate = func(context.Context, resource.Tx, *model.SysRole, *model.SysRole, repo.ColumnSet) error {
		return errors.New("forced Role update rollback")
	}
	rollbackHooks.AfterDelete = func(context.Context, resource.Tx, *model.SysRole) error {
		return errors.New("forced Role delete rollback")
	}
	rollbackHooks.AfterBatchDelete = func(context.Context, resource.Tx, []*model.SysRole) error {
		return errors.New("forced Role batch rollback")
	}
	rollbackController, err := resourcegen.NewRoleController(resourcegen.RoleControllerDeps{
		Store:      testutil.Store(t, base),
		Hooks:      rollbackHooks,
		Repository: newRoleRepository(),
		Overrides:  &roleOverrides{repo: base},
	})
	if err != nil {
		t.Fatalf("NewRoleController rollback fixture: %v", err)
	}
	rollbackNameChanged := "回滚角色不应提交"
	if _, err := rollbackController.UpdateRole(ctx, &adminv1.UpdateRoleRequest{Id: rollbackRole.Id, RoleName: &rollbackNameChanged}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("forced Role update rollback code = %v err=%v, want Internal", connect.CodeOf(err), err)
	}
	var rollbackStored model.SysRole
	if err := base.DB().Where("id = ?", rollbackRole.Id).First(&rollbackStored).Error; err != nil {
		t.Fatalf("load Role after update rollback: %v", err)
	}
	if rollbackStored.RoleName != rollbackName {
		t.Fatalf("Role update rollback leaked name %q, want %q", rollbackStored.RoleName, rollbackName)
	}
	if err := base.DB().Exec("INSERT INTO sys_role_menu (role_id, menu_id) VALUES (?, ?)", rollbackRole.Id, 1).Error; err != nil {
		t.Fatalf("seed rollback role_menu: %v", err)
	}
	if _, err := rollbackController.DeleteRole(ctx, &adminv1.DeleteRoleRequest{Id: rollbackRole.Id}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("forced Role delete rollback code = %v err=%v, want Internal", connect.CodeOf(err), err)
	}
	if err := base.DB().Where("id = ? AND del_flag = 0", rollbackRole.Id).First(&rollbackStored).Error; err != nil {
		t.Fatalf("Role delete rollback did not restore active row: %v", err)
	}
	if got := roleMenuIDs(t, base, rollbackRole.Id); !equalInt64s(got, []int64{1}) {
		t.Fatalf("Role delete rollback removed relation: %v", got)
	}

	batchOne := createIntegrationRole(t, handler, "批量角色一", "batch-role-one")
	batchTwo := createIntegrationRole(t, handler, "批量角色二", "batch-role-two")
	if err := base.DB().Exec("INSERT INTO sys_role_menu (role_id, menu_id) VALUES (?, ?), (?, ?)", batchOne.Id, 1, batchTwo.Id, 100).Error; err != nil {
		t.Fatalf("seed batch role_menu: %v", err)
	}
	if _, err := rollbackController.BatchDeleteRoles(ctx, &adminv1.BatchDeleteRolesRequest{Ids: []int64{batchOne.Id, batchTwo.Id}}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("forced Role batch rollback code = %v err=%v, want Internal", connect.CodeOf(err), err)
	}
	var activeBatch int64
	if err := base.DB().Model(&model.SysRole{}).Where("id IN ? AND del_flag = 0", []int64{batchOne.Id, batchTwo.Id}).Count(&activeBatch).Error; err != nil {
		t.Fatalf("count Role batch after rollback: %v", err)
	}
	if activeBatch != 2 {
		t.Fatalf("Role batch rollback active count = %d, want 2", activeBatch)
	}
	if err := base.DB().Exec("INSERT INTO sys_user_role (user_id, role_id) VALUES (?, ?)", 1, rollbackRole.Id).Error; err != nil {
		t.Fatalf("seed rollback user_role: %v", err)
	}
	if err := base.DB().Exec("INSERT INTO sys_role_dept (role_id, dept_id) VALUES (?, ?)", rollbackRole.Id, 1).Error; err != nil {
		t.Fatalf("seed rollback role_dept: %v", err)
	}

	if _, err := handler.DeleteRole(ctx, &adminv1.DeleteRoleRequest{Id: rollbackRole.Id}); err != nil {
		t.Fatalf("DeleteRole: %v", err)
	}
	var deleted model.SysRole
	if err := base.DB().Where("id = ?", rollbackRole.Id).First(&deleted).Error; err != nil {
		t.Fatalf("load soft-deleted Role: %v", err)
	}
	var userRoleCount, roleDeptCount int64
	if err := base.DB().Table("sys_user_role").Where("role_id = ?", rollbackRole.Id).Count(&userRoleCount).Error; err != nil {
		t.Fatalf("count deleted Role user relations: %v", err)
	}
	if err := base.DB().Table("sys_role_dept").Where("role_id = ?", rollbackRole.Id).Count(&roleDeptCount).Error; err != nil {
		t.Fatalf("count deleted Role dept relations: %v", err)
	}
	if deleted.DelFlag != 1 || len(roleMenuIDs(t, base, rollbackRole.Id)) != 0 || userRoleCount != 0 || roleDeptCount != 0 {
		t.Fatalf("DeleteRole did not soft-delete/clear relations: row=%+v menus=%v user_roles=%d role_depts=%d", deleted, roleMenuIDs(t, base, rollbackRole.Id), userRoleCount, roleDeptCount)
	}
	if _, err := handler.GetRole(ctx, &adminv1.GetRoleRequest{Id: rollbackRole.Id}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("Get deleted Role code = %v err=%v, want NotFound", connect.CodeOf(err), err)
	}
	if _, err := handler.BatchDeleteRoles(ctx, &adminv1.BatchDeleteRolesRequest{Ids: []int64{batchOne.Id, batchTwo.Id, batchOne.Id}}); err != nil {
		t.Fatalf("BatchDeleteRoles: %v", err)
	}
	var deletedBatch []model.SysRole
	if err := base.DB().Where("id IN ?", []int64{batchOne.Id, batchTwo.Id}).Find(&deletedBatch).Error; err != nil {
		t.Fatalf("load batch-deleted Roles: %v", err)
	}
	if len(deletedBatch) != 2 || deletedBatch[0].DelFlag != 1 || deletedBatch[1].DelFlag != 1 {
		t.Fatalf("BatchDeleteRoles rows = %#v, want two soft-deleted rows", deletedBatch)
	}
	if got := roleMenuIDs(t, base, batchOne.Id); len(got) != 0 {
		t.Fatalf("BatchDeleteRoles left batchOne relations: %v", got)
	}
	if _, err := handler.DeleteRole(ctx, &adminv1.DeleteRoleRequest{Id: 999999999}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("Delete missing Role code = %v err=%v, want NotFound", connect.CodeOf(err), err)
	}
	if _, err := handler.BatchDeleteRoles(ctx, &adminv1.BatchDeleteRolesRequest{Ids: []int64{999999999}}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("BatchDelete missing Role code = %v err=%v, want NotFound", connect.CodeOf(err), err)
	}

	status := adminv1.Status_STATUS_ENABLED
	filtered, err := handler.ListRoles(ctx, &adminv1.ListRolesRequest{
		Page:   &adminv1.PageRequest{PageNum: 1, PageSize: 100},
		Status: &status,
	})
	if err != nil {
		t.Fatalf("ListRoles status filter: %v", err)
	}
	for _, item := range filtered.Items {
		if item.Status != adminv1.Status_STATUS_ENABLED {
			t.Fatalf("Role status filter returned %v: %#v", item.Status, item)
		}
	}

	if err := base.DB().Model(&model.SysRole{}).Where("id = ?", role.Id).Update("status", 0).Error; err != nil {
		t.Fatalf("seed explicit-zero ListRoles status: %v", err)
	}
	unspecified := adminv1.Status_STATUS_UNSPECIFIED
	zeroFiltered, err := handler.ListRoles(ctx, &adminv1.ListRolesRequest{
		Page: &adminv1.PageRequest{PageNum: 1, PageSize: 100}, Keyword: "resource-role", Status: &unspecified,
	})
	if err != nil || zeroFiltered.Page == nil || zeroFiltered.Page.Total != 1 || len(zeroFiltered.Items) != 1 || zeroFiltered.Items[0].Id != role.Id {
		t.Fatalf("ListRoles explicit STATUS_UNSPECIFIED err=%v items=%v page=%v, want status=0 row", err, zeroFiltered.Items, zeroFiltered.Page)
	}

	// A malformed persisted enum must not be silently narrowed to a valid wire value.
	if err := base.DB().Model(&model.SysRole{}).Where("id = ?", role.Id).Update("status", 99).Error; err != nil {
		t.Fatalf("seed invalid Role status: %v", err)
	}
	if _, err := handler.GetRole(ctx, &adminv1.GetRoleRequest{Id: role.Id}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("unknown Role status Get code = %v err=%v, want Internal", connect.CodeOf(err), err)
	}

	// Keep this assertion in the integration gate so a future implementation
	// cannot accidentally move relation writes outside the transaction.
	if afterRelationUpdate.UpdateTime.Before(beforeRelationUpdate.UpdateTime) {
		t.Fatalf("unexpected clock regression: before=%s after=%s", beforeRelationUpdate.UpdateTime, afterRelationUpdate.UpdateTime)
	}
}
