import { Tag } from "antd";
import { timestampDate, type Timestamp } from "@bufbuild/protobuf/wkt";
import { Status } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";

export function fmtTime(ts?: Timestamp): string {
  return ts ? timestampDate(ts).toLocaleString() : "";
}

export function statusTag(status: Status) {
  return status === Status.DISABLED ? <Tag color="red">停用</Tag> : <Tag color="green">正常</Tag>;
}
