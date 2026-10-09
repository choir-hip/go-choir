# Search plane: one caller cancel cools down every provider; Brave misconfigured; a key in error summaries (2026-10-09)

Found while tracing Texture acceptance T6 (research) on staging, run 4
(`problems/texture-cancel-and-research-convergence-2026-10-09.md` §2). The
T6 failure is not Texture: research had no search. Mutation class of the
fixes: red (gateway / provider calls). Problem first; no fix here.

## Evidence

Gateway journal on Node B, research computer `vm-6071b4cd…`:

- 19:22:09 – 19:22:26: four searches succeed (tavily 25 results,
  parallel 15–24 results).
- 19:24:26 – 19:25:25: 12 consecutive `search outage` responses.

Search-plane health (`/var/lib/go-choir/gateway/search-health.db`,
`provider_health`, read-only at 19:55Z):

| Provider | State | Cooldown until | Last failure |
|---|---|---|---|
| tavily | cooling_down | 19:26:28 | timeout: request `context canceled` (19:24:28) |
| parallel | cooling_down | 19:26:26 | timeout: request `context canceled` (19:24:26) |
| serpapi | cooling_down | 19:28:26 | timeout: request `context canceled` (19:24:26) |
| exa | cooling_down | 19:38:09 | timeout: `context canceled` |
| brave | cooling_down | **2026-10-15** | 422 "Count should be less than or equal to 20" (22 strikes) |
| serper | cooling_down | **2026-10-15** | 400 "Not enough credits" |
| searxng | cooling_down | 2026-10-15 | success_empty (12 strikes) |

## What this shows

1. **A caller cancellation strikes every provider.** At 19:24:26 the
   three working providers each failed with `context canceled` within two
   seconds and went into cooldown together. `context canceled` is the
   request context ending, not the provider failing. Counting it as a
   provider strike turns one cancelled fan-out into a plane-wide outage,
   and every research desk that searches in the next two minutes gets
   nothing. Texture then correctly waits for evidence (T6).
2. **Brave is cooled down for a week by our own parameter.** The research
   desk asks for `max_results=40`; Brave's limit is 20, so every Brave
   call is a 422 and Brave is out until 2026-10-15.
3. **Serper is out of credits** (operational; owner decision whether to
   refill or drop it).
4. **SearXNG returns empty** (12 strikes).
5. **An API key is stored in error summaries.** SerpAPI's request URL
   carries its key as a query parameter, and `last_error_summary` stores
   the full URL. The key is in plaintext in the gateway's health database
   (root-only on Node B) and possibly in the gateway journal. It must be
   redacted from summaries and logs; the owner may want to rotate it.

## Fix directions

- Do not count `context.Canceled` (caller gone) as a provider strike; count
  only provider-side failures and deadline overruns the provider caused.
- Clamp per-provider `max_results` to each provider's limit (Brave 20).
- Redact query strings (and any `api_key`/`key`/`token` parameter) from
  provider error summaries and logs.
- Serper credits: owner decision.
