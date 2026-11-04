import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { message } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ProductSchema } from "../gen/proto/company/product/v1/product_pb";
import { productSchema } from "../gen/resource/product.schema.gen";

const facade = vi.hoisted(() => ({
  permissions: new Set<string>(),
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  delete: vi.fn(),
  batchDelete: vi.fn(),
  keyOf: vi.fn(),
  transport: {},
}));

vi.mock("@abysstract/admin-core", async () => {
  const antd = await import("antd");
  return {
    useAccess: () => ({ has: (permission: string) => facade.permissions.has(permission) }),
    useAppMessage: () => antd.message,
    useAdminRuntime: () => ({
      transport: facade.transport,
      auth: { signOut: async () => undefined },
    }),
    formatErrorMessage: (error: unknown) =>
      error instanceof Error ? error.message : String(error),
  };
});

vi.mock("../gen/resource/product.operations.gen", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../gen/resource/product.operations.gen")>();
  return {
    ...actual,
    createProductOperations: () => ({
      list: facade.list,
      create: facade.create,
      update: facade.update,
      delete: facade.delete,
      batchDelete: facade.batchDelete,
      keyOf: facade.keyOf,
    }),
  };
});

import ProductList from "./ProductList";

const row = create(ProductSchema, {
  id: 7n,
  productNo: "AB-100",
  name: "原商品名称",
  priceCents: 9007199254740993n,
  deptId: 1n,
  createBy: 1n,
  remark: "before",
});

function allow(...permissions: string[]): void {
  facade.permissions.clear();
  for (const permission of permissions) facade.permissions.add(permission);
}

function resetFacade(): void {
  allow("biz:product:list", "biz:product:add", "biz:product:edit", "biz:product:remove");
  facade.list.mockReset().mockResolvedValue({
    items: [row],
    total: 1n,
    pageNum: 1,
    pageSize: 10,
  });
  facade.create.mockReset().mockResolvedValue(row);
  facade.update.mockReset().mockResolvedValue(row);
  facade.delete.mockReset().mockResolvedValue(undefined);
  facade.batchDelete.mockReset().mockResolvedValue(undefined);
  facade.keyOf.mockReset().mockImplementation((entity: typeof row) => entity.id);
  vi.mocked(message.success).mockClear();
  vi.mocked(message.error).mockClear();
}

async function openCreateForm(): Promise<void> {
  await screen.findByText("原商品名称");
  fireEvent.click(screen.getByRole("button", { name: "新建商品" }));
  await screen.findByRole("dialog", { name: "新建商品" });
}

describe("ProductList", () => {
  beforeEach(() => resetFacade());

  it("renders schema columns and list query", async () => {
    render(<ProductList />);
    expect(document.querySelector('[data-resource-table="Product"]')).toBeInTheDocument();
    await waitFor(() =>
      expect(facade.list).toHaveBeenCalledWith({
        pageNum: 1,
        pageSize: 10,
      }),
    );
    expect(screen.getByRole("columnheader", { name: "商品编号" })).toBeInTheDocument();
    expect(screen.getByText("9007199254740993")).toBeInTheDocument();
  });

  it("keeps search pagination and amount sort capability on the schema", async () => {
    render(<ProductList />);
    await screen.findByText("原商品名称");
    fireEvent.change(screen.getByRole("textbox", { name: "关键词" }), {
      target: { value: "AB" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));
    await waitFor(() =>
      expect(facade.list).toHaveBeenLastCalledWith({
        pageNum: 1,
        pageSize: 10,
        keyword: "AB",
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await waitFor(() =>
      expect(facade.list).toHaveBeenLastCalledWith({
        pageNum: 2,
        pageSize: 10,
        keyword: "AB",
      }),
    );
    expect(
      productSchema.fields.filter((field) => field.sortable).map((field) => field.name),
    ).toEqual(["priceCents"]);
    fireEvent.click(screen.getByRole("button", { name: "价格(分)" }));
    await waitFor(() =>
      expect(facade.list).toHaveBeenLastCalledWith({
        pageNum: 2,
        pageSize: 10,
        keyword: "AB",
        orders: [{ field: "priceCents", desc: true }],
      }),
    );
  });

  it("creates with decimal bigint fields", async () => {
    render(<ProductList />);
    await openCreateForm();
    fireEvent.change(screen.getByLabelText("商品编号"), { target: { value: "ab-200" } });
    fireEvent.change(screen.getByLabelText("商品名称"), { target: { value: "新商品" } });
    fireEvent.change(screen.getByLabelText("价格(分)"), { target: { value: "9007199254740993" } });
    fireEvent.change(screen.getByLabelText("部门ID"), { target: { value: "1" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));
    await waitFor(() => expect(facade.create).toHaveBeenCalledOnce());
    expect(facade.create.mock.calls[0]?.[0]).toMatchObject({
      productNo: "ab-200",
      name: "新商品",
      priceCents: 9007199254740993n,
      deptId: 1n,
    });
  });

  it("keeps input after create failure", async () => {
    facade.create.mockRejectedValueOnce(new Error("create failed"));
    render(<ProductList />);
    await openCreateForm();
    fireEvent.change(screen.getByLabelText("商品编号"), { target: { value: "AB-FAIL" } });
    fireEvent.change(screen.getByLabelText("商品名称"), { target: { value: "失败商品名称" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));
    await waitFor(() => expect(message.error).toHaveBeenCalledWith("create failed"));
    expect(screen.getByRole("dialog", { name: "新建商品" })).toBeInTheDocument();
    expect(screen.getByLabelText("商品名称")).toHaveValue("失败商品名称");
  });

  it("edits with explicit empty remark and large amount", async () => {
    render(<ProductList />);
    fireEvent.click(await screen.findByRole("button", { name: "编辑" }));
    await screen.findByRole("dialog", { name: "编辑商品" });
    fireEvent.change(screen.getByLabelText("备注"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));
    await waitFor(() => expect(facade.update).toHaveBeenCalledOnce());
    const submission = facade.update.mock.calls[0]?.[0] as {
      key: bigint;
      values: { remark: string; priceCents: bigint };
      touched: ReadonlySet<string>;
    };
    expect(submission.key).toBe(7n);
    expect(submission.values.remark).toBe("");
    expect(submission.touched.has("remark")).toBe(true);
  });

  it("confirms delete and skips request when cancelled", async () => {
    render(<ProductList />);
    await screen.findByText("原商品名称");
    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    fireEvent.click(screen.getByRole("button", { name: "取消" }));
    expect(facade.delete).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(facade.delete).toHaveBeenCalledWith(7n));
  });

  it("confirms batch delete and skips when cancelled", async () => {
    render(<ProductList />);
    await screen.findByText("原商品名称");
    fireEvent.click(screen.getByRole("checkbox", { name: "选择 7" }));
    fireEvent.click(screen.getByRole("button", { name: "批量删除" }));
    fireEvent.click(screen.getByRole("button", { name: "取消" }));
    expect(facade.batchDelete).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "批量删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认批量删除" }));
    await waitFor(() => expect(facade.batchDelete).toHaveBeenCalledWith([7n]));
  });

  it("hides write actions for reader permissions", async () => {
    allow("biz:product:list");
    render(<ProductList />);
    await screen.findByText("原商品名称");
    expect(screen.queryByRole("button", { name: "新建商品" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "编辑" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "删除" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "批量删除" })).not.toBeInTheDocument();
  });
});
