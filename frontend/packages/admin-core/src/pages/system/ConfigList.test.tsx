import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { message } from "antd";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ConfigSchema } from "@abysstract/admin-proto/abysstract/admin/v1/config_pb";
import { configSchema } from "../../gen/resource/config.schema.gen";

const facade = vi.hoisted(() => ({
  permissions: new Set<string>(),
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  delete: vi.fn(),
  batchDelete: vi.fn(),
  keyOf: vi.fn(),
}));

vi.mock("../../runtime/access", () => ({
  useAccess: () => ({ has: (permission: string) => facade.permissions.has(permission) }),
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
const alternateRowKey = 9007199254740993n;

function allow(...permissions: string[]): void {
  facade.permissions.clear();
  for (const permission of permissions) facade.permissions.add(permission);
}

function resetFacade(): void {
  allow("system:config:list", "system:config:add", "system:config:edit", "system:config:remove");
  facade.list.mockReset().mockResolvedValue({
    items: [row],
    total: 1n,
    pageNum: 1,
    pageSize: 10,
  });
  facade.create.mockReset().mockResolvedValue(row);
  facade.update.mockReset().mockResolvedValue(row);
  facade.delete.mockReset().mockResolvedValue(undefined);
  facade.batchDelete.mockReset().mockResolvedValue(undefined);
  facade.keyOf.mockReset().mockImplementation((entity: typeof row) => entity.id);
  vi.mocked(message.success).mockClear();
  vi.mocked(message.error).mockClear();
}

async function openCreateForm(): Promise<void> {
  await screen.findByText("原名称");
  fireEvent.click(screen.getByRole("button", { name: "新建参数" }));
  await screen.findByRole("dialog", { name: "新建参数" });
}

async function openEditForm(): Promise<void> {
  await screen.findByRole("button", { name: "编辑" });
  fireEvent.click(screen.getByRole("button", { name: "编辑" }));
  await screen.findByRole("dialog", { name: "编辑参数" });
}

describe("ConfigList Resource consumption", () => {
  beforeEach(() => resetFacade());

  it("renders the generated Config schema and facade through ResourceTable", async () => {
    render(<ConfigList />);

    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("新建参数");
    expect(document.querySelector('[data-resource-table="Config"]')).toBeInTheDocument();
    await waitFor(() =>
      expect(facade.list).toHaveBeenCalledWith({
        pageNum: 1,
        pageSize: 10,
      }),
    );
    expect(screen.getByRole("columnheader", { name: "参数名称" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "参数键名" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "参数键值" })).toBeInTheDocument();
  });

  it("keeps ResourceTable keyword search and pagination query behavior", async () => {
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

  it("lets the generated schema own sorting capability rather than constructing protobuf orders", () => {
    // Config's current wire/schema contract exposes keyword search only. The
    // page therefore has no local sort list to drift from that contract.
    expect(configSortableFields()).toEqual([]);
  });

  it("uses ResourceForm for create and keeps configType server policy out of the UI", async () => {
    render(<ConfigList />);
    await openCreateForm();

    expect(screen.getByLabelText("参数名称")).toBeInTheDocument();
    expect(screen.getByLabelText("参数键名")).toBeInTheDocument();
    expect(screen.getByLabelText("参数键值")).toBeInTheDocument();
    expect(screen.queryByLabelText("configType")).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("参数名称"), { target: { value: "新名称" } });
    fireEvent.change(screen.getByLabelText("参数键名"), { target: { value: "new.key" } });
    fireEvent.change(screen.getByLabelText("参数键值"), { target: { value: "new-value" } });
    fireEvent.change(screen.getByLabelText("备注"), { target: { value: "新备注" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(facade.create).toHaveBeenCalledOnce());
    expect(facade.create).toHaveBeenCalledWith({
      configName: "新名称",
      configKey: "new.key",
      configValue: "new-value",
      configType: 0,
      remark: "新备注",
    });
    expect(message.success).toHaveBeenCalledWith("创建成功");
    await waitFor(() => expect(facade.list).toHaveBeenCalledTimes(2));
    expect(screen.queryByRole("dialog", { name: "新建参数" })).not.toBeInTheDocument();
  });

  it("keeps the create form open and shows the original failure message when create fails", async () => {
    facade.create.mockRejectedValueOnce(new Error("create failed"));
    render(<ConfigList />);
    await openCreateForm();

    fireEvent.change(screen.getByLabelText("参数名称"), { target: { value: "新名称" } });
    fireEvent.change(screen.getByLabelText("参数键名"), { target: { value: "new.key" } });
    fireEvent.change(screen.getByLabelText("参数键值"), { target: { value: "new-value" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(message.error).toHaveBeenCalledWith("create failed"));
    expect(screen.getByRole("dialog", { name: "新建参数" })).toBeInTheDocument();
    expect(facade.list).toHaveBeenCalledOnce();
  });

  it("uses an explicit bigint rowKey and submits only touched update presence", async () => {
    facade.keyOf.mockReturnValue(alternateRowKey);
    render(<ConfigList />);
    await openEditForm();

    expect(screen.queryByLabelText("参数键名")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("configType")).not.toBeInTheDocument();
    expect(screen.getByLabelText("参数名称")).toHaveValue("原名称");
    expect(screen.getByLabelText("参数键值")).toHaveValue("before-value");

    fireEvent.change(screen.getByLabelText("备注"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(facade.update).toHaveBeenCalledOnce());
    const submission = facade.update.mock.calls[0]?.[0] as {
      key: bigint;
      values: Record<string, unknown>;
      touched: ReadonlySet<string>;
      dirty: ReadonlySet<string>;
    };
    expect(submission.key).toBe(alternateRowKey);
    expect(submission.key).not.toBe(row.id);
    expect(typeof submission.key).toBe("bigint");
    expect(submission.values).toEqual({
      configName: "原名称",
      configValue: "before-value",
      remark: "",
    });
    expect(submission.values).not.toHaveProperty("configKey");
    expect(submission.values).not.toHaveProperty("id");
    expect(submission.touched).toEqual(new Set(["remark"]));
    expect(submission.dirty).toEqual(new Set(["remark"]));
    expect(message.success).toHaveBeenCalledWith("已保存");
    await waitFor(() => expect(facade.list).toHaveBeenCalledTimes(2));
  });

  it("keeps the update form open, reports failure, and restores submit state", async () => {
    facade.update
      .mockReset()
      .mockRejectedValueOnce(new Error("update failed"))
      .mockResolvedValueOnce(row);
    render(<ConfigList />);
    await openEditForm();

    fireEvent.change(screen.getByLabelText("参数名称"), { target: { value: "失败名称" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(message.error).toHaveBeenCalledWith("update failed"));
    expect(screen.getByRole("dialog", { name: "编辑参数" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "提交" })).not.toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "提交" }));
    await waitFor(() => expect(facade.update).toHaveBeenCalledTimes(2));
    expect(message.success).toHaveBeenCalledWith("已保存");
  });

  it("preserves touched presence when a field is reverted to its baseline", async () => {
    render(<ConfigList />);
    await openEditForm();

    const name = screen.getByLabelText("参数名称");
    fireEvent.change(name, { target: { value: "临时名称" } });
    fireEvent.change(name, { target: { value: "原名称" } });
    fireEvent.click(screen.getByRole("button", { name: "提交" }));

    await waitFor(() => expect(facade.update).toHaveBeenCalledOnce());
    const submission = facade.update.mock.calls[0]?.[0] as {
      touched: ReadonlySet<string>;
      dirty: ReadonlySet<string>;
    };
    expect(submission.touched).toEqual(new Set(["configName"]));
    expect(submission.dirty).toEqual(new Set());
  });

  it("uses the typed delete facade, confirms, reloads, and preserves success/error handling", async () => {
    render(<ConfigList />);
    await screen.findByText("原名称");

    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(facade.delete).toHaveBeenCalledWith(7n));
    await waitFor(() => expect(facade.list).toHaveBeenCalledTimes(2));
    expect(message.success).toHaveBeenCalledWith("已删除");

    resetFacade();
    facade.delete.mockRejectedValueOnce(new Error("delete failed"));
    render(<ConfigList />);
    await screen.findByText("原名称");
    fireEvent.click(screen.getAllByRole("button", { name: "删除" })[0]);
    fireEvent.click(screen.getByRole("button", { name: "确认删除" }));
    await waitFor(() => expect(message.error).toHaveBeenCalledWith("delete failed"));
  });

  it("uses the typed batchDelete facade and reloads after success", async () => {
    render(<ConfigList />);
    await screen.findByText("原名称");

    fireEvent.click(screen.getByRole("checkbox", { name: "选择 7" }));
    fireEvent.click(screen.getByRole("button", { name: "批量删除" }));
    fireEvent.click(screen.getByRole("button", { name: "确认批量删除" }));

    await waitFor(() => expect(facade.batchDelete).toHaveBeenCalledWith([7n]));
    await waitFor(() => expect(facade.list).toHaveBeenCalledTimes(2));
    expect(message.success).toHaveBeenCalledWith("已删除");
  });

  it("resets the form when its modal is closed and reopened", async () => {
    render(<ConfigList />);
    await openEditForm();

    fireEvent.change(screen.getByLabelText("参数名称"), { target: { value: "未保存" } });
    fireEvent.click(screen.getByRole("button", { name: "关闭" }));
    expect(screen.queryByRole("dialog", { name: "编辑参数" })).not.toBeInTheDocument();

    await openEditForm();
    expect(screen.getByLabelText("参数名称")).toHaveValue("原名称");
  });

  it("shows list errors with a retry action", async () => {
    facade.list
      .mockReset()
      .mockRejectedValueOnce(new Error("list failed"))
      .mockResolvedValueOnce({ items: [row], total: 1n, pageNum: 1, pageSize: 10 });
    render(<ConfigList />);

    expect(await screen.findByRole("alert")).toHaveTextContent("list failed");
    expect(message.error).toHaveBeenCalledWith("list failed");
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    await waitFor(() => expect(facade.list).toHaveBeenCalledTimes(2));
    expect(await screen.findByText("原名称")).toBeInTheDocument();
  });

  it.each([
    {
      label: "add",
      permissions: ["system:config:list", "system:config:edit", "system:config:remove"],
      hidden: ["新建参数"],
    },
    {
      label: "edit",
      permissions: ["system:config:list", "system:config:add", "system:config:remove"],
      hidden: ["编辑"],
    },
    {
      label: "remove",
      permissions: ["system:config:list", "system:config:add", "system:config:edit"],
      hidden: ["删除", "批量删除"],
    },
  ])(
    "hides only the $label action through ResourceTable access",
    async ({ permissions, hidden }) => {
      allow(...permissions);
      render(<ConfigList />);

      await screen.findByText("原名称");
      await waitFor(() => expect(facade.list).toHaveBeenCalledOnce());
      for (const buttonName of hidden) {
        expect(screen.queryByRole("button", { name: buttonName })).not.toBeInTheDocument();
      }
    },
  );

  it("hides every standard action when only list permission is available", async () => {
    allow("system:config:list");
    render(<ConfigList />);

    await waitFor(() => expect(facade.list).toHaveBeenCalledOnce());
    expect(screen.queryByRole("button", { name: "新建参数" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "编辑" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "删除" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "批量删除" })).not.toBeInTheDocument();
  });

  it("does not list when list permission is missing", async () => {
    allow("system:config:add", "system:config:edit", "system:config:remove");
    render(<ConfigList />);

    expect(await screen.findByRole("alert")).toHaveTextContent(/system:config:list/);
    expect(facade.list).not.toHaveBeenCalled();
  });
});

function configSortableFields(): string[] {
  return configSchema.fields.filter((field) => field.sortable).map((field) => field.name);
}
