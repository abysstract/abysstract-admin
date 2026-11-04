import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { Code, ConnectError } from "@connectrpc/connect";
import { create } from "@bufbuild/protobuf";
import { message } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RoleSchema } from "@abysstract/admin-proto/abysstract/admin/v1/role_pb";
import { Status } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";

const mocks = vi.hoisted(() => {
  const list = vi.fn();
  const create = vi.fn();
  const update = vi.fn();
  const deleteFn = vi.fn();
  const batchDelete = vi.fn();
  const keyOf = vi.fn();
  const changeRoleStatus = vi.fn();
  const getRoleMenus = vi.fn();
  const updateRoleMenus = vi.fn();
  const menuList = vi.fn();
  return {
    permissions: new Set<string>(),
    list,
    create,
    update,
    delete: deleteFn,
    batchDelete,
    keyOf,
    changeRoleStatus,
    getRoleMenus,
    updateRoleMenus,
    menuList,
    operations: {
      menuOperations: { list: menuList },
      roleOperations: {
        list,
        create,
        update,
        delete: deleteFn,
        batchDelete,
        keyOf,
        custom: { changeRoleStatus, getRoleMenus, updateRoleMenus },
      },
    },
  };
});

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

vi.mock("../../system/hooks", () => ({
  useSystemClients: () => ({ commonExport: {} }),
  useSystemOperations: () => mocks.operations,
}));

import RoleList from "./RoleList";

const row = create(RoleSchema, {
  id: 7n,
  roleName: "原角色",
  roleKey: "original-role",
  roleSort: 1,
  dataScope: 1,
  status: Status.ENABLED,
  remark: "原备注",
});

function resetMocks(): void {
  mocks.permissions.clear();
  for (const permission of [
    "system:role:list",
    "system:role:add",
    "system:role:edit",
    "system:role:remove",
    "system:menu:list",
  ]) {
    mocks.permissions.add(permission);
  }
  mocks.list.mockReset().mockResolvedValue({ items: [row], total: 1n, pageNum: 1, pageSize: 10 });
  mocks.create.mockReset().mockResolvedValue(row);
  mocks.update.mockReset().mockResolvedValue(row);
  mocks.delete.mockReset().mockResolvedValue(undefined);
  mocks.batchDelete.mockReset().mockResolvedValue(undefined);
  mocks.keyOf.mockReset().mockImplementation((entity: typeof row) => entity.id);
  mocks.changeRoleStatus.mockReset().mockResolvedValue(undefined);
  mocks.getRoleMenus.mockReset().mockResolvedValue({ menuIds: [] });
  mocks.updateRoleMenus.mockReset().mockResolvedValue(undefined);
  mocks.menuList.mockReset().mockResolvedValue({ items: [], total: 0n, pageNum: 1, pageSize: 100 });
  vi.mocked(message.success).mockClear();
  vi.mocked(message.error).mockClear();
}

async function openCreateForm(): Promise<void> {
  await screen.findByText("原角色");
  fireEvent.click(screen.getByRole("button", { name: "新建角色" }));
  await screen.findByRole("dialog", { name: "新建角色" });
}

async function openEditForm(): Promise<void> {
  await screen.findByRole("button", { name: "编辑" });
  fireEvent.click(screen.getByRole("button", { name: "编辑" }));
  await screen.findByRole("dialog", { name: "编辑角色" });
}

describe("RoleList Resource migration", () => {
  beforeEach(() => resetMocks());

  it("consumes the generated Role schema and operations facade through ResourceTable", async () => {
    render(<RoleList />);

    expect(document.querySelector('[data-resource-table="Role"]')).toBeInTheDocument();
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("新建角色");
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("批量删除");
    await waitFor(() =>
      expect(mocks.list).toHaveBeenCalledWith({
        pageNum: 1,
        pageSize: 10,
      }),
    );
    expect(screen.getByRole("columnheader", { name: "角色名称" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "权限标识" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "数据范围" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "状态" })).toBeInTheDocument();
  });

  it("uses ResourceForm with typed data_scope options and generated create values", async () => {
    render(<RoleList />);
    await openCreateForm();

    expect(screen.getByLabelText("角色名称")).toBeInTheDocument();
    expect(screen.getByLabelText("权限标识")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "数据范围" })).toBeInTheDocument();
    expect(screen.getAllByText("全部数据").length).toBeGreaterThan(0);

    fireEvent.change(screen.getByLabelText("角色名称"), { target: { value: "新角色" } });
    fireEvent.change(screen.getByLabelText("权限标识"), { target: { value: "new-role" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(mocks.create).toHaveBeenCalledOnce());
    expect(mocks.create).toHaveBeenCalledWith({
      roleName: "新角色",
      roleKey: "new-role",
      roleSort: 0,
      dataScope: 1,
      remark: "",
    });
    expect(message.success).toHaveBeenCalledWith("创建成功");
  });

  it("uses generated update presence and keeps role_key out of the update form", async () => {
    render(<RoleList />);
    await openEditForm();

    expect(screen.queryByLabelText("权限标识")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("备注"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(mocks.update).toHaveBeenCalledOnce());
    const submission = mocks.update.mock.calls[0]?.[0] as {
      key: bigint;
      values: Record<string, unknown>;
      touched: ReadonlySet<string>;
      dirty: ReadonlySet<string>;
    };
    expect(submission.key).toBe(7n);
    expect(submission.values).toEqual({
      roleName: "原角色",
      roleSort: 1,
      dataScope: 1,
      remark: "",
    });
    expect(submission.touched).toEqual(new Set(["remark"]));
    expect(submission.dirty).toEqual(new Set(["remark"]));
    expect(message.success).toHaveBeenCalledWith("已保存");
  });

  it("routes typed custom status and menu actions through the facade", async () => {
    render(<RoleList />);
    await screen.findByText("原角色");

    fireEvent.click(screen.getByLabelText("原角色状态"));
    await waitFor(() =>
      expect(mocks.changeRoleStatus).toHaveBeenCalledWith({ id: 7n, status: Status.DISABLED }),
    );
    expect(message.success).toHaveBeenCalledWith("角色已停用");
    expect(screen.getByLabelText("原角色状态")).toBeInTheDocument();
    await waitFor(() => expect(mocks.list).toHaveBeenCalledTimes(2));

    fireEvent.click(screen.getByRole("button", { name: "分配菜单" }));
    await waitFor(() => expect(mocks.getRoleMenus).toHaveBeenCalledWith({ roleId: 7n }));
    expect(screen.getByRole("dialog", { name: "分配菜单权限 - 原角色" })).toBeInTheDocument();
  });

  it("hides menu assignment when the menu list prerequisite is missing", async () => {
    mocks.permissions.delete("system:menu:list");
    render(<RoleList />);
    await screen.findByText("原角色");

    expect(screen.queryByRole("button", { name: "分配菜单" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "编辑" })).toBeInTheDocument();
  });

  it("shows business text instead of the ConnectRPC status code", async () => {
    mocks.changeRoleStatus.mockRejectedValueOnce(
      new ConnectError("不能停用当前登录用户的唯一有效角色", Code.FailedPrecondition),
    );
    render(<RoleList />);
    await screen.findByText("原角色");

    fireEvent.click(screen.getByLabelText("原角色状态"));
    await waitFor(() =>
      expect(message.error).toHaveBeenCalledWith("不能停用当前登录用户的唯一有效角色"),
    );
    expect(message.error).not.toHaveBeenCalledWith(expect.stringContaining("failed_precondition"));
  });

  it("uses standard generated delete and batch-delete facade actions", async () => {
    render(<RoleList />);
    await screen.findByText("原角色");

    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(mocks.delete).toHaveBeenCalledWith(7n));

    fireEvent.click(screen.getAllByRole("checkbox")[0]);
    fireEvent.click(screen.getByRole("button", { name: "批量删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认批量删除" }));
    await waitFor(() => expect(mocks.batchDelete).toHaveBeenCalledWith([7n]));
  });

  it("keeps the role list visible when a delete is rejected by business policy", async () => {
    mocks.delete.mockRejectedValueOnce(
      new ConnectError("超级管理员角色不可删除", Code.FailedPrecondition),
    );
    render(<RoleList />);
    await screen.findByText("原角色");

    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(message.error).toHaveBeenCalledWith("超级管理员角色不可删除"));

    expect(screen.getByText("原角色")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("hides standard actions from generated permission decisions", async () => {
    mocks.permissions.delete("system:role:add");
    mocks.permissions.delete("system:role:edit");
    mocks.permissions.delete("system:role:remove");
    render(<RoleList />);

    await waitFor(() => expect(mocks.list).toHaveBeenCalledOnce());
    expect(screen.queryByRole("button", { name: "新建角色" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "编辑" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("原角色状态")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "分配菜单" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "删除" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "批量删除" })).not.toBeInTheDocument();
  });
});
