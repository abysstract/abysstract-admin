// 表格列设置持久化（需求 9.10）。
// ProTable 的 columnsState 通过 localStorage 按用户+页面键保存与恢复。
// 本模块提供存取纯函数，便于单元测试（属性 16）。

export type ColumnStateMap = Record<string, ColumnState>;

export type ColumnState = {
  show?: boolean;
  order?: number;
  fixed?: "left" | "right" | null;
};

function storageKey(userID: number | string, page: string): string {
  return `abysstract:columns:${userID}:${page}`;
}

// saveColumns：保存列设置。同一 user+page 键覆盖。
export function saveColumns(
  userID: number | string,
  page: string,
  state: ColumnStateMap,
  storage: Storage = localStorage,
): void {
  storage.setItem(storageKey(userID, page), JSON.stringify(state));
}

// loadColumns：恢复列设置。
// 不存在/解析失败返回空对象（由 ProTable 走默认值）。
export function loadColumns(
  userID: number | string,
  page: string,
  storage: Storage = localStorage,
): ColumnStateMap {
  const raw = storage.getItem(storageKey(userID, page));
  if (!raw) return {};
  try {
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === "object") return parsed as ColumnStateMap;
    return {};
  } catch {
    return {};
  }
}

// clearColumns：清除指定 user+page 的列设置（调试 / 重置）。
export function clearColumns(
  userID: number | string,
  page: string,
  storage: Storage = localStorage,
): void {
  storage.removeItem(storageKey(userID, page));
}

// persistenceKey 暴露存储 key 生成逻辑，供调用方自行拼装（如 columnsStateMap onChange）。
export function persistenceKey(userID: number | string, page: string): string {
  return storageKey(userID, page);
}
