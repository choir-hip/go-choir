// node --test scripts/reeng_invariants_test.mjs
//
// Proves the Cut 0 invariant checker on planted violations before any pass
// is trusted (acceptance row "checker on planted-violation copies",
// docs/definitions/choir-reeng-cut0-eyes-2026-10-12.md). A realistic
// synthetic bundle (the shape of docs/reports/reeng-trace-bundle-format-2026-10-12.md:
// a prompt-bar story, Texture/management/research desks, a crash drill
// mid-run, an owner follow-up after the boot, publication) passes I1-I5.
// Each plantIn copy must fail In and only In; each copy with a rule's layer
// unserved must yield unknown, never pass.
//
// Ways the checker could fail, written before the code:
//  - a rule passes because it read nothing (empty or unserved layer);
//  - a derived layer (cells, egress) is read although its parent is unserved;
//  - I1 treats an owner edit as a violation, or a non-desk appagent write as Texture;
//  - I2 accepts a pending/running run with no reason;
//  - I3 flags the in-flight run's boot passivation (a closure) as a resume,
//    or the owner's legitimate follow-up as a violation, or misses a
//    resume because the resumed run was created before the boot;
//  - I4 passes a quote it never compared, or fails a quote on a truncated body;
//  - I5 passes without load, or misses a crossed request id;
//  - a plant mutates the caller's bundle or breaks another rule.

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, readFileSync, readdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { execFileSync, spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { checkBundle, checkI1, checkI2, checkI3, checkI4, checkI5, checkI6, checkI7, exitCode } from './reeng_invariants.mjs';
import { plantI1, plantI2, plantI3, plantI4, plantI5, PLANTS } from './reeng_invariants_plant.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const T0 = Date.parse('2026-10-12T20:00:00.000Z');
const at = (s) => new Date(T0 + Math.round(s * 1000)).toISOString().replace('Z', '123Z').replace(/\.(\d{3})123Z$/, '.$1000123Z');

const OWNER = 'user-owner-1';
const COMPUTER = 'computer-test-1';
const TRAJ = 'traj-1';
const DOC = 'doc-1';

const SOURCE_BODY_A = 'Lab X said on Tuesday that the new model was released to all developers, with pricing unchanged.';
const SOURCE_BODY_B = 'Regulators in the EU asked for “more transparency” about training data before year end.';

function sources(withB) {
  const list = [
    {
      source_entity_id: 'src-a',
      target: { kind: 'web_url', uri: 'https://news.example/lab-x-release' },
      selectors: [{ kind: 'text_quote', data: { exact: 'the new model was released to all developers' } }],
      display: { mode: 'numbered_ref', title: 'Lab X release' },
      evidence: { state: 'available', open_surface: 'source' },
      provenance: { created_by: 'research' },
      reader_snapshot: { snapshot_kind: 'cleaned_reader_markdown', text_content: SOURCE_BODY_A, truncated: false },
    },
  ];
  if (withB) {
    list.push({
      source_entity_id: 'src-b',
      target: { kind: 'web_url', uri: 'https://news.example/eu-transparency' },
      selectors: [{ kind: 'text_quote', data: { exact: 'asked for "more   transparency" about training data' } }],
      display: { mode: 'numbered_ref', title: 'EU ask' },
      evidence: { state: 'available', open_surface: 'source' },
      provenance: { created_by: 'research' },
      reader_snapshot: { text_content: SOURCE_BODY_B },
    });
  }
  return list;
}
function bodyDoc(withB) {
  const para = (id, text, ref) => ({ type: 'paragraph', attrs: { id }, content: [{ type: 'text', text }, { type: 'source_ref', attrs: { id: `${id}-ref`, source_entity_id: ref, display_mode: 'numbered_ref' } }] });
  const content = [para('p1', 'Lab X shipped its model to every developer. ', 'src-a')];
  if (withB) content.push(para('p2', 'EU regulators want more transparency. ', 'src-b'));
  return { schema: 'choir.texture_doc.v1', doc: { type: 'doc', attrs: { id: 'doc-root' }, content } };
}

function run(id, agent, profile, createdS, extra = {}) {
  return {
    loop_id: id,
    agent_id: agent,
    agent_profile: profile,
    agent_role: profile,
    owner_id: OWNER,
    computer_id: COMPUTER,
    trajectory_id: TRAJ,
    channel_id: DOC,
    prompt: `${profile} prompt`,
    created_at: at(createdS),
    updated_at: at(createdS + 1),
    ...extra,
  };
}

let seq = 0;
function rev(event, s, loop, kind, payload) {
  seq++;
  return { event_id: `ev-${loop}-${seq}`, seq, stream_seq: seq, ts: at(s), loop_id: loop, agent_id: '', owner_id: OWNER, trajectory_id: TRAJ, kind: event || kind, payload };
}
let rseq = 0;
function lev(kind, s, extra = {}) {
  rseq++;
  return { schema: 'choir.durable_work.v1', event_id: `lev-${rseq}`, owner_id: OWNER, computer_id: COMPUTER, trajectory_id: TRAJ, kind, reducer_version: 'durable-work/v1', reducer_seq: rseq, command_id: `cmd-${rseq}`, command_digest: `d-${rseq}`, created_at: at(s), ...extra };
}

export function buildCleanBundle() {
  seq = 0;
  rseq = 0;
  const runs = [
    run('run-conductor-1', 'conductor', 'conductor', 0, { state: 'completed', finished_at: at(1), metadata: { submission_surface: 'prompt_bar', input_source: 'prompt_bar', seed_prompt: 'AI news today' } }),
    // The Texture desk reuses one run record across activations
    // (texture_controller.go:1024-1053): passivated now, reactivated after the
    // boot by the owner's follow-up revision (current_revision_id).
    run('run-texture-1', `texture:${DOC}`, 'texture', 2, { state: 'passivated', metadata: { passivated_reason: 'idle_actor_passivate', lifecycle_work_item_id: 'work-texture-1', current_revision_id: 'rev-owner-2', actor_reactivated_from_passivated: true } }),
    run('run-mgmt-1', 'management', 'management', 40, { requested_by_run_id: 'run-texture-1', state: 'completed', finished_at: at(90), metadata: { lifecycle_work_item_id: 'work-mgmt-1' } }),
    run('run-research-1', 'research', 'research', 45, { requested_by_run_id: 'run-mgmt-1', state: 'completed', finished_at: at(80) }),
    run('run-research-2', 'research', 'research', 50, { requested_by_run_id: 'run-mgmt-1', state: 'completed', finished_at: at(85) }),
    // In flight when the restart was sent; the new process passivates it at boot.
    run('run-mgmt-2', 'management', 'management', 100, { requested_by_run_id: 'run-texture-1', state: 'passivated', metadata: { passivated_reason: 'runtime_restarted', lifecycle_work_item_id: 'work-mgmt-1' } }),
    run('run-mgmt-3', 'management', 'management', 231, { requested_by_run_id: 'run-texture-1', state: 'completed', finished_at: at(300), metadata: { lifecycle_work_item_id: 'work-mgmt-1' } }),
  ];

  const cell = (loop, sess, pid, s, f, extra = {}) => rev('desk.cell', f, loop, null, { desk_session: sess, worker_pid: pid, run_id: loop, started_at: at(s), finished_at: at(f), duration_ms: Math.round((f - s) * 1000), waited_for_run: '', waited_ms: 0, error: '', reuse: '', diag: '', intents: 1, ...extra });
  const egressEv = (loop, sess, pid, req, s, f, verb = 'web_search') => rev('desk.egress', f, loop, null, { request_id: req, response_request_id: req, verb, desk_session: sess, worker_pid: pid, run_id: loop, started_at: at(s), finished_at: at(f), is_error: false, error: '' });

  const runEvents = [
    rev('loop.submitted', 0, 'run-conductor-1', null, {}),
    rev('loop.completed', 1, 'run-conductor-1', null, {}),
    rev('loop.started', 2.1, 'run-texture-1', null, {}),
    rev('tool.invoked', 90, 'run-texture-1', null, { tool: 'desk_go_eval', call_id: 'c1' }),
    cell('run-texture-1', `agent:texture:${DOC}`, 5151, 90, 95),
    rev('activation.passivated', 96, 'run-texture-1', null, { reason: 'idle' }),
    rev('loop.started', 40.1, 'run-mgmt-1', null, {}),
    cell('run-mgmt-1', 'agent:management', 6161, 41, 88),
    rev('loop.completed', 90, 'run-mgmt-1', null, {}),
    rev('loop.started', 45.1, 'run-research-1', null, {}),
    egressEv('run-research-1', 'agent:research', 4242, 'r3-a', 47.0, 47.3),
    egressEv('run-research-1', 'agent:research', 4242, 'r3-b', 47.01, 47.31),
    egressEv('run-research-1', 'agent:research', 4242, 'r3-c', 47.02, 47.32, 'fetch_url'),
    cell('run-research-1', 'agent:research', 4242, 46, 60),
    rev('loop.completed', 80, 'run-research-1', null, {}),
    rev('loop.started', 50.1, 'run-research-2', null, {}),
    egressEv('run-research-2', 'agent:research', 4242, 'r4-a', 61, 61.5),
    cell('run-research-2', 'agent:research', 4242, 60.001, 70, { waited_for_run: 'run-research-1', waited_ms: 9900 }),
    rev('loop.completed', 85, 'run-research-2', null, {}),
    rev('loop.started', 100.1, 'run-mgmt-2', null, {}),
    cell('run-mgmt-2', 'agent:management', 6161, 101, 110),
    rev('tool.invoked', 115, 'run-mgmt-2', null, { tool: 'desk_go_eval', call_id: 'c9' }),
    // boot passivation by the new process (runtime.go:2381-2424)
    rev('activation.passivated', 140, 'run-mgmt-2', null, { recovery: 'passivated_on_restart' }),
    // owner follow-up reactivates the Texture desk
    rev('loop.started', 201, 'run-texture-1', null, {}),
    cell('run-texture-1', `agent:texture:${DOC}`, 8181, 202, 228),
    cell('run-texture-1', `agent:texture:${DOC}`, 8181, 310, 320),
    rev('activation.passivated', 330, 'run-texture-1', null, { reason: 'idle' }),
    rev('loop.started', 231.1, 'run-mgmt-3', null, {}),
    cell('run-mgmt-3', 'agent:management', 7171, 232, 298),
    rev('loop.completed', 300, 'run-mgmt-3', null, {}),
  ].sort((a, b) => (a.loop_id < b.loop_id ? -1 : a.loop_id > b.loop_id ? 1 : a.seq - b.seq));

  const packets = [
    { update_id: 'pkt-ctl-1', owner_id: OWNER, computer_id: COMPUTER, agent_id: `texture:${DOC}`, target_agent_id: 'management', channel_id: DOC, trajectory_id: TRAJ, direction: 'control', source_run_id: 'run-texture-1', created_at: at(39), delivered_to_loop_id: 'run-mgmt-1', delivered_at: at(40), packet: {}, content: 'cast' },
    { update_id: 'pkt-rep-1', owner_id: OWNER, computer_id: COMPUTER, agent_id: 'management', target_agent_id: `texture:${DOC}`, channel_id: DOC, trajectory_id: TRAJ, direction: 'report', source_run_id: 'run-mgmt-1', created_at: at(89), delivered_to_loop_id: 'run-texture-1', delivered_at: at(95), packet: {}, content: 'report' },
    { update_id: 'pkt-ctl-2', owner_id: OWNER, computer_id: COMPUTER, agent_id: `texture:${DOC}`, target_agent_id: 'management', channel_id: DOC, trajectory_id: TRAJ, direction: 'control', source_run_id: 'run-texture-1', created_at: at(230), delivered_to_loop_id: 'run-mgmt-3', delivered_at: at(232), packet: {}, content: 'cast follow-up' },
    { update_id: 'pkt-rep-2', owner_id: OWNER, computer_id: COMPUTER, agent_id: 'management', target_agent_id: `texture:${DOC}`, channel_id: DOC, trajectory_id: TRAJ, direction: 'report', source_run_id: 'run-mgmt-3', created_at: at(299), delivered_to_loop_id: 'run-texture-1', delivered_at: at(305), packet: {}, content: 'report 2' },
  ];
  const workItems = [
    { work_item_id: 'work-texture-1', trajectory_id: TRAJ, owner_id: OWNER, computer_id: COMPUTER, objective: 'AI news today', status: 'completed', assigned_agent_id: `texture:${DOC}`, created_at: at(1), updated_at: at(390) },
    { work_item_id: 'work-mgmt-1', trajectory_id: TRAJ, owner_id: OWNER, computer_id: COMPUTER, objective: 'research the story', status: 'completed', assigned_agent_id: 'management', created_by_loop_id: 'run-texture-1', created_at: at(39), updated_at: at(390) },
  ];
  const lifecycleEvents = [
    lev('trajectory_started', 1),
    lev('work_opened', 1, { work_item_id: 'work-texture-1' }),
    lev('control_queued', 39, { update_id: 'pkt-ctl-1' }),
    lev('work_opened', 39, { work_item_id: 'work-mgmt-1' }),
    lev('control_delivered', 40, { update_id: 'pkt-ctl-1', run_id: 'run-mgmt-1', agent_id: 'management', work_item_id: 'work-mgmt-1' }),
    lev('update_queued', 89, { update_id: 'pkt-rep-1' }),
    lev('run_terminalized', 90, { run_id: 'run-mgmt-1' }),
    lev('update_delivered', 95, { update_id: 'pkt-rep-1', run_id: 'run-texture-1' }),
    lev('texture_turn_committed', 95, { run_id: 'run-texture-1' }),
    lev('artifact_head_advanced', 95, { artifact_refs: ['rev-desk-1'] }),
    lev('artifact_head_advanced', 200, { command_id: 'owner-revise:client-req-2', artifact_refs: ['rev-owner-2'] }),
    lev('run_reactivated', 201, { artifact_refs: ['run-texture-1'] }),
    lev('control_queued', 230, { update_id: 'pkt-ctl-2' }),
    lev('control_delivered', 232, { update_id: 'pkt-ctl-2', run_id: 'run-mgmt-3', agent_id: 'management', work_item_id: 'work-mgmt-1' }),
    lev('update_queued', 299, { update_id: 'pkt-rep-2' }),
    lev('run_terminalized', 300, { run_id: 'run-mgmt-3' }),
    lev('update_delivered', 305, { update_id: 'pkt-rep-2', run_id: 'run-texture-1' }),
    lev('texture_turn_committed', 320, { run_id: 'run-texture-1' }),
    lev('artifact_head_advanced', 320, { artifact_refs: ['rev-desk-3'] }),
    lev('work_settled', 390, { work_item_id: 'work-mgmt-1' }),
    lev('work_settled', 390, { work_item_id: 'work-texture-1' }),
    lev('trajectory_settled', 400),
  ];
  const revision = (id, v, s, kind, label, m, withB, extra = {}) => ({
    revision_id: id,
    doc_id: DOC,
    owner_id: OWNER,
    author_kind: kind,
    author_label: label,
    version_number: v,
    content: 'projection',
    body_doc: bodyDoc(withB),
    source_entities: sources(withB),
    citations: [],
    metadata: m,
    parent_revision_id: '',
    created_at: at(s),
    ...extra,
  });
  const revisions = [
    { revision_id: 'rev-owner-0', doc_id: DOC, owner_id: OWNER, author_kind: 'user', author_label: OWNER, version_number: 0, content: 'AI news today', created_at: at(1) },
    revision('rev-desk-1', 1, 95, 'appagent', 'appagent', { source: 'texture_cell', loop_id: 'run-texture-1', texture_edit_tool: 'texture_cell' }, false, { parent_revision_id: 'rev-owner-0', provenance: { schema_version: 1 } }),
    revision('rev-owner-2', 2, 200, 'user', OWNER, { input_origin: 'user_prompt', owner_prompt: 'also cover the EU angle' }, false, { parent_revision_id: 'rev-desk-1' }),
    revision('rev-desk-3', 3, 320, 'appagent', 'appagent', { source: 'texture_cell', loop_id: 'run-texture-1' }, true, { parent_revision_id: 'rev-owner-2', provenance: { schema_version: 1 } }),
  ];
  const snapshot = {
    schema: 'choir.durable_work.v1',
    trajectory: { trajectory_id: TRAJ, owner_id: OWNER, computer_id: COMPUTER, kind: 'document', status: 'settled', settlement_rule: { require_no_open_work_items: true }, created_at: at(0), updated_at: at(400), settled_at: at(400) },
    work_items: workItems,
    agents: [{ agent_id: `texture:${DOC}`, profile: 'texture' }, { agent_id: 'management', profile: 'management' }],
    co_super_assignments: [],
    updates: packets,
    document: { doc_id: DOC, owner_id: OWNER, title: 'AI news today', current_revision_id: 'rev-desk-3' },
    head_revision: { revision_id: 'rev-desk-3' },
    events: [],
  };
  const deskEvents = (k) => runEvents.filter((e) => e.kind === k);
  const L = (status, items, sources = [], note = '') => ({ status, sources, note, items });
  const restart = { kind: 'crash_or_stop' };
  return {
    schema: 'choir.trace_bundle.v1',
    exported_at: at(500),
    base_url: 'https://choir.news',
    computer_id: COMPUTER,
    trajectory_id: TRAJ,
    build: { commit: 'abc123', deployed_commit: 'abc123', built_at: at(-3600) },
    drill: {
      label: 'c0-test',
      restart_requested_at: at(120),
      in_flight_run_id: 'run-mgmt-2',
      ready_at: at(150),
      obligations_before: { restart: { kind: 'crash_or_stop' } },
      obligations_after: { restart },
      load: { parallel_cell_requested: true, second_run_submitted_at: at(50) },
      attempts: 1,
    },
    layers: {
      trajectory: L('present', [snapshot], [`GET /api/trajectories/${TRAJ}`]),
      lifecycle_events: L('present', lifecycleEvents, [`GET /api/trajectories/${TRAJ}/events`]),
      runs: L('present', runs, [`GET /api/runs?trajectory_id=${TRAJ}`]),
      run_events: L('present', runEvents, ['GET /api/runs/{id}/events']),
      cells: L('present', deskEvents('desk.cell'), ['derived:run_events']),
      egress: L('present', deskEvents('desk.egress').map((e) => ({ ...e.payload, event_id: e.event_id })), ['derived:run_events']),
      deliveries: L('present', [...packets, ...workItems], ['derived:trajectory']),
      records: L('empty', [], ['GET /api/inboxes/{inbox}/records']),
      inboxes: L('empty', [], ['GET /api/inboxes/{inbox}/records']),
      census: L('empty', [{ kinds: ['choir.work_item', 'choir.worker_update'], counts: {}, objects: [] }], ['GET /api/runtime/census']),
      turn_commits: L('present', lifecycleEvents.filter((e) => e.kind === 'texture_turn_committed'), ['derived:lifecycle_events']),
      revisions: L('present', revisions, [`GET /api/texture/documents/${DOC}/revisions`]),
      publications: L('unserved', [], [], 'no publication route resolvable'),
      search_arrivals: L('unserved', [], [], 'until Cut 2'),
      research_children: L('unserved', [], [], 'children run without a run record'),
      obligations: L('present', [{ computer_id: COMPUTER, restart, runs: {}, open_work_items: { count: 0 } }], ['GET /api/runtime/obligations']),
      boot: L('present', [{ computer_id: COMPUTER, build: { commit: 'abc123' }, recent_log: ['runtime: passivated run run-mgmt-2 (was running) after restart'], restart }], ['GET /api/runtime/observability']),
    },
  };
}

const RULES = ['I1', 'I2', 'I3', 'I4', 'I5'];
const verdicts = (b) => Object.fromEntries(checkBundle(b).map((r) => [r.rule, r.verdict]));
const unserve = (b, ...names) => {
  const c = structuredClone(b);
  for (const n of names) c.layers[n] = { status: 'unserved', sources: [], note: 'test: unserved', items: [] };
  return c;
};

test('clean bundle passes I1-I5 and I7; I6 reads nothing from an empty records layer', () => {
  const b = buildCleanBundle();
  const rules = checkBundle(b);
  const v = Object.fromEntries(rules.map((r) => [r.rule, r]));
  for (const r of RULES) assert.equal(v[r].verdict, 'pass', `${r}: ${v[r].reason}`);
  assert.equal(v.I6.verdict, 'unknown');
  assert.equal(v.I7.verdict, 'pass', v.I7.reason);
  assert.equal(exitCode(rules), 0);
  // the passes read real items
  assert.match(v.I3.reason, /post-boot dispatches all trace to owner input/);
  assert.ok(v.I5.load.appeared);
  assert.match(v.I4.reason, /all 4 citations/);
});

for (const [rule, plant] of Object.entries(PLANTS)) {
  test(`planted ${rule} fails ${rule} and only ${rule}`, () => {
    const b = buildCleanBundle();
    const before = JSON.stringify(b);
    const planted = plant(b);
    assert.equal(JSON.stringify(b), before, 'plant must not mutate its input');
    const v = verdicts(planted);
    assert.equal(v[rule], 'fail', `${rule} should fail on its plant`);
    for (const other of RULES) if (other !== rule) assert.equal(v[other], 'pass', `${other} should still pass on planted ${rule}`);
    assert.equal(exitCode(checkBundle(planted)), 1);
  });
}

test('plants work on a bare bundle (every layer unserved, no drill)', () => {
  const bare = { schema: 'choir.trace_bundle.v1', exported_at: '2026-10-12T21:00:00Z', computer_id: 'c', trajectory_id: 't', drill: null, layers: {} };
  for (const r of RULES) assert.equal(verdicts(bare)[r], 'unknown', `bare ${r}`);
  for (const [rule, plant] of Object.entries(PLANTS)) assert.equal(verdicts(plant(bare))[rule], 'fail', `bare planted ${rule}`);
});

test('plants stay fail-only-their-rule on copies with other layers unserved', () => {
  const b = unserve(buildCleanBundle(), 'records', 'publications');
  for (const [rule, plant] of Object.entries(PLANTS)) assert.equal(verdicts(plant(b))[rule], 'fail');
});

// --- unserved layers never pass
const unservedCases = [
  ['I1', ['revisions']],
  ['I2', ['runs']],
  ['I2', ['trajectory']],
  ['I3', ['runs']],
  ['I3', ['lifecycle_events']],
  ['I3', ['run_events']],
  ['I3', ['trajectory']],
  ['I3', ['revisions']],
  ['I4', ['revisions']],
  ['I5', ['run_events']],
  ['I5', ['egress']],
  ['I5', ['cells']],
];
for (const [rule, layers] of unservedCases) {
  test(`${rule} with ${layers.join(',')} unserved is unknown, never pass`, () => {
    const v = verdicts(unserve(buildCleanBundle(), ...layers));
    assert.equal(v[rule], 'unknown');
  });
}

test('derived layers inherit unserved from their parent even if marked present', () => {
  const b = unserve(buildCleanBundle(), 'run_events');
  assert.equal(b.layers.cells.status, 'present');
  const r = checkI5(b);
  assert.equal(r.verdict, 'unknown');
  assert.equal(r.halves.request_id.verdict, 'unknown');
  assert.equal(r.halves.one_worker.verdict, 'unknown');
});

test('empty layers are unknown, not pass', () => {
  const b = buildCleanBundle();
  b.layers.revisions = { status: 'empty', sources: [], note: '', items: [] };
  assert.equal(checkI1(b).verdict, 'unknown');
  assert.equal(checkI4(b).verdict, 'unknown');
});

test('I3 without a drill, or after a planned boot, or with unknown boot kind is unknown', () => {
  const b = buildCleanBundle();
  assert.equal(checkI3({ ...b, drill: null }).verdict, 'unknown');
  const planned = structuredClone(b);
  planned.layers.boot.items[0].restart = { kind: 'planned', reason: 'platform update' };
  assert.equal(checkI3(planned).verdict, 'unknown');
  const unk = unserve(b, 'boot');
  delete unk.drill.obligations_after;
  assert.equal(checkI3(unk).verdict, 'unknown');
});

// --- rule semantics beyond the generic plants

test('I1: owner edits pass; a user revision labelled by someone else fails; the frozen processor seed fails', () => {
  const b = buildCleanBundle();
  const imp = structuredClone(b);
  imp.layers.revisions.items[2].author_label = 'someone-else';
  assert.equal(checkI1(imp).verdict, 'fail');
  const seed = structuredClone(b);
  Object.assign(seed.layers.revisions.items[1], { author_label: 'processor-agent', metadata: { source: 'coagent_texture_seed' } });
  assert.equal(checkI1(seed).verdict, 'fail');
  const wrongRun = structuredClone(b);
  wrongRun.layers.revisions.items[1].metadata.loop_id = 'run-mgmt-1';
  assert.equal(checkI1(wrongRun).verdict, 'fail');
});

test('I2: pending fails, blocked with error passes, passivated without reason fails, live trajectory with nothing owed fails', () => {
  const b = buildCleanBundle();
  const pending = structuredClone(b);
  pending.layers.runs.items[2].state = 'pending';
  assert.equal(checkI2(pending).verdict, 'fail');
  const blocked = structuredClone(b);
  Object.assign(blocked.layers.runs.items[2], { state: 'blocked', error: 'provider unavailable' });
  assert.equal(checkI2(blocked).verdict, 'pass');
  const pas = structuredClone(b);
  delete pas.layers.runs.items[1].metadata.passivated_reason;
  assert.equal(checkI2(pas).verdict, 'fail');
  const live = structuredClone(b);
  live.layers.trajectory.items[0].trajectory.status = 'live';
  delete live.layers.records;
  delete live.layers.inboxes;
  assert.equal(checkI2(live).verdict, 'unknown', 'nothing owed by work items and no records layer: missing evidence, not a pass or a fail');
  live.layers.records = { status: 'empty', sources: ['GET /api/inboxes/{inbox}/records'], note: '', items: [] };
  live.layers.inboxes = { status: 'empty', sources: ['same'], note: '', items: [] };
  assert.equal(checkI2(live).verdict, 'fail');
  live.layers.trajectory.items[0].work_items[0].status = 'open';
  live.layers.trajectory.items[0].work_items[0].reason = 'awaiting owner review';
  assert.equal(checkI2(live).verdict, 'pass');
});

test('I2 on records: an open round or a system closure with a fate names why a live trajectory is open', () => {
  const b = buildCleanBundle();
  b.layers.trajectory.items[0].trajectory.status = 'live';
  b.layers.trajectory.items[0].work_items = [];
  const conv = `conv:research:${DOC}|texture:${DOC}`;
  const cast = { record_id: 'c1', inbox: `research:${DOC}`, sender_inbox: `texture:${DOC}`, inbox_seq: 1, conversation_id: conv, round: 1, directive: { subtype: 'cast' }, provenance: { committed_at: at(10) } };
  b.layers.records = { status: 'present', sources: ['GET /api/inboxes/{inbox}/records'], note: '', items: [cast] };
  b.layers.inboxes = { status: 'present', sources: ['same'], note: '', items: [
    { inbox: `research:${DOC}`, head: 1, cursor: 1, open_rounds: [{ conversation_id: conv, round: 1, open: true, caster_inbox: `texture:${DOC}`, recipient_inbox: `research:${DOC}` }] },
  ] };
  assert.equal(checkI2(b).verdict, 'pass', checkI2(b).reason);
  const closed = structuredClone(b);
  closed.layers.inboxes.items[0].open_rounds = [];
  closed.layers.records.items.push({ record_id: `${conv}:r1:restart`, inbox: `texture:${DOC}`, sender_inbox: `research:${DOC}`, inbox_seq: 1, conversation_id: conv, round: 1, final: true, emitter_kind: 'system', report: { stop_reason: 'interrupted_by_restart' }, provenance: { committed_at: at(20) } });
  assert.equal(checkI2(closed).verdict, 'pass', checkI2(closed).reason);
  const answered = structuredClone(closed);
  Object.assign(answered.layers.records.items[1], { emitter_kind: 'agent', report: {} });
  assert.equal(checkI2(answered).verdict, 'fail', 'a live trajectory whose rounds were all answered owes nothing and names no reason');
});

test('I3: an observed quiet window after the boot passes; a short one is unknown', () => {
  // Keep only what happened before the restart: nothing dispatches after boot.
  const b = buildCleanBundle();
  const cut = Date.parse(b.drill.restart_requested_at);
  const before = (x) => {
    const t = Date.parse(x?.created_at || x?.ts || x?.started_at || '');
    return !Number.isFinite(t) || t < cut;
  };
  for (const name of ['runs', 'run_events', 'lifecycle_events', 'revisions', 'cells', 'egress', 'turn_commits', 'deliveries']) {
    const l = b.layers[name];
    if (l?.items) l.items = l.items.filter((x) => before(x.item || x));
  }
  const snap = b.layers.trajectory.items[0];
  for (const k of ['work_items', 'updates', 'co_super_assignments']) if (Array.isArray(snap[k])) snap[k] = snap[k].filter(before);
  const quiet = checkI3(b);
  assert.equal(quiet.verdict, 'pass', quiet.reason);
  assert.match(quiet.reason, /nothing was dispatched/);
  const short = structuredClone(b);
  short.exported_at = new Date(Date.parse(short.drill.boot_at || short.drill.ready_at) + 30_000).toISOString();
  assert.equal(checkI3(short).verdict, 'unknown');
});

test('I3 treats an upper-bound boot_at as approximate and reads from the restart request', () => {
  const b = buildCleanBundle();
  b.drill.boot_at_source = 'first answer after restart (upper bound)';
  b.drill.boot_at = new Date(Date.parse(b.drill.restart_requested_at) + 60_000).toISOString();
  const r = checkI3(b);
  assert.match(r.evidence[0].boot_at_source, /approximate/);
  assert.equal(Date.parse(r.evidence[0].boot_at), Date.parse(b.drill.restart_requested_at));
});

test('I6 and I7 on records: a gap, a cursor past head, a foreign closer and a packet on a records route fail', () => {
  const b = buildCleanBundle();
  const rec = (id, inbox, sender, seq, extra = {}) => ({ record_id: id, inbox, sender_inbox: sender, inbox_seq: seq, conversation_id: `conv:research:${DOC}|texture:${DOC}`, round: 1, provenance: { committed_at: at(10 + seq) }, ...extra });
  b.layers.records = { status: 'present', sources: ['GET /api/inboxes/{inbox}/records'], note: '', items: [
    rec('c1', `research:${DOC}`, `texture:${DOC}`, 1, { directive: { subtype: 'cast' } }),
    rec('r1', `texture:${DOC}`, `research:${DOC}`, 1, { final: true }),
  ] };
  b.layers.inboxes = { status: 'present', sources: ['same'], note: '', items: [
    { inbox: `research:${DOC}`, head: 1, cursor: 1, open_rounds: [] }, { inbox: `texture:${DOC}`, head: 1, cursor: 1, open_rounds: [] },
  ] };
  assert.equal(checkI6(b).verdict, 'pass', checkI6(b).reason);
  const gap = structuredClone(b);
  gap.layers.records.items[1].inbox_seq = 2;
  assert.equal(checkI6(gap).verdict, 'fail');
  const past = structuredClone(b);
  past.layers.inboxes.items[0].cursor = 5;
  assert.equal(checkI6(past).verdict, 'fail');
  const foreign = structuredClone(b);
  foreign.layers.records.items[1].sender_inbox = `management:${OWNER}`;
  assert.equal(checkI6(foreign).verdict, 'fail');
  const noCensus = structuredClone(b);
  delete noCensus.layers.census;
  assert.equal(checkI7(noCensus).verdict, 'unknown', 'without the census the rule has no work-state evidence, whatever the trajectory carries');
});

test('I7 reads the census: missing is unknown, a work item or packet on a records route fails, engineering is allowed', () => {
  const b = buildCleanBundle();
  b.layers.records = { status: 'present', sources: ['GET /api/inboxes/{inbox}/records'], note: '', items: [] };
  const census = (objects, extra = {}) => ({ status: 'present', sources: ['GET /api/runtime/census'], note: '', items: [{ kinds: ['choir.work_item', 'choir.worker_update'], counts: {}, objects, ...extra }] });
  const obj = (kind, from, to, id = 'o1') => ({ kind, canonical_id: id, from, to, created_at: at(5) });
  delete b.layers.census;
  assert.equal(checkI7(b).verdict, 'unknown', 'census layer absent');
  const unserved = structuredClone(b);
  unserved.layers.census = { status: 'unserved', sources: ['GET /api/runtime/census'], note: 'GET /api/runtime/census: 404', items: [] };
  assert.equal(checkI7(unserved).verdict, 'unknown');
  const empty = structuredClone(b);
  empty.layers.census = census([]);
  assert.equal(checkI7(empty).verdict, 'pass', checkI7(empty).reason);
  const wi = structuredClone(b);
  wi.layers.census = census([obj('choir.work_item', 'loop-1', `research:${DOC}`)]);
  assert.equal(checkI7(wi).verdict, 'fail');
  const pkt = structuredClone(b);
  pkt.layers.census = census([obj('choir.worker_update', `research:${DOC}`, `texture:${DOC}`)]);
  assert.equal(checkI7(pkt).verdict, 'fail');
  const eng = structuredClone(b);
  eng.layers.census = census([obj('choir.work_item', 'loop-1', 'engineering:e1'), obj('choir.worker_update', 'engineering:e1', `management:${OWNER}`, 'o2')]);
  assert.equal(checkI7(eng).verdict, 'pass', 'engineering work state stays route-scoped legal until B4');
  const trunc = structuredClone(eng);
  trunc.layers.census.items[0].truncated = true;
  assert.equal(checkI7(trunc).verdict, 'unknown');
  const engToTexture = structuredClone(empty);
  engToTexture.layers.records.items = [{ record_id: 'r9', inbox: `texture:${DOC}`, sender_inbox: 'engineering:e1', inbox_seq: 1 }];
  assert.equal(checkI7(engToTexture).verdict, 'fail');
});

test('required rules: a missing mandatory input fails I2, I3 and I7 once each on a planted snapshot', () => {
  const b = buildCleanBundle();
  const required = ['I2', 'I3', 'I7'];
  assert.equal(exitCode(checkBundle(b), required), 0, 'the clean bundle passes every required rule');
  // I2: a live trajectory owing nothing on work items, records unserved
  const i2 = structuredClone(b);
  i2.layers.trajectory.items[0].trajectory.status = 'live';
  i2.layers.records = { status: 'unserved', sources: [], note: 'test: unserved', items: [] };
  const r2 = Object.fromEntries(checkBundle(i2).map((r) => [r.rule, r.verdict]));
  assert.equal(r2.I2, 'unknown');
  assert.equal(exitCode(checkBundle(i2)), 0, 'without --require an unknown is not a failure');
  assert.equal(exitCode(checkBundle(i2), ['I2']), 1);
  // I3: records unserved
  const i3 = structuredClone(b);
  delete i3.layers.records;
  assert.equal(checkI3(i3).verdict, 'unknown');
  assert.match(checkI3(i3).reason, /records/);
  assert.equal(exitCode(checkBundle(i3), ['I3']), 1);
  // I7: census unserved
  const i7 = structuredClone(b);
  i7.layers.census = { status: 'unserved', sources: ['GET /api/runtime/census'], note: '404', items: [] };
  assert.equal(checkI7(i7).verdict, 'unknown');
  assert.equal(exitCode(checkBundle(i7), ['I7']), 1);
});

test('I3 counts a census object minted after the boot that the snapshot does not explain as ambiguous', () => {
  const b = buildCleanBundle();
  const late = Date.parse(b.exported_at) - 1000;
  assert.ok(late > Date.parse(b.drill.restart_requested_at));
  b.layers.census.items[0].objects = [{ kind: 'choir.worker_update', canonical_id: 'u-orphan', from: `research:${DOC}`, to: `texture:${DOC}`, created_at: new Date(late).toISOString() }];
  const r = checkI3(b);
  assert.notEqual(r.verdict, 'pass', r.reason);
  assert.ok(r.evidence.some((e) => e.dispatch === 'census_object_minted' && e.canonical_id === 'u-orphan'), JSON.stringify(r.evidence.slice(0, 2)));
});

test('exit code: 3 when no rule passed', () => {
  assert.equal(exitCode([{ rule: 'I1', verdict: 'unknown' }, { rule: 'I6', verdict: 'not_defined' }]), 3);
  assert.equal(exitCode([{ rule: 'I1', verdict: 'unknown' }, { rule: 'I2', verdict: 'fail' }]), 1);
});

test('I3 catches a research run started after the boot by a pre-boot record (Cut 1 records route)', () => {
  const b = buildCleanBundle();
  b.layers.records = { status: 'present', sources: ['GET /api/inboxes/{inbox}/records'], note: '', items: [
    { record_id: 'texture-cell:cast:tray-1', inbox: `research:${DOC}`, sender_inbox: `texture:${DOC}`, provenance: { agent_id: `texture:${DOC}`, committed_at: at(100) } },
  ] };
  b.layers.runs.items.push(run('run-research-inbox', `research:${DOC}`, 'research', 141, { state: 'completed', finished_at: at(145), metadata: { request_source: 'inbox_record' } }));
  const r = checkI3(b);
  assert.equal(r.verdict, 'fail', r.reason + ' ' + JSON.stringify(r.evidence));
  const hit = r.evidence.find((e) => e.dispatch === 'run_dispatched' && e.loop_id === 'run-research-inbox');
  assert.ok(hit, JSON.stringify(r.evidence));
  assert.ok(hit.causes.some((c) => c.kind === 'record' && c.resolved === 'pre_boot'), JSON.stringify(hit.causes));
});

test('I3 catches the documented restart recast of an engineering assignment', () => {
  const b = buildCleanBundle();
  const snap = b.layers.trajectory.items[0];
  const binding = (attempt) => ({ owner_id: OWNER, computer_id: COMPUTER, trajectory_id: TRAJ, parent_loop_id: 'run-mgmt-1', attempt });
  snap.co_super_assignments = [
    { schema: 'x', assignment_id: 'asg-1', binding: binding(1), disposition: 'cancelled', disposition_reason: 'restart revoked absent assignment capsule', created_at: at(60), updated_at: at(141) },
    { schema: 'x', assignment_id: 'asg-1', binding: binding(2), disposition: 'open', created_at: at(142), updated_at: at(142) },
  ];
  const r = checkI3(b);
  assert.equal(r.verdict, 'fail');
  assert.ok(r.evidence.some((e) => e.dispatch === 'assignment_opened' && e.assignment_id === 'asg-1'));
});

test('I3 catches a pre-boot run that resumes before any owner input (boot rewarm)', () => {
  const b = buildCleanBundle();
  b.layers.run_events.items.push(rev('loop.started', 145, 'run-mgmt-2', null, {}));
  const r = checkI3(b);
  assert.equal(r.verdict, 'fail');
  assert.ok(r.evidence.some((e) => e.dispatch === 'run_resumed' && e.loop_id === 'run-mgmt-2'));
});

test('I3 catches a pre-boot packet redelivered after the boot', () => {
  const b = buildCleanBundle();
  const p = b.layers.trajectory.items[0].updates[0];
  p.delivered_at = at(146);
  const r = checkI3(b);
  assert.equal(r.verdict, 'fail');
  assert.ok(r.evidence.some((e) => e.dispatch === 'packet_redelivered' && e.update_id === 'pkt-ctl-1'));
});

test('I3 catches delivery of a computer-scoped pre-boot packet the snapshot does not carry', () => {
  const b = buildCleanBundle();
  b.layers.lifecycle_events.items.push(lev('control_queued', 110, { update_id: 'pkt-directive-1', trajectory_id: '' }));
  b.layers.lifecycle_events.items.push(lev('control_delivered', 147, { update_id: 'pkt-directive-1', run_id: 'run-mgmt-2' }));
  const r = checkI3(b);
  assert.equal(r.verdict, 'fail');
  assert.ok(r.evidence.some((e) => e.dispatch === 'packet_redelivered' && e.update_id === 'pkt-directive-1'));
});

test('I3 does not fail a candidate violation while owner input is unreadable', () => {
  const planted = plantI3(buildCleanBundle());
  assert.equal(checkI3(planted).verdict, 'fail');
  assert.equal(checkI3(unserve(planted, 'revisions')).verdict, 'unknown');
});

test('I3 is unknown when a pre-boot-caused dispatch follows owner input but names no link', () => {
  const b = buildCleanBundle();
  // the reactivated Texture run no longer names the owner revision
  b.layers.runs.items[1].metadata.current_revision_id = 'rev-desk-1';
  assert.equal(checkI3(b).verdict, 'unknown');
});

test('I3 ignores the boot passivation of the in-flight run (a closure, not a dispatch)', () => {
  const r = checkI3(buildCleanBundle());
  assert.equal(r.verdict, 'pass');
  assert.equal(r.evidence[0].boot_at_source, 'first restart closure run event');
  assert.ok(!r.evidence.some((e) => e.loop_id === 'run-mgmt-2'));
});

test('I4: no quote fails; truncated body without the quote is unknown; absent body is unknown; normalisation tolerates spacing and curly quotes', () => {
  const b = buildCleanBundle();
  const noQuote = structuredClone(b);
  noQuote.layers.revisions.items[3].source_entities[0].selectors = [{ kind: 'whole_resource' }];
  assert.equal(checkI4(noQuote).verdict, 'fail');
  const trunc = structuredClone(b);
  for (const r of trunc.layers.revisions.items) for (const e of r.source_entities || []) if (e.source_entity_id === 'src-a') e.reader_snapshot = { text_content: 'Lab X said on Tuesday', truncated: true };
  assert.equal(checkI4(trunc).verdict, 'unknown');
  const nobody = structuredClone(b);
  for (const r of nobody.layers.revisions.items) for (const e of r.source_entities || []) delete e.reader_snapshot;
  assert.equal(checkI4(nobody).verdict, 'unknown');
  const legacy = structuredClone(b);
  legacy.layers.revisions.items[0].citations = [{ id: 'c1', type: 'url', value: 'https://news.example/x' }];
  assert.equal(checkI4(legacy).verdict, 'fail');
});

test('I5: no load is unknown; overlapping cells on one desk fail; egress from another worker fails; clean error passes', () => {
  const b = buildCleanBundle();
  // A desk wait is reported, not required: runs of one agent are serialized.
  const noWait = structuredClone(b);
  for (const c of noWait.layers.cells.items) c.payload.waited_for_run = '';
  assert.equal(checkI5(noWait).verdict, 'pass');
  const noLoad = structuredClone(b);
  noLoad.layers.egress.items = noLoad.layers.egress.items.slice(0, 1);
  const r = checkI5(noLoad);
  assert.equal(r.verdict, 'unknown');
  assert.equal(r.halves.request_id.verdict, 'pass');
  const overlap = structuredClone(b);
  const c2 = overlap.layers.cells.items.find((c) => c.loop_id === 'run-research-2');
  c2.payload.started_at = at(55);
  c2.payload.worker_pid = 9999;
  assert.equal(checkI5(overlap).verdict, 'fail');
  const foreign = structuredClone(b);
  foreign.layers.egress.items[0].worker_pid = 1;
  assert.equal(checkI5(foreign).verdict, 'fail');
  const cleanErr = structuredClone(b);
  Object.assign(cleanErr.layers.egress.items[0], { response_request_id: '', is_error: true, error: 'context canceled' });
  assert.equal(checkI5(cleanErr).verdict, 'pass');
});

test('I5 derives egress calls from the rows folded into desk.cell events (journal cut R4), as it did from desk.egress events', () => {
  const b = buildCleanBundle();
  const want = checkI5(b);
  assert.equal(want.verdict, 'pass');
  // A new build appends one desk.cell per cell with its calls in egress[];
  // the bundle's layers are then derived from run_events alone.
  const folded = structuredClone(b);
  delete folded.layers.egress;
  delete folded.layers.cells;
  const events = folded.layers.run_events.items;
  for (const e of events.filter((x) => x.kind === 'desk.egress')) {
    const cell = events.find((c) => c.kind === 'desk.cell' && c.loop_id === e.loop_id && c.payload.desk_session === e.payload.desk_session
      && Date.parse(c.payload.started_at) <= Date.parse(e.payload.started_at) && Date.parse(e.payload.finished_at) <= Date.parse(c.payload.finished_at));
    assert.ok(cell, `a cell holds egress ${e.payload.request_id}`);
    (cell.payload.egress ||= []).push(e.payload);
  }
  folded.layers.run_events.items = events.filter((x) => x.kind !== 'desk.egress');
  const got = checkI5(folded);
  assert.equal(got.verdict, 'pass', JSON.stringify(got.halves));
  assert.equal(got.halves.request_id.verdict, 'pass');
  assert.equal(got.halves.request_id.reason, want.halves.request_id.reason);
  assert.equal(got.load.appeared, want.load.appeared);
  // The rule still bites on a folded row: another caller's request id fails.
  const crossed = structuredClone(folded);
  const holder = crossed.layers.run_events.items.find((x) => x.kind === 'desk.cell' && x.payload.egress);
  holder.payload.egress[0].response_request_id = 'someone-else';
  assert.equal(checkI5(crossed).verdict, 'fail');
});

// --- CLIs

test('checker CLI: exit 0 on clean, 1 on a planted copy, 2 on unreadable input; --json writes the report', () => {
  const dir = mkdtempSync(join(tmpdir(), 'reeng-inv-'));
  const clean = join(dir, 'clean.json');
  writeFileSync(clean, JSON.stringify(buildCleanBundle()));
  const out = join(dir, 'report.json');
  const ok = spawnSync(process.execPath, [join(here, 'reeng_invariants.mjs'), clean, '--json', out], { encoding: 'utf8' });
  assert.equal(ok.status, 0, ok.stderr);
  assert.match(ok.stdout, /^I1 pass/m);
  assert.match(ok.stdout, /^I6 unknown/m);
  const report = JSON.parse(readFileSync(out, 'utf8'));
  assert.equal(report.rules.length, 7);
  assert.match(report.bundle_sha256, /^[0-9a-f]{64}$/);

  const plantedDir = join(dir, 'planted');
  execFileSync(process.execPath, [join(here, 'reeng_invariants_plant.mjs'), clean, plantedDir]);
  assert.deepEqual(readdirSync(plantedDir).sort(), RULES.map((r) => `planted-${r}.json`));
  for (const r of RULES) {
    const res = spawnSync(process.execPath, [join(here, 'reeng_invariants.mjs'), join(plantedDir, `planted-${r}.json`)], { encoding: 'utf8' });
    assert.equal(res.status, 1, `planted ${r} exit`);
    assert.match(res.stdout, new RegExp(`^${r} fail`, 'm'));
  }

  const bad = join(dir, 'bad.json');
  writeFileSync(bad, '{not json');
  assert.equal(spawnSync(process.execPath, [join(here, 'reeng_invariants.mjs'), bad]).status, 2);
  const wrong = join(dir, 'wrong.json');
  writeFileSync(wrong, JSON.stringify({ schema: 'other', layers: {} }));
  assert.equal(spawnSync(process.execPath, [join(here, 'reeng_invariants.mjs'), wrong]).status, 2);
  assert.equal(spawnSync(process.execPath, [join(here, 'reeng_invariants.mjs'), join(dir, 'missing.json')]).status, 2);
});
