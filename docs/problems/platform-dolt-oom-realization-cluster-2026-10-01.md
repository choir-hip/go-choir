# Clustering assessment: platform-dolt OOM → realization flap → serial-drain starvation

**Status:** ASSESSMENT 2026-10-01. Written under AGENTS.md Convergence Before
Patching — 3+ symptoms in the same subsystem within days trigger a
substrate-level review before the next fix. This document names the shared
root and the boundary; it is **not** a fix. Recorded after the overnight QA
drive surfaced a second blocking symptom on the same substrate.

## The symptoms (all on `computer-03335285269bdba4f94377e56879f9e6`, staging)

| Receipt | Surface | Date |
|---|---|---|
| `vmctl-pressure-reclaim-mid-respawn-gap-2026-09-28.md` | VM lifecycle / pressure reclaim | 09-28 |
| `vmctl-idle-sweep-hibernates-busy-guest-2026-09-28.md` | VM lifecycle / idle sweep | 09-28 |
| parked-run acceptance blocked (goal `choir-parked-run-reactivation-2026-09-30`, `excluded`) | wake drain starved by OOM | 09-30 |
| `api-key-computer-resolution-flap-2026-10-01.md` | `run start` 502 — realization `State!=active`/`ComputerURL` flap | 10-01 |

## Shared substrate

The platform-dolt store (`go-choir-platform-dolt`, host `choiros-b`) grows RSS
without bound (measured ~23.7 GiB peak, 28 GiB on a 31 GiB host per the Jev
metamission record), and the host kills it plus the retained guest VM on
memory pressure. Each kill drops the VM, which:

1. Flaps the proxy's realized computer record (`State`/`ComputerURL`), so
   `run start` 502s during the recreate window —
   `api_key_computer_authority.go:182`.
2. Reboots the guest, which re-arms the full pending wake outbox via
   `MigrateActorWakeOutbox` (runtime.go:2598) — then the serial drain races
   the next OOM before it reaches any given wake.

Both observed failures are symptoms of that cycle, not independent defects:

- **The `362febb2` parked-run strand** persists not because the fix is wrong
  but because no uptime window has outlasted a drain since deploy.
- **The overnight QA 502s** are the same flap surfacing on the submit path.

## Why substrate-level, not symptom-level

Patching the proxy (retry/backoff in `resolveComputerURLForComputerTarget`)
would hide the OOM behind a slower failure and is an orange-class routing
change needing its own rollback story. Patching the drain (concurrency) under
memory pressure risks worsening the kill frequency. Neither touches the root:
unbounded platform-dolt memory on a fixed-capacity host.

## The owed mission

A substrate stabilization mission, scope: bound the platform-dolt memory
footprint (cache/buffer cap, cgroup `MemoryMax`, or host capacity — an
ops/deploy-shape decision, not a silent code patch) AND bound the wake-drain
latency so a boot's re-armed queue clears within an uptime window. Acceptance:
sustained guest uptime sufficient to drain the wake outbox and complete a
`run start`, then observed `362febb2` passivated→running + packet consumed.

**Mutation class:** red-black edge — it touches deploy shape and VM lifecycle.
The memory cap is an ops tradeoff and is the one place a blocking owner ask is
permitted under the No Blocking Asks rule.

## Sequencing guard

Do not mutate guest lifecycle mid-acceptance. Either observe `362febb2`'s
transition first if a window appears, or ring-fence capacity changes so the
reactivation observation isn't confounded. Keep the parked-run repair goal
open with `deployed_acceptance` pending — the substrate mission's first win
(a stable window) discharges it as a side effect.

## Named residuals / dissent

- The 502 flap's causal link is **unconfirmed** — capture the resolved target
  row (`State`, `ComputerURL`) during an actual 502 before assuming it is the
  same substrate. It may be an independent routing defect that survives on a
  stable guest.
- If drain throughput provably cannot outlast an OOM window, the acceptance
  needs a deliberate action (e.g., a sanctioned manual drain trigger) recorded
  as a residual, not a blocking ask.
- `delivery_attempts` poisoning is a tail risk: rapid OOM cycles can push a
  pending control past `MaxDeferrals=64` into terminal — watch for
  `delivery_attempts_exhausted` on `4158e48b`.
