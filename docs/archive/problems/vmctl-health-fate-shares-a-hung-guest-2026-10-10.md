# vmctl health waits on every guest, so one hung guest fails the deploy and degrades choir.news (2026-10-10)

## Evidence

- Deploy of 17b97b8f (CI run 38064706897) failed at "vmctl did not become
  healthy after restart". The host services were installed (proxy and
  corpusd report commit 17b97b8f), vmctl was active, and it had reattached
  three guests with no destruction.
- From 15:51:07Z the owner's computer accepted TCP on its service port but
  never answered `/health` (vmctl: "adopting ... unready: alive, health
  probe unanswered", epoch 1188 unchanged). It started during the host
  NixOS switch, which restarted the platform, gateway, auth, mail and proxy
  services between 15:50 and 15:51. The rerun 13 computer refused
  connections and the wedge watchdog stopped it at 16:00:18 (as designed).
- `curl http://127.0.0.1:8083/health` on Node B answered 200 in 8.01 s,
  three times out of three. The proxy probes vmctl health with a 2 s
  timeout, so `https://choir.news/health` reports `vmctl_status:
  unavailable`, `status: degraded`, and the deploy gate's vmctl health
  wait times out.

## Cause (code reading)

`vmctl.HandleHealth` calls `CheckIdleOwnerships` and `PressureReclaimPlan`.
Both run `guestBusy()`, which GETs every active guest's `/health` in turn
with a 4 s timeout. One guest that accepts the connection but does not
answer costs 4 s in each, so 8 s per vmctl health call. vmctl's own health
shares fate with its slowest guest; the deploy gate and the public health
then report the host down because one computer is slow.

## Open: why the owner's computer stopped answering at the switch

Counts and timings only (owner's computer): the process is alive (about
7% CPU, 3.9 GB resident) and accepts TCP, but `/health` does not answer.
Hypothesis: a request it held to a platform or gateway service that the
switch restarted left a lock held. This is the application-level wedge
residual: the watchdog only acts on refused connections. Not acted on; the
owner decides about their computer.

## Fix

vmctl health reports the busy answers recorded by the last idle sweep and
never probes a guest itself. A guest with no recorded answer counts as busy
(fail closed, as the sweep does). The sweep and reclaim still probe guests
live. Mutation class red (vmctl); rollback is git revert.
