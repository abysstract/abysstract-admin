package resourcegen

import (
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/coredefs"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// 11.1b: each declared standard operation emits an explicit registration
// function. The compile fixture consumes every renderGo file, so this test plus
// TestWidgetControllerGeneratedSourcesCompile proves the generated source is
// valid Go rather than merely a template string.
func TestRenderStandardPermissionRegistration(t *testing.T) {
	files, err := renderGo(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderGo: %v", err)
	}
	var source string
	for _, file := range files {
		if file.name == "widget_permissions.gen.go" {
			source = string(file.content)
			break
		}
	}
	if source == "" {
		t.Fatal("renderGo did not emit widget_permissions.gen.go")
	}
	for _, want := range []string{
		"func RegisterWidgetPermissions(reg permission.Registrar) error",
		`"/resource.schema.testpb.WidgetService/ListWidgets"`,
		`"test:widget:list"`,
		`"/resource.schema.testpb.WidgetService/GetWidget"`,
		`"/resource.schema.testpb.WidgetService/CreateWidget"`,
		`"test:widget:add"`,
		`"/resource.schema.testpb.WidgetService/UpdateWidget"`,
		`"test:widget:edit"`,
		`"/resource.schema.testpb.WidgetService/DeleteWidget"`,
		`"/resource.schema.testpb.WidgetService/BatchDeleteWidgets"`,
		`"test:widget:remove"`,
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("generated permission registration missing %q\n---\n%s", want, source)
		}
	}
}

func TestRenderLegacyGetPermissionOverride(t *testing.T) {
	resource := coredefs.RoleResource
	plan, err := resource.Build()
	if err != nil {
		t.Fatalf("build Role permission override fixture: %v", err)
	}
	files, err := renderGo(plan)
	if err != nil {
		t.Fatalf("renderGo: %v", err)
	}
	var source string
	for _, file := range files {
		if file.name == "role_permissions.gen.go" {
			source = string(file.content)
			break
		}
	}
	if source == "" {
		t.Fatal("renderGo did not emit role_permissions.gen.go")
	}
	if !strings.Contains(source, `Procedure: "/abysstract.admin.v1.RoleService/GetRole", Permission: "system:role:query"`) {
		t.Fatalf("legacy Get permission override missing\n---\n%s", source)
	}
	if !strings.Contains(source, `Procedure: "/abysstract.admin.v1.RoleService/UpdateRole", Permission: "system:role:edit"`) {
		t.Fatal("legacy permission override changed an unrelated standard mapping")
	}
}

func TestBuildRejectsUnregisteredLegacyPermissionOverride(t *testing.T) {
	resource := schematest.WidgetFixtureResource()
	for i := range resource.Operations {
		if resource.Operations[i].Kind == schema.OperationGet {
			resource.Operations[i].LegacyPermission = schema.LegacyPermissionGetQuery
		}
	}
	if _, err := resource.Build(); err == nil || !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("unregistered legacy permission override error = %v, want registration diagnostic", err)
	}
}

func TestBuildRejectsLegacyPermissionOnNonGetOperation(t *testing.T) {
	resource := *coredefs.RoleResource
	resource.Operations = append([]schema.Operation(nil), coredefs.RoleResource.Operations...)
	for i := range resource.Operations {
		if resource.Operations[i].Kind == schema.OperationDelete {
			resource.Operations[i].LegacyPermission = schema.LegacyPermissionGetQuery
		}
	}
	if _, err := resource.Build(); err == nil || !strings.Contains(err.Error(), "only valid for standard Get") {
		t.Fatalf("legacy permission on Delete error = %v, want standard Get diagnostic", err)
	}
}
