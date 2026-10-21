# Every guest fetches the public internet itself (SDC NET-01), 2026-10-21

Status: open. Documented before any fix (CLAUDE.md, problem documentation
first). Mutation class of this record: green.

## The problem

Research Read and fetch_url, content import, the Source Viewer snapshot,
podcast subscribe and refresh, podcast search, YouTube transcripts, and every
source poll (RSS, the Telegram HTTP preview, GDELT) dial the public internet
from inside the guest, as guest root, through the open tap egress. The
inventory lists them as rows A1 to A6 and B1 to B6
([guest-network-inventory-2026-10-21.md](../reports/guest-network-inventory-2026-10-21.md)).

So the guest needs a general internet route, and as long as it has one:

1. **The guard runs where the attacker is.** The address guard
   (`internal/sourcefetch/policy.go`: resolve once, refuse non-public
   answers, dial the checked address, re-check each redirect) runs in the
   guest daemon. Anything that gets code execution as guest root (an
   unconfined desk cell before SEC-01 lands, a parser bug in `pdftotext` or
   `pandoc` run on a fetched document, the Management Console's shell, row
   A9) skips it and dials whatever the floor allows.
2. **There is no host record of what a guest read.** The host sees flows at
   the tap, not URLs, statuses or byte counts, and has no per-computer
   budget on fetches.
3. **Closing the guest NIC (NET-04) is impossible** without breaking
   capabilities 1 and 3 of "Networking must enable, not starve" (SDC).

## Smaller defects on the same paths

- Podcast search (`internal/agentcore/podcast.go:91, 355-373`, row B1) uses
  a plain `http.Client`: no address guard, redirects followed unchecked.
- `sourcefetch` has no body limit of its own; GDELT reads with a bare
  `io.ReadAll` (`internal/sources/gdelt.go:92, 395`, row A5).
- `sourcefetch` sets `Proxy=nil`, so a host route cannot be added by
  environment; it needs a code change.

## Evidence

Read from code on main at 53a691be6; the inventory's "Checked by hand"
section re-verified the decision-changing claims. Every guarded caller builds
its client with `sourcefetch.Client(timeout)`: `internal/researchtools/read.go:112`,
`internal/agentcore/tool_profiles.go:373`, `internal/content/content.go:298,
761, 879, 1029`, `internal/sources/rss.go:38`, `telegram.go:23`,
`gdelt.go:24`. That single constructor is the seam a host route can use
without touching each caller.

## Constraints on the fix

- No allowlist: any public URL stays readable (SDC, "No allowlist for
  research reads"). Moving the path moves where the check runs, not what is
  reachable.
- The host side must refuse what the guest side refuses, and also the host's
  own addresses: a host that dials its own public address from itself
  reaches listeners the guest's floor drops.
- Off by default and selectable per computer: a guest keeps fetching itself
  until a computer is granted the host route. Closing general guest egress
  is a later step (NET-04), not part of this fix.
- Polling must not starve: the paper polls about 180 feeds every 15 minutes
  plus article imports, so the host budget cannot be the gateway's shared
  60-a-minute inference limit.
- Telegram MTProto: there is no MTProto code (inventory, Fact 1); the owner
  ruled it out. No MTProto route is built.

## Fix

Station FP10 ([choir-launch-fp10-net01-host-fetch-2026-10-21.md](../definitions/choir-launch-fp10-net01-host-fetch-2026-10-21.md)).
