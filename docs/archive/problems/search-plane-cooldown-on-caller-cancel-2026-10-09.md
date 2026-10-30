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
`provider_health`, read-only at about 19:30Z):

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

## Amendment (19:36Z): the key reaches user computers and model context

Reading the response path: the gateway returns `provider_health` (with
`last_error_summary`) and per-attempt `error` strings to the calling
computer, in both the success and the `search_outage` responses
(`internal/gateway/search_plane.go`). The guest search client keeps them
(`internal/search/search.go`), and the research tool puts
`provider_health` and attempt errors into the model-facing tool result on
an outage (`internal/researchtools/researchtools.go`). So while SerpAPI's
last failure summary held its request URL, every research search that hit
an outage carried the SerpAPI key into the research desk's context, its
tool results on the computer's tape, and the model provider's request.

The tape is append-only and is not edited; redaction cannot reach what is
already recorded. The remedy for past exposure is rotating the SerpAPI key
(owner action: credential). The fix must also redact on read, because the
stored summary in `search-health.db` keeps the key until that provider's
next failure overwrites it.

Further fix directions:

- Redact URLs' query strings in every error summary the search plane
  stores or returns, at write (summary truncation) and at read (gateway
  response mapping), so stored legacy summaries are also clean.

## Amendment (19:51Z): the ops routes have no caller check

`/provider/v1/search/health`, `/provider/v1/search/health/reset`,
`/provider/v1/breakers` and `/provider/v1/breakers/reset`
(`internal/gateway/handlers.go`) check no caller. The gateway listens on
all interfaces and every computer's tap may reach port 8084 (iptables
`go-choir-vm-*` ACCEPT rules; inference needs it). The public address
times out from outside. So any computer could read every provider's
health, including the stored SerpAPI summary with the key, and reset or
read any search or inference breaker. vmctl and the platform bind
internal authority to the transport (loopback or unix socket); the
gateway already has the same check for credential routes
(`isAuthorizedCredentialCaller`), not applied to these four.

Fix direction: require `isAuthorizedCredentialCaller` on the four ops
routes. Operators reach them from Node B over loopback.

## After the fix (20:10Z)

Deployed 97970cda; gateway restarted 19:44:37Z. Brave and SerpAPI health
reset by hand on Node B (reset writes a clean record, which also removed
the stored SerpAPI summary with its key). In Texture suite run 5 every
search from the test computer succeeded (19:52:53–19:58:45Z). The gateway
journal holds no line with an `api_key` parameter (14-day window).

Residual `search-rate-limit-cooldown-scale`: Brave answered one 429 at
19:55:11Z and is now cooled down for 24 hours. The policy gives every
`rate_limited` outcome the quota base (24 h). A per-second limit (Brave's
plan allows about one request per second; the plane fans out) should cost
seconds, not a day. Telling the two apart needs the provider's rate-limit
headers. Not fixed tonight; Parallel and SerpAPI carry the load.
