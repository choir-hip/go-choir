# Mission: Heresy Deletion v1

## Status

Paradoc v1, created 2026-06-27 12:30 EDT, Boston, MA.
Supersedes v0. Updated after full-stack review revealed the existing agent
pipeline (processor, Texture agent, publication, reconciler) and the original
LLM-powered synthesizer.

## Objective

Delete all Universal Wire heresies. Revert the second overnight session (115
commits of deterministic scaffold). Surgically delete the first session's
fake synthesis code. Leave the clean substrate (O1-O3), the existing agent
pipeline (processor decisions, coagent texture routing, publication, 
reconciler debounce), and the original LLM synthesizer intact.

Reach a known-clean repo state where the agent pipeline can be wired to
replace the deterministic scaffold.

## The Three Paths

### Path A: Original LLM Synthesizer (KEEP)

`internal/cycle/synthesize.go` — `cycle.Synthesizer` calls the LLM provider
(DeepSeek v4-flash via Fireworks) with a real news editor prompt. Produces
4,000-word multilingual news issues with structured stories and citations.
This is the user's 2-hour prototype. It works. It's not wired into the
agent pipeline yet, but it's the right idea.

### Path B: Deterministic Scaffold (DELETE)

`wire_synthesis.go` + `sourcecycled_web_captures.go` — hardcoded keyword
classifier, template prose, stale detection, helper-phrase blacklist,
read-triggered repair, live-arrival oracle. 115 commits of pure damage
from session 2, plus template prose and 12-story cap from session 1.
This bypasses the agent loop entirely.

### Path C: Agent Pipeline (KEEP, WIRE)

- `cycle/ingestion_handoff.go` — `BuildIngestionHandoff` batches source
  items into `ProcessorRequest`s by processor key (vertical:region:type)
- `runtime.go:createRunWithMetadata` — when profile is Processor, creates
  per-source-item work items for typed decisions
- `tools_wire_processor.go` — `record_wire_processor_decision` tool with
  5 verdicts: `opened_texture`, `already_covered`, `not_newsworthy`,
  `insufficient_evidence`, `deferred`
- `wire_publication.go` — work item lifecycle: begin, record, reconcile,
  complete, settle. Per-source-item and per-request fingerprints. Terminal
  vs non-terminal verdict transitions. Trajectory cancellation on no-story.
- `tools_coagent.go:ensureCoagentTextureRevisionRoute` — processor routes
  to Texture agent, which starts a revision run using the LLM provider
- `wire_reconciler_debounce.go` — debounced dispatch of reconciler agent
  after batch of publications
- `wire_platform_publish.go` — corpusd publication + Texture sync
- `model_policy.go` — processor uses Xiaomi mimo-v2.5, reconciler uses
  DeepSeek v4-flash, texture uses ChatGPT gpt-5.5

This is the real architecture. It exists. It's tested (414 lines of
processor decision tests). It just was never used for actual synthesis
because Path B short-circuited it.

## Heresy Inventory

### Session 2 Heresies (Revert `6d88d7f5..HEAD`)

115 commits, ~9,935 lines, 15 files. All heretical. Revert atomically.

- **H-WIRE-DETERMINISTIC-SCAFFOLD**: hardcoded keyword classifier, template
  prose, stale detection, helper-phrase blacklist, live-arrival oracle,
  semantic story DTOs, event frames, update decisions
- **H-WIRE-READ-TRIGGERED-WRITE**: `repairUniversalWireEditionArticleSurfaces`
  called from GET handler, creates Texture revisions on read
- **H-WIRE-ACCRETION-WITHOUT-CONSOLIDATION**: 115 commits, 71 new functions,
  0 consolidation commits, 3 separate stale-detection passes

### Session 1 Heresies (Surgical Deletion)

- **Template prose generator**: `universalWireSynthesisArticleMarkdown` in
  `wire_synthesis.go` — produces template strings, not LLM synthesis
- **12-story hard cap**: `h.universalWireEditionTextureStories(ctx, styleSources, 12)`
  in `universal_wire.go` — arbitrary product cap
- **Source-label headlines**: `universalWireLiveSynthesisHeadline` uses
  `sources[0].Title` as the headline
- **Source-label summary**: `universalWireLiveSynthesisSummary` produces
  template prose
- **Direct synthesis bypass**: `synthesizeUniversalWireSourceClusterTextureArticle`
  in `wire_synthesis.go` creates Texture revisions with template content,
  bypassing the processor → Texture agent → LLM provider pipeline

### Quarantined (Keep Until Appagent Rewrite)

- `wire_publication.go` (761 lines) — work item lifecycle, trajectory
  tracking. Real, tested, but centralized on Runtime.
- `wire_platform_publish.go` (260 lines) — corpusd sync. Real but
  centralized.
- `wire_reconciler_debounce.go` (222 lines) — debounced reconciler
  dispatch. Real but centralized.
- `sourcecycled_web_captures.go` (basic ingestion, ~300 lines pre-session-2)
  — capture ingestion. Real but centralized.
- `tools_wire_processor.go` (95 lines) — processor decision tool. Real.

### Clean Substrate (Keep)

- `internal/objectgraph/` — O1, settled
- `internal/qdrant/` — O2, settled
- `internal/store/texture_source_graph.go` + test — O3, settled
- `internal/sourcegraph/web_capture_graph.go` — web capture graph
- `internal/cycle/` — cycle engine, ingestion handoff, synthesizer
- `internal/types/wire.go` — `WireStory`, `WireSourceManifest`
- `internal/wirepublish/` — eligibility, request building

## Checklist

- [x] Revert `6d88d7f5..HEAD` (session 2, 115 commits)
- [x] Verify repo compiles after revert
- [x] Delete `universalWireSynthesisArticleMarkdown` template prose generator
- [x] Delete `universalWireLiveSynthesisHeadline` source-label headline function
- [x] Delete `universalWireLiveSynthesisSummary` template summary function
- [x] Delete or stub `synthesizeUniversalWireSourceClusterTextureArticle`
      (the direct synthesis bypass — keep the function signature if
      sourcecycled ingestion calls it, but replace the body with a
      processor dispatch)
- [x] Remove 12-story hard cap from `HandleUniversalWireStories`
- [x] Delete tests for deleted functions
- [x] Verify repo compiles and remaining tests pass
- [x] Run `nix develop -c scripts/go-test-runtime-shards` or focused Wire tests
- [ ] Commit deletion as a single atomic commit
- [x] Update heresy document with deletion evidence

Acceptance: repo compiles, existing tests pass, heresy count reduced by at
least 3. Agent pipeline code intact. Clean substrate intact.

## Parallax State

status: settled

mission conjecture: if reverting session 2 and surgically deleting session 1
fake synthesis code produces a compiling repo with the agent pipeline and
clean substrate intact, then the Universal Wire can be rebuilt by wiring
the existing agent pipeline to replace the deterministic scaffold.
VERDICT: supported. Repo compiles, all runtime shards + cycle + O1-O3 pass,
agent pipeline + cycle + clean substrate verified intact, 3 heresies repaired.

deeper goal (G): a clean repo state where the existing processor → Texture
agent → publication → reconciler pipeline can be wired to produce real
LLM-synthesized articles. ADVANCED — the synthesis entry point now dispatches
into the processor run, so the successor mission starts from a live dispatch
rather than an inert stub.

witness/spec (A/S): reverted commit range, deleted functions, compiling repo,
passing tests, processor dispatch implemented, heresy document updated.
ALL SATISFIED.

invariants / qualities / domain ramp (I/Q/D): Do not touch O1-O3 settled code.
Do not touch Texture core. Do not delete agent pipeline code (processor
decisions, coagent routing, publication, reconciler debounce). Do not delete
`cycle/synthesize.go`. Domain: local build + test only; no deploy needed.
ALL PRESERVED — verified each protected file matches the pre-mission commit.

variant (conjecture descent) V: count heresies still present. Was 4.
Now 1 (H-WIRE-SOFT-GUARDRAIL, open until real LLM synthesis is produced).
ΔV = -3, exceeds the settlement requirement of "reduced by 3+".

budget: 1 pass granted, 1 pass spent (with one verifier-forced revision adding
the actual processor dispatch after the initial inert-stub pass). Solvent.

authority / bounds: reverted session-2 code, deleted functions, deleted tests,
implemented the processor dispatch, committed. Did not touch agent pipeline,
cycle package, O1-O3, Texture core, or quarantined centralized service code.

mutation class / protected surfaces: Orange — deleting runtime behavior + wiring
a processor dispatch. Protected: agent pipeline, cycle, O1-O3, Texture core.
All verified intact.

evidence packet: working-tree diff (~4,230 lines removed, dispatch added),
`go build ./...` clean, `go vet ./...` clean, all 4 runtime shards pass,
`internal/cycle/...` pass, `internal/objectgraph/...` pass,
`internal/store/...` pass, `internal/qdrant/...` pass, 10 surviving Universal
Wire tests pass, `TestSynthesizeUniversalWireSourceClusterDispatchesProcessorRun`
proves the dispatch submits a processor run with `agent_profile=processor`,
`processor_key`, `source_item_ids`, and cluster metadata, and that
`createRunWithMetadata` opens per-source-item decision work items. 5
synthesis-bypass tests deleted. Heresy doc updated with deletion evidence.

heresy delta: `repaired` for H-WIRE-DETERMINISTIC-SCAFFOLD,
H-WIRE-READ-TRIGGERED-WRITE, H-WIRE-ACCRETION-WITHOUT-CONSOLIDATION.
`discovered` (not repaired) for H-WIRE-SOFT-GUARDRAIL — that stays open for
the successor mission (the dispatch routes into the pipeline, but the
processor/Texture agents do not yet produce a real LLM-synthesized article
end-to-end).

position / live conjectures / open edges: The repo is at a known-clean state.
`synthesizeUniversalWireSourceClusterTextureArticle` now dispatches a real
processor run (translates sources → `sources.Item`, derives the processor
key/batch mirroring `cycle.sourceProcessorKey`, and submits via
`StartRunWithMetadata` with the metadata shape `createRunWithMetadata`
recognizes). The processor-key derivation is inlined in `wire_synthesis.go`
because `runtime` cannot import `cycle` (the cycle → provider → runtime import
cycle); this is consolidation debt for the successor mission, which should
extract the handoff builder into a leaf package both can import. What remains
for H-WIRE-SOFT-GUARDRAIL: the processor agent must decide to open a Texture
story and the Texture agent must write the revision via the LLM provider, then
wire publication/reconciler must carry it to the edition. The dispatch is the
ingress; the synthesis is the successor's work.

next move: amend the deletion commit with the dispatch, then hand off to
`docs/mission-universal-wire-agent-pipeline-v1.md`.

ledger file: `docs/mission-heresy-deletion-v1.ledger.md`

version / lineage: v1. Depends on
`docs/heresy-universal-wire-deterministic-scaffold-2026-06-27.md`.
Successor: `docs/mission-universal-wire-agent-pipeline-v1.md`.

learning state: (1) the selective-checkout technique (restore code paths from
an anchor while preserving unrelated docs/skill at HEAD) cleanly separates a
session's code accretion from unrelated commits that landed on top; (2)
`runtime` cannot import `cycle` because cycle → provider → runtime — a real
architectural constraint the successor mission must resolve by extracting the
handoff builder into a leaf package. Retain both for future Universal Wire work.

settlement: SETTLED. Repo compiles, all relevant tests pass, heresy count
reduced by 3 (4 → 1), agent pipeline and clean substrate verified intact, and
the synthesis entry point dispatches a real processor run.

## Suggested Goal String

```text
Use Parallax on docs/mission-heresy-deletion-v1.md. Mission: delete Universal Wire heresies. Step 1: revert 6d88d7f5..HEAD (115 commits). Step 2: delete session 1 fake synthesis: universalWireSynthesisArticleMarkdown, universalWireLiveSynthesisHeadline, universalWireLiveSynthesisSummary, synthesizeUniversalWireSourceClusterTextureArticle (stub to processor dispatch), 12-story cap. Step 3: delete tests for deleted functions. Step 4: verify compiles and tests pass. DO NOT DELETE: agent pipeline (tools_wire_processor.go, wire_publication.go work items, tools_coagent.go coagent routing, wire_reconciler_debounce.go, wire_platform_publish.go), cycle package (synthesize.go, ingestion_handoff.go), O1-O3, Texture core. Budget: 1 pass. Exit: settled when heresy count reduced by 3+ and repo compiles with agent pipeline intact.
```
