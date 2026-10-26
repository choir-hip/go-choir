// The restore target of a proof run now comes from the checkpoints the
// computer binds automatically (GET /api/computers/{id}/lifecycle/checkpoints),
// not from a manual bind: POST /lifecycle/checkpoint is deleted
// (docs/design/apply-checkpoint-without-replay-2026-10-26.md, 4.4).

// pickNewestCheckpoint returns the newest listed checkpoint a restore would
// not refuse (restore refuses only a checkpoint the host judged
// not_derivable), or null. The listing is newest first.
export function pickNewestCheckpoint(listing) {
  const rows = Array.isArray(listing?.checkpoints) ? listing.checkpoints : [];
  return rows.find((row) => row?.checkpoint_digest && row?.verdict?.status !== 'not_derivable') || null;
}

// checkpointArtifact is the Checkpoint a restore takes, from the answer of
// GET ...?checkpoint_digest=D (a CheckpointResponse), or null.
export function checkpointArtifact(body) {
  const checkpoint = body?.checkpoint;
  return checkpoint && typeof checkpoint === 'object' && Object.keys(checkpoint).length > 0 ? checkpoint : null;
}

// restoredToPinnedCheckpoint is the restore leg's predicate: the restore
// matched the witness and the frontend, it restored the pinned checkpoint, and
// the apply had moved the head past it. The pinned checkpoint is the newest
// the cadence had bound, at or before the pre-apply head, so its head need not
// equal the pre-apply head.
export function restoredToPinnedCheckpoint(claim) {
  const restore = claim?.restore;
  const pinned = claim?.pre_apply_checkpoint;
  return Boolean(restore?.status === 200 && restore.witness_matched && restore.frontend_restaged
    && pinned?.checkpoint_digest && restore.checkpoint_digest === pinned.checkpoint_digest
    && claim.applied?.effective_head && claim.applied.effective_head !== pinned.effective_event_head);
}

// pinNewestCheckpoint reads the computer's checkpoint listing through api(path)
// (an object with body and status) and returns the Checkpoint to restore to,
// with the digest it was fetched under, or {checkpoint: null, error}. The
// computer binds checkpoints itself; when none exists the cadence has not run
// (it is off unless the host selected the computer).
export async function pinNewestCheckpoint(api, lifecyclePath) {
  const listing = await api(lifecyclePath('checkpoints'));
  const row = pickNewestCheckpoint(listing?.body);
  if (!row) {
    return { checkpoint: null, error: `no restorable checkpoint (status ${listing?.status}: ${listing?.body?.error || listing?.error || 'the computer has bound none; the cadence runs only on computers the host selected'})` };
  }
  const fetched = await api(`${lifecyclePath('checkpoints')}?checkpoint_digest=${encodeURIComponent(row.checkpoint_digest)}`);
  const checkpoint = checkpointArtifact(fetched?.body);
  if (!checkpoint) return { checkpoint: null, row, error: `checkpoint ${row.checkpoint_digest} could not be fetched (status ${fetched?.status})` };
  return { checkpoint, row };
}
