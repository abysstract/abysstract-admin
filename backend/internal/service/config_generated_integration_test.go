//go:build integration

package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func setupGeneratedConfigHarness(t *testing.T, mutateHooks func(*resource.HookSet[model.SysConfig])) generatedConfigHarness {
	t.Helper()
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	baseRepo := repo.New(gdb)
	inner := &countingConfigRepository{inner: resourcegen.NewConfigRepository()}
	cachedRepo, err := newConfigCachedRepository(
		inner,
		resource.NewVersionedCache[resourcegen.ConfigListCacheKey, configListCacheEntry](),
	)
	if err != nil {
		t.Fatalf("newConfigCachedRepository: %v", err)
	}
	hooks := newConfigCacheHooks(cachedRepo)
	if mutateHooks != nil {
		mutateHooks(hooks)
	}
	controller, err := resourcegen.NewConfigController(resourcegen.ConfigControllerDeps{
		Store:      testutil.Store(t, baseRepo),
		Hooks:      hooks,
		Repository: cachedRepo,
	})
	if err != nil {
		t.Fatalf("NewConfigController: %v", err)
	}
	adapter, err := resourcegen.NewConfigServiceAdapter(controller)
	if err != nil {
		t.Fatalf("NewConfigServiceAdapter: %v", err)
	}
	return generatedConfigHarness{svc: adapter, counter: inner, db: gdb}
}

func setupGeneratedConfigAdapter(t *testing.T, mutateHooks func(*resource.HookSet[model.SysConfig])) (adminv1connect.ConfigServiceHandler, *countingConfigRepository) {
	t.Helper()
	h := setupGeneratedConfigHarness(t, mutateHooks)
	return h.svc, h.counter
}

func TestGeneratedConfigServiceCommittedWritesInvalidateConfigCache(t *testing.T) {
	cases := []struct {
		name  string
		write func(*testing.T, adminv1connect.ConfigServiceHandler, generatedConfigFixture)
	}{
		{
			name: "create",
			write: func(t *testing.T, svc adminv1connect.ConfigServiceHandler, _ generatedConfigFixture) {
				t.Helper()
				if _, err := svc.CreateConfig(context.Background(), &adminv1.CreateConfigRequest{ConfigName: "缓存参数三", ConfigKey: "cache.config.three", ConfigValue: "three"}); err != nil {
					t.Fatalf("CreateConfig: %v", err)
				}
			},
		},
		{
			name: "update",
			write: func(t *testing.T, svc adminv1connect.ConfigServiceHandler, f generatedConfigFixture) {
				t.Helper()
				if _, err := svc.UpdateConfig(context.Background(), &adminv1.UpdateConfigRequest{Id: f.secondID, ConfigValue: proto.String("updated")}); err != nil {
					t.Fatalf("UpdateConfig: %v", err)
				}
			},
		},
		{
			name: "delete",
			write: func(t *testing.T, svc adminv1connect.ConfigServiceHandler, f generatedConfigFixture) {
				t.Helper()
				if _, err := svc.DeleteConfig(context.Background(), &adminv1.DeleteConfigRequest{Id: f.secondID}); err != nil {
					t.Fatalf("DeleteConfig: %v", err)
				}
			},
		},
		{
			name: "batch_delete",
			write: func(t *testing.T, svc adminv1connect.ConfigServiceHandler, f generatedConfigFixture) {
				t.Helper()
				if _, err := svc.BatchDeleteConfigs(context.Background(), &adminv1.BatchDeleteConfigsRequest{Ids: []int64{f.secondID}}); err != nil {
					t.Fatalf("BatchDeleteConfigs: %v", err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, counter := setupGeneratedConfigAdapter(t, nil)
			fixture := seedGeneratedConfigFixture(t, svc)
			warmGeneratedConfigList(t, svc, counter, 1)
			tc.write(t, svc, fixture)
			warmGeneratedConfigList(t, svc, counter, 2)
		})
	}
}

func TestGeneratedConfigServiceRollbackDoesNotInvalidateConfigCache(t *testing.T) {
	errRollback := errors.New("rollback after config update hook")
	svc, counter := setupGeneratedConfigAdapter(t, func(hooks *resource.HookSet[model.SysConfig]) {
		hooks.AfterUpdate = func(context.Context, resource.Tx, *model.SysConfig, *model.SysConfig, repo.ColumnSet) error {
			return errRollback
		}
	})
	fixture := seedGeneratedConfigFixture(t, svc)
	warmGeneratedConfigList(t, svc, counter, 1)
	_, err := svc.UpdateConfig(context.Background(), &adminv1.UpdateConfigRequest{Id: fixture.secondID, ConfigValue: proto.String("rollback")})
	if !errors.Is(err, errRollback) {
		t.Fatalf("UpdateConfig rollback error = %v, want %v", err, errRollback)
	}
	warmGeneratedConfigList(t, svc, counter, 1)
}

func TestGeneratedConfigServiceUpdateDatabaseSemantics(t *testing.T) {
	t.Run("empty_patch_is_invalid_before_lookup", func(t *testing.T) {
		h := setupGeneratedConfigHarness(t, nil)
		fixture := seedGeneratedConfigFixture(t, h.svc)
		before := mustLoadGeneratedConfig(t, h.db, fixture.secondID)

		_, err := h.svc.UpdateConfig(context.Background(), &adminv1.UpdateConfigRequest{Id: fixture.secondID})
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("empty UpdateConfig code = %v, want InvalidArgument (err=%v)", connect.CodeOf(err), err)
		}
		if got := h.counter.getByKeyForUpdateCalls.Load(); got != 0 {
			t.Fatalf("empty patch lock reads = %d, want 0", got)
		}
		if got := h.counter.updateCalls.Load(); got != 0 {
			t.Fatalf("empty patch updates = %d, want 0", got)
		}
		after := mustLoadGeneratedConfig(t, h.db, fixture.secondID)
		if after != before {
			t.Fatalf("empty patch changed row\nbefore=%+v\nafter=%+v", before, after)
		}
	})

	t.Run("selective_update_persists_only_changed_column_and_updated_at", func(t *testing.T) {
		h := setupGeneratedConfigHarness(t, nil)
		fixture := seedGeneratedConfigFixture(t, h.svc)
		before := mustLoadGeneratedConfig(t, h.db, fixture.secondID)

		time.Sleep(2 * time.Millisecond)
		resp, err := h.svc.UpdateConfig(context.Background(), &adminv1.UpdateConfigRequest{
			Id:          fixture.secondID,
			ConfigValue: proto.String("updated-value"),
		})
		if err != nil {
			t.Fatalf("UpdateConfig: %v", err)
		}
		after := mustLoadGeneratedConfig(t, h.db, fixture.secondID)
		if after.ConfigValue != "updated-value" {
			t.Fatalf("updated config_value = %q, want updated-value", after.ConfigValue)
		}
		if after.ConfigName != before.ConfigName || after.ConfigKey != before.ConfigKey || after.ConfigType != before.ConfigType || after.Remark != before.Remark {
			t.Fatalf("selective update changed untouched columns\nbefore=%+v\nafter=%+v", before, after)
		}
		if !after.UpdateTime.After(before.UpdateTime) {
			t.Fatalf("UpdatedAt did not advance: before=%v after=%v", before.UpdateTime, after.UpdateTime)
		}
		if got := resp.GetItem().GetUpdateTime().AsTime(); !got.Equal(after.UpdateTime) {
			t.Fatalf("response UpdatedAt = %v, want persisted %v", got, after.UpdateTime)
		}
		if got := h.counter.getByKeyForUpdateCalls.Load(); got != 1 {
			t.Fatalf("selective update lock reads = %d, want 1", got)
		}
		if got := h.counter.updateCalls.Load(); got != 1 {
			t.Fatalf("selective update repository updates = %d, want 1", got)
		}
	})

	t.Run("same_value_patch_returns_current_item_without_update_or_after_commit", func(t *testing.T) {
		var afterCommitCalls atomic.Int64
		h := setupGeneratedConfigHarness(t, func(hooks *resource.HookSet[model.SysConfig]) {
			hooks.AfterCommitUpdate = func(context.Context, model.SysConfig) {
				afterCommitCalls.Add(1)
			}
		})
		fixture := seedGeneratedConfigFixture(t, h.svc)
		before := mustLoadGeneratedConfig(t, h.db, fixture.secondID)

		time.Sleep(2 * time.Millisecond)
		resp, err := h.svc.UpdateConfig(context.Background(), &adminv1.UpdateConfigRequest{
			Id:          fixture.secondID,
			ConfigValue: proto.String(before.ConfigValue),
		})
		if err != nil {
			t.Fatalf("same-value UpdateConfig: %v", err)
		}
		after := mustLoadGeneratedConfig(t, h.db, fixture.secondID)
		if after != before {
			t.Fatalf("same-value patch changed row\nbefore=%+v\nafter=%+v", before, after)
		}
		if got := resp.GetItem().GetUpdateTime().AsTime(); !got.Equal(before.UpdateTime) {
			t.Fatalf("same-value response UpdatedAt = %v, want unchanged %v", got, before.UpdateTime)
		}
		if got := h.counter.getByKeyForUpdateCalls.Load(); got != 1 {
			t.Fatalf("same-value patch lock reads = %d, want 1", got)
		}
		if got := h.counter.updateCalls.Load(); got != 0 {
			t.Fatalf("same-value patch repository updates = %d, want 0", got)
		}
		if got := afterCommitCalls.Load(); got != 0 {
			t.Fatalf("same-value patch AfterCommitUpdate calls = %d, want 0", got)
		}
	})
}

func TestGeneratedConfigCreateNarrowingOverflowDoesNotWrite(t *testing.T) {
	h := setupGeneratedConfigHarness(t, nil)
	_, err := h.svc.CreateConfig(context.Background(), &adminv1.CreateConfigRequest{
		ConfigName:  "overflow",
		ConfigKey:   "overflow.config_type",
		ConfigValue: "value",
		ConfigType:  65536,
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateConfig overflow code = %v, want InvalidArgument (err=%v)", connect.CodeOf(err), err)
	}
	if got := h.counter.createCalls.Load(); got != 0 {
		t.Fatalf("overflow Create repository calls = %d, want 0", got)
	}
	var count int64
	if err := h.db.WithContext(context.Background()).Model(&model.SysConfig{}).Where("config_key = ?", "overflow.config_type").Count(&count).Error; err != nil {
		t.Fatalf("count overflow config: %v", err)
	}
	if count != 0 {
		t.Fatalf("overflow Create persisted %d rows, want 0", count)
	}
}

func TestGeneratedConfigServiceHookAndBatchDatabaseSemantics(t *testing.T) {
	t.Run("update_rollback_and_after_commit", func(t *testing.T) {
		rollback := errors.New("rollback config update")
		var afterCommitCalls atomic.Int64
		h := setupGeneratedConfigHarness(t, func(hooks *resource.HookSet[model.SysConfig]) {
			hooks.AfterUpdate = func(context.Context, resource.Tx, *model.SysConfig, *model.SysConfig, repo.ColumnSet) error {
				return rollback
			}
			hooks.AfterCommitUpdate = func(context.Context, model.SysConfig) { afterCommitCalls.Add(1) }
		})
		fixture := seedGeneratedConfigFixture(t, h.svc)
		before := mustLoadGeneratedConfig(t, h.db, fixture.secondID)
		_, err := h.svc.UpdateConfig(context.Background(), &adminv1.UpdateConfigRequest{Id: fixture.secondID, ConfigValue: proto.String("rollback")})
		if !errors.Is(err, rollback) {
			t.Fatalf("UpdateConfig rollback error = %v, want %v", err, rollback)
		}
		after := mustLoadGeneratedConfig(t, h.db, fixture.secondID)
		if after != before {
			t.Fatalf("Update rollback changed row\nbefore=%+v\nafter=%+v", before, after)
		}
		if got := afterCommitCalls.Load(); got != 0 {
			t.Fatalf("Update rollback AfterCommit calls = %d, want 0", got)
		}
	})

	t.Run("update_success_after_commit_observes_persisted_row", func(t *testing.T) {
		var afterCommitCalls atomic.Int64
		var snapshot model.SysConfig
		var commitDB *gorm.DB
		h := setupGeneratedConfigHarness(t, func(hooks *resource.HookSet[model.SysConfig]) {
			hooks.AfterCommitUpdate = func(_ context.Context, item model.SysConfig) {
				afterCommitCalls.Add(1)
				snapshot = item
				persisted := mustLoadGeneratedConfig(t, commitDB, item.ID)
				if persisted.ConfigValue != item.ConfigValue || !persisted.UpdateTime.Equal(item.UpdateTime) {
					t.Fatalf("Update AfterCommit snapshot not persisted\nsnapshot=%+v\npersisted=%+v", item, persisted)
				}
			}
		})
		commitDB = h.db
		fixture := seedGeneratedConfigFixture(t, h.svc)
		resp, err := h.svc.UpdateConfig(context.Background(), &adminv1.UpdateConfigRequest{Id: fixture.secondID, ConfigValue: proto.String("committed")})
		if err != nil {
			t.Fatalf("UpdateConfig success: %v", err)
		}
		if got := afterCommitCalls.Load(); got != 1 {
			t.Fatalf("Update AfterCommit calls = %d, want 1", got)
		}
		if snapshot.ConfigValue != "committed" {
			t.Fatalf("Update AfterCommit snapshot value = %q, want committed", snapshot.ConfigValue)
		}
		if got := resp.GetItem().GetUpdateTime().AsTime(); !got.Equal(snapshot.UpdateTime) {
			t.Fatalf("Update response UpdatedAt = %v, want AfterCommit snapshot %v", got, snapshot.UpdateTime)
		}
	})

	t.Run("delete_rollback_and_after_commit_snapshot", func(t *testing.T) {
		rollback := errors.New("rollback config delete")
		var afterCommitCalls atomic.Int64
		h := setupGeneratedConfigHarness(t, func(hooks *resource.HookSet[model.SysConfig]) {
			hooks.AfterDelete = func(context.Context, resource.Tx, *model.SysConfig) error { return rollback }
			hooks.AfterCommitDelete = func(context.Context, model.SysConfig) { afterCommitCalls.Add(1) }
		})
		fixture := seedGeneratedConfigFixture(t, h.svc)
		_, err := h.svc.DeleteConfig(context.Background(), &adminv1.DeleteConfigRequest{Id: fixture.secondID})
		if !errors.Is(err, rollback) {
			t.Fatalf("DeleteConfig rollback error = %v, want %v", err, rollback)
		}
		_ = mustLoadGeneratedConfig(t, h.db, fixture.secondID)
		if got := afterCommitCalls.Load(); got != 0 {
			t.Fatalf("Delete rollback AfterCommit calls = %d, want 0", got)
		}
	})

	t.Run("delete_success_after_commit_observes_committed_snapshot", func(t *testing.T) {
		var afterCommitCalls atomic.Int64
		var snapshot model.SysConfig
		h := setupGeneratedConfigHarness(t, func(hooks *resource.HookSet[model.SysConfig]) {
			hooks.AfterDelete = func(_ context.Context, _ resource.Tx, old *model.SysConfig) error {
				old.ConfigName = "hook-mutated"
				return nil
			}
			hooks.AfterCommitDelete = func(_ context.Context, item model.SysConfig) {
				afterCommitCalls.Add(1)
				snapshot = item
			}
		})
		fixture := seedGeneratedConfigFixture(t, h.svc)
		before := mustLoadGeneratedConfig(t, h.db, fixture.secondID)
		if _, err := h.svc.DeleteConfig(context.Background(), &adminv1.DeleteConfigRequest{Id: fixture.secondID}); err != nil {
			t.Fatalf("DeleteConfig: %v", err)
		}
		assertGeneratedConfigMissing(t, h.db, fixture.secondID)
		if got := afterCommitCalls.Load(); got != 1 {
			t.Fatalf("Delete AfterCommit calls = %d, want 1", got)
		}
		if snapshot.ConfigName != before.ConfigName {
			t.Fatalf("Delete AfterCommit snapshot name = %q, want %q", snapshot.ConfigName, before.ConfigName)
		}
	})

	t.Run("batch_delete_rollback_and_not_found_are_all_or_nothing", func(t *testing.T) {
		rollback := errors.New("rollback config batch delete")
		var afterCommitCalls atomic.Int64
		h := setupGeneratedConfigHarness(t, func(hooks *resource.HookSet[model.SysConfig]) {
			hooks.AfterBatchDelete = func(context.Context, resource.Tx, []*model.SysConfig) error { return rollback }
			hooks.AfterCommitBatchDelete = func(context.Context, []model.SysConfig) { afterCommitCalls.Add(1) }
		})
		fixture := seedGeneratedConfigFixture(t, h.svc)
		_, err := h.svc.BatchDeleteConfigs(context.Background(), &adminv1.BatchDeleteConfigsRequest{Ids: []int64{fixture.firstID, fixture.secondID}})
		if !errors.Is(err, rollback) {
			t.Fatalf("BatchDeleteConfigs rollback error = %v, want %v", err, rollback)
		}
		_ = mustLoadGeneratedConfig(t, h.db, fixture.firstID)
		_ = mustLoadGeneratedConfig(t, h.db, fixture.secondID)
		if got := afterCommitCalls.Load(); got != 0 {
			t.Fatalf("Batch rollback AfterCommit calls = %d, want 0", got)
		}

		clean := setupGeneratedConfigHarness(t, nil)
		fixture = seedGeneratedConfigFixture(t, clean.svc)
		_, err = clean.svc.BatchDeleteConfigs(context.Background(), &adminv1.BatchDeleteConfigsRequest{Ids: []int64{fixture.firstID, 999999999}})
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("BatchDeleteConfigs missing code = %v, want NotFound (err=%v)", connect.CodeOf(err), err)
		}
		_ = mustLoadGeneratedConfig(t, clean.db, fixture.firstID)
		_ = mustLoadGeneratedConfig(t, clean.db, fixture.secondID)
	})
}
