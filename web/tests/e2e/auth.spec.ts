import { test, expect } from "@playwright/test";

test.describe("authentication", () => {
  test("shows actionable validation for an invalid login", async ({ page }) => {
    await page.goto("/login");

    await page.getByLabel("Email").fill("not-an-email");
    await page.getByLabel("Password").fill("");
    await page.getByRole("button", { name: "Sign in" }).click();

    await expect(page.getByText("Invalid email")).toBeVisible();
    await expect(page.getByText("Password is required")).toBeVisible();
  });

  test("logs in and lands on the dashboard with the responsive auth shell", async ({
    page,
  }) => {
    await page.route("**/v1/**", async (route) => {
      const url = new URL(route.request().url());
      if (url.pathname.endsWith("/auth/login")) {
        await route.fulfill({
          json: { data: { access_token: "e2e-access-token" } },
        });
        return;
      }
      if (url.pathname.endsWith("/users/me")) {
        await route.fulfill({
          json: {
            data: {
              id: "user-1",
              email: "sam@example.com",
              display_name: "Sam Rivera",
              currency_pref: "LKR",
              timezone: "UTC",
              identity_type: "registered",
            },
          },
        });
        return;
      }
      if (url.pathname.endsWith("/teams")) {
        await route.fulfill({ json: { data: [] } });
        return;
      }
      if (url.pathname.endsWith("/notifications")) {
        await route.fulfill({ json: { data: { items: [], has_more: false } } });
        return;
      }
      await route.fulfill({ json: { data: [] } });
    });
    await page.route("**/auth/login", async (route) => {
      await route.fulfill({
        json: { data: { access_token: "e2e-access-token" } },
      });
    });
    await page.route("**/graphql", async (route) => {
      await route.fulfill({
        json: {
          data: {
            dashboardAggregates: {
              totalOwed: 4200,
              totalOwing: 1700,
              netBalance: 2500,
            },
          },
        },
      });
    });

    await page.goto("/login");
    await page.getByLabel("Email").fill("sam@example.com");
    await page.getByLabel("Password").fill("correct horse battery staple");
    await page.getByRole("button", { name: "Sign in" }).click();

    await expect(page).toHaveURL(/\/dashboard$/);
    await expect(
      page.getByRole("heading", { name: /Good to see you, Sam/ }),
    ).toBeVisible();
    await expect(page.getByText("LKR 25.00")).toBeVisible();
  });
});
