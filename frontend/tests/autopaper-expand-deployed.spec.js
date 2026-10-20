/**
 * Asking a story to be expanded, on the deployed origin (story expansion
 * design, phase 1, section 9:
 * docs/reports/story-expansion-request-design-2026-10-20.md). It needs:
 *
 *   CHOIR_DEPLOYED_BASE_URL     default https://choir.news
 *   EXPAND_ALLOWED_STATE        Playwright storage state of an account whose
 *                               user id is in PROXY_EXPAND_REQUESTERS (the
 *                               owner, or an expansion-QA account beside it)
 *   EXPAND_ORDINARY_STATE       storage state of an ordinary QA account
 *   EXPAND_API_KEY              optional: an API key of the allowlisted
 *                               account (the refusal check is skipped
 *                               without it)
 *   EXPAND_TARGET_ROUTE         the bare route of the story to expand,
 *                               /pub/texture/<slug>: the owner picks one old,
 *                               low-importance story, because a good result
 *                               publishes a real new version. Without it only
 *                               the refusal checks and the browser check run.
 *   EXPAND_WINDOW_MINUTES       how long to follow the fate; default 20
 *   AUTOPAPER_EXPAND_ARTIFACT_DIR  where to write the JSON artifact
 *
 * Ways this could fail (the design's section 7, as checked here):
 *   F3 A non-owner asks: anonymous (POST 401, read allowed:false), an
 *      ordinary account (403), an API key on the allowlisted account (403).
 *   F9 A route that is not a story answers differently from an unknown one.
 *   E5 A second ask for the same published version sends a second revise.
 *   F1 The expansion rewrites or shortens the story: the new latest version
 *      keeps at least 80% of the earlier paragraphs verbatim, cites at least
 *      as many sources, and gained words or sources. A version that loses
 *      the old story fails the run.
 *   F2 Money spent, nothing changed: valid, and recorded ("no_new_version").
 *   The page: at 375 by 812 the button is tappable (44 px), nothing scrolls
 *      sideways, and a signed-out reader has no button.
 *
 * The trace half of the design's hazard check (the first prompt Texture saw
 * must not contain "not prose to keep"; controls[] to research; cost for the
 * window) needs the paper account's credential and is an operator step:
 * `choir trace export` over the window this artifact records. This spec
 * records the window and the ids it needs.
 *
 * The restart drill (design section 9) is a separate operator step.
 *
 *   cd frontend && EXPAND_ALLOWED_STATE=... EXPAND_ORDINARY_STATE=... \
 *     EXPAND_TARGET_ROUTE=/pub/texture/<slug> \
 *     npx playwright test tests/autopaper-expand-deployed.spec.js
 */
import fs from 'node:fs';
import nodePath from 'node:path';
import { expect, request as pwRequest, test } from '@playwright/test';

const BASE_URL = (process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news').replace(/\/+$/, '');
const ALLOWED_STATE = process.env.EXPAND_ALLOWED_STATE || '';
const ORDINARY_STATE = process.env.EXPAND_ORDINARY_STATE || '';
const API_KEY = process.env.EXPAND_API_KEY || '';
const TARGET_ROUTE = process.env.EXPAND_TARGET_ROUTE || '';
const WINDOW_MS = Number(process.env.EXPAND_WINDOW_MINUTES || 20) * 60_000;
const ARTIFACT_DIR = process.env.AUTOPAPER_EXPAND_ARTIFACT_DIR || '';
const POLL_MS = 30_000;

test.describe.configure({ mode: 'serial', timeout: WINDOW_MS + 300_000 });

const record = { base_url: BASE_URL, route: TARGET_ROUTE, started_at: new Date().toISOString() };

function slugOf(route) {
  return String(route || '').replace(/^\/pub\/texture\//, '').split('/')[0];
}

async function asAccount(stateFile) {
  return pwRequest.newContext({ baseURL: BASE_URL, storageState: stateFile || { cookies: [], origins: [] } });
}

const readPath = (route) => `/api/platform/publications/expansion?route=${encodeURIComponent(route)}`;

async function bundleOf(api, route) {
  const res = await api.get(`/api/platform/publications/resolve?route=${encodeURIComponent(route)}`, { timeout: 60_000 });
  expect(res.status(), 'resolve status').toBe(200);
  return res.json();
}

function paragraphsOf(bundle) {
  const text = String(bundle?.artifact?.content || '');
  return text.split(/\n\s*\n/).map((p) => p.trim()).filter(Boolean);
}

function sourceCountOf(bundle) {
  const manifest = typeof bundle?.payload?.manifest === 'string' ? JSON.parse(bundle.payload.manifest) : bundle?.payload?.manifest || {};
  return Array.isArray(manifest.anchors) ? manifest.anchors.length : 0;
}

const wordsOf = (bundle) => String(bundle?.artifact?.content || '').split(/\s+/).filter(Boolean).length;

test.afterAll(async () => {
  if (!ARTIFACT_DIR) return;
  fs.mkdirSync(ARTIFACT_DIR, { recursive: true });
  record.finished_at = new Date().toISOString();
  fs.writeFileSync(nodePath.join(ARTIFACT_DIR, `autopaper-expand-${Date.now()}.json`), JSON.stringify(record, null, 2));
});

test('anonymous callers and ordinary accounts are refused (F3)', async () => {
  const route = TARGET_ROUTE || '/pub/texture/none';
  const anonymous = await asAccount('');
  const read = await anonymous.get(readPath(route));
  expect(read.status()).toBe(200);
  expect((await read.json()).allowed).toBe(false);
  const post = await anonymous.post('/api/platform/publications/expand', { data: { route } });
  expect(post.status(), 'anonymous POST').toBe(401);

  test.skip(!ORDINARY_STATE, 'EXPAND_ORDINARY_STATE not set: the ordinary-account check is skipped');
  const ordinary = await asAccount(ORDINARY_STATE);
  const oread = await ordinary.get(readPath(route));
  expect(oread.status()).toBe(200);
  expect((await oread.json()).allowed).toBe(false);
  expect((await ordinary.post('/api/platform/publications/expand', { data: { route } })).status(), 'ordinary POST').toBe(403);
  record.refusals = { anonymous_post: 401, ordinary_post: 403 };
});

test('an API key on the allowlisted account is refused; a non-story route is a 404 (F3, F9)', async () => {
  test.skip(!ALLOWED_STATE, 'EXPAND_ALLOWED_STATE not set');
  const allowed = await asAccount(ALLOWED_STATE);
  const unknown = await allowed.post('/api/platform/publications/expand', { data: { route: '/pub/texture/never-published-zzzzzz' } });
  const handNote = await allowed.post('/api/platform/publications/expand', { data: { route: '/somewhere/else' } });
  expect(unknown.status()).toBe(404);
  expect(handNote.status()).toBe(404);
  expect(await handNote.text(), 'unknown and non-story routes answer alike').toBe(await unknown.text());

  if (API_KEY) {
    const keyed = await pwRequest.newContext({ baseURL: BASE_URL, extraHTTPHeaders: { Authorization: `Bearer ${API_KEY}` }, storageState: { cookies: [], origins: [] } });
    const res = await keyed.post('/api/platform/publications/expand', { data: { route: TARGET_ROUTE || '/pub/texture/none' } });
    expect(res.status(), 'API key POST').toBe(403);
    record.refusals = { ...(record.refusals || {}), api_key_post: 403 };
  }
});

test.describe('the page at phone width', () => {
  test.use({ viewport: { width: 375, height: 812 } });

  test('the allowlisted reader has a tappable button and a signed-out reader has none', async ({ browser }) => {
    test.skip(!ALLOWED_STATE || !TARGET_ROUTE, 'EXPAND_ALLOWED_STATE and EXPAND_TARGET_ROUTE are needed');
    const slug = slugOf(TARGET_ROUTE);
    const context = await browser.newContext({ storageState: ALLOWED_STATE, viewport: { width: 375, height: 812 } });
    const page = await context.newPage();
    await page.goto(`${BASE_URL}/autopaper/${slug}`, { waitUntil: 'domcontentloaded', timeout: 45_000 });
    await expect(page.locator('[data-texture-published-reader]')).toBeVisible({ timeout: 30_000 });
    const button = page.locator('[data-autopaper-expand-button]');
    await expect(button).toBeVisible({ timeout: 30_000 });
    expect((await button.boundingBox()).height).toBeGreaterThanOrEqual(44);
    const overflow = await page.evaluate(() => [document.scrollingElement.scrollWidth, document.scrollingElement.clientWidth]);
    expect(overflow[0]).toBeLessThanOrEqual(overflow[1] + 1);
    await context.close();

    const anonymous = await browser.newContext({ storageState: { cookies: [], origins: [] }, viewport: { width: 375, height: 812 } });
    const apage = await anonymous.newPage();
    await apage.goto(`${BASE_URL}/autopaper/${slug}`, { waitUntil: 'domcontentloaded', timeout: 45_000 });
    await expect(apage.locator('[data-texture-published-reader]')).toBeVisible({ timeout: 30_000 });
    await apage.waitForTimeout(1500);
    await expect(apage.locator('[data-autopaper-expand]')).toHaveCount(0);
    await anonymous.close();
  });
});

test('ask once, a second ask is refused, and the story keeps what it had (E5, F1, F2)', async () => {
  test.skip(!ALLOWED_STATE || !TARGET_ROUTE, 'EXPAND_ALLOWED_STATE and EXPAND_TARGET_ROUTE are needed');
  const allowed = await asAccount(ALLOWED_STATE);
  const before = await bundleOf(allowed, TARGET_ROUTE);
  record.before = { version: before.nav?.latest, paragraphs: paragraphsOf(before).length, sources: sourceCountOf(before), words: wordsOf(before) };

  const read0 = await (await allowed.get(readPath(TARGET_ROUTE))).json();
  expect(read0.allowed, 'the allowlisted account may ask').toBe(true);

  record.asked_at = new Date().toISOString();
  const first = await allowed.post('/api/platform/publications/expand', { data: { route: TARGET_ROUTE } });
  expect(first.status(), await first.text()).toBe(202);
  const asked = await first.json();
  expect(asked.status).toBe('asked');
  expect(asked.request_id).toMatch(/^rx-[0-9a-f]{12}$/);
  record.request_id = asked.request_id;
  record.owner_revision_id = asked.revision_id;

  const second = await allowed.post('/api/platform/publications/expand', { data: { route: TARGET_ROUTE } });
  expect(second.status()).toBe(409);
  const again = await second.json();
  expect(again.status).toBe('already_asked');
  expect(again.request_id).toBe(asked.request_id);

  // Follow the fate for the window; revised and no_new_version are both valid.
  const deadline = Date.now() + WINDOW_MS;
  const fates = [];
  let state = 'asked';
  while (Date.now() < deadline) {
    const read = await (await allowed.get(readPath(TARGET_ROUTE))).json();
    const mine = (read.requests || []).find((r) => r.request_id === asked.request_id);
    state = mine?.state || 'missing';
    if (!fates.length || fates[fates.length - 1].state !== state) fates.push({ state, at: new Date().toISOString() });
    if (state === 'revised' || state === 'no_new_version' || state === 'unknown') break;
    await new Promise((resolve) => setTimeout(resolve, POLL_MS));
  }
  record.fates = fates;
  record.final_state = state;
  expect(['revised', 'no_new_version'], `the fate within ${WINDOW_MS / 60_000} minutes: ${JSON.stringify(fates)}`).toContain(state);

  const after = await bundleOf(allowed, TARGET_ROUTE);
  record.after = { version: after.nav?.latest, paragraphs: paragraphsOf(after).length, sources: sourceCountOf(after), words: wordsOf(after) };
  if (state !== 'revised') return; // an honest "no new version": nothing to compare

  const oldParagraphs = paragraphsOf(before);
  const newText = String(after?.artifact?.content || '');
  const kept = oldParagraphs.filter((p) => newText.includes(p)).length;
  record.kept_paragraphs = `${kept}/${oldParagraphs.length}`;
  expect(kept / Math.max(1, oldParagraphs.length), 'the new version keeps at least 80% of the earlier paragraphs').toBeGreaterThanOrEqual(0.8);
  expect(record.after.sources, 'the new version cites at least as many sources').toBeGreaterThanOrEqual(record.before.sources);
  expect(record.after.sources > record.before.sources || record.after.words > record.before.words, 'the new version gained sources or words').toBe(true);
});
