package interceptor_test

import (
	"sort"
	"testing"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/interceptor"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
)

var expectedDictProcedurePermissions = map[string]string{
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
}

func TestBuiltinRegistryIncludesDictStandardSixPermissionMappings(t *testing.T) {
	reg := interceptor.NewPermRegistry()
	if err := interceptor.RegisterBuiltinPermissions(reg); err != nil {
		t.Fatalf("register builtin permissions: %v", err)
	}
	for procedure, want := range expectedDictProcedurePermissions {
		if got, ok := reg.RequiredPermission(procedure); !ok || got != want {
			t.Fatalf("builtin dict permission %q = (%q, %v), want (%q, true)", procedure, got, ok, want)
		}
	}
}

func TestGeneratedDictPermissionRegistrationsRemainIdempotentAndCanonical(t *testing.T) {
	reg := interceptor.NewPermRegistry()
	if err := interceptor.RegisterBuiltinPermissions(reg); err != nil {
		t.Fatalf("register builtin permissions: %v", err)
	}
	if err := resourcegen.RegisterDictTypePermissions(reg); err != nil {
		t.Fatalf("register generated DictType permissions: %v", err)
	}
	if err := resourcegen.RegisterDictDataPermissions(reg); err != nil {
		t.Fatalf("register generated DictData permissions: %v", err)
	}

	perms := map[string]struct{}{}
	for procedure, want := range expectedDictProcedurePermissions {
		got, ok := reg.RequiredPermission(procedure)
		if !ok || got != want {
			t.Fatalf("dict permission %q = (%q, %v), want (%q, true)", procedure, got, ok, want)
		}
		perms[got] = struct{}{}
	}
	gotDistinct := make([]string, 0, len(perms))
	for perm := range perms {
		gotDistinct = append(gotDistinct, perm)
	}
	sort.Strings(gotDistinct)
	wantDistinct := []string{"system:dict:add", "system:dict:edit", "system:dict:list", "system:dict:remove"}
	if len(gotDistinct) != len(wantDistinct) {
		t.Fatalf("distinct dict permissions = %v, want %v", gotDistinct, wantDistinct)
	}
	for i := range wantDistinct {
		if gotDistinct[i] != wantDistinct[i] {
			t.Fatalf("distinct dict permissions = %v, want %v", gotDistinct, wantDistinct)
		}
	}
}
