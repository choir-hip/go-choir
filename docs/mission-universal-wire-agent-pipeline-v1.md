# Mission: Universal Wire Agent Pipeline v1.1

## Status

Paradoc v1.1, created 2026-06-27 13:38 EDT, Boston, MA.
Supersedes v1. Updated after Mission 1 (heresy deletion) settled.
The dispatch is wired; remaining work is verifying and fixing the
processor → Texture agent → LLM provider chain end-to-end.

## Objective

Complete the agent pipeline so that sourcecycled ingestion produces real
LLM-synthesized articles on staging. Mission 1 already wired the dispatch
(processor runs are submitted with correct metadata and work items). The
remaining work is verifying the processor agent routes to Texture, the
Texture agent receives source body text, gpt-5.5 synthesizes an article,
and the publication pipeline carries it to the edition.

The article-quality gate is the first acceptance: headline and body must
be about the underlying event, not source mechanics.

## What Mission 1 Did

- Reverted session 2 (115 commits of deterministic scaffold)
- Deleted session 1 fake synthesis (template prose, source-label headlines,
  12-story cap)
- Replaced `synthesizeUniversalWireSourceClusterTextureArticle` with a real
  processor dispatch: translates sources → `sources.Item`, derives processor
  key, submits processor runs via `StartRunWithMetadata`
- Commit: `4b5c665b` on `main`
- Agent pipeline, cycle package, O1-O3, Texture core — all intact
- Heresy count: 4 → 1 (H-WIRE-SOFT-GUARDRAIL remains open)

## What Already Exists (Intact After Mission 1)

### Agent Profiles and Model Policy

From `internal/runtime/model_policy.go`:

| Profile | Provider | Model | Reasoning |
|---------|----------|-------|-----------|
| processor | chatgpt | gpt-5.5 | low |
| reconciler | chatgpt | gpt-5.5 | low |
| texture | chatgpt | gpt-5.5 | low |
| conductor | chatgpt | gpt-5.4-mini | medium |
| super | chatgpt | gpt-5.5 | medium |
| researcher | chatgpt | gpt-5.4-mini | medium |

All three Wire roles (processor, texture, reconciler) use gpt-5.5 (low)
for low-cost development. Not ideal to use the same model for everything,
but it's included in the subscription. Can diversify later.

### Dispatch (Wired by Mission 1)

`wire_synthesis.go:synthesizeUniversalWireSourceClusterTextureArticle`:
- Translates cluster sources → `sources.Item`
- Derives processor key (mirroring `cycle.sourceProcessorKey`, inlined
  because `runtime` cannot import `cycle`)
- Submits processor runs via `StartRunWithMetadata` with:
  `agent_profile=processor`, `processor_key`, `source_item_ids`,
  `ingestion_handoff_request_id`, `continuity_ref`,
  `universal_wire_story_cluster_id`
- `createRunWithMetadata` opens per-source-item decision work items
- Proven by `TestSynthesizeUniversalWireSourceClusterDispatchesProcessorRun`

`sourcecycled_web_captures.go`: ingestion handler projects items into
objectgraph, then calls `synthesizeUniversalWireLiveSourcecycledClusterFromGraphCaptures`
which calls the dispatch.

### Processor Agent

`tools_wire_processor.go`: `record_wire_processor_decision` tool with 5
verdicts: `opened_texture`, `already_covered`, `not_newsworthy`,
`insufficient_evidence`, `deferred`.

`wire_publication.go`: work item lifecycle, verdict transitions,
reconciliation, trajectory cancellation.

### Coagent Texture Routing

`tools_coagent.go:ensureCoagentTextureRevisionRoute`:
- Processor calls this to route a story to a Texture agent
- Resolves or creates a Texture document
- Builds source entities from content items
- Starts a Texture agent revision run with gpt-5.5
- Records `opened_texture` decision
- Creates `wire_story_resolution` work item

### Publication Pipeline

`wire_publication.go:maybeAutonomousPublishWireArticle`: publishes to
platformd, adds to Wire edition, records trajectory refs, completes work
items, settles trajectory.

### Reconciler

`wire_reconciler_debounce.go`: debounced dispatch of reconciler agent
after batch of publications (threshold: 10 or 300s).

## Critical Gap: Source Body Text Not in Texture Prompt

**This is the key finding from the v1.1 review.**

`buildCoagentTextureRevisionPrompt` (in `tools_coagent.go:438`) builds the
prompt for the Texture agent. It includes:
- The objective ("Write the first publication-quality article revision...")
- Source entity IDs, labels, content IDs, and item IDs
- Style.texture source context
- Hard requirements (use patch_texture, reference source entities, etc.)

But it does **NOT** include the source body text. The source entities
carry `text_content` (up to 100K runes via `applyContentItemReaderSnapshot`)
and `excerpt_text` (2K runes), but the prompt only lists entity IDs and
labels — it never includes the actual article text from the sources.

The Texture agent (gpt-5.5) receives a prompt saying "reference these
source entities" but has no source text to synthesize from. It cannot
write an article about an event it cannot read.

**This is likely where the previous agent gave up and built the
deterministic scaffold instead of fixing the prompt.**

### Fix

The prompt needs to include source body text. Options:

**Option A (preferred): Include excerpt text in the prompt.**
Add a "Source briefs" section to `buildCoagentTextureRevisionPrompt` that
includes the `excerpt_text` (2K runes per source) from each source entity.
For a 5-source cluster, that's ~10K runes of source text — well within
gpt-5.5's context window. The Texture agent can synthesize from this.

**Option B: Include full text_content in the prompt.**
For clusters with few sources or short articles, include the full
`text_content` (up to 100K runes). This may be too large for many sources.

**Option C: Use a separate "source brief" revision.**
The processor agent writes a source brief revision (summary of source
content) and the Texture agent synthesizes from that. This adds a step
but keeps prompts small.

**Recommend Option A** — it's the minimal fix. Add excerpt text to the
prompt. The Texture agent already has the source entities; it just needs
the text.

## Consolidation Debt from Mission 1

The processor-key derivation is duplicated:
- `wire_synthesis.go:universalWireSourceProcessorKey` (inlined for runtime)
- `cycle/ingestion_handoff.go:sourceProcessorKey` (original)

This is because `runtime` cannot import `cycle` (import cycle:
`cycle → provider → runtime`). The fix is to extract the handoff builder
into a leaf package both can import (e.g., `internal/handoff/` or
`internal/sourcebatch/`). This should be done as part of this mission
or deferred to Mission 3 (service-as-appagent).

## What Needs to Be Done

### 1. Fix the Texture Agent Prompt (CRITICAL)

Add source body text to `buildCoagentTextureRevisionPrompt` in
`tools_coagent.go`. Include `excerpt_text` from each source entity in a
"Source briefs" section. The Texture agent needs the actual source text
to synthesize an article.

### 2. Verify Processor Agent Routes to Texture

The processor agent (mimo-v2.5) receives source items and should call
`ensureCoagentTextureRevisionRoute` for newsworthy items. Verify:
- The processor agent's tool registry includes the coagent route tool
- The processor agent prompt instructs it to route newsworthy items
- The processor agent actually calls the tool (not just records
  `not_newsworthy` for everything)

### 3. Verify Publication Pipeline

Once the Texture agent produces a real article revision,
`maybeAutonomousPublishWireArticle` should publish it to platformd and
add it to the Wire edition. Verify:
- The eligibility check passes for the platform owner
- The platformd publish succeeds
- The article appears in the Wire edition

### 4. Extract Handoff Builder (Optional, Can Defer to Mission 3)

Extract `sourceProcessorKey` and `BuildIngestionHandoff` into a leaf
package to eliminate the duplication. Both `runtime` and `cycle` import
the leaf package.

### 5. Deploy and Verify on Staging

Deploy to `https://choir.news`. Trigger a sourcecycled cycle. Verify:
- Processor run is dispatched
- Processor routes to Texture agent
- Texture agent produces an article using gpt-5.5
- Article is published to platformd
- Article appears in the Wire edition
- Article headline is about the event
- Article body is English prose
- Source citations are present

## The Soft Guardrail

The previous autonomous run spent 22 hours building infrastructure to
avoid calling the LLM provider for news synthesis. The corrective constraint:

**The first move is to fix the Texture agent prompt to include source
body text, then verify gpt-5.5 produces a real article.** Not
infrastructure. Not a classifier. Not a template. A prompt fix and a
staging verification.

If the model refuses or produces poor output, document it and ask the
owner. Do not build a deterministic workaround.

The models are already configured and paid for:
- Processor: ChatGPT gpt-5.5 low (decides what's newsworthy)
- Texture: ChatGPT gpt-5.5 low (writes the article)
- Reconciler: ChatGPT gpt-5.5 low (reviews corpus)

## Checklist

- [ ] Fix `buildCoagentTextureRevisionPrompt` to include source body text
      (excerpt_text from each source entity)
- [ ] Verify processor agent tool registry includes coagent route tool
- [ ] Verify processor agent prompt instructs routing newsworthy items
- [ ] Write a local test that runs the full chain with a mock provider:
      dispatch → processor → Texture agent → article revision
- [ ] Verify publication pipeline publishes the article
- [ ] Extract handoff builder into leaf package (optional, can defer)
- [ ] Deploy to staging
- [ ] Trigger a sourcecycled cycle on staging
- [ ] Verify processor run is dispatched
- [ ] Verify Texture agent produces an article using gpt-5.5
- [ ] Verify article is published to platformd
- [ ] Verify article appears in the Wire edition
- [ ] Run authenticated staging acceptance: load Universal Wire, open an
      article, verify headline and body are about the event
- [ ] Verify source citations are present and openable

Acceptance: one real LLM-synthesized article on staging. Headline about the
event. Body is English prose. Sources cited. Article openable. Produced
through the agent pipeline (processor → Texture agent → publication), not
through direct runtime synthesis.

## Parallax State

status: proposed

mission conjecture: if the Texture agent prompt is fixed to include source
body text, and the existing agent pipeline (processor → Texture agent →
publication → reconciler) runs end-to-end on staging, then Universal Wire's
core product requirement is met and the soft guardrail heresy is repaired.

deeper goal (G): the automatic newspaper — broad multilingual ingestion,
event understanding, English synthesis, live article updates. Running
through the agent pipeline with gpt-5.5, not through deterministic runtime
code.

witness/spec (A/S): fixed prompt with source body text, processor run
with typed decisions, Texture agent revision with LLM-synthesized content,
platformd publication, Wire edition entry, authenticated staging product
replay.

invariants / qualities / domain ramp (I/Q/D): Do not reintroduce
deterministic synthesis. Do not add a 12-story cap. Do not use source
labels as headlines. Use the existing agent pipeline. Use the configured
models. Do not touch Texture core, O1-O3, or delete agent pipeline code.
Domain ramp: local test with mock provider → local test with real
provider → staging deploy → authenticated staging acceptance.

variant (conjecture descent) V: count conjectures about the agent pipeline.
Current: 4.
- C1: sourcecycled ingestion dispatches processor runs (PROVEN by Mission 1)
- C2: processor agent routes newsworthy items to Texture agent
- C3: Texture agent produces article-grade English from source text
  (BLOCKED by missing source body text in prompt)
- C4: publication pipeline publishes the article to staging
Target: 0.

budget: 3-5 passes. Prompt fix, local verification, staging acceptance.

authority / bounds: may modify `tools_coagent.go` (prompt fix),
`texture_agent_revision.go` (prompt), `tool_profiles.go` (tool registry
verification), `wire_synthesis.go` (consolidation). May deploy to staging.
May not touch Texture core, O1-O3, or delete agent pipeline code.

mutation class / protected surfaces: Orange/Red — fixing agent prompts,
wiring runtime behavior, deploying to staging. Protected: Texture revision
creation, platformd sync contract, source entity graph.

evidence packet: fixed prompt diff, processor run log, Texture agent
revision content, staging commit SHA, CI/deploy status, authenticated
product replay.

heresy delta: `repaired` for H-WIRE-SOFT-GUARDRAIL (if gpt-5.5 produces
real article output). `discovered` for any quality gaps.

next move: fix `buildCoagentTextureRevisionPrompt` in `tools_coagent.go`
to include source body text (excerpt_text from each source entity). This
is the critical gap — the Texture agent cannot synthesize without source
text.

ledger file: `docs/mission-universal-wire-agent-pipeline-v1.ledger.md`

version / lineage: v1.1. Depends on `docs/mission-heresy-deletion-v1.md`
(settled). Successor: `docs/mission-universal-wire-service-as-appagent-v1.md`.

settlement: settled when one real LLM-synthesized article is on staging,
produced through the agent pipeline. Open handoff if gpt-5.5 refuses to
produce news synthesis (document the refusal).

## Suggested Goal String

```text
Use Parallax on docs/mission-universal-wire-agent-pipeline-v1.md. Mission: complete the Universal Wire agent pipeline to produce real LLM-synthesized articles. Mission 1 (heresy deletion) already wired the dispatch: synthesizeUniversalWireSourceClusterTextureArticle submits processor runs with correct metadata and work items. CRITICAL GAP: buildCoagentTextureRevisionPrompt in tools_coagent.go does NOT include source body text — it lists source entity IDs and labels but not the actual article text. The Texture agent (gpt-5.5) cannot synthesize without source text. First move: fix the prompt to include excerpt_text from each source entity. Then verify processor agent routes newsworthy items to Texture via ensureCoagentTextureRevisionRoute. Then deploy to staging and verify one real LLM-synthesized article: event-grade headline, English body, cited sources, openable on choir.news. Do not reintroduce deterministic synthesis. Models: processor=gpt-5.5 (low), texture=gpt-5.5 (low), reconciler=gpt-5.5 (low). Budget: 3-5 passes. Exit: settled when one real article is on staging produced through the agent pipeline.
```
