package module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicGenerationFollowsSelectedModules(t *testing.T) {
	root := fixture(t)
	if _, err := execute(t, root, "new", "notes"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend/internal/modules/notes/proto/company/notes/v1/note.proto"), []byte("syntax = \"proto3\";"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "sync"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"backend/internal/codegen/modules.json", "backend/internal/codegen/resources/tasks.gen.go", "buf.yaml"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "notes") {
			t.Fatalf("public generation includes unselected notes: %s", path)
		}
	}
	for _, path := range []string{"admin.modules.json", "backend/internal/app/modules.gen.go", "frontend/apps/reference/src/modules.gen.ts"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "notes") {
			t.Fatalf("development installed notes: %s", path)
		}
	}
	if _, err := executePreparation(t, root, "add", "notes"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"backend/internal/codegen/modules.json", "backend/internal/codegen/resources/tasks.gen.go", "buf.yaml"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "notes") {
			t.Fatalf("selected module omitted: %s", path)
		}
	}
	if _, err := executePreparation(t, root, "remove", "notes"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"backend/internal/codegen/modules.json", "backend/internal/codegen/resources/tasks.gen.go", "buf.yaml"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "notes") {
			t.Fatalf("removed module remains in public generation: %s", path)
		}
	}
	if err := os.RemoveAll(filepath.Join(root, "backend/internal/modules/notes")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(root, "frontend/apps/reference/src/modules/notes")); err != nil {
		t.Fatal(err)
	}

	if _, err := execute(t, root, "check"); err != nil {
		t.Fatal(err)
	}
}
