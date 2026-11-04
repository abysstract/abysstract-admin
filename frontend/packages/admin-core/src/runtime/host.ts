import type { AdminHost } from "./types";

export function normalizeApiBaseUrl(value: string): string {
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    throw new Error("host.apiBaseUrl 必须是 http(s) 绝对地址");
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new Error("host.apiBaseUrl 必须是 http(s) 绝对地址");
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
