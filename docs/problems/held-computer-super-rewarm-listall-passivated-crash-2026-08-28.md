# Unheld Guest Dies During Super Rewarm ListAllRunsByState(passivated)

<problem_id: held-computer-super-rewarm-listall-passivated-crash-2026-08-28>
<first_observed: 2026-08-28T09:20Z>
<mutation_class: red>
<deployed_commit: 3a38a6e8fab206299d0cd09d18d24bc2539f8089>
<affected_surfaces: [internal/agentcore/runtime.go, internal/agentcore/super_controller.go, internal/store/graph_store.go, guest systemd go-choir-autoputer]>

## 1. Problem Description

`3a38a6e8` (owner-scoped terminal-outcome boot repair) is live on
`computer-03335285269bdba4f94377e56879f9e6`. Product-path start receipt
`01a047ab` epoch **816** boots the new autoputer binary
(`commit=3a38a6e8`, `deployed_at=2026-08-28T09:20:36Z`).

`Runtime.Start` still never logs `runtime: started` and never logs
`boot terminal outcome owner-scoped`. The guest dies ~35s after listen
**during** `rewarm_persistent_super`, after skipping two Super tombstones.

`04fd704d` stopped aborting Super rewarm on `ErrLifecycleInvalidTransition`
and stopped calling reconcile once per trajectory. It did **not** stop
loading every passivated run body via `ListAllRunsByState(passivated)`.
That scan still happens twice:

1. `rewarmInterruptedPersistentSuperActors` — full passivated keyset
2. `reactivateRestartedPersistentSuperControlRun` — the same keyset again
   for the first owner+agent candidate

Crash-loop passivation adds more Super tombstones (`05e9537e-…` is new
since `9023abbb`). Each boot makes the next `ListAllRunsByState` larger.

## 2. Evidence (live, 2026-08-28 UTC, choir.news)

CI `33157175911` deployed `3a38a6e8` (Node B 09:17:56Z). Gateway health
`x-choir-build-commit: 3a38a6e8…`. Start
`POST /api/computers/computer-03335285269bdba4f94377e56879f9e6/lifecycle/start`
`201` receipt `01a047ab`, epoch 815 `degraded` → 816 `active`.

| t (UTC) | observation |
|---|---|
| 09:20:36 | guest `deployed_at`; commit `3a38a6e8` |
| 09:20:42 | topology + `starting server on 0.0.0.0:8085` |
| 09:20:42 | start receipt; lifecycle `active` epoch 816 |
| 09:20:57 | boot phases through `rewarm_lifecycle_activations` (90ms) |
| 09:20:58 | `boot phase begin rewarm_persistent_super`; candidate `25493355-…` |
| 09:21:03 | skip Super tombstone `05e9537e-…` (new) |
| 09:21:05 | skip Super tombstone `5c5a2f9d-…` |
| 09:21:08 | still HTTP 200, `runtime_health=ready`, 13 log lines; **no** dispatched, **no** terminal-outcome log |
| 09:21:17 | lifecycle still `active`; runtime unreachable; host disk warning flips to critical |
| 09:22:22 | observability timeout; bootstrap 502 `failed to resolve user autoputer` |

No `runtime: boot persistent-Super rewarm dispatched`. No
`runtime: boot terminal outcome owner-scoped`. No `runtime: started`.

Guest disk while reachable: 11.2 / 31.2 GiB, `critical=false`. Host
VM-dir occupancy is the known mismatch, not guest ENOSPC.

## 3. Mechanism

`rewarmInterruptedPersistentSuperActors` (`runtime.go`):

```go
runs, err := rt.store.ListAllRunsByState(ctx, types.RunPassivated)
```

`ListAllRunsByState` → `ogListAllByMetadata(choir.run, state, passivated)`
JSON_EXTRACT + full body for every passivated run.

Then for the first matching Super control run it calls
`reconcilePersistentSuperActor` → `reactivateRestartedPersistentSuperControlRun`,
which does **the same list again**, then
`listPendingLifecyclePacketsDeliveredToRun` per matching tombstone (~2s each).

`ListObjectRefsByKindOwner` already exists and is what `3a38a6e8` used for
terminal children. Super rewarm still takes the full-keyset path.

## 4. Non-fixes

- Do not re-hold, SSH, HTTP Super-start, raise 200-iter cap, or empty the
  computer.
- Do not raise 32 GiB `data.img` cap.
- Do not treat host VM-dir `used_percent>100` as guest ENOSPC.
- Do not skip Super rewarm or Texture owner start.
- Do not keep `ListAllRunsByState(passivated)` as a production fallback.

## 5. Fix (general product path)

1. Production computers (`computer-*` or `CHOIR_OWNER_ID` /
   `selfdevRouteOwnerID`) list passivated Super control runs through
   `ListObjectRefsByKindOwner` (indexed kind+owner, no body, no
   JSON_EXTRACT). `GetObject` only for header rows whose `state` is
   `passivated` and whose `agent_id` is the persistent Super (or
   `agent_profile=super`).
2. `reactivateRestartedPersistentSuperControlRun` uses that same
   owner-scoped list when `ownerID` is set. Tests without an owner keep
   `ListAllRunsByState` on tiny stores.
3. Log `boot persistent-Super rewarm owner-scoped` with candidate count
   so the next death names the remaining phase.

After deploy: product-path start, guest `commit=HEAD`, observability
shows `runtime: started` plus `boot persistent-Super rewarm owner-scoped`
and `boot terminal outcome owner-scoped`, `deployed_at` does not reset
for ≥3 minutes, and no 502 recycle.
