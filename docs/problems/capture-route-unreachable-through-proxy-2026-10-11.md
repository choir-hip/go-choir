# Guests cannot reach the capture route: the proxy forwards only objects and edges

Found 2026-10-11 during the W1b2 acceptance on staging (cd7221ae).
Evidence class: code reading plus Node B ports and logs.

## What happened

W1b2 added `POST /internal/platform/captures` and
`GET /internal/platform/captures/bodies/<sha256>` to corpusd. A guest's
`RUNTIME_CORPUSD_URL` is the proxy on port 8082, not corpusd (port 8086)
(`internal/vmctl/handlers.go`, guest service env). The proxy forwards only
`/internal/platform/objects` and `/internal/platform/edges` to corpusd
(`internal/proxy/handlers.go`, `RegisterRoutes`). Any other path falls to
the computer surface handler, which answers a POST with 405. A guest's
`choir.PollSource` on cd7221ae would therefore fail on every poll, and no
guest could read a body.

The unit test (`internal/platform/captures_test.go`) and the
researchtools test called corpusd's handler directly, so neither crossed
the proxy hop.

The staging run at 10:57Z did not show this, because the test computer
(demo-arxiv-2026-10-10, guest 10.200.72.2) was still running c1d21d5e,
the superseded build that wrote captures into its own embedded graph. Its
five "new" BBC captures are in that computer's graph, not in Store B;
Node B has no `capture-bodies` directory, and corpusd and the proxy logged
nothing for the poll.

## Cause

The guest path to corpusd is guest → proxy (bound-guest check) → corpusd
(as an internal caller). The proxy holds the guest ownership check for
object writes (`requirePlatformObjectGraphCaller`), and corpusd sees the
proxy's `X-Internal-Caller: true`. A new corpusd route that guests must
reach needs a proxy route and the same ownership check in the proxy. W1b2
added neither.

## Test

Through the proxy handler: a bound guest posting captures for its own
owner and computer reaches corpusd; a guest posting for another computer
is refused at the proxy; a bound guest reads a body; an unbound caller is
refused.
