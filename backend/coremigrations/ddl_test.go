//go:build integration

package coremigrations_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"

	"github.com/abysstract/abysstract-admin/backend/testdb"
)

func TestPartialDDLForwardRepair(t *testing.T) {
	cfg, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	db, err := testdb.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := pool.Close(); err != nil {
			t.Error(err)
		}
	}()
	pool.SetMaxOpenConns(2)
	pool.SetMaxIdleConns(2)
	ctx := context.Background()
	if err := testdb.ResetSchema(ctx, db, cfg); err != nil {
		t.Fatal(err)
	}
	release := acquireFixtureLock(t, pool, cfg)
	defer release()
	dialect := goose.DialectPostgres
	if cfg.Driver == "sqlite" {
		dialect = goose.DialectSQLite3
	}
	if cfg.Driver == "mysql" || cfg.Driver == "mariadb" {
		dialect = goose.DialectMySQL
	}
	sources := fstest.MapFS{"00001_partial.sql": &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE ddl_first(id BIGINT PRIMARY KEY);\nCREATE TABLE ddl_second(id INVALID_TYPE);\n")}}
	if cfg.Driver == "sqlite" {
		sources["00001_partial.sql"].Data = []byte("-- +goose Up\nCREATE TABLE ddl_first(id BIGINT PRIMARY KEY);\nCREATE TABLE ddl_second(id INTEGER, id INTEGER);\n")
	}
	original := sha256.Sum256(sources["00001_partial.sql"].Data)
	provider, err := goose.NewProvider(dialect, pool, sources, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Up(ctx); err == nil {
		t.Fatal("expected DDL failure")
	}
	var applied int64
	if err := db.Raw("SELECT COUNT(*) FROM goose_db_version WHERE version_id=1 AND is_applied=true").Scan(&applied).Error; err != nil {
		t.Fatal(err)
	}
	if applied != 0 || db.Migrator().HasTable("ddl_second") {
		t.Fatal("unexpected partial version/state")
	}
	if cfg.Driver == "mysql" || cfg.Driver == "mariadb" {
		if !db.Migrator().HasTable("ddl_first") {
			t.Fatal("first DDL was rolled back")
		}
		var table, ddl string
		if err := pool.QueryRowContext(ctx, "SHOW CREATE TABLE ddl_first").Scan(&table, &ddl); err != nil {
			t.Fatal(err)
		}
		t.Log("checked committed structure:", ddl)
		if _, err := pool.ExecContext(ctx, "INSERT INTO goose_db_version(version_id,is_applied) VALUES (1,true)"); err != nil {
			t.Fatal(err)
		}
		sources["00002_repair.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE ddl_second(id BIGINT PRIMARY KEY);\n")}
	} else {
		if db.Migrator().HasTable("ddl_first") {
			t.Fatal("PostgreSQL failed DDL retained a table")
		}
		sources["00001_partial.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE ddl_first(id BIGINT PRIMARY KEY);\nCREATE TABLE ddl_second(id BIGINT PRIMARY KEY);\n")}
	}
	sources["00003_followup.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE ddl_third(id BIGINT PRIMARY KEY);\n")}
	provider, err = goose.NewProvider(dialect, pool, sources, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	changes, err := provider.Up(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 2 {
		t.Fatalf("repair/followup count %d", len(changes))
	}
	changes, err = provider.Up(ctx)
	if err != nil || len(changes) != 0 {
		t.Fatalf("repeat %d %v", len(changes), err)
	}
	if (cfg.Driver == "mysql" || cfg.Driver == "mariadb") && sha256.Sum256(sources["00001_partial.sql"].Data) != original {
		t.Fatal("failed source changed")
	}
	for _, name := range []string{"ddl_first", "ddl_second", "ddl_third"} {
		if !db.Migrator().HasTable(name) {
			t.Fatal(name)
		}
	}
	t.Log("partial state checked, forward repair/followup completed, repeat added zero")
}
