package gen

import (
	"testing"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
)

func TestGeneratedDeptPermissionsCoverStandardSixProcedures(t *testing.T) {
	registry := interceptor.NewPermRegistry()
	if err := RegisterDeptPermissions(registry); err != nil {
		t.Fatalf("RegisterDeptPermissions: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	want := map[string]string{
		adminv1connect.DeptServiceListDeptRowsProcedure:     "system:dept:list",
		adminv1connect.DeptServiceCreateDeptProcedure:       "system:dept:add",
		adminv1connect.DeptServiceGetDeptProcedure:          "system:dept:query",
		adminv1connect.DeptServiceUpdateDeptProcedure:       "system:dept:edit",
		adminv1connect.DeptServiceDeleteDeptProcedure:       "system:dept:remove",
		adminv1connect.DeptServiceBatchDeleteDeptsProcedure: "system:dept:remove",
	}
	got := registry.Snapshot().RequiredPermissions
	if len(got) != len(want) {
		t.Fatalf("generated Dept permission count = %d, want %d: %#v", len(got), len(want), got)
	}
	for procedure, permission := range want {
		if got[procedure] != permission {
			t.Errorf("generated Dept permission %s = %q, want %q", procedure, got[procedure], permission)
		}
	}
}
