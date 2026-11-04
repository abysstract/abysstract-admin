package schema

import (
	"strings"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
)

func validUserListTarget() *Resource {
	return &Resource{
		Name: "UserOptionTarget", PermPrefix: "system:user", Mode: Generated, Table: model.TableNameSysUser,
		Model: (*model.SysUser)(nil), Entity: (*adminv1.User)(nil),
		CreateReq: (*adminv1.CreateUserRequest)(nil), UpdateReq: (*adminv1.UpdateUserRequest)(nil),
		Fields: []Field{
			F("ID", ServerManaged()),
			F("Username", CreateOnly(), Searchable()),
			F("Status", ReadOnly(), Filterable(), EnumMap(map[string]int16{"STATUS_ENABLED": 1, "STATUS_DISABLED": 2})),
			F("UpdateTime", ServerManaged()),
		},
		Ignore:       []string{"password", "nickname", "email", "phone", "dept_id", "role_ids", "remark"},
		EntityIgnore: []string{"nickname", "email", "phone", "dept_id", "role_ids", "remark", "create_time"},
		Service: ServiceConfig{
			Descriptor: adminv1.File_abysstract_admin_v1_user_proto.Services().ByName("UserService"),
			Handler:    (*adminv1connect.UserServiceHandler)(nil),
		},
		Operations: []Operation{{
			Kind: OperationList, Method: "ListUsers",
			Request: (*adminv1.ListUsersRequest)(nil), Response: (*adminv1.ListUsersResponse)(nil),
			Binding: OperationBinding{List: ListBinding{
				Page: "Page", Keyword: "Keyword", Filters: map[string]string{"Status": "Status"}, Items: "Items", PageInfo: "Page",
			}},
			Implementation: OperationGenerated,
		}},
		Key: KeyConfig{ModelField: "ID"}, Audit: AuditConfig{UpdatedAtField: "UpdateTime"},
	}
}

func validRelationSource(target *Resource) *Resource {
	return &Resource{
		Name: "DictDataRelationSource", PermPrefix: "system:dict", Mode: Generated, Table: model.TableNameSysDictDatum,
		Model: (*model.SysDictDatum)(nil), Entity: (*adminv1.DictData)(nil),
		CreateReq: (*adminv1.CreateDictDataRequest)(nil), UpdateReq: (*adminv1.UpdateDictDataRequest)(nil),
		Fields: []Field{
			F("ID", ServerManaged()),
			F("DictType", CreateOnly(), RelationTo(func() *Resource { return target }, "Username", "Username")),
			F("UpdateTime", ServerManaged()),
		},
		Ignore: []string{"dict_sort", "dict_label", "dict_value", "is_default", "remark"},
		EntityIgnore: []string{
			"dict_sort", "dict_label", "dict_value", "css_class", "list_class", "is_default", "status", "remark", "create_time",
		},
		Key: KeyConfig{ModelField: "ID"}, Audit: AuditConfig{UpdatedAtField: "UpdateTime"},
	}
}

func TestBuildCreatesFilterPlanFromPresenceAwareListField(t *testing.T) {
	plan, err := validUserListTarget().Build()
	if err != nil {
		t.Fatal(err)
	}
	list := plan.Operation(OperationList).List
	if len(list.Filters) != 1 {
		t.Fatalf("FilterPlan count = %d, want 1", len(list.Filters))
	}
	filter := list.Filters[0]
	if filter.SchemaField != "Status" || filter.RequestField != "Status" || filter.RequestJSONName != "status" || filter.DBName != "status" {
		t.Fatalf("unexpected FilterPlan: %+v", filter)
	}
	if filter.ConversionPlan.Kind != ConvEnum || filter.ConversionPlan.SourceType == nil || filter.ConversionPlan.TargetType == nil {
		t.Fatalf("filter conversion was not precomputed: %+v", filter.ConversionPlan)
	}
}

func TestBuildRejectsFilterWithoutPresenceBeforeSQLPlan(t *testing.T) {
	r := validUserListTarget()
	r.Operations[0].Binding.List.Filters["Status"] = "Keyword"
	_, err := r.Build()
	if err == nil || !strings.Contains(err.Error(), "presence") {
		t.Fatalf("non-optional filter must fail Build: %v", err)
	}
}

func TestBuildRegistryCreatesRelationOptionLoadPlan(t *testing.T) {
	target := validUserListTarget()
	source := validRelationSource(target)
	plans, err := BuildRegistry([]*Resource{source, target})
	if err != nil {
		t.Fatal(err)
	}
	var sourcePlan *Plan
	for _, plan := range plans {
		if plan.Res == source {
			sourcePlan = plan
		}
	}
	relation := sourcePlan.Field("DictType").Relation
	if relation == nil || relation.TargetResource != target.Name || relation.RefJSONName != "username" || relation.LabelJSONName != "username" || !relation.RemoteSearchEnabled {
		t.Fatalf("unexpected relation load plan: %+v", relation)
	}
	decl := source.Fields[1].Relation
	if decl.RefJSONName != "username" || decl.LabelJSONName != "username" || !decl.RemoteSearchEnabled {
		t.Fatalf("resolved relation metadata was not retained on declaration: %+v", decl)
	}
}

func TestBuildRegistryRejectsUnpageableOrUnsafeRelations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(source, target *Resource)
		want   string
	}{
		{"target without standard list", func(_ *Resource, target *Resource) {
			target.Operations = nil
			target.Service = ServiceConfig{}
		}, "standard paginated List"},
		{"non-string label", func(source, _ *Resource) { source.Fields[1].Relation.LabelField = "Status" }, "LabelField"},
		{"enum ref key", func(source, _ *Resource) { source.Fields[1].Relation.RefKey = "Status" }, "RefKey"},
		{"non-listable label", func(_ *Resource, target *Resource) {
			for i := range target.Fields {
				if target.Fields[i].Name == "Username" {
					target.Fields[i].Listable = false
				}
			}
		}, "listable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := validUserListTarget()
			source := validRelationSource(target)
			tt.mutate(source, target)
			_, err := BuildRegistry([]*Resource{source, target})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("BuildRegistry() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestBuildRegistryRejectsDuplicateOwnershipAndIdentityDrift(t *testing.T) {
	t.Run("same service method", func(t *testing.T) {
		a := validDictTypeResource()
		b := validDictTypeResource()
		b.Name = "OtherResource"
		b.Table = "other_table"
		_, err := BuildRegistry([]*Resource{a, b})
		if err == nil || !strings.Contains(err.Error(), "service method") {
			t.Fatalf("duplicate method ownership must fail: %v", err)
		}
	})

	t.Run("same-name unregistered relation pointer", func(t *testing.T) {
		target := validUserListTarget()
		impostor := validUserListTarget()
		source := validRelationSource(impostor)
		_, err := BuildRegistry([]*Resource{source, target})
		if err == nil || !strings.Contains(err.Error(), "same instance") {
			t.Fatalf("relation identity drift must fail: %v", err)
		}
	})
}

func TestBuildRejectsInvalidLocalRelationShape(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(source, target *Resource)
		want   string
	}{
		{"nil target function", func(source, _ *Resource) { source.Fields[1].Relation.Target = nil }, "Target"},
		{"missing ref", func(source, _ *Resource) { source.Fields[1].Relation.RefKey = "Missing" }, "RefKey"},
		{"non-string label", func(source, _ *Resource) { source.Fields[1].Relation.LabelField = "Status" }, "LabelField"},
		{"illegal enum ref", func(source, _ *Resource) { source.Fields[1].Relation.RefKey = "Status" }, "RefKey"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := validUserListTarget()
			source := validRelationSource(target)
			tt.mutate(source, target)
			_, err := source.Build()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Build() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestBuildResolvesAuthModelFieldsThroughGORMMetadata(t *testing.T) {
	r := validUserListTarget()
	r.Auth = AuthConfig{DataScope: true, DeptField: "DeptID", CreatorField: "CreateBy"}
	plan, err := r.Build()
	if err != nil {
		t.Fatal(err)
	}
	if plan.DeptDBName != "dept_id" || plan.CreatorDBName != "create_by" {
		t.Fatalf("Auth DB names were not resolved through GORM: dept=%q creator=%q", plan.DeptDBName, plan.CreatorDBName)
	}
}
