#!/usr/bin/env node
// S0m RN3c deployed acceptance probe: record-native `escalate` cutover.
//
// Drives the real prompt bar on the owner staging computer with a prompt that
// forces a research leg whose report hits a timeout-adjacent outcome, then —
// in the same trajectory — asks the desk to escalate to management. Watches
// the trajectory lifecycle events for:
//   1. update_queued / control_queued with update_id containing ":escalate:"
//      — CommitLifecycleAct minted record + directive packet in one commit;
//   2. update_delivered / control_delivered — the packet bound to the
//      persistent management desk.
// Exit 0 = queued+delivered for an escalate update observed.

import { mkdirSync, writeFileSync } from 'node:fs';

const HOST = process.env.CHOIR_HOST || 'https://choir.news';
const API_KEY = process.env.CHOIR_API_KEY || '';
const args = process.argv.slice(2);
const arg = (name, dflt) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : dflt;
};
const COMPUTER = arg('computer', 'computer-03335285269bdba4f94377e56879f9e6');
const MARKER = arg('marker', `s0m-rn3c-escalate-${Date.now()}`);
const TIMEOUT_MIN = Number(arg('timeout-min', '25'));
const OUT = arg('out', `docs/evidence/s0m-rn3c-escalate-acceptance-${new Date().toISOString().slice(0, 10)}.json`);

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
  probe: 's0m_rn3c_escalate_acceptance',
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
  `choir.Escalate("management", "${MARKER}") and then end the run. Do not take other actions; ` +
  `the document content does not matter for this probe.`;

const submit = await api('/api/prompt-bar', 'POST', { text: prompt, command_id: `s0m-rn3c-${MARKER}` });
if (submit.status !== 200 && submit.status !== 202) {
  recordLeg('prompt_bar_submit', { ok: false, status: submit.status, body: submit.body });
  finish(1);
}
let TRAJ = submit.body?.trajectory_id;
let DOC = submit.body?.doc_id;
recordLeg('prompt_bar_submit', { ok: true, trajectory: TRAJ, doc: DOC });
const deadline = t0 + TIMEOUT_MIN * 60 * 1000;
let queued = null, delivered = null;
const seen = new Set();

while (Date.now() < deadline) {
  if (TRAJ) {
    const ev = await api(`/api/trajectories/${TRAJ}/events?limit=400`);
    for (const e of ev.body?.events || []) {
      const key = `${e.kind}:${e.update_id || ''}:${e.event_id}`;
      if (seen.has(key)) continue;
      seen.add(key);
      if ((e.kind === 'update_queued' || e.kind === 'control_queued') &&
        typeof e.update_id === 'string' && e.update_id.includes(':escalate:')) {
        queued = e;
        recordLeg('escalate_queued', { update_id: e.update_id, kind: e.kind, command_id: e.command_id });
      }
      if (queued && (e.kind === 'update_delivered' || e.kind === 'control_delivered') &&
        e.update_id === queued.update_id) {
        delivered = e;
        recordLeg('escalate_delivered', { update_id: e.update_id, kind: e.kind });
      }
    }
  }
  if (queued && delivered) {
    result.escalate_update_id = queued.update_id;
    result.delivered_run = delivered.run_id;
    finish(0);
  }
  await new Promise((r) => setTimeout(r, 15000));
}

result.failure = `timeout: queued=${!!queued} delivered=${!!delivered}`;
finish(1);

function finish(code) {
  result.ok = code === 0;
  result.finished_at = iso(Date.now());
  result.wall_ms = Date.now() - t0;
  result.trajectory_id = TRAJ;
  result.doc_id = DOC;
  try {
    mkdirSync(OUT.slice(0, OUT.lastIndexOf('/')), { recursive: true });
    writeFileSync(OUT, JSON.stringify(result, null, 2));
    console.log(`evidence -> ${OUT}`);
  } catch (err) {
    console.error('evidence write failed:', err);
  }
  process.exit(code);
}
