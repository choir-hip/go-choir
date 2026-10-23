#!/usr/bin/env node
// Cut 0 invariant checker, artifact (3) of
// docs/definitions/choir-reeng-cut0-eyes-2026-10-12.md.
//
//   node scripts/reeng_invariants.mjs <bundle.json> [--json <out>]
//
// Reads one trace bundle (docs/reports/reeng-trace-bundle-format-2026-10-12.md)
// offline. It never fetches. It checks substrate facts only, never content
// quality. Each rule returns pass | fail | unknown with evidence and a reason.
// A rule whose layer is unserved, or whose fields are absent, or that read
// nothing, returns unknown, never pass.
//
// Exit: 0 when no rule fails (unknown allowed), 1 when any rule fails,
// 2 when the input cannot be read as a bundle.
//
// Field names are the raw API JSON keys of the Go types; each is cited to
// internal/types (file:line) where it is read. Layer items are raw API JSON.

import { readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { pathToFileURL } from 'node:url';

export const BUNDLE_SCHEMA = 'choir.trace_bundle.v1';
export const REPORT_SCHEMA = 'choir.reeng_invariants.v1';
const EVIDENCE_CAP = 40;

// ---------------------------------------------------------------- layers

// Derived layers inherit `unserved` from their parent (bundle format, "Layers").
const DERIVED_FROM = {
  cells: 'run_events',
  egress: 'run_events',
  deliveries: 'trajectory',
  turn_commits: 'lifecycle_events',
};

// layer returns {name, status, usable, items, note}. A layer missing from the
// bundle is treated as unserved. `usable` means the route answered
// (present or empty), so an empty list is a real "nothing there".
export function layer(bundle, name) {
  const raw = bundle?.layers?.[name];
  let status = raw && typeof raw.status === 'string' ? raw.status : 'unserved';
  let note = raw ? String(raw.note || '') : 'layer absent from bundle';
  const parent = DERIVED_FROM[name];
  if (parent) {
    const p = bundle?.layers?.[parent];
    const pStatus = p && typeof p.status === 'string' ? p.status : 'unserved';
    if (pStatus === 'unserved' || pStatus === 'absent') {
      status = 'unserved';
      note = `derived from unserved layer ${parent}`;
    }
  }
  if (status !== 'present' && status !== 'empty') status = 'unserved';
  const items = status === 'unserved' ? [] : Array.isArray(raw?.items) ? raw.items : [];
  return { name, status, usable: status !== 'unserved', items, note };
}

// ---------------------------------------------------------------- helpers

const str = (v) => (typeof v === 'string' ? v.trim() : v == null ? '' : String(v).trim());

// Parse RFC3339 / RFC3339Nano to epoch ms. Fractions beyond milliseconds are
// truncated (floor), which is monotonic, so it never creates an overlap that
// the nanosecond values do not have.
export function ts(v) {
  if (v == null || v === '') return NaN;
  if (typeof v === 'number') return v;
  const s = String(v).replace(/(\.\d{3})\d+/, '$1');
  const t = Date.parse(s);
  return Number.isFinite(t) ? t : NaN;
}
const iso = (ms) => (Number.isFinite(ms) ? new Date(ms).toISOString() : '');

function parseMaybeJSON(v) {
  if (typeof v !== 'string') return v;
  const t = v.trim();
  if (!t) return null;
  try {
    return JSON.parse(t);
  } catch {
    return null;
  }
}

function capEvidence(list) {
  if (list.length <= EVIDENCE_CAP) return list;
  return [...list.slice(0, EVIDENCE_CAP), { truncated: list.length - EVIDENCE_CAP }];
}

function result(rule, verdict, reason, evidence = [], extra = {}) {
  return { rule, verdict, reason, evidence: capEvidence(evidence), ...extra };
}

// combine: fail dominates; then unknown; pass only when every part passed.
function combine(verdicts) {
  if (verdicts.includes('fail')) return 'fail';
  if (verdicts.length === 0 || verdicts.includes('unknown')) return 'unknown';
  return 'pass';
}

// RunRecord: internal/types/task.go:89-151. The run id is JSON key `loop_id`
// (task.go:93); agent_id :97; requested_by_run_id :106; agent_profile :114;
// state :127; error :138; created_at :141; finished_at :147; metadata :150.
const runID = (r) => str(r?.loop_id);
const meta = (o) => (o && typeof o.metadata === 'object' && o.metadata !== null ? o.metadata : parseMaybeJSON(o?.metadata) || {});

// EventRecord: internal/types/task.go:322-364. event_id :324, seq :329,
// ts :336, loop_id :340, kind :356, payload :363 (json.RawMessage, so an
// embedded object; a string is tolerated).
function eventPayload(ev) {
  const p = ev?.payload;
  if (p && typeof p === 'object') return p;
  return parseMaybeJSON(p) || {};
}

// cells layer items are one per desk.cell run event; egress layer items are
// one per egress row, whether a standalone desk.egress event or a row folded
// into a desk.cell event's egress[] (bundle format, journal cut R4). Accept either the raw EventRecord (payload nested)
// or the flattened payload; fall back to deriving from run_events when the
// layer key itself is missing from the bundle.
function deskItems(bundle, name, kind) {
  const l = layer(bundle, name);
  const rawLayer = bundle?.layers?.[name];
  let items = l.items;
  let status = l.status;
  if (!rawLayer) {
    const re = layer(bundle, 'run_events');
    status = re.status === 'unserved' ? 'unserved' : 'derived';
    items = re.items.filter((e) => str(e.kind) === kind);
    // Since journal cut R4 a cell's egress calls ride its desk.cell event, in
    // its egress array; the desk.egress events are older tapes, refusals and
    // the end of an async search. A folded row reads as the event it replaces.
    if (kind === 'desk.egress') {
      for (const cell of re.items.filter((e) => str(e.kind) === 'desk.cell')) {
        const rows = eventPayload(cell).egress;
        if (!Array.isArray(rows)) continue;
        rows.forEach((row, i) => {
          items.push({ kind, payload: row, event_id: `${str(cell.event_id)}#egress.${i}`, seq: cell.seq, loop_id: cell.loop_id, ts: cell.ts });
        });
      }
    }
  }
  const flat = items.map((it) => {
    if (it && typeof it === 'object' && 'payload' in it && 'kind' in it) {
      return { ...eventPayload(it), event_id: it.event_id, seq: it.seq, event_loop_id: it.loop_id, event_ts: it.ts };
    }
    return it;
  });
  return { status, usable: status !== 'unserved', items: flat };
}

function snapshotOf(bundle) {
  const l = layer(bundle, 'trajectory');
  return l.usable && l.items.length ? l.items[0] : null;
}

// ---------------------------------------------------------------- I1

// Texture desk revision (the only canonical writer, CLAUDE.md desk ontology):
// internal/textureowner/tools_texture.go:356-369 builds it with
// author_kind "appagent" (types.AuthorAppAgent, internal/types/texture.go:42),
// author_label "appagent", and metadata {source: "texture_cell",
// loop_id: <run id>} (texture_revision_metadata.go:18-21,
// tools_texture.go:1596-1606; isTextureWriteToolName tools_texture.go:113-120
// admits only "texture_cell"). The same path refuses a run whose agent is not
// "texture:<doc_id>" (tools_texture.go:376-381; currentTextureAgentID
// handler.go:315-321; agentprofile.Texture = "texture").
//
// Owner revisions are legitimate: every owner path writes author_kind "user"
// (types.AuthorUser, texture.go:36) with author_label = the owner id
// (texture_lifecycle_create.go:100, texture_agent_revision.go:160-167,
// texture.go:530-535, 654-659, 1115-1120, 1263-1268, 1821-1826,
// texture_handoff.go:243-245,
// agentcore/selfdev_texture_join.go:79-81). So: a "user" revision passes
// only when author_label equals the revision's owner_id (and that owner is
// the trajectory's owner). Anything else, including the frozen processor
// seed (coagent_route.go:205-210, metadata.source "coagent_texture_seed",
// author_label = the calling agent), is not the Texture desk and fails.
export function checkI1(bundle) {
  const revs = layer(bundle, 'revisions');
  if (!revs.usable) return result('I1', 'unknown', `revisions layer unserved (${revs.note})`);
  if (revs.items.length === 0) return result('I1', 'unknown', 'revisions layer is empty: no revision to check');
  const runs = layer(bundle, 'runs');
  const runByID = new Map(runs.items.map((r) => [runID(r), r]));
  const trajOwner = str(snapshotOf(bundle)?.trajectory?.owner_id);
  const fails = [];
  const evidence = [];
  let unknowns = 0;
  for (const rev of revs.items) {
    const id = str(rev.revision_id);
    const kind = str(rev.author_kind); // texture.go:111
    const label = str(rev.author_label); // texture.go:117
    const owner = str(rev.owner_id);
    const docID = str(rev.doc_id);
    const m = parseMaybeJSON(rev.metadata) || (typeof rev.metadata === 'object' && rev.metadata) || {};
    const ev = { revision_id: id, doc_id: docID, version_number: rev.version_number, author_kind: kind, author_label: label };
    if (kind === 'user') {
      if (!owner || label !== owner) {
        fails.push({ ...ev, why: 'user revision whose author_label is not its owner_id' });
      } else if (trajOwner && owner !== trajOwner) {
        fails.push({ ...ev, why: `user revision owner ${owner} is not the trajectory owner ${trajOwner}` });
      } else {
        evidence.push({ ...ev, decided: 'owner edit' });
      }
      continue;
    }
    if (kind !== 'appagent') {
      fails.push({ ...ev, why: `author_kind ${JSON.stringify(kind)} is neither the Texture desk nor the owner` });
      continue;
    }
    const source = str(m.source);
    const loop = str(m.loop_id);
    if (label !== 'appagent' || source !== 'texture_cell' || !loop) {
      fails.push({ ...ev, metadata_source: source, metadata_loop_id: loop, why: 'appagent revision not written through the Texture desk cell path' });
      continue;
    }
    const run = runByID.get(loop);
    if (!run) {
      // The desk path itself proves the agent (tools_texture.go:376-381);
      // the run is only corroboration and the runs layer need not carry it.
      if (!runs.usable) unknowns++;
      evidence.push({ ...ev, loop_id: loop, decided: runs.usable ? 'desk cell metadata; run not in bundle' : 'desk cell metadata; runs layer unserved' });
      continue;
    }
    const agent = str(run.agent_id);
    const profile = str(run.agent_profile);
    if (agent !== `texture:${docID}` && !(profile === 'texture' && agent.startsWith('texture:'))) {
      fails.push({ ...ev, loop_id: loop, run_agent_id: agent, why: `writing run's agent is not texture:${docID}` });
      continue;
    }
    evidence.push({ ...ev, loop_id: loop, run_agent_id: agent, decided: 'Texture desk run' });
  }
  if (fails.length) return result('I1', 'fail', `${fails.length} of ${revs.items.length} revisions not written by the Texture desk or the owner`, fails);
  if (unknowns) return result('I1', 'unknown', `${unknowns} desk revisions could not be tied to a run (runs layer unserved)`, evidence);
  return result('I1', 'pass', `all ${revs.items.length} revisions written by the Texture desk or the owner`, evidence);
}

// ---------------------------------------------------------------- I2

// RunState (internal/types/task.go:24-51; Terminal :55-62): completed,
// failed, cancelled are closed. Open states with a named reason:
//  - passivated + metadata.passivated_reason (written at boot
//    agentcore/runtime.go:2403, and runtime.go:3265, 3628, 3675,
//    runtime_persistence.go:87, textureowner/texture_controller.go:564);
//  - blocked + error (task.go:136-138: "structured error ... when blocked").
// pending / running with no reason fail: the tracer contract is to export
// after quiescence (the run reached publication and the computer is idle),
// so a run still pending or running at export has no named reason.
//
// Trajectory (TrajectoryRecord internal/types/trajectory.go:52-67, status
// :58, values :11-13): settled / cancelled are closed. A live trajectory is
// open with a named reason only when a cancellation was requested with a
// reason (lifecycle kind trajectory_cancellation_requested, lifecycle.go:60,
// LifecycleEvent.reason :669), or every open work item names its reason
// (WorkItemRecord.reason trajectory.go:91, status :96), or (C1e, records)
// the inboxes owe an open round (ConversationHead, types/inbox.go:113, the
// round and its recipient are the named reason) or the last round of a
// conversation was closed by the system with a stop reason
// (NewRoundClosureRecord, types/commitment.go:437, e.g.
// interrupted_by_restart). A live trajectory with nothing owed and no
// reason fails; one whose only possible evidence is an unserved records
// layer is unknown.
// recordsOwed reads what the records owe: open rounds from the inboxes
// layer, and conversations whose latest round was closed by a system
// closure naming a stop reason.
function recordsOwed(bundle) {
  const recL = layer(bundle, 'records');
  const inL = layer(bundle, 'inboxes');
  if (!recL.usable || !inL.usable) {
    return { usable: false, note: [recL, inL].filter((l) => !l.usable).map((l) => `${l.name}: ${l.note}`).join('; '), openRounds: [], closures: [] };
  }
  const openRounds = [];
  for (const state of inL.items) {
    for (const round of state.open_rounds || []) {
      if (round.open === false) continue;
      openRounds.push({ inbox: str(state.inbox), conversation_id: str(round.conversation_id), round: Number(round.round) || 0, recipient: str(round.recipient_inbox) });
    }
  }
  const latest = new Map();
  for (const r of recL.items) {
    const k = str(r.conversation_id);
    if (!k) continue;
    const prev = latest.get(k);
    if (!prev || ts(r.provenance?.committed_at) >= ts(prev.provenance?.committed_at)) latest.set(k, r);
  }
  const closures = [];
  for (const [conv, r] of latest) {
    const fate = str(r.report?.stop_reason);
    if (r.final && str(r.emitter_kind) === 'system' && fate) closures.push({ conversation_id: conv, round: Number(r.round) || 0, record_id: str(r.record_id), fate });
  }
  return { usable: true, note: '', openRounds, closures };
}

export function checkI2(bundle) {
  const runsL = layer(bundle, 'runs');
  const trajL = layer(bundle, 'trajectory');
  const lev = layer(bundle, 'lifecycle_events');
  const parts = {};
  const evidence = [];
  const fails = [];

  // runs half
  if (!runsL.usable) parts.runs = { verdict: 'unknown', reason: `runs layer unserved (${runsL.note})` };
  else if (runsL.items.length === 0) parts.runs = { verdict: 'unknown', reason: 'runs layer is empty' };
  else {
    let bad = 0;
    for (const r of runsL.items) {
      const st = str(r.state);
      const m = meta(r);
      const e = { loop_id: runID(r), agent_id: str(r.agent_id), state: st };
      if (st === 'completed' || st === 'failed' || st === 'cancelled') {
        evidence.push({ ...e, closed: true, error: str(r.error) || undefined });
      } else if (st === 'passivated' && str(m.passivated_reason)) {
        evidence.push({ ...e, open_reason: str(m.passivated_reason) });
      } else if (st === 'blocked' && (str(r.error) || str(m.blocked_reason))) {
        evidence.push({ ...e, open_reason: str(r.error) || str(m.blocked_reason) });
      } else {
        bad++;
        fails.push({ ...e, why: st ? `run ${st} with no named reason at export` : 'run has no state' });
      }
    }
    parts.runs = bad
      ? { verdict: 'fail', reason: `${bad} of ${runsL.items.length} runs open with no named reason` }
      : { verdict: 'pass', reason: `${runsL.items.length} runs closed or open with a named reason` };
  }

  // trajectory half
  const snap = snapshotOf(bundle);
  if (!trajL.usable) parts.trajectory = { verdict: 'unknown', reason: `trajectory layer unserved (${trajL.note})` };
  else if (!snap || !snap.trajectory) parts.trajectory = { verdict: 'unknown', reason: 'trajectory layer carries no snapshot' };
  else {
    const t = snap.trajectory;
    const status = str(t.status);
    const e = { trajectory_id: str(t.trajectory_id), status };
    const events = lev.items.filter((x) => str(x.trajectory_id) === str(t.trajectory_id) || !str(x.trajectory_id));
    if (status === 'settled' || status === 'cancelled') {
      const closer = events.find((x) => str(x.kind) === (status === 'settled' ? 'trajectory_settled' : 'trajectory_cancelled'));
      evidence.push({ ...e, closed: true, event_id: closer ? str(closer.event_id) : undefined, reason: closer ? str(closer.reason) || undefined : undefined });
      parts.trajectory = { verdict: 'pass', reason: `trajectory ${status}` };
    } else if (status === 'live') {
      const cancelReq = [...events].reverse().find((x) => str(x.kind) === 'trajectory_cancellation_requested' && str(x.reason));
      const open = (snap.work_items || []).filter((w) => str(w.status) === 'open');
      const unnamed = open.filter((w) => !str(w.reason));
      const owed = recordsOwed(bundle);
      if (cancelReq) {
        evidence.push({ ...e, open_reason: str(cancelReq.reason), event_id: str(cancelReq.event_id) });
        parts.trajectory = { verdict: 'pass', reason: 'trajectory live with a requested cancellation' };
      } else if (open.length > 0 && unnamed.length === 0) {
        evidence.push({ ...e, open_work_items: open.map((w) => ({ work_item_id: str(w.work_item_id), reason: str(w.reason) })) });
        parts.trajectory = { verdict: 'pass', reason: `trajectory live with ${open.length} open work items, each naming a reason` };
      } else if (open.length === 0 && owed.openRounds.length > 0) {
        evidence.push({ ...e, open_rounds: owed.openRounds });
        parts.trajectory = { verdict: 'pass', reason: `trajectory live with ${owed.openRounds.length} open round(s) on records, each owed by its recipient` };
      } else if (open.length === 0 && owed.closures.length > 0) {
        evidence.push({ ...e, closed_rounds: owed.closures });
        parts.trajectory = { verdict: 'pass', reason: `trajectory live; its last round(s) closed by the system with a named fate (${[...new Set(owed.closures.map((c) => c.fate))].join(', ')})` };
      } else if (open.length === 0 && !owed.usable) {
        parts.trajectory = { verdict: 'unknown', reason: `trajectory live with no open work items and no cancellation; the records evidence is unserved (${owed.note})` };
      } else {
        fails.push({ ...e, open_work_items: open.length, unnamed_work_items: unnamed.map((w) => str(w.work_item_id)), why: open.length ? 'open work items name no reason' : 'live trajectory with nothing owed and no named reason' });
        parts.trajectory = { verdict: 'fail', reason: open.length ? `trajectory live; ${unnamed.length} open work items name no reason` : 'trajectory live with no open work and no named reason' };
      }
    } else {
      fails.push({ ...e, why: `unrecognised trajectory status ${JSON.stringify(status)}` });
      parts.trajectory = { verdict: 'fail', reason: `trajectory status ${JSON.stringify(status)}` };
    }
  }
  const verdict = combine([parts.runs.verdict, parts.trajectory.verdict]);
  const reason = `runs: ${parts.runs.reason}; trajectory: ${parts.trajectory.reason}`;
  return result('I2', verdict, reason, verdict === 'fail' ? fails : evidence, { parts });
}

// ---------------------------------------------------------------- I3

// Boot kind: GET /api/runtime/obligations restart.kind is "planned" when the
// planned-restart marker was consumed, else "crash_or_stop"
// (internal/agentcore/obligations_surface.go:157-161). Read from the boot
// layer item, then the drill's obligations_after.
export function bootKind(bundle) {
  const boot = layer(bundle, 'boot');
  const fromBoot = boot.usable && boot.items.length ? str(boot.items[0]?.restart?.kind) : '';
  if (fromBoot) return { kind: fromBoot, source: 'boot.items[0].restart.kind' };
  const after = bundle?.drill?.obligations_after;
  const fromDrill = str(after?.restart?.kind);
  if (fromDrill) return { kind: fromDrill, source: 'drill.obligations_after.restart.kind' };
  return { kind: '', source: '' };
}

// Boot instant, in order of precision:
//  1. drill.boot_at or boot.items[0].booted_at (if the tracer records one);
//  2. the earliest run event at or after restart_requested_at of kind
//     activation.passivated whose payload.recovery is "passivated_on_restart":
//     the new process writes it while passivating the previous process's
//     runs at boot (agentcore/runtime.go:2381-2424; kind task.go:183);
//  3. drill.restart_requested_at (approximation: the old process may still
//     drain for a moment after the request; see report assumptions).
export function bootAt(bundle) {
  const drill = bundle?.drill;
  const requested = ts(drill?.restart_requested_at);
  if (!Number.isFinite(requested)) return { at: NaN, source: '' };
  const explicit = ts(drill?.boot_at);
  // The tracer falls back to the first answer after the restart, an upper
  // bound: a dispatch between the real boot and that answer would read as
  // pre-boot. Use the restart request instead, marked approximate.
  if (Number.isFinite(explicit) && /upper bound/.test(str(drill?.boot_at_source))) {
    return { at: requested, source: 'drill.restart_requested_at (approximate; boot_at is an upper bound)' };
  }
  if (Number.isFinite(explicit)) return { at: explicit, source: 'drill.boot_at' };
  const boot = layer(bundle, 'boot');
  const booted = boot.usable && boot.items.length ? ts(boot.items[0]?.booted_at) : NaN;
  if (Number.isFinite(booted)) return { at: booted, source: 'boot.items[0].booted_at' };
  const re = layer(bundle, 'run_events');
  let first = NaN;
  for (const e of re.items) {
    const p = eventPayload(e);
    const restartClose = (str(e.kind) === 'activation.passivated' && str(p.recovery) === 'passivated_on_restart') ||
      (str(e.kind) === 'loop.cancelled' && str(p.fate) === 'interrupted_by_restart');
    if (!restartClose) continue;
    const t = ts(e.ts);
    if (t >= requested && !(first <= t)) first = t;
  }
  if (Number.isFinite(first)) return { at: first, source: 'first restart closure run event' };
  return { at: requested, source: 'drill.restart_requested_at (approximate)' };
}

// Run events that close or release a run rather than act on it.
const CLOSURE_RUN_EVENTS = new Set(['activation.passivated', 'loop.failed', 'loop.cancelled', 'loop.blocked']);

// Owner input (what lets post-boot work happen without resuming):
//  - a run the owner submitted: metadata.submission_surface / input_source
//    "prompt_bar" (textureowner/prompt_bar.go:94-103) or "management_open"
//    (textureowner/management_open.go:104-111);
//  - an owner revision: author_kind "user" with author_label = owner_id
//    (see I1);
//  - an owner revise commit: lifecycle artifact_head_advanced whose command_id
//    starts "owner-revise:" (texture_agent_revision.go:178,
//    texture_turn_runtime.go:174).
const OWNER_SURFACES = new Set(['prompt_bar', 'management_open']);
function isOwnerRun(r) {
  const m = meta(r);
  return OWNER_SURFACES.has(str(m.submission_surface)) || OWNER_SURFACES.has(str(m.input_source));
}
function isOwnerRevision(rev) {
  return str(rev.author_kind) === 'user' && str(rev.author_label) !== '' && str(rev.author_label) === str(rev.owner_id);
}

// I3: no dispatch after a markerless boot whose cause predates the boot.
//
// Post-boot dispatches (each with time t and recorded causes):
//  a. runs created at/after boot (RunRecord.created_at task.go:141), not owner
//     runs. Causes: requested_by_run_id (task.go:106, also metadata),
//     metadata.current_revision_id (texture_controller.go:1037),
//     metadata.lifecycle_work_item_id, packets delivered to it
//     (CoagentSourcePacket.delivered_to_loop_id, types/evidence.go:233).
//  b. runs created before boot that act after it: a non-closure run event
//     with ts >= boot, or a run_reactivated lifecycle event (run id in
//     artifact_refs, store/lifecycle.go:5418-5423). Causes: the run itself
//     (pre-boot), metadata.current_revision_id, packets delivered to it.
//  c. packets (snapshot updates, CreatedAt evidence.go:232) created at/after
//     boot. Causes: source_run_id (evidence.go:212).
//  d. packets created before boot but delivered after it: delivered_at >= boot
//     or a control_delivered / update_delivered event naming update_id
//     (store/lifecycle_control_delivery.go:552). Cause: the packet (pre-boot).
//  e. work items created at/after boot (WorkItemRecord.created_at
//     trajectory.go:103). Cause: created_by_loop_id (trajectory.go:101).
//  f. engineering assignments created at/after boot (co_super_assignments,
//     types/engineering_assignment.go:291-307). Causes: binding.parent_loop_id
//     (:77) and any earlier attempt with the same assignment_id created before
//     boot (the restart recast, agentcore/engineering_desk.go:213-279).
//
// A dispatch is legitimate when one of its causes is an owner input at/after
// boot, or a legitimate post-boot item that existed at time t. Otherwise, if
// a cause predates the boot (or is itself a violation): it is a violation
// when no owner input happened in [boot, t], and ambiguous (unknown) when
// owner input happened in that window but the record names no link to it.
// A dispatch with no recorded cause is ambiguous.
// The quiet span after a markerless boot that makes an observed absence of
// dispatches an I3 pass.
export const I3_MIN_QUIET_MS = 120_000;

export function checkI3(bundle) {
  const kind = bootKind(bundle);
  const drill = bundle?.drill;
  if (!drill || !Number.isFinite(ts(drill.restart_requested_at))) {
    return result('I3', 'unknown', 'no drill block with restart_requested_at: no boot to check');
  }
  if (!kind.kind) return result('I3', 'unknown', 'restart kind unknown (boot layer unserved and no drill.obligations_after.restart)');
  if (kind.kind === 'planned') {
    return result('I3', 'unknown', 'boot was planned (marker present); I3 covers markerless boots only', [{ restart_kind: kind.kind, source: kind.source }]);
  }
  const boot = bootAt(bundle);
  const B = boot.at;
  const runsL = layer(bundle, 'runs');
  const levL = layer(bundle, 'lifecycle_events');
  const reL = layer(bundle, 'run_events');
  const trajL = layer(bundle, 'trajectory');
  const revL = layer(bundle, 'revisions');
  // Records are the dispatch authority for the conversations on records
  // (C1e): an unserved records layer leaves those dispatches unread, so it
  // is a missing input, never an observed absence.
  const recordsL = layer(bundle, 'records');
  const missing = [];
  for (const l of [runsL, levL, reL, trajL, revL, recordsL]) if (!l.usable) missing.push(l.name);

  const snap = snapshotOf(bundle) || {};
  const runs = runsL.items;
  const runByID = new Map(runs.map((r) => [runID(r), r]));
  const packets = Array.isArray(snap.updates) ? snap.updates : [];
  const packetByID = new Map(packets.map((p) => [str(p.update_id), p]));
  // Cut 1 L3a: the Texture-research conversation is on records. A record is
  // committed by its sender's run; a run started for an inbox
  // (request_source inbox_record) is caused by the newest record of that
  // inbox committed before it.
  const recL = layer(bundle, 'records');
  const recordsList = recL.usable ? recL.items : [];
  const recordByID = new Map(recordsList.map((r) => [str(r.record_id), r]));
  const recordAt = (r) => ts(r?.provenance?.committed_at);
  const workItems = Array.isArray(snap.work_items) ? snap.work_items : [];
  const workByID = new Map(workItems.map((w) => [str(w.work_item_id), w]));
  const revByID = new Map(revL.items.map((r) => [str(r.revision_id), r]));
  const assignments = Array.isArray(snap.co_super_assignments) ? snap.co_super_assignments : [];

  // owner inputs at/after boot
  const ownerInputs = [];
  for (const r of runs) if (isOwnerRun(r) && ts(r.created_at) >= B) ownerInputs.push({ kind: 'owner_run', id: runID(r), t: ts(r.created_at) });
  for (const rev of revL.items) if (isOwnerRevision(rev) && ts(rev.created_at) >= B) ownerInputs.push({ kind: 'owner_revision', id: str(rev.revision_id), t: ts(rev.created_at) });
  for (const e of levL.items) {
    if (str(e.kind) === 'artifact_head_advanced' && str(e.command_id).startsWith('owner-revise:') && ts(e.created_at) >= B) {
      ownerInputs.push({ kind: 'owner_revise_event', id: str(e.event_id), t: ts(e.created_at) });
    }
  }
  ownerInputs.sort((a, b) => a.t - b.t);
  const firstOwner = ownerInputs.length ? ownerInputs[0].t : Infinity;
  const ownerRunIDs = new Set(ownerInputs.filter((o) => o.kind === 'owner_run').map((o) => o.id));
  const ownerRevIDs = new Map(ownerInputs.filter((o) => o.kind === 'owner_revision').map((o) => [o.id, o.t]));

  const nodes = [];
  const deliveredTo = (rid) => packets.filter((p) => str(p.delivered_to_loop_id) === rid);
  const runCauses = (r, self) => {
    const m = meta(r);
    const c = [];
    for (const rb of [str(r.requested_by_run_id), str(m.requested_by_run_id)]) if (rb && rb !== self) c.push({ kind: 'run', id: rb });
    if (str(m.current_revision_id)) c.push({ kind: 'revision', id: str(m.current_revision_id) });
    if (str(m.lifecycle_work_item_id)) c.push({ kind: 'work_item', id: str(m.lifecycle_work_item_id) });
    for (const p of deliveredTo(runID(r))) c.push({ kind: 'packet', id: str(p.update_id) });
    if (str(m.request_source) === 'inbox_record') {
      const created = ts(r.created_at);
      let newest = null;
      for (const rec of recordsList) {
        if (str(rec.inbox) !== str(r.agent_id) || !(recordAt(rec) <= created)) continue;
        if (!newest || recordAt(rec) > recordAt(newest)) newest = rec;
      }
      if (newest) c.push({ kind: 'record', id: str(newest.record_id) });
    }
    return c;
  };

  // a + b
  const reactivatedAt = new Map();
  for (const e of levL.items) {
    if (str(e.kind) !== 'run_reactivated') continue;
    const t = ts(e.created_at);
    if (!(t >= B)) continue;
    const rid = str((e.artifact_refs || [])[0]) || str(e.run_id);
    if (rid && !(reactivatedAt.get(rid) <= t)) reactivatedAt.set(rid, t);
  }
  const firstActivity = new Map();
  for (const e of reL.items) {
    const t = ts(e.ts);
    if (!(t >= B) || CLOSURE_RUN_EVENTS.has(str(e.kind))) continue;
    const rid = str(e.loop_id);
    if (rid && !(firstActivity.get(rid)?.t <= t)) firstActivity.set(rid, { t, event_id: str(e.event_id), kind: str(e.kind), seq: e.seq });
  }
  for (const r of runs) {
    const id = runID(r);
    const created = ts(r.created_at);
    if (ownerRunIDs.has(id)) continue;
    if (created >= B) {
      nodes.push({ kind: 'run_dispatched', id, t: created, causes: runCauses(r, id), ev: { loop_id: id, agent_id: str(r.agent_id), created_at: r.created_at } });
      continue;
    }
    const act = firstActivity.get(id);
    const re = reactivatedAt.get(id);
    let t = act ? act.t : Infinity;
    if (re !== undefined && re < t) t = re;
    if (Number.isFinite(t)) {
      nodes.push({ kind: 'run_resumed', id, t, causes: [{ kind: 'run_self', id }, ...runCauses(r, id)], ev: { loop_id: id, agent_id: str(r.agent_id), created_at: r.created_at, first_post_boot_activity: iso(t), event_id: act?.event_id, event_kind: act?.kind, seq: act?.seq } });
    }
  }
  // c + d
  const deliveryEvent = new Map();
  for (const e of levL.items) {
    const k = str(e.kind);
    if (k !== 'control_delivered' && k !== 'update_delivered') continue;
    const t = ts(e.created_at);
    const u = str(e.update_id);
    if (u && t >= B && !(deliveryEvent.get(u)?.t <= t)) deliveryEvent.set(u, { t, event_id: str(e.event_id), reducer_seq: e.reducer_seq });
  }
  for (const p of packets) {
    const id = str(p.update_id);
    const created = ts(p.created_at);
    if (created >= B) {
      const c = [];
      if (str(p.source_run_id)) c.push({ kind: 'run', id: str(p.source_run_id) });
      nodes.push({ kind: 'packet_created', id, t: created, causes: c, ev: { update_id: id, source_run_id: str(p.source_run_id), created_at: p.created_at } });
      continue;
    }
    const dAt = ts(p.delivered_at);
    const dEv = deliveryEvent.get(id);
    let t = dAt >= B ? dAt : Infinity;
    if (dEv && dEv.t < t) t = dEv.t;
    if (Number.isFinite(t)) nodes.push({ kind: 'packet_redelivered', id, t, causes: [{ kind: 'packet_self', id }], ev: { update_id: id, created_at: p.created_at, delivered_at: p.delivered_at, delivered_to_loop_id: str(p.delivered_to_loop_id), event_id: dEv?.event_id } });
  }
  // Delivery events naming packets the snapshot does not carry (the snapshot
  // holds only this trajectory's packets, store/lifecycle.go:2420-2427; a
  // computer-scoped directive has no trajectory). The packet's creation time
  // is then its control_queued / update_queued event (lifecycle.go:3955-3960,
  // lifecycle_commit_act.go:397-403): queued before boot means a pre-boot
  // packet delivered after it; queued after boot with no recorded source is
  // ambiguous.
  const queuedAt = new Map();
  for (const e of levL.items) {
    const k = str(e.kind);
    if (k !== 'control_queued' && k !== 'update_queued') continue;
    const u = str(e.update_id);
    const t = ts(e.created_at);
    if (u && Number.isFinite(t) && !(queuedAt.get(u) <= t)) queuedAt.set(u, t);
  }
  for (const [u, d] of deliveryEvent) {
    if (packetByID.has(u)) continue;
    const q = queuedAt.get(u);
    if (q < B) nodes.push({ kind: 'packet_redelivered', id: u, t: d.t, causes: [{ kind: 'packet_self', id: u }], ev: { update_id: u, queued_at: iso(q), event_id: d.event_id } });
    else nodes.push({ kind: 'packet_delivery_unresolved', id: u, t: d.t, causes: [], ev: { update_id: u, queued_at: Number.isFinite(q) ? iso(q) : null, event_id: d.event_id } });
  }
  // records committed after the boot: caused by the sender's run active then
  for (const rec of recordsList) {
    const t = recordAt(rec);
    if (!(t >= B)) continue;
    const sender = str(rec.sender_inbox) || str(rec.provenance?.agent_id);
    const c = [];
    if (str(rec.emitter_kind) === 'system') c.push({ kind: 'system_record', id: str(rec.record_id) });
    for (const r of runs) {
      if (str(r.agent_id) !== sender || !(ts(r.created_at) <= t)) continue;
      const end = ts(r.finished_at);
      if (Number.isFinite(end) && end < t) continue;
      c.push({ kind: 'run', id: runID(r) });
    }
    nodes.push({ kind: 'record_committed', id: str(rec.record_id), t, causes: c, ev: { record_id: str(rec.record_id), inbox: str(rec.inbox), sender: sender, committed_at: rec.provenance?.committed_at } });
  }
  // census: retired work-state objects minted after the boot that the
  // trajectory snapshot does not carry (computer-scoped, or after C1e drops
  // them from the snapshot). The census names no cause, so each is
  // ambiguous unless the snapshot explains it.
  const censusL = layer(bundle, 'census');
  if (censusL.usable) {
    for (const o of Array.isArray(censusL.items[0]?.objects) ? censusL.items[0].objects : []) {
      const id = str(o.canonical_id);
      const t = ts(o.created_at);
      if (!(t >= B) || packetByID.has(id) || workByID.has(id)) continue;
      nodes.push({ kind: 'census_object_minted', id, t, causes: [], ev: { kind: str(o.kind), canonical_id: id, from: str(o.from), to: str(o.to), created_at: o.created_at } });
    }
  }
  // e
  for (const w of workItems) {
    const created = ts(w.created_at);
    if (!(created >= B)) continue;
    const c = str(w.created_by_loop_id) ? [{ kind: 'run', id: str(w.created_by_loop_id) }] : [];
    nodes.push({ kind: 'work_opened', id: str(w.work_item_id), t: created, causes: c, ev: { work_item_id: str(w.work_item_id), created_by_loop_id: str(w.created_by_loop_id), created_at: w.created_at } });
  }
  // f
  for (const a of assignments) {
    const created = ts(a.created_at);
    if (!(created >= B)) continue;
    const c = [];
    const parent = str(a.binding?.parent_loop_id);
    if (parent) c.push({ kind: 'run', id: parent });
    for (const prior of assignments) {
      if (prior === a || str(prior.assignment_id) !== str(a.assignment_id)) continue;
      if (ts(prior.created_at) < B) c.push({ kind: 'assignment_prior', id: `${str(prior.assignment_id)}#${prior.binding?.attempt ?? '?'}`, t: ts(prior.created_at), reason: str(prior.disposition_reason) });
    }
    nodes.push({ kind: 'assignment_opened', id: `${str(a.assignment_id)}#${a.binding?.attempt ?? '?'}`, t: created, causes: c, ev: { assignment_id: str(a.assignment_id), attempt: a.binding?.attempt, parent_loop_id: parent, created_at: a.created_at } });
  }

  nodes.sort((x, y) => x.t - y.t);
  const status = new Map(); // `${kind}:${id}` -> {status, t}
  const runStatus = new Map(); // run id -> {status, t}
  const packetStatus = new Map();
  const workStatus = new Map();
  const recordStatus = new Map();
  for (const n of nodes) {
    const resolved = n.causes.map((c) => resolveCause(c, n.t));
    let st;
    if (resolved.some((x) => x === 'legit')) st = 'legit';
    else if (resolved.some((x) => x === 'pre_boot' || x === 'violation')) st = n.t < firstOwner ? 'violation' : 'ambiguous';
    else if (resolved.length === 0) st = 'ambiguous';
    else st = 'ambiguous';
    n.status = st;
    n.resolved = n.causes.map((c, i) => ({ ...c, t: undefined, resolved: resolved[i] }));
    status.set(`${n.kind}:${n.id}`, st);
    if (n.kind === 'run_dispatched' || n.kind === 'run_resumed') runStatus.set(n.id, { status: st, t: n.t });
    if (n.kind === 'packet_created' || n.kind === 'packet_redelivered') packetStatus.set(n.id, { status: st, t: n.t });
    if (n.kind === 'work_opened') workStatus.set(n.id, { status: st, t: n.t });
    if (n.kind === 'record_committed') recordStatus.set(n.id, { status: st, t: n.t });
  }

  function resolveCause(c, t) {
    switch (c.kind) {
      case 'run_self':
      case 'packet_self':
        return 'pre_boot';
      case 'assignment_prior':
        return 'pre_boot';
      case 'system_record':
        // A closure the boot writes (Cut 1 L3) is the restart's own fate.
        return 'legit';
      case 'record': {
        const s = recordStatus.get(c.id);
        if (s && s.t <= t) return s.status === 'legit' ? 'legit' : s.status === 'violation' ? 'violation' : 'unrelated';
        const rec = recordByID.get(c.id);
        if (rec && recordAt(rec) < B) return 'pre_boot';
        return 'unresolved';
      }
      case 'run': {
        if (ownerRunIDs.has(c.id)) return ownerInputTime(c.id) <= t ? 'legit' : 'unrelated';
        const s = runStatus.get(c.id);
        const r = runByID.get(c.id);
        if (s && s.t <= t) return s.status === 'legit' ? 'legit' : s.status === 'violation' ? 'violation' : 'unrelated';
        if (r && ts(r.created_at) < B) return 'pre_boot';
        return 'unresolved';
      }
      case 'revision': {
        if (ownerRevIDs.has(c.id)) return ownerRevIDs.get(c.id) <= t ? 'legit' : 'unrelated';
        const rev = revByID.get(c.id);
        if (rev && ts(rev.created_at) < B) return 'pre_boot';
        return 'unresolved';
      }
      case 'work_item': {
        const s = workStatus.get(c.id);
        if (s && s.t <= t) return s.status === 'legit' ? 'legit' : s.status === 'violation' ? 'violation' : 'unrelated';
        const w = workByID.get(c.id);
        if (w && ts(w.created_at) < B) return 'pre_boot';
        return 'unresolved';
      }
      case 'packet': {
        const s = packetStatus.get(c.id);
        if (s && s.t <= t) return s.status === 'legit' ? 'legit' : s.status === 'violation' ? 'violation' : 'unrelated';
        const p = packetByID.get(c.id);
        if (p && ts(p.created_at) < B) return 'pre_boot';
        return 'unresolved';
      }
      default:
        return 'unresolved';
    }
  }
  function ownerInputTime(id) {
    const o = ownerInputs.find((x) => x.id === id);
    return o ? o.t : Infinity;
  }

  const violations = nodes.filter((n) => n.status === 'violation');
  const ambiguous = nodes.filter((n) => n.status === 'ambiguous');
  const ctx = { boot_at: iso(B), boot_at_source: boot.source, restart_kind: kind.kind, restart_kind_source: kind.source, first_owner_input_after_boot: Number.isFinite(firstOwner) ? iso(firstOwner) : null, post_boot_dispatches: nodes.length };
  const show = (n) => ({ dispatch: n.kind, ...n.ev, at: iso(n.t), status: n.status, causes: n.resolved });
  // Owner input is read from runs and revisions; with either unserved, an
  // owner act could hide behind a candidate violation, so it is not a fail.
  const ownerBlind = [runsL, revL].filter((l) => !l.usable).map((l) => l.name);
  if (violations.length && ownerBlind.length) {
    return result('I3', 'unknown', `${violations.length} candidate violation(s), but owner input cannot be ruled out (unserved: ${ownerBlind.join(', ')})`, [ctx, ...violations.map(show)]);
  }
  if (violations.length) {
    return result('I3', 'fail', `${violations.length} dispatch(es) after a markerless boot caused by pre-boot work with no owner input since the boot`, [ctx, ...violations.map(show)]);
  }
  if (missing.length) return result('I3', 'unknown', `layers unserved: ${missing.join(', ')}`, [ctx, ...ambiguous.map(show)]);
  if (nodes.length === 0) {
    // An observed absence passes only when every layer I3 reads is served
    // (checked above) and the bundle covers at least I3_MIN_QUIET_MS after
    // the boot; a shorter window read too little to tell absence from not
    // yet (agentic-consensus 2026-10-13, convergent C3).
    const covered = ts(bundle?.exported_at) - B;
    if (Number.isFinite(covered) && covered >= I3_MIN_QUIET_MS) {
      return result('I3', 'pass', `nothing was dispatched in the ${Math.round(covered / 1000)} s between the boot and the export`, [{ ...ctx, observed_quiet_ms: covered }]);
    }
    return result('I3', 'unknown', `nothing was dispatched after the boot, but the bundle covers only ${Number.isFinite(covered) ? Math.round(covered / 1000) + ' s' : 'an unknown span'} after it (need ${I3_MIN_QUIET_MS / 1000} s)`, [ctx]);
  }
  if (ambiguous.length) return result('I3', 'unknown', `${ambiguous.length} post-boot dispatch(es) whose cause is unrecorded, or pre-boot but after owner input with no recorded link to it`, [ctx, ...ambiguous.map(show)]);
  const evidence = [ctx, ...nodes.map(show)];
  if (boot.source.endsWith('(approximate)')) {
    return result('I3', 'pass', `${nodes.length} post-boot dispatches all trace to owner input (boot instant approximated by restart_requested_at)`, evidence);
  }
  return result('I3', 'pass', `${nodes.length} post-boot dispatches all trace to owner input`, evidence);
}

// ---------------------------------------------------------------- I4

// Citations on a revision (Revision internal/types/texture.go:97-171):
//  - structured (current writer): body_doc (:131) source_ref nodes carry
//    attrs.source_entity_id (internal/texturedoc/schema.go:279-300) naming an
//    entry of source_entities (:136; texturedoc.SourceEntity schema.go:33-42:
//    source_entity_id, target {kind, uri, id} :44-49, selectors
//    [{kind, data}] :51-54, reader_snapshot). The source is target.uri or
//    target.id (tools_texture.go:620-628). The quote is a text_quote selector's
//    data.exact (tools_texture.go:216, 1361; sourcecontract/selector.go:7).
//  - legacy: citations (:141) is a []types.Citation {id, type, value, label}
//    (texture.go:193-205). value names the source; the type has no quote
//    field, so a legacy citation names no quote unless the item carries
//    quote / text_quote.
// The source body, when the bundle carries one: reader_snapshot.text_content
// (proxy/platform_publish.go:333-347), .text or .content
// (tools_texture.go:630-637); truncated snapshots and the source graph
// object's body (textureSourceEntityObjectResponse.body texture.go:228, which
// may hold a summary) can confirm a quote but never refute one.
const normText = (s) =>
  String(s ?? '')
    .normalize('NFKC')
    .replace(/[‘’‚‛′]/g, "'")
    .replace(/[“”„‟″]/g, '"')
    .replace(/[‐-―−]/g, '-')
    .replace(/ /g, ' ')
    .toLowerCase()
    .replace(/\s+/g, ' ')
    .trim();
const isTextQuoteKind = (k) => ['text_quote', 'textquote', 'text_quote_selector', 'textquoteselector'].includes(String(k ?? '').toLowerCase().replace(/[^a-z_]/g, ''));

function walkSourceRefs(node, out, refs) {
  if (!node || typeof node !== 'object') return;
  if (node.type === 'source_ref') {
    // Cut 2: a citation written by choir.Write carries a Ref {doc, version,
    // cid, hash, quote} naming a capture block; older ones name a source
    // entity.
    if (refs && node.attrs?.ref && typeof node.attrs.ref === 'object') refs.push({ anchor: str(node.attrs.id), ...node.attrs.ref });
    else out.push(str(node.attrs?.source_entity_id));
  }
  if (Array.isArray(node.content)) for (const c of node.content) walkSourceRefs(c, out, refs);
}

function entityQuote(entity) {
  for (const s of entity?.selectors || []) {
    const k = s.kind ?? s.selector_kind;
    if (!isTextQuoteKind(k)) continue;
    const q = str(s.data?.exact ?? s.data?.text_quote ?? s.text_quote ?? s.exact);
    if (q) return q;
  }
  return '';
}

function entityBodies(entity, rev) {
  const full = [];
  const partial = [];
  const snap = entity?.reader_snapshot;
  if (snap && typeof snap === 'object') {
    const truncated = snap.truncated === true || entity?.reader_snapshot_status?.truncated === true;
    for (const k of ['text_content', 'text', 'content']) {
      if (typeof snap[k] === 'string' && snap[k].trim()) (truncated ? partial : full).push({ from: `reader_snapshot.${k}`, text: snap[k] });
    }
  }
  for (const obj of rev?.source_entity_objects || []) {
    if (str(obj.legacy_source_entity_id) === str(entity?.source_entity_id) && typeof obj.body === 'string' && obj.body.trim()) {
      partial.push({ from: 'source_entity_objects.body', text: obj.body });
    }
  }
  return { full, partial };
}

export function checkI4(bundle) {
  const revs = layer(bundle, 'revisions');
  if (!revs.usable) return result('I4', 'unknown', `revisions layer unserved (${revs.note})`);
  if (revs.items.length === 0) return result('I4', 'unknown', 'revisions layer is empty: no citation to check');
  const fails = [];
  const unknowns = [];
  const passes = [];
  let total = 0;
  // Capture blocks by capture id (the bundle's captures layer, Cut 2 A5).
  const caps = layer(bundle, 'captures');
  const captureBlocks = new Map();
  for (const c of caps.usable ? caps.items : []) captureBlocks.set(str(c.capture_id), new Map((c.blocks || []).map((b) => [str(b.id), b])));
  for (const rev of revs.items) {
    const rid = str(rev.revision_id);
    const body = parseMaybeJSON(rev.body_doc) || (typeof rev.body_doc === 'object' ? rev.body_doc : null);
    const entities = parseMaybeJSON(rev.source_entities) || (Array.isArray(rev.source_entities) ? rev.source_entities : []);
    const byID = new Map((Array.isArray(entities) ? entities : []).map((e) => [str(e.source_entity_id), e]));
    const refs = [];
    const blockRefs = [];
    if (body) walkSourceRefs(body.doc || body, refs, blockRefs);
    for (const r of blockRefs) {
      total++;
      const ev = { revision_id: rid, anchor: r.anchor, doc: str(r.doc), cid: str(r.cid) };
      if (!str(r.doc) || !str(r.cid)) { fails.push({ ...ev, why: 'Ref names no capture or block' }); continue; }
      if (!str(r.quote)) { fails.push({ ...ev, why: 'Ref carries no quote' }); continue; }
      const blocks = captureBlocks.get(str(r.doc));
      if (!blocks) { unknowns.push({ ...ev, why: caps.usable ? 'cited capture absent from the captures layer' : 'captures layer unserved' }); continue; }
      const block = blocks.get(str(r.cid));
      if (!block) { fails.push({ ...ev, why: 'cited block id not in the capture' }); continue; }
      const segments = String(r.quote).split(/\.\.\.|…/).map(normText).filter(Boolean);
      const text = normText(block.text);
      let at = 0;
      const inBlock = segments.every((seg) => { const i = text.indexOf(seg, at); if (i < 0) return false; at = i + seg.length; return true; });
      if (inBlock) passes.push({ ...ev, found_in: 'capture block' });
      else fails.push({ ...ev, quote: r.quote, why: 'quote not found in the cited capture block' });
    }
    for (const sid of [...new Set(refs)]) {
      total++;
      const ev = { revision_id: rid, source_entity_id: sid };
      const ent = byID.get(sid);
      if (!sid || !ent) {
        fails.push({ ...ev, why: 'source_ref names no source entity in source_entities' });
        continue;
      }
      const source = str(ent.target?.uri) || str(ent.target?.id);
      const quote = entityQuote(ent);
      if (!source) {
        fails.push({ ...ev, why: 'citation names no source (target.uri and target.id empty)' });
        continue;
      }
      if (!quote) {
        const kinds = (ent.selectors || []).map((s) => s.kind ?? s.selector_kind);
        fails.push({ ...ev, source, selectors: kinds, why: 'citation names no quote (no text_quote selector with data.exact)' });
        continue;
      }
      const { full, partial } = entityBodies(ent, rev);
      const q = normText(quote);
      const hit = [...full, ...partial].find((b) => normText(b.text).includes(q));
      if (hit) passes.push({ ...ev, source, found_in: hit.from });
      else if (full.length) fails.push({ ...ev, source, quote, searched: full.map((b) => b.from), why: 'quote not found in the source body' });
      else unknowns.push({ ...ev, source, quote, why: partial.length ? 'only a partial body (truncated snapshot or summary); quote not found there' : 'bundle carries no source body' });
    }
    const legacy = parseMaybeJSON(rev.citations) || (Array.isArray(rev.citations) ? rev.citations : []);
    for (const c of Array.isArray(legacy) ? legacy : []) {
      total++;
      const ev = { revision_id: rid, legacy_citation_id: str(c.id) };
      const source = str(c.value);
      const quote = str(c.quote ?? c.text_quote);
      if (!source) fails.push({ ...ev, why: 'legacy citation names no source (value empty)' });
      else if (!quote) fails.push({ ...ev, source, why: 'legacy citation names no quote (types.Citation has no quote field)' });
      else unknowns.push({ ...ev, source, quote, why: 'legacy citation: bundle carries no source body' });
    }
  }
  if (fails.length) return result('I4', 'fail', `${fails.length} of ${total} citations lack a source or quote, or quote text absent from the source body`, fails);
  if (total === 0) return result('I4', 'unknown', `no citations in ${revs.items.length} revisions: the rule read nothing`);
  if (unknowns.length) return result('I4', 'unknown', `${unknowns.length} of ${total} citations could not be checked against a source body`, [...unknowns, ...passes]);
  return result('I4', 'pass', `all ${total} citations name a source and a quote found in its body`, passes);
}

// ---------------------------------------------------------------- I5

// desk.cell / desk.egress run event payloads (bundle format, "Run event kinds
// added in Cut 0"): cell {desk_session, worker_pid, run_id, started_at,
// finished_at, waited_for_run, ...}; egress {request_id, response_request_id,
// verb, desk_session, worker_pid, run_id, started_at, finished_at, is_error}.
const overlaps = (a, b) => a.s < b.f && b.s < a.f;

export function checkI5(bundle) {
  const cells = deskItems(bundle, 'cells', 'desk.cell');
  const egress = deskItems(bundle, 'egress', 'desk.egress');
  const halves = {};
  const fails = [];
  const evidence = [];

  // request-id half
  if (!egress.usable) halves.request_id = { verdict: 'unknown', reason: 'egress layer unserved' };
  else if (egress.items.length === 0) halves.request_id = { verdict: 'unknown', reason: 'no egress calls in bundle' };
  else {
    let bad = 0;
    for (const e of egress.items) {
      // An async search's end (Cut 2) is a host event, not a call.
      if (str(e.verb) === 'search_done') continue;
      const req = str(e.request_id);
      const resp = str(e.response_request_id);
      const ev = { event_id: str(e.event_id) || undefined, run_id: str(e.run_id), desk_session: str(e.desk_session), request_id: req, response_request_id: resp, verb: str(e.verb) };
      if (!req) {
        bad++;
        fails.push({ ...ev, why: 'egress call carries no request_id' });
      } else if (resp === req) {
        evidence.push({ ...ev, ok: true });
      } else if (!resp && e.is_error === true) {
        evidence.push({ ...ev, ok: true, clean_error: str(e.error) });
      } else {
        bad++;
        fails.push({ ...ev, why: resp ? "result frame carries another caller's request id" : 'result without a request id and not an error' });
      }
    }
    halves.request_id = bad
      ? { verdict: 'fail', reason: `${bad} of ${egress.items.length} egress results do not carry their caller's request id` }
      : { verdict: 'pass', reason: `${egress.items.length} egress results each carry their caller's request id` };
  }

  // one-worker half
  const cellIv = cells.items.map((c) => ({ c, s: ts(c.started_at), f: ts(c.finished_at), sess: str(c.desk_session), pid: c.worker_pid }));
  if (!cells.usable) halves.one_worker = { verdict: 'unknown', reason: 'cells layer unserved' };
  else if (cells.items.length === 0) halves.one_worker = { verdict: 'unknown', reason: 'no cells in bundle' };
  else {
    let bad = 0;
    let incomplete = 0;
    const bySess = new Map();
    for (const x of cellIv) {
      if (!x.sess || !Number.isFinite(x.s) || !Number.isFinite(x.f)) {
        incomplete++;
        continue;
      }
      if (!bySess.has(x.sess)) bySess.set(x.sess, []);
      bySess.get(x.sess).push(x);
    }
    for (const [sess, list] of bySess) {
      list.sort((a, b) => a.s - b.s);
      for (let i = 0; i < list.length; i++) {
        for (let j = i + 1; j < list.length && list[j].s < list[i].f; j++) {
          if (!overlaps(list[i], list[j])) continue;
          bad++;
          fails.push({ desk_session: sess, why: list[i].pid !== list[j].pid ? 'two workers on one desk at once' : 'two cells on one desk session at once', cells: [list[i], list[j]].map((x) => ({ event_id: str(x.c.event_id) || undefined, run_id: str(x.c.run_id), worker_pid: x.pid, started_at: x.c.started_at, finished_at: x.c.finished_at })) });
        }
      }
    }
    // an egress call must come from the worker of the cell that holds its session
    for (const e of egress.items) {
      const s = ts(e.started_at);
      const sess = str(e.desk_session);
      const holder = (bySess.get(sess) || []).find((x) => x.s <= s && s <= x.f);
      if (holder && e.worker_pid != null && holder.pid != null && e.worker_pid !== holder.pid) {
        bad++;
        fails.push({ desk_session: sess, why: 'egress from a worker other than the one holding the desk', request_id: str(e.request_id), egress_worker_pid: e.worker_pid, cell_worker_pid: holder.pid });
      }
    }
    if (bad) halves.one_worker = { verdict: 'fail', reason: `${bad} overlap(s) of workers or cells on one desk session` };
    else if (incomplete === cells.items.length) halves.one_worker = { verdict: 'unknown', reason: 'cells carry no desk_session/started_at/finished_at' };
    else halves.one_worker = { verdict: 'pass', reason: `${cells.items.length - incomplete} cells over ${bySess.size} desk sessions, one worker and one cell at a time` };
  }

  // load: concurrent egress within one cell, and two runs waiting on one desk
  let concurrentPairs = 0;
  const egIv = egress.items.map((e) => ({ e, s: ts(e.started_at), f: ts(e.finished_at), sess: str(e.desk_session), run: str(e.run_id) }));
  for (let i = 0; i < egIv.length; i++) {
    for (let j = i + 1; j < egIv.length; j++) {
      const a = egIv[i];
      const b = egIv[j];
      if (a.sess && a.sess === b.sess && a.run === b.run && overlaps(a, b)) {
        const cell = cellIv.find((x) => x.sess === a.sess && x.s <= Math.min(a.s, b.s) && Math.max(a.f, b.f) <= x.f);
        if (cell) concurrentPairs++;
      }
    }
  }
  const waits = cells.items.filter((c) => str(c.waited_for_run) && str(c.waited_for_run) !== str(c.run_id));
  const load = {
    concurrent_egress_pairs_in_one_cell: concurrentPairs,
    runs_waiting_on_a_busy_desk: waits.map((c) => ({ run_id: str(c.run_id), waited_for_run: str(c.waited_for_run), desk_session: str(c.desk_session), waited_ms: c.waited_ms })),
    drill_load: bundle?.drill?.load ?? null,
  };
  // A desk wait cannot appear on the product path: runs of one desk agent
  // are already serialized (C0 acceptance trace c0-accept-1: the second
  // research run started only after the first finished), so the session
  // wait is a defence the tests cover. Waits are reported, not required.
  load.appeared = concurrentPairs > 0;
  evidence.push({ load });

  const hv = [halves.request_id.verdict, halves.one_worker.verdict];
  let verdict;
  let reason = `request_id: ${halves.request_id.reason}; one_worker: ${halves.one_worker.reason}`;
  if (hv.includes('fail')) verdict = 'fail';
  else if (hv.includes('unknown')) verdict = 'unknown';
  else if (!load.appeared) {
    verdict = 'unknown';
    reason += `; load never appeared (no concurrent egress pair in one cell)`;
  } else {
    verdict = 'pass';
    reason += `; under load (${concurrentPairs} concurrent egress pairs, ${waits.length} desk waits)`;
  }
  return result('I5', verdict, reason, verdict === 'fail' ? [...fails, { load }] : evidence, { halves, load });
}

// ---------------------------------------------------------------- driver

// ---------------------------------------------------------------- I6, I7 (Cut 1)

// I6: the records' own bookkeeping. Every inbox's sequence numbers are
// gapless from 1, its head equals its last record, its cursor is never past
// its head; a conversation has at most one open round; only the round's
// recipient's final Report or a system closure closes a round (Cut 1
// invariants 1-4 and 8, read from the records and inboxes layers).
export function checkI6(bundle) {
  const recL = layer(bundle, 'records');
  const inL = layer(bundle, 'inboxes');
  if (!recL.usable || !inL.usable) return result('I6', 'unknown', `layers unserved: ${[recL, inL].filter((l) => !l.usable).map((l) => l.name).join(', ')}`);
  if (recL.items.length === 0) return result('I6', 'unknown', 'no records: the rule read nothing');
  const fails = [];
  const byInbox = new Map();
  for (const r of recL.items) {
    const k = str(r.inbox);
    if (!byInbox.has(k)) byInbox.set(k, []);
    byInbox.get(k).push(r);
  }
  for (const [inbox, recs] of byInbox) {
    const seqs = recs.map((r) => Number(r.inbox_seq)).sort((a, b) => a - b);
    seqs.forEach((q, i) => { if (q !== i + 1) fails.push({ inbox, why: `sequence ${seqs.join(',')} is not gapless from 1` }); });
    const state = inL.items.find((i) => str(i.inbox) === inbox);
    const last = seqs[seqs.length - 1] || 0;
    if (state && Number(state.head) !== last) fails.push({ inbox, why: `head ${state.head} != last record ${last}` });
    if (state && Number(state.cursor) > Number(state.head)) fails.push({ inbox, why: `cursor ${state.cursor} past head ${state.head}` });
  }
  const byConv = new Map();
  for (const r of recL.items) {
    const k = str(r.conversation_id);
    if (!k) continue;
    if (!byConv.has(k)) byConv.set(k, []);
    byConv.get(k).push(r);
  }
  for (const [conv, recs] of byConv) {
    recs.sort((a, b) => ts(a.provenance?.committed_at) - ts(b.provenance?.committed_at));
    let open = null; // {round, caster, recipient}
    for (const r of recs) {
      const isCast = str(r.directive?.subtype) === 'cast';
      const round = Number(r.round) || 0;
      if (isCast && !open) open = { round, caster: str(r.sender_inbox), recipient: str(r.inbox) };
      else if (isCast && open && round !== open.round) fails.push({ conversation: conv, record_id: str(r.record_id), why: `Cast into round ${round} while round ${open.round} is open` });
      if (r.final) {
        const system = str(r.emitter_kind) === 'system';
        if (!open) fails.push({ conversation: conv, record_id: str(r.record_id), why: 'final Report with no open round' });
        else if (!system && str(r.sender_inbox) !== open.recipient) fails.push({ conversation: conv, record_id: str(r.record_id), why: `final Report from ${r.sender_inbox}, not the round's recipient ${open.recipient}` });
        open = null;
      }
    }
  }
  for (const state of inL.items) {
    const seen = new Set();
    for (const round of state.open_rounds || []) {
      const k = str(round.conversation_id);
      if (seen.has(k)) fails.push({ inbox: str(state.inbox), why: `conversation ${k} has two open rounds` });
      seen.add(k);
    }
  }
  if (fails.length) return result('I6', 'fail', `${fails.length} records bookkeeping violation(s)`, fails);
  return result('I6', 'pass', `${recL.items.length} records over ${byInbox.size} inboxes: sequences gapless, heads equal their last record, cursors within heads, one open round per conversation, rounds closed by their recipient or the system`);
}

// I7: one authority per route. The conversations on records (Texture with
// research and with management) mint no packet and no work item, and no
// engineering record is addressed to texture:* (Cut 1 routing). The census
// layer (GET /api/runtime/census) is the authority for the retired
// work-state objects: an unserved census is missing evidence, so the rule
// is unknown rather than read off the trajectory snapshot, which stops
// carrying them in C1e. Route-scoped until C1e B4, when any census object
// fails.
const RECORDS_ROUTE = (a, b) => {
  const da = str(a).split(':')[0];
  const db = str(b).split(':')[0];
  const pair = [da, db].sort().join('|');
  return pair === 'research|texture' || pair === 'management|texture';
};
const CENSUS_WORK_ITEM = 'choir.work_item';
const CENSUS_PACKET = 'choir.worker_update';
export function checkI7(bundle) {
  const censusL = layer(bundle, 'census');
  const recL = layer(bundle, 'records');
  if (!censusL.usable || !recL.usable) return result('I7', 'unknown', `layers unserved: ${[censusL, recL].filter((l) => !l.usable).map((l) => l.name).join(', ')}`);
  const census = censusL.items[0] || {};
  const objects = Array.isArray(census.objects) ? census.objects : [];
  const fails = [];
  for (const o of objects) {
    const kind = str(o.kind);
    const from = str(o.from);
    const to = str(o.to);
    if (kind === CENSUS_PACKET && RECORDS_ROUTE(from, to)) fails.push({ canonical_id: str(o.canonical_id), kind, from, to, why: 'packet on a records route' });
    if (kind === CENSUS_WORK_ITEM && (to.startsWith('research:') || to.startsWith('management:'))) fails.push({ canonical_id: str(o.canonical_id), kind, to, why: 'work item on a records route' });
  }
  for (const r of recL.items) {
    if (str(r.sender_inbox).startsWith('engineering') && str(r.inbox).startsWith('texture:')) fails.push({ record_id: str(r.record_id), why: 'engineering record addressed to texture:*' });
  }
  if (fails.length) return result('I7', 'fail', `${fails.length} route authority violation(s)`, fails);
  if (census.truncated) return result('I7', 'unknown', `census truncated at ${objects.length} objects; no violation among those listed`);
  return result('I7', 'pass', `census lists ${objects.length} retired work-state object(s), none on a records route; no engineering record to texture:*`);
}

export function checkBundle(bundle) {
  const rules = [checkI1, checkI2, checkI3, checkI4, checkI5, checkI6, checkI7].map((fn) => {
    try {
      return fn(bundle);
    } catch (err) {
      return result(fn.name.replace('check', ''), 'unknown', `checker error: ${err?.message || err}`);
    }
  });
  return rules;
}

export function readBundle(path) {
  const text = readFileSync(path, 'utf8');
  const bundle = JSON.parse(text);
  if (!bundle || typeof bundle !== 'object' || Array.isArray(bundle)) throw new Error('bundle is not a JSON object');
  if (bundle.schema !== BUNDLE_SCHEMA) throw new Error(`bundle schema ${JSON.stringify(bundle.schema)} is not ${BUNDLE_SCHEMA}`);
  if (!bundle.layers || typeof bundle.layers !== 'object') throw new Error('bundle has no layers object');
  return { bundle, sha256: createHash('sha256').update(text).digest('hex') };
}

// Exit 1 on any fail, or when a required rule (--require) did not pass: an
// acceptance run names the rules it rests on, and a required rule that is
// unknown or not defined is missing evidence, which must not read as a pass
// (C1e B0). 3 when no rule passed (every verdict unknown or not defined:
// the bundle proved nothing, e.g. an export whose layers are all unserved);
// 0 otherwise.
export function exitCode(rules, required = []) {
  if (rules.some((r) => r.verdict === 'fail')) return 1;
  for (const name of required) {
    const r = rules.find((x) => x.rule === name);
    if (!r || r.verdict !== 'pass') return 1;
  }
  if (!rules.some((r) => r.verdict === 'pass')) return 3;
  return 0;
}

function main(argv) {
  const args = argv.slice(2);
  let jsonOut = '';
  let required = [];
  const pos = [];
  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--json') jsonOut = args[++i] || '';
    else if (args[i] === '--require') required = String(args[++i] || '').split(',').map((x) => x.trim()).filter(Boolean);
    else pos.push(args[i]);
  }
  const known = new Set(['I1', 'I2', 'I3', 'I4', 'I5', 'I6', 'I7']);
  if (pos.length !== 1 || required.some((r) => !known.has(r))) {
    process.stderr.write('usage: node scripts/reeng_invariants.mjs <bundle.json> [--json <out>] [--require I2,I3,I7]\n');
    return 2;
  }
  let loaded;
  try {
    loaded = readBundle(pos[0]);
  } catch (err) {
    process.stderr.write(`reeng_invariants: cannot read bundle ${pos[0]}: ${err.message}\n`);
    return 2;
  }
  const rules = checkBundle(loaded.bundle);
  for (const r of rules) {
    const req = required.includes(r.rule) && r.verdict !== 'pass' ? ' [required]' : '';
    if (r.verdict === 'not_defined') process.stdout.write(`${r.rule} not yet defined (${r.reason})${req}\n`);
    else process.stdout.write(`${r.rule} ${r.verdict.padEnd(7)} ${r.reason}${req}\n`);
  }
  if (jsonOut) {
    const report = {
      schema: REPORT_SCHEMA,
      bundle: pos[0],
      bundle_sha256: loaded.sha256,
      trajectory_id: loaded.bundle.trajectory_id || '',
      checked_at: new Date().toISOString(),
      required,
      rules,
    };
    writeFileSync(jsonOut, JSON.stringify(report, null, 2) + '\n');
  }
  return exitCode(rules, required);
}

if (import.meta.url === pathToFileURL(process.argv[1] || '').href) {
  process.exitCode = main(process.argv);
}
