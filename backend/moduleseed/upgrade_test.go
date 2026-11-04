package moduleseed_test

import (
	"testing"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/moduleseed"
)

func TestUpgradePreservesEditsAndEvolvesPermissions(t *testing.T) {
	db := seedDB(t)
	before := spec()
	if err := db.Transaction(func(tx *gorm.DB) error { return moduleseed.Install(tx, before) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE sys_menu SET menu_name = 'Custom' WHERE perms = 'biz:orders:list'").Error; err != nil {
		t.Fatal(err)
	}
	after := spec()
	after.Menus[1].Name = "New default"
	after.Menus[1].Permission = "biz:orders:read"
	after.Menus = append(after.Menus, moduleseed.Menu{Key: "orders.create", Parent: "orders.list", Name: "Create", Type: "F", Permission: "biz:orders:add"})
	if err := db.Transaction(func(tx *gorm.DB) error { return moduleseed.Upgrade(tx, before, after) }); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := db.Table("sys_menu").Select("menu_name").Where("perms = ?", "biz:orders:read").Scan(&name).Error; err != nil {
		t.Fatal(err)
	}
	if name != "Custom" {
		t.Fatalf("administrator edit lost: %q", name)
	}
	var id int64
	if err := db.Table("sys_menu").Select("id").Where("perms = ?", "biz:orders:add").Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("new permission missing")
	}
	if err := db.Exec("INSERT INTO sys_role_menu (role_id, menu_id) VALUES (1, ?)", id).Error; err != nil {
		t.Fatal(err)
	}
	final := spec()
	final.Menus = final.Menus[:1]
	final.Roles[0].Menus = []string{"orders.root"}
	if err := db.Transaction(func(tx *gorm.DB) error { return moduleseed.Upgrade(tx, after, final) }); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Table("sys_role_menu").Where("menu_id = ?", id).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("deleted permission retains grants")
	}
	if err := db.Table("sys_module_binding").Where("object_key IN ?", []string{"orders.list", "orders.create"}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("removed menus retain ownership")
	}
}

func TestUpgradeRollsBackForRetainedChild(t *testing.T) {
	db := seedDB(t)
	before := spec()
	if err := db.Transaction(func(tx *gorm.DB) error { return moduleseed.Install(tx, before) }); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO sys_menu (menu_name, parent_id, menu_type) SELECT 'External', id, 'F' FROM sys_menu WHERE perms = 'biz:orders:list'").Error; err != nil {
		t.Fatal(err)
	}
	after := spec()
	after.Menus = after.Menus[:1]
	after.Roles[0].Menus = []string{"orders.root"}
	if err := db.Transaction(func(tx *gorm.DB) error { return moduleseed.Upgrade(tx, before, after) }); err == nil {
		t.Fatal("accepted retained external child")
	}
	var count int64
	if err := db.Table("sys_menu").Where("perms = ?", "biz:orders:list").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("failed upgrade changed existing menu")
	}
}
