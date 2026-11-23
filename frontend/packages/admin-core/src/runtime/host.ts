import type { AdminHost } from "./types";

const invalidBaseMessage = "host.apiBaseUrl 必须是 http(s) 绝对地址或以 / 开头的同源路径";

/**
 * 规范化 API 基础地址，去掉末尾的 `/`。
 *
 * - 绝对地址：只接受 http(s)，例如 `https://admin-api.example.com`。
 * - 同源路径：以单个 `/` 开头，例如 `/` 或 `/api`，按页面地址（`window.location`）解析为绝对地址，
 *   同一份构建产物可以部署在任何域名下，由反向代理把请求转发给后端。
 */
export function normalizeApiBaseUrl(value: string, pageUrl?: string): string {
  const trimmed = value.trim();
  let parsed: URL;
  try {
    if (trimmed.startsWith("/")) {
      // `//host` 是协议相对地址，会指向别的域名，不当作同源路径接受。
      if (trimmed.startsWith("//")) throw new Error(invalidBaseMessage);
      const base = pageUrl ?? (typeof window !== "undefined" ? window.location.href : undefined);
      if (base == null) throw new Error(invalidBaseMessage);
      parsed = new URL(trimmed, base);
      parsed.search = "";
      parsed.hash = "";
    } else {
      parsed = new URL(trimmed);
    }
  } catch {
    throw new Error(invalidBaseMessage);
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new Error(invalidBaseMessage);
  }
  const normalized = parsed.toString();
  return normalized.endsWith("/") ? normalized.slice(0, -1) : normalized;
}

export function validateHost(host: AdminHost): AdminHost {
  if (host == null || typeof host !== "object") {
    throw new Error("host 必填");
  }
  if (typeof host.apiBaseUrl !== "string" || host.apiBaseUrl.trim() === "") {
    throw new Error("host.apiBaseUrl 必填");
  }
  if (host.mode !== "development" && host.mode !== "production") {
    throw new Error("host.mode 必须是 development 或 production");
  }
  if (host.authStorage == null || typeof host.authStorage !== "object") {
    throw new Error("host.authStorage 必填");
  }
  if (typeof host.title !== "string" || host.title.trim() === "") {
    throw new Error("host.title 必填");
  }
  if (host.mode === "production" && host.developmentLogin) {
    throw new Error("production mode 不能传入 developmentLogin");
  }
  return {
    ...host,
    apiBaseUrl: normalizeApiBaseUrl(host.apiBaseUrl),
  };
}
