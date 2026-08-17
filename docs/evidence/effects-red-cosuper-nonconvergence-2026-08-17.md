# Effects CoSuper recovery non-convergence — structural assessment — 2026-08-17

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`

## The symptom

`run:assignment-fa38b037-bd9d-5270-a640-e668afa4eb57` (bound CoSuper under trajectory `e826402d-b666-503f-93ba-72b3bcf51e8d`) stays `running` with frozen `updated_at` (`2026-08-17T02:05:08.829Z`), empty result, no tokens. Its assignment stays `bound`/capsule `active`. A second assignment, `31b710eb-f928-5d16-9066-27b27ec29164`, stays `open`/capsule `unbound`.

## What was landed (three iterations, all correct in isolation)

1. `ed3ffa3f` — bound `ForceDestroy` process wait to 10s. Deployed, refreshed. No effect.
2. `beb32aeb` — indexed bind state; rewarm reconciles absent capsules. Deployed, refreshed. No effect.
3. `23fbd857` — computer-wide `ListCoSuperAssignmentsForComputer` + `reconcileCoSuperAssignmentCapsulesAfterRestart`, independent of run-state and work-item projections. Deployed, refreshed. No effect.

Each fix's unit/regression tests pass (`./internal/store`, `./internal/agentcore`).

## Confirmed root cause (substrate)

`rewarmInterruptedLifecycleActivations` discovers interrupted lifecycle runs via `ListLifecycleRunsByState`, which queries object metadata `$.state`. That index is only written by bind paths introduced in `beb32aeb`. `run:assignment-fa38b037` was bound before that, so it carries no `$.state` and is invisible to both the generic `ListRunsByState` and the lifecycle `ListLifecycleRunsByState`. `23fbd857` removes that dependency by listing the durable assignment projection directly (`ogKindCoSuperAssignment`, kind + computer filter), and `CancelCoSuperAssignment` does terminalize the bound run.

## Why the fix does not take effect

Owner-scoped refresh does force-reboot the guest (`vmmanager.RefreshVMWithConfig` → `killFirecrackerProcess` → `bootVM` with `RefreshRuntime`), and `CHOIR_CAPSULE_*` is baked into `nix/autoputer-vm.nix`, so the capsule executor should be wired. Yet across three refreshes the five assignments are byte-for-byte unchanged — including `31b710eb`, which the reconcile would have cancelled at minimum. Therefore boot reconcile is never reaching these assignments on the running guest.

The dependency that cannot be verified through the product API:

```
reconcile runs  <=  guest binary == deployed commit (c8580621)
                <=  guest boot reaches reconcileCoSuperAssignmentCapsulesAfterRestart
                <=  no guest-side error aborts Start / the sweep
```

`/api/acceptance/execution-identity` returns 503 (`execution identity authority unavailable`). The proxy `/health` in vmctl-routing mode suppresses the upstream autoputer build (`UpstreamBuild` unset). There is no product-API surface that reports the running guest's binary commit or its boot log. Standing question #9 (no-SSH diagnosis) is not satisfied by this surface.

## Dependency graph

```
CoSuper run terminal
  -> CancelCoSuperAssignment (store, terminalizes run)          [verified in code]
  -> ReconcileCoSuperAssignmentsForTrajectory (agentcore)        [verified, tests pass]
  -> reconcileCoSuperAssignmentCapsulesAfterRestart (new sweep)  [verified, tests pass]
  -> Runtime.Start boot recovery on guest                        [code path present]
  -> guest binary == c8580621                                   [UNVERIFIABLE via API]
  -> guest log (why reconcile did not run)                       [UNVERIFIABLE via API]
```

## Decision needed

The next step is not another runtime patch; it is one of:

(a) expose the running guest build identity + boot log through the product API so the deploy→guest cutover and reconcile outcome are observable (satisfies standing question #9); or
(b) confirm whether the constructed computer's guest image is actually replaced by the `Deploy to Staging (Node B)` job, or is pinned by G4 so refreshes boot a stale autoputer; or
(c) authorize a product-surface recovery action for the confirmed-hung assignment (explicit assignment/trajectory cancellation), replacing the "wait for reconcile" path.

I have not cancelled the assignment, retried the operations POST, self-promoted, sent mail, CAS'd `qualified_consensus`, or used OwnerRecovery `663540be` for promotion.
