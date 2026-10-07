# management-open 500: `replace durable activation: lifecycle invalid transition` on fresh disposable

**Status**: confirmed on staging, reproducible. Blocks SMG close (legs 1–3
require `POST /api/texture/management-open` to succeed on a fresh computer).

**Found**: 2026-10-07, running `scripts/smg_rlm_acceptance_probe.mjs` on
disposable `computer-65cbfa4d55ef37a0ef420567a0e88ac2` (user
`8449219f-a61c-4dd3-bfa6-1bd896ac1bbc`, vm `vm-8f1caa42f962e6206a4c6d60e6d7f8b6`,
deployed `7a36713c`).

## Symptom

`POST /api/texture/management-open` (via proxy, API-key auth) returns
`500 "failed to open management"` on a fresh disposable computer whose
`provisioned genesis` minted successfully at boot.

Guest log line:

```
2026/10/07 08:24:19 management-open: issue control: start initial Texture
  agent revision: replace durable activation: lifecycle invalid transition
```

The `EnsureTextureHandoff` → `submitTextureAgentRevisionRun` →
`StartRunWithMetadata` → `persistLifecycleSubmittedRun` →
`ReplaceLifecycleActivation` chain fails with `ErrLifecycleInvalidTransition`.

## Evidence

- `bootstrap-chain` returned 200 for the fresh disposable (genesis present).
- Guest console confirms `provisioned genesis minted for computer-65cbfa4d…`
  — SA slice 0 fix is live in `7a36713c`.
- Guest health reports `commit: 7a36713c5555`, `status: ready`.
- An earlier disposable (`computer-6fa241fa`, same deploy) passed
  `management_open` at wall=1413ms on the same build.
- The failure is not "already open": this is the *first* management-open
  call on `computer-65cbfa4d` (marker `smg-close2-1791361153`).

## Root cause (confirmed 2026-10-07)

`StartLifecycle` commits the work item with `LifecycleVersion=1` and mints a
`lifecycle_work_assigned` actor wake for the texture agent in the same batch.
`sweepActorWakeOutbox` fires that wake immediately, calling
`reconcileAgentWakeLocked` → `submitTextureAgentRevisionRun` →
`ReplaceLifecycleActivation`, which commits `agent.ActiveRunID=run-A` while the
VM's EnsureTextureHandoff path is still between StartLifecycle and its own
submit call. When `EnsureTextureHandoff` then calls
`submitTextureAgentRevisionRun` (attempting run-B), `projectLifecycleRun` sees
`previousActiveRunID=run-A` still active → `ErrLifecycleInvalidTransition`.

The wake-race is timing-dependent: it fires only when the sweep goroutine wins
the interlock before EnsureTextureHandoff's `rt.activate` dispatch goroutine
schedules. Disposable computers hit it ~50% of the time; slower stores miss it.

Confirmed via instrumented `projectLifecycleRun`: the failing gate is
`lifecycleRunOwnsActivation(previousRun.State)` at the previous-active-run
check — `prevRunID="run-winner-1" prevState="pending"`.

## Fix shape (landed)

`submitTextureAgentRevisionRun` treats `ErrLifecycleInvalidTransition` as a
possible raced activation: it loads the agent's `ActiveRunID` via
`recoverRacedTextureActivation` and returns that run if it is still active.
The winner's run is the canonical activation; the loser returns it idempotently.

## Rollback

git revert of the fix commit.

