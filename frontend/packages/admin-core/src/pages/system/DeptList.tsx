import type {
  DeptCreateValues,
  DeptFilters,
  DeptUpdateValues,
} from "../../gen/resource/dept.operations.gen";
import { useSystemOperations } from "../../system/hooks";
import { useEffect, useState } from "react";
import { Select } from "antd";
import { PageContainer } from "@ant-design/pro-components";
import type { Dept } from "@abysstract/admin-proto/abysstract/admin/v1/dept_pb";
import { deptSchema } from "../../gen/resource/dept.schema.gen";
import { ResourceTable } from "../../components/resource";
import type {
  ResourceColumnOverrides,
  ResourceFormItemOverrides,
  ResourceRowActionOverride,
  ResourceToolbarOverride,
} from "../../components/resource";
import { fmtTime, statusTag } from "../../util";
import { useAccess } from "../../runtime/access";
import { formatErrorMessage, useAppMessage } from "../../runtime/message";

type DeptCreateItems = ResourceFormItemOverrides<
  typeof deptSchema,
  DeptCreateValues,
  "create",
  Dept
>;
type DeptUpdateItems = ResourceFormItemOverrides<
  typeof deptSchema,
  DeptUpdateValues,
  "update",
  Dept
>;
type DeptParentOption = { readonly label: string; readonly value: bigint };

const deptColumns: ResourceColumnOverrides<typeof deptSchema, Dept> = {
  id: { hidden: true },
  parentId: { title: "上级部门 ID", hidden: true },
  deptName: { title: "部门名称" },
  orderNum: { title: "排序" },
  leader: { title: "负责人" },
  phone: { title: "电话" },
  email: { title: "邮箱", hidden: true },
  status: { title: "状态", render: ({ value }) => statusTag(value) },
  createTime: { title: "创建时间", render: ({ value }) => fmtTime(value), hidden: true },
  updateTime: { title: "更新时间", render: ({ value }) => fmtTime(value), hidden: true },
};

const deptCreateItems: DeptCreateItems = {
  parentId: {
    label: "上级部门",
    valueType: "custom",
    component: DeptCreateParentSelect,
  },
  deptName: { label: "部门名称" },
  orderNum: { label: "排序", valueType: "number", fieldProps: { min: 0 } },
  leader: { label: "负责人" },
  phone: { label: "联系电话" },
  email: { label: "邮箱" },
};

const deptUpdateItems: DeptUpdateItems = {
  parentId: {
    label: "上级部门",
    valueType: "custom",
    component: DeptUpdateParentSelect,
  },
  deptName: { label: "部门名称" },
  orderNum: { label: "排序", valueType: "number", fieldProps: { min: 0 } },
  leader: { label: "负责人" },
  phone: { label: "联系电话" },
  email: { label: "邮箱" },
};

const deptRowActions: readonly ResourceRowActionOverride<Dept, bigint>[] = [
  { action: "update", merge: "replace", title: "编辑" },
  { action: "delete", merge: "replace", title: "删除" },
];

const deptToolbar: readonly ResourceToolbarOverride<Dept, bigint>[] = [
  { action: "create", merge: "replace", title: "新建部门" },
  { action: "batchDelete", merge: "replace", title: "批量删除" },
];

function DeptCreateParentSelect({
  value,
  disabled,
  onChange,
}: {
  readonly value: bigint | undefined;
  readonly disabled?: boolean;
  readonly onChange: (value: bigint | undefined) => void;
}) {
  return <DeptParentSelect value={value} disabled={disabled} onChange={onChange} />;
}

function DeptUpdateParentSelect({
  value,
  disabled,
  onChange,
}: {
  readonly value: bigint;
  readonly disabled?: boolean;
  readonly onChange: (value: bigint) => void;
}) {
  return (
    <DeptParentSelect
      value={value}
      disabled={disabled}
      onChange={(nextValue) => onChange(nextValue ?? 0n)}
    />
  );
}

function DeptParentSelect({
  value,
  disabled,
  onChange,
}: {
  readonly value: bigint | undefined;
  readonly disabled?: boolean;
  readonly onChange: (value: bigint | undefined) => void;
}) {
  const { deptOperations } = useSystemOperations();
  const [options, setOptions] = useState<readonly DeptParentOption[]>([
    { label: "顶级部门", value: 0n },
  ]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | undefined>();

  useEffect(() => {
    let active = true;
    const loadOptions = async () => {
      setLoading(true);
      setError(undefined);
      try {
        const nextOptions: DeptParentOption[] = [{ label: "顶级部门", value: 0n }];
        let loadedCount = 0n;
        let pageNum = 1;
        let total = 0n;
        do {
          const result = await deptOperations.list({
            pageNum,
            pageSize: 100,
            orders: [{ field: "orderNum", desc: false }],
          });
          const pageItems = result.items ?? [];
          nextOptions.push(...pageItems.map((dept) => ({ label: dept.deptName, value: dept.id })));
          loadedCount += BigInt(pageItems.length);
          total = result.total;
          if (pageItems.length === 0) {
            if (loadedCount < total) {
              throw new Error("部门选项列表返回不完整");
            }
            break;
          }
          pageNum += 1;
        } while (loadedCount < total);
        if (active) setOptions(nextOptions);
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
    <Select
      aria-label="上级部门"
      value={value ?? 0n}
      disabled={disabled}
      loading={loading}
      showSearch
      optionFilterProp="label"
      options={[...options]}
      status={error === undefined ? undefined : "error"}
      notFoundContent={error === undefined ? "暂无部门" : formatErrorMessage(error, "暂无部门")}
      onChange={(nextValue) =>
        onChange(typeof nextValue === "bigint" ? nextValue : BigInt(String(nextValue)))
      }
    />
  );
}

function buildDeptTree(rows: readonly Dept[]): readonly Dept[] {
  const nodes = new Map<bigint, Dept>();
  for (const row of rows) {
    // rc-table treats an existing empty `children: []` as expandable.
    // Leaves must keep children undefined so they render without a plus icon.
    nodes.set(row.id, { ...row, children: undefined as unknown as Dept[] });
  }
  const roots: Dept[] = [];
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

function flattenDeptTree(rows: readonly Dept[]): readonly Dept[] {
  return rows.flatMap((row) => [row, ...flattenDeptTree(row.children ?? [])]);
}

export default function DeptList() {
  const { deptOperations } = useSystemOperations();
  const message = useAppMessage();
  const { has } = useAccess();

  return (
    <PageContainer>
      <ResourceTable<
        typeof deptSchema,
        Dept,
        bigint,
        DeptFilters,
        DeptCreateValues,
        DeptUpdateValues
      >
        schema={deptSchema}
        operations={deptOperations}
        access={{ can: (permission) => has(permission) }}
        onError={(error) => message.error(formatErrorMessage(error))}
        columns={deptColumns}
        treeData={{
          pageSize: 100,
          build: buildDeptTree,
          flatten: flattenDeptTree,
        }}
        rowActions={deptRowActions}
        toolbar={deptToolbar}
        query={{ defaults: { orders: [{ field: "orderNum", desc: false }] } }}
        form={{
          createConfig: {
            title: "新建部门",
            initialValues: { parentId: 0n, orderNum: 0 },
            itemOverrides: deptCreateItems,
            onSuccess: async () => {
              message.success("创建成功");
            },
          },
          editConfig: {
            title: "编辑部门",
            initialValues: (entity) => ({
              parentId: entity.parentId,
              deptName: entity.deptName,
              orderNum: entity.orderNum,
              leader: entity.leader,
              phone: entity.phone,
              email: entity.email,
            }),
            itemOverrides: deptUpdateItems,
            onSuccess: async () => {
              message.success("已保存");
            },
          },
        }}
      />
    </PageContainer>
  );
}
