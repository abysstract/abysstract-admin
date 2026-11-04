package repo

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func execFixtureSQL(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	for _, statement := range strings.Split(sql, ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if err := db.Exec(statement, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func countFixtureRows(t *testing.T, db *gorm.DB, table, where string, args ...any) int64 {
	t.Helper()
	var count int64
	if err := db.Table(table).Where(where, args...).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}
