# Search is duplicated less than we thought, and discarded more (2026-10-14)

Owner, 2026-10-14 about 14:30Z: "search is one of the bottlenecks and cost
challenges … I bet that we are not utilizing search so efficiently — of
those 4,000 searches, I bet a lot of them are duplicated or we're getting
duplicated results … efficiency before scaling … we should be building up
the object graph … from searching." This record measures that. Mutation
class of any fix: **red** (gateway provider calls). Problem first; fixes
come after this commit and reference it. Companion: the cooling record
(`search-providers-cooling-most-of-the-time-2026-10-14.md`), which covers
provider health and spent accounts. Design:
`docs/reports/search-efficiency-and-reuse-2026-10-14.md`.

## Evidence

All read-only. Raw extracts are outside the repo in
`~/.config/choir-qa/traces/search/` (gateway journal lines, parsed
requests, the paper's research run events). No secret was printed.

### Sources

- Gateway journal on Node B, `go-choir-gateway`, 2026-10-12 21:53Z to
  10-14 14:21Z (about 38 hours with traffic): 7,810
  `search request from autoputer … (query=… max_results=…)` lines and
  6,705 `answered M of N providers` lines.
- The Autopaper computer (`vm-1669020a…`, QA credential
  `autopaper-paper.json`): its 500 newest runs (10-14 10:20Z to 14:23Z),
  136 of them research runs, and all 17,835 events of those 136 runs
  (`GET /api/runs/<id>/events`). The runs' 1,233 `search_start` egress
  events reconcile with the gateway's 1,267 requests from that computer in
  the same window.

### Volume

| | |
|---|---|
| Searches, all computers | 10-12 (partial) 401, 10-13 3,183, 10-14 to 14:21Z 4,226 |
| Autopaper computer share | 5,222 of 7,810 (67%) |
| Peak hour | 971 (10-14 07Z); mean 206/hour over hours with traffic |
| Bursts | 1,530 single searches; 2,000+ seconds with 2–10 searches from one computer in the same second |
| `max_results` | 40 on every request (guest constant `deskSearchMaxResults`) |
| Provider calls per search | 1.95 on 10-13, 1.35 on 10-14 (low only because 7 of 9 providers are benched; the plane calls every eligible provider) |
| Research runs (paper, 4 h) | 136; web searches per run mean 9.1, median 6, max 71; 16 runs searched nothing |

### Are the searches duplicated?

Exactly duplicated: **few**.

| Normalization | Distinct | Repeats | Share |
|---|---|---|---|
| exact text | 7,533 | 277 | 3.5% |
| + case and whitespace | 7,533 | 277 | 3.5% |
| + quotes, `site:`/date operators, punctuation stripped | 7,510 | 300 | 3.8% |
| + order-free token set, stop words and years dropped | 7,464 | 346 | 4.4% |

Of the 300 normalized repeats, 177 came within 10 minutes of the previous
identical query, 255 within an hour, 298 within a day; 206 from the same
computer, 94 from another computer. Only 13 were in flight together (within
5 s). The largest repeat groups ("new AI model release October 2026" ×19,
"AI model release safety evaluation" ×15, …) are the same fixed prompt run
on six QA computers on 10-13 between 02:00 and 07:20Z, not the paper.

Nearly duplicated: **a third**. A search whose token set overlaps an
earlier search **from the same computer within 10 minutes** with Jaccard
≥ 0.5: 2,445 (31.3%); ≥ 0.7: 690 (8.8%). Widening the window to a day adds
little (34.7% / 10.4%), and overlap with *other* computers is small (≤ 432
at a day). Greedy clustering over all queries: 20.9% redundant at Jaccard
0.6, 34.7% at 0.5. So the redundancy is **one researcher reformulating**,
not many researchers asking the same thing.

What the reformulations are, read from the paper's queries (07:45–07:49Z
sample): a researcher hunting for one page it cannot name. The parlioSDR
story produced 12 searches in 3 minutes — `"raw 14-bit IQ" "320 Msps"
"2.4 GHz spectrum" README`, `"14-bit IQ" "live 2.4"`, `"parlioSDR" "raw
14-bit IQ"`, `site:github.com/radiolab81/parlioSDR README`, … — each a
quoted phrase from the article it already had. "Muse Wonder" was searched
three ways for its Hacker News item (`site:news.ycombinator.com`, "points
comments", "hacker news 663 376"), an item the paper ingests from its own
HN feed. Across all traffic, 18.3% of queries carry quotes, 5.1% carry two
or more quoted phrases, 11.4% use `site:`, and 74 hunt for HN items.

### Are the results duplicated?

**Not measurable from the gateway today**: it logs no result URL, title or
count, and keeps no result. Two partial views:

- The result URLs visible in the paper's cell outputs (only what a model
  chose to print): 474 distinct URLs from 73 printed search answers; 100 of
  them (21%) appear in more than one search's printed output.
- Within one search the guest already merges providers by URL
  (`copySearch`, `searchHitKey`), so cross-provider duplicates within a
  search are invisible to the model but still paid for once per provider.

### How much of what search returns is used?

| Measure (paper, 136 research runs, 4 h) | Value |
|---|---|
| Web searches | 1,233 |
| Results returned per search | up to 40 per answering provider (×1.35 today) ≈ 50 |
| Page reads (`read` egress) | 1,219, of which **329 failed (27%)**: 401/403/404 from publishers |
| Reads per search | 0.72 successful |
| Share of returned results read | about 1.5% (0.72 / ≈ 50) |
| Visible result URLs later read | 55 of 474 |
| Repeat reads of a URL already read | 90 of 542 parsed `choir.Read` calls (17%); 22 across runs |
| `read_capture` (reading an existing capture) | 696 |

Keenable — today the only provider answering every search — returns page
text, often several paragraphs of the article body, as the snippet. That
text reaches the worker's in-memory store and nothing else.

### Corpus first

122 of 136 research runs called `choir.SearchWireCorpus` (117 as their
first external act); 114 got at least one hit (93%), typically the story's
own source capture. Runs with a corpus hit still made **9.9 web searches**
on average (runs with a miss 7.3; runs that skipped the corpus 3.2). The
corpus check precedes web search; it does not replace any.

## Where search results go today (code, this worktree)

- **Gateway** (`internal/gateway/handlers.go` `HandleSearch`,
  `searchplane/router.go` `Router.Start`): every search is a fresh fan-out
  to every eligible provider. No result cache, no query cache, no
  in-flight de-duplication anywhere (`singleflight`/cache grep: none in the
  search path). Results are streamed to the caller and dropped. The only
  durable search state is provider health (`search-health.db`).
- **Guest** (`internal/agentcore/desk_search.go` `startSearch`,
  `internal/yaegikernel/search_store.go` `SearchStore`): results live in an
  in-memory store for one desk worker session (`s1`, `s2`, … per session),
  shared with no other session, run or computer. Run events record
  `desk.egress` `search_start`/`search_done` with counts only — no query,
  no URL. A result survives only if the model printed it, as unstructured
  text inside a `tool.result` cell output.
- **Reads** (`researchtools/read.go` `readPage`,
  `sourcegraph.WriteCaptures`): a successful read becomes a
  `choir.web_capture` object in corpusd's Store B (Dolt `og_objects`),
  bodies on host disk (`platform-artifacts/capture-bodies`, 1.4 GB,
  13,710 files, 13,389 written in the last 24 h). `Read` always fetches
  first; the capture identity (`canonical URL + doc hash`) detects a repeat
  only after the fetch, so a re-read costs a full fetch.
- **Corpus search** (`researchtools/corpus_search.go` `searchCaptures`,
  `store/texture.go` `SearchDocuments`): words matched against capture
  *titles*, outlet, host and labels of this computer's **newest 1,000
  captures** (metadata list from corpusd), plus a linear scan of Texture
  documents. No block text, no full-text index, no embeddings. At about
  4,500 captures a day the window covers roughly the last 5 hours.
- **Search results never become objects.** No query, provider, snippet or
  result URL reaches a capture, source item or object-graph row.
  `WebCaptureMetadata` has `embedding_model`/`embedding_version` fields,
  unused; `internal/qdrant` (Ollama embedder) is dormant, kept by owner
  ruling for a retrieval layer (dead-code census 2026-10-12).

## What this shows

The owner's hypothesis is half right, and the wrong half matters for the
fix:

1. **Exact duplicate searches are rare (3.5–4.4%).** A query cache alone
   saves little. It is still worth having (cheap, and the QA computers
   repeat fixed prompts), but it is not the lever.
2. **The paid unit is the provider call, not the search.** Each search
   calls every eligible provider; at full health that is up to 7 metered
   calls per search. The cooling record's budget-aware router (free first,
   metered on shortfall, daily caps) is the largest saving and is still an
   owner decision there.
3. **A third of searches are a researcher re-asking itself** within
   minutes, mostly hunting a known page by quoted phrases. Those are
   lexical lookups over text we already hold or just discarded: the
   article it is reading, the HN item we ingested, the 50 results of the
   previous search.
4. **We pay for ~50 results and keep none.** About 1.5% of returned
   results are read; the rest — titles, URLs, Keenable's page text — are
   dropped when the worker session ends. 27% of reads fail at the
   publisher while the search hit carried text for the same URL.
5. **Corpus first cannot substitute for search** because the corpus is
   titles of the newest 1,000 captures, with no search results in it and
   no body text indexed.

Substrate, not symptom: the search path has **no memory**. Every layer
(gateway, guest store, corpus) holds results only for the life of a call
or a session. Caching, budget routing and better prompts each help, but
the reuse the owner asks for needs search results and read pages to land
in one durable, indexed store — the object graph — with provenance.

## Failure modes any fix must not have

1. A cached answer is served after its freshness TTL (news goes stale).
2. A failed or empty search is cached and hides a later success.
3. One computer's search answers another computer's request under the
   default scope (owner privacy; timing side channel).
4. A cached or joined answer records provider outcomes (successes or
   strikes) it did not cause.
5. An in-flight join deadlocks, duplicates or loses arrivals, or the first
   caller leaving cancels the joined caller's search.
6. Cache memory grows without bound (Keenable entries carry page text).
7. The NDJSON stream changes shape incompatibly for the guest decoder.
8. Shared result slices race between concurrent searches.
9. The kill switch does not fully restore today's behaviour.
10. Normalization merges queries that differ in meaning (quotes,
    `site:`, operators).
11. Cache hits are indistinguishable from live searches in the log, which
    would corrupt the next measurement.
12. Persisted search hits flood the corpus window (the newest-1,000 bound)
    and push real captures out of corpus search.

## Heresy delta

- discovered: search path with no memory at any layer; corpus search over
  titles of the newest 1,000 captures only; search hits with page text
  discarded while 27% of reads fail; `Read` re-fetches before it can
  detect a repeat; no result-level logging, so result duplication is
  unmeasurable.
- introduced: none (documentation).
- repaired: none yet.

## Fix on the branch (after this record; not pushed, not deployed)

- 035592faa `red(searchplane)`: design §1. `searchplane.Cache` (result
  cache + in-flight join, per-computer scope, TTL 30 min / 10 min for
  time-sensitive queries, 64 MiB, kill switch
  `CHOIR_SEARCH_CACHE_TTL_SECONDS=0`); additive `cached_at` on the stream's
  start line; the search log line gains
  `served=miss|hit|joined|off results=N unique=M seen=K`. Tests:
  `searchplane/cache_test.go` (failure modes 1–6, 8–10),
  `gateway/search_cache_test.go` (3, 7, 9, 11, seen counter).
  `go test -race ./internal/gateway/... ./internal/search/` pass;
  `check-decode-roots.sh`, `check-v1-writer-purity.sh` pass.

Repaired once deployed: repeat searches within the TTL (measured 2.6% of
searches per computer, 3.8% shared) and identical in-flight searches; the
journal then measures result duplication (`seen=`). Not repaired: the
fan-out to every provider (budget router, owner decision), reformulations
(design §3), discarding results (design §2), corpus search over titles of
the newest 1,000 captures (design §3). Failure mode 12 belongs to §2.

Behaviour change to note: with the cache on, a caller that disconnects no
longer cancels its search's provider calls (a joiner may need them); the
calls run to their own timeout and their outcomes are the provider's own.

After deploy, re-measure from the journal: share of `served=hit|joined`,
and `seen/unique` per search (result duplication per computer per day).
