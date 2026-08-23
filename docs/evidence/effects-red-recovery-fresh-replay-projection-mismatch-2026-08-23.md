# Fresh replay projection mismatch after checkpoint repair

Date: 2026-08-23
Mutation class: red

## Observation

After the replay-only Dolt checkpoint repair was deployed and the stale manager/warmness fence was deployed, a clean `recover_current` reached the guest without the earlier 30-second/10-minute replay timeout. The guest transferred approximately 566 MB of replay data and spent about nine minutes in local replay, then failed with:

```text
autoputer: reconstruct computer event authority: computer event projection repair required
```

This occurred in a single fresh realization after the old VM was fenced; no concurrent e15cb Firecracker process was present at recovery start. The failure is therefore distinct from the previously documented checkpoint bottleneck and stale-manager race.

## Belief state

`ComputerEventAppender` completed its source replay path without returning a page, prepare, or context error, but its final local projection head did not compare equal to the corpusd head. The exact head fields are not yet recorded; the next repair must expose local/platform sequence and head values rather than widening timeouts again.

## Root-cause clustering

The recovery subsystem has now produced three substrate-level failures in one mission: per-event Dolt checkpoint cost, stale VM/warmness fencing, and a fresh-replay projection mismatch. Further symptom patches are unsafe until the final head fields identify whether the defect is reducer/input reconstruction, replay page metadata, or local projection persistence.

## Next safe probe

Add bounded diagnostic fields to the projection-repair error (sequence and canonical/desired/effective/state commitments only), reproduce once with the recovery marker fencing warmness, and choose the repair from that evidence. Do not rewind corpusd events or accept the recovery as active.
