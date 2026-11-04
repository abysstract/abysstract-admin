package coremigrations_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"syscall"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/testdb"
)

func lockAcquireSQL(cfg testdb.Config) string {
	if cfg.Driver == "mysql" || cfg.Driver == "mariadb" {
		sum := sha256.Sum256([]byte(cfg.Database))
		return fmt.Sprintf("SELECT GET_LOCK('abysstract-admin:%x', 0)", sum[:20])
	}
	return "SELECT pg_try_advisory_lock(728195301)"
}

func lockReleaseSQL(cfg testdb.Config) string {
	if cfg.Driver == "mysql" || cfg.Driver == "mariadb" {
		sum := sha256.Sum256([]byte(cfg.Database))
		return fmt.Sprintf("SELECT RELEASE_LOCK('abysstract-admin:%x')", sum[:20])
	}
	return "SELECT pg_advisory_unlock(728195301)"
}

// Hold the same product lock from a separate process/connection as production.
func acquireFixtureLock(t *testing.T, pool *sql.DB, cfg testdb.Config) func() {
	t.Helper()
	if cfg.Driver == "sqlite" {
		pool.SetMaxOpenConns(1)
		pool.SetMaxIdleConns(1)
		file, err := os.OpenFile(cfg.Database+".migrate.lock", os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			t.Fatal(err)
		}
		return func() {
			if err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
	conn, err := pool.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var acquired bool
	if err := conn.QueryRowContext(context.Background(), lockAcquireSQL(cfg)).Scan(&acquired); err != nil || !acquired {
		t.Fatalf("lock: %v %v", acquired, err)
	}
	return func() {
		var released bool
		if err := conn.QueryRowContext(context.Background(), lockReleaseSQL(cfg)).Scan(&released); err != nil || !released {
			t.Fatalf("unlock: %v %v", released, err)
		}
		if err := conn.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
