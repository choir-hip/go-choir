---
definition_version: 4

# Station FP10, item RES-10, of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md). Item: balloon stats
# reader and emergency inflation (security-density checklist, P2, red).
readiness: drafted

review:
  reviewer: none yet
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T00:00:00Z
  source:
    canonical_ref: lane/fp10-res10-balloon from main e4b792b7f
    deploy_identity: unknown to this lane
  worktrees:
    - path: /home/agent/lanes/fp10-res10-balloon
      status: clean
      class: goal_candidate
      owner: FP10 RES-10
      touch: goal_owned
      recovery: branch lane/fp10-res10-balloon

finish:
  deliver: >-
    A design, not code: how vmctl reads guest balloon statistics and, behind
    an off-by-default flag, inflates a guest balloon under host memory
    pressure. Code waits for FP8 (lane/fp8-image-jailer) to land its
    vmctl/vmmanager/image changes.
  artifact: this file and docs/problems/balloon-stats-unread-no-emergency-inflation-2026-10-21.md (green)
---

# Why design only

Every part of the slice sits in files FP8 is changing: `fcapi.go` (new
`GET /balloon/statistics`, `PATCH /balloon`), `balloon.go` and `config.go`
(stats interval default), `manager.go` (boot config), and the confined/jailed
API socket path (`confinement.go`). Building it here would conflict on merge.
The item is also red (VM lifecycle) and has an owner-visible tradeoff (below).

# Design

Three steps, each shippable alone, each off until FP0 proves it on staging.

1. **Stats on (observe).** Set `VM_BALLOON_STATS_POLLING_INTERVAL_S=5` in the
   host env. Boot-time only, so it takes effect as guests cold-boot or
   restore; snapshot-restored guests keep their saved device config
   (UNVERIFIED: whether a restored balloon honors a changed interval; the
   probe below answers it). No code change. Rollback: unset.
2. **Reader.** Add `fcAPIClient.BalloonStatistics(ctx)` decoding
   `GET /balloon/statistics` (`target_pages`, `actual_pages`, `free_memory`,
   `available_memory`, `total_memory`, `swap_in/out`, `major_faults`,
   `disk_caches`). Expose it as `Manager.GuestMemoryStats(vmID)` through the
   existing `runningAPISocket` helper (memory_hotplug.go), and export per-VM
   gauges on the vmctl metrics surface. Read only, no mutation. Test first as
   an httptest fake of the socket, then one E2E: boot a guest on staging,
   fetch stats, assert `available_memory > 0` and bounded by `total_memory`.
3. **Emergency inflation (red, flag `VM_BALLOON_EMERGENCY=observe|on`,
   default off).** `Manager.SetBalloonTargetMiB` = `PATCH /balloon
   {amount_mib: N}`, N capped to `mem_size_mib - floor` (floor from the
   hotplug controller's base, never below guest working set + 256 MiB from
   stats). A vmctl pressure loop (host `MemAvailable` below a threshold for
   N consecutive samples) picks the guest with the largest
   `available_memory`, steps its target up, and deflates when pressure
   clears. `observe` logs the decision and the would-be target without
   calling PATCH. Never inflates a guest with a live engineering capsule
   (CLAUDE.md: no disturbance mid self-development apply). Inflation never
   closes work; it creates no obligations, so no restart-marker issue.

# Owner decisions and interactions

- **RES-09 overlap.** virtio-mem leases and the balloon both move guest
  memory; running both needs one controller owning the target. Recommend step
  3 waits for RES-09 proof, and only step 1 and 2 ship first.
- **`MemTotal` and `deflate_on_oom`** (see problem record): verify on 6.18
  before inflating, since GOMEMLIMIT and capsule admission read `MemTotal`
  once at boot.
- Guest cooperation is not guaranteed; do not present this as isolation.

# FP0 steps (live, after FP8 lands)

1. `scripts/` probe to write then (not authored here to avoid colliding with
   FP8 paths): boot a test computer with the interval set, `curl --unix-socket
   <api.sock> http://localhost/balloon/statistics`, record JSON; then
   `PATCH /balloon {"amount_mib":256}`, confirm `actual_pages` converges and
   guest `MemTotal`/`MemAvailable`; PATCH to 0 and confirm deflation.
2. Record whether a snapshot-restored guest reports stats.

# now

Design written; problem record committed. Blocked on FP8 landing for any
code. Residuals: the three UNVERIFIED claims in the problem record. No owner
question needs an answer before step 1 and 2.
