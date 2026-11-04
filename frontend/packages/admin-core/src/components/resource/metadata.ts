import type { ReactNode } from "react";

import type {
  FieldRelation,
  ResourceSchema,
  ResourceSchemaField,
  ResourceSchemaFieldName,
} from "./schema";

export type ResourceValueType =
  "text" | "textarea" | "number" | "bigint" | "boolean" | "date" | "select" | "custom";

export type ResourceOverrideMerge = "append" | "replace" | "hide";

export interface ResourceAccessContext<TEntity = unknown> {
  readonly entity?: TEntity;
  readonly key?: string | number | bigint;
}

/** The sole permission decision entry point for Resource UI actions. */
export interface ResourceAccess<TEntity = unknown> {
  can(permission: string, context?: ResourceAccessContext<TEntity>): boolean;
}

export type ResourcePermissionAction =
  "list" | "get" | "create" | "update" | "delete" | "batchDelete";

/** Default v1 permission mapping. Generated schemas may provide a validated
 * legacy Get mapping through ResourceSchema.permissions. */
export type ResourcePermissionMap<
  TPermPrefix extends string = string,
  TGetPermission extends string = `${TPermPrefix}:list`,
> = {
  readonly list: `${TPermPrefix}:list`;
  readonly get: TGetPermission;
  readonly create: `${TPermPrefix}:add`;
  readonly update: `${TPermPrefix}:edit`;
  readonly delete: `${TPermPrefix}:remove`;
  readonly batchDelete: `${TPermPrefix}:remove`;
};

export function resourcePermissions<TPermPrefix extends string>(
  permPrefix: TPermPrefix,
): ResourcePermissionMap<TPermPrefix> {
  return {
    list: `${permPrefix}:list`,
    get: `${permPrefix}:list`,
    create: `${permPrefix}:add`,
    update: `${permPrefix}:edit`,
    delete: `${permPrefix}:remove`,
    batchDelete: `${permPrefix}:remove`,
  } as ResourcePermissionMap<TPermPrefix>;
}

export interface ResourceFieldContext<TEntity = unknown> {
  readonly schema: ResourceSchema;
  readonly fieldName: string;
  readonly field: ResourceSchema["fields"][number];
  readonly mode: "create" | "update";
  readonly entity?: TEntity;
}

export type ResourceVisibility<TEntity = unknown> =
  boolean | ((context: ResourceFieldContext<TEntity>) => boolean);

export type ResourceRender<TEntity, TValue> = (args: {
  readonly value: TValue;
  readonly entity: TEntity;
  readonly index?: number;
  /** ResourceTable reload after a typed cell action completes. */
  readonly reload: () => Promise<void>;
}) => ReactNode;

export interface ResourceColumnOverrideBase<TEntity, TValue> {
  readonly title?: ReactNode;
  readonly render?: ResourceRender<TEntity, TValue>;
  readonly valueType?: ResourceValueType | (string & {});
  readonly hidden?: ResourceVisibility<TEntity>;
  /** A column override may hide a generated column, never turn listability on. */
  readonly listable?: false;
}

type CapabilityOverride<TField, TCapability extends "searchable" | "sortable"> =
  TField extends Record<TCapability, true>
    ? { readonly [K in TCapability]?: boolean }
    : { readonly [K in TCapability]?: false };

/** Capability flags can only narrow generated schema capabilities. */
export type ResourceColumnOverride<TField, TEntity, TValue> = ResourceColumnOverrideBase<
  TEntity,
  TValue
> &
  CapabilityOverride<TField, "searchable"> &
  CapabilityOverride<TField, "sortable">;

export type SchemaFields<TSchema extends ResourceSchema> = ResourceSchemaField<TSchema>;
export type SchemaFieldName<TSchema extends ResourceSchema> = ResourceSchemaFieldName<TSchema>;

type FieldWithRelation<TSchema extends ResourceSchema> = Extract<
  SchemaFields<TSchema>,
  { readonly relation: FieldRelation }
>;

export type RelationFieldName<TSchema extends ResourceSchema> = FieldWithRelation<TSchema>["name"] &
  string;

export type FieldValue<TEntity, TFieldName extends PropertyKey> = TFieldName extends keyof TEntity
  ? TEntity[TFieldName]
  : unknown;

export type ResourceColumnOverrides<TSchema extends ResourceSchema, TEntity> = Partial<{
  [TFieldName in SchemaFieldName<TSchema>]: ResourceColumnOverride<
    Extract<SchemaFields<TSchema>, { readonly name: TFieldName }>,
    TEntity,
    FieldValue<TEntity, TFieldName>
  >;
}>;

export type ResourceStandardRowAction = "create" | "update" | "delete" | "batchDelete";
export type ResourceStandardToolbarAction = "create" | "batchDelete";

export interface ResourceRowActionContext<TEntity, TKey extends string | number | bigint> {
  readonly entity: TEntity;
  readonly key: TKey;
  readonly selectedKeys: readonly TKey[];
  readonly reload: () => Promise<void>;
}

export interface ResourceStandardRowActionOverride<TEntity, TKey extends string | number | bigint> {
  readonly action: ResourceStandardRowAction;
  /** Standard action replace/hide cannot carry a replacement permission. */
  readonly merge: "replace" | "hide";
  readonly title?: ReactNode;
  readonly onClick?: (context: ResourceRowActionContext<TEntity, TKey>) => void | Promise<void>;
}

type ResourceCustomPermission = { readonly permission: string };

export type ResourceCustomRowActionOverride<
  TEntity,
  TKey extends string | number | bigint,
> = ResourceCustomPermission & {
  readonly action: "custom";
  readonly name: string;
  readonly merge: "append";
  readonly title?: ReactNode;
  readonly hidden?: boolean | ((context: ResourceRowActionContext<TEntity, TKey>) => boolean);
  /**
   * Optional typed row renderer for controls such as Switch. ResourceTable
   * still owns permission, row context and standard reload/error boundaries;
   * the renderer only replaces the default button presentation.
   */
  readonly render?: (context: ResourceRowActionContext<TEntity, TKey>) => ReactNode;
  readonly onClick?: (context: ResourceRowActionContext<TEntity, TKey>) => void | Promise<void>;
};

export type ResourceRowActionOverride<TEntity, TKey extends string | number | bigint> =
  ResourceStandardRowActionOverride<TEntity, TKey> | ResourceCustomRowActionOverride<TEntity, TKey>;

export interface ResourceToolbarContext<TEntity, TKey extends string | number | bigint> {
  readonly selectedKeys: readonly TKey[];
  readonly rows: readonly TEntity[];
  readonly reload: () => Promise<void>;
  /** Clears the table-owned selection after a successful custom batch action. */
  readonly clearSelection: () => void;
}

export interface ResourceStandardToolbarOverride<TEntity, TKey extends string | number | bigint> {
  readonly action: ResourceStandardToolbarAction;
  readonly merge: "replace" | "hide";
  readonly title?: ReactNode;
  readonly render?: (context: ResourceToolbarContext<TEntity, TKey>) => ReactNode;
}

export type ResourceCustomToolbarOverride<
  TEntity,
  TKey extends string | number | bigint,
> = ResourceCustomPermission & {
  readonly action: "custom";
  readonly name: string;
  readonly merge: "append";
  readonly title?: ReactNode;
  readonly hidden?: boolean | ((context: ResourceToolbarContext<TEntity, TKey>) => boolean);
  readonly render?: (context: ResourceToolbarContext<TEntity, TKey>) => ReactNode;
};

export type ResourceToolbarOverride<TEntity, TKey extends string | number | bigint> =
  ResourceStandardToolbarOverride<TEntity, TKey> | ResourceCustomToolbarOverride<TEntity, TKey>;
