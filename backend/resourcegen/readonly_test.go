package resourcegen

import (
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// readOnlyWidgetPlan is the Widget fixture with its standard writes omitted,
// the shape a JSON resource with all standard writes turned off generates.
func readOnlyWidgetPlan(t *testing.T) *schema.Plan {
	t.Helper()
	r := schematest.WidgetFixtureResource()
	r.Operations = schema.OmitOperations(r.Operations,
		schema.OperationCreate, schema.OperationUpdate, schema.OperationDelete, schema.OperationBatchDelete)
	plan, err := r.Build()
	if err != nil {
		t.Fatalf("read-only widget build: %v", err)
	}
	return plan
}

func TestOmitOperationsKeepsOrderAndOtherKinds(t *testing.T) {
	ops := schematest.WidgetFixtureResource().Operations
	got := schema.OmitOperations(ops, schema.OperationCreate, schema.OperationDelete)
	var kinds []schema.OperationKind
	for _, op := range got {
		kinds = append(kinds, op.Kind)
	}
	want := []schema.OperationKind{schema.OperationList, schema.OperationGet, schema.OperationUpdate, schema.OperationBatchDelete}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", kinds, want)
		}
	}
	if len(ops) != 6 {
		t.Fatalf("input mutated: %d operations", len(ops))
	}
}

func TestGeneratedReadOnlyResourceUsesReadController(t *testing.T) {
	plan := readOnlyWidgetPlan(t)
	if !hasGeneratedReadController(plan) {
		t.Fatal("Generated List+Get should use the read controller")
	}
	if hasGeneratedReadController(widgetPlan(t)) {
		t.Fatal("full six must not use the read controller")
	}
	files, err := renderGo(plan)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]string{}
	for _, f := range files {
		sources[f.name] = string(f.content)
	}
	ctrl := sources["widget_controller.gen.go"]
	if ctrl == "" {
		t.Fatalf("no controller rendered: %v", files)
	}
	for _, want := range []string{"func (c *WidgetController) ListWidgets(", "func (c *WidgetController) GetWidget(", "resource.ReadRepository["} {
		if !strings.Contains(ctrl, want) {
			t.Errorf("controller lacks %q", want)
		}
	}
	for _, forbidden := range []string{"CreateWidget(", "UpdateWidget(", ") DeleteWidget(", "BatchDeleteWidgets(", "Overrides"} {
		if strings.Contains(ctrl, forbidden) {
			t.Errorf("controller renders %q", forbidden)
		}
	}
	perms := sources["widget_permissions.gen.go"]
	if !strings.Contains(perms, "ListWidgets") || strings.Contains(perms, "CreateWidget") || strings.Contains(perms, "DeleteWidget") {
		t.Errorf("permissions:\n%s", perms)
	}

	code := renderTSOperationsFor(t, plan)
	if !strings.Contains(code, "satisfies ResourceReadOperations<") || strings.Contains(code, "satisfies ResourceOperations<") {
		t.Errorf("read-only facade:\n%s", code)
	}
}

func TestGeneratedReadOnlyResourceCompiles(t *testing.T) {
	compileWidgetPlanWithRuntime(t, readOnlyWidgetPlan(t), "")
}
