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

type dictDataListCacheEntry struct {
	items []*model.SysDictDatum
	total int64
}

// dictDataCachedRepository adds unrestricted-only cache-aside reads around the
// generated DictData repository using the generated canonical cache key.
type dictDataCachedRepository struct {
	inner resource.Repository[model.SysDictDatum, resourcegen.DictDataListQuery]
	cache *resource.VersionedCache[resourcegen.DictDataListCacheKey, dictDataListCacheEntry]
}

var _ resource.Repository[model.SysDictDatum, resourcegen.DictDataListQuery] = (*dictDataCachedRepository)(nil)

func newDictDataCachedRepository(
	inner resource.Repository[model.SysDictDatum, resourcegen.DictDataListQuery],
	cache *resource.VersionedCache[resourcegen.DictDataListCacheKey, dictDataListCacheEntry],
) (*dictDataCachedRepository, error) {
	if inner == nil {
		return nil, fmt.Errorf("dict data cached repository: inner repository is nil")
	}
	if cache == nil {
		return nil, fmt.Errorf("dict data cached repository: cache is nil")
	}
	return &dictDataCachedRepository{inner: inner, cache: cache}, nil
}

func cloneDictDataList(items []*model.SysDictDatum) []*model.SysDictDatum {
	if items == nil {
		return nil
	}
	out := make([]*model.SysDictDatum, len(items))
	copy(out, items)
	return out
}

func (r *dictDataCachedRepository) InvalidateAll() {
	r.cache.InvalidateAll()
}

func (r *dictDataCachedRepository) Create(ctx context.Context, txRepo resource.Session, m *model.SysDictDatum) error {

	return r.inner.Create(ctx, txRepo, m)
}

func (r *dictDataCachedRepository) GetByKey(ctx context.Context, txRepo resource.Session, key int64, access repo.AccessScope) (*model.SysDictDatum, error) {

	return r.inner.GetByKey(ctx, txRepo, key, access)
}

func (r *dictDataCachedRepository) GetByKeyForUpdate(ctx context.Context, txRepo resource.Session, key int64, access repo.AccessScope) (*model.SysDictDatum, error) {

	return r.inner.GetByKeyForUpdate(ctx, txRepo, key, access)
}

func (r *dictDataCachedRepository) Update(ctx context.Context, txRepo resource.Session, key int64, m *model.SysDictDatum, columns repo.ColumnSet, access repo.AccessScope) error {

	return r.inner.Update(ctx, txRepo, key, m, columns, access)
}

func (r *dictDataCachedRepository) DeleteByKey(ctx context.Context, txRepo resource.Session, key int64, access repo.AccessScope) error {

	return r.inner.DeleteByKey(ctx, txRepo, key, access)
}

func (r *dictDataCachedRepository) GetManyForUpdate(ctx context.Context, txRepo resource.Session, keys []int64, access repo.AccessScope) ([]*model.SysDictDatum, error) {

	return r.inner.GetManyForUpdate(ctx, txRepo, keys, access)
}

func (r *dictDataCachedRepository) BatchDelete(ctx context.Context, txRepo resource.Session, keys []int64, access repo.AccessScope) (int64, error) {

	return r.inner.BatchDelete(ctx, txRepo, keys, access)
}

// normalizeDictDataListQuery preserves the established DictData display order
// for callers that do not specify an OrderBy: dict_sort ASC, then the generated
// repository appends id as the stable pagination tie-breaker. Normalizing before
// cache-key construction makes omitted and explicit ascending dict_sort requests
// semantically identical cache queries.
func normalizeDictDataListQuery(q resourcegen.DictDataListQuery) resourcegen.DictDataListQuery {
	if len(q.Orders) == 0 {
		q.Orders = []repo.Order{{Column: resourcegen.DictDataColumnDictSort}}
	}
	return q
}

func (r *dictDataCachedRepository) List(ctx context.Context, txRepo resource.Session, q resourcegen.DictDataListQuery, access repo.AccessScope) ([]*model.SysDictDatum, int64, error) {

	q = normalizeDictDataListQuery(q)
	if !repo.CanUseGlobalCache(access) {
		return r.inner.List(ctx, txRepo, q, access)
	}
	key := resourcegen.NewDictDataListCacheKey(q)
	if cached, ok := r.cache.Get(key); ok {
		return cloneDictDataList(cached.items), cached.total, nil
	}
	version := r.cache.Version()
	items, total, err := r.inner.List(ctx, txRepo, q, access)
	if err != nil {
		return nil, 0, err
	}
	r.cache.SetIfVersion(version, key, dictDataListCacheEntry{items: cloneDictDataList(items), total: total})
	return items, total, nil
}

func newDictTypeCacheHooks(cache *dictDataCachedRepository) *resource.HookSet[model.SysDictType] {
	invalidate := func(context.Context, model.SysDictType) { cache.InvalidateAll() }
	invalidateBatch := func(context.Context, []model.SysDictType) { cache.InvalidateAll() }
	return &resource.HookSet[model.SysDictType]{
		AfterCommitCreate:      invalidate,
		AfterCommitUpdate:      invalidate,
		AfterCommitDelete:      invalidate,
		AfterCommitBatchDelete: invalidateBatch,
	}
}

func newDictDataCacheHooks(cache *dictDataCachedRepository) *resource.HookSet[model.SysDictDatum] {
	invalidate := func(context.Context, model.SysDictDatum) { cache.InvalidateAll() }
	invalidateBatch := func(context.Context, []model.SysDictDatum) { cache.InvalidateAll() }
	return &resource.HookSet[model.SysDictDatum]{
		AfterCommitCreate:      invalidate,
		AfterCommitUpdate:      invalidate,
		AfterCommitDelete:      invalidate,
		AfterCommitBatchDelete: invalidateBatch,
	}
}

func NewGeneratedDictService(baseRepo *repo.Repo) (adminv1connect.DictServiceHandler, error) {
	store, err := newResourceStore(baseRepo)
	if err != nil {
		return nil, err
	}
	cachedRepo, err := newDictDataCachedRepository(
		resourcegen.NewDictDataRepository(),
		resource.NewVersionedCache[resourcegen.DictDataListCacheKey, dictDataListCacheEntry](),
	)
	if err != nil {
		return nil, err
	}
	dictTypeController, err := resourcegen.NewDictTypeController(resourcegen.DictTypeControllerDeps{
		Store: store,
		Hooks: resource.ComposeHooks(newDictTypeCacheHooks(cachedRepo), nil),
	})
	if err != nil {
		return nil, err
	}
	dictDataController, err := resourcegen.NewDictDataController(resourcegen.DictDataControllerDeps{
		Store:      store,
		Hooks:      resource.ComposeHooks(newDictDataCacheHooks(cachedRepo), nil),
		Repository: cachedRepo,
	})
	if err != nil {
		return nil, err
	}
	return resourcegen.NewDictServiceAdapter(dictTypeController, dictDataController)
}
