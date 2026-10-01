# Stranded passivated run — wake may be consumed before the run drains to it

**Status:** OBSERVED 2026-10-01 on staging (build `4a718af4`, guest
`computer-03335285269bdba4f94377e56879f9e6`). Recorded per
Problem-Documentation-First. This is the residual defect the parked-run
repair (`14f5682b`) was meant to close; the capacity fix removed the OOM
amplifier but not the ordering gap.

## Observed

- Run `362febb2-c58f-…` (research trajectory `ba6199f1-…`, agent
  `research:cfa90b87-…`) **passivated at boot** 2026-09-30 17:41:37
  (`runtime: passivated run … after restart`). It had first failed at 17:37
  on `resolve head ... connection refused` (the OOM window).
- The bound wake for `research:cfa90b87` is a `result:sha256:…` row in the
  actor wake outbox.
- **The serial drain disposed 496 `disposed dead wake` rows**, then went quiet
  (0 disposals in the last 15min of observation) — the backlog is cleared.
- `362febb2` never produced a reactivation line; it no longer appears in
  `run list` (dropped to a terminal/passivated boundary state, not
  enumerable as active).
- Contrast case: run `50581094-…` reactivated cleanly at 05:14 from
  `state pending` on `coagent_result` — the reconcile path works for a
  *pending* run.

## Hypothesis (edge: missing_oracle)

`ReconcileParkedLifecycleCoagentWake` reactivates an exact actor-memory run
when a wake arrives for an already-parked run. For `362febb2` the wake appears
to have been **enqueued, then disposed as a dead wake during an OOM window
before the run's passivated state was visible to the reconciler** — leaving the
run stranded with no second wake generated. Pending-vs-passivated reactivation
is asymmetric in the code/tests: `pending` reactivates on result arrival;
`passivated` may require the wake to still be live when the reconciler runs.

**Unverified:** whether the wake was (a) consumed-and-dropped while the run
was passivated, (b) never enqueued for this run shape, or (c) enqueued but
its reconcile early-exited. The wake table (Dolt) is required — journald only
shows the disposal of the sha-keyed result, not which run/agent it bound.

## What this is NOT

- Not the OOM bug itself — the cap is holding (OOMKills=0, RSS ~12GiB); this
  is the *next-order* defect that survives under a stable substrate.
- Not proof the repair failed — the repair removed the amplifier; this names
  the residual ordering edge it was scoped to expose.

## Probe (smallest safe next step)

Query the wake/outbox table for the `research:cfa90b87` result row bound to
run `362febb2` — `disposed dead` vs `pending` vs `absent` decides which of
the three hypotheses. If consumed-while-passivated, the fix is to reconcile
passivated runs against *consumed* wakes (not just live ones), or to defer
dead-wake disposal until the bound run confirms terminal — closing the gap the
repair was designed to close.
