import type {
  RoleCreateValues,
  RoleFilters,
  RoleUpdateValues,
} from "../../gen/resource/role.operations.gen";
import { useSystemOperations } from "../../system/hooks";
import { Switch, Tag } from "antd";
import { PageContainer } from "@ant-design/pro-components";
import { useState } from "react";
import type { Role } from "@abysstract/admin-proto/abysstract/admin/v1/role_pb";
import { Status } from "@abysstract/admin-proto/abysstract/admin/v1/common_pb";
import { menuSchema } from "../../gen/resource/menu.schema.gen";
import { roleSchema } from "../../gen/resource/role.schema.gen";
import { ResourceTable } from "../../components/resource";
import type {
  ResourceColumnOverrides,
  ResourceFormItemOverrides,
  ResourceRowActionContext,
  ResourceRowActionOverride,
  ResourceToolbarOverride,
} from "../../components/resource";
import { fmtTime, statusTag } from "../../util";
import { useAccess } from "../../runtime/access";
import { formatErrorMessage, useAppMessage } from "../../runtime/message";
import RoleMenuDrawer from "./RoleMenuDrawer";

type RoleCreateItems = ResourceFormItemOverrides<
  typeof roleSchema,
  RoleCreateValues,
  "create",
  Role
>;
type RoleUpdateItems = ResourceFormItemOverrides<
  typeof roleSchema,
  RoleUpdateValues,
  "update",
  Role
>;

const dataScopeOptions = [
  { label: "全部数据", value: 1 },
  { label: "自定义数据", value: 2 },
  { label: "本部门数据", value: 3 },
  { label: "本部门及以下", value: 4 },
  { label: "仅本人数据", value: 5 },
] as const;

const dataScopeLabels: Record<number, string> = Object.fromEntries(
  dataScopeOptions.map((option) => [option.value, option.label]),
);

function roleColumns(canEdit: boolean): ResourceColumnOverrides<typeof roleSchema, Role> {
  return {
    id: { hidden: true },
    roleName: { title: "角色名称" },
    roleKey: { title: "权限标识" },
    roleSort: { title: "排序" },
    dataScope: {
      title: "数据范围",
      render: ({ value }) => <Tag>{dataScopeLabels[value] ?? String(value)}</Tag>,
    },
    status: {
      title: "状态",
      render: ({ entity, reload }) =>
        canEdit ? <RoleStatusSwitch entity={entity} reload={reload} /> : statusTag(entity.status),
    },
    remark: { title: "备注", hidden: true },
    createTime: { title: "创建时间", render: ({ value }) => fmtTime(value), hidden: true },
    updateTime: { title: "更新时间", render: ({ value }) => fmtTime(value), hidden: true },
  };
}

const roleCreateItems: RoleCreateItems = {
  roleName: { label: "角色名称" },
  roleKey: { label: "权限标识" },
  roleSort: { label: "排序", valueType: "number", fieldProps: { min: 0 } },
  dataScope: {
    label: "数据范围",
    valueType: "select",
    fieldProps: { options: dataScopeOptions },
  },
  remark: { label: "备注" },
};

const roleUpdateItems: RoleUpdateItems = {
  roleName: { label: "角色名称" },
  roleSort: { label: "排序", valueType: "number", fieldProps: { min: 0 } },
  dataScope: {
    label: "数据范围",
    valueType: "select",
    fieldProps: { options: dataScopeOptions },
  },
  remark: { label: "备注" },
};

const roleToolbar: readonly ResourceToolbarOverride<Role, bigint>[] = [
  { action: "create", merge: "replace", title: "新建角色" },
  { action: "batchDelete", merge: "replace", title: "批量删除" },
];

type RoleStatusSwitchProps = Pick<ResourceRowActionContext<Role, bigint>, "entity" | "reload">;

function RoleStatusSwitch({ entity, reload }: RoleStatusSwitchProps) {
  const { roleOperations } = useSystemOperations();
  const message = useAppMessage();
  const [loading, setLoading] = useState(false);
  const enabled = entity.status === Status.ENABLED;

  const changeStatus = async (checked: boolean) => {
    setLoading(true);
    try {
      await roleOperations.custom.changeRoleStatus({
        id: entity.id,
        status: checked ? Status.ENABLED : Status.DISABLED,
      });
      message.success(checked ? "角色已启用" : "角色已停用");
      await reload();
    } catch (cause) {
      message.error(formatErrorMessage(cause, "状态更新失败"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Switch
      aria-label={`${entity.roleName}状态`}
      checked={enabled}
      loading={loading}
      disabled={loading}
      onChange={(checked) => void changeStatus(checked)}
    />
  );
}

const roleListPermission = roleSchema.permissions?.list ?? `${roleSchema.permPrefix}:list`;
const roleUpdatePermission = roleSchema.permissions?.update ?? `${roleSchema.permPrefix}:edit`;
const menuListPermission = menuSchema.permissions?.list ?? `${menuSchema.permPrefix}:list`;

export default function RoleList() {
  const { roleOperations } = useSystemOperations();
  const message = useAppMessage();
  const { has } = useAccess();
  const [drawerRole, setDrawerRole] = useState<Role | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const canEdit = has(roleUpdatePermission);
  const canAssignMenus = canEdit && has(roleListPermission) && has(menuListPermission);

  const roleRowActions: readonly ResourceRowActionOverride<Role, bigint>[] = [
    { action: "update", merge: "replace", title: "编辑" },
    {
      action: "custom",
      name: "assignMenus",
      permission: roleUpdatePermission,
      merge: "append",
      title: "分配菜单",
      // The drawer performs GetRoleMenus and loads the menu tree. Hide it
      // unless both the role and menu read prerequisites are available.
      hidden: !canAssignMenus,
      onClick: ({ entity }) => {
        setDrawerRole(entity);
        setDrawerOpen(true);
      },
    },
    { action: "delete", merge: "replace", title: "删除" },
  ];

  return (
    <PageContainer>
      <ResourceTable<
        typeof roleSchema,
        Role,
        bigint,
        RoleFilters,
        RoleCreateValues,
        RoleUpdateValues
      >
        schema={roleSchema}
        operations={roleOperations}
        access={{ can: (permission) => has(permission) }}
        onError={(error) => message.error(formatErrorMessage(error))}
        columns={roleColumns(canEdit)}
        rowActions={roleRowActions}
        toolbar={roleToolbar}
        form={{
          createConfig: {
            title: "新建角色",
            initialValues: { roleSort: 0, dataScope: 1 },
            itemOverrides: roleCreateItems,
            onSuccess: async () => {
              message.success("创建成功");
            },
          },
          editConfig: {
            title: "编辑角色",
            initialValues: (entity) => ({
              roleName: entity.roleName,
              roleSort: entity.roleSort,
              dataScope: entity.dataScope,
              remark: entity.remark,
            }),
            itemOverrides: roleUpdateItems,
            onSuccess: async () => {
              message.success("已保存");
            },
          },
        }}
      />
      <RoleMenuDrawer role={drawerRole} open={drawerOpen} onClose={() => setDrawerOpen(false)} />
    </PageContainer>
  );
}
