import { useEffect, useMemo, type ComponentType } from "react";
import { useInRouterContext } from "react-router-dom";
import { App as AntdApp, ConfigProvider } from "antd";
import type { AdminApplication, AdminFrontendModule, AdminHost } from "./types";
import { validateHost } from "./host";
import { SessionController } from "./session";
import { createAdminTransport } from "./transport";
import { AdminRuntimeProvider, type AdminRuntimeState } from "./context";
import { AppErrorBoundary } from "./ErrorBoundary";
import { createSystemClients } from "../system/clients";
import { createSystemOperations } from "../system/operations";
import { createModuleRegistry } from "../module/registry";
import AdminApp from "../App";

export { bindAdminAppPageLifecycle } from "./page-lifecycle";
export type { BindAdminAppPageLifecycleInput, AdminPageLifecycleTarget } from "./page-lifecycle";

export function createAdminApp(input: {
  host: AdminHost;
  modules: readonly AdminFrontendModule[];
}): AdminApplication {
  const registry = createModuleRegistry(input.modules);
  const host = validateHost(input.host);
  const session = new SessionController(host.authStorage, host.reportError);
  const transport = createAdminTransport({ baseUrl: host.apiBaseUrl, session });
  const clients = createSystemClients(transport);
  const operations = createSystemOperations(transport);
  let disposed = false;
  let liveRoots = 0;

  const signOut = async () => {
    try {
      await clients.auth.logout({});
    } catch (error) {
      host.reportError?.(error, { operation: "auth.logout" });
    } finally {
      session.clearToken();
    }
  };

  const runtime: AdminRuntimeState = {
    transport,
    auth: { signOut },
    host,
    session,
    clients,
    operations,
    registry,
  };

  const Root: ComponentType = () => {
    if (disposed || session.isDisposed()) {
      throw new Error("createAdminApp 实例已释放，不能再次挂载");
    }
    if (!useInRouterContext()) {
      throw new Error("createAdminApp Root 需要由宿主提供 Router");
    }
    useEffect(() => {
      liveRoots += 1;
      if (liveRoots > 1) {
        liveRoots -= 1;
        throw new Error("一个 application 同时只能挂载一个 Root");
      }
      return () => {
        liveRoots -= 1;
      };
    }, []);
    const memo = useMemo(() => runtime, []);
    return (
      <ConfigProvider
        {...host.configProvider}
        button={{ autoInsertSpace: false, ...host.configProvider?.button }}
        theme={host.theme}
        locale={host.locale}
      >
        <AntdApp>
          <AdminRuntimeProvider value={memo}>
            <AppErrorBoundary
              onError={(error) => host.reportError?.(error, { operation: "app.render" })}
            >
              <AdminApp />
            </AppErrorBoundary>
          </AdminRuntimeProvider>
        </AntdApp>
      </ConfigProvider>
    );
  };

  return {
    Root,
    resumeFromPageCache() {
      if (disposed) return;
      session.resyncFromStorage();
    },
    dispose() {
      if (disposed) return;
      disposed = true;
      session.dispose();
    },
  };
}
