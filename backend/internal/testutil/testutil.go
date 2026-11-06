// Package testutil provides PostgreSQL fixtures admitted by the shared testdb guard.
package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/abysstract/abysstract-admin/backend/database"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/migrations"
	"github.com/abysstract/abysstract-admin/backend/testdb"
)

// OpenDB requires explicit disposable-database configuration before connecting.
func OpenDB(t *testing.T) *gorm.DB {
	t.Helper()
	config, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	db, err := testdb.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	return db
}

// Migrate applies the application's migrations to a test database.
func Migrate(gdb *gorm.DB) error {
	return migrations.Run(context.Background(), gdb)
}

// ResetSchema 清空 public schema 所有对象，回到初始可迁移状态。
// 用于迁移幂等测试 / e2e 测试的隔离。
//
// 注意：所有调用 ResetSchema 的测试共享同一个 public schema，因此**必须串行执行**
// （`go test -tags=integration -p 1`）。CI 已配置该参数。并行运行会导致测试间互相 DROP 对方的对象。
func ResetSchema(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	config, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	// MySQL/MariaDB drop tables one by one; slow CI runners need more than a few seconds.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := testdb.Verify(ctx, gdb, config); err != nil {
		t.Fatal(err)
	}
	if err := testdb.ResetSchema(ctx, gdb, config); err != nil {
		t.Fatal(err)
	}

}

// SchemaSnapshot 返回当前 public schema 的表/列/约束/索引快照，用于幂等比较。
// 返回的结构可直接用字符串相等比较。
type SchemaSnapshot struct {
	Tables      []string
	Columns     []string // 每行 "table.column:type"
	Constraints []string // 每行 "table.constraint_name:definition"
	Indexes     []string // 每行 "table.indexname:definition"
}

// Snapshot 拍摄当前 schema 快照。
func Snapshot(t *testing.T, gdb *gorm.DB) SchemaSnapshot {
	t.Helper()
	var snap SchemaSnapshot

	if database.DriverOf(gdb) == "sqlite" {
		for query, dest := range map[string]*[]string{
			"SELECT name FROM sqlite_master WHERE type='table' ORDER BY name":                            &snap.Tables,
			"SELECT name || ':' || sql FROM sqlite_master WHERE type='table' ORDER BY name":              &snap.Constraints,
			"SELECT name || ':' || COALESCE(sql,'') FROM sqlite_master WHERE type='index' ORDER BY name": &snap.Indexes,
		} {
			if err := gdb.Raw(query).Scan(dest).Error; err != nil {
				t.Fatal(err)
			}
		}
		return snap
	}

	if err := gdb.Raw(`
		SELECT tablename FROM pg_tables
		WHERE schemaname = 'public' ORDER BY tablename`).Scan(&snap.Tables).Error; err != nil {
		t.Fatalf("snapshot tables: %v", err)
	}

	if err := gdb.Raw(`
		SELECT table_name || '.' || column_name || ':' || data_type
		FROM information_schema.columns
		WHERE table_schema = 'public'
		ORDER BY table_name, ordinal_position`).Scan(&snap.Columns).Error; err != nil {
		t.Fatalf("snapshot columns: %v", err)
	}

	if err := gdb.Raw(`
		SELECT conrelid::regclass::text || '.' || conname || ':' || pg_get_constraintdef(oid)::text
		FROM pg_constraint
		WHERE connamespace = 'public'::regnamespace
		ORDER BY conrelid::regclass::text, conname`).Scan(&snap.Constraints).Error; err != nil {
		t.Fatalf("snapshot constraints: %v", err)
	}

	if err := gdb.Raw(`
		SELECT schemaname || '.' || tablename || '.' || indexname || ':' || indexdef
		FROM pg_indexes
		WHERE schemaname = 'public'
		ORDER BY tablename, indexname`).Scan(&snap.Indexes).Error; err != nil {
		t.Fatalf("snapshot indexes: %v", err)
	}
	return snap
}

// Equal 报告两个 schema 快照是否完全一致（迁移幂等校验）。
// 不一致时返回详细 diff 描述。
func (s SchemaSnapshot) Equal(other SchemaSnapshot) error {
	if !strSlicesEqual(s.Tables, other.Tables) {
		return fmt.Errorf("tables differ:\n got=%v\nwant=%v", s.Tables, other.Tables)
	}
	if !strSlicesEqual(s.Columns, other.Columns) {
		return fmt.Errorf("columns differ:\n got=%v\nwant=%v", s.Columns, other.Columns)
	}
	if !strSlicesEqual(s.Constraints, other.Constraints) {
		return fmt.Errorf("constraints differ:\n got=%v\nwant=%v", s.Constraints, other.Constraints)
	}
	if !strSlicesEqual(s.Indexes, other.Indexes) {
		return fmt.Errorf("indexes differ:\n got=%v\nwant=%v", s.Indexes, other.Indexes)
	}
	return nil
}

func strSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
