import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

// Real antd: ConfigProvider forwarding is only observable through antd itself.
vi.unmock("antd");
vi.unmock("@ant-design/icons");

import { createAdminApp } from "./application";
import { createMemoryAuthStorage } from "./storage";
import type { AdminHost } from "./types";

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
  createSystemOperations: () => ({ userOperations: { list: vi.fn() } }),
}));

function host(overrides: Partial<AdminHost> = {}): AdminHost {
  return {
    apiBaseUrl: "http://localhost:8080",
    mode: "development",
    authStorage: createMemoryAuthStorage(),
    title: "ConfigProvider 测试",
    ...overrides,
  };
}

async function renderLoginButton(overrides: Partial<AdminHost>): Promise<HTMLElement> {
  const app = createAdminApp({ host: host(overrides), modules: [] });
  apps.push(app);
  render(
    <MemoryRouter initialEntries={["/login"]}>
      <app.Root />
    </MemoryRouter>,
  );
  await screen.findByText("ConfigProvider 测试");
  return screen.getByRole("button", { name: /登\s?录/ });
}

const apps: { dispose(): void }[] = [];

describe("createAdminApp ConfigProvider", () => {
  afterEach(() => {
    apps.splice(0).forEach((app) => app.dispose());
  });

  it("disables antd's two-character button spacing by default", async () => {
    const button = await renderLoginButton({});
    expect(button.textContent).toBe("登录");
  });

  it("forwards host.configProvider and lets the host override button spacing", async () => {
    const button = await renderLoginButton({
      configProvider: { button: { autoInsertSpace: true }, componentSize: "small" },
    });
    expect(button.textContent).toBe("登 录");
  });
});
