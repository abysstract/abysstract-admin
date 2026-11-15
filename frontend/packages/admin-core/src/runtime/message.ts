import { Code, ConnectError } from "@connectrpc/connect";
import { App as AntdApp, Modal, message as staticMessage, type ModalFuncProps } from "antd";
import type { MessageInstance } from "antd/es/message/interface";
import { useAdminRuntimeState } from "./context";

// 技术性错误映射为用户可理解的文案，原始错误通过 console.error 保留给开发者；
// 业务校验类错误（FailedPrecondition / InvalidArgument 等）的服务器文案本身面向用户，原样保留。
const codeMessages: Partial<Record<Code, string>> = {
  [Code.Unauthenticated]: "登录已过期，请重新登录",
  [Code.PermissionDenied]: "没有执行这个操作的权限",
  [Code.Internal]: "服务暂时不可用，请稍后再试",
  [Code.Unavailable]: "服务暂时不可用，请稍后再试",
  [Code.DataLoss]: "服务暂时不可用，请稍后再试",
  [Code.Unknown]: "服务暂时不可用，请稍后再试",
  [Code.DeadlineExceeded]: "请求超时，请稍后再试",
  [Code.Canceled]: "请求已取消",
};

export function formatErrorMessage(error: unknown, fallback = "无法完成操作，请稍后重试"): string {
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

export interface AppFeedback {
  readonly message: MessageInstance;
  readonly modal: {
    readonly confirm: (config: ModalFuncProps) => unknown;
    readonly error: (config: ModalFuncProps) => unknown;
  };
}

/**
 * antd App 提供的 message 与 modal；在 App 之外渲染时（例如单独使用公共组件），
 * App.useApp() 返回空对象，这里回退到静态方法。
 */
export function useAppFeedback(): AppFeedback {
  const app = AntdApp.useApp();
  return {
    message: typeof app.message?.success === "function" ? app.message : staticMessage,
    modal: typeof app.modal?.error === "function" ? app.modal : Modal,
  };
}
