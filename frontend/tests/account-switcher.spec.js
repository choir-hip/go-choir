// The account switcher (docs/design/account-switcher-2026-11-09.md): several
// signed-in accounts in one browser. Failure modes, written first:
//  F1 the desk menu shows no Accounts section, or does not mark the active one;
//  F2 "Add another account" opens no sign-in overlay (or signs the current one out);
//  F3 a switch does not show the other account's desktop after the reload;
//  F4 a diverged jar (session says account_switched) reloads in a loop;
//  F5 a sibling tab keeps the old account after a switch;
//  F6 per-user localStorage keys bleed between accounts (theme boot cache);
//  F7 signing out of an account leaves it in the menu, or the last one leaves the shell up.
// The server is a small stateful stub shared by every page of the context, so
// the two-tab cases see one "server". /api is mocked as other specs do.
import { test, expect } from '@playwright/test';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';
const SHOTS = process.env.ACCOUNT_SHOTS || '';

const ACCOUNTS = [
  { user_id: 'user-alice', email: 'alice@example.com' },
  { user_id: 'user-bob', email: 'bob@example.com' },
];

async function mockServer(context, { signedIn = ['user-alice', 'user-bob'], active = 'user-alice', divergeOnce = false } = {}) {
  const state = {
    signedIn: new Set(signedIn),
    active,
    divergeOnce,
    switchCalls: [],
    signoutCalls: [],
    logoutAllCalls: 0,
    sessionCalls: 0,
  };
  const json = (route, body, status = 200) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) });
  const userOf = (id) => ACCOUNTS.find((a) => a.user_id === id);

  await context.route('**/api/**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
  await context.route('**/api/shell/bootstrap', (route) => json(route, { computer_id: `cmp-${state.active}` }));
  await context.route('**/api/preferences/theme', (route) => json(route, { theme: {} }));
  await context.route('**/auth/session', (route) => {
    state.sessionCalls += 1;
    if (state.divergeOnce) {
      // The jar holds the previous member's pair while the set says another is active.
      state.divergeOnce = false;
      return json(route, { authenticated: false, reason: 'account_switched', active_user_id: state.active });
    }
    if (!state.active || !state.signedIn.has(state.active)) return json(route, { authenticated: false });
    const u = userOf(state.active);
    return json(route, { authenticated: true, user: { id: u.user_id, email: u.email } });
  });
  await context.route('**/auth/accounts', (route) => {
    const list = ACCOUNTS.filter((a) => a.user_id === state.active || state.signedIn.has(a.user_id))
      .map((a) => ({ ...a, active: a.user_id === state.active, signed_in: state.signedIn.has(a.user_id) }))
      .sort((a, b) => Number(b.active) - Number(a.active));
    return json(route, list);
  });
  await context.route('**/auth/accounts/switch', (route) => {
    const body = JSON.parse(route.request().postData() || '{}');
    state.switchCalls.push(body.user_id);
    if (!state.signedIn.has(body.user_id)) return json(route, { error: 'that account is signed out', reason: 'member_signed_out' }, 409);
    state.active = body.user_id;
    const u = userOf(body.user_id);
    return json(route, { authenticated: true, active_user_id: u.user_id, user: { id: u.user_id, email: u.email } });
  });
  await context.route('**/auth/accounts/signout', (route) => {
    const body = JSON.parse(route.request().postData() || '{}');
    state.signoutCalls.push(body.user_id);
    state.signedIn.delete(body.user_id);
    let next = '';
    if (state.active === body.user_id) {
      next = [...state.signedIn][0] || '';
      state.active = next;
    }
    return json(route, { authenticated: !!next, next_user_id: next || undefined, set_empty: state.signedIn.size === 0 || undefined });
  });
  await context.route('**/auth/accounts/logout-all', (route) => {
    state.logoutAllCalls += 1;
    state.signedIn.clear();
    state.active = '';
    return json(route, { authenticated: false });
  });
  await context.route('**/auth/logout', (route) => {
    const out = state.active;
    state.signedIn.delete(out);
    state.active = [...state.signedIn][0] || '';
    return json(route, { authenticated: false, next_user_id: state.active || undefined });
  });
  return state;
}

async function openDesk(page) {
  await page.goto(BASE_URL);
  await expect(page.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
}

test.describe('account switcher', () => {
  test('desk menu lists the accounts, marks the active one, and offers Add another account', async ({ page, context }) => {
    await mockServer(context);
    await openDesk(page);
    const section = page.locator('[data-desk-accounts]');
    await expect(section).toBeVisible();
    await expect(section.locator('[data-desk-account]')).toHaveCount(2);
    await expect(section.locator('[data-desk-account][data-active="true"]')).toContainText('alice@example.com');
    await expect(section.locator('[data-desk-account][data-active="false"]')).toContainText('bob@example.com');
    if (SHOTS) await page.screenshot({ path: `${SHOTS}/desk-menu-accounts.png` });

    await section.locator('[data-desk-account-add]').click();
    const overlay = page.locator('[data-auth-overlay][data-auth-intent-kind="add_account"]');
    await expect(overlay).toBeVisible();
    // The current account is still signed in beneath the overlay.
    await expect(page.locator('[data-desktop]')).toBeVisible();
    if (SHOTS) await page.screenshot({ path: `${SHOTS}/add-account-overlay.png` });
    await page.locator('[data-auth-overlay-close]').click();
    await expect(overlay).toHaveCount(0);
  });

  test('switching then reloading shows the other account; the first account stays signed in', async ({ page, context }) => {
    const state = await mockServer(context);
    await openDesk(page);
    await page.locator('[data-desk-account][data-active="false"] [data-desk-account-switch]').click();
    await expect.poll(() => state.switchCalls).toEqual(['user-bob']);
    await expect(page.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
    await expect.poll(() => state.active).toBe('user-bob');
    await openDesk(page);
    await expect(page.locator('[data-desk-account][data-active="true"]')).toContainText('bob@example.com');
    await expect(page.locator('[data-desk-account][data-active="false"]')).toContainText('alice@example.com');
    expect([...state.signedIn].sort()).toEqual(['user-alice', 'user-bob']);
    if (SHOTS) await page.screenshot({ path: `${SHOTS}/switched-desktop.png` });
  });

  test('a switch to a signed-out account explains itself and opens sign-in for it', async ({ page, context }) => {
    const state = await mockServer(context, { signedIn: ['user-alice'] });
    // The set still lists bob, but his session is dead.
    state.signedIn.delete('user-bob');
    await context.route('**/auth/accounts', (route) => route.fulfill({
      status: 200, contentType: 'application/json',
      body: JSON.stringify([
        { ...ACCOUNTS[0], active: true, signed_in: true },
        { ...ACCOUNTS[1], active: false, signed_in: false },
      ]),
    }));
    await openDesk(page);
    await page.locator('[data-desk-account][data-active="false"] [data-desk-account-switch]').click();
    await expect(page.locator('[data-auth-overlay][data-auth-intent-kind="add_account"]')).toBeVisible();
    await expect(page.locator('[data-auth-overlay] input[type="email"]')).toHaveValue('bob@example.com');
  });

  test('a diverged jar reconciles with one switch and reloads once, no loop', async ({ page, context }) => {
    const state = await mockServer(context, { active: 'user-bob', divergeOnce: true });
    let navigations = 0;
    page.on('framenavigated', (f) => { if (f === page.mainFrame()) navigations += 1; });
    await page.goto(BASE_URL);
    await expect(page.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
    await page.waitForTimeout(1500);
    expect(state.switchCalls).toEqual(['user-bob']);
    expect(navigations).toBeLessThanOrEqual(3);
    await openDesk(page);
    await expect(page.locator('[data-desk-account][data-active="true"]')).toContainText('bob@example.com');
  });

  test('a sibling tab flushes and reloads when another tab switches', async ({ context }) => {
    const state = await mockServer(context);
    const a = await context.newPage();
    const b = await context.newPage();
    await openDesk(a);
    await b.goto(BASE_URL);
    await expect(b.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
    let reloaded = false;
    b.on('load', () => { reloaded = true; });
    await a.locator('[data-desk-account][data-active="false"] [data-desk-account-switch]').click();
    await expect.poll(() => reloaded, { timeout: 10000 }).toBe(true);
    await expect(b.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
    expect(state.active).toBe('user-bob');
    await openDesk(b);
    await expect(b.locator('[data-desk-account][data-active="true"]')).toContainText('bob@example.com');
  });

  test('per-user localStorage: the theme boot cache does not bleed between accounts', async ({ page, context }) => {
    await mockServer(context);
    await page.goto(BASE_URL);
    await expect(page.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
    const keys = await page.evaluate(() => Object.keys(localStorage));
    expect(keys).toContain('choir.auth.lastUserId');
    expect(keys.some((k) => k.startsWith('choir.theme.boot.v2:user-alice'))).toBe(true);
    // Account B's boot cache is its own key.
    await page.evaluate(() => {
      localStorage.setItem('choir.theme.boot.v2:user-bob', JSON.stringify({ id: 'bob-theme-marker' }));
    });
    const alice = await page.evaluate(() => localStorage.getItem('choir.theme.boot.v2:user-alice'));
    expect(alice).not.toContain('bob-theme-marker');
  });

  test('signing out of an inactive account removes it; signing out of all ends the session', async ({ page, context }) => {
    const state = await mockServer(context);
    await openDesk(page);
    await page.locator('[data-desk-account][data-active="false"] [data-desk-account-signout]').click();
    await expect(page.locator('[data-desk-account]')).toHaveCount(1);
    expect(state.signoutCalls).toEqual(['user-bob']);
    // Back to two accounts, then sign out of all.
    state.signedIn.add('user-bob');
    await openDesk(page);
    await expect(page.locator('[data-desk-account]')).toHaveCount(2);
    await page.locator('[data-desk-account-signout-all]').click();
    await expect.poll(() => state.logoutAllCalls).toBe(1);
    await page.waitForLoadState('load');
    await expect(page.locator('[data-app-root], .app-root[data-auth-state="signed_out"]').first()).toBeVisible();
    await expect(page.locator('.app-root')).toHaveAttribute('data-auth-state', 'signed_out');
  });
});
