import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Code, ConnectError } from "@connectrpc/connect";
import { afterEach, describe, it, expect, vi } from "vitest";
import { createAdminApp } from "../runtime/application";
import { createMemoryAuthStorage } from "../runtime/storage";
import type { AdminHost, AdminLoginCaptchaProps } from "../runtime/types";
import {
  CaptchaError,
  LoginChallengeSchema,
} from "@abysstract/admin-proto/abysstract/admin/v1/auth_pb";

// 共享 setup 把 antd 换成了简化版，这里用真实的 Form 才能验证提交和字段错误。
vi.unmock("antd");

const login = vi.hoisted(() => vi.fn());
const getCaptcha = vi.hoisted(() => vi.fn());

vi.mock("../system/clients", () => ({
  createSystemClients: () => ({
    auth: { login, getCaptcha, logout: vi.fn(), getUserInfo: vi.fn(), changePassword: vi.fn() },
    dict: {},
    config: {},
    notice: {},
    log: {},
    upload: {},
    commonExport: {},
    profile: {},
    dashboard: {},
  }),
}));

vi.mock("../system/operations", () => ({
  createSystemOperations: () => ({}),
}));

function renderLogin(host: Partial<AdminHost> = {}) {
  const app = createAdminApp({
    host: {
      apiBaseUrl: "http://localhost:8080",
      mode: "development",
      authStorage: createMemoryAuthStorage(),
      title: "示例后台",
      ...host,
    },
    modules: [],
  });
  render(
    <MemoryRouter initialEntries={["/login"]}>
      <app.Root />
    </MemoryRouter>,
  );
  return app;
}

async function submit(username: string, password: string) {
  fireEvent.change(screen.getByLabelText("用户名"), { target: { value: username } });
  fireEvent.change(screen.getByLabelText("密码"), { target: { value: password } });
  fireEvent.click(screen.getByRole("button", { name: "登录" }));
}

afterEach(() => {
  login.mockReset();
  getCaptcha.mockReset();
});

function challengeError(
  message: string,
  detail: { captchaRequired?: boolean; captchaError?: CaptchaError },
) {
  return new ConnectError(message, Code.Unauthenticated, undefined, [
    { desc: LoginChallengeSchema, value: detail },
  ]);
}

const slideChallenge = {
  kind: "slide",
  id: "challenge-1",
  expiresIn: 120n,
  payload: JSON.stringify({
    image: "data:image/jpeg;base64,",
    thumb: "data:image/png;base64,",
    thumbX: 0,
    thumbY: 40,
    thumbWidth: 60,
    thumbHeight: 60,
    width: 300,
    height: 220,
  }),
};

// 用键盘完成内置滑块：右移 n 次后回车。
async function solveSlide(steps: number) {
  const slider = await screen.findByRole("slider");
  await waitFor(() => expect(slider).toHaveAttribute("tabindex", "0"));
  for (let i = 0; i < steps; i++) fireEvent.keyDown(slider, { key: "ArrowRight" });
  fireEvent.keyDown(slider, { key: "Enter" });
}

describe("Login 页面", () => {
  it("显示产品名和带标签的用户名、密码", () => {
    const app = renderLogin();
    expect(screen.getByText("示例后台")).toBeInTheDocument();
    expect(screen.getByLabelText("用户名")).toBeInTheDocument();
    expect(screen.getByLabelText("密码")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "登录" })).toBeInTheDocument();
    app.dispose();
  });

  it("显示宿主配置的 logo、副标题和页脚", () => {
    const app = renderLogin({
      login: {
        logo: <img alt="示例 logo" src="data:," />,
        subtitle: "示例副标题",
        footer: "示例页脚",
      },
    });
    expect(screen.getByAltText("示例 logo")).toBeInTheDocument();
    expect(screen.getByText("示例副标题")).toBeInTheDocument();
    expect(screen.getByText("示例页脚")).toBeInTheDocument();
    app.dispose();
  });

  it("用户名或密码不对时，错误标在密码下方", async () => {
    login.mockRejectedValue(new ConnectError("用户名或密码错误", Code.Unauthenticated));
    const app = renderLogin();
    await submit("admin", "wrong");
    expect(await screen.findByText("用户名或密码不正确")).toBeInTheDocument();
    expect(screen.queryByText("登录已过期，请重新登录")).not.toBeInTheDocument();
    app.dispose();
  });

  it("账号锁定时显示服务端给出的原因", async () => {
    login.mockRejectedValue(
      new ConnectError("账号已锁定，请 15 分钟后再试", Code.PermissionDenied),
    );
    const app = renderLogin();
    await submit("admin", "admin123");
    expect(await screen.findByText("账号已锁定，请 15 分钟后再试")).toBeInTheDocument();
    app.dispose();
  });

  it("服务端要求验证码后显示内置滑块，登录时带上作答", async () => {
    login.mockRejectedValueOnce(challengeError("用户名或密码错误", { captchaRequired: true }));
    getCaptcha.mockResolvedValue(slideChallenge);
    const app = renderLogin();
    await submit("admin", "wrong");
    expect(await screen.findByText("用户名或密码不正确")).toBeInTheDocument();
    expect(await screen.findByText("安全验证")).toBeInTheDocument();
    expect(getCaptcha).toHaveBeenCalledTimes(1);

    // 没完成验证就提交：标在验证码下方，不发请求。
    login.mockResolvedValue({ token: "t" });
    fireEvent.click(screen.getByRole("button", { name: "登录" }));
    expect(await screen.findByText("请完成安全验证")).toBeInTheDocument();
    expect(login).toHaveBeenCalledTimes(1);

    // 完成后自动重新提交。
    await solveSlide(5);
    await waitFor(() => expect(login).toHaveBeenCalledTimes(2));
    expect(login.mock.calls[1][0]).toMatchObject({
      username: "admin",
      captchaId: "challenge-1",
      captcha: JSON.stringify({ x: 10 }),
    });
    app.dispose();
  });

  it("点登录后才要求验证码时，完成验证即自动提交", async () => {
    login.mockRejectedValueOnce(
      challengeError("请完成人机验证", {
        captchaRequired: true,
        captchaError: CaptchaError.MISSING,
      }),
    );
    login.mockResolvedValueOnce({ token: "t" });
    getCaptcha.mockResolvedValue(slideChallenge);
    const app = renderLogin();
    await submit("admin", "admin123");
    await solveSlide(1);
    await waitFor(() => expect(login).toHaveBeenCalledTimes(2));
    expect(screen.queryByText("用户名或密码不正确")).not.toBeInTheDocument();
    app.dispose();
  });

  it("验证未通过时提示并换一张", async () => {
    login.mockRejectedValueOnce(
      challengeError("请完成人机验证", {
        captchaRequired: true,
        captchaError: CaptchaError.MISSING,
      }),
    );
    login.mockRejectedValueOnce(
      challengeError("人机验证未通过", {
        captchaRequired: true,
        captchaError: CaptchaError.INVALID,
      }),
    );
    getCaptcha.mockResolvedValue(slideChallenge);
    const app = renderLogin();
    await submit("admin", "admin123");
    await solveSlide(3);
    expect(await screen.findByText("验证未通过，请重试")).toBeInTheDocument();
    await waitFor(() => expect(getCaptcha).toHaveBeenCalledTimes(2));
    app.dispose();
  });

  it("使用宿主提供的验证码组件", async () => {
    function HostCaptcha({ onChange }: AdminLoginCaptchaProps) {
      return (
        <button type="button" onClick={() => onChange({ token: "host-token" })}>
          宿主验证
        </button>
      );
    }
    login.mockRejectedValueOnce(
      challengeError("请完成人机验证", {
        captchaRequired: true,
        captchaError: CaptchaError.MISSING,
      }),
    );
    login.mockResolvedValueOnce({ token: "t" });
    const app = renderLogin({ login: { captcha: HostCaptcha } });
    await submit("admin", "admin123");
    fireEvent.click(await screen.findByRole("button", { name: "宿主验证" }));
    await waitFor(() => expect(login).toHaveBeenCalledTimes(2));
    expect(login.mock.calls[1][0]).toMatchObject({ captchaId: "", captcha: "host-token" });
    expect(getCaptcha).not.toHaveBeenCalled();
    app.dispose();
  });

  it("来源 IP 尝试过多时显示服务端给出的原因", async () => {
    login.mockRejectedValue(
      new ConnectError("尝试次数过多，请 15 分钟后再试", Code.ResourceExhausted),
    );
    const app = renderLogin();
    await submit("admin", "admin123");
    expect(await screen.findByText("尝试次数过多，请 15 分钟后再试")).toBeInTheDocument();
    app.dispose();
  });

  it("文案可以由宿主覆盖", () => {
    const app = renderLogin({ texts: { login: { submit: "进入", username: "账号" } } });
    expect(screen.getByRole("button", { name: "进入" })).toBeInTheDocument();
    expect(screen.getByLabelText("账号")).toBeInTheDocument();
    app.dispose();
  });
});
