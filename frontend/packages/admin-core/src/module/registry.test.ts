import { lazy, type ComponentType, type LazyExoticComponent } from "react";
import { describe, expect, it } from "vitest";
import { createModuleRegistry } from "./registry";

const PageA = lazy(() => Promise.resolve({ default: () => null }));
const PageB = lazy(() => Promise.resolve({ default: () => null }));
const PageC = lazy(() => Promise.resolve({ default: () => null }));

function page(key: string, component: LazyExoticComponent<ComponentType> = PageA) {
  return { [key]: component };
}

describe("createModuleRegistry", () => {
  it("rejects empty module names", () => {
    expect(() => createModuleRegistry([{ name: "" }])).toThrow(/name 必填/);
    expect(() => createModuleRegistry([{ name: "   " }])).toThrow(/name 必填/);
  });

  it("rejects duplicate module names", () => {
    expect(() =>
      createModuleRegistry([{ name: "system" }, { name: "system", pages: page("other", PageB) }]),
    ).toThrow(/模块名称重复：system/);
  });

  it("rejects duplicate page keys and names the source modules", () => {
    expect(() =>
      createModuleRegistry([
        { name: "alpha", pages: page("system/user/index", PageA) },
        { name: "beta", pages: page("system/user/index", PageB) },
      ]),
    ).toThrow(/页面标识重复：system\/user\/index（模块 alpha 与模块 beta）/);
  });

  it("rejects the same component registered twice", () => {
    expect(() =>
      createModuleRegistry([
        { name: "alpha", pages: page("one", PageA) },
        { name: "beta", pages: page("two", PageA) },
      ]),
    ).toThrow(/同一页面组件被重复注册：模块 alpha \/ one 与模块 beta \/ two/);
  });

  it("rejects duplicate icon keys", () => {
    expect(() =>
      createModuleRegistry([
        { name: "alpha", icons: { user: "A" } },
        { name: "beta", icons: { user: "B" } },
      ]),
    ).toThrow(/图标标识重复：user（模块 alpha 与模块 beta）/);
  });

  it("rejects empty page and icon keys", () => {
    expect(() => createModuleRegistry([{ name: "alpha", pages: page("  ", PageA) }])).toThrow(
      /页面标识必填：模块 alpha/,
    );
    expect(() => createModuleRegistry([{ name: "alpha", icons: { "": "x" } }])).toThrow(
      /图标标识必填：模块 alpha/,
    );
  });

  it("stores __proto__ as a normal key", () => {
    const pages = Object.create(null) as Record<string, LazyExoticComponent<ComponentType>>;
    const icons = Object.create(null) as Record<string, string>;
    Object.defineProperty(pages, "__proto__", {
      value: PageA,
      enumerable: true,
      configurable: true,
      writable: true,
    });
    Object.defineProperty(icons, "__proto__", {
      value: "icon",
      enumerable: true,
      configurable: true,
      writable: true,
    });
    const registry = createModuleRegistry([{ name: "probe", pages, icons }]);
    expect(registry.page("__proto__")).toBe(PageA);
    expect(registry.icon("__proto__")).toBe("icon");
    expect(Object.getPrototypeOf(pages)).toBeNull();
  });

  it("copies input maps so later mutation is ignored", () => {
    const pages: Record<string, LazyExoticComponent<ComponentType>> = { keep: PageA };
    const icons: Record<string, string> = { keep: "old" };
    const registry = createModuleRegistry([{ name: "biz", pages, icons }]);
    pages.keep = PageB;
    pages.extra = PageC;
    icons.keep = "new";
    icons.extra = "x";
    expect(registry.page("keep")).toBe(PageA);
    expect(registry.hasPage("extra")).toBe(false);
    expect(registry.icon("keep")).toBe("old");
    expect(registry.icon("extra")).toBeUndefined();
  });

  it("keeps the same key independent across registries", () => {
    const first = createModuleRegistry([{ name: "one", pages: page("shared", PageA) }]);
    const second = createModuleRegistry([{ name: "two", pages: page("shared", PageB) }]);
    expect(first.page("shared")).toBe(PageA);
    expect(second.page("shared")).toBe(PageB);
  });

  it("freezes moduleNames so runtime mutation is rejected", () => {
    const registry = createModuleRegistry([{ name: "system" }]);
    expect(Object.isFrozen(registry)).toBe(true);
    expect(Object.isFrozen(registry.moduleNames)).toBe(true);
    expect(registry.moduleNames).toEqual(["system"]);
    expect(() => (registry.moduleNames as string[]).push("injected")).toThrow(TypeError);
    expect(registry.moduleNames).toEqual(["system"]);
  });
});
