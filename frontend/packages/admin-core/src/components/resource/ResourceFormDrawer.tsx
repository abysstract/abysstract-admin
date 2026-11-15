import { useState, type ReactNode } from "react";
import { Drawer } from "antd";

export interface ResourceFormDrawerProps {
  readonly open: boolean;
  readonly title: ReactNode;
  readonly onClose: () => void;
  /** 抽屉宽度只用两档：表单、详情 720；需要并排两栏的复杂编辑 1000。 */
  readonly width?: 720 | 1000;
  readonly loading?: boolean;
  /**
   * 抽屉内容。把 actionsContainer 传给 ResourceForm，表单的「取消 保存」按钮就会
   * 出现在抽屉标题栏右侧（见 DESIGN.md「按钮」）。
   */
  readonly children: (actionsContainer: HTMLElement | null) => ReactNode;
}

/** 新增、编辑用的表单抽屉：720 宽，按钮在右上，关闭后销毁内容。 */
export function ResourceFormDrawer({
  open,
  title,
  onClose,
  width = 720,
  loading,
  children,
}: ResourceFormDrawerProps) {
  const [actionsContainer, setActionsContainer] = useState<HTMLElement | null>(null);
  return (
    <Drawer
      title={title}
      open={open}
      onClose={onClose}
      size={width}
      loading={loading}
      destroyOnHidden
      extra={<span ref={setActionsContainer} />}
    >
      {open ? children(actionsContainer) : null}
    </Drawer>
  );
}
