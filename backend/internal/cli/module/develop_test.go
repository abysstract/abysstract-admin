package module

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestContinuousDevelopment(t *testing.T) {
	root := fixture(t)
	if _, err := execute(t, root, "new", "notes"); err != nil {
		t.Fatal(err)
	}
	dsn := filepath.Join(t.TempDir(), "development.sqlite")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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
	if err := db.Exec("CREATE TABLE notes_note (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatal(err)
	}
	base := "backend/internal/modules/notes"
	if _, err := execute(t, root, "resource", "notes", "notes_note", "--title", "Notes"); err != nil {
		t.Fatal(err)
	}
	var spec crudSpec
	if err := readJSON(filepath.Join(root, base, "resources/note.json"), &spec); err != nil {
		t.Fatal(err)
	}
	spec.Fields[0].Required = true
	spec.Fields[0].Searchable = true
	data, err := jsonBytes(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeFiles(root, map[string][]byte{base + "/resources/note.json": data}, false); err != nil {
		t.Fatal(err)
	}
	files, err := prepareDevelopment(root, "notes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files[base+"/definition/note.gen.go"]), `biz:notes:note`) {
		t.Fatalf("resource permission prefix: %s", files[base+"/definition/note.gen.go"])
	}
	if _, err := writeFiles(root, files, false); err != nil {
		t.Fatal(err)
	}
	extension := base + "/note.go"
	original, err := os.ReadFile(filepath.Join(root, extension))
	if err != nil {
		t.Fatal(err)
	}
	edited := append(original, []byte("\n// Business hooks remain here.\n")...)
	if err := os.WriteFile(filepath.Join(root, extension), edited, 0644); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE notes_note ADD COLUMN published BOOLEAN").Error; err != nil {
		t.Fatal(err)
	}
	spec.Fields = append([]crudField{{Column: "published", Label: "Published"}}, spec.Fields...)
	data, err = jsonBytes(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeFiles(root, map[string][]byte{base + "/resources/note.json": data}, false); err != nil {
		t.Fatal(err)
	}
	files, err = prepareDevelopment(root, "notes")
	if err != nil {
		t.Fatal(err)
	}
	proto := string(files[base+"/proto/company/notes/v1/note.proto"])
	if !strings.Contains(proto, "title = 2;") || !strings.Contains(proto, "published = 4;") {
		t.Fatalf("unstable numbers: %s", proto)
	}
	if _, exists := files[extension]; exists {
		t.Fatal("overwriting business extension")
	}
	if _, err := writeFiles(root, files, false); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE notes_tag (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, root, "resource", "notes", "notes_tag", "--title", "Tags"); err != nil {
		t.Fatal(err)
	}
	files, err = prepareDevelopment(root, "notes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files[base+"/module.gen.go"]), "generatedTagModule") {
		t.Fatal("second resource not wired")
	}
	if _, err := writeFiles(root, files, false); err != nil {
		t.Fatal(err)
	}
	files, err = prepareDevelopment(root, "notes")
	if err != nil {
		t.Fatal(err)
	}
	changes, err := writeFiles(root, files, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 0 {
		t.Fatalf("generation not idempotent: %v", changes)
	}
	spec.Fields = spec.Fields[1:]
	data, err = jsonBytes(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeFiles(root, map[string][]byte{base + "/resources/note.json": data}, false); err != nil {
		t.Fatal(err)
	}
	files, err = prepareDevelopment(root, "notes")
	if err != nil {
		t.Fatal(err)
	}
	proto = string(files[base+"/proto/company/notes/v1/note.proto"])
	if !strings.Contains(proto, "reserved 4;") || !strings.Contains(proto, `reserved "published";`) {
		t.Fatal("retired number and name missing")
	}
	spec.Operations = []crudOperation{{Method: "InspectNote", Request: "InspectNoteRequest", Response: "InspectNoteResponse", Action: "inspect", Title: "Inspect", Proto: "company/notes/v1/actions.proto", Scope: "exempt"}}
	data, err = jsonBytes(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeFiles(root, map[string][]byte{base + "/resources/note.json": data, base + "/proto/company/notes/v1/actions.proto": []byte("syntax = \"proto3\";\n")}, false); err != nil {
		t.Fatal(err)
	}
	files, err = prepareDevelopment(root, "notes")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(files[base+"/definition/note.gen.go"]), "schema.Partial") || !strings.Contains(string(files[base+"/proto/company/notes/v1/note.proto"]), "rpc InspectNote") {
		t.Fatal("custom operation missing from generated contract")
	}
	if _, err := writeFiles(root, files, false); err != nil {
		t.Fatal(err)
	}

	protected := base + "/proto/company/notes/v1/note.proto"
	originalContract := files[protected]
	if err := os.WriteFile(filepath.Join(root, protected), append(append([]byte(nil), originalContract...), []byte("\n// handwritten edit\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareDevelopment(root, "notes"); err == nil || !strings.Contains(err.Error(), "has edits") {
		t.Fatalf("overwrote edited generated contract: %v", err)
	}
	if _, err := writeFiles(root, map[string][]byte{protected: originalContract}, false); err != nil {
		t.Fatal(err)
	}
	spec.Operations = nil
	data, err = jsonBytes(spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeFiles(root, map[string][]byte{base + "/resources/note.json": data}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareDevelopment(root, "notes"); err == nil || !strings.Contains(err.Error(), "InspectNote was removed") {
		t.Fatalf("accepted removed custom operation: %v", err)
	}
}

func TestProtocolRejectsBreakingChanges(t *testing.T) {
	initial := crudView{Fields: []crudFieldView{{Column: "title", ProtoType: "string"}}}
	state, err := assignProtocol(&initial, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []crudFieldView{{Column: "title", ProtoType: "int64"}, {Column: "title", ProtoType: "string", Nullable: true}, {Column: "title", ProtoType: "string", Managed: true}} {
		changed := crudView{Fields: []crudFieldView{field}}
		if _, err := assignProtocol(&changed, state); err == nil {
			t.Fatalf("accepted incompatible field: %+v", field)
		}
	}
	empty := crudView{}
	retired, err := assignProtocol(&empty, state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assignProtocol(&initial, retired); err == nil {
		t.Fatal("reused retired field")
	}
}
