package resourcegen

import (
	"strings"
	"testing"

	"github.com/abysstract/abysstract-admin/backend/internal/resource/coredefs"
	"github.com/abysstract/abysstract-admin/backend/internal/resource/schematest"
	"github.com/abysstract/abysstract-admin/backend/resource/schema"
)

// renderTSOperationsFor renders the generated TS operations file for a plan.
func renderTSOperationsFor(t *testing.T, plan *schema.Plan) string {
	t.Helper()
	files, err := renderTSOperations(plan)
	if err != nil {
		t.Fatalf("renderTSOperations: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.name, ".operations.gen.ts") {
			return string(f.content)
		}
	}
	t.Fatalf("no .operations.gen.ts output in %v", files)
	return ""
}

// the Operations template must emit the unified typed facade for a
// resource with the standard six, binding RPC roles only from validated IR.
func TestRenderTSOperationsEmitsUnifiedFacade(t *testing.T) {
	code := renderTSOperationsFor(t, widgetPlan(t))

	for _, want := range []string{
		// imports: raw + schema + protobuf-ES types + contract types
		`import type { WidgetRaw } from "./widget.raw.gen";`,
		`import { widgetSchema } from "./widget.schema.gen";`,
		`import type { WidgetEntity, CreateWidgetRequest, ListWidgetsRequest } from "../testpb/test_pb";`,
		`ResourceUpdateSubmission,`,
		`SortableFieldName`,
		// typed filter/order aliases
		`export type WidgetFilters = Pick<ListWidgetsRequest, "enabled">;`,
		`export type WidgetOrderField = SortableFieldName<typeof widgetSchema>;`,
		// keyOf keeps int64 as bigint
		`keyOf(entity: Readonly<WidgetEntity>): bigint {`,
		`return entity.id;`,
		// list maps page/keyword/orders/filters onto the typed request
		`async list(`,
		`query: ResourceListQuery<WidgetFilters, WidgetOrderField>,`,
		`Promise<ResourceListResult<WidgetEntity>>`,
		`page: { pageNum: query.pageNum, pageSize: query.pageSize },`,
		`keyword: query.keyword ?? "",`,
		`orders: query.orders?.map((order) => ({ field: order.field, desc: order.desc })) ?? [],`,
		`const filters = query.filters;`,
		`if (filters !== undefined && filters.enabled !== undefined) {`,
		`request.enabled = filters.enabled;`,
		`total: page.total,`,
		// get/create/update/delete/batchDelete bind the raw RPC methods
		`async get(key: bigint): Promise<WidgetEntity> {`,
		`await raw.getWidget({ id: key });`,
		`export type WidgetCreateValues = Parameters<WidgetRaw["createWidget"]>[0];`,
		`async create(values: Readonly<WidgetCreateValues>): Promise<WidgetEntity> {`,
		`await raw.createWidget(values);`,
		`update: widgetUpdate,`,
		`async delete(key: bigint): Promise<void> {`,
		`await raw.deleteWidget({ id: key });`,
		`async batchDelete(keys: readonly bigint[]): Promise<void> {`,
		`await raw.batchDeleteWidgets({ ids: [...keys] });`,
		// frozen contract satisfaction
		`satisfies ResourceOperations<`,
		`WidgetEntity,`,
		`bigint,`,
		`WidgetFilters,`,
		`WidgetCreateValues,`,
		`WidgetUpdateValues,`,
		`void,`,
		`WidgetOrderField`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated facade missing %q\n---\n%s", want, code)
		}
	}
}

// both update paths must funnel through the generated buildUpdateRequest,
// so presence/touched semantics live in exactly one builder.
func TestRenderTSOperationsUpdateReusesGeneratedBuilder(t *testing.T) {
	code := renderTSOperationsFor(t, widgetPlan(t))

	for _, want := range []string{
		`raw.updateWidget(`,
		`widgetBuildUpdateRequest(submission.key, submission.values, submission.touched),`,
		`if (response.item === undefined) {`,
		`Widget Update response is missing item`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated update missing %q\n---\n%s", want, code)
		}
	}
	if strings.Contains(code, "changedFields") {
		t.Error("generated facade must never contain changed-fields logic")
	}
	if strings.Contains(code, "any") {
		t.Error("generated facade must not use any")
	}
}

// the legacy identifier/values/touched update shape must remain callable
// while the submission overload satisfies the frozen contract.
func TestRenderTSOperationsUpdateOverloads(t *testing.T) {
	code := renderTSOperationsFor(t, widgetPlan(t))

	for _, want := range []string{
		`async function widgetUpdate(`,
		`submission: ResourceUpdateSubmission<bigint, WidgetUpdateValues>,`,
		`identifier: bigint,`,
		`values: WidgetUpdateValues,`,
		`touched: WidgetUpdateTouched,`,
		`submission = { key: submissionOrIdentifier, values, touched, dirty: touched };`,
		`} else {`,
		`submission = submissionOrIdentifier;`,
		`dirty: touched`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated overloads missing %q\n---\n%s", want, code)
		}
	}
}

// the static registry must expose the raw List registry (relation
// loader path) plus full facade entries, all as plain object literals.
func TestRenderTSOperationsRegistryEmitsFacadeEntries(t *testing.T) {
	plan := widgetPlan(t)
	out, err := renderTSOperationsRegistry([]*schema.Plan{plan})
	if err != nil {
		t.Fatalf("renderTSOperationsRegistry: %v", err)
	}
	if out.name != "resource-operations.gen.ts" {
		t.Fatalf("registry file name = %q", out.name)
	}
	code := string(out.content)

	for _, want := range []string{
		`import { createWidgetRaw } from "./widget.raw.gen";`,
		`import { createWidgetOperations } from "./widget.operations.gen";`,
		`export function createResourceOperations(transport: Transport)`,
		`const widgetRaw = createWidgetRaw(transport);`,
		`Widget: widgetRaw,`,
		`Widget: createWidgetOperations(widgetRaw),`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("registry missing %q\n---\n%s", want, code)
		}
	}
	if strings.Contains(code, "register(") || strings.Contains(code, "init(") {
		t.Errorf("registry must stay side-effect-free\n---\n%s", code)
	}
}

// a resource without the full standard six must not emit a facade that
// would otherwise smuggle incomplete bindings into the contract.
func TestRenderTSOperationsSkipsFacadeWithoutFullSix(t *testing.T) {
	files, err := renderTSOperations(fiveOperationWidgetPlan(t))
	if err != nil {
		t.Fatalf("renderTSOperations: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected a raw + legacy operations file for a five-operation resource")
	}
	for _, f := range files {
		code := string(f.content)
		if strings.Contains(code, "satisfies ResourceOperations") {
			t.Fatalf("resource without full six must not emit a facade\n---\n%s", code)
		}
	}
}

// A Partial Resource may still expose a standard List and several typed
// CUSTOM workflows. Its generated frontend object must therefore satisfy the
// read-only contract only; ordinary profile fields may use the generated
// presence builder, while password/relationship fields stay outside it.
func TestRenderTSOperationsPartialUserEmitsReadFacade(t *testing.T) {
	plan, err := coredefs.UserResource.Build()
	if err != nil {
		t.Fatalf("build User: %v", err)
	}
	code := renderTSOperationsFor(t, plan)

	for _, want := range []string{
		`import type { UserRaw } from "./user.raw.gen";`,
		`import { userSchema } from "./user.schema.gen";`,
		`export type UserCreateValues = Pick<`,
		`"username" | "nickname" | "email" | "phone" | "deptId" | "remark"`,
		`export type UserUpdateValues = {`,
		`userBuildUpdateRequest(`,
		`id: identifier,`,
		`if (touched.has("nickname"))`,
		`custom: {`,
		`createUser(request: Parameters<UserRaw["createUser"]>[0])`,
		`async list(`,
		`satisfies ResourceReadOperations<`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("partial User facade missing %q\n---\n%s", want, code)
		}
	}

	for _, forbidden := range []string{
		`satisfies ResourceOperations<`,
		`async get(`,
		`async create(`,
		`async update(`,
		`async delete(`,
		`async batchDelete(`,
	} {
		if strings.Contains(code, forbidden) {
			t.Errorf("partial User facade must not emit standard CRUD member %q\n---\n%s", forbidden, code)
		}
	}
}

// keywordOptionalListRequest mirrors a standard List request whose keyword
// field has protobuf presence, so the facade must not materialize "".
type keywordOptionalListRequest struct {
	Page    struct{} `protobuf:"bytes,1,opt,name=page,json=page"`
	Keyword string   `protobuf:"bytes,2,opt,name=keyword,json=keyword,proto3,oneof"`
}

// 评审项 1：list facade 必须按 Keyword 字段的可选性区分 presence——必填
// keyword 归一化为 ""，optional keyword 原样转发（不设即缺省）。
func TestRenderTSOperationsKeywordPresenceForms(t *testing.T) {
	required := renderTSOperationsFor(t, widgetPlan(t))
	if !strings.Contains(required, `keyword: query.keyword ?? "",`) {
		t.Fatalf("required keyword must normalize to empty string\n---\n%s", required)
	}
	if strings.Contains(required, "keyword: query.keyword,") {
		t.Fatalf("required keyword must not be forwarded as undefined-presence\n---\n%s", required)
	}

	optional := widgetPlan(t)
	mutated := false
	for i := range optional.Operations {
		if optional.Operations[i].Kind != schema.OperationList {
			continue
		}
		optional.Operations[i].Operation.Request = (*keywordOptionalListRequest)(nil)
		mutated = true
	}
	if !mutated {
		t.Fatal("widget plan has no List operation to mutate")
	}
	code := renderTSOperationsFor(t, optional)
	if !strings.Contains(code, "keyword: query.keyword,") {
		t.Fatalf("optional keyword must be forwarded as-is without defaulting\n---\n%s", code)
	}
	if strings.Contains(code, `keyword: query.keyword ?? "",`) {
		t.Fatalf("optional keyword must not be normalized to empty string\n---\n%s", code)
	}
}

// 评审项 3：无全量六操作的资源必须导出独立命名的 legacy 适配器。
func TestRenderTSOperationsLegacyExportName(t *testing.T) {
	code := renderTSOperationsFor(t, fiveOperationWidgetPlan(t))
	if !strings.Contains(code, "export function createWidgetLegacyOperations(") {
		t.Fatalf("legacy adapter must use WidgetLegacyOperations export name\n---\n%s", code)
	}
	if strings.Contains(code, "export function createWidgetOperations(") && strings.Contains(code, "satisfies ResourceOperations") {
		t.Fatalf("legacy adapter must not shadow the full facade export name\n---\n%s", code)
	}
	if strings.Contains(code, "satisfies ResourceOperations") {
		t.Fatalf("legacy adapter must not satisfy ResourceOperations\n---\n%s", code)
	}
}

// fiveOperationWidgetPlan returns a standard widget plan without Get.
func fiveOperationWidgetPlan(t *testing.T) *schema.Plan {
	t.Helper()
	res := schematest.WidgetFixtureResource()
	kept := res.Operations[:0]
	for _, op := range res.Operations {
		if op.Kind != schema.OperationGet {
			kept = append(kept, op)
		}
	}
	res.Operations = kept
	plan, err := res.Build()
	if err != nil {
		t.Fatalf("build without Get: %v", err)
	}
	return plan
}
