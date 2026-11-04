//go:build integration

package database_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	sqlite3 "github.com/mattn/go-sqlite3"

	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/testdb"
)

func TestSQLiteFileConnections(t *testing.T) {
	ctx := context.Background()
	run := "11111111111111111111111111111111"
	cfg, err := testdb.CreateSQLite(ctx, run, filepath.Join(t.TempDir(), "admin_core_k2_"+run))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := testdb.DropSQLite(cfg); err != nil {
			t.Error(err)
		}
	}()
	db, err := database.Open(ctx, "sqlite", cfg.DSN+"?_fk=off&_foreign_keys=off&_journal=DELETE&_timeout=1&_txlock=deferred")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if pool.Stats().MaxOpenConnections != 4 {
		t.Fatal(pool.Stats())
	}
	for _, statement := range []string{"CREATE TABLE parent(id INTEGER PRIMARY KEY)", "CREATE TABLE child(id INTEGER PRIMARY KEY, parent_id BIGINT REFERENCES parent(id))", "INSERT INTO parent VALUES (7)"} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	a, err := pool.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := pool.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i, conn := range []*sql.Conn{a, b} {
		var journal, version string
		var fk, timeout int
		for query, dest := range map[string]any{"PRAGMA journal_mode": &journal, "PRAGMA foreign_keys": &fk, "PRAGMA busy_timeout": &timeout, "SELECT sqlite_version()": &version} {
			if err := conn.QueryRowContext(ctx, query).Scan(dest); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("physical connection %d: engine=%s journal=%s fk=%d timeout=%d", i, version, journal, fk, timeout)
		if version != "3.45.1" || journal != "wal" || fk != 1 || timeout != 250 {
			t.Fatal("connection settings")
		}
		_, err := conn.ExecContext(ctx, "INSERT INTO child VALUES(1,999)")
		var native sqlite3.Error
		if !errors.As(err, &native) || native.ExtendedCode != sqlite3.ErrConstraintForeignKey {
			t.Fatalf("connection %d FK: %v", i, err)
		}
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := testdb.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	owned, err := reopened.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer owned.Close()
	var id int64
	if err := reopened.Raw("SELECT id FROM parent").Scan(&id).Error; err != nil || id != 7 {
		t.Fatalf("persistence id=%d: %v", id, err)
	}
}
