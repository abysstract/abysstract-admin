import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type MouseEvent,
  type ReactElement,
  type ReactNode,
} from "react";
import { EllipsisOutlined } from "@ant-design/icons";
import { Button, Dropdown, Empty, Modal, Space, type ButtonProps } from "antd";
import { useHref, useInRouterContext, useLinkClickHandler } from "react-router-dom";
import { ProTable, type ProColumns } from "@ant-design/pro-components";
import type { FilterValue, SorterResult, TablePaginationConfig } from "antd/es/table/interface";

import {
  resourcePermissions,
  type ResourceColumnOverrides,
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
import { formatErrorMessage, useAppFeedback } from "../../runtime/message";
import { useAdminTexts } from "../../runtime/texts";
import { ResourceFormDrawer } from "./ResourceFormDrawer";
import { readUrlQuery, writeUrlQuery } from "./urlQuery";
import { AdminButtonSpacing } from "../AdminButtonSpacing";
import { ResourceSearchBar, useResourceSurfaceStyle } from "./ResourceSearchBar";
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
  readonly buttonType?: ButtonProps["type"];
  readonly danger?: boolean;
};

/** 一行里的一个操作：文字链接（可收进「⋯」）或页面自定义的控件（始终可见）。 */
type RowAction =
  | {
      readonly key: string;
      readonly kind: "button";
      readonly title: ReactNode;
      readonly onClick: () => void;
      readonly danger?: boolean;
      /** Link target: rendered as an anchor instead of a click handler. */
      readonly href?: string;
    }
  | { readonly key: string; readonly kind: "node"; readonly node: ReactNode };

/** 操作列最多露出两个文字链接，其余收进最右边的「⋯」（见 DESIGN.md「数据列表」）。 */
const MAX_INLINE_ROW_ACTIONS = 2;

function ActionButton({
  title,
  onClick,
  buttonType = "link",
  danger,
}: ActionButtonProps): ReactElement {
  return (
    <Button
      type={buttonType}
      size={buttonType === "link" ? "small" : undefined}
      danger={danger}
      onClick={onClick}
    >
      {title}
    </Button>
  );
}

type RowLinkProps = {
  readonly to: string;
  readonly title: ReactNode;
  readonly danger?: boolean;
  /** "button": inline link button; "text": a plain anchor inside the "⋯" menu. */
  readonly variant: "button" | "text";
};

/**
 * A row action that opens another page. It is a real anchor (`href`), so the
 * browser opens it in a new tab on middle-click, Ctrl/⌘-click or from the
 * context menu; a plain left click navigates inside the app.
 */
function RowLink(props: RowLinkProps): ReactElement {
  return useInRouterContext() ? (
    <RouterRowLink {...props} />
  ) : (
    <RowAnchor {...props} href={props.to} />
  );
}

function RouterRowLink(props: RowLinkProps): ReactElement {
  const href = useHref(props.to);
  const navigate = useLinkClickHandler<HTMLElement>(props.to);
  return <RowAnchor {...props} href={href} onClick={navigate} />;
}

function RowAnchor({
  href,
  title,
  danger,
  variant,
  onClick,
}: RowLinkProps & {
  readonly href: string;
  readonly onClick?: (event: MouseEvent<HTMLElement>) => void;
}): ReactElement {
  if (variant === "text") {
    return (
      <a href={href} onClick={onClick}>
        {title}
      </a>
    );
  }
  return (
    <Button type="link" size="small" danger={danger} href={href} onClick={onClick}>
      {title}
    </Button>
  );
}

function RowActions({
  actions,
  moreLabel,
}: {
  readonly actions: readonly RowAction[];
  readonly moreLabel: string;
}): ReactElement {
  const nodes = actions.filter((action) => action.kind === "node");
  const buttons = actions.filter(
    (action): action is Extract<RowAction, { kind: "button" }> => action.kind === "button",
  );
  const collapse = buttons.length > MAX_INLINE_ROW_ACTIONS;
  const inline = collapse ? buttons.slice(0, MAX_INLINE_ROW_ACTIONS - 1) : buttons;
  const folded = collapse ? buttons.slice(MAX_INLINE_ROW_ACTIONS - 1) : [];
  return (
    <span className="resource-table__actions">
      {nodes.map((action) => (
        <span key={action.key}>{action.node}</span>
      ))}
      {inline.map((action) =>
        action.href !== undefined ? (
          <RowLink
            key={action.key}
            to={action.href}
            title={action.title}
            danger={action.danger}
            variant="button"
          />
        ) : (
          <ActionButton
            key={action.key}
            title={action.title}
            danger={action.danger}
            onClick={action.onClick}
          />
        ),
      )}
      {folded.length > 0 ? (
        <Dropdown
          trigger={["click"]}
          menu={{
            items: folded.map((action) => ({
              key: action.key,
              label:
                action.href !== undefined ? (
                  <RowLink to={action.href} title={action.title} variant="text" />
                ) : (
                  action.title
                ),
              danger: action.danger,
            })),
            // Link items navigate through their anchor.
            onClick: ({ key }) => {
              const action = folded.find((item) => item.key === key);
              if (action?.href === undefined) action?.onClick();
            },
          }}
        >
          <Button type="link" size="small" aria-label={moreLabel} icon={<EllipsisOutlined />} />
        </Dropdown>
      ) : null}
    </span>
  );
}

/** 列表成功提示与失败弹窗；在 antd App 之外渲染时回退到静态方法。 */
function useTableFeedback() {
  const { message, modal } = useAppFeedback();
  return useMemo(
    () => ({
      success: (content: string) => void message.success(content),
      error: (content: string) => void message.error(content),
      blocking: (title: string, content: string) => void modal.error({ title, content }),
    }),
    [message, modal],
  );
}

type TableCreateDrawerProps<
  TSchema extends ResourceSchema,
  TEntity extends object,
  TKey extends ResourceKey,
  TFilters extends object,
  TCreateValues extends object,
  TUpdateValues extends object,
> = {
  readonly open: boolean;
  readonly onClose: () => void;
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
};

function CreateFormDrawer<
  TSchema extends ResourceSchema,
  TEntity extends object,
  TKey extends ResourceKey,
  TFilters extends object,
  TCreateValues extends object,
  TUpdateValues extends object,
>({
  open,
  onClose,
  schema,
  operations,
  config,
  title,
  reload,
}: TableCreateDrawerProps<
  TSchema,
  TEntity,
  TKey,
  TFilters,
  TCreateValues,
  TUpdateValues
>): ReactElement {
  const [loading, setLoading] = useState(false);
  const texts = useAdminTexts();
  const feedback = useTableFeedback();

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
      feedback.success(texts.feedback.created);
      onClose();
      await reload();
    } catch (cause) {
      // 抽屉不关、表单内容保留；原因写在提示里。
      feedback.error(formatErrorMessage(cause, texts.feedback.cannotSave));
    } finally {
      setLoading(false);
    }
  };

  return (
    <ResourceFormDrawer open={open} title={config.title ?? title} onClose={onClose}>
      {(actionsContainer) => (
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
          onCancel={onClose}
          actionsContainer={actionsContainer}
        />
      )}
    </ResourceFormDrawer>
  );
}

type TableEditDrawerProps<
  TSchema extends ResourceSchema,
  TEntity extends object,
  TKey extends ResourceKey,
  TFilters extends object,
  TCreateValues extends object,
  TUpdateValues extends object,
> = {
  readonly target: { readonly entity: TEntity; readonly key: TKey } | undefined;
  readonly onClose: () => void;
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
  readonly title: ReactNode;
  readonly reload: () => Promise<void>;
};

function EditFormDrawer<
  TSchema extends ResourceSchema,
  TEntity extends object,
  TKey extends ResourceKey,
  TFilters extends object,
  TCreateValues extends object,
  TUpdateValues extends object,
>({
  target,
  onClose,
  schema,
  operations,
  config,
  title,
  reload,
}: TableEditDrawerProps<
  TSchema,
  TEntity,
  TKey,
  TFilters,
  TCreateValues,
  TUpdateValues
>): ReactElement {
  const [loading, setLoading] = useState(false);
  const texts = useAdminTexts();
  const feedback = useTableFeedback();
  const initialValues = useMemo(
    () =>
      target === undefined
        ? undefined
        : ({
            ...Object.fromEntries(
              schema.fields
                .filter((field) => field.updatable && !field.serverManaged)
                .map((field) => [field.name, fieldValue(target.entity, field.name)]),
            ),
            ...config.initialValues?.(target.entity),
          } as Readonly<Partial<ResourceUpdateFormValues<TSchema, TUpdateValues>>>),
    [config, schema.fields, target],
  );

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
      feedback.success(texts.feedback.saved);
      onClose();
      await reload();
    } catch (cause) {
      feedback.error(formatErrorMessage(cause, texts.feedback.cannotSave));
    } finally {
      setLoading(false);
    }
  };

  return (
    <ResourceFormDrawer open={target !== undefined} title={config.title ?? title} onClose={onClose}>
      {(actionsContainer) =>
        target === undefined ? null : (
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
            key={display(target.key)}
            schema={schema}
            mode="update"
            rowKey={target.key}
            initialValues={initialValues}
            relationLoaders={config.relationLoaders}
            itemOverrides={config.itemOverrides}
            loading={loading}
            onUpdate={submit}
            onCancel={onClose}
            actionsContainer={actionsContainer}
          />
        )
      }
    </ResourceFormDrawer>
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

/** 表格默认每页条数（见 DESIGN.md「导航」：表格 20）。 */
export const DEFAULT_RESOURCE_PAGE_SIZE = 20;

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
    emptyText,
    reload: externalReload,
  } = props;
  const isReadOnly = props.readOnly === true;
  const texts = useAdminTexts();
  const feedback = useTableFeedback();
  const syncUrl = queryConfig?.syncUrl !== false && treeData === undefined;

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

  const sortableNames = useMemo(
    () =>
      schema.fields
        .filter((field) => {
          const override =
            columnOverrides?.[field.name as keyof ResourceColumnOverrides<TSchema, TEntity>];
          return field.sortable && override?.sortable !== false;
        })
        .map((field) => field.name),
    [columnOverrides, schema.fields],
  );

  // 占位文字写清搜索范围（见 DESIGN.md「数据列表」）：用可搜索字段的列标题，例如「用户名、昵称」。
  // 有字段没有中文标题、或字段太多时，用默认的「关键词」。
  const searchPlaceholder = useMemo(() => {
    const labels = schema.fields
      .filter((field) => {
        const override =
          columnOverrides?.[field.name as keyof ResourceColumnOverrides<TSchema, TEntity>];
        return field.searchable && override?.searchable !== false;
      })
      .map((field) => {
        const title =
          columnOverrides?.[field.name as keyof ResourceColumnOverrides<TSchema, TEntity>]?.title;
        return typeof title === "string" ? title : field.label;
      });
    if (labels.length === 0 || labels.length > 3) return undefined;
    if (labels.some((label) => label === undefined || label === "")) return undefined;
    return labels.join("、");
  }, [columnOverrides, schema.fields]);

  const defaultQuery = useMemo<TQuery>(() => {
    const defaults = queryConfig?.defaults ?? {};
    return {
      pageNum: defaults.pageNum ?? 1,
      pageSize: defaults.pageSize ?? DEFAULT_RESOURCE_PAGE_SIZE,
      ...(defaults.orders !== undefined ? { orders: defaults.orders } : {}),
    } as TQuery;
  }, [queryConfig]);

  const initialQuery = useMemo<TQuery>(() => {
    const defaults = queryConfig?.defaults ?? {};
    const initial = queryConfig?.initial ?? {};
    const fromUrl = syncUrl ? readUrlQuery() : {};
    const urlOrder =
      fromUrl.sort !== undefined && sortableNames.includes(fromUrl.sort.field)
        ? [{ field: fromUrl.sort.field as TOrderField, desc: fromUrl.sort.desc }]
        : undefined;
    const keyword = fromUrl.keyword ?? initial.keyword ?? defaults.keyword;
    const orders = urlOrder ?? initial.orders ?? defaults.orders;
    const filters = initial.filters ?? defaults.filters;
    return {
      pageNum: fromUrl.pageNum ?? initial.pageNum ?? defaults.pageNum ?? 1,
      pageSize:
        fromUrl.pageSize ?? initial.pageSize ?? defaults.pageSize ?? DEFAULT_RESOURCE_PAGE_SIZE,
      ...(keyword !== undefined ? { keyword } : {}),
      ...(orders !== undefined ? { orders } : {}),
      ...(filters !== undefined ? { filters } : {}),
    } as TQuery;
    // The URL is read once per query configuration; later changes are written by load.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [queryConfig]);

  const [currentQuery, setCurrentQuery] = useState<TQuery>(initialQuery);
  const [keyword, setKeyword] = useState(initialQuery.keyword ?? "");
  const surfaceStyle = useResourceSurfaceStyle();
  const [rows, setRows] = useState<readonly TEntity[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ResourceOperationError | undefined>();
  const [selectedKeys, setSelectedKeys] = useState<readonly TKey[]>([]);
  const [pending, setPending] = useState<PendingConfirmation<TKey> | undefined>();
  const [deleting, setDeleting] = useState(false);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<{ entity: TEntity; key: TKey } | undefined>();
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
      if (syncUrl) {
        writeUrlQuery(
          nextQuery as ResourceListQuery<object, string>,
          defaultQuery as ResourceListQuery<object, string>,
        );
      }
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
    [canList, defaultQuery, onError, operations, permissions.list, queryConfig, syncUrl, treeData],
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
          ...(override?.numeric === true
            ? { align: "right", className: "resource-table__numeric" }
            : {}),
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

  const search = (nextKeyword: string = keyword): void => {
    runQuery({ ...currentQuery, pageNum: 1, keyword: nextKeyword });
  };

  const clearFilters = (): void => {
    setKeyword("");
    search("");
  };

  const confirmPending = async (): Promise<void> => {
    const action = pending;
    if (action === undefined) return;
    if (!("delete" in operations) || !("batchDelete" in operations)) return;
    setDeleting(true);
    try {
      if (action.kind === "delete") {
        await operations.delete(action.keys[0]);
      } else {
        await operations.batchDelete(action.keys);
      }
      if (action.kind === "batchDelete") {
        setSelectedKeys([]);
      }
      setPending(undefined);
      feedback.success(
        action.kind === "batchDelete"
          ? texts.feedback.batchDeleted(action.keys.length)
          : texts.feedback.deleted,
      );
      await externalReload?.();
      await reload();
    } catch (cause) {
      // Mutation failures are not list-load failures: keep the current rows
      // visible and explain why in a dialog the user has to acknowledge
      // (for example a role that is still assigned to users).
      setPending(undefined);
      feedback.blocking(texts.feedback.cannotDelete, formatErrorMessage(cause));
    } finally {
      setDeleting(false);
    }
  };

  const toolbarContext = { selectedKeys, rows, reload, clearSelection };
  const canCreateWithForm =
    !isReadOnly && canCreate && "create" in operations && form?.createConfig !== undefined;
  const createOverride = toolbarOverrides?.find(
    (item): item is ResourceStandardToolbarOverride<TEntity, TKey> => item.action === "create",
  );
  const createTitle = createOverride?.title ?? texts.action.create;

  const renderCreate = (): ReactNode => {
    if (isReadOnly || !canCreate || !("create" in operations)) return null;
    if (createOverride?.merge === "hide") return null;
    if (createOverride?.render) {
      return createOverride.render(toolbarContext);
    }
    if (form?.createConfig) {
      return (
        <ActionButton title={createTitle} buttonType="primary" onClick={() => setCreating(true)} />
      );
    }
    if (form?.create) {
      return form.create({ schema, operations, reload });
    }
    // No way to create from this page: hide the action instead of showing a dead button.
    return null;
  };

  const customToolbar = (toolbarOverrides ?? []).filter(
    (override): override is Extract<typeof override, { action: "custom" }> =>
      override.action === "custom" &&
      access.can(override.permission) &&
      override.hidden !== true &&
      !(typeof override.hidden === "function" && override.hidden(toolbarContext)) &&
      override.render !== undefined,
  );

  const renderToolbar = (): ReactNode[] => {
    const items: ReactNode[] = [];
    const create = renderCreate();
    if (create !== null) items.push(<span key="create">{create}</span>);
    for (const override of customToolbar) {
      if (override.placement === "selection") continue;
      items.push(<span key={override.name}>{override.render?.(toolbarContext)}</span>);
    }
    return items;
  };

  const batchOverride = toolbarOverrides?.find(
    (item): item is ResourceStandardToolbarOverride<TEntity, TKey> => item.action === "batchDelete",
  );
  const showBatchDelete = canBatchDelete && batchOverride?.merge !== "hide";
  const selectionActions: ReactNode[] = [];
  if (showBatchDelete) {
    selectionActions.push(
      batchOverride?.render ? (
        <span key="batch-delete">{batchOverride.render(toolbarContext)}</span>
      ) : (
        <ActionButton
          key="batch-delete"
          title={batchOverride?.title ?? texts.action.delete}
          buttonType="default"
          danger
          onClick={() => setPending({ kind: "batchDelete", keys: selectedKeys })}
        />
      ),
    );
  }
  for (const override of customToolbar) {
    if (override.placement !== "selection") continue;
    selectionActions.push(<span key={override.name}>{override.render?.(toolbarContext)}</span>);
  }
  // Checkboxes only make sense when something can be done with the selection.
  // Custom toolbar renderers still receive selectedKeys, so keep selection for them.
  const selectable =
    selectionActions.length > 0 ||
    customToolbar.some((override) => override.placement !== "selection");

  const buildRowActions = (entity: TEntity, index: number): RowAction[] => {
    const key = getRowKey(entity, index);
    const actions: RowAction[] = [];
    const updateOverride = findRowActionOverride("update");
    if (!isReadOnly && "update" in operations && canUpdate && updateOverride?.merge !== "hide") {
      const title = updateOverride?.title ?? texts.action.edit;
      if (updateOverride?.onClick) {
        actions.push({
          key: "update",
          kind: "button",
          title,
          onClick: () => void updateOverride.onClick?.({ entity, key, selectedKeys, reload }),
        });
      } else if (form?.editConfig) {
        actions.push({
          key: "update",
          kind: "button",
          title,
          onClick: () => setEditing({ entity, key }),
        });
      } else if (form?.edit) {
        actions.push({
          key: "update",
          kind: "node",
          node: form.edit({ schema, entity, key, operations, reload }),
        });
      }
    }

    const dangerous: RowAction[] = [];
    for (const override of rowActionOverrides ?? []) {
      if (override.action !== "custom") continue;
      if (!access.can(override.permission, { entity, key })) continue;
      if (override.hidden === true) continue;
      const context = { entity, key, selectedKeys, reload };
      if (typeof override.hidden === "function" && override.hidden(context)) continue;
      const action: RowAction = override.render
        ? { key: override.name, kind: "node", node: override.render(context) }
        : {
            key: override.name,
            kind: "button",
            title: override.title ?? override.name,
            danger: override.danger,
            href: override.href?.(context),
            onClick: () => void override.onClick?.(context),
          };
      (override.danger ? dangerous : actions).push(action);
    }

    const deleteOverride = findRowActionOverride("delete");
    if (!isReadOnly && "delete" in operations && canDelete && deleteOverride?.merge !== "hide") {
      dangerous.push({
        key: "delete",
        kind: "button",
        title: deleteOverride?.title ?? texts.action.delete,
        danger: true,
        onClick: () => {
          if (deleteOverride?.onClick) {
            void deleteOverride.onClick({ entity, key, selectedKeys, reload });
          } else {
            setPending({ kind: "delete", keys: [key] });
          }
        },
      });
    }
    // Dangerous actions go last in the group.
    return [...actions, ...dangerous];
  };

  const actionColumn: ProColumns<TEntity & Record<string, unknown>> | undefined =
    canUpdate || canDelete || (rowActionOverrides ?? []).some((item) => item.action === "custom")
      ? {
          key: "actions",
          title: texts.table.actionColumn,
          render: (_value, entity, index) => (
            <RowActions
              actions={buildRowActions(entity as TEntity, index)}
              moreLabel={texts.action.more}
            />
          ),
        }
      : undefined;

  const errorContent = error
    ? (renderError?.(error, reload) ?? (
        <div role="alert" className="resource-table__error">
          {formatErrorMessage(error, texts.feedback.cannotLoad)}
          <Button type="link" onClick={() => void reload()}>
            {texts.action.retry}
          </Button>
        </div>
      ))
    : null;

  const filtered = (currentQuery.keyword ?? "") !== "";
  const emptyContent = loading ? (
    texts.table.loading
  ) : emptyText !== undefined ? (
    emptyText
  ) : (
    <Empty
      image={Empty.PRESENTED_IMAGE_SIMPLE}
      description={filtered ? texts.table.noMatch : texts.table.empty}
    >
      {filtered ? (
        <Button onClick={clearFilters}>{texts.action.clearFilters}</Button>
      ) : canCreateWithForm && createOverride?.merge !== "hide" ? (
        <Button type="primary" onClick={() => setCreating(true)}>
          {createTitle}
        </Button>
      ) : null}
    </Empty>
  );

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

  const activeOrder = currentQuery.orders?.[0];
  const columnsWithSortState = (actionColumn ? [...proColumns, actionColumn] : proColumns).map(
    (column) =>
      column.key !== undefined && column.key === activeOrder?.field
        ? { ...column, sortOrder: activeOrder.desc ? "descend" : "ascend" }
        : column.sorter
          ? { ...column, sortOrder: null }
          : column,
  ) as ProColumns<TEntity & Record<string, unknown>>[];

  return (
    <div data-resource-table={schema.name} className="resource-table">
      <ResourceSearchBar
        keyword={keyword}
        placeholder={searchPlaceholder}
        onKeywordChange={setKeyword}
        onSearch={(next) => search(next)}
        onReload={() => void reload()}
      />
      <div className="resource-table__content" style={surfaceStyle}>
        {errorContent ?? (
          <ProTable<TEntity & Record<string, unknown>, TQuery>
            columns={columnsWithSortState}
            // ProTable row identity and controlled selection share the same UI key.
            // The selectedKeys state retains the original typed operation keys.
            rowKey={(entity) => display(getRowKey(entity as TEntity))}
            rowSelection={
              selectable
                ? {
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
                  }
                : undefined
            }
            tableAlertRender={false}
            search={false}
            options={false}
            sortDirections={["ascend", "descend", null]}
            onChange={onProTableChange}
            toolBarRender={() => renderToolbar()}
            dataSource={rows as readonly (TEntity & Record<string, unknown>)[]}
            loading={loading}
            locale={{ emptyText: emptyContent }}
            pagination={
              treeData
                ? false
                : {
                    current: currentQuery.pageNum,
                    pageSize: currentQuery.pageSize,
                    total,
                    hideOnSinglePage: true,
                    showSizeChanger: true,
                    showQuickJumper: total > currentQuery.pageSize * 5,
                  }
            }
          />
        )}
        {selectedKeys.length > 0 && selectionActions.length > 0 ? (
          <div className="resource-table__selection" role="toolbar">
            <Space>
              <span>{texts.table.selected(selectedKeys.length)}</span>
              <Button type="link" onClick={clearSelection}>
                {texts.action.clearSelection}
              </Button>
              {selectionActions}
            </Space>
          </div>
        ) : null}
      </div>
      {!isReadOnly && "create" in operations && form?.createConfig ? (
        <CreateFormDrawer
          open={creating}
          onClose={() => setCreating(false)}
          schema={schema}
          operations={operations}
          config={form.createConfig}
          title={createTitle}
          reload={reload}
        />
      ) : null}
      {!isReadOnly && "update" in operations && form?.editConfig ? (
        <EditFormDrawer
          target={editing}
          onClose={() => setEditing(undefined)}
          schema={schema}
          operations={operations}
          config={form.editConfig}
          title={findRowActionOverride("update")?.title ?? texts.action.edit}
          reload={reload}
        />
      ) : null}
      {pending ? (
        <Modal
          open
          title={
            pending.kind === "batchDelete"
              ? texts.confirm.batchDeleteTitle(pending.keys.length)
              : texts.confirm.deleteTitle
          }
          onCancel={() => setPending(undefined)}
          footer={[
            <Button key="cancel" disabled={deleting} onClick={() => setPending(undefined)}>
              {texts.action.cancel}
            </Button>,
            <Button
              key="confirm"
              type="primary"
              danger
              loading={deleting}
              onClick={() => void confirmPending()}
            >
              {texts.action.delete}
            </Button>,
          ]}
        >
          <p>{texts.confirm.deleteContent}</p>
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
    return (
      <AdminButtonSpacing>
        <ProductionResourceTable {...props} />
      </AdminButtonSpacing>
    );
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
