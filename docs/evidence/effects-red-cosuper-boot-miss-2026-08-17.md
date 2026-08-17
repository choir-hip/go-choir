# Effects CoSuper boot missed assignment-fa38b037 — 2026-08-17

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `ed3ffa3f7eb290e13a510af5a1382d86d57329c2`
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **286**
**CoSuper:** `run:assignment-fa38b037-bd9d-5270-a640-e668afa4eb57` still `running` at inspect, `updated_at` frozen at `2026-08-17T02:05:08.829Z`

## Why epoch-286 boot did not terminalize

Assigned CoSuper capabilities are process-local. After guest refresh they are gone. `ReconcileCoSuperAssignmentsForTrajectory` is supposed to revoke the absent capsule and cancel the assignment, projecting the public run terminal.

Two gaps kept that from joining:

1. **Rewarm never listed the run.** `BindCoSuperAssignment` wrote the lifecycle run with `lifecycleMetadata("run_id", ...)` and no `state`. `ListLifecycleRunsByState` indexes `state`. Boot rewarm therefore never saw `run:assignment-fa38b037`, so it could not reconcile it and must not re-dispatch it.

2. **Sweep can fail closed on leftover cgroup load.** Work-item sweep does call reconcile. `CleanupOrphanedCapsule` then `Load`s the leftover cgroup; after a guest restart that load can fail even though the identity is no longer live, so revoke acknowledgement never commits and the public run stays `running`.

Overlay-bind failure after restart (`AssignmentHandle` empty) used generic `persistActivationState` and did not join assignment cancel.

This is not a freeze. Super `f8ee744f` remains completed. Do not retry the operations POST while this CoSuper run is live.

## Repair

Index `state` on assigned-CoSuper run objects so rewarm can see them. Rewarm an assigned CoSuper only when the capsule is still usable; otherwise reconcile and do not wake. Join overlay-bind failure through `terminalizeRun`. If leftover cgroup load fails, kill/remove the exact path before requiring absence.
