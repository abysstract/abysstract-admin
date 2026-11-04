package schematest

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// These fixture definitions are explicit inputs to generator tests. The system
// generation input is owned by coredefs.Resources.

// WidgetModel 是 Widget fixture 的 GORM model（仅测试，不进正式 migrate）。
type WidgetModel struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	Name      string    `gorm:"column:name"`
	Enabled   bool      `gorm:"column:enabled"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
	// DataScope 列（不进 Entity；EntityIgnore 声明），供 data_scope fixture 使用。
	DeptID    int64 `gorm:"column:dept_id"`
	CreatedBy int64 `gorm:"column:create_by"`
}

func (*WidgetModel) TableName() string { return "test_widget" }

// BoolModel is the single-Resource Generated compile-sample model. It is
// intentionally small, while still exercising a scalar filter and audit field.
type BoolModel struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	Enabled     bool      `gorm:"column:enabled"`
	NarrowValue int16     `gorm:"column:narrow_value"`
	Status      int16     `gorm:"column:status"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (*BoolModel) TableName() string { return "test_bool" }

// BoolServiceHandler mirrors the six-operation fixture service. It is a typed
// handler target so the generated Sample A adapter assertion participates in
// the isolated-package go test.
type BoolServiceHandler interface {
	ListBools(context.Context, *testpb.ListBoolsRequest) (*testpb.ListBoolsResponse, error)
	CreateBool(context.Context, *testpb.CreateBoolRequest) (*testpb.CreateBoolResponse, error)
	GetBool(context.Context, *testpb.GetBoolRequest) (*testpb.GetBoolResponse, error)
	UpdateBool(context.Context, *testpb.UpdateBoolRequest) (*testpb.UpdateBoolResponse, error)
	DeleteBool(context.Context, *testpb.DeleteBoolRequest) (*testpb.DeleteBoolResponse, error)
	BatchDeleteBools(context.Context, *testpb.BatchDeleteBoolsRequest) (*testpb.BatchDeleteBoolsResponse, error)
}

// BoolCompileSampleResource supplies compile samples A/B: one Generated Resource
// with all standard operations. Callers choose default or injected Repository
// wiring in the generated controller deps; the static Schema remains identical.
func BoolCompileSampleResource() *schema.Resource {
	return &schema.Resource{
		Name: "Bool", PermPrefix: "test:bool", Mode: schema.Generated, Table: "test_bool",
		Model: (*BoolModel)(nil), Entity: (*testpb.BoolEntity)(nil),
		CreateReq: (*testpb.CreateBoolRequest)(nil), UpdateReq: (*testpb.UpdateBoolRequest)(nil),
		Fields: []schema.Field{
			schema.F("ID", schema.ServerManaged(), schema.SortableOpt()),
			schema.F("Enabled", schema.Filterable()),
			schema.F("NarrowValue", schema.Filterable(), schema.ReadOnly()),
			schema.F("Status", schema.Filterable(), schema.ReadOnly(), schema.EnumMap(map[string]int16{"BATCH_STATUS_UNSPECIFIED": 0, "BATCH_STATUS_ACTIVE": 1})),
			schema.F("UpdatedAt", schema.ServerManaged()),
		},
		Service: schema.ServiceConfig{
			Descriptor: testpb.File_testpb_test_proto.Services().ByName("BoolService"),
			Handler:    (*BoolServiceHandler)(nil),
		},
		Operations: schema.StandardOperations(schema.StandardOperationsConfig{
			List: schema.StandardOperation{
				Method: "ListBools", Request: (*testpb.ListBoolsRequest)(nil), Response: (*testpb.ListBoolsResponse)(nil),
				Binding: schema.OperationBinding{List: schema.ListBinding{Keyword: "Keyword", Sort: "Orders", Filters: map[string]string{"Enabled": "Enabled", "NarrowValue": "NarrowValue", "Status": "Status"}}},
			},
			Create:      schema.StandardOperation{Method: "CreateBool", Request: (*testpb.CreateBoolRequest)(nil), Response: (*testpb.CreateBoolResponse)(nil)},
			Get:         schema.StandardOperation{Method: "GetBool", Request: (*testpb.GetBoolRequest)(nil), Response: (*testpb.GetBoolResponse)(nil)},
			Update:      schema.StandardOperation{Method: "UpdateBool", Request: (*testpb.UpdateBoolRequest)(nil), Response: (*testpb.UpdateBoolResponse)(nil)},
			Delete:      schema.StandardOperation{Method: "DeleteBool", Request: (*testpb.DeleteBoolRequest)(nil), Response: (*testpb.DeleteBoolResponse)(nil)},
			BatchDelete: schema.StandardOperation{Method: "BatchDeleteBools", Request: (*testpb.BatchDeleteBoolsRequest)(nil), Response: (*testpb.BatchDeleteBoolsResponse)(nil), Binding: schema.OperationBinding{BatchIDs: "Ids"}},
		}),
		Key: schema.KeyConfig{ModelField: "ID"}, Audit: schema.AuditConfig{UpdatedAtField: "UpdatedAt"},
	}
}

// WidgetServiceHandler 覆盖 fixture service 的标准与 CUSTOM 操作，供
// ServiceConfig.Handler 与生成 ServiceAdapter 作精确编译期匹配。
type WidgetServiceHandler interface {
	ListWidgets(context.Context, *testpb.ListWidgetsRequest) (*testpb.ListWidgetsResponse, error)
	CreateWidget(context.Context, *testpb.CreateWidgetRequest) (*testpb.CreateWidgetResponse, error)
	GetWidget(context.Context, *testpb.GetWidgetRequest) (*testpb.GetWidgetResponse, error)
	UpdateWidget(context.Context, *testpb.UpdateWidgetRequest) (*testpb.UpdateWidgetResponse, error)
	DeleteWidget(context.Context, *testpb.DeleteWidgetRequest) (*testpb.DeleteWidgetResponse, error)
	BatchDeleteWidgets(context.Context, *testpb.BatchDeleteWidgetsRequest) (*testpb.BatchDeleteWidgetsResponse, error)
	RefreshWidget(context.Context, *testpb.RefreshWidgetRequest) (*testpb.RefreshWidgetResponse, error)
}

// widgetFixtureResource 声明完整六操作 + 标准 List wire 的 fixture Resource。
// WidgetFixtureResource 返回完整六操作 + 标准 List wire 的 fixture Resource。
func WidgetFixtureResource() *schema.Resource {
	return &schema.Resource{
		Name: "Widget", PermPrefix: "test:widget", Mode: schema.Generated, Table: "test_widget",
		Model: (*WidgetModel)(nil), Entity: (*testpb.WidgetEntity)(nil),
		CreateReq: (*testpb.CreateWidgetRequest)(nil), UpdateReq: (*testpb.UpdateWidgetRequest)(nil),
		Fields: []schema.Field{
			schema.F("ID", schema.ServerManaged(), schema.SortableOpt()),
			schema.F("Name", schema.Required(), schema.Searchable(), schema.MaxLen(100)),
			schema.F("Enabled", schema.Filterable()),
			schema.F("UpdatedAt", schema.ServerManaged()),
		},
		Service: schema.ServiceConfig{
			Descriptor: testpb.File_testpb_test_proto.Services().ByName("WidgetService"),
			Handler:    (*WidgetServiceHandler)(nil),
		},
		Operations: schema.StandardOperations(schema.StandardOperationsConfig{
			List: schema.StandardOperation{
				Method: "ListWidgets", Request: (*testpb.ListWidgetsRequest)(nil), Response: (*testpb.ListWidgetsResponse)(nil),
				Binding: schema.OperationBinding{List: schema.ListBinding{
					Keyword: "Keyword", Sort: "Orders",
					Filters: map[string]string{"Enabled": "Enabled"},
				}},
			},
			Create:      schema.StandardOperation{Method: "CreateWidget", Request: (*testpb.CreateWidgetRequest)(nil), Response: (*testpb.CreateWidgetResponse)(nil)},
			Get:         schema.StandardOperation{Method: "GetWidget", Request: (*testpb.GetWidgetRequest)(nil), Response: (*testpb.GetWidgetResponse)(nil)},
			Update:      schema.StandardOperation{Method: "UpdateWidget", Request: (*testpb.UpdateWidgetRequest)(nil), Response: (*testpb.UpdateWidgetResponse)(nil)},
			Delete:      schema.StandardOperation{Method: "DeleteWidget", Request: (*testpb.DeleteWidgetRequest)(nil), Response: (*testpb.DeleteWidgetResponse)(nil)},
			BatchDelete: schema.StandardOperation{Method: "BatchDeleteWidgets", Request: (*testpb.BatchDeleteWidgetsRequest)(nil), Response: (*testpb.BatchDeleteWidgetsResponse)(nil)},
		}),
		EntityIgnore: []string{"DeptID", "CreatedBy"},
		Key:          schema.KeyConfig{ModelField: "ID"}, Audit: schema.AuditConfig{UpdatedAtField: "UpdatedAt"},
	}
}

// WidgetRefreshModel is a second test-only Resource model sharing WidgetService
// through its CUSTOM RefreshWidget operation. Its distinct table/model lets
// BuildRegistry exercise true multi-Resource service composition.
type WidgetRefreshModel struct {
	ID        int64     `gorm:"column:id;primaryKey"`
	Name      string    `gorm:"column:name"`
	Enabled   bool      `gorm:"column:enabled"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (*WidgetRefreshModel) TableName() string { return "test_widget_refresh" }

// WidgetRefreshFixtureResource supplies only a typed CUSTOM operation. The
// ServiceAdapter test combines it with WidgetFixtureResource's standard six.
func WidgetRefreshFixtureResource() *schema.Resource {
	return &schema.Resource{
		Name: "WidgetRefresh", PermPrefix: "test:widget-refresh", Mode: schema.Partial, Table: "test_widget_refresh",
		Model: (*WidgetRefreshModel)(nil), Entity: (*testpb.WidgetRefreshEntity)(nil),
		CreateReq: (*testpb.CreateWidgetRequest)(nil), UpdateReq: (*testpb.UpdateWidgetRequest)(nil),
		Fields: []schema.Field{
			schema.F("ID", schema.ServerManaged()),
			schema.F("Name", schema.Required(), schema.Searchable(), schema.MaxLen(100)),
			schema.F("Enabled", schema.Filterable()),
			schema.F("UpdatedAt", schema.ServerManaged()),
		},
		Service: schema.ServiceConfig{
			Descriptor: testpb.File_testpb_test_proto.Services().ByName("WidgetService"),
			Handler:    (*WidgetServiceHandler)(nil),
		},
		Operations: []schema.Operation{{
			Kind: schema.OperationCustom, Method: "RefreshWidget",
			Request: (*testpb.RefreshWidgetRequest)(nil), Response: (*testpb.RefreshWidgetResponse)(nil),
			Implementation: schema.OperationOverride, Action: "refresh", ScopePolicy: schema.ScopePolicyExempt,
		}},
		Key: schema.KeyConfig{ModelField: "ID"}, Audit: schema.AuditConfig{UpdatedAtField: "UpdatedAt"},
	}
}

// RenamedColumnsFixtureModel separates GORM DB names from proto names for
// RenamedColumnsFixtureModel verifies database column mappings in generated code.
type RenamedColumnsFixtureModel struct {
	ID          int64     `gorm:"column:pk;primaryKey"`
	DisplayName string    `gorm:"column:name"`
	UpdatedAt   time.Time `gorm:"column:updated_at;precision:6"`
}

func (*RenamedColumnsFixtureModel) TableName() string { return "renamed_columns_fixture" }

type RenamedColumnsFixtureEntity struct {
	ID          int64                  `protobuf:"varint,1,opt,name=id,proto3"`
	DisplayName string                 `protobuf:"bytes,2,opt,name=display_name,json=displayName,proto3"`
	UpdatedAt   *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=updated_at,json=updatedAt,proto3"`
}
type RenamedColumnsFixtureCreate struct {
	DisplayName string `protobuf:"bytes,1,opt,name=display_name,json=displayName,proto3"`
}
type RenamedColumnsFixtureUpdate struct {
	ID          int64   `protobuf:"varint,1,opt,name=id,proto3"`
	DisplayName *string `protobuf:"bytes,2,opt,name=display_name,json=displayName,proto3,oneof"`
}

func RenamedColumnsFixtureResource() *schema.Resource {
	return &schema.Resource{Name: "RenamedColumnsFixture", PermPrefix: "test:renamed", Mode: schema.Generated, Table: "renamed_columns_fixture", Model: (*RenamedColumnsFixtureModel)(nil), Entity: (*testpb.RenamedColumnsEntity)(nil), CreateReq: (*testpb.CreateRenamedColumnsRequest)(nil), UpdateReq: (*testpb.UpdateRenamedColumnsRequest)(nil), Fields: []schema.Field{schema.F("ID", schema.ServerManaged()), schema.F("DisplayName"), schema.F("UpdatedAt", schema.ServerManaged())}, Key: schema.KeyConfig{ModelField: "ID"}, Audit: schema.AuditConfig{UpdatedAtField: "UpdatedAt"}}
}
func (x *RenamedColumnsFixtureCreate) GetDisplayName() string {
	if x != nil {
		return x.DisplayName
	}
	return ""
}
func (x *RenamedColumnsFixtureUpdate) GetDisplayName() *string {
	if x != nil {
		return x.DisplayName
	}
	return nil
}
