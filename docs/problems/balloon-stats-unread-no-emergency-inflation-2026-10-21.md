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

## Failure modes before code (steps 1 and 2, 2026-10-22, lane fp10-res10-reader)

Written before the tests and the code. Step 1 is stats on per computer
(`VM_BALLOON_STATS_ONLY`, the FP8 selector of `computer_select.go`); step 2
is the read-only reader. Step 3 (inflation) is out of scope.

Facts this list rests on (code at 58e705802): staging runs
`VM_BALLOON_FREE_PAGE_REPORTING=1` and `VM_HIBERNATE_MODE=snapshot`
(nix/node-b.nix), so every guest has a balloon and an API socket; the
snapshot launch-config hash (`snapshot.go` `launchConfigHash`) is the sha256
of the whole Firecracker config, balloon included, recomputed from today's
selection at restore (`expectedLaunchConfig`); Firecracker saves the balloon
device's config, stats interval included, in the snapshot and restores it.

1. **Stats on for an unselected computer.** `VM_BALLOON_STATS_POLLING_INTERVAL_S`
   alone (today's global meaning), a selector parse slip, or an empty list
   read as "all" turns stats on everywhere. Every guest's config and every
   snapshot hash change: the paper and the owner's computer cold boot at
   their next wake. Guard: the interval applies only to computers the list
   names; empty list, no stats.
2. **Unselected config not byte-identical.** A new key, an int that becomes
   another type, or a changed default alters the JSON of every launch and
   refuses every snapshot (`snapshot_reason=launch_config`). Guard: a test
   compares the unselected balloon object and the whole config JSON with
   today's bytes.
3. **A selected boot Firecracker refuses.** An interval out of u16 range or
   an extra field (BalloonDeviceConfig denies unknown fields) fails the boot.
   Guard: the interval is validated 1..65535 at load, the field set is
   unchanged.
4. **Selection with the balloon off.** Selecting stats must not add a balloon
   device (that is the free page reporting rollout's switch). Guard: no
   balloon, no stats; the reader then says "no balloon device".
5. **A restored guest reports zeros or the wrong state.** A restored guest
   keeps its saved device: a guest snapshotted without stats has none after
   restore. Selecting it changes its hash, so its snapshot is refused and it
   cold boots once with stats (the same cost as `VM_GUEST_RECLAIM_ONLY`);
   deselecting does the same in reverse. The reader must ask the VMM what
   the device has (`GET /balloon`, `stats_polling_interval_s`), not trust
   what vmctl believes it launched (vmctl restarts reattach without the
   launch config). Interval 0 reports "stats disabled at boot", never zeros.
6. **Absent fields decoded as 0.** Before the guest driver's first report,
   or for a tag the guest kernel does not send, Firecracker omits the field.
   A plain `uint64` would show 0 available memory, which a later inflation
   step would read as "this guest has nothing to give" or worse "total 0".
   Guard: optional fields are pointers; an absent `available_memory` or
   `total_memory` is the reason "guest has not reported stats yet".
7. **No API socket** (kill hibernate mode without a hotplug device, a
   jailed VM whose socket path is the jail's). Reason "no API socket", via
   `runningAPISocket`, which already resolves the jailed path.
8. **A stuck VMM stalls vmctl.** An API call that never answers holds a
   handler goroutine. Guard: the read has a 2 s deadline for both calls; no
   vmctl loop calls it (on demand only); the all-VM read fans out in
   parallel so one stuck VMM costs the request 2 s, not 2 s times N.
9. **Racing a lifecycle operation.** Reading while boot, hibernate or
   restore holds the VM could talk to a socket being torn down. Guard:
   `tryLockVMOperation`; busy is the reason "VM operation in progress", it
   never waits.
10. **A guest reads the host's view of other guests.** Co-tenant memory use
    is a side channel. Guard: host-only route (`isInternalCaller`: loopback
    or the unix socket plus the host credential); no guest route; no proxy
    path.
11. **Mutation creep.** The reader must never send `PATCH /balloon` or
    `PATCH /balloon/statistics` (the latter changes the interval live).
    Guard: a test's fake socket fails on any method but GET.
12. **Unit confusion.** Firecracker reports `free_memory`, `available_memory`,
    `total_memory`, `disk_caches`, `swap_in`, `swap_out` in bytes,
    `target_pages`/`actual_pages` in 4 KiB pages, `target_mib`/`actual_mib`
    in MiB, and faults as counts (docs/ballooning.md, v1.15). The reader
    keeps Firecracker's names and units and says so; the probe converts to
    MiB once, labeled.
13. **Errors that look like readings.** Every failure is a reason string
    with no numbers beside it; a reading carries `observed_at`.
14. **Stats cost.** UNVERIFIED (problem record above): one descriptor
    exchange per interval per selected guest; selection keeps it to test
    computers until FP0 has measured it.
