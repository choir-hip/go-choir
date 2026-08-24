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

## Next (separate task)

1. Root-cause the genesis validation path (store.GetVerfAttest... the
   `validateGenesis`/transition checker at the batch append; the run's
   trajectory mint the first time the guest runtime started after the
   projection).
2. Determine WHY the batch append rejects: the base genesis record version vs
   the store's genesisG0/G1 receipts (the self-development genesis
   receipts may mismatch the computer's ancestors).
3. Fix + landing loop (red-class ceremony; the computer is LIVE — the fix
   goes through the guest image rebuild; the loop's failed runs are inert).
