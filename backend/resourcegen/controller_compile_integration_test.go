//go:build integration

package resourcegen

import (
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// Controller code must compile together with its generated converter and
// repository adapter; gofmt/string checks alone cannot validate imports/types.
func TestWidgetControllerGeneratedSourcesCompile(t *testing.T) {
	compileWidgetPlan(t, widgetPlan(t), false)
}

// Sample A must be a complete, single-Resource Generated service: the
// generated default repository, controller and ServiceAdapter are compiled in
// an isolated package, and the adapter is asserted against the handler.
func TestCompileSampleASingleGeneratedDefaultRepository(t *testing.T) {
	plan, err := schematest.BoolCompileSampleResource().Build()
	if err != nil {
		t.Fatalf("build Sample A: %v", err)
	}
	compileServicePlansWithRuntime(t, []*schema.Plan{plan}, widgetSampleARuntimeTest)
}

const widgetSampleARuntimeTest = `package gen

import (
	"context"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

var _ schematest.BoolServiceHandler = (*BoolServiceAdapter)(nil)

func TestSampleADefaultRepositoryControllerAndAdapter(t *testing.T) {
	db := testutil.OpenDB(t)
	if err := db.AutoMigrate(&schematest.BoolModel{}); err != nil {
		t.Fatal(err)
	}
	controller, err := NewBoolController(BoolControllerDeps{Store: testutil.Store(t, repo.New(db))})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewBoolServiceAdapter(controller)
	if err != nil {
		t.Fatal(err)
	}
	created, err := adapter.CreateBool(context.Background(), &testpb.CreateBoolRequest{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if created.GetItem() == nil || !created.GetItem().GetEnabled() {
		t.Fatalf("unexpected item: %+v", created.GetItem())
	}
}
`

// Sample B proves an injected repository remains fully typed and that
// HookSet execution survives the Controller -> ServiceAdapter path.
func TestCompileSampleBInjectedTypedRepositoryAndHookSet(t *testing.T) {
	res := schematest.BoolCompileSampleResource()
	res.Repository.Implementation = schema.RepositoryInjected
	plan, err := res.Build()
	if err != nil {
		t.Fatalf("build Sample B: %v", err)
	}
	compileServicePlansWithRuntime(t, []*schema.Plan{plan}, boolSampleBRuntimeTest)
}

const boolSampleBRuntimeTest = `package gen

import (
	"context"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

type sampleBRepository struct {
	inner       resource.Repository[schematest.BoolModel, BoolListQuery]
	createCalls int
}

var _ resource.Repository[schematest.BoolModel, BoolListQuery] = (*sampleBRepository)(nil)

func (d *sampleBRepository) Create(ctx context.Context, rSession resource.Session, model *schematest.BoolModel) error {
	r := repo.New(rSession.DB())

	d.createCalls++
	return d.inner.Create(ctx, r, model)
}
func (d *sampleBRepository) GetByKey(ctx context.Context, rSession resource.Session, id int64, scope repo.AccessScope) (*schematest.BoolModel, error) {
	r := repo.New(rSession.DB())

	return d.inner.GetByKey(ctx, r, id, scope)
}
func (d *sampleBRepository) GetByKeyForUpdate(ctx context.Context, rSession resource.Session, id int64, scope repo.AccessScope) (*schematest.BoolModel, error) {
	r := repo.New(rSession.DB())

	return d.inner.GetByKeyForUpdate(ctx, r, id, scope)
}
func (d *sampleBRepository) Update(ctx context.Context, rSession resource.Session, id int64, model *schematest.BoolModel, cols repo.ColumnSet, scope repo.AccessScope) error {
	r := repo.New(rSession.DB())

	return d.inner.Update(ctx, r, id, model, cols, scope)
}
func (d *sampleBRepository) DeleteByKey(ctx context.Context, rSession resource.Session, id int64, scope repo.AccessScope) error {
	r := repo.New(rSession.DB())

	return d.inner.DeleteByKey(ctx, r, id, scope)
}
func (d *sampleBRepository) GetManyForUpdate(ctx context.Context, rSession resource.Session, ids []int64, scope repo.AccessScope) ([]*schematest.BoolModel, error) {
	r := repo.New(rSession.DB())

	return d.inner.GetManyForUpdate(ctx, r, ids, scope)
}
func (d *sampleBRepository) BatchDelete(ctx context.Context, rSession resource.Session, ids []int64, scope repo.AccessScope) (int64, error) {
	r := repo.New(rSession.DB())

	return d.inner.BatchDelete(ctx, r, ids, scope)
}
func (d *sampleBRepository) List(ctx context.Context, rSession resource.Session, query BoolListQuery, scope repo.AccessScope) ([]*schematest.BoolModel, int64, error) {
	r := repo.New(rSession.DB())

	return d.inner.List(ctx, r, query, scope)
}

func TestSampleBInjectedDecoratorAndHookSet(t *testing.T) {
	db := testutil.OpenDB(t)
	if err := db.AutoMigrate(&schematest.BoolModel{}); err != nil {
		t.Fatal(err)
	}
	decorator := &sampleBRepository{inner: NewBoolRepository()}
	beforeCreate, afterCommit := false, 0
	controller, err := NewBoolController(BoolControllerDeps{
		Store: testutil.Store(t, repo.New(db)), Repository: decorator,
		Hooks: &resource.HookSet[schematest.BoolModel]{
			BeforeCreate:      func(context.Context, resource.Tx, *schematest.BoolModel) error { beforeCreate = true; return nil },
			AfterCommitCreate: func(context.Context, schematest.BoolModel) { afterCommit++ },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewBoolServiceAdapter(controller)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.CreateBool(context.Background(), &testpb.CreateBoolRequest{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if decorator.createCalls != 1 {
		t.Fatalf("decorator Create calls = %d, want 1", decorator.createCalls)
	}
	if !beforeCreate {
		t.Fatal("BeforeCreate was not invoked")
	}
	if afterCommit != 1 {
		t.Fatalf("AfterCommitCreate calls = %d, want 1", afterCommit)
	}
}
`

// Generated audit timestamps use microsecond precision and the persisted
// model read back in the caller transaction reaches AfterCommit.
func TestGeneratedUpdateNormalizesAuditTimestampToMicroseconds(t *testing.T) {
	plan, err := schematest.BoolCompileSampleResource().Build()
	if err != nil {
		t.Fatalf("build timestamp fixture: %v", err)
	}
	compileServicePlansWithRuntime(t, []*schema.Plan{plan}, boolUpdatedAtPrecisionRuntimeTest)
}

const boolUpdatedAtPrecisionRuntimeTest = `package gen

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

func TestUpdateAuditTimestampHasMicrosecondPrecision(t *testing.T) {
	db := testutil.OpenDB(t)
	if err := db.AutoMigrate(&schematest.BoolModel{}); err != nil {
		t.Fatal(err)
	}
	var afterCommit time.Time
	controller, err := NewBoolController(BoolControllerDeps{
		Store: testutil.Store(t, repo.New(db)),
		Hooks: &resource.HookSet[schematest.BoolModel]{AfterCommitUpdate: func(_ context.Context, snapshot schematest.BoolModel) { afterCommit = snapshot.UpdatedAt }},
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewBoolServiceAdapter(controller)
	if err != nil {
		t.Fatal(err)
	}
	created, err := adapter.CreateBool(context.Background(), &testpb.CreateBoolRequest{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := adapter.UpdateBool(context.Background(), &testpb.UpdateBoolRequest{Id: created.GetItem().GetId(), Enabled: proto.Bool(true)})
	if err != nil {
		t.Fatal(err)
	}
	responseTime := updated.GetItem().GetUpdatedAt().AsTime()
	if responseTime.Nanosecond()%int(time.Microsecond) != 0 {
		t.Fatalf("response UpdatedAt %s is not PostgreSQL microsecond precision", responseTime)
	}
	if afterCommit.Nanosecond()%int(time.Microsecond) != 0 {
		t.Fatalf("AfterCommit UpdatedAt %s is not PostgreSQL microsecond precision", afterCommit)
	}
	if !afterCommit.Equal(responseTime) {
		t.Fatalf("AfterCommit UpdatedAt %s != response %s", afterCommit, responseTime)
	}
}
`

// This test compiles and runs an independently generated
// package test rather than a source/render assertion. Its runtime fixture
// exercises update/delete transaction ordering, empty and same-value patches,
// rollback paths, scope-invisible hooks, and commit-only callbacks.
func TestUpdateDeleteTimingCompileSample(t *testing.T) {
	res := schematest.WidgetFixtureResource()
	res.Auth = schema.AuthConfig{DataScope: true, DeptField: "DeptID", CreatorField: "CreatedBy"}
	plan, err := res.Build()
	if err != nil {
		t.Fatal(err)
	}
	compileWidgetPlanWithRuntime(t, plan, widgetDataScopeRuntimeTest)
}

func TestWidgetDataScopeControllerGeneratedSourcesCompile(t *testing.T) {
	res := schematest.WidgetFixtureResource()
	res.Auth = schema.AuthConfig{DataScope: true, DeptField: "DeptID", CreatorField: "CreatedBy"}
	plan, err := res.Build()
	if err != nil {
		t.Fatal(err)
	}
	compileWidgetPlan(t, plan, true)
}

func TestWidgetDataScopeSkipCreateCheckGeneratedSourcesCompile(t *testing.T) {
	res := schematest.WidgetFixtureResource()
	res.Auth = schema.AuthConfig{
		DataScope: true, DeptField: "DeptID", CreatorField: "CreatedBy",
		SkipCreateScopeCheck: true,
	}
	plan, err := res.Build()
	if err != nil {
		t.Fatal(err)
	}
	compileWidgetPlanWithRuntime(t, plan, widgetSkipCreateScopeRuntimeTest)
}

func compileWidgetPlan(t *testing.T, plan *schema.Plan, includeDataScopeRuntimeTest bool) {
	t.Helper()
	runtimeTest := ""
	if includeDataScopeRuntimeTest {
		runtimeTest = widgetDataScopeRuntimeTest
	}
	compileWidgetPlanWithRuntime(t, plan, runtimeTest)
}

const widgetSkipCreateScopeRuntimeTest = `package gen

import (
	"context"
	"testing"

	adminv1 "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

type skipCreateDenyScope struct{}

func (skipCreateDenyScope) DataScope(context.Context, string, string) repo.AccessScope {
	return repo.AccessScope{}
}

func TestSkipCreateScopeCheckStillRequiresProviderButBypassesValidator(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := db.AutoMigrate(&schematest.WidgetModel{}); err != nil {
		t.Fatal(err)
	}
	r := repo.New(db)
	if _, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r)}); err == nil {
		t.Fatal("skip_create_scope_check accepted nil DataScopeProvider")
	}
	c, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r), DataScopeProvider: skipCreateDenyScope{}})
	if err != nil {
		t.Fatalf("skip_create_scope_check controller construction: %v", err)
	}
	if _, err := c.CreateWidget(context.Background(), &testpb.CreateWidgetRequest{Name: "skip-scope-create"}); err != nil {
		t.Fatalf("Create unexpectedly required a validator: %v", err)
	}
	var count int64
	if err := db.Model(&schematest.WidgetModel{}).Where("name = ?", "skip-scope-create").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("skip create did not persist: count=%d err=%v", count, err)
	}
	listed, err := c.ListWidgets(context.Background(), &testpb.ListWidgetsRequest{Page: &adminv1.PageRequest{PageNum: 1, PageSize: 10}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed.Items) != 0 || listed.Page.GetTotal() != 0 {
		t.Fatalf("skip flag weakened read DataScope: items=%+v page=%+v", listed.Items, listed.Page)
	}
}
`

func TestPartialWidgetGeneratedSourcesCompile(t *testing.T) {
	compileWidgetPlan(t, partialWidgetPlan(t), false)
}

func TestPartialWidgetCustomOverrideRuntime(t *testing.T) {
	compileWidgetPlanWithRuntime(t, partialWidgetPlan(t), widgetPartialOverrideRuntimeTest)
}

const widgetPartialOverrideRuntimeTest = `package gen

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

type widgetRefreshOverride struct{ calls int }

func (o *widgetRefreshOverride) RefreshWidget(_ context.Context, req *testpb.RefreshWidgetRequest) (*testpb.RefreshWidgetResponse, error) {
	o.calls++
	if req.GetId() != 42 {
		return nil, connect.NewError(connect.CodeInvalidArgument, "")
	}
	return &testpb.RefreshWidgetResponse{}, nil
}

func TestPartialCustomOverrideIsRequiredAndDelegated(t *testing.T) {
	r := repo.New(testutil.OpenDB(t))
	if _, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r)}); err == nil {
		t.Fatal("Partial controller accepted nil Overrides")
	}
	override := &widgetRefreshOverride{}
	controller, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r), Overrides: override})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := controller.RefreshWidget(context.Background(), &testpb.RefreshWidgetRequest{Id: 42}); err != nil {
		t.Fatal(err)
	}
	if override.calls != 1 {
		t.Fatalf("override calls = %d, want 1", override.calls)
	}
}
`

// one service aggregates two Resources: Widget provides generated CRUD
// while Partial WidgetRefresh supplies an EXEMPT CUSTOM Override.
func TestCompileSampleCSameServiceGeneratedAndPartialOverride(t *testing.T) {
	compileServicePlansWithRuntime(t, widgetServicePlans(t), widgetServiceAdapterRuntimeTest)
}

const widgetServiceAdapterRuntimeTest = `package gen

import (
	"context"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
)

var _ schematest.WidgetServiceHandler = (*WidgetServiceAdapter)(nil)

type sampleCRefreshOverride struct{ calls int }

func (o *sampleCRefreshOverride) RefreshWidget(_ context.Context, req *testpb.RefreshWidgetRequest) (*testpb.RefreshWidgetResponse, error) {
	o.calls++
	if req.GetId() != 99 {
		return nil, connect.NewError(connect.CodeInvalidArgument, "")
	}
	return &testpb.RefreshWidgetResponse{}, nil
}

func TestSampleCAdapterControllerOverrideChain(t *testing.T) {
	db := testutil.OpenDB(t)
	testutil.ResetSchema(t, db)
	if err := db.AutoMigrate(&schematest.WidgetModel{}); err != nil {
		t.Fatal(err)
	}
	r := repo.New(db)
	widget, err := NewWidgetController(WidgetControllerDeps{Store: testutil.Store(t, r)})
	if err != nil {
		t.Fatal(err)
	}
	override := &sampleCRefreshOverride{}
	refresh, err := NewWidgetRefreshController(WidgetRefreshControllerDeps{Overrides: override})
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := NewWidgetServiceAdapter(widget, refresh)
	if err != nil {
		t.Fatal(err)
	}
	created, err := adapter.CreateWidget(context.Background(), &testpb.CreateWidgetRequest{Name: "sample-c", Enabled: true})
	if err != nil || created.Item == nil || created.Item.Name != "sample-c" {
		t.Fatalf("standard CRUD delegation failed: resp=%+v err=%v", created, err)
	}
	if _, err := adapter.RefreshWidget(context.Background(), &testpb.RefreshWidgetRequest{Id: 99}); err != nil {
		t.Fatal(err)
	}
	if override.calls != 1 {
		t.Fatalf("Override calls = %d, want 1", override.calls)
	}
}
`

// real DictType + DictData schemas must render complete generated
// controllers plus one typed DictServiceAdapter, and DictData must accept a
// typed injected repository while DictType stays on the default adapter.
func TestDictServiceGeneratedSourcesCompile(t *testing.T) {
	compileServicePlansWithRuntime(t, dictServicePlans(t), dictServiceAdapterRuntimeTest)
}

const dictServiceAdapterRuntimeTest = `package gen

import (
	"context"
	"testing"

	adminv1connect "github.com/abysstract/abysstract-admin/backend/gen/abysstract/admin/v1/adminv1connect"
	"github.com/abysstract/abysstract-admin/backend/internal/dal/model"
	"github.com/abysstract/abysstract-admin/backend/internal/repo"
	"github.com/abysstract/abysstract-admin/backend/internal/testutil"
	"github.com/abysstract/abysstract-admin/backend/resource"
)

var _ adminv1connect.DictServiceHandler = (*DictServiceAdapter)(nil)

type dictDataInjectedRepository struct{ inner *DictDataRepository }

func (d *dictDataInjectedRepository) Create(ctx context.Context, rSession resource.Session, m *model.SysDictDatum) error {
	r := repo.New(rSession.DB())
	return d.inner.Create(ctx, r, m)
}
func (d *dictDataInjectedRepository) GetByKey(ctx context.Context, rSession resource.Session, key int64, access repo.AccessScope) (*model.SysDictDatum, error) {
	r := repo.New(rSession.DB())

	return d.inner.GetByKey(ctx, r, key, access)
}
func (d *dictDataInjectedRepository) GetByKeyForUpdate(ctx context.Context, rSession resource.Session, key int64, access repo.AccessScope) (*model.SysDictDatum, error) {
	r := repo.New(rSession.DB())

	return d.inner.GetByKeyForUpdate(ctx, r, key, access)
}
func (d *dictDataInjectedRepository) Update(ctx context.Context, rSession resource.Session, key int64, m *model.SysDictDatum, columns repo.ColumnSet, access repo.AccessScope) error {
	r := repo.New(rSession.DB())

	return d.inner.Update(ctx, r, key, m, columns, access)
}
func (d *dictDataInjectedRepository) DeleteByKey(ctx context.Context, rSession resource.Session, key int64, access repo.AccessScope) error {
	r := repo.New(rSession.DB())

	return d.inner.DeleteByKey(ctx, r, key, access)
}
func (d *dictDataInjectedRepository) GetManyForUpdate(ctx context.Context, rSession resource.Session, keys []int64, access repo.AccessScope) ([]*model.SysDictDatum, error) {
	r := repo.New(rSession.DB())

	return d.inner.GetManyForUpdate(ctx, r, keys, access)
}
func (d *dictDataInjectedRepository) BatchDelete(ctx context.Context, rSession resource.Session, keys []int64, access repo.AccessScope) (int64, error) {
	r := repo.New(rSession.DB())

	return d.inner.BatchDelete(ctx, r, keys, access)
}
func (d *dictDataInjectedRepository) List(ctx context.Context, rSession resource.Session, q DictDataListQuery, access repo.AccessScope) ([]*model.SysDictDatum, int64, error) {
	r := repo.New(rSession.DB())

	return d.inner.List(ctx, r, q, access)
}

var _ resource.Repository[model.SysDictDatum, DictDataListQuery] = (*dictDataInjectedRepository)(nil)

func TestDictServiceConstructorsValidateTypedDeps(t *testing.T) {
	baseRepo := repo.New(testutil.OpenDB(t))
	if _, err := NewDictTypeController(DictTypeControllerDeps{}); err == nil {
		t.Fatal("DictType controller accepted nil BaseRepo")
	}
	if _, err := NewDictDataController(DictDataControllerDeps{Store: testutil.Store(t, baseRepo)}); err == nil {
		t.Fatal("DictData controller accepted nil injected Repository")
	}
	dictType, err := NewDictTypeController(DictTypeControllerDeps{Store: testutil.Store(t, baseRepo)})
	if err != nil {
		t.Fatal(err)
	}
	dictData, err := NewDictDataController(DictDataControllerDeps{Store: testutil.Store(t, baseRepo), Repository: &dictDataInjectedRepository{inner: NewDictDataRepository()}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewDictServiceAdapter(nil, dictData); err == nil {
		t.Fatal("DictServiceAdapter accepted nil DictType controller")
	}
	if _, err := NewDictServiceAdapter(dictType, nil); err == nil {
		t.Fatal("DictServiceAdapter accepted nil DictData controller")
	}
	if _, err := NewDictServiceAdapter(dictType, dictData); err != nil {
		t.Fatal(err)
	}
}
`
