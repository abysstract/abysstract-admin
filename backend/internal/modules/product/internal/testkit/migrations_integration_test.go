//go:build integration

package producttest

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/abysstract/abysstract-admin/backend/migrations"
	"github.com/abysstract/abysstract-admin/backend/migrations/mariadb"
	"github.com/abysstract/abysstract-admin/backend/migrations/mysql"
	"github.com/abysstract/abysstract-admin/backend/migrations/postgres"
	"github.com/abysstract/abysstract-admin/backend/migrations/sqlite"
	"github.com/abysstract/abysstract-admin/backend/testdb"
)

func TestProductMenusRollbackAndRetry(t *testing.T) {
	cfg, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	gdb, err := testdb.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := gdb.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	ctx := context.Background()
	if err := testdb.ResetSchema(ctx, gdb, cfg); err != nil {
		t.Fatal(err)
	}
	seed := postgres.SeedAdminMigration()
	menus := postgres.ProductMenusMigration()
	dialect := goose.DialectPostgres
	if cfg.Driver == "mariadb" {
		seed = mariadb.SeedAdminMigration()
		menus = mariadb.ProductMenusMigration()
		dialect = goose.DialectMySQL
	}
	if cfg.Driver == "mysql" {
		seed = mysql.SeedAdminMigration()
		menus = mysql.ProductMenusMigration()
		dialect = goose.DialectMySQL
	}
	if cfg.Driver == "sqlite" {
		seed = sqlite.SeedAdminMigration()
		menus = sqlite.ProductMenusMigration()
		dialect = goose.DialectSQLite3
	}
	menuVersion := menus.Version
	provider, err := goose.NewProvider(dialect, pool, os.DirFS("../../../../../migrations/"+cfg.Driver),
		goose.WithDisableGlobalRegistry(true), goose.WithGoMigrations(seed, menus))
	if err != nil {
		t.Fatal(err)
	}
	// Stop immediately before the historical product menu migration so the
	// conflict exercises that transaction, before later table renames.
	if _, err := provider.UpTo(ctx, menuVersion-1); err != nil {
		t.Fatal(err)
	}

	if err := gdb.Exec(`INSERT INTO sys_role (role_name, role_key) VALUES ('外部只读', 'biz_product_reader')`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, menuVersion); err == nil || !strings.Contains(err.Error(), "role key biz_product_reader already exists without module binding") {
		t.Fatalf("expected product role conflict, got %v", err)
	}
	for _, query := range []string{
		`SELECT count(*) FROM biz_admin_binding`,
		`SELECT count(*) FROM sys_menu WHERE component='biz/product/index'`,
		`SELECT count(*) FROM sys_role WHERE role_key='biz_product_editor'`,
	} {
		var count int64
		if err := gdb.Raw(query).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("failed Go migration left %d rows for %s", count, query)
		}
	}
	if err := gdb.Exec(`DELETE FROM sys_role WHERE role_key='biz_product_reader'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Exec(`INSERT INTO sys_role (role_name, role_key, del_flag) VALUES ('旧只读', 'biz_product_reader', 1)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Exec(`INSERT INTO sys_menu (id, menu_name, parent_id, order_num, path, component, menu_type) VALUES (50000, '占位高ID', 0, 99, 'placeholder', '', 'M')`).Error; err != nil {
		t.Fatal(err)
	}
	if cfg.Driver == "postgres" {
		if err := gdb.Exec(`SELECT setval(pg_get_serial_sequence('sys_menu','id'),(SELECT MAX(id) FROM sys_menu))`).Error; err != nil {
			t.Fatal(err)
		}
	}
	var failedVersion int64
	if err := gdb.Raw("SELECT count(*) FROM goose_db_version WHERE version_id=? AND is_applied=true", menuVersion).Scan(&failedVersion).Error; err != nil || failedVersion != 0 {
		t.Fatalf("failed version %d %v", failedVersion, err)
	}
	if _, err := provider.UpTo(ctx, menuVersion); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := gdb.Raw(`SELECT count(*) FROM sys_role WHERE role_key IN ('biz_product_reader','biz_product_editor') AND del_flag=0`).Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("product roles=%d", count)
	}
	var pageID int64
	if err := gdb.Raw(`SELECT id FROM sys_menu WHERE component='biz/product/index'`).Scan(&pageID).Error; err != nil {
		t.Fatal(err)
	}
	if pageID <= 50000 {
		t.Fatalf("product page id=%d", pageID)
	}
	if err := migrations.Run(ctx, gdb); err != nil {
		t.Fatal(err)
	}
}
