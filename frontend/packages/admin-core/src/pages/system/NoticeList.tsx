import type {
  NoticeCreateValues,
  NoticeFilters,
  NoticeUpdateValues,
} from "../../gen/resource/notice.operations.gen";
import { useSystemOperations } from "../../system/hooks";
import { Tag } from "antd";
import { PageContainer } from "@ant-design/pro-components";
import { useAccess } from "../../runtime/access";
import type { Notice } from "@abysstract/admin-proto/abysstract/admin/v1/notice_pb";
import { noticeSchema } from "../../gen/resource/notice.schema.gen";
import { ResourceTable } from "../../components/resource";
import type {
  ResourceColumnOverrides,
  ResourceFormItemOverrides,
  ResourceRowActionOverride,
  ResourceToolbarOverride,
} from "../../components/resource";
import { statusTag, timeCell } from "../../util";

type NoticeCreateItems = ResourceFormItemOverrides<
  typeof noticeSchema,
  NoticeCreateValues,
  "create",
  Notice
>;
type NoticeUpdateItems = ResourceFormItemOverrides<
  typeof noticeSchema,
  NoticeUpdateValues,
  "update",
  Notice
>;

const noticeTypeOptions = [
  { label: "通知", value: 1 },
  { label: "公告", value: 2 },
] as const;

const noticeColumns: ResourceColumnOverrides<typeof noticeSchema, Notice> = {
  id: { hidden: true },
  noticeTitle: { title: "标题" },
  noticeType: {
    title: "类型",
    render: ({ value }) => (
      // 通知类型是普通分类，用默认灰色 Tag（见 DESIGN.md「颜色」）。
      <Tag>{value === 2 ? "公告" : "通知"}</Tag>
    ),
  },
  noticeContent: { hidden: true },
  status: { title: "状态", render: ({ value }) => statusTag(value) },
  remark: { hidden: true },
  createTime: { title: "创建时间", render: ({ value }) => timeCell(value) },
  updateTime: { hidden: true },
};

const noticeCreateItems: NoticeCreateItems = {
  noticeTitle: { label: "标题" },
  noticeType: {
    label: "类型",
    valueType: "select",
    fieldProps: { options: noticeTypeOptions },
  },
  noticeContent: { label: "内容", valueType: "textarea", fieldProps: { rows: 6 } },
  remark: { label: "备注" },
};

const noticeUpdateItems: NoticeUpdateItems = {
  noticeTitle: { label: "标题" },
  noticeType: {
    label: "类型",
    valueType: "select",
    fieldProps: { options: noticeTypeOptions },
  },
  noticeContent: { label: "内容", valueType: "textarea", fieldProps: { rows: 6 } },
  remark: { label: "备注" },
};

const noticeRowActions: readonly ResourceRowActionOverride<Notice, bigint>[] = [
  { action: "update", merge: "replace", title: "编辑" },
  { action: "delete", merge: "replace", title: "删除" },
];

const noticeToolbar: readonly ResourceToolbarOverride<Notice, bigint>[] = [];

export default function NoticeList() {
  const { noticeOperations } = useSystemOperations();
  const { has } = useAccess();

  return (
    <PageContainer>
      <ResourceTable<
        typeof noticeSchema,
        Notice,
        bigint,
        NoticeFilters,
        NoticeCreateValues,
        NoticeUpdateValues
      >
        schema={noticeSchema}
        operations={noticeOperations}
        access={{ can: (permission) => has(permission) }}
        columns={noticeColumns}
        rowActions={noticeRowActions}
        toolbar={noticeToolbar}
        form={{
          createConfig: {
            title: "新增公告",
            initialValues: { noticeType: 1 },
            itemOverrides: noticeCreateItems,
          },
          editConfig: {
            title: "编辑公告",
            initialValues: (entity) => ({
              noticeTitle: entity.noticeTitle,
              noticeType: entity.noticeType,
              noticeContent: entity.noticeContent,
              remark: entity.remark,
            }),
            itemOverrides: noticeUpdateItems,
          },
        }}
      />
    </PageContainer>
  );
}
