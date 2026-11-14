# Keenable auth errors leave some of the paper's searches with no provider

Recorded 2026-11-14T17:40Z. Not repaired. Read-only observation from the
hourly tick.

## Problem

Since 16:24Z the gateway benches Keenable, the one uncapped search provider,
with `auth_error` from time to time. Every paid provider is quota-limited or
capped all day, so while Keenable is benched a search is answered by nobody.

## Evidence

- Gateway journal on Node B, 16:35Z to 17:35Z: 153 search lines for the paper;
  130 `answered 1 of 1`, 22 `answered 0 of 0`, 1 `answered 0 of 1`.
- The empty ones read `answered 0 of 0 providers (8 cooling, 0 pending)
  skipped=[searxng=success_empty tavily=quota_limited brave=quota_limited
  parallel=quota_limited serper=error serpapi=quota_limited
  keenable=auth_error chatgpt=capped]`.
- In the last six hours `keenable=auth_error` appears in 37 skip lists; the
  first is at 16:24:51Z. Before that Keenable was never skipped for auth.
- `scripts/status-hourly`: 19 of 208 unanswered at 16:35Z, 23 of 147 (15.6%)
  at 17:35Z.

## Belief

Keenable answers most calls and refuses some with an auth failure (a 401 or
403), and the health policy benches it on each. I have not read the provider's
responses, so whether the key is being rotated, rate limited under an auth
status, or failing at Keenable's end is unknown. The start time does not match
a deploy of ours (the 16:20Z landing was tests only and did not deploy).

## Remaining error

- The cause of the auth failures is not known.
- Whether an auth error on a provider that answers the next call deserves the
  same bench as a dead key is a policy question for the search plane.
- The paper keeps publishing; the cost is searches that return nothing.
