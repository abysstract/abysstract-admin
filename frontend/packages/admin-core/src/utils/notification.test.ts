import { Code, ConnectError } from "@connectrpc/connect";
import { describe, expect, it } from "vitest";
import { formatErrorMessage } from "./notification";

describe("formatErrorMessage", () => {
  it("uses ConnectError rawMessage and hides the protocol code", () => {
    const error = new ConnectError("不能停用当前登录用户的唯一有效角色", Code.FailedPrecondition);

    expect(error.message).toContain("failed_precondition");
    expect(formatErrorMessage(error)).toBe("不能停用当前登录用户的唯一有效角色");
  });

  it("keeps ordinary Error text and applies a fallback for unknown values", () => {
    expect(formatErrorMessage(new Error("业务失败"))).toBe("业务失败");
    expect(formatErrorMessage(undefined, "加载失败")).toBe("加载失败");
    expect(formatErrorMessage("请求失败")).toBe("请求失败");
  });
});
