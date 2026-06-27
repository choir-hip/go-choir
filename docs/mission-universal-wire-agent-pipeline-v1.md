# Mission: Universal Wire Agent Pipeline v1

## Status

Paradoc v1, created 2026-06-27 12:30 EDT, Boston, MA.
Supersedes v0. Updated after full-stack review revealed the existing agent
pipeline and original LLM synthesizer.

## Objective

Wire the existing agent pipeline to replace the deleted deterministic
scaffold. Sourcecycled ingestion dispatches processor agents. Processor
agents make typed decisions and route stories to Texture agents. Texture
agents use the LLM provider to synthesize English articles from source
text. Publication pipeline publishes to platformd and adds to the Wire
edition. Reconciler agents review the corpus.

The article-quality gate is the first acceptance: headline and body must
be about the underlying event, not source mechanics.

## What Already Exists

### Agent Profiles and Model Policy

From `internal/runtime/model_policy.go`:

| Profile | Provider | Model | Reasoning |
|---------|----------|-------|-----------|
| processor | xiaomi | mimo-v2.5 | medium |
| reconciler | deepseek | deepseek-v4-flash | medium |
| texture | chatgpt | gpt-5.5 | medium |
| conductor | chatgpt | gpt-5.4-mini | medium |
| super | chatgpt | gpt-5.5 | medium |
| researcher | chatgpt | gpt-5.4-mini | medium |

The models are already configured. The user's subscription pays for
gpt-5.4-mini and gpt-5.5. These models can run the system even though we
can't trust them to write system code.

### Ingestion Handoff

`internal/cycle/ingestion_handoff.go`:
- `BuildIngestionHandoff` batches source items by processor key
  (`processor:vertical:region:sourceType`)
- Batches capped at 50 items (`maxProcessorBatchItems`)
- Produces `ProcessorRequest` with stable request IDs, source item IDs,
  continuity refs, and a handoff prompt

### Processor Agent

`internal/runtime/runtime.go:createRunWithMetadata`:
- When profile is `AgentProfileProcessor`, automatically creates:
  - Request-level work item (`wire_processor_request_resolution`)
  - Per-source-item work items (`wire_source_item_resolution`)

`internal/runtime/tools_wire_processor.go`:
- `record_wire_processor_decision` tool with 5 verdicts:
  - `opened_texture` — route to Texture agent for article creation
  - `already_covered` — existing article covers this (requires published doc ID)
  - `not_newsworthy` — terminal no-story decision
  - `insufficient_evidence` — terminal no-story decision
  - `deferred` — non-terminal, keeps request open

`internal/runtime/wire_publication.go`:
- `recordWireProcessorDecision` — records verdict, updates work items
- `reconcileWireProcessorRequestResolution` — checks if all source items
  resolved, updates request resolution state, cancels trajectory if no story
- Verdict transitions: only `deferred` is non-terminal; all others are
  immutable once recorded
- `validateWireAlreadyCoveredDecision` — verifies cited doc is published

### Coagent Texture Routing

`internal/runtime/tools_coagent.go:ensureCoagentTextureRevisionRoute`:
- Processor calls this to route a story to a Texture agent
- Resolves or creates a Texture document for the story
- Starts a Texture agent revision run with source entities
- Records `opened_texture` decision on the source item work item
- Creates `wire_story_resolution` work item to track the article

`internal/runtime/texture_agent_revision.go`:
- `submitTextureAgentRevisionRun` — starts a Texture agent run with the
  LLM provider (gpt-5.5 per model policy)
- The Texture agent receives source text and writes an article revision

### Publication Pipeline

`internal/runtime/wire_publication.go:maybeAutonomousPublishWireArticle`:
- Checks eligibility (platform owner, canonical revision, agent run)
- Publishes to platformd via `publishWireArticleToPlatform`
- Persists publication ref
- Adds article to Wire edition document
- Records trajectory refs (publish_ref, edition_ref)
- Completes publication and story resolution work items
- Settles trajectory if all obligations resolved

`internal/runtime/wire_platform_publish.go`:
- `publishWireArticleToPlatform` — posts to platformd or wire publish URL
- `syncWireTextureDocumentToPlatformd` — syncs doc + revision rows

### Reconciler

`internal/runtime/wire_reconciler_debounce.go`:
- `wirePublishDebouncer` — batches publications (threshold: 10 or
  300s debounce)
- `dispatchStoryCorpusReconcilerFromPublishBatch` — starts a reconciler
  agent run with published doc IDs and revision IDs
- Reconciler reviews corpus for consensus, contradictions, drift

### Original LLM Synthesizer

`internal/cycle/synthesize.go`:
- `Synthesizer` calls DeepSeek v4-flash with a real news editor prompt
- Produces 4,000-word multilingual news issues with structured stories
- Has citation format: `[S1]`, `[S2]`, etc.
- This is the user's prototype. It works but isn't wired into the agent
  pipeline. It could be used as a reference for the Texture agent prompt,
  or as a fallback synthesizer.

## What Needs to Be Wired

### 1. Sourcecycled → Processor Dispatch

Currently `HandleInternalSourcecycledWebCaptures` in
`sourcecycled_web_captures.go` does:
1. Projects items into objectgraph as web captures
2. Calls `synthesizeUniversalWireLiveSourcecycledClusterFromGraphCaptures`
   (the deterministic scaffold — being deleted)

After deletion, it should:
1. Project items into objectgraph as web captures (KEEP)
2. Call `cycle.BuildIngestionHandoff` to create processor requests
3. Dispatch each `ProcessorRequest` via `StartRunWithMetadata` with
   `AgentProfileProcessor`

The ingestion handoff already builds the requests. The runtime already
creates work items for processor runs. The processor already has tools
for typed decisions. This is wiring, not new code.

### 2. Processor → Texture Agent

Already wired via `ensureCoagentTextureRevisionRoute`. The processor agent
receives source items, decides which are newsworthy, and routes stories to
the Texture agent. The Texture agent starts a revision run with the LLM
provider.

The key question: does the Texture agent prompt include source body text?
Need to verify `buildCoagentTextureRevisionPrompt` passes source text to
the Texture agent so it can synthesize an article.

### 3. Texture Agent → Article

The Texture agent uses gpt-5.5 (per model policy). It receives:
- Source entities (title, URL, language, body text)
- An objective ("Draft the article.")
- The existing document state (if updating)

The Texture agent should:
- Read source body text from the source entities
- Synthesize an English article about the underlying event
- Cite claims to sources using source_ref
- Write the article as a Texture revision

The prompt may need improvement to produce news-quality output. The
`cycle/synthesize.go` prompt is a good reference:

```
You are the lead editor for the Choir Universal Wire, the Automatic
Newspaper of Record for Planet Earth. Your mission is to synthesize
high-signal global information into a deeply contextualized, multilingual
news issue.
```

### 4. Publication → Edition → Reconciler

Already wired. `maybeAutonomousPublishWireArticle` publishes to platformd,
adds to edition, and the debouncer dispatches the reconciler. This should
work as-is once the Texture agent produces real article content.

## The Soft Guardrail

The previous autonomous run spent 22 hours building infrastructure to
avoid calling the LLM provider for news synthesis. The corrective constraint:

**The first move after deletion is to wire sourcecycled ingestion to
processor dispatch and verify the Texture agent produces a real article
using gpt-5.5.** Not infrastructure. Not a classifier. Not a template.
A processor run that routes to a Texture agent that calls gpt-5.5 and
produces an English article about the underlying event.

If the model refuses or produces poor output, document it and ask the
owner. Do not build a deterministic workaround.

The models are already configured:
- Processor: Xiaomi mimo-v2.5 (decides what's newsworthy)
- Texture: ChatGPT gpt-5.5 (writes the article)
- Reconciler: DeepSeek v4-flash (reviews corpus)

These are paid for by the subscription. They can run the system.

## Checklist

- [ ] Verify Mission 1 (heresy deletion) is settled
- [ ] Verify `buildCoagentTextureRevisionPrompt` passes source body text
- [ ] Wire `HandleInternalSourcecycledWebCaptures` to call
      `BuildIngestionHandoff` and dispatch processor runs
- [ ] Remove or stub the deterministic synthesis call path
- [ ] Write a test that dispatches a processor run with 2 source items,
      verifies the processor routes to Texture, and the Texture agent
      produces article-grade output
- [ ] Verify publication pipeline publishes the article to platformd
- [ ] Verify the article appears in the Wire edition
- [ ] Deploy to staging
- [ ] Run authenticated staging acceptance: load Universal Wire, open an
      article, verify headline and body are about the event
- [ ] Verify source citations are present and openable

Acceptance: one real LLM-synthesized article on staging. Headline about the
event. Body is English prose. Sources cited. Article openable. Produced
through the agent pipeline (processor → Texture agent → publication), not
through direct runtime synthesis.

## Parallax State

status: proposed

mission conjecture: if the existing agent pipeline (processor → Texture
agent → publication → reconciler) is wired to replace the deterministic
scaffold, and the Texture agent uses gpt-5.5 to synthesize articles from
source text, then Universal Wire's core product requirement is met and the
soft guardrail heresy is repaired.

deeper goal (G): the automatic newspaper — broad multilingual ingestion,
event understanding, English synthesis, live article updates. Running
through the agent pipeline, not through deterministic runtime code.

witness/spec (A/S): processor run with typed decisions, Texture agent
revision with LLM-synthesized content, platformd publication, Wire edition
entry, authenticated staging product replay.

invariants / qualities / domain ramp (I/Q/D): Do not reintroduce
deterministic synthesis. Do not add a 12-story cap. Do not use source
labels as headlines. Use the existing agent pipeline. Use the configured
models. Domain ramp: local test with mock provider → local test with real
provider → staging deploy → authenticated staging acceptance.

variant (conjecture descent) V: count conjectures about the agent pipeline.
Current: 4.
- C1: sourcecycled ingestion can dispatch processor runs
- C2: processor agent routes newsworthy items to Texture agent
- C3: Texture agent produces article-grade English from source text
- C4: publication pipeline publishes the article to staging
Target: 0.

budget: 3-5 passes. Wiring, prompt verification, staging acceptance.

authority / bounds: may modify `sourcecycled_web_captures.go` (dispatch
wiring), `tools_coagent.go` (prompt improvement), `texture_agent_revision.go`
(prompt). May deploy to staging. May not touch Texture core, O1-O3, or
delete agent pipeline code.

mutation class / protected surfaces: Orange/Red — wiring runtime behavior,
modifying agent prompts. Protected: Texture revision creation, platformd
sync contract, source entity graph.

evidence packet: processor run log, Texture agent revision content,
staging commit SHA, CI/deploy status, authenticated product replay.

heresy delta: `repaired` for H-WIRE-SOFT-GUARDRAIL (if gpt-5.5 produces
real article output). `discovered` for any quality gaps.

next move: verify `buildCoagentTextureRevisionPrompt` passes source body
text to the Texture agent. If it doesn't, fix the prompt. Then wire
sourcecycled ingestion to processor dispatch.

ledger file: `docs/mission-universal-wire-agent-pipeline-v1.ledger.md`

version / lineage: v1. Depends on `docs/mission-heresy-deletion-v1.md`.
Successor: `docs/mission-universal-wire-service-as-appagent-v1.md`.

settlement: settled when one real LLM-synthesized article is on staging,
produced through the agent pipeline. Open handoff if gpt-5.5 refuses to
produce news synthesis (document the refusal).

## Suggested Goal String

```text
Use Parallax on docs/mission-universal-wire-agent-pipeline-v1.md. Mission: wire the existing agent pipeline to produce real Universal Wire articles. The pipeline exists: cycle.BuildIngestionHandoff creates processor requests, processor agent uses record_wire_processor_decision tool with 5 verdicts, ensureCoagentTextureRevisionRoute starts Texture agent runs, Texture agent uses gpt-5.5 per model policy, maybeAutonomousPublishWireArticle publishes to platformd and adds to Wire edition, wirePublishDebouncer dispatches reconciler. First move: verify buildCoagentTextureRevisionPrompt passes source body text to Texture agent. Then wire HandleInternalSourcecycledWebCaptures to call BuildIngestionHandoff and dispatch processor runs instead of the deleted deterministic scaffold. Then deploy and verify one real LLM-synthesized article on staging with event-grade headline, English body, and cited sources. Do not reintroduce deterministic synthesis. Models: processor=mimo-v2.5, texture=gpt-5.5, reconciler=deepseek-v4-flash. Budget: 3-5 passes. Exit: settled when one real article is on staging produced through the agent pipeline.
```
