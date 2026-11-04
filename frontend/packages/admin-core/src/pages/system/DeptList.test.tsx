import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { message } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DeptSchema } from "@abysstract/admin-proto/abysstract/admin/v1/dept_pb";
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
    deptOperations: {
      ...actual.deptOperations,
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

import DeptList from "./DeptList";

const row = create(DeptSchema, {
  id: 7n,
  parentId: 0n,
  deptName: "原部门",
  orderNum: 1,
  leader: "负责人",
  phone: "13800000000",
  email: "dept@example.com",
  status: Status.ENABLED,
});

function resetMocks(): void {
  mocks.permissions.clear();
  for (const permission of [
    "system:dept:list",
    "system:dept:add",
    "system:dept:edit",
    "system:dept:remove",
  ]) {
    mocks.permissions.add(permission);
  }
  mocks.list.mockReset().mockResolvedValue({ items: [row], total: 1n, pageNum: 1, pageSize: 10 });
  mocks.create.mockReset().mockResolvedValue(row);
  mocks.update.mockReset().mockResolvedValue(row);
  mocks.delete.mockReset().mockResolvedValue(undefined);
  mocks.batchDelete.mockReset().mockResolvedValue(undefined);
  mocks.keyOf.mockReset().mockImplementation((entity: typeof row) => entity.id);
  vi.mocked(message.success).mockClear();
  vi.mocked(message.error).mockClear();
}

async function openCreateForm(): Promise<void> {
  await screen.findByText("原部门");
  fireEvent.click(screen.getByRole("button", { name: "新建部门" }));
  await screen.findByRole("dialog", { name: "新建部门" });
}

async function openEditForm(): Promise<void> {
  await screen.findByRole("button", { name: "编辑" });
  fireEvent.click(screen.getByRole("button", { name: "编辑" }));
  await screen.findByRole("dialog", { name: "编辑部门" });
}

describe("DeptList Resource migration", () => {
  beforeEach(() => resetMocks());

  it("consumes generated Dept schema/operations through ResourceTable", async () => {
    render(<DeptList />);
    expect(document.querySelector('[data-resource-table="Dept"]')).toBeInTheDocument();
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("新建部门");
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("批量删除");
    await waitFor(() =>
      expect(mocks.list).toHaveBeenCalledWith({
        pageNum: 1,
        pageSize: 100,
        orders: [{ field: "orderNum", desc: false }],
      }),
    );
    expect(screen.getByRole("columnheader", { name: "部门名称" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "负责人" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "状态" })).toBeInTheDocument();
  });

  it("loads the final parent option at an exact 100-row page boundary", async () => {
    const departments = Array.from({ length: 101 }, (_, index) =>
      create(DeptSchema, {
        id: BigInt(index + 1),
        parentId: 0n,
        deptName: `部门${index + 1}`,
        orderNum: index,
        status: Status.ENABLED,
      }),
    );
    mocks.list.mockImplementation(({ pageNum, pageSize }: { pageNum: number; pageSize: number }) =>
      Promise.resolve({
        items: departments.slice((pageNum - 1) * pageSize, pageNum * pageSize),
        total: BigInt(departments.length),
        pageNum,
        pageSize,
      }),
    );

    render(<DeptList />);
    fireEvent.click(screen.getByRole("button", { name: "新建部门" }));
    await screen.findByRole("dialog", { name: "新建部门" });

    expect(await screen.findByRole("option", { name: "部门101" })).toBeInTheDocument();
  });

  it("uses ResourceForm create overrides and keeps typed bigint parent values", async () => {
    render(<DeptList />);
    await openCreateForm();

    expect(screen.getByLabelText("部门名称")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "上级部门" })).toBeInTheDocument();
    expect(screen.getByText("顶级部门")).toBeInTheDocument();
    fireEvent.change(screen.getByRole("combobox", { name: "上级部门" }), {
      target: { value: "7" },
    });
    fireEvent.change(screen.getByLabelText("部门名称"), { target: { value: "新部门" } });
    fireEvent.change(screen.getByLabelText("负责人"), { target: { value: "新负责人" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(mocks.create).toHaveBeenCalledOnce());
    expect(mocks.create.mock.calls[0]?.[0]).toMatchObject({
      parentId: 7n,
      deptName: "新部门",
      leader: "新负责人",
      orderNum: 0,
    });
    expect(message.success).toHaveBeenCalledWith("创建成功");
  });

  it("uses generated update presence and preserves an explicit empty email", async () => {
    render(<DeptList />);
    await openEditForm();

    fireEvent.change(screen.getByLabelText("邮箱"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(mocks.update).toHaveBeenCalledOnce());
    const submission = mocks.update.mock.calls[0]?.[0] as {
      key: bigint;
      values: Record<string, unknown>;
      touched: ReadonlySet<string>;
      dirty: ReadonlySet<string>;
    };
    expect(submission.key).toBe(7n);
    expect(submission.values.email).toBe("");
    expect(submission.touched).toEqual(new Set(["email"]));
    expect(submission.dirty).toEqual(new Set(["email"]));
    expect(message.success).toHaveBeenCalledWith("已保存");
  });

  it("routes standard delete and batch delete through the generated facade", async () => {
    render(<DeptList />);
    await screen.findByText("原部门");

    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(mocks.delete).toHaveBeenCalledWith(7n));

    fireEvent.click(screen.getAllByRole("checkbox")[0]);
    fireEvent.click(screen.getByRole("button", { name: "批量删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认批量删除" }));
    await waitFor(() => expect(mocks.batchDelete).toHaveBeenCalledWith([7n]));
  });

  it("keeps list errors and permission decisions in ResourceTable", async () => {
    mocks.list
      .mockReset()
      .mockRejectedValueOnce(new Error("list failed"))
      .mockResolvedValueOnce({ items: [row], total: 1n, pageNum: 1, pageSize: 10 });
    const rendered = render(<DeptList />);

    expect(await screen.findByRole("alert")).toHaveTextContent("list failed");
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => expect(mocks.list).toHaveBeenCalledTimes(2));

    mocks.permissions.delete("system:dept:add");
    mocks.permissions.delete("system:dept:edit");
    mocks.permissions.delete("system:dept:remove");
    rendered.rerender(<DeptList />);
    expect(screen.queryByRole("button", { name: "新建部门" })).not.toBeInTheDocument();
  });
});
