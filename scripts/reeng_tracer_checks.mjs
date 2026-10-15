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

// doneJobs keeps the jobs that ended done with a receipt.
export const doneJobs = (jobs) => (jobs || []).filter((j) => j?.state === 'done' && j.receipt_ref);

// completingInbox is the engineering inbox whose jobs the bundle came from:
// the sender of the completing Report, else the newest Cast's inbox.
export function completingInbox(claim, report) {
  if (report?.sender) return report.sender;
  const casts = claim?.engineering_casts || [];
  return casts.length ? casts[casts.length - 1].inbox : null;
}
