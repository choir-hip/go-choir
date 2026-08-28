# Unheld Guest Dies Listing Every Worker-Update Body During Super Rewarm

<problem_id: held-computer-super-rewarm-worker-update-scan-crash-2026-08-28>
<first_observed: 2026-08-28T11:18Z>
<mutation_class: red>
<deployed_commit: 15e6d6d065e9e448d538fdc0970f49dd26d4657e>
<affected_surfaces: [internal/store/lifecycle_control_delivery.go, internal/agentcore/super_controller.go, internal/agentcore/runtime.go, internal/store/graph_store.go, guest systemd go-choir-autoputer]>

## 1. Problem Description

`15e6d6d0` (header-scan `latestLifecycleRunByAgent`) is live on
`computer-03335285269bdba4f94377e56879f9e6`. Product-path start receipt
`01a04817` epoch **821** boots `commit=15e6d6d0`
(`deployed_at=2026-08-28T11:18:32Z`).

Owner-scoped Super listing, tombstone `TrajectoryID` skip, and
`persistent-Super rewarm validate run=` all fire. `Runtime.Start` still
never logs `rewarm dispatched`, `boot terminal outcome owner-scoped`, or
`runtime: started`.

The guest dies ~20s after listen **while listing delivered controls for
every empty-trajectory Super tombstone**. systemd restarts. The second
generation dies in `passivate_interrupted_activations` via
`ListRunsByState(pending|running)` (`JSON_EXTRACT` + body). Host
`degraded` epoch 821.

## 2. Evidence (live, 2026-08-28 UTC, choir.news)

CI `33164995176` deployed `15e6d6d0`. Start
`POST .../lifecycle/start` idempotency
`effects-start-15e6d6d0-20260828T1117Z` → 201, epoch 820 `degraded` →
821 `active`.

| t (UTC) | observation |
|---|---|
| 11:18:32 | guest `deployed_at`; commit `15e6d6d0` |
| 11:18:38 | topology + listen |
| 11:19:00 | 48 log lines: `boot persistent-Super rewarm owner-scoped`, `skip … non-empty trajectory_id`, **`validate run=aa8f274b` / `9f2ff6d0` / `80e55525`** |
| 11:19:00 | no dispatched, no `runtime: started` |
| 11:19:12 | observability 502; host still `active` 821 |
| 11:19:29 | systemd restart, new listen, same `deployed_at` |
| 11:19:43 | `boot phase begin passivate_interrupted_activations`; skip spawned work for Super `a6cc1845-…` |
| 11:19:57 | process dead |
| 11:20:45 | host `degraded` epoch 821 |

## 3. Mechanism

`d17457f1` replaced `ReadObjectSnapshot` with
`ListObjects(kind=choir.worker_update, owner, computer)`. That still
`SELECT`s **every worker-update body** for the computer (cap 65536).
`reactivateRestartedPersistentSuperControlRun` then repeats that scan
for each empty-`TrajectoryID` Super tombstone (`validate run=` ×3 in
one second). Crash-loop passivation mints a new empty-trajectory Super
each boot (`b563243b`, `a6cc1845`, …), so the scan set grows.

`listWorkerUpdateObjects` does not predicate on
`delivered_to_loop_id`. The page function already drops rows whose
`DeliveredToRunID != targetRunID` **after** loading them.

Second death: `passivateInterruptedActivations` still
`ListRunsByState(pending|running, 100)` → `JSON_EXTRACT` over
`og_objects` + body. Concurrent `/health` `RunningCountByProfile` does
the same for `running`.

## 4. Non-fixes

- Do not re-hold, SSH, HTTP Super-start, raise 200-iter cap, or empty the
  computer.
- Do not raise 32 GiB `data.img` cap.
- Do not skip Super rewarm or Texture owner start.
- Do not load every `choir.worker_update` body to find one run's packets.
- Do not keep `ListRunsByState` as production boot/health.

## 5. Fix (general product path)

1. `listWorkerUpdateObjects` lists by
   `ListObjectsByOwnerAndBody($.delivered_to_loop_id = targetRunID)`.
   Do not materialize other runs' packets.
2. `reactivateRestartedPersistentSuperControlRun` stops at the first
   newest-first Super with pending controls (refs are already
   `updated_at DESC`).
3. Production `passivateInterruptedActivations` and
   `RunningCountByProfile` list `choir.run` headers by kind+owner and
   `GetObject` only matching pending/running rows.

After deploy: product-path start, guest `commit=HEAD`, observability
shows `runtime: started` plus `boot persistent-Super rewarm owner-scoped`
and `boot terminal outcome owner-scoped`, `deployed_at` does not reset
for ≥3 minutes, and no 502 recycle.

## 6. Live proof that per-run JSON_EXTRACT still dies (2026-08-28T11:52Z)

`5e5e8671` is on the guest (`deployed_at=2026-08-28T11:52:01Z`). Deploy
refresh epoch **823**. Observability:

| t (UTC) | observation |
|---|---|
| 11:52:58 | listen |
| 11:53:12 | `boot passivation owner-scoped candidates=1`; Super `02668507-…` |
| 11:53:19 | passivate 6.683s; Super rewarm `candidates=40`; candidate `02668507-…` |
| 11:53:20 | `validate run=02668507-…` |
| 11:53:24 | `packets run=02668507-… pending=0` (SQL filter paid, ~4s) |
| 11:53:25 | `validate run=9151c01b-…` — second full worker-update JSON_EXTRACT |
| 11:53:36–11:54:01 | `/health` `RunningCountByProfile` → `ListObjectRefsByKindOwner` **context canceled** |
| 11:54:13 | observability 502; host `degraded` epoch 823 |

No `rewarm dispatched`. No `runtime: started`. Owner-scoped passivation
held. First empty-trajectory Super listed in 4s with pending=0. The
second tombstone plus concurrent health header scans killed Dolt.

Follow-up: in-memory processor count until `runtime: started`; one
worker-update `ListObjects` to skip tombstones with no delivered packets
before any per-run JSON_EXTRACT.

## 7. Follow-up fix (not yet live)

`5e5e8671` paid per-run `JSON_EXTRACT($.delivered_to_loop_id)` and owner-scoped
passivation. Live death: second tombstone extract plus concurrent `/health`
`RunningCountByProfile` → `ListObjectRefsByKindOwner` context-canceled.

Code in this landing:

1. `Runtime.bootInProgress` — while `Runtime.Start` is executing,
   `RunningCountByProfile` uses in-memory `RunningCount()`. HTTP is up
   during boot; health must not share the Dolt engine with Super rewarm.
   Default false so overload admission still counts durable processors.
2. `ListJSONBodyFieldsByKindOwner` extracts `$.delivered_to_loop_id` and
   `$.disposition` for `choir.worker_update` without `SELECT body`.
3. `CountPendingDeliveredWorkerUpdatesByRun` builds that index once.
   `reactivateRestartedPersistentSuperControlRun` logs
   `delivered-pending-runs=N` and `packets run=… pending=0` **without**
   `validate run=` for tombstones that have nothing pending. Per-run
   `ListLifecycleControlsDeliveredToRunPage` runs only for a run with
   pending > 0, then still `break`s.

Do not start the guest until this commit is on it. Acceptance unpaid until
observability shows `runtime: started`, owner-scoped Super + terminal logs,
`deployed_at` stable ≥3 min, no 502 recycle.
