import { useSystemClients } from "../../system/hooks";
import { useCallback } from "react";
import { Button, Popconfirm, Tag, Typography } from "antd";
import { PageContainer, type ProColumns } from "@ant-design/pro-components";
import type { OperLog } from "@abysstract/admin-proto/abysstract/admin/v1/log_pb";
import { textCell, timeCell } from "../../util";
import { useAccess } from "../../runtime/access";
import { formatErrorMessage, useAppMessage } from "../../runtime/message";
import { LogTable, type LogQuery } from "./LogTable";

const columns: ProColumns<OperLog>[] = [
  { title: "操作", dataIndex: "title" },
  {
    title: "方法",
    dataIndex: "method",
    ellipsis: true,
    render: (_, r) => <Typography.Text type="secondary">{r.method}</Typography.Text>,
  },
  { title: "操作人", dataIndex: "operName" },
  { title: "IP", dataIndex: "operIp" },
  {
    title: "请求参数",
    dataIndex: "operParam",
    ellipsis: true,
    width: 240,
    // 请求参数需要逐字核对，保留等宽字体（见 DESIGN.md「字体」）。
    render: (_, r) =>
      r.operParam ? <Typography.Text code>{r.operParam}</Typography.Text> : textCell(),
  },
  {
    title: "结果",
    dataIndex: "status",
    render: (_, r) =>
      r.status === 2 ? <Tag color="error">失败</Tag> : <Tag color="success">成功</Tag>,
  },
  {
    title: "耗时（ms）",
    dataIndex: "costTime",
    align: "right",
    className: "resource-table__numeric",
    render: (_, r) => r.costTime.toLocaleString("en-US"),
  },
  { title: "时间", dataIndex: "operTime", render: (_, r) => timeCell(r.operTime) },
];

export default function OperLogList() {
  const { log: logClient } = useSystemClients();
  const message = useAppMessage();
  const { has } = useAccess();

  const fetch = useCallback(
    async ({ keyword, pageNum, pageSize }: LogQuery) => {
      const res = await logClient.listOperLogs({ page: { pageNum, pageSize }, keyword });
      return { items: res.items, total: Number(res.page?.total ?? 0n) };
    },
    [logClient],
  );

  const clean = async (reload: () => Promise<void>) => {
    try {
      await logClient.cleanOperLogs({});
      message.success("已清空");
      await reload();
    } catch (e) {
      message.error(formatErrorMessage(e, "无法清空，请稍后重试"));
    }
  };

  return (
    <PageContainer>
      <LogTable<OperLog>
        placeholder="操作人、操作"
        fetch={fetch}
        columns={columns}
        rowKey={(r) => String(r.id)}
        emptyText="还没有操作日志"
        noMatchText="没有符合条件的操作日志"
        actions={(reload) =>
          has("system:operlog:remove") ? (
            <Popconfirm
              title="清空全部操作日志？"
              description="清空后无法恢复。"
              okText="清空"
              okButtonProps={{ danger: true }}
              cancelText="取消"
              onConfirm={() => clean(reload)}
            >
              <Button danger>清空</Button>
            </Popconfirm>
          ) : null
        }
      />
    </PageContainer>
  );
}
