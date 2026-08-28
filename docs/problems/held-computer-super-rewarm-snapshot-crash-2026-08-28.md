# Unheld Guest Dies on Super Delivered-Control Snapshot

<problem_id: held-computer-super-rewarm-snapshot-crash-2026-08-28>
<first_observed: 2026-08-28T10:08Z>
<mutation_class: red>
<deployed_commit: fb1c9e9303d9925710905f841817e0cdcee6fad4>
<affected_surfaces: [internal/store/lifecycle_control_delivery.go, internal/agentcore/super_controller.go, internal/objectgraph/dolt_store.go, guest systemd go-choir-autoputer]>

## 1. Problem Description

`fb1c9e93` (owner-scoped Super rewarm without `ListAllRunsByState`) is live
on `computer-03335285269bdba4f94377e56879f9e6`. Product-path deploy refresh
boots the new autoputer (`commit=fb1c9e93`,
`deployed_at=2026-08-28T10:07:53Z`, epoch **817**).

Owner-scoped listing **works**: observability shows
`boot persistent-Super rewarm owner-scoped` with `candidates=27` at
`10:08:12`. `Runtime.Start` still never logs `rewarm dispatched`,
`boot terminal outcome owner-scoped`, or `runtime: started`.

The guest dies ~25s after listen, still inside `rewarm_persistent_super`,
after skipping two Super tombstones. Host lookup grace expires →
`degraded` epoch 817 → `failed to resolve user autoputer`.

## 2. Evidence (live, 2026-08-28 UTC, choir.news)

Force-deploy CI `33160584506` success. Gateway health
`x-choir-build-commit: fb1c9e93…` at `2026-08-28T10:07:52Z`. Active VM
refresh produced epoch 817 `active` without a second `lifecycle/start`.

| t (UTC) | observation |
|---|---|
| 10:07:53 | guest `deployed_at`; commit `fb1c9e93` |
| 10:07:58 | topology + `starting server on 0.0.0.0:8085` |
| 10:08:12 | boot phases through `rewarm_lifecycle_activations` (77ms) |
| 10:08:12 | `boot persistent-Super rewarm owner-scoped owner=5bd6de97-… limit=1024` |
| 10:08:12 | `candidates=27`; candidate `a23afc9c-…` |
| 10:08:15 | skip Super tombstone `05e9537e-…: lifecycle invalid transition` |
| 10:08:17 | skip Super tombstone `5c5a2f9d-…: lifecycle invalid transition` |
| 10:08:14 | observability HTTP 200 (13 log lines); **no** dispatched, **no** phase end |
| 10:08:42 | runtime unreachable; host `degraded` epoch 817 |
| 10:09:21–10:11:50 | observability 502 `failed to resolve user autoputer` |

No `runtime: boot persistent-Super rewarm dispatched`. No
`runtime: boot terminal outcome owner-scoped`. No `runtime: started`.
`deployed_at` did not recycle; the process died.

## 3. Mechanism

`fb1c9e93` stopped loading every passivated **run body**. Reactivation of
the first owner+agent candidate still calls
`listPendingLifecyclePacketsDeliveredToRun` →
`ListLifecycleControlsDeliveredToRunPage`.

That page function:

1. `GetRunByOwner` (cheap). Super with non-empty `TrajectoryID` returns
   `ErrLifecycleInvalidTransition` **before** the snapshot — the two
   logged skips.
2. The first Super that satisfies empty `TrajectoryID` + matching
   `assignment_trajectory_id` then calls
   `graph.ReadObjectSnapshot(owner, computer)`:

```sql
SELECT canonical_id, object_kind, …, body, metadata, …
  FROM og_objects
 WHERE owner_id = ? AND computer_id = ?
 ORDER BY canonical_id
```

That is every kind in the owner graph, including `choir.run` and
`choir.event` bodies (the 132k-event tape lives here). Guest is 4 GiB.
Dolt is in-process. One snapshot is enough to OOM. systemd restarts.
CaptureBootLog never sees a later phase line.

`enqueuePersistentSuperRecoveryOccurrence` uses only `packets[0]`. The
boot path still materializes the entire computer to find that one
pending control.

`ListHistoricalLifecycleControlsDeliveredToRun` uses the same snapshot.

## 4. Non-fixes

- Do not re-hold, SSH, HTTP Super-start, raise 200-iter cap, or empty the
  computer.
- Do not raise 32 GiB `data.img` cap.
- Do not treat host VM-dir `used_percent>100` as guest ENOSPC.
- Do not skip Super rewarm or Texture owner start.
- Do not keep `ReadObjectSnapshot` as the delivered-control listing path.
- Do not JSON_EXTRACT `choir.run` or `choir.event` to find worker updates.

## 5. Fix (general product path)

1. `ListLifecycleControlsDeliveredToRunPage` and
   `ListHistoricalLifecycleControlsDeliveredToRun` list
   `choir.worker_update` for that owner+computer via `ListObjects`
   (`idx_og_objects_kind_owner`). Do not load other kinds. Fail closed if
   the kind-scoped scan exceeds a cap.
2. `reactivateRestartedPersistentSuperControlRun` skips Super runs with
   non-empty `TrajectoryID` from the run header **before** listing
   packets, so 27 tombstones do not each start a worker-update scan.
3. Log `persistent-Super rewarm validate run=` before the remaining
   packet list so the next death names the exact run.

After deploy: product-path start or deploy refresh, guest `commit=HEAD`,
observability shows `runtime: started` plus `boot persistent-Super rewarm
owner-scoped` and `boot terminal outcome owner-scoped`, `deployed_at`
does not reset for ≥3 minutes, and no 502 recycle.
