// Deployed acceptance for the desktop crash guard
// (docs/problems/desktop-restore-reopens-the-window-that-crashed-the-page-crash-loop-2026-11-16.md).
// A fresh account is registered with a virtual authenticator (Chromium, as the
// other deployed specs do), then the sequence runs against the real desktop
// and backend, no mocks: a window pending when the page "dies" comes back held,
// stays held across reloads, Reopen then a second death counts two, Reopen then
// proof clears the hold, and an ordinary reload holds nothing. Only light
// built-in apps are opened (Settings, Files); no prompt is submitted, so no
// agent work starts. No existing account or computer is touched.
//
//   BASE_URL=https://choir.news npx playwright test tests/crash-guard-deployed.spec.js
//   CRASH_GUARD_ENGINE=webkit ...   (the sequence in WebKit with the same fresh
//                                    account's session; registration stays in Chromium)
//
// Writes a JSON receipt to CRASH_GUARD_RECEIPT when set (no address in it).
import fs from 'node:fs';
import { chromium, webkit, test, expect } from '@playwright/test';
import { registerPasskey } from './helpers/auth.js';
import { setupVirtualAuthenticator } from './helpers/webauthn.js';

const BASE_URL = process.env.BASE_URL || 'https://choir.news';
const RECEIPT = process.env.CRASH_GUARD_RECEIPT || '';
const ENGINE = process.env.CRASH_GUARD_ENGINE === 'webkit' ? 'webkit' : 'chromium';
const QUIET_PAST_MS = 25_000;

// The auth service allows 10 requests a minute per address; a 429 waits out the window.
async function paced(fn) {
  for (let i = 0; i < 4; i++) {
    try {
      return await fn();
    } catch (err) {
      if (!/429|too many auth requests/.test(String(err))) throw err;
      await new Promise((r) => setTimeout(r, 61_000));
    }
  }
  throw new Error('still rate limited');
}

// Registered before the app: swallow the leaving handlers once window.__dying is
// set, so the reload that follows is what a hung or killed page leaves behind.
const DYING_INIT = () => {
  const stop = (e) => { if (window.__dying) e.stopImmediatePropagation(); };
  window.addEventListener('pagehide', stop, true);
  document.addEventListener('visibilitychange', stop, true);
};

const READY = '[data-desktop][data-authenticated="true"][data-desktop-ready="true"]';
const win = (page, appId) => page.locator(`[data-window][data-window-app-id="${appId}"]`);
const body = (page, appId) => page.locator(`[data-app-host][data-app-id="${appId}"]`);

async function guardState(page) {
  return page.evaluate(() => {
    const key = Object.keys(localStorage).find((k) => k.startsWith('choir.crashGuard.v1'));
    const state = key ? JSON.parse(localStorage.getItem(key) || '{}') : {};
    return {
      key: key ? key.replace(/:.*/, ':<user>') : '',
      pending: Object.values(state.pending || {}).map((e) => ({ windowId: e.windowId, appId: e.appId, step: e.step || '' })),
      held: Object.fromEntries(Object.entries(state.held || {}).map(([k, h]) => [k, h.count])),
    };
  });
}

async function serverWindows(page) {
  return page.evaluate(async () => {
    const res = await fetch('/api/desktop/state', { credentials: 'include' });
    const json = await res.json().catch(() => ({}));
    return (json.windows || []).map((w) => w.app_id);
  });
}

async function ready(page) {
  await page.locator(READY).waitFor({ state: 'visible', timeout: 180_000 });
}

async function reload(page, { dies = false } = {}) {
  if (dies) await page.evaluate(() => { window.__dying = true; });
  await page.reload({ waitUntil: 'domcontentloaded' });
  await ready(page);
}

test('the crash guard on the deployed desktop: held, durable, reopened, proven, ordinary reload', async () => {
  test.setTimeout(1_200_000);
  const tag = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const email = `crash-guard-${tag}@example.com`;
  const receipt = { base_url: BASE_URL, engine: ENGINE, started_at: new Date().toISOString(), steps: [] };
  let failed = '';
  const run = async (name, fn) => {
    const at = new Date().toISOString();
    try {
      const observed = (await fn()) || {};
      receipt.steps.push({ name, at, pass: true, ...observed });
    } catch (err) {
      receipt.steps.push({ name, at, pass: false, error: String(err.message || err).split('\n')[0] });
      failed = name;
      throw err;
    }
  };

  let regBrowser;
  let browser;
  let context;
  try {
    // The deployed commit.
    const health = await (await fetch(`${BASE_URL}/health`)).json().catch(() => ({}));
    receipt.deployed_commit = health?.build?.deployed_commit || health?.platform?.deployed_commit || '';

    // A fresh account, registered in Chromium with the virtual authenticator.
    regBrowser = await chromium.launch();
    const regContext = await regBrowser.newContext();
    const regPage = await regContext.newPage();
    await setupVirtualAuthenticator(regPage);
    await regPage.goto(BASE_URL);
    expect(await paced(() => registerPasskey(regPage, email, BASE_URL))).toBeTruthy();
    const storageState = await regContext.storageState();
    await regContext.close();
    await regBrowser.close();
    regBrowser = null;

    browser = await (ENGINE === 'webkit' ? webkit : chromium).launch();
    context = await browser.newContext({ storageState, viewport: { width: 1440, height: 900 } });
    await context.addInitScript(DYING_INIT);
    const page = await context.newPage();

    await run('0 the frontend this account is served carries the crash guard', async () => {
      await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' });
      await ready(page);
      // A signed-in page is served by the account's own computer, whose frontend can
      // trail the platform's deployed commit until that computer is refreshed.
      const scripts = await page.evaluate(async () => {
        const urls = [...new Set(performance.getEntriesByType('resource').map((e) => e.name).filter((u) => /\/assets\/.*\.js/.test(u)))];
        const out = [];
        for (const u of urls) out.push({ file: u.replace(/^.*\/assets\//, ''), has_guard: (await (await fetch(u)).text()).includes('choir.crashGuard') });
        return out;
      });
      expect(scripts.some((x) => x.has_guard), `no loaded script carries the crash guard: ${JSON.stringify(scripts)}`).toBe(true);
      return { scripts };
    });

    await run('1 open a light app; its marker exists before the quiet period ends', async () => {
      await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' });
      await ready(page);
      const initial = await win(page, 'settings').count();
      await page.locator('[data-desktop-icon-id="settings"]').click();
      await expect(body(page, 'settings')).toBeAttached();
      const t0 = Date.now();
      await expect.poll(async () => (await guardState(page)).pending.length, { timeout: 4000 }).toBeGreaterThan(0);
      const marker = await guardState(page);
      const msAfterOpen = Date.now() - t0;
      // The desktop state must be on the server before the page "dies" (a dying page cannot flush).
      await expect.poll(() => serverWindows(page), { timeout: 20_000 }).toContain('settings');
      return { initial_settings_windows: initial, marker, ms_to_see_marker: msAfterOpen };
    });

    await run('2 page death while pending: restored held, no app body', async () => {
      await reload(page, { dies: true });
      const held = win(page, 'settings').locator('[data-crash-held]');
      await expect(held).toBeVisible({ timeout: 30_000 });
      await expect(held).toContainText('was still opening when the page was reloaded or stopped');
      await expect(body(page, 'settings')).toHaveCount(0);
      return { count: await held.getAttribute('data-crash-held-count'), guard: await guardState(page) };
    });

    await run('3 two ordinary reloads: still held, count unchanged', async () => {
      const held = win(page, 'settings').locator('[data-crash-held]');
      const counts = [];
      for (let i = 0; i < 2; i++) {
        await reload(page);
        await expect(held).toBeVisible({ timeout: 30_000 });
        await expect(body(page, 'settings')).toHaveCount(0);
        counts.push(await held.getAttribute('data-crash-held-count'));
      }
      expect(counts).toEqual(['1', '1']);
      return { counts };
    });

    await run('4 Reopen, die again: held with count 2, Close offered first', async () => {
      const held = win(page, 'settings').locator('[data-crash-held]');
      await held.locator('[data-crash-held-reopen]').click();
      await expect(body(page, 'settings')).toBeAttached();
      await expect.poll(async () => (await guardState(page)).pending.length, { timeout: 4000 }).toBeGreaterThan(0);
      await reload(page, { dies: true });
      await expect(held).toBeVisible({ timeout: 30_000 });
      await expect(held).toHaveAttribute('data-crash-held-count', '2');
      await expect(held.locator('button').first()).toHaveAttribute('data-crash-held-close', '');
      await expect(body(page, 'settings')).toHaveCount(0);
      return { count: await held.getAttribute('data-crash-held-count'), guard: await guardState(page) };
    });

    await run('5 Reopen, prove past the quiet period, reload: mounts, no hold remains', async () => {
      const held = win(page, 'settings').locator('[data-crash-held]');
      await held.locator('[data-crash-held-reopen]').click();
      await expect(body(page, 'settings')).toBeAttached();
      await expect.poll(async () => (await guardState(page)).pending.length, { timeout: QUIET_PAST_MS }).toBe(0);
      const proven = await guardState(page);
      expect(proven.held).toEqual({});
      await reload(page);
      await expect(body(page, 'settings')).toBeAttached({ timeout: 30_000 });
      await expect(page.locator('[data-crash-held]')).toHaveCount(0);
      return { after_proof: proven, after_reload: await guardState(page) };
    });

    await run('6 a second window proves; an ordinary reload holds nothing', async () => {
      await page.locator('[data-desktop-icon-id="files"]').click();
      await expect(body(page, 'files')).toBeAttached({ timeout: 30_000 });
      await expect.poll(async () => (await guardState(page)).pending.length, { timeout: QUIET_PAST_MS }).toBe(0);
      await expect.poll(() => serverWindows(page), { timeout: 20_000 }).toEqual(expect.arrayContaining(['settings', 'files']));
      await reload(page);
      await expect(body(page, 'settings')).toBeAttached({ timeout: 30_000 });
      await expect(body(page, 'files')).toBeAttached({ timeout: 30_000 });
      await expect(page.locator('[data-crash-held]')).toHaveCount(0);
      return { guard: await guardState(page) };
    });

    await run('7 close everything; the desktop is left empty', async () => {
      for (const appId of ['files', 'settings']) {
        await win(page, appId).locator('[data-window-close]').first().click();
        await expect(win(page, appId)).toHaveCount(0);
      }
      await expect.poll(() => serverWindows(page), { timeout: 20_000 }).toEqual([]);
      return { server_windows: await serverWindows(page), guard: await guardState(page) };
    });
  } finally {
    receipt.finished_at = new Date().toISOString();
    receipt.failed_step = failed;
    if (RECEIPT) fs.writeFileSync(RECEIPT, JSON.stringify(receipt, null, 2) + '\n');
    await context?.close().catch(() => {});
    await browser?.close().catch(() => {});
    await regBrowser?.close().catch(() => {});
  }
});
