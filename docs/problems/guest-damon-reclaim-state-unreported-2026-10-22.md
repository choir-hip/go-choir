# Guest DAMON reclaim state is unreported, so a flat host RSS has two readings

Date: 2026-10-22. Station: FP8 P4 (RES-06) with FP10 RES-10.
Mutation class of this record: green. Lane: lane/fp8-damon-report.

## Problem

The paper's Firecracker holds 7.2 to 8.2 GiB of host RSS while its guest
holds 2 to 2.7 GiB of anonymous memory (ledger 2026-10-21 18:50, 2026-10-22
04:20, 05:08: procmem read MemTotal 7,942 MiB, page cache 5,227 MiB, anon
2,065 MiB). Guest page cache is the gap. RES-06 put DAMON_RECLAIM boot
arguments on two test computers (`VM_GUEST_RECLAIM_ONLY`, nix/node-b.nix).

The 13:06 reading (ledger, from `_runs/memread/busy-idle-20261022.txt`;
corrected at 14:05: both computers were reclaim-selected, so there was no
control): host RSS stayed flat for 16 idle minutes on both (2,743 to 2,753
and 2,426 to 2,432 MiB), and the selected guest's page cache fell only
1,055 to 1,024 MiB. That reading cannot tell apart:

1. **DAMON reclaim is not running** (the boot arguments did not take, the
   kernel thread never started, the watermarks kept it inactive, or
   `min_age` = 300 s plus the quota left too little time in 16 minutes);
2. **DAMON frees pages, but free page reporting does not return them**
   (freed 4 KiB pages scattered among used ones never form a free block of
   `page_reporting_order`, 2 MiB by default, so they are never reported;
   or the balloon did not negotiate reporting on that guest);
3. **pages are returned**, and the reading simply ended too early.

Nothing in the guest reports DAMON state today, and
`scripts/proofs/fp8-image-jailer.sh` step 8 says "in the guest:
/sys/module/damon_reclaim/parameters/enabled is Y and kdamond_pid >= 0" as a
`manual` line: FP0 has no guest shell, so that check has never been run.

## Evidence: what the guest kernel enables

From the 2026-10-14 reads of the guest kernel config (kernel 6.18.37;
`docs/reports/vm-density-limits-2026-10-14.md` §2,
`docs/reports/lr-guest-memory-return-2026-10-14.md` "Guest kernel"):

| Option | Value | Meaning here |
|---|---|---|
| `CONFIG_DAMON_RECLAIM` | y | built in: its parameters are boot arguments (`damon_reclaim.*`) and files under `/sys/module/damon_reclaim/parameters/` |
| `CONFIG_PAGE_REPORTING` | y | built in: `/sys/module/page_reporting/parameters/page_reporting_order` |
| `CONFIG_MEMORY_BALLOON`, `CONFIG_BALLOON_COMPACTION` | y | `/proc/vmstat` `balloon_inflate`, `balloon_deflate`, `balloon_migrate` |
| `CONFIG_VIRTIO_BALLOON` | m | loaded at boot (`boot.kernelModules`, nix/autoputer-vm.nix) |
| `CONFIG_LRU_GEN_ENABLED` | y | multi-gen LRU on |

The guest kernel moved to 6.18.55 since (ledger 13:20; flake.nix pins
`linuxPackages_6_18` with a 6.18.45 floor). The config was not re-read for
6.18.55 by this lane (UNVERIFIED that every option above is unchanged; the
route below reads whatever exists).

DAMON_RECLAIM's parameter files on 6.x kernels (mm/damon/reclaim.c and
modules-common.h; from kernel documentation, not observed on this guest):
inputs `enabled`, `commit_inputs`, `min_age`, `quota_ms`, `quota_sz`,
`quota_reset_interval_ms`, `wmarks_interval`, `wmarks_high`, `wmarks_mid`,
`wmarks_low`, `sample_interval`, `aggr_interval`, `min_nr_regions`,
`max_nr_regions`, `monitor_region_start`, `monitor_region_end`,
`skip_anon` (and on newer kernels `quota_mem_pressure_us`,
`quota_autotune_feedback`, `addr_unit`); outputs `kdamond_pid` (-1 when not
running), `nr_reclaim_tried_regions`, `bytes_reclaim_tried_regions`,
`nr_reclaimed_regions`, `bytes_reclaimed_regions`, `nr_quota_exceeds`.
Their modes are 0600 and 0400: only root reads them. The guest daemon
(`go-choir-autoputer`) runs as root with no `ProtectKernelTunables`, so it
can.

## Evidence: what the guest image sets at boot

- No unit and no sysctl touches DAMON or page reporting
  (nix/autoputer-vm.nix: `boot.kernelModules` loads `virtio_balloon` and
  `virtio_mem`; `boot.kernel.sysctl` holds only SEC-15 hardening keys).
- vmctl adds the DAMON boot arguments only for computers in
  `VM_GUEST_RECLAIM_ONLY` (internal/vmmanager/guest_reclaim.go):
  `min_age=300000000` (5 min), `wmarks_high=950`, `wmarks_mid=900`,
  `wmarks_low=5`, `skip_anon=Y`, then `enabled=Y`. Quotas stay at the
  kernel defaults (10 ms CPU and 128 MiB per second).
- vmctl adds a never-inflated balloon with `free_page_reporting: true` to
  every cold boot (`VM_BALLOON_FREE_PAGE_REPORTING=1`, nix/node-b.nix;
  internal/vmmanager/balloon.go). `page_reporting_order` is left at its
  default.

## Evidence: what the host can read today

- `GET <guest>:8085/internal/debug/pprof/procmem` (host-sourced callers
  only; internal/server/debug_procmem.go): guest meminfo (MemTotal,
  MemFree, MemAvailable, Buffers, Cached, Shmem, Slab, AnonPages) and
  per-process RSS. No DAMON, page reporting, balloon or buddy-allocator
  fields.
- `GET 127.0.0.1:8083/internal/vmctl/memory/balloon` (RES-10 step 2, host
  only): Firecracker's balloon statistics, or a reason. Statistics need
  `VM_BALLOON_STATS_ONLY` to name the computer; it is not set on Node B
  today, so every computer reads `stats_disabled`.
- Host `VmRSS` of the computer's firecracker process (`/proc/<pid>/status`).

So the host sees the input (guest page cache) and the output (host RSS) but
not the mechanism between them: whether kdamond runs, how many bytes it has
reclaimed, whether the balloon negotiated reporting, and how much guest
free memory sits in blocks large enough to be reported.

## What would separate the three readings

Read every 30 s across a burst and 20 idle minutes on one reclaim-selected
test computer:

- `kdamond_pid` and `enabled`: reading 1 if -1 or N;
- `bytes_reclaimed_regions` rising while guest MemFree rises and host RSS
  stays flat, with little free memory at or above `page_reporting_order` in
  `/proc/buddyinfo`: reading 2;
- the balloon device's negotiated features (`VIRTIO_BALLOON_F_REPORTING`,
  bit 5, in `/sys/bus/virtio/devices/virtio*/features` for the device bound
  to `virtio_balloon`): reading 2 if absent;
- host RSS falling after guest MemFree rises: reading 3.

## Next

Tests first (yellow), then the read-only fields on the existing host-only
procmem route (orange), then a probe script FP0 runs on a test computer.
No lever is built in this lane; the outcome table is in
`docs/reports/guest-memory-return-outcomes-2026-10-22.md` (written with the
probe).
