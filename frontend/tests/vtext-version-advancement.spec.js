import fs from 'fs';
import path from 'path';
import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = process.env.PLAYWRIGHT_BASE_URL || 'https://choir.news';
const EVIDENCE_DIR = path.resolve(
  process.env.VTEXT_REPRO_EVIDENCE_DIR || '../test-results/vtext-version-advancement-repro',
);

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(360_000);

function uniqueEmail() {
  return `vtext-regression-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function fetchJSON(page, requestPath, options = {}) {
  return page.evaluate(async ({ requestPath, options }) => {
    const res = await fetch(requestPath, {
      credentials: 'include',
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...(options.headers || {}),
      },
    });
    const text = await res.text();
    let body = null;
    if (text) {
      try {
        body = JSON.parse(text);
      } catch {
        body = { raw: text };
      }
    }
    if (!res.ok) {
      throw new Error(`${requestPath} failed: ${res.status} ${text}`);
    }
    return body;
  }, { requestPath, options });
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]').waitFor({
    state: 'visible',
    timeout: 120_000,
  });
}

async function waitForPromptDecision(page, submissionId, samples, timeout = 150_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
    samples.push({
      at: new Date().toISOString(),
      state: status.state,
      has_decision: !!status.decision,
      error: status.error || '',
    });
    if (status.decision) return status;
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) return status;
    await page.waitForTimeout(1500);
  }
  return fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
}

async function loadVTextState(page, docId) {
  const [doc, revisions] = await Promise.all([
    fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(docId)}`),
    fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(docId)}/revisions`),
  ]);
  return { doc, revisions: revisions.revisions || [] };
}

async function pollVText(page, docId, samples, timeout = 210_000) {
  const deadline = Date.now() + timeout;
  let latest = null;
  while (Date.now() < deadline) {
    latest = await loadVTextState(page, docId);
    const revisions = latest.revisions || [];
    const appagentRevisions = revisions.filter((revision) => revision.author_kind === 'appagent');
    samples.push({
      at: new Date().toISOString(),
      current_revision_id: latest.doc.current_revision_id,
      revision_count: revisions.length,
      appagent_revision_count: appagentRevisions.length,
      appagent_revision_ids: appagentRevisions.map((revision) => revision.revision_id),
      appagent_authors: appagentRevisions.map((revision) => revision.author_label),
      metadata_sources: revisions.map((revision) => revision.metadata?.source || ''),
    });
    if (appagentRevisions.length >= 2) return latest;
    await page.waitForTimeout(3000);
  }
  return latest || loadVTextState(page, docId);
}

async function loadTrace(page, trajectoryId) {
  return fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(trajectoryId)}`);
}

function classify({ finalState, trace }) {
  const revisions = finalState?.revisions || [];
  const appagentRevisions = revisions.filter((revision) => revision.author_kind === 'appagent');
  const moments = trace?.moments || [];
  const agents = trace?.agents || [];
  const summaries = moments.map((moment) => `${moment.kind || ''}:${moment.summary || ''}`);
  const sawSearch = summaries.some((summary) => summary.includes('web_search'));
  const sawSubmitUpdate = summaries.some((summary) => summary.includes('submit_coagent_update'));
  const sawSpawn = summaries.some((summary) => summary.includes('spawn_agent'));
  const sawResearcher = agents.some((agent) => agent.role === 'researcher' || agent.profile === 'researcher');

  if (appagentRevisions.length >= 2) return 'version_advancement_observed';
  if (appagentRevisions.length === 0) return 'vtext_did_not_write_first_appagent_revision';
  if (sawSearch && !sawSubmitUpdate) return 'worker_evidence_not_checkpointed';
  if (sawResearcher && !sawSearch) return 'researcher_started_without_search_result';
  if (sawSpawn && !sawResearcher) return 'worker_spawn_without_researcher_trace';
  return 'single_appagent_revision_without_classifiable_worker_evidence';
}

test('VText advances versions through the product prompt path', async ({ page, authenticator }) => {
  fs.mkdirSync(EVIDENCE_DIR, { recursive: true });
  const email = uniqueEmail();
  const prompt = process.env.VTEXT_REPRO_PROMPT ||
    'What happened in baseball last night? Give me a concise evidence-grounded brief with sources.';
  const evidence = {
    base_url: BASE_URL,
    email,
    prompt,
    started_at: new Date().toISOString(),
    prompt_samples: [],
    revision_samples: [],
  };

  try {
    await registerAndLoadDesktop(page, email);
    evidence.health = await page.evaluate(async () => {
      const res = await fetch('/health', { credentials: 'include' });
      return res.ok ? res.json() : { status: res.status, body: await res.text() };
    });

    const submission = await fetchJSON(page, '/api/prompt-bar', {
      method: 'POST',
      body: JSON.stringify({ text: prompt }),
    });
    evidence.submission = submission;

    const status = await waitForPromptDecision(page, submission.submission_id, evidence.prompt_samples);
    evidence.prompt_status = status;
    evidence.decision = status.decision || null;
    expect(status.decision?.doc_id, `prompt status: ${JSON.stringify(status)}`).toBeTruthy();

    const docId = status.decision.doc_id;
    evidence.doc_id = docId;
    evidence.final_state = await pollVText(page, docId, evidence.revision_samples);
    evidence.trace = await loadTrace(page, submission.submission_id);
    evidence.classification = classify({
      finalState: evidence.final_state,
      trace: evidence.trace,
    });
    evidence.finished_at = new Date().toISOString();

    const appagentCount = (evidence.final_state.revisions || [])
      .filter((revision) => revision.author_kind === 'appagent').length;
    expect(appagentCount, JSON.stringify({
      classification: evidence.classification,
      doc_id: docId,
      submission_id: submission.submission_id,
      revision_samples: evidence.revision_samples.slice(-5),
    }, null, 2)).toBeGreaterThanOrEqual(2);
  } catch (err) {
    evidence.error = err?.stack || String(err);
    evidence.finished_at = new Date().toISOString();
    throw err;
  } finally {
    const safeTime = new Date().toISOString().replace(/[:.]/g, '-');
    const outPath = path.join(EVIDENCE_DIR, `repro-${safeTime}.json`);
    fs.writeFileSync(outPath, JSON.stringify(evidence, null, 2));
    console.log(`VText repro evidence: ${outPath}`);
  }
});
