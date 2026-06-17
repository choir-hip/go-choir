// Read-only Texture revision-cadence probe.
//
// Submits one substantive prompt-bar request against the deployed product and
// records, with no mutation, how the Texture loop paces revisions: time to V0,
// time to the first appagent revision (V1), every subsequent revision, the total
// revision count, and the research activity (web_search / source_search /
// spawn_agent / findings) that drives them. Purpose: quantify the V1-only +
// slow-first-paint problem in docs/mission-texture-product-loop-recovery-v0.md
// before any cadence fix, and isolate any latency interaction from the
// 2026-06-17 web_search breadth changes.
//
// Product/public APIs only: /api/prompt-bar, /api/prompt-bar/submissions/{id},
// /api/texture/documents/*, /api/trace/trajectories/*. No vmctl, no internal
// routes, no writes beyond the single owner prompt.

import { chromium } from '../frontend/node_modules/playwright/index.mjs';
import { registerPasskey } from '../frontend/tests/helpers/auth.js';
import { setupVirtualAuthenticator } from '../frontend/tests/helpers/webauthn.js';

const BASE_URL = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const PROMPT =
  process.env.CHOIR_CADENCE_PROMPT ||
  "What's going on with Anthropic and the US government?";
const WINDOW_MS = Number(process.env.CHOIR_CADENCE_WINDOW_MS || 360_000);
const QUIET_STOP_MS = Number(process.env.CHOIR_CADENCE_QUIET_MS || 75_000);

const result = {
  base_url: BASE_URL,
  prompt: PROMPT,
  started_at: new Date().toISOString(),
  window_ms: WINDOW_MS,
};

function uniqueEmail() {
  return `texture-cadence-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function fetchJSON(page, path, options = {}) {
  return page.evaluate(
    async ({ requestPath, requestOptions }) => {
      const init = {
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', ...(requestOptions.headers || {}) },
        ...requestOptions,
      };
      let res = await fetch(requestPath, init);
      if (res.status === 401) {
        await fetch('/auth/session', { credentials: 'include' }).catch(() => null);
        res = await fetch(requestPath, init);
      }
      const body = await res.text();
      if (!res.ok) throw new Error(`${requestOptions.method || 'GET'} ${requestPath} -> ${res.status} ${body}`);
      return body ? JSON.parse(body) : null;
    },
    { requestPath: path, requestOptions: options },
  );
}

async function waitForDesktopReady(page, timeout = 180_000) {
  await page
    .locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]')
    .waitFor({ state: 'visible', timeout });
}

async function loadTextureState(page, docID) {
  const [doc, revisionsResponse] = await Promise.all([
    fetchJSON(page, `/api/texture/documents/${encodeURIComponent(docID)}`),
    fetchJSON(page, `/api/texture/documents/${encodeURIComponent(docID)}/revisions`),
  ]);
  return { doc, revisions: revisionsResponse.revisions || [] };
}

async function loadTrace(page, trajectoryID) {
  return fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(trajectoryID)}`);
}

async function waitForPromptDecision(page, submissionID, timeout = 180_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionID)}`);
    if (status.decision) return status.decision;
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) {
      throw new Error(`submission ${submissionID} ended ${status.state}: ${status.error || ''}`);
    }
    await page.waitForTimeout(1000);
  }
  throw new Error(`submission ${submissionID} produced no decision`);
}

function momentCounts(trace) {
  const moments = trace.moments || [];
  const count = (needle) =>
    moments.filter((m) => m.kind === 'tool.result' && String(m.summary || '').includes(needle)).length;
  return {
    web_search: count('web_search'),
    source_search: count('source_search'),
    spawn_agent: count('spawn_agent'),
    update_coagent: count('update_coagent'),
    moment_count: moments.length,
  };
}

const browser = await chromium.launch();
const context = await browser.newContext();
const page = await context.newPage();
await setupVirtualAuthenticator(page);

try {
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });
  result.email = uniqueEmail();
  await registerPasskey(page, result.email, BASE_URL);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });
  await waitForDesktopReady(page, 180_000);

  result.health = await fetchJSON(page, '/health');

  const promptBarResponse = page.waitForResponse(
    (response) => new URL(response.url()).pathname === '/api/prompt-bar' && response.request().method() === 'POST',
    { timeout: 90_000 },
  );
  const submitAt = Date.now();
  await page.locator('[data-prompt-input]').fill(PROMPT);
  await page.locator('[data-prompt-input]').press('Enter');
  const promptBar = await promptBarResponse;
  if (promptBar.status() !== 202) throw new Error(`prompt-bar status ${promptBar.status()}`);
  result.submission = await promptBar.json();
  result.submit_at = new Date(submitAt).toISOString();

  const decision = await waitForPromptDecision(page, result.submission.submission_id);
  if (decision.app !== 'texture' || !decision.doc_id) throw new Error(`unexpected decision ${JSON.stringify(decision)}`);
  const docID = decision.doc_id;
  const trajectoryID = result.submission.submission_id;
  result.doc_id = docID;
  result.decision_ms_from_submit = Date.now() - submitAt;

  const seenRevisions = new Map();
  const revisionTimeline = [];
  let lastChangeAt = Date.now();
  const deadline = submitAt + WINDOW_MS;

  while (Date.now() < deadline) {
    let state;
    let trace;
    try {
      [state, trace] = await Promise.all([loadTextureState(page, docID), loadTrace(page, trajectoryID)]);
    } catch {
      await page.waitForTimeout(2000);
      continue;
    }
    for (const rev of state.revisions) {
      if (seenRevisions.has(rev.revision_id)) continue;
      seenRevisions.set(rev.revision_id, true);
      const entry = {
        version_number: rev.version_number,
        author_kind: rev.author_kind || rev.last_author_kind || '',
        ms_from_submit: Date.now() - submitAt,
        content_chars: (rev.content || '').length,
      };
      revisionTimeline.push(entry);
      lastChangeAt = Date.now();
      console.log(`[revision] v${entry.version_number} ${entry.author_kind} +${Math.round(entry.ms_from_submit / 1000)}s chars=${entry.content_chars}`);
    }
    const live = Boolean(trace.trajectory?.live);
    const quietFor = Date.now() - lastChangeAt;
    if (!live && quietFor > QUIET_STOP_MS) break;
    await page.waitForTimeout(2500);
  }

  const [finalState, finalTrace] = await Promise.all([loadTextureState(page, docID), loadTrace(page, trajectoryID)]);
  const appagentRevisions = revisionTimeline.filter((r) => r.author_kind === 'appagent');
  result.revisions = revisionTimeline;
  result.appagent_revision_count = appagentRevisions.length;
  result.first_paint_ms = appagentRevisions.length ? appagentRevisions[0].ms_from_submit : null;
  result.total_revision_count = finalState.revisions.length;
  result.final_head_chars = (finalState.revisions.find((r) => r.revision_id === finalState.doc.current_revision_id)?.content || '').length;
  result.research = momentCounts(finalTrace);
  result.trajectory = {
    state: finalTrace.trajectory?.state,
    live: Boolean(finalTrace.trajectory?.live),
    search_attempt_count: finalTrace.trajectory?.search_attempt_count,
    search_success_count: finalTrace.trajectory?.search_success_count,
    agent_count: finalTrace.trajectory?.agent_count,
    delegation_count: finalTrace.trajectory?.delegation_count,
  };
  result.finished_at = new Date().toISOString();

  console.log('\n=== TEXTURE CADENCE RESULT ===');
  console.log(JSON.stringify(result, null, 2));
} catch (error) {
  result.error = String(error);
  console.log('\n=== TEXTURE CADENCE RESULT (ERROR) ===');
  console.log(JSON.stringify(result, null, 2));
  process.exitCode = 1;
} finally {
  await browser.close();
}
