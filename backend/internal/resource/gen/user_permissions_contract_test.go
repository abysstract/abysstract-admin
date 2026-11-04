package gen

import (
	"testing"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
)

func TestGeneratedUserPermissionsCoverReadAndDedicatedWorkflowProcedures(t *testing.T) {
	registry := interceptor.NewPermRegistry()
	if err := RegisterUserPermissions(registry); err != nil {
		t.Fatalf("RegisterUserPermissions: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	want := map[string]string{
		adminv1connect.UserServiceListUsersProcedure:        "system:user:list",
		adminv1connect.UserServiceGetUserProcedure:          "system:user:query",
		adminv1connect.UserServiceCreateUserProcedure:       "system:user:add",
		adminv1connect.UserServiceUpdateUserProcedure:       "system:user:edit",
		adminv1connect.UserServiceDeleteUserProcedure:       "system:user:remove",
		adminv1connect.UserServiceBatchDeleteUsersProcedure: "system:user:remove",
		adminv1connect.UserServiceChangeUserStatusProcedure: "system:user:edit",
		adminv1connect.UserServiceResetPasswordProcedure:    "system:user:resetPwd",
	}
	got := registry.Snapshot().RequiredPermissions
	if len(got) != len(want) {
		t.Fatalf("generated User permission count = %d, want %d: %#v", len(got), len(want), got)
	}
	for procedure, permission := range want {
		if got[procedure] != permission {
			t.Errorf("generated User permission %s = %q, want %q", procedure, got[procedure], permission)
		}
	}
}
