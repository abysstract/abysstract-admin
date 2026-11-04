import { useSystemClients } from "../../system/hooks";
import { useEffect, useState } from "react";
import { Card, Col, Row, Statistic, Table, type TableColumnsType } from "antd";
import { UserOutlined, TeamOutlined, LoginOutlined } from "@ant-design/icons";
import { PageContainer } from "@ant-design/pro-components";
import type {
  OverviewResponse,
  RecentLogin,
} from "@abysstract/admin-proto/abysstract/admin/v1/dashboard_pb";

import { formatErrorMessage, useAppMessage } from "../../runtime/message";

export default function DashboardPage() {
  const { dashboard: dashboardClient } = useSystemClients();
  const message = useAppMessage();
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<OverviewResponse | null>(null);

  useEffect(() => {
    dashboardClient
      .overview({})
      .then((res: OverviewResponse) => setData(res))
      .catch((cause: unknown) => {
        message.error(formatErrorMessage(cause, "加载仪表盘失败"));
      })
      .finally(() => setLoading(false));
  }, [dashboardClient, message]);

  const columns: TableColumnsType<RecentLogin> = [
    { title: "用户名", dataIndex: "username", key: "username" },
    { title: "IP", dataIndex: "ip", key: "ip" },
    { title: "时间", dataIndex: "loginTime", key: "loginTime" },
    {
      title: "结果",
      dataIndex: "status",
      key: "status",
      render: (v: number) => (v === 1 ? "成功" : "失败"),
    },
  ];

  return (
    <PageContainer>
      <Row gutter={16}>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic
              title="用户数"
              value={Number(data?.userCount ?? 0n)}
              prefix={<UserOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic
              title="角色数"
              value={Number(data?.roleCount ?? 0n)}
              prefix={<TeamOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic
              title="今日登录"
              value={Number(data?.todayLoginCount ?? 0n)}
              prefix={<LoginOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card loading={loading}>
            <Statistic title="在线" value={Number(data?.onlineCount ?? 0n)} />
          </Card>
        </Col>
      </Row>
      <Card title="最近登录" style={{ marginTop: 16 }}>
        <Table
          loading={loading}
          rowKey={(r) => `${r.username}-${r.loginTime}`}
          columns={columns}
          dataSource={data?.recentLogins ?? []}
          pagination={false}
          size="small"
        />
      </Card>
    </PageContainer>
  );
}
