# Mission: Global Wire SourceMaxx Newsroom Runtime

**Status:** rewritten MissionGradient mission after architecture correction.
**Requirements contract:** `docs/choir-global-wire-style-vtext-dual-object-spec-2026-06-07.md`  
**Prior mission context:** `docs/mission-global-wire-style-vtext-collaborative-storygraph-v0.md`  
**Created:** 2026-06-07
**Rewritten:** 2026-06-07

## Goal String

```text
/goal Run docs/mission-global-wire-sourcemaxx-publication-system-v0.md as an overnight MissionGradient mission. Redesign and deliver Global Wire as a SourceMaxx newsroom runtime: continuous high-volume source ingestion feeds long-running processors with hot-context/KV-cache continuity; processors compact themselves as needed while preserving handles to full source content; reconcilers review the article/story corpus, source state, processor notes, researcher packets, contradictions, consensus, drift, and open questions; existing researcher agents perform additional evidence work; existing VText agents write and revise normal article/story VTexts from processor notes, reconciler notes, researcher packets, and matched deep Style.vtext artifacts. A story/article is a VText; do not create a separate story object taxonomy. Processor notes, reconciler notes, and researcher packets should also be represented as VTexts where practical, so early reasoning can become the v0/v1 seed for later article versions. VText is the provenance-bearing object: sources, multimedia transclusions, Style.vtext citations, authoring context, and source references are per-version, and native VText versioning backed by DoltDB carries the version provenance. VText indexes may help navigation, discovery, and future VText-path narration, but they must index VTexts/versions/transclusions and must not replace or override VText-native version/source provenance. Processors and reconcilers use the same agentic loop/tool model as other Choir agents: they can take tool calls, request researchers, receive research results through durable state, and request VText work without becoming standalone researcher/writer systems. Build toward hundreds of GDELT/RSS/Telegram/search SourceItems per 15 minutes or faster where feasible, with durable source ingestion, dedupe, routing, processor/reconciler records, user-owned VText forks/edits/publications, publication-quality Style.vtext projections, and clean newspaper-style Global Wire views with no nested scrolling panels, repeated card wall, story boxes, or story border lines. Keep the News app minimal: readable newspaper columns, source chronology with filters, compact provenance/style disclosure, and per-article VText open affordances for full reading/editing/forking; every article must be openable as a VText, but do not repeat `Open in VText` label text on every item. Mobile is a responsive Global Wire app inside the Choir web desktop/shell, not a native phone app. Do not build a heavy contribution dashboard, bespoke story reader, or Autoradio surface when the VText app already owns reading/editing and TTS/STT work is still a later horizon. Plan for a later choir.news feed of published user-owned VTexts. Do not require clustering or embeddings in the first architecture pass; defer them until the processor/reconciler loop is proven. Do not run every style over every story by default; VText agents should select, rank, mix, compose, withhold, or let users customize Style.vtexts based on story fit, audience, source state, publication need, and user context. Delete or replace architecture/UI/data paths that encode the wrong object, including one-result refresh bottlenecks, frontend-only preview authority, shallow style tabs, redundant artifact panels, fake story object classes, and standalone researcher/writer implementations that bypass existing agents. Preserve these invariants: every story/article is a normal editable VText; user edits/forks/contributions/publications are user-owned and never mutate platform stories; platform corrections/updates are ordinary new VText versions through explicit candidate/review/version records; Style.vtext is a citeable/selectable/composable/replaceable source artifact; projections preserve evidence and cite per-version source/style lineage; news remains non-oracle with uncertainty, contrary evidence, source standing, gaps, corrections, and change history inspectable; durable VText indexes are accelerators over VTexts/versions/transclusions and do not replace live processor/reconciler cognition or VText-native provenance; existing researcher and VText agents are reused unless a documented invariant proves they cannot serve the role; all views work in Futuristic Noir, Carbon Fiber Kintsugi, and London Salmon; product-path/staging proof is required before claiming behavior. Use staging/product-path proof for source volume, processor/reconciler behavior, existing researcher/VText agent reuse, ownership boundaries, Style.vtext quality/routing, readable UI, and deployed commit identity. Update this mission doc with checkpoint/resumption state before stopping.
```

## Why This Mission Exists

The earlier Global Wire trajectory was under-specified at the most important
layer: what happens between a high-volume source firehose and a
publication-quality article.

The wrong object is:

```text
user clicks refresh
-> one or a few source results
-> deterministic signal
-> many UI panels
-> shallow style projection
```

The right object is:

```text
SourceMaxx ingestion
-> long-running processors with live understanding
-> reconcilers that see cross-story links and conflicts
-> existing researchers for targeted evidence work
-> existing VText agents writing with deep Style.vtext
-> VTexts with per-version source/style provenance
-> lightweight VText traversal/source indexes
-> readable publication surface
```

This is not a cosmetic mission. It is an architecture reset. Deletion is part
of the work when existing paths encode the wrong newsroom object.

## Cognitive Transform Set

Current obstacle: the product has been treated like a data surface and graph
slice, while the missing system is a live newsroom cognition loop.

Selected transforms:

1. **Depth extraction:** the banal object is "news app with sources." The deep
   object is a source-maxxed newsroom runtime whose live agents maintain
   understanding and publish through VText.
2. **Resident cognition:** processors should not wake up stateless and rebuild
   context from text every cycle. They should preserve hot context/KV cache,
   compact when needed, and carry handles to full source content.
3. **Role naming as architecture:** processors may span categories, regions,
   event families, source classes, or load-balanced firehose slices. The name
   must describe source processing, not a fixed beat boundary.
4. **Corpus reconciliation:** reconcilers are not downstream processors. They
   range over the story corpus: existing published VTexts, active platform
   VTexts, authorized user-owned VTexts, source state, processor notes,
   contradictions, questions, and update history.
5. **Reuse before invention:** Choir already has researcher and VText agents.
   SourceMaxx should route work into those agents instead of creating parallel
   evidence or writing systems.
6. **Style as editorial instrument:** `Style.vtext` is not a tab or prompt
   string. It is a citeable editorial source artifact selected because it fits
   the story and publication need.
7. **Subtractive product design:** a clean newspaper column over real source
   breadth is more correct than a busy dashboard exposing repeated internal
   artifacts.

Changed route:

- Build source volume plus processor and reconciler contracts before UI
  expansion, while keeping reconcilers corpus-oriented rather than
  fetch-cycle-oriented.
- Reuse existing researcher and VText agents as the evidence and writing path.
- Treat durable VText indexes as navigation/query accelerators over VTexts,
  versions, sources, and transclusions, not the authority for provenance or the
  whole intelligence.
- Prove readable publication output, not just API record creation.

## Real Artifact

The artifact is an AI newsroom runtime inside Choir:

```text
source registry and fetch loops
-> SourceItem ledger
-> simple routing
-> long-running processors
-> processor compaction chain and source handles
-> reconcilers over the story corpus
-> existing researcher requests/results
-> existing VText write/revision requests
-> article/story VTexts and Style.vtext projections
-> per-version source/style provenance inside VText
-> durable VText traversal/source indexes
-> clean Global Wire newspaper/workbench
-> user-owned forks, edits, contributions, and publications
```

The mission is incomplete if any core edge is missing: source intake,
processors, reconcilers, researcher reuse, VText reuse, Style.vtext routing,
ownership boundaries, or readable publication output.

## Value Criterion

Move Choir closer to a live, source-grounded, publication-quality newsroom
while preserving VText ownership, provenance, non-oracle news, and staging
proof.

The product moves uphill when:

- source intake approaches hundreds of SourceItems per 15 minutes and has a
  credible path to faster/live ingestion;
- SourceItems are durable, deduped, source-tagged, time-tagged, and linked to
  fetch provenance;
- processors receive routed source batches and maintain high-context
  understanding across turns;
- processor compaction preserves source handles, unresolved questions, watch
  items, active developments, and prior judgments;
- reconcilers inspect the story corpus, published VTexts, active platform and
  user-owned VTexts where authorized, processor notes, source state,
  contradictions, consensus, drift, and questions;
- processors, reconcilers, and VText agents can request existing researcher
  agents for additional evidence;
- existing VText agents write or revise normal VTexts from processor/reconciler
  notes, research packets, and matched Style.vtext artifacts;
- `Style.vtext` routing is selective and explainable rather than all styles
  over all stories;
- publication output is genuinely readable and useful;
- the app presents news in clear columns, not nested panels or repeated cards;
- staging proof shows the actual product path, not local-only or test-only
  behavior.

## Hard Invariants

- Every story remains a normal editable VText.
- User edits, forks, contributions, and publications are user-owned and never
  mutate platform stories directly.
- Platform corrections/updates are ordinary new VText versions through
  explicit candidate/review/version records.
- `Style.vtext` is a citeable, selectable, composable, replaceable source
  artifact.
- Style projections preserve evidence and cite source/style lineage within the
  produced VText version.
- News is non-oracle: uncertainty, contrary evidence, source standing, and
  change history remain inspectable.
- VText versions are the provenance-bearing objects. Sources are per-version,
  not per-VText. Durable indexes may accelerate VText graph walking, but they
  do not replace VText-native provenance or live processor/reconciler
  cognition.
- Existing researcher agents and existing VText agents must be reused unless a
  documented invariant proves they cannot serve the role.
- All app views work in Futuristic Noir, Carbon Fiber Kintsugi, and London Salmon.
- Product-path/staging proof is required before claiming behavior.

## Architecture

### Deterministic Substrate

Deterministic code owns scale, identity, provenance, routing, and mutation
boundaries.

Responsibilities:

- source registry for GDELT, many RSS feeds, many Telegram feeds, search
  providers, and curated/provider sources;
- scheduled fetch cycles and faster/live paths where providers allow;
- rate limits, backoff, provider health, and fetch-run metrics;
- SourceItem persistence with canonical URL/content refs, source IDs, source
  type, publisher/channel, timestamps, raw and cleaned content refs, hashes,
  standing metadata, and policy metadata;
- exact and safe dedupe while preserving echo/duplicate evidence when useful;
- simple routing by source, topic hints, geography, language, source class,
  event family, and load budget;
- durable request/result records for processors, reconcilers, researchers, and
  VText agents;
- ownership, native VText versioning, rollback refs, and publication indexes.

Clustering and embeddings are not required for the first pass. They are later
realism axes once the processor/reconciler loop is proven.

### Processors

Processors are long-running agents that receive source batches and maintain
live understanding. They are not necessarily vertical-specific.

A processor may cover:

- a broad topic area;
- several related verticals;
- a geography;
- a source class;
- a developing event family;
- a load-balanced slice of the firehose.

Processors should:

- keep hot context/KV cache across source batches;
- ingest new SourceItems with source handles, not pasted untraceable blobs;
- update their live understanding of active developments;
- maintain watch items, unresolved questions, changed beliefs, and candidate
  story/update briefs;
- request existing researcher agents when evidence is missing, contradictory,
  high-risk, or publication-sensitive;
- request existing VText agents when a story should be written or revised;
- compact when context pressure rises, preserving source handles and the
  current cognitive state needed for continuation.

Processor compaction is not a generic summary. It is an agent-owned continuity
artifact with handles to full source content, prior compactions, active briefs,
research requests, VText requests, and unresolved questions.

### Reconcilers

Reconcilers are corpus-level story agents, not a stage after processors.

They range over active and historical story state:

- existing published VTexts;
- active platform VTexts;
- authorized user-owned VTexts, including published user versions;
- processor notes/briefs and source handles;
- researcher evidence packets;
- current relationship/index records;
- unresolved contradictions and open questions.

They ask:

- What connects across existing stories?
- What contradicts or has drifted since publication?
- What old story needs an update because new source flow changed public
  meaning?
- What published story should be linked, split, corrected, or followed up?
- What story needs more research before publication?
- What question should a processor, researcher, or VText agent pursue next?
- What new insight or perspective deserves its own VText?

Reconcilers may request existing researcher agents and existing VText agents.
They also write durable relationship/question/contradiction notes so the News
app and other agents can expose their work.

Processors and reconcilers use the same underlying agentic loop as other Choir
agents. They are role/prompt/capability specializations with durable state,
tool calls, compaction, continuation, and channel/request records, not separate
harnesses.

### Existing Researcher Agents

Researchers remain Choir's bounded evidence workers.

Processors, reconcilers, and VText agents may ask researchers to:

- verify a claim;
- find missing sources;
- inspect contradictory accounts;
- evaluate source standing;
- gather context;
- prepare an evidence packet for a VText agent.

Researchers return source-backed evidence packets. They do not own final
article voice.

### Existing VText Agents

VText agents remain Choir's writers/editors.

They receive:

- processor notes/briefs;
- reconciler notes/briefs;
- researcher evidence packets;
- current VText handles when revising;
- matched `Style.vtext` artifacts;
- user/publication context.

They produce or revise ordinary article/story VTexts. A processor note,
reconciler note, or researcher packet may itself be a VText and may become the
v0/v1 seed for the article. They may request additional research when the brief
is too thin, contradictory, or risky.

### VText Traversal And Source Indexes

Use indexes to make VText-native provenance and transclusion paths searchable
and readable without creating a second authority:

- VText id and version id;
- per-version source refs and multimedia transclusions;
- VText-to-VText transclusions and links;
- `Style.vtext` citations and compositions;
- processor/reconciler/research VTexts where they exist;
- contradiction/update/timeline/related-story refs derived from VTexts and
  reconciler notes;
- publication state and user-owned published versions.

Do not make "StoryGraph authority" a vague abstraction. The concrete rule is:
agents write VText notes, candidate VText updates, and index records; canonical
platform stories change only through explicit VText version/update paths.
Corrections and updates are normal new VText versions, not special correction
objects. Source and style provenance lives inside each VText version; indexes
only point at it.

Future Autoradio likely needs this index because audio traversal means walking
a path through VText graph space and turning that path into one fluid narrative.
Do not build Autoradio in this mission; TTS/STT model exploration and narrative
path rendering are separate later work.

### Deep Style.vtext And Style Routing

`Style.vtext` artifacts must be publication-grade editorial sources, not short
prompt snippets.

A high-quality Style.vtext includes:

- editorial purpose and audience;
- voice principles;
- structure and section patterns;
- evidence and citation rules;
- uncertainty/correction/source-standing rules;
- examples of strong output;
- anti-patterns;
- revision policy;
- applicability metadata;
- "do not use" cases;
- composition/replacement rules;
- projection evaluation criteria.

Style routing asks: which style source or style composition should guide this
VText, if any?

Default behavior must not run every style over every story. The system should
select, rank, compose, or withhold styles based on story domain, audience,
source state, publication need, user/publication context, and evidence risk.
Users can request a different style, customize a style, or create a new
`Style.vtext` that their VText agents can use going forward.

### News App Redesign

Global Wire should read like a clean newspaper/workbench and should transclude
VTexts rather than replacing the VText app:

- front page columns, not card walls;
- no nested scrolling panels;
- no repeated display of the same limited information;
- no story border lines, boxes, or grid rules; text, whitespace, and section
  rhythm provide structure;
- primary scan shows headline, live change, source breadth, and tension;
- source chronology is available in reverse chronological order with filters
  by source class, vertical/topic hints, geography, and later search;
- full reading, editing, forking, style changes, and user publication happen
  by opening the VText in the VText app;
- evidence, style, VText traversal, processor/reconciler notes, and research
  detail are progressive disclosure, not dashboard panels;
- controls are compact and predictable;
- all three themes preserve readability and capability.

Do not build a contribution surface inside the News app for the first slice.
Contributions are user-owned VText/source/style artifacts. The platform should
eventually surface a feed of user-published VTexts on `choir.news`, but that is
not the first UI priority unless it becomes the highest-value proof.
Do not build Autoradio in this mission; keep only the architectural horizon for
future VText-path narration.

Browser/Computer Use screenshots are required for visual proof.

## Deletion And Replacement Targets

Audit and delete, replace, or quarantine:

- one-result or click-time source-refresh bottlenecks;
- frontend-only preview authority that looks like product truth;
- repeated artifact panels;
- nested scroll containers inside panels;
- card-heavy layouts that prevent news scanning;
- shallow style tabs;
- projection matrices that run all styles over all stories;
- deterministic single-signal source review paths that erase processor context
  or reconciler questions;
- new standalone researcher or writer systems that bypass existing agents;
- tests that prove singleton/button behavior when the product requires
  high-volume processor/reconciler behavior.

Follow problem-documentation-first before fixing newly discovered deployed
behavior problems. Planned deletion from this mission should be recorded in the
mission checkpoint or commit message.

## Homotopy Axes

Increase realism along these axes:

- **Source volume:** a few feeds -> many feeds -> hundreds per 15 minutes ->
  faster/live where feasible.
- **Source diversity:** one provider -> GDELT/RSS/Telegram/search -> curated
  domain/source-class sets.
- **Freshness:** manual refresh -> scheduled batches -> shorter cadences ->
  live events where supported.
- **Processor continuity:** stateless batch handling -> long-running processors
  -> hot-context/KV-cache preservation -> compaction chains with source refs.
- **Reconciler realism:** no corpus role -> existing-article review ->
  contradiction/consensus/question records -> research and VText requests.
- **Research reuse:** ad hoc evidence notes -> existing researcher requests ->
  source-backed evidence packets used by VText agents.
- **VText reuse:** app-local prose -> existing VText agent write/revise
  requests -> normal article/story VText versions.
- **Style depth:** short style source -> publication-quality Style.vtext ->
  composition/replacement/revision.
- **Style routing:** manual choice -> story-fit ranking ->
  select/compose/withhold -> user/publication override with provenance.
- **UI readability:** panel wall -> newspaper columns -> responsive
  publication/workbench across themes.
- **Product proof:** local checks -> product-path API -> staging source volume
  -> browser screenshots -> deployed acceptance record.

## Dense Feedback And Verifiers

Proof must include:

- source adapter and ingest tests;
- batch count, provider count, dedupe count, freshness, backoff, and error
  metrics;
- routing records showing SourceItems delivered to processors;
- processor evidence showing context continuity, compaction, and source handles;
- reconciler evidence showing connections, contradictions, questions, and
  researcher/VText requests;
- product evidence that existing researcher agents receive and answer requests;
- product evidence that existing VText agents write/revise normal
  article/story VTexts;
- Style.vtext routing proof showing fitting styles selected and non-fitting
  styles withheld or deprioritized with reasons;
- ownership proof that user edits/forks remain user-owned;
- browser screenshots across Futuristic Noir, Carbon Fiber Kintsugi, and London Salmon;
- staging commit identity and deployed product-path acceptance.

Use browser-public product APIs only for product proof. Do not use internal,
test-only, or raw mutation endpoints.

## Forbidden Shortcuts

- Do not claim SourceMaxx by increasing `max_results` on a click-time refresh.
- Do not use frontend seed data as product truth.
- Do not require clustering or embeddings before the processor/reconciler loop
  is proven.
- Do not create new researcher or writer agent systems when existing
  researcher and VText agents should be reused.
- Do not run all styles over all stories by default.
- Do not flatten sources into untraceable processor summaries.
- Do not hide provenance to make the UI clean.
- Do not add more panels to compensate for unclear architecture.
- Do not ship generic assistant prose as publication-quality output.
- Do not let user edits mutate platform stories.
- Do not claim staging behavior from local-only evidence.

## Stopping Condition

Mark `complete` only when staging proves:

- high-volume source ingestion from multiple source classes;
- durable SourceItem provenance, dedupe, and routing;
- long-running processor behavior with hot-context continuity and compaction
  handles;
- reconciler behavior over the story corpus, including existing published
  VTexts and current source/processor state;
- existing researcher agent reuse for targeted evidence work;
- existing VText agent reuse for normal article/story VText writing/revision;
- VText traversal/source indexes over VTexts, versions, and transclusions;
- user-owned fork/edit behavior;
- deep Style.vtext artifacts and intelligent style routing;
- publication-quality VText output;
- readable newspaper-style Global Wire UI in all three themes;
- CI/deploy/staging identity and product-path acceptance evidence.

Use `checkpoint_incomplete` if useful progress lands but any requirement is not
proven. Use `blocked_incomplete` only after root-cause investigation,
alternative routes, cognitive transforms, and the smallest safe next probe are
recorded.

## Run Checkpoint & Resumption State

status: checkpoint_incomplete

last checkpoint: 2026-06-07 mission rewritten around SourceMaxx newsroom
runtime: processors, reconcilers, existing researcher reuse, existing VText
reuse, deep Style.vtext routing, and readable publication UI.

current artifact state: prior Global Wire slices exist and staging has some
Source Service-backed paths, StoryGraph/VText/projection/contribution records,
publication artifacts, newsletter ledgers, and a dense Global Wire app surface.
The current product direction is insufficient: source processing is too close
to manual/source-refresh semantics, the UI is too busy, processors/reconcilers
do not yet exist as product roles, and Style.vtexts are not yet deep
publication artifacts. A partial source-refresh batch experiment from the
superseded route is preserved in
`stash@{0}` named
`superseded-global-wire-source-refresh-batch-experiment-2026-06-07`.

what shipped: docs-only mission rewrite unless a later checkpoint says
otherwise.

what was proven: not yet run under this rewritten mission.

unproven or partial claims:

- hundreds of source items per 15 minutes;
- many GDELT/RSS/Telegram/search sources configured and observed on staging;
- processor contracts and long-running context continuity;
- processor compaction with handles to full source content;
- reconciler contracts and corpus-level contradiction/question behavior across
  existing stories, current stories, and new source state;
- reuse of existing researcher agents from processor/reconciler/VText requests;
- reuse of existing VText agents for article writing/revision;
- intelligent Style.vtext routing and withholding/deprioritization;
- publication-quality VText output;
- readable newspaper UI across all themes.

belief-state changes:

- SourceMaxx requires resident processors plus corpus-level reconcilers, not
  just indexes or refresh endpoints.
- VText transclusion/version structure is the implicit graph; explicit indexes
  are accelerators, not authority.
- Existing researcher and VText agents are required infrastructure to reuse.
- Style.vtext routing is editorial judgment, not exhaustive permutation.
- UI correctness depends on source breadth and readability, not more panels.

remaining error field:

- exact current sourcecycled staging configuration and source volume;
- source daemon/storage ability to handle hundreds of items per 15 minutes;
- processor/reconciler runtime contracts, same-loop tool use, request/result
  channels, and compaction policy;
- processor load budget and routing scheme;
- current researcher/VText agent invocation contracts for this workflow;
- deletion/reuse map for current Global Wire UI/source paths.

highest-impact remaining uncertainty: whether the deployed source system and
runtime can ingest high-volume GDELT/RSS/Telegram/search batches and feed
long-running processors with preserved context without deeper source daemon,
storage, or agent-runtime changes.

next executable probe:

1. Inspect `cmd/sourcecycled`, `internal/sources`, source storage, runtime agent
   role contracts, researcher invocation, VText invocation, and current Global
   Wire source paths.
2. Produce a deletion/reuse map for source-refresh, Global Wire UI, existing
   researcher, existing VText, and VText traversal/index paths.
3. Decide whether the stashed source-refresh batch experiment belongs as a
   narrow compatibility fix or should be discarded under the new architecture.
4. Implement or specify the smallest proof of high-volume SourceItems routed to
   processors, processor compaction handles, corpus-level reconciler review
   over existing stories plus new source state, researcher reuse, and VText
   reuse through product-safe paths.
5. Redesign Global Wire into readable VText columns plus source chronology and
   per-article VText open affordances, then verify with browser screenshots
   across desktop and responsive Choir web desktop layouts.

suggested resume goal string: use the Goal String section above.

evidence artifact refs: none yet for this rewritten mission.

rollback refs: prior branch/worktree state before this mission;
`stash@{0}` named
`superseded-global-wire-source-refresh-batch-experiment-2026-06-07` preserves
the abandoned source-refresh batch edits; behavior commits must record their
own rollback SHAs.
