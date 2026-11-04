package producttest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/abysstract/abysstract-admin/backend/adminconfig"
	"github.com/abysstract/abysstract-admin/backend/migrations"
	"github.com/abysstract/abysstract-admin/backend/testdb"
)

var migrateMu sync.Mutex

// Open admitted disposable PostgreSQL and applies system then product migrations.
func OpenMigrated(t *testing.T) *gorm.DB {
	t.Helper()
	migrateMu.Lock()
	t.Cleanup(migrateMu.Unlock)

	config, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	db, err := testdb.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	resetSchema(t, db, config)
	if err := migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// OpenEmpty admits the disposable database with an empty public schema.
func OpenEmpty(t *testing.T) *gorm.DB {
	t.Helper()
	migrateMu.Lock()
	t.Cleanup(migrateMu.Unlock)

	config, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	db, err := testdb.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	resetSchema(t, db, config)
	return db
}

func resetSchema(t *testing.T, db *gorm.DB, config testdb.Config) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := testdb.Verify(ctx, db, config); err != nil {
		t.Fatal(err)
	}
	if err := testdb.ResetSchema(ctx, db, config); err != nil {
		t.Fatal(err)
	}
}

// TestConfig returns a valid public application config for the disposable database.
func TestConfig(t *testing.T, dsn string) adminconfig.Config {
	t.Helper()
	cfg, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	return adminconfig.Config{
		Env:      adminconfig.EnvDev,
		Addr:     "127.0.0.1:0",
		DBDriver: cfg.Driver, DBDSN: dsn,
		JWTSecret:           "product-example-test-secret",
		JWTExpiresIn:        time.Hour,
		CORSOrigins:         []string{"http://localhost:5173"},
		ShutdownTimeout:     5 * time.Second,
		DBMaxOpenConns:      5,
		DBMaxIdleConns:      2,
		LoginMaxFailures:    5,
		LoginLockoutSeconds: 900,
		UploadMaxBytes:      1 << 20,
		AvatarMaxBytes:      1 << 20,
		UploadDir:           t.TempDir(),
	}
}

// CurrentDSN returns the admitted DSN for the current process.
func CurrentDSN(t *testing.T) string {
	t.Helper()
	config, err := testdb.FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	return config.DSN
}

func Count(t *testing.T, db *gorm.DB, sql string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.Raw(sql, args...).Scan(&n).Error; err != nil {
		t.Fatalf("count %s: %v", sql, err)
	}
	return n
}

func MustExec(t *testing.T, db *gorm.DB, sql string, args ...any) {
	t.Helper()
	if err := db.Exec(sql, args...).Error; err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func QueryInt64(t *testing.T, db *gorm.DB, sql string, args ...any) int64 {
	t.Helper()
	var value int64
	if err := db.Raw(sql, args...).Scan(&value).Error; err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return value
}

func QueryString(t *testing.T, db *gorm.DB, sql string, args ...any) string {
	t.Helper()
	var value string
	if err := db.Raw(sql, args...).Scan(&value).Error; err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return value
}

func SnapshotUserRoles(t *testing.T, db *gorm.DB, userID int64) []int64 {
	t.Helper()
	var ids []int64
	if err := db.Raw(`SELECT role_id FROM sys_user_role WHERE user_id = ? ORDER BY role_id`, userID).Scan(&ids).Error; err != nil {
		t.Fatal(err)
	}
	if ids == nil {
		return []int64{}
	}
	return ids
}

func FormatIDs(ids []int64) string {
	return fmt.Sprintf("%v", ids)
}
