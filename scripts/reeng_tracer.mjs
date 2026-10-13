#!/usr/bin/env node
// Cut 0 tracer (docs/definitions/choir-reeng-cut0-eyes-2026-10-12.md, artifact 2;
// bundle contract docs/reports/reeng-trace-bundle-format-2026-10-12.md).
//
// One attempt:
//   1. mint a fresh staging computer (scripts/qa_test_computer.mjs) and act
//      only on that computer id;
//   2. submit a fixed AI-news story through the prompt bar route; the story
//      asks Texture to cast research twice in one turn, each cast running
//      eight tagged searches started at once in one cell (choir.SearchStart);
//   3. while a run of the story is mid-cell, submit a follow-up to the same
//      document (the second run on a busy desk);
//   4. crash drill: only after observing an in-flight run, restart the
//      computer through the owner route, record when it answers again and the
//      obligations before and after;
//   5. continue the story (an owner follow-up if the restart closed the work)
//      and record whether anything publishes without an owner act;
//   6. export the trajectory (`choir trace export`), insert the drill block,
//      keep the bundle under ~/.config/choir-qa/traces/<label>/ (0600) and
//      check it for the load (overlapping egress in one cell; a cell that
//      waited for another run is reported, not required, since runs of one
//      desk agent are already serialized). If the load never appeared,
//      retry the whole
//      flow on a fresh computer, up to --attempts.
//
// No SSH and no /internal/* anywhere (standing question 9). The scoped key is
// read from the 0600 credential file the mint wrote and is never printed.
// Per-phase lines go to stderr; the non-secret receipt JSON goes to stdout and
// to ~/.config/choir-qa/traces/<label>/receipt.json.
//
//   node scripts/reeng_tracer.mjs --label c0-tracer-dev-1 [--attempts 1]
//     [--timeout-min 15] [--settle-min 3] [--quiesce-min 2]
//     [--exporter "go run ./cmd/choir trace export"]
//     [--skip-export] [--owner-publish] [--keep-computer]
//
// Defaults are the fast loop (owner pace ruling 2026-10-13, metamission
// boundaries.pace): one attempt, short bounded waits. A run that needs
// longer is a finding about the desks, not a reason to wait.
//
// Exit: 0 bundle kept with the load present; 3 bundle(s) kept, load absent
// (I5 unknown); 4 export unavailable (drill kept); 5 contaminated (the guest
// build changed during the run; --expect-sha); 6 the scenario's claim failed
// (--scenario crash); 1 attempt failed; 2 usage.

import { execFile, spawn } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { chmodSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const REPO = join(dirname(fileURLToPath(import.meta.url)), '..');
const HOST = (process.env.CHOIR_DEPLOYED_BASE_URL || process.env.CHOIR_HOST || 'https://choir.news').replace(/\/$/, '');
const args = process.argv.slice(2);
const arg = (name, dflt) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 && i + 1 < args.length ? args[i + 1] : dflt;
};
const flag = (name) => args.includes(`--${name}`);
const LABEL = arg('label', '');
const ATTEMPTS = Math.max(1, Number(arg('attempts', '1')));
const TIMEOUT_MIN = Number(arg('timeout-min', '15'));
const SETTLE_MIN = Number(arg('settle-min', '3'));
const EXPORTER = arg('exporter', 'go run ./cmd/choir trace export');
const SKIP_EXPORT = flag('skip-export');
const OWNER_PUBLISH = flag('owner-publish');
const QUIESCE_MIN = Number(arg('quiesce-min', '2'));
const KEEP_COMPUTER = flag('keep-computer');
// Identity fence (agentic-consensus 2026-10-13): with --expect-sha the tracer
// waits (at most --fence-min) until staging reports that commit and no CI run
// on main is queued or deploying, and marks the run contaminated (exit 5) if
// the guest build changes during it.
const EXPECT_SHA = arg('expect-sha', '').toLowerCase();
const FENCE_MIN = Number(arg('fence-min', '30'));
// Scenarios (agentic-consensus 2026-10-13, convergent C1): "story" is the
// full flow (load, crash, owner re-entry, publication) for cut boundaries;
// "crash" proves restart closure only: crash at the first run in flight,
// then a quiet window with no owner act, then export. Its acceptance is the
// claim, not the I5 load.
const SCENARIO = arg('scenario', 'story');
if (!['story', 'crash', 'management', 'research'].includes(SCENARIO)) {
  console.error(`unknown --scenario ${SCENARIO} (story | crash | management | research)`);
  process.exit(2);
}
const CRASH_QUIET_MS = Number(arg('quiet-s', '150')) * 1000;
const OPEN_RUN_STATES = new Set(['pending', 'running', 'passivated', 'blocked']);
if (!LABEL || !/^[A-Za-z0-9._-]+$/.test(LABEL)) {
  console.error('usage: node scripts/reeng_tracer.mjs --label <label> [--attempts 1] [--timeout-min 15] [--settle-min 3] [--quiesce-min 2] [--exporter <cmd>] [--skip-export] [--owner-publish] [--keep-computer]');
  process.exit(2);
}
const TRACE_DIR = join(homedir(), '.config', 'choir-qa', 'traces', LABEL);

// The fixed story script. What the desks write is not under the tracer's
// control; the bundle check decides whether the load appeared.
const STORY = [
  'Write a short news story (about 300 words, every claim cited) on the most significant AI model release of the past two weeks.',
  'Research instructions, pass them to research verbatim. Cast research TWICE in this same turn, as two separate casts:',
  '(A) In ONE desk_go_eval cell, start eight searches with choir.SearchStart, one per query, tagged q1..q8:',
  'q1 "new AI model release October 2026", q2 "open-weight language model release this week", q3 "frontier AI lab model announcement",',
  'q4 "new AI model benchmark results", q5 "AI model release safety evaluation", q6 "AI model API pricing announcement",',
  'q7 "AI accelerator chip news this week", q8 "AI regulation news this week".',
  'Read each search with choir.Searches.Get as its providers answer, and report which result came back for which tag',
  '(tag, query, first result title and URL), flagging any result that does not match its query.',
  '(B) The same cell shape (eight searches started at once) with tags r1..r8 on reactions to and criticism of that release:',
  'r1 "AI model release criticism", r2 "AI model release independent evaluation", r3 "AI model release developer reaction",',
  'r4 "AI model release hallucination tests", r5 "AI model release open source community", r6 "AI model release enterprise adoption",',
  'r7 "AI model release compute cost", r8 "AI model release policy response". Report the tag-to-result mapping the same way.',
].join(' ');
// The management scenario (Cut 1 L3b): a request only management can take
// up. Its claim is the conversation on records: Texture's Cast reaches
// management:<owner>, management answers with a final Report that closes the
// round, and a committed Texture turn reads it (Texture's cursor reaches the
// Report; whether that turn revises or asks for more is the model's call).
const MANAGEMENT_STORY = [
  'Produce a CSV file with the populations of the five largest cities in Europe and attach it to this document.',
  'This needs execution: Cast the request to management in your first cell, write a short plan revision with it,',
  'and incorporate management\'s answer when it arrives.',
].join(' ');

// The research scenario (Cut 2 2a, with Cut 1 B5 and B6): research starts
// several searches at once and reads providers as they answer; its claim is
// the async search (a search_done egress event with answered providers, and
// a cell result showing new arrivals) and the conversation (research's final
// Report reaches texture:<doc> and a committed Texture turn reads it). The
// first revision completes the owner's request, so the Report arrives on a
// Texture whose work item has closed (B5).
const RESEARCH_STORY = [
  'Write a short news brief (about 200 words, every claim cited) on the most significant open-weight AI model release of the past two weeks.',
  'In your first cell, write a one-paragraph plan as your first revision with work_disposition "completed" (that revision completes this request),',
  'and in the same cell Cast research to find the release, its benchmark claims and one independent evaluation.',
  'Research instructions, pass them to research verbatim: start four searches at once with choir.SearchStart, one per query,',
  'read them with choir.Searches.Get as providers answer, Read the pages you will cite, and send one final Report.',
  'When research reports, incorporate its findings in a later revision.',
].join(' ');

const FOLLOW_UP = [
  'Follow-up on this story: add a short section on what independent evaluators said about this release.',
  'Ask research for one more desk_go_eval cell that starts eight tagged searches at once with choir.SearchStart',
  '(tags f1..f8) and report which result came back for which tag.',
].join(' ');
const AFTER_RESTART = [
  'The computer restarted while this story was in progress. Finish the story from what is already on the page;',
  'if research results were lost, ask research again with the same eight tagged searches started at once in one cell.',
].join(' ');

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const iso = (ms = Date.now()) => new Date(ms).toISOString();
const log = (msg) => console.error(`[tracer ${iso().slice(11, 19)}] ${msg}`);
const TERMINAL = new Set(['completed', 'failed', 'cancelled', 'blocked', 'passivated']);

function writePrivate(path, content) {
  mkdirSync(dirname(path), { recursive: true, mode: 0o700 });
  writeFileSync(path, content, { mode: 0o600 });
  chmodSync(path, 0o600);
}

// ---- mint ----------------------------------------------------------------

function runMint(label) {
  return new Promise((resolve, reject) => {
    execFile(process.execPath, [join(REPO, 'scripts', 'qa_test_computer.mjs'), '--label', label],
      { cwd: REPO, encoding: 'utf8', maxBuffer: 4 * 1024 * 1024, timeout: 30 * 60_000 },
      (err, stdout, stderr) => {
        if (err) {
          const text = String(stderr || err.message);
          const cause = text.split('\n').filter((l) => /^\s*(\w*Error|Error):/.test(l)).pop() || text.slice(-400);
          return reject(new Error(`mint failed: ${cause.trim()}`));
        }
        const line = String(stdout).trim().split('\n').filter(Boolean).pop() || '';
        try { resolve(JSON.parse(line)); } catch { reject(new Error(`mint printed no JSON line: ${line.slice(0, 200)}`)); }
      });
  });
}

// ---- a client that only talks to the computer this invocation minted -------

class MintedComputer {
  constructor(minted) {
    this.computerID = minted.computer_id;
    const cred = JSON.parse(readFileSync(minted.credential_file, 'utf8'));
    if (cred.computer_id !== this.computerID) throw new Error('credential file names a different computer than the mint printed');
    this.apiKey = cred.api_key; // never printed, logged or written
    if (!this.apiKey) throw new Error('credential file has no key');
  }

  guard(computerID) {
    if (computerID !== this.computerID) throw new Error(`refusing to act on ${computerID}: not minted by this invocation`);
  }

  async api(path, method = 'GET', body = undefined, timeoutMs = 60_000) {
    if (path.startsWith('/internal/')) throw new Error('internal routes are not product API');
    const m = path.match(/^\/api\/computers\/([^/]+)\//);
    if (m) this.guard(decodeURIComponent(m[1]));
    const started = Date.now();
    try {
      const res = await fetch(`${HOST}${path}`, {
        method,
        headers: {
          Authorization: `Bearer ${this.apiKey}`, 'X-Choir-Computer': this.computerID,
          ...(body ? { 'Content-Type': 'application/json' } : {}),
        },
        body: body ? JSON.stringify(body) : undefined,
        signal: AbortSignal.timeout(timeoutMs),
      });
      const text = await res.text();
      let json = null;
      try { json = text ? JSON.parse(text) : null; } catch { json = null; }
      return { status: res.status, body: json, ms: Date.now() - started };
    } catch (err) {
      return { status: 0, body: null, error: String(err?.message || err).slice(0, 300), ms: Date.now() - started };
    }
  }

  // The guest must say it is the minted computer before and after a restart.
  async observability() {
    const obs = await this.api('/api/runtime/observability', 'GET', undefined, 20_000);
    if (obs.status === 200 && obs.body?.computer_id && obs.body.computer_id !== this.computerID) {
      throw new Error(`guest answered as ${obs.body.computer_id}, not the minted ${this.computerID}`);
    }
    return obs;
  }
}

// ---- reads ---------------------------------------------------------------

const runID = (r) => r?.run_id || r?.loop_id || '';
const runTrajectory = (r) => r?.trajectory_id || r?.metadata?.trajectory_id || '';

async function trajectoryRuns(c, traj, sinceISO) {
  const res = await c.api(`/api/runs?trajectory_id=${encodeURIComponent(traj)}&limit=200`);
  const all = Array.isArray(res.body?.runs) ? res.body.runs : [];
  return {
    status: res.status,
    ofTrajectory: all.filter((r) => runTrajectory(r) === traj),
    sinceStart: all.filter((r) => String(r.created_at || '') >= sinceISO),
  };
}

// Run events are served by GET /api/runs/{id}/events once Cut 0's read route
// lands; until then the tracer falls back to run state alone.
async function cellInFlight(c, rid) {
  let after = 0;
  let open = 0;
  for (let page = 0; page < 20; page++) {
    const res = await c.api(`/api/runs/${encodeURIComponent(rid)}/events?after=${after}&limit=1000`);
    if (res.status !== 200 || !Array.isArray(res.body?.events)) return page === 0 ? { served: false, inFlight: null } : { served: true, inFlight: open > 0 };
    for (const e of res.body.events) {
      let payload = e.payload;
      if (typeof payload === 'string') { try { payload = JSON.parse(payload); } catch { payload = {}; } }
      if (payload?.tool !== 'desk_go_eval') continue;
      if (e.kind === 'tool.invoked') open++;
      if (e.kind === 'tool.result') open = Math.max(0, open - 1);
    }
    const next = Number(res.body.next_after || 0);
    if (res.body.events.length === 0 || !(next > after)) break;
    after = next;
  }
  return { served: true, inFlight: open > 0 };
}

async function deskRevisions(c, doc) {
  const res = await c.api(`/api/texture/documents/${encodeURIComponent(doc)}/revisions`);
  const revs = Array.isArray(res.body?.revisions) ? res.body.revisions : [];
  return revs.filter((r) => r.author_kind !== 'user');
}

async function obligations(c) {
  const res = await c.api('/api/runtime/obligations', 'GET', undefined, 30_000);
  return res.status === 200 ? res.body : { unavailable: true, status: res.status, error: res.error || res.body?.error || null };
}

// ---- writes (all through owner product routes) ------------------------------

async function submitPrompt(c, text, tag, deadline) {
  let last = null;
  while (Date.now() < deadline) {
    last = await c.api('/api/prompt-bar', 'POST', { text, command_id: `${LABEL}-${tag}-${randomUUID()}` }, 120_000);
    if (last.status >= 200 && last.status < 300 && last.body?.trajectory_id) return last;
    if (last.status >= 400 && last.status < 500 && ![404, 409, 425, 429].includes(last.status)) break;
    await sleep(5_000);
  }
  throw new Error(`prompt-bar submit refused: ${last?.status} ${last?.body?.error || last?.error || ''}`);
}

async function revise(c, doc, text, tag, deadline) {
  let last = null;
  while (Date.now() < deadline) {
    last = await c.api(`/api/texture/documents/${encodeURIComponent(doc)}/revise`, 'POST',
      { prompt: text, client_request_id: `${LABEL}-${tag}-${randomUUID()}` }, 120_000);
    if (last.status >= 200 && last.status < 300) return last;
    if (last.status !== 409 && last.status < 500 && last.status !== 0) break;
    await sleep(4_000);
  }
  return last;
}

// ---- one attempt -----------------------------------------------------------

async function attempt(n, out) {
  const label = ATTEMPTS > 1 ? `${LABEL}-a${n}` : LABEL;
  Object.assign(out, { attempt: n, label, timings: {}, phases: [] });
  const phase = (name, extra = {}) => {
    const row = { name, at: iso(), ...extra };
    out.phases.push(row);
    log(`${name} ${JSON.stringify(extra)}`);
  };

  if (EXPECT_SHA) {
    const fence = await waitForQuietDeploy();
    phase('identity_fence', fence);
    if (!fence.ok) throw new Error(`identity fence: ${fence.reason}`);
  }
  const mintStarted = Date.now();
  const minted = await runMint(label);
  out.timings.mint_ms = Date.now() - mintStarted;
  out.timings.mint_breakdown = minted.timings || null;
  out.computer_id = minted.computer_id;
  out.guest_build_commit = minted.guest_build_commit || null;
  phase('mint', { computer_id: minted.computer_id, mint_ms: out.timings.mint_ms, build: out.guest_build_commit });
  out.contamination = [];
  const fenceBuild = (where, commit) => {
    const got = String(commit?.commit || commit || '').toLowerCase();
    if (EXPECT_SHA && got && !got.startsWith(EXPECT_SHA)) {
      out.contamination.push({ where, expected: EXPECT_SHA, got });
      phase('contaminated', { where, expected: EXPECT_SHA, got });
    }
  };
  fenceBuild('mint', out.guest_build_commit);
  const c = new MintedComputer(minted);
  out.stopComputer = () => stopMinted(c, label);

  const t0 = Date.now();
  const deadline = t0 + TIMEOUT_MIN * 60_000;
  const startISO = iso(t0 - 5_000);
  const obs = await c.observability();
  out.build = obs.body?.build || null;

  // Story submit; the first accepted submit is the "first ready prompt bar".
  const story = await submitPrompt(c, SCENARIO === 'management' ? MANAGEMENT_STORY : SCENARIO === 'research' ? RESEARCH_STORY : STORY, 'story', deadline);
  out.timings.first_prompt_accepted_ms = Date.now() - t0;
  out.trajectory_id = story.body.trajectory_id;
  out.doc_id = story.body.doc_id;
  const submittedAt = Date.now();
  phase('story_submitted', { trajectory_id: out.trajectory_id, doc_id: out.doc_id, after_mint_ms: out.timings.first_prompt_accepted_ms });
  if (SCENARIO === 'management') return managementScenario(c, out, n, deadline, phase);
  if (SCENARIO === 'research') return researchScenario(c, out, n, deadline, phase);

  // Load: second run on a busy desk, sent while a run is observed mid-cell.
  const load = { parallel_cell_requested: true, double_cast_requested: true, second_run_submitted_at: null, observed_run_id: null, observed_by: null };
  const runningSince = new Map();
  let firstRevisionAt = null;
  const loadDeadline = SCENARIO === 'crash' ? submittedAt : Math.min(deadline, submittedAt + 12 * 60_000);
  while (Date.now() < loadDeadline && !load.second_run_submitted_at) {
    const { ofTrajectory } = await trajectoryRuns(c, out.trajectory_id, startISO);
    for (const r of ofTrajectory) {
      if (r.state !== 'running') { runningSince.delete(runID(r)); continue; }
      if (!runningSince.has(runID(r))) runningSince.set(runID(r), Date.now());
      const cell = await cellInFlight(c, runID(r));
      const busy = cell.served ? cell.inFlight : Date.now() - runningSince.get(runID(r)) >= 8_000;
      if (!busy) continue;
      const res = await revise(c, out.doc_id, FOLLOW_UP, 'follow-up', Math.min(deadline, Date.now() + 120_000));
      load.observed_run_id = runID(r);
      load.observed_profile = r.agent_profile || null;
      load.observed_by = cell.served ? 'run_events_cell_in_flight' : 'run_state_running_8s';
      load.second_run_status = res?.status ?? null;
      if (res && res.status >= 200 && res.status < 300) {
        load.second_run_submitted_at = iso();
        load.second_run_route = 'POST /api/texture/documents/{id}/revise';
      } else {
        load.second_run_error = res?.body?.error || res?.error || null;
      }
      phase('second_run', { observed_run_id: load.observed_run_id, profile: load.observed_profile, by: load.observed_by, status: load.second_run_status, error: load.second_run_error || undefined });
      break;
    }
    if (load.observed_run_id) break;
    if (!firstRevisionAt && (await deskRevisions(c, out.doc_id)).length > 0) firstRevisionAt = Date.now();
    await sleep(3_000);
  }
  if (!load.observed_run_id) phase('second_run_not_sent', { reason: 'no run of the trajectory observed mid-cell before the load window closed' });

  // First desk revision of the story.
  while (SCENARIO === 'story' && !firstRevisionAt && Date.now() < deadline) {
    if ((await deskRevisions(c, out.doc_id)).length > 0) firstRevisionAt = Date.now();
    else await sleep(5_000);
  }
  if (firstRevisionAt) {
    out.timings.submit_to_first_revision_ms = firstRevisionAt - submittedAt;
    phase('first_revision', { submit_to_first_revision_ms: out.timings.submit_to_first_revision_ms });
  } else {
    phase('first_revision_missing', {});
  }

  // Let the load land before the drill: a restart mid-cell would kill the
  // parallel cell before its desk.cell and desk.egress events are persisted.
  // Settled = at least one research run seen and none still running (or
  // --settle-min passes; the drill then lands on whatever is in flight).
  const settleDeadline = SCENARIO === 'crash' ? Date.now() : Math.min(deadline - 5 * 60_000, Date.now() + SETTLE_MIN * 60_000);
  const researchSeen = new Set();
  while (Date.now() < settleDeadline) {
    const { ofTrajectory } = await trajectoryRuns(c, out.trajectory_id, startISO);
    const research = ofTrajectory.filter((r) => r.agent_profile === 'research');
    research.forEach((r) => researchSeen.add(runID(r)));
    if (research.length > 0 && research.every((r) => r.state !== 'running' && r.state !== 'pending')) break;
    await sleep(5_000);
  }
  out.timings.submit_to_load_settled_ms = Date.now() - submittedAt;
  phase('load_settled', { research_runs: researchSeen.size, ms: out.timings.submit_to_load_settled_ms });

  // Crash drill: restart only after observing an in-flight run.
  const drill = out.drill = { label, restart_requested_at: null, in_flight_run_id: null, ready_at: null, obligations_before: null, obligations_after: null, load, attempts: n, note: [] };
  let inFlight = null;
  let provoked = false;
  const quietSince = Date.now();
  while (Date.now() < deadline && !inFlight) {
    const { ofTrajectory, sinceStart } = await trajectoryRuns(c, out.trajectory_id, startISO);
    const candidate = ofTrajectory.find((r) => r.state === 'running') || sinceStart.find((r) => r.state === 'running');
    if (candidate) {
      // Obligations first, then the last look at the run, then the restart:
      // nothing slow sits between observing the run in flight and the POST.
      drill.obligations_before = await obligations(c);
      const again = await c.api(`/api/runs/${encodeURIComponent(runID(candidate))}`);
      if (again.status === 200 && again.body?.state === 'running') {
        inFlight = { ...candidate, run_id: runID(candidate) };
        // Every run of the trajectory still open at the crash: the crash
        // scenario's claim is about all of them.
        const seen = new Map();
        for (const r of [...ofTrajectory, ...sinceStart]) if (OPEN_RUN_STATES.has(r.state)) seen.set(runID(r), { run_id: runID(r), profile: r.agent_profile || null, state: r.state });
        drill.pre_crash_open = [...seen.values()];
        break;
      }
    }
    if (!provoked && Date.now() - quietSince > 90_000) {
      const res = await revise(c, out.doc_id, FOLLOW_UP.replace('Follow-up on this story', 'Drill follow-up'), 'drill', Math.min(deadline, Date.now() + 120_000));
      provoked = true;
      drill.note.push(`no run was in flight 90 s after the load phase; sent a drill follow-up (status ${res?.status})`);
      phase('drill_provoke', { status: res?.status });
    }
    await sleep(2_000);
  }
  if (!inFlight) throw new Error('crash drill: no in-flight run observed before the deadline; not restarting an idle computer');
  drill.in_flight_run_id = inFlight.run_id;
  drill.in_flight_run_profile = inFlight.agent_profile || null;
  drill.in_flight_run_trajectory = runTrajectory(inFlight) || null;
  drill.restart_requested_at = iso();
  const restartStarted = Date.now();
  phase('restart_requested', { in_flight_run_id: inFlight.run_id, profile: drill.in_flight_run_profile });
  const restart = await c.api(`/api/computers/${encodeURIComponent(c.computerID)}/lifecycle/restart`, 'POST',
    { idempotency_key: `${label}-restart-${randomUUID()}` }, 15 * 60_000);
  out.timings.restart_call_ms = Date.now() - restartStarted;
  drill.restart_response = { status: restart.status, error: restart.body?.error || restart.error || null, receipt: restart.status < 300 ? restart.body : null };
  phase('restart_answered', { status: restart.status, ms: out.timings.restart_call_ms });

  const readyDeadline = Math.max(deadline, Date.now() + 10 * 60_000);
  while (Date.now() < readyDeadline) {
    const o = await c.observability();
    if (o.status === 200 && o.body?.computer_id === c.computerID) {
      drill.ready_at = iso();
      drill.ready_build = o.body.build || null;
      fenceBuild('after_restart', drill.ready_build);
      Object.assign(drill, bootInstant(o.body, drill.restart_requested_at, drill.ready_at));
      break;
    }
    await sleep(3_000);
  }
  if (!drill.ready_at) throw new Error('computer did not answer after the restart');
  out.timings.restart_to_ready_ms = Date.parse(drill.ready_at) - restartStarted;
  phase('ready', { restart_to_ready_ms: out.timings.restart_to_ready_ms, boot_at: drill.boot_at, boot_at_source: drill.boot_at_source });
  drill.obligations_after = await obligations(c);

  // Fate of the in-flight run: closed (crash rule) or resumed.
  let fate = null;
  for (let i = 0; i < 20; i++) {
    const r = await c.api(`/api/runs/${encodeURIComponent(drill.in_flight_run_id)}`);
    fate = r.status === 200 ? { state: r.body?.state, error: r.body?.error || null, finished_at: r.body?.finished_at || null } : { status: r.status };
    if (fate.state && fate.state !== 'running') break;
    await sleep(3_000);
  }
  drill.in_flight_run_after = fate;
  drill.restart_kind = drill.obligations_after?.restart?.kind || null;
  phase('in_flight_fate', { ...fate, restart_kind: drill.restart_kind });

  if (SCENARIO === 'crash') {
    // No owner act: watch the quiet window, then read every run that was
    // open at the crash.
    const quietUntil = Date.parse(drill.ready_at) + CRASH_QUIET_MS;
    while (Date.now() < quietUntil) await sleep(5_000);
    drill.closure = [];
    for (const r of drill.pre_crash_open || []) {
      const got = await c.api(`/api/runs/${encodeURIComponent(r.run_id)}`);
      drill.closure.push({ ...r, after: got.status === 200 ? { state: got.body?.state, error: got.body?.error || null } : { status: got.status } });
    }
    drill.quiet_window_ms = CRASH_QUIET_MS;
    phase('closure', { open_at_crash: drill.closure.length, closed_with_fate: drill.closure.filter((x) => x.after?.state === 'cancelled' && x.after?.error === 'interrupted_by_restart').length });
  }

  // Continue the story. If the restart closed the work and nothing moves,
  // the owner asks again (an owner act, recorded).
  if (SCENARIO === 'story') {
  const revsBefore = new Set((await deskRevisions(c, out.doc_id)).map((r) => r.revision_id));
  let continued = null;
  let askedAgain = false;
  const quietAfterReady = Date.now();
  while (Date.now() < deadline) {
    const revs = await deskRevisions(c, out.doc_id);
    const fresh = revs.filter((r) => !revsBefore.has(r.revision_id));
    if (fresh.length > 0) { continued = fresh[0]; break; }
    const { ofTrajectory } = await trajectoryRuns(c, out.trajectory_id, startISO);
    const moving = ofTrajectory.some((r) => r.state === 'running' || r.state === 'pending');
    if (!askedAgain && !moving && Date.now() - quietAfterReady > 60_000) {
      const res = await revise(c, out.doc_id, AFTER_RESTART, 'after-restart', Math.min(deadline, Date.now() + 120_000));
      askedAgain = true;
      drill.note.push(`after the restart nothing moved for 60 s; owner asked again (status ${res?.status}${res?.body?.error ? `: ${res.body.error}` : ''})`);
      phase('owner_asked_again', { status: res?.status, error: res?.body?.error || undefined });
    }
    await sleep(5_000);
  }
  if (continued) {
    out.timings.ready_to_next_revision_ms = Date.parse(continued.created_at || iso()) - Date.parse(drill.ready_at);
    phase('revision_after_restart', { revision_id: continued.revision_id, owner_asked_again: askedAgain });
  } else {
    drill.note.push('no desk revision after the restart before the deadline');
  }
  drill.continued_after_restart = { revision_id: continued?.revision_id || null, owner_asked_again: askedAgain };

  // Publication. A prompt-bar story has no autonomous publish path today
  // (wirepublish.EligibleForAutonomousPublish admits only the platform wire
  // owner); publishing is the owner's POST /api/platform/texture/publications.
  drill.publication = { autonomous: false, owner_act: null };
  if (OWNER_PUBLISH && continued) {
    const pub = await c.api('/api/platform/texture/publications', 'POST', { doc_id: out.doc_id }, 120_000);
    drill.publication.owner_act = { route: 'POST /api/platform/texture/publications', status: pub.status, error: pub.body?.error || pub.error || null, publication: pub.status < 300 ? pub.body : null };
    phase('owner_publish', { status: pub.status });
  } else {
    drill.note.push('nothing publishes a prompt-bar story without an owner act; owner publish not requested (--owner-publish)');
  }
  }
  // Export only after quiescence (no run of this computer pending or
  // running), bounded; an open run at export reads as an I2 failure.
  const quiesceDeadline = SCENARIO === 'crash' ? Date.now() : Date.now() + QUIESCE_MIN * 60_000;
  drill.quiescent = false;
  let open = [];
  while (Date.now() < quiesceDeadline) {
    const { ofTrajectory, sinceStart } = await trajectoryRuns(c, out.trajectory_id, startISO);
    const seen = new Map([...sinceStart, ...ofTrajectory].map((r) => [runID(r), r]));
    open = [...seen.values()].filter((r) => r.state === 'running' || r.state === 'pending');
    if (open.length === 0) { drill.quiescent = true; drill.quiescent_at = iso(); break; }
    await sleep(10_000);
  }
  if (!drill.quiescent) {
    drill.open_at_export = open.map((r) => ({ run_id: runID(r), profile: r.agent_profile || null, state: r.state }));
    drill.note.push(`not quiescent after ${QUIESCE_MIN} min; exported with ${open.length} run(s) open`);
  }
  phase('quiescence', { quiescent: drill.quiescent, open: open.length });
  drill.note = drill.note.join('; ');

  // Export, insert the drill block, keep the bundle.
  const drillPath = join(TRACE_DIR, `drill-a${n}.json`);
  writePrivate(drillPath, `${JSON.stringify(drill, null, 2)}\n`);
  out.drill_file = drillPath;
  delete out.drill;
  if (SKIP_EXPORT) {
    out.export = { skipped: true };
    load.I5_load = 'unknown';
    return out;
  }
  const bundlePath = join(TRACE_DIR, `bundle-a${n}.json`);
  const exported = await runExporter(c, out.trajectory_id, bundlePath);
  out.export = { command: EXPORTER, exit: exported.code, ms: exported.ms, stderr_tail: exported.stderr.slice(-600) };
  out.timings.export_ms = exported.ms;
  if (exported.code !== 0 || !existsSync(bundlePath)) {
    load.I5_load = 'unknown';
    out.export.failed = true;
    writePrivate(drillPath, `${JSON.stringify(drill, null, 2)}\n`);
    phase('export_failed', { exit: exported.code });
    return out;
  }
  const bundle = JSON.parse(readFileSync(bundlePath, 'utf8'));
  if (bundle.computer_id && bundle.computer_id !== c.computerID) throw new Error(`exported bundle names ${bundle.computer_id}, not the minted computer`);
  const check = loadCheck(bundle);
  load.I5_load = check.present ? 'present' : 'absent';
  load.evidence = check.evidence;
  bundle.drill = drill;
  writePrivate(bundlePath, `${JSON.stringify(bundle, null, 2)}\n`);
  writePrivate(drillPath, `${JSON.stringify(drill, null, 2)}\n`);
  out.bundle_path = bundlePath;
  out.sha256 = createHash('sha256').update(readFileSync(bundlePath)).digest('hex');
  out.load = { I5_load: load.I5_load, ...check.evidence };
  phase('bundle_kept', { path: bundlePath, sha256: out.sha256, I5_load: load.I5_load });
  out.runs = runLegs(bundle);
  const checkerPath = join(REPO, 'scripts', 'reeng_invariants.mjs');
  if (existsSync(checkerPath)) {
    const reportPath = join(TRACE_DIR, `invariants-a${n}.json`);
    const verdicts = await new Promise((resolve) => {
      execFile(process.execPath, [checkerPath, bundlePath, '--json', reportPath], { cwd: REPO, encoding: 'utf8', timeout: 120_000 },
        (err, stdout) => resolve({ exit: err ? err.code ?? -1 : 0, lines: String(stdout || '').trim().split('\n').filter(Boolean) }));
    });
    if (existsSync(reportPath)) chmodSync(reportPath, 0o600);
    out.invariants = { report: reportPath, exit: verdicts.exit, lines: verdicts.lines };
    if (SCENARIO === 'crash') {
      let rules = [];
      try { rules = JSON.parse(readFileSync(reportPath, 'utf8')).rules || []; } catch { rules = []; }
      const verdict = (id) => rules.find((r) => r.rule === id)?.verdict || 'missing';
      const closure = drill.closure || [];
      const allClosed = closure.length > 0 && closure.every((x) => x.after?.state === 'cancelled' && x.after?.error === 'interrupted_by_restart');
      out.acceptance = {
        scenario: 'crash', claim: 'a markerless boot closes every run open at the crash with fate interrupted_by_restart and dispatches nothing',
        runs_open_at_crash: closure.length, all_closed_with_fate: allClosed, I3: verdict('I3'), quiet_window_ms: drill.quiet_window_ms,
        pass: allClosed && verdict('I3') === 'pass',
      };
      phase('acceptance', out.acceptance);
    }
    phase('invariants', { exit: verdicts.exit, verdicts: verdicts.lines.map((l) => l.split(/\s+/).slice(0, 2).join(' ')) });
  }
  return out;
}

// Staging holds few live computers; a fresh owner's computer stays
// not_started while capacity is full. The tracer stops what it minted once
// the bundle is kept (or the attempt ends), unless --keep-computer.
async function stopMinted(c, label) {
  if (!c || KEEP_COMPUTER) return null;
  const res = await c.api(`/api/computers/${encodeURIComponent(c.computerID)}/lifecycle/stop`, 'POST',
    { idempotency_key: `${label}-stop-${randomUUID()}` }, 10 * 60_000);
  log(`stopped ${c.computerID}: ${res.status} ${res.body?.resulting_lifecycle_state || res.body?.error || res.error || ''}`);
  return { status: res.status, state: res.body?.resulting_lifecycle_state || null };
}

function runExporter(c, traj, outPath) {
  const [cmd, ...pre] = EXPORTER.split(/\s+/).filter(Boolean);
  const started = Date.now();
  return new Promise((resolve) => {
    const child = spawn(cmd, [...pre, traj, '--out', outPath], {
      cwd: REPO,
      env: { ...process.env, CHOIR_API_KEY: c.apiKey, CHOIR_COMPUTER: c.computerID, CHOIR_HOST: HOST },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    let stderr = '';
    child.stdout.on('data', () => {});
    child.stderr.on('data', (d) => { stderr += d; });
    const timer = setTimeout(() => child.kill('SIGKILL'), 15 * 60_000);
    child.on('error', (err) => { clearTimeout(timer); resolve({ code: -1, stderr: String(err), ms: Date.now() - started }); });
    child.on('close', (code) => {
      clearTimeout(timer);
      // The key is in the child's env only; scrub it from anything kept.
      resolve({ code, stderr: stderr.split(c.apiKey).join('<redacted>'), ms: Date.now() - started });
    });
  });
}

// bootInstant: when the restarted guest booted. The guest's recent_log is a
// 512-line ring of standard-logger lines ("YYYY/MM/DD HH:MM:SS ...", UTC on
// the guest); while the ring has not wrapped, its first line is the boot. The
// stamp must fall between the restart request and the first answer, else the
// first answer after the restart stands in (an upper bound).
function bootInstant(body, requestedAt, readyAt) {
  const lines = Array.isArray(body?.recent_log) ? body.recent_log : [];
  const m = lines.length > 0 && lines.length < 512 ? String(lines[0]).match(/^(\d{4})\/(\d{2})\/(\d{2}) (\d{2}):(\d{2}):(\d{2})/) : null;
  if (m) {
    const at = Date.UTC(+m[1], +m[2] - 1, +m[3], +m[4], +m[5], +m[6]);
    if (at >= Date.parse(requestedAt) - 2_000 && at <= Date.parse(readyAt) + 2_000) {
      return { boot_at: new Date(at).toISOString(), boot_at_source: 'observability.recent_log[0]' };
    }
  }
  return { boot_at: readyAt, boot_at_source: 'first answer after restart (upper bound)' };
}

// The load: two desk.egress events with overlapping [started_at, finished_at]
// inside one desk.cell. A desk.cell that waited for another run is reported
// only: runs of one desk agent are serialized, so it cannot be provoked.
function loadCheck(bundle) {
  const items = (layer) => (bundle.layers?.[layer]?.status === 'present' ? bundle.layers[layer].items || [] : []);
  const cells = items('cells');
  const egress = items('egress');
  const t = (s) => Date.parse(s || '');
  let overlapCell = null;
  for (const cell of cells) {
    const inside = egress.filter((e) => e.run_id === cell.run_id
      && (!cell.desk_session || !e.desk_session || e.desk_session === cell.desk_session)
      && t(e.started_at) >= t(cell.started_at) && t(e.finished_at) <= t(cell.finished_at))
      .sort((a, b) => t(a.started_at) - t(b.started_at));
    let maxEnd = -Infinity;
    let overlaps = 0;
    for (const e of inside) {
      if (t(e.started_at) < maxEnd) overlaps++;
      maxEnd = Math.max(maxEnd, t(e.finished_at));
    }
    if (overlaps > 0 && (!overlapCell || overlaps > overlapCell.overlapping)) {
      overlapCell = { run_id: cell.run_id, started_at: cell.started_at, egress_in_cell: inside.length, overlapping: overlaps };
    }
  }
  const waited = cells.filter((cell) => cell.waited_for_run);
  return {
    present: Boolean(overlapCell),
    evidence: {
      cells: cells.length, egress: egress.length,
      cells_layer: bundle.layers?.cells?.status || 'missing', egress_layer: bundle.layers?.egress?.status || 'missing',
      concurrent_egress_cell: overlapCell,
      waited_cells: waited.slice(0, 5).map((cell) => ({ run_id: cell.run_id, waited_for_run: cell.waited_for_run, waited_ms: cell.waited_ms })),
    },
  };
}

// Per-run latency and token figures (the timing baseline m0_qa_probe kept).
function runLegs(bundle) {
  const runs = bundle.layers?.runs?.items || [];
  return runs.map((r) => ({
    run_id: runID(r), profile: r.agent_profile || null, state: r.state || null,
    wall_ms: r.finished_at && r.created_at ? Date.parse(r.finished_at) - Date.parse(r.created_at) : null,
    input_tokens: r.metadata?.input_tokens ?? null, output_tokens: r.metadata?.output_tokens ?? null,
    model: r.metadata?.llm_model || null,
  }));
}

// managementScenario watches the Texture-management conversation on records
// until management's final Report and a Texture revision after it, then
// exports and checks. No crash, no owner act after the request.
async function managementScenario(c, out, n, deadline, phase) {
  const textureInbox = `texture:${out.doc_id}`;
  let ownerID = null;
  const inbox = async (id) => {
    const r = await c.api(`/api/inboxes/${encodeURIComponent(id)}/records?limit=1000`);
    return r.status === 200 ? r.body : null;
  };
  const claim = { cast_to_management: null, management_final_report: null, texture_read_report: null };
  while (Date.now() < deadline && !(claim.management_final_report && claim.texture_read_report)) {
    if (!ownerID) {
      const runs = await c.api(`/api/runs?trajectory_id=${encodeURIComponent(out.trajectory_id)}&limit=50`);
      const list = runs.body?.runs || runs.body?.items || (Array.isArray(runs.body) ? runs.body : []);
      ownerID = list.find((r) => r.owner_id)?.owner_id || null;
    }
    if (ownerID && !claim.cast_to_management) {
      const m = await inbox(`management:${ownerID}`);
      const cast = (m?.records || []).find((r) => r.sender_inbox === textureInbox && r.directive?.subtype === 'cast');
      if (cast) { claim.cast_to_management = { record_id: cast.record_id, round: cast.round, at: cast.provenance?.committed_at }; phase('cast_to_management', claim.cast_to_management); }
    }
    if (claim.cast_to_management && !claim.management_final_report) {
      const t = await inbox(textureInbox);
      const fin = (t?.records || []).find((r) => r.sender_inbox === `management:${ownerID}` && r.final);
      if (fin) { claim.management_final_report = { record_id: fin.record_id, round: fin.round, seq: fin.inbox_seq, at: fin.provenance?.committed_at }; phase('management_final_report', claim.management_final_report); }
    }
    if (claim.management_final_report && !claim.texture_read_report) {
      const t = await inbox(textureInbox);
      if (t && Number(t.cursor) >= Number(claim.management_final_report.seq)) { claim.texture_read_report = { cursor: t.cursor, head: t.head, at: new Date().toISOString() }; phase('texture_read_report', claim.texture_read_report); }
    }
    await sleep(5_000);
  }
  out.drill = { label: out.label, scenario: 'management', claim, note: [] };
  const bundlePath = join(TRACE_DIR, `bundle-a${n}.json`);
  const exported = await runExporter(c, out.trajectory_id, bundlePath);
  out.export = { command: EXPORTER, exit: exported.code, ms: exported.ms, stderr_tail: exported.stderr.slice(-600) };
  if (exported.code !== 0 || !existsSync(bundlePath)) { out.export.failed = true; phase('export_failed', { exit: exported.code }); return out; }
  out.bundle_path = bundlePath;
  out.sha256 = createHash('sha256').update(readFileSync(bundlePath)).digest('hex');
  const bundle = JSON.parse(readFileSync(bundlePath, 'utf8'));
  const mgmtInbox = (bundle.layers?.inboxes?.items || []).find((i) => String(i.inbox || '').startsWith('management:'));
  const roundClosed = mgmtInbox ? !(mgmtInbox.open_rounds || []).some((r) => r.caster_inbox === textureInbox && r.round === claim.cast_to_management?.round) : false;
  out.acceptance = {
    scenario: 'management', claim: 'Texture and management converse on records: Cast to management:<owner>, management final Report closes the round, a committed Texture turn reads it',
    ...claim, round_closed: roundClosed,
    pass: Boolean(claim.cast_to_management && claim.management_final_report && claim.texture_read_report && roundClosed),
  };
  phase('acceptance', out.acceptance);
  delete out.drill;
  return out;
}

// researchScenario watches the Texture-research conversation until
// research's final Report and a committed Texture turn reading it, then
// exports and reads the async-search claim from the bundle's egress and
// cell results. No crash, no owner act after the request.
async function researchScenario(c, out, n, deadline, phase) {
  const textureInbox = `texture:${out.doc_id}`;
  const researchInbox = `research:${out.doc_id}`;
  const inbox = async (id) => {
    const r = await c.api(`/api/inboxes/${encodeURIComponent(id)}/records?limit=1000`);
    return r.status === 200 ? r.body : null;
  };
  const claim = { cast_to_research: null, research_final_report: null, texture_read_report: null };
  while (Date.now() < deadline && !(claim.research_final_report && claim.texture_read_report)) {
    if (!claim.cast_to_research) {
      const r = await inbox(researchInbox);
      const cast = (r?.records || []).find((x) => x.sender_inbox === textureInbox && x.directive?.subtype === 'cast');
      if (cast) { claim.cast_to_research = { record_id: cast.record_id, round: cast.round, at: cast.provenance?.committed_at }; phase('cast_to_research', claim.cast_to_research); }
    }
    if (claim.cast_to_research && !claim.research_final_report) {
      const t = await inbox(textureInbox);
      const fin = (t?.records || []).find((x) => x.sender_inbox === researchInbox && x.final);
      if (fin) {
        claim.research_final_report = { record_id: fin.record_id, round: fin.round, seq: fin.inbox_seq, system: fin.emitter_kind === 'system', stop: fin.report?.stop_reason || null, at: fin.provenance?.committed_at };
        phase('research_final_report', claim.research_final_report);
      }
    }
    if (claim.research_final_report && !claim.texture_read_report) {
      const t = await inbox(textureInbox);
      if (t && Number(t.cursor) >= Number(claim.research_final_report.seq)) { claim.texture_read_report = { cursor: t.cursor, head: t.head, at: new Date().toISOString() }; phase('texture_read_report', claim.texture_read_report); }
    }
    await sleep(5_000);
  }
  out.drill = { label: out.label, scenario: 'research', claim, note: [] };
  const bundlePath = join(TRACE_DIR, `bundle-a${n}.json`);
  const exported = await runExporter(c, out.trajectory_id, bundlePath);
  out.export = { command: EXPORTER, exit: exported.code, ms: exported.ms, stderr_tail: exported.stderr.slice(-600) };
  if (exported.code !== 0 || !existsSync(bundlePath)) { out.export.failed = true; phase('export_failed', { exit: exported.code }); return out; }
  out.bundle_path = bundlePath;
  out.sha256 = createHash('sha256').update(readFileSync(bundlePath)).digest('hex');
  const bundle = JSON.parse(readFileSync(bundlePath, 'utf8'));
  const events = bundle.layers?.run_events?.items || [];
  const payload = (e) => e.payload || e.data || {};
  const done = events.filter((e) => (e.kind || e.type) === 'desk.egress' && payload(e).verb === 'search_done').map(payload);
  const started = events.filter((e) => (e.kind || e.type) === 'desk.egress' && /search_?start/i.test(String(payload(e).verb || ''))).length;
  const countLines = events.filter((e) => {
    if ((e.kind || e.type) !== 'tool.result') return false;
    let result = null;
    try { result = JSON.parse(String(payload(e).output || '')); } catch { return false; }
    return Array.isArray(result?.searches) && result.searches.some((line) => /\+\d+ new/.test(String(line)));
  }).length;
  const search = {
    started, done: done.length,
    answered: done.reduce((a, d) => a + Number(d.answered || 0), 0),
    total: done.reduce((a, d) => a + Number(d.total || 0), 0),
    cell_results_with_new_arrivals: countLines,
  };
  const textureWork = (bundle.layers?.trajectory?.items?.[0]?.work_items || bundle.layers?.trajectory?.work_items || [])
    .filter((w) => w.assigned_agent_id === textureInbox).map((w) => ({ work_item_id: w.work_item_id, status: w.status }));
  out.acceptance = {
    scenario: 'research',
    claim: 'research searches asynchronously (a search_done with answered providers, and a cell result showing new arrivals) and its final Report reaches texture:<doc>, where a committed Texture turn reads it',
    ...claim, search, texture_work_items_at_export: textureWork,
    pass: Boolean(claim.cast_to_research && claim.research_final_report && claim.texture_read_report && search.answered > 0 && search.cell_results_with_new_arrivals > 0),
  };
  phase('acceptance', out.acceptance);
  delete out.drill;
  return out;
}

// waitForQuietDeploy: staging reports EXPECT_SHA and no CI run on main is
// queued or in progress (a cancelled run's changes can deploy with the next
// push, so a docs push is not proof the lane is quiet).
async function waitForQuietDeploy() {
  const until = Date.now() + FENCE_MIN * 60_000;
  let last = {};
  while (Date.now() < until) {
    let commit = '';
    try {
      const r = await fetch(`${HOST}/health`, { signal: AbortSignal.timeout(10_000) });
      const b = await r.json();
      commit = String(b?.build?.commit || b?.commit || '').toLowerCase();
    } catch { commit = ''; }
    let busy = null;
    try {
      const out = await new Promise((resolve, reject) => execFile('gh', ['run', 'list', '--branch', 'main', '--limit', '8', '--json', 'status'],
        { cwd: REPO, encoding: 'utf8', timeout: 30_000 }, (err, stdout) => (err ? reject(err) : resolve(stdout))));
      busy = JSON.parse(out).filter((r) => r.status !== 'completed').length;
    } catch { busy = null; }
    last = { staging: commit.slice(0, 12), ci_busy: busy };
    if (commit.startsWith(EXPECT_SHA) && busy === 0) return { ok: true, ...last };
    await sleep(20_000);
  }
  return { ok: false, reason: `staging not quiet at ${EXPECT_SHA} after ${FENCE_MIN} min`, ...last };
}

// ---- main ------------------------------------------------------------------

mkdirSync(TRACE_DIR, { recursive: true, mode: 0o700 });
chmodSync(TRACE_DIR, 0o700);
const receipt = { schema: 'choir.reeng_tracer_receipt.v1', label: LABEL, host: HOST, started_at: iso(), attempts: [] };
let exitCode = 1;
for (let n = 1; n <= ATTEMPTS; n++) {
  const result = { attempt: n };
  receipt.attempts.push(result);
  try {
    await attempt(n, result);
    if (result.stopComputer) result.computer_stop = await result.stopComputer();
  } catch (err) {
    if (result.stopComputer) result.computer_stop = await result.stopComputer().catch(() => null);
    result.error = String(err?.message || err).slice(0, 600);
    log(`attempt ${n} failed: ${result.error}`);
    if (result.drill) {
      // Keep what the drill saw so a failed attempt is still evidence.
      if (Array.isArray(result.drill.note)) result.drill.note = result.drill.note.join('; ');
      result.drill_file = join(TRACE_DIR, `drill-a${n}.json`);
      writePrivate(result.drill_file, `${JSON.stringify(result.drill, null, 2)}\n`);
      delete result.drill;
    }
    exitCode = 1;
    delete result.stopComputer;
    break;
  }
  delete result.stopComputer;
  if (result.contamination?.length) { exitCode = 5; log(`attempt ${n}: contaminated by a build change; the bundle is not acceptance evidence`); break; }
  if (result.export?.failed) { exitCode = 4; break; }
  if (result.export?.skipped) { exitCode = 0; break; }
  if (result.acceptance) { exitCode = result.acceptance.pass ? 0 : 6; break; }
  if (result.load?.I5_load === 'present') { exitCode = 0; break; }
  exitCode = 3;
  if (n < ATTEMPTS) log(`attempt ${n}: load absent in the bundle; retrying on a fresh computer`);
}
const last = receipt.attempts[receipt.attempts.length - 1] || {};
Object.assign(receipt, {
  finished_at: iso(),
  computer_id: last.computer_id || null,
  trajectory_id: last.trajectory_id || null,
  bundle_path: last.bundle_path || null,
  sha256: last.sha256 || null,
  timings: last.timings || null,
  I5_load: last.load?.I5_load || (last.export ? 'unknown' : null),
  attempt_count: receipt.attempts.length,
  exit_code: exitCode,
});
writePrivate(join(TRACE_DIR, 'receipt.json'), `${JSON.stringify(receipt, null, 2)}\n`);
for (const a of receipt.attempts) {
  if (a.timings) log(`timings a${a.attempt}: ${JSON.stringify({ mint_ms: a.timings.mint_ms, first_prompt_accepted_ms: a.timings.first_prompt_accepted_ms, submit_to_first_revision_ms: a.timings.submit_to_first_revision_ms, restart_to_ready_ms: a.timings.restart_to_ready_ms })}`);
}
if (receipt.sha256) log(`bundle ${receipt.bundle_path} sha256 ${receipt.sha256}`);
console.log(JSON.stringify(receipt));
process.exit(exitCode);
