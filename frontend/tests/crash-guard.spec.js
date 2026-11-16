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
  page.evaluate((key) => JSON.parse(localStorage.getItem(key) || '{}').pending || {}, KEY);

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
  await expect(held).toContainText('was opening when the page stopped last time');
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
  // Proven after about 8 s of the page being alive and visible.
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
