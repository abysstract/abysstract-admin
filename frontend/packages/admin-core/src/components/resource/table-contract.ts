import type { ReactNode } from "react";

import type {
  ResourceAccess,
  ResourceColumnOverrides,
  ResourceCustomRowActionOverride,
  ResourceCustomToolbarOverride,
  ResourceRowActionOverride,
  ResourceToolbarOverride,
} from "./metadata";
import type {
  ResourceKey,
  ResourceListQuery,
  ResourceOperations,
  ResourceOperationError,
  ResourceReadOperations,
} from "./operations";
import type { ResourcePermissionMap } from "./metadata";
import type {
  ResourceFormCreateOperations,
  ResourceFormItemOverrides,
  ResourceCreateFormValues,
  ResourceUpdateFormValues,
  ResourceRelationLoaders,
  ResourceFormUpdateOperations,
} from "./form-contract";
import type { ResourceSchema, SortableFieldName } from "./schema";

/**
 * The production default is operations.keyOf, generated from the Resource key.
 * The callback form remains available for deliberate custom business keys.
 */
type KeyField<TEntity, TKey extends ResourceKey> = {
  [TFieldName in keyof TEntity]-?: NonNullable<TEntity[TFieldName]> extends TKey
    ? TFieldName
    : never;
}[keyof TEntity];

export type ResourceRowKey<TEntity, TKey extends ResourceKey> =
  KeyField<TEntity, TKey> | ((row: Readonly<TEntity>, index: number) => TKey);

export interface ResourceQueryConfig<TSchema extends ResourceSchema, TFilters extends object> {
  readonly defaults?: Partial<ResourceListQuery<TFilters, SortableFieldName<TSchema>>>;
  readonly initial?: Partial<ResourceListQuery<TFilters, SortableFieldName<TSchema>>>;
  readonly onChange?: (query: ResourceListQuery<TFilters, SortableFieldName<TSchema>>) => void;
}

/**
 * Optional tree projection for resources whose standard List remains flat.
 * ResourceTable fetches all pages with the same typed query, then delegates
 * only the presentation projection and selection flattening to the page.
 */
export interface ResourceTableTreeConfig<TEntity> {
  readonly pageSize?: number;
  readonly build: (rows: readonly TEntity[]) => readonly TEntity[];
  readonly flatten?: (rows: readonly TEntity[]) => readonly TEntity[];
}

export interface ResourceTableReload {
  readonly reload: () => Promise<void>;
}

export interface ResourceTableErrorBoundary {
  /** Optional page-level callback for legacy integrations; ResourceTable always reloads itself. */
  readonly reload?: () => Promise<void>;
  readonly onError?: (error: ResourceOperationError) => void;
  readonly renderError?: (error: ResourceOperationError, retry: () => Promise<void>) => ReactNode;
}

export interface ResourceTableCreateFormConfig<
  TSchema extends ResourceSchema,
  TEntity,
  TCreateValues extends object,
> {
  readonly title?: ReactNode;
  readonly initialValues?: Readonly<Partial<ResourceCreateFormValues<TSchema, TCreateValues>>>;
  readonly relationLoaders?: ResourceRelationLoaders<
    TSchema,
    ResourceCreateFormValues<TSchema, TCreateValues>
  >;
  readonly itemOverrides?: ResourceFormItemOverrides<
    TSchema,
    ResourceCreateFormValues<TSchema, TCreateValues>,
    "create",
    TEntity
  >;
  readonly onSuccess?: (entity: TEntity) => void | Promise<void>;
}

export interface ResourceTableEditFormConfig<
  TSchema extends ResourceSchema,
  TEntity,
  TUpdateValues extends object,
> {
  readonly title?: ReactNode;
  readonly initialValues?: (
    entity: Readonly<TEntity>,
  ) => Readonly<Partial<ResourceUpdateFormValues<TSchema, TUpdateValues>>>;
  readonly relationLoaders?: ResourceRelationLoaders<
    TSchema,
    ResourceUpdateFormValues<TSchema, TUpdateValues>
  >;
  readonly itemOverrides?: ResourceFormItemOverrides<
    TSchema,
    ResourceUpdateFormValues<TSchema, TUpdateValues>,
    "update",
    TEntity
  >;
  readonly onSuccess?: (entity: TEntity) => void | Promise<void>;
}

export interface ResourceTableFormIntegration<
  TSchema extends ResourceSchema,
  TEntity,
  TKey extends ResourceKey,
  TCreateValues extends object,
  TUpdateValues extends object,
> {
  /** Preferred production path. ResourceTable owns trigger, modal, loading, error and reload. */
  readonly createConfig?: ResourceTableCreateFormConfig<TSchema, TEntity, TCreateValues>;
  readonly editConfig?: ResourceTableEditFormConfig<TSchema, TEntity, TUpdateValues>;
  /** Compatibility escape hatches for non-standard layouts; not needed for standard CRUD. */
  /**
   * Custom render callbacks receive the generated schema and typed transport to
   * compose ResourceForm with the existing operations.
   */
  readonly create?: (context: {
    readonly schema: TSchema;
    readonly operations: ResourceFormCreateOperations<TCreateValues, TEntity>;
    readonly reload: () => Promise<void>;
  }) => ReactNode;
  readonly edit?: (context: {
    readonly schema: TSchema;
    readonly entity: TEntity;
    readonly key: TKey;
    readonly operations: ResourceFormUpdateOperations<
      TKey,
      TUpdateValues,
      keyof TUpdateValues,
      TEntity
    >;
    readonly reload: () => Promise<void>;
  }) => ReactNode;
  readonly createValues?: Readonly<TCreateValues>;
  readonly updateValues?: Readonly<TUpdateValues>;
}

export interface ResourceTableProductionProps<
  TSchema extends ResourceSchema,
  TEntity,
  TKey extends ResourceKey,
  TFilters extends object,
  TCreateValues extends object,
  TUpdateValues extends object,
> extends ResourceTableErrorBoundary {
  /** Discriminator kept optional for backwards-compatible full CRUD callers. */
  readonly readOnly?: false;
  readonly schema: TSchema;
  readonly operations: ResourceOperations<
    TEntity,
    TKey,
    TFilters,
    TCreateValues,
    TUpdateValues,
    TEntity,
    void,
    SortableFieldName<TSchema>
  >;
  /** Defaults to operations.keyOf; pages only provide this for a deliberate custom key. */
  readonly rowKey?: ResourceRowKey<TEntity, TKey>;
  readonly access: ResourceAccess<TEntity>;
  /** Omit to derive the fixed list/add/edit/remove map from schema.permPrefix. */
  readonly permissionMap?: ResourcePermissionMap<
    TSchema["permPrefix"],
    `${TSchema["permPrefix"]}:list` | `${TSchema["permPrefix"]}:query`
  >;
  readonly columns?: ResourceColumnOverrides<TSchema, TEntity>;
  readonly rowActions?: readonly ResourceRowActionOverride<TEntity, TKey>[];
  readonly toolbar?: readonly ResourceToolbarOverride<TEntity, TKey>[];
  readonly query?: ResourceQueryConfig<TSchema, TFilters>;
  /** Optional client-side tree projection over the standard flat List result. */
  readonly treeData?: ResourceTableTreeConfig<TEntity>;
  readonly form?: ResourceTableFormIntegration<
    TSchema,
    TEntity,
    TKey,
    TCreateValues,
    TUpdateValues
  >;
  readonly emptyText?: ReactNode;
}

/**
 * Production contract for a Partial Resource whose standard surface is
 * read-only. CUSTOM workflows are still allowed, but the table cannot expose
 * a fake standard CRUD operation or silently call an absent mutation method.
 */
export interface ResourceTableReadOnlyProps<
  TSchema extends ResourceSchema,
  TEntity,
  TKey extends ResourceKey,
  TFilters extends object,
> extends ResourceTableErrorBoundary {
  readonly readOnly: true;
  readonly schema: TSchema;
  readonly operations: ResourceReadOperations<TEntity, TKey, TFilters, SortableFieldName<TSchema>>;
  /** Defaults to operations.keyOf; pages only provide this for a deliberate custom key. */
  readonly rowKey?: ResourceRowKey<TEntity, TKey>;
  readonly access: ResourceAccess<TEntity>;
  /** Omit to derive the fixed list/get map from schema.permPrefix. */
  readonly permissionMap?: ResourcePermissionMap<
    TSchema["permPrefix"],
    `${TSchema["permPrefix"]}:list` | `${TSchema["permPrefix"]}:query`
  >;
  readonly columns?: ResourceColumnOverrides<TSchema, TEntity>;
  readonly rowActions?: readonly ResourceCustomRowActionOverride<TEntity, TKey>[];
  readonly toolbar?: readonly ResourceCustomToolbarOverride<TEntity, TKey>[];
  readonly query?: ResourceQueryConfig<TSchema, TFilters>;
  /** Optional client-side tree projection over the standard flat List result. */
  readonly treeData?: ResourceTableTreeConfig<TEntity>;
  readonly emptyText?: ReactNode;
  /** Standard form integration is intentionally unavailable in read-only mode. */
  readonly form?: never;
}

/**
 * The rows overload supports lightweight prototypes. Generated pages use
 * ResourceTableProductionProps for their data and operation bindings.
 */
export interface ResourceTablePrototypeProps {
  readonly schema: ResourceSchema;
  readonly rows: readonly Readonly<Record<string, unknown>>[];
  readonly rowKey: (row: Readonly<Record<string, unknown>>, index: number) => ResourceKey;
}

export type ResourceTableProps<
  TSchema extends ResourceSchema = ResourceSchema,
  TEntity = Readonly<Record<string, unknown>>,
  TKey extends ResourceKey = ResourceKey,
  TFilters extends object = Record<never, never>,
  TCreateValues extends object = Partial<TEntity>,
  TUpdateValues extends object = Partial<TEntity>,
> =
  | ResourceTableProductionProps<TSchema, TEntity, TKey, TFilters, TCreateValues, TUpdateValues>
  | ResourceTableReadOnlyProps<TSchema, TEntity, TKey, TFilters>
  | ResourceTablePrototypeProps;
