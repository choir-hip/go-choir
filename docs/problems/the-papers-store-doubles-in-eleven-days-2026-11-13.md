# The paper's store doubles in eleven days

Recorded 2026-11-13T23:55Z. Status: measured, cause not known, under
investigation (`lane/store-size`).

## Problem

The paper's guest database, `/mnt/persistent/state.db`, is 43.76 GiB and
grows about 2 GiB a day. Nothing bounds it. It is the largest single object
of state on the platform and it drives three other costs: the paper's data
image on the host (51.9 GiB), the size and duration of every snapshot
(46.7 GB, 37 min 40 s at 22:36Z), and the chunks each snapshot adds (4.8 GB
sealed).

## Evidence

- `data_volume` on the paper, 2026-11-13T23:41:54Z: `state.db` 43.76 GiB,
  `choir-updater` 4.31 GiB (all `releases`), `state-actor.db` 1.37 GiB,
  everything else under 0.3 GiB. Volume 125.7 GiB, 69.15 GiB available.
- `store.split.summary` on the paper: 22,382,874,624 bytes at the split of
  2026-11-02T04:13Z, with 3,411,969 `og_objects` rows and 2,735,246 rows in
  other tables.
- d@e's `state.db` is 1.03 GiB.
- Node B at 23:50Z, `btrfs filesystem du`: 312 GiB used. Tape segments 89.4,
  the paper's VM directory 51.9, d@e's 29.2, receipts and the rest of
  Postgres 34.5, file and snapshot chunks 33.2, capture bodies 22.9, a
  `candidate-fleet-bf04…` VM directory 19.2 last written on 11-10, the nix
  store 17 to 29.

## Belief

The store keeps every projected row forever: run-memory entries of finished
runs, telemetry, and object-graph objects for every event. The same events
are held three times: as tape segments on the host, as receipts in Postgres,
and as projected rows in the guest. Unverified. Which tables hold the bytes
is not measured; there is no per-table reading.

## Remaining error

- Composition of the 43.76 GiB by table and index: unknown.
- How much is free pages SQLite never returns: unknown.
- Whether the 19.2 GiB `candidate-fleet` directory is dead: unknown
  (separate host survey running).

## Next

A per-table reading in the observability endpoint if it can be bounded, then
bounds ranked by bytes saved per unit of risk
(`docs/design/bounding-the-guest-store-2026-11-14.md`, to be written).
