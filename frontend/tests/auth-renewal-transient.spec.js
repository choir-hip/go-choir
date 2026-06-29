import { expect, test } from '@playwright/test';

async function importAuthModule(page) {
  await page.goto('/src/lib/auth.js');
  await page.evaluate(() => {
    window.setTimeout = (callback) => {
      queueMicrotask(callback);
      return 0;
    };
  });
}

test('fetchWithRenewal preserves transient renewal failure as typed transient error', async ({ page }) => {
  await page.route('**/api/protected', async (route) => {
    await route.fulfill({ status: 401, body: 'access expired' });
  });
  await page.route('**/auth/session', async (route) => {
    await route.fulfill({ status: 503, body: 'deploy restart' });
  });

  await importAuthModule(page);

  const result = await page.evaluate(async () => {
    const auth = await import('/src/lib/auth.js');
    try {
      await auth.fetchWithRenewal('/api/protected');
      return { threw: false };
    } catch (err) {
      return {
        threw: true,
        name: err?.name,
        transient: err instanceof auth.TransientAuthError,
        authRequired: err instanceof auth.AuthRequiredError,
      };
    }
  });

  expect(result).toEqual({
    threw: true,
    name: 'TransientAuthError',
    transient: true,
    authRequired: false,
  });
});

test('fetchWithRenewal still reports definitive signed-out renewal as auth required', async ({ page }) => {
  await page.route('**/api/protected', async (route) => {
    await route.fulfill({ status: 401, body: 'access expired' });
  });
  await page.route('**/auth/session', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ authenticated: false }),
    });
  });

  await importAuthModule(page);

  const result = await page.evaluate(async () => {
    const auth = await import('/src/lib/auth.js');
    try {
      await auth.fetchWithRenewal('/api/protected');
      return { threw: false };
    } catch (err) {
      return {
        threw: true,
        name: err?.name,
        transient: err instanceof auth.TransientAuthError,
        authRequired: err instanceof auth.AuthRequiredError,
      };
    }
  });

  expect(result).toEqual({
    threw: true,
    name: 'AuthRequiredError',
    transient: false,
    authRequired: true,
  });
});
