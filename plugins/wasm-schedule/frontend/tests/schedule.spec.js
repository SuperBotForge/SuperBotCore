const { expect, test } = require("@playwright/test");

const frontendURL = process.env.SCHEDULE_FRONTEND_URL || "http://127.0.0.1:5173";
const coreURL = process.env.SCHEDULE_CORE_URL || "http://127.0.0.1:4000";
const isBundled = new URL(frontendURL).origin === coreURL;

test("renders schedule returned by the core HTTP trigger", async ({ page }) => {
  await page.route(`${coreURL}/api/triggers/http/schedule/api/schedule**`, async (route) => {
    await route.fulfill({
      status: 200,
      headers: {
        "access-control-allow-credentials": "true",
        "access-control-allow-origin": frontendURL,
        "content-type": "application/json",
      },
      body: JSON.stringify({
        building: "2",
        room: "203",
        date: "2026-05-27",
        classes: [
          { time: "08:30-10:00", subject: "Databases", teacher: "Kozlov I.P." },
          { time: "10:15-11:45", subject: "OS", teacher: "Morozova T.N." },
        ],
      }),
    });
  });

  await page.goto(frontendURL);
  if (isBundled) {
    await expect(page.locator("#coreUrlField")).toBeHidden();
  } else {
    await expect(page.locator("#coreUrlField")).toBeVisible();
  }
  await page.getByLabel("Building").getByText("2").click();
  await page.locator("#room").fill("203");
  await page.getByRole("button", { name: "Load schedule" }).click();

  await expect(page.locator("#statusText")).toHaveText("Loaded");
  await expect(page.locator("#summary")).toHaveText("Building 2, room 203, 2026-05-27");
  await expect(page.getByText("Databases")).toBeVisible();
  await expect(page.locator(".class-row")).toHaveCount(2);
  await page.screenshot({ path: "/tmp/schedule-frontend-success.png", fullPage: true });
});

test("builds core TSU login redirect with frontend return_to", async ({ page }) => {
  await page.route(`${coreURL}/api/triggers/http/schedule/api/schedule**`, async (route) => {
    await route.fulfill({
      status: 401,
      headers: {
        "access-control-allow-credentials": "true",
        "access-control-allow-origin": frontendURL,
        "content-type": "application/json",
      },
      body: JSON.stringify({ error: "authentication required" }),
    });
  });
  await page.route(`${coreURL}/api/auth/tsu/start**`, async (route) => {
    await route.fulfill({ status: 200, body: "login redirect captured" });
  });

  await page.goto(frontendURL);
  await page.getByRole("button", { name: "Login" }).click();

  await expect(page.getByText("login redirect captured")).toBeVisible();
  const url = new URL(page.url());
  expect(url.origin).toBe(coreURL);
  expect(url.pathname).toBe("/api/auth/tsu/start");
  const expectedReturnTo = isBundled
    ? new URL(frontendURL).pathname
    : `${frontendURL}/`;
  expect(url.searchParams.get("return_to")).toBe(expectedReturnTo);
});
