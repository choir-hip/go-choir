# Other active computer loops on "invalid genesis" trajectory mints — 2026-08-24

## Evidence (node-b, journal)

The OTHER active owned computer (`candidate-fleet-d03dacaa7404b1e4412b2e6f`,
firecracker pid 3155521, guest uptime ~25k s at observation) has looped since
at least ~21:24 UTC emitting every ~3s:

```
runtime: mint trajectory <uuid> for run <uuid>: store: create object: store:
append projection batch: invalid computer event transition: invalid genesis
runtime api: submit internal run: persist agent: store: append projection batch:
invalid computer event transition: invalid genesis
```

Same trajectory UUID pattern per cycle (mint → submit → fail → retry-mint
new UUID). No process crash; a failed-run churn loop on a live computer.

## Impact

- The computer keeps minting (and failing) runs — a live failure loop on the
  fleet (the second active computer alongside the recovered 0333528).
- The failing call is the store's projection-batch append (the same
  projection substrate as the recovery work): "invalid computer event
  transition: invalid genesis" — the transition/genesis validation rejects the
  batch. Distinct from the recovery findings (no replay, no guest I/O).

## Classification

- Red: live computer runtime loop; store projection append validation.
- NOT touched in the recovery mission (kept out of the recovery scope to
  avoid a second semantic writer on a live computer).
- This receipt is the problem record (document first); a fix is separate
  follow-up work. Discovered during recovery monitoring 2026-08-24; not
  counted as a repair.

## Root cause — resolved (preflight PF-3a, 2026-08-24)

### Cause statement

The computer has an **empty canonical event chain**: no `genesis_imported`
(sequence 1) was ever CAS'd on the platform, and its embedded projection has
zero `computer_event_index` rows and no `computer_event_projection_heads` row.
Every semantic write (trajectory mint, run persist) routes through
`ComputerEventAppender.appendLocked` (`internal/computerevent/appender.go
544-550`): it resolves `platformHead` (nil for an empty chain; the corpusd head
HTTP 404 maps to nil in `http_client.go`), passes it to
`Reduce(nil, event, input)`, and `Reduce`'s nil-current branch
(`internal/computerevent/reducer.go:68-70`) unconditionally routes through
`reduceGenesis`, which rejects any non-`genesis_imported` event with
`invalid computer event transition: invalid genesis`. The run-dispatch retry
re-mints a new trajectory each cycle → the observed ~3s failed-run churn.

### Verified evidence

- **Embedded store (d03 snapshot, host reflink
  `data.img.pre-upgrade-20260824T074931Z`, opened through the v1 binary):
  `computer_event_index` COUNT = 0; `computer_event_projection_heads` = no row;
  no `genesis_imported` event; no `%bootstrap%` idempotency key.** (Owner-state
  probe via `github.com/dolthub/driver` DSN `file://...&database=texture`.)
- The store's ~153 MiB of projected state is the image-seed residue only —
  nothing was ever projected from a tape because no tape events exist.
- **Platform (node-b journal, read-only)**: d03 VM started 2026-08-23 21:10:34
  (vmctl `Running Firecracker ... candidate-fleet-d03dacaa...`); credential
  issued 21:10:33; **no `POST .../lifecycle/bootstrap-chain` call ever appears**
  in the proxy logs; firecracker pid 3155521, uptime ~25k s at 2026-08-24 noon
  — one continuous run since creation, no restart.
- `runReplayPhase` (`internal/autoputer/run.go`): `Reconstruct` on an empty
  chain is a no-op (no pages), then `gate.setPending(false)` +
  `reconcilePendingLifecycleReceipts` (none) + `rt.Start(ctx)` — **nothing
  checks that the chain has a genesis before serving**; a pre-genesis computer
  is silently operational and fails only when semantic writes arrive.

### Why "invalid genesis" and not another head error

The empty chain makes both the platform head and the embedded head nil, so
`sameHead` passes; the failure lands inside `Reduce` exactly at the nil-head
genesis branch. A non-empty-but-divergent chain would instead surface
`ErrNeedsProjectionRepair`; a duplicate genesis on a non-nil head surfaces
`duplicate genesis`. Only an empty chain produces `invalid genesis` on a
non-genesis event.

### Class and escalation

- **Not substrate-level.** The reducer's nil-head semantics are intentional and
  test-covered (`computerevent_test.go` `Reduce(nil, genesis, ...)`); the d03
  loop is two contract gaps, not a shared-code rewrite: (1) the creation path
  (external fleet harness; computer-owner key is fleet-scoped, not this
  owner's) never called `bootstrap-chain`; (2) the guest runtime boot accepts
  an empty chain and enters normal service. Both have narrow, bounded fix legs
  (no store schema change, no tape mutation, no shared-genesis rewrite) → no
  escalation under the repo dead-end rule.
- **Red**: live computer runtime; the fix leg touches the guest boot contract
  (`autoputer/run.go`) and the d03 computer's lifecycle (bootstrap + refresh,
  fleet-owner key).

### Fix legs (PF-3b, sequenced with the pre-flight flip)

1. **Repair for this computer (product path, no code):** fleet-owner
   `POST /api/computers/computer-4c20ff4a21a021c4306d8c783be0037d/lifecycle/bootstrap-chain`
   (idempotent; binds the deployed release identity), then one guest refresh so
   the boot replay materializes the genesis; the residue seed rows remain as
   the pre-genesis base (same as other fleet seed).
2. **Contract fix (compute side, red, lands in a guest image):** after a
   completed reconstruct, if the chain is still empty (and not in an explicit
   recovery/import mode), the guest must NOT enter normal service — refuse
   runtime start (or serve `pre_genesis` 503 health + refuse run dispatch)
   instead of silently accepting writes that all fail; the write-side error
   message at the mint should surface "genesis required" (misclassification of
   the nil-head case) for operators.
3. **Containment (interim, no code):** stop run submissions to d03 (fleet
   operator side) — the churn is driven purely by new submissions; the failed
   runs are inert, and the guest stays healthy apart from the error flow.
