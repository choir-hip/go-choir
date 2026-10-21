/**
 * Op-ed end to end on the deployed origin
 * (docs/problems/oped-not-connected-2026-10-20.md): a signed-in test account
 * writes a short piece in Texture, publishes it as opinion, and a logged-out
 * reader finds it under Opinion with a byline and the label, and not in the
 * news. It uses the same fixture as the Texture acceptance suite
 * (tests/helpers/fixtures.js desktopSession: a fresh passkey account on the
 * target origin), so it needs no stored credentials.
 *
 *   CHOIR_DEPLOYED_BASE_URL   default https://choir.news
 *   OPED_ARTIFACT_DIR         where to write the JSON receipt
 *                             (default <repo>/docs/evidence)
 *
 * Needs CORPUSD_OPINION_LISTING=on on the target (nix/node-b.nix sets it). With
 * the switch off the publish answers 409 and this spec fails on purpose: the
 * deploy did not turn the feature on.
 *
 * Ways this could fail (the problem doc's failure modes, as checked here):
 *   F1 The piece is on the news front, or its page carries the AI disclosure.
 *   F2 An anonymous caller can publish to Opinion.
 *   F4 The piece is not listed under Opinion for a logged-out reader.
 *   F9 The byline is missing, or looks like an email.
 *
 *   cd frontend && npx playwright test tests/oped-deployed.spec.js
 */
import { mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { request as pwRequest } from '@playwright/test';
import { test, expect } from './helpers/fixtures.js';

const BASE_URL = (process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news').replace(/\/+$/, '');
const here = path.dirname(fileURLToPath(import.meta.url));
const stamp = new Date().toISOString().replace(/[:.]/g, '-');
const artifactDir = process.env.OPED_ARTIFACT_DIR || path.resolve(here, '../../docs/evidence');
const receiptPath = path.join(artifactDir, `oped-deployed-${stamp}.json`);
const receipt = { started_at: new Date().toISOString(), base_url: BASE_URL, checks: {} };

function record(id, data) {
  receipt.checks[id] = { at: new Date().toISOString(), ...data };
  mkdirSync(path.dirname(receiptPath), { recursive: true });
  writeFileSync(receiptPath, JSON.stringify(receipt, null, 2) + '\n');
}

// Access tokens live 5 minutes; like the editor, a 401 renews the session
// through GET /auth/session and retries once.
async function api(page, method, apiPath, body) {
  return page.evaluate(async ({ method, apiPath, body }) => {
    const send = () => fetch(apiPath, {
      method,
      credentials: 'include',
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    });
    let res = await send();
    if (res.status === 401) {
      await fetch('/auth/session', { credentials: 'include' });
      res = await send();
    }
    const json = await res.json().catch(() => null);
    return { status: res.status, body: json };
  }, { method, apiPath, body });
}

test.describe.configure({ mode: 'serial', timeout: 300_000 });

test('a signed-in account publishes an op-ed that a logged-out reader finds under Opinion', async ({ desktopSession }) => {
  const { page, context } = desktopSession;
  const suffix = `${Date.now()}`;
  const title = `Oped QA ${suffix}`;
  const sentence = `I think the harbor matters more than the bridge, said the QA writer ${suffix}.`;

  // 1. Write a short piece: a new Texture document with the writer's own text.
  const created = await api(page, 'POST', '/api/texture/lifecycle-documents', {
    title,
    initial_content: sentence,
    client_request_id: `oped-create-${suffix}`,
  });
  expect(created.status, JSON.stringify(created.body)).toBe(201);
  const docID = created.body?.doc_id;
  expect(docID, 'created doc id').toBeTruthy();
  const doc = await api(page, 'GET', `/api/texture/documents/${docID}`);
  expect(doc.status).toBe(200);
  const revisionID = doc.body?.current_revision_id;
  expect(revisionID, 'current revision id').toBeTruthy();
  record('write', { doc_id: docID, revision_id: revisionID, title });

  // 2. F2: an anonymous caller cannot publish to Opinion.
  const anon = await pwRequest.newContext({ baseURL: BASE_URL, storageState: { cookies: [], origins: [] } });
  const refused = await anon.post('/api/platform/texture/opinions', { data: { doc_id: docID, revision_id: revisionID } });
  expect(refused.status(), 'anonymous opinion publish').toBe(401);
  record('F2_anonymous_refused', { status: refused.status() });

  // 3. Publish as opinion with the signed-in session.
  const published = await api(page, 'POST', '/api/platform/texture/opinions', { doc_id: docID, revision_id: revisionID });
  expect(published.status, `publish as opinion: ${JSON.stringify(published.body)} (409 means CORPUSD_OPINION_LISTING is off)`).toBe(201);
  const routePath = published.body?.route_path;
  expect(routePath, 'route path').toMatch(/^\/pub\/texture\//);
  const slug = routePath.replace('/pub/texture/', '');
  record('publish', { route_path: routePath, public_url: published.body?.public_url, publication_id: published.body?.publication_id });

  // 4. A logged-out reader finds it under Opinion, by API.
  const opinion = await (await anon.get('/api/platform/publications/front?section=opinion&limit=50')).json();
  const row = (opinion.stories || []).find((s) => s.route?.bare === routePath);
  expect(row, 'the piece is listed under Opinion').toBeTruthy();
  expect(row.kind).toBe('opinion');
  expect(row.title).toBe(title);
  expect(row.author, 'byline').toMatch(/^writer-[0-9a-f]{6}$/);
  expect(row.author).not.toContain('@');
  record('F4_F9_listed', { author: row.author, kind: row.kind });

  // 5. F1: it is not in the news front, and its page names the kind.
  const news = await (await anon.get('/api/platform/publications/front?limit=100')).json();
  expect((news.stories || []).some((s) => s.route?.bare === routePath), 'the piece is on the news front').toBe(false);
  const resolved = await (await anon.get(`/api/platform/publications/resolve?route=${encodeURIComponent(routePath)}`)).json();
  expect(resolved.publication?.kind).toBe('opinion');
  expect(resolved.publication?.author_label).toBe(row.author);
  record('F1_not_news', { resolved_kind: resolved.publication?.kind });

  // 6. The same, in a logged-out browser: the front's Opinion section and the story page.
  const reader = await context.browser().newContext({ storageState: { cookies: [], origins: [] } });
  try {
    const front = await reader.newPage();
    await front.goto(`${BASE_URL}/autopaper`, { waitUntil: 'domcontentloaded' });
    const section = front.locator('[data-autopaper-opinion]');
    await expect(section).toBeVisible({ timeout: 60_000 });
    await expect(section.locator('[data-autopaper-opinion-heading]')).toHaveText('Opinion');
    const rowLocator = section.locator(`[data-publication-route="${routePath}"]`);
    await expect(rowLocator).toBeVisible();
    await expect(rowLocator).toContainText(title);
    await expect(rowLocator.locator('[data-autopaper-opinion-label]')).toHaveText('Opinion');
    await expect(rowLocator.locator('[data-autopaper-byline]')).toContainText(row.author);
    await expect(front.locator('[data-autopaper-grid]')).not.toContainText(title);

    const pagePath = `${BASE_URL}/autopaper/${slug}`;
    const story = await reader.newPage();
    await story.goto(pagePath, { waitUntil: 'domcontentloaded' });
    const article = story.locator('[data-texture-published-reader]');
    await expect(article).toBeVisible({ timeout: 60_000 });
    await expect(article.locator('[data-texture-opinion-label]')).toContainText('Opinion');
    await expect(article.locator('[data-texture-opinion-label]')).toContainText(row.author);
    await expect(article.locator('[data-texture-opinion-notice]')).toBeVisible();
    await expect(article.locator('[data-texture-ai-disclosure]')).toHaveCount(0);
    await expect(article).toContainText(String(suffix));
    record('browser_logged_out', { front_listed: true, page: pagePath });
  } finally {
    await reader.close();
  }

  // Research's own path (choir.SearchWireCorpus -> the guest-only published
  // search) is not reachable from a browser; the proxy carve-out and the tool
  // are covered by internal/proxy and internal/researchtools tests.
  receipt.finished_at = new Date().toISOString();
  record('done', { route_path: routePath });
  await anon.dispose();
});
