# Deploy of 97e023100 (P4): nixos switch 24 min, vmctl crash-looped on its route-database ping, deploy marked failed (2026-10-15)

Date: 2026-10-15 10:08-10:48 UTC. CI run 38717028854 (head
`97e023100`, P4 standing consensus; code on top of `f8007f5d6`). Status:
documented; evidence only, no fix attempted. Found by the overnight lander.

## Observed

- Deploy job started 10:08:58Z. "Activating NixOS config" began about 10:09;
  `Deploy service diagnostics: nixos switch failed before retry` at 10:38:55
  (about 29 min); frontend and host services then installed.
- `Restarting vmctl when selected classes require it` at 10:40:43; at 10:43:43
  `vmctl did not become healthy after restart`; the job exited 1 and recorded
  `/var/lib/go-choir/deploy-failures/38717028854-1.json` (`status: incomplete`,
  `target_commit 97e0231…`, exit 1).
- vmctl journal 10:42:23-10:45:43: the unit started, logged its config lines,
  and exited `status=1/FAILURE` 30 s later on every start:
  `vmctl: ping ComputerVersion route database: context deadline exceeded`.
  Seven restarts, then `start-limit-hit` (burst 10 / 10 min). A new start at
  10:47:44 (after the limit window or a reset, by someone other than the
  lander) came up healthy: staging `/health` `ok`, `vmctl_status ok`.
- The vmctl unit's cgroup reported 29.1 GB memory peak and 10.7 GB swap peak
  on each failed start; Node B load average was 11-12 during the window and
  2.4 at 10:48. `firecracker` processes survive vmctl exits (managed VMs are
  left running by design): the paper computer
  `vm-1669020adcece51c2f5ff9b6df399ef5` was reattached at 10:47:44
  (`ready`, 0 running runs), after one transient `health check failed` at
  10:40:42.
- Staging `/health` `deployed_commit` stayed `f8007f5d6`; the new proxy binary
  was serving (`commit 97e023100`).

## Classification (hypotheses; the platform route database was not queried)

The 30 s `ComputerVersion` route-database ping is the proximate cause of each
exit. It timed out while the host was heavily loaded by the nixos switch
(which restarts the platform database server). So the deploy failure is a
startup-ordering/timeout problem between vmctl and the route database under
host load, not a P4 code failure: vmctl's own package was unchanged in this
deploy (`embedded_commit 2eae77de6…`). Whether vmctl should retry the ping
instead of exiting 1, and whether the deploy script's vmctl health wait
(about 3 min) should outlast `start-limit-hit`, are open design questions.

## Consequences

- P4's host-side change is partially live (proxy at `97e023100`) while the
  deploy was recorded incomplete. A rerun of the failed job on the same SHA is
  the clean way to complete it.
- The restart loop was a visible outage of the vmctl-routed surface for about
  5 minutes (`/health` `degraded`, `vmctl_status unavailable`, 502 at
  10:40) on staging, with the paper computer running throughout.

## Heresy delta

discovered: vmctl exits instead of retrying its route-database ping (a
startup crash loop under load); introduced: none; repaired: none.
