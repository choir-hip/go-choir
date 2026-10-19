# The paper's 48 GB disk filled with Dolt history and live data, and the runtime crash-loops (2026-10-19)

Status: recovered by growing the disk (option 1) at 16:25–16:29 UTC; history still unbounded (owner decision). Found at the 16:22 UTC hourly
check. Class: red (the always-on paper's persistent state and VM
lifecycle). Owner rule: "we need paper publishing 24/7, even after
crashes."

This is the residual named in
[`guest-dolt-storage-growth-2026-10-15`](guest-dolt-storage-growth-2026-10-15.md),
under "Belief and remaining error": "the history and live class grows about
6 GB/day on the paper. That fills the 48 GB disk in about a week after the
fix, unless history is bounded". It filled in four and a half days. Bounding
history (squash plus `dolt gc --full`) is the next station the 10-15
doc named. (Correction: I first called it the owner's "Store A squash"
decision; Store A is the platform's Dolt store, not this guest's.)

## Evidence (guest console on Node B, read-only; counts, sizes, states and times only)

- **The warning was on for about 41 hours.** The first "compaction needed"
  line is at 2026-10-17 23:17:19: "avail 5508 MiB is under 25% of the disk
  after gc; history or live data fills it, not the journal". The online GC
  logged it after every run from then on. Nothing acted on it, and my
  hourly checks watched Node B's btrfs, not the guest's disk.
- **Live data, not journal.** The GC's own lines, sampled:
  `used=42 GiB live=38 GiB` (avail 4556 MiB), then `used=46 GiB live=43
  GiB` (avail 703 MiB), then `emergency: avail 0 MiB … used=46 GiB live=44
  GiB`, then `live=45 GiB`, then `live=46 GiB`. Each online GC collected
  the journal (180, 430, 154 MiB to 0) and freed nothing: "avail 0 MiB ->
  0 MiB".
- **Writes started failing before the crash.** "conjoin of database failed
  … write /mnt/persistent/… " warnings from 15:08:26, every five minutes.
- **The crash.** At uptime 41155 s (about 15:40 UTC) Dolt raised a fatal
  error: "write …/.dolt/noms/vvvv…: no space left on device: error writing
  to database journal file". The runtime restarted and ran on until a
  periodic GC failed at 16:18:40 ("gc_copier, CopyTableFile error: write
  …/noms/oldgen/nbs_table_…: no space left on device"), then died again.
- **The crash loop.** From 16:19:11 to 16:19:19 the runtime restarted five
  times, about every two seconds. Each boot ran the emergency boot GC
  ("avail 0 MiB <= 512 MiB; used=46 GiB live=46 GiB") and died writing a
  manifest ("nbs_manifest_…: no space left on device: fatal error closing
  table persister", from `store.MaybeRunDoltGC`). No runtime line after
  16:19:19.
- **The platform.** choir.news health is ok on 8be39bb36. The front page
  serves its stored stories. The paper's runs API returns no runs, and
  nothing new publishes.
- `data.img` is 49,152 MiB (48 GiB), last modified 16:20.

## Mechanism

- Default Dolt GC collects only the journal. Everything reachable from a
  commit stays, and the store commits a checkpoint every 100 finalized
  events. History plus live data grew from 38 to 46 GiB in about two days
  here, in line with the 6 GB/day measured on 10-15.
- The emergency GC at 0 MiB cannot finish: default GC writes its output
  before it frees anything (measured 1.9 GB on 10-15). So every boot dies
  in the boot GC. That is failure mode FM3 from the 10-15 doc, reached
  through history rather than the journal.
- `compaction_needed` was made visible on 10-15 (FM8) so that this would
  not be silent. It was visible. No one was watching it: it is a signal
  with no consumer.

## Recovery options (not chosen yet)

1. **Grow the disk while the computer is stopped** (the 10-15 recovery,
   32 to 48 GiB). Hold the computer, stop the VM, reflink-copy
   `data.img` as the rollback, extend the file, grow the filesystem, boot.
   Fast and proven. It buys about two to three days at 6 GB/day, plus
   whatever the boot GC then reclaims.
2. **Offline compaction** (`docs/runbooks/offline-guest-gc.md` with
   `projection-compact`: squash plus `dolt gc --full`). This reclaims most
   of the 46 GiB (1.7 GiB after compaction on 10-09). The runbook keeps
   `--full` "not automated pending review", and the squash is the owner's
   open decision.
3. Both: grow now to restore publishing, then compact on the owner's
   decision.

## Lessons

- A health flag without a consumer is the same as no flag. The hourly
  checks should read the guest's `dolt_gc` record and disk, not only the
  host's.
- The 10-15 doc predicted this fill date to within days. A predicted
  deadline belongs in the handoff's open-decision list with its date.

## Recovery (option 1, 2026-10-19 16:25 to 16:33 UTC)

The conservative, reversible recovery, as on 10-15. Compaction (option 2)
stays the owner's decision: it rewrites Dolt history.

1. **16:25:35** hold (`/internal/vmctl/hold`, reason names this doc). The
   ownership was `state=degraded`, epoch 13134.
2. **16:25:41** stop; no firecracker process left for the VM; no loop
   device attached.
3. Reflink backup `data.img.pre-grow-20261019T1626Z` (48 GiB, ext4
   `go-choir-data`). **Rollback:** hold, stop, swap this file back in.
4. `truncate -s 64G`, `e2fsck -f -p` (it replayed the ext4 journal and
   returned 1, corrected; a second pass returned 0), `resize2fs` to
   16,777,216 blocks: **about 15.8 GiB free**. Loop detached.
5. **16:26:28** `maintenance-serve`: boot under the fence, Dolt
   maintenance phase complete, no crash.
6. **16:27:17** unhold and refresh. A crash boot, as the owner rule
   requires: pre-boot work closed as `interrupted_by_restart`; every
   source schedule "stays active" with its next tick (16:44 for most).
   No maintenance boot argument left in `fc-config.json`. `runtime:
   started` at 16:29:04. The paper's API answered from 16:32:51 (two
   502s from the gateway's resolve before that).

**Downtime:** about 15:40 to 16:29 UTC (about 50 minutes), noticed at the
16:22 hourly check.

**New deadline:** about 15.8 GiB free, at the measured 3 to 6 GB a day of
history and live growth, and a GC needs about 2 GB of headroom to finish.
The disk fills again in about **two and a half to four days, around
October 22**, unless history is compacted or the disk grown again.

**Watching from now:** the hourly check reads the guest's last `store:
dolt gc` line (avail MiB, live GiB), not only Node B's btrfs.

**Acceptance (16:50):** 38 runs since 16:29 (19 Texture, 16
management, 3 research), all gpt-6-luna; no fatal, panic or "no space"
line since the boot. Four front stories published 16:46 to 16:51
(Shakahola v14, WFP Gaza v6, Meta No Kings v7, the budget synth story
v5). The paper publishes again.

## Second mechanism: Dolt's conjoin needs the whole old generation in free space (17:36)

The 17:36 hourly check, reading the guest's GC lines:

- After the grow, free space went 15.8 GiB (16:26) → 12.1 GiB at the
  16:51 GC (journal 1188 MiB) → 13.1 GiB after it → 11.9 GiB at the 17:28
  GC → 12.9 GiB after it. Live data held at 46 GiB.
- **After each GC, Dolt starts a conjoin of the old generation**
  ("beginning conjoin of database … generation=old … upstream_len=268",
  then 269), and it fails: "conjoin of database failed … write
  …/noms/oldgen/nbs_table_…: copy_file_range: no space left on device"
  (17:30:27, with about 12.9 GiB free). The same failure ran every five
  minutes from 15:08 to the 15:40 crash.
- **Why (Dolt source, pinned version):** `store/nbs/conjoiner.go`
  conjoins when a table set holds more than `defaultMaxTables = 256`
  files. `chooseConjoinees` sorts by chunk count and keeps taking tables
  while the running sum exceeds the next table. Every online GC since the
  10-15 fix promoted one table of similar size into the old generation,
  so the sum always exceeds the next one, and it chooses **all 269
  tables, about 45 GiB**. That copy can never fit, and each attempt
  fills the disk to zero before it fails.
- **Effect:** every GC (about every 35 minutes now) briefly takes the
  disk to zero. Any journal write in that window is fatal, which is how
  the 15:40 crash happened ("error writing to database journal file").
  The grow alone does not make the paper safe.

Decision (conservative, reversible, same procedure): grow the disk again,
to 128 GiB, so the conjoin of about 45 GiB fits once. After it, the old
generation is one large table plus small ones, and the conjoiner then
takes only the small ones. Host cost: Node B's btrfs has about 138 GiB
free inside its data chunks, so no new chunk and no metadata-chunk
growth. Offline compaction (squash plus `gc --full` with
`projection-compact`, owner-approved once on 10-09) remains the real fix
for history growth, and the next station.

## Second grow (2026-10-19 17:39 to 17:41 UTC)

- 0 runs in flight. **17:39:45** hold and stop; no firecracker process,
  no loop. Reflink backup `data.img.pre-grow-20261019T1740Z` (64 GiB).
  **Rollback:** hold, stop, swap it back in (the 48 GiB backup from 16:26
  is also kept).
- `truncate -s 128G`; `e2fsck -f -p` (journal replay, returned 1; second
  pass 0); `resize2fs` to 33,554,432 blocks: **about 78 GiB free**.
  Loop detached.
- **17:40:19** `maintenance-serve`: Dolt maintenance complete, fenced,
  no crash. **17:40:51** unhold and refresh; no maintenance boot argument
  left. `runtime: started` at 17:41:19; schedules ticked at 17:56:18 and
  runs resumed (5 management runs by 17:58).
- Downtime about 17:39 to 17:56 for publishing.
- Dolt has no setting to turn conjoin off: `DisableConjoin` is only used
  inside GC.
- To confirm: the first GC after the boot is followed by "conjoin
  completed successfully" with a small `new_upstream_len`, and free space
  stays near 78 GiB minus growth.

**Confirmed (18:08):** the first GC after the boot ran at 18:05:29
(journal 1131 MiB, avail 72,947 MiB → 73,883 MiB). The conjoin began at
18:06:01 with 270 old-generation tables and logged "conjoin completed
successfully … new_upstream_len=2" at 18:08:33. No fatal or "no space"
line. The table count is now 2, so the next conjoin is about 255 GCs away
(days), and the conjoiner will then take only the small tables.

Remaining: history and live data still grow (46 GiB live). At the
measured rate, about 73 GiB free lasts roughly two to three weeks.
Offline compaction is the next station, and should come before that.

## Host side effect: Node B's btrfs has no unallocated space (18:59)

The grows moved the problem to the host. At the 18:59 check, Node B's
btrfs reads: device allocated 475.31 of 475.31 GiB, **unallocated 1 MiB**
(40 GiB at the 00:29 headroom run). Data chunks grew from 400 to 442 GiB
(used 262 → 330 GiB): the conjoin wrote about 45 GiB of new table files
into the grown, previously sparse part of `data.img`. The guest freed the
270 old tables inside ext4, but without discard the host still holds
those blocks, and the two reflink backups pin the old extents too.
Metadata is 15.02 of 16.64 GiB (90.3%) and **cannot get a new chunk**.
Data chunks still have about 112 GiB free inside them.

Risk: when metadata fills, every write on Node B fails with ENOSPC, every
computer and the platform included. At the day's rate (about 0.04 GiB of
metadata an hour) that is over a day away, but bursts are faster.

Remedy, in order: run the designed headroom service now (`btrfs balance
-dusage=20,limit=20`, idle IO) instead of waiting for 00:58; if it frees
too little, a wider `-dusage` balance; trim the guest's freed blocks
(offline `fstrim` on the next stop) and drop the 48 GiB backup once the
paper has run a day on the new disk.

**Done (19:02 to 19:05):** the headroom service's balance
(`-dusage=20,limit=20`) returned 8 GiB in 9 s. A second idle-priority
balance (`-dusage=40,limit=30`, `ionice -c3 nice -n19`) relocated 30 of
456 chunks: **unallocated 31 GiB**, data chunks 407 GiB (81% used),
metadata got a new chunk (18.64 GiB, 80.5% used). The paper kept running
throughout; choir.news health ok.

Still open: Node B's data use rose about 68 GiB today, mostly the guest's
freed-but-not-trimmed blocks and the two backups. Next stop of the paper:
offline `fstrim`, then drop the 48 GiB backup.

## History compaction (owner, 2026-10-19 about 20:30: "I want history compaction and other stuff to reduce resource consumption")

Mutation class: red (the always-on paper's persistent store). Same tool
and sequence as the owner-approved 10-09 window
(`guest-store-history-bloat-and-memory-shape-2026-10-09.md`): hold, stop,
reflink backup, offline mount, `projection-compact` (squash the single
branch to one commit on its root, then `CALL DOLT_GC('--full')`; it
refuses unless the content witness, schema plus per-table digests, is
identical before and after), `fstrim`, unmount, maintenance-serve,
unhold, refresh.

- Canonical head label: the last head the paper reported to vmctl
  (`859f4f2d…`, sequence 1,334,778, reported at the 17:41 boot). The
  head only labels the squash commit; the witness compares content.
- The tool runs on Node B under a memory cap (`systemd-run`,
  MemoryHigh 8G, MemoryMax 10G, low IO weight), since the 10-15 GC test
  peaked at 8 GB and Node B has about 13 GiB available.
- Expected: Dolt store from about 46 GiB to a few GiB; `fstrim` returns
  the freed blocks to Node B once the older backups are dropped.
- Rollback: hold, stop, swap `data.img.pre-compact-<ts>` back in.
- Failure modes to check: the witness differs (the tool refuses; restore
  nothing, boot the unchanged store); more than one branch (refused);
  the GC runs out of memory (capped; the store is untouched until the
  GC's final swap, and the backup covers a torn swap); the boot after
  compaction replays slowly or misses heads (verify `runtime: started`,
  schedules active, runs, and a publish).

## Compaction aborted; the store restored unchanged (20:30 to 21:27)

- 20:30:46 hold and stop; reflink backup `data.img.pre-compact-20261019T2035Z`;
  offline mount (store 48 GiB, 7 old-generation archives after the
  conjoin). 20:31:00 `projection-compact` started as a transient unit
  (MemoryHigh 8G, MemoryMax 10G).
- **The full GC did not fit in memory.** At 20:56 it held 8.3 GB of
  anonymous memory and 8.4 GB in swap, so its 115 GB of "writes" were swap,
  not store output (the guest disk stayed at 49 GiB used). I raised the cap
  to 13G/15G at about 20:57. By 21:07 it held 14 GB plus 16.9 GB of swap,
  **Node B's 15 GB of swap was full**, and it had read 781 GB: thrashing,
  not converging. choir.news stayed healthy.
- 21:09:27 stopped (SIGTERM, then SIGKILL on timeout). It had used 13 GB
  of memory and 15.7 GB of swap at peak, 902 GB read, 147 GB written, in
  38 minutes. Node B recovered at once (swap 0, 22 GB available).
- **Restore, not repair:** the squash had committed but the GC was killed
  and the witness check never ran. I moved the image aside
  (`data.img.compact-aborted-20261019T2110Z`, kept for diagnosis) and
  restored the 20:30 backup by reflink. Read-only fsck clean.
  maintenance-serve 21:10:08, unhold and refresh 21:10:42, schedules
  active, next ticks 21:26:54. Downtime about 20:30 to 21:27.

**Lesson.** `DOLT_GC('--full')` holds the store's reachable set in memory.
At 9.5 GiB (10-09) it fit; at 48 GiB of mostly history it needed well over
30 GB. Offline full GC on Node B cannot compact this store. Options for
the history bound, to choose next:
1. Rebuild instead of GC: copy the squashed HEAD's tables into a fresh
   database (row copy, bounded memory) and swap it in, with the witness
   compared. Or re-materialize from the latest compact base plus the tape
   tail (the 10-09 doc's "guest reclaim = rebase onto a compact base").
2. Compact more often so each full GC stays small (it fit at 9.5 GiB).
3. Cut the write volume at the source: group commit in the projection
   tape (`docs/problems/platform-artifact-small-file-growth-2026-10-19.md`
   measures 3x to 7x fewer events), which also slows this store's history
   growth.

### Why the full GC needed so much memory (code reading, 21:25; hypotheses until traced)

A read-only reading of the pinned Dolt module (go@v0.40.5-0.20260716231223-6bea93f2f665)
by a Sonnet subagent; file:line refer to that module.

- **Memory scales with reachable chunks, not disk size.** Each full GC sweep is
  a breadth-first walk from the roots. Per reachable chunk it keeps a `visited`
  entry (every chunk counts as non-leaf when `IncrementalFileSize=0`,
  `store/nbs/store.go:2414`), the output archive writer's per-chunk state (one
  unrotated output file, `archive_writer.go:157,183-184`, `gc_copier.go:55-58`)
  and the walk frontier with copies (`store.go:2432-2459`): roughly 200 B per
  chunk plus about 150 B at the widest level. Archive indexes for every file
  also load onto the heap (mmap off, `archive_reader.go:224-237`).
- **Hypothesis: C memory the Go GC cannot see.** Archive reads cache 256 zstd
  dictionaries per file (`archive_reader.go:216`); a hash-order walk misses and
  builds new ones freed only by finalizers. `projection-compact` sets no
  `GOMEMLIMIT`.
- **The 900 GB of reads** were the page cache squeezed by anonymous memory under
  the cgroup cap: indexes and archive pages read back again and again.
- **Open question that decides everything: is the reachable set small after the
  squash?** `internal/projectionbase/compact.go:96-100` checks only
  `dolt_branches`; Dolt's GC also roots tags, remotes, internal refs, working
  sets and stashes (`doltdb/doltdb.go:1985-2005`). Any such ref keeps the whole
  history reachable, and the squash then frees nothing. Count the refs on the
  aborted image (offline, read-only) before choosing.
- **The witness is memory-heavy too:** `queryTableContentHash`
  (`internal/computerversion/dolt_state_extractor.go:322-380`) holds every row
  of a table as JSON and sorts it. It fit under the 10 GB cap before the squash.
- **The Dolt v2.4.2 bump does not change this:** the mark-and-sweep code is
  byte-identical.

Ranked options from that reading:
1. **Fresh-database row copy** (`CREATE DATABASE`, `INSERT … SELECT` per table in
   the same engine, swap directories). Memory is independent of chunk count
   (pending edits flush at 64K, `store/prolly/tuple_mutable_map.go:30`; caches
   256 MiB). Needs schema, indexes and auto-increment recreated; the witness
   checks it, and the witness should become streaming.
2. **Push the squashed branch to an empty `file://` remote** and use that store.
   The puller keeps one seen set (about 40 B a chunk) and rotates files at
   1 GiB: about a quarter of GC's bookkeeping. Not verified end to end.
3. **Full GC with bounds:** `DOLT_GC('--full','--incremental-file-size',…,'--archive-level',0)`,
   `GOMEMLIMIT` about 20 GiB, `TMPDIR` on the data disk, `MemoryMax` without
   `MemoryHigh`. Bounded only if the reachable set is modest.

Next safe probe: on `data.img.compact-aborted-20261019T2110Z` (not the live
image), list every ref class and count chunks reachable from the squashed head.

### Bounded full GC works: 48 GiB to 7.5 GiB in 2 min 19 s at 6.1 GB (21:30 to 22:05)

Probe on a reflink copy of `data.img.compact-aborted-20261019T2110Z` (already
squashed), mounted on Node B at `/mnt/probe-refs`, never the live image.

- **Refs:** one branch (`main`), no tags, no remotes, no stashes, an empty
  `dolt_status`, two commits. The squash did make the history unreachable, so
  the earlier GC's memory was not a pinned ref.
- **Live data:** `computer_event_index` 1,356,556 rows (3.84 GiB of event and
  receipt JSON), `og_objects` 1,017,782 rows (2.18 GiB of body and metadata),
  `run_memory_entries` 424,992 rows (0.76 GiB), `og_edges` 23,547,
  `texture_agent_mutations` 9,345. About 6.8 GiB raw; the other ~40 GiB was
  history.
- **The killed run's GC output went to the host's `/tmp`.** The embedded
  driver keeps Dolt's process-wide temp provider at `os.TempDir()`, fixed at
  package init (`store/util/tempfiles/temp_files.go:135`); the `dolt` CLI
  points it at `.dolt/temptf` but the embedded driver does not. A 4.2 GB
  `buffered_file_byte_sink_*` was left in Node B's `/tmp` (removed). `/tmp`
  is on the root btrfs, not tmpfs, so this was disk, not memory.
- **Experiment:** `dolt gc --full --archive-level=0 --incremental-file-size=268435456`
  (dolt CLI 2.4.2, whose mark-and-sweep is byte-identical to the pinned
  module) with `GOMEMLIMIT=5GiB GOGC=50`, `MemoryMax=8G`, `MemorySwapMax=0`.
  **It finished in 2 min 19 s: 6.1 GB memory peak, 77 GB read, 18.1 GB
  written, store 48 GiB to 7.5 GiB** (19 table files). Row counts after:
  identical in all five tables; two commits.

So the unbounded `DOLT_GC('--full')` was the problem, not the store size.
With incremental files, leaf chunks skip the `visited` set and the writer's
per-chunk state resets per file (`store/nbs/store.go:2414`,
`gc_copier.go:204-234`); archive level 0 avoids the archive writer and its
zstd dictionaries.

**Failure modes for the tool change, written before the code:**
1. A flag the pinned module does not parse: `DOLT_GC` accepts
   `--archive-level` and `--incremental-file-size` there
   (`dprocedures/dolt_gc.go:249-266`).
2. Temp output on a different filesystem from the store: set the temp
   provider to `<workspace>/texture/.dolt/temptf` before opening.
3. Memory still unbounded through the witness (`queryTableContentHash` holds a
   table's rows as JSON): it fit under 10 GB before; set a Go memory limit and
   run under `MemoryMax` with no swap, so it fails fast instead of thrashing.
4. Archive level 0 leaves plain table files: 7.5 GiB instead of a smaller
   archive; the guest's own online GC writes archives later. Accept.
5. Pinned-module result differs from the CLI's: prove it end to end with the
   built tool on a fresh probe copy (witness must match) before any live use.

### The content witness is the next bottleneck (22:20 to 22:55)

End-to-end run of the deployed tool (471689517, bounded GC) on a reflink
probe of the unsquashed 20:30 backup (`/var/lib/go-choir/probe-paper-e2e.img`):
it spent the whole 34 min in the **first** content witness, before the
squash: 8.6 GB peak (cap raised from 8G to 12G at the owner's go-ahead), 2 h
33 min of CPU (about 4.4 cores, competing with the paper), 65 GB read,
store untouched. Stopped at 22:54. `queryTableContentHash` JSON-encodes and
sorts every row of each table; on 1.36M event rows that is the cost.

Remedy: compare Dolt root value hashes (`dolt_hashof_db`) instead. Prolly
trees are history-independent, so equal schemas and rows give an equal root
hash whatever the history; it costs nothing. The squash's `-A` commits any
uncommitted working changes, so the check is the WORKING root before against
the HEAD and WORKING roots after. The full witness stays behind a flag.

## History compacted: 52.0 GB to 8.06 GB, root hash identical (23:23 to 23:41)

- **E2E first** (deployed a3fe4c98c, reflink probe of the unsquashed 20:30
  backup): 13,539 commits to 2, 51.4 GB to 7.97 GB, root
  `7rlefddgk6ksbo1rjcdgk8mrc2rfkhg0` before and after, 12 min 26 s, 8.4 GB
  peak (cgroup, page cache included), no swap.
- **Live:** hold and stop 23:23:25 (head `69a2cd27…7f2e`); reflink backup
  `data.img.pre-compact-20261019T2324Z`; `projection-compact` under
  `MemoryMax=10G`, `MemorySwapMax=0`, 23:23:50 to 23:34:55: **13,700 commits
  to 2, 51,954,497,839 to 8,062,682,469 bytes, root
  `6kpdt7e7vr87ctv60ikiuvch5gaim62a` before and after**, 10 GB peak, 79.9 GB
  read, 28.7 GB written. Guest disk 49 GB to 8.0 GB used of 126 GB.
  Read-only fsck clean. maintenance-serve 23:35:21 (crash boot: in-flight
  polls closed as interrupted, per the restart rule), unhold and refresh
  23:41:15.
- **Host side effect:** the compaction's 28.7 GB of new blocks are CoW
  against the backup images, so Node B unallocated fell from 30 to 16 GiB.
  Reclaim: delete superseded backups (pre-grow 1626Z and 1740Z, aborted
  2110Z, pre-compact 2035Z), keep 2324Z until the paper is verified, then
  trim the image and run the headroom balance.

**Standing remedy.** Run this compaction on a schedule before the store
nears its disk (it grows about 0.25 GiB an hour: weekly keeps it under
about 50 GB of history). Today it needs a hold and about 18 minutes down.
The durable fix is fewer writes at the source (the appendOps design,
docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md).
