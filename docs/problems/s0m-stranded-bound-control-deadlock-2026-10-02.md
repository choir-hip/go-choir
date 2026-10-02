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
packet has remained `pending` + `delivered_to_loop_id=5e1de937` for 5+
minutes while reducer_seq advanced — stranded, never rebind, never
terminalized.

## Root cause — circular invisibility

`unbindStrandedLifecycleControls` (`management_controller.go:1131`) exists
to repair exactly this — it clears the dead run's claim so the packet
re-enters the pending set. But it is invoked **only inside the target desk's
own activation path** (`reconcileUpdatedCoagentActor` → :1833, and the
persistent-management arm :307), gated on `lifecycleAgent` (research +
`LifecycleVersion>0`).

A bound-but-unconsumed packet is excluded from `ListPendingLifecycleUpdates`
(bound rows filtered out), so it cannot generate the `coagent_result` wake
that would re-activate the desk — and the desk never re-activates on its own
for lifecycle research (`reconcileAssignedWorkItemActor` returns nil for
lifecycle research, :2789, deferring to the texture-control reconciler).
Result:

```
packet bound to dead run → excluded from pending → no wake generated →
desk never activates → unbind never runs → packet stays bound forever
```

The repair exists but is unreachable from the stranded state it is meant to
recover.

## Why this is the failure the acceptance forbids

S0m acceptance: "delivery-failure-degrades-to-scored-failure invariant holds
under run death" — a bound packet's carrier dying must leave an open, aging
record that rebinds or exhausts. Instead it wedges: bound, invisible,
un-repaired. This is the stranded-bound failure class made permanent, not
silent in the moment but permanent in the ledger.

## Fix direction

`unbindStrandedLifecycleControls` must run on a path that does not require
the target desk to already be activated — e.g. a periodic/reconcile sweep
that lists bound-pending updates for open-work research desks and clears
claims whose carrier run is terminal; or a desk-activation check that runs
the unbind before the pending gate so a bound-to-dead-run packet re-enters
the wake set. The store invariant
(`TestReconcileUpdateDeliveryRebindsStrandedAndExhaustsAtCap`) already proves
rebind + at-cap terminalization once the reconcile runs — the gap is purely
that nothing reaches the reconcile when the only trigger is the stranded
packet itself.

## Evidence

- Trajectory `0bdcbf61`, update `baab2d12`: `disposition=pending`,
  `direction=control`, `target_agent_id=research:f4e29b3f-…`,
  `delivered_to_loop_id=5e1de937-…` (completed 18:49:24).
- `management_controller.go:1124` — comment: bound rows "invisible to the
  pending list"; :1833 unbind only on lifecycle-agent activation; :2789
  lifecycle research skipped by generic work recovery.
- `adapter_test.go:1464`-family + `wakeUpdatedCoagent` (:3055) — wake only
  mints on a packet's first queue/bind, not for an already-bound stranded row.
