# The carry drops the newest candidate: a bind during a pending verify carries nothing, and an older stopped round outranks it (2026-10-26)

Residual register rows SD-03 and SD-04
(docs/evidence/residual-register-2026-10-26.md); trace review Round 5,
problems R5-3 and R5-4 (docs/evidence/selfdev-trace-reviews-2026-10-25.md).
Mutation class of this record: green. The fix is red (the carry decides what
source an engineering round starts from).

## What

The owner's intent for self-development iteration (2026-10-25: "if we get
part of the work done, but not everything, we can iteratively improve on it
rather than having to start over") is that each engineering round is a
differential next step on the newest candidate the owner has seen. Two
selection rules in the carry (internal/agentcore/carried_patch.go) break it,
and both fail silently: the engineer is told nothing, or told about the wrong
tree.

1. **SD-03, a bind during a pending verify carries nothing.**
   `pickCarriedCandidate` sorts the trajectory's rounds newest first and ends
   the chain when the newest is not carriable. A candidate whose
   self-development operation is still open (frozen, verified,
   awaiting_approval, accepted, materializing, and the other
   `selfdev.OpenStates`) is not carriable, so a round that binds inside that
   window is offered nothing. The offer is decided once per assignment and
   recorded, so the round never learns the candidate settled a minute later.
   It starts from the base and rebuilds work that already exists.
2. **SD-04, a stopped round that did not start from the newest candidate
   outranks it.** With `engineering.carry_unfinished` on, a round stopped
   before it froze joins the history as an unfinished round dated when its
   capsule was snapshotted (`mergeUnfinishedRounds`, unfinished_patch.go).
   Newest wins, so that kept patch outranks the older frozen candidate even
   when the stopped round started from the base. `nameOlderFrozenCandidate`
   and `OlderFrozenStartedFrom` (carried_patch.go:181, 503-524, 586-600)
   only name the case in the notice; they do not gate it.

## Evidence

Round 5 (reader-4 repair, trajectory ac0cb480, computer b@c, 2026-10-25
06:36-06:51; `choir trace export` at 06:45, 06:48, 06:51, 06:52):

- Assignment 81bbc97b froze candidate selfdev-5820206f at 06:42:42 and
  completed 06:43:11. Management's verify (run b3dbc3b5, 06:43:11-06:43:59)
  failed the operation at 06:43:35.
- Assignment 4513328f (opened by the duplicate cast, R5-2) bound at 06:43:13,
  two seconds after 81bbc97b completed and 22 seconds before the verify
  failed the operation. Its capsule had no /selfdev/carried; by 06:49 it had
  spent 45 iterations rebuilding the Reader from the base.
- By code: had 4513328f been cancelled with carry_unfinished on, its kept
  snapshot (CreatedAt = TakenAt, the newest) would have won the next carry
  over 5820206f, so the round after it would carry the restart, not the
  candidate.
- FP0's manual workaround: carry_unfinished off at 06:49:22, cancel 4513328f
  at 06:49:31 (no snapshot), on again at 06:50:45. Assignment d989b783 then
  spawned with carried=true from 81bbc97b's tree.

## Where

- internal/agentcore/carried_patch.go: `pickCarriedCandidate` (the chain
  end at :219-238), `offerCarriedPatch` (decided once, :254),
  `nameOlderFrozenCandidate` (:503).
- internal/agentcore/unfinished_patch.go: `mergeUnfinishedRounds` (a kept
  round's CreatedAt is its snapshot time).
- internal/agentcore/engineering_admission.go: `admitEngineeringAssignment`,
  the one place a request can wait without being cancelled.

## Why it matters

The carried patch is the only thing that makes a round build on the last
one. Losing it costs a full rebuild (Round 5: 45 iterations before FP0
noticed), and the rebuild can differ from what the owner already reviewed,
so the owner reviews a different change than the one they asked to have
revised. A round that binds while the trajectory's candidate is still open
is also doomed at Freeze: the self-development store allows one open
operation per computer (`selfdev.ErrOperationOpen`), so its Freeze is
refused unless the earlier operation settles first. Both defects were found
only by reading traces; nothing on the tape or in the notice says the newest
candidate was passed over.

## Not covered here

- R5-1 and R5-2 (Texture re-casts on a progress report; management opens a
  second assignment onto a bound request) are what let the second round
  exist. They are separate lanes.
- No owner act today says "start over": a rejection always carries its
  candidate. That is the normal iteration and is unchanged.
