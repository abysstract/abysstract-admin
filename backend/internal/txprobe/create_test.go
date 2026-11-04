package txprobe

import (
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/testdb"
)

func countBy(t *testing.T, database *testdb.Database, table, column, value string) int64 {
	t.Helper()
	var count int64
	if err := database.DB.Table(database.TableName(table)).Where(column+" = ?", value).Count(&count).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}
