# S0m finding: post-boot desk-mint cells never dispatch — boot re-drive does not cover live mints

**Date observed**: 2026-10-02
**Computer**: `computer-03335285269bdba4f94377e56879f9e6` (owner guest, `candidate-fleet-e15cb89f25d963c220319b7b`)
**Deployed**: autoputer `558afe86` (carries the `ogKindRun` initial_dispatch outbox + `MigrateActorWakeOutbox` repair)

## Symptom

Fresh `s0m_stranded_bound_probe` prompt-bar submissions on a healthy, post-reboot
guest mint a trajectory and complete the conductor run, but **never mint a
`texture:` desk cell** — the run registry (500-entry sweep) shows zero
`texture:*` runs for the probe channel, the trajectory carries `pending_updates:
0`, and no control packet is ever produced. Observed identically on five probe
trajectories (`d81d1bd4`, `bc35b9d8`, `5c8cf4ac`, `16585a04`, `b86cc432`).

The guest is otherwise live: `running_runs` 6-7, `researcher_count` 3, research
runs complete on other trajectories, the `db7b0701` texture cell for an older
trajectory runs and updates `updated_at` continuously.

## What is NOT the cause

- Guest liveness / epoch churn: guest is `ready` on `558afe86`, epoch 1026,
  single IP `10.200.189.2`, health green across the window.
- The earlier `s0m-desk-run-dispatch-stall` repair: `MigrateActorWakeOutbox`
  minted **2009 pending** outbox rows at the 23:14 boot and is running (we see
  `actor wake outbox disposed dead wake …` lines and live `running` cells). The
  repair heals pre-existing pending rows — it does not cover the mint-time drop.
- Model stall: `running` texture cells update `updated_at`; the failure is the
  cell never existing, not a hung inference.

## Mechanism (confirmed by guest log)

The desk-cell mint is not missing a dispatch — it is erroring inside
`submitTextureAgentRevisionRun` -> `persistLifecycleSubmittedRun` ->
`ReplaceLifecycleActivation`. The guest autoputer journal records:

```
texture prompt bar: submit: start initial Texture agent revision:
  replace durable activation: objectgraph dolt: scan object: context deadline exceeded
```

`StartRunWithMetadata` -> `persistLifecycleSubmittedRun` -> `ReplaceLifecycleActivation`
performs the run-object write through the objectgraph Dolt store; under the
post-boot outbox backlog (2009 re-armed rows draining serially) plus the
`selfdev_active_operations` churn, the scan exceeds its deadline, the commit
fails, `submitTextureAgentRevisionRun` returns the error, `ensureConductorTextureRoute`
returns error, and `HandlePromptBar` writes 500 (the staging proxy surfaces it
as the observed 502). The lifecycle trajectory + work item commit in the first
`StartLifecycle` step, but the run mint does not — leaving the trajectory `live`
with `pending_updates: 0`, an open `InitialWork` work item assigned to
`texture:<docID>`, and no desk run. Cells for earlier submissions minted fine;
only submissions landing inside the backlog-drain window fail.

Two consequences:
1. The mint-time `ogKindRun` outbox is irrelevant — the run object never
   projects, so there is nothing to re-drive.
2. This is a store-write-timeout-under-load defect, not a missing-dispatch
   obligation: the fix is either making `ReplaceLifecycleActivation` tolerate
   the load (bounded retry on `context deadline exceeded` for the activation
   commit) or backpressure / a lighter scan, not another outbox kind.

## Evidence

- `docs/evidence/s0m-stranded-bound-2026-10-02.json` — five probe runs, each
  `ok:false` with `no control packet bound to a live carrier within window`;
  trajectories `d81d1bd4`, `bc35b9d8`, `16585a04`, `b86cc432`, `5c8cf4ac`.
- `boot-timeline.json` epoch 1026: `MigrateActorWakeOutbox` minted 2009 pending;
  boot phases clean (`first_healthy` at +15 s).
- Run list sweep: 35 texture cells `pending`, 0 for probe channels; conductor
  `completed` for every probe trajectory.
- `0bdcbf61` stranded wedges auto-healed to `pending`/unbound by
  `MigrateActorWakeOutbox` on the same boot (proof the re-arm path works for the
  rows it does cover) — but never rebind (their desk cell is pending too).

## Handoff

Blocks the stranded-bound deployed acceptance (needs a live `Ask`→control→bind
on a probe trajectory) and the mechanical-resolve ask→reply→resolve leg for the
same reason. This is the residual defect of the dispatch-stall repair: the boot
re-drive does not extend to post-boot mints. Next boundary: make post-mint
`initial_dispatch` drops re-drivable outside boot (a live pending-run sweep, or a
mint-time durable obligation that does not depend on the synchronous dispatch
succeeding).

## Repair verification (2026-10-03, commit `ebfd2e98`)

Landed `fix(s0m): retry lifecycle activation commit on deadline-exceeded` in
`internal/agentcore/runtime_persistence.go`: `persistLifecycleSubmittedRun`
now wraps `ReplaceLifecycleActivation` in a bounded retry that re-attempts on
`context.DeadlineExceeded` with `context.WithoutCancel` + a fresh 60s deadline.
The commit is idempotent via `CommandDigest`, so a re-attempted mint cannot
double-submit.

**Measured on guest `ebfd2e98` (staging):** submit `ffc400c2` returned ok and,
~51s later, minted and ran desk cell `e9bdff26`, which authored `apply` on
`4461a4c9`. Previously the mint error'd `context deadline exceeded` and no cell
existed. The activation-commit starvation is repaired.

**Residual:** the minted cell completed its turn (`apply`) without calling
`activate_control` — the `Ask` never became a control packet bound to research.
The mint blocker is gone; a separate gap remains where a desk cell turn ends
without routing the control plane. Probe `s0m-stranded-bound` still times out at
"no control packet bound." Next: why a healthy desk-cell turn terminates without
emitting `activate_control` (cell prompt/steering path, not the commit store).

## Residual disambiguation (2026-10-03)

Two distinct residuals now share the "no control bound" symptom. Separate them:

1. **Steering (this probe's current blocker).** On `ffc400c2`, cell
   `e9bdff26` ran WITH the lifecycle control overlay armed
   (`lifecycle_work_item_id=4b5eb882`, `trajectory_id=ffc400c2`) yet emitted
   `op:"apply"` with zero `controls[]` — it self-answered instead of opening
   `open_researcher`. The affordance and the instruction both exist; the model
   did not follow the steer. Both `s0m_stranded_bound` and `s0m_ask` probes
   stall here. This is a desk-cell steering/faithfulness gap, not the dispatch
   substrate.
2. **Dispatch-obligation (the code defect this doc names).** The fresh-mint
   resume watchdog `armFreshMintManagementResumeWatchdog` gates on
   `isPersistentManagementAgentRun`, which is false for Texture desk cells
   (`agentprofile.Texture`, agent id `texture:<doc>`). A desk-cell mint whose
   `initial_dispatch` drops has NO re-drive — `e9bdff26` survived only because
   the guest refresh triggered boot rewarm (`actor_reactivated_from_passivated`).
   Next boundary: a dispatch watchdog for lifecycle-minted non-Management runs
   (re-send `initial_dispatch`, not a Management recovery occurrence).
