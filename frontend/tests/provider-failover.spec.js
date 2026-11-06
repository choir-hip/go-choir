// Watchdog failover and the yank reaching the kernel, slice L4 (plan section
// 11, Library row; docs/design/modular-architecture-2026-11-04.md Phase E L4,
// section 9 and decision 4). Two parts:
//
//  - on the real kernel, loaded through vite (no browser): the platform's yank
//    arrives with the update sync and the slot falls back to its base-image
//    package, never to another third party's provider ranked by priority; the
//    write carries only that slot; the theme gate refuses an escaped
//    declaration break;
//  - in the shell (vite dev server on 5173): a renderer that throws is
//    reported to the watchdog, the base menu answers every time, and the third
//    throw sets it aside with the reason (recovery shows it, one toast says
//    it); a surface that throws while mounting is set aside with its reason;
//    the boot recovery entry ?recovery=core loads no surface package, opens
//    the recovery surface, the right-click opens the base menu, and a reload
//    without it is the normal shell (nothing was stored).
//
// Failure modes, written before the code:
//  F1 a provider that throws keeps the slot, or the owner gets no menu while
//     it does (the base must answer each time);
//  F2 the trip has no visible reason, or the owner is told once per failure;
//  F3 a yanked provider's slot falls to another third party by priority;
//  F4 the yank fallback's write clobbers another slot's record (or a choice
//     in another window clobbers it);
//  F5 the yank fallback runs before the owner's records are read, and so
//     misses a chosen slot;
//  F6 ?recovery=core is stored (a later load without it is still in recovery)
//     or still loads a surface package (a broken one would hide the fix);
//  F7 an escaped ';' or '}' passes the theme gate.
//
//   cd frontend && npx vite --port 5173 &
//   npx playwright test tests/provider-failover.spec.js
import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:5173';
const KEY_PATH = '/api/preferences/packages.selection';

test.describe('the yank on the kernel', () => {
  let vite; let S; let K; let P; let A; let T;
  let stored = {};
  let acts = [];
  const puts = [];
  const realFetch = globalThis.fetch;
  const json = (body, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });
  test.beforeAll(async () => {
    globalThis.fetch = async (url, init = {}) => {
      const p = new URL(String(url), 'http://localhost').pathname;
      const method = init.method || 'GET';
      if (p === KEY_PATH && method === 'PUT') {
        // The computer's merge (internal/mediastate handleSelections).
        const changes = JSON.parse(String(init.body)).value;
        puts.push(changes);
        stored = { ...stored, ...changes };
        return json({ key: 'packages.selection', value: stored });
      }
      if (p === KEY_PATH) return json({ key: 'packages.selection', value: stored, default: Object.keys(stored).length === 0 });
      if (p === '/api/shell/bootstrap') return json({ computer_id: 'computer-1' });
      if (p === '/api/computers/computer-1/packages/updates') return json({ updates: acts, policy: {} });
      if (p === '/api/packages/installed') return json({ packages: [] });
      return json({ error: 'not_stubbed' }, 404);
    };
    vite = await createServer({ root, cacheDir: fs.mkdtempSync(path.join(os.tmpdir(), 'choir-failover-vite-')), server: { middlewareMode: true, hmr: false, watch: null }, appType: 'custom', logLevel: 'silent' });
    S = await vite.ssrLoadModule('/src/lib/shell-slots.ts');
    K = await vite.ssrLoadModule('/src/lib/shell-kernel.ts');
    P = await vite.ssrLoadModule('/src/lib/services/provider-selection.ts');
    A = await vite.ssrLoadModule('/src/lib/apps/packages.ts');
    T = await vite.ssrLoadModule('/src/lib/theme.ts');
    await K.shellKernel.services.settled();
  });
  test.afterAll(async () => {
    globalThis.fetch = realFetch;
    await vite?.close();
  });

  const row = async (slot) => (await P.providerSlots()).find((r) => r.slot === slot);

  test('F3 F4 F5: a yanked chosen provider falls back to the base, not to the other third party, and only its slot is written', async () => {
    const scope = K.shellKernel.scope('failover-test');
    const Fake = function Fake() {};
    S.registerSurface(scope, 'launcher', { id: 'launcher-fancy', component: Fake, priority: 20 });
    S.registerSurface(scope, 'launcher', { id: 'launcher-other', component: Fake, priority: 40 });
    stored = { prompt: { package: 'prompt-bar', at: '2026-11-06T10:00:00.000Z' } };
    await P.loadProviderSelections();
    expect(await P.selectProvider('launcher', 'launcher-fancy')).toEqual({ ok: true });
    // F4: the choice wrote only its own slot; the computer's merge kept the other.
    expect(Object.keys(puts.at(-1))).toEqual(['launcher']);
    expect(stored.prompt.package).toBe('prompt-bar');
    expect((await row('launcher')).active).toBe('launcher-fancy');

    // The platform yanks launcher-fancy; the next sync carries the act.
    acts = [{ package: 'launcher-fancy', state: 'removed_by_platform', detail: 'the publisher key was revoked' }];
    A.clearInstalledPackages(); // a fresh sign-in: the sync runs once more
    const writes = puts.length;
    await A.refreshInstalledPackages();
    await expect.poll(async () => (await row('launcher')).active).toBe('launcher');
    const r = await row('launcher');
    // F3: the base, although launcher-other outranks it by priority and is healthy.
    expect(r.selected).toBe('launcher');
    expect(r.providers.find((p) => p.id === 'launcher-other').healthy).toBe(true);
    const yanked = r.providers.find((p) => p.id === 'launcher-fancy');
    expect(yanked.healthy).toBe(false);
    expect(yanked.reason).toBe('yanked by the platform: the publisher key was revoked');
    // F4: one write, for the launcher slot only; the prompt record stands.
    expect(puts.length).toBe(writes + 1);
    expect(Object.keys(puts.at(-1))).toEqual(['launcher']);
    expect(stored.launcher.package).toBe('launcher');
    expect(stored.prompt.package).toBe('prompt-bar');

    // A sync with no act changes nothing.
    acts = [];
    A.clearInstalledPackages();
    await A.refreshInstalledPackages();
    await K.shellKernel.services.settled();
    expect(puts.length).toBe(writes + 1);
    await scope.dispose();
    await P.clearProviderSelections();
  });

  test('F7: an escaped declaration break does not pass the theme gate', () => {
    const base = T.DEFAULT_THEME;
    for (const bad of ['red\\3b background: blue', 'red\\7d body\\7b color: red', 'red/**/\\3B x: y', 'red; x: y']) {
      const theme = { ...base, colors: { ...base.colors, selected: bad } };
      const v = T.validateThemeConfig(theme);
      expect(v.ok, bad).toBe(false);
      expect(v.errors.join(' '), bad).toContain('colors.selected must be a safe CSS value');
      expect(T.normalizeThemeConfig(theme).colors.selected, bad).toBe(base.colors.selected);
    }
    expect(T.validateThemeConfig({ ...base, colors: { ...base.colors, selected: '#ffcc00' } }).ok).toBe(true);
  });
});

test.describe('failover and recovery in the shell', () => {
  // The shell's own instance of a module: the URL the page loaded it by (a dev server may add ?t=).
  const inShell = (page, file, fn, arg) => page.evaluate(async ({ file, fn, arg }) => {
    const url = performance.getEntriesByType('resource').map((e) => e.name).find((n) => new URL(n).pathname === file) || file;
    const mod = await import(/* @vite-ignore */ url);
    // eslint-disable-next-line no-new-func
    return new Function('mod', 'arg', `return (${fn})(mod, arg);`)(mod, arg);
  }, { file, fn, arg });

  async function openShell(page, query = '') {
    await page.route(/\/(auth|api)\//, (route) => route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"unauthenticated"}' }));
    await page.goto(BASE_URL + query);
    // In a recovery boot the prompt-bar package (the desk button) is not loaded; the host's last-resort field is.
    await expect(page.locator(query ? '[data-last-resort-apps]' : '[data-desk-menu-button]')).toBeVisible();
  }
  const recoveryKey = process.platform === 'darwin' ? 'Meta+Alt+R' : 'Control+Alt+R';
  const toasts = (page, text) => page.locator('[role="status"], [role="alert"]').filter({ hasText: text });

  test('F1 F2: a renderer that throws gets the base menu each time, is set aside on the third throw with the reason, and the owner is told once', async ({ page }) => {
    await openShell(page);
    await inShell(page, '/src/lib/shell-slots.ts', `async (slots) => {
      const url = performance.getEntriesByType('resource').map((e) => e.name).find((n) => new URL(n).pathname === '/src/lib/services/context-menu.ts');
      const menu = await import(url);
      menu.registerContextMenuRenderer(slots.surfaceScope('menu-throws'), {
        id: 'menu-throws', priority: 90, grants: ['rank.high'],
        impl: { render() { throw new Error('the fixture renderer throws'); } },
      });
    }`);
    await expect.poll(() => inShell(page, '/src/lib/shell-slots.ts', `(s) => s.describeRecovery().find((r) => r.slot === 'contextMenu.renderer').active`)).toBe('menu-throws');

    const icon = page.locator('[data-desktop-icon][data-desktop-icon-id="files"]');
    for (let i = 0; i < 3; i++) {
      await icon.click({ button: 'right' });
      await expect(page.locator('[data-last-resort="menu"]')).toBeVisible(); // F1: the base answers
      await page.keyboard.press('Escape');
      await expect(page.locator('[data-last-resort="menu"]')).toHaveCount(0);
    }
    // F2: one toast, naming the provider and the reason.
    await expect(toasts(page, 'menu-throws stopped drawing contextMenu.renderer')).toHaveCount(1);
    await expect(toasts(page, 'the fixture renderer throws')).toHaveCount(1);
    // Un-ranked: menu-default draws again.
    await icon.click({ button: 'right' });
    await expect(page.locator('[data-context-menu]').first()).toBeVisible();
    await expect(page.locator('[data-last-resort="menu"]')).toHaveCount(0);
    await page.keyboard.press('Escape');

    // A surface that throws while mounting is set aside at once, with its reason.
    await inShell(page, '/src/lib/shell-slots.ts', `(s) => {
      function Broken() { throw new Error('the fixture surface throws'); }
      s.registerSurface(s.surfaceScope('icons-broken'), 'desktop.icons', { id: 'icons-broken', component: Broken, priority: 90 });
    }`);
    await expect(toasts(page, 'icons-broken stopped drawing desktop.icons')).toHaveCount(1);
    await expect(page.locator('[data-shell-slot="desktop.icons"]')).toHaveAttribute('data-shell-slot-provider', 'desktop-icons');

    // The recovery surface shows both, with the reasons.
    await page.keyboard.press(recoveryKey);
    const recovery = page.locator('[data-last-resort="recovery"]');
    await expect(recovery).toBeVisible();
    await expect(recovery.locator('[data-recovery-boot]')).toHaveCount(0);
    await expect(recovery.locator('[data-recovery-slot="contextMenu.renderer"] [data-recovery-failed="menu-throws"]')).toContainText('render threw: the fixture renderer throws');
    await expect(recovery.locator('[data-recovery-slot="desktop.icons"] [data-recovery-failed="icons-broken"]')).toContainText('failed to mount: the fixture surface throws');
    await expect(recovery.locator('[data-recovery-slot="contextMenu.renderer"]')).toHaveAttribute('data-recovery-last-resort', 'false');
  });

  test('F6: ?recovery=core loads no surface package, opens recovery, the right-click opens the base menu, and a reload without it is the normal shell', async ({ page }) => {
    await openShell(page, '/?recovery=core');
    const recovery = page.locator('[data-last-resort="recovery"]');
    await expect(recovery).toBeVisible();
    await expect(recovery.locator('[data-recovery-boot]')).toBeVisible();
    for (const slot of ['launcher', 'desktop.icons', 'overview', 'prompt', 'notifications.host', 'contextMenu.renderer']) {
      await expect(recovery.locator(`[data-recovery-slot="${slot}"]`)).toHaveAttribute('data-recovery-last-resort', 'true');
    }
    await expect(page.locator('[data-desktop-icon]')).toHaveCount(0);
    // Bring the icons back for this session only; the renderer stays the host's.
    await recovery.locator('[data-recovery-restore="desktop-icons"]').click();
    await expect(recovery.locator('[data-recovery-slot="desktop.icons"]')).toHaveAttribute('data-recovery-last-resort', 'false');
    await recovery.locator('[data-last-resort-close]').click();
    await page.locator('[data-desktop-icon][data-desktop-icon-id="files"]').click({ button: 'right' });
    const base = page.locator('[data-last-resort="menu"]');
    await expect(base).toBeVisible();
    await base.locator('[data-context-menu-item="open"]').click();
    await expect(page.locator('[data-window][data-window-app-id="files"]')).toBeVisible();

    // F6: nothing was stored; a plain load is the normal shell.
    await page.goto(BASE_URL);
    await expect(page.locator('[data-desk-menu-button]')).toBeVisible();
    await expect(page.locator('[data-last-resort="recovery"]')).toHaveCount(0);
    await expect(page.locator('[data-shell-slot="desktop.icons"]')).toHaveAttribute('data-shell-slot-provider', 'desktop-icons');
    await page.locator('[data-desktop-icon][data-desktop-icon-id="files"]').click({ button: 'right' });
    await expect(page.locator('[data-context-menu]').first()).toBeVisible();
    await expect(page.locator('[data-last-resort="menu"]')).toHaveCount(0);
  });
});
