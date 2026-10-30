# VM Density: How Many Computers Fit on a Host, and What Raises It

Date: 2026-10-14, about 15:00 UTC. Owner question (about 15:00 UTC):
"you're saying 90 VMs on 128 gigabytes of RAM. I think we can do better
than that. Wasn't one of the ideas to sparsify snapshots ...? When we get
all our optimizations, what's the real limit of the number of VMs on a
machine?"

Mutation class of this report: green (docs only). It changes nothing. Each
lever names its own class.

Sources:
- today's load and density run on Node B (session scratchpad
  `perf/load_density_summary.md`, raw JSON beside it);
- the token read (`perf/tokens.md`);
- read-only Node B reads at 14:56 to 15:04 UTC: `/proc/meminfo`,
  `/proc/pressure/*`, `/proc/<firecracker>/status` and `stat`, slabinfo,
  `ls -ls` of each `data.img`, the vmctl journal, live `fc-config.json`
  balloon entries, the guest kernel config, and the vmctl unit's
  capabilities;
- the code at `da9de233f` and Firecracker v1.15.1 (the CHANGELOG, the
  snapshot, pmem and memory-hotplug docs, and the API swagger).

No load was run for this report and no computer was booted. Every number
says whether it was measured or estimated. The estimates are what the
benchmark at the end has to replace.

**Lever status, 2026-10-21** (code at 8f71f9c64; tracked in the
[checklist](../security-density-architecture-checklist-2026-10-21.md)): done: L1, L2. Partial: L3b (FIEMAP bound,
unverified on staging), L6 (`GOMEMLIMIT`; Dolt cache knob off), L7
(scheduled compaction), L8 (fstrim; discard waits for Firecracker 1.17),
L11 (lease controller on test computers, no staging proof, no small base),
L17 (jailer leaf without `memory.max`, jailer off), L19 (artifact GC).
Not started: L3, L4, L5 (in-guest reclaim), L9, L10, L12, L16, L18, L20,
L21. Rejected: L13, L14, L15; host KSM is still not explicitly off. The
benchmark in §4 has not been run. Finding 3's page cache is on the shared
store image only if a guest `fincore` read says so (RES-07). New: the
jailer copies the store image per VM
([jailer-copies-store-image-per-vm-2026-10-21.md](../problems/jailer-copies-store-image-per-vm-2026-10-21.md)).
Versions moved since: Firecracker 1.16.1, guest kernel 6.18.55.

## The short answer

Today's "90 on 128 GB" counts idle computers kept running at 1.21 GiB
each. That is the wrong unit for two reasons.

1. **Idle computers should not be running.** A snapshot wake already takes
   0.34 to 0.52 s to healthy. Once a snapshot is cheap to write, an idle
   computer should hibernate after a few minutes, and then it costs disk,
   not RAM.
2. **What a host really runs out of is busy computers.** Busy computers
   cost CPU, model calls and memory that grows with work. On the EPYC 7371
   the 16 Zen 1 cores bind at about **20 busy computers** (range 15 to
   30). RAM stops being the limit once a busy computer holds about 3 GiB
   instead of its full 8 GiB.

Headline limits for the EPYC 7371 with 128 GB, after the quick and medium
tiers below:

| limit | today | after all tiers | what binds |
|---|---|---|---|
| (a) registered users (disk) | ~500 | ~2,000 on the host; no host limit with an off-host cold tier | per-user disk |
| (b) idle computers running at once | ~76 | ~360 (medium), ~550 (hard) | RAM, then idle CPU and vmctl's per-VM costs |
| (c) busy computers at once | ~12 | ~20 (15 to 30) | CPU (16 cores), and before that model accounts and dollars |
| (d) wakes | ~5 to 10 a second, estimated | 10 to 20 a second | snapshot page-in and corpusd credential writes; after a deploy, cold boots at ~1 a second |

For an interactive population (1.5% of registered users busy at the peak
hour, an assumption explained in section 3), one EPYC 128 GB serves
**about 1,200 to 2,000 registered users**. 256 GB does not raise the busy
limit, because the same 16 cores bind first. It only raises idle and
registered capacity, which are already not binding.

Sparse snapshots answer a different problem than the owner expected. A
hibernated computer already holds no RAM. Its snapshot already uses only
about 560 MB of real disk, because btrfs zstd compresses the untouched
pages. What sparse snapshots fix is the **8 GiB write** behind every
hibernate (7.6 s of I/O, which pushes memory PSI to 2.5 and set off the
reclaim cascade today) and the snapshot budget. The RAM win comes from
three other levers: admission by measured use, guest page-cache reclaim,
and (hard) virtio-mem.

## New findings in this pass

1. **The snapshot budget still counts 8 GiB per snapshot.** The btrfs
   measure falls back to `st_blocks`. vmctl's unit bounds its capabilities
   to `cap_net_admin cap_sys_ptrace`, so the extent search (which needs
   `CAP_SYS_ADMIN`) cannot run. The 13:33 hibernate logged
   `disk_bytes=8590024704 disk_bytes_method=st_blocks`. So 48 GiB admits
   about 5 warm snapshots, not about 70. Documented first in
   [the snapshot budget problem](../problems/snapshot-budget-falls-back-to-logical-bytes-2026-10-14.md).
2. **Free page reporting is on, but it cannot return page cache.** The live
   paper computer was cold-booted at 14:27 with
   `"balloon":{"amount_mib":0,"free_page_reporting":true,...}` in its
   `fc-config.json`. Thirty-four minutes later it held 7.85 GB RSS: its
   whole guest. Reporting only returns pages that the guest has *freed*.
   A Linux guest fills free memory with page cache (here, Dolt store
   reads), and page-cache pages are not free. So this is why the guest
   keeps 8 GiB: not leaked anon memory, but a cache that grows to the
   ceiling because nothing bounds it.
3. **An idle guest is mostly kernel reservation and page cache, not
   processes.** Idle test computers (procmem, n=3) showed AnonPages 80 to
   132 MiB, page cache 640 MiB and slab 57 MiB. 248 MiB of the 8 GiB never
   shows in guest `MemTotal` (memmap and reserved memory, touched at boot).
   After a snapshot wake, the Firecracker process held 529 MiB, of which
   only **126 MiB was anonymous**. The other 403 MiB was clean pages mapped
   from the snapshot file, which the host can evict and read back. So the
   true private floor of an idle computer is about 0.13 GiB plus kernel
   overhead.
4. **Host slab is large.** `btrfs_inode` slab was 5.9 GiB and total slab
   9.75 GiB (8.4 GiB reclaimable). The likely cause is the artifact store's
   file count, plus `du` scans. It is reclaimable, but it competes with
   guests and feeds direct reclaim.
5. **KSM is switched on but does nothing.** `/sys/kernel/mm/ksm/run`=1 and
   `pages_sharing`=0, because Firecracker never marks guest memory
   mergeable. It costs nothing and saves nothing.
6. **Admission does not reserve.** `admitHostMemory`
   (`internal/vmctl/host_capacity.go:249-291`, called at
   `ownership.go:1353` and `:2009`) compares one sample against
   floor + 8 GiB, and nothing records the boots already in flight. Today
   the 8 GiB over-reservation hides this. Once admission counts measured
   use, concurrent wakes would all pass the same check.
7. **The data disk only grows.** Firecracker's virtio-blk has no
   discard (the v1.15.1 swagger has no discard field). Blocks the guest
   frees stay allocated in `data.img`. The paper computer's image is
   24.5 GB allocated now; on 10-13 the state dir read 6.9 GB (that read's
   method is not recorded, so the growth rate is unconfirmed). An offline
   compaction once took the owner's image from 17.7 GB to 4.5 GB
   ([store bloat](../archive/problems/guest-store-history-bloat-and-memory-shape-2026-10-09.md)).
8. **Busy CPU, first number.** The paper computer (14 sources, busy 24/7)
   used 2,463 CPU-seconds in 2,042 s of life: **1.2 cores on average**.
   That includes its boot replay. This is one sample, with a Xeon core.

## Hosts compared

| | Node B (today) | EPYC 7371, 128 GB | EPYC 7371, 256 GB |
|---|---|---|---|
| CPU | Xeon E-2236, 6c/12t, 3.4 to 4.8 GHz (Coffee Lake) | 16c/32t Zen 1, 3.1 to 3.8 GHz, 4 dies (4 NUMA nodes), 8 CCX of 2 cores / 4 threads / 8 MB L3 | same |
| RAM | 31.1 GiB, single node | ~125.5 GiB usable, 4 × ~32 GB nodes | ~251 GiB, 4 × ~64 GB nodes |
| disk | 2 × 512 GB NVMe RAID1, btrfs zstd:3, 157 GB free (67% used) | 2 × 960 GB NVMe | same |
| guest budget, today's settings | ~22 GiB (31.1 − 2.8 services − ~1.6 kernel − 4.67 floor) | ~100 GiB (15% floor = 18.8 GiB) | ~206 GiB (15% floor = 37.7 GiB) |
| guest budget, tuned floor | ~22 GiB | ~109 GiB (8 GiB floor, 4 GiB burst pool) | ~229 GiB |

Zen 1 runs about 15 to 25% fewer instructions per clock than Coffee Lake,
at a lower clock. A busy computer that uses 1 Xeon core is taken as 1.3
Zen 1 threads. If a security review turns SMT off for tenant isolation,
the EPYC has 16 threads and every busy number below roughly halves.

## 1. The four limits, separated

### (a) Registered users: disk, not RAM

A hibernated or stopped computer holds no RAM. It holds:
- its `data.img`. Read 2026-10-14 by `ls -ls`: 27 images. A fresh computer
  allocates 536 to 539 MB, which is the seeded git checkout plus ext4
  metadata (`createDataImage`, `internal/vmmanager/manager.go:2664-2760`).
  Lightly used test computers hold 0.6 to 3.7 GB (median about 0.67 GB),
  the owner's computer 7.4 GB, and the paper 24.5 GB. These are `st_blocks`
  values, the size before compression. Real disk is lower by btrfs zstd's
  ratio on ext4/Dolt data (unmeasured: compsize is not installed).
- a snapshot, only if it hibernated in the last 36 h
  (`VM_SNAPSHOT_MAX_AGE=36h`): about 560 MB real (compsize, earlier
  today), counted as 8 GiB by the budget (finding 1).
- console logs, about 5 MB.

Working figure: **about 1 GB per dormant user today**, plus 0.56 GB while
it is warm.

Usable disk:
- Node B: 157 GB free, less the 32 GiB state-dir floor, so about 120 GB.
  The artifact store takes 12 GB a day of that
  ([problem](../problems/artifact-store-grows-12gb-a-day-2026-10-14.md)),
  so the platform, not computers, is what fills Node B.
- EPYC: about 890 GB on RAID1. If it also carries the platform (Dolt
  stores, nix store, builder, artifacts, about 250 GB on Node B today),
  about 600 GB is left for computers.

| | Node B | EPYC (either RAM size) |
|---|---|---|
| today (1 GB per user; 30% warm at 0.56 GB) | ~100 more users | ~500 |
| quick tier: reflinked seed image, sparse snapshots | ~250 | ~1,200 |
| medium tier: automatic store compaction, re-sparsified data images | ~400 | ~2,000 |
| with an off-host cold tier (SP3) for users dormant > 30 days | host holds recent users only | registered users have no host limit |

Today the warm-snapshot count is the sharper limit. The budget admits
about 5 warm snapshots (finding 1). The sixth hibernation evicts the
oldest snapshot, and that computer cold-boots at its next wake.

### (b) Idle computers running at once: RAM per idle VM

Measured:
- fresh cold boot, 3 min idle: 1.21 GiB host RSS (n=3, spread under 0.5%);
  guest used 0.92 GiB; guest processes 370 MiB, of which anon is 80 to
  132 MiB;
- after a snapshot wake: 0.19 GiB at once, 0.41 GiB at 30 s, 0.51 GiB at
  5 min, of which 0.12 GiB is anon;
- a used computer stays at its high-water mark: vm-48bc 1.3 to 1.5 GiB,
  the owner 2.7 to 4.4 GiB, the paper 7.85 GiB.

Today, admission needs floor + 8 GiB available for every unprotected boot
or wake. So the running count stops at (guest budget − 8 GiB) / RSS per VM:

| idle RAM per VM | Node B (empty host) | EPYC 128 | EPYC 256 |
|---|---|---|---|
| today, fresh, 1.21 GiB | 11 | 76 | 164 |
| today, used computers, ~2 GiB | 7 | 46 | 99 |
| quick tier, ~0.5 GiB | 44 | 218 | 458 |
| medium tier, ~0.3 GiB | 73 | 363 | 763 |
| hard tier, ~0.2 GiB | 110 | 545 | ~800, CPU-bound |

The "90" is the first row with a smaller reserve. Node B today also carries
the paper and the owner computer (about 11 GiB), which leaves 2 to 3 more
unprotected computers. The run hit exactly that: the third test computer
put the host under the admission line.

Above about 500 VMs, other per-VM costs bind before RAM:
- idle CPU (unmeasured; an estimated 0.01 to 0.02 core each from guest
  timers, the Go runtime and 15 s health checks; 800 VMs would use 8 to 16
  threads);
- one iptables DNAT and FORWARD rule set per VM;
- vmctl's single registry mutex and its health-probe fan-out.

The quick tier makes this row matter less. With hibernate after a few
idle minutes, the running set is the users who are online right now.

### (c) Busy computers at once: working set, CPU, models, I/O

Measured:
- a prompt-bar burst on a test computer: 8 runs at once in every sample
  (the dispatcher default `MaxConcurrent = 8`,
  `internal/actor/dispatcher.go:176`; that this is the cap is a
  hypothesis). RSS went from 0.5 to 2.1 GiB in 4 minutes and was still
  rising 120 MiB per 30 s. About 142 MiB per added desk worker.
- a sustained busy computer (the paper): 7.85 GiB at 34 minutes (the
  ceiling), 1.2 cores on average.
- model calls are I/O-bound: p50 about 3 s, p90 10 to 15 s, and no latency
  change within the 4-minute run.

Working set per sustained busy computer: 8 GiB today (the ceiling). About
3 GiB in the quick tier (page cache bounded, Dolt cache budget, worker
idle budget). About 2 GiB in the medium tier (store compaction, shared
store). About 1.3 GiB in the hard tier. The tier figures are estimates.

CPU: about 1 Xeon core per sustained busy computer, taken as 1.3 Zen 1
threads. A host is filled to 70% of its threads, after 1.5 to 2 threads
for the platform.

| | Node B | EPYC 128 | EPYC 256 |
|---|---|---|---|
| today (RAM: 8 GiB each) | 2 to 3 (the paper and the owner already fill it) | 12 (RAM) | ~18 (CPU binds before RAM's 25) |
| quick tier (RAM 3 GiB; CPU) | 7 (RAM and CPU meet) | ~18 (CPU) | ~18 (CPU) |
| medium tier (+ NUMA placement, ~10% CPU) | 7 to 8 | ~20 | ~20 |
| hard tier (less guest CPU per run) | 8 to 10 | 25 to 30 | 25 to 30 |
| with SMT off | about 5 | 10 to 15 | 10 to 15 |

Before CPU, two limits outside the host bind:
- **Model accounts.** One paper burst already drew 54 retries an hour and
  429s on two providers. Twenty busy computers make 20 to 80 model calls at
  once (1 to 4 in flight each). Per-account rate limits bind before the
  host does.
- **Dollars.** A 14-source paper costs about $109 a day at list rates. So
  20 paper-like busy computers cost about $2,200 a day, and the host's busy
  limit is also a budget decision. Search providers are cooling for most
  of the hour already
  ([problem](../archive/problems/search-providers-cooling-most-of-the-time-2026-10-14.md)).

I/O: busy computers write Dolt commits through ext4, then btrfs zstd, then
RAID1. Per-VM I/O is unmeasured, because there is no per-VM cgroup (lever
L17). The largest single I/O event today is the hibernate's 8 GiB write.

### (d) Wake latency and admission throughput

Measured (n=2 snapshot wakes, n=3 cold boots):

| phase | snapshot wake | cold boot |
|---|---|---|
| host prep (validate, credential issue, consume + fsync, spawn) | 64 to 159 ms | data image 2.9 to 3.2 s (git clone + mkfs for a new computer) |
| load / guest boot | 6 to 7 ms load | ~7.4 s guest boot |
| guest `resumed` (clock step, credential exchange, consume, `key_revoked` append) | 240 to 315 ms | — |
| to healthy | **338 to 515 ms** (API 0.69 to 0.83 s) | **10.4 to 10.7 s** |
| hibernate | 7.6 to 8.1 s, 8 GiB write, PSI to 2.5 | — |

Throughput (estimated; the benchmark must measure it):
- **Snapshot wakes.** Each wake reads 0.2 to 0.4 GiB of snapshot pages in
  its first 30 s. The reads come through btrfs zstd, which decompresses a
  128 KiB extent per fault. Each wake also makes three corpusd writes
  (issue, consume, `key_revoked`) on the platform Dolt. Estimate: Node B
  3 to 5 wakes a second, EPYC 10 to 20.
- **Hibernates** are one at a time today: each writes 8 GiB and can set
  off the next reclaim. That is about 0.1 a second, safely. With sparse
  snapshots, about 1 a second.
- **Cold boots.** About 0.3 to 0.5 a second on Node B and 1 to 1.5 on the
  EPYC (CPU-bound guest boots). The reflinked seed takes 3 s out of a new
  computer's boot.
- **The deploy storm.** A deploy that rewrites guest artifacts
  invalidates every snapshot (S3 failure modes 7 and 8). For the next
  36 h every wake is a cold boot, so the wake rate falls to the cold-boot
  rate. This, not the wake path, is the realistic throughput limit (lever
  L20).

Demand side: 2,000 registered users at 30% daily activity is 600 wakes
spread over a day, perhaps 100 in the peak hour. That is far below any of
these rates. Wake throughput binds only in a deploy storm or a mass
reconnect after an outage.

## 2. The optimization ledger

Class: G green, Y yellow, O orange, R red. Effort: XS (hours), S (1 to 2
days), M (a week), L (several weeks), XL (a month or more).

| id | lever | mechanism | expected saving | effort | risk / class | evidence | code |
|---|---|---|---|---|---|---|---|
| L1 | Admission by measured use, with in-flight reservation | need = expected working set (wake p95 ~0.6 GiB, cold ~1.3 GiB), plus a shared burst pool H, minus reservations in flight | removes the 8 GiB per boot: +6 running on Node B; ends refusals at 12.67 GiB | S | R (vmctl). Overcommit if many guests spike together: L2 and L11 are the backstop | finding 6; run: a third computer refused with 11.8 GiB free | `vmctl/host_capacity.go:256-291`, `ownership.go:1353, 2009` |
| L2 | Pressure reclaim on a sustained signal | PSI some avg60 (not avg10) plus MemAvailable below floor for 2 to 3 samples; one hibernate in flight; ignore pressure caused by our own snapshot write; never reclaim a computer active in the last N min | ends false reclaims (3 this morning) and the cascade; lets the host run with MemFree near zero | S to M | R (vmctl) | [pressure reclaim problem](../problems/pressure-reclaim-wedged-a-user-computer-2026-10-14.md) | `vmctl/pressure_reclaim.go:245-300`, `nix/node-b.nix:828-837` |
| L3 | Sparse snapshots | `snapshot_type: Diff` with dirty tracking off, so Firecracker writes only resident pages (mincore). This is exact for a cold-booted VM, because non-resident means never touched or reported free, so zero, and guests cannot swap (vmctl unit `MemorySwapMax=0`). For a VM that was itself restored, keep the old memory file, reflink-copy it and `snapshot-editor edit-memory rebase` the diff onto it; or take a full snapshot as today | write 8 GiB down to ~1.2 GiB (−85%); hibernate 7.6 s down to ~1 s; no PSI spike; `st_blocks` becomes honest, so the budget works without `CAP_SYS_ADMIN` (~5 down to 40+ warm snapshots on 48 GiB) | M | R (S3). Diff is a developer preview, and 1.15.0 fixed a diff corruption bug (#5705). Wrong if any guest page can swap. Verify on staging with checksums of guest memory | S3 note D3 (rejected Diff because of swap; guests in fact never swap); FC 1.13 #5274 | `vmmanager/snapshot_lifecycle.go`, `fcapi.go`, `snapshot.go` |
| L3b | Budget measure | log the errno; then either give the measure `CAP_SYS_ADMIN` (vmctl unit or a helper) or rely on L3 | 5 up to ~70 warm snapshots right away | XS | R (unit capabilities) | finding 1 | `vmmanager/snapshot_disk_linux.go:24-50`, `nix/node-b.nix` (vmctl `CapabilityBoundingSet`) |
| L4 | Hibernate after short idle | `VMCTL_IDLE_TIMEOUT` 30m down to 5 to 10m; keep-alive "under-capacity" becomes "online only" | the running set is online users, not everyone from the last 30 min: idle RAM 3 to 6 times lower at the same users | XS (config) after L2, L3, L3b | O/R. A returning user pays 0.3 to 0.8 s. A deploy turns those wakes cold (L20) | wake 338 to 515 ms | `nix/node-b.nix` (VMCTL_IDLE_TIMEOUT, PRIMARY_KEEPALIVE_MODE), `vmctl/warmness_policy.go` |
| L5 | Guest page-cache reclaim | DAMON_RECLAIM (`CONFIG_DAMON_RECLAIM=y` already) with min_age 60 to 120 s and a quota, or a guest timer writing cgroup v2 `memory.reclaim` when idle; lower `page_reporting.page_reporting_order` so smaller freed runs are reported | idle 1.21 down to ~0.5 GiB; sustained busy 8 down to ~3 to 4 GiB | S to M | O (guest image). Re-read cost: store reads hit the shared host page cache, Dolt data does not; some CPU | finding 2 (paper at 7.85 GiB with reporting on); guest kernel has DAMON, LRU_GEN, PAGE_REPORTING | `nix/autoputer-vm.nix` (`boot.kernelModules`, kernel params, a reclaim unit) |
| L6 | Guest process budgets | turn on `CHOIR_DOLT_CACHE_NODE_MIB` (for example 64); desk-worker idle cache by memory budget, not count; review `MaxConcurrent` against measured worker cost | busy −0.5 to 1.5 GiB | S (config), M (budgeted LRU) | O. A smaller Dolt cache slows boot replay | GOMEMLIMIT already set; workers ~142 MiB each | `internal/gomemlimit/gomemlimit.go`, `agentcore/tools_desk.go:88-90`, `actor/dispatcher.go:160-176` |
| L7 | Automatic store compaction (R1) | durable Dolt GC and compaction on a schedule | long-lived computers: disk and working set 3 to 4 times lower | L | R (guest store) | 17.7 to 4.5 GB, RSS 14.7 to 2.7 GB (`d339311e`) | guest store; see the store bloat problem |
| L8 | Re-sparsify data images | there is no discard in virtio-blk, so while a computer is hibernated or stopped: `e2fsck` + `zerofree`, then `fallocate --dig-holes`. Or a vhost-user-block backend that supports discard | the paper: 24.5 GB allocated against its live store; ~30 to 70% for churny stores (estimate) | M | R (persistent disk, offline only) | finding 7 | `vmmanager/manager.go:2664-2780` |
| L9 | Reflinked seed data image | build one seeded `data.img` per deploy; new computers get `cp --reflink` (FICLONE) instead of git clone + mkfs | −3 s per new computer's boot; −0.54 GB per fresh computer until it diverges | S | O | boot timeline: `data_image_ready` 2.9 to 3.2 s | `vmmanager/manager.go:2664-2760` |
| L10 | Shared store via virtio-pmem + DAX | the nix store image as a read-only pmem device (`MAP_SHARED`), erofs mounted with `-o dax`. DAX needs an uncompressed erofs, so the image grows from 2.9 GB lz4 to several GB, stored once | guest page cache for the store goes to 0; one host copy shared by all VMs. Estimate 0.3 to 0.45 GiB per idle guest (most of the 640 MiB cache is store reads: hypothesis) | M | R (VM devices, guest boot). The Firecracker pmem doc warns that a shared backing file can be a side channel (Flush+Reload on shared code pages). Needs a security panel | guest kernel: `VIRTIO_PMEM=m`, `FS_DAX=y`, `EROFS_FS=m`; FC 1.14 #5463 | `vmmanager/manager.go:1578-1600` (drives), `nix/autoputer-vm.nix:269-289` |
| L11 | virtio-mem memory leases | boot with a 1 to 2 GiB base plus a hotplug region up to 8 or 16 GiB (128 MiB slots, 2 MiB blocks, `memhp_default_state=online_movable`); vmmanager plugs memory when the guest asks and unplugs it when idle | memmap −~110 MiB a VM; snapshots carry holes for unplugged memory (documented); guest growth becomes a host grant, which makes overcommit safe and admission exact | L | R (VM lifecycle, guest kernel). A guest can refuse to unplug; needs the API socket (present in snapshot mode) | FC 1.14 #5534, 1.15.1 #5793; guest `VIRTIO_MEM=m`, `MEMORY_HOTREMOVE=y` | new: vmmanager hotplug controller, `fcapi.go` |
| L12 | Smaller guest base | fewer always-on guest services, an on-demand updater (73 MiB RSS) | ~50 to 100 MiB; mostly shared text, so small | M | O | idle anon is only 80 to 132 MiB | `nix/autoputer-vm.nix` |
| L13 | KSM across guests | `prctl(PR_SET_MEMORY_MERGE)` in a launch wrapper (kernel ≥ 6.4; check that it survives exec) | 0.2 to 0.5 GiB per idle guest from duplicate kernel and store pages (estimate); mostly the same pages L10 shares | S | R. Cross-tenant dedup side channels; ksmd costs about a core at 100 GB scanned quickly. **Not recommended** for multi-tenant hosts | finding 5 | new launch wrapper in vmmanager |
| L14 | Huge pages | `huge_pages: "2M"` (hugetlbfs) | a few % CPU for busy guests | S | **Rejected for density.** The pool is preallocated (no overcommit), it defeats 4 KiB reporting and balloon, and restore needs UFFD. Dirty tracking cancels most of the gain (FC snapshot doc) | host THP `never` | — |
| L15 | Swap / zswap for cold guest pages | host zswap or guest zram | idle anon is only ~0.1 GiB, so little | S | **Rejected.** Owner ruling: swapping is service degradation (`nix/node-b.nix:1322`). It breaks L3's mincore exactness. Hibernation already moves whole idle guests to disk | — | — |
| L16 | NUMA placement (EPYC Naples) | vmctl picks the least-loaded of the 4 nodes per VM: cpuset plus a preferred (not strict) memory policy via a `numactl` wrapper; busy VMs pinned to one CCX (4 threads = one 4-vCPU VM); automatic NUMA balancing stays on as the fallback | busy CPU 5 to 15% (cross-die latency on Naples is roughly 1.5 to 2 times local); none for idle | M | O/R (vmctl launch). Strict binding can exhaust one node's 32 GB while others are free | — | `vmmanager/manager.go` (launch), host config |
| L17 | Per-VM cgroups | each Firecracker in its own cgroup (systemd scope or a vmctl subtree): `memory.high`, `memory.pressure` (per-VM PSI), `io.max` for snapshot writes, `cpu.weight` | no direct saving; it makes L1, L2 and the benchmark exact, and stops one guest's I/O stalling the rest | M | R (vmctl, VM lifecycle) | no jailer and no per-VM cgroup today; all VMs share vmctl's cgroup (`MemoryLow=14G`) | `vmmanager/process_control.go`, `nix/node-b.nix:715-745` |
| L18 | UFFD lazy restore with a page server | `mem_backend: Uffd`; one host page server dedups identical pages across snapshots, stores them compressed, and prefetches each computer's recorded working set | wake hot set and latency stable under load; snapshot disk lower by cross-VM dedup; idle VMs can return cold pages | XL | R | FC Uffd backend (1.12 fixes) | new component |
| L19 | Host overhead | fix the artifact store growth; `vfs_cache_pressure` | 2 to 6 GiB of slab and page cache back; 12 GB a day of disk | S to M | O | finding 4; artifact store problem | `nix/node-b.nix`, platform artifact store |
| L20 | Snapshots that survive deploys | versioned guest artifact paths (kernel, initrd, store image) kept until no snapshot needs them; resume on the old release, then update at the next idle moment (LU) | no deploy storm of cold boots; wakes stay at 0.5 s across deploys | M | R (S3 validity rules, deploy) | S3 failure modes 7 and 8; the 11:43 `launch_config_mismatch` that wedged a user | `vmmanager/snapshot.go` (identity checks), deploy scripts |
| L21 | vmctl at 500+ VMs | nftables sets instead of per-VM iptables rules; health checks scaled with N; the registry lock split | removes the non-RAM ceiling around 500 to 800 VMs | L | R | — | `internal/vmctl`, `internal/vmmanager` |

Firecracker v1.15.1 (from nixpkgs; the boot timeline names
`/nix/store/...-firecracker-1.15.1`) supports what the ledger relies on:
- full and Diff snapshots (Diff is a developer preview; mincore mode since
  1.13, #5274; Diff corruption fixed in 1.15.0, #5705);
- File and Uffd memory backends;
- virtio-pmem and virtio-mem (1.14, #5463 and #5534);
- balloon free page reporting (1.14, #5491);
- 2 MiB hugetlbfs.

It has no virtio-blk discard. The guest kernel (6.18.37) already builds
every module the ledger needs: `VIRTIO_MEM=m`, `VIRTIO_PMEM=m`,
`FS_DAX=y`, `DAMON_RECLAIM=y`, `LRU_GEN_ENABLED=y`, `PAGE_REPORTING=y`,
`MEMORY_HOTREMOVE=y`, `ZRAM=m`.

## 3. Numbers by tier, with assumptions and confidence

Tiers:
- **Quick:** L3b, L2, L1, L3, L4, L5, L9, L6.
- **Medium:** + L7, L8, L10, L16, L17, L20.
- **Hard:** + L11, L18, L21, L12.

| limit | host | today | quick | medium | hard | confidence |
|---|---|---|---|---|---|---|
| (a) registered users (disk) | Node B | ~100 more | ~250 | ~400 | (cold tier) | medium: data images measured; compression ratio not |
| | EPYC 128 / 256 | ~500 | ~1,200 | ~2,000 | no host limit with SP3 | medium |
| warm snapshots | Node B | **~5** (finding 1) | 40 to 70 | 70+ | — | high today; medium after |
| (b) idle running | Node B | 11 (2 to 3 beside the paper and the owner) | 44 | 73 | 110 | high today; medium quick (0.5 GiB is the measured post-wake figure); low hard |
| | EPYC 128 | 76 | 218 | 363 | ~550 | same |
| | EPYC 256 | 164 | 458 | 763 | ~800 (CPU) | same; idle CPU unmeasured |
| (c) busy | Node B | 2 to 3 | 7 | 7 to 8 | 8 to 10 | medium: CPU from one sample |
| | EPYC 128 | 12 | ~18 | ~20 | 25 to 30 | low to medium: Zen 1 factor and busy CPU estimated |
| | EPYC 256 | ~18 (CPU; RAM alone would allow 25) | ~18 | ~20 | 25 to 30 | same |
| (d) snapshot wakes a second | Node B | 3 to 5 (est.) | 3 to 5 | 5 to 8 | 10+ | low: n=2 latencies only |
| | EPYC | 10 to 20 (est.) | 10 to 20 | 15 to 25 | 30+ | low |
| hibernates a second | any | ~0.1 (cascade risk) | ~1 | ~1 | 2+ | medium |
| cold boots a second | Node B / EPYC | 0.3 to 0.5 / 1 to 1.5 | +30% (seed reflink) | — | — | low |

### What share of users is online at once

There is no real-user activity data yet: 146 of 179 ownerships are
retained test accounts. The planning figures below are assumptions, to be
replaced by the product's own numbers.

- daily active users: 30% of registered (an engaged early product);
- online at the peak hour: 15% of daily users, so 4.5% of registered;
- busy (agents running) at the peak: a third of online, so **1.5% of
  registered**;
- computers with Autopaper schedules: always busy. Each counts as one busy
  computer, whoever owns it.

Registered users one host serves = the lowest of: disk; busy limit ÷ busy
share; idle limit ÷ online share.

| EPYC 128 | busy share 0.5% (light) | 1.5% (planning) | 5% (heavy) |
|---|---|---|---|
| quick tier (busy 18, idle 218, disk 1,200) | 1,200 (disk) | 1,200 | 360 |
| medium tier (busy 20, idle 363, disk 2,000) | 2,000 (disk) | 1,330 | 400 |
| hard tier (busy 28, cold tier) | ~5,600 | ~1,870 | 560 |

Each always-on paper computer takes one busy slot out of these. Ten
14-source papers take half a host's busy budget, and about $1,100 a day at
list rates.

On Node B, the medium tier serves about 400 registered users (disk), or
about 470 at the 1.5% busy share. Today it serves about 100 more
registered users, with only 2 to 3 busy computers beside the paper and the
owner.

## 4. Order, measurement, and stations

### Order

1. **L3b**: log the errno, then fix the capability. This gives back about
   65 warm snapshots at once. XS.
2. **L2**: a sustained reclaim signal and one hibernate in flight. This
   stops wedging users while every other lever is measured.
3. **L1**: admission by measured use, with in-flight reservations.
4. **L3**: sparse snapshots. They are the precondition for L4, and they
   remove the 8 GiB writes.
5. **L4**: hibernate after 5 to 10 idle minutes.
6. **L5** and **L6**: bound the guest page cache and the process budgets.
   This is where the busy computer goes from 8 GiB to ~3 GiB.
7. **L9**: the reflinked seed.
8. Medium tier, starting with L7 and L8 (disk); then L20 (deploy storms)
   before user counts grow; L17 with the EPYC bring-up; L16 on the EPYC
   only; L10 after a security panel.
9. Hard tier, only if the measured busy share says RAM or idle CPU binds
   before the CPU: L11 first (it subsumes much of L1, L3 and L5), then L18
   and L21.

Not planned: L13 (KSM), L14 (huge pages), L15 (swap).

### Density benchmark (after the quick tier; repeat on the EPYC at bring-up)

Run on minted test computers only, never during an owner window or a
deploy, at most one boot or wake in flight unless the step says otherwise.
Stop all of these computers at the end and record their ids.

Safety stops (any one of them ends the step):
- host memory PSI some avg10 > 1.0;
- MemAvailable below floor + 2 GiB;
- any reclaim of a non-test computer;
- a wake-to-healthy time over 3 s;
- host CPU above 80% for 60 s.

Steps:
1. **Idle ladder.** Mint computers one by one (cold). At 3, 10 and 30
   minutes, record host RSS split into anon and file, and guest meminfo
   (Cached, AnonPages, Slab). Record idle CPU per VM from `/proc/<pid>/stat`
   deltas over 10 minutes. Stop at N = 20 on Node B or 100 on the EPYC, or
   at a safety stop.
2. **Hibernate and wake.** Hibernate all of them one at a time. Record the
   snapshot's logical size, `st_blocks`, compsize, write time and PSI.
   Wake them one at a time, then in waves of 2, 4 and 8 at once. Record
   wake-to-healthy p50 and p95, corpusd append latency, and RSS at 0, 30
   and 300 s.
3. **Busy ladder.** Use one fixed workload: a replayed 3-source schedule,
   plus a fixed set of 4 prompt-bar questions, on the cheapest routed
   model. Run it on 1, 2, 4, then 8 computers. Record CPU per computer, RSS
   over 30 minutes, run latency, model calls in flight and 429s. Stop at
   the first of: CPU 70%, a PSI stop, or a 1.5 times rise in p50 run
   latency.
4. **Deploy storm** (staging, an announced window): after a guest-artifact
   deploy, wake k hibernated test computers and measure the cold-boot rate.
5. Output JSONL plus a summary under `docs/evidence/` with the commit, the
   host and the tier. It is repeatable from a script in `scripts/`. The
   numbers replace the estimates in section 3.

### Stations

| station | levers |
|---|---|
| S3 (snapshot hibernation / fast resume) | L3, L3b, L4, L20; later L11 and L18 |
| LR (resource / cost track) | L1, L2, L5, L6, L7, L8, L9, L19; the benchmark |
| SA (agent density) | busy working set and CPU per busy computer; `MaxConcurrent`; L6's worker budget; L12 |
| SP (production infrastructure) | EPYC bring-up, L16, L17, L21; SP3 cold tier (registered users off-host); SP4 multi-host |
| security panel before | L10 (shared pmem), L13 (KSM), the SMT decision on the EPYC |

## Residuals

- Busy CPU is one sample (the paper, 1.2 cores including boot replay). The
  EPYC busy limit moves between about 15 and 30 with it.
  - Follow-up: [computer CPU efficiency](../archive/reports/computer-cpu-efficiency-2026-10-14.md)
    profiles that CPU. 98% of the paper's guest time is the main daemon,
    and most of that is unindexed store scans.
  - Its target is 0.2 to 0.3 core, about 50 busy computers per EPYC by
    CPU.
  - It measures idle VMs at 0.008 to 0.009 cores.
- The btrfs compression ratio of `data.img` is unmeasured (no compsize on
  the host). Registered-user figures assume about 1 GB per user today.
- The active-share figures are assumptions. Choir has no real-user
  activity curve yet.
- Wake throughput is extrapolated from two wakes.
- Whether the 640 MiB idle page cache is mostly store reads (which decides
  L10's value) needs a guest `fincore`-style read.
