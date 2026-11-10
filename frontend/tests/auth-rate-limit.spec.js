// A 429 from /auth is a wait, never "signed out" and never the generic passkey
// failure (docs/problems/auth-rate-limit-locks-out-account-switching-2026-11-09.md).
// Failure modes, written first:
//  R1 a 429 on /auth/session signs the shell out;
//  R2 the sign-in sheet hides a 429 behind "Passkey sign-in failed";
//  R3 the account switcher hides a 429 on switch behind a generic error;
//  R4 the session read is not retried after the 429 clears.
// /auth and /api are mocked, so no staging or local services are needed beyond
// the Vite dev server (GO_CHOIR_UX_BASE_URL, default http://127.0.0.1:5173).
import { test, expect } from '@playwright/test';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';
const ALICE = { id: 'user-alice', email: 'alice@example.com' };
const BOB = { id: 'user-bob', email: 'bob@example.com' };

const json = (route, body, status = 200, headers = {}) =>
  route.fulfill({ status, contentType: 'application/json', headers, body: JSON.stringify(body) });

async function mockCommon(context) {
  await context.route('**/api/**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
  await context.route('**/api/shell/bootstrap', (route) => json(route, { computer_id: 'cmp-alice' }));
  await context.route('**/api/preferences/theme', (route) => json(route, { theme: {} }));
}

test.describe('auth rate limit is a wait', () => {
  test('a 429 on /auth/session keeps the shell signed in and retries after Retry-After', async ({ page, context }) => {
    await mockCommon(context);
    const state = { limited: false, calls: 0, callsWhileLimited: 0 };
    await context.route('**/auth/session', (route) => {
      state.calls += 1;
      if (state.limited) {
        state.callsWhileLimited += 1;
        return json(route, { error: 'too many auth requests from this address', retry_after: 4 }, 429, { 'Retry-After': '4' });
      }
      return json(route, { authenticated: true, user: ALICE });
    });
    await page.goto(BASE_URL);
    await expect(page.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();

    state.limited = true;
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await expect.poll(() => state.callsWhileLimited).toBeGreaterThan(0);
    await page.waitForTimeout(500);
    // Still signed in: no sign-in overlay, the desktop is up, the Desk menu still lists Sign out, not Sign in.
    await expect(page.locator('[data-desktop]')).toBeVisible();
    await expect(page.locator('.app-root')).toHaveAttribute('data-auth-state', 'signed_in');
    await expect(page.locator('[data-auth-overlay]')).toHaveCount(0);

    // The window frees: the retry (after at least Retry-After) reads the session again.
    state.limited = false;
    const before = state.calls;
    await expect.poll(() => state.calls, { timeout: 15000 }).toBeGreaterThan(before);
    await expect(page.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
    await expect(page.locator('.app-root')).toHaveAttribute('data-auth-state', 'signed_in');
    await expect(page.locator('[data-auth-overlay]')).toHaveCount(0);
  });

  test('the sign-in sheet shows the wait for a 429 on login/begin, not the generic failure', async ({ page, context }) => {
    await mockCommon(context);
    await context.route('**/auth/session', (route) => json(route, { authenticated: false }));
    await context.route('**/auth/login/begin', (route) =>
      json(route, { error: 'too many auth requests from this address', retry_after: 17 }, 429, { 'Retry-After': '17' }));
    await page.goto(BASE_URL);
    await page.locator('[data-desk-menu-button]').click();
    await page.locator('[data-prompt-surface-login]').click();
    const overlay = page.locator('[data-auth-overlay]');
    await overlay.waitFor({ state: 'visible' });
    const toggle = overlay.locator('[data-login-toggle]');
    if (await toggle.count()) await toggle.click();
    await overlay.locator('input[type="email"]').fill('alice@example.com');
    await overlay.locator('[data-auth-submit]').click();
    const message = overlay.locator('[data-passkey-error]');
    await expect(message).toContainText('Too many sign-in attempts from this network. Try again in 17 seconds.');
    await expect(message).not.toContainText('Passkey sign-in failed');
  });

  test('the account switcher shows the wait when a switch is rate limited', async ({ page, context }) => {
    await mockCommon(context);
    await context.route('**/auth/session', (route) => json(route, { authenticated: true, user: ALICE }));
    await context.route('**/auth/accounts', (route) => json(route, [
      { user_id: ALICE.id, email: ALICE.email, active: true, signed_in: true },
      { user_id: BOB.id, email: BOB.email, active: false, signed_in: true },
    ]));
    await context.route('**/auth/accounts/switch', (route) =>
      json(route, { error: 'too many auth requests from this address', retry_after: 9 }, 429, { 'Retry-After': '9' }));
    await page.goto(BASE_URL);
    await expect(page.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
    await page.locator('[data-desk-menu-button]').click();
    await page.locator('[data-desk-account][data-active="false"] [data-desk-account-switch]').click();
    await expect(page.locator('[data-desk-accounts-error]')).toContainText('Try again in 9 seconds.');
    // The active account stays active and signed in.
    await expect(page.locator('[data-desk-account][data-active="true"]')).toContainText('alice@example.com');
  });
});
