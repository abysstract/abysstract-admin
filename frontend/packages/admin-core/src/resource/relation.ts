/**
 * A relation option loader is intentionally promise-based: target permission,
 * DataScope, NotFound, and network failures propagate to the consuming form
 * instead of being converted into an indistinguishable empty option set.
 */
export interface RelationOptionLoader<V extends string | number | bigint> {
  readonly remoteSearchEnabled: boolean;
  load(input: { pageNum: number; pageSize: number; keyword?: string }): Promise<{
    options: ReadonlyArray<{ value: V; label: string }>;
    nextPage?: number;
  }>;
}

type RelationPageInput = {
  pageNum: number;
  pageSize: number;
  keyword?: string;
};

type RelationPage<V extends string | number | bigint> = {
  options: ReadonlyArray<{ value: V; label: string }>;
  nextPage?: number;
};

/**
 * Adds the fixed remote-search policy around generated target List calls.
 * A non-searchable target receives no keyword property at all; errors remain
 * rejected so forms can display authorization, DataScope, and network errors.
 */
export function createRelationOptionLoader<V extends string | number | bigint>(config: {
  remoteSearchEnabled: boolean;
  loadPage: (input: RelationPageInput) => Promise<RelationPage<V>>;
}): RelationOptionLoader<V> {
  return {
    remoteSearchEnabled: config.remoteSearchEnabled,
    async load(input) {
      const pageInput: RelationPageInput = {
        pageNum: input.pageNum,
        pageSize: input.pageSize,
      };
      if (config.remoteSearchEnabled && input.keyword !== undefined) {
        pageInput.keyword = input.keyword;
      }
      return config.loadPage(pageInput);
    },
  };
}
