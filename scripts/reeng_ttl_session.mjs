#!/usr/bin/env node
// Cut 0 desk-session TTL probe (docs/definitions/choir-reeng-cut0-eyes-2026-10-12.md,
// artifact 6: "one test-computer desk session driven past 70 minutes with cells
// under 30 minutes apart, exported").
//
// Question: the desk broker handle is issued once per worker with a one-hour
// lifetime (internal/yaegikernel/choir.go, verified on every verb in
// handles.go), while a desk session is kept per agent (agent:<agent_id>) as long
// as it gets a cell at least every 30 minutes (deskSessionRetention). Does a
// choir.* verb get refused ("handles: handle has expired") after minute 60 of a
// worker's life on the deployed system?
//
// The script submits one research-heavy story prompt through the product path
// (POST /api/prompt-bar with a computer-scoped key), waits for the first desk
// revision, then every --interval-min minutes writes a follow-up into the SAME
// document (a user revision on the Texture document, which wakes its desk and
// the research desk it commissions), until --minutes have elapsed since the
// first research cell, plus one final follow-up after minute 65. Finally it
// reads the trajectory's runs and run events, summarises desk.cell and
// desk.egress per desk session and worker pid, and writes a trace bundle with
// `choir trace export`.
//
// Usage (the key file is the 0600 one written by scripts/qa_test_computer.mjs;
// the key is only ever read from the environment and never printed):
//   CHOIR_API_KEY=$(jq -r .api_key <file>) node scripts/reeng_ttl_session.mjs \
//     --computer computer-... --minutes 75 --interval-min 20 \
//     --out ~/.config/choir-qa/traces/<label>/ttl-report.json [--label <label>]
//
// Exit 0 = the probe ran to the end and wrote its report (the verdict is in the
// report, not the exit code); 1 = it could not run.

import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const HOST = process.env.CHOIR_HOST || 'https://choir.news';
const API_KEY = process.env.CHOIR_API_KEY || '';
const args = process.argv.slice(2);
const arg = (name, dflt) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : dflt;
};
const expandHome = (p) => (p.startsWith('~/') ? join(homedir(), p.slice(2)) : p);
const COMPUTER = arg('computer', '');
const MINUTES = Number(arg('minutes', '75'));
const INTERVAL_MIN = Number(arg('interval-min', '20'));
const FINAL_FOLLOWUP_MIN = Number(arg('final-followup-min', '66'));
const MAX_GAP_MIN = 25; // submit early if the research desk has had no cell this long
const LABEL = arg('label', `reeng-ttl-${new Date().toISOString().slice(0, 16).replace(/[-:T]/g, '')}`);
const OUT = expandHome(arg('out', join(homedir(), '.config', 'choir-qa', 'traces', LABEL, 'ttl-report.json')));
const TICK_MS = 45_000;

if (!API_KEY || !COMPUTER) {
  console.error('CHOIR_API_KEY and --computer are required');
  process.exit(2);
}
const headers = { Authorization: `Bearer ${API_KEY}`, 'X-Choir-Computer': COMPUTER };
const iso = (ms) => new Date(ms).toISOString();
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const log = (...a) => console.log(`[${new Date().toISOString().slice(11, 19)}]`, ...a);

async function api(path, method = 'GET', body = undefined) {
  for (let attempt = 0; ; attempt++) {
    try {
      const res = await fetch(`${HOST}${path}`, {
        method,
        headers: { ...headers, ...(body ? { 'Content-Type': 'application/json' } : {}) },
        body: body ? JSON.stringify(body) : undefined,
      });
      return { status: res.status, body: await res.json().catch(() => null) };
    } catch (err) {
      if (attempt >= 4) return { status: 0, body: null, error: String(err) };
      await sleep(3000 * (attempt + 1));
    }
  }
}

const result = {
  probe: 'reeng_ttl_session', host: HOST, computer: COMPUTER, label: LABEL,
  params: { minutes: MINUTES, interval_min: INTERVAL_MIN, final_followup_min: FINAL_FOLLOWUP_MIN },
  started_at: iso(Date.now()), trajectories: [], submissions: [], notes: [],
};
const note = (s) => { result.notes.push(s); log('NOTE', s); };

// ---- run and event collection (incremental; the same code is the final read) ----
const runs = new Map(); // run_id -> run record
const events = new Map(); // run_id -> [event]
const cursors = new Map(); // run_id -> last seq read
const TERMINAL = new Set(['completed', 'failed', 'cancelled', 'canceled', 'succeeded', 'done', 'error']);

async function collect() {
  for (const t of result.trajectories) {
    const list = await api(`/api/runs?trajectory_id=${encodeURIComponent(t)}&limit=500`);
    for (const r of list.body?.runs || []) runs.set(r.run_id, r);
  }
  for (const id of runs.keys()) {
    let after = cursors.get(id) || 0;
    for (let pages = 0; pages < 200; pages++) {
      const page = await api(`/api/runs/${encodeURIComponent(id)}/events?after=${after}&limit=1000`);
      if (page.status !== 200 || !page.body) break;
      const evs = page.body.events || [];
      if (!events.has(id)) events.set(id, []);
      events.get(id).push(...evs);
      if (evs.length) after = Math.max(after, evs[evs.length - 1].seq || 0);
      cursors.set(id, after);
      if (!page.body.next_after) break;
      after = page.body.next_after;
    }
  }
}

const allEvents = () => [...events.entries()].flatMap(([run_id, evs]) => evs.map((e) => ({ ...e, run_id: e.loop_id || run_id })));
const parsePayload = (e) => {
  if (e.payload && typeof e.payload === 'object') return e.payload;
  try { return JSON.parse(e.payload || '{}'); } catch { return {}; }
};
const profileOfAgent = () => {
  const m = new Map();
  for (const r of runs.values()) if (r.agent_id) m.set(r.agent_id, r.agent_profile || r.agent_role || '');
  return m;
};
const sessionProfile = (session) => {
  const agent = String(session).startsWith('agent:') ? String(session).slice(6) : '';
  return profileOfAgent().get(agent) || '';
};
function cellEvents() {
  return allEvents().filter((e) => e.kind === 'desk.cell').map((e) => ({ ...parsePayload(e), event_run_id: e.run_id, agent_id: e.agent_id }));
}
function firstResearchCellMs() {
  let first = null;
  for (const c of cellEvents()) {
    if (sessionProfile(c.desk_session) !== 'research' || !c.started_at) continue;
    const ms = Date.parse(c.started_at);
    if (first === null || ms < first) first = ms;
  }
  return first;
}
function lastResearchCellMs() {
  let last = null;
  for (const c of cellEvents()) {
    if (sessionProfile(c.desk_session) !== 'research' || !c.finished_at) continue;
    const ms = Date.parse(c.finished_at);
    if (last === null || ms > last) last = ms;
  }
  return last;
}
const liveRuns = () => [...runs.values()].filter((r) => !TERMINAL.has(String(r.state || '').toLowerCase()));

// ---- submissions ----
const STORY_PROMPT =
  'Write a sourced news story on the most recent AI chip announcement. Research the web thoroughly: ' +
  'search for and read at least four independent web sources (company announcement, trade press, analyst coverage) ' +
  'and cite each with its link. Cover what was announced, the claimed performance and availability, and what analysts say.';

async function submitStory() {
  const t = Date.now();
  const sub = await api('/api/prompt-bar', 'POST', { text: STORY_PROMPT, command_id: `reeng-ttl-${t}-0` });
  if (sub.status >= 300 || !sub.body?.trajectory_id) {
    throw new Error(`prompt-bar submit refused: ${sub.status} ${sub.body?.error || sub.error || ''}`);
  }
  result.trajectories.push(sub.body.trajectory_id);
  result.primary = { trajectory_id: sub.body.trajectory_id, doc_id: sub.body.doc_id, owner_revision: sub.body.revision_id };
  result.submissions.push({ kind: 'prompt-bar', at: iso(t), trajectory_id: sub.body.trajectory_id, doc_id: sub.body.doc_id });
  log(`story submitted: trajectory=${sub.body.trajectory_id} doc=${sub.body.doc_id}`);
}

const FACTS = [
  'the announced memory bandwidth or HBM configuration',
  'the announced process node and foundry',
  'the announced power draw or rack-level cooling requirement',
  'the announced list price or named launch customers',
  'the announced software stack or framework support',
  'a competing chip announced in the same month and how analysts compare them',
];
let followupN = 0;

async function submitFollowup(reason) {
  followupN++;
  const t = Date.now();
  const traj = result.primary.trajectory_id;
  const request = `Follow-up ${followupN}: search the web for one new, sourced fact not yet in this story about ${FACTS[(followupN - 1) % FACTS.length]}, ` +
    'read the source, add it to the story, and cite its link.';
  const snap = await api(`/api/trajectories/${encodeURIComponent(traj)}`);
  const head = snap.body?.head_revision || snap.body?.current_document_head;
  const status = snap.body?.trajectory?.status;
  const version = snap.body?.trajectory?.lifecycle_version;
  const entry = { kind: 'followup', n: followupN, at: iso(t), reason, trajectory_status: status || null, lifecycle_version: version || null };
  if (!head?.revision_id || !version) {
    entry.error = `no lifecycle head/version in snapshot (HTTP ${snap.status})`;
  } else {
    const content = `${head.content || ''}\n\n${request}\n`;
    const rev = await api(`/api/texture/documents/${encodeURIComponent(result.primary.doc_id)}/revisions`, 'POST', {
      content, parent_revision_id: head.revision_id,
      idempotency_key: `reeng-ttl-${t}-${followupN}`, expected_lifecycle_version: version,
    });
    entry.http = rev.status;
    entry.revision_id = rev.body?.revision_id || null;
    if (rev.status >= 300) entry.error = rev.body?.error || rev.error || 'revision refused';
  }
  if (status && status !== 'live') note(`follow-up ${followupN}: trajectory status is ${status}, so a user revision does not wake the desk`);
  result.submissions.push(entry);
  log(`follow-up ${followupN} (${reason}): http=${entry.http ?? '-'} status=${status || '-'} ${entry.error || 'ok'}`);
  return entry;
}

// ---- analysis ----
const ageMin = (ms, base) => Math.round(((ms - base) / 60000) * 100) / 100;
function analyse() {
  const cells = cellEvents();
  // A cell's egress calls ride its desk.cell event since journal cut R4; the
  // standalone desk.egress events are older tapes, refusals and search_done.
  const egress = allEvents().flatMap((e) => {
    if (e.kind === 'desk.egress') return [{ ...parsePayload(e), event_run_id: e.run_id }];
    const rows = e.kind === 'desk.cell' ? parsePayload(e).egress : null;
    return Array.isArray(rows) ? rows.map((row) => ({ ...row, event_run_id: e.run_id })) : [];
  });
  const sessions = new Map();
  const sess = (key) => {
    if (!sessions.has(key)) sessions.set(key, { desk_session: key, profile: sessionProfile(key), workers: new Map() });
    return sessions.get(key);
  };
  const worker = (key, pid) => {
    const s = sess(key);
    if (!s.workers.has(pid)) s.workers.set(pid, { worker_pid: pid, cells: 0, first_ms: null, last_ms: null, egress: 0, egress_errors: 0, last_ok_egress_ms: null, cell_errors: [] });
    return s.workers.get(pid);
  };
  const touch = (w, startMs, endMs) => {
    if (startMs && (w.first_ms === null || startMs < w.first_ms)) w.first_ms = startMs;
    const last = endMs || startMs;
    if (last && (w.last_ms === null || last > w.last_ms)) w.last_ms = last;
  };
  for (const c of cells) {
    const w = worker(c.desk_session, c.worker_pid);
    w.cells++;
    touch(w, Date.parse(c.started_at), Date.parse(c.finished_at));
    if (c.error) w.cell_errors.push({ started_at: c.started_at, error: String(c.error).slice(0, 400) });
  }
  for (const g of egress) {
    const w = worker(g.desk_session, g.worker_pid);
    w.egress++;
    touch(w, Date.parse(g.started_at), Date.parse(g.finished_at));
    if (g.is_error) w.egress_errors++;
    else if (w.last_ok_egress_ms === null || Date.parse(g.finished_at) > w.last_ok_egress_ms) w.last_ok_egress_ms = Date.parse(g.finished_at);
  }
  const egressErrors = egress.filter((g) => g.is_error).map((g) => {
    const w = worker(g.desk_session, g.worker_pid);
    return {
      desk_session: g.desk_session, worker_pid: g.worker_pid, verb: g.verb, started_at: g.started_at,
      age_min: ageMin(Date.parse(g.started_at), w.first_ms), error: String(g.error || '').slice(0, 400),
    };
  }).sort((a, b) => a.started_at.localeCompare(b.started_at));

  const sessionRows = [...sessions.values()].map((s) => {
    const ws = [...s.workers.values()].sort((a, b) => a.first_ms - b.first_ms);
    const first = Math.min(...ws.map((w) => w.first_ms));
    const last = Math.max(...ws.map((w) => w.last_ms));
    return {
      desk_session: s.desk_session, profile: s.profile,
      session_span_min: ageMin(last, first), first_activity: iso(first), last_activity: iso(last),
      distinct_worker_pids: ws.length,
      workers: ws.map((w) => ({
        worker_pid: w.worker_pid, cells: w.cells, egress: w.egress, egress_errors: w.egress_errors,
        first_activity: iso(w.first_ms), last_activity: iso(w.last_ms), worker_span_min: ageMin(w.last_ms, w.first_ms),
        last_ok_egress_age_min: w.last_ok_egress_ms ? ageMin(w.last_ok_egress_ms, w.first_ms) : null,
        cell_errors: w.cell_errors.slice(0, 5),
      })),
    };
  });

  // Catch-all: does the phrase appear anywhere in any run event, run result or run error?
  const haystacks = [];
  for (const e of allEvents()) haystacks.push({ where: `event ${e.kind} run=${e.run_id} seq=${e.seq}`, text: JSON.stringify(e.payload ?? '') });
  for (const r of runs.values()) haystacks.push({ where: `run ${r.run_id} (${r.agent_profile || ''}) ${r.state}`, text: `${r.error || ''}\n${r.result || ''}` });
  const expiredHits = haystacks.filter((h) => /expired/i.test(h.text) && /handle/i.test(h.text)).map((h) => h.where);

  // Research desk: did one worker pid cover the whole span?
  const research = sessionRows.filter((s) => s.profile === 'research');
  const researchCells = cells.filter((c) => sessionProfile(c.desk_session) === 'research').sort((a, b) => Date.parse(a.started_at) - Date.parse(b.started_at));
  let maxGapMin = 0;
  for (let i = 1; i < researchCells.length; i++) {
    maxGapMin = Math.max(maxGapMin, ageMin(Date.parse(researchCells[i].started_at), Date.parse(researchCells[i - 1].finished_at)));
  }
  const researchOnePid = research.map((s) => ({ desk_session: s.desk_session, one_pid_covered_whole_span: s.distinct_worker_pids === 1, span_min: s.session_span_min, pids: s.workers.map((w) => w.worker_pid) }));

  const expiredErrors = egressErrors.filter((g) => /expired/i.test(g.error));
  const maxOkAge = Math.max(0, ...sessionRows.flatMap((s) => s.workers.map((w) => w.last_ok_egress_age_min || 0)));
  const maxWorkerSpan = Math.max(0, ...sessionRows.flatMap((s) => s.workers.map((w) => w.worker_span_min)));
  let verdict;
  if (expiredErrors.length) {
    verdict = `handle expiry observed at minute ${Math.min(...expiredErrors.map((g) => g.age_min))}`;
  } else if (maxOkAge > 60) {
    verdict = `no expiry: one worker held verbs for ${maxOkAge} minutes`;
  } else if (expiredHits.length) {
    verdict = `handle expiry text found outside desk.egress (${expiredHits.length} hits); see expired_text_hits`;
  } else {
    verdict = `inconclusive: no successful egress older than 60 minutes on any worker (longest worker span ${maxWorkerSpan} min, longest ok egress age ${maxOkAge} min, ${research.length} research session(s))`;
  }
  return {
    cell_count: cells.length, egress_count: egress.length, run_count: runs.size,
    sessions: sessionRows, egress_errors: egressErrors,
    any_error_contains_expired: expiredErrors.length > 0 || expiredHits.length > 0,
    expired_text_hits: expiredHits.slice(0, 20),
    research: { sessions: researchOnePid, cells: researchCells.length, max_gap_between_cells_min: maxGapMin, gaps_under_30_min: maxGapMin < 30 },
    verdict,
  };
}

// ---- main ----
async function main() {
  const obs = await api('/api/runtime/observability');
  result.guest_build = obs.body?.build || null;
  await submitStory();

  // Wait for the first desk revision (the Texture desk's first authored version).
  const revDeadline = Date.now() + 30 * 60_000;
  let firstDeskRevision = null;
  while (Date.now() < revDeadline && !firstDeskRevision) {
    const revs = (await api(`/api/texture/documents/${result.primary.doc_id}/revisions`)).body?.revisions || [];
    const desk = revs.filter((r) => r.revision_id !== result.primary.owner_revision && r.author_kind !== 'user');
    if (desk.length) firstDeskRevision = desk[desk.length - 1];
    else await sleep(10_000);
  }
  if (!firstDeskRevision) note('no desk revision within 30 minutes; continuing with the schedule anyway');
  else log(`first desk revision ${firstDeskRevision.revision_id}`);
  result.first_desk_revision_at = firstDeskRevision ? firstDeskRevision.created_at : null;

  // Anchor the schedule on the first research cell (fall back to the first desk revision).
  const anchorDeadline = Date.now() + 25 * 60_000;
  let t0 = null;
  while (Date.now() < anchorDeadline && t0 === null) {
    await collect();
    t0 = firstResearchCellMs();
    if (t0 === null) await sleep(TICK_MS);
  }
  if (t0 === null) {
    t0 = firstDeskRevision ? Date.parse(firstDeskRevision.created_at) : Date.now();
    note(`no research desk.cell seen within 25 minutes of the first revision; schedule anchored on ${iso(t0)} instead`);
  }
  result.research_t0 = iso(t0);
  log(`schedule anchor (first research cell) ${iso(t0)}`);

  const endMs = t0 + MINUTES * 60_000;
  const plan = [];
  for (let k = 1; t0 + k * INTERVAL_MIN * 60_000 < endMs; k++) plan.push(t0 + k * INTERVAL_MIN * 60_000);
  const finalMs = t0 + FINAL_FOLLOWUP_MIN * 60_000;
  if (finalMs < endMs && !plan.some((p) => Math.abs(p - finalMs) < 3 * 60_000 && p > t0 + 65 * 60_000)) plan.push(finalMs);
  plan.sort((a, b) => a - b);
  result.followup_plan = plan.map((p) => ({ at: iso(p), minute: ageMin(p, t0) }));
  log('follow-up plan (minutes after anchor):', result.followup_plan.map((p) => p.minute).join(', '));

  let lastSubmitMs = Date.now();
  while (Date.now() < endMs) {
    await sleep(TICK_MS);
    await collect();
    const now = Date.now();
    if (plan.length && now >= plan[0]) {
      plan.shift();
      await submitFollowup('scheduled');
      lastSubmitMs = now;
      continue;
    }
    // Keep cells under 30 minutes apart: if the research desk has been quiet, nudge it early.
    const lastCell = lastResearchCellMs();
    if (lastCell !== null && now - lastCell > MAX_GAP_MIN * 60_000 && now - lastSubmitMs > 8 * 60_000) {
      await submitFollowup(`research quiet for ${Math.round((now - lastCell) / 60000)} min`);
      lastSubmitMs = now;
    }
  }

  // Let in-flight runs settle (bounded), then do the final read.
  const settleDeadline = Date.now() + 12 * 60_000;
  while (Date.now() < settleDeadline) {
    await collect();
    if (liveRuns().length === 0) break;
    await sleep(TICK_MS);
  }
  await collect();
  result.live_runs_at_end = liveRuns().map((r) => ({ run_id: r.run_id, profile: r.agent_profile, state: r.state }));
  result.analysis = analyse();
  result.runs = [...runs.values()].map((r) => ({ run_id: r.run_id, agent_id: r.agent_id, profile: r.agent_profile, state: r.state, created_at: r.created_at, error: r.error ? String(r.error).slice(0, 300) : null }));

  // Trace bundle for the primary trajectory.
  const bundleDir = join(homedir(), '.config', 'choir-qa', 'traces', LABEL);
  mkdirSync(bundleDir, { recursive: true });
  const bundle = join(bundleDir, 'bundle.json');
  const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..');
  try {
    const out = execFileSync('go', ['run', './cmd/choir', 'trace', 'export', result.primary.trajectory_id, '--out', bundle], {
      cwd: repo, encoding: 'utf8', timeout: 600_000, maxBuffer: 1 << 24,
      env: { ...process.env, CHOIR_HOST: HOST, CHOIR_API_KEY: API_KEY, CHOIR_COMPUTER: COMPUTER },
    });
    result.trace_export = { path: bundle, stdout: out.trim().slice(-2000), sha256: (out.match(/\b[0-9a-f]{64}\b/) || [null])[0] };
  } catch (err) {
    result.trace_export = { path: bundle, error: String(err.stderr || err.message).slice(0, 2000), stdout: String(err.stdout || '').slice(-1000) };
  }
  finish();
}

function finish() {
  result.finished_at = iso(Date.now());
  mkdirSync(dirname(OUT), { recursive: true });
  writeFileSync(OUT, `${JSON.stringify(result, null, 2)}\n`, { mode: 0o600 });
  const a = result.analysis;
  if (a) {
    console.log('\nPER DESK SESSION');
    for (const s of a.sessions) {
      console.log(`${s.desk_session} [${s.profile || '?'}] span=${s.session_span_min}m pids=${s.distinct_worker_pids}`);
      for (const w of s.workers) console.log(`   pid ${w.worker_pid}: ${w.first_activity} .. ${w.last_activity} (${w.worker_span_min}m) cells=${w.cells} egress=${w.egress} errors=${w.egress_errors} last_ok_egress_age=${w.last_ok_egress_age_min}m`);
    }
    console.log(`\nEGRESS ERRORS (${a.egress_errors.length})`);
    for (const g of a.egress_errors) console.log(`  ${g.started_at} ${g.desk_session} pid=${g.worker_pid} ${g.verb} age=${g.age_min}m: ${g.error}`);
    console.log(`\nany error contains "expired": ${a.any_error_contains_expired}`);
    console.log(`research: ${JSON.stringify(a.research)}`);
    console.log(`\nVERDICT: ${a.verdict}`);
  }
  console.log(`trace export: ${JSON.stringify(result.trace_export || null)}`);
  console.log(`report -> ${OUT}`);
  process.exit(result.failure ? 1 : 0);
}

main().catch((err) => {
  result.failure = String(err && err.stack ? err.stack : err);
  console.error(result.failure);
  finish();
});
