#!/usr/bin/env node
// S0m finish acceptance probe: the record-native ask→report→resolve chain.
//
// Prompts the owner staging computer with a research question framed as the
// document task itself — texture must stake a question to research
// (choir.Ask / Precommit), research answers via choir.Reply, and the ask
// record resolves mechanically. Watches the trajectory events for:
//   1. a lifecycle event carrying update_id with ":ask:" or ":precommit:" —
//      the staked question minted record+packet atomically;
//   2. a ":reply:" update — the answering desk's record-native report;
//   3. a resolve record minted against the ask.
// Exit 0 = ask packet observed (queued or delivered). Reply/resolve legs
// strengthen the chain but the question mint alone proves the cutover.

import { mkdirSync, writeFileSync } from 'node:fs';

const HOST = process.env.CHOIR_HOST || 'https://choir.news';
const API_KEY = process.env.CHOIR_API_KEY || '';
const args = process.argv.slice(2);
const arg = (name, dflt) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : dflt;
};
const COMPUTER = arg('computer', 'computer-03335285269bdba4f94377e56879f9e6');
const MARKER = arg('marker', `s0m-ask-${Date.now()}`);
const TIMEOUT_MIN = Number(arg('timeout-min', '25'));
const OUT = arg('out', `docs/evidence/s0m-ask-acceptance-${new Date().toISOString().slice(0, 10)}.json`);

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
  probe: 's0m_ask_acceptance',
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
  `Operator-authorized record-native acceptance probe (marker ${MARKER}). ` +
  `Task: write one line into this document reporting the current UTC date. ` +
  `To get it right you MUST gather the date from the research desk. Open a ` +
  `researcher atomically: stage one choir.ApplyTexture apply turn whose ` +
  `controls[] contains {"open_researcher": true, "objective": "Report the ` +
  `current UTC date", "packet": {"kind": "question", "summary": "What is ` +
  `today's UTC date?", "questions": ["What is today's UTC date? Cite a ` +
  `time.is or worldtimeapi.org page."]}}. When the research report arrives, ` +
  `incorporate the answer into the document, and end. The document is ` +
  `disposable.`;

const submit = await api('/api/prompt-bar', 'POST', { text: prompt, command_id: `s0m-ask-${MARKER}` });
let TRAJ = submit.body?.trajectory_id;
let DOC = submit.body?.doc_id;
if (submit.status !== 200 && submit.status !== 202) {
  recordLeg('prompt_bar_submit', { ok: false, status: submit.status, body: submit.body });
  finish(1);
}
recordLeg('prompt_bar_submit', { ok: true, trajectory: TRAJ, doc: DOC });

const deadline = t0 + TIMEOUT_MIN * 60 * 1000;
let asked = null, replied = null, resolved = null;
const seen = new Set();

while (Date.now() < deadline) {
  if (TRAJ) {
    const ev = await api(`/api/trajectories/${TRAJ}/events?limit=400`);
    for (const e of ev.body?.events || []) {
      const key = `${e.kind}:${e.update_id || ''}:${e.event_id}`;
      if (seen.has(key)) continue;
      seen.add(key);
      const uid = String(e.update_id || '');
      if ((e.kind === 'update_queued' || e.kind === 'control_queued' || e.kind === 'control_delivered' || e.kind === 'update_delivered')) {
        // The "ask" is the open_researcher control packet the desk mints —
        // a bound control/question directed at research. Also catch an
        // explicit ask/precommit record if the desk stakes one.
        if (uid.includes(':ask:') || uid.includes(':precommit:') ||
            e.kind === 'control_queued' || e.kind === 'control_delivered' ||
            e.direction === 'control' ||
            String(e.packet_kind || '') === 'question') {
          if (!asked || !asked.delivered) {
            asked = { ...asked, e, delivered: e.kind.includes('delivered') };
            recordLeg('ask_' + e.kind, { update_id: uid });
          }
        }
        if (uid.includes(':reply:') || uid.includes(':report:') ||
            e.direction === 'producer_report' ||
            String(e.packet_kind || '') === 'execution_result' ||
            String(e.packet_kind || '') === 'evidence_update') {
          replied = e;
          recordLeg('reply_' + e.kind, { update_id: uid });
        }
      }
      if (e.kind === 'commitment_record_minted' || e.kind === 'record_minted') {
        if (String(e.record_kind || '') === 'resolve' || uid.includes('resolve')) {
          resolved = e;
          recordLeg('resolve_minted', { update_id: uid, kind: e.record_kind });
        }
      }
    }
  }
  if (asked && replied) {
    result.ask_update_id = asked.e?.update_id;
    result.reply_update_id = replied.update_id;
    result.resolved = !!resolved;
    finish(0);
  }
  await new Promise((r) => setTimeout(r, 15000));
}

result.failure = `timeout: asked=${!!asked} replied=${!!replied} resolved=${!!resolved}`;
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
