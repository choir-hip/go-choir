# Search providers are "cooling" most of the time (2026-10-14)

Found 2026-10-14 while tracing a thin Autopaper front-page story (the
Mauritius riots stub: research ran three web searches, each answered by one
or two providers, read two pages, cited one source). Mutation class of any
fix: **red** (gateway / provider calls). Problem first; the fix commits come
after this one and reference it.

## Evidence

All read-only on Node B, about 14:00 to 14:10Z. No secret was printed:
the provider env file was read for variable names only, and error
summaries come from the gateway's redacted health route.

### Configured providers

`/var/lib/go-choir/gateway-provider.env` (names only) sets
`TAVILY_API_KEY`, `BRAVE_API_KEY`, `EXA_API_KEY`, `SERPER_API_KEY`,
`PARALLEL_API_KEY`, `SERPAPI_API_KEY`, `KEENABLE_API_KEY` and
`CHATGPT_AUTH_PATH`; the unit sets `SEARXNG_URL=http://127.0.0.1:8888`.
`NewSearchClient` registers only providers whose `IsAvailable()` is true,
so all nine are configured and the "N cooling" count in the log is **not**
inflated by unconfigured providers. The pool really is nine; seven of them
are out.

### Health store

`/var/lib/go-choir/gateway/search-health.db` (SQLite, read from a copy) and
`GET /provider/v1/search/health` (loopback):

| Provider | State | Until | Strikes | Attempts / successes | Last strike at | Last failure (redacted) |
|---|---|---|---|---|---|---|
| keenable | active | | 0 | 7502 / 7417 | | |
| brave | flapping: active ⇄ cooling (4 min at 14:04) | 14:07:53 | 0 ⇄ 3 | 3367 / 1542 | 14:03:53 | 429 "Request rate limit exceeded for plan", plan "Free AI", rate_limit 1 |
| chatgpt | cooling_down | 10-15 11:30 | 12 | 1491 / 1438 | 10-14 01:22:07 | 429 Too Many Requests |
| tavily | cooling_down | 10-18 01:26 | 3 | 2914 / 2818 | 10-14 01:26:02 | 432 "This request exceeds your plan's set usage limit" |
| parallel | cooling_down | 10-17 12:36 | 3 | 5591 / 5553 | 10-13 12:36:11 | 402 "Insufficient credit in account" |
| serpapi | cooling_down | 10-14 21:53 | 2 | 115 / 109 | 10-12 21:53:29 | 429 "Your account has run out of searches" |
| serper | cooling_down | 10-15 20:12 | 16 | 16 / 0 | 10-08 20:12:35 | 400 "Not enough credits" |
| exa | cooling_down | 10-20 10:32 | 14 | 67 / 2 | 10-13 10:32:36 | error: "parse response: unexpected end of JSON input" |
| searxng | cooling_down | 10-15 20:12 | 12 | 86 / 16 | 10-08 20:12:31 | success_empty |

SearXNG probed directly on Node B (`/search?q=Mauritius+riots&format=json`):
0 results; `unresponsive_engines`: brave "too many requests", duckduckgo
"CAPTCHA", startpage "unexpected crash". Its upstream engines block the
datacenter address.

### Gateway journal

Per-search summary lines, by day (`answered M of N providers (C cooling)`;
the line exists since the streaming search deploy on 10-13):

| Day | N=0 (outage) | N=1 | N=2 | N=3 | N=4 |
|---|---|---|---|---|---|
| 10-13 | 75 | 623 | 1140 | 630 | 11 |
| 10-14 (to 14:08Z) | 176 | 2287 | 1560 | 28 | 0 |

Search requests per day (all computers): 10-09 101, 10-10 56, 10-11 490,
10-12 401, **10-13 3183, 10-14 4140** (to 14:08Z). One computer,
`vm-1669020a…` (the Autopaper computer), made 1524 on 10-13 and 3573 on
10-14, peaking at **971 in the hour 07:00–08:00Z**. Searches arrive in
bursts of 2–4 in the same second.

The gateway restarted at 13:41Z, 13:49Z and 14:02Z (deploys). The cooling
set is identical before and after each restart: health is durable in
SQLite by design (`searchplane.OpenDefaultHealthStore`), so a restart does
not clear it, and nothing is re-tripped by the restart itself.

## What this shows, per provider

| Provider | Why it is out | Whose doing | Self-heals? |
|---|---|---|---|
| tavily | plan usage limit (432) | account: quota spent | no, until plan resets or upgrade |
| parallel | 402 insufficient credit | account: prepaid balance spent | no, until topped up |
| serpapi | run out of searches | account: monthly searches spent (since 10-12) | at month reset |
| serper | not enough credits | account: one-time credits spent (since 10-08) | no |
| chatgpt | 429 | shared ChatGPT/Codex usage limit (see chatgpt-usage-limit-stops-research-and-management-2026-10-11.md); benched 34 h by 12 strikes | the limit resets in hours; the bench does not |
| brave | 429 per-second limit (plan "Free AI", 1 request/s) | **ours**: 2–4 searches in one second all call Brave | yes, but 54% of its recorded calls failed and each burst benches it 1–4 min |
| exa | parse error | **ours** (hypothesis, strong): we ask for `numResults=40` with `contents.text=true` (full page text) and read at most 256 KiB (`ExaProvider.Search`, `io.LimitReader(…, 256*1024)`); 40 full pages exceed that, the JSON is cut, `json.Unmarshal` says "unexpected end of JSON input" | no: every probe fails the same way; benched 7 days |
| searxng | always empty | Node B's SearXNG engines are CAPTCHA'd / rate-limited upstream | no |
| keenable | — (healthy) | | |

So the real pool today is **Keenable, plus Brave about half the time**.
That is exactly the "1 of 1 / 2 of 2 (7–8 cooling)" in the journal.

Answers to the four questions asked:

1. **Configured / cooling / why**: table above. Cooling is durable across
   restarts (SQLite) — not re-tripped. Trip rule: any failure adds one
   strike; cooldown = class base × 2^(strikes−1), capped at 7 days; base
   60 s rate limit, 24 h quota, 7 d auth (and `disabled`), 2 min
   server/timeout, 5 min other, empty only after 10 in a row. Strikes reset
   only on a success. Health is **one global record per provider, shared
   by every computer**: one computer's burst benches the provider for all.
2. **Unconfigured counted as cooling?** No. All nine are configured; the
   log is honest about the count but not about the cause (it says
   "cooling" for both "spent the month's quota" and "429 a second ago").
3. **Does our own fan-out cause the trips?** Yes, in three ways:
   (a) every search calls *every* eligible provider (streaming search,
   a4a419760, deployed 10-13), so paid quota burns once per provider per
   search; at 3–4k searches a day this exhausted Parallel (10-13 12:36)
   and Tavily (10-14 01:26) within a day of the volume jump; (b) bursts of
   2–4 simultaneous searches exceed Brave's 1 request/s; (c) the Exa
   failure is our own request shape (40 results × full text) against our
   own read cap. Timeouts and `max_results=40` are not tripping anything
   today (no timeout strikes in the store).
4. **Cost / quota**: see "Owner decisions" below.

## The benching mechanics amplify every one of these

Read from the store, the strike arithmetic shows two mechanism defects:

- **One burst, many strikes.** Parallel's record: 3 strikes, last strike
  10-13 12:36:11, cooldown exactly 96 h (= 24 h × 2²) later. A provider
  that is benched is not called, so strikes 1 and 2 cannot have come from
  earlier searches (Parallel was answering that morning); all three came
  from calls already in flight when the first 402 landed. Tavily's record
  is the same shape (3 strikes, 96 h, one instant). Brave gets 2–3 strikes
  per burst (3 strikes at 14:03:53, 4 minutes). ChatGPT has 12 strikes:
  60 s × 2¹¹ = 34 h for a usage limit that resets in hours. Every call in
  flight is charged separately for one fact about the provider, so the
  exponent grows with our concurrency, not with the provider's history.
- **Unbounded doubling for transient classes; days-long probing for
  quota.** A rate limit doubles up to the 7-day cap; a parse error
  doubles to 7 days (Exa); quota doubles from 24 h to 4 days, so an owner
  top-up is not noticed for up to a week. Each expiry admits every
  concurrent search at once, which re-strikes several times and doubles
  again.

## Clustering assessment (CLAUDE.md: Root Cause Clustering)

This is the fourth record about provider health in five days:

1. search-plane-cooldown-on-caller-cancel-2026-10-09 (fixed 97970cda:
   caller cancel never strikes) — landed.
2. search-plane-empty-result-benches-healthy-providers-2026-10-11 (fixed
   4d365e47: ten empties in a row) — landed.
3. search-rate-limit-benches-provider-for-a-day-2026-10-11 (fixed
   b7668dc3: rate limit base 60 s, quota phrases keep 24 h) — landed.
4. this one.

All three earlier fixes landed and work as written; each changed *which
outcomes* count. None changed *how a count becomes a bench*. The shared
substrate cause is the benching model itself: **provider health is a
per-call strike counter with unbounded exponential backoff**, and it is
the only lever the plane has. It cannot distinguish:

- a fact about the provider (out of credit, key revoked) — needs an
  infrequent single probe, and a loud "exhausted" label;
- a fact about our traffic (per-second limit) — needs pacing, not a
  bench;
- a transient fault (5xx, timeout, malformed body) — needs short, bounded
  backoff;
- the same fact reported by N concurrent calls — is one observation.

And a fifth cause sits above the health store: **nothing in the plane
knows about cost**. Since 10-13 every search calls every eligible paid
provider; health benching was the only thing slowing quota burn, and the
paid accounts drained anyway. The earlier design
(docs/archive/search-plane-design-fast-deep-broad-2026-10-11.md §4 clustering)
named "no single time authority" and "no honest attribution"; this adds
"no episode boundary in strikes" and "no budget".

## Design (substrate-level, short)

Mechanism changes, contained to `internal/gateway/searchplane` and the
gateway log line:

1. **One episode, one strike.** An outcome from a call that started before
   the bench now covering it was set (the call's start time is before
   `cooldown_until` and the provider is already benched) adds no strike
   and does not extend the bench. Concurrency no longer reaches the
   exponent. No schema change: the test uses the stored state and the
   call's start time.
2. **Bounded backoff per class.** Doubling stays, with per-class caps:
   rate limit ≤ 15 min, server error / timeout ≤ 30 min, other errors
   ≤ 1 h, a run of empties ≤ 6 h, quota base 1 h ≤ 6 h (a top-up is seen
   within hours; each probe is one call). Auth stays at 7 days/disabled
   (needs an owner act; the reset route exists).
3. **Pacing instead of benching for our own bursts.** Per-provider minimum
   interval between calls (Brave: 1 request/s by default; env
   `CHOIR_SEARCH_PROVIDER_RPS=brave=1,...`). A call waits for its slot up to
   a short bound (3 s); beyond that the provider is skipped for this search
   as `paced` — not a failure, no strike.
4. **Honest skip reasons.** Each skipped provider carries the reason
   (`quota_limited`, `rate_limited`, `paced`, `success_empty`, …) on the
   NDJSON start line (`cooling[].reason`, additive) and in the gateway log
   line, so "8 cooling" reads "tavily=quota_limited parallel=quota_limited
   … brave=paced".

Adapter defect (separate commit so it can be dropped): Exa asks for at most
10 results with page text capped at 1,000 characters, and the read cap is
raised to 2 MiB. **This turns Exa back on and it starts spending** (see
owner decisions).

Not in this change (owner decision first): a **budget-aware router**
(free providers first, metered providers only when the free answers fall
short, per-provider daily call caps). Design sketch: providers carry a
cost tier (free: keenable, searxng; metered: brave, tavily, parallel, exa,
serper, serpapi; shared-subscription: chatgpt). A search starts the free
tier; a metered provider joins when the free tier returns fewer than
`min_results` distinct URLs within the soft deadline, or always for a
`deep` search; each metered provider has a daily call cap after which it
is skipped as `budget`. Without this, refilled credits will drain at the
current volume within days.

Update (2026-10-14, owner: "efficiency before scaling"): the budget-aware
router is built on a branch (fa69be74b, not pushed, not deployed). Brave
is in the free tier within its pacing; a capped provider is skipped with
reason `capped` (not `budget`). Design, defaults, tuning, the ops note on
the daily counts and the rollback switch (`CHOIR_SEARCH_ROUTING=fanout`,
`CHOIR_SEARCH_PROVIDER_DAILY_CAPS=none`) are in
docs/archive/reports/search-efficiency-and-reuse-2026-10-14.md §4.1. It should
land before any paid account is refilled.

## Failure modes the fix must not have

1. A bench stops being set at all (a real 402 must still bench).
2. A provider in a true outage is called on every search because the
   episode test is too wide (e.g. treats every later call as "in flight").
3. A success no longer clears strikes / bench.
4. Pacing skips a provider that is idle (first call of a quiet period must
   go straight through), or waits past the caller's context.
5. Pacing counts as a failure or a strike, or a paced provider appears in
   the N of "M of N".
6. Caps change the existing classes' first-strike durations (60 s rate,
   2 min timeout, ten-empties rule).
7. The env override `CHOIR_SEARCH_BACKOFF_BASE_SECONDS` stops applying.
8. A skip reason leaks an error summary (secrets) into the log.
9. The NDJSON start line changes shape incompatibly for the guest
   (`internal/search` decodes `cooling[].provider/state/until`).

## Owner decisions (cannot be made by an agent)

Prices are public list prices as last seen (the repo's 2026-10-13 sourcing
report and earlier notes); verify on each provider's billing page.

1. **Which paid accounts to restore.** Out today: Tavily (plan usage
   limit; ~$0.008/credit pay-as-you-go), Parallel (prepaid balance spent),
   SerpAPI (monthly searches spent since 10-12), Serper (one-time credits
   spent since 10-08). At today's volume (~4,000 searches/day) and the
   current fan-out (every provider on every search), **each** metered
   provider costs on the order of $20–40/day (≈ $600–1,200/month) —
   e.g. Tavily 4,000 × $0.008 = $32/day; Brave paid ≈ $5/1,000 = $20/day;
   Exa ≈ $5–7/1,000 + page text. A SerpAPI/Serper monthly plan of a few
   thousand searches lasts about a day.
2. **Whether to cap spend before refilling**: approve the budget-aware
   router above (free tier first, metered only when short, daily caps
   per provider). Recommendation: yes, before any refill.
3. **Brave plan**: Free AI is 1 request/s. Pacing keeps it within that,
   but at ~4,000 searches/day Brave will then answer most searches and
   may reach its monthly allowance (amount not visible in the truncated
   error; check the Brave dashboard). Upgrade or accept that it goes
   `quota_limited` mid-month.
4. **Exa**: fixing the request turns on a metered provider that has made
   2 successful calls ever. Keep the fix (spend) or drop the commit.
5. **ChatGPT search**: shares the ChatGPT/Codex usage limit with
   inference (hypothesis in the 10-11 record). Keep it in the per-search
   fan-out or move it to a deep lane only.
6. **SearXNG**: its engines are blocked from Node B's address. Enable
   engines that tolerate datacenter traffic (an orange nix change) or
   remove it.
7. **Search volume**: 971 searches in one hour from the Autopaper computer.
   Whether that rate is intended is a product question for the Autopaper
   schedule, not the gateway.

## Protected surfaces, evidence class, rollback

- Protected surface: gateway provider calls (red). No provider keys, env,
  or health records were changed; nothing was restarted.
- Admissible evidence for the fix: unit tests of the policy, store and
  router; on staging, the health route after a day of traffic (strike
  counts equal to episodes, bench lengths within the caps, Brave
  `paced` instead of 429-benched) and the journal's per-search line.
- Rollback: `git revert` of the fix commits. Stored health records written
  by the new code are the same schema and remain valid for the old code.
- Stored benches written by the old code (Tavily to 10-18, Parallel to
  10-17, Exa to 10-20, ChatGPT to 10-15) stay until they expire or an
  operator resets them (`POST /provider/v1/search/health/reset`); the
  owner rule from 10-11 is no health-store edits by agents.

## Heresy delta

- discovered: per-call strikes counting one episode many times; unbounded
  doubling for transient classes; plane with no cost model fanning every
  search to every paid provider; Exa request shape vs read cap; log label
  "cooling" conflating exhausted, rate-limited and empty.
- introduced: none (this commit is documentation).
- repaired: none yet.

## Fix on the branch (after this record; not pushed, not deployed)

- f9294d4ec `red(searchplane)`: design items 1–4. One episode, one strike
  (`sameEpisode` in `health.go`; `FileHealthStore.RecordOutcome`
  serialized); per-class caps (`BackoffPolicy.MaxByClass`, quota base
  1 h); pacing (`Config.MinInterval`/`PaceWait`,
  `CHOIR_SEARCH_PROVIDER_RPS`, default `brave=1`, wait 3 s); skip reasons
  (`cooling[].reason` on the stream, `skipped=[name=reason …]` in the log
  line); "quota limit exceeded" classed as quota. Tests:
  `searchplane/benching_test.go` (failure modes 1–8), updated
  `policy_test.go`, `search_stream_test.go`.
- 020aeeb61 `red(gateway)`: Exa bounded request and 2 MiB read cap
  (`search_exa_test.go`). Drop this commit if the owner does not want Exa
  spending.

Repaired by these (once deployed): burst strikes, multi-day benches for
transient classes, Brave benched by our own bursts, Exa parse failures,
the misleading log label. **Not** repaired: the four spent accounts
(owner), SearXNG's blocked engines (owner/ops), the absence of a cost
model (owner decision 2). After deploy the plane is still Keenable +
Brave until the owner acts; what changes is that Brave stops flapping and
any top-up is picked up within hours instead of days (stored benches
written by the old code — Tavily to 10-18, Parallel to 10-17, Exa to
10-20, ChatGPT to 10-15 — need an operator reset of that provider via
`POST /provider/v1/search/health/reset` with `X-Internal-Caller: true` on
Node B loopback, or they run out on their own).

Residuals: the gateway still logs no per-provider arrival status (the
cause of a single failure is only in the health store's last summary);
Brave's monthly allowance (`quota_limit` 2000 in the 10-11 fixture) may
be reached faster once pacing stops wasting its calls on 429s.
