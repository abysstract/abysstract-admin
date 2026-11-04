package schematest

import (
	"testing"

	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestWidgetFixtureBuildsAllSixOperations(t *testing.T) {
	plan, err := WidgetFixtureResource().Build()
	if err != nil {
		t.Fatalf("widget fixture must build: %v", err)
	}
	for _, kind := range []schema.OperationKind{schema.OperationList, schema.OperationCreate, schema.OperationGet, schema.OperationUpdate, schema.OperationDelete, schema.OperationBatchDelete} {
		if plan.Operation(kind) == nil {
			t.Fatalf("operation %v missing from plan", kind)
		}
	}
	list := plan.Operation(schema.OperationList).List
	if list == nil || list.KeywordField != "Keyword" || list.SortField != "Orders" {
		t.Fatalf("list plan wrong: %+v", list)
	}
	if len(list.Filters) != 1 || list.Filters[0].SchemaField != "Enabled" {
		t.Fatalf("filter plan wrong: %+v", list.Filters)
	}
	if len(list.SearchDBNames) != 1 || list.SearchDBNames[0] != "name" {
		t.Fatalf("search DBNames wrong: %v", list.SearchDBNames)
	}
	if list.SortableDBNames["id"] != "id" {
		t.Fatalf("sortable whitelist wrong: %v", list.SortableDBNames)
	}
}
