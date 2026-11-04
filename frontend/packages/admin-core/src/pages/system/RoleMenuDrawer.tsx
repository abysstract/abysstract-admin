import { useSystemOperations } from "../../system/hooks";
import { useEffect, useState } from "react";
import { Drawer, Button, Tree, Space } from "antd";
import type { DataNode } from "antd/es/tree";
import type { Menu } from "@abysstract/admin-proto/abysstract/admin/v1/menu_pb";
import type { Role } from "@abysstract/admin-proto/abysstract/admin/v1/role_pb";
import { formatErrorMessage, useAppMessage } from "../../runtime/message";

async function loadAllMenus(menuOperations: {
  list: (query: {
    pageNum: number;
    pageSize: number;
    orders: { field: "orderNum"; desc: boolean }[];
  }) => Promise<{ items?: readonly Menu[]; total: bigint }>;
}): Promise<readonly Menu[]> {
  const items: Menu[] = [];
  let pageNum = 1;
  let total = 0n;
  do {
    const result = await menuOperations.list({
      pageNum,
      pageSize: 100,
      orders: [{ field: "orderNum", desc: false }],
    });
    const pageItems = result.items ?? [];
    items.push(...pageItems);
    total = result.total;
    if (pageItems.length === 0) {
      if (BigInt(items.length) < total) {
        throw new Error("菜单列表返回不完整");
      }
      break;
    }
    pageNum += 1;
  } while (BigInt(items.length) < total);
  return items;
}

function buildTreeData(menus: readonly Menu[]): DataNode[] {
  const nodes = new Map<bigint, DataNode>();
  for (const menu of menus) {
    nodes.set(menu.id, { key: String(menu.id), title: menu.menuName });
  }
  const roots: DataNode[] = [];
  for (const menu of menus) {
    const node = nodes.get(menu.id);
    if (node === undefined) continue;
    const parent = nodes.get(menu.parentId);
    if (menu.parentId !== 0n && parent !== undefined) {
      (parent.children ??= []).push(node);
    } else {
      roots.push(node);
    }
  }
  return roots;
}

type Props = {
  role: Role | null;
  open: boolean;
  onClose: () => void;
};

export default function RoleMenuDrawer({ role, open, onClose }: Props) {
  const { menuOperations, roleOperations } = useSystemOperations();
  const message = useAppMessage();
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [checked, setChecked] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(true);
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (!open || !role) return;
    setLoading(true);
    setReady(false);
    setTreeData([]);
    setChecked([]);
    let active = true;
    (async () => {
      const [menus, roleMenus] = await Promise.all([
        loadAllMenus(menuOperations),
        roleOperations.custom.getRoleMenus({ roleId: role.id }),
      ]);
      if (!active) return;
      setTreeData(buildTreeData(menus));
      setChecked(roleMenus.menuIds.map(String));
      setReady(true);
    })()
      .catch((cause: unknown) => {
        if (active) message.error(formatErrorMessage(cause, "加载菜单权限失败"));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [menuOperations, message, open, role, roleOperations]);

  const save = async () => {
    if (!role) return;
    setSaving(true);
    try {
      const menuIds = checked.map((key) => BigInt(key));
      await roleOperations.custom.updateRoleMenus({ roleId: role.id, menuIds });
      message.success("权限已保存");
      onClose();
    } catch (cause) {
      message.error(formatErrorMessage(cause, "保存失败"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Drawer
      title={role ? `分配菜单权限 - ${role.roleName}` : "分配菜单权限"}
      open={open}
      onClose={onClose}
      width={420}
      loading={loading}
      extra={
        <Space>
          <Button onClick={onClose}>取消</Button>
          <Button type="primary" loading={saving} disabled={!ready} onClick={save}>
            保存
          </Button>
        </Space>
      }
    >
      <p>目录、菜单和按钮权限按勾选项独立授权。</p>
      <Tree
        checkable
        checkStrictly
        treeData={treeData}
        checkedKeys={checked}
        onCheck={(checkedKeys) => {
          const keys = Array.isArray(checkedKeys) ? checkedKeys : checkedKeys.checked;
          setChecked(keys.map(String));
        }}
        defaultExpandAll
      />
    </Drawer>
  );
}
