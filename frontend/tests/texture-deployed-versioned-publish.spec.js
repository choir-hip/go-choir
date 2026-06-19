import { test, expect } from '@playwright/test';
import { setupVirtualAuthenticator, removeVirtualAuthenticator } from './helpers/webauthn.js';
import { registerPasskey } from './helpers/auth.js';
import { waitForDesktopReady } from './helpers/auth-state.js';

// D7 deployed acceptance proof: a published Texture IS its full versioned
// history. Drives the real product path on choir.news — register -> prompt ->
// multi-revision grounded Texture -> publish -> resolve the /pub bundle — and
// asserts the published artifact carries the whole version_history chain with
// per-revision provenance, the tamper-evident hash chain, and a manifest hash
// that matches the publish response. Backs the D5/D7 settlement evidence.
//
// Gate: set GO_CHOIR_RUN_DEPLOYED_VERSIONED_PUBLISH=1 (real provider/search
// calls; ~5-10 min). Defaults to choir.news; override with
// CHOIR_DEPLOYED_BASE_URL.

const BASE_URL = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const VERSION_HISTORY_SCHEMA = 'choir.platform.version_history.v0';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(600_000);
test.skip(
  process.env.GO_CHOIR_RUN_DEPLOYED_VERSIONED_PUBLISH !== '1',
  'set GO_CHOIR_RUN_DEPLOYED_VERSIONED_PUBLISH=1 to verify deployed full-history publish serves the version_history chain'
);

function uniqueEmail() {
  return `texture-versioned-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function waitForStagingReady(page, timeout = 180_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const health = await page.evaluate(async () => {
      const res = await fetch('/health', { credentials: 'include' });
      return res.ok ? res.json() : { status: 'error', upstream: 'error' };
    });
    if (health.status === 'ok' && health.upstream === 'ok') {
      return health;
    }
    await page.waitForTimeout(5000);
  }
  throw new Error('staging health never reached status=ok upstream=ok');
}

async function sendJSON(page, path, { method = 'GET', body = null } = {}) {
  return page.evaluate(async ({ requestPath, requestOptions }) => {
    const init = { method: requestOptions.method, credentials: 'include' };
    if (requestOptions.body !== null) {
      init.headers = { 'Content-Type': 'application/json' };
      init.body = requestOptions.body;
    }
    let res = await fetch(requestPath, init);
    if (res.status === 401) {
      await fetch('/auth/session', { credentials: 'include' }).catch(() => null);
      res = await fetch(requestPath, init);
    }
    const text = await res.text();
    if (!res.ok) {
      throw new Error(`${requestOptions.method} ${requestPath} failed: ${res.status} ${text}`);
    }
    return text ? JSON.parse(text) : null;
  }, { requestPath: path, requestOptions: { method, body: body === null ? null : JSON.stringify(body) } });
}

async function fetchJSON(page, path) {
  return sendJSON(page, path, { method: 'GET' });
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await waitForDesktopReady(page, 120_000);
}

async function waitForPromptDecision(page, submissionId, timeout = 150_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
    if (status.decision) return status.decision;
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) {
      throw new Error(`prompt submission ${submissionId} ended as ${status.state}: ${status.error || ''}`);
    }
    await page.waitForTimeout(1000);
  }
  throw new Error(`prompt submission ${submissionId} did not produce a decision`);
}

async function loadTextureState(page, docId) {
  const [doc, revisionsResponse] = await Promise.all([
    fetchJSON(page, `/api/texture/documents/${encodeURIComponent(docId)}`),
    fetchJSON(page, `/api/texture/documents/${encodeURIComponent(docId)}/revisions`),
  ]);
  const revisions = revisionsResponse.revisions || [];
  const head = revisions.find((revision) => revision.revision_id === doc.current_revision_id);
  return { doc, revisions, head };
}

// Wait until the document has V0 (the verbatim prompt) plus >=2 appagent
// revisions that consumed researcher evidence and grounded the head in 2026
// sources. Mirrors the live-search acceptance bar so the published chain
// actually carries per-revision provenance.
async function waitForGroundedTextureRevision(page, docId, prompt, samples = [], timeout = 360_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const state = await loadTextureState(page, docId);
    const revisions = state.revisions || [];
    const v0 = revisions.find((revision) => revision.version_number === 0);
    const appagentRevisions = revisions.filter((revision) => revision.author_kind === 'appagent');
    const consumed = revisions.flatMap((revision) => revision?.metadata?.worker_updates_consumed || []);
    const consumedResearch = consumed.some((item) => item.role === 'researcher');
    const content = state.head?.content || '';
    samples.push({
      at: new Date().toISOString(),
      revision_count: revisions.length,
      appagent_revision_count: appagentRevisions.length,
      head_has_revision_hash: Boolean(state.head?.revision_hash),
      head_has_provenance: Boolean(state.head?.provenance),
      consumed_researcher: consumedResearch,
    });
    if (
      v0 &&
      v0.content === prompt &&
      consumedResearch &&
      appagentRevisions.length >= 2 &&
      /2026/.test(content) &&
      /https?:\/\/|source|evidence|fetched|search/i.test(content)
    ) {
      return { ...state, consumed, v0, appagentRevisions };
    }
    await page.waitForTimeout(3000);
  }
  const state = await loadTextureState(page, docId);
  throw new Error(`document ${docId} never produced grounded multi-revision history; samples=${JSON.stringify(samples.slice(-8))} head=${JSON.stringify(state.head)?.slice(0, 400)}`);
}

test('deployed full-history publish serves the version_history chain', async ({ browser }) => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const { client, authenticatorId } = await setupVirtualAuthenticator(page);

  try {
    await registerAndLoadDesktop(page, uniqueEmail());
    const health = await waitForStagingReady(page);
    console.log('staging health:', JSON.stringify(health.build || health));

    const prompt = [
      'Create a texture briefing about what is new in AI infrastructure news today.',
      'Use live researcher web_search evidence before making the substantive revision.',
      'The final document should include source-grounded 2026 evidence and should not rely on model-prior knowledge.',
    ].join(' ');

    const revisionSamples = [];

    const promptBarResponse = page.waitForResponse((response) =>
      new URL(response.url()).pathname === '/api/prompt-bar' && response.request().method() === 'POST'
    );
    await page.locator('[data-prompt-input]').fill(prompt);
    await page.locator('[data-prompt-input]').press('Enter');

    const response = await promptBarResponse;
    const body = await response.json();
    expect(response.status()).toBe(202);
    expect(body.submission_id).toBeTruthy();

    const decision = await waitForPromptDecision(page, body.submission_id);
    expect(decision.action).toBe('open_app');
    expect(decision.app).toBe('texture');
    expect(decision.doc_id).toBeTruthy();

    const textureWindow = page.locator('[data-texture-app]').last();
    await expect(textureWindow).toBeVisible({ timeout: 30_000 });

    // Wait for a genuine multi-revision, grounded document.
    const grounded = await waitForGroundedTextureRevision(page, decision.doc_id, prompt, revisionSamples);
    console.log('Texture revision progression:', JSON.stringify(revisionSamples.slice(-6), null, 2));

    const headRevisionId = grounded.doc.current_revision_id;
    expect(headRevisionId).toBeTruthy();
    const groundedHead = grounded.revisions.find((r) => r.revision_id === headRevisionId);
    // The private head revision already carries the D2 hash + D1 provenance.
    expect(groundedHead.revision_hash).toBeTruthy();
    expect(groundedHead.provenance).toBeTruthy();

    // Publish the head revision through the product path.
    const publishResp = await sendJSON(page, '/api/platform/texture/publications', {
      method: 'POST',
      body: { doc_id: decision.doc_id, revision_id: headRevisionId },
    });
    expect(publishResp.route_path).toMatch(/^\/pub\/texture\//);
    // D5: publish now persists the whole chain, so the response reports it.
    expect(publishResp.version_count).toBeGreaterThanOrEqual(2);
    expect(publishResp.version_history_hash).toBeTruthy();
    console.log('published:', publishResp.route_path, 'version_count=', publishResp.version_count, 'hash=', publishResp.version_history_hash);

    // Resolve the public bundle and assert it serves the full version history.
    const bundle = await fetchJSON(page, `/api/platform/publications/resolve?route=${encodeURIComponent(publishResp.route_path)}`);
    expect(bundle.version_history, 'bundle must carry version_history').toBeTruthy();
    const history = bundle.version_history;
    expect(history.schema).toBe(VERSION_HISTORY_SCHEMA);
    expect(history.revision_count).toBe(publishResp.version_count);
    expect(history.revision_count).toBeGreaterThanOrEqual(2);
    // The manifest hash in the bundle must match what publish reported.
    expect(history.manifest_hash).toBe(publishResp.version_history_hash);
    // Tamper-evident spine: the chain head hash is the head revision's hash.
    expect(history.chain_head_hash).toBeTruthy();
    expect(history.chain_head_hash).toBe(groundedHead.revision_hash);

    expect(Array.isArray(history.revisions)).toBe(true);
    expect(history.revisions.length).toBe(history.revision_count);
    // Oldest-first causal order. version_number uses omitempty so the V0 genesis
    // (version 0) drops the field; normalize undefined -> 0.
    for (let i = 1; i < history.revisions.length; i += 1) {
      expect(history.revisions[i].version_number ?? 0).toBeGreaterThanOrEqual(history.revisions[i - 1].version_number ?? 0);
    }
    // The authoritative causal-order invariant: each non-genesis revision's
    // parent links to the previous entry in the chain.
    for (let i = 1; i < history.revisions.length; i += 1) {
      if (history.revisions[i].parent_revision_id) {
        expect(history.revisions[i].parent_revision_id).toBe(history.revisions[i - 1].revision_id);
      }
    }
    const firstEntry = history.revisions[0];
    const headEntry = history.revisions[history.revisions.length - 1];
    expect(firstEntry.revision_id).toBeTruthy();
    expect(headEntry.revision_id).toBe(headRevisionId);
    expect(headEntry.revision_hash).toBe(history.chain_head_hash);
    // Per-revision provenance is carried for appagent revisions (the head is one).
    expect(headEntry.provenance).toBeTruthy();
    expect(JSON.stringify(headEntry.provenance).length).toBeGreaterThan(2);
    // V0 (the verbatim prompt) is preserved as the genesis of the chain.
    expect(firstEntry.content).toBe(prompt);

    console.log('version_history OK:', JSON.stringify({
      schema: history.schema,
      revision_count: history.revision_count,
      chain_head_hash: history.chain_head_hash,
      manifest_hash: history.manifest_hash,
      first_revision: firstEntry.revision_id,
      head_revision: headEntry.revision_id,
    }));

    // Synthesize a durable RunAcceptanceRecord from the real trajectory evidence
    // (the prompt-bar submission_id is the trajectory id). This is the honest
    // settlement artifact: the runtime derives checkpoints/level from what the
    // trajectory actually shows, not from this spec's assertions.
    const acceptance = await sendJSON(page, '/api/run-acceptances/synthesize', {
      method: 'POST',
      body: {
        target_mission_id: 'mission-texture-versioned-artifact-v0',
        trajectory_id: body.submission_id,
        source_prompt_or_objective: prompt,
      },
    });
    expect(acceptance.acceptance_id).toBeTruthy();
    console.log('RunAcceptanceRecord:', JSON.stringify({
      acceptance_id: acceptance.acceptance_id,
      acceptance_level: acceptance.acceptance_level,
      state: acceptance.state,
      trajectory_id: body.submission_id,
      published_route: publishResp.route_path,
    }));

    // End-to-end reader proof: navigate to the public /pub route and assert the
    // version-history disclosure renders the chain (Option A reader UX).
    const publicURL = publishResp.public_url || `${BASE_URL}${publishResp.route_path}`;
    await page.goto(publicURL);
    const publishedReader = page.locator('[data-texture-published-reader]').last();
    await expect(publishedReader).toBeVisible({ timeout: 30_000 });
    const versionHistoryPanel = page.locator('[data-texture-version-history]');
    await expect(versionHistoryPanel).toBeAttached();
    // Open the disclosure so the lineage is visible.
    await versionHistoryPanel.locator('summary').click();
    const lineage = versionHistoryPanel.locator('[data-version-lineage] .vh-rev');
    await expect(lineage).toHaveCount(history.revision_count);
    await expect(versionHistoryPanel.locator('[data-chain-verified]')).toBeVisible();
    console.log('reader version-history panel rendered:', history.revision_count, 'lineage rows at', publishResp.route_path);
  } finally {
    await removeVirtualAuthenticator(client, authenticatorId);
    await context.close();
  }
});
