//go:build integration

package service

import (
	"context"
	"errors"
	"reflect"
	"sync"
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

func TestGeneratedDictDataCacheRejectsStaleReadAfterCommittedUpdate(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once

	h := setupGeneratedDictHarness(t, generatedDictHookMutators{})
	fixture := seedGeneratedDictFixture(t, h.svc)
	h.counter.afterList = func() {
		once.Do(func() {
			close(started)
			<-release
		})
	}

	query := &adminv1.ListDictDataRequest{DictType: proto.String(fixture.type1)}
	errCh := make(chan error, 1)
	go func() {
		_, err := h.svc.ListDictData(context.Background(), query)
		errCh <- err
	}()
	<-started // the old DB result has been read, but cache fill is still paused

	const updatedLabel = "已提交的新缓存值"
	if _, err := h.svc.UpdateDictData(context.Background(), &adminv1.UpdateDictDataRequest{
		Id: fixture.data1ID, DictLabel: proto.String(updatedLabel),
	}); err != nil {
		t.Fatalf("UpdateDictData: %v", err)
	}
	close(release)
	if err := <-errCh; err != nil {
		t.Fatalf("stale ListDictData: %v", err)
	}

	fresh, err := h.svc.ListDictData(context.Background(), query)
	if err != nil {
		t.Fatalf("fresh ListDictData: %v", err)
	}
	if got := fresh.GetItems(); len(got) != 1 || got[0].GetDictLabel() != updatedLabel {
		t.Fatalf("fresh ListDictData returned stale entry: %+v", got)
	}
	if got := h.counter.listCalls.Load(); got != 2 {
		t.Fatalf("stale result must not fill cache; inner List calls = %d, want 2", got)
	}
}

func setupGeneratedDictHarness(t *testing.T, mutate generatedDictHookMutators) generatedDictHarness {
	t.Helper()
	gdb := testutil.OpenDB(t)
	testutil.ResetSchema(t, gdb)
	if err := testutil.Migrate(gdb); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	baseRepo := repo.New(gdb)
	inner := &countingDictDataRepository{inner: resourcegen.NewDictDataRepository()}
	cachedRepo, err := newDictDataCachedRepository(
		inner,
		resource.NewVersionedCache[resourcegen.DictDataListCacheKey, dictDataListCacheEntry](),
	)
	if err != nil {
		t.Fatalf("newDictDataCachedRepository: %v", err)
	}
	typeHooks := newDictTypeCacheHooks(cachedRepo)
	dataHooks := newDictDataCacheHooks(cachedRepo)
	if mutate.typeHooks != nil {
		mutate.typeHooks(typeHooks)
	}
	if mutate.dataHooks != nil {
		mutate.dataHooks(dataHooks)
	}
	dictTypeController, err := resourcegen.NewDictTypeController(resourcegen.DictTypeControllerDeps{Store: testutil.Store(t, baseRepo), Hooks: typeHooks})
	if err != nil {
		t.Fatalf("NewDictTypeController: %v", err)
	}
	dictDataController, err := resourcegen.NewDictDataController(resourcegen.DictDataControllerDeps{Store: testutil.Store(t, baseRepo), Hooks: dataHooks, Repository: cachedRepo})
	if err != nil {
		t.Fatalf("NewDictDataController: %v", err)
	}
	adapter, err := resourcegen.NewDictServiceAdapter(dictTypeController, dictDataController)
	if err != nil {
		t.Fatalf("NewDictServiceAdapter: %v", err)
	}
	return generatedDictHarness{svc: adapter, counter: inner, db: gdb}
}

func setupGeneratedDictAdapter(t *testing.T, mutateDataHooks func(*resource.HookSet[model.SysDictDatum])) (adminv1connect.DictServiceHandler, *countingDictDataRepository) {
	t.Helper()
	h := setupGeneratedDictHarness(t, generatedDictHookMutators{dataHooks: mutateDataHooks})
	return h.svc, h.counter
}

func TestGeneratedDictServiceCommittedWritesInvalidateDictDataCache(t *testing.T) {
	cases := []struct {
		name  string
		write func(*testing.T, adminv1connect.DictServiceHandler, generatedDictFixture)
	}{
		{
			name: "dict_data_create",
			write: func(t *testing.T, svc adminv1connect.DictServiceHandler, f generatedDictFixture) {
				t.Helper()
				if _, err := svc.CreateDictData(context.Background(), &adminv1.CreateDictDataRequest{
					DictType: "cache_type_two", DictLabel: "新增旁路数据", DictValue: "side-2", DictSort: 3,
				}); err != nil {
					t.Fatalf("CreateDictData: %v", err)
				}
			},
		},
		{
			name: "dict_data_update",
			write: func(t *testing.T, svc adminv1connect.DictServiceHandler, f generatedDictFixture) {
				t.Helper()
				if _, err := svc.UpdateDictData(context.Background(), &adminv1.UpdateDictDataRequest{Id: f.data2ID, DictLabel: proto.String("旁路数据已更新")}); err != nil {
					t.Fatalf("UpdateDictData: %v", err)
				}
			},
		},
		{
			name: "dict_data_delete",
			write: func(t *testing.T, svc adminv1connect.DictServiceHandler, f generatedDictFixture) {
				t.Helper()
				if _, err := svc.DeleteDictData(context.Background(), &adminv1.DeleteDictDataRequest{Id: f.data2ID}); err != nil {
					t.Fatalf("DeleteDictData: %v", err)
				}
			},
		},
		{
			name: "dict_data_batch_delete",
			write: func(t *testing.T, svc adminv1connect.DictServiceHandler, f generatedDictFixture) {
				t.Helper()
				if _, err := svc.BatchDeleteDictData(context.Background(), &adminv1.BatchDeleteDictDataRequest{Ids: []int64{f.data2ID}}); err != nil {
					t.Fatalf("BatchDeleteDictData: %v", err)
				}
			},
		},
		{
			name: "dict_type_create",
			write: func(t *testing.T, svc adminv1connect.DictServiceHandler, f generatedDictFixture) {
				t.Helper()
				if _, err := svc.CreateDictType(context.Background(), &adminv1.CreateDictTypeRequest{DictName: "新增类型", DictType: "cache_type_three"}); err != nil {
					t.Fatalf("CreateDictType: %v", err)
				}
			},
		},
		{
			name: "dict_type_update",
			write: func(t *testing.T, svc adminv1connect.DictServiceHandler, f generatedDictFixture) {
				t.Helper()
				if _, err := svc.UpdateDictType(context.Background(), &adminv1.UpdateDictTypeRequest{Id: f.type2ID, DictName: proto.String("缓存类型二已更新")}); err != nil {
					t.Fatalf("UpdateDictType: %v", err)
				}
			},
		},
		{
			name: "dict_type_delete",
			write: func(t *testing.T, svc adminv1connect.DictServiceHandler, f generatedDictFixture) {
				t.Helper()
				if _, err := svc.DeleteDictType(context.Background(), &adminv1.DeleteDictTypeRequest{Id: f.type2ID}); err != nil {
					t.Fatalf("DeleteDictType: %v", err)
				}
			},
		},
		{
			name: "dict_type_batch_delete",
			write: func(t *testing.T, svc adminv1connect.DictServiceHandler, f generatedDictFixture) {
				t.Helper()
				if _, err := svc.BatchDeleteDictTypes(context.Background(), &adminv1.BatchDeleteDictTypesRequest{Ids: []int64{f.type2ID}}); err != nil {
					t.Fatalf("BatchDeleteDictTypes: %v", err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, counter := setupGeneratedDictAdapter(t, nil)
			fixture := seedGeneratedDictFixture(t, svc)
			warmGeneratedDictDataList(t, svc, counter, fixture.type1, 1)
			tc.write(t, svc, fixture)
			if _, err := svc.ListDictData(context.Background(), &adminv1.ListDictDataRequest{DictType: proto.String(fixture.type1)}); err != nil {
				t.Fatalf("post-write ListDictData: %v", err)
			}
			if got := counter.listCalls.Load(); got != 2 {
				t.Fatalf("committed write should invalidate cache; inner list calls = %d, want 2", got)
			}
		})
	}
}

func TestGeneratedDictServiceRollbackDoesNotInvalidateDictDataCache(t *testing.T) {
	errRollback := errors.New("rollback after update hook")
	svc, counter := setupGeneratedDictAdapter(t, func(hooks *resource.HookSet[model.SysDictDatum]) {
		hooks.AfterUpdate = func(context.Context, resource.Tx, *model.SysDictDatum, *model.SysDictDatum, repo.ColumnSet) error {
			return errRollback
		}
	})
	fixture := seedGeneratedDictFixture(t, svc)
	warmGeneratedDictDataList(t, svc, counter, fixture.type1, 1)
	_, err := svc.UpdateDictData(context.Background(), &adminv1.UpdateDictDataRequest{Id: fixture.data2ID, DictLabel: proto.String("回滚更新")})
	if !errors.Is(err, errRollback) {
		t.Fatalf("UpdateDictData error = %v, want %v", err, errRollback)
	}
	warmGeneratedDictDataList(t, svc, counter, fixture.type1, 1)
}

func TestGeneratedDictServiceSelectiveUpdatePersistsNormalizedUpdatedAt(t *testing.T) {
	h := setupGeneratedDictHarness(t, generatedDictHookMutators{})
	fixture := seedGeneratedDictFixture(t, h.svc)
	before := mustLoadDictData(t, h.db, fixture.data2ID)

	time.Sleep(2 * time.Millisecond)
	resp, err := h.svc.UpdateDictData(context.Background(), &adminv1.UpdateDictDataRequest{
		Id:        fixture.data2ID,
		DictLabel: proto.String("旁路数据已更新"),
	})
	if err != nil {
		t.Fatalf("UpdateDictData: %v", err)
	}
	after := mustLoadDictData(t, h.db, fixture.data2ID)

	if after.DictLabel != "旁路数据已更新" {
		t.Fatalf("updated dict_label = %q, want %q", after.DictLabel, "旁路数据已更新")
	}
	if after.DictValue != before.DictValue || after.DictSort != before.DictSort || after.DictType != before.DictType || after.IsDefault != before.IsDefault || after.Remark != before.Remark {
		t.Fatalf("selective update changed untouched fields\nbefore=%+v\nafter=%+v", before, after)
	}
	if !after.UpdateTime.After(before.UpdateTime) {
		t.Fatalf("updated_at did not advance: before=%v after=%v", before.UpdateTime, after.UpdateTime)
	}
	if got := resp.GetItem().GetUpdateTime().AsTime(); !got.Equal(after.UpdateTime) {
		t.Fatalf("response updated_at = %v, want persisted %v", got, after.UpdateTime)
	}
}

func TestGeneratedDictServiceNoOpPatchKeepsUpdatedAtAndSkipsAfterCommit(t *testing.T) {
	var afterCommitCalls atomic.Int64
	h := setupGeneratedDictHarness(t, generatedDictHookMutators{dataHooks: func(hooks *resource.HookSet[model.SysDictDatum]) {
		hooks.AfterCommitUpdate = func(context.Context, model.SysDictDatum) {
			afterCommitCalls.Add(1)
		}
	}})
	fixture := seedGeneratedDictFixture(t, h.svc)
	before := mustLoadDictData(t, h.db, fixture.data2ID)

	time.Sleep(2 * time.Millisecond)
	resp, err := h.svc.UpdateDictData(context.Background(), &adminv1.UpdateDictDataRequest{
		Id:        fixture.data2ID,
		DictLabel: proto.String(before.DictLabel),
	})
	if err != nil {
		t.Fatalf("UpdateDictData noop patch: %v", err)
	}
	after := mustLoadDictData(t, h.db, fixture.data2ID)

	if !after.UpdateTime.Equal(before.UpdateTime) {
		t.Fatalf("noop patch changed updated_at: before=%v after=%v", before.UpdateTime, after.UpdateTime)
	}
	if got := resp.GetItem().GetUpdateTime().AsTime(); !got.Equal(before.UpdateTime) {
		t.Fatalf("noop response updated_at = %v, want unchanged %v", got, before.UpdateTime)
	}
	if got := afterCommitCalls.Load(); got != 0 {
		t.Fatalf("noop patch after-commit calls = %d, want 0", got)
	}
}

func TestGeneratedDictServiceCreateOnlyDictTypeRejectsUpdate(t *testing.T) {
	h := setupGeneratedDictHarness(t, generatedDictHookMutators{})
	fixture := seedGeneratedDictFixture(t, h.svc)
	before := mustLoadDictType(t, h.db, fixture.type1ID)

	time.Sleep(2 * time.Millisecond)
	_, err := h.svc.UpdateDictType(context.Background(), &adminv1.UpdateDictTypeRequest{
		Id:       fixture.type1ID,
		DictType: proto.String("changed_type"),
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("UpdateDictType create-only error code = %v, want %v (err=%v)", connect.CodeOf(err), connect.CodeInvalidArgument, err)
	}
	after := mustLoadDictType(t, h.db, fixture.type1ID)
	if after.DictType != before.DictType {
		t.Fatalf("create-only dict_type changed: before=%q after=%q", before.DictType, after.DictType)
	}
	if !after.UpdateTime.Equal(before.UpdateTime) {
		t.Fatalf("create-only rejection changed updated_at: before=%v after=%v", before.UpdateTime, after.UpdateTime)
	}
}

func TestGeneratedDictServiceUpdateHookRollbackAndAfterCommit(t *testing.T) {
	t.Run("rollback", func(t *testing.T) {
		var afterCommitCalls atomic.Int64
		errRollback := errors.New("rollback after update hook")
		h := setupGeneratedDictHarness(t, generatedDictHookMutators{dataHooks: func(hooks *resource.HookSet[model.SysDictDatum]) {
			hooks.AfterUpdate = func(context.Context, resource.Tx, *model.SysDictDatum, *model.SysDictDatum, repo.ColumnSet) error {
				return errRollback
			}
			hooks.AfterCommitUpdate = func(context.Context, model.SysDictDatum) {
				afterCommitCalls.Add(1)
			}
		}})
		fixture := seedGeneratedDictFixture(t, h.svc)
		before := mustLoadDictData(t, h.db, fixture.data2ID)

		_, err := h.svc.UpdateDictData(context.Background(), &adminv1.UpdateDictDataRequest{
			Id:        fixture.data2ID,
			DictLabel: proto.String("不会提交"),
		})
		if !errors.Is(err, errRollback) {
			t.Fatalf("UpdateDictData rollback error = %v, want %v", err, errRollback)
		}
		after := mustLoadDictData(t, h.db, fixture.data2ID)
		if after.DictLabel != before.DictLabel || !after.UpdateTime.Equal(before.UpdateTime) {
			t.Fatalf("rollback update changed row\nbefore=%+v\nafter=%+v", before, after)
		}
		if got := afterCommitCalls.Load(); got != 0 {
			t.Fatalf("rollback after-commit calls = %d, want 0", got)
		}
	})

	t.Run("after_commit", func(t *testing.T) {
		var afterCommitCalls atomic.Int64
		var snapshot model.SysDictDatum
		var commitDB *gorm.DB
		h := setupGeneratedDictHarness(t, generatedDictHookMutators{dataHooks: func(hooks *resource.HookSet[model.SysDictDatum]) {
			hooks.AfterCommitUpdate = func(_ context.Context, m model.SysDictDatum) {
				afterCommitCalls.Add(1)
				snapshot = m
				persisted := mustLoadDictData(t, commitDB, m.ID)
				if persisted.DictLabel != m.DictLabel || !persisted.UpdateTime.Equal(m.UpdateTime) {
					t.Fatalf("after-commit snapshot not committed\nsnapshot=%+v\npersisted=%+v", m, persisted)
				}
			}
		}})
		commitDB = h.db
		fixture := seedGeneratedDictFixture(t, h.svc)

		resp, err := h.svc.UpdateDictData(context.Background(), &adminv1.UpdateDictDataRequest{
			Id:        fixture.data2ID,
			DictLabel: proto.String("提交后的标签"),
		})
		if err != nil {
			t.Fatalf("UpdateDictData success: %v", err)
		}
		if got := afterCommitCalls.Load(); got != 1 {
			t.Fatalf("after-commit calls = %d, want 1", got)
		}
		if snapshot.DictLabel != "提交后的标签" {
			t.Fatalf("after-commit snapshot dict_label = %q, want %q", snapshot.DictLabel, "提交后的标签")
		}
		if got := resp.GetItem().GetUpdateTime().AsTime(); !got.Equal(snapshot.UpdateTime) {
			t.Fatalf("response updated_at = %v, want after-commit snapshot %v", got, snapshot.UpdateTime)
		}
	})
}

func TestGeneratedDictServiceDeleteHookRollbackAndAfterCommitSnapshot(t *testing.T) {
	t.Run("rollback", func(t *testing.T) {
		var afterCommitCalls atomic.Int64
		errRollback := errors.New("rollback after delete hook")
		h := setupGeneratedDictHarness(t, generatedDictHookMutators{dataHooks: func(hooks *resource.HookSet[model.SysDictDatum]) {
			hooks.AfterDelete = func(context.Context, resource.Tx, *model.SysDictDatum) error {
				return errRollback
			}
			hooks.AfterCommitDelete = func(context.Context, model.SysDictDatum) {
				afterCommitCalls.Add(1)
			}
		}})
		fixture := seedGeneratedDictFixture(t, h.svc)

		_, err := h.svc.DeleteDictData(context.Background(), &adminv1.DeleteDictDataRequest{Id: fixture.data2ID})
		if !errors.Is(err, errRollback) {
			t.Fatalf("DeleteDictData rollback error = %v, want %v", err, errRollback)
		}
		_ = mustLoadDictData(t, h.db, fixture.data2ID)
		if got := afterCommitCalls.Load(); got != 0 {
			t.Fatalf("rollback delete after-commit calls = %d, want 0", got)
		}
	})

	t.Run("after_commit_snapshot_ignores_hook_mutation", func(t *testing.T) {
		var afterCommitCalls atomic.Int64
		var snapshot model.SysDictDatum
		h := setupGeneratedDictHarness(t, generatedDictHookMutators{dataHooks: func(hooks *resource.HookSet[model.SysDictDatum]) {
			hooks.AfterDelete = func(_ context.Context, _ resource.Tx, old *model.SysDictDatum) error {
				old.DictLabel = "hook-mutated"
				return nil
			}
			hooks.AfterCommitDelete = func(_ context.Context, m model.SysDictDatum) {
				afterCommitCalls.Add(1)
				snapshot = m
			}
		}})
		fixture := seedGeneratedDictFixture(t, h.svc)
		before := mustLoadDictData(t, h.db, fixture.data2ID)

		if _, err := h.svc.DeleteDictData(context.Background(), &adminv1.DeleteDictDataRequest{Id: fixture.data2ID}); err != nil {
			t.Fatalf("DeleteDictData success: %v", err)
		}
		assertDictDataMissing(t, h.db, fixture.data2ID)
		if got := afterCommitCalls.Load(); got != 1 {
			t.Fatalf("delete after-commit calls = %d, want 1", got)
		}
		if snapshot.DictLabel != before.DictLabel {
			t.Fatalf("after-commit delete snapshot dict_label = %q, want pre-hook %q", snapshot.DictLabel, before.DictLabel)
		}
	})
}

func TestGeneratedDictServiceBatchDeleteHookRollbackAfterCommitAndNotFound(t *testing.T) {
	t.Run("rollback", func(t *testing.T) {
		var afterCommitCalls atomic.Int64
		errRollback := errors.New("rollback after batch delete hook")
		h := setupGeneratedDictHarness(t, generatedDictHookMutators{dataHooks: func(hooks *resource.HookSet[model.SysDictDatum]) {
			hooks.AfterBatchDelete = func(context.Context, resource.Tx, []*model.SysDictDatum) error {
				return errRollback
			}
			hooks.AfterCommitBatchDelete = func(context.Context, []model.SysDictDatum) {
				afterCommitCalls.Add(1)
			}
		}})
		fixture := seedGeneratedDictFixture(t, h.svc)

		_, err := h.svc.BatchDeleteDictData(context.Background(), &adminv1.BatchDeleteDictDataRequest{Ids: []int64{fixture.data1ID, fixture.data2ID}})
		if !errors.Is(err, errRollback) {
			t.Fatalf("BatchDeleteDictData rollback error = %v, want %v", err, errRollback)
		}
		_ = mustLoadDictData(t, h.db, fixture.data1ID)
		_ = mustLoadDictData(t, h.db, fixture.data2ID)
		if got := afterCommitCalls.Load(); got != 0 {
			t.Fatalf("rollback batch-delete after-commit calls = %d, want 0", got)
		}
	})

	t.Run("not_found_is_all_or_nothing", func(t *testing.T) {
		h := setupGeneratedDictHarness(t, generatedDictHookMutators{})
		fixture := seedGeneratedDictFixture(t, h.svc)

		_, err := h.svc.BatchDeleteDictData(context.Background(), &adminv1.BatchDeleteDictDataRequest{Ids: []int64{fixture.data1ID, 999999999}})
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("BatchDeleteDictData error code = %v, want %v (err=%v)", connect.CodeOf(err), connect.CodeNotFound, err)
		}
		_ = mustLoadDictData(t, h.db, fixture.data1ID)
		_ = mustLoadDictData(t, h.db, fixture.data2ID)
	})

	t.Run("after_commit_snapshot_ignores_hook_mutation", func(t *testing.T) {
		var afterCommitCalls atomic.Int64
		var snapshots []model.SysDictDatum
		h := setupGeneratedDictHarness(t, generatedDictHookMutators{dataHooks: func(hooks *resource.HookSet[model.SysDictDatum]) {
			hooks.AfterBatchDelete = func(_ context.Context, _ resource.Tx, models []*model.SysDictDatum) error {
				models[0].DictLabel = "hook-mutated"
				return nil
			}
			hooks.AfterCommitBatchDelete = func(_ context.Context, ms []model.SysDictDatum) {
				afterCommitCalls.Add(1)
				snapshots = append([]model.SysDictDatum(nil), ms...)
			}
		}})
		fixture := seedGeneratedDictFixture(t, h.svc)
		before1 := mustLoadDictData(t, h.db, fixture.data1ID)
		before2 := mustLoadDictData(t, h.db, fixture.data2ID)

		if _, err := h.svc.BatchDeleteDictData(context.Background(), &adminv1.BatchDeleteDictDataRequest{Ids: []int64{fixture.data1ID, fixture.data2ID, fixture.data2ID}}); err != nil {
			t.Fatalf("BatchDeleteDictData success: %v", err)
		}
		assertDictDataMissing(t, h.db, fixture.data1ID)
		assertDictDataMissing(t, h.db, fixture.data2ID)
		if got := afterCommitCalls.Load(); got != 1 {
			t.Fatalf("batch-delete after-commit calls = %d, want 1", got)
		}
		if len(snapshots) != 2 {
			t.Fatalf("after-commit batch-delete snapshots len = %d, want 2", len(snapshots))
		}
		if snapshots[0].DictLabel != before1.DictLabel || snapshots[1].DictLabel != before2.DictLabel {
			t.Fatalf("after-commit batch-delete snapshots mutated\nsnapshots=%+v\nbefore1=%+v\nbefore2=%+v", snapshots, before1, before2)
		}
	})
}

func TestGeneratedDictServiceDefaultsDictDataListToDictSortThenID(t *testing.T) {
	h := setupGeneratedDictHarness(t, generatedDictHookMutators{})
	ctx := context.Background()
	if _, err := h.svc.CreateDictType(ctx, &adminv1.CreateDictTypeRequest{
		DictName: "默认排序", DictType: "dict_default_order",
	}); err != nil {
		t.Fatalf("CreateDictType: %v", err)
	}
	create := func(label string, sort int32) int64 {
		t.Helper()
		resp, err := h.svc.CreateDictData(ctx, &adminv1.CreateDictDataRequest{
			DictType: "dict_default_order", DictLabel: label, DictValue: label, DictSort: sort,
		})
		if err != nil {
			t.Fatalf("CreateDictData(%s): %v", label, err)
		}
		return resp.GetItem().GetId()
	}
	highID := create("后排序", 20)
	firstLowID := create("先低排序", 10)
	secondLowID := create("后低排序", 10)

	resp, err := h.svc.ListDictData(ctx, &adminv1.ListDictDataRequest{
		DictType: proto.String("dict_default_order"),
		Page:     &adminv1.PageRequest{PageNum: 1, PageSize: 10},
	})
	if err != nil {
		t.Fatalf("ListDictData: %v", err)
	}
	items := resp.GetItems()
	if len(items) != 3 {
		t.Fatalf("ListDictData item count = %d, want 3", len(items))
	}
	got := []int64{items[0].GetId(), items[1].GetId(), items[2].GetId()}
	want := []int64{firstLowID, secondLowID, highID}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default ListDictData order = %v, want dict_sort,id order %v", got, want)
	}
}
