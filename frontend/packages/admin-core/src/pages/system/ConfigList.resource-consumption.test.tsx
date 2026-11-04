import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ConfigSchema } from "@abysstract/admin-proto/abysstract/admin/v1/config_pb";
import { configSchema } from "../../gen/resource/config.schema.gen";

const facade = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  delete: vi.fn(),
  batchDelete: vi.fn(),
  keyOf: vi.fn(),
}));

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
    configOperations: {
      ...actual.configOperations,
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

import ConfigList from "./ConfigList";

const row = create(ConfigSchema, {
  id: 7n,
  configName: "原名称",
  configKey: "fixed.key",
  configValue: "before-value",
  configType: 0,
  remark: "before",
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

describe("ConfigList real Resource component integration", () => {
  it("renders the generated schema through a real ResourceTable and calls operations.list", async () => {
    render(<ConfigList />);

    expect(configSchema.name).toBe("Config");
    expect(document.querySelector('[data-resource-table="Config"]')).toBeInTheDocument();
    await waitFor(() => expect(facade.list).toHaveBeenCalledOnce());
    expect(facade.list).toHaveBeenCalledWith({ pageNum: 1, pageSize: 10 });
    expect(screen.getByRole("columnheader", { name: "参数名称" })).toBeInTheDocument();
  });

  it("keeps ResourceTable search and pagination on the typed facade", async () => {
    render(<ConfigList />);
    await screen.findByText("原名称");

    fireEvent.change(screen.getByRole("textbox", { name: "关键词" }), {
      target: { value: "fixed" },
    });
    fireEvent.click(screen.getByRole("button", { name: "查询" }));
    await waitFor(() =>
      expect(facade.list).toHaveBeenLastCalledWith({
        pageNum: 1,
        pageSize: 10,
        keyword: "fixed",
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    await waitFor(() =>
      expect(facade.list).toHaveBeenLastCalledWith({
        pageNum: 2,
        pageSize: 10,
        keyword: "fixed",
      }),
    );
  });

  it("renders ResourceForm create/update branches and keeps CreateOnly configKey create-only", async () => {
    render(<ConfigList />);
    await screen.findByText("原名称");

    fireEvent.click(screen.getByRole("button", { name: "新建参数" }));
    expect(screen.getByRole("dialog", { name: "新建参数" })).toBeInTheDocument();
    expect(screen.getByLabelText("参数键名")).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("参数名称"), { target: { value: "新名称" } });
    fireEvent.change(screen.getByLabelText("参数键名"), { target: { value: "new.key" } });
    fireEvent.change(screen.getByLabelText("参数键值"), { target: { value: "new-value" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));
    await waitFor(() => expect(facade.create).toHaveBeenCalledOnce());
    await waitFor(() => expect(facade.list).toHaveBeenCalledTimes(2));

    fireEvent.click(screen.getByRole("button", { name: "编辑" }));
    expect(screen.queryByLabelText("参数键名")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("id")).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("备注"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));
    await waitFor(() => expect(facade.update).toHaveBeenCalledOnce());

    const submission = facade.update.mock.calls[0]?.[0] as {
      key: bigint;
      values: Record<string, unknown>;
      touched: ReadonlySet<string>;
      dirty: ReadonlySet<string>;
    };
    expect(submission.key).toBe(nonEntityKey);
    expect(submission.key).not.toBe(row.id);
    expect(submission.values).not.toHaveProperty("configKey");
    expect(submission.values).not.toHaveProperty("id");
    expect(submission.values.remark).toBe("");
    expect(submission.touched).toEqual(new Set(["remark"]));
    expect(submission.dirty).toEqual(new Set(["remark"]));
    await waitFor(() => expect(facade.list).toHaveBeenCalledTimes(3));
  });

  it("routes single and batch delete through typed facades and reloads after cache invalidation", async () => {
    render(<ConfigList />);
    await screen.findByText("原名称");

    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(facade.delete).toHaveBeenCalledWith(nonEntityKey));
    await waitFor(() => expect(facade.list).toHaveBeenCalledTimes(2));

    fireEvent.click(screen.getByRole("checkbox", { name: `选择 ${nonEntityKey}` }));
    fireEvent.click(screen.getByRole("button", { name: "批量删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认批量删除" }));
    await waitFor(() => expect(facade.batchDelete).toHaveBeenCalledWith([nonEntityKey]));
    await waitFor(() => expect(facade.list).toHaveBeenCalledTimes(3));
  });

  it("shows ResourceTable list errors and retries through the facade", async () => {
    facade.list
      .mockReset()
      .mockRejectedValueOnce(new Error("permission denied"))
      .mockResolvedValueOnce({ items: [row], total: 1n, pageNum: 1, pageSize: 10 });
    render(<ConfigList />);

    expect(await screen.findByRole("alert")).toHaveTextContent("permission denied");
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => expect(facade.list).toHaveBeenCalledTimes(2));
    expect(await screen.findByText("原名称")).toBeInTheDocument();
  });
});
