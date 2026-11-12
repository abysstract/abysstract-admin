import { useSystemClients } from "../../system/hooks";
import { useRef } from "react";
import { Button, Popconfirm, Tag, Typography } from "antd";
import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from "@ant-design/pro-components";
import type { OperLog } from "@abysstract/admin-proto/abysstract/admin/v1/log_pb";
import { fmtTime } from "../../util";
import { useAccess } from "../../runtime/access";

import { formatErrorMessage, useAppMessage } from "../../runtime/message";
export default function OperLogList() {
  const { log: logClient } = useSystemClients();
  const message = useAppMessage();
  const actionRef = useRef<ActionType | undefined>(undefined);
  const { has } = useAccess();

  const clean = async () => {
    try {
      await logClient.cleanOperLogs({});
      message.success("已清空");
      actionRef.current?.reload();
    } catch (e) {
      message.error(formatErrorMessage(e, "操作失败"));
    }
  };

  const columns: ProColumns<OperLog>[] = [
    { title: "操作", dataIndex: "title" },
    { title: "方法", dataIndex: "method", ellipsis: true },
    { title: "操作人", dataIndex: "operName" },
    { title: "IP", dataIndex: "operIp" },
    {
      title: "请求参数",
      dataIndex: "operParam",
      ellipsis: true,
      width: 240,
      render: (_, r) => (r.operParam ? <Typography.Text code>{r.operParam}</Typography.Text> : "-"),
    },
    {
      title: "结果",
      dataIndex: "status",
      render: (_, r) =>
        r.status === 2 ? <Tag color="red">失败</Tag> : <Tag color="green">成功</Tag>,
    },
    { title: "耗时(ms)", dataIndex: "costTime", render: (_, r) => String(r.costTime) },
    { title: "时间", dataIndex: "operTime", render: (_, r) => fmtTime(r.operTime) },
  ];

  return (
    <PageContainer>
      <ProTable<OperLog>
        actionRef={actionRef}
        rowKey={(r) => String(r.id)}
        // Avoid the legacy DensityIcon Tooltip/ResizeObserver findDOMNode fallback.
        options={{ density: false }}
        search={false}
        columns={columns}
        request={async (params) => {
          const res = await logClient.listOperLogs({
            page: { pageNum: params.current ?? 1, pageSize: params.pageSize ?? 10 },
          });
          return { data: res.items, total: Number(res.page?.total ?? 0n), success: true };
        }}
        toolBarRender={() => [
          has("system:operlog:remove") && (
            <Popconfirm key="clean" title="确认清空所有操作日志？" onConfirm={clean}>
              <Button danger>清空</Button>
            </Popconfirm>
          ),
        ]}
      />
    </PageContainer>
  );
}
