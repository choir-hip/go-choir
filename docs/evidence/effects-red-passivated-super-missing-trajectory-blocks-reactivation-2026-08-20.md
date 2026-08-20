# Staging Evidence: Passivated Super with Missing Trajectory Blocks Super Reactivation

- Date: 2026-08-20
- Mutation Class: Red
- Computer ID: `computer-03335285269bdba4f94377e56879f9e6`
- Affected Subsystem: `internal/agentcore/super_controller.go` (`reactivateRestartedPersistentSuperControlRun`)

## Summary

Following the refresh to realization epoch 344 on commit `2c6db59e`, an attempt to start a fresh self-development operation via `POST /api/computers/.../self-development/operations` failed with HTTP 409 Conflict:
`"start self-development run: validate restarted persistent-Super control run d077a206-5f88-42a6-be00-a78795e814e3: record not found"`

Investigation revealed:
1. `reactivateRestartedPersistentSuperControlRun` scans all passivated runs in the store (`ListAllRunsByState(types.RunPassivated)`).
2. For each passivated run matching the owner and persistent Super agent, it calls `listPendingLifecyclePacketsDeliveredToRun(ctx, run)` to verify if there are delivered controls pending re-execution.
3. For passivated run `d077a206-5f88-42a6-be00-a78795e814e3` (which was passivated on an older trajectory during computer refresh), `listAllLifecyclePacketsDeliveredToRun` queries `ListLifecycleControlsDeliveredToRunPage`, which returned `record not found` because the referenced trajectory or snapshot record was absent.
4. Line 324 of `super_controller.go` treated any error from `listPendingLifecyclePacketsDeliveredToRun` as fatal (`return nil, false, fmt.Errorf("validate restarted persistent-Super control run %s: %w", run.RunID, readErr)`), aborting the entire scan and preventing `reconcilePersistentSuperActor` from proceeding to create or start any new Super.

## Repair Strategy
In `reactivateRestartedPersistentSuperControlRun`, if `listPendingLifecyclePacketsDeliveredToRun` returns `ErrNotFound` or `record not found`, that candidate passivated run has no available delivered controls and must simply be skipped (`continue`), allowing the loop to inspect remaining candidates or allow `reconcilePersistentSuperActor` to proceed to new pending controls.
