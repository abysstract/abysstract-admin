import { render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { describe, expect, it, vi } from "vitest";
import { RoleSchema } from "@abysstract/admin-proto/abysstract/admin/v1/role_pb";

const list = vi.hoisted(() => vi.fn());
const noopMutation = vi.hoisted(() => vi.fn());

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
    roleOperations: {
      ...actual.roleOperations,
      list,
      create: noopMutation,
      update: noopMutation,
      delete: noopMutation,
      batchDelete: noopMutation,
      custom: {
        ...actual.roleOperations.custom,
        changeRoleStatus: noopMutation,
        getRoleMenus: vi.fn().mockResolvedValue({ menuIds: [] }),
        updateRoleMenus: noopMutation,
      },
    },
  };
  return {
    useSystemClients: () => ({ commonExport: {} }),
    useSystemOperations: () => operations,
  };
});

import RoleList from "./RoleList";

describe("RoleList real Resource component integration", () => {
  it("renders the generated ResourceTable and binds the generated facade list operation", async () => {
    list.mockReset().mockResolvedValue({
      items: [
        create(RoleSchema, {
          id: 9007199254740993n,
          roleName: "真实页面角色",
          roleKey: "real-role",
          dataScope: 1,
        }),
      ],
      total: 1n,
      pageNum: 1,
      pageSize: 10,
    });

    render(<RoleList />);

    expect(screen.getByTestId("pro-table-toolbar")).toBeInTheDocument();
    await waitFor(() => expect(list).toHaveBeenCalledOnce());
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("新建角色");
    expect(screen.getByText("真实页面角色")).toBeInTheDocument();
    expect(screen.getByText("全部数据")).toBeInTheDocument();
  });
});
