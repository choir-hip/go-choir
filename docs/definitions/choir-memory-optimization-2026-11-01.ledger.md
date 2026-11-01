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
