# Node A ran out of memory and the kernel killed the agents build VM

2026-11-14T06:42Z. Recorded 06:44Z.

## Problem

Node A's kernel killed the hypervisor process of the agents build VM in a
global out-of-memory kill. The VM restarted by itself within thirty seconds.
A landing train (late take plus the run-memory retirement census) was two
minutes into its suites on that VM and died with it. Nothing was pushed, so
main is unaffected; the train has to run again.

## Evidence

- Node A kernel log, 06:42:32Z: `oom-kill: constraint=CONSTRAINT_NONE ...
  global_oom, task_memcg=.../microvm@agents.service`, killed
  `cloud-hyperviso` with 10.47 GB resident in shared memory.
- `scripts/land` on the workstation: the VM closed the connection, "the run
  on choir-agents died without an EXIT line".
- Node A has 31.9 GB and no swap. Configured guest memory: agents 12.5 GB,
  runner 12.5 GB, ws-owner 7.5 GB, ws-03 4 GB, ws-04 4 GB: about 40 GB.
- Resident at 06:43Z, after the kill: runner 10.5 GB, ws-owner 6.3 GB,
  agents 5.1 GB one minute after boot, ws-03 and ws-04 0.5 GB each; 6.6 GB
  available on the host.

## Belief

The memory overcommit (owner ruling N5 of the store-once goal) is safe only
while the build VMs are not both full. A guest never gives page cache back
to the host, so each build VM grows to its configured size during a suite
and stays there. For the first time tonight a landing ran on each build VM
at once while ws-owner held 6 GB, and the sum passed the host. The kernel
picks the largest process; it chose a build VM this time, and with other
numbers it can choose the runner mid-landing. The two assigned user VMs
(ws-03, ws-04) are small and so unlikely victims, but nothing guarantees it.

## What I am doing now

No heavy suite on the agents VM until this is repaired. Landings and
builder suites go through the runner only, one at a time. This halves the
landing rate.

## Remaining error and options

Not repaired. Options, for a deliberate Node A change (test, fresh login,
switch): a memory balloon or free-page reporting so idle guests return
cache; zram or disk swap on the host so the kill becomes slowness;
`OOMScoreAdjust` so the user VMs are never chosen and a build VM always is;
smaller build VMs. The protection for ws-03 and ws-04 is the one that
should not wait.

## Investigation (2026-11-15, 02:39Z to 03:05Z, read only)

### Evidence

- Host: MemTotal 32,630,604 kB (31.1 GiB), no swap device (`disks.nix`:
  `swapDevices = [ ]`), zram module present but unused, zswap off, KSM run=1
  with nothing shared (guest memory is shared memfd, KSM cannot merge it),
  no hugepages, no balloon on any VM. Memory pressure (PSI full) 1.3% over
  the last five minutes. 7.2 GB available.
- The configured sizes in the deployed config (`/root/mike-infra`, which
  evaluates to exactly the running system, `xasyjjzr...-nixos-system-node-a`):
  agents 10 GiB, runner 10 GiB, ws-owner 6 GiB, ws-03 3 GiB, ws-04 3 GiB,
  32 GiB in all on a 31.1 GiB host. The 12.5/7.5/4 GB in the first record
  were cgroup readings, which also count the host's page cache of each VM's
  disk image.
- Resident now (shared memory of each `cloud-hypervisor`): runner 10.0 GiB
  (since 01:57Z yesterday), ws-owner 6.0 GiB, agents 4.9 GiB (restarted at
  06:42Z, idle since), ws-03 0.45 GiB, ws-04 0.45 GiB. ws-01 and ws-02 are
  parked (stopped, disks kept).
- Every VM unit has `OOMScoreAdjust=0`, no `MemoryMax`, `MemoryHigh` or
  `MemoryLow`; so has `nix-daemon` (Node B's remote builds). `oom_score`
  today: runner 880, ws-owner 794, agents 772, ws-03 and ws-04 676 each.
  Nothing makes Mike's workstations a worse victim than the Choir VMs except
  their size.
- The 06:42:32Z kill, from the kernel's report: shmem 28.3 GB, of which the
  five guests held 27.5 GiB (agents 10.0, runner 10.0, ws-owner 6.0, ws-03
  and ws-04 0.45 each); all host processes together held 2.5 GiB anonymous
  memory; kernel (slab, page tables, stacks) about 0.6 GiB. The allocation
  that failed was a `compile` process in a container (uid 30010) in
  `user.slice`, at 0.4 GiB. Both build VMs were full to their configured
  size. The October kills of the same VM (five on 2026-10-23, before
  guest memory was shared) and one restic kill on 2026-10-30 are in the
  same log.
- What a heavy suite needs, measured inside the runner from 02:45Z to
  03:05Z while a landing ran `go test -race -p=1 ./internal/agentcore`
  (51 minutes in) and a builder compiled and tested beside it: guest
  anonymous memory peaked at 3.0 GiB (a `compile` at 1.7 GiB beside
  `agentcore.test` at 0.7 GiB; earlier `agentcore.test` 1.2 GiB with a
  `link` 0.6 GiB and `vet` 0.3 GiB), tmpfs at most 0.6 GiB, and at least
  5.6 GiB of the guest's 10 GiB always available. Over the runner's 25 hours of uptime its memory
  PSI totals 1.4 s and its OOM count is 0. Its IO PSI (full) is 13 to 22%
  and its CPU PSI under 1%: inside a build VM the stall is disk IO, not
  memory or CPU.
- The agents VM is up, idle (load 0.00), its home disk intact (99 of
  125 GiB used); it holds two `claude` processes (remote control).
- New and separate: the host's root filesystem is 98% full (920 of 953 GB;
  `/nix/store` 543 GB; the agents and runner disk images 119 and 47 GB and
  sparse, so they grow). Recorded in
  `node-a-root-filesystem-98-percent-full-2026-11-15.md`.

### Belief, corrected

The first record said the overcommit was safe while the build VMs were not
both full. It was never safe: a guest's configured memory is a commitment,
not an average. Guest RAM is a shared memfd, so each touched page is host
shmem that cannot be reclaimed without swap; a Linux guest fills its RAM with
page cache within one heavy run, and with no balloon it never returns it. So
every build VM reaches its configured size and stays there, and the only safe
rule is: configured sum of the VMs that start, plus the host's own needs, at
most RAM. The binding limit on build throughput is not memory per suite (one
heavy suite with a builder compiling beside it peaked at 3.0 GiB of
anonymous memory) but the 10 GiB each
build VM was given and then held as page cache.

### Remaining error

- The landing's own compile and link of `internal/agentcore` was not
  sampled (the run was already in its tests); the 1.7 GiB `compile` seen was
  a builder's. One heavy job in a 4 GiB VM is therefore expected to fit
  (about 1.7 GiB compile or 1.2 GiB test, 0.6 GiB tmpfs, 0.4 GiB for the
  `claude` processes), not shown; two heavy jobs at once in 4 GiB would not.
- Node B's remote builds (up to 3 jobs) and Mike's container builds were
  not measured; they are covered by a cushion and the OOM order, not by the
  budget.

Repair design and the patch: `docs/design/node-a-memory-budget-2026-11-15.md`.
