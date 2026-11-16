/**
 * Crash guard: a window that was opening when the page died comes back held
 * and is not mounted; Reopen mounts it, Close removes it; a normal reload
 * after the quiet period restores it normally
 * (docs/problems/desktop-restore-reopens-the-window-that-crashed-the-page-crash-loop-2026-11-16.md).
 * Mocked backend; run against `vite preview` on :4173. "Died while pending"
 * is simulated by leaving the pending marker in localStorage and reloading,
 * which is exactly what a page death leaves behind.
 */
import { test, expect } from '@playwright/test';

const WINDOWS = [
  { window_id: 'w-settings', app_id: 'settings', title: 'Settings', geometry: { x: 60, y: 40, width: 640, height: 480 }, mode: 'normal', z_index: 5, app_context: { windowTitle: 'Settings' } },
];
const KEY = 'choir.crashGuard.v1:user-1';

let stateStatus = 200;

async function mock(context) {
  await context.route('**/auth/**', (route) => {
    const p = new URL(route.request().url()).pathname;
    const body = p === '/auth/session'
      ? { authenticated: true, user: { id: 'user-1', user_id: 'user-1', email: 'x@example.com', display_name: 'X' } }
      : {};
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });
  await context.route('**/api/**', (route) => {
    const req = route.request();
    const p = new URL(req.url()).pathname;
    const json = (b, s = 200) => route.fulfill({ status: s, contentType: 'application/json', body: JSON.stringify(b) });
    if (p === '/api/desktop/state') {
      if (req.method() === 'GET' && stateStatus !== 200) return json({ error: 'down' }, stateStatus);
      if (req.method() === 'GET') return json({ owner_id: 'user-1', windows: WINDOWS, active_window_id: 'w-settings', icon_positions: {} });
      return json({ ok: true, updated_at: new Date().toISOString() });
    }
    if (p === '/api/preferences/theme') return json({ theme: { schema_version: 2, id: 'aero-chrome-heart' } });
    if (p === '/api/media/recents') return json({ items: [] });
    if (p === '/api/shell/bootstrap') return json({ ok: true, computer_id: 'computer-demo' });
    return json({ error: 'not mocked' }, 404);
  });
}

async function open(browser) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  await mock(context);
  const page = await context.newPage();
  await page.routeWebSocket(/\/api\/ws/, (ws) => ws.send(JSON.stringify({ type: 'connected' })));
  return { context, page };
}

async function ready(page) {
  await page.goto('/', { waitUntil: 'domcontentloaded' });
  await page.locator('[data-desktop-ready="true"]').first().waitFor({ timeout: 30000 });
}

const pendingFor = (page) =>
  page.evaluate((key) => {
    // Entries are keyed window|page; answer by window id.
    const pending = JSON.parse(localStorage.getItem(key) || '{}').pending || {};
    return Object.fromEntries(Object.values(pending).map((e) => [e.windowId, e]));
  }, KEY);

test('a window pending at the last death is held, reopened, and closed', async ({ browser }) => {
  const { context, page } = await open(browser);
  await ready(page);
  const window = page.locator('[data-window][data-window-id="w-settings"]');
  await expect(window).toBeVisible();
  // Mounting wrote the pending marker before the body existed.
  await expect.poll(() => pendingFor(page)).toHaveProperty('w-settings');

  // The page "dies": nothing clears the marker; the tab is reloaded.
  await page.reload({ waitUntil: 'domcontentloaded' });
  await page.locator('[data-desktop-ready="true"]').first().waitFor({ timeout: 30000 });
  const held = page.locator('[data-window][data-window-id="w-settings"] [data-crash-held]');
  await expect(held).toBeVisible();
  await expect(held).toContainText('was still opening when the page was reloaded or stopped');
  await expect(page.locator('[data-window][data-window-id="w-settings"] [data-suspended-app]')).toHaveCount(0);
  expect(await pendingFor(page)).toEqual({});

  // Reopen mounts the body and writes the marker again.
  await held.locator('[data-crash-held-reopen]').click();
  await expect(held).toHaveCount(0);
  await expect.poll(() => pendingFor(page)).toHaveProperty('w-settings');

  // A second death: held again, now with the count and Close first.
  await page.reload({ waitUntil: 'domcontentloaded' });
  await page.locator('[data-desktop-ready="true"]').first().waitFor({ timeout: 30000 });
  await expect(held).toHaveAttribute('data-crash-held-count', '2');
  await expect(held.locator('button').first()).toHaveAttribute('data-crash-held-close', '');

  // Close removes the window and the guard's memory of it.
  await held.locator('[data-crash-held-close]').click();
  await expect(page.locator('[data-window][data-window-id="w-settings"]')).toHaveCount(0);
  expect(await pendingFor(page)).toEqual({});
  await context.close();
});

test('a reload after the quiet period restores the window normally', async ({ browser }) => {
  const { context, page } = await open(browser);
  await ready(page);
  await expect(page.locator('[data-window][data-window-id="w-settings"]')).toBeVisible();
  // The marker was written first (an inert guard would pass the next line alone)...
  expect(Object.keys(await pendingFor(page))).not.toEqual([]);
  // ...and is proven away after about 8 s of the page being alive and visible.
  await expect.poll(() => pendingFor(page), { timeout: 20000 }).toEqual({});
  await page.reload({ waitUntil: 'domcontentloaded' });
  await page.locator('[data-desktop-ready="true"]').first().waitFor({ timeout: 30000 });
  await expect(page.locator('[data-window][data-window-id="w-settings"]')).toBeVisible();
  await expect(page.locator('[data-crash-held]')).toHaveCount(0);
  await context.close();
});

test('a pending marker from a tab that is gone holds the window in a brand-new tab', async ({ browser }) => {
  // The first context leaves a pending marker and is abandoned without a clean exit.
  const first = await open(browser);
  await ready(first.page);
  await expect.poll(() => pendingFor(first.page)).toHaveProperty('w-settings');
  const stored = await first.page.evaluate((key) => localStorage.getItem(key), KEY);
  await first.context.close();

  // A second context has new sessionStorage (a new tab id) and, here, its own localStorage seeded
  // with what the dead tab left behind.
  const second = await open(browser);
  await second.context.addInitScript(([key, value]) => {
    if (!localStorage.getItem(key)) localStorage.setItem(key, value);
  }, [KEY, stored]);
  await ready(second.page);
  const held = second.page.locator('[data-window][data-window-id="w-settings"] [data-crash-held]');
  await expect(held).toBeVisible();
  await second.context.close();
});

test('storage entirely unavailable: the desktop works as before, nothing held', async ({ browser }) => {
  const { context, page } = await open(browser);
  await context.addInitScript(() => {
    const boom = () => { throw new Error('storage blocked'); };
    for (const name of ['localStorage', 'sessionStorage']) {
      Object.defineProperty(window, name, { configurable: true, get: boom });
    }
  });
  const errors = [];
  page.on('pageerror', (err) => errors.push(String(err)));
  await ready(page);
  await expect(page.locator('[data-window][data-window-id="w-settings"]')).toBeVisible();
  await expect(page.locator('[data-crash-held]')).toHaveCount(0);
  expect(errors.filter((e) => /crash|storage blocked/i.test(e))).toEqual([]);
  await context.close();
});

test('the pending marker exists in storage before the app body root exists', async ({ browser }) => {
  const { context, page } = await open(browser);
  await context.addInitScript((key) => {
    window.__order = { markerAtBodyRoot: null };
    const check = () => {
      if (window.__order.markerAtBodyRoot !== null) return;
      if (!document.querySelector('[data-app-host][data-app-window-id="w-settings"]')) return;
      window.__order.markerAtBodyRoot = Object.keys(JSON.parse(localStorage.getItem(key) || '{}').pending || {});
    };
    new MutationObserver(check).observe(document, { childList: true, subtree: true });
  }, KEY);
  await ready(page);
  await expect(page.locator('[data-app-host][data-app-window-id="w-settings"]')).toBeAttached();
  const seen = await page.evaluate(() => window.__order.markerAtBodyRoot);
  expect(seen).not.toBeNull();
  expect(seen.some((k) => k.startsWith('w-settings|'))).toBe(true);
  await context.close();
});

test('a planned reload (account switched in another tab) is not a crash', async ({ browser }) => {
  const { context, page } = await open(browser);
  await ready(page);
  expect(Object.keys(await pendingFor(page))).not.toEqual([]);
  // Still inside the quiet period: an unplanned reload here would hold the window.
  await Promise.all([
    page.waitForEvent('framenavigated'),
    page.evaluate(() => new BroadcastChannel('choir-accounts').postMessage({ type: 'account-switched', from: 'another-tab', user_id: 'user-1' })),
  ]);
  await page.locator('[data-desktop-ready="true"]').first().waitFor({ timeout: 30000 });
  await expect(page.locator('[data-window][data-window-id="w-settings"]')).toBeVisible();
  await expect(page.locator('[data-crash-held]')).toHaveCount(0);
  await context.close();
});

test('a failed desktop-state fetch does not leave an unguarded desktop', async ({ browser }) => {
  stateStatus = 500;
  try {
    const { context, page } = await open(browser);
    await ready(page);
    await page.locator('[data-desktop-icon-id="settings"]').click();
    const body = page.locator('[data-app-host][data-app-id="settings"]');
    await expect(body).toBeAttached();
    const keys = Object.keys(await pendingFor(page));
    expect(keys.length).toBe(1);
    await context.close();
  } finally {
    stateStatus = 200;
  }
});
