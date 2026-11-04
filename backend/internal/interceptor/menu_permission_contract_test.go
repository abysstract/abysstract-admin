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

func TestGeneratedMenuPermissionDenyFailsClosedBeforeHandler(t *testing.T) {
	registry := interceptor.NewPermRegistry()
	if err := resourcegen.RegisterMenuPermissions(registry); err != nil {
		t.Fatalf("RegisterMenuPermissions: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	called := false
	next := connect.ServerFunc(func(context.Context, connect.Spec, connect.ServerStream) error {
		called = true
		return nil
	})
	permission := interceptor.NewPermissionInterceptor(registry, func(context.Context, int64) ([]string, error) {
		return nil, nil
	})
	ctx := interceptor.WithClaims(context.Background(), &jwt.Claims{UserID: 1, Username: "menu-test"})
	for _, procedure := range []string{
		adminv1connect.MenuServiceListMenusProcedure,
		adminv1connect.MenuServiceGetMenuProcedure,
		adminv1connect.MenuServiceCreateMenuProcedure,
		adminv1connect.MenuServiceUpdateMenuProcedure,
		adminv1connect.MenuServiceDeleteMenuProcedure,
		adminv1connect.MenuServiceBatchDeleteMenusProcedure,
	} {
		called = false
		err := permission(next)(ctx, connect.Spec{Procedure: procedure}, nil)
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("Menu procedure %s deny code = %v, want PermissionDenied (err=%v)", procedure, connect.CodeOf(err), err)
		}
		if called {
			t.Errorf("Menu procedure %s reached handler after permission denial", procedure)
		}
	}
}
