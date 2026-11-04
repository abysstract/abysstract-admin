package module

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gen"

	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/internal/codegen/modelcfg"
)

func TestResourceCommand(t *testing.T) {
	root := fixture(t)
	if _, err := execute(t, root, "new", "notes", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, "backend/internal/modules/notes")
	if _, err := os.Stat(base); !os.IsNotExist(err) {
		t.Fatalf("dry new wrote module: %v", err)
	}
	if _, err := execute(t, root, "new", "notes"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(base, "resources"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			t.Fatalf("unexpected initial JSON: %s", entry.Name())
		}
	}
	if _, err := prepareDevelopment(root, "notes"); err != nil {
		t.Fatal(err)
	}
	// Existing handwritten modules may have no resources directory yet.
	if err := os.RemoveAll(filepath.Join(base, "resources")); err != nil {
		t.Fatal(err)
	}
	dsn := filepath.Join(t.TempDir(), "resources.sqlite")
	db, err := database.Open(context.Background(), "sqlite", "file:"+dsn)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("ADMIN_DB_DRIVER", "sqlite")
	t.Setenv("ADMIN_DB_DSN", "file:"+dsn)
	for _, ddl := range []string{
		"CREATE TABLE notes_note (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, enabled BOOLEAN, active_marker TEXT DEFAULT '', update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)",
		"INSERT INTO notes_note (title) VALUES ('keep')",
		"CREATE TABLE notes_tag (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, changed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)",
		"CREATE TABLE notes_bad (id TEXT PRIMARY KEY, title TEXT, update_time DATETIME NOT NULL)",
		"CREATE TABLE notes_audit (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT, update_time DATETIME)",
		"CREATE TABLE notes_float (id INTEGER PRIMARY KEY AUTOINCREMENT, amount REAL, update_time DATETIME NOT NULL)",
		"CREATE TABLE notes_ignored (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT, private TEXT DEFAULT '', update_time DATETIME NOT NULL)",
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	modelcfg.Overrides["notes_ignored"] = []gen.ModelOpt{gen.FieldIgnore("private")}
	t.Cleanup(func() { delete(modelcfg.Overrides, "notes_ignored") })
	before, err := os.ReadFile(dsn)
	if err != nil {
		t.Fatal(err)
	}
	out, err := execute(t, root, "resource", "notes", "notes_note", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"name": "Note"`) || !strings.Contains(out, `"title": "Note"`) || strings.Contains(out, "required") || strings.Contains(out, "active_marker") {
		t.Fatalf("unexpected plan: %s", out)
	}
	if _, err := os.Stat(filepath.Join(base, "resources/note.json")); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote JSON: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "resources")); !os.IsNotExist(err) {
		t.Fatalf("dry run created resources directory: %v", err)
	}
	after, err := os.ReadFile(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("dry run changed database")
	}
	if _, err := execute(t, root, "resource", "notes", "notes_note"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(base, "resources/note.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, flags := range [][]string{nil, {"--dry-run"}} {
		args := append([]string{"resource", "notes", "notes_note"}, flags...)
		if _, err := execute(t, root, args...); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Fatalf("duplicate accepted: %v", err)
		}
	}
	retained, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, retained) {
		t.Fatal("existing JSON changed")
	}
	if _, err := execute(t, root, "resource", "notes", "notes_tag", "--name", "Label", "--title", "标签", "--updated-at", "changed_at"); err != nil {
		t.Fatal(err)
	}
	var custom crudSpec
	if err := readJSON(filepath.Join(base, "resources/label.json"), &custom); err != nil {
		t.Fatal(err)
	}
	if custom.Name != "Label" || custom.Title != "标签" || custom.UpdatedAt != "changed_at" || len(custom.Fields) != 1 {
		t.Fatalf("custom options: %+v", custom)
	}
	if _, err := prepareDevelopment(root, "notes"); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "resource", "notes", "notes_ignored"); err != nil {
		t.Fatal(err)
	}
	var ignored crudSpec
	if err := readJSON(filepath.Join(base, "resources/ignored.json"), &ignored); err != nil {
		t.Fatal(err)
	}
	if len(ignored.Fields) != 1 || ignored.Fields[0].Column != "title" {
		t.Fatalf("model overrides ignored: %+v", ignored)
	}
	for _, tc := range []struct{ table, message string }{
		{"notes_missing", "read table notes_missing"}, {"notes_bad", "primary key"}, {"notes_audit", "nonnullable time.Time"}, {"notes_float", "Resource v1 supports"}, {"other_note", "notes_"},
	} {
		if _, err := execute(t, root, "resource", "notes", tc.table); err == nil || !strings.Contains(err.Error(), tc.message) {
			t.Errorf("%s: %v", tc.table, err)
		}
	}
}
