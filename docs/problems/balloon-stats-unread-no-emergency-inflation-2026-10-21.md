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
