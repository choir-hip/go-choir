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

## 2026-06-13 - Forced Active Refresh Deploy Failed Sandbox Activation

Post-fix code commit:
`25c498365221485cfe19bcb5d2a1992bb8bd6986` (`runtime: accept prompt vtext smoke evidence`).
Local proof before push:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestRunAcceptanceSynthesizeAcceptsPromptVTextStagingSmoke|TestRunAcceptanceSynthesizeRecordsWorkerDelegateBlocker|TestRunAcceptanceSynthesizeRecordsPendingWorkerDelegateInvocation|TestRunAcceptanceSynthesizeAcceptsRuntimeSupervisionWithoutAppPackage|TestRunAcceptanceSynthesizeDerivesExportLevelRecord' -count=1`
- `nix develop -c scripts/go-test-runtime-shards`
- `nix develop -c go test ./internal/types ./internal/store`
- `git diff --check`

Push CI/deploy run `27460486381` succeeded for SHA
`25c498365221485cfe19bcb5d2a1992bb8bd6986`, and staging `/health` reported
proxy plus upstream sandbox deployed at that SHA. A fresh public product-path
acceptance proof used real browser/WebAuthn staging user
`qa-1781336201345-chxecv@example.com`, created prompt-bar submission
`dfc758a2-dceb-4cc3-b767-3bcb0c72f8c9`, and observed completed VText doc
`b20de060-ba35-41ff-9c3c-6147b8564f58` with initial loop
`2ece0826-129a-4ab0-bb66-6cea4f9ee8cd`. However
`POST /api/run-acceptances/synthesize` returned
`runacc-2a9f454e6978d2df3a5d` as `staging-smoke-level` but still `blocked`,
with the old `product_path_observed` and `worker_mutation_bounded` invariant
semantics. Inference: the host deploy reached proxy/sandbox identity, but at
least one active interactive computer serving the authenticated product path
had not refreshed onto the new runtime semantics.

To test that inference without changing code, workflow dispatch run
`27460579519` was started at the same SHA with `force_staging_deploy=true`.
All code gates passed, and the deploy-impact job set:

- `DEPLOY_ACTIVE_VM_REFRESH=true`
- `DEPLOY_HOST_OS=true`
- `deploy_host=true`

The Node B deploy then failed in job `81173547329` with exit code 4 during
NixOS activation. The deploy log shows the switch stopped and restarted
`go-choir-gateway.service`, `go-choir-proxy.service`,
`go-choir-sandbox.service`, `go-choir-sourcecycled.service`, and
`go-choir-vmctl.service`; `go-choir-sandbox.service` exited from
`go-choir-sandbox-exec` with status 1 on both the initial switch and the retry.
After that failed deploy, `https://choir.news/health` reported:

- `status=degraded`
- `service=proxy`
- `upstream=unreachable`
- `vmctl_status=ok`
- proxy build/deployed commit
  `25c498365221485cfe19bcb5d2a1992bb8bd6986`
- lifecycle `api.upstream` errors as HTTP 502s

Observed problem: the stronger active-refresh path needed to prove the
acceptance fix cannot currently complete, because forced deploy can leave the
sandbox runtime service down even though the proxy reports the target commit.
This blocks deployed acceptance proof and staging recovery. It does not justify
weakening RunAcceptance gates, claiming continuation-level, or claiming M3
settlement.

Next proof move: recover staging to healthy proxy+sandbox identity at
`25c498365221485cfe19bcb5d2a1992bb8bd6986`, then rerun the public
prompt-bar/VText/RunAcceptance synthesis proof. If recovery requires a code
change, this section is the Problem Documentation First checkpoint for that
fix.

## 2026-06-13 - Sandbox Startup Failure Root Cause

Diagnostic deploy change:
`68fd27e4dde77470a39c4b3071d937c9e63590ca`
(`ci: capture nixos switch deploy diagnostics`) moved service diagnostics ahead
of the NixOS switch retry and prints systemd status, recent journals, and local
health probes when `switch-to-configuration` fails.

Local proof before push:

- `.github/scripts/deploy-impact-classify-test`
- `git diff --check`

Forced workflow dispatch run `27461068327` at SHA
`68fd27e4dde77470a39c4b3071d937c9e63590ca` passed TLA+, Go vet/build,
non-runtime tests, all four runtime shards, integration smoke, frontend build,
and the aggregate Go gate. Deploy job `81174942714` then failed with exit code
4 during Node B NixOS activation. The new diagnostics captured the concrete
sandbox startup root cause:

```text
sandbox: open runtime store: runtime store: bootstrap: apply schema:
Error 1072: key column 'delivered_at' doesn't exist in table
```

The same diagnostic block showed local health at the failed deploy point:
auth, vmctl, gateway, platformd, and maild were healthy; proxy was degraded
because upstream sandbox was unreachable. Public `https://choir.news/health`
after the failed deploy reported `status=degraded`, `upstream=unreachable`,
`vmctl_status=ok`, and proxy build/deployed commit
`68fd27e4dde77470a39c4b3071d937c9e63590ca`.

Observed problem: deployed staging has an existing runtime `worker_updates`
table without the newer `delivered_at` delivery column, but runtime bootstrap
tries to create `idx_worker_updates_pending_target` on
`worker_updates(owner_id, target_agent_id, delivered_at, created_at)` inside
the main schema DDL before the compatibility `ensureColumn` migration can add
`delivered_at`.

Claim/scope: repair runtime store bootstrap ordering so legacy Dolt stores can
add worker-update delivery columns before indexes that depend on them. Do not
change worker-update delivery semantics, RunAcceptance gates, or acceptance
level rules. This is a staging recovery prerequisite, not M3 settlement.

Expected Delta V: 0 for lifecycle semantics, but it should unblock the staging
recovery path. The mission variant remains open until a forced deploy reaches
healthy proxy+sandbox identity and the public product-path RunAcceptance smoke
returns an accepted `staging-smoke-level` record.

## 2026-06-13 - First Schema Bootstrap Fix Did Not Recover Staging

Fix attempt:
`a08076eda2ac6ca9ebcacb27e466d0399e6a1db2`
(`store: migrate worker update delivery columns before indexes`) moved the
known worker-update and inbox-delivery delivery-column indexes out of the main
schema DDL, added compatibility column migrations before those index creates,
and added a Dolt regression for reopening a legacy `worker_updates` table
without delivery columns.

Local proof before push:

- `nix develop -c go test ./internal/store -run 'TestOpenMigratesWorkerUpdatesBeforeDeliveryIndex|TestOpenCreatesDatabase|TestOpenImportsLegacySQLiteRuntimeState|TestUpdateRunAndMarkWorkerUpdatesDelivered' -count=1`
- `nix develop -c go test ./internal/types ./internal/store`
- `nix develop -c scripts/go-test-runtime-shards`
- `git diff --check`

Push CI run `27461261681` for SHA
`a08076eda2ac6ca9ebcacb27e466d0399e6a1db2` passed the code gates and entered
deploy job `81175517320`. During that deploy, public
`https://choir.news/health` showed the proxy already deployed at
`a08076eda2ac6ca9ebcacb27e466d0399e6a1db2`, but the route stayed degraded:
`upstream=unreachable` and `vmctl_status=unavailable`. Read-only Node B probes
at `2026-06-13T08:19:46Z` showed:

- `go-choir-vmctl.service`: `active/running`, but local port `8083` health
  timed out after 5 seconds.
- `go-choir-sandbox.service`: `activating/auto-restart`,
  `ExecMainStatus=1`.
- auth, gateway, platformd, and maild health endpoints were healthy; platformd
  reported build/deployed commit
  `a08076eda2ac6ca9ebcacb27e466d0399e6a1db2`.
- the sandbox journal, both for the host service and freshly refreshed guest
  runtime, still repeated:

```text
sandbox: open runtime store: runtime store: bootstrap: apply schema:
Error 1072: key column 'delivered_at' doesn't exist in table
```

Observed problem: the first schema-ordering fix was incomplete. Staging still
has a runtime store bootstrap path that evaluates a `delivered_at`-dependent
index before the corresponding compatibility column exists, or the deployed
sandbox runtime package is still constructing an older schema order than the
fixed host package. The earlier worker-update-only hypothesis is therefore not
settled by the code change.

Next proof move: inspect the complete runtime store schema bootstrap order and
the deployed package path before changing code again. The next code fix must
carry a regression that reproduces the actual missing-column/index order, not
only the narrower `worker_updates` table case. No staging acceptance,
continuation-level, promotion-level, or M3 settlement is claimed while
`go-choir-sandbox.service` is crash-looping.

## 2026-06-13 - Service Pointer Deploy Does Not Drive Systemd ExecStart

Follow-up read-only Node B inspection narrowed the failed `a08076ed` deploy.
The active systemd unit for `go-choir-sandbox.service` still points at a NixOS
closure generated from commit `68fd27e4dde77470a39c4b3071d937c9e63590ca`:

```text
Environment=RUNTIME_WORKER_REPO_BASE_SHA=68fd27e4dde77470a39c4b3071d937c9e63590ca
ExecStart=/nix/store/ra3hn32zm5hgm72114hqjc2xiqvnlyi7-go-choir-sandbox-exec
```

That wrapper execs:

```text
/nix/store/ikmd0b6bzrdw2wn4vbvmvbqz4fxhim70-sandbox-0.1.0/bin/sandbox
```

At the same time, the service-pointer directory had been updated during the
`a08076ed` deploy:

```text
/var/lib/go-choir/services/sandbox/bin/sandbox
mtime: 2026-06-13 08:11 UTC
```

Observed problem: for `internal/store` pushes the deploy-impact classifier
selects host service pointer deployment for `gateway,sandbox`, and the deploy
script builds/copies the fast-built packages into
`/var/lib/go-choir/services/<service>`. However the currently deployed NixOS
units still execute their baked Nix package wrappers directly. Restarting
`go-choir-sandbox.service` after a pointer deploy therefore restarts the old
Nix package, not the newly copied pointer package. The public health build
identity can move through `/var/lib/go-choir/deploy.env`, masking that the
sandbox process is still old.

Claim/scope: fix the host service execution contract so pointer deployments
are the executable path systemd uses, while retaining the baked Nix package as
a fallback for fresh hosts or missing pointers. This is deploy plumbing and
staging recovery; it does not alter runtime lifecycle semantics or acceptance
levels.

Expected Delta V: 0 for lifecycle semantics. It should allow the already-built
runtime store bootstrap fix to actually start on Node B and unblock the public
product-path acceptance rerun.

## 2026-06-13 - Service Pointer Deploy Repair Recovered Staging Smoke

Fix commit:
`05f9a1507f5060ec92e2ff173c006d4be8fbbf88`
(`deploy: run host services through pointer packages`) changed host service
wrappers so systemd executes `/var/lib/go-choir/services/<service>/bin/<service>`
when the pointer package exists, while retaining the baked Nix package as a
fallback. Local proof before push:

- `.github/scripts/deploy-impact-classify-test`
- `nix eval .#nixosConfigurations.go-choir-b.config.systemd.services.go-choir-sandbox.serviceConfig.ExecStart --raw`
- `git diff --check`

Push CI run `27461596479` for SHA
`05f9a1507f5060ec92e2ff173c006d4be8fbbf88` succeeded. Deploy job
`81176429793` completed successfully at `2026-06-13T08:30:55Z`. Public
`https://choir.news/health` then reported `status=ok`, `upstream=ok`,
`vmctl_status=ok`, and proxy plus sandbox build/deployed commit
`05f9a1507f5060ec92e2ff173c006d4be8fbbf88`.

Deployed product-path smoke used browser WebAuthn registration and only
browser-public product APIs. It submitted prompt-bar request
`8502e863-ab64-41c7-836d-4c737a87e7cf`, opened VText document
`958f8575-60b8-48d5-ac03-a67ebf69e28b`, and synthesized
RunAcceptanceRecord `runacc-e2a8723d1f297b9d8389`. The record state was
`accepted`, level `staging-smoke-level`, deployment/health commit
`05f9a1507f5060ec92e2ff173c006d4be8fbbf88`, checkpoints `submitted` and
`vtext_opened` passed, invariants `product_path_observed`,
`worker_mutation_bounded`, `promotion_not_overclaimed`, and
`checkpoint_causal_order` passed, with no residual risks and no observed
forbidden browser-public routes.

Claim/scope: staging recovery and the prompt/VText RunAcceptance semantics are
proved at deployed smoke level for the repaired SHA. This does not prove the
full M3 restart falsifier, continuation-level, promotion-level, or final M3
settlement.

Expected Delta V: 0 for lifecycle semantics, actual Delta V: 0 for lifecycle
semantics. The staging recovery blocker is closed; the remaining discriminator
is deployed restart/rewarm evidence for durable actor lifecycle behavior.

## 2026-06-13 - Batch I Actor Memory Snapshot Bridge

Claim/scope: narrow the R2 run-memory blocker without inventing a second
lifecycle model. Before this batch, `run_memory_entries` were only keyed and
rehydrated by `loop_id`, so a cold replacement activation for the same durable
agent could start from the wake prompt without the previous activation's
compacted memory unless continuation machinery copied it. The target model says
passivated actors rewarm from compacted memory plus durable log tail.

Move: add a store query for the latest prior inactive activation memory log for
the same `(owner_id, agent_id)`, and have `runMemoryManager.initialize` append a
deterministic `actor_rewarm` compaction checkpoint into a fresh activation's
memory log before appending the wake message. The bridge uses existing
`run_memory_entries` and `runs` tables; it selects only prior
`completed`/`passivated` activations and excludes the current run. `blocked`
runs remain active/unresolved and are deliberately excluded.

Receipts:

- `nix develop -c go test ./internal/runtime -run TestRunMemoryInitializeSeedsPriorActorSnapshot -count=1`
- `nix develop -c go test ./internal/store -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestRunMemoryInitializeSeedsPriorActorSnapshot|TestStartSweepsAssignedOpenWorkItemsAfterPassivation|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestCoagentRewarmUsesResidentActivationNotActiveRunProxy|TestCoagentRewarmIgnoresBlockedHistoricalActivation' -count=1`
- `nix develop -c scripts/go-test-runtime-shards`

Expected Delta V: 0 for the whole mission variant because the deployed
kill/restart falsifier and old lifecycle deletion gates remain open. Actual
Delta V: 0 at mission level, but the run-memory subclaim narrowed: first
provider-call context for a replacement tool-loop activation now carries the
prior actor checkpoint and wake input under the new activation's evidence row.

Open edge: this is a v0 bridge, not the final schema. The durable target is
still an actor-scoped memory snapshot plus log tail; `loop_id` remains an
activation/evidence key until that explicit schema cut lands.

Landing receipts:

- Behavior commit:
  `a7b43100bf789480ee8da1a2ec4c78f0b0217e2b`
  (`runtime: seed rewarm activations with actor memory`).
- Push CI run `27462249760` succeeded. The run included Go non-runtime tests,
  all four `internal/runtime` shards, Go vet/build, integration-tagged smoke,
  TLA+ model check, deploy-impact classification, and staging deploy.
- FlakeHub publish run `27462249771` succeeded.
- Staging deploy job `81178185271` succeeded. Public
  `https://choir.news/health` reported `status=ok`, `upstream=ok`,
  `vmctl_status=ok`, and proxy plus sandbox commit/deployed commit
  `a7b43100bf789480ee8da1a2ec4c78f0b0217e2b`.
- Deployed lifecycle smoke passed:
  `GO_CHOIR_RUN_DEPLOYED_LIFECYCLE=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news pnpm --dir frontend exec playwright test tests/adaptive-lifecycle-control-deployed.spec.js --project=chromium --reporter=list`.
- Browser-public acceptance synthesis used WebAuthn registration and product
  APIs only. It submitted prompt-bar trajectory/run
  `d224018b-a651-40b1-8e1e-dd9287d94c28`, opened VText document
  `e93fead9-2f1b-49ab-8b0f-b87e6f0c2f52`, and synthesized
  RunAcceptanceRecord `runacc-cd78deed35b77e23cddd`. The record state was
  `accepted`, level `staging-smoke-level`, deployment/health commit
  `a7b43100bf789480ee8da1a2ec4c78f0b0217e2b`, checkpoints `submitted` and
  `vtext_opened` passed, invariants `product_path_observed`,
  `worker_mutation_bounded`, `promotion_not_overclaimed`, and
  `checkpoint_causal_order` passed, with residual risk:
  `continuation-level acceptance is not proven until run-memory compaction and
  continuation evidence are recorded`.

Landing Delta V: expected 0 and actual 0 for full M3. The actor-memory bridge
is deployed and smoke-accepted, but the remaining discriminator is still the
deployed kill/restart actor-memory rewarm falsifier plus deletion of permanent
dual lifecycle models. No continuation-level, promotion-level, or final M3
settlement is claimed.

## 2026-06-13 - Batch J Problem Checkpoint: vSuper Cancel Active-Run Fallback

Claim/scope: document a newly confirmed authority-boundary problem before the
fix. In `cancel_agent`, vSuper callers first consult the caller trajectory's
co-super slot registry, but when no slot is found the implementation falls
through to `GetLatestActiveRunByAgent`. That fallback can select a same-owner
agent activation in a different trajectory and let the caller vSuper cancel it.

Evidence: code inspection of `internal/runtime/tools_coagent.go` showed the
slot lookup only guarded the found-slot branch; the generic active-run fallback
remained reachable for vSuper. The existing comprehensive regression
`TestVSuperCancelAgentDoesNotCancelExportedChild` also encoded the wrong
expectation for a different-trajectory child: it expected cancellation instead
of a caller-trajectory guard.

Expected Delta V: 0 for full M3; actual Delta V: 0. The fix target is narrow:
make co-super slot ownership in the caller trajectory the only vSuper
`cancel_agent` authority, while leaving non-vSuper compatibility cancellation
fallbacks out of scope for this batch.

## 2026-06-13 - Batch J Fix: vSuper Cancel Uses Caller-Trajectory Slots Only

Move: change `cancel_agent` so vSuper callers treat a missing co-super slot in
the caller trajectory as `agent not active in caller trajectory` instead of
falling through to `GetLatestActiveRunByAgent`. Slot-owned cancellation and
exported-package protection still use the slot run; non-vSuper cancellation
keeps the compatibility active-run fallback.

Receipts:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run TestVSuperCancelAgentDoesNotCancelExportedChild -count=1`
- `nix develop -c go test ./internal/runtime -run 'TestToolRegistry|TestExecuteToolsVSuperSkipsDuplicateCoordinationSideEffects|TestCoagent' -count=1`
- `nix develop -c scripts/go-test-runtime-shards`
- `git diff --check`

Expected Delta V: 0 for full M3; actual Delta V: 0. The active-run control
fallback set is smaller, but the deployed restart falsifier, non-vSuper
compatibility cancellation fallback, and permanent lifecycle-model deletion
gates remain open.

Landing receipts:

- Problem checkpoint commit:
  `d91d2a72f08bd5840c03687e90d15eb0bab79254c`
  (`docs: record vsuper cancel authority gap`) documented the authority bug
  before the code fix.
- Behavior commit:
  `dd165ada20609f3dca0e2bd968f46e7796a83e5f`
  (`runtime: bind vsuper cancel to trajectory slots`).
- Push CI run `27462568946` succeeded. The run included Go non-runtime tests,
  all four `internal/runtime` shards, Go vet/build, integration-tagged smoke,
  TLA+ model check, deploy-impact classification, and staging deploy.
- FlakeHub publish run `27462568944` succeeded.
- Staging deploy job `81179081886` succeeded. Public
  `https://choir.news/health` reported `status=ok`, `upstream=ok`,
  `vmctl_status=ok`, and proxy plus sandbox commit/deployed commit
  `dd165ada20609f3dca0e2bd968f46e7796a83e5f`.
- Deployed lifecycle smoke passed:
  `GO_CHOIR_RUN_DEPLOYED_LIFECYCLE=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news pnpm --dir frontend exec playwright test tests/adaptive-lifecycle-control-deployed.spec.js --project=chromium --reporter=list`.
- Browser-public acceptance synthesis used WebAuthn registration and product
  APIs only. It submitted prompt-bar trajectory/run
  `cd07ccc4-f35c-4855-9e06-bdb9d2df99cb`, opened VText document
  `a84a3aed-c463-4380-925c-fb46ca800a0a`, and synthesized
  RunAcceptanceRecord `runacc-3326b96bd926f0ac5692`. The record state was
  `accepted`, level `staging-smoke-level`, deployment/health commit
  `dd165ada20609f3dca0e2bd968f46e7796a83e5f`, checkpoints `submitted` and
  `vtext_opened` passed, invariants `product_path_observed`,
  `worker_mutation_bounded`, `promotion_not_overclaimed`, and
  `checkpoint_causal_order` passed, with residual risk:
  `continuation-level acceptance is not proven until run-memory compaction and
  continuation evidence are recorded`.

Landing Delta V: expected 0 and actual 0 for full M3. The cross-trajectory
vSuper cancel authority path is closed and deployed, but this is not the
kill/restart rewarm falsifier and does not settle M3.

## 2026-06-13 - Batch K Problem Checkpoint: Combined Restart Backlog Rewarm

Claim/scope: document a newly confirmed restart-falsifier gap before the fix.
On boot, `Runtime.Start` passivates interrupted activations, then calls
`sweepPendingUpdateActors`, then `sweepOpenWorkItemActors`. The first sweep can
start a cold coagent activation from pending `update_coagent` rows. Because the
resident-agent index is keyed by owner and agent, the subsequent assigned-work
sweep for the same actor returns the resident activation without attaching the
assigned work item IDs or the work-item recovery prompt to that activation.

Evidence: code inspection of `internal/runtime/runtime.go` and
`internal/runtime/super_controller.go` shows the update-created activation
records `request_source=update_coagent` and `worker_update_ids`, while
`reconcileAssignedWorkItemActor` returns the resident run before building the
`trajectory_work_item_sweep` prompt/metadata. `TrajectoryObligations` still
counts the open work item, so the bug is not invisible settlement; it is that
the cold rewarm activation may not receive all durable backlog for its assigned
actor/trajectory.

Expected Delta V: 0 for full M3; actual Delta V: 0. The next bounded fix is a
combined restart regression and implementation path that makes the replacement
activation carry both pending update IDs and assigned work item IDs for the
same actor trajectory, without marking the work item complete or weakening the
open-obligation query.

## 2026-06-13 - Batch K Problem Checkpoint: Multi-Trajectory Backlog Batch

Claim/scope: document the independent-review expansion of the combined restart
backlog bug before the fix is committed. The first candidate fix attached
assigned open work items only for `updates[0].TrajectoryID`, but
`ListPendingWorkerUpdates` batches all undelivered updates for the target actor.
If one durable coagent has pending updates for trajectories A and B, the cold
replacement activation can carry both updates while only receiving assigned
work from trajectory A; the later assigned-work sweep then sees the actor
resident and skips trajectory B's assigned work group.

Evidence: independent review of the uncommitted diff by
`/root/combined_rewarm_review` found the mismatch between
`internal/store/store.go:1925` batching by target agent,
`internal/runtime/super_controller.go` using only the first update trajectory,
and `internal/runtime/runtime.go` boot order/resident skip behavior.

Expected Delta V: 0 for full M3; actual Delta V: 0. The fix target expands from
same-trajectory combined backlog to every distinct non-empty trajectory in the
pending update batch for the same actor, with a two-trajectory regression.

## 2026-06-13 - Batch K Fix: Combined Backlog Rewarm Includes Assigned Work

Move: change cold `update_coagent` backlog rewarm so the replacement coagent
activation includes assigned open work items for every distinct non-empty
trajectory represented in the pending update batch. The activation metadata now
records both `worker_update_ids` and `work_item_ids`, and the wake prompt
contains both pending update content and assigned-work objectives. The work
items remain open until the actor actually closes them; `TrajectoryObligations`
continues to report open obligations.

Receipts:

- Problem checkpoints:
  `96636bee89fe526158e81c29e685c3a2d0900fb1`
  (`docs: record combined restart backlog gap`) and
  `6abca2999b8a76e351abeb607b6267a44b5334c5`
  (`docs: record multi-trajectory rewarm gap`).
- Behavior commit:
  `63767a43673007aaca27e926c74dd6e9ee7093f3`
  (`runtime: include assigned work in coagent rewarm`).
- Local focused proof:
  `nix develop -c go test ./internal/runtime -run TestStartRewarmsCoagentWithPendingUpdatesAndAssignedWork -count=1`.
- Local surrounding proof:
  `nix develop -c go test ./internal/runtime -run 'TestStart(RewarmsCoagentWithPendingUpdatesAndAssignedWork|SweepsAssignedOpenWorkItemsAfterPassivation)|TestUpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|TestCoagentRewarm|TestTrajectoryObligationsReportPendingUpdateCoagent|TestUpdateCoagentDelivery' -count=1`.
- Independent review re-run by `/root/combined_rewarm_review` found no
  findings after the multi-trajectory expansion. Its focused command passed:
  `nix develop -c go test ./internal/runtime -run 'TestStartRewarmsCoagentWithPendingUpdatesAndAssignedWork|TestStartSweepsAssignedOpenWorkItemsAfterPassivation|TestCoagentRewarmUsesResidentActivationNotActiveRunProxy|TestCoagentRewarmIgnoresBlockedHistoricalActivation|TestUpdateCoagentWarmActivationInjectsPendingTurn' -count=1`.
- Local batch proof:
  `nix develop -c scripts/go-test-runtime-shards`;
  `git diff --check`.

Landing receipts:

- Push CI run `27462963675` succeeded. The run included Go non-runtime tests,
  all four `internal/runtime` shards, Go vet/build, integration-tagged smoke,
  TLA+ model check, deploy-impact classification, and staging deploy.
- FlakeHub publish run `27462963683` succeeded.
- Staging deploy job `81180171763` succeeded.
- Public `https://choir.news/health` reported `status=ok`, `upstream=ok`,
  `vmctl_status=ok`, `vmctl_routing=enabled`, and proxy plus sandbox
  build/deployed commit `63767a43673007aaca27e926c74dd6e9ee7093f3`.
- Deployed lifecycle smoke passed:
  `GO_CHOIR_RUN_DEPLOYED_LIFECYCLE=1 CHOIR_DEPLOYED_BASE_URL=https://choir.news pnpm --dir frontend exec playwright test tests/adaptive-lifecycle-control-deployed.spec.js --project=chromium --reporter=list`.
- Browser-public acceptance synthesis used WebAuthn registration and product
  APIs only. It submitted prompt-bar trajectory/run
  `89ca3c23-3477-4e40-8ecb-1a738b3191ac`, opened VText document
  `902ef3c2-e045-4acb-ad85-695ed0393e95`, and synthesized
  RunAcceptanceRecord `runacc-e6f3ae1cde0f9536c812`. The record state was
  `accepted`, level `staging-smoke-level`, deployment/health commit
  `63767a43673007aaca27e926c74dd6e9ee7093f3`, checkpoints `submitted` and
  `vtext_opened` passed.

Landing Delta V: expected 0 and actual 0 for full M3. Batch K closes the
combined update-plus-assigned-work restart backlog gap, including the
multi-trajectory update batch variant, but it is still an in-process restart
regression plus deployed product-path smoke. No continuation-level,
promotion-level, or final M3 settlement is claimed. The next realism axis is a
deployed kill/restart or equivalent actor rewarm falsifier with zero stranded
messages and no zero-obligation stalls. Rollback reference for behavior is the
previous deployed behavior commit
`dd165ada20609f3dca0e2bd968f46e7796a83e5f`.

## 2026-06-13 - Batch L Probe: OS-Kill Restart Rewarm Oracle

Claim/scope: add a stronger local restart oracle before attempting a staging
service-kill proof. Prior Batch G/K restart regressions reopened the same
store and called `Runtime.Start`, but they seeded interrupted rows directly.
The new probe starts a real child test process with a running coagent
activation against a persistent runtime store, kills that process with
`SIGKILL`, then boots a fresh child process over the same store.

Move: add `TestProcessRestartRewarmsCoagentAfterOSKill` in
`internal/runtime/update_coagent_cutover_test.go`. The start child seeds a
live trajectory with one pending `worker_update`, one assigned open work item,
and a running activation for the assigned coagent. The parent waits until the
store reports the activation as `running`, kills the process, then starts a
recovery child. The recovery child calls `Runtime.Start`, proves the killed
activation becomes `passivated`, waits for a replacement run for the same
durable actor to reach `running`, and writes proof that the replacement run
metadata contains both `worker_update_ids` and `work_item_ids`. It also checks
`TrajectoryObligations` still reports one pending update and one open work item,
so the proof does not collapse open obligations into false settlement.

Receipts:

- Focused OS-kill proof:
  `nix develop -c go test ./internal/runtime -run TestProcessRestartRewarmsCoagentAfterOSKill -count=1 -v`.
- Focused surrounding restart/rewarm proof:
  `nix develop -c go test ./internal/runtime -run 'Test(ProcessRestartRewarmsCoagentAfterOSKill|StartRewarmsCoagentWithPendingUpdatesAndAssignedWork|StartSweepsAssignedOpenWorkItemsAfterPassivation|UpdateCoagentPendingUpdateSurvivesRestartAndDeliversOnce|CoagentRewarmUsesResidentActivationNotActiveRunProxy|CoagentRewarmIgnoresBlockedHistoricalActivation)' -count=1`.
- Independent review by `/root/process_restart_test_review` first found that
  the probe could pass on a pending replacement row; the test was tightened to
  require the replacement run to reach `running`. Re-review found no findings.
- Batch-boundary proof:
  `nix develop -c scripts/go-test-runtime-shards`.
- `git diff --check`.

Expected Delta V: 0 for full M3; actual Delta V: 0. Observer evidence
improved: the restart proof now crosses an actual OS process-death and
fresh-process store reopen boundary, including killed-process store state. This
still does not settle the R1/R2 falsifier because it is local test-process
evidence, not deployed Node B service-kill evidence, and it does not prove
multi-agent staging load or week-old VText memory continuation. The next
realism axis remains a deployed kill/restart or equivalent staging proof with
zero stranded messages and no zero-obligation stalls.
