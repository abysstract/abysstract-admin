import { defineConfig, devices } from "@playwright/test";

const baseURL = process.env.ADMIN_E2E_BASE_URL ?? "http://127.0.0.1:4173";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  timeout: 180_000,
  expect: { timeout: 20_000 },
  reporter: [["list"]],
  outputDir: process.env.ADMIN_E2E_OUTPUT ?? "/tmp/admin-reference-playwright-results",
  use: {
    baseURL,
    trace: "off",
    screenshot: "off",
    video: "off",
    ...devices["Desktop Chrome"],
    // BFCache requires the full browser; CI uses Chromium's new headless mode.
    channel: process.env.CI ? "chromium" : "chrome",
  },
});
