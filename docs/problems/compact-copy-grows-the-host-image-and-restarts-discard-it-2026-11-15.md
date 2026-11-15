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
