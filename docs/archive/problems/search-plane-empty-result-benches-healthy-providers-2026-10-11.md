# Search plane: one empty result benches a healthy provider for everyone (2026-10-11)

Mission: [research fan-out](../definitions/choir-research-fanout-2026-10-10.md),
slice S1. Mutation class of the fix: red (gateway / provider calls).
Problem first; no fix here.

## Evidence

S1 demo on staging 0b7250ef (test computer demo-arxiv-2026-10-10c, document
ee7e8c2b). Research children searched in parallel. Gateway journal on
Node B:

- 00:02:39 to 00:03:18: searches succeed on tavily, brave, parallel and
  serpapi (10 to 40 results each).
- 00:03:26 `site:rein.security October 8 2026 funding $25 million` and
  00:03:29 `site:unit.ai October 7 2026 acquire Boost.ai`: narrow site
  queries.
- 00:03:59: `search outage` for the unit.ai query; from then on every
  search is answered by serpapi alone.

Search-plane health (`/var/lib/go-choir/gateway/search-health.db`, copied
read-only at about 00:10Z):

| Provider | State | Cooldown until | Strikes | Window successes / attempts | Last failure |
|---|---|---|---|---|---|
| parallel | cooling_down | 00:08:26 | 1 | 4617 / 4629 | success_empty (00:03:26) |
| brave | cooling_down | 00:08:26 | 1 | 29 / 33 | success_empty (00:03:26) |
| tavily | cooling_down | 00:08:29 | 1 | 2218 / 2293 | success_empty (00:03:29) |
| searxng | cooling_down | 2026-10-15 | 12 | 16 / 86 | success_empty |
| exa | cooling_down | 15:14 today | 9 | 2 / 62 | error: parse response: unexpected end of JSON input |
| serper | cooling_down | 2026-10-15 | 16 | 0 / 16 | quota_limited: Not enough credits |
| serpapi | active | | 0 | 81 / 81 | |

Parallel, with 4,617 successes in 4,629 calls, went out for five minutes
because one site query found nothing. Research parents then searched
during the cooldown, received serpapi's nine results or an outage, and
reported "live search providers are cooling down" to Texture. The research
run that saw this (0abcac12) gave up on the subtopic.

## Cause

`searchplane.DefaultBackoffPolicy` gives `success_empty` a five-minute
base cooldown with a strike, doubling per consecutive strike
(`policy.go`; design table in `archive/design-search-provider-plane-v1.md`:
"success_empty | yes (shorter base) | +1"). An empty result is usually a
property of the query (a narrow `site:` filter, a date the index does not
have), not of the provider. Charging it to provider health turns one
narrow query into a plane-wide outage for every computer for five minutes,
and repeated empties compound: searxng is out for a week after twelve.

## Clustering

This is the second search-plane defect of one class in two days.
`search-plane-cooldown-on-caller-cancel-2026-10-09.md` found a caller's
`context canceled` striking every provider. Both charge a per-query
outcome to shared provider health. The class: provider health must move
only on evidence about the provider (refusals, rate and quota limits, auth
errors, server errors, malformed responses), never on what the query asked
for or whether the caller stayed.

## Fix direction

An empty result does not cool a provider down. A provider that returns
nothing over and over while others find results is a real signal (a
SearXNG with every engine blocked), so a run of consecutive empties may
still bench it, but one or a few may not. Exa's parse errors and Serper's
credits are separate (operational; the Serper credit is an owner
decision, reported 2026-10-09).
