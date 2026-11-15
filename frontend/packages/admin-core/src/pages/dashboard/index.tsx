import { useSystemClients } from "../../system/hooks";
import { useEffect, useState } from "react";
import {
  Button,
  Card,
  Col,
  Empty,
  Result,
  Row,
  Statistic,
  Table,
  Tag,
  theme,
  type TableColumnsType,
} from "antd";
import { PageContainer } from "@ant-design/pro-components";
import type {
  OverviewResponse,
  RecentLogin,
} from "@abysstract/admin-proto/abysstract/admin/v1/dashboard_pb";

import { formatErrorMessage } from "../../runtime/message";
import { useAdminTexts } from "../../runtime/texts";
import { fmtTimeText } from "../../util";

const columns: TableColumnsType<RecentLogin> = [
  { title: "用户名", dataIndex: "username", key: "username" },
  { title: "IP", dataIndex: "ip", key: "ip" },
  {
    title: "时间",
    dataIndex: "loginTime",
    key: "loginTime",
    render: (value: string) => <span style={{ whiteSpace: "nowrap" }}>{fmtTimeText(value)}</span>,
  },
  {
    title: "结果",
    dataIndex: "status",
    key: "status",
    render: (v: number) =>
      v === 1 ? <Tag color="success">成功</Tag> : <Tag color="error">失败</Tag>,
  },
];

export default function DashboardPage() {
  const { dashboard: dashboardClient } = useSystemClients();
  const texts = useAdminTexts();
  const { token } = theme.useToken();
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<OverviewResponse | null>(null);
  const [error, setError] = useState<unknown>();
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    setLoading(true);
    setError(undefined);
    dashboardClient
      .overview({})
      .then((res: OverviewResponse) => setData(res))
      .catch((cause: unknown) => setError(cause))
      .finally(() => setLoading(false));
  }, [dashboardClient, attempt]);

  // 加载失败时说明原因并给“重试”，不把数字显示成 0（见 DESIGN.md「数据概览页」）。
  if (error !== undefined) {
    return (
      <PageContainer>
        <Result
          status="error"
          title="无法加载首页数据"
          subTitle={formatErrorMessage(error)}
          extra={
            <Button onClick={() => setAttempt((value) => value + 1)}>{texts.action.retry}</Button>
          }
        />
      </PageContainer>
    );
  }

  const stats = [
    { key: "users", title: "用户数", value: data?.userCount },
    { key: "roles", title: "角色数", value: data?.roleCount },
    { key: "todayLogins", title: "今日登录", value: data?.todayLoginCount },
    { key: "online", title: "在线", value: data?.onlineCount },
  ];

  return (
    <PageContainer>
      <Row gutter={16}>
        {stats.map((stat) => (
          <Col key={stat.key} span={6}>
            <Card loading={loading}>
              <Statistic title={stat.title} value={Number(stat.value ?? 0n)} />
            </Card>
          </Col>
        ))}
      </Row>
      <Card title="最近登录" style={{ marginTop: token.margin }}>
        <Table
          loading={loading}
          rowKey={(r) => `${r.username}-${r.loginTime}`}
          columns={columns}
          dataSource={data?.recentLogins ?? []}
          pagination={false}
          size="small"
          locale={{
            emptyText: loading ? (
              texts.table.loading
            ) : (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有登录记录" />
            ),
          }}
        />
      </Card>
    </PageContainer>
  );
}
