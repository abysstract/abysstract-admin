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

func TestGeneratedUserPermissionDenyFailsClosedBeforeHandler(t *testing.T) {
	registry := interceptor.NewPermRegistry()
	if err := resourcegen.RegisterUserPermissions(registry); err != nil {
		t.Fatalf("RegisterUserPermissions: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	permission := interceptor.NewPermissionInterceptor(registry, func(context.Context, int64) ([]string, error) { return nil, nil })
	ctx := interceptor.WithClaims(context.Background(), &jwt.Claims{UserID: 1, Username: "user-test"})
	for _, procedure := range []string{
		adminv1connect.UserServiceListUsersProcedure,
		adminv1connect.UserServiceGetUserProcedure,
		adminv1connect.UserServiceCreateUserProcedure,
		adminv1connect.UserServiceUpdateUserProcedure,
		adminv1connect.UserServiceDeleteUserProcedure,
		adminv1connect.UserServiceBatchDeleteUsersProcedure,
		adminv1connect.UserServiceChangeUserStatusProcedure,
		adminv1connect.UserServiceResetPasswordProcedure,
	} {
		t.Run(procedure, func(t *testing.T) {
			called := false
			next := connect.ServerFunc(func(context.Context, connect.Spec, connect.ServerStream) error {
				called = true
				return nil
			})
			err := permission(next)(ctx, connect.Spec{Procedure: procedure}, nil)
			if connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Fatalf("User procedure deny code = %v err=%v, want PermissionDenied", connect.CodeOf(err), err)
			}
			if called {
				t.Fatal("User procedure reached handler after permission denial")
			}
		})
	}
}
