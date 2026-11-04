import { useMemo, type ChangeEvent } from "react";
import { Input } from "antd";
import { PageContainer } from "@ant-design/pro-components";
import {
  formatErrorMessage,
  useAccess,
  useAdminRuntime,
  useAppMessage,
} from "@abysstract/admin-core";
import { ResourceTable } from "@abysstract/admin-core/resource";
import type {
  ResourceColumnOverrides,
  ResourceFormItemOverrides,
  ResourceRowActionOverride,
  ResourceToolbarOverride,
} from "@abysstract/admin-core/resource";
import type { Product } from "../gen/proto/company/product/v1/product_pb";
import {
  createProductOperations,
  type ProductCreateValues,
  type ProductFilters,
  type ProductUpdateValues,
} from "../gen/resource/product.operations.gen";
import { createProductRaw } from "../gen/resource/product.raw.gen";
import { productSchema } from "../gen/resource/product.schema.gen";
import { formatDecimalBigint, parseDecimalBigint } from "../bigint";

type ProductCreateItems = ResourceFormItemOverrides<
  typeof productSchema,
  ProductCreateValues,
  "create",
  Product
>;
type ProductUpdateItems = ResourceFormItemOverrides<
  typeof productSchema,
  ProductUpdateValues,
  "update",
  Product
>;

function DecimalBigintInput({
  value,
  onChange,
  disabled,
  "aria-label": ariaLabel,
}: {
  value?: bigint;
  onChange: (next: bigint) => void;
  disabled?: boolean;
  "aria-label"?: string;
}) {
  return (
    <Input
      aria-label={ariaLabel}
      disabled={disabled}
      inputMode="numeric"
      value={formatDecimalBigint(value)}
      onChange={(event: ChangeEvent<HTMLInputElement>) => {
        const next = parseDecimalBigint(event.currentTarget.value);
        if (next === undefined) {
          if (event.currentTarget.value.trim() === "") {
            onChange(0n);
          }
          return;
        }
        onChange(next);
      }}
    />
  );
}

function PriceCentsInput(props: Omit<Parameters<typeof DecimalBigintInput>[0], "aria-label">) {
  return <DecimalBigintInput {...props} aria-label="价格(分)" />;
}

function DeptIdInput(props: Omit<Parameters<typeof DecimalBigintInput>[0], "aria-label">) {
  return <DecimalBigintInput {...props} aria-label="部门ID" />;
}

const productColumns: ResourceColumnOverrides<typeof productSchema, Product> = {
  id: { hidden: true },
  priceCents: {
    render: ({ value }) => formatDecimalBigint(value as bigint),
  },
  deptId: {
    render: ({ value }) => formatDecimalBigint(value as bigint),
  },
  createBy: { hidden: true },
  createTime: { hidden: true },
  updateTime: { hidden: true },
};

const productCreateItems: ProductCreateItems = {
  priceCents: {
    component: PriceCentsInput as NonNullable<ProductCreateItems["priceCents"]>["component"],
  },
  deptId: { component: DeptIdInput as NonNullable<ProductCreateItems["deptId"]>["component"] },
  remark: { valueType: "textarea" },
};

const productUpdateItems: ProductUpdateItems = {
  priceCents: {
    component: PriceCentsInput as NonNullable<ProductUpdateItems["priceCents"]>["component"],
  },
  remark: { valueType: "textarea" },
};

const rowActions: readonly ResourceRowActionOverride<Product, bigint>[] = [
  { action: "update", merge: "replace", title: "编辑" },
  { action: "delete", merge: "replace", title: "删除" },
];

const toolbar: readonly ResourceToolbarOverride<Product, bigint>[] = [
  { action: "create", merge: "replace", title: "新建商品" },
  { action: "batchDelete", merge: "replace", title: "批量删除" },
];

export default function ProductList() {
  const { transport } = useAdminRuntime();
  const operations = useMemo(
    () => createProductOperations(createProductRaw(transport)),
    [transport],
  );
  const message = useAppMessage();
  const { has } = useAccess();

  return (
    <PageContainer>
      <ResourceTable<
        typeof productSchema,
        Product,
        bigint,
        ProductFilters,
        ProductCreateValues,
        ProductUpdateValues
      >
        schema={productSchema}
        operations={operations}
        access={{ can: (permission) => has(permission) }}
        reload={async () => {
          message.success("删除成功");
        }}
        onError={(error) => message.error(formatErrorMessage(error))}
        columns={productColumns}
        rowActions={rowActions}
        toolbar={toolbar}
        form={{
          createConfig: {
            title: "新建商品",
            initialValues: { deptId: 1n },
            itemOverrides: productCreateItems,
            onSuccess: async () => {
              message.success("创建成功");
            },
          },
          editConfig: {
            title: "编辑商品",
            itemOverrides: productUpdateItems,
            onSuccess: async () => {
              message.success("已保存");
            },
          },
        }}
      />
    </PageContainer>
  );
}
