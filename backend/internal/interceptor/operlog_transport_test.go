package interceptor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connecthttp"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/jwt"
)

func TestOperLogThroughHTTP(t *testing.T) {
	for _, tc := range []struct {
		name                                     string
		trustProxy, denied, oversized, anonymous bool
		code                                     connect.Code
	}{
		{name: "direct peer"},
		{name: "trusted proxy", trustProxy: true},
		{name: "business error", denied: true, code: connect.CodePermissionDenied},
		{name: "oversized body", oversized: true, code: connect.CodeResourceExhausted},
		{name: "anonymous", anonymous: true, code: connect.CodeUnauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			procedure := adminv1connect.UserServiceCreateUserProcedure
			registry := NewPermRegistry()
			if err := registry.RegisterGenerated(PermissionEntry{Procedure: procedure, Permission: AuthSentinel}); err != nil {
				t.Fatal(err)
			}
			if err := registry.Freeze(); err != nil {
				t.Fatal(err)
			}
			manager := jwt.NewManager("audit-transport-test", time.Hour)
			token, _, err := manager.Generate(7, "auditor", 0)
			if err != nil {
				t.Fatal(err)
			}
			var entries []OperLogEntry
			server := connect.NewServer(
				NewAuthInterceptor(registry, manager, nil, nil),
				NewPermissionInterceptor(registry, nil),
				NewOperLogInterceptor(func(_ context.Context, entry OperLogEntry) { entries = append(entries, entry) }, tc.trustProxy),
			)
			server.Register(connect.Method{
				Spec: connect.Spec{Procedure: procedure, Schema: adminv1.File_abysstract_admin_v1_user_proto.Services().ByName("UserService").Methods().ByName("CreateUser")},
				Handler: func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
					var req adminv1.CreateUserRequest
					if err := stream.Receive(&req); err != nil {
						return err
					}
					if tc.denied {
						return connect.NewError(connect.CodePermissionDenied, "业务拒绝")
					}
					return stream.Send(&adminv1.CreateUserResponse{})
				},
			})
			mux := http.NewServeMux()
			connecthttp.Mount(mux, server, connecthttp.WithReadMaxBytes(1024))
			httpServer := httptest.NewServer(mux)
			defer httpServer.Close()
			client := adminv1connect.NewUserServiceClient(connect.NewClient(connecthttp.NewTransport(httpServer.Client(), httpServer.URL)))
			ctx, info := connect.NewClientContext(context.Background())
			if !tc.anonymous {
				info.RequestHeader().Set("Authorization", "Bearer "+token)
			}
			info.RequestHeader().Set("X-Forwarded-For", "203.0.113.9, 198.51.100.2")
			req := &adminv1.CreateUserRequest{Username: "audit-user", Password: "sensitive-value"}
			if tc.oversized {
				req.Username = strings.Repeat("x", 2048)
			}
			_, err = client.CreateUser(ctx, req)
			if tc.code == 0 {
				if err != nil {
					t.Fatal(err)
				}
			} else if connect.CodeOf(err) != tc.code {
				t.Fatalf("code=%v, error=%v", connect.CodeOf(err), err)
			}
			if tc.anonymous || tc.oversized {
				if len(entries) != 0 {
					t.Fatalf("undecoded request produced audit entries: %+v", entries)
				}
				return
			}
			if len(entries) != 1 {
				t.Fatalf("audit count=%d", len(entries))
			}
			entry := entries[0]
			wantIP := "127.0.0.1"
			if tc.trustProxy {
				wantIP = "203.0.113.9"
			}
			if entry.OperIP != wantIP || entry.OperID != 7 || entry.OperName != "auditor" || entry.Method != procedure {
				t.Fatalf("audit identity: %+v", entry)
			}
			if !strings.Contains(entry.OperParam, "audit-user") || !strings.Contains(entry.OperParam, "***") || strings.Contains(entry.OperParam, "sensitive-value") {
				t.Fatalf("audit parameters=%s", entry.OperParam)
			}
			if tc.denied {
				if entry.Status != 2 || !strings.Contains(entry.ErrorMsg, "业务拒绝") || !strings.Contains(err.Error(), "业务拒绝") {
					t.Fatalf("audit error: %+v, client error=%v", entry, err)
				}
			} else if entry.Status != 1 {
				t.Fatalf("audit status=%d", entry.Status)
			}
		})
	}
}
