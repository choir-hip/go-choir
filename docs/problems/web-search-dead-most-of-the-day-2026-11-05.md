# Web search answers nothing about 22 hours a day (2026-11-05)

Status: open. Documented, not fixed. Class: orange (gateway search plane
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
