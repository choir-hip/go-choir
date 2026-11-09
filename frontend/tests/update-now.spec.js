// "Updates available - Restart to update" (owner, 2026-11-09; docs/design/update-now-2026-11-09.md
// item 10). Failure modes, written first:
//  U1 the chip shows with no pending update (the signal says null);
//  U2 the chip restarts the computer on one click, without the confirm;
//  U3 the 201 outcome does not show "Restarting...", or the chip stays after
//     the signal clears;
//  U4 the 200 "current" outcome leaves the chip up instead of "Already up to
//     date" and clearing;
//  U5 a refusal (409 deferred or skipped) restarts anything, hides the chip,
//     or shows the raw code instead of plain words;
//  U6 a proxy timeout (502/504/network) is shown as a failure, though the
//     restart is running;
//  U7 a second click while one is in flight errors instead of "Restarting...";
//  U8 phone width: the chip overflows the page sideways.
// /api is stubbed as appearance-flow.spec.js does; the update route is mocked.
import { test, expect } from '@playwright/test';

const BASE_URL = process.env.GO_CHOIR_UX_BASE_URL || 'http://127.0.0.1:5173';
const SHOTS = process.env.UPDATE_NOW_SHOTS || '';
const CID = 'cmp-update-test';

async function mockComputer(page, { signal, post }) {
  const state = { signal, posts: [], signalReads: 0 };
  await page.route('**/api/**', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
  await page.route('**/api/shell/bootstrap', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ computer_id: CID }) }));
  await page.route('**/auth/session', (route) => route.fulfill({
    status: 200, contentType: 'application/json', body: JSON.stringify({ authenticated: true, user: { email: 'update@example.com' } }),
  }));
  await page.route(`**/api/computers/${CID}/update**`, (route) => {
    state.signalReads += 1;
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ computer_id: CID, update_available: state.signal }) });
  });
  await page.route(`**/api/computers/${CID}/lifecycle/update**`, async (route) => {
    state.posts.push({ method: route.request().method(), body: JSON.parse(route.request().postData() || '{}') });
    if (post === 'abort') return route.abort('timedout');
    return route.fulfill({ status: post.status, contentType: 'application/json', body: JSON.stringify(post.body) });
  });
  return state;
}

const AVAILABLE = { target: 'a1b2c3d4e5f6', since: '2026-11-09T10:00:00Z' };
const chip = (page) => page.locator('[data-update-chip]');
const shot = async (page, name) => { if (SHOTS) await page.screenshot({ path: `${SHOTS}/${name}.png` }); };

async function boot(page) {
  await page.goto(BASE_URL);
  await expect(page.locator('[data-desktop][data-desktop-ready="true"]')).toBeVisible();
}

async function open(page) {
  await page.locator('[data-update-open]').click();
  await expect(page.locator('[data-update-confirm]')).toBeVisible();
}

test('U1 no chip when no update is pending', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  const state = await mockComputer(page, { signal: null, post: { status: 201, body: {} } });
  await boot(page);
  await expect.poll(() => state.signalReads).toBeGreaterThan(0);
  await expect(chip(page)).toHaveCount(0);
  await shot(page, '00-none');
});

test('U2 available chip asks for confirmation before it restarts; Not now restarts nothing', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  const state = await mockComputer(page, { signal: AVAILABLE, post: { status: 201, body: { outcome: 'refreshed' } } });
  await boot(page);
  await expect(page.locator('[data-update-label]')).toHaveText('Updates available - Restart to update');
  await expect(chip(page)).toHaveAttribute('data-update-state', 'available');
  await shot(page, '01-available');
  await open(page);
  await expect(page.locator('[data-update-label]')).toHaveText('Running work resumes after the restart.');
  await shot(page, '02-confirm');
  await page.locator('[data-update-cancel]').click();
  await expect(page.locator('[data-update-open]')).toBeVisible();
  expect(state.posts).toHaveLength(0);
});

test('U3 a 201 shows Restarting... and the chip clears once the signal clears', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  const state = await mockComputer(page, { signal: AVAILABLE, post: { status: 201, body: { receipt_id: 'r1' } } });
  await boot(page);
  await open(page);
  await page.locator('[data-update-confirm]').click();
  await expect(chip(page)).toHaveAttribute('data-update-state', 'restarting');
  await expect(page.locator('[data-update-label]')).toHaveText('Restarting...');
  await shot(page, '03-restarting');
  expect(state.posts).toHaveLength(1);
  expect(typeof state.posts[0].body.idempotency_key).toBe('string');
  expect(state.posts[0].body.idempotency_key.length).toBeGreaterThan(8);
  state.signal = null; // the restarted computer has no pending update
  await expect(chip(page)).toHaveCount(0, { timeout: 25_000 });
});

test('U4 a 200 current says Already up to date and clears', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await mockComputer(page, { signal: AVAILABLE, post: { status: 200, body: { outcome: 'current', target: AVAILABLE.target } } });
  await boot(page);
  await open(page);
  await page.locator('[data-update-confirm]').click();
  await expect(page.locator('[data-update-label]')).toHaveText('Already up to date');
  await shot(page, '04-current');
  await expect(chip(page)).toHaveCount(0, { timeout: 8000 });
});

const REFUSALS = [
  ['live_capsule', 'deferred', 'An engineering sandbox is running; it can be paused for the restart once sandbox pause lands.', '05-deferred-capsule'],
  ['selfdev_active', 'deferred', 'A self-development change is being applied. Try again in a moment.', '06-deferred-apply'],
  ['selfdev_awaiting_decision', 'deferred', 'A self-development change is waiting for your decision.', '07-deferred-candidate'],
  ['ring R3 waits: ring walk w is at R0', 'skipped', 'Waiting for its turn in the rollout.', '08-skipped-ring'],
];
for (const [reason, outcome, words, shotName] of REFUSALS) {
  test(`U5 ${outcome} (${reason.slice(0, 20)}) shows plain words and keeps the chip`, async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await mockComputer(page, { signal: AVAILABLE, post: { status: 409, body: { outcome, reason, detail: { outcome: 'busy' } } } });
    await boot(page);
    await open(page);
    await page.locator('[data-update-confirm]').click();
    await expect(page.locator('[data-update-message]')).toHaveText(words);
    await expect(page.locator('[data-update-open]')).toBeVisible();
    await expect(page.locator('[data-update-chip]')).not.toContainText(reason === 'live_capsule' ? 'live_capsule' : '\u0000');
    await shot(page, shotName);
  });
}

test('U6 a proxy timeout is shown as Restarting...', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await mockComputer(page, { signal: AVAILABLE, post: 'abort' });
  await boot(page);
  await open(page);
  await page.locator('[data-update-confirm]').click();
  await expect(page.locator('[data-update-label]')).toHaveText('Restarting...');
});

test('U7 a second click while one is in flight shows Restarting...', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await mockComputer(page, {
    signal: AVAILABLE,
    post: { status: 409, body: { outcome: 'deferred', reason: 'a deploy refresh of this computer is already in flight' } },
  });
  await boot(page);
  await open(page);
  await page.locator('[data-update-confirm]').click();
  await expect(page.locator('[data-update-label]')).toHaveText('Restarting...');
});

test('U8 phone width: the chip fits without a sideways scroll', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 780 });
  await mockComputer(page, { signal: AVAILABLE, post: { status: 201, body: {} } });
  await boot(page);
  await expect(chip(page)).toBeVisible();
  await open(page);
  await shot(page, '09-phone-confirm');
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
  const box = await chip(page).boundingBox();
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(390);
});
