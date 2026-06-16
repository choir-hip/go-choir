import { test, expect } from './helpers/fixtures.js';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(360_000);
test.skip(
  process.env.GO_CHOIR_RUN_DEPLOYED_SOURCE_SERVICE_RESEARCH !== '1',
  'set GO_CHOIR_RUN_DEPLOYED_SOURCE_SERVICE_RESEARCH=1 to verify deployed researcher source_search -> Texture source entities'
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
  const hitsByItemId = new Map();
  let latestSnapshot = null;
  while (Date.now() < deadline) {
    const { snapshot, results } = await traceToolResults(page, trajectoryId, 'source_search');
    latestSnapshot = snapshot;
    for (const result of results) {
      if (result.payload.is_error) {
        lastError = result.output.raw_output || result.payload.output || 'source_search failed';
        continue;
      }
      const hits = Array.isArray(result.output.results) ? result.output.results : [];
      for (const hit of hits) {
        if (hit.target_kind === 'source_service_item' && hit.item_id) {
          hitsByItemId.set(hit.item_id, hit);
        }
      }
      if (hitsByItemId.size > 0) {
        return { snapshot, result, hitsByItemId };
      }
    }
    await page.waitForTimeout(2500);
  }
  throw new Error(`trajectory ${trajectoryId} never produced source_search source_service_item result; lastError=${lastError}; moments=${latestSnapshot?.moments?.length || 0}`);
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

async function waitForSourceEntityRevision(page, docId, candidateItemIds, timeout = 180_000) {
  const candidates = new Set(candidateItemIds);
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const state = await loadTextureState(page, docId);
    const sourceEntities = state.head?.metadata?.source_entities || [];
    const matchedEntity = sourceEntities.find((entity) =>
      entity?.target?.target_kind === 'source_service_item' &&
      candidates.has(entity?.target?.item_id)
    );
    const consumedResearcher = (state.head?.metadata?.worker_updates_consumed || [])
      .some((entry) => entry.role === 'researcher');
    if (matchedEntity && consumedResearcher && state.head?.author_kind === 'appagent') {
      return { ...state, sourceEntities, matchedEntity };
    }
    await page.waitForTimeout(2500);
  }
  const state = await loadTextureState(page, docId);
  throw new Error(`document ${docId} never produced source_service_item metadata for any searched item ${JSON.stringify([...candidates])}; head=${JSON.stringify(state.head)}`);
}

test('deployed researcher source_search becomes Texture source entities', async ({ desktopSession }) => {
  const { page } = desktopSession;
  const marker = `SOURCE_SERVICE_RESEARCH_${Date.now()}`;
  const prompt = [
    `Create a Texture briefing titled ${marker}.`,
    'Use a researcher coagent with the source_search tool for current economy evidence from the platform Source Service.',
    'The researcher update must include refs like source_service_item:<id> so Texture can preserve source_entities metadata.',
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
  expect(decision.app).toBe('texture');
  expect(decision.doc_id).toBeTruthy();

  const { snapshot, hitsByItemId } = await waitForSuccessfulSourceSearch(page, body.submission_id);
  expect((snapshot.agents || []).some((agent) => agent.role === 'researcher' || agent.profile === 'researcher')).toBe(true);
  const firstHit = [...hitsByItemId.values()][0];
  expect(firstHit).toBeTruthy();
  expect(firstHit.source_id).toBeTruthy();
  expect(firstHit.fetch_id).toBeTruthy();

  const finalState = await waitForSourceEntityRevision(page, decision.doc_id, hitsByItemId.keys());
  expect(finalState.head.content).toContain(marker);
  expect(finalState.head.content).toContain(`source:${finalState.matchedEntity.entity_id}`);
  expect(finalState.matchedEntity).toMatchObject({
    kind: 'source_service_item',
    target: {
      target_kind: 'source_service_item',
      item_id: finalState.matchedEntity.target.item_id,
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
