# Search throughput: is Keenable underused? (2026-10-15)

Owner request (2026-10-15): "Keenable has an extremely generous free tier,
so we should take advantage of it. We basically hit the limits on
everything else, but we could probably triple our Keenable throughput."

Mutation class of any fix: **orange** (provider routing in
`internal/gateway/searchplane`). Rollback: revert the fix commits, or set
`CHOIR_SEARCH_KEENABLE_MONTHLY_ALLOWANCE=0` (no monthly pace) and
`CHOIR_SEARCH_PROVIDER_RPS=keenable=0` (no Keenable pacing) in the gateway
environment. Problem first; the fix commits come after this one and
reference it.

## Evidence

### Keenable billing page (owner, 2026-10-15 about 03:40Z)

- Free plan plus pay-as-you-go: 100,000 free requests a month, **89,224
  left**, renews **2026-11-05**. After that, $4.00 per 1,000 requests.
- **No payment method on file.** Past the allowance, Keenable stops
  answering until 11-05. We must not plan on overflow.
- Rate limit: 10 requests per second.
- Used this period: 100,000 − 89,224 = **10,776**.

### Gateway journal (Node B, as relayed by the owner)

Nearly every search logs:

```
answered 1 of 1 providers (8 cooling) skipped=[searxng=success_empty
tavily=quota_limited brave=quota_limited parallel=quota_limited exa=error
serper=quota_limited serpapi=quota_limited chatgpt=rate_limited] ...
routing=budget ... today=[keenable=NNN]
```

and at startup `caps=[chatgpt=50,exa=150,parallel=150,serpapi=100,serper=150,tavily=150]`.
Keenable has no cap. Exa has been `exa=error` on every search since at
least 10-14 14:56Z (2,335 lines).

### Search volume (earlier records)

From docs/archive/problems/search-is-duplicated-and-discarded-2026-10-14.md and
docs/archive/problems/search-providers-cooling-most-of-the-time-2026-10-14.md:
searches per day (all computers) 10-11 490, 10-12 401, **10-13 3,183,
10-14 4,226 by 14:21Z**; the Autopaper computer makes about two thirds.
Repeats are 3.5–4.4% of searches, so the result cache (30 min) saves few
calls. Since 10-13 Keenable has been the only provider answering nearly
every search, so Keenable calls ≈ searches.

## Findings

### 1. Keenable is not underused at today's run rate (the premise is off)

The owner's "~1,000 a day" is 10,776 spread over the ten days since 10-05.
But almost all of that use is from 10-13 on:

| Window | Searches (≈ Keenable calls) |
|---|---|
| 10-05 to 10-12 | ≈ 1,050 (101 + 56 + 490 + 401, plus small earlier days) |
| 10-13 | 3,183 |
| 10-14 to 14:21Z | 4,226 |
| 10-14 14:21Z to 10-15 03:40Z (by subtraction from 10,776) | ≈ 2,900 |

That last row is 13.3 hours at about 220 an hour: **a run rate of about
5,000 Keenable calls a day**, not 1,000. The free headroom is 89,224 over
the 21 days to 11-05: **≈ 4,250 a day** at 100%, ≈ 3,800 a day at 90%.
So the paper is already running slightly **above** the free pace. At this
rate the allowance runs out about 11-01; tripled (≈ 15,000 a day) it runs
out about 10-21, and search then has no answering provider until 11-05.
(Hypothesis grade: the per-day split is reconstructed from three
measurements; the exact daily Keenable counts are in
`provider_daily_calls` in `/var/lib/go-choir/gateway/search-health.db`
from 10-14 about 17Z, and on `GET /provider/v1/search/health`
`daily_calls.keenable`.)

What Keenable lacks is not demand but a **pace**: nothing stops it from
spending the month early, and nothing protects it from a demand spike.

### 2. What limits search volume end to end

Gateway (`internal/gateway`):

| Limit | Value | Binds? |
|---|---|---|
| Per-computer search rate limit (`handlers.go` `computerID:search` bucket; `GATEWAY_RATE_LIMIT_MAX_REQUESTS`/`_WINDOW`) | 60 a minute per computer | no: peak was 971 an hour over all computers |
| Paid daily caps (`searchplane.DefaultDailyCaps`) | serper/parallel/exa/tavily 150, serpapi 100, chatgpt 50 | no: those accounts are spent or failing, so they are benched before their caps |
| Keenable cap | none | — (that is the gap) |
| Keenable pacing (`CHOIR_SEARCH_PROVIDER_RPS`, default `brave=1`) | none | no today (≈ 0.06 rps mean, bursts of 2–10 in a second), but nothing keeps a burst under Keenable's 10 rps |
| Result cache (`CHOIR_SEARCH_CACHE_TTL_SECONDS`, 30 min, per computer) | — | reduces calls by a few percent |
| `max_results` | 40 (gateway cap 50) | per search, not volume |

Guest (one client, `internal/search/search.go` `Stream`, called only from
`internal/agentcore/desk_search.go` `startSearch`, reached when a desk
cell runs `choir.SearchStart`):

| Limit | Value | Binds? |
|---|---|---|
| Searches per run / story / poll / turn / day | **none** | — |
| `deskSearchMaxResults` (`desk_search.go`) | 40 | per search |
| `researchChildConcurrency` (`research_children.go`) | 4 children at once | indirect |
| `researchChildCallInterval` | 1.5 s between child model calls | indirect |
| Autopaper source poll (`source_schedule.go`) | 15 min default per source; a poll makes no searches itself | indirect |

Measured demand: 136 paper research runs in 4 hours, 9.1 searches a run
(median 6, max 71). **No configured limit binds.** Search volume is
(research runs) × (searches the model chooses per run); research runs
follow source arrivals through management → Texture → research.

So there is no demand-side limit to raise, and raising demand is not
safe: finding 1 says today's demand already meets the free pace.

### 3. The `today=` counter is durable, not in memory

`today=[keenable=NNN]` comes from `searchplane.Route.noteCount`, fed by
`CallCounter.ReserveCall`. The gateway's counter is the health store
(`FileHealthStore`, table `provider_daily_calls` in
`/var/lib/go-choir/gateway/search-health.db`, since 0e8248b5c on 10-14),
so counts **survive a restart**. They reset at UTC midnight, by design
(a count is per UTC day). Only `CHOIR_SEARCH_HEALTH_MEMORY=1` (not set on
Node B) would make them in-memory.

The counts start mid-period: the ≈ 9,000 Keenable calls before 10-14 17Z
are not in the table. A monthly pace computed from the table alone would
undercount this period's use.

### 4. Why Exa fails: logged nowhere in the journal

A failing call's error goes to the stream (`arrival.Error`, redacted, to
the guest) and to the health store (`last_error_summary`, redacted, on the
loopback route `GET /provider/v1/search/health`). The gateway journal only
shows the skip reason class (`exa=error`). `error` is the class for an
error that is not a status the classifier knows (no 4xx/5xx match).

The 10-14 record read the health store directly: Exa's last failure is
`parse response: unexpected end of JSON input`, with the strong hypothesis
that our own request shape breaks it (`numResults=40` with
`contents.text=true`, read through a 256 KiB `io.LimitReader`, so the
JSON is cut). The adapter fix was left for an owner decision (it turns a
metered provider back on) and is still not in `ExaProvider.Search`. So
Exa is very likely **not** a key or credit problem.

## What the fix must do (and not do)

1. Pace Keenable over its billing period: each UTC day's cap is (period
   target − period used before today) / days left, with the target 90% of
   100,000 and the period renewing on the 5th; env-driven.
2. Count from the durable daily counts, so a restart does not reset the
   pace; accept a seed (calls used earlier in the period that the counts
   never saw) so this period's pace is not overstated.
3. Spread the day's cap across the day, so a demand spike in the morning
   does not leave the paper without search all afternoon.
4. Keep Keenable at ≤ 8 calls a second across every computer (client-side
   pacing, the existing `MinInterval` pacer).
5. Keep Keenable started on every budget-routed search while it has
   budget; paid providers stay in reserve for escalation.
6. Log, for each failing provider call, the outcome class, HTTP status and
   failing stage; never the key, never the body.

Failure modes the fix must not have:

1. The period total passes the allowance (any day, any restart, any
   concurrency).
2. A gateway restart forgets the period's use.
3. Keenable is called above 10 rps.
4. A capped Keenable counts as a failure or a strike, or appears in N.
5. Paid providers start before Keenable while Keenable has budget.
6. The period boundary is off by a month (renewal day, short months,
   year end).
7. The error log line carries a key or a response body.
8. A provider without a monthly quota changes behavior.

## Fix (branch `feat/keenable-throughput`, 37f9c5ac3; not pushed, not deployed)

- `searchplane/monthly.go` with the router changes: the monthly pace above.
  The day's cap is computed once per UTC day from `provider_daily_calls`,
  logged as `searchplane: keenable monthly pace day= period= used= target=
  days_left= day_cap=`, and shown on the health route as
  `daily_calls.keenable.monthly`. The cap accrues through the day (an
  hour's share open at UTC midnight, all of it by 23:00). A capped Keenable
  is skipped as `capped` with `until` = when the next call opens.
- `DefaultProviderRPS` is `brave=1,keenable=8`; `CHOIR_SEARCH_PROVIDER_RPS`
  now merges over the defaults (rate 0 removes one provider's pacing).
- Each failing provider call logs `searchplane: <p> call failed class=
  http_status= stage=` (for example `stage=parse_response` for Exa if the
  10-14 hypothesis holds).
- `nix/node-b.nix` writes out the defaults and seeds this period with
  `CHOIR_SEARCH_KEENABLE_PERIOD_USED=10776@2026-10-15`. The seed already
  includes 10-15's first 3h40m, which the 10-15 count also includes. That
  double count (several hundred calls) errs on the safe side. Drop the
  seed after 11-05; the code ignores it outside its period anyway.
- Routing preference: unchanged and already as asked. Keenable is free
  tier, so it starts on every budget search while it has budget, and the
  paid providers wait in reserve (test
  `TestMonthlyPace_KeenableFirstThenEscalatesWhenCapped`).

Tests (failure modes listed in the header of `searchplane/monthly_test.go`):
the pace never passes the target at any demand level (pure pace and
through the router with a restart every day), a restart mid-day keeps the
day's count, concurrent searches never pass the cap, period boundaries,
the seed, the accrual, 8/s under concurrent searches, the env knobs, and a
failure log line with no key, query or body.

Expected volume after deploy (if deployed about 10-16): period use
≈ 10,776 + about 5,000 (10-15) ≈ 15,800. Target 90,000. That leaves
≈ 74,000 over 20 days, **≈ 3,700 Keenable calls a day**, rising if demand
dips. Demand of about 5,000 a day means about 1,300 searches a day find
Keenable capped. Those searches fall to Brave (free tier, when it is not
quota-limited) or come back empty. That is the price of having Keenable
every day until 11-05 instead of losing it from about 11-01.

## Owner decisions left open

- **Exa adapter fix** (10 results, page text capped, larger read): turns
  a metered provider back on (≤ 150 calls a day at its cap). Unchanged.
- **More search volume** needs either more free allowance or a paid
  provider refill; Keenable alone carries about 4,000 searches a day at
  the free pace. Paying for Keenable overflow ($4 per 1,000) needs a
  payment method on file.
