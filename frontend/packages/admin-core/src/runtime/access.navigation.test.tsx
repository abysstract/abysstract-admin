import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes, Link } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Transport } from "@connectrpc/connect";
import { AccessProvider } from "./access";
import { AdminRuntimeProvider } from "./context";
import { SessionController } from "./session";
import { createMemoryAuthStorage } from "./storage";
import { createModuleRegistry } from "../module/registry";
import type { SystemClients } from "../system/clients";
import type { SystemOperations } from "../system/operations";
import type { AdminHost } from "./types";

const auth = vi.hoisted(() => ({
  login: vi.fn(),
  logout: vi.fn(),
  getUserInfo: vi.fn(),
  changePassword: vi.fn(),
}));

function host(overrides: Partial<AdminHost> = {}): AdminHost {
  return {
    apiBaseUrl: "http://localhost:8080",
    mode: "development",
    authStorage: createMemoryAuthStorage("nav"),
    title: "Abysstract 后台管理",
    ...overrides,
  };
}

function renderAccessHarness(authStorage: AdminHost["authStorage"]) {
  const session = new SessionController(authStorage);
  render(
    <MemoryRouter initialEntries={["/a"]}>
      <AdminRuntimeProvider
        value={{
          transport: {} as Transport,
          auth: { signOut: () => Promise.resolve() },
          host: host({ authStorage }),
          session,
          clients: { auth } as unknown as SystemClients,
          operations: {} as unknown as SystemOperations,
          registry: createModuleRegistry([]),
        }}
      >
        <AccessProvider>
          <Routes>
            <Route
              path="/a"
              element={
                <>
                  <Link to="/b">go-b</Link>
                  <div>page-a</div>
                </>
              }
            />
            <Route path="/b" element={<div>page-b</div>} />
          </Routes>
        </AccessProvider>
      </AdminRuntimeProvider>
    </MemoryRouter>,
  );
  return session;
}

describe("AccessProvider 导航稳定性", () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it("菜单导航不重新拉取用户信息，子内容不回退全屏加载", async () => {
    const storage = createMemoryAuthStorage("nav");
    storage.setToken("token-a");
    auth.getUserInfo.mockResolvedValue({
      permissions: ["a:b:c"],
      roles: [],
      menus: [],
      user: { nickname: "导航用户" },
      pwdResetRequired: false,
    });
    renderAccessHarness(storage);

    expect(await screen.findByText("page-a")).toBeInTheDocument();
    expect(auth.getUserInfo).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("link", { name: "go-b" }));

    expect(screen.getByText("page-b")).toBeInTheDocument();
    expect(screen.queryByText("正在进入工作台…")).not.toBeInTheDocument();
    expect(auth.getUserInfo).toHaveBeenCalledTimes(1);
  });

  it("token 消失后清空 access，重新出现时按新 epoch 重新加载", async () => {
    const storage = createMemoryAuthStorage("nav");
    storage.setToken("token-a");
    auth.getUserInfo
      .mockResolvedValueOnce({
        permissions: ["a:b:c"],
        roles: [],
        menus: [],
        user: { nickname: "第一版" },
        pwdResetRequired: false,
      })
      .mockResolvedValueOnce({
        permissions: ["d:e:f"],
        roles: [],
        menus: [],
        user: { nickname: "第二版" },
        pwdResetRequired: false,
      });
    renderAccessHarness(storage);

    expect(await screen.findByText("page-a")).toBeInTheDocument();
    storage.clearToken();
    await screen.findByRole("status");
    expect(screen.queryByText("page-a")).not.toBeInTheDocument();
    expect(auth.getUserInfo).toHaveBeenCalledTimes(1);

    storage.setToken("token-b");
    expect(await screen.findByText("page-a")).toBeInTheDocument();
    expect(auth.getUserInfo).toHaveBeenCalledTimes(2);
  });
});
