import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { message } from "antd";
import { create } from "@bufbuild/protobuf";
import { describe, expect, it, vi } from "vitest";
import { DictTypeSchema } from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";

const mocks = vi.hoisted(() => ({
  permissions: new Set<string>(),
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  delete: vi.fn(),
  batchDelete: vi.fn(),
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

vi.mock("../../system/hooks", async () => {
  const { createSystemOperations } = await import("../../system/operations");
  const actual = createSystemOperations({} as never);
  const operations = {
    ...actual,
    dictTypeOperations: {
      ...actual.dictTypeOperations,
      list: mocks.list,
      create: mocks.create,
      update: mocks.update,
      delete: mocks.delete,
      batchDelete: mocks.batchDelete,
    },
  };
  return {
    useSystemClients: () => ({ commonExport: {} }),
    useSystemOperations: () => operations,
  };
});

vi.mock("./DictDataDrawer", () => ({
  default: ({ dictType, open }: { dictType: string; open: boolean }) =>
    open ? <aside data-testid="dict-data-drawer">{dictType}</aside> : null,
}));

import DictList from "./DictList";

const row = create(DictTypeSchema, {
  id: 7n,
  dictName: "原名称",
  dictType: "fixed-code",
  remark: "before",
});

function resetFacade() {
  mocks.permissions.clear();
  for (const permission of [
    "system:dict:list",
    "system:dict:add",
    "system:dict:edit",
    "system:dict:remove",
  ]) {
    mocks.permissions.add(permission);
  }
  mocks.list.mockReset().mockResolvedValue({
    items: [row],
    total: 1n,
    pageNum: 1,
    pageSize: 10,
  });
  mocks.create.mockReset().mockResolvedValue(row);
  mocks.update.mockReset().mockResolvedValue(row);
  mocks.delete.mockReset().mockResolvedValue(undefined);
  mocks.batchDelete.mockReset().mockResolvedValue(undefined);
  vi.mocked(message.success).mockClear();
  vi.mocked(message.error).mockClear();
}

describe("DictList Resource consumption", () => {
  it("renders the generated schema/facade through ResourceTable", async () => {
    resetFacade();
    render(<DictList />);

    expect(screen.getByTestId("pro-table-toolbar")).toBeInTheDocument();
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("新建字典");
    await waitFor(() =>
      expect(mocks.list).toHaveBeenCalledWith({
        pageNum: 1,
        pageSize: 10,
      }),
    );
    expect(screen.getByText("字典名称")).toBeInTheDocument();
  });

  it("keeps the old-page visible columns and hides id/remark/updateTime", async () => {
    resetFacade();
    render(<DictList />);

    await waitFor(() => expect(mocks.list).toHaveBeenCalledOnce());
    // 页面可见列：字典名称/字典类型/状态/创建时间；id/备注/更新时间不渲染。
    expect(screen.getByRole("columnheader", { name: "字典名称" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "字典类型" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "状态" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "创建时间" })).toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "id" })).not.toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "remark" })).not.toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: "updateTime" })).not.toBeInTheDocument();
  });

  it("uses ResourceForm for update without CreateOnly dictType and preserves bigint presence", async () => {
    resetFacade();
    render(<DictList />);

    await screen.findByText("编辑");
    fireEvent.click(screen.getByRole("button", { name: "编辑" }));
    expect(screen.getByRole("dialog", { name: "编辑字典类型" })).toBeInTheDocument();
    expect(screen.queryByLabelText("字典类型")).not.toBeInTheDocument();
    const name = screen.getByLabelText("字典名称");
    fireEvent.change(name, { target: { value: "新名称" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(mocks.update).toHaveBeenCalledOnce());
    const submission = mocks.update.mock.calls[0]?.[0] as {
      key: bigint;
      values: { dictName: string; remark: string };
      touched: ReadonlySet<string>;
      dirty: ReadonlySet<string>;
    };
    expect(submission.key).toBe(7n);
    expect(submission.values).toEqual({ dictName: "新名称", remark: "before" });
    expect(submission.touched).toEqual(new Set(["dictName"]));
    expect(submission.dirty).toEqual(new Set(["dictName"]));
  });

  it("derives standard create/edit/delete visibility from Resource permissions", async () => {
    resetFacade();
    mocks.permissions.delete("system:dict:add");
    mocks.permissions.delete("system:dict:edit");
    mocks.permissions.delete("system:dict:remove");
    render(<DictList />);

    await waitFor(() => expect(mocks.list).toHaveBeenCalledOnce());
    expect(screen.queryByRole("button", { name: "新建字典" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "编辑" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "删除" })).not.toBeInTheDocument();
  });

  it("keeps the DictDataDrawer custom row action and standard delete facade action", async () => {
    resetFacade();
    render(<DictList />);

    await screen.findByText("数据");
    fireEvent.click(screen.getByRole("button", { name: "数据" }));
    expect(screen.getByTestId("dict-data-drawer")).toHaveTextContent("fixed-code");

    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(mocks.delete).toHaveBeenCalledWith(7n));
  });

  it("keeps create success and reload behavior in the page form override", async () => {
    resetFacade();
    render(<DictList />);

    fireEvent.click(screen.getByRole("button", { name: "新建字典" }));
    fireEvent.change(screen.getByLabelText("字典名称"), { target: { value: "新字典" } });
    fireEvent.change(screen.getByLabelText("字典类型"), { target: { value: "new-code" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(mocks.create).toHaveBeenCalledOnce());
    expect(message.success).toHaveBeenCalledWith("创建成功");
    expect(mocks.create).toHaveBeenCalledWith({
      dictName: "新字典",
      dictType: "new-code",
      remark: "",
    });
    await waitFor(() => expect(mocks.list.mock.calls.length).toBeGreaterThan(1));
  });
});
