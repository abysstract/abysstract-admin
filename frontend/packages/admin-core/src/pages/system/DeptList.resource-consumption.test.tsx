import { render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { describe, expect, it, vi } from "vitest";
import { DeptSchema } from "@abysstract/admin-proto/abysstract/admin/v1/dept_pb";

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
    deptOperations: {
      ...actual.deptOperations,
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

import DeptList from "./DeptList";

describe("DeptList real Resource component integration", () => {
  it("renders the single generated flat Resource page", async () => {
    list.mockReset().mockResolvedValue({
      items: [
        create(DeptSchema, {
          id: 9007199254740993n,
          parentId: 0n,
          deptName: "真实部门",
          orderNum: 1,
        }),
        create(DeptSchema, {
          id: 2n,
          parentId: 9007199254740993n,
          deptName: "子部门",
          orderNum: 2,
        }),
      ],
      total: 2n,
      pageNum: 1,
      pageSize: 10,
    });

    render(<DeptList />);

    expect(document.querySelector('[data-resource-table="Dept"]')).toBeInTheDocument();
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("新建部门");
    await waitFor(() =>
      expect(list).toHaveBeenCalledWith({
        pageNum: 1,
        pageSize: 100,
        orders: [{ field: "orderNum", desc: false }],
      }),
    );
    expect(screen.getByText("真实部门")).toBeInTheDocument();
    expect(screen.getByText("子部门")).toBeInTheDocument();
  });
});
