package coredefs

import (
	"testing"

	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestUserResourceKeepsSecurityWorkflowsOutsideGeneratedScalarCRUD(t *testing.T) {
	if UserResource == nil {
		t.Fatal("User resource is nil")
	}
	if UserResource.Mode != schema.Partial {
		t.Fatalf("User mode = %v, want Partial", UserResource.Mode)
	}
	if UserResource.Repository.Implementation != schema.RepositoryInjected {
		t.Fatalf("User repository implementation = %v, want RepositoryInjected", UserResource.Repository.Implementation)
	}
	if !UserResource.Auth.DataScope || UserResource.Auth.DeptField != "DeptID" || UserResource.Auth.CreatorField != "CreateBy" {
		t.Fatalf("User DataScope metadata = %+v, want DeptID/CreateBy scope", UserResource.Auth)
	}
	if len(UserResource.Ignore) != 2 || UserResource.Ignore[0] != "password" || UserResource.Ignore[1] != "role_ids" {
		t.Fatalf("User request ignore boundary = %v", UserResource.Ignore)
	}
	if len(UserResource.EntityIgnore) != 1 || UserResource.EntityIgnore[0] != "role_ids" {
		t.Fatalf("User entity relation boundary = %v", UserResource.EntityIgnore)
	}
	standard := map[schema.OperationKind]schema.Operation{}
	custom := map[string]schema.Operation{}
	for _, operation := range UserResource.Operations {
		if operation.Kind == schema.OperationCustom {
			custom[operation.Method] = operation
		} else {
			standard[operation.Kind] = operation
		}
	}
	if _, ok := standard[schema.OperationList]; !ok {
		t.Fatal("User generated List operation missing")
	}
	if _, ok := standard[schema.OperationGet]; ok {
		t.Fatal("User detail Get must remain dedicated while it projects role_ids for the current page")
	}
	for _, method := range []string{"GetUser", "CreateUser", "UpdateUser", "DeleteUser", "BatchDeleteUsers", "ChangeUserStatus", "ResetPassword"} {
		operation, ok := custom[method]
		if !ok || operation.Implementation != schema.OperationOverride || operation.ScopePolicy != schema.ScopePolicyExempt || operation.Action == "" {
			t.Fatalf("User custom operation %s = %+v", method, operation)
		}
	}
	fields := map[string]schema.Field{}
	for _, field := range UserResource.Fields {
		fields[field.Name] = field
	}
	for _, name := range []string{"ID", "Username", "Nickname", "Email", "Phone", "Status", "DeptID", "Remark", "CreateTime", "UpdateTime"} {
		if _, ok := fields[name]; !ok {
			t.Fatalf("User field %s missing", name)
		}
	}
	if !fields["Username"].Required || !fields["Username"].Searchable || fields["Username"].Lifecycle.EffUpdatable() {
		t.Fatalf("UserName metadata = %+v, want required searchable CreateOnly", fields["Username"])
	}
	if !fields["Status"].ReadOnlyFlag || !fields["Status"].Filterable {
		t.Fatalf("User status metadata = %+v", fields["Status"])
	}
	if fields["Password"].Name != "" || fields["RoleIds"].Name != "" {
		t.Fatal("password/role_ids must not be declared as User Resource fields")
	}
}
