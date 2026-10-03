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

## Mechanism (hypothesis from code + observation)

`Runtime.activate` dispatches `initial_dispatch` synchronously at mint; a drop
leaves the fresh run `pending` with no live trigger. The boot repair added an
`ogKindRun` outbox + `MigrateActorWakeOutbox` re-drive, but that sweep runs at
boot and drains the pre-existing backlog serially (`startProjector` sweeps every
500 ms, each dispatch is a network CAS). Two gaps remain:

1. **Post-boot mints have no outbox row** if the `initial_dispatch` obligation
   was not derivable at mint for this desk-cell shape — they are invisible to
   both `MigrateActorWakeOutbox` (already ran) and the live projector.
2. **Pre-existing pending desk cells are queued behind a 2009-row backlog** and,
   where their backing `ActorOccurrence` was consumed or terminalized before the
   wake projected, are `disposed dead wake` (mark-projected) rather than
   re-driven — the run stays `pending` with no re-dispatch authority.

The desk-cell mint for the probe trajectory appears to drop its `initial_dispatch`
entirely (no pending `texture:` row in the run list at all) — consistent with a
mint-time store write that fails before the run object projects, or a
`prompt-bar`→conductor→desk handoff that completes the conductor without minting
the desk cell. Either way the trajectory is left `live` with `pending_updates: 0`
and no desk.

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
