package resourcegen

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/gen"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func TestWidgetCompileInspectFixturesUpToDate(t *testing.T) {
	t.Helper()
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}

	// Both checked-in Widget consumers share this generated fixture query package.
	queryDir := filepath.Join(root, "internal", "resource", "schematest", "query")
	tmp := t.TempDir()
	q := gen.NewGenerator(gen.Config{OutPath: filepath.Join(tmp, "query")})
	q.ApplyBasic(schematest.WidgetModel{})
	q.Execute()
	entries, err := os.ReadDir(filepath.Join(tmp, "query"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		want, err := os.ReadFile(filepath.Join(tmp, "query", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(queryDir, entry.Name())
		if os.Getenv("UPDATE_WIDGET_COMPILE_INSPECT") == "1" {
			if err := os.MkdirAll(queryDir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, want, 0644); err != nil {
				t.Fatal(err)
			}
		} else {
			got, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("stale query fixture %s", target)
			}
		}
	}
	expected, err := widgetCompileInspectExpected(t)
	if err != nil {
		t.Fatal(err)
	}
	dirs := []string{
		filepath.Join(root, "internal", "resource", "widgetcompileinspect"),
		filepath.Join(root, "cmd", "adminresource-gen", "internal", "resource", "widgetcompileinspect"),
	}
	if os.Getenv("UPDATE_WIDGET_COMPILE_INSPECT") == "1" {
		for _, dir := range dirs {
			for name, content := range expected {
				if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
					t.Fatalf("write %s/%s: %v", dir, name, err)
				}
			}
		}
		return
	}
	for _, dir := range dirs {
		for name, want := range expected {
			got, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("read %s/%s: %v", dir, name, err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("stale compile-inspect fixture %s/%s; rerun with UPDATE_WIDGET_COMPILE_INSPECT=1", dir, name)
			}
		}
	}
}

func widgetCompileInspectExpected(t *testing.T) (map[string][]byte, error) {
	t.Helper()
	res := schematest.WidgetFixtureResource()
	res.Auth = schema.AuthConfig{DataScope: true, DeptField: "DeptID", CreatorField: "CreatedBy"}
	plan, err := res.Build()
	if err != nil {
		return nil, err
	}
	files, err := renderGo(plan)
	if err != nil {
		return nil, err
	}
	name, controller, err := renderController(plan)
	if err != nil {
		return nil, err
	}
	runtimeTest, err := format.Source([]byte("//go:build integration\n\n" + widgetDataScopeRuntimeTest))
	if err != nil {
		return nil, err
	}
	expected := map[string][]byte{
		name:                         controller,
		"data_scope_runtime_test.go": runtimeTest,
	}
	for _, file := range files {
		expected[file.name] = file.content
	}
	return expected, nil
}
