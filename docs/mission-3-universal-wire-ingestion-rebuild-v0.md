# Mission 3: Universal Wire Ingestion Rebuild

**Status:** planning  
**Date:** 2026-06-27  
**Spikes:** `docs/mission-3-spikes-2026-06-27.md` (all 6 complete, evidence gathered)  
**Predecessor:** `docs/mission-heresy-deletion-v1.md` (settled — deterministic scaffold deleted, processor dispatch wired)  
**Related:** O4 in `docs/mission-overnight-autoradio-platform-checklist-v0.md` (News/Universal Wire)

## Objective

Rebuild the Universal Wire ingestion pipeline from the clean substrate left by
the heresy deletion. The deterministic scaffold is already gone; the remaining
work is wiring in Qdrant semantic routing, Dolt object graph as canonical
state, and LLM-only synthesis, then expanding source coverage to new
protocols.

## What's Broken

The heresy deletion removed the deterministic scaffold and wired the processor
dispatch. Verified state of the pipeline:

1. **GDELT 15-min cadence coupled to universal polling** — the 15-min ticker
   in `cmd/sourcecycled/main.go` ticks ALL sources, not just GDELT. RSS and
   Telegram should poll faster. Decouple in 3a.
2. **~~`sourcegraph` heuristic clustering~~** — Verified: `sourcegraph` is
   provenance infrastructure (creates source entities, web captures, edges).
   `cycle.Engine.Cluster` is shallow vertical grouping by metadata field,
   not heuristic token-concept clustering. Both are real infrastructure.
3. **Processor key duplication** — `cycle.sourceProcessorKey` and
   `wire_synthesis.go:universalWireSourceProcessorKey` are identical logic
   duplicated because `runtime` cannot import `cycle` (import cycle:
   cycle → provider → runtime). Needs extraction to a leaf package. (3a)
4. **No real embeddings** — Qdrant pipeline used a deterministic hash embedder
   for tests. OllamaEmbedder exists (Spike 2) but is not wired in. (3b)
5. **No Dolt object graph** — objectgraph.Store uses SQLite. DoltStore exists
   (Spike 3) but is not wired in. (3b)
6. **No Qdrant on staging** — now fixed (Spike 1, deployed via CI).

## What Works (Keep)

- `sourcecycled` ingestion: fetches sources, stores items, dispatches to runtime
- Processor → Texture agent → LLM provider pipeline (Path C from heresy deletion)
- `cycle/synthesize.go` — original LLM synthesizer (user's prototype)
- O1-O3 settled code: objectgraph, qdrant schema/pipeline/projection, texture source graph
- Publication, reconciler debounce, platformd sync
- Source body text in Texture prompts (fixed in prior session)

## Spike Evidence (All Complete)

| Spike | Evidence | Key Finding |
|-------|----------|-------------|
| 1 (Qdrant node-b) | Deployed, healthz passing | Qdrant 1.18.1 running on node-b |
| 2 (Ollama Embedder) | 7 unit tests pass | 1024-dim embeddings via /api/embed |
| 3 (Dolt Store) | 13 tests pass | DoltStore implements Store, MySQL dialect works |
| 4 (Dolt branches) | 4 tests pass | Append-mostly: zero conflicts, 170µs merge |
| 4b (Dolt TCP branches) | 3 tests pass | Branch isolation over TCP confirmed. Working set is per-branch. |
| 5 (Qdrant routing) | 5 search tests pass | Routing threshold ~0.7862 on 25 headlines. Same-story avg 0.84, different-topic avg 0.43. |

## Mission Split

This is an umbrella mission split into 4 sub-missions, each with its own paradoc.
Sub-missions run sequentially unless noted.

### 3a: Cleanup & Component Wiring

**Paradoc:** `docs/mission-3a-cleanup-wiring-v0.md`  
**Confidence:** High — mechanical extraction + connecting proven components  
**Risk:** Low — wiring bugs, not architecture uncertainty  

Extract duplicated logic and wire spike components into the real pipeline:
- Extract 5 duplicated processor key functions from `cycle/ingestion_handoff.go` to a leaf package both `cycle` and `runtime` can import
- Replace deterministic hash embedder with `OllamaEmbedder` in Qdrant pipeline
- Replace `SQLiteStore` with `DoltStore` as objectgraph.Store
- Configure Qdrant client to point at node-b instance
- Confirm source body text reaches Texture agent (verify prior fix still works)
- Create real Qdrant collection with production schema on node-b

### 3b: New Ingestion Path (E2E)

**Paradoc:** `docs/mission-3b-ingestion-path-v0.md` (to be drafted after 3a)  
**Confidence:** Medium — real captures, dynamic threshold, synthesis quality, cadence change  
**Risk:** Medium — E2E flow may reveal integration issues  

Build the new ingestion path end-to-end on a single VM:
- Decouple GDELT 15-min cadence from universal polling loop — GDELT stays 15-min, RSS/Telegram get faster configurable intervals
- sourcecycled → Qdrant routing (semantic dedup) → Dolt object graph → Texture synthesis → publish
- Dynamic routing threshold (function of content volume, shard count, embedding model — not a magic constant)
- Calibration run with real captures to set initial threshold
- Acceptance: real source capture produces a source-grounded article on staging within minutes

### 3c: Distribution

**Paradoc:** `docs/mission-3c-distribution-v0.md` (to be drafted after 3b)  
**Confidence:** Medium — proven mechanics, unproven under real load  
**Risk:** Medium — merge contention, routing accuracy at scale  

Scale to multiple VMs:
- Branch-per-VM, merge VM micro-batch merge
- Qdrant routing for shard assignment (route to VM that owns the semantic cluster)
- 2-3 VMs running, captures distributed
- Acceptance: captures distributed across VMs, branches merged, articles from merged state

### 3d: Source Expansion

**Paradoc:** `docs/mission-3d-source-expansion-v0.md` (to be drafted after 3b)  
**Confidence:** Medium — per-source risk varies  
**Risk:** Low-medium per source, research-heavy  

Expand source coverage to new protocols:
- **MTProto** — Telegram's real API protocol (needs api_id + api_hash from my.telegram.org)
- **ATProto** — Bluesky's protocol (public firehose via WebSocket, or authenticated app)
- **RSSHub** — open-source RSS generator for Asian social media (Weibo, Zhihu, Bilibili, etc.)
- **More** — Hacker News, Reddit, Mastodon streaming, etc.
- Can parallel with 3c (independent of distribution work)

## Post-Mission 3 Portfolio (Future, Not Yet Scoped)

- Object graph as retrieval base for Texture/researchers (reduce/replace web search)
- Desktop/Wails auth fix + Qdrant/Ollama bundling
- O6 (Nucleus capsules), O7 (Choir Base), O8 (Autoradio)

## Architecture Principles

1. **Dolt is canonical, Qdrant is derived and rebuildable** — Qdrant can be rebuilt from Dolt at any time
2. **No deterministic synthesis** — LLM-only, always
3. **Source body text reaches the Texture agent** — no synthesis without real source content
4. **Each worker VM has its own Dolt branch** — working set is per-branch (Spike 4b finding)
5. **Merge is micro-batch, append-mostly** — zero conflicts on different PKs (Spike 4 finding)
6. **GDELT cadence decoupled** — GDELT polls every 15 min, other sources at their own cadence
7. **No heuristic token-concept clustering** — Qdrant semantic routing replaces it
8. **Routing threshold is dynamic** — function of content volume, shard count, embedding model; stored in config, not hardcoded
9. **Desktop/Wails is out of scope** — auth is broken, will be addressed after Mission 3

## Suggested Goal String

```text
Mission 3 is an umbrella mission with 4 sub-missions (3a-3d). Start with 3a (cleanup & component wiring). See docs/mission-3-universal-wire-ingestion-rebuild-v0.md for the full architecture and docs/mission-3a-cleanup-wiring-v0.md for the 3a paradoc. Spike evidence is in docs/mission-3-spikes-2026-06-27.md. All spikes are merged to main. Qdrant is deployed on node-b.
```
