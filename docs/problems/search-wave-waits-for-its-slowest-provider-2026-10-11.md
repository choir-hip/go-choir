# A search wave waits for its slowest provider

Found 2026-10-11 in S1f (staging c1d21d5e, document b5841e05). Evidence
class: research cell results, gateway journal, and provider attempt
latencies from the S1e and S1f traces (scratchpad demo13/final,
demo14/final).

## What happened

The S1f brief says: "repeated search-provider timeouts cut that track
short". Two research web searches failed in the guest with `gateway
search: ... context deadline exceeded`. The gateway logged every search
as successful, but each one took exactly 30 seconds (10:20:46 to 10:21:16,
10:21:17 to 10:21:47, 10:21:51 to 10:22:21). Searches before 10:18 took 1
to 4 seconds.

Provider attempt latencies across S1e and S1f:

| provider | calls | median | max |
|---|---|---|---|
| chatgpt | 12 | 22.9 s | 23.6 s |
| tavily | 50 | 1.9 s | 4.1 s |
| parallel | 62 | 1.5 s | 2.8 s |
| brave | 12 | 0.5 s | 0.8 s |
| keenable | 40 | 0.2 s | 0.5 s |

## Cause

`Router.executeWave` (internal/gateway/searchplane/router.go) runs a
wave's providers in an errgroup and waits for all of them. A wave that
includes ChatGPT search (added in 78c9173e) returns no sooner than about
23 seconds, even when Keenable answered in 0.2 seconds with enough
results. A second wave, or a slow provider at its 30-second request
timeout, pushes the call past the guest's own deadline.

## Test

A wave whose fast providers already meet the merged-result target returns
within a short grace of them, and the slow providers are cancelled
without being counted as failures. A slow provider still contributes when
the fast ones fall short.
