# Balloon stats are never read and there is no emergency inflation, 2026-10-21

Checklist item RES-10 (P2, red). Status: documented; design in
`docs/definitions/choir-launch-fp10-res-10-2026-10-21.md`. No fix yet.

## Evidence (code at e4b792b7f)

- `internal/vmmanager/balloon.go`: the balloon is configured with
  `amount_mib: 0`, `deflate_on_oom: true`, `free_page_reporting: true` and
  `stats_polling_interval_s` from `VM_BALLOON_STATS_POLLING_INTERVAL_S`
  (config.go:115), which is unset on every host. Firecracker only collects
  stats when the interval is non-zero at boot, and cannot turn them on later.
- `internal/vmmanager/fcapi.go` has no balloon call (no `GET /balloon/statistics`,
  no `PATCH /balloon`, no `PATCH /balloon/statistics`). Nothing in vmctl or
  vmmanager reads guest memory pressure from the VMM.
- The backend seam's `MemoryController.SetBalloonTargetMiB` is documented
  (`docs/reports/vm-backend-interface-2026-10-14.md`) but unimplemented.
- The rationale in `docs/reports/lr-guest-memory-return-2026-10-14.md:86-88`
  ("with `--no-api` nobody can read stats") is stale: snapshot hibernate mode
  always has an API socket (`playwright-in-capsules-memory-2026-10-15.md` §1.3;
  `memory_hotplug.go` `wantsAPISocket`).

## Consequence

Node B memory overcommit (see node-b-memory-overcommitted-swap-2026-10-09.md)
has no host-side signal about which guest holds reclaimable memory and no
lever to take it back short of killing the VM. Free page reporting returns
only pages the guest has already freed.

## Belief and unknowns

- UNVERIFIED: Linux 6.18 `virtio_balloon` with `deflate_on_oom` leaves guest
  `MemTotal` unchanged on inflate (affects `internal/gomemlimit` and the
  capsule admission budget, which read it once).
- UNVERIFIED: stats-on cost per VM (one 2-byte-aligned descriptor exchange per
  interval; v1.15.1 fixed oversized-descriptor stall #5794).
- Inflation is guest-cooperative: a hostile guest can ignore it, so it is a
  density lever, not an isolation control.

## Staging evidence, 2026-10-22 07:41 to 07:46 UTC (FP0)

A host memory stall 14 minutes after deploy H's planned refresh of the
paper (boot 07:30:42). The self-development monitor saw the vmctl list
call, a test computer's /health and the public API time out for 30 to 60 s
at 07:44 to 07:45, with host `/proc/pressure/memory` full stall about 18%
over the minute before 07:46; vmctl logged the paper's VM unhealthy at
07:41:38 and 07:45:23. Everything recovered by 07:46 with no kill. Read at
07:50: full avg300 6.02%, MemAvailable 9.1 GiB of 31 GiB, swap 15 GiB with
190 MiB used.

Firecracker resident sizes at 07:50: the paper (vm-1669020a, 8 GiB guest)
8,093 MiB, the owner's computer (vm-4a9afa23, stuck self-development
operation) 3,930 MiB, a candidate fleet VM 2,177 MiB, the test computer
(vm-c5c801f8, refreshed at 07:34) 1,618 MiB, another user's computer
1,330 MiB; go-choir-vmctl.service peak 25,614 MiB, current 19,690 MiB.

Inside the paper the guest reported MemAvailable 5.2 to 6.0 GiB the whole
time (FP0's boot-window sampler, every 20 s), so about 5 GiB of the 8 GiB
the host holds for it is guest page cache the guest would give back if
asked. Fourteen minutes after a cold boot the paper's VMM was already at
its whole guest size. That is this record's gap: the host has no signal
and no lever for memory a guest holds only as cache (RES-10 emergency
inflation, RES-08 shared store pages, RES-07's split). No new record; the
cause class is this one plus node-b-memory-overcommitted-swap-2026-10-09.md.
