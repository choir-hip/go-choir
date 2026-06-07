# Choir Global Wire / Style.vtext Dual Object Spec - 2026-06-07

**Status:** product/architecture spec for the next mission  
**Scope:** live news ingestion, StoryGraph, Story VTexts, Style.vtext
projections, collaborative user-owned versions, and the News app surface.

## Purpose

Choir Global Wire and `Style.vtext` should not be separate tracks. The shared
object is a collaborative story system:

```text
24/7 source ingestion
-> StoryGraph
-> Story VTexts
-> Style.vtext projections
-> News app views
-> user-owned edits/contributions
-> research/reconciliation path
-> newsletters / researchers / Autoradio
```

The goal is not to make a news feed with style tabs. The goal is to represent
public reality as source-grounded, editable, multiperspectival VTexts where
style is an authored source artifact and every story can be read, traced,
edited, forked, cited, and eventually reconciled.

## Core Invariants

- Every story is a normal VText. The News app may add story/news affordances,
  but the story must remain viewable and editable through ordinary VText
  semantics.
- User edits do not mutate the platform story. They create user-owned VText
  versions/forks that can be published by the user.
- Platform story mutation requires a platform process. Later merge and
  reconciliation can promote or merge user contributions, but this mission must
  not silently rewrite canonical/public stories from arbitrary user edits.
- Stories cite raw sources, sourcecycled items, web-search evidence, other
  Story VTexts, user arguments/notes, and `Style.vtext` artifacts.
- `Style.vtext` is a source artifact, not hardcoded app config. It can be
  selected, replaced, composed, merged, hybridized, forked, published, or
  permissioned.
- News is non-oracle. It represents claims, counterclaims, uncertainty, source
  standing, evidence gaps, corrections, and changes over time.
- Graph nodes are story headlines / Story VTexts by default. Sources, claims,
  entities, and timelines are overlays, not the default graph object.
- Node neighborhoods communicate shared/similar sourcing and story family.
  Node color communicates recency/change state. Node size communicates
  prominence, source density, retrieval demand, or editorial weight.
- All UI views must work in all Choir themes: Future Noir, Carbon Kintsugi, and
  London Salmon. Theme is a user choice, not a product-mode decision.
- Low-resolution implementation must preserve the final topology. It may have
  fewer sources/styles/views, but it must not skip ingestion, StoryGraph,
  Story VText, style projection, VText edit/fork semantics, contribution path,
  or app view entirely.

## Product Object Model

### SourceItem

Normalized evidence from sourcecycled, web search, uploaded/user-provided
sources, or imported external material.

Minimum fields:

- stable id;
- source id / source class;
- canonical URL or content ref;
- title, author/publisher, timestamp;
- raw snapshot / cleaned text pointer;
- policy/standing metadata;
- fetch/provenance metadata.

### StoryGraph

The evidence and relationship object behind one or more Story VTexts.

Minimum fields:

- story id;
- headline/current title;
- source manifest with lead, supporting, contrary/qualifying, and ambient
  context tiers;
- claim set with uncertainty and dispute state;
- related Story VText refs;
- source overlap / citation / contradiction / update edges;
- timeline of material changes;
- prominence and freshness metadata;
- contribution/research queue refs.

### Story VText

A readable/editable VText projection over a StoryGraph.

Types:

- `PlatformStory.vtext`: platform/public story projection.
- `ProjectionStory.vtext`: style-specific projection over the same StoryGraph.
- `UserStory.vtext`: user-owned fork/edit/projection, publishable by user.
- `CounterStory.vtext`: user or publication argument that extends, disputes,
  or reframes an existing story.

### Style.vtext

An authored style source used to guide VText generation/revision.

It may represent:

- wire/news voice;
- legal-risk analysis;
- market/investor brief;
- policy/institutional brief;
- historical context;
- skeptical claim audit;
- publication voice;
- user/client/private voice;
- hybrid/composed style.

`Style.vtext` should be cited by any projection it materially shapes.

### Story Projection

Projection is a relation, not a fake persona:

```text
StoryGraph + Style.vtext + audience/task context -> Story VText
```

Allowed variation:

- opening frame;
- ordering of emphasis;
- domain vocabulary;
- rhetorical rhythm;
- which uncertainties are foregrounded;
- what counts as salient.

Not allowed:

- inventing facts;
- hiding material contrary evidence;
- losing source provenance;
- presenting one style as an oracle;
- impersonating real people without explicit authority.

## Collaboration Model

Users can contribute:

- sources;
- counter-sources;
- claims;
- disputes;
- arguments;
- context;
- style projections;
- edits / rewritten versions.

Contribution flow:

```text
user contribution
-> user-owned VText/source artifact
-> research task or review note
-> StoryGraph evidence/contribution queue
-> possible platform reconciliation later
```

The user contribution may improve their own published version immediately. It
does not automatically become the platform story.

## News App Views

The News app is a VText-centered collaborative story system with news
affordances.

Required views:

- Front Page: a live newspaper/broadsheet view of current Story VTexts.
- Story Reader: normal VText story view with news metadata.
- Story Editor/Fork: user-owned editing path from a platform story.
- Evidence/Trace: source manifest, lead/supporting/contrary/context tiers,
  related Story VTexts, and change history.
- Story Graph: story-headline node graph with source-neighborhood semantics.
- Style Projection Switcher: select/replace/compose `Style.vtext` sources and
  compare projections.
- Contribution Surface: add source, dispute point, make argument, request more
  research, publish user version.
- Autoradio/Ask Choir hooks: ask about the story, play/read projections, and
  follow related story neighborhoods.

All views must render in all three themes. Theme changes should not remove
features, hide evidence, or alter ownership semantics.

## Graph Semantics

Default graph:

- node = Story VText / headline;
- edge = shared/similar source basis, citation, update relation,
  contradiction, or claim overlap;
- neighborhood = story family/source neighborhood;
- color = recency/live-change state;
- size = prominence/source density/retrieval demand/editorial weight;
- outline/badge = tension, contradiction, claim changed, source-quality issue.

Overlays:

- sources;
- claims;
- entities;
- timeline events;
- styles/projections;
- user contributions;
- research tasks.

The graph is not a node-and-edge ornament. It is a navigable story topology.
Readable headline neighborhoods matter more than force-directed purity.

## Theme Requirement

The same app and all depth views must work in:

- Future Noir;
- Carbon Kintsugi;
- London Salmon.

Themes may change typography, color, density, contrast, and mood. They must not
change product capability. A user may read the newspaper view in Future Noir,
inspect the graph in London Salmon, or edit a user fork in Carbon Kintsugi.

## 24/7 Ingestion And Publication Loop

Target loop:

```text
source registry
-> fetch cycles / live source ingestion
-> normalized SourceItems
-> dedupe and source standing
-> claim/event/entity extraction
-> story clustering
-> StoryGraph update
-> projection jobs
-> Story VText revisions
-> publication/update feed
-> user contributions and research tasks
```

A new SourceItem should not blindly rewrite every story. The system should
classify the update:

- no visible change;
- source manifest update;
- claim changed;
- contradiction added;
- related story edge added;
- projection revision required;
- front-page prominence changed.

## Style.vtext Evaluation Through News

News gives `Style.vtext` a strong qualitative evaluation arena because the same
evidence graph can be projected through multiple styles.

Eval questions:

- Did each projection preserve the same evidence?
- Did each projection cite the same StoryGraph/source manifest honestly?
- Did style change framing, salience, rhythm, and judgment without inventing
  facts?
- Did the system avoid oracle voice?
- Did the projections actually differ, or did they all sound like the same
  assistant?
- Did the user contribution/edit path produce useful `Style.vtext` revision
  proposals without silently mutating style artifacts?

## Implementation Trajectory

The next mission should start at the lowest honest resolution of the whole
object and keep increasing resolution until the product is ship-worthy or a
true blocker is found.

Resolution axes:

- Source volume and source quality.
- StoryGraph durability and claim/tension richness.
- Story VText normal rendering, editing, versioning, and publication.
- `Style.vtext` selection, citation, composition, and revision proposals.
- Projection variety and evidence preservation.
- Contribution/research/reconciliation readiness.
- News app depth views across all themes.
- Product-path and staging evidence.

## Non-Goals For The First Mission

- Full final merge/reconciliation UX.
- Real-person impersonation.
- Detector optimization.
- Theme-specific feature divergence.
- Graph as decorative visualization without story semantics.
- User edits directly mutating the platform story.
- Skipping VText semantics to ship a bespoke news-card renderer.

