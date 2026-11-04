package module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/moduleseed"
)

func TestUpdateLifecycleAndMigrationRetry(t *testing.T) {
	root := fixture(t)
	if _, err := execute(t, root, "new", "notes"); err != nil {
		t.Fatal(err)
	}
	// Fail migration execution to exercise saved-state retries without a database.
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte("#!/bin/sh\nexit 23\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	before, err := os.ReadFile(filepath.Join(root, "admin.modules.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "update", "notes", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, "admin.modules.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("dry-run changed module selection")
	}
	snapshots := func() []string {
		t.Helper()
		paths, err := filepath.Glob(filepath.Join(root, "backend/migrations/modules/*_notes_*.json"))
		if err != nil {
			t.Fatal(err)
		}
		return paths
	}
	if len(snapshots()) != 0 {
		t.Fatal("dry-run saved migration snapshots")
	}
	run := func(action string) {
		t.Helper()
		_, err := execute(t, root, action, "notes")
		if err == nil || !strings.Contains(err.Error(), "exit status 23") || !strings.Contains(err.Error(), "rerun module "+action+" notes") {
			t.Fatalf("expected migration failure with recovery command, got %v", err)
		}
	}
	run("update")
	run("update")
	paths := snapshots()
	if len(paths) != 1 || !strings.HasSuffix(paths[0], "_install.json") {
		t.Fatalf("repeated update should reuse installation snapshot: %v", paths)
	}
	d, err := readDescriptor(root, "backend/internal/modules/notes/module.json")
	if err != nil {
		t.Fatal(err)
	}
	var seed moduleseed.Spec
	if err := readJSON(filepath.Join(root, d.Seed), &seed); err != nil {
		t.Fatal(err)
	}
	seed.Menus = append(seed.Menus, moduleseed.Menu{Key: "notes.root", Name: "Notes", Type: "M"})
	data, err := jsonBytes(seed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, d.Seed), data, 0644); err != nil {
		t.Fatal(err)
	}
	run("update")
	run("update")
	if paths = snapshots(); len(paths) != 2 || !strings.HasSuffix(paths[1], "_upgrade.json") {
		t.Fatalf("changed seed should create one upgrade snapshot: %v", paths)
	}
	run("uninstall")
	for _, path := range []string{"backend/internal/codegen/modules.json", "backend/internal/codegen/resources/tasks.gen.go", "buf.yaml"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "notes") {
			t.Fatalf("uninstall retained public task: %s", path)
		}
	}
	run("update")
	if paths = snapshots(); len(paths) != 4 || !strings.HasSuffix(paths[2], "_remove.json") || !strings.HasSuffix(paths[3], "_install.json") {
		t.Fatalf("reinstallation should preserve migration history: %v", paths)
	}
	if _, err := execute(t, root, "check"); err != nil {
		t.Fatal(err)
	}
}
