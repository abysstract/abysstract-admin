import type { ResourceListQuery } from "./operations";

// 列表的关键词、页码、每页条数、排序和结构化筛选写进地址栏（见 DESIGN.md「导航」：可分享的位置都在地址里）。

export type UrlQueryState = {
  readonly keyword?: string;
  readonly pageNum?: number;
  readonly pageSize?: number;
  readonly sort?: { readonly field: string; readonly desc: boolean };
};

const URL_PARAMS = { keyword: "q", page: "page", size: "size", sort: "sort" } as const;

/** Value types a structured filter can carry in the address bar. */
export type UrlFilterType = "bigint" | "number" | "string" | "boolean";

const RESERVED_PARAMS: ReadonlySet<string> = new Set(Object.values(URL_PARAMS));

/**
 * Query parameter of a structured filter: its own name (`?deptId=3`), so the
 * address stays readable. A filter named like a list parameter (`q`, `page`,
 * `size`, `sort`) is written as `f.<name>`.
 */
export function urlFilterParam(name: string): string {
  return RESERVED_PARAMS.has(name) ? `f.${name}` : name;
}

function encodeFilterValue(value: unknown): string | undefined {
  switch (typeof value) {
    case "bigint":
    case "boolean":
      return String(value);
    case "number":
      return Number.isFinite(value) ? String(value) : undefined;
    case "string":
      return value;
    default:
      return undefined;
  }
}

function decodeFilterValue(text: string, type: UrlFilterType): unknown {
  switch (type) {
    case "bigint":
      return /^-?\d+$/.test(text) ? BigInt(text) : undefined;
    case "number": {
      if (text.trim() === "") return undefined;
      const value = Number(text);
      return Number.isFinite(value) ? value : undefined;
    }
    case "boolean":
      return text === "true" ? true : text === "false" ? false : undefined;
    case "string":
      return text;
  }
}

/**
 * Reads the structured filters named in `types` from the address bar. Values
 * that do not parse as the declared type are ignored, so a hand-edited or
 * outdated link falls back to the page defaults instead of failing.
 */
export function readUrlFilters(
  types: Readonly<Record<string, UrlFilterType>>,
): Record<string, unknown> {
  if (typeof window === "undefined") return {};
  const params = new URLSearchParams(window.location.search);
  const filters: Record<string, unknown> = {};
  for (const [name, type] of Object.entries(types)) {
    const text = params.get(urlFilterParam(name));
    if (text === null) continue;
    const value = decodeFilterValue(text, type);
    if (value !== undefined) filters[name] = value;
  }
  return filters;
}

export function readUrlQuery(): UrlQueryState {
  if (typeof window === "undefined") return {};
  const params = new URLSearchParams(window.location.search);
  const positive = (name: string): number | undefined => {
    const value = Number(params.get(name));
    return Number.isInteger(value) && value > 0 ? value : undefined;
  };
  const sortText = params.get(URL_PARAMS.sort) ?? "";
  const sortField = sortText.replace(/^-/, "");
  return {
    keyword: params.get(URL_PARAMS.keyword) ?? undefined,
    pageNum: positive(URL_PARAMS.page),
    pageSize: positive(URL_PARAMS.size),
    sort: sortField === "" ? undefined : { field: sortField, desc: sortText.startsWith("-") },
  };
}

/**
 * Writes the list state to the address bar with `replaceState`. Only values
 * that differ from `defaults` are written. Structured filters are written for
 * the names in `filterTypes`; a filter equal to its default, or unset, is
 * removed from the address.
 */
export function writeUrlQuery(
  query: ResourceListQuery<object, string>,
  defaults: ResourceListQuery<object, string>,
  filterTypes: Readonly<Record<string, UrlFilterType>> = {},
): void {
  if (typeof window === "undefined") return;
  const url = new URL(window.location.href);
  const set = (name: string, value: string | undefined) => {
    if (value === undefined || value === "") url.searchParams.delete(name);
    else url.searchParams.set(name, value);
  };
  set(URL_PARAMS.keyword, query.keyword);
  set(URL_PARAMS.page, query.pageNum === 1 ? undefined : String(query.pageNum));
  set(URL_PARAMS.size, query.pageSize === defaults.pageSize ? undefined : String(query.pageSize));
  const order = query.orders?.[0];
  const defaultOrder = defaults.orders?.[0];
  const sameAsDefault =
    order?.field === defaultOrder?.field && Boolean(order?.desc) === Boolean(defaultOrder?.desc);
  set(
    URL_PARAMS.sort,
    order === undefined || sameAsDefault ? undefined : `${order.desc ? "-" : ""}${order.field}`,
  );
  const filters = (query.filters ?? {}) as Readonly<Record<string, unknown>>;
  const defaultFilters = (defaults.filters ?? {}) as Readonly<Record<string, unknown>>;
  for (const name of Object.keys(filterTypes)) {
    const value = filters[name];
    const param = urlFilterParam(name);
    if (value === undefined || value === defaultFilters[name]) {
      url.searchParams.delete(param);
      continue;
    }
    const text = encodeFilterValue(value);
    // An empty string is a real filter value ("equals empty"), so keep `name=`.
    if (text === undefined) url.searchParams.delete(param);
    else url.searchParams.set(param, text);
  }
  if (url.href !== window.location.href) {
    // Keep the router's history state so back/forward still restores this entry.
    window.history.replaceState(window.history.state, "", url);
  }
}

/** 读写地址栏里的其他筛选参数（例如日志页的结果、时间范围）。 */
export function readUrlParam(name: string): string | undefined {
  if (typeof window === "undefined") return undefined;
  return new URLSearchParams(window.location.search).get(name) ?? undefined;
}

export function writeUrlParams(values: Readonly<Record<string, string | undefined>>): void {
  if (typeof window === "undefined") return;
  const url = new URL(window.location.href);
  for (const [name, value] of Object.entries(values)) {
    if (value === undefined || value === "") url.searchParams.delete(name);
    else url.searchParams.set(name, value);
  }
  if (url.href !== window.location.href) {
    window.history.replaceState(window.history.state, "", url);
  }
}
