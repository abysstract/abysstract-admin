import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  listUsers: vi.fn(),
  listRoles: vi.fn(),
  listDepts: vi.fn(),
  createUser: vi.fn(),
}));

vi.mock("../../runtime/access", () => ({
  useAccess: () => ({ has: () => true }),
}));
vi.mock("../../runtime/message", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../../runtime/message")>();
  const antd = await import("antd");
  return {
    ...actual,
    useAppMessage: () => antd.message,
  };
});

vi.mock("../../system/hooks", () => {
  const operations = {
    userOperations: {
      keyOf: (user: { id: bigint }) => user.id,
      list: (...args: unknown[]) => mocks.listUsers(...args),
      custom: {
        getUser: vi.fn(),
        createUser: (...args: unknown[]) => mocks.createUser(...args),
        updateUser: vi.fn(),
        deleteUser: vi.fn(),
        batchDeleteUsers: vi.fn(),
        changeUserStatus: vi.fn(),
        resetPassword: vi.fn(),
      },
    },
    userBuildUpdateRequest: vi.fn(),
    roleOperations: {
      list: (...args: unknown[]) => mocks.listRoles(...args),
    },
    deptOperations: {
      list: (...args: unknown[]) => mocks.listDepts(...args),
    },
  };
  return {
    useSystemClients: () => ({ commonExport: {} }),
    useSystemOperations: () => operations,
  };
});

import UserList from "./UserList";

async function openCreateForm() {
  render(<UserList />);
  await waitFor(() => expect(mocks.listUsers).toHaveBeenCalledOnce());
  fireEvent.click(screen.getByRole("button", { name: "新建用户" }));
}

describe("UserList 新建用户校验", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.listUsers.mockResolvedValue({ items: [], total: 0n, pageNum: 1, pageSize: 10 });
    mocks.listRoles.mockResolvedValue({ items: [{ id: 1n, roleName: "管理员" }] });
    mocks.listDepts.mockResolvedValue({ items: [] });
    mocks.createUser.mockResolvedValue({});
  });

  it("未选择角色时在角色字段下显示表单错误，而不是黑色全局文本", async () => {
    await openCreateForm();
    fireEvent.change(screen.getByLabelText("用户名"), { target: { value: "new-user" } });
    fireEvent.change(screen.getByLabelText("密码"), { target: { value: "GoodPass1" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    const roleError = await screen.findByText("请至少选择一个角色");
    expect(roleError.closest("[role=alert]")).toBeInTheDocument();
    expect(mocks.createUser).not.toHaveBeenCalled();
  });

  it("在提交前校验邮箱格式，不依赖后端返回错误", async () => {
    await openCreateForm();
    fireEvent.change(screen.getByLabelText("用户名"), { target: { value: "new-user" } });
    fireEvent.change(screen.getByLabelText("邮箱"), { target: { value: "not-an-email" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    expect(await screen.findByText("邮箱格式不正确")).toBeInTheDocument();
    expect(mocks.createUser).not.toHaveBeenCalled();
  });

  it("在提交前校验中国大陆手机号格式，不依赖后端返回错误", async () => {
    await openCreateForm();
    fireEvent.change(screen.getByLabelText("用户名"), { target: { value: "new-user" } });
    fireEvent.change(screen.getByLabelText("手机号"), { target: { value: "1380000000" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    expect(await screen.findByText("手机号格式不正确")).toBeInTheDocument();
    expect(mocks.createUser).not.toHaveBeenCalled();
  });

  it("分页加载超过后端单页上限的部门和角色选项", async () => {
    const roles = Array.from({ length: 205 }, (_, index) => ({
      id: BigInt(index + 1),
      roleName: `角色${index + 1}`,
    }));
    const depts = Array.from({ length: 205 }, (_, index) => ({
      id: BigInt(index + 1),
      deptName: `部门${index + 1}`,
    }));
    const page = <T,>(items: readonly T[], pageNum: number, pageSize: number) => ({
      items: items.slice((pageNum - 1) * pageSize, pageNum * pageSize),
      total: BigInt(items.length),
      pageNum,
      pageSize,
    });
    mocks.listRoles.mockImplementation(
      ({ pageNum, pageSize }: { pageNum: number; pageSize: number }) =>
        Promise.resolve(page(roles, pageNum, pageSize)),
    );
    mocks.listDepts.mockImplementation(
      ({ pageNum, pageSize }: { pageNum: number; pageSize: number }) =>
        Promise.resolve(page(depts, pageNum, pageSize)),
    );

    await openCreateForm();

    await waitFor(() => {
      expect(mocks.listRoles).toHaveBeenCalledTimes(3);
      expect(mocks.listDepts).toHaveBeenCalledTimes(3);
    });
    expect(mocks.listRoles.mock.calls.map(([query]) => query)).toEqual([
      { pageNum: 1, pageSize: 100 },
      { pageNum: 2, pageSize: 100 },
      { pageNum: 3, pageSize: 100 },
    ]);
    expect(mocks.listDepts.mock.calls.map(([query]) => query)).toEqual([
      { pageNum: 1, pageSize: 100 },
      { pageNum: 2, pageSize: 100 },
      { pageNum: 3, pageSize: 100 },
    ]);
  });
});
