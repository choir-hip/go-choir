# Node B's host slab held 9.75 GiB, 5.9 GiB of it btrfs inodes (2026-10-22)

Status: documented, not fixed. Checklist item RES-22 (P2, red), density
lever L19 (`docs/reports/vm-density-limits-2026-10-14.md`, finding 4).
The btrfs metadata half of RES-22 has its own record:
[node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md](node-b-btrfs-metadata-exhausted-small-artifact-files-2026-10-16.md).
Design: `docs/design/fp10-snapshot-levers-and-host-slab-2026-10-22.md`.

## Evidence

The only reading is the density report's, Node B, 2026-10-14 about 15:00
UTC: total slab 9.75 GiB, 8.4 GiB of it reclaimable, `btrfs_inode`
5.9 GiB. Node B has 31.1 GiB. The slab is reclaimable, but it competes
with guests for the same memory, and reclaiming it is direct reclaim
work on the paths that boot and wake guests.

## Hypotheses (not findings; no trace confirms them)

1. **The tape's file count.** The platform artifact store held about
   7.6 million small files on 2026-10-16, growing about 1.5 million a day
   (two receipts and two artifacts per event). Any walk of those
   directories fills the inode cache.
2. **The walkers.** Candidates that walk large trees on a schedule: the
   off-host backup (`go-choir-offhost-backup`, 02:30 UTC daily; restic
   walks the whole tape), the hourly artifact GC (it skips the tape
   namespaces but walks `og` and the chunk store), the disk-retention
   sweep every four hours, and ad hoc `du` reads.
3. **The kernel's default.** `vm.vfs_cache_pressure` is unset (100), so
   the kernel reclaims inodes and page cache at the same rate.

## What changed since

Tape segments (A2) are on for every computer (`PLATFORM_TAPE_SEGMENTS=write`,
`docs/definitions/choir-tape-segments-a2-repack-2026-10-21.md`): new
projection-batch pins go to per-computer segments instead of one file
each. If that works, the file count stops growing. The legacy files stay
until the repack, which is not built. No reading since the flag went on
shows whether the growth stopped.

## Not known

- Today's slab size and its top caches.
- Whether slab rises at the backup or GC window.
- How fast the tape's flat directories grow now.

The read-only commands are in the design, section 4.
