import type { ComponentType, LazyExoticComponent, ReactNode } from "react";
import type { AdminFrontendModule } from "../runtime/types";

export interface ModuleRegistry {
  readonly moduleNames: readonly string[];
  page(component: string): LazyExoticComponent<ComponentType> | undefined;
  icon(name: string): ReactNode | undefined;
  hasPage(component: string): boolean;
}

type PageComponent = LazyExoticComponent<ComponentType>;

function enumerableOwnStringKeys(value: object): string[] {
  return Object.getOwnPropertyNames(value).filter((key) => {
    const desc = Object.getOwnPropertyDescriptor(value, key);
    return desc?.enumerable === true;
  });
}

function requireName(mod: AdminFrontendModule | null | undefined, index: number): string {
  if (!mod || typeof mod.name !== "string" || mod.name.trim() === "") {
    throw new Error(`modules[${index}].name 必填`);
  }
  return mod.name;
}

function readPageMap(mod: AdminFrontendModule): Record<string, PageComponent> {
  if (mod.pages == null) return {};
  if (typeof mod.pages !== "object") {
    throw new Error(`模块 ${mod.name} 的 pages 必须是对象`);
  }
  return mod.pages as Record<string, PageComponent>;
}

function readIconMap(mod: AdminFrontendModule): Record<string, ReactNode> {
  if (mod.icons == null) return {};
  if (typeof mod.icons !== "object") {
    throw new Error(`模块 ${mod.name} 的 icons 必须是对象`);
  }
  return mod.icons as Record<string, ReactNode>;
}

export function createModuleRegistry(modules: readonly AdminFrontendModule[]): ModuleRegistry {
  if (!Array.isArray(modules)) {
    throw new Error("modules 必填");
  }

  const moduleNames: string[] = [];
  const seenNames = new Map<string, number>();
  const pages = new Map<string, PageComponent>();
  const icons = new Map<string, ReactNode>();
  const pageSources = new Map<string, string>();
  const iconSources = new Map<string, string>();
  const componentSources = new Map<PageComponent, { module: string; key: string }>();

  modules.forEach((mod, index) => {
    const name = requireName(mod, index);
    const previous = seenNames.get(name);
    if (previous !== undefined) {
      throw new Error(`模块名称重复：${name}（modules[${previous}] 与 modules[${index}]）`);
    }
    seenNames.set(name, index);
    moduleNames.push(name);

    const pageMap = readPageMap(mod);
    for (const key of enumerableOwnStringKeys(pageMap)) {
      if (key.trim() === "") {
        throw new Error(`页面标识必填：模块 ${name}`);
      }
      const component = pageMap[key];
      const existing = pageSources.get(key);
      if (existing !== undefined) {
        throw new Error(`页面标识重复：${key}（模块 ${existing} 与模块 ${name}）`);
      }
      const sameComponent = componentSources.get(component);
      if (sameComponent !== undefined) {
        throw new Error(
          `同一页面组件被重复注册：模块 ${sameComponent.module} / ${sameComponent.key} 与模块 ${name} / ${key}`,
        );
      }
      pages.set(key, component);
      pageSources.set(key, name);
      componentSources.set(component, { module: name, key });
    }

    const iconMap = readIconMap(mod);
    for (const key of enumerableOwnStringKeys(iconMap)) {
      if (key.trim() === "") {
        throw new Error(`图标标识必填：模块 ${name}`);
      }
      const existing = iconSources.get(key);
      if (existing !== undefined) {
        throw new Error(`图标标识重复：${key}（模块 ${existing} 与模块 ${name}）`);
      }
      icons.set(key, iconMap[key]);
      iconSources.set(key, name);
    }
  });

  return Object.freeze({
    moduleNames: Object.freeze([...moduleNames]),
    page(component: string) {
      return pages.get(component);
    },
    icon(name: string) {
      return icons.get(name);
    },
    hasPage(component: string) {
      return pages.has(component);
    },
  });
}
