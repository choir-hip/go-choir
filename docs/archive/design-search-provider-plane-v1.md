1su|# Design: Search Provider Plane v1
2yy|
3ye|**Status:** approved design (implement before further search prompt work)  
4bh|**Date:** 2026-05-26  
5hm|**Owner:** `internal/gateway`  
6kd|**Related:** `docs/mission-research-runtime-evidence-cadence-v1.md`, `internal/gateway/search.go`
7yy|
8ej|---
9yy|
10ly|## Summary
11yy|
12ae|Choir product path routes researcher `web_search` through the gateway (`sandbox -> gatewaySearchClient -> POST /provider/v1/search`). Search is **infrastructure**, not agent policy. This design replaces the current sequential round-robin loop, fixed 10-minute in-memory cooldown map, and success-with-zero-results behavior with a **durable control plane**: pluggable providers, persistent health, exponential backoff, parallel fan-out, and explicit outage errors.
13yy|
14vn|**Non-goal:** Researchers or VText managing provider rotation via prompts.
15yy|
16ej|---
17yy|
18jo|## Problem statement
19yy|
20sv|Observed failures (staging Trace, mission evidence):
21yy|
22ns|- Multiple `web_search` calls returning **0 results** while HTTP succeeds
23ez|- Most providers in **cooling_down** or quota-limited; only 1-2 healthy at a time
24dr|- Current gateway calls providers **sequentially** until K succeed (latency = sum, not max)
25ih|- Cooldown is **fixed duration**, **overwrites** on each failure (no strike escalation)
26lx|- Health is **in-memory** — lost on gateway restart
27zr|- Researchers stall retrying search instead of checkpointing
28yy|
29dg|We will **not** design agents around empty search. We fix the gateway.
30yy|
31ej|---
32yy|
33uv|## Goals
34yy|
35lo|| Goal | Requirement |
36aq||------|-------------|
37uw|| High yield | No 200 + empty results when any eligible provider can return hits |
38cz|| Healthy-only | No HTTP to providers in cooling_down until cooldown_until |
39xc|| Parallel fan-out | Default K=2 providers per wave; latency ~ max(call times) |
40pr|| Learn and recover | Exponential backoff on rate limit / quota / repeated failure; reset on success |
41cb|| Pluggable | Add/remove providers without changing router |
42mn|| Durable health | Survive gateway restart; support tier upgrades via reset |
43vr|| Observable | Trace, tool output, ops API show attempts + health |
44md|| Configurable | Policy via env for K, timeouts, backoff |
45yy|
46ej|---
47yy|
48ld|## Architecture
49yy|
50fy|```
51no|POST /provider/v1/search
52fb|        |
53wo|        v
54uz|   SearchService  (validate, clamp max_results)
55fb|        |
56ji|   Router + Executor + Merger + Policy
57fb|        |
58rb|   Registry | HealthStore | BackoffPolicy
59fy|```
60yy|
61wx|**Layers:** Provider adapters (HTTP only); Registry; durable HealthStore; BackoffPolicy; parallel Router/Executor; URL Merger.
62yy|
63wc|**Deprecate** `internal/search` for product paths; gateway is canonical.
64yy|
65ej|---
66yy|
67dl|## Provider registry
68yy|
69oa|- Interface: `Name()`, `IsConfigured()`, `Search(ctx, query, maxResults)`
70yr|- Built-in priority order: tavily, brave, parallel, exa, serper
71oh|- Adding provider #6: new adapter + registry entry only
72yy|
73ej|---
74yy|
75kr|## Health store
76yy|
77er|Per-provider record: `state` (active | cooling_down | disabled), `cooldown_until`, `strike_count`, rolling window stats (`window_attempts`, `window_successes`, `window_results_total`), `last_failure_class`, truncated `last_error_summary`.
78yy|
79vm|**Interface:** `Snapshot`, `Get`, `RecordOutcome`, `ResetProvider`
80yy|
81hb|**v1:** SQLite file at `CHOIR_SEARCH_HEALTH_PATH` (default under `/var/lib/go-choir/gateway/search-health.db`).
82yy|
83mf|**Future:** RemoteHealthStore for multi-instance gateway.
84yy|
85ej|---
86yy|
87zw|## Outcome classification
88yy|
89wh|| Class | Cooldown | Strikes |
90ko||-------|----------|---------|
91pb|| success (>=1 result) | none | reset to 0 |
92wt|| success_empty | yes (shorter base) | +1 |
93my|| rate_limited / quota_limited | yes (24h base) | +1 |
94mt|| auth_error | long + disabled | +1 |
95fj|| server_error / timeout | short | +1 |
96lk|| skipped_cooling_down | no HTTP | — |
97yy|
98ej|---
99yy|
100cp|## Backoff policy
101yy|
102fy|```
103li|cooldown = min(max_cooldown, base_for_class * multiplier^(strike_count - 1))
104fy|```
105yy|
106wo|Defaults: base 24h, max 7d, multiplier 2. Env: `CHOIR_SEARCH_BACKOFF_BASE_SECONDS`, `CHOIR_SEARCH_BACKOFF_MAX_SECONDS`, `CHOIR_SEARCH_BACKOFF_MULTIPLIER`.
107yy|
108ru|**On success:** reset strikes, state=active.
109yy|
110ld|**Ops:** `POST /provider/v1/search/health/reset` after paid tier upgrade.
111yy|
112ej|---
113yy|
114ho|## Router algorithm
115yy|
116rn|1. Load health; build eligible set (configured, enabled, not cooling).
117rs|2. If zero eligible -> `search_outage` (503), no HTTP.
118ie|3. Score eligible; pick K = min(CHOIR_SEARCH_PROVIDERS_PER_QUERY, len(eligible)) with fairness.
119fh|4. **Parallel** Search on K providers (errgroup, per-call timeout).
120vi|5. Record outcomes; merge with URL dedupe.
121kf|6. Optional second wave if merged < MIN_MERGED_RESULTS and more eligible remain.
122me|7. If merged still 0 -> `search_outage`; never 200 + [].
123yy|
124ab|**Env:** `CHOIR_SEARCH_PROVIDERS_PER_QUERY=2`, `CHOIR_SEARCH_MIN_MERGED_RESULTS=5`, `CHOIR_SEARCH_MAX_WAVES=2`, `CHOIR_SEARCH_REQUEST_TIMEOUT_SECONDS=30`.
125yy|
126ej|---
127yy|
128jn|## API contract
129yy|
130in|Extend `SearchResponse` with `provider_health`, `merged_count`, `waves`, `degraded`.
131yy|
132lj|Errors: `no_search_providers_configured`, `search_outage` (includes health snapshot).
133yy|
134ep|**Forbidden:** HTTP 200 with empty results when providers were eligible.
135yy|
136ve|**Ops:** `GET /provider/v1/search/health`, `POST /provider/v1/search/health/reset`.
137yy|
138ej|---
139yy|
140bg|## Runtime / Trace / Chyron
141yy|
142ih|- Runtime projection: compact attempts + one-line provider health summary
143dg|- Trace: search stats + health per trajectory
144df|- Chyron (future): server line e.g. "Search: 12 hits (brave, parallel)"
145yy|
146sb|Researchers never manage cooldowns.
147yy|
148ej|---
149yy|
150ie|## Testing
151yy|
152ie|BackoffPolicy table tests; classifier tests; HealthStore persist; router parallel vs sequential; zero eligible outage; live integration probes (env-gated).
153yy|
154ej|---
155yy|
156pk|## Migration phases
157yy|
158ad|| Phase | Deliverable |
159ob||-------|-------------|
160il|| A | `internal/gateway/search/` package skeleton |
161ei|| B | FileHealthStore + tests |
162xd|| C | Parallel executor + outcome recording |
163an|| D | Wire SearchClient.Search -> Router |
164xh|| E | Response + runtime + Trace |
165vi|| F | Ops endpoints + runbook |
166yn|| G | Deprecate legacy internal/search |
167yy|
168ej|---
169yy|
170ck|## Runbook: tier upgrade
171yy|
172ja|1. Deploy new API key / tier in gateway env.
173ue|2. POST health/reset for provider names.
174qe|3. Verify GET health shows active, strike_count 0.
175cn|4. Confirm staging web_search merged_count >= min.
176yy|
177ej|---
178yy|
179wg|## Acceptance criteria
180yy|
181ld|- [ ] Two healthy providers called in parallel per wave
182id|- [ ] Cooling providers skipped until cooldown_until (no HTTP)
183op|- [ ] Strike doubles backoff (unit test)
184vg|- [ ] Health survives gateway restart
185bu|- [ ] No success+empty when eligible providers exist
186fj|- [ ] search_outage when all cooling
187ji|- [ ] Ops reset without redeploy
188yy|
189ej|---
190yy|
191ef|## References
192yy|
193cz|- `internal/gateway/search.go`
194kg|- `internal/runtime/search_gateway.go`
195cq|- `docs/mission-search-provider-plane-v1.md` — MissionGradient execution mission (`/goal` in that file)
- `docs/mission-research-runtime-evidence-cadence-v1.md`
196yy|
