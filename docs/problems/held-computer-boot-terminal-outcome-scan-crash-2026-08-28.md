# Unheld Guest Dies After Super Rewarm During Terminal-Outcome Scan

<problem_id: held-computer-boot-terminal-outcome-scan-crash-2026-08-28>
<first_observed: 2026-08-28T07:52Z>
<mutation_class: red>
<deployed_commit: 9023abbbc702dfacb85dfdef581d3710eeefff67>
<affected_surfaces: [internal/agentcore/runtime.go, internal/agentcore/researcher_checkpoint_fallback.go, internal/store/graph_store.go, internal/store/lifecycle.go, guest systemd go-choir-autoputer]>

## 1. Problem Description

`04fd704d` (Super tombstone skip + once-per-agent boot sweep) is live.
Product-path start of `computer-03335285269bdba4f94377e56879f9e6` boots
the new autoputer binary. Persistent-Super rewarm now **skips** control
run `5c5a2f9d-…` (`lifecycle invalid transition`) and **dispatches** a
healthy Super. `Runtime.Start` still never logs `runtime: started`.

The guest dies ~90s after listen, same as the previous crash-loop, but
the log no longer shows per-trajectory Super scans. The next synchronous
boot phase is `recoverOpenWirePublicationClaims` then
`reconcileTerminalRunOutcomes`, which materializes **every**
completed/failed/cancelled run in the object graph (`ListAllRunsByState`
+ `ListLifecycleRunsByState` → `ogListAllByMetadata`) and then
`GetLifecycleRun` / `ensurePersistedTerminalRunOutcome` (parent run +
agent + `ListWorkerUpdatesBySourceRun`) per record.

That is historical-terminal audit, not crash-window repair. Guest is 4 GiB.
Process dies without a Go fatal. systemd restarts. Host lookup grace
expires → `degraded` → `failed to resolve user autoputer`.

## 2. Evidence (live, 2026-08-28 UTC, choir.news)

Gateway `deployed_commit=04fd704d` at `2026-08-28T07:49:30Z`
(CI `33151481361`). Start receipt `01a0475b` epoch **811**.

| t (UTC) | observation |
|---|---|
| 07:52:21 | guest build `04fd704d` `deployed_at` |
| 07:52:26 | topology + `starting server on 0.0.0.0:8085` |
| 07:52:41 | Super rewarm candidate `25493355-d0d5-4a95-bf29-4d9331d5cd67` |
| 07:52:46 | `skip restarted persistent-Super control run 5c5a2f9d-…: lifecycle invalid transition` |
| 07:52:55 | `boot persistent-Super rewarm dispatched run=25493355-…` |
| 07:53:09 | HTTP 200, 10 `recent_log` lines, **no** `runtime: started` |
| 07:54:12 | 502 `failed to resolve user autoputer`; lifecycle `degraded` epoch 811 |

No `validate restarted` per-trajectory loop. No `runtime: started`.
`CaptureBootLog` (512) would have kept later phase logs if any were
emitted; `Runtime.Start` currently logs nothing between Super dispatch
and `runtime: started`.

## 3. Mechanism

`Runtime.Start` order after Super rewarm:

1. `recoverOpenWirePublicationClaims` — open `wire_publication` work items only
2. `reconcileTerminalRunOutcomes` — **all** terminal runs, three states
3. `sweepPassivatedSpawnedCoagentWork` — `ListRunsByState(passivated, 1000)` (today still load-all-then-truncate)
4. `sweepOpenWorkItemActors`
5. `sweepPendingUpdateActors`
6. `log runtime: started`

`reconcileTerminalRunOutcomes`:

- `ListLifecycleRunsByState` calls `ogListAllByMetadata(choir.run, state, …)`
  and holds every matching object, then `ReconcileLifecycleSettlementForTerminalRun`
  → `GetLifecycleRun` even when `lifecycle_terminal_settlement_requested` is false.
- `ListAllRunsByState` accumulates every generic terminal run, then
  `ensurePersistedTerminalRunOutcome` for each capable profile. Root runs
  return after a cheap `RequestedByRunID` check, but delegated children
  resolve parent run + requester agent + list producer updates.

Crash-window repair only needs terminal children whose outcome packet is
missing. Historical already-bound children are the volume.

## 4. Non-fixes

- Do not re-hold, SSH, HTTP Super-start, raise 200-iter cap, or empty the
  computer.
- Do not raise 32 GiB `data.img` cap.
- Do not treat host VM-dir `used_percent>100` as guest ENOSPC.
- Do not skip Super rewarm or Texture owner start.

## 5. Fix (general product path)

1. `Runtime.Start`: log begin/end/duration of each boot phase so the next
   death names a phase.
2. Store: page `ForEachRunsByState` / `ForEachLifecycleRunsByState` without
   accumulating the full keyset. `ListRunsByState` must stop at `limit`
   instead of load-all-then-truncate.
3. `reconcileTerminalRunOutcomes`: skip lifecycle `GetLifecycleRun` unless
   the listed record requests settlement; skip generic roots with empty
   `RequestedByRunID` before `ensurePersistedTerminalRunOutcome`.
4. `ensurePersistedTerminalRunOutcome`: `GetWorkerUpdate` of the
   deterministic synthetic outcome ID before parent/agent/producer-list
   work, so already-repaired children are O(1) by ID.
5. Production computers (`computer-*` or `CHOIR_OWNER_ID`) must not call
   `ForEachRunsByState` / `ForEachLifecycleRunsByState` at boot. Repair
   recent delegated children through `ListObjectRefsByKindOwner` +
   `GetObject` (indexed kind+owner, no body, no JSON_EXTRACT). Tests
   without an owner keep the exhaustive walk on tiny stores.

After deploy: product-path start, guest `commit=HEAD`, observability
shows `runtime: started` plus `boot terminal outcome owner-scoped`,
boot `deployed_at` does not reset for ≥3 minutes, and no 502 recycle.

## 6. Live proof that paging is not enough (2026-08-28T08:41Z)

`9023abbb` is on the guest (`deployed_at=2026-08-28T08:41:40Z`).
Product-path start receipt `01a04788` epoch **814**. Dense 8s polls:

| t (UTC) | observation |
|---|---|
| 08:41:59 | boot phases through Super rewarm begin; candidate `25493355-…` |
| 08:42:05 | skip Super tombstone `5c5a2f9d-…` |
| 08:42:15 | Super **dispatched**; `rewarm_persistent_super` 16.428s; wire claims 27ms; **`boot phase begin reconcile_terminal_run_outcomes`** |
| 08:42:15–08:42:42 | no `scanned=256`, no lifecycle `scanned=` |
| 08:42:42 | process restart (`starting server on 0.0.0.0:8085`); same `deployed_at` |
| 08:43:01 | second boot Super rewarm begin |
| 08:43:28 | `count running processor runs: Error 1105: context canceled` |
| 08:43:47 | lifecycle `degraded` epoch 814; observability 502 |

`ListObjectsByMetadataPage` still does
`JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?))` over `og_objects`
and selects **body**. Page size 256 does not prevent a full-kind scan or
loading huge run payloads. The first page never returns, so boot-phase
progress logs never fire. Indexes on this table are
`(object_kind, owner_id)` and `(updated_at)` — not metadata.state.

Follow-up fix: production boot repair lists **headers** via
`idx_og_objects_kind_owner` (`ListObjectRefsByKindOwner`) and
`GetObject` only for recent terminal children with `requested_by_run_id`.
Do not JSON_EXTRACT the completed-state keyset at boot.

## 7. Live proof that owner-scoped terminal repair never ran (2026-08-28T09:20Z)

`3a38a6e8` is on the guest (`deployed_at=2026-08-28T09:20:36Z`). Product-path
start receipt `01a047ab` epoch **816**. Super rewarm still
`ListAllRunsByState(passivated)` twice. Process died ~35s after listen,
after skipping Super tombstones `05e9537e-…` and `5c5a2f9d-…`, **before**
`boot terminal outcome owner-scoped` and **before** `runtime: started`.

See `held-computer-super-rewarm-listall-passivated-crash-2026-08-28`.
