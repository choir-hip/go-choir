# actorruntime parked-handler tests flake under -race — unbound Texture owner deferral loop

**Status:** DIAGNOSED 2026-10-01 on CI shard 6 (run 36844200815, commit
`243665b4`) and reproduced locally at parent `8180c7f4` — i.e. present
without the texture read-path commit that tripped it.

## Symptom

Two tests added 2026-09-30 in `14f5682b` (parked-run recovery via
`DeliveredToRunID`) fail intermittently under `-race`:

- `TestHandlerParkedLifecycleControlReconcilesBeforeRetryAcknowledgement`
- `TestHandlerCoagentResultRecoversBoundRunFromDeliveredToRunID`

Failure signature:

```
dispatcher: deferred owner-actor-parked-handler-retry...texture:doc-... 
  cause=actor: defer unprocessed occurrence: actorruntime: Texture owner is not bound
  (deferrals=1, deferrals=2, ...)
runtime: lifecycle Research run ... remains idle:
  run projection is stale or outside the exact lifecycle Research scope
adapter_test.go:922: retry parked handler: actorruntime: execute resumed
  activation: provider admission refusal "run projection is stale or
  outside the exact lifecycle Research scope" could not passivate:
  persist lifecycle Research provider-admission passivation:
  lifecycle invalid transition
```

## Evidence

- CI run 36844200815 (commit `243665b4`, race shard 6): both tests FAIL.
- Local: `go test ./internal/actorruntime -run 'TestHandlerParked...|
  TestHandlerCoagentResult...' -race -count=3` at **`8180c7f4`** (parent of
  the suspect commit) reproduces the identical deferral-loop → refusal →
  `lifecycle invalid transition` failure. The `243665b4` diff touches
  objectgraph read paths and `frontend/src/lib/lifecycle.js` only — no
  actorruntime, dispatcher, or lifecycle-transition code — so the flake
  predates it; faster reads may merely shift the timing window.
- CI at `8180c7f4` itself passed: the failure is timing-dependent, not
  deterministic.

## Suspected mechanism

The parked-handler retry races with texture-owner binding: the deferred
occurrence keeps hitting "Texture owner is not bound", and by the time the
run reactivates its projection is stale relative to the lifecycle scope
check, so provider admission refuses and the passivation write then hits
`lifecycle invalid transition` (already-terminal state?). Whether the test
is missing a wait-for-binding step or the reactivation path genuinely
mishandles unbound-owner retries needs a real look at
`adapter_test.go:922` + `ReconcileParkedLifecycleCoagentWake` ordering.

## Next probe

Run the pair with `-race -count=10` on both `8180c7f4` and `14f5682b~1`
to confirm introduction commit; inspect whether the test drives the owner
binding before the retry tick or relies on dispatcher timing.
