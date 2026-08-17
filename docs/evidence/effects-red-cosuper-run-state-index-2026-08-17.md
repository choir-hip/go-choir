# Effects legacy CoSuper invisible to run-state index — 2026-08-17

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` reports `beb32aeb265b39a9e0e089450b5239219beedd6d`, deployed `2026-08-17T04:42:10Z`.

## Root cause

`rewarmInterruptedLifecycleActivations` discovers interrupted lifecycle runs through `ListLifecycleRunsByState`, which queries the object metadata index `$.state`. That index is only written by the bind path introduced in `beb32aeb`. Assignment `assignment-fa38b037-bd9d-5270-a640-e668afa4eb57` was bound at `2026-08-17T02:05:08Z`, before `beb32aeb`, so its run `run:assignment-fa38b037-bd9d-5270-a640-e668afa4eb57` carries no `$.state` metadata and is never listed. Rewarm therefore never reaches `ReconcileCoSuperAssignmentsForTrajectory` for it.

The assignment projection is durable and correct. Under trajectory `e826402d-b666-503f-93ba-72b3bcf51e8d`:

- `assignment-fa38b037-bd9d-5270-a640-e668afa4eb57`: disposition `bound`, capsule_disposition `active`, capsule `capsule-83337f60-080b-5b45-9df5-bc116d823867`, parent loop `f8ee744f-dc92-4079-9746-47a759d82331`, `computer_id` correct.

`CancelCoSuperAssignment` does terminalize the bound run (`coSuperTerminalRunState`), so the run stayed `running` because reconcile was never invoked, not because cancellation failed to project.

## Fix

`23fbd857` adds `Store.ListCoSuperAssignmentsForComputer` (lists `ogKindCoSuperAssignment` by computer, independent of run-state and work-item projections) and `Runtime.reconcileCoSuperAssignmentCapsulesAfterRestart` (sweeps non-terminal assignments and delegates to the existing per-trajectory reconciler). `Start` invokes it after rewarm.

Tests: `TestListCoSuperAssignmentsForComputerFindsBoundAssignment`, `TestListCoSuperAssignmentsForComputerIndependentOfRunStateIndex`, `TestReconcileCoSuperAssignmentCapsulesAfterRestartTerminalizesAbsentCapsule`. All pass; `./internal/store` and `./internal/agentcore` CoSuper/assignment suites pass.

Not deployed yet. Do not retry the operations POST while `run:assignment-fa38b037` is live. Do not cancel it solely for silence.
