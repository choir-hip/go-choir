# S0m — stranded-bound control packet deadlocks its target desk

**Status:** confirmed on staging (deployed proof), 2026-10-02
**Found:** during S0m finish-acceptance "stranded-bound rebind under run death"
probe; surfaced as a live trajectory stalling with a bound-and-invisible packet
**Mutation class:** red — desk delivery/recovery correctness

## Symptom

On staging trajectory `0bdcbf61-9af0-5b49-b80f-f698e8df7fd5` (owner computer
`computer-03335285269bdba4f94377e56879f9e6`), control packet
`baab2d12-1fdf-4a4a-9f88-cba25d97b273` (texture→research ask) bound to
research run `5e1de937-c512-44e9-898c-38f5184f4736` at 18:40:38 and was
delivered, but the run **completed** at 18:49:24 without consuming it. The
packet stayed `pending` + `delivered_to_loop_id=5e1de937` for 5+ minutes —
stranded, never rebound, never terminalized.

## Root cause — circular invisibility

`unbindStrandedLifecycleControls` (`management_controller.go:1131`) exists
to repair exactly this — it clears the dead run's claim so the packet
re-enters the pending set. But it is invoked **only inside the target desk's
own activation path** (`reconcileUpdatedCoagentActor`, and the
persistent-management arm), gated on `lifecycleAgent` (research +
`LifecycleVersion>0`).

A bound-but-unconsumed packet is excluded from `ListPendingLifecycleUpdates`
(bound rows filtered out), so it cannot generate the `coagent_result` wake
that would re-activate the desk — and the desk never re-activates on its own
for lifecycle research (`reconcileAssignedWorkItemActor` returns nil for
lifecycle research, deferring to the texture-control reconciler).
Result:

```
packet bound to dead run → excluded from pending → no wake generated →
desk never activates → unbind never runs → packet stays bound forever
```

The repair exists but is unreachable from the stranded state it is meant to
recover.

## Cluster — this is the dominant failure, not an edge

The same trajectory carries **9+ stranded control packets**, each bound to a
now-`completed` research carrier (`447d4f92`, `0c7b4052`, `c003159f`,
`ae168dd3`, `d8b59af4`, `7dcbdcfd`, `5e1de937`, `e9df05c2`, `10d79130`,
`94d2f45b` → 10 distinct dead carriers). Every texture→research ask whose
carrier terminalized before consuming left a permanent wedge. Per the
Root-Cause-Clustering rule this is substrate-level: the run-terminalization
event never released its held claims, so *any* carrier death strands the
packet — not a coincidental path.

Pre-existing wedges do not retro-heal under the fix (their carriers already
terminalized before the release hook existed). The fix stops new wedges at
the terminalization event.

## Why this is the failure the acceptance forbids

S0m acceptance: "delivery-failure-degrades-to-scored-failure invariant holds
under run death" — a bound packet's carrier dying must leave an open, aging
record that rebinds or exhausts. Instead it wedges: bound, invisible,
un-repaired. This is the stranded-bound failure class made permanent, not
silent in the moment but permanent in the ledger.

## Fix direction

Landed in `c9180cd3`: `bindTerminalRunOutcome` — the hook every
run-termination path already funnels through — now calls
`unbindStrandedLifecycleControls` on a terminal lifecycle research/management
run before the terminal-outcome binding, then re-drives the desk via
`wakeUpdatedCoagent` for each freed packet so it wakes, rebinds through
`bindLifecycleControlsToRun`, and consumes; past the cap it terminalizes
`delivery_attempts_exhausted` (scored, never silent). Regression:
`TestLifecycleRunTerminalizeReleasesStrandedControlAndRewakesDesk`.

## Evidence

- Trajectory `0bdcbf61`, update `baab2d12`: `disposition=pending`,
  `direction=control`, `target_agent_id=research:f4e29b3f-…`,
  `delivered_to_loop_id=5e1de937-…` (completed 18:49:24); plus 9 further
  stranded controls on dead carriers (`447d4f92`, `0c7b4052`, `c003159f`,
  `ae168dd3`, `d8b59af4`, `7dcbdcfd`, `e9df05c2`, `10d79130`, `94d2f45b`).
- `management_controller.go:1124` — comment: bound rows "invisible to the
  pending list"; unbind previously ran only on lifecycle-agent activation;
  lifecycle research is skipped by generic work recovery.
- `wakeUpdatedCoagent` — wake only mints on a packet's first queue/bind, not
  for an already-bound stranded row.
- `adapter_test.go:1464`-family — wake mints on a packet's first queue/bind.
