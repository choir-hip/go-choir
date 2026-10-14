# Node B's Artifact Store Grows About 12.6 GB a Day; Garbage Collection Never Runs

Date: 2026-10-14, about 12:00 UTC. Found while sizing the performance
report. Status: documented; not fixed. Owner: "That much storage growth?
Another storage leak to fix huh."

## Evidence (read-only, Node B)

- Disk: `/dev/md127` 476 GB, 374 GB used (80%), 98 GB free. It was 67% on
  the morning of 10-14 per the cost-track investigation.
- Largest homes: `/nix/store` 168 GB; `/var/lib/go-choir/platform-artifacts`
  125 GB (of which `sha256/` 119 GB, `checkpoint-scratch/` 6.6 GB,
  `capture-bodies/` 1.2 GB); `vm-state` 110 GB (guest data disks and
  snapshots); `platform-dolt` 23 GB; `corpus-dolt` 22 GB (the corpus was
  torn down on 10-09 by owner decision, but its data remains).
- `platform-artifacts/sha256`: files written in the last 24 h: 1,182,580
  files, 12.6 GB; last 7 days: 1,944,690 files, 61.5 GB. The largest objects
  are `projection-base/*` at 8.7-15.9 GB each (10-07, 10-08, 10-09, 09-09).
- Artifact GC runs in dry-run mode only (`nix/node-b.nix`,
  `PLATFORM_ARTIFACT_GC_MODE`, noted in the cost-track investigation).
- At about 12.6 GB a day, the 98 GB left lasts roughly 1-2 weeks, less on
  heavy deploy days.

## Also found

- Snapshot memory files are compressed by btrfs (zstd:3): about 560 MB on
  disk per 8 GiB snapshot (compsize: 6%). The snapshot budget
  (`VM_SNAPSHOT_BUDGET_GIB=48`) counts logical bytes, so it admits about 6
  where real disk would allow about 85.
- Snapshots of computers that were later stopped stay on disk
  (`vm-0f24d3b8...`, `vm-db4bb41c...`: both computers stopped at 08:30).

## To determine before fixing

What writes 1.2 million objects a day (checkpoints? per-boot projection
bases? capture bodies?), which of them anything still references, the
retention each kind needs (restore points, rollback refs), and why GC was
left in dry-run. Old NixOS generations and the corpus database are
separate, simpler cleanups.
