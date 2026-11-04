# A rematerialized stack is never served: a computer with applied changes stops getting platform updates (2026-11-04)

Status: problem record. No fix in this commit.
Class: red (self-development apply authority; the fix touches the rebase path
that opens a platform-accepted operation).
Evidence file: `pool-obs.json` (a `/health` read of the test computer "pool",
2026-11-04 15:48Z; no secrets).

## Observation (computer "pool", 2026-11-04)

| Time (Z) | Event |
|---|---|
| 15:38:03 | boot: `stack_base_moved base=d32c536 booted=755617851` |
| 15:38:22 | base fetch lands: `stack_rematerialized base=755617851 head=af5c67d9` |
| 15:43:03 | `rebase notice: none minted: the stack is stack_rematerialized, not stack_base_moved` |

At 15:38:22 S6 (package lock) kept pool's `library-next` whole:
`package_behind_base`, `conflict: true`. `files-next` and `slides-next` are
`package_base_moved` (byte-identical). The shell was untouched.

`/health` `effective_release` afterwards: `diverged: true`,
`source_projection.state = stack_rematerialized`, `served_base = d32c536`,
`booted_base = 755617851`, and the reason ends "the served build is still the
stack on d32c536". Pool keeps serving the old build.

## Cause

Two facts compose.

1. `reconcileStack` (internal/sourceproj/sourceproj.go) returns
   `stack_rematerialized` whenever the packages-aware graft succeeds: the
   projection (the capsule source) moves to the booted base, but the served
   build is not touched. S6 accepted that: "the served build does not change;
   approving the next candidate cast on this projection approves the
   re-materialization"
   (docs/design/s6-package-lock-build-2026-11-04.md, The Cast gate).
2. Everything that moves a served build off an old base is armed only at
   `stack_base_moved`:
   - `armRebaseNotice` (rebase_notice.go:116) arms the timer only for
     `StateBaseMoved`;
   - `mintRebaseNotice` (rebase_notice.go:222) returns "not stack_base_moved"
     for anything else, so `cleanRebaseBeforeNotice` (clean_rebase.go) never
     runs;
   - the rebase Cast gate (`sourceProjectionCastGateFor`) answers a rebase
     Cast at `stack_rematerialized` with `rebase_not_needed`; the owner retry
     (rebase_retry.go:56) the same.

So a computer whose applied change does not touch what the platform changed
(the common case) has its projection advanced and its served build frozen.
Each deploy widens the gap. The only way out is an unrelated candidate being
approved. Before S6 the same stack was `stack_base_moved` and the clean rebase
or the notice served it; S6 moved the common case out of that path.

## Impact

A computer with applied changes silently stops receiving platform updates
(here, the frontend). The owner's intent (2026-11-04: updates arrive; clean
rebases without approval) is not met for exactly the computers that have
self-developed. A conflict (`library-next`) additionally has no path to the
owner: no notice is minted and a rebase Cast is refused.

## Belief state

- Confirmed by the log above and by the code at the cited lines: no timer, no
  clean-rebase attempt and no notice exist for `stack_rematerialized`.
- Confirmed: `Result.ServedBase` is the base the applied patch is on and
  `Result.Base` is the booted base in that state, so the clean rebase's inputs
  (operation, stack base, patch, booted base) are all available.
- Hypothesis, to be shown by the new end-to-end tests: `sourceproj.Reapply` of
  the applied patch onto the booted base fails for pool's `library-next`
  (a different app at the same path), so pool would get the notice; and
  succeeds for a stack whose graft is byte-identical.
- Not yet known: whether the notice flow (Texture, engineering rebase round)
  has any other dependence on `stack_base_moved` beyond the three gates above.

## Candidate fixes

A. Arm and run the existing rebase path also at `stack_rematerialized` when the
   served base is not the booted base. Try the clean rebase first (Reapply of
   the applied patch, host build, platform-clean-rebase receipt; corpusd gate
   unchanged, test tier only); if the patch does not apply cleanly, or the
   graft carries a package conflict, mint the ordinary notice; the rebase Cast
   gate and the owner retry accept the stack. One attempt per (operation,
   booted base) pair, as today. Chosen: smallest change, reuses every
   guard (restart rules, open-operation refusal, wedged-attempt fall-through).
B. Make the materialization itself serve: promote the rematerialized
   projection by a new platform acceptance kind. Rejected: a new authority
   with no owner act, wider than the 2026-11-04 ruling and needing a new
   corpusd gate.
C. Leave it and wait for an approved candidate (the S6 plan). Rejected by the
   owner: updates must arrive.

## Failure modes of fix A (to be tested before the code)

1. A loop: a computer whose served base equals the booted base is acted on
   again. Guard: the rule requires `ServedBase != "" && ServedBase !=
   BootedBase`; the e2e "nothing" test.
2. A second attempt for the same pair after the first failed or applied.
   Guard: the operation's idempotency key (existing); the "second fire" test.
3. A conflicting package applied silently over the owner's version. Guard: a
   graft with a package conflict goes to the notice, never the clean rebase.
4. A restart mid-apply or with a live capsule. Guard: the existing refusals
   (open operation, planned-restart marker, materializer) are unchanged.
5. A notice that cannot be answered because the Cast gate refuses it.
   Guard: the gate and the owner retry accept `stack_rematerialized` with a
   served base that is not the booted base.
6. An open selfdev operation at fire time (review, Devin F2, 2026-11-04).
   At `stack_rematerialized` ordinary Casts are admitted, so an open
   operation is routine. The first build of fix A let `cleanRebaseBeforeNotice`
   return hold=false while an operation was open, so `mintRebaseNotice` minted
   a notice; a rebase assignment claiming it would have its Freeze refused by
   `selfdev.ErrOperationOpen`, with the pair's claim standing and only the
   owner retry reviving it. Guard: an open operation holds the pair (no
   notice, no attempt, the timer rechecks at the recheck interval); the next
   fire after the operation closes proceeds. Test:
   `TestCleanRebaseEndToEndOpenOperationHoldsTheNoticeThenProceeds`. No new
   resumer: the hold only re-arms the in-memory timer; a crash still closes
   open attempts as "interrupted by a restart".

## Named residuals (review, 2026-11-04; no code in this change)

- F4: a package root the platform already adopted falls to the notice path.
  The stack's package files are left as `B'` has them, so `Reapply`
  (plain `git apply --cached`) fails because the after-image is already
  present. Conservative and consistent with "does not apply -> notice"; it is
  not a bug, and a smarter already-applied detection is later work.
- F5: two candidate paths can both be open at `stack_rematerialized`: an
  ordinary Cast (graft; serves the kept-whole package) and the rebase Cast
  (serves `B'` plus the change). `refuseCastOntoOpenAssignment` is per
  (trajectory, round), so both may be open and approved, the second apply
  rolling back the first; for a package conflict the two produce different
  package content. No corruption, but the product does not say which path
  the owner should use. Open decision, to settle before the rebase Cast is
  offered at a package conflict as the preferred path.

Residual R1 (Devin, round 2): the open-operation hold covers only the first mint. A notice already on the tape can still be claimed while an operation is open, the package-conflict path mints without the hold, and re-mints of attempts 2 and 3 proceed while an operation is open. Each ends with the Freeze refused and the claim standing; the owner retry recovers it. Pre-existing in kind at stack_base_moved.
