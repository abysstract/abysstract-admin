import type { ConfigProviderProps, ThemeConfig } from "antd";
import type { Transport } from "@connectrpc/connect";
import type { LazyExoticComponent, ComponentType, ReactNode } from "react";

export interface AuthStorage {
  getToken(): string | null;
  setToken(token: string): void;
  clearToken(): void;
  subscribe(listener: () => void): () => void;
}

export interface AdminHost {
  apiBaseUrl: string;
  mode: "development" | "production";
  authStorage: AuthStorage;
  title: string;
  theme?: ThemeConfig;
  locale?: ConfigProviderProps["locale"];
  /**
   * 透传给根 ConfigProvider 的其余全局配置，例如 `button`、`form`、`componentSize`。
   * theme 与 locale 使用上面的同名字段。admin-core 默认 `button.autoInsertSpace: false`，
   * 这里的 `button` 与默认值合并。
   */
  configProvider?: Omit<ConfigProviderProps, "children" | "theme" | "locale">;
  reportError?: (error: unknown, context: { operation: string }) => void;
  developmentLogin?: Readonly<{ username: string; password: string }>;
}

export interface AdminFrontendModule {
  readonly name: string;
  readonly pages?: Readonly<Record<string, LazyExoticComponent<ComponentType>>>;
  readonly icons?: Readonly<Record<string, ReactNode>>;
}

export interface AdminApplication {
  readonly Root: ComponentType;
  dispose(): void;
  resumeFromPageCache(): void;
}

export interface SessionSnapshot {
  readonly epoch: number;
  readonly token: string | null;
}

export interface AdminRuntimeValue {
  readonly transport: Transport;
  readonly auth: { signOut(): Promise<void> };
}
