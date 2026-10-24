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
