import { Code, ConnectError } from "@connectrpc/connect";
import { App as AntdApp } from "antd";
import type { MessageInstance } from "antd/es/message/interface";
import { useAdminRuntimeState } from "./context";

// 技术性错误映射为用户可理解的文案，原始错误通过 console.error 保留给开发者；
// 业务校验类错误（FailedPrecondition / InvalidArgument 等）的服务器文案本身面向用户，原样保留。
const codeMessages: Partial<Record<Code, string>> = {
  [Code.Unauthenticated]: "登录已过期，请重新登录",
  [Code.PermissionDenied]: "没有执行此操作的权限",
  [Code.Internal]: "服务暂时不可用，请稍后再试",
  [Code.Unavailable]: "服务暂时不可用，请稍后再试",
  [Code.DataLoss]: "服务暂时不可用，请稍后再试",
  [Code.Unknown]: "服务暂时不可用，请稍后再试",
  [Code.DeadlineExceeded]: "请求超时，请稍后再试",
  [Code.Canceled]: "请求已取消",
};

export function formatErrorMessage(error: unknown, fallback = "操作失败"): string {
  if (error instanceof ConnectError) {
    const mapped = codeMessages[error.code];
    if (mapped !== undefined) {
      console.error("[admin-core] 请求失败", error);
      return mapped;
    }
    return error.rawMessage || fallback;
  }
  if (error instanceof Error) {
    return error.message || fallback;
  }
  if (typeof error === "string" && error.length > 0) {
    return error;
  }
  return fallback;
}

export function useAppMessage(): MessageInstance {
  useAdminRuntimeState();
  return AntdApp.useApp().message;
}
