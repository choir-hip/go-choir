# MissionGradient: YouTube And Image Review Studio v0

**Status:** draft
**Date:** 2026-05-30
**Related docs:** [project-goals.md](project-goals.md),
[current-architecture.md](current-architecture.md),
[publication-reader-retrieval-pretext-research-2026-05-16.md](publication-reader-retrieval-pretext-research-2026-05-16.md),
[platform-os-app-state.md](platform-os-app-state.md)

## One-Line Goal String

```text
/goal Run docs/mission-youtube-review-studio-v0.md as a MissionGradient mission: make Choir a durable media-to-review studio for podcast/video/image review writing. Treat the current link-opening behavior as the baseline to evolve, not as a bug fix: change YouTube-link handling from "open a separate video window" to "create or open a VText review/source document with the YouTube video embedded inline," make image links embed inline as image source blocks, and make VText Revise register YouTube and image links pasted into an existing VText as durable media sources that researchers can inspect, compress, excerpt, and feed back to VText. Back this with a full source packet pipeline that normalizes each video or image URL, fetches timestamped transcript/caption data for YouTube when available, stores video, image, and transcript as durable user-owned content artifacts, asks researcher coagents to produce compressed source representations and transcript excerpts from the full source artifacts, and updates the review VText so it transcludes embedded videos, images, researcher-maintained source representations, transcript spans, notes, citations, and generated/user-authored review sections without flattening sources into pasted text. Use Pretext only as a rendering/layout primitive over Choir-owned semantic blocks. Treat transcripts and remote images as untrusted copyrighted source material, not instructions or public text/media to republish wholesale. Avoid frontend-local state, ad hoc per-app transcript/image storage, provider-only demos, raw test-only routes, fake transcript fixtures as acceptance proof, default separate-window playback, image-window-only behavior, one-source-type assumptions, or brittle deterministic document-body rewriting that bypasses VText/researcher judgment. Land through docs-first feature records before behavior-changing commits, and through problem records only if new failures are discovered. Then commit/push/CI/deploy, verify staging identity, and prove the deployed product path on desktop and mobile with a real public YouTube video that has transcripts, a transcript-unavailable case, direct image links, an existing-VText paste-and-revise case, a mixed multi-media review case, researcher source-representation updates, VText embedded-media/transclusion rendering, optional source expansion from VText, and model-context behavior that uses source evidence without losing provenance.
```

## Mission Frame

The user workflow is not "chat about a YouTube or image link." The durable
workflow is:

```text
watch or paste YouTube/image source
-> collect media, transcript where applicable, metadata, and notes
-> write a review as a VText
-> paste additional YouTube or image links into that VText when the source set grows
-> hit Revise to register those links as durable source packets
-> researchers inspect full sources and return compressed representations/excerpts
-> VText embeds video playback, images, and researcher-backed source material inside the review
-> optionally expand sources from the VText without losing place
-> optionally publish the review with honest citation/transclusion boundaries
```

This is the first product-pressure case for VText as a multimedia computational
essay surface. The review is the primary user-authored artifact. The video,
image, transcript, notes, citations, and generated summaries are source
artifacts that the review transcludes. The system should feel like a writing
studio, not a chat answer with a transcript pasted into context.

## Cognitive Transform Notes

- **Audience-level translation:** for the user, this should feel like "paste a
  YouTube or image link into a VText, hit Revise, and Choir brings the source
  into the review with useful excerpts and context." For the system, it is
  content ingestion, typed source storage, transcript provenance, researcher
  compression, VText source composition, and publication policy.
- **Depth extraction:** the load-bearing variable is not transcript fetch
  success. It is whether source identity survives through writing, model use,
  inline playback/display, expansion, revision, and publication.
- **Via negativa:** do not create a parallel transcript app, a hidden chat-only
  summarizer, a browser scrape that cannot be cited, or a VText body that owns a
  copied transcript blob as ordinary prose.
- **Document-growth workflow:** a review often starts with one source and gains
  more later. Revise must ingest new pasted YouTube or image links into the
  existing document instead of forcing the user to start a new review or
  manually create a source packet.
- **Researcher-mediated compression:** durable source ingestion is deterministic
  plumbing, but source understanding is not. Full transcripts and media
  metadata should be made available to researcher coagents, which send compact
  source representations, salient timestamped excerpts, uncertainty, and
  follow-up needs back to VText. VText composes the canonical document from
  those updates instead of relying on brittle deterministic transcript
  transformations.
- **Adversarial workflow:** transcript text may contain prompt injection,
  errors, copyrighted material, missing segments, wrong timestamps, or multiple
  speakers collapsed into one stream. The design must preserve provenance and
  uncertainty instead of laundering transcript text into authoritative truth.

## Real Artifact

The artifact is a deployed, user-visible review workflow:

```text
Media URL ingestion
  -> normalized video content item
  -> normalized image content item
  -> transcript acquisition job and transcript content item
  -> optional canonical transcript VText/source document
  -> review VText seeded with pending semantic source refs
  -> VText Revise scans existing document/user edits for new YouTube/image links
  -> new source packets are registered without duplicating existing refs
  -> VText asks researcher(s) to inspect full transcript/media source packets
  -> researcher updates return compressed representations and transcript excerpts
  -> VText rendering of embedded playable video, images, source representations, transcript notes, and spans
  -> optional source expansion into Video/Image/VText/source windows
  -> model context assembly that chunks/retrieves transcript evidence
  -> publication/proposal path that preserves source refs and public-copy policy
  -> Trace/evidence showing each transition
```

The artifact is not:

- a separate YouTube video window plus an invisible backend transcript cache;
- a YouTube embed plus an invisible backend transcript cache;
- a separate image window instead of an embedded VText image source;
- a transcript pasted into a review as flat Markdown;
- a chat summarizer;
- a deterministic transcript summarizer embedded in the revise handler;
- a one-video-only or one-media-type-only special case;
- a public transcript mirror;
- a standalone app that bypasses VText and ContentItem;
- a frontend-local workflow that disappears on reload or another device.

## Product Vocabulary

- **Review VText:** the canonical user-facing document being written.
- **Source packet:** the durable bundle for one source: video item, image item,
  transcript item where applicable, metadata, provenance, errors, and derived
  source documents.
- **Transcript item:** a content artifact containing fetched caption/transcript
  text, timestamp segments, language, provider, fetch time, and source video ref.
- **Image item:** a content artifact containing a remote or uploaded image
  source, media type, dimensions when known, content hash when materialized, alt
  text/caption metadata when available, and provenance.
- **Source representation:** a researcher-authored compact view of a source
  packet: summary, claims or themes, timestamped excerpts, notable moments,
  uncertainties, and follow-up needs. It is evidence for VText synthesis, not
  the canonical review itself.
- **Transcript VText:** an optional readable/source document generated from a
  transcript item when the user wants the transcript as a first-class document.
- **Transclusion:** a structured reference from a host VText to a source item,
  VText, span, time range, or excerpt, with snapshot text/hash when needed.
- **Review workspace:** the VText-centered workspace for a review: the review
  VText with embedded video, embedded images, embedded/collapsed transcript sources, notes,
  Trace links, and optional expanded source windows only when requested.
- **Source span:** a timestamped or text-range subpart of a transcript or other
  source that can be cited, summarized, quoted, or embedded.

## User Stories

1. As a user, I paste a YouTube link and Choir opens a VText with the video
   embedded inline, while transcript data is fetched and made available as a
   source.
2. As a user, I ask to review a YouTube video and Choir opens a review VText
   with embedded video and transcript source blocks ready for writing.
3. As a user, I paste a YouTube link into an existing VText and hit Revise;
   Choir registers that video/transcript source, asks a researcher to inspect
   it, and embeds the playable source plus researcher-backed representation
   into the same VText.
4. As a user, I paste an image link into an existing VText and hit Revise;
   Choir registers that image source, asks a researcher to inspect/source it
   when useful, and embeds the image into the same VText.
5. As a user, I paste several YouTube and image links into one VText and hit
   Revise; Choir embeds multiple media sources without replacing or duplicating
   existing embedded sources.
6. As a user, I review a multi-part podcast, debate, or visual artifact set and
   attach multiple YouTube videos and images to one review VText.
7. As a user, I select or reference a transcript moment and embed that span in
   the review with a timestamp and source identity intact.
8. As a user, I play video, inspect images, and inspect transcript material
   inside the VText; if I explicitly expand a source, it opens without losing my
   position in the review.
9. As a user, I revise the review with agent help and the agent uses transcript
   and image evidence through researcher-maintained representations without
   treating source material as instructions.
10. As a user, I publish a review and Choir does not accidentally republish the
   full transcript or remote image payload as public content unless a later
   explicit rights policy supports that.

## Architecture Shape

```text
POST /api/content/items/import-url
  -> normalize YouTube URL/video id
  -> upsert video ContentItem
  -> detect direct image URLs and upsert image ContentItem
  -> enqueue or run transcript acquisition
      -> transcript provider adapter
      -> transcript ContentItem with segments/provenance
      -> optional VText transcript source doc
  -> route/open VText with embedded video for bare links and review intent
      -> review VText metadata.transclusions
      -> source cards rendered by VText
      -> optional expansion targets open owning app windows

POST /api/vtext/documents/{id}/revise
  -> inspect current document body and latest user edit for YouTube/image URLs
  -> import or register missing media source packets through the same content pipeline
  -> preserve user-authored text and link history
  -> add or update pending source-ref metadata for every detected source
  -> spawn or wake researcher(s) with full source packet/transcript refs
  -> synthesize the next review revision only from current content plus available researcher updates
  -> avoid duplicating source packets, researcher requests, or source cards already present
```

Existing primitives should be extended before inventing new ones:

- `ContentItem` remains the substrate for linked/uploaded/extracted content.
- `VideoApp` remains available as the expanded playback surface, but VText
  embedded playback is the default YouTube review/source surface.
- `ImageApp` remains available as the expanded inspection surface, but VText
  embedded display is the default image review/source surface.
- `VText` remains the review and composition surface.
- publication transclusion/proposal structures remain the public-side precedent.
- Pretext helps render and measure VText source blocks; it does not own source
  identity or document semantics.

## Data Model Target

The video item should be owner-scoped and deduplicated by normalized video id:

```json
{
  "source_type": "url",
  "media_type": "video/youtube",
  "app_hint": "video",
  "source_url": "https://www.youtube.com/watch?v=...",
  "canonical_url": "https://www.youtube.com/watch?v=VIDEO_ID",
  "metadata": {
    "platform": "youtube",
    "video_id": "VIDEO_ID",
    "title": "...",
    "channel": "...",
    "duration_seconds": 1234
  }
}
```

The transcript item should be separately addressable:

```json
{
  "source_type": "derived_transcript",
  "media_type": "text/x-youtube-transcript",
  "app_hint": "vtext",
  "source_url": "https://www.youtube.com/watch?v=VIDEO_ID",
  "canonical_url": "youtube://VIDEO_ID/transcript/en",
  "text_content": "full transcript text for private source use",
  "metadata": {
    "platform": "youtube",
    "video_content_id": "content-...",
    "video_id": "VIDEO_ID",
    "language": "en",
    "kind": "manual|auto",
    "provider": "youtube-transcript-api",
    "segments": [
      {"start": 12.3, "duration": 4.2, "text": "..."}
    ],
    "fetched_at": "2026-05-30T00:00:00Z",
    "availability": "available|unavailable|pending|error"
  },
  "provenance": {
    "rights_scope": "private_user_source",
    "untrusted_source_text": true
  }
}
```

The image item should use the same source substrate:

```json
{
  "source_type": "url",
  "media_type": "image/jpeg",
  "app_hint": "image",
  "source_url": "https://example.com/image.jpg",
  "canonical_url": "https://example.com/image.jpg",
  "metadata": {
    "kind": "image",
    "width": 1600,
    "height": 900,
    "alt_text": "...",
    "caption": "...",
    "materialized": true
  },
  "provenance": {
    "rights_scope": "private_user_source",
    "untrusted_source_media": true
  }
}
```

The review VText should carry source references in revision metadata before the
editor has a full block model:

```json
{
  "transclusions": [
    {
      "type": "media_source",
      "source_kind": "content_item",
      "source_id": "video-content-id",
      "display": {"mode": "embedded_video_card"}
    },
    {
      "type": "media_source",
      "source_kind": "content_item",
      "source_id": "image-content-id",
      "display": {"mode": "embedded_image_card"}
    },
    {
      "type": "transcript_source",
      "source_kind": "content_item",
      "source_id": "transcript-content-id",
      "selector": {"kind": "time_range", "start": 0, "end": 600},
      "display": {"mode": "collapsed_transcript_card"}
    },
    {
      "type": "source_representation",
      "source_kind": "researcher_update",
      "source_id": "update-youtube-summary-id",
      "content_item_refs": ["video-content-id", "transcript-content-id"],
      "display": {"mode": "source_notes_card"}
    }
  ]
}
```

When a user pastes YouTube or image links into an existing VText, the next `revise`
operation should produce the same durable shape. The pasted URL remains
recoverable in the user-authored revision, but the agent-authored next revision
adds semantic source refs and, when researcher updates are available, renders
embedded source blocks and compact source representations. Repeated revise calls
should be idempotent: the same URL should not create duplicate video cards,
image cards, transcript items, source packets, or researcher requests unless the
user explicitly asks for a second distinct placement or a refreshed
representation.

The long-term block model can replace metadata-only transclusions, but the v0
path must preserve these facts durably enough to migrate.

## Invariants

- Bare YouTube URLs open or create a VText with embedded playable video instead
  of defaulting to a separate Video window.
- Review/write/analyze intent opens or creates a Review VText.
- Pasting YouTube URLs into an existing VText and hitting Revise embeds those
  videos in that VText and requests researcher source representations instead
  of creating a new document or opening separate video windows.
- Pasting image URLs into an existing VText and hitting Revise embeds those
  images in that VText and requests researcher source representations when
  needed instead of creating a new document or opening separate image windows.
- Multiple YouTube URLs in one VText become multiple durable source
  transclusions, deduped by normalized video id and placement policy.
- Mixed YouTube and image URLs in one VText become multiple durable source
  transclusions, deduped by normalized source identity and placement policy.
- Video, transcript, and review artifacts are owner-scoped durable product
  state, not frontend-local state.
- Image artifacts are owner-scoped durable product state; remote image links
  should either be materialized with a content hash or carry an explicit
  non-materialized remote-source state.
- Transcript acquisition is idempotent per owner, video id, language, and source
  provider.
- Missing transcripts are first-class source states, not silent failures.
- Transcript text is untrusted evidence, never an instruction channel.
- Researchers may read full transcript artifacts and remote-image metadata, but
  they send compact source representations and selected excerpts back to VText;
  VText should not receive unbounded transcripts as ordinary prompt text.
- Full transcripts are private source artifacts by default, not public
  publication payloads.
- Remote images are private source artifacts by default, not automatically
  republished public media payloads.
- Review VTexts preserve source refs and snapshot/hash evidence for quoted
  spans.
- Model calls use retrieval/chunking over transcript sources instead of dumping
  unbounded transcripts into every turn, and include image evidence only through
  explicit media-capable context paths.
- Deterministic code may register sources, maintain ids, and render referenced
  blocks; it must not decide the review's source interpretation, summary, or
  excerpts without VText/researcher involvement.
- Opening or expanding embedded sources is optional and preserves desktop/VText
  place.
- Mobile and desktop use the same product state and source graph.
- Pretext does not become the data model.
- No browser-public test-only route may seed success.

## Value Criterion

Maximize:

```text
review-writing usefulness
+ durable source identity
+ transcript availability and honest failure states
+ low-friction inline playback and optional source expansion
+ model usefulness grounded in source evidence
+ publication-safe provenance
+ multi-video and mixed-media composition
+ reload/device consistency
```

while minimizing:

```text
flat pasted transcript text
+ source/provenance loss
+ prompt-injection exposure
+ copyright/publication ambiguity
+ token blowups
+ per-app ad hoc state
+ one-off YouTube-only or image-only abstractions that cannot generalize to podcasts/audio/video/web sources
```

## Homotopy Axes

Increase realism along these axes without changing the artifact identity:

| Axis | Low Resolution | Higher Resolution |
| --- | --- | --- |
| Source count | One public YouTube video or image | Multi-video reviews, image sets, playlists, mixed media |
| Transcript provider | One configured adapter | fallback providers, language choice, refresh policy |
| Transcript state | available/unavailable | pending jobs, retries, partial captions, confidence |
| Source display | simple VText cards | Pretext layout, inline spans, side notes, columns |
| Review scaffold | seeded headings | user-specific review templates and learned style |
| Model context | chunk by timestamp | retrieval, source ranking, quote budget, summaries |
| Researcher representation | first compact source notes | refreshed source maps, excerpt indexes, contrasting viewpoints |
| Display/expansion | embedded video/image in VText | synchronized playback/transcript position and optional expanded windows |
| Publication | private review only | public review with citation/transclusion policy |
| Verification | deterministic API tests | staging browser video, mobile, real transcript cases |

A low-resolution version is valid only if it uses the same ContentItem/VText
source graph, same durable transcript/image state, same transclusion semantics,
and same product path as the full version.

## Dense Feedback

Backend/unit evidence:

- YouTube URL normalization handles `youtube.com/watch`, `youtu.be`, mobile,
  shorts where appropriate, query noise, and playlist noise.
- Image URL detection handles direct image paths, image content types, query
  noise, and unsupported remote states honestly.
- Content import creates or reuses video and image items idempotently.
- Transcript acquisition creates transcript items with segment metadata.
- Transcript-unavailable and transcript-error states persist with provenance.
- Review VText creation stores transclusion metadata for one and many video/image
  sources.
- VText Revise detects newly pasted YouTube/image links, imports missing source
  packets, preserves user text, updates source-ref metadata idempotently, and
  creates at most one pending researcher obligation per new source set.
- Researcher updates can reference full transcript items, return compact source
  representations, and include selected timestamped excerpts without copying
  the full transcript into VText.
- Model context assembly treats transcript text as quoted evidence, includes
  image evidence only through explicit media-capable context paths, and enforces
  chunk/size bounds.

Frontend/browser evidence:

- Bare YouTube or image paste opens or creates a VText with embedded media and
  source state.
- Review intent opens a Review VText without requiring a separate source window.
- Pasting one or more YouTube/image links into an existing VText and clicking
  Revise embeds the corresponding media into that same VText.
- The first VText revision after link insertion can show pending researcher
  source notes; later researcher delivery updates the compact representation
  without losing the embedded media refs.
- Repeating Revise does not duplicate already embedded media sources.
- VText renders embedded video, image, and transcript cards.
- Embedded video playback and image display work inside VText; transcript cards
  expand inline or open a transcript source only when requested.
- Reload preserves review workspace, selected source state, window state, and
  document state.
- Mobile 390x844 preserves the same state and interaction path.

Staging acceptance evidence:

- Deployed commit identity matches the pushed SHA.
- A real public video with transcript succeeds.
- A very new, live, private, or transcript-disabled video records an honest
  unavailable/error state.
- Direct image links create embedded image source blocks.
- Existing-VText paste-and-revise turns pasted YouTube/image links into
  embedded source blocks without replacing user-authored prose.
- Researcher delivery returns a compact representation and transcript excerpts
  for the full transcript-backed source packet.
- A mixed multi-media review records multiple source packets and transclusions.
- The model-generated/revised review cites or references transcript spans and
  image sources without losing source identity.
- Public publication does not expose full private transcript text or remote
  image bytes by default.

## Forbidden Shortcuts

- Do not store transcript state only in Svelte component state.
- Do not add YouTube transcript data as a special field on Video app state when
  `ContentItem` can own it.
- Do not add image-link data as a special field on Image app state when
  `ContentItem` can own it.
- Do not paste the full transcript into the review VText as ordinary body prose.
- Do not claim transcript support from a fake fixture alone.
- Do not implement transcript summarization as a deterministic backend string
  transform; use researcher/VText agent flow for interpretation, compression,
  and excerpt selection.
- Do not use a provider key or local Python script path that cannot run in
  staging or candidate computers.
- Do not treat an embed iframe as evidence that transcript ingestion works.
- Do not let transcript text override system/developer/user instructions.
- Do not publish full transcript text as part of a public review by accident.
- Do not publish remote image bytes as part of a public review by accident.
- Do not make Pretext a semantic persistence layer.
- Do not bypass conductor/VText ownership for review-writing intent.

## Implementation Pressure

1. Document the current baseline behavior, intended source graph, and feature
   contract before code changes.
2. Extend content ingestion to normalize YouTube ids, detect image URLs, and
   store video/image metadata.
3. Add a transcript acquisition adapter behind a small backend interface.
4. Store transcript items, image items, and availability/materialization states
   in `ContentItem`.
5. Add product APIs for source packets and review-workspace creation if existing
   content/VText APIs cannot express the workflow cleanly.
6. Seed Review VText documents with durable transclusion metadata and useful
   review structure.
7. Teach VText Revise to discover YouTube and image URLs in the current document,
   import missing source packets, add idempotent embedded-media/transcript refs,
   and create researcher obligations for source compression/excerpts.
8. Teach researcher/VText prompts and tools to pass source-packet refs, compact
   representations, timestamped excerpts, and refresh requests without copying
   full transcripts into canonical VText.
9. Render playable video, image, transcript source cards, and researcher source
   representation cards in VText, with optional expansion targets in the
   desktop.
10. Add source-aware model-context assembly with untrusted-source guards,
   bounded transcript retrieval/chunking, and explicit media-capable image
   context.
11. Add publication safeguards so public reviews preserve citations without
   mirroring full private transcripts or remote image bytes.
12. Verify on staging with real videos, direct image links, missing transcript
    cases, existing-VText paste-and-revise, repeated-revise dedupe, mixed-media
    reviews, reload/device persistence, desktop, and mobile.

## Verification Matrix

| Claim | Required Evidence |
| --- | --- |
| YouTube playback is VText-embedded | deployed browser proof for bare link opening VText with playable embed |
| Image display is VText-embedded | deployed browser proof for image link opening/embedding in VText |
| Transcript fetch works | real public video creates transcript item with segments |
| Missing transcripts are honest | unavailable/error state visible and durable |
| Review VText owns composition | review doc/revision contains transclusion refs |
| Existing VText paste-and-revise works | pasted YouTube or image URL in an existing VText becomes an embedded source on Revise |
| Multi-link revise works | several pasted YouTube/image URLs become multiple embedded sources in one VText |
| Researcher representation loop works | researcher update references transcript/media source packets and returns compact notes/excerpts to VText |
| Repeated revise is idempotent | same URL does not create duplicate source packets, cards, or researcher obligations |
| Optional source expansion works | embedded source can open expanded Video/Image/transcript window without replacing the VText |
| Mixed-media reviews work | one review VText references multiple video/image/transcript source packets |
| Reload/device state is stable | desktop and 390x844 reload proof |
| Agent revisions are grounded | revision uses transcript spans and image refs with source identity |
| Prompt injection is contained | transcript instruction text and remote metadata cannot control agent |
| Publication is rights-aware | public review omits full transcript and remote image bytes by default |

## Run Checkpoint & Resumption State

### 2026-05-30 Staging Transcript Probe Finding

After commit `0e4fbaa` deployed, staging product-path API proof with a newly
registered QA user showed that VText Revise created durable source packets for
two YouTube links and one direct image link, and repeated Revise returned the
same pending run without duplicating content items. The direct image was stored
as an owner-scoped `image/jpeg` ContentItem. Both YouTube links created
`video/youtube` ContentItems plus derived transcript status ContentItems, but
the nominal transcript-backed public video reported `unavailable` with
`caption track had no text`. The likely root cause is caption URLs that already
carry a non-JSON `fmt` parameter; the first implementation only appended
`fmt=json3` when no `fmt` existed, so a caption URL such as `fmt=srv3` could be
fetched and then parsed as JSON. This blocks the real-transcript acceptance case
until the caption URL format is forced to JSON3 or a transcript provider adapter
replaces the direct YouTube timedtext path.

### 2026-05-30 Deployed Source-Packet Proof

Commit `673a8ed38113b3822725598599739ae87d76b206` deployed successfully to
staging after CI run `26676482105`. Product-path API and browser proof with QA
user `qa-1780121526346-qb1lh6@example.com` created VText document
`594ecf95-8ae8-4add-b2ac-a1a018cb4494`, pasted two YouTube links and one
direct image link, and submitted Revise through
`07d1bdd0-4039-495a-b637-66904a5452bf`.

Evidence from the deployed path:

- appagent revision `b7a7ec01-6b3a-4bf9-af45-91939fa23cca` carried 3
  `media_source_refs`;
- one researcher update was consumed into the VText revision metadata;
- content items included 2 `video/youtube` refs, 2 derived transcript status
  refs, and 1 `image/jpeg` ref;
- repeated Revise returned a new run handle without increasing relevant
  content item count (`5 -> 5`);
- desktop rendering showed 3 source cards, 2 YouTube iframes, and 1 image;
- mobile 390x844 rendering showed the same 3 source cards with no horizontal
  overflow.

The same proof still did not satisfy the real-transcript acceptance case:
YouTube transcript items remained `unavailable` (`caption track had no text` or
`caption tracks unavailable`). The source-packet and researcher-mediated review
path is now deployed at low resolution, but transcript acquisition needs a
stronger provider strategy before the mission can be complete.

```text
status: checkpoint_incomplete
last checkpoint: deployed product-path source-packet proof at 673a8ed
current artifact state: Content import special-cases YouTube URLs into owner-scoped video ContentItems, best-effort transcript status ContentItems with availability/provenance, and canonical video-id dedupe; VText Revise scans current document content for YouTube/direct-image URLs, imports/registers missing source packets, stores durable media_source_refs in revision/run metadata, and marks researcher work required only when new source refs are added; appagent revisions carry those refs forward; VText renders source cards for YouTube and image refs from revision metadata without frontend-local persistence.
what shipped: commits 0e4fbaa, 78d4092, and 673a8ed pushed to main and deployed to staging
what was proven: focused comprehensive runtime tests prove YouTube import dedupe, revise-time mixed YouTube/image source-ref registration, prompt source contract, unavailable transcript state, image source registration, durable metadata propagation input, and repeated source registration dedupe; frontend production build proves embedded-card code compiles; deployed staging proof proves mixed YouTube/image Revise registration, researcher update consumption, relevant content-item dedupe, desktop embedded source cards, and mobile no-overflow rendering
unproven or partial claims: real public YouTube transcript acquisition in staging, transcript segment quality, bare prompt-surface link opening as Review VText, optional source expansion, publication safeguards, reload/device proof beyond persisted VText window restoration, and source-aware model-context retrieval over full transcript text
belief-state changes: ContentItem plus VText metadata are sufficient for the first low-resolution source-packet path; direct YouTube HTML/timedtext transcript scraping is not reliable enough from staging and should be replaced or wrapped by a stronger transcript provider adapter
remaining error field: transcript provider brittleness against YouTube HTML/caption changes, image materialization/cache policy, source-card editing ergonomics, source-ref removal semantics, researcher update schema for source representations/excerpts, model-context retrieval over transcripts, public rights filtering, staging deploy proof
highest-impact remaining uncertainty: whether the best-effort YouTube caption-track fetch works reliably enough on staging for real public videos or needs a dedicated transcript provider adapter/service before product acceptance
next executable probe: implement or configure a stronger transcript acquisition adapter, then rerun the same staging product-path proof with a real public video whose transcript item reaches availability=available with non-empty timestamped segments
suggested resume goal string: /goal Run docs/mission-youtube-review-studio-v0.md through the first receding-horizon loop: document the current baseline behavior and feature contract, inspect the content/VText revise/researcher/publication code paths, implement the smallest durable source-packet and transcript acquisition path that preserves the mission invariants, make paste-YouTube-or-image-link-then-Revise register and embed one or more media sources in the existing VText while routing transcript/media understanding through researcher source representations, and verify on staging with real transcript, transcript-unavailable, direct image, mixed multi-link, researcher representation, and repeated-revise dedupe cases.
evidence artifact refs: CI 26676482105; staging /health proxy+sandbox deployed_commit 673a8ed38113b3822725598599739ae87d76b206; local tests `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestContentImportURLCreatesProvenanceRecord|TestContentImportURLUsesSearXNGAlternateWhenPrimaryLowContent|TestContentCreateSupportsDurableMediaReferences|TestContentImportURLDedupesYouTubeSourcePackets|TestVTextAgentRevisionAcceptsReviseEventWithoutPrompt|TestVTextAgentRevisionRegistersMediaSourceRefs'`; `nix develop -c go test ./internal/runtime -run TestDoesNotExist`; `npm run build` in frontend; screenshots `/tmp/choir-vtext-media-source-cards-673a8ed.png` and `/tmp/choir-vtext-media-source-cards-mobile-673a8ed.png`
rollback refs: revert commits 673a8ed and 0e4fbaa if needed; content artifacts are owner-scoped and addressable by content id, but explicit source-ref deletion/removal UX remains unimplemented
```

### 2026-05-30 Transcript Provider Reprobe

Follow-up direct probes showed that the JSON3 format fix is necessary but not
sufficient. Public watch pages still expose `captionTracks`, but fetching the
track `baseUrl` for several widely captioned public videos returns HTTP 200
with an empty response body for `json3`, `srv3`, `ttml`, and `vtt`. The same
watch pages include a `getTranscriptEndpoint`, but calling the corresponding
Innertube endpoint with the page's `INNERTUBE_API_KEY`, client version,
visitor data, YouTube client headers, origin, and referer returns
`FAILED_PRECONDITION`.

This now looks like YouTube proof-of-origin / subtitle enforcement rather than
only a caption URL parser bug. The best-effort direct caption-track scraper can
remain as an unavailable-state fallback, but it is not a durable product
strategy for staging acceptance. The next implementation step should introduce
a small transcript-provider adapter boundary, using a configured managed
transcript provider when available and preserving the existing unavailable
state when no configured provider or direct YouTube fallback can return
segments. The adapter must still store transcript output only as private
`ContentItem` source material for researcher-mediated source representations,
not as deterministic review prose.

### 2026-05-30 Transcript Adapter Contract

The next runtime slice adds a transcript-provider boundary while keeping the
existing direct YouTube caption-track fetch as an unavailable-state fallback.
The configured-provider path is selected with:

```text
CHOIR_YOUTUBE_TRANSCRIPT_PROVIDER=gettranscript|transcriptapi|youtube-transcript-io|generic-post|generic
CHOIR_YOUTUBE_TRANSCRIPT_API_URL=https://provider.example/...
CHOIR_YOUTUBE_TRANSCRIPT_API_KEY=...
CHOIR_YOUTUBE_TRANSCRIPT_AUTH_SCHEME=bearer|basic|x-api-key|none
```

The adapter accepts common provider response shapes with either a root
`segments` array, nested `transcript`/`captions` arrays, or text fields. The
normalized output remains the same private transcript `ContentItem`: full text,
timestamp segments when present, language/kind/provider metadata, and
untrusted-source provenance. If the configured provider fails or returns no
text, Choir falls back to the direct caption-track probe and stores the durable
unavailable/error state instead of inventing a transcript.

### 2026-05-30 Deployed Transcript Adapter Proof

Commit `5e97651057969019d9e8db66b11324b8b551f391` deployed to staging after CI
run `26676915145`. Staging `/health` reported both proxy and sandbox
`deployed_commit` as `5e97651057969019d9e8db66b11324b8b551f391`.

Local proof for the adapter:

- focused comprehensive runtime tests passed for configured-provider fetch,
  configured-provider `ContentItem` storage, provider payload normalization,
  JSON3 URL forcing, YouTube source-packet dedupe, and VText revise-time media
  source refs;
- normal runtime compile check passed with
  `nix develop -c go test ./internal/runtime -run TestDoesNotExist`.

Deployed product-path probe with QA user
`qa-1780122729501-ngbs32@example.com` imported
`https://www.youtube.com/watch?v=jNQXAC9IVRw` through
`POST /api/content/import-url`. The deployed runtime created video
ContentItem `0b29f7e7-2041-47fe-ac6b-938aa3292fd4` and transcript status
ContentItem `2a705547-1426-4993-a621-3320899e8855` with
`source_type=derived_transcript`,
`media_type=text/x-youtube-transcript`,
`rights_scope=private_user_source`, and `untrusted_source_text=true`.

The transcript item remained `availability=unavailable` with
`provider=youtube_caption_tracks` and `error=caption tracks unavailable`. This
proves the deployed fallback and durable unavailable state, but not the real
transcript acceptance case. Staging does not appear to have a configured
transcript provider yet, so the remaining next step is to configure a managed
provider behind the adapter or implement another durable provider path that can
return real transcript segments from staging.

### 2026-05-30 InnerTube Android Transcript Path

A direct probe of YouTube's Android InnerTube player path returned usable
caption tracks where the web watch-page and timedtext path returned empty
bodies or `FAILED_PRECONDITION`. The runtime now tries transcript acquisition
in this order:

1. configured managed provider, when `CHOIR_YOUTUBE_TRANSCRIPT_PROVIDER` is
   present;
2. built-in Android InnerTube player request
   (`youtube_innertube_android`) to discover caption tracks and fetch JSON3
   caption segments;
3. legacy web watch-page caption-track scrape as an unavailable-state fallback.

This still preserves the mission boundary: InnerTube output is normalized into
the same private transcript `ContentItem` with timestamp segments and
untrusted-source provenance. It does not summarize, excerpt, or rewrite the
review deterministically; researcher/VText still own source interpretation.

### 2026-05-30 Researcher Source-Artifact Access Gap

Fresh staging proof at deployed commit `b84a6052f1f293aee41aa6ce5dcb844587349c7a`
showed that YouTube transcripts now materialize as private transcript
`ContentItem` records and VText can ask a researcher for source
representations. The researcher delivery was consumed by a later VText
revision, but the researcher did not have a direct tool for reading an existing
`content_id` or `transcript_content_id`. It could import URLs and fetch/search
the web, so it treated the source packet refs as identifiers and then
re-probed public URLs instead of reading the already stored transcript
artifact.

That is misaligned with the core invariant for this mission. Deterministic
ingestion may fetch and store source artifacts, but researcher understanding
must be grounded in the durable owner-scoped content substrate when those refs
exist. The next code change should add a read-only content item tool to the
research tool registry, expose private transcript text/segments with
provenance and truncation metadata, and update the VText media-source research
objective to prefer `read_content_item` for every listed `content_id` and
`transcript_content_id` before falling back to URL import or web fetch.

### 2026-05-30 Deployed Researcher Source-Artifact Proof

Commit `e4479a758fb372fe7b19bcc199539e4b3604d8a5` deployed to staging after CI
run `26677725324` and FlakeHub run `26677725334`. Staging `/health` reported
both proxy and sandbox `deployed_commit` as
`e4479a758fb372fe7b19bcc199539e4b3604d8a5`, deployed at
`2026-05-30T07:12:10Z`.

Local proof:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestResearcherReadContentItemReturnsPrivateSourceArtifact|TestVTextAgentRevisionRegistersMediaSourceRefs|TestMarkVTextMediaSourceRefsResearchState'`
- `nix develop -c go test ./internal/runtime -run TestDoesNotExist`

Deployed product-path proof with QA user
`playwright-state-1780125219958-0foc67@example.com` created VText document
`7383ea16-8544-4d73-b893-cf1ef9f53de2`, pasted two YouTube links and one
direct image link, and submitted Revise through
`f9e28bf3-e16d-4f97-81c9-4d5e36e69933`.

Evidence from the deployed path:

- real transcript source `dQw4w9WgXcQ` created video ContentItem
  `7820888d-ca0f-4b3d-af9f-37375701879e` and private transcript ContentItem
  `5ba1d970-013d-4b74-bf31-f7b29bfd01c6` with
  `availability=available`, `provider=youtube_innertube_android`, 61
  timestamped segments, and private/untrusted provenance;
- unavailable transcript source `00000000000` created video ContentItem
  `0c23c674-3299-4337-a6a3-c74edcb1a5a0` and transcript status ContentItem
  `dd6992d9-4f93-432e-a8e0-ef6cd0cb2009` with
  `availability=unavailable` and a precise unavailable-video/caption blocker;
- direct image source created image ContentItem
  `b6280b9f-250f-40b4-8eae-7e7130e069a9` with `media_type=image/jpeg`;
- Trace for the initial revise trajectory contained 10 `read_content_item`
  moments, proving the researcher used stored source artifacts rather than only
  re-probing public URLs;
- the head VText revision consumed one researcher update, carried exactly three
  `media_source_refs`, and marked all three refs `research_state=represented`;
- repeated Revise through `c494b0cf-250b-46f2-8216-e3dfc526321f` preserved the
  same three source refs and did not duplicate source packets for the pasted
  URLs;
- the VText UI rendered three source cards on desktop and mobile: two YouTube
  iframes, one image, represented research labels, and no horizontal overflow.

```text
status: first_receding_horizon_slice_complete
what shipped: durable YouTube/image source-packet registration, Android
InnerTube transcript acquisition, private transcript ContentItems, VText
embedded source cards, researcher read_content_item access to stored source
artifacts, source-ref represented state after researcher consumption, and
repeated-revise source-packet dedupe.
remaining error field: image cache/materialization policy, source-ref removal
semantics, richer typed source-representation schema, model-context retrieval
over large/multiple transcripts, public rights filtering, optional source
expansion windows, and bare prompt-surface URL-to-review defaults.
```
