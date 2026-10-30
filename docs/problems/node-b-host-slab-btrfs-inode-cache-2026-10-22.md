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
`docs/archive/definitions/choir-tape-segments-a2-repack-2026-10-21.md`): new
projection-batch pins go to per-computer segments instead of one file
each. If that works, the file count stops growing. The legacy files stay
until the repack, which is not built. No reading since the flag went on
shows whether the growth stopped.

## Not known

- Today's slab size and its top caches. (Answered by S1 below.)
- Whether slab rises at the backup or GC window. (S2 is running.)
- How fast the tape's flat directories grow now. (S4 below: about 98%
  slower than on 10-16, not zero.)

The read-only commands are in the design, section 4.

## Readings (FP0, Node B, read only, 2026-10-22)

Taken as root over `ssh node-b`, commands as written in the design.
Node B was up 6 days 18 h (booted about 2026-10-15 10:50), kernel
6.18.55, so it has run a week since the 10-14 reading without a reboot.

**S1, slab now (04:47:05Z).**

```
MemTotal:       32629560 kB
MemAvailable:    4849772 kB
Cached:          4276200 kB
Slab:            1272456 kB
SReclaimable:     809364 kB
SUnreclaim:       463092 kB
    OBJS   ACTIVE  USE OBJ SIZE  SLABS OBJ/SLAB CACHE SIZE NAME
  848120   336367  39%    0.57K  30290       28     484640 radix_tree_node
  291444   180956  62%    0.99K   9420       32     301440 btrfs_inode
  288352    52365  18%    0.50K   9011       32     144176 kmalloc-rnd-06-512
   42556     7993  18%    1.00K   1336       32      42752 kmalloc-rnd-06-1k
   21178     5356  25%    2.00K   1328       16      42496 kmalloc-rnd-06-2k
   96789    94564  97%    0.19K   4609       21      18436 dentry
  466650   193861  41%    0.04K   4575      102      18300 lsm_inode_cache
   73780    34001  46%    0.23K   2170       34      17360 btrfs_extent_buffer
vm.vfs_cache_pressure = 100
```

Reading: slab is 1.21 GiB and `btrfs_inode` 294 MiB, against 9.75 GiB
and 5.9 GiB on 10-14. Slab is not near the guests' working set now. The
host is short of memory for another reason: MemAvailable was 4.6 GiB,
and the six running guests held about 22 GB RSS (vmctl's cgroup
`memory.current` 23.6 GB, hibernate record R2). That shortage refused
admission to FP0's R5 test computer (hibernate record R5).

**S2, slab across a day.** Started 04:45:15Z from FP0's Mac as a nohup
loop, `/Users/wiz/Dev/lanes/_runs/res22/s2-slab-loop.sh` (PID 63419),
every 10 minutes for 26 h, ending about 2026-10-23 06:45Z, writing
`/Users/wiz/Dev/lanes/_runs/res22/slab-2026-10-22.log`. First sample:

```
2026-10-22T04:45:15Z
MemAvailable: 5818476 kB Slab: 1273240 kB SReclaimable: 810448 kB btrfs_inode_bytes=296139616
```

Reading: none yet; the log is read against S5's windows when it ends.

**S3, btrfs chunks (04:47:14Z).**

```
    Device unallocated:		       50466914304
Data,single: Size:419841441792, Used:308789510144 (73.55%)
Metadata,DUP: Size:20018888704, Used:16479322112 (82.32%)
```

`go-choir-btrfs-headroom`, with timestamps (`-o short-iso`):

```
2026-10-20T00:26:48 btrfs unallocated: 20 GiB
2026-10-20T00:26:51 btrfs unallocated after balance: 40 GiB   (relocated 20 of 444 chunks)
2026-10-21T00:06:14 btrfs unallocated: 44 GiB
2026-10-22T00:31:35 btrfs unallocated: 47 GiB
```

Reading: 47.0 GiB unallocated, growing about 3 GiB a day since the one
balance on 10-20. Metadata is 15.35 of 18.64 GiB (82.3%, DUP), 2.7
points under the 85% line the alert slice proposes, so that alert would
not fire today but has little margin. Chunks are not the limit now.

**S4, tape file growth, first read (04:45:33Z).**

```
/var/lib/go-choir/platform-artifacts/sha256/pin-receipts 672671064
/var/lib/go-choir/platform-artifacts/sha256/computer-event 311991424
/var/lib/go-choir/platform-artifacts/sha256/computer-event-payload 311954432
/var/lib/go-choir/platform-artifacts/sha256/og 1305192
```

Name lengths (one `ls -f` entry each): 69 for `pin-receipts` (`.json`),
64 for `computer-event` and `computer-event-payload`, 68 for `og`
(`.bin`). So the entries are about 4,874,000 pin receipts, 2,437,000
events, 2,437,000 payloads and 9,600 `og` objects. Against the 10-16
23:00 `du --inodes` counts (3,883,975, 1,932,281, 1,948,350, 575,912),
that is about 990,000 receipts and 505,000 events in 5.2 days, about
190,000 receipts a day against 730,000 a day on 10-16. The two methods
differ (`du --inodes` counts subdirectories too), so this says the growth
slowed, not by how much. `og` shrank, as the artifact GC deletes in
that namespace.

**S4, second read (06:42:53Z), 1 h 57 min after the first.**

```
/var/lib/go-choir/platform-artifacts/sha256/pin-receipts 672807546
/var/lib/go-choir/platform-artifacts/sha256/computer-event 312054784
/var/lib/go-choir/platform-artifacts/sha256/computer-event-payload 312017664
/var/lib/go-choir/platform-artifacts/sha256/og 1339872
```

Net entries added in the interval (Δsize / (2 × name length)):
`pin-receipts` +989 (136,482 B / 138), `computer-event` +495 (63,360 /
128), `computer-event-payload` +494 (63,232 / 128), `og` +255 (34,680 /
136). Scaled to a day: about 12,100 receipts and 6,070 events, against
730,000 receipts and 366,000 events a day on 10-16.

Reading: tape segments took over most of the growth, about 98%, but not
all of it. The three tape directories still gain about 500 events (one
receipt each for the event and its payload) every 2 hours, so some
event kinds still write one legacy file each. Which kinds is the tape
segments goal's question, not this record's. The interval is 2 hours,
not the design's 24, so the daily figure is an extrapolation from a
night with few users.

**S2 so far (04:45 to 06:35Z, 12 samples).** Slab 1.07 to 1.21 GiB,
SReclaimable 0.65 to 0.77 GiB, `btrfs_inode` 200 to 282 MiB, MemAvailable
3.3 to 13.9 GiB. Slab drifted down slightly; no window has passed yet
that the S5 walkers would explain (the next backup is 2026-10-23
02:35Z). The full reading waits for the log's end.

**S5, the walkers' windows (04:47:15Z).** `systemctl list-timers --all | grep go-choir`
(spaces squeezed, the ACTIVATES column dropped):

```
Thu 2026-10-22 04:47:32 UTC 17s Thu 2026-10-22 04:42:32 UTC 4min 42s ago go-choir-paper-liveness-probe.timer
Thu 2026-10-22 04:48:14 UTC 59s Thu 2026-10-22 04:47:14 UTC 854ms ago go-choir-checkpointd.timer
Thu 2026-10-22 04:48:56 UTC 1min 41s Thu 2026-10-22 04:38:54 UTC 8min ago go-choir-front-importance.timer
Thu 2026-10-22 08:34:16 UTC 3h 47min Thu 2026-10-22 04:34:31 UTC 12min ago go-choir-nix-gc.timer
Thu 2026-10-22 10:39:03 UTC 5h 51min Thu 2026-10-22 04:39:03 UTC 8min ago go-choir-offhost-backup-freshness.timer
Thu 2026-10-22 12:00:00 UTC 7h Wed 2026-10-21 12:00:01 UTC 16h ago go-choir-digest.timer
Fri 2026-10-23 00:11:40 UTC 19h Thu 2026-10-22 00:21:10 UTC 4h 26min ago go-choir-platform-dolt-history-audit.timer
Fri 2026-10-23 00:30:09 UTC 19h Thu 2026-10-22 00:31:35 UTC 4h 15min ago go-choir-btrfs-headroom.timer
Fri 2026-10-23 00:34:39 UTC 19h Thu 2026-10-22 00:26:20 UTC 4h 20min ago go-choir-disk-gc.timer
Fri 2026-10-23 02:35:28 UTC 21h Thu 2026-10-22 02:44:34 UTC 2h 2min ago go-choir-offhost-backup.timer
Fri 2026-10-23 04:00:00 UTC 23h Thu 2026-10-22 04:00:00 UTC 47min ago go-choir-store-compaction.timer
Sun 2026-10-25 06:11:24 UTC 3 days - - go-choir-offhost-backup-verify.timer
- - Thu 2026-10-22 04:47:14 UTC 117ms ago go-choir-dolt-liveness-probe.timer
```

Reading: the daily walkers run 00:2x to 00:3x (disk GC, Dolt history
audit, headroom), 02:35 to 02:45 (off-host backup) and 04:00 (store
compaction); nix GC runs every 4 h. The hourly artifact GC is not a
timer: it runs inside `go-choir-corpusd` (`PLATFORM_ARTIFACT_GC_INTERVAL=1h`),
and its clock restarts with each deploy, so tonight's passes were at
02:28, 02:41, 03:01, 03:49, 04:28 and 04:39. The four-hourly disk
retention sweep was not located as a timer either (vmctl's retention
prune is in-process). S2's log is read against these windows.
