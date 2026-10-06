#!/usr/bin/env node
// SMG station deployed acceptance probe: the management desk is a full RLM —
// exactly one tool (desk_go_eval), all capability in-cell via choir.* verbs.
//
// Mint driver: deterministic owner-side POST /api/texture/management-open
// (the desk's open_persistent_super authoring is a model-behavior dependency
// — replaced here so legs are reproducible). The management activation is
// instructed to exercise the cutover verbs:
//   1. an UNBOUND choir.ReportPacket first (no delivered lifecycle control
//      yet) — must be refused into the cell (no-binding refusal leg);
//   2. a choir.Cast to engineering for a trivial objective, then
//      choir.CancelAssignment on it — durable revoke + executor ack
//      (cancellation leg);
//   3. a bound choir.ReportPacket back to the owning Texture control with
//      work_disposition completed — the typed producer report surviving the
//      cutover (bound-report leg).
//
// Watches run events for: a persistent-management run mint, an assignment
// open + cancel/revoke record, a bound producer report (direction
// producer_report + delivered_to_loop_id), and the refusal text surfaced
// in a report. Exit 0 when the bound management report lands.
// Polling is status-aware: a non-2xx response aborts the leg with the
// HTTP status recorded, so a routing/auth failure never reads as "no
// bound report" (panel finding D).
import { mkdirSync, writeFileSync } from 'node:fs';

const HOST = process.env.CHOIR_HOST || 'https://choir.news';
const API_KEY = process.env.CHOIR_API_KEY || '';
const args = process.argv.slice(2);
const arg = (name, dflt) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : dflt;
};
const COMPUTER = arg('computer', 'computer-03335285269bdba4f94377e56879f9e6');
const MARKER = arg('marker', `smg-rlm-${Date.now()}`);
const TIMEOUT_MIN = Number(arg('timeout-min', '30'));
const OUT = arg('out', `docs/evidence/smg-rlm-acceptance-${new Date().toISOString().slice(0, 10)}.json`);


// Optional deploy-identity gate: --expect-commit <sha> refuses to run legs
// until the guest reports that build. A fresh disposable minted before the
// image build finished otherwise tests the prior release (deploy-race —
// docs/problems/sa-management-mint-no-start-slot-deadlock-2026-10-06.md).
const EXPECT_COMMIT = arg('expect-commit', '');
if (!API_KEY) { console.error('CHOIR_API_KEY is required'); process.exit(2); }

const headers = { Authorization: `Bearer ${API_KEY}`, 'X-Choir-Computer': COMPUTER };

async function api(path, method = 'GET', body = undefined) {
  const res = await fetch(`${HOST}${path}`, {
    method,
    headers: { ...headers, ...(body ? { 'Content-Type': 'application/json' } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const json = await res.json().catch(() => null);
  return { status: res.status, body: json };
}

const t0 = Date.now();
const iso = (ms) => new Date(ms).toISOString();
const result = {
  probe: 'smg_rlm_acceptance',
  computer: COMPUTER,
  marker: MARKER,
  started_at: iso(t0),
  legs: [],
  ok: false,
  failure: null,
};

function recordLeg(name, extra) {
  const leg = { name, ended_at: iso(Date.now()), wall_ms: Date.now() - t0, ...extra };
  result.legs.push(leg);
  console.log(`[leg] ${name} wall=${leg.wall_ms}ms ${JSON.stringify(extra)}`);
  return leg;
}
function finish(code, failure) {
  result.ok = code === 0;
  result.failure = failure || null;
  result.ended_at = iso(Date.now());
  try { mkdirSync(new URL('.', `file://${OUT}`).pathname, { recursive: true }); } catch {}
  writeFileSync(OUT, JSON.stringify(result, null, 2));
  console.log(`wrote ${OUT}`);
  process.exit(code);
}

// Objective drives the management activation's cell work. The mint itself is
// deterministic (management-open POST below) — the prompt only describes the
// three legs the activation must run inside desk_go_eval cells.
const objective =
  `SMG management-RLM acceptance probe (marker ${MARKER}). Inside desk_go_eval ` +
  `cells, in order: (a) BEFORE any bound report, call choir.ReportPacket(` +
  `"texture", {"kind":"execution_result","summary":"unbound probe"}, "") and ` +
  `record the refusal text; (b) choir.Cast("engineering", "SMG no-op: create ` +
  `a file smg-probe.txt containing the marker ${MARKER}", {}) then ` +
  `choir.CancelAssignment(<assignment id from the cast result>, "probe") and ` +
  `record the revoke outcome; (c) choir.ReportPacket back to texture with ` +
  `{"kind":"execution_result","summary":"SMG legs complete: unbound=<refusal ` +
  `text>; cancel=<revoke outcome>","work_disposition":"completed"}, "". ` +
  `Do not edit the document body.`;

// Deploy-identity gate: if --expect-commit was passed, the guest must report
// that build before we mint — otherwise a stale image would silently test the
// prior release. A freshly-refreshed guest replays boot passivation before
// answering authenticated endpoints — wait up to 5min for the auth path.
if (EXPECT_COMMIT) {
  let obs = null;
  const gateDeadline = Date.now() + 300 * 1000;
  while (Date.now() < gateDeadline) {
    obs = await api('/api/runtime/observability');
    if (obs.status === 200) break;
    await new Promise((r) => setTimeout(r, 15000));
  }
  const actual = String(obs?.body?.build?.commit || '');
  recordLeg('build_identity', { ok: obs?.status === 200, status: obs?.status, expected: EXPECT_COMMIT, actual });
  if (obs?.status !== 200) finish(1, `build identity fetch failed status=${obs?.status}`);
  if (!actual.startsWith(EXPECT_COMMIT)) {
    finish(1, `guest build ${actual} does not match expected ${EXPECT_COMMIT} — refusing to test stale release`);
  }
}

// Deterministic owner-side mint (panel item C): mints a real texture
// activation + issues an execution_request control to management:<owner>,
// replacing the desk's open_persistent_super authoring as the leg trigger.
const open = await api('/api/texture/management-open', 'POST', {
  objective,
  actions: [{ type: 'inspect_file', objective: 'exercise report/cancel/report legs via desk_go_eval cells', safety: { mutation_class: 'green', network: 'forbidden', file_mutation: 'forbidden' } }],
  command_id: `smg-rlm-open-${MARKER}`,
});
if (open.status !== 200 && open.status !== 202) {
  recordLeg('management_open', { ok: false, status: open.status, body: open.body });
  finish(1, 'management-open mint failed');
}
const TRAJ = open.body?.trajectory_id;
recordLeg('management_open', {
  ok: true, trajectory: TRAJ, doc: open.body?.doc_id,
  texture_run: open.body?.texture_run_id, work_item: open.body?.work_item_id,
  control: open.body?.control_id, update: open.body?.update_id,
});

// Poll the management agent's own trajectory for run events. The minted
// management run derives its trajectory from the bound control, so the same
// trajectory id surfaces run opens / producer reports. Status-aware: a
// non-2xx response aborts the leg with the HTTP status recorded, so a
// routing/auth failure never reads as "no events" (panel finding D).
async function trajEvents(trajID) {
  const ev = await api(`/api/trajectories/${trajID}/events?limit=500`);
  if (ev.status !== 200) {
    return { ok: false, status: ev.status, body: ev.body, events: [] };
  }
  return { ok: true, status: 200, events: ev.body?.events || [] };
}

const deadline = t0 + TIMEOUT_MIN * 60 * 1000;
let boundReport = null, cancelSeen = null, unboundRefusal = null, mgmtOpen = null;
const seen = new Set();

while (Date.now() < deadline) {
  const ev = await trajEvents(TRAJ);
  if (!ev.ok) {
    recordLeg('trajectory_events_poll', { ok: false, status: ev.status, body: ev.body });
    finish(1, `trajectory events fetch failed status=${ev.status}`);
  }
  for (const e of ev.events) {
    const key = `${e.kind}:${e.update_id || ''}:${e.event_id}`;
    if (seen.has(key)) continue;
    seen.add(key);
    const uid = String(e.update_id || '');
    const blob = JSON.stringify(e);
    // Persistent-management activation opened: a control_queued/delivered
    // event for the management agent, or an update whose id carries the
    // initial_dispatch convention (run:<id>:initial_dispatch).
    if (!mgmtOpen && (e.kind === 'control_queued' || e.kind === 'control_delivered' || uid.includes(':initial_dispatch'))) {
      mgmtOpen = e;
      recordLeg('persistent_management_signal', { update_id: uid, kind: e.kind });
    }
    // Cancellation leg: assignment opened then revoked.
    if (!cancelSeen && (blob.includes('assignment_revoked') || blob.includes('cancel_assignment') || blob.includes('CancelAssignment') || blob.includes('revoked'))) {
      cancelSeen = e;
      recordLeg('assignment_cancel_verb', { update_id: uid, kind: e.kind });
    }
    // Unbound refusal surfaced (management reported the refused error).
    if (!unboundRefusal && (blob.includes('unbound') || blob.includes('bound delivered control') || blob.includes('missing the ask') || blob.includes('requires'))) {
      unboundRefusal = e;
      recordLeg('unbound_refusal_evidence', { update_id: uid });
    }
    // Bound report: a result:sha256 update bound to the control's work item.
    // The queued event carries work_item_id as null; the delivered event
    // binds it, so the delivered event is the durable match.
    if ((e.kind === 'update_queued' || e.kind === 'update_delivered') && uid.startsWith('result:') && e.work_item_id === open.body?.work_item_id) {
      boundReport = { ...e };
      recordLeg('bound_producer_report', { update_id: uid, kind: e.kind, work_item_id: e.work_item_id });
    }
  }
  const traj = await api(`/api/trajectories/${TRAJ}`);
  if (traj.status !== 200) {
    recordLeg('trajectory_poll', { ok: false, status: traj.status, body: traj.body });
    finish(1, `trajectory fetch failed status=${traj.status}`);
  }
  // A bound producer report is the durable acceptance signal; a prompt-bar
  // trajectory stays live after the legs land, so do not gate on it.
  if (boundReport) break;
  await new Promise((r) => setTimeout(r, 8000));
}

recordLeg('final', {
  bound_report: boundReport ? true : false,
  cancel_verb: cancelSeen ? true : false,
  unbound_refusal: unboundRefusal ? true : false,
  mgmt_signal: mgmtOpen ? true : false,
});
finish(boundReport ? 0 : 1, boundReport ? null : 'no bound producer report observed');
