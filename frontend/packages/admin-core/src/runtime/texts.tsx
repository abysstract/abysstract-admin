import { createContext, useContext, useMemo, type ReactNode } from "react";

// 框架共用组件（ResourceTable、ResourceForm、ResourceSearchBar 与系统页面）的界面文案。
// 写法见 DESIGN.md「文案」：称呼用“你”，出错写“无法……”，成功不写“成功”二字。
// 宿主通过 AdminHost.texts 覆盖其中一部分，未覆盖的沿用默认值。
export interface AdminTexts {
  readonly action: {
    readonly create: string;
    readonly edit: string;
    readonly delete: string;
    readonly save: string;
    readonly cancel: string;
    readonly search: string;
    readonly reload: string;
    readonly retry: string;
    readonly back: string;
    readonly clearFilters: string;
    readonly clearSelection: string;
    readonly more: string;
  };
  readonly table: {
    readonly actionColumn: string;
    readonly searchPlaceholder: string;
    readonly keywordLabel: string;
    readonly loading: string;
    /** 第一次使用：从来没有过数据。 */
    readonly empty: string;
    /** 没有结果：有数据，但被关键词挡住了。 */
    readonly noMatch: string;
    readonly selected: (count: number) => string;
  };
  readonly confirm: {
    readonly deleteTitle: string;
    readonly batchDeleteTitle: (count: number) => string;
    readonly deleteContent: string;
  };
  readonly feedback: {
    readonly created: string;
    readonly saved: string;
    readonly deleted: string;
    readonly batchDeleted: (count: number) => string;
    readonly cannotDelete: string;
    readonly cannotSave: string;
    readonly cannotLoad: string;
    /** 无法归类的失败的兜底文案。 */
    readonly fallbackError: string;
  };
  readonly form: {
    readonly required: (label: string) => string;
    readonly requiredSelect: (label: string) => string;
    readonly maxLength: (max: number) => string;
  };
}

export const defaultAdminTexts: AdminTexts = {
  action: {
    create: "新增",
    edit: "编辑",
    delete: "删除",
    save: "保存",
    cancel: "取消",
    search: "查询",
    reload: "刷新",
    retry: "重试",
    back: "返回",
    clearFilters: "清除筛选",
    clearSelection: "取消选择",
    more: "更多",
  },
  table: {
    actionColumn: "操作",
    searchPlaceholder: "关键词",
    keywordLabel: "关键词",
    loading: "加载中",
    empty: "还没有记录",
    noMatch: "没有符合条件的记录",
    selected: (count) => `已选 ${count} 项`,
  },
  confirm: {
    deleteTitle: "删除这条记录？",
    batchDeleteTitle: (count) => `删除选中的 ${count} 条记录？`,
    deleteContent: "删除后无法恢复。",
  },
  feedback: {
    created: "已创建",
    saved: "已保存",
    deleted: "已删除",
    batchDeleted: (count) => `已删除 ${count} 条`,
    cannotDelete: "无法删除",
    cannotSave: "无法保存",
    cannotLoad: "无法加载",
    fallbackError: "无法完成操作，请稍后重试",
  },
  form: {
    required: (label) => `请填写${label}`,
    requiredSelect: (label) => `请选择${label}`,
    maxLength: (max) => `最多 ${max} 个字`,
  },
};

type DeepPartial<T> = {
  readonly [K in keyof T]?: T[K] extends (...args: never[]) => unknown
    ? T[K]
    : T[K] extends object
      ? DeepPartial<T[K]>
      : T[K];
};

export type AdminTextsOverride = DeepPartial<AdminTexts>;

export function mergeAdminTexts(override?: AdminTextsOverride): AdminTexts {
  if (override === undefined) return defaultAdminTexts;
  const merged: Record<string, unknown> = {};
  for (const [group, defaults] of Object.entries(defaultAdminTexts)) {
    merged[group] = { ...defaults, ...(override as Record<string, object | undefined>)[group] };
  }
  return merged as unknown as AdminTexts;
}

const TextsContext = createContext<AdminTexts>(defaultAdminTexts);

export function AdminTextsProvider({
  texts,
  children,
}: {
  texts?: AdminTextsOverride;
  children: ReactNode;
}) {
  const value = useMemo(() => mergeAdminTexts(texts), [texts]);
  return <TextsContext.Provider value={value}>{children}</TextsContext.Provider>;
}

/** 当前生效的界面文案；在 createAdminApp Root 之外渲染时返回默认文案。 */
export function useAdminTexts(): AdminTexts {
  return useContext(TextsContext);
}
