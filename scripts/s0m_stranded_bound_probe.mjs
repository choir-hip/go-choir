// S0m stranded-bound deployed probe (fix c9180cd3): force the carrier-dies-
// unconsumed wedge and prove the desk recovers instead of deadlocking.
//
// Shape: submit an owner prompt that obliges texture to choir.Ask research.
// When a control packet shows bound to a live research run
// (delivered_to_loop_id=<run>), immediately POST /api/runs/<run>/cancel —
// racing the consume. The carrier terminalizes while still bound. Pre-fix the
// packet stayed bound+pending forever (bound rows are invisible to the pending
// list, so no wake ever re-activated the desk). Post-fix,
// bindTerminalRunOutcome releases the claim and re-drives the desk: the freed
// packet re-enters pending, then rebinds to a fresh run OR terminalizes
// delivery_attempts_exhausted — either is a scored recovery, never silent.
//
// Exit 0 = stranded packet recovered (rebound+consumed, or exhausted).
// Exit 1 = the packet stayed bound to the dead run past the deadline.
import { writeFileSync, mkdirSync } from 'node:fs';

const HOST = process.env.CHOIR_DEPLOYED_BASE_URL || 'https://choir.news';
const API_KEY = process.env.CHOIR_API_KEY;
const arg = (name, dflt) => {
  const i = process.argv.indexOf('--' + name);
  return i > 0 && process.argv[i + 1] ? process.argv[i + 1] : dflt;
};
const COMPUTER = arg('computer', 'computer-03335285269bdba4f94377e56879f9e6');
const MARKER = arg('marker', `s0m-strand-${Date.now()}`);
const EXISTING_TRAJ = arg('trajectory', '');
const BIND_WINDOW_MIN = Number(arg('bind-window-min', '12'));
const RECOVER_MIN = Number(arg('recover-min', '8'));
const OUT = arg('out', `docs/evidence/s0m-stranded-bound-${new Date().toISOString().slice(0, 10)}.json`);

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
const result = { probe: 's0m_stranded_bound', computer: COMPUTER, marker: MARKER, started_at: iso(t0), legs: [], ok: false, failure: null };
function recordLeg(name, extra) {
  const leg = { name, ended_at: Date.now(), wall_ms: Date.now() - t0, ...extra };
  result.legs.push(leg);
  console.log(`[leg] ${name} wall=${leg.wall_ms}ms ${JSON.stringify(extra)}`);
  return leg;
}
function finish(code) {
  result.ok = code === 0;
  result.finished_at = iso(Date.now());
  result.wall_ms = Date.now() - t0;
  try {
    mkdirSync(OUT.slice(0, OUT.lastIndexOf('/')), { recursive: true });
    writeFileSync(OUT, JSON.stringify(result, null, 2));
    console.log(`evidence -> ${OUT}`);
  } catch (e) { console.error('evidence write failed:', e); }
  process.exit(code);
}

const prompt =
  `Operator-authorized stranded-bound acceptance probe (marker ${MARKER}). ` +
  `Task: record the current UTC date into this document. To get it right you ` +
  `MUST ask the research desk — call choir.Ask("research", "What is today's ` +
  `UTC date? Answer in one short line.") — then wait for the reply, write the ` +
  `answer into the document, and end. The document is disposable.`;

let TRAJ = EXISTING_TRAJ;
if (!TRAJ) {
  const submit = await api('/api/prompt-bar', 'POST', { text: prompt, command_id: `s0m-strand-${MARKER}` });
  TRAJ = submit.body?.trajectory_id;
  // The proxy can 502 a slow submit while the trajectory mints server-side.
  // Recover it: the newest live trajectory created in the last ~3 minutes.
  if (!TRAJ) {
    for (let i = 0; i < 8 && !TRAJ; i++) {
      const list = await api('/api/trajectories?limit=8');
      const cutoff = Date.now() - 3 * 60 * 1000;
      for (const t of (list.body?.trajectories || list.body || [])) {
        const ts = Date.parse(t.created_at || '');
        if (t.status === 'live' && ts >= cutoff) { TRAJ = t.trajectory_id; break; }
      }
      if (!TRAJ) await new Promise((r) => setTimeout(r, 6000));
    }
  }
  recordLeg('prompt_bar_submit', { ok: !!TRAJ, trajectory: TRAJ, submitted: true });
} else {
  recordLeg('prompt_bar_submit', { ok: true, trajectory: TRAJ, existing: true });
}
result.trajectory_id = TRAJ;
if (!TRAJ) finish(1);

// Phase 1 — wait for a control packet bound to a live research run, then
// cancel that carrier mid-bind to force the wedge.
const bindDeadline = Date.now() + BIND_WINDOW_MIN * 60 * 1000;
let stranded = null; // { update_id, carrier_run_id }
while (Date.now() < bindDeadline && !stranded) {
  const t = await api(`/api/trajectories/${TRAJ}`);
  for (const u of t.body?.updates || []) {
    if (u.direction !== 'control' || u.disposition !== 'pending') continue;
    const carrier = String(u.delivered_to_loop_id || '');
    if (!carrier) continue;
    // Carrier must be live (not yet consumed). Cancel it to strand the packet.
    const run = await api(`/api/runs/${carrier}`);
    if (run.body && run.body.state && !['completed', 'cancelled', 'failed', 'timed_out'].includes(run.body.state)) {
      const cancel = await api(`/api/runs/${carrier}/cancel`, 'POST');
      recordLeg('carrier_cancelled', { update_id: u.update_id, carrier_run_id: carrier, carrier_state: run.body.state, cancel_status: cancel.status });
      stranded = { update_id: u.update_id, carrier_run_id: carrier };
      break;
    }
  }
  if (!stranded) await new Promise((r) => setTimeout(r, 1200));
}
if (!stranded) {
  result.failure = 'no control packet bound to a live carrier within window — ask never reached research';
  finish(1);
}

// Phase 2 — watch the stranded packet recover: claim clears AND it either
// rebinds to a live run or terminalizes exhausted. Never still-bound-to-dead.
const recoverDeadline = Date.now() + RECOVER_MIN * 60 * 1000;
while (Date.now() < recoverDeadline) {
  const t = await api(`/api/trajectories/${TRAJ}`);
  const u = (t.body?.updates || []).find((x) => x.update_id === stranded.update_id);
  if (u) {
    const boundTo = String(u.delivered_to_loop_id || '');
    const disp = u.disposition;
    const exhausted = disp === 'terminal' || disp === 'failed' || disp === 'cancelled' ||
      (u.disposition_reason || '').includes('exhausted') || disp === 'delivery_failed';
    const reboundLive = boundTo && boundTo !== stranded.carrier_run_id;
    if (exhausted) { recordLeg('exhausted_scored', { update_id: u.update_id, disp, reason: u.disposition_reason }); finish(0); }
    if (reboundLive) {
      const run = await api(`/api/runs/${boundTo}`);
      if (run.body && !['completed', 'cancelled', 'failed', 'timed_out'].includes(run.body.state)) {
        recordLeg('rebound_live_run', { update_id: u.update_id, new_run_id: boundTo, disp });
      }
    }
    if (disp === 'incorporated' || disp === 'applied' || disp === 'consumed') {
      recordLeg('recovered_consumed', { update_id: u.update_id, disp, bound_to: boundTo });
      finish(0);
    }
    // A cleared claim that re-entered pending (boundTo empty) is recovery in
    // flight — keep watching for the consume/exhaust terminal.
    if (disp === 'pending' && !boundTo) {
      recordLeg('claim_released_pending', { update_id: u.update_id });
    }
  }
  await new Promise((r) => setTimeout(r, 3000));
}
const t = await api(`/api/trajectories/${TRAJ}`);
const u = (t.body?.updates || []).find((x) => x.update_id === stranded.update_id);
result.final_update = u ? { disposition: u.disposition, delivered_to_loop_id: u.delivered_to_loop_id, reason: u.disposition_reason } : null;
result.failure = `stranded packet did not recover: final=${JSON.stringify(result.final_update)}`;
finish(1);
