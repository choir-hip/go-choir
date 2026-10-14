# Search efficiency and reuse: design (2026-10-14)

Problem record: `docs/problems/search-is-duplicated-and-discarded-2026-10-14.md`
(evidence, failure modes). Companion: `docs/problems/search-providers-cooling-most-of-the-time-2026-10-14.md`
(provider health; the budget-aware router sketch reused in §3).
Owner rule: efficiency before scaling; "I would rather put more money into
storing more text and making the object graph bigger" than into repeat
searches. Mutation class: red (gateway provider calls) for §1 and §3;
orange to red for §2 and §4 (object graph writes, corpus search).

## 0. What the measurement says, in one paragraph

About 4,000 searches a day, two thirds from the Autopaper computer, each
fanned out to every healthy provider with `max_results=40`. Exact repeats
are only 3.5% (4.4% with generous normalization), mostly the QA computers
running a fixed prompt. A third of searches (31%) re-ask, within ten
minutes, something the same computer just asked — a researcher hunting one
page by quoted phrases. About 1.5% of returned results are read; none are
kept. 27% of page reads fail at the publisher. Corpus search sees titles of
the newest 1,000 captures and never replaces a web search. So the levers,
in order of money: **(a) stop paying every provider for every search, (b)
keep what we fetched and look there first, (c) don't repeat a search we
just made.** (c) is the smallest and the easiest; it ships first because it
is contained and it is the instrument that measures result duplication.

## 1. Slice 1 (built on this branch): result cache, in-flight join, result-level log

Where: `internal/gateway/searchplane/cache.go` (new), wired in
`SearchClient.StreamFor` and `HandleSearch`.

- **Key**: scope + query with whitespace collapsed + `max_results`. No
  case folding, quotes and operators kept (`OR`, `site:`, phrases mean
  something; case folding added zero repeats in the data anyway). Scope is
  the computer id by default; `CHOIR_SEARCH_CACHE_SCOPE=shared` widens it
  to all computers (owner decision §6.2).
- **Hit**: an identical search answered within the TTL is replayed from
  memory: the same NDJSON shape (start, one line per provider that
  answered with results, done), `latency_ms` 0, and an additive
  `cached_at` on the start line. No provider is called; the health store
  is not touched.
- **Join**: an identical search already in flight is joined: the joiner
  receives every arrival already landed, then the rest as they land. The
  shared search runs on a context detached from the first caller (bounded
  by the provider timeout plus pace wait), so the first caller leaving
  does not cancel the joiner's providers.
- **What is cached**: only arrivals that answered with at least one
  result; a search with none is not cached (a failure or empty answer
  never hides a later success).
- **TTL**: 30 min by default, 10 min for time-sensitive queries (a
  time word — today, latest, breaking, this week, … — or a month name).
  Data: 177 of 300 repeats came within 10 min, 255 within an hour.
  `CHOIR_SEARCH_CACHE_TTL_SECONDS`, `CHOIR_SEARCH_CACHE_FRESH_TTL_SECONDS`.
- **Bound**: 64 MiB of result text (`CHOIR_SEARCH_CACHE_MAX_MB`), oldest
  evicted first. Keenable entries carry page text (~100 KB each).
- **Kill switch**: `CHOIR_SEARCH_CACHE_TTL_SECONDS=0` disables cache and
  join; the path is then exactly today's.
- **Measurement**: the per-search log line gains
  `served=miss|hit|joined|off results=N unique=M seen=K`, where `seen` is
  how many of this search's distinct URLs the same computer already
  received from an earlier search in the last 24 h (in-memory, bounded).
  That is the number the owner asked for ("are we getting duplicated
  results") and today cannot be measured.

Expected effect: 2.6% (per-computer scope) to 3.8% (shared) fewer
searches, ×(providers per search) fewer provider calls; QA reruns of a
fixed prompt mostly free. Small on its own; it is also the first tier of
the budget router (a cache answer costs nothing).

## 2. Slice 2: keep every search, as objects with provenance

Today the guest's `SearchStore` holds results for one worker session. The
guest already has the full frames when a search ends (`forwardSearch` in
`internal/agentcore/desk_search.go`), and it already writes captures to
corpusd Store B through `CorpusdCaptureSink`. Write each finished search
there too:

- **Object** `choir.search_result` (one per search): owner, computer, run,
  desk, search id, query, normalized key, started/finished, providers
  answered and skipped (with reasons), and the merged hits (title, URL,
  canonical URL, snippet, published_at, provider, rank). Hits go into a
  content-addressed body blob like capture bodies; metadata stays small.
- **Edges**: `search_result —returned→ url-key` per distinct canonical
  URL; when a URL is later read, the capture gets
  `found_by → search_result`. That is the "building up the object graph
  from searching" the owner asked for: every URL we ever saw, which query
  found it, which provider, and whether we read it.
- **Not a capture.** A hit is a third party's description of a page, not
  the page. Keeping it a separate kind keeps provenance honest and keeps
  hits out of the newest-1,000 capture window (failure mode 12).
- **Run event**: `search_done` gains the query, the hit count and the
  object id, so a trace shows what was asked and where its answer lives.
- **Size**: ~50 hits × ~1.5 KB ≈ 75 KB per search raw, ~300 MB/day at
  today's volume; after URL de-dupe and zstd, an estimated 50–100 MB/day
  (1.5–3 GB/month). Captures already write about 1.4 GB/day (measured), so
  hits are a small addition. Disk, not search hits, is the constraint (§6.3).
- **Provider terms**: some search APIs restrict storing results beyond
  caching; whether each provider's plan permits storage is an owner check
  before slice 2 writes snippets (§6.5). Titles and URLs alone are safe to
  keep under any reading; snippets/page text are the question.

## 3. Slice 3: look where we already are, before the web

1. **A full-text index over the corpus** in corpusd: capture blocks and
   search hits, BM25 with phrase queries (SQLite FTS5 next to Store B, or
   Bleve in-process; FTS5 is the conservative choice: one file, already a
   dependency of the gateway). It removes the newest-1,000 bound and the
   titles-only match. Quoted-phrase lookups — 18% of queries carry quotes,
   5% two or more phrases — are exactly what phrase search over held text
   answers, and what embeddings do not.
2. **`SearchWireCorpus` reads that index**, and returns earlier
   searches' hits (`search_result` objects) next to captures and stories.
3. **Read-through**: `choir.Read(url)` first looks for a capture of the
   same canonical URL younger than a freshness bound (e.g. 6 h for news,
   7 d for docs/READMEs) and returns it with its capture id; `fresh: true`
   forces a fetch. 17% of reads today re-fetch a URL already read.
4. **Refused-read fallback**: when a publisher refuses a read (27% of
   reads), return the stored hit text for that URL, labelled as a search
   provider's snippet, not the page. Evidence class stays honest; the
   researcher can cite the publisher and say it was not readable.
5. **Prompt and budget**: the research prompt says "search what the
   computer holds, including earlier searches, before the web"; the desk
   sees a per-run search budget (e.g. 12 web searches, counted in the cell
   footer) and the reformulation it is about to make. The budget is a
   pressure, not a wall: the 71-search run is the case it targets.

Expected effect: the reformulation third (31% near-duplicates, mostly
lexical) is where this bites; if half of it is answered from held text,
**~15% fewer searches**. Reads saved: ~17% of fetches.

## 4. Slice 4: budget-aware routing (owner decision in the cooling record)

From the cooling record, unchanged: providers carry a cost tier (free:
cache, Keenable, SearXNG; metered: Brave, Tavily, Parallel, Exa, Serper,
SerpAPI; shared subscription: ChatGPT). A search starts with the free tier;
one metered provider (cheapest healthy, rotating) joins when the free
answers return fewer than `min_results` distinct URLs within the soft
deadline, or for an explicit `deep` search; each metered provider has a
daily call cap, after which it is skipped as `budget` (a skip reason, not a
strike). The cache in §1 is the first free tier.

Estimate at 4,000 searches/day with every paid account refilled
(list prices as last seen, verify on billing pages: Tavily $0.008, Brave
paid ~$0.005, Parallel ~$0.005, Exa ~$0.005–0.007, SerpAPI ~$0.015,
Serper ~$0.001 per call):

| Routing | Metered calls/search | Metered calls/day | ≈ $/day | ≈ $/month |
|---|---|---|---|---|
| today's fan-out, all accounts healthy | 6 | 24,000 | ~$155 | ~$4,700 |
| free first, one metered on shortfall (Keenable answers 98.9% of calls; assume 10% shortfall) | 0.1 | 400 | ~$3 | ~$90 |
| free first plus one metered always (diversity) | 1 | 4,000 | ~$20–30 | ~$600–900 |
| plus §1 and §3 (−18% searches) | ×0.82 | | | |

This is the largest lever by two orders of magnitude, and it should land
before any account is refilled.

### 4.1 Slice 4 design (owner, 2026-10-14: "efficiency before scaling")

The owner will spend more on search once it is efficient, and wants more
resources for important stories, so paid providers stay reachable: the
router escalates to them when the free answers fall short, it does not
remove them. Mutation class red (gateway provider calls). Contained to
`internal/gateway/searchplane` (router, config, health DB), the gateway
search handler and its log line; one guest tolerance change (below).

- **Tiers.** Each provider has a tier and a relative cost
  (`CHOIR_SEARCH_PROVIDER_TIERS`, merged over defaults): `free`
  (keenable, searxng, brave within its pacing), `paid` with a cost
  (serper, parallel, exa, tavily, serpapi, cheapest first by list price),
  `constrained` (chatgpt: shares the ChatGPT/Codex usage limit with
  inference; always last). A provider not named is `paid` at the highest
  cost: an unknown provider never spends by default.
- **Free first.** A search starts every eligible free provider at once (as
  today). Eligible paid providers are held in reserve; the start line
  names them (`reserve`, additive), they are not cooling and not in N.
- **Escalation rule** (one place, `shortfall` in the router). With
  `min = min(MinResults, max_results)`, the search escalates by **one**
  paid provider, cheapest first, when fewer than `min` distinct URLs have
  arrived and either (a) every started call has answered (`short`; `free_failed`
  when none answered with a result; `no_free` when no free provider could
  start), or (b) the step's soft deadline passed (`slow`). After each paid
  provider the rule is evaluated again, up to `MaxPaid` paid providers per
  search. No paid provider starts later than the escalation window after
  the search began. A `deep` search (optional request field
  `depth: "deep"`, which the guest may send later) starts the cheapest
  paid provider with the free tier and uses `DeepMinResults` /
  `DeepMaxPaid`.
- **Daily caps.** Each provider may have a calls-per-day cap
  (`CHOIR_SEARCH_PROVIDER_DAILY_CAPS`). Every provider call is counted in
  the health DB (`provider_daily_calls`, one row per provider per UTC
  day): one state authority, durable across restarts, a new day at UTC
  midnight. A call reserves its count atomically before it is sent, so
  concurrent searches cannot overshoot a cap. A capped provider is
  skipped with state and reason `capped`: not a failure, no strike, not in
  N. Free providers have no cap by default.
- **Stream shape.** Additive only: start line `routing`, `reserve`;
  `cooling[]` may carry `capped`; an escalated provider's arrival is an
  ordinary `provider` line; the final line's `total` counts every provider
  started (escalated ones included) and gains `escalated[]`
  (`provider`, `reason`). No new line type: the guest decoder rejects
  unknown types.
- **Guest tolerance.** The guest's search store takes N from the start
  line. An arrival from a provider not named there (an escalated one) is
  appended to its providers, so "M of N" stays M ≤ N. Old guests still work
  (they show M of N with M possibly above N until updated).
- **Log line.** Per search: `routing=budget|fanout`, `escalated=` (each
  step `reason:provider`, or `no`), `paid_calls=` (paid calls this request
  caused; 0 for a cache hit or join), `today=[provider=calls/cap …]` for
  the providers this search called or skipped as capped.
- **Ops.** `GET /provider/v1/search/health` gains `daily_calls`:
  per provider, today's UTC count, its cap and its tier.
- **Rollback switch.** `CHOIR_SEARCH_ROUTING=fanout` restores fan-out to
  every eligible provider; `CHOIR_SEARCH_PROVIDER_DAILY_CAPS=none` removes
  the caps. Both together are today's behaviour (counts are still
  recorded; they change nothing).
- **Per-run budget (not built).** The guest sends only `query` and
  `max_results`; the gateway sees the computer, not the run. The soft
  per-run budget needs the guest to send `run_id` (the desk run that owns
  the session, from `startSearch`'s caller) in the request body; the
  gateway would then count searches per (computer, run) and add
  `budget_exceeded: {searches, budget}` to the start and final lines
  after the threshold (12), and the guest would surface it in the cell's
  search stub. Skipped in this slice.

#### Failure modes this slice must not have

1. The free tier is sufficient and a paid provider is called anyway.
2. The free tier falls short and no paid provider is reached (an important
   story starves), including when a free provider hangs until its timeout.
3. Escalation starts every paid provider at once, or not cheapest first,
   or exceeds `MaxPaid`.
4. A cap is overshot by concurrent searches (check then act), counts are
   lost on restart, or the day rolls at local rather than UTC midnight.
5. A capped provider is recorded as a failure or strike, counted in N, or
   skipped without the reason `capped` on the stream and in the log.
6. A paid provider benched, paced or capped by the time it is reached is
   called anyway, or stops escalation instead of yielding to the next.
7. A cache hit or join calls a provider or counts a call; a `deep` search
   is answered from a cached ordinary one.
8. The stream changes shape incompatibly for the guest (new line type,
   `total` below the arrivals, removed fields).
9. The rollback switch does not restore fan-out to every eligible provider.
10. Escalation outlives the stream bound, so escalated arrivals are cut
    and their calls charged for nothing.
11. Shared state (route, counts) races between the coordinator, the cache
    joiners and the handler.

#### Slice 4 on the branch (not pushed, not deployed)

- fa69be74b `red(searchplane)`: the router, caps, stream fields, log
  line and health route above. Code: `searchplane/router.go` (one
  coordinator goroutine per search; `Route` is the shared record),
  `searchplane/counter.go` + `sqlite_health.go` (`provider_daily_calls`),
  `searchplane/config.go` (tiers, caps, thresholds),
  `gateway/search_plane.go`, `gateway/handlers.go`. Tests:
  `searchplane/budget_test.go` (failure modes 1–7, 9–11),
  `gateway/search_budget_test.go` (5, 7, 8, log line, health route).
- ab0f82bc6 `fix(yaegikernel)`: an escalated provider joins N in the
  worker's search store. Old guests keep working without it.

**Defaults** (all from `ConfigFromEnv`; nothing new is needed in the env):

| Setting | Env | Default |
|---|---|---|
| Routing | `CHOIR_SEARCH_ROUTING` | `budget` (`fanout` = every eligible provider at once) |
| Tiers and costs | `CHOIR_SEARCH_PROVIDER_TIERS` (merged over default) | keenable, searxng, brave `free`; serper `paid:1`, parallel `paid:5`, exa `paid:6`, tavily `paid:8`, serpapi `paid:15`; chatgpt `constrained`; unknown → paid at 1000 |
| Daily caps | `CHOIR_SEARCH_PROVIDER_DAILY_CAPS` (merged; `name=none` lifts one; `none` lifts all) | serper, parallel, exa, tavily 150; serpapi 100; chatgpt 50; free providers none |
| Enough results | `CHOIR_SEARCH_MIN_RESULTS` / `CHOIR_SEARCH_DEEP_MIN_RESULTS` | 10 / 30 distinct URLs (never above the request's `max_results`) |
| Paid per search | `CHOIR_SEARCH_MAX_PAID` / `CHOIR_SEARCH_DEEP_MAX_PAID` | 2 / 4 (`-1`: no paid provider; `0`: the default) |
| Step deadline | `CHOIR_SEARCH_ESCALATE_AFTER_MS` | 8000 |
| Escalation window | `CHOIR_SEARCH_ESCALATE_WINDOW_MS` | 20000 (the stream bound becomes 30 + 20 + 5 s) |

Worst case at the caps, list prices: 150 × ($0.001 + $0.005 + $0.006 +
$0.008) + 100 × $0.015 ≈ **$4.50/day (~$135/month)**, plus ChatGPT's 50
calls against the subscription limit. Expected, with every account
refilled and ~10% of searches short: ~400–500 paid calls/day, mostly
Serper, **≈ $1–3/day** against ~$155/day for today's fan-out (the table
in §4). Today the four paid accounts are spent, so the immediate change
is no spend; the saving applies from the first refill.

**Tuning.** Escalation rate too high (most lines `escalated=short…`):
lower `CHOIR_SEARCH_MIN_RESULTS` or check what Keenable returns for those
queries. Important stories thin: raise `CHOIR_SEARCH_MAX_PAID` or a
provider's cap, or have the guest send `depth: "deep"` for them (the
guest does not send it yet: `internal/search` would add it to the request
body, and the desk would choose it). Cheaper or better provider first:
change its cost in `CHOIR_SEARCH_PROVIDER_TIERS`. Stop one provider
spending without removing its key: `CHOIR_SEARCH_PROVIDER_DAILY_CAPS=tavily=0`.

**Ops: reading the daily counts.** On Node B loopback,
`curl -s -H 'X-Internal-Caller: true' http://127.0.0.1:8084/provider/v1/search/health | jq .daily_calls`
returns `daily_calls`: per provider `{day, calls, cap, tier}` for the
current UTC day (`cap` −1 = no cap). The journal's per-search line ends
`routing=budget escalated=short(3):serper paid_calls=1 today=[keenable=812 serper=14/150]`:
`escalated=no` means the free tier was enough; `reason(unique)` is why and
at how many distinct URLs it escalated; `paid_calls` is paid calls this
request caused (0 for a cache hit or a join; the leader's line counts
them); `today` is the day's running count for each provider this search
called or skipped as capped. A day's paid spend is the sum of
`paid_calls`, or the `calls` of paid providers on the health route. A
capped provider shows `skipped=[… serper=capped]` until UTC midnight. The
counts live in `/var/lib/go-choir/gateway/search-health.db`, table
`provider_daily_calls` (provider, day, calls), kept 40 days.

**Not built: the per-run soft budget.** The guest sends only `query` and
`max_results`; the gateway sees the computer, not the run. It needs the
guest's `startSearch` to send the owning desk run's id (`run_id`) in the
request body; the gateway would then count searches per (computer, run)
and, past 12, add `budget_exceeded: {searches, budget}` to the start and
final lines, which the guest shows in the cell's search stub. No hard
refusal.

Residuals: `SearchClient.ensurePlane` is unsynchronized for the first
concurrent searches after a gateway start (pre-existing); the escalation
threshold (10 distinct URLs) is a guess until the journal shows the
escalation rate; a pacing slot booked for a provider that is then capped
is not returned (it only delays that provider's next call by its
interval).

## 5. Horizon: local embeddings and a retrieval index over the graph

The owner wants local embeddings (no cloud dependency) and "Hadoop"-scale
retrieval eventually. Sized from today's numbers:

**Inventory growth.** ~4,500 captures/day (~10 blocks each → ~45k chunks)
plus ~50–100k distinct search hits/day (one chunk each: title + snippet)
→ **~100–150k chunks/day, ~40–55M chunks/year** at today's volume.

**Models (local, open weights).**

| Model | Size | Dim | Languages | CPU (Node B, 6c/12t Xeon E-2236, no GPU), rough | GPU (L4 / RTX 4000 Ada class), rough |
|---|---|---|---|---|---|
| bge-small-en-v1.5 | 33M | 384 | English | 150–300 chunks/s → a day's chunks in ~10 min | trivial |
| multilingual-e5-small | 118M | 384 | 100 | 60–150 chunks/s → 15–40 min/day | trivial |
| EmbeddingGemma-300M | 308M | 768 (MRL to 128) | 100+ | 30–80 chunks/s → 30–80 min/day | ~2k/s |
| bge-m3 / Qwen3-Embedding-0.6B | 0.57–0.6B | 1024 | 100+ | 10–30 chunks/s → 1.5–4 h/day of all cores | ~1–3k/s, minutes/day |

Throughputs are order-of-magnitude estimates for ONNX/llama.cpp int8 at
~256 tokens per chunk; benchmark before choosing. Autopaper is multilingual
(the OSINT sourcing report), which favours EmbeddingGemma-300M or bge-m3
over English-only models. Node B runs production on 12 threads and 31 GB;
a small multilingual model fits there as a low-priority batch job; a 0.6B
model belongs on a separate box (a single-GPU server rents for roughly
$150–250/month; a used CPU box with 64 GB for less; verify prices).

**Vector storage.** 1M chunks × 1024 float32 = 4 GB; int8 1 GB; binary
128 MB. At 40–55M chunks/year: float32 ~200 GB, int8 ~50 GB, binary ~6 GB
(with int8 rescoring on disk). Quantization and a retention tier (e.g.
search hits older than 90 days drop to title + URL) are part of the design,
not an afterthought.

**Index.** Qdrant is already in the repo (`internal/qdrant`, dormant,
owner-kept for a retrieval layer): HNSW, payload filters (owner, computer,
kind, time), scalar/binary quantization, sparse vectors for BM25-style
hybrid in one engine, single binary on CPU. Alternatives: SQLite
`sqlite-vec` or `usearch` embedded (simplest, single node, no service);
LanceDB (columnar, disk-first); pgvector (we run no Postgres). Recommended
path: **FTS5 now (§3) → Qdrant hybrid (dense + sparse) when embeddings
land**, with the FTS index kept for phrase queries.

**Hadoop.** Not needed at this scale. Hadoop/HDFS/Spark are batch systems;
the need here is low-latency hybrid retrieval over 10^7–10^8 chunks, which
one Qdrant node with quantization serves on 64–128 GB RAM. Batch work —
re-embedding the corpus when the model changes — is a queue job over the
object graph. Revisit distributed serving (Qdrant cluster, Vespa,
OpenSearch) past ~10^9 chunks.

**One state authority.** The object graph (Dolt rows + content-addressed
blobs) stays the only authority. The FTS and vector indexes are derived
projections: each row carries the object id and `embedding_model` /
`embedding_version` (fields already on `WebCaptureMetadata`, unused), can
be rebuilt from the graph, and is never the sole holder of a fact. A
retrieval answer always resolves to objects; index lag is visible (the
newest indexed object time is reported with every answer).

## 6. Owner decisions

1. **Budget router** (§4): approve free-first routing and daily caps before
   refilling paid accounts. Recommendation: yes; it is the 50× lever.
2. **Cache scope** (§1): per computer (default on this branch) or shared
   across computers. Shared adds ~1.2 points of savings and makes QA
   reruns free, but lets one computer infer from a fast answer that
   another asked the same query within 30 minutes.
3. **Disk on Node B**: 162 GB free, captures already write ~1.4 GB/day →
   roughly 3–4 months to full without search hits. Slice 2 adds
   ~0.05–0.1 GB/day. A storage plan (bigger disk, object storage, or
   retention) is needed independent of this design.
4. **Embedding hardware** (§5): small multilingual model on Node B as a
   batch job, or a separate (GPU) box for a 0.6B model.
5. **Provider terms for storing results** (§2): check each provider's
   plan for storage rights before snippets/page text are persisted;
   titles and URLs are kept either way.
6. **Per-run search budget** (§3.5): the number (suggest 12) and whether
   it is a soft pressure (recommended) or a hard cap.

## 7. Rollback

- Slice 1: `CHOIR_SEARCH_CACHE_TTL_SECONDS=0` on the gateway unit restores
  today's path without a code change; `git revert` of the slice commit
  removes it. No stored state is written (the cache is memory only).
- Slices 2–3: objects are additive kinds; the index is derived and can be
  dropped; prompts revert by commit.

## Owner ruling, about 15:10 UTC: retrieval over the Dolt object graph first

"Even before we move into embeddings and vector search, take our retrieval
and query patterns over the object graph as a serious research project. We
basically should table the vector embeddings until we reach diminishing
marginal returns with our Dolt versioned SQL object graph." Embeddings
(docs/research/embeddinggemma-2-cpu-2026-10-14.md) and the new server wait;
the next retrieval work is structured and full-text retrieval over the
object graph (claims, entities, events, captures, search-result pointers),
measured until its gains flatten.

First pass of that project, with a local benchmark at 1M captures / 10M
edges / 5M claims: [object-graph-retrieval-research-2026-10-14.md](object-graph-retrieval-research-2026-10-14.md).
