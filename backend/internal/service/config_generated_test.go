package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	resourcegen "github.com/abysstract/abysstract-admin/backend/internal/resource/gen"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

type fakeConfigRepository struct {
	listCalls int64
	listFn    func(resourcegen.ConfigListQuery, repo.AccessScope) ([]*model.SysConfig, int64, error)
}

func (r *fakeConfigRepository) Create(context.Context, resource.Session, *model.SysConfig) error {
	panic("unexpected Create")
}

func (r *fakeConfigRepository) GetByKey(context.Context, resource.Session, int64, repo.AccessScope) (*model.SysConfig, error) {
	panic("unexpected GetByKey")
}

func (r *fakeConfigRepository) GetByKeyForUpdate(context.Context, resource.Session, int64, repo.AccessScope) (*model.SysConfig, error) {
	panic("unexpected GetByKeyForUpdate")
}

func (r *fakeConfigRepository) Update(context.Context, resource.Session, int64, *model.SysConfig, repo.ColumnSet, repo.AccessScope) error {
	panic("unexpected Update")
}

func (r *fakeConfigRepository) DeleteByKey(context.Context, resource.Session, int64, repo.AccessScope) error {
	panic("unexpected DeleteByKey")
}

func (r *fakeConfigRepository) GetManyForUpdate(context.Context, resource.Session, []int64, repo.AccessScope) ([]*model.SysConfig, error) {
	panic("unexpected GetManyForUpdate")
}

func (r *fakeConfigRepository) BatchDelete(context.Context, resource.Session, []int64, repo.AccessScope) (int64, error) {
	panic("unexpected BatchDelete")
}

func (r *fakeConfigRepository) List(_ context.Context, _ resource.Session, q resourcegen.ConfigListQuery, access repo.AccessScope) ([]*model.SysConfig, int64, error) {
	atomic.AddInt64(&r.listCalls, 1)
	return r.listFn(q, access)
}

var _ resource.Repository[model.SysConfig, resourcegen.ConfigListQuery] = (*fakeConfigRepository)(nil)

func TestConfigCachedRepositoryUsesGeneratedCanonicalKeyAndScopePolicy(t *testing.T) {
	inner := &fakeConfigRepository{listFn: func(q resourcegen.ConfigListQuery, _ repo.AccessScope) ([]*model.SysConfig, int64, error) {
		return []*model.SysConfig{{ID: int64(q.PageNum), ConfigName: q.Keyword}}, int64(q.PageSize), nil
	}}
	cached, err := newConfigCachedRepository(
		inner,
		resource.NewVersionedCache[resourcegen.ConfigListCacheKey, configListCacheEntry](),
	)
	if err != nil {
		t.Fatalf("newConfigCachedRepository: %v", err)
	}
	ctx := context.Background()
	queryA := resourcegen.ConfigListQuery{
		PageNum: 0, PageSize: 0, Keyword: "  alpha  ",
		Orders: []repo.Order{{Column: resourcegen.ConfigColumnConfigKey}},
	}
	queryB := resourcegen.ConfigListQuery{
		PageNum: 1, PageSize: repo.DefaultPageSize, Keyword: "alpha",
		Orders: []repo.Order{{Column: resourcegen.ConfigColumnConfigKey}},
	}
	first, total, err := cached.List(ctx, nil, queryA, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatalf("first unrestricted List: %v", err)
	}
	second, secondTotal, err := cached.List(ctx, nil, queryB, repo.UnrestrictedAccessScope())
	if err != nil {
		t.Fatalf("canonical unrestricted List: %v", err)
	}
	if len(first) != len(second) || first[0].ConfigName != second[0].ConfigName || total != secondTotal {
		t.Fatalf("cache hit changed list result: first=%+v/%d second=%+v/%d", first, total, second, secondTotal)
	}
	if got := atomic.LoadInt64(&inner.listCalls); got != 1 {
		t.Fatalf("canonical equivalent queries called inner %d times, want 1", got)
	}

	for _, q := range []resourcegen.ConfigListQuery{
		{PageNum: 2, PageSize: repo.DefaultPageSize, Keyword: "alpha", Orders: queryB.Orders},
		{PageNum: 1, PageSize: repo.DefaultPageSize, Keyword: "beta", Orders: queryB.Orders},
		{PageNum: 1, PageSize: repo.DefaultPageSize, Keyword: "alpha", Orders: []repo.Order{{Column: resourcegen.ConfigColumnConfigKey, Desc: true}}},
	} {
		if _, _, err := cached.List(ctx, nil, q, repo.UnrestrictedAccessScope()); err != nil {
			t.Fatalf("distinct unrestricted List(%+v): %v", q, err)
		}
	}
	if got := atomic.LoadInt64(&inner.listCalls); got != 4 {
		t.Fatalf("page/keyword/order variants should miss independently; inner calls = %d, want 4", got)
	}

	for range 2 {
		if _, _, err := cached.List(ctx, nil, queryB, repo.AccessScope{}); err != nil {
			t.Fatalf("deny-all List: %v", err)
		}
	}
	if got := atomic.LoadInt64(&inner.listCalls); got != 6 {
		t.Fatalf("deny-all scope must bypass global cache; inner calls = %d, want 6", got)
	}

	for index, scope := range []repo.AccessScope{rowLevelScopeForTest(), rowLevelScopeForTest()} {
		for range 2 {
			if _, _, err := cached.List(ctx, nil, queryB, scope); err != nil {
				t.Fatalf("row-level scope %d List: %v", index, err)
			}
		}
	}
	if got := atomic.LoadInt64(&inner.listCalls); got != 10 {
		t.Fatalf("two row-level scopes must both bypass global cache; inner calls = %d, want 10", got)
	}
}

type countingConfigRepository struct {
	inner                  resource.Repository[model.SysConfig, resourcegen.ConfigListQuery]
	createCalls            atomic.Int64
	listCalls              atomic.Int64
	getByKeyForUpdateCalls atomic.Int64
	updateCalls            atomic.Int64
}

func (r *countingConfigRepository) Create(ctx context.Context, txRepo resource.Session, m *model.SysConfig) error {

	r.createCalls.Add(1)
	return r.inner.Create(ctx, txRepo, m)
}

func (r *countingConfigRepository) GetByKey(ctx context.Context, txRepo resource.Session, key int64, access repo.AccessScope) (*model.SysConfig, error) {

	return r.inner.GetByKey(ctx, txRepo, key, access)
}

func (r *countingConfigRepository) GetByKeyForUpdate(ctx context.Context, txRepo resource.Session, key int64, access repo.AccessScope) (*model.SysConfig, error) {

	r.getByKeyForUpdateCalls.Add(1)
	return r.inner.GetByKeyForUpdate(ctx, txRepo, key, access)
}

func (r *countingConfigRepository) Update(ctx context.Context, txRepo resource.Session, key int64, m *model.SysConfig, columns repo.ColumnSet, access repo.AccessScope) error {

	r.updateCalls.Add(1)
	return r.inner.Update(ctx, txRepo, key, m, columns, access)
}

func (r *countingConfigRepository) DeleteByKey(ctx context.Context, txRepo resource.Session, key int64, access repo.AccessScope) error {

	return r.inner.DeleteByKey(ctx, txRepo, key, access)
}

func (r *countingConfigRepository) GetManyForUpdate(ctx context.Context, txRepo resource.Session, keys []int64, access repo.AccessScope) ([]*model.SysConfig, error) {

	return r.inner.GetManyForUpdate(ctx, txRepo, keys, access)
}

func (r *countingConfigRepository) BatchDelete(ctx context.Context, txRepo resource.Session, keys []int64, access repo.AccessScope) (int64, error) {

	return r.inner.BatchDelete(ctx, txRepo, keys, access)
}

func (r *countingConfigRepository) List(ctx context.Context, txRepo resource.Session, q resourcegen.ConfigListQuery, access repo.AccessScope) ([]*model.SysConfig, int64, error) {

	r.listCalls.Add(1)
	return r.inner.List(ctx, txRepo, q, access)
}

type generatedConfigFixture struct {
	firstID  int64
	secondID int64
}

type generatedConfigHarness struct {
	svc     adminv1connect.ConfigServiceHandler
	counter *countingConfigRepository
	db      *gorm.DB
}

func seedGeneratedConfigFixture(t *testing.T, svc adminv1connect.ConfigServiceHandler) generatedConfigFixture {
	t.Helper()
	create := func(name, key string) int64 {
		t.Helper()
		resp, err := svc.CreateConfig(context.Background(), &adminv1.CreateConfigRequest{
			ConfigName: name, ConfigKey: key, ConfigValue: key,
		})
		if err != nil {
			t.Fatalf("CreateConfig(%s): %v", key, err)
		}
		return resp.GetItem().GetId()
	}
	return generatedConfigFixture{
		firstID:  create("缓存参数一", "cache.config.one"),
		secondID: create("缓存参数二", "cache.config.two"),
	}
}

func mustLoadGeneratedConfig(t *testing.T, db *gorm.DB, id int64) model.SysConfig {
	t.Helper()
	var item model.SysConfig
	if err := db.WithContext(context.Background()).First(&item, id).Error; err != nil {
		t.Fatalf("load config %d: %v", id, err)
	}
	return item
}

func assertGeneratedConfigMissing(t *testing.T, db *gorm.DB, id int64) {
	t.Helper()
	var item model.SysConfig
	err := db.WithContext(context.Background()).First(&item, id).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("config %d existence error = %v, want record not found", id, err)
	}
}

func warmGeneratedConfigList(t *testing.T, svc adminv1connect.ConfigServiceHandler, counter *countingConfigRepository, wantCalls int64) {
	t.Helper()
	req := &adminv1.ListConfigsRequest{Page: &adminv1.PageRequest{PageNum: 1, PageSize: repo.DefaultPageSize}}
	first, err := svc.ListConfigs(context.Background(), req)
	if err != nil {
		t.Fatalf("first ListConfigs: %v", err)
	}
	second, err := svc.ListConfigs(context.Background(), req)
	if err != nil {
		t.Fatalf("second ListConfigs: %v", err)
	}
	if !proto.Equal(first, second) {
		t.Fatalf("cache hit changed Config List response\nfirst=%v\nsecond=%v", first, second)
	}
	if got := counter.listCalls.Load(); got != wantCalls {
		t.Fatalf("Config inner List calls = %d, want %d", got, wantCalls)
	}
}

func TestConfigCachedRepositoryRejectsStaleFillAfterAfterCommitInvalidation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	first := true
	inner := &fakeConfigRepository{listFn: func(q resourcegen.ConfigListQuery, _ repo.AccessScope) ([]*model.SysConfig, int64, error) {
		if first {
			first = false
			close(started)
			<-release
		}
		return []*model.SysConfig{{ID: 1, ConfigName: q.Keyword}}, 1, nil
	}}
	cached, err := newConfigCachedRepository(
		inner,
		resource.NewVersionedCache[resourcegen.ConfigListCacheKey, configListCacheEntry](),
	)
	if err != nil {
		t.Fatalf("newConfigCachedRepository: %v", err)
	}
	hooks := newConfigCacheHooks(cached)
	query := resourcegen.ConfigListQuery{PageNum: 1, PageSize: repo.DefaultPageSize, Keyword: "stale"}
	errCh := make(chan error, 1)
	go func() {
		_, _, err := cached.List(context.Background(), nil, query, repo.UnrestrictedAccessScope())
		errCh <- err
	}()
	<-started
	versionBefore := cached.cache.Version()
	hooks.AfterCommitUpdate(context.Background(), model.SysConfig{})
	if versionAfter := cached.cache.Version(); versionAfter == versionBefore {
		t.Fatal("AfterCommitUpdate did not advance Config cache epoch")
	}
	close(release)
	if err := <-errCh; err != nil {
		t.Fatalf("stale Config List returned error: %v", err)
	}
	key := resourcegen.NewConfigListCacheKey(query)
	if _, ok := cached.cache.Get(key); ok {
		t.Fatal("stale Config List refilled cache after AfterCommit invalidation")
	}
	if _, _, err := cached.List(context.Background(), nil, query, repo.UnrestrictedAccessScope()); err != nil {
		t.Fatalf("fresh Config List after invalidation: %v", err)
	}
	if got := atomic.LoadInt64(&inner.listCalls); got != 2 {
		t.Fatalf("stale Config fill should be rejected and re-read once; inner calls = %d, want 2", got)
	}
}

func TestNewGeneratedConfigServiceRejectsNilBaseRepo(t *testing.T) {
	if _, err := NewGeneratedConfigService(nil); err == nil {
		t.Fatal("NewGeneratedConfigService accepted nil BaseRepo")
	}
}

func TestGeneratedConfigCreateOnlyFieldsRemainOutsideUpdateColumns(t *testing.T) {
	old := &model.SysConfig{ConfigName: "name", ConfigKey: "fixed.key", ConfigValue: "old", ConfigType: 1, Remark: "remark"}
	updated, columns, err := resourcegen.ConfigUpdateToModel(old, &adminv1.UpdateConfigRequest{ConfigValue: proto.String("new")})
	if err != nil {
		t.Fatalf("ConfigUpdateToModel: %v", err)
	}
	if updated.ConfigKey != old.ConfigKey || updated.ConfigType != old.ConfigType {
		t.Fatalf("CreateOnly fields changed through Update converter: old=%+v updated=%+v", old, updated)
	}
	for _, column := range columns.Values() {
		if column == resourcegen.ConfigColumnConfigKey {
			t.Fatalf("CreateOnly config_key appeared in update columns: %v", columns.Values())
		}
	}
}
