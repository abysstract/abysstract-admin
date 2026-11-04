import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { message } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { NoticeSchema } from "@abysstract/admin-proto/abysstract/admin/v1/notice_pb";
import { Status } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";

const mocks = vi.hoisted(() => ({
  permissions: new Set<string>(),
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  delete: vi.fn(),
  batchDelete: vi.fn(),
  keyOf: vi.fn(),
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
    noticeOperations: {
      ...actual.noticeOperations,
      list: mocks.list,
      create: mocks.create,
      update: mocks.update,
      delete: mocks.delete,
      batchDelete: mocks.batchDelete,
      keyOf: mocks.keyOf,
    },
  };
  return {
    useSystemClients: () => ({ commonExport: {} }),
    useSystemOperations: () => operations,
  };
});

import NoticeList from "./NoticeList";

const row = create(NoticeSchema, {
  id: 7n,
  noticeTitle: "原公告",
  noticeType: 1,
  noticeContent: "原内容",
  status: Status.ENABLED,
  remark: "原备注",
});

function resetMocks(): void {
  mocks.permissions.clear();
  for (const permission of [
    "system:notice:list",
    "system:notice:add",
    "system:notice:edit",
    "system:notice:remove",
  ]) {
    mocks.permissions.add(permission);
  }
  mocks.list.mockReset().mockResolvedValue({ items: [row], total: 1n, pageNum: 1, pageSize: 10 });
  mocks.create.mockReset().mockResolvedValue(row);
  mocks.update.mockReset().mockResolvedValue(row);
  mocks.delete.mockReset().mockResolvedValue(undefined);
  mocks.batchDelete.mockReset().mockResolvedValue(undefined);
  mocks.keyOf.mockReset().mockImplementation((entity: typeof row) => entity.id);
  vi.mocked(message.success).mockClear();
  vi.mocked(message.error).mockClear();
}

async function openCreateForm(): Promise<void> {
  await screen.findByText("原公告");
  fireEvent.click(screen.getByRole("button", { name: "新建公告" }));
  await screen.findByRole("dialog", { name: "新建公告" });
}

async function openEditForm(): Promise<void> {
  await screen.findByRole("button", { name: "编辑" });
  fireEvent.click(screen.getByRole("button", { name: "编辑" }));
  await screen.findByRole("dialog", { name: "编辑公告" });
}

describe("NoticeList Resource migration", () => {
  beforeEach(() => resetMocks());

  it("consumes the generated Notice schema and operations facade through ResourceTable", async () => {
    render(<NoticeList />);

    expect(document.querySelector('[data-resource-table="Notice"]')).toBeInTheDocument();
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("新建公告");
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("批量删除");
    await waitFor(() =>
      expect(mocks.list).toHaveBeenCalledWith({
        pageNum: 1,
        pageSize: 10,
      }),
    );
    expect(screen.getByRole("columnheader", { name: "标题" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "类型" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "状态" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "创建时间" })).toBeInTheDocument();
  });

  it("uses generated searchable capability for keyword pagination rather than page-local PageRequest", async () => {
    render(<NoticeList />);
    await screen.findByText("原公告");

    fireEvent.change(screen.getByRole("textbox", { name: "关键词" }), {
      target: { value: "公告" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));
    await waitFor(() =>
      expect(mocks.list).toHaveBeenLastCalledWith({
        pageNum: 1,
        pageSize: 10,
        keyword: "公告",
      }),
    );

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await waitFor(() =>
      expect(mocks.list).toHaveBeenLastCalledWith({
        pageNum: 2,
        pageSize: 10,
        keyword: "公告",
      }),
    );
  });

  it("keeps sorting capability owned by the generated schema", async () => {
    const { noticeSchema } = await import("../../gen/resource/notice.schema.gen");
    expect(noticeSchema.fields.filter((field) => field.sortable)).toEqual([]);
  });

  it("uses ResourceForm for create with typed notice_type options and generated validation", async () => {
    render(<NoticeList />);
    await openCreateForm();

    expect(screen.getByLabelText("标题")).toBeInTheDocument();
    expect(screen.getByLabelText("类型")).toBeInTheDocument();
    expect(screen.getByLabelText("内容")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "类型" })).toBeInTheDocument();
    fireEvent.mouseDown(screen.getByRole("combobox", { name: "类型" }));
    expect(screen.getByText("公告")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "提交" }));
    expect(mocks.create).not.toHaveBeenCalled();
    expect(screen.getAllByText(/必填/).length).toBeGreaterThan(0);

    fireEvent.change(screen.getByLabelText("标题"), { target: { value: "x".repeat(101) } });
    fireEvent.change(screen.getByLabelText("内容"), { target: { value: "内容" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));
    expect(mocks.create).not.toHaveBeenCalled();
    expect(screen.getByText(/100|maxLen|长度/)).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("标题"), { target: { value: "新公告" } });
    fireEvent.change(screen.getByLabelText("内容"), { target: { value: "新内容" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(mocks.create).toHaveBeenCalledOnce());
    expect(mocks.create).toHaveBeenCalledWith({
      noticeTitle: "新公告",
      noticeType: 1,
      noticeContent: "新内容",
      remark: "",
    });
    expect(message.success).toHaveBeenCalledWith("创建成功");
    await waitFor(() => expect(mocks.list).toHaveBeenCalledTimes(2));
  });

  it("uses generated update presence and preserves an explicit empty remark", async () => {
    render(<NoticeList />);
    await openEditForm();

    fireEvent.change(screen.getByLabelText("备注"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(mocks.update).toHaveBeenCalledOnce());
    const submission = mocks.update.mock.calls[0]?.[0] as {
      key: bigint;
      values: {
        noticeTitle: string;
        noticeType: number;
        noticeContent: string;
        remark: string;
      };
      touched: ReadonlySet<string>;
      dirty: ReadonlySet<string>;
    };
    expect(submission.key).toBe(7n);
    expect(submission.values).toEqual({
      noticeTitle: "原公告",
      noticeType: 1,
      noticeContent: "原内容",
      remark: "",
    });
    expect(submission.touched).toEqual(new Set(["remark"]));
    expect(submission.dirty).toEqual(new Set(["remark"]));
    expect(message.success).toHaveBeenCalledWith("已保存");
  });

  it("uses standard generated delete and batch-delete facade actions", async () => {
    render(<NoticeList />);
    await screen.findByText("原公告");

    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(mocks.delete).toHaveBeenCalledWith(7n));

    const selection = screen.getAllByRole("checkbox")[0];
    fireEvent.click(selection);
    fireEvent.click(screen.getByRole("button", { name: "批量删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认批量删除" }));
    await waitFor(() => expect(mocks.batchDelete).toHaveBeenCalledWith([7n]));
  });

  it("shows list errors and retries through ResourceTable", async () => {
    mocks.list
      .mockReset()
      .mockRejectedValueOnce(new Error("list failed"))
      .mockResolvedValueOnce({ items: [row], total: 1n, pageNum: 1, pageSize: 10 });
    render(<NoticeList />);

    expect(await screen.findByRole("alert")).toHaveTextContent("list failed");
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => expect(mocks.list).toHaveBeenCalledTimes(2));
    expect(await screen.findByText("原公告")).toBeInTheDocument();
  });

  it("hides standard actions from generated permission decisions", async () => {
    mocks.permissions.delete("system:notice:add");
    mocks.permissions.delete("system:notice:edit");
    mocks.permissions.delete("system:notice:remove");
    render(<NoticeList />);

    await waitFor(() => expect(mocks.list).toHaveBeenCalledOnce());
    expect(screen.queryByRole("button", { name: "新建公告" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "编辑" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "删除" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "批量删除" })).not.toBeInTheDocument();
  });
});
