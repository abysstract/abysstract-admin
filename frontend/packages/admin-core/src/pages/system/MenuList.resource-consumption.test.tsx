import { render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { describe, expect, it, vi } from "vitest";
import { MenuSchema } from "@abysstract/admin-proto/abysstract/admin/v1/menu_pb";

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
    menuOperations: {
      ...actual.menuOperations,
      list,
      create: noopMutation,
      update: noopMutation,
      delete: noopMutation,
      batchDelete: noopMutation,
    },
  };
  return {
    useSystemClients: () => ({ commonExport: {} }),
    useSystemOperations: () => operations,
  };
});

import MenuList from "./MenuList";

describe("MenuList real Resource component integration", () => {
  it("renders the generated ResourceTable with flat rows projected to Tree Table data", async () => {
    list.mockReset().mockResolvedValue({
      items: [
        create(MenuSchema, { id: 1n, parentId: 0n, menuName: "系统管理", menuType: "M" }),
        create(MenuSchema, { id: 2n, parentId: 1n, menuName: "菜单管理", menuType: "C" }),
      ],
      total: 2n,
      pageNum: 1,
      pageSize: 100,
    });

    render(<MenuList />);

    expect(document.querySelector('[data-resource-table="Menu"]')).toBeInTheDocument();
    await waitFor(() =>
      expect(list).toHaveBeenCalledWith({
        pageNum: 1,
        pageSize: 100,
        orders: [{ field: "orderNum", desc: false }],
      }),
    );
    expect(screen.getByText("系统管理")).toBeInTheDocument();
    expect(screen.getByText("菜单管理")).toBeInTheDocument();
  });
});
