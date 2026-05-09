import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = 'http://localhost:4173';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(240_000);
test.skip(
  process.env.GO_CHOIR_RUN_REAL_VTEXT_DEMO !== '1',
  'set GO_CHOIR_RUN_REAL_VTEXT_DEMO=1 with a real provider and real search keys to run this product proof'
);

function uniqueEmail() {
  return `vtext-real-demo-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function assertRealSearchConfigured() {
  const gatewayURL = process.env.GO_CHOIR_REAL_DEMO_GATEWAY_URL ||
    process.env.RUNTIME_GATEWAY_URL ||
    'http://127.0.0.1:8084';
  let health;
  try {
    const res = await fetch(`${gatewayURL}/health`);
    if (!res.ok) {
      throw new Error(`status ${res.status}`);
    }
    health = await res.json();
  } catch (err) {
    throw new Error(`real vtext demo requires a reachable gateway with real search configured at ${gatewayURL}: ${err.message}`);
  }
  const providers = health.search_providers || [];
  if (!Array.isArray(providers) || providers.length === 0) {
    throw new Error(`real vtext demo requires at least one gateway search provider; ${gatewayURL}/health reported ${JSON.stringify(providers)}`);
  }
}

function fileAPIPath(path) {
  return `/api/files/${path.split('/').map(encodeURIComponent).join('/')}`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 10000 });
}

async function fetchJSON(page, path) {
  return page.evaluate(async (requestPath) => {
    const res = await fetch(requestPath, { credentials: 'include' });
    if (!res.ok) {
      const body = await res.text();
      throw new Error(`${requestPath} failed: ${res.status} ${body}`);
    }
    return res.json();
  }, path);
}

async function waitForTraceRoles(page, trajectoryId, requiredRoles, timeout = 150_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const snapshot = await fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(trajectoryId)}`);
    const roles = (snapshot.agents || []).map((agent) => agent.role || agent.profile || agent.label);
    if (requiredRoles.every((role) => roles.includes(role))) {
      return { snapshot, roles };
    }
    await page.waitForTimeout(1500);
  }
  const snapshot = await fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(trajectoryId)}`);
  const roles = (snapshot.agents || []).map((agent) => agent.role || agent.profile || agent.label);
  throw new Error(`trajectory ${trajectoryId} did not include product-path roles ${requiredRoles.join(', ')}; saw ${roles.join(', ')}`);
}

async function waitForPromptDecision(page, submissionId, timeout = 120_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
    if (status.decision) {
      return status.decision;
    }
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) {
      throw new Error(`prompt submission ${submissionId} ended as ${status.state}: ${status.error || ''}`);
    }
    await page.waitForTimeout(1000);
  }
  throw new Error(`prompt submission ${submissionId} did not produce a decision`);
}

async function listRevisions(page, docId) {
  return fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(docId)}/revisions`);
}

async function loadVTextState(page, docId) {
  const [doc, revisions] = await Promise.all([
    fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(docId)}`),
    listRevisions(page, docId),
  ]);
  const head = (revisions.revisions || []).find((revision) => revision.revision_id === doc.current_revision_id);
  return { doc, revisions, head };
}

async function waitForFinalDocument(page, docId, roles, checks, timeout = 150_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const state = await loadVTextState(page, docId);
    const consumed = (state.revisions.revisions || []).flatMap((revision) =>
      revision?.metadata?.worker_updates_consumed || []
    );
    const content = state.head?.content || '';
    const rolesConsumed = roles.every((role) => consumed.some((item) => item.role === role));
    const contentReady = checks.every((check) =>
      typeof check === 'string' ? content.includes(check) : check.test(content)
    );
    if (rolesConsumed && contentReady) return { ...state, consumed };
    await page.waitForTimeout(1500);
  }
  const state = await loadVTextState(page, docId);
  throw new Error(`final vtext document did not include required live workflow material; head=${JSON.stringify(state.head)}`);
}

async function waitForFileText(page, path, timeout = 90_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const text = await page.evaluate(async (requestPath) => {
      const res = await fetch(requestPath, { credentials: 'include' });
      if (!res.ok) return null;
      return res.text();
    }, fileAPIPath(path));
    if (text) return text;
    await page.waitForTimeout(1000);
  }
  throw new Error(`file ${path} was not generated`);
}

test('real vtext workflow demo uses live LLM, search, generated artifact, and verification', async ({ page, authenticator }) => {
  await assertRealSearchConfigured();
  await registerAndLoadDesktop(page, uniqueEmail());

  const browserSpawnRequests = [];
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (request.method() === 'POST' && url.pathname === '/api/agent/spawn') {
      browserSpawnRequests.push(url.pathname);
    }
  });

  const marker = `REAL_VTEXT_DEMO_${Date.now()}`;
  const artifactPath = `artifacts/${marker.toLowerCase()}-evolution-ca.html`;
  const verifyPath = `artifacts/${marker.toLowerCase()}-evolution-ca.verify.js`;
  const prompt = [
    `Create a vtext document for ${marker}.`,
    'Research cellular automata as toy models of biological evolution, then build and verify a small interactive visualization artifact.',
    `The evolving document must preserve this user marker, cite live search evidence, mention ${artifactPath}, and include the verification result for ${verifyPath}.`,
    `The generated HTML artifact and Node verification script must both contain the literal marker ${marker}.`,
  ].join(' ');

  const conductorResponse = page.waitForResponse((response) =>
    response.url().includes('/api/prompt-bar') && response.request().method() === 'POST'
  );
  await page.locator('[data-prompt-input]').fill(prompt);
  await page.locator('[data-prompt-input]').press('Enter');

  const conductorSubmitted = await (await conductorResponse).json();
  const conductorDecision = await waitForPromptDecision(page, conductorSubmitted.submission_id, 120_000);
  expect(conductorDecision.action).toBe('open_app');
  expect(conductorDecision.app).toBe('vtext');
  expect(conductorDecision.initial_loop_id || '').toBeTruthy();

  const initialState = await loadVTextState(page, conductorDecision.doc_id);
  const v1 = (initialState.revisions.revisions || []).find((revision) => revision.revision_id === conductorDecision.framing_revision_id);
  expect(v1?.content || '').toContain(marker);
  expect(v1?.content || '').not.toMatch(/Conductor framing|Use this vtext|User request:|Current requirements:|Grounding status:/);

  const vtextWindow = page.locator('[data-vtext-app]').last();
  await expect(vtextWindow).toBeVisible({ timeout: 15000 });
  await expect(vtextWindow.locator('[data-vtext-editor-area]')).toContainText(new RegExp(marker));
  await expect(vtextWindow.locator('[data-vtext-editor-area]')).not.toContainText(/Conductor framing|Use this vtext|User request:/);

  const traceWithWorkers = await waitForTraceRoles(page, conductorSubmitted.submission_id, ['conductor', 'vtext', 'researcher', 'super'], 180_000);
  expect(browserSpawnRequests).toHaveLength(0);

  const html = await waitForFileText(page, artifactPath);
  const verify = await waitForFileText(page, verifyPath);
  expect(html).toContain(marker);
  expect(html).toMatch(/canvas|grid/i);
  expect(verify).toContain(marker);
  expect(verify).toMatch(/\bpass(?:ed)?\b|assert/i);

  const finalState = await waitForFinalDocument(page, conductorDecision.doc_id, ['researcher', 'super'], [
    marker,
    artifactPath,
    verifyPath,
    /https?:\/\/|source|evidence/i,
    /verification|passed|node/i,
  ], 180_000);
  expect(finalState.head.content).not.toMatch(/Task completed successfully|stub provider|Worker update ready\.|Research findings ready\.|Conductor framing|Use this vtext|User request:/i);
  expect(finalState.head.metadata.source).toBe('edit_vtext');
  expect(finalState.head.metadata.vtext_edit_kind).toBe('vtext_edit');

  await page.locator('[data-desktop-icon-id="files"]').dblclick();
  const filesApp = page.locator('[data-files-app]').last();
  await expect(filesApp).toBeVisible({ timeout: 10000 });
  await filesApp.locator('[data-file-item]').filter({ hasText: 'artifacts' }).first().click();
  await expect(filesApp.locator('[data-file-item]').filter({ hasText: artifactPath.split('/').pop() })).toBeVisible({ timeout: 10000 });
  await expect(filesApp.locator('[data-file-item]').filter({ hasText: verifyPath.split('/').pop() })).toBeVisible();

  const traceSnapshot = traceWithWorkers.snapshot;
  const roles = traceWithWorkers.roles;
  expect(roles).toEqual(expect.arrayContaining(['conductor', 'vtext', 'researcher', 'super']));
  expect(traceSnapshot.moments.some((moment) => /web_search|Research findings|Worker update|vtext revision/i.test(moment.summary))).toBe(true);

  await page.locator('[data-desktop-icon-id="trace"]').dblclick();
  const traceApp = page.locator('[data-trace-app]').last();
  await expect(traceApp).toBeVisible({ timeout: 10000 });
  const trajectory = traceApp.locator(`[data-trace-trajectory-id="${conductorSubmitted.submission_id}"]`);
  await expect(trajectory).toBeVisible({ timeout: 10000 });
  await trajectory.click();
  await expect(traceApp.locator('[data-trace-agent-node]').filter({ hasText: /vtext/i })).toBeVisible();
  await expect(traceApp.locator('[data-trace-agent-node]').filter({ hasText: /researcher/i }).first()).toBeVisible();
  await expect(traceApp.locator('[data-trace-agent-node]').filter({ hasText: /^super\b/i }).first()).toBeVisible();
});
