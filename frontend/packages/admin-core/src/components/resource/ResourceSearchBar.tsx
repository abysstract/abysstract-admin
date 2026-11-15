import type { CSSProperties, ReactNode } from "react";
import { Button, Input, Space, theme } from "antd";

import { useAdminTexts } from "../../runtime/texts";

// useResourceSurfaceStyle 返回 Resource 页面卡片（搜索栏、表格区）的主题色。
// 颜色来自 antd token，跟随应用主题和暗色算法；布局（间距、圆角以外的尺寸）在 CSS 中。
// 页面内的卡片不加阴影，用边框区分（见 DESIGN.md「阴影与层级」）。
export function useResourceSurfaceStyle(): CSSProperties {
  const { token } = theme.useToken();
  return {
    background: token.colorBgContainer,
    border: `${token.lineWidth}px ${token.lineType} ${token.colorBorderSecondary}`,
    borderRadius: token.borderRadiusLG,
  };
}

export interface ResourceSearchBarProps {
  /** 关键词输入框的值。 */
  readonly keyword: string;
  readonly onKeywordChange: (keyword: string) => void;
  /**
   * 点击「查询」、在输入框中按回车，或把关键词删空时调用。删空时传入空字符串，
   * 调用方应以它为准，而不是读取尚未更新的状态。
   */
  readonly onSearch: (keyword?: string) => void;
  /** 点击「刷新」；不传时不显示刷新按钮。 */
  readonly onReload?: () => void;
  /** 占位文字，写清搜索范围（如「用户名、昵称」）；默认「关键词」。 */
  readonly placeholder?: string;
  /** 放在关键词输入框之后、按钮之前的额外筛选控件。 */
  readonly children?: ReactNode;
  readonly className?: string;
  readonly style?: CSSProperties;
}

// ResourceSearchBar 是 ResourceTable 顶部的搜索卡片，可单独用于自定义列表页，
// 使其与 Resource 页面外观一致。
export function ResourceSearchBar({
  keyword,
  onKeywordChange,
  onSearch,
  onReload,
  placeholder,
  children,
  className,
  style,
}: ResourceSearchBarProps) {
  const surface = useResourceSurfaceStyle();
  const texts = useAdminTexts();
  return (
    <div
      className={className ? `resource-table__search ${className}` : "resource-table__search"}
      style={{ ...surface, ...style }}
    >
      <Space role="search" wrap>
        <Input
          aria-label={texts.table.keywordLabel}
          placeholder={placeholder ?? texts.table.searchPlaceholder}
          allowClear
          value={keyword}
          onChange={(event) => {
            const next = event.currentTarget.value;
            onKeywordChange(next);
            // 关键词删空时立即生效（见 DESIGN.md「数据列表」）。
            if (next === "" && keyword !== "") onSearch(next);
          }}
          onPressEnter={() => onSearch()}
        />
        {children}
        <Button type="primary" onClick={() => onSearch()}>
          {texts.action.search}
        </Button>
        {onReload ? <Button onClick={onReload}>{texts.action.reload}</Button> : null}
      </Space>
    </div>
  );
}
