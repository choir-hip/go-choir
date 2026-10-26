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
// Scenarios: story (default) | crash | management | research | paper |
// engineering | adopt. The engineering scenario is C1e's cut-1 self-development
// drill (see engineeringScenario); its flags:
//   --scenario engineering [--change static|go|go-served] [--timeout-min 15] [--apply-min 10]
//     [--restore-min 12] [--require-served] [--skip-export] [--keep-computer]
//     [--autonomous [--autonomy-min 120] [--decide-min 75]]  (P4: arm standing_consensus once; no owner approval;
//                                       the host builds every candidate before corpusd decides, so the decision
//                                       wait covers P5's queue deadline and build timeout)
// The adopt scenario is Station B minimal (P8, see adoptScenario): computer A
// runs the engineering drill to applied (no restore), its owner publishes the
// release, a second minted computer B refuses a tampered and an unpublished
// record before any state change, then adopts the release as source through
// its own desks and gate:
//   --scenario adopt [--timeout-min 30] [--apply-min 10] [--expect-sha <sha>]
//
// Exit: 0 bundle kept with the load present (or the scenario's claim held);
// 3 bundle(s) kept, load absent (I5 unknown); 4 export unavailable (drill
// kept); 5 contaminated (the guest build changed during the run;
// --expect-sha); 6 the scenario's claim failed (crash, management, research,
// paper, engineering); 1 attempt failed; 2 usage.

import { execFile, spawn } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { chmodSync, existsSync, mkdirSync, readFileSync, readdirSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  castOnRecords, completingInbox, doneJobs, formatWitnessGapTable, goTestCells, isGoBuildOfAutoputer, isGoTest, reportNamesBundle,
  verifiedNamedBundle, visualEvidenceChecks, witnessGapTable, witnessJoined,
} from './reeng_tracer_checks.mjs';
import { inProgressReceipt, interruptedReceipt, signalExitCode, strandedTracerComputers } from './reeng_tracer_receipt.mjs';
import { pinNewestCheckpoint, restoredToPinnedCheckpoint } from './checkpoint_pick.mjs';

const REPO = join(dirname(fileURLToPath(import.meta.url)), '..');
const HOST = (process.env.CHOIR_DEPLOYED_BASE_URL || process.env.CHOIR_HOST || 'https://choir.news').replace(/\/$/, '');
const args = process.argv.slice(2);
const arg = (name, dflt) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 && i + 1 < args.length ? args[i + 1] : dflt;
};
const flag = (name) => args.includes(`--${name}`);
const LABEL = arg('label', '');
const CRED_FILE = arg('cred', '');
// --story-file replaces the research scenario's story with a file's text
// (for example a story that needs a keyed source API).
const STORY_FILE = arg('story-file', '');
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
const USAGE = 'usage: node scripts/reeng_tracer.mjs --label <label> [--scenario story|crash|management|research|paper|engineering|adopt] [--attempts 1] [--timeout-min 15] [--settle-min 3] [--quiesce-min 2] [--exporter <cmd>] [--skip-export] [--owner-publish] [--keep-computer] [--expect-sha <sha>] [--fence-min 30] [--quiet-s 150] [--sources arxiv:cs_ai] [--interval-min 10] [--max-items 6] [--apply-min 10] [--restore-min 12] [--require-served] [--change static|go|go-served|rewrite] [--app <ComponentName>] [--delegate] [--autonomous] [--autonomy-min 120] [--decide-min 75] [--visual]';
if (flag('help')) {
  console.log(USAGE);
  process.exit(0);
}
const SCENARIO = arg('scenario', 'story');
if (!['story', 'crash', 'management', 'research', 'paper', 'engineering', 'adopt'].includes(SCENARIO)) {
  console.error(`unknown --scenario ${SCENARIO} (story | crash | management | research | paper | engineering | adopt)`);
  process.exit(2);
}
// --change (engineering scenario): static asks for a static frontend file,
// which touches no Go input; go asks for a Go file and its test, so Freeze
// must build bin/autoputer and the desk must run the build and the test as
// capsule jobs (docs/design/capsule-long-exec-2026-10-15.md §7). go-served
// is Station A's probe (P5, docs/reports/p5-host-build-layered-apply-2026-10-15.md):
// a guest route GET /api/selfdev-probe that answers the run's nonce, which
// only a binary built from the patched source can serve. The claim adds:
// the route does not serve the nonce before apply, serves it after apply
// from a layered release whose code_commit is the serving build's commit,
// and stops serving it after the restore (the restore restarts into the
// pre-apply release). go-served also keeps the release witness (S6-0,
// docs/problems/s6-activation-trusts-not-compares-2026-10-20.md) fetched at
// frozen, accepted, applied and restored, checks `witness_joined`, and prints
// the gap table (each join, compared at the gate or only recorded).
const CHANGE = arg('change', 'static');
if (!['static', 'go', 'go-served', 'rewrite'].includes(CHANGE)) {
  console.error(`unknown --change ${CHANGE} (static | go | go-served | rewrite)`);
  process.exit(2);
}
// --change rewrite --app <App> [--delegate]: a frontend app rewrite drill.
// The request asks the computer to rewrite frontend/src/lib/<App>.svelte
// (default ImageApp) for clarity, accessibility and maintainability with its
// props, events, exported API, routes and visible behaviour unchanged. It is
// a frontend-only change (no Go probe, no bin/autoputer, no served marker
// file). With --delegate, engineering writes the rewrite through the Claude
// delegate; the build job links the guest's Nix-built dependencies, so the
// same round builds, freezes and completes
// (docs/design/delegate-one-round-build-2026-10-21.md).
const APP = arg('app', 'ImageApp');
const DELEGATE = flag('delegate');
const REWRITE = CHANGE === 'rewrite';
if (!REWRITE && (DELEGATE || arg('app', '') !== '')) {
  console.error('--app and --delegate belong to --change rewrite');
  process.exit(2);
}
if (REWRITE && SCENARIO !== 'engineering') {
  console.error('--change rewrite needs --scenario engineering');
  process.exit(2);
}
if (REWRITE) {
  const appFile = join(REPO, 'frontend', 'src', 'lib', `${APP}.svelte`);
  if (!/^[A-Z][A-Za-z]+App$/.test(APP)) {
    console.error(`invalid --app ${APP} (must match /^[A-Z][A-Za-z]+App$/)`);
    process.exit(2);
  }
  if (!existsSync(appFile)) {
    console.error(`--app ${APP}: ${appFile} does not exist`);
    process.exit(2);
  }
}
const GO_CHANGE = CHANGE === 'go' || CHANGE === 'go-served';
const PROBE_ROUTE = '/api/selfdev-probe';
// The engineering scenario's bounded waits (fast-loop defaults): the
// conversation through the owner-decision boundary is bounded by
// --timeout-min; the apply (approval to applied, with its planned restart)
// by --apply-min; the restore request by --restore-min. --require-served
// makes the served-file check (the change visible on the computer's surface
// after apply and gone after restore) part of the claim; without it, it is
// observed only.
const APPLY_MIN = Number(arg('apply-min', '10'));
const RESTORE_MIN = Number(arg('restore-min', '12'));
const REQUIRE_SERVED = flag('require-served');
if (REQUIRE_SERVED && (GO_CHANGE || REWRITE)) {
  console.error('--require-served checks a static file; a --change go or rewrite drill serves none (go-served checks its own route)');
  process.exit(2);
}
// --autonomous (engineering scenario; P4,
// docs/problems/p4-standing-consensus-autonomy-2026-10-15.md): the drill's
// only owner act is arming standing_consensus once (reversible-selfdev-v1,
// for --autonomy-min minutes, at most 24 h; test-tier computers only). There
// is no owner_approval step: corpusd's decision poster decides from the host
// builder's signed ballot and management's, and the drill expects applied
// with a qualified-consensus decision actor and no decision call of its own.
const AUTONOMOUS = flag('autonomous');
const AUTONOMY_MIN = Number(arg('autonomy-min', '120'));
const DECIDE_MIN = Number(arg('decide-min', '75'));
const REVERSIBLE_SELFDEV_V1 = 'c34ddf073aecaacc307f375d6f2e398798350d7a48c8d3c2e7c6d10248b394d7';
if (AUTONOMOUS && (SCENARIO !== 'engineering' || !(AUTONOMY_MIN > 0 && AUTONOMY_MIN <= 1440))) {
  console.error('--autonomous needs --scenario engineering and 0 < --autonomy-min <= 1440');
  process.exit(2);
}
// --visual (engineering scenario, --change static; docs/design/visual-evidence-
// slice-2026-10-15.md section 5): the change is a visible page,
// frontend/public/choir-drill/<token>.html, whose heading is the token.
// Engineering builds the frontend, takes a screenshot of that page with
// choir-capture (a capsule job), precommits visual_captured and freezes. The
// drill claims, through product routes only: the operation's evidence listing
// names a vouched, non-blank, 1280x800 capture of that route that the asset
// store holds; the asset route serves it (PNG, nosniff, immutable, digest as
// ETag, bytes that hash to the digest, a Range request answered 206). It saves
// the PNG to the trace directory. A vision model's reading of the heading
// (POST /api/model-policy/verify with the image ref) is observed, not
// claimed. The computer must be granted visual capture
// (CHOIR_VISUAL_CAPTURE_COMPUTERS names it or class:test); without the grant
// the capture job exits 3 and the drill fails at visual_listing.
const VISUAL = flag('visual');
if (VISUAL && (SCENARIO !== 'engineering' || CHANGE !== 'static')) {
  console.error('--visual needs --scenario engineering and --change static');
  process.exit(2);
}
const CRASH_QUIET_MS = Number(arg('quiet-s', '150')) * 1000;
// The paper scenario's sources and schedule (Cut 2 2b): the owner act that
// starts Autopaper on a computer is POST /api/autopaper/sources/schedules.
const PAPER_SOURCES = String(arg('sources', 'arxiv:cs_ai')).split(',').map((x) => x.trim()).filter(Boolean);
const PAPER_INTERVAL_MIN = Number(arg('interval-min', '10'));
const PAPER_MAX_ITEMS = Number(arg('max-items', '6'));
const OPEN_RUN_STATES = new Set(['pending', 'running', 'passivated', 'blocked']);
if (!LABEL || !/^[A-Za-z0-9._-]+$/.test(LABEL)) {
  console.error(USAGE);
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
  'In your first cell, write a one-paragraph plan as your first revision with choir.Write and complete this request with choir.Done in the same cell,',
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

// A drill that stops short keeps where the computer was stopped: the in-flight
// steps, the reads that did not answer, and the goroutine stacks
// (engineering-completion-path-cluster-2026-10-20.md, failure 9). The dump
// goes to the private trace directory, never the repo.
async function stallSnapshot(c, n, phase) {
  const obs = await c.api('/api/runtime/observability?goroutines=1', 'GET', undefined, 60_000);
  const path = join(TRACE_DIR, `observability-stall-a${n}.json`);
  if (obs.status === 200) writePrivate(path, JSON.stringify(obs.body, null, 2));
  phase('stall_observed', {
    status: obs.status,
    in_flight: obs.body?.steps?.in_flight || [],
    unanswered: obs.body?.unanswered || [],
    goroutines_bytes: (obs.body?.goroutines || '').length,
    path: obs.status === 200 ? path : null,
  });
}

// ---- mint ----------------------------------------------------------------

function reuseMinted(path) {
  const cred = JSON.parse(readFileSync(path, 'utf8'));
  if (!cred.computer_id || !cred.api_key || !/^test-|@example\.com$/.test(String(cred.email || cred.label || ''))) {
    throw new Error(`--cred ${path}: not a minted test computer credential`);
  }
  return { computer_id: cred.computer_id, user_id: cred.user_id, credential_file: path, guest_build_commit: null, timings: null };
}

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
    this.ownerID = minted.user_id || cred.user_id || '';
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

  // bytes reads a binary product route (an asset) with optional request
  // headers: status, lowercased response headers and the body as a Buffer.
  async bytes(path, headers = {}, timeoutMs = 60_000) {
    if (path.startsWith('/internal/')) throw new Error('internal routes are not product API');
    try {
      const res = await fetch(`${HOST}${path}`, {
        headers: { Authorization: `Bearer ${this.apiKey}`, 'X-Choir-Computer': this.computerID, ...headers },
        signal: AbortSignal.timeout(timeoutMs),
      });
      return { status: res.status, headers: Object.fromEntries([...res.headers].map(([k, v]) => [k.toLowerCase(), v])), bytes: Buffer.from(await res.arrayBuffer()) };
    } catch (err) {
      return { status: 0, headers: {}, bytes: Buffer.alloc(0), error: String(err?.message || err).slice(0, 300) };
    }
  }

  // surface reads a page of the computer's own surface (the guest frontend the
  // proxy serves to its owner), as text. Not an API route.
  async surface(path, timeoutMs = 20_000) {
    if (path.startsWith('/internal/') || path.startsWith('/api/')) throw new Error('surface reads are not API routes');
    try {
      const res = await fetch(`${HOST}${path}`, {
        headers: { Authorization: `Bearer ${this.apiKey}`, 'X-Choir-Computer': this.computerID },
        signal: AbortSignal.timeout(timeoutMs),
      });
      return { status: res.status, text: (await res.text()).slice(0, 400) };
    } catch (err) {
      return { status: 0, text: '', error: String(err?.message || err).slice(0, 300) };
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
//
// A cell in flight is a tool.invoked without its tool.result. tool.invoked is
// a bus-only line since journal cut R3, so a new build's tape has no signal
// for it: the answer is "not served" (the caller then uses its run-age
// fallback) unless the run's tape holds an invoked row, which only an older
// build wrote.
async function cellInFlight(c, rid) {
  let after = 0;
  let open = 0;
  let sawInvoked = false;
  for (let page = 0; page < 20; page++) {
    const res = await c.api(`/api/runs/${encodeURIComponent(rid)}/events?after=${after}&limit=1000`);
    if (res.status !== 200 || !Array.isArray(res.body?.events)) return page === 0 ? { served: false, inFlight: null } : { served: sawInvoked, inFlight: sawInvoked ? open > 0 : null };
    for (const e of res.body.events) {
      let payload = e.payload;
      if (typeof payload === 'string') { try { payload = JSON.parse(payload); } catch { payload = {}; } }
      if (payload?.tool !== 'desk_go_eval') continue;
      if (e.kind === 'tool.invoked') { open++; sawInvoked = true; }
      if (e.kind === 'tool.result') open = Math.max(0, open - 1);
    }
    const next = Number(res.body.next_after || 0);
    if (res.body.events.length === 0 || !(next > after)) break;
    after = next;
  }
  return { served: sawInvoked, inFlight: sawInvoked ? open > 0 : null };
}

// All events of one run, paged; null when the route does not answer.
async function runEvents(c, rid) {
  const events = [];
  let after = 0;
  for (let page = 0; page < 20; page++) {
    const res = await c.api(`/api/runs/${encodeURIComponent(rid)}/events?after=${after}&limit=1000`);
    if (res.status !== 200 || !Array.isArray(res.body?.events)) return page === 0 ? null : events;
    events.push(...res.body.events);
    const next = Number(res.body.next_after || 0);
    if (res.body.events.length === 0 || !(next > after)) break;
    after = next;
  }
  return events;
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
    persistReceipt();
  };

  if (EXPECT_SHA) {
    const fence = await waitForQuietDeploy();
    phase('identity_fence', fence);
    if (!fence.ok) throw new Error(`identity fence: ${fence.reason}`);
  }
  const mintStarted = Date.now();
  // --cred <file> reuses a computer qa_test_computer.mjs minted for this drill
  // (a per-computer grant such as visual capture names the computer before the
  // deploy, so the drill cannot mint it fresh). It is still a computer minted
  // for the drill, never an owner's.
  const minted = CRED_FILE ? reuseMinted(CRED_FILE) : await runMint(label);
  out.timings.mint_ms = Date.now() - mintStarted;
  out.timings.mint_breakdown = minted.timings || null;
  out.computer_id = minted.computer_id;
  out.guest_build_commit = minted.guest_build_commit || null;
  // Bound to the attempt before any phase, so an interrupt from here on
  // stops this computer.
  const c = new MintedComputer(minted);
  out.stopComputer = () => stopMinted(c, label);
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

  const t0 = Date.now();
  const deadline = t0 + TIMEOUT_MIN * 60_000;
  const startISO = iso(t0 - 5_000);
  const obs = await c.observability();
  out.build = obs.body?.build || null;

  if (SCENARIO === 'paper') return paperScenario(c, out, n, deadline, phase);
  if (SCENARIO === 'engineering') return engineeringScenario(c, out, n, deadline, phase, fenceBuild);
  if (SCENARIO === 'adopt') return adoptScenario(c, out, n, deadline, phase, fenceBuild, label);
  // Story submit; the first accepted submit is the "first ready prompt bar".
  const story = await submitPrompt(c, SCENARIO === 'management' ? MANAGEMENT_STORY : SCENARIO === 'research' ? (STORY_FILE ? readFileSync(STORY_FILE, 'utf8').trim() : RESEARCH_STORY) : STORY, 'story', deadline);
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
  // (the platform-owner wire publish path was deleted in Cut 2); publishing
  // is the owner's POST /api/platform/texture/publications.
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
  // A cell's egress calls ride its desk.cell event since journal cut R4; the
  // standalone desk.egress events are older tapes, refusals and search_done.
  const egressRows = [
    ...events.filter((e) => (e.kind || e.type) === 'desk.egress').map(payload),
    ...events.filter((e) => (e.kind || e.type) === 'desk.cell').flatMap((e) => (Array.isArray(payload(e).egress) ? payload(e).egress : [])),
  ];
  const done = egressRows.filter((row) => row.verb === 'search_done');
  const started = egressRows.filter((row) => /search_?start/i.test(String(row.verb || ''))).length;
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
  // Support judgement (Cut 2 2a v), observed, not gated: a Texture commit
  // with new citations lists its pairs to management:<owner>, and
  // management records verdicts; failures come back to texture:<doc>.
  const allRecords = bundle.layers?.records?.items || [];
  const pairsRecords = allRecords.filter((r) => String(r.inbox || '').startsWith('management:') && (r.support?.pairs || []).length > 0);
  const failureRecords = allRecords.filter((r) => r.inbox === textureInbox && (r.support?.failures || []).length > 0);
  const support = {
    pairs_records: pairsRecords.length,
    pairs_listed: pairsRecords.reduce((a, r) => a + (r.support?.pairs || []).length, 0),
    failure_records_to_texture: failureRecords.length,
    management_inbox: (bundle.layers?.inboxes?.items || []).filter((i) => String(i.inbox || '').startsWith('management:')).map((i) => ({ cursor: i.cursor, head: i.head })),
  };
  // What Texture still owes at export, read from records (open rounds whose
  // recipient is texture:<doc>) and from the census of retired work-state
  // objects addressed to it, not from the trajectory's work items (C1e B0).
  const textureOwed = {
    open_rounds: (bundle.layers?.inboxes?.items || []).flatMap((i) => i.open_rounds || [])
      .filter((r) => r.recipient_inbox === textureInbox).map((r) => ({ conversation_id: r.conversation_id, round: r.round })),
    census_objects: (bundle.layers?.census?.items?.[0]?.objects || [])
      .filter((o) => o.to === textureInbox).map((o) => ({ kind: o.kind, canonical_id: o.canonical_id })),
    census_status: bundle.layers?.census?.status || 'absent',
  };
  out.acceptance = {
    scenario: 'research',
    claim: 'research searches asynchronously (a search_done with answered providers, and a cell result showing new arrivals) and its final Report reaches texture:<doc>, where a committed Texture turn reads it',
    ...claim, search, support, texture_owed_at_export: textureOwed,
    pass: Boolean(claim.cast_to_research && claim.research_final_report && claim.texture_read_report && search.answered > 0 && search.cell_results_with_new_arrivals > 0),
  };
  phase('acceptance', out.acceptance);
  delete out.drill;
  return out;
}

// paperScenario is Autopaper on a fresh computer (Cut 2 2b, living stories
// and automatic publishing): one owner act starts the source schedules,
// then nothing else. Its claim: a poll's captures reach management as a
// record, management opens a story by a Cast to texture:new, Texture writes
// it, the story publishes as an Autopaper story (publish_mode auto, read by
// doc id as its owner), and its public route resolves with its citations. The schedules are stopped at the end.
async function paperScenario(c, out, n, deadline, phase) {
  const claim = { schedules_started: [], capture_arrivals: 0, stories: [], published: [] };
  for (const source of PAPER_SOURCES) {
    const r = await c.api('/api/autopaper/sources/schedules', 'POST', { source_id: source, interval_minutes: PAPER_INTERVAL_MIN, max_items: PAPER_MAX_ITEMS, reason: `tracer ${out.label}` });
    claim.schedules_started.push({ source_id: source, status: r.status });
    phase('schedule_started', { source_id: source, status: r.status });
  }
  let ownerID = null;
  const seen = new Set();
  while (Date.now() < deadline) {
    if (!ownerID) {
      const me = await c.api('/api/autopaper/sources/schedules');
      ownerID = (me.body?.schedules || []).find((x) => x.owner_id)?.owner_id || null;
    }
    if (ownerID) {
      const m = await c.api(`/api/inboxes/${encodeURIComponent(`management:${ownerID}`)}/records?limit=1000`);
      const arrivals = (m.body?.records || []).filter((r) => String(r.sender_inbox || '').startsWith('source:'));
      if (arrivals.length !== claim.capture_arrivals) { claim.capture_arrivals = arrivals.length; phase('capture_arrivals', { records: arrivals.length, captures: arrivals.reduce((a, r) => a + (r.captures?.items || []).length, 0) }); }
    }
    const docs = await c.api('/api/texture/documents?limit=100');
    for (const d of docs.body?.documents || docs.body?.items || (Array.isArray(docs.body) ? docs.body : [])) {
      const id = d.doc_id || d.id;
      if (!id || seen.has(id)) continue;
      const inbox = await c.api(`/api/inboxes/${encodeURIComponent(`texture:${id}`)}/records?limit=50`);
      const opening = (inbox.body?.records || []).find((r) => String(r.sender_inbox || '').startsWith('management:') && r.directive?.subtype === 'cast');
      if (!opening) continue;
      seen.add(id);
      claim.stories.push({ doc_id: id, title: d.title || '', publish: opening.cast?.publish || null, captures: (opening.cast?.captures || []).length, opened_at: opening.provenance?.committed_at });
      phase('story_opened', claim.stories[claim.stories.length - 1]);
      if (!out.trajectory_id && d.trajectory_id) out.trajectory_id = d.trajectory_id;
    }
    // Each opened story's publication, read as its owner by doc id: the
    // front list shows only the allowlisted paper owner's stories, never a
    // minted test computer's (docs/problems/tracer-paper-blind-to-test-publications-2026-10-13.md).
    for (const story of claim.stories) {
      const r = await c.api(`/api/platform/texture/publications?doc_id=${encodeURIComponent(story.doc_id)}`);
      if (r.status !== 200 || !r.body) continue;
      const row = r.body;
      const known = claim.published.find((p) => p.doc_id === story.doc_id);
      if (known && known.version_count === row.version_count) continue;
      const entry = { doc_id: story.doc_id, title: row.title, publish_mode: row.publish_mode, on_front: row.on_front, version_count: row.version_count, latest_revision: row.latest_revision, bare: row.route?.bare || null };
      if (known) Object.assign(known, entry); else claim.published.push(entry);
      phase('story_published', entry);
    }
    if (claim.published.some((p) => p.version_count >= 2)) break;
    await sleep(20_000);
  }
  // Read one published story through the public bundle route, logged out.
  let bundleCheck = null;
  const first = claim.published[0];
  if (first?.bare) {
    try {
      const r = await fetch(`${HOST}/api/platform/publications/resolve?route=${encodeURIComponent(first.bare)}`, { signal: AbortSignal.timeout(20_000) });
      const b = r.ok ? await r.json() : null;
      bundleCheck = { status: r.status, citations: (b?.checks?.citations || []).length, states: (b?.checks?.citations || []).map((x) => x.state || x.judgement || 'pending'), nav: b?.nav || null };
    } catch (err) { bundleCheck = { error: String(err).slice(0, 200) }; }
  }
  for (const source of PAPER_SOURCES) {
    const r = await c.api(`/api/autopaper/sources/schedules?source_id=${encodeURIComponent(source)}`, 'DELETE');
    phase('schedule_stopped', { source_id: source, status: r.status });
  }
  out.acceptance = {
    scenario: 'paper',
    claim: 'one owner act starts the schedules; a poll\'s captures reach management; management opens a story by a Cast to texture:new; the story publishes automatically (publish_mode auto) and its public route resolves',
    ...claim, public_bundle: bundleCheck,
    pass: claim.capture_arrivals > 0 && claim.stories.length > 0 && claim.published.some((p) => p.publish_mode === 'auto') && bundleCheck?.status === 200,
  };
  phase('acceptance', out.acceptance);
  if (out.trajectory_id) {
    const bundlePath = join(TRACE_DIR, `bundle-a${n}.json`);
    const exported = await runExporter(c, out.trajectory_id, bundlePath);
    out.export = { command: EXPORTER, exit: exported.code, ms: exported.ms, stderr_tail: exported.stderr.slice(-600) };
    if (exported.code === 0 && existsSync(bundlePath)) {
      out.bundle_path = bundlePath;
      out.sha256 = createHash('sha256').update(readFileSync(bundlePath)).digest('hex');
    }
  }
  return out;
}

// ---- engineering scenario ----------------------------------------------------
//
// engineeringScenario is the cut-1 self-development drill on records (C1e
// step 4, acceptance 3; docs/definitions/choir-reeng-c1e-engineering-on-records-2026-10-13.md,
// design docs/reports/c1e-b1-design-note-2026-10-13.md). On a fresh test
// computer: the owner arms propose_only (Freeze opens a candidate only under
// a proposing mode; a fresh computer's mode is off), then asks for one small
// change only engineering can make (a static file on the computer's frontend
// whose content is a per-run token, checked in the capsule before Freeze).
// The drill watches, through product routes only:
//   1. Texture's Cast to management:<owner> (records);
//   2. management's Cast to engineering:<assignment> on records, its parent
//      naming Texture's Cast;
//   3. the operation Freeze opened (the candidate bundle's effect journal),
//      reaching frozen with a bundle digest;
//   4. engineering's final Report into management:<owner> (stop_reason
//      completed), its evidence refs naming selfdev-operation:<id> and
//      bundle:<digest> equal to the operation's;
//   5. management's Verify (verifier_refs on the operation) and the owner
//      decision boundary (awaiting_approval);
//   6. the owner's approval act: pin a pre-apply checkpoint, read the head,
//      arm accept_once bound to this operation and head, POST the decision;
//   7. the apply: exactly one new boot after the approval, served by
//      /api/runtime/obligations as restart kind planned, reason
//      self_development_apply, and the operation reaching applied with its
//      materialization, checkpoint and route receipts;
//   8. a restore to the pinned pre-apply checkpoint, whose report must name
//      that checkpoint with the witness matched and the frontend restaged.
// Every phase is a timestamped row in the receipt; event heads are read at
// each boundary. No SSH and no /internal/* (standing question 9). Run
// acceptance from the fold is B3's, not this drill's.
//
// Ways this drill could falsely pass, and what closes each:
//   P1. A stale or foreign operation satisfies the polls. The computer is
//       fresh, and the operation is the one engineering's final Report names
//       (selfdev-operation:<id>); its trajectory must be the request's.
//   P2. The operation reaches applied with no restart, or through a crash
//       restart. Exactly one new boot must be observed after the approval and
//       the obligations route must serve it as kind planned, reason
//       self_development_apply. Residual: the route does not serve the
//       marker's target (the operation id), and two boots inside one poll
//       interval (3 s) look like one.
//   P3. Verify recorded against a different bundle. The reducer refuses a
//       digest mismatch; the drill also requires the digest in the final
//       Report to equal the operation's bundle digest, and verifier_refs to
//       be present when the operation reaches awaiting_approval. Verify
//       cannot precede the Report: it refuses a non-completed assignment,
//       and the row turns completed in the Report's own transaction (D3).
//   P4. The approval reached around the owner (harness ballots, an SSH mode
//       arm). The only arm is accept_once through PUT
//       /api/computers/{id}/self-development/mode, and the decision is the
//       owner's POST .../operations/{id}/decision with the same binding.
//   P5. A restore that lands somewhere else (a post-apply checkpoint), or
//       whose witness did not match. The restore report must name the pinned
//       checkpoint's digest, with witness_matched and frontend_restaged, and
//       the pinned checkpoint's effective head must be the head the approval
//       was bound to (the pre-apply head), unequal to the post-apply one.
//   P6. The final Report is a cancel or run-end closure (stop_reason
//       cancelled or run_<state>) after an earlier Freeze. stop_reason must
//       be completed.
//   P7. The served-file check passes on the SPA fallback (200 with
//       index.html). The body must equal the per-run token exactly.
//   P8. A platform deploy refreshes the computer mid-run and its restart is
//       taken for the apply's. The guest build is fenced at the decision
//       boundary (--expect-sha), and a boot's reason must be
//       self_development_apply.
//
// Ways it could falsely fail, and what avoids each:
//   F1. Engineering builds the release tree (bin/autoputer and the frontend)
//       in the capsule before Freeze, which can outlast the fast-loop
//       --timeout-min. The receipt names the last phase reached; a slow desk
//       is a finding, and the lead may raise --timeout-min.
//   F2. A fresh computer's self-development mode is off, so Freeze would be
//       refused. The drill arms propose_only before the request.
//   F3. The guest is unreachable while the apply restarts it (status 0 or
//       502). Reads during the apply and the restore retry; an unreachable
//       window is recorded, not failed.
//   F4. The event head moves between the binding read and the decision
//       (409). The binding is re-read and the decision retried, at most three
//       times, each with its own idempotency key.
//   F5. accept_once refuses a non-canonical expiry. expires_at is whole
//       seconds, UTC, with no fractional part.
//   F6. A self-development apply changes the guest's build identity on
//       purpose (CLAUDE.md "Product restore vs platform deploy"). The build
//       is fenced only up to the decision boundary; after it, the build is
//       recorded, never counted as contamination.
//   F7. The checkpoint bind or the restore is slow (Dolt reconstruction is
//       minutes on staging). The checkpoint and restore requests get the
//       proxy's replay budget (--restore-min), retried on transport errors.
//   F8. A model declines a step (Texture answers without casting, management
//       never casts engineering, engineering completes without Freeze). That
//       is a desk finding; the receipt carries the records it saw.
//   F9. The served-file path depends on how the release frontend is built
//       and served, which this drill does not control; without
//       --require-served it is observed, not claimed.
//
// Named gaps (recorded in the receipt's drill.gaps, never worked around
// through /internal/*): the restart marker's target is not served (only its
// kind and reason are); the computer's own projection head after a restore
// has no product route (corpusd's head is append-only, so a restore is a
// forward event on the chain); and any route this drill needs that answers
// 404 or 405 is recorded as a gap at the step that needed it.

// The drill's phases in order; the receipt names the last one reached.
// Under --autonomous the owner_approval phase is corpusd's consensus_decision.
const REACHED = ['mint', 'mode', 'story_submitted', 'texture_cast_to_management', 'management_cast_engineering', 'operation_frozen',
  'engineering_final_report', 'decision_boundary', AUTONOMOUS ? 'consensus_decision' : 'owner_approval', 'applied', 'restore'];
const OP_SETTLED = new Set(['applied', 'rejected', 'rolled_back', 'failed', 'degraded']);
const OP_ORDER = ['requested', 'executing', 'frozen', 'verified', 'awaiting_approval', 'accepted', 'materializing', 'applied'];
const opAtLeast = (state, floor) => OP_ORDER.indexOf(state) >= OP_ORDER.indexOf(floor);

function engineeringStory(token) {
  if (CHANGE === 'go-served') {
    const pkg = 'internal/selfdevprobe';
    return [
      `Change this computer itself: add a guest HTTP route GET ${PROBE_ROUTE} that answers 200 with the JSON object {"nonce":"${token}"}.`,
      `Add one Go package ${pkg} to the platform workspace: ${pkg}/selfdevprobe.go declares package selfdevprobe, const Nonce = "${token}",`,
      'and func Handle(w http.ResponseWriter, r *http.Request) that writes {"nonce": Nonce} as application/json (405 for any method but GET);',
      `${pkg}/selfdevprobe_test.go is a test that fails unless Handle answers 200 with that nonce.`,
      `Register the route in internal/apihandler/routes.go next to /api/runtime/observability: s.HandleFunc("${PROBE_ROUTE}", selfdevprobe.Handle).`,
      'Change nothing else. Deterministic checks, to run in the capsule before freezing, from /workspace/platform:',
      `go test ./${pkg}/ and go vet ./internal/apihandler/.`,
      'This change touches Go, so the release tree needs bin/autoputer built from the workspace source as well as the frontend.',
      'Only engineering can change this computer: Cast this request to management verbatim in your first cell, with a one-paragraph plan revision,',
      'and incorporate management\'s answer when it arrives.',
      'Management: Cast it to engineering; engineering makes the change, runs the check, builds the release tree, freezes the candidate with choir.Freeze and completes;',
      'then Verify the frozen bundle that engineering\'s final Report names, and answer this request.',
    ].join(' ');
  }
  if (CHANGE === 'rewrite') {
    const file = `frontend/src/lib/${APP}.svelte`;
    const delegate = DELEGATE ? [
      'This takes one engineering round.',
      `Management's Cast leaves the spec's "network" field out, so the capsule is in the claude mode and the Claude delegate is available; engineering saves a copy of the file, writes the rewrite through the delegate (choir-delegate start with a brief naming the absolute path /workspace/platform/${file}), and reviews the diff against the copy.`,
      'Then, in the same round, engineering runs the frontend build job (with its release copy): package.json and package-lock.json are unchanged, so it links this computer\'s locked dependencies and needs no network. It records the toolchain, Freezes, and Completes with the frozen bundle. Once those checks pass, the round freezes; it does not stop to ask for provenance.',
    ] : [];
    return [
      `Change this computer itself: rewrite ${file} in the platform workspace for clarity, accessibility (labels, keyboard focus, alt text), and maintainability.`,
      'Keep its props, events, exported API, routes and visible behaviour unchanged. Touch only that file (and a small helper module next to it if clearly needed). Change nothing else.',
      'Deterministic checks, to run in the capsule before freezing, from /workspace/platform/frontend:',
      `the frontend build job (which also copies dist into the release tree) must end done with exit code 0; and diff -u against a copy of ${file} saved before the change must show it changed: diff -u exits 1 when the file changed, and that exit 1 is the passing result.`,
      'The capsule has no .git: git diff and git status cannot run there, and a missing .git is expected, never a blocker.',
      'This change touches the frontend only, so the release tree needs the frontend release tree, plus bin/autoputer built from the workspace source only if Freeze refuses the running binary because it was built from another commit.',
      ...delegate,
      'Only engineering can change this computer: Cast this request to management verbatim in your first cell, with a one-paragraph plan revision,',
      'and incorporate management\'s answer when it arrives.',
      'Management: Cast it to engineering; engineering makes the change, runs the checks, builds the release tree, freezes the candidate with choir.Freeze and completes;',
      'then Verify the frozen bundle that engineering\'s final Report names, and answer this request.',
    ].join(' ');
  }
  if (CHANGE === 'go') {
    const pkg = 'internal/drillmark';
    return [
      `Change this computer itself: add one Go package ${pkg} to the platform workspace, with two files.`,
      `${pkg}/drillmark.go declares package drillmark and const Token = "${token}". ${pkg}/drillmark_test.go is a test that fails unless Token equals "${token}".`,
      'Change nothing else. Deterministic check, to run in the capsule before freezing: go test ./internal/drillmark/ from /workspace/platform.',
      'This change touches Go, so the release tree needs bin/autoputer built from the workspace source as well as the frontend.',
      'Only engineering can change this computer: Cast this request to management verbatim in your first cell, with a one-paragraph plan revision,',
      'and incorporate management\'s answer when it arrives.',
      'Management: Cast it to engineering; engineering makes the change, runs the check, builds the release tree, freezes the candidate with choir.Freeze and completes;',
      'then Verify the frozen bundle that engineering\'s final Report names, and answer this request.',
    ].join(' ');
  }
  if (VISUAL) {
    return [
      `Change this computer itself: add one visible page to its web frontend so that it is served at /choir-drill/${token}.html.`,
      `The file is frontend/public/choir-drill/${token}.html in the platform workspace: a complete HTML document with a dark background (#102030)`,
      `and one large white <h1> (at least 64px) whose text is exactly ${token}. Change nothing else.`,
      'Deterministic check, to run in the capsule before freezing:',
      `grep -q '<h1' frontend/public/choir-drill/${token}.html and grep -q '${token}' frontend/public/choir-drill/${token}.html.`,
      'The candidate must carry a screenshot of that page: after the frontend build, take it with the capsule capture tool',
      `(choir-capture --route /choir-drill/${token}.html, started as a capsule job), and record a visual_captured precommit for that route before you build.`,
      'Only engineering can change this computer: Cast this request to management verbatim in your first cell, with a one-paragraph plan revision,',
      'and incorporate management\'s answer when it arrives.',
      'Management: Cast it to engineering; engineering makes the change, runs the check, builds the release tree, takes the screenshot, freezes the candidate with choir.Freeze and completes;',
      'then Verify the frozen bundle that engineering\'s final Report names, and answer this request.',
    ].join(' ');
  }
  return [
    `Change this computer itself: add one static file to its web frontend so that it is served at /choir-drill/${token}.txt.`,
    `The file is frontend/public/choir-drill/${token}.txt in the platform workspace and contains exactly one line: ${token}`,
    'Change nothing else. Deterministic check, to run in the capsule before freezing:',
    `test "$(cat frontend/public/choir-drill/${token}.txt)" = "${token}".`,
    'Only engineering can change this computer: Cast this request to management verbatim in your first cell, with a one-paragraph plan revision,',
    'and incorporate management\'s answer when it arrives.',
    'Management: Cast it to engineering; engineering makes the change, runs the check, builds the release tree, freezes the candidate with choir.Freeze and completes;',
    'then Verify the frozen bundle that engineering\'s final Report names, and answer this request.',
  ].join(' ');
}

// pickHead keeps the event-head fields a drill compares.
function pickHead(h) {
  if (!h || typeof h !== 'object') return null;
  return {
    sequence: h.sequence ?? null, canonical_event_head: h.canonical_event_head || null,
    desired_event_head: h.desired_event_head || null, effective_event_head: h.effective_event_head || null,
    pending_transition_ref: h.pending_transition_ref ?? null,
    desired_state_commitment: h.desired_state_commitment || null, effective_state_commitment: h.effective_state_commitment || null,
  };
}

// The boot a guest is in, from its observability log ring: a new process
// starts a new ring, so its first line changes and (unless the old ring had
// not wrapped) the ring gets shorter.
const bootSig = (obs) => (obs?.status === 200 && Array.isArray(obs.body?.recent_log)
  ? { first: String(obs.body.recent_log[0] || ''), len: obs.body.recent_log.length } : null);
const isNewBoot = (prev, cur) => Boolean(prev && cur && cur.first !== prev.first && (cur.len < 512 || cur.len < prev.len));
const CRASH_LINE = /crash boot|treating boot as a crash/;

// canonicalExpiry is a whole-second UTC instant in Go's RFC3339Nano form,
// which accept_once requires (no fractional part, Z).
const canonicalExpiry = (ms) => new Date(Math.floor(ms / 1000) * 1000).toISOString().replace(/\.000Z$/, 'Z');

// visualEvidenceStep reads the frozen candidate's screenshots through product
// routes and records what it saw in claim.visual (the checks are
// visualEvidenceChecks'). The PNG is kept in the trace directory.
async function visualEvidenceStep(c, sd, op, claim, token, phase, gap) {
  const visual = { listing: null, asset: null, range: null, vision: null, png_file: null };
  claim.visual = visual;
  const listing = await c.api(sd(`operations/${encodeURIComponent(op.operation_id)}/visual-evidence`), 'GET', undefined, 30_000);
  visual.listing = { status: listing.status, body: listing.body };
  phase('visual_listing', { status: listing.status, captures: listing.body?.captures?.length ?? 0, capture_receipt: listing.body?.capture_receipt || null });
  if (listing.status !== 200) {
    gap('visual', 'GET /api/computers/{id}/self-development/operations/{op}/visual-evidence', `status ${listing.status}: ${listing.body?.error || listing.error || 'no body'}`);
    return;
  }
  const capture = (listing.body.captures || []).find((x) => x.route === `/choir-drill/${token}.html`);
  if (!capture?.asset_url) return;
  // The receipt records sizes and digests; the bytes stay readable for the
  // checks but out of the JSON (a non-enumerable property).
  const keep = (res) => {
    const row = { status: res.status, headers: res.headers, length: res.bytes.length, sha256: createHash('sha256').update(res.bytes).digest('hex') };
    Object.defineProperty(row, 'bytes', { value: res.bytes, enumerable: false });
    return row;
  };
  const asset = await c.bytes(capture.asset_url);
  visual.asset = keep(asset);
  const range = await c.bytes(capture.asset_url, { Range: 'bytes=0-7' });
  visual.range = keep(range);
  if (asset.status === 200) {
    visual.png_file = join(TRACE_DIR, `visual-${token}.png`);
    writePrivate(visual.png_file, asset.bytes);
  }
  // Observed, not claimed: what a vision model reads off the picture.
  const vision = await c.api('/api/model-policy/verify', 'POST', {
    role: 'verifier_multimodal', image_ref: capture.artifact,
    prompt: 'This is a screenshot of a web page. Reply with only the large heading text on it, exactly as written.',
  }, 120_000);
  visual.vision = {
    status: vision.status, error: vision.body?.error || vision.error || null,
    reading_names_token: JSON.stringify(vision.body || '').includes(token),
  };
  phase('visual_asset', { asset_status: asset.status, range_status: range.status, png_file: visual.png_file, vision: visual.vision });
}

async function engineeringScenario(c, out, n, deadline, phase, fenceBuild, options = {}) {
  const cid = encodeURIComponent(c.computerID);
  const sd = (rest) => `/api/computers/${cid}/self-development/${rest}`;
  const lc = (rest) => `/api/computers/${cid}/lifecycle/${rest}`;
  const token = `choir-drill-${randomUUID().slice(0, 8)}`;
  const markerPath = `/choir-drill/${token}.${VISUAL ? 'html' : 'txt'}`;
  const gaps = [];
  const gap = (step, route, detail) => {
    if (gaps.some((g) => g.step === step && g.route === route && g.detail === detail)) return;
    gaps.push({ step, route, detail });
    phase('gap', { step, route, detail });
  };
  const claim = {
    mode_propose_only: null, texture_cast_to_management: null, texture_casts_to_management: [], engineering_casts: [], engineering_final_reports: [],
    operation: null, operation_states: [], verify: null, decision_boundary: null,
    pre_apply_checkpoint: null, owner_approval: null, boots: [], unreachable: [], applied: null, route_receipt: null,
    standing_arm: null, consensus_decision: null, decision_calls: 0,
    restore: null, served: {}, probe: {}, witness: {}, heads: {}, obligations: {}, census: null, management_final_report_to_texture: null,
  };
  claim.token = token;
  out.drill = { label: out.label, scenario: 'engineering', change: CHANGE, ...(REWRITE ? { app: APP, delegate: DELEGATE } : {}), token, claim, gaps, note: [] };
  let reached = 'mint';
  const at = (name) => { if (REACHED.indexOf(name) > REACHED.indexOf(reached)) reached = name; };
  const readHead = async (tag) => {
    const r = await c.api(sd('head'), 'GET', undefined, 30_000);
    claim.heads[tag] = r.status === 200 ? { at: iso(), ...pickHead(r.body) } : { at: iso(), status: r.status, error: r.body?.error || r.error || null };
    return r.status === 200 ? r.body : null;
  };
  const readObligations = async (tag) => {
    const o = await obligations(c);
    claim.obligations[tag] = o?.unavailable ? o : { at: iso(), restart: o?.restart || null, runs: o?.runs || null, wakes_unprojected: o?.wakes?.unprojected ?? null };
    return o;
  };
  const served = async (tag) => {
    if (REWRITE) return; // the rewrite serves no marker file; the build and the diff are the evidence
    const r = await c.surface(markerPath);
    claim.served[tag] = { at: iso(), status: r.status, served: r.status === 200 && r.text.trim() === token, error: r.error || null };
    phase('served_check', { tag, ...claim.served[tag] });
  };
  // probe (go-served): the new guest route and the effective release, read
  // through product routes. served means 200 with this run's nonce; layered
  // means the running release is a layered one whose code_commit is the
  // serving build's commit (the host-built binary is the one answering).
  const probe = async (tag) => {
    const r = await c.api(PROBE_ROUTE, 'GET', undefined, 20_000);
    const obs = await c.api('/api/runtime/observability', 'GET', undefined, 15_000);
    const eff = obs.body?.effective_release || null;
    const build = obs.body?.build?.commit || null;
    // P6 block (live manifest `current`): layered and code_commit come from
    // the manifest, build_commit from the running binary; the same meaning as
    // P5's startup block, but a restore flips layered when `current` is
    // restaged, not at the restart. Falls back to build.commit on an older guest.
    const serving = eff?.build_commit || build;
    claim.probe[tag] = {
      at: iso(), status: r.status, served: r.status === 200 && r.body?.nonce === token, error: r.body?.error || r.error || null,
      build, effective_release: eff, layered_serving: Boolean(eff?.layered && eff.code_commit && eff.code_commit === serving),
    };
    phase('probe_check', { tag, ...claim.probe[tag] });
    return claim.probe[tag];
  };
  // witness (go-served): the release witness for the operation, a read of the
  // joined release identity (S6-0). Stored whole in the receipt; the
  // acceptance check `witness_joined` reads it and prints the gap table. A
  // failed fetch is kept (status, error) and fails that check, nothing else.
  // retryMs retries only a transport or 5xx answer (the restore restarts the
  // runtime a few seconds after it answers).
  const witness = async (tag, opID, retryMs = 0) => {
    if (CHANGE !== 'go-served' || !opID) return null;
    const until = Date.now() + retryMs;
    let r;
    for (;;) {
      r = await c.api(sd(`operations/${encodeURIComponent(opID)}/release-witness`), 'GET', undefined, 60_000);
      if (r.status === 200 || ![0, 502, 503, 504].includes(r.status) || Date.now() >= until) break;
      await sleep(5_000);
    }
    claim.witness[tag] = {
      at: iso(), status: r.status, operation_id: opID, operation_state: r.status === 200 ? (r.body?.operation?.state || null) : null,
      error: r.status === 200 ? null : (r.body?.error || r.error || null), body: r.status === 200 ? r.body : null,
    };
    phase('witness', { tag, status: r.status, operation_state: claim.witness[tag].operation_state, read_consistent: r.body?.read_consistent ?? null, summary: r.body?.summary || null });
    return claim.witness[tag];
  };
  const inbox = async (id) => {
    const r = await c.api(`/api/inboxes/${encodeURIComponent(id)}/records?limit=1000`);
    return r.status === 200 ? r.body : null;
  };
  const getOp = async (id) => {
    const r = await c.api(sd(`operations/${encodeURIComponent(id)}`), 'GET', undefined, 30_000);
    return r.status === 200 ? r.body : null;
  };
  const seenStates = new Set();
  const noteOpState = (op) => {
    if (!op?.state || seenStates.has(op.state)) return;
    seenStates.add(op.state);
    const row = { operation_id: op.operation_id || null, state: op.state, at: iso(), updated_at: op.updated_at || null, bundle_digest: op.bundle_digest || null, verifier_refs: op.verifier_refs || [], last_intent_error: op.last_intent_error || null, error: op.error || null };
    claim.operation_states.push(row);
    phase('operation_state', { operation_id: op.operation_id, ...row });
  };
  const finish = () => acceptEngineering(out, claim, gaps, reached, phase);

  // Event head before anything; a pre-genesis chain is bootstrapped through
  // the owner's lifecycle route.
  if (!(await readHead('start'))) {
    const boot = await c.api(lc('bootstrap-chain'), 'POST', {}, 120_000);
    phase('bootstrap_chain', { status: boot.status, error: boot.body?.error || boot.error || null });
    if (!(await readHead('start'))) gap('start', 'GET /api/computers/{id}/self-development/head', `no event head (${claim.heads.start?.status})`);
  }
  await readObligations('start');

  // F2: arm propose_only, the owner's act that lets Freeze open a candidate.
  const mode = await c.api(sd('mode'));
  if (mode.status !== 200) {
    gap('mode', 'GET /api/computers/{id}/self-development/mode', `status ${mode.status}`);
    return finish();
  }
  if (AUTONOMOUS) {
    // P4: the drill's one owner act. standing_consensus is a proposing
    // mode too, so Freeze opens the candidate under it.
    const armed = await c.api(sd('mode'), 'PUT', {
      mode: 'standing_consensus', expected_generation: mode.body?.generation ?? 0, idempotency_key: `${out.label}-standing-${randomUUID()}`,
      policy_digest: REVERSIBLE_SELFDEV_V1, expires_at: canonicalExpiry(Date.now() + AUTONOMY_MIN * 60_000),
    });
    claim.standing_arm = {
      at: iso(), status: armed.status, from: mode.body?.mode || null, mode: armed.body?.mode || null, generation: armed.body?.generation ?? null,
      expires_at: armed.body?.expires_at || null, computer_tier: armed.body?.receipt?.computer_tier || null,
      owner_act_receipt_digest: armed.body?.receipt?.owner_act_receipt_digest || null, error: armed.body?.error || armed.error || null,
    };
    claim.mode_propose_only = { ...claim.standing_arm, armed_by_drill: true };
    if (armed.status !== 200 || armed.body?.mode !== 'standing_consensus') {
      phase('standing_arm_refused', claim.standing_arm);
      return finish();
    }
    phase('standing_arm', claim.standing_arm);
  } else if (['propose_only', 'accept_once', 'qualified_consensus'].includes(mode.body?.mode)) {
    claim.mode_propose_only = { at: iso(), mode: mode.body.mode, generation: mode.body.generation, armed_by_drill: false };
  } else {
    const armed = await c.api(sd('mode'), 'PUT', { mode: 'propose_only', expected_generation: mode.body?.generation ?? 0, idempotency_key: `${out.label}-propose-only-${randomUUID()}` });
    claim.mode_propose_only = { at: iso(), status: armed.status, from: mode.body?.mode || null, mode: armed.body?.mode || null, generation: armed.body?.generation ?? null, armed_by_drill: true, error: armed.body?.error || armed.error || null };
    if (armed.status !== 200 || armed.body?.mode !== 'propose_only') {
      phase('mode_propose_only_refused', claim.mode_propose_only);
      return finish();
    }
  }
  phase('mode_propose_only', claim.mode_propose_only);
  at('mode');
  await served('pre_apply');
  if (CHANGE === 'go-served') await probe('pre_apply');
  // The computer binds checkpoints itself (POST /lifecycle/checkpoint is
  // deleted: docs/design/apply-checkpoint-without-replay-2026-10-26.md, 4.4);
  // the restore target is the newest it has bound, at or before this head.
  const pinPreApplyCheckpoint = async () => {
    const { checkpoint, row, error } = await pinNewestCheckpoint((p) => c.api(p, 'GET', undefined, RESTORE_MIN * 60_000), lc);
    claim.pre_apply_checkpoint = {
      at: iso(), status: checkpoint ? 200 : null, checkpoint_digest: checkpoint?.checkpoint_digest || row?.checkpoint_digest || null,
      accepted_event_head: checkpoint?.request?.accepted_event_head || null, effective_event_head: checkpoint?.request?.effective_event_head || null,
      verdict: row?.verdict?.status || null, error: error || null,
    };
    phase('pre_apply_checkpoint', claim.pre_apply_checkpoint);
    if (!checkpoint) gap('pre_apply_checkpoint', 'GET /api/computers/{id}/lifecycle/checkpoints', error);
    return checkpoint;
  };
  // --autonomous: corpusd decides as soon as Verify passes, so the restore
  // target and the boot baseline are taken before the request.
  const autonomousPinned = AUTONOMOUS ? await pinPreApplyCheckpoint() : null;
  const autonomousBootPrev = AUTONOMOUS ? bootSig(await c.observability()) : null;

  const story = await submitPrompt(c, engineeringStory(token), 'engineering', deadline);
  out.trajectory_id = story.body.trajectory_id;
  out.doc_id = story.body.doc_id;
  phase('story_submitted', { trajectory_id: out.trajectory_id, doc_id: out.doc_id, token });
  at('story_submitted');
  const textureInbox = `texture:${out.doc_id}`;
  let ownerID = c.ownerID;
  const mgmtInbox = () => `management:${ownerID}`;

  // Through the owner-decision boundary, bounded by --timeout-min.
  const knownAssignments = new Set();
  let opID = null;
  let op = null;
  while (Date.now() < deadline) {
    if (!ownerID) {
      const runs = await c.api('/api/runs?limit=50');
      ownerID = (runs.body?.runs || []).find((r) => r.owner_id)?.owner_id || '';
      if (!ownerID) { await sleep(5_000); continue; }
    }
    const m = await inbox(mgmtInbox());
    // Every Texture Cast to management: a story whose engineering fails is
    // re-cast in later rounds, and management's engineering Cast names the
    // round it answers.
    for (const cast of (m?.records || []).filter((r) => r.sender_inbox === textureInbox && r.directive?.subtype === 'cast')) {
      if (claim.texture_casts_to_management.some((x) => x.record_id === cast.record_id)) continue;
      const row = { record_id: cast.record_id, round: cast.round, seq: cast.inbox_seq, at: cast.provenance?.committed_at || null };
      claim.texture_casts_to_management.push(row);
      if (!claim.texture_cast_to_management) {
        claim.texture_cast_to_management = row;
        phase('texture_cast_to_management', row);
        at('texture_cast_to_management');
      } else {
        phase('texture_recast_to_management', row);
      }
    }
    // Engineering runs are named engineering:<assignment> (D1); the Cast that
    // opened each is in that inbox, from management:<owner>.
    const runs = await c.api('/api/runs?limit=200');
    for (const r of runs.body?.runs || []) {
      const agent = String(r.agent_id || '');
      if (!agent.startsWith('engineering:') || knownAssignments.has(agent)) continue;
      const e = await inbox(agent);
      const cast = (e?.records || []).find((x) => x.sender_inbox === mgmtInbox() && x.directive?.subtype === 'cast');
      if (!cast) continue;
      knownAssignments.add(agent);
      const row = {
        assignment_id: agent.slice('engineering:'.length), inbox: agent, record_id: cast.record_id, round: cast.round,
        parent_id: cast.parent_id || null, run_id: runID(r), run_state: r.state || null, at: cast.provenance?.committed_at || null,
      };
      claim.engineering_casts.push(row);
      phase('management_cast_engineering', row);
      at('management_cast_engineering');
    }
    for (const rec of m?.records || []) {
      if (!String(rec.sender_inbox || '').startsWith('engineering:') || !rec.final) continue;
      if (claim.engineering_final_reports.some((x) => x.record_id === rec.record_id)) continue;
      const refs = rec.evidence_refs || [];
      const row = {
        record_id: rec.record_id, sender: rec.sender_inbox, seq: rec.inbox_seq, round: rec.round, stop_reason: rec.report?.stop_reason || null,
        operation_id: (refs.find((x) => x.startsWith('selfdev-operation:')) || '').slice('selfdev-operation:'.length) || null,
        bundle_digest: (refs.find((x) => x.startsWith('bundle:')) || '').slice('bundle:'.length) || null,
        evidence_refs: refs, at: rec.provenance?.committed_at || null,
      };
      claim.engineering_final_reports.push(row);
      phase('engineering_final_report', row);
      at('engineering_final_report');
    }
    // P1: the operation is the one the final Report names; until a Report
    // names one, the newest on this request's trajectory is watched.
    const named = claim.engineering_final_reports.find((x) => x.stop_reason === 'completed' && x.operation_id);
    if (named) opID = named.operation_id;
    if (!opID) {
      const list = await c.api(sd('operations'));
      if (list.status === 404 || list.status === 405) gap('operations', 'GET /api/computers/{id}/self-development/operations', `status ${list.status}`);
      const ours = (list.body?.operations || []).find((x) => x.trajectory_id === out.trajectory_id);
      if (ours) op = ours;
    } else {
      op = (await getOp(opID)) || op;
    }
    if (op) {
      noteOpState(op);
      if (!claim.operation) phase('operation_opened', { operation_id: op.operation_id, trajectory_id: op.trajectory_id });
      claim.operation = { operation_id: op.operation_id, trajectory_id: op.trajectory_id, trajectory_matches: op.trajectory_id === out.trajectory_id, capsule_id: op.capsule_id || null, base_head: op.base_head || null, bundle_digest: op.bundle_digest || null, state: op.state };
      if (opAtLeast(op.state, 'frozen')) at('operation_frozen');
      if (OP_SETTLED.has(op.state)) break;
      // Verify needs a completed assignment, whose final Report commits with
      // the row (D3), so a Report exists by now; acceptance checks it names
      // this operation.
      if (op.state === 'awaiting_approval' && claim.engineering_final_reports.length > 0) break;
      // --autonomous: corpusd may decide between two polls.
      if (AUTONOMOUS && ['accepted', 'materializing', 'applied'].includes(op.state) && claim.engineering_final_reports.length > 0) break;
    }
    await sleep(5_000);
  }
  // Management's answer to Texture (observed; it may come after the decision).
  const t = await inbox(textureInbox);
  const answer = (t?.records || []).find((r) => r.sender_inbox === mgmtInbox() && r.final);
  if (answer) claim.management_final_report_to_texture = { record_id: answer.record_id, stop_reason: answer.report?.stop_reason || null, at: answer.provenance?.committed_at || null };
  const pastBoundary = AUTONOMOUS && ['accepted', 'materializing', 'applied'].includes(op?.state);
  if (!op || (op.state !== 'awaiting_approval' && !pastBoundary)) {
    phase('decision_boundary_not_reached', { state: op?.state || null, last_intent_error: op?.last_intent_error || null });
    await stallSnapshot(c, n, phase);
    return finish();
  }
  claim.verify = { verifier_refs: op.verifier_refs || [], bundle_digest: op.bundle_digest, last_intent_error: op.last_intent_error || null };
  claim.decision_boundary = { at: iso(), operation_id: op.operation_id };
  await witness('frozen', op.operation_id);
  // Capsule jobs (--change go): each engineering inbox holds one job record
  // per job that ended, from job:<id>, with its state and receipt. The
  // bundle's jobs are the completing assignment's; earlier assignments'
  // jobs are kept for the record.
  claim.jobs_by_inbox = {};
  for (const cast of claim.engineering_casts) {
    const e = await inbox(cast.inbox);
    if (!e) gap('jobs', 'GET /api/inboxes/{id}/records', `engineering inbox ${cast.inbox} unreadable`);
    claim.jobs_by_inbox[cast.inbox] = (e?.records || []).filter((r) => r.job).map((r) => ({
      job_id: r.job.job_id, state: r.job.state, exit_code: r.job.exit_code ?? null, argv: r.job.argv || [], receipt_ref: r.job.receipt_ref || null,
      started_at: r.job.started_at || null, finished_at: r.job.finished_at || null,
    }));
  }
  const jobsInbox = completingInbox(claim, claim.engineering_final_reports.find((x) => x.stop_reason === 'completed' && x.operation_id) || null);
  if (jobsInbox) {
    claim.jobs_inbox = jobsInbox;
    claim.jobs = claim.jobs_by_inbox[jobsInbox] || [];
    phase('jobs', { inbox: jobsInbox, count: claim.jobs.length, done: claim.jobs.filter((j) => j.state === 'done').length });
    if (GO_CHANGE) {
      // The requested go test may run as a check cell, not a job; Freeze
      // admits either. Read the completing assignment's run events for it.
      const { ofTrajectory } = await trajectoryRuns(c, out.trajectory_id, '');
      const events = [];
      for (const r of ofTrajectory.filter((x) => (x.agent_id || x.metadata?.agent_id) === jobsInbox)) {
        const got = await runEvents(c, runID(r));
        if (got === null) gap('jobs', 'GET /api/runs/{id}/events', `engineering run ${runID(r)} events unreadable`);
        else events.push(...got);
      }
      claim.go_test_cells = goTestCells(events, jobsInbox);
      phase('go_test_cells', { inbox: jobsInbox, passed: claim.go_test_cells.length });
    }
  }
  if (VISUAL) await visualEvidenceStep(c, sd, op, claim, token, phase, gap);
  phase('decision_boundary', { operation_id: op.operation_id, bundle_digest: op.bundle_digest, verifier_refs: op.verifier_refs });
  at('decision_boundary');
  await readHead('decision_boundary');

  // P8 / F6: the guest build is fenced up to here and only recorded after.
  const preObs = await c.observability();
  claim.build_pre_apply = preObs.body?.build?.commit || null;
  // Past the boundary (--autonomous) the apply may already have changed
  // the build on purpose (F6); the fence then stops at the request.
  if (!pastBoundary) fenceBuild('decision_boundary', preObs.body?.build);
  await readObligations('pre_apply');

  // The restore target: a checkpoint pinned at the pre-apply head (under
  // --autonomous it was pinned before the request, since corpusd decides
  // as soon as Verify passes).
  const pinned = AUTONOMOUS ? autonomousPinned : await pinPreApplyCheckpoint();

  let bootPrev = AUTONOMOUS ? autonomousBootPrev : bootSig(await c.observability());
  let decided = null;
  if (AUTONOMOUS) {
    // P4: no owner act. The host builds the candidate (P5), then corpusd's
    // poster decides on that build; the drill only watches the operation
    // leave awaiting_approval and reads who decided.
    const decideDeadline = Date.now() + Math.max(APPLY_MIN, DECIDE_MIN) * 60_000;
    let cur = op;
    while (Date.now() < decideDeadline && cur && cur.state === 'awaiting_approval') {
      await sleep(3_000);
      cur = (await getOp(op.operation_id)) || cur;
    }
    if (cur) { op = cur; noteOpState(op); }
    claim.consensus_decision = {
      at: iso(), state: cur?.state || null, decision_actor: cur?.decision_actor || null, decision_event: cur?.decision_event || null,
      decision_receipt: cur?.decision_receipt || null, last_intent_error: cur?.last_intent_error || null,
      consensus_actor: /^[0-9a-f]{64}$/.test(cur?.decision_actor || ''),
    };
    phase('consensus_decision', claim.consensus_decision);
    if (cur && ['accepted', 'materializing', 'applied'].includes(cur.state) && claim.consensus_decision.consensus_actor) {
      decided = { status: 200, autonomous: true, state: cur.state };
      claim.heads.pre_apply = claim.heads.decision_boundary || claim.heads.start;
      at('consensus_decision');
    }
  }
  // The owner's approval act (P4, F4, F5), as `choir self-dev approve` does it.
  for (let tryN = 1; tryN <= 3 && !decided && !AUTONOMOUS; tryN++) {
    const cur = await getOp(op.operation_id);
    if (cur && cur.state !== 'awaiting_approval') { if (['accepted', 'materializing', 'applied'].includes(cur.state)) decided = { status: 200, recovered: true, state: cur.state }; break; }
    const head = await readHead(`binding_${tryN}`);
    if (!head) break;
    const binding = {
      bundle_digest: op.bundle_digest,
      expected_desired_event_head: head.desired_event_head, expected_effective_event_head: head.effective_event_head,
      expected_pending_transition_ref: head.pending_transition_ref || '',
      expected_desired_state_commitment: head.desired_state_commitment, expected_effective_state_commitment: head.effective_state_commitment,
    };
    const m0 = await c.api(sd('mode'));
    const arm = await c.api(sd('mode'), 'PUT', {
      mode: 'accept_once', expected_generation: m0.body?.generation ?? 0, idempotency_key: `${out.label}-accept-once-${tryN}-${op.operation_id}`,
      operation_id: op.operation_id, expires_at: canonicalExpiry(Date.now() + 15 * 60_000), ...binding,
    });
    const row = { try: tryN, at: iso(), head: pickHead(head), arm_status: arm.status, arm_generation: arm.body?.generation ?? null, arm_error: arm.body?.error || arm.error || null };
    if (arm.status !== 200 || arm.body?.mode !== 'accept_once') { claim.owner_approval = row; phase('owner_approval_arm_refused', row); break; }
    claim.decision_calls++;
    const d = await c.api(sd(`operations/${encodeURIComponent(op.operation_id)}/decision`), 'POST', {
      decision: 'approve', idempotency_key: `${out.label}-approve-${tryN}-${op.operation_id}`, verifier_ref: (op.verifier_refs || [])[0] || '', ...binding,
    }, 120_000);
    Object.assign(row, { decision_status: d.status, decision_error: d.body?.error || d.error || null, decision_state: d.body?.state || null, decision_actor: d.body?.decision_actor || null, decision_event: d.body?.decision_event || null });
    claim.owner_approval = row;
    phase('owner_approval', row);
    if (d.status === 200) { decided = d; claim.heads.pre_apply = claim.heads[`binding_${tryN}`]; break; }
    const after = await getOp(op.operation_id);
    if (after && ['accepted', 'materializing', 'applied'].includes(after.state)) { decided = { status: 200, recovered: true, state: after.state }; claim.heads.pre_apply = claim.heads[`binding_${tryN}`]; break; }
    if (d.status !== 409) break;
  }
  if (!decided) return finish();
  at('owner_approval');
  await witness('accepted', op.operation_id);

  // The apply (P2, F3): poll the operation and the guest's boot every 3 s
  // until applied or settled, then 30 s more for a second boot.
  const applyDeadline = Date.now() + APPLY_MIN * 60_000;
  let appliedAt = 0;
  let down = null;
  while (Date.now() < applyDeadline && (!appliedAt || Date.now() - appliedAt < 30_000)) {
    const obs = await c.api('/api/runtime/observability', 'GET', undefined, 15_000);
    if (obs.status !== 200) {
      if (!down) down = { from: iso(), status: obs.status };
    } else {
      if (down) { claim.unreachable.push({ ...down, to: iso() }); down = null; }
      if (obs.body?.computer_id && obs.body.computer_id !== c.computerID) throw new Error(`guest answered as ${obs.body.computer_id}, not the minted ${c.computerID}`);
      const sig = bootSig(obs);
      if (isNewBoot(bootPrev, sig)) {
        const o = await obligations(c);
        const lines = obs.body.recent_log || [];
        const row = {
          observed_at: iso(), ...bootInstant(obs.body, (claim.owner_approval || claim.standing_arm)?.at, iso()), restart_kind: o?.restart?.kind || null, restart_reason: o?.restart?.reason || null, restart_target: o?.restart?.target || null,
          crash_lines: lines.filter((l) => CRASH_LINE.test(l)).slice(0, 5), operation_state_before: op.state, build: obs.body?.build?.commit || null,
        };
        claim.boots.push(row);
        phase('boot_observed', row);
      }
      if (sig) bootPrev = sig;
    }
    const cur = await getOp(op.operation_id);
    if (cur) {
      op = cur;
      noteOpState(op);
      if (op.state === 'applied' && !appliedAt) { appliedAt = Date.now(); at('applied'); }
      if (OP_SETTLED.has(op.state) && op.state !== 'applied') break;
    }
    await sleep(3_000);
  }
  if (down) claim.unreachable.push({ ...down, to: null });
  claim.applied = { state: op.state, at: appliedAt ? iso(appliedAt) : null, error: op.error || null, effective_head: op.effective_head || null, desired_head: op.desired_head || null };
  phase('apply_settled', claim.applied);
  if (op.state === 'applied') {
    const rr = await c.api(sd(`operations/${encodeURIComponent(op.operation_id)}/receipts`));
    claim.route_receipt = rr.status === 200 ? {
      status: 200, materialization_receipt: rr.body.materialization_receipt || null, checkpoint_ref: rr.body.checkpoint_ref || null,
      route_certificate: rr.body.route_certificate || null, route_generation: rr.body.route_generation ?? null,
      decision_event: rr.body.decision_event || null, lifecycle_receipt: rr.body.lifecycle_receipt || null, terminal_state: rr.body.terminal_state || null,
    } : { status: rr.status, error: rr.body?.error || rr.error || null };
    phase('route_receipt', claim.route_receipt);
    if (rr.status === 404 || rr.status === 405) gap('route_receipt', 'GET /api/computers/{id}/self-development/operations/{op}/receipts', `status ${rr.status}`);
  }
  await readHead('post_apply');
  await readObligations('post_apply');
  const postApplyObs = (await c.api('/api/runtime/observability', 'GET', undefined, 15_000)).body;
  claim.build_post_apply = postApplyObs?.build?.commit || null;
  claim.effective_release_post_apply = postApplyObs?.effective_release || null;
  await served('post_apply');
  if (CHANGE === 'go-served') await probe('post_apply');
  await witness('applied', op.operation_id);
  const census = await c.api('/api/runtime/census', 'GET', undefined, 30_000);
  claim.census = census.status === 200 ? { counts: census.body?.counts || {}, objects: (census.body?.objects || []).length, truncated: Boolean(census.body?.truncated) } : { status: census.status };

  // The round trip's trajectory bundle, before the restore rewinds the guest.
  if (!SKIP_EXPORT && out.trajectory_id) {
    const bundlePath = join(TRACE_DIR, `bundle-a${n}.json`);
    const exported = await runExporter(c, out.trajectory_id, bundlePath);
    out.export = { command: EXPORTER, exit: exported.code, ms: exported.ms, stderr_tail: exported.stderr.slice(-600) };
    if (exported.code === 0 && existsSync(bundlePath)) {
      out.bundle_path = bundlePath;
      out.sha256 = createHash('sha256').update(readFileSync(bundlePath)).digest('hex');
    } else {
      out.export.unavailable = true; // observed only: the drill's claim decides the exit
    }
    phase('export', { exit: exported.code, ms: exported.ms });
  }

  // The restore to the pinned pre-apply checkpoint (P5, F7).
  if (options.skipRestore && op.state === 'applied') {
    // The adopt scenario publishes this operation; restoring first would
    // make the publisher's effective release the pre-apply one.
    phase('restore_skipped', { reason: options.skipRestore });
  } else if (op.state === 'applied' && pinned) {
    const restoreDeadline = Date.now() + RESTORE_MIN * 60_000;
    let r = null;
    let tries = 0;
    while (Date.now() < restoreDeadline) {
      tries++;
      r = await c.api(lc('restore'), 'POST', { checkpoint: pinned, operand_scopes: ['vm_local', 'computer_surface_frontend'] }, Math.max(60_000, restoreDeadline - Date.now()));
      if (![0, 502, 503, 504].includes(r.status)) break;
      await sleep(10_000);
    }
    claim.restore = {
      at: iso(), tries, status: r?.status ?? null, checkpoint_digest: r?.body?.checkpoint_digest || null, method: r?.body?.method || null,
      witness_matched: r?.body?.witness_matched === true, frontend_restaged: r?.body?.frontend_restaged === true,
      tail_target_sequence: r?.body?.tail_target_sequence ?? null, tail_events_applied: r?.body?.tail_events_applied ?? null,
      error: r?.body?.error || r?.error || null,
    };
    phase('restore', claim.restore);
    if (r?.status === 404 || r?.status === 405) gap('restore', 'POST /api/computers/{id}/lifecycle/restore', `status ${r.status}`);
    if (r?.status === 200) at('restore');
    await readHead('post_restore');
    const after = await getOp(op.operation_id);
    claim.restore.operation_state_after = after?.state || null;
    claim.build_post_restore = (await c.api('/api/runtime/observability', 'GET', undefined, 15_000)).body?.build?.commit || null;
    await served('post_restore');
    if (CHANGE === 'go-served') {
      // The restore restarts the runtime into the restored release a few
      // seconds after it answers (restart_scheduled); wait for the probe to
      // stop serving the nonce, through the unreachable window.
      claim.restore.exec_changed = r?.body?.exec_changed === true;
      claim.restore.restart_scheduled = r?.body?.restart_scheduled === true;
      // A busy computer defers the restart to its next idle moment.
      claim.restore.restart_deferred = r?.body?.restart_deferred || null;
      const probeDeadline = Date.now() + RESTORE_MIN * 60_000;
      let p = await probe('post_restore');
      while (Date.now() < probeDeadline && (p.served || p.status === 0 || p.status >= 500)) {
        await sleep(5_000);
        p = await probe('post_restore');
      }
      await witness('restored', op.operation_id, Math.max(0, probeDeadline - Date.now()));
    }
    gap('restore', 'none', 'no product route serves the computer\'s projection head after a restore; corpusd\'s head is append-only (a restore is a forward event), so the landing is read from the restore report\'s checkpoint binding');
  } else if (op.state === 'applied') {
    gap('restore', 'POST /api/computers/{id}/lifecycle/restore', 'not attempted: no pre-apply checkpoint to restore to');
  }
  if (claim.boots.length && !claim.boots.some((b) => b.restart_target)) gap('apply_restart', 'GET /api/runtime/obligations', 'the restart marker\'s target (the operation id) is not served; only its kind and reason are');
  return finish();
}

// ---- adopt scenario (Station B minimal, P8) --------------------------------
//
// adoptScenario proves source adoption onto the adopter's own tree
// (docs/reports/p6-p8-release-adopt-2026-10-15.md, "Station B tracer probe"):
//   1. computer A (this attempt's mint) runs the engineering drill (static
//      change) to applied, with no restore;
//   2. A's owner publishes the applied operation as a release R
//      (POST /api/computers/A/releases) and reads it back
//      (GET /api/releases/R);
//   3. a second fresh computer B is minted and armed propose_only;
//   4. negative legs on B, each refused with B's event head, operation list
//      and adoption list unchanged: R's record with one field altered and
//      the id kept (release_digest_mismatch), and a self-consistent record
//      that was never published (A's draft with another test command;
//      release_not_published);
//   5. B adopts R (POST .../self-development/adoptions), a repeat names the
//      same adoption, B's desks carry it to awaiting_approval, B's owner
//      approves, B applies.
// Accepted when B's adoption names R and B's trajectory, B's frozen patch is
// R's (same base), B applied through a planned self_development_apply
// restart, B serves A's token file, and B's effective_release names B's
// operation with adopted_release_id R and R's tree hash.
//
// Ways it could falsely pass, and what closes each:
//   Q1. B's operation is a stale or unrelated one: it must be the operation
//       on the adoption's trajectory, read from the adoption itself.
//   Q2. B serves the token because A's bytes reached B: nothing but the
//       release record and the patch crosses (the guest stages only those),
//       and B's frozen patch is B's own diff (its sha equals R's only
//       because the bases and the change are equal).
//   Q3. A refusal changed state anyway: head, operations and adoptions are
//       read before and after each negative leg.
// Ways it could falsely fail: the desks decline or are slow (F1, F8 of the
// engineering drill); the receipt names the last phase reached.

const ADOPT_REACHED = ['publisher_applied', 'published', 'adopter_minted', 'adopter_armed', 'tamper_refused', 'unpublished_refused',
  'adoption_requested', 'adopter_awaiting_approval', 'adopter_approved', 'adopter_applied'];

async function adoptScenario(cA, out, n, deadline, phase, fenceBuild, label) {
  const claim = { publisher: null, release: null, negative: {}, adoption: null, adoption_repeat: null, adopter_states: [], approval: null, applied: null, served: null, identity: null, restart: null };
  out.drill = { label: out.label, scenario: 'adopt', claim, note: [] };
  let reached = null;
  const at = (name) => { if (ADOPT_REACHED.indexOf(name) > ADOPT_REACHED.indexOf(reached)) reached = name; };
  const finish = () => acceptAdopt(out, claim, reached, phase);

  // 1. A runs the engineering drill to applied, without the restore.
  const pubOut = { label: `${out.label}-publisher`, phases: out.phases, timings: {} };
  await engineeringScenario(cA, pubOut, n, deadline, phase, fenceBuild, { skipRestore: 'adopt publishes this operation' });
  const pubClaim = pubOut.drill_claim || {};
  const opA = pubClaim.operation?.operation_id || null;
  claim.publisher = { computer_id: cA.computerID, operation_id: opA, state: pubClaim.applied?.state || null, token: pubClaim.token || null, reached: pubOut.acceptance?.reached || null };
  phase('publisher', claim.publisher);
  if (claim.publisher.state !== 'applied' || !opA) return finish();
  at('publisher_applied');
  const token = pubClaim.token;
  const markerPath = `/choir-drill/${token}.txt`;
  const testCommand = `test "$(cat frontend/public/choir-drill/${token}.txt)" = "${token}"`;

  // 2. A's owner publishes; the release reads back verified by the CLI's rule.
  const pub = await cA.api(`/api/computers/${encodeURIComponent(cA.computerID)}/releases`, 'POST', { operation_id: opA, test_command: testCommand }, 120_000);
  const releaseID = pub.body?.release_id || null;
  claim.release = { status: pub.status, release_id: releaseID, published: pub.body?.published ?? null, error: pub.body?.error || pub.error || null };
  phase('release_published', claim.release);
  if (![200, 201].includes(pub.status) || !releaseID) return finish();
  const shown = await cA.api(`/api/releases/${releaseID}`);
  const record = shown.body?.record || null;
  Object.assign(claim.release, {
    read_status: shown.status, operation_id: record?.operation?.operation_id || null, bundle_digest: record?.operation?.bundle_digest || null,
    patch_sha256: record?.source?.patch_sha256 || null, base_commit: record?.source?.base_commit || null, tree_hash: record?.source?.tree_hash || null,
    changed_paths: record?.source?.changed_paths || [], precommitments: (record?.precommitments || []).map((p) => ({ kind: p.kind, settled: p.settled || null })),
    host_build: record?.host_build || null,
  });
  phase('release_read', claim.release);
  if (shown.status !== 200 || !record) return finish();
  at('published');

  // A self-consistent record that was never published: A's own draft with
  // another declared test command (a different release id).
  const unpublishedDraft = await cA.api(`/api/computers/${encodeURIComponent(cA.computerID)}/self-development/operations/${encodeURIComponent(opA)}/release-draft?test_command=${encodeURIComponent('true')}`);

  // 3. Mint B and arm it.
  const mintedB = await runMint(`${label}-b`);
  const cB = new MintedComputer(mintedB);
  fenceBuild('mint_b', mintedB.guest_build_commit);
  const stopA = out.stopComputer;
  out.stopComputer = async () => ({ a: await stopA?.(), b: await stopMinted(cB, `${label}-b`) });
  claim.adopter = { computer_id: cB.computerID, build: mintedB.guest_build_commit || null };
  phase('adopter_minted', claim.adopter);
  at('adopter_minted');
  const sdB = (rest) => `/api/computers/${encodeURIComponent(cB.computerID)}/self-development/${rest}`;
  let headB = await cB.api(sdB('head'), 'GET', undefined, 30_000);
  if (headB.status !== 200) {
    const boot = await cB.api(`/api/computers/${encodeURIComponent(cB.computerID)}/lifecycle/bootstrap-chain`, 'POST', {}, 120_000);
    phase('adopter_bootstrap_chain', { status: boot.status });
  }
  const modeB = await cB.api(sdB('mode'));
  if (!['propose_only', 'accept_once', 'qualified_consensus'].includes(modeB.body?.mode)) {
    const armed = await cB.api(sdB('mode'), 'PUT', { mode: 'propose_only', expected_generation: modeB.body?.generation ?? 0, idempotency_key: `${out.label}-b-propose-only-${randomUUID()}` });
    claim.adopter.mode = { status: armed.status, mode: armed.body?.mode || null, error: armed.body?.error || armed.error || null };
    phase('adopter_mode', claim.adopter.mode);
    if (armed.status !== 200) return finish();
  }
  at('adopter_armed');

  // 4. Negative legs: refused, and nothing on B moved (Q3).
  const stateOfB = async () => {
    const h = await cB.api(sdB('head'), 'GET', undefined, 30_000);
    const ops = await cB.api(sdB('operations'));
    const ads = await cB.api(sdB('adoptions'));
    return { head: h.body?.canonical_event_head || null, sequence: h.body?.sequence ?? null, operations: (ops.body?.operations || []).length, adoptions: (ads.body?.adoptions || []).length };
  };
  const negative = async (name, body, wantReason) => {
    const before = await stateOfB();
    const r = await cB.api(sdB('adoptions'), 'POST', body, 120_000);
    const after = await stateOfB();
    const row = { status: r.status, reason: r.body?.reason || null, error: r.body?.error || r.error || null, before, after,
      refused: r.status >= 400 && r.body?.reason === wantReason, unchanged: JSON.stringify(before) === JSON.stringify(after) };
    claim.negative[name] = row;
    phase(`negative_${name}`, row);
    return row.refused && row.unchanged;
  };
  const tampered = JSON.parse(JSON.stringify(record));
  tampered.declarations = { ...(tampered.declarations || {}), test_command: 'true' };
  if (await negative('tampered', { release_id: releaseID, record: tampered }, 'release_digest_mismatch')) at('tamper_refused');
  if (unpublishedDraft.status === 200 && unpublishedDraft.body?.release_id && unpublishedDraft.body.release_id !== releaseID) {
    if (await negative('unpublished', { release_id: unpublishedDraft.body.release_id, record: unpublishedDraft.body.record }, 'release_not_published')) at('unpublished_refused');
  } else {
    claim.negative.unpublished = { status: unpublishedDraft.status, error: 'no unpublished draft to offer' };
  }

  // 5. Adopt R on B.
  const adopt = await cB.api(sdB('adoptions'), 'POST', { release_id: releaseID, record }, 180_000);
  claim.adoption = { status: adopt.status, adoption_id: adopt.body?.adoption_id || null, trajectory_id: adopt.body?.trajectory_id || null,
    same_base: adopt.body?.same_base ?? null, adopter_tree_hash: adopt.body?.adopter_tree_hash || null, reason: adopt.body?.reason || null, error: adopt.body?.error || adopt.error || null };
  phase('adoption_requested', claim.adoption);
  if (adopt.status !== 201 || !claim.adoption.adoption_id) return finish();
  at('adoption_requested');
  out.trajectory_id = claim.adoption.trajectory_id;
  const repeat = await cB.api(sdB('adoptions'), 'POST', { release_id: releaseID, record }, 120_000);
  claim.adoption_repeat = { status: repeat.status, adoption_id: repeat.body?.adoption_id || null };

  // B's desks carry it to the decision boundary (Q1: the operation is the
  // adoption's own).
  const adoptionPath = sdB(`adoptions/${encodeURIComponent(claim.adoption.adoption_id)}`);
  let view = null;
  const seen = new Set();
  while (Date.now() < deadline) {
    const r = await cB.api(adoptionPath);
    view = r.status === 200 ? r.body : view;
    const st = view?.operation?.state;
    if (st && !seen.has(st)) {
      seen.add(st);
      claim.adopter_states.push({ state: st, at: iso(), operation_id: view.operation.operation_id, patch_matches_release: view.patch_matches_release });
      phase('adopter_operation_state', claim.adopter_states[claim.adopter_states.length - 1]);
    }
    if (st === 'awaiting_approval' || OP_SETTLED.has(st)) break;
    await sleep(10_000);
  }
  const opB = view?.operation?.operation_id || null;
  if (view?.operation?.state !== 'awaiting_approval' || !opB) return finish();
  at('adopter_awaiting_approval');
  claim.patch_matches_release = Boolean(view.patch_matches_release);

  // B's owner approves (accept_once bound to the operation and the head).
  let op = (await cB.api(sdB(`operations/${encodeURIComponent(opB)}`))).body;
  for (let tryN = 1; tryN <= 3; tryN++) {
    const head = (await cB.api(sdB('head'), 'GET', undefined, 30_000)).body;
    const binding = {
      bundle_digest: op.bundle_digest, expected_desired_event_head: head.desired_event_head, expected_effective_event_head: head.effective_event_head,
      expected_pending_transition_ref: head.pending_transition_ref || '', expected_desired_state_commitment: head.desired_state_commitment,
      expected_effective_state_commitment: head.effective_state_commitment,
    };
    const m0 = await cB.api(sdB('mode'));
    const arm = await cB.api(sdB('mode'), 'PUT', {
      mode: 'accept_once', expected_generation: m0.body?.generation ?? 0, idempotency_key: `${out.label}-b-accept-once-${tryN}-${opB}`,
      operation_id: opB, expires_at: canonicalExpiry(Date.now() + 15 * 60_000), ...binding,
    });
    const d = arm.status === 200 ? await cB.api(sdB(`operations/${encodeURIComponent(opB)}/decision`), 'POST', {
      decision: 'approve', idempotency_key: `${out.label}-b-approve-${tryN}-${opB}`, verifier_ref: (op.verifier_refs || [])[0] || '', ...binding,
    }, 120_000) : { status: 0 };
    claim.approval = { try: tryN, arm_status: arm.status, decision_status: d.status, error: d.body?.error || arm.body?.error || null };
    phase('adopter_approval', claim.approval);
    if (d.status === 200 || d.status === 409) {
      const cur = (await cB.api(sdB(`operations/${encodeURIComponent(opB)}`))).body;
      if (d.status === 200 || ['accepted', 'materializing', 'applied'].includes(cur?.state)) break;
    } else {
      break;
    }
  }
  if (!(claim.approval?.decision_status === 200)) return finish();
  at('adopter_approved');

  // B applies (its own updater, a planned restart).
  const applyDeadline = Date.now() + APPLY_MIN * 60_000;
  while (Date.now() < applyDeadline) {
    const cur = await cB.api(sdB(`operations/${encodeURIComponent(opB)}`), 'GET', undefined, 15_000);
    if (cur.status === 200) {
      op = cur.body;
      if (!seen.has(op.state)) {
        seen.add(op.state);
        claim.adopter_states.push({ state: op.state, at: iso(), operation_id: opB });
        phase('adopter_operation_state', { state: op.state });
      }
      if (OP_SETTLED.has(op.state)) break;
    }
    await sleep(3_000);
  }
  claim.applied = { state: op.state, error: op.error || null };
  if (op.state !== 'applied') return finish();
  at('adopter_applied');
  const o = await obligations(cB);
  claim.restart = { kind: o?.restart?.kind || null, reason: o?.restart?.reason || null, target: o?.restart?.target || null };
  // The planned restart and P6's 30 s identity retry (effectiveTreeRetry) make
  // both reads settle after `applied`: poll every 5 s for up to ~120 s until
  // the marker is served and the identity names the adoption. On timeout the
  // last observed values stay in claim.served / claim.identity.
  const identityNames = (idb) => Boolean(idb && idb.adopted_release_id === (claim.release || {}).release_id && idb.operation_id
    && (!claim.adoption?.same_base || idb.tree_hash === (claim.release || {}).tree_hash));
  const pollStart = Date.now();
  claim.adopter_poll = { attempts: 0, served_after_ms: null, identity_after_ms: null, deadline_ms: 120_000 };
  while (true) {
    claim.adopter_poll.attempts += 1;
    try {
      const s = await cB.surface(markerPath);
      claim.served = { status: s.status, served: s.status === 200 && s.text.trim() === token };
    } catch (e) {
      claim.served = { status: 0, served: false, error: String(e?.message || e) };
    }
    try {
      const obs = await cB.api('/api/runtime/observability', 'GET', undefined, 15_000);
      claim.identity = obs.body?.effective_release || null;
    } catch (e) {
      claim.identity = { error: String(e?.message || e) };
    }
    if (claim.served.served && claim.adopter_poll.served_after_ms === null) claim.adopter_poll.served_after_ms = Date.now() - pollStart;
    if (identityNames(claim.identity) && claim.adopter_poll.identity_after_ms === null) claim.adopter_poll.identity_after_ms = Date.now() - pollStart;
    if ((claim.served.served && identityNames(claim.identity)) || Date.now() - pollStart >= claim.adopter_poll.deadline_ms) break;
    await sleep(5_000);
  }
  phase('adopter_served', { ...claim.served, poll: claim.adopter_poll });
  phase('adopter_identity', claim.identity || {});
  return finish();
}

function acceptAdopt(out, claim, reached, phase) {
  const r = claim.release || {};
  const id = claim.identity || {};
  const checks = {
    publisher_applied: claim.publisher?.state === 'applied',
    release_published_and_reads_back: Boolean(r.release_id && r.read_status === 200 && r.operation_id === claim.publisher?.operation_id && r.patch_sha256),
    tampered_record_refused_unchanged: Boolean(claim.negative.tampered?.refused && claim.negative.tampered?.unchanged),
    unpublished_record_refused_unchanged: Boolean(claim.negative.unpublished?.refused && claim.negative.unpublished?.unchanged),
    adoption_names_release_and_trajectory: Boolean(claim.adoption?.status === 201 && claim.adoption.trajectory_id),
    repeat_names_same_adoption: Boolean(claim.adoption_repeat?.status === 200 && claim.adoption_repeat.adoption_id === claim.adoption?.adoption_id),
    frozen_patch_is_the_release: Boolean(claim.patch_matches_release),
    adopter_applied: claim.applied?.state === 'applied',
    planned_restart: claim.restart?.kind === 'planned' && claim.restart?.reason === 'self_development_apply',
    adopter_serves_the_change: Boolean(claim.served?.served),
    identity_names_adoption: Boolean(id.adopted_release_id === r.release_id && id.operation_id && (!claim.adoption?.same_base || id.tree_hash === r.tree_hash)),
  };
  out.acceptance = {
    scenario: 'adopt',
    claim: 'A publishes an applied change as a source release; B refuses a tampered and an unpublished record before any state change, adopts the release as source through its own desks and gate, applies it, serves it, and reports it as its effective release',
    reached, checks, failed: Object.keys(checks).filter((k) => !checks[k]),
    release_id: r.release_id || null, publisher: claim.publisher?.computer_id || null, adopter: claim.adopter?.computer_id || null,
    pass: Object.values(checks).every(Boolean),
  };
  phase('acceptance', { pass: out.acceptance.pass, reached, failed: out.acceptance.failed });
  out.drill_claim = claim;
  delete out.drill;
  return out;
}

// acceptEngineering turns what the drill saw into the claim and its verdict.
function acceptEngineering(out, claim, gaps, reached, phase) {
  const report = claim.engineering_final_reports.find((x) => x.stop_reason === 'completed' && x.operation_id) || null;
  const op = claim.operation;
  const cast = claim.engineering_casts.find((x) => report && x.inbox === report.sender) || claim.engineering_casts[0] || null;
  const boot = claim.boots.length === 1 ? claim.boots[0] : null;
  const preHead = claim.heads.pre_apply || null;
  const rr = claim.route_receipt || {};
  const checks = {
    texture_cast_to_management: Boolean(claim.texture_cast_to_management),
    // A Cast that answers any Texture round is on records (a failed
    // assignment is re-cast in a later round).
    management_cast_engineering_on_records: castOnRecords(claim, cast),
    frozen: claim.operation_states.some((s) => opAtLeast(s.state, 'frozen') && s.bundle_digest),
    // The Report names the digest Freeze gave the operation; Verify then
    // finalizes the bundle under a new digest (finalizeVerifiedCapsuleBundle).
    final_report_names_bundle: reportNamesBundle(claim, report, op),
    operation_on_request_trajectory: Boolean(op?.trajectory_matches),
    verified_against_named_bundle: verifiedNamedBundle(claim, report, op),
    owner_approved: Boolean(claim.owner_approval && (claim.owner_approval.decision_status === 200 || ['accepted', 'materializing', 'applied'].includes(claim.applied?.state))),
    applied: claim.applied?.state === 'applied',
    // A served marker target must name this operation (older guests serve none).
    one_planned_restart: Boolean(boot && boot.restart_kind === 'planned' && boot.restart_reason === 'self_development_apply' && boot.crash_lines.length === 0
      && (!boot.restart_target || boot.restart_target === op?.operation_id)),
    route_receipt: Boolean(rr.materialization_receipt && rr.checkpoint_ref && rr.route_certificate),
    // The pinned checkpoint is the newest the computer had bound, so its head
    // is at or before the pre-apply head, not necessarily equal to it.
    restore_to_pre_apply_head: restoredToPinnedCheckpoint(claim) && Boolean(preHead?.effective_event_head),
  };
  if (AUTONOMOUS) {
    // P4: one owner act (the standing arm, test tier), no owner_approval
    // step and no decision call from the drill; the decision's actor is a
    // consensus decision digest corpusd minted (an owner decision's actor
    // is the owner's user id).
    delete checks.owner_approved;
    checks.standing_armed_test_tier = Boolean(claim.standing_arm?.status === 200 && claim.standing_arm.computer_tier === 'test' && claim.standing_arm.owner_act_receipt_digest);
    checks.no_owner_approval_step = claim.owner_approval === null && claim.decision_calls === 0;
    checks.consensus_accepted = Boolean(claim.consensus_decision?.consensus_actor && ['accepted', 'materializing', 'applied'].includes(claim.consensus_decision.state));
    // P4 on P5: a consensus-accepted candidate runs only as the host build
    // corpusd voted on, frontend-only or not: the serving release is layered
    // and its binary is the host-built one.
    const eff = claim.effective_release_post_apply;
    checks.consensus_applied_host_build = Boolean(eff?.layered && eff.code_commit && eff.code_commit === claim.build_post_apply);
    checks.restore_to_pre_apply_head = Boolean(claim.restore?.status === 200 && claim.restore.witness_matched && claim.restore.frontend_restaged
      && claim.pre_apply_checkpoint?.checkpoint_digest && claim.restore.checkpoint_digest === claim.pre_apply_checkpoint.checkpoint_digest);
  }
  if (GO_CHANGE) {
    // The completing assignment built bin/autoputer as a job that ended
    // done, and ran the requested go test, as a done job (direct or through
    // sh -c) or as a passing check cell.
    const done = doneJobs(claim.jobs);
    checks.go_build_job_done = done.some(isGoBuildOfAutoputer);
    const testJob = done.some(isGoTest);
    checks.go_test_passed = testJob || (claim.go_test_cells || []).length > 0;
    claim.go_test_evidence = testJob ? 'job' : ((claim.go_test_cells || []).length > 0 ? 'cell' : null);
  }
  if (VISUAL) {
    // Existence, binding and serving; the vision reading stays observed.
    Object.assign(checks, visualEvidenceChecks({ token: claim.token, listing: claim.visual?.listing, asset: claim.visual?.asset, range: claim.visual?.range }));
  }
  const servedOK = claim.served.pre_apply?.served === false && claim.served.post_apply?.served === true && claim.served.post_restore?.served === false;
  if (REQUIRE_SERVED) checks.served_change_then_restored = servedOK;
  if (CHANGE === 'go-served') {
    // Station A: the route exists only in the host-built binary; it serves
    // after apply from a layered release whose binary is the one answering,
    // and the restore takes it away again.
    checks.probe_absent_before_apply = claim.probe.pre_apply?.served === false && claim.probe.pre_apply?.status !== 0;
    checks.probe_served_after_apply = claim.probe.post_apply?.served === true;
    checks.layered_release_serving = claim.probe.post_apply?.layered_serving === true;
    // Scheduled now, or deferred to the next idle moment; the probe check
    // below is what proves the restart happened.
    checks.restore_restarted_into_pre_apply = claim.restore?.restart_scheduled === true
      || (typeof claim.restore?.restart_deferred === 'string' && claim.restore.restart_deferred.startsWith('busy'));
    checks.probe_gone_after_restore = claim.probe.post_restore?.served === false && claim.probe.post_restore?.status !== 0
      && claim.probe.post_restore?.effective_release?.layered !== true;
    // S6-0: the release witness answered at frozen, accepted, applied and
    // restored with every pair and its gate flag. The gap table is printed
    // and kept in the claim; an unbound pair does not fail this check.
    const joined = witnessJoined(claim.witness);
    claim.witness_joined = joined;
    claim.witness_gap_table = witnessGapTable(claim.witness);
    log(`release witness gap table (pair, compared at the gate, bound at frozen/accepted/applied/restored):\n${formatWitnessGapTable(claim.witness_gap_table)}`);
    checks.witness_joined = joined.ok;
  }
  out.acceptance = {
    scenario: 'engineering',
    claim: AUTONOMOUS
      ? 'the owner arms standing_consensus once on a test computer; management Casts engineering on records; engineering Freezes; management Verifies; the host builds the candidate and corpusd decides from its own ballot on that build and management\'s signed ballot, with no owner approval; the materializer applies the host-built layered release through one planned restart; a restore lands on the pre-request checkpoint'
      : 'management Casts engineering on records; engineering Freezes; management Verifies the bundle digest engineering\'s final Report names; the owner approves; the materializer applies through one planned restart that resumes once; a restore lands on the pre-apply checkpoint',
    autonomous: AUTONOMOUS,
    reached, checks, failed: Object.keys(checks).filter((k) => !checks[k]),
    served_change_then_restored: servedOK, served_required: REQUIRE_SERVED,
    ...(REWRITE ? { change: 'rewrite', app: APP, delegate: DELEGATE, engineering_assignments: claim.engineering_casts.length } : {}),
    operation_id: op?.operation_id || null, bundle_digest: op?.bundle_digest || null, assignment_id: cast?.assignment_id || null,
    build: { pre_apply: claim.build_pre_apply || null, post_apply: claim.build_post_apply || null, post_restore: claim.build_post_restore || null },
    census_forbidden_objects: claim.census?.objects ?? null,
    gaps,
    pass: Object.values(checks).every(Boolean),
  };
  phase('acceptance', { pass: out.acceptance.pass, reached, failed: out.acceptance.failed, gaps: gaps.length });
  out.drill_claim = claim;
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
if (KEEP_COMPUTER) receipt.keep_computer = true;
const RECEIPT_PATH = join(TRACE_DIR, 'receipt.json');

// The receipt goes to disk as each phase happens, marked in_progress, so a
// kill (even SIGKILL) leaves the minted computer and the phases
// (docs/problems/tracer-killed-leaves-computer-running-2026-10-15.md).
function persistReceipt() {
  try {
    writePrivate(RECEIPT_PATH, `${JSON.stringify(inProgressReceipt(receipt, iso()), null, 2)}\n`);
  } catch (err) {
    log(`receipt write failed (the run continues): ${err?.message || err}`);
  }
}

// Report, never stop, computers that earlier killed or interrupted runs
// left running: a tracer acts only on the computer it minted.
try {
  const root = dirname(TRACE_DIR);
  const entries = readdirSync(root, { withFileTypes: true })
    .filter((d) => d.isDirectory() && d.name !== LABEL && existsSync(join(root, d.name, 'receipt.json')))
    .map((d) => {
      try { return { label: d.name, receipt: JSON.parse(readFileSync(join(root, d.name, 'receipt.json'), 'utf8')) }; } catch { return { label: d.name, receipt: null }; }
    });
  for (const s of strandedTracerComputers(entries)) {
    log(`stranded: tracer ${s.label} left ${s.computer_id} (${s.reason}, since ${s.since}); stop it through POST /api/computers/{id}/lifecycle/stop if it is still running`);
  }
} catch (err) {
  log(`stranded-computer report skipped: ${err?.message || err}`);
}

// SIGINT, SIGTERM and SIGHUP stop the minted computer, close the receipt as
// interrupted, and exit 128 + the signal number. A second signal exits at
// once; the stop is bounded so a hung API cannot hold the exit.
let currentResult = null;
let interrupting = false;
for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
  process.on(signal, async () => {
    if (interrupting) process.exit(signalExitCode(signal));
    interrupting = true;
    log(`${signal}: stopping the minted computer and closing the receipt`);
    let stop = null;
    if (currentResult?.stopComputer) {
      stop = await Promise.race([currentResult.stopComputer().catch(() => null), sleep(90_000).then(() => null)]);
    }
    try {
      writePrivate(RECEIPT_PATH, `${JSON.stringify(interruptedReceipt(receipt, signal, iso(), stop), null, 2)}\n`);
    } catch (err) {
      log(`interrupted receipt write failed: ${err?.message || err}`);
    }
    process.exit(signalExitCode(signal));
  });
}

let exitCode = 1;
for (let n = 1; n <= ATTEMPTS; n++) {
  const result = { attempt: n };
  receipt.attempts.push(result);
  currentResult = result;
  persistReceipt();
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
  in_progress: false,
  exit_code: exitCode,
});
currentResult = null;
writePrivate(RECEIPT_PATH, `${JSON.stringify(receipt, null, 2)}\n`);
for (const a of receipt.attempts) {
  if (a.timings) log(`timings a${a.attempt}: ${JSON.stringify({ mint_ms: a.timings.mint_ms, first_prompt_accepted_ms: a.timings.first_prompt_accepted_ms, submit_to_first_revision_ms: a.timings.submit_to_first_revision_ms, restart_to_ready_ms: a.timings.restart_to_ready_ms })}`);
}
if (receipt.sha256) log(`bundle ${receipt.bundle_path} sha256 ${receipt.sha256}`);
console.log(JSON.stringify(receipt));
process.exit(exitCode);
