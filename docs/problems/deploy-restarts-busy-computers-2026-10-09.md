# Deploys restart busy computers (2026-10-09)

Mutation class of the fix: red (deployment routing, VM lifecycle).

## Problem

Every deploy that changes guest runtime code restarts every active
interactive computer, one at a time, whether or not it is working
(`.github/workflows/ci.yml`, "Refreshing active interactive computers",
`POST /internal/vmctl/refresh` per computer). A forced deploy does the same
(`force_staging_deploy` sets `deploy_active_vm_refresh=true`).

This got worse on 2026-10-09: a47122de sent every guest-runtime change
through this path and removed the app-layer offer, which a guest could
refuse. The forced deploy of 799097e3 (run 37942423384, 14:25Z) restarted
every active computer, the owner's included.

Owner, 2026-10-09: "We shouldn't push a release that reboots live
computers and interrupts them."

## What already exists

- The idle sweep (`vmctl/warmness_policy.go:158`, every 2 min, 30 min idle
  timeout) skips a computer whose guest reports running runs. Its
  "hibernate" stops the VM and keeps the disk; the next visit boots it on
  the current base, so it already picks up new code.
- Real memory-snapshot hibernation does not exist yet
  (`vmmanager/manager.go:927`). When it does, resuming a snapshot would
  restore old code, so a computer with a pending update must boot fresh.

## Considerations (owner discussion, 2026-10-09)

1. Never interrupt work.
2. Fixes must arrive, including for staging acceptance.
3. Unbounded skew between running guests and host services is a
   maintenance burden, so the update window must be bounded.
4. Real hibernation will not update anything by itself.
5. Gate 2 already treats platform updates as offers (S6 divergence guard).
6. Updating at idle moments, not all at once, limits a bad deploy's blast
   radius.

## Decision: update when idle

Owner-approved middle way:

- A deploy rebuilds the base and restarts **only idle computers**: no
  running runs and no owner activity for 10 minutes. Busy computers are
  skipped and listed; they update at their next idle moment through the
  existing idle sweep.
- A restart for an update is a **planned restart**: the host writes a
  durable marker before it, and the guest consumes it once at boot and may
  resume work. A boot without a marker is a crash, and open work closes as
  "interrupted by a restart" (AGENTS.md "Restarts End Work (Crash) Or
  Resume It"; implemented in SL).
- Later, with real hibernation, a computer with a pending update boots
  fresh on wake instead of restoring memory.
- An urgent security fix shows the owner an "update needed" notice. There
  is never a silent forced restart (policy deferred until needed).
- A computer that is never idle for several days shows a notice (later).

## Known weak spot

"Busy" means running runs. An obligation stuck in a loop can keep a
computer busy forever and block its updates. SL's retry budgets close that.

## Fix shape

1. Deploy refresh loop: before refreshing, read the guest's `/health`
   `running_runs` and the ownership's `last_active_at`; skip a computer
   with running runs or activity in the last 10 minutes; list skipped
   computers in the deploy log. A guest that does not answer is skipped,
   not restarted.
2. Forced deploys follow the same rule.
3. The planned-restart marker is part of SL, not this change. Until it
   lands, the idle-only rule means no work is interrupted by a deploy.
