import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, it, expect, vi } from "vitest";
import { createAdminApp } from "../runtime/application";
import { createMemoryAuthStorage } from "../runtime/storage";

vi.mock("../system/clients", () => ({
  createSystemClients: () => ({
    auth: { login: vi.fn(), logout: vi.fn(), getUserInfo: vi.fn(), changePassword: vi.fn() },
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

describe("Login 页面冒烟测试", () => {
  it("应渲染标志性元素", () => {
    const app = createAdminApp({
      host: {
        apiBaseUrl: "http://localhost:8080",
        mode: "development",
        authStorage: createMemoryAuthStorage(),
        title: "Abysstract 后台管理",
      },
      modules: [],
    });
    render(
      <MemoryRouter>
        <app.Root />
      </MemoryRouter>,
    );

    expect(screen.getByText("Abysstract 后台管理")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("用户名")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("密码")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /登录/ })).toBeInTheDocument();
    app.dispose();
  });
});
