import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, it, expect, vi, beforeEach } from "vitest";
import UserList from "../pages/system/UserList";

const mocks = vi.hoisted(() => ({
  listUsers: vi.fn(),
}));

vi.mock("../runtime/access", () => ({
  useAccess: () => ({ has: () => true }),
}));

vi.mock("../runtime/message", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../runtime/message")>();
  const antd = await import("antd");
  return {
    ...actual,
    useAppMessage: () => antd.message,
  };
});

vi.mock("../system/hooks", () => {
  const operations = {
    userOperations: {
      keyOf: (user: { id: bigint }) => user.id,
      list: (...args: unknown[]) => mocks.listUsers(...args),
      custom: {
        getUser: vi.fn(),
        createUser: vi.fn(),
        updateUser: vi.fn(),
        deleteUser: vi.fn(),
        batchDeleteUsers: vi.fn(),
        changeUserStatus: vi.fn(),
        resetPassword: vi.fn(),
      },
    },
    userBuildUpdateRequest: vi.fn(),
    roleOperations: { list: vi.fn().mockResolvedValue({ items: [] }) },
    deptOperations: { list: vi.fn().mockResolvedValue({ items: [] }) },
  };
  return {
    useSystemClients: () => ({ commonExport: {} }),
    useSystemOperations: () => operations,
  };
});

describe("UserList 页面冒烟测试", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.listUsers.mockResolvedValue({
      items: [
        {
          id: 1n,
          username: "admin",
          nickname: "管理员",
          email: "admin@example.com",
          phone: "",
          status: 1,
          deptId: 0n,
          roleIds: [],
          remark: "",
        },
      ],
      total: 1n,
      pageNum: 1,
      pageSize: 10,
    });
  });

  it("应渲染标志性元素并绑定 generated List facade", async () => {
    render(
      <MemoryRouter initialEntries={["/"]}>
        <Routes>
          <Route path="*" element={<UserList />} />
        </Routes>
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(screen.getByText("新建用户")).toBeInTheDocument();
    });
    expect(mocks.listUsers).toHaveBeenCalledOnce();
    expect(await screen.findByText("admin")).toBeInTheDocument();
  });
});
