import type { CSSProperties, ReactNode } from "react";
import { Button, Input, Space, theme } from "antd";

// useResourceSurfaceStyle 返回 Resource 页面卡片（搜索栏、表格区）的主题色。
// 颜色来自 antd token，跟随应用主题和暗色算法；布局（间距、圆角以外的尺寸）在 CSS 中。
export function useResourceSurfaceStyle(): CSSProperties {
  const { token } = theme.useToken();
  return {
    background: token.colorBgContainer,
    border: `${token.lineWidth}px ${token.lineType} ${token.colorBorderSecondary}`,
    borderRadius: token.borderRadiusLG,
    boxShadow: token.boxShadowTertiary,
  };
}

export interface ResourceSearchBarProps {
  /** 关键词输入框的值。 */
  readonly keyword: string;
  readonly onKeywordChange: (keyword: string) => void;
  /** 点击「查询」或在输入框中按回车。 */
  readonly onSearch: () => void;
  /** 点击「刷新」；不传时不显示刷新按钮。 */
  readonly onReload?: () => void;
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
  placeholder = "请输入关键词",
  children,
  className,
  style,
}: ResourceSearchBarProps) {
  const surface = useResourceSurfaceStyle();
  return (
    <div
      className={className ? `resource-table__search ${className}` : "resource-table__search"}
      style={{ ...surface, ...style }}
    >
      <Space role="search" wrap>
        <Input
          aria-label="关键词"
          placeholder={placeholder}
          value={keyword}
          onChange={(event) => onKeywordChange(event.currentTarget.value)}
          onPressEnter={onSearch}
        />
        {children}
        <Button type="primary" onClick={onSearch}>
          查询
        </Button>
        {onReload ? <Button onClick={onReload}>刷新</Button> : null}
      </Space>
    </div>
  );
}
