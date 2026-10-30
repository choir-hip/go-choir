# The platform Dolt stalls under its 3 GiB MemoryHigh while auto-GC runs, and the liveness probe kills it

Status: open. Recorded 2026-10-24 by FP0, from a read-only investigation of
Node B's journal and cgroup files (no SQL, no restart). Mutation class of this
record: green.

## What happened

The liveness probe (go-choir-dolt-liveness-probe, `SELECT 1` every 30 s, 5 s
deadline, 3 strikes) aborted the platform Dolt (Store A, port 13306) twice:

| | first good probe before | strikes 1, 2, 3 | abort | back |
|---|---|---|---|---|
| hang 1 (pid 181856, up since 10-23 02:09:51) | 06:07:56 | 06:08:26, 06:09:01, 06:09:31 | SIGABRT 06:09:31; stop 06:11:22, SIGKILL 06:11:42 | 06:11:43 |
| hang 2 (pid 973888) | 06:41:39 | 06:42:09, 06:42:39, 06:43:09 | SIGABRT 06:43:09, exit 06:43:13 | 06:43:18 |

The 06:11 escalation also took corpusd down (SIGKILL at 06:11:22). At 06:43:13
corpusd logged `doltbatch platform: snapshot commit failed: ... invalid
connection` and vmctl logged `packets.go:58 unexpected EOF`. On b@c's computer
a guest event append got `corpusd returned 400: computer event CAS: commit:
invalid connection`, then `403 computer event capability refused`, and a
27-minute engineering run (synthetic DeepSeek V4.1 Flash, 203 iterations) died
without a candidate.

There was no warning in the ten minutes before either abort. Nothing else heavy
ran in 06:00 to 06:46 (backup 02:34, compaction 04:00, nix GC 04:40). Host
memory was fine: about 12.7 GB available, system memory PSI 0.

## What the server was doing (the two SIGABRT dumps)

Both show a Dolt auto-GC (archive compression) in flight, and no deadlock:

- the GC driver `AutoGCController.doWork` -> `RunDoltGC` -> `ValueStore.GC` ->
  `markAndSweeper.SaveHashes` (nbs/store.go:2621);
- one goroutine runnable in `gozstd.CompressDict` <- `convertSnappyAndStage`
  <- `gcCopier.addChunk`, doing the compression; 15 GC workers waiting on the
  same mutex (store.go:2622);
- behind it, 8 goroutines in `NomsBlockStore.Root` RLock (from
  `NewDoltTransaction`, `GetBranches`), the stats worker in
  `NomsBlockStore.Get`, and a client COMMIT in `journalWriter.writeCompressedChunk`
  waiting for the journal write lock.

No lock waiter is older than a minute: a convoy, not a deadlock.

## Why (hypothesis, strongly supported)

- The unit has `MemoryHigh=3G`, `MemorySwapMax=0`, and no `GOMEMLIMIT`
  (nix/node-b.nix, go-choir-platform-dolt). The cap dates from 2026-10-09, when
  the process measured about 1.1 GB RSS.
- The process now sits at the cap: `memory.current == memory.high`, anon
  3.12 GB, file 87 MB, so nothing is reclaimable. The old process logged
  22.4 M `memory.high` events; the new one logged 121,901 in its first
  31 minutes. At the 06:09:31 abort the cgroup's `memory.pressure` read
  `full avg10=77.49`, and one thread sat in `__mem_cgroup_handle_over_high`.
- The journal grows about 17 MB/min, and auto-GC runs about every 34 minutes.
  A GC allocates past the cap, the kernel throttles the allocating threads,
  the single compression goroutine slows while it holds the GC mutex, and the
  store's Root, Get and commit paths queue behind it. `SELECT 1` misses 5 s.
- A GC that ends quickly is a 25 s blip (strike 1/3 only, six times since
  10-23 19:54, five of them within 25 s of a GC finishing). A GC that runs
  past 90 s hits 3/3 and is aborted mid-flight, which leaves a bigger journal
  for the next GC. The 06:43 to 06:52 GC finished and wrote a 158 MB archive,
  larger than the earlier ~110 MB ones.

Not proven: a writer-starvation pattern in the GC read path could stall the
store even without throttling; the throttle may only amplify it. The next GC
(about 07:25 to 07:30) can confirm with a 1 s sampler of `memory.events high`,
`memory.pressure`, `/proc/<pid>/io` and per-thread wchan.

## The 07:23 GC (sampler, 1 s, read-only)

- The GC that ended at about 07:23:40 (journal 541 MB to 120 KB) caused no
  probe strike, and the cgroup's `memory.pressure full avg10` stayed at 0.00
  throughout, with `memory.current` at the cap the whole time. Only about
  2,500 `high` events fell in the window. So a GC at the cap is not always a
  stall: the 06:09 and 06:43 stalls had `full avg10` 77.5 and 37.5. The
  throttle hypothesis stands for those two, but it is not the whole story.
- The cgroup holds about 87 MB of page cache, because anon (2.9 to 3.2 GB)
  fills the cap, so nearly every chunk read goes to disk: `read_bytes` grew
  by about 8.5 GB from 07:17:22 to 07:23:58 (about 20 MB/s), 70 GB in the new
  process's first 40 minutes, and 5.8 TB read over the old process's
  28 hours. Room above the Go heap would be page cache, which is a durable
  disk-read saving whether or not it ends the stalls.

## Who it reaches

Every computer: Store A holds the canonical event and control log. Each abort
breaks in-flight commits for every guest, and one abort also restarted
corpusd. Long agent runs die on the append that lands in the window.

## Options (not taken here)

1. Tell Go about the cap: `GOMEMLIMIT` under `MemoryHigh` (the restic and
   squash units already do this), so the Go collector works before the kernel
   throttles. Alone it cannot cover cgo allocations (gozstd).
2. Raise `MemoryHigh` (for example 4G with GOMEMLIMIT 3GiB). The host has
   about 12.7 GB available; the cap was sized for a 1.1 GB process.
3. Make the liveness probe GC-aware (a running auto-GC is alive, not hung), or
   lengthen its window, so the probe stops aborting the GC it is waiting on.
4. Lower the GC cost: reduce the journal growth rate (RES-23's write cuts), or
   tune Dolt's auto-GC threshold.

FP0's lean: 1 and 2 together as one deploy of their own (it restarts the
platform Dolt), then 3 after the sampler confirms the mechanism.

## Evidence

- Node B journal: go-choir-platform-dolt, go-choir-dolt-liveness-probe,
  go-choir-corpusd, 06:00 to 06:46 UTC 2026-10-24; the two goroutine dumps.
- The conductor's log: /Users/wiz/Dev/lanes/_runs/apprewrite/log.md, 06:50 entry (local).
