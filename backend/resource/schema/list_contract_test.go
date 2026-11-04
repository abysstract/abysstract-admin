package schema

import (
	"reflect"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
)

type listWithSortRequest struct {
	Page    *adminv1.PageRequest `protobuf:"bytes,1,opt,name=page,proto3"`
	Keyword string               `protobuf:"bytes,2,opt,name=keyword,proto3"`
	Orders  []*adminv1.OrderBy   `protobuf:"bytes,3,rep,name=orders,proto3"`
}

type listWithSortResponse struct {
	Items []*adminv1.DictType `protobuf:"bytes,1,rep,name=items,proto3"`
	Page  *adminv1.PageInfo   `protobuf:"bytes,2,opt,name=page,proto3"`
}

func TestListPlanConsumesGeneratedOrderByAndValidatedDBNames(t *testing.T) {
	plan := &Plan{
		EntityType: reflect.TypeOf(adminv1.DictType{}),
		Fields: []*FieldPlan{
			{ModelField: "ID", JSONName: "id", DBName: "id", Decl: F("ID", SortableOpt(), ServerManaged())},
			{ModelField: "DictName", JSONName: "dictName", DBName: "dict_name", Decl: F("DictName", Searchable())},
		},
	}
	op := Operation{
		Kind: OperationList, Method: "List",
		Binding: OperationBinding{List: ListBinding{Page: "Page", Keyword: "Keyword", Sort: "Orders", Items: "Items", PageInfo: "Page"}},
	}
	var errors []string
	list := buildListPlan(op, plan, structFieldsByGoName((*listWithSortRequest)(nil)), structFieldsByGoName((*listWithSortResponse)(nil)), func(format string, args ...any) {
		errors = append(errors, format)
	})
	if len(errors) != 0 {
		t.Fatalf("valid List binding failed: %v", errors)
	}
	if list.SortableDBNames["id"] != "id" || len(list.SearchDBNames) != 1 || list.SearchDBNames[0] != "dict_name" {
		t.Fatalf("List plan did not reuse validated DB names: %+v", list)
	}
}
