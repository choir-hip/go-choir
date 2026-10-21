/**
 * Playwright tests for Web Lens, the launcher.
 *
 * Web Lens used to run a second browser inside the computer (Obscura, a
 * headless browser the guest daemon started as root) and an unsandboxed
 * iframe preview. It is now a launcher: it opens the original page in the
 * device's own browser, and reads a Source Viewer snapshot made by the
 * guarded import path
 * (docs/problems/obscura-untrusted-javascript-as-guest-root-2026-10-21.md).
 *
 * These specs run on the logged-out desktop, so they need only the static
 * frontend. The route-level guards matter more than the markup:
 *
 * - no request to /api/browser ever leaves the page;
 * - nothing in the launcher is an iframe;
 * - only http and https addresses become links, so javascript:, data: and
 *   file: addresses are refused;
 * - the link is a top-level navigation that cannot reach back to the
 *   desktop (noopener) or leak its address (noreferrer).
 */
import { test, expect } from './helpers/fixtures.js';
import { webLensAddress, storedSnapshotApplies } from '../src/lib/source-url.ts';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173';

function trackBrowserBackendRequests(page) {
  const requests = [];
  page.on('request', (request) => {
    const pathname = new URL(request.url()).pathname;
    if (pathname.startsWith('/api/browser')) requests.push(pathname);
  });
  return requests;
}

// The specs need only the static frontend. Answer the session probe the way a
// healthy server answers a signed-out browser (401, no retry) so the desktop
// does not wait on a backend that is not there.
async function routeSignedOutSession(page) {
  await page.route('**/auth/session', (route) =>
    route.fulfill({ status: 401, json: { error: 'not authenticated' } }),
  );
}

async function openWebLens(page) {
  await routeSignedOutSession(page);
  await page.goto(BASE_URL);
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
  await expect(page.locator('[data-desktop]')).toHaveAttribute('data-authenticated', 'false');
  await page.locator('[data-desk-menu-button]').click();
  await page.locator('[data-desk-sheet-app][data-desk-app-id="browser"]').click();
  const app = page.locator('[data-browser-app]').last();
  await expect(app).toBeVisible({ timeout: 5000 });
  return app;
}

async function submitAddress(app, value) {
  const input = app.locator('[data-browser-url-input]');
  await input.fill(value);
  await app.locator('[data-browser-go-btn]').click();
}

test('web lens is a launcher: no iframe, no browser history, no backend request', async ({ page }) => {
  const backendRequests = trackBrowserBackendRequests(page);
  const app = await openWebLens(page);

  await expect(app).toHaveAttribute('data-browser-launcher', '');
  await expect(app.locator('[data-browser-url-input]')).toBeVisible();
  await expect(app.locator('[data-browser-url-input]')).toBeEditable();
  await expect(app.locator('[data-browser-go-btn]')).toBeVisible();

  await submitAddress(app, 'https://example.com/some/page');
  await expect(app.locator('[data-browser-open-original]')).toBeVisible();

  await expect(app.locator('iframe')).toHaveCount(0);
  await expect(app.locator('[data-browser-iframe]')).toHaveCount(0);
  await expect(app.locator('[data-browser-nav-back]')).toHaveCount(0);
  await expect(app.locator('[data-browser-nav-forward]')).toHaveCount(0);
  await expect(app.locator('[data-browser-nav-reload]')).toHaveCount(0);
  await expect(app.locator('[data-browser-backend-status]')).toHaveCount(0);
  await expect(app.locator('[data-browser-backend-control]')).toHaveCount(0);
  expect(backendRequests).toEqual([]);
});

test('open original is a top-level link that cannot reach back or leak the desktop address', async ({ page }) => {
  const backendRequests = trackBrowserBackendRequests(page);
  const app = await openWebLens(page);

  await submitAddress(app, 'https://example.com/some/page?q=1');
  const link = app.locator('a[data-browser-open-original]');
  await expect(link).toBeVisible();
  await expect(link).toHaveAttribute('href', 'https://example.com/some/page?q=1');
  await expect(link).toHaveAttribute('target', '_blank');
  const rel = (await link.getAttribute('rel')) || '';
  expect(rel.split(/\s+/)).toEqual(expect.arrayContaining(['noopener', 'noreferrer']));
  expect(backendRequests).toEqual([]);
});

test('an address without a scheme gets https', async ({ page }) => {
  const app = await openWebLens(page);

  await submitAddress(app, 'example.com');
  await expect(app.locator('a[data-browser-open-original]')).toHaveAttribute('href', 'https://example.com');
});

for (const [label, address] of [
  ['javascript', 'javascript:alert(document.domain)'],
  ['data', 'data:text/html;charset=utf-8,%3Ch1%3Ex%3C%2Fh1%3E'],
  ['file', 'file:///etc/passwd'],
  ['blob', 'blob:https://example.com/00000000-0000-0000-0000-000000000000'],
  ['plain words', 'not an address'],
]) {
  test(`web lens refuses a ${label} address`, async ({ page }) => {
    const backendRequests = trackBrowserBackendRequests(page);
    const app = await openWebLens(page);

    await submitAddress(app, address);
    await expect(app.locator('[data-browser-error]')).toBeVisible();
    await expect(app.locator('[data-browser-open-original]')).toHaveCount(0);
    await expect(app.locator('[data-browser-open-snapshot]')).toHaveCount(0);
    await expect(app.locator('iframe')).toHaveCount(0);
    expect(backendRequests).toEqual([]);
  });
}

test('logged-out web lens offers the original but no snapshot, and calls no protected route', async ({ page }) => {
  const protectedRequests = [];
  page.on('request', (request) => {
    const pathname = new URL(request.url()).pathname;
    if (
      pathname.startsWith('/api/browser') ||
      pathname.startsWith('/api/content') ||
      pathname.startsWith('/api/trajectories') ||
      pathname.startsWith('/api/runs')
    ) {
      protectedRequests.push(pathname);
    }
  });
  const app = await openWebLens(page);

  await submitAddress(app, 'https://example.com');
  await expect(app.locator('[data-browser-open-original]')).toBeVisible();
  await expect(app.locator('[data-browser-open-snapshot]')).toHaveCount(0);
  expect(protectedRequests).toEqual([]);
});

test('web lens works in mobile focus mode', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 });
  const app = await openWebLens(page);

  await submitAddress(app, 'https://example.com');
  const link = app.locator('a[data-browser-open-original]');
  await expect(link).toBeVisible();
  const box = await link.boundingBox();
  expect(box).not.toBeNull();
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(375);
});

// ---------------------------------------------------------------
// The address check is one pure function; pin its table directly.
// ---------------------------------------------------------------
test('webLensAddress accepts only http and https addresses with a real host', () => {
  const accepted = [
    ['https://example.com', 'https://example.com'],
    ['  http://example.com/a?b=c#d  ', 'http://example.com/a?b=c#d'],
    ['HTTPS://EXAMPLE.COM/Path', 'HTTPS://EXAMPLE.COM/Path'],
    ['example.com', 'https://example.com'],
    ['example.com/path?q=1', 'https://example.com/path?q=1'],
    ['example.com:8080/path', 'https://example.com:8080/path'],
    ['http://localhost:3000/', 'http://localhost:3000/'],
  ];
  for (const [input, want] of accepted) {
    expect(webLensAddress(input), input).toBe(want);
  }

  const refused = [
    '',
    '   ',
    'javascript:alert(1)',
    'JaVaScRiPt:alert(1)',
    'javascript://example.com/%0aalert(1)',
    'data:text/html,<h1>x</h1>',
    'file:///etc/passwd',
    'blob:https://example.com/00000000-0000-0000-0000-000000000000',
    'vbscript:msgbox(1)',
    'mailto:someone@example.com',
    'ftp://example.com/file',
    'view-source:https://example.com',
    'about:blank',
    'https://example.com@evil.test/',
    'example.com@evil.test',
    'user:pass@example.com',
    'not an address',
    'localhost',
    'https://',
  ];
  for (const input of refused) {
    expect(webLensAddress(input), input).toBe('');
  }
  expect(webLensAddress(undefined)).toBe('');
  expect(webLensAddress(null)).toBe('');
  expect(webLensAddress({ toString: () => 'https://example.com' })).toBe('https://example.com');
});

// ---------------------------------------------------------------
// Signed in: the snapshot button opens the Source Viewer, which reads the
// address through /api/content/import-url (the guarded path research uses).
// The session and the computer are answered by routes, so no backend runs.
// ---------------------------------------------------------------
test('signed-in web lens opens the Source Viewer through the guarded import path', async ({ page }) => {
  const backendRequests = trackBrowserBackendRequests(page);
  const imports = [];

  // The last route registered wins, so the catch-all goes first.
  await page.route('**/api/**', (route) => route.fulfill({ json: {} }));
  await page.route('**/auth/session', (route) =>
    route.fulfill({ json: { authenticated: true, user: { id: 'user-1', email: 'owner@example.com' } } }),
  );
  await page.route('**/api/shell/bootstrap', (route) =>
    route.fulfill({ json: { computer_id: 'computer-1' } }),
  );
  await page.route('**/api/content/import-url', async (route) => {
    imports.push(route.request().postDataJSON());
    await route.fulfill({
      json: {
        content_id: 'content-1',
        title: 'Example page',
        source_url: 'https://example.com/read',
        media_type: 'text/markdown',
        app_hint: 'content',
        text_content: 'Snapshot text from the guarded import path.',
      },
    });
  });

  await page.goto(BASE_URL);
  await page
    .locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]')
    .waitFor({ state: 'visible', timeout: 30000 });
  await page.locator('[data-desk-menu-button]').click();
  await page.locator('[data-desk-sheet-app][data-desk-app-id="browser"]').click();
  const app = page.locator('[data-browser-app]').last();
  await expect(app).toBeVisible({ timeout: 5000 });

  await submitAddress(app, 'https://example.com/read');
  await expect(app.locator('[data-browser-open-original]')).toBeVisible();
  expect(imports).toEqual([]);

  await app.locator('[data-browser-open-snapshot]').click();
  const viewer = page.locator('[data-content-viewer]').last();
  await expect(viewer).toContainText('Snapshot text from the guarded import path.', { timeout: 10000 });
  expect(imports).toHaveLength(1);
  expect(imports[0].url).toBe('https://example.com/read');
  expect(backendRequests).toEqual([]);
});

// A stored reader snapshot is the immutable text of the address it was taken
// from. Changing the address must not let that text be imported under the new
// address (the Open in Texture import would relabel it with the wrong
// source_url, canonical_url and provenance).
test('a stored snapshot applies only while the address is the one it was taken from', () => {
  const original = 'https://example.com/original';
  expect(storedSnapshotApplies(original, original)).toBe(true);
  expect(storedSnapshotApplies(original, 'https://example.com/other')).toBe(false);
  expect(storedSnapshotApplies(original, '')).toBe(false);
  // A snapshot taken from no address belongs to no address that is typed.
  expect(storedSnapshotApplies('', '')).toBe(true);
  expect(storedSnapshotApplies('', 'https://example.com/typed')).toBe(false);
});
