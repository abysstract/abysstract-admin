package schema

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

func dictService() ServiceConfig {
	return ServiceConfig{
		Descriptor: adminv1.File_abysstract_admin_v1_dict_proto.Services().ByName("DictService"),
		Handler:    (*adminv1connect.DictServiceHandler)(nil),
	}
}

func validDictTypeResource() *Resource {
	return &Resource{
		Name:       "DictTypeContract",
		PermPrefix: "system:dict",
		Mode:       Generated,
		Table:      model.TableNameSysDictType,
		Model:      (*model.SysDictType)(nil),
		Entity:     (*adminv1.DictType)(nil),
		CreateReq:  (*adminv1.CreateDictTypeRequest)(nil),
		UpdateReq:  (*adminv1.UpdateDictTypeRequest)(nil),
		Fields: []Field{
			F("ID", ServerManaged()),
			F("DictName", Required(), Searchable(), MaxLen(100)),
			F("DictType", Required(), MaxLen(100), CreateOnly()),
			F("Status", EnumMap(map[string]int16{"STATUS_ENABLED": 1, "STATUS_DISABLED": 2}), ReadOnly()),
			F("Remark", MaxLen(500)),
			F("CreateTime", ServerManaged()),
			F("UpdateTime", ServerManaged()),
		},
		Service: dictService(),
		Operations: []Operation{
			{
				Kind: OperationList, Method: "ListDictTypes",
				Request: (*adminv1.ListDictTypesRequest)(nil), Response: (*adminv1.ListDictTypesResponse)(nil),
				Binding:        OperationBinding{List: ListBinding{Page: "Page", Keyword: "Keyword", Items: "Items", PageInfo: "Page"}},
				Implementation: OperationGenerated,
			},
			{
				Kind: OperationCreate, Method: "CreateDictType",
				Request: (*adminv1.CreateDictTypeRequest)(nil), Response: (*adminv1.CreateDictTypeResponse)(nil),
				Binding: OperationBinding{Item: "Item"}, Implementation: OperationGenerated,
			},
			{
				Kind: OperationUpdate, Method: "UpdateDictType",
				Request: (*adminv1.UpdateDictTypeRequest)(nil), Response: (*adminv1.UpdateDictTypeResponse)(nil),
				Binding: OperationBinding{Identifier: "Id", Item: "Item"}, Implementation: OperationGenerated,
			},
			{
				Kind: OperationDelete, Method: "DeleteDictType",
				Request: (*adminv1.DeleteDictTypeRequest)(nil), Response: (*adminv1.DeleteDictTypeResponse)(nil),
				Binding: OperationBinding{Identifier: "Id"}, Implementation: OperationGenerated,
			},
		},
		Repository: RepositoryConfig{Implementation: RepositoryGenerated},
		Key:        KeyConfig{ModelField: "ID"},
		Audit:      AuditConfig{UpdatedAtField: "UpdateTime"},
	}
}

func TestBuildProducesUnifiedFieldKeyAuditAndOperationPlans(t *testing.T) {
	plan, err := validDictTypeResource().Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	field := plan.Field("DictName")
	if field == nil || field.ProtoName != "dict_name" || field.JSONName != "dictName" || field.DBName != "dict_name" || field.TSType != "string" {
		t.Fatalf("DictName FieldPlan not unified: %+v", field)
	}
	if plan.Key.ModelField != "ID" || plan.Key.JSONName != "id" || plan.Key.DBName != "id" || plan.Key.TSType != "bigint" {
		t.Fatalf("unexpected Key plan: %+v", plan.Key)
	}
	if plan.UpdatedAtModelField != "UpdateTime" || plan.UpdatedAtDBName != "update_time" {
		t.Fatalf("unexpected audit plan: %s/%s", plan.UpdatedAtModelField, plan.UpdatedAtDBName)
	}
	if got := plan.Operation(OperationList); got == nil || got.List == nil || got.List.PageRequestField != "Page" || got.List.PageInfoField != "Page" {
		t.Fatalf("standard List operation was not expanded: %+v", got)
	} else if got.RequestProtoModule != "abysstract/admin/v1/dict_pb" || got.ResponseProtoModule != "abysstract/admin/v1/dict_pb" {
		t.Fatalf("protobuf-es modules were not inferred: request=%q response=%q", got.RequestProtoModule, got.ResponseProtoModule)
	}
}

func TestBuildRejectsBrokenServiceMethodSignature(t *testing.T) {
	r := validDictTypeResource()
	r.Operations[1].Request = (*adminv1.UpdateDictTypeRequest)(nil)
	_, err := r.Build()
	if err == nil || !strings.Contains(err.Error(), "CreateDictType") || !strings.Contains(err.Error(), "Request") {
		t.Fatalf("wrong RPC request type must fail: %v", err)
	}
}

func TestBuildStandardListCapabilityReachability(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Resource)
		want   string
	}{
		{"missing page", func(r *Resource) { r.Operations[0].Binding.List.Page = "" }, "Page"},
		{"missing page info", func(r *Resource) { r.Operations[0].Binding.List.PageInfo = "" }, "PageInfo"},
		{"search without keyword", func(r *Resource) { r.Operations[0].Binding.List.Keyword = "" }, "Searchable"},
		{"filter without mapping", func(r *Resource) {
			for i := range r.Fields {
				if r.Fields[i].Name == "DictType" {
					r.Fields[i].Filterable = true
				}
			}
		}, "Filterable"},
		{"sort without binding", func(r *Resource) {
			for i := range r.Fields {
				if r.Fields[i].Name == "DictName" {
					r.Fields[i].Sortable = true
				}
			}
		}, "Sortable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := validDictTypeResource()
			tt.mutate(r)
			_, err := r.Build()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Build() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

type renamedColumnsModel struct {
	ID          int64     `gorm:"column:pk;primaryKey"`
	DisplayName string    `gorm:"column:name"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (*renamedColumnsModel) TableName() string { return "renamed_columns" }

type renamedColumnsEntity struct {
	ID          int64                  `protobuf:"varint,1,opt,name=id,proto3"`
	DisplayName string                 `protobuf:"bytes,2,opt,name=display_name,json=displayName,proto3"`
	UpdatedAt   *timestamppb.Timestamp `protobuf:"bytes,3,opt,name=updated_at,json=updatedAt,proto3"`
}

type renamedColumnsCreate struct {
	DisplayName string `protobuf:"bytes,1,opt,name=display_name,json=displayName,proto3"`
}

type renamedColumnsUpdate struct {
	ID          int64   `protobuf:"varint,1,opt,name=id,proto3"`
	DisplayName *string `protobuf:"bytes,2,opt,name=display_name,json=displayName,proto3,oneof"`
}

func renamedColumnsResource() *Resource {
	return &Resource{
		Name: "RenamedColumns", PermPrefix: "test:renamed", Mode: Generated, Table: "renamed_columns",
		Model: (*renamedColumnsModel)(nil), Entity: (*renamedColumnsEntity)(nil),
		CreateReq: (*renamedColumnsCreate)(nil), UpdateReq: (*renamedColumnsUpdate)(nil),
		Fields: []Field{
			F("ID", ServerManaged()),
			F("DisplayName"),
			F("UpdatedAt", ServerManaged()),
		},
		Key: KeyConfig{ModelField: "ID"}, Audit: AuditConfig{UpdatedAtField: "UpdatedAt"},
	}
}

func TestBuildUsesGORMDBNameInsteadOfProtoOrJSONName(t *testing.T) {
	plan, err := renamedColumnsResource().Build()
	if err != nil {
		t.Fatal(err)
	}
	field := plan.Field("DisplayName")
	if field.DBName != "name" || field.ProtoName != "display_name" || field.JSONName != "displayName" {
		t.Fatalf("wrong names in FieldPlan: %+v", field)
	}
	if plan.Key.DBName != "pk" || plan.UpdatedAtDBName != "updated_at" {
		t.Fatalf("key/audit did not reuse GORM metadata: key=%+v audit=%s", plan.Key, plan.UpdatedAtDBName)
	}
}

func TestBuildRejectsInvalidKeyAuditAndDBNames(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Resource)
		want   string
	}{
		{"missing key", func(r *Resource) { r.Key.ModelField = "" }, "Key.ModelField"},
		{"key not primary", func(r *Resource) { r.Key.ModelField = "DisplayName" }, "primary key"},
		{"missing audit", func(r *Resource) { r.Audit.UpdatedAtField = "" }, "Audit.UpdatedAtField"},
		{"audit missing schema field", func(r *Resource) { r.Audit.UpdatedAtField = "Nope" }, "Audit.UpdatedAtField"},
		{"audit not server managed", func(r *Resource) {
			for i := range r.Fields {
				if r.Fields[i].Name == "UpdatedAt" {
					r.Fields[i] = F("UpdatedAt", ReadOnly())
				}
			}
		}, "ServerManaged"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := renamedColumnsResource()
			tt.mutate(r)
			_, err := r.Build()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Build() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestConversionPlansRemainDirectionIndependent(t *testing.T) {
	modelType := reflect.TypeOf(int16(0))
	entityType := reflect.TypeOf(adminv1.Status_STATUS_ENABLED)
	requestType := reflect.TypeOf(int32(0))
	field := &Field{EnumMap: map[string]int16{"STATUS_ENABLED": 1}}
	if got := planConversion(modelType, entityType, enumIdentity(entityType, field)); got.Kind != ConvEnum {
		t.Fatalf("model->entity kind = %v, want ConvEnum", got.Kind)
	}
	if got := planConversion(requestType, modelType, enumIdentity(requestType, field)); got.Kind != ConvNarrow || !got.NeedsBounds {
		t.Fatalf("request->model plan = %+v, want bounded ConvNarrow", got)
	}
}

func TestBuildValidatesIdentifierAgainstExplicitKey(t *testing.T) {
	r := validDictTypeResource()
	r.Operations[2].Binding.Identifier = "DictName"
	_, err := r.Build()
	if err == nil || !strings.Contains(err.Error(), "exact builtin int64") {
		t.Fatalf("string Identifier must not be accepted for int64 Key: %v", err)
	}
}

func TestBuildValidatesBatchIDsElementAgainstExplicitKey(t *testing.T) {
	r := validDictTypeResource()
	r.Name = "DictTypeBatchContract"
	r.Service = ServiceConfig{
		Descriptor: adminv1.File_abysstract_admin_v1_common_export_proto.Services().ByName("CommonExportService"),
		Handler:    (*adminv1connect.CommonExportServiceHandler)(nil),
	}
	r.Operations = []Operation{{
		Kind: OperationBatchDelete, Method: "BatchDelete",
		Request: (*adminv1.BatchDeleteRequest)(nil), Response: (*adminv1.BatchDeleteResponse)(nil),
		Binding: OperationBinding{BatchIDs: "Ids"}, Implementation: OperationGenerated,
	}}
	if _, err := r.Build(); err != nil {
		t.Fatalf("valid repeated int64 BatchIDs must build: %v", err)
	}
	r.Operations[0].Binding.BatchIDs = "Resource"
	_, err := r.Build()
	if err == nil || !strings.Contains(err.Error(), "BatchIDs") {
		t.Fatalf("non-repeated BatchIDs binding must fail: %v", err)
	}
}
