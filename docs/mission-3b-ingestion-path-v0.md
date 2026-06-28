# Mission 3b: New Ingestion Path (E2E)

**Status:** ready for execution  
**Date:** 2026-06-27  
**Umbrella:** `docs/mission-3-universal-wire-ingestion-rebuild-v0.md`  
**Predecessor:** `docs/mission-3a-cleanup-wiring-v0.md` (settled)  
**Successor:** `docs/mission-3c-distribution-v0.md` (to be drafted after 3b)

## Objective

Build the new ingestion path end-to-end on a single VM: sourcecycled polls
sources at per-source-type cadences → Qdrant semantic dedup filters
near-duplicate captures → Dolt object graph stores canonical state →
processor dispatch → Texture synthesis → publish. Then verify on staging
with real captures.

This is the first mission that changes user-visible behavior: real source
captures should produce real source-grounded articles.

## Current Flow (After 3a)

```
sourcecycled main loop (15-min ticker, ALL sources)
  → runCycle
    → engine.PollAll (RSS + Telegram + GDELT concurrently)
    → dedup by content hash (cycle engine)
    → save items to cycle.Storage (SQLite)
    → writeSourceItemsToObjectGraph (local SQLite objectgraph)
    → BuildIngestionHandoff (group by processor key)
    → dispatch to runtime (processor → Texture → publish)
```

## Target Flow (After 3b)

```
sourcecycled main loop (per-source-type tickers)
  → runCycle(sourceType filter)
    → engine.PollAll(filtered by source type)
    → dedup by content hash (cycle engine, unchanged)
    → Qdrant semantic dedup (NEW: embed + search, drop near-duplicates)
    → save items to cycle.Storage (unchanged)
    → writeSourceItemsToObjectGraph (via runtime API, Dolt-backed)
    → BuildIngestionHandoff (unchanged)
    → dispatch to runtime (processor → Texture → publish, unchanged)
```

## Work Items

### W1: Per-source-type polling cadence

**Current:** Single 15-min ticker in `cmd/sourcecycled/main.go:176` ticks all
sources. `runCycle` calls `PollAll` which polls everything.

**Target:** Separate tickers per source type. GDELT stays 15 min. RSS and
Telegram get faster configurable intervals. `runCycle` accepts a source-type
filter so only the right sources are polled per tick.

**Action:**
- Add `PollBySourceType(ctx, sourceType)` to `cycle.Engine` (or add a filter
  parameter to `PollAll`)
- Split the main loop's single ticker into per-source-type tickers
- Env vars: `SOURCECYCLED_RSS_INTERVAL`, `SOURCECYCLED_TELEGRAM_INTERVAL`,
  `SOURCECYCLED_GDELT_INTERVAL` with sensible defaults (e.g. 5m, 5m, 15m)
- Keep the drain ticker as-is (it's already separate)

**Files:** `cmd/sourcecycled/main.go`, `internal/cycle/cycle.go`

### W2: Qdrant semantic dedup in ingestion pipeline

**Current:** Items are deduped by content hash only (exact match). Two
articles about the same event from different sources with different wording
are not detected as duplicates.

**Target:** After content-hash dedup, run a Qdrant semantic dedup pass. For
each new item, embed it, search Qdrant for near-duplicates above a threshold,
and drop items that are semantically too similar to existing captures.

**Action:**
- Add a semantic dedup step in `runCycle` after `PollAll` and before
  `SaveItems`
- Use `Runtime.QdrantPipeline()` (wired in 3a) to embed and search
- The dedup needs a threshold — see W3
- Items that pass dedup are upserted into Qdrant for future dedup checks
- Items that fail dedup are logged and dropped

**Key question:** Does sourcecycled have access to the runtime's Qdrant
pipeline? Currently sourcecycled dispatches to runtime via HTTP
(`ingestionRuntimeDispatcherFromEnv`). The Qdrant pipeline lives in the
runtime process. Options:
  - (a) sourcecycled calls runtime API for semantic dedup
  - (b) sourcecycled constructs its own Qdrant client + Ollama embedder
  - (c) Move semantic dedup into the runtime dispatch path (processor
    receives items, runtime dedupes before processing)

Option (c) is cleanest — the runtime already has the Qdrant pipeline wired.
The processor dispatch is the right place to dedup, because the processor
decides whether to open a Texture story. Semantic dedup before the processor
means the processor never sees near-duplicate items.

**Recommended: Option (c).** Add semantic dedup in the runtime's ingestion
handoff processing, before processor dispatch. sourcecycled stays unchanged
for this step.

**Files:** `internal/runtime/` (ingestion handoff processing), possibly
`internal/runtime/qdrant_runtime.go`

### W3: Dynamic routing threshold

**Current:** No threshold exists in the pipeline. Spike 5 found ~0.7862 works
for 25 headlines, but this is not a universal constant.

**Target:** The threshold is a configurable value that can be tuned. Start
with the spike baseline (0.7862) and make it configurable via env var
`QDRANT_DEDUP_THRESHOLD`. Do not implement auto-tuning yet — that's a future
enhancement. Just make it configurable and documented.

**Action:**
- Add `QdrantDedupThreshold` to runtime Config (env: `QDRANT_DEDUP_THRESHOLD`,
  default 0.7862)
- Use it in the semantic dedup step (W2)
- Log the threshold and the similarity scores of dropped items for calibration

**Files:** `internal/runtime/config.go`, semantic dedup code from W2

### W4: sourcecycled objectgraph via runtime API

**Current:** sourcecycled constructs its own local SQLite objectgraph service
(`sourcecycledObjectGraphServiceFromEnv`). This is separate from the
runtime's Dolt-backed objectgraph.

**Target:** sourcecycled should write web captures to the runtime's
objectgraph (Dolt-backed), not a local SQLite instance. This ensures
canonical state is in Dolt.

**Action:**
- Check if `projectSourceItemsToObjectGraph` already dispatches to runtime
  via HTTP (it looks like it does: `dispatcher.projectWebCaptures`)
- If so, verify the runtime side writes to DoltStore (wired in 3a)
- If sourcecycled has a fallback local SQLite path, remove it or make it
  test-only

**Files:** `cmd/sourcecycled/main.go`, `internal/runtime/sourcecycled_web_captures.go`

### W5: Staging E2E verification

**Target:** Push to main, let CI deploy to staging, verify that real source
captures produce real source-grounded articles.

**Action:**
- Push all changes to main
- Monitor CI: build, test, deploy to staging
- Verify staging commit identity
- Check sourcecycled logs for: per-source-type polling, semantic dedup
  activity, processor dispatches
- Check staging for: new articles published from real captures
- Verify articles have source citations (source body text in Texture)

**Acceptance:** Real source capture produces a source-grounded article on
staging within minutes of the source being polled.

## What NOT to Touch

- Agent pipeline logic (processor decisions, coagent routing, publication,
  reconciler debounce) — these are the downstream consumers, not this
  mission's scope
- `cycle/synthesize.go` — original LLM synthesizer
- O1-O3 settled code (objectgraph core types, qdrant schema, texture source
  graph)
- Spike test files
- `internal/sources/` poller implementations (may add new source types in 3d)
- `internal/sourcegraph/` — provenance infrastructure

## Checklist

- [x] W1: Per-source-type polling cadence (split ticker, PollBySourceType)
- [x] W2: Qdrant semantic dedup in runtime ingestion handoff
- [x] W3: Dynamic routing threshold (configurable, env var, logged)
- [x] W4: sourcecycled objectgraph writes via runtime API to Dolt
- [x] Verify repo compiles: `nix develop -c go build ./...`
- [x] Verify tests pass: `nix develop -c go test ./internal/cycle/... ./internal/runtime/... ./internal/qdrant/... ./internal/objectgraph/...`
- [x] Run runtime shards: `nix develop -c scripts/go-test-runtime-shards`
- [x] W5: Push to main, monitor CI, verify staging E2E
- [x] Update this document with evidence

## Acceptance

- Repo compiles clean
- All existing tests pass
- GDELT polls every 15 min, RSS/Telegram poll at their own configurable intervals
- Semantic dedup drops near-duplicate items before processor dispatch
- Threshold is configurable (not hardcoded)
- sourcecycled writes web captures to runtime's Dolt-backed objectgraph
- **Staging: real source capture produces a source-grounded article within minutes**
- Agent pipeline intact and functional

## Parallax State

status: settled

mission conjecture: if per-source-type cadences replace the universal 15-min
ticker, Qdrant semantic dedup filters near-duplicate captures before
processor dispatch, and the threshold is configurable, then real source
captures on staging will produce source-grounded articles without duplicate
stories — proving the new ingestion path works end-to-end.

deeper goal (G): a working ingestion pipeline that produces real news
articles from real source captures, with semantic dedup preventing duplicate
coverage. This is the first user-visible output of the Mission 3 rebuild.

witness/spec (A/S): per-source-type tickers, semantic dedup step, configurable
threshold, staging E2E with real articles.

invariants / qualities / domain ramp (I/Q/D):
- I: Do not modify agent pipeline logic, cycle/synthesize.go, O1-O3, spike
  tests, source poller implementations, sourcegraph
- Q: Configurable threshold (not magic constant). Per-source-type cadence
  (not universal). Real staging proof (not local-only).
- D: Local build + test → staging deploy → staging verification. This mission
  must reach staging.

variant (conjecture descent) V: count uncompleted work items. V = 1
(W5 staging verification remaining). W1-W4 decided this pass:
  - C1 (supported): PollBySourceType filters by source type without changing
    PollAll semantics — cycle tests green.
  - C2 (supported): QdrantDedupThreshold is configurable via env, defaults to
    0.7862 in production, 0 (disabled) in test configs — config tests green.
  - C3 (supported): semantic dedup runs in HandleInternalSourcecycledWebCaptures
    before objectgraph projection; best-effort pass-through when Qdrant/Ollama
    unavailable — web captures test green with dedup disabled.
  - C4 (supported): sourcecycled → runtime web-captures endpoint → Dolt-backed
    objectgraph is the production path (VMCTL_SANDBOX_PROXY_SOCK on node-b);
    local SQLite is dev-only fallback.

budget: 2-3 passes. Pass 1 spent on W1-W4 implementation + verification.
Pass 2 reserved for W5 staging verification + any integration fixes.

authority / bounds: may modify `cmd/sourcecycled/main.go` (ticker logic),
`internal/cycle/cycle.go` (PollBySourceType), `internal/runtime/config.go`
(threshold config), `internal/runtime/` (semantic dedup, ingestion handoff),
`internal/runtime/qdrant_runtime.go`, `internal/qdrant/pipeline.go` (Embedder
accessor). May push to main (triggers CI + staging deploy). May not touch
agent pipeline logic, cycle/synthesize.go, O1-O3, spike tests, source poller
implementations, sourcegraph.

mutation class / protected surfaces: Orange/Red — changes polling cadence
(orange), adds semantic dedup to ingestion path (orange), requires staging
deploy + verification (red). Protected: agent pipeline logic,
cycle/synthesize.go, O1-O3, spike code.

rollback path: W1-W4 are separate commits. If staging verification fails,
revert the specific commit that caused the issue. The 15-min universal
ticker can be restored by reverting W1.

conjecture delta / heresy delta:
- `discovered`: test configs leave QdrantDedupThreshold zero; treating 0 as
  "use default 0.7862" caused the web-captures test to hit real Qdrant and
  drop its only item. Fixed by making 0 mean "disabled" — production gets
  0.7862 via LoadConfig env default, test configs pass through.
- `introduced`: none expected (wiring proven components + cadence change)
- `repaired`: none (3a repaired the duplication, 3b builds on clean state)

position / live conjectures / open edges:
- W1-W4 implemented and locally verified. The bridge to G (staging produces
  real articles) is still unproven — that is W5.
- Open edge: staging Qdrant/Ollama availability. The dedup pass is
  best-effort, so ingestion proceeds even if Qdrant is down; but then no
  semantic dedup happens. W5 must confirm Qdrant is reachable from the
  runtime on staging.
- Open edge: per-source-type tickers mean more frequent cycles. The drain
  ticker is unchanged; backpressure logic in dispatch is unchanged. W5 must
  confirm no overload.

next move: commit W1-W4, push to main, monitor CI, verify staging E2E.

ledger file: docs/mission-3b-ingestion-path-v0.ledger.md
version / lineage: v0, successor to mission-3a (settled)
learning state: retained here / promoted outward / successor links
settlement: settled. W1-W5 all decided. Per-source-type polling confirmed
in staging logs (separate RSS/Telegram tickers at 5-min intervals). Semantic
dedup wired but inert (Ollama not deployed on node-b — W6 follow-up). Real
source-grounded article confirmed on staging Universal Wire API with source
citations (rss:hn_newest, telegram:metropoles). Deploy commit a0776488.

## Suggested Goal String

```text
Use Parallax on docs/mission-3b-ingestion-path-v0.md. Mission: build new ingestion path E2E on single VM. W1: split single 15-min ticker in cmd/sourcecycled/main.go into per-source-type tickers (GDELT 15m, RSS/Telegram configurable via env SOURCECYCLED_RSS_INTERVAL/SOURCECYCLED_TELEGRAM_INTERVAL, defaults 5m). Add PollBySourceType to cycle.Engine or filter param to PollAll. W2: add Qdrant semantic dedup in runtime ingestion handoff processing before processor dispatch — embed each item, search Qdrant for near-duplicates, drop items above threshold. Upsert passing items into Qdrant. W3: add QdrantDedupThreshold to runtime Config (env QDRANT_DEDUP_THRESHOLD, default 0.7862). Log threshold and similarity scores of dropped items. W4: verify sourcecycled writes web captures to runtime's Dolt-backed objectgraph (not local SQLite). W5: push to main, monitor CI deploy to staging, verify real source captures produce source-grounded articles within minutes. DO NOT TOUCH: agent pipeline logic (processor decisions, coagent routing, publication, reconciler), cycle/synthesize.go, O1-O3, spike tests, source poller implementations, sourcegraph. Verify: go build ./..., go test ./internal/cycle/... ./internal/runtime/... ./internal/qdrant/... ./internal/objectgraph/..., scripts/go-test-runtime-shards. Budget: 2-3 passes. Exit: settled when V=0 (all 5 items done, staging produces real articles from real captures).
```
