# Choir Global Wire / Style.vtext Newsroom Runtime Spec - 2026-06-07

**Status:** product/architecture spec for SourceMaxx Global Wire and
publication-quality `Style.vtext`.
**Scope:** high-volume source ingestion, long-running processors,
reconcilers, existing researcher agents, existing VText agents, Story VTexts,
deep Style.vtext routing, user-owned versions, graph lineage, and the News app
surface.

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
-> Story VTexts shaped by Style.vtext
-> durable graph/source/research/lineage records
-> readable News app
-> user-owned edits, forks, contributions, and publications
```

The goal is not a news feed with style tabs. It is a live publication system
where sources, processors, reconcilers, researchers, VText agents, Style.vtexts,
and users collaborate without losing ownership, provenance, or editorial
quality.

## Core Invariants

- Every story is a normal VText. The News app may add news affordances, but the
  story must remain viewable and editable through ordinary VText semantics.
- User edits do not mutate the platform story. They create user-owned VText
  versions/forks that the user can publish.
- Platform story mutation requires explicit versioned update/review records.
  No source, processor, reconciler, researcher, VText agent, or user edit may
  silently rewrite a canonical story.
- Stories cite raw sources, sourcecycled items, web/search evidence, other
  Story VTexts, processor/reconciler/research artifacts where relevant, user
  artifacts, and `Style.vtext` artifacts.
- `Style.vtext` is a source artifact, not hardcoded app config. It can be
  selected, replaced, composed, merged, hybridized, forked, published, or
  permissioned.
- News is non-oracle. It represents claims, counterclaims, uncertainty, source
  standing, evidence gaps, corrections, unanswered questions, contradictions,
  and changes over time.
- Processors and reconcilers are live cognition roles. Durable graph records
  preserve the projection and lineage of their work; graph records do not
  replace the live agent roles.
- Existing researcher agents are reused for bounded evidence work.
- Existing VText agents are reused for article writing and revision.
- Graph nodes are story headlines / Story VTexts by default. Sources, claims,
  entities, processor artifacts, reconciler artifacts, and timelines are
  overlays.
- All UI views must work in Future Noir, Carbon Kintsugi, and London Salmon.
- Low-resolution implementation must preserve topology: source ingestion,
  processors, reconcilers, researcher reuse, VText agent reuse, Style.vtext
  routing, Story VTexts, user ownership, graph lineage, and app views.

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

Reconcilers are live agents that bridge processor outputs.

They look for:

- cross-processor story links;
- contradictions;
- duplicate or overlapping developments;
- missing evidence;
- questions that need research;
- stories that need linking, updating, or splitting;
- stories that need VText treatment because the public meaning changed.

Reconcilers may request existing researcher agents and existing VText agents.
They also write durable reconciler artifacts so the graph and News app can show
relationships, contradictions, questions, and follow-up work.

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
- current Story VText handles when revising;
- matched `Style.vtext` artifacts;
- user/publication context.

They produce or revise normal VTexts:

- `PlatformStory.vtext`: platform/public story projection.
- `ProjectionStory.vtext`: style-specific projection over the same evidence.
- `UserStory.vtext`: user-owned fork/edit/projection, publishable by user.
- `CounterStory.vtext`: user or publication argument that extends, disputes,
  or reframes an existing story.

VText agents may request additional research when the brief is too thin,
contradictory, or risky.

## Durable Graph And Lineage

The durable graph is not the whole intelligence. It is the product-visible
lineage and relationship layer over source, processor, reconciler, researcher,
VText, Style.vtext, and user artifacts.

Default graph:

- node = Story VText / headline;
- edge = source overlap, citation, update relation, contradiction, claim
  overlap, reconciler link, or publication/update lineage;
- neighborhood = story family/source relationship;
- color = recency/live-change state;
- size = prominence/source density/retrieval demand/editorial weight;
- outline/badge = tension, contradiction, source-quality issue, research
  pending, or processor/reconciler attention.

Overlays:

- sources;
- processor briefs;
- reconciler questions/links;
- researcher evidence packets;
- claims;
- entities;
- timelines;
- styles/projections;
- user contributions;
- research tasks.

Canonical platform stories change only through explicit versioned update paths.
User-owned VTexts can diverge immediately without becoming platform truth.

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
-> Story VText
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

## Collaboration Model

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
-> graph evidence/contribution queue
-> possible platform reconciliation later
```

The user contribution may improve the user's own published version immediately.
It does not automatically become the platform story.

## News App Views

The News app is a VText-centered newsroom surface.

Required views:

- **Front Page:** readable newspaper/broadsheet columns of current Story VTexts.
- **Story Reader:** normal VText story view with news metadata and provenance.
- **Story Editor/Fork:** user-owned editing path from a platform story.
- **Evidence/Trace:** source manifest, processor/reconciler/research lineage,
  related Story VTexts, and change history.
- **Graph:** story-headline node graph with source/reconciler relationship
  semantics.
- **Processor/Reconciler View:** inspect live processor/reconciler outputs,
  questions, and requested research/VText work without turning the front page into a
  dashboard.
- **Style Routing View:** inspect selected/composed/withheld `Style.vtext`
  choices and reasons.
- **Contribution Surface:** add source, dispute point, argument, request more
  research, or publish user version.
- **Autoradio/Ask Choir hooks:** ask about the story, play/read projections,
  and follow related story neighborhoods.

UI requirements:

- front page columns, not card walls;
- no nested scrolling panels;
- no repeated display of the same limited information;
- details by progressive disclosure;
- evidence visible without overwhelming the reading surface;
- all views render in Future Noir, Carbon Kintsugi, and London Salmon.

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
-> reconcilers connect/conflict/question across processors
-> existing researchers answer targeted evidence requests
-> existing VText agents write/revise using Style.vtext
-> Story VTexts and publication/update feed
-> user contributions and reconciliation queues
```

A new SourceItem should not blindly rewrite every story. The system should
classify or route the update into work such as:

- no visible change;
- processor watch update;
- research needed;
- contradiction/question for reconciler;
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

- Did reconcilers find meaningful cross-story connections?
- Did they surface contradictions and open questions?
- Did they request research when evidence was insufficient?
- Did they create useful related-story/update records?

### Researcher Evaluation

- Were existing researcher agents reused?
- Did research packets preserve source provenance?
- Did research answer the requested question without taking over article voice?

### VText And Style Evaluation

- Were existing VText agents reused?
- Did Story VTexts remain normal editable VTexts?
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

Start at the lowest honest resolution of the whole object and increase realism
without changing topology.

Resolution axes:

- source volume and source diversity;
- source freshness;
- processor routing and context continuity;
- processor compaction quality;
- reconciler connections/questions/contradictions;
- existing researcher reuse;
- existing VText reuse;
- graph/source/research/VText lineage durability;
- Style.vtext depth and routing;
- publication-quality prose;
- user-owned VText editing/forking/publication;
- clean newspaper UI across themes;
- product-path and staging evidence.

## Non-Goals For The First Mission

- Clustering or embeddings as a prerequisite.
- Full final merge/reconciliation UX.
- Real-person impersonation.
- Detector optimization.
- Theme-specific feature divergence.
- Graph as decorative visualization without story semantics.
- User edits directly mutating the platform story.
- Skipping existing researcher agents.
- Skipping existing VText agents.
- Skipping VText semantics to ship a bespoke news-card renderer.
