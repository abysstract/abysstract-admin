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
import { timeCell } from "../../util";

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
  createTime: { title: "创建时间", render: ({ value }) => timeCell(value) },
  updateTime: { hidden: true },
};

const configCreateItems: ConfigCreateItems = {
  configName: { label: "参数名称" },
  configKey: { label: "参数键名", sameRowAsPrevious: true },
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

const toolbar: readonly ResourceToolbarOverride<Config, bigint>[] = [];

export default function ConfigList() {
  const { configOperations } = useSystemOperations();
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
        columns={configColumns}
        rowActions={rowActions}
        toolbar={toolbar}
        form={{
          createConfig: {
            title: "新增参数",
            initialValues: { configType: 0, remark: "" },
            itemOverrides: configCreateItems,
          },
          editConfig: {
            title: "编辑参数",
            initialValues: (entity) => ({
              configName: entity.configName,
              configValue: entity.configValue,
              remark: entity.remark,
            }),
            itemOverrides: configUpdateItems,
          },
        }}
      />
    </PageContainer>
  );
}
