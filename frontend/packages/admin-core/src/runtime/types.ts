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
