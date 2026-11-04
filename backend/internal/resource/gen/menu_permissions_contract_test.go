package gen

import (
	"testing"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
)

func TestGeneratedMenuPermissionsCoverStandardSixProcedures(t *testing.T) {
	registry := interceptor.NewPermRegistry()
	if err := RegisterMenuPermissions(registry); err != nil {
		t.Fatalf("RegisterMenuPermissions: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	want := map[string]string{
		adminv1connect.MenuServiceListMenusProcedure:        "system:menu:list",
		adminv1connect.MenuServiceCreateMenuProcedure:       "system:menu:add",
		adminv1connect.MenuServiceGetMenuProcedure:          "system:menu:query",
		adminv1connect.MenuServiceUpdateMenuProcedure:       "system:menu:edit",
		adminv1connect.MenuServiceDeleteMenuProcedure:       "system:menu:remove",
		adminv1connect.MenuServiceBatchDeleteMenusProcedure: "system:menu:remove",
	}
	got := registry.Snapshot().RequiredPermissions
	if len(got) != len(want) {
		t.Fatalf("generated Menu permission count = %d, want %d: %#v", len(got), len(want), got)
	}
	for procedure, permission := range want {
		if got[procedure] != permission {
			t.Errorf("generated Menu permission %s = %q, want %q", procedure, got[procedure], permission)
		}
	}
}
