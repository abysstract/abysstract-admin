//go:build integration

package database_test

import (
	"context"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/testdb"
)

func TestConnectionContract(t *testing.T) {
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
	var env struct{ Version, Engine, Collation, SQLMode, TimeZone string }
	if cfg.Driver == "mysql" || cfg.Driver == "mariadb" {
		if err := db.Raw("SELECT VERSION() AS version, @@default_storage_engine AS engine, @@collation_server AS collation, @@sql_mode AS sql_mode, @@session.time_zone AS time_zone").Scan(&env).Error; err != nil {
			t.Fatal(err)
		}
		t.Logf("environment: %+v", env)
	}
	if database.DriverOf(db) != cfg.Driver {
		t.Fatal("product identity")
	}
	if cfg.Driver == "mysql" && (env.Version != "8.4.2" || env.Engine != "InnoDB" || env.TimeZone != "+00:00") {
		t.Fatal("unexpected MySQL environment")
	}
	if cfg.Driver == "mariadb" && (!strings.HasPrefix(env.Version, "11.4.13-MariaDB") || env.Engine != "InnoDB" || env.TimeZone != "+00:00") {
		t.Fatalf("unexpected MariaDB environment: %+v", env)
	}
	if cfg.Driver == "mysql" || cfg.Driver == "mariadb" {
		other := "mysql"
		if cfg.Driver == "mysql" {
			other = "mariadb"
		}
		if wrong, err := database.Open(context.Background(), other, cfg.DSN); err == nil {
			pool, err := wrong.DB()
			if err != nil {
				t.Fatal(err)
			}
			if err := pool.Close(); err != nil {
				t.Fatal(err)
			}
			t.Fatal("mislabeled server accepted")
		}
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if cfg.Driver == "mysql" || cfg.Driver == "mariadb" {
			if err := tx.Exec("DELETE FROM p0_owner").Error; err != nil {
				return err
			}
			if err := testdb.Verify(context.Background(), tx, cfg); err == nil {
				t.Fatal("missing ownership admitted")
			}
		}
		return gorm.ErrInvalidTransaction
	}); err != gorm.ErrInvalidTransaction {
		t.Fatal(err)
	}
	if err := testdb.Verify(context.Background(), db, cfg); err != nil {
		t.Fatal(err)
	}
	if database.DriverOf(db.WithContext(context.Background())) != cfg.Driver {
		t.Fatal("session identity")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if database.DriverOf(tx) != cfg.Driver {
			t.Fatal("transaction identity")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
