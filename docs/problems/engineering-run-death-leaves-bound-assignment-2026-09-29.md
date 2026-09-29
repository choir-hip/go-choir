# Bound engineering run dies silently — assignment stays "usable" forever, op pins executing

Date: 2026-09-29
Status: **problem documented; repair in progress.**
Mutation class: **red** (engineering assignment fate, selfdev wedge repair).
Clustering note: fourth defect in the engineering-verification chain this
session (`engineering-verification-chain-dead`, `capsule-subject-artifact-ephemeral`,
now this). The chain's failure surface is now well-mapped; each fix exposes the
next substrate assumption.

## Observed on staging (deployed `90bcf657`, the artifact/reconcile repair)

M11 episode probe `M11_SELFDEV_EPISODE_1790644062296`, computer
`computer-25d14d0efb88319567f6722a84986403`, VM `vm-fc4832748821ceccc30ff8b4e072eba2`,
operation `selfdev-1b9ff0d953b21399f8e06ae671934640`:

- 01:07:55Z `primary_started` — op entered `executing`; implementation cast
  bound and running (`running_runs=1`).
- Tool loop reached **iteration 155** (well past the 80 provider-call cap
  recorded in `textureActorToolLoopBudget`, so tokens/elapsed were the binding
  constraint — 1.2M-token budget at ~3s/iteration).
- **01:19:30Z: the run's log simply stops.** No `run → failed/cancelled`
  line, no completion record, no error emission in the guest boot ring.
  `running_runs` dropped to 0; `selfdev_active_operations` stayed 1.
- Op stayed `executing` for 60+ minutes with zero live runs. No recast, no
  terminal fate, no verification opener. The probe timed out waiting for
  `awaiting_approval`.

## Root cause — two stacked gaps

### Gap 1: `assignedEngineeringCapsuleUsable` checks capsule liveness, not run state

`engineering_assignment_fate.go:76`:

```go
handle, handleErr := exec.AssignmentHandle(assignment.BoundRunID, assignment.Binding.CapsuleID)
diagnostics, inspectErr := exec.InspectCapsuleRaw(assignment.Binding.CapsuleID)
return handleErr == nil && handle != "" && inspectErr == nil &&
    diagnostics != nil && diagnostics.ID == assignment.Binding.CapsuleID &&
    diagnostics.State == capsule.StateActive
```

The capsule stays `StateActive` after its run dies (capsule teardown is part of
cell teardown/actor shutdown, not bound to run fate). A `BoundRunID` whose
run record is terminal (or absent) still reads usable → `continue` → the
assignment is never revoked/cancelled/recast. The op wedges at `executing`
indefinitely.

### Gap 2: run terminalization never wakes the engineering reconcile

`terminalizeRun` (runtime.go:1518) handles `assignment_id` runs via
`cancelBoundEngineeringRun` — but only when the run is *active* when
terminalize is called. A run that ends through `failRun`/budget-exhaustion/
context-death persists its terminal state and emits `run_failed` —
`bindTerminalRunOutcome` binds the outcome — but nothing calls
`ReconcileEngineeringAssignmentsForTrajectory`. The assignment's desk wake
never fires; the trajectory reconcile never re-runs; the bound-but-dead row
sits there.

Even with Gap 1 fixed (usable→false when the bound run is terminal), the
reconcile only runs when a new occurrence lands on the desk work item — a
dead run produces no new occurrence.

## Repair (this commit)

1. `assignedEngineeringCapsuleUsable` gains a bound-run liveness check:
   `GetLifecycleRun(BoundRunID)` must exist and be non-terminal. A dead bound
   run makes the assignment unusable → the existing revoke→cancel→
   restart-recast path closes it (same `restartCancelledAssignmentReason`
   flow that already handles guest restarts).
2. On run terminalization for an assignment-bound run
   (`assignment_id` in metadata, engineering profile), enqueue
   `ReconcileEngineeringAssignmentsForTrajectory` so the fate pass runs on
   the run-death transition, not just at boot or occurrence wake.

Residual: if the run's terminal write itself is lost (hard process kill),
only boot reconcile covers it — same as before.

Refs: `docs/problems/engineering-verification-chain-dead-2026-09-28.md`,
`docs/problems/capsule-subject-artifact-ephemeral-2026-09-28.md`.
Mission: `docs/definitions/choir-selfdev-gate-2026-09-27.md` (M11).
