import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { create } from "@bufbuild/protobuf";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  AccessLogSchema,
  AccessLogSinkStatusSchema,
  GetAccessLogStatusResponseSchema,
  ListAccessLogsResponseSchema,
} from "@abysstract/admin-proto/abysstract/admin/v1/log_pb";
import { PageInfoSchema } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";

const mocks = vi.hoisted(() => ({
  getAccessLogStatus: vi.fn(),
  listAccessLogs: vi.fn(),
}));

vi.mock("../../system/hooks", () => ({
  useSystemClients: () => ({
    log: { getAccessLogStatus: mocks.getAccessLogStatus, listAccessLogs: mocks.listAccessLogs },
  }),
}));

import AccessLogList from "./AccessLogList";

const readable = create(GetAccessLogStatusResponseSchema, {
  enabled: true,
  readable: true,
  sinks: [create(AccessLogSinkStatusSchema, { name: "db", reader: true })],
});

const row = create(AccessLogSchema, {
  id: "42",
  time: timestampFromDate(new Date("2026-11-10T08:00:00Z")),
  requestId: "req-42",
  kind: "rpc",
  service: "sync.v1.SyncService",
  procedure: "/sync.v1.SyncService/PushItems",
  method: "POST",
  path: "/sync.v1.SyncService/PushItems",
  principalType: "api_credential",
  principalId: "key-9",
  principalName: "warehouse",
  clientIp: "203.0.113.9",
  code: "invalid_argument",
  status: 400,
  success: false,
  error: "2 items rejected",
  durationUs: 5300n,
  attrsJson: '{"items_failed":2,"items_ok":8}',
});

describe("AccessLogList", () => {
  beforeEach(() => {
    mocks.getAccessLogStatus.mockReset().mockResolvedValue(readable);
    mocks.listAccessLogs.mockReset().mockResolvedValue(
      create(ListAccessLogsResponseSchema, {
        items: [row],
        page: create(PageInfoSchema, { total: 1n, pageNum: 1, pageSize: 10 }),
      }),
    );
  });

  it("lists entries and shows business attrs in the detail drawer", async () => {
    render(<AccessLogList />);
    expect(await screen.findByText("/sync.v1.SyncService/PushItems")).toBeTruthy();
    expect(screen.getByText("warehouse（api_credential）")).toBeTruthy();
    expect(screen.getByText("invalid_argument")).toBeTruthy();
    expect(screen.getByText("5.30")).toBeTruthy();
    expect(mocks.listAccessLogs).toHaveBeenCalledWith(
      expect.objectContaining({ page: { pageNum: 1, pageSize: 20 }, result: "", principal: "" }),
    );

    // 点接口名打开详情，不另放“详情”按钮。
    expect(screen.queryByRole("button", { name: "详情" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "/sync.v1.SyncService/PushItems" }));
    const attrs = await screen.findByTestId("access-log-attrs");
    expect(attrs.textContent).toContain('"items_ok": 8');
    const drawer = screen.getByRole("dialog", { name: "请求详情" });
    expect(within(drawer).getByText("2 items rejected")).toBeTruthy();
    expect(within(drawer).getByText("req-42")).toBeTruthy();
  });

  it("explains how to enable a readable sink instead of querying", async () => {
    mocks.getAccessLogStatus.mockResolvedValue(
      create(GetAccessLogStatusResponseSchema, {
        enabled: true,
        readable: false,
        sinks: [create(AccessLogSinkStatusSchema, { name: "stdout" })],
      }),
    );
    render(<AccessLogList />);
    expect(await screen.findByText("请求日志没有保存到数据库")).toBeTruthy();
    expect(screen.getByText(/stdout/)).toBeTruthy();
    await waitFor(() => expect(mocks.listAccessLogs).not.toHaveBeenCalled());
  });

  it("reports a disabled request log", async () => {
    mocks.getAccessLogStatus.mockResolvedValue(create(GetAccessLogStatusResponseSchema, {}));
    render(<AccessLogList />);
    expect(await screen.findByText("请求日志没有开启")).toBeTruthy();
  });
});
