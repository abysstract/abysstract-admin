import type {
  ConfigCreateValues,
  ConfigFilters,
  ConfigUpdateValues,
} from "../../gen/resource/config.operations.gen";
import { useSystemOperations } from "../../system/hooks";
import { PageContainer } from "@ant-design/pro-components";
import { useAccess } from "../../runtime/access";
import type { Config } from "@abysstract/admin-proto/abysstract/admin/v1/config_pb";
import { configSchema } from "../../gen/resource/config.schema.gen";
import { ResourceTable } from "../../components/resource";
import type {
  ResourceColumnOverrides,
  ResourceFormItemOverrides,
  ResourceRowActionOverride,
  ResourceToolbarOverride,
} from "../../components/resource";
import { fmtTime } from "../../util";

import { formatErrorMessage, useAppMessage } from "../../runtime/message";
type ConfigCreateItems = ResourceFormItemOverrides<
  typeof configSchema,
  ConfigCreateValues,
  "create",
  Config
>;
type ConfigUpdateItems = ResourceFormItemOverrides<
  typeof configSchema,
  ConfigUpdateValues,
  "update",
  Config
>;

const configColumns: ResourceColumnOverrides<typeof configSchema, Config> = {
  id: { hidden: true },
  configName: { title: "参数名称" },
  configKey: { title: "参数键名" },
  configValue: { title: "参数键值" },
  configType: { hidden: true },
  remark: { hidden: true },
  createTime: { title: "创建时间", render: ({ value }) => fmtTime(value) },
  updateTime: { hidden: true },
};

const configCreateItems: ConfigCreateItems = {
  configName: { label: "参数名称" },
  configKey: { label: "参数键名" },
  configValue: { label: "参数键值" },
  configType: { visible: false },
  remark: { label: "备注", valueType: "textarea" },
};

const configUpdateItems: ConfigUpdateItems = {
  configName: { label: "参数名称" },
  configValue: { label: "参数键值" },
  remark: { label: "备注", valueType: "textarea" },
};

const rowActions: readonly ResourceRowActionOverride<Config, bigint>[] = [
  { action: "update", merge: "replace", title: "编辑" },
  { action: "delete", merge: "replace", title: "删除" },
];

const toolbar: readonly ResourceToolbarOverride<Config, bigint>[] = [
  { action: "create", merge: "replace", title: "新建参数" },
  { action: "batchDelete", merge: "replace", title: "批量删除" },
];

export default function ConfigList() {
  const { configOperations } = useSystemOperations();
  const message = useAppMessage();
  const { has } = useAccess();

  return (
    <PageContainer>
      <ResourceTable<
        typeof configSchema,
        Config,
        bigint,
        ConfigFilters,
        ConfigCreateValues,
        ConfigUpdateValues
      >
        schema={configSchema}
        operations={configOperations}
        access={{ can: (permission) => has(permission) }}
        reload={async () => {
          message.success("已删除");
        }}
        onError={(error) => message.error(formatErrorMessage(error))}
        columns={configColumns}
        rowActions={rowActions}
        toolbar={toolbar}
        form={{
          createConfig: {
            title: "新建参数",
            initialValues: { configType: 0, remark: "" },
            itemOverrides: configCreateItems,
            onSuccess: async () => {
              message.success("创建成功");
            },
          },
          editConfig: {
            title: "编辑参数",
            initialValues: (entity) => ({
              configName: entity.configName,
              configValue: entity.configValue,
              remark: entity.remark,
            }),
            itemOverrides: configUpdateItems,
            onSuccess: async () => {
              message.success("已保存");
            },
          },
        }}
      />
    </PageContainer>
  );
}
