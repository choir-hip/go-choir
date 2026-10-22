# FP0's host-side probes sent the host-internal service token to guests, the paper included

Date: 2026-10-22. Status: **documented; the sending stopped at 09:22 UTC; whether
the token must be rotated is under audit**. Found by Devin's review of the RES-10
probe (bunch 40), which pointed at the same pattern in FP0's own scripts.
Mutation class of this record: green. Evidence class: host commands FP0 ran on
Node B and their scripts; no guest-side reading.

## What happened

`internal/hostauth/hostauth.go` (`MarkInternal`, about line 300) promises that
the host-internal credential "is never sent to a guest (directly, or through
a 127.0.0.1 DNAT port) or the internet". FP0's host-side reads broke that
promise. They ran on Node B as root and added
`-H @/var/lib/go-choir/secrets/internal-service-token.header` to requests for
guest addresses on port 8085:

- **The boot-window sampler** (`lanes/_runs/bootwin/poll.sh`, outside the
  repo): every 20 s, `GET http://<paper>:8085/api/runtime/observability` with
  the token, `X-Internal-Caller: vmctl` and the paper's identity headers.
  Runs logged for deploys C to I, from about 03:17 to 09:20 UTC on
  2026-10-22, with gaps between deploys. The guest was the paper,
  `computer-1355b83b…` (vm-1669020a), at each of its addresses that day.
- **The paper desk-worker watch** (scratchpad `paper-reaper-watch.sh`):
  every 3 min from 07:13 to 08:28, `GET …:8085/internal/debug/pprof/procmem`
  on the paper with the token.
- **Ad-hoc reads** of `/internal/debug/pprof/procmem` and `/pagecache` with
  the token. On the paper they were the RES-07 readings. The owner's
  computer (vm-4a9afa23, read-only diagnosis, owner-authorized) got them at
  10.201.4.2, and so did test computers.
- **In the repo**, `scripts/res08_pagecache_probe.py` (`guest_read`) sends
  the same header to guests. The RES-10 probe did too until its round 2
  (lane `fp10-res10-reader`, `3e2c19443`). Other scripts read the same
  header file (`memory_hotplug_probe.py`, `s3_snapshot_probe.py`,
  `node_b_host_reboot.py`, `density/host_collect.py`, `density/lib.py`,
  `proofs/lib.sh`). Each needs checking for guest destinations.

The guest gate for these routes does not need the token. It accepts
host-sourced callers by source address (`HostSourcedCaller`), and guests run
hostauth in mode off (Devin's review: `internal/server/debug_pprof.go:38`,
`internal/server/host_caller.go:60`, `internal/hostauth/hostauth.go:199`).
So the token added nothing to these reads. It only crossed into the guests'
network namespaces.

## Why it matters

The paper's agents process untrusted web content. A guest that captured the
header holds the value every host service uses for host-internal calls.
`hostauth.Authorize` checks only the token. Each service supplies its own
transport check: vmctl's internal routes also require loopback or its unix
socket and reject tap sources. Some host ports are DNAT'd from a guest's
gateway address to 127.0.0.1. Whether any route a guest can reach accepts the
token without a source check is the open question, and it decides whether the
token must be rotated.

## Immediate actions taken

- 09:22 UTC: FP0 stopped the boot-window sampler, and killed its loop
  (`bash -s 5400`) on Node B. No FP0 process on Node B now sends the header.
- From now on, FP0's guest reads send no token: only the source address
  gates them.

## Next

1. Audit, read-only and under way: every host service that accepts the token,
   its source check, and whether a guest can reach it with the token. The
   answer decides between rotating now, rotating at the next deploy, or not
   rotating. A rotation touches every host service's credential and is the
   owner's call (CLAUDE.md: a credential question is a blocking ask).
2. A helper for host probes that refuses to attach the header to a
   non-loopback URL, mirroring `MarkInternal`, and a fix to every script
   above that sends it to a guest.
3. FP0's own scripts (`bootwin/poll.sh`, the reaper watches) drop the
   header.
