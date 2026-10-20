/**
 * Deployed acceptance for the Autopaper daily digest, phase 1 (design
 * docs/reports/autopaper-daily-digest-design-2026-10-20.md section 10;
 * docs/problems/autopaper-daily-digest-phase1-2026-10-20.md). It targets
 * staging (https://choir.news) AFTER the landing loop: commit, push, CI,
 * deploy, staging identity. It has NOT been run: the work that wrote it ran
 * offline and never pushed.
 *
 * Account: a fresh passkey account registered as delivered+digest<run>@resend.dev,
 * Resend's own sink address, so a real send-now goes nowhere a person has to
 * read and does not bounce at a made-up domain. The spec never signs in as the
 * owner and never sets MAILD_DIGEST or any server state. It sends real mail
 * once (send-now) to that sink.
 *
 * Failure modes it can show (each is an assertion below):
 *   A1 The digest routes are readable or writable without a session, or an API
 *      key can opt an account in (API key only when DIGEST_QA_API_KEY is set).
 *   A2 Subscribing does not record the opt-in, or the status leaks the full
 *      address instead of the masked one.
 *   A3 The preview lists a story that is not on the live front list, a
 *      promotional or known one-source story, more than 3 from a section,
 *      fewer than 3 or more than 8 stories, or an order unlike the front
 *      page's own rank (rankFrontStories in frontend/src/lib/autopaper.ts).
 *   A4 The rendered text carries a script, an image tag, an http: link, or a
 *      link that is not https://choir.news/autopaper/<slug> (the manage link
 *      aside).
 *   A5 send-now sends without a provider message id, is not limited to one an
 *      hour, or sends after the owner unsubscribed.
 *   A6 (next day) the host timer did not run: the account's last run is not a
 *      scheduled, sent run for today's UTC date.
 *
 * Run (after deploy):
 *   BASE_URL=https://choir.news npx playwright test tests/autopaper-digest-deployed.spec.js
 * To arm the scheduled-run check, run once with DIGEST_ARM_SCHEDULED=1 (it
 * leaves the QA account opted in and saves its session to
 * playwright/.auth/digest-qa.storage.json); the next day after 12:00 UTC run
 *   BASE_URL=https://choir.news DIGEST_CHECK_SCHEDULED=1 npx playwright test tests/autopaper-digest-deployed.spec.js -g scheduled
 *
 * The artifact (attached to the test and written under test-results/) holds the
 * front list read, the preview story ids with their ranks, the send result,
 * the run rows, and the rendered text. Re-running the spec reproduces it.
 *
 * Human step, not automated: the owner opens the first real digest in the
 * inbox, uses "Show original", and confirms SPF, DKIM and DMARC pass and which
 * folder it landed in. Record that in the final report.
 */
import fs from 'node:fs';
import path from 'node:path';
import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';
import { rankFrontStories, canLead } from '../src/lib/autopaper.ts';

const BASE_URL = (process.env.BASE_URL || 'https://choir.news').replace(/\/+$/, '');
const RUN_ID = `${Date.now().toString(36)}${Math.floor(Math.random() * 1296).toString(36)}`;
const QA_EMAIL = `delivered+digest${RUN_ID}@resend.dev`;
const STATE_PATH = path.resolve('playwright', '.auth', 'digest-qa.storage.json');
const DIGEST = '/api/email/digest';
const LINK_OK = /^https:\/\/choir\.news\/autopaper\/[A-Za-z0-9._~-]+$/;
const MANAGE_OK = /^https:\/\/choir\.news\/\?app=email(&digest=off)?$/;

async function api(page, route, options = {}) {
  return page.evaluate(
    async ({ route, options }) => {
      const res = await fetch(route, { credentials: 'include', ...options });
      const text = await res.text();
      let body = text;
      try {
        body = JSON.parse(text);
      } catch (_e) {
        /* not JSON */
      }
      return { status: res.status, body };
    },
    { route, options },
  );
}

test('deployed digest: opt-in, preview, send-now, opt-out (A1 to A5)', async ({ page, authenticator, browser }, testInfo) => {
  test.setTimeout(300_000);
  const artifact = { base_url: BASE_URL, qa_email: QA_EMAIL, started_at: new Date().toISOString() };

  // A1: a signed-out visitor sees 401 on every digest route.
  const anon = await browser.newContext();
  try {
    const anonPage = await anon.newPage();
    await anonPage.goto(BASE_URL);
    expect((await api(anonPage, DIGEST)).status).toBe(401);
    expect((await api(anonPage, DIGEST, { method: 'PUT' })).status).toBe(401);
    expect((await api(anonPage, `${DIGEST}/send-now?preview=1`, { method: 'POST' })).status).toBe(401);
  } finally {
    await anon.close();
  }
  if (process.env.DIGEST_QA_API_KEY) {
    // An API key (which an agent inside a computer may hold) cannot opt in.
    const keyed = await browser.newContext();
    try {
      const res = await keyed.request.put(`${BASE_URL}${DIGEST}`, {
        headers: { Authorization: `Bearer ${process.env.DIGEST_QA_API_KEY}` },
      });
      expect(res.status()).toBe(403);
    } finally {
      await keyed.close();
    }
  } else {
    testInfo.annotations.push({ type: 'note', description: 'DIGEST_QA_API_KEY not set: API-key refusal not checked here (covered by the proxy Go test)' });
  }

  // A fresh account, signed in.
  await page.goto(BASE_URL);
  const reg = await registerPasskey(page, QA_EMAIL, BASE_URL);
  expect(reg.ok || reg.user).toBeTruthy();
  await page.reload();
  await page.locator('[data-desktop][data-authenticated="true"]').waitFor({ state: 'visible', timeout: 120_000 });

  // A2: subscribe, then the status shows it with a masked address.
  const before = await api(page, DIGEST);
  expect(before.status).toBe(200);
  expect(before.body.subscribed).toBe(false); // nobody is opted in by default
  expect(before.body.sender_configured, 'the Resend key must be configured on the host').toBe(true);
  const put = await api(page, DIGEST, { method: 'PUT' });
  expect(put.status).toBe(200);
  const status = await api(page, DIGEST);
  expect(status.body.subscribed).toBe(true);
  expect(status.body.address_masked).toMatch(/^d\*\*\*@resend\.dev$/);
  expect(JSON.stringify(status.body)).not.toContain(QA_EMAIL);
  artifact.status_after_subscribe = status.body;

  // A3: the preview against the live front list.
  const frontRes = await api(page, '/api/platform/publications/front?limit=60');
  expect(frontRes.status).toBe(200);
  const front = frontRes.body.stories || [];
  artifact.front_list = front.map((s) => ({ id: s.publication_id, title: s.title, updated_at: s.updated_at, importance: s.importance, section: s.section, source_count: s.source_count, promo: s.promo }));
  const preview = await api(page, `${DIGEST}/send-now?preview=1`, { method: 'POST' });
  expect(preview.status).toBe(200);
  expect(preview.body.preview).toBe(true);
  expect(preview.body.status, 'a thin front list makes a skipped preview (too_few); re-run when the paper has stories').toBe('ok');
  const stories = preview.body.stories;
  expect(stories.length).toBeGreaterThanOrEqual(3);
  expect(stories.length).toBeLessThanOrEqual(8);
  const byId = new Map(front.map((s) => [s.publication_id, s]));
  const perSection = {};
  for (const s of stories) {
    const row = byId.get(s.publication_id);
    expect(row, `${s.publication_id} must be on the live front list`).toBeTruthy();
    expect(row.promo, `${s.publication_id} is promotional`).not.toBe(true);
    if (row.source_count !== undefined && row.source_count !== null) {
      expect(row.source_count, `${s.publication_id} cites fewer than two sources`).toBeGreaterThanOrEqual(2);
    }
    if (row.section) perSection[row.section] = (perSection[row.section] || 0) + 1;
  }
  for (const [section, n] of Object.entries(perSection)) expect(n, `section ${section}`).toBeLessThanOrEqual(3);
  // Order: the digest's stories keep the front page's own relative order.
  const pageOrder = rankFrontStories(front).map((s) => s.publication_id);
  const digestOrder = stories.map((s) => s.publication_id);
  const positions = digestOrder.map((id) => pageOrder.indexOf(id));
  expect(positions, 'digest order follows the front-page rank').toEqual([...positions].sort((a, b) => a - b));
  expect(canLead(byId.get(digestOrder[0])), 'the lead may lead').toBe(true);
  artifact.preview = { ranked_by: preview.body.ranked_by, subject: preview.body.subject, stories: stories.map((s) => ({ id: s.publication_id, rank: s.rank, link: s.link })) };

  // A4: the rendered text.
  const text = preview.body.text;
  expect(text).not.toMatch(/<script/i);
  expect(text).not.toMatch(/<img/i);
  expect(text).not.toMatch(/http:/i);
  for (const url of text.match(/https:\/\/[^\s"'<>)]+/g) || []) {
    expect(LINK_OK.test(url) || MANAGE_OK.test(url), `unexpected link ${url}`).toBe(true);
  }
  for (const s of stories) expect(s.link).toMatch(LINK_OK);
  artifact.rendered_text = text;

  // A5: send-now sends once, is limited, and stops after opt-out.
  const sent = await api(page, `${DIGEST}/send-now`, { method: 'POST' });
  expect(sent.status).toBe(200);
  expect(sent.body.status).toBe('sent');
  expect(sent.body.provider_message_id).toBeTruthy();
  artifact.send_now = sent.body;
  expect((await api(page, `${DIGEST}/send-now`, { method: 'POST' })).status).toBe(429);
  const afterSend = await api(page, DIGEST);
  expect(afterSend.body.last_run.kind).toBe('manual');
  expect(afterSend.body.last_run.status).toBe('sent');
  artifact.runs = afterSend.body.recent_runs;

  if (process.env.DIGEST_ARM_SCHEDULED === '1') {
    // Leave the account opted in for tomorrow's 12:00 UTC timer run.
    fs.mkdirSync(path.dirname(STATE_PATH), { recursive: true });
    await page.context().storageState({ path: STATE_PATH });
    artifact.armed_for_scheduled_run = true;
  } else {
    const del = await api(page, DIGEST, { method: 'DELETE' });
    expect(del.status).toBe(200);
    expect(del.body.subscribed).toBe(false);
    expect((await api(page, `${DIGEST}/send-now`, { method: 'POST' })).status).toBe(409);
  }

  const out = testInfo.outputPath('digest-acceptance.json');
  fs.writeFileSync(out, JSON.stringify(artifact, null, 2));
  await testInfo.attach('digest-acceptance.json', { path: out, contentType: 'application/json' });
});

test('deployed digest: the host timer sent today\'s scheduled digest (A6)', async ({ browser }, testInfo) => {
  test.skip(process.env.DIGEST_CHECK_SCHEDULED !== '1', 'set DIGEST_CHECK_SCHEDULED=1 the day after a DIGEST_ARM_SCHEDULED=1 run');
  expect(fs.existsSync(STATE_PATH), `missing ${STATE_PATH}; arm first`).toBe(true);
  const context = await browser.newContext({ storageState: STATE_PATH });
  try {
    const page = await context.newPage();
    await page.goto(BASE_URL);
    const status = await api(page, DIGEST);
    expect(status.status).toBe(200);
    const today = new Date().toISOString().slice(0, 10);
    const scheduled = (status.body.recent_runs || []).find((r) => r.kind === 'scheduled' && r.run_date === today);
    expect(scheduled, 'no scheduled run for today (UTC): did the timer fire after 12:00 UTC?').toBeTruthy();
    expect(scheduled.status, JSON.stringify(scheduled)).toBe('sent');
    expect(scheduled.provider_message_id).toBeTruthy();
    const out = testInfo.outputPath('digest-scheduled.json');
    fs.writeFileSync(out, JSON.stringify({ today, scheduled, recent_runs: status.body.recent_runs }, null, 2));
    await testInfo.attach('digest-scheduled.json', { path: out, contentType: 'application/json' });
  } finally {
    await context.close();
  }
});
