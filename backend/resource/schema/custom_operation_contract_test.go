package schema_test

import (
	schema "github.com/abysstract/abysstract-admin/backend/resource/schema"

	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"

	schematest "github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
)

func customWidgetResource(action string) *schema.Resource {
	resource := schematest.WidgetFixtureResource()
	resource.Mode = schema.Partial
	resource.Operations = []schema.Operation{{
		Kind:           schema.OperationCustom,
		Method:         "ListWidgets",
		Request:        (*testpb.ListWidgetsRequest)(nil),
		Response:       (*testpb.ListWidgetsResponse)(nil),
		Implementation: schema.OperationOverride,
		Action:         action,
		ScopePolicy:    schema.ScopePolicyExempt,
	}}
	return resource
}

// Requirement 11.2: a CUSTOM RPC cannot silently fall back to an unnamed
// permission. Its action is the only source for <perm_prefix>:<action>.
func TestCustomOperationRequiresAction(t *testing.T) {
	_, err := customWidgetResource("").Build()
	if err == nil {
		t.Fatal("CUSTOM operation without action unexpectedly built")
	}
	if !strings.Contains(err.Error(), "Action") {
		t.Fatalf("Build error = %v, want Action diagnostic", err)
	}
}

// Requirement 11.7: skipping the create-scope validator is meaningful only
// when row-level data scope is enabled. Reject a silent, misleading flag on
// unrestricted Resources rather than treating it as an unnoticed no-op.
func TestSkipCreateScopeCheckRequiresDataScope(t *testing.T) {
	resource := schematest.WidgetFixtureResource()
	resource.Auth = schema.AuthConfig{SkipCreateScopeCheck: true}
	_, err := resource.Build()
	if err == nil {
		t.Fatal("SkipCreateScopeCheck without DataScope unexpectedly built")
	}
	if !strings.Contains(err.Error(), "SkipCreateScopeCheck") {
		t.Fatalf("Build error = %v, want SkipCreateScopeCheck diagnostic", err)
	}
}

// Requirement 13.1/13.2: operation implementation is a closed mode contract.
// Standard CRUD cannot be overridden; CUSTOM exists only on Partial Resources
// and must use the explicit Override + EXEMPT security policy.
func TestBuildRejectsInvalidResourceOperationModeCombinations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*schema.Resource)
		want   string
	}{
		{
			name: "standard operation override",
			mutate: func(resource *schema.Resource) {
				resource.Operations[0].Implementation = schema.OperationOverride
			},
			want: "OperationGenerated",
		},
		{
			name: "generated resource custom",
			mutate: func(resource *schema.Resource) {
				resource.Mode = schema.Generated
				resource.Operations = customWidgetResource("refresh").Operations
			},
			want: "Generated Resource",
		},
		{
			name: "partial requires custom",
			mutate: func(resource *schema.Resource) {
				resource.Mode = schema.Partial
			},
			want: "at least one CUSTOM",
		},
		{
			name: "custom must override",
			mutate: func(resource *schema.Resource) {
				*resource = *customWidgetResource("refresh")
				resource.Operations[0].Implementation = schema.OperationGenerated
			},
			want: "OperationOverride",
		},
		{
			name: "custom scope policy exempt only",
			mutate: func(resource *schema.Resource) {
				*resource = *customWidgetResource("refresh")
				resource.Operations[0].ScopePolicy = schema.ScopePolicyUnspecified
			},
			want: "ScopePolicyExempt",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resource := schematest.WidgetFixtureResource()
			test.mutate(resource)
			_, err := resource.Build()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Build() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

// Requirement 13.1: RepositoryImplementation and OperationImplementation are
// intentionally distinct named types. This negative compile sample prevents a
// future untyped alias from allowing cross-assignment by accident.
func TestImplementationEnumsCannotCrossAssign(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for filepath.Base(root) != "backend" {
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("backend module root not found")
		}
		root = parent
	}
	dir, err := os.MkdirTemp(filepath.Join(root, "internal", "resource", "schema"), "implementation-enum-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	const source = `package enumcompile

import schema "github.com/abysstract/abysstract-admin/backend/resource/schema"

var _ schema.OperationImplementation = schema.RepositoryGenerated
`
	if err := os.WriteFile(filepath.Join(dir, "enum_compile.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "test", "./"+filepath.ToSlash(filepath.Join("internal", "resource", "schema", filepath.Base(dir))))
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("cross-assignment unexpectedly compiled:\n%s", output)
	}
	if !strings.Contains(string(output), "cannot use schema.RepositoryGenerated") {
		t.Fatalf("negative compile failed for unexpected reason:\n%s", output)
	}
}
