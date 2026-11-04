import { Suspense, type ReactNode } from "react";
import { Result, Spin } from "antd";
import type { ModuleRegistry } from "./registry";
import type { RouteConflict } from "./routes";

export function renderRegisteredPage(component: string, registry: ModuleRegistry): ReactNode {
  const Comp = registry.page(component);
  if (!Comp) {
    return (
      <Result
        status="404"
        title="页面未注册"
        subTitle={`component=${component}。当前应用没有注册该页面，请检查模块 pages 表。`}
      />
    );
  }
  return (
    <Suspense fallback={<Spin spinning style={{ marginTop: 120, width: "100%" }} />}>
      <Comp />
    </Suspense>
  );
}

export function renderRouteConflicts(conflicts: readonly RouteConflict[]): ReactNode {
  const detail = conflicts
    .map((conflict) => {
      const reason = conflict.reason === "reserved" ? "保留路由" : "重复路径";
      return `${conflict.path}（${reason}）：${conflict.sources.join("；")}`;
    })
    .join("。");
  return <Result status="error" title="菜单路由冲突" subTitle={detail} />;
}
