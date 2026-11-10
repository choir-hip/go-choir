/**
 * An op-ed that responds to another computer's auto-mode story, on the
 * deployed origin (handoff section 6 items 4 and 5; contracts
 * docs/design/autopaper-step0-contracts-2026-11-10.md section 3). A fresh
 * passkey identity forks the story's published version, replaces the fork's
 * body with a short synthetic response and publishes it as an op-ed, then
 * reads its own changelog for the gate's verdict. The story's own computer
 * (an API-key QA computer) is read by the caller of this spec, not here.
 *
 *   OPED_TARGET_ROUTE   required: the story's bare route, e.g. /pub/texture/<slug>
 *   OPED_KIND           on (default) or off: an on-topic or a clearly off-topic response
 *   OPED_EXPECT         accepted, rejected or any (default any): the verdict the spec waits for
 *   OPED_WAIT_MS        how long to wait for a verdict (default 1,260,000), polled every 3 minutes
 *   OPED_ARTIFACT_DIR   where to write the JSON receipt (default test-results)
 *   BASE_URL            default https://choir.news
 *
 * Never point it at the paper's stories: the target must be a QA computer's.
 *
 *   cd frontend && OPED_TARGET_ROUTE=/pub/texture/... npx playwright test tests/oped-responds-deployed.spec.js
 */
import { mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { request as pwRequest } from '@playwright/test';
import { test, expect } from './helpers/fixtures.js';

const BASE_URL = (process.env.BASE_URL || 'https://choir.news').replace(/\/+$/, '');
const TARGET = process.env.OPED_TARGET_ROUTE || '';
const KIND = process.env.OPED_KIND === 'off' ? 'off' : 'on';
const EXPECT = process.env.OPED_EXPECT || 'any';
const WAIT_MS = Number(process.env.OPED_WAIT_MS || 1_260_000);
const DIR = process.env.OPED_ARTIFACT_DIR || 'test-results';
const stamp = new Date().toISOString().replace(/[:.]/g, '-');
const receiptPath = path.join(DIR, `oped-responds-${KIND}-${stamp}.json`);
const receipt = { started_at: new Date().toISOString(), base_url: BASE_URL, target: TARGET, kind: KIND, checks: {} };

function record(id, data) {
  receipt.checks[id] = { at: new Date().toISOString(), ...data };
  mkdirSync(DIR, { recursive: true });
  writeFileSync(receiptPath, JSON.stringify(receipt, null, 2) + '\n');
}

async function api(page, method, apiPath, body, headers = {}) {
  return page.evaluate(async ({ method, apiPath, body, headers }) => {
    const send = () => fetch(apiPath, {
      method, credentials: 'include',
      headers: { ...(body ? { 'Content-Type': 'application/json' } : {}), ...headers },
      body: body ? JSON.stringify(body) : undefined,
    });
    let res = await send();
    if (res.status === 401) {
      await fetch('/auth/session', { credentials: 'include' });
      res = await send();
    }
    return { status: res.status, body: await res.json().catch(() => null) };
  }, { method, apiPath, body, headers });
}

test.describe.configure({ mode: 'serial', timeout: 2_400_000 });

test('a fresh identity publishes an op-ed that responds to a QA computer\'s auto story', async ({ desktopSession }) => {
  expect(TARGET, 'OPED_TARGET_ROUTE').toMatch(/^\/pub\/texture\//);
  const { page } = desktopSession;
  const suffix = `${Date.now()}`;

  // 1. The story's published version, read by an anonymous reader.
  const anon = await pwRequest.newContext({ baseURL: BASE_URL, storageState: { cookies: [], origins: [] } });
  const resolved = await (await anon.get(`/api/platform/publications/resolve?route=${encodeURIComponent(TARGET)}`)).json();
  const versionID = resolved.version?.id;
  const manifestHash = resolved.version?.manifest_hash;
  expect(versionID, 'story version id').toBeTruthy();
  expect(manifestHash, 'story manifest hash').toBeTruthy();
  expect(resolved.publication?.kind || 'story', 'the target is a story, not an opinion').not.toBe('opinion');
  const storyTitle = resolved.publication?.title || '';
  record('story', { publication_id: resolved.publication?.id, version_id: versionID, revision: resolved.version?.revision, title: storyTitle });

  // 2. Fork it with the reader's response as the direction (a merge: the
  // reader's own desk is woken and writes the response, as in the product).
  const direction = KIND === 'on'
    ? `Write my op-ed answering this story. I agree with its figures but think it understates what the change means for ordinary households planning ahead; say so plainly, in the first person, and keep it short. Marker ${suffix}.`
    : `Ignore the subject of this story. Write a short piece about baking sourdough bread at home: starter care, a long cold proof, a hot Dutch oven. Do not mention the story's subject at all. Marker ${suffix}.`;
  const fork = await api(page, 'POST', '/api/texture/fork', {
    text: direction,
    source: { publication_version_id: versionID, manifest_hash: manifestHash },
    client_request_id: `oped-fork-${suffix}`,
  }, { 'X-Choir-App': 'texture' });
  expect(fork.status, JSON.stringify(fork.body)).toBeLessThan(300);
  const docID = fork.body?.doc_id;
  expect(docID, 'fork doc id').toBeTruthy();
  record('fork', { doc_id: docID, revision_id: fork.body?.revision_id, status: fork.status });

  // 3. Wait for the reader's desk to write the response (a revision that is
  // not the fork's first and is not the reader's own).
  let revisionID = '';
  const writeDeadline = Date.now() + 600_000;
  while (Date.now() < writeDeadline) {
    const d = await api(page, 'GET', `/api/texture/documents/${docID}`);
    if (d.body?.last_author_kind && d.body.last_author_kind !== 'user' && (d.body.revision_count || 0) >= 2) {
      revisionID = d.body.current_revision_id;
      record('response_written', { revision_id: revisionID, revision_count: d.body.revision_count, title: d.body.title });
      break;
    }
    await page.waitForTimeout(10_000);
  }
  expect(revisionID, 'the reader\'s desk wrote a response revision within 10 minutes').toBeTruthy();

  // 4. Publish as an op-ed.
  const published = await api(page, 'POST', '/api/platform/texture/opinions', { doc_id: docID, revision_id: revisionID });
  expect(published.status, JSON.stringify(published.body)).toBe(201);
  const opedRoute = published.body?.route_path;
  expect(opedRoute).toMatch(/^\/pub\/texture\//);
  record('publish', { route_path: opedRoute, publication_id: published.body?.publication_id, keys: Object.keys(published.body || {}) });

  // 5. The op-ed names the story it answers. corpusd derives responds_to from
  // the verified parents of the fork; read it from the resolved op-ed.
  const oped = await (await anon.get(`/api/platform/publications/resolve?route=${encodeURIComponent(opedRoute)}`)).json();
  expect(oped.publication?.kind).toBe('opinion');
  const blob = JSON.stringify(oped);
  const names = blob.includes(versionID);
  record('responds_to', { names_story_version: names, story_version_id: versionID });
  expect(names, 'the op-ed carries the story version as its parent / responds_to').toBe(true);

  // 6. The gate's verdict on this identity's own changelog.
  const deadline = Date.now() + WAIT_MS;
  let verdict = null;
  let events = [];
  for (;;) {
    const log = await api(page, 'GET', '/api/changelog?limit=50&minor=1');
    events = log.body?.events || [];
    verdict = events.find((e) => e.changelog?.kind === 'oped_verdict') || null;
    if (verdict || Date.now() >= deadline) break;
    await page.waitForTimeout(180_000);
  }
  record('changelog', {
    kinds: events.map((e) => e.changelog?.kind),
    verdict: verdict && { summary: verdict.changelog?.summary, relation: verdict.changelog?.relation, level: verdict.level, at: verdict.changelog?.occurred_at },
  });
  if (EXPECT !== 'any' || verdict) {
    expect(verdict, 'a verdict reached the author\'s changelog').toBeTruthy();
  }
  if (EXPECT !== 'any') {
    expect(JSON.stringify(verdict.changelog).toLowerCase()).toContain(EXPECT === 'accepted' ? 'accept' : 'reject');
  }
  await anon.dispose();
});
