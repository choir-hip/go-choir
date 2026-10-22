// Pure acceptance predicates for the engineering scenario of
// scripts/reeng_tracer.mjs, kept apart so node --test can prove them
// (scripts/reeng_tracer_checks_test.mjs). Drill selfdev-jobs-0101
// (docs/problems/selfdev-go-drill-jobs-0101-2026-10-15.md) failed four
// checks that were stale, not the product:
//  - the cast check knew only Texture's first Cast, so a re-cast story
//    (Texture rounds 2-4) failed it;
//  - the bundle checks compared the Report with the operation's current
//    digest, which Verify replaces (finalizeVerifiedCapsuleBundle);
//  - the job checks read the first assignment's inbox and matched argv
//    word by word, so `sh -c "... go test ..."` was missed.

import { createHash } from 'node:crypto';

// textureCastIDs is every Texture Cast to management the drill saw.
export function textureCastIDs(claim) {
  const ids = new Set();
  for (const c of claim?.texture_casts_to_management || []) if (c?.record_id) ids.add(c.record_id);
  if (claim?.texture_cast_to_management?.record_id) ids.add(claim.texture_cast_to_management.record_id);
  return ids;
}

// castOnRecords: management's engineering Cast names a Texture Cast to
// management as its parent (any round).
export function castOnRecords(claim, cast) {
  return Boolean(cast?.record_id && cast.parent_id && textureCastIDs(claim).has(cast.parent_id));
}

// bundleDigestsOf is the digests an engineering Report may name for an
// operation: the one it carried in state frozen (Freeze's content digest),
// and its current one (equal to it until Verify finalizes the bundle under
// a new digest).
export function bundleDigestsOf(claim, op) {
  const digests = new Set();
  if (!op?.operation_id) return digests;
  for (const s of claim?.operation_states || []) {
    if (s?.operation_id && s.operation_id !== op.operation_id) continue;
    if (s?.state === 'frozen' && s.bundle_digest) digests.add(s.bundle_digest);
  }
  if (op.frozen_bundle_digest) digests.add(op.frozen_bundle_digest);
  if (op.bundle_digest) digests.add(op.bundle_digest);
  return digests;
}

// reportNamesBundle: the completing Report names this operation and a
// digest this operation carried.
export function reportNamesBundle(claim, report, op) {
  return Boolean(report && op && report.operation_id === op.operation_id && report.bundle_digest
    && bundleDigestsOf(claim, op).has(report.bundle_digest));
}

// verifiedNamedBundle: a verifier ref moved the operation the Report names
// to awaiting_approval.
export function verifiedNamedBundle(claim, report, op) {
  return Boolean(claim?.verify && (claim.verify.verifier_refs || []).length > 0 && reportNamesBundle(claim, report, op));
}

const argvText = (job) => (Array.isArray(job?.argv) ? job.argv.join(' ') : '');

export const isGoBuildOfAutoputer = (job) => /\bgo\s+build\b/.test(argvText(job)) && argvText(job).includes('autoputer');
export const isGoTest = (job) => /\bgo\s+test\b/.test(argvText(job));

// goTestCells returns the passing go test check cells of one engineering
// inbox, from its run events (a result carries the cell source since journal
// cut R3; older tapes pair it with the tool.invoked row): a capsule_go_eval cell whose source runs go
// test through choir.Exec and panics when it fails, and whose result exited
// 0 with a capsule-go-eval receipt. The engineering prompt and Freeze admit
// such a cell as test evidence beside a test job.
const parsePayload = (p) => {
  if (typeof p !== 'string') return p || {};
  try { return JSON.parse(p); } catch { return {}; }
};
const GO_TEST_SOURCE = [
  /choir\.Exec\(\s*"sh"[\s\S]*\bgo\s+test\b/,
  /choir\.Exec\(\s*"go"\s*,\s*\[\]string\{[^}]*"test"/,
];
export const isGoTestCellSource = (src) => GO_TEST_SOURCE.some((re) => re.test(src)) && /\bpanic\(/.test(src);

export function goTestCells(events, inbox) {
  const sources = new Map();
  const passed = [];
  for (const e of events || []) {
    if (e?.agent_id !== inbox) continue;
    const p = parsePayload(e.payload);
    if (p.tool !== 'capsule_go_eval' || !p.call_id) continue;
    if (e.kind === 'tool.invoked') {
      sources.set(p.call_id, String(p.arguments?.source || ''));
      continue;
    }
    if (e.kind !== 'tool.result') continue;
    // tool.invoked is a bus-only line since journal cut R3: a new tape's
    // result carries the arguments itself. An old tape's result is matched
    // to its invoked row by call_id.
    const source = p.arguments ? String(p.arguments.source || '') : sources.get(p.call_id) || '';
    if (!isGoTestCellSource(source)) continue;
    const result = parsePayload(p.output);
    const receipt = String(result.receipt_ref || '');
    if (p.is_error || result.error || result.exit_code !== 0 || !receipt.startsWith('capsule-go-eval:sha256:')) continue;
    passed.push({ call_id: p.call_id, receipt_ref: receipt });
  }
  return passed;
}

// doneJobs keeps the jobs that ended done with a receipt.
export const doneJobs = (jobs) => (jobs || []).filter((j) => j?.state === 'done' && j.receipt_ref);

// completingInbox is the engineering inbox whose jobs the bundle came from:
// the sender of the completing Report, else the newest Cast's inbox.
export function completingInbox(claim, report) {
  if (report?.sender) return report.sender;
  const casts = claim?.engineering_casts || [];
  return casts.length ? casts[casts.length - 1].inbox : null;
}

// ---- --visual drill (docs/design/visual-evidence-slice-2026-10-15.md section 5) ----

const PNG_MAGIC = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a];
const sha256Of = (bytes) => createHash('sha256').update(bytes).digest('hex');
const isPng = (bytes) => Boolean(bytes && bytes.length >= PNG_MAGIC.length && PNG_MAGIC.every((b, i) => bytes[i] === b));

// visualEvidenceChecks decides the --visual drill's claims from what the
// drill read through product routes: the operation's evidence listing, the
// asset the listing points at, and a Range request on it. The drill's page is
// /choir-drill/<token>.html. It claims existence, binding and serving; what a
// vision model reads off the picture is observed by the drill, never a check
// here. Every group fails closed on a missing input.
export function visualEvidenceChecks({ token, listing, asset, range }) {
  const body = listing?.status === 200 ? listing.body : null;
  const captures = Array.isArray(body?.captures) ? body.captures : [];
  const route = `/choir-drill/${token}.html`;
  const capture = captures.find((c) => c?.route === route) || null;
  const digest = capture?.digest || '';
  const headers = asset?.headers || {};
  const bytes = asset?.bytes || null;
  const served = asset?.status === 200 && Boolean(bytes);
  const requested = (bytes && range?.bytes) ? bytes.subarray(0, 8) : null;
  return {
    visual_listing: Boolean(body && captures.length > 0),
    visual_capture_receipted: typeof body?.capture_receipt === 'string' && /^capsule-exec:sha256:[0-9a-f]{64}$/.test(body.capture_receipt),
    visual_drill_route_captured: Boolean(capture),
    visual_capture_not_blank: Boolean(capture) && capture.blank === false,
    visual_capture_viewport: Boolean(capture) && capture.width === 1280 && capture.height === 800,
    visual_capture_resolvable: Boolean(capture) && capture.resolvable === true && capture.asset_url === `/api/assets/sha256/${digest}`,
    visual_asset_headers: served && String(headers['content-type'] || '').split(';')[0].trim() === 'image/png'
      && String(headers['x-content-type-options'] || '').toLowerCase() === 'nosniff'
      && /\bimmutable\b/.test(String(headers['cache-control'] || '')) && /\bprivate\b/.test(String(headers['cache-control'] || ''))
      && headers.etag === `"sha256-${digest}"`,
    visual_asset_bytes_match_digest: served && Boolean(digest) && sha256Of(bytes) === digest,
    visual_asset_is_png: served && isPng(bytes),
    visual_asset_range: range?.status === 206 && Boolean(requested) && Buffer.compare(Buffer.from(range.bytes), Buffer.from(requested)) === 0
      && range.headers?.['content-range'] === `bytes 0-7/${bytes?.length}`,
  };
}

// ---- release witness (S6-0) ------------------------------------------------
//
// The engineering go-served scenario fetches
// GET /api/computers/{id}/self-development/operations/{op}/release-witness at
// four moments and keeps each in the receipt. `witness_joined` holds when every
// fetch answered with every expected pair, each carrying a gate flag, the same
// flag at every moment, and a reason.
// WITNESS_PAIRS must equal the Go table selfdevGatePairs
// (internal/agentcore/selfdev_release_witness.go); a Go test reads this list.
export const WITNESS_SCHEMA = 'choir.selfdev.release_witness.v1';
export const WITNESS_TAGS = ['frozen', 'accepted', 'applied', 'restored'];
export const WITNESS_PAIRS = [
  'decision_operation',
  'decision_expected_heads',
  'bundle_operation',
  'test_receipts_exit',
  'bundle_source_patch',
  'bundle_host_build',
  'builder_receipt_patch',
  'manifest_closure_commit',
  'manifest_builder_receipt',
  'manifest_decision',
  'decision_pending_transition',
  'frozen_surface_decision_heads',
  'checkpoint_event',
  'route_event',
  'serving_current',
  'effective_release_operation',
  'divergence_derived',
];

// witnessJoined reads {tag: {status, body}} and says whether every tag holds a
// complete witness. It never reads `bound` for the verdict: a join may be
// unbound and the check still holds (the gap table is the finding, not a
// failure of the witness).
export function witnessJoined(witnesses) {
  const missing = [];
  const unexpected = [];
  const malformed = [];
  const tagsFailed = [];
  const flags = new Map();
  const inconsistent = [];
  for (const tag of WITNESS_TAGS) {
    const w = witnesses?.[tag];
    const body = w?.body;
    if (!w || w.status !== 200 || body?.schema !== WITNESS_SCHEMA || !Array.isArray(body.bindings)) {
      tagsFailed.push(tag);
      continue;
    }
    const seen = new Map();
    for (const b of body.bindings) seen.set(b?.pair, (seen.get(b?.pair) || 0) + 1);
    for (const pair of WITNESS_PAIRS) {
      if (seen.get(pair) !== 1) missing.push(`${tag}:${pair}`);
    }
    for (const b of body.bindings) {
      if (!WITNESS_PAIRS.includes(b?.pair)) { unexpected.push(`${tag}:${b?.pair}`); continue; }
      const boundOK = b.bound === null || typeof b.bound === 'boolean';
      if (typeof b.compared_at_gate !== 'boolean' || !boundOK || typeof b.reason !== 'string' || b.reason.trim() === '') {
        malformed.push(`${tag}:${b.pair}`);
      }
      if (flags.has(b.pair) && flags.get(b.pair) !== b.compared_at_gate) inconsistent.push(b.pair);
      flags.set(b.pair, b.compared_at_gate);
    }
  }
  return {
    ok: tagsFailed.length === 0 && missing.length === 0 && unexpected.length === 0 && malformed.length === 0 && inconsistent.length === 0,
    tags_failed: tagsFailed, missing, unexpected, malformed, inconsistent: [...new Set(inconsistent)],
  };
}

// witnessGapTable is one row per pair: whether a gate compares it and what the
// witness observed at each moment (true bound, false unbound, null unknown,
// undefined absent from that fetch).
export function witnessGapTable(witnesses) {
  const rows = [];
  for (const pair of WITNESS_PAIRS) {
    const row = { pair, compared_at_gate: null, compared_at: null, bound: {}, reason: {} };
    for (const tag of WITNESS_TAGS) {
      const b = (witnesses?.[tag]?.body?.bindings || []).find((x) => x?.pair === pair);
      if (!b) { row.bound[tag] = undefined; continue; }
      row.compared_at_gate = b.compared_at_gate;
      row.compared_at = b.compared_at;
      row.bound[tag] = b.bound;
      row.reason[tag] = b.reason;
    }
    rows.push(row);
  }
  return rows;
}

// formatWitnessGapTable prints the table: pair, whether a gate compares it,
// then bound at each moment (Y bound, N unbound, - unknown, ? absent).
export function formatWitnessGapTable(rows) {
  const mark = (v) => (v === true ? 'Y' : v === false ? 'N' : v === null ? '-' : '?');
  const width = Math.max(...rows.map((r) => r.pair.length), 4);
  const lines = [`${'pair'.padEnd(width)}  ${'compared_at_gate'.padEnd(26)}  ${WITNESS_TAGS.map((t) => t.padEnd(8)).join(' ')}`];
  for (const r of rows) {
    const gate = r.compared_at_gate === true ? `yes (${r.compared_at})` : r.compared_at_gate === false ? `NO (${r.compared_at})` : '?';
    lines.push(`${r.pair.padEnd(width)}  ${gate.padEnd(26)}  ${WITNESS_TAGS.map((t) => mark(r.bound[t]).padEnd(8)).join(' ')}`);
  }
  return lines.join('\n');
}
