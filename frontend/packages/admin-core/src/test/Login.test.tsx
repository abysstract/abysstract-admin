import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { Code, ConnectError } from "@connectrpc/connect";
import { afterEach, describe, it, expect, vi } from "vitest";
import { createAdminApp } from "../runtime/application";
import { createMemoryAuthStorage } from "../runtime/storage";
import type { AdminHost } from "../runtime/types";

// 共享 setup 把 antd 换成了简化版，这里用真实的 Form 才能验证提交和字段错误。
vi.unmock("antd");

const login = vi.hoisted(() => vi.fn());

vi.mock("../system/clients", () => ({
  createSystemClients: () => ({
    auth: { login, logout: vi.fn(), getUserInfo: vi.fn(), changePassword: vi.fn() },
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
});

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

  it("文案可以由宿主覆盖", () => {
    const app = renderLogin({ texts: { login: { submit: "进入", username: "账号" } } });
    expect(screen.getByRole("button", { name: "进入" })).toBeInTheDocument();
    expect(screen.getByLabelText("账号")).toBeInTheDocument();
    app.dispose();
  });
});
