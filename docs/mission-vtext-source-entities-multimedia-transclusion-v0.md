# Mission Campaign: VText Source Entities, Multimedia Review, And Transclusion v0

**Status:** draft
**Date:** 2026-05-31
**Method:** MissionGradient with Cognitive Transform Portfolio
**Related docs:**
[mission-youtube-review-studio-v0.md](mission-youtube-review-studio-v0.md),
[podcast-radio-brief-proof-2026-05-13.md](podcast-radio-brief-proof-2026-05-13.md),
[publication-reader-retrieval-pretext-research-2026-05-16.md](publication-reader-retrieval-pretext-research-2026-05-16.md),
[platform-dolt-publication-retrieval-citation-research-2026-05-16.md](platform-dolt-publication-retrieval-citation-research-2026-05-16.md),
[mission-standalone-sourcecycled-data-platform-v0.md](mission-standalone-sourcecycled-data-platform-v0.md)

## One-Line Goal String

```text
/goal Run docs/mission-vtext-source-entities-multimedia-transclusion-v0.md as a Codex-operated MissionGradient campaign. Build Choir's VText-native Source Entity substrate so VText can embed, cite, expand, and transclude multimedia, web content, podcast/YouTube transcripts, and other VTexts inline without flattening sources into prose. Preserve VText as the canonical artifact-level surface, ContentItem as the owner-scoped source artifact substrate, platform publication records as the public immutable citation/transclusion ledger, and researcher coagents as source-representation producers rather than canonical writers. Start with the smallest deployed product path that lets a user create or revise a VText with inline expandable source entities over a real YouTube video and transcript when available, then deform the same object toward podcast transcript review, web source packets, VText-to-VText transclusion, publication proposals, and later sourcecycled manifest import. Do not build a parallel transcript app, source browser, static article renderer, markdown-only citation syntax, or sourcecycled integration shortcut that bypasses VText/source provenance. Do not claim success without deployed VText rendering, durable revision metadata, source artifact resolution, and user-visible expandable citations on staging.
```

## Executive Review

Choir already has pieces of the target, but not the product object.

The current system can persist VText revisions with `citations_json` and
`metadata_json`. It can store owner-scoped `ContentItem`s. It can import
YouTube URLs into video content items, attempt transcript acquisition, and store
transcript items as private untrusted source artifacts when available. VText
revise can detect YouTube and image URLs and attach `media_source_refs` to a
run/revision path. The VText frontend can render those refs as source cards
with a YouTube iframe or image preview.

Podcast is further along as an app surface. The podcast app has durable feed
library, search/import/subscription paths, playback controls, and a proven path
from podcast feed to a VText radio brief. But podcast is not yet a transcript
review system. It lacks durable episode transcripts, timestamped source spans,
clip citations, and VText-native review composition over transcript evidence.

Publication and public transclusion are started at the platform edge. Platform
records can represent citation edges and proposal transclusions. The design
work correctly says Pretext is a rendering/layout primitive, not the semantic
trust model. But private VText editing does not yet have a first-class inline
source entity model. Today the product has scattered source concepts:
`citations_json`, `metadata.transclusions`, `media_source_refs`, `ContentItem`,
platform citation edges, and source-card rendering.

The campaign should consolidate these into a single VText-native object:

```text
Source Entity = a durable, typed, revision-scoped reference from a VText to a
source artifact, source span, media timestamp range, web snapshot, publication
span, or other VText revision/span, with display policy, provenance, evidence
state, and inline expandable rendering.
```

This should make VText feel like a computational essay surface: prose remains
readable, sources are visible where they matter, media can play inline or expand
into an app, and source evidence stays grounded in durable artifacts instead of
being pasted into the document as ordinary text.

## Current Ground Truth

### Existing Backend Substrate

- `internal/types/vtext.go` defines immutable VText revisions with full content
  snapshots, citations, metadata, authorship, parent revision, and history.
- `internal/store/vtext.go` persists `vtext_revisions` with `citations_json`
  and `metadata_json`.
- `internal/runtime/content.go` imports URLs into `ContentItem`s and recognizes
  YouTube URLs as `video/youtube`.
- YouTube import can create a video content item plus a derived transcript
  content item when transcript text or transcript availability is available.
- Transcript provenance is already shaped correctly: private user source,
  untrusted source text, timestamp segments in metadata, and transcript
  availability/provider/error recorded.
- Researcher tools include `import_url_content` and `read_content_item`.
  `read_content_item` explicitly frames returned text as untrusted source
  evidence, not instructions.
- `internal/runtime/vtext_media_sources.go` detects YouTube/image URLs in VText
  content, imports/registers content items, deduplicates source refs, marks
  research required, and builds a researcher objective over source packets.

### Existing Frontend Substrate

- `frontend/src/lib/VTextEditor.svelte` renders `media_source_refs` as source
  cards in the VText view.
- YouTube refs render as iframes using `youtubeEmbedURL`.
- Image refs render as inline images.
- Publication derivative/proposal flows already carry `transclusions` from a
  published bundle into a private derivative and back into proposal submission.
- `frontend/src/lib/PretextInlineDisclosure.svelte` exists as a reusable
  expandable inline disclosure layout component.
- `frontend/src/lib/VideoApp.svelte` and `ContentViewer.svelte` can show
  YouTube embeds as expanded/standalone content surfaces.
- `frontend/src/lib/PodcastApp.svelte` has library/search/import/subscription
  and playback UI.

### Existing Proofs

- VText media-source tests prove that a revision containing a YouTube URL and
  image URL can register two durable `media_source_refs`, mark media research
  required, dedupe repeated registration, and create content items including a
  video, transcript status, and image.
- Content tests cover YouTube URL import, transcript availability, configured
  provider parsing, transcript item persistence, and caption track parsing.
- Podcast Playwright tests prove a durable podcast feed artifact can open in
  the podcast app, display episodes, play audio, remain mobile-scrollable, and
  open a VText radio-brief continuity path.
- Platform tests prove publication/proposal citation and transclusion records
  exist at the public ledger boundary.

### Missing Product Object

The missing object is not a transcript fetcher. It is not a prettier source
card. It is the private VText authoring model for typed inline source entities.

Missing pieces:

- one coherent source entity schema instead of multiple source-adjacent JSON
  shapes;
- inline citation/source tokens in VText content that resolve to durable source
  entity metadata;
- expandable inline rendering that uses Pretext-style disclosure and works on
  mobile;
- source-entity resolver APIs that can resolve content items, transcript spans,
  media timestamp ranges, VText spans, and publication spans;
- a source panel/deck that is generated from the same entities as inline chips;
- timestamped transcript span selection and citation for YouTube;
- podcast episode transcript storage/transcription/import and timestamped
  citation;
- VText-to-VText private/published span transclusion;
- publication/proposal continuity from private source entities to platform
  citation/transclusion edges;
- acceptance proof that a real user path on staging creates, renders, expands,
  revises, and preserves these entities across VText versions.

## Cognitive Transform Notes

### 1. Audience-Level Translation

**Audience:** user/author.
**Core idea:** VText should let you write while keeping sources alive inside
the writing.
**Words to avoid in product UI:** schema, provenance graph, metadata,
selector, transclusion protocol.
**Usable explanation:** Paste a video, podcast, article, image, or another
VText into your document. Choir turns it into a source you can cite inline.
Tap the citation to expand the exact clip, transcript passage, page excerpt, or
source VText span without losing your place.
**Action change:** Build the first slice as an authoring/review workflow, not
as a backend citation database milestone.

### 2. Depth Extraction / Esoteric Upgrade

**Banal version:** add citations and media embeds to VText.
**Deep version:** preserve source identity through model use, revision history,
inline reading, expansion, publication, proposal, and future retrieval.
**Load-bearing variable:** source identity survival across transformations.
**Common failure mode:** pasting transcript excerpts into Markdown and calling
that citation.
**Action change:** Treat transcript/media/web/VText references as typed source
entities backed by durable artifacts, not as formatted text.

### 3. Via Negativa

Delete routes that make the source graph impossible to trust:

- no parallel transcript app that owns source understanding outside VText;
- no source-card-only rendering that cannot be cited inline;
- no full transcript pasted into the review body as ordinary prose;
- no sourcecycled live integration before the VText source entity importer
  exists;
- no static publication reader that bypasses VText;
- no fake citation syntax that does not resolve to owner-scoped artifacts or
  immutable public refs;
- no agent-authored factual claims that are not grounded in source entities or
  researcher evidence.

**Action change:** First implementation should consolidate and remove bypass
surfaces, not add another source-specific feature path.

### 4. Artifact Substrate Transform

The real artifact is not "YouTube review" or "podcast transcripts." Those are
pressure cases. The durable substrate is:

```text
VText revision
  -> source_entities metadata
  -> inline source refs in content/render model
  -> ContentItem or VText/publication target
  -> resolver/evidence state
  -> expandable rendering
  -> publication/proposal projection
```

**Action change:** Campaign v0 should make one source entity path complete
before broadening source types.

### 5. Homotopy, Not Ladder

The low-resolution version must be the same object as the final version.

Acceptable simplification:

```text
one YouTube source entity with transcript unavailable or available
  -> same schema and resolver that later supports podcast/web/VText/sourcecycled
```

Unacceptable ladder:

```text
hardcoded YouTube embed markdown now
  -> rewrite into source entities later
```

**Action change:** Even the first YouTube slice must use the general source
entity schema and resolver.

### 6. OODA / Inner-Loop Transform

The authoring loop should reduce time from source discovery to grounded
revision:

```text
paste source
-> source entity appears
-> local files, owned content, public Choir/Dolt records, and web results are
   searched for related source context
-> VText/researcher process evidence
-> author sees inline support/uncertainty
-> next revision preserves source identity
```

**Action change:** Acceptance should measure the whole loop, not only import or
render.

## Real Artifact

The artifact is a deployed VText source-entity authoring path:

```text
User opens or creates a VText
  -> user pastes a YouTube URL / source URL / VText ref
  -> VText revise registers typed SourceEntity objects
  -> ContentItem/source artifacts are imported or resolved
  -> transcript/source representation is fetched or marked unavailable/pending
  -> researcher can inspect source artifacts and submit compact source updates
  -> VText writes the next version with inline source entity refs
  -> VText renders compact inline citations/entities
  -> user expands an entity in place or opens it in its owning VText/media/app
     window to see media/excerpt/provenance
  -> source deck/panel shows the same entities
  -> revision history preserves entities
  -> publication/proposal path projects citations/transclusions to platform
     records without losing source identity
```

The artifact is not:

- a one-off YouTube embed;
- a podcast-only transcript feature;
- a source browser detached from VText;
- a general OSINT ingestion platform;
- a static public article renderer;
- a Markdown citation convention with no resolver;
- a backend-only schema with no author-visible workflow.

## Product Vocabulary

**Source Entity:** a typed source reference attached to a VText revision or
document. It has identity, target, display policy, provenance, evidence state,
selectors, and optional researcher/source-representation data.

**Inline Source Ref:** a compact token or rendered chip inside VText prose that
points to a source entity.

**Source Target:** the thing being referenced: `ContentItem`, transcript span,
media timestamp range, private VText revision/span, public publication
version/span, local filesystem file, web snapshot/excerpt, global public
Choir/Dolt publication record, or later sourcecycled manifest item.

**Source Discovery Field:** the search space used to find related evidence while
the system is already doing source work. It includes owner-scoped content items,
the user's persistent filesystem, private VTexts, public Choir/Dolt publication
records, and web search/fetch results. Web search should not be treated as a
separate mode that ignores local and public Choir memory.

**Source Representation:** a researcher-authored compact description of a
source packet: summary, notable claims, timestamped excerpts, uncertainty,
follow-up needs. It is evidence for VText synthesis, not the canonical VText
body.

**Expansion:** in-place or adjacent source disclosure that reveals the target:
video embed, audio player, image, transcript span, web excerpt, VText span,
provenance, and evidence status.

**Open In Owning Surface:** a heavier version of expansion where the entity
opens in the app/window that owns the target: VText for VText refs, Video for
videos, Podcast for podcast episodes, Browser/ContentViewer for web pages,
Image/PDF/EPUB/etc. for file and media artifacts. Inline expansion and owning
window opening are complementary, not competing, interaction modes.

**Transclusion:** source material intentionally included from another artifact
into a host VText. A transclusion is stronger than a citation. Citation says a
source supports/relates to a claim; transclusion says source material appears
here by reference.

**Pinned Ref:** a source target bound to an immutable revision/version/hash.

**Live Ref:** a source target bound to a mutable head. Live refs are useful but
should not be the default citation/transclusion target in v0.

## Proposed Source Entity Shape

This is a design target, not a mandatory exact schema:

```json
{
  "entity_id": "src_...",
  "kind": "youtube_video|podcast_episode|web_page|image|audio|video|file|private_vtext_span|published_vtext_span|sourcecycled_item",
  "label": "Interview with ...",
  "target": {
    "target_kind": "content_item|local_file|vtext_revision|publication_version|public_choir_dolt_record|external_url|sourcecycled_manifest_item",
    "content_id": "content-...",
    "file_path": "/mnt/persistent/files/...",
    "doc_id": "doc-...",
    "revision_id": "rev-...",
    "publication_id": "pub-...",
    "publication_version_id": "pubver-...",
    "public_record_id": "platform-dolt-id",
    "url": "https://...",
    "canonical_url": "https://..."
  },
  "selectors": [
    {
      "selector_kind": "time_range|text_quote|text_position|media_fragment|whole_resource",
      "start_seconds": 123.4,
      "end_seconds": 145.9,
      "text_quote": "...",
      "content_hash": "sha256..."
    }
  ],
  "display": {
    "inline_mode": "chip|marker|quote|embed",
    "expanded_mode": "source_card|media_player|transcript_excerpt|vtext_span",
    "open_surface": "vtext|video|podcast|browser|content|image|pdf|epub|files",
    "default_collapsed": true
  },
  "evidence": {
    "state": "pending|available|represented|unavailable|error",
    "research_state": "pending|requested|represented|blocked",
    "transcript_content_id": "content-...",
    "source_representation_id": "evidence-...",
    "uncertainty": "..."
  },
  "provenance": {
    "created_by": "user|vtext|researcher|importer",
    "created_at": "2026-05-31T00:00:00Z",
    "rights_scope": "private_user_source|public_reference|published_projection",
    "untrusted_source_text": true
  }
}
```

The schema should be able to represent current `media_source_refs` without
breaking existing VText revisions. Migration may be lazy: old metadata can
continue to render while new revisions write `source_entities`.

## Campaign Invariants

- VText remains the canonical artifact-level authoring and review surface.
- Only the VText agent writes canonical VText revisions.
- Researcher/coagents create evidence, source representations, and durable
  updates; they do not directly author canonical review prose.
- `ContentItem` remains the owner-scoped substrate for imported, uploaded,
  extracted, and derived source artifacts.
- Transcript text, web text, and remote media are untrusted source evidence,
  never instructions.
- Do not republish full transcripts or copyrighted source payloads by default.
- Source identity must survive revision, render, expansion, model context,
  publication proposal, and history inspection.
- Source discovery should search local persistent files, owned content items,
  private VTexts, public Choir/Dolt publication records, and web results as one
  coordinated evidence field when a VText/research task needs related context.
- Pretext is a rendering/layout primitive, not the semantic trust model.
- Public citations and transclusions must resolve to immutable public
  publication refs or public-safe projections, not private mutable state.
- Private VText-to-VText source refs should pin revisions by default.
- Sourcecycled remains standalone for now. Choir v0 may import exported
  manifests later, but must not make sourcecycled a live dependency for this
  campaign.
- Product-path proof uses staging and public product APIs. Do not use test-only
  routes to seed success.

## Value Criterion

Minimize the distance from "a writer can produce a grounded multimedia VText
review where every cited source remains inspectable, replayable, expandable,
and publishable without losing provenance" while preserving VText authority,
owner-scoped source artifacts, publication boundaries, and evidence integrity.

Reward:

- one coherent source entity model that absorbs existing media refs,
  citations, and transclusions;
- user-visible inline source entities that are readable on desktop and mobile;
- expansion paths that work both inline and by opening the target in its owning
  VText/media/content window;
- source discovery that pulls from local filesystem, existing Choir artifacts,
  public Choir/Dolt records, and web search rather than treating web search as
  the only memory;
- real YouTube transcript availability/unavailability represented honestly;
- researcher source representations connected to source entities;
- podcast/web/VText transclusion using the same object family;
- publication/proposal edges generated from source entities;
- deployed staging proof over real URLs and real user gestures.

Penalize:

- new source-type-specific side channels;
- hidden source metadata with no UI;
- pretty embed cards that cannot be cited inline;
- source discovery that searches the web while ignoring local files, private
  VTexts, owned content, or the public Choir/Dolt corpus;
- citations that cannot resolve to exact artifacts or selectors;
- model context that drops source identity;
- platform publication records disconnected from private VText entities;
- broad sourcecycled integration before the VText substrate is real.

## Quality Gradient

Expected quality: **solid**.

Solid means:

- source entity schema is simple, documented in code or docs, and backwards
  compatible with existing `media_source_refs`;
- rendering is usable on mobile and desktop;
- source entities survive reload and revision history;
- at least one real YouTube path works end to end on staging;
- transcript unavailable/error states render honestly;
- no full transcript is pasted into VText prose by default;
- tests cover schema round-trip, resolver behavior, rendering, revise path, and
  source identity preservation;
- final report includes deployed commit SHA, CI/deploy status, staging
  identity, and acceptance proof.

Excellent means:

- podcast transcript entities use the same path;
- web source snapshots/excerpts use the same path;
- VText-to-VText transclusion uses the same path;
- publication proposal maps private source entities into platform citation and
  transclusion edges;
- source panel/deck and inline chips share one resolver and one source of
  truth.

## Campaign Structure

This is a campaign, not one brittle checklist. Each mission should leave the
artifact closer to the same final object.

### Mission 0: Source Entity Nucleus

**Goal:** introduce the private VText source entity object and render it.

Minimum product path:

```text
create/revise VText with YouTube URL
  -> source_entities metadata written
  -> old media_source_refs still accepted
  -> VText renders inline entity/chip and expandable source card
  -> source deck shows same entity
  -> reload/history preserves entity
```

Implementation contour:

- define source entity JSON shape and decoder/normalizer;
- map current `media_source_refs` to source entities lazily;
- update VText revise metadata generation to write source entities;
- render inline source chips/entities in VText;
- reuse or adapt `PretextInlineDisclosure` for compact expansion;
- ensure contenteditable serialization does not erase source refs;
- add tests around round-trip and rendering.

Acceptance:

- deployed staging: user pastes real YouTube URL into VText, hits Revise, sees
  inline expandable source entity and source deck card after reload.

### Mission 1: YouTube Review Studio

**Goal:** make YouTube review a real authoring workflow over source entities.

Minimum product path:

```text
paste/open YouTube review intent
  -> VText review doc created/opened
  -> video source entity embedded
  -> transcript content item fetched or unavailable state recorded
  -> transcript span/source representation can be cited inline
  -> VText next version preserves playable media and source identity
```

Key requirements:

- no separate video window as default for review intent;
- transcript full text remains source artifact, not document prose;
- timestamp selectors are represented even if UI selection is minimal in v1;
- researcher source representation is attached to the entity;
- acceptance includes transcript-available and transcript-unavailable cases.

### Mission 2: Podcast Transcript Review

**Goal:** extend source entities to podcast episodes and transcript/clip spans.

Minimum product path:

```text
search/import podcast
  -> select episode
  -> transcript artifact exists via import/provider/transcription or honest pending state
  -> create VText review/radio brief with episode source entity
  -> cite timestamped clip/transcript span inline
```

Key requirements:

- keep podcast app as playback/library surface;
- make VText the review/citation surface;
- episode source identity should include feed URL, episode GUID/link, audio URL,
  duration when known, and content hash/provenance where available;
- if transcription provider is unavailable, represent pending/unavailable
  honestly and do not fake transcript proof.

### Mission 3: Web Source Packets

**Goal:** support ordinary web pages as source entities with bounded excerpts
and source snapshots, while searching local and Choir-native context alongside
the external web.

Minimum product path:

```text
paste article/web URL into VText
  -> search related local files, owned content items, private VTexts, and public
     Choir/Dolt records while fetching/searching the web
  -> import/snapshot/extract source packet
  -> VText renders inline source entity
  -> expansion shows title, URL, excerpt, provenance, fetch status
  -> optional owning-surface open launches Browser/ContentViewer for full page
  -> researcher can produce source representation
```

Key requirements:

- source text remains untrusted;
- exact excerpts/selectors should be stored;
- paywall/fetch errors are represented, not papered over;
- local filesystem and public Choir/Dolt matches are represented as source
  candidates with their own provenance, not silently merged into web results;
- browser display remains available for full page, but VText owns citation.

### Mission 4: VText-to-VText Transclusion

**Goal:** make another VText revision/span embeddable inside a host VText.

Minimum product path:

```text
source VText revision/span
  -> host VText source entity
  -> inline expandable transcluded span
  -> optional open action opens the source VText in its own VText window
  -> pinned revision by default
  -> proposal/publication path preserves transclusion edge
```

Key requirements:

- private refs remain owner-scoped;
- public refs resolve through platform publication versions;
- live-head refs, if present, are visibly live and not default;
- local IDs/selectors avoid collisions in composed documents.

### Mission 5: Publication Projection

**Goal:** project private source entities into platform-safe citation and
transclusion records during publication/proposal.

Minimum product path:

```text
private VText with source entities
  -> publish/propose
  -> platform records citation/transclusion edges
  -> public/guest reader can see public-safe source affordances
```

Key requirements:

- private source artifacts are not leaked;
- public citations point to external references or public publication versions;
- public transclusions use projection hashes/snapshot text where needed;
- author proposal flow preserves source intent.

### Mission 6: Sourcecycled Manifest Import

**Goal:** consume standalone sourcecycled exports as source entity packets.

Minimum product path:

```text
sourcecycled issue/item manifest
  -> Choir import creates source entities/content refs
  -> VText review cites sourcecycled items inline
  -> sourcecycled remains standalone and replaceable
```

Key requirements:

- no live dependency on sourcecycled service for v0;
- import exact manifest identity/hash;
- preserve item provenance and source policy;
- sourcecycled output becomes evidence/source packet, not canonical VText prose.

### Cross-Campaign Mission: Unified Source Search

**Goal:** whenever VText/researcher needs source context, search the local
machine, Choir-owned artifacts, public Choir/Dolt records, and the web as a
coordinated source discovery field.

Minimum product path:

```text
VText/research request needs sources
  -> query owner ContentItems and private VTexts
  -> search persistent local filesystem where authorized
  -> query global public Choir/Dolt publication/citation records
  -> run web search/fetch where needed
  -> return typed source candidates with provenance and confidence
  -> VText turns selected/accepted candidates into SourceEntities
```

Key requirements:

- local search is user-computer scoped and respects private ownership;
- public Choir/Dolt search uses immutable publication/version/citation records;
- web search is one evidence source, not the default replacement for memory;
- each candidate records where it came from and whether it can be cited,
  transcluded, or only used as private background;
- this should become the default source discovery behavior for source-grounded
  VText and researcher work.

## Homotopy Parameters

Increase realism continuously along these axes:

- source types: `YouTube -> image -> web -> podcast episode -> VText span ->
  sourcecycled item`;
- selectors: `whole resource -> timestamp range -> transcript text quote ->
  text position/hash -> composed transclusion`;
- rendering: `source deck card -> inline chip -> expandable inline disclosure ->
  open owning app/window -> synchronized media/transcript view`;
- discovery: `current revision refs -> owned content search -> local filesystem
  search -> public Choir/Dolt search -> web search/fetch -> combined ranked
  source candidates`;
- evidence: `pending/unavailable -> imported content item -> transcript item ->
  researcher representation -> cited excerpt`;
- publication: `private revision metadata -> publication proposal payload ->
  platform citation edge -> public-safe reader rendering`;
- verification: `unit tests -> Svelte tests -> Playwright local -> staging
  desktop/mobile product proof`.

A lower-resolution proof is valid only if it uses the same source entity family
and can deform into the higher-resolution path without a rewrite.

## Dense Feedback And Verification

Required feedback loops:

- unit tests for source entity decode/normalize/migration from
  `media_source_refs`;
- runtime tests for VText revise writing source entities and preserving
  transcript availability;
- content tests for YouTube transcript available/unavailable/error;
- frontend tests for inline rendering, expansion, reload, and mobile layout;
- frontend/product tests for "open in owning surface" from source entities:
  VText refs open VText, videos open Video, podcast episodes open Podcast,
  web refs open Browser/ContentViewer, and files/media open their owner apps;
- Playwright product proof for VText source entity creation/revision;
- staging proof after deploy for at least one real public YouTube URL;
- evidence record naming transcript provider status and any external blocker.

For podcast/web/VText missions, add:

- podcast transcript/episode identity tests;
- web import snapshot/excerpt tests;
- private VText span resolver tests;
- source discovery tests proving local files, owned content, public Choir/Dolt
  records, and web results remain typed and provenance-separated;
- platform publication/proposal projection tests.

## Forbidden Shortcuts

- Do not use frontend-only local state for source entities.
- Do not treat full transcript paste as source integration.
- Do not create a separate transcript/review app as the canonical writing
  surface.
- Do not hide source identity in prompts without rendering it in VText.
- Do not create citation markers that have no resolver.
- Do not make inline expansion the only interaction if the source has a natural
  owning app/window.
- Do not make owning-window opening the only interaction if a compact inline
  source disclosure would preserve reading flow.
- Do not perform web search as a substitute for searching local filesystem,
  owned content, private VTexts, and public Choir/Dolt records.
- Do not claim transcript support from fixtures only.
- Do not bypass `ContentItem` for owner-scoped sources.
- Do not bypass platform publication records for public citation/transclusion.
- Do not integrate sourcecycled before a manifest import seam exists.
- Do not let researcher write canonical VText prose directly.

## Belief State

Current belief:

- VText revision persistence, metadata, and citations are stable enough to host
  source entities.
- Content import has enough YouTube transcript machinery for a real first slice,
  but external transcript availability/provider behavior must be proven on
  staging.
- Podcast is app-real but transcript-review-incomplete.
- Publication transclusion/citation records exist but are not yet driven by
  private inline source entities.
- Pretext can support inline disclosure, but it should not be made the semantic
  model.

Main uncertainties:

- how contenteditable VText serialization should represent inline source refs
  without making editing brittle;
- whether the first inline source ref should be explicit textual syntax,
  invisible metadata anchored to content ranges, or a hybrid;
- how much transcript span selection can be built in v0 without derailing the
  source entity substrate;
- which podcast transcript provider/path is acceptable for staging;
- how to project private source entities into public-safe records without
  leaking private artifacts.

Highest-impact uncertainty:

Whether VText can round-trip inline source refs through edit, render, revise,
reload, and history without losing source identity or making the editor brittle.

Next observation that reduces uncertainty:

Build a source entity nucleus for one YouTube URL and prove the entity survives
VText edit/revision/reload/history on staging.

## Receding-Horizon Control

Operate in small control intervals:

1. Define the source entity shape and migration compatibility.
2. Prove backend round-trip for one YouTube source.
3. Prove frontend render/expand/reload for one source entity.
4. Add transcript availability and researcher representation.
5. Add inline citation placement/range anchoring.
6. Only then broaden to podcast/web/VText transclusion.

At each interval, update belief state:

- what source identity survived;
- where it was lost or flattened;
- which resolver/display path owns it;
- which test or staging proof supports the claim.

## Rollback Policy

Source entity rollout should be backwards compatible:

- old revisions with `media_source_refs` still render;
- new revisions may write both `source_entities` and compatibility
  `media_source_refs` during transition;
- no migration should rewrite historical VText content in place;
- if rendering fails, VText should still show readable prose and a recoverable
  source deck;
- publication projection should ignore unknown source entity kinds rather than
  leaking private data.

Platform behavior changes follow the normal landing loop:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

Docs-only campaign updates do not require platform deploy.

## Learning Side-Channel

Each mission should update this document or a linked evidence artifact with:

- source entity schema changes;
- new source kinds supported;
- staging proof commands/URLs/screenshots;
- transcript provider behavior;
- unresolved selector/rendering limitations;
- publication projection caveats;
- next realism axis.

Do not bury these learnings only in Trace or chat logs.

## Stopping Conditions

### Mission 0 Complete

Mission 0 is complete when staging proves:

- a user can create/revise a VText with a real YouTube URL;
- a durable source entity is attached to the revision;
- the entity resolves to owner-scoped content artifacts;
- VText renders a compact inline source affordance and expandable source card;
- reload/history preserves the entity;
- transcript availability/unavailability is visible and honest;
- old `media_source_refs` revisions still render.

### Campaign Checkpoint

The campaign can stop at a checkpoint when:

- one source type works end to end through the general source entity path;
- the next source type is clearly parameterized by the same schema/resolver;
- evidence and residual risks are recorded.

### Campaign Complete

The v0 campaign is complete when:

- YouTube, podcast episode, web page, and VText span all use the same source
  entity family;
- inline expansion works on mobile and desktop;
- publication/proposal projection preserves citation/transclusion intent;
- sourcecycled manifest import is designed or implemented as a clean standalone
  import seam;
- deployed staging proof covers the main authoring/review path.

## Suggested First Resume Goal

```text
/goal Run Mission 0 from docs/mission-vtext-source-entities-multimedia-transclusion-v0.md. Implement the Source Entity nucleus for VText using the existing YouTube media-source path as the first pressure case. Preserve backwards compatibility with media_source_refs, write durable source_entities metadata on VText revise, render compact inline expandable source entities in VText on desktop/mobile, resolve entities to owner-scoped ContentItems, and prove on staging that a real YouTube URL survives revise, reload, history, and expansion without flattening transcript/source identity into prose.
```

## Run Checkpoint & Problem Record - 2026-05-31

**status:** checkpoint_incomplete

**last checkpoint:** campaign authorized for implementation as a
Codex-operated MissionGradient run.

**current artifact state:** VText can persist revision metadata and citations,
`ContentItem` can represent YouTube/video/transcript/image artifacts, VText
revise can register legacy `media_source_refs`, and the VText frontend can
render source cards from those refs. This is a useful predecessor, but it is
not yet the Source Entity substrate.

**problem being fixed before code changes:** source identity is split across
legacy `media_source_refs`, content items, revision metadata, prompt text, and
frontend source-card rendering. There is no first-class `source_entities`
object that can serve as the common private VText authoring substrate for
inline citations, expandable source disclosure, owning-window opens,
researcher source representations, future podcast/web/VText transclusion, and
publication projection.

**evidence:**

- `internal/runtime/vtext_media_sources.go` defines `vtextMediaSourceRef` and
  registers YouTube/image URLs as media refs.
- `internal/runtime/vtext.go` carries `media_source_refs` into VText appagent
  run metadata and prompt context.
- `frontend/src/lib/VTextEditor.svelte` renders `media_source_refs` as a
  source deck, but does not render a general source entity model or inline
  source affordance.
- VText revision storage supports `metadata_json` and `citations_json`, so the
  missing piece is product modeling and round-trip behavior, not a new
  persistence substrate.

**remaining error field:** implement the Mission 0 nucleus without breaking old
media refs: normalize legacy refs into `source_entities`, write durable source
entity metadata on revise, render compact inline expandable source entities
plus source cards from the same object family, preserve source identity through
edit serialization/reload/history, and verify on product paths.

**highest-impact remaining uncertainty:** whether source entities can round-trip
through VText's contenteditable render/edit/serialize path without being erased
or flattened into ordinary prose.

**next executable probe:** add a backend source entity normalizer/migration path
for current YouTube/image media refs and a focused runtime test proving the
appagent revise run metadata contains both legacy refs and the new
`source_entities` shape.

## Run Checkpoint & Resumption State - 2026-05-31

**status:** checkpoint_incomplete

**last checkpoint:** Mission 0 Source Entity nucleus landed and deployed at the
behavior commit.

**current artifact state:** VText now has a general `source_entities` metadata
shape normalized from legacy YouTube/image `media_source_refs`. VText revise
run metadata carries both legacy refs and source entities. Appagent-authored
revisions preserve `source_entities` as durable metadata. The VText frontend
renders source entities as compact expandable source affordances and source
cards, and the source card can open the source in its owning media/app surface
through the desktop launch path.

**what shipped:**

- Backend `vtextSourceEntity` nucleus with target, selectors, display,
  evidence, and provenance fields.
- Lazy normalization from legacy `media_source_refs` into `source_entities`.
- Prompt context for detected source entities.
- Durable metadata carry-forward for `source_entities`.
- Research-state marking for both legacy media refs and source entities.
- VText rendering from `source_entities`, with legacy fallback.
- Inline expandable source affordances and source deck cards.
- Owning-surface launch from VText source cards.
- Playwright coverage for rendering, expansion, and opening the Video app from
  a YouTube source entity.

**commits:**

- `a4d1e0d` docs: record vtext source entity mission checkpoint
- `3949a4f` feat: add vtext source entity nucleus
- `e6bf0af` docs: clean vtext source entity mission formatting
- `e9a0996` test: avoid fetching fixture source entity content
- `4081d1f` test: target vtext source entity article

**what was proven:**

- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextAgentRevisionRegistersMediaSourceRefs|TestMarkVTextMediaSourceRefsResearchState|TestFetchYouTubeTranscriptUsesConfiguredProvider|TestFetchYouTubeTranscriptUsesInnerTubeAndroidFallback' -count=1`
  passed.
- Frontend compiled from a temporary clean npm install with
  `npm ci && npm run build`.
- GitHub Actions passed for final pushed commit `4081d1f`: CI run
  `26722450619`, FlakeHub publish run `26722450617`.
- Staging `/health` reported proxy and sandbox deployed at behavior commit
  `e6bf0afffbf249be0904f36559743ece338c4afc`.
- Deployed Playwright acceptance against `https://choir.news` passed:
  `BASE_URL=https://choir.news ... npx playwright test tests/vtext-source-entities.spec.js --project=chromium --timeout=120000`.

**unproven or partial claims:**

- Source entities are not yet inserted at arbitrary prose spans; Mission 0
  renders a compact source affordance rail plus source deck from durable
  metadata.
- YouTube transcript span selection is not yet implemented.
- Podcast transcript review, web source packets, private VText-to-VText
  transclusion, publication projection from private source entities, unified
  source search, and sourcecycled manifest import remain future campaign
  missions.
- Final pushed test-only commits are newer than the deployed behavior commit;
  staging behavior proof is tied to `e6bf0af`, which contains the product code.

**belief-state changes:**

- The VText editor can render and serialize around source entity affordances
  without flattening those affordances into document prose.
- Owning-surface opening can be implemented through existing desktop app launch
  events rather than a new source browser.
- The initial source entity object can absorb the current YouTube/image media
  refs without a destructive migration.

**remaining error field:** turn the source affordance rail into true
claim/span-attached inline source refs; add transcript span selectors; resolve
source entities through explicit APIs; broaden to podcast/web/VText targets;
project private source entities into public-safe platform citation and
transclusion records.

**highest-impact remaining uncertainty:** whether span-attached inline refs can
round-trip through contenteditable editing, appagent revision writing, and
publication without brittle DOM anchoring.

**next executable probe:** implement an explicit inline source-ref syntax or
structured anchor for one source entity in VText prose, then prove edit,
serialize, revise, reload, and history preserve the anchor and resolve it to
the same source entity.

**suggested resume goal string:**

```text
/goal Resume docs/mission-vtext-source-entities-multimedia-transclusion-v0.md from the 2026-05-31 Mission 0 checkpoint. Implement span-attached inline SourceEntity refs for one YouTube source entity, preserving the existing source_entities metadata and legacy media_source_refs fallback. Prove that an inline source ref survives edit, serialize, revise, reload, history navigation, expansion, and owning Video app open on staging without flattening source identity into prose.
```

## Run Checkpoint & Problem Record - Inline Source Refs - 2026-05-31

**status:** checkpoint_incomplete

**last checkpoint:** Source Entity nucleus is deployed and proven for
metadata-backed source affordance rail/deck rendering.

**current artifact state:** VText source entities are durable revision metadata
and can render as expandable source affordances plus owning-surface open
actions. They are not yet attached to exact claims or source references in the
document prose.

**problem being fixed before code changes:** current rendering proves source
identity can survive as document-level metadata, but the authoring surface still
cannot express "this phrase/claim cites this source entity" inside the prose.
Without a span-attached inline ref, source entities risk remaining a source
deck bolted onto the document rather than becoming first-class inline citation
and transclusion entities.

**evidence:**

- `frontend/src/lib/VTextEditor.svelte` renders source entities from
  revision metadata as a rail and deck before the Markdown body.
- The serializer skips `[data-vtext-source-entity]` nodes so those affordances
  do not flatten into prose.
- There is no source-aware Markdown/link syntax that renders a source entity at
  a chosen text location and serializes back to the same source reference.

**remaining error field:** implement the smallest syntax and render path for
one inline source ref, preferably Markdown-compatible, that resolves to a
`source_entities` entry, renders as a compact expandable/clickable source
entity, preserves the authored label and entity id, and serializes back to the
same source-ref syntax.

**highest-impact remaining uncertainty:** whether contenteditable can preserve
source-backed inline refs without turning them into plain text, deleting them,
or confusing ordinary links.

**next executable probe:** support `[label](source:ENTITY_ID)` in VText
Markdown rendering/serialization, render it as an inline source entity chip
with compact expandable details, and prove edit/reload/open behavior in the
existing source entity Playwright spec.

## Run Checkpoint - Inline Source Refs - 2026-05-31

**status:** checkpoint_incomplete

**what shipped:**

- VText Markdown rendering now recognizes `[label](source:ENTITY_ID)` as an
  inline Source Entity reference rather than an ordinary external link.
- Inline refs resolve against revision `source_entities` metadata, render as a
  compact non-editable chip, expose source kind and evidence state, and expand
  in-place to show source facts.
- Inline refs serialize back to the same `[label](source:ENTITY_ID)` form when
  the contenteditable VText surface is saved.
- Missing inline refs render as missing-source chips instead of silently
  flattening source identity into prose.
- The existing source entity Playwright spec now proves rail/deck rendering,
  inline expansion, Video app opening, browser-like typed edit, autosave
  revision creation, and source-ref preservation in the saved revision.

**commits:**

- `552a927` docs: record inline vtext source ref problem
- `6446e13` feat: render inline vtext source refs
- `24a6688` test: prove inline vtext source ref edit round trip

**what was proven:**

- Clean frontend build passed from a temporary npm install:
  `npm ci && npm run build`.
- GitHub Actions passed for behavior commit `6446e13`: CI run
  `26722625502`, FlakeHub publish run `26722625503`.
- Staging `/health` reported proxy and sandbox deployed at behavior commit
  `6446e139ccbbcb6a88c49229b3041aa8583ec935`, deployed at
  `2026-05-31T19:46:25Z`.
- Deployed Playwright acceptance against `https://choir.news` passed:
  `BASE_URL=https://choir.news ... npx playwright test tests/vtext-source-entities.spec.js --project=chromium --timeout=120000`.
- The acceptance created a VText over a real YouTube URL, rendered the inline
  source ref and source deck, expanded the inline ref, expanded the media
  iframe, opened the owning Video app, typed a user-like edit into VText,
  observed a saved revision containing `Round-trip note.`, and verified that
  the saved revision still contained `[source](source:src-fixture-youtube)`.
- GitHub Actions passed for final test-only commit `24a6688`: CI run
  `26722764019`, FlakeHub publish run `26722764020`.

**important observation:**

The VText revisions API does not guarantee oldest-to-newest ordering for the
test's purposes. The acceptance now finds the saved revision by content instead
of assuming the array tail is the latest revision.

**unproven or partial claims:**

- Inline refs are Markdown-compatible source anchors, not yet structured DOM
  spans with transcript offsets or claim-level selector ranges.
- The proof covers private VText rendering and autosave, not publication
  projection into public immutable citation/transclusion records.
- Transcript availability remains `unavailable` for the deployed YouTube
  fixture; transcript span selection is still a future deformation.
- Podcast, web source packets, VText-to-VText transclusion, global/local source
  search, and sourcecycled manifest import remain future campaign work.

**remaining error field:** attach source refs to richer selectors and source
artifact resolution APIs; resolve transcript spans when available; preserve
inline refs through VText agent-authored revisions; project private source
entities into public platform publication records; prove VText-to-VText
transclusion and source expansion without a parallel source browser.

**highest-impact remaining uncertainty:** whether VText agent revisions will
preserve, move, or intentionally rewrite inline source refs while using source
entities as evidence rather than flattening them into prose.

**next executable probe:** have VText revise a document containing
`[label](source:ENTITY_ID)` plus `source_entities` metadata, then prove the
agent-authored next version keeps the source ref attached to the relevant claim
or emits a durable reason when it deliberately changes the citation.
