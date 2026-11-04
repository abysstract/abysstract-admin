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
	if _, err := inspectCRUD(db, "sqlite", "notes", spec); err == nil || !strings.Contains(err.Error(), "owner requires a value") {
		t.Fatalf("missing required column accepted: %v", err)
	}
	spec.Fields = append(spec.Fields, crudField{Column: "owner", Label: "所有者"}, crudField{Column: "missing", Label: "无此字段"})
	if _, err := inspectCRUD(db, "sqlite", "notes", spec); err == nil || !strings.Contains(err.Error(), "missing is absent") {
		t.Fatalf("missing field accepted: %v", err)
	}
}
