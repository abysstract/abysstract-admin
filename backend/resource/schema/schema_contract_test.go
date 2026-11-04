package schema

import (
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

func TestStandardOperationsExpandsSixGeneratedOperations(t *testing.T) {
	ops := StandardOperations(StandardOperationsConfig{
		List:        StandardOperation{Method: "List", Request: (*adminv1.ListDictTypesRequest)(nil), Response: (*adminv1.ListDictTypesResponse)(nil)},
		Create:      StandardOperation{Method: "Create", Request: (*adminv1.CreateDictTypeRequest)(nil), Response: (*adminv1.CreateDictTypeResponse)(nil)},
		Get:         StandardOperation{Method: "Get", Request: (*adminv1.DeleteDictTypeRequest)(nil), Response: (*adminv1.CreateDictTypeResponse)(nil)},
		Update:      StandardOperation{Method: "Update", Request: (*adminv1.UpdateDictTypeRequest)(nil), Response: (*adminv1.UpdateDictTypeResponse)(nil)},
		Delete:      StandardOperation{Method: "Delete", Request: (*adminv1.DeleteDictTypeRequest)(nil), Response: (*adminv1.DeleteDictTypeResponse)(nil)},
		BatchDelete: StandardOperation{Method: "BatchDelete", Request: (*adminv1.BatchDeleteRequest)(nil), Response: (*adminv1.BatchDeleteResponse)(nil)},
	})
	want := []OperationKind{OperationList, OperationCreate, OperationGet, OperationUpdate, OperationDelete, OperationBatchDelete}
	if len(ops) != len(want) {
		t.Fatalf("StandardOperations() produced %d operations, want %d", len(ops), len(want))
	}
	for i, op := range ops {
		if op.Kind != want[i] || op.Implementation != OperationGenerated {
			t.Fatalf("operation %d = kind %v implementation %v", i, op.Kind, op.Implementation)
		}
	}
	if ops[0].Binding.List.Keyword != "" || ops[0].Binding.List.Sort != "" {
		t.Fatalf("optional Keyword/Sort bindings must not be guessed: %+v", ops[0].Binding.List)
	}
}

func TestRepositoryAndOperationImplementationAreDistinctTypes(t *testing.T) {
	var repository RepositoryImplementation = RepositoryGenerated
	var operation OperationImplementation = OperationGenerated
	if repository != RepositoryGenerated || operation != OperationGenerated {
		t.Fatal("implementation discriminants changed")
	}
}

func TestFieldOptionsKeepFrozenVocabularySemantics(t *testing.T) {
	f := F("Name", Required(), MaxLen(10), Searchable(), Filterable(), SortableOpt(), NotListable(), ProtoName("display_name"), CreateOnly())
	if !f.Required || !f.MaxLenSet || f.MaxLen != 10 || !f.Searchable || !f.Filterable || !f.Sortable || f.Listable {
		t.Fatalf("field options were not applied: %+v", f)
	}
	if f.ProtoName != "display_name" || !f.Lifecycle.EffCreatable() || f.Lifecycle.EffUpdatable() {
		t.Fatalf("field lifecycle/proto name incorrect: %+v", f)
	}

	managed := F("ID", ServerManaged())
	if managed.Lifecycle.EffCreatable() || managed.Lifecycle.EffUpdatable() || !managed.Lifecycle.ServerManaged {
		t.Fatalf("server-managed field must be client-unwritable: %+v", managed)
	}
}
