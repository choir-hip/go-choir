// The shell shows the default theme for the system colour scheme to an owner
// who has not chosen one, and a saved choice always wins.
// Run against a dev server: PLAYWRIGHT_BASE_URL=http://localhost:5173
//
// Failure list, written before the code:
//  S1 dark system, no choice: the desktop paints the light theme;
//  S2 the default is written to the boot cache as if chosen, so the next
//     scheme change cannot move it;
//  S3 the scheme changes while running and the default does not follow;
//  S4 an authenticated owner with no server preference stays on a stale cached
//     default (the old default) instead of the scheme default;
//  S5 an owner's saved choice is replaced by the scheme default;
//  S6 the first paint (boot cache) shows a theme other than the one the owner
//     chose.
import { test, expect } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';
const CACHE_KEY = 'choir.theme.boot.v2';

const aeroTheme = { schema_version: 2, id: 'aero-chrome-heart', name: 'Aero Chrome Heart', layout: { promptSurfacePlacement: 'bottom' } };

async function signedOut(page) {
  await page.route('**/auth/session', (route) => route.fulfill({
    status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: false }),
  }));
}
async function signedIn(page, preference) {
  await page.route('**/auth/session', (route) => route.fulfill({
    status: 200, contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, user: { id: 'u-theme-default', email: 'theme-default@example.com' } }),
  }));
  await page.route('**/api/preferences/theme', (route) => route.fulfill({
    status: 200, contentType: 'application/json', body: JSON.stringify(preference ? { theme: preference } : { theme: {} }),
  }));
}
const root = (page) => page.locator('[data-auth-state]');
// Every boot cache entry, plain or user-scoped (scopedKey is `${base}:${userId}`), as [key, value].
const bootEntries = (page) => page.evaluate((key) => Object.keys(window.localStorage)
  .filter((k) => k === key || k.startsWith(`${key}:`))
  .map((k) => [k, window.localStorage.getItem(k)]), CACHE_KEY);
const bootCache = async (page) => (await bootEntries(page)).map(([, v]) => v);
// What the old release wrote for every visitor: the whole default theme, with its colours, never marked as a choice.
const legacyDefault = { ...aeroTheme, colors: { bg: '#C4ECF9' } };
const packTheme = { schema_version: 2, id: 'paper-night.dark', name: 'Paper Night', chosen: true };

test('S1, S2: dark paints Liquid Metal, light paints Chrome Pastel, and neither is stored as a choice', async ({ page }) => {
  await signedOut(page);
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.goto(BASE_URL);
  await expect(root(page)).toHaveAttribute('data-theme-id', 'liquid-metal');
  expect(await bootCache(page)).toEqual([]);
  await page.emulateMedia({ colorScheme: 'light' });
  await page.reload();
  await expect(root(page)).toHaveAttribute('data-theme-id', 'chrome-pastel.hybrid');
  expect(await bootCache(page)).toEqual([]);
});

test('S3: the default follows a scheme change while running', async ({ page }) => {
  await signedOut(page);
  await page.emulateMedia({ colorScheme: 'light' });
  await page.goto(BASE_URL);
  await expect(page.locator('html')).toHaveAttribute('data-theme-id', 'chrome-pastel.hybrid');
  await page.emulateMedia({ colorScheme: 'dark' });
  await expect(page.locator('html')).toHaveAttribute('data-theme-id', 'liquid-metal');
  await expect(root(page)).toHaveAttribute('data-theme-id', 'liquid-metal');
});

test('S4: a stale cached default with no server preference gives way to the scheme default', async ({ page }) => {
  await signedIn(page, null);
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.addInitScript((theme) => {
    // The old default, cached by the old rule, which cached every applied theme.
    window.localStorage.setItem('choir.theme.boot.v2', JSON.stringify(theme));
  }, legacyDefault);
  await page.goto(BASE_URL);
  await expect(root(page)).toHaveAttribute('data-auth-state', 'signed_in');
  await expect(root(page)).toHaveAttribute('data-theme-id', 'liquid-metal');
  // The shown default is not stored as a choice, under the signed-in user's key or any other.
  expect((await bootCache(page)).filter((c) => /liquid-metal|chrome-pastel/.test(c))).toEqual([]);
  expect((await bootEntries(page)).filter(([k]) => k.startsWith(`${CACHE_KEY}:`))).toEqual([]);
});

test('S7: a saved choice whose pack has not registered yet is kept, not wiped, while the scheme default shows', async ({ page }) => {
  await signedOut(page);
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.addInitScript((theme) => {
    window.localStorage.setItem('choir.theme.boot.v2', JSON.stringify(theme));
  }, packTheme);
  await page.goto(BASE_URL);
  await expect(root(page)).toHaveAttribute('data-theme-id', 'liquid-metal');
  await expect(root(page)).toHaveAttribute('data-auth-state', 'signed_out');
  const kept = await bootCache(page);
  expect(kept.length).toBe(1);
  expect(JSON.parse(kept[0]).id).toBe('paper-night.dark');
});

test('S8: the old auto-written default is not a choice: signed out, dark shows Liquid Metal and follows the scheme', async ({ page }) => {
  await signedOut(page);
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.addInitScript((theme) => {
    window.localStorage.setItem('choir.theme.boot.v2', JSON.stringify(theme));
  }, legacyDefault);
  await page.goto(BASE_URL);
  await expect(root(page)).toHaveAttribute('data-auth-state', 'signed_out');
  await expect(root(page)).toHaveAttribute('data-theme-id', 'liquid-metal');
  await page.emulateMedia({ colorScheme: 'light' });
  await expect(root(page)).toHaveAttribute('data-theme-id', 'chrome-pastel.hybrid');
});

test('S9: a theme seeded the way choir-shot does (id only) and a marked choice are honoured in dark mode', async ({ page }) => {
  await signedOut(page);
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.addInitScript((theme) => {
    window.localStorage.setItem('choir.theme.boot.v2', JSON.stringify(theme));
  }, aeroTheme);
  await page.goto(BASE_URL);
  await expect(root(page)).toHaveAttribute('data-theme-id', 'aero-chrome-heart');
});

test('S10: a marked choice of Aero Chrome Heart is honoured in dark mode', async ({ page }) => {
  await signedOut(page);
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.addInitScript((theme) => {
    window.localStorage.setItem('choir.theme.boot.v2', JSON.stringify(theme));
  }, { ...legacyDefault, chosen: true });
  await page.goto(BASE_URL);
  await expect(root(page)).toHaveAttribute('data-theme-id', 'aero-chrome-heart');
});

test('S5, S6: a saved choice wins in dark mode, at first paint and after the server answers, and is not rewritten', async ({ page }) => {
  let writes = 0;
  await signedIn(page, aeroTheme);
  await page.route('**/api/preferences/theme', (route) => {
    if (route.request().method() !== 'GET') writes += 1;
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ theme: aeroTheme }) });
  });
  await page.emulateMedia({ colorScheme: 'dark' });
  await page.addInitScript((theme) => {
    window.localStorage.setItem('choir.theme.boot.v2', JSON.stringify(theme));
  }, aeroTheme);
  await page.goto(BASE_URL);
  await expect(root(page)).toHaveAttribute('data-theme-id', 'aero-chrome-heart');
  await expect(root(page)).toHaveAttribute('data-auth-state', 'signed_in');
  await expect(root(page)).toHaveAttribute('data-theme-id', 'aero-chrome-heart');
  await page.emulateMedia({ colorScheme: 'light' });
  await page.emulateMedia({ colorScheme: 'dark' });
  await expect(root(page)).toHaveAttribute('data-theme-id', 'aero-chrome-heart');
  expect(writes).toBe(0);
});
