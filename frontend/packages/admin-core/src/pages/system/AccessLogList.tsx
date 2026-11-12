import { useSystemClients } from "../../system/hooks";
import { useEffect, useState } from "react";
import { Alert, Button, Descriptions, Drawer, Result, Spin, Tag, Typography } from "antd";
import { PageContainer, ProTable, type ProColumns } from "@ant-design/pro-components";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import type {
  AccessLog,
  GetAccessLogStatusResponse,
} from "@abysstract/admin-proto/abysstract/admin/v1/log_pb";
import { fmtTime } from "../../util";
import { formatErrorMessage } from "../../runtime/message";

type SearchParams = {
  current?: number;
  pageSize?: number;
  timeRange?: [string, string];
  principal?: string;
  procedure?: string;
  result?: string;
  code?: string;
  requestId?: string;
};

/** ProTable date-time pickers submit local "YYYY-MM-DD HH:mm:ss" strings. */
function toTimestamp(value?: string) {
  if (!value) return undefined;
  const date = new Date(value.includes("T") ? value : value.replace(" ", "T"));
  return Number.isNaN(date.getTime()) ? undefined : timestampFromDate(date);
}

function durationText(us: bigint): string {
  const ms = Number(us) / 1000;
  return ms >= 100 ? ms.toFixed(0) : ms.toFixed(ms >= 10 ? 1 : 2);
}

function target(r: AccessLog): string {
  return r.procedure || `${r.method} ${r.path}`;
}

function principalText(r: AccessLog): string {
  if (!r.principalType && !r.principalName && !r.principalId) return "-";
  const name = r.principalName || r.principalId;
  return r.principalType ? `${name}（${r.principalType}）` : name;
}

function resultTag(r: AccessLog) {
  const label = r.code || (r.status ? String(r.status) : "");
  return r.success ? (
    <Tag color="green">{label || "成功"}</Tag>
  ) : (
    <Tag color="red">{label || "失败"}</Tag>
  );
}

function prettyAttrs(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

export default function AccessLogList() {
  const { log: logClient } = useSystemClients();
  const [status, setStatus] = useState<GetAccessLogStatusResponse>();
  const [statusError, setStatusError] = useState<string>();
  const [detail, setDetail] = useState<AccessLog>();

  useEffect(() => {
    let active = true;
    logClient
      .getAccessLogStatus({})
      .then((res) => active && setStatus(res))
      .catch(
        (e: unknown) => active && setStatusError(formatErrorMessage(e, "读取请求日志状态失败")),
      );
    return () => {
      active = false;
    };
  }, [logClient]);

  if (statusError) {
    return (
      <PageContainer>
        <Result status="error" title="无法读取请求日志状态" subTitle={statusError} />
      </PageContainer>
    );
  }
  if (!status) {
    return (
      <PageContainer>
        <Spin />
      </PageContainer>
    );
  }
  if (!status.readable) {
    return (
      <PageContainer>
        <Result
          status="info"
          title={status.enabled ? "请求日志未配置可查询的存储" : "请求日志未启用"}
          subTitle={
            status.enabled
              ? `当前 sink（${status.sinks.map((s) => s.name).join(", ")}）不支持查询。将 ADMIN_ACCESS_LOG_SINKS 设置为包含 db 后重启服务。`
              : "设置 ADMIN_ACCESS_LOG_SINKS（例如 db,stdout）后重启服务。"
          }
        />
      </PageContainer>
    );
  }

  const columns: ProColumns<AccessLog>[] = [
    {
      title: "时间",
      dataIndex: "timeRange",
      valueType: "dateTimeRange",
      hideInTable: true,
    },
    {
      title: "时间",
      dataIndex: "time",
      search: false,
      width: 180,
      render: (_, r) => fmtTime(r.time),
    },
    {
      title: "调用方",
      dataIndex: "principal",
      tooltip: "精确匹配调用方 ID 或名称",
      render: (_, r) => principalText(r),
    },
    {
      title: "接口",
      dataIndex: "procedure",
      ellipsis: true,
      tooltip: "以 / 开头按前缀匹配，否则按包含匹配；同时匹配 HTTP 路径",
      render: (_, r) => <Typography.Text code>{target(r)}</Typography.Text>,
    },
    {
      title: "结果",
      dataIndex: "result",
      valueType: "select",
      valueEnum: {
        success: { text: "成功", status: "Success" },
        failure: { text: "失败", status: "Error" },
      },
      render: (_, r) => resultTag(r),
    },
    { title: "错误码", dataIndex: "code", hideInTable: true },
    { title: "请求 ID", dataIndex: "requestId", hideInTable: true },
    { title: "IP", dataIndex: "clientIp", search: false },
    {
      title: "耗时(ms)",
      dataIndex: "durationUs",
      search: false,
      render: (_, r) => durationText(r.durationUs),
    },
    {
      title: "操作",
      valueType: "option",
      render: (_, r) => [
        <Button key="detail" type="link" size="small" onClick={() => setDetail(r)}>
          详情
        </Button>,
      ],
    },
  ];

  return (
    <PageContainer>
      {status.dropped > 0n && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          title={`自启动以来因队列已满丢弃了 ${status.dropped} 条请求日志`}
        />
      )}
      <ProTable<AccessLog, SearchParams>
        rowKey={(r) => r.id}
        // Avoid the legacy DensityIcon Tooltip/ResizeObserver findDOMNode fallback.
        options={{ density: false }}
        columns={columns}
        request={async (params) => {
          const res = await logClient.listAccessLogs({
            page: { pageNum: params.current ?? 1, pageSize: params.pageSize ?? 10 },
            startTime: toTimestamp(params.timeRange?.[0]),
            endTime: toTimestamp(params.timeRange?.[1]),
            principal: params.principal?.trim() ?? "",
            procedure: params.procedure?.trim() ?? "",
            result: params.result ?? "",
            code: params.code?.trim() ?? "",
            requestId: params.requestId?.trim() ?? "",
          });
          return { data: res.items, total: Number(res.page?.total ?? 0n), success: true };
        }}
      />
      <Drawer title="请求详情" size={640} open={!!detail} onClose={() => setDetail(undefined)}>
        {detail && (
          <Descriptions
            column={1}
            bordered
            size="small"
            items={[
              { key: "time", label: "时间", children: fmtTime(detail.time) },
              { key: "requestId", label: "请求 ID", children: detail.requestId || "-" },
              { key: "kind", label: "类型", children: detail.kind },
              ...(detail.service
                ? [{ key: "service", label: "服务", children: detail.service }]
                : []),
              { key: "target", label: "接口", children: target(detail) },
              {
                key: "http",
                label: "HTTP",
                children:
                  [detail.method, detail.path, detail.status ? String(detail.status) : ""]
                    .filter(Boolean)
                    .join(" ") || "-",
              },
              { key: "principal", label: "调用方", children: principalText(detail) },
              ...(detail.principalId
                ? [{ key: "principalId", label: "调用方 ID", children: detail.principalId }]
                : []),
              { key: "clientIp", label: "IP", children: detail.clientIp || "-" },
              { key: "userAgent", label: "User-Agent", children: detail.userAgent || "-" },
              { key: "result", label: "结果", children: resultTag(detail) },
              ...(detail.error ? [{ key: "error", label: "错误", children: detail.error }] : []),
              { key: "duration", label: "耗时(ms)", children: durationText(detail.durationUs) },
              {
                key: "attrs",
                label: "业务字段",
                children: detail.attrsJson ? (
                  <pre data-testid="access-log-attrs" style={{ margin: 0, whiteSpace: "pre-wrap" }}>
                    {prettyAttrs(detail.attrsJson)}
                  </pre>
                ) : (
                  "-"
                ),
              },
            ]}
          />
        )}
      </Drawer>
    </PageContainer>
  );
}
