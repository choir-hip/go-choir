# One Keenable error cools the only uncapped search provider

Recorded 2026-11-13T23:40Z. Status: fixed on lane/search-cooldown (not yet landed or
verified on staging). Code read; see Read of the code and Fix.

## Problem

The paper's searches return nothing for minutes at a time. Keenable is the
abundant primary (owner, 2026-11-13: "Keenable is abundant. That should be
working regardless"); every paid provider is quota-limited by design. When
Keenable is marked `error` and cools, no provider is left and each search is
a miss.

## Evidence

Gateway journal on Node B, read at 23:38Z:

- 22:59:28Z `searchplane: keenable call failed class=error http_status=404
  stage=http_status latency_ms=6342`. One line in 90 minutes.
- 23:02Z to 23:03Z: 30 paper searches logged `answered 0 of 0 providers
  (8 cooling, 0 pending)` with `keenable=error` in the skipped list.
- Hourly counts of paper searches answered by no provider: 0 of 236
  (20:35Z), 0 of 330 (21:35Z), 18 of 349 (22:35Z), 32 of 238 (23:35Z).
- Keenable had answered 6,970 searches that day.

## Belief

A single failed Keenable call puts it in the same cooldown as any other
provider, and the cooldown lasts minutes. For a provider with no cap and no
substitute, that turns one upstream 404 into a few dozen empty searches.

## Remaining error

- The cooldown length and whether it grows with repeated errors are not read
  from the code.
- Why Keenable returned a 404 after 6.3 s is unknown (a bad query path, or
  an upstream fault).
- The 22:35Z hour's 18 misses were not traced to a Keenable error line.
- Whether the paper's researcher retries a missed search later is unknown.

## Fix direction

Not a limit to tune blindly. Read the cooldown rule first; the likely change
is that a provider with nothing behind it is retried on the next request
(or after a short, fixed backoff following a real refusal) rather than
skipped for minutes.

## Read of the code

One mechanism, `applyOutcome` (`internal/gateway/searchplane/health.go`),
benched a provider on any non-success outcome, with a duration from
`BackoffPolicy.CooldownDuration` (`policy.go`): rate_limited 60 s (cap 15 min),
quota_limited 1 h (cap 6 h), auth_error 15 min (cap 6 h), server_error and
timeout 2 min (cap 30 min), error 5 min doubling per strike (cap 1 h),
success_empty only after 10 empties in a row. A cooling provider is skipped
at the start of every search and at escalation (`router.go` StartRequest,
`escalate`), and listed as `name=class` in the `skipped=[...]` of the log
line. The one cooldown did four jobs: (1) quota exhaustion on metered
providers, (2) a provider's own rate refusal, (3) an auth fault, (4) a
transient fault, where the provider is not out of anything. Job 4 is right
only for a metered provider, where "an error" generally means the quota is
gone (owner, 2026-11-13); for a free-tier provider it benched the only
provider with nothing behind it.

## Fix

The router marks an outcome `Uncapped` when the provider's tier is free
(`TierOf`, the existing tier table; no name check). For such a provider a
transient class (error, server_error, timeout) is no cooldown until
`TransientStrikesBeforeCooldown` (5) in a row, then a fixed
`TransientBackoff` (30 s) that does not grow; a success clears the run. Real
refusals (rate_limited, quota_limited, auth_error) back off as before, and
every metered or constrained provider is unchanged. The log lines are
unchanged. No retry of the failing request: the search that hit the error
still escalates to a paid provider if short (`EscalateFailed`), and the next
search calls Keenable again.

Remaining error: why Keenable answered 404 after 6.3 s; whether 5 and 30 s
are right for a real Keenable outage (no outage has been observed; the
numbers come from the 8 calls a second pace and the 2 minute server_error
base).
