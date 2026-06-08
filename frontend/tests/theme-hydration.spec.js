import { test, expect } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';

const carbonTheme = {
  schema_version: 2,
  id: 'carbon-fiber-kintsugi',
  name: 'Carbon Fiber Kintsugi',
  layout: { promptSurfacePlacement: 'bottom' },
};

test('theme boot cache paints before authenticated server preference resolves', async ({ page }) => {
  let releaseSession;
  const sessionReady = new Promise((resolve) => {
    releaseSession = resolve;
  });
  let releaseTheme;
  const themeReady = new Promise((resolve) => {
    releaseTheme = resolve;
  });

  await page.addInitScript((theme) => {
    window.localStorage.setItem('choir.theme.boot.v2', JSON.stringify(theme));
  }, carbonTheme);

  await page.route('**/auth/session', async (route) => {
    await sessionReady;
    await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({
      authenticated: true,
      user: { email: 'theme-hydration@example.com' },
    }),
    });
  });

  await page.route('**/api/preferences/theme', async (route) => {
    await themeReady;
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ theme: carbonTheme }),
    });
  });

  await page.goto(BASE_URL);

  const root = page.locator('[data-auth-state]');
  await expect(root).toHaveAttribute('data-theme-id', 'carbon-fiber-kintsugi');
  await expect(root).toHaveAttribute('data-auth-state', 'checking');

  releaseSession();
  releaseTheme();
  await expect(root).toHaveAttribute('data-auth-state', 'signed_in');
  await expect(root).toHaveAttribute('data-theme-id', 'carbon-fiber-kintsugi');
  await expect(page.locator('[data-desktop]')).toHaveCount(1);
});
