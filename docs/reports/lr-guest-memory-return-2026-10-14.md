# LR: Guest Memory Return and Memory Bounds

Date: 2026-10-14. Station: LR cost track, items 2 and 3
([cost-track investigation](cost-track-sa-s3-sp-investigation-2026-10-14.md)).
Asked by the owner for cost levers that raise agent density per host.
Mutation class: orange to red (Firecracker device config, guest kernel
modules, guest process memory policy). Each lever has its own one-line
rollback.

This note is written before the code. It names the facts, the failure
modes, and the staging proof. The code is not proven until that proof runs.

## The three levers

1. **Free page reporting** (virtio-balloon, host side). The guest kernel
   reports freed 2 MiB blocks to Firecracker, which drops them with
   `madvise(MADV_DONTNEED)`. Host RSS of the Firecracker process falls when
   the guest frees memory. Without it, the host keeps every page the guest
   ever touched until the guest stops. This is the multiplier on the other
   two levers.
2. **A soft Go memory limit** (`debug.SetMemoryLimit`) in the guest daemon
   and in each desk-session worker, derived from guest RAM. It makes the Go
   GC collect, and the scavenger return memory to the guest kernel, before
   the heap reaches GOGC's 2x target, once the heap nears the limit.
3. **A Dolt cache budget.** Off by default. Dolt sizes its node cache,
   memtable and decoded-chunk cache from the Go memory limit once, at first
   use. A separate knob gives the Dolt caches their own budget without
   lowering the daemon's limit.

## Verified facts

Firecracker (staging runs `Firecracker v1.15.1`, read on node-b).
Sources: the v1.15.1 tag of `firecracker-microvm/firecracker`.
- `src/vmm/src/vmm_config/balloon.rs`: `BalloonDeviceConfig` has
  `amount_mib`, `deflate_on_oom`, `stats_polling_interval_s`,
  `free_page_hinting`, `free_page_reporting` (`deny_unknown_fields`, so a
  misspelled field fails the boot).
- `src/vmm/src/resources.rs`: the `--config-file` JSON (`VmmConfig`,
  kebab-case) takes a top-level `"balloon"` object. No API call is needed,
  so it works with `--no-api`.
- `CHANGELOG.md`, v1.14.0: "Added support for virtio-balloon free page
  reporting and hinting" (#5491), then called a developer preview. In the
  v1.15.1 `docs/ballooning.md` only hinting carries the developer-preview
  mark; reporting does not. Hinting needs the API (host-initiated runs) and
  has a documented corruption race; we do not use it.
- `src/vmm/src/device_manager/mod.rs` `attach_virtio_device`: with PCI
  enabled every virtio device, the balloon included, attaches as
  virtio-pci. So `--enable-pci` is compatible; the guest needs
  `virtio_pci`, which it already loads for its disks.
- `docs/ballooning.md`: reporting is set before boot only and runs
  continually. The guest driver reports after about 2 s, in ranges of at
  least `page_reporting_order` (default: pageblock order, 2 MiB on x86_64).
- v1.15.1 fixed (#5794) a guest-controlled oversized stats descriptor that
  could stall the VMM loop; only relevant with stats polling on.

Guest kernel (staging guest image built from `66fe0917`; read on node-b
from the manifest's `kernel_config`, `linux-config-6.18.37`):
- `CONFIG_MEMORY_BALLOON=y`, `CONFIG_BALLOON_COMPACTION=y`,
  `CONFIG_PAGE_REPORTING=y`, `CONFIG_VIRTIO_BALLOON=m`,
  `CONFIG_VIRTIO_PCI=m`.
- `virtio_balloon.ko.xz` is in the guest system's `kernel-modules` tree.
  udev should load it by modalias when the device appears; the guest
  configuration now also lists it in `boot.kernelModules` so it loads even
  if that does not happen.

Dolt (`dolthub/dolt/go v0.40.5-0.20260716231223-6bea93f2f665`, embedded via
`dolthub/driver/v2 v2.2.0`):
- `libraries/doltcore/memlimit/memlimit.go`: `Init` reads the Go memory
  limit once (`sync.Once`) and gives 50% of it to three caches at 47/47/6:
  the shared prolly node cache, the NBS memtable and the decoded-chunks
  cache. Each is clamped to its default (256 MiB, 256 MiB, 32 MiB) as a
  maximum and to 16, 4 and 4 MiB as a minimum. With no limit set, the
  defaults apply.
- So the caches shrink only when the limit is below about 1.09 GiB. A 4 GiB
  daemon limit leaves Dolt exactly as it is today.
- The driver exposes no cache parameter: `openEngineWithRetry` passes only
  `disable_singleton_cache` and `fail_on_journal_lock_timeout`. The journal
  store uses the fixed `defaultMemTableSize`. `pool.BuffPool` is a plain
  `make([]byte, n)`: the "buffer pool" heap in profiles is node memory held
  by the node cache and by live trees, not a separate pool.

## Chosen values

- Balloon: `amount_mib: 0` (never inflated; reporting only),
  `deflate_on_oom: true` (no effect at 0, safe if a later caller inflates),
  `free_page_reporting: true`, stats polling off unless
  `VM_BALLOON_STATS_POLLING_INTERVAL_S` is set (with `--no-api` nobody can
  read stats; stats cannot be turned on after boot, so the knob exists for
  when the API socket lands). Gate: `VM_BALLOON_FREE_PAGE_REPORTING`, off in
  code, on in `nix/node-b.nix`.
- Daemon soft limit: half of guest `MemTotal`, at least 512 MiB (about 4 GiB
  on an 8 GiB guest). The paper computer's main process measured Sys 1.46 GB
  and HeapInuse 1.05 GB, so 4 GiB is about 2.7 times its footprint: the
  limit does not bind at today's working set and cannot cause GC thrash
  there. It binds only when the heap would grow past half the guest, the
  region where the page cache and workers are squeezed. On a 4 GiB guest it
  is 2 GiB, still above the measured heap.
- Desk worker soft limit: 1/16 of guest `MemTotal`, at least 256 MiB
  (512 MiB on 8 GiB). Workers are not measured yet. At most 12 idle plus the
  busy ones (4 research children per call): about 16 workers at 1/16 each
  sum to one guest. A cell that holds more than the limit live slows (the
  GC uses more CPU, capped near 50% by the runtime's GC CPU limiter); it is
  not killed. The hard bound stays `RLIMIT_AS` 8 GiB.
- `GOMEMLIMIT` in the environment always wins; the code then does nothing.
  `GOMEMLIMIT=off` is the rollback for the daemon (guest image env line).
  Workers get a clean environment, so `CHOIR_DESK_WORKER_GOMEMLIMIT` in the
  daemon's environment is passed to each worker as `GOMEMLIMIT` (validated
  first: a malformed `GOMEMLIMIT` makes the Go runtime refuse to start).
- Dolt cache budget: `CHOIR_DOLT_CACHE_NODE_MIB`, unset by default (today's
  behavior). When set, the daemon sets a temporary memory limit that makes
  Dolt's own partition give the node cache that many MiB, calls Dolt's
  `memlimit.Init`, then sets its real limit. Clamped to Dolt's 16 to 256 MiB
  node range. Not enabled anywhere by this change.

## Failure modes

Balloon / free page reporting:
- **Boot breaks.** A field Firecracker rejects fails every cold boot on the
  host (`deny_unknown_fields`). Mitigation: field names are taken from the
  v1.15.1 source; the gate is off in code. Rollback: delete the
  `VM_BALLOON_FREE_PAGE_REPORTING` line in `nix/node-b.nix` and redeploy
  vmctl. Running guests keep the config they booted with; the change only
  reaches a computer at its next cold boot.
- **Driver does not bind.** Reporting is silently inert: no harm, no gain.
  Detected by the probe (host RSS does not fall; guest has no
  `/sys/bus/virtio/drivers/virtio_balloon/virtio*`).
- **CPU and latency cost.** Freed 2 MiB blocks are reported and dropped; a
  guest that reallocates them takes fresh page faults (zeroed pages).
  Churny workloads pay this repeatedly. The 2 MiB minimum limits it. Watch
  host CPU and desk turn latency in the probe.
- **Host overcommit.** Lower RSS lets the pressure gate admit more
  computers. Guests that then spike together can exceed host RAM: each guest
  can still touch its full `mem_size_mib`. Nothing here changes admission;
  the density gain must be taken deliberately, after the probe.
- **Snapshots (S3, concurrent work).** The balloon is part of the device
  state saved in a snapshot. A snapshot restores whatever balloon it was
  taken with; flipping the env does not change a restored computer until it
  cold-boots. Reporting should shrink what a snapshot holds (fewer resident
  pages); unmeasured. Huge-page backing would defeat 4 KiB `MADV_DONTNEED`;
  we do not use huge pages.
- **PCI transport.** Balloon over virtio-pci in Firecracker is newer than
  over MMIO. Only the staging probe proves it on our host.

GOMEMLIMIT:
- **GC thrash near the working set.** If a process's live heap sits near its
  limit, the GC runs nearly continuously (up to the 50% CPU cap). The daemon
  limit sits at 2.7 times the measured heap. The worker limit is a guess
  until procmem measures workers.
- **OOM.** A soft limit cannot cause an OOM kill; it can only make the GC
  work harder. Too high a limit does nothing.
- **Bad env value.** A malformed `GOMEMLIMIT` aborts the Go runtime at
  start. The worker pass-through is validated; the daemon's own env is the
  operator's (guest image) responsibility.
- **Host-process mode** (no Firecracker, Linux): `MemTotal` is the host's,
  so the soft limit is half the host. Harmless; on macOS there is no
  `/proc/meminfo` and no limit is set.

Dolt cache budget:
- **Order.** Dolt reads the limit once, at first use. The call is the first
  thing `autoputer.Run` does, before `MaybeRunDoltGC` and `store.Open`. A
  future change that touches Dolt earlier would make the knob silently
  inert; the boot log line names the budget Dolt actually took.
- **Slower reads.** A smaller node cache means more chunk reads through the
  page cache and disk. Boot replay is the read-heaviest phase; a smaller
  cache can slow it and push it toward `VM_REPLAY_STALL_TIMEOUT`. That is why
  the knob is off by default and enabled only for the probe.
- **The transient limit.** Between the two `SetMemoryLimit` calls the limit
  is low (about 70 to 1,100 MiB). At process start the heap is a few MiB,
  so no GC is forced.

## Staging proof plan

All on minted test computers, never on the paper or owner computer.

1. Deploy. Confirm vmctl's env carries `VM_BALLOON_FREE_PAGE_REPORTING=1`
   and a new computer's `fc-config.json` (in its vm-state directory) has the
   `balloon` object.
2. Mint test computer A (reporting on). Cold boot. Confirm boot health and
   that the guest bound the driver: procmem should show unchanged health,
   and the guest boot journal names `virtio_balloon`.
3. Baseline: host RSS of A's Firecracker process (`VmRSS` of the PID in
   `firecracker.pid`), and guest `procmem`
   (`GET /internal/debug/pprof/procmem`, host-sourced) after boot settles.
4. Burst: drive memory up inside the guest (a research run with several
   children, or a capsule `go build`), then read both again at peak.
5. Idle 10 minutes (no runs). Read both again. Expected with reporting on:
   host RSS falls toward guest `MemTotal - MemFree`, well below the peak.
6. Control: set `VM_BALLOON_FREE_PAGE_REPORTING=0` on a test deploy (or use
   a computer booted before the change), repeat 2 to 5 on computer B.
   Expected: host RSS stays near the peak.
7. GOMEMLIMIT: the daemon's boot log line `autoputer: gomemlimit
   role=daemon mem_total_mib=… limit_mib=… desk_worker_limit_mib=…
   dolt_budget=default dolt_node_cache_mib=256` names its limit, the limit
   each worker computes from the same `MemTotal`, and the Dolt budget
   (workers log nothing: their stderr is a buffer read only on spawn
   failure). procmem shows main and worker RSS before and after the burst;
   compare with the previous deploy's numbers in the LR reports.
8. Optional Dolt probe on a third test computer with
   `CHOIR_DOLT_CACHE_NODE_MIB=64` in the guest env (needs a guest image
   built with that env): compare boot replay time and main-process RSS.
9. Record: computer ids, PIDs, the four RSS reads per computer, guest
   meminfo, desk turn latency, host load. The lever is proven when idle
   host RSS with reporting on is materially below the control at equal
   guest usage, with no boot or latency regression.

## What stays unverified until staging

- That the balloon attaches over PCI on our host kernel and Firecracker
  build, and the guest driver binds.
- The actual reclaim and CPU cost of reporting under our workload.
- Worker working sets (the worker limit is a reasoned default, not a
  measurement).
