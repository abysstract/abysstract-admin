import { useContext, type ReactNode } from "react";
import { ConfigProvider } from "antd";

// AdminButtonSpacing 让 admin-core 组件默认不在两个汉字的按钮文字间插入空格。
// createAdminApp 的 Root 已在根 ConfigProvider 上设置该默认值，并允许宿主通过
// host.configProvider.button 覆盖；这里只为 Root 之外单独渲染的公共组件补默认值，
// 已有显式配置时不再包一层。
export function AdminButtonSpacing({ children }: { children: ReactNode }) {
  const { button } = useContext(ConfigProvider.ConfigContext);
  if (button?.autoInsertSpace !== undefined) return <>{children}</>;
  return <ConfigProvider button={{ ...button, autoInsertSpace: false }}>{children}</ConfigProvider>;
}
