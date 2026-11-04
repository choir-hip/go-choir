# Texture settles a rejected round as complete (2026-11-04)

## Problem

On the crouton arm (computer-848b21d55df1dd1a953ae798b8edc696, model
stealth/crouton-alpha, no delegate), Calendar round 8 used all six candidates
and every one was rejected. The last, selfdev-a2f83150, was failed at
11:14:14Z with 10 of 12 shots rejected and the zone fault (events placed at
their own wall-clock time under a UTC header) still present.

At 11:14:29Z, fifteen seconds later, crouton's Texture settled the round:
"done: Calendar round 8 complete: package calendar-next built, 12 fresh
screens verified (2 phone at phone size, 10 desktop at desktop size),
owner's 11:07Z/11:08Z/11:13Z directions applied".

That is a success claim the artifact contradicts: no candidate passed, the
screens were judged, not verified good, and the zone direction was never
applied.

## Evidence

- Lane C report 11:15Z, op selfdev-a2f83150, trajectory eb3a2cc5,
  assignment-a7772e7e; Texture settle event 11:14:29Z on doc 1d045237.
- live8 11:25Z: six candidates 10:55:41 to 11:13:34, all failed.

## Hypothesis (not yet confirmed from the ledger)

The settle text is written by the desk's model from its own account of the
work, not derived from the operation's terminal fate. If so, a model that
lost track of the verdicts can report success. Test: read crouton's
management turn that wrote the settle and check whether the operation fate
(rejected, cap reached) was in its context.

## Not yet decided

Whether a round's settle line must be derived from, or checked against, the
operation fate before Texture records it. The crouton-with-delegate arm
(claude r3) ended the same way; check whether its settle was honest.

## Root cause (code path, 2026-11-04, read from code; not yet confirmed on a staging ledger)

The send-back loop exists inside one engineering assignment
(`internal/agentcore/engineer_reuse.go`): a rejected candidate resumes the
live engineer with the reasons, for candidates 1 to
`store.EngineeringFreezeMaxRevisions` (6). The sixth candidate takes the old
order: the engineer's Complete commits, the round reads as complete, and
management's later Verify fail changes only the operation. Nothing joined the
operation's fate to Texture's turn: Texture's `Done` (`commitTextureTurn` in
`internal/textureowner/texture_cell_author.go`) completes the work item and
the trajectory settles. The model's account of the work was the only source of
the settle text. The same hole covers an owner or consensus reject.

## Fix (owner ruling 2026-11-04: "Rejected rounds don't complete but get sent back")

`agentcore.RejectedRoundOf` reads the trajectory's operations. A `Done` on a
trajectory whose last candidate was rejected, with nothing live, is refused:
with attempts left it carries the reasons and says to Cast engineering again;
with the budget (6) spent, the work item is refused with the fate "rejected
after N attempts: reasons". A crash close and an owner abandon fail an
operation with no verdict, so they are not rejections; nothing dispatches, so
a restart never re-dispatches.

## Residual

The send-back after an early engineer exit (attempts left, engineer gone) is
a forced Cast by the Texture desk, not an automatic new assignment.
