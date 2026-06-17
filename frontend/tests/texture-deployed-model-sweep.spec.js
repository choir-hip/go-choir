import { test, expect } from '@playwright/test';
import { setupVirtualAuthenticator, removeVirtualAuthenticator } from './helpers/webauthn.js';
import { registerPasskey } from './helpers/auth.js';
import { waitForDesktopReady } from './helpers/auth-state.js';

// Deployed model sweep for the prompt-bar -> Texture live-search flow.
//
// Each arm pins an owner-visible model-policy overlay onto the whole trajectory
// (texture + the researchers it spawns) via PUT /api/files, then drives the
// overlay-pinned eval endpoint POST /api/evals/texture-prompt. Verification
// reuses the same public product routes as the live-search spec: trace moments
// for web_search evidence and Texture revisions for grounded output.
//
// Arms are run sequentially and per-arm failures are captured (not fail-fast)
// so the sweep always reports a full comparison table.

const BASE_URL = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(2_400_000);

test.skip(
  process.env.GO_CHOIR_RUN_DEPLOYED_MODEL_SWEEP !== '1',
  'set GO_CHOIR_RUN_DEPLOYED_MODEL_SWEEP=1 to run the deployed Texture model sweep'
);

const ARMS = [
  { id: 'sweep-glm-5-2-medium', provider: 'zai', model: 'glm-5.2', reasoning: 'medium' },
  { id: 'sweep-mimo-v2-5-medium', provider: 'xiaomi', model: 'mimo-v2.5', reasoning: 'medium' },
  { id: 'sweep-mimo-v2-5-pro-medium', provider: 'xiaomi', model: 'mimo-v2.5-pro', reasoning: 'medium' },
  { id: 'sweep-gpt-5-4-mini-medium', provider: 'chatgpt', model: 'gpt-5.4-mini', reasoning: 'medium' },
  { id: 'sweep-gpt-5-5-low', provider: 'chatgpt', model: 'gpt-5.5', reasoning: 'low' },
];

const PROMPT = [
  'Create a texture briefing about what is new in AI infrastructure news today, June 17, 2026.',
  'Use live researcher web_search evidence before making the substantive revision.',
  'The final document should include source-grounded 2026 evidence and should not rely on model-prior knowledge.',
].join(' ');

function uniqueEmail() {
  return `texture-model-sweep-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

function overlayToml(arm) {
  // Expire ~3h out so a re-run with a fresh suffix never collides with a stale arm.
  const expires = new Date(Date.now() + 3 * 60 * 60 * 1000).toISOString();
  return [
    '[overlay]',
    `expires_at = "${expires}"`,
    '',
    '[roles.texture]',
    `provider = "${arm.provider}"`,
    `model = "${arm.model}"`,
    `reasoning = "${arm.reasoning}"`,
    '',
    '[roles.researcher]',
    `provider = "${arm.provider}"`,
    `model = "${arm.model}"`,
    `reasoning = "${arm.reasoning}"`,
    '',
  ].join('\n');
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

async function putFile(page, path, contents) {
  return page.evaluate(async ({ requestPath, body }) => {
    const res = await fetch(requestPath, {
      method: 'PUT',
      credentials: 'include',
      headers: { 'Content-Type': 'text/plain' },
      body,
    });
    const text = await res.text();
    return { status: res.status, body: text };
  }, { requestPath: path, body: contents });
}

async function ensureDir(page, path) {
  // POST /api/files/{path} creates a directory; 201 created or 409 already
  // exists are both acceptable. Parent must already exist, so callers ensure
  // ancestors first.
  const res = await page.evaluate(async (requestPath) => {
    const r = await fetch(requestPath, { method: 'POST', credentials: 'include' });
    const text = await r.text();
    return { status: r.status, body: text };
  }, path);
  if (![201, 409].includes(res.status)) {
    throw new Error(`mkdir ${path} -> ${res.status} ${res.body}`);
  }
  return res;
}

async function postJSON(page, path, payload) {
  return page.evaluate(async ({ requestPath, body }) => {
    const res = await fetch(requestPath, {
      method: 'POST',
      credentials: 'include',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const text = await res.text();
    return { status: res.status, body: text ? JSON.parse(text) : null };
  }, { requestPath: path, body: payload });
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await waitForDesktopReady(page, 120_000);
}

async function waitForStagingReady(page, timeout = 180_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const health = await page.evaluate(async () => {
      const res = await fetch('/health', { credentials: 'include' });
      return res.ok ? res.json() : { status: 'error', upstream: 'error' };
    });
    if (health.status === 'ok' && health.upstream === 'ok') return health;
    await page.waitForTimeout(5000);
  }
  throw new Error('staging health never reached status=ok upstream=ok');
}

function eventPayload(event) {
  if (!event?.payload) return {};
  if (typeof event.payload === 'string') {
    try { return JSON.parse(event.payload); } catch { return {}; }
  }
  return event.payload;
}

function parseToolOutput(payload) {
  const raw = payload?.output;
  if (typeof raw !== 'string') return raw || {};
  try { return JSON.parse(raw); } catch { return { raw }; }
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

async function waitForSuccessfulWebSearch(page, trajectoryId, timeout = 300_000) {
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
        if (output.provider && results.length > 0 && /2026/.test(JSON.stringify(results))) {
          return { provider: output.provider, providers: output.providers || [], results };
        }
      }
    }
    await page.waitForTimeout(2500);
  }
  throw new Error(`no successful 2026 web_search; errors=${lastSearchErrors.join(' | ')}`);
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

async function waitForGroundedTextureRevision(page, docId, timeout = 360_000) {
  const deadline = Date.now() + timeout;
  let last = null;
  while (Date.now() < deadline) {
    const state = await loadTextureState(page, docId);
    const revisions = state.revisions || [];
    const v0 = revisions.find((revision) => revision.version_number === 0);
    const appagentRevisions = revisions.filter((revision) => revision.author_kind === 'appagent');
    const consumed = revisions.flatMap((revision) => revision?.metadata?.worker_updates_consumed || []);
    const consumedResearch = consumed.some((item) => item.role === 'researcher');
    const content = state.head?.content || '';
    last = {
      revision_count: revisions.length,
      appagent_revision_count: appagentRevisions.length,
      v0_matches_prompt: v0?.content === PROMPT,
      consumed_researcher: consumedResearch,
      head_len: content.length,
    };
    // A grounded integrate is grounded whether the model wrote one combined
    // revision or an initial draft plus an integrate revision, so the gate is
    // >= 1 appagent revision that consumed researcher findings with live
    // 2026 evidence in the head — not a fixed revision count.
    if (
      v0 &&
      v0.content === PROMPT &&
      consumedResearch &&
      appagentRevisions.length >= 1 &&
      /2026/.test(content) &&
      /https?:\/\/|source|evidence|fetched|search/i.test(content) &&
      !/search (?:was )?unavailable|search unavailable|model knowledge through|mid-2024|broad trend extrapolation/i.test(content)
    ) {
      return { state, appagentRevisions, last };
    }
    await page.waitForTimeout(2500);
  }
  throw new Error(`no grounded revision; last=${JSON.stringify(last)}`);
}

test('deployed Texture model sweep across pinned overlay arms', async ({ browser }) => {
  const context = await browser.newContext();
  const page = await context.newPage();
  const { client, authenticatorId } = await setupVirtualAuthenticator(page);

  const results = [];
  try {
    await registerAndLoadDesktop(page, uniqueEmail());
    const health = await waitForStagingReady(page);
    const build = health.build?.commit || health.build || health;
    console.log('staging health build:', JSON.stringify(build));

    // Fresh owners do not have System/model-policy-overlays yet; create the
    // ancestor chain before any overlay PUT (parent must exist for PUT).
    await ensureDir(page, '/api/files/System');
    await ensureDir(page, '/api/files/System/model-policy-overlays');

    for (const arm of ARMS) {
      const armResult = { arm: arm.model, reasoning: arm.reasoning, overlay: arm.id };
      const started = Date.now();
      try {
        const put = await putFile(
          page,
          `/api/files/System/model-policy-overlays/${arm.id}.toml`,
          overlayToml(arm)
        );
        if (put.status !== 200 && put.status !== 201 && put.status !== 204) {
          throw new Error(`overlay PUT ${arm.id} -> ${put.status} ${put.body}`);
        }

        const post = await postJSON(page, '/api/evals/texture-prompt', {
          text: PROMPT,
          model_policy_overlay_id: arm.id,
        });
        if (post.status !== 202) {
          throw new Error(`eval POST -> ${post.status} ${JSON.stringify(post.body)}`);
        }
        armResult.submission_id = post.body.submission_id;
        armResult.doc_id = post.body.doc_id;
        armResult.resolved = `${post.body.provider}/${post.body.model}/${post.body.reasoning_effort}`;
        expect(post.body.model).toBe(arm.model);

        const search = await waitForSuccessfulWebSearch(page, post.body.submission_id);
        armResult.search_provider = (search.providers && search.providers[0]) || search.provider;
        armResult.search_results = search.results.length;

        const grounded = await waitForGroundedTextureRevision(page, post.body.doc_id);
        armResult.appagent_revisions = grounded.appagentRevisions.length;
        armResult.outcome = 'PASS';
      } catch (err) {
        armResult.outcome = 'FAIL';
        armResult.error = String(err && err.message ? err.message : err);
      } finally {
        armResult.elapsed_s = Math.round((Date.now() - started) / 1000);
        results.push(armResult);
        console.log(`[sweep arm] ${arm.model}/${arm.reasoning}: ${JSON.stringify(armResult)}`);
      }
    }
  } finally {
    console.log('=== MODEL SWEEP RESULTS ===');
    console.log(JSON.stringify(results, null, 2));
    await removeVirtualAuthenticator(client, authenticatorId);
    await context.close();
  }

  // The sweep is informational across arms; assert only that every arm reached
  // a recorded outcome so a crash mid-loop fails the run.
  expect(results.length).toBe(ARMS.length);
  for (const r of results) {
    expect(['PASS', 'FAIL']).toContain(r.outcome);
  }
});
