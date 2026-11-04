//go:build integration

package migrations_test

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"

	"github.com/abysstract/abysstract-admin/backend/database"

	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/migrations"
)

func TestGooseRollsBackFailedSQLAndRetries(t *testing.T) {
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	pool, err := gdb.DB()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := migrations.Run(ctx, gdb); err != nil {
		t.Fatal(err)
	}
	const name = "20991022120000_retry.sql"
	failed := fstest.MapFS{name: &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE migration_retry_probe (id integer);\nINSERT INTO absent_migration_table (id) VALUES (1);\n")}}
	dialect := goose.DialectPostgres
	if database.DriverOf(gdb) == "sqlite" {
		dialect = goose.DialectSQLite3
	}
	if database.DriverOf(gdb) == "mysql" || database.DriverOf(gdb) == "mariadb" {
		dialect = goose.DialectMySQL
	}
	provider, err := goose.NewProvider(dialect, pool, failed, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err == nil {
		t.Fatal("expected SQL failure")
	}
	var count int64
	exists := gdb.Migrator().HasTable("migration_retry_probe")
	if exists != (database.DriverOf(gdb) == "mysql" || database.DriverOf(gdb) == "mariadb") {
		t.Fatalf("partial DDL state exists=%v", exists)
	}
	if err := gdb.Raw(`SELECT count(*) FROM goose_db_version WHERE version_id=20991022120000 AND is_applied`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed version recorded as applied")
	}
	fixed := fstest.MapFS{name: &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE migration_retry_probe (id integer);\n")}}
	if database.DriverOf(gdb) == "mysql" || database.DriverOf(gdb) == "mariadb" {
		if err := gdb.Exec("INSERT INTO goose_db_version(version_id,is_applied) VALUES(20991022120000,true)").Error; err != nil {
			t.Fatal(err)
		}
		fixed = failed
		fixed["20991022120001_forward.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE absent_migration_table(id integer);\nINSERT INTO absent_migration_table(id) VALUES(1);\n")}
	}
	provider, err = goose.NewProvider(dialect, pool, fixed, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := gdb.Raw(`SELECT count(*) FROM goose_db_version WHERE version_id=20991022120000 AND is_applied`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("retry applied records=%d", count)
	}
}
