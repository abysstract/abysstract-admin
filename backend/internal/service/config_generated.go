package service

import (
	"context"
	"fmt"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

type configListCacheEntry struct {
	items []*model.SysConfig
	total int64
}

// configCachedRepository adds unrestricted-only cache-aside List reads around
// the generated Config repository. Its key is the generated canonical
// ConfigListCacheKey, so page/keyword/order semantics are preserved without
// a second handwritten key format.
type configCachedRepository struct {
	inner resource.Repository[model.SysConfig, resourcegen.ConfigListQuery]
	cache *resource.VersionedCache[resourcegen.ConfigListCacheKey, configListCacheEntry]
}

var _ resource.Repository[model.SysConfig, resourcegen.ConfigListQuery] = (*configCachedRepository)(nil)

func newConfigCachedRepository(
	inner resource.Repository[model.SysConfig, resourcegen.ConfigListQuery],
	cache *resource.VersionedCache[resourcegen.ConfigListCacheKey, configListCacheEntry],
) (*configCachedRepository, error) {
	if inner == nil {
		return nil, fmt.Errorf("config cached repository: inner repository is nil")
	}
	if cache == nil {
		return nil, fmt.Errorf("config cached repository: cache is nil")
	}
	return &configCachedRepository{inner: inner, cache: cache}, nil
}

func cloneConfigList(items []*model.SysConfig) []*model.SysConfig {
	if items == nil {
		return nil
	}
	out := make([]*model.SysConfig, len(items))
	copy(out, items)
	return out
}

func (r *configCachedRepository) InvalidateAll() {
	r.cache.InvalidateAll()
}

func (r *configCachedRepository) Create(ctx context.Context, txRepo resource.Session, m *model.SysConfig) error {

	return r.inner.Create(ctx, txRepo, m)
}

func (r *configCachedRepository) GetByKey(ctx context.Context, txRepo resource.Session, key int64, access repo.AccessScope) (*model.SysConfig, error) {

	return r.inner.GetByKey(ctx, txRepo, key, access)
}

func (r *configCachedRepository) GetByKeyForUpdate(ctx context.Context, txRepo resource.Session, key int64, access repo.AccessScope) (*model.SysConfig, error) {

	return r.inner.GetByKeyForUpdate(ctx, txRepo, key, access)
}

func (r *configCachedRepository) Update(ctx context.Context, txRepo resource.Session, key int64, m *model.SysConfig, columns repo.ColumnSet, access repo.AccessScope) error {

	return r.inner.Update(ctx, txRepo, key, m, columns, access)
}

func (r *configCachedRepository) DeleteByKey(ctx context.Context, txRepo resource.Session, key int64, access repo.AccessScope) error {

	return r.inner.DeleteByKey(ctx, txRepo, key, access)
}

func (r *configCachedRepository) GetManyForUpdate(ctx context.Context, txRepo resource.Session, keys []int64, access repo.AccessScope) ([]*model.SysConfig, error) {

	return r.inner.GetManyForUpdate(ctx, txRepo, keys, access)
}

func (r *configCachedRepository) BatchDelete(ctx context.Context, txRepo resource.Session, keys []int64, access repo.AccessScope) (int64, error) {

	return r.inner.BatchDelete(ctx, txRepo, keys, access)
}

func (r *configCachedRepository) List(ctx context.Context, txRepo resource.Session, q resourcegen.ConfigListQuery, access repo.AccessScope) ([]*model.SysConfig, int64, error) {

	if !repo.CanUseGlobalCache(access) {
		return r.inner.List(ctx, txRepo, q, access)
	}
	key := resourcegen.NewConfigListCacheKey(q)
	if cached, ok := r.cache.Get(key); ok {
		return cloneConfigList(cached.items), cached.total, nil
	}
	version := r.cache.Version()
	items, total, err := r.inner.List(ctx, txRepo, q, access)
	if err != nil {
		return nil, 0, err
	}
	r.cache.SetIfVersion(version, key, configListCacheEntry{items: cloneConfigList(items), total: total})
	return items, total, nil
}

// newConfigCacheHooks invalidates every unrestricted Config List cache entry
// only after the generated controller transaction commits. Rollback paths never
// invoke these callbacks, and VersionedCache epoching rejects a stale in-flight
// read that started before the commit.
func newConfigCacheHooks(cache *configCachedRepository) *resource.HookSet[model.SysConfig] {
	invalidate := func(context.Context, model.SysConfig) { cache.InvalidateAll() }
	invalidateBatch := func(context.Context, []model.SysConfig) { cache.InvalidateAll() }
	return &resource.HookSet[model.SysConfig]{
		AfterCommitCreate:      invalidate,
		AfterCommitUpdate:      invalidate,
		AfterCommitDelete:      invalidate,
		AfterCommitBatchDelete: invalidateBatch,
	}
}

// NewGeneratedConfigService composes the generated Config controller with its
// typed cache decorator and commit-only invalidation hooks for server requests.
func NewGeneratedConfigService(baseRepo *repo.Repo) (adminv1connect.ConfigServiceHandler, error) {
	store, err := newResourceStore(baseRepo)
	if err != nil {
		return nil, err
	}
	cachedRepo, err := newConfigCachedRepository(
		resourcegen.NewConfigRepository(),
		resource.NewVersionedCache[resourcegen.ConfigListCacheKey, configListCacheEntry](),
	)
	if err != nil {
		return nil, err
	}
	controller, err := resourcegen.NewConfigController(resourcegen.ConfigControllerDeps{
		Store:      store,
		Hooks:      resource.ComposeHooks(newConfigCacheHooks(cachedRepo), nil),
		Repository: cachedRepo,
	})
	if err != nil {
		return nil, err
	}
	return resourcegen.NewConfigServiceAdapter(controller)
}
