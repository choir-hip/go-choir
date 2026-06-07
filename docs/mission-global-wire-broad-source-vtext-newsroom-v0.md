# Mission: Global Wire Broad-Source VText Newsroom

**Status:** replacement MissionGradient mission after architecture correction.  
**Spec:** `docs/choir-global-wire-broad-source-vtext-newsroom-spec-2026-06-07.md`  
**Supersedes:** `docs/mission-global-wire-sourcemaxx-publication-system-v0.md` as
the active target. The old mission remains historical evidence.
**Created:** 2026-06-07

## Goal String

```text
/goal Run docs/mission-global-wire-broad-source-vtext-newsroom-v0.md. Ship Global Wire as a broad-source VText newsroom: many-language source ingestion, processors/reconcilers, researcher reuse, VText-owned articles, source/VText transclusion, clean newspaper UI, staging proof.
```

## Architecture Correction

The old framing accidentally turned "get many more sources" into a product
name and then optimized around an index/stub path. That is the wrong object.

The corrected object is:

```text
broad source ingestion
-> durable SourceItems with source embeddings/transclusion handles
-> long-running processors with hot context and compaction
-> existing researchers for bounded source/evidence work
-> existing VText agents that own article creation/revision
-> reconcilers over the VText/source corpus
-> publication-quality article VTexts with transcluded sources and related VTexts
-> rebuildable VText/source graph indexes for discovery/performance
-> clean Global Wire newspaper collection view
-> normal VText app for reading, editing, revising, forking, and publishing
```

Global Wire is not an article database. It is not a stub generator. It is not a
dashboard. It is a news collection surface over normal VText publication.

## Cognitive Transform Set

Current obstacle: the system has repeatedly shipped surfaces and indices before
the source/runtime/article object was real enough.

Selected transforms:

1. **Name the real object:** this is a newsroom runtime whose canonical article
   object is VText, not a news card or story row.
2. **Load-bearing variable:** source breadth is priority #1. A clean UI over
   14 configured sources is still a toy; hundreds of items from one global feed
   plus a few feeds is not enough source diversity.
3. **Transclusion over listing:** if a source or related VText matters, it
   should be embedded/transcluded into the article experience. Lists are
   discovery aids, not the publication substrate.
4. **Ownership normalization:** VText agents own articles the same way prompt
   bar VText work does. Processors and reconcilers request VText work; they do
   not create canonical article records.
5. **Quality inversion:** a published article is not v0. Briefs and notes may
   seed the work, but the VText agent must develop them into publication-
   quality articles before the news surface treats them as articles.

Changed route:

- First prove broad multilingual source ingestion with real provider/source
  breadth, provider health, and per-source counts.
- Then prove processors can consume source flow and ask researchers/VText
  agents for the right work.
- Then prove VText agents create real article VTexts with source and related
  VText transclusions.
- Then prove reconcilers operate over existing articles, sources, notes, and
  user-published VTexts where authorized.
- Keep the UI minimal and readable: newspaper columns, source chronology,
  quiet VText affordances, no cards/panels/local theme selector, no special
  edit subsystem.

## Priority 1: Source Breadth

Before more article/UI work, Global Wire must stop looking like a small demo
feed. The next implementation phase should research and add many more sources
in many languages.

Targets:

- maintain the GDELT/global-event path;
- expand RSS/Atom from 10 feeds to a serious registry across regions,
  languages, domains, official sources, local outlets, and specialist sources;
- expand Telegram public-preview/channel ingestion from 3 feeds to many
  policy-compliant feeds where allowed;
- add source health and per-source item counts to proof, so source breadth is
  measured by active sources, successful fetches, language/region coverage,
  and item counts, not a vague "many";
- keep hundreds of SourceItems per 15 minutes as the low serious floor, with a
  path toward faster/live ingestion where providers allow.

External research is required for the expanded registry, but this mission file
does not perform that research. It defines it as the first execution axis.

## Hard Invariants

- Every article is a normal editable VText.
- VText is natively editable; do not add a Global Wire "my edits" subsystem.
- User edits, forks, revisions, and publications are user-owned VText flows and
  never mutate platform articles directly.
- A platform correction/update is a normal new VText version through the
  ordinary review/version path.
- Sources are embedded/transcluded through the existing source system.
- Related VTexts are transcluded where editorially useful, not merely listed.
- `Style.vtext` is a citeable editorial source selected intelligently by VText
  agents; do not run all styles over all stories by default.
- Processors and reconcilers are shared-harness agent profiles, not forked
  agent loops.
- Existing researcher agents and existing VText agents are reused.
- News remains non-oracle: uncertainty, contradictions, source standing,
  corrections, and questions stay inspectable.
- Futuristic Noir, Carbon Fiber Kintsugi, and London Salmon must all work via
  the desktop theme system.
- Product-path staging proof is required before claiming behavior.

## Agent Roles

### Processors

Processors receive source flow and maintain live understanding. They may cover
a topic, region, language/source class, developing event family, or
load-balanced firehose slice. They preserve source handles and full-content
paths. They compact for continuity, not provenance replacement.

Processors may request researchers, VText agents, more source search/fetch, or
reconciler attention. They do not own canonical articles.

### Researchers

Researchers answer bounded evidence questions and return source-backed packets.
They do not own article voice or platform article mutation.

### VText Agents

VText agents own articles. They create and revise normal VTexts from briefs,
research packets, source transclusion handles, related VText handles, matched
`Style.vtext` sources, and publication context.

Articles must be real articles: developed prose, source integration,
transcluded evidence, version-local provenance, and style appropriate to the
story.

### Reconcilers

Reconcilers work over the corpus: platform articles, existing published VTexts,
authorized user-owned/published VTexts, source state, processor notes,
research packets, transclusions, questions, contradictions, corrections, and
change history.

They produce notes, questions, research requests, article-update requests, and
new-article prompts. They do not mutate articles directly.

## News App Surface

Global Wire should be a calm collection view:

- newspaper columns, not cards or dashboards;
- source chronology column;
- no nested scrolling panels;
- no border-line grid;
- compact VText affordance on every article;
- no repeated visible "Open in VText" label;
- no app-local theme selector;
- no special contribution/edit panel;
- mobile remains inside Choir desktop/web shell;
- VText mobile menu banner must not overlap buttons or labels.

## Delivery Evidence

Required staging proof:

- source registry count by provider/type/language/region;
- latest ingestion cycle item count, active source count, failed source count,
  and per-source counts;
- evidence that GDELT, RSS/Atom, and Telegram-class sources are actually
  running;
- processors and reconcilers as shared-harness profiles;
- researcher requests/results from processors, reconcilers, or VText agents;
- VText-owned article creation/revision with real article content, not stubs;
- source and related-VText transclusions inside article VTexts;
- intelligent `Style.vtext` selection/use;
- user edits through normal VText flows;
- clean newspaper UI across desktop/mobile and all three themes;
- commit SHA, CI/deploy status, staging health/build identity, and
  product-path browser/API acceptance proof.

## Forbidden Shortcuts

- Do not rename "14 sources" into a success state.
- Do not count hundreds of GDELT items as source diversity by itself.
- Do not create article stubs and call them articles.
- Do not make a finished article the initial VText version unless it is truly
  a complete article.
- Do not build a Global Wire edit subsystem.
- Do not list sources/related VTexts where transclusion is the correct object.
- Do not patch around VText ownership with a separate article store.
- Do not polish UI while source breadth remains toy-sized unless fixing a
  blocking readability defect.

## Run Checkpoint & Resumption State

status: checkpoint_incomplete

last checkpoint: 2026-06-07 architecture correction after user review rejected
the "SourceMaxx" naming/object, article stubs, special edit framing, related
VText/source lists, and mobile VText banner overlap.

current artifact state: staging has a source service that pulls all three
source classes, but only from a small registry: 1 GDELT source, 10 RSS/official
feeds, and 3 Telegram public-preview feeds. Latest observed cycle returned 503
items from 14 fetches. The clean newspaper UI exists, but the broader system is
not yet delivered.

what shipped: prior commits shipped a clean Global Wire preview surface,
source service cycles, processor/reconciler handoff scaffolding, and platform
runtime persistence. Those are useful substrate, not final delivery.

what was proven: deployed source service is running and pulling hundreds of
items per cycle across GDELT/RSS/Telegram classes; source breadth is still
insufficient.

unproven or partial claims: broad multilingual source coverage, real article
VTexts with source/related-VText transclusions, VText-owned article lifecycle
through prompt-bar-normalized workflow, reconciler corpus behavior, and mobile
VText banner correctness.

belief-state changes: source volume exists but source diversity is the first
critical gap. The active architecture should optimize for broad source
registry expansion and VText-native publication, not a named SourceMaxx layer.

remaining error field: add many more multilingual sources; prove per-source
health/counts; normalize article creation through existing VText agents; make
articles real; use source/VText transclusion; fix the mobile VText menu banner.

highest-impact remaining uncertainty: which source providers/feeds can be
added fastest while staying policy-compliant, multilingual, reliable, and
useful for processors.

next executable probe: research and propose a large multilingual source
registry expansion, then implement the first high-confidence batch with
provider health/per-source count proof.

suggested resume goal string: use the Goal String section above.

evidence artifact refs: staging Source Service latest cycle observed
2026-06-07: 503 items, 14 fetches, 11 processor requests, 1 reconciler request;
deployed registry currently has 1 GDELT, 10 RSS/official, and 3 Telegram
sources.

rollback refs: docs-only architecture correction; behavior rollback remains
the prior deployed code state until new implementation commits ship.
