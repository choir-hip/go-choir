// The shell surfaces as packages, slices H1 to H4 (lib/shell-slots.ts,
// lib/ShellSlot.svelte; plan section 11). Two parts:
//
//  - the slots on the real kernel, loaded through vite (no browser): every
//    shell slot (the context-menu renderer among them, H2) has its in-tree
//    surface package as the active provider, a bad
//    registration is refused, a failed surface is set aside, and a package that
//    leaves takes its surface with it;
//  - the drills in the shell (vite dev server, like context-menu-open-with):
//    the surfaces render from their packages, and with a surface package gone
//    the host's last-resort surface keeps the slot usable.
//
// Failure modes (shell-slots.ts S1 to S5):
//  S1 a surface for a slot the shell does not have, or with no component: refused;
//  S2 (requires_sdk out of range) is covered by check-packages (L10 for provides);
//  S3 a surface that fails is out of the ranking and the slot falls back;
//  S4 the package leaves: its surface goes, the slot falls to the last resort;
//  S5 the last resort does not flash before the boot registrations settle.
// H3, the prompt bar:
//  P1 with the prompt-bar package gone the owner has no door: the host's
//     last-resort field must take text (to the conductor) and open the apps;
//  P2 Cmd+K, the host's command, stops opening the desk with the package gone.
//
//   cd frontend && npx vite --port 5173 &
//   npx playwright test tests/shell-surfaces.spec.js
import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';

test.describe('the slots on the kernel', () => {
  let vite; let M; let K; let C;
  test.beforeAll(async () => {
    // Its own dependency cache, so this server never re-optimizes the dev server's under a running page.
    vite = await createServer({ root, cacheDir: fs.mkdtempSync(path.join(os.tmpdir(), 'choir-surfaces-vite-')), server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
    M = await vite.ssrLoadModule('/src/lib/shell-slots.ts');
    K = await vite.ssrLoadModule('/src/lib/shell-kernel.ts');
    C = await vite.ssrLoadModule('/src/lib/services/context-menu.ts');
    await vite.ssrLoadModule('/src/lib/menu-host.ts');
    await K.shellKernel.services.settled();
  });
  test.afterAll(async () => { await vite?.close(); });

  test('every shell slot is drawn by its in-tree package, an ordinary provider in the snapshot', () => {
    const want = { launcher: 'launcher', 'desktop.icons': 'desktop-icons', overview: 'overview', prompt: 'prompt-bar' };
    const slots = Object.fromEntries(M.describeSlots().map((s) => [s.slot, s]));
    for (const [slot, pkg] of Object.entries(want)) {
      expect(slots[slot].active, slot).toBe(pkg);
      expect(slots[slot].providers).toContain(pkg);
    }
    const tree = M.inTreeSurfaces();
    for (const [slot, pkg] of Object.entries(want)) {
      expect(tree.find((s) => s.slot === slot)).toMatchObject({ name: pkg });
      expect(tree.find((s) => s.slot === slot).component).toBeTruthy();
    }
    const services = K.shellKernel.services.describe().services;
    expect(services.launcher.abi).toBe('choir.service/launcher/v1');
    // H2: the context menu's default renderer is the menu-default package; the host's last resort is the base.
    expect(services[C.RENDERER_SLOT].active).toBe('menu-default');
    expect(services[C.RENDERER_SLOT].providers.map((p) => p.id).sort()).toEqual([C.BASE_RENDERER_ID, 'menu-default'].sort());
    expect(tree.find((s) => s.slot === C.RENDERER_SLOT)).toMatchObject({ name: 'menu-default', priority: 10 });
  });

  test('S1: a surface for no slot, or without a component, is refused', () => {
    const scope = K.shellKernel.scope('surface-test-s1');
    expect(() => M.registerSurface(scope, 'taskbar', { id: 'x', component: () => {} })).toThrow(/not a shell slot/);
    expect(() => M.registerSurface(scope, 'launcher', { id: 'Bad Id', component: () => {} })).toThrow(/package name/);
    expect(() => M.registerSurface(scope, 'launcher', { id: 'nocomp' })).toThrow(/component is missing/);
  });

  test('S3, S4: a higher-ranked surface takes the slot; when it fails or leaves, the default returns', async () => {
    const scope = K.shellKernel.scope('surface-test-s3');
    const Fake = function Fake() {};
    M.registerSurface(scope, 'launcher', { id: 'launcher-compact', component: Fake, priority: 20 });
    await K.shellKernel.services.settled();
    let active;
    const off = M.activeSurfaces.subscribe((v) => { active = v; });
    expect(active.launcher.id).toBe('launcher-compact');
    M.reportSurfaceFailure('launcher', 'launcher-compact', 'threw while mounting');
    await K.shellKernel.services.settled();
    expect(active.launcher.id).toBe('launcher');
    await scope.dispose();
    await K.shellKernel.services.settled();
    expect(active.launcher.id).toBe('launcher');
    // S4: the default package leaves; nothing provides the slot (the host shows its last resort).
    await M.removeSurfacePackage('overview');
    await K.shellKernel.services.settled();
    expect(active.overview).toBeUndefined();
    expect(M.describeSlots().find((s) => s.slot === 'overview').providers).toEqual([]);
    off();
  });
});

test.describe('the drills in the shell', () => {
  // The shell's own instance of shell-slots.ts: the URL the page loaded it by (a dev server may add ?t=).
  const removeSurfacePackages = (page, names) => page.evaluate(async (list) => {
    const entry = performance.getEntriesByType('resource').map((e) => e.name).find((n) => new URL(n).pathname === '/src/lib/shell-slots.ts');
    const slots = await import(/* @vite-ignore */ entry || '/src/lib/shell-slots.ts');
    for (const name of list) await slots.removeSurfacePackage(name);
  }, names);

  async function openShell(page) {
    await page.route(/\/(auth|api)\//, (route) => route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"unauthenticated"}' }));
    await page.goto(BASE_URL);
    await expect(page.locator('[data-desk-menu-button]')).toBeVisible();
  }

  test('the launcher, the desktop icons and the overview render from their packages', async ({ page }) => {
    await openShell(page);
    await expect(page.locator('[data-shell-slot="desktop.icons"]')).toHaveAttribute('data-shell-slot-provider', 'desktop-icons');
    await expect(page.locator('[data-desktop-icon]').first()).toBeVisible();
    await page.locator('[data-desk-menu-button]').click();
    await expect(page.locator('[data-shell-slot="launcher"]')).toHaveAttribute('data-shell-slot-provider', 'launcher');
    await expect(page.locator('[data-desk-sheet]')).toBeVisible();
    await page.locator('[data-desk-overview]').click();
    await expect(page.locator('[data-desk-sheet]')).toHaveCount(0);
    await expect(page.locator('[data-shell-slot="overview"]')).toHaveAttribute('data-shell-slot-provider', 'overview');
    await expect(page.locator('[data-desktop-overview]')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.locator('[data-desktop-overview]')).toHaveCount(0);
  });

  test('with the launcher package gone the last-resort launcher opens apps; with the icons gone windows still work', async ({ page }) => {
    await openShell(page);
    await removeSurfacePackages(page, ['launcher', 'desktop-icons']);
    await expect(page.locator('[data-desktop-icon]')).toHaveCount(0);
    await page.locator('[data-desk-menu-button]').click();
    const lastResort = page.locator('[data-last-resort="launcher"]');
    await expect(lastResort).toBeVisible();
    await lastResort.locator('[data-last-resort-app="files"]').click();
    await expect(lastResort).toHaveCount(0);
    await expect(page.locator('[data-window][data-window-app-id="files"]')).toBeVisible();
  });

  test('H2: the menu-default package renders the context menu; without it the last-resort menu answers and runs the item', async ({ page }) => {
    await openShell(page);
    const icon = page.locator('[data-desktop-icon][data-desktop-icon-id="files"]');
    await icon.click({ button: 'right' });
    const menu = page.locator('[data-context-menu]').first();
    await expect(menu).toBeVisible();
    await expect(page.locator('[data-last-resort="menu"]')).toHaveCount(0);
    await expect(menu.locator('[data-context-menu-item="use-default-menu"]')).toHaveCount(0); // the default offers no escape from itself
    await page.keyboard.press('Escape');
    await expect(page.locator('[data-context-menu]')).toHaveCount(0);

    await removeSurfacePackages(page, ['menu-default']);
    await icon.click({ button: 'right' });
    const lastResort = page.locator('[data-last-resort="menu"]');
    await expect(lastResort).toBeVisible();
    await lastResort.locator('[data-context-menu-item="open"]').click();
    await expect(lastResort).toHaveCount(0);
    await expect(page.locator('[data-window][data-window-app-id="files"]')).toBeVisible();
  });

  test('H3: the prompt bar renders from its package; without it the last-resort field takes text and Cmd+K still opens the desk', async ({ page }) => {
    await openShell(page);
    await expect(page.locator('[data-shell-slot="prompt"]')).toHaveAttribute('data-shell-slot-provider', 'prompt-bar');
    await expect(page.locator('[data-prompt-surface]')).toBeVisible();

    await removeSurfacePackages(page, ['prompt-bar']);
    await expect(page.locator('[data-prompt-surface]')).toHaveCount(0);
    const field = page.locator('[data-last-resort="prompt"]');
    await expect(field).toBeVisible();
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+K' : 'Control+K');
    await expect(page.locator('[data-desk-sheet]')).toBeVisible();
    await page.keyboard.press(process.platform === 'darwin' ? 'Meta+K' : 'Control+K');
    await expect(page.locator('[data-desk-sheet]')).toHaveCount(0);
    await field.locator('[data-last-resort-apps]').click();
    await expect(page.locator('[data-desk-sheet]')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.locator('[data-desk-sheet]')).toHaveCount(0);
    // Signed out, a submit asks the visitor to sign in, as the prompt bar does.
    await field.locator('[data-last-resort-prompt-input]').fill('Write about currents');
    await field.locator('[data-last-resort-prompt-input]').press('Enter');
    await expect(page.locator('[data-auth-overlay]')).toBeVisible();
  });
});
