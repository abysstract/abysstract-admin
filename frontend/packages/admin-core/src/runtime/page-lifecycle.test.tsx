import { act, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { createAdminApp } from "./application";
import { bindAdminAppPageLifecycle } from "./page-lifecycle";
import { createLocalAuthStorage, createMemoryAuthStorage } from "./storage";
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

function persistedEvent(type: "pagehide" | "pageshow", persisted: boolean): Event {
  const event = new Event(type);
  Object.defineProperty(event, "persisted", { value: persisted });
  return event;
}

describe("bindAdminAppPageLifecycle", () => {
  afterEach(() => {
    vi.clearAllMocks();
    window.localStorage.clear();
  });

  it("keeps the mounted tree on persisted pagehide and releases on permanent hide", async () => {
    auth.getUserInfo.mockResolvedValue({
      permissions: ["*:*:*"],
      roles: ["admin"],
      menus: [],
      user: { nickname: "缓存用户" },
      pwdResetRequired: false,
    });
    const app = createAdminApp({
      host: host({ authStorage: createMemoryAuthStorage("token") }),
      modules: [],
    });
    const view = render(
      <MemoryRouter initialEntries={["/"]}>
        <app.Root />
      </MemoryRouter>,
    );
    const root = { unmount: vi.fn(() => view.unmount()) };
    const stop = bindAdminAppPageLifecycle({ application: app, root, target: window });
    await screen.findByText("缓存用户");
    window.dispatchEvent(persistedEvent("pagehide", true));
    expect(root.unmount).not.toHaveBeenCalled();
    expect(screen.getByText("缓存用户")).toBeInTheDocument();
    window.dispatchEvent(persistedEvent("pagehide", false));
    expect(root.unmount).toHaveBeenCalledTimes(1);
    expect(() =>
      render(
        <MemoryRouter>
          <app.Root />
        </MemoryRouter>,
      ),
    ).toThrow(/已释放/);
    stop();
  });

  it("resyncs login state on persisted pageshow after another tab switches users", async () => {
    const key = "admin-core-bfcache-token";
    window.localStorage.setItem(key, "token-a");
    const storage = createLocalAuthStorage(key);
    auth.getUserInfo.mockImplementation(async () => {
      const token = storage.getToken();
      if (token === "token-a") {
        return {
          permissions: ["*:*:*"],
          roles: ["admin"],
          menus: [{ menuName: "旧菜单", path: "/old", component: "system/user/index" }],
          user: { nickname: "用户A" },
          pwdResetRequired: false,
        };
      }
      if (token === "token-b") {
        return {
          permissions: ["system:user:list"],
          roles: ["user"],
          menus: [{ menuName: "新菜单", path: "/new", component: "system/role/index" }],
          user: { nickname: "用户B" },
          pwdResetRequired: false,
        };
      }
      throw new Error("missing token");
    });
    const app = createAdminApp({ host: host({ authStorage: storage }), modules: [] });
    const view = render(
      <MemoryRouter initialEntries={["/"]}>
        <app.Root />
      </MemoryRouter>,
    );
    const root = { unmount: vi.fn(() => view.unmount()) };
    const stop = bindAdminAppPageLifecycle({ application: app, root, target: window });
    await screen.findByText("用户A");
    window.dispatchEvent(persistedEvent("pagehide", true));
    expect(root.unmount).not.toHaveBeenCalled();
    window.localStorage.setItem(key, "token-b");
    await act(async () => {
      window.dispatchEvent(persistedEvent("pageshow", true));
    });
    await screen.findByText("用户B");
    expect(screen.queryByText("用户A")).not.toBeInTheDocument();
    expect(root.unmount).not.toHaveBeenCalled();
    stop();
    app.dispose();
  });

  it("ignores an old GetUserInfo result after a cached page resumes with a new session", async () => {
    const key = "admin-core-bfcache-stale-token";
    window.localStorage.setItem(key, "token-a");
    const storage = createLocalAuthStorage(key);
    let resolveOld: ((value: unknown) => void) | undefined;
    auth.getUserInfo.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveOld = resolve;
        }),
    );
    const app = createAdminApp({ host: host({ authStorage: storage }), modules: [] });
    const view = render(
      <MemoryRouter initialEntries={["/"]}>
        <app.Root />
      </MemoryRouter>,
    );
    const root = { unmount: vi.fn(() => view.unmount()) };
    const stop = bindAdminAppPageLifecycle({ application: app, root, target: window });
    window.dispatchEvent(persistedEvent("pagehide", true));
    window.localStorage.setItem(key, "token-b");
    auth.getUserInfo.mockResolvedValue({
      permissions: ["system:user:list"],
      roles: ["user"],
      menus: [],
      user: { nickname: "用户B" },
      pwdResetRequired: false,
    });
    await act(async () => {
      window.dispatchEvent(persistedEvent("pageshow", true));
    });
    await screen.findByText("用户B");
    await act(async () => {
      resolveOld?.({
        permissions: ["*:*:*"],
        roles: ["admin"],
        menus: [{ menuName: "旧菜单", path: "/old", component: "system/user/index" }],
        user: { nickname: "用户A" },
        pwdResetRequired: false,
      });
    });
    expect(screen.getByText("用户B")).toBeInTheDocument();
    expect(screen.queryByText("用户A")).not.toBeInTheDocument();
    expect(root.unmount).not.toHaveBeenCalled();
    stop();
    app.dispose();
  });

  it("shows login after a cached page resumes onto a logout written by another tab", async () => {
    const key = "admin-core-bfcache-logout-token";
    window.localStorage.setItem(key, "token-a");
    const storage = createLocalAuthStorage(key);
    auth.getUserInfo.mockResolvedValue({
      permissions: ["*:*:*"],
      roles: ["admin"],
      menus: [],
      user: { nickname: "用户A" },
      pwdResetRequired: false,
    });
    const app = createAdminApp({ host: host({ authStorage: storage }), modules: [] });
    const view = render(
      <MemoryRouter initialEntries={["/"]}>
        <app.Root />
      </MemoryRouter>,
    );
    const root = { unmount: vi.fn(() => view.unmount()) };
    const stop = bindAdminAppPageLifecycle({ application: app, root, target: window });
    await screen.findByText("用户A");
    window.dispatchEvent(persistedEvent("pagehide", true));
    window.localStorage.removeItem(key);
    await act(async () => {
      window.dispatchEvent(persistedEvent("pageshow", true));
    });
    await waitFor(() => {
      expect(screen.getByPlaceholderText("用户名")).toBeInTheDocument();
    });
    expect(screen.queryByText("用户A")).not.toBeInTheDocument();
    stop();
    app.dispose();
  });
});
