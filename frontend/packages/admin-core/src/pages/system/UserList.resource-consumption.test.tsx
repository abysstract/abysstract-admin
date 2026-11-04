import { render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { describe, expect, it, vi } from "vitest";
import { UserSchema } from "@abysstract/admin-proto/abysstract/admin/v1/user_pb";

const list = vi.hoisted(() => vi.fn());

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

vi.mock("../../system/hooks", async () => {
  const { createSystemOperations } = await import("../../system/operations");
  const actual = createSystemOperations({} as never);
  const operations = {
    ...actual,
    userOperations: {
      ...actual.userOperations,
      list,
      custom: {
        ...actual.userOperations.custom,
        getUser: vi.fn(),
        createUser: vi.fn(),
        updateUser: vi.fn(),
        deleteUser: vi.fn(),
        batchDeleteUsers: vi.fn(),
        changeUserStatus: vi.fn(),
        resetPassword: vi.fn(),
      },
    },
  };
  return {
    useSystemClients: () => ({ commonExport: {} }),
    useSystemOperations: () => operations,
  };
});

import UserList from "./UserList";

describe("UserList real Resource component integration", () => {
  it("renders ResourceTable and binds the generated read facade", async () => {
    list.mockReset().mockResolvedValue({
      items: [
        create(UserSchema, {
          id: 9007199254740993n,
          username: "真实页面用户",
          nickname: "迁移用户",
          email: "user@example.com",
          phone: "",
          status: 1,
          deptId: 0n,
          roleIds: [],
          remark: "",
        }),
      ],
      total: 1n,
      pageNum: 1,
      pageSize: 10,
    });

    render(<UserList />);

    expect(screen.getByTestId("pro-table-toolbar")).toBeInTheDocument();
    await waitFor(() => expect(list).toHaveBeenCalledOnce());
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("新建用户");
    expect(screen.getByText("真实页面用户")).toBeInTheDocument();
  });
});
