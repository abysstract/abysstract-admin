package schema

import (
	"context"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
)

type boolFilterModel struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	Enabled     bool      `gorm:"column:enabled"`
	NarrowValue int16     `gorm:"column:narrow_value"`
	Status      int16     `gorm:"column:status"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (*boolFilterModel) TableName() string { return "bool_filter" }

type boolServiceHandler interface {
	ListBools(context.Context, *testpb.ListBoolsRequest) (*testpb.ListBoolsResponse, error)
}

func boolContractResource(service ServiceConfig, operations []Operation) *Resource {
	return &Resource{
		Name: "BoolFilter", PermPrefix: "test:bool", Mode: Generated, Table: "bool_filter",
		Model: (*boolFilterModel)(nil), Entity: (*testpb.BoolEntity)(nil),
		CreateReq: (*testpb.CreateBoolRequest)(nil), UpdateReq: (*testpb.UpdateBoolRequest)(nil),
		Fields: []Field{
			F("ID", ServerManaged()),
			F("Enabled", Filterable()),
			F("NarrowValue", Filterable(), ReadOnly()),
			F("Status", Filterable(), ReadOnly(), EnumMap(map[string]int16{"BATCH_STATUS_UNSPECIFIED": 0, "BATCH_STATUS_ACTIVE": 1})),
			F("UpdatedAt", ServerManaged()),
		},
		Service: service, Operations: operations,
		Key: KeyConfig{ModelField: "ID"}, Audit: AuditConfig{UpdatedAtField: "UpdatedAt"},
	}
}

func TestBuildReachesBoolFilterPlanThroughFullResourceFlow(t *testing.T) {
	resource := boolContractResource(ServiceConfig{
		Descriptor: testpb.File_testpb_test_proto.Services().ByName("BoolService"),
		Handler:    (*boolServiceHandler)(nil),
	}, []Operation{{
		Kind: OperationList, Method: "ListBools",
		Request: (*testpb.ListBoolsRequest)(nil), Response: (*testpb.ListBoolsResponse)(nil),
		Binding: OperationBinding{List: ListBinding{
			Page: "Page", Filters: map[string]string{"Enabled": "Enabled", "NarrowValue": "NarrowValue", "Status": "Status"}, Items: "Items", PageInfo: "Page",
		}},
		Implementation: OperationGenerated,
	}})

	plan, err := resource.Build()
	if err != nil {
		t.Fatalf("bool resource must build: %v", err)
	}
	field := plan.Field("Enabled")
	if field.ModelToEntity.Kind != ConvDirect || field.CreateToModel.Kind != ConvDirect || field.UpdateToModel.Kind != ConvDirect {
		t.Fatalf("bool field conversion plans are not direct: %+v", field)
	}
	filters := plan.Operation(OperationList).List.Filters
	if len(filters) != 3 || filters[0].ConversionPlan.Kind != ConvDirect || filters[1].ConversionPlan.Kind != ConvNarrow || !filters[1].ConversionPlan.NeedsBounds || filters[2].ConversionPlan.Kind != ConvEnum {
		t.Fatalf("Bool FilterPlans do not retain direct/narrow/enum conversions: %+v", filters)
	}
}
