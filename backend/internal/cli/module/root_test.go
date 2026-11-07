package module

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRootAcceptsRootAndNestedGoMod(t *testing.T) {
	for name, mod := range map[string]string{"root module": "go.mod", "nested module": "backend/go.mod"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			start := filepath.Join(root, "backend", "internal", "modules")
			if err := os.MkdirAll(start, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, mod), []byte("module example.com/app\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := findRoot(start)
			if err != nil {
				t.Fatal(err)
			}
			if got != root {
				t.Fatalf("findRoot = %s, want %s", got, root)
			}
		})
	}
}

func TestFindRootRequiresBackendTree(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := findRoot(root); err == nil {
		t.Fatalf("findRoot = %s, want error without backend/", got)
	}
}
