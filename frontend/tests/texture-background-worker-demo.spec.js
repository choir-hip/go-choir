import { test, expect } from './helpers/fixtures.js';
import { registerPasskey } from './helpers/auth.js';

const BASE_URL = process.env.GO_CHOIR_WORKER_DEMO_BASE_URL ||
  process.env.PLAYWRIGHT_BASE_URL ||
  'http://localhost:4173';

test.use({ trace: 'on', video: 'on', screenshot: 'on' });
test.setTimeout(420_000);
test.skip(
  process.env.GO_CHOIR_RUN_BACKGROUND_WORKER_DEMO !== '1',
  'set GO_CHOIR_RUN_BACKGROUND_WORKER_DEMO=1 to prove product-path background worker execution'
);

function uniqueEmail() {
  return `texture-worker-demo-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

async function registerAndLoadDesktop(page, email) {
  await page.goto(BASE_URL);
  await registerPasskey(page, email, BASE_URL);
  await page.reload();
  await page.locator('[data-desktop].desktop-ready').waitFor({ state: 'visible', timeout: 120000 });
}

async function fetchJSON(page, path) {
  return page.evaluate(async (requestPath) => {
    let res = await fetch(requestPath, { credentials: 'include' });
    if (res.status === 401) {
      await fetch('/auth/session', { credentials: 'include' }).catch(() => null);
      res = await fetch(requestPath, { credentials: 'include' });
    }
    if (!res.ok) {
      const body = await res.text();
      throw new Error(`${requestPath} failed: ${res.status} ${body}`);
    }
    return res.json();
  }, path);
}

async function postJSON(page, path, body) {
  return page.evaluate(async ({ requestPath, payload }) => {
    let res = await fetch(requestPath, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify(payload),
    });
    if (res.status === 401) {
      await fetch('/auth/session', { credentials: 'include' }).catch(() => null);
      res = await fetch(requestPath, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(payload),
      });
    }
    if (!res.ok) {
      const responseBody = await res.text();
      throw new Error(`${requestPath} failed: ${res.status} ${responseBody}`);
    }
    return res.json();
  }, { requestPath: path, payload: body });
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
  const [doc, revisions] = await Promise.all([
    fetchJSON(page, `/api/texture/documents/${encodeURIComponent(docId)}`),
    fetchJSON(page, `/api/texture/documents/${encodeURIComponent(docId)}/revisions`),
  ]);
  const head = (revisions.revisions || []).find((revision) => revision.revision_id === doc.current_revision_id);
  return { doc, revisions, head };
}

async function loadToolResults(page, trajectoryId, toolName) {
  const snapshot = await fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(trajectoryId)}`);
  const toolMoments = (snapshot.moments || []).filter((moment) =>
    moment.kind === 'tool.result' && moment.summary === `${toolName} returned`
  );
  const results = [];
  for (const moment of toolMoments) {
    const detail = await fetchJSON(
      page,
      `/api/trace/trajectories/${encodeURIComponent(trajectoryId)}/moments/${encodeURIComponent(moment.moment_id)}`
    );
    for (const event of detail.events || []) {
      const payload = event.payload || {};
      if (payload.tool !== toolName || payload.is_error) continue;
      let output = payload.output;
      if (typeof output === 'string') {
        try {
          output = JSON.parse(output);
        } catch {
          output = { raw_output: payload.output };
        }
      }
      results.push({ moment, event, output });
    }
  }
  return { snapshot, results };
}

async function waitForToolResult(page, trajectoryId, toolName, predicate, timeout = 240_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const loaded = await loadToolResults(page, trajectoryId, toolName);
    const match = loaded.results.find((result) => predicate(result.output, result));
    if (match) return { ...loaded, match };
    await page.waitForTimeout(1500);
  }
  const loaded = await loadToolResults(page, trajectoryId, toolName);
  throw new Error(`trajectory ${trajectoryId} did not produce expected ${toolName} result; saw ${JSON.stringify(loaded.results.map((result) => result.output))}`);
}

async function waitForDocumentContent(page, docId, checks, timeout = 180_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const state = await loadTextureState(page, docId);
    const content = state.head?.content || '';
    if (checks.every((check) => typeof check === 'string' ? content.includes(check) : check.test(content))) {
      return state;
    }
    await page.waitForTimeout(1500);
  }
  const state = await loadTextureState(page, docId);
  throw new Error(`final texture document did not include background worker proof material; head=${JSON.stringify(state.head)}`);
}

function appChangePackagesFromResults(results) {
  return results.flatMap((result) =>
    (result.output?.app_change_packages || [])
      .filter((item) => item?.package_id && String(item?.status || '').startsWith('published'))
      .map((item) => ({
        worker_vm_id: result.output.worker_vm_id,
        package_id: item.package_id,
        package_manifest_sha256: item.package_manifest_sha256,
        runtime_source_delta_sha256: item.runtime_source_delta_sha256,
        ui_source_delta_sha256: item.ui_source_delta_sha256,
        base_sha: item.base_sha,
        candidate_head_sha: item.candidate_head_sha,
        source_candidate_id: item.source_candidate_id,
        github_push: item.github_push,
      }))
  );
}

function contentIncludesPackage(content, item) {
  return Boolean(
    item?.worker_vm_id &&
    item?.package_id &&
    item?.package_manifest_sha256 &&
    item?.base_sha &&
    item?.candidate_head_sha &&
    content.includes(item.worker_vm_id) &&
    content.includes(item.package_id) &&
    content.includes(item.package_manifest_sha256) &&
    content.includes(item.base_sha) &&
    content.includes(item.candidate_head_sha)
  );
}

async function waitForDocumentContentWithPackage(page, docId, trajectoryId, marker, timeout = 180_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const state = await loadTextureState(page, docId);
    const content = state.head?.content || '';
    const delegated = await loadToolResults(page, trajectoryId, 'delegate_worker_vm');
    const appChangePackages = appChangePackagesFromResults(delegated.results);
    const matchedPackage = appChangePackages.find((item) => contentIncludesPackage(content, item));
    if (
      state.head?.metadata?.source === 'edit_texture' &&
      content.includes(marker) &&
      /verified|verification|grep|passed/i.test(content) &&
      matchedPackage
    ) {
      return { state, matchedPackage, appChangePackages };
    }
    await page.waitForTimeout(1500);
  }
  const state = await loadTextureState(page, docId);
  throw new Error(`final texture document did not include concrete AppChangePackage proof material; head=${JSON.stringify(state.head)}`);
}

test('prompt bar can route coding work through a background worker VM AppChangePackage', async ({ page, authenticator }) => {
  await registerAndLoadDesktop(page, uniqueEmail());

  const forbiddenBrowserRequests = [];
  page.on('request', (request) => {
    const url = new URL(request.url());
    const forbidden =
      url.pathname.startsWith('/internal/') ||
      url.pathname.startsWith('/api/agent/') ||
      url.pathname.startsWith('/api/prompts') ||
      url.pathname.startsWith('/api/test/') ||
      url.pathname === '/api/events';
    if (forbidden) forbiddenBrowserRequests.push(`${request.method()} ${url.pathname}`);
  });

  const marker = `BACKGROUND_WORKER_DEMO_${Date.now()}`;
  const prompt = [
    `Create a texture document for ${marker}.`,
    'This is a coding workflow proof, not a research brief.',
    'Texture should ask super to perform the mutable coding work in a background worker VM, not in the active desktop VM.',
    `The worker should create a tiny git repository containing README.md with the literal marker ${marker},`,
    'commit the change, verify it with grep, and call publish_app_change_package.',
    'The final Texture document must report the worker VM id, AppChangePackage id, package manifest SHA, source delta SHAs, base SHA, candidate head SHA, and verification result.',
  ].join(' ');

  const promptBarResponse = page.waitForResponse((response) =>
    response.url().includes('/api/prompt-bar') && response.request().method() === 'POST'
  );
  await page.locator('[data-prompt-input]').fill(prompt);
  await page.locator('[data-prompt-input]').press('Enter');

  const submitted = await (await promptBarResponse).json();
  const decision = await waitForPromptDecision(page, submitted.submission_id);
  expect(decision.action).toBe('open_app');
  expect(decision.app).toBe('texture');
  expect(decision.doc_id || '').toBeTruthy();

  const initial = await loadTextureState(page, decision.doc_id);
  const v1 = (initial.revisions.revisions || []).find((revision) => revision.revision_id === decision.framing_revision_id);
  expect(v1?.content || '').toContain(marker);
  expect(v1?.content || '').not.toMatch(/Conductor framing|Use this texture|User request:/);

  await waitForToolResult(page, submitted.submission_id, 'request_worker_vm', (output) =>
    output?.status === 'worker_requested' &&
    output?.handle?.vm_id &&
    output?.handle?.sandbox_url
  );

  const delegated = await waitForToolResult(page, submitted.submission_id, 'delegate_worker_vm', (output) =>
    output?.status === 'worker_run_completed' &&
    output?.worker_vm_id &&
    Array.isArray(output?.app_change_packages) &&
    output.app_change_packages.some((item) =>
      String(item?.status || '').startsWith('published') &&
      item?.github_push === false &&
      item?.package_id &&
      item?.package_manifest_sha256 &&
      item?.base_sha &&
      item?.candidate_head_sha
    )
  );

  const initialAppChangePackages = appChangePackagesFromResults(delegated.results);
  expect(initialAppChangePackages.length).toBeGreaterThan(0);
  expect(initialAppChangePackages.every((item) => item.github_push === false)).toBe(true);

  const { state: finalState, appChangePackages } = await waitForDocumentContentWithPackage(
    page,
    decision.doc_id,
    submitted.submission_id,
    marker
  );
  expect(finalState.head.metadata.source).toBe('edit_texture');
  const finalContent = finalState.head.content || '';
  expect(appChangePackages.some((item) => contentIncludesPackage(finalContent, item))).toBe(true);

  expect(forbiddenBrowserRequests).toHaveLength(0);

  const trace = delegated.snapshot;
  const roles = (trace.agents || []).map((agent) => agent.role || agent.profile || agent.label);
  expect(roles).toEqual(expect.arrayContaining(['conductor', 'texture', 'super']));

  const acceptance = await postJSON(page, '/api/run-acceptances/synthesize', {
    target_mission_id: 'mission-run-acceptance-verification-v0',
    source_prompt_or_objective: prompt,
    trajectory_id: submitted.submission_id,
    staging_url: new URL(BASE_URL).origin,
  });
  expect(acceptance.state).toBe('blocked');
  const checkpointKinds = (acceptance.checkpoints || []).map((checkpoint) => checkpoint.kind);
  expect(checkpointKinds).toEqual(expect.arrayContaining([
    'submitted',
    'texture_opened',
    'super_requested',
    'worker_leased',
    'worker_delegated',
    'app_package_published',
  ]));
  expect(acceptance.evidence_refs?.length || 0).toBeGreaterThan(4);
  const packageEvidence = (acceptance.evidence_refs || []).find((ref) => ref.kind === 'tool.result' && ref.summary?.includes('AppChangePackage'));
  expect(packageEvidence?.details?.package_count || 0).toBeGreaterThan(0);
  expect(acceptance.gateway_provider_evidence || '').toContain('active_provider=');
  expect(acceptance.base_sha || '').toBeTruthy();

  const storedAcceptance = await fetchJSON(page, `/api/run-acceptances/${encodeURIComponent(acceptance.acceptance_id)}`);
  expect(storedAcceptance.acceptance_id).toBe(acceptance.acceptance_id);
});
