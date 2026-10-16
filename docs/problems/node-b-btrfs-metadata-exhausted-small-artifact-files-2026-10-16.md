# Node B ran out of btrfs metadata space with 135 GB free: millions of small artifact files (2026-10-16)

Status: mitigated at 11:25 UTC by a bounded data balance. The cause is
not fixed.

## What happened

From about 11:00 to 11:25 UTC, writes on Node B's root filesystem failed
with `no space left on device`, while `df` showed 135 GB free.

`btrfs filesystem usage /` at 11:22:

| | Allocated | Used |
|---|---|---|
| Device | 475.31 GiB of 475.31 GiB (**1 MiB unallocated**) | 339 GiB |
| Data | 449.01 GiB | 314.21 GiB |
| Metadata (DUP) | 13.14 GiB | **12.63 GiB (96%)** |

Every chunk was allocated. Metadata could not get a new chunk, and the
free space sat inside data chunks, where metadata cannot use it.

## What failed (journal and paper console)

- 11:00:29 logrotate: could not rename its state file.
- 11:02:58–11:04:02, on the paper:
  - source-schedule capture writes failed: corpusd could not install
    capture bodies under `platform-artifacts/capture-bodies`;
  - 4 event-layer memberships failed: corpusd could not install a
    `computer-event-payload` blob. These are the paper's 4 "unresolved"
    in the V1 window;
  - 73 console lines in all.
- 11:04:08 Dolt (corpus, Store B): auto GC failed renaming a table file.
- 11:04:47 vmctl: could not persist the ownership registry. Its next
  write, at 11:21:43, succeeded. The file is valid JSON with 3 owners.
- 11:21:43 vmctl: the hibernate snapshot of the test computer
  `v3-events-0146` failed, so vmctl used the kill path. That computer is
  ours, minted for V3a evidence. No owner computer was affected this way.

## Why metadata filled

btrfs stores small files inline in metadata. The platform artifact store
writes one small file per object, in flat directories:

| `platform-artifacts/sha256/` directory | Files |
|---|---|
| `pin-receipts` | 3,519,356 |
| `computer-event-payload` | 1,766,674 |
| `computer-event` | 1,749,283 |
| `og` | 574,782 |

That is about 7.6 million small files. `platform-artifacts` is 106 GB.
Each computer event writes at least an event file, a payload file and a
pin receipt, so metadata grows with the event rate of every computer on
the node. Data chunks were never compacted, so the allocator ran out of
unallocated space to give metadata.

## Mitigation (11:23–11:28, owner consent for node-b operations)

Online, filtered data balances. These move only data chunks that are
mostly empty and return them as unallocated space:

1. `btrfs balance start -dusage=0 /`: no empty chunks, so nothing moved.
2. `-dusage=10,limit=10`: relocated 10 chunks, leaving 9 GiB unallocated.
   Metadata immediately grew a chunk.
3. `-dusage=20,limit=20`: relocated 20 chunks, leaving **23 GiB
   unallocated**, metadata 12.62 of 16.64 GiB (76%).

Staging stayed healthy at bc67aee10 throughout.

## Not fixed (residuals)

- **Recurrence.** Data chunk allocation will creep back toward full. The
  node needs periodic filtered balancing, for example a systemd timer
  running `btrfs balance start -dusage=20,limit=20 /` weekly, or
  btrfsmaintenance, plus an alert on unallocated space below about
  20 GiB. Usage alone (`df`) does not show this failure. This is a
  node-b NixOS configuration change, to land through the repo.
- **Growth.** About 7.6M small artifact files and rising. Each computer
  event writing three files is the substrate cause. Retention or packing
  of event payloads and pin receipts (for example, packing into larger
  blobs, or pruning receipts of reclaimed computers) is a design question
  for the artifact store, not a tuning knob.
- **The lost writes.** 4 event memberships and the paper's captures from
  11:03 are not retried by design. The next polls re-capture the items;
  the 4 captures stay without an event.
- **Dolt auto GC** for corpus failed once. Its next attempt should
  succeed; check the corpus Dolt log after the next GC.

## Recurrence guard landed

14308b42e and a2276e78c (the first run failed: `awk` was not on the
script's PATH) add `go-choir-btrfs-headroom`, a daily timer. Below
30 GiB unallocated it runs the same bounded balance at idle IO priority.
A manual run at 11:42 found 25 GiB, relocated 20 chunks in 35 s, and
left **45 GiB unallocated**, with metadata at 12.65 of 15.64 GiB. An
alert on low unallocated space and the small-file growth remain open.
