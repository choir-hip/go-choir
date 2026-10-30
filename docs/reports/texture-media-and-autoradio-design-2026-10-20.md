# Texture media, the audio layer, and Autoradio (design, 2026-10-20)

Owner, 2026-10-20 (owner notes §28): visual evidence for self-development
lives inside Texture; "Embedding multimedia in texture is one of my next
goals" (images, video including YouTube, audio including podcasts, slides,
calendar and email); Autoradio is then built "with texture and its audio
layer", "feature complete without the full desktop, while still being able to
drive the autoputer from mobile"; and the native stack is reopened (UIKit and
Swift, or Capacitor reusing Svelte, or Kotlin for Android).

Status: design for review. Nothing here is built. Mutation classes are named
per step in §7.

## 1. The idea

Texture stops being a text editor with citations and becomes the medium. A
Texture document can hold any media, as typed blocks, and every document has
an audio rendition. Three things follow:

- **Self-development is supervised in Texture.** The candidate's screenshots
  and capsule video sit in the supervision document next to the approve
  button.
- **Most media apps become embed renderers.** Image, video, audio, PDF,
  slides and podcast are views of an embed, not desktop apps of their own.
- **Autoradio is Texture plus its audio layer on a phone.** It reads, plays
  and narrates Texture documents, takes voice and text into the prompt bar,
  and carries owner acts (approve, reject, roll back), so a person can run
  their computer from mobile without the desktop.

## 2. What exists today

From the code, verified 2026-10-20 (file references in the research notes the
two survey agents returned; the main ones are repeated here).

- **The document schema is closed and has no media block.**
  `internal/texturedoc/schema.go` (`choir.texture_doc.v1`) allows paragraph,
  heading, lists, blockquote, code block and rule; inlines are text, hard break
  and `source_ref`. `validateBlock` rejects anything else. Revisions are
  immutable and hash-chained (`internal/types/texture.go:107-185`).
- **Media already renders, but only inside citations.**
  `sourceEntityMedia` (`frontend/src/lib/texture-source-renderer.ts:284-323`)
  renders YouTube iframes, `<video>`, `<img>`, `<audio>` and PDF objects in
  citation popovers. Published stories never call it. The server turns YouTube
  and image URLs into source entities with `ExpandedMode: media_player`
  (`internal/textureowner/texture_media_sources.go`).
- **`{{embed <handle>}}` transcludes capture text**, rendered as a figure in
  published stories (`internal/texturewrite/inline.go:12`).
- **The source contract has unused selectors** for `timestamp_range` and
  `image_region` (`internal/sourcecontract/selector.go:12-16`).
- **There is a content-addressed image store with no browser route.**
  `internal/imageblob` holds PNG, JPEG, GIF and WebP by sha256 and re-hashes
  on read. Media apps read files through the path-based, owner-authenticated
  `/api/files/<path>` route instead.
- **Publication pins text, not media.** `resolved_blocks` are pinned and
  verified on read; no image, audio or video is pinned.
- **There is no content security policy** outside Mail. `MailApp.svelte`
  renders message bodies in a sandboxed `srcdoc` iframe with its own CSP and
  strips active tags: the one hardened pattern in the frontend. The citation
  YouTube iframe is not sandboxed.
- **The doctrine fences schema changes.** `docs/source-external-data-publication.md`
  (around line 240-300) says not to add display modes or recreate
  `source_embed` without a promoted schema change. This design is that change.
- **Autoradio** has a 1,563-line plan
  (`docs/archive/reports/autoradio-mobile-app-plan-2026-10-14.md`): UIKit-first Swift,
  no general web view, iOS 26 minimum, on-device speech (Parakeet) and
  speech synthesis (Kokoro), code under `mobile/`, Android later in Kotlin and
  Compose. A browser prototype sits on `prototype/autoradio-2026-10-14`,
  unmerged. There is no native code and no speech pipeline on the server.
- **Visual evidence** for self-development is half built on
  `feat/visual-evidence-slice`, now being rebased; its design already planned
  the review in a Texture row (report §5.2).

## 3. One embed block

Add one block type to the schema, `embed`, with a typed target, rather than a
block type per medium. Every renderer (web, iOS, later Android) implements one
switch over the target kind, and an unknown kind renders as a plain card with
its title and a link. That keeps the schema closed and the renderers small,
and lets a new kind ship on the web before the phone learns it.

```
embed {
  id, kind, target,          // what it shows
  caption, alt,              // text every renderer and the audio layer can use
  selector?,                 // timestamp_range | image_region | page_range
  poster?, transcript?,      // asset refs
  privacy: private | public  // derived, never chosen by the writer (§4)
}
```

| Kind | Target | Web renders | Published |
| --- | --- | --- | --- |
| `image` | asset digest | `<img>` | pinned copy |
| `video` | asset digest | `<video>` with range requests | pinned copy |
| `audio` | asset digest | `<audio>` | pinned copy |
| `external_video` | provider + id (YouTube) | sandboxed privacy-mode iframe, start/end | reference only |
| `podcast_episode` | feed item ref + enclosure URL | `<audio>` from the publisher's URL | reference only |
| `slides` | deck asset (PDF or page images) + page range | page viewer | pinned copy |
| `document` | PDF or EPUB asset + page range | reader | pinned copy |
| `calendar_event` | computer object ref | event card, live | never (snapshot only by owner act) |
| `mail_message` | computer object ref | message card; body in Mail's sandbox | never (snapshot only by owner act) |
| `capsule_evidence` | asset-store digest (`imageblob`) + operation id | screenshot or clip, with the candidate's identity | never |
| `texture` | another document or revision | transcluded card | per that document |

Rules that keep this honest:

- **Owned bytes are pinned; other people's media is referenced.** A
  document's own images, recordings and generated audio are assets with
  digests. YouTube and podcast episodes are never re-hosted: they play from
  the provider (`docs/research/autopaper-media-and-rights-2026-10-14/media-sources-and-terms.md`).
- **Live private objects stay live and private.** A calendar event or a mail
  message is a reference into the owner's computer, rendered as a card with
  its current state. It is never copied into a published document. Turning
  one into a published snapshot is an explicit owner act.
- **Evidence is never published.** `capsule_evidence` is supervision
  material for the owner.

## 4. Assets and privacy

- **Store.** Generalize `imageblob` into a per-computer media asset store:
  same content addressing and re-hash on read, with audio, video and PDF
  types added and a size cap per kind.
- **Route.** `GET /api/assets/sha256/<hex>`, owner-authenticated, with HTTP
  range requests (video and audio seek), `Content-Type` from the stored kind,
  `X-Content-Type-Options: nosniff`, and immutable caching. This is the route
  the visual-evidence design needs too; build it once.
- **Publish.** Publication refuses a document that references a private asset
  or a private live object, the same way public transclusions must already
  resolve to public-safe artifacts (`docs/source-external-data-publication.md:330`).
  Public assets are copied into the platform, pinned beside
  `resolved_blocks`, and served from a public digest route.
- **Security.**
  - A CSP for the app, with `frame-src` limited to the privacy-mode YouTube
    domain and `media-src` to self plus https.
  - Every third-party player in a sandboxed iframe.
  - The citation iframe gets the same sandbox.
  - Mail bodies keep Mail's pattern.
  - No embed runs script in the document's origin.

### Corrections from the visual evidence build (2026-10-20)

The visual evidence slice (`docs/design/visual-evidence-slice-2026-10-15.md`
section 10, draft PR 76) built the first half of T1's asset path and found
five places where this design was loose.

- **Evidence digests are asset-store digests.** A capture is stored in the
  computer's `imageblob` store at Freeze and read through
  `/api/assets/sha256/<hex>`; the embed names that digest, not an event
  artifact.
- **The owner route is not the public route.** The asset route serves any
  digest in the owner's store to an owner-authenticated caller (a
  `read:runtime` key scope included). T2's public route is a separate route
  that serves only digests a publication pinned.
- **The store has no retention yet.** A Texture revision that embeds an asset
  must pin it, so a later reclamation cannot leave the document pointing at
  nothing. Retention lands with T1, before any sweep touches assets.
- **Only the kind typing exists.** Audio and video kinds are typed and
  refused; the per-kind size caps are still to come, and the store enforces
  only the 20 MiB image limit.
- **Texture writes the words.** A capture carries no description. The
  embed's `alt` and `caption` are written by Texture (the sole canonical
  writer), never by the capture job.

## 5. The audio layer

Every Texture revision has an audio rendition: a timeline of segments, each
tied to a block id, so playback can highlight where it is and a listener can
jump by heading.

- Text blocks are narrated.
- `audio` and `podcast_episode` embeds play their own audio for the selected
  range.
- `video` and `external_video` play their audio track when allowed, or a
  short spoken caption.
- Images and slides read their alt text and caption.
- Calendar and mail cards read a one-line summary.
- `capsule_evidence` reads the candidate's summary and says there is a video
  to watch.

The timeline is data, not audio: a list of segments with block ids, kinds,
text or source and durations. Each client turns it into sound. The open
question is where speech synthesis runs (§8, question 4). The Autoradio plan
already chose on-device synthesis for the phone. The web needs either the
browser's voices or a server pass, and a server pass costs money and moves
private text off the computer.

## 6. Autoradio and the native stack

### What Autoradio is under this design

A phone app that:

- reads and plays Texture documents with the audio layer, in the
  background and on the lock screen;
- renders embeds natively;
- takes voice or text into the prompt bar;
- performs owner acts with Face ID;
- shows the self-development supervision document, including the capsule
  video.

It does not carry the desktop. The plan's scope list (Autopaper, Texture, PDF,
EPUB, image, audio, video, slides, supervision) becomes "Texture and its
embeds", which is smaller.

### The three options

**A. Native Swift on iOS now, Kotlin and Compose on Android later (the
current plan).**

- Background audio, lock-screen and Now Playing controls, CarPlay, Siri and
  App Intents, Live Activities, widgets, on-device speech, and passkeys bound
  to the associated domain are all native, and several are Swift-only.
- Those surfaces are most of Autoradio's value.
- The cost is two clients.

**B. Capacitor, reusing the Svelte frontend.**

- Fastest to a first build on both platforms.
- But the audio layer, CarPlay, App Intents, Live Activities and passkeys
  would each be a native plugin, so the valuable part is native anyway.
- The part reused is the weakest one on a phone. The Texture editor is a
  2,654-line `contenteditable` surface built for a desktop window, and
  `contenteditable` in WKWebView has open keyboard bugs on iOS 26.
- App Review guideline 4.2 is a real risk for an app that is mostly a web
  view.
- On 2026-10-14 you ruled out shipping the web desktop as a phone app ("it's
  just not cool. It should be a purpose-built thing"). Capacitor around the
  Svelte frontend is close to that.

**C. A native shell with a web-view island for the editor.**

- Possible later for rich editing.
- It adds a bridge to maintain and does not help the reading and listening
  path.

### Recommendation

Stay native: Swift with a UIKit core on iOS now, and Kotlin with Compose on
Android after TestFlight. Make the reuse happen one level down, where it is
cheap and durable:

1. **The schema is the shared contract.**
   - `choir.texture_doc` with the `embed` block is closed and typed.
   - A renderer for it is a switch over about a dozen block kinds and a
     dozen embed kinds: small in Swift, small in Kotlin, already done in
     Svelte.
2. **The audio timeline is shared data.** All three clients play the same
   timeline format.
3. **Golden documents are shared tests.**
   - A set of fixture documents covering every block and embed kind.
   - Each renderer must render them, and each audio player must play them,
     with the same block order and timings.
   - This is how three renderers stay in step without sharing code.
4. **The server does the thinking.** Agents, search, summaries and
   publishing stay on the computer, so the clients are renderers, players
   and input.

This matches your instinct that Kotlin is not hard. With a closed schema, the
per-platform code is renderers and media plumbing, which agents write well in
either language.

What would change my mind: if Android users matter before the iOS app proves
the audio experience. Then a Capacitor build of the reader alone, with no
editor, would be a stopgap. Revisit at the plan's M5, as it already says.

Editing on the phone in version one is voice and text into the prompt bar,
comments, and approvals. Full block editing on the phone comes later, native.

## 7. Effect on the app rewrites and the mission order

- **Media apps become renderers.** Image, video, audio, PDF, EPUB, slides,
  podcast and the content viewer stop being rewrite candidates as desktop
  apps. Each becomes an embed renderer plus at most a thin full-screen viewer
  that opens one embed. That removes about half of the fourteen rewrite
  candidates.
- **Calendar stays an app** and gains an embeddable event card.
- **The rest are rewritten as packages after S7-0:** files, browser, compute
  monitor, pulse and the super console.

Proposed steps. Each lands through the Landing Loop with problem
documentation first.

| Step | What | Class |
| --- | --- | --- |
| T1 | `texture_doc` gains `embed` (image, video, audio, external video, capsule evidence); the asset store and range route; the web renderer; the CSP and sandboxes | red (Texture canonical schema), orange (route, renderer) |
| T2 | Publication pins public assets and refuses private ones; public asset route | red (publication) |
| T3 | Slides, document, podcast episode, calendar and mail cards | orange |
| T4 | The audio timeline, played on the web | orange |
| A0 | Autoradio iOS: reader, embeds, audio layer, prompt bar, owner acts; golden documents shared with the web | new client |

Order against your ruling (S6, then S7-0, then rewrites):

- **T1 runs beside S6.** Visual evidence approval needs the `capsule_evidence`
  embed and the asset route, and both are small.
- **T2 and T3 run beside S7-0.**
- **The op-ed section keeps its place before Autoradio**, as you set on the
  night of October nineteenth.
- **A0 starts once T1 and T4 exist,** so the phone has something real to
  render and play.

## 8. Questions for you

Each has a recommendation.

1. One `embed` block with typed kinds, rather than a block per medium?
   Recommend yes.
2. Third-party media (YouTube, podcasts) is referenced, never re-hosted, and
   private live objects (calendar, mail) never enter published documents
   except by an explicit owner act? Recommend yes.
3. Should the media apps become embed renderers with a thin viewer, taking
   them off the rewrite list? Recommend yes.
4. Where does speech synthesis run for the audio layer? Recommend on device
   (the phone's Kokoro, the browser's voices on the web) first, with a server
   pass only for published Autopaper stories, where the text is already
   public and one rendition serves every reader.
5. Stay native: Swift on iOS now, Kotlin on Android later, with the schema,
   the audio timeline and golden documents as the shared layer? Recommend
   yes.
6. Version one of Autoradio reads, plays, prompts and approves, with no
   block editing on the phone? Recommend yes.
7. T1 runs beside S6, ahead of S7-0? Recommend yes, because approval with
   visible evidence depends on it.
