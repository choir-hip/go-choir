# Repo Survey: Media, Embeds, Captures and Rights Fields

Date: 2026-10-14, at `origin/main` 1f84fa908. Read-only survey; line
numbers are approximate pointers.

**One-line verdict.** Media exists only as a *source entity inside a
citation popover*, never as a document block. A Texture document cannot
hold an image, video or audio node. The editor renders media in citation
popovers; the published Autopaper path does not. The capture pipeline and
the Go RSS parser drop every image and enclosure URL. Rights fields exist
on sources as declarative labels; nothing evaluates robots.txt.

## 1. Texture documents

| what | where | state |
|---|---|---|
| Block types: `paragraph`, `heading`, `bullet_list`, `ordered_list`, `list_item`, `blockquote`, `code_block`, `horizontal_rule`; anything else is rejected | `internal/texturedoc/schema.go` `validateBlock` (~191-265), `SchemaV1 = "choir.texture_doc.v1"` | exists, closed set |
| Inline: `text`, `hard_break`, `source_ref` (`source_entity_id`, `display_mode` = `numbered_ref` or `expanded_ref`) | `schema.go` `validateInline` (~269-312) | exists |
| Marks: `strong`, `emphasis`, `code`; `link` only in world docs | `schema.go` (~316+), `internal/texturedoc/world.go` | exists |
| Source target kinds include `image`, `video`, `audio`, `pdf`, `transcript`, `screenshot`, `video_artifact` | `schema.go` `validSourceTargetKind` (~402-413) | half-built: accepted, little consumes them |
| Open surfaces include `image`, `video`, `audio`, `transcript` | `schema.go` `validOpenSurface`; `internal/sourcecontract/open_surface.go` | half-built |
| Selector kinds `timestamp_range`, `transcript_segment`, `image_region` | `internal/sourcecontract/selector.go:12-16`, schema JSON `source_contract_schema.json:184-187`, generated TS `frontend/src/lib/source-contract.generated.ts:321-328` | **defined, no consumer**. This is the hook for timestamped transcript citations and image-region citations. |
| Source kinds `image`, `video`, `youtube_video`, `audio`, `transcript` | `internal/sourcecontract/source_kind.go:14-19` | defined |
| YouTube and direct image URLs in revision text become source entities (`youtube_video` / `image`, `ExpandedMode: media_player`, `DefaultCollapsed: true`) | `internal/textureowner/texture_media_sources.go` (`classifyTextureMediaSourceURL` :124, `isDirectImageURL` :142, `mediaSourceRefToSourceEntity` ~485); called from `texture_agent_revision.go` ~417 | **the half-started YouTube embed**: works as a citation, not as a figure; has `TranscriptContentID` / `TranscriptAvailability` fields |
| `{{embed <handle>}}` in the Write grammar | `internal/texturewrite/inline.go:12`, `grammar.go:19` | exists; transcludes *text blocks* of a capture, not media |
| Image/figure/video block, caption, alt text, licence, attribution on any node | — | **absent** |

## 2. Rendering, citations, expand/collapse (frontend)

| what | where | state |
|---|---|---|
| Inline citation button (numbered label, hover popover with title, quote, media, "Open source") and `expanded_ref` variant | `frontend/src/lib/texture-source-renderer.ts` `renderInlineSourceRef` (~371-402) | exists |
| Media inside citation popovers: YouTube iframe (`youtubeEmbedURL` in `media-utils.js`), `<video controls>`, `<img loading=lazy>`, `<audio controls>`, `<object>` for PDF | `texture-source-renderer.ts` `sourceEntityMedia` (:284-323) | exists in editor/local path only |
| Expand/collapse: `toggleInlineSourceRef` flips `data-expanded`, collapses siblings, mounts the journal flow; pointer and Enter/Space; media entities get `data-source-expansion-surface="media"` and expand in place | `frontend/src/lib/TextureEditor.svelte` (~1593-1700, CSS ~2286-2350); `texture-source-flow.ts` | exists, the natural home for auto-expanded media |
| Published citations (Autopaper stories): marker with check badge; `{{embed}}` rendered as `<figure class="texture-embed-block">` with pinned blockquotes and figcaption | `frontend/src/lib/texture-published-citations.ts` `renderPublishedCitationRef` (:243), `renderPublishedEmbedBlock` (:263) | exists; **never calls `sourceEntityMedia`**, so published stories show no media |
| Autopaper front page | `frontend/src/lib/AutopaperApp.svelte`, `AutopaperStory.svelte`, `AutopaperRoute.svelte`, `autopaper.ts` (`FrontStory` type ~:18) | no image/media code |
| Front-page row | `internal/platform/publication_front.go` `FrontStory` (:36): id, title, dek, updated_at, version_count, revision, route, importance, promo, section, source_count, words | no image field |
| Front importance scorer (Jev) | `internal/frontimportance`, wired in `cmd/corpusd/main.go` | exists; a sibling "image value" judgement would sit here |
| Desktop media apps | `ImageApp.svelte`, `AudioApp.svelte`, `VideoApp.svelte`, `PodcastApp.svelte` (registry `apps/registry.ts` ~147-188); playback state `internal/mediastate/handler.go` (`media_progress`, `media_recents`) | exist as viewers, not document embeds |

## 3. Captures and storage

| what | where | state |
|---|---|---|
| Capture bodies on host disk, content-addressed `root/<hh>/<sha256>`, returns `sha256:<hex>` | `internal/platform/captures.go` `CaptureBodyStore` (:79), `PutBody` (:86) | exists; image bytes could use the same store |
| Capture metadata object `choir.web_capture`: `ContentBlobID`, `ExtractedTextBlobID`, `BlocksBlobID`, `DocHash`, `Extractor`, `MediaType`, `RawBytes` | `internal/objectgraph/web_capture.go` (:13-20) | exists; no image list, no licence field |
| Capture write path, `choir.capture_blocks.v1` | `internal/sourcegraph/captures.go` (~37-134) | exists |
| Block extraction: kinds include `figure_caption`; `<figure>`, `<picture>`, `<video>`, `<audio>` are block boundaries | `internal/sourcefetch/blocks.go` (~34-44), `readability.go` (~130, ~386-388) | **half-built**: captions kept as text, but `<img src>`, `alt`, `<source>` never recorded; `iframe` dropped (~456) |
| og:image / twitter:image | — | **absent** |
| Go RSS/Atom parser: guid, title, link, description, dates | `internal/sources/rss.go` (~201-222), `sources.Item` in `types.go` (~74) | **drops** `enclosure`, `media:content`, `media:thumbnail`, `itunes:image` |
| Client-side podcast parser reads `enclosure` / `media:content` as `audioUrl` | `frontend/src/lib/podcast.js` (:67, :78-87) | exists (desktop PodcastApp only) |
| Encrypted CAS for computer files | `internal/filecas/filecas.go`, `internal/platform/file_cas_blobs.go` | exists; not used for captures |

## 4. Transcripts and Autoradio

| what | where | state |
|---|---|---|
| YouTube import as `video/youtube` content item | `internal/content/content.go` `importYouTubeURLContent` (~570) | exists |
| YouTube transcript fetch: caption tracks, then **InnerTube player API fallback** (`youtube_innertube_android`), or a configured provider; kill switch `CHOIR_DISABLE_YOUTUBE_TRANSCRIPT_FETCH=1` | `content.go` `fetchYouTubeTranscript` (:704), `fetchYouTubeTranscriptFromInnerTube` (:754), `applyYouTubeInnerTubeHeaders` | exists; **YouTube ToS risk** (automated access outside the official API; see media-sources-and-terms.md §3) |
| Speech-to-text (Whisper or other ASR), oEmbed | — | absent |
| Autoradio | object kind `choir.autoradio_run_sheet` (`internal/objectgraph/registry.go:58`), design in `docs/memo-diagonal-media-strategy-2026-09-17.md` (~124-165) | kind registered, no consumer; no TTS code |

## 5. Rights and provenance fields on sources

| what | where | state |
|---|---|---|
| Per-source `tos_class`, `robots_policy`, `auth_policy`, `store_body_policy`, `retention_days`, `jurisdictions`, `user_agent` | `internal/sources/types.go` (~30-48), `internal/sources/registry.json` | exists as **declarative labels**; `robots_policy` values are `rss_feed` (111), `public_preview_only` (73), `feed_allowed` (27), `dataset_feed` (1) |
| `SourceRobotsPolicy` carried onto items | `internal/sources/types.go:94`, `internal/sourceapi/types.go:150` | carried, not evaluated |
| robots.txt / TDMRep / aipref fetch and evaluation | — | **absent** (no code reads robots.txt or `tdm-reservation`) |
| Quote checker: a quote must match inside one block; renders the source's own span | `internal/quotematch/quotematch.go`; callers `texturewrite/checks.go`, `platform/publication_checks.go`, `researchtools/texture_captures.go` | exists; works unchanged for timed transcript blocks |
