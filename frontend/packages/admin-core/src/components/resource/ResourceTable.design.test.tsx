import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  DictDataSchema,
  type CreateDictDataRequest,
  type DictData,
  type ListDictDataRequest,
} from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";

import { dictDataSchema } from "../../gen/resource/dict-data.schema.gen";
import { AdminTextsProvider } from "../../runtime/texts";
import { ResourceTable } from "./ResourceTable";
import type { ResourceRowActionOverride } from "./metadata";
import type { ResourceOperations } from "./operations";
import type { SortableFieldName } from "./schema";

// Behaviour required by DESIGN.md: page size, URL state, collapsed row actions,
// hidden actions without permission, and empty states that say why.

type Filters = Pick<ListDictDataRequest, "dictType">;
type OrderField = SortableFieldName<typeof dictDataSchema>;
type UpdateValues = {
  dictSort: number;
  dictLabel: string;
  dictValue: string;
  isDefault: number;
  remark: string;
};
type Operations = ResourceOperations<
  DictData,
  bigint,
  Filters,
  CreateDictDataRequest,
  UpdateValues,
  DictData,
  void,
  OrderField
>;

const row = create(DictDataSchema, { id: 1n, dictLabel: "启用", dictValue: "on", dictSort: 1 });

function operations(items: readonly DictData[] = [row]): Operations {
  return {
    keyOf: (entity) => entity.id,
    list: vi.fn(async () => ({ items, total: BigInt(items.length), pageNum: 1, pageSize: 20 })),
    get: vi.fn(async () => row),
    create: vi.fn(async () => row),
    update: vi.fn(async () => row),
    delete: vi.fn(async () => undefined),
    batchDelete: vi.fn(async () => undefined),
  };
}

function renderTable(
  ops: Operations,
  extra: {
    can?: (permission: string) => boolean;
    rowActions?: readonly ResourceRowActionOverride<DictData, bigint>[];
    withForms?: boolean;
  } = {},
) {
  return render(
    <ResourceTable<
      typeof dictDataSchema,
      DictData,
      bigint,
      Filters,
      CreateDictDataRequest,
      UpdateValues
    >
      schema={dictDataSchema}
      operations={ops}
      access={{ can: extra.can ?? (() => true) }}
      columns={{ dictLabel: { title: "标签", render: ({ value }) => String(value) } }}
      rowActions={extra.rowActions}
      form={
        extra.withForms === false
          ? undefined
          : { createConfig: { title: "新增字典数据" }, editConfig: { title: "编辑字典数据" } }
      }
    />,
  );
}

describe("ResourceTable design defaults", () => {
  it("loads 20 rows per page by default and writes the query into the URL", async () => {
    const ops = operations();
    renderTable(ops);
    await screen.findByText("启用");
    expect(ops.list).toHaveBeenCalledWith({ pageNum: 1, pageSize: 20 });
    expect(window.location.search).toBe("");

    fireEvent.change(screen.getByRole("textbox", { name: "关键词" }), {
      target: { value: "启" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));
    await waitFor(() =>
      expect(ops.list).toHaveBeenLastCalledWith({ pageNum: 1, pageSize: 20, keyword: "启" }),
    );
    expect(new URLSearchParams(window.location.search).get("q")).toBe("启");
  });

  it("restores keyword, page, size and sort from the URL", async () => {
    window.history.replaceState(null, "", "/?q=abc&page=3&size=50&sort=-dictSort");
    const ops = operations();
    renderTable(ops);
    await screen.findByText("启用");
    expect(ops.list).toHaveBeenCalledWith({
      pageNum: 3,
      pageSize: 50,
      keyword: "abc",
      orders: [{ field: "dictSort", desc: true }],
    });
    expect(screen.getByRole("textbox", { name: "关键词" })).toHaveValue("abc");
  });

  it("shows at most two row actions and folds the rest into ⋯ with delete last", async () => {
    const assign = vi.fn();
    renderTable(operations(), {
      rowActions: [
        {
          action: "custom",
          name: "assign",
          title: "分配",
          permission: "x",
          merge: "append",
          onClick: assign,
        },
      ],
    });
    await screen.findByText("启用");
    const more = screen.getByRole("button", { name: "更多" });
    // The Dropdown test double renders folded items next to the trigger.
    const folded = within(more.parentElement as HTMLElement).getAllByRole("button");
    expect(folded.map((button) => button.textContent)).toEqual(["", "分配", "删除"]);
    expect(screen.getByRole("button", { name: "编辑" })).toBeInTheDocument();
    fireEvent.click(
      within(more.parentElement as HTMLElement).getByRole("button", { name: "分配" }),
    );
    expect(assign).toHaveBeenCalledOnce();
  });

  it("hides actions the user cannot perform instead of disabling them", async () => {
    renderTable(operations(), {
      can: (permission) => permission.endsWith(":list") || permission.endsWith(":query"),
    });
    await screen.findByText("启用");
    expect(screen.queryByRole("button", { name: "新增" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "编辑" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "删除" })).not.toBeInTheDocument();
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
  });

  it("explains an empty list and offers the next step", async () => {
    const ops = operations([]);
    renderTable(ops);
    expect(await screen.findByText("还没有记录")).toBeInTheDocument();
    // Empty state create + toolbar create.
    expect(screen.getAllByRole("button", { name: "新增" })).toHaveLength(2);

    fireEvent.change(screen.getByRole("textbox", { name: "关键词" }), {
      target: { value: "nothing" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));
    expect(await screen.findByText("没有符合条件的记录")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "清除筛选" }));
    await waitFor(() =>
      expect(ops.list).toHaveBeenLastCalledWith({ pageNum: 1, pageSize: 20, keyword: "" }),
    );
  });

  it("opens create in a drawer with「取消 保存」in its header", async () => {
    renderTable(operations());
    await screen.findByText("启用");
    fireEvent.click(screen.getByRole("button", { name: "新增" }));
    const drawer = screen.getByRole("dialog", { name: "新增字典数据" });
    expect(within(drawer).getByRole("button", { name: "取消" })).toBeInTheDocument();
    expect(within(drawer).getByRole("button", { name: "保存" })).toBeInTheDocument();
  });

  it("lets the host replace interface copy", async () => {
    render(
      <AdminTextsProvider texts={{ action: { create: "Add" }, table: { empty: "Nothing yet" } }}>
        <ResourceTable<
          typeof dictDataSchema,
          DictData,
          bigint,
          Filters,
          CreateDictDataRequest,
          UpdateValues
        >
          schema={dictDataSchema}
          operations={operations([])}
          access={{ can: () => true }}
          form={{ createConfig: {} }}
        />
      </AdminTextsProvider>,
    );
    expect(await screen.findByText("Nothing yet")).toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Add" }).length).toBeGreaterThan(0);
    // Untouched groups keep the defaults.
    expect(screen.getByRole("button", { name: "查询" })).toBeInTheDocument();
  });
});

describe("ResourceTable structured filters in the URL", () => {
  function renderFiltered(ops: Operations) {
    return render(
      <ResourceTable<
        typeof dictDataSchema,
        DictData,
        bigint,
        Filters,
        CreateDictDataRequest,
        UpdateValues
      >
        schema={dictDataSchema}
        operations={ops}
        access={{ can: () => true }}
        filterControls={({ filters, setFilter }) => (
          <span>
            <span data-testid="current-type">{filters.dictType ?? "(all)"}</span>
            <button type="button" onClick={() => setFilter("dictType", "sys_status")}>
              只看状态
            </button>
            <button type="button" onClick={() => setFilter("dictType", undefined)}>
              全部类型
            </button>
          </span>
        )}
      />,
    );
  }

  it("writes filter changes to the URL and restarts from page 1", async () => {
    window.history.replaceState(null, "", "/?page=2");
    const ops = operations();
    renderFiltered(ops);
    await screen.findByText("启用");
    expect(ops.list).toHaveBeenLastCalledWith({ pageNum: 2, pageSize: 20 });

    fireEvent.click(screen.getByRole("button", { name: "只看状态" }));
    await waitFor(() =>
      expect(ops.list).toHaveBeenLastCalledWith({
        pageNum: 1,
        pageSize: 20,
        filters: { dictType: "sys_status" },
      }),
    );
    expect(window.location.search).toBe("?dictType=sys_status");
    expect(screen.getByTestId("current-type")).toHaveTextContent("sys_status");

    fireEvent.click(screen.getByRole("button", { name: "全部类型" }));
    await waitFor(() =>
      expect(ops.list).toHaveBeenLastCalledWith({ pageNum: 1, pageSize: 20, filters: {} }),
    );
    expect(window.location.search).toBe("");
  });

  it("restores filters from the URL, e.g. after refresh or in a new tab", async () => {
    window.history.replaceState(null, "", "/?dictType=sys_status&page=3");
    const ops = operations([]);
    renderFiltered(ops);
    await waitFor(() =>
      expect(ops.list).toHaveBeenCalledWith({
        pageNum: 3,
        pageSize: 20,
        filters: { dictType: "sys_status" },
      }),
    );
    expect(screen.getByTestId("current-type")).toHaveTextContent("sys_status");
    // Filters other than the defaults count as "filtered": the empty state offers to clear them.
    fireEvent.click(await screen.findByRole("button", { name: "清除筛选" }));
    await waitFor(() =>
      expect(ops.list).toHaveBeenLastCalledWith({ pageNum: 1, pageSize: 20, keyword: "" }),
    );
    expect(window.location.search).toBe("");
  });
});
