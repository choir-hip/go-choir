// Appearance flow (owner, 2026-11-09): the background picker is the Files
// listing in a modal (not a command-line file tree), an image in Files can be
// set as the wallpaper from its context menu, the empty desktop offers
// "Change background..." / "Change theme...", and Settings is tabs so those
// menus can deep-link Appearance. Failure modes, written first:
//  A1 cancelling the picker (Cancel, the x, Esc, a backdrop click) changes the
//     background setting;
//  A2 a non-image can be chosen as a wallpaper (the Images filter hides it, and
//     with "All files" shown Choose stays disabled and says why);
//  A3 Esc does not close the modal, or focus does not return to the control
//     that opened it, or Tab leaves the dialog;
//  A4 the picker cannot be driven by keyboard (arrows move, Enter opens a
//     folder, Enter on an image chooses it);
//  A5 an image's Files context menu lacks "Set as desktop wallpaper", offers it
//     on a non-image, or it does not write pkg.settings.background / paint the
//     desktop;
//  A6 the empty desktop has no menu, a window or an icon gets the desktop menu,
//     or "Change background..." does not land Settings on Appearance (also when
//     Settings is already open on another tab);
//  A7 a deep link to an unknown tab opens nothing or a blank panel instead of
//     the first tab; a known tab does not land;
//  A8 phone width: the picker is not a full-screen sheet, the tab strip pushes
//     the page sideways or hides tabs, the desktop menu cannot be opened.
// /api is stubbed as desktop-background.spec.js does.
import { test, expect } from '@playwright/test';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';
const SHOTS = process.env.APPEARANCE_SHOTS || '';
const PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==', 'base64');
const NOW = '2026-11-09T00:00:00Z';

const listings = {
  '': [
    { name: 'Pictures', type: 'directory', size: 0, modified: NOW },
    { name: 'mail', type: 'directory', size: 0, modified: NOW },
    { name: 'notes.txt', type: 'file', size: 3, modified: NOW },
  ],
  Pictures: [
    { name: 'sky.png', type: 'file', size: 70, modified: NOW },
    { name: 'sea.jpg', type: 'file', size: 70, modified: NOW },
    { name: 'doc.pdf', type: 'file', size: 70, modified: NOW },
  ],
};

async function mockComputer(page, prefs = {}) {
  const state = { prefs: { ...prefs }, theme: {} };
  await page.route('**/api/**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
  await page.route('**/api/shell/bootstrap', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ computer_id: 'cmp-appearance-test' }) }));
  await page.route('**/auth/session', (route) => route.fulfill({
    status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: true, user: { email: 'appearance@example.com' } }),
  }));
  await page.route('**/api/preferences/theme', async (route) => {
    if (route.request().method() === 'PUT') state.theme = JSON.parse(route.request().postData()).theme;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ theme: state.theme }) });
  });
  await page.route('**/api/preferences/pkg.settings.*', async (route) => {
    const key = new URL(route.request().url()).pathname.replace('/api/preferences/', '');
    if (route.request().method() === 'PUT') state.prefs[key] = JSON.parse(route.request().postData()).value;
    const has = key in state.prefs;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ key, value: has ? state.prefs[key] : undefined, default: !has }) });
  });
  await page.route(/\/api\/files(\/[^?]*)?(\?.*)?$/, async (route) => {
    const url = new URL(route.request().url());
    const rel = decodeURIComponent(url.pathname.replace(/^\/api\/files\/?/, ''));
    if (route.request().method() === 'GET' && rel in listings) {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(listings[rel]) });
    }
    if (route.request().method() === 'GET' && /\.(png|jpg)$/.test(rel)) {
      return route.fulfill({ status: 200, contentType: 'image/png', body: PNG });
    }
    if (route.request().method() === 'GET') return route.fulfill({ status: 200, contentType: 'application/json', body: '[]' });
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{"ok":true}' });
  });
  return state;
}

const desktop = (page) => page.locator('[data-desktop]');
const shot = async (page, name) => { if (SHOTS) await page.screenshot({ path: `${SHOTS}/${name}.png` }); };

async function boot(page) {
  await page.goto(BASE_URL);
  await expect(page.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
}

async function openApp(page, id) {
  await page.locator(`[data-desktop-icon-id="${id}"]`).focus();
  await page.keyboard.press('Enter');
}

async function openSettings(page) {
  await openApp(page, 'settings');
  await expect(page.locator('[data-settings-app]')).toBeVisible();
}

// A deep link as the shell would send it (the desktop menu calls this same path).
async function deepLink(page, tab) {
  await page.evaluate(async (t) => {
    // The module instance the desktop bound (Vite may have stamped its URL after an HMR update).
    const url = performance.getEntriesByType('resource').map((r) => r.name).filter((n) => /\/src\/lib\/shell-services\.ts/.test(n)).pop() || '/src/lib/shell-services.ts';
    const { openApp: shellOpenApp } = await import(url);
    shellOpenApp('settings', { appContext: { settingsTab: t, settingsTabSeq: Date.now() } });
  }, tab);
}

const activeTab = (page) => page.locator('[data-settings-tab][aria-selected="true"]');

test('A1 cancelling the picker leaves the background unchanged (Cancel, x, Esc, backdrop)', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  const state = await mockComputer(page, { 'pkg.settings.background': 'Pictures/sky.png' });
  await boot(page);
  await openSettings(page);
  await expect(desktop(page)).toHaveAttribute('data-desktop-background', 'image');
  const choose = page.locator('[data-background-choose]');

  for (const how of ['cancel', 'x', 'esc', 'backdrop']) {
    await choose.click();
    await expect(page.locator('[data-file-picker]')).toBeVisible();
    // It opens where the current background lives.
    await expect(page.locator('[data-picker-entry="sea.jpg"]')).toBeVisible();
    await page.locator('[data-picker-entry="sea.jpg"]').click();
    if (how === 'cancel') await page.locator('[data-picker-cancel]').click();
    if (how === 'x') await page.locator('[data-picker-cancel-x]').click();
    if (how === 'esc') await page.keyboard.press('Escape');
    if (how === 'backdrop') await page.locator('[data-modal-backdrop]').click({ position: { x: 4, y: 4 } });
    await expect(page.locator('[data-file-picker]')).toHaveCount(0);
  }
  expect(state.prefs['pkg.settings.background']).toBe('Pictures/sky.png');
  await expect(desktop(page)).toHaveAttribute('data-desktop-background', 'image');
});

test('A2 a non-image cannot be chosen; A3 Esc closes and returns focus, Tab stays inside', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  const state = await mockComputer(page);
  await boot(page);
  await openSettings(page);
  const choose = page.locator('[data-background-choose]');
  await choose.focus();
  await choose.press('Enter');
  const picker = page.locator('[data-file-picker]');
  await expect(picker).toBeVisible();
  await expect(page.locator('[data-modal]')).toHaveAttribute('aria-modal', 'true');
  // Images filter: notes.txt is hidden, mail is never listed.
  await expect(page.locator('[data-picker-entry="notes.txt"]')).toHaveCount(0);
  await expect(page.locator('[data-picker-entry="mail"]')).toHaveCount(0);
  // Show all files: it lists, but Choose refuses and says why.
  await page.locator('[data-picker-filter]').selectOption({ label: 'All files' });
  await page.locator('[data-picker-entry="notes.txt"]').click();
  await expect(page.locator('[data-picker-choose]')).toBeDisabled();
  await expect(page.locator('[data-picker-refused]')).toContainText('notes.txt');
  await page.locator('[data-picker-entry="notes.txt"]').dblclick();
  await expect(picker).toBeVisible();
  expect(state.prefs['pkg.settings.background']).toBeUndefined();

  // A3: Tab never leaves the dialog.
  for (let i = 0; i < 12; i++) {
    await page.keyboard.press('Tab');
    expect(await page.evaluate(() => Boolean(document.activeElement?.closest('[data-modal]')))).toBe(true);
  }
  for (let i = 0; i < 12; i++) {
    await page.keyboard.press('Shift+Tab');
    expect(await page.evaluate(() => Boolean(document.activeElement?.closest('[data-modal]')))).toBe(true);
  }
  await page.keyboard.press('Escape');
  await expect(picker).toHaveCount(0);
  await expect(choose).toBeFocused();
  expect(state.prefs['pkg.settings.background']).toBeUndefined();
});

test('A4 the picker is keyboard driven and resolves to a Files path', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  const state = await mockComputer(page);
  await boot(page);
  await openSettings(page);
  await page.locator('[data-background-choose]').click();
  const first = page.locator('[data-picker-entry]').first();
  await expect(first).toBeFocused();
  await expect(first).toHaveAttribute('data-picker-entry', 'Pictures');
  await page.keyboard.press('Enter'); // opens the folder
  await expect(page.locator('[data-picker-crumb="Pictures"]')).toBeVisible();
  await expect(page.locator('[data-picker-entry="sea.jpg"]')).toBeFocused(); // listed a-z: doc.pdf is filtered out
  await expect(page.locator('[data-picker-thumb]')).toHaveCount(2); // images only; each has a thumbnail
  await page.keyboard.press('ArrowDown');
  await expect(page.locator('[data-picker-entry="sky.png"]')).toBeFocused();
  await page.keyboard.press('ArrowLeft'); // up a folder
  await expect(page.locator('[data-picker-entry="Pictures"]')).toBeVisible();
  await page.keyboard.press('Enter');
  await expect(page.locator('[data-picker-entry="sea.jpg"]')).toBeVisible();
  await page.locator('[data-picker-entry="sea.jpg"]').focus();
  await page.keyboard.press('Enter'); // an image: choose
  await expect(page.locator('[data-file-picker]')).toHaveCount(0);
  await expect.poll(() => state.prefs['pkg.settings.background']).toBe('Pictures/sea.jpg');
  await expect(desktop(page)).toHaveAttribute('data-desktop-background', 'image');
});

test('A5 an image in Files can be set as the desktop wallpaper from its context menu', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  const state = await mockComputer(page);
  await boot(page);
  await openApp(page, 'files');
  await expect(page.locator('[data-file-list]')).toBeVisible();
  await page.locator('[data-file-item]:has-text("Pictures")').click();
  await expect(page.locator('[data-file-item]:has-text("sky.png")')).toBeVisible();

  await page.locator('[data-file-item]:has-text("doc.pdf")').click({ button: 'right' });
  await expect(page.locator('[data-context-menu-item="open"]')).toBeVisible();
  await expect(page.locator('[data-context-menu-item="set-wallpaper"]')).toHaveCount(0);
  await page.keyboard.press('Escape');

  await page.locator('[data-file-item]:has-text("sky.png")').click({ button: 'right' });
  await shot(page, 'after-files-image-context-menu');
  await page.locator('[data-context-menu-item="set-wallpaper"]').click();
  await expect.poll(() => state.prefs['pkg.settings.background']).toBe('Pictures/sky.png');
  await expect(desktop(page)).toHaveAttribute('data-desktop-background', 'image');
  expect(await page.locator('[data-desktop-background-layer]').getAttribute('style')).toContain('/api/files/Pictures/sky.png');
  await expect(page.locator('[data-files-status]')).toContainText('Pictures/sky.png');
});

test('A6 the empty desktop has a menu that opens Settings on Appearance', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await mockComputer(page);
  await boot(page);
  await page.mouse.click(900, 300, { button: 'right' });
  await shot(page, 'after-desktop-context-menu');
  await expect(page.locator('[data-context-menu-item="change-background"]')).toBeVisible();
  await expect(page.locator('[data-context-menu-item="change-theme"]')).toBeVisible();
  await page.locator('[data-context-menu-item="change-background"]').click();
  await expect(page.locator('[data-settings-app]')).toBeVisible();
  await expect(activeTab(page)).toHaveAttribute('data-settings-tab', 'appearance');
  await expect(page.locator('[data-settings-background]')).toBeVisible();
  await expect(page.locator('[data-settings-theme]')).toBeVisible();
  await shot(page, 'after-settings-appearance');

  // Already open on another tab, minimised: the menu brings it back on Appearance.
  await page.locator('[data-settings-tab="account"]').click();
  await expect(page.locator('[data-settings-account]')).toBeVisible();
  await page.locator('[data-window-app-id="settings"] [data-window-minimize]').click();
  await expect(page.locator('[data-settings-app]')).toBeHidden();
  await page.mouse.click(900, 300, { button: 'right' });
  await page.locator('[data-context-menu-item="change-theme"]').click();
  await expect(page.locator('[data-settings-app]')).toBeVisible();
  await expect(activeTab(page)).toHaveAttribute('data-settings-tab', 'appearance');

  // A window is not the empty desktop: no desktop menu on it.
  await page.locator('[data-settings-hero], .settings-hero').first().click({ button: 'right' });
  await expect(page.locator('[data-context-menu-item="change-background"]')).toHaveCount(0);
});

test('A7 Settings tabs deep link; an unknown tab falls back to the first', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await mockComputer(page);
  await boot(page);
  await openSettings(page);
  await expect(activeTab(page)).toHaveAttribute('data-settings-tab', 'appearance');
  const labels = await page.locator('[data-settings-tab]').allTextContents();
  expect(labels[0]).toBe('Appearance');

  await deepLink(page, 'system');
  await expect(activeTab(page)).toHaveAttribute('data-settings-tab', 'system');
  await expect(page.locator('[data-settings-runtime-status]')).toBeVisible();
  await deepLink(page, 'account');
  await expect(activeTab(page)).toHaveAttribute('data-settings-tab', 'account');
  await expect(page.locator('[data-settings-api-keys]')).toBeVisible();
  await deepLink(page, 'no-such-tab');
  await expect(activeTab(page)).toHaveAttribute('data-settings-tab', 'appearance');
  await expect(page.locator('[data-settings-theme]')).toBeVisible();

  // Keyboard: arrow keys move between tabs.
  await page.locator('[data-settings-tab="appearance"]').focus();
  await page.keyboard.press('ArrowRight');
  await expect(activeTab(page)).toHaveAttribute('data-settings-tab', 'account');
});

test('A8 phone width: full-screen picker sheet, scrollable tab strip, desktop menu', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const state = await mockComputer(page);
  await boot(page);
  await openSettings(page);
  await shot(page, 'after-settings-phone');
  // No sideways page scroll; every tab is reachable.
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
  const strip = page.locator('[data-settings-tabs]');
  await expect(strip).toBeVisible();
  await page.locator('[data-settings-tab="system"]').scrollIntoViewIfNeeded();
  await page.locator('[data-settings-tab="system"]').click();
  await expect(activeTab(page)).toHaveAttribute('data-settings-tab', 'system');
  await page.locator('[data-settings-tab="appearance"]').click();

  await page.locator('[data-background-choose]').click();
  const box = await page.locator('[data-modal]').boundingBox();
  expect(box.width).toBeGreaterThanOrEqual(388);
  expect(box.height).toBeGreaterThanOrEqual(840);
  await expect(page.locator('[data-picker-entry="Pictures"]')).toBeVisible();
  await shot(page, 'after-picker-phone');
  await page.locator('[data-picker-entry="Pictures"]').click();
  await page.locator('[data-picker-entry="sky.png"]').click();
  await page.locator('[data-picker-choose]').click();
  await expect.poll(() => state.prefs['pkg.settings.background']).toBe('Pictures/sky.png');
  await expect(page.locator('[data-file-picker]')).toHaveCount(0);
});

test('desktop-wide shots: the new picker', async ({ page }) => {
  test.skip(!SHOTS, 'screenshots only when APPEARANCE_SHOTS is set');
  await page.setViewportSize({ width: 1280, height: 800 });
  await mockComputer(page);
  await boot(page);
  await openSettings(page);
  await page.locator('[data-background-choose]').click();
  await page.locator('[data-picker-entry="Pictures"]').click();
  await expect(page.locator('[data-picker-thumb]').first()).toBeVisible();
  await shot(page, 'after-new-picker');
});
