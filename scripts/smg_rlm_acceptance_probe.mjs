#!/usr/bin/env node
// SMG station deployed acceptance probe: the management desk is a full RLM —
// exactly one tool (desk_go_eval), all capability in-cell via choir.* verbs.
//
// Drives one prompt-bar trajectory that asks Texture to open a persistent
// Management activation whose objective exercises the cutover verbs:
//   1. an UNBOUND choir.ReportPacket first (no delivered lifecycle control
//      yet) — must be refused into the cell (no-binding refusal leg);
//   2. a choir.Cast to engineering for a trivial objective, then
//      choir.CancelAssignment on it — durable revoke + executor ack
//      (cancellation leg);
//   3. a bound choir.ReportPacket back to the owning Texture control with
//      work_disposition completed — the typed producer report surviving the
//      cutover (bound-report leg).
//
// Watches trajectory + run events for: a persistent-management update
// (producer report bound to a delivered control), an assignment open +
// cancel/revoke record, and the refusal text surfaced in a report.
// Exit 0 when the bound management report lands; other legs strengthen.

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

const prompt =
  `Operator-authorized SMG management-RLM acceptance probe (marker ${MARKER}). ` +
  `Task: exercise the management desk's in-cell lifecycle surface. In your ` +
  `ApplyTexture turn, open a persistent Management activation via controls[] ` +
  `with exactly this shape (actions must be non-empty): ` +
  `choir.ApplyTexture({"controls":[{"open_persistent_super":true,` +
  `"objective":"SMG RLM acceptance probe ${MARKER}","packet":{"kind":` +
  `"execution_request","summary":"Run the SMG cutover legs","actions":[` +
  `{"type":"probe","objective":"exercise report/cancel/report legs"}]}}],` +
  `"work_disposition":"open"}). ` +
  `The management activation must do all of the following IN ORDER inside ` +
  `desk_go_eval cells (choir.ReportPacket takes THREE args: toDesk, packet, ` +
  `resolverID): (a) BEFORE any bound report, call choir.ReportPacket("texture", ` +
  `{"kind":"execution_result","summary":"unbound probe"}, "") and record the ` +
  `refusal text; (b) choir.Cast("engineering", "SMG no-op: create a file ` +
  `smg-probe.txt containing the marker ${MARKER}", {}) then ` +
  `choir.CancelAssignment(<assignment id from the cast result>, "probe") and ` +
  `record the revoke result; (c) choir.ReportPacket back to texture with ` +
  `{"kind":"execution_result","summary":"SMG legs complete: unbound=<refusal ` +
  `text>; cancel=<revoke outcome>","work_disposition":"completed"}, "". ` +
  `The document is disposable. Do not edit the document body.`;

const submit = await api('/api/prompt-bar', 'POST', { text: prompt, command_id: `smg-rlm-${MARKER}` });
let TRAJ = submit.body?.trajectory_id;
if (submit.status !== 200 && submit.status !== 202) {
  recordLeg('prompt_bar_submit', { ok: false, status: submit.status, body: submit.body });
  finish(1, 'prompt submit failed');
}
recordLeg('prompt_bar_submit', { ok: true, trajectory: TRAJ, doc: submit.body?.doc_id });

const deadline = t0 + TIMEOUT_MIN * 60 * 1000;
let boundReport = null, cancelSeen = null, unboundRefusal = null, mgmtOpen = null;
const seen = new Set();

while (Date.now() < deadline) {
  if (TRAJ) {
    const ev = await api(`/api/trajectories/${TRAJ}/events?limit=500`);
    for (const e of ev.body?.events || []) {
      const key = `${e.kind}:${e.update_id || ''}:${e.event_id}`;
      if (seen.has(key)) continue;
      seen.add(key);
      const uid = String(e.update_id || '');
      const blob = JSON.stringify(e);
      // Persistent-management activation opened.
      if (!mgmtOpen && (uid.includes('persistent') || blob.includes('open_persistent_super') || blob.includes('management'))) {
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
      // Bound report: a producer report on the lifecycle control.
      if (e.direction === 'producer_report' || uid.includes(':report:') || blob.includes('work_disposition')) {
        boundReport = { ...e };
        recordLeg('bound_producer_report', { update_id: uid, kind: e.kind, direction: e.direction });
      }
    }
    const traj = await api(`/api/trajectories/${TRAJ}`);
    const status = traj.body?.status || traj.body?.trajectory?.status;
    if (boundReport && status && ['settled', 'completed', 'resolved', 'closed'].includes(String(status))) break;
  }
  await new Promise((r) => setTimeout(r, 8000));
}

recordLeg('final', {
  bound_report: boundReport ? true : false,
  cancel_verb: cancelSeen ? true : false,
  unbound_refusal: unboundRefusal ? true : false,
  mgmt_signal: mgmtOpen ? true : false,
});
finish(boundReport ? 0 : 1, boundReport ? null : 'no bound producer report observed');
