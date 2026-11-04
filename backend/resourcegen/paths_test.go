package resourcegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/resource"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

func testRootConfig(t *testing.T) (Config, string) {
	t.Helper()
	root := t.TempDir()
	cfg := fixtureConfig()
	cfg.RootDir = root
	cfg.GoOutputDir = "backend/gen"
	cfg.TSOutputDir = "frontend/gen"
	return cfg, root
}

func TestGenerateRejectsParentTraversal(t *testing.T) {
	cfg, root := testRootConfig(t)
	cfg.GoOutputDir = "../outside"
	if err := Generate(cfg, nil); err == nil || !strings.Contains(err.Error(), "escapes RootDir") {
		t.Fatalf("traversal error = %v", err)
	}
	if entries, _ := os.ReadDir(root); len(entries) != 0 {
		t.Fatalf("traversal must not write: %v", names(entries))
	}
}

func TestGenerateKeepsAdjacentSamePrefixDirectory(t *testing.T) {
	cfg, root := testRootConfig(t)
	neighbor := filepath.Join(root, "backend", "gen2")
	if err := os.MkdirAll(neighbor, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(neighbor, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(cfg, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatal("adjacent same-prefix directory must be unchanged")
	}
}

func TestGenerateRejectsEscapingDirectorySymlink(t *testing.T) {
	cfg, root := testRootConfig(t)
	outside := t.TempDir()
	link := filepath.Join(root, "backend")
	if err := os.MkdirAll(filepath.Join(root, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := Generate(cfg, nil); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("dir symlink error = %v", err)
	}
}

func TestGenerateRejectsManagedFileSymlink(t *testing.T) {
	cfg, root := testRootConfig(t)
	goDir := filepath.Join(root, "backend", "gen")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "frontend", "gen"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(goDir, "real.gen.go")
	if err := os.WriteFile(target, []byte(generatedHeader+"\npackage gen\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(goDir, "format.gen.go")); err != nil {
		t.Fatal(err)
	}
	if err := Generate(cfg, nil); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("file symlink error = %v", err)
	}
}

func TestGenerateRejectsDuplicateOutputNames(t *testing.T) {
	g := testGenerator()
	err := g.checkOutputs(t.TempDir(), "go", ".gen.go", []outFile{{name: "a.gen.go"}, {name: "a.gen.go"}})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestGenerateRejectsHandwrittenSameName(t *testing.T) {
	cfg, root := testRootConfig(t)
	goDir := filepath.Join(root, "backend", "gen")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "frontend", "gen"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "format.gen.go"), []byte("package handwritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(cfg, nil); err == nil || !strings.Contains(err.Error(), "not a generated file") {
		t.Fatalf("handwritten error = %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(goDir, "format.gen.go"))
	if string(got) != "package handwritten\n" {
		t.Fatal("handwritten file must stay unchanged")
	}
}

func TestGenerateEmptyResourceListCleansManagedFilesOnly(t *testing.T) {
	cfg, root := testRootConfig(t)
	goDir := filepath.Join(root, "backend", "gen")
	tsDir := filepath.Join(root, "frontend", "gen")
	if err := os.MkdirAll(goDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "old.gen.go"), []byte(generatedHeader+"\npackage gen\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, "notes.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, ".adminresource-go.manifest"), []byte("old.gen.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Generate(cfg, []*schema.Resource{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(goDir, "old.gen.go")); !os.IsNotExist(err) {
		t.Fatal("managed old file should be cleaned")
	}
	if _, err := os.Stat(filepath.Join(goDir, "notes.txt")); err != nil {
		t.Fatal("unmanaged file should remain")
	}
	if _, err := os.Stat(filepath.Join(goDir, "format.gen.go")); err != nil {
		t.Fatal("format file should be written")
	}
}

func TestGenerateZeroFormatVersionFailsBeforeWrite(t *testing.T) {
	cfg, root := testRootConfig(t)
	cfg.FormatVersion = 0
	if err := Generate(cfg, nil); err == nil {
		t.Fatal("zero format must fail")
	}
	if _, err := os.Stat(filepath.Join(root, cfg.GoOutputDir)); !os.IsNotExist(err) {
		t.Fatal("zero format must not write")
	}
	_ = resource.GeneratorFormatV1
}

func names(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.Name())
	}
	return out
}
