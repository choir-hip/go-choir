#!/usr/bin/env node
// S0m boundary-close probe P3: channel mail addressed to texture:* rejects
// at reduce/cast time — never a durable dead letter.
//
// b18f3baf guards channelCast: an addressed cast whose target is
// "texture" or "texture:<doc>" fails with
//   channel cast: "texture:..." is not reachable by channel mail; use the
//   lifecycle packet path
// A desk cell's choir.Emit -> CastEnvelope -> channelCast surfaces that error
// back into the cell's tool result, which appears on the trajectory.
//
// This probe asks the texture desk to emit an addressed signal to a texture
// target and watches the trajectory for the refusal string. Refusal present
// = proof the dead-letter path is closed on deployed staging.
//
// Usage: CHOIR_API_KEY=... node scripts/s0m_channel_mail_reject_probe.mjs
//   [--computer id] [--out path] [--timeout-min N] [--trajectory id]
// Exit 0 = refusal observed; exit 1 = timeout / emit succeeded (bug!).

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
const MARKER = arg('marker', `s0m-cm-${Date.now()}`);
const TIMEOUT_MIN = Number(arg('timeout-min', '15'));
const EXISTING_TRAJ = arg('trajectory', '');
const OUT = arg('out', `docs/evidence/s0m-channel-mail-reject-${new Date().toISOString().slice(0, 10)}.json`);

if (!API_KEY) { console.error('CHOIR_API_KEY required'); process.exit(2); }
const headers = { Authorization: `Bearer ${API_KEY}`, 'X-Choir-Computer': COMPUTER, 'Content-Type': 'application/json' };

async function api(path, method = 'GET', body = undefined) {
  const res = await fetch(`${HOST}${path}`, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
  let parsed = null; try { parsed = await res.json(); } catch {}
  return { status: res.status, body: parsed };
}

const t0 = Date.now();
const iso = (ms) => new Date(ms).toISOString();
const result = { probe: 's0m_channel_mail_reject', computer: COMPUTER, marker: MARKER, started_at: iso(t0), legs: [], ok: false, failure: null };
function recordLeg(name, extra) { result.legs.push({ name, at: iso(Date.now()), ...extra }); console.log(`[leg] ${name} ${JSON.stringify(extra || {})}`); }
function finish(code) { result.finished_at = iso(Date.now()); result.ok = code === 0; mkdirSync(dirname(OUT), { recursive: true }); writeFileSync(OUT, JSON.stringify(result, null, 2)); console.log(`${result.ok ? 'OK' : 'FAIL'} -> ${OUT}`); process.exit(code); }

// The emit target is the texture profile itself — the exact class the guard
// rejects (texture and texture:*). Instructing the desk to emit makes it
// exercise the real cast path; the refusal must come back as the tool error.
const prompt =
  `Operator-authorized channel-mail rejection probe (marker ${MARKER}). ` +
  `As your first cell act, stage a choir.Emit signal to desk "texture" with ` +
  `kind "ping" and body "${MARKER}". Then, as your second act, write one ` +
  `line into this document reporting exactly what the emit call returned ` +
  `(its result or its error). End after that. The document is disposable.`;

let TRAJ = EXISTING_TRAJ;
if (!TRAJ) {
  const submit = await api('/api/prompt-bar', 'POST', { text: prompt, command_id: `s0m-cm-${MARKER}` });
  TRAJ = submit.body?.trajectory_id;
  if (!TRAJ) {
    for (let i = 0; i < 8 && !TRAJ; i++) {
      const list = await api('/api/trajectories?limit=8');
      const cutoff = Date.now() - 3 * 60 * 1000;
      const trajs = Array.isArray(list.body?.trajectories) ? list.body.trajectories : (Array.isArray(list.body) ? list.body : []);
      for (const t of trajs) {
        if (t.status === 'live' && Date.parse(t.created_at || '') >= cutoff) { TRAJ = t.trajectory_id; break; }
      }
      if (!TRAJ) await new Promise(r => setTimeout(r, 6000));
    }
  }
}
result.trajectory_id = TRAJ;
recordLeg('prompt_bar_submit', { ok: !!TRAJ, trajectory: TRAJ });
if (!TRAJ) finish(1);

const deadline = t0 + TIMEOUT_MIN * 60 * 1000;
let refusal = null, emitSuccess = false;
const REFUSAL_NEEDLE = 'not reachable by channel mail';
const CAST_NEEDLE = 'channel cast';
const seen = new Set();
let lastErr = '';

while (Date.now() < deadline) {
  const t = await api(`/api/trajectories/${TRAJ}`);
  if (t.status !== 200) { lastErr = `trajectory ${t.status}`; await new Promise(r => setTimeout(r, 8000)); continue; }
  result.doc_id = result.doc_id || t.body?.doc_id || null;
  const snapshotBlob = JSON.stringify({
    updates: t.body?.updates, events: t.body?.events,
    head_revision: t.body?.head_revision, document: t.body?.document,
  });
  if (!refusal && snapshotBlob.includes(REFUSAL_NEEDLE)) {
    refusal = { source: 'snapshot', excerpt: snapshotBlob.match(/.{0,160}not reachable by channel mail.{0,120}/s)?.[0] || '' };
    recordLeg('refusal_observed', refusal);
  }
  for (const e of t.body?.updates || []) {
    const key = `${e.kind}:${e.update_id || ''}:${e.seq ?? ''}`;
    if (seen.has(key)) continue;
    seen.add(key);
    const blob = JSON.stringify(e);
    if (!refusal && blob.includes(REFUSAL_NEEDLE)) {
      refusal = { kind: e.kind, update_id: e.update_id, seq: e.seq, excerpt: blob.match(/.{0,120}not reachable by channel mail.{0,80}/s)?.[0] || '' };
      recordLeg('refusal_observed', refusal);
    }
    if (blob.includes(CAST_NEEDLE) && !blob.includes(REFUSAL_NEEDLE)) {
      recordLeg('cast_seen_no_refusal', { kind: e.kind, update_id: e.update_id });
    }
    if ((String(e.direction || '') === 'emit' || String(e.kind || '').includes('emit')) && blob.includes('delivered')) {
      emitSuccess = true;
      recordLeg('emit_delivered_suspicious', { kind: e.kind, update_id: e.update_id });
    }
  }
  if (refusal) {
    if (emitSuccess) { result.failure = 'refusal recorded but an emit also delivered'; finish(1); }
    finish(0);
  }
  await new Promise(r => setTimeout(r, 10000));
}

result.failure = `timeout: refusal=${!!refusal} emitDelivered=${emitSuccess} lastErr=${lastErr}`;
finish(1);
