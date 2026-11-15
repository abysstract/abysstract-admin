import type {
  DictTypeCreateValues,
  DictTypeFilters,
  DictTypeUpdateValues,
} from "../../gen/resource/dict-type.operations.gen";
import { useSystemOperations } from "../../system/hooks";
import { useState } from "react";
import { PageContainer } from "@ant-design/pro-components";
import { Typography } from "antd";
import { useAccess } from "../../runtime/access";
import type { DictType } from "@abysstract/admin-proto/abysstract/admin/v1/dict_pb";
import { dictTypeSchema } from "../../gen/resource/dict-type.schema.gen";
import { ResourceTable } from "../../components/resource";
import type {
  ResourceColumnOverrides,
  ResourceFormItemOverrides,
  ResourceToolbarOverride,
} from "../../components/resource";
import { statusTag, timeCell } from "../../util";
import DictDataDrawer from "./DictDataDrawer";

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

// 点字典名称打开它的数据（见 DESIGN.md「数据列表」：点名称打开，不另放“查看”）。
function dictTypeColumns(
  openData: (entity: DictType) => void,
): ResourceColumnOverrides<typeof dictTypeSchema, DictType> {
  return {
    id: { hidden: true },
    dictName: {
      title: "字典名称",
      render: ({ entity }) => (
        <Typography.Link onClick={() => openData(entity)}>{entity.dictName}</Typography.Link>
      ),
    },
    dictType: {
      title: "字典类型",
      render: ({ value }) => <Typography.Text type="secondary">{value}</Typography.Text>,
    },
    status: { title: "状态", render: ({ value }) => statusTag(value) },
    remark: { hidden: true },
    createTime: { title: "创建时间", render: ({ value }) => timeCell(value) },
    updateTime: { hidden: true },
  };
}

const dictTypeCreateItems: DictTypeCreateItems = {
  dictName: { label: "字典名称" },
  dictType: { label: "字典类型", sameRowAsPrevious: true },
  remark: { label: "备注", valueType: "textarea" },
};

const dictTypeUpdateItems: DictTypeUpdateItems = {
  dictName: { label: "字典名称" },
  remark: { label: "备注", valueType: "textarea" },
};

const dictTypeToolbar: readonly ResourceToolbarOverride<DictType, bigint>[] = [];

export default function DictList() {
  const { dictTypeOperations } = useSystemOperations();
  const { has } = useAccess();
  const [drawerType, setDrawerType] = useState("");
  const [drawerOpen, setDrawerOpen] = useState(false);

  const openData = (entity: DictType) => {
    setDrawerType(entity.dictType);
    setDrawerOpen(true);
  };

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
        columns={dictTypeColumns(openData)}
        toolbar={dictTypeToolbar}
        form={{
          createConfig: {
            title: "新增字典",
            itemOverrides: dictTypeCreateItems,
          },
          editConfig: {
            title: "编辑字典",
            initialValues: (entity) => ({
              dictName: entity.dictName,
              remark: entity.remark,
            }),
            itemOverrides: dictTypeUpdateItems,
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
