import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type ComponentType,
  type FormEvent,
  type ReactNode,
} from "react";
import { Button, Form, Input, InputNumber, Select, Space, Switch } from "antd";
import { ProForm } from "@ant-design/pro-components";

import type { RelationOptionLoader } from "./relation";
import type { ResourceKey } from "./operations";
import type { FieldSchema, ResourceSchema } from "./schema";
import type { ResourceFieldContext } from "./metadata";
import { formatErrorMessage } from "../../runtime/message";
import type {
  ResourceFormItemOverride,
  ResourceFormPrototypeProps,
  ResourceFormMode,
  ResourceFormProps,
  ResourceValues,
} from "./form-contract";
import "./ResourceForm.css";

export type {
  ResourceFormPrototypeProps,
  ResourceFormMode,
  ResourceFormProps,
  ResourceFormSubmission,
  ResourceValues,
} from "./form-contract";

const emptyInitialValues: ResourceValues = {};
type RelationValue = string | number | bigint;
type AnyRelationOptionLoader = RelationOptionLoader<RelationValue>;
type RelationOption = { readonly value: RelationValue; readonly label: string };
type FormItemOverride = ResourceFormItemOverride<unknown, Record<string, unknown>, unknown>;
type RuntimeSubmission = {
  readonly values: Readonly<Record<string, unknown>>;
  readonly touched: ReadonlySet<string>;
  readonly dirty: ReadonlySet<string>;
};
type RuntimeCreateProps = {
  readonly mode: "create";
  readonly onCreate?: (submission: RuntimeSubmission) => Promise<unknown> | unknown;
  readonly operations?: {
    readonly create: (values: Readonly<Record<string, unknown>>) => Promise<unknown>;
  };
};
type RuntimeUpdateProps = {
  readonly mode: "update";
  readonly rowKey: ResourceKey;
  readonly onUpdate?: (
    submission: RuntimeSubmission & { readonly key: ResourceKey },
  ) => Promise<unknown> | unknown;
  readonly operations?: {
    readonly update: (
      submission: RuntimeSubmission & { readonly key: ResourceKey },
    ) => Promise<unknown>;
  };
};
type ProductionProps = ResourceFormProps;

function editable(field: FieldSchema, mode: ResourceFormMode): boolean {
  if (field.serverManaged) return false;
  return mode === "create" ? field.creatable : field.updatable;
}

function sameInitialValues(left: ResourceValues, right: ResourceValues): boolean {
  const leftKeys = Object.keys(left);
  if (leftKeys.length !== Object.keys(right).length) return false;
  return leftKeys.every(
    (key) => Object.prototype.hasOwnProperty.call(right, key) && Object.is(left[key], right[key]),
  );
}

function inputValue(field: FieldSchema, value: unknown): string | boolean {
  if (field.tsType === "boolean") return Boolean(value);
  return value === undefined || value === null ? "" : String(value);
}

/** Value of a cleared input: nullable fields return to "not set". */
function emptyValue(field: FieldSchema): unknown {
  return field.optional === true ? undefined : "";
}

function parseInput(field: FieldSchema, value: string | boolean, fallback?: unknown): unknown {
  switch (field.tsType) {
    case "boolean":
      return value;
    case "number":
      return value === "" ? emptyValue(field) : Number(value);
    case "bigint":
      if (value === "") return emptyValue(field);
      try {
        return BigInt(value);
      } catch {
        // BigInt text inputs have transient states (for example `-` or `1e`).
        // Keep the last valid value instead of letting a native BigInt error
        // escape from the controlled input's onChange handler.
        return fallback;
      }
    default:
      return value;
  }
}

function isBlank(value: unknown): boolean {
  return (
    value === undefined || value === null || (typeof value === "string" && value.trim() === "")
  );
}

function unicodeLength(value: string): number {
  return Array.from(value).length;
}

function fieldContext(
  schema: ResourceSchema,
  field: FieldSchema,
  mode: ResourceFormMode,
  entity: unknown,
): ResourceFieldContext {
  return { schema, fieldName: field.name, field, mode, entity };
}

function RelationOptions({
  fieldName,
  loader,
}: {
  fieldName: string;
  loader: AnyRelationOptionLoader;
}) {
  const [options, setOptions] = useState<ReadonlyArray<RelationOption>>([]);
  const [nextPage, setNextPage] = useState<number | undefined>();
  const [keyword, setKeyword] = useState<string | undefined>();
  const [error, setError] = useState<Error | undefined>();
  const latestLoad = useRef(0);

  const load = async (pageNum: number, nextKeyword?: string, append = false) => {
    const loadID = ++latestLoad.current;
    setError(undefined);
    try {
      const result = await loader.load({
        pageNum,
        pageSize: 20,
        ...(nextKeyword === undefined ? {} : { keyword: nextKeyword }),
      });
      if (loadID !== latestLoad.current) return;
      setOptions((previous) => (append ? [...previous, ...result.options] : result.options));
      setNextPage(result.nextPage);
    } catch (caught) {
      if (loadID === latestLoad.current) {
        setError(caught instanceof Error ? caught : new Error(String(caught)));
      }
    }
  };

  useEffect(() => {
    void load(1);
    return () => {
      latestLoad.current += 1;
    };
    // The loader identity is the lifecycle trigger; load is intentionally
    // recreated per render and must not be an effect dependency.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loader]);

  return (
    <Space direction="vertical" size="small" style={{ width: "100%" }}>
      {loader.remoteSearchEnabled && (
        <Input
          aria-label={`${fieldName} search`}
          type="search"
          value={keyword}
          onChange={(event) => {
            const nextKeyword = event.currentTarget.value;
            setKeyword(nextKeyword);
            void load(1, nextKeyword);
          }}
        />
      )}
      {error !== undefined && (
        <div className="resource-form__error" role="alert">
          {formatErrorMessage(error)}
        </div>
      )}
      <ul>
        {options.map((option) => (
          <li key={String(option.value)}>{option.label}</li>
        ))}
      </ul>
      {nextPage !== undefined && (
        <Button
          type="link"
          size="small"
          autoInsertSpace={false}
          onClick={() => void load(nextPage, keyword, true)}
        >
          加载更多
        </Button>
      )}
    </Space>
  );
}

function RelationSelect({
  fieldName,
  fieldProps,
  loader,
  value,
  disabled,
  onChange,
}: {
  fieldName: string;
  fieldProps?: { readonly options?: readonly RelationOption[]; readonly showSearch?: boolean };
  loader?: AnyRelationOptionLoader;
  value: unknown;
  disabled: boolean;
  onChange: (value: unknown) => void;
}) {
  const [options, setOptions] = useState<ReadonlyArray<RelationOption>>(() =>
    (fieldProps?.options ?? []).flatMap((option) =>
      typeof option.value === "string" ||
      typeof option.value === "number" ||
      typeof option.value === "bigint"
        ? [{ value: option.value, label: option.label }]
        : [],
    ),
  );
  const [nextPage, setNextPage] = useState<number | undefined>();
  const [keyword, setKeyword] = useState<string | undefined>();
  const [error, setError] = useState<Error | undefined>();
  const latestLoad = useRef(0);
  const loading = useRef(false);

  const load = async (pageNum: number, nextKeyword?: string, append = false) => {
    if (loader === undefined || (append && loading.current)) return;
    const loadID = ++latestLoad.current;
    loading.current = true;
    setError(undefined);
    try {
      const result = await loader.load({
        pageNum,
        pageSize: 20,
        ...(nextKeyword === undefined ? {} : { keyword: nextKeyword }),
      });
      if (loadID !== latestLoad.current) return;
      setOptions((previous) => (append ? [...previous, ...result.options] : result.options));
      setNextPage(result.nextPage);
    } catch (caught) {
      if (loadID === latestLoad.current) {
        setError(caught instanceof Error ? caught : new Error(String(caught)));
      }
    } finally {
      if (loadID === latestLoad.current) loading.current = false;
    }
  };

  useEffect(() => {
    if (loader !== undefined) void load(1);
    return () => {
      latestLoad.current += 1;
    };
    // The loader identity is the lifecycle trigger; load is intentionally
    // recreated per render and must not be an effect dependency.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loader]);

  const searchEnabled = loader?.remoteSearchEnabled === true;
  return (
    <Space direction="vertical" size="small" style={{ width: "100%" }}>
      <Select
        aria-label={fieldName}
        value={value as RelationValue | undefined}
        disabled={disabled}
        options={[...options]}
        onChange={(nextValue) => onChange(nextValue)}
      />
      {searchEnabled && (
        <Input
          aria-label={`${fieldName} search`}
          type="search"
          value={keyword}
          onChange={(event) => {
            const nextKeyword = event.currentTarget.value;
            setKeyword(nextKeyword);
            void load(1, nextKeyword);
          }}
        />
      )}
      {error !== undefined && <div role="alert">{formatErrorMessage(error)}</div>}
      {nextPage !== undefined && (
        <Button
          type="link"
          size="small"
          autoInsertSpace={false}
          onClick={() => void load(nextPage, keyword, true)}
        >
          加载更多
        </Button>
      )}
    </Space>
  );
}

function LegacyResourceForm({ props }: { props: ResourceFormPrototypeProps }): JSX.Element {
  const schema = props.schema;
  const mode = props.mode;
  const initialValues = (props.initialValues ?? emptyInitialValues) as ResourceValues;
  const relationLoaders = (props.relationLoaders ?? {}) as Readonly<
    Record<string, AnyRelationOptionLoader>
  >;
  const fields = useMemo(
    () => schema.fields.filter((field) => editable(field, mode)),
    [mode, schema.fields],
  );
  const [values, setValues] = useState<Record<string, unknown>>(() => ({ ...initialValues }));
  const [touched, setTouched] = useState<ReadonlySet<string>>(() => new Set());
  const [dirty, setDirty] = useState<ReadonlySet<string>>(() => new Set());
  const baselineValues = useRef<ResourceValues>(initialValues);
  const priorInitialValues = useRef(initialValues);
  const priorResetKey = useRef(props.resetKey);

  useEffect(() => {
    const resetChanged = !Object.is(priorResetKey.current, props.resetKey);
    const valuesChanged = !sameInitialValues(priorInitialValues.current, initialValues);
    const hasUnsavedChanges = dirty.size > 0;
    if (resetChanged || (valuesChanged && !hasUnsavedChanges)) {
      baselineValues.current = initialValues;
      setValues({ ...initialValues });
      setTouched(new Set());
      setDirty(new Set());
    }
    priorInitialValues.current = initialValues;
    priorResetKey.current = props.resetKey;
  }, [dirty, initialValues, props.resetKey]);

  const change = (field: FieldSchema, next: unknown) => {
    setValues((previous) => ({ ...previous, [field.name]: next }));
    setTouched((previous) => new Set(previous).add(field.name));
    setDirty((previous) => {
      const updated = new Set(previous);
      if (Object.is(next, baselineValues.current[field.name])) updated.delete(field.name);
      else updated.add(field.name);
      return updated;
    });
  };

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    props.onSubmit({ values, touched, dirty });
  };

  return (
    <form onSubmit={submit}>
      {fields.map((field) => {
        const value = inputValue(field, values[field.name]);
        if (field.tsType === "boolean") {
          return (
            <label key={field.name}>
              {field.name}
              <input
                aria-label={field.name}
                name={field.name}
                type="checkbox"
                checked={value as boolean}
                onChange={(event) => change(field, parseInput(field, event.currentTarget.checked))}
              />
            </label>
          );
        }
        const loader = field.relation === undefined ? undefined : relationLoaders[field.name];
        return (
          <div key={field.name}>
            <label>
              {field.name}
              <input
                aria-label={field.name}
                name={field.name}
                type={field.tsType === "number" ? "number" : "text"}
                value={value as string}
                onChange={(event) =>
                  change(field, parseInput(field, event.currentTarget.value, values[field.name]))
                }
              />
            </label>
            {loader !== undefined && <RelationOptions fieldName={field.name} loader={loader} />}
          </div>
        );
      })}
      <button type="submit">提交</button>
    </form>
  );
}

function ProductionResourceForm({ props }: { props: ProductionProps }): JSX.Element {
  const { schema, mode, resetKey, initialValues: suppliedInitialValues } = props;
  const relationLoaders = (props.relationLoaders ?? {}) as Readonly<
    Record<string, AnyRelationOptionLoader>
  >;
  const itemOverrides = (props.itemOverrides ?? {}) as unknown as Readonly<
    Record<string, FormItemOverride | undefined>
  >;
  const { initialValues, visibilityValues } = useMemo(() => {
    const defaults: Record<string, unknown> = {};
    if (mode === "create") {
      for (const field of schema.fields) {
        // Nullable fields start empty: a zero value would be stored, and for an
        // optional foreign key it references a row that does not exist.
        if (!editable(field, mode) || field.relation !== undefined || field.optional === true)
          continue;
        switch (field.tsType) {
          case "string":
            defaults[field.name] = "";
            break;
          case "number":
            defaults[field.name] = 0;
            break;
          case "bigint":
            defaults[field.name] = 0n;
            break;
          case "boolean":
            defaults[field.name] = false;
            break;
        }
      }
    }
    const visibilityValues = {
      ...defaults,
      ...(suppliedInitialValues as ResourceValues | undefined),
    };
    for (const field of schema.fields.filter((field) => field.name in defaults)) {
      const visible = itemOverrides[field.name]?.visible;
      const context = fieldContext(schema, field, mode, visibilityValues);
      if (visible === false || (typeof visible === "function" && !visible(context))) {
        delete defaults[field.name];
      }
    }
    return {
      visibilityValues,
      initialValues: {
        ...defaults,
        ...(suppliedInitialValues as ResourceValues | undefined),
      } as ResourceValues,
    };
  }, [schema, mode, suppliedInitialValues, itemOverrides]);
  const fields = useMemo(
    () => schema.fields.filter((field) => editable(field, mode)),
    [mode, schema.fields],
  );
  const [values, setValues] = useState<Record<string, unknown>>(() =>
    Object.fromEntries(fields.map((field) => [field.name, initialValues[field.name]])),
  );
  const [touched, setTouched] = useState<ReadonlySet<string>>(() => new Set());
  const [dirty, setDirty] = useState<ReadonlySet<string>>(() => new Set());
  const [errors, setErrors] = useState<Readonly<Record<string, ReactNode>>>({});
  const baselineValues = useRef<ResourceValues>(
    Object.fromEntries(fields.map((field) => [field.name, initialValues[field.name]])),
  );
  const priorInitialValues = useRef(initialValues);
  const priorResetKey = useRef(resetKey);

  useEffect(() => {
    const resetChanged = !Object.is(priorResetKey.current, resetKey);
    const valuesChanged = !sameInitialValues(priorInitialValues.current, initialValues);
    if (resetChanged) {
      const nextValues = Object.fromEntries(
        fields.map((field) => [field.name, initialValues[field.name]]),
      );
      baselineValues.current = nextValues;
      setValues(nextValues);
      setTouched(new Set());
      setDirty(new Set());
      setErrors({});
    } else if (valuesChanged && dirty.size === 0) {
      const nextValues = Object.fromEntries(
        fields.map((field) => [field.name, initialValues[field.name]]),
      );
      baselineValues.current = nextValues;
      setValues(nextValues);
      setTouched(new Set());
      setDirty(new Set());
      setErrors({});
    } else if (valuesChanged) {
      // A server refresh may update untouched fields, but never overwrites a
      // user's edit or the baseline against which that edit is measured.
      const dirtyFields = dirty;
      setValues((previous) => {
        const next = { ...previous };
        for (const field of fields) {
          if (!dirtyFields.has(field.name)) next[field.name] = initialValues[field.name];
        }
        return next;
      });
      const nextBaseline = { ...baselineValues.current };
      for (const field of fields) {
        if (!dirtyFields.has(field.name)) nextBaseline[field.name] = initialValues[field.name];
      }
      baselineValues.current = nextBaseline;
    }
    priorInitialValues.current = initialValues;
    priorResetKey.current = resetKey;
  }, [dirty, fields, initialValues, resetKey]);

  const change = (field: FieldSchema, next: unknown) => {
    setValues((previous) => ({ ...previous, [field.name]: next }));
    setTouched((previous) => new Set(previous).add(field.name));
    setDirty((previous) => {
      const updated = new Set(previous);
      if (Object.is(next, baselineValues.current[field.name])) updated.delete(field.name);
      else updated.add(field.name);
      return updated;
    });
    setErrors((previous) => {
      if (!(field.name in previous)) return previous;
      const nextErrors = { ...previous };
      delete nextErrors[field.name];
      return nextErrors;
    });
  };

  const validate = async (): Promise<boolean> => {
    const nextErrors: Record<string, ReactNode> = {};
    for (const field of fields) {
      const override = itemOverrides[field.name];
      const value = values[field.name];
      const blank = isBlank(value);
      if (field.required && blank) {
        nextErrors[field.name] = "该字段为必填项 (required)";
        continue;
      }
      if (field.maxLen > 0 && typeof value === "string" && unicodeLength(value) > field.maxLen) {
        nextErrors[field.name] = `长度不能超过 ${field.maxLen} (maxLen)`;
        continue;
      }
      for (const rule of override?.rules ?? []) {
        if (rule.required && blank) {
          nextErrors[field.name] = rule.message ?? "该字段为必填项 (required)";
          break;
        }
        if (
          rule.maxLength !== undefined &&
          typeof value === "string" &&
          unicodeLength(value) > rule.maxLength
        ) {
          nextErrors[field.name] = rule.message ?? `长度不能超过 ${rule.maxLength}`;
          break;
        }
        if (rule.validator !== undefined) {
          try {
            await rule.validator(value);
          } catch (caught) {
            nextErrors[field.name] = rule.message ?? formatErrorMessage(caught, String(caught));
            break;
          }
        }
      }
    }
    setErrors(nextErrors);
    return Object.keys(nextErrors).length === 0;
  };

  const submit = async () => {
    if (props.loading || !(await validate())) return;
    const submissionValues = {
      ...(typeof initialValues.$typeName === "string"
        ? { $typeName: initialValues.$typeName }
        : {}),
      ...Object.fromEntries(fields.map((field) => [field.name, values[field.name]])),
    } as ResourceValues;
    if (mode === "create") {
      const createProps = props as unknown as RuntimeCreateProps;
      if (createProps.onCreate !== undefined) {
        await createProps.onCreate({ values: submissionValues, touched, dirty });
      } else if (createProps.operations !== undefined) {
        await createProps.operations.create(submissionValues);
      }
    } else {
      const updateProps = props as unknown as RuntimeUpdateProps;
      const submission = {
        key: updateProps.rowKey,
        values: submissionValues,
        touched,
        dirty,
      };
      if (updateProps.onUpdate !== undefined) await updateProps.onUpdate(submission);
      else if (updateProps.operations !== undefined)
        await updateProps.operations.update(submission);
    }
  };

  const reset = (): void => {
    const nextValues = Object.fromEntries(
      fields.map((field) => [field.name, baselineValues.current[field.name]]),
    );
    setValues(nextValues);
    setTouched(new Set());
    setDirty(new Set());
    setErrors({});
  };

  const renderField = (field: FieldSchema) => {
    const override = itemOverrides[field.name];
    const context = fieldContext(schema, field, mode, initialValues);
    const visible = override?.visible;
    if (
      visible === false ||
      (typeof visible === "function" &&
        !visible(fieldContext(schema, field, mode, visibilityValues)))
    )
      return null;
    const disabled =
      override?.disabled === true ||
      (typeof override?.disabled === "function" && override.disabled(context));
    const label = override?.label ?? field.label ?? field.name;
    const value = values[field.name];
    const fieldProps = override?.fieldProps;
    const ariaLabel = typeof label === "string" ? label : field.name;
    const common = {
      id: field.name,
      name: field.name,
      "aria-label": ariaLabel,
      disabled,
      placeholder: fieldProps?.placeholder,
    };

    let control: ReactNode;
    if (override?.component !== undefined) {
      const Component = override.component as ComponentType<{
        readonly value: unknown;
        readonly values: Readonly<Record<string, unknown>>;
        readonly disabled?: boolean;
        readonly onChange: (value: unknown) => void;
      }>;
      control = (
        <Component
          value={value}
          values={values}
          disabled={disabled}
          onChange={(nextValue) => change(field, nextValue)}
        />
      );
    } else if (field.relation !== undefined) {
      control = (
        <RelationSelect
          fieldName={field.name}
          fieldProps={
            fieldProps as
              | { readonly options?: readonly RelationOption[]; readonly showSearch?: boolean }
              | undefined
          }
          loader={relationLoaders[field.name]}
          value={value}
          disabled={disabled}
          onChange={(nextValue) => change(field, nextValue)}
        />
      );
    } else if (override?.valueType === "textarea") {
      control = (
        <Input.TextArea
          {...common}
          rows={fieldProps?.rows}
          onChange={(event) => change(field, event.currentTarget.value)}
          value={inputValue(field, value) as string}
        />
      );
    } else if (override?.valueType === "select") {
      const selectOptions = (fieldProps?.options ?? []).flatMap((option) =>
        typeof option.value === "string" ||
        typeof option.value === "number" ||
        typeof option.value === "bigint"
          ? [{ value: option.value, label: option.label }]
          : [],
      );
      control = (
        <Select
          {...common}
          value={value as RelationValue | undefined}
          options={selectOptions}
          onChange={(nextValue) => change(field, nextValue)}
        />
      );
    } else if (field.tsType === "number" || override?.valueType === "number") {
      control = (
        <InputNumber
          {...common}
          min={fieldProps?.min}
          max={fieldProps?.max}
          step={fieldProps?.step}
          value={value as number | undefined}
          onChange={(nextValue) => change(field, nextValue ?? emptyValue(field))}
        />
      );
    } else if (field.tsType === "boolean" || override?.valueType === "boolean") {
      control = (
        <Switch
          {...common}
          checked={Boolean(value)}
          onChange={(nextValue) => change(field, nextValue)}
        />
      );
    } else {
      control = (
        <Input
          {...common}
          type={field.tsType === "bigint" ? "text" : "text"}
          value={inputValue(field, value) as string}
          onChange={(event) =>
            change(field, parseInput(field, event.currentTarget.value, values[field.name]))
          }
        />
      );
    }

    return (
      <Form.Item
        key={field.name}
        label={label}
        required={field.required}
        validateStatus={errors[field.name] === undefined ? undefined : "error"}
        help={errors[field.name]}
      >
        {control}
      </Form.Item>
    );
  };

  return (
    <ProForm submitter={false} onFinish={() => submit()}>
      {props.error !== undefined && (
        <div className="resource-form__error" role="alert">
          {formatErrorMessage(props.error)}
        </div>
      )}
      {fields.map(renderField)}
      {props.extraFields}
      <Space>
        <Button htmlType="button" autoInsertSpace={false} disabled={props.loading} onClick={reset}>
          重置
        </Button>
        <Button
          type="primary"
          htmlType="submit"
          autoInsertSpace={false}
          loading={props.loading}
          onClick={(event) => {
            // ResourceForm owns validation and the controlled values. Handle
            // the click explicitly instead of letting the browser submit a
            // native form with an unrelated Form.Item state.
            event.preventDefault();
            void submit();
          }}
        >
          {props.loading ? "提交中" : "提交"}
        </Button>
      </Space>
    </ProForm>
  );
}

/**
 * ResourceForm is a production ProForm orchestration layer. The old native
 * callback remains intentionally isolated so existing prototype consumers do
 * not change while pages migrate to the typed create/update contract.
 */
export function ResourceForm<
  TSchema extends ResourceSchema,
  TValues extends object,
  TMode extends ResourceFormMode,
  TKey extends ResourceKey,
  TCreateValues extends object,
  TUpdateValues extends object,
  TUpdateField extends keyof TUpdateValues,
  TEntity extends object,
>(
  props: ResourceFormProps<
    TSchema,
    TValues,
    TMode,
    TKey,
    TCreateValues,
    TUpdateValues,
    TUpdateField,
    TEntity
  >,
): JSX.Element;
export function ResourceForm(props: ResourceFormPrototypeProps): JSX.Element;
export function ResourceForm(props: object): JSX.Element {
  if ("onSubmit" in props)
    return <LegacyResourceForm props={props as ResourceFormPrototypeProps} />;
  return <ProductionResourceForm props={props as ProductionProps} />;
}
