# A thrashing guest can obstruct other guests through the shared disk

Date: 2026-11-01. Found while closing the memory mission
(docs/definitions/choir-memory-optimization-2026-11-01.md). No fix yet.

## The requirement

Owner, 2026-11-01: "The guests will grow. We are building a platform for
users. Each user gets a vm. We will need more hardware very soon actually.
But obv I don't want to protect file cache. The point is that the host
shouldn't thrash, neither should the guests. But a thrashing guest shouldn't
obstruct other guests, ofc."

## What holds today

- Guest RAM is bounded by the guest's own size (mem_size_mib) and is
  Firecracker anonymous memory that never swaps (MemorySwapMax=0 on the vmctl
  unit and system-choirvms.slice). One guest cannot take another's RAM.
- The elastic ceiling (internal/vmctl/guest_balloon.go, guestmem ModeCeiling)
  trims only clean page cache and stands aside while a capsule or
  self-development is live, so it never forces a guest into anonymous-memory
  thrash.
- memory.low is 0 everywhere since 785868996: no file cache is protected.

## The gap

- Disk I/O is unweighted. A guest whose working set exceeds its RAM refaults
  its state.db from disk without limit; every guest's disk image is on the
  same host disk. The jailer passes only cpu.weight
  (internal/vmmanager/confinement.go:367); no io.weight or io.max is set, and
  no Firecracker drive rate_limiter is configured.
- Direct (unjailed) VMs, the paper among them, run together in the vmctl
  unit's cgroup (nix/node-b.nix, vmctl Delegate/supervisor), so no cgroup
  control can separate them; jailed VMs have one cgroup each under
  system-choirvms.slice/vms/<vm>.
- Nothing measures a guest's own thrash: the memory meter (go-choir-memory-meter)
  records per-VM RSS and read_bytes and host memory pressure, not per-VM
  pressure.

## Directions (not started)

1. Per-VM io.weight (the same proportional share as cpu.weight) on the jailed
   cgroup; move the remaining direct VMs into the jailed slice (SEC-04 waves).
2. A Firecracker drive rate_limiter as a hard ceiling only if weights prove
   insufficient (a limiter also slows a guest when the disk is idle).
3. Record per-VM cgroup memory.pressure and io.pressure in the memory meter,
   and read refault rates from the guest's ceiling acts, so "a guest thrashes"
   and "a guest obstructs others" become numbers.
