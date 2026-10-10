import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { test, expect } from './helpers/fixtures.js';

// Texture acceptance suite (Gate 1 exit item): docs/texture-acceptance-suite.md.
// Each check is a failure the owner reported this week or the crash rule
// (AGENTS.md "Restarts End Work (Crash) Or Resume It"). Writes a receipt to
// docs/evidence/texture-acceptance-<stamp>.json.

const here = path.dirname(fileURLToPath(import.meta.url));
const stamp = new Date().toISOString().replace(/[:.]/g, '-');
const receiptPath = path.resolve(here, '../../docs/evidence', `texture-acceptance-${stamp}.json`);
const receipt = { started_at: new Date().toISOString(), checks: {} };
function record(id, data) {
  receipt.checks[id] = { at: new Date().toISOString(), ...data };
  mkdirSync(path.dirname(receiptPath), { recursive: true });
  writeFileSync(receiptPath, JSON.stringify(receipt, null, 2) + '\n');
}

// Access tokens live 5 minutes. Like the editor, a 401 renews the session
// through GET /auth/session (refresh rotation) and retries once.
async function api(page, method, apiPath, body) {
  return page.evaluate(async ({ method, apiPath, body }) => {
    const send = () => fetch(apiPath, {
      method,
      credentials: 'include',
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    });
    const started = performance.now();
    let res = await send();
    let renewed = false;
    if (res.status === 401) {
      await fetch('/auth/session', { credentials: 'include' });
      renewed = true;
      res = await send();
    }
    const json = await res.json().catch(() => null);
    return { status: res.status, body: json, ms: Math.round(performance.now() - started), renewed };
  }, { method, apiPath, body });
}

async function waitFor(fn, timeoutMs, intervalMs = 5000) {
  const deadline = Date.now() + timeoutMs;
  let last = null;
  while (Date.now() < deadline) {
    last = await fn();
    if (last?.done) return last;
    await new Promise((r) => setTimeout(r, intervalMs));
  }
  return last;
}

async function appagentRevisions(page, docID) {
  const res = await api(page, 'GET', `/api/texture/documents/${docID}/revisions`);
  const list = Array.isArray(res.body?.revisions) ? res.body.revisions : [];
  return list.filter((r) => r.author_kind === 'appagent');
}

async function revise(page, docID, prompt, suffix) {
  const res = await api(page, 'POST', `/api/texture/documents/${docID}/revise`, {
    prompt, intent: 'revise', client_request_id: `acceptance-${suffix}`,
  });
  expect(res.status, JSON.stringify(res.body)).toBe(202);
  return res.body?.revision_id;
}

// agent_revision_pending is omitempty: "not pending" is an absent field.
async function waitForTurn(page, docID, priorAppagent, timeoutMs) {
  return waitFor(async () => {
    const doc = await api(page, 'GET', `/api/texture/documents/${docID}`);
    const agent = await appagentRevisions(page, docID);
    return { done: agent.length > priorAppagent && doc.status === 200 && doc.body?.agent_revision_pending !== true, doc: doc.body, agent };
  }, timeoutMs);
}

async function cancelTurn(page, docID) {
  const doc = await api(page, 'GET', `/api/texture/documents/${docID}`);
  const trajectoryID = doc.body?.trajectory_id;
  const summary = await api(page, 'GET', `/api/trajectories/${encodeURIComponent(trajectoryID)}?view=summary`);
  const pending = summary.body?.pending_cancellation;
  const version = pending ? pending.expected_lifecycle_version : summary.body?.trajectory?.lifecycle_version;
  const head = pending ? pending.expected_head_revision_id : summary.body?.head_revision_id;
  return api(page, 'POST', `/api/trajectories/${encodeURIComponent(trajectoryID)}/cancel`, {
    idempotency_key: pending?.idempotency_key || `acceptance-cancel:${trajectoryID}:${version}:${head}`,
    expected_lifecycle_version: version,
    expected_head_revision_id: head,
    reason: 'acceptance cancel',
  });
}

// Long enough that slow turns plus T6's waits never starve T7 and T8 (run 9).
test.describe.configure({ mode: 'serial', timeout: 1_500_000 });

test('Texture acceptance suite on staging', async ({ desktopSession }) => {
  const { page, baseURL } = desktopSession;
  receipt.base_url = baseURL;
  const suffix = `${Date.now()}`;

  // T1 — document list.
  const list1 = await api(page, 'GET', '/api/texture/documents');
  const list2 = await api(page, 'GET', '/api/texture/documents');
  record('T1_list', { status: [list1.status, list2.status], ms: [list1.ms, list2.ms], count: list2.body?.documents?.length });
  expect(list1.status).toBe(200);
  expect(Math.max(list1.ms, list2.ms), 'document list read time (ms)').toBeLessThan(1500);

  // T2 — revise.
  const created = await api(page, 'POST', '/api/texture/lifecycle-documents', {
    title: `Acceptance ${suffix}`,
    initial_content: `Acceptance seed ${suffix}: persistent computers keep working while you sleep.`,
    client_request_id: `acceptance-create-${suffix}`,
  });
  expect(created.status, JSON.stringify(created.body)).toBe(201);
  let docID = created.body?.doc_id;
  receipt.doc_id = docID;
  // T2a — the create wake produces the first draft with no further owner
  // action (soft: a failure is recorded and the suite continues).
  const startedCreate = Date.now();
  const firstTurn = await waitForTurn(page, docID, 0, 300_000);
  const t2a = { pass: !!firstTurn?.done, seconds: Math.round((Date.now() - startedCreate) / 1000), appagent_revisions: firstTurn?.agent?.length || 0 };
  if (!t2a.pass) {
    // Diagnosis for texture-create-occurrence-deferred-never-refires: what
    // the computer says it owes while the first draft is missing.
    t2a.document = firstTurn?.doc;
    t2a.obligations = (await api(page, 'GET', '/api/runtime/obligations')).body;
  }
  record('T2a_first_draft', t2a);
  expect.soft(firstTurn?.done, 'first draft landed after create without another owner action').toBeTruthy();

  // T2b — the owner revises.
  const priorAgent = firstTurn?.agent?.length || 0;
  const startedRevise = Date.now();
  await revise(page, docID, 'Rewrite this as two short paragraphs about persistent computers.', `t2-${suffix}`);
  const turn = await waitForTurn(page, docID, priorAgent, 300_000);
  record('T2b_revise', { pass: !!turn?.done, seconds: Math.round((Date.now() - startedRevise) / 1000), appagent_revisions: turn?.agent?.length });
  expect(turn?.done, 'Texture revision landed and the document is idle').toBeTruthy();

  // T3 — version chevrons.
  const revisions = await api(page, 'GET', `/api/texture/documents/${docID}/revisions`);
  const ids = (revisions.body?.revisions || []).slice(0, 4).map((r) => r.revision_id);
  const reads = [];
  for (const id of ids) {
    const r = await api(page, 'GET', `/api/texture/revisions/${id}`);
    reads.push({ id, status: r.status, ms: r.ms });
  }
  record('T3_chevrons', { reads });
  expect(reads.length).toBeGreaterThan(1);
  for (const r of reads) {
    expect(r.status).toBe(200);
    expect(r.ms, `revision ${r.id} read time (ms)`).toBeLessThan(1000);
  }

  // T4 — no "Revising…" zombie after reload.
  await page.reload();
  const idle = [];
  for (let i = 0; i < 3; i++) {
    const doc = await api(page, 'GET', `/api/texture/documents/${docID}`);
    idle.push(doc.body?.agent_revision_pending === true);
    await page.waitForTimeout(5000);
  }
  record('T4_reload_idle', { pending_reads: idle });
  expect(idle.some(Boolean), 'document reads pending after a finished turn').toBeFalsy();

  // T5 — cancel during a turn.
  const beforeCancel = (await appagentRevisions(page, docID)).length;
  await revise(page, docID, 'Expand this into a long essay with many sections.', `t5-${suffix}`);
  const pendingSeen = await waitFor(async () => {
    const doc = await api(page, 'GET', `/api/texture/documents/${docID}`);
    return { done: doc.body?.agent_revision_pending === true };
  }, 60_000, 1000);
  const cancelAt = Date.now();
  const cancelled = await cancelTurn(page, docID);
  const cleared = await waitFor(async () => {
    const doc = await api(page, 'GET', `/api/texture/documents/${docID}`);
    return { done: doc.status === 200 && doc.body?.agent_revision_pending !== true };
  }, 20_000, 1000);
  record('T5_cancel', {
    pending_seen: !!pendingSeen?.done, cancel_status: cancelled.status,
    cleared: !!cleared?.done, seconds_to_clear: Math.round((Date.now() - cancelAt) / 1000),
    appagent_before: beforeCancel,
  });
  expect(cancelled.status, JSON.stringify(cancelled.body)).toBeLessThan(300);
  expect(cleared?.done, 'pending cleared within 20 s of cancel').toBeTruthy();

  // T5b — the owner revises again after a cancel (soft: the editor's Cancel
  // cancels the document trajectory; this records whether the document
  // stays revisable).
  const afterCancel = await api(page, 'POST', `/api/texture/documents/${docID}/revise`, {
    prompt: 'Shorten this to one paragraph.', intent: 'revise', client_request_id: `acceptance-t5b-${suffix}`,
  });
  record('T5b_revise_after_cancel', { pass: afterCancel.status === 202, status: afterCancel.status, body: afterCancel.body });
  expect.soft(afterCancel.status, `revise after cancel: ${JSON.stringify(afterCancel.body)}`).toBe(202);

  // T6 and T7 run on a fresh document so a terminal trajectory from the
  // cancel cannot mask them.
  const created2 = await api(page, 'POST', '/api/texture/lifecycle-documents', {
    title: `Acceptance research ${suffix}`,
    initial_content: `Acceptance seed ${suffix}: persistent personal computers and their owners.`,
    client_request_id: `acceptance-create2-${suffix}`,
  });
  expect(created2.status, JSON.stringify(created2.body)).toBe(201);
  docID = created2.body?.doc_id;
  receipt.doc_id_research = docID;
  await waitForTurn(page, docID, 0, 300_000);

  // T6 — research.
  const researchPrior = await appagentRevisions(page, docID);
  const beforeResearch = researchPrior.length;
  // The revision list is newest first; compare by id, not by position.
  const priorIDs = new Set(researchPrior.map((r) => r.revision_id));
  await revise(page, docID, 'Research current reporting on persistent personal computers and add two cited web sources.', `t6-${suffix}`);
  // Two properties, recorded apart (problems/texture-research-sources-never-
  // reach-the-citing-turn-2026-10-09.md, run 8): a cited research revision
  // lands (T6), and the document idles after it (T6_idle).
  const cited = await waitFor(async () => {
    const agent = await appagentRevisions(page, docID);
    for (const r of agent.filter((x) => !priorIDs.has(x.revision_id))) {
      const rev = await api(page, 'GET', `/api/texture/revisions/${r.revision_id}`);
      const sources = Array.isArray(rev.body?.source_entities) ? rev.body.source_entities.length : 0;
      const refs = Array.isArray(rev.body?.source_refs) ? rev.body.source_refs.length : 0;
      if (sources > 0 && refs > 0) return { done: true, revision: r.revision_id, sources, refs };
    }
    return { done: false };
  }, 360_000);
  record('T6_research', { landed: !!cited?.done, revision: cited?.revision, source_entities: cited?.sources ?? 0, source_refs: cited?.refs ?? 0 });
  // Soft: a research failure must not hide the crash legs (T7, T8).
  expect.soft(cited?.done, 'cited research revision landed').toBeTruthy();
  const researched = await waitForTurn(page, docID, beforeResearch, cited?.done ? 180_000 : 1_000);
  record('T6_idle', { idle: !!researched?.done, pending: researched?.doc?.agent_revision_pending === true });
  expect.soft(researched?.done, 'document idles after the research revision').toBeTruthy();

  // T7 — crash mid-turn: interrupted, never resumed.
  const session = await page.evaluate(async () => (await fetch('/auth/session', { credentials: 'include' })).json());
  const userID = session?.user?.id || session?.user_id;
  expect(userID, JSON.stringify(session)).toBeTruthy();
  await revise(page, docID, 'Write a long detailed history of personal computing, decade by decade.', `t7-${suffix}`);
  await waitFor(async () => {
    const doc = await api(page, 'GET', `/api/texture/documents/${docID}`);
    return { done: doc.body?.agent_revision_pending === true };
  }, 60_000, 1000);
  const beforeCrash = (await appagentRevisions(page, docID)).length;
  const payload = JSON.stringify({ user_id: userID, desktop_id: 'primary' });
  const refresh = execFileSync('ssh', ['-o', 'BatchMode=yes', 'node-b',
    `curl -sS -m 300 -X POST -H "X-Internal-Caller: true" -H "Content-Type: application/json" -d '${payload}' http://127.0.0.1:8083/internal/vmctl/refresh`],
  { encoding: 'utf8' }).trim();
  const back = await waitFor(async () => {
    const doc = await api(page, 'GET', `/api/texture/documents/${docID}`);
    return { done: doc.status === 200, doc: doc.body };
  }, 300_000, 5000);
  await page.waitForTimeout(120_000);
  const after = await api(page, 'GET', `/api/texture/documents/${docID}`);
  const afterAgent = (await appagentRevisions(page, docID)).length;
  record('T7_crash_interrupts', {
    refresh: refresh.slice(0, 300), back: !!back?.done,
    interrupted: after.body?.agent_revision_interrupted === true,
    pending: after.body?.agent_revision_pending === true,
    appagent_before: beforeCrash, appagent_after: afterAgent,
  });
  expect(after.body?.agent_revision_pending, 'pending after crash').not.toBe(true);
  expect(after.body?.agent_revision_interrupted, 'interrupted flag after crash').toBe(true);
  expect(afterAgent, 'a new appagent revision appeared after the crash (resumed)').toBe(beforeCrash);

  // T8 — what is owed after the crash (SL fault-matrix leg d, restart mid-turn).
  const owed = await waitFor(async () => {
    const res = await api(page, 'GET', '/api/runtime/obligations');
    const b = res.body || {};
    const settled = res.status === 200 && (b.wakes?.unprojected ?? 1) === 0 && !(b.runs?.running > 0);
    return { done: settled, res };
  }, 120_000, 5000);
  const ob = owed?.res?.body || {};
  record('T8_obligations_after_crash', {
    status: owed?.res?.status, ms: owed?.res?.ms, restart: ob.restart,
    wakes: ob.wakes, runs: ob.runs, open_work_items: ob.open_work_items, errors: ob.errors,
  });
  expect(owed?.res?.status).toBe(200);
  expect(ob.restart?.kind, 'boot after an unplanned refresh').toBe('crash_or_stop');
  expect(ob.errors || [], 'obligations surface read errors').toEqual([]);
  expect(ob.wakes?.unprojected, 'wakes still owed 2 min after the crash').toBe(0);
  expect(ob.wakes?.exhausted, 'wakes that exhausted their dispatch budget').toBe(0);
  expect(ob.runs?.running || 0, 'runs still running after the crash').toBe(0);

  const failedSoft = Object.entries(receipt.checks).filter(([, v]) => v.pass === false).map(([k]) => k);
  receipt.result = failedSoft.length ? 'fail' : 'pass';
  receipt.finished_at = new Date().toISOString();
  record('summary', { result: receipt.result, failed: failedSoft });
});
