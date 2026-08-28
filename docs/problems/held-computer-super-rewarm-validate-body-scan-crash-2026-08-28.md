# Unheld Guest Dies JSON_EXTRACT-Scanning Worker-Update Bodies During Super Validate

<problem_id: held-computer-super-rewarm-validate-body-scan-crash-2026-08-28>
<first_observed: 2026-08-28T13:21Z>
<mutation_class: red>
<deployed_commit: 82cbd2b73750>
<affected_surfaces: [internal/agentcore/super_controller.go, internal/store/lifecycle_control_delivery.go, internal/objectgraph/dolt_store.go, guest systemd go-choir-autoputer]>

## 1. Problem Description

`82cbd2b7` (kind-scoped `ListPendingLifecycleUpdates` for the work-item
sweep) is live on `computer-03335285269bdba4f94377e56879f9e6`. Deploy
refresh epoch **826** boots `commit=82cbd2b7`
(`deployed_at=2026-08-28T13:21:44Z`).

The pending-delivered index still pays: `delivered-pending-runs=1280`
and `packets … pending=0` skip empty tombstones. Super rewarm then
logs four `validate run=` (`fe92ea2b`, `aa8f274b`, `9f2ff6d0`,
`80e55525`) and never `rewarm dispatched`, never
`boot work-item sweep researcher`, never `runtime: started`.

Each `validate` still calls `ListLifecycleControlsDeliveredToRunPage` →
`listWorkerUpdateObjects` → `ListObjectsByOwnerAndBody($.delivered_to_loop_id)`.
That `JSON_EXTRACT`s **every** `choir.worker_update` body for the owner
to find one run. Four false-positive index hits do that scan four times
on a 4 GiB guest. Process dies ~5 minutes after listen. Host `degraded`
epoch 826.

The work-item-sweep snapshot fix in `82cbd2b7` is unproven: this boot
never reached `sweep_open_work_item_actors`.

## 2. Evidence (live, 2026-08-28 UTC, choir.news)

No extra product-path start. Deploy refresh already booted epoch 826.

| t (UTC) | observation |
|---|---|
| 13:21:44 | guest `deployed_at`; commit `82cbd2b7`; host epoch **826** `active` |
| 13:22:03–13:22:09 | passivate owner-scoped Super `2d0d89af-…` 6.01s |
| 13:22:09 | Super rewarm `candidates=43`; `delivered-pending-runs=1280` |
| 13:22:09–13:22:10 | prefetch `packets … pending=0`; skip non-empty `TrajectoryID`; **`validate run=` ×4** |
| 13:22:10–13:26:18 | observability 200; host `active` 826; no dispatched; no `runtime: started` |
| 13:27:13 | observability timeout; then 502 `failed to resolve user autoputer` |
| after | host `degraded` epoch 826 |

`093c270a` completed the same four validates and dispatched Super
`02668507-…` in ~9s, then died in the work-item sweep. `82cbd2b7` dies
in the validates themselves. Same production path: per-run body
`JSON_EXTRACT`.

## 3. Mechanism

`CountPendingDeliveredWorkerUpdatesByRun` uses
`ListJSONBodyFieldsByKindOwner` (computer_id + JSON fields, **no body**).
That index is cheap and completed in <1s. False positives remain:
index pending>0, Super page then rejects (stale assignment trajectory,
wrong target agent, mixed delivered set).

`reactivateRestartedPersistentSuperControlRun` still called
`listPendingLifecyclePacketsDeliveredToRun` for every remaining
tombstone. That page:

1. `GetAgent` / `GetRun` (cheap)
2. `listWorkerUpdateObjects(delivered_to_loop_id=runID)` — owner-wide
   `JSON_EXTRACT` of every worker-update **body**
3. decode + Super-contract filter

Four sequential owner-wide body scans OOM or stall Dolt until systemd
kills the guest.

Recovery enqueue only needs **one** packet (`packets[0]`). The field
index already saw the canonical IDs.

## 4. Non-fixes

- Do not re-hold, SSH, HTTP Super-start, or empty the computer.
- Do not raise 32 GiB `data.img`.
- Do not skip Super rewarm or Texture owner start.
- Do not keep per-tombstone `ListLifecycleControlsDeliveredToRunPage`
  as the Super boot path.
- Do not treat host VM-dir `used_percent>100` as guest ENOSPC.

## 5. Fix (general product path)

1. `ListJSONBodyFieldsByKindOwner` returns `canonical_id`.
2. `PendingDeliveredWorkerUpdateCanonicalIDsByRun` maps run ID → newest
   pending worker-update IDs (cap 8).
3. Super rewarm `GetObject` those rows and keeps the first packet that
   matches Super recovery scope (owner/computer/target agent/trajectory,
   pending, delivered-to this run). No per-tombstone body scan.
4. Empty-owner `ListAllRunsByState` fallback still lists the page; production
   always has owner.

After deploy: do not extra-start if deploy refresh already booted.
Observability must show `rewarm dispatched`, then
`boot work-item sweep researcher`, then `runtime: started`,
`deployed_at` stable ≥3 minutes, no 502 recycle.
