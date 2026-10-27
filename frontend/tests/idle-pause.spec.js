/**
 * Idle pause: after the idle window the desktop sets data-idle on the root and
 * every CSS animation stops (docs/problems/idle-tab-cpu-after-the-field-fix-2026-10-27.md).
 * Mocked backend; run against `vite preview` on :4173. The idle window is
 * shortened with localStorage `choir.idleMs`.
 */
import { test, expect } from '@playwright/test';

function win(appId, title, g, i) {
  return { window_id: 'w-' + appId, app_id: appId, title, geometry: g, mode: 'normal', z_index: 5 + i, app_context: { windowTitle: title } };
}
const G = [
  { x: 60, y: 40, width: 640, height: 480 },
  { x: 300, y: 120, width: 640, height: 480 },
  { x: 540, y: 200, width: 640, height: 480 },
];
const WINDOWS = [win('files', 'Files', G[0], 0), win('settings', 'Settings', G[1], 1), win('slides', 'Slides', G[2], 2)];

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
      if (req.method() === 'GET') return json({ owner_id: 'user-1', windows: WINDOWS, active_window_id: 'w-slides', icon_positions: {} });
      return json({ ok: true, updated_at: new Date().toISOString() });
    }
    if (p === '/api/preferences/theme') return json({ theme: { schema_version: 2, id: 'aero-chrome-heart' } });
    if (p === '/api/media/recents') return json({ items: [] });
    if (p === '/api/files' || p === '/api/files/') return json([{ name: 'notes.md', type: 'file', size: 10 }]);
    if (p === '/api/shell/bootstrap') return json({ ok: true, computer_id: 'computer-demo' });
    return json({ error: 'not mocked' }, 404);
  });
}

const running = (page) =>
  page.evaluate(
    () =>
      document
        .getAnimations()
        .filter((a) => a.playState === 'running' && a.effect && a.effect.getComputedTiming().iterations === Infinity).length,
  );

test('idle pauses every infinite animation and input resumes them', async ({ browser }) => {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'no-preference' });
  await context.addInitScript(() => localStorage.setItem('choir.idleMs', '1500'));
  await mock(context);
  const page = await context.newPage();
  await page.routeWebSocket(/\/api\/ws/, (ws) => ws.send(JSON.stringify({ type: 'connected' })));
  await page.goto('/', { waitUntil: 'domcontentloaded' });
  await page.locator('[data-desktop-ready="true"]').first().waitFor({ timeout: 30000 });

  // Active: windows are open, so infinite animations run and the root is not idle.
  await expect.poll(() => running(page), { timeout: 5000 }).toBeGreaterThan(0);

  // Idle: no input for the shortened window.
  await expect(page.locator('html')).toHaveAttribute('data-idle', /.*/, { timeout: 10000 });
  await expect.poll(() => running(page)).toBe(0);

  // Input resumes.
  await page.mouse.move(400, 400);
  await page.mouse.move(420, 410);
  await expect(page.locator('html')).not.toHaveAttribute('data-idle', /.*/);
  await expect.poll(() => running(page)).toBeGreaterThan(0);

  // And it idles again.
  await expect(page.locator('html')).toHaveAttribute('data-idle', /.*/, { timeout: 10000 });
  await context.close();
});
