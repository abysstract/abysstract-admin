import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { ConfigProvider, Modal } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { UserSchema } from "@abysstract/admin-proto/abysstract/admin/v1/user_pb";

// Exercise the real confirmation popup, not the pass-through global mock.
vi.unmock("antd");

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  batchDeleteUsers: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}));

vi.mock("../../runtime/access", () => ({
  useAccess: () => ({
    has: (permission: string) => ["system:user:list", "system:user:remove"].includes(permission),
  }),
}));
vi.mock("../../runtime/message", async () => {
  const { Modal } = await import("antd");
  const message = {
    success: mocks.success,
    error: mocks.error,
    warning: vi.fn(),
    info: vi.fn(),
  };
  return {
    useAppMessage: () => message,
    // Resolve Modal at call time so the test can spy on Modal.error.
    useAppFeedback: () => ({
      message,
      modal: {
        error: (config: unknown) => Modal.error(config as never),
        confirm: (config: unknown) => Modal.confirm(config as never),
      },
    }),
    formatErrorMessage: (error: unknown, fallback = "操作失败") =>
      error instanceof Error ? error.message || fallback : fallback,
  };
});
vi.mock("../../system/hooks", async () => {
  const { createSystemOperations } = await import("../../system/operations");
  const actual = createSystemOperations({} as never);
  const operations = {
    ...actual,
    userOperations: {
      ...actual.userOperations,
      list: mocks.list,
      custom: { ...actual.userOperations.custom, batchDeleteUsers: mocks.batchDeleteUsers },
    },
  };
  return {
    useSystemClients: () => ({ commonExport: {} }),
    useSystemOperations: () => operations,
  };
});

import UserList from "./UserList";

const id = 9007199254740993n;

beforeEach(() => {
  vi.clearAllMocks();
  mocks.list.mockResolvedValue({
    items: [create(UserSchema, { id, username: "待删除用户", status: 1 })],
    total: 1n,
    pageNum: 1,
    pageSize: 20,
  });
  mocks.batchDeleteUsers.mockReset().mockResolvedValue({ deleted: 1 });
});

async function selectUser() {
  render(
    <ConfigProvider theme={{ token: { motion: false } }}>
      <UserList />
    </ConfigProvider>,
  );
  const checkbox = await screen.findByRole("checkbox", { name: `选择 ${id}` });
  fireEvent.click(checkbox);
  // 批量删除在选中行后出现在表格底部的操作条里。
  fireEvent.click(within(screen.getByRole("toolbar")).getByRole("button", { name: /^删\s*除$/ }));
  await screen.findByText("删除选中的 1 个用户？");
  expect(mocks.batchDeleteUsers).not.toHaveBeenCalled();
  return checkbox;
}

/** Popconfirm 的「删除」按钮渲染在 body 末尾的浮层里，排在操作条按钮之后。 */
function confirmButton(): HTMLElement {
  const buttons = screen.getAllByRole("button", { name: /^删\s*除$/ });
  return buttons[buttons.length - 1];
}

describe("User bulk-delete confirmation", () => {
  it("does not offer bulk deletion with no selection", async () => {
    render(
      <ConfigProvider theme={{ token: { motion: false } }}>
        <UserList />
      </ConfigProvider>,
    );
    await screen.findByRole("checkbox", { name: `选择 ${id}` });
    expect(screen.queryByRole("toolbar")).not.toBeInTheDocument();
    expect(mocks.batchDeleteUsers).not.toHaveBeenCalled();
  });

  it("cancel keeps selection and never calls the RPC or reload", async () => {
    const checkbox = await selectUser();
    const calls = mocks.list.mock.calls.length;
    fireEvent.click(screen.getByRole("button", { name: /^取\s*消$/ }));
    await waitFor(() =>
      expect(screen.queryByText("删除选中的 1 个用户？")).not.toBeInTheDocument(),
    );
    expect(mocks.batchDeleteUsers).not.toHaveBeenCalled();
    expect(mocks.list).toHaveBeenCalledTimes(calls);
    expect(checkbox).toBeChecked();
  });

  it("only confirmation submits bigint IDs once, then clears selection and reloads", async () => {
    let resolve!: (value: { deleted: number }) => void;
    mocks.batchDeleteUsers.mockReturnValue(
      new Promise<{ deleted: number }>((done) => {
        resolve = done;
      }),
    );
    const checkbox = await selectUser();
    const calls = mocks.list.mock.calls.length;
    const confirm = confirmButton();
    fireEvent.click(confirm);
    await waitFor(() =>
      expect(mocks.batchDeleteUsers).toHaveBeenCalledExactlyOnceWith({ ids: [id] }),
    );
    expect(checkbox).toBeChecked();
    fireEvent.click(confirm);
    expect(mocks.batchDeleteUsers).toHaveBeenCalledTimes(1);
    await act(async () => resolve({ deleted: 1 }));
    await waitFor(() => expect(mocks.list).toHaveBeenCalledTimes(calls + 1));
    expect(checkbox).not.toBeChecked();
    expect(mocks.success).toHaveBeenCalledWith("已删除 1 条");
  });

  it("failure keeps selection and does not reload or report success", async () => {
    mocks.batchDeleteUsers.mockRejectedValue(new Error("删除被拒绝"));
    const checkbox = await selectUser();
    const calls = mocks.list.mock.calls.length;
    const errorDialog = vi.spyOn(Modal, "error").mockImplementation(
      () =>
        ({
          destroy: () => undefined,
          update: () => undefined,
          then: () => Promise.resolve(),
        }) as unknown as ReturnType<typeof Modal.error>,
    );
    fireEvent.click(confirmButton());
    // 删除失败用弹窗写清原因（见 DESIGN.md「反馈」）。
    await waitFor(() =>
      expect(errorDialog).toHaveBeenCalledWith({ title: "无法删除", content: "删除被拒绝" }),
    );
    errorDialog.mockRestore();
    expect(checkbox).toBeChecked();
    expect(mocks.list).toHaveBeenCalledTimes(calls);
    expect(mocks.success).not.toHaveBeenCalled();
  });
});
