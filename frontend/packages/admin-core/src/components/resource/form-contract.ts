import type { ComponentType, ReactNode } from "react";

import type { RelationOptionLoader } from "./relation";
import type { ResourceKey, ResourcePresence, ResourceUpdateSubmission } from "./operations";
import type {
  FieldValue,
  RelationFieldName,
  ResourceFieldContext,
  SchemaFieldName,
} from "./metadata";
import type { CreatableFieldName, ResourceSchema, UpdatableFieldName } from "./schema";

export type ResourceFormMode = "create" | "update";
export type ResourceValues = Readonly<Record<string, unknown>>;

export type EditableFieldName<
  TSchema extends ResourceSchema,
  TMode extends ResourceFormMode,
> = TMode extends "create" ? CreatableFieldName<TSchema> : UpdatableFieldName<TSchema>;

/** Prototype-only submission; production create/update use mode-specific types below. */
export interface ResourceFormSubmission<
  TValues extends object = ResourceValues,
  TField extends keyof TValues = keyof TValues,
> extends ResourcePresence<TField> {
  readonly values: Readonly<TValues>;
}

export interface ResourceCreateSubmission<
  TValues extends object,
  TField extends keyof TValues = keyof TValues,
> extends ResourcePresence<TField> {
  readonly values: Readonly<TValues>;
}

export interface ResourceFormRule<TValue> {
  readonly required?: boolean;
  readonly maxLength?: number;
  readonly message?: ReactNode;
  readonly validator?: (value: TValue) => void | Promise<void>;
}

export interface ResourceFieldComponentProps<TValue, TValues extends object> {
  readonly value: TValue;
  readonly values: Readonly<TValues>;
  readonly disabled?: boolean;
  readonly onChange: (value: TValue) => void;
}

/** Typed boundary for the standard controls; custom controls use `component`. */
export interface ResourceFieldProps<TValue = unknown> {
  readonly placeholder?: string;
  readonly min?: number;
  readonly max?: number;
  readonly step?: number;
  readonly rows?: number;
  readonly allowClear?: boolean;
  readonly showSearch?: boolean;
  readonly options?: readonly {
    readonly value: TValue;
    readonly label: string;
  }[];
}

export interface ResourceFormItemOverride<TValue, TValues extends object, TEntity = unknown> {
  readonly label?: ReactNode;
  readonly component?: ComponentType<ResourceFieldComponentProps<TValue, TValues>>;
  readonly valueType?:
    "text" | "textarea" | "number" | "bigint" | "boolean" | "date" | "select" | "custom";
  readonly visible?: boolean | ((context: ResourceFieldContext<TEntity>) => boolean);
  readonly disabled?: boolean | ((context: ResourceFieldContext<TEntity>) => boolean);
  /** Generated required/maxLen rules are always retained; these rules append. */
  readonly rules?: readonly ResourceFormRule<TValue>[];
  readonly fieldProps?: ResourceFieldProps<TValue>;
}

export type ResourceFormItemOverrides<
  TSchema extends ResourceSchema,
  TValues extends object,
  TMode extends ResourceFormMode,
  TEntity = unknown,
> = Partial<{
  [TFieldName in EditableFieldName<TSchema, TMode>]: ResourceFormItemOverride<
    FieldValue<TValues, TFieldName>,
    TValues,
    TEntity
  >;
}> &
  Partial<Record<Exclude<SchemaFieldName<TSchema>, EditableFieldName<TSchema, TMode>>, never>>;

type RelationValue<TValues extends object, TFieldName extends PropertyKey> = Extract<
  FieldValue<TValues, TFieldName>,
  string | number | bigint
>;

export type ResourceRelationLoaders<
  TSchema extends ResourceSchema,
  TValues extends object,
> = Partial<{
  [TFieldName in RelationFieldName<TSchema>]: RelationOptionLoader<
    RelationValue<TValues, TFieldName>
  >;
}>;

export interface ResourceSubmitState {
  readonly loading?: boolean;
  readonly error?: Error;
}

export type ResourceFormCreate<
  TCreateValues extends object,
  TResult,
  TField extends keyof TCreateValues,
> = (submission: ResourceCreateSubmission<TCreateValues, TField>) => Promise<TResult> | TResult;

export type ResourceFormUpdate<
  TKey extends ResourceKey,
  TUpdateValues extends object,
  TUpdateField extends keyof TUpdateValues = keyof TUpdateValues,
  TResult = unknown,
> = (
  submission: ResourceUpdateSubmission<TKey, TUpdateValues, TUpdateField>,
) => Promise<TResult> | TResult;

export interface ResourceFormCreateOperations<TCreateValues extends object, TResult = unknown> {
  readonly create: (values: Readonly<TCreateValues>) => Promise<TResult>;
}

export interface ResourceFormUpdateOperations<
  TKey extends ResourceKey,
  TUpdateValues extends object,
  TUpdateField extends keyof TUpdateValues = keyof TUpdateValues,
  TResult = unknown,
> {
  readonly update: (
    submission: ResourceUpdateSubmission<TKey, TUpdateValues, TUpdateField>,
  ) => Promise<TResult>;
}

type LifecycleValues<TValues extends object, TAllowedField extends string> =
  Exclude<Extract<keyof TValues, string>, TAllowedField | "$typeName" | "$unknown"> extends never
    ? TValues
    : never;

type CreateValues<TSchema extends ResourceSchema, TValues extends object> = LifecycleValues<
  TValues,
  CreatableFieldName<TSchema>
>;

type UpdateValues<TSchema extends ResourceSchema, TValues extends object> = LifecycleValues<
  TValues,
  UpdatableFieldName<TSchema>
>;

/** Lifecycle-filtered values used by the production table/form bridge. */
export type ResourceCreateFormValues<
  TSchema extends ResourceSchema,
  TValues extends object,
> = CreateValues<TSchema, TValues>;

export type ResourceUpdateFormValues<
  TSchema extends ResourceSchema,
  TValues extends object,
> = UpdateValues<TSchema, TValues>;

interface ResourceFormCommonProps<
  TSchema extends ResourceSchema,
  TMode extends ResourceFormMode,
  TValues extends object,
  TEntity extends object,
> extends ResourceSubmitState {
  readonly schema: TSchema;
  readonly resetKey?: ResourceKey;
  readonly relationLoaders?: ResourceRelationLoaders<TSchema, TValues>;
  readonly itemOverrides?: ResourceFormItemOverrides<TSchema, TValues, TMode, TEntity>;
  /**
   * Typed escape hatch for a CUSTOM workflow's non-Resource controls (for
   * example a password or repeated relationship field). Generated schema
   * fields still own the ordinary CRUD values and presence bookkeeping.
   */
  readonly extraFields?: ReactNode;
}

type CreateTransport<
  TCreateValues extends object,
  TCreateField extends keyof TCreateValues,
  TResult,
> =
  | {
      readonly onCreate: ResourceFormCreate<TCreateValues, TResult, TCreateField>;
      readonly operations?: ResourceFormCreateOperations<TCreateValues, TResult>;
    }
  | {
      readonly onCreate?: never;
      readonly operations: ResourceFormCreateOperations<TCreateValues, TResult>;
    };

type UpdateTransport<
  TKey extends ResourceKey,
  TUpdateValues extends object,
  TUpdateField extends keyof TUpdateValues,
  TResult,
> =
  | {
      readonly onUpdate: ResourceFormUpdate<TKey, TUpdateValues, TUpdateField, TResult>;
      readonly operations?: ResourceFormUpdateOperations<
        TKey,
        TUpdateValues,
        TUpdateField,
        TResult
      >;
    }
  | {
      readonly onUpdate?: never;
      readonly operations: ResourceFormUpdateOperations<TKey, TUpdateValues, TUpdateField, TResult>;
    };

export type ResourceFormCreateProps<
  TSchema extends ResourceSchema,
  TCreateValues extends object,
  TCreateField extends keyof TCreateValues = keyof TCreateValues,
  TResult = unknown,
  TEntity extends object = TCreateValues,
> = ResourceFormCommonProps<TSchema, "create", CreateValues<TSchema, TCreateValues>, TEntity> &
  CreateTransport<CreateValues<TSchema, TCreateValues>, TCreateField, TResult> & {
    readonly mode: "create";
    readonly initialValues?: Readonly<Partial<CreateValues<TSchema, TCreateValues>>>;
    readonly key?: never;
  };

export type ResourceFormUpdateProps<
  TSchema extends ResourceSchema,
  TKey extends ResourceKey,
  TUpdateValues extends object,
  TUpdateField extends keyof TUpdateValues = keyof TUpdateValues,
  TResult = unknown,
  TEntity extends object = TUpdateValues,
> = ResourceFormCommonProps<TSchema, "update", UpdateValues<TSchema, TUpdateValues>, TEntity> &
  UpdateTransport<TKey, UpdateValues<TSchema, TUpdateValues>, TUpdateField, TResult> & {
    readonly mode: "update";
    /** Explicit form identity; named rowKey to avoid React's reserved `key` prop. */
    readonly rowKey: TKey;
    readonly initialValues?: Readonly<Partial<UpdateValues<TSchema, TUpdateValues>>>;
  };

/** Compatibility-only props for the existing native form prototype. */
export interface ResourceFormPrototypeProps extends ResourceSubmitState {
  readonly schema: ResourceSchema;
  readonly mode: ResourceFormMode;
  readonly initialValues?: ResourceValues;
  readonly resetKey?: ResourceKey;
  readonly relationLoaders?: Readonly<
    Partial<Record<string, RelationOptionLoader<string | number | bigint>>>
  >;
  readonly onSubmit: (submission: ResourceFormSubmission) => void | Promise<void>;
}

/** Production contract only; the prototype props are deliberately excluded. */
export type ResourceFormProps<
  TSchema extends ResourceSchema = ResourceSchema,
  TValues extends object = ResourceValues,
  TMode extends ResourceFormMode = ResourceFormMode,
  TKey extends ResourceKey = ResourceKey,
  TCreateValues extends object = Partial<TValues>,
  TUpdateValues extends object = Partial<TValues>,
  TUpdateField extends keyof TUpdateValues = keyof TUpdateValues,
  TEntity extends object = TValues,
> = TMode extends "create"
  ? ResourceFormCreateProps<TSchema, TCreateValues, keyof TCreateValues, unknown, TEntity>
  : TMode extends "update"
    ? ResourceFormUpdateProps<TSchema, TKey, TUpdateValues, TUpdateField, unknown, TEntity>
    : | ResourceFormCreateProps<TSchema, TCreateValues, keyof TCreateValues, unknown, TEntity>
      | ResourceFormUpdateProps<TSchema, TKey, TUpdateValues, TUpdateField, unknown, TEntity>;

export type ResourceFieldName<TSchema extends ResourceSchema> = SchemaFieldName<TSchema>;
