import { execFileSync } from 'node:child_process';
import { chromium } from '../frontend/node_modules/playwright/index.mjs';
import { registerPasskey } from '../frontend/tests/helpers/auth.js';
import {
  removeVirtualAuthenticator,
  setupVirtualAuthenticator,
} from '../frontend/tests/helpers/webauthn.js';

const runtimeProcess = globalThis.process || null;
const BASE_URL = runtimeProcess?.env?.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const marker = `M3_VMCTL_REFRESH_${Date.now()}`;

const result = {
  marker,
  base_url: BASE_URL,
  started_at: new Date().toISOString(),
  harness_version: '2026-06-17',
  predicate:
    'Pre-refresh proof waits for live delegated/coordinated work on the prompt-bar trajectory without forcing an exact researcher sequence. Refresh proof requires correct vmctl target identity before/after, post-refresh trajectory activity, a post-refresh Texture revision, at least one downstream worker update consumed, and no pending worker updates left on the head revision.',
};

function uniqueEmail() {
  return `m3-vmctl-refresh-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.com`;
}

function nodeB(command, input) {
  return execFileSync(
    'ssh',
    ['-o', 'BatchMode=yes', '-o', 'ConnectTimeout=8', 'node-b', command],
    {
      input,
      encoding: 'utf8',
      stdio: ['pipe', 'pipe', 'pipe'],
      maxBuffer: 20 * 1024 * 1024,
    },
  ).trim();
}

function nodeBJSON(command, input) {
  const raw = nodeB(command, input);
  return raw ? JSON.parse(raw) : null;
}

function vmctlOwnership(userID) {
  return nodeBJSON(
    `curl -fsS -H "X-Internal-Caller: true" http://127.0.0.1:8083/internal/vmctl/list | jq -c --arg u '${userID}' '.ownerships[] | select(.user_id==$u and .desktop_id=="primary" and .kind=="interactive")'`,
  );
}

function vmctlRefresh(userID) {
  return nodeBJSON(
    'curl -fsS -X POST -H "Content-Type: application/json" -H "X-Internal-Caller: true" --data-binary @- http://127.0.0.1:8083/internal/vmctl/refresh',
    JSON.stringify({ user_id: userID, desktop_id: 'primary' }),
  );
}

function vmHealth(sandboxURL) {
  if (!sandboxURL) return null;
  return nodeBJSON(
    `curl -fsS --max-time 5 '${sandboxURL.replace(/'/g, "'\\''")}/health' | jq -c '{status, service, runtime_health, running_runs, running_processor_runs, build, persistent_disk}'`,
  );
}

function parseJSON(text) {
  if (!text) return null;
  return JSON.parse(text);
}

async function fetchJSON(page, path, options = {}) {
  return page.evaluate(
    async ({ requestPath, requestOptions }) => {
      const init = {
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          ...(requestOptions.headers || {}),
        },
        ...requestOptions,
      };
      let res = await fetch(requestPath, init);
      if (res.status === 401) {
        await fetch('/auth/session', { credentials: 'include' }).catch(() => null);
        res = await fetch(requestPath, init);
      }
      const body = await res.text();
      if (!res.ok) {
        throw new Error(`${requestOptions.method || 'GET'} ${requestPath} failed: ${res.status} ${body}`);
      }
      return body ? JSON.parse(body) : null;
    },
    { requestPath: path, requestOptions: options },
  );
}

async function postJSON(page, path, body) {
  return fetchJSON(page, path, {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

async function bootstrap(page) {
  return fetchJSON(page, '/api/shell/bootstrap');
}

async function waitForDesktopReady(page, timeout = 180_000) {
  await page
    .locator('[data-desktop][data-authenticated="true"][data-desktop-ready="true"]')
    .waitFor({
      state: 'visible',
      timeout,
    });
}

async function loadTextureState(page, docID) {
  const [doc, revisionsResponse] = await Promise.all([
    fetchJSON(page, `/api/texture/documents/${encodeURIComponent(docID)}`),
    fetchJSON(page, `/api/texture/documents/${encodeURIComponent(docID)}/revisions`),
  ]);
  const revisions = revisionsResponse.revisions || [];
  const head = revisions.find((revision) => revision.revision_id === doc.current_revision_id) || null;
  return { doc, revisions, head };
}

async function loadTextureDiagnosis(page, docID, limit = 12) {
  return fetchJSON(
    page,
    `/api/texture/documents/${encodeURIComponent(docID)}/diagnosis?limit=${encodeURIComponent(String(limit))}`,
  );
}

async function loadTrace(page, trajectoryID) {
  return fetchJSON(page, `/api/trace/trajectories/${encodeURIComponent(trajectoryID)}`);
}

function eventPayload(event) {
  const payload = event?.payload || {};
  if (typeof payload === 'string') {
    try {
      return JSON.parse(payload);
    } catch {
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
  } catch {
    return { raw_output: raw };
  }
}

function roleName(agent) {
  return String(agent?.role || agent?.profile || agent?.label || '').trim().toLowerCase();
}

function uniqueSorted(values) {
  return [...new Set(values.filter(Boolean))].sort();
}

function isoMs(value) {
  const ms = Date.parse(value || '');
  return Number.isFinite(ms) ? ms : 0;
}

function stateTimeMs(obj) {
  return isoMs(obj?.created_at || obj?.updated_at || obj?.finished_at || '');
}

function headPendingWorkerUpdates(state) {
  return state?.head?.metadata?.worker_updates_pending || [];
}

function headConsumedWorkerUpdateRoles(state) {
  const consumed = state?.head?.metadata?.worker_updates_consumed || [];
  return uniqueSorted(consumed.map((entry) => String(entry?.role || '').trim().toLowerCase()));
}

function summarizeRun(run) {
  return {
    loop_id: run?.loop_id || run?.run_id || '',
    agent_id: run?.agent_id || '',
    agent_profile: run?.agent_profile || '',
    agent_role: run?.agent_role || '',
    state: run?.state || '',
    created_at: run?.created_at || '',
    updated_at: run?.updated_at || '',
    metadata: run?.metadata || {},
  };
}

async function waitForPromptDecision(page, submissionID, timeout = 180_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const status = await fetchJSON(page, `/api/prompt-bar/submissions/${encodeURIComponent(submissionID)}`);
    if (status.decision) return { status, decision: status.decision };
    if (['failed', 'blocked', 'cancelled'].includes(status.state)) {
      throw new Error(`prompt submission ${submissionID} ended as ${status.state}: ${status.error || ''}`);
    }
    await page.waitForTimeout(1000);
  }
  throw new Error(`prompt submission ${submissionID} did not produce a decision`);
}

async function waitForCoordinationInFlight(page, trajectoryID, docID, timeout = 240_000) {
  const deadline = Date.now() + timeout;
  const samples = [];
  while (Date.now() < deadline) {
    const [trace, diagnosis, state] = await Promise.all([
      loadTrace(page, trajectoryID),
      loadTextureDiagnosis(page, docID),
      loadTextureState(page, docID),
    ]);
    const roles = uniqueSorted((trace.agents || []).map(roleName));
    const downstreamRoles = roles.filter((role) => role !== 'conductor' && role !== 'texture');
    const requestMoments = (trace.moments || []).filter((moment) =>
      moment.kind === 'tool.result' &&
      (moment.summary === 'request_super_execution returned' || moment.summary === 'spawn_agent returned'),
    );
    const activeRuns = (diagnosis.runs || []).filter((run) => {
      const stateValue = String(run?.state || '').toLowerCase();
      return stateValue === 'running' || stateValue === 'passivated';
    });
    samples.push({
      at: new Date().toISOString(),
      live: Boolean(trace.trajectory?.live),
      roles,
      downstream_roles: downstreamRoles,
      request_moment_count: requestMoments.length,
      active_doc_run_count: activeRuns.length,
      revision_count: state.revisions.length,
      head_revision_id: state.head?.revision_id || null,
    });
    if (
      (trace.trajectory?.live || activeRuns.length > 0) &&
      (downstreamRoles.length > 0 || requestMoments.length > 0 || activeRuns.length > 0)
    ) {
      return { trace, diagnosis, state, roles, samples };
    }
    await page.waitForTimeout(1500);
  }
  throw new Error(`trajectory ${trajectoryID} never reached observable coordinated in-flight work; samples=${JSON.stringify(samples.slice(-8))}`);
}

async function waitForOwnershipAdvance(userID, beforeEpoch, timeout = 120_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const own = vmctlOwnership(userID);
    if (own && Number(own.epoch || 0) > Number(beforeEpoch || 0)) return own;
    await new Promise((resolve) => setTimeout(resolve, 1500));
  }
  throw new Error(`vmctl ownership epoch for ${userID} did not advance beyond ${beforeEpoch}`);
}

async function waitForVMHealth(sandboxURL, timeout = 180_000) {
  const deadline = Date.now() + timeout;
  let last = null;
  while (Date.now() < deadline) {
    try {
      last = vmHealth(sandboxURL);
      if (last?.status === 'ready' || last?.status === 'ok') return last;
    } catch (error) {
      last = { error: error.message };
    }
    await new Promise((resolve) => setTimeout(resolve, 1500));
  }
  throw new Error(`vm health never recovered for ${sandboxURL}: ${JSON.stringify(last)}`);
}

async function waitForRecoveredProgress(page, trajectoryID, docID, refreshAtMs, timeout = 420_000) {
  const deadline = Date.now() + timeout;
  const samples = [];
  while (Date.now() < deadline) {
    const [trace, diagnosis, state] = await Promise.all([
      loadTrace(page, trajectoryID),
      loadTextureDiagnosis(page, docID),
      loadTextureState(page, docID),
    ]);
    const consumedRoles = headConsumedWorkerUpdateRoles(state);
    const pending = headPendingWorkerUpdates(state);
    const headTime = stateTimeMs(state.head);
    const postRefreshMoments = (trace.moments || []).filter((moment) => isoMs(moment.timestamp) >= refreshAtMs);
    const postRefreshRuns = (diagnosis.runs || []).filter((run) => {
      const runTime = Math.max(isoMs(run?.created_at), isoMs(run?.updated_at));
      return runTime >= refreshAtMs;
    });
    const passivatedAfterRefresh = (diagnosis.runs || []).filter((run) => {
      const stateValue = String(run?.state || '').toLowerCase();
      return stateValue === 'passivated' && isoMs(run?.updated_at) >= refreshAtMs;
    });
    samples.push({
      at: new Date().toISOString(),
      head_revision_id: state.head?.revision_id || null,
      head_created_at: state.head?.created_at || null,
      head_source: state.head?.metadata?.source || null,
      consumed_roles: consumedRoles,
      pending_worker_updates: pending,
      post_refresh_moment_count: postRefreshMoments.length,
      post_refresh_run_count: postRefreshRuns.length,
      passivated_after_refresh: passivatedAfterRefresh.map(summarizeRun),
    });
    if (
      headTime >= refreshAtMs &&
      (state.head?.content || '').includes(marker) &&
      consumedRoles.some((role) => role !== 'conductor' && role !== 'texture') &&
      pending.length === 0 &&
      postRefreshMoments.length > 0 &&
      postRefreshRuns.length > 0
    ) {
      return { trace, diagnosis, state, consumedRoles, pending, samples };
    }
    await page.waitForTimeout(2000);
  }
  const [trace, diagnosis, state] = await Promise.all([
    loadTrace(page, trajectoryID).catch((error) => ({ error: String(error) })),
    loadTextureDiagnosis(page, docID).catch((error) => ({ error: String(error) })),
    loadTextureState(page, docID).catch((error) => ({ error: String(error) })),
  ]);
  throw new Error(
    `trajectory ${trajectoryID} did not show post-refresh recovery proof; samples=${JSON.stringify(samples.slice(-8))}; trace=${JSON.stringify(trace)}; diagnosis=${JSON.stringify(diagnosis)}; state=${JSON.stringify(state)}`,
  );
}

const browser = await chromium.launch();
const context = await browser.newContext();
const page = await context.newPage();
const { client, authenticatorId } = await setupVirtualAuthenticator(page);

try {
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded', timeout: 60_000 });

  result.email = uniqueEmail();
  await registerPasskey(page, result.email, BASE_URL);
  await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 });
  await waitForDesktopReady(page, 180_000);

  result.health_before = await fetchJSON(page, '/health');
  result.bootstrap_before = await bootstrap(page);
  result.compute_status_before = await fetchJSON(page, '/api/compute/status');

  const prompt = [
    `Create a Texture document for ${marker}.`,
    'Produce a concise lifecycle proof note about restart-safe coordination.',
    'Include one current fact or source-grounded evidence point if needed, and one verified execution or artifact note if needed.',
    'Choose whatever coordination path Texture judges appropriate; do not rely on a fixed role sequence.',
    `The final document must repeat ${marker}, explain what work completed after coordination, and name at least one concrete evidence handle, source, or verification result.`,
  ].join(' ');
  result.prompt = prompt;

  const promptBarResponse = page.waitForResponse(
    (response) => {
      const url = new URL(response.url());
      return url.pathname === '/api/prompt-bar' && response.request().method() === 'POST';
    },
    { timeout: 90_000 },
  );
  await page.locator('[data-prompt-input]').fill(prompt);
  await page.locator('[data-prompt-input]').press('Enter');

  const promptBar = await promptBarResponse;
  result.prompt_bar_status = promptBar.status();
  result.submission = await promptBar.json();
  if (result.prompt_bar_status !== 202) {
    throw new Error(`prompt-bar status = ${result.prompt_bar_status}`);
  }

  const { status, decision } = await waitForPromptDecision(page, result.submission.submission_id);
  result.prompt_status = status;
  result.decision = decision;
  if (decision.action !== 'open_app' || decision.app !== 'texture' || !decision.doc_id) {
    throw new Error(`unexpected prompt-bar decision ${JSON.stringify(decision)}`);
  }

  const initialState = await loadTextureState(page, decision.doc_id);
  result.initial_texture = {
    doc_id: initialState.doc.doc_id,
    owner_id: initialState.doc.owner_id,
    current_revision_id: initialState.doc.current_revision_id,
    current_version_number: initialState.doc.current_version_number,
    initial_loop_id: decision.initial_loop_id || null,
    revision_count: initialState.revisions.length,
  };

  const ownerID = initialState.doc.owner_id;
  result.vmctl_before = vmctlOwnership(ownerID);
  if (!result.vmctl_before?.sandbox_url) {
    throw new Error(`vmctl ownership missing sandbox_url for owner ${ownerID}`);
  }
  result.vm_health_before = vmHealth(result.vmctl_before.sandbox_url);

  const inFlight = await waitForCoordinationInFlight(page, result.submission.submission_id, decision.doc_id);
  result.pre_refresh = {
    roles: inFlight.roles,
    trace_live: Boolean(inFlight.trace.trajectory?.live),
    agent_count: (inFlight.trace.agents || []).length,
    moment_count: (inFlight.trace.moments || []).length,
    doc_run_states: (inFlight.diagnosis.runs || []).map((run) => ({
      loop_id: run.loop_id || run.run_id || '',
      agent_profile: run.agent_profile || '',
      state: run.state || '',
    })),
    samples: inFlight.samples.slice(-6),
  };

  const refreshAtMs = Date.now();
  result.refresh_request = {
    owner_id: ownerID,
    desktop_id: 'primary',
    before_vm_id: result.vmctl_before.vm_id,
    before_epoch: result.vmctl_before.epoch,
    before_sandbox_url: result.vmctl_before.sandbox_url,
    at: new Date(refreshAtMs).toISOString(),
  };
  result.vmctl_refresh_response = vmctlRefresh(ownerID);
  result.vmctl_after = await waitForOwnershipAdvance(ownerID, result.vmctl_before.epoch);
  result.vm_health_after = await waitForVMHealth(result.vmctl_after.sandbox_url);
  result.health_after = await fetchJSON(page, '/health');
  result.bootstrap_after = await bootstrap(page);
  result.compute_status_after = await fetchJSON(page, '/api/compute/status');

  if (result.vmctl_after.user_id !== result.vmctl_before.user_id) {
    throw new Error(`owner changed across refresh: before=${result.vmctl_before.user_id} after=${result.vmctl_after.user_id}`);
  }
  if (result.vmctl_after.desktop_id !== result.vmctl_before.desktop_id) {
    throw new Error(`desktop changed across refresh: before=${result.vmctl_before.desktop_id} after=${result.vmctl_after.desktop_id}`);
  }
  if (result.vmctl_after.vm_id !== result.vmctl_before.vm_id) {
    throw new Error(`vm_id changed across refresh: before=${result.vmctl_before.vm_id} after=${result.vmctl_after.vm_id}`);
  }
  if (!(Number(result.vmctl_after.epoch || 0) > Number(result.vmctl_before.epoch || 0))) {
    throw new Error(`vmctl epoch did not advance: before=${result.vmctl_before.epoch} after=${result.vmctl_after.epoch}`);
  }

  const recovered = await waitForRecoveredProgress(
    page,
    result.submission.submission_id,
    decision.doc_id,
    refreshAtMs,
  );
  result.post_refresh = {
    consumed_roles: recovered.consumedRoles,
    pending_worker_updates: recovered.pending,
    head_revision_id: recovered.state.head?.revision_id || null,
    head_created_at: recovered.state.head?.created_at || null,
    head_source: recovered.state.head?.metadata?.source || null,
    revision_count: recovered.state.revisions.length,
    post_refresh_roles: uniqueSorted((recovered.trace.agents || []).map(roleName)),
    samples: recovered.samples.slice(-6),
  };

  result.acceptance = await postJSON(page, '/api/run-acceptances/synthesize', {
    target_mission_id: 'mission-lifecycle-cutover-v0',
    source_prompt_or_objective: prompt,
    trajectory_id: result.submission.submission_id,
    staging_url: new URL(BASE_URL).origin,
  }).catch((error) => ({ error: error.message }));
  if (result.acceptance?.acceptance_id) {
    result.acceptance_stored = await fetchJSON(
      page,
      `/api/run-acceptances/${encodeURIComponent(result.acceptance.acceptance_id)}`,
    ).catch((error) => ({ error: error.message }));
  }

  result.finished_at = new Date().toISOString();
  result.status = 'passed';
  console.log(JSON.stringify(result, null, 2));
} catch (error) {
  result.status = 'failed';
  result.error = error?.stack || String(error);
  try {
    result.health_failure = await fetchJSON(page, '/health');
  } catch {}
  try {
    result.compute_status_failure = await fetchJSON(page, '/api/compute/status');
  } catch {}
  console.log(JSON.stringify(result, null, 2));
  if (runtimeProcess) {
    runtimeProcess.exitCode = 1;
  }
} finally {
  await removeVirtualAuthenticator(client, authenticatorId).catch(() => {});
  await context.close().catch(() => {});
  await browser.close().catch(() => {});
}
