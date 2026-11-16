import type { ConfigProviderProps, ThemeConfig } from "antd";
import type { Transport } from "@connectrpc/connect";
import type { LazyExoticComponent, ComponentType, ReactNode } from "react";
import type { AdminTextsOverride } from "./texts";

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
  /** 覆盖框架界面文案（按钮、提示、空状态等）的一部分，未覆盖的沿用默认值。 */
  texts?: AdminTextsOverride;
  /** 登录页上可替换的部分；不传时只显示 `title`，背景用跟随主题的默认色块。 */
  login?: AdminLoginConfig;
  reportError?: (error: unknown, context: { operation: string }) => void;
  developmentLogin?: Readonly<{ username: string; password: string }>;
}

export interface AdminLoginConfig {
  /** 显示在产品名左侧，建议 32×32 的图片或 SVG。 */
  logo?: ReactNode;
  /** 产品名下方的一句话，次要色显示。 */
  subtitle?: ReactNode;
  /**
   * 登录页背景，CSS `background` 的值，例如 `"url(/login.jpg) center / cover"`。
   * 宽屏用于左侧品牌区，窄屏铺满整页。不传时用由主题 token 生成的浅色色块，暗色主题下同样适用。
   */
  background?: string;
  /** 页面底部的一行，例如版权或备案信息，次要色小字显示。 */
  footer?: ReactNode;
  /**
   * 服务端要求验证码时显示的组件。不传时使用内置组件，支持服务端的内置滑块
   * （`ADMIN_CAPTCHA_PROVIDER=slide`）。接 Turnstile、hCaptcha 等第三方服务时，
   * 在这里渲染对方的组件，把得到的 token 交给 `onChange`。
   */
  captcha?: ComponentType<AdminLoginCaptchaProps>;
}

/** 交给登录请求的验证码结果。 */
export interface AdminLoginCaptchaValue {
  /** GetCaptcha 返回的挑战 ID；第三方验证不需要。 */
  readonly id?: string;
  /** 第三方组件给出的 token，或对挑战的作答。 */
  readonly token: string;
}

/** 一次验证码挑战，即 GetCaptcha 的结果。 */
export interface AdminLoginCaptchaChallenge {
  /** 渲染组件：slide、turnstile、hcaptcha 或应用自定义。 */
  readonly kind: string;
  /** 交给组件的 JSON，第三方验证里有 `siteKey`。 */
  readonly payload: string;
  readonly id: string;
}

export interface AdminLoginCaptchaProps {
  /** 完成验证时传入结果；结果失效（如换一张）时传入 undefined。 */
  onChange(value: AdminLoginCaptchaValue | undefined): void;
  /** 每次登录失败后加 1。验证结果只能用一次，组件据此重置。 */
  readonly resetKey: number;
  /** 向服务端取一次挑战（GetCaptcha）。 */
  loadChallenge(): Promise<AdminLoginCaptchaChallenge>;
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
