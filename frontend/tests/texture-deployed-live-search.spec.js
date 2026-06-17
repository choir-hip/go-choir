import { test, expect } from '@playwright/test';
import { setupVirtualAuthenticator, removeVirtualAuthenticator } from './helpers/webauthn.js';
import { registerPasskey } from './helpers/auth.js';
import { waitForDesktopReady } from './helpers/auth-state.js';

const BASE_URL = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(600_000);

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
test.skip(
  process.env.GO_CHOIR_RUN_DEPLOYED_LIVE_SEARCH !== '1',
  'set GO_CHOIR_RUN_DEPLOYED_LIVE_SEARCH=1 to verify deployed prompt-bar -> Texture live search'
);

function uniqueEmail() {
  return `texture-live-search-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function fetchJSON(page, path) {
  return page.evaluate(async (requestPath) => {
    let res = await fetch(requestPath, { credentials: 'include' });
    if (res.status === 401) {
      await fetch('/auth/session', { credentials: 'include' }).catch(() => null);
      res = await fetch(requestPath, { credentials: 'include' });
    }
    const body = await res.text();
    if (!res.ok) {
      throw new Error(`${requestPath} failed: ${res.status} ${body}`);
    }
    return body ? JSON.parse(body) : null;
  }, path);
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

function eventPayload(event) {
  if (!event?.payload) return {};
  if (typeof event.payload === 'string') {
    try {
      return JSON.parse(event.payload);
    } catch {
      return {};
    }
  }
  return event.payload;
}

function parseToolOutput(payload) {
  const raw = payload?.output;
  if (typeof raw !== 'string') return raw || {};
  try {
    return JSON.parse(raw);
  } catch {
    return { raw };
  }
}

async function traceMomentDetails(page, trajectoryId, snapshot) {
  const details = [];
  for (const moment of snapshot.moments || []) {
    if (moment.kind !== 'tool.result') continue;
    const detail = await fetchJSON(
      page,
      `/api/trace/trajectories/${encodeURIComponent(trajectoryId)}/moments/${encodeURIComponent(moment.moment_id)}`
    );
    details.push(detail);
  }
  return details;
}

async function waitForSuccessfulWebSearch(page, trajectoryId, timeout = 240_000) {
  const deadline = Date.now() + timeout;
  let lastSearchErrors = [];
  while (Date.now() < deadline) {
    const snapshot = await fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(trajectoryId)}`);
    const details = await traceMomentDetails(page, trajectoryId, snapshot);
    lastSearchErrors = [];

    for (const detail of details) {
      for (const event of detail.events || []) {
        const payload = eventPayload(event);
        if (payload.tool !== 'web_search') continue;
        const output = parseToolOutput(payload);
        if (payload.is_error) {
          lastSearchErrors.push(output.raw || payload.output || 'web_search failed');
          continue;
        }
        const results = Array.isArray(output.results) ? output.results : [];
        const serializedResults = JSON.stringify(results);
        if (output.provider && results.length > 0 && /2026/.test(serializedResults)) {
          return { snapshot, provider: output.provider, providers: output.providers || [], results };
        }
      }
    }

    await page.waitForTimeout(2000);
  }
  throw new Error(`trajectory ${trajectoryId} never produced a successful 2026 web_search result; errors=${lastSearchErrors.join(' | ')}`);
}

async function waitForGroundedTextureRevision(page, docId, prompt, samples = [], timeout = 300_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const state = await loadTextureState(page, docId);
    const revisions = state.revisions || [];
    const v0 = revisions.find((revision) => revision.version_number === 0);
    const appagentRevisions = revisions.filter((revision) => revision.author_kind === 'appagent');
    const consumed = revisions.flatMap((revision) =>
      revision?.metadata?.worker_updates_consumed || []
    );
    const consumedResearch = consumed.some((item) => item.role === 'researcher');
    const content = state.head?.content || '';
    samples.push({
      at: new Date().toISOString(),
      revision_count: revisions.length,
      appagent_revision_count: appagentRevisions.length,
      current_revision_id: state.doc.current_revision_id,
      v0_content_len: v0?.content?.length || 0,
      consumed_researcher: consumedResearch,
    });
    if (
      v0 &&
      v0.content === prompt &&
      consumedResearch &&
      appagentRevisions.length >= 2 &&
      /2026/.test(content) &&
      /https?:\/\/|source|evidence|fetched|search/i.test(content) &&
      !/search (?:was )?unavailable|search unavailable|model knowledge through|mid-2024|broad trend extrapolation/i.test(content)
    ) {
      return { ...state, consumed, v0, appagentRevisions };
    }
    await page.waitForTimeout(2000);
  }
  const state = await loadTextureState(page, docId);
  throw new Error(`document ${docId} never produced grounded live-search revisions; samples=${JSON.stringify(samples.slice(-8))} head=${JSON.stringify(state.head)}`);
}

test('deployed prompt-bar Texture flow uses live search for current 2026 evidence', async ({ browser }) => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const { client, authenticatorId } = await setupVirtualAuthenticator(page);

  const forbiddenRuntimeRequests = [];
  page.on('request', (request) => {
    const url = new URL(request.url());
    if (request.method() === 'POST' && url.pathname === '/api/agent/spawn') {
      forbiddenRuntimeRequests.push(url.pathname);
    }
    if (['/api/agent/topology', '/api/prompts', '/api/events'].includes(url.pathname)) {
      forbiddenRuntimeRequests.push(url.pathname);
    }
  });

  try {
    await registerAndLoadDesktop(page, uniqueEmail());
    const health = await waitForStagingReady(page);
    console.log('staging health:', JSON.stringify(health.build || health));

    const prompt = [
      'Create a texture briefing about what is new in AI infrastructure news today, June 16, 2026.',
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
    expect(response.request().postDataJSON()).toEqual({ text: prompt });

    const decision = await waitForPromptDecision(page, body.submission_id);
    expect(decision.action).toBe('open_app');
    expect(decision.app).toBe('texture');
    expect(decision.doc_id).toBeTruthy();

    const textureWindow = page.locator('[data-texture-app]').last();
    await expect(textureWindow).toBeVisible({ timeout: 30_000 });
    await expect(textureWindow.locator('[data-texture-intake]')).toHaveCount(0);
    await expect(textureWindow.locator('[data-texture-editor-area]')).toContainText(prompt, { timeout: 30_000 });

    const search = await waitForSuccessfulWebSearch(page, body.submission_id);
    const searchProviders = Array.isArray(search.providers) && search.providers.length > 0
      ? search.providers
      : [search.provider];
    expect(searchProviders.some((provider) => ['tavily', 'brave', 'exa', 'serper', 'parallel', 'serpapi'].includes(provider))).toBeTruthy();
    expect(search.results.length).toBeGreaterThan(0);

    const finalState = await waitForGroundedTextureRevision(page, decision.doc_id, prompt, revisionSamples);
    expect(finalState.v0.content).toBe(prompt);
    expect(finalState.appagentRevisions.length).toBeGreaterThanOrEqual(2);
    expect(finalState.head.content).toMatch(/2026/);
    expect(finalState.head.content).not.toMatch(/search (?:was )?unavailable|model knowledge through|mid-2024|stub provider/i);
    console.log('Texture revision progression:', JSON.stringify(revisionSamples, null, 2));
    expect(forbiddenRuntimeRequests).toHaveLength(0);

    await expect(page.locator('[data-desktop-icon-id="trace"]')).toHaveCount(0);
    const traceSnapshot = await fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(body.submission_id)}`);
    expect(traceSnapshot.trajectory?.trajectory_id || traceSnapshot.trajectory_id).toBe(body.submission_id);
  } finally {
    await removeVirtualAuthenticator(client, authenticatorId);
    await context.close();
  }
});
