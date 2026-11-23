package module

import (
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCRUDRejectsStorageAndDeclarationMismatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
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
	if err := db.Exec("CREATE TABLE notes_note (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL, owner TEXT NOT NULL, update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatal(err)
	}
	spec := crudSpec{Name: "Note", Table: "notes_note", Title: "笔记", UpdatedAt: "update_time", Fields: []crudField{{Column: "title", Label: "标题"}}}
	if _, err := inspectCRUD(db, "sqlite", moduleView{Module: "notes"}, "backend/internal/modules/notes", spec); err == nil || !strings.Contains(err.Error(), "owner requires a value") {
		t.Fatalf("missing required column accepted: %v", err)
	}
	spec.Fields = append(spec.Fields, crudField{Column: "owner", Label: "所有者"}, crudField{Column: "missing", Label: "无此字段"})
	if _, err := inspectCRUD(db, "sqlite", moduleView{Module: "notes"}, "backend/internal/modules/notes", spec); err == nil || !strings.Contains(err.Error(), "missing is absent") {
		t.Fatalf("missing field accepted: %v", err)
	}
}

func TestCRUDCreateOnlyAndReadOnlyFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
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
	if err := db.Exec("CREATE TABLE notes_card (id INTEGER PRIMARY KEY AUTOINCREMENT, code TEXT NOT NULL, title TEXT NOT NULL, revision INTEGER NOT NULL, update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatal(err)
	}
	spec := crudSpec{Name: "Card", Table: "notes_card", Title: "卡片", UpdatedAt: "update_time", Fields: []crudField{
		{Column: "code", Label: "编号", Required: true, CreateOnly: true},
		{Column: "title", Label: "标题"},
		{Column: "revision", Label: "版本", ReadOnly: true},
	}}
	if err := validateCRUD("notes", spec); err != nil {
		t.Fatal(err)
	}
	// A readOnly NOT NULL column without a default could never be inserted.
	if _, err := inspectCRUD(db, "sqlite", moduleView{Module: "notes"}, "backend/internal/modules/notes", spec); err == nil || !strings.Contains(err.Error(), "revision requires a value") {
		t.Fatalf("readOnly column without default accepted: %v", err)
	}
	if err := db.Exec("DROP TABLE notes_card").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE notes_card (id INTEGER PRIMARY KEY AUTOINCREMENT, code TEXT NOT NULL, title TEXT NOT NULL, revision INTEGER NOT NULL DEFAULT 1, update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatal(err)
	}
	view, err := inspectCRUD(db, "sqlite", moduleView{Module: "notes", Backend: "backend/internal/modules/notes", DefinitionDir: "backend/internal/modules/notes/definition", ProtoDir: "backend/internal/modules/notes/proto", Frontend: "frontend/src/modules/notes"}, "backend/internal/modules/notes", spec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := assignProtocol(&view, protocolFields{}); err != nil {
		t.Fatal(err)
	}
	files, _, err := renderCRUD(view)
	if err != nil {
		t.Fatal(err)
	}
	proto := string(files["backend/internal/modules/notes/proto/company/notes/v1/card.proto"])
	message := func(name string) string {
		start := strings.Index(proto, "message "+name+" {")
		if start < 0 {
			t.Fatalf("message %s missing:\n%s", name, proto)
		}
		end := strings.Index(proto[start:], "}")
		return proto[start : start+end]
	}
	entity, create, update := message("Card"), message("CreateCardRequest"), message("UpdateCardRequest")
	for _, col := range []string{"code", "title", "revision"} {
		if !strings.Contains(entity, " "+col+" = ") {
			t.Errorf("entity lacks %s:\n%s", col, entity)
		}
	}
	if !strings.Contains(create, "string code = ") || !strings.Contains(create, "string title = ") || strings.Contains(create, "revision") {
		t.Errorf("create request:\n%s", create)
	}
	if strings.Contains(update, "code") || !strings.Contains(update, "optional string title = ") || strings.Contains(update, "revision") {
		t.Errorf("update request:\n%s", update)
	}
	definition := string(files["backend/internal/modules/notes/definition/card.gen.go"])
	if !strings.Contains(definition, `schema.F("Code", schema.Label("编号"), schema.Required(), schema.CreateOnly())`) || !strings.Contains(definition, `schema.F("Revision", schema.Label("版本"), schema.ReadOnly())`) {
		t.Errorf("definition options:\n%s", definition)
	}

	for _, bad := range []crudField{
		{Column: "revision", Label: "版本", ReadOnly: true, CreateOnly: true},
		{Column: "revision", Label: "版本", ReadOnly: true, Required: true},
	} {
		invalid := spec
		invalid.Fields = []crudField{spec.Fields[0], spec.Fields[1], bad}
		if err := validateCRUD("notes", invalid); err == nil || !strings.Contains(err.Error(), "readOnly excludes") {
			t.Errorf("%+v accepted: %v", bad, err)
		}
	}
}

func TestCRUDStandardWritesOff(t *testing.T) {
	no, yes := false, true
	spec := crudSpec{Name: "Entry", Table: "notes_entry", Title: "记录", UpdatedAt: "update_time", Fields: []crudField{{Column: "message", Label: "内容"}}}
	spec.StandardOperations = &crudStandardOperations{Create: &no, Update: &yes, Delete: &no}
	if err := validateCRUD("notes", spec); err == nil || !strings.Contains(err.Error(), "together") {
		t.Fatalf("partial standard writes accepted: %v", err)
	}
	spec.StandardOperations = &crudStandardOperations{Create: &no, Delete: &no}
	if err := validateCRUD("notes", spec); err == nil {
		t.Fatal("omitted update flag must count as on")
	}
	spec.StandardOperations = &crudStandardOperations{Create: &no, Update: &no, Delete: &no}
	if err := validateCRUD("notes", spec); err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pool.Close() })
	if err := db.Exec("CREATE TABLE notes_entry (id INTEGER PRIMARY KEY AUTOINCREMENT, message TEXT NOT NULL, update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatal(err)
	}
	view, err := inspectCRUD(db, "sqlite", moduleView{Module: "notes"}, "backend/internal/modules/notes", spec)
	if err != nil {
		t.Fatal(err)
	}
	view.moduleView = moduleView{Module: "notes", Backend: "b", DefinitionDir: "d", ProtoDir: "p", Frontend: "f"}
	files, menus, err := renderCRUD(view)
	if err != nil {
		t.Fatal(err)
	}
	proto := string(files["p/company/notes/v1/entry.proto"])
	for _, want := range []string{"rpc ListEntries(", "rpc GetEntry(", "message CreateEntryRequest", "message UpdateEntryRequest"} {
		if !strings.Contains(proto, want) {
			t.Errorf("proto lacks %q:\n%s", want, proto)
		}
	}
	for _, forbidden := range []string{"rpc CreateEntry", "rpc UpdateEntry", "rpc DeleteEntry", "rpc BatchDeleteEntries", "CreateEntryResponse", "DeleteEntryRequest"} {
		if strings.Contains(proto, forbidden) {
			t.Errorf("proto has %q:\n%s", forbidden, proto)
		}
	}
	definition := string(files["d/entry.gen.go"])
	if !strings.Contains(definition, "schema.OmitOperations(schema.StandardOperations(") || strings.Contains(definition, `"CreateEntry"`) {
		t.Errorf("definition:\n%s", definition)
	}
	page := string(files["f/pages/EntryList.tsx"])
	if !strings.Contains(page, "    readOnly\n") || strings.Contains(page, "EntryCreateValues") || strings.Contains(page, "rowActions") {
		t.Errorf("page:\n%s", page)
	}
	var permissions []string
	for _, m := range menus {
		permissions = append(permissions, m.Permission)
	}
	if strings.Join(permissions, ",") != "biz:notes:entry:list,biz:notes:entry:list" {
		t.Errorf("menus: %v", permissions)
	}
}
