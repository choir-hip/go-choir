import { test, expect } from '@playwright/test';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';

test('a failed app module offers a reload that recovers', async ({ page }) => {
  await page.route('**/auth/session', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: false }),
  }));

  let textureModuleRequests = 0;
  await page.route(/\/src\/lib\/TextureEditor\.svelte(?:\?.*)?$/, async (route) => {
    textureModuleRequests += 1;
    if (textureModuleRequests === 1) {
      await route.fulfill({ status: 503, body: 'transient module failure' });
      return;
    }
    await route.continue();
  });

  await page.goto(BASE_URL);
  await expect(page.getByRole('alert')).toContainText('Could not open Texture');

  await page.getByRole('button', { name: 'Reload app' }).click();

  await expect(page.locator('[data-texture-editor]')).toBeVisible();
  expect(textureModuleRequests).toBeGreaterThanOrEqual(2);
});
