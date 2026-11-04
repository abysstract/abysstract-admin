//go:build integration

package testdb

import (
	"context"
	"testing"

	"gorm.io/gorm"
)

func TestResetPreservesOwnership(t *testing.T) {
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(cfg)
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
	ctx := context.Background()
	if err := ResetSchema(ctx, db, cfg); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE test_reset_parent(id BIGINT PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE test_reset_child(id BIGINT PRIMARY KEY, parent_id BIGINT, FOREIGN KEY(parent_id) REFERENCES test_reset_parent(id))").Error; err != nil {
		t.Fatal(err)
	}
	if err := ResetSchema(ctx, db, cfg); err != nil {
		t.Fatal(err)
	}
	if err := Verify(ctx, db, cfg); err != nil {
		t.Fatal(err)
	}
	if db.Migrator().HasTable("test_reset_parent") || db.Migrator().HasTable("test_reset_child") {
		t.Fatal("tables retained")
	}
	if cfg.Driver == "mysql" || cfg.Driver == "mariadb" {
		if err := db.Connection(func(conn *gorm.DB) error {
			var enabled bool
			if err := conn.Raw("SELECT @@foreign_key_checks").Scan(&enabled).Error; err != nil {
				return err
			}
			if !enabled {
				t.Fatal("foreign key checks leaked")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}
