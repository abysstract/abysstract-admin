package interceptor

import (
	"context"
	"fmt"
	"testing"

	"connectrpc.com/connect/v2"

	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
)

const testProcedure = "/abysstract.admin.v1.TestService/Test"

type mockRequest struct {
	procedure string
}

func (m mockRequest) Spec() connect.Spec { return connect.Spec{Procedure: m.procedure} }

func fakePermLoader(perms []string) PermLoader {
	return func(_ context.Context, _ int64) ([]string, error) {
		return perms, nil
	}
}

func fakePermLoaderErr() PermLoader {
	return func(_ context.Context, _ int64) ([]string, error) {
		return nil, fmt.Errorf("load failed")
	}
}

func withClaims(ctx context.Context) context.Context {
	return context.WithValue(ctx, claimsKey, &jwt.Claims{UserID: 1, Username: "test"})
}

// Feature: open-source-readiness, Property 1: 权限拦截器授权决策
func TestPermissionInterceptorAuthorization(t *testing.T) {
	tests := []struct {
		name      string
		public    bool
		perm      string
		userPerms []string
		loaderErr bool
		wantNext  bool
	}{
		{name: "公开过程放行", public: true, wantNext: true},
		{name: "非公开未声明权限拒绝", wantNext: false},
		{name: "AuthSentinel 已登录放行", perm: AuthSentinel, wantNext: true},
		{name: "用户有权限放行", perm: "system:user:list", userPerms: []string{"system:user:list"}, wantNext: true},
		{name: "通配权限放行", perm: "system:user:list", userPerms: []string{"*:*:*"}, wantNext: true},
		{name: "用户无权限拒绝", perm: "system:user:add", userPerms: []string{"system:user:list"}, wantNext: false},
		{name: "PermLoader 错误拒绝", perm: "system:user:list", loaderErr: true, wantNext: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewPermRegistry()
			if tt.public {
				if err := registry.RegisterPublic(testProcedure); err != nil {
					t.Fatalf("register public procedure: %v", err)
				}
			}
			if tt.perm != "" {
				if err := registry.RegisterGenerated(PermissionEntry{Procedure: testProcedure, Permission: tt.perm}); err != nil {
					t.Fatalf("register permission: %v", err)
				}
			}
			if err := registry.Freeze(); err != nil {
				t.Fatalf("freeze registry: %v", err)
			}

			var loader PermLoader
			if tt.loaderErr {
				loader = fakePermLoaderErr()
			} else {
				loader = fakePermLoader(tt.userPerms)
			}

			interceptor := NewPermissionInterceptor(registry, loader)
			nextCalled := false
			next := connect.ServerFunc(func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
				nextCalled = true
				return nil
			})

			req := &mockRequest{procedure: testProcedure}
			ctx := withClaims(context.Background())
			_ = interceptor(next)(ctx, req.Spec(), nil)

			if nextCalled != tt.wantNext {
				t.Errorf("nextCalled = %v, want %v", nextCalled, tt.wantNext)
			}
		})
	}
}

// Feature: open-source-readiness, Property 1: 基础设施不可用 fail-closed
func TestPermissionInterceptorInfraUnavailable(t *testing.T) {
	interceptor := NewPermissionInterceptor(nil, fakePermLoader(nil))
	nextCalled := false
	next := connect.ServerFunc(func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
		nextCalled = true
		return nil
	})

	req := &mockRequest{procedure: testProcedure}
	_ = interceptor(next)(context.Background(), req.Spec(), nil)

	if nextCalled {
		t.Error("基础设施不可用时不应调用 next")
	}
}
