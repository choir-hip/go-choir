// Context-menu coverage sweep, slice MA5.2 (lib/context-coverage.ts and each
// surface's own use:contextMenu). Every registered surface shows a menu on a
// right-click: the desktop, a desktop icon, a launcher tile, a window's title
// bar, an overview card, a text field and every shell slot. The "essential"
// items (window close and minimize) stay.
//
// Failure modes:
//  M1 a surface a right-click lands on shows the browser's menu, or nothing;
//  M3 an unclaimed right-click opens a second menu on top of a surface's own;
//  M4 Shift+right-click loses the browser's menu.
//
//   cd frontend && npx vite --port 5173 &
//   npx playwright test tests/context-menu-sweep.spec.js
import { test, expect } from '@playwright/test';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';

async function openShell(page) {
  await page.route(/\/(auth|api)\//, (route) => route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"unauthenticated"}' }));
  await page.goto(BASE_URL);
  await expect(page.locator('[data-desk-menu-button]')).toBeVisible();
}

const menu = (page) => page.locator('[data-context-menu]').first();
const itemIds = (page) => page.locator('[data-context-menu-item]').evaluateAll((els) => els.map((e) => e.getAttribute('data-context-menu-item')));

async function expectMenuAt(page, locator, opts = {}) {
  await locator.click({ button: 'right', position: opts.position });
  await expect(menu(page)).toBeVisible();
  await expect(page.locator('[data-context-menu]')).toHaveCount(1); // M3
  const ids = await itemIds(page);
  expect(ids.length).toBeGreaterThan(0); // M1
  await page.keyboard.press('Escape');
  await expect(page.locator('[data-context-menu]')).toHaveCount(0);
  return ids;
}

async function launchFiles(page) {
  await page.locator('[data-desk-menu-button]').click();
  await page.locator('[data-desk-sheet-app][data-desk-app-id="files"]').click();
  const win = page.locator('[data-window][data-window-app-id="files"]');
  await expect(win).toBeVisible();
  return win;
}

test('the desktop background shows the desktop menu', async ({ page }) => {
  await openShell(page);
  const ids = await expectMenuAt(page, page.locator('[data-desktop-windows]'), { position: { x: 600, y: 400 } });
  expect(ids).toEqual(expect.arrayContaining(['show-desktop', 'overview', 'launcher']));
});

test('a desktop icon, a launcher tile and an overview card each show a menu', async ({ page }) => {
  await openShell(page);
  await expectMenuAt(page, page.locator('[data-desktop-icon]').first());

  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  const tileIds = await expectMenuAt(page, page.locator('[data-desk-sheet-app]').first());
  expect(tileIds).toContain('open');
  await page.keyboard.press('Escape');

  await launchFiles(page);
  await page.locator('[data-desk-menu-button]').click();
  await page.locator('[data-desk-overview]').click();
  await expect(page.locator('[data-desktop-overview]')).toBeVisible();
  const cardIds = await expectMenuAt(page, page.locator('[data-overview-card]').first());
  expect(cardIds).toEqual(expect.arrayContaining(['focus', 'minimize', 'close']));
});

test('a window title bar shows a menu and the essential items stay', async ({ page }) => {
  await openShell(page);
  const win = await launchFiles(page);
  const ids = await expectMenuAt(page, win.locator('[data-window-titlebar]'), { position: { x: 80, y: 10 } });
  expect(ids).toEqual(expect.arrayContaining(['close', 'minimize']));
});

test('a text field shows the text menu, and Shift+right-click keeps the browser menu (M4)', async ({ page }) => {
  await openShell(page);
  const field = page.locator('textarea, input[type="text"], input:not([type])').first();
  await expect(field).toBeVisible();
  await field.fill('hello');
  const ids = await expectMenuAt(page, field);
  expect(ids).toEqual(expect.arrayContaining(['copy', 'select-all']));
  const prevented = await page.evaluate(() => {
    const el = document.querySelector('[data-desktop-windows]');
    const ev = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, shiftKey: true, clientX: 500, clientY: 300 });
    el.dispatchEvent(ev);
    return ev.defaultPrevented;
  });
  expect(prevented).toBe(false);
  await expect(page.locator('[data-context-menu]')).toHaveCount(0);
});

test('every shell slot drawn on screen answers a right-click with a menu (M1)', async ({ page }) => {
  await openShell(page);
  const slots = page.locator('[data-shell-slot]');
  const n = await slots.count();
  expect(n).toBeGreaterThan(0);
  for (let i = 0; i < n; i++) {
    const slot = slots.nth(i);
    const box = await slot.boundingBox();
    if (!box || box.width < 4 || box.height < 4) continue; // a slot with nothing drawn
    await page.mouse.click(box.x + Math.min(box.width / 2, 40), box.y + Math.min(box.height / 2, 20), { button: 'right' });
    await expect(menu(page), `slot ${await slot.getAttribute('data-shell-slot')}`).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.locator('[data-context-menu]')).toHaveCount(0);
  }
});
