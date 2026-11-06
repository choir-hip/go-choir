// The sandboxed frame for app UIs from other publishers (MA6.1;
// src/lib/apps/package-sandbox.ts).
//
// No server: the spec builds this tree's shell, serves it with the contract
// runner's stub backend, and lists two releases on a stranger's track:
//   1. a hostile module, written by hand so no build lint stands between it
//      and the frame, which tries every way out it can reach: the shell's DOM,
//      the cookie, storage, a direct fetch, another app's route (plain and
//      encoded), the session route, an SDK name outside its uses or outside
//      the bridge (by forging messages on the runtime's own port), another
//      app's identity header, and another window;
//   2. the fixture packages of tests/fixtures/sandboxed-packages through the
//      contract runner in the sandboxed mode.
//
// Failure modes (package-sandbox.ts F1-F8) checked here: F1 F2 F3 F4 F5.
// Artifact: <output dir>/package-sandbox-receipt.json and the hostile window's
// screenshot.
import { test, expect } from '@playwright/test';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildShell, checkPackages, loadPackages, serveDist, stubBackend, STUB_STRANGER } from '../scripts/package-contracts.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');
const sha256 = (s) => createHash('sha256').update(s).digest('hex');

const HOSTILE = `
export const abi = 'choir.app-frontend/module/v1';
export const requires_sdk = '^2.0';
export const sdk_imports = ['fetchWithRenewal', 'setAppContext'];
export function mount(target, ctx) {
  const out = {};
  const attempt = async (key, fn) => {
    try { out[key] = 'reached:' + String(await fn()); } catch (e) { out[key] = 'blocked:' + (e && e.name) + ':' + (e && e.message); }
  };
  (async () => {
    await attempt('parentDom', () => window.parent.document.querySelector('[data-shell-secret]').textContent);
    await attempt('topDom', () => window.top.document.body.innerHTML.length);
    await attempt('cookie', () => document.cookie);
    await attempt('storage', () => localStorage.getItem('choir'));
    await attempt('directFetch', async () => (await fetch('/auth/session', { credentials: 'include' })).status);
    await attempt('directFetchOtherHost', async () => (await fetch('https://example.com/?leak=1')).status);
    await attempt('otherRoute', async () => (await ctx.sdk.fetchWithRenewal('/api/app-packages/other-app/items')).status);
    await attempt('encodedRoute', async () => (await ctx.sdk.fetchWithRenewal('/api/app-packages/hostile/..%2Fother-app/items')).status);
    await attempt('dotRoute', async () => (await ctx.sdk.fetchWithRenewal('/api/app-packages/hostile/../other-app/items')).status);
    await attempt('sessionRoute', async () => (await ctx.sdk.fetchWithRenewal('/auth/session')).status);
    await attempt('filesRoute', async () => (await ctx.sdk.fetchWithRenewal('/api/files/')).status);
    out.sdkLogout = typeof ctx.sdk.logout;
    out.currentUser = JSON.stringify(ctx.props.currentUser === undefined ? 'absent' : ctx.props.currentUser);
    // Capture the runtime's port by patching postMessage, then forge calls on it.
    let port = null;
    const orig = MessagePort.prototype.postMessage;
    MessagePort.prototype.postMessage = function (...args) { port = this; return orig.apply(this, args); };
    const own = await ctx.sdk.fetchWithRenewal('/api/app-packages/hostile/items', { headers: { 'X-Choir-App': 'texture', Cookie: 'x=1' } });
    out.own = await own.text();
    MessagePort.prototype.postMessage = orig;
    const forged = (name, args) => new Promise((resolve) => {
      const id = 900000 + Math.floor(Math.random() * 1000);
      const prev = port.onmessage;
      port.onmessage = (e) => {
        if (e.data && e.data.t === 'reply' && e.data.id === id) { port.onmessage = prev; resolve(e.data.ok ? 'answered' : 'refused:' + e.data.error.message); return; }
        prev && prev(e);
      };
      port.postMessage({ t: 'call', id, name, args });
    });
    out.forgedLogout = port ? await forged('logout', []) : 'no port';
    out.forgedToast = port ? await forged('toast', ['pwned']) : 'no port';
    out.forgedOpenApp = port ? await forged('openApp', ['texture', {}]) : 'no port';
    window.parent.postMessage({ t: 'call', id: 1, name: 'logout' }, '*');
    await ctx.sdk.setAppContext({ windowId: 'someone-else', title: 'Hostile was here' });
    const pre = document.createElement('pre');
    pre.setAttribute('data-hostile', '');
    pre.textContent = JSON.stringify(out);
    target.appendChild(pre);
  })();
  return { destroy() {} };
}
`;

const HOSTILE_MANIFEST = {
  schema: 'choir.package/v2', name: 'hostile', version: '1.0.0',
  display: { title: 'Hostile', icon: 'H', description: 'A hostile fixture' },
  app: {
    entry: 'App.svelte', abi: 'choir.app-frontend/svelte-inline/v1', aliases: [], requires_sdk: '^2.0',
    launcher: { menu: true, desktopIcon: false, order: 991 }, window: { singleton: true, heavy: false },
    auth: { preview: 'private' }, theme: { surface: 'standard', shellDataAttr: 'data-hostile-window', contentClass: 'hostile-content' },
  },
  kinds: [], uses_kinds: [], contributes: { desks: [] }, capabilities: [],
};

test.describe.configure({ mode: 'serial' });

test.describe('MA6.1: a stranger\'s app runs in the sandboxed frame', () => {
  test.skip(!fs.existsSync(path.join(frontendRoot, 'node_modules/vite')), 'frontend/node_modules is not installed');
  let tmp;
  let server;
  const receipt = { hostile: null, contracts: null };

  test.beforeAll(async () => {
    test.setTimeout(180_000);
    tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-sandbox-'));
    server = await serveDist(await buildShell({ outDir: path.join(tmp, 'dist') }));
  });

  test.afterAll(async ({}, testInfo) => {
    fs.writeFileSync(testInfo.outputPath('package-sandbox-receipt.json'), JSON.stringify(receipt, null, 2));
    await server?.close();
    if (tmp) fs.rmSync(tmp, { recursive: true, force: true });
  });

  test('a hostile module cannot read the shell, the session or another app\'s routes (F1-F5)', async ({ page }, testInfo) => {
    const digest = sha256(HOSTILE);
    const record = {
      name: 'hostile', release_id: 'hostile-release', install_id: 'hostile-install', installed_at: '2026-11-06T00:00:00Z',
      track: STUB_STRANGER, isolation: 'frame', uses: ['fetchWithRenewal', 'setAppContext'], requires_sdk: '^2.0',
      manifest: HOSTILE_MANIFEST, module: { abi: 'choir.app-frontend/module/v1', sha256: digest, url: `/api/packages/modules/${digest}.js` }, available: true,
    };
    const unanswered = [];
    await stubBackend(page, { installed: [record], modules: new Map([[digest, HOSTILE]]), onUnanswered: (r) => unanswered.push(r) });
    const reached = [];
    await page.route(/\/api\/app-packages\//, (route) => {
      const req = route.request();
      reached.push({ path: new URL(req.url()).pathname, app: req.headers()['x-choir-app'] || '', cookie: req.headers().cookie || '' });
      return route.fulfill({ status: 200, contentType: 'text/plain', body: `identity=${req.headers()['x-choir-app'] || ''}` });
    });
    const outside = [];
    page.on('request', (req) => { if (!req.url().startsWith(server.base) && !req.url().startsWith('data:') && !req.url().startsWith('blob:')) outside.push(req.url()); });
    await page.goto(server.base);
    await page.locator('[data-desktop][data-desktop-ready="true"]').waitFor();
    await page.evaluate(() => {
      const s = document.createElement('div');
      s.setAttribute('data-shell-secret', '');
      s.textContent = 'shell secret';
      document.body.appendChild(s);
    });
    await page.locator('[data-desk-menu-button]').first().click();
    await page.locator('[data-desk-sheet-app][data-desk-app-id="hostile"]').first().click();
    const win = page.locator('[data-window][data-window-app-id="hostile"]');
    const frameHost = win.locator('[data-package-frame]');
    await expect(frameHost).toHaveAttribute('data-package-state', 'mounted');
    await expect(frameHost).toHaveAttribute('data-package-isolation', 'frame');
    await expect(win.locator('iframe[data-package-sandbox="hostile"]')).toHaveAttribute('sandbox', 'allow-scripts allow-forms');
    const pre = page.frameLocator('iframe[data-package-sandbox="hostile"]').locator('[data-hostile]');
    await expect(pre).toBeVisible({ timeout: 15_000 });
    const out = JSON.parse(await pre.innerText());
    receipt.hostile = { out, reached, outside, unanswered };
    await page.screenshot({ path: testInfo.outputPath('package-sandbox-hostile.png') });

    // F1: no shell DOM, no cookie, no storage, no network of its own.
    for (const k of ['parentDom', 'topDom', 'cookie', 'storage', 'directFetch', 'directFetchOtherHost']) expect(out[k], k).toMatch(/^blocked:/);
    expect(outside, 'the frame reached a host').toEqual([]);
    // F2: only its own routes, named by the shell.
    for (const k of ['otherRoute', 'encodedRoute', 'dotRoute', 'sessionRoute', 'filesRoute']) expect(out[k], k).toMatch(/^blocked:SandboxRefused:/);
    expect(out.own).toBe('identity=hostile');
    expect(reached).toEqual([{ path: '/api/app-packages/hostile/items', app: 'hostile', cookie: '' }]);
    // F3: names outside the bridge or its uses are refused, even forged on the port.
    expect(out.sdkLogout).toBe('undefined');
    expect(out.forgedLogout).toBe('refused:logout is not offered to a framed app');
    expect(out.forgedOpenApp).toBe('refused:openApp is not offered to a framed app');
    expect(out.forgedToast).toBe('refused:hostile does not use toast');
    // The signed-in person's identity does not cross.
    expect(out.currentUser).toBe('"absent"');
    // F4: setAppContext titles its own window, whatever id it names; the shell is still signed in.
    await expect(win).toContainText('Hostile was here');
    await expect(page.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
    await expect(page.locator('[data-shell-secret]')).toHaveText('shell secret');
  });

  test('a framed module whose uses the frame does not offer is refused before it runs (F5)', async ({ page }) => {
    const digest = sha256(HOSTILE);
    const record = {
      name: 'hostile', release_id: 'hostile-release', install_id: 'hostile-install', installed_at: '2026-11-06T00:00:00Z',
      track: STUB_STRANGER, isolation: 'frame', uses: ['fetchWithRenewal', 'logout', 'openContextMenu'], requires_sdk: '^2.0',
      manifest: HOSTILE_MANIFEST, module: { abi: 'choir.app-frontend/module/v1', sha256: digest, url: `/api/packages/modules/${digest}.js` }, available: true,
    };
    await stubBackend(page, { installed: [record], modules: new Map([[digest, HOSTILE]]) });
    await page.goto(server.base);
    await page.locator('[data-desktop][data-desktop-ready="true"]').waitFor();
    await page.locator('[data-desk-menu-button]').first().click();
    await page.locator('[data-desk-sheet-app][data-desk-app-id="hostile"]').first().click();
    const frameHost = page.locator('[data-window][data-window-app-id="hostile"] [data-package-frame]');
    await expect(frameHost).toHaveAttribute('data-package-state', 'refused');
    await expect(frameHost).toContainText('does not offer: logout, openContextMenu');
    await expect(page.locator('iframe[data-package-sandbox]')).toHaveCount(0);
  });

  test('the contract runner passes in the sandboxed mode', async ({ browser }) => {
    test.setTimeout(180_000);
    const packages = loadPackages(path.join(frontendRoot, 'tests/fixtures/sandboxed-packages'));
    expect(packages.length).toBeGreaterThan(0);
    const rows = await checkPackages({ browser, base: server.base, packages, modes: ['sandboxed'] });
    receipt.contracts = rows;
    for (const row of rows) expect(row.pass, `${row.name}: ${JSON.stringify(row.modes)} ${row.problems.join('; ')}`).toBe(true);
  });
});
