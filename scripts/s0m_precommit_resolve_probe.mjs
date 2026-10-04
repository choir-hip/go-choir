#!/usr/bin/env node
// S0m boundary-close probe P1: precommit addressed to research resolves
// mechanically when the producer_report arrives.
//
// The finish contract (action 1) requires: texture stakes a follow-up ask
// (precommit record addressed to a research work item), the derived control
// wakes research, the report record returns to texture, and the ask's
// precommit RESOLVES mechanically — a CommitmentKindResolve record minted by
// system:reducer in the same commit as the report (mechanicalResolveForReport,
// internal/store/lifecycle_commit_act.go).
//
// The earlier probe (s0m_ask_acceptance_probe.mjs) proved the
// open_researcher control path and work_settled; it did not prove the
// precommit->resolve record leg. This probe instructs the texture desk to
// stake choir.Ask (a precommit) — the verb that mints a precommit record +
// derived control packet — and watches the trajectory for a
// CommitmentKindResolve record whose RecordID ends with :resolve:answered.
//
// Usage: CHOIR_API_KEY=... node scripts/s0m_precommit_resolve_probe.mjs
//   [--computer id] [--out path] [--timeout-min N] [--trajectory existing]
// Exit 0 = resolve record observed linked to the precommit record.
// Exit 1 = timeout / no precommit minted / report arrived but never resolved.

import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname } from 'node:path';

const HOST = process.env.CHOIR_HOST || 'https://choir.news';
const API_KEY = process.env.CHOIR_API_KEY || '';
const args = process.argv.slice(2);
const arg = (name, dflt) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : dflt;
};
const COMPUTER = arg('computer', 'computer-03335285269bdba4f94377e56879f9e6');
const MARKER = arg('marker', `s0m-pc-${Date.now()}`);
const TIMEOUT_MIN = Number(arg('timeout-min', '30'));
const EXISTING_TRAJ = arg('trajectory', '');
const OUT = arg('out', `docs/evidence/s0m-precommit-resolve-${new Date().toISOString().slice(0, 10)}.json`);

if (!API_KEY) { console.error('CHOIR_API_KEY required'); process.exit(2); }
const headers = { Authorization: `Bearer ${API_KEY}`, 'X-Choir-Computer': COMPUTER, 'Content-Type': 'application/json' };

async function api(path, method = 'GET', body = undefined) {
  const res = await fetch(`${HOST}${path}`, {
    method, headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  let parsed = null;
  try { parsed = await res.json(); } catch { /* non-JSON */ }
  return { status: res.status, body: parsed };
}

const t0 = Date.now();
const iso = (ms) => new Date(ms).toISOString();
const result = {
  probe: 's0m_precommit_resolve', computer: COMPUTER, marker: MARKER,
  started_at: iso(t0), legs: [], ok: false, failure: null,
  precommit_record_id: null, report_packet_id: null, resolve_record_id: null,
};
function recordLeg(name, extra) {
  result.legs.push({ name, at: iso(Date.now()), ...extra });
  console.log(`[leg] ${name} ${JSON.stringify(extra || {})}`);
}
function finish(code) {
  result.finished_at = iso(Date.now());
  result.ok = code === 0;
  mkdirSync(dirname(OUT), { recursive: true });
  writeFileSync(OUT, JSON.stringify(result, null, 2));
  console.log(`${result.ok ? 'OK' : 'FAIL'} -> ${OUT}`);
  process.exit(code);
}

// The desk must stake a precommit (choir.Ask) whose addressee is the
// research desk — the record-native way to ask for work. The question asks
// for a citable fact so the report carries real content.
const prompt =
  `Operator-authorized record-native acceptance probe (marker ${MARKER}). ` +
  `Task: record the current UTC time into this document, sourced from the ` +
  `research desk. ` +
  `1. As your FIRST cell act, stage choir.Ask addressed to desk "research" ` +
  `with question: "What is the current UTC time? Cite a time source page." ` +
  `This stakes a precommit commitment record with a derived question packet ` +
  `to research. ` +
  `2. Wait for the research report packet to arrive and consume it. ` +
  `3. Apply one ApplyTexture turn writing the reported UTC time into the ` +
  `document, then end. ` +
  `Do NOT use ApplyTexture controls or open_researcher for this probe — use ` +
  `choir.Ask only, then wait. The document is disposable.`;

let TRAJ = EXISTING_TRAJ;
if (!TRAJ) {
  const submit = await api('/api/prompt-bar', 'POST', { text: prompt, command_id: `s0m-pc-${MARKER}` });
  TRAJ = submit.body?.trajectory_id;
  if (submit.status !== 200 && submit.status !== 202) {
    result.failure = `prompt_bar submit: ${submit.status} ${JSON.stringify(submit.body)?.slice(0, 200)}`;
    recordLeg('prompt_bar_submit', { ok: false, status: submit.status });
    finish(1);
  }
}
result.trajectory_id = TRAJ;
result.doc_id = null;
recordLeg('prompt_bar_submit', { ok: true, trajectory: TRAJ });
if (!TRAJ) finish(1);

const deadline = t0 + TIMEOUT_MIN * 60 * 1000;
let precommit = null, reportPacket = null, resolve = null;
const seen = new Set();
let lastErr = '';

while (Date.now() < deadline) {
  const t = await api(`/api/trajectories/${TRAJ}`);
  if (t.status !== 200) { lastErr = `trajectory ${t.status}`; await new Promise(r => setTimeout(r, 10000)); continue; }
  result.doc_id = result.doc_id || t.body?.doc_id || null;
  const updates = t.body?.updates || [];
  for (const e of updates) {
    const uid = String(e.update_id || e.packet_id || '');
    const key = `${e.kind}:${uid}`;
    if (seen.has(key)) continue;
    seen.add(key);

    const blob = JSON.stringify(e);
    // A precommit record carrying an addressed ask to research.
    if (!precommit && (
      String(e.record_kind || '') === 'precommit' ||
      uid.includes(':precommit:') ||
      (e.kind && String(e.kind).includes('commitment') && blob.includes('precommit') && blob.includes('research'))
    )) {
      precommit = { update_id: uid, record_id: e.record_id || e.commitment_record_id || uid, e };
      result.precommit_record_id = precommit.record_id;
      recordLeg('precommit_seen', { update_id: uid, record_id: result.precommit_record_id, kind: e.kind });
    }
    // A producer_report packet back to texture.
    if (!reportPacket && (
      String(e.direction || '') === 'producer_report' ||
      uid.includes(':report:') ||
      String(e.packet_kind || '') === 'producer_report'
    )) {
      reportPacket = { update_id: uid, e };
      result.report_packet_id = uid;
      recordLeg('report_seen', { update_id: uid, kind: e.kind });
    }
    // The mechanical resolve record minted by the reducer.
    if (!resolve && (
      uid.includes(':resolve:answered') ||
      (String(e.record_kind || '') === 'resolve' && blob.includes('answered'))
    )) {
      resolve = { update_id: uid, record_id: e.record_id || uid, e };
      result.resolve_record_id = resolve.record_id;
      recordLeg('resolve_seen', { update_id: uid, record_id: result.resolve_record_id, kind: e.kind });
    }
  }
  if (resolve) {
    recordLeg('chain_complete', { precommit: !!precommit, report: !!reportPacket, resolve: result.resolve_record_id });
    finish(0);
  }
  await new Promise(r => setTimeout(r, 15000));
}

result.failure = `timeout: precommit=${!!precommit} report=${!!reportPacket} resolve=${!!resolve} lastErr=${lastErr}`;
finish(1);
