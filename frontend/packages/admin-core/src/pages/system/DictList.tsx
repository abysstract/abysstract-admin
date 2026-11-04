import type {
  DictTypeCreateValues,
  DictTypeFilters,
  DictTypeUpdateValues,
} from "../../gen/resource/dict-type.operations.gen";
import { useSystemOperations } from "../../system/hooks";
import { useState } from "react";
import { PageContainer } from "@ant-design/pro-components";
import { useAccess } from "../../runtime/access";
import type { DictType } from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { dictTypeSchema } from "../../gen/resource/dict-type.schema.gen";
import { ResourceTable } from "../../components/resource";
import type {
  ResourceColumnOverrides,
  ResourceFormItemOverrides,
  ResourceRowActionOverride,
  ResourceToolbarOverride,
} from "../../components/resource";
import { fmtTime, statusTag } from "../../util";
import DictDataDrawer from "./DictDataDrawer";

import { formatErrorMessage, useAppMessage } from "../../runtime/message";
type DictTypeCreateItems = ResourceFormItemOverrides<
  typeof dictTypeSchema,
  DictTypeCreateValues,
  "create",
  DictType
>;
type DictTypeUpdateItems = ResourceFormItemOverrides<
  typeof dictTypeSchema,
  DictTypeUpdateValues,
  "update",
  DictType
>;

const dictTypeColumns: ResourceColumnOverrides<typeof dictTypeSchema, DictType> = {
  id: { hidden: true },
  dictName: { title: "字典名称" },
  dictType: { title: "字典类型" },
  status: { title: "状态", render: ({ value }) => statusTag(value) },
  remark: { hidden: true },
  createTime: { title: "创建时间", render: ({ value }) => fmtTime(value) },
  updateTime: { hidden: true },
};

const dictTypeCreateItems: DictTypeCreateItems = {
  dictName: { label: "字典名称" },
  dictType: { label: "字典类型" },
  remark: { label: "备注", valueType: "textarea" },
};

const dictTypeUpdateItems: DictTypeUpdateItems = {
  dictName: { label: "字典名称" },
  remark: { label: "备注", valueType: "textarea" },
};

const dictTypeToolbar: readonly ResourceToolbarOverride<DictType, bigint>[] = [
  { action: "create", merge: "replace", title: "新建字典" },
];

export default function DictList() {
  const { dictTypeOperations } = useSystemOperations();
  const message = useAppMessage();
  const { has } = useAccess();
  const [drawerType, setDrawerType] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);

  const rowActions: readonly ResourceRowActionOverride<DictType, bigint>[] = [
    {
      action: "custom",
      name: "data",
      merge: "append",
      permission: `${dictTypeSchema.permPrefix}:list`,
      title: "数据",
      onClick: ({ entity }) => {
        setDrawerType(entity.dictType);
        setDrawerOpen(true);
      },
    },
  ];

  return (
    <PageContainer>
      <ResourceTable<
        typeof dictTypeSchema,
        DictType,
        bigint,
        DictTypeFilters,
        DictTypeCreateValues,
        DictTypeUpdateValues
      >
        schema={dictTypeSchema}
        operations={dictTypeOperations}
        access={{ can: (permission) => has(permission) }}
        reload={async () => {
          message.success("删除成功");
        }}
        onError={(error) => message.error(formatErrorMessage(error))}
        columns={dictTypeColumns}
        rowActions={rowActions}
        toolbar={dictTypeToolbar}
        form={{
          createConfig: {
            title: "新建字典类型",
            itemOverrides: dictTypeCreateItems,
            onSuccess: async () => {
              message.success("创建成功");
            },
          },
          editConfig: {
            title: "编辑字典类型",
            initialValues: (entity) => ({
              dictName: entity.dictName,
              remark: entity.remark,
            }),
            itemOverrides: dictTypeUpdateItems,
            onSuccess: async () => {
              message.success("已保存");
            },
          },
        }}
      />
      <DictDataDrawer
        dictType={drawerType}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
      />
    </PageContainer>
  );
}
