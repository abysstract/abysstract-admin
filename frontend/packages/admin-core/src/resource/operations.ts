/**
 * UI-facing Resource operations. Generated raw RPC clients stay behind this
 * boundary; pages and resource components only consume these semantic CRUD
 * methods.
 */

export type ResourceKey = string | number | bigint;

export interface ResourceOrder<TField extends string = never> {
  readonly field: TField;
  readonly desc: boolean;
}

/** The standard query shape owned by the Resource UI, not by an RPC method. */
export interface ResourceListQuery<
  TFilters extends object = Record<never, never>,
  TOrderField extends string = never,
> {
  readonly pageNum: number;
  readonly pageSize: number;
  readonly keyword?: string;
  readonly orders?: readonly ResourceOrder<TOrderField>[];
  readonly filters?: TFilters;
}

export type ResourceListRows<TEntity> = readonly TEntity[];

export interface ResourcePageInfo {
  /** Facades preserve protobuf int64 exactly until this explicit UI boundary. */
  readonly total: bigint;
  readonly pageNum: number;
  readonly pageSize: number;
}

/**
 * A facade may use either `rows` or `items` as its canonical collection. The
 * union prevents adapters from returning neither while allowing generated
 * response naming to remain an implementation detail.
 */
export type ResourceListResult<TEntity> =
  | (ResourcePageInfo & {
      readonly rows: readonly TEntity[];
      readonly items?: readonly TEntity[];
    })
  | (ResourcePageInfo & {
      readonly rows?: readonly TEntity[];
      readonly items: readonly TEntity[];
    });

/**
 * The smallest operations contract for a Partial Resource page. A partial
 * resource may deliberately keep create/update/delete as typed CUSTOM
 * workflows, while its standard List still benefits from the ResourceTable
 * query, pagination, bigint-key, and error boundaries.
 */
export interface ResourceReadOperations<
  TEntity,
  TKey extends ResourceKey,
  TFilters extends object = Record<never, never>,
  TOrderField extends string = never,
> {
  /** Canonical key source used by ResourceTable when rowKey is omitted. */
  readonly keyOf: (entity: Readonly<TEntity>) => TKey;
  readonly list: (
    query: ResourceListQuery<TFilters, TOrderField>,
  ) => Promise<ResourceListResult<TEntity>>;
}

export interface ResourcePresence<TField extends PropertyKey> {
  /** Fields the user explicitly supplied, including explicit zero/false/"". */
  readonly touched: ReadonlySet<TField>;
  /** Fields whose current value differs from the initial value. */
  readonly dirty: ReadonlySet<TField>;
}

/**
 * Update input deliberately carries presence separately from values. A
 * generated adapter is responsible for writing the identifier and exactly
 * touched fields; callers must never calculate `changedFields` themselves.
 */
export interface ResourceUpdateSubmission<
  TKey extends ResourceKey,
  TValues extends object,
  TField extends keyof TValues = keyof TValues,
> extends ResourcePresence<TField> {
  readonly key: TKey;
  readonly values: Readonly<TValues>;
}

export interface ResourceOperations<
  TEntity,
  TKey extends ResourceKey,
  TFilters extends object = Record<never, never>,
  TCreateValues extends object = Partial<TEntity>,
  TUpdateValues extends object = Partial<TEntity>,
  TMutationResult = TEntity,
  TDeleteResult = void,
  TOrderField extends string = never,
> {
  /** Canonical key source used by ResourceTable when rowKey is omitted. */
  readonly keyOf: (entity: Readonly<TEntity>) => TKey;
  readonly list: (
    query: ResourceListQuery<TFilters, TOrderField>,
  ) => Promise<ResourceListResult<TEntity>>;
  readonly get: (key: TKey) => Promise<TEntity>;
  readonly create: (values: Readonly<TCreateValues>) => Promise<TMutationResult>;
  readonly update: (
    submission: ResourceUpdateSubmission<TKey, TUpdateValues>,
  ) => Promise<TMutationResult>;
  readonly delete: (key: TKey) => Promise<TDeleteResult>;
  readonly batchDelete: (keys: readonly TKey[]) => Promise<TDeleteResult>;
}

export type ResourceOperationError = Error & {
  readonly code?: string;
  readonly details?: unknown;
};

export type ProTableTotalOverflowPolicy = "error" | "saturate";

/**
 * The only bigint-to-ProTable conversion boundary. The default is fail-closed:
 * an unsafe total is an error rather than a silently rounded pagination count.
 */
export function toProTableTotal(
  total: bigint,
  policy: ProTableTotalOverflowPolicy = "error",
): number {
  const max = BigInt(Number.MAX_SAFE_INTEGER);
  if (total < 0n) {
    throw new RangeError("Resource total cannot be negative");
  }
  if (total > max) {
    if (policy === "saturate") {
      return Number.MAX_SAFE_INTEGER;
    }
    throw new RangeError("Resource total exceeds Number.MAX_SAFE_INTEGER");
  }
  return Number(total);
}

/** Normalizes either allowed list collection spelling for component code. */
export function resourceListRows<TEntity>(result: ResourceListResult<TEntity>): readonly TEntity[] {
  if (result.rows !== undefined) {
    return result.rows;
  }
  return result.items ?? [];
}
