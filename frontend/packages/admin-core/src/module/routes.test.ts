import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import { MenuSchema, type Menu } from "@abysstract/admin-proto/abysstract/admin/v1/menu_pb";
import { compileMenuPages, normalizeMenuPath } from "./routes";

function menu(partial: {
  menuName: string;
  menuType: string;
  path?: string;
  component?: string;
  icon?: string;
  children?: Menu[];
}): Menu {
  return create(MenuSchema, partial);
}

describe("compileMenuPages", () => {
  it("returns no pages for an empty menu tree", () => {
    expect(compileMenuPages([])).toEqual({ pages: [], conflicts: [] });
  });

  it("builds nested C pages and skips F buttons", () => {
    const tree = [
      menu({
        menuName: "系统管理",
        menuType: "M",
        path: "system",
        icon: "setting",
        children: [
          menu({
            menuName: "用户管理",
            menuType: "C",
            path: "user",
            component: "system/user/index",
            children: [menu({ menuName: "用户新增", menuType: "F", path: "", component: "" })],
          }),
        ],
      }),
    ];
    expect(compileMenuPages(tree)).toEqual({
      pages: [{ path: "/system/user", component: "system/user/index", menuName: "用户管理" }],
      conflicts: [],
    });
  });

  it("detects the same normalized path from different parents", () => {
    const tree = [
      menu({
        menuName: "目录甲",
        menuType: "M",
        path: "system",
        children: [
          menu({
            menuName: "用户甲",
            menuType: "C",
            path: "user",
            component: "system/user/index",
          }),
        ],
      }),
      menu({
        menuName: "用户乙",
        menuType: "C",
        path: "/system/user",
        component: "biz/user/index",
      }),
    ];
    const compiled = compileMenuPages(tree);
    expect(compiled.pages).toEqual([]);
    expect(compiled.conflicts).toEqual([
      {
        path: "/system/user",
        reason: "duplicate",
        sources: ["用户甲 (system/user/index)", "用户乙 (biz/user/index)"],
      },
    ]);
  });

  it("flags reserved /login routes", () => {
    const compiled = compileMenuPages([
      menu({
        menuName: "登录页",
        menuType: "C",
        path: "login",
        component: "system/user/index",
      }),
    ]);
    expect(compiled.pages).toEqual([]);
    expect(compiled.conflicts[0]).toMatchObject({ path: "/login", reason: "reserved" });
  });

  it("keeps non-conflicting nested routes", () => {
    const compiled = compileMenuPages([
      menu({
        menuName: "系统管理",
        menuType: "M",
        path: "/system/",
        children: [
          menu({
            menuName: "角色管理",
            menuType: "C",
            path: "role",
            component: "system/role/index",
          }),
        ],
      }),
    ]);
    expect(compiled.pages).toEqual([
      { path: "/system/role", component: "system/role/index", menuName: "角色管理" },
    ]);
  });
});

describe("normalizeMenuPath", () => {
  it("joins relative segments and collapses slashes", () => {
    expect(normalizeMenuPath("", "system")).toBe("/system");
    expect(normalizeMenuPath("/system", "user")).toBe("/system/user");
    expect(normalizeMenuPath("/system", "/absolute")).toBe("/absolute");
    expect(normalizeMenuPath("/system/", "user/")).toBe("/system/user");
  });
});
