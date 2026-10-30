# platform-dolt's oldgen grows 1.5 to 2.5 GiB an hour and nothing collects it (2026-10-22)

Date: 2026-10-22, 15:00 UTC. Found by: FP0, read-only on Node B, while
measuring [node-b-disk-fills-from-the-paper-data-image-2026-10-22.md](../../problems/node-b-disk-fills-from-the-paper-data-image-2026-10-22.md).
Status: open, problem record before any fix. Mutation class of a fix: red
(Store A is the platform's canonical event and control store, vmctl's route
database and the checkpoint source).

## Evidence (FP0, read-only, 14:50 to 15:00 UTC)

- `/var/lib/go-choir/platform-dolt` is 36 GiB, almost all
  `platform/.dolt/noms/oldgen/*.darc`.
- Auto GC runs about every 30 minutes and each run leaves a new oldgen
  archive: a 25.7 GB file from 11:12, a 6.2 GB file at 14:54.
- So oldgen grows about 1.5 to 2.5 GiB an hour (about 45 MiB a minute of the
  host's 200 MiB/min fall). Nothing removes oldgen files.
- The 2026-10-15 clustering assessment measured +1.6 GB a **day** of oldgen
  ([dolt-clustering-assessment-2026-10-15.md](dolt-clustering-assessment-2026-10-15.md));
  today's rate is roughly 25 to 35 times that. Why the rate rose is not
  established (see Open questions).

## How the host Dolt server runs (read in nix/node-b.nix)

- `go-choir-platform-dolt`: `dolt sql-server --host 127.0.0.1 --port 13306`
  in `/var/lib/go-choir/platform-dolt/platform`, no `--config`. The server
  binary is `pkgs.dolt` with the flake's overlay, **Dolt 2.4.2**
  (`flake.nix` `choirDoltRelease`).
- With no `--config` the server takes its settings from the command line
  only. It writes a `config.yaml` into the data dir if none exists
  (`generateYamlConfigIfNone`) but does not read one back. So the settings
  are the defaults: `behavior.auto_gc_behavior.enable` true
  (`servercfg.DefaultAutoGCBehaviorEnable`), `archive_level` 1 (`.darc`
  archives), `incremental_file_size` 0.
- Store A commits its working set about every 45 s (`internal/doltbatch`,
  `CALL DOLT_COMMIT('-Am', …)`;
  [store-a-weekly-squash-2026-10-20.md](store-a-weekly-squash-2026-10-20.md)).
- The daily `go-choir-platform-dolt-history-audit` fails the unit when
  oldgen exceeds 50 GiB (`GO_CHOIR_DOLT_MAX_OLDGEN_KIB`) or the log exceeds
  1 M commits. It is a signal; nothing consumes it.

## What auto GC does to oldgen (read in Dolt's code, not guessed)

Read in the Go module cache, `github.com/dolthub/dolt/go`
v0.40.5-0.20260716231223-6bea93f2f665, which is **Dolt 2.2.0**
(`cmd/dolt/doltversion/version.go`). The server is 2.4.2; the GC code paths
below are the ones 2.2.0 has, and a change between 2.2.0 and 2.4.2 is not
ruled out (named residual).

- Auto GC (`libraries/doltcore/sqle/auto_gc.go`) calls
  `RunDoltGC(…, chunks.NewGCConfig(chunks.GCMode_Default, arcLevel, incrementalFileSize))`.
  It is requested by a commit hook when the journal exceeds 128 MiB, or,
  after a first GC, when the store has grown by more than the newgen size
  at the end of the last GC and as much time has passed as the last GC took
  (`shouldRequestGC`).
- `DoltDB.GC` (`libraries/doltcore/doltdb/doltdb.go:1967`) puts every
  branch, remote and internal ref in the **oldgen** set and the working sets
  in the **newgen** set.
- `ValueStore.GC` (`store/types/value_store.go:568`), mode Default:
  1. walks every chunk reachable from the branch refs, filtered by
     `OldGenGCFilter` (chunks already in oldgen are skipped), and
     `oldGenFinalizer.AddChunksToStore` **adds** them to oldgen as one new
     table file (the new `.darc` FP0 saw at each run);
  2. rewrites newgen with only what the working sets still need
     (`newGenFinalizer.SwapChunksInStore`);
  3. only `if gcConfig.Mode == chunks.GCMode_Full` does it call
     `oldGenFinalizer.SwapChunksInStore`, which replaces oldgen's file set.
  `GenerationalNBS.BeginGC` says the same in its comment: "In normal
  operation, the OldGen is not being collected because it is still growing
  monotonically and nothing in it is at risk of going away."
- So a Default GC never removes anything from oldgen. Every chunk that was
  ever part of a commit stays, and with a commit every 45 s every superseded
  row version is part of some commit.
- `--shallow` (`DoltDB.ShallowGC`) only prunes table files that no manifest
  references.

## What would collect it

1. **A full GC alone does not.** `GCMode_Full` rewrites oldgen with only the
   chunks reachable from the refs, but every commit since the last squash is
   reachable, so it keeps the history it would need to drop. This is the
   2026-08-26 finding
   ([platform-dolt-oldgen-218g-dead-history-2026-08-26.md](../evidence/platform-dolt-oldgen-218g-dead-history-2026-08-26.md)):
   a full GC "would have walked the 218G, rewritten all reachable history,
   reclaimed nothing". It reclaims only chunks no commit reaches (for
   example after an earlier squash).
2. **A history squash, then a full GC.** The squash makes the old commits
   unreachable; the full GC then writes a new oldgen holding only the live
   set and swaps it in. This is the built, never-run tool of
   [store-a-weekly-squash-2026-10-20.md](store-a-weekly-squash-2026-10-20.md)
   (`scripts/platform-dolt-squash.sh`, unit `go-choir-platform-dolt-squash`,
   `platformDoltSquashTimerEnabled = false`): offline, service stopped,
   reflink backup first, root hash identical before and after, bounded GC
   flags (`--archive-level 0 --incremental-file-size 268435456`) under a
   memory cap.

Cost and lock behaviour of a full GC, as read in the code:

- **Disk:** the full GC writes a new copy of every reachable chunk before it
  swaps the file sets, so it needs free space about the size of the
  reachable store (after a squash, the live set) on top of the old files,
  until the swap. The squash tool's preflight asks for at least 50% of the
  store size and at least 10 GiB free (its FM8).
- **Memory:** the paper's bounded full GC took 6.1 GB and 2 min 19 s on a
  52 GB store (store-a-weekly-squash record); unbounded it did not finish in
  13 GB plus 15 GB of swap. The server unit has `MemoryHigh=3G`,
  `MemorySwapMax=0`, so an online full GC inside it would be throttled.
- **Locks (online):** `dprocedures/dolt_gc.go` uses the session-aware
  safepoint controller by default (`UseSessionAwareSafepointController`,
  env `DOLT_GC_SAFEPOINT_CONTROLLER_CHOICE=kill_connections` switches to the
  old one). It does not kill connections; at each safepoint it waits for each
  session's in-flight command to end and visits its in-memory roots, and a
  session starting a command while its roots are being visited blocks until
  that visit ends (`doltdb/gcctx/gc_safepoint_controller.go`). A full GC
  takes read dependencies on oldgen as well as newgen
  (`GenerationalNBS.BeginGC`). On 2026-10-15 an online server stalled for
  40 minutes after a conjoin plus auto GC; the squash design rejects an
  online run for that reason and because the root-hash check needs a quiet
  store.
- **Offline (the built tool):** the store is closed, so there is no lock
  contention; the cost is the window in which Store A and its `Requires=`
  dependents are stopped (estimated in the squash record), and vmctl's
  behaviour with its route database down is an open hypothesis there.

## Open questions (hypotheses, not findings)

- **Why 1.5 to 2.5 GiB an hour now against 1.6 GB a day on 10-15?** Either
  the volume written to Store A rose (more events, larger rows, the paper's
  publishing), or each commit now changes more chunks. Not read from any
  trace. The next measurement: `dolt_log` count and its rate, and
  per-table row counts, compared across two audits.
- **The 25.7 GB file at 11:12.** One Default GC adds only chunks new since
  the last GC; 25.7 GB in one archive is either a large write burst or a
  conjoin of older oldgen files into one (conjoin rewrites files into one
  bigger file; the 10-15 assessment saw "beginning conjoin … generation=old").
  If it is a conjoin, its inputs should have been dropped from the manifest;
  whether their files remain on disk until a later prune was not checked.
- Whether Dolt 2.4.2 changed auto GC's mode or oldgen handling relative to
  the 2.2.0 code read here.

## Options (recommendation first)

1. **Run the built Store A squash once, supervised** (owner's ruling §23
   already orders it; the record's first-run procedure). It needs a
   maintenance window and free space for the backup's divergence and the GC
   copy; today's free space (72 GiB, falling) is the binding constraint, so
   it should run after the paper's next compaction frees its image.
2. Then enable its weekly timer. At today's rate weekly is too slow (about
   250 to 400 GiB a week); the schedule must follow the measured rate, or
   the rate's cause must be found first.
3. Do not raise the history audit's 50 GiB threshold; it is the only alarm.
