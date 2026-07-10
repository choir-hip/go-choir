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
  await expect(page.locator('[data-auth-overlay]')).toBeVisible();
}

test('new-user mode preserves email when switching to sign in', async ({ page }) => {
  await mockSignedOut(page);
  await openAuth(page);

  const registerEmail = page.locator('[data-register-view] input[type="email"]');
  await expect(registerEmail).toBeFocused();
  await registerEmail.fill('person@example.com');
  await page.locator('[data-login-toggle]').click();

  const loginEmail = page.locator('[data-login-view] input[type="email"]');
  await expect(loginEmail).toHaveValue('person@example.com');
  await expect(loginEmail).toBeFocused();
  await expect(page.locator('[data-login-toggle]')).toHaveAttribute('aria-selected', 'true');
  await expect(page.locator('[data-auth-entry]')).toContainText('Then Choir returns you to the action above.');
});

test('a known returning browser opens directly on sign in', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('choir.auth.returning', 'true'));
  await mockSignedOut(page);
  await openAuth(page);

  const loginEmail = page.locator('[data-login-view] input[type="email"]');
  await expect(loginEmail).toBeVisible();
  await expect(loginEmail).toBeFocused();
  await expect(page.locator('[data-register-view]')).toHaveCount(0);
});
