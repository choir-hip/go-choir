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
  return `vtext-worker-demo-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
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

async function loadVTextState(page, docId) {
  const [doc, revisions] = await Promise.all([
    fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(docId)}`),
    fetchJSON(page, `/api/vtext/documents/${encodeURIComponent(docId)}/revisions`),
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
    const state = await loadVTextState(page, docId);
    const content = state.head?.content || '';
    if (checks.every((check) => typeof check === 'string' ? content.includes(check) : check.test(content))) {
      return state;
    }
    await page.waitForTimeout(1500);
  }
  const state = await loadVTextState(page, docId);
  throw new Error(`final vtext document did not include background worker proof material; head=${JSON.stringify(state.head)}`);
}

function exportedPatchsetsFromResults(results) {
  return results.flatMap((result) =>
    (result.output?.export_patchsets || [])
      .filter((item) => item?.status === 'exported')
      .map((item) => ({
        worker_vm_id: result.output.worker_vm_id,
        manifest_path: item.manifest_path,
        patchset_path: item.patchset_path,
        base_sha: item.base_sha,
        worker_head: item.worker_head,
        github_push: item.github_push,
      }))
  );
}

function contentIncludesExport(content, item) {
  return Boolean(
    item?.worker_vm_id &&
    item?.manifest_path &&
    item?.patchset_path &&
    item?.base_sha &&
    item?.worker_head &&
    content.includes(item.worker_vm_id) &&
    content.includes(item.manifest_path) &&
    content.includes(item.patchset_path) &&
    content.includes(item.base_sha) &&
    content.includes(item.worker_head)
  );
}

async function waitForDocumentContentWithExport(page, docId, trajectoryId, marker, timeout = 180_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const state = await loadVTextState(page, docId);
    const content = state.head?.content || '';
    const delegated = await loadToolResults(page, trajectoryId, 'delegate_worker_vm');
    const exportedPatchsets = exportedPatchsetsFromResults(delegated.results);
    const matchedExport = exportedPatchsets.find((item) => contentIncludesExport(content, item));
    if (
      state.head?.metadata?.source === 'edit_vtext' &&
      content.includes(marker) &&
      /verified|verification|grep|passed/i.test(content) &&
      matchedExport
    ) {
      return { state, matchedExport, exportedPatchsets };
    }
    await page.waitForTimeout(1500);
  }
  const state = await loadVTextState(page, docId);
  throw new Error(`final vtext document did not include concrete exported patchset proof material; head=${JSON.stringify(state.head)}`);
}

test('prompt bar can route coding work through a background worker VM export', async ({ page, authenticator }) => {
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
    `Create a vtext document for ${marker}.`,
    'This is a coding workflow proof, not a research brief.',
    'VText should ask super to perform the mutable coding work in a background worker VM, not in the active desktop VM.',
    `The worker should create a tiny git repository containing README.md with the literal marker ${marker},`,
    'commit the change, verify it with grep, and call export_patchset.',
    'The final VText document must report the worker VM id, export manifest path, patchset path, base SHA, worker head SHA, and verification result.',
  ].join(' ');

  const promptBarResponse = page.waitForResponse((response) =>
    response.url().includes('/api/prompt-bar') && response.request().method() === 'POST'
  );
  await page.locator('[data-prompt-input]').fill(prompt);
  await page.locator('[data-prompt-input]').press('Enter');

  const submitted = await (await promptBarResponse).json();
  const decision = await waitForPromptDecision(page, submitted.submission_id);
  expect(decision.action).toBe('open_app');
  expect(decision.app).toBe('vtext');
  expect(decision.doc_id || '').toBeTruthy();

  const initial = await loadVTextState(page, decision.doc_id);
  const v1 = (initial.revisions.revisions || []).find((revision) => revision.revision_id === decision.framing_revision_id);
  expect(v1?.content || '').toContain(marker);
  expect(v1?.content || '').not.toMatch(/Conductor framing|Use this vtext|User request:/);

  await waitForToolResult(page, submitted.submission_id, 'request_worker_vm', (output) =>
    output?.status === 'worker_requested' &&
    output?.handle?.vm_id &&
    output?.handle?.sandbox_url
  );

  const delegated = await waitForToolResult(page, submitted.submission_id, 'delegate_worker_vm', (output) =>
    output?.status === 'worker_run_completed' &&
    output?.worker_vm_id &&
    Array.isArray(output?.export_patchsets) &&
    output.export_patchsets.some((item) =>
      item?.status === 'exported' &&
      item?.github_push === false &&
      item?.manifest_path &&
      item?.patchset_path &&
      item?.base_sha &&
      item?.worker_head
    )
  );

  const initialExportedPatchsets = exportedPatchsetsFromResults(delegated.results);
  expect(initialExportedPatchsets.length).toBeGreaterThan(0);
  expect(initialExportedPatchsets.every((item) => item.github_push === false)).toBe(true);

  const { state: finalState, exportedPatchsets } = await waitForDocumentContentWithExport(
    page,
    decision.doc_id,
    submitted.submission_id,
    marker
  );
  expect(finalState.head.metadata.source).toBe('edit_vtext');
  const finalContent = finalState.head.content || '';
  expect(exportedPatchsets.some((item) => contentIncludesExport(finalContent, item))).toBe(true);

  expect(forbiddenBrowserRequests).toHaveLength(0);

  const trace = delegated.snapshot;
  const roles = (trace.agents || []).map((agent) => agent.role || agent.profile || agent.label);
  expect(roles).toEqual(expect.arrayContaining(['conductor', 'vtext', 'super']));

  const acceptance = await postJSON(page, '/api/run-acceptances/synthesize', {
    target_mission_id: 'mission-run-acceptance-verification-v0',
    source_prompt_or_objective: prompt,
    trajectory_id: submitted.submission_id,
    staging_url: new URL(BASE_URL).origin,
  });
  expect(acceptance.acceptance_level).toBe('export-level');
  expect(acceptance.state).toBe('accepted');
  const checkpointKinds = (acceptance.checkpoints || []).map((checkpoint) => checkpoint.kind);
  expect(checkpointKinds).toEqual(expect.arrayContaining([
    'submitted',
    'vtext_opened',
    'super_requested',
    'worker_leased',
    'worker_delegated',
    'export_observed',
    'promotion_candidate_queued',
    'rollback_available',
  ]));
  expect(acceptance.evidence_refs?.length || 0).toBeGreaterThan(4);
  const promotionEvidence = (acceptance.evidence_refs || []).find((ref) => ref.kind === 'promotion_candidate');
  expect(promotionEvidence?.details?.patchset_sha256 || '').toMatch(/^[a-f0-9]{64}$/);
  expect(promotionEvidence?.details?.patchset_path || '').toContain('/promotion-artifacts/');
  expect(acceptance.gateway_provider_evidence || '').toContain('active_provider=');
  expect(acceptance.base_sha || '').toBeTruthy();

  const storedAcceptance = await fetchJSON(page, `/api/run-acceptances/${encodeURIComponent(acceptance.acceptance_id)}`);
  expect(storedAcceptance.acceptance_id).toBe(acceptance.acceptance_id);
});
