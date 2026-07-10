import { test, expect } from '@playwright/test';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';

async function mockSignedOut(page) {
  await page.route('**/auth/session', (route) => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ authenticated: false }),
  }));
}

async function openAuth(page) {
  await page.goto(BASE_URL);
  await page.locator('[data-desk-menu-button]').click();
  await page.locator('[data-prompt-surface-login]').click();
  await expect(page.getByRole('dialog', { name: 'Sign in to Choir' })).toBeVisible();
}

test('Escape closes sign-in and returns focus to the desktop prompt', async ({ page }) => {
  await mockSignedOut(page);
  await openAuth(page);

  await page.keyboard.press('Escape');

  await expect(page.locator('[data-auth-overlay]')).toHaveCount(0);
  await expect(page.locator('[data-prompt-input]')).toBeFocused();
});

test('email validation is connected to the active field', async ({ page }) => {
  await mockSignedOut(page);
  await openAuth(page);

  const email = page.locator('[data-register-view] input[type="email"]');
  await email.fill('not-an-email');
  await page.locator('[data-register-view] [data-auth-submit]').click();

  await expect(email).toHaveAttribute('aria-invalid', 'true');
  await expect(email).toHaveAttribute('aria-errormessage', 'auth-error');
  await expect(page.locator('#auth-error')).toContainText('valid email address');
});

test('missing returning-user passkey gives a concrete account recovery path', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('choir.auth.returning', 'true'));
  await mockSignedOut(page);
  await page.route('**/auth/login/begin', (route) => route.fulfill({
    status: 404,
    contentType: 'application/json',
    body: JSON.stringify({ error: 'passkey not found' }),
  }));
  await openAuth(page);

  await page.locator('[data-login-view] input[type="email"]').fill('person@example.com');
  await page.locator('[data-login-view] [data-auth-submit]').click();

  const error = page.locator('[data-passkey-error]');
  await expect(error).toContainText('No passkey was found for that email');
  await expect(error).toContainText('Create account');
  await expect(error).not.toContainText('404');
});
