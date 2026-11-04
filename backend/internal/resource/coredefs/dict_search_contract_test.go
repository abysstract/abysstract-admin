package coredefs

import (
	"reflect"
	"testing"

	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// DictType's generated List must preserve the existing repository behavior:
// keyword searches cover both the display name and the stable type code.
func TestDictTypeListSearchesNameAndTypeCode(t *testing.T) {
	plan, err := DictTypeResource.Build()
	if err != nil {
		t.Fatalf("build DictType resource: %v", err)
	}
	list := plan.Operation(schema.OperationList)
	if list == nil || list.List == nil {
		t.Fatal("DictType standard List plan is missing")
	}
	want := []string{"dict_name", "dict_type"}
	if !reflect.DeepEqual(list.List.SearchDBNames, want) {
		t.Fatalf("DictType search columns = %v, want %v", list.List.SearchDBNames, want)
	}
}
