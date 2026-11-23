// A full sign-up is a plain answer, not a failure (owner, 2026-11-23: sign-up
// closes at an owner-set number; "sign-up is full for now; you are on the
// list"). Failure mode, written first:
//  F1 the sheet hides the server's 403 registration_waitlist behind "Passkey
//     sign-in failed. Nothing changed—try again", so a person at the cap
//     retries a sign-up that cannot succeed.
// /auth and /api are mocked; only the Vite dev server is needed
// (GO_CHOIR_UX_BASE_URL, default http://127.0.0.1:5173).
import { test, expect } from '@playwright/test';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';
const json = (route, body, status = 200) =>
  route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });

test('the create-account sheet says the sign-up is full, in the server\'s words', async ({ page, context }) => {
  await context.route('**/api/**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
  await context.route('**/auth/session', (route) => json(route, { authenticated: false }));
  await context.route('**/auth/register/begin', (route) =>
    json(route, { error: 'Sign-up is full for now. You are on the list.', code: 'registration_waitlist' }, 403));
  await page.goto(BASE_URL);
  await page.locator('[data-desk-menu-button]').click();
  await page.locator('[data-prompt-surface-login]').click();
  const overlay = page.locator('[data-auth-overlay]');
  await overlay.waitFor({ state: 'visible' });
  await overlay.locator('input[type="email"]').fill('stranger@example.com');
  await overlay.locator('[data-auth-submit]').click();
  const message = overlay.locator('[data-passkey-error]');
  await expect(message).toContainText('Sign-up is full for now. You are on the list.');
  await expect(message).not.toContainText('Passkey sign-in failed');
});
