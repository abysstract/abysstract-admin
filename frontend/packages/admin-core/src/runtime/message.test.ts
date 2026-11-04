import { Code, ConnectError } from "@connectrpc/connect";
import { describe, expect, it, vi } from "vitest";
import { formatErrorMessage } from "./message";

describe("formatErrorMessage", () => {
  it("maps technical codes to user-facing messages and logs the original error", () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});
    const error = new ConnectError("connection refused 10.0.0.1:5432", Code.Internal);
    expect(formatErrorMessage(error)).toBe("服务暂时不可用，请稍后再试");
    expect(consoleError).toHaveBeenCalledWith("[admin-core] 请求失败", error);
    consoleError.mockRestore();
  });

  it("maps auth and permission codes", () => {
    expect(formatErrorMessage(new ConnectError("未认证", Code.Unauthenticated))).toBe(
      "登录已过期，请重新登录",
    );
    expect(formatErrorMessage(new ConnectError("denied", Code.PermissionDenied))).toBe(
      "没有执行此操作的权限",
    );
  });

  it("keeps server messages for business codes", () => {
    expect(
      formatErrorMessage(
        new ConnectError("不能停用当前登录用户的唯一有效角色", Code.FailedPrecondition),
      ),
    ).toBe("不能停用当前登录用户的唯一有效角色");
  });

  it("falls back for empty and non-Connect errors", () => {
    expect(formatErrorMessage(new ConnectError("", Code.FailedPrecondition))).toBe("操作失败");
    expect(formatErrorMessage(new Error("boom"))).toBe("boom");
    expect(formatErrorMessage("自定义消息")).toBe("自定义消息");
    expect(formatErrorMessage(undefined)).toBe("操作失败");
  });
});
