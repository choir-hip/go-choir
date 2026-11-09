// Installed shell surfaces (plan section 11 surface mount, 12.1; Devin's
// verdict item 5): an installed package that provides a shell surface (here a
// context-menu renderer, menu-default's slot) replaces the in-tree package of
// that name on the owner's own track, with the L4 failover.
//
// The spec builds a fixture release with the computer's own module build
// (internal/pkgruntime/build-module.mjs, surface mode), serves it from
// page.route at the module route, and hands the shell an installed list, so
// the shell's real loader, verifier, slot registrar and watchdog run.
//
// Failure modes, written before the code:
//  I1 an installed menu is inert: the in-tree menu still draws;
//  I2 a release off the owner's own track (isolation "frame") mounts in the
//     shell realm, or ?recovery=core mounts an installed surface;
//  I3 an installed renderer that throws leaves the owner with no menu, or is
//     never set aside, or the in-tree menu does not come back;
//  I4 uninstall leaves the installed menu drawing;
//  I5 the shell cannot tell which release serves the slot (the state view).
//
//   cd frontend && npx vite --port 5173 &
//   npx playwright test tests/installed-surfaces.spec.js
import { test, expect } from '@playwright/test';
import { spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendRoot = path.resolve(__dirname, '..');
const repoRoot = path.resolve(frontendRoot, '..');
const runtimeDir = path.join(repoRoot, 'internal/pkgruntime');
const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';

const MANIFEST = {
  schema: 'choir.package/v2', name: 'menu-default', version: '1.1.0',
  display: { title: 'Menu', icon: 'M', description: 'An installed context-menu renderer' },
  kinds: [], uses_kinds: [], contributes: { desks: [] },
  capabilities: [{ grant: 'ui.shell' }],
  provides: [{ service: 'contextMenu.renderer', abi: 'choir.service/contextMenu.renderer/v1', entry: 'Menu.surface.svelte', requires_sdk: '^2.0', priority: 10 }],
};

const GOOD = `<script>
  import { closeContextMenu } from '../../lib/app-sdk';
  export let request = null;
</script>
{#if request}
  <div data-installed-menu role="menu" style="position:fixed;left:{request.x}px;top:{request.y}px;z-index:99999;background:#fff;color:#000;padding:4px">
    {#each request.items as item}{#if !item.separator}
      <button data-installed-item={item.id || item.label} on:click={() => { item.onSelect?.(); closeContextMenu(); }}>{item.label}</button>
    {/if}{/each}
    <button data-installed-extra on:click={() => closeContextMenu()}>Installed menu item</button>
  </div>
{/if}
`;

const THROWS = `<script>
  export let request = null;
  throw new Error('the installed renderer throws');
</script>
{#if request}<div>never</div>{/if}
`;

function buildSurface(files, name = 'menu-default') {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'choir-surface-'));
  const front = path.join(tmp, 'frontend');
  const pkg = path.join(front, 'src/packages', name);
  fs.mkdirSync(pkg, { recursive: true });
  for (const [rel, content] of Object.entries(files)) fs.writeFileSync(path.join(pkg, rel), content);
  for (const script of ['build-module.mjs', 'package-boundary-plugin.mjs', 'css-fetch-functions.json']) fs.copyFileSync(path.join(runtimeDir, script), path.join(front, script));
  fs.copyFileSync(path.join(frontendRoot, 'package.json'), path.join(front, 'package.json'));
  fs.symlinkSync(path.join(frontendRoot, 'node_modules'), path.join(front, 'node_modules'));
  const out = path.join(tmp, 'module.js');
  const r = spawnSync(process.execPath, [path.join(front, 'build-module.mjs'), front, name, out, '^2.0'], { encoding: 'utf8', cwd: front });
  const bytes = r.status === 0 ? fs.readFileSync(out) : null;
  fs.rmSync(tmp, { recursive: true, force: true });
  if (!bytes) throw new Error(`surface build failed: ${r.stdout}${r.stderr}`);
  return bytes;
}

const sha256 = (bytes) => createHash('sha256').update(bytes).digest('hex');
const LAUNCHER_MANIFEST = {
  schema: 'choir.package/v2', name: 'launcher', version: '1.1.0',
  display: { title: 'Launcher', icon: 'L', description: 'An installed launcher' },
  kinds: [], uses_kinds: [], contributes: { desks: [] },
  capabilities: [{ grant: 'ui.shell' }],
  provides: [{ service: 'launcher', abi: 'choir.service/launcher/v1', entry: 'Launcher.surface.svelte', requires_sdk: '^2.0' }],
};
// A module that loads but draws no surface.
const NOSURFACE = Buffer.from("export const abi = 'choir.app-frontend/module/v1';\nexport const requires_sdk = '^2.0';\nexport const sdk_imports = [];\n");
let good; let throws; let launcher;

test.beforeAll(() => {
  test.skip(!fs.existsSync(path.join(frontendRoot, 'node_modules/vite')), 'frontend/node_modules is not installed');
  const manifest = JSON.stringify(MANIFEST);
  good = buildSurface({ 'choir-package.json': manifest, 'Menu.surface.svelte': GOOD });
  throws = buildSurface({ 'choir-package.json': manifest, 'Menu.surface.svelte': THROWS });
  launcher = buildSurface({ 'choir-package.json': JSON.stringify(LAUNCHER_MANIFEST), 'Launcher.surface.svelte': '<div data-installed-launcher>installed launcher</div>\n' }, 'launcher');
});

const record = (bytes, extra = {}) => ({
  name: 'menu-default',
  release_id: sha256(Buffer.concat([bytes, Buffer.from('release')])),
  install_id: `install-${sha256(bytes).slice(0, 12)}`,
  installed_at: '2026-11-08T00:00:00.000Z',
  track: 'owner-1',
  isolation: 'shell',
  uses: ['closeContextMenu'],
  requires_sdk: '^2.0',
  manifest: MANIFEST,
  module: { abi: 'choir.app-frontend/module/v1', sha256: sha256(bytes), url: `/api/packages/modules/${sha256(bytes)}.js` },
  available: true,
  ...extra,
});

async function openShell(page, query = '') {
  await page.route(/\/(auth|api)\//, (route) => route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"unauthenticated"}' }));
  for (const bytes of [good, throws, launcher, NOSURFACE]) {
    await page.route(`**/api/packages/modules/${sha256(bytes)}.js`, (route) => route.fulfill({ status: 200, contentType: 'text/javascript', body: bytes.toString('utf8') }));
  }
  await page.goto(BASE_URL + query);
  await expect(page.locator(query ? '[data-last-resort-apps]' : '[data-desk-menu-button]')).toBeVisible();
}

// The shell's own instance of a module: the URL the page loaded it by (a dev server may add ?t=).
const inShell = (page, file, fn, arg) => page.evaluate(async ({ file, fn, arg }) => {
  const url = performance.getEntriesByType('resource').map((e) => e.name).find((n) => new URL(n).pathname === file) || file;
  const mod = await import(/* @vite-ignore */ url);
  // eslint-disable-next-line no-new-func
  return new Function('mod', 'arg', `return (${fn})(mod, arg);`)(mod, arg);
}, { file, fn, arg });

const install = (page, records) => inShell(page, '/src/lib/apps/packages.ts', '(p, records) => p.installedPackages.set(records)', records);
const rendererActive = (page) => inShell(page, '/src/lib/shell-slots.ts', `(s) => s.describeRecovery().find((r) => r.slot === 'contextMenu.renderer').active`);
const surfaceState = (page) => inShell(page, '/src/lib/installed-surfaces.ts', '(m) => m.describeInstalledSurfaces()');
const recoveryRow = (page) => inShell(page, '/src/lib/shell-slots.ts', `(s) => s.describeRecovery().find((r) => r.slot === 'contextMenu.renderer')`);
const toasts = (page, text) => page.locator('[role="status"], [role="alert"]').filter({ hasText: text });

async function filesRow(page) {
  await page.locator('[data-desktop-icon][data-desktop-icon-id="files"]').click();
  const files = page.locator('[data-window][data-window-app-id="files"]');
  const row = files.locator('[data-file-item]', { hasText: 'quarterly-review.pptx' });
  await expect(row).toBeVisible();
  return row;
}

test('I1 I4 I5: an installed menu-default displaces the in-tree menu on a file in Files, and uninstall returns it', async ({ page }) => {
  await openShell(page);
  await page.evaluate(() => { window.__tab = 'x'; });
  const row = await filesRow(page);
  await row.click({ button: 'right' });
  await expect(page.locator('[data-context-menu]').first()).toBeVisible();
  await expect(page.locator('[data-installed-menu]')).toHaveCount(0);
  await page.keyboard.press('Escape');
  await expect(page.locator('[data-context-menu]')).toHaveCount(0);

  const rec = record(good);
  await install(page, [rec]);
  await expect.poll(async () => (await surfaceState(page)).map((s) => `${s.package}:${s.state}`)).toEqual(['menu-default:active']);
  expect((await surfaceState(page))[0].release_id).toBe(rec.release_id);
  // The same provider id serves the slot; it is the installed one, so the in-tree menu is not drawn.
  expect(await rendererActive(page)).toBe('menu-default');
  await row.click({ button: 'right' });
  const installed = page.locator('[data-installed-menu]');
  await expect(installed).toBeVisible();
  await expect(installed.locator('[data-installed-item="open"]')).toBeVisible(); // the file's own items
  await expect(installed.locator('[data-installed-extra]')).toHaveText('Installed menu item'); // and the one the release adds
  await expect(page.locator('[data-context-menu]')).toHaveCount(0);
  await expect(page.locator('[data-last-resort="menu"]')).toHaveCount(0);
  await installed.locator('[data-installed-extra]').click();
  await expect(installed).toHaveCount(0);

  // I4: package revert = uninstall; the in-tree package returns.
  await install(page, []);
  await expect.poll(async () => (await surfaceState(page)).length).toBe(0);
  await expect.poll(async () => (await recoveryRow(page)).active).toBe('menu-default');
  await row.click({ button: 'right' });
  await expect(page.locator('[data-context-menu]').first()).toBeVisible();
  await expect(page.locator('[data-installed-menu]')).toHaveCount(0);
});

test('I3: an installed renderer that throws gets the base menu each time, is set aside on the third, and the in-tree menu returns', async ({ page }) => {
  await openShell(page);
  await install(page, [record(throws)]);
  await expect.poll(async () => (await surfaceState(page)).map((s) => s.state)).toEqual(['active']);
  const icon = page.locator('[data-desktop-icon][data-desktop-icon-id="files"]');
  for (let i = 0; i < 3; i++) {
    await icon.click({ button: 'right' });
    await expect(page.locator('[data-last-resort="menu"]')).toBeVisible(); // the base answers every time
    await page.keyboard.press('Escape');
    await expect(page.locator('[data-last-resort="menu"]')).toHaveCount(0);
  }
  await expect(toasts(page, 'menu-default stopped drawing contextMenu.renderer')).toHaveCount(1);
  await expect.poll(async () => (await surfaceState(page)).map((s) => s.state)).toEqual(['set-aside']);
  expect((await surfaceState(page))[0].reason).toContain('the installed renderer throws');
  // The in-tree menu-default serves again.
  await expect.poll(async () => (await recoveryRow(page)).failed).toEqual([]);
  await icon.click({ button: 'right' });
  await expect(page.locator('[data-context-menu]').first()).toBeVisible();
  await expect(page.locator('[data-last-resort="menu"]')).toHaveCount(0);
  await page.keyboard.press('Escape');

  // The same release is not mounted again by a refresh; uninstalling clears the record.
  await install(page, [record(throws)]);
  await install(page, []);
  await expect.poll(async () => (await surfaceState(page)).length).toBe(0);
});

test('I2: a release off the owner\'s own track stays in its frame and mounts no surface', async ({ page }) => {
  await openShell(page);
  await install(page, [record(good, { isolation: 'frame', track: 'someone-else' })]);
  await page.waitForTimeout(500);
  expect(await surfaceState(page)).toEqual([]);
  await page.locator('[data-desktop-icon][data-desktop-icon-id="files"]').click({ button: 'right' });
  await expect(page.locator('[data-context-menu]').first()).toBeVisible();
  await expect(page.locator('[data-installed-menu]')).toHaveCount(0);
});

test('I2: ?recovery=core mounts no installed surface and the base menu answers', async ({ page }) => {
  await openShell(page, '/?recovery=core');
  await install(page, [record(good)]);
  await page.waitForTimeout(500);
  expect(await surfaceState(page)).toEqual([]);
  await expect(page.locator('[data-installed-menu]')).toHaveCount(0);
});

// Review blockers 1 to 4 (Sonnet, Devin).

test('B1: a replacement release the shell refuses leaves the in-tree menu serving, not an empty slot', async ({ page }) => {
  await openShell(page);
  await install(page, [record(good)]);
  await expect.poll(async () => (await surfaceState(page)).map((s) => s.state)).toEqual(['active']);
  // A newer release whose SDK range this shell cannot satisfy replaces it.
  await install(page, [record(good, { install_id: 'install-newer', requires_sdk: '^9.0' })]);
  await expect.poll(async () => (await surfaceState(page)).map((s) => s.state)).toEqual(['refused']);
  await expect.poll(async () => (await recoveryRow(page)).active).toBe('menu-default');
  await page.locator('[data-desktop-icon][data-desktop-icon-id="files"]').click({ button: 'right' });
  await expect(page.locator('[data-context-menu]').first()).toBeVisible();
  await expect(page.locator('[data-installed-menu]')).toHaveCount(0);
});

test('B2: a provide the slot registrar swallows is detected, and the in-tree menu keeps serving', async ({ page }) => {
  await openShell(page);
  // The per-provide SDK range is one this shell cannot satisfy: registerRendererSurface skips it silently.
  const manifest = { ...MANIFEST, provides: [{ ...MANIFEST.provides[0], requires_sdk: '^9.0' }] };
  await install(page, [record(good, { manifest })]);
  await expect.poll(async () => (await surfaceState(page)).map((s) => s.state)).toEqual(['refused']);
  await expect.poll(async () => (await recoveryRow(page)).active).toBe('menu-default');
  await page.locator('[data-desktop-icon][data-desktop-icon-id="files"]').click({ button: 'right' });
  await expect(page.locator('[data-context-menu]').first()).toBeVisible();
  await expect(page.locator('[data-installed-menu]')).toHaveCount(0);
});

test('B3: "Use default menu" on an installed renderer brings the in-tree menu back and leaves no decline behind', async ({ page }) => {
  await openShell(page);
  const rec = record(good);
  await install(page, [rec]);
  await expect.poll(async () => (await surfaceState(page)).map((s) => s.state)).toEqual(['active']);
  const icon = page.locator('[data-desktop-icon][data-desktop-icon-id="files"]');
  await icon.click({ button: 'right' });
  await page.locator('[data-installed-item="use-default-menu"]').click();
  await expect.poll(async () => (await surfaceState(page)).map((s) => s.state)).toEqual(['set-aside']);
  await expect.poll(async () => (await recoveryRow(page)).active).toBe('menu-default');
  await icon.click({ button: 'right' });
  await expect(page.locator('[data-context-menu]').first()).toBeVisible(); // the in-tree menu, not the last resort
  await expect(page.locator('[data-last-resort="menu"]')).toHaveCount(0);
  await page.keyboard.press('Escape');
  // Nothing is held against the slot: uninstall, install again, and the installed menu serves.
  const declined = await inShell(page, '/src/lib/shell-kernel.ts', '(k) => k.shellKernel.snapshot().services?.["contextMenu.renderer"] ?? k.shellKernel.services.describe().declined');
  expect(JSON.stringify(declined ?? {})).not.toContain('menu-default"]');
  await install(page, []);
  await expect.poll(async () => (await surfaceState(page)).length).toBe(0);
  await install(page, [rec]);
  await expect.poll(async () => (await surfaceState(page)).map((s) => s.state)).toEqual(['active']);
  await icon.click({ button: 'right' });
  await expect(page.locator('[data-installed-menu]')).toBeVisible();
});

test('B4: a provider the kernel sets aside inside reconcile (activate failed) gives the slot back to the in-tree package', async ({ page }) => {
  await openShell(page);
  // A subscriber that throws for any launcher but the in-tree one makes the installed launcher's activation fail.
  await inShell(page, '/src/lib/shell-slots.ts', `(s) => {
    let inTree = null;
    s.activeSurfaces.subscribe((m) => {
      if (!inTree && m.launcher) inTree = m.launcher.component;
      if (m.launcher && inTree && m.launcher.component !== inTree) throw new Error('fixture: activation refused');
    });
  }`);
  const manifest = LAUNCHER_MANIFEST;
  await install(page, [record(launcher, { name: 'launcher', manifest })]);
  await expect.poll(async () => (await surfaceState(page)).map((s) => `${s.package}:${s.state}`)).toEqual(['launcher:set-aside']);
  expect((await surfaceState(page))[0].reason).toContain('activate failed');
  await expect.poll(async () => (await inShell(page, '/src/lib/shell-slots.ts', `(s) => s.describeRecovery().find((r) => r.slot === 'launcher').active`))).toBe('launcher');
  await expect(page.locator('[data-installed-launcher]')).toHaveCount(0);
});

test('R2-1: a module that loads but draws no surface is refused and shown, not retried as transient', async ({ page }) => {
  let fetches = 0;
  await openShell(page);
  await page.route(`**/api/packages/modules/${sha256(NOSURFACE)}.js`, (route) => { fetches += 1; return route.fulfill({ status: 200, contentType: 'text/javascript', body: NOSURFACE.toString('utf8') }); });
  const rec = record(NOSURFACE);
  await install(page, [rec]);
  await expect.poll(async () => (await surfaceState(page)).map((s) => s.state)).toEqual(['refused']);
  expect((await surfaceState(page))[0].reason).toContain('draws no surface');
  await install(page, [rec]); // a refresh of the same release does not fetch it again
  await page.waitForTimeout(300);
  expect(fetches).toBe(1);
  await page.locator('[data-desktop-icon][data-desktop-icon-id="files"]').click({ button: 'right' });
  await expect(page.locator('[data-context-menu]').first()).toBeVisible();
});

test('R2-2: a trip that races a replace does not take the newer mount down', async ({ page }) => {
  await openShell(page);
  const first = record(good);
  const second = record(good, { install_id: 'install-second' });
  await install(page, [first]);
  await expect.poll(async () => (await surfaceState(page)).map((s) => s.state)).toEqual(['active']);
  // In one task: the newer release arrives (its sync is queued), then the older one trips.
  await page.evaluate(async ({ second }) => {
    const resource = (file) => performance.getEntriesByType('resource').map((e) => e.name).find((n) => new URL(n).pathname === file) || file;
    const pk = await import(/* @vite-ignore */ resource('/src/lib/apps/packages.ts'));
    const sk = await import(/* @vite-ignore */ resource('/src/lib/shell-kernel.ts'));
    pk.installedPackages.set([second]);
    sk.shellKernel.services.trip('contextMenu.renderer', 'menu-default', 'raced');
  }, { second });
  await expect.poll(async () => (await surfaceState(page)).map((s) => `${s.state}`)).toContain('active');
  await page.waitForTimeout(500);
  const rows = await surfaceState(page);
  expect(rows.filter((r) => r.state === 'active').length).toBe(1);
  // The newer mount really serves: the installed menu opens, and the slot is not wedged.
  await page.locator('[data-desktop-icon][data-desktop-icon-id="files"]').click({ button: 'right' });
  await expect(page.locator('[data-installed-menu]')).toBeVisible();
});
