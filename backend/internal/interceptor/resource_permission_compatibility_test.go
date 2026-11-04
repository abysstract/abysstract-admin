package interceptor_test

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
)

// These are the legacy procedure permissions for the three resources whose
// permissions moved from RegisterBuiltinPermissions to generated registration.
// ListDeptRows is the wire-compatible replacement for the removed ListDepts RPC.
var expectedMigratedLegacyPermissions = map[string]string{
	adminv1connect.RoleServiceListRolesProcedure:        "system:role:list",
	adminv1connect.RoleServiceGetRoleProcedure:          "system:role:query",
	adminv1connect.RoleServiceCreateRoleProcedure:       "system:role:add",
	adminv1connect.RoleServiceUpdateRoleProcedure:       "system:role:edit",
	adminv1connect.RoleServiceDeleteRoleProcedure:       "system:role:remove",
	adminv1connect.RoleServiceChangeRoleStatusProcedure: "system:role:edit",
	adminv1connect.MenuServiceListMenusProcedure:        "system:menu:list",
	adminv1connect.MenuServiceGetMenuProcedure:          "system:menu:query",
	adminv1connect.MenuServiceCreateMenuProcedure:       "system:menu:add",
	adminv1connect.MenuServiceUpdateMenuProcedure:       "system:menu:edit",
	adminv1connect.MenuServiceDeleteMenuProcedure:       "system:menu:remove",
	adminv1connect.DeptServiceListDeptRowsProcedure:     "system:dept:list",
	adminv1connect.DeptServiceGetDeptProcedure:          "system:dept:query",
	adminv1connect.DeptServiceCreateDeptProcedure:       "system:dept:add",
	adminv1connect.DeptServiceUpdateDeptProcedure:       "system:dept:edit",
	adminv1connect.DeptServiceDeleteDeptProcedure:       "system:dept:remove",
}

func registerMigratedResourcePermissions(t *testing.T) *interceptor.PermRegistry {
	t.Helper()
	registry := interceptor.NewPermRegistry()
	if err := resourcegen.RegisterDeptPermissions(registry); err != nil {
		t.Fatalf("RegisterDeptPermissions: %v", err)
	}
	if err := resourcegen.RegisterMenuPermissions(registry); err != nil {
		t.Fatalf("RegisterMenuPermissions: %v", err)
	}
	if err := resourcegen.RegisterRolePermissions(registry); err != nil {
		t.Fatalf("RegisterRolePermissions: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	return registry
}

// Feature: ai-native-resource, Requirement 11.1b: generated migration
// registrations preserve every legacy Role/Menu/Dept permission mapping.
func TestGeneratedMigratedResourcePermissionsPreserveLegacyMappings(t *testing.T) {
	registry := registerMigratedResourcePermissions(t)
	for procedure, want := range expectedMigratedLegacyPermissions {
		if got, ok := registry.RequiredPermission(procedure); !ok || got != want {
			t.Errorf("procedure %q permission = (%q, %v), want (%q, true)", procedure, got, ok, want)
		}
	}
}

// Feature: ai-native-resource, Requirement 11.1b: permission decisions remain
// compatible, not merely the strings in the generated registry.
func TestGeneratedGetPermissionsRejectListPermissionAndAllowLegacyQuery(t *testing.T) {
	registry := registerMigratedResourcePermissions(t)
	ctx := interceptor.WithClaims(context.Background(), &jwt.Claims{UserID: 1, Username: "permission-compat"})

	getCases := []struct {
		name      string
		procedure string
		query     string
		list      string
	}{
		{
			name:      "role",
			procedure: adminv1connect.RoleServiceGetRoleProcedure,
			query:     "system:role:query",
			list:      "system:role:list",
		},
		{
			name:      "menu",
			procedure: adminv1connect.MenuServiceGetMenuProcedure,
			query:     "system:menu:query",
			list:      "system:menu:list",
		},
		{
			name:      "dept",
			procedure: adminv1connect.DeptServiceGetDeptProcedure,
			query:     "system:dept:query",
			list:      "system:dept:list",
		},
	}
	for _, tc := range getCases {
		t.Run(tc.name+"/legacy-query-allows", func(t *testing.T) {
			called := false
			next := connect.ServerFunc(func(context.Context, connect.Spec, connect.ServerStream) error {
				called = true
				return nil
			})
			loader := interceptor.NewPermissionInterceptor(registry, func(_ context.Context, _ int64) ([]string, error) {
				return []string{tc.query}, nil
			})
			if err := loader(next)(ctx, connect.Spec{Procedure: tc.procedure}, nil); err != nil {
				t.Fatalf("legacy query permission rejected Get: %v", err)
			}
			if !called {
				t.Fatal("legacy query permission did not reach handler")
			}
		})
		t.Run(tc.name+"/list-does-not-allow", func(t *testing.T) {
			called := false
			next := connect.ServerFunc(func(context.Context, connect.Spec, connect.ServerStream) error {
				called = true
				return nil
			})
			loader := interceptor.NewPermissionInterceptor(registry, func(_ context.Context, _ int64) ([]string, error) {
				return []string{tc.list}, nil
			})
			err := loader(next)(ctx, connect.Spec{Procedure: tc.procedure}, nil)
			if connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Fatalf("list permission code = %v, want PermissionDenied; err=%v", connect.CodeOf(err), err)
			}
			if called {
				t.Fatal("list permission reached Get handler")
			}
		})
	}
}
