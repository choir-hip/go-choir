# CI flake: pre-bind research-recovery delivery row intermittently absent

**Status:** RESOLVED 2026-10-04 by `e87f3294`. Root cause confirmed as a stale
post-S0m assertion racing the stranded-bound release, not a store defect.

**Mutation class of this record:** green. The fix is orange (test + event
payload fields; no runtime authority change).

## Symptom

`internal/actorruntime`: `TestAdapterSQLitePreBindResearchRecoveryBindsAndExecutesWithoutSnapshot`
failed ~80% on `main` (`go test -count=15` → 12 FAIL) and on both legs of CI
run 37181074660:

```
adapter_test.go:2574: pre-bind recovery delivery=[] err=<nil>
```

The run completed (`RunCompleted`, `counting.calls == 1`) while
`ListLifecycleControlsDeliveredToRun(rec.RunID)` returned empty.

## Root cause

S0m's stranded-bound release (`c9180cd3`,
`internal/agentcore/research_checkpoint_fallback.go:26-60`): a lifecycle
research run that terminalizes **unconsumed** releases its bound-but-unconsumed
control claims back to `deliveredTo="" disp=pending` inside
`bindTerminalRunOutcome`, then re-drives the desk so the packet rebinds and
consumes on the next carrier.

The test asserted the **live** `DeliveredToRunID` claim after run completion.
That claim is correctly transient post-S0m: it exists only for the duration of
the carrier's activation and is released at terminalize. The assertion raced
the release commit — PASS when the assert read the row first (~20%), FAIL when
the release landed first (~80%).

Instrumented evidence (`e87f3294` debugging, later removed): on every FAIL run
`admitLifecycleResearchProviderEntry` returned `lifecycle=true admitted=true`
and the run carried `lifecycle_control_bindings` metadata — the bind *had*
committed and been released, exactly as designed.

## Fix

`e87f3294`:

1. Re-pinned the assertion to the durable `control_delivered` lifecycle event
   (`ListLifecycleEvents`, `Kind==LifecycleControlDelivered`, `RunID==rec.RunID`),
   which is emitted inside the bind commit and survives the release.
2. Added `RunID` + `AgentID` to the `control_delivered` event
   (`lifecycle_control_delivery.go`) and to the `ReconcileUpdateDelivery` emit
   (`lifecycle_update_delivery.go`) so the durable receipt names its bound run
   directly rather than only via CommandID.

`go test -count=12` on the fix: 12/12 PASS; full `TestAdapterSQLite*` shard
green 3x.

## What this was NOT

- Not a bind-write race: the bind committed every time (proven by the
  `lifecycle_control_bindings` metadata + admission `lifecycle=true`).
- Not a regression from `86d79c63`: the test failed identically on clean
  `main` at `55df9418`.
- Not a texture-owner gap: the `Texture owner is not bound` deferrals in logs
  are expected noise — the test never binds a texture owner.

## Rollback

None — test + event-field-only change. The event fields are additive; no
consumer reads them yet.
