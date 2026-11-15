import { useSystemClients } from "../../system/hooks";
import { useCallback } from "react";
import { Button, Popconfirm, Tag } from "antd";
import { PageContainer, type ProColumns } from "@ant-design/pro-components";
import type { LoginLog } from "@abysstract/admin-proto/abysstract/admin/v1/log_pb";
import { textCell, timeCell } from "../../util";
import { useAccess } from "../../runtime/access";
import { formatErrorMessage, useAppMessage } from "../../runtime/message";
import { LogTable, type LogQuery } from "./LogTable";

const columns: ProColumns<LoginLog>[] = [
  { title: "用户名", dataIndex: "username" },
  { title: "IP", dataIndex: "ip" },
  {
    title: "结果",
    dataIndex: "status",
    render: (_, r) =>
      r.status === 2 ? <Tag color="error">失败</Tag> : <Tag color="success">成功</Tag>,
  },
  { title: "信息", dataIndex: "msg", render: (_, r) => textCell(r.msg) },
  { title: "时间", dataIndex: "loginTime", render: (_, r) => timeCell(r.loginTime) },
];

export default function LoginLogList() {
  const { log: logClient } = useSystemClients();
  const message = useAppMessage();
  const { has } = useAccess();

  const fetch = useCallback(
    async ({ keyword, pageNum, pageSize }: LogQuery) => {
      const res = await logClient.listLoginLogs({ page: { pageNum, pageSize }, keyword });
      return { items: res.items, total: Number(res.page?.total ?? 0n) };
    },
    [logClient],
  );

  const clean = async (reload: () => Promise<void>) => {
    try {
      await logClient.cleanLoginLogs({});
      message.success("已清空");
      await reload();
    } catch (e) {
      message.error(formatErrorMessage(e, "无法清空，请稍后重试"));
    }
  };

  return (
    <PageContainer>
      <LogTable<LoginLog>
        placeholder="用户名"
        fetch={fetch}
        columns={columns}
        rowKey={(r) => String(r.id)}
        emptyText="还没有登录日志"
        noMatchText="没有符合条件的登录日志"
        actions={(reload) =>
          has("system:logininfor:remove") ? (
            <Popconfirm
              title="清空全部登录日志？"
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
