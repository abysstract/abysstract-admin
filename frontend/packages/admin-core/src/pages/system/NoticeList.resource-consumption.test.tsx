import { render, screen, waitFor } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { describe, expect, it, vi } from "vitest";
import { NoticeSchema } from "@abysstract/admin-proto/abysstract/admin/v1/notice_pb";

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
    noticeOperations: {
      ...actual.noticeOperations,
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

import NoticeList from "./NoticeList";

describe("NoticeList real Resource component integration", () => {
  it("renders ResourceTable and binds the generated facade list operation", async () => {
    list.mockReset().mockResolvedValue({
      items: [
        create(NoticeSchema, {
          id: 9007199254740993n,
          noticeTitle: "真实页面公告",
          noticeType: 2,
          noticeContent: "内容",
        }),
      ],
      total: 1n,
      pageNum: 1,
      pageSize: 10,
    });

    render(<NoticeList />);

    expect(screen.getByTestId("pro-table-toolbar")).toBeInTheDocument();
    await waitFor(() => expect(list).toHaveBeenCalledOnce());
    expect(screen.getByTestId("pro-table-toolbar")).toHaveTextContent("新建公告");
    expect(screen.getByText("真实页面公告")).toBeInTheDocument();
    expect(screen.getByText("公告")).toBeInTheDocument();
  });
});
