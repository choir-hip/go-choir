// The desktop background is its own setting, separate from the theme (owner,
// 2026-11-09). Failure modes, written first:
//  B1 switching the theme changes or clears the background;
//  B2 switching the background changes the theme;
//  B3 "Theme colour" leaves the image painted;
//  B4 the choice is lost on reload;
//  B5 a path under mail/ (Mail-agent custody) is listed, or painted when stored;
//  B6 the image is inlined as a data URI instead of the Files read route;
//  B7 the picker is unusable at phone width.
// /api is mocked as other specs do; the preference route is a small stateful stub.
import { test, expect } from '@playwright/test';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';
const SHOTS = process.env.DESKTOP_BG_SHOTS || '';
// A 1x1 PNG.
const PNG = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==', 'base64');

const listings = {
  '': [
    { name: 'Pictures', type: 'directory', size: 0, modified: '2026-11-09T00:00:00Z' },
    { name: 'mail', type: 'directory', size: 0, modified: '2026-11-09T00:00:00Z' },
    { name: 'notes.txt', type: 'file', size: 3, modified: '2026-11-09T00:00:00Z' },
  ],
  Pictures: [
    { name: 'sky.png', type: 'file', size: 70, modified: '2026-11-09T00:00:00Z' },
    { name: 'doc.pdf', type: 'file', size: 70, modified: '2026-11-09T00:00:00Z' },
  ],
};

async function mockComputer(page, prefs = {}) {
  const state = { prefs: { ...prefs }, theme: {}, imageRequests: 0 };
  await page.route('**/api/**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
  await page.route('**/api/shell/bootstrap', (route) => route.fulfill({
    status: 200, contentType: 'application/json', body: JSON.stringify({ computer_id: 'cmp-bg-test' }),
  }));
  await page.route('**/auth/session', (route) => route.fulfill({
    status: 200, contentType: 'application/json',
    body: JSON.stringify({ authenticated: true, user: { email: 'bg@example.com' } }),
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
    if (route.request().method() === 'GET' && rel.endsWith('.png')) {
      state.imageRequests += 1;
      return route.fulfill({ status: 200, contentType: 'image/png', body: PNG });
    }
    if (route.request().method() === 'GET') return route.fulfill({ status: 200, contentType: 'application/json', body: '[]' });
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{"ok":true}' });
  });
  return state;
}

async function openSettings(page) {
  await page.goto(BASE_URL);
  await expect(page.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
  await page.locator('[data-desktop-icon-id="settings"]').focus();
  await page.keyboard.press('Enter');
  await expect(page.locator('[data-settings-background]')).toBeVisible();
}

async function pickSky(page) {
  await page.locator('[data-background-choose]').click();
  await expect(page.locator('[data-background-picker]')).toBeVisible();
  // B5: mail/ is never offered, nor non-images.
  await expect(page.locator('[data-background-dir="mail"]')).toHaveCount(0);
  await expect(page.locator('[data-background-file="notes.txt"]')).toHaveCount(0);
  await page.locator('[data-background-dir="Pictures"]').click();
  await expect(page.locator('[data-background-file="doc.pdf"]')).toHaveCount(0);
  await page.locator('[data-background-file="sky.png"]').click();
}

const desktop = (page) => page.locator('[data-desktop]');

for (const [label, size] of [['desktop', { width: 1280, height: 800 }], ['phone 390x844', { width: 390, height: 844 }]]) {
  test(`background lifecycle on ${label}`, async ({ page }) => {
    await page.setViewportSize(size);
    const state = await mockComputer(page);
    await openSettings(page);
    await expect(desktop(page)).toHaveAttribute('data-desktop-background', 'theme');

    await pickSky(page);
    await expect(desktop(page)).toHaveAttribute('data-desktop-background', 'image');
    const layer = page.locator('[data-desktop-background-layer]');
    const style = await layer.getAttribute('style');
    expect(style).toContain('/api/files/Pictures/sky.png');
    expect(style).not.toContain('data:'); // B6
    await expect.poll(() => state.imageRequests).toBeGreaterThan(0);
    expect(state.prefs['pkg.settings.background']).toBe('Pictures/sky.png');

    // B1/B2: a theme switch keeps the image; the background switch kept the theme.
    const themeBefore = await page.locator('[data-theme-id]').first().getAttribute('data-theme-id');
    const other = page.locator('[data-theme-preset]:not(.active)').first();
    const otherId = await other.getAttribute('data-theme-preset');
    expect(otherId).not.toBe(themeBefore);
    await other.click();
    await expect(page.locator('[data-theme-id]').first()).toHaveAttribute('data-theme-id', otherId);
    await expect(desktop(page)).toHaveAttribute('data-desktop-background', 'image');
    expect(await layer.getAttribute('style')).toContain('/api/files/Pictures/sky.png');
    expect(JSON.stringify(state.theme)).not.toContain('sky.png');

    // Fit options.
    await page.locator('[data-background-fit="tile"]').click();
    await expect(layer).toHaveCSS('background-repeat', 'repeat');
    await page.locator('[data-background-fit="contain"]').click();
    await expect(layer).toHaveCSS('background-size', 'contain');
    expect(state.prefs['pkg.settings.background_fit']).toBe('contain');

    if (SHOTS) await page.screenshot({ path: `${SHOTS}/background-${label.replace(/\W+/g, '-')}.png` });

    // B4: reload keeps both.
    await page.reload();
    await expect(desktop(page)).toHaveAttribute('data-desktop-background', 'image');
    await expect(page.locator('[data-theme-id]').first()).toHaveAttribute('data-theme-id', otherId);

    // B3: Theme colour clears the image, keeps the theme.
    await openSettings(page);
    await page.locator('[data-background-theme-colour]').click();
    await expect(desktop(page)).toHaveAttribute('data-desktop-background', 'theme');
    await expect(page.locator('[data-desktop-background-layer]')).toHaveCount(0);
    await expect(page.locator('[data-theme-id]').first()).toHaveAttribute('data-theme-id', otherId);
    expect(state.prefs['pkg.settings.background']).toBe('');
  });
}

test('B5: a stored path under mail/ is never painted', async ({ page }) => {
  await mockComputer(page, { 'pkg.settings.background': 'mail/inbox/photo.png' });
  await page.goto(BASE_URL);
  await expect(page.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
  await expect(desktop(page)).toHaveAttribute('data-desktop-background', 'theme');
});
