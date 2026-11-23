import type { Menu } from "@abysstract/admin-proto/abysstract/admin/v1/menu_pb";

export const RESERVED_PATHS = new Set(["/login"]);

/**
 * `Menu.visible` value of a page that has a route and a permission check but
 * no sidebar entry, for pages reached only by links (detail pages). Set it
 * with `"hidden": true` in a module seed. `1` is an ordinary visible menu.
 */
export const MENU_HIDDEN = 2;

export function isHiddenMenu(menu: Pick<Menu, "visible">): boolean {
  return menu.visible === MENU_HIDDEN;
}

export type PageRoute = {
  path: string;
  component: string;
  menuName: string;
  /** No sidebar entry (the menu or one of its ancestors is hidden). */
  hidden: boolean;
};

export type RouteConflict = {
  path: string;
  reason: "duplicate" | "reserved";
  sources: string[];
};

export type CompiledMenuPages = {
  pages: PageRoute[];
  conflicts: RouteConflict[];
};

export function normalizeMenuPath(parentPath: string, path: string): string {
  const joined = path.startsWith("/") ? path : `${parentPath}/${path}`;
  const parts: string[] = [];
  for (const part of joined.split("/")) {
    if (part === "" || part === ".") continue;
    if (part === "..") {
      parts.pop();
      continue;
    }
    parts.push(part);
  }
  return `/${parts.join("/")}`;
}

function walkPages(menus: readonly Menu[], parentPath: string, parentHidden: boolean): PageRoute[] {
  const out: PageRoute[] = [];
  for (const menu of menus) {
    if (menu.menuType === "F") continue;
    const full = normalizeMenuPath(parentPath, menu.path);
    const hidden = parentHidden || isHiddenMenu(menu);
    if (menu.menuType === "C" && menu.component) {
      out.push({ path: full, component: menu.component, menuName: menu.menuName, hidden });
    }
    if (menu.children?.length) {
      const childParent = menu.menuType === "M" || menu.menuType === "C" ? full : parentPath;
      out.push(...walkPages(menu.children, childParent, hidden));
    }
  }
  return out;
}

export function compileMenuPages(menus: readonly Menu[]): CompiledMenuPages {
  const flattened = walkPages(menus, "", false);
  const sources = new Map<string, string[]>();
  for (const page of flattened) {
    const list = sources.get(page.path);
    const label = `${page.menuName} (${page.component})`;
    if (list) list.push(label);
    else sources.set(page.path, [label]);
  }

  const conflicts: RouteConflict[] = [];
  const conflictPaths = new Set<string>();
  for (const [path, labels] of sources) {
    if (RESERVED_PATHS.has(path)) {
      conflicts.push({ path, reason: "reserved", sources: labels });
      conflictPaths.add(path);
      continue;
    }
    if (labels.length > 1) {
      conflicts.push({ path, reason: "duplicate", sources: labels });
      conflictPaths.add(path);
    }
  }

  return {
    pages: flattened.filter((page) => !conflictPaths.has(page.path)),
    conflicts,
  };
}
