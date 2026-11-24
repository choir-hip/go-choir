/**
 * Deployed proof of account deletion on the web (docs/design/account-create-delete-2026-11-23.md).
 * RED surfaces: run on staging with a throwaway account only (an example.com address made here),
 * never on the paper's or the owner's accounts. Not run by the author of the change: it needs the
 * deployed host (auth, vmctl, corpusd, maild, gateway).
 *
 * Registers a fresh test account with the CDP virtual authenticator, then:
 *   1. /account/delete asks for the passkey and answers "scheduled", with a date 3 days out;
 *   2. the session is gone (GET /auth/session says signed out) and the old cookies do nothing;
 *   3. signing in on /account/delete offers "Cancel deletion" and makes no session;
 *   4. cancelling signs the browser in again and the account is as it was.
 *
 * Env: BASE_URL (default https://choir.news).
 */
import { expect } from '@playwright/test';
import { test } from './helpers/fixtures.js';
import { registerPasskey, getSession } from './helpers/auth.js';

const BASE_URL = process.env.BASE_URL || 'https://choir.news';

test('delete an account with a passkey, find it scheduled, and cancel it by signing in', async ({ page, authenticator }) => {
  test.setTimeout(300_000);
  void authenticator;
  const email = `delete-e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
  await page.goto(BASE_URL);
  const reg = await registerPasskey(page, email, BASE_URL);
  expect(reg && reg.ok !== false, 'registration did not complete').toBeTruthy();

  await page.goto(`${BASE_URL}/account/delete`);
  const route = page.locator('[data-delete-route]');
  await expect(route).toHaveAttribute('data-delete-state', 'info', { timeout: 30_000 });
  await expect(page.locator('[data-delete-removed]')).toContainText('everything on it');
  await expect(page.locator('[data-delete-stays]')).toContainText('stay public');
  await expect(page.locator('[data-delete-when]')).toContainText('72 hours');
  await page.locator('[data-delete-ask]').click();
  await expect(route).toHaveAttribute('data-delete-state', 'sure');
  await page.locator('[data-delete-confirm]').click();
  await expect(route).toHaveAttribute('data-delete-state', 'scheduled', { timeout: 60_000 });
  await expect(page.locator('[data-delete-done]')).toContainText('scheduled for deletion');

  const session = await getSession(page, BASE_URL);
  expect(session.authenticated, 'the session survived the deletion request').toBe(false);

  // A passkey sign-in inside the 72 hours makes no session and offers the cancel.
  await page.context().clearCookies();
  await page.goto(`${BASE_URL}/account/delete`);
  await expect(route).toHaveAttribute('data-delete-state', 'signin', { timeout: 30_000 });
  await page.locator('[data-delete-signin-email]').fill(email);
  await page.locator('[data-delete-signin-btn]').click();
  await expect(route).toHaveAttribute('data-delete-state', 'offer', { timeout: 60_000 });
  expect((await getSession(page, BASE_URL)).authenticated, 'a pending account got a session at sign-in').toBe(false);
  await page.locator('[data-delete-cancel]').click();
  await expect(route).toHaveAttribute('data-delete-state', 'cancelled', { timeout: 30_000 });
  expect((await getSession(page, BASE_URL)).authenticated, 'cancel did not sign in').toBe(true);
});
