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

func TestGeneratedRolePermissionDenyFailsClosedBeforeHandler(t *testing.T) {
	registry := interceptor.NewPermRegistry()
	if err := resourcegen.RegisterRolePermissions(registry); err != nil {
		t.Fatalf("RegisterRolePermissions: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	permission := interceptor.NewPermissionInterceptor(registry, func(context.Context, int64) ([]string, error) {
		return nil, nil
	})
	ctx := interceptor.WithClaims(context.Background(), &jwt.Claims{UserID: 1, Username: "role-test"})
	for _, procedure := range []string{
		adminv1connect.RoleServiceListRolesProcedure,
		adminv1connect.RoleServiceGetRoleProcedure,
		adminv1connect.RoleServiceCreateRoleProcedure,
		adminv1connect.RoleServiceUpdateRoleProcedure,
		adminv1connect.RoleServiceDeleteRoleProcedure,
		adminv1connect.RoleServiceBatchDeleteRolesProcedure,
		adminv1connect.RoleServiceChangeRoleStatusProcedure,
		adminv1connect.RoleServiceGetRoleMenusProcedure,
		adminv1connect.RoleServiceUpdateRoleMenusProcedure,
	} {
		t.Run(procedure, func(t *testing.T) {
			called := false
			next := connect.ServerFunc(func(context.Context, connect.Spec, connect.ServerStream) error {
				called = true
				return nil
			})
			err := permission(next)(ctx, connect.Spec{Procedure: procedure}, nil)
			if connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Fatalf("Role procedure deny code = %v, want PermissionDenied (err=%v)", connect.CodeOf(err), err)
			}
			if called {
				t.Fatal("Role procedure reached handler after permission denial")
			}
		})
	}
}
