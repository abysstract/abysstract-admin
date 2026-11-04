//go:build integration

package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestConnectedIdentityRejectsBeforeDDLAndClosesPool(t *testing.T) {
	name, _ := DatabaseName(runID)
	c, _ := ForDatabase("host=localhost dbname=old sslmode=disable", name, runID)
	for _, identity := range [][2]string{{"abysstract_admin", c.Marker()}, {name, ""}, {name, "abysstract-admin:test:another-run"}} {
		db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		pool, _ := db.DB()
		ddl := 0
		admitted, err := open(c, func(dsn string) (*gorm.DB, error) {
			if dsn != c.DSN {
				t.Fatal("dial used a different string")
			}
			return db, nil
		}, func(*gorm.DB) error { return c.verifyIdentity(identity[0], identity[1]) })
		if err == nil {
			ddl++
			admitted.Exec("CREATE TABLE forbidden (id integer)")
		}
		if err == nil || ddl != 0 {
			t.Fatal("rejected identity reached DDL")
		}
		if pool.Ping() == nil {
			t.Fatal("rejected pool remains open")
		}
	}
	if err := c.verifyIdentity(name, c.Marker()); err != nil {
		t.Fatal(err)
	}
}

// A local SQLite catalog implements only PostgreSQL's shared-object comment
// function. This exercises Verify's query and scan without a PostgreSQL socket.
func TestVerifySharedCatalogWithSQLite(t *testing.T) {
	name, _ := DatabaseName(runID)
	config, _ := ForDatabase("host=localhost dbname=old sslmode=disable", name, runID)
	for i, tt := range []struct {
		database, marker string
		wantOK           bool
	}{
		{name, config.Marker(), true},
		{"abysstract_admin", config.Marker(), false},
		{name, "", false},
		{name, "abysstract-admin:test:another-run", false},
	} {
		driver := fmt.Sprintf("testdb_shared_catalog_%d_%d", catalogCounter.Add(1), i)
		sql.Register(driver, &sqlite3.SQLiteDriver{ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			if err := conn.RegisterFunc("current_database", func() string { return tt.database }, true); err != nil {
				return err
			}
			return conn.RegisterFunc("shobj_description", func(oid int64, catalog string) string {
				if oid != 1 || catalog != "pg_database" {
					return ""
				}
				return tt.marker
			}, true)
		}})
		db, err := gorm.Open(sqlite.Dialector{DriverName: driver, DSN: ":memory:"}, &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		pool, _ := db.DB()
		t.Cleanup(func() { _ = pool.Close() })
		if err := db.Exec("CREATE TABLE pg_database (oid INTEGER, datname TEXT)").Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("INSERT INTO pg_database VALUES (1, ?)", tt.database).Error; err != nil {
			t.Fatal(err)
		}
		err = Verify(context.Background(), db, config)
		if (err == nil) != tt.wantOK {
			t.Fatalf("case %d verify=%v wantOK=%v", i, err, tt.wantOK)
		}
	}
}
