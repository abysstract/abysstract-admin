import { create } from "@bufbuild/protobuf";
import { act, render, screen } from "@testing-library/react";
import { lazy } from "react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MenuSchema, type Menu } from "@abysstract/admin-proto/abysstract/admin/v1/menu_pb";
import { createAdminApp } from "./runtime/application";
import { createMemoryAuthStorage } from "./runtime/storage";
import type { AdminHost } from "./runtime/types";

const auth = vi.hoisted(() => ({
  login: vi.fn(),
  logout: vi.fn(),
  getUserInfo: vi.fn(),
  changePassword: vi.fn(),
}));

vi.mock("./system/clients", () => ({
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

vi.mock("./system/operations", () => ({
  createSystemOperations: () => ({}),
}));

const PageKeep = lazy(() => Promise.resolve({ default: () => <div>keep-page</div> }));
const PageAlt = lazy(() => Promise.resolve({ default: () => <div>alt-page</div> }));
const PageOther = lazy(() => Promise.resolve({ default: () => <div>other-page</div> }));

function host(overrides: Partial<AdminHost> = {}): AdminHost {
  return {
    apiBaseUrl: "http://localhost:8080",
    mode: "development",
    authStorage: createMemoryAuthStorage("token"),
    title: "Abysstract 后台管理",
    ...overrides,
  };
}

function menu(partial: {
  menuName: string;
  menuType: string;
  path?: string;
  component?: string;
  children?: Menu[];
}): Menu {
  return create(MenuSchema, partial);
}

describe("AuthedApp 路由", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("shows NoAccess for an empty menu without redirecting to login", async () => {
    const storage = createMemoryAuthStorage("empty");
    storage.setToken("token-a");
    auth.getUserInfo.mockResolvedValue({
      permissions: [],
      roles: [],
      menus: [],
      user: { nickname: "空菜单用户" },
      pwdResetRequired: false,
    });
    const app = createAdminApp({
      host: host({ authStorage: storage }),
      modules: [{ name: "system", pages: { "system/user/index": PageKeep } }],
    });
    render(
      <MemoryRouter initialEntries={["/"]}>
        <app.Root />
      </MemoryRouter>,
    );
    expect(await screen.findByText("无可用菜单")).toBeInTheDocument();
    expect(screen.queryByPlaceholderText("用户名")).not.toBeInTheDocument();
    app.dispose();
  });

  it("renders a registered nested page", async () => {
    const storage = createMemoryAuthStorage("nested");
    storage.setToken("token-a");
    auth.getUserInfo.mockResolvedValue({
      permissions: ["*:*:*"],
      roles: ["admin"],
      menus: [
        menu({
          menuName: "系统管理",
          menuType: "M",
          path: "system",
          children: [
            menu({
              menuName: "用户管理",
              menuType: "C",
              path: "user",
              component: "system/user/index",
            }),
          ],
        }),
      ],
      user: { nickname: "管理员" },
      pwdResetRequired: false,
    });
    const app = createAdminApp({
      host: host({ authStorage: storage }),
      modules: [{ name: "system", pages: { "system/user/index": PageKeep } }],
    });
    render(
      <MemoryRouter initialEntries={["/system/user"]}>
        <app.Root />
      </MemoryRouter>,
    );
    expect(await screen.findByText("keep-page")).toBeInTheDocument();
    app.dispose();
  });

  it("shows a diagnostic for an unregistered component", async () => {
    const storage = createMemoryAuthStorage("missing");
    storage.setToken("token-a");
    auth.getUserInfo.mockResolvedValue({
      permissions: ["*:*:*"],
      roles: ["admin"],
      menus: [
        menu({
          menuName: "缺失页",
          menuType: "C",
          path: "missing",
          component: "missing/page",
        }),
      ],
      user: { nickname: "管理员" },
      pwdResetRequired: false,
    });
    const app = createAdminApp({
      host: host({ authStorage: storage }),
      modules: [{ name: "system", pages: { "system/user/index": PageKeep } }],
    });
    render(
      <MemoryRouter initialEntries={["/missing"]}>
        <app.Root />
      </MemoryRouter>,
    );
    expect(await screen.findByText("页面未注册")).toBeInTheDocument();
    expect(screen.getByText(/component=missing\/page/)).toBeInTheDocument();
    app.dispose();
  });

  it("shows reserved route conflicts", async () => {
    const storage = createMemoryAuthStorage("reserved");
    storage.setToken("token-a");
    auth.getUserInfo.mockResolvedValue({
      permissions: ["*:*:*"],
      roles: ["admin"],
      menus: [
        menu({
          menuName: "伪登录",
          menuType: "C",
          path: "login",
          component: "system/user/index",
        }),
      ],
      user: { nickname: "管理员" },
      pwdResetRequired: false,
    });
    const app = createAdminApp({
      host: host({ authStorage: storage }),
      modules: [{ name: "system", pages: { "system/user/index": PageKeep } }],
    });
    render(
      <MemoryRouter initialEntries={["/"]}>
        <app.Root />
      </MemoryRouter>,
    );
    expect(await screen.findByText("菜单路由冲突")).toBeInTheDocument();
    expect(screen.getByText(/\/login/)).toBeInTheDocument();
    app.dispose();
  });

  it("drops previous user routes after the session epoch changes", async () => {
    const storage = createMemoryAuthStorage("epoch");
    storage.setToken("token-a");
    auth.getUserInfo.mockImplementation(async () => {
      const token = storage.getToken();
      if (token === "token-b") {
        return {
          permissions: ["*:*:*"],
          roles: ["admin"],
          menus: [
            menu({
              menuName: "另一页",
              menuType: "C",
              path: "other",
              component: "other/index",
            }),
          ],
          user: { nickname: "用户B" },
          pwdResetRequired: false,
        };
      }
      return {
        permissions: ["*:*:*"],
        roles: ["admin"],
        menus: [
          menu({
            menuName: "保留页",
            menuType: "C",
            path: "keep",
            component: "keep/index",
          }),
        ],
        user: { nickname: "用户A" },
        pwdResetRequired: false,
      };
    });
    const app = createAdminApp({
      host: host({ authStorage: storage }),
      modules: [
        {
          name: "probe",
          pages: {
            "keep/index": PageKeep,
            "other/index": PageOther,
          },
        },
      ],
    });
    render(
      <MemoryRouter initialEntries={["/keep"]}>
        <app.Root />
      </MemoryRouter>,
    );
    expect(await screen.findByText("keep-page")).toBeInTheDocument();
    act(() => {
      storage.setToken("token-b");
    });
    expect(await screen.findByText("other-page")).toBeInTheDocument();
    expect(screen.queryByText("keep-page")).not.toBeInTheDocument();
    app.dispose();
  });

  it("ignores input map mutations after createAdminApp", async () => {
    const storage = createMemoryAuthStorage("copy");
    storage.setToken("token-a");
    auth.getUserInfo.mockResolvedValue({
      permissions: ["*:*:*"],
      roles: ["admin"],
      menus: [
        menu({
          menuName: "保留页",
          menuType: "C",
          path: "keep",
          component: "keep/index",
        }),
      ],
      user: { nickname: "管理员" },
      pwdResetRequired: false,
    });
    const pages: Record<string, typeof PageKeep> = { "keep/index": PageKeep };
    const app = createAdminApp({
      host: host({ authStorage: storage }),
      modules: [{ name: "probe", pages }],
    });
    pages["keep/index"] = PageAlt;
    pages["alt/index"] = PageAlt;
    render(
      <MemoryRouter initialEntries={["/keep"]}>
        <app.Root />
      </MemoryRouter>,
    );
    expect(await screen.findByText("keep-page")).toBeInTheDocument();
    expect(screen.queryByText("alt-page")).not.toBeInTheDocument();
    app.dispose();
  });

  it("lets two applications resolve the same key to different pages", async () => {
    const firstStorage = createMemoryAuthStorage("first");
    const secondStorage = createMemoryAuthStorage("second");
    firstStorage.setToken("token-a");
    secondStorage.setToken("token-b");
    auth.getUserInfo.mockResolvedValue({
      permissions: ["*:*:*"],
      roles: ["admin"],
      menus: [
        menu({
          menuName: "共享页",
          menuType: "C",
          path: "shared",
          component: "shared/index",
        }),
      ],
      user: { nickname: "管理员" },
      pwdResetRequired: false,
    });
    const first = createAdminApp({
      host: host({ authStorage: firstStorage, title: "One" }),
      modules: [{ name: "one", pages: { "shared/index": PageKeep } }],
    });
    const second = createAdminApp({
      host: host({ authStorage: secondStorage, title: "Two" }),
      modules: [{ name: "two", pages: { "shared/index": PageOther } }],
    });
    const view = render(
      <MemoryRouter initialEntries={["/shared"]}>
        <first.Root />
      </MemoryRouter>,
    );
    expect(await screen.findByText("keep-page")).toBeInTheDocument();
    view.unmount();
    render(
      <MemoryRouter initialEntries={["/shared"]}>
        <second.Root />
      </MemoryRouter>,
    );
    expect(await screen.findByText("other-page")).toBeInTheDocument();
    first.dispose();
    second.dispose();
  });
});
