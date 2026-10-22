# FP0's host-side probes sent the host-internal service token to guests, the paper included

Date: 2026-10-22. Status: **documented and audited; FP0's sending stopped at about
09:15 UTC (the monitor's at 09:18); no guest-usable route found; rotation at a
deploy recommended, the owner's call; the repo-script fix is on lane
`fp0-guest-token-probes` (98dd1aeac, df682772c; review round 2 85d10406b,
5c7b30687), reviewed in bunch 44**. Found by Devin's review of the RES-10
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
  Runs logged for deploys C to I, from about 03:17 to about 09:15 UTC on
  2026-10-22, with gaps between deploys. The guest was the paper,
  `computer-1355b83b…` (vm-1669020a), at each of its addresses that day.
- **The paper desk-worker watch** (scratchpad `paper-reaper-watch.sh`):
  every 3 min from 07:13 to 08:28, `GET …:8085/internal/debug/pprof/procmem`
  on the paper with the token.
- **Ad-hoc reads** of `/internal/debug/pprof/procmem` and `/pagecache` with
  the token. On the paper they were the RES-07 readings. The owner's
  computer (vm-4a9afa23, read-only diagnosis, owner-authorized) got them at
  10.201.4.2, and so did test computers.
- **The self-development monitor** (FP0's agent): from about 04:05 to
  09:12 UTC, `check-nodeb.sh` (about every 10 min) sent the header with guest
  GETs to `:8085` on the owner's computer (`ad1b46d4`), the paper
  (`1355b83b`), and computers `03335285` and `7cc96dc6`. It also reached
  test computer `71afbf14` on most passes from 04:40, with more calls from
  `fc.sh`, `get.sh` and `ops.sh` from about 06:37 to 09:00. Computer
  `2bd23366` was reached from 05:14 to 06:35, and `9648031f` once at 04:51.
  The monitor removed the header from every guest call at 09:18. Its guest
  reads still return 200 without it.
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

- About 09:15 UTC: FP0 stopped the boot-window sampler, and killed its loop
  (`bash -s 5400`) on Node B. The monitor dropped the header from its guest
  calls at 09:18. No FP0 process on Node B now sends it to a guest.
- From now on, FP0's guest reads send no token: only the source address
  gates them.

## Audit (FP0's read-only agent, 2026-10-22 09:30 UTC)

**No host route accepts the token from a guest.** Every host unit runs hostauth
in enforce (`nix/node-b.nix:75`, `:80`). Every place that checks the token
sits behind a loopback or unix-socket source check, and a guest's connection
never passes it:

| Service (port) | Token check | Source check first |
|---|---|---|
| vmctl 8083 | `isInternalCaller` (handlers.go:1207) | `internalTransport`, loopback or unix only (:1214-1232) |
| gateway 8084 | handlers.go:806, host_judgment.go:35 | `isLocalhost` (:790) plus the marker |
| corpusd/platform 8086 | authority.go:67, key_escrow_http.go:308 | `trustedInternalTransport` (authority.go:22-37) |
| maild 8087 | api.go:135, :247; auth_mail.go:61; digest.go:271 | `isMaildLoopbackHost` (api.go:164); guests get the vmctl ownership binding instead |
| pprof on every host server | debug_pprof.go:38 | `HostSourcedCaller` (host_caller.go:31-61) |

- Guests reach host ports 8082 to 8089 by DNAT from their gateway address to
  127.0.0.1 (manager.go:3590-3606, hostfirewall.go:162-165). DNAT rewrites
  only the destination, and MASQUERADE runs in POSTROUTING, which locally
  delivered packets never pass. So every host service sees the guest's own
  address as RemoteAddr, and the anti-spoof rule drops any other source
  (hostfirewall.go:114-116).
- The one way a guest's request can reach a service as loopback is through
  the public site, because Caddy forwards to 127.0.0.1. Every token-gated
  route is under `/internal/*` or `/provider/*`, and Caddy refuses both
  (node-b.nix:603-608). That Caddy block is now the only barrier for token
  checks on requests that arrive through the public site.
- **Guest side:** guests hold no token by design (nix/autoputer-vm.nix never
  mentions it, so hostauth runs off). The guest's `hostauth.Middleware`
  deletes the header before the mux, in every mode (hostauth.go:258-260), and
  no access log wraps it. So the token never reached a guest's handlers,
  traces, tape or pprof. It crossed the tap as plaintext HTTP, where guest
  root or a packet capture could read it.
- **More senders in the repo:** `scripts/res08_pagecache_probe.py:111-112`,
  `scripts/density/host_collect.py:175` (through `auth_headers()`, :77-85),
  `scripts/paper_boot_window_probe.py:117` (with `--token-file`), and the curl
  line in `docs/reports/res-08-shared-base-os-pmem-dax-design-2026-10-22.md:195`.

**Why rotate anyway.** The token exists to stop host-namespace processes,
such as the Firecracker VMMs, from using internal routes (hostauth.go:6-9).
Suppose a guest escaped into its VMM while holding the captured value. It
would then pass the loopback checks, and so get vmctl lifecycle acts, gateway
credential minting, maild sending and corpusd internal writes. Rotating is
cheap: delete both secret files and redeploy (node-b.nix:2030-2032). Rotating
before the probes are fixed would only leak the new value.

## Next

1. Owner decision: rotate the internal service token in the deploy that lands
   the probe fix, by deleting both files on Node B so the deploy regenerates
   them. Not urgent, since the token alone gives a guest nothing. It changes a
   credential that every host unit loads, so it is the owner's call. FP0 will
   not do it unasked.
2. **Done on lane `fp0-guest-token-probes`** (to land): `98dd1aeac` adds
   `scripts/selftest-guest-token`, failing first on the four senders;
   `df682772c` fixes them. Every repo script now sends only the marker
   (`X-Internal-Caller`) to a guest. The helpers follow `MarkInternal` but
   are narrower: MarkInternal accepts all of 127.0.0.0/8 and any case of
   `localhost`, and the helpers accept three literals.
   Python `scripts/choir_internal.py` (`internal_headers(url)`,
   `internal_curl_flags(url)`) and shell `scripts/lib/choir_internal.sh`
   (`choir_internal_curl`). They attach the token only for 127.0.0.1, ::1
   or localhost on 8083, 8084, 8086 or 8087. A request that carries the
   token goes with no proxy from the environment and follows no redirect
   (`internal_urlopen`, `--noproxy '*'`; hardening, not a known leak). The
   fixes:
   - `res08_pagecache_probe.py` `guest_read`: marker only, via the helper.
   - `density/host_collect.py` `http_json`: `internal_headers(url)`. The
     file carries a verbatim copy of the helper, because it is piped alone
     to the host, and the selftest proves the copy.
   - `paper_boot_window_probe.py`: `--token-file` removed; procmem gets
     the marker only.
   - The res-08 design report's manual guest curl drops the header.
   - Loopback users moved to the helpers: `node_b_host_reboot.py` and
     `proofs/lib.sh` (`vmctl_get`, `vmctl_post`).
   - Checked and left alone, loopback only: `density/lib.py`,
     `memory_hotplug_probe.py`, `s3_snapshot_probe.py`,
     `node-b-deploy-disk-preflight`, ci.yml's deploy, the frontend helper
     `node-b-internal-auth.js` and its users, and the commands in
     `docs/runbooks/node-b-host-reboot.md` and the 2026-10-14 corpus record.

   What the selftest scans: the tracked non-Go files that name the header
   file (`internal-service-token.header`) or the header
   (`X-Choir-Internal-Token`), plus the files that use a long upper-case
   name holding it (such as `NODE_B_INTERNAL_AUTH`). From those it follows
   the names that hold the token. A request that carries it may name only
   those destinations, and a destination it cannot resolve fails. It
   misses a sender that never names either literal: a curl `-K` config
   file, a path alias or copy of the header file, a token read from the
   environment, or a holder passed across files under a short name. It
   fails, never passes empty, outside a git work tree, at a bad `--rev`, or
   when no file names the literal (review bunch 44). It is not wired into
   CI yet (no scripts selftest is): run `scripts/selftest-guest-token`.
3. Optional detector (red, hostauth): a guest's `hostauth.Middleware` logs one
   line when a token header reaches a process with no token loaded, so a
   future leak shows in the journal.
4. FP0's own scripts (`bootwin/poll.sh`, the reaper watches) drop the header.

## Earlier plan (kept for the record)

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

## 2026-10-22: the scanner runs only by hand

`scripts/selftest-guest-token` (fix item 2) is the only guard that a tracked
script keeps the token on loopback internal ports, and no CI job runs it. A
script change that sends the header to a guest URL lands green unless
someone remembers to run it. Fix (lane `fp0-ci-guards`, yellow): run
`python3 -I scripts/selftest-guest-token` in the CI job that already runs
the script gates (Vocabulary Gates, `v1-inventory`), which runs on every
non-docs change and is skipped on docs-only pushes, like the other gates.
