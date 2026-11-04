import { render, screen } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { Status } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";
import { UserSchema } from "@abysstract/admin-proto/abysstract/admin/v1/user_pb";

const mocks = vi.hoisted(() => ({
  permissions: new Set<string>(),
  listUsers: vi.fn(),
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

vi.mock("../../system/hooks", () => {
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

import UserList from "./UserList";

const row = create(UserSchema, {
  id: 1n,
  username: "partial-user",
  nickname: "部分权限用户",
  status: Status.ENABLED,
  deptId: 0n,
});

function allow(...permissions: string[]): void {
  mocks.permissions.clear();
  for (const permission of permissions) mocks.permissions.add(permission);
}

async function renderUserList(): Promise<void> {
  render(<UserList />);
  await screen.findByText("部分权限用户");
}

describe("UserList CUSTOM workflow permissions", () => {
  beforeEach(() => {
    allow(
      "system:user:list",
      "system:user:add",
      "system:user:edit",
      "system:user:query",
      "system:role:list",
      "system:dept:list",
    );
    mocks.listUsers.mockReset().mockResolvedValue({
      items: [row],
      total: 1n,
      pageNum: 1,
      pageSize: 10,
    });
  });

  it.each([
    ["system:user:query", "dedicated GetUser"],
    ["system:role:list", "role options"],
    ["system:dept:list", "department options"],
  ])("hides edit when %s is missing (%s)", async (missingPermission) => {
    mocks.permissions.delete(missingPermission);

    await renderUserList();

    expect(screen.queryByRole("button", { name: "编辑" })).not.toBeInTheDocument();
  });

  it.each([
    ["system:role:list", "role options"],
    ["system:dept:list", "department options"],
  ])("hides create when %s is missing (%s)", async (missingPermission) => {
    mocks.permissions.delete(missingPermission);

    await renderUserList();

    expect(screen.queryByRole("button", { name: "新建用户" })).not.toBeInTheDocument();
  });
});
