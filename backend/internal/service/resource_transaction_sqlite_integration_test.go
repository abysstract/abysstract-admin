//go:build integration

package service

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

// This exercises the actual generated controller and system cache decorator on
// an in-memory fixture. PostgreSQL lock/timestamp checks remain a separate gate.
func TestGeneratedConfigPublicTransactionWithSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	t.Cleanup(func() { _ = pool.Close() })
	pool.SetMaxOpenConns(1)
	for _, sql := range []string{
		`CREATE TABLE sys_config (id INTEGER PRIMARY KEY AUTOINCREMENT, config_name TEXT, config_key TEXT UNIQUE, config_value TEXT, config_type INTEGER, remark TEXT, create_time DATETIME DEFAULT CURRENT_TIMESTAMP, update_time DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE config_events (name TEXT)`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	store, err := resource.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	cache := resource.NewVersionedCache[resourcegen.ConfigListCacheKey, configListCacheEntry]()
	cached, err := newConfigCachedRepository(resourcegen.NewConfigRepository(), cache)
	if err != nil {
		t.Fatal(err)
	}
	var handle *gorm.DB
	before, after, committed := 0, 0, 0
	fail := false
	system := resource.ComposeHooks(newConfigCacheHooks(cached), &resource.HookSet[model.SysConfig]{
		BeforeCreate: func(ctx context.Context, tx resource.Tx, m *model.SysConfig) error {
			before++
			handle = tx.DB()
			internal := repo.New(tx.DB())
			if internal.DB() != handle {
				t.Fatal("internal wrapper changed transaction")
			}
			return internal.DB().WithContext(ctx).Exec("INSERT INTO config_events VALUES (?)", "system").Error
		},
		AfterCreate: func(_ context.Context, tx resource.Tx, _ *model.SysConfig) error {
			if tx.DB() != handle {
				t.Fatal("after hook transaction changed")
			}
			after++
			return nil
		},
	})
	business := &resource.HookSet[model.SysConfig]{
		BeforeCreate: func(ctx context.Context, tx resource.Tx, _ *model.SysConfig) error {
			if tx.DB() != handle {
				t.Fatal("business hook transaction changed")
			}
			return tx.DB().WithContext(ctx).Exec("INSERT INTO config_events VALUES (?)", "business").Error
		},
		AfterCreate: func(context.Context, resource.Tx, *model.SysConfig) error {
			if fail {
				return errors.New("rollback business")
			}
			return nil
		},
		AfterCommitCreate: func(_ context.Context, m model.SysConfig) {
			committed++
			var count int64
			if err := db.Model(&model.SysConfig{}).Where("id = ?", m.ID).Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("commit visibility: %d %v", count, err)
			}
			if cache.Version() != uint64(committed) {
				t.Fatal("mandatory invalidation count changed")
			}
			panic("business notification failed after commit")
		},
	}
	controller, err := resourcegen.NewConfigController(resourcegen.ConfigControllerDeps{Store: store, Repository: cached, Hooks: resource.ComposeHooks(system, business)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.CreateConfig(context.Background(), &adminv1.CreateConfigRequest{ConfigName: "committed", ConfigKey: "committed", ConfigValue: "value"}); err != nil {
		t.Fatal(err)
	}
	fail = true
	if _, err := controller.CreateConfig(context.Background(), &adminv1.CreateConfigRequest{ConfigName: "rolled back", ConfigKey: "rollback", ConfigValue: "value"}); err == nil {
		t.Fatal("transaction error was lost")
	}
	var rows, events int64
	if err := db.Model(&model.SysConfig{}).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("config_events").Count(&events).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 1 || events != 2 || before != 2 || after != 1 || committed != 1 || cache.Version() != 1 {
		t.Fatalf("rows=%d events=%d before=%d after=%d committed=%d epoch=%d", rows, events, before, after, committed, cache.Version())
	}
}
