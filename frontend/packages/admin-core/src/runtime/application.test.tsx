import { act, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Code, ConnectError } from "@connectrpc/connect";
import { createAdminApp } from "./application";
import { createMemoryAuthStorage } from "./storage";
import { useAccess, useAdminRuntime, useAppMessage } from "../index";
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
  createSystemOperations: () => ({
    userOperations: { list: vi.fn() },
  }),
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

describe("createAdminApp", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("rejects missing host fields and production developmentLogin", () => {
    expect(() => createAdminApp({ host: { ...host(), apiBaseUrl: "" }, modules: [] })).toThrow(
      /apiBaseUrl/,
    );
    expect(() =>
      createAdminApp({ host: { ...host(), apiBaseUrl: "localhost:8080" }, modules: [] }),
    ).toThrow(/绝对地址/);
    expect(() =>
      createAdminApp({
        host: {
          ...host(),
          mode: "production",
          developmentLogin: { username: "admin", password: "admin123" },
        },
        modules: [],
      }),
    ).toThrow(/developmentLogin/);
  });

  it("throws when hooks run outside Root", () => {
    function Probe() {
      useAdminRuntime();
      return null;
    }
    expect(() => render(<Probe />)).toThrow(/createAdminApp Root/);
    function MessageProbe() {
      useAppMessage();
      return null;
    }
    expect(() => render(<MessageProbe />)).toThrow(/createAdminApp Root/);
    function AccessProbe() {
      useAccess();
      return null;
    }
    expect(() => render(<AccessProbe />)).toThrow(/useAccess/);
  });

  it("throws when Root is rendered without a Router", () => {
    const app = createAdminApp({ host: host(), modules: [] });
    expect(() => render(<app.Root />)).toThrow(/Router/);
    app.dispose();
  });

  it("throws after dispose and keeps dispose idempotent", () => {
    const app = createAdminApp({ host: host(), modules: [] });
    app.dispose();
    app.dispose();
    expect(() =>
      render(
        <MemoryRouter>
          <app.Root />
        </MemoryRouter>,
      ),
    ).toThrow(/已释放/);
  });

  it("isolates two applications by storage, title and token", async () => {
    auth.getUserInfo.mockResolvedValue({
      permissions: ["*:*:*"],
      roles: ["admin"],
      menus: [],
      user: { nickname: "A" },
      pwdResetRequired: false,
    });
    const first = createAdminApp({
      host: host({ title: "App One", authStorage: createMemoryAuthStorage("token-one") }),
      modules: [],
    });
    const second = createAdminApp({
      host: host({ title: "App Two", authStorage: createMemoryAuthStorage() }),
      modules: [],
    });
    const view = render(
      <MemoryRouter>
        <first.Root />
      </MemoryRouter>,
    );
    await screen.findByText("App One");
    view.unmount();
    render(
      <MemoryRouter>
        <second.Root />
      </MemoryRouter>,
    );
    await screen.findByText("App Two");
    first.dispose();
    second.dispose();
  });

  it("keeps a new login when an old request returns 401", async () => {
    const storage = createMemoryAuthStorage();
    let releaseOld: ((error: unknown) => void) | undefined;
    auth.login.mockImplementation(
      () =>
        new Promise((_, reject) => {
          releaseOld = reject;
        }),
    );
    const app = createAdminApp({ host: host({ authStorage: storage }), modules: [] });
    render(
      <MemoryRouter>
        <app.Root />
      </MemoryRouter>,
    );
    await screen.findByPlaceholderText("用户名");
    await act(async () => {
      storage.setToken("new-token");
    });
    auth.getUserInfo.mockResolvedValue({
      permissions: ["*:*:*"],
      roles: [],
      menus: [],
      user: { nickname: "新用户" },
      pwdResetRequired: false,
    });
    await act(async () => {
      releaseOld?.(new ConnectError("expired", Code.Unauthenticated));
    });
    expect(storage.getToken()).toBe("new-token");
    app.dispose();
  });

  it("ignores a stale GetUserInfo result after a newer login", async () => {
    const storage = createMemoryAuthStorage("old-token");
    let resolveOld: ((value: unknown) => void) | undefined;
    auth.getUserInfo.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveOld = resolve;
        }),
    );
    const app = createAdminApp({ host: host({ authStorage: storage }), modules: [] });
    render(
      <MemoryRouter initialEntries={["/"]}>
        <app.Root />
      </MemoryRouter>,
    );
    await act(async () => {
      storage.setToken("new-token");
    });
    auth.getUserInfo.mockResolvedValue({
      permissions: ["system:user:list"],
      roles: ["user"],
      menus: [],
      user: { nickname: "新用户" },
      pwdResetRequired: false,
    });
    await screen.findByText("新用户");
    await act(async () => {
      resolveOld?.({
        permissions: ["*:*:*"],
        roles: ["admin"],
        menus: [{ menuName: "旧菜单", path: "/old", component: "system/user/index" }],
        user: { nickname: "旧用户" },
        pwdResetRequired: false,
      });
    });
    expect(screen.getByText("新用户")).toBeInTheDocument();
    expect(screen.queryByText("旧用户")).not.toBeInTheDocument();
    app.dispose();
  });

  it("does not restore login state from a request that finished after logout", async () => {
    const storage = createMemoryAuthStorage("token");
    let resolveInfo: ((value: unknown) => void) | undefined;
    auth.getUserInfo.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveInfo = resolve;
        }),
    );
    const app = createAdminApp({ host: host({ authStorage: storage }), modules: [] });
    render(
      <MemoryRouter initialEntries={["/"]}>
        <app.Root />
      </MemoryRouter>,
    );
    await act(async () => {
      storage.clearToken();
    });
    await screen.findByPlaceholderText("用户名");
    await act(async () => {
      resolveInfo?.({
        permissions: ["*:*:*"],
        roles: ["admin"],
        menus: [],
        user: { nickname: "不应出现" },
        pwdResetRequired: false,
      });
    });
    expect(screen.queryByText("不应出现")).not.toBeInTheDocument();
    expect(storage.getToken()).toBeNull();
    app.dispose();
  });

  it("keeps the token and shows retry when GetUserInfo fails with a network error", async () => {
    const storage = createMemoryAuthStorage("token");
    auth.getUserInfo.mockRejectedValueOnce(new ConnectError("unavailable", Code.Unavailable));
    const app = createAdminApp({ host: host({ authStorage: storage }), modules: [] });
    render(
      <MemoryRouter initialEntries={["/"]}>
        <app.Root />
      </MemoryRouter>,
    );
    await screen.findByText("加载用户信息失败");
    expect(storage.getToken()).toBe("token");
    auth.getUserInfo.mockResolvedValueOnce({
      permissions: ["*:*:*"],
      roles: [],
      menus: [],
      user: { nickname: "恢复" },
      pwdResetRequired: false,
    });
    await act(async () => {
      screen.getByRole("button", { name: "重试" }).click();
    });
    await waitFor(() => expect(storage.getToken()).toBe("token"));
    app.dispose();
  });
});
