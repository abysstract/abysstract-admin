import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { create } from "@bufbuild/protobuf";
import type {
  CreateDictDataRequest,
  DictData,
  ListDictDataRequest,
} from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { DictDataSchema } from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { dictDataSchema } from "../../gen/resource/dict-data.schema.gen";
import { ResourceTable } from "./ResourceTable";
import type {
  ResourceAccess,
  ResourceColumnOverrides,
  ResourceRowActionOverride,
  ResourceToolbarOverride,
} from "./metadata";
import {
  toProTableTotal,
  type ResourceReadOperations,
  type ResourceOperations,
  type ResourceOperationError,
} from "./operations";
import type { ResourceTableProductionProps, ResourceTableReadOnlyProps } from "./table-contract";
import type { SortableFieldName } from "./schema";

/**
 * ResourceTable operations: this is deliberately production-only. The rows-only
 * overload is covered by ResourceTable.test.tsx and must not satisfy these assertions.
 * The production branch is covered here; the rows-only overload remains a compatibility path.
 */
type DictDataFilters = Pick<ListDictDataRequest, "dictType">;
type DictDataOrderField = SortableFieldName<typeof dictDataSchema>;
type DictDataCreateValues = CreateDictDataRequest;
type DictDataUpdateValues = {
  dictSort: number;
  dictLabel: string;
  dictValue: string;
  isDefault: number;
  remark: string;
};

const largeKey = 9007199254740993n;
const row = create(DictDataSchema, {
  id: largeKey,
  dictSort: 1,
  dictLabel: "启用",
  dictValue: "enabled",
  dictType: "notice",
  cssClass: "",
  listClass: "primary",
  isDefault: 0,
  remark: "",
});

function makeOperations(
  list: ResourceOperations<
    DictData,
    bigint,
    DictDataFilters,
    DictDataCreateValues,
    DictDataUpdateValues,
    DictData,
    void,
    DictDataOrderField
  >["list"],
): ResourceOperations<
  DictData,
  bigint,
  DictDataFilters,
  DictDataCreateValues,
  DictDataUpdateValues,
  DictData,
  void,
  DictDataOrderField
> {
  return {
    keyOf: (entity) => entity.id,
    list,
    get: vi.fn(async () => row),
    create: vi.fn(async () => row),
    update: vi.fn(async () => row),
    delete: vi.fn(async () => undefined),
    batchDelete: vi.fn(async () => undefined),
  };
}

function accessThatAllowsEverything(): ResourceAccess<DictData> {
  return { can: () => true };
}

function productionProps(
  operations: ResourceOperations<
    DictData,
    bigint,
    DictDataFilters,
    DictDataCreateValues,
    DictDataUpdateValues,
    DictData,
    void,
    DictDataOrderField
  >,
  access: ResourceAccess<DictData> = accessThatAllowsEverything(),
): ResourceTableProductionProps<
  typeof dictDataSchema,
  DictData,
  bigint,
  DictDataFilters,
  DictDataCreateValues,
  DictDataUpdateValues
> {
  const columns: ResourceColumnOverrides<typeof dictDataSchema, DictData> = {
    dictLabel: { title: "标签", render: ({ value }) => String(value) },
  };
  const rowActions: readonly ResourceRowActionOverride<DictData, bigint>[] = [
    { action: "delete", merge: "replace", title: "删除此行" },
    { action: "update", merge: "hide" },
    {
      action: "custom",
      name: "export",
      merge: "append",
      permission: "system:dict:export",
      title: "导出",
    },
  ];
  const toolbar: readonly ResourceToolbarOverride<DictData, bigint>[] = [
    { action: "batchDelete", merge: "replace", title: "批量删除" },
    {
      action: "custom",
      name: "export",
      merge: "append",
      permission: "system:dict:export",
      title: "导出",
    },
  ];
  return {
    schema: dictDataSchema,
    reload: async () => undefined,
    operations,
    access,
    columns,
    rowActions,
    toolbar,
    query: {
      defaults: {
        pageNum: 2,
        pageSize: 25,
        keyword: "notice",
        orders: [{ field: "dictSort", desc: true }],
        filters: { dictType: "notice" },
      },
    },
    renderError: (error: ResourceOperationError, retry) => (
      <div role="alert">
        {error.message}
        <button type="button" onClick={() => void retry()}>
          重试
        </button>
      </div>
    ),
  };
}

describe("ResourceTable production behavior failure gates", () => {
  it("keeps list content visible when a delete mutation fails", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues,
          DictData,
          void,
          DictDataOrderField
        >["list"]
      >()
      .mockResolvedValue({
        items: [row],
        total: 1n,
        pageNum: 1,
        pageSize: 10,
      });
    const deleteOperation = vi
      .fn<
        ResourceOperations<
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues,
          DictData,
          void,
          DictDataOrderField
        >["delete"]
      >()
      .mockRejectedValue(new Error("超级管理员角色不可删除"));
    const operations = { ...makeOperations(list), delete: deleteOperation };
    const onError = vi.fn();

    render(<ResourceTable {...productionProps(operations)} onError={onError} />);
    await screen.findByText("启用");

    fireEvent.click(screen.getByRole("button", { name: "删除此行" }));
    fireEvent.click(screen.getByRole("button", { name: /确认.*删除/ }));
    await waitFor(() => expect(onError).toHaveBeenCalledWith(expect.any(Error)));

    expect(screen.getByText("启用")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(list).toHaveBeenCalledOnce();
  });

  it("does not let an older list response overwrite a newer query", async () => {
    type Result = { items: readonly DictData[]; total: bigint; pageNum: number; pageSize: number };
    let resolveFirst: (result: Result) => void = () => undefined;
    let resolveSecond: (result: Result) => void = () => undefined;
    const first = new Promise<Result>((resolve) => {
      resolveFirst = resolve;
    });
    const second = new Promise<Result>((resolve) => {
      resolveSecond = resolve;
    });
    const freshRow = { ...row, dictLabel: "新查询" };
    const oldRow = { ...row, dictLabel: "旧查询" };
    const list = vi.fn(async (query: { keyword?: string }) =>
      query.keyword === "fresh" ? second : first,
    );
    const operations = makeOperations(list);

    render(<ResourceTable {...productionProps(operations)} />);
    await waitFor(() => expect(list).toHaveBeenCalledOnce());

    fireEvent.change(screen.getByRole("search").querySelector("input")!, {
      target: { value: "fresh" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));

    resolveSecond({ items: [freshRow], total: 1n, pageNum: 1, pageSize: 10 });
    expect(await screen.findByText("新查询")).toBeInTheDocument();
    resolveFirst({ items: [oldRow], total: 1n, pageNum: 2, pageSize: 25 });
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByText("旧查询")).not.toBeInTheDocument();
    expect(screen.getByText("新查询")).toBeInTheDocument();
  });

  it("does not show an error when an older request rejects after a newer response lands", async () => {
    type Result = { items: readonly DictData[]; total: bigint; pageNum: number; pageSize: number };
    let rejectFirst: (cause: Error) => void = () => undefined;
    let resolveSecond: (result: Result) => void = () => undefined;
    const first = new Promise<Result>((_resolve, reject) => {
      rejectFirst = reject;
    });
    const second = new Promise<Result>((resolve) => {
      resolveSecond = resolve;
    });
    const freshRow = { ...row, dictLabel: "新查询" };
    const list = vi.fn(async (query: { keyword?: string }) =>
      query.keyword === "fresh" ? second : first,
    );
    const operations = makeOperations(list);

    render(<ResourceTable {...productionProps(operations)} />);
    await waitFor(() => expect(list).toHaveBeenCalledOnce());

    fireEvent.change(screen.getByRole("search").querySelector("input")!, {
      target: { value: "fresh" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));

    resolveSecond({ items: [freshRow], total: 1n, pageNum: 1, pageSize: 10 });
    expect(await screen.findByText("新查询")).toBeInTheDocument();
    rejectFirst(new Error("旧请求失败"));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByText("新查询")).toBeInTheDocument();
  });

  it("does not let an older resolved request overwrite a newer error", async () => {
    type Result = { items: readonly DictData[]; total: bigint; pageNum: number; pageSize: number };
    let resolveFirst: (result: Result) => void = () => undefined;
    let rejectSecond: (cause: Error) => void = () => undefined;
    const first = new Promise<Result>((resolve) => {
      resolveFirst = resolve;
    });
    const second = new Promise<Result>((_resolve, reject) => {
      rejectSecond = reject;
    });
    const oldRow = { ...row, dictLabel: "旧行数据" };
    const list = vi.fn(async (query: { keyword?: string }) =>
      query.keyword === "fresh" ? second : first,
    );
    const operations = makeOperations(list);

    render(<ResourceTable {...productionProps(operations)} />);
    await waitFor(() => expect(list).toHaveBeenCalledOnce());

    fireEvent.change(screen.getByRole("search").querySelector("input")!, {
      target: { value: "fresh" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));

    rejectSecond(new Error("最新请求失败"));
    expect(await screen.findByRole("alert")).toHaveTextContent("最新请求失败");
    resolveFirst({ items: [oldRow], total: 1n, pageNum: 1, pageSize: 10 });
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.getByRole("alert")).toHaveTextContent("最新请求失败");
    expect(screen.queryByText("旧行数据")).not.toBeInTheDocument();
  });

  it("keeps loading until the latest request settles even when an older request completes first", async () => {
    type Result = { items: readonly DictData[]; total: bigint; pageNum: number; pageSize: number };
    let resolveFirst: (result: Result) => void = () => undefined;
    let resolveSecond: (result: Result) => void = () => undefined;
    const first = new Promise<Result>((resolve) => {
      resolveFirst = resolve;
    });
    const second = new Promise<Result>((resolve) => {
      resolveSecond = resolve;
    });
    const freshRow = { ...row, dictLabel: "最新行" };
    const list = vi.fn(async (query: { keyword?: string }) =>
      query.keyword === "fresh" ? second : first,
    );
    const operations = makeOperations(list);

    render(<ResourceTable {...productionProps(operations)} />);
    await waitFor(() => expect(list).toHaveBeenCalledOnce());

    fireEvent.change(screen.getByRole("search").querySelector("input")!, {
      target: { value: "fresh" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));

    // The stale initial request settles first; loading must stay active
    // because the latest request is still pending.
    resolveFirst({
      items: [{ ...row, dictLabel: "旧行" }],
      total: 1n,
      pageNum: 2,
      pageSize: 25,
    });
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByText("旧行")).not.toBeInTheDocument();
    // 旧请求完成不得在最新请求仍 pending 时提前结束 loading。
    expect(screen.getByRole("status")).toBeInTheDocument();

    resolveSecond({ items: [freshRow], total: 1n, pageNum: 1, pageSize: 10 });
    expect(await screen.findByText("最新行")).toBeInTheDocument();
    await waitFor(() => expect(screen.queryByRole("status")).toBeNull());
  });

  it("binds list query, search, page, schema sort, reload, and bigint row keys", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues,
          DictData,
          void,
          DictDataOrderField
        >["list"]
      >()
      .mockResolvedValue({
        items: [row],
        total: 1n,
        pageNum: 2,
        pageSize: 25,
      });
    const operations = makeOperations(list);

    render(<ResourceTable {...productionProps(operations)} />);

    await waitFor(() => {
      expect(list).toHaveBeenCalledWith({
        pageNum: 2,
        pageSize: 25,
        keyword: "notice",
        orders: [{ field: "dictSort", desc: true }],
        filters: { dictType: "notice" },
      });
    });
    expect(screen.getByText(largeKey.toString())).toBeInTheDocument();
    expect(typeof operations.keyOf(row)).toBe("bigint");

    fireEvent.change(screen.getByRole("textbox", { name: /keyword|关键词/i }), {
      target: { value: "new keyword" },
    });
    fireEvent.click(screen.getByRole("button", { name: /查询|search/i }));
    expect(list).toHaveBeenLastCalledWith(
      expect.objectContaining({ keyword: "new keyword", pageNum: 1 }),
    );

    fireEvent.click(screen.getByRole("button", { name: /下一页|next/i }));
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ pageNum: 2 }));
    fireEvent.click(screen.getByRole("button", { name: /刷新|reload/i }));
    expect(list).toHaveBeenCalledTimes(4);
  });

  it("supports row selection, single delete, and typed batchDelete without narrowing bigint keys", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues,
          DictData,
          void,
          DictDataOrderField
        >["list"]
      >()
      .mockResolvedValue({
        items: [row],
        total: 1n,
        pageNum: 1,
        pageSize: 10,
      });
    const operations = makeOperations(list);

    render(<ResourceTable {...productionProps(operations)} />);
    await screen.findByText("启用");

    fireEvent.click(screen.getByRole("checkbox", { name: /启用|选择/i }));
    fireEvent.click(screen.getByRole("button", { name: "删除此行" }));
    expect(screen.getByRole("dialog")).toHaveTextContent(/确认.*删除/);
    fireEvent.click(screen.getByRole("button", { name: /确认.*删除/ }));
    await waitFor(() => expect(operations.delete).toHaveBeenCalledWith(largeKey));

    fireEvent.click(screen.getByRole("button", { name: "批量删除" }));
    expect(screen.getByRole("dialog")).toHaveTextContent(/确认.*批量.*删除/);
    fireEvent.click(screen.getByRole("button", { name: /确认.*批量.*删除/ }));
    await waitFor(() => expect(operations.batchDelete).toHaveBeenCalledWith([largeKey]));
  });

  it("uses the generated schema permission for Get when it differs from the default", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues,
          DictData,
          void,
          DictDataOrderField
        >["list"]
      >()
      .mockResolvedValue({ items: [row], total: 1n, pageNum: 1, pageSize: 10 });
    const access = vi.fn(() => true);
    const operations = makeOperations(list);
    const schemaWithLegacyGet = {
      ...dictDataSchema,
      permissions: { ...dictDataSchema.permissions, get: "system:dict:query" },
    } as unknown as typeof dictDataSchema;

    render(
      <ResourceTable
        {...productionProps(operations, { can: access })}
        schema={schemaWithLegacyGet}
        rowActions={[]}
        toolbar={[]}
      />,
    );
    await screen.findByText("启用");
    expect(access).toHaveBeenCalledWith("system:dict:query");
  });

  it("applies fixed standard permissions and typed column/action/toolbar overrides", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues,
          DictData,
          void,
          DictDataOrderField
        >["list"]
      >()
      .mockResolvedValue({
        items: [row],
        total: 1n,
        pageNum: 1,
        pageSize: 10,
      });
    const access = vi.fn((permission: string) => permission !== "system:dict:export");
    const operations = makeOperations(list);
    const props = productionProps(operations, { can: access });

    const { rerender } = render(<ResourceTable {...props} rowActions={[]} toolbar={[]} />);
    await screen.findByText("启用");

    const checkedPermissions = access.mock.calls.map(([permission]) => permission);
    expect(checkedPermissions).toEqual(
      expect.arrayContaining([
        "system:dict:list",
        "system:dict:add",
        "system:dict:edit",
        "system:dict:remove",
      ]),
    );
    expect(screen.getByRole("columnheader", { name: "标签" })).toBeInTheDocument();

    const overriddenProps = productionProps(operations, { can: access });
    rerender(<ResourceTable {...overriddenProps} />);
    await screen.findByText("启用");
    const allCheckedPermissions = access.mock.calls.map(([permission]) => permission);
    expect(allCheckedPermissions).toContain("system:dict:export");
    expect(screen.queryByRole("button", { name: "导出" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "编辑" })).not.toBeInTheDocument();
  });

  it("renders typed custom row controls with the ResourceTable row context", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues,
          DictData,
          void,
          DictDataOrderField
        >["list"]
      >()
      .mockResolvedValue({
        items: [row],
        total: 1n,
        pageNum: 1,
        pageSize: 10,
      });
    const operations = makeOperations(list);

    render(
      <ResourceTable
        {...productionProps(operations)}
        rowActions={[
          {
            action: "custom",
            name: "typed-control",
            merge: "append",
            permission: "system:dict:edit",
            render: ({ entity, key }) => (
              <span data-testid="typed-row-control">{`${entity.dictLabel}:${key.toString()}`}</span>
            ),
          },
        ]}
      />,
    );

    await screen.findByText("启用");
    expect(screen.getByTestId("typed-row-control")).toHaveTextContent(`${row.dictLabel}:${row.id}`);
  });

  it("supports a read-only Partial surface without inventing standard CRUD actions", async () => {
    const list = vi
      .fn<ResourceReadOperations<DictData, bigint, DictDataFilters, DictDataOrderField>["list"]>()
      .mockResolvedValue({
        items: [row],
        total: 1n,
        pageNum: 1,
        pageSize: 10,
      });
    const operations: ResourceReadOperations<
      DictData,
      bigint,
      DictDataFilters,
      DictDataOrderField
    > = {
      keyOf: (entity) => entity.id,
      list,
    };
    const props: ResourceTableReadOnlyProps<
      typeof dictDataSchema,
      DictData,
      bigint,
      DictDataFilters
    > = {
      readOnly: true,
      schema: dictDataSchema,
      operations,
      access: accessThatAllowsEverything(),
      rowActions: [
        {
          action: "custom",
          name: "inspect",
          permission: "system:dict:inspect",
          merge: "append",
          render: () => <span data-testid="partial-action">查看</span>,
        },
      ],
      toolbar: [
        {
          action: "custom",
          name: "reload",
          permission: "system:dict:reload",
          merge: "append",
          render: () => <span data-testid="partial-toolbar">自定义工具</span>,
        },
      ],
    };

    render(<ResourceTable {...props} />);
    await screen.findByText("启用");

    expect(screen.getByTestId("partial-action")).toBeInTheDocument();
    expect(screen.getByTestId("partial-toolbar")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "编辑" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "删除" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "批量删除" })).not.toBeInTheDocument();
  });

  it("rejects unsafe bigint totals at the sole ProTable overflow boundary", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues,
          DictData,
          void,
          DictDataOrderField
        >["list"]
      >()
      .mockResolvedValue({ items: [row], total: largeKey, pageNum: 1, pageSize: 10 });
    const operations = makeOperations(list);

    expect(() => toProTableTotal(largeKey)).toThrow(/exceeds/i);
    expect(toProTableTotal(largeKey, "saturate")).toBe(Number.MAX_SAFE_INTEGER);
    render(<ResourceTable {...productionProps(operations)} />);

    expect(await screen.findByRole("alert")).toHaveTextContent(/exceeds|overflow|安全/i);
  });

  it("prefers update override onClick over the default edit Modal", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues,
          DictData,
          void,
          DictDataOrderField
        >["list"]
      >()
      .mockResolvedValue({
        items: [row],
        total: 1n,
        pageNum: 1,
        pageSize: 10,
      });
    const operations = makeOperations(list);
    const onClick = vi.fn(async () => undefined);
    const props: ResourceTableProductionProps<
      typeof dictDataSchema,
      DictData,
      bigint,
      DictDataFilters,
      DictDataCreateValues,
      DictDataUpdateValues
    > = {
      schema: dictDataSchema,
      reload: async () => undefined,
      operations,
      access: accessThatAllowsEverything(),
      rowActions: [
        { action: "update", merge: "replace", title: "编辑行", onClick },
        { action: "delete", merge: "hide" },
      ],
      toolbar: [],
      form: {
        editConfig: {
          title: "默认编辑 Modal",
          initialValues: (entity) => ({
            dictLabel: entity.dictLabel,
            remark: entity.remark,
          }),
          onSuccess: async () => undefined,
        },
      },
    };

    render(<ResourceTable {...props} />);
    await screen.findByText("启用");
    fireEvent.click(screen.getByRole("button", { name: "编辑行" }));
    await waitFor(() =>
      expect(onClick).toHaveBeenCalledWith(expect.objectContaining({ key: largeKey })),
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("keeps the default create title and lets a replace override title cover it", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues,
          DictData,
          void,
          DictDataOrderField
        >["list"]
      >()
      .mockResolvedValue({
        items: [row],
        total: 1n,
        pageNum: 1,
        pageSize: 10,
      });
    const operations = makeOperations(list);
    const base: ResourceTableProductionProps<
      typeof dictDataSchema,
      DictData,
      bigint,
      DictDataFilters,
      DictDataCreateValues,
      DictDataUpdateValues
    > = {
      schema: dictDataSchema,
      reload: async () => undefined,
      operations,
      access: accessThatAllowsEverything(),
      rowActions: [],
      toolbar: [],
      form: {
        createConfig: {
          initialValues: {},
          itemOverrides: {},
          onSuccess: async () => undefined,
        },
      },
    };

    const { unmount } = render(<ResourceTable {...base} />);
    await screen.findByText("启用");
    expect(screen.getByRole("button", { name: "新建" })).toBeInTheDocument();
    unmount();

    render(
      <ResourceTable
        {...base}
        toolbar={[{ action: "create", merge: "replace", title: "新建参数" }]}
      />,
    );
    await screen.findByText("启用");
    fireEvent.click(screen.getByRole("button", { name: "新建参数" }));
    expect(screen.getByRole("dialog", { name: "新建参数" })).toBeInTheDocument();
  });

  it("separates search and result surfaces like the legacy ProTable card layout", async () => {
    const list = vi
      .fn<
        ResourceOperations<
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues,
          DictData,
          void,
          DictDataOrderField
        >["list"]
      >()
      .mockResolvedValue({
        items: [row],
        total: 1n,
        pageNum: 1,
        pageSize: 10,
      });

    render(<ResourceTable {...productionProps(makeOperations(list))} />);
    await screen.findByText("启用");

    const root = document.querySelector('[data-resource-table="DictData"]');
    expect(root?.querySelector(".resource-table__search")).toBeInTheDocument();
    expect(root?.querySelector(".resource-table__content")).toBeInTheDocument();
    expect(root?.querySelector('[role="search"]')?.parentElement).toHaveClass(
      "resource-table__search",
    );
  });

  it("renders operation errors through the production error boundary and retries", async () => {
    const error = new Error("ListPermissionDenied");
    const list = vi
      .fn<
        ResourceOperations<
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues,
          DictData,
          void,
          DictDataOrderField
        >["list"]
      >()
      .mockRejectedValue(error);
    const operations = makeOperations(list);

    render(<ResourceTable {...productionProps(operations)} />);
    expect(await screen.findByRole("alert")).toHaveTextContent("ListPermissionDenied");
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    expect(list).toHaveBeenCalledTimes(2);
  });
});

it("shares labels, fills editable fields by default and merges explicit edit values", async () => {
  const schema = {
    ...dictDataSchema,
    fields: dictDataSchema.fields.map((field) => ({
      ...field,
      label: field.name === "dictLabel" ? "标签" : field.name,
    })),
  };
  const operations = makeOperations(
    vi.fn(async () => ({ items: [row], total: 1n, pageNum: 1, pageSize: 20 })),
  );
  render(
    <ResourceTable
      schema={schema}
      operations={operations}
      access={accessThatAllowsEverything()}
      form={{ editConfig: { initialValues: () => ({ remark: "显式回填" }) } }}
    />,
  );
  await screen.findByRole("columnheader", { name: "标签" });
  fireEvent.click(screen.getByRole("button", { name: /编辑|update/i }));
  await waitFor(() => expect(screen.getByLabelText("标签")).toHaveValue(row.dictLabel));
  expect(screen.getByLabelText("remark")).toHaveValue("显式回填");
  expect(screen.queryByLabelText("dictType")).toBeNull();
  fireEvent.change(screen.getByLabelText("remark"), { target: { value: "" } });
  fireEvent.click(screen.getByRole("button", { name: "提交" }));
  await waitFor(() => expect(operations.update).toHaveBeenCalledOnce());
  const submission = vi.mocked(operations.update).mock.calls[0][0];
  expect(submission.dirty).toEqual(new Set(["remark"]));
  expect(submission.values.remark).toBe("");
});
