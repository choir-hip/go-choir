# Web search answers nothing about 22 hours a day (2026-11-05)

Status: open; the answers-none state is stale tonight (see the evening investigation below). Class: orange (gateway search plane
budgets and providers); the money side is an owner decision.

## Evidence

Node B gateway journal, read-only, 2026-11-04T13Z to 2026-11-05T19Z. Each row
is one hour: searches the guests asked for, and how many were answered by
zero providers.

- 2026-11-04 13Z to 23Z: 1434 searches, 1431 answered by none.
- 2026-11-05 00Z and 01Z: 234 searches, all answered (the paid daily budgets
  reset at UTC midnight).
- 2026-11-05 02Z: 173 searches, 75 answered by none (the budgets run out).
- 2026-11-05 03Z to 19Z: 775 searches, 740 answered by none.

Volume is steady at about 120 searches an hour (most from the paper,
vm-1669020adcece51c2f5ff9b6df399ef5), so this is not a spike.

The skip reasons on an empty search (2026-11-05 18Z):
`searxng=success_empty tavily=capped brave=quota_limited parallel=capped
exa=quota_limited serper=quota_limited serpapi=quota_limited`.

- `capped`: the search plane's own daily call cap
  (internal/gateway/searchplane/router.go, Config.DailyCaps), skipped until
  UTC midnight.
- `quota_limited`: the provider's account refused; cooldowns run to about
  22:17Z.
- searxng (Node B, 127.0.0.1:8888) answers but empty: a direct query returns
  0 results with `unresponsive_engines` brave "too many requests",
  duckduckgo "CAPTCHA", google cse "too many requests", wikipedia "too many
  requests". The free tier is blocked upstream, so it is not a fallback.

## Effect

Research on every computer, the paper included, runs without web search for
roughly 22 hours of each day. Research runs still complete (they spend model
calls, record that search found nothing, and write from what they have), so
nothing fails loudly. The paper's stories outside the first two UTC hours are
written without fresh sources.

## Open questions (for the owner, not decided here)

- Budget: roughly 2900 searches a day against paid caps that last about two
  hours. Raise the caps or the provider plans, or cut searches per story.
- searxng: its upstream engines are rate-limited from Node B's address. Fewer
  engines, a different egress, or retire it as a provider.
- Search reuse: the earlier archived problems (search-is-duplicated-and-
  discarded, search-providers-cooling-most-of-the-time, 2026-10-14) named
  duplicate queries; whether the paper still repeats queries is unmeasured.

## Keenable benched for good by one auth error (found 2026-11-05 19Z)

Keenable is the provider with the most access (owner: about 100,000
searches a month) and it has been skipped as `keenable=auth_error` on every
search since at least 2026-11-03T18:54Z (the journal's start). Cause: an
auth error set the provider's state to `disabled`, and
`searchplane.IsEligible` refused a disabled provider regardless of its
cooldown deadline, so no call ever probed it again and only an operator
reset could clear it. Whether the original error was a passing fault or a
bad key is unknown; a probe will tell.

Fix (owner, 2026-11-05: "A single error shouldn't permanently block. ... we
should be doing exponential backoff"): an auth error is a cooldown on the
doubling schedule, 15 minutes first, capped at six hours; a stored
disabled record clamps to six hours from its last update at gateway start
and expires like a cooldown. If the key itself is bad, the probes keep
failing and the owner replaces the key.

## Investigation 2026-11-09 evening

Read-only: gateway journal on Node B, 2026-11-09 19Z to 2026-11-10 01:33Z, plus
main at b45451433. [M] measured, [I] inferred.

(a) Truth tonight. The headline (search answers nothing 22 hours a day) is
stale for the paper. Of 772 search lines in 3 hours, 765 are `served=miss`, and
every one shown answers `1 of 1 providers` with `results=40`. Keenable
answers; nothing else does. Skip sets over 3 hours [M]: 616 x
`searxng=success_empty brave=quota_limited parallel=quota_limited`; 86 x the
same plus tavily, exa and serpapi `quota_limited`; 61 with serpapi.
`paid_calls=0`, `today=[keenable=308]` at 01:32Z. The Keenable bench fix
(d613108ed, 279cba57d) works. The failure has moved: one provider carries the
whole load.

(b) Volume [M]. vm-1669020a searches per hour (UTC): 19Z 121, 20Z 308, 21Z
350, 22Z 298, 23Z 315, 00Z 120, 01Z 195 (partial). That is about 300 an hour,
2.5 times the 120 an hour of 11-05. At 300 an hour a day is about 7,200
searches against an owner-stated Keenable allowance of about 100,000 a month
(about 3,300 a day): the month would last about 14 days [I: the real
accounting of the allowance is unread]. Keenable has no daily cap and no pace
(279cba57d deleted them), so nothing slows the paper before the account
refuses; the other providers are already in quota cooldown, so search then
dies as it did on 11-05. The result cache does not help: 7 hits in 772.

(c) Why each other provider is skipped [M, "searchplane: X call failed" lines]:
- parallel: `http_status=402` -> quota_limited. 402 is payment required, so
  the account is out of credit. Real.
- serpapi: 429 -> quota_limited; plausibly real.
- brave: 429 classified quota_limited, then rate_limited, then quota_limited
  within 3 seconds (20:14:09-11). `ClassifyCall`
  (internal/gateway/searchplane/policy.go:165-181) separates quota from rate
  by phrases in the text ("monthly", "exceeded your quota" ...), so a
  per-second 429 whose body mentions the plan can read as quota. Hypothesis;
  confirm by logging brave's 429 body once. Cost is small (cooldown 1 hour
  doubling to 6, policy.go:43,59).
- serper: `http_status=400` -> `class=error` (5 minute cooldown). Serper is
  believed to answer 400 for an account with no credits [I]; probable
  misclassification, small effect.
- chatgpt: 429 rate_limited, a few an hour, capped at 50 a day by design.
- searxng: success_empty (upstream engines blocked, unchanged since 11-05).
- tavily, exa: quota_limited in the 86-line set.
The plane's own daily caps (config.go:113) are not what blocks the paid
providers now; their accounts are.

(d) Smallest correct fix, orange. (1) Cut waste before adding supply: find
which research step issues about 300 searches an hour with no cache hits
(needs a run trace, not read here); unsized. (2) A Keenable daily budget
(allowance over days left) so exhaustion degrades to fewer searches instead
of none; about 40 lines, 2-3 hours; reverses part of 279cba57d, the owner's
call. (3) Classify serper 400 and brave's monthly 429 as quota; about 10
lines, 1 hour.

(e) Fixed on main: the Keenable bench (d613108ed, 279cba57d). The "answers
none" status is stale tonight; the budget question remains and is now a
Keenable-allowance question.
