package service

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"unsafe"

	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

type fakeDictDataRepository struct {
	listCalls int64
	listFn    func(resourcegen.DictDataListQuery, repo.AccessScope) ([]*model.SysDictDatum, int64, error)
}

func (r *fakeDictDataRepository) Create(context.Context, resource.Session, *model.SysDictDatum) error {
	panic("unexpected Create")
}

func (r *fakeDictDataRepository) GetByKey(context.Context, resource.Session, int64, repo.AccessScope) (*model.SysDictDatum, error) {
	panic("unexpected GetByKey")
}

func (r *fakeDictDataRepository) GetByKeyForUpdate(context.Context, resource.Session, int64, repo.AccessScope) (*model.SysDictDatum, error) {
	panic("unexpected GetByKeyForUpdate")
}

func (r *fakeDictDataRepository) Update(context.Context, resource.Session, int64, *model.SysDictDatum, repo.ColumnSet, repo.AccessScope) error {
	panic("unexpected Update")
}

func (r *fakeDictDataRepository) DeleteByKey(context.Context, resource.Session, int64, repo.AccessScope) error {
	panic("unexpected DeleteByKey")
}

func (r *fakeDictDataRepository) GetManyForUpdate(context.Context, resource.Session, []int64, repo.AccessScope) ([]*model.SysDictDatum, error) {
	panic("unexpected GetManyForUpdate")
}

func (r *fakeDictDataRepository) BatchDelete(context.Context, resource.Session, []int64, repo.AccessScope) (int64, error) {
	panic("unexpected BatchDelete")
}

func (r *fakeDictDataRepository) List(_ context.Context, _ resource.Session, q resourcegen.DictDataListQuery, access repo.AccessScope) ([]*model.SysDictDatum, int64, error) {
	atomic.AddInt64(&r.listCalls, 1)
	return r.listFn(q, access)
}

type countingDictDataRepository struct {
	inner     resource.Repository[model.SysDictDatum, resourcegen.DictDataListQuery]
	listCalls atomic.Int64
	afterList func()
}

func (r *countingDictDataRepository) Create(ctx context.Context, txRepo resource.Session, m *model.SysDictDatum) error {

	return r.inner.Create(ctx, txRepo, m)
}

func (r *countingDictDataRepository) GetByKey(ctx context.Context, txRepo resource.Session, key int64, access repo.AccessScope) (*model.SysDictDatum, error) {

	return r.inner.GetByKey(ctx, txRepo, key, access)
}

func (r *countingDictDataRepository) GetByKeyForUpdate(ctx context.Context, txRepo resource.Session, key int64, access repo.AccessScope) (*model.SysDictDatum, error) {

	return r.inner.GetByKeyForUpdate(ctx, txRepo, key, access)
}

func (r *countingDictDataRepository) Update(ctx context.Context, txRepo resource.Session, key int64, m *model.SysDictDatum, columns repo.ColumnSet, access repo.AccessScope) error {

	return r.inner.Update(ctx, txRepo, key, m, columns, access)
}

func (r *countingDictDataRepository) DeleteByKey(ctx context.Context, txRepo resource.Session, key int64, access repo.AccessScope) error {

	return r.inner.DeleteByKey(ctx, txRepo, key, access)
}

func (r *countingDictDataRepository) GetManyForUpdate(ctx context.Context, txRepo resource.Session, keys []int64, access repo.AccessScope) ([]*model.SysDictDatum, error) {

	return r.inner.GetManyForUpdate(ctx, txRepo, keys, access)
}

func (r *countingDictDataRepository) BatchDelete(ctx context.Context, txRepo resource.Session, keys []int64, access repo.AccessScope) (int64, error) {

	return r.inner.BatchDelete(ctx, txRepo, keys, access)
}

func (r *countingDictDataRepository) List(ctx context.Context, txRepo resource.Session, q resourcegen.DictDataListQuery, access repo.AccessScope) ([]*model.SysDictDatum, int64, error) {

	r.listCalls.Add(1)
	items, total, err := r.inner.List(ctx, txRepo, q, access)
	if r.afterList != nil {
		r.afterList()
	}
	return items, total, err
}

func rowLevelScopeForTest() repo.AccessScope {
	var scope repo.AccessScope
	v := reflect.ValueOf(&scope).Elem()
	applyField := v.FieldByName("apply")
	reflect.NewAt(applyField.Type(), unsafe.Pointer(applyField.UnsafeAddr())).Elem().Set(
		reflect.ValueOf(func(db *gorm.DB) *gorm.DB { return db.Where("id = ?", 1) }),
	)
	return scope
}

func TestDictDataCachedRepositoryUsesCanonicalKeyAndScopePolicy(t *testing.T) {
	inner := &fakeDictDataRepository{listFn: func(q resourcegen.DictDataListQuery, access repo.AccessScope) ([]*model.SysDictDatum, int64, error) {
		return []*model.SysDictDatum{{ID: 1, DictType: q.Keyword, DictLabel: "cached"}}, 1, nil
	}}
	cached, err := newDictDataCachedRepository(
		inner,
		resource.NewVersionedCache[resourcegen.DictDataListCacheKey, dictDataListCacheEntry](),
	)
	if err != nil {
		t.Fatalf("newDictDataCachedRepository: %v", err)
	}
	ctx := context.Background()
	dictType := "type-a"
	queryA := resourcegen.DictDataListQuery{
		PageNum: 0, PageSize: 0, Keyword: "  alpha  ", DictType: &dictType,
		Orders: []repo.Order{{Column: resourcegen.DictDataColumnDictSort, Desc: true}},
	}
	queryB := resourcegen.DictDataListQuery{
		PageNum:  repo.DefaultPageSize / repo.DefaultPageSize,
		PageSize: repo.DefaultPageSize,
		Keyword:  "alpha", DictType: &dictType,
		Orders: []repo.Order{{Column: resourcegen.DictDataColumnDictSort, Desc: true}},
	}
	if _, _, err := cached.List(ctx, nil, queryA, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("first unrestricted list: %v", err)
	}
	if _, _, err := cached.List(ctx, nil, queryB, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("canonical unrestricted list: %v", err)
	}
	if got := atomic.LoadInt64(&inner.listCalls); got != 1 {
		t.Fatalf("canonical equivalent unrestricted queries hit inner %d times, want 1", got)
	}

	if _, _, err := cached.List(ctx, nil, resourcegen.DictDataListQuery{PageNum: 1, PageSize: repo.DefaultPageSize, Keyword: "alpha"}, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("nil presence list: %v", err)
	}
	empty := ""
	if _, _, err := cached.List(ctx, nil, resourcegen.DictDataListQuery{PageNum: 1, PageSize: repo.DefaultPageSize, Keyword: "alpha", DictType: &empty}, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("empty-string presence list: %v", err)
	}
	if got := atomic.LoadInt64(&inner.listCalls); got != 3 {
		t.Fatalf("presence-sensitive unrestricted keys hit inner %d times, want 3", got)
	}

	if _, _, err := cached.List(ctx, nil, queryB, repo.AccessScope{}); err != nil {
		t.Fatalf("deny-all list 1: %v", err)
	}
	if _, _, err := cached.List(ctx, nil, queryB, repo.AccessScope{}); err != nil {
		t.Fatalf("deny-all list 2: %v", err)
	}
	if got := atomic.LoadInt64(&inner.listCalls); got != 5 {
		t.Fatalf("deny-all scope should bypass cache, inner calls = %d want 5", got)
	}

	rowLevel := rowLevelScopeForTest()
	if _, _, err := cached.List(ctx, nil, queryB, rowLevel); err != nil {
		t.Fatalf("row-level list 1: %v", err)
	}
	if _, _, err := cached.List(ctx, nil, queryB, rowLevel); err != nil {
		t.Fatalf("row-level list 2: %v", err)
	}
	if got := atomic.LoadInt64(&inner.listCalls); got != 7 {
		t.Fatalf("row-level scope should bypass cache, inner calls = %d want 7", got)
	}
}

func TestDictDataCachedRepositoryNormalizesDefaultOrderBeforeCacheKey(t *testing.T) {
	var seen resourcegen.DictDataListQuery
	inner := &fakeDictDataRepository{listFn: func(q resourcegen.DictDataListQuery, _ repo.AccessScope) ([]*model.SysDictDatum, int64, error) {
		seen = q
		return []*model.SysDictDatum{{ID: 1}}, 1, nil
	}}
	cached, err := newDictDataCachedRepository(
		inner,
		resource.NewVersionedCache[resourcegen.DictDataListCacheKey, dictDataListCacheEntry](),
	)
	if err != nil {
		t.Fatalf("newDictDataCachedRepository: %v", err)
	}
	query := resourcegen.DictDataListQuery{PageNum: 1, PageSize: repo.DefaultPageSize}
	if _, _, err := cached.List(context.Background(), nil, query, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("default-order List: %v", err)
	}
	explicit := query
	explicit.Orders = []repo.Order{{Column: resourcegen.DictDataColumnDictSort}}
	if _, _, err := cached.List(context.Background(), nil, explicit, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("explicit-order List: %v", err)
	}
	if got := atomic.LoadInt64(&inner.listCalls); got != 1 {
		t.Fatalf("omitted and explicit default order should share a cache entry; inner calls = %d, want 1", got)
	}
	if want := []repo.Order{{Column: resourcegen.DictDataColumnDictSort}}; !reflect.DeepEqual(seen.Orders, want) {
		t.Fatalf("normalized default order = %v, want %v", seen.Orders, want)
	}
}

func TestDictDataCachedRepositoryRejectsStaleFillAfterInvalidation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	first := true
	inner := &fakeDictDataRepository{listFn: func(q resourcegen.DictDataListQuery, access repo.AccessScope) ([]*model.SysDictDatum, int64, error) {
		if first {
			first = false
			close(started)
			<-release
		}
		return []*model.SysDictDatum{{ID: 1, DictType: "stale", DictLabel: "stale"}}, 1, nil
	}}
	cached, err := newDictDataCachedRepository(
		inner,
		resource.NewVersionedCache[resourcegen.DictDataListCacheKey, dictDataListCacheEntry](),
	)
	if err != nil {
		t.Fatalf("newDictDataCachedRepository: %v", err)
	}
	ctx := context.Background()
	query := resourcegen.DictDataListQuery{PageNum: 1, PageSize: repo.DefaultPageSize, DictType: proto.String("epoch-test")}
	errCh := make(chan error, 1)
	go func() {
		_, _, err := cached.List(ctx, nil, query, repo.UnrestrictedAccessScope())
		errCh <- err
	}()
	<-started
	versionBefore := cached.cache.Version()
	cached.InvalidateAll()
	versionAfter := cached.cache.Version()
	if versionAfter == versionBefore {
		t.Fatal("InvalidateAll did not advance cache epoch")
	}
	close(release)
	if err := <-errCh; err != nil {
		t.Fatalf("stale list returned error: %v", err)
	}
	key := resourcegen.NewDictDataListCacheKey(query)
	if _, ok := cached.cache.Get(key); ok {
		t.Fatal("stale read refilled cache after invalidation")
	}
	if _, _, err := cached.List(ctx, nil, query, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("fresh list after invalidation: %v", err)
	}
	if got := atomic.LoadInt64(&inner.listCalls); got != 2 {
		t.Fatalf("stale fill should be rejected and re-read once, inner calls = %d want 2", got)
	}
}

type generatedDictFixture struct {
	type1ID int64
	type2ID int64
	data1ID int64
	data2ID int64
	type1   string
}

type generatedDictHookMutators struct {
	typeHooks func(*resource.HookSet[model.SysDictType])
	dataHooks func(*resource.HookSet[model.SysDictDatum])
}

type generatedDictHarness struct {
	svc     adminv1connect.DictServiceHandler
	counter *countingDictDataRepository
	db      *gorm.DB
}

func seedGeneratedDictFixture(t *testing.T, svc adminv1connect.DictServiceHandler) generatedDictFixture {
	t.Helper()
	ctx := context.Background()
	createType := func(name, dictType string) int64 {
		resp, err := svc.CreateDictType(ctx, &adminv1.CreateDictTypeRequest{DictName: name, DictType: dictType})
		if err != nil {
			t.Fatalf("CreateDictType(%s): %v", dictType, err)
		}
		return resp.GetItem().GetId()
	}
	type1 := "cache_type_one"
	type2 := "cache_type_two"
	type1ID := createType("缓存类型一", type1)
	type2ID := createType("缓存类型二", type2)
	data1, err := svc.CreateDictData(ctx, &adminv1.CreateDictDataRequest{
		DictType: type1, DictLabel: "缓存数据一", DictValue: "cache-1", DictSort: 1,
	})
	if err != nil {
		t.Fatalf("CreateDictData(type1): %v", err)
	}
	data2, err := svc.CreateDictData(ctx, &adminv1.CreateDictDataRequest{
		DictType: type2, DictLabel: "旁路数据", DictValue: "side-1", DictSort: 2,
	})
	if err != nil {
		t.Fatalf("CreateDictData(type2): %v", err)
	}
	return generatedDictFixture{type1ID: type1ID, type2ID: type2ID, data1ID: data1.GetItem().GetId(), data2ID: data2.GetItem().GetId(), type1: type1}
}

func mustLoadDictData(t *testing.T, db *gorm.DB, id int64) model.SysDictDatum {
	t.Helper()
	var item model.SysDictDatum
	if err := db.WithContext(context.Background()).First(&item, id).Error; err != nil {
		t.Fatalf("load dict data %d: %v", id, err)
	}
	return item
}

func mustLoadDictType(t *testing.T, db *gorm.DB, id int64) model.SysDictType {
	t.Helper()
	var item model.SysDictType
	if err := db.WithContext(context.Background()).First(&item, id).Error; err != nil {
		t.Fatalf("load dict type %d: %v", id, err)
	}
	return item
}

func assertDictDataMissing(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	var item model.SysDictDatum
	err := db.WithContext(context.Background()).First(&item, id).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("dict data %d existence error = %v, want record not found", id, err)
	}
}

func warmGeneratedDictDataList(t *testing.T, svc adminv1connect.DictServiceHandler, counter *countingDictDataRepository, dictType string, wantCalls int64) {
	t.Helper()
	ctx := context.Background()
	req := &adminv1.ListDictDataRequest{DictType: proto.String(dictType)}
	first, err := svc.ListDictData(ctx, req)
	if err != nil {
		t.Fatalf("first ListDictData: %v", err)
	}
	second, err := svc.ListDictData(ctx, req)
	if err != nil {
		t.Fatalf("second ListDictData: %v", err)
	}
	if !proto.Equal(first, second) {
		t.Fatalf("cache hit changed response\nfirst=%v\nsecond=%v", first, second)
	}
	if got := counter.listCalls.Load(); got != wantCalls {
		t.Fatalf("ListDictData inner calls = %d, want %d", got, wantCalls)
	}
}

func TestNewGeneratedDictServiceRejectsNilBaseRepo(t *testing.T) {
	if _, err := NewGeneratedDictService(nil); err == nil {
		t.Fatal("NewGeneratedDictService accepted nil BaseRepo")
	}
}
