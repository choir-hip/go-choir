// The shell surfaces as packages, slices H1 to H4 (lib/shell-slots.ts,
// lib/ShellSlot.svelte; plan section 11). Two parts:
//
//  - the slots on the real kernel, loaded through vite (no browser): every
//    shell slot has its in-tree surface package as the active provider, a bad
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
//
//   cd frontend && npx vite --port 5173 &
//   npx playwright test tests/shell-surfaces.spec.js
import { test, expect } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';

test.describe('the slots on the kernel', () => {
  let vite; let M; let K;
  test.beforeAll(async () => {
    vite = await createServer({ root, server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
    M = await vite.ssrLoadModule('/src/lib/shell-slots.ts');
    K = await vite.ssrLoadModule('/src/lib/shell-kernel.ts');
    await K.shellKernel.services.settled();
  });
  test.afterAll(async () => { await vite?.close(); });

  test('every shell slot is drawn by its in-tree package, an ordinary provider in the snapshot', () => {
    const want = { launcher: 'launcher', 'desktop.icons': 'desktop-icons', overview: 'overview' };
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
    await page.evaluate(async () => {
      const slots = await import('/src/lib/shell-slots.ts');
      await slots.removeSurfacePackage('launcher');
      await slots.removeSurfacePackage('desktop-icons');
    });
    await expect(page.locator('[data-desktop-icon]')).toHaveCount(0);
    await page.locator('[data-desk-menu-button]').click();
    const lastResort = page.locator('[data-last-resort="launcher"]');
    await expect(lastResort).toBeVisible();
    await lastResort.locator('[data-last-resort-app="files"]').click();
    await expect(lastResort).toHaveCount(0);
    await expect(page.locator('[data-window][data-window-app-id="files"]')).toBeVisible();
  });
});
