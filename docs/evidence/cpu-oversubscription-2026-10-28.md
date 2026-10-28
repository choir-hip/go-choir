# CPU oversubscription, isolation tiers and desktop hosting (DeepSeek research, 2026-10-28)

Status: raw research by the DeepSeek v4.1 flash seat, landed verbatim at FP0's handoff (token-limited). **Not spot-checked by FP0**: treat every source and number as [H] until verified. Its predictions are not yet in the precommitment ledger. Prompt: `.agentic-consensus/data-arch/cpu-oversub/prompt.md` (gitignored). Context: the owner notes of ~16:00 and ~16:10 UTC in docs/definitions/choir-data-architecture-selection-2026-10-28.md.

---

# CPU oversubscription, isolation tiers, and the desktop as host

Panel answer, convergent mode. Research seat: DeepSeek v4.1 flash. Date: 2026-10-28.
Read-only on the repo; the only file written is this answer.

Scope: the owner note at the end of
`docs/definitions/choir-data-architecture-selection-2026-10-28.md` (CPU
oversubscription, isolation tiers, desktop as the main host). Host facts are
Node B (Xeon E-2236, 6C/12T, SMT on, 31.1 GiB, kernel booted 6.18.16, l1tf /
mds / mmio_stale_data "SMT vulnerable", 3 Firecracker VMs, load 0.08).
Evidence is marked **[V]** verified against a primary source or repo
measurement, **[H]** hypothesis with its test named, **[I]** inference.

---

## Verdict

1. **Oversubscribe with `cpu.weight`, not `cpu.max`.** Weight is
   work-conserving; a hard quota throttles a vCPU that holds the store lock
   and converts a short queue into a multi-millisecond stall. Use `cpu.max`
   only as a tier ceiling, and pair it with a sized `cpu.max.burst` so a
   turn-sized burst is absorbed rather than throttled. Target mean host
   utilization at or below 0.7 of schedulable threads; measure per-wakeup
   run-queue delay, not load average.

2. **The safe ratio is a function of duty cycle, burst correlation and host
   size, not a constant.** At the post-store-fix target of 20% mean duty per
   continuously working computer (0.2-0.3 core each, versus ~1.2 today) and
   independent load, a 12-thread host is safe to about 3.5 vCPUs per thread
   (about 42 one-vCPU computers) at p99 run-queue delay below 10 ms. The
   ratio rises with sqrt(host size): about 4.2 at 64 threads, 4.6 at 256,
   asymptote 1/duty = 5. Correlation, not host size, is the binding term: a
   10% common-mode component caps the ratio near 2.6:1 regardless of host
   size. At today's ~1 core per continuously busy computer the ratio is
   ~0.8:1, so the store fix is a prerequisite for the 3.5:1 figure, not an
   alternative to it.

3. **On Node B today, memory binds before CPU, and the store binds before
   either.** At the owner's 2 GiB steady-state target and the ~22 GiB guest
   budget, Node B realizes 10 to 14 running computers once admission counts
   measured use (today's floor-plus-8-GiB rule leaves room for only 2 to 3
   more) — about
   1 vCPU per thread, far below the CPU-safe 42. The measured busy computer
   used 1.2 cores, and 73% of its guest CPU was the embedded store executing
   scans; idle computers use 0.008 to 0.009 cores. Fix the store first; then
   oversubscribe; then tier by isolation.

4. **Before strangers share a host: disable SMT on Node B, and do not
   substitute core scheduling for it.** Kernel documentation states that
   core scheduling does not stop MDS between a guest sibling and host
   kernel context, and does not stop L1TF from a guest attacker; the
   E-2236's microcode is final. With 1-vCPU computers, per-owner core
   scheduling rarely has a same-cookie sibling to pair, so it recovers
   little of nosmt's density cost anyway. Core scheduling is a
   density-preserving compromise for newer hardware, not for this host.

5. **Hypervisor choice should not be made on vCPU hotplug.** No Choir
   computer needs it at 1-2 vCPU. The differences that matter are memory
   reclaim (virtio-mem), live migration, native affinity/core scheduling,
   and confidential computing. Keep Firecracker for stage 1; Cloud
   Hypervisor is the stage-2 candidate for migration and virtio-mem. None
   of the server hypervisors offers confidential computing on Coffee Lake.

6. **The desktop is the best isolation tier and the worst availability
   tier, so it must be a leased replica, not the authority.** One active
   realization per computer, a monotonic fencing epoch checked by the
   acceptor, self-fencing on lease expiry, and a semantic checkpoint (tape
   tail + CAS + small authority snapshot) that never depends on a memory
   snapshot crossing hypervisors. A planned lid-close handover is a
   planned restart (resume); an unplanned sleep or power loss is a crash
   (close in-flight work). The phone must talk to the cloud control plane,
   never to the desktop's NAT path.

7. **Cheapest high-value lever found: set KVM `halt_poll_ns` to 0 (or
   ~20 us) for this workload.** The measured 200 us halt poll cost 0.094
   cores on one busy computer (7% of it). At 40 computers that is a
   double-digit fraction of the host spent spinning after guest HLTs,
   for wakeups that are network-driven and microseconds-to-milliseconds
   away.

---

## 1. Mechanics of CPU oversubscription under KVM

### 1.1 vCPU threads and the host scheduler

Each Firecracker microVM has a VMM thread, one host thread per vCPU, and an
API thread; the repo measured VMM plus virtio at 0.007 cores for the paper
[V: `docs/reports/computer-cpu-efficiency-2026-10-14.md` §1a]. A vCPU thread
is an ordinary host task that issues `KVM_RUN`; when the guest executes
`HLT` (idle), the thread blocks in KVM. The host scheduler therefore sees
only runnable vCPUs. A computer "waiting on the model API" is a halted
vCPU: the measured paper's 4 vCPUs were halted 73% of the time, and an idle
computer costs 0.008 to 0.009 cores [V: same report].

Scheduling is EEVDF since 6.6: tasks carry a lag (virtual run time minus
actual), eligible tasks (lag >= 0) are run in earliest-virtual-deadline
order, and a wakeup with an earlier deadline can preempt the running task.
A vCPU that slept keeps lag, so a burst that wakes from HLT is served
promptly; a synchronized herd of woken vCPUs queues in deadline order.
Time slices are tunable per task through `sched_setattr` (`sched_runtime`,
100 us to 100 ms) [V: EEVDF doc; LWN "Completing the EEVDF scheduler"].
The practical consequence: oversubscription is cheap for *uncorrelated*
wakeups and expensive for *aligned* ones.

`cpu.weight` distributes only among runnable children and is
work-conserving: an idle computer contributes no weight and consumes no
share. `cpu.max` is a quota per period; when exhausted the cgroup is not
scheduled again until the next period. `cpu.max.burst` accumulates unused
runtime (up to the quota value) and lets a workload borrow against future
underrun, at the cost of interference. `cpuset` gives exclusive cores (the
dedicated tier) [V: cgroup v2 doc; CFS bandwidth doc].

Two host costs are specific to this stack:

- **Halt polling.** KVM spins after guest HLT up to `halt_poll_ns`
  (per-vCPU, grown/shrunk per wakeup) [V: KVM halt-polling doc]. Node B's
  200 us setting cost 0.094 cores on the paper, 7% of its total. The
  poll only pays when the guest wakes within microseconds; a model-API
  wait does not.
- **Interrupt exits.** The host reports `enable_apicv=N`, so every
  interrupt is a VM exit: 10,557 exits/s and 3,740 interrupt injections/s
  on the paper [V: CPU report]. The planned `nested=0` reboot may allow
  APICv to be enabled; if so, exits and halt-poll pressure both fall [H:
  check `enable_apicv` after the reboot].

### 1.2 cgroup v2 controls, steal, PSI

| Control | What it does | Use in Choir |
|---|---|---|
| `cpu.weight` (1..10000, default 100) | Proportional share among runnable tasks; work-conserving | Default tier; paid share is a weight, not a cap |
| `cpu.max` (`$MAX $PERIOD`, us) | Hard bandwidth ceiling; throttles until the next period | Tier ceiling only; below guest vCPU count it is a latency hazard |
| `cpu.max.burst` (us, <= $MAX) | Accumulated runtime a cgroup may borrow | Absorb a turn burst on the dedicated tier |
| `cpuset.cpus` | Exclusive core assignment | Dedicated CPU tier; also L1TF's recommended guest confinement |
| `cpu.stat` | `nr_throttled`, `throttled_usec`, `nr_bursts` | Detect quota-induced p99 inflation |
| `cpu.pressure` / `/proc/pressure/cpu` | PSI `some`/`full`, 10/60/300 s averages and total stall us | Coarse host/guest saturation signal |

Steal time is the guest-visible "runnable but not running" time, delivered
through `MSR_KVM_STEAL_TIME`; the kernel explicitly does **not** count
idle (halted) time as steal [V: KVM msr doc]. That is why a mostly-waiting
fleet shows near-zero steal even at high oversubscription: the vCPUs are
idle, not starved. Steal is the right signal for a *busy* computer and the
wrong one for an idle one.

PSI's `some` is the share of time at least one task is stalled; `full` is
the share where all non-idle tasks are stalled; CPU `full` is undefined at
system level [V: PSI doc]. PSI is a time-share, not a per-wakeup tail: use
it as a trigger and use run-queue histograms for the p99.

### 1.3 Lock-holder preemption and the Go runtime

Two guest-side effects make oversubscription more expensive than the CPU
accounting suggests:

- **Lock-holder preemption.** The store serializes every `ogStore` read and
  write through `engineMu`; it was held 70% of a 122 s window, 89% of that
  in reads, and a second reader can add at most about 2x before the guest's
  own cores contend [V: `docs/design/store-engine-concurrency-2026-10-21.md`].
  If the goroutine holding that mutex is preempted by the host for 10 ms,
  every reader waits 10 ms. KVM's directed yield (the yield hypercall and
  PLE-driven yields on Intel) can help only when the lock holder is a
  runnable vCPU of the same VM; it cannot help when the holder is behind
  other tenants. Go's `sync.Mutex` spins briefly and then parks waiters,
  so the guest does not burn a core spinning, but the latency
  amplification remains. **The host-level guardrail is therefore store wait and turn
  p99, not vCPU utilization.**
- **Go runtime in a small guest.** `GOMAXPROCS` defaults to the guest's
  logical CPUs (container-aware since Go 1.25, so a cgroup quota below the
  vCPU count also lowers it) [V: Go 1.25 release notes]. In a 1-vCPU
  guest, GC mark/sweep and the mutator serialize; the paper's profile
  showed 9% background GC and 18% allocation cost, driven by the store's
  row decoding [V: CPU report]. Two consequences: the store itself needs
  at most about 0.7 core as it stands, and adding vCPUs does not fix a
  store-bound computer; reducing bytes and scans does. A 2-vCPU guest is
  for capsule spikes (Playwright) and GC headroom, not for steady desks.

### 1.4 What to measure

| Layer | Metric | Source / method |
|---|---|---|
| Host per vCPU | Duty (utime+stime), migrations, switches | `/proc/<pid>/task/*/stat` deltas |
| Host per vCPU | p99 run-queue delay per wakeup | `/proc/<pid>/schedstat` field 2 (ns waiting on runqueue) deltas; better, bpftrace histogram of `sched_wakeup` -> `sched_switch` |
| Host per VM | PSI, throttling | cgroup `cpu.pressure`, `cpu.stat` (`nr_throttled`, `throttled_usec`), `cpu.max.burst` counters |
| Host | Halt-poll waste, exits | KVM debugfs `halt_poll_success_ns`/`fail_ns`, exits/s, L1D flushes/s |
| Guest | Steal, PSI, run delay | `/proc/stat` steal, `/proc/pressure/cpu`, `/proc/schedstat` |
| Product | Turn p99, store wait, model wait | Existing counters; keep model wait separate |
| Workload | Duty distribution, **correlation matrix**, burst length | 1 s-bucket CPU per computer over 24 h; pairwise correlation and autocorrelation |

The correlation matrix is the measurement that decides the safe ratio. A
mean and a p99 are not enough: two fleets with the same duty can have
2.6:1 and 5:1 ceilings depending on common-mode load (section 2).

### 1.5 Choir's measured numbers (inputs to the predictions)

- Paper: 4 vCPU, 8 GiB, 1.2 cores average, 0.95 guest-mode cores, 73%
  halted, 31 model calls/min, about 2.3 CPU-s per call; 73% of guest CPU
  is the embedded store, 40% JSON decode, 52% one inbox scan path [V: CPU
  report].
- Idle computer: 0.008-0.009 cores; 1.21 GiB host RSS fresh; 0.19-0.51
  GiB after snapshot wake; 1.3-4.4 GiB for a used computer, 7.85 GiB for
  the paper [V: density
  report].
- After the store levers (C1-C7), target 0.2-0.3 core per busy computer,
  versus 1.2 today [V: CPU report estimate].
- Node B: 31.1 GiB, ~22 GiB guest budget, 12 schedulable threads.
- Planned `nested=0`; current `enable_apicv=N`.

---

## 2. Statistical multiplexing: why the safe ratio grows like sqrt(N), and what caps it

### 2.1 The math

Let one computer's CPU demand have mean `mu`, standard deviation `sigma`,
and a per-computer ceiling of 1 vCPU. If `N` computers are independent, the
aggregate mean grows as `N*mu` while the standard deviation grows as
`sigma*sqrt(N)`; capacity for a tail target `z` is
`C = N*mu + z*sigma*sqrt(N)`. Solving for `N` at fixed `C = M` threads and
writing `R = N/M`:

    R(M) -> 1/mu as M grows, with deficit O(1/sqrt(M)).

This is the classic multiplexing-gain result: the margin is proportional to
the square root of the aggregate, so it shrinks as a fraction of capacity
[V: Addie, Zukerman and Neame, "Broadband Traffic Modeling: Simple
Solutions to Hard Problems", IEEE Communications Magazine, Aug 1998]. With
`mu = 0.20`, `sigma = 0.25`, `z = 2.33` (99th percentile):

| Threads M | Aggregate vCPUs N | Ratio vCPU:thread |
|---|---|---|
| 12 (Node B) | ~41 | 3.4:1 |
| 64 | ~272 | 4.2:1 |
| 256 | ~1181 | 4.6:1 |
| infinity | - | 5.0:1 (1/mu) |

The headline: **a 21x larger host buys about 34% more ratio, not 21x.**
Host size is a weak lever; duty cycle and correlation are the strong ones.

Correlation changes the exponent. With pairwise correlation `rho`, the
aggregate variance is `sigma^2*N*(1+(N-1)*rho)`; for large N the common
term dominates and the tail is proportional to `N`, not `sqrt(N)`. The
ratio ceiling becomes:

    R -> 1/(mu + z*sigma*sqrt(rho)).

With the same `mu`, `sigma`, `z`: rho = 0 gives 5.0; rho = 0.01 gives 3.9;
rho = 0.10 gives 2.6; rho = 0.50 gives 1.6. A 10% common-mode component
costs almost half the ceiling. A binary common mode (a deploy herd where
every affected computer is busy at once) is worse: capacity must cover the
whole herd, ratio -> 1.

Two caveats. The Gaussian approximation is only good once aggregation is
sufficient; the Addie paper says exactly this, and tail-sensitive audio or
interactive work should be measured, not inferred. And the 10 ms target is
a queueing target, not a loss target: with millisecond-class EEVDF slices,
a p99 wait below 10 ms implies roughly 2-3 runnable vCPUs per thread at the
tail, i.e. mean utilization around 0.6-0.75 for bursty arrivals. Operate at
0.7, not 0.9.

### 2.2 Choir's correlation sources

- **Deploy/restart herds.** A deploy that rewrites guest artifacts
  invalidates every snapshot; the density report measured that the next
  36 h turn every wake into a cold boot, at about 1 cold boot/s. vmctl's
  sweeper wakes always-on and critical computers about a minute after it
  starts [V: kernel problem doc]. These are near-perfectly correlated.
- **Aligned schedules.** The paper ran 28 source schedules; if their ticks
  share an offset, their store scans align. A single scan path measured
  ~1.4 s/call in the store design [V: store concurrency doc]; 28 aligned
  ticks are tens of CPU-seconds in a burst.
- **Time of day.** The owner's interactive peak and the paper's publication
  cadence both concentrate load.
- **Provider outages.** The paper already drew 54 retries/hour and 429s on
  two providers [V: CPU report]. A provider recovery makes every waiting
  agent retry at once: CPU, network and gateway all spike together.
- **Publication/adoption fan-out.** One published object read by many
  computers (the Autopaper downstream) is a correlated read burst.

Mitigations, in value order: per-computer jitter on every schedule
(derived from computer id, not random per boot), ring deploys with drain,
retry with full jitter and a token bucket, admission control on wake/boot
(do not boot a herd), and staggering the sweeper. These attack `rho`
directly, which is worth more than buying a bigger host.

### 2.3 Industry ratios

| Provider / product | Published share | Implied per-vCPU ratio | Source |
|---|---|---|---|
| AWS T3/T4g burstable | 5%-40% baseline per vCPU, credits, unlimited mode | 2.5:1 to 20:1 sold; credits bound the average | AWS burstable docs |
| GCP E2 shared-core | e2-micro 2 vCPU at 12.5% each (0.25 vCPU), e2-small 25% (0.5), e2-medium 50% (1.0) | 8:1, 4:1, 2:1 sustained per instance | GCP general-purpose machines |
| Fly.io shared CPU | 5 ms per 80 ms period = 6.25%; burst balance 5 s initial, 500 s max; enforced by CFS quota | 16:1 per vCPU, burst-limited | Fly CPU performance docs |
| AWS Lambda / Firecracker | "thousands of MicroVMs on the same hardware"; "hundreds or thousands per worker"; 150 microVMs/s/host; <5 MiB overhead; <125 ms boot | ratio set by memory and per-function concurrency, not published | Firecracker NSDI'20 paper |

These are per-instance shares, not host ratios: providers oversubscribe the
host further because not every tenant is busy. Fly's 6.25% is the closest
published host-share analogue, and it is deliberately quota-based with a
burst balance, not weight-only. That is a warning for Choir: the cheapest
published density comes with throttling, which is exactly what this
workload cannot absorb on the store-lock holder.

### 2.4 What this means on Node B

At 20% duty the CPU-safe count is ~41 one-vCPU computers, but 41 x 2 GiB =
82 GiB, and Node B has ~22 GiB for guests. Memory binds at 10-14 computers,
about 1 vCPU per thread. Therefore:

- On Node B, CPU oversubscription is not the lever that raises density
  today. Reducing per-computer memory (the data architecture goal) and
  reducing the store's CPU per turn are the levers. Caveat: if most of
  those 10-14 computers are simultaneously busy at today's ~1.2 cores
  apiece, CPU binds first; the 20% figure assumes the store fix and a
  mixed active/idle set, as today's fleet is (load 0.08, CPU pressure 0).
- On a 128 GiB / 16-core host, CPU binds first: the density report puts 20
  busy computers on 16 cores at today's 1.2 cores each, and SMT off halves
  that. There, ratio and isolation policy matter.
- A bigger host helps sub-linearly (sqrt), so do not plan a density story
  that depends on host size; plan one that depends on duty cycle, jitter,
  and per-computer memory.

---

## 3. Hypervisor differences

| Capability | Firecracker | Cloud Hypervisor | QEMU/KVM | Apple Virtualization.framework | Hyper-V |
|---|---|---|---|---|---|
| vCPU count | Fixed at boot (`vcpu_count`, 1-32, `smt` topology flag); no vCPU hotplug API | `boot=` and `max=`; ACPI CPU hotplug controller; SEV-SNP disallows CPU hotplug | `-smp N,maxcpus=M` + `device_add`/`device_del` (ACPI) | `cpuCount` set before boot; no hotplug API | Gen2 hot add/remove documented for memory and network adapters only; CPU hot-add is not in the documented set |
| Pinning / affinity | Host cpuset/cgroup; no native per-vCPU affinity | `affinity=` per vCPU; cpuset | External (libvirt `vcpupin`, taskset); no native | None exposed | Root scheduler places VPs; no per-VP affinity API for guests |
| Core scheduling | None native; host `PR_SCHED_CORE` wrapper (jailer currently sets speculation controls) | Native `core_scheduling` = `vm`, `vcpu` or `off` (per-VM or per-vCPU cookie, not per-owner) | None native; host wrapper | None | Core scheduler (`hypervisorschedulertype Core`); default Server 2019+; client uses root scheduler |
| Per-VM CPU controls | Host cgroup weight/quota | Host cgroup weight/quota | Host cgroup; libvirt shares/quota/period | None; host process priority only | Classic/core scheduler: caps, weights, reserves; **root scheduler (client default): not supported** |
| Memory reclaim | virtio-mem hotplug (separate pool, 128 MiB slots, guest >=5.16); balloon + free page reporting | virtio-mem or ACPI DIMM; `shared=on` changes accounting | Balloon, virtio-mem, DIMM | Virtio balloon device | Dynamic Memory; smart paging; memory weight |
| Snapshot / restore | Full + diff (diff is developer preview); UFFD lazy restore | Full snapshots (sparse, UFFD/copy-on-write restore); incremental is an open request (#8645) | Full VM state (`savevm`) or external disk snapshots; no Firecracker-style diff memory snapshot | `saveMachineStateTo` / `restoreMachineStateFrom` to a state file | Standard/production checkpoints; live migration (Server only) |
| Live migration | No | Yes; same-host, `downtime_ms` bounded (prior research ~50 ms); requires `shared=on` memory | Yes | No | Yes (Server only; not Windows client) |
| Confidential computing | None (no SEV/TDX support in the VMM) [I] | AMD SEV-SNP (IGVM; restrictions incl. no CPU hotplug); no TDX config in the current tree [I] | SEV, SEV-SNP, TDX via `confidential-guest-support` | None exposed | Azure confidential VMs (SEV-SNP/TDX); Windows Server shielded VMs via guarded fabric |
| Attack surface | Minimal device model, seccomp, optional jailer | Moderate; Rust VMM, Landlock opt-in | Largest (full device model) | Apple's framework (macOS 11+) | Large; Windows integration |

Sources: Firecracker `docs/design.md`, `memory-hotplug.md`,
`device-hotplug.md`, `prod-host-setup.md`, the API swagger (`vcpu_count`),
snapshot docs, and `src/firecracker/src/main.rs` (SSBD only on aarch64; no
`PR_SPEC_INDIRECT_BRANCH` opt-in on x86) [V]. Cloud Hypervisor `docs/cpu.md`
(`core_scheduling`, `affinity`, `max`), `memory.md`, `hotplug.md`,
`snapshot_restore.md`, `live_migration.md`, `amd_sev_snp.md` [V]. QEMU
`docs/system/cpu-hotplug.rst`, `confidential-guest-support.rst`,
`i386/tdx.rst`, `i386/amd-memory-encryption.rst` [V]. Apple
`VZVirtualMachineConfiguration.cpuCount`,
`VZVirtualMachine.saveMachineStateTo/restoreMachineStateFrom`,
`VZMemoryBalloonDeviceConfiguration` [V]. Microsoft "Manage Hyper-V
hypervisor scheduler types", "About Hyper-V hypervisor scheduler type
selection", "Hyper-V features and terminology", and the Azure confidential
VM overview [V].

**Decision.** None of this changes stage 1. vCPU hotplug is irrelevant to a
1-2 vCPU computer: you restart onto a new shape, you do not plug. Cloud
Hypervisor's real gains for Choir are (a) virtio-mem, which matters because
memory binds first, (b) live migration, the owner's stage 2, and (c) native
affinity/core-scheduling, useful only if the isolation policy later needs
per-tenant cookies. Its built-in `vm` mode is per-VM, not per-owner, so it
does not by itself implement "one cookie per tenant". Firecracker keeps the
stage-1 density baseline and the smallest device model; do not switch for
CPU reasons.

---

## 4. Cross-tenant security tiers on Coffee Lake

### 4.1 What the silicon and kernel say

- **L1TF (Foreshadow).** A malicious guest can attack physical host memory;
  SMT lets it attack data the sibling brings into L1D. The kernel offers
  L1D flush (observed cost 1%-50%, depending on VMEXIT rate), exclusive
  cpusets, and nosmt; it explicitly says flushing does not fix the SMT
  problem, and that exclusive cpusets hurt load balancing [V: L1TF doc].
  Node B reports `l1tf: ... SMT vulnerable`.
- **MDS.** Buffers are shared across hyperthreads; "on CPUs which are
  affected by MFBDS or MLPDS it is necessary to disable SMT for full
  protection" [V: MDS doc]. Node B reports `mds: ... SMT vulnerable`. MMIO
  stale data is the same class and also reports SMT vulnerable.
- **Other classes are software-mitigated, not tier differentiators.** TAA
  (only where TSX is enabled), SRBDS on affected parts, Retbleed-class
  return-thunk mitigations on pre-eIBRS parts, and base Spectre v1/v2 are
  handled by microcode and kernel patches. They are why kernel currency
  matters; they are not what the sharing choice changes. The tier
  differentiators on this silicon are L1TF, MDS, MMIO stale data and
  VMScape.
- **VMScape (CVE-2025-40300).** Coffee Lake is explicitly vulnerable to
  vBTI guest-user -> host-user and guest-supervisor -> host-user, on the
  same thread and, because STIBP is not always enabled, across SMT
  siblings; the recommended mitigation is IBPB on VMEXIT, and the kernel
  doc adds that "for complete protection ... in SMT environments, STIBP
  should be enabled" [V: VMScape paper; kernel vmscape doc]. Node B's
  `vmscape` state is "IBPB before exit to userspace" (the same-thread case
  is covered); the VMMs are not opted in to the per-process indirect-branch
  control (STIBP/IBPB), though the jailer launch thread sets it when the
  jailer is on, which it is not yet [V: boundary ladder; checklist L17].
- **Core scheduling's holes.** The kernel's own doc says core scheduling
  does not protect against MDS between a user-mode sibling and kernel-mode
  context, nor against an L1TF guest attacker, and does not protect kernel
  contexts (IRQ, syscall, VMEXIT) from sibling threads; cookie 0 (all
  untagged tasks, including host kernel threads) is globally trusted, so a
  tagged guest vCPU can still share a core with a host thread [V: core
  scheduling doc]. It also warns that when total runnable threads are at
  or below half the CPUs, core scheduling can perform worse than SMT off
  because of cross-CPU synchronization.
- **The host boundary is the real escape risk.** The ladder records a
  confirmed guest-to-host escape against a Firecracker-on-KVM product on
  2026-10-03 with no public write-up, and a booted kernel 216 days behind
  [V: kernel problem doc]. No CPU tier fixes that; patch cadence does.

### 4.2 Tier table

| Tier | What it stops on Coffee Lake | What remains | Cost |
|---|---|---|---|
| Time-sliced sharing, SMT sibling across owners | Nothing SMT-specific | L1TF, MDS, MMIO stale data, VMScape cross-SMT; LLC Prime+Probe | Zero |
| Shared + STIBP opt-in + `vmentry_l1d_flush=always` | Cross-SMT branch/return attacks (VMScape cross-SMT); reduces L1TF L1D exposure | MDS host-kernel <-> guest sibling; L1TF guest attacker with crafted PTEs; LLC; IPI windows | STIBP low single digits to ~10% [H]; L1D always flush 5%-15% at 10k exits/s [H] |
| nosmt | All SMT-sibling classes (L1TF, MDS, MMIO, VMScape cross-SMT) | Same-thread Spectre/VMScape (IBPB live), LLC Prime+Probe, host/VMM escapes | 15%-25% computers/host at fixed p99 [H] |
| Core scheduling per owner | Cross-owner SMT sibling classes, while keeping SMT for same-owner pairs | MDS kernel-context, L1TF guest attacker, kernel-context windows; with 1-vCPU tenants, little SMT gain to keep | Kernel doc: can be worse than nosmt when lightly loaded; dev work; per-owner grouping not native in CH |
| Dedicated cores (exclusive cpuset + nosmt) | Time-sharing and sibling sharing; good for latency | LLC still shared (cache-allocation partitioning, where available, is not a security boundary); host/VMM | Capacity loss; load-balancing loss |
| Dedicated host | All cross-tenant co-residency | Host/VMM boundary, firmware/microcode bugs | Whole-host cost |
| Confidential VM (SEV-SNP/TDX) | Host operator/VMM reading guest memory | Cache/branch side channels (VMScape explicitly includes the malicious-host -> SEV-SNP/TDX guest primitive); no Coffee Lake support | Hardware change; memory bandwidth |
| Self-host | Platform cannot read the guest; owner controls the host | Owner's own operational security; platform still sees the semantic stream | User hardware; availability |

**Rule that matters more than the tier names:** while SMT is on, never place
two owners on the same physical core. On Node B, the only way to guarantee
that for 1-vCPU computers without per-owner cookies is nosmt.

### 4.3 Pricing-tier mapping

| Tier | Mechanism | What the customer is told | Where it is available |
|---|---|---|---|
| Shared (default) | `cpu.weight`; same-owner SMT sharing only; different owners never share a core | "Best price; your computer shares the host, never a core with another owner" | Stranger hosts only after nosmt (or newer hardware with equivalent isolation) |
| Dedicated CPU (upcharge) | Exclusive cpuset; `cpu.max` = vCPU count + sized `cpu.max.burst`; no SMT sibling | "Your cores are yours; LLC and host are shared" | Node B / future hosts |
| Dedicated machine (more) | Whole host, single tenant, SMT may stay on | "No other tenant on the machine" | Future hosts |
| Confidential (highest cloud tier) | SEV-SNP or TDX hardware | "The operator cannot read your memory"; explicitly not a cache-side-channel tier | Not on Coffee Lake; future EPYC/Xeon |
| Self-host (credits) | Owner's hardware; lease-held hours with receipts | "Your hardware, your trust domain"; best isolation, owner-controlled availability | Mac/Windows/Linux desktops and single boxes |

Do not sell "dedicated CPU" as side-channel-free: the LLC is still shared,
and cache-allocation partitioning is not a security boundary even where it
exists. Do not sell "confidential" as the side-channel tier: VMScape's
malicious-host primitive targets exactly SEV-SNP/TDX guests. The honest claims are: shared = no
cross-owner core sharing; dedicated CPU = no co-tenancy on your cores;
dedicated machine = no co-tenancy on the box; confidential = memory
confidentiality from the operator; self-host = the operator is you.

### 4.4 The policy I would land before strangers

1. `nosmt=force` on Node B (not just `mitigations=auto,nosmt`, so it cannot
   be re-enabled at runtime), plus `kvm-intel.nested=0`, `/dev/kvm` 0660
   root:kvm, KSM off. This is a correctness boundary, not a feature switch;
   the "on, never off" rule does not apply to a security refusal.
2. Keep the kernel at or above the current fix floor, and keep the IBPB
   VMScape mitigation; VMM speculation opt-in through the jailer launch
   thread when the jailer lands.
3. Do not build core scheduling for 1-2 vCPU tenants. Revisit it when (a)
   the host has many multi-vCPU or multi-VM-per-owner computers, or (b) the
   hardware has eIBRS/current microcode and the MDS kernel-context hole is
   addressed. On a future host, per-owner cookies plus STIBP plus
   `vmentry_l1d_flush=always` is the density-preserving option.
4. Treat the E-2236 as a stopgap for strangers; its microcode is final
   (0xfa, servicing ended 2025-06-30) [V: ladder, citing Intel servicing].

---

## 5. The user desktop as the main host, controlled from the phone

### 5.1 What the desktop is good and bad at

A desktop is a **dedicated machine for one owner**: the best isolation tier
in section 4 (no co-tenancy, no cross-tenant SMT), and the **worst
availability tier** (lid close, sleep, battery, thermal throttling, NAT,
residential power and network). That inverts the usual cloud model: the
desktop is the preferred *realization* for privacy and cost, and the cloud
is the *availability and failover* tier. The phone must therefore treat the
cloud control plane as the authority and the desktop as a data-plane host.

### 5.2 Lease and fencing

The only safe multi-realization model is a lease with a fencing token.

- The lease authority is off the desktop (cloud service, or a quorum of
  relays). It issues a monotonically increasing epoch on every grant.
- Every tape append and every side effect carries the epoch. The acceptor
  (tape append path, relay quorum) rejects any epoch lower than the highest
  it has seen. This is the actual safety property; a local expiry check is
  not enough, because a paused process can resume after its lease expired
  and after another realization has written. Kleppmann's fencing argument
  covers exactly this: GC pauses, page faults, network delays and clock
  jumps all break lease-by-time alone [V: Kleppmann, "How to do distributed
  locking"].
- The desktop **self-fences** on any doubt: if the lease is not renewed, or
  the control plane is unreachable past the TTL, it stops the local writer
  (freeze the appender or the VM), not merely stops admitting new turns.
  Self-fencing is defense in depth; the acceptor check is the guarantee.
- The fencing epoch belongs in the tape schema now (the goal already names
  tape schema v2: content-derived event id, `parents[]`, `branch_id`,
  lease epoch on every append) [V: definition, "Correction 2026-10-28"].
  A `stream_seq` high-water mark is not a fencing epoch.

### 5.3 Sleep, lid close, battery, and the doctrine

The doctrine "a computer stays on while it is working" cannot mean a laptop
lid stays open. It means **the work stays alive**:

- **Planned handover (lid close with notice, low battery, a heavy task).**
  Write a durable planned-handover record (reason, target, epoch); stop
  admitting turns; drain in-flight turns to a deadline; fsync the tape tail
  and write the semantic checkpoint; release the lease; the cloud acquires
  and boots from tape + CAS + small authority snapshot. This is a planned
  restart: work resumes.
- **Unplanned loss (power, forced sleep, network partition).** The cloud
  can take over only from the last head it received. In-flight work is
  closed as "interrupted by a restart"; standing intents (Autopaper
  schedules) continue on the new realization because they are durable
  records, not in-memory timers. No auto-resume. This is the repo's existing
  rule and it applies unchanged to desktops.
- **Drain windows are OS-specific.** macOS `NSWorkspace.willSleepNotification`
  lets an observer delay sleep by up to 30 s [V: Apple doc]; Linux logind
  emits `PrepareForSleep(true/false)` and supports delay inhibitors [V:
  logind doc]; on Windows, `SetThreadExecutionState(ES_SYSTEM_REQUIRED |
  ES_CONTINUOUS)` resets the idle timer [V: Microsoft doc], but a lid close
  or power button is an owner act with no guaranteed drain window. Design
  the handover to complete in a few seconds and treat the OS notification
  as a bonus, not the mechanism.
- **While a turn is in flight** on macOS, hold a `ProcessInfo.beginActivity`
  assertion (idle system sleep disabled) and release it after the drain
  [V: Apple doc]; on Windows, the execution-state call; on Linux, a logind
  inhibitor. Never fight an explicit lid close: hand over.
- **Battery and thermal.** The governor should stop admitting new turns and
  shed heavy capsules to the cloud below a battery/thermal threshold, then
  hand over if the computer can no longer be a viable host. Idle waiting is
  cheap (0.008-0.009 cores measured); a Playwright capsule is not.
- **NAT and the phone.** The desktop dials out to a rendezvous and keeps an
  outbound tunnel (WireGuard/QUIC); direct paths use ICE, and symmetric NAT
  falls back to a TURN relay at the host edge [V: RFC 8445, RFC 8656]. The
  phone talks to the cloud control plane, which routes to the live
  realization; the phone never depends on the desktop's NAT path. For
  Autoradio, the media relay must hold the session across handover to meet
  the 250 ms p99 gap target.

### 5.4 The failover data-loss tradeoff

The owner wants "with no network it automatically offloads to the cloud".
That cannot be done after the network is gone. The cloud can take over only
from the last head it received, so the desktop's unreplicated tail is the
loss window unless desktop acks wait for an off-desktop receipt (relay
quorum). Two honest options:

1. **Failover-safe acks:** every owner-visible ack waits for an off-desktop
   receipt (2-of-3 or 3-of-5 relays). Latency cost is a same-region quorum
   round trip; the cloud can always take over from the last ack.
2. **Bounded loss window:** desktop-only acks, with the window documented
   and bounded by checkpoint cadence. The cloud closes in-flight work as
   interrupted.

Pick one per tier; do not let "automatic fallback" quietly imply option 1
when the implementation is option 2. (The goal's named residual already
says a self-hosted single box cannot produce an off-host receipt.)

### 5.5 Self-hosting credits

The lease gives a verifiable contribution metric: lease-held hours with
signed receipts per epoch. That is better than self-reported uptime and it
pairs with the owner's credits idea. Hosting *other users'* computers on a
user's hardware is a different trust model (the host operator sees the
guest) and should wait for confidential VMs.

---

## 6. Precommitted predictions

Candidate round-0e records for
`docs/definitions/choir-data-architecture-selection-2026-10-28.precommit.md`.
Intervals are 80%. Every claim names a falsifier and the measurement that
settles it; the measurement column names the run that decides.

| ID | Claim | Interval / estimate | Falsifier | Measurement |
|---|---|---|---|---|
| R0e-01 | A 1-vCPU computer with desks idle (waiting on model APIs, no turn in flight) uses this share of one host thread; during a turn it uses this share of its vCPU | idle 1% [0.5, 1.5]; turn 40% [20, 80] | idle >3%, or turn <20% / >80% | Host per-thread utime+stime over 10 min; guest `/proc/stat`; 3 idle + 3 turning computers |
| R0e-02 | On a 12-thread host, 1-vCPU computers at the post-store-fix 20% mean duty (p95 60%; 0.2-0.3 core per continuously working computer) hold p99 per-wakeup host run-queue delay <10 ms up to 3.5 vCPU:thread (42 vCPUs) | 3.5:1 [2.8, 4.5]; ~7:1 at 10% duty; ~0.8:1 at today's ~1 core per busy computer | p99 >10 ms at 2.8:1, or p99 <10 ms at 4.5:1 | CPU-only sweep with minimal guests (memory not binding); schedstat deltas + bpftrace wakeup->switch histogram; guest steal as cross-check |
| R0e-03 | On Node B, memory binds before CPU: with admission counting measured use and 2 GiB per computer, running computers stop at 10-14, i.e. ~1 vCPU:thread, before p99 run-queue reaches 10 ms (today's floor-plus-8-GiB admission leaves room for only 2-3 more) | 10-14 [9, 16] | 20 computers run at p99 <10 ms with PSI full <5%, or memory pressure below 8 | Capacity run under the admission path; `memory.pressure`, OOM events, per-VM RSS |
| R0e-04 | nosmt costs 15%-25% of computers per host at fixed p99; per-owner core scheduling recovers at most 5% of that on a host of 1-vCPU computers | nosmt 20% [8, 32]; core-sched recovery 3% [0, 10] | nosmt cost <5% or >35%; core-sched recovery >15% | A/B on the same synthetic load: SMT on, nosmt, core-sched per owner, STIBP-only; measure computers/host at p99 |
| R0e-05 | KVM halt polling at 200 us costs a double-digit share of a 40-computer host at 20% duty; setting 0 recovers most of it with no p99 regression | cost 18% [10, 30]; recovery 15% [8, 25] | cost <5% or recovery <5%; p99 worse by >15% | A/B `halt_poll_ns=200000` vs `20000` vs `0` at 20-40 computers; KVM debugfs counters; turn p99 |
| R0e-06 | `cpu.max` at 1 CPU on a 1-vCPU computer is near-no-op; on a 2-vCPU computer the same quota at 50% aggregate utilization at least doubles p99 turn latency vs weight-only | 1-vCPU throttle <1% of periods; 2-vCPU latency 2.0x [1.4, 3.0] | >5% of periods throttled on 1 vCPU; <1.2x or >4x on 2 vCPU | `cpu.stat` `nr_throttled`/`throttled_usec`; turn and store-wait p99 A/B |
| R0e-07 | Forcing STIBP on Firecracker threads (`PR_SET_SPECULATION_CTRL, PR_SPEC_INDIRECT_BRANCH, PR_SPEC_FORCE_DISABLE`) costs less than 10% of computers per host at fixed p99 | 4% [1, 10] | >15% | A/B with and without the prctl in the launch wrapper; same synthetic load |
| R0e-08 | Desktop handover with a fencing epoch: 100 forced lid-close/lease-expiry trials produce zero accepted stale-epoch appends, at most one lost turn per trial, and p99 handover (last pre-sleep append to first cloud append) at most 5 s | 0 stale appends; <=1 lost turn; p99 5 s [2, 10] | any accepted stale epoch; >2 lost turns; p99 >15 s | Fault-injection drill on a Mac + cloud pair; count epochs at the acceptor; phase-instrument checkpoint, boot, first append |

Notes on measurement difficulty:

- **Duty is bimodal.** A 10-minute average hides turns. Report idle, turn,
  and per-minute p99 separately; the ratio prediction uses the *mean*, the
  latency prediction uses the *turn*.
- **Run-queue delay needs per-wakeup samples.** `/proc/schedstat` is
  cumulative; histogram deltas per vCPU thread, or use bpftrace. Guest
  steal is a cross-check, not a substitute (idle time is not steal).
- **Synthetic load must be independent first, then correlated.** Measure
  the ratio at rho = 0, then replay a deploy/tick trace to measure rho > 0.
  A single mixed run cannot separate the two.
- **Memory confounds CPU sweeps.** Use minimal guests for R0e-02; use the
  real guest for R0e-03.
- **Provider limits dominate live turn latency.** Stub model calls at
  recorded latency for the CPU experiments, and report provider-bound
  throughput separately (as R0d already says).

---

## 7. Dissent and risks

**Dissent I expect, and my answer.**

- *"Core scheduling is better than nosmt because it keeps SMT."* On newer
  hardware with eIBRS and current microcode, yes. On Coffee Lake it is not
  equivalent: the kernel doc's MDS kernel-context and L1TF guest-attacker
  holes remain, cookie 0 trusts host kernel threads, and with 1-vCPU
  tenants the same-cookie sibling is usually absent, so the density
  recovered is small. Measure it (R0e-04) rather than assume it.
- *"CPU is the density limit, so oversubscription is the priority."* On a
  128 GiB / 16-core host it is. On Node B, memory binds at 10-14 computers
  and the store consumes 73% of the busy computer's guest CPU;
  oversubscription shares a bottleneck that is better removed.
- *"Use hard quotas for fairness."* Hard quotas are fair but throttle the
  store-lock holder; weight is work-conserving and degrades gracefully.
  Use quota + burst only for the paid dedicated tier.
- *"The desktop should be the primary host, so the cloud is optional."*
  The desktop is the best trust tier, but failover requires an off-desktop
  receipt or a documented loss window, and the phone needs a control plane
  that is up when the desktop sleeps. The cloud is not optional for
  availability; it is optional for *confidentiality*.
- *"Firecracker should move to Cloud Hypervisor for CPU features."* vCPU
  hotplug is irrelevant at 1-2 vCPU. Move for virtio-mem and live
  migration, on their own evidence.

**Risks and edge cases.**

- The E-2236 is end-of-microcode; no software tier can restore silicon
  fixes. Strangers on it are a stopgap.
- `nosmt=force` cuts host build threads; deploys and compaction get slower.
  Schedule them off-peak or accept the slower window (resource use ahead of
  short-term uptime, but a planned restart is still a restart).
- A 2-vCPU computer doubles its burst footprint; do not give 2 vCPU by
  default "for headroom". Capsules are the exception.
- `cpu.max.burst` borrows against future underrun; a tenant that bursts
  constantly will starve its own future and interfere with neighbors.
  Bound the burst and monitor `nr_bursts`.
- The correlation matrix is measured on today's fleet (one owner, one
  paper). Stranger traffic may correlate differently (time zones, provider
  outages); re-measure before selling a ratio.
- Desktop handover assumes the guest can be checkpointed semantically. If
  a turn's in-flight model call cannot be redelivered idempotently, the
  "at most one lost turn" prediction becomes "one lost call", and the
  agent protocol needs request ids.
- Tape schema v2 (fencing epoch) is a predecessor, not a detail; without
  it, desktop + cloud is split brain by construction.

---

## 8. Confidence

- **Mechanics (EEVDF, weight/quota/burst, steal, PSI, halt polling): high.**
  Kernel docs, KVM docs, and the repo's own measurements agree.
- **Security tiers: high on mechanism, medium on practical exploitability.**
  The kernel documents the holes; no Choir-specific PoC exists for MDS or
  L1TF, and VMScape's practical exploit targeted QEMU/AMD, not Firecracker.
  The policy (no cross-owner SMT sibling) does not depend on exploitability
  estimates.
- **Ratio numbers: medium.** They depend on the 20% duty assumption and the
  Gaussian approximation; the falsifiers are cheap and should be run before
  any capacity promise.
- **Hypervisor matrix: high.** Read from each project's docs/source.
- **Desktop design: medium-high on mechanics (lease, fencing, sleep
  windows, NAT), medium on UX latency and the receipt/loss-window
  tradeoff.**
- **Overall verdict: high** on weight-not-quota, nosmt before strangers on
  this host, halt-poll off, memory-first sequencing, and lease+fencing for
  desktops. **Medium** on the exact 3.5:1 and 15-25% figures.

---

## Sources

Primary (verified 2026-10-28):

- Linux EEVDF scheduler: https://docs.kernel.org/scheduler/sched-eevdf.html
- LWN, "Completing the EEVDF scheduler": https://lwn.net/Articles/969062/
- cgroup v2: https://docs.kernel.org/admin-guide/cgroup-v2.html
- CFS bandwidth control (quota, burst): https://docs.kernel.org/scheduler/sched-bwc.html
- PSI: https://docs.kernel.org/accounting/psi.html
- KVM steal time (MSR_KVM_STEAL_TIME): https://docs.kernel.org/virt/kvm/x86/msr.html
- KVM halt polling: https://docs.kernel.org/virt/kvm/halt-polling.html
- schedstat (per-task run-queue wait): https://docs.kernel.org/scheduler/sched-stats.html
- Core scheduling: https://docs.kernel.org/admin-guide/hw-vuln/core-scheduling.html
- L1TF: https://docs.kernel.org/admin-guide/hw-vuln/l1tf.html
- MDS: https://docs.kernel.org/admin-guide/hw-vuln/mds.html
- VMScape kernel doc: https://docs.kernel.org/admin-guide/hw-vuln/vmscape.html
- VMScape paper: https://comsec-files.ethz.ch/papers/vmscape_sp26.pdf
- Speculation control prctl: https://docs.kernel.org/userspace-api/spec_ctrl.html
- Addie, Zukerman, Neame, "Broadband Traffic Modeling: Simple Solutions to
  Hard Problems", IEEE Communications Magazine, Aug 1998 (author copy):
  http://syafii.scriptmania.com/syafii.scriptmania.com/pdf/Broadband_Traffic_Modelling.pdf
- Firecracker prod host setup (disable SMT): https://github.com/firecracker-microvm/firecracker/blob/main/docs/prod-host-setup.md
- Firecracker memory hotplug (virtio-mem): https://github.com/firecracker-microvm/firecracker/blob/main/docs/memory-hotplug.md
- Firecracker device hotplug (PCI only): https://github.com/firecracker-microvm/firecracker/blob/main/docs/device-hotplug.md
- Firecracker snapshots (diff developer preview, UFFD): https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/snapshot-support.md
- Firecracker API (`vcpu_count`): https://github.com/firecracker-microvm/firecracker/blob/main/src/firecracker/swagger/firecracker.yaml
- Firecracker source (SSBD aarch64 only; no PR_SPEC_INDIRECT_BRANCH on x86): https://github.com/firecracker-microvm/firecracker/blob/main/src/firecracker/src/main.rs
- Firecracker NSDI'20 paper: https://www.usenix.org/system/files/nsdi20-paper-agache.pdf
- Cloud Hypervisor CPU (affinity, core_scheduling, max vCPU): https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/cpu.md
- Cloud Hypervisor hotplug: https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/hotplug.md
- Cloud Hypervisor memory (virtio-mem, shared): https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/memory.md
- Cloud Hypervisor live migration (shared=on): https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/live_migration.md
- Cloud Hypervisor AMD SEV-SNP: https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/amd_sev_snp.md
- QEMU CPU hotplug: https://www.qemu.org/docs/master/system/cpu-hotplug.html
- QEMU confidential guest support: https://www.qemu.org/docs/master/system/confidential-guest-support.html
- QEMU TDX: https://www.qemu.org/docs/master/system/i386/tdx.html
- QEMU AMD memory encryption (SEV-SNP): https://www.qemu.org/docs/master/system/i386/amd-memory-encryption.html
- Apple VZ cpuCount: https://developer.apple.com/documentation/virtualization/vzvirtualmachineconfiguration/cpucount
- Apple VZ save/restore: https://developer.apple.com/documentation/virtualization/vzvirtualmachine
- Apple willSleepNotification (up to 30 s delay): https://developer.apple.com/documentation/appkit/nsworkspace/willsleepnotification
- Apple beginActivity: <https://developer.apple.com/documentation/foundation/processinfo/beginactivity(options:reason:)>
- Hyper-V scheduler type selection: https://learn.microsoft.com/en-us/windows-server/virtualization/hyper-v/manage/about-hyper-v-scheduler-type-selection
- Hyper-V scheduler types management (client root scheduler): https://learn.microsoft.com/en-us/windows-server/virtualization/hyper-v/manage/manage-hyper-v-scheduler-types
- Hyper-V features (Gen2 hot add: memory and network): https://learn.microsoft.com/en-us/windows-server/virtualization/hyper-v/features-terminology
- Azure confidential VMs (SEV-SNP/TDX): https://learn.microsoft.com/en-us/azure/confidential-computing/confidential-vm-overview
- AWS burstable credits and baselines: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/burstable-credits-baseline-concepts.html
- GCP shared-core machine types: https://cloud.google.com/compute/docs/general-purpose-machines#sharedcore
- Fly CPU performance (6.25% baseline, burst, CFS quota): https://docs.fly.io/machines/cpu-performance
- Datadog, Kubernetes CPU requests/limits (quota throttles with spare CPU): https://www.datadoghq.com/blog/kubernetes-cpu-requests-limits/
- AWS Dedicated Hosts vs Dedicated Instances: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/dedicated-hosts-overview.html
- Go 1.25 container-aware GOMAXPROCS: https://go.dev/doc/go1.25
- Kleppmann, fencing tokens: https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html
- RFC 8445 (ICE): https://www.rfc-editor.org/rfc/rfc8445
- RFC 8656 (TURN): https://www.rfc-editor.org/rfc/rfc8656
- logind PrepareForSleep: https://www.freedesktop.org/software/systemd/man/latest/org.freedesktop.login1.html
- Windows SetThreadExecutionState: https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-setthreadexecutionstate

Repo evidence:

- `docs/reports/computer-cpu-efficiency-2026-10-14.md` (measured CPU split,
  halt polling 0.094 cores, 73% halted, APICv off, store 73%).
- `docs/reports/vm-density-limits-2026-10-14.md` (idle 0.008-0.009 cores,
  1.2 cores busy, 1.3-4.4 GiB used, deploy storm, 20 busy on the EPYC, SMT
  off halves).
- `docs/design/store-engine-concurrency-2026-10-21.md` (engine mutex 70%,
  89% reads, 1.4 s scans, MaxConcurrent analysis).
- `docs/problems/host-kernel-216-days-unpatched-2026-10-15.md` (SMT
  decision, nosmt estimate, marker TTL, sweeper behavior).
- `docs/reports/boundary-strength-ladder-2026-10-15.md` (CPU side door,
  VMScape state, microcode EOL, stranger policy options).
- `docs/security-density-architecture-checklist-2026-10-21.md` (NET-07,
  DEC-06, L17 jailer off).
- `docs/definitions/choir-data-architecture-selection-2026-10-28.md` (owner
  notes, host facts, tape schema v2, FP0 desktop notes).
- `docs/definitions/choir-data-architecture-selection-2026-10-28.precommit.md`
  (R0d density records; these are candidate R0e records).
