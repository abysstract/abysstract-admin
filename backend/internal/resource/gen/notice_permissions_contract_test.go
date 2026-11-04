package gen

import (
	"testing"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
)

func TestGeneratedNoticePermissionsPreserveLegacyMappingAndAddBatchDelete(t *testing.T) {
	registry := interceptor.NewPermRegistry()
	if err := RegisterNoticePermissions(registry); err != nil {
		t.Fatalf("RegisterNoticePermissions: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	want := map[string]string{
		adminv1connect.NoticeServiceListNoticesProcedure:        "system:notice:list",
		adminv1connect.NoticeServiceGetNoticeProcedure:          "system:notice:list",
		adminv1connect.NoticeServiceCreateNoticeProcedure:       "system:notice:add",
		adminv1connect.NoticeServiceUpdateNoticeProcedure:       "system:notice:edit",
		adminv1connect.NoticeServiceDeleteNoticeProcedure:       "system:notice:remove",
		adminv1connect.NoticeServiceBatchDeleteNoticesProcedure: "system:notice:remove",
	}
	got := registry.Snapshot().RequiredPermissions
	if len(got) != len(want) {
		t.Fatalf("generated Notice permission count = %d, want %d: %#v", len(got), len(want), got)
	}
	for procedure, permission := range want {
		if got[procedure] != permission {
			t.Errorf("generated Notice permission %s = %q, want %q", procedure, got[procedure], permission)
		}
	}
}
