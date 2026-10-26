# U4 stack rebase, build delta (2026-10-26): stopped at step 2, the mint

Status: **build stopped before tests and code.** Reading the code for station X3
showed that step 2 of the U4 design ("Management casts one engineering assignment
of a new kind, `rebase`") has no mint path in the code. This document records
what exists, the structural finding, the options, and the failure modes, so the
choice is made on the record and nothing is improvised. Class when built: red
(cast gate, a new assignment kind, reconcile at boot), review tier 2. This commit
is green (docs only).

Design: `docs/design/computer-update-architecture-rnd-2026-10-24.md`, "U4 design:
rebasing an applied stack onto a new base". Problem: `docs/problems/promoted-stack-stays-base-moved-2026-10-26.md`;
residual SD-01.

## What already exists (read from code at 618727c97)

- Detection: `sourceproj.reconcileStack` answers `stack_base_moved`, keeps the
  projection commit on the old base `B` as HEAD, and `Result.Refusal()` makes the
  cast refuse (`internal/sourceproj/sourceproj.go`). `Contained` (three-way) already
  retires an upstreamed stack.
- The cast gate: `rt.sourceProjectionCastGate` (`source_projection.go`), called once,
  from `planEngineeringCast` (`engineering_assignment_runtime.go`), before
  `PreflightSourceSnapshot(ctx, "")`, which reads the repository HEAD.
- The apply check and its conflict naming: `capsule.CheckPatchAppliesToTree`,
  `CarriedPatchConflictError{Paths, Clean}`, reached through
  `Executor.CheckCarriedPatchApplies(patch, retryBaseRef)`.
- The offer, its staging and its remembered decision: `offerCarriedPatch`,
  `stageCarriedPatch`, `<root>/carried/offers/<assignment>-<attempt>.json`
  (`internal/agentcore/carried_patch.go`). Its history source is
  `storeCarriedHistory.CarriedCandidates(trajectory)`. An applied operation is not
  `roundCarriable` (only rejected or failed), so the stack top is never a candidate
  for it. That is by design ("an ordinary assignment still never carries an applied
  operation").
- Freeze: `composeFreezePatch` makes the bundle patch cumulative from the stack
  base using the capsule's pinned commit (`sourceproj.Cumulative`); `openFreezeCandidate`
  opens the operation keyed by `store.EngineeringFreezeIdempotencyKey(assignment)`.
  `CapsuleEffectBundle` has no lineage field; adding one changes
  `ComputeContentDigest` for bundles that carry it (omitempty keeps old ones).
- Settings: `store.KnownComputerSettings` (`computer_setting.go`) and
  `rt.computerSettingOn`. `source.rebase_offer` is not present anywhere.
- Crash rule: a crash boot closes open assignments "interrupted by a restart"
  (`engineering_assignment_boot_test.go`, `closeSelfDevelopmentOperationsAfterCrash`).
- Host side: `hostbuild.baseRefusal` already refuses a host build whose patch base
  is not the booted image's build commit (`ReasonBaseCommitMismatch`). There is
  **no** base check in the guest materializer (`self_development_materializer.go`);
  the design's "a U4 check, not read to exist today" is confirmed.
- Nothing partly built: no `rebase_offer`, `rebase_of`, `RebaseOf` or rebase
  assignment kind in `internal/`, `cmd/`.

## Structural finding: step 2 cannot be expressed on the records route

The design says management casts a `rebase` assignment, automatically, when the
switch is on and reconcile says `stack_base_moved`. In the code, the only way an
engineering assignment is ever created is a management turn's Cast on the records
route, and every layer of that path requires a Texture request:

1. `store.OpenEngineeringAssignment` refuses anything that is not a records
   binding: "only a records cast opens an assignment" (`engineering_assignments.go`).
2. `EngineeringAssignmentBinding.Validate` for a records binding requires
   `Kind == implementation` and `Attempt == 1`, a `ParentConversationID` that is
   management's conversation with a `texture:` inbox, and a positive `ParentRound`
   (`internal/types/engineering_assignment.go`). Kind `rebase` fails the
   `implementation or verification` check and the records check.
3. `requireEngineeringRecordsAdmission` requires the Cast's `ParentID` to be a
   Texture's Cast into management, its round still open, and the Texture document's
   `TrajectoryID` to equal the binding's trajectory.
4. `planEngineeringCast` reads that parent record and document, and the objective
   is read back from the Cast record (`assignmentObjective`). The final Report goes
   back through the parent round; progress relays go to the Texture inbox
   (`engineeringProgressRelays`, `ParentConversationID`).
5. `refuseCastOntoOpenAssignment`, the capsule tool gate
   (`requireCapsuleMutationRole`, kind must be implementation or verification),
   the tools overlay, run context, run acceptance and the store's report rules all
   switch on `assignment_kind` (about a dozen sites).

A boot-time or reconcile-time rebase has no Texture request, no open round, no
management cell and no Cast record, so none of the above can be satisfied. Also:

- The applied operation's own Texture document and trajectory are no help: that
  round is long closed, and a settled document refuses a new revise (TX-01,
  `docs/problems/settled-document-refuses-owner-revision-2026-10-26.md`).
- The capsule must start from `B'`, but the cast's preflight reads the repository
  HEAD, which at `stack_base_moved` is the projection commit on `B`. A rebase round
  needs `PreflightSourceSnapshot` on an explicit `capsule-source-git:<B'>` ref
  (the resume path of that function accepts one; the cast path does not use it).
- The carry's history is keyed by trajectory. The stack top lives on another
  trajectory, so the "one new source" for the offer is a new branch in
  `offerCarriedPatch`, not a reuse of the selection.

So the design's "reuse the existing cast gate and assignment kinds" understates the
work: the new thing is a **non-Texture mint** of an engineering assignment, which
touches the store's open command (tape writes, a new binding variant, a trajectory
the platform would have to create), the Report path, and about a dozen kind
switches. That is the red, tier 2 core of the station, not a small addition.

## Options (for FP0 or the owner; none is chosen here)

- **A. Platform-minted assignment (the design's intent).** A new
  `CastAuthority` (for example `platform_rebase`), a new `OpenEngineeringAssignment`
  variant that takes the objective from the binding, a platform-created trajectory,
  a Report path to management without a Texture round, and `Kind` or a lineage field
  threaded through the kind switches. Honest size: a store change plus a Validate
  change plus the report path; new concurrency on the assignment open. Tier 2.
- **B. Texture-triggered rebase (smaller, not automatic).** The owner, or a platform
  notice to the Texture of the stack operation, asks for the rebase as an ordinary
  Cast with `spec {"rebase_of": "<operation id>"}` on the existing records route
  (Kind stays `implementation`; the lineage is a binding or spec field). The gate
  exception admits a Cast whose `rebase_of` names the stack top and whose preflight
  is pinned to `B'`. Needs a live Texture round, so it is blocked whenever TX-01
  bites; "automatic" is lost.
- **C. Host-side rebase first (the design's later half).** The host replays the
  stack and returns a rebased bundle; no guest assignment at all. Out of scope for
  X3 but it removes the mint problem; the guest then only adopts an offered bundle.

Recommendation: A is the design's mechanism and is buildable, but it is a larger red
slice than the design says and deserves its own definition (store slice first, with
tests). B is not what the design describes and is not built here.

## Failure modes (written before any code)

The design's six, with the test that would pin each:

1. *Double apply* (patch partly in `B'`): hunks already present fail
   `CheckPatchAppliesToTree`, are named as conflicts, never applied twice. Test: a
   fixture base with one of two hunks already present; the check names that hunk's
   path.
2. *Base moves again*: the candidate is pinned to `B'`; add a guest check that the
   materializer refuses a candidate whose `SourcePatchBaseCommit` is not the booted
   base (none exists today). Test: apply of a bundle based on a non-booted commit is
   refused with a named reason; the next reconcile starts a new round.
3. *Loop*: one automatic round per (stack operation, booted base) pair, decided once
   and remembered under `<root>/rebase/`, like the carried offers; a failed or
   unfinished round does not re-cast at the next boot. Test: two reconciles and a
   simulated reboot cast once.
4. *Crash*: the crash boot closes the round "interrupted by a restart" and that
   counts as the pair's round; no boot-time resumer is added. Test: crash boot
   leaves the pair marked and cast count stays one.
5. *Rollback semantics*: rolling back the rebased operation restores the old stack on
   `B` and the computer returns to `stack_base_moved`, named in the receipt. Test:
   apply rebased, roll back, reconcile answers `stack_base_moved`.
6. *Gate exception widens*: admit a rebase round only if its pinned base equals the
   booted base and `rebase_of` names the current stack top; anything else keeps the
   refusal. Test: wrong base, wrong top, switch off, projection off: all refused.

Found while reading (additional):

7. *Switch off or `source.projection` off*: `source.rebase_offer` requires
   `source.projection`; with either off nothing is cast and the refusal text is
   today's. Test: both combinations.
8. *Held decision*: an open self-development operation on the computer already
   blocks a Freeze; a rebase round waits behind it (admission's
   `openOperationHold`). Test: rebase request queued while an operation is open does
   not freeze.
9. *Wrong source for the capsule*: the capsule must start from `B'`, not HEAD (the
   projection commit on `B`). If it started from HEAD the round would "rebase" onto
   the old base. Test: the rebase capsule's pinned commit equals the booted base.
10. *`B'` objects missing* (`base_objects_missing`, residual R3): no check and no
    round; the state stays as it is. Test: reconcile with missing objects casts
    nothing.
11. *Freeze patch cumulation*: Freeze must produce the stack's patch on `B'`, not a
    cumulative patch through the old projection commit. Test: rebased bundle's
    `SourcePatchBaseCommit` is `B'` and its patch applies to `B'` to give the same
    tree as `P` on `B` plus the named resolutions.
12. *Lineage in the bundle*: a new bundle field changes the content digest of bundles
    that carry it; verifiers that re-validate old bundles must be unchanged
    (omitempty). Test: an old bundle's digest is stable.
13. *Owner sees it*: the round must be visible (a Texture item or supervision
    document), or the owner will not know why the computer is casting by itself.
    Needs the same Texture binding as option A's trajectory; unresolved here.

## Files that would change (option A, for the next definition)

`internal/types/engineering_assignment.go` (binding variant and Validate),
`internal/store/engineering_assignments.go` (open variant),
`internal/store/computer_setting.go` (`source.rebase_offer`),
`internal/sourceproj/sourceproj.go` (a rebase-eligible state or flag on the result),
`internal/agentcore/source_projection.go` (gate exception, rebase pair record),
`internal/agentcore/engineering_assignment_runtime.go` (mint and `B'` preflight),
`internal/agentcore/carried_patch.go` (the stack-top source and notice),
`internal/agentcore/tools_capsule.go`, `engineering_assignment_tools_overlay.go`,
`tool_profiles.go`, `run_context.go`, `run_acceptance.go` (kind switches),
`internal/agentcore/self_development_materializer.go` (booted-base check),
`internal/capsule/transaction/builder.go` (lineage field, if kept in the bundle).

## Slices (proposed, after the mint is decided)

1. Store and types: the platform-minted binding and its open, with tests first.
2. Gate exception, switch, pair record, `B'` preflight (guest only, tests first).
3. Offer from the stack top, notice, conflict naming.
4. Materializer base check.
5. Staging proof on the test computer d@e per the design's "Proof on staging".
