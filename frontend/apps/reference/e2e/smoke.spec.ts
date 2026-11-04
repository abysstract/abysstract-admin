import { expect, test, type Page } from "@playwright/test";

const username = process.env.ADMIN_E2E_USERNAME ?? "admin";
const password = process.env.ADMIN_E2E_PASSWORD ?? "admin123";

async function login(page: Page, user: string, pass: string) {
  await page.goto("/");
  await page.getByPlaceholder("用户名").fill(user);
  await page.getByPlaceholder("密码").fill(pass);
  const loginRequest = page.waitForResponse(
    (response) => response.url().includes("AuthService/Login"),
    { timeout: 20_000 },
  );
  await page.locator("button.ant-btn-primary").click();
  const response = await loginRequest;
  expect(response.ok(), `login HTTP ${response.status()}`).toBeTruthy();
}

test.afterEach(async ({ page }) => {
  await page.evaluate(() => {
    localStorage.clear();
    sessionStorage.clear();
  });
  await page.context().clearCookies();
});

test("login and system user form", async ({ page }) => {
  test.setTimeout(180_000);
  await login(page, username, password);
  await expect(page.getByText("超级管理员")).toBeVisible({ timeout: 20_000 });
  // The seeded administrator lands on the first authorized page after login.
  await expect(page).toHaveURL(/\/system\/user$/);
  await expect(page.getByRole("button", { name: /新建用户/ })).toBeVisible();
  await expect(page.getByRole("columnheader", { name: "用户名" })).toBeVisible();
  await page.getByRole("button", { name: /新建用户/ }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  await expect(dialog.locator(".ant-select, [role='combobox']").first()).toBeVisible();
  await page.locator(".ant-modal-close").click();
  await expect(dialog).toBeHidden();
});
