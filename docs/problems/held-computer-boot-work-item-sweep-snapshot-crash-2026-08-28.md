# Unheld Guest Dies in Boot Work-Item Sweep After Super Rewarm

<problem_id: held-computer-boot-work-item-sweep-snapshot-crash-2026-08-28>
<first_observed: 2026-08-28T12:49Z>
<mutation_class: red>
<deployed_commit: 093c270a370c1190b56b6456dc6ec68dfa05dbdb>
<affected_surfaces: [internal/store/lifecycle.go, internal/agentcore/runtime.go, guest systemd go-choir-autoputer]>

## 1. Problem Description

`093c270a` (boot-health `bootInProgress` + pending-delivered worker-update
index) is live on `computer-03335285269bdba4f94377e56879f9e6`. Deploy
refresh epoch **824** boots `commit=093c270a`
(`deployed_at=2026-08-28T12:48:50Z`).

Owner-scoped Super rewarm **completes**. First time this recovery arc
logs `boot persistent-Super rewarm dispatched` and
`boot terminal outcome owner-scoped`. `Runtime.Start` still never logs
`runtime: started`.

The guest dies during `sweep_open_work_item_actors` (`open_items=20`).
systemd restarts. Host lookup grace keeps epoch 824 `active`.

## 2. Evidence (live, 2026-08-28 UTC, choir.news)

CI `33171067533` success including Deploy Node B. Guest
`x-choir-build-commit=093c270a`. No extra product-path start.

Generation 1:

| t (UTC) | observation |
|---|---|
| 12:48:50 | `deployed_at`; commit `093c270a`; host epoch **824** `active` |
| 12:49:10 | passivate 82ms; Super rewarm `delivered-pending-runs=1280` |
| 12:49:11 | prefetch `packets … pending=0` (no `validate run=`) for empty tombstones |
| 12:49:11–12:49:20 | four `validate run=` false positives, then **`rewarm dispatched run=02668507-…`**; phase 9.485s |
| 12:49:20 | **`boot terminal outcome owner-scoped candidates=0`** |
| 12:49:20 | `sweep_open_work_item_actors` `open_items=20` — last gen-1 log |
| 12:48:50–12:53:08 | `deployed_at` stable; health 200; host `active` 824; no `runtime: started` |

Generation 2 (systemd restart, same binary):

| t (UTC) | observation |
|---|---|
| 12:53:37 | listen `0.0.0.0:8085`; same `deployed_at` |
| 12:53:52–12:53:58 | passivate Super `7a627b53-…` (was pending) 6.669s |
| 12:53:58 | Super rewarm `candidates=41`; candidate `7a627b53-…` |

## 3. Mechanism

`sweepOpenWorkItemActors` groups open assigned work by
owner+agent+trajectory. Super reconcile is once-per-agent. Each
Researcher trajectory calls `ListAllPendingLifecycleUpdates` →
`ListPendingLifecycleUpdates` → **`ReadObjectSnapshot` of every kind**
(runs, events, texture, worker-updates). 20 items ⇒ up to 20 full-graph
materializations on a 4 GiB guest.

The pending-control predicate only needs `choir.worker_update` rows with
`target_agent_id`, `disposition=pending`, empty `delivered_to_loop_id`.

## 4. Non-fixes

- Do not re-hold, SSH, HTTP Super-start, or empty the computer.
- Do not raise 32 GiB `data.img`.
- Do not skip the work-item sweep or Texture owner start.
- Do not keep `ReadObjectSnapshot` as the pending-mailbox read.

## 5. Fix (general product path)

`ListPendingLifecycleUpdates` lists `choir.worker_update` by
`ListObjectsByOwnerAndBody` (`$.target_agent_id`, `$.disposition=pending`,
missing/empty `$.delivered_to_loop_id`). Filter `DeliveredAt==nil` and
`LifecycleVersion>0` in memory. Apply the caller limit after sort.

After deploy: do not extra-start if deploy refresh already booted.
Observability must show `runtime: started` plus owner-scoped Super and
terminal logs, `deployed_at` stable ≥3 minutes, no 502 recycle.
