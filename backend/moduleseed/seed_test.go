package moduleseed_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/coremigrations"
	"github.com/abysstract/abysstract-admin/backend/moduleseed"
)

func seedDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "seed.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Error(err)
		}
	})
	sources, err := coremigrations.SourcesFor("sqlite")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"20261001090001_init_sys.sql", "20261102090000_module_binding.sql"} {
		data, err := fs.ReadFile(sources, name)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(data), "-- +goose Down")
		if err := db.Exec(up).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func spec() moduleseed.Spec {
	return moduleseed.Spec{Module: "orders", Menus: []moduleseed.Menu{
		{Key: "orders.root", Name: "Orders", Type: "M", Path: "orders"},
		{Key: "orders.list", Parent: "orders.root", Name: "List", Type: "C", Component: "orders/index", Permission: "biz:orders:list"},
	}, Roles: []moduleseed.Role{{Key: "orders_reader", Name: "Reader", DataScope: 3, Menus: []string{"orders.root", "orders.list"}}}}
}

func TestInstallRemoveReinstall(t *testing.T) {
	db := seedDB(t)
	s := spec()
	install := func() error { return db.Transaction(func(tx *gorm.DB) error { return moduleseed.Install(tx, s) }) }
	if err := install(); err != nil {
		t.Fatal(err)
	}
	var menuID, roleID int64
	if err := db.Table("sys_module_binding").Select("system_id").Where("object_key = ?", "orders.list").Scan(&menuID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("sys_module_binding").Select("system_id").Where("object_key = ?", "orders_reader").Scan(&roleID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE sys_menu SET menu_name = 'Custom' WHERE id = ?", menuID).Error; err != nil {
		t.Fatal(err)
	}
	// Existing ordinary roles can receive grants and survive module removal.
	if err := db.Exec("INSERT INTO sys_role_menu (role_id,menu_id) VALUES (1,?)", menuID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO sys_user_role (user_id,role_id) VALUES (123,?)", roleID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE orders_item (id BIGINT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO orders_item VALUES (7)").Error; err != nil {
		t.Fatal(err)
	}
	if err := install(); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := db.Table("sys_menu").Select("menu_name").Where("id = ?", menuID).Scan(&name).Error; err != nil {
		t.Fatal(err)
	}
	if name != "Custom" {
		t.Fatal("overwrote administrator edit")
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return moduleseed.Remove(tx, "orders") }); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"sys_module_binding", "sys_user_role"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("stale rows in %s", table)
		}
	}
	var count int64
	if err := db.Table("orders_item").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("business data changed: %d %v", count, err)
	}
	if err := db.Table("sys_role").Where("id = 1").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("ordinary role removed")
	}
	if err := install(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveRejectsExternalChildren(t *testing.T) {
	db := seedDB(t)
	s := spec()
	if err := db.Transaction(func(tx *gorm.DB) error { return moduleseed.Install(tx, s) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO sys_menu (menu_name,parent_id,menu_type) SELECT 'External',system_id,'C' FROM sys_module_binding WHERE object_key = 'orders.root'").Error; err != nil {
		t.Fatal(err)
	}
	err := db.Transaction(func(tx *gorm.DB) error { return moduleseed.Remove(tx, "orders") })
	if err == nil || !strings.Contains(err.Error(), "externally owned child") {
		t.Fatalf("expected rejection, got %v", err)
	}
	var count int64
	if err := db.Table("sys_module_binding").Count(&count).Error; err != nil || count != 3 {
		t.Fatalf("failed removal changed bindings: %d %v", count, err)
	}
}
