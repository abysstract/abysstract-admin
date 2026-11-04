import { useSystemClients } from "../../system/hooks";
import { useRef } from "react";
import { Button, Popconfirm, Tag } from "antd";
import {
  PageContainer,
  ProTable,
  type ActionType,
  type ProColumns,
} from "@ant-design/pro-components";
import type { LoginLog } from "@abysstract/admin-proto/abysstract/admin/v1/log_pb";
import { fmtTime } from "../../util";
import { useAccess } from "../../runtime/access";

import { formatErrorMessage, useAppMessage } from "../../runtime/message";
export default function LoginLogList() {
  const { log: logClient } = useSystemClients();
  const message = useAppMessage();
  const actionRef = useRef<ActionType>();
  const { has } = useAccess();

  const clean = async () => {
    try {
      await logClient.cleanLoginLogs({});
      message.success("已清空");
      actionRef.current?.reload();
    } catch (e) {
      message.error(formatErrorMessage(e, "操作失败"));
    }
  };

  const columns: ProColumns<LoginLog>[] = [
    { title: "用户名", dataIndex: "username" },
    { title: "IP", dataIndex: "ip" },
    {
      title: "结果",
      dataIndex: "status",
      render: (_, r) =>
        r.status === 2 ? <Tag color="red">失败</Tag> : <Tag color="green">成功</Tag>,
    },
    { title: "信息", dataIndex: "msg" },
    { title: "时间", dataIndex: "loginTime", render: (_, r) => fmtTime(r.loginTime) },
  ];

  return (
    <PageContainer>
      <ProTable<LoginLog>
        actionRef={actionRef}
        rowKey={(r) => String(r.id)}
        // Avoid the legacy DensityIcon Tooltip/ResizeObserver findDOMNode fallback.
        options={{ density: false }}
        search={false}
        columns={columns}
        request={async (params) => {
          const res = await logClient.listLoginLogs({
            page: { pageNum: params.current ?? 1, pageSize: params.pageSize ?? 10 },
          });
          return { data: res.items, total: Number(res.page?.total ?? 0n), success: true };
        }}
        toolBarRender={() => [
          has("system:logininfor:remove") && (
            <Popconfirm key="clean" title="确认清空所有登录日志？" onConfirm={clean}>
              <Button danger>清空</Button>
            </Popconfirm>
          ),
        ]}
      />
    </PageContainer>
  );
}
