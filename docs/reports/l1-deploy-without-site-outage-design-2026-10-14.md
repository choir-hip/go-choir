# L1: Deploy Without Taking The Site Down

Date: 2026-10-14. Station: L1 (24/7 paper, owner ruling 6). Problem:
`docs/problems/deploy-takes-staging-down-during-gateway-drain-2026-10-14.md`.
Mutation class of the change: red (deployment routing). This note is
green.

## What the code does today

A host deploy (`.github/workflows/ci.yml`, Deploy to Staging) runs in
this order:
1. `switch-to-configuration switch`. Every go-choir unit whose unit file
   changed is stopped in the stop phase and started in the start phase.
   The unit files change on every closure, because `ExecStart` names a
   wrapper script in the store (`serviceExec`, `nix/node-b.nix:56-67`).
2. `node-b-sync-service-pointers auth gateway maild corpusd proxy vmctl
   checkpointd`. Each service's binary pointer under
   `/var/lib/go-choir/services/<name>` moves to the new build.
3. `systemctl restart go-choir-auth go-choir-maild go-choir-corpusd
   go-choir-gateway go-choir-proxy`.

The wrapper execs the pointer when it exists (`serviceExec`), so the
restart in step 1 starts the old binaries. Only step 3 loads the new
ones. Step 1 is therefore a restart that does nothing useful, and it is
the one that takes the site down: the stop phase waits on the gateway's
graceful drain (`TimeoutStopSec = "11min"`) before the start phase brings
the proxy back. Tonight that was 3, 4 and 11 minutes.

## Decision

**D1. The switch leaves the five services running.** Set
`restartIfChanged = false` on go-choir-auth, -maild, -corpusd, -gateway
and -proxy. The switch then installs their new unit files and does not
stop them. Step 3, after `daemon-reload`, is the one restart, and it
loads both the new unit files and the new binaries.
- A first boot or a unit that is not running is still started by the
  switch (`restartIfChanged` governs only restarts of running units).
- vmctl and checkpointd keep their current handling, which is outside
  this change.
- Precedent: vmctl got exactly this change for the same cause in
  `2f755bb3` ("a deploy restarts vmctl once, on the new binary"): its
  switch-time restart ran the old pointer and doubled every deploy's
  restarts. D1 extends that to the other five services. Because the
  pattern is settled and proven on vmctl, this change goes without a
  panel.

**D2. Step 3 restarts the gateway last, on its own.**
- First `systemctl restart` auth, maild, corpusd and proxy together.
  Their ordering (`after`, `requires`) is unchanged, and none of them
  waits on the gateway.
- Then `systemctl restart go-choir-gateway`. While it drains, the site,
  the API and auth are up. Only new model calls from computers are
  refused until the new gateway starts, as today.
- The deploy's health check then waits for the gateway as it does now.

**D3 (not in this change).** Keeping the gateway accepting during its
own drain means two processes overlapping on one listener (socket
handover or `SO_REUSEPORT` with two unit instances). That is a separate
design. Its problem is model calls refused during a drain, which this
change neither causes nor fixes.

## Rollback

Revert the commit. The next deploy runs the old order. No state changes.

## Ways this could fail (written before the code)

1. A unit-file change that needs a restart does not get one. Step 3
   restarts all five after `daemon-reload`, on every host deploy.
   Checked: the deploy's final service checks read the new unit's
   `ExecStart` (systemctl show) for each of the five.
2. A deploy path that switches but skips step 3. Read: the service-pointer
   path restarts per service; the host path runs step 3; the frontend-only
   path does not switch. Any new path must restart. A test in the deploy
   script's own checks: after a host switch, each of the five units'
   `ActiveEnterTimestamp` is later than the switch.
3. A dependency now restarts out of order (the proxy requires auth). The
   proxy, auth, maild and corpusd restart in one `systemctl` call, so
   systemd orders the jobs.
4. The gateway restart fails. The deploy fails, as today, and its
   diagnostics dump runs. The site stays up.
5. The first deploy after this change may still restart the units at
   the switch, depending on whether the switch reads the option from the
   old or the new unit file. The proof therefore uses the second deploy.

## Proof on staging

Two deploys after the change lands. During the second, a probe polls
`/health` every 2 seconds from before the switch to the end of the
deploy, and the journal timestamps of each unit's stop and start are
recorded. Pass: no gap over 10 seconds in `/health` while the gateway
drains, and each of the five units runs its new binary (the deployed
commit in `/health`, and each pointer's target).

## Deployed proof (2026-10-14)

Deploy of `ecc51d79` (CI run 38495519250; it carried the change
`2db4bc02`), deployed 01:39:07 UTC.
- The switch, at 01:34:31, restarted none of the five services. The option
  took effect on the first deploy, so failure mode 5 did not occur.
- The one restart came after the pointer sync. At 01:36:05, auth, maild,
  corpusd and proxy restarted together within the same second. The
  gateway then drained from 01:36:05 to 01:38:44 (2 min 39 s) and
  started again.
- A probe polled `/health` every 2 seconds from 01:24:16 to the end of
  the deploy: 417 samples, all 200, with no gap. Before this change, the
  three deploys earlier that night took the site down for 3, 4 and 11
  minutes. The probe log is at
  `~/.config/choir-qa/traces/l1/`.
- Each of the five units runs its service pointer (`/var/lib/go-choir/
  services/<name>/bin/<name>`). `/health` reports `commit` and
  `deployed_commit` `ecc51d79`.
- Separately: the first switch attempt failed with status 4 at 01:36:03
  ("Reload failed for D-Bus System Message Bus"), and the deploy's
  existing one retry succeeded. This happened before and is unrelated.
- Residual (D3): model calls from computers are refused while the gateway
  drains, for 2 min 39 s on this deploy and up to 11 min.

## Residual D4 (2026-10-14 07:46): a 10-second proxy drain

Deploy ce59d406: 251 of 256 health probes answered 200; five 502s from
07:46:12 to 07:46:21. The journal shows `Stopping go-choir Proxy Service`
at 07:46:11 and `proxy: server stopped` at 07:46:21: the proxy took 10 s
to stop, and requests in that window were refused. Hypothesis: a
long-lived connection (live events or SSE) held the graceful shutdown open
until its timeout. Earlier deploys tonight saw zero or one 502. Candidate
fixes: close idle and streaming connections at the start of shutdown so
the drain is short; or socket activation so the new proxy accepts before
the old one stops.
