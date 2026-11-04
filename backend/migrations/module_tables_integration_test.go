//go:build integration

package migrations_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/pressly/goose/v3"
	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/database"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/migrations"
	"github.com/abysstract/abysstract-admin/backend/migrations/mariadb"
	"github.com/abysstract/abysstract-admin/backend/migrations/mysql"
	"github.com/abysstract/abysstract-admin/backend/migrations/postgres"
	"github.com/abysstract/abysstract-admin/backend/migrations/sqlite"
)

func TestModuleTableRenamePreservesDataAndHistory(t *testing.T) {
	const renameVersion int64 = 20261101090000
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	ctx := context.Background()
	provider, menuVersion := applicationMigrationProvider(t, db)
	if _, err := provider.UpTo(ctx, menuVersion); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		"INSERT INTO biz_product(product_no,name,dept_id,create_by) VALUES('preserved','existing',1,1)",
		"INSERT INTO biz_product_event(product_id,actor_id,action) VALUES(1,1,'create')",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	var bindings []struct {
		ObjectKey  string
		ObjectType string
		SystemID   int64
	}
	if err := db.Table("biz_admin_binding").Order("object_key").Find(&bindings).Error; err != nil {
		t.Fatal(err)
	}
	var history int64
	if err := db.Table("goose_db_version").Count(&history).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, renameVersion); err != nil {
		t.Fatal(err)
	}
	var after []struct {
		ObjectKey  string
		ObjectType string
		SystemID   int64
	}
	if err := db.Table("product_binding").Order("object_key").Find(&after).Error; err != nil {
		t.Fatal(err)
	}
	if len(bindings) == 0 || !reflect.DeepEqual(bindings, after) {
		t.Fatalf("bindings changed: %v -> %v", bindings, after)
	}
	var name string
	if err := db.Raw("SELECT name FROM product_item WHERE id=1 AND product_no='preserved'").Scan(&name).Error; err != nil || name != "existing" {
		t.Fatalf("product data: %q %v", name, err)
	}
	var events int64
	if err := db.Table("product_event").Where("product_id=1 AND actor_id=1 AND action='create'").Count(&events).Error; err != nil || events != 1 {
		t.Fatalf("events: %d %v", events, err)
	}
	var versions int64
	if err := db.Table("goose_db_version").Count(&versions).Error; err != nil || versions != history+1 {
		t.Fatalf("history: %d -> %d %v", history, versions, err)
	}
	if _, err := provider.UpTo(ctx, renameVersion); err != nil {
		t.Fatal(err)
	}
	var repeated int64
	if err := db.Table("goose_db_version").Count(&repeated).Error; err != nil || repeated != versions {
		t.Fatalf("repeat versions: %d %v", repeated, err)
	}
	// The renamed sequence/autoincrement still assigns the next identity.
	if err := db.Exec("INSERT INTO product_item(product_no,name,dept_id,create_by) VALUES('next','next',1,1)").Error; err != nil {
		t.Fatal(err)
	}
	var next int64
	if err := db.Raw("SELECT id FROM product_item WHERE product_no='next'").Scan(&next).Error; err != nil || next <= 1 {
		t.Fatalf("next identity: %d %v", next, err)
	}
	for _, table := range []string{"biz_product", "biz_product_event", "biz_admin_binding"} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("historical table remains: %s", table)
		}
	}
	upgraded := databaseStructure(t, db)
	testutil.ResetSchema(t, db)
	provider, _ = applicationMigrationProvider(t, db)
	if _, err := provider.UpTo(ctx, renameVersion); err != nil {
		t.Fatal(err)
	}
	if fresh := databaseStructure(t, db); !reflect.DeepEqual(fresh, upgraded) {
		t.Fatalf("fresh structure=%v upgraded=%v", fresh, upgraded)
	}
}

// Full application verification follows the current migration set and compares
// schema/history, independently of historical product rows and permissions.
func TestApplicationMigrationsFreshUpgradeAndRepeat(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	ctx := context.Background()
	provider, menuVersion := applicationMigrationProvider(t, db)
	if _, err := provider.UpTo(ctx, menuVersion); err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	upgraded := databaseStructure(t, db)
	var history []int64
	if err := db.Table("goose_db_version").Order("id").Pluck("version_id", &history).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	var repeated []int64
	if err := db.Table("goose_db_version").Order("id").Pluck("version_id", &repeated).Error; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(history, repeated) {
		t.Fatalf("repeat history: %v -> %v", history, repeated)
	}
	testutil.ResetSchema(t, db)
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	if fresh := databaseStructure(t, db); !reflect.DeepEqual(fresh, upgraded) {
		t.Fatalf("fresh structure=%v upgraded=%v", fresh, upgraded)
	}
}

func applicationMigrationProvider(t *testing.T, db *gorm.DB) (*goose.Provider, int64) {
	t.Helper()
	driver := database.DriverOf(db)
	seed, menus := postgres.SeedAdminMigration(), postgres.ProductMenusMigration()
	dialect := goose.DialectPostgres
	switch driver {
	case "mysql":
		seed, menus, dialect = mysql.SeedAdminMigration(), mysql.ProductMenusMigration(), goose.DialectMySQL
	case "mariadb":
		seed, menus, dialect = mariadb.SeedAdminMigration(), mariadb.ProductMenusMigration(), goose.DialectMySQL
	case "sqlite":
		seed, menus, dialect = sqlite.SeedAdminMigration(), sqlite.ProductMenusMigration(), goose.DialectSQLite3
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(dialect, pool, os.DirFS(driver), goose.WithDisableGlobalRegistry(true), goose.WithGoMigrations(seed, menus))
	if err != nil {
		t.Fatal(err)
	}

	return provider, menus.Version
}

func databaseStructure(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var result []string
	tables, err := db.Migrator().GetTables()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		columns, err := db.Migrator().ColumnTypes(table)
		if err != nil {
			t.Fatal(err)
		}
		for _, column := range columns {
			nullable, _ := column.Nullable()
			primary, _ := column.PrimaryKey()
			value, hasDefault := column.DefaultValue()
			result = append(result, fmt.Sprintf("%s.%s:%s:%t:%t:%t:%s", table, column.Name(), column.DatabaseTypeName(), nullable, primary, hasDefault, value))
		}
	}
	sort.Strings(result)
	return result
}
