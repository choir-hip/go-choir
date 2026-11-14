# One Keenable 403 left the paper's searches with no provider for fifteen minutes

Recorded 2026-11-14T17:36Z (first written with 17:40Z, which was ahead of the clock). Not repaired. Read-only observation from the
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

## Correction, 2026-11-14T17:38Z

The first version of this record said Keenable was benched "from time to
time". That was wrong: I counted skip lines, not failures. Read again with the
owner:

- The gateway logged **one** failure: `16:24:15 searchplane: keenable call
  failed class=auth_error http_status=403 stage=http_status
  latency_ms=19122`. A 403 that takes 19 seconds is not what a refused key
  looks like (the same endpoint answers in about 0.2 s).
- The policy benches an auth error for 15 minutes on the first strike
  (`internal/gateway/searchplane/policy.go:72`). All 37 `keenable=auth_error`
  skips fall between 16:24:51Z and 16:39:01Z, and so do all the unanswered
  searches of both hourly readings (19 before 16:35Z, 22 after).
- Since 16:39Z Keenable answers: 19 of 19 searches in the fifteen minutes to
  17:37Z.
- Eight calls from a developer machine with the key in the gitignored `.env`,
  same URL, header and body as `KeenableProvider.Search`: eight 200s in 0.18
  to 0.35 s. Whether that key is the one deployed on Node B was not checked.
- The only other Keenable failures in eight hours are six `parse_response`
  errors in one burst at 10:36Z to 10:37Z.

So the key is good and the event is over. What remains is the policy
question: one slow 403 from the only uncapped provider cost fifteen minutes of
search while every paid provider was quota-limited.

## Repair, 2026-11-14

Owner rule, stated again today: Keenable is to be retried, not benched,
because its rate limit is very hard to hit (100,000 searches a month). The
2026-11-13 repair made an uncapped provider tolerate error, server_error and
timeout, but `transientClass` left out auth_error, so one 403 took the
`OutcomeAuthError` base of 15 minutes.

Decision: for an uncapped provider (`Outcome.Uncapped`), auth_error joins the
transient classes as ONE streak. Five failures of any of those classes in a
row bench it for `TransientBackoff` (30 s) and one probe decides. A metered
provider keeps 15 minutes doubling to 6 hours. `rate_limited` and
`quota_limited` are left as they are: a real 429 is a provider's real
refusal and stays backed off (60 s base, 15 min cap for rate_limited; an
hour for quota_limited), and the code does not bench an uncapped provider
long on a single 429.

Ways it can fail, written before the code:
1. One 403 on an uncapped provider benches it, so the next search is
   answered by nobody (both stores, through `Router.Start`).
2. Five 403s in a row bench it for 15 minutes, not `TransientBackoff`, or
   the bench ends and no single probe goes out.
3. A metered provider's single 403 stops benching it for 15 minutes.
4. A 403 after four transient errors starts a new streak instead of being
   the fifth strike (or the reverse), or a success between does not reset it.
5. `needsProbe` ignores an auth_error streak, so the benched provider is
   called by every search once the bench ends.

### Review findings and stated behavior

- The streak count restarts when an uncapped provider enters or leaves the
  transient-plus-auth class. The first version restarted it only on
  entering, so four 403s then one 429 counted as a fifth strike (15 minutes
  instead of 60 s; quota_limited 6 h instead of 1 h). The same inheritance
  existed before this change for transient errors then a 429 (flagged
  2026-11-13) and is repaired with it. Test:
  `TestRefusalAndUncappedStreakDoNotShareStrikes`.
- A health row benched under the old rule before the deploy keeps its
  `CooldownUntil` until it expires (up to 15 minutes for one strike; the
  stored-cooldown clamp does not shorten an auth_error bench), unless an
  operator resets it. After it expires, a row with fewer than five strikes
  takes the remaining strikes before the 30 s bench.
- A dead key (every call a 403) costs five calls, then one probe per 30 s.
  While it is benched a search escalates to paid providers at once; while a
  call is out, a fast failure escalates on arrival and a slow one at the 8 s
  escalation threshold.
