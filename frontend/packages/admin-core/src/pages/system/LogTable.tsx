import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { Button, Empty } from "antd";
import { ProTable, type ProColumns } from "@ant-design/pro-components";

import {
  ResourceSearchBar,
  useResourceSurfaceStyle,
} from "../../components/resource/ResourceSearchBar";
import { DEFAULT_RESOURCE_PAGE_SIZE } from "../../components/resource/ResourceTable";
import { readUrlQuery, writeUrlParams, writeUrlQuery } from "../../components/resource/urlQuery";
import { formatErrorMessage } from "../../runtime/message";
import { useAdminTexts } from "../../runtime/texts";
import "../../components/resource/ResourceTable.css";

export type LogFilters = Readonly<Record<string, string | undefined>>;

export interface LogQuery {
  readonly keyword: string;
  readonly pageNum: number;
  readonly pageSize: number;
  readonly filters: LogFilters;
}

export interface LogTableProps<T extends Record<string, unknown>> {
  /** 关键词框的占位文字，写清搜索范围。 */
  readonly placeholder: string;
  /** 下拉、日期等筛选条件；改了立即生效，并写进地址栏。 */
  readonly filters?: LogFilters;
  /** 放在关键词框之后的筛选控件。 */
  readonly filterControls?: ReactNode;
  /** 清除筛选时调用，页面把 filters 重置为空。 */
  readonly onClearFilters?: () => void;
  /** 写进地址栏的筛选键；默认全部。 */
  readonly urlFilterKeys?: readonly string[];
  readonly fetch: (query: LogQuery) => Promise<{ items: readonly T[]; total: number }>;
  readonly columns: ProColumns<T>[];
  readonly rowKey: (row: T) => string;
  /** 结果面板右上角的页面操作，例如「清空」。参数是重新加载当前页的函数。 */
  readonly actions?: (reload: () => Promise<void>) => ReactNode;
  /** 第一次使用（没有任何日志）时的空状态文案。 */
  readonly emptyText: string;
  /** 有筛选条件但没有结果时的空状态文案。 */
  readonly noMatchText: string;
}

// 日志页没有 Resource schema，这里按 ResourceTable 的样子组合：上面筛选面板，下面结果面板，
// 关键词、页码和筛选写进地址栏（见 DESIGN.md「数据列表」「导航」）。
export function LogTable<T extends Record<string, unknown>>({
  placeholder,
  filters = {},
  filterControls,
  onClearFilters,
  urlFilterKeys,
  fetch,
  columns,
  rowKey,
  actions,
  emptyText,
  noMatchText,
}: LogTableProps<T>) {
  const texts = useAdminTexts();
  const surface = useResourceSurfaceStyle();
  const initial = useMemo(() => readUrlQuery(), []);
  const [keyword, setKeyword] = useState(initial.keyword ?? "");
  const [query, setQuery] = useState<Omit<LogQuery, "filters">>({
    keyword: initial.keyword ?? "",
    pageNum: initial.pageNum ?? 1,
    pageSize: initial.pageSize ?? DEFAULT_RESOURCE_PAGE_SIZE,
  });
  const [rows, setRows] = useState<readonly T[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<unknown>();
  const latest = useRef(0);
  const filtersKey = JSON.stringify(filters);
  const firstFilters = useRef(true);

  const load = useCallback(
    async (next: Omit<LogQuery, "filters">) => {
      const id = ++latest.current;
      setQuery(next);
      setLoading(true);
      setError(undefined);
      writeUrlQuery(
        { keyword: next.keyword, pageNum: next.pageNum, pageSize: next.pageSize },
        { pageNum: 1, pageSize: DEFAULT_RESOURCE_PAGE_SIZE },
      );
      const currentFilters = JSON.parse(filtersKey) as LogFilters;
      writeUrlParams(
        urlFilterKeys === undefined
          ? currentFilters
          : Object.fromEntries(urlFilterKeys.map((key) => [key, currentFilters[key]])),
      );
      try {
        const result = await fetch({ ...next, filters: currentFilters });
        if (id !== latest.current) return;
        setRows(result.items);
        setTotal(result.total);
      } catch (cause) {
        if (id === latest.current) setError(cause);
      } finally {
        if (id === latest.current) setLoading(false);
      }
    },
    [fetch, filtersKey, urlFilterKeys],
  );

  useEffect(() => {
    // Filters apply immediately and restart from page 1; the first run keeps the URL page.
    const next = firstFilters.current ? query : { ...query, pageNum: 1 };
    firstFilters.current = false;
    void load(next);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [filtersKey]);

  const reload = useCallback(() => load(query), [load, query]);
  const filtered =
    query.keyword !== "" ||
    Object.values(filters).some((value) => value !== undefined && value !== "");

  return (
    <div className="resource-table">
      <ResourceSearchBar
        keyword={keyword}
        placeholder={placeholder}
        onKeywordChange={setKeyword}
        onSearch={(next) => void load({ ...query, keyword: next ?? keyword, pageNum: 1 })}
        onReload={() => void reload()}
      >
        {filterControls}
      </ResourceSearchBar>
      <div className="resource-table__content" style={surface}>
        {error !== undefined ? (
          <div role="alert" className="resource-table__error">
            {formatErrorMessage(error, texts.feedback.cannotLoad)}
            <Button type="link" onClick={() => void reload()}>
              {texts.action.retry}
            </Button>
          </div>
        ) : (
          <ProTable<T>
            rowKey={rowKey}
            search={false}
            options={false}
            columns={columns}
            dataSource={rows as T[]}
            loading={loading}
            toolBarRender={actions ? () => [<span key="actions">{actions(reload)}</span>] : false}
            locale={{
              emptyText: loading ? (
                texts.table.loading
              ) : (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={filtered ? noMatchText : emptyText}
                >
                  {filtered ? (
                    <Button
                      onClick={() => {
                        setKeyword("");
                        const cleared = { ...query, keyword: "", pageNum: 1 };
                        const hasFilters = Object.values(filters).some(
                          (value) => value !== undefined && value !== "",
                        );
                        if (hasFilters && onClearFilters !== undefined) {
                          // The filters effect reloads with this query once the page clears them.
                          setQuery(cleared);
                          onClearFilters();
                        } else {
                          void load(cleared);
                        }
                      }}
                    >
                      {texts.action.clearFilters}
                    </Button>
                  ) : null}
                </Empty>
              ),
            }}
            pagination={{
              current: query.pageNum,
              pageSize: query.pageSize,
              total,
              hideOnSinglePage: true,
              showSizeChanger: true,
              showQuickJumper: total > query.pageSize * 5,
              onChange: (pageNum, pageSize) => void load({ ...query, pageNum, pageSize }),
            }}
          />
        )}
      </div>
    </div>
  );
}
