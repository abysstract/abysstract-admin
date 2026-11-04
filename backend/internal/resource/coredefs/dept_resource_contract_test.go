package coredefs

import (
	"testing"

	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestDeptResourceDeclaresStandardFlatCRUD(t *testing.T) {
	if DeptResource == nil {
		t.Fatal("Dept resource is nil")
	}
	if DeptResource.Mode != schema.Generated {
		t.Fatalf("Dept mode = %v, want Generated for the flat Resource", DeptResource.Mode)
	}
	if DeptResource.Repository.Implementation != schema.RepositoryInjected {
		t.Fatalf("Dept repository implementation = %v, want RepositoryInjected for soft-delete/ancestor state", DeptResource.Repository.Implementation)
	}
	if DeptResource.PermPrefix != "system:dept" {
		t.Fatalf("Dept permission prefix = %q, want system:dept", DeptResource.PermPrefix)
	}
	if DeptResource.Audit.UpdatedAtField != "UpdateTime" {
		t.Fatalf("Dept UpdatedAtField = %q, want UpdateTime", DeptResource.Audit.UpdatedAtField)
	}
	if DeptResource.Auth.DataScope {
		t.Fatal("Dept must not opt into a new DataScope behavior absent from the legacy service")
	}

	standardKinds := map[schema.OperationKind]bool{}
	for _, operation := range DeptResource.Operations {
		standardKinds[operation.Kind] = true
	}
	for _, kind := range []schema.OperationKind{schema.OperationList, schema.OperationCreate, schema.OperationGet, schema.OperationUpdate, schema.OperationDelete, schema.OperationBatchDelete} {
		if !standardKinds[kind] {
			t.Fatalf("Dept missing standard operation kind %v", kind)
		}
	}

	fields := make(map[string]schema.Field, len(DeptResource.Fields))
	for _, field := range DeptResource.Fields {
		fields[field.Name] = field
	}
	for _, name := range []string{"ID", "ParentID", "DeptName", "OrderNum", "Leader", "Phone", "Email", "Status", "CreateTime", "UpdateTime"} {
		if _, ok := fields[name]; !ok {
			t.Fatalf("Dept field %s missing", name)
		}
	}
	if !fields["DeptName"].Required || !fields["DeptName"].Searchable || fields["DeptName"].MaxLen != 64 {
		t.Fatalf("DeptName metadata = %+v, want required/searchable/maxLen 64", fields["DeptName"])
	}
	if !fields["Leader"].Searchable || fields["Leader"].MaxLen != 64 || !fields["Phone"].Searchable || fields["Phone"].MaxLen != 32 || !fields["Email"].Searchable || fields["Email"].MaxLen != 128 {
		t.Fatalf("Dept searchable/maxLen metadata is incomplete: leader=%+v phone=%+v email=%+v", fields["Leader"], fields["Phone"], fields["Email"])
	}
	if !fields["OrderNum"].Sortable || !fields["ParentID"].Filterable || !fields["Status"].Filterable {
		t.Fatalf("Dept list capabilities missing: parent=%+v order=%+v status=%+v", fields["ParentID"], fields["OrderNum"], fields["Status"])
	}
	// Status is read-only because the current wire has no create/update status.
	if !fields["Status"].ReadOnlyFlag {
		t.Fatal("Dept status must remain read-only")
	}
	if len(DeptResource.Ignore) != 0 || len(DeptResource.EntityIgnore) != 1 || DeptResource.EntityIgnore[0] != "children" {
		t.Fatalf("Dept entity projection boundary = request=%v entity=%v", DeptResource.Ignore, DeptResource.EntityIgnore)
	}

}
