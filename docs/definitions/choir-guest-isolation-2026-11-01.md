# Choir guest isolation: one guest cannot hurt another or the host (2026-11-01)

The owner made this the next mission on 2026-11-01: "Ok, good plan. We
should make this the next mission actually. Make a throughline. I'll drop
the current goal." The requirement, the same evening: "The guests will
grow. We are building a platform for users. Each user gets a vm. We will
need more hardware very soon actually. But obv I don't want to protect file
cache. The point is that the host shouldn't thrash, neither should the
guests. But a thrashing guest shouldn't obstruct other guests, ofc." And:
"reduce real memory usage while maximizing flexibility for bursts/spikes";
"the key deliverable to close out the memory and storage work ... is
numbers. Current stable state and spike data, and storage growth. And
ongoing observability." Earlier authority: the jailer on every computer
(owner, 2026-10-24: "Let's apply the jailer everywhere").

It succeeds the memory mission
([choir-memory-optimization-2026-11-01.md](choir-memory-optimization-2026-11-01.md)),
whose open items (the paper's reading under the elastic ceiling, the
closing numbers report) are station I0 here.

```yaml
definition_version: throughline/1
readiness: executable
readiness_authority: owner, 2026-11-01 (quoted above)
review:
  reviewer: Devin (swe-2-max), one seat; the I2 diff itself had Devin and DeepSeek (tier 2)
  frozen_ref: 0ac3dd57050118a1
  verdict: accept (nine non-blocking findings, adjudicated in the ledger)
  evidence_ref: ledger 2026-11-01 review entry
start:
  observed_at: 2026-11-01T19:45Z
  deployed: 03c8dbf12 (elastic guest memory ceiling, telemetry cited ids once per run, snapshot and compaction cache drops, per-minute memory meter, security epoch 29), live at 19:32Z; the paper rebooted 19:31:40Z
  local_unpushed: >-
    On lane/backup-unshard (worktree agent-a68a1d96eb5bf63ea, goal-owned):
    785868996 memory.low 0 on system.slice, the VM slice and the vmctl unit;
    461c1e057 docs/problems/thrashing-guest-can-obstruct-other-guests-2026-11-01.md;
    the compaction drop after a copy that ran (Devin #3). Held so the push
    does not reboot the paper inside its first telemetry run.
  node_b: >-
    12 cores, 31 GiB, 25 GiB available, memory pressure 0. Root is md127
    (RAID1 of nvme0n1 and nvme1n1) on btrfs, 204 GB free; guest data images
    are sparse, 288 GB apparent in total (the paper's 128 GB, 24 GB used).
    Block scheduler none on both NVMe devices; iocost compiled in
    (CONFIG_BLK_CGROUP_IOCOST=y) and off (io.cost.qos empty). The jailed
    VMs' parent (system-choirvms.slice/vms) delegates only cpu and pids. Host
    io pressure some avg60 about 1.6 (inventory read). No tc qdisc and no
    Firecracker net or drive rate_limiter; cpu.max is max everywhere.
  vms: >-
    Running: the paper, direct (unjailed, Firecracker as root, in the vmctl
    unit's cgroup, VMCTL_JAILER_ONLY excludes critical_protected), and the
    owner's computer, jailed. Hibernated: d@e, f1daccdb, b@c, and a real
    user's computer-7cc96dc6 (stopped since 2026-10-22). Epoch 29 makes each
    hibernated computer boot fresh at its next wake.
  inventory: >-
    Sonnet read-only inventory 2026-11-01 (table in the ledger): disk space
    high (no per-guest cap, the host disk alarm only logs), disk I/O high,
    direct VMs high, network bandwidth medium-high, CPU ceiling medium,
    per-VM memory backstop medium; reachability, seccomp, kernel floor and
    image permissions in good shape.
finish:
  deliver: >-
    On Node B, a guest that thrashes, fills its disk, saturates its disk
    bandwidth, its network or its CPU slows only itself; other guests and
    the host keep their headroom. Each guest still bursts to its own size
    when the host is idle. The numbers that show it are recorded every minute
    and reported every day.
  artifacts:
    - "I0: the memory mission closed with numbers: the paper's p50, p95 and max RSS under the elastic ceiling through a telemetry run, host stable state, disk reads per day, storage growth per day, in a published report"
    - "I1: per-VM memory, io and cpu pressure (cgroup *.pressure) in the memory meter and its report, with flags"
    - "I2: the paper jailed (SEC-04, the last wave), at a watched planned refresh, confinement verified; public_platform computers jailed by class at their next launch, so SEC-04's residual closes"
    - "I3: disk I/O isolation that bites under scheduler none: iocost on the NVMe members with io delegated to the VM slice and a per-VM io.weight, and the VM slice weighted below the host services so the host keeps its share; io.max or a Firecracker drive rate_limiter only if the drill shows weights are not enough"
    - "I4: disk space: a host reserve floor that acts (refuses starts and image growth below it) and a per-guest data image size policy"
    - "I5: network: a per-VM bandwidth limit (tap qdisc or Firecracker net rate_limiter) a per-source conntrack limit, and per-tap anti-spoof (a guest sends only from its own MAC and address and cannot reach a sibling over the bridge)"
    - "I6: per-VM cpu.max (the vCPU count plus headroom for the VMM's own threads) and a per-VM memory backstop (memory.high above guest size plus VMM overhead)"
  acceptance:
    - action: "drill on d@e only: d@e thrashes its disk (a whole-store read loop past its RAM) while an observer guest (a test computer, never the paper or the owner's) runs a fixed read workload"
      proves: the observer's io pressure and read latency stay within a stated bound of its solo baseline, and the host's pressure stays under the meter's flag
      evidence_class: deployed drill, memory meter ledger and report
    - action: "the same drill shape for disk space (d@e grows its image toward the floor), network (d@e floods egress) and CPU (d@e spins every vCPU)"
      proves: each limit acts on d@e and leaves the observer and the host within bound
      evidence_class: deployed drill; each drill has an abort bound (the disk drill stops 20 GB above the floor, the network drill runs 60 s at most)
    - action: "the paper jailed after the planned refresh"
      proves: scripts/vmm_confinement_verify.py passes for the paper, its health answers, its ceiling acts and its meter row reads its own cgroup
      evidence_class: deployed observation
    - action: "d@e alone on an idle host runs the same loads"
      proves: no limit binds when nothing else needs the resource (bursts are free)
      evidence_class: deployed drill
    - action: "the memory meter report after a day"
      proves: stable state, spikes, disk reads, per-VM pressure and storage growth are published numbers
      evidence_class: deployed observation
  rollback: >-
    Each station is one config line or one revert: VMCTL_JAILER_ONLY back to
    class:test,class:primary,class:premium_always_on (the paper cold boots
    direct); iocost off (io.cost.qos enable=0) and the io delegation dropped;
    each limit's own variable. The security epoch never falls.
  landing:
    required: true
    environment: staging (choir.news, Node B)
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]
value:
  principle: >-
    Memory is the bottleneck; NVMe reads are cheap (owner, 2026-11-01: "we
    can wait for an NVMe read for something that we're not reading that
    often"; "we do need to do the next optimization to make sure that we're
    not doing any extra reads"). Guests keep little cache; the code must not
    reread what it does not need.
  better_means: >-
    Minimize how much one guest's load (disk I/O, disk space, network, CPU,
    memory) raises other guests' pressure stalls and lowers the host's
    headroom, while a guest alone on an idle host still gets the whole
    resource.
  goodharting_would_be: >-
    Static low caps that make every guest slow even on an idle host;
    enabling a controller with no observed effect (io.weight under scheduler
    none); proving isolation on idle test computers or with averages that
    hide the spike; drills on the paper or the owner's computer.
homotopy:
  realism_axis: >-
    One noisy guest (d@e) and one observer guest on today's Node B, then the
    paper and the owner's computer as passive observers, then the fleet under
    real load as users and hardware are added.
boundaries:
  mutation_class: red (vmctl, VM lifecycle, the jailer, host cgroups and block I/O, host disk, guest network)
  authority_sources:
    - owner 2026-11-01 (quoted above)
    - owner 2026-10-24 jailer everywhere
    - CLAUDE.md (restart rules, mutation classes, landing loop)
  must_preserve:
    - crash restarts close work, planned restarts resume; the paper is refreshed only at a planned, marker-backed refresh, never mid self-development apply or with a live capsule
    - adversarial drills on d@e only, never on the paper or the owner's computers
    - guests never swap; no file cache is protected (memory.low 0)
    - the elastic ceiling never takes anonymous memory and stands aside for capsules and self-development
    - the internal-service token only to 127.0.0.1 ports 8083, 8084, 8086, 8087; guests get X-Internal-Caller only
    - no secrets or owner email addresses in the repo; production reads are metadata only
    - no deletion of a user's computer without the owner naming it
  excluded:
    - Node A (owner, 2026-11-01: "Don't bother with node a rn")
    - buying or adding hardware (this mission makes the next host safe to fill)
    - encryption at rest (NET-06) and SMT or core scheduling (NET-07), planned separately
    - the internet egress policy (NET-01, NET-04)
  protected_surfaces: [vmctl launch and jailer args, host cgroup tree, block device io.cost, guest network taps, the paper's lifecycle]
now:
  status: checkpoint_incomplete
  slice: >-
    I1, I2, I3 (synthetic), I4, I5 and I6 deployed; the reread fixes
    landed. f7a560fb3 live 23:05Z: per-VM memory.high (size + 1 GiB),
    memory.swap.max 0, cpu.max (vCPUs + 1), io.weight 100 on every jailed
    leaf; iocost latency qos; disk admission floor; connlimit 8192 per
    guest; cake on the uplink; the retirement pass reads 0.47 GB in 24 s
    (was about 8.5 GB). Next: the d@e end-to-end drills (disk, network,
    CPU, lone-guest burst), a day of meter data, the closing report.
  source_ref: f7a560fb3
  deploy_identity: f7a560fb381af8637a8c245fd924182757550d2a
  candidate:
    id: none-held
    state: landed
    ref: main
    base: f7a560fb3
    scope: []
  conjecture:
    id: C1
    claim: >-
      iocost on nvme0n1 and nvme1n1 with io delegated to the VM slice and a
      per-VM io.weight bounds a disk-thrashing guest's effect on another
      guest, through md127 RAID1 and btrfs, while leaving a lone guest its
      full bandwidth.
    test: the I3 drill, read through per-VM io.pressure (I1) and observer read latency
    edge: missing_oracle
    delta_o: I1, per-VM pressure in the meter, before any I/O control lands
    scope_if_supported: Node B's disk layout (md RAID1 over NVMe, btrfs); another host's layout is re-tested
    status: supported (synthetic rung, and end to end on d@e for reads and CPU; a write storm halves a co-reader, residual)
    evidence_refs: [ledger 2026-11-01 21:00Z I3 synthetic drill, ledger d@e end-to-end drill results-20261101T234609Z]
  decision:
    what: >-
      Order: observe (I0, I1) before controlling; jail the paper before
      per-VM limits, since only a jailed VM has a cgroup of its own.
    kind: operational
    status: settled
    evidence_ref: this file
    owner_ratification_ref: owner 2026-11-01 "Ok, good plan"
  belief:
    believed_state: >-
      Every running guest is jailed with memory, swap, CPU and I/O bounds on
      its own leaf; disk admission and network sharing are live; a
      synthetic noisy cgroup no longer starves an observer (513 MB/s, p99
      8.6 ms, against 143 MB/s under auto qos). The paper's steady host RSS
      is about 1.4 GB; its reads are megabytes a minute between passes.
    main_uncertainty: >-
      Whether the bounds hold end to end with a real guest as the noisy
      neighbour (d@e), and what the 1 GiB leaf headroom does to a
      cache-heavy guest write (DeepSeek I6 #2).
    next_observation: the meter's day of per-VM memory, reads and pressure under ordinary load with every bound live
  blocker_or_risk: >-
    Time gate: I0 needs the meter's first full day (until 2026-11-02T19:32Z);
    guest deploys are held until the closing report so the day is not cut by
    reboots. Residuals: every vmctl deploy reboots all guests (deploy-impact
    classification); TestManagementDelegatesIntakeToTasks flakes in CI
    (asynchronous session release); no test pins the restore shape (I6 #4).
  next_action: collect a day of meter data (from 2026-11-01), then write and publish the closing numbers report; in the meantime the write-storm residual and the remaining whole-kind reads
receipts:
  - station: I1
    status: landed
    ref: per-VM memory, io and cpu pressure in the meter (mem-2026-11.jsonl, from 2026-11-01T19:32Z)
  - station: I2
    status: landed
    ref: 8ab7b2da5; the paper jailed (vmm_confinement_verify --expect jailer ok)
  - station: I3
    status: landed
    ref: a30787be5, 54ee9cfc3; synthetic drill and d@e end-to-end drill (ledger; results-20261101T234609Z)
  - station: I4
    status: landed (not drilled)
    ref: e8307302f in 43c070756
  - station: I5
    status: landed (observed, not drilled)
    ref: d32e6c2c3, c4f19e126 in 43c070756
  - station: I6
    status: landed
    ref: 6a9941f3e, 5d6fd3100 in 43c070756; Devin and DeepSeek accept
  - station: reread
    status: landed
    ref: 8092c9dbf, 37790f91d, e1ef09abd, 93237523b, f7a560fb3; retirement pass 0.47 GB in 24 s (was about 8.5 GB)
  - station: I0
    status: checkpoint
    ref: docs/reports/guest-isolation-checkpoint-2026-11-01.md; the closing report after the meter's first day (2026-11-02T19:32Z)
```

## Stations

- **I0, close the memory mission with numbers.** The paper under the
  elastic ceiling through a telemetry run; push the held batch; a day of
  the memory meter; the closing report (stable state, spikes, disk reads,
  storage growth), published.
- **I1, per-VM pressure.** The memory meter reads each VM cgroup's
  memory.pressure, io.pressure and cpu.pressure (jailed VMs have one each;
  the direct paper until I2 reads as the vmctl unit) and the report flags a
  guest whose stall raises another's. This is the observer every later
  station is proved with.
- **I2, jail the paper.** VMCTL_JAILER_ONLY gains the paper's class (or `*`)
  at a watched planned refresh; scripts/vmm_confinement_verify.py; the
  paper's health, the ceiling, the balloon and the meter under the jail.
- **I3, disk I/O.** iocost on the NVMe members (model auto, qos from a
  measured baseline), io added to the VM slice's subtree_control, io.weight
  per VM next to cpu.weight (internal/vmmanager/confinement.go). Drill
  first; io.max or a drive rate_limiter only if C1 fails.
- **I4, disk space.** The host disk alarm becomes an action: below a
  reserve floor vmctl refuses new starts and image growth; a per-guest data
  image size policy (apparent size against the host) now that guests grow.
- **I5, network.** Per-VM bandwidth (tap qdisc or Firecracker
  rate_limiter) and a per-source conntrack limit.
- **I6, CPU and memory backstops.** cpu.max about the vCPU count;
  memory.high on the jailed leaf above guest size plus VMM overhead.

Each station lands through the landing loop and is proved by its drill on
d@e, read through I1's numbers. Code review is risk-tiered (CLAUDE.md):
tier 1 (Devin) for the additive, switched limits; tier 2 (Devin and
DeepSeek) for the paper's jail and anything on the boot or lifecycle path.

## Successor

Owner, 2026-11-01: "Once we finish this mission, get all our wins and much
needed isolation, then we should go to self-dev, and see how much we can do
concurrently. That's the load test." The next mission is a self-development
concurrency load test: as many concurrent self-development capsules as Node B
carries, read through this mission's per-VM pressure numbers. It is authored
at this mission's terminal receipt.
