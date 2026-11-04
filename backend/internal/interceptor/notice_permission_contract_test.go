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

func TestGeneratedNoticePermissionDenyFailsClosedBeforeHandler(t *testing.T) {
	registry := interceptor.NewPermRegistry()
	if err := resourcegen.RegisterNoticePermissions(registry); err != nil {
		t.Fatalf("RegisterNoticePermissions: %v", err)
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
	ctx := interceptor.WithClaims(context.Background(), &jwt.Claims{UserID: 1, Username: "notice-test"})
	for _, procedure := range []string{
		adminv1connect.NoticeServiceListNoticesProcedure,
		adminv1connect.NoticeServiceGetNoticeProcedure,
		adminv1connect.NoticeServiceCreateNoticeProcedure,
		adminv1connect.NoticeServiceUpdateNoticeProcedure,
		adminv1connect.NoticeServiceDeleteNoticeProcedure,
		adminv1connect.NoticeServiceBatchDeleteNoticesProcedure,
	} {
		called = false
		err := permission(next)(ctx, connect.Spec{Procedure: procedure}, nil)
		if connect.CodeOf(err) != connect.CodePermissionDenied {
			t.Errorf("Notice procedure %s deny code = %v, want PermissionDenied (err=%v)", procedure, connect.CodeOf(err), err)
		}
		if called {
			t.Errorf("Notice procedure %s reached handler after permission denial", procedure)
		}
	}
}
