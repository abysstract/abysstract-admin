import { useSystemClients } from "../../system/hooks";
import { useCallback, useEffect, useMemo, useState, type ComponentProps } from "react";
import {
  Alert,
  Button,
  Collapse,
  DatePicker,
  Descriptions,
  Drawer,
  Input,
  Result,
  Select,
  Skeleton,
  Tag,
  Typography,
  type DescriptionsProps,
} from "antd";
import { PageContainer, type ProColumns } from "@ant-design/pro-components";
import { timestampFromDate } from "@bufbuild/protobuf/wkt";
import type {
  AccessLog,
  GetAccessLogStatusResponse,
} from "@abysstract/admin-proto/abysstract/admin/v1/log_pb";
import { fmtTime, timeCell } from "../../util";
import { formatErrorMessage } from "../../runtime/message";
import { useAdminTexts } from "../../runtime/texts";
import { readUrlParam } from "../../components/resource/urlQuery";
import { LogTable, type LogFilters, type LogQuery } from "./LogTable";

type RangeValue = Parameters<
  NonNullable<ComponentProps<typeof DatePicker.RangePicker>["onChange"]>
>[0];

function durationText(us: bigint): string {
  const ms = Number(us) / 1000;
  return ms >= 100 ? ms.toFixed(0) : ms.toFixed(ms >= 10 ? 1 : 2);
}

function target(r: AccessLog): string {
  return r.procedure || `${r.method} ${r.path}`;
}

function principalText(r: AccessLog): string {
  if (!r.principalType && !r.principalName && !r.principalId) return "";
  const name = r.principalName || r.principalId;
  return r.principalType ? `${name}（${r.principalType}）` : name;
}

function resultTag(r: AccessLog) {
  const label = r.code || (r.status ? String(r.status) : "");
  return r.success ? (
    <Tag color="success">{label || "成功"}</Tag>
  ) : (
    <Tag color="error">{label || "失败"}</Tag>
  );
}

function prettyAttrs(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

function toTimestamp(value?: string) {
  if (!value) return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : timestampFromDate(date);
}

const FILTER_KEYS = ["result", "principal", "code", "requestId"] as const;

function initialFilters(): LogFilters {
  return Object.fromEntries(FILTER_KEYS.map((key) => [key, readUrlParam(key)]));
}

/** 调用方、错误码、请求 ID 这类文字筛选：回车或离开输入框时生效。 */
function TextFilter({
  value,
  placeholder,
  onCommit,
}: {
  readonly value?: string;
  readonly placeholder: string;
  readonly onCommit: (value: string | undefined) => void;
}) {
  const [draft, setDraft] = useState(value ?? "");
  useEffect(() => setDraft(value ?? ""), [value]);
  const commit = () => onCommit(draft.trim() === "" ? undefined : draft.trim());
  return (
    <Input
      aria-label={placeholder}
      placeholder={placeholder}
      allowClear
      value={draft}
      style={{ width: 160 }}
      onChange={(event) => {
        setDraft(event.currentTarget.value);
        if (event.currentTarget.value === "") onCommit(undefined);
      }}
      onPressEnter={commit}
      onBlur={commit}
    />
  );
}

function AccessLogDetail({ log }: { readonly log: AccessLog }) {
  const http = [log.method, log.path, log.status ? String(log.status) : ""]
    .filter(Boolean)
    .join(" ");
  const principal = principalText(log);
  // 详情里的空字段不显示（见 DESIGN.md「数据展示」）。
  const items: DescriptionsProps["items"] = [
    { key: "target", label: "接口", children: target(log), span: 2 },
    { key: "result", label: "结果", children: resultTag(log) },
    { key: "duration", label: "耗时（ms）", children: durationText(log.durationUs) },
    ...(principal ? [{ key: "principal", label: "调用方", children: principal }] : []),
    { key: "time", label: "时间", children: fmtTime(log.time) },
    ...(log.error ? [{ key: "error", label: "错误", children: log.error, span: 2 }] : []),
    ...(log.clientIp ? [{ key: "clientIp", label: "IP", children: log.clientIp }] : []),
    ...(http ? [{ key: "http", label: "HTTP", children: http }] : []),
  ];
  const more: DescriptionsProps["items"] = [
    ...(log.requestId ? [{ key: "requestId", label: "请求 ID", children: log.requestId }] : []),
    { key: "kind", label: "类型", children: log.kind },
    ...(log.service ? [{ key: "service", label: "服务", children: log.service }] : []),
    ...(log.principalId
      ? [{ key: "principalId", label: "调用方 ID", children: log.principalId }]
      : []),
    ...(log.userAgent
      ? [{ key: "userAgent", label: "User-Agent", children: log.userAgent, span: 2 }]
      : []),
  ];
  return (
    <>
      <Descriptions column={2} items={items} />
      <Collapse
        ghost
        items={[
          {
            key: "more",
            label: "更多信息",
            children: <Descriptions column={2} items={more} />,
          },
          ...(log.attrsJson
            ? [
                {
                  key: "attrs",
                  label: "业务字段",
                  children: (
                    <pre
                      data-testid="access-log-attrs"
                      style={{ margin: 0, whiteSpace: "pre-wrap" }}
                    >
                      {prettyAttrs(log.attrsJson)}
                    </pre>
                  ),
                },
              ]
            : []),
        ]}
      />
    </>
  );
}

export default function AccessLogList() {
  const { log: logClient } = useSystemClients();
  const texts = useAdminTexts();
  const [status, setStatus] = useState<GetAccessLogStatusResponse>();
  const [statusError, setStatusError] = useState<string>();
  const [statusAttempt, setStatusAttempt] = useState(0);
  const [detail, setDetail] = useState<AccessLog>();
  const [filters, setFilters] = useState<LogFilters>(initialFilters);
  const [range, setRange] = useState<RangeValue>(null);
  const [showMore, setShowMore] = useState(
    () =>
      readUrlParam("principal") !== undefined ||
      readUrlParam("code") !== undefined ||
      readUrlParam("requestId") !== undefined,
  );

  useEffect(() => {
    let active = true;
    setStatus(undefined);
    setStatusError(undefined);
    logClient
      .getAccessLogStatus({})
      .then((res) => active && setStatus(res))
      .catch((e: unknown) => active && setStatusError(formatErrorMessage(e, "请稍后重试")));
    return () => {
      active = false;
    };
  }, [logClient, statusAttempt]);

  const setFilter = (key: string, value: string | undefined) =>
    setFilters((previous) => ({ ...previous, [key]: value }));

  const timeFilters = useMemo(
    () => ({
      from: range?.[0]?.toDate().toISOString(),
      to: range?.[1]?.toDate().toISOString(),
    }),
    [range],
  );

  const fetch = useCallback(
    async ({ keyword, pageNum, pageSize, filters: active }: LogQuery) => {
      const res = await logClient.listAccessLogs({
        page: { pageNum, pageSize },
        startTime: toTimestamp(active.from),
        endTime: toTimestamp(active.to),
        principal: active.principal ?? "",
        procedure: keyword.trim(),
        result: active.result ?? "",
        code: active.code ?? "",
        requestId: active.requestId ?? "",
      });
      return { items: res.items, total: Number(res.page?.total ?? 0n) };
    },
    [logClient],
  );

  if (statusError) {
    return (
      <PageContainer>
        <Result
          status="error"
          title="无法读取请求日志状态"
          subTitle={statusError}
          extra={
            <Button onClick={() => setStatusAttempt((value) => value + 1)}>
              {texts.action.retry}
            </Button>
          }
        />
      </PageContainer>
    );
  }
  if (!status) {
    return (
      <PageContainer>
        <Skeleton active />
      </PageContainer>
    );
  }
  if (!status.readable) {
    return (
      <PageContainer>
        <Result
          status="info"
          title={status.enabled ? "请求日志没有保存到数据库" : "请求日志没有开启"}
          subTitle={
            <>
              <div>
                {status.enabled
                  ? "开启数据库存储并重启服务后，就能在这里查询。"
                  : "开启并选择数据库存储、重启服务后，就能在这里查询。"}
              </div>
              <Typography.Text type="secondary">
                {status.enabled
                  ? `当前存储：${status.sinks.map((s) => s.name).join("、")}。在 ADMIN_ACCESS_LOG_SINKS 里加上 db。`
                  : "设置 ADMIN_ACCESS_LOG_SINKS，例如 db,stdout。"}
              </Typography.Text>
            </>
          }
          extra={<Button onClick={() => setStatusAttempt((value) => value + 1)}>重新检查</Button>}
        />
      </PageContainer>
    );
  }

  const columns: ProColumns<AccessLog>[] = [
    { title: "时间", dataIndex: "time", render: (_, r) => timeCell(r.time) },
    {
      title: "接口",
      dataIndex: "procedure",
      ellipsis: true,
      // 点接口打开详情（见 DESIGN.md「数据列表」：点名称打开，不另放“详情”）。
      render: (_, r) => <Typography.Link onClick={() => setDetail(r)}>{target(r)}</Typography.Link>,
    },
    {
      title: "调用方",
      dataIndex: "principal",
      render: (_, r) => principalText(r) || <Typography.Text type="secondary">-</Typography.Text>,
    },
    { title: "结果", dataIndex: "result", render: (_, r) => resultTag(r) },
    { title: "IP", dataIndex: "clientIp" },
    {
      title: "耗时（ms）",
      dataIndex: "durationUs",
      align: "right",
      className: "resource-table__numeric",
      render: (_, r) => durationText(r.durationUs),
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
      <LogTable<AccessLog>
        placeholder="接口或路径"
        filters={{ ...filters, ...timeFilters }}
        urlFilterKeys={FILTER_KEYS}
        onClearFilters={() => {
          setFilters({});
          setRange(null);
        }}
        filterControls={
          <>
            <Select
              aria-label="结果"
              placeholder="结果"
              allowClear
              style={{ width: 120 }}
              value={filters.result}
              options={[
                { label: "成功", value: "success" },
                { label: "失败", value: "failure" },
              ]}
              onChange={(value?: string) => setFilter("result", value)}
            />
            <DatePicker.RangePicker
              aria-label="时间范围"
              showTime
              value={range}
              onChange={(value) => setRange(value)}
            />
            {showMore ? (
              <>
                <TextFilter
                  placeholder="调用方"
                  value={filters.principal}
                  onCommit={(value) => setFilter("principal", value)}
                />
                <TextFilter
                  placeholder="错误码"
                  value={filters.code}
                  onCommit={(value) => setFilter("code", value)}
                />
                <TextFilter
                  placeholder="请求 ID"
                  value={filters.requestId}
                  onCommit={(value) => setFilter("requestId", value)}
                />
              </>
            ) : (
              <Button type="link" onClick={() => setShowMore(true)}>
                更多筛选
              </Button>
            )}
          </>
        }
        fetch={fetch}
        columns={columns}
        rowKey={(r) => r.id}
        emptyText="还没有请求日志"
        noMatchText="没有符合条件的请求日志"
      />
      <Drawer title="请求详情" size={720} open={!!detail} onClose={() => setDetail(undefined)}>
        {detail && <AccessLogDetail log={detail} />}
      </Drawer>
    </PageContainer>
  );
}
