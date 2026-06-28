# Mission 3: Universal Wire Ingestion Rebuild

**Status:** in progress — 3a settled, 3b settled, 3c (actor migration) next  
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
   Telegram should poll faster. **Decouple in 3b.**
2. **~~`sourcegraph` heuristic clustering~~** — Verified: `sourcegraph` is
   provenance infrastructure (creates source entities, web captures, edges).
   `cycle.Engine.Cluster` is shallow vertical grouping by metadata field,
   not heuristic token-concept clustering. Both are real infrastructure.
3. **~~Processor key duplication~~** — **Fixed in 3a.** 5 functions extracted
   to `internal/wire/processorkey/` leaf package. Both `cycle` and `runtime`
   import it. `universalWire*` duplicates deleted.
4. **~~No real embeddings~~** — **Fixed in 3a.** `OllamaEmbedder` wired into
   `Runtime.QdrantPipeline()` with configurable URL + model (env:
   `OLLAMA_URL`, `OLLAMA_EMBEDDING_MODEL`).
5. **~~No Dolt object graph~~** — **Fixed in 3a.** `DoltStore` replaces
   `SQLiteStore` as `objectgraph.Store`. Workspace path derived from store
   path. `Service.Config.SQLite` renamed to `Durable`.
6. **~~No Qdrant on staging~~** — Fixed (Spike 1, deployed via CI).
7. **No Qdrant semantic dedup in pipeline** — Qdrant pipeline is wired but
   not used in the ingestion path. Items are deduped by content hash only.
   **Add semantic dedup in 3b.**
8. **No production Qdrant collection** — `EnsureProductionCollection` exists
   (3a) but hasn't been called on staging startup. **Wire in 3b.**

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

### 3a: Cleanup & Component Wiring ✅ Settled

**Paradoc:** `docs/mission-3a-cleanup-wiring-v0.md` (settled, V=0)  
**Commit:** `e76aa068`  
**Confidence:** High — mechanical extraction + connecting proven components  
**Risk:** Low — wiring bugs, not architecture uncertainty  

Completed:
- Extracted 5 duplicated processor key functions to `internal/wire/processorkey/` leaf package
- Wired `OllamaEmbedder` into `Runtime.QdrantPipeline()` (lazy init, configurable via env)
- Replaced `SQLiteStore` with `DoltStore` as `objectgraph.Store` (renamed `Config.SQLite` → `Config.Durable`)
- Configured Qdrant client for node-b (env: `QDRANT_URL`, default `http://127.0.0.1:6333`)
- Added `EnsureProductionCollection` for idempotent collection creation (1024-dim, Cosine, payload indexes)
- Added `CreatePayloadIndex` to `API` interface (removed type assertion hack)
- Verified source body text reaches Texture agent — all tests pass

### 3b: New Ingestion Path (E2E) ✅ Settled

**Paradoc:** `docs/mission-3b-ingestion-path-v0.md` (settled, V=0)  
**Deploy commit:** `a0776488`  
**Confidence:** Medium — real captures, dynamic threshold, synthesis quality, cadence change  
**Risk:** Medium — E2E flow may reveal integration issues, staging deploy required  

Completed:
- Per-source-type polling cadences (RSS 5m, Telegram 5m, GDELT 15m, env-configurable)
- Qdrant semantic dedup wired in runtime before processor dispatch (best-effort pass-through)
- Configurable threshold (env: `QDRANT_DEDUP_THRESHOLD`, default 0.7862, 0=disabled)
- sourcecycled writes web captures to runtime's Dolt-backed objectgraph via HTTP API
- Staging E2E: real article produced with source citations (rss:hn_newest, telegram:metropoles)
- Semantic dedup inert (Ollama not deployed on node-b — W6 follow-up)
- Fixed flake.nix internalDirs (missing internal/qdrant, internal/wire/processorkey)
- Fixed stale sourcecycled tests (3a Durable rename + 3b runCycle signature)

Follow-up items (not blocking):
- W6: Deploy Ollama on node-b to activate semantic dedup
- W7: Add explicit QDRANT_URL/QDRANT_DEDUP_THRESHOLD env vars to sandbox service config
- W8: Improve sandbox runtime logging for processor runs and dedup activity

### 3c: Actor Runtime Migration

**Paradoc:** `docs/mission-3c-actor-runtime-migration-v0.md`  
**Plan doc:** `docs/actor-runtime-migration-and-agents-md-revision-2026-06-27.md`  
**Confidence:** High — replacement exists and is verified, work is extraction + rewiring  
**Risk:** Medium — State 6 (wire pipeline migration) is high-entanglement; 3b changes add to entanglement  
**Priority:** **Key priority.** The actor runtime (`internal/actor/`, 326 lines, 1 mutex) was built and verified on 2026-06-11 but never wired in. The wire pipeline runs on the borked `internal/runtime/` (3797 lines, 15 mutexes) — the exact substrate bugs that caused two weeks of misdirected debugging. This must land before distribution work.  

Two parts:
- **Part 1: AGENTS.md revision** — split into 3 files, add 4 new rules (check-for-existing-fixes, root cause clustering, substrate-vs-symptom, dead-end escalation), add deletion-first heuristic, simplify mutation class ceremony
- **Part 2: Runtime migration** — 8-state machine: extract interfaces → rewire providers → extract tool registry → build actor adapter → rewire sandbox → migrate wire pipeline → delete old concurrency → E2E verify

29 pts total. Hybrid execution: agent does mechanical States 1-3, human+agent do States 4 and 6, agent does 7-8.

### 3d: Distribution

**Paradoc:** `docs/mission-3d-distribution-v0.md` (to be drafted after 3c)  
**Confidence:** Medium — proven mechanics, unproven under real load  
**Risk:** Medium — merge contention, routing accuracy at scale  

Scale to multiple VMs:
- Branch-per-VM, merge VM micro-batch merge
- Qdrant routing for shard assignment (route to VM that owns the semantic cluster)
- 2-3 VMs running, captures distributed
- Acceptance: captures distributed across VMs, branches merged, articles from merged state

### 3e: Source Expansion

**Paradoc:** `docs/mission-3e-source-expansion-v0.md` (to be drafted after 3c)  
**Confidence:** Medium — per-source risk varies  
**Risk:** Low-medium per source, research-heavy  

Expand source coverage to new protocols:
- **MTProto** — Telegram's real API protocol (needs api_id + api_hash from my.telegram.org)
- **ATProto** — Bluesky's protocol (public firehose via WebSocket, or authenticated app)
- **RSSHub** — open-source RSS generator for Asian social media (Weibo, Zhihu, Bilibili, etc.)
- **More** — Hacker News, Reddit, Mastodon streaming, etc.
- Can parallel with 3d (independent of distribution work)

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
Mission 3 is an umbrella mission with 5 sub-missions (3a-3e). 3a (cleanup & component wiring) is settled — commit e76aa068. 3b (new ingestion path E2E) is settled — deploy a0776488, real article on staging. Next: 3c (actor runtime migration) — the key priority. The actor runtime exists but was never wired in; the wire pipeline runs on the borked old runtime. See docs/mission-3-universal-wire-ingestion-rebuild-v0.md for the full architecture, docs/mission-3c-actor-runtime-migration-v0.md for the 3c paradoc, and docs/actor-runtime-migration-and-agents-md-revision-2026-06-27.md for the detailed plan. Spike evidence is in docs/mission-3-spikes-2026-06-27.md. All spikes are merged to main. Qdrant is deployed on node-b.
```
