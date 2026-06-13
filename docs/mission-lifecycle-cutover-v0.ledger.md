# Mission M3 - Lifecycle Cutover Ledger

## 2026-06-13 - Paradoc Compiled

Claim/scope: compile M3 from the portfolio and durable-actors cutover program
after M2 settlement. No lifecycle code changes in this pass.

Move: created `docs/mission-lifecycle-cutover-v0.md` with Parallax State,
variant V=8, initial conjectures, domain ramp, and first inventory move.

Expected Delta V: 0. This is a handoff/preparation move.

Actual Delta V: 0. M3 is ready for a worker thread to begin with lifecycle
inventory and classification.

Receipts:
- M2 predecessor settled at `794d28dd76ff00a2ae27c98a14dbce9e34834695`.
- Source program: `docs/mission-portfolio-2026-06-11.md` section M3 and
  `docs/choir-rearchitecture-durable-actors-2026-06-11.md` cutover step 4.
- Paradoc path: `docs/mission-lifecycle-cutover-v0.md`.

Open edge: the first worker pass must verify the current code inventory before
choosing a construct batch; line numbers in source docs are intentionally not
trusted.

## 2026-06-13 - Lifecycle Inventory Classified

Claim/scope: the required first M3 move can close the unclassified lifecycle
map before behavior changes. Scope is repository code evidence only; no runtime
behavior change and no staging claim.

Move: probed current code for `executeRun`, `recoverInterruptedRuns`,
`CancelRunGraph`, `ParentRunID`, active-run graph queries, boot recovery,
run-memory compaction, update-woken delivery, and acceptance evidence; rewrote
Parallax State plus added the inventory table in
`docs/mission-lifecycle-cutover-v0.md`.

Expected Delta V: -1 by eliminating the unclassified control-read blocker and
selecting the first construct batch.

Actual Delta V: -1. Current V=7. The next construct batch is activation shim
plus boot sweep; permanent dual lifecycle models remain, so settlement is not
available.

Receipts:
- `internal/runtime/runtime.go`: `Start` calls `recoverInterruptedRuns`;
  `startRunAsync` and `StartChildRun` launch `go rt.executeRun`; `CancelRunGraph`
  recurses over `ListChildRuns`.
- `internal/store/store.go`: `parent_loop_id` persists on runs; active child
  and latest active agent queries still use non-terminal run state; pending
  `worker_updates` are the durable `update_coagent` wake backlog.
- `internal/runtime/super_controller.go`: update-woken super/coagent paths
  create/reuse runs and mark updates delivered through run terminal handling.
- `internal/runtime/run_acceptance.go`: acceptance records are trajectory-keyed
  but still retain `loop_id` and transitional `continued` checkpoint logic.

Open edge: code still has old run lifecycle behavior. The next move must update
the code and tests, starting with the smallest activation shim that preserves
the current tool loop and run-memory compaction behavior while deleting boot
blanket-fail recovery.

## 2026-06-13 - Boot Blanket-Fail Deleted, Pending Update Sweep Added

Claim/scope: the first construct batch can delete restart-as-failure without
rewriting the full LLM loop by representing interrupted in-process work as a
passivated activation and sweeping durable `update_coagent` backlog on boot.
Scope is in-process runtime/store/types behavior; no deployed or kill -9
claim.

Move: added `RunPassivated` and `activation.passivated`; replaced
`recoverInterruptedRuns` with boot passivation plus `sweepPendingUpdateActors`;
added `Store.ListPendingWorkerUpdatesAll`; updated VText restart recovery so
stale mutations no longer block rewarm; removed old restart-failure fixture
text from Go tests.

Expected Delta V: -2 for the planned Batch A.

Actual Delta V: -1. Deleted the boot blanket-fail blocker and proved pending
`update_coagent` backlog can re-warm actors through the existing wake path.
The activation shim is still shallow: delivery is still tied to run terminal
handling for many paths, warm step-boundary steering is not implemented, and
run memory is still keyed by `loop_id`.

Receipts:
- Code: `internal/runtime/runtime.go`, `internal/store/store.go`,
  `internal/types/task.go`, `internal/runtime/api_trace.go`.
- Focused tests:
  `nix develop -c go test ./internal/types ./internal/store ./internal/runtime -run 'TestTaskState|TestRecovery_InterruptedTasksPassivatedOnRestart|TestRecovery_RecoveredTasksEmitPassivatedEvents|TestInterruptedRunningTasksPassivatedOnStart|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestRestartRecoveryClearsInterruptedVTextMutationAndRelaunches'`.
- Batch boundary tests:
  `nix develop -c go test ./internal/types ./internal/store`;
  `nix develop -c scripts/go-test-runtime-shards`.

## 2026-06-13 - Batch C Cancellation Checkpoint

Reliable evidence before code: `Runtime.CancelRunTrajectory` exists and VText
mutation cancel enters it, but `Runtime.CancelRunGraph` still exists as an
exported parent-tree control helper that recursively follows
`store.ListChildRuns(parent_loop_id)`. Comprehensive cancellation coverage also
still names the old graph behavior and manually constructs parent/child rows
instead of proving shared trajectory membership. That leaves the old lifecycle
model available even if the main VText path has started using trajectories.

Claim/scope: Batch C should delete the graph cancellation entry point and make
VText mutation cancellation prove trajectory/work-item authority. Direct
`CancelRun` remains as activation termination evidence; spawned-by provenance
must not decide cancellation reach.

Expected Delta V: -1 if no parent-tree cancellation entry point remains and
the regression checks that cancellation follows `trajectory_id`, not
`parent_loop_id`.

Move: deleted `Runtime.CancelRunGraph`; VText mutation cancel now uses the
existing `CancelRunTrajectory` path only. Renamed the comprehensive VText
regression and changed its fixture so one run shares the mutation trajectory
without being a child, while another run is a `ParentRunID` child on a different
trajectory.

Actual Delta V: -1. Current V=4. The parent-tree cancellation entry point is
gone; cancellation reach for this path is now trajectory membership. Remaining
variant is not zero because parent-child active queries, spawn budget, verifier
sequencing, trace graph compatibility, restart falsifier, and acceptance
repointing still preserve old lifecycle concepts.

Receipts:
- Code: `internal/runtime/runtime.go`,
  `internal/runtime/vtext_agent_revision.go`.
- Regression coverage:
  `TestVTextCancelAgentRevisionCancelsTrajectoryAndLeavesMutationResumable`.
- Focused test:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextCancelAgentRevisionCancelsTrajectoryAndLeavesMutationResumable|TestVSuperCancelAgentDoesNotCancelExportedChild' -count=1`.
- Batch boundary test:
  `nix develop -c scripts/go-test-runtime-shards`.

## 2026-06-13 - Batch C Review Checkpoint

Reliable review evidence before code: trajectory cancellation still has three
sharp edges after deleting `CancelRunGraph`.

- Legacy or manually inserted rows can have empty persisted `runs.trajectory_id`.
  `CancelRunTrajectory` derives a fallback trajectory in memory, but then
  queries `ListActiveRunsByTrajectory`, which reads the stored column. VText can
  mark the mutation cancelled while the target activation remains running.
- The active-run query is capped at 1000 rows once; a larger trajectory can be
  marked cancelled while later active activations remain alive.
- The Batch C VText regression proves run membership but not the documented
  work-item/trajectory-status part of the cancellation authority claim.

Claim/scope: fix Batch C so cancellation persists fallback trajectory identity,
drains active activations in pages, and proves trajectory status plus work-item
cancellation in the VText regression.

Expected Delta V: 0. This is a correction to Batch C's claimed cut, not a new
variant reduction.

Move: `UpdateRun` / `UpdateRunAndMarkWorkerUpdatesDelivered` now persist the
`trajectory_id` column. `CancelRunTrajectory` persists fallback trajectory
identity before listing active activations and drains active runs with an
exclude-based paging query. The VText regression now creates an open work item
and asserts the work item plus trajectory are cancelled.

Actual Delta V: 0. Current V=4. The Batch C deletion claim now has the missing
legacy-row, page-drain, and work-item/status coverage.

Receipts:
- Code: `internal/runtime/runtime.go`, `internal/store/store.go`.
- Regression coverage:
  `TestCancelRunTrajectoryPersistsFallbackTrajectoryID`,
  `TestCancelRunTrajectoryDrainsMoreThanOneActivePage`,
  `TestVTextCancelAgentRevisionCancelsTrajectoryAndLeavesMutationResumable`.
- Focused tests:
  `nix develop -c go test ./internal/runtime -run 'TestCancelRunTrajectoryPersistsFallbackTrajectoryID|TestCancelRunTrajectoryDrainsMoreThanOneActivePage|TestSpawnMintsTrajectoryAndChildJoinsIt|TestTrajectoryObligationsAnswersWaitingOn' -count=1`;
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextCancelAgentRevisionCancelsTrajectoryAndLeavesMutationResumable|TestVSuperCancelAgentDoesNotCancelExportedChild' -count=1`;
  `nix develop -c go test ./internal/store -run 'TestUpdateRunAndMarkWorkerUpdatesDelivered' -count=1`.
- Batch boundary tests:
  `nix develop -c go test ./internal/types ./internal/store`;
  `nix develop -c scripts/go-test-runtime-shards`.
- Follow-up focused tests after stale fixture cleanup:
  `nix develop -c go test ./internal/runtime -run 'TestRunAcceptance|TestDelegateWorkerVMReportsFailedWorkerRunWithoutSynchronousRetry|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestRestartRecoveryClearsInterruptedVTextMutationAndRelaunches|TestRecovery_InterruptedTasksPassivatedOnRestart|TestInterruptedRunningTasksPassivatedOnStart'`.

Open edge: boot sweep currently covers pending `worker_updates`; it does not
yet sweep every live trajectory obligation, prove kill -9 under multi-agent
load, or make warm updates enter an already-running LLM loop at a step boundary.

## 2026-06-13 - Batch A Review Fixes

Claim/scope: independent review found three correctness hazards inside the
Batch A implementation; fixing them is consolidation of the same passivation
claim, not a new lifecycle-blocker decrease.

Move: made `RunPassivated` non-active for co-super slot ownership; changed boot
passivation to drain all pending/running activation pages instead of one
100-row page; removed the legacy synchronous `delegate_worker_vm`
restart-flavored auto-retry so failed worker activations surface as evidence
for the supervisor rather than hidden local retry.

Expected Delta V: 0. These were review-hardening moves against the already
claimed Batch A boundary.

Actual Delta V: 0. Current V=6. The review hazards are closed, but the next
real decrease still requires cutting another blocker: trajectory/work-item
cancellation, parent provenance, active-run residency queries, warm
step-boundary steering, actor memory identity, or deployed restart falsifier.

Receipts:
- Code: `internal/types/task.go`, `internal/store/store.go`,
  `internal/runtime/runtime.go`, `internal/runtime/tools_vmctl.go`.
- Regression coverage:
  `TestTaskStateActive`,
  `TestVSuperCoSuperSlotReusedByTrajectorySlot`,
  `TestInterruptedActivationPassivationDrainsBatches`, and the renamed worker
  delegation no-synchronous-retry test.
- Focused tests:
  `nix develop -c go test ./internal/types ./internal/store -run 'TestTaskState|TestVTextAgentMutationMarkStaleClearsPending' -count=1`;
  `nix develop -c go test ./internal/runtime -run 'TestRunAcceptanceSynthesizeRecordsWorkerDelegateBlocker|TestDelegateWorkerVMReportsFailedWorkerRunWithoutSynchronousRetry|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestVSuperCoSuperSlotReusedByTrajectorySlot' -count=1`;
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestRestartRecoveryClearsInterruptedVTextMutationAndRelaunches|TestRecovery_InterruptedTasksPassivatedOnRestart|TestRecovery_RecoveredTasksEmitPassivatedEvents|TestInterruptedRunningTasksPassivatedOnStart|TestInterruptedActivationPassivationDrainsBatches' -count=1`.
- Batch boundary tests:
  `nix develop -c scripts/go-test-runtime-shards`.
- Hygiene:
  `git diff --check`.

Open edge: passivation still does not perform the protocol-level atomic
idle/backlog check. It is a compatibility release state for interrupted
activations; final settlement still requires real actor residency and restart
falsifier proof.

## 2026-06-13 - Batch B Problem Checkpoint

Claim/scope: code inspection of the generic super/coagent wake path revealed a
new Batch B correctness problem before the fix: update delivery is coupled to
terminal run persistence, not successful update incorporation.

Move: recorded the problem in Parallax State before code changes. Evidence:
`Runtime.updateTerminalRunAndMarkCoagentUpdatesDelivered` marks
`worker_update_ids` delivered whenever the update-woken run is terminal, so
failed or cancelled activations can consume the durable backlog that boot sweep
needs for rewarm.

Expected Delta V: 0; documentation checkpoint.

Actual Delta V: 0. Current V=6. The next construct must make generic
`update_coagent` delivery successful-activation scoped: completed activations
mark incorporated updates delivered; failed/cancelled activations leave the
updates pending.

Open edge: this does not yet solve warm step-boundary steering. It only
prevents the compatibility shim from losing durable wakes on failed terminal
outcomes.

## 2026-06-13 - Batch B Lost-Wake Guard

Claim/scope: generic super/coagent update-woken activations should not consume
durable `update_coagent` backlog unless the activation completes successfully.
Scope is the compatibility activation shim, not full warm step-boundary
steering.

Move: replaced `updateTerminalRunAndMarkCoagentUpdatesDelivered` with
`updateRunAndMarkSuccessfulCoagentActivationDelivered`; only `RunCompleted`
update-woken runs mark `worker_update_ids` delivered. Failed, cancelled, and
blocked activations persist their run state while leaving the worker update
pending for cold rewarm.

Expected Delta V: -1 for Batch B.

Actual Delta V: 0. Current V=6. This closes the lost-wake bug found in the
checkpoint, but it is still a compatibility rule over completed run evidence.
The broader update-woken delivery blocker remains until warm steering and an
explicit activation incorporation boundary exist.

Receipts:
- Code: `internal/runtime/super_controller.go`,
  `internal/runtime/runtime.go`.
- Regression coverage:
  `TestUpdateCoagentDeliveryRequiresSuccessfulActivation`.
- Focused tests:
  `nix develop -c go test ./internal/runtime -run 'TestUpdateCoagentDeliveryRequiresSuccessfulActivation|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestTrajectoryObligationsReportPendingUpdateCoagent|TestVSuperCoSuperSlotReusedByTrajectorySlot' -count=1`;
  `nix develop -c go test ./internal/store -run 'TestUpdateRunAndMarkWorkerUpdatesDelivered' -count=1`.
- Batch boundary tests:
  `nix develop -c go test ./internal/types ./internal/store`;
  `nix develop -c scripts/go-test-runtime-shards`.
- Hygiene:
  `git diff --check`.

Open edge: warm actors still are not steered at a step boundary. Pending updates
behind a blocked/failed activation may retry as a new cold activation; that is
safer than lost wake but can duplicate work until the real actor mailbox/step
boundary lands.

## 2026-06-13 - Batch B Warm Steering Shim

Claim/scope: generic super/coagent activations can receive warm
`update_coagent` records through the existing tool-loop step boundary without a
parallel lifecycle loop. Scope excludes VText's separate document-merge path and
does not claim the final actor mailbox/passivation implementation.

Move: wired `Runtime.executeWithToolLoop` to pass a coagent update injector into
`RunToolLoop`. The injector lists pending updates for the activation's
owner/agent, appends fresh updates as runtime-owned user turns after tool
iterations and at the final checkpoint, records their IDs in run metadata, and
relies on the successful-activation delivery rule to mark them delivered only
when the run completes.

Expected Delta V: -1.

Actual Delta V: -1. Current V=5. The generic warm steering part of the
update-woken delivery blocker is cut: pending updates can enter a running
super/coagent activation before completion, and failed activations keep those
updates pending for rewarm.

Receipts:
- Code: `internal/runtime/super_controller.go`,
  `internal/runtime/runtime.go`.
- Regression coverage:
  `TestUpdateCoagentWarmActivationInjectsPendingTurn`.
- Focused tests:
  `nix develop -c go test ./internal/runtime -run 'TestUpdateCoagentWarmActivationInjectsPendingTurn|TestUpdateCoagentDeliveryRequiresSuccessfulActivation|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce' -count=1`.
- Batch boundary tests:
  `nix develop -c go test ./internal/types ./internal/store`;
  `nix develop -c scripts/go-test-runtime-shards`.
- Hygiene:
  `git diff --check`.

Open edge: this is still a compatibility shim over run metadata and successful
run completion, not the final actor mailbox. It does not sweep all live
trajectory obligations, implement the atomic idle/backlog passivation check, or
remove cancellation/parent graph control reads.

## 2026-06-13 - Batch B Review Correction

Claim/scope: the warm steering shim must not let stray or user-supplied
`worker_update_ids` metadata on an unrelated completed run consume a pending
worker update. Delivery evidence must prove the activation is an eligible
generic actor and that the update was either the cold `update_coagent` wake or
was injected into that actor.

Move: tightened update delivery to eligible generic actor profiles and added
the `worker_updates_injected` run metadata flag. Successful terminal delivery is
now target-scoped through `worker_updates.target_agent_id`, and the store helper
requires the run's agent to match the pending update target before marking it
delivered.

Expected Delta V: 0. This is a correctness correction to Batch B, not a new
variant reduction.

Actual Delta V: 0. Current V=5. The Batch B claim now matches the code: warm
updates are delivered only when they were incorporated by the addressed generic
actor activation and that activation completed successfully.

Receipts:
- Code: `internal/runtime/super_controller.go`,
  `internal/store/store.go`, `internal/runtime/runtime.go`.
- Regression coverage:
  `TestUpdateCoagentDeliveryIgnoresStrayWorkerUpdateMetadata`.
- Focused tests:
  `nix develop -c go test ./internal/runtime -run 'TestUpdateCoagentDeliveryIgnoresStrayWorkerUpdateMetadata|TestUpdateCoagentWarmActivationInjectsPendingTurn|TestUpdateCoagentDeliveryRequiresSuccessfulActivation|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce' -count=1`;
  `nix develop -c go test ./internal/store -run 'TestUpdateRunAndMarkWorkerUpdatesDelivered' -count=1`.
- Batch boundary tests:
  `nix develop -c go test ./internal/types ./internal/store`;
  `nix develop -c scripts/go-test-runtime-shards`.

## 2026-06-13 - Batch D Problem Checkpoint

Reliable code-inspection evidence before code: the highest-impact remaining
`ParentRunID` control reads are in the vSuper co-super admission path.
`Runtime.enforceChildSpawnBudget` counts active rows by direct
`parent_loop_id`, and `Runtime.enforceVSuperVerifierSequencing` falls back to
`ListChildRuns(parent_loop_id)` to decide whether verifier work may start.

That preserves parent-child liveness semantics after Batch C already moved
cancellation to trajectory membership. A co-super activation on the same
trajectory but not a direct child can evade the budget, while a spawned-by child
on another trajectory can satisfy or block verifier sequencing even though it
is only provenance.

Claim/scope: Batch D should make vSuper co-super admission and verifier
sequencing use trajectory-scoped co-super slot ownership, not parent-child
active queries. Historical `parent_loop_id` data may remain for trace/API and
spawned-by lineage, but it must not decide co-super liveness or slot ordering.

Expected Delta V: -1 if vSuper co-super budget/sequencing no longer depends on
`CountActiveChildRuns` or `ListChildRuns`, and regressions prove that
trajectory slot records, not spawned-by ancestry, decide admission.

## 2026-06-13 - Batch D Co-Super Slot Authority

Claim/scope: vSuper co-super admission and verifier sequencing should be
trajectory-slot authority, not parent-child liveness authority. Scope is the
vSuper co-super budget and verifier prerequisite path; trace/API
`parent_loop_id` compatibility and other provenance reads remain.

Move: added store helpers for recorded co-super slot runs and active
co-super-slot counts. Runtime vSuper budget now counts active co-super slots on
the parent trajectory, and verifier sequencing reads the implementation slot
record: active implementation blocks verifier, terminal implementation permits
verifier, and same-parent implementation runs on other trajectories do not
satisfy the prerequisite.

Expected Delta V: -1.

Actual Delta V: -1. Current V=3. One high-impact `ParentRunID` control cluster
is cut: vSuper co-super liveness/order now follows `(owner, trajectory, slot)`
records. Remaining variant still includes active-run residency queries,
child-package evidence reuse, trace/API compatibility, restart falsifier, and
acceptance repointing.

Receipts:
- Code: `internal/runtime/runtime.go`, `internal/store/store.go`.
- Regression coverage:
  `TestCoSuperSlotRunAndActiveSlotCountUseTrajectorySlots`,
  `TestVSuperSpawnAgentEnforcesActiveChildBudget`,
  `TestVSuperVerifierSpawnRequiresCompletedImplementation`,
  `TestVSuperSpawnAgentReusesActiveCoSuperSlot`.
- Focused tests:
  `nix develop -c go test ./internal/store -run 'TestCoSuperSlotRunAndActiveSlotCountUseTrajectorySlots|TestReleaseCoSuperSlotClaimOnlyClearsMatchingRun' -count=1`;
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVSuperSpawnAgentEnforcesActiveChildBudget|TestVSuperVerifierSpawnRequiresCompletedImplementation|TestVSuperSpawnAgentReusesActiveCoSuperSlot' -count=1`.
- Batch boundary tests:
  `nix develop -c go test ./internal/types ./internal/store`;
  `nix develop -c scripts/go-test-runtime-shards`.

Open edge: `CountActiveChildRuns` and `ListChildRuns` still exist for legacy
fixtures/evidence paths. The next cut must either remove or relabel the
remaining callers as provenance, especially package reuse and actor wake/routing
paths that still derive liveness from active runs.

## 2026-06-13 - Batch D Runtime Slot Admission Checkpoint

Reliable code-inspection evidence before code: the `spawn_agent` tool rejects a
vSuper `role="co-super"` call without `slot="implementation"` or
`slot="verifier"`, but the lower-level `Runtime.StartChildRun` path still
accepts a vSuper parent plus co-super child constraints with no co-super slot.
Those unscoped co-super activations do not claim `co_super_slots`, so the new
Batch D slot count would not see them after admission.

Claim/scope: the runtime admission path must enforce the same vSuper co-super
slot requirement as the product tool before checking or claiming slot authority.
This is a Batch D hardening correction, not a new variant decrease.

Expected Delta V: 0. The fix should make unscoped lower-level vSuper co-super
spawns fail before they can bypass trajectory slot accounting, while preserving
valid implementation/verifier slot reuse.

Move: added the lower-level `StartChildRun` slot requirement for vSuper
co-super children before budget and slot-claim handling. Updated the admission
regression so full trajectory slots are checked through the slot-budget guard,
while unscoped co-super children are rejected by the lower-level runtime
boundary.

Actual Delta V: 0. Current V=3. This hardens Batch D without shrinking the
declared lifecycle blocker count: co-super slot authority cannot be bypassed
by callers below the `spawn_agent` tool.

Receipts:
- Code: `internal/runtime/runtime.go`.
- Regression coverage:
  `TestVSuperSpawnAgentEnforcesActiveChildBudget`,
  `TestVSuperSpawnAgentReusesActiveCoSuperSlot`.
- Focused test:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVSuperSpawnAgentEnforcesActiveChildBudget|TestVSuperVerifierSpawnRequiresCompletedImplementation|TestVSuperSpawnAgentReusesActiveCoSuperSlot' -count=1`.

## 2026-06-13 - Batch E Package Reuse Checkpoint

Reliable code-inspection evidence before code: vSuper
`publish_app_change_package` reuse still calls
`Runtime.latestChildAppChangePackage`, which scans
`ListChildRuns(parent_loop_id)`. That keeps package handoff tied to spawned-by
ancestry after Batch D made co-super slot ownership trajectory-scoped.

The failure mode is symmetric: a completed implementation co-super slot on the
same trajectory but not a direct child can be missed, while a direct child on
another trajectory can have its package reused by the wrong vSuper. Package
reuse is evidence incorporation, not liveness, but it still decides which
candidate package a vSuper publishes.

Claim/scope: Batch E should make vSuper package reuse read the trajectory
implementation co-super slot record and that run's package evidence, not
direct children. `parent_loop_id` may remain in the returned compatibility
payload, but it must not select the package.

Expected Delta V: -1 if the runtime has no `ListChildRuns` caller for package
reuse and regressions prove same-trajectory slot evidence is reused while
same-parent other-trajectory evidence is ignored.

## 2026-06-13 - Batch E Trajectory Package Reuse

Claim/scope: vSuper package reuse should incorporate package evidence from the
trajectory implementation co-super slot, not from direct-child ancestry. Scope
is `publish_app_change_package` reuse for vSuper; historical
`parent_loop_id` response fields remain compatibility output.

Move: replaced `latestChildAppChangePackage(parentRunID)` with
`latestTrajectoryCoSuperAppChangePackage(parentRun)`. The helper resolves the
parent trajectory, loads the `implementation` co-super slot record, and reuses
that run's latest successful `publish_app_change_package` output. The
regression now creates a same-trajectory implementation slot that is not a
direct child plus a same-parent implementation run on another trajectory, and
asserts the same-trajectory slot package is reused.

Expected Delta V: -1.

Actual Delta V: -1. Current V=2. Runtime production code no longer calls
`ListChildRuns`; remaining `ListChildRuns` hits are the store helper itself and
email appagent tests. The next remaining control cluster is active-run
residency/authority: `GetLatestActiveRunByAgent` still selects warm actor
reuse and some skip-level/cancel paths still infer authority from active run
parentage.

Receipts:
- Code: `internal/runtime/runtime.go`, `internal/runtime/tools_shipper.go`.
- Regression coverage:
  `TestVSuperPublishAppChangePackageReusesChildPackage`.
- Focused test:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVSuperPublishAppChangePackageReusesChildPackage' -count=1`.
- Batch boundary test:
  `nix develop -c scripts/go-test-runtime-shards`.

Open edge: package evidence reuse is now trajectory-slot based, but
`GetLatestActiveRunByAgent` remains the broad active-run residency proxy and
`parent_loop_id` remains exposed for trace/API compatibility and test
fixtures.

## 2026-06-13 - Batch F Skip-Level Authority Checkpoint

Reliable code-inspection evidence before code: co-super skip-level authority
still depends on the latest active run plus direct `ParentRunID`. The
`update_coagent` super-to-co-super guard in `tools_worker_update.go` looks up
`GetLatestActiveRunByAgent`, follows `ParentRunID`, and blocks only when that
parent run is a vSuper. The vSuper `cancel_agent` exported-package guard in
`tools_coagent.go` also checks whether the target's active run has
`ParentRunID == current vSuper run`.

That preserves run-shaped authority after Batches D/E moved vSuper co-super
budget, sequencing, and package reuse to trajectory slot records. A co-super
owned by a vSuper trajectory slot but whose active activation was not a direct
child can receive a skip-level super directive or be cancelled without the
package-evidence guard. A same-parent co-super activation on another
trajectory can trigger the guard even though the parent edge is only
spawned-by provenance.

Claim/scope: Batch F should make skip-level co-super directive and vSuper
export-cancel guards read the `(owner, trajectory, slot)` registry, not
active-run parentage. Historical `parent_loop_id` remains compatibility and
provenance; it must not decide skip-level authority.

Expected Delta V: -1 if these guards no longer use `ParentRunID` to decide
co-super ownership and regressions prove slot-owned same-trajectory co-supers
are protected while same-parent other-trajectory co-supers are ignored.

## 2026-06-13 - Batch F Slot Authority Guards

Claim/scope: co-super skip-level directive blocking and vSuper
exported-package cancel protection should use trajectory co-super slot records,
not direct spawned-by ancestry. Scope is the super `update_coagent`
super-to-co-super guard and the vSuper `cancel_agent` preservation guard for a
co-super that already produced `publish_app_change_package` evidence.

Move: added same-trajectory co-super slot lookup by agent to the store and
used it from the vSuper cancel guard. The cancel path still uses the latest
active run as a fallback, but a vSuper with a matching trajectory slot targets
that slot run first, so reused agent IDs on other trajectories cannot steal the
cancel target. The exported-package exception applies only when that activation
is the run recorded in the caller vSuper trajectory's co-super slot. The super
update guard now blocks direct co-super directives from slot ownership rather
than from parent-run lookup.

Expected Delta V: -1.

Actual Delta V: -1. Current V=1. `ParentRunID` no longer decides these
skip-level authority guards: the updated regressions cover a slot-owned
same-trajectory co-super that is not a direct child, and a same-parent
other-trajectory co-super whose exported package no longer prevents
cancellation. Independent review found one same-agent cross-trajectory target
selection risk in `cancel_agent`; the final regression covers that case too.

Receipts:
- Code: `internal/store/store.go`, `internal/runtime/tools_worker_update.go`,
  `internal/runtime/tools_coagent.go`.
- Regression coverage:
  `TestCoSuperSlotRunAndActiveSlotCountUseTrajectorySlots`,
  `TestSuperSkipLevelCastRequiresCopiedVSuper`,
  `TestVSuperCancelAgentDoesNotCancelExportedChild`.
- Focused tests:
  `nix develop -c go test ./internal/store -run 'TestCoSuperSlot' -count=1`;
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestSuperSkipLevelCastRequiresCopiedVSuper|TestVSuperCancelAgentDoesNotCancelExportedChild' -count=1`.
- Batch boundary tests:
  `nix develop -c go test ./internal/types ./internal/store`;
  `nix develop -c scripts/go-test-runtime-shards`.

Open edge: `GetLatestActiveRunByAgent` remains an activation lookup/rewarm
proxy in several paths. The next discriminator is the final restart/residency
acceptance cluster, not another parentage guard.

## 2026-06-13 - Batch G Open-Obligation Sweep Checkpoint

Reliable code-inspection evidence before code: `Runtime.Start` passivates
interrupted activations and calls `sweepPendingUpdateActors`, but that sweep
only reads `ListPendingWorkerUpdatesAll`. `TrajectoryObligations` already
defines the silent-stall oracle from open work items plus pending
`update_coagent` backlog, so a live trajectory with an open assigned work item,
no pending update, and no active activation remains observable but not
rewarmed by boot. That is the exact spec v1 gap between `Sweep(a)` over
unprocessed backlog and the current implementation's narrower
pending-update-only sweep.

Claim/scope: Batch G should make boot recovery use open trajectory obligations
as a cold-actor rewarm source, without inventing a second message primitive or
weakening `update_coagent` as the normal wake path. The bounded target is open
work items with an `assigned_agent_id`; unassigned obligations remain
observable in `TrajectoryObligations` but require an owner/supervisor decision
about who should work them.

Expected Delta V: -1 if runtime boot rewarms assigned open work items whose
prior activations were passivated, and a focused restart regression proves a
trajectory with zero pending updates but an assigned open work item does not
stall silently after `Start`.

## 2026-06-13 - Batch G Assigned Open-Obligation Rewarm

Claim/scope: boot recovery should treat assigned open trajectory work items as
durable cold-actor backlog when no pending `update_coagent` row exists. Scope
is deliberately bounded to open work items on live trajectories with a
non-empty `assigned_agent_id`; unassigned work stays visible through
`TrajectoryObligations` and needs an owner/supervisor routing decision.

Move: added a store query for live/open/assigned work items and extended
`Runtime.Start` after update-backlog sweep with an assigned-work sweep grouped
by `(owner, agent, trajectory)`. If the assigned actor has no active
activation, the runtime starts a replacement activation with
`request_source=trajectory_work_item_sweep`, the trajectory id, and the work
item ids in metadata. The generic sweep keeps the same controller boundary as
`update_coagent` rewarm: conductor, email, and vtext appagents are skipped
because they have specialized routes.

Expected Delta V: -1 for the Batch G open-obligation subclaim.

Actual Delta V: -1 for the Batch G subclaim; full mission V remains 1. The
specific silent-stall gap is closed and proved locally: an interrupted
co-super activation is passivated on `Start`, has zero pending worker updates,
and is replaced from the assigned open work item. The mission is not settled:
the runtime still had the compatibility activation body and active-run lookup
was broader than a final actor-residency table; no commit/push/CI/deploy
staging acceptance proof has happened.

Receipts:
- Code: `internal/store/trajectory.go`, `internal/runtime/runtime.go`.
- Regression coverage:
  `TestListOpenAssignedWorkItemsOnlyReturnsLiveAssignedOpenItems`,
  `TestStartSweepsAssignedOpenWorkItemsAfterPassivation`.
- Focused tests:
  `nix develop -c go test ./internal/store -run 'TestListOpenAssignedWorkItemsOnlyReturnsLiveAssignedOpenItems' -count=1`;
  `nix develop -c go test ./internal/runtime -run 'TestStartSweepsAssignedOpenWorkItemsAfterPassivation' -count=1`.
- Batch boundary tests:
  `nix develop -c go test ./internal/types ./internal/store`;
  `nix develop -c scripts/go-test-runtime-shards`;
  `git diff --check`.

Open edge: the sweep currently declines to create a second activation when the
assigned agent already has any active run, even if that active run is on
another trajectory. That matches the current durable-agent single-residency
assumption, but the final actor residency table should make this explicit
rather than depending on `GetLatestActiveRunByAgent`.

## 2026-06-13 - Batch H Residency Proxy Checkpoint

Reliable code-inspection evidence before code: the protocol reference runtime
in `internal/actor` keeps a volatile resident-agent registry and makes
send/passivate decisions under one mutex, but the product runtime still uses
`rt.running` keyed by `loop_id` plus persisted active-run queries as a proxy
for warm actor residency. `reconcilePersistentSuperActor`,
`reconcileUpdatedCoagentActor`, `reconcileAssignedWorkItemActor`,
`reconcileVTextWorkerState`, `Runtime.CancelAgent`, and the generic
`cancel_agent` fallback all ask `GetLatestActiveRunByAgent` whether an actor is
active. That conflates three states that the actor protocol keeps separate:
resident in this process, blocked/non-terminal historical activation, and cold
durable backlog eligible for sweep.

Claim/scope: Batch H should introduce an explicit in-process residency index
keyed by `(owner_id, agent_id)` and use it for warm-actor reuse/cancel checks
where the question is "is this agent resident in this process?" The existing
`executeRun` tool-loop body may remain the compatibility activation body for
this batch, but warm residency must no longer be inferred from arbitrary
non-terminal rows in the store. Blocked historical activations can still be
consulted where a controller deliberately wants blocked-state evidence.

Expected Delta V: -1 if product-runtime warm reuse/cancel paths use the
resident-agent index, if resident entries are registered/cleared with
`rt.running`, and if a regression proves a passivated or blocked historical
run does not suppress rewarm while a truly resident activation still does.

## 2026-06-13 - Batch H Resident-Agent Index

Claim/scope: warm actor reuse should ask the product runtime's volatile
resident-agent index, not infer residency from persisted active rows. Scope is
the in-process runtime activation boundary; the existing tool-loop body remains
the activation body.

Move: added `residentAgents` keyed by `(owner_id, agent_id)`, registered it
with run activation start, and cleared it with cancellation, stop, and normal
activation exit. Super/coagent rewarm, assigned-work rewarm, and VText worker
reconcile now use this resident index for warm reuse. `CancelAgent` and the
`cancel_agent` tool use resident-first lookup while preserving the store-active
fallback for explicit cancellation compatibility. The hot path formerly named
`executeRun` is now `executeActivation` so the runtime code names the body as
an activation rather than the durable lifecycle.

Expected Delta V: -1 for the Batch H residency-proxy subclaim.

Actual Delta V: -1 for the Batch H subclaim; full mission V remains 1 until
landing/staging acceptance and any remaining compatibility fallbacks are either
proved as provenance/compatibility or removed. The broad
`GetLatestActiveRunByAgent` query still exists, but remaining production calls
are now blocked-state evidence, requester provenance, or cancellation
compatibility fallback rather than the first warm-reuse oracle.

Receipts:
- Code: `internal/runtime/runtime.go`,
  `internal/runtime/super_controller.go`,
  `internal/runtime/vtext_controller.go`,
  `internal/runtime/tools_coagent.go`.
- Regression coverage:
  `TestCoagentRewarmUsesResidentActivationNotActiveRunProxy`,
  `TestCoagentRewarmIgnoresBlockedHistoricalActivation`.
- Focused tests:
  `nix develop -c go test ./internal/runtime -run 'TestCoagentRewarmUsesResidentActivationNotActiveRunProxy|TestCoagentRewarmIgnoresBlockedHistoricalActivation|TestStartSweepsAssignedOpenWorkItemsAfterPassivation' -count=1`;
  `nix develop -c go test ./internal/runtime -run 'TestCoagentRewarmUsesResidentActivationNotActiveRunProxy|TestCoagentRewarmIgnoresBlockedHistoricalActivation|TestStartSweepsAssignedOpenWorkItemsAfterPassivation|TestSubmitTaskReturnsStableHandle' -count=1`.
- Batch boundary tests:
  `nix develop -c go test ./internal/types ./internal/store`;
  `nix develop -c scripts/go-test-runtime-shards`;
  `git diff --check`.

Open edge: cancellation still keeps a store-active fallback for legacy/manual
active-row compatibility, and `executeActivation` still marks run terminal
state as activation evidence. The next proof must decide whether those
compatibility fallbacks are acceptable for v0 landing or need another code cut
before staging acceptance.

## 2026-06-13 - Landing Proof Exposes Acceptance-Repointing Blocker

Reliable staging evidence before code: commits `8a826254` and
`a2252af27b5db087cbbb931e8d1b5dc04e402285` landed on `origin/main`. GitHub
Actions run `27460187209` completed successfully, including the Node B staging
deploy. `https://choir.news/health` then reported both proxy and sandbox
`deployed_commit` as `a2252af27b5db087cbbb931e8d1b5dc04e402285`.

Product-path acceptance smoke used a real browser-authenticated staging user
created through public `/auth/*` WebAuthn routes:
`qa-1781335537059-6oqwnm@example.com`. The proof called public authenticated
product APIs only:

- `POST /api/prompt-bar` created submission
  `8a73c212-e1bb-4dcb-9188-43937a77bc09`.
- `GET /api/prompt-bar/submissions/8a73c212-e1bb-4dcb-9188-43937a77bc09`
  returned `state=completed` with VText doc
  `feede330-9de4-4773-9fd9-03e4759154c8` and initial loop
  `4445146e-9153-4e3c-b07a-bea4addcb2e6`.
- `POST /api/run-acceptances/synthesize` produced
  `runacc-ffec1c9975f357724d29` at `staging-smoke-level`, with
  `deployment_commit` and `health_commit` both equal to
  `a2252af27b5db087cbbb931e8d1b5dc04e402285`.

Observed problem: the acceptance record remained `state=blocked`. Its derived
checkpoints were `submitted` and `vtext_opened`, but invariant
`product_path_observed` still required `super_requested`, and invariant
`worker_mutation_bounded` still required worker/export/adoption evidence even
though this deployed smoke did not attempt worker mutation. This is not a
staging deploy failure; it is the M3 acceptance-evidence repointing blocker
from the mission variant made concrete.

Claim/scope: repair RunAcceptance staging-smoke semantics so prompt/VText
product-path evidence can be accepted as `staging-smoke-level` when no worker
mutation path was attempted. Do not weaken blocked worker delegation,
export-level, promotion-level, or continuation-level gates.

Expected Delta V: -1 if the same deployed smoke can synthesize an accepted
`staging-smoke-level` record after CI/deploy, while existing tests still prove
worker delegate blockers remain blocked and runtime-supervision/export paths
keep their stronger gates.
