package service

import (
	"context"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
)

func newRoleIntegrationHandler(t *testing.T, base *repo.Repo) adminv1connect.RoleServiceHandler {
	t.Helper()
	handler, err := NewGeneratedRoleService(base)
	if err != nil {
		t.Fatalf("NewGeneratedRoleService: %v", err)
	}
	return handler
}

func createIntegrationRole(t *testing.T, handler adminv1connect.RoleServiceHandler, name, key string) *adminv1.Role {
	t.Helper()
	response, err := handler.CreateRole(context.Background(), &adminv1.CreateRoleRequest{
		RoleName: name,
		RoleKey:  key,
		RoleSort: 1,
		Remark:   "测试角色",
	})
	if err != nil {
		t.Fatalf("CreateRole(%q): %v", key, err)
	}
	if response.Role == nil || response.Role.Id == 0 {
		t.Fatalf("CreateRole(%q) returned no Role: %v", key, response.Role)
	}
	if response.Role.Status != adminv1.Status_STATUS_ENABLED || response.Role.DataScope != 1 {
		t.Fatalf("CreateRole(%q) defaults = status=%v data_scope=%d, want enabled/1", key, response.Role.Status, response.Role.DataScope)
	}
	return response.Role
}

func roleMenuIDs(t *testing.T, base *repo.Repo, roleID int64) []int64 {
	t.Helper()
	var ids []int64
	if err := base.DB().Raw("SELECT menu_id FROM sys_role_menu WHERE role_id = ? ORDER BY menu_id", roleID).Scan(&ids).Error; err != nil {
		t.Fatalf("load role_menu for role %d: %v", roleID, err)
	}
	return ids
}

func equalInt64s(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
