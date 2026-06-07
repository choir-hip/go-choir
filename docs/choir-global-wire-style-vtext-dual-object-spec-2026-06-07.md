# Choir Global Wire / Style.vtext Newsroom Runtime Spec - 2026-06-07

**Status:** product/architecture spec for SourceMaxx Global Wire and
publication-quality `Style.vtext`.
**Scope:** high-volume source ingestion, long-running processors,
reconcilers, existing researcher agents, existing VText agents, story/article
VTexts, deep Style.vtext routing, user-owned versions, publication feeds,
VText traversal/indexing, and the News app surface.

## Purpose

Choir Global Wire is an AI newsroom runtime. It should ingest far more source
material than a human can read, preserve provenance, maintain live agent
understanding, and publish source-grounded editable VTexts.

The shared object is:

```text
SourceMaxx ingestion
-> durable SourceItems
-> long-running processors
-> reconcilers
-> existing researcher agents
-> existing VText agents
-> story/article VTexts shaped by Style.vtext
-> durable VText traversal/source indexes
-> readable News app
-> user-owned edits, forks, contributions, and publications
```

The goal is not a news feed with style tabs. It is a live publication system
where sources, processors, reconcilers, researchers, VText agents, Style.vtexts,
and users collaborate without losing ownership, provenance, or editorial
quality.

## Core Invariants

- Every story/article is a normal VText. Do not introduce a separate story
  object type, renderer, or ownership class.
- User edits do not mutate the platform story. They create user-owned VText
  versions/forks that the user can publish.
- Platform story correction/update is just a new version of the relevant VText
  through explicit candidate/review/version records.
  No source, processor, reconciler, researcher, VText agent, or user edit may
  silently rewrite a canonical story.
- Stories cite raw sources, sourcecycled items, web/search evidence, other
  VTexts, processor/reconciler/research VTexts where relevant, user artifacts,
  and `Style.vtext` artifacts.
- `Style.vtext` is a source artifact, not hardcoded app config. It can be
  selected, replaced, composed, merged, hybridized, forked, published, or
  permissioned.
- News is non-oracle. It represents claims, counterclaims, uncertainty, source
  standing, evidence gaps, corrections, unanswered questions, contradictions,
  and changes over time.
- Processors and reconcilers are live cognition roles. VText transclusion and
  version structure create the implicit graph. Any explicit index is a
  rebuildable accelerator over VTexts, versions, sources, and transclusions; it
  does not replace VText-native provenance or the live agent roles.
- Existing researcher agents are reused for bounded evidence work.
- Existing VText agents are reused for article writing and revision.
- Relationship nodes are story/article headlines by default. Sources, claims,
  entities, processor notes, reconciler notes, and timelines are overlays over
  VTexts and source records.
- All UI views must work in Futuristic Noir, Carbon Fiber Kintsugi, and London Salmon.
- Delivery implementation must preserve topology while raising resolution:
  source ingestion, processors, reconcilers, researcher reuse, VText agent
  reuse, Style.vtext routing, story/article VTexts, user ownership, VText
  traversal/indexing, publication feeds, and app views move as one product
  object.

## Product Runtime

### SourceMaxx Ingestion

Global Wire must be designed for high source volume:

- hundreds of SourceItems per 15 minutes as an early target;
- faster or live ingestion where providers and cost allow;
- GDELT, many RSS/Atom feeds, many Telegram feeds, search providers, and future
  curated/provider source sets;
- per-source and per-provider cadence, rate policy, backoff, and health;
- durable fetch-run metrics: started, finished, provider/source, item count,
  dedupe count, error count, latency, and freshness window.

Ingestion is deterministic infrastructure. It owns identity, provenance,
dedupe, rate limits, and source ledger integrity. It does not decide the final
article narrative.

### SourceItem

`SourceItem` is normalized evidence from sourcecycled, web/search providers,
uploaded/user sources, or imported external material.

Minimum fields:

- stable id;
- source id, source class, and provider;
- canonical URL or content ref;
- title, author/publisher/channel, timestamp;
- language/geography/topic hints where available;
- raw snapshot pointer;
- cleaned text/content pointer;
- content hash and dedupe metadata;
- source-standing/policy metadata;
- fetch-run/provenance metadata.

Duplicate or echoing items may still matter as source-standing/attention
evidence. The system should dedupe display and identity without erasing useful
echo evidence.

### Routing

Routing sends SourceItems to processors. It may use simple deterministic rules
at first:

- source class;
- source list;
- topic hints;
- geography;
- language;
- publisher/channel;
- event family;
- load budget.

Clustering and embeddings are explicitly not required for the first
architecture pass. They are later tools after the processor/reconciler loop is
working.

### Processors

Processors are long-running agents that maintain live understanding over
routed source flow. They are not necessarily vertical-specific.

Processors are shared-harness Choir agents. Their specialization is profile
prompt, model/tool policy, source/query capability, durable continuity state,
and source-batch inputs. They must reuse the same core loop, provider
semantics, run store, event stream, compaction/continuation mechanics,
delegation path, and worker-update path used by existing agent profiles unless
a later invariant proves shared-harness reuse cannot protect correctness,
security, authority boundaries, or resource isolation.

A processor may own:

- a broad topic area;
- multiple related verticals;
- a geography;
- a source class;
- a developing event family;
- a load-balanced firehose slice.

Processors receive SourceItem batches with handles to full source content. They
should keep hot context/KV cache across turns where the runtime permits.

Processors produce:

- processor briefs;
- active developments;
- changed beliefs;
- watch items;
- unresolved questions;
- source-backed claims or tensions;
- requests to existing researcher agents;
- requests to existing VText agents;
- compaction artifacts when context pressure requires it.

Processor compaction is not a generic text summary. It is a continuity artifact
that preserves:

- source handles;
- prior compaction handles;
- active developments;
- unresolved questions;
- research requests/results;
- VText requests/results;
- watch items;
- important prior judgments.

The durable artifact is recovery, audit, and handoff state. It should not force
every processor turn to reconstruct understanding from scratch.

### Reconcilers

Reconcilers are corpus-level story agents, not a downstream stage after
processors.

Reconcilers are also shared-harness Choir agents. They use a reconciler
profile and corpus/source/VText traversal toolset, not a separate runtime
architecture.

They review articles/stories and surrounding evidence across:

- existing published VTexts;
- current platform VTexts;
- authorized user-owned VTexts and published user versions;
- processor notes/briefs;
- source ledger state;
- researcher evidence packets;
- VText traversal/index records;
- open questions, contradictions, and change history.

They look for:

- consensus across pieces;
- contradictions within and between pieces;
- claims that drifted since publication;
- duplicate or overlapping developments;
- missing evidence;
- questions that need research;
- articles that need ordinary VText updates/corrections;
- new ideas, insights, or perspectives that should become new VTexts.

Reconcilers may request existing researcher agents and existing VText agents.
They write durable reconciler notes and relationship/question records so other
agents and the News app can expose connections without turning the News app
into a control dashboard.

### Existing Researcher Agents

Researchers are the existing Choir evidence agents. They should be reused, not
reimplemented.

Processors, reconcilers, and VText agents can ask researchers to:

- verify a claim;
- find missing sources;
- inspect source standing;
- compare contradictory accounts;
- gather context;
- produce an evidence packet for writing or review.

Researchers return source-backed evidence packets. They do not own article
voice or canonical story mutation.

### Existing VText Agents

VText agents are the existing Choir writing/editing agents. They should be
reused, not reimplemented.

VText agents receive:

- processor briefs;
- reconciler briefs;
- researcher evidence packets;
- current VText handles when revising;
- matched `Style.vtext` artifacts;
- user/publication context.

They produce or revise normal VTexts. Platform articles, user-owned forks,
published user versions, counterstories, and style-shaped projections are all
ordinary VTexts with ownership, publication, citation, and version metadata.

Processor notes, reconciler notes, and researcher packets should also use VText
where practical. A processor brief or reconciler note can become the early VText
version that the writing agent develops, rather than a parallel artifact that
must be copied into VText later.

VText agents may request additional research when the brief is too thin,
contradictory, or risky.

## VText Traversal And Indexing

The primary graph is implicit in VText markup, transclusion, native
Dolt-backed versions, per-version sources, multimedia source references,
`Style.vtext` citations, and links between VTexts. Build skill at walking
VTexts before introducing new graph-shaped authority.

An explicit index may be useful for performance and discovery. It should index
VTexts and VText versions, not merely extracted facts, because future surfaces
such as Autoradio will need to turn paths through VText graph space into a
single fluid narrative. Autoradio itself is beyond this delivery run; TTS/STT
model work has not started and should not distract SourceMaxx shipping.

Index design rules:

- key by VText id and version id;
- index per-version source transclusions and citations;
- index VText-to-VText transclusions and links;
- index `Style.vtext` citations and style composition;
- index processor/reconciler/research VTexts where they exist;
- index publication state and user-owned published versions;
- support fast neighborhood/path queries for reading and later audio traversal;
- remain rebuildable from VText/source state;
- never become the authority for version provenance, source provenance, or
  canonical text.

Relationship views may show source overlap, citation, update relation,
contradiction, claim overlap, reconciler link, publication/update lineage,
recency, source density, tension, research pending state, and related article
neighborhoods. Those are views over VText/index state, not separate truth.

Canonical platform articles change only through explicit VText version/update
paths. User-owned VTexts can diverge immediately and may also be published by
their owners without becoming platform truth.

## Deep Style.vtext

`Style.vtext` is an authored editorial source used by VText agents.

It may represent:

- wire/news voice;
- market/investor brief;
- policy/institutional brief;
- skeptical claim audit;
- legal-risk analysis;
- historical context;
- publication voice;
- user/client/private voice;
- hybrid/composed style.

A publication-quality `Style.vtext` should include:

- editorial purpose and audience;
- voice principles;
- structure and section patterns;
- evidence and citation rules;
- uncertainty, correction, and source-standing rules;
- examples of strong output;
- anti-patterns;
- revision policy;
- applicability metadata;
- explicit "do not use" cases;
- composition/replacement rules;
- projection evaluation criteria.

`Style.vtext` must be cited by projections it materially shapes.

## Style Routing And Projection

Projection is a relation:

```text
evidence + processor/reconciler/research context + Style.vtext + audience/task
-> VText
```

Do not run every style over every story by default.

The system should select, rank, compose, replace, or withhold styles based on:

- story domain;
- source state;
- audience/publication context;
- evidence risk;
- user preference;
- story maturity;
- urgency;
- whether the style is applicable.

VText agents may choose from a collection of `Style.vtext` sources, mix or
compose compatible styles, or withhold styles that do not fit the story. Users
may request a different style, customize a style, or create a new `Style.vtext`
that their VText agents can use going forward.

Allowed projection variation:

- opening frame;
- ordering of emphasis;
- domain vocabulary;
- rhetorical rhythm;
- which uncertainties are foregrounded;
- what counts as salient;
- amount of context or analysis.

Not allowed:

- inventing facts;
- hiding material contrary evidence;
- losing source provenance;
- presenting one style as an oracle;
- impersonating real people without explicit authority;
- rendering a non-fitting style just because it exists.

## Collaboration And Publication Model

Users can contribute:

- sources;
- counter-sources;
- claims;
- disputes;
- arguments;
- context;
- style artifacts or style revision proposals;
- edits / rewritten versions.

Contribution flow:

```text
user contribution
-> user-owned VText/source/style artifact
-> research task, processor note, or reconciler note
-> VText traversal/source index for discovery where needed
-> possible platform reconciliation later
```

The user contribution may improve the user's own published version immediately.
It does not automatically become the platform story.

The public `choir.news` platform should eventually surface a feed of published
user-owned VTexts. That feed is part of the publication object, even if this
delivery run prioritizes platform Global Wire ingestion, processing, and
readable article output before full user-publication discovery.

## News App Views

The News app is a focused VText-centered newsroom collection surface. It should
transclude VTexts rather than replacing the VText app. Focused does not mean
unfinished: the delivered surface should feel intentional, readable, and nice.

Required views:

- **Front Page:** readable newspaper/broadsheet columns of current VTexts.
- **Source Chronology:** reverse-chronological source feed with filters by
  source class, geography, topic/vertical hints, and later search.
- **VText open affordance:** every article opens in the normal VText app for
  reading, editing, forking, style changes, or user publication. Do not repeat
  `Open in VText` as visible label text for every article; use a small VText
  icon/glyph, click target, or contextual action.
- **Style/Provenance Disclosure:** compact access to selected/composed
  `Style.vtext` sources, per-version sources, and change history.

Deferred or optional views:

- VText traversal/relationship exploration;
- processor notes;
- reconciler notes;
- research queues;
- published user VText discovery feed.
- Autoradio/voice traversal after TTS/STT models and VText path narration are
  explored separately.

UI requirements:

- front page columns, not card walls;
- no nested scrolling panels;
- no repeated display of the same limited information;
- no borders, rules, or boxes around each story; text, whitespace, and section
  rhythm provide structure;
- details by progressive disclosure;
- evidence visible without overwhelming the collection surface;
- no contribution dashboard inside the News app; contribution and editing
  happen through VText/source/style flows;
- all views render in Futuristic Noir, Carbon Fiber Kintsugi, and London Salmon.

Theme changes may change typography, density, contrast, and mood. They must not
change product capability, hide evidence, or alter ownership semantics.

## SourceMaxx Publication Loop

Target loop:

```text
source registry
-> fetch cycles / live source ingestion
-> durable SourceItems
-> dedupe and routing
-> processors absorb source flow
-> processor compaction when needed
-> reconcilers review articles, source state, and live notes for consensus,
   contradictions, updates, and follow-up ideas
-> existing researchers answer targeted evidence requests
-> existing VText agents write/revise using Style.vtext
-> VTexts and publication/update feed
-> user-owned VTexts and later user-publication discovery
```

A new SourceItem should not blindly rewrite every story. The system should
classify or route the update into work such as:

- no visible change;
- processor watch update;
- research needed;
- contradiction/question for reconciler review;
- VText write request;
- VText revision request;
- related story link;
- front-page prominence change;
- publication update candidate.

## Evaluation

### SourceMaxx Evaluation

- How many SourceItems were ingested per 15-minute window?
- Which source classes were represented?
- What was deduped, and what echo evidence was preserved?
- What freshness window was achieved?
- Which processors received which source batches?

### Processor Evaluation

- Did the processor preserve useful context across turns?
- Did compaction retain source handles and unresolved questions?
- Did the processor avoid reconstructing from scratch unnecessarily?
- Did it request research or VText work at the right time?

### Reconciler Evaluation

- Did reconcilers review articles/stories, not just processor outputs?
- Did they find meaningful cross-story connections?
- Did they surface contradictions and open questions?
- Did they request research when evidence was insufficient?
- Did they create useful related-story/update records and VText update/new
  VText requests?

### Researcher Evaluation

- Were existing researcher agents reused?
- Did research packets preserve source provenance?
- Did research answer the requested question without taking over article voice?

### VText And Style Evaluation

- Were existing VText agents reused?
- Did every article/story remain a normal editable VText?
- Did projections cite source/style lineage honestly?
- Did `Style.vtext` materially improve framing, rhythm, salience, and judgment?
- Was style routing selective and explainable?
- Were non-fitting styles withheld or deprioritized?
- Did the prose reach publication quality rather than generic assistant
  summary?

### UI Evaluation

- Does the front page read cleanly as news?
- Are sources/provenance accessible without turning the page into a dashboard?
- Are there nested scroll panels or repeated card walls?
- Do all required views work across all three themes?

## Implementation Trajectory

Build topology, not ladder. Start with the whole product object at the highest
workable resolution, then raise realism along the axes below until staging has
delivered behavior or a true blocker is recorded. Do not ship an underpowered
demo, parallel demo, or isolated UI polish path.

Resolution axes:

- source volume and source diversity;
- source freshness;
- processor routing and context continuity;
- processor compaction quality;
- reconciler connections/questions/contradictions;
- existing researcher reuse;
- existing VText reuse;
- VText traversal/index durability;
- Style.vtext depth and routing;
- publication-quality prose;
- user-owned VText editing/forking/publication;
- clean newspaper UI across themes;
- product-path and staging evidence.

## Deferred Or Out Of Scope For This Delivery Run

- Clustering or embeddings as a prerequisite.
- Full final merge/reconciliation UX.
- Full published-user-VText discovery feed.
- Autoradio and TTS/STT model exploration.
- Real-person impersonation.
- Detector optimization.
- Theme-specific feature divergence.
- Decorative graph visualization without VText traversal semantics.
- User edits directly mutating the platform story.
- Skipping existing researcher agents.
- Skipping existing VText agents.
- Skipping VText semantics to ship a bespoke news-card renderer.
