# Ledger: Heresy Deletion v1

Append-only. Written every pass; consult only when the Parallax State has
lost a thread.

## Pass 1 (2026-06-27) — single-pass settlement

- Conjecture: reverting session 2 + surgically deleting session 1 fake
  synthesis yields a compiling repo with agent pipeline + clean substrate
  intact, enabling the agent-pipeline rebuild.
- Verdict: `supported`. Repo compiles, all shards + cycle + O1-O3 pass,
  agent pipeline + cycle + clean substrate verified intact.
- Move: construct (batched). Selective checkout of session-2 code paths back
  to `6d88d7f5` (preserving docs + parallax skill at HEAD), then consolidated
  surgical deletion of session-1 fake synthesis in one pass.
- Expected ΔV: -3 (4 → 1 heresies). Actual ΔV: -3. Match.
- Receipts:
  - `go build ./...` exit 0
  - `go vet ./...` exit 0
  - `scripts/go-test-runtime-shards` (4 shards) all pass, 0 FAIL
  - `internal/cycle/...` pass; `internal/objectgraph/...` pass;
    `internal/store/...` pass; `internal/qdrant/...` pass
  - 10 surviving Universal Wire tests pass; 5 synthesis-bypass tests deleted
  - protected files verified unchanged from HEAD:
    `tools_wire_processor.go`, `wire_publication.go`, `tools_coagent.go`,
    `wire_reconciler_debounce.go`, `wire_platform_publish.go`,
    `cycle/synthesize.go`, `cycle/ingestion_handoff.go`
- Heresy delta: `repaired` x3 (H-WIRE-DETERMINISTIC-SCAFFOLD,
  H-WIRE-READ-TRIGGERED-WRITE, H-WIRE-ACCRETION-WITHOUT-CONSOLIDATION);
  `discovered` (not repaired) H-WIRE-SOFT-GUARDRAIL, open for successor.
- Open edge: the synthesis stub is inert ("not yet wired"). Successor mission
  `docs/mission-universal-wire-agent-pipeline-v1.md` must wire the agent
  pipeline to replace it; until then H-WIRE-SOFT-GUARDRAIL stays open.
- Net diff vs HEAD: ~4,253 lines removed across 13 files; docs + parallax
  skill preserved at HEAD.

## Pass 1b (2026-06-27) — verifier-forced dispatch revision

- Conjecture: the paradoc's Step 2 named "synthesizeUniversalWireSourceClusterTextureArticle
  (stub to processor dispatch)"; the completion verifier flagged that pass 1
  implemented an inert error stub rather than an actual dispatch, an explicit
  self-acknowledged deviation from a named deliverable.
- Verdict: `supported` after revision. The stub is replaced with a real
  processor dispatch.
- Move: construct. Investigated the processor-run creation contract
  (`createRunWithMetadata` keys on `agent_profile=processor` +
  `processor_key` + `source_item_ids`; `beginWireProcessorSourceDecisionWorkItems`
  reads `source_item_ids` and opens per-source-item work items). Discovered
  `runtime` cannot import `cycle` (cycle → provider → runtime import cycle),
  so the processor-key derivation is inlined mirroring `cycle.sourceProcessorKey`
  rather than imported. Implemented the dispatch + a focused test.
- Expected ΔV: 0 (conjecture already supported; this closes a deliverable gap,
  not a heresy). Actual ΔV: 0. Match.
- Receipts:
  - `go build ./...` exit 0; `go vet ./...` exit 0
  - all 4 runtime shards pass, 0 FAIL
  - `TestSynthesizeUniversalWireSourceClusterDispatchesProcessorRun` PASS —
    proves the dispatch returns the `processor_dispatched:<run_id>` sentinel,
    the run has `agent_profile=processor` + `processor_key=processor:...` +
    `universal_wire_story_cluster_id` + `source_item_ids` for both cluster
    sources, and per-source-item decision work items are created
  - 10 surviving Universal Wire tests + agent pipeline tests still pass
- Open edge: the processor-key derivation is duplicated in `wire_synthesis.go`
  and `cycle/ingestion_handoff.go` (consolidation debt for the successor
  mission — extract the handoff builder into a leaf package). H-WIRE-SOFT-
  GUARDRAIL still open: the dispatch is the ingress, but the processor/Texture
  agents do not yet produce a real LLM-synthesized article end-to-end.

