package resourcegen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReconcile_RemovesOrphans(t *testing.T) {
	g := testGenerator()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a_conv.gen.go"), []byte(generatedHeader+"\npackage gen\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b_conv.gen.go"), []byte(generatedHeader+"\npackage gen\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".adminresource-go.manifest"), []byte("a_conv.gen.go\nb_conv.gen.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.reconcile(dir, "go", ".gen.go", []string{"a_conv.gen.go"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b_conv.gen.go")); !os.IsNotExist(err) {
		t.Fatal("孤儿 b_conv.gen.go 应被清理")
	}
	if _, err := os.Stat(filepath.Join(dir, "a_conv.gen.go")); err != nil {
		t.Fatal("a_conv.gen.go 应保留")
	}
}

func TestReconcile_KeepsUnmanaged(t *testing.T) {
	g := testGenerator()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "handwritten.go"), []byte("package handwritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".adminresource-go.manifest"), []byte("a_conv.gen.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a_conv.gen.go"), []byte(generatedHeader+"\npackage gen\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.reconcile(dir, "go", ".gen.go", []string{"a_conv.gen.go"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "handwritten.go")); err != nil {
		t.Fatal("非受管文件不应被删除")
	}
}

func TestReconcile_RejectsTraversal(t *testing.T) {
	g := testGenerator()
	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "victim.gen.go")
	if err := os.WriteFile(victim, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel, _ := filepath.Rel(dir, victim)
	if err := os.WriteFile(filepath.Join(dir, ".adminresource-go.manifest"), []byte("a_conv.gen.go\n"+rel+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.reconcile(dir, "go", ".gen.go", []string{"a_conv.gen.go"}); err == nil {
		t.Fatal("illegal manifest item must fail")
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatal("目录外文件不应被删除")
	}
}

func TestReconcile_RejectsNonGenSuffix(t *testing.T) {
	g := testGenerator()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "important.go"), []byte("package important\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".adminresource-go.manifest"), []byte("important.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := g.reconcile(dir, "go", ".gen.go", nil); err == nil {
		t.Fatal("illegal manifest item must fail")
	}
	if _, err := os.Stat(filepath.Join(dir, "important.go")); err != nil {
		t.Fatal("非 .gen.go 文件不应被删除")
	}
}
