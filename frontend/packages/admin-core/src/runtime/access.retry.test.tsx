import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createAdminApp } from "./application";
import { createMemoryAuthStorage } from "./storage";
import type { AdminHost } from "./types";

const auth = vi.hoisted(() => ({
  login: vi.fn(),
  logout: vi.fn(),
  getUserInfo: vi.fn(),
  changePassword: vi.fn(),
}));

vi.mock("../system/clients", () => ({
  createSystemClients: () => ({
    auth,
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

function host(overrides: Partial<AdminHost> = {}): AdminHost {
  return {
    apiBaseUrl: "http://localhost:8080",
    mode: "development",
    authStorage: createMemoryAuthStorage(),
    title: "Abysstract 后台管理",
    ...overrides,
  };
}

describe("AccessProvider 菜单加载失败", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("shows a retry action and loads menus after a later success", async () => {
    const storage = createMemoryAuthStorage("retry");
    storage.setToken("token-a");
    auth.getUserInfo.mockRejectedValueOnce(new Error("网络中断")).mockResolvedValueOnce({
      permissions: [],
      roles: [],
      menus: [],
      user: { nickname: "重试用户" },
      pwdResetRequired: false,
    });
    const app = createAdminApp({ host: host({ authStorage: storage }), modules: [] });
    render(
      <MemoryRouter initialEntries={["/"]}>
        <app.Root />
      </MemoryRouter>,
    );
    expect(await screen.findByText("加载用户信息失败")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "重试" }));
    expect(await screen.findByText("无可用菜单")).toBeInTheDocument();
    expect(auth.getUserInfo).toHaveBeenCalledTimes(2);
    app.dispose();
  });
});
