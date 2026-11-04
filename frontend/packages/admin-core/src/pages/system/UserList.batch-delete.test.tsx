import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { ConfigProvider } from "antd";
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
vi.mock("../../runtime/message", () => ({
  useAppMessage: () => ({
    success: mocks.success,
    error: mocks.error,
    warning: vi.fn(),
    info: vi.fn(),
  }),
  formatErrorMessage: (error: unknown, fallback = "操作失败") =>
    error instanceof Error ? error.message || fallback : fallback,
}));
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
    pageSize: 10,
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
  fireEvent.click(screen.getByRole("button", { name: /批量删除（1）/ }));
  await screen.findByText("确认删除选中的 1 个用户？");
  expect(mocks.batchDeleteUsers).not.toHaveBeenCalled();
  return checkbox;
}

describe("User bulk-delete confirmation", () => {
  it("does not offer bulk deletion with no selection", async () => {
    render(
      <ConfigProvider theme={{ token: { motion: false } }}>
        <UserList />
      </ConfigProvider>,
    );
    await screen.findByRole("checkbox", { name: `选择 ${id}` });
    expect(screen.queryByRole("button", { name: /批量删除/ })).not.toBeInTheDocument();
    expect(mocks.batchDeleteUsers).not.toHaveBeenCalled();
  });

  it("cancel keeps selection and never calls the RPC or reload", async () => {
    const checkbox = await selectUser();
    const calls = mocks.list.mock.calls.length;
    fireEvent.click(screen.getByRole("button", { name: /取\s*消/ }));
    await waitFor(() =>
      expect(screen.queryByText("确认删除选中的 1 个用户？")).not.toBeInTheDocument(),
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
    const confirm = screen.getByRole("button", { name: "确认删除" });
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
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(mocks.error).toHaveBeenCalledWith("删除被拒绝"));
    expect(checkbox).toBeChecked();
    expect(mocks.list).toHaveBeenCalledTimes(calls);
    expect(mocks.success).not.toHaveBeenCalled();
  });
});
