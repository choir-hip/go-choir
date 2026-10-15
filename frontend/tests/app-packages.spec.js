// S7-0a: the desktop loads an enabled app package's frontend module
// dynamically, beside the static APP_REGISTRY, and only after the bytes'
// SHA-256 matches the digest bound on the computer's tape
// (docs/design/s7-0-app-packages-2026-10-14.md §9, failure modes F30-F32).
//
// Runs against the Vite dev server with the computer API mocked; the guest
// side of the same contract is internal/autoputer/app_packages_test.go.
import { test, expect } from '@playwright/test';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';
const here = path.dirname(fileURLToPath(import.meta.url));
const HELLO_SOURCE = readFileSync(path.join(here, '..', '..', 'packages', 'hello', 'frontend', 'index.js'));
const BUILD_HEX = 'b'.repeat(64);
const ENTRY_URL = `/api/app-packages/builds/${BUILD_HEX}/frontend/index.js`;

function sha256(bytes) {
  return `sha256:${createHash('sha256').update(bytes).digest('hex')}`;
}

function helloRecord({ status = 'enabled', digest = sha256(HELLO_SOURCE) } = {}) {
  return {
    name: 'hello',
    version: '0.1.0',
    status,
    title: 'Hello',
    icon: '👋',
    description: 'S7-0a fixture package',
    manifest_digest: `sha256:${'a'.repeat(64)}`,
    build_digest: `sha256:${BUILD_HEX}`,
    data_store: 'package_kv/v1',
    available: true,
    window: { singleton: true, width: 520, height: 360 },
    frontend: { entry: 'frontend/index.js', abi: 'choir.app-frontend/v1', url: ENTRY_URL, sha256: digest },
  };
}

// mockComputer stands up an authenticated desktop whose computer has the
// given package records and serves `moduleBytes` for the hello entry.
async function mockComputer(page, { packages, moduleBytes = HELLO_SOURCE, desktopState = {} }) {
  const seen = { moduleFetches: 0, dataPuts: [] };
  await page.route('**/auth/**', async (route) => {
    const pathname = new URL(route.request().url()).pathname;
    const body = pathname === '/auth/session'
      ? { authenticated: true, user: { email: 'pkg-review@example.com' } }
      : { ok: true };
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });
  await page.route('**/health', (route) => route.fulfill({
    status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'ok' }),
  }));
  await page.route('**/api/**', async (route) => {
    const request = route.request();
    const pathname = new URL(request.url()).pathname;
    if (pathname === ENTRY_URL) {
      seen.moduleFetches += 1;
      await route.fulfill({ status: 200, contentType: 'text/javascript', body: moduleBytes });
      return;
    }
    if (pathname === '/api/app-packages') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ seq: 2, head: `sha256:${'c'.repeat(64)}`, packages }),
      });
      return;
    }
    if (pathname === '/api/app-packages/hello/data/count') {
      if (request.method() === 'PUT') {
        seen.dataPuts.push(JSON.parse(request.postData() || '{}'));
        await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
      } else {
        await route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify({ error: 'not found' }) });
      }
      return;
    }
    let body = {};
    if (pathname === '/api/preferences/theme') body = { theme: {} };
    else if (pathname === '/api/shell/bootstrap') body = { computer_id: 'pkg-test-computer' };
    else if (pathname === '/api/desktop/state') body = desktopState;
    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
  });
  return seen;
}

async function openFromDesk(page, appId) {
  await expect(page.locator('[data-prompt-input]')).toBeEnabled({ timeout: 15_000 });
  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  await page.locator(`[data-desk-sheet-app][data-desk-app-id="${appId}"]`).click();
}

test('loads an enabled package module by verified digest and writes its data', async ({ page }) => {
  const seen = await mockComputer(page, { packages: [helloRecord()] });
  await page.goto(BASE_URL);
  await openFromDesk(page, 'package:hello');

  const win = page.locator('[data-window][data-window-app-id="package:hello"]');
  await expect(win.locator('[data-hello-package]')).toBeVisible();
  await expect(win.locator('[data-hello-version]')).toContainText('hello 0.1.0');
  await expect(win.locator('[data-package-frame]')).toHaveAttribute('data-package-state', 'mounted');
  expect(seen.moduleFetches).toBe(1);

  await win.locator('[data-hello-increment]').click();
  await expect(win.locator('[data-hello-count]')).toHaveText('1');
  await expect.poll(() => seen.dataPuts.length).toBe(1);
  expect(seen.dataPuts[0]).toEqual({ value: 1 });

  // Static apps are untouched: the registry still opens Files.
  await openFromDesk(page, 'files');
  await expect(page.locator('[data-window][data-window-app-id="files"]')).toBeVisible();
});

test('refuses module bytes that do not match the digest bound on the tape', async ({ page }) => {
  const tampered = Buffer.from(`${HELLO_SOURCE.toString()}\n// tampered after the build\n`);
  const seen = await mockComputer(page, { packages: [helloRecord()], moduleBytes: tampered });
  await page.goto(BASE_URL);
  await openFromDesk(page, 'package:hello');

  const win = page.locator('[data-window][data-window-app-id="package:hello"]');
  await expect(win.getByRole('alert')).toContainText('digest');
  await expect(win.locator('[data-package-frame]')).toHaveAttribute('data-package-state', 'refused');
  await expect(page.locator('[data-hello-package]')).toHaveCount(0);
  expect(seen.moduleFetches).toBe(1);
});

test('contains a module without mount inside its window', async ({ page }) => {
  const broken = Buffer.from('export const notMount = 1;\n');
  await mockComputer(page, { packages: [helloRecord({ digest: sha256(broken) })], moduleBytes: broken });
  await page.goto(BASE_URL);
  await openFromDesk(page, 'package:hello');

  const win = page.locator('[data-window][data-window-app-id="package:hello"]');
  await expect(win.getByRole('alert')).toContainText('mount');
  await expect(win.locator('[data-package-frame]')).toHaveAttribute('data-package-state', 'failed');
  // The shell still works.
  await openFromDesk(page, 'files');
  await expect(page.locator('[data-window][data-window-app-id="files"]')).toBeVisible();
});

test('a disabled package is not offered and a restored window loads no code', async ({ page }) => {
  const seen = await mockComputer(page, {
    packages: [helloRecord({ status: 'disabled' })],
    desktopState: {
      windows: [{
        window_id: 'win-pkg-hello',
        app_id: 'package:hello',
        title: 'Hello',
        geometry: { x: 120, y: 90, width: 520, height: 360 },
        mode: 'normal',
        z_index: 3,
        app_context: {},
      }],
      active_window_id: 'win-pkg-hello',
    },
  });
  await page.goto(BASE_URL);
  await expect(page.locator('[data-prompt-input]')).toBeEnabled({ timeout: 15_000 });

  const win = page.locator('[data-window][data-window-app-id="package:hello"]');
  await expect(win.locator('[data-package-frame]')).toHaveAttribute('data-package-state', 'unavailable');
  await expect(win).toContainText('not enabled');

  await page.locator('[data-desk-menu-button]').click();
  await expect(page.locator('[data-desk-sheet]')).toBeVisible();
  await expect(page.locator('[data-desk-sheet-app][data-desk-app-id="package:hello"]')).toHaveCount(0);
  expect(seen.moduleFetches).toBe(0);
});
