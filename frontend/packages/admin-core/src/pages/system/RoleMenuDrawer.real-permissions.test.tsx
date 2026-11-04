import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { create } from "@bufbuild/protobuf";
import { MenuSchema } from "@abysstract/admin-proto/abysstract/admin/v1/menu_pb";
import { RoleSchema } from "@abysstract/admin-proto/abysstract/admin/v1/role_pb";

vi.unmock("antd");
vi.unmock("@ant-design/icons");

const api = vi.hoisted(() => {
  const list = vi.fn();
  const getRoleMenus = vi.fn();
  const updateRoleMenus = vi.fn(async () => ({}));
  return {
    message: { success: vi.fn(), error: vi.fn() },
    list,
    getRoleMenus,
    updateRoleMenus,
    operations: {
      menuOperations: { list },
      roleOperations: {
        custom: { getRoleMenus, updateRoleMenus },
      },
    },
  };
});
vi.mock("../../system/hooks", () => ({
  useSystemClients: () => ({ commonExport: {} }),
  useSystemOperations: () => api.operations,
}));
vi.mock("../../runtime/message", () => ({
  useAppMessage: () => api.message,
  formatErrorMessage: String,
}));

import RoleMenuDrawer from "./RoleMenuDrawer";

describe("RoleMenuDrawer exact permission selection", () => {
  it("enables saving only after the role permissions have loaded", async () => {
    api.list.mockResolvedValue({ items: [], total: 0n });
    let resolvePermissions!: (value: { menuIds: bigint[] }) => void;
    api.getRoleMenus.mockReturnValue(
      new Promise<{ menuIds: bigint[] }>((resolve) => {
        resolvePermissions = resolve;
      }),
    );
    render(
      <RoleMenuDrawer
        role={create(RoleSchema, { id: 7n, roleName: "只读角色" })}
        open
        onClose={vi.fn()}
      />,
    );
    const dialog = screen.getByRole("dialog");
    const save = within(dialog).getByRole("button", { name: /保\s*存/ });
    expect(save).toBeDisabled();
    expect(dialog.querySelector(".ant-skeleton")).toBeInTheDocument();

    resolvePermissions({ menuIds: [] });

    await waitFor(() => expect(save).toBeEnabled());
    expect(dialog.querySelector(".ant-skeleton")).not.toBeInTheDocument();
  });

  it("keeps a read-only menu independent from its write-action children", async () => {
    api.list.mockResolvedValue({
      items: [
        create(MenuSchema, { id: 105n, menuName: "参数设置", menuType: "C" }),
        create(MenuSchema, { id: 1051n, parentId: 105n, menuName: "参数新增", menuType: "F" }),
      ],
      total: 2n,
    });
    api.getRoleMenus.mockResolvedValue({ menuIds: [105n] });
    const close = vi.fn();
    render(
      <RoleMenuDrawer
        role={create(RoleSchema, { id: 7n, roleName: "只读角色" })}
        open
        onClose={close}
      />,
    );
    const menu = await screen.findByText("参数设置", { exact: true });
    const menuNode = menu.closest(".ant-tree-treenode");
    if (menuNode === null) throw new Error("menu node missing");
    const switcher = menuNode.querySelector(".ant-tree-switcher_close");
    if (switcher) fireEvent.click(switcher);
    const action = await screen.findByText("参数新增", { exact: true });
    const actionNode = action.closest(".ant-tree-treenode");
    if (actionNode === null) throw new Error("action node missing");
    expect(menuNode.querySelector(".ant-tree-checkbox")).toHaveClass("ant-tree-checkbox-checked");
    expect(actionNode.querySelector(".ant-tree-checkbox")).not.toHaveClass(
      "ant-tree-checkbox-checked",
    );
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: /保\s*存/ }));
    await waitFor(() =>
      expect(api.updateRoleMenus).toHaveBeenCalledWith({ roleId: 7n, menuIds: [105n] }),
    );
    await waitFor(() => expect(close).toHaveBeenCalledTimes(1));
    const actionCheckbox = actionNode.querySelector(".ant-tree-checkbox");
    const menuCheckbox = menuNode.querySelector(".ant-tree-checkbox");
    if (actionCheckbox === null || menuCheckbox === null) throw new Error("checkbox missing");
    fireEvent.click(actionCheckbox);
    expect(actionCheckbox).toHaveClass("ant-tree-checkbox-checked");
    fireEvent.click(menuCheckbox);
    expect(menuCheckbox).not.toHaveClass("ant-tree-checkbox-checked");
    expect(actionCheckbox).toHaveClass("ant-tree-checkbox-checked");
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: /保\s*存/ }));
    await waitFor(() =>
      expect(api.updateRoleMenus).toHaveBeenLastCalledWith({ roleId: 7n, menuIds: [1051n] }),
    );
    await waitFor(() => expect(close).toHaveBeenCalledTimes(2));
  });
});
