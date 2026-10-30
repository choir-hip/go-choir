# Off-host backup: a 55 GB Store A copy each night, and restic OOM-killed (2026-10-30)

Class: green (problem record). Host: Node B.

## Evidence [V]

- 02:40-02:43 UTC, `go-choir-offhost-backup` synced Store A into
  `/var/lib/go-choir/platform-dolt/.choir-backup/platform` (55 GB, one
  49.6 GB oldgen `.darc`). Store A's data directory is not a btrfs subvolume,
  so the script syncs a full copy instead of taking a snapshot
  (`scripts/off-host-backup.sh:233,433-436`, `is_subvolume` at :360). Store B
  is a subvolume and was captured as a read-only snapshot. The copy is not
  shared with the live store (btrfs exclusive 109 GiB for the two together).
  Root fs went from 105 GB free to 48 GB.
- 03:08:12, `restic backup failed (restic killed by signal 9 ... cgroup OOM
  kill)`; unit `MemoryMax` 12 GiB; unit state `failed`. The previous run
  (10-29 03:28) succeeded; the repo is 339 GiB of 450.
- Yesterday's cleanup had deleted the previous `.choir-backup` (52 GB), so this
  night's sync wrote a full fresh copy; whether that made restic's run larger
  is a hypothesis [I].
- 03:35, the copy was deleted by FP0 (owner: the local backup copy does not
  matter); root fs back to 102 GB free (79%). Tonight's run recreates it.

## Belief state

- The nightly 55 GB spike is structural: it recurs every night until Store A's
  data directory is a subvolume (`scripts/dolt-data-to-subvolume.sh` exists)
  or Store A is replaced by the engine move.
- The off-host copy of the keep set is one night stale (last success 10-29
  03:28). A second failure tonight makes it two.
- Cause of the restic OOM: not established; candidates are the fresh 55 GB
  copy, the 339 GiB repo index, or both. The run's own logs and a memory
  profile decide, not inference.

## Remaining error and next safe probes

- Re-run the backup by hand under observation (peak RSS of restic) with the
  copy absent, then present, to separate the two candidates.
- Converting Store A to a subvolume needs a short Store A stop (corpusd and
  the paper's publishing pause); it is a resource change the operating
  contract allows with proof and rollback, but it is Dolt work, which the
  owner has deprioritized before the engine move.
