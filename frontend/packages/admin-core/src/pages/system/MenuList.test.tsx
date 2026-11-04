import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { message } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MenuSchema } from "@abysstract/admin-proto/abysstract/admin/v1/menu_pb";
import { Status } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";

const mocks = vi.hoisted(() => ({
  permissions: new Set<string>(),
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  delete: vi.fn(),
  batchDelete: vi.fn(),
  keyOf: vi.fn(),
}));

vi.mock("../../runtime/access", () => ({
  useAccess: () => ({ has: (permission: string) => mocks.permissions.has(permission) }),
}));
vi.mock("../../runtime/message", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../runtime/message")>();
  const antd = await import("antd");
  return {
    ...actual,
    useAppMessage: () => antd.message,
  };
});

vi.mock("../../system/hooks", async () => {
  const { createSystemOperations } = await import("../../system/operations");
  const actual = createSystemOperations({} as never);
  const operations = {
    ...actual,
    menuOperations: {
      ...actual.menuOperations,
      list: mocks.list,
      create: mocks.create,
      update: mocks.update,
      delete: mocks.delete,
      batchDelete: mocks.batchDelete,
      keyOf: mocks.keyOf,
    },
  };
  return {
    useSystemClients: () => ({ commonExport: {} }),
    useSystemOperations: () => operations,
  };
});

import MenuList from "./MenuList";

const row = create(MenuSchema, {
  id: 7n,
  parentId: 0n,
  menuName: "原菜单",
  menuType: "C",
  orderNum: 1,
  path: "original",
  component: "system/original",
  perms: "system:original:list",
  status: Status.ENABLED,
});

function resetMocks(): void {
  mocks.permissions.clear();
  for (const permission of [
    "system:menu:list",
    "system:menu:add",
    "system:menu:edit",
    "system:menu:remove",
  ]) {
    mocks.permissions.add(permission);
  }
  mocks.list.mockReset().mockResolvedValue({ items: [row], total: 1n, pageNum: 1, pageSize: 100 });
  mocks.create.mockReset().mockResolvedValue(row);
  mocks.update.mockReset().mockResolvedValue(row);
  mocks.delete.mockReset().mockResolvedValue(undefined);
  mocks.batchDelete.mockReset().mockResolvedValue(undefined);
  mocks.keyOf.mockReset().mockImplementation((entity: typeof row) => entity.id);
  vi.mocked(message.success).mockClear();
  vi.mocked(message.error).mockClear();
}

async function openCreateForm(): Promise<void> {
  await screen.findByText("原菜单");
  fireEvent.click(screen.getByRole("button", { name: "新建菜单" }));
  await screen.findByRole("dialog", { name: "新建菜单" });
}

async function openEditForm(): Promise<void> {
  await screen.findByRole("button", { name: "编辑" });
  fireEvent.click(screen.getByRole("button", { name: "编辑" }));
  await screen.findByRole("dialog", { name: "编辑菜单" });
}

describe("MenuList Resource migration", () => {
  beforeEach(() => resetMocks());

  it("consumes generated Menu schema/operations through ResourceTable", async () => {
    render(<MenuList />);
    expect(document.querySelector('[data-resource-table="Menu"]')).toBeInTheDocument();
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("新建菜单");
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("批量删除");
    await waitFor(() => expect(mocks.list).toHaveBeenCalledOnce());
    expect(screen.getByRole("columnheader", { name: "菜单名称" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "类型" })).toBeInTheDocument();
    expect(await screen.findByText("菜单")).toBeInTheDocument();
  });

  it("uses TreeSelect for parent and Select for menu_type in ResourceForm create", async () => {
    const child = create(MenuSchema, {
      id: 8n,
      parentId: row.id,
      menuName: "子菜单",
      menuType: "C",
      orderNum: 2,
    });
    mocks.list.mockReset().mockResolvedValue({
      items: [row, child],
      total: 2n,
      pageNum: 1,
      pageSize: 100,
    });
    render(<MenuList />);
    await openCreateForm();

    expect(screen.getByTestId("tree-select")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "上级菜单" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "类型" })).toBeInTheDocument();
    expect(screen.getByText("顶级菜单")).toBeInTheDocument();
    expect(screen.getByText("└ 子菜单")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("菜单名称"), { target: { value: "新菜单" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));
    await waitFor(() => expect(mocks.create).toHaveBeenCalledOnce());
    expect(mocks.create.mock.calls[0]?.[0]).toMatchObject({
      parentId: 0n,
      menuName: "新菜单",
      menuType: "C",
    });
    expect(message.success).toHaveBeenCalledWith("创建成功");
  });

  it("uses generated optional update presence", async () => {
    render(<MenuList />);
    await openEditForm();
    fireEvent.change(screen.getByLabelText("路由地址"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));
    await waitFor(() => expect(mocks.update).toHaveBeenCalledOnce());
    const submission = mocks.update.mock.calls[0]?.[0] as {
      key: bigint;
      values: Record<string, unknown>;
      touched: ReadonlySet<string>;
    };
    expect(submission.key).toBe(7n);
    expect(submission.values.path).toBe("");
    expect(submission.touched).toEqual(new Set(["path"]));
    expect(message.success).toHaveBeenCalledWith("已保存");
  });

  it("routes standard delete and batch delete through generated operations", async () => {
    render(<MenuList />);
    await screen.findByText("原菜单");
    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(mocks.delete).toHaveBeenCalledWith(7n));
    fireEvent.click(screen.getAllByRole("checkbox")[0]);
    fireEvent.click(screen.getByRole("button", { name: "批量删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认批量删除" }));
    await waitFor(() => expect(mocks.batchDelete).toHaveBeenCalledWith([7n]));
  });
});
