// The restore target of a proof run now comes from the checkpoints the
// computer binds automatically (GET /api/computers/{id}/lifecycle/checkpoints),
// not from a manual bind: POST /lifecycle/checkpoint is deleted
// (docs/design/apply-checkpoint-without-replay-2026-10-26.md, 4.4).

// pickNewestCheckpoint returns the newest listed checkpoint a restore would
// not refuse, or null. The listing is newest first.
export function pickNewestCheckpoint(listing) {
  return null;
}

// checkpointArtifact is the Checkpoint a restore takes, from the answer of
// GET ...?checkpoint_digest=D (a CheckpointResponse), or null.
export function checkpointArtifact(body) {
  return null;
}

// restoredToPinnedCheckpoint is the restore leg's predicate: the restore
// matched the witness and the frontend, it restored the pinned checkpoint, and
// the apply had moved the head past it. The pinned checkpoint is the newest
// the cadence had bound, at or before the pre-apply head, so its head need not
// equal the pre-apply head.
export function restoredToPinnedCheckpoint(claim) {
  return false;
}
