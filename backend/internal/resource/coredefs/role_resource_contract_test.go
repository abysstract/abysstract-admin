package coredefs

import (
	"testing"

	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestRoleResourceDeclaresStandardCRUDAndAuthorizationBoundary(t *testing.T) {
	if RoleResource == nil {
		t.Fatal("Role resource is nil")
	}
	if RoleResource.Mode != schema.Partial {
		t.Fatalf("Role mode = %v, want Partial for authorization/status CUSTOM operations", RoleResource.Mode)
	}
	if RoleResource.Repository.Implementation != schema.RepositoryInjected {
		t.Fatalf("Role repository implementation = %v, want RepositoryInjected", RoleResource.Repository.Implementation)
	}
	if RoleResource.PermPrefix != "system:role" {
		t.Fatalf("Role permission prefix = %q, want system:role", RoleResource.PermPrefix)
	}
	if RoleResource.Auth.DataScope {
		t.Fatal("Role Resource itself must not apply row DataScope")
	}
	standard := map[schema.OperationKind]bool{}
	custom := map[string]schema.Operation{}
	for _, op := range RoleResource.Operations {
		if op.Kind == schema.OperationCustom {
			custom[op.Method] = op
		} else {
			standard[op.Kind] = true
		}
	}
	for _, kind := range []schema.OperationKind{schema.OperationList, schema.OperationCreate, schema.OperationGet, schema.OperationUpdate, schema.OperationDelete, schema.OperationBatchDelete} {
		if !standard[kind] {
			t.Fatalf("Role missing standard operation kind %v", kind)
		}
	}
	for _, method := range []string{"ChangeRoleStatus", "GetRoleMenus", "UpdateRoleMenus"} {
		op, ok := custom[method]
		if !ok || op.Implementation != schema.OperationOverride || op.ScopePolicy != schema.ScopePolicyExempt || op.Action == "" {
			t.Fatalf("Role custom operation %s = %+v", method, op)
		}
	}
	fields := make(map[string]schema.Field, len(RoleResource.Fields))
	for _, field := range RoleResource.Fields {
		fields[field.Name] = field
	}
	for _, name := range []string{"ID", "RoleName", "RoleKey", "RoleSort", "DataScope", "Status", "Remark", "CreateTime", "UpdateTime"} {
		if _, ok := fields[name]; !ok {
			t.Fatalf("Role field %s missing", name)
		}
	}
	if !fields["RoleName"].Required || !fields["RoleName"].Searchable || fields["RoleName"].MaxLen != 64 {
		t.Fatalf("RoleName metadata = %+v", fields["RoleName"])
	}
	if !fields["RoleKey"].Required || !fields["RoleKey"].Lifecycle.EffCreatable() || fields["RoleKey"].Lifecycle.EffUpdatable() {
		t.Fatalf("RoleKey must be required CreateOnly: %+v", fields["RoleKey"])
	}
	if !fields["Status"].ReadOnlyFlag || !fields["Status"].Filterable {
		t.Fatalf("Role status metadata = %+v", fields["Status"])
	}
	if len(RoleResource.Ignore) != 0 || len(RoleResource.EntityIgnore) != 1 || RoleResource.EntityIgnore[0] != "menu_ids" {
		t.Fatalf("Role relation projection boundary = request=%v entity=%v", RoleResource.Ignore, RoleResource.EntityIgnore)
	}
}
