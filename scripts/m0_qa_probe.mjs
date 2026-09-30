#!/usr/bin/env node
// M0 QA repro + per-desk-turn timing baseline.
//
// Drives the real prompt bar on staging (POST /api/prompt-bar via the choir
// CLI), then watches the resulting trajectory: conductor routes, texture
// authors v1, a research leg runs, and texture commits a v2 revision.
// Records wall_ms + tokens per desk leg into the baseline artifact.
//
// Usage:
//   CHOIR_API_KEY=... node scripts/m0_qa_probe.mjs \
//     [--computer computer-...] [--prompt "What's new in ai today"] \
//     [--timeout-min 15] [--out docs/evidence/m0-qa-baseline-timings-<ts>.json]
//
// Exit 0 = v2 committed (stall closed). Exit 1 = timed out / failed leg —
// stdout carries the observed leg table for the residual-evidence doc.

import { execFileSync } from 'node:child_process';
import { writeFileSync } from 'node:fs';

const HOST = process.env.CHOIR_HOST || 'https://choir.news';
const API_KEY = process.env.CHOIR_API_KEY || '';
const args = process.argv.slice(2);
const arg = (name, dflt) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : dflt;
};
const COMPUTER = arg('computer', 'computer-03335285269bdba4f94377e56879f9e6');
const PROMPT = arg('prompt', "What's new in ai today");
const TIMEOUT_MIN = Number(arg('timeout-min', '15'));
const OUT = arg('out', `docs/evidence/m0-qa-baseline-timings-${new Date().toISOString().slice(0, 10)}.json`);

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
  probe: 'm0_qa_repro',
  computer: COMPUTER,
  prompt: PROMPT,
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

async function listRuns() {
  const res = await api('/api/runs?limit=25');
  return Array.isArray(res.body?.runs) ? res.body.runs : [];
}

async function revisions(docID) {
  const res = await api(`/api/texture/documents/${docID}/revisions`);
  return Array.isArray(res.body?.revisions) ? res.body.revisions : [];
}

// 1. Submit the prompt via the real prompt bar.
const idem = `m0-qa-${Date.now()}`;
let submitOut;
try {
  submitOut = JSON.parse(execFileSync(
    '/tmp/choir',
    ['run', 'start', '--computer', COMPUTER, '--idempotency-key', idem, PROMPT],
    { encoding: 'utf8', timeout: 90000 },
  ));
} catch (err) {
  recordLeg('prompt_bar_submit', { ok: false, error: String(err).slice(0, 400) });
  result.failure = 'prompt_bar submit failed';
  finish(1);
}
const { trajectory_id: TRAJ, doc_id: DOC, revision_id: REV0 } = submitOut;
recordLeg('prompt_bar_submit', { ok: true, trajectory: TRAJ, doc: DOC, owner_revision: REV0 });

const deadline = t0 + TIMEOUT_MIN * 60 * 1000;
const seenRuns = new Map(); // run_id -> state
let researchRun = null;
let textureV1 = null;
let textureV2 = null;

while (Date.now() < deadline) {
  const runs = await listRuns();
  for (const r of runs) {
    if (r.created_at < iso(t0)) continue; // only this probe's trajectory's legs
    if (seenRuns.get(r.run_id) !== r.state) {
      seenRuns.set(r.run_id, r.state);
      if (r.agent_profile === 'research' && r.created_at >= iso(t0)) researchRun = r;
      if (['completed', 'failed', 'cancelled'].includes(r.state)) {
        const wall = new Date(r.finished_at) - new Date(r.created_at);
        const m = r.metadata || {};
        // record a terminal leg once
        if (!result.legs.some((l) => l.run_id === r.run_id)) {
          result.legs.push({
            name: `run:${r.agent_profile}:${r.state}`,
            run_id: r.run_id,
            wall_ms: wall,
            input_tokens: m.input_tokens ?? null,
            output_tokens: m.output_tokens ?? null,
            model: m.llm_model || m.model || null,
          });
        }
      }
    }
  }

  const revs = await revisions(DOC);
  const ownerIdx = revs.findIndex((r) => r.revision_id === REV0);
  const post = ownerIdx >= 0 ? revs.slice(ownerIdx + 1) : revs;
  if (!textureV1 && post.length >= 1) {
    textureV1 = post[0];
    recordLeg('texture_v1', { revision: textureV1.revision_id, author: textureV1.author_kind });
  }
  if (!textureV2 && post.length >= 2) {
    textureV2 = post[post.length - 1];
    recordLeg('texture_v2_committed', { revision: textureV2.revision_id, author: textureV2.author_kind });
    result.ok = true;
    break;
  }
  if (researchRun && ['failed', 'cancelled'].includes(researchRun.state)) {
    result.failure = `research run ${researchRun.run_id} ${researchRun.state}`;
    break;
  }
  await new Promise((r) => setTimeout(r, 5000));
}

if (!result.ok && !result.failure) result.failure = `timed out after ${TIMEOUT_MIN}min (legs: ${result.legs.map((l) => l.name).join(',')})`;
if (researchRun) result.research_run = { id: researchRun.run_id, state: researchRun.state };
result.finished_at = iso(Date.now());
result.total_wall_ms = Date.now() - t0;

function finish(code) {
  writeFileSync(OUT, JSON.stringify(result, null, 2));
  console.log(`\n${result.ok ? 'PASS' : 'FAIL'} — ${result.failure || 'v2 committed'}`);
  console.log(`baseline -> ${OUT}`);
  process.exit(code);
}
finish(result.ok ? 0 : 1);
