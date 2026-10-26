# A failed operation turns an owner's new direction into a wait (R10-1) (2026-10-26)

Status: documented; fix not built. Class of the fix: orange (Texture run
context and operation selection, no state write), review tier 1.

## Evidence (staging, d@e, computer-59c902c5)

- Document fc280237, trajectory 1c53ea26. Themes round 1's operation
  selfdev-f6412cdc failed at 10:04:08 (verify rejected it).
- At 10:05:42 the owner revised the document with a round 2 direction (revision
  72169553). The revision woke Texture (run 3cf6c53f, request intent
  apply_owner_revision); the full direction was in its prompt.
- Texture ran two cells in 13.9 s: `choir.Records()` (empty), then
  `choir.Wait("Round 1 remains failed ...; this supervision turn has no new
  self-development report, and its instructions prohibit re-running or
  requesting verification, approval, or application. Await a report through
  choir.Updates().")`. No Cast, no revision. The direction was dropped.
- Trace review round 10, section 2 (docs/evidence/selfdev-trace-reviews-2026-10-25.md).

## Cause (code finding; delivery to the model is a hypothesis)

`runContextForRun` (internal/agentcore/run_context.go) appends a
"Self-development supervision" block to every Texture run whose trajectory has
a self-development operation: "Do not ask to verify, approve, apply or re-run
it ... When a report adds nothing the document needs, call choir.Wait(why)".
The operation comes from `textureRunSelfDevelopmentOperation` via
`pickTrajectoryOperation` (internal/agentcore/freeze_candidate.go), which skips
terminal operations but then falls back to the newest one when all are
terminal. So a document whose only operation has failed is told it supervises
that operation, and an owner revision asking for a new round reads as a
forbidden re-run.

The Wait text matches the block phrase by phrase; the trace export does not
carry the run-context message, so that the block reached the model is not
directly observed.

## Consequences

The owner's way out was a new document, which loses the carried tree (R10-7)
and leaves the old request open, which a later Texture turn on another
document then read as a conflicting open request ("awaiting the owner's choice
before dispatch", d@e trajectory b0b9cddf, 10:45:25).

## Fix design

1. Operation selection for the run context returns no operation when every
   operation on the trajectory is terminal (failed, rejected, rolled back,
   applied). The supervision block is then not attached.
2. When the block is attached, it names the operation's state, and it says
   that an owner revision with new direction (intent apply_owner_revision) is
   a new round to Cast, not a re-run.
3. The trace export carries the run-context message, so the next review can
   see what the model was told.

Tests first: a trajectory whose only operation failed gets no supervision
block; a live operation still gets it; the block text for an
apply_owner_revision run says a new direction is a new round.

Rollback: revert; the block returns to today's selection.
