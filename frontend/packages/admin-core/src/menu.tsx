import type { ReactNode } from "react";
import type { Menu } from "@abysstract/admin-proto/abysstract/admin/v1/menu_pb";
import type { ModuleRegistry } from "./module/registry";
import { normalizeMenuPath } from "./module/routes";

export type LayoutRoute = { path: string; name: string; icon?: ReactNode; routes?: LayoutRoute[] };

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
    if (menu.children?.length) {
      node.routes = buildLayoutRoutes(menu.children, registry, full);
    }
    return [node];
  });
}
