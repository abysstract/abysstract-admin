import { expect, test } from "@playwright/test";

test.use({
  launchOptions: {
    ignoreDefaultArgs: ["--disable-back-forward-cache"],
    args: ["--enable-features=BackForwardCache"],
  },
});

test("restores the login page from the browser back/forward cache", async ({ page }) => {
  await page.goto("/login");
  await expect(page.getByPlaceholder("用户名")).toBeVisible();
  await page.evaluate(() => {
    const state = window as Window & { __adminPageShowPersisted?: boolean };
    state.__adminPageShowPersisted = false;
    window.addEventListener("pageshow", (event) => {
      state.__adminPageShowPersisted = event.persisted;
    });
  });

  await page.goto("/login?navigation=away");
  await expect(page.getByPlaceholder("用户名")).toBeVisible();
  await page.goBack({ waitUntil: "commit" });

  await expect
    .poll(() =>
      page.evaluate(
        () => (window as Window & { __adminPageShowPersisted?: boolean }).__adminPageShowPersisted,
      ),
    )
    .toBe(true);
  await expect(page.getByPlaceholder("用户名")).toBeVisible();
});
