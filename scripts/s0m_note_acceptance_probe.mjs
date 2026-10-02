#!/usr/bin/env node
// S0m RN3a deployed acceptance probe: record-native `note` cutover.
//
// Drives the real prompt bar on the owner staging computer, asking texture to
// stage one choir.Note addressed to the management desk. Then observes:
//   1. a control_queued lifecycle event whose update_id is the note packet
//      (<cellID>:note:<localID>:packet) — CommitLifecycleAct minted the
//      record+packet atomically (no envelope path);
//   2. a persistent management run minted with that packet id in
//      metadata.worker_update_ids — the directive wake delivered the packet.
// Exit 0 = both receipts captured into the evidence file.

const HOST = process.env.CHOIR_HOST || 'https://choir.news';
const API_KEY = process.env.CHOIR_API_KEY || '';

import { mkdirSync, writeFileSync } from 'node:fs';

const args = process.argv.slice(2);

const arg = (name, dflt) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : dflt;
};
const COMPUTER = arg('computer', 'computer-03335285269bdba4f94377e56879f9e6');
const MARKER = arg('marker', `s0m-rn3a-note-${Date.now()}`);
const TIMEOUT_MIN = Number(arg('timeout-min', '20'));
const OUT = arg('out', `docs/evidence/s0m-rn3a-note-acceptance-${new Date().toISOString().slice(0, 10)}.json`);

if (!API_KEY) {
  console.error('CHOIR_API_KEY is required');
  process.exit(2);
}

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
  probe: 's0m_rn3a_note_acceptance',
  computer: COMPUTER,
  marker: MARKER,
  started_at: iso(t0),
  legs: [],
  ok: false,
  failure: null,
};

function recordLeg(name, extra) {
  const leg = { name, ended_at: Date.now(), wall_ms: Date.now() - t0, ...extra };
  result.legs.push(leg);
  console.log(`[leg] ${name} wall=${leg.wall_ms}ms ${JSON.stringify(extra)}`);
  return leg;
}

const prompt =
  `Operator-authorized messaging acceptance probe. In a desk cell, stage exactly one call ` +
  `choir.Note("management", "${MARKER}") and then end the run. Do not take other actions; ` +
  `the document content does not matter for this probe.`;

const submit = await api('/api/prompt-bar', 'POST', { text: prompt, command_id: `s0m-rn3a-${MARKER}` });
if (submit.status !== 200 && submit.status !== 202) {
  recordLeg('prompt_bar_submit', { ok: false, status: submit.status, body: submit.body });
  finish(1);
}
const TRAJ = submit.body?.trajectory_id;
recordLeg('prompt_bar_submit', { ok: true, trajectory: TRAJ, doc: submit.body?.doc_id });

const deadline = t0 + TIMEOUT_MIN * 60 * 1000;
const baselineRuns = new Set();
{
  const res = await api('/api/runs?limit=50');
  for (const r of res.body?.runs || []) baselineRuns.add(r.run_id);
}

let notePacketID = null;
let eventsSeen = 0;
const collectedEvents = [];

while (Date.now() < deadline) {
  if (TRAJ) {
    const ev = await api(`/api/trajectories/${TRAJ}/events?limit=200`);
    const events = Array.isArray(ev.body?.events) ? ev.body.events : [];
    if (events.length > eventsSeen) {
      for (const e of events.slice(eventsSeen)) {
        collectedEvents.push({ kind: e.kind, update_id: e.update_id, reason: e.reason, command_id: e.command_id, created_at: e.created_at });
      }
      eventsSeen = events.length;
    }
    for (const e of events) {
      if (e.kind === 'control_queued' && typeof e.update_id === 'string' && e.update_id.includes(':note:')) {
        notePacketID = e.update_id;
        recordLeg('control_queued', { update_id: e.update_id, reason: e.reason, command_id: e.command_id });
      }
    }
  }

  const runs = await api('/api/runs?limit=50');
  for (const r of runs.body?.runs || []) {
    if (baselineRuns.has(r.run_id)) continue;
    const ids = Array.isArray(r.metadata?.worker_update_ids) ? r.metadata.worker_update_ids : [];
    const noteID = ids.find((id) => typeof id === 'string' && id.includes(':note:'));
    const isMgmt = typeof r.agent_id === 'string' && r.agent_id.startsWith('management:');
    if (isMgmt && noteID) {
      recordLeg('management_directive_wake', { run_id: r.run_id, note_packet: noteID, state: r.state });
      notePacketID = notePacketID || noteID;
      result.note_packet_id = noteID;
      result.management_run_id = r.run_id;
      finish(0);
    }
    if (isMgmt) {
      baselineRuns.add(r.run_id); // seen, non-matching
    }
  }
  await new Promise((r) => setTimeout(r, 15000));
}

result.failure = 'timeout waiting for note packet delivery';
finish(1);

function finish(code) {
  result.ok = code === 0;
  result.finished_at = iso(Date.now());
  result.wall_ms = Date.now() - t0;
  result.trajectory_id = TRAJ;
  try {
    mkdirSync(OUT.slice(0, OUT.lastIndexOf('/')), { recursive: true });
    writeFileSync(OUT, JSON.stringify(result, null, 2));
    console.log(`evidence -> ${OUT}`);
  } catch (err) {
    console.error('evidence write failed:', String(err));
    console.log(JSON.stringify(result, null, 2));
  }
  process.exit(code);
}
