//go:build integration

package module

import (
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/testdb"
)

// TestCRUDAcceptsMySQLUnsignedKeys covers BIGINT UNSIGNED identifiers and
// unsigned reference columns on MySQL/MariaDB.
func TestCRUDAcceptsMySQLUnsignedKeys(t *testing.T) {
	cfg, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Driver != "mysql" && cfg.Driver != "mariadb" {
		t.Skip("UNSIGNED integers exist only on MySQL/MariaDB")
	}
	db, err := testdb.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, table := range []string{"notes_unsigned", "notes_signed_int"} {
			if err := db.Exec("DROP TABLE IF EXISTS " + table).Error; err != nil {
				t.Error(err)
			}
		}
		if err := pool.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, ddl := range []string{
		`CREATE TABLE notes_unsigned (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  title VARCHAR(100) NOT NULL,
  owner_id BIGINT UNSIGNED NULL,
  set_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  qty INT UNSIGNED NOT NULL DEFAULT 0,
  rank_no SMALLINT UNSIGNED NULL,
  signed_qty INT NOT NULL DEFAULT 0,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB`,
		`CREATE TABLE notes_signed_int (
  id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  title VARCHAR(100) NOT NULL,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB`,
	} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatal(err)
		}
	}
	meta, err := readCRUDModel(db, cfg.Driver, "notes_unsigned", "backend/internal/modules/notes")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"id": "int64", "owner_id": "*int64", "set_id": "int64", "qty": "int64", "rank_no": "*int32", "signed_qty": "int32"}
	for _, f := range meta.Fields {
		if expected, ok := want[f.ColumnName]; ok && f.Type != expected {
			t.Errorf("%s: Go type %s, want %s", f.ColumnName, f.Type, expected)
		}
	}
	spec := crudSpec{Name: "Unsigned", Table: "notes_unsigned", Title: "无符号", UpdatedAt: "update_time", Fields: []crudField{
		{Column: "title", Label: "标题"}, {Column: "owner_id", Label: "所有者"}, {Column: "set_id", Label: "卡组"}, {Column: "qty", Label: "数量"}, {Column: "rank_no", Label: "排名"},
	}}
	view, err := inspectCRUDModel("notes", spec, cfg.Driver, meta)
	if err != nil {
		t.Fatal(err)
	}
	protoTypes := map[string]string{}
	for _, f := range view.Fields {
		protoTypes[f.Column] = f.ProtoType
	}
	for column, expected := range map[string]string{"owner_id": "int64", "set_id": "int64", "qty": "int64", "rank_no": "int32"} {
		if protoTypes[column] != expected {
			t.Errorf("%s: Proto type %s, want %s", column, protoTypes[column], expected)
		}
	}

	signed := crudSpec{Name: "SignedInt", Table: "notes_signed_int", Title: "有符号", UpdatedAt: "update_time", Fields: []crudField{{Column: "title", Label: "标题"}}}
	if _, err := inspectCRUD(db, cfg.Driver, moduleView{Module: "notes"}, "backend/internal/modules/notes", signed); err == nil || !strings.Contains(err.Error(), "must generate an int64 ID") {
		t.Fatalf("signed INT identifier accepted: %v", err)
	}
}
