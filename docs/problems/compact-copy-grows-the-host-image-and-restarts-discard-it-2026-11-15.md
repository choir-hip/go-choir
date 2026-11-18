# The paper's compact copy grows the host image, and a restart discards it

Recorded 2026-11-15T01:35Z from the hourly status read. Read-only
investigation; nothing was changed.

## Problem

The hourly status flagged Node B free space falling 15.7 GiB in six hours
and about 40 GiB in the last hour (free 133 GiB at 01:28Z). The grower is
the paper's data image: +32.1 GiB allocated in 24 hours.

## Evidence

- Host: the paper's `data.img` is 131,072 MiB apparent, 88,547 MiB
  allocated. Guest `statfs` on the same volume: 58.3 GiB used. About 30 GiB
  of host blocks were written once by the guest, freed inside the guest and
  never returned to the host.
- Guest data volume at 01:28Z: `state.db` 46.6 GB,
  `state.db.compact-staging` 9.1 GB and growing.
- Paper console: at 01:23:32Z "store: compact copy: due by freelist,
  1126246 of 11378262 pages free, live ~41992257536 bytes, copying". An
  earlier copy had reached "checkpoint after compact copy" at 01:01:14Z and
  was in `checkCompactCopy` when the 00:54Z deploy walk restarted the paper
  at 01:03Z. The boot's `compact-rebase` phase completed in 18 ms, and the
  copy started again from zero twenty minutes later.
- `store_stalled holder ... vacuumIntoCopy` and `... checkCompactCopy` lines
  appear during the copy.

## Belief

1. Run-memory retirement is now freeing pages (10% of the store is on the
   freelist), so the compact copy of `fd6ca5b2d` is due. This is the store
   starting to shrink, which is what the storage mission wanted.
2. Each copy writes about 42 GB of new guest blocks. The guest volume has no
   discard path to the host, so the host image grows by up to that much and
   keeps it. The growth is bounded by the image size: at most 42.5 GiB more,
   which leaves Node B about 93 GiB free. Not an emergency.
3. A copy takes longer than the gap between deploys while everything is
   expedited. A restart before the copy is verified discards it, so the paper
   can write 42 GB per deploy and never shrink. Not yet confirmed from code:
   whether the 01:03Z boot discarded a finished copy or an unverified one.

## Remaining error

- Whether the planned-restart marker lets a boot adopt a copy that was
  complete but not yet checked, and whether the copy can resume.
- Whether the store stall during the copy slows the paper's appends (the
  `store_stalled` lines name the copy as holder).
- The host never reclaims guest-freed blocks (no discard or trim on the data
  image). After a successful compact the guest frees about 46 GB that the
  host will still hold.
- The hourly status reads this as an unexplained "fast drain"; it should name
  a running compact copy.

## Update 2026-11-18: the swing is regular, the host gets the space back, and its size tracks the store

The hourly status flagged a fast drain again at 05:38Z on 2026-11-18 (free
154 GiB to 102 GiB), recovered by the next read. The lead first wrote a
second record for it without reading this one; that record is deleted and
its evidence is here.

Growth meter ledger on Node B (hourly rows, read only), every hour-to-hour
change of more than 8 GiB in the paper's `data.img` in November:

| Up (UTC) | Allocated, GiB | Back down | Free at the peak, GiB |
| --- | --- | --- | --- |
| 11-01 04:00 | 23.2 to 42.2 | 06:01 | 203 |
| 11-02 05:01 | 23.8 to 44.6 | 11-03 06:01 | 168 |
| 11-09 05:00 | 37.6 to 70.5 | 06:00, up again 07:00, down 10:02 | 68 |
| 11-15 01:00 | 49.4 to 84.2 | 04:00 | 135 |
| 11-16 01:01 | 43.2 to 82.1 | 02:00 | 145 |
| 11-16 19:00 | 44.8 to 85.8 | 20:00 | 125 |
| 11-17 06:01 | 46.1 to 87.4 | 08:00 | 122 |
| 11-17 17:01 | 47.4 to 90.5 | 18:01 | 118 |
| 11-18 05:00 | 48.8 to 93.2 | 06:01 | 103 |

What this changes in the record above:

- **The host now gets the blocks back.** Each swing returns to the resting
  size within one to three hours, so the "never reclaims" item under
  remaining error no longer holds as written. Not checked: which change
  added the discard path.
- **The copy completes.** Since the 16th every swing is up for one or two
  hours and down again, once or twice a day; belief 3 (a deploy restarts it
  before it finishes, forever) is not what the ledger shows this week.
- **The space it needs is the size of the store, and the store is growing.**
  The resting size went from 23 GiB on the 1st to 49 GiB on the 18th. Each
  copy needs that much free at once. On the 9th free space fell to 68 GiB
  during one. The forecast unit reads daily minima and says "not declining";
  the fast-drain warning fires and clears; neither says a swing the size of
  the store is due.
- Not confirmed for the 18th: that the 05:00Z swing was a compact copy (the
  console lines for that hour were not read; the paper's snapshot root is
  stamped 05:05:13Z, which may be coincidence). The pattern and the ratio
  (1.8 to 1.9) match the copies recorded above.
- Still open from above: the hourly status should name a running compact
  copy instead of an unexplained fast drain.
