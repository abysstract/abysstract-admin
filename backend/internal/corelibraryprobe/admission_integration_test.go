//go:build integration

package corelibraryprobe

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/testdb"
)

// The explicit test configuration authorizes this disposable database only.
// This test exercises the shared-catalog guard and ResetSchema on PostgreSQL.
func TestDatabaseAdmissionAndResetRoundTrip(t *testing.T) {
	config, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	db := testutil.OpenDB(t)
	ctx := context.Background()
	if err := testdb.Verify(ctx, db, config); err != nil {
		t.Fatal(err)
	}
	testutil.ResetSchema(t, db)
	if err := testdb.Verify(ctx, db, config); err != nil {
		t.Fatalf("marker after reset: %v", err)
	}
	const table = "admission_sentinel"
	if err := db.Exec("CREATE TABLE " + table + " (id integer PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + table + " VALUES (1)").Error; err != nil {
		t.Fatal(err)
	}
	restore := "COMMENT ON DATABASE " + pgx.Identifier{config.Database}.Sanitize() + " IS '" + config.Marker() + "'"
	change := "COMMENT ON DATABASE " + pgx.Identifier{config.Database}.Sanitize() + " IS 'wrong-owner'"
	if config.Driver == "mysql" || config.Driver == "mariadb" {
		restore = "UPDATE p0_owner SET run_id='" + config.RunID + "'"
		change = "UPDATE p0_owner SET run_id='ffffffffffffffffffffffffffffffff'"
	}
	restoreMarker := func() error {
		if config.Driver == "sqlite" {
			return os.WriteFile(filepath.Join(filepath.Dir(config.Database), ".owner"), []byte(config.Marker()), 0600)
		}
		return db.Exec(restore).Error
	}
	t.Cleanup(func() {
		if err := restoreMarker(); err != nil {
			t.Errorf("restore owned database marker: %v", err)
		}
	})
	if config.Driver == "sqlite" {
		if err := os.WriteFile(filepath.Join(filepath.Dir(config.Database), ".owner"), []byte("wrong-owner"), 0600); err != nil {
			t.Fatal(err)
		}
	} else if err := db.Exec(change).Error; err != nil {
		t.Fatal(err)
	}

	if err := testdb.Verify(ctx, db, config); err == nil {
		t.Fatal("changed ownership was admitted")
	}
	if rejected, err := testdb.Open(config); err == nil || rejected != nil {
		t.Fatalf("new session admitted changed owner: db=%v err=%v", rejected, err)
	}
	var rows int64
	if err := db.Table(table).Count(&rows).Error; err != nil || rows != 1 {
		t.Fatalf("rejection changed data: rows=%d err=%v", rows, err)
	}
	if err := restoreMarker(); err != nil {
		t.Fatal(err)
	}
	testutil.ResetSchema(t, db)
	if err := testdb.Verify(ctx, db, config); err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasTable(table) {
		t.Fatal("reset retained sentinel")
	}
	t.Logf("exact database=%s; marker survived reset; wrong owner rejected before schema DDL; restored marker verified", config.Database)
}
