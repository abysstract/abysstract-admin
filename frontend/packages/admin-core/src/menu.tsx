import type { ReactNode } from "react";
import type { Menu } from "@abysstract/admin-proto/abysstract/admin/v1/menu_pb";
import type { ModuleRegistry } from "./module/registry";
import { isHiddenMenu, normalizeMenuPath } from "./module/routes";

export type LayoutRoute = {
  path: string;
  name: string;
  icon?: ReactNode;
  routes?: LayoutRoute[];
  /** Hidden menus keep their route (breadcrumb, title) without a sidebar entry. */
  hideInMenu?: boolean;
};

export const unknownMenuIcon = <span data-icon="unknown" />;

export function buildLayoutRoutes(
  menus: readonly Menu[],
  registry: ModuleRegistry,
  parentPath = "",
): LayoutRoute[] {
  return menus.flatMap((menu) => {
    if (menu.menuType === "F") return [];
    const full = normalizeMenuPath(parentPath, menu.path);
    const node: LayoutRoute = {
      path: full,
      name: menu.menuName,
      icon: menu.icon ? (registry.icon(menu.icon) ?? unknownMenuIcon) : undefined,
    };
    if (isHiddenMenu(menu)) node.hideInMenu = true;
    if (menu.children?.length) {
      node.routes = buildLayoutRoutes(menu.children, registry, full);
    }
    return [node];
  });
}
