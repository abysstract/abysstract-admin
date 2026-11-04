//go:build integration

package moduleseed_test

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/migrations"
	"github.com/abysstract/abysstract-admin/backend/moduleseed"
	"github.com/abysstract/abysstract-admin/backend/testdb"
)

// This test admits only a disposable database provisioned by admin-testdb.
func TestApplicationHistoryAndSeedLifecycle(t *testing.T) {
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
	defer pool.Close()
	if err := testdb.ResetSchema(context.Background(), db, cfg); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	var count int64
	s := spec()
	if err := db.Transaction(func(tx *gorm.DB) error { return moduleseed.Install(tx, s) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return moduleseed.Remove(tx, s.Module) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("sys_menu").Where("perms = ?", "biz:orders:list").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("stale module menus: %d %v", count, err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return moduleseed.Install(tx, s) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Table("sys_module_binding").Where("module_name = ?", s.Module).Count(&count).Error; err != nil || count != 3 {
		t.Fatalf("reinstallation: %d %v", count, err)
	}
	// Remaining old migrations are already applied and stay idempotent.
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
}
