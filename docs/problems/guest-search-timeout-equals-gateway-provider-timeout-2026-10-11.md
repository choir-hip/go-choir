# A slow search succeeds at the gateway after the guest gave up

Found 2026-10-11 in the P2a deployed demo (test computer demo-p2a-2026-10-11,
staging a2bcf724). Status: documented, not fixed.

## What happened

After the demo's follow-up, research's last round reported: "A new search
for dated AI funding and policy/regulation items for 4–11 October 2026 timed
out at the search gateway, so no source-bearing report was added." Texture
then rejected that report, as it should for a report with no sources, and
took no revision. Research spent its round and the funding and policy
branches stayed open.

The gateway's log for the same search says it succeeded:

- 19:47:26 search request (query "AI funding deal October 2026 startup investment")
- 19:47:56 search succeeded (provider tavily, 40 results)

That is thirty seconds and a few hundred milliseconds.

## The two budgets are the same

- The guest's gateway search client times out at 30 seconds
  (internal/search/search.go, NewGatewayClientFromEnv,
  hostlink.NewClient(30 * time.Second)).
- The gateway gives each search provider 30 seconds
  (internal/gateway/search.go, every provider's http.Client).

- The gateway's router runs up to four waves by default
  (internal/gateway/searchplane/config.go, CHOIR_SEARCH_MAX_WAVES=4), each
  provider call bounded by RequestTimeout (30 seconds), until it has 40
  merged results. A search can legitimately take two minutes at the
  gateway.

So any provider call that takes close to its full budget succeeds at the
gateway after the guest has already returned a timeout to the cell. If the
gateway falls back from one provider to another after a slow failure, the
total exceeds 30 seconds by design, and the guest never sees the fallback's
results. The gateway's work and the provider's quota are spent either way.

## Hypotheses (unconfirmed)

1. The guest's deadline should exceed the gateway's whole fallback budget,
   or the gateway should bound its total time below the guest's deadline
   (a request-scoped deadline passed through, not two fixed timeouts).
2. Some searches today already fail this way without being noticed,
   because the gateway logs them as successes. The next observation: count
   gateway search successes over 29 seconds on staging, against research
   cells that reported a search timeout.

## Why it matters

Research reads a timeout as "the search gateway is down" and says so in its
report. Earlier in the same demo it blamed "search-provider cooldowns and
timeouts" for thin coverage when the gateway had one seven-second outage in
sixteen calls. A desk that cannot tell a slow search from a failed one
narrows its own coverage and explains it wrongly.

## Preferred fix (deferred)

Raising the guest's deadline to cover four waves would make a cell wait two
minutes on one search. The better fix is a total deadline on the gateway's
router, below the guest's, that returns the merged results it has when the
deadline arrives (marked degraded), instead of letting waves run past the
caller. That is a gateway provider-call change (red), off Autopaper's path,
so it waits; this document is its first commit.

Mutation class of a fix: red (gateway provider calls) on the gateway side,
orange on the guest client.
