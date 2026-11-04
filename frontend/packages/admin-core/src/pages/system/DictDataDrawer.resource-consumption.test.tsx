import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { create } from "@bufbuild/protobuf";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DictDataSchema } from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";

const facade = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  delete: vi.fn(),
  batchDelete: vi.fn(),
  keyOf: vi.fn(),
}));

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
    dictDataOperations: {
      ...actual.dictDataOperations,
      list: facade.list,
      create: facade.create,
      update: facade.update,
      delete: facade.delete,
      batchDelete: facade.batchDelete,
      keyOf: facade.keyOf,
    },
  };
  return {
    useSystemClients: () => ({ commonExport: {} }),
    useSystemOperations: () => operations,
  };
});

import DictDataDrawer from "./DictDataDrawer";

const row = create(DictDataSchema, {
  id: 7n,
  dictSort: 3,
  dictLabel: "标签",
  dictValue: "value",
  dictType: "notice",
  isDefault: 0,
  remark: "",
});
const nonEntityKey = 9007199254740993n;

beforeEach(() => {
  facade.list.mockReset().mockResolvedValue({ items: [row], total: 1n, pageNum: 1, pageSize: 10 });
  facade.create.mockReset().mockResolvedValue(row);
  facade.update.mockReset().mockResolvedValue(row);
  facade.delete.mockReset().mockResolvedValue(undefined);
  facade.batchDelete.mockReset().mockResolvedValue(undefined);
  facade.keyOf.mockReset().mockReturnValue(nonEntityKey);
});

describe("DictDataDrawer real Resource consumption gate", () => {
  it("renders ResourceTable and sends the relation filter through operations.list", async () => {
    render(<DictDataDrawer dictType="notice" open onClose={() => undefined} />);

    await waitFor(() => expect(facade.list).toHaveBeenCalledOnce());
    expect(facade.list).toHaveBeenCalledWith({
      pageNum: 1,
      pageSize: 10,
      orders: [{ field: "dictSort", desc: false }],
      filters: { dictType: "notice" },
    });
    expect(document.querySelector('[data-resource-table="DictData"]')).toBeInTheDocument();
  });

  it("passes ResourceTable's bigint rowKey into ResourceForm update, not entity.id", async () => {
    render(<DictDataDrawer dictType="notice" open onClose={() => undefined} />);
    await screen.findByRole("button", { name: "编辑" }, { timeout: 5000 });

    fireEvent.click(screen.getByRole("button", { name: "编辑" }));
    fireEvent.change(screen.getByLabelText("标签"), { target: { value: "新标签" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(facade.update).toHaveBeenCalledOnce());
    const submission = facade.update.mock.calls[0]?.[0] as {
      key: bigint;
      values: Record<string, unknown>;
    };
    expect(submission.key).toBe(nonEntityKey);
    expect(submission.key).not.toBe(row.id);
    expect(typeof submission.key).toBe("bigint");
    expect(submission.values).not.toHaveProperty("id");
    await waitFor(() => expect(facade.list).toHaveBeenCalledTimes(2));
  });
});
