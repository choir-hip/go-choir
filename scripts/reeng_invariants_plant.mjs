#!/usr/bin/env node
// Planted-violation generator for the Cut 0 invariant checker
// (scripts/reeng_invariants.mjs). Acceptance row "the invariant checker run
// against planted-violation copies of a real bundle (one per rule)" in
// docs/definitions/choir-reeng-cut0-eyes-2026-10-12.md: a rule never shown
// failing on a planted violation does not count.
//
//   node scripts/reeng_invariants_plant.mjs <bundle.json> <outdir>
//
// writes <outdir>/planted-I1.json ... planted-I5.json. Each plantIn(bundle)
// returns a deep copy of any spec-conformant bundle with one violation of
// rule In added, touching nothing the other rules read where feasible.
// Planted objects carry ids prefixed "planted-" so they are easy to spot.

import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { readBundle, bootAt, ts, BUNDLE_SCHEMA } from './reeng_invariants.mjs';

const clone = (b) => structuredClone(b);
const iso = (ms) => new Date(ms).toISOString();
const str = (v) => (typeof v === 'string' ? v.trim() : '');

// ensureLayer makes a layer servable so a planted item can be read; an
// unserved layer becomes present (the plant documents itself in note).
function ensureLayer(b, name, rule) {
  b.layers = b.layers || {};
  const l = b.layers[name];
  if (!l || l.status === 'unserved' || !Array.isArray(l.items)) {
    b.layers[name] = { status: 'present', sources: l?.sources || [], note: `planted by ${rule}`, items: Array.isArray(l?.items) ? l.items : [] };
  } else if (l.status === 'empty') {
    l.status = 'present';
  }
  return b.layers[name];
}

function snapshot(b) {
  const l = b.layers?.trajectory;
  return l && Array.isArray(l.items) && l.items[0] ? l.items[0] : null;
}
function ownerOf(b) {
  return str(snapshot(b)?.trajectory?.owner_id) || str(b.layers?.runs?.items?.[0]?.owner_id) || str(b.layers?.revisions?.items?.[0]?.owner_id) || 'planted-owner';
}
function docOf(b) {
  return str(snapshot(b)?.document?.doc_id) || str(b.layers?.revisions?.items?.[0]?.doc_id) || 'planted-doc';
}
function earliest(b) {
  const times = [];
  for (const r of b.layers?.runs?.items || []) times.push(ts(r.created_at));
  for (const r of b.layers?.revisions?.items || []) times.push(ts(r.created_at));
  times.push(ts(snapshot(b)?.trajectory?.created_at));
  const req = ts(b.drill?.restart_requested_at);
  if (Number.isFinite(req)) times.push(req - 1000);
  const ok = times.filter(Number.isFinite);
  return ok.length ? Math.min(...ok) : ts(b.exported_at) - 3600_000 || Date.parse('2026-01-01T00:00:00Z');
}
function latestRevision(b) {
  const items = b.layers?.revisions?.items || [];
  return items.length ? items[items.length - 1] : null;
}

// I1: a revision written by a desk other than Texture (management), through
// a non-desk path. It keeps the template's body, sources and created_at, so
// I3 (owner inputs) and I4 (citations, same as the template) are unchanged.
export function plantI1(bundle) {
  const b = clone(bundle);
  const revs = ensureLayer(b, 'revisions', 'I1');
  const tpl = latestRevision(b) || { doc_id: docOf(b), owner_id: ownerOf(b), version_number: 0, content: '', created_at: iso(earliest(b)) };
  const nonTexture = (b.layers?.runs?.items || []).find((r) => !str(r.agent_id).startsWith('texture:'));
  revs.items.push({
    ...structuredClone(tpl),
    revision_id: 'planted-i1-revision',
    author_kind: 'appagent',
    author_label: 'management',
    version_number: (tpl.version_number || 0) + 1000,
    metadata: { source: 'management_cell', loop_id: nonTexture ? str(nonTexture.loop_id) : 'planted-i1-run' },
  });
  return b;
}

// I2: a run still running at export with no named reason. Created before any
// boot so I3 does not see it as a dispatch; it has no events or cells.
export function plantI2(bundle) {
  const b = clone(bundle);
  const runs = ensureLayer(b, 'runs', 'I2');
  const tpl = runs.items[0] || {};
  const m = { ...(tpl.metadata || {}) };
  delete m.passivated_reason;
  delete m.submission_surface;
  delete m.input_source;
  const created = iso(earliest(b));
  const run = {
    ...structuredClone(tpl),
    loop_id: 'planted-i2-run',
    agent_id: str(tpl.agent_id) || 'management',
    owner_id: str(tpl.owner_id) || ownerOf(b),
    trajectory_id: str(tpl.trajectory_id) || str(b.trajectory_id),
    state: 'running',
    created_at: created,
    updated_at: created,
    metadata: m,
  };
  delete run.error;
  delete run.result;
  delete run.finished_at;
  delete run.requested_by_run_id;
  runs.items.push(run);
  return b;
}

// I3: right after a markerless boot, an engineering run is dispatched at the
// request of a run that predates the boot, before any owner input. If the
// bundle has no drill or the boot was planned, the copy gets a markerless
// drill (restart kind crash_or_stop) so the rule has a boot to judge.
export function plantI3(bundle) {
  const b = clone(bundle);
  const runs = ensureLayer(b, 'runs', 'I3');
  // I3 reads owner input from runs and revisions; an unserved revisions
  // layer becomes an answered empty one so the plant is judged, not blinded.
  const rl = b.layers.revisions;
  if (!rl || rl.status === 'unserved' || !Array.isArray(rl.items)) b.layers.revisions = { status: 'empty', sources: rl?.sources || [], note: 'planted by I3', items: [] };
  if (!b.drill || !Number.isFinite(ts(b.drill.restart_requested_at))) {
    const times = runs.items.map((r) => ts(r.created_at)).filter(Number.isFinite).sort((x, y) => x - y);
    const at = times.length ? times[Math.floor(times.length / 2)] + 1 : ts(b.exported_at) - 600_000;
    b.drill = { ...(b.drill || {}), label: 'planted-I3', restart_requested_at: iso(at), ready_at: iso(at + 30_000) };
  }
  const bootLayer = b.layers?.boot;
  if (bootLayer && bootLayer.status === 'present' && bootLayer.items?.[0]) {
    bootLayer.items[0].restart = { kind: 'crash_or_stop', reason: '' };
  } else {
    b.drill.obligations_after = { ...(b.drill.obligations_after || {}), restart: { kind: 'crash_or_stop' } };
  }
  const B = bootAt(b).at;
  let cause = runs.items.filter((r) => ts(r.created_at) < B).sort((x, y) => ts(x.created_at) - ts(y.created_at))[0];
  if (!cause) {
    cause = { loop_id: 'planted-i3-preboot-run', agent_id: 'management', owner_id: ownerOf(b), trajectory_id: str(b.trajectory_id), state: 'completed', prompt: 'planted', created_at: iso(B - 60_000), updated_at: iso(B - 30_000), finished_at: iso(B - 30_000), metadata: {} };
    runs.items.push(cause);
  }
  const t = B + 1;
  runs.items.push({
    loop_id: 'planted-i3-run',
    agent_id: 'engineering:planted',
    agent_profile: 'engineering',
    requested_by_run_id: str(cause.loop_id),
    owner_id: str(cause.owner_id) || ownerOf(b),
    computer_id: str(cause.computer_id) || str(b.computer_id),
    trajectory_id: str(cause.trajectory_id) || str(b.trajectory_id),
    state: 'completed',
    prompt: 'planted boot resume',
    result: 'planted',
    created_at: iso(t),
    updated_at: iso(t + 1000),
    finished_at: iso(t + 1000),
    metadata: { request_source: 'planted_boot_resume' },
  });
  return b;
}

// I4: a revision (clone of the latest, same author so I1 is unchanged) whose
// one citation quotes text that its own source body does not contain.
export function plantI4(bundle) {
  const b = clone(bundle);
  const revs = ensureLayer(b, 'revisions', 'I4');
  const owner = ownerOf(b);
  const tpl = latestRevision(b) || { doc_id: docOf(b), owner_id: owner, author_kind: 'user', author_label: owner, version_number: 0, created_at: iso(earliest(b)) };
  const rev = structuredClone(tpl);
  rev.revision_id = 'planted-i4-revision';
  rev.version_number = (tpl.version_number || 0) + 2000;
  rev.body_doc = {
    schema: 'choir.texture_doc.v1',
    doc: {
      type: 'doc',
      attrs: { id: 'planted-i4-doc' },
      content: [
        {
          type: 'paragraph',
          attrs: { id: 'planted-i4-p1' },
          content: [
            { type: 'text', text: 'A planted claim. ' },
            { type: 'source_ref', attrs: { id: 'planted-i4-ref', source_entity_id: 'planted-i4-source', display_mode: 'numbered_ref' } },
          ],
        },
      ],
    },
  };
  rev.source_entities = [
    {
      source_entity_id: 'planted-i4-source',
      target: { kind: 'web_url', uri: 'https://planted.invalid/source' },
      selectors: [{ kind: 'text_quote', data: { exact: 'a sentence the source never says' } }],
      display: { mode: 'numbered_ref', title: 'Planted source' },
      evidence: { state: 'available', open_surface: 'source' },
      provenance: { created_by: 'research' },
      reader_snapshot: { text_content: 'The source body talks about something else entirely.', truncated: false },
    },
  ];
  rev.citations = [];
  delete rev.source_entity_objects;
  delete rev.source_refs;
  revs.items.push(rev);
  return b;
}

// I5: one egress result carries another caller's request id (the cross-wire
// defect). Mutates the first egress item (and its run event); with no egress
// in the bundle, adds one call at the start of an existing run so I3 sees no
// post-boot activity.
export function plantI5(bundle) {
  const b = clone(bundle);
  const egress = ensureLayer(b, 'egress', 'I5');
  const runEvents = ensureLayer(b, 'run_events', 'I5');
  if (b.layers.cells && b.layers.cells.status === 'unserved') b.layers.cells = { status: 'empty', sources: [], note: 'planted by I5', items: [] };
  const payloadOf = (it) => (it && typeof it === 'object' && 'payload' in it && 'kind' in it ? it.payload : it);
  if (egress.items.length > 0) {
    const first = payloadOf(egress.items[0]);
    const other = egress.items.map(payloadOf).find((p) => str(p.request_id) && str(p.request_id) !== str(first.request_id));
    const crossed = other ? str(other.request_id) : `${str(first.request_id) || 'planted'}:crossed`;
    const req = str(first.request_id);
    first.response_request_id = crossed;
    first.is_error = false;
    for (const ev of runEvents.items) {
      if (ev.kind === 'desk.egress' && ev.payload && str(ev.payload.request_id) === req) {
        ev.payload.response_request_id = crossed;
        ev.payload.is_error = false;
      }
      // Egress folded into a desk.cell event (journal cut R4) is mutated too.
      if (ev.kind === 'desk.cell' && ev.payload && Array.isArray(ev.payload.egress)) {
        for (const row of ev.payload.egress) {
          if (row && str(row.request_id) === req) {
            row.response_request_id = crossed;
            row.is_error = false;
          }
        }
      }
    }
    return b;
  }
  const run = (b.layers?.runs?.items || [])[0] || { loop_id: 'planted-i5-run', created_at: iso(earliest(b)), agent_id: 'research' };
  const t = ts(run.created_at);
  const start = Number.isFinite(t) ? t + 1 : earliest(b);
  const payload = {
    request_id: 'planted-i5-req-a',
    response_request_id: 'planted-i5-req-b',
    verb: 'web_search',
    desk_session: `agent:${str(run.agent_id) || 'research'}`,
    worker_pid: 1,
    run_id: str(run.loop_id),
    started_at: iso(start),
    finished_at: iso(start + 1),
    is_error: false,
    error: '',
  };
  egress.items.push(payload);
  runEvents.items.push({ event_id: 'planted-i5-event', seq: 1_000_000, ts: iso(start), loop_id: str(run.loop_id), kind: 'desk.egress', payload: { ...payload } });
  return b;
}

export const PLANTS = { I1: plantI1, I2: plantI2, I3: plantI3, I4: plantI4, I5: plantI5 };

function main(argv) {
  const [, , input, outdir] = argv;
  if (!input || !outdir) {
    process.stderr.write('usage: node scripts/reeng_invariants_plant.mjs <bundle.json> <outdir>\n');
    return 2;
  }
  let loaded;
  try {
    loaded = readBundle(input);
  } catch (err) {
    process.stderr.write(`reeng_invariants_plant: cannot read bundle ${input}: ${err.message}\n`);
    return 2;
  }
  if (loaded.bundle.schema !== BUNDLE_SCHEMA) return 2;
  mkdirSync(outdir, { recursive: true });
  for (const [rule, plant] of Object.entries(PLANTS)) {
    const out = join(outdir, `planted-${rule}.json`);
    writeFileSync(out, JSON.stringify(plant(loaded.bundle), null, 2) + '\n');
    process.stdout.write(`${rule} ${out}\n`);
  }
  return 0;
}

if (import.meta.url === pathToFileURL(process.argv[1] || '').href) {
  process.exitCode = main(process.argv);
}
