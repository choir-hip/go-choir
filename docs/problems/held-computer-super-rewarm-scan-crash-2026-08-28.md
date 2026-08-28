# Unheld Guest Recycles During Persistent-Super Boot Sweep

<problem_id: held-computer-super-rewarm-scan-crash-2026-08-28>
<first_observed: 2026-08-28T07:12Z>
<mutation_class: red>
<deployed_commit: 757af7e14a4418189bf98d8d247da7e5b3c27b79>
<affected_surfaces: [internal/agentcore/runtime.go, internal/agentcore/super_controller.go, internal/store/lifecycle_control_delivery.go, guest systemd go-choir-autoputer]>

## 1. Problem Description

`757af7e1` (lookup health-grace) is live. Host routing for
`computer-03335285269bdba4f94377e56879f9e6` stays `active` across guest
blips: bootstrap 200, observability 200, `runtime_health=ready`.

The **guest autoputer process still dies ~100s after listen** and systemd
`Restart=on-failure` (1s) brings it back. Host epoch stays the same (809).
`CaptureBootLog` never records `runtime: started` before the death, so
`Runtime.Start` does not finish. Texture.Start has not run yet.

This is not the lookup-demote bug and not the held Texture fatal. It is a
general boot-sweep defect: persistent Super reconcile is owner+agent scoped
but the open-work sweep invokes it **once per trajectory**, and each call
does `ListAllRunsByState(passivated)` then aborts the whole Super rewarm on
the first `ErrLifecycleInvalidTransition`.

## 2. Evidence (live, 2026-08-28 UTC, choir.news)

Gateway `deployed_commit=757af7e1` at `2026-08-28T07:10:51Z` (force-deploy
CI `33149135092` success).

Guest generations (same epoch 809):

| listen | death | downtime |
|---|---|---|
| 07:12:34 | 07:14:27 | ~3s |
| 07:14:27 | 07:16:10 | ~3s |
| 07:16:10 | 07:17:52 | ~3s |
| 07:17:55 | (watch) | |

Observability poll 07:17:27–07:17:52 (generation 07:16:10):

- `07:16:10` topology + `starting server on 0.0.0.0:8085` (replay path:
  HTTP listens before `rt.Start`)
- `07:16:23` SPA underivable (deferred, not fatal)
- `07:16:23–07:16:28` Super rewarm candidate
  `fe92ea2b-…` → `validate restarted persistent-Super control run
  5c5a2f9d-e258-4a9b-828c-023e33aab144: lifecycle invalid transition`
- `07:17:03–07:17:40` **same** invalid transition, once per work-item
  trajectory (`e826402d`, `5242ca03`, `3fa254bc`, `d8ccd11b`, `91492b4e`,
  `d6f1b0ee`, `24693e87`, `123d42f9`, …) ~5s apart
- no `runtime: started`, no `runtime startup refused`, no OOM line
- `07:17:44` still HTTP 200; `07:17:52` 502; `07:17:55` new listen

`researcher_count=3` is orchestration **config** (`rtCfg.ResearcherCount`),
not proof Texture reconcile finished.

Guest disk while live: 11.2 / 31.2 GiB, `critical=false`. Do not treat as
ENOSPC. Host VM-dir block accounting remains a separate mismatch
(`held-computer-persistent-image-critically-full-2026-08-28`).

## 3. Mechanism

1. `run.go` with a computer-event appender: `go runReplayPhase` then
   `s.Start()`. HTTP is up during reconstruct + `Adapter.Start`.
2. `Runtime.Start` (void): passivate → Super rewarm (once,
   `ListAllRunsByState`) → passivated spawned-work (up to 1000) →
   `sweepOpenWorkItemActors`.
3. Sweep groups open work by `owner+agent+trajectory`. For persistent Super
   it calls `reconcilePersistentSuperActor(owner, agent)` **per group**.
4. That calls `reactivateRestartedPersistentSuperControlRun`, which
   **re-lists every passivated run** and on
   `ListLifecycleControlsDeliveredToRunPage` → `ErrLifecycleInvalidTransition`
   **returns** (`super_controller.go:570`). Super runs with a non-empty
   `TrajectoryID` always fail that page
   (`lifecycle_control_delivery.go:488-490`: Super must have empty
   `TrajectoryID` and matching `assignment_trajectory_id`).
5. Error aborts minting a healthy Super from pending controls. Sweep retries
   the same full-table scan for the next trajectory. Dolt working set grows;
   guest is 4 GiB (`MaybeRunDoltGC` already refuses GC above 5 GiB used).
   Process dies without a Go fatal (OOM or equivalent). systemd restarts in
   ~3s. Lookup grace keeps host `active`.

## 4. Non-fixes

- Do not re-hold, SSH, HTTP Super-start, raise 200-iter cap, or empty the
  computer.
- Do not raise 32 GiB `data.img` cap for the disk warning.
- Do not “harden” Super continuation tests with ListRuns fallback (already
  made `TestPersistentSuperReplacementContinuationAfterUnflaggedClaim` flake
  worse).
- Do not treat `researcher_count=3` as Texture-success.

## 5. Fix (general product path)

1. `reactivateRestartedPersistentSuperControlRun`: on
   `ErrLifecycleInvalidTransition`, **skip that run** and keep scanning;
   do not fail the whole Super reconcile.
2. `sweepOpenWorkItemActors`: reconcile persistent Super **once per
   owner+agent**, not once per trajectory (the reconciler is already
   owner+agent scoped and already selects pending controls across
   trajectories).

After deploy: same 60s+ product-path probe **and** observability must show
`runtime: started` and a boot timestamp that does not reset for ≥3 minutes.
