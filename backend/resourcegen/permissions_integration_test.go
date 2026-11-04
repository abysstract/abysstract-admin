//go:build integration

package resourcegen

import (
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestRenderCustomPermissionRegistration(t *testing.T) {
	resource := schematest.WidgetFixtureResource()
	resource.Mode = schema.Partial
	resource.Operations = []schema.Operation{{
		Kind:           schema.OperationCustom,
		Method:         "ListWidgets",
		Request:        (*testpb.ListWidgetsRequest)(nil),
		Response:       (*testpb.ListWidgetsResponse)(nil),
		Implementation: schema.OperationOverride,
		Action:         "refresh",
		ScopePolicy:    schema.ScopePolicyExempt,
	}}
	plan, err := resource.Build()
	if err != nil {
		t.Fatalf("build custom fixture: %v", err)
	}
	files, err := renderGo(plan)
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
		t.Fatal("renderGo did not emit custom permission registration")
	}
	if !strings.Contains(source, `Permission: "test:widget:refresh"`) {
		t.Fatalf("CUSTOM action permission missing\n---\n%s", source)
	}
	compileWidgetPlan(t, plan, false)
}
