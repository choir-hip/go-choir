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
