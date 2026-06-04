import { test, expect } from './helpers/fixtures.js';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(360_000);
test.skip(
  process.env.GO_CHOIR_RUN_DEPLOYED_SOURCE_SERVICE_RESEARCH !== '1',
  'set GO_CHOIR_RUN_DEPLOYED_SOURCE_SERVICE_RESEARCH=1 to verify deployed researcher source_search -> VText source entities'
);

async function fetchJSON(page, path, options = {}) {
  return page.evaluate(async ({ requestPath, requestOptions }) => {
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
    const text = await res.text();
    let body = null;
    try {
      body = text ? JSON.parse(text) : null;
    } catch (_err) {
      body = text;
    }
    if (!res.ok) {
      throw new Error(`${requestOptions.method || 'GET'} ${requestPath} failed ${res.status}: ${text}`);
    }
    return body;
  }, { requestPath: path, requestOptions: options });
}

async function waitForPromptDecision(page, submissionId, timeout = 120_000) {
  await expect.poll(async () => {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) {
      throw new Error(`prompt submission ${submissionId} ended as ${status.state}: ${status.error || ''}`);
    }
    return status.decision || null;
  }, { timeout, intervals: [1000, 1500, 2500] }).not.toBeNull();
  const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionId)}`);
  return status.decision;
}

function eventPayload(event) {
  const payload = event?.payload || {};
  if (typeof payload === 'string') {
    try {
      return JSON.parse(payload);
    } catch (_err) {
      return {};
    }
  }
  return payload;
}

function parseToolOutput(payload) {
  const raw = payload?.output;
  if (typeof raw !== 'string') return raw || {};
  try {
    return JSON.parse(raw);
  } catch (_err) {
    return { raw_output: raw };
  }
}

async function traceToolResults(page, trajectoryId, toolName) {
  const snapshot = await fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(trajectoryId)}`);
  const results = [];
  for (const moment of snapshot.moments || []) {
    if (moment.kind !== 'tool.result') continue;
    const detail = await fetchJSON(
      page,
      `/api/trace/trajectories/${encodeURIComponent(trajectoryId)}/moments/${encodeURIComponent(moment.moment_id)}`
    );
    for (const event of detail.events || []) {
      const payload = eventPayload(event);
      if (payload.tool !== toolName) continue;
      results.push({ moment, event, payload, output: parseToolOutput(payload) });
    }
  }
  return { snapshot, results };
}

async function waitForSuccessfulSourceSearch(page, trajectoryId, timeout = 180_000) {
  const deadline = Date.now() + timeout;
  let lastError = '';
  while (Date.now() < deadline) {
    const { snapshot, results } = await traceToolResults(page, trajectoryId, 'source_search');
    for (const result of results) {
      if (result.payload.is_error) {
        lastError = result.output.raw_output || result.payload.output || 'source_search failed';
        continue;
      }
      const hits = Array.isArray(result.output.results) ? result.output.results : [];
      const hit = hits.find((item) => item.target_kind === 'source_service_item' && item.item_id);
      if (hit) return { snapshot, result, hit };
    }
    await page.waitForTimeout(2500);
  }
  throw new Error(`trajectory ${trajectoryId} never produced source_search source_service_item result; lastError=${lastError}`);
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

async function waitForSourceEntityRevision(page, docId, itemId, timeout = 180_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const state = await loadVTextState(page, docId);
    const sourceEntities = state.head?.metadata?.source_entities || [];
    const hasEntity = sourceEntities.some((entity) =>
      entity?.target?.target_kind === 'source_service_item' &&
      entity?.target?.item_id === itemId
    );
    const consumedResearcher = (state.head?.metadata?.worker_updates_consumed || [])
      .some((entry) => entry.role === 'researcher');
    if (hasEntity && consumedResearcher && state.head?.author_kind === 'appagent') {
      return { ...state, sourceEntities };
    }
    await page.waitForTimeout(2500);
  }
  const state = await loadVTextState(page, docId);
  throw new Error(`document ${docId} never produced source_service_item metadata for ${itemId}; head=${JSON.stringify(state.head)}`);
}

test('deployed researcher source_search becomes VText source entities', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const marker = `SOURCE_SERVICE_RESEARCH_${Date.now()}`;
  const prompt = [
    `Create a VText briefing titled ${marker}.`,
    'Use a researcher coagent with the source_search tool for current economy evidence from the platform Source Service.',
    'The researcher update must include refs like source_service_item:<id> so VText can preserve source_entities metadata.',
    'Write a concise revision that includes the marker and a visible citation marker for the source-service item.',
  ].join(' ');

  const promptBarResponse = page.waitForResponse((response) =>
    new URL(response.url()).pathname === '/api/prompt-bar' && response.request().method() === 'POST'
  );
  await page.locator('[data-prompt-input]').fill(prompt);
  await page.locator('[data-prompt-input]').press('Enter');

  const response = await promptBarResponse;
  expect(response.status()).toBe(202);
  const body = await response.json();
  const decision = await waitForPromptDecision(page, body.submission_id);
  expect(decision.action).toBe('open_app');
  expect(decision.app).toBe('vtext');
  expect(decision.doc_id).toBeTruthy();

  const { snapshot, hit } = await waitForSuccessfulSourceSearch(page, body.submission_id);
  expect((snapshot.agents || []).some((agent) => agent.role === 'researcher' || agent.profile === 'researcher')).toBe(true);
  expect(hit.source_id).toBeTruthy();
  expect(hit.fetch_id).toBeTruthy();

  const finalState = await waitForSourceEntityRevision(page, decision.doc_id, hit.item_id);
  expect(finalState.head.content).toContain(marker);
  expect(finalState.head.content).toContain(`source:${finalState.sourceEntities[0].entity_id}`);
  expect(finalState.sourceEntities[0]).toMatchObject({
    kind: 'source_service_item',
    target: {
      target_kind: 'source_service_item',
      item_id: hit.item_id,
    },
    display: {
      inline_mode: 'collapsed_citation',
      open_surface: 'source',
    },
    evidence: {
      research_state: 'represented',
    },
  });
  await expect(page.locator('[data-desktop-icon-id="trace"]')).toHaveCount(0);
});
