# Mission 3a: Cleanup & Component Wiring

**Status:** ready for execution  
**Date:** 2026-06-27  
**Umbrella:** `docs/mission-3-universal-wire-ingestion-rebuild-v0.md`  
**Predecessor:** `docs/mission-heresy-deletion-v1.md` (settled)  
**Successor:** `docs/mission-3b-ingestion-path-v0.md` (to be drafted after 3a)

## Objective

Extract duplicated logic to a leaf package and wire the spike components
(OllamaEmbedder, DoltStore, Qdrant client) into the real pipeline. Each
component is already proven by spike tests. This mission connects them.

No new architecture. No E2E flow. No staging deploy. Pure local wiring.

## Work Items

### W1: Extract 5 duplicated processor key functions

**Verified:** `cycle/ingestion_handoff.go` and `runtime/wire_synthesis.go`
contain 5 character-for-character identical functions (except name prefixes):

| `cycle/ingestion_handoff.go` | `runtime/wire_synthesis.go` |
|---|---|
| `sourceProcessorKey` | `universalWireSourceProcessorKey` |
| `stableRequestID` | `universalWireStableRequestID` |
| `safeKeyPart` | `universalWireSafeKeyPart` |
| `orderedSourceItemIDs` | `universalWireOrderedSourceItemIDs` |
| `processorHandoffPrompt` | `universalWireProcessorHandoffPrompt` |

Duplicated because `runtime` cannot import `cycle` (import cycle:
cycle → provider → runtime).

**Action:** Extract all 5 to a new leaf package (e.g.,
`internal/wire/processorkey/`) that both `cycle` and `runtime` can import.
Update both call sites. Delete the `universalWire*` versions in
`wire_synthesis.go`.

**Files:** `internal/cycle/ingestion_handoff.go`, `internal/runtime/wire_synthesis.go`, new leaf package

### W2: Wire OllamaEmbedder into Qdrant pipeline

**Spike evidence:** `internal/qdrant/ollama_embedder.go` — 7 unit tests pass.
Implements the `Embedder` interface. 1024-dim embeddings via Ollama `/api/embed`.

**Action:** Find where the Qdrant pipeline currently gets its embedder (likely
a deterministic hash embedder for tests). Replace it with `OllamaEmbedder`
configured to point at the Ollama instance. Make the Ollama base URL and model
name configurable via env vars with sensible defaults.

**Files:** `internal/qdrant/` pipeline code, possibly `cmd/sourcecycled/main.go` or `internal/runtime/` for wiring

### W3: Wire DoltStore as objectgraph.Store

**Spike evidence:** `internal/objectgraph/dolt_store.go` — 13 tests pass.
Implements the `Store` interface. MySQL dialect, embedded Dolt workspace.

**Action:** Find where `objectgraph.Store` is currently instantiated (likely
`SQLiteStore`). Replace with `DoltStore` or `OpenDoltStore`. Configure the
workspace path. Ensure the schema migration runs on startup.

**Files:** Wherever `SQLiteStore` is constructed (likely `internal/runtime/` or `cmd/`), `internal/objectgraph/`

### W4: Configure Qdrant client for node-b

**Spike evidence:** Qdrant is deployed on node-b, healthz passing at
`http://127.0.0.1:6333`. The Qdrant client exists in `internal/qdrant/`.

**Action:** Configure the Qdrant client URL to point at node-b's Qdrant
instance. For local dev, use env var override. For staging, the service is
already running. Create the production Qdrant collection with the correct
schema (vector size 1024, distance Cosine, payload indexes for vm_owner and
content_hash).

**Files:** Qdrant client construction code, possibly `nix/` for config

### W5: Verify source body text reaches Texture agent

**Prior fix:** `sourceEntityExcerptText` helper added in prior session,
tests pass in `wire_processor_decision_test.go`.

**Action:** Verify the fix is still working after all the spike merges. Run
the relevant tests. If broken, fix.

**Files:** `internal/runtime/tools_coagent.go`, `internal/runtime/wire_processor_decision_test.go`

## What NOT to Touch

- Agent pipeline: `tools_wire_processor.go`, `wire_publication.go`, `tools_coagent.go`, `wire_reconciler_debounce.go`, `wire_platform_publish.go`
- `cycle/synthesize.go` — original LLM synthesizer
- O1-O3 settled code (objectgraph core, qdrant schema, texture source graph)
- Spike test files (ollama_embedder_test.go, dolt_store_test.go, dolt_branch_test.go, dolt_tcp_branch_test.go, routing_test.go)
- `internal/sources/` type definitions
- `internal/sourcegraph/` — provenance infrastructure
- No staging deploy (local build + test only)

## Checklist

- [ ] W1: Extract 5 processor key functions to leaf package, update both call sites
- [ ] W2: Wire OllamaEmbedder into Qdrant pipeline (replace hash embedder)
- [ ] W3: Wire DoltStore as objectgraph.Store (replace SQLiteStore)
- [ ] W4: Configure Qdrant client for node-b, create production collection
- [ ] W5: Verify source body text fix still works
- [ ] Verify repo compiles: `nix develop -c go build ./...`
- [ ] Verify tests pass: `nix develop -c go test ./internal/cycle/... ./internal/runtime/... ./internal/qdrant/... ./internal/objectgraph/... ./internal/sourcegraph/... ./internal/sources/...`
- [ ] Run runtime shards: `nix develop -c scripts/go-test-runtime-shards`
- [ ] Commit each work item separately (W1, W2, W3, W4, W5)
- [ ] Update this document with evidence

## Acceptance

- Repo compiles clean
- All existing tests pass
- Processor key logic exists in exactly one place (leaf package)
- OllamaEmbedder is the active embedder in the Qdrant pipeline
- DoltStore is the active Store implementation
- Qdrant client points at node-b (configurable via env)
- Source body text reaches Texture agent (tests pass)
- Agent pipeline intact and functional
- No staging deploy required

## Parallax State

status: ready

mission conjecture: if the 5 duplicated functions are extracted to a leaf
package and the spike components (OllamaEmbedder, DoltStore, Qdrant client)
are wired into the real pipeline, then the Universal Wire pipeline has a
clean substrate with real embeddings, real Dolt storage, and live Qdrant —
ready for the E2E ingestion path (3b).

deeper goal (G): a wired pipeline where Qdrant routing, Dolt object graph,
and LLM-only synthesis can be tested end-to-end without mock embedders,
SQLite stores, or local-only Qdrant.

witness/spec (A/S): extracted leaf package, wired components, compiling repo,
passing tests, Qdrant collection created on node-b.

invariants / qualities / domain ramp (I/Q/D):
- I: Do not modify agent pipeline, cycle/synthesize.go, O1-O3, spike tests, source types, sourcegraph
- Q: No new behavior. No E2E flow. No staging deploy. Each work item is a separate commit.
- D: Local build + test only. Qdrant collection creation on node-b is read/write to Qdrant only (no NixOS change).

variant (conjecture descent) V: count uncompleted work items. V = 0
(settled). All 5 work items (W1-W5) completed in one pass. Repo compiles,
all tests pass, agent pipeline intact.

## Parallax State (settled 2026-06-27)

- **W1**: Extracted 5 functions to `internal/wire/processorkey/`. Updated
  call sites in `internal/cycle/ingestion_handoff.go`,
  `internal/cycle/ingestion_event.go`, and `internal/runtime/wire_synthesis.go`.
  Deleted all `universalWire*` duplicates.
- **W2**: Added `OllamaURL`, `OllamaEmbeddingModel` config fields with env
  vars `OLLAMA_URL`, `OLLAMA_EMBEDDING_MODEL`. `Runtime.QdrantPipeline()`
  lazily constructs a `qdrant.Pipeline` with `OllamaEmbedder`.
- **W3**: Replaced `SQLiteStore` with `DoltStore` via `OpenDoltStore` in
  `objectgraph_runtime.go`. New `runtimeObjectGraphDoltWorkspace` derives
  workspace path from store path.
- **W4**: Added `QdrantURL` config field with env var `QDRANT_URL` (default
  `http://127.0.0.1:6333`). Added `qdrant.EnsureProductionCollection` and
  `Runtime.EnsureProductionQdrantCollection` for idempotent collection
  creation (1024-dim, Cosine, payload indexes on `vm_owner` + `content_hash`).
- **W5**: All `wire_processor_decision` tests pass. Full test suite passes:
  `go build ./...`, `go test ./internal/cycle/... ./internal/qdrant/...
  ./internal/objectgraph/... ./internal/sourcegraph/... ./internal/sources/...`,
  `scripts/go-test-runtime-shards`.

budget: 1-2 passes. W1 is mechanical extraction. W2-W4 are wiring proven
components. W5 is verification. Should fit in one focused session.

authority / bounds: may create one new leaf package. May modify
`internal/cycle/ingestion_handoff.go`, `internal/runtime/wire_synthesis.go`,
Qdrant pipeline wiring code, objectgraph Store construction code, Qdrant
client construction code. May not touch agent pipeline, cycle/synthesize.go,
O1-O3, spike tests, source types, sourcegraph.

mutation class / protected surfaces: Orange — wiring changes runtime
behavior (embedder, store, Qdrant client). Protected: agent pipeline,
cycle/synthesize.go, O1-O3, spike code.

rollback path: each work item is a separate commit. Revert the specific
commit if it breaks something unexpected.

## Suggested Goal String

```text
Use Parallax on docs/mission-3a-cleanup-wiring-v0.md. Mission: extract duplicated processor key functions + wire spike components into real pipeline. W1: extract 5 duplicated functions (sourceProcessorKey, stableRequestID, safeKeyPart, orderedSourceItemIDs, processorHandoffPrompt) from internal/cycle/ingestion_handoff.go to a new leaf package (e.g., internal/wire/processorkey/) that both cycle and runtime can import. Update both call sites. Delete universalWire* versions in wire_synthesis.go. W2: replace deterministic hash embedder with OllamaEmbedder in Qdrant pipeline (configure base URL + model via env). W3: replace SQLiteStore with DoltStore as objectgraph.Store. W4: configure Qdrant client to point at node-b (http://127.0.0.1:6333, configurable via env), create production collection (1024-dim, Cosine, payload indexes for vm_owner + content_hash). W5: verify source body text reaches Texture agent (run wire_processor_decision tests). DO NOT TOUCH: agent pipeline (tools_wire_processor.go, wire_publication.go, tools_coagent.go, wire_reconciler_debounce.go, wire_platform_publish.go), cycle/synthesize.go, O1-O3, spike tests, internal/sources/ types, sourcegraph. No staging deploy. Verify: go build ./..., go test ./internal/cycle/... ./internal/runtime/... ./internal/qdrant/... ./internal/objectgraph/... ./internal/sourcegraph/... ./internal/sources/..., scripts/go-test-runtime-shards. Budget: 1-2 passes. Exit: settled when V=0 (all 5 work items done, repo compiles, tests pass, agent pipeline intact).
```
