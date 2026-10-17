import { test, expect } from "@playwright/test";

test("app shell is visible", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByTestId("app-shell")).toBeVisible();
});
