# Problem Receipt: Guest CPU Spin After Replay (2026-09-03)

- Date: 2026-09-03
- Mutation class of this receipt: green (documentation); any repair is red
- Status: root-caused (OOM loop); repair in flight (4 CPU / 8 GiB guests)
- Computer: `computer-03335285269bdba4f94377e56879f9e6`, epochs 866 (normal
  boot, stopped) / 867 (held maintenance-serve, OOM-looping)

## The Problem

Every boot on the retained computer's drive that gets PAST event replay
degenerates into a sustained ~190% CPU spin (2 vCPUs pegged) with no further
log output, starving the guest HTTP server: vmctl health checks fail, the VM
is marked degraded, and all product paths return "failed to resolve user
autoputer". Observed on:

- 19:10Z normal boot (fix `a4fae242`): replay passed seq 138612 (server up
  19:10:03, passivation scanned 8 candidates, passivated `24df8086` at
  19:10:38), then silence + 190% burn; health failed 19:10:49/19:11:04/19:11:16.
- 19:15Z held boot (`maintenance_serving`, mutation-fenced): served briefly
  (observability + one runs list), then a runs-list scan exceeded 20s
  (guest: `objectgraph dolt: iterate snapshot: Error 1105: context canceled`),
  then unresponsive + 190% burn with zero load.

## What It Is Not (ruled out)

- Not the 138612 poison: replay passes it now (server-up + passivation +
  held serve all happen strictly after replay).
- Not a single hang: the held guest's autoputer was OOM-killed at ~1.8 GB
  anon-rss (2 GiB VM) six times in ~12 minutes
  (`Out of memory: Killed process ... (autoputer)`), with guest systemd
  restarting it each time — a restart loop. The 190% burn is Go allocation +
  GC thrash ahead of the killer, not a deadlock.
- Not query-driven: the burn continues with zero traffic between restarts.
- Not replay volume: replay is incremental (local head 138611, one event)
  and fast (19:10Z boot through replay in ~15s). Skipping replay would not
  have helped and would leave seq 138612 unfinalized.

## Root Cause

The 11 GiB texture/object-graph store working set (replay-finalize tx,
passivation scans, runs-list snapshot iteration over ~94k objects / ~220 MB
bodies) does not fit the 2 CPU / 2 GiB interactive guest: every boot
allocation-spikes past the 2 GiB line and the guest OOM-killer restarts the
runtime every ~2 minutes. The 15:55Z boot survived on thinner headroom; the
outage window's unclean kills and repeated crash-loop replays pushed the
working set over.

## Repair (in flight)

4 CPU / 8 GiB interactive guests: env-overridable shape in
`internal/vmctl/ownership.go` (`VM_INTERACTIVE_CPU_COUNT` /
`VM_INTERACTIVE_MEM_MIB`, defaults unchanged) + Node B env in
`nix/node-b.nix`. Host has 12 cores / 32 GiB. Residual: 94k-object scans are
still minutes-long on 4 vCPUs and will flap health checks during boot
passivation; store hygiene (GC policy currently skips above 5 GiB while the
DB is ~10 GiB, indexes, fewer full scans) remains follow-up work.

## Safety State

- Computer is under a durable host maintenance hold
  (`reason: passivation-wedge diagnosis 2026-09-03`): vmctl refuses automatic
  lifecycle actions and deploy refresh skips it. Unhold only when healthy.
- No tape writes by the repair path: held boot is mutation-fenced; fence
  `99949fe2` untouched.
- Diagnostic footprint on Node B: `/mnt/guestro` (read-only `noload` mount of
  the 18:22 drive state), `/var/tmp/texture-copy` (9.8 GB offline copy),
  `/tmp/scanb.py`, `/tmp/scanm.py`, `/tmp/ev.json`, `/tmp/batch138612.*`,
  `/tmp/guestq`. Held VM epoch 867 running (do not stop while polling).
