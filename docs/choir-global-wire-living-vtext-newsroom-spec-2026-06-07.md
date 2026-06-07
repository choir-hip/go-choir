# Choir Global Wire Living VText Newsroom Spec - 2026-06-07

**Status:** active architecture spec draft after visual/product review.  
**Supersedes:** the earlier source-volume-named mission/spec and the earlier broad-source
draft. Those files remain context; this is the intended object.

## Thesis

Global Wire is a living VText newsroom.

It is not a feed renderer, story database, source-list dashboard, style
projection demo, or outline generator. It is a source ingestion and agentic
publication system whose canonical article object is a normal VText owned by
the VText agent workflow.

The product exists to turn broad, multilingual, high-volume source flow into
publication-quality living articles whose evidence and related context are
native VText/source transclusions. It should pull from established outlets,
wire/event feeds, public institutions, specialist publications, community
surfaces, and long-tail social channels; source breadth is a reality sensor,
not a hardcoded authority model.

## The Correct Object

```text
broad multilingual source ingestion
-> durable SourceItems with native source embedding/transclusion handles
-> long-running processors maintaining evolving source understanding
-> existing researchers filling bounded evidence gaps
-> VText agents owning article creation and revision
-> reconcilers watching the article/source corpus for updates and tensions
-> living article VTexts with real prose, source transclusions, related-VText
   transclusions, citations, and version-local provenance
-> rebuildable indexes over VText/source graph state for discovery and speed
-> Global Wire newspaper surface as a collection view
-> normal VText app for full reading, editing, revising, forking, source
   traversal, and publication
```

## Non-Negotiable Corrections

### Articles Are Not Stubs

A generated outline, claim list, source manifest, metadata dump, or projection
section is not an article.

An article VText must contain developed prose with reporting structure,
meaningful source integration, and editorial judgment. It may include callouts,
source points, transcluded evidence, and related VText passages, but the main
object must read as an article.

### The Article Is Not v0 Unless v0 Is A Real Article

Processor notes, reconciler notes, research packets, and outlines may precede
an article, but they are not the article just because they can be stored as
VTexts. A finished article should not surface as "v0" unless that first version
is already a real publication-quality article.

If an early artifact is a brief, mark it as a brief or note. The article starts
when the VText agent produces article-quality prose.

### VText Agents Own Articles

VText agents are not a tool called by Global Wire to get article text. They are
the article owners.

Processors, reconcilers, researchers, and users can message/request VText
agents. The VText agent decides how to revise the document within its prompt,
tool policy, source handles, style sources, and ownership boundaries.

Article lifecycle should look like prompt-bar-normal VText work:

```text
intent / brief / evidence / update
-> VText agent turn
-> VText document/revision
-> native version/source/provenance state
```

not:

```text
Global Wire handler
-> generated text blob
-> article row
-> fake VText wrapper
```

### VText Is Already Editable

Do not add a Global Wire "My Edit" section or special edit object. VText is
natively editable and versioned.

User edits, forks, revisions, and publications are ordinary user-owned VText
flows. Platform articles are not mutated by user edits.

### Sources Must Be Transcluded

The existing source embedding/transclusion system is mandatory for news.

Sources should appear as native source points/transclusions that can open into
source viewer windows or source-specific views, preserving source handles,
snippets, media, timestamps, and version-local provenance.

A text-only "Source Manifest" list is insufficient. It can exist as an index
or debug/proof artifact, but it is not the article experience.

### Related VTexts Must Be Transcluded

Related VTexts should not be rendered as a bare list of links or IDs. If a
related VText matters editorially, the article should transclude the relevant
passage, note, update, or context block using VText-native mechanics.

Indexes may discover relationships; articles should present them through
transclusion.

### Stories Are Living Versioned Articles

Ongoing stories accumulate versions. New source flow should cause processors
and reconcilers to determine whether it:

- updates an existing article;
- contradicts or qualifies an existing article;
- requires a correction;
- requires a new article;
- requires research before publication.

Reconcilers message/request VText agents with update briefs. The VText agent
creates a new article version when appropriate.

## Source Breadth Is Priority 1

The current 14-source registry is not enough. Hundreds of items from one
global firehose plus a few feeds proves the ingestion substrate exists; it does
not prove a serious newsroom.

The first execution axis is broad, multilingual, policy-compliant source
coverage. The registry should include categories for observability and routing,
but those categories must not encode permanent trust tiers.

Required direction:

- keep GDELT or equivalent global event feed ingestion;
- expand RSS/Atom by region, language, beat, sector, community, and medium;
- expand Telegram/public-channel ingestion where allowed and policy-compliant,
  including long-tail local, regional, conflict, community, tech, finance, and
  social-sentiment channels that established outlets may ignore;
- include official/public institutional sources, local outlets, specialist
  sources, regional sources, financial/economic sources, crisis/conflict
  sources, science/health sources, industry sources, hacker/community sources,
  open-source/community sources, policy sources, labor sources, academic
  sources, trade publications, market sources, logistics/shipping sources,
  energy sources, agriculture sources, and culture/technology sources;
- include Hacker News and comparable technical community surfaces;
- include non-English technology, science, finance, industrial, regional, and
  specialist media, not only English general news;
- track active source count, successful source count, failed source count,
  per-source item count, provider latency, freshness, language, region, and
  descriptive source category;
- treat hundreds of SourceItems per 15 minutes as a floor, not the finish line;
- research source discovery before implementing a large registry expansion.
- do not hardcode source trust tiers or static source standing in the registry;
  source reliability is learned and reasoned from track record, corroboration,
  correction history, freshness, source behavior, researcher packets, and model
  context. Long-tail social sources are valuable evidence and sentiment, not
  automatic article support.
- use prompting to softly remind models of generally known source reputations
  when useful, but keep that reasoning visible, revisable, and subordinate to
  evidence, corroboration, and versioned corrections.

## Agent Roles

### Processors

Processors are long-running shared-harness agents that ingest routed
SourceItems and maintain evolving understanding. They may span topics,
regions, languages, descriptive source categories, events, communities,
sectors, or load-balanced slices.

They preserve source handles and full-content paths. They should keep hot
context where possible and compact only as continuity state, preserving source
handles, active developments, unresolved questions, watch items, research
results, VText requests, and prior judgments.

Processors may request researchers, VText agents, more source search/fetch, or
reconciler attention. They do not own canonical articles.

### Researchers

Researchers are existing Choir evidence agents. They answer bounded questions,
compare source accounts and track records, gather missing context, and return
source-backed packets. They do not own article voice.

### VText Agents

VText agents own article documents. They create and revise article VTexts from
briefs, research packets, source transclusion handles, related VText handles,
matched `Style.vtext` sources, and publication context.

They are responsible for publication-quality prose, source integration,
version-local provenance, and using native VText/source transclusion.

### Reconcilers

Reconcilers are corpus agents. They work over current platform article VTexts,
existing published VTexts, authorized user-owned/published VTexts, source
state, processor notes, researcher packets, transclusions, questions,
contradictions, corrections, and change history.

They produce notes, research requests, article-update requests, and new-article
prompts. They do not directly mutate articles.

## Style.vtext

`Style.vtext` is a citeable editorial source artifact.

VText agents choose style sources intelligently for the story and audience.
They may mix, compose, fork, or replace styles. Global Wire must not run every
style over every story by default.

Style should improve article quality, not produce visible metadata sludge.

## VText/Source Graph Index

The graph is implicit in VText markup, source transclusion, VText-to-VText
transclusion, citations, versions, source handles, publication records, and
ownership.

Indexes are rebuildable accelerators for discovery, performance, reconcilers,
search, source chronology, and future voice/radio traversal. They are not
canonical article state.

## Global Wire UI

Global Wire is the collection surface, not the article editor.

Required direction:

- readable newspaper columns;
- serif article headlines;
- professional article typography at normal desktop widths, wide desktop, and
  mobile-in-desktop-shell sizes;
- no huge sans headline blocks that collapse normal-width layouts;
- no dashboard panels or card grid;
- no nested panel scrolling;
- no border-line grid;
- source chronology as quiet provenance/breadth signal;
- compact VText affordance on every article;
- no repeated visible "Open in VText" label;
- no app-local theme selector;
- no special contribution/edit panel;
- mobile stays inside Choir desktop/web shell;
- VText mobile menu/banner buttons must not overlap labels.

The VText app must also render article documents professionally: no metadata
dump in the article body, no source manifest as plain text in lieu of
transclusions, no related VText bullet list, no "My Edit" section.

## Delivery Gates

Do not claim delivery until staging proof shows:

- significantly expanded multilingual source registry beyond 14 configured
  sources;
- latest cycle source count, active source count, failed source count,
  language/region coverage, and per-source item counts;
- GDELT/global events, RSS/Atom, Telegram/public-channel sources, Hacker News
  or comparable technical community sources, official/public sources,
  specialist sources, industry sources, finance sources, science sources, and
  regional/non-English sources actually running;
- processors and reconcilers as shared-harness profiles;
- researcher requests/results reused by processors, reconcilers, or VText
  agents;
- VText agents owning article creation/revision through normal VText flows;
- article VTexts with real prose, not outlines/stubs;
- source transclusions that open source viewer/source windows;
- related VText transclusions where relevant;
- ongoing-story update path: new information produces VText article revisions;
- intelligent `Style.vtext` use without visible metadata sludge;
- no Global Wire "my edits" subsystem;
- clean Global Wire UI across Futuristic Noir, Carbon Fiber Kintsugi, and
  London Salmon;
- desktop normal-width, wide desktop, and mobile-in-desktop-shell screenshots
  proving typography/layout;
- staging commit identity, CI/deploy status, and product-path acceptance
  evidence.

## Forbidden Shortcuts

- Do not rename source volume into a product layer.
- Do not count one high-volume feed as source diversity.
- Do not encode permanent source trust tiers or static standing in config.
- Do not turn long-tail Telegram/social sentiment into article support
  without corroboration, uncertainty, or research.
- Do not call outlines or manifests articles.
- Do not make metadata visible in article prose.
- Do not list sources or related VTexts where native transclusion is required.
- Do not make a Global Wire edit subsystem.
- Do not use VText as a text-generation helper while another system owns the
  article.
- Do not polish a fake article surface before fixing article ownership and
  source breadth, except for blocking readability/accessibility defects.
