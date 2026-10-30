# The off-host backup reads terabytes and needs swap (2026-10-22)

Date: 2026-10-22. Lane: `lane/fp9-backup-memory` (FP0, FP9 Q4). Class: this
record green; the change it leads to is red (host deployment config, data
durability). Status: problem recorded; options ranked below; the change is
in the next commit; proof is FP0's, after deploy (the proof plan below).

## Why this record exists

The owner wants host swap removed (2026-10-22, owner-notes §40, FP9 Q4:
"Prob remove swap. Swap seems to lead to compounding problems").
`lane/fp9-remove-swap` (record `docs/problems/node-b-host-swap-2026-10-22.md`,
dependency table row 5 and follow-up F2) is held because one unit depends on
swap: `go-choir-offhost-backup`. Its `serviceConfig` in `nix/node-b.nix`
(`offHostBackupUnit`, about :384-393) sets `MemoryHigh=10G`, `MemoryMax=12G`,
`MemorySwapMax=4G`. Under cgroup v2 `memory.max` caps resident memory and
`memory.swap.max` adds swap beyond it, so the unit's effective ceiling today
is 16G, and it uses all of it.

## Evidence

FP0 read Node B at 2026-10-22 06:05 UTC (systemd's own accounting):

| Run | Window | Result | MemoryPeak | MemorySwapPeak | Read | Written | Other |
|---|---|---|---:|---:|---:|---:|---|
| last (2026-10-22) | 02:44:34 to 03:55:21 (1 h 11 min) | success | 9,341,034,496 B (8.7 GiB) | 4,184,424,448 B (3.9 GiB, the whole allowance) | not read | not read | |
| earlier | 1 h 18 min | | 10.1G | 4G | 2.3T | 66.4G | |
| earlier | 2 h 7 min | killed, status=15/TERM | 10.1G | 4G | 3.6T | 57.4G | |
| earlier | | | 10G | 2.9G | 571.9G | | |
| earlier | | | 8.6G | 3.8G | 917.7G | 189.4G | 88G outgoing |

The earlier rows match runs in `docs/problems/no-off-host-backup-2026-10-20.md`
§13: 2.3T in 1 h 18 min is run 3 (item 17, the daily `restic check` at the
memory line); 3.6T and 2 h 7 min killed is run 4 (item 18, stopped by hand
during `check`); 571.9G with 2.9G swap is run 5 (item 20, `restic stats`
at the memory line). The 917.7G row is not matched to a run here.

Host: 31.9 GiB RAM, about 7.7 GiB `MemAvailable` while guests run. The
guests are Firecracker VMs whose memory is the product; vmctl reserves
`MemoryLow=14G` for them and they never swap (`MemorySwapMax=0` on the vmctl
unit). A backup that needs 13 to 14 GB and reads terabytes competes with
them every night.

One reading in the table matters more than the rest: on the last run the
memory peak (8.7 GiB) stayed **below** the unit's own `MemoryHigh` (10G),
and swap still filled to 3.9 GiB. The unit's own line did not push restic
into swap; host-wide reclaim did. With `vm.swappiness = 1` the kernel swaps
anonymous pages only once file cache is nearly gone, so at that moment the
host as a whole was short of memory, not only the unit. That is the
competition with the guests, measured.

## What the unit backs up, and how

From `scripts/off-host-backup.sh` (`resolve_paths`, `stage`, `EXCLUDES`) and
the design record (§3):

- `platform-artifacts` read live: the event tape (`sha256/computer-event`,
  `computer-event-payload`, `pin-receipts`), file-CAS chunks and roots, `og`
  and other bodies, the signing and key-escrow keys. Excludes
  `sha256/projection-base` (47 GB, rebuilt from the tape) and
  `checkpoint-scratch`.
- Store A and Store B (Dolt): a read-only btrfs snapshot of each server's
  data directory under `offhost-backup/staging/dolt/` (§13 item 16), at
  fixed paths so restic matches unchanged files with the previous run.
- `auth.db` and `mail.db`: `sqlite3 .backup` into staging.
- Guest `data.img` of the paper and owner computers only (item 11):
  `cp --reflink=always` into staging, deleted at exit.
- Secrets, env files, `mail/`, `files/`, `vm-state/ownerships.json`, a run
  manifest (Dolt heads, restic and Dolt versions).
- One `restic backup` invocation with `RESTIC_PACK_SIZE=64`,
  `RESTIC_READ_CONCURRENCY=2`, `RESTIC_COMPRESSION=auto`, `GOGC=50`, no
  `GOMEMLIMIT`; encrypted on Node B, sent through an SSH forward to an
  append-only rest-server on node-a. The daily run no longer runs
  `restic check` (item 18) or `restic stats` (item 20); the weekly verify
  unit runs `check --with-cache --read-data-subset=n/10` under the same
  limits.

Restore is exercised by `scripts/off-host-restore-drill.sh` (pieces `tape`,
`snap`, `image`); the `snap` drill passed on 2026-10-21 at a 7 GB peak with
no swap (item 21).

## Why restic reads terabytes

Two different things are counted as "read from disk" (the cgroup's block
IO, `io.stat`):

1. **Source files restic must re-read every run (legitimate, bounded).**
   restic skips a file whose type, size, mtime, ctime and inode match the
   parent snapshot; anything else is read and chunked in full.
   - The two guest image clones: `cp --reflink=always` makes a new inode
     with a new mtime each night, and the live image changes anyway, so
     both are re-read in full every run. Holes cost no disk IO; the
     allocated extents do (the owner computer's image was 40 GB on disk,
     128 GB apparent, item 10).
   - Dolt files in the btrfs snapshots: a snapshot keeps inode numbers and
     timestamps, so unchanged table files are skipped (run 4 added 6.5 GB);
     the chunk journal (the journal file and `journal.idx` under
     `.dolt/noms`, appended on every commit) and any table file rewritten by GC or the
     04:00 store compaction are re-read whole.
   - New tape files (about 1.5 million a day before segments, about 2.3 GB)
     and new file-CAS chunks.
   - The staged sqlite copies (new inode each run, MBs).
   Order of magnitude: tens to low hundreds of GB a night, dominated by the
   images.
2. **Reclaim thrash (most of the terabytes; hypothesis).** Run 4 at the
   memory line showed about 290 MB/s of disk reads "by page fault with no
   read calls (`rchar` flat)" (item 18). Reads with no read calls are
   swap-ins and refaults of file-backed pages (restic's own text, its cache
   files) evicted by reclaim. 2.3 TB in 78 minutes is about 490 MB/s
   sustained, against a 231 GB source. The writes fit the same story: 57 to
   189 GB written for 3 to 10 GB of new packed data is mostly swap-out.
   Not attributed per phase yet; a hypothesis, not a finding, until a run
   with `MemorySwapMax=0` shows the read volume fall (the proof plan).

## What holds restic's memory

restic is a Go program. Its memory, largest first (shape from item 18,
measured; per-structure sizes inferred, not measured):

- **The trees of three flat directories.** The source is 9.7 million files
  in about 1,440 directories; `sha256/pin-receipts` (4.58 M),
  `computer-event` (2.29 M) and `computer-event-payload` (2.29 M) are one
  tree blob each, hundreds of MB of JSON. For each, restic loads the parent
  snapshot's tree and decodes every node into Go structs, reads the
  directory's names, and builds the new tree's JSON, then compresses and
  encrypts it. Several copies of a multi-GB structure are live at once, and
  this grows with the tape's file count.
- **The repository index.** One entry per blob, held in memory for dedup;
  407 MB on disk (compressed) for about 10 M blobs, larger in memory.
- **Read and pack buffers.** Two file readers (`RESTIC_READ_CONCURRENCY=2`)
  with chunker buffers of a few MB each; packs (`RESTIC_PACK_SIZE=64` MiB)
  are assembled in temp files under the unit's `PrivateTmp`, not RAM. Small.
- **The scanner** (corrected in review round 2). Without `--no-scan`, a
  second goroutine walks the whole source in parallel only to print totals
  for the progress line. It keeps nothing but four counters (`ScanStats`:
  files, dirs, others, bytes; restic 0.19.1 `internal/archiver/scanner.go`).
  While it is inside a directory it holds that directory's names, read and
  sorted (`scanner.go:126-130`), and frees them when it leaves: for the
  4.58 M-entry `pin-receipts` about 0.4 GB for the minutes the walk is
  there, not retained. Its main cost is a second `lstat` of all 9.7 M files
  (IO and dentry/inode slab). `--no-scan` therefore buys IO and a transient
  slice, not the heap peak; it does not strengthen the "under 10G"
  expectation below. The run's `--json` summary and the snapshot do not
  depend on the scan (`cmd_backup.go` runs `arch.Snapshot` and the summary
  unconditionally).
- **Go heap slack.** `GOGC=50` lets the heap grow to 1.5 times the live
  set before a collection, and there is no `GOMEMLIMIT`, so nothing tells
  the runtime about the cgroup's line. If the live set is about 9 GB, the
  heap may reach about 13 GB, which is what run 1 measured (item 8: "about
  13 GB"). How much of the 13 GB is live and how much is collectable
  garbage is not measured.
- **Page cache** of the files restic reads is charged to the unit's cgroup
  and counts in `MemoryPeak`; it is reclaimable and is the first thing
  reclaimed at `MemoryHigh`.

## What the page-cache and memory churn does to guests

- Firecracker guest memory is anonymous memory in the vmctl cgroup:
  `MemoryLow=14G` shields it from reclaim, and with `MemorySwapMax=0` it
  never swaps. It is safe from the backup's reclaim up to 14G, and not
  beyond.
- Guest disk IO goes through the host page cache (charged to vmctl).
  Host-wide reclaim caused by the backup evicts that cache, so guests' disk
  reads go back to the NVMe while the backup runs (hypothesis: not
  measured per guest).
- vmctl admission and pressure reclaim read host `MemAvailable` against a
  15% floor (4779 MiB). A backup holding 9 to 13 GB on a host with about
  7.7 GiB available drives `MemAvailable` under the floor: new computers
  are refused for the length of the run, and if memory PSI `some avg60`
  also stays over its threshold for the sustained samples, pressure
  reclaim hibernates idle computers (`internal/vmctl/pressure_reclaim.go`
  :19-24). Whether that has happened at 02:30 to 04:00 is not known;
  probe in the proof plan.
- Swap itself compounds it: swap IO inflates memory and IO PSI, which is
  the pressure-reclaim input (the swap record's "compounding" loop).
- Walking 9.7 M files also fills the dentry and `btrfs_inode` slab, which
  outlives the run (`node-b-host-slab-btrfs-inode-cache-2026-10-22.md`,
  hypothesis 2 there names this unit).
- **OOM order.** The unit has no `OOMScoreAdjust` (0). If the host runs
  out of memory with no swap, the kernel's badness is RSS plus
  `oom_score_adj` times MemTotal/1000, so an ordinary guest at +500 with a
  few GB resident outscores a 10 GB backup at 0: **a guest dies before the
  backup.** This is the row that makes swap removal unsafe today, more than
  the ceiling.

## Options, ranked

Each with its effect on memory, read volume, durability and restore.

1. **(a) GOMEMLIMIT for restic, under the unit's line; keep GOGC=50; add
   `--no-scan`.** `GOMEMLIMIT` is the Go runtime's soft limit: as the heap
   nears it the collector runs more often and the scavenger returns freed
   pages to the kernel, so the garbage slack above the live set is cut
   instead of being pushed to swap. Set to 8 GiB with `MemoryHigh=10G`: 2
   GiB under the throttle for page cache and non-heap memory.
   `--no-scan` removes the parallel progress walk.
   - Memory: peak about the live set plus page cache, expected under 10G
     (from about 13 GB). If the live set itself exceeds 8 GiB, the
     collector runs at its 50% CPU cap and the heap grows anyway: no
     failure, more CPU, and then `MemoryHigh` and `MemoryMax` govern.
   - Read volume: removes the swap-in and refault thrash if the heap stays
     under the line; `--no-scan` removes the second walk's `lstat`s.
   - Durability and restore: unchanged. Same paths, same snapshots, same
     repository; `--no-scan` changes only progress output.
   - restic's own settings: read concurrency is already 2 (the lowest
     useful) and packs go to temp files, so smaller packs would add round
     trips and save no RAM. No change there.
2. **(d) Memory and OOM isolation of the unit.** `MemorySwapMax=0`
   (restic can never swap, whether or not the host has swap; this also
   means this change can be proven before the swap removal lands);
   `OOMScoreAdjust=1000` (in a host OOM the backup dies before any guest or
   service; it inherits to restic and the ssh forward); keep
   `MemoryHigh=10G` (throttle, reclaim its own page cache first) under
   `MemoryMax=12G` (a cgroup-local OOM kill: only the backup dies).
   - Memory: bounds the unit at 12G resident, 0 swap. A run whose live set
     exceeds that is killed inside its own cgroup, loudly: unit failed,
     `last_attempt_result: failed` with the cause, and the freshness check
     fails at its next 6-hourly run (review round 2, finding 1; before the
     fix a killed run left the previous `ok` standing for up to 36 h).
   - Read volume: no swap IO from this unit.
   - Durability: no change while the live set fits; a night that does not
     fit fails loudly and keeps every earlier snapshot. That is a real
     durability risk, which is why (a) comes with it and the proof runs
     before swap removal.
   - IO isolation: `IOSchedulingClass=idle` (set today) and `IOWeight`
     act only under BFQ or with the io.cost controller; Node B's NVMe
     devices use the kernel's default (`none` for NVMe; nothing in `nix/`
     sets a scheduler), so both are probably inert. Not added here: an
     inert knob reads as protection. FP0 checks
     `cat /sys/block/nvme*/queue/scheduler`. If IO isolation is wanted, it
     is `IOReadBandwidthMax` on the md device, measured first.
3. **The substrate: fewer, bigger tape files (tape segments, A2 repack).**
   The memory is the three flat directories' trees. Segments are on for
   writes (`PLATFORM_TAPE_SEGMENTS=write`,
   `docs/archive/definitions/choir-tape-segments-a2-repack-2026-10-21.md`), so new
   pins stop adding files; the repack of the legacy 9 M files is not built.
   - Memory: the tree peak shrinks with the file count, roughly in
     proportion; the index shrinks too. The real fix for both the backup
     and the weekly verify.
   - Read volume: fewer `lstat`s and smaller trees; slab relief.
   - Durability and restore: the restore unit becomes a segment; legacy
     deletes are black and owner-gated. Its own mission, not this lane.
4. **(b) Back up a consistent export per computer instead of raw files.**
   The Dolt stores are already captured as atomic btrfs snapshots, which
   restore whole (item 21). The remaining raw files are the two guest
   images; dropping them for a tape-only rebuild waits on item 10 (b) (a
   computer rebuilt from a restored tape matches its live witness), which
   has not been shown. Exporting the tape per computer is option 3 under
   another name.
   - Memory: images cost read volume, not memory (they are a few large
     files); no memory gain.
   - Read volume: removes the largest legitimate re-read (two images).
   - Durability and restore: weaker until item 10 (b) passes. Not in this
     lane.
5. **(c) Exclude more regenerable files.** Already excluded with a reason:
   `projection-base` (rebuilt from the tape), `checkpoint-scratch`
   (scratch), the backup's own key, the live `mail.db` and its WAL (staged
   via `.backup`), Store B's old sync copy, and the live server's
   `sql-server.info`. Not backed up at all: the Nix store, guest base
   images, `host-build`, Firecracker memory snapshots, `qdrant`. Nothing in
   the backed-up set left to exclude is known to be regenerable without a
   listing of `platform-artifacts` by size and file count from Node B; the
   memory is in the tape, which is never excluded.
   - Memory and read volume: no known gain. Durability: any exclusion is a
     restore gap. Not in this lane.
6. **(e) Raise `MemoryMax` to 16G (the swap record's F2).** Keeps today's
   effective 16G ceiling as RAM.
   - Memory: 16G of RAM on a host with about 7.7 GiB available beside the
     guests. The cap would never bind; the host would run short first, and
     without swap that ends in global reclaim (guests' page cache), vmctl
     admission refusals, possible pressure hibernations, and, at the end, a
     global OOM kill whose first victim at today's scores is a guest.
   - Read volume and durability: none better than today.
   - Not enough on its own: a cgroup ceiling protects the host only if it
     is below what the host can give. With (a) and (d) it is a fallback
     (the proof plan), never a substitute.

## Chosen change (this lane)

(a) plus (d), the smallest that makes the unit safe without swap and does
not change what is backed up:

- `nix/node-b.nix`, `offHostBackupUnit` (all three units: backup, verify,
  freshness): `MemorySwapMax = "0"`, `OOMPolicy = "continue"`,
  `GOMEMLIMIT=8GiB` in the units' environment; `MemoryHigh=10G` and
  `MemoryMax=12G` unchanged. Backup and verify only: `OOMScoreAdjust = 1000`.
  Verify only: `GOMAXPROCS=2` (finding 2 below).
- `scripts/off-host-backup.sh`: `restic backup --no-scan`; attempt marks
  in status.json (finding 1); the verify's check with two connections.
- `scripts/off-host-restore-drill.sh`: the example unit line drops
  `MemorySwapMax=4G` for `MemorySwapMax=0`, `OOMScoreAdjust=1000` and
  `GOMEMLIMIT=8GiB` under 10G/12G (comment only).

Expected: `MemorySwapPeak` 0 (enforced), `MemoryPeak` at or under 10 GiB
(the throttle line; the page cache fills the gap above the Go heap),
read volume down from terabytes to the legitimate re-read set (section
above, tens to low hundreds of GB). Conjecture: most of the 13 GB was Go
heap slack, not live set. Wrong if the run is OOM-killed at 12G with the
collector pinned (high CPU in the "Consumed" line), which would mean the
live set is above 10 GB and only option 3 fixes it.

Order: this lands and is proven on one run **before** `lane/fp9-remove-swap`
deploys. `MemorySwapMax=0` makes the proof valid while host swap still
exists.

## Proof plan (FP0 on Node B, after deploy; not run by this lane)

Before `lane/fp9-remove-swap` deploys. Host swap may still be on; the unit's
`MemorySwapMax=0` makes the run a no-swap run regardless.

0. Confirm the deploy reached the unit:
   `systemctl show go-choir-offhost-backup -p MemorySwapMax -p MemoryHigh -p MemoryMax -p OOMScoreAdjust -p Environment`
   shows `MemorySwapMax=0`, `MemoryHigh=10737418240`, `MemoryMax=12884901888`,
   `OOMScoreAdjust=1000`, `OOMPolicy=continue`, and `GOMEMLIMIT=8GiB` in
   `Environment`; the verify unit the same plus `GOMAXPROCS=2`, and the
   freshness unit `OOMScoreAdjust=0`. Read `status.json`'s
   `last_verify_result` first: if the last Sunday verify failed, the new
   freshness check fails from its first run (it now flags a failed verify
   at once). Also
   `cat /sys/block/nvme*/queue/scheduler` (for the IO note in option 2),
   and the baseline: `grep -E 'MemAvailable|Swap' /proc/meminfo`,
   `grep -E 'pswpin|pswpout' /proc/vmstat`.
1. Run: the next timer run (02:30 UTC plus up to 15 min), or by hand when
   no run is active: `systemctl start --no-block go-choir-offhost-backup`.
   During the run, every few minutes: `systemctl show go-choir-offhost-backup
   -p MemoryCurrent -p MemorySwapCurrent`, `cat /proc/pressure/memory`,
   `grep MemAvailable /proc/meminfo`, and the restic pid's `oom_score_adj`
   (`cat /proc/$(pgrep -x restic)/oom_score_adj`, expect 1000).
2. After it ends:
   `systemctl show go-choir-offhost-backup -p Result -p MemoryPeak -p MemorySwapPeak -p IOReadBytes -p IOWriteBytes`,
   the journal's "Consumed" line
   (`journalctl -u go-choir-offhost-backup -n 50 | grep Consumed`),
   `jq . /var/lib/go-choir/offhost-backup/status.json`, and
   `journalctl -k --since <start> | grep -i -E 'oom|killed process'`.
   Also vmctl's pressure-reclaim and admission lines in the window
   (`journalctl -u go-choir-vmctl --since <start> --until <end> | grep -i -E 'pressure|reclaim|hibernat|admission'`),
   and the same grep for the previous night (02:30 to 04:00 on 2026-10-22)
   as the baseline.
3. Restore drill on the new snapshot (`last_snapshot_id`), in the capped
   unit from the drill script's header:
   `off-host-restore-drill.sh <id> snap` and `off-host-restore-drill.sh <id> tape`.
4. The weekly verify by hand once (`systemctl start go-choir-offhost-backup-verify`),
   read with the same `systemctl show` line. This gates `lane/fp9-remove-swap`
   as much as the backup does.
5. A freshness run after each (`systemctl start go-choir-offhost-backup-freshness`;
   exit 0, journal "fresh: ...").

Pass:

| Reading | Pass | Today |
|---|---|---|
| `Result` / `last_attempt_result` | `success` / `ok`, a new `last_snapshot_id` | success |
| `MemorySwapPeak` | 0 | 4,184,424,448 |
| `MemoryPeak` | under 12 GiB (12,884,901,888), expected at or under about 10.7 GB (the 10G throttle) | 9,341,034,496 |
| OOM | no `oom_kill` in the unit, no oom-kill line in the kernel log | none |
| Read from disk ("Consumed") | under 500 GB (the legitimate re-read set plus margin) | 0.57 to 3.6 TB |
| Written | under 30 GB (packs in temp files plus the clones' metadata) | 57 to 189 GB |
| Wall time | under 2 h | 1 h 11 min |
| Drill `snap` | both stores `found=1` | passed 2026-10-21 |
| Drill `tape` | 200 of 200 identical, both sqlite `integrity=ok` | passed 2026-10-20 |
| Verify | `Result=success`, `last_verify_result: ok`, `MemorySwapPeak` 0, `MemoryPeak` under 12 GiB | 10.3 GB + 4 GB swap (item 18, as the daily check) |
| status.json on a failure | `last_*_result: failed` with `last_*_cause`; never a stale `ok`; `running` only while its process lives (otherwise freshness flags it); valid JSON with both units' keys when backup and verify overlap, a guarantee that depends on util-linux `flock` (on the units' PATH; without it the script warns and writes unlocked) (rounds 3, 4) | a kill left the previous `ok` |

If the run is OOM-killed at 12G (the conjecture is wrong: the live set is
above 10 GB): read the "Consumed" CPU line (the collector at its cap shows
as CPU time well above run 5's), keep this change (the kill stayed inside
the unit, which is the point), and hold `lane/fp9-remove-swap`. Then the
interim, by owner choice: `MemoryMax` up to what the host can give at 02:30
(read `MemAvailable` then) with `OOMScoreAdjust=1000` kept, while option 3
lands. A read volume that stays in terabytes with `MemorySwapPeak` 0 means
the thrash is file refaults, not swap; then `GOMEMLIMIT` goes lower, to 7
GiB.

Rollback: `git revert` of the code commit (the unit returns to 4G of swap
and no OOM adjustment; the script returns to the scan). Live, without a
deploy: `systemctl set-property --runtime go-choir-offhost-backup
MemorySwapMax=4G` (the environment and the script need the revert).

## Residual risks

- **The weekly verify** (`restic check --with-cache`, Sunday 06:00) walked
  every tree at 10.3 GB resident with 4 GB of swap (item 18). It now has no
  swap, fewer tree loaders and `GOMEMLIMIT` (finding 2); whether that fits
  12G is unmeasured. If it is OOM-killed, status.json says so and the
  freshness check fails within 6 h. FP0 runs it by hand after the backup
  proof; swap removal waits for a pass.
- The tape's file count still grows where segments are not written (legacy
  paths), and the tree memory with it. Option 3 is the fix.
- The images' nightly re-read stays (option 4).
- The thrash attribution (section "Why restic reads terabytes", 2) is a
  hypothesis until the proof run's read volume.

## Review round 2 (panel bunch 26), dispositions

Gemini flash accept; Devin accept with lows; DeepSeek v4.1 flash accept
with edits (restic 0.19.1 pinned, nix evaluated, systemd 261.3, all four
script tests passing). Fixed in e025d9121 unless stated.

1. **Medium: a killed run left `last_attempt_result` at the previous
   `ok`.** Fixed. Backup and verify write `last_<kind>_result: running`
   (with `_run`, `_pid`, an empty `_cause`) after only the tool preflight
   (`need`), the exit trap and, for verify, `install -d` of its run
   directory; before the disk guard, the tunnel or any restic call
   (corrected in round 3). A non-zero
   restic exit writes `failed` with a cause ("restic killed by signal 9
   (KILL: a cgroup OOM kill sends this)"). The EXIT trap, with TERM, INT
   and HUP traps, turns this run's `running` into `failed`, "interrupted
   by SIGTERM before a result was written". A SIGKILL of bash leaves
   `running`; freshness reads `running` with a dead or foreign pid as
   interrupted (not fresh) and with a live backup process as in progress.
   The units take `OOMPolicy=continue`: an OOM kill of restic is handled
   by the script, which records it, instead of systemd stopping the unit
   around it (the default `stop` would make the cause "interrupted by
   SIGTERM"). Test: `scripts/off-host-backup-attempt-status-test.sh`, six
   failure modes written first; it failed 11 of 16 checks before the code
   and passes all after.
2. **Medium: the verify unit was under-provisioned.** Fixed by reducing
   its peak, not raising its cap; a larger cap is option 6's problem
   (the host has about 7.7 GiB beside the guests). From restic 0.19.1
   (`internal/data/tree_stream.go`, `StreamTrees`): `check` runs
   connections + GOMAXPROCS + 1 tree loaders, each holding one decoded
   tree. Trees over 50 MiB (the three flat directories) go to one
   dedicated loader, so the big trees are one at a time already; the rest
   scale with the loader count (5 + cores + 1 by default). The verify
   unit now sets `GOMAXPROCS=2` and its check `-o rest.connections=2` (5
   loaders), under `GOMEMLIMIT=8GiB`, 10G and 12G, no swap. Kept
   `--read-data-subset` (the data read streams packs and adds little
   memory) and `--with-cache`. Not available: `check` without its
   structural walk (it always walks every tree of the snapshots checked);
   `check latest` (restic 0.19.1 accepts snapshot filters) walks fewer
   trees but not a smaller peak, since the newest snapshot holds the
   largest trees. The fit is a conjecture; a killed verify is now loud
   (finding 1), and swap removal waits for a hand verify that passes.
3. **Low: the drill's `GOMEMLIMIT=6GiB` contradicted its "about 8 GB"
   note.** Fixed: the example line is now `MemoryHigh=10G`,
   `MemoryMax=12G`, `GOMEMLIMIT=8GiB`, matching the note and item 21's
   7 GB peak with no swap.
4. **Low: `OOMScoreAdjust=1000` on freshness; the comment omitted
   front-importance.** Fixed: the adjustment is on the backup and verify
   units only, and the comment names nix-daemon builds, checkpointd and
   front-importance as the +1000 ties (all smaller than the backup).
5. **Low (doc): the scanner memory claim.** Corrected in "What holds
   restic's memory". Part of the finding stands and part does not: the
   scanner retains only four counters (DeepSeek is right that nothing is
   kept), but it does read and sort each directory's names while inside
   it (`scanner.go:126-130`), so its memory is a transient slice the size
   of the largest directory (about 0.4 GB), not O(1). `--no-scan` saves
   that and a second 9.7 M-file `lstat` walk, not the heap peak.

## Review round 3 (panel bunch 32), dispositions

Gemini flash and DeepSeek accept with edits; Devin pending at writing.

1. **Medium-low: status writers were not serialized.** Fixed in
   f7c963c81, test first in d36ff175d. The daily backup and the weekly
   verify are separate units and can overlap, and both now write at
   start, end and exit; `status_merge` was an unlocked read-modify-write
   through one fixed `status.json.tmp`. It now holds `flock` on
   `status.json.lock` beside the file only around the read-merge-write
   (never across restic; round 4: it waits without a timeout and never
   writes unlocked when `flock` exists), and writes through its own
   `mktemp` in the same directory before the rename. Round 4: a failed jq
   or a failed rename removes the temp, and under the lock any
   `status.json.??????` over an hour old (a writer SIGKILLed mid-merge)
   is deleted.
   `scripts/off-host-backup-status-lock-test.sh` runs the script's own
   status-file section with two concurrent writers of 60 keys each: 61 of
   120 keys survived before the fix; 120 of 120, valid JSON, no temp left
   after (5 of 5 repeats).
2. **Low: the attempt-status test's mode 4 assumed `/proc`.** Fixed in
   the script, not the test: freshness reads the recorded pid's command
   line from `/proc/<pid>/cmdline` and falls back to `ps -p <pid> -o
   args=` where there is none (darwin). On Linux the `/proc` path is the
   one the tests exercise; the fallback runs only off Linux and was not
   run here.
3. **Low: "before anything else" overstated.** Corrected in finding 1 of
   round 2 above and in the script's comment: the mark follows the tool
   preflight, the exit trap and (verify) `install -d`, and precedes the
   disk guard, the tunnel and every restic call.

## Review round 4 (panel bunch 35), dispositions

Flash accept; DeepSeek accept with edits. Test first in 7904d3b13 (it
failed 2 of 6 checks on f67d64c85's script, the wait and the reap),
fix in 645a07439.

1. **Medium: the 60 s fail-open.** Fixed: `flock 9` with no timeout. The
   holder runs one jq on a small file, and the unit's `TimeoutStartSec`
   (12 h) bounds a stuck holder: the waiting unit is then killed and its
   mark reads as interrupted. If `flock` itself errors, nothing is
   written. Test: a holder keeps the lock 65 s and merges what it read
   before sleeping; the waiter's key and the holder's both survive.
2. **Medium-low: the silent no-flock branch.** Fixed: the owner hand-run
   line (`NIX_SHELL_LINE`) includes `util-linux`, and without `flock` on
   PATH `status_merge` warns on stderr that overlapping backup and verify
   can lose each other's marks, then writes (a developer host case).
3. **Low: the lock test on hosts without flock.** Fixed: it prints
   `SKIP: no flock on this host` and exits 0 (checked with an empty
   PATH). The Linux run, 6 of 6 in 66 s, is the evidence.
4. **Low: temp cleanup.** Fixed: explicit removal after a failed jq or a
   failed `mv`, and the hour-old reap under the lock (tested: a 2-hour-old
   temp goes, a fresh one stays). Round 3's sentence above now says this.
5. **Low: the pass table's overlap guarantee.** The row now names the
   util-linux `flock` dependency.

## Review round 5 (Devin's lows on round 4), dispositions

All three reviewers accepted round 4 (76d36c6b9). No behaviour change in
this round.

1. **Low: the lock test's counts.** Corrected above: the test has six
   checks (three from round 3, three from round 4), not seven. Rerun on
   2026-10-22: f67d64c85's script fails 2 of 6 (the 65 s wait, the
   hour-old reap); this head passes 6 of 6.
2. **Low: a lock error's return is discarded (named residual, not
   changed).** `status_merge` returns non-zero when `flock` itself
   errors, and no call site checks it (the attempt marks in
   `attempt_start` and `attempt_end`, and the backup's and verify's final
   status writes; the script has no `set -e`). So a persistently failing
   `flock` does not fail the run: the backup or verify continues
   unmarked, and status.json stays frozen at its last write. That is
   deliberate: refusing the backup over a status-file lock would trade
   real durability for bookkeeping. It is bounded by the journal (each
   failure logs "could not take the status.json lock; status not
   written") and by the freshness check's 36 h `last_success_epoch`
   alarm, which a frozen file trips. Round 4's "nothing is written" means
   exactly that: the status write is skipped, not the run.
3. **Low: `touch -d` is GNU only.** The lock test back-dates its stale
   temp with `touch -d` (Node B, unchanged), else `touch -t` with a stamp
   from BSD `date -v` (darwin), else it prints a SKIP line for that one
   check.

## Heresy delta

discovered: the backup sits at OOM score 0, above-guest priority in a
host OOM (a guest at +500 dies first); `IOSchedulingClass=idle` is
probably inert on NVMe's `none` scheduler; the unit's swap filled while
its memory was under its own line (host-wide reclaim, the competition with
guests). introduced: none. repaired: none in this commit (the next commit
repairs the first).

Round 2: discovered: a killed run left the previous `ok` in status.json
(freshness silent up to 36 h); a failed verify was not flagged for 10
days. introduced: none. repaired: both (e025d9121), and the drill's
contradicting limits.

## Staging evidence, 2026-10-24: the backup refuses computer starts

FP0 read Node B during and after the 02:34 run (the first under Dolt root
locked, D23). **[V]** systemd's accounting, journal and `free -m`.

- The run succeeded: 02:34:14 to 03:21:09 (46 min 55 s), MemoryPeak 10.0
  GiB (10738446336 B), 400.9 G read from disk, 66.6 G written, snapshot
  660746260ede added 41608672131 bytes.
- At 03:08, restic (`.restic-wrapped`) held 6.8 GB RSS and the unit's
  MemoryCurrent was 8.2 GB, rising to 9.2 GB at 03:11, under
  `GOMEMLIMIT=8GiB`. Host `available` was 5.2 to 6.6 GiB.
- vmctl refused all three starts FP0 asked for at 03:08:42 to 03:08:50
  (T, J1, J2: stop, then resolve) with `computer recovery blocked
  (host_memory_pressure)`, for example "host memory available 6639 MiB;
  519 MiB reserved for starts in flight; this computer needs 1491 MiB
  (measured) above the 4779 MiB floor and 1024 MiB headroom".
- After the run ended, `available` was 11.5 GiB at 03:21:22. The same three
  starts, retried at 03:21, all succeeded in 12.6 to 13.8 s.

This confirms the mechanism in "What else the run costs" above: while
the backup runs, any computer that is not already running cannot start.
That covers a wake from hibernate, a crash recovery and a first launch.
It lasts the length of the run, about 47 minutes a night at 02:34. Pressure
reclaim did not hibernate anyone (`eligible=0 protected=2`; only
protected computers were active). Tonight's refusals were FP0's own probes.
**[H]** Any user whose computer is asleep at 02:34 and is woken in the
window would be refused the same way. Before launch that is a few test
users. After launch it is a nightly outage for strangers.

The largest term in restic's memory is the trees of the three flat
directories (section above): about 9.2 M of the 9.7 M files are the
legacy flat tape (`sha256/pin-receipts`, `computer-event`,
`computer-event-payload`). The owner's recorded yes is to delete those
legacy files once the repack census shows a verified segment copy of each
(ledger 10-22 23:21, ruling 4). That delete is the substrate fix for this
cost. A lower `GOMEMLIMIT` or a later start time would only move the
cost around. Not acted on here.
