import type { Menu } from "@abysstract/admin-proto/abysstract/admin/v1/menu_pb";

export const RESERVED_PATHS = new Set(["/login"]);

export type PageRoute = {
  path: string;
  component: string;
  menuName: string;
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

function walkPages(menus: readonly Menu[], parentPath: string): PageRoute[] {
  const out: PageRoute[] = [];
  for (const menu of menus) {
    if (menu.menuType === "F") continue;
    const full = normalizeMenuPath(parentPath, menu.path);
    if (menu.menuType === "C" && menu.component) {
      out.push({ path: full, component: menu.component, menuName: menu.menuName });
    }
    if (menu.children?.length) {
      const childParent = menu.menuType === "M" || menu.menuType === "C" ? full : parentPath;
      out.push(...walkPages(menu.children, childParent));
    }
  }
  return out;
}

export function compileMenuPages(menus: readonly Menu[]): CompiledMenuPages {
  const flattened = walkPages(menus, "");
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
