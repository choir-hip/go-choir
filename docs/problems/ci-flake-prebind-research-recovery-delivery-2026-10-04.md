# CI flake: pre-bind research-recovery delivery row intermittently absent

**Status:** confirmed — intermittent; blocked the `86d79c63` deploy run on
2026-10-04. Owner: the S0m record-native lane (delivery/incorporate ledger).

**Mutation class of this record:** green.
**Flake rate:** ~80% — `go test -count=15` on `main` produced 12 `delivery=[]`
FAILs, 3 PASS. CI run 37181074660 failed on both the initial and the `--failed`
rerun at the same assertion.

## Symptom

`internal/actorruntime`: `TestAdapterSQLitePreBindResearchRecoveryBindsAndExecutesWithoutSnapshot`
fails ~half the time on `main` at `55df9418` (and in CI run 37181074660):

```
adapter_test.go:2574: pre-bind recovery delivery=[] err=<nil>
```

The run completes (`RunCompleted`, `counting.calls == 1`); the assertion that
fails is `ListLifecycleControlsDeliveredToRun(...) == 1` with
`DeliveredToRunID == rec.RunID` — the delivered control packet for the
seeded research control never appears in the page, even after a 5s poll that
already waits for run completion.

## Why it is not a simple ordering race

The delivered-page query (`internal/store/lifecycle_control_delivery.go:628-770`)
is a store-integrity filter: packets are skipped/errored on mismatch
(disposition, DeliveredAt, target work binding, etc.). A packet that lands
with a different binding field — not merely late — produces `delivery=[]`
permanently. Repeated `go test -count=10` on `main` shows both PASS and the
same `delivery=[]` FAIL, i.e. the write itself is non-deterministic across
## Trace evidence (2026-10-04 second investigation pass)

PASS runs show one `defer unprocessed occurrence: actorruntime: Texture owner
is not bound` deferral then succeed; FAIL runs show the same deferral repeated
`deferrals=1..4` until the 5s deadline. The Texture-owner deferral is unrelated
noise — this test never binds a texture owner (`New(cfg, s, bus, counting, nil)`
with no `BindTextureOwner`), so its occurrences defer by design. The actual
failure is the research-side bind: the run completes (`RunCompleted`,
`calls==1`) while `ListLifecycleControlsDeliveredToRun(rec.RunID)` stays empty.

The run's own `admitLifecycleResearchProviderEntry` binds the pending control
when `deliveryMode == "pending"` before provider entry
(`runtime.go:3113-3121`). If the row never lands, either the run was admitted
on the `delivered` arm with `DeliveredToRunID` pointing at a *different* run
record, or the pending-control rebind path raced the run admission. Commit
`23c461b5` ("K: delete restart-resumption sweeps + process-local progress
continuations", owner ruling "boot causes no work") deleted
`sweep_pending_update_actors` and `rewarm_lifecycle_activations`; the pending
control's rebind now depends on whichever wake path reaches it first, making
the bind target racy across restart.

## Suspected cause / next step

Dump the run set on failure: list every `RunRecord` for the research agent
and check whether a second `RunPending`/`RunRunning` was created on restart
that captured the bind (leaving `rec.RunID` undelivered). If so, the seeder's
`failBind` arm leaves a stale pending run that competes with `rec`; the fix
is to assert delivery on the bound run, or to close the duplicate-pending-run
window in the restart path.

Plausible substrate: S0m's record-native consume/incorporate changes
(`d61c9b1b` consume-marking, `c9180cd3` stranded-bound rebind) changed how a
bound control's delivery is recorded; the test's `true` argument to
`seedAdapterLifecycleResearchControl` selects a path whose delivery row may
now be folded rather than written as a `DeliveredToRunID` packet.

## Evidence

- CI run `37181074660` (shard 6), commit `86d79c63`: FAIL, `delivery=[]`.
- Local `go test -count=10` on `55df9418`: mixed PASS / FAIL `delivery=[]`.
- Local single-run `-count=2`: PASS (flake ~40-60%).

## Suspected cause / next step

Compare `seedAdapterLifecycleResearchControl`'s seeded packet fields against
`ListLifecycleControlsDeliveredToRunPage`'s required invariants (trajectory,
target agent, DeliveredToRunID, Disposition, work binding). If S0m changed
which fields the delivered packet carries, the test's seed or assertion must
be re-pinned to the post-S0m record shape; if the delivery write itself is
racing, the write path (rebind carrier vs direct delivery) needs an ordering
guarantee before the test asserts.

## Rollback

None — record only. The failing run's deploy jobs rerun on the next push;
the flake does not block product behavior, only CI deploy progression.
