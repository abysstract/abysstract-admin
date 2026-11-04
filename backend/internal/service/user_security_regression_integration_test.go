//go:build integration

package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

func openUserSecurityIntegration(t *testing.T) (*repo.Repo, func()) {
	t.Helper()
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return repo.New(gdb), func() {}
}

func TestGeneratedUserSecurityBoundaryAndDataScopeAgainstDatabase(t *testing.T) {
	base, closeDB := openUserSecurityIntegration(t)
	defer closeDB()
	handler := newUserSecurityHandler(t, base)
	adminCtx := userClaimsContext(1, "admin")
	ctx := context.Background()

	resourceUser := createSecurityUser(t, handler, adminCtx, "resource", 1, []int64{1})
	if !equalInt64s(resourceUser.RoleIds, []int64{1}) {
		t.Fatalf("dedicated CreateUser role IDs = %v, want [1]", resourceUser.RoleIds)
	}
	_, invalidCreatePhoneErr := handler.CreateUser(adminCtx, &adminv1.CreateUserRequest{
		Username: "u-invalid-phone",
		Password: "GoodPass1",
		Phone:    "1380000000",
		RoleIds:  []int64{1},
	})
	if connect.CodeOf(invalidCreatePhoneErr) != connect.CodeInvalidArgument || !strings.Contains(invalidCreatePhoneErr.Error(), "手机号格式不正确") {
		t.Fatalf("invalid CreateUser phone error = %v, want InvalidArgument 手机号格式不正确", invalidCreatePhoneErr)
	}
	_, duplicateErr := handler.CreateUser(adminCtx, &adminv1.CreateUserRequest{
		Username: "u-resource",
		Password: "GoodPass1",
		Nickname: "重复用户名",
		RoleIds:  []int64{1},
	})
	if connect.CodeOf(duplicateErr) != connect.CodeAlreadyExists || !strings.Contains(duplicateErr.Error(), "用户名已存在，请更换用户名") {
		t.Fatalf("duplicate username error = %v, want friendly AlreadyExists message", duplicateErr)
	}
	invalidUpdatePhone := "1380000000"
	_, invalidUpdatePhoneErr := handler.UpdateUser(adminCtx, &adminv1.UpdateUserRequest{
		Id:      resourceUser.Id,
		Phone:   &invalidUpdatePhone,
		RoleIds: []int64{1},
	})
	if connect.CodeOf(invalidUpdatePhoneErr) != connect.CodeInvalidArgument || !strings.Contains(invalidUpdatePhoneErr.Error(), "手机号格式不正确") {
		t.Fatalf("invalid UpdateUser phone error = %v, want InvalidArgument 手机号格式不正确", invalidUpdatePhoneErr)
	}
	listed, err := handler.ListUsers(adminCtx, &adminv1.ListUsersRequest{
		Page:    &adminv1.PageRequest{PageNum: 1, PageSize: 100},
		Keyword: "u-resource",
	})
	if err != nil {
		t.Fatalf("generated ListUsers: %v", err)
	}
	if listed.Page == nil || listed.Page.Total != 1 || len(listed.Items) != 1 {
		t.Fatalf("generated ListUsers = items=%v page=%v, want one row", listed.Items, listed.Page)
	}
	if len(listed.Items[0].RoleIds) != 0 {
		t.Fatalf("generated User List projected role IDs: %v", listed.Items[0].RoleIds)
	}
	detail, err := handler.GetUser(adminCtx, &adminv1.GetUserRequest{Id: resourceUser.Id})
	if err != nil {
		t.Fatalf("dedicated GetUser: %v", err)
	}
	if !equalInt64s(detail.User.RoleIds, []int64{1}) {
		t.Fatalf("dedicated GetUser role IDs = %v, want [1]", detail.User.RoleIds)
	}
	if _, err := handler.GetUser(ctx, &adminv1.GetUserRequest{Id: resourceUser.Id}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("GetUser without Claims code = %v err=%v, want Unauthenticated", connect.CodeOf(err), err)
	}

	var before model.SysUser
	if err := base.DB().Where("id = ?", resourceUser.Id).First(&before).Error; err != nil {
		t.Fatalf("load User before optional update: %v", err)
	}
	newNickname := "resource-renamed"
	updated, err := handler.UpdateUser(adminCtx, &adminv1.UpdateUserRequest{
		Id:       resourceUser.Id,
		Nickname: &newNickname,
		RoleIds:  []int64{1},
	})
	if err != nil {
		t.Fatalf("dedicated optional UpdateUser: %v", err)
	}
	if updated.User == nil || updated.User.Nickname != newNickname || !equalInt64s(updated.User.RoleIds, []int64{1}) {
		t.Fatalf("optional UpdateUser response = %#v, omitted roles were not preserved", updated.User)
	}
	var after model.SysUser
	if err := base.DB().Where("id = ?", resourceUser.Id).First(&after).Error; err != nil {
		t.Fatalf("load User after optional update: %v", err)
	}
	if after.Nickname != newNickname || after.Password != before.Password || !equalInt64s(roleIDsOfUser(t, base, resourceUser.Id), []int64{1}) {
		t.Fatalf("optional UpdateUser changed protected state: before=%+v after=%+v roles=%v", before, after, roleIDsOfUser(t, base, resourceUser.Id))
	}

	// A regular authenticated user with the user:edit permission must not be
	// able to replace the built-in administrator's profile or role bindings.
	attacker := createSecurityUser(t, handler, adminCtx, "update-builtin", 1, nil)
	attackerCtx := userClaimsContext(attacker.Id, attacker.Username)
	builtinBefore, err := base.GetUser(ctx, builtinAdminID)
	if err != nil {
		t.Fatalf("load builtin admin before protected UpdateUser: %v", err)
	}
	builtinRolesBefore := roleIDsOfUser(t, base, builtinAdminID)
	forgedNickname := "overwritten-builtin"
	forgedDeptID := int64(2)
	if _, err := handler.UpdateUser(attackerCtx, &adminv1.UpdateUserRequest{
		Id:       builtinAdminID,
		Nickname: &forgedNickname,
		DeptId:   &forgedDeptID,
		RoleIds:  nil,
	}); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("ordinary UpdateUser on builtin admin code = %v err=%v, want PermissionDenied", connect.CodeOf(err), err)
	}
	if err := base.UpdateUser(attackerCtx, &model.SysUser{ID: builtinAdminID}, nil); !errors.Is(err, repo.ErrProtectedUserUpdate) {
		t.Fatalf("direct repo UpdateUser on builtin admin err=%v, want ErrProtectedUserUpdate", err)
	}
	builtinAfter, err := base.GetUser(ctx, builtinAdminID)
	if err != nil {
		t.Fatalf("load builtin admin after protected UpdateUser: %v", err)
	}
	if builtinAfter.Nickname != builtinBefore.Nickname || !reflect.DeepEqual(builtinAfter.DeptID, builtinBefore.DeptID) ||
		!equalInt64s(roleIDsOfUser(t, base, builtinAdminID), builtinRolesBefore) {
		t.Fatalf("protected UpdateUser changed builtin admin: before=%+v after=%+v roles_before=%v roles_after=%v",
			builtinBefore, builtinAfter, builtinRolesBefore, roleIDsOfUser(t, base, builtinAdminID))
	}

	denied, err := handler.ListUsers(ctx, &adminv1.ListUsersRequest{Page: &adminv1.PageRequest{PageNum: 1, PageSize: 100}})
	if err != nil {
		t.Fatalf("generated deny-all ListUsers: %v", err)
	}
	if denied.Page == nil || denied.Page.Total != 0 || len(denied.Items) != 0 {
		t.Fatalf("generated deny-all ListUsers = items=%v page=%v, want empty", denied.Items, denied.Page)
	}
	if repo.CanUseGlobalCache(base.DataScopeAccessScope(adminCtx, "dept_id", "create_by")) {
		t.Fatal("User DataScope unexpectedly marked eligible for global cache")
	}

	if err := base.DB().Model(&model.SysUser{}).Where("id = ?", resourceUser.Id).Update("status", 0).Error; err != nil {
		t.Fatalf("seed explicit-zero ListUsers status: %v", err)
	}
	unspecified := adminv1.Status_STATUS_UNSPECIFIED
	zeroFiltered, err := handler.ListUsers(adminCtx, &adminv1.ListUsersRequest{
		Page: &adminv1.PageRequest{PageNum: 1, PageSize: 100}, Keyword: "resource", Status: &unspecified,
	})
	if err != nil || zeroFiltered.Page == nil || zeroFiltered.Page.Total != 1 || len(zeroFiltered.Items) != 1 || zeroFiltered.Items[0].Id != resourceUser.Id {
		t.Fatalf("ListUsers explicit STATUS_UNSPECIFIED err=%v items=%v page=%v, want status=0 row", err, zeroFiltered.Items, zeroFiltered.Page)
	}

	if err := base.DB().Exec("INSERT INTO sys_dept (id, parent_id, ancestors, dept_name, order_num) VALUES (?, ?, ?, ?, ?)", 2, 1, "0,1", "二部", 2).Error; err != nil {
		t.Fatalf("seed second department: %v", err)
	}
	scopeRole := model.SysRole{RoleName: "部门范围角色", RoleKey: "scope-dept-user-test", RoleSort: 10, DataScope: 3, Status: 1, Remark: "security test"}
	if err := base.DB().Create(&scopeRole).Error; err != nil {
		t.Fatal(err)
	}
	scopeRoleID := scopeRole.ID
	scopeUser := createSecurityUser(t, handler, adminCtx, "scope", 1, nil)
	if err := base.DB().Exec("INSERT INTO sys_user_role (user_id, role_id) VALUES (?, ?)", scopeUser.Id, scopeRoleID).Error; err != nil {
		t.Fatalf("bind department scope Role: %v", err)
	}
	deptOneUser := createSecurityUser(t, handler, adminCtx, "dept-one", 1, nil)
	deptTwoUser := createSecurityUser(t, handler, adminCtx, "dept-two", 2, nil)
	scopeCtx := userClaimsContext(scopeUser.Id, "u-scope")
	scoped, err := handler.ListUsers(scopeCtx, &adminv1.ListUsersRequest{Page: &adminv1.PageRequest{PageNum: 1, PageSize: 100}})
	if err != nil {
		t.Fatalf("department-scoped ListUsers: %v", err)
	}
	for _, user := range scoped.Items {
		if user.DeptId != 1 {
			t.Fatalf("department scope returned out-of-scope User: %#v", user)
		}
	}
	if containsUserID(scoped.Items, deptTwoUser.Id) || !containsUserID(scoped.Items, scopeUser.Id) || !containsUserID(scoped.Items, deptOneUser.Id) {
		t.Fatalf("department scope rows = %v, want scope/dept-one and not dept-two", scoped.Items)
	}
	if _, err := handler.GetUser(scopeCtx, &adminv1.GetUserRequest{Id: deptTwoUser.Id}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("out-of-scope GetUser code = %v err=%v, want NotFound", connect.CodeOf(err), err)
	}

	selfUser := createSecurityUser(t, handler, adminCtx, "self", 2, nil)
	if err := base.DB().Model(&model.SysUser{}).Where("id = ?", selfUser.Id).Update("create_by", selfUser.Id).Error; err != nil {
		t.Fatalf("set self User creator: %v", err)
	}
	selfCtx := userClaimsContext(selfUser.Id, "u-self")
	selfRows, err := handler.ListUsers(selfCtx, &adminv1.ListUsersRequest{Page: &adminv1.PageRequest{PageNum: 1, PageSize: 100}})
	if err != nil {
		t.Fatalf("creator-scoped ListUsers: %v", err)
	}
	if len(selfRows.Items) != 1 || selfRows.Items[0].Id != selfUser.Id {
		t.Fatalf("creator scope rows = %v, want only self User", selfRows.Items)
	}

	if _, err := handler.DeleteUser(adminCtx, &adminv1.DeleteUserRequest{Id: 1}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("Delete builtin admin code = %v err=%v, want FailedPrecondition", connect.CodeOf(err), err)
	}
	if _, err := handler.ChangeUserStatus(adminCtx, &adminv1.ChangeUserStatusRequest{Id: 1, Status: adminv1.Status_STATUS_DISABLED}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("ChangeStatus builtin admin code = %v err=%v, want FailedPrecondition", connect.CodeOf(err), err)
	}
	if _, err := handler.DeleteUser(scopeCtx, &adminv1.DeleteUserRequest{Id: scopeUser.Id}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("Delete current User code = %v err=%v, want FailedPrecondition", connect.CodeOf(err), err)
	}
	if _, err := handler.ChangeUserStatus(scopeCtx, &adminv1.ChangeUserStatusRequest{Id: scopeUser.Id, Status: adminv1.Status_STATUS_DISABLED}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("ChangeStatus current User code = %v err=%v, want FailedPrecondition", connect.CodeOf(err), err)
	}
	if _, err := handler.ResetPassword(scopeCtx, &adminv1.ResetPasswordRequest{Id: 1, NewPassword: "NewGood1"}); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("Reset builtin admin by non-admin code = %v err=%v, want PermissionDenied", connect.CodeOf(err), err)
	}

	batchOne := createSecurityUser(t, handler, adminCtx, "batch-one", 1, []int64{1})
	batchTwo := createSecurityUser(t, handler, adminCtx, "batch-two", 1, []int64{1})
	if _, err := handler.BatchDeleteUsers(adminCtx, &adminv1.BatchDeleteUsersRequest{
		Ids: []int64{batchOne.Id, 1},
	}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("BatchDelete builtin admin code = %v err=%v, want FailedPrecondition", connect.CodeOf(err), err)
	}
	if _, err := handler.BatchDeleteUsers(scopeCtx, &adminv1.BatchDeleteUsersRequest{
		Ids: []int64{batchTwo.Id, scopeUser.Id},
	}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("BatchDelete current User code = %v err=%v, want FailedPrecondition", connect.CodeOf(err), err)
	}
	for _, id := range []int64{batchOne.Id, batchTwo.Id} {
		var user model.SysUser
		if err := base.DB().Where("id = ? AND del_flag = 0", id).First(&user).Error; err != nil {
			t.Fatalf("protected User BatchDelete partially deleted %d: %v", id, err)
		}
	}
	if _, err := handler.BatchDeleteUsers(adminCtx, &adminv1.BatchDeleteUsersRequest{
		Ids: []int64{batchOne.Id, 999999999, batchTwo.Id},
	}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("mixed User BatchDelete code = %v err=%v, want NotFound", connect.CodeOf(err), err)
	}
	for _, id := range []int64{batchOne.Id, batchTwo.Id} {
		var user model.SysUser
		if err := base.DB().Where("id = ? AND del_flag = 0", id).First(&user).Error; err != nil {
			t.Fatalf("mixed User BatchDelete partially deleted %d: %v", id, err)
		}
	}
	deleted, err := handler.BatchDeleteUsers(adminCtx, &adminv1.BatchDeleteUsersRequest{
		Ids: []int64{batchOne.Id, batchTwo.Id, batchOne.Id},
	})
	if err != nil || deleted.Deleted != 2 {
		t.Fatalf("typed User BatchDelete = response=%v err=%v, want deleted=2", deleted, err)
	}
	for _, id := range []int64{batchOne.Id, batchTwo.Id} {
		var user model.SysUser
		if err := base.DB().Where("id = ?", id).First(&user).Error; err != nil {
			t.Fatalf("load batch-deleted User %d: %v", id, err)
		}
		if user.DelFlag != 1 {
			t.Fatalf("User %d del_flag = %d, want 1", id, user.DelFlag)
		}
		if got := roleIDsOfUser(t, base, id); len(got) != 0 {
			t.Fatalf("User %d role bindings after BatchDelete = %v, want empty", id, got)
		}
	}

	if err := base.DB().Model(&model.SysUser{}).Where("id = ?", deptOneUser.Id).Update("status", 99).Error; err != nil {
		t.Fatalf("seed invalid User status: %v", err)
	}
	if _, err := handler.ListUsers(adminCtx, &adminv1.ListUsersRequest{Page: &adminv1.PageRequest{PageNum: 1, PageSize: 100}}); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("unknown User status List code = %v err=%v, want Internal", connect.CodeOf(err), err)
	}
}
