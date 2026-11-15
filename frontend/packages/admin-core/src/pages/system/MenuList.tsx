import type {
  MenuCreateValues,
  MenuFilters,
  MenuUpdateValues,
} from "../../gen/resource/menu.operations.gen";
import { useSystemOperations } from "../../system/hooks";
import { useEffect, useState } from "react";
import { Tag, TreeSelect } from "antd";
import { PageContainer } from "@ant-design/pro-components";
import type { Menu } from "@abysstract/admin-proto/abysstract/admin/v1/menu_pb";
import { menuSchema } from "../../gen/resource/menu.schema.gen";
import { ResourceTable } from "../../components/resource";
import type {
  ResourceColumnOverrides,
  ResourceFormItemOverrides,
  ResourceRowActionOverride,
  ResourceToolbarOverride,
} from "../../components/resource";
import { statusTag, timeCell } from "../../util";
import { useAccess } from "../../runtime/access";
import { formatErrorMessage } from "../../runtime/message";

type MenuCreateItems = ResourceFormItemOverrides<
  typeof menuSchema,
  MenuCreateValues,
  "create",
  Menu
>;
type MenuUpdateItems = ResourceFormItemOverrides<
  typeof menuSchema,
  MenuUpdateValues,
  "update",
  Menu
>;
type MenuParentTreeNode = {
  readonly title: string;
  readonly value: string;
  readonly key: string;
  children?: MenuParentTreeNode[];
};

// 菜单类型是普通分类，用默认灰色 Tag（见 DESIGN.md「颜色」）。
const typeLabels: Record<string, string> = { M: "目录", C: "菜单", F: "按钮" };

const typeOptions = [
  { label: "目录", value: "M" },
  { label: "菜单", value: "C" },
  { label: "按钮", value: "F" },
] as const;

const menuColumns: ResourceColumnOverrides<typeof menuSchema, Menu> = {
  id: { hidden: true },
  menuName: { title: "菜单名称" },
  parentId: { title: "上级菜单 ID", hidden: true },
  orderNum: { title: "排序", numeric: true },
  path: { title: "路由" },
  component: { title: "组件", hidden: true },
  query: { title: "参数", hidden: true },
  isFrame: { title: "外链", hidden: true },
  isCache: { title: "缓存", hidden: true },
  menuType: {
    title: "类型",
    render: ({ value }) => {
      return <Tag>{typeLabels[String(value)] ?? String(value)}</Tag>;
    },
  },
  visible: { title: "可见", hidden: true },
  status: { title: "状态", render: ({ value }) => statusTag(value) },
  perms: { title: "权限标识" },
  icon: { title: "图标", hidden: true },
  createTime: { title: "创建时间", render: ({ value }) => timeCell(value), hidden: true },
  updateTime: { title: "更新时间", render: ({ value }) => timeCell(value), hidden: true },
};

function menuParentItems(
  parentComponent: typeof MenuCreateParentSelect | typeof MenuUpdateParentSelect,
): MenuCreateItems | MenuUpdateItems {
  return {
    parentId: { label: "上级菜单", valueType: "custom", component: parentComponent },
    menuName: { label: "菜单名称" },
    orderNum: { label: "排序", valueType: "number", fieldProps: { min: 0 } },
    path: { label: "路由地址" },
    component: { label: "组件路径" },
    query: { label: "路由参数", visible: false },
    isFrame: { label: "是否外链", visible: false },
    isCache: { label: "是否缓存", visible: false },
    menuType: { label: "类型", valueType: "select", fieldProps: { options: typeOptions } },
    visible: { label: "是否可见", visible: false },
    perms: { label: "权限标识", fieldProps: { placeholder: "system:user:add" } },
    icon: { label: "图标" },
  } as MenuCreateItems | MenuUpdateItems;
}

const menuCreateItems = menuParentItems(MenuCreateParentSelect) as MenuCreateItems;
const menuUpdateItems = menuParentItems(MenuUpdateParentSelect) as MenuUpdateItems;

const menuRowActions: readonly ResourceRowActionOverride<Menu, bigint>[] = [
  { action: "update", merge: "replace", title: "编辑" },
  { action: "delete", merge: "replace", title: "删除" },
];
const menuToolbar: readonly ResourceToolbarOverride<Menu, bigint>[] = [];

function MenuCreateParentSelect({
  value,
  disabled,
  onChange,
}: {
  readonly value: bigint | undefined;
  readonly disabled?: boolean;
  readonly onChange: (value: bigint | undefined) => void;
}) {
  return <MenuParentSelect value={value} disabled={disabled} onChange={onChange} />;
}

function MenuUpdateParentSelect({
  value,
  disabled,
  onChange,
}: {
  readonly value: bigint;
  readonly disabled?: boolean;
  readonly onChange: (value: bigint) => void;
}) {
  return (
    <MenuParentSelect
      value={value}
      disabled={disabled}
      onChange={(nextValue) => onChange(nextValue ?? 0n)}
    />
  );
}

function MenuParentSelect({
  value,
  disabled,
  onChange,
}: {
  readonly value: bigint | undefined;
  readonly disabled?: boolean;
  readonly onChange: (value: bigint | undefined) => void;
}) {
  const { menuOperations } = useSystemOperations();
  const [treeData, setTreeData] = useState<MenuParentTreeNode[]>([
    { title: "顶级菜单", value: "0", key: "0" },
  ]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | undefined>();

  useEffect(() => {
    let active = true;
    const loadOptions = async () => {
      try {
        const menus: Menu[] = [];
        let pageNum = 1;
        let total = 0n;
        do {
          const result = await menuOperations.list({
            pageNum,
            pageSize: 100,
            orders: [{ field: "orderNum", desc: false }],
          });
          const pageItems = result.items ?? [];
          menus.push(...pageItems);
          total = result.total;
          if (pageItems.length === 0) {
            if (BigInt(menus.length) < total) {
              throw new Error("菜单列表返回不完整");
            }
            break;
          }
          pageNum += 1;
        } while (BigInt(menus.length) < total);

        const nodes = new Map<bigint, MenuParentTreeNode>();
        for (const menu of menus) {
          if (menu.menuType === "F") continue;
          nodes.set(menu.id, {
            title: menu.menuName,
            value: menu.id.toString(),
            key: menu.id.toString(),
          });
        }
        const roots: MenuParentTreeNode[] = [{ title: "顶级菜单", value: "0", key: "0" }];
        for (const menu of menus) {
          const node = nodes.get(menu.id);
          if (node === undefined) continue;
          const parent = nodes.get(menu.parentId);
          if (menu.parentId !== 0n && parent !== undefined) {
            const children = parent.children ?? [];
            children.push(node);
            parent.children = children;
          } else {
            roots.push(node);
          }
        }
        if (active) setTreeData(roots);
      } catch (cause) {
        if (active) setError(cause instanceof Error ? cause : new Error(String(cause)));
      } finally {
        if (active) setLoading(false);
      }
    };
    void loadOptions();
    return () => {
      active = false;
    };
  }, []);

  return (
    <TreeSelect
      aria-label="上级菜单"
      value={(value ?? 0n).toString()}
      disabled={disabled}
      loading={loading}
      showSearch
      treeNodeFilterProp="title"
      treeDefaultExpandAll
      treeData={treeData}
      status={error === undefined ? undefined : "error"}
      notFoundContent={error === undefined ? "没有菜单" : formatErrorMessage(error, "无法加载菜单")}
      onChange={(nextValue) => onChange(BigInt(String(nextValue ?? "0")))}
    />
  );
}

function buildMenuTree(rows: readonly Menu[]): readonly Menu[] {
  const nodes = new Map<bigint, Menu>();
  for (const row of rows) {
    nodes.set(row.id, { ...row, children: undefined as unknown as Menu[] });
  }
  const roots: Menu[] = [];
  for (const row of rows) {
    const node = nodes.get(row.id);
    if (node === undefined) continue;
    const parent = nodes.get(row.parentId);
    if (row.parentId !== 0n && parent !== undefined) {
      if (!parent.children) parent.children = [];
      parent.children.push(node);
    } else {
      roots.push(node);
    }
  }
  return roots;
}

function flattenMenuTree(rows: readonly Menu[]): readonly Menu[] {
  return rows.flatMap((row) => [row, ...flattenMenuTree(row.children ?? [])]);
}

export default function MenuList() {
  const { menuOperations } = useSystemOperations();
  const { has } = useAccess();

  return (
    <PageContainer>
      <ResourceTable<
        typeof menuSchema,
        Menu,
        bigint,
        MenuFilters,
        MenuCreateValues,
        MenuUpdateValues
      >
        schema={menuSchema}
        operations={menuOperations}
        access={{ can: (permission) => has(permission) }}
        columns={menuColumns}
        treeData={{ pageSize: 100, build: buildMenuTree, flatten: flattenMenuTree }}
        rowActions={menuRowActions}
        toolbar={menuToolbar}
        query={{ defaults: { orders: [{ field: "orderNum", desc: false }] } }}
        form={{
          createConfig: {
            title: "新增菜单",
            initialValues: {
              parentId: 0n,
              orderNum: 0,
              path: "",
              component: "",
              query: "",
              isFrame: 0,
              isCache: 0,
              menuType: "C",
              visible: 1,
              perms: "",
              icon: "",
            },
            itemOverrides: menuCreateItems,
          },
          editConfig: {
            title: "编辑菜单",
            initialValues: (entity) => ({
              menuName: entity.menuName,
              parentId: entity.parentId,
              orderNum: entity.orderNum,
              path: entity.path,
              component: entity.component,
              query: entity.query,
              isFrame: entity.isFrame,
              isCache: entity.isCache,
              menuType: entity.menuType,
              visible: entity.visible,
              perms: entity.perms,
              icon: entity.icon,
            }),
            itemOverrides: menuUpdateItems,
          },
        }}
      />
    </PageContainer>
  );
}
