# Ledger: choir memory optimization (2026-11-01)

- 05:40Z start. Paper firecracker RSS 7.8 GiB (6.1 GiB inactive file in the guest); restic thrashing at 4G MemoryHigh; b@c wake refused (charged its 7.2 GiB peak).
- 05:39Z runtime-only cap raise on go-choir-offhost-backup (7G/8G); shard rate 84 -> ~1,700 files/s (docs/problems/offhost-backup-shard-thrashes-at-4g-2026-11-01.md).
- 05:52Z problem: a832d94db (seed deletion) failed CI's guest-security-epoch gate (guest code, epoch not raised). Fix: dd84b1f6c raises 27 -> 28 in the same push; the gate passes from the live base 9d3cbb7a4.
- 06:41Z deployed dd84b1f6c: busy_cold (d@e active, paper observe), VMCTL_ADMISSION_NEED=wake, seed deletion.
- 06:50Z paper busy_cold dry run: target 1024 MiB, reporting negotiated, refaults 0.
- 07:0xZ segment change round 2: Devin accept, DeepSeek accept, no blocking findings. Drill fix landed (silent abort, empty sample). Devin's class allowlist declined: the class set is open, and a missed class would stay loose forever.
- 07:09Z deployed 16f7b5d01 (segment rule).
- d@e holds 266 MiB inactive file, under the 512 MiB keep: it cannot prove a busy_cold act. Edge accepted; the paper flipped to active (88654a665) with per-act watch.
- Old-rule census stopped at 70 min (superseded); new-rule census started at idle IO.
- 07:34Z paper's first active act: guest reclaimed 1031 MiB, host RSS 7748 -> 7574 (17%), refaults 0. Guest free 160 MiB minutes later (refilled; kswapd steal 1.84 GiB and file refaults 791 MiB since boot). Outcome B; the paper back to observe (docs/problems/paper-busy-cold-freed-cache-refills-2026-11-01.md).
- 07:5xZ paper's state.db stream traced: the derived-key validation restarts at every boot (05:30, 06:41, 07:09, 07:24Z), hours per pass, reading og_objects bodies; no snapshot ran (docs/problems/derived-key-validation-rereads-og-objects-every-boot-2026-11-01.md). Backup finished 07:10:55Z: 4h40m, peak 5.6 GB, 26.8 CPU-hours.
- Derived-key marks (6b30bcd67 + 3e0f2997a): Devin accept, DeepSeek accept (round 2 after a timeout), no blocking findings. Accepted edges, named: two processes on one store defeat the head check (one daemon per guest); the weekly full pass waits 30 min after each start, so a faster deploy cadence defers it; the design note's case 7 and the lost durable delete are documented, not closed.
- Census listing read 390 GB for 30.6 GB of segments: digest-order record reads over btrfs zstd extents. Fix 2c2d3515e reads each segment in offset order; its proof is the next census matching the current one's counts with far fewer bytes read.
- ~10:xxZ copy-1101 (receipt 4d49c40e…): 10,858,884 records copied into segments, 24.3 GB, 16,802 appends; it read 30 GB where the old census read 512 GB. Post-copy census-1101c (e0f76416…): every eligible loose file covered; 1720 + 1720 ineligible (public non-batch), 158 unreceipted.
- 11:07–15:38Z delete-1101 --apply (verify e0f76416… -> 36f8f6bf…, healthy): deleted 10,858,631, 24.30 GB freed, kept_by_list 252 of 3,462 keep digests, refused 3,440 ineligible + 158 unreceipted, already_gone 0. Loose files left: computer-event 400, payload 1,730, pin-receipts 1,720 (3,850 = 252 + 3,440 + 158). Node B / 253G -> 218G used. Ran at nice 19, idle IO, peak about 1 GB of its 2G cap.
- 15:40Z pushed 06731745d: the off-host backup runs unsharded by default (OFFHOST_BIG_DIRS); the 2026-11-01 sharded set is forgotten whole after the cutoff.
