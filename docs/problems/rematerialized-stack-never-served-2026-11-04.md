# A rematerialized stack is never served: a computer with applied changes stops getting platform updates (2026-11-04)

Status: problem record. Fix A landed 2026-11-04 (751471387 and its series); tonight's shot-review failure re-strands the same way (see Investigation 2026-11-09 evening).
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
   (docs/archive/design/s6-package-lock-build-2026-11-04.md, The Cast gate).
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


## Investigation 2026-11-09 evening

Read-only: code at origin/main 0ebab72a7; d@e's `/api/runtime/observability`
(build 03300ad5f, about 01:30Z); the tonight record
[shot-review-repeats-on-every-rebase-2026-11-09.md](shot-review-repeats-on-every-rebase-2026-11-09.md)
and its design
[shot-verdict-reuse-2026-11-09.md](../design/shot-verdict-reuse-2026-11-09.md)
(a builder is implementing it; not redesigned here).

### (e) Fix A landed

The candidate fix chosen above (arm and run the rebase path also at
`stack_rematerialized` when the served base is not the booted base) is on
main: 751471387 (2026-11-04) and its series. `armRebaseNotice`
(`internal/agentcore/rebase_notice.go:117-127`) now handles
`sourceproj.StateRematerialized`, and `clean_rebase.go:163` carries the
open-operation hold. d@e shows it working: after the 00:38:50Z deploy the
boot logged the moved base, a clean rebase started at 00:43:51 and was
accepted at 00:46:23. The original silence ("none minted: the stack is
stack_rematerialized") no longer occurs.

### (a) The stranding is still visible, by another road

d@e's `effective_release` at about 01:30Z: `layered: true`, `base_commit`
d6d91f1a6, `build_commit` 03300ad5f, `diverged: true`. The computer is
behind the fleet exactly as this record described, now because the gate
refused: "operation clean-rebase-e68c31dc ... shot review: both shots show
the identical Loading... state ... the candidate failed to apply" (00:50:14Z),
then "rebase notice minted ... (attempt 1)" at 00:53:54, then
"none minted: an assignment claimed the pair" at 01:08:59. The model-route
round is the only path that serves it, and it costs an engineering round.

### What stranding remains after verdict reuse

The design's ladder is B (equal frontend digest: reuse, liveness check), C
(shoot, compare hashes to the accepted AFTER shots), D (today's full review).
Reuse removes the failure for the common case (a Go-only or docs deploy).
What it leaves, by construction:

1. Rung D on a changed frontend. A deploy that moves the frontend bytes under
   the layer (the launcher change was one) still reaches a judge, and a judge
   can fail it. `recordMaterializationFailed` ("the computer keeps its current
   release", `self_development_materializer.go:727`) is unchanged: the
   computer stays on the old base, and the only recovery is the rebase notice
   to the model route. One clean-rebase attempt per (operation, booted base)
   pair stays as it is, so the stranding lasts until the model round finishes
   or the next deploy gives a new pair.
2. The first rebase after the build has no `attested` prior (the design's
   failure 8): it shoots once, so one more exposure to rung D.
3. A driver change (`driver_id`) forces C or D on the deploy that changes it.
4. A liveness-check failure (backend-only break) goes to D by design; correct,
   but it is a stranding when the failure is the sandbox's, not the build's.
5. The 45-minute shots deadline (`desk_shots.go:60`) still blocks engineering
   admission while an accepted operation waits.

Unchanged, and the root of 1: a failed *review* of a rebase and a failed
*apply* of a candidate are the same gate outcome. A rebase differs: the layer
was already approved, so a failure says something about the sandbox, the
judge or the platform change, not the layer. The design's S0 makes a not-ready
page a failed job with a cause (not a verdict). One further step, not in the
design, would remove most of what is left: for a clean-rebase operation, a
failed *job* (not-ready, sandbox fault) releases the pair for exactly one
re-attempt after the next shoot, while a judge *fail* verdict goes to the
notice as today. That is a gate change (red, tier 2), small (the pair claim in
`rebase_notice.go`/`clean_rebase.go`, the materializer's failure branch), about
3 hours, and it should wait for the S0 to S3 builder to land. Suggested, not
required.

### Belief

Confirmed: fix A is on main and fires; d@e is currently stranded by the
Loading verdict, and the notice route is working it (an assignment claimed the
pair). Hypothesis: the second road (gate failure) will recur on every deploy
that moves frontend bytes until S0 lands. Class of this record's remaining
work: red; owner: the verdict-reuse builder. Cluster: not a member (no
obligation is lost here; a pair claim stands), though the stranded pair and
the stranded Wait share one symptom (an owner-visible computer that stopped
moving, with a notice as the only wake).
