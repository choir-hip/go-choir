#!/usr/bin/env node
// M0 model pools: deployed acceptance (docs/model-pools-design-2026-10-12.md).
//
// Submits prompts through the real prompt bar on a fresh test computer, waits
// for each document's first desk-authored revision, then reads every run's
// model metadata and checks the pool contract:
//   - every run names a spec and an assignment (llm_spec, llm_assignment);
//   - every run's model is a member of the default pool;
//   - runs on one thread (llm_thread) keep one model;
//   - across threads both pool members are used (reported per model).
// Failover events are run events, which owners cannot read until Cut 0's
// trace export; that check is carried to C0.
//
// Usage (the key file is the 0600 one written by scripts/qa_test_computer.mjs):
//   CHOIR_API_KEY=$(jq -r .api_key <file>) node scripts/m0_pools_acceptance.mjs \
//     --computer computer-... [--prompts 3] [--timeout-min 20] [--out <json>]
//
// Exit 0 = all checks pass; 1 = a check failed or timed out. The artifact
// (non-secret ids, models, states) is written to --out.

import { writeFileSync } from 'node:fs';

const HOST = process.env.CHOIR_HOST || 'https://choir.news';
const API_KEY = process.env.CHOIR_API_KEY || '';
const args = process.argv.slice(2);
const arg = (name, dflt) => {
  const i = args.indexOf(`--${name}`);
  return i >= 0 ? args[i + 1] : dflt;
};
const COMPUTER = arg('computer', '');
const N_PROMPTS = Number(arg('prompts', '3'));
const TIMEOUT_MIN = Number(arg('timeout-min', '20'));
const OUT = arg('out', `docs/evidence/m0-model-pools-acceptance-${new Date().toISOString().slice(0, 10)}.json`);
const POOL = new Set(['synthetic/hf:deepseek-ai/DeepSeek-V4.1-Flash-1017', 'chatgpt/gpt-6-luna']);
const PROMPTS = [
  'What changed in open-weight language models this week?',
  'Summarize the state of grid-scale battery storage deployments.',
  'What is new in semiconductor export controls?',
  'What are the latest results on protein structure prediction?',
];

if (!API_KEY || !COMPUTER) {
  console.error('CHOIR_API_KEY and --computer are required');
  process.exit(2);
}
const headers = { Authorization: `Bearer ${API_KEY}`, 'X-Choir-Computer': COMPUTER };

async function api(path, method = 'GET', body = undefined) {
  const res = await fetch(`${HOST}${path}`, {
    method,
    headers: { ...headers, ...(body ? { 'Content-Type': 'application/json' } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  return { status: res.status, body: await res.json().catch(() => null) };
}

const t0 = Date.now();
const iso = (ms) => new Date(ms).toISOString();
const result = { probe: 'm0_model_pools_acceptance', host: HOST, computer: COMPUTER, started_at: iso(t0), submissions: [], runs: [], checks: {}, ok: false };

const obs = await api('/api/runtime/observability');
result.guest_build = obs.body?.build || null;

for (let i = 0; i < N_PROMPTS; i++) {
  const text = PROMPTS[i % PROMPTS.length];
  const sub = await api('/api/prompt-bar', 'POST', { text, command_id: `m0-pools-${t0}-${i}` });
  if (sub.status >= 300) {
    result.failure = `prompt-bar submit ${i} refused: ${sub.status} ${sub.body?.error || ''}`;
    finish();
  }
  result.submissions.push({ text, trajectory_id: sub.body?.trajectory_id, doc_id: sub.body?.doc_id, owner_revision: sub.body?.revision_id });
}

const deadline = t0 + TIMEOUT_MIN * 60_000;
const authored = new Map();
while (Date.now() < deadline && authored.size < result.submissions.length) {
  for (const s of result.submissions) {
    if (authored.has(s.doc_id)) continue;
    // The list is newest first; a desk revision is any non-owner author.
    const revs = (await api(`/api/texture/documents/${s.doc_id}/revisions`)).body?.revisions || [];
    const desk = revs.filter((r) => r.revision_id !== s.owner_revision && r.author_kind !== 'user');
    if (desk.length > 0) {
      authored.set(s.doc_id, desk[desk.length - 1].revision_id);
      console.log(`[doc] ${s.doc_id} first desk revision after ${Math.round((Date.now() - t0) / 1000)}s`);
    }
  }
  await new Promise((r) => setTimeout(r, 10_000));
}
for (const s of result.submissions) s.first_desk_revision = authored.get(s.doc_id) || null;

const runs = ((await api('/api/runs?limit=200')).body?.runs || []).filter((r) => r.created_at >= iso(t0 - 60_000));
for (const r of runs) {
  const m = r.metadata || {};
  result.runs.push({
    run_id: r.run_id, profile: r.agent_profile, state: r.state,
    spec: m.llm_spec || null, assignment: m.llm_assignment || null, thread: m.llm_thread || null,
    model: m.llm_provider && m.llm_model ? `${m.llm_provider}/${m.llm_model}` : null,
    fallbacks: m.llm_fallbacks || null, error: r.error ? String(r.error).slice(0, 300) : null,
  });
}

const modelRuns = result.runs.filter((r) => r.model);
const byThread = new Map();
for (const r of modelRuns) {
  if (!r.thread) continue;
  if (!byThread.has(r.thread)) byThread.set(r.thread, new Set());
  byThread.get(r.thread).add(r.model);
}
const perModel = {};
for (const r of modelRuns) perModel[r.model] = (perModel[r.model] || 0) + 1;
const threadModels = {};
for (const [thread, models] of byThread) for (const m of models) threadModels[m] = (threadModels[m] || 0) + 1;

result.checks = {
  documents_authored: { pass: authored.size === result.submissions.length, detail: `${authored.size}/${result.submissions.length}` },
  runs_seen: { pass: modelRuns.length > 0, detail: `${modelRuns.length} runs with a model (${result.runs.length} total)` },
  spec_and_assignment_recorded: { pass: modelRuns.every((r) => r.spec && r.assignment), detail: modelRuns.filter((r) => !(r.spec && r.assignment)).map((r) => r.run_id) },
  models_in_pool: { pass: modelRuns.every((r) => POOL.has(r.model)), detail: perModel },
  thread_keeps_one_model: { pass: [...byThread.values()].every((s) => s.size === 1), detail: Object.fromEntries([...byThread].map(([k, v]) => [k, [...v]])) },
  both_members_used: { pass: Object.keys(threadModels).length === POOL.size, detail: threadModels },
  no_catalog_refusals: { pass: !result.runs.some((r) => /not in the catalog/.test(r.error || '')), detail: result.runs.filter((r) => /not in the catalog/.test(r.error || '')).map((r) => r.run_id) },
};
result.ok = Object.values(result.checks).every((c) => c.pass);
if (!result.ok && !result.failure) result.failure = Object.entries(result.checks).filter(([, c]) => !c.pass).map(([k]) => k).join(',');
finish();

function finish() {
  result.finished_at = iso(Date.now());
  result.total_wall_ms = Date.now() - t0;
  writeFileSync(OUT, `${JSON.stringify(result, null, 2)}\n`);
  for (const [k, c] of Object.entries(result.checks)) console.log(`${c.pass ? 'PASS' : 'FAIL'} ${k} ${JSON.stringify(c.detail)}`);
  console.log(`\n${result.ok ? 'PASS' : 'FAIL'}${result.failure ? ` — ${result.failure}` : ''}\nartifact -> ${OUT}`);
  process.exit(result.ok ? 0 : 1);
}
