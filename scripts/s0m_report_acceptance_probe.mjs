#!/usr/bin/env node
// S0m RN3b deployed acceptance probe: record-native `report` cutover.
//
// Drives the real prompt bar on the owner staging computer with a prompt that
// forces a research leg (conductor opens a researcher, the researcher reports
// through choir.Report to texture). Then observes the trajectory lifecycle
// events for the record-native delivery chain:
//   1. update_queued with update_id containing ":report:" — CommitLifecycleAct
//      minted record+producer_report packet in one commit (record-derived ID);
//   2. update_delivered — the packet bound to a texture activation;
//   3. update_applied/update_late or a v2 revision — texture incorporated it.
// Exit 0 = queued+delivered observed (applied/v2 strengthen but queue+deliver
// already proves the RN3b record→packet→wake chain end to end).

import { mkdirSync, writeFileSync } from 'node:fs';

const HOST = process.env.CHOIR_HOST || 'https://choir.news';
const API_KEY = process.env.CHOIR_API_KEY || '';
const args = process.argv.slice(2);
const arg = (name, dflt) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : dflt;
};
const COMPUTER = arg('computer', 'computer-03335285269bdba4f94377e56879f9e6');
const MARKER = arg('marker', `s0m-rn3b-report-${Date.now()}`);
const TIMEOUT_MIN = Number(arg('timeout-min', '30'));
const OUT = arg('out', `docs/evidence/s0m-rn3b-report-acceptance-${new Date().toISOString().slice(0, 10)}.json`);

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
  probe: 's0m_rn3b_report_acceptance',
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
  `Operator-authorized messaging acceptance probe (marker ${MARKER}). ` +
  `Ask research one question it can answer from the web quickly — ` +
  `"what is today's UTC date and one headline from a major news front page" — ` +
  `incorporate the report into a short revision, and end. The document is disposable.`;

const submit = await api('/api/prompt-bar', 'POST', { text: prompt, command_id: `s0m-rn3b-${MARKER}` });
if (submit.status !== 200 && submit.status !== 202) {
  recordLeg('prompt_bar_submit', { ok: false, status: submit.status, body: submit.body });
  finish(1);
}
const TRAJ = submit.body?.trajectory_id;
const DOC = submit.body?.doc_id;
recordLeg('prompt_bar_submit', { ok: true, trajectory: TRAJ, doc: DOC });

const deadline = t0 + TIMEOUT_MIN * 60 * 1000;
let queued = null, delivered = null, applied = null, revCount = 0;
const seen = new Set();

while (Date.now() < deadline) {
  if (TRAJ) {
    const ev = await api(`/api/trajectories/${TRAJ}/events?limit=400`);
    for (const e of ev.body?.events || []) {
      const key = `${e.kind}:${e.update_id || ''}:${e.event_id}`;
      if (seen.has(key)) continue;
      seen.add(key);
      if (e.kind === 'update_queued' && typeof e.update_id === 'string' && e.update_id.includes(':report:')) {
        queued = e;
        recordLeg('update_queued', { update_id: e.update_id, command_id: e.command_id });
      }
      if (queued && e.kind === 'update_delivered' && e.update_id === queued.update_id) {
        delivered = e;
        recordLeg('update_delivered', { update_id: e.update_id });
      }
      if (queued && (e.kind === 'update_applied' || e.kind === 'update_late') && e.update_id === queued.update_id) {
        applied = e;
        recordLeg('update_terminal', { kind: e.kind, update_id: e.update_id });
      }
    }
  }
  if (DOC) {
    const revs = await api(`/api/texture/documents/${DOC}/revisions`);
    const n = Array.isArray(revs.body?.revisions) ? revs.body.revisions.length : 0;
    if (n !== revCount) {
      revCount = n;
      recordLeg('revisions', { count: n });
    }
  }
  if (queued && delivered) {
    result.report_update_id = queued.update_id;
    result.delivered_run = delivered.run_id;
    result.applied = !!applied;
    finish(0);
  }
  await new Promise((r) => setTimeout(r, 15000));
}

result.failure = `timeout: queued=${!!queued} delivered=${!!delivered} applied=${!!applied}`;
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
    console.error('evidence write failed:', String(err));
    console.log(JSON.stringify(result, null, 2));
  }
  process.exit(code);
}
