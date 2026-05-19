# MissionGradient: Media Content Immersion v0

**Status:** ready for execution
**Date:** 2026-05-19
**Operator:** Codex supervising staging, product-path Playwright, git, CI, deploy, and owner review
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)
**Starting deployed baseline:** `32b79ccc42b32a07ed23e5f8edb5c5d86841559e`

## One-Line Goal String

```text
/goal Run docs/mission-media-content-immersion-v0.md as a Codex-operated MissionGradient mission: make Choir's Image, PDF, EPUB, Audio, Video, and Podcast apps content-first inside the floating desktop window. Preserve the separate app code paths, but fix the UX so the media or reader content occupies the full window by default; metadata, provenance/source/hash fields, page/chapter/search controls, zoom/rotation controls, and secondary playback controls must be collapsed into user-opened overlays/drawers by default and must return the content to full-window occupancy when closed. Keep mobile as the same powerful floating-window desktop, not a phone-mode rewrite. Do not reintroduce shared media CSS, generic MediaFileApp/ContentViewer behavior, fake placeholders, local-only proof, or visible debug/source/provenance chrome in ordinary use. Land through git/CI/deploy, verify staging identity, and use Playwright screenshots plus DOM metrics on desktop and 390x844 mobile to prove every media app's primary content stage fills the app window before and after opening/closing controls, with rollback refs, residual risks, and the next realism axis.
```

## Mission Frame

The recent media split repaired the app-boundary problem: Image, Audio, Video,
PDF, EPUB, and Podcast now have separate code paths and no longer inherit a
single `MediaFileApp`/`ContentViewer` stylesheet. That was necessary but not
sufficient. The screenshots show the remaining UX failure clearly: toolbars,
metadata accordions, source/provenance/hash affordances, and reader controls
still compete with the media itself.

This mission is narrower than the full UX bag sweep. It is the content
immersion pass: when a user opens media, the thing they opened should own the
window. Controls and metadata should be available, but should not be the default
visual structure.

## Current Surface Review

- `ImageApp.svelte`: separate app path exists. The image stage is real, but the
  rotation/fit/zoom toolbar is expanded by default at the top and the Info
  drawer is visible at the bottom. On mobile this makes a photo feel framed by
  controls instead of opened as an image.
- `PdfApp.svelte`: PDF.js rendering, page count, zoom, and search exist. The
  toolbar is in normal flow above the page and metadata sits below it, so the
  PDF page does not own the window. A full-screen reader should show the page
  first, then reveal controls on demand.
- `EpubApp.svelte`: EPUB archive parsing, chapter rendering, font/width/search,
  and progress exist. The controls are permanently in the flow, and long title
  toasts/metadata can obscure the reader. The chapter text should own the
  window, with reader controls as an overlay/drawer.
- `AudioApp.svelte`: app-specific player exists. Audio has no visual document,
  so the player can be the primary content, but secondary metadata and
  nonessential control chrome should still be collapsed. The default should feel
  like a focused playback surface, not a settings panel.
- `VideoApp.svelte`: the video/theater stage is close to the desired shape.
  Embedded status, custom transport, and metadata should remain hidden or
  auto-revealed only when the user asks. Native video controls may appear on
  hover/tap; persistent app chrome should not shrink the theater.
- `PodcastApp.svelte`: treat as a regression/reference app. Podcast is allowed
  to have a library/detail/player product structure, but within an episode feed
  or playback view the episode list/player content should dominate and
  provenance/source/debug details should stay hidden unless explicitly opened.

## Real Artifact

The artifact is the deployed media app UX geometry:

```text
Files / prompt / launcher
-> typed app routing
-> Image / PDF / EPUB / Audio / Video / Podcast app windows
-> primary content stage fills the window
-> controls/details are collapsed overlays or drawers by default
-> Playwright-visible desktop and mobile proof
```

The artifact is not:

- a renamed generic media viewer;
- a shared CSS sheet that forces every app into the same compromises;
- visible source hashes, provenance, metadata, or toolbars as the default state;
- a screenshot-only claim without DOM measurements;
- mobile simplification that abandons the web desktop.

## Invariants

- Each media surface stays a separate app with app-specific layout and state.
- Shared abstractions may be introduced only after at least two apps have a
  good app-specific implementation and the abstraction is clearly subordinate.
- Primary content must occupy the window by default:
  - Image: image canvas.
  - PDF: rendered page area.
  - EPUB: scrollable reader text.
  - Video: video theater or embedded frame.
  - Audio: focused player surface.
  - Podcast: library/feed/player content, not provenance/source chrome.
- Metadata, source URLs, filenames, content hashes, provenance, and debug
  evidence are hidden by default in closed `details` or equivalent drawers.
- Toolbars and secondary controls are hidden by default and user-opened. They
  must not reserve layout height while closed.
- Opening controls must be reversible. Closing controls restores the full-window
  content geometry.
- Controls remain keyboard/touch accessible and test-addressable.
- Mobile remains a floating-window desktop. Do not replace it with a single
  phone app view.
- Platform behavior claims require deployed staging proof after git/CI/deploy.

## Value Criterion

Minimize:

```text
visible default chrome
+ content area lost to controls
+ source/provenance/debug leakage into ordinary use
+ app-specific UX flattened into shared CSS
+ mobile occlusion
+ unreachable controls after hiding chrome
+ unmeasured visual claims
+ regressions in Files/prompt routing
```

subject to the invariants above.

The mission moves uphill when a normal user opens media and sees the media,
book, document, or playback surface first, with controls available only as an
intentional reveal.

## Homotopy Axes

- **Control visibility:** persistent toolbar -> closed overlay drawer -> richer
  context-sensitive controls.
- **Content occupancy:** partial stage -> dominant stage -> full app content
  geometry with only transient overlays.
- **App specificity:** separate code paths -> app-specific design quality ->
  carefully extracted helpers only where they preserve each app's taste.
- **Proof realism:** local build -> local Playwright geometry -> deployed
  staging Playwright screenshots and DOM metrics.

## Implementation Direction

Use app-specific designs first.

- Image: default to image-only canvas. Add a compact "Controls" affordance,
  likely a top-corner closed `details`, containing fit/original/zoom/rotate
  actions. Keep Info separately closed.
- PDF: make the stage the first layout row and full height. Move page, zoom,
  and search controls into a closed reader-controls drawer or overlay. Keep
  search results in the same drawer unless opened. Metadata stays closed.
- EPUB: make the scroll reader fill the window. Move font size, width, chapter,
  progress, and search into a closed reader-controls drawer. Reader content
  should not jump permanently when controls open/close.
- Video: keep theater full-window. Hide custom transport and metadata by
  default; reveal controls as an overlay. Embedded players should not show a
  permanent "embedded controls active" badge over the video.
- Audio: treat the focused player as primary content. Hide metadata and
  secondary details. Preserve play, seek, skip, speed, and local position.
- Podcast: preserve current working behavior while removing any visible
  source/provenance/debug chrome from ordinary views and measuring that feed or
  player content is not compressed by chrome.

## Playwright Proof Requirements

Add or update focused Playwright coverage. The final proof must run against
`https://draft.choir-ip.com` after deploy.

Required viewports:

- desktop, for example `1280x900`;
- mobile web desktop, `390x844`.

Required apps:

- Image;
- PDF;
- EPUB;
- Audio;
- Video;
- Podcast regression/reference.

For each app, collect screenshots and DOM metrics:

```text
appBox = app root bounding box
stageBox = primary content stage bounding box
stageArea / appArea >= target threshold
closed controls are not visibly occupying flow height
details/drawers are closed by default
open controls -> required controls visible and usable
close controls -> stage area returns to default occupancy
no document horizontal overflow
```

Target thresholds:

- Image/PDF/EPUB/Video: primary stage should occupy at least 85% of app content
  area when controls are closed. If the floating-window title bar is included
  in the measured window box, subtract it or measure from the app root.
- Audio: primary player surface should occupy at least 80% of app content area,
  with metadata closed.
- Podcast: library/feed/player primary region should occupy at least 80% of app
  content area, with source/provenance/debug details closed.

If a threshold is not appropriate for one app, record the reason and replace it
with a stricter app-specific metric that protects the same invariant.

Proof commands should include:

```text
pnpm build
GO_CHOIR_CONTENT_BASE_URL=https://draft.choir-ip.com GO_CHOIR_RUN_CONTENT_APPS=1 pnpm exec playwright test tests/content-apps-routing.spec.js --workers=1 --reporter=line
<new focused media-immersion Playwright proof> --workers=1 --reporter=line
```

The mission is not complete until the operator has personally inspected the
Playwright screenshots or trace artifacts and verified that the media content
visibly owns the window.

## Dense Feedback

- Use local build and local/browser Playwright for fast geometry iteration.
- Before landing, run focused tests for app routing plus the new immersion
  metrics.
- After landing, push `main`, monitor CI, monitor staging deploy, verify
  `/health` reports the pushed SHA, and rerun the deployed Playwright proof.
- When a visual claim is made, attach screenshot path, viewport, app, and DOM
  ratio metrics.

## Forbidden Shortcuts

- Do not reintroduce `media-app.css`, `MediaFileApp`, or a generic ContentViewer
  path for these app views.
- Do not hide controls by deleting functionality.
- Do not make metadata inaccessible; hide it behind deliberate disclosure.
- Do not claim mobile success from desktop screenshots.
- Do not claim success from local-only proof for deployed platform behavior.
- Do not use fake placeholder panels, fake readers, fake media controls, or
  debug-only routes.
- Do not let tests pass by weakening the content-occupancy invariant.

## Rollback

The immediate rollback target is the deployed baseline before this mission:

```text
32b79ccc42b32a07ed23e5f8edb5c5d86841559e
```

If the mission lands multiple commits, the final report must name the exact
rollback command or revert range, plus any dependency or generated-artifact
changes.

## Stopping Condition

Stop only when one of these is true:

- Full-chain success: the code is committed, pushed, CI/deploy are green,
  staging reports the pushed SHA, and deployed Playwright screenshots plus DOM
  metrics prove content-first default geometry for Image, PDF, EPUB, Audio,
  Video, and Podcast.
- Precise blocker: after root-cause probes and at least one cognitive
  search-space transform, a named invariant-level blocker remains. The final
  report must include the failing app, screenshot/trace path, DOM metrics,
  implicated files, rollback refs, and the next executable probe.

## Final Report Requirements

The final report must include:

- pushed commit SHA(s);
- CI run URL and status;
- staging `/health` build identity;
- deployed Playwright command(s) and result(s);
- screenshot/trace artifact paths;
- DOM metric table by app and viewport;
- controls/details default-open/default-closed evidence;
- rollback refs;
- residual risks;
- next realism axis.
