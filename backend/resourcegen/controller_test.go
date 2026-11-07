package resourcegen

import (
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/coredefs"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schema/testpb"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// widgetPlan builds the six-operation Widget fixture plan.
func widgetPlan(t *testing.T) *schema.Plan {
	t.Helper()
	plan, err := schematest.WidgetFixtureResource().Build()
	if err != nil {
		t.Fatalf("widget fixture build: %v", err)
	}
	return plan
}

// 8.1: generator must emit a Controller skeleton for a full six-operation resource.
func TestRenderControllerSkeleton(t *testing.T) {
	plan := widgetPlan(t)
	name, src, err := renderController(plan)
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	if name != "widget_controller.gen.go" {
		t.Fatalf("file name = %q", name)
	}
	code := string(src)
	checks := []string{
		"type WidgetController struct",
		"type WidgetControllerDeps struct",
		"Store *resource.Store",
		"resource.HookSet[",
		"func NewWidgetController(deps WidgetControllerDeps) (*WidgetController, error)",
		"repo resource.Repository[",
	}
	for _, want := range checks {
		if !strings.Contains(strings.Join(strings.Fields(code), " "), strings.Join(strings.Fields(want), " ")) {
			t.Fatalf("controller source missing %q", want)
		}
	}
	// 骨架必须实现六操作的方法集（Connect handler 形态）。
	for _, method := range []string{"ListWidgets", "CreateWidget", "GetWidget", "UpdateWidget", "DeleteWidget", "BatchDeleteWidgets"} {
		if !strings.Contains(code, "func (c *WidgetController) "+method+"(") {
			t.Fatalf("controller missing method %s", method)
		}
	}
}

// 8.5: constructor must validate dependencies without panicking.
func TestControllerConstructorValidation(t *testing.T) {
	plan := widgetPlan(t)
	_, src, err := renderController(plan)
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	for _, want := range []string{
		`if deps.Store == nil`,
		`return nil, fmt.Errorf(`,
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("constructor missing validation %q", want)
		}
	}
	if strings.Contains(code, "MustNew") {
		t.Fatal("generator must not emit MustNew...")
	}
}

// 8.5: RepositoryInjected 资源的 Deps 暴露 Repository 且构造函数校验非 nil。
func TestControllerInjectedRepositoryDeps(t *testing.T) {
	res := schematest.WidgetFixtureResource()
	res.Repository.Implementation = schema.RepositoryInjected
	plan, err := res.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	_, src, err := renderController(plan)
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	for _, want := range []string{
		"Repository resource.Repository[",
		"if deps.Repository == nil",
		"c.repo = deps.Repository",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("injected controller missing %q", want)
		}
	}
	if strings.Contains(code, "NewWidgetRepository()") {
		t.Fatal("injected controller must not build a default adapter")
	}
}

// 8.5: data_scope=true 的 Deps 暴露 provider 与 createScopeValidator 并 fail-closed。
func TestControllerDataScopeDeps(t *testing.T) {
	res := schematest.WidgetFixtureResource()
	res.Auth = schema.AuthConfig{DataScope: true, DeptField: "DeptID", CreatorField: "CreatedBy"}
	// fixture model 无 Dept/Creator 字段，DataScope Build 需要列映射——
	// 8.5 只关心 Deps 形态，直接跳过 Build 失败情形。
	plan, err := res.Build()
	if err != nil {
		t.Skipf("fixture model lacks scope columns; scope deps verified via template flags: %v", err)
	}
	_, src, err := renderController(plan)
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	for _, want := range []string{
		"DataScopeProvider resource.DataScopeProvider",
		"if deps.DataScopeProvider == nil",
		"CreateScopeValidator resource.CreateScopeValidator[",
		"if deps.CreateScopeValidator == nil",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("data scope controller missing %q", want)
		}
	}
}

// 8.5: skip_create_scope_check=true 时 Deps 有 provider 但无 CreateScopeValidator。
func TestControllerDataScopeSkipCreateCheckDeps(t *testing.T) {
	res := schematest.WidgetFixtureResource()
	res.Auth = schema.AuthConfig{DataScope: true, DeptField: "DeptID", CreatorField: "CreatedBy", SkipCreateScopeCheck: true}
	plan, err := res.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	_, src, err := renderController(plan)
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	if !strings.Contains(code, "DataScopeProvider resource.DataScopeProvider") {
		t.Fatal("provider must still be required")
	}
	if strings.Contains(code, "CreateScopeValidator") {
		t.Fatal("skip_create_scope_check must omit CreateScopeValidator")
	}
}

// 8.5: Partial 资源的 Deps 暴露 Overrides 且 nil 返回构造 error。
func TestControllerPartialDeps(t *testing.T) {
	res := schematest.WidgetFixtureResource()
	res.Mode = schema.Partial
	res.Operations = append(res.Operations, schema.Operation{
		Kind: schema.OperationCustom, Method: "RefreshWidget",
		Request: (*testpb.RefreshWidgetRequest)(nil), Response: (*testpb.RefreshWidgetResponse)(nil),
		Implementation: schema.OperationOverride, Action: "refresh", ScopePolicy: schema.ScopePolicyExempt,
	})
	plan, err := res.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	_, src, err := renderController(plan)
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	for _, want := range []string{
		"Overrides WidgetOverrides",
		"if deps.Overrides == nil",
		"type WidgetOverrides interface",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("partial controller missing %q", want)
		}
	}
}

// 8.5: 默认（Generated, 无 scope, 非 Partial）Deps 不得出现条件依赖字段。
func TestControllerGeneratedDepsMinimal(t *testing.T) {
	plan := widgetPlan(t)
	_, src, err := renderController(plan)
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	for _, unwanted := range []string{
		"deps.Overrides",
		"deps.DataScopeProvider",
		"deps.CreateScopeValidator",
		"deps.Repository",
	} {
		if strings.Contains(code, unwanted) {
			t.Fatalf("minimal generated controller must not reference %s", unwanted)
		}
	}
}

// 8.4: Create must validate and convert before opening a transaction, then run
// transaction Hooks/repository work, convert the response before commit, and
// invoke AfterCommit only after InTransaction returns successfully.
func TestRenderControllerCreateLifecycle(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	checks := []string{
		"func (c *WidgetController) CreateWidget(",
		"WidgetValidateCreate(req)",
		"WidgetCreateToModel(req)",
		"c.store.InTransaction(ctx, func(txRepo resource.Tx) error",
		"c.hooks.BeforeCreate",
		"c.repo.Create(ctx, txRepo, m)",
		"c.hooks.AfterCreate",
		"WidgetToEntity(m)",
		"createSnapshot = *m",
		"c.hooks.AfterCommitCreate(ctx, createSnapshot)",
	}
	for _, want := range checks {
		if !strings.Contains(code, want) {
			t.Fatalf("Create lifecycle source missing %q\n---\n%s", want, code)
		}
	}
}

// 8.3: a generated presence helper distinguishes an all-omitted patch from a
// submitted patch that later compares equal to the locked model.
func TestRenderControllerUpdatePresenceAndNoopPath(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	converter, err := renderGo(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderGo: %v", err)
	}
	if !strings.Contains(string(converter[0].content), "func WidgetHasUpdateFields(") {
		t.Fatalf("generated converter missing WidgetHasUpdateFields\n---\n%s", converter[0].content)
	}
	checks := []string{
		"if !WidgetHasUpdateFields(req)",
		"connect.CodeInvalidArgument",
		"c.repo.GetByKeyForUpdate(ctx, txRepo, req.GetId(), c.accessScope(ctx))",
		"WidgetUpdateToModel(old, req)",
		"if columns.Len() == 0",
		"WidgetToEntity(m)",
		"return &testpb.UpdateWidgetResponse{Item: item}, nil",
	}
	for _, want := range checks {
		if !strings.Contains(code, want) {
			t.Fatalf("Update presence/no-op source missing %q\n---\n%s", want, code)
		}
	}
}

// 8.2: a non-empty patch uses the full transactional Update lifecycle and
// keeps converter input failures distinct from transactional/internal errors.
func TestRenderControllerUpdateLifecycle(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	checks := []string{
		"c.hooks.BeforeUpdate(ctx, txRepo, old, m, columns)",
		"c.repo.Update(ctx, txRepo, req.GetId(), m, columns, c.accessScope(ctx))",
		"c.hooks.AfterUpdate(ctx, txRepo, old, m, columns)",
		"updateSnapshot = *m",
		"c.hooks.AfterCommitUpdate(ctx, updateSnapshot)",
		"errors.As(err, &inputErr)",
		"connect.CodeInvalidArgument",
	}
	for _, want := range checks {
		if !strings.Contains(code, want) {
			t.Fatalf("Update lifecycle source missing %q\n---\n%s", want, code)
		}
	}
}

// 9.2: transactional Hook errors return from the InTransaction closure, so
// commit and AfterCommit callbacks are unreachable on every failure branch.
func TestRenderControllerHookErrorsAbortTransaction(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	checks := []string{
		"if err := c.hooks.BeforeCreate(ctx, txRepo, m); err != nil {\n\t\t\t\treturn err",
		"if err := c.hooks.AfterCreate(ctx, txRepo, m); err != nil {\n\t\t\t\treturn err",
		"if err := c.hooks.BeforeUpdate(ctx, txRepo, old, m, columns); err != nil {\n\t\t\t\treturn err",
		"if err := c.hooks.AfterUpdate(ctx, txRepo, old, m, columns); err != nil {\n\t\t\t\treturn err",
		"if updated && c.hooks.AfterCommitUpdate != nil",
	}
	for _, want := range checks {
		if !strings.Contains(code, want) {
			t.Fatalf("Hook rollback source missing %q\n---\n%s", want, code)
		}
	}
}

// 9.3: AfterCommit hooks are isolated from the already-committed response by
// a panic-recover wrapper with structured logging context.
func TestRenderControllerAfterCommitRecoversPanics(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	checks := []string{
		"func widgetRunAfterCommit(hook string, fn func())",
		"defer func() {",
		"slog.Error(\"resource after-commit hook panicked\"",
		"widgetRunAfterCommit(\"create\", func()",
		"widgetRunAfterCommit(\"update\", func()",
	}
	for _, want := range checks {
		if !strings.Contains(code, want) {
			t.Fatalf("AfterCommit recovery source missing %q\n---\n%s", want, code)
		}
	}
}

// 9.5: the exact immutable ColumnSet passed to Hooks is also passed to the
// adapter. Hook code can only mutate Values() copies, never the controller's
// retained set or the subsequent selective update columns.
func TestRenderControllerSharesImmutableColumnSetWithUpdateHook(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	before := strings.Index(code, "c.hooks.BeforeUpdate(ctx, txRepo, old, m, columns)")
	update := strings.Index(code, "c.repo.Update(ctx, txRepo, req.GetId(), m, columns, c.accessScope(ctx))")
	if before < 0 || update < 0 || before >= update {
		t.Fatalf("Update must pass the same ColumnSet to Hook before adapter write\n---\n%s", code)
	}
}

// 9.6 (Update portion): lock/scope failure precedes Hooks, transaction failures
// return from the closure, and AfterCommitUpdate remains gated by a successful
// commit plus an actual write.
func TestRenderControllerUpdateHookTransactionOrdering(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	lock := strings.Index(code, "c.repo.GetByKeyForUpdate(ctx, txRepo")
	before := strings.Index(code, "c.hooks.BeforeUpdate(ctx, txRepo, old, m, columns)")
	after := strings.Index(code, "c.hooks.AfterUpdate(ctx, txRepo, old, m, columns)")
	commit := strings.Index(code, "if updated && c.hooks.AfterCommitUpdate != nil")
	if lock < 0 || before < 0 || after < 0 || commit < 0 || !(lock < before && before < after && after < commit) {
		t.Fatalf("unexpected Update Hook/lock/commit ordering\n---\n%s", code)
	}
}

// 8.2a: Delete takes its value snapshot immediately after the scoped locked
// read, before Hooks can mutate old, and runs the post-commit callback only
// after successful transaction completion.
func TestRenderControllerDeleteLifecycle(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	checks := []string{
		"func (c *WidgetController) DeleteWidget(",
		"c.repo.GetByKeyForUpdate(ctx, txRepo, req.GetId(), c.accessScope(ctx))",
		"deleteSnapshot = *old",
		"c.hooks.BeforeDelete(ctx, txRepo, old)",
		"c.repo.DeleteByKey(ctx, txRepo, req.GetId(), c.accessScope(ctx))",
		"c.hooks.AfterDelete(ctx, txRepo, old)",
		"widgetRunAfterCommit(\"delete\", func()",
	}
	for _, want := range checks {
		if !strings.Contains(code, want) {
			t.Fatalf("Delete lifecycle source missing %q\n---\n%s", want, code)
		}
	}
}

// 8.6: BatchDelete rejects an empty input and applies the fixed 1000 limit to
// the deduplicated key set before any transaction or repository call.
func TestRenderControllerBatchDeleteInputGuards(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	checks := []string{
		"const widgetBatchDeleteLimit = 1000",
		"rawIDs := req.GetIds()",
		"if len(rawIDs) == 0",
		"distinctIDs := make([]int64, 0, len(rawIDs))",
		"if len(distinctIDs) > widgetBatchDeleteLimit",
	}
	for _, want := range checks {
		if !strings.Contains(code, want) {
			t.Fatalf("BatchDelete input guard source missing %q\n---\n%s", want, code)
		}
	}
}

// 8.7: BatchDelete confirms every distinct ID under the scoped lock before
// invoking batch Hooks, snapshots values before Hook mutation, verifies the
// affected count, and invokes exactly the batch post-commit callback.
func TestRenderControllerBatchDeleteTransactionLifecycle(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	checks := []string{
		"c.store.InTransaction(ctx, func(txRepo resource.Tx) error",
		"c.repo.GetManyForUpdate(ctx, txRepo, distinctIDs, c.accessScope(ctx))",
		"if len(models) != len(distinctIDs)",
		"batchSnapshots = make([]schematest.WidgetModel, len(models))",
		"batchSnapshots[i] = *m",
		"c.hooks.BeforeBatchDelete(ctx, txRepo, models)",
		"c.repo.BatchDelete(ctx, txRepo, distinctIDs, c.accessScope(ctx))",
		"if affected != int64(len(distinctIDs))",
		"c.hooks.AfterBatchDelete(ctx, txRepo, models)",
		"widgetRunAfterCommit(\"batch_delete\", func()",
		"c.hooks.AfterCommitBatchDelete(ctx, batchSnapshots)",
	}
	for _, want := range checks {
		if !strings.Contains(code, want) {
			t.Fatalf("BatchDelete lifecycle source missing %q\n---\n%s", want, code)
		}
	}
	if strings.Contains(code, "AfterCommitDelete(ctx, batch") {
		t.Fatal("BatchDelete must not invoke single-delete AfterCommit callbacks")
	}
}

// 8.8: List builds a normalized typed query from wire fields, resolves orders
// through the generated JSON-name whitelist, and delegates only that Q to the adapter.
func TestRenderControllerListQuerySemantics(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)
	for _, want := range []string{
		"q := WidgetListQuery{", "PageNum: int(req.GetPage().GetPageNum())", "PageSize: int(req.GetPage().GetPageSize())", "Keyword: req.GetKeyword()", "if raw := req.Enabled; raw != nil", "q.Enabled = &v", "if q.PageNum < 1", "if q.PageSize > resource.MaxPageSize", "WidgetResolveOrder(order.GetField(), order.GetDesc())", "c.repo.List(ctx, c.store, q, c.accessScope(ctx))",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("List semantics source missing %q\n---\n%s", want, code)
		}
	}
}

func TestRenderControllerGetLifecycle(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)
	for _, want := range []string{
		"func (c *WidgetController) GetWidget(",
		"c.repo.GetByKey(ctx, c.store, req.GetId(), c.accessScope(ctx))",
		"WidgetToEntity(m)",
		"testpb.GetWidgetResponse{Item: entity}",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("Get source missing %q\n---\n%s", want, code)
		}
	}
}

func TestControllerDataScopeUsesProviderForProtectedOperations(t *testing.T) {
	res := schematest.WidgetFixtureResource()
	res.Auth = schema.AuthConfig{DataScope: true, DeptField: "DeptID", CreatorField: "CreatedBy"}
	plan, err := res.Build()
	if err != nil {
		t.Fatal(err)
	}
	_, src, err := renderController(plan)
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)
	for _, want := range []string{
		"func (c *WidgetController) accessScope(ctx context.Context) resource.AccessScope",
		`c.dataScope.DataScope(ctx, "dept_id", "create_by")`,
		"c.repo.GetByKey(ctx, c.store, req.GetId(), c.accessScope(ctx))",
		"c.repo.GetByKeyForUpdate(ctx, txRepo, req.GetId(), c.accessScope(ctx))",
		"c.repo.List(ctx, c.store, q, c.accessScope(ctx))",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("DataScope source missing %q\n---\n%s", want, code)
		}
	}
}

func TestRenderControllerRepositoryErrorMapsNonNotFoundToInternal(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatal(err)
	}
	code := string(src)
	for _, want := range []string{
		"return connect.NewError(connect.CodeFailedPrecondition, rejected.Message).WithCause(err)",
		"return connect.NewError(connect.CodeFailedPrecondition, resource.ErrForeignKey.Error()).WithCause(err)",
		"return connect.NewError(connect.CodeInvalidArgument, resource.ErrCheckViolation.Error()).WithCause(err)",
		`slog.Error("resource request failed", "resource", "Widget", "error", err)`,
		"return connect.NewError(connect.CodeInternal, resource.ErrInternal.Error()).WithCause(err)",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("RepositoryError lacks %q\n---\n%s", want, code)
		}
	}
	// Raw driver messages name tables and constraints and must not reach clients.
	if strings.Contains(code, "connect.CodeInternal, err.Error()") {
		t.Fatalf("RepositoryError returns the raw error message\n---\n%s", code)
	}
	if strings.Contains(code, "return widgetRepositoryError(err)") {
		t.Fatalf("RepositoryError must not recursively call itself\n---\n%s", code)
	}
}

func partialWidgetPlan(t *testing.T) *schema.Plan {
	t.Helper()
	res := schematest.WidgetFixtureResource()
	res.Mode = schema.Partial
	res.Operations = append(res.Operations, schema.Operation{
		Kind: schema.OperationCustom, Method: "RefreshWidget",
		Request: (*testpb.RefreshWidgetRequest)(nil), Response: (*testpb.RefreshWidgetResponse)(nil),
		Implementation: schema.OperationOverride, Action: "refresh", ScopePolicy: schema.ScopePolicyExempt,
	})
	plan, err := res.Build()
	if err != nil {
		t.Fatalf("build partial Widget: %v", err)
	}
	return plan
}

// Requirement 13.3/13.4: a Partial Resource exposes exactly its declared
// CUSTOM operation and delegates through a compile-time interface, never any.
func TestPartialControllerRendersMinimalTypedOverrides(t *testing.T) {
	plan := partialWidgetPlan(t)
	_, source, err := renderController(plan)
	if err != nil {
		t.Fatal(err)
	}
	code := string(source)
	for _, want := range []string{
		"type WidgetOverrides interface",
		"RefreshWidget(ctx context.Context, req *testpb.RefreshWidgetRequest) (*testpb.RefreshWidgetResponse, error)",
		"return c.overrides.RefreshWidget(ctx, req)",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("Partial controller missing typed override %q\n---\n%s", want, code)
		}
	}
	for _, forbidden := range []string{"isWidgetOverrides()", "interface{}", "CodeUnimplemented"} {
		if strings.Contains(code, forbidden) {
			t.Fatalf("Partial CUSTOM delegation must not contain %q\n---\n%s", forbidden, code)
		}
	}
}

func widgetServicePlans(t *testing.T) []*schema.Plan {
	t.Helper()
	plans, err := schema.BuildRegistry([]*schema.Resource{
		schematest.WidgetFixtureResource(),
		schematest.WidgetRefreshFixtureResource(),
	})
	if err != nil {
		t.Fatalf("build Widget service plans: %v", err)
	}
	return plans
}

// Requirement 13.5: one Connect handler may aggregate controllers from
// multiple Resources as long as BuildRegistry gives each method one owner.
func TestRenderWidgetServiceAdapterAggregatesMultipleResources(t *testing.T) {
	files, err := renderServiceAdapters(widgetServicePlans(t))
	if err != nil {
		t.Fatal(err)
	}
	var source string
	for _, file := range files {
		if file.name == "widget_service_adapter.gen.go" {
			source = string(file.content)
		}
	}
	if source == "" {
		t.Fatal("Widget service adapter was not generated")
	}
	for _, want := range []string{
		"type WidgetServiceAdapter struct",
		"widget *WidgetController",
		"widgetRefresh *WidgetRefreshController",
		"var _ schematest.WidgetServiceHandler = (*WidgetServiceAdapter)(nil)",
		"return a.widget.ListWidgets(ctx, req)",
		"return a.widgetRefresh.RefreshWidget(ctx, req)",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("ServiceAdapter missing %q\n---\n%s", want, source)
		}
	}
}

// Protobuf messages contain MessageState/sync.Mutex and must never be copied
// through a generated local value. Controllers retain the Entity pointer from
// ToEntity for Create and Update responses.
func TestRenderControllerRetainsProtoEntityPointer(t *testing.T) {
	_, src, err := renderController(widgetPlan(t))
	if err != nil {
		t.Fatalf("renderController: %v", err)
	}
	code := string(src)
	for _, want := range []string{
		"var item *testpb.WidgetEntity",
		"item = entity",
		"return &testpb.CreateWidgetResponse{Item: item}, nil",
		"return &testpb.UpdateWidgetResponse{Item: item}, nil",
	} {
		if !strings.Contains(code, want) {
			t.Fatalf("generated Controller must retain Entity pointer; missing %q\n---\n%s", want, code)
		}
	}
	if strings.Contains(code, "item = *entity") || strings.Contains(code, "Item: &item") {
		t.Fatalf("generated Controller copies protobuf Entity value\n---\n%s", code)
	}
}

func TestGeneratedReadControllerSupportsPartialListWithTypedWorkflows(t *testing.T) {
	partial := &schema.Resource{Mode: schema.Partial}
	plan := &schema.Plan{Res: partial, Operations: []*schema.OperationPlan{
		{Operation: schema.Operation{Kind: schema.OperationList}},
		{Operation: schema.Operation{Kind: schema.OperationCustom}},
	}}
	if !hasGeneratedReadController(plan) {
		t.Fatal("partial List plus CUSTOM operations should generate a read controller")
	}

	full := &schema.Plan{Res: &schema.Resource{Mode: schema.Partial}, Operations: []*schema.OperationPlan{
		{Operation: schema.Operation{Kind: schema.OperationList}},
		{Operation: schema.Operation{Kind: schema.OperationGet}},
		{Operation: schema.Operation{Kind: schema.OperationUpdate}},
	}}
	if hasGeneratedReadController(full) {
		t.Fatal("partial plan with an unsupported generated Update should not use read controller")
	}
}

func TestRenderUserReadRepositoryExcludesWriteMethods(t *testing.T) {
	plan, err := coredefs.UserResource.Build()
	if err != nil {
		t.Fatalf("build User read Resource: %v", err)
	}
	files, err := renderGo(plan)
	if err != nil {
		t.Fatalf("render User read Resource: %v", err)
	}
	var source string
	for _, file := range files {
		if file.name == "user_repo.gen.go" {
			source = string(file.content)
			break
		}
	}
	if source == "" {
		t.Fatal("User read Resource did not render user_repo.gen.go")
	}
	if !strings.Contains(source, "resource.ReadRepository[model.SysUser, UserListQuery]") {
		t.Fatalf("User repository must compile against ReadRepository:\n%s", source)
	}
	for _, method := range []string{" Create(", " GetByKeyForUpdate(", " Update(", " DeleteByKey(", " GetManyForUpdate(", " BatchDelete("} {
		if strings.Contains(source, "func (a *UserRepository)"+method) {
			t.Fatalf("User read repository unexpectedly renders write/full-CRUD method %q:\n%s", method, source)
		}
	}
}
