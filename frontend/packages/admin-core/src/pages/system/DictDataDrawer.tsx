import type {
  DictDataCreateValues,
  DictDataFilters,
  DictDataUpdateValues,
} from "../../gen/resource/dict-data.operations.gen";
import { useSystemOperations } from "../../system/hooks";
import { Drawer } from "antd";
import { useAccess } from "../../runtime/access";
import type { DictData } from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { dictDataSchema } from "../../gen/resource/dict-data.schema.gen";
import { ResourceTable } from "../../components/resource";
import type {
  ResourceColumnOverrides,
  ResourceFormItemOverrides,
  ResourceRowActionOverride,
  ResourceToolbarOverride,
} from "../../components/resource";

import { useAppMessage } from "../../runtime/message";
type Props = { dictType: string; open: boolean; onClose: () => void };

type DictDataCreateItems = ResourceFormItemOverrides<
  typeof dictDataSchema,
  DictDataCreateValues,
  "create",
  DictData
>;
type DictDataUpdateItems = ResourceFormItemOverrides<
  typeof dictDataSchema,
  DictDataUpdateValues,
  "update",
  DictData
>;

const dictDataColumns: ResourceColumnOverrides<typeof dictDataSchema, DictData> = {
  id: { hidden: true },
  dictLabel: { title: "标签" },
  dictValue: { title: "键值" },
  dictSort: { title: "排序" },
  dictType: { hidden: true },
  cssClass: { hidden: true },
  listClass: { hidden: true },
  isDefault: { hidden: true },
  status: { hidden: true },
  remark: { hidden: true },
  createTime: { hidden: true },
  updateTime: { hidden: true },
};

const dictDataCreateItems: DictDataCreateItems = {
  dictSort: { label: "排序", valueType: "number", fieldProps: { min: 0 } },
  dictLabel: { label: "标签" },
  dictValue: { label: "键值" },
  dictType: { visible: false },
  isDefault: { visible: false },
  remark: { label: "备注", valueType: "textarea" },
};

const dictDataUpdateItems: DictDataUpdateItems = {
  dictSort: { label: "排序", valueType: "number", fieldProps: { min: 0 } },
  dictLabel: { label: "标签" },
  dictValue: { label: "键值" },
  isDefault: { visible: false },
  remark: { label: "备注", valueType: "textarea" },
};

export default function DictDataDrawer({ dictType, open, onClose }: Props) {
  const { dictDataOperations, dictDataRelationLoaders } = useSystemOperations();
  const message = useAppMessage();
  const { has } = useAccess();
  const rowActions: readonly ResourceRowActionOverride<DictData, bigint>[] = [
    { action: "update", merge: "replace", title: "编辑" },
    { action: "delete", merge: "replace", title: "删除" },
  ];
  const toolbar: readonly ResourceToolbarOverride<DictData, bigint>[] = [
    { action: "create", merge: "replace", title: "新建" },
    { action: "batchDelete", merge: "replace", title: "批量删除" },
  ];

  return (
    <Drawer title={`字典数据 - ${dictType}`} open={open} onClose={onClose} width={640}>
      {open ? (
        <ResourceTable<
          typeof dictDataSchema,
          DictData,
          bigint,
          DictDataFilters,
          DictDataCreateValues,
          DictDataUpdateValues
        >
          key={dictType}
          schema={dictDataSchema}
          operations={dictDataOperations}
          access={{ can: (permission) => has(permission) }}
          reload={async () => undefined}
          columns={dictDataColumns}
          rowActions={rowActions}
          toolbar={toolbar}
          query={{
            defaults: {
              pageSize: 10,
              orders: [{ field: "dictSort", desc: false }],
            },
            initial: { filters: { dictType } },
          }}
          form={{
            createConfig: {
              title: "新建字典数据",
              initialValues: { dictType, dictSort: 0, isDefault: 0, remark: "" },
              itemOverrides: dictDataCreateItems,
              relationLoaders: { dictType: dictDataRelationLoaders.dictType },
              onSuccess: async () => {
                message.success("创建成功");
              },
            },
            editConfig: {
              title: "编辑字典数据",
              initialValues: (entity) => ({
                dictSort: entity.dictSort,
                dictLabel: entity.dictLabel,
                dictValue: entity.dictValue,
                isDefault: entity.isDefault,
                remark: entity.remark,
              }),
              itemOverrides: dictDataUpdateItems,
              onSuccess: async () => {
                message.success("已保存");
              },
            },
          }}
        />
      ) : null}
    </Drawer>
  );
}
