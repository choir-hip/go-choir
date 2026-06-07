# Mission: Global Wire Living VText Newsroom

**Status:** active MissionGradient draft after visual/product correction.  
**Spec:** `docs/choir-global-wire-living-vtext-newsroom-spec-2026-06-07.md`  
**Supersedes:** the SourceMaxx-named mission and the first broad-source draft.  
**Created:** 2026-06-07

## Goal String

```text
/goal Run docs/mission-global-wire-living-vtext-newsroom-v1.md. Ship Global Wire as a living VText newsroom: broad multilingual sources, processors/reconcilers, researcher reuse, VText-owned real articles with source/related-VText transclusion, living updates, clean newspaper UI, staging proof.
```

## Mission Identity

Global Wire is a living VText newsroom.

The mission is not to fix the current surface incrementally. The current
surface exposed a deeper failure: article stubs, visible metadata, source
lists, related VText lists, a "My Edit" section, and poor responsive
typography show that the architecture is not yet normalized around VText as
the article owner.

The mission succeeds only when broad source flow produces real article VTexts
through the existing VText agent workflow, with native source and related-VText
transclusions, and the Global Wire app acts as a clean collection surface.

## Real Object

```text
broad multilingual source ingestion
-> durable SourceItems with source embedding/transclusion handles
-> processors with live context over source flow
-> researchers for evidence gaps
-> VText agents that own article creation/revision
-> reconcilers watching ongoing stories and corpus tensions
-> article VTexts with real prose, source transclusions, related-VText
   transclusions, citations, and version-local provenance
-> rebuildable VText/source indexes for discovery/performance
-> Global Wire newspaper collection surface
-> normal VText app for full article reading/editing/source traversal
```

## Cognitive Transform Set

Current obstacle: the implementation has treated article output as a generated
view rather than as a living VText document owned by a VText agent.

Selected transforms:

1. **Article ownership transform:** ask "which VText agent owns this article
   and what caused this version?" instead of "what story row generated this
   text?"
2. **Transclusion transform:** sources and related VTexts are not lists; they
   are graph objects that should enter the article through native VText/source
   transclusion.
3. **Living story transform:** ongoing stories have versions. New source flow
   should create update requests to VText agents, not detached stubs.
4. **Publication-quality transform:** an outline, source manifest, metadata
   dump, or claim list is not progress unless it is clearly an internal brief.
   The article must read as an article.
5. **Source-breadth transform:** hundreds of items from a tiny registry is not
   enough. Source diversity by language, region, source type, beat, outlet
   class, and long-tail social perspective is the first realism axis.
6. **Collection-surface transform:** Global Wire is the newspaper surface. The
   VText app is the article reader/editor/source traversal surface.

Changed route:

- Start with source breadth and source proof, not UI polish.
- Normalize article lifecycle through VText agents before trying to index or
  render more article rows.
- Require native source and related-VText transclusion in article VTexts.
- Treat reconcilers as ongoing story monitors that message/request VText agent
  updates.
- Fix typography and mobile banner defects as product-quality gates, but do
  not mistake surface cleanup for the architecture being delivered.

## Priority 1: Broad Multilingual Sources

The deployed source service currently proves the substrate, not the target:
one GDELT source, ten RSS/official feeds, and three Telegram public-preview
feeds is not enough.

The first implementation phase must research and expand sources in many
languages.

Acceptance direction:

- maintain GDELT/global-event ingestion;
- add many RSS/Atom feeds across languages, regions, beats, and outlet/source
  classes;
- add many Telegram/public-channel sources where policy-compliant, with an
  explicit bias toward long-tail local, regional, conflict, community,
  technology, finance, and social-sentiment channels;
- include official, local, regional, specialist, financial/economic,
  conflict/crisis, science/health, climate, culture, technology, industry,
  hacker/community, and long-tail social sources;
- add many Telegram/public-preview channels for local perspective, social
  sentiment, weak signals, rumor surfaces, and sources ignored by established
  outlets; articles must corroborate these rather than treating them as
  standalone authority;
- expose source registry counts by type, language, region, beat, and outlet
  class;
- expose latest-cycle active source count, failed source count, per-source item
  counts, latency, freshness, and errors;
- keep hundreds of SourceItems per 15 minutes as a low floor, not the finish
  line.
- do not hardcode source trust tiers or static source standing in the registry;
  track record should be learned over time from outcomes, corroboration,
  corrections, freshness, error history, and researcher/model judgment.

## Priority 2: VText-Owned Article Lifecycle

The current generated VText output is not acceptable:

- `v0` reads as the finished article even though it is only a projection/stub;
- metadata appears in the document body;
- source manifest is plain text instead of transcluded sources;
- related VTexts are listed instead of transcluded;
- "My Edit" appears as a section even though VText is natively editable.

Correct lifecycle:

```text
processor/reconciler/researcher/user intent
-> VText agent receives brief + source handles + related VText handles + style
-> VText agent creates/revises normal article VText
-> article version contains prose + transclusions + citations + provenance
-> Global Wire indexes/displays article excerpt
-> full reading/editing/source traversal happens in VText
```

Do not create a separate Global Wire article store that owns the canonical
article.

## Priority 3: Living Updates And Reconcilers

Ongoing stories get updated as the world changes.

Processors notice developments from source flow. Reconcilers compare new
source state against existing article VTexts and related VTexts. When an
article needs an update, correction, qualification, or follow-up, the
reconciler sends a request to the owning VText agent.

The update produces a normal new VText version. Corrections are good; they are
evidence of living versioned publication.

## Priority 4: UI And Typography

Global Wire UI must stop looking like a dashboard or typography stress test.

Required product fixes:

- remove the old source-volume product label;
- use serif article headlines;
- avoid huge sans headline blocks;
- make normal desktop widths readable, not only wide screens;
- keep mobile inside the Choir desktop/web shell but make it responsive;
- no cards, no border-line grid, no nested panel scrolling;
- source chronology should be quiet and useful, not a heavy dashboard;
- every article has a compact VText affordance;
- no repeated "Open in VText" labels;
- VText mobile banner/menu must not overlap buttons or labels;
- VText article rendering must hide metadata sludge and render source/related
  VText transclusions natively.

## Hard Invariants

- Every article is a normal VText.
- VText agents own article creation and revision.
- User edits are normal user-owned VText revisions/forks/publications.
- No Global Wire "My Edit" section or edit subsystem.
- Platform articles change only through normal VText version/update paths.
- Sources must use native source embedding/transclusion.
- Related VTexts must be transcluded where editorially useful.
- `Style.vtext` is a citeable editorial source selected intelligently.
- News remains non-oracle and provenance-rich.
- Processors, reconcilers, researchers, and VText agents use the shared agent
  harness with role profiles/tool policies.
- The Global Wire app is a collection surface, not the article editor.
- Product-path staging proof is required before claiming behavior.

## Delivery Evidence

Required proof:

- source registry expanded substantially beyond 14 configured sources;
- source registry summarized by type, language, region, beat, and outlet class;
- latest source cycle proves active source count, per-source item counts,
  failures, latency, freshness, and item volume;
- GDELT/global event, RSS/Atom, Telegram/public-channel, official, and
  specialist source classes are actually running;
- processors receive source batches and preserve source handles;
- researchers are requested and return source-backed packets;
- VText agents create/revise article VTexts as owners, not helper tools;
- article VTexts contain real prose, source transclusions, related VText
  transclusions, and citations;
- reconcilers detect update/correction/follow-up needs and message/request
  VText agent revisions;
- user edits/forks happen through normal VText flows;
- Global Wire UI and VText article view pass visual/product checks on normal
  desktop, wide desktop, and mobile-in-desktop-shell sizes across Futuristic
  Noir, Carbon Fiber Kintsugi, and London Salmon;
- staging health/build identity, CI/deploy status, and product-path
  browser/API acceptance proof are recorded.

## Forbidden Shortcuts

- Do not use the old source-volume label as a product name.
- Do not treat 14 sources as adequate.
- Do not count one high-volume source as source diversity.
- Do not encode permanent trust tiers or source standing in config.
- Do not let long-tail social feeds become article authority without
  corroboration, research, or explicit uncertainty.
- Do not call outlines, manifests, or claim lists articles.
- Do not display metadata in article prose.
- Do not list sources/related VTexts where transclusion is required.
- Do not build a Global Wire edit subsystem.
- Do not use VText as a text-generation subroutine while Global Wire owns the
  article.
- Do not claim UI proof while normal desktop widths or mobile menus are broken.

## Run Checkpoint & Resumption State

status: checkpoint_incomplete

last checkpoint: 2026-06-07 user visual/product review confirmed the current
Global Wire/VText output is wrong-object work: poor desktop typography,
normal-width layout failure, mobile issues, article stubs, visible metadata,
source lists, related VText lists, a "My Edit" section, and no native source
transclusion. 2026-06-07 source architecture correction removed static source
trust tiers/standing from the registry design and expanded the local registry
candidate set toward broad RSS/Atom plus long-tail Telegram/social evidence.

current artifact state: staging source service runs and pulls hundreds of
items per cycle from a small registry. Local source registry work now expands
configuration from 14 to 170 sources: 1 GDELT, 110 RSS/Atom, and 59 Telegram
public-preview sources across 15 language tags, with tech, science, industry,
finance, regional, conflict, and long-tail social/sentiment sources. Global
Wire has a clean-ish collection surface but still exposes old naming and weak
typography; opened article VTexts are projection/stub documents rather than
real living articles.

what shipped: prior work shipped source service substrate, processor/reconciler
handoff scaffolding, some VText agent usage, and a cleaner newspaper preview.
Those are substrate only.

what was proven: source service can ingest GDELT/RSS/Telegram-class items;
staging can deploy and show Global Wire; the screenshots prove the current
article/VText object is not acceptable. Local validation checked working
RSS/Atom and Telegram public-preview URLs before adding them; `nix develop -c
go test ./internal/sources ./internal/cycle` passed after removing static
source tiers/standing from config.

unproven or partial claims: broad multilingual source coverage, VText agent as
article owner, publication-quality articles, native source transclusion,
related VText transclusion, living updates, reconciler-driven revisions, and
responsive/typographic quality.

belief-state changes: source breadth and VText ownership are the first
architectural blockers. UI cleanup alone cannot solve the wrong object.

remaining error field: research/add many multilingual sources; normalize
article lifecycle through VText agents; replace source/related lists with
transclusions; remove metadata/edit sludge from article documents; fix
typography and mobile banner overlap.

highest-impact remaining uncertainty: whether the expanded registry runs
cleanly on staging over full source-service cycles, what per-source failure
distribution appears, and how to add learned source track-record state without
turning it into static editorial authority.

next executable probe: deploy the expanded source registry, observe staging
source-service cycles, add source proof surfaces for registry/cycle counts and
learned track-record metrics, then return to VText-owned article generation.

suggested resume goal string: use the Goal String section above.

evidence artifact refs: user screenshots from 2026-06-07 at 14:36-14:39 show
the UI/article/VText failures; staging source service latest observed cycle
had hundreds of items but only 14 configured sources.

rollback refs: docs draft only unless later committed/pushed. Behavior
rollback remains prior deployed code until implementation commits ship.
