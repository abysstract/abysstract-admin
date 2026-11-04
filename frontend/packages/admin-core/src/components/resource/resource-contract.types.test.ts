import { create } from "@bufbuild/protobuf";
import type {
  CreateDictDataRequest,
  DictData as GeneratedDictData,
  ListDictDataRequest,
} from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import {
  CreateDictDataRequestSchema,
  DictDataSchema,
  ListDictDataResponseSchema,
} from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { dictDataSchema } from "../../gen/resource/dict-data.schema.gen";
import {
  dictDataBuildUpdateRequest,
  type DictDataUpdateValues,
} from "../../gen/resource/dict-data.operations.gen";
import { createDictDataRaw } from "../../gen/resource/dict-data.raw.gen";

type DictDataRawT = ReturnType<typeof createDictDataRaw>;
import type { ResourceListResult } from "./operations";
import type {
  ResourceFieldProps,
  ResourceFormCreateProps,
  ResourceFormItemOverrides,
  ResourceFormProps,
  ResourceFormUpdateProps,
} from "./form-contract";
import type {
  ResourceColumnOverrides,
  ResourceRowActionOverride,
  ResourceToolbarOverride,
} from "./metadata";
import {
  toProTableTotal,
  type ResourceListQuery,
  type ResourceOperations,
  type ResourceUpdateSubmission,
} from "./operations";
import type { ResourceTableProductionProps } from "./table-contract";
import type { ListableFieldName, ResourceSchema, SortableFieldName } from "./schema";
import { describe, expect, it } from "vitest";

// All entity/request/filter types below come from generated protobuf or
// generated Resource output. There is intentionally no handwritten entity DTO.
type DictDataCreateValues = CreateDictDataRequest;
type DictDataFilters = Pick<ListDictDataRequest, "dictType">;
type DictDataKey = GeneratedDictData["id"];
type DictDataOrderField = SortableFieldName<typeof dictDataSchema>;

const syntheticServerManagedSchema = {
  name: "Synthetic",
  permPrefix: "system:synthetic",
  table: "sys_synthetic",
  fields: [
    {
      name: "secret",
      protoName: "secret",
      tsType: "string",
      required: false,
      searchable: false,
      filterable: false,
      sortable: true,
      listable: true,
      maxLen: 0,
      creatable: true,
      updatable: true,
      serverManaged: true,
    },
  ],
} as const satisfies ResourceSchema;

const syntheticSortableField: SortableFieldName<typeof syntheticServerManagedSchema> = "secret";
const syntheticListableField: ListableFieldName<typeof syntheticServerManagedSchema> = "secret";

const generatedEntity = create(DictDataSchema);
const generatedCreateValues = create(CreateDictDataRequestSchema);
const generatedListResponse = create(ListDictDataResponseSchema);
const generatedUpdateValues: DictDataUpdateValues = {
  dictSort: 0,
  dictLabel: "label",
  dictValue: "value",
  isDefault: 0,
  remark: "",
};
type GeneratedDictDataListResponse = Awaited<ReturnType<DictDataRawT["listDictData"]>>;

function adaptGeneratedListResponse(
  response: GeneratedDictDataListResponse,
  query: ResourceListQuery<DictDataFilters, DictDataOrderField>,
): ResourceListResult<GeneratedDictData> {
  if (response.page === undefined) {
    throw new Error("generated List response is missing page info");
  }
  return {
    items: response.items,
    total: response.page.total,
    pageNum: query.pageNum,
    pageSize: query.pageSize,
  };
}

const operations: ResourceOperations<
  GeneratedDictData,
  DictDataKey,
  DictDataFilters,
  DictDataCreateValues,
  DictDataUpdateValues,
  GeneratedDictData,
  void,
  DictDataOrderField
> = {
  keyOf: (entity) => entity.id,
  list: async (query) => ({
    items: [],
    total: 0n,
    pageNum: query.pageNum,
    pageSize: query.pageSize,
  }),
  get: async () => generatedEntity,
  create: async () => generatedEntity,
  update: async () => generatedEntity,
  delete: async () => undefined,
  batchDelete: async () => undefined,
};

const filters: DictDataFilters = { dictType: "notice" };
const query: ResourceListQuery<DictDataFilters, DictDataOrderField> = {
  pageNum: 1,
  pageSize: 20,
  keyword: "notice",
  orders: [{ field: "dictSort", desc: false }],
  filters,
};

const columns: ResourceColumnOverrides<typeof dictDataSchema, GeneratedDictData> = {
  dictLabel: {
    title: "标签",
    valueType: "text",
    searchable: false,
    render: ({ value }) => value.toUpperCase(),
  },
};

const rowActions: readonly ResourceRowActionOverride<GeneratedDictData, bigint>[] = [
  {
    action: "custom",
    name: "archive",
    merge: "append",
    permission: "system:dict:remove",
    onClick: async ({ key }) => operations.delete(key),
  },
  { action: "delete", merge: "replace" },
  { action: "batchDelete", merge: "hide" },
];

const toolbar: readonly ResourceToolbarOverride<GeneratedDictData, bigint>[] = [
  {
    action: "custom",
    name: "export",
    merge: "append",
    permission: "system:dict:list",
  },
  { action: "create", merge: "replace" },
];

const tableProps: ResourceTableProductionProps<
  typeof dictDataSchema,
  GeneratedDictData,
  bigint,
  DictDataFilters,
  DictDataCreateValues,
  DictDataUpdateValues
> = {
  schema: dictDataSchema,
  operations,
  access: { can: (permission) => permission.length > 0 },
  columns,
  rowActions,
  toolbar,
  query: { defaults: { pageNum: 1, pageSize: 20, filters } },
  form: {
    create: ({ schema, operations: createOperations }) => {
      void createOperations.create(generatedCreateValues);
      return schema.name;
    },
    edit: ({ schema, key, operations: updateOperations }) => {
      void updateOperations.update(update);
      return `edit ${schema.name}:${key.toString()}`;
    },
  },
  reload: async () => undefined,
};

const update: ResourceUpdateSubmission<bigint, DictDataUpdateValues> = {
  key: 9007199254740993n,
  values: generatedUpdateValues,
  touched: new Set(["dictLabel", "dictSort"]),
  dirty: new Set(["dictLabel"]),
};

// The generated builder remains the adapter's only request construction path.
const generatedUpdateRequest: Parameters<DictDataRawT["updateDictData"]>[0] =
  dictDataBuildUpdateRequest(update.key, update.values, update.touched);

void generatedCreateValues;
void generatedListResponse;
void adaptGeneratedListResponse;
void generatedUpdateRequest;
void operations.get(9007199254740993n);
void operations.update(update);
void operations.delete(9007199254740993n);
void operations.batchDelete([9007199254740993n]);
void operations.list(query);

const formOverrides: ResourceFormItemOverrides<
  typeof dictDataSchema,
  DictDataUpdateValues,
  "update",
  GeneratedDictData
> = {
  dictLabel: {
    label: "标签",
    visible: true,
    disabled: ({ mode }) => mode === "update",
    rules: [{ required: true, maxLength: 100 }],
    fieldProps: { placeholder: "请输入标签" },
  },
};

const numericSelectOverride: ResourceFormItemOverrides<
  typeof dictDataSchema,
  { dictSort: number },
  "update"
> = {
  dictSort: {
    fieldProps: { options: [{ value: 1, label: "一" }] },
  },
};

const bigintFieldProps: ResourceFieldProps<bigint> = {
  options: [{ value: 9007199254740993n, label: "bigint" }],
};

const invalidBigintFieldProps: ResourceFieldProps<bigint> = {
  options: [
    // @ts-expect-error bigint control options cannot use string values.
    { value: "9007199254740993", label: "错误" },
  ],
};

const invalidFieldOptionType: ResourceFormItemOverrides<
  typeof dictDataSchema,
  DictDataUpdateValues,
  "update"
> = {
  dictSort: {
    fieldProps: {
      // @ts-expect-error number field options cannot use string values.
      options: [{ value: "0", label: "零" }],
    },
  },
};

const createFormProps: ResourceFormProps<
  typeof dictDataSchema,
  GeneratedDictData,
  "create",
  bigint,
  DictDataCreateValues,
  DictDataUpdateValues,
  keyof DictDataUpdateValues,
  GeneratedDictData
> = {
  schema: dictDataSchema,
  mode: "create",
  initialValues: generatedCreateValues,
  onCreate: async (submission) => {
    await operations.create(submission.values);
  },
};

const formProps: ResourceFormProps<
  typeof dictDataSchema,
  GeneratedDictData,
  "update",
  bigint,
  DictDataCreateValues,
  DictDataUpdateValues,
  keyof DictDataUpdateValues,
  GeneratedDictData
> = {
  schema: dictDataSchema,
  mode: "update",
  rowKey: 9007199254740993n,
  initialValues: generatedUpdateValues,
  itemOverrides: formOverrides,
  onUpdate: (submission) => operations.update(submission),
};

void tableProps;
void formProps;
void createFormProps;

const invalidProductionSubmit: ResourceFormProps<
  typeof dictDataSchema,
  GeneratedDictData,
  "create",
  bigint,
  DictDataCreateValues,
  DictDataUpdateValues,
  keyof DictDataUpdateValues,
  GeneratedDictData
> = {
  schema: dictDataSchema,
  mode: "create",
  // @ts-expect-error production props do not expose prototype onSubmit.
  onSubmit: async () => undefined,
};

const invalidCreateMode: ResourceFormProps<
  typeof dictDataSchema,
  GeneratedDictData,
  "create",
  bigint,
  DictDataCreateValues,
  DictDataUpdateValues,
  keyof DictDataUpdateValues,
  GeneratedDictData
> = {
  schema: dictDataSchema,
  // @ts-expect-error a create-only production type cannot select update mode.
  mode: "update",
  rowKey: 1n,
  onUpdate: async () => generatedEntity,
};

const invalidSyntheticCreateOverride: ResourceFormItemOverrides<
  typeof syntheticServerManagedSchema,
  { secret: string },
  "create"
> = {
  // @ts-expect-error serverManaged remains excluded even if creatable is true.
  secret: { label: "禁止" },
};

const invalidSyntheticUpdateOverride: ResourceFormItemOverrides<
  typeof syntheticServerManagedSchema,
  { secret: string },
  "update"
> = {
  // @ts-expect-error serverManaged remains excluded even if updatable is true.
  secret: { label: "禁止" },
};

const invalidSyntheticCreateProps: ResourceFormCreateProps<
  typeof syntheticServerManagedSchema,
  { secret: string }
> = {
  schema: syntheticServerManagedSchema,
  mode: "create",
  // @ts-expect-error lifecycle values cannot add a server-managed field.
  initialValues: { secret: "blocked" },
  onCreate: async (submission) => submission.values,
};

const invalidGeneratedUpdateValues: ResourceFormUpdateProps<
  typeof dictDataSchema,
  bigint,
  DictDataUpdateValues & { id: bigint }
> = {
  schema: dictDataSchema,
  mode: "update",
  rowKey: 1n,
  // @ts-expect-error update values cannot add an id/server-managed field.
  initialValues: { id: 1n },
  onUpdate: async () => generatedEntity,
};

// @ts-expect-error ResourceTableProductionProps does not expose an arbitrary order-field generic.
type InvalidTableOrderWidening = ResourceTableProductionProps<
  typeof dictDataSchema,
  GeneratedDictData,
  bigint,
  DictDataFilters,
  DictDataCreateValues,
  DictDataUpdateValues,
  string
>;
const invalidTableOrderWidening: InvalidTableOrderWidening | undefined = undefined;
void invalidTableOrderWidening;

const invalidColumn: ResourceColumnOverrides<typeof dictDataSchema, GeneratedDictData> = {
  // @ts-expect-error keys must be real fields from the generated schema.
  notAField: { title: "错误字段" },
};

const invalidQuery: ResourceListQuery<DictDataFilters, DictDataOrderField> = {
  pageNum: 1,
  pageSize: 20,
  // @ts-expect-error filters cannot introduce an unrelated generated request field.
  filters: { notAFilter: "nope" },
};

const invalidOrder: ResourceListQuery<DictDataFilters, DictDataOrderField> = {
  pageNum: 1,
  pageSize: 20,
  orders: [
    // @ts-expect-error dictLabel is not generated as sortable.
    { field: "dictLabel", desc: false },
  ],
};

// @ts-expect-error a number cannot replace a generated bigint resource key.
void operations.delete(7);

const invalidCapabilityColumn: ResourceColumnOverrides<typeof dictDataSchema, GeneratedDictData> = {
  status: {
    // @ts-expect-error an override cannot enable schema-disabled sorting.
    sortable: true,
  },
};

const invalidStandardPermission: ResourceRowActionOverride<GeneratedDictData, bigint> = {
  action: "delete",
  merge: "replace",
  // @ts-expect-error standard action permissions are fixed by permPrefix.
  permission: "some:unrelated:permission",
};

// @ts-expect-error custom actions must declare an explicit permission.
const invalidCustomAction: ResourceRowActionOverride<GeneratedDictData, bigint> = {
  action: "custom",
  name: "export",
  merge: "append",
  title: "导出",
};

const invalidPresence: ResourceUpdateSubmission<bigint, DictDataUpdateValues> = {
  key: 1n,
  values: generatedUpdateValues,
  // @ts-expect-error touched fields are limited to generated updatable values.
  touched: new Set(["dictType"]),
  dirty: new Set(["dictLabel"]),
};

const invalidUpdateOverride: ResourceFormItemOverrides<
  typeof dictDataSchema,
  DictDataUpdateValues,
  "update",
  GeneratedDictData
> = {
  // @ts-expect-error dictType is CreateOnly and cannot be an update override.
  dictType: { label: "禁止" },
};

const invalidReadonlyOverride: ResourceFormItemOverrides<
  typeof dictDataSchema,
  GeneratedDictData,
  "update",
  GeneratedDictData
> = {
  // @ts-expect-error status is read-only and cannot be an update override.
  status: { label: "禁止" },
};

const invalidServerManagedOverride: ResourceFormItemOverrides<
  typeof dictDataSchema,
  DictDataUpdateValues,
  "update",
  GeneratedDictData
> = {
  // @ts-expect-error id is server-managed and cannot be an update override.
  id: { label: "禁止" },
};

void syntheticSortableField;
void syntheticListableField;
void numericSelectOverride;
void bigintFieldProps;
void invalidBigintFieldProps;
void invalidFieldOptionType;
void invalidProductionSubmit;
void invalidCreateMode;
void invalidSyntheticCreateOverride;
void invalidSyntheticUpdateOverride;
void invalidSyntheticCreateProps;
void invalidGeneratedUpdateValues;
void invalidColumn;
void invalidQuery;
void invalidOrder;
void invalidCapabilityColumn;
void invalidStandardPermission;
void invalidCustomAction;
void invalidPresence;
void invalidUpdateOverride;
void invalidServerManagedOverride;
void invalidReadonlyOverride;

describe("Resource UI contract compile samples", () => {
  it("keeps generated bigint totals exact until the ProTable boundary", () => {
    expect(toProTableTotal(3n)).toBe(3);
    expect(() => toProTableTotal(BigInt(Number.MAX_SAFE_INTEGER) + 1n)).toThrow(RangeError);
    expect(toProTableTotal(BigInt(Number.MAX_SAFE_INTEGER) + 1n, "saturate")).toBe(
      Number.MAX_SAFE_INTEGER,
    );
  });

  it("uses real generated schema/entity/request types", () => {
    expect(operations.keyOf(generatedEntity)).toBe(generatedEntity.id);
    expect(tableProps.schema.name).toBe("DictData");
    expect(formProps.mode).toBe("update");
  });
});
