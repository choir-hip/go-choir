import { test, expect } from '@playwright/test';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';

test('a failed app module offers a reload that recovers', async ({ page }) => {
  await page.route('**/auth/session', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: false }),
  }));

  // The logged-out home opens the Autopaper window first.
  let autopaperModuleRequests = 0;
  await page.route(/\/src\/lib\/AutopaperApp\.svelte(?:\?.*)?$/, async (route) => {
    autopaperModuleRequests += 1;
    if (autopaperModuleRequests === 1) {
      await route.fulfill({ status: 503, body: 'transient module failure' });
      return;
    }
    await route.continue();
  });

  await page.goto(BASE_URL);
  await expect(page.getByRole('alert')).toContainText('Could not open Autopaper');

  await page.getByRole('button', { name: 'Reload app' }).click();

  await expect(page.locator('[data-autopaper]')).toBeVisible();
  expect(autopaperModuleRequests).toBeGreaterThanOrEqual(2);
});
