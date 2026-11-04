import { render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { describe, expect, it, vi } from "vitest";
import { DictTypeSchema } from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";

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
    dictTypeOperations: {
      ...actual.dictTypeOperations,
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

vi.mock("./DictDataDrawer", () => ({ default: () => null }));

import DictList from "./DictList";

describe("DictList real Resource component integration", () => {
  it("renders ResourceTable and binds its operations.list facade", async () => {
    list.mockResolvedValue({
      items: [
        create(DictTypeSchema, {
          id: 9007199254740993n,
          dictName: "名称",
          dictType: "type",
          remark: "",
        }),
      ],
      total: 1n,
      pageNum: 1,
      pageSize: 10,
    });

    render(<DictList />);

    expect(screen.getByTestId("pro-table-toolbar")).toBeInTheDocument();
    await waitFor(() => expect(list).toHaveBeenCalledOnce());
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("新建字典");
  });
});
