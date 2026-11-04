import { lazy, type ReactNode } from "react";
import { ShoppingOutlined } from "@ant-design/icons";
import type { AdminFrontendModule } from "@abysstract/admin-core";

const pages = {
  "biz/product/index": lazy(() => import("./pages/ProductList")),
} as const;

const icons: Readonly<Record<string, ReactNode>> = {
  product: <ShoppingOutlined />,
};

export const productModule: AdminFrontendModule = Object.freeze({
  name: "product",
  pages: Object.freeze({ ...pages }),
  icons: Object.freeze({ ...icons }),
});
