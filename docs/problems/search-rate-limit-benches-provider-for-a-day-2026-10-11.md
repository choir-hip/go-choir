# One rate-limit response benches a search provider for a day

Found 2026-10-11 in S1c (PD1 deployed proof, staging, deploy 16d7c21a).
Evidence class: run traces (scratchpad s1c-5) plus code
(internal/gateway/searchplane/policy.go).

## What happened

Every research round in S1c reported "search providers were heavily
rate-limited or in cooldown; only Parallel and Tavily returned results".
The search plane health in the trace shows brave, exa, searxng, serpapi and
serper all `cooling_down`, with `cooldown_until` around
2026-10-12T00:38Z. That is 24 hours after S1b's rate-limit storm. The
recorded errors are:

- brave: `429 Too Many Requests ... "Request rate limit exceeded for plan"
  ... "plan":"Free AI"`. That is a per-second plan limit.
- serper: `429 ... "Your account has run out of searches"`. That is real
  quota exhaustion.

The S1c document reached about 980 words and 9 source links in 27 minutes.
The baseline has 26 in-window items. Two of the seven providers served the
whole run.

## Cause

`ClassifyCall` maps any error containing "429", "rate limit" or "too many
requests" to `OutcomeRateLimited`. `DefaultBackoffPolicy` gives that class
the same base as quota exhaustion, 24 hours, doubling per strike up to
seven days. A per-second limit clears in a second, but the provider stays
benched for a day. Parallel research children make bursts, so one burst
benches every rate-limited provider at once. The searxng cooldown that
reaches October 15 is the doubling at work.

## Why it matters

Search capacity, not the model, bounds research depth. The plane discards
most of its capacity after one burst. Recovery takes a day and is not
automatic in any useful sense.

## Belief and test

I believe a transient rate limit should bench a provider for seconds to
minutes: honor Retry-After when present, otherwise use a short base that
doubles. Quota exhaustion ("run out", "quota", "credits", "monthly") should
keep the long bench even when it arrives as a 429. Test: a focused
classifier and policy test, then a demo where the providers that rate-limit
come back within minutes.

Stored cooldowns on Node B are left to expire (owner rule: no health-store
edits). The fix applies to new strikes. The current benches end
2026-10-12T00:38Z, except searxng.
