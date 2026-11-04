import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useNavigate } from "react-router-dom";
import { Button, Card, Result, Spin, Typography } from "antd";
import { Code, ConnectError } from "@connectrpc/connect";
import type { Menu } from "@abysstract/admin-proto/abysstract/admin/v1/menu_pb";
import { useAdminRuntimeState } from "./context";
import { formatErrorMessage } from "./message";

export interface AccessValue {
  loaded: boolean;
  permissions: readonly string[];
  roles: readonly string[];
  menus: readonly Menu[];
  nickname: string;
  pwdResetRequired: boolean;
  has(permission: string): boolean;
}

const AccessContext = createContext<AccessValue | null>(null);

const emptyAccess: AccessValue = {
  loaded: false,
  permissions: [],
  roles: [],
  menus: [],
  nickname: "用户",
  pwdResetRequired: false,
  has: () => false,
};

export function AccessProvider({ children }: { children: ReactNode }) {
  const runtime = useAdminRuntimeState();
  const navigate = useNavigate();
  // navigate 的引用随每次导航变化（BrowserRouter 下 useNavigate 非稳定），
  // 不能作为 load 的依赖，否则每次切菜单都会重跑加载并闪全屏 loading。
  const navigateRef = useRef(navigate);
  const [access, setAccess] = useState<AccessValue>(emptyAccess);
  const [loadError, setLoadError] = useState<unknown>(null);
  const [retry, setRetry] = useState(0);
  // 记录已成功加载的 access 所属的会话 epoch：同 epoch 内刷新时保留旧数据渲染，
  // 只有首次加载或 epoch 切换（登录态变化）才回退到全屏 loading。
  const loadedEpochRef = useRef<number | null>(null);

  const load = useCallback(() => {
    const snapshot = runtime.session.snapshot();
    if (!snapshot.token) {
      loadedEpochRef.current = null;
      setAccess(emptyAccess);
      setLoadError(null);
      return () => undefined;
    }
    let cancelled = false;
    setAccess((current) => (loadedEpochRef.current === snapshot.epoch ? current : emptyAccess));
    setLoadError(null);
    runtime.clients.auth
      .getUserInfo({})
      .then((res) => {
        if (cancelled || runtime.session.isDisposed()) return;
        const current = runtime.session.snapshot();
        if (current.epoch !== snapshot.epoch) return;
        loadedEpochRef.current = current.epoch;
        const permissions = res.permissions;
        setAccess({
          loaded: true,
          permissions,
          roles: res.roles,
          menus: res.menus,
          nickname: res.user?.nickname || res.user?.username || "用户",
          pwdResetRequired: res.pwdResetRequired,
          has: (perm: string) => permissions.includes("*:*:*") || permissions.includes(perm),
        });
      })
      .catch((error: unknown) => {
        if (cancelled || runtime.session.isDisposed()) return;
        const current = runtime.session.snapshot();
        if (current.epoch !== snapshot.epoch) return;
        if (error instanceof ConnectError && error.code === Code.Unauthenticated) {
          loadedEpochRef.current = null;
          setAccess(emptyAccess);
          navigateRef.current("/login", { replace: true });
          return;
        }
        setLoadError(error);
      });
    return () => {
      cancelled = true;
    };
  }, [runtime, retry]);

  useEffect(() => {
    navigateRef.current = navigate;
  }, [navigate]);

  useEffect(() => load(), [load]);

  useEffect(() => {
    return runtime.session.subscribe(() => {
      setRetry((value) => value + 1);
    });
  }, [runtime.session]);

  if (loadError) {
    return (
      <Result
        status="error"
        title="加载用户信息失败"
        subTitle={formatErrorMessage(loadError, "网络异常，请重试")}
        extra={
          <Button type="primary" onClick={() => setRetry((value) => value + 1)}>
            重试
          </Button>
        }
      />
    );
  }

  if (!access.loaded) {
    return (
      <div
        role="status"
        aria-live="polite"
        style={{
          minHeight: "100vh",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          background: "#f0f2f5",
        }}
      >
        <Card style={{ width: 380, textAlign: "center" }}>
          <Typography.Title level={3}>{runtime.host.title}</Typography.Title>
          <div aria-hidden="true" style={{ paddingTop: 16 }}>
            <Spin size="large" />
          </div>
          <div style={{ marginTop: 24, marginBottom: 8 }}>
            <Typography.Text type="secondary">正在进入工作台…</Typography.Text>
          </div>
        </Card>
      </div>
    );
  }

  return <AccessContext.Provider value={access}>{children}</AccessContext.Provider>;
}

export function useAccess(): AccessValue {
  const value = useContext(AccessContext);
  if (!value) {
    throw new Error("useAccess 必须在 createAdminApp Root 内调用");
  }
  return value;
}
