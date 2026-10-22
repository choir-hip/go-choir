---
definition_version: 6

# Station FP10, item RES-10, of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md). Item: balloon stats
# reader and emergency inflation (security-density checklist, P2, red).
readiness: built_steps_1_2

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
    - path: /home/agent/lanes/fp10-res10-reader
      status: clean
      class: goal_candidate
      owner: FP10 RES-10 steps 1 and 2
      touch: goal_owned
      recovery: branch lane/fp10-res10-reader (from main 58e705802)
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

1. **Stats on (observe), per computer.** (Revised 2026-10-22: not host-wide.
   A host-wide interval changes every guest's config and every snapshot
   hash, cold booting the paper and the owner's computer.)
   `VM_BALLOON_STATS_ONLY` (the FP8 selector) names the computers;
   `VM_BALLOON_STATS_POLLING_INTERVAL_S` is their interval, default 5.
   Unselected computers keep their exact config. A selected hibernated
   computer's snapshot is refused by hash and it cold boots once; a
   restored guest keeps its saved device. Rollback: unset the selector.
2. **Reader.** Add `fcAPIClient.BalloonStatistics(ctx)` decoding
   `GET /balloon/statistics` (`target_pages`, `actual_pages`, `free_memory`,
   `available_memory`, `total_memory`, `swap_in/out`, `major_faults`,
   `disk_caches`). Expose it as `Manager.GuestMemoryStats(vmID)` through the
   existing `runningAPISocket` helper (memory_hotplug.go). Built surface
   (revised 2026-10-22): not gauges. vmctl has no metrics endpoint (no
   Prometheus or expvar surface), and the existing per-VM memory readings
   are host-only JSON routes (`/internal/vmctl/memory`, `/internal/vmctl/list`
   memory fields), so the reading is the JSON route
   `GET /internal/vmctl/memory/balloon[?vm_id=|computer_id=]` beside them:
   one entry per computer with a reading or a reason, read on demand (no
   sampling loop, so a stuck VMM never stalls vmctl). Read only, no
   mutation. Test first as an httptest fake of the socket, then one E2E:
   boot a guest on staging, fetch stats, assert `available_memory > 0` and
   bounded by `total_memory`.
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

# Built (lane fp10-res10-reader, 2026-10-22)

Steps 1 and 2, inert until selected; step 3 not built. What a selected and
an unselected boot send, the reader's reasons, the vmctl route
(`GET /internal/vmctl/memory/balloon`, host only), the probe
(`scripts/res10_balloon_stats_probe.sh`), rollback and FP0's staging steps
are in the problem record's "Built" and "FP0 staging steps" sections.
Tests: `go test -race ./internal/vmmanager` (whole package),
`go test -race ./internal/vmctl -run 'Balloon|Memory'`,
`go test ./internal/vmctl ./cmd/vmctl`.

# now

Steps 1 and 2 built and pushed on lane/fp10-res10-reader; FP0 lands and
runs the staging steps (one test computer selected, cold boot, probe,
restore question). Step 3 waits for that evidence and for RES-09 (one
controller owning guest memory targets). Residuals: stats cost per guest
(UNVERIFIED), `MemTotal` under inflation (step 3), whether a restored
balloon keeps its interval (FP0 step 4 answers it; code is honest either
way); `class:` tokens do not match the platform computer or a resumed boot
(problem record, "Residuals"). Review round 2 (bunch 40) closed: probe
sends no credential to guests and fails loudly on non-2xx; no_api_socket
names its path. No owner question blocks steps 1 and 2.
