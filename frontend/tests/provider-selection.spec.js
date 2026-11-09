// Selection records and "Who provides this?", slice L2
// (lib/services/provider-selection.ts; plan section 11, Library row;
// docs/design/modular-architecture-2026-11-04.md section 2.4 and decisions 4
// and 11). Two parts:
//
//  - the service on the real kernel, loaded through vite (no browser): two
//    providers of a slot, the owner's choice outranks priority, a refused
//    write changes nothing, the choice is re-read after a sign-out and
//    sign-in, "Use default" selects the base without removing the other;
//  - the drill in the shell (vite dev server on 5173): a second launcher
//    provider takes the slot by priority; Settings, "Who provides this?",
//    shows it, "Use default" puts the built-in launcher back with the other
//    still installed, and after a reload (the computer's record read again at
//    sign-in) the choice still holds against the higher-priority provider.
//
// Failure modes (provider-selection.ts P1 to P5), written before the code:
//  P1 a record naming a package that is not loaded breaks the ranking: it must
//     be passed over, and the choice must return with the package;
//  P2 a failed read or a value this shell cannot read throws or blanks a slot;
//  P3 a write the server refuses is applied anyway (kernel and record disagree);
//  P4 (another window's write) is a live event; covered by the Go route test
//     emitting setting.updated and the listener here reading the same value;
//  P5 a sign-out leaves the previous owner's choice ranked;
//  W1 "Use default" uninstalls or unregisters the other provider;
//  W2 the choice does not survive a reload (it lived only in the page).
//
//   cd frontend && npx vite --port 5173 &
//   npx playwright test tests/provider-selection.spec.js
import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { stubBackend } from '../scripts/package-contracts.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';
const KEY_PATH = '/api/preferences/packages.selection';

test.describe('selection records on the kernel', () => {
  let vite; let S; let K; let P;
  let stored = {};
  let refuse = false;
  const puts = [];
  const realFetch = globalThis.fetch;
  test.beforeAll(async () => {
    globalThis.fetch = async (url, init = {}) => {
      const p = new URL(String(url), 'http://localhost').pathname;
      if (p !== KEY_PATH) return new Response('{"error":"not_stubbed"}', { status: 404 });
      if ((init.method || 'GET') === 'PUT') {
        if (refuse) return new Response('{"error":"selections refused"}', { status: 400 });
        const body = JSON.parse(String(init.body));
        puts.push(body.value);
        stored = body.value;
        return new Response(JSON.stringify({ key: 'packages.selection', value: stored }), { status: 200 });
      }
      return new Response(JSON.stringify({ key: 'packages.selection', value: stored, default: Object.keys(stored).length === 0 }), { status: 200 });
    };
    vite = await createServer({ root, cacheDir: fs.mkdtempSync(path.join(os.tmpdir(), 'choir-selection-vite-')), server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
    S = await vite.ssrLoadModule('/src/lib/shell-slots.ts');
    K = await vite.ssrLoadModule('/src/lib/shell-kernel.ts');
    P = await vite.ssrLoadModule('/src/lib/services/provider-selection.ts');
    await K.shellKernel.services.settled();
  });
  test.afterAll(async () => {
    globalThis.fetch = realFetch;
    await vite?.close();
  });

  const row = async (slot) => (await P.providerSlots()).find((r) => r.slot === slot);

  test('the choice outranks priority, a refused write changes nothing, a sign-in reads it back, Use default keeps the other', async () => {
    const scope = K.shellKernel.scope('selection-test');
    const Fake = function Fake() {};
    S.registerSurface(scope, 'launcher', { id: 'launcher-alt', component: Fake, priority: 20 });
    let r = await row('launcher');
    expect(r.active).toBe('launcher-alt'); // priority, with no record
    expect(r.defaultPackage).toBe('launcher');
    expect(r.providers.map((p) => p.id).sort()).toEqual(['launcher', 'launcher-alt']);

    // P3: refused by the server, nothing applied.
    refuse = true;
    const bad = await P.useDefaultProvider('launcher');
    expect(bad).toEqual({ ok: false, error: 'selections refused' });
    expect((await row('launcher')).active).toBe('launcher-alt');
    refuse = false;

    // "Use default": the base is selected and drawn; the other stays registered (W1).
    expect(await P.useDefaultProvider('launcher')).toEqual({ ok: true });
    r = await row('launcher');
    expect(r.active).toBe('launcher');
    expect(r.selected).toBe('launcher');
    expect(r.providers.map((p) => p.id)).toContain('launcher-alt');
    expect(puts.at(-1).launcher.package).toBe('launcher');
    expect(puts.at(-1).launcher.at).toMatch(/^\d{4}-\d\d-\d\dT/);

    // P5 then W2: a sign-out withdraws the choice; the next sign-in reads the record again.
    await P.clearProviderSelections();
    expect((await row('launcher')).active).toBe('launcher-alt');
    await P.loadProviderSelections();
    expect((await row('launcher')).active).toBe('launcher');

    // Choosing the other: recorded, drawn.
    expect(await P.selectProvider('launcher', 'launcher-alt')).toEqual({ ok: true });
    expect((await row('launcher')).active).toBe('launcher-alt');
    // Bad input is refused before any write.
    expect((await P.selectProvider('windows', 'launcher')).ok).toBe(false);
    expect((await P.selectProvider('launcher', 'last-resort')).ok).toBe(false);

    // P1: the chosen package leaves; the slot falls back; the record stays and the choice returns with it.
    await scope.dispose();
    r = await row('launcher');
    expect(r.active).toBe('launcher');
    expect(r.selected).toBe('launcher-alt');
    const again = K.shellKernel.scope('selection-test-2');
    S.registerSurface(again, 'launcher', { id: 'launcher-alt', component: Fake, priority: 0 });
    expect((await row('launcher')).active).toBe('launcher-alt');
    await again.dispose();

    // P2: a value this shell cannot read is no selection, never a throw.
    stored = { launcher: { package: 'Not A Name', at: 'x' }, windows: { package: 'x' } };
    await P.loadProviderSelections();
    r = await row('launcher');
    expect(r.selected).toBeNull();
    expect(r.active).toBe('launcher');
    await P.clearProviderSelections();
  });
});

test.describe('Who provides this? in the shell', () => {
  // The shell's own instance of a module: the URL the page loaded it by (a dev server may add ?t=).
  const inShell = (page, file, fn, arg) => page.evaluate(async ({ file, fn, arg }) => {
    const url = performance.getEntriesByType('resource').map((e) => e.name).find((n) => new URL(n).pathname === file) || file;
    const mod = await import(/* @vite-ignore */ url);
    // eslint-disable-next-line no-new-func
    return new Function('mod', 'arg', `return (${fn})(mod, arg);`)(mod, arg);
  }, { file, fn, arg });

  // A second launcher, from the same component, at a priority that outranks the built-in one.
  const addAltLauncher = (page) => inShell(page, '/src/lib/shell-slots.ts', `async (slots) => {
    const comp = (await import('/src/packages/launcher/Launcher.surface.svelte')).default;
    slots.registerSurface(slots.surfaceScope('launcher-alt'), 'launcher', { id: 'launcher-alt', component: comp, priority: 20 });
  }`);
  const activeLauncher = (page) => inShell(page, '/src/lib/services/provider-selection.ts', `async (m) => (await m.providerSlots()).find((r) => r.slot === 'launcher')`);

  async function openShell(page, record) {
    await stubBackend(page, {});
    await page.route((url) => url.pathname === KEY_PATH, async (route) => {
      const req = route.request();
      if (req.method() === 'PUT') record.value = JSON.parse(req.postData() || '{}').value;
      await route.fulfill({ json: { key: 'packages.selection', value: record.value, default: Object.keys(record.value).length === 0 } });
    });
    await page.goto(BASE_URL);
    await page.locator('[data-desktop][data-desktop-ready="true"]').waitFor();
  }

  async function openSettings(page) {
    await page.locator('[data-desk-menu-button]').first().click();
    await page.locator('[data-desk-sheet-app][data-desk-app-id="settings"]').first().click();
    const content = page.locator('[data-window][data-window-app-id="settings"] [data-window-content]').first();
    await content.locator('[data-settings-tab="packages"]').click(); // Settings is tabs; "Who provides this?" is on Packages
    await content.locator('[data-who-provides]').waitFor();
    return content;
  }

  test('a second provider takes the slot; Use default puts the built-in one back without removing it; the choice holds after a reload', async ({ page }) => {
    test.setTimeout(90_000);
    const record = { value: {} };
    await openShell(page, record);
    await addAltLauncher(page);
    await expect.poll(async () => (await activeLauncher(page)).active).toBe('launcher-alt');

    const content = await openSettings(page);
    const launcher = content.locator('[data-who-slot="launcher"]');
    await expect(launcher.locator('[data-who-active]')).toHaveText('Drawn by launcher-alt');
    await expect(launcher.locator('[data-who-choose]')).toHaveText(['launcher (built in)', 'launcher-alt']);
    await expect(content.locator('[data-who-slot]')).toHaveCount(6);

    await launcher.locator('[data-who-use-default="launcher"]').click();
    await expect(launcher.locator('[data-who-active]')).toHaveText('Drawn by launcher (built in)');
    await expect(launcher.locator('[data-who-selected="launcher"]')).toBeVisible();
    await expect(launcher.locator('[data-who-choose="launcher-alt"]')).toBeVisible(); // still installed (W1)
    expect(record.value.launcher.package).toBe('launcher');

    await launcher.locator('[data-who-choose="launcher-alt"]').click();
    await expect(launcher.locator('[data-who-active]')).toHaveText('Drawn by launcher-alt');
    expect(record.value.launcher.package).toBe('launcher-alt');
    await launcher.locator('[data-who-use-default="launcher"]').click();
    await expect(launcher.locator('[data-who-active]')).toHaveText('Drawn by launcher (built in)');

    // The reload: the page forgets everything; the record on the computer does not (W2).
    await page.reload();
    await page.locator('[data-desktop][data-desktop-ready="true"]').waitFor();
    await expect.poll(async () => (await activeLauncher(page)).selected).toBe('launcher');
    await addAltLauncher(page);
    const after = await activeLauncher(page);
    expect(after.providers.map((p) => p.id).sort()).toEqual(['launcher', 'launcher-alt']);
    expect(after.active).toBe('launcher'); // the choice, not the priority
    await page.locator('[data-desk-menu-button]').first().click();
    await expect(page.locator('[data-shell-slot="launcher"]')).toHaveAttribute('data-shell-slot-provider', 'launcher');
  });
});
