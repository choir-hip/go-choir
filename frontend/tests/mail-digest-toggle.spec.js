/**
 * The owner's opt-in toggle for the Autopaper daily digest, in the Mail app
 * (docs/reports/autopaper-daily-digest-design-2026-10-20.md, section 7;
 * docs/problems/autopaper-daily-digest-phase1-2026-10-20.md). Runs against the
 * built frontend (vite preview) with the session and the email routes mocked;
 * the server side is covered by the Go tests and the deployed spec. Written
 * before the toggle.
 *
 * Ways it can fail:
 *   G1 The toggle shows to a signed-out visitor, who has no digest to manage.
 *   G2 Nothing is opted in by default: the switch starts Off when the server
 *      says not subscribed, and opening the Mail app alone never calls the
 *      digest route (state is read only when the panel opens, so the mailbox
 *      bootstrap stays one aliases and one mailbox request).
 *   G3 Turning it on sends PUT and off sends DELETE to /api/email/digest, and
 *      the panel shows the server's reply, not the click.
 *   G4 A refused PUT leaves the switch showing On.
 *   G5 A server without a Resend key looks healthy: the panel must say the
 *      sender is not configured.
 *   G6 The panel shows the full address instead of the masked one.
 *
 *   cd frontend && npm run build && npx vite preview --port 4173 --strictPort &
 *   npx playwright test tests/mail-digest-toggle.spec.js
 */
import { expect, test } from '@playwright/test';

const BASE_URL = (process.env.PLAYWRIGHT_BASE_URL || 'http://localhost:4173').replace(/\/+$/, '');

const JSON_HEADERS = { 'content-type': 'application/json' };

function digestState(overrides = {}) {
  return {
    enabled: true,
    subscribed: false,
    address_masked: 'o***@example.com',
    sender_configured: true,
    schedule: '12:00 UTC daily',
    last_run: null,
    recent_runs: [],
    ...overrides,
  };
}

async function mockSignedIn(page, { authenticated = true } = {}) {
  await page.route('**/auth/**', async (route) => {
    const pathname = new URL(route.request().url()).pathname;
    const body = pathname === '/auth/session'
      ? (authenticated ? { authenticated: true, user: { email: 'owner@example.com' } } : { authenticated: false })
      : { ok: true };
    await route.fulfill({ status: 200, headers: JSON_HEADERS, body: JSON.stringify(body) });
  });
  await page.route('**/health', (route) => route.fulfill({ status: 200, headers: JSON_HEADERS, body: '{"status":"ok"}' }));
}

// The email routes: aliases, mailbox lists, and a digest server whose state
// the test controls. Every digest request is recorded.
async function mockEmail(page, digest, { putStatus = 200, calls = [] } = {}) {
  const requests = [];
  await page.route('**/api/**', async (route) => {
    const url = new URL(route.request().url());
    const method = route.request().method();
    if (url.pathname === '/api/email/digest') {
      calls.push(method);
      if (method === 'PUT' && putStatus !== 200) {
        await route.fulfill({ status: putStatus, headers: JSON_HEADERS, body: '{"error":"owner session required"}' });
        return;
      }
      if (method === 'PUT') digest.subscribed = true;
      if (method === 'DELETE') digest.subscribed = false;
      await route.fulfill({ status: 200, headers: JSON_HEADERS, body: JSON.stringify(digest) });
      return;
    }
    requests.push(`${method} ${url.pathname}`);
    let body = {};
    if (url.pathname === '/api/email/aliases') body = { aliases: [{ address: 'owner@choir.news' }] };
    else if (url.pathname === '/api/email/messages') body = { messages: [], total: 0, unread: 0 };
    else if (url.pathname === '/api/email/drafts') body = { drafts: [] };
    else if (url.pathname === '/api/preferences/theme') body = { theme: {} };
    else if (url.pathname === '/api/shell/bootstrap') body = { computer_id: 'digest-test-computer' };
    await route.fulfill({ status: 200, headers: JSON_HEADERS, body: JSON.stringify(body) });
  });
  return requests;
}

async function openMail(page) {
  await page.goto(BASE_URL);
  await page.locator('[data-desktop-icon-id="email"]').click({ timeout: 20_000 });
  const mail = page.locator('[data-mail-app]').last();
  await expect(mail).toBeVisible({ timeout: 10_000 });
  return mail;
}

test('signed out: no digest toggle (G1)', async ({ page }) => {
  await mockSignedIn(page, { authenticated: false });
  const calls = [];
  await mockEmail(page, digestState(), { calls });
  const mail = await openMail(page);
  await expect(mail.locator('[data-mail-folder="inbox"]').first()).toBeVisible();
  await expect(mail.locator('[data-mail-digest-toggle]')).toHaveCount(0);
  expect(calls).toEqual([]);
});

test('starts Off, is lazy, and turns on and off with PUT and DELETE (G2, G3, G6)', async ({ page }) => {
  await mockSignedIn(page);
  const calls = [];
  const digest = digestState();
  await mockEmail(page, digest, { calls });
  const mail = await openMail(page);

  // The mailbox is open; the digest route has not been called (G2).
  await expect(mail.locator('[data-mail-digest-toggle]')).toBeVisible();
  await page.waitForTimeout(500);
  expect(calls).toEqual([]);

  await mail.locator('[data-mail-digest-toggle]').click();
  const panel = mail.locator('[data-mail-digest]');
  await expect(panel).toBeVisible();
  const sw = panel.locator('[data-mail-digest-switch]');
  await expect(sw).not.toBeChecked();
  await expect(panel).toContainText('o***@example.com');
  await expect(panel).not.toContainText('owner@example.com');
  await expect(panel).toContainText('No digest sent yet.');
  expect(calls).toEqual(['GET']);

  await sw.check();
  await expect(sw).toBeChecked();
  expect(calls).toEqual(['GET', 'PUT']);
  await expect(panel).toContainText('On');

  digest.last_run = { run_date: '2026-10-21', kind: 'scheduled', status: 'sent' };
  await sw.uncheck();
  await expect(sw).not.toBeChecked();
  expect(calls).toEqual(['GET', 'PUT', 'DELETE']);
  await expect(panel).toContainText('Last run 2026-10-21: sent.');
});

test('a refused subscribe does not show On (G4)', async ({ page }) => {
  await mockSignedIn(page);
  const calls = [];
  await mockEmail(page, digestState(), { putStatus: 403, calls });
  const mail = await openMail(page);
  await mail.locator('[data-mail-digest-toggle]').click();
  const sw = mail.locator('[data-mail-digest-switch]');
  await expect(sw).not.toBeChecked();
  await sw.check();
  await expect(mail.locator('[data-mail-digest-error]')).toContainText('owner session required');
  await expect(sw).not.toBeChecked();
});

test('says so when the mail sender is not configured (G5)', async ({ page }) => {
  await mockSignedIn(page);
  await mockEmail(page, digestState({ sender_configured: false, last_run: { run_date: '2026-10-21', kind: 'scheduled', status: 'failed', reason: 'provider: RESEND_API_KEY is not configured' } }));
  const mail = await openMail(page);
  await mail.locator('[data-mail-digest-toggle]').click();
  await expect(mail.locator('[data-mail-digest-warn]')).toContainText('not configured');
  await expect(mail.locator('[data-mail-digest-last]')).toContainText('failed');
});
