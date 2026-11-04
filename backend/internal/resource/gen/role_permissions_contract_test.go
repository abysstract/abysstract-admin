package gen

import (
	"testing"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
)

func TestGeneratedRolePermissionsCoverStandardAndCustomProcedures(t *testing.T) {
	registry := interceptor.NewPermRegistry()
	if err := RegisterRolePermissions(registry); err != nil {
		t.Fatalf("RegisterRolePermissions: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	want := map[string]string{
		adminv1connect.RoleServiceListRolesProcedure:        "system:role:list",
		adminv1connect.RoleServiceCreateRoleProcedure:       "system:role:add",
		adminv1connect.RoleServiceGetRoleProcedure:          "system:role:query",
		adminv1connect.RoleServiceUpdateRoleProcedure:       "system:role:edit",
		adminv1connect.RoleServiceDeleteRoleProcedure:       "system:role:remove",
		adminv1connect.RoleServiceBatchDeleteRolesProcedure: "system:role:remove",
		adminv1connect.RoleServiceChangeRoleStatusProcedure: "system:role:edit",
		adminv1connect.RoleServiceGetRoleMenusProcedure:     "system:role:list",
		adminv1connect.RoleServiceUpdateRoleMenusProcedure:  "system:role:edit",
	}
	got := registry.Snapshot().RequiredPermissions
	if len(got) != len(want) {
		t.Fatalf("generated Role permission count = %d, want %d: %#v", len(got), len(want), got)
	}
	for procedure, permission := range want {
		if got[procedure] != permission {
			t.Errorf("generated Role permission %s = %q, want %q", procedure, got[procedure], permission)
		}
	}
}
