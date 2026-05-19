# MissionGradient: Real Readers And Media Apps UX Sweep v0

**Status:** ready for overnight execution
**Date:** 2026-05-19
**Operator:** Codex supervising staging, product-path Playwright, Choir-in-Choir workers where healthy, git, CI, deploy, Trace, VText, and owner review
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)

## One-Line Goal String

```text
/goal Run docs/mission-real-media-apps-ux-sweep-v0.md as a Codex-operated MissionGradient mission: continue from deployed platform state c42108f and make Choir's non-Podcast media/readers and desktop shell app-grade. Treat Podcast as a working regression/reference app, not the mission center. Build real first-class PDF and EPUB reader paths first, then improve Image, Audio, and Video with app-specific state, controls, Files routing, launcher entries, mobile floating-desktop behavior, and product-path proof. Keep shared helpers small and subordinate; do not revive a generic MediaFileApp or stuff media back into ContentViewer. Continue the broader UX sweep where it blocks app usability: Trace single-scroll/readability, VText coexistence/flicker, prompt/bottom-bar/app switching, contextual candidate/promotion surfaces, logged-out read/explore with auth only at mutation, and VM wake/status UX. Prefer Choir-in-Choir worker/candidate dispatch when healthy; if substrate blocks progress, root-cause and repair it directly through git/CI/deploy. Finish with updated platform OS/app state docs, staging identity, screenshots, DOM metrics, VText/Trace/run-acceptance evidence, rollback refs, residual risks, and the next realism axis.
```

## Mission Frame

The platform now has the right app boundary direction: Podcast has become a real
app, Files can route known media to separate apps, and the launcher/start bar can
surface app entries. The next mission should not spend its energy rebuilding
Podcast. Podcast is the regression reference for what "app-grade" starts to
mean: ordinary controls, library/detail state, mobile scrollability, and durable
playback position.

The mission is to make the rest of the automatic-computer media surface catch
up, while preserving the web desktop model on mobile. The highest-value gaps are
PDF and EPUB. Today they have first-class app boundaries, but PDF is mostly an
embedded browser object and EPUB blocks raw archives unless text has already
been extracted. Those are app-boundary placeholders, not real reader apps.

## Real Artifact

The artifact is the deployed platform/default automatic-computer app substrate:

```text
Files / prompt / launcher
-> typed app routing
-> PDF, EPUB, Image, Audio, Video, Podcast apps
-> floating-window desktop shell on desktop and mobile
-> Trace/VText/candidate evidence surfaces
-> staging proof and platform state ledger
```

The artifact is not:

- a generic media viewer with app names painted over it;
- a resurrected `ContentViewer` island;
- a phone-mode UI that abandons floating windows on mobile;
- local-only screenshots;
- a checklist that passes while PDF/EPUB are still not readers.

## Starting Belief State

Known state from the current platform ledger:

- Staging is deployed at `c42108fc2b322e60b4d65c815ab3f8e2aee5dfa2`.
- Podcast is a working app-grade v0 and should be protected by regression tests.
- Files opens known PDF/EPUB/image/audio/video file types into dedicated apps;
  unknown binaries still download.
- `ImageApp`, `AudioApp`, and `VideoApp` are first-class apps with basic
  controls, but they need app-grade state and richer ordinary controls.
- `PdfApp` is only a first-class opener with toolbar/object embedding. It needs
  a real reader path.
- `EpubApp` renders extracted text but cannot yet parse raw EPUB archives. It
  needs archive parsing, chapters/spine/TOC, search, and persisted reading state.
- `ContentViewer` remains as legacy code pressure and must not receive new media
  behavior.
- Trace and VText have tests, but mobile evidence readability and coexistence
  remain recurring UX risks.
- Shell mobile behavior is improving but app switching, prompt/bottom bar
  growth/shrink, and boot/wake status still need product-path proof.

Highest-impact uncertainty:

- Whether the next failing behavior is app-local reader implementation, shared
  shell geometry, Files/routing, or auth/bootstrap. Probe with Playwright before
  mutating broad shell code.

## Invariants

- Mobile remains the same web desktop, with floating windows and app switching.
  Add touch affordances; do not gimp it into single-pane phone navigation.
- Podcast remains a regression reference. Do not rewrite it unless a regression
  or shared helper boundary requires a narrow fix.
- PDF and EPUB must become real apps continuously deformable into long-term
  readers. A simplified reader is acceptable; a fake reader is not.
- Image, Audio, Video, PDF, EPUB, and Podcast stay separate apps with
  app-specific state machines.
- Shared helpers are subordinate primitives, not a user-facing app architecture.
- Files, launcher, prompt/conductor decisions, and public routes should agree on
  app identity for the same artifact.
- Debug/provenance/source hashes must not dominate ordinary app workflows.
  Put them in details, Trace, or evidence views.
- Logged-out read/explore remains available where no private state or mutation
  is touched. Mutation asks for auth at the boundary.
- Active user computers are not the mutation playground for risky development.
  Use candidate/worker paths when healthy; platform substrate fixes land through
  git/CI/deploy.
- Platform behavior claims require deployed staging proof.

## Value Criterion

Minimize:

```text
reader fakery
+ generic-viewer leakage
+ Files/download surprises
+ launcher discoverability gaps
+ mobile shell friction
+ media-control incompleteness
+ lost playback/reading position
+ Trace evidence unreadability
+ VText focus instability
+ auth overblocking
+ candidate/promotion manual-ID friction
+ undocumented platform state drift
+ future cleanup debt
```

subject to the invariants above.

The mission moves uphill when a normal user can open Files or the launcher,
open a PDF/EPUB/image/audio/video artifact, use the expected controls in a
mobile floating window, switch to Trace/VText without layout collapse, and find
honest evidence of what changed.

## Priority Surfaces

### P0: PDF As A Real Reader

Implement the smallest real PDF reader path:

- render PDF pages in-app rather than relying only on browser object fallback;
- show actual page count and page navigation;
- support zoom, fit width, fit page, and scrollable page view;
- keep controls reachable on mobile;
- add search/text extraction if feasible in the same pass, or a precise
  blocker with the extraction path;
- preserve source/file routing from Files and prompt decisions;
- prove with an actual PDF fixture and deployed mobile screenshot/DOM metrics.

### P0: EPUB As A Real Reader

Implement the smallest real EPUB archive reader path:

- parse the EPUB zip/container/package/spine;
- render chapters safely from XHTML/text;
- expose table of contents when available;
- support font size, measure/width, progress, search or precise blocker, and
  persisted reading position;
- keep extracted text/VText handoff as a future-friendly path;
- prove with an actual EPUB fixture and mobile reader metrics.

### P1: Image App

Move beyond basic image open:

- pan/drag and zoom centered on intent;
- touch/pinch where feasible;
- rotate/reset;
- fit-to-window/original-size;
- folder gallery next/previous from Files context;
- metadata/details as secondary UI, not primary chrome.

### P1: Audio App

Move toward ordinary audio-player expectations:

- playback-position persistence;
- queue/playlist from Files folder context;
- title/source metadata;
- speed controls and seek controls remain reachable;
- Media Session API and keyboard shortcuts if feasible;
- transcript/VText hook as a precise next path if not implemented.

### P1: Video App

Move toward ordinary video-player expectations:

- playback-position persistence for native media;
- fullscreen/theater/fit behavior that works in floating windows;
- captions/subtitle track detection or precise blocker;
- transcript/VText hook for YouTube/native sources;
- playlist/folder navigation where Files context exists.

### P1: Shell, Launcher, And Files

Keep app access coherent:

- launcher/start menu exposes real app entries without crowding mobile;
- Files opens known types in apps and unknown binaries as downloads;
- task buttons reliably raise/restore focused windows;
- bottom/prompt bar grows only for prompt content and shrinks when empty;
- mobile window bounds leave controls visible and avoid accidental hidden tiny
  scroll regions.

### P1: Trace And VText Evidence Surfaces

Continue hardening only where it supports the sweep:

- Trace should have one intentional app-level vertical scroll surface on mobile,
  except bounded code/payload blocks;
- Trace should make run/candidate/promotion/rollback evidence inspectable;
- VText should remain editable and stable while Trace or media apps are open;
- final proof should include Trace/VText/run-acceptance evidence when a
  Choir-in-Choir or candidate path is used.

### P2: Candidate/Promotion And VM Status UX

Do not let supporting UX block real reader progress, but fix high-impact
failures encountered in product-path proof:

- candidate/promotion surfaces should appear from context rather than requiring
  manual IDs;
- boot/wake/recovery should show honest warm/waking/recovering/degraded states;
- VM priority policy must continue protecting real primary user computers while
  candidates/workers hibernate first.

## Homotopy Axes

Increase realism continuously:

- PDF: object embed -> rendered pages -> search/text -> annotations/thumbnails;
- EPUB: extracted-text display -> raw archive parse -> TOC/search/bookmarks ->
  VText/transclusion integration;
- Image: static display -> pan/zoom/rotate -> gallery/metadata;
- Audio/Video: single source controls -> persisted state -> queues/transcripts;
- Files routing: app open -> context-preserving app open -> app package/source
  provenance;
- Shell: fits windows -> reliable focus/restore -> configurable top/bottom
  panel/overview;
- Evidence: screenshot/test -> Trace/run-acceptance -> promotion/rollback;
- State docs: common platform ledger -> per-computer product-visible state.

Avoid fake ladders. A simple PDF renderer can be real. A button that changes a
URL fragment on an opaque browser object is not enough.

## Investigation And Cognitive Reframing

Before stopping on a blocker:

1. Classify it as app-local, shared helper, Files/routing, shell geometry,
   auth/bootstrap, browser/library limitation, build/deploy, or invariant-level.
2. Run the smallest probe that distinguishes those classes.
3. If the next probe/fix is inside current authority, execute another
   receding-horizon loop instead of ending.
4. Apply route-changing transforms before declaring a hard blocker:
   - Boundary transform: is the wrong app owning the state?
   - Reader transform: is this a real reader path or a fake embed?
   - Evidence transform: what would convince a skeptical reviewer on staging?
   - Shell-vs-app transform: would fixing window geometry solve multiple apps?
   - Authority transform: is auth/active-computer bootstrap being confused with
     read-only app exploration?

## Receding-Horizon Control

Operate in short loops:

1. Baseline one concrete product failure with Playwright/screenshot/DOM metric.
2. Identify ownership.
3. Make a bounded mutation.
4. Run focused local verification.
5. Commit and push platform behavior changes.
6. Monitor CI and deploy.
7. Verify staging commit identity.
8. Run deployed product-path proof.
9. Update [platform-os-app-state.md](platform-os-app-state.md).
10. Choose the next highest-gradient app or shell failure.

## Dense Feedback Channels

Use:

- `npm run build` in `frontend`;
- focused Playwright for content app routing, Files, Podcast regression, PDF,
  EPUB, Image, Audio, Video, shell, Trace, and VText;
- Go tests for proxy/runtime/file-serving changes;
- staging `/health` build identity;
- screenshots at `390x844` and a desktop viewport;
- DOM metrics for overflow, scroll ownership, visible controls, and window
  bounds;
- Trace trajectory/run-acceptance records for Choir-in-Choir work;
- updated platform OS/app state docs.

## Evidence Ledger

For each nontrivial claim record:

```text
claim:
evidence source:
command or observation:
artifact path:
result:
uncertainty/caveat:
promotion relevance:
```

Claims that need evidence:

- PDF renders real pages with working page count/navigation/zoom on staging.
- EPUB opens a real archive and renders chapters on staging.
- Files opens PDF/EPUB/image/audio/video apps without browser downloads.
- Launcher/start menu exposes the app family on mobile.
- Image/Audio/Video improved controls remain reachable on mobile.
- Podcast regression tests still pass.
- Trace/VText coexistence remains usable enough for evidence inspection.
- Staging is on the pushed commit.
- Platform state docs were updated with the new truth.

## Forbidden Shortcuts

- Reintroducing `MediaFileApp` or using `ContentViewer` as the media app island.
- Claiming PDF success from a browser object fallback alone.
- Claiming EPUB success while raw EPUB archives still cannot be parsed.
- Putting source hashes, UUIDs, provenance accordions, or debug manifests in the
  primary user workflow.
- Browser-public internal/test routes as product proof.
- Manual success seeding.
- Local-only proof for deployed behavior.
- Disabling mobile floating windows to make tests easier.
- Mutating active user computers directly for risky app/platform development.
- Hiding provider/build/browser-library failures behind generic labels.

## Rollback Policy

- Every platform behavior-changing patch has a git rollback SHA.
- Reader/app changes should be revertible without data migration when possible.
- File-serving changes must preserve download fallback for unknown binaries.
- Persisted playback/reading state must be additive and tolerate missing old
  state.
- VM/wake/status changes must preserve candidate/worker reclaim and avoid
  leaking private user or VM identifiers.

## Learning Side-Channel

Update:

- [platform-os-app-state.md](platform-os-app-state.md) for current state;
- this mission doc if the mission target or invariants need reparameterization;
- a dated proof/report doc only for run evidence that should not become
  canonical state.

Do not bury app-boundary or reader-architecture learnings only in chat.

## Stopping Condition

Stop only when either:

1. Full-chain deployed proof exists:
   - PDF and EPUB have real reader paths or one has a precise invariant-level
     blocker after root-cause probes;
   - Image/Audio/Video have materially improved app-specific controls/state or
     precise next blockers;
   - Files and launcher route users to the app family coherently;
   - Podcast regression still passes;
   - Trace and VText remain usable enough for evidence inspection;
   - staging identity, screenshots, DOM metrics, tests, rollback refs,
     residual risks, and updated platform state docs are recorded.

2. A hard blocker remains:
   - exact failing layer named;
   - evidence captured;
   - at least one cognitive reframing changed the probe route;
   - no safe executable next probe remains inside authority;
   - rollback/no-mutation status and next safe probe are concrete.

Do not stop merely because one app opens. The objective is a coherent
automatic-computer app substrate, with PDF/EPUB as the next realism axis and
Podcast as the regression reference.
