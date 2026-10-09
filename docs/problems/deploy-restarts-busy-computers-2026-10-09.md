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

## Refinement: human focus and an update prompt (owner, 2026-10-09)

Owner: "we should try to avoid rebooting if we can detect the human user's
focus. Like, if there's been any input in the last short time interval."
And: "we could show like a little pop up on the screen saying you know
update now or you know wait five minutes kind of thing."

- **Focus signal.** Activity means real human input: keyboard, pointer,
  touch or scroll while the Choir tab is visible and focused. Background
  polling and an open idle tab do not count. The frontend reports input
  to its computer at most every 30 s while input is happening; the guest
  exposes `last_owner_input_at` on `/health`. Today no such signal exists:
  `last_active_at` moves only on ownership resolution, and the frontend
  sends no presence.
- **Who restarts silently.** Only a computer with no running runs and no
  human input in the last 10 minutes.
- **Everyone else gets a prompt.** A small notice in the desktop: "An
  update is ready. Update now / In 5 minutes." "In 5 minutes" asks again
  later. "Update now" is a planned restart. Until SL's planned-restart
  marker lands (so work resumes), the prompt is offered only when no runs
  are running; while work runs, the computer waits.
- **How the frontend knows.** It needs the computer's running guest commit
  and the guest commit the host would boot now. vmctl knows both (the
  booted base and the current base); an owner endpoint reports them and
  takes the "Update now" request.

## Known weak spot

"Busy" means running runs. An obligation stuck in a loop can keep a
computer busy forever and block its updates. SL's retry budgets close that.

## Fix shape, in slices

1. **Focus signal:** frontend input reporter (visible and focused tab,
   throttled), guest `last_owner_input_at` on `/health`.
2. **Deploy restarts only idle computers:** read `running_runs` and
   `last_owner_input_at`; skip busy or recently used computers and list
   them; a guest that does not answer is skipped, not restarted. Forced
   deploys follow the same rule.
3. **Update prompt:** owner endpoint reporting running vs available guest
   commit and accepting "Update now"; desktop notice with "Update now / In
   5 minutes".
4. **Planned-restart marker and resume:** part of SL. Until then, no
   deploy or prompt restarts a computer with running work.

## Interim slice verified (2026-10-09 ~17:00Z)

f6001aea (deploys never restart running computers) was first exercised by
run 37961856452 (e68c8c36, a flake change classified as host OS plus guest
boot contract). The deploy succeeded and logged "Update pending, not
restarted" for both active interactive computers; their epochs were the
same before and after (12953, 12954). Staging reports e68c8c36. Slices
1–3 (focus signal, idle-only restart, update prompt) remain.
