# News System State — Current Code and Improvement Plan

Date: 2026-06-06  
Owner: repo-local authoring state  
Scope: Source ingestion + news workflow in this repo only  
Reviewed against: current worktree code at `fab6b25b`, plus the locally imported dirty-main doc

This document is a current-state note and code-review reconciliation, not an
execution mission. The improvement ideas below are intentionally rough; future
missions should re-read the code and define their own scoped acceptance
criteria before implementation.

## Product Constraint: Not An Oracle

Choir Global Wire should not present itself as an oracle that declares "what is
true" from above. The product target is multiperspectival coverage:

- represent claims, counterclaims, uncertainty, source standing, and missing
  evidence explicitly;
- show how claim ranges differ by source, region, institution, ideology,
  market position, domain role, and time;
- preserve the history of how claims and confidence changed across issues;
- distinguish reported facts, interpretations, projections, contested claims,
  corrections, and editorial synthesis;
- make citations and provenance inspectable enough that a reader, agent, or
  radio listener can ask "who says this, on what evidence, and what changed?"
- avoid hidden "on background" inputs. Every material source or context packet
  that shaped a story should be represented in provenance, even when only a
  small number of lead citations are shown inline.

The newspaper projection can still have voice and editorial judgment, but its
authority should come from visible evidence topology and honest uncertainty,
not from flattening the world into one confident summary.

Source visibility should be weighted rather than binary:

- **lead citations** are the small set of sources that directly support the
  visible sentence or claim;
- **supporting context** includes additional sources that shaped framing,
  chronology, entities, priors, or alternative interpretations;
- **contrary/qualifying context** includes sources that dispute, narrow, or
  complicate the lead claim;
- **ambient corpus context** can be summarized as a bounded manifest with
  source counts, source classes, recency, and selection rationale rather than
  flooding the article with hundreds of equal-weight references.

The UI can keep the article readable, but the data model should preserve enough
context for drilldown, audit, correction, and future radio traversal.

## Current state snapshot

News work in this repo is **partially implemented** and is best described as:

- A **running V0 ingestion daemon** in `cmd/sourcecycled/main.go`.
- A **source schema + storage layer** in `internal/sources` and `internal/cycle`.
- A **podcast content app** in the Choir desktop, not a full general-purpose news app.
- No single user-facing "news surface" or newspaper app. Research agents can
  use `source_search` when the runtime is configured with Source Service, but
  that is an agent/tool integration over the internal service API, not a
  product news subscription surface.

### What exists in code today

- `sourcecycled` is implemented as a single long-running daemon that:
  - loads source registry from config (`configs/sources.json`),
  - initializes persistent SQLite storage (`sourcecycled.db` path by default),
  - runs an ingestion loop with `time.NewTicker(15 * time.Minute)`,
  - exposes a narrow HTTP API under `/internal/source-service/...`.
- Source adapters are present for:
  - RSS/Atom via `internal/sources/rss.go`,
  - Telegram web preview scraping via `internal/sources/telegram.go`,
  - GDELT last-update ingest via `internal/sources/gdelt.go`.
- Storage has tables for sources, fetches, items, cycles, cycle events, and issues in `internal/cycle/storage.go`.
- API surface currently includes:
  - `/internal/source-service/health`
  - `/internal/source-service/search`
  - `/internal/source-service/items/<id>`
- Runtime researcher tooling can expose `source_search` from
  `internal/runtime/tools_research.go` when `SOURCE_SERVICE_BASE_URL`,
  `SOURCE_SERVICE_URL`, or `SOURCECYCLED_API_URL` is configured. Node B config
  currently sets `SOURCE_SERVICE_BASE_URL=http://127.0.0.1:8787`.
- VText has code paths that recognize `source_service_item:<id>` references and
  preserve them as source entities, but there is no dedicated News app or
  first-class front-page/newsroom UI.
- Frontend has podcast-specific ingestion and playback paths in
  `frontend/src/lib/PodcastApp.svelte` and podcast routes, which are
  RSS-library oriented and not a unified news experience.

## What this means operationally

- **Ingestion is real but not user-facing**: the backend collects and stores source artifacts, but the user/product path is not a live news stream yet.
- **Polling works, but cadence is uniform**: per-source cadence exists in config/types (`poll_interval_seconds`) but current main loop does not yet schedule per-source frequencies.
- **Event stream exists conceptually, not as subscription transport**: events are recorded in storage, but no WebSocket/SSE endpoint is exposed in the running daemon.
- **Integrated as researcher search, not as subscription**: `source_search`
  can query the Source Service ledger for researcher turns when configured, but
  there is no native "subscribe to source/vertical/author" path for agents or
  researchers to receive new items as a stream.
- **Source Service should become the basis, not the ceiling, of research**:
  when a researcher hits a relevant Source Service story/item/cluster, the
  normal research path should also run web search or another live external
  expansion path. Sourcecycled gives Choir a durable owned source ledger and
  retrieval prior; web search finds what the ledger missed, checks freshness,
  and broadens the claim range.
- **Not productized as a standalone API contract in v0**: no CLI subcommands and no public API surface (`/v1/...`) planned in mission docs are currently visible.

## Code-review findings

### 1) Source scheduler / fetch behavior

- `configs/sources.json` supports per-source intervals and rate metadata, but `run` loop is fixed 15-minute global scheduling.
- Need:
  - per-source due-time calculation,
  - failure-aware backoff,
  - missed-tick recovery after downtime,
  - optional source-level run policies (frequency + jitter + max-attempts).

### 2) Deduplication and restart semantics

- `internal/cycle.Engine` keeps a process-local `Seen` map and dedupes by
  item id/hash only inside the running daemon.
- Item IDs are stable and storage uses `items.id` as a primary key, so SQLite
  can collapse duplicate item rows on write.
- After daemon restart, however, the engine forgets `Seen` and can re-report
  previously seen items as "new" within a cycle before storage upsert behavior
  collapses them. Issue synthesis and cycle counts should not be treated as
  durable "new item" proof until restart-aware dedupe is explicit.

Need:

- durable "already observed" checks before synthesis;
- source/fetch/item accounting that distinguishes fetched, observed, inserted,
  updated, duplicate, and synthesized items;
- tests that cover restart and repeated poll behavior.

### 3) Ingestion quality and trust

- Deduplication is present but not clearly durable across all failure/restart paths in the current flow.
- Fetch ledger exists, but many adapters still need tighter:
  - status/error semantics,
  - policy enforcement,
  - parser robustness,
  - source identity consistency.
- Synthesis failures currently degrade gracefully but can leave missing issue content without explicit manifest guarantee.

### 4) Distribution and subscriptions

- No push path to researchers/agents today.
- No explicit source/vertical subscription API with delivery preferences.
- No delivery hooks (webhook/SSE/WebSocket) for downstream clients.

### 5) Product surfaces

- No dedicated “News” app in Choir that consumes sourcecycled artifacts.
- No sourceledger -> VText source-entity importer path in active UI. The lower
  level source ref recognition path exists for `source_service_item:<id>`, so
  this is a product workflow gap, not a total representation gap.
- Existing content surface is podcast-centric and not a unified newsroom stream.

## Rough improvement directions

Treat these as candidate directions, not accepted scope.

### 1) Baseline correctness pass (smallest safe increment)

- Make per-source scheduling explicit and enforced.
- Persist and query last-run timing robustly from storage metadata.
- Add deterministic next-run calculation, including source downtime catch-up behavior.
- Ensure deduplication keys and stable IDs are validated across process restarts.

### 2) Product API and event contract

- Expose a public `v1` API surface for source lifecycle and result access:
  - `GET /v1/sources`, `GET /v1/cycles`, `GET /v1/items`, `GET /v1/issues`
  - `GET /v1/events` + `GET /v1/events?cursor=...`
  - `POST /v1/cycles/run` and source-level control endpoints.
- Add WebSocket or SSE endpoint for event tailing and cycle progress (subscription model).
- Keep internal API endpoints for local/legacy compatibility, but do not block product work on them.

### 3) Researcher/agent consumption path

- Add a first query model for researcher work:
  - query by vertical, source, timeframe, evidence level, and policy flags,
  - include stable source/item references in every result row,
  - add a short “subscription spec” object for alerting.
- Create a minimal evidence packet format for agent handoff (items + source metadata + fetch provenance).
- Define the combined retrieval policy:
  - start with Source Service when it has relevant ledger coverage,
  - call web search after a relevant source hit unless the task explicitly
    forbids external lookup or the source item is already the only permitted
    evidence,
  - checkpoint which claims came from the source ledger versus live web search,
  - record missing-ledger findings as candidates for future source onboarding.

### 4) Build a News app (after API exists)

- Add a dedicated news surface in `frontend` that consumes only public product API contracts.
- Show live ticker/list updates from event stream.
- Support:
  - source/vertical filters,
  - recency and freshness controls,
  - open-source trace of each card to issue/item details.
- Make the default story object multiperspectival:
  - claim set,
  - source positions,
  - confidence/gap/correction state,
  - timeline of claim changes,
  - weighted source/context manifest,
  - links to underlying source items and VText story artifacts.
- Keep it intentionally thin; avoid duplicating parser logic in frontend.

### 5) Future readiness (same track, later)

- Add optional webhook adapters and on-demand source polling.
- Add event-driven ingestion for sources that can push.
- Add export formats for issue manifests and trace bundles to avoid hidden data dependence.
- Add stronger anti-abuse and policy guardrails (TOS flags, source standing, retry windows).

### 6) Newsletter feature + email agent integration

### Current status

- There is no newsletter pipeline yet. The current user path stops at source ingestion and basic API/search.
- The repo already has an Email app and mailbox/maild runtime, plus runtime guidance that email send flows should remain reviewable (draft-first) rather than automatic send.
- Personalization infrastructure for cross-feature ranking/preferences is not yet fully shared by news, so a full user-level newsletter experience would still be inconsistent without broader platform work.

### Proposed integration model

- Treat newsletter as a **news-to-email adaptation job**, not a new ingestion path.
- Reuse existing issue or cluster outputs as deterministic input:
  - source item bundle → curated story set → issue manifest → email draft payload.
- Use the email agent as the delivery adapter:
  1. `NewsDigestJob` creates a deterministic digest artifact (title, summary, ordered items, source links, source IDs).
  2. Dispatch a reviewable draft request to the email agent/runtime (same approval/safety model used for normal email drafting).
  3. Email agent finalizes plain-text and HTML variants from the same source manifest.
  4. Send is initiated only through the existing user-approval path.
- Keep dedupe/correlation strict:
  - include `source_item_id` and `issue_id` in email metadata,
  - persist digest hash/fingerprint,
  - record newsletter send attempts and suppress duplicates per recipient/time window.

Newsletter should remain a projection of the same source/story artifacts used
by the News app and researchers. It should not fork its own ranking,
provenance, or ingestion path.

### 7) Continuous retrieval / Autoradio substrate

After source search, web-search expansion, story manifests, and newsletter
projection are reliable, the same retrieval loop becomes the basis of
Autoradio:

```text
source ledger hit
-> live web expansion/check
-> weighted story/source manifest
-> VText/story artifact
-> queue item
-> TTS narration and/or podcast/video/audio playback
-> user interruption or continuation
-> next retrieval hop
```

The important shift is from one-shot search to continuous background retrieval.
Researchers, News, newsletters, and Autoradio should share the same source
identity, claim state, and weighted provenance model.

### Personalization roadmap (uncertainty-aware)

- **Phase 0 (today):** no behavior personalization, only profile filters.
  - opt-in topic list per owner,
  - opt-in frequency (`daily`, `every 4h`, `manual`),
  - source allow/deny list,
  - evidence policy threshold (`high | medium | anything`).
- **Phase 1 (once platform preferences are consistent):** optional learned ranking.
  - signal sources by user explicit actions (opens, stars, skip),
  - apply recency/freshness and vertical weight,
  - add source standing and risk flags into sort score.
- **Phase 2 (later):** contextual personalization.
  - role/space-aware defaults (legal/client/news),
  - entity-level filters,
  - conflict avoidance and duplicate suppression across digest and VText/alerts.

### Open design question (explicit)

- Keep newsletter generation in `sourcecycled` (tight coupling) or move it to a separate platform service after the news ledger stabilizes?
- Recommended now: keep **sourceledger ownership in sourcecycled**, move **delivery/orchestration** to a platform-owned newsletter worker once platform-level user preferences and event telemetry are consistent.

## Recommended implementation sequence (pragmatic 3 milestones)

1. **Milestone A: Make scheduling + contracts real**
   - Per-source cadence, stable resume, event stream endpoint, API v1 read endpoints.
2. **Milestone B: Build consumption and delivery**
   - Subscription queries, stream tailing, research/agent packet schema, and
     Source Service -> web-search expansion policy.
3. **Milestone C: Add News product surface**
   - Thin frontend stream app bound to the same API/contracts; no duplicate ingestion logic.
4. **Milestone D: Add newsletter + email agent first-class path**
   - Add digest job, email-agent draft handoff, and manual-send workflow behind explicit owner controls.
   - Add profile-gated delivery preferences before any content ranking.
5. **Milestone E: Add continuous retrieval queue for Autoradio**
   - Convert story/source manifests into queue items with TTS, podcast/video
     playback refs, traversal edges, and interruption state.

## Current status decision

This codebase is **not currently shipping a full news product**, but it has a valid and useful V0 ingestion foundation. The immediate objective should be to turn it from “daemon-only” into a **source ledger + event stream + subscription + news consumer** platform with clean boundaries so researchers and app surfaces can consume it without touching internal-only endpoints.

## File anchors used in this assessment

- [cmd/sourcecycled/main.go](/Users/wiz/.codex/worktrees/5b24/go-choir/cmd/sourcecycled/main.go)
- [internal/cycle/cycle.go](/Users/wiz/.codex/worktrees/5b24/go-choir/internal/cycle/cycle.go)
- [internal/cycle/storage.go](/Users/wiz/.codex/worktrees/5b24/go-choir/internal/cycle/storage.go)
- [internal/runtime/tools_research.go](/Users/wiz/.codex/worktrees/5b24/go-choir/internal/runtime/tools_research.go)
- [internal/runtime/vtext_media_sources.go](/Users/wiz/.codex/worktrees/5b24/go-choir/internal/runtime/vtext_media_sources.go)
- [internal/sourceapi/types.go](/Users/wiz/.codex/worktrees/5b24/go-choir/internal/sourceapi/types.go)
- [internal/sources/types.go](/Users/wiz/.codex/worktrees/5b24/go-choir/internal/sources/types.go)
- [internal/sources/rss.go](/Users/wiz/.codex/worktrees/5b24/go-choir/internal/sources/rss.go)
- [internal/sources/telegram.go](/Users/wiz/.codex/worktrees/5b24/go-choir/internal/sources/telegram.go)
- [internal/sources/gdelt.go](/Users/wiz/.codex/worktrees/5b24/go-choir/internal/sources/gdelt.go)
- [configs/sources.json](/Users/wiz/.codex/worktrees/5b24/go-choir/configs/sources.json)
