package resourcegen

import "testing"

func TestAssignAliasesStableForSamePackageName(t *testing.T) {
	set := assignAliases([]pkgRef{
		{path: "example.com/b/model", name: "model"},
		{path: "example.com/a/model", name: "model"},
	})
	if set.alias("example.com/a/model") != "model" {
		t.Fatalf("first sorted path alias = %q", set.alias("example.com/a/model"))
	}
	if set.alias("example.com/b/model") != "model_2" {
		t.Fatalf("second sorted path alias = %q", set.alias("example.com/b/model"))
	}
}
