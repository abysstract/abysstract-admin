package coredefs

import (
	"strings"
	"testing"

	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestNoticeResourceDeclaresGeneratedStandardCRUDAndKnownBoundaries(t *testing.T) {
	if NoticeResource == nil {
		t.Fatal("Notice resource is nil")
	}
	if NoticeResource.Mode != schema.Generated {
		t.Fatalf("Notice mode = %v, want Generated", NoticeResource.Mode)
	}
	if NoticeResource.Repository.Implementation != schema.RepositoryGenerated {
		t.Fatalf("Notice repository implementation = %v, want RepositoryGenerated", NoticeResource.Repository.Implementation)
	}
	if NoticeResource.PermPrefix != "system:notice" {
		t.Fatalf("Notice permission prefix = %q, want system:notice", NoticeResource.PermPrefix)
	}
	if NoticeResource.Audit.UpdatedAtField != "UpdateTime" {
		t.Fatalf("Notice UpdatedAtField = %q, want UpdateTime", NoticeResource.Audit.UpdatedAtField)
	}
	if len(NoticeResource.Operations) != 6 {
		t.Fatalf("Notice operation count = %d, want standard six", len(NoticeResource.Operations))
	}
	plan, err := NoticeResource.Build()
	if err != nil {
		t.Fatalf("NoticeResource.Build: %v", err)
	}
	list := plan.Operation(schema.OperationList)
	if list == nil || len(list.Binding.List.DefaultOrders) != 1 || list.Binding.List.DefaultOrders[0].Field != "ID" || list.Binding.List.DefaultOrders[0].Desc {
		t.Fatalf("Notice default List order = %+v, want ID ASC", list)
	}

	fields := make(map[string]schema.Field, len(NoticeResource.Fields))
	for _, field := range NoticeResource.Fields {
		fields[field.Name] = field
	}
	for _, name := range []string{"ID", "NoticeTitle", "NoticeType", "NoticeContent", "Status", "Remark", "CreateTime", "UpdateTime"} {
		if _, ok := fields[name]; !ok {
			t.Fatalf("Notice field %s missing", name)
		}
	}
	if !fields["NoticeTitle"].Required || !fields["NoticeTitle"].Searchable || fields["NoticeTitle"].MaxLen != 100 {
		t.Fatal("NoticeTitle must retain required/searchable/VARCHAR(100) behavior")
	}
	if !fields["NoticeContent"].Required || fields["NoticeContent"].MaxLenSet {
		t.Fatal("NoticeContent must be required without an invented MaxLen for PostgreSQL TEXT")
	}
	if fields["Remark"].MaxLen != 500 {
		t.Fatal("Remark must retain VARCHAR(500) validation")
	}
	if !fields["Status"].ReadOnlyFlag || fields["Status"].Lifecycle.EffCreatable() || fields["Status"].Lifecycle.EffUpdatable() {
		t.Fatal("Status must remain read-only and outside Create/Update")
	}
	if len(fields["NoticeType"].EnumMap) != 0 {
		t.Fatal("plain int32 notice_type must not be modeled as a protobuf enum")
	}
}

func TestNoticeResourceBuildDerivesNarrowEnumAuditAndDBNamePlans(t *testing.T) {
	plan, err := NoticeResource.Build()
	if err != nil {
		t.Fatalf("NoticeResource.Build: %v", err)
	}
	for _, fieldName := range []string{"NoticeType", "NoticeContent", "NoticeTitle", "Remark"} {
		field := plan.Field(fieldName)
		if field == nil {
			t.Fatalf("Notice plan field %s missing", fieldName)
		}
		if field.DBName == "" {
			t.Fatalf("Notice plan field %s has empty DBName", fieldName)
		}
	}
	noticeType := plan.Field("NoticeType")
	if noticeType.CreateToModel.Kind != schema.ConvNarrow || !noticeType.CreateToModel.NeedsBounds || noticeType.UpdateToModel.Kind != schema.ConvNarrow || !noticeType.UpdateToModel.NeedsBounds {
		t.Fatalf("NoticeType conversion = create(%v,bounds=%v) update(%v,bounds=%v), want bounded ConvNarrow", noticeType.CreateToModel.Kind, noticeType.CreateToModel.NeedsBounds, noticeType.UpdateToModel.Kind, noticeType.UpdateToModel.NeedsBounds)
	}
	if noticeType.DBName != "notice_type" || plan.Key.DBName != "id" || plan.UpdatedAtDBName != "update_time" {
		t.Fatalf("Notice DB plan drift: type=%q key=%q updatedAt=%q", noticeType.DBName, plan.Key.DBName, plan.UpdatedAtDBName)
	}
	if noticeType.UpdateOptional != true {
		t.Fatal("NoticeType update presence must be represented in the plan")
	}
}

func TestNoticeResourceBuildRejectsEntityFieldDrift(t *testing.T) {
	resource := *NoticeResource
	resource.Fields = append([]schema.Field(nil), NoticeResource.Fields...)
	for i, field := range resource.Fields {
		if field.Name == "NoticeType" {
			resource.Fields = append(resource.Fields[:i], resource.Fields[i+1:]...)
			break
		}
	}
	_, err := resource.Build()
	if err == nil || !strings.Contains(err.Error(), `Entity field "notice_type" is not declared`) {
		t.Fatalf("entity/schema drift must fail Build: %v", err)
	}
}

func TestNoticeResourceBuildRejectsOperationAuditAndEnumDrift(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*schema.Resource)
		want   string
	}{
		{
			name: "missing Page",
			mutate: func(resource *schema.Resource) {
				resource.Operations = append([]schema.Operation(nil), resource.Operations...)
				resource.Operations[0].Binding.List.Page = ""
			},
			want: "requires Page binding",
		},
		{
			name: "missing PageInfo",
			mutate: func(resource *schema.Resource) {
				resource.Operations = append([]schema.Operation(nil), resource.Operations...)
				resource.Operations[0].Binding.List.PageInfo = ""
			},
			want: "requires PageInfo binding",
		},
		{
			name: "missing UpdatedAt",
			mutate: func(resource *schema.Resource) {
				resource.Audit.UpdatedAtField = ""
			},
			want: "Audit.UpdatedAtField",
		},
		{
			name: "enum mapping conflict",
			mutate: func(resource *schema.Resource) {
				for i, field := range resource.Fields {
					if field.Name == "Status" {
						resource.Fields[i] = schema.F("Status", schema.ReadOnly(), schema.EnumMap(map[string]int16{
							"STATUS_UNSPECIFIED": 0,
							"STATUS_ENABLED":     0,
						}))
					}
				}
			},
			want: "duplicate model value",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resource := *NoticeResource
			resource.Fields = append([]schema.Field(nil), NoticeResource.Fields...)
			test.mutate(&resource)
			if _, err := resource.Build(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Notice drift %s error = %v, want %q", test.name, err, test.want)
			}
		})
	}
}
