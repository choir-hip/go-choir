import { test, expect } from '@playwright/test';
import { setupVirtualAuthenticator, removeVirtualAuthenticator } from './helpers/webauthn.js';

const BASE_URL = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://draft.choir-ip.com';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(420_000);
test.skip(
  process.env.GO_CHOIR_RUN_DEPLOYED_LIVE_SEARCH !== '1',
  'set GO_CHOIR_RUN_DEPLOYED_LIVE_SEARCH=1 to verify deployed prompt-bar -> VText live search'
);

function uniqueEmail() {
  return `vtext-live-search-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function fetchJSON(page, path) {
  return page.evaluate(async (requestPath) => {
    const res = await fetch(requestPath, { credentials: 'include' });
    const body = await res.text();
    if (!res.ok) {
      throw new Error(`${requestPath} failed: ${res.status} ${body}`);
    }
    return body ? JSON.parse(body) : null;
  }, path);
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await page.getByRole('textbox', { name: 'Email' }).fill(email);
  await page.getByRole('button', { name: /register with passkey/i }).click();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 30_000 });
  await page.reload();
  await page.locator('[data-desktop]').waitFor({ state: 'visible', timeout: 20_000 });
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

async function loadVTextState(page, docId) {
  const [doc, revisionsResponse] = await Promise.all([
    fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(docId)}`),
    fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(docId)}/revisions`),
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

async function waitForGroundedVTextRevision(page, docId, timeout = 240_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const state = await loadVTextState(page, docId);
    const content = state.head?.content || '';
    const consumed = state.revisions.flatMap((revision) =>
      revision?.metadata?.worker_updates_consumed || []
    );
    const consumedResearch = consumed.some((item) => item.role === 'researcher');
    if (
      consumedResearch &&
      /2026/.test(content) &&
      /https?:\/\/|source|evidence|fetched|search/i.test(content) &&
      !/search (?:was )?unavailable|search unavailable|model knowledge through|mid-2024|broad trend extrapolation/i.test(content)
    ) {
      return { ...state, consumed };
    }
    await page.waitForTimeout(2000);
  }
  const state = await loadVTextState(page, docId);
  throw new Error(`document ${docId} never produced a grounded live-search revision; head=${JSON.stringify(state.head)}`);
}

test('deployed prompt-bar VText flow uses live search for current 2026 evidence', async ({ browser }) => {
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

    const prompt = [
      'Create a vtext briefing about what is new in AI this week ending May 4, 2026.',
      'Use live researcher web_search evidence before making the substantive revision.',
      'The final document should include source-grounded 2026 evidence and should not rely on model-prior knowledge.',
    ].join(' ');

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
    expect(decision.app).toBe('vtext');
    expect(decision.doc_id).toBeTruthy();

    const vtextWindow = page.locator('[data-vtext-app]').last();
    await expect(vtextWindow).toBeVisible({ timeout: 30_000 });
    await expect(vtextWindow.locator('[data-vtext-editor-area]')).toHaveValue(/live|search|evidence|source|2026/i, { timeout: 30_000 });

    const search = await waitForSuccessfulWebSearch(page, body.submission_id);
    const searchProviders = Array.isArray(search.providers) && search.providers.length > 0
      ? search.providers
      : [search.provider];
    expect(searchProviders.some((provider) => ['tavily', 'brave', 'exa', 'serper'].includes(provider))).toBeTruthy();
    expect(search.results.length).toBeGreaterThan(0);

    const finalState = await waitForGroundedVTextRevision(page, decision.doc_id);
    expect(finalState.head.metadata.source).toBe('edit_vtext');
    expect(finalState.head.content).toMatch(/2026/);
    expect(finalState.head.content).not.toMatch(/search (?:was )?unavailable|model knowledge through|mid-2024|stub provider/i);
    expect(forbiddenRuntimeRequests).toHaveLength(0);

    await page.locator('[data-desktop-icon-id="trace"]').dblclick();
    const traceApp = page.locator('[data-trace-app]').last();
    await expect(traceApp).toBeVisible({ timeout: 15_000 });
    const trajectory = traceApp.locator(`[data-trace-trajectory-id="${body.submission_id}"]`);
    await expect(trajectory).toBeVisible({ timeout: 15_000 });
  } finally {
    await removeVirtualAuthenticator(client, authenticatorId);
    await context.close();
  }
});
