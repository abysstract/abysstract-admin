package coredefs

import (
	"testing"

	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestMenuResourceDeclaresStandardFlatCRUDAndTreeProjection(t *testing.T) {
	if MenuResource == nil {
		t.Fatal("Menu resource is nil")
	}
	if MenuResource.Mode != schema.Generated {
		t.Fatalf("Menu mode = %v, want Generated", MenuResource.Mode)
	}
	if MenuResource.Repository.Implementation != schema.RepositoryInjected {
		t.Fatalf("Menu repository implementation = %v, want RepositoryInjected for role_menu cleanup/child protection", MenuResource.Repository.Implementation)
	}
	if MenuResource.PermPrefix != "system:menu" {
		t.Fatalf("Menu permission prefix = %q, want system:menu", MenuResource.PermPrefix)
	}
	if MenuResource.Auth.DataScope {
		t.Fatal("Menu must not enable DataScope")
	}
	if len(MenuResource.Operations) != 6 {
		t.Fatalf("Menu operation count = %d, want standard six", len(MenuResource.Operations))
	}
	fields := make(map[string]schema.Field, len(MenuResource.Fields))
	for _, field := range MenuResource.Fields {
		fields[field.Name] = field
	}
	for _, name := range []string{"ID", "MenuName", "ParentID", "OrderNum", "Path", "Component", "Query", "IsFrame", "IsCache", "MenuType", "Visible", "Status", "Perms", "Icon", "CreateTime", "UpdateTime"} {
		if _, ok := fields[name]; !ok {
			t.Fatalf("Menu field %s missing", name)
		}
	}
	if !fields["MenuName"].Required || !fields["MenuName"].Searchable || fields["MenuName"].MaxLen != 64 {
		t.Fatalf("MenuName metadata = %+v", fields["MenuName"])
	}
	if !fields["ParentID"].Filterable || !fields["OrderNum"].Sortable {
		t.Fatalf("Menu parent/order list capability missing: parent=%+v order=%+v", fields["ParentID"], fields["OrderNum"])
	}
	if !fields["MenuType"].Required || !fields["MenuType"].Filterable || fields["MenuType"].MaxLen != 1 {
		t.Fatalf("MenuType metadata = %+v", fields["MenuType"])
	}
	if !fields["Status"].ReadOnlyFlag || !fields["Status"].Filterable {
		t.Fatalf("Menu Status metadata = %+v", fields["Status"])
	}
	if len(MenuResource.Ignore) != 0 || len(MenuResource.EntityIgnore) != 1 || MenuResource.EntityIgnore[0] != "children" {
		t.Fatalf("Menu entity projection boundary = request=%v entity=%v", MenuResource.Ignore, MenuResource.EntityIgnore)
	}
}
