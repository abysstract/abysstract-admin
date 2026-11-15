import type { ResourceListQuery } from "./operations";

// 列表的关键词、页码、每页条数和排序写进地址栏（见 DESIGN.md「导航」：可分享的位置都在地址里）。

export type UrlQueryState = {
  readonly keyword?: string;
  readonly pageNum?: number;
  readonly pageSize?: number;
  readonly sort?: { readonly field: string; readonly desc: boolean };
};

const URL_PARAMS = { keyword: "q", page: "page", size: "size", sort: "sort" } as const;

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

export function writeUrlQuery(
  query: ResourceListQuery<object, string>,
  defaults: ResourceListQuery<object, string>,
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
