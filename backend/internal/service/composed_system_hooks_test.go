package service

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"

	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func checkCacheHookCounts[M any](t *testing.T, system *resource.HookSet[M], version func() uint64) {
	t.Helper()
	calls := 0
	expected := version() + 1
	callback := func(context.Context, M) {
		calls++
		if version() != expected {
			t.Fatalf("cache invalidations=%d want=%d", version(), expected)
		}
	}
	hooks := resource.ComposeHooks(system, &resource.HookSet[M]{AfterCommitCreate: callback, AfterCommitUpdate: callback, AfterCommitDelete: callback, AfterCommitBatchDelete: func(ctx context.Context, ms []M) { var m M; callback(ctx, m) }})
	var m M
	for _, invoke := range []func(){func() { hooks.AfterCommitCreate(context.Background(), m) }, func() { hooks.AfterCommitUpdate(context.Background(), m) }, func() { hooks.AfterCommitDelete(context.Background(), m) }, func() { hooks.AfterCommitBatchDelete(context.Background(), []M{m}) }} {
		invoke()
		if version() != expected {
			t.Fatal("system invalidation must run exactly once")
		}
		expected++
	}
	if calls != 4 {
		t.Fatalf("business callback count=%d", calls)
	}
}

func TestComposedSystemCacheCallbacksRunExactlyOnce(t *testing.T) {
	configCache := resource.NewVersionedCache[resourcegen.ConfigListCacheKey, configListCacheEntry]()
	configs, err := newConfigCachedRepository(resourcegen.NewConfigRepository(), configCache)
	if err != nil {
		t.Fatal(err)
	}
	checkCacheHookCounts(t, newConfigCacheHooks(configs), configCache.Version)
	dictCache := resource.NewVersionedCache[resourcegen.DictDataListCacheKey, dictDataListCacheEntry]()
	dicts, err := newDictDataCachedRepository(resourcegen.NewDictDataRepository(), dictCache)
	if err != nil {
		t.Fatal(err)
	}
	checkCacheHookCounts(t, newDictDataCacheHooks(dicts), dictCache.Version)
	checkCacheHookCounts(t, newDictTypeCacheHooks(dicts), dictCache.Version)
}

func TestComposedSystemRoleSafetyRunsBeforeBusiness(t *testing.T) {
	calls := 0
	hooks := resource.ComposeHooks(newRoleHooks(), &resource.HookSet[model.SysRole]{
		BeforeDelete:      func(context.Context, resource.Tx, *model.SysRole) error { calls++; return nil },
		BeforeBatchDelete: func(context.Context, resource.Tx, []*model.SysRole) error { calls++; return nil },
	})
	ctx := context.Background()
	protected := &model.SysRole{RoleKey: repo.SuperAdminRoleKey}
	if err := hooks.BeforeDelete(ctx, resource.Tx{}, protected); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("protected delete=%v", err)
	}
	if err := hooks.BeforeBatchDelete(ctx, resource.Tx{}, []*model.SysRole{protected}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("protected batch=%v", err)
	}
	if calls != 0 {
		t.Fatal("business callback entered after a mandatory rejection")
	}
	if err := hooks.BeforeDelete(ctx, resource.Tx{}, &model.SysRole{RoleKey: "operator"}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("business callback calls=%d", calls)
	}
}
