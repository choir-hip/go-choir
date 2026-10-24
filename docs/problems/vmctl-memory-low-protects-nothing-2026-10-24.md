# vmctl's MemoryLow Protects Nothing: system.slice Has memory.low 0

Date: 2026-10-24, 00:5x UTC. Found by the VM slice design worker
(lane/fp0-vm-slice), which saw it on the agents VM. FP0 confirmed it
read-only on Node B. Mutation class of any fix: orange (host memory
policy), red if it changes VM lifecycle.

## Evidence (Node B, read-only)

- `/sys/fs/cgroup/system.slice/go-choir-vmctl.service/memory.low` =
  15032385536 (14 GiB); `systemctl show go-choir-vmctl` MemoryLow=15032385536,
  MemorySwapMax=0.
- `/sys/fs/cgroup/system.slice/memory.low` = 0 and `memory.min` = 0;
  `systemctl show system.slice` MemoryLow=0.

## Why it matters

In cgroup v2 a cgroup's effective low protection is bounded by its parent's.
`memory.low` is "best-effort memory protection ... up to the parent's
effective low" (kernel admin-guide cgroup-v2, Memory Interface Files and
"memory.low"). With `system.slice` at 0, the vmctl unit's 14 GiB, and so the
paper's and every guest's Firecracker memory, gets no protection from
reclaim when the host is under pressure. The setting reads as if it works,
and it does not. The memory cap and swap settings are separate and
unaffected.

## Not yet known

- Whether reclaim pressure on Node B has ever hit the guests because of
  this. The 10-22 OOM records were the agents VM on node-a, not Node B.
- What other slices on Node B hold memory that would compete.

## Fix directions (to decide)

1. Set `MemoryLow=` on `system.slice` (a NixOS `systemd.slices.system`
   override) to at least the sum its children ask for. Today that is
   vmctl's 14 GiB, plus 4 GiB for the VM slice if lane/fp0-vm-slice lands.
2. Use `DefaultMemoryLow=` on `system.slice`.
3. Drop the unit's MemoryLow as dead configuration, if protection is not
   wanted.

Every option is a host memory policy change. Read the current
memory.pressure and the reclaim counters first, under the "investigate
eagerly; act deliberately" rule.

## Readings and decision (2026-10-24, 17:4x to 18:1x UTC)

Owner: "Go with memory protection." Node B, read only:

- 31 GiB of RAM and a 16 GiB swapfile (207 MiB used); the vmctl unit and
  the VM slice have MemorySwapMax=0, so guest anon memory cannot be
  reclaimed at all. What protection keeps is the guests' file cache.
- 45.7 h of uptime: memory PSI full 3,067 s (about 1.9% of wall time),
  allocstall 187k, pgscan_direct 30.2M, oom_kill 0.
- system.slice 25.6 GiB current (17.4 anon, 4.9 file), 29.4 GiB peak.
  vmctl unit: 14.7 GiB anon, 2.1 GiB file; workingset_refault_file 216M
  pages since boot (about 860 GB of the guests' disk cache read back in).
  Platform Dolt: 1.8 anon, 0.8 file, at its 4G MemoryHigh. VM slice empty.

Decision (fix direction 1):
- `system.slice` MemoryLow=18G, as a NixOS drop-in (`asDropinIfExists`).
- `system-choirvms.slice` MemoryLow 4G -> 14G, the same as the vmctl unit,
  so protection follows guests as the jailer waves move them. The kernel
  shares the parent's 18G between the two children by protected usage.
- The vmctl unit's 14G stays until every computer is jailed; then it
  shrinks to vmctl's own size.

Checked before the deploy (bunch 192): Node B mounts cgroup2 with
`memory_recursiveprot`, so protection reaches the leaf groups vmctl creates
(`supervisor`, `vms/<vm>`), which set no memory.low of their own. NixOS
already writes `system.slice` as a full unit file (FragmentPath
/etc/systemd/system/system.slice, empty), so the change modifies that file
(slices apply on daemon-reload and are never restarted) and carries
`X-StopOnRemoval=false` against the removal path. Under recursiveprot the
unclaimed part of the 18G goes to the other services by usage, so the
guests' effective share can be below 18G.

Baseline (18:00 to 18:10 UTC, 601 s): the vmctl unit refaults 14.1 MiB/s of
file cache, system.slice 20.2 MiB/s.

Proof: after the deploy, `/sys/fs/cgroup/system.slice/memory.low` reads
19327352832, `system-choirvms.slice` reads 15032385536, and the vmctl unit's
refault rate is read again against the baseline. Rollback: MemoryLow "0" in
the config, or live `systemctl set-property --runtime system.slice
MemoryLow=0`.
