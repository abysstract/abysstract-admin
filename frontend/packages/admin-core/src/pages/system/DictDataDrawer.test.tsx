import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { create } from "@bufbuild/protobuf";
import { message } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { DictData } from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { DictDataSchema } from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { dictDataSchema } from "../../gen/resource/dict-data.schema.gen";

vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  const message = { success: vi.fn(), error: vi.fn() };
  return {
    ...actual,
    Drawer: (props: Record<string, unknown>) => <div>{props.children as ReactNode}</div>,
    App: Object.assign(({ children }: { children?: ReactNode }) => <>{children}</>, {
      useApp: () => ({ message }),
    }),
    message,
  };
});

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
    dictDataOperations: {
      ...actual.dictDataOperations,
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

import DictDataDrawer from "./DictDataDrawer";

const entity = create(DictDataSchema, {
  id: 11n,
  dictSort: 9,
  dictLabel: "启用",
  dictValue: "1",
  dictType: "sys_yes_no",
  cssClass: "",
  listClass: "primary",
  isDefault: 1,
  remark: "keep",
});
const alternateRowKey = 9007199254740993n;

function allow(...permissions: string[]): void {
  mocks.permissions.clear();
  for (const permission of permissions) mocks.permissions.add(permission);
}

function resetFacade(): void {
  allow("system:dict:list", "system:dict:add", "system:dict:edit", "system:dict:remove");
  mocks.list.mockReset().mockResolvedValue({
    items: [entity],
    total: 1n,
    pageNum: 1,
    pageSize: 10,
  });
  mocks.create.mockReset().mockResolvedValue(entity);
  mocks.update.mockReset().mockResolvedValue(entity);
  mocks.delete.mockReset().mockResolvedValue(undefined);
  mocks.batchDelete.mockReset().mockResolvedValue(undefined);
  mocks.keyOf.mockReset().mockImplementation((row: DictData) => row.id);
  vi.mocked(message.success).mockClear();
  vi.mocked(message.error).mockClear();
}

describe("DictDataDrawer Resource consumption", () => {
  beforeEach(() => resetFacade());

  it("renders the generated schema and operations facade through a real ResourceTable", async () => {
    render(<DictDataDrawer dictType="sys_yes_no" open onClose={() => undefined} />);

    expect(dictDataSchema.name).toBe("DictData");
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("新建");
    await waitFor(() =>
      expect(mocks.list).toHaveBeenCalledWith({
        pageNum: 1,
        pageSize: 10,
        orders: [{ field: "dictSort", desc: false }],
        filters: { dictType: "sys_yes_no" },
      }),
    );
    expect(screen.getByRole("columnheader", { name: "标签" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "键值" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "排序" })).toBeInTheDocument();
  });

  it("preserves ResourceTable pagination and schema sort behavior", async () => {
    render(<DictDataDrawer dictType="sys_yes_no" open onClose={() => undefined} />);
    await screen.findByText("启用");

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await waitFor(() =>
      expect(mocks.list).toHaveBeenLastCalledWith({
        pageNum: 2,
        pageSize: 10,
        orders: [{ field: "dictSort", desc: false }],
        filters: { dictType: "sys_yes_no" },
      }),
    );

    fireEvent.click(screen.getByRole("columnheader", { name: "排序" }).querySelector("button")!);
    await waitFor(() =>
      expect(mocks.list).toHaveBeenLastCalledWith({
        pageNum: 2,
        pageSize: 10,
        orders: [{ field: "dictSort", desc: true }],
        filters: { dictType: "sys_yes_no" },
      }),
    );
  });

  it("uses ResourceForm for create, preserves dictType context, and reloads after success", async () => {
    render(<DictDataDrawer dictType="sys_yes_no" open onClose={() => undefined} />);
    await screen.findByText("启用");

    fireEvent.click(screen.getByRole("button", { name: "新建" }));
    fireEvent.change(screen.getByLabelText("标签"), { target: { value: "新标签" } });
    fireEvent.change(screen.getByLabelText("键值"), { target: { value: "new" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(mocks.create).toHaveBeenCalledOnce());
    expect(mocks.create).toHaveBeenCalledWith(
      expect.objectContaining({
        dictType: "sys_yes_no",
        dictLabel: "新标签",
        dictValue: "new",
        dictSort: 0,
        isDefault: 0,
        remark: "",
      }),
    );
    expect(mocks.create.mock.calls[0]?.[0]).not.toHaveProperty("id");
    expect(message.success).toHaveBeenCalledWith("创建成功");
    await waitFor(() => expect(mocks.list).toHaveBeenCalledTimes(2));
  });

  it("uses ResourceForm update with explicit bigint rowKey and presence-aware values", async () => {
    mocks.keyOf.mockReturnValue(alternateRowKey);
    render(<DictDataDrawer dictType="sys_yes_no" open onClose={() => undefined} />);
    await screen.findByText("启用");

    fireEvent.click(screen.getByRole("button", { name: "编辑" }));
    fireEvent.change(screen.getByLabelText("标签"), { target: { value: "已改" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(mocks.update).toHaveBeenCalledOnce());
    const submission = mocks.update.mock.calls[0]?.[0] as {
      key: bigint;
      values: Record<string, unknown>;
      touched: ReadonlySet<string>;
      dirty: ReadonlySet<string>;
    };
    expect(submission.key).toBe(alternateRowKey);
    expect(typeof submission.key).toBe("bigint");
    expect(submission.key).not.toBe(entity.id);
    expect(submission.values).not.toHaveProperty("id");
    expect(submission.values).not.toHaveProperty("dictType");
    expect(submission.values).not.toHaveProperty("createTime");
    expect(submission.touched).toEqual(new Set(["dictLabel"]));
    expect(submission.dirty).toEqual(new Set(["dictLabel"]));
    expect(message.success).toHaveBeenCalledWith("已保存");
    await waitFor(() => expect(mocks.list).toHaveBeenCalledTimes(2));
  });

  it("delegates single and batch delete, reloads, and keeps remove permission boundaries", async () => {
    const view = render(<DictDataDrawer dictType="sys_yes_no" open onClose={() => undefined} />);
    await screen.findByText("启用");

    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(mocks.delete).toHaveBeenCalledWith(entity.id));
    await waitFor(() => expect(mocks.list).toHaveBeenCalledTimes(2));

    fireEvent.click(screen.getByRole("checkbox", { name: `选择 ${entity.id}` }));
    fireEvent.click(screen.getByRole("button", { name: "批量删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认批量删除" }));
    await waitFor(() => expect(mocks.batchDelete).toHaveBeenCalledWith([entity.id]));
    await waitFor(() => expect(mocks.list).toHaveBeenCalledTimes(3));

    view.unmount();
    resetFacade();
    allow("system:dict:list", "system:dict:add", "system:dict:edit");
    render(<DictDataDrawer dictType="sys_yes_no" open onClose={() => undefined} />);
    await screen.findByText("启用");
    expect(screen.queryByRole("button", { name: "删除" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "批量删除" })).not.toBeInTheDocument();
  });

  it("resets filters and errors when the drawer closes, reopens, or changes dictType", async () => {
    mocks.list.mockRejectedValueOnce(new Error("filter failed"));
    const view = render(<DictDataDrawer dictType="first" open onClose={() => undefined} />);
    expect(await screen.findByRole("alert")).toHaveTextContent("filter failed");

    view.rerender(<DictDataDrawer dictType="first" open={false} onClose={() => undefined} />);
    view.rerender(<DictDataDrawer dictType="second" open onClose={() => undefined} />);
    await waitFor(() =>
      expect(mocks.list).toHaveBeenLastCalledWith({
        pageNum: 1,
        pageSize: 10,
        orders: [{ field: "dictSort", desc: false }],
        filters: { dictType: "second" },
      }),
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
