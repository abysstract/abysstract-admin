import type { ReactNode } from "react";
import { Tag, Typography } from "antd";
import { timestampDate, type Timestamp } from "@bufbuild/protobuf/wkt";
import { Status } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";

function pad(value: number): string {
  return String(value).padStart(2, "0");
}

/** 格式化为 `YYYY-MM-DD HH:mm:ss`（本地时区，24 小时制），不随浏览器语言变化。 */
export function formatDateTime(date: Date): string {
  return (
    `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ` +
    `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  );
}

/** Timestamp 转 `YYYY-MM-DD HH:mm:ss`；空值返回 `-`。 */
export function fmtTime(ts?: Timestamp): string {
  return ts ? formatDateTime(timestampDate(ts)) : "-";
}

/**
 * 后端以字符串返回的时间（RFC 3339 或 `YYYY-MM-DD HH:mm:ss`）按统一格式显示；
 * 无法解析时原样返回，空值返回 `-`。
 */
export function fmtTimeText(value?: string): string {
  if (value === undefined || value.trim() === "") return "-";
  if (/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/.test(value)) return value;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : formatDateTime(date);
}

/** 空值统一显示次要色的 `-`。 */
export function EmptyValue() {
  return <Typography.Text type="secondary">-</Typography.Text>;
}

/** 表格时间列：不换行，空值为次要色 `-`。 */
export function timeCell(ts?: Timestamp): ReactNode {
  if (!ts) return <EmptyValue />;
  return <span style={{ whiteSpace: "nowrap" }}>{fmtTime(ts)}</span>;
}

/** 表格里的普通文本：空值为次要色 `-`。 */
export function textCell(value?: string): ReactNode {
  return value === undefined || value === "" ? <EmptyValue /> : value;
}

export function statusTag(status: Status) {
  return status === Status.DISABLED ? <Tag>停用</Tag> : <Tag color="success">正常</Tag>;
}
