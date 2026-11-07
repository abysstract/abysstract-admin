// Resource Schema 的前端基础类型（手写维护）。
// 生成物通过 `satisfies ResourceSchema` 引用本文件，TS 编译期校验生成结构。

/** 生成器格式 1 的配套类型，生成代码引用后由类型检查确认运行库存在。 */
export type GeneratorFormatV1 = 1;

/** 字段的 TS 投影类型 */
export type FieldTsType = "bigint" | "number" | "string" | "boolean" | "unknown";

/** v1 relation 只允许带 discriminator 的 Resource relation。 */
export interface ResourceFieldRelation {
  readonly kind: "resource";
  readonly target: string;
  readonly valueField: string;
  readonly labelField: string;
}

export type FieldRelation = ResourceFieldRelation;

/** 单个字段的资源行为投影 */
export interface FieldSchema {
  name: string;
  /** Shared list/create/update label; page overrides take precedence. */
  label?: string;
  protoName: string;
  tsType: FieldTsType;
  required: boolean;
  searchable: boolean;
  filterable: boolean;
  sortable: boolean;
  listable: boolean;
  maxLen: number;
  creatable: boolean;
  updatable: boolean;
  serverManaged: boolean;
  /**
   * Nullable field whose create request member is a Proto `optional`. The
   * standard create form leaves it empty and omits it from the request.
   */
  optional?: boolean;
  relation?: FieldRelation;
}

/** Generated procedure permissions, including validated legacy exceptions. */
export interface ResourceSchemaPermissions {
  readonly list?: string;
  readonly get?: string;
  readonly create?: string;
  readonly update?: string;
  readonly delete?: string;
  readonly batchDelete?: string;
}

/** 资源 schema 投影（生成物必须满足此结构，否则 TS 编译失败） */
export interface ResourceSchema {
  name: string;
  permPrefix: string;
  table: string;
  fields: readonly FieldSchema[];
  permissions?: ResourceSchemaPermissions;
}

export type ResourceSchemaField<TSchema extends ResourceSchema> = TSchema["fields"][number];
export type ResourceSchemaFieldName<TSchema extends ResourceSchema> =
  ResourceSchemaField<TSchema>["name"] & string;

type FieldNamesWithFlag<TSchema extends ResourceSchema, TFlag extends keyof FieldSchema> = Extract<
  ResourceSchemaField<TSchema>,
  Record<TFlag, true>
>["name"] &
  string;

type EditableFieldNamesWithFlag<
  TSchema extends ResourceSchema,
  TFlag extends "creatable" | "updatable",
> = Extract<
  ResourceSchemaField<TSchema>,
  Record<TFlag, true> & { readonly serverManaged: false }
>["name"] &
  string;

export type SearchableFieldName<TSchema extends ResourceSchema> = FieldNamesWithFlag<
  TSchema,
  "searchable"
>;
export type FilterableFieldName<TSchema extends ResourceSchema> = FieldNamesWithFlag<
  TSchema,
  "filterable"
>;
export type SortableFieldName<TSchema extends ResourceSchema> = FieldNamesWithFlag<
  TSchema,
  "sortable"
>;
export type ListableFieldName<TSchema extends ResourceSchema> = FieldNamesWithFlag<
  TSchema,
  "listable"
>;
export type CreatableFieldName<TSchema extends ResourceSchema> = EditableFieldNamesWithFlag<
  TSchema,
  "creatable"
>;
export type UpdatableFieldName<TSchema extends ResourceSchema> = EditableFieldNamesWithFlag<
  TSchema,
  "updatable"
>;
