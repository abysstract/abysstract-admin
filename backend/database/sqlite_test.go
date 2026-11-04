package database_test

import (
	"testing"

	"github.com/abysstract/abysstract-admin/backend/database"
)

func TestSQLiteRejectsNonFileTargets(t *testing.T) {
	for _, dsn := range []string{":memory:", "file:relative.db", "file:/tmp/x?mode=memory", "file://remote/tmp/x", "file:/tmp/x?mode=ro"} {
		if _, err := database.SQLitePath(dsn); err == nil {
			t.Fatalf("accepted %s", dsn)
		}
	}
}
