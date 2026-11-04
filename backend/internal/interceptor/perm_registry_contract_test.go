package interceptor

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"connectrpc.com/connect/v2"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
)

// These expectations define the public procedure and permission contract
// independently from the production registration declarations.
var expectedPublicProcedures = []string{
	adminv1connect.AuthServiceLoginProcedure,
	adminv1connect.AuthServiceRefreshTokenProcedure,
}

var expectedRequiredPermissions = map[string]string{
	adminv1connect.AuthServiceLogoutProcedure:               AuthSentinel,
	adminv1connect.AuthServiceGetUserInfoProcedure:          AuthSentinel,
	adminv1connect.AuthServiceChangePasswordProcedure:       AuthSentinel,
	adminv1connect.DictServiceListDictTypesProcedure:        "system:dict:list",
	adminv1connect.DictServiceCreateDictTypeProcedure:       "system:dict:add",
	adminv1connect.DictServiceGetDictTypeProcedure:          "system:dict:list",
	adminv1connect.DictServiceUpdateDictTypeProcedure:       "system:dict:edit",
	adminv1connect.DictServiceDeleteDictTypeProcedure:       "system:dict:remove",
	adminv1connect.DictServiceBatchDeleteDictTypesProcedure: "system:dict:remove",
	adminv1connect.DictServiceListDictDataProcedure:         "system:dict:list",
	adminv1connect.DictServiceCreateDictDataProcedure:       "system:dict:add",
	adminv1connect.DictServiceGetDictDataProcedure:          "system:dict:list",
	adminv1connect.DictServiceUpdateDictDataProcedure:       "system:dict:edit",
	adminv1connect.DictServiceDeleteDictDataProcedure:       "system:dict:remove",
	adminv1connect.DictServiceBatchDeleteDictDataProcedure:  "system:dict:remove",
	adminv1connect.ConfigServiceListConfigsProcedure:        "system:config:list",
	adminv1connect.ConfigServiceCreateConfigProcedure:       "system:config:add",
	adminv1connect.ConfigServiceGetConfigProcedure:          "system:config:list",
	adminv1connect.ConfigServiceUpdateConfigProcedure:       "system:config:edit",
	adminv1connect.ConfigServiceDeleteConfigProcedure:       "system:config:remove",
	adminv1connect.ConfigServiceBatchDeleteConfigsProcedure: "system:config:remove",
	adminv1connect.LogServiceListOperLogsProcedure:          "system:operlog:list",
	adminv1connect.LogServiceCleanOperLogsProcedure:         "system:operlog:remove",
	adminv1connect.LogServiceListLoginLogsProcedure:         "system:logininfor:list",
	adminv1connect.LogServiceCleanLoginLogsProcedure:        "system:logininfor:remove",
	adminv1connect.UploadServiceUploadFileProcedure:         AuthSentinel,
	adminv1connect.UploadServiceUploadAvatarProcedure:       AuthSentinel,
	adminv1connect.CommonExportServiceBatchDeleteProcedure:  AuthSentinel,
	adminv1connect.CommonExportServiceExportExcelProcedure:  AuthSentinel,
	adminv1connect.CommonExportServiceImportExcelProcedure:  AuthSentinel,
	adminv1connect.ProfileServiceGetProfileProcedure:        AuthSentinel,
	adminv1connect.ProfileServiceUpdateProfileProcedure:     AuthSentinel,
	adminv1connect.DashboardServiceOverviewProcedure:        AuthSentinel,
}

func TestPermRegistryRegisterGeneratedIsIdempotent(t *testing.T) {
	reg := NewPermRegistry()
	entry := PermissionEntry{Procedure: "/test.v1.WidgetService/ListWidgets", Permission: "system:widget:list"}
	if err := reg.RegisterGenerated(entry); err != nil {
		t.Fatalf("first registration: %v", err)
	}
	if err := reg.RegisterGenerated(entry); err != nil {
		t.Fatalf("identical registration must be idempotent: %v", err)
	}

	snapshot := reg.Snapshot()
	if got := snapshot.RequiredPermissions[entry.Procedure]; got != entry.Permission {
		t.Fatalf("required permission = %q, want %q", got, entry.Permission)
	}
	if len(snapshot.RequiredPermissions) != 1 {
		t.Fatalf("required permission count = %d, want 1", len(snapshot.RequiredPermissions))
	}
}

func TestPermRegistryRejectsConflictingGeneratedMapping(t *testing.T) {
	reg := NewPermRegistry()
	if err := reg.RegisterGenerated(PermissionEntry{Procedure: "/test.v1.WidgetService/ListWidgets", Permission: "system:widget:list"}); err != nil {
		t.Fatalf("seed registration: %v", err)
	}
	if err := reg.RegisterGenerated(PermissionEntry{Procedure: "/test.v1.WidgetService/ListWidgets", Permission: "system:widget:read"}); err == nil {
		t.Fatal("conflicting procedure mapping unexpectedly succeeded")
	}
	if got, ok := reg.RequiredPermission("/test.v1.WidgetService/ListWidgets"); !ok || got != "system:widget:list" {
		t.Fatalf("conflict changed existing mapping: (%q, %v)", got, ok)
	}
}

func TestPermRegistryGeneratedCannotOverridePublicProcedure(t *testing.T) {
	reg := NewPermRegistry()
	const procedure = "/test.v1.AuthService/Login"
	if err := reg.RegisterPublic(procedure); err != nil {
		t.Fatalf("register public procedure: %v", err)
	}
	if err := reg.RegisterGenerated(PermissionEntry{Procedure: procedure, Permission: "system:auth:login"}); err == nil {
		t.Fatal("generated mapping unexpectedly overrode public procedure")
	}
	if !reg.IsPublic(procedure) {
		t.Fatal("failed generated registration changed public procedure")
	}
	if _, ok := reg.RequiredPermission(procedure); ok {
		t.Fatal("public procedure acquired a required permission")
	}
}

func TestPermRegistryRegisterGeneratedIsAtomic(t *testing.T) {
	reg := NewPermRegistry()
	const existing = "/test.v1.WidgetService/GetWidget"
	if err := reg.RegisterGenerated(PermissionEntry{Procedure: existing, Permission: "system:widget:list"}); err != nil {
		t.Fatalf("seed registration: %v", err)
	}

	err := reg.RegisterGenerated(
		PermissionEntry{Procedure: "/test.v1.WidgetService/ListWidgets", Permission: "system:widget:list"},
		PermissionEntry{Procedure: existing, Permission: "system:widget:conflict"},
	)
	if err == nil {
		t.Fatal("mixed valid/conflicting registration unexpectedly succeeded")
	}
	if _, ok := reg.RequiredPermission("/test.v1.WidgetService/ListWidgets"); ok {
		t.Fatal("atomic registration leaked an entry before returning its error")
	}
	if got, _ := reg.RequiredPermission(existing); got != "system:widget:list" {
		t.Fatalf("atomic registration changed existing mapping to %q", got)
	}
}

func TestPermRegistryRejectsMutationAfterFreeze(t *testing.T) {
	reg := NewPermRegistry()
	if err := reg.RegisterGenerated(PermissionEntry{Procedure: "/test.v1.WidgetService/ListWidgets", Permission: "system:widget:list"}); err != nil {
		t.Fatalf("seed registration: %v", err)
	}
	if err := reg.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if err := reg.RegisterGenerated(PermissionEntry{Procedure: "/test.v1.WidgetService/GetWidget", Permission: "system:widget:list"}); err == nil {
		t.Fatal("registration after Freeze unexpectedly succeeded")
	}
	if err := reg.RegisterPublic("/test.v1.AuthService/Login"); err == nil {
		t.Fatal("public registration after Freeze unexpectedly succeeded")
	}
}

// Feature: ai-native-resource, Requirement 11.1b: legacy RPC authorization is
// unchanged after moving the three globals into the bootstrap registry.
func TestBuiltinPermissionRegistryPreservesAuthorizationMappings(t *testing.T) {
	reg := NewPermRegistry()
	if err := RegisterBuiltinPermissions(reg); err != nil {
		t.Fatalf("register legacy bootstrap entries: %v", err)
	}
	if err := reg.Freeze(); err != nil {
		t.Fatalf("freeze: %v", err)
	}

	snapshot := reg.Snapshot()
	gotPublic := append([]string(nil), snapshot.PublicProcedures...)
	wantPublic := append([]string(nil), expectedPublicProcedures...)
	sort.Strings(gotPublic)
	sort.Strings(wantPublic)
	if !reflect.DeepEqual(gotPublic, wantPublic) {
		t.Fatalf("public procedures changed\n got: %v\nwant: %v", gotPublic, wantPublic)
	}
	if !reflect.DeepEqual(snapshot.RequiredPermissions, expectedRequiredPermissions) {
		t.Fatalf("required permission mapping changed\n got: %#v\nwant: %#v", snapshot.RequiredPermissions, expectedRequiredPermissions)
	}

	for _, procedure := range wantPublic {
		if !reg.IsPublic(procedure) {
			t.Errorf("legacy public procedure %q is no longer public", procedure)
		}
		if _, ok := reg.RequiredPermission(procedure); ok {
			t.Errorf("legacy public procedure %q unexpectedly requires a permission", procedure)
		}
	}
	for procedure, want := range expectedRequiredPermissions {
		if reg.IsPublic(procedure) {
			t.Errorf("legacy protected procedure %q unexpectedly became public", procedure)
		}
		if got, ok := reg.RequiredPermission(procedure); !ok || got != want {
			t.Errorf("legacy procedure %q permission = (%q, %v), want (%q, true)", procedure, got, ok, want)
		}
	}
}

// Feature: ai-native-resource, Requirement 11.1b: Auth/Permission decisions
// for every legacy procedure are read from the same frozen Registry instance.
func TestBuiltinPermissionRegistryEnforcesAuthorizationDecisions(t *testing.T) {
	registry := NewPermRegistry()
	if err := RegisterBuiltinPermissions(registry); err != nil {
		t.Fatalf("register builtin permissions: %v", err)
	}
	if err := registry.Freeze(); err != nil {
		t.Fatalf("freeze registry: %v", err)
	}

	for _, procedure := range expectedPublicProcedures {
		t.Run("public/"+procedure, func(t *testing.T) {
			nextCalled := false
			interceptor := NewPermissionInterceptor(registry, fakePermLoader(nil))
			next := connect.ServerFunc(func(context.Context, connect.Spec, connect.ServerStream) error {
				nextCalled = true
				return nil
			})
			if err := interceptor(next)(context.Background(), connect.Spec{Procedure: procedure}, nil); err != nil {
				t.Fatalf("public procedure rejected: %v", err)
			}
			if !nextCalled {
				t.Fatal("public procedure did not reach next")
			}
		})
	}

	for procedure, permission := range expectedRequiredPermissions {
		t.Run("protected/allow/"+procedure, func(t *testing.T) {
			nextCalled := false
			interceptor := NewPermissionInterceptor(registry, fakePermLoader([]string{permission}))
			next := connect.ServerFunc(func(context.Context, connect.Spec, connect.ServerStream) error {
				nextCalled = true
				return nil
			})
			if err := interceptor(next)(withClaims(context.Background()), connect.Spec{Procedure: procedure}, nil); err != nil {
				t.Fatalf("legacy permission %q rejected its procedure: %v", permission, err)
			}
			if !nextCalled {
				t.Fatal("allowed protected procedure did not reach next")
			}
		})
		if permission == AuthSentinel {
			continue
		}
		t.Run("protected/deny/"+procedure, func(t *testing.T) {
			nextCalled := false
			interceptor := NewPermissionInterceptor(registry, fakePermLoader(nil))
			next := connect.ServerFunc(func(context.Context, connect.Spec, connect.ServerStream) error {
				nextCalled = true
				return nil
			})
			err := interceptor(next)(withClaims(context.Background()), connect.Spec{Procedure: procedure}, nil)
			if connect.CodeOf(err) != connect.CodePermissionDenied {
				t.Fatalf("missing legacy permission code = %v, want PermissionDenied; err=%v", connect.CodeOf(err), err)
			}
			if nextCalled {
				t.Fatal("denied protected procedure reached next")
			}
		})
	}
}
