import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactElement,
  type ReactNode,
} from "react";
import { Button, Input, Modal, Space, type ButtonProps } from "antd";
import { ProTable, type ProColumns } from "@ant-design/pro-components";
import type { FilterValue, SorterResult, TablePaginationConfig } from "antd/es/table/interface";

import {
  resourcePermissions,
  type ResourceColumnOverrides,
  type ResourceRowActionOverride,
  type ResourceStandardRowActionOverride,
  type ResourceStandardToolbarOverride,
} from "./metadata";
import {
  resourceListRows,
  toProTableTotal,
  type ResourceKey,
  type ResourceListQuery,
  type ResourceOperationError,
  type ResourceReadOperations,
  type ResourceUpdateSubmission,
} from "./operations";
import type {
  ResourceTableProductionProps,
  ResourceTablePrototypeProps,
  ResourceTableReadOnlyProps,
} from "./table-contract";
import { ResourceForm } from "./ResourceForm";
import type {
  ResourceFormCreateProps,
  ResourceCreateFormValues,
  ResourceUpdateFormValues,
  ResourceFormUpdateProps,
} from "./form-contract";
import type { ResourceTableCreateFormConfig, ResourceTableEditFormConfig } from "./table-contract";
import type { ResourceSchema, SortableFieldName } from "./schema";
import { formatErrorMessage } from "../../runtime/message";
import "./ResourceTable.css";

export type { ResourceTableProps } from "./table-contract";

function display(value: unknown): string {
  if (typeof value === "bigint") {
    return value.toString();
  }
  if (value === undefined || value === null) {
    return "";
  }
  return String(value);
}

function operationError(error: unknown): ResourceOperationError {
  if (error instanceof Error) {
    return error as ResourceOperationError;
  }
  return new Error(String(error)) as ResourceOperationError;
}

function fieldValue<TEntity>(entity: TEntity, fieldName: string): unknown {
  return (entity as unknown as Record<string, unknown>)[fieldName];
}

function defaultValueType(tsType: string): string {
  switch (tsType) {
    case "number":
      return "digit";
    case "bigint":
      return "text";
    case "boolean":
      return "switch";
    default:
      return "text";
  }
}

type PendingConfirmation<TKey extends ResourceKey> = {
  readonly kind: "delete" | "batchDelete";
  readonly keys: readonly TKey[];
};

type ActionButtonProps = {
  readonly title: ReactNode;
  readonly onClick: () => void;
  readonly disabled?: boolean;
  readonly buttonType?: ButtonProps["type"];
  readonly danger?: boolean;
};

type TableCreateModalProps<
  TSchema extends ResourceSchema,
  TEntity extends object,
  TKey extends ResourceKey,
  TFilters extends object,
  TCreateValues extends object,
  TUpdateValues extends object,
> = {
  readonly schema: TSchema;
  readonly operations: ResourceTableProductionProps<
    TSchema,
    TEntity,
    TKey,
    TFilters,
    TCreateValues,
    TUpdateValues
  >["operations"];
  readonly config: ResourceTableCreateFormConfig<TSchema, TEntity, TCreateValues>;
  readonly title: ReactNode;
  readonly reload: () => Promise<void>;
  readonly onError?: (error: ResourceOperationError) => void;
};

function CreateFormModal<
  TSchema extends ResourceSchema,
  TEntity extends object,
  TKey extends ResourceKey,
  TFilters extends object,
  TCreateValues extends object,
  TUpdateValues extends object,
>({
  schema,
  operations,
  config,
  title,
  reload,
  onError,
}: TableCreateModalProps<
  TSchema,
  TEntity,
  TKey,
  TFilters,
  TCreateValues,
  TUpdateValues
>): ReactElement {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);

  const submit: NonNullable<
    ResourceFormCreateProps<
      TSchema,
      TCreateValues,
      keyof TCreateValues,
      unknown,
      TEntity
    >["onCreate"]
  > = async ({ values }) => {
    setLoading(true);
    try {
      const entity = await operations.create(values);
      await config.onSuccess?.(entity);
      setOpen(false);
      await reload();
    } catch (cause) {
      const error = operationError(cause);
      onError?.(error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button type="primary" autoInsertSpace={false} onClick={() => setOpen(true)}>
        {title}
      </Button>
      <Modal
        title={config.title ?? title}
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        destroyOnHidden
      >
        <ResourceForm<
          TSchema,
          ResourceCreateFormValues<TSchema, TCreateValues>,
          "create",
          TKey,
          TCreateValues,
          ResourceUpdateFormValues<TSchema, TUpdateValues>,
          keyof TUpdateValues,
          TEntity
        >
          schema={schema}
          mode="create"
          initialValues={config.initialValues}
          relationLoaders={config.relationLoaders}
          itemOverrides={config.itemOverrides}
          loading={loading}
          onCreate={submit}
        />
      </Modal>
    </>
  );
}

type TableEditModalProps<
  TSchema extends ResourceSchema,
  TEntity extends object,
  TKey extends ResourceKey,
  TFilters extends object,
  TCreateValues extends object,
  TUpdateValues extends object,
> = {
  readonly schema: TSchema;
  readonly operations: ResourceTableProductionProps<
    TSchema,
    TEntity,
    TKey,
    TFilters,
    TCreateValues,
    TUpdateValues
  >["operations"];
  readonly config: ResourceTableEditFormConfig<TSchema, TEntity, TUpdateValues>;
  readonly entity: TEntity;
  readonly rowKey: TKey;
  readonly title: ReactNode;
  readonly reload: () => Promise<void>;
  readonly onError?: (error: ResourceOperationError) => void;
};

function EditFormModal<
  TSchema extends ResourceSchema,
  TEntity extends object,
  TKey extends ResourceKey,
  TFilters extends object,
  TCreateValues extends object,
  TUpdateValues extends object,
>({
  schema,
  operations,
  config,
  entity,
  rowKey,
  title,
  reload,
  onError,
}: TableEditModalProps<
  TSchema,
  TEntity,
  TKey,
  TFilters,
  TCreateValues,
  TUpdateValues
>): ReactElement {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const initialValues = {
    ...Object.fromEntries(
      schema.fields
        .filter((field) => field.updatable && !field.serverManaged)
        .map((field) => [field.name, fieldValue(entity, field.name)]),
    ),
    ...config.initialValues?.(entity),
  } as Readonly<Partial<ResourceUpdateFormValues<TSchema, TUpdateValues>>>;

  const submit: NonNullable<
    ResourceFormUpdateProps<
      TSchema,
      TKey,
      TUpdateValues,
      keyof TUpdateValues,
      unknown,
      TEntity
    >["onUpdate"]
  > = async (submission) => {
    setLoading(true);
    try {
      const updated = await operations.update(
        submission as ResourceUpdateSubmission<TKey, TUpdateValues>,
      );
      await config.onSuccess?.(updated);
      setOpen(false);
      await reload();
    } catch (cause) {
      const error = operationError(cause);
      onError?.(error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button type="link" autoInsertSpace={false} onClick={() => setOpen(true)}>
        {title}
      </Button>
      <Modal
        title={config.title ?? title}
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        destroyOnHidden
      >
        <ResourceForm<
          TSchema,
          ResourceUpdateFormValues<TSchema, TUpdateValues>,
          "update",
          TKey,
          TCreateValues,
          TUpdateValues,
          keyof TUpdateValues,
          TEntity
        >
          schema={schema}
          mode="update"
          rowKey={rowKey}
          initialValues={initialValues}
          relationLoaders={config.relationLoaders}
          itemOverrides={config.itemOverrides}
          loading={loading}
          onUpdate={submit}
        />
      </Modal>
    </>
  );
}

function ActionButton({
  title,
  onClick,
  disabled,
  buttonType = "link",
  danger,
}: ActionButtonProps): ReactElement {
  return (
    <Button
      type={buttonType}
      danger={danger}
      autoInsertSpace={false}
      disabled={disabled}
      onClick={onClick}
    >
      {title}
    </Button>
  );
}

async function loadAllPages<
  TEntity,
  TKey extends ResourceKey,
  TFilters extends object,
  TOrderField extends string,
>(
  operations: ResourceReadOperations<TEntity, TKey, TFilters, TOrderField>,
  query: ResourceListQuery<TFilters, TOrderField>,
  pageSize: number,
) {
  const items: TEntity[] = [];
  let pageNum = 1;
  let total = 0n;
  do {
    const result = await operations.list({ ...query, pageNum, pageSize });
    const pageItems = resourceListRows(result);
    items.push(...pageItems);
    total = result.total;
    if (pageItems.length === 0) {
      // A concurrently changing dataset can report a stale total. Do not
      // spin forever when a later page is empty but the stale total is still
      // larger than the number of rows collected so far.
      if (BigInt(items.length) < total) {
        throw new Error("列表返回不完整");
      }
      break;
    }
    pageNum += 1;
  } while (BigInt(items.length) < total);
  return { items, total, pageNum: 1, pageSize };
}

function ProductionResourceTable<
  TSchema extends ResourceSchema,
  TEntity extends object,
  TKey extends ResourceKey,
  TFilters extends object,
  TCreateValues extends object,
  TUpdateValues extends object,
>(
  props:
    | ResourceTableProductionProps<TSchema, TEntity, TKey, TFilters, TCreateValues, TUpdateValues>
    | ResourceTableReadOnlyProps<TSchema, TEntity, TKey, TFilters>,
): ReactElement {
  const {
    schema,
    operations,
    access,
    rowKey,
    permissionMap,
    columns: columnOverrides,
    rowActions: rowActionOverrides,
    toolbar: toolbarOverrides,
    query: queryConfig,
    form,
    treeData,
    renderError,
    onError,
    emptyText = "暂无数据",
    reload: externalReload,
  } = props;
  const isReadOnly = props.readOnly === true;

  type TOrderField = SortableFieldName<TSchema>;
  type TQuery = ResourceListQuery<TFilters, TOrderField>;

  const permissions = useMemo(() => {
    if (permissionMap !== undefined) return permissionMap;
    const defaults = resourcePermissions(schema.permPrefix);
    const generated = schema.permissions;
    return {
      list: generated?.list ?? defaults.list,
      get: generated?.get ?? defaults.get,
      create: generated?.create ?? defaults.create,
      update: generated?.update ?? defaults.update,
      delete: generated?.delete ?? defaults.delete,
      batchDelete: generated?.batchDelete ?? defaults.batchDelete,
    };
  }, [permissionMap, schema.permissions, schema.permPrefix]);
  const canList = access.can(permissions.list);
  const canCreate = !isReadOnly && access.can(permissions.create);
  const canUpdate = !isReadOnly && access.can(permissions.update);
  const canDelete = !isReadOnly && access.can(permissions.delete);
  const canBatchDelete = !isReadOnly && access.can(permissions.batchDelete);
  // get is part of the fixed Resource permission map even though the table
  // usually receives entities from List. Evaluating it keeps access decisions
  // centralized and lets a page apply a stricter get policy when needed.
  access.can(permissions.get);

  const initialQuery = useMemo<TQuery>(() => {
    const defaults = queryConfig?.defaults ?? {};
    const initial = queryConfig?.initial ?? {};
    return {
      pageNum: initial.pageNum ?? defaults.pageNum ?? 1,
      pageSize: initial.pageSize ?? defaults.pageSize ?? 10,
      ...(initial.keyword !== undefined || defaults.keyword !== undefined
        ? { keyword: initial.keyword ?? defaults.keyword }
        : {}),
      ...(initial.orders !== undefined || defaults.orders !== undefined
        ? { orders: initial.orders ?? defaults.orders }
        : {}),
      ...(initial.filters !== undefined || defaults.filters !== undefined
        ? { filters: initial.filters ?? defaults.filters }
        : {}),
    } as TQuery;
  }, [queryConfig]);

  const [currentQuery, setCurrentQuery] = useState<TQuery>(initialQuery);
  const [keyword, setKeyword] = useState(initialQuery.keyword ?? "");
  const [rows, setRows] = useState<readonly TEntity[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ResourceOperationError | undefined>();
  const [selectedKeys, setSelectedKeys] = useState<readonly TKey[]>([]);
  const [pending, setPending] = useState<PendingConfirmation<TKey> | undefined>();
  const latestLoad = useRef(0);
  const clearSelection = useCallback(() => setSelectedKeys([]), []);

  const getRowKey = useCallback(
    (entity: Readonly<TEntity>, index = 0): TKey => {
      if (typeof rowKey === "function") {
        return rowKey(entity, index);
      }
      if (typeof rowKey === "string") {
        return fieldValue(entity, rowKey) as TKey;
      }
      return operations.keyOf(entity);
    },
    [operations, rowKey],
  );

  const load = useCallback(
    async (nextQuery: TQuery): Promise<void> => {
      if (!canList) {
        const denied = operationError(`Permission denied: ${permissions.list}`);
        setError(denied);
        onError?.(denied);
        return;
      }
      const loadID = ++latestLoad.current;
      setLoading(true);
      setError(undefined);
      try {
        const result = treeData
          ? await loadAllPages(operations, nextQuery, treeData.pageSize ?? 100)
          : await operations.list(nextQuery);
        if (loadID !== latestLoad.current) return;
        const flatRows = resourceListRows(result);
        setRows(treeData === undefined ? flatRows : treeData.build(flatRows));
        setTotal(toProTableTotal(result.total));
        setCurrentQuery(nextQuery);
        queryConfig?.onChange?.(nextQuery);
      } catch (cause) {
        if (loadID !== latestLoad.current) return;
        const nextError = operationError(cause);
        setError(nextError);
        onError?.(nextError);
      } finally {
        if (loadID === latestLoad.current) setLoading(false);
      }
    },
    [canList, onError, operations, permissions.list, queryConfig, treeData],
  );

  const reload = useCallback(async (): Promise<void> => {
    await load(currentQuery);
  }, [currentQuery, load]);

  useEffect(() => {
    void load(initialQuery);
    return () => {
      latestLoad.current += 1;
    };
    // The first request is intentionally tied to the operations instance, not
    // to currentQuery: query controls call load explicitly and must not double
    // submit when React batches their state updates.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [operations, canList]);

  const visibleFields = useMemo(
    () =>
      schema.fields.filter((field) => {
        if (!field.listable) return false;
        const override =
          columnOverrides?.[field.name as keyof ResourceColumnOverrides<TSchema, TEntity>];
        const hidden = override?.hidden;
        const hiddenByOverride =
          hidden === true ||
          (typeof hidden === "function" &&
            hidden({ schema, fieldName: field.name, field, mode: "update" }));
        return override?.listable !== false && !hiddenByOverride;
      }),
    // Schema fields are the stable dependency used by this projection; the
    // hidden-field callback is intentionally not a separate dependency.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [columnOverrides, schema.fields],
  );

  const proColumns = useMemo<ProColumns<TEntity & Record<string, unknown>>[]>(
    () =>
      visibleFields.map((field) => {
        const override =
          columnOverrides?.[field.name as keyof ResourceColumnOverrides<TSchema, TEntity>];
        const searchable = field.searchable && override?.searchable !== false;
        const sortable = field.sortable && override?.sortable !== false;
        const render = override?.render as
          | ((args: {
              value: unknown;
              entity: TEntity;
              index?: number;
              reload: () => Promise<void>;
            }) => ReactNode)
          | undefined;
        const renderCell =
          render !== undefined
            ? (_value: unknown, entity: TEntity, index: number) =>
                render({ value: fieldValue(entity, field.name), entity, index, reload })
            : field.tsType === "bigint"
              ? (_value: unknown, entity: TEntity) => display(fieldValue(entity, field.name))
              : undefined;
        return {
          key: field.name,
          dataIndex: field.name,
          title: override?.title ?? field.label ?? field.name,
          valueType: override?.valueType ?? defaultValueType(field.tsType),
          search: searchable,
          sorter: sortable,
          sortDirections: sortable ? ["ascend", "descend", null] : undefined,
          hideInTable: override?.hidden === true || override?.listable === false,
          render: renderCell,
        } as ProColumns<TEntity & Record<string, unknown>>;
      }),
    [columnOverrides, reload, visibleFields],
  );

  const findRowActionOverride = useCallback(
    (action: "update" | "delete") => {
      if (isReadOnly) return undefined;
      return rowActionOverrides?.find(
        (override): override is ResourceStandardRowActionOverride<TEntity, TKey> =>
          override.action === action,
      );
    },
    [isReadOnly, rowActionOverrides],
  );

  const runQuery = useCallback(
    (nextQuery: TQuery): void => {
      setCurrentQuery(nextQuery);
      void load(nextQuery);
    },
    [load],
  );

  const queryAction = (action: "search" | "reload"): void => {
    if (action === "search") {
      runQuery({ ...currentQuery, pageNum: 1, keyword });
      return;
    }
    void reload();
  };

  const confirmPending = async (): Promise<void> => {
    const action = pending;
    setPending(undefined);
    if (action === undefined) return;
    if (!("delete" in operations) || !("batchDelete" in operations)) return;
    try {
      if (action.kind === "delete") {
        await operations.delete(action.keys[0]);
      } else {
        await operations.batchDelete(action.keys);
      }
      if (action.kind === "batchDelete") {
        setSelectedKeys([]);
      }
      await externalReload?.();
      await reload();
    } catch (cause) {
      // Mutation failures are not list-load failures: keep the current rows
      // visible and let the page-level callback show the business message.
      // The list error boundary is reserved for operations.list failures.
      const nextError = operationError(cause);
      onError?.(nextError);
    }
  };

  const invokeCustomRowAction = async (
    override: Extract<ResourceRowActionOverride<TEntity, TKey>, { action: "custom" }>,
    entity: TEntity,
    key: TKey,
  ): Promise<void> => {
    await override.onClick?.({ entity, key, selectedKeys, reload });
  };

  const renderCreate = (): ReactNode => {
    if (isReadOnly || !canCreate || !("create" in operations)) return null;
    const override = toolbarOverrides?.find(
      (item): item is ResourceStandardToolbarOverride<TEntity, TKey> => item.action === "create",
    );
    if (override?.merge === "hide") return null;
    const context = { selectedKeys, rows, reload, clearSelection };
    if (override?.render) {
      return override.render(context);
    }
    if (form?.createConfig) {
      return (
        <CreateFormModal
          schema={schema}
          operations={operations}
          config={form.createConfig}
          title={override?.title ?? "新建"}
          reload={reload}
          onError={onError}
        />
      );
    }
    if (form?.create) {
      return form.create({ schema, operations, reload });
    }
    return <ActionButton title={override?.title ?? "新建"} disabled onClick={() => undefined} />;
  };

  const renderToolbar = (): ReactNode[] => {
    const items: ReactNode[] = [];
    const create = renderCreate();
    if (create !== null) items.push(<span key="create">{create}</span>);

    const batchOverride = toolbarOverrides?.find(
      (item): item is ResourceStandardToolbarOverride<TEntity, TKey> =>
        item.action === "batchDelete",
    );
    if (canBatchDelete && batchOverride?.merge !== "hide") {
      items.push(
        batchOverride?.render ? (
          <span key="batch-delete">
            {batchOverride.render({ selectedKeys, rows, reload, clearSelection })}
          </span>
        ) : (
          <ActionButton
            key="batch-delete"
            title={batchOverride?.title ?? "批量删除"}
            disabled={selectedKeys.length === 0}
            buttonType="default"
            danger
            onClick={() => setPending({ kind: "batchDelete", keys: selectedKeys })}
          />
        ),
      );
    }

    for (const override of toolbarOverrides ?? []) {
      if (override.action !== "custom") continue;
      if (!access.can(override.permission)) continue;
      if (override.hidden === true) continue;
      const context = { selectedKeys, rows, reload, clearSelection };
      if (typeof override.hidden === "function" && override.hidden(context)) continue;
      items.push(
        override.render ? (
          <span key={override.name}>{override.render(context)}</span>
        ) : (
          <ActionButton
            key={override.name}
            title={override.title ?? override.name}
            disabled
            onClick={() => undefined}
          />
        ),
      );
    }
    return items;
  };

  const renderRowAction = (entity: TEntity, index: number): ReactNode[] => {
    const key = getRowKey(entity, index);
    const items: ReactNode[] = [];
    const updateOverride = findRowActionOverride("update");
    if (!isReadOnly && "update" in operations && canUpdate && updateOverride?.merge !== "hide") {
      const updateContext = { schema, entity, key, operations, reload };
      const updateNode = updateOverride?.onClick ? (
        <ActionButton
          title={updateOverride.title ?? "编辑"}
          onClick={() => void updateOverride.onClick?.({ entity, key, selectedKeys, reload })}
        />
      ) : form?.editConfig ? (
        <EditFormModal
          schema={schema}
          operations={operations}
          config={form.editConfig}
          entity={entity}
          rowKey={key}
          title={updateOverride?.title ?? "编辑"}
          reload={reload}
          onError={onError}
        />
      ) : form?.edit ? (
        form.edit(updateContext)
      ) : (
        <ActionButton
          title={updateOverride?.title ?? "编辑"}
          disabled={updateOverride?.onClick === undefined}
          onClick={() => void updateOverride?.onClick?.({ entity, key, selectedKeys, reload })}
        />
      );
      items.push(<span key="update">{updateNode}</span>);
    }

    const deleteOverride = findRowActionOverride("delete");
    if (!isReadOnly && "delete" in operations && canDelete && deleteOverride?.merge !== "hide") {
      items.push(
        <ActionButton
          key="delete"
          title={deleteOverride?.title ?? "删除"}
          danger
          onClick={() => {
            if (deleteOverride?.onClick) {
              void deleteOverride.onClick({ entity, key, selectedKeys, reload });
            } else {
              setPending({ kind: "delete", keys: [key] });
            }
          }}
        />,
      );
    }

    for (const override of rowActionOverrides ?? []) {
      if (override.action !== "custom") continue;
      if (!access.can(override.permission, { entity, key })) continue;
      if (override.hidden === true) continue;
      const context = { entity, key, selectedKeys, reload };
      if (typeof override.hidden === "function" && override.hidden(context)) continue;
      items.push(
        override.render ? (
          <span key={override.name}>{override.render(context)}</span>
        ) : (
          <ActionButton
            key={override.name}
            title={override.title ?? override.name}
            onClick={() => void invokeCustomRowAction(override, entity, key)}
          />
        ),
      );
    }
    return items;
  };

  const actionColumn: ProColumns<TEntity & Record<string, unknown>> | undefined =
    canUpdate || canDelete || (rowActionOverrides ?? []).some((item) => item.action === "custom")
      ? {
          key: "actions",
          title: "操作",
          render: (_value, entity, index) => renderRowAction(entity as TEntity, index),
        }
      : undefined;

  const errorContent = error
    ? (renderError?.(error, reload) ?? (
        <div role="alert">
          {formatErrorMessage(error)}
          <Button type="link" autoInsertSpace={false} onClick={() => void reload()}>
            重试
          </Button>
        </div>
      ))
    : null;

  const onProTableChange = (
    pagination: TablePaginationConfig,
    _filters: Record<string, FilterValue | null>,
    sorter:
      | SorterResult<TEntity & Record<string, unknown>>
      | SorterResult<TEntity & Record<string, unknown>>[],
  ): void => {
    const sorters = Array.isArray(sorter) ? sorter : [sorter];
    const sorterWithField = sorters.find((candidate) => typeof candidate.field === "string");
    const activeSorter = sorters.find(
      (candidate) =>
        (candidate.order === "ascend" || candidate.order === "descend") &&
        typeof candidate.field === "string",
    );
    const sortableNames = schema.fields
      .filter((field) => {
        const override =
          columnOverrides?.[field.name as keyof ResourceColumnOverrides<TSchema, TEntity>];
        return field.sortable && override?.sortable !== false;
      })
      .map((field) => field.name);
    const activeField =
      typeof activeSorter?.field === "string" && sortableNames.includes(activeSorter.field)
        ? (activeSorter.field as TOrderField)
        : undefined;
    // ProTable/Ant Table represents a cleared sorter as an object whose
    // field and order are both unset (some versions use an empty array).
    // Restore the Resource query defaults instead of letting injected
    // repositories fall back to a different key-only order.
    const sorterCleared =
      sorterWithField === undefined &&
      activeSorter === undefined &&
      (sorters.length === 0 ||
        sorters.every((candidate) => candidate.field === undefined && candidate.order == null));
    const nextQuery: TQuery = {
      ...currentQuery,
      pageNum: pagination.current ?? currentQuery.pageNum,
      pageSize: pagination.pageSize ?? currentQuery.pageSize,
      orders:
        activeField !== undefined
          ? [{ field: activeField, desc: activeSorter?.order === "descend" }]
          : sorterCleared
            ? queryConfig?.defaults?.orders
            : currentQuery.orders,
    };
    runQuery(nextQuery);
  };

  return (
    <div data-resource-table={schema.name} className="resource-table">
      <div className="resource-table__search">
        <Space role="search">
          <Input
            aria-label="关键词"
            placeholder="请输入关键词"
            value={keyword}
            onChange={(event) => setKeyword(event.currentTarget.value)}
          />
          <Button type="primary" autoInsertSpace={false} onClick={() => queryAction("search")}>
            查询
          </Button>
          <Button autoInsertSpace={false} onClick={() => queryAction("reload")}>
            刷新
          </Button>
        </Space>
      </div>
      <div className="resource-table__content">
        {errorContent ?? (
          <ProTable<TEntity & Record<string, unknown>, TQuery>
            columns={actionColumn ? [...proColumns, actionColumn] : proColumns}
            // ProTable row identity and controlled selection share the same UI key.
            // The selectedKeys state retains the original typed operation keys.
            rowKey={(entity) => display(getRowKey(entity as TEntity))}
            rowSelection={{
              selectedRowKeys: selectedKeys.map((key) => display(key)),
              onChange: (keys) => {
                const selected = new Set(keys.map((key) => String(key)));
                const selectableRows = treeData?.flatten?.(rows) ?? rows;
                setSelectedKeys(
                  selectableRows
                    .map((entity, index) => getRowKey(entity, index))
                    .filter((key) => selected.has(display(key))),
                );
              },
            }}
            search={false}
            options={false}
            sortDirections={["ascend", "descend", null]}
            onChange={onProTableChange}
            toolBarRender={() => renderToolbar()}
            dataSource={rows as readonly (TEntity & Record<string, unknown>)[]}
            loading={loading}
            locale={{ emptyText: loading ? "加载中" : emptyText }}
            pagination={
              treeData
                ? false
                : {
                    current: currentQuery.pageNum,
                    pageSize: currentQuery.pageSize,
                    total,
                  }
            }
          />
        )}
      </div>
      {pending ? (
        <Modal
          open
          title={pending.kind === "batchDelete" ? "确认批量删除" : "确认删除"}
          onCancel={() => setPending(undefined)}
          footer={[
            <Button key="cancel" autoInsertSpace={false} onClick={() => setPending(undefined)}>
              取消
            </Button>,
            <Button
              key="confirm"
              type="primary"
              danger
              autoInsertSpace={false}
              onClick={() => void confirmPending()}
            >
              {pending.kind === "batchDelete" ? "确认批量删除" : "确认删除"}
            </Button>,
          ]}
        >
          <p>{pending.kind === "batchDelete" ? "确认批量删除？" : "确认删除？"}</p>
        </Modal>
      ) : null}
    </div>
  );
}

/** ResourceTable renders only generated listable fields and never coerces bigint to number. */
export function ResourceTable(props: ResourceTablePrototypeProps): ReactElement;
export function ResourceTable<
  TSchema extends ResourceSchema,
  TEntity,
  TKey extends ResourceKey,
  TFilters extends object,
>(props: ResourceTableReadOnlyProps<TSchema, TEntity, TKey, TFilters>): ReactElement;
export function ResourceTable<
  TSchema extends ResourceSchema,
  TEntity,
  TKey extends ResourceKey,
  TFilters extends object,
  TCreateValues extends object,
  TUpdateValues extends object,
>(
  props: ResourceTableProductionProps<
    TSchema,
    TEntity,
    TKey,
    TFilters,
    TCreateValues,
    TUpdateValues
  >,
): ReactElement;
export function ResourceTable(
  props:
    | ResourceTablePrototypeProps
    | ResourceTableProductionProps<ResourceSchema, object, ResourceKey, object, object, object>
    | ResourceTableReadOnlyProps<ResourceSchema, object, ResourceKey, object>,
): ReactElement {
  if (!("rows" in props)) {
    return <ProductionResourceTable {...props} />;
  }

  const { schema, rows, rowKey } = props;
  const fields = schema.fields.filter((field) => field.listable);
  return (
    <table>
      <thead>
        <tr>
          {fields.map((field) => (
            <th key={field.name} scope="col">
              {field.name}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, index) => (
          <tr key={display(rowKey(row, index))}>
            {fields.map((field) => (
              <td key={field.name}>{display(row[field.name])}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
