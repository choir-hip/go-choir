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
- 19:47:56 search succeeded (provider tavily alone, 10 results)

That is thirty seconds and a few hundred milliseconds.

## The two budgets are the same

- The guest's gateway search client times out at 30 seconds
  (internal/search/search.go, NewGatewayClientFromEnv,
  hostlink.NewClient(30 * time.Second)).
- The gateway gives each search provider 30 seconds
  (internal/gateway/search.go, every provider's http.Client).

- The gateway handler bounds the whole search at 30 seconds as well
  (internal/gateway/handlers.go, context.WithTimeout(r.Context(), 30s)),
  and returns the partial results it has at that deadline. (An earlier
  version of this document said a search could take two minutes through
  the router's waves; that was wrong: the handler's deadline caps it.)

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

## Measurement and design (2026-10-11, later)

docs/archive/search-plane-design-fast-deep-broad-2026-10-11.md measured 509
searches over 24 hours: 51 (10%) ran to exactly 30.0 seconds at the gateway
and were logged as succeeded, usually with only one provider's 10 results;
every guest-side search timeout in two saved demo traces matches one of
them. The likely stalled provider is ChatGPT web search (inference, medium
to high confidence). The proposed first fix orders the deadlines so each
layer gives up before the one above it (provider about 10 seconds, gateway
about 25, guest about 35). Not built: the owner asked for design before
building.

Mutation class of a fix: red (gateway provider calls) on the gateway side,
orange on the guest client.

## Revised 2026-10-12

The deadline-ladder station is deleted. The owner chose asynchronous
search instead: the call returns a stub at once and each provider's
results land in REPL state as they arrive, so nothing waits on the slowest
provider and the 30-second tie stops mattering
([re-engineering decisions](../re-engineering-decisions-2026-10-12.md) §5).
