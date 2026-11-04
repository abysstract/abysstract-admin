//go:build integration

package migrations_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/abysstract/abysstract-admin/backend/migrations"
	"github.com/abysstract/abysstract-admin/backend/testdb"
)

func TestSQLiteBaselineContract(t *testing.T) {
	ctx := context.Background()
	run := "22222222222222222222222222222222"
	cfg, err := testdb.CreateSQLite(ctx, run, filepath.Join(t.TempDir(), "admin_core_k2_"+run))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := testdb.DropSQLite(cfg); err != nil {
			t.Error(err)
		}
	}()
	db, err := testdb.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	pool.SetMaxOpenConns(1)
	pool.SetMaxIdleConns(1)
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	var firstVersionRows int64
	if err := db.Table("goose_db_version").Count(&firstVersionRows).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	var tables []struct{ Name, SQL string }
	if err := db.Raw("SELECT name, sql FROM sqlite_master WHERE type='table' AND name LIKE 'sys_%' ORDER BY name").Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		t.Logf("%s: %s", table.Name, table.SQL)
	}
	var admin struct {
		ID         int64
		Password   string
		CreateTime time.Time
	}
	if err := db.Table("sys_user").Where("username = ?", "admin").Take(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte("admin123")); err != nil {
		t.Fatal(err)
	}
	var versionRows int64
	if err := db.Table("goose_db_version").Count(&versionRows).Error; err != nil || versionRows != firstVersionRows {
		t.Fatalf("versions=%d err=%v", versionRows, err)
	}
	var defaults struct {
		ID         int64
		LoginDate  *time.Time
		CreateTime time.Time
		Status     int16
	}
	if err := db.Raw("INSERT INTO sys_user(username) VALUES ('alice') RETURNING id,login_date,create_time,status").Scan(&defaults).Error; err != nil {
		t.Fatal(err)
	}
	if defaults.ID <= admin.ID || defaults.LoginDate != nil || defaults.CreateTime.IsZero() || defaults.Status != 1 {
		t.Fatalf("defaults: %+v", defaults)
	}
	t.Logf("admin=%d next=%d default=%s nullable=%v status=%d versions=%d", admin.ID, defaults.ID, defaults.CreateTime.Format(time.RFC3339Nano), defaults.LoginDate, defaults.Status, versionRows)
	for i := 0; i < 2; i++ {
		if err := db.Exec("UPDATE sys_user SET del_flag=1 WHERE username='alice' AND del_flag=0").Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("INSERT INTO sys_user(username) VALUES ('alice')").Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Exec("INSERT INTO sys_role(role_name,role_key) VALUES ('A','KeyA'),('a','keya')").Error; err != nil {
		t.Fatal(err)
	}
}
