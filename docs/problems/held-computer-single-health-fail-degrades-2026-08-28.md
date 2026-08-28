# Single Lookup Health-Fail Marks Unheld Computer Degraded After Recover

<problem_id: held-computer-single-health-fail-degrades-2026-08-28>
<first_observed: 2026-08-28T06:00Z>
<mutation_class: red>
<deployed_commit: 445c8fc2f4c42060478eb62fca3d8b9caaa1677b>
<affected_surfaces: [internal/vmctl/ownership.go, internal/vmmanager/manager.go, internal/autoputer/run.go, internal/autoputer/file_sync.go, internal/agentcore/api.go]>

## 1. Problem Description

After product-path recover cleared **both** holds, `computer-03335285269bdba4f94377e56879f9e6`
booted `active` (epoch 805, then 806), served matching `/api/shell/bootstrap` 200s,
and admitted researchers (`researcher_count=3`, so `RUNTIME_MAINTENANCE_HOLD` is
off and `Adapter.Start` / Texture reconcile **succeeded**).

Within tens of seconds the same computer is `state: degraded`. API-key bootstrap
then 502s (`failed to resolve user autoputer`) because
`resolveComputerURLForComputerTarget` requires `state == "active"`. A later
`POST .../lifecycle/start` returns it to `active` (new epoch) and the cycle
repeats.

This is **not** the hold-fatal crash-loop (`eb27cac8` already made Texture
reconcile benign while held). The unheld guest reaches `runtime_health=ready`.
The new defect is that **one failed vmctl lookup health probe**
(`CheckHealth` → `probeGuestHealth`, 3s HTTP timeout on `GET {HostURL}/health`)
writes `VMStateDegraded` (`ownership.go` lookup path ~1738) with no consecutive-
failure threshold and without the routing grace
(`activeVMCanRouteDuringHealthGrace`). API-key traffic then treats a still-booting
or briefly stalling guest as a dead realization.

## 2. Evidence (live, 2026-08-28 UTC, choir.news)

- Recover `01a046ee`: epoch 804 `stopped` → 805 `active` (12.7s). Three
  bootstrap 200s (154ms / 138ms / 416ms) including after a 20s window.
  compute/status: `premium_always_on`, `runtime_health=ready`, `researcher_count=3`,
  guest disk 11.0 / 31.2 GiB, `critical=false`.
- ~6 min later: epoch 805 `degraded`; compute/status `runtime` omitted; host
  VM-dir occupancy warning `persistent data image is critically full` (98 GiB
  allocated blocks vs 32 GiB `data.img` virtual size — **not** guest-ext4
  fullness; see `held-computer-persistent-image-critically-full-2026-08-28`).
- Start `01a046f7`: 805 `degraded` → 806 `active` (12.1s). Immediate bootstrap
  200 in 128ms; guest disk 11.76 / 31.2 GiB. ~20s later bootstrap 502 (10.1s),
  status `degraded` epoch 806.
- Code: `activeResolveReadinessCheckInterval = 10s`. After that, lookup calls
  `mgr.CheckHealth`; a single `healthy=false` (timeout, 5xx, or systemd restart
  window) persists `degraded`.
- `MaybeRunDoltGC` still runs at autoputer startup (`run.go:131-135`) **before**
  store-open — the 2026-08-24 B11 OOM family is still in the boot path. Live
  epoch 806 reached `researcher_count=3`, so that GC either skipped or finished;
  it remains a 4 GiB-guest landmine and is **not** disproven.
- File-CAS `HydrateIfNeeded` is a no-op when the local Files tree already has
  regular files. This computer almost certainly has a local tree, so boot does
  **not** restore from CAS. Track F restore is unpaid until a product-path
  force-hydrate exists or the tree is proven empty.

## 3. Required Repair Invariants

1. **Do not re-hold.** Host Unhold + unheld boot already proved the guest fence
   is off. Further recover/start must not set `RUNTIME_MAINTENANCE_HOLD`.
2. **Do not mark `degraded` on a single lookup probe.** Require consecutive
   failures outside `activeResolveUnhealthyRouteGrace`, or keep routing to the
   persisted `ComputerURL` while the manager instance is `running`.
3. **Product-path diagnosis first.** Next start must immediately `GET
   /api/runtime/observability` (bounded `recent_log`) plus `/health` for ≥60s.
   Do not SSH. Capture whether the process fatals or only the host state flips.
4. **File-CAS restore is a separate product verb.** Do not rely on
   `HydrateIfNeeded` for a non-empty tree. Do not raise the 32 GiB cap.
5. **Acceptance:** `active` for a 60s window, two consecutive matching
   bootstraps at t=0 and t=60s, `runtime_health=ready`, no flip to `degraded`,
   observability log without `runtime startup refused` / OOM.

## 4. Classification & Ceremony

- **Mutation class:** `red` (vmctl ownership state, guest boot, File-CAS).
- **Not a regression of** `eb27cac8` (hold-benign) or `445c8fc2` (recover verb).
- **CI substrate:** push CI `33146514765` for `fdb0759c` failed non-runtime
  shard 5 on `TestAdapterSQLitePreBindResearcherRecoveryBindsAndExecutesWithoutSnapshot`
  (`SQLITE_BUSY`) — actorruntime flake, not the proxy WithoutCancel test (that
  test passed). Do not patch that test as the first move; force-deploy the
  already-pushed SHA.
- **Rollback:** leave epoch 806 `degraded`; do not destroy `data.img`.

## 5. Next Safe Probe

1. Land `fdb0759c` (bootstrap resolve `WithoutCancel`) via
   `workflow_dispatch force_staging_deploy=true` after the failed push CI
   settles. Unheld refresh should auto-start the computer.
2. The instant status is `active`, `GET /api/runtime/observability` and
   `/api/shell/bootstrap` on a 5s cadence for 60s. Record `recent_log`.
3. Only then decide among: debounce `degraded`, defer startup `MaybeRunDoltGC`,
   or File-CAS force-hydrate.
