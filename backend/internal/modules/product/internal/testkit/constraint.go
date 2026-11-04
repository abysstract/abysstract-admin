package producttest

import (
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/database"
)

// AddConstraint gives failure fixtures the same real CHECK/FK on each product.
// SQLite changes table constraints by rebuilding the table and its explicit indexes.
func AddConstraint(t *testing.T, db *gorm.DB, table, name, definition string) {
	t.Helper()
	if database.DriverOf(db) != "sqlite" {
		if err := db.Exec("ALTER TABLE " + table + " ADD CONSTRAINT " + name + " " + definition).Error; err != nil {
			t.Fatal(err)
		}
		return
	}
	err := db.Connection(func(conn *gorm.DB) error {
		var ddl string
		if err := conn.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&ddl).Error; err != nil {
			return err
		}
		var indexes []string
		if err := conn.Raw("SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name=? AND sql IS NOT NULL ORDER BY name", table).Scan(&indexes).Error; err != nil {
			return err
		}
		if err := conn.Exec("PRAGMA foreign_keys=OFF").Error; err != nil {
			return err
		}
		err := conn.Transaction(func(tx *gorm.DB) error {
			end := strings.LastIndex(ddl, ")")
			altered := strings.Replace(ddl[:end], table, table+"_constraint_fixture", 1) + ", CONSTRAINT " + name + " " + definition + ddl[end:]
			for _, sql := range []string{altered, "INSERT INTO " + table + "_constraint_fixture SELECT * FROM " + table, "DROP TABLE " + table, "ALTER TABLE " + table + "_constraint_fixture RENAME TO " + table} {
				if err := tx.Exec(sql).Error; err != nil {
					return err
				}
			}
			for _, sql := range indexes {
				if err := tx.Exec(sql).Error; err != nil {
					return err
				}
			}
			return nil
		})
		return errors.Join(err, conn.Exec("PRAGMA foreign_keys=ON").Error)
	})
	if err != nil {
		t.Fatal(err)
	}
}
