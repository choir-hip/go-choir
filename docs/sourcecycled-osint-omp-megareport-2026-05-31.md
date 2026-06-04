# Sourcecycled, OSINT Integration, And OMP Session Mega Report

**Date:** 2026-05-31  
**Primary context source:** OMP / oh-my-pi session from 2026-05-30  
**Primary branch reviewed:** `origin/sourcecycled`  
**Primary branch commit:** `0c04a33 feat(wire): add Choir Global Wire V0 -- autonomous multilingual news ingestion & synthesis`  
**Primary repo:** `go-choir`  
**Purpose:** preserve the valuable OMP context on `sourcecycled`, the Global
Wire demo, source ingestion architecture, OSINT/data-source learnings, and the
integration plan for Choir.

## 0. Location Of The Recovered OMP Context

The relevant OMP state lives under:

```text
/Users/wiz/.omp
```

The latest relevant `go-choir` OMP session is:

```text
/Users/wiz/.omp/agent/sessions/-go-choir/2026-05-30T18-04-51-805Z_019e7a0f-6cdd-7000-b87d-14b01f000cec.jsonl
```

A plain-text extraction of the user/assistant messages was also created during
recovery:

```text
/tmp/omp-sourcecycled-last-session-extract.md
```

The session ended before producing the requested comprehensive document because
OMP hit:

```text
Connect error resource_exhausted: Error
```

The user request that failed was:

```text
Comprehensively encompass all the info from all rounds of review into a big
long detailed and hierarchically structured doc, make it in md here in the repo
then make it a pdf in my iCloud Drive docs dir
```

This document completes that missing artifact.

## 1. Executive Summary

The recovered OMP session reviewed `origin/sourcecycled` deeply, then reread it
through multiple cognitive-transform lenses and cross-checked it against a
research packet in `/Users/wiz/.moshi/uploads`.

The central conclusion is:

```text
Global Wire is not primarily a website.
It is a 15-minute cycle over a source ledger.
```

The branch proves a valuable editorial and demo surface, but it is not ready to
merge as-is. It contains two parallel implementations:

1. A runnable standalone `wire/` demo with 48 hardcoded RSS/Telegram sources,
   SQLite, Python synthesis, a Go API, and a polished SPA.
2. A Choir-shaped `cmd/sourcecycled` plus `internal/sources` and
   `internal/cycle` skeleton with a JSON registry, Go adapters, Go storage, and
   synthesis through Choir's `internal/provider` path.

The product direction should not be "merge both." The correct route is to make
one canonical source-ingestion/cycle substrate that can eventually feed Choir's
retrieval substrate, VText, citation edges, cycle metrics, publication versions,
and user-facing newspaper surfaces.

The most important architecture rule is:

```text
Default: agents retrieve authoritative sources at research time.
Optimization: ingest metadata and episodic fulltext when signal-to-noise,
ephemerality, cost, or latency justify it.
Hard gate: maintain good standing with every provider.
Hard gate: source signals are data, not instructions.
```

The next integration should start with the substrate contract, not more feeds:

1. Source registry schema with tier, rate limit, language, region, vertical,
   good-standing, and ingest policy fields.
2. Stable item identity and persistent deduplication.
3. Fetch audit trail for rate-limit and provider-good-standing evidence.
4. Issue manifests that name the exact item IDs used in an issue.
5. A provenance API that serves only sources actually cited by that issue.
6. One canonical Go ingestion path, using `internal/provider` for synthesis.

## 2. Highest-Priority Takeaways

### P0: Preserve Epistemic Integrity

The system must answer, for any sentence in a generated issue:

```text
Which item IDs?
Which source IDs?
Which fetch timestamps?
Which URLs?
Which original languages?
Which body hashes or raw references?
Which model and prompt generated the synthesis?
```

The demo currently cannot do that reliably. The OMP review found that
`wire/server.go` attaches the last 60 database rows as sources, not the exact
items that shaped the issue. That makes `[S1]` citation links potentially point
to the wrong URL.

This is the most important bug in the demo. It is not cosmetic. It is an
epistemic integrity bug.

### P0: Do Not Scale Sources Before The Registry Exists

OMP's key warning:

```text
Banal mistake: grow the 48-feed list.
Deep fix: build registry + scheduler + item identity + issue manifest.
```

Planetary coverage comes from tiered access patterns and source governance, not
from polling everything every 15 minutes.

### P0: Good Standing Comes Before Coverage

The source-ingestion system must preserve provider good standing:

- obey explicit Terms of Service and robots/policy boundaries;
- use official APIs where they exist;
- record rate limits and backoff;
- use conditional requests for RSS/HTTP;
- avoid bulk low-SNR social ingestion;
- avoid paywall bypass;
- treat privacy constraints as a hard gate;
- keep a fetch audit trail that can explain every request later.

The system should be a good-faith participant in the open information ecosystem,
not an indiscriminate scraper.

### P0: One Canonical Ingestion Path

`sourcecycled` currently has two stacks:

```text
wire/ standalone demo
cmd/sourcecycled + internal/* Choir skeleton
```

The merge path should consolidate into one canonical Go ingestion and cycle
path. The `wire/static` publication surface can be kept as a projector/demo
surface, but the data path should not remain duplicated.

### P1: Treat The Newspaper As A Projection

The newspaper page is not the durable object. The durable object is:

```text
cycle -> source items -> fetches -> clusters -> issue manifest -> publication
```

The SPA, markdown issue, VText draft, or public archive are projections of the
source ledger and issue manifest.

### P1: Multilingual And OSINT Work Needs A Source-Ledger Foundation

The OSINT/global-source research points toward:

- GDELT as global event metadata backbone;
- curated Telegram as high-signal conflict/ground-truth feed where allowed;
- RSS/Atom as the low-friction traditional source base;
- arXiv and scientific feeds for AI/science;
- WHO/ProMED and public-health feeds;
- Polymarket/Kalshi as prediction-market signals;
- Bluesky Jetstream, Wikimedia EventStreams, CertStream, AIS/transport streams
  as future stream workers;
- RSSHub for some China/Korea/India routes if self-hosted and governed.

But every one of those should enter through a registry with tier, source
contract, rate policy, and provenance semantics.

## 3. Source Packet And Evidence Reviewed

### 3.1 OMP Session

Relevant session:

```text
/Users/wiz/.omp/agent/sessions/-go-choir/2026-05-30T18-04-51-805Z_019e7a0f-6cdd-7000-b87d-14b01f000cec.jsonl
```

User's initial instruction to OMP:

```text
we have a new branch, sourcecycled. review it deeply. then use cognitive
transforms to review it again from multiple perspectives. later tonight we will
move to integrate these learning and more sources into choir itself
```

User then supplied screenshots and research notes, with the instruction that the
real work should go deep on sources and data architecture now that the demo had
proved the surface.

### 3.2 Research Notes In `.moshi/uploads`

The research packet is:

```text
/Users/wiz/.moshi/uploads/Choir Source Ingestion Manifesto.md
/Users/wiz/.moshi/uploads/Comprehensive Research Report: Global Source Ingestion System for Choir.md
/Users/wiz/.moshi/uploads/Global South & Multilingual Source Ingestion: Expanding Choir's Planetary Coverage.md
/Users/wiz/.moshi/uploads/MISSION-AUTOMATIC-NEWSPAPER.md: The Choir Global Wire.md
/Users/wiz/.moshi/uploads/Technical Audit: Environmental, Sub-National Government, and Uncovered Public Signal Categories.md
/Users/wiz/.moshi/uploads/Technical Audit: Maritime, Financial, Sports, Legal, and Real Estate Public Data Access.md
/Users/wiz/.moshi/uploads/Technical Data Source Audit: Ingestion Constraints, Rate Limits, & Protocols for Choir.md
```

### 3.3 Branch Shape

`origin/sourcecycled` contains one sourcecycled commit:

```text
0c04a33 feat(wire): add Choir Global Wire V0 -- autonomous multilingual news ingestion & synthesis
```

Diff from its base:

```text
21 files changed
3360 insertions
```

Files added:

```text
cmd/sourcecycled/main.go
configs/sources.json
docs/MISSION-AUTOMATIC-NEWSPAPER.md
internal/cycle/cycle.go
internal/cycle/storage.go
internal/cycle/synthesize.go
internal/sources/gdelt.go
internal/sources/rss.go
internal/sources/telegram.go
internal/sources/types.go
wire/.gitignore
wire/README.md
wire/cycle_runner.py
wire/go.mod
wire/go.sum
wire/ingest.go
wire/server.go
wire/static/about.html
wire/static/index.html
wire/static/pretext.js
wire/synthesize.py
```

Current worktree note: the integrated daemon subset is already staged in this
repo:

```text
A  cmd/sourcecycled/main.go
A  configs/sources.json
A  internal/cycle/cycle.go
A  internal/cycle/storage.go
A  internal/cycle/synthesize.go
A  internal/sources/gdelt.go
A  internal/sources/rss.go
A  internal/sources/telegram.go
A  internal/sources/types.go
```

The standalone `wire/` demo files and `docs/MISSION-AUTOMATIC-NEWSPAPER.md`
remain on `origin/sourcecycled` but are not staged in this worktree.

## 4. What The Sourcecycled Branch Actually Contains

OMP's first major finding was that the branch is not one implementation. It is
two implementations plus a mission doc.

### 4.1 Stack A: Standalone `wire/`

Location:

```text
wire/
```

Role:

```text
48-source poll -> SQLite vanguard.db -> Python LLM synthesis -> Go API -> SPA
```

Maturity:

```text
runnable proof of concept
```

Architecture as reviewed:

```text
wire/ingest.go
  -> polls hardcoded RSS and Telegram sources
  -> writes SQLite vanguard.db

wire/synthesize.py
  -> reads recent rows
  -> calls OpenAI directly
  -> writes issue markdown

wire/server.go
  -> serves API and static newspaper UI

wire/static/*
  -> publication shell, archive, Pretext behavior, citation display
```

Strengths:

- End-to-end demo loop exists.
- 48 RSS/Telegram sources are hardcoded and visible.
- Publication surface is polished for V0.
- Story structure uses the right editorial frame: Signal, Context, Contested,
  Watch.
- Multilingual intent appears in prompts and source labels.
- Static + Pretext + markdown is a plausible publication shell.
- It proves the feel of a 15-minute newspaper rather than a raw feed.

Critical gaps:

- Importance x Rarity is claimed but not computed.
- Citations can point at wrong sources because the server attaches last N rows,
  not the items used.
- Telegram IDs are unstable if generated from `time.Now().UnixNano()`.
- Paths are hardcoded for a deploy island.
- There are no tests or CI.
- Ingest is sequential despite concurrency claims.
- Direct OpenAI path bypasses Choir gateway/provider observability.
- Prompt truncation limits source body depth.
- No structured issue output or issue manifest.
- Telegram and premium RSS use require a stronger policy/good-standing layer.

### 4.2 Stack B: `cmd/sourcecycled` And `internal/*`

Locations:

```text
cmd/sourcecycled
configs/sources.json
internal/sources
internal/cycle
```

Role:

```text
JSON registry -> adapters -> cycle engine -> internal/provider synthesis -> SQLite
```

Maturity:

```text
Choir-shaped skeleton
```

Strengths:

- Uses Choir's Go codebase instead of a separate `wire` module.
- Uses `internal/provider` / Fireworks-style integration rather than direct
  OpenAI.
- RSS adapter has conditional request support (`ETag`, `If-Modified-Since`).
- Telegram adapter uses `data-post` IDs, a better identity basis than time.
- GDELT adapter begins a proper metadata stream path.
- Storage has the beginning of an items/issues schema.
- `docs/MISSION-AUTOMATIC-NEWSPAPER.md` is a serious integration brief.

Critical gaps:

- At OMP review time, `go build ./cmd/sourcecycled` failed because root
  `go.mod` lacked `github.com/mmcdole/gofeed`.
- `configs/sources.json` included Polymarket but no Polymarket adapter.
- Only five sources existed in the registry.
- Deduplication was in memory, so duplicates return after restart.
- `poll_interval_seconds` was ignored.
- Clustering was just vertical bucketing, not event clustering.
- `SaveIssue` stored token usage as zero.
- GDELT deferred `rc.Close()` inside a loop, risking file-handle leakage.
- RSS item identity could collide when GUID is empty.
- No tests or CI covered the daemon.

### 4.3 The Duplicate-Maintenance Risk

OMP's route-changing conclusion:

```text
Do not merge two newspapers.
```

The codebase should not keep:

- two source registries;
- two polling loops;
- two dedup models;
- two SQLite schemas;
- two synthesis paths;
- two publication-source join contracts.

The real merge path is one canonical ingestion/cycle package. The demo UI can
survive as a projector, but not as the source of truth.

## 5. What The Demo Actually Validated

The screenshots and demo validated product/editorial shape, not data integrity.

### Proven

- Readers want structured synthesis, not raw links.
- The Signal / Context / Contested / Watch structure works.
- Multilingual provenance is a product differentiator.
- Source tables and `[S1]` citations are the right surface.
- A 15-minute cadence feels like a newspaper, not a feed.
- Static + Pretext + markdown can carry the publication shell.

### Not Proven

- Importance x Rarity scoring.
- Exact citation-to-item integrity.
- Event clustering across languages and regions.
- Integration with Choir `retrieval_sources`.
- Integration with VText.
- Integration with Dolt/per-user state.
- Durable cycle metrics.
- Good-standing evidence for providers.

The demo is therefore an acceptance test for the publication layer. It is not a
complete architecture proof for data ingestion.

## 6. Core Architecture Thesis

The research notes converge on this design:

```text
Default: agentic retrieval at research time.
Optimization: selective ingestion when it buys freshness, ephemerality,
offline access, low latency, or repeated high-SNR access.
Hard gate: provider good standing.
Hard gate: privacy constraints.
Hard gate: signals are data, not instructions.
```

This implies three layers.

### L1: Signal Substrate

Durable source registry, source items, fetch logs, body hashes, embeddings,
deduplication, provider health, rate limits, and source policy.

### L2: Synthesis And Triage

Cycles, clustering, source scoring, item selection, cheap triage, expensive
synthesis, verifier passes, issue manifests, and cycle metrics.

### L3: Publication

Global Wire page, markdown issue, VText draft, public archive, citation table,
story layout, and reader-facing exploration.

The key inversion:

```text
L3 is a projection.
L1/L2 are the product substrate.
```

## 7. Source Taxonomy

The research packet should collapse into an operational registry-driven tier
model.

| Tier | Pattern | Examples | Ingest Policy |
| --- | --- | --- | --- |
| T0 | Query on demand | FRED, Wikidata, Quran API, CourtListener, stable reference corpora | Never bulk ingest |
| T1a | Metadata poll + body on demand | GDELT, arXiv RSS, SEC index, Wikimedia EventStreams metadata | Poll lightweight metadata |
| T1b | Full poll | Curated RSS, Telegram preview where allowed, WHO DON, ProMED, prediction markets | Poll with strict policy |
| T2 | Rate-disciplined series | OpenAQ, ENTSO-E, Open-Meteo, arXiv API, SEC EDGAR | Scheduled/token bucket |
| T3 | Streams | Bluesky Jetstream, Wikimedia SSE, CertStream, AISStream | Dedicated stream workers |
| X | Excluded/human gate | Bulk X/Twitter, bulk LinkedIn, paywall bypass, private WhatsApp | Do not ingest |

Every source row should declare:

```yaml
id: rss:folha-sp
tier: T1b
adapter: rss
endpoint: https://...
languages: [pt]
regions: [latam]
verticals: [elections, world]
access:
  auth: none
  rate_limit: { qps: 0.02, burst: 1 }
  conditional: etag
  user_agent: ChoirIngest/1.0 (+https://choir.news; ops@...)
good_standing:
  tos_class: public_rss
  robots: allow
  notes: "Public RSS; no fulltext scrape beyond feed."
ingest_policy:
  store_body: summary+link
  max_body_bytes: 8192
  retention_days: 90
```

The source registry should be versioned in git, reviewable as code, and
eventually inspectable by agents.

## 8. Canonical Data Model

OMP recommended upgrading from demo SQLite into a durable schema that can
deform into Choir's retrieval/VText/publication substrate.

### 8.1 `sources`

Purpose:

```text
versioned source registry plus runtime health/cache
```

Core fields:

```text
source_id
tier
adapter
endpoint
display_name
languages
regions
verticals
rate_limit
conditional_request_policy
tos_class
robots_policy
auth_policy
store_body_policy
retention_days
health_state
last_polled_at
last_success_at
backoff_until
```

### 8.2 `items`

Purpose:

```text
immutable signal records
```

Stable identity rule:

```text
id = sha256(source_id || canonical_original_id)
```

Core fields:

```text
id
source_id
original_id
canonical_uri
title
summary
body_ref
language_detected
languages_claimed
published_at
fetched_at
content_hash
raw_ref
vertical_tags
embedding_ref
fetch_status
http_cache_headers
```

Load-bearing rule:

```text
Do not use time.Now or UnixNano as item identity.
```

Stable examples:

- RSS GUID, falling back to canonical link/content hash;
- Telegram `data-post`;
- GDELT `GLOBALEVENTID`;
- Polymarket market/event ID;
- arXiv ID;
- SEC accession number;
- Wikimedia revision ID.

### 8.3 `fetches`

Purpose:

```text
good-standing audit trail
```

Every HTTP/source interaction gets a row:

```text
fetch_id
source_id
url
requested_at
status_code
bytes
latency_ms
etag_sent
last_modified_sent
etag_received
last_modified_received
rate_limit_policy
backoff_until
error_class
user_agent
```

This lets Choir explain its behavior if a publisher or provider asks.

### 8.4 `clusters`

Purpose:

```text
event candidates before expensive synthesis
```

Core fields:

```text
cluster_id
created_at
updated_at
centroid_embedding
item_ids
vertical
importance
rarity
status
```

Importance should be computed from deterministic features first:

- source tier weight;
- source credibility/curation weight;
- vertical match;
- recency;
- GDELT magnitude/tone proxies where available;
- number of independent sources;
- geographic relevance.

Rarity should include:

- language rarity in current corpus;
- geographic rarity;
- source uniqueness;
- low duplicate count;
- undercovered vertical boost.

### 8.5 `issues`

Purpose:

```text
synthesis artifact with manifest
```

Core fields:

```text
issue_id
cycle_id
created_at
content_markdown
source_item_ids
cluster_ids
llm_model
tokens_in
tokens_out
prompt_hash
publication_status
```

Hard rule:

```text
The issue API must join sources by issue.source_item_ids only.
```

This fixes the demo's "last 60 rows" citation bug.

### 8.6 `cycles`

Purpose:

```text
observable 15-minute run ledger
```

Core fields:

```text
cycle_id
started_at
completed_at
status
items_polled
items_new
items_duplicate
clusters_created
issues_created
tokens_in
tokens_out
errors_by_source
provider_cost
```

### 8.7 Choir Bridge

| Sourcecycled Object | Choir Target |
| --- | --- |
| `sources` | source registry, retrieval source catalog, agent source tools |
| `items` | `retrieval_sources` / content substrate rows |
| `fetches` | trace/evidence/good-standing audit |
| `clusters` | cycle triage substrate / MissionBag candidates |
| `issues` | VText revision, publication version, public wire issue |
| `source_item_ids` | citation edges / span references |
| `cycles` | `internal/events`, Trace/evidence, run acceptance |

## 9. Adapter Map

The adapter interface should be one simple contract:

```go
type Adapter interface {
    Poll(ctx context.Context, source Source, state State) ([]RawItem, error)
    FetchBody(ctx context.Context, item ItemRef) ([]byte, error)
}
```

### P0 Adapters

| Adapter | Why It Matters | Notes |
| --- | --- | --- |
| `rss` | broad curated source base | ETag, Last-Modified, adaptive polling, good User-Agent |
| `telegram_web` | conflict/OSINT and regional ground truth | public preview only, `data-post`, rate discipline, policy review |
| `gdelt_gkg` | global event metadata backbone | 15-minute files, metadata first, body on demand |

### P1 Adapters

| Adapter | Why It Matters | Notes |
| --- | --- | --- |
| `polymarket` / `kalshi` | prediction-market signal | unauthenticated market APIs where permitted |
| `who_don` / `promed_rss` | public-health early warning | official/RSS sources |
| `arxiv_rss` | AI/science vertical | metadata first, full text on demand |
| `sec_edgar` | filings and regulatory events | explicit SEC rate limits |

### P2 Adapters

| Adapter | Why It Matters | Notes |
| --- | --- | --- |
| `rsshub` | CN/KR/IN and platform-specific feeds | self-hosted, Redis cache, strict route policy |
| `openaq` | air quality/public health | rate-limited API |
| `open_meteo` | weather/climate events | useful environmental substrate |
| `entsoe` / energy APIs | grid/energy shock signals | credential/policy dependent |
| `courts/legal` | legal/regulatory vertical | query-on-demand unless high-value metadata |

### P3 Stream Workers

| Stream | Why It Matters | Notes |
| --- | --- | --- |
| Bluesky Jetstream | AI/culture/social early signals | filtered WebSocket worker |
| Wikimedia EventStreams | edits/current-events signal | SSE, bot User-Agent, rate policy |
| CertStream | internet infrastructure/security | dedicated stream worker |
| AIS/maritime streams | shipping/logistics | often auth or paid; metadata policy needed |

## 10. Scheduler And Good-Standing Requirements

OMP emphasized that the scheduler is a first-class component, not a loop inside
`RunCycle`.

Required scheduler features:

- per-source token buckets;
- global 15-minute cycle budget;
- adaptive polling based on source health and freshness;
- ETag/Last-Modified support;
- durable backoff on 429/5xx;
- source health state;
- explicit provider outage errors;
- bounded request concurrency;
- differentiated policies for APIs, RSS, public HTML, and streams.

Examples from the research packet:

- SEC EDGAR: explicit public rate discipline.
- arXiv API: low request cadence.
- RSS/Atom: conditional requests and adaptive polling.
- Telegram preview: slow per-channel polling and careful policy review.
- RSSHub: self-hosted with cache and route governance.
- Wikimedia: identified bot User-Agent and published API norms.

## 11. Multilingual And Global South Pipeline

The Global South / multilingual research contributes the following pipeline:

```text
raw text
  -> language detection
  -> item language metadata
  -> multilingual embedding
  -> cross-lingual event clustering
  -> translation at synthesis time
  -> English issue with original-language citations preserved
```

Important rule:

```text
Cluster before expensive synthesis.
```

Otherwise the system pays frontier-model tokens to rediscover that three
languages describe the same event.

### Regional Paths From The Research Packet

Asia:

- China-related routes may need RSSHub and explicit route governance.
- East/South Asian aggregators can provide curated public RSS and news metadata.
- Regional language coverage should be recorded at the source registry level.

Latin America:

- Messaging infrastructure is dominant; public Telegram may matter.
- Collaborative investigative journalism networks are high-SNR.
- Spanish/Portuguese source coverage should be a first-class vertical.

Africa:

- Civic tech and open data repositories matter.
- Localized news syndication is likely high value.
- Coverage should prioritize signal-rich sources over generic broad scraping.

Messenger commons:

- Telegram is valuable for conflict and real-time public channels.
- WhatsApp should remain excluded/human-gated unless policy and consent are
  explicit.

## 12. Domain-Specific Source Learnings

### Traditional News And RSS

Use curated RSS as the first broad source base. Avoid indiscriminate news
aggregation. RSS sources need:

- stable source IDs;
- conditional requests;
- feed health;
- language/region metadata;
- vertical tags;
- body policy;
- paywall policy.

### GDELT

GDELT is foundational because it gives global, multilingual, 15-minute event
metadata and source links. The correct v0 use is metadata ingestion plus
on-demand article retrieval, not bulk fulltext scraping.

### Government, Economic, And Institutional Data

Use official public APIs where possible:

- FRED;
- World Bank;
- OECD;
- BLS;
- SEC EDGAR;
- CourtListener and legal APIs;
- municipal open-data portals.

Most stable statistical sources should be T0 query-on-demand unless there is a
clear value in storing time-series deltas.

### Academic And Scientific Sources

arXiv, Semantic Scholar, OpenAlex, and related metadata sources are strong for
AI/science verticals. Store metadata first; fetch full paper content on demand.

### Social And Decentralized Signals

Bulk social ingestion is dangerous. Use filtered, high-SNR public channels and
stream workers only when policy and value justify them.

Potential future streams:

- Bluesky Jetstream;
- Wikimedia EventStreams;
- CertStream.

Excluded or human-gated:

- bulk X/Twitter;
- bulk LinkedIn;
- general Facebook/TikTok/Instagram feeds;
- private WhatsApp;
- paywall bypass.

### Climate, Health, Logistics, And Environment

Useful categories:

- WHO DON and ProMED for health;
- OpenAQ for air quality;
- Open-Meteo and NOAA for weather;
- energy grid APIs where accessible;
- maritime AIS only with clear policy and source contracts;
- transport GTFS realtime;
- wildfire/flood/open hazard feeds.

## 13. OSINT Integration Principles

The branch and research packet use "OSINT" in the high-signal public-source
sense, not as a license to collect everything.

OSINT should mean:

- public, authorized, or official sources;
- careful provenance;
- exact source URLs and timestamps;
- clear source policy;
- language and region metadata;
- rate discipline;
- citation integrity;
- no private or non-consensual aggregation;
- no bypassing access controls.

For conflict or politically sensitive sources:

- prefer public channels and official reports where possible;
- preserve original-language source references;
- track source confidence and source type;
- keep raw item boundaries intact;
- distinguish observed claims from synthesized claims;
- preserve a route for human review when source safety is uncertain.

## 14. Importance x Rarity

OMP found the branch claims Importance x Rarity but does not implement it.

This should not be LLM theater. It should be computed at cluster/item level
first, then optionally refined by a cheap model.

### Importance Features

- source tier;
- source curation weight;
- vertical match;
- recency;
- source count;
- event magnitude;
- GDELT tone/magnitude proxy;
- affected population or institutional relevance;
- known user/editorial vertical weight.

### Rarity Features

- language rarity in current cycle;
- region rarity;
- source uniqueness;
- low duplicate count;
- undercovered vertical boost;
- first appearance of a topic;
- divergence from Western/English source dominance.

### API Requirement

If the UI displays Importance, Rarity, or Priority, the backend must expose the
computed values and explain their basis. Otherwise the UI should not claim them.

## 15. Cognitive-Transform Review From OMP

OMP applied multiple transforms. The important output was route-changing, not
decorative.

### Boundary Transform

Finding:

```text
wire/ is a deployable island.
internal/* is the Choir boundary.
```

Changed plan:

```text
Use one canonical internal ingest/cycle substrate.
Keep wire/static only as a projection or example surface.
```

### Depth Extraction

Banal version:

```text
48 feeds + LLM + pretty page.
```

Deep version:

```text
idempotent ingest + stable source IDs + issue manifest + provenance.
```

Changed verifier:

```text
An issue is valid only if its API returns sources by source_item_ids used for
that issue, not by recent database rows.
```

### Good-Standing / Authority Transform

Finding:

```text
Highest risk is source policy and provider good standing, not LLM quality.
```

Changed scope:

```text
Source registry fields for tos_class, robots/policy, rate_limit, auth, and
ingest body policy are required before production ingestion.
```

### Unit Transform

Finding:

```text
The unit of shipping is one cycle producing one promotable artifact, not a
3360-line branch merge.
```

Changed plan:

```text
Unify ingest, fix provenance API, use one synthesis path, then prove one cycle.
```

### Homotopy Transform

Finding:

```text
wire/ can deform into Choir only if outputs become typed records.
```

Changed route:

```text
items -> content substrate
issues -> VText drafts/publication versions
source_item_ids -> citation edges
cycles -> internal events / trace evidence
```

## 16. Merge Readiness Verdict

OMP's verdict:

| Dimension | Rating | Reason |
| --- | --- | --- |
| Vision / docs | A- | Mission direction is strong |
| Wire PoC | B | Demoable, but provenance and scoring gaps |
| Choir integration code | C+ | Right shape, incomplete build/registry/adapters |
| Tests / CI | F | No meaningful coverage |
| Epistemic integrity | D | Wrong sources on API, unstable Telegram IDs |
| Merge to `main` as-is | Not recommended | Consolidate and fix manifest first |

Suggested post-consolidation PR title from OMP:

```text
feat(wire): unified global wire cycle with provenanced issues (V0)
```

## 17. Recommended Consolidation Plan

### Phase 0: Decide The Canonical Boundary

Choose the Choir-shaped Go path as canonical:

```text
cmd/sourcecycled
internal/sources
internal/cycle
configs/sources.json or configs/sources/*.json
```

Keep from `wire/`:

- static UI / publication shell;
- curated source list seed;
- editorial prompt structure;
- demo archive/reader ideas.

Retire or demote from `wire/`:

- duplicate Go ingestor;
- Python synthesis path;
- OpenAI direct credentials path;
- in-memory or accidental provenance joins;
- hardcoded deployment paths.

### Phase 1: Make It Build And Tell The Truth

1. Add required dependencies such as `github.com/mmcdole/gofeed` to root
   `go.mod`.
2. Implement or remove Polymarket adapter from registry.
3. Expand the registry from 5 sources toward the demo list only after schema
   fields exist.
4. Implement persistent deduplication.
5. Respect `poll_interval_seconds`.
6. Add tests for RSS identity fallback, Telegram `data-post`, GDELT resource
   closing, storage, and issue manifest joins.

### Phase 2: Fix Provenance Contract

1. Add `source_item_ids` to issues.
2. Store cluster IDs and source item IDs at synthesis time.
3. Serve issue sources by manifest join only.
4. Add regression for `[S1]` source integrity.
5. Store model, prompt hash, tokens, and cycle ID.

### Phase 3: Implement Scheduler And Good-Standing Ledger

1. Add fetch table.
2. Add per-source health/backoff.
3. Add token bucket policy.
4. Add global cycle budgets.
5. Record User-Agent and conditional request headers.
6. Add source review checklist.

### Phase 4: Add First Serious Source Spine

1. RSS curated set.
2. GDELT 15-minute metadata worker.
3. Telegram public channel set with policy and slow stagger.
4. arXiv RSS for AI.
5. WHO DON / ProMED for health.
6. Prediction markets once adapter exists.

### Phase 5: Choir Integration

1. Map source items to retrieval/content substrate.
2. Emit `internal/events` per cycle.
3. Promote issue to VText draft or publication version.
4. Preserve citation edges/span refs.
5. Expose source registry tool to researcher agents for T0 query-on-demand.

### Phase 6: Planetary Expansion

1. Add RSSHub for selected routes if self-hosted and governed.
2. Add stream workers for Bluesky/Wikimedia/CertStream.
3. Add domain-specific adapters only when vertical missions need them.
4. Grow source list by tier and measured signal, not by raw availability.

## 18. Proposed Immediate Artifact

OMP recommended creating:

```text
docs/source-ingestion-architecture-v1.md
```

Contents should include:

- tier definitions T0/T1a/T1b/T2/T3/X;
- source registry schema;
- item/fetch/cluster/issue/cycle schemas;
- adapter interface;
- scheduler rules;
- good-standing checklist;
- mapping to `retrieval_sources`, VText, cycles, publication versions, and
  citation edges;
- acceptance tests for citation integrity and stable item IDs.

This report can serve as the source packet for that architecture doc, but the
architecture doc should be shorter and normative. This report is intentionally
descriptive and comprehensive.

## 19. Immediate Engineering Risks To Check Before Coding

### Risk 1: Current Branch Base Is Old

`origin/sourcecycled` is based on `ba49f0f`, while current `main` has moved
significantly after the VText/Super Console/Zot work. Any integration must
cherry-pick or port sourcecycled files carefully, not merge the branch wholesale.

### Risk 2: Current Worktree Has Staged Sourcecycled Subset

The staged daemon subset should be preserved. Avoid clobbering it with a merge.

### Risk 3: `wire/` Demo Is Not In Current Staging

The standalone UI/demo files are only on `origin/sourcecycled`. If they are
needed, bring them in intentionally as demo assets or an example, not as a
second production path.

### Risk 4: Provider Path Must Use Choir Gateway/Provider

Avoid adding another direct OpenAI/Python credential path. Use Choir's provider
plane and record model/cost/evidence.

### Risk 5: Good-Standing Review Is Product-Critical

Do not ship large-scale scraping before source policy is represented in data.

## 20. Acceptance Criteria For V1 Sourcecycled

V1 should not be accepted because it has 48 feeds. It should be accepted when:

1. `go test ./cmd/sourcecycled ./internal/cycle ./internal/sources` passes.
2. One command runs a dry cycle with deterministic fixture sources.
3. RSS IDs are stable when GUID is empty.
4. Telegram IDs use source-stable post IDs.
5. GDELT closes resources correctly.
6. Dedup survives process restart.
7. `poll_interval_seconds` is respected.
8. Every fetch writes an audit row.
9. A generated issue stores exact `source_item_ids`.
10. The source API returns only the issue's `source_item_ids`.
11. Importance/Rarity are either computed or absent from UI/API claims.
12. Source registry rows contain tier, policy, rate, language, region, vertical,
    and retention fields.
13. The synthesis path uses `internal/provider`.
14. The cycle emits metrics.
15. The output can become a VText draft or publication candidate without losing
    citation provenance.

## 21. Bottom Line

The recovered OMP session is valuable because it names the category error:

```text
Do not treat Global Wire as a website.
Treat it as a cycle over a source ledger.
```

The `sourcecycled` branch is useful raw material:

- `wire/` proves the publication feel;
- `internal/sources` and `internal/cycle` point toward the right integration;
- the research packet defines the global source strategy;
- the OMP review identifies the exact integrity and architecture gaps.

The next correct move is not more scraping and not a wholesale branch merge.
The next correct move is a source-ingestion architecture contract and a narrow
V1 implementation that proves stable items, fetch audit, issue manifests,
citation integrity, and one canonical Choir-native cycle.
