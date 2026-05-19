# Platform OS And App State

**Status:** canonical platform-level state ledger
**Last updated:** 2026-05-19
**Baseline checked:** desktop restore recovery baseline
`e61434e88708fdbc6df4c8fbe27e2f64f869d7ca`

This document records the current common state of the Choir automatic computer:
the platform substrate, desktop shell, app catalog, app boundaries, known proof,
and known gaps. Keep it updated when platform-level OS, shell, routing, app, VM
lifecycle, promotion, or public/default computer behavior changes.

User computers are allowed to diverge. For now, this ledger describes the common
platform/default state that new and ordinary user computers inherit or project
through. Later, each divergent computer should have its own product-visible state
ledger derived from the same schema: app catalog, source/build refs, package
adoptions, local app state, artifact digests, verifier evidence, and rollback
refs.

## Update Discipline

Update this document when a mission changes any of the following:

- the app catalog, app identity, launcher entries, or Files/prompt routing;
- app-specific capabilities, controls, persisted state, or proof status;
- desktop shell behavior: windows, app switching, prompt bar, Shelf/Desk,
  auth-on-mutation, mobile geometry, or logged-out read/explore;
- VM/computer lifecycle behavior that affects user-visible boot, warmness,
  hibernation, recovery, or priority classes;
- candidate, promotion, package, run-acceptance, Trace, or VText evidence
  surfaces;
- the line between platform substrate deploys and user-computer promotion.

State claims should name evidence when possible: commit SHA, staging `/health`
identity, tests, screenshots, Trace/run-acceptance ids, or a precise blocker.
Do not rewrite this file as a wish list. Put aspirational work in mission docs
and keep this file as the state ledger.

## State Model

The common state has three layers:

```text
platform substrate
  -> platform/default computer state
      -> user active computers
          -> candidate/background computers
```

- **Platform substrate** is shared deploy machinery: GitHub `origin/main`, CI,
  NixOS deploy, Go proxy/runtime services, Svelte frontend, `vmctl`,
  Firecracker/NixOS VM support, host auth/session state, and platform APIs.
- **Platform/default computer state** is the official app/desktop experience
  currently projected by `draft.choir-ip.com` and used as the conceptual base
  for new user computers.
- **User active computers** are persistent private machine-worlds with their own
  files, embedded Dolt state, desktop state, app state, prompts, and eventual
  source/build divergence.
- **Candidate/background computers** are speculative mutation contexts. They may
  break, build, test, and produce deltas; they should not mutate active
  foreground state before verification and promotion.

Platform behavior changes still land through:

```text
commit -> push origin main -> CI -> deploy -> staging identity -> product proof
```

User-computer app/source changes should eventually land through personal
promotion rather than global platform deploy. Until that substrate is complete,
platform docs record the common baseline and the desired divergence semantics.

## Substrate State

- **Acceptance environment:** `https://draft.choir-ip.com`.
- **Source of truth:** GitHub `origin/main` for tracked platform files.
- **Deploy model:** GitHub Actions deploys tracked platform changes to Node B.
  Documentation-only commits intentionally skip CI/CD.
- **Runtime shape:** Go proxy/runtime services, Svelte frontend, `vmctl`, NixOS
  host services, Firecracker-backed user computers, and embedded per-user Dolt
  for runtime/control product state.
- **Public platform state:** host/platform services own accounts, routing,
  lifecycle, publication/public artifact records, and aggregate health. Browsers
  do not talk to Dolt directly.
- **Computer lifecycle:** active primary computers are protected ahead of
  candidate/worker computers. Ordinary primaries stay warm while capacity
  allows; configured always-on primaries have a protected lane. Candidate and
  worker computers hibernate first. See [vm-priority-policy.md](vm-priority-policy.md).

## Desktop Shell State

The shell is a web desktop with floating windows, freely placed desktop icons,
Shelf/Desk menu, prompt bar, live status, and persisted desktop state for
signed-in users. Mobile is intended to use the same overlapping desktop model
as desktop, with tighter geometry and better overview controls rather than a
separate phone-mode navigation stack.

Current capabilities:

- signed-out users can enter a public desktop projection without hydrating a
  private computer;
- mutation intent should ask for auth at the boundary and resume after auth;
- signed-in users bootstrap their active computer before private desktop state
  loads;
- floating windows support focus, minimize, restore, maximize, move, resize,
  compact non-fullscreen default geometry on mobile, and visible stack depth;
- the Shelf exposes app switching, the Desk menu, and app launching;
- Desktop Overview is the intended shell mode for seeing and managing all open
  windows at once;
- desktop state persists windows and icon positions for authenticated sessions;
- restore recovery can avoid hydrating every saved heavy app at once, and heavy
  restored background apps may stay suspended until raised;
- Compute Monitor is the product surface for the signed-in user's current
  computer, background candidate computers, app restore weight, and bounded
  recovery actions. It intentionally does not expose host/platform pressure,
  global vmctl inventory, deployed build metadata, or raw VM handles.

Known gaps:

- boot/wake still needs trend history and deeper event trails for long
  recovery sequences;
- app switching and window raise/restore need continued long-session mobile
  proof across more real user accounts;
- the Shelf/prompt bar must keep shrinking when input is empty and only grow
  for real prompt content;
- Desk menu, Shelf placement, and desktop icon surfaces are not yet a unified,
  tasteful, configurable desktop environment;
- Desktop Overview v0 is card/spatial-state based, not live thumbnail based.

## App Catalog

| App | Current state | Known gaps / next realism axis |
| --- | --- | --- |
| **Files** | First-class file browser with navigation, upload, text-to-VText open, and known media routing to Image/Audio/Video/PDF/EPUB apps. Unknown binaries still download. | Keep proving that PDF/EPUB/media open in apps instead of downloading. Add richer previews only through app boundaries. |
| **VText** | Primary appagent and versioned document editor. Owns canonical document versions and prompt-created writing surfaces. | Continue flicker/focus/coexistence hardening, especially while Trace or live updates are open. Improve document/read surfaces without mixing worker patches directly into canonical text. |
| **Trace** | Evidence app for trajectories, runs, summaries, timelines, inspectors, search/provider stats, and run-acceptance surfaces. | Eliminate confusing multi-scroll layouts on mobile except bounded code/payload blocks. Make run/candidate/promotion evidence easier to drill into on desktop and mobile. |
| **Web Lens / Browser** | Browser-style URL input with backend Web Lens snapshots when authenticated/configured and iframe fallback for guest/external pages. | Backend control/screenshot support remains a distinct substrate frontier. Browser should remain an app, not a bypass around product APIs. |
| **Terminal** | Floating terminal backed by `ghostty-web` and `/api/terminal/ws`, with independent PTY sessions per window. | Keep guarded as a signed-in/mutation surface. Do not treat terminal proof as product proof for app/VM/promotion behavior. |
| **Settings** | Account, runtime health, theme presets/editing, and promotion queue controls. | Theme system needs taste/design hardening. Promotion/candidate surfaces should become contextual rather than manual/debug-first. |
| **Compute Monitor** | First-class app for user-computer health and recovery. It uses authenticated product APIs to show only the current user's current computer, background candidate computers, warmness/protection, current runtime health, app/window restore weight, safe desktop-state recovery actions, and disabled unsafe controls. It is available from the launcher and Settings. | Add event-backed trend history, app-owned process/resource accounting, candidate discard/hibernate actions, conductor recovery intents, and stronger long-session regression proof. |
| **Candidate Desktop** | Preview surface for candidate VM desktops. | Candidate/promotion cards should appear from current run/Trace context instead of requiring manual IDs. |
| **Podcast** | Working app-grade v0. It has library/search/recommendations, hidden advanced RSS import, feed detail, scrollable episode list, full player controls, speed/seek, and local playback-position persistence. | Treat as a regression/reference app, not the center of the next media mission. Continue improving subscription durability, played/unplayed state, conductor actions, and VText radio continuity later. |
| **Image** | First-class app with source resolution, title, fit/original, zoom controls, rotate left/right, reset, and image rendering. | Add pan/drag, touch/pinch behavior, folder gallery navigation, richer metadata, and persisted viewer state. |
| **Audio** | First-class app with play/pause, 15s back, 30s forward, scrubber, speed, current/duration, native audio fallback, and local playback-position persistence. | Add queue/playlist from Files, metadata, Media Session integration, transcript/VText hook, and keyboard controls. |
| **Video** | First-class app for native video and YouTube embeds. Native video has custom/native controls, speed/seek, and local playback-position persistence. | Add fullscreen/theater fit, captions/subtitles, transcript/VText hook, playlist/folder navigation, and consistent YouTube/native control surfaces. |
| **PDF** | Real reader path using PDF.js: browser-fetchable PDFs render to canvas pages with actual page count, page navigation, zoom/fit width/fit page, and text search. Files/prompt routes can open the PDF app. | Add thumbnails/outline, annotations, richer text selection, and server-side/import fallback for CORS-blocked remote PDFs. |
| **EPUB** | Real reader path using EPUB archive parsing: browser-fetchable EPUBs parse container/package/spine, render chapters as safe text blocks, expose chapter selection, font/width/progress controls, search, and local reading-position persistence. Extracted text still renders as a reader source. | Add richer XHTML formatting, EPUB nav/TOC semantics, bookmarks, image assets, server-side extraction, and VText/transclusion handoff. |
| **ContentViewer** | Legacy generic content surface still exists in code but is not the place to add media behavior. | Do not put new app work here. Retain only as fallback/dispatcher/inspector until it can be safely retired or narrowed. |

## App Boundary Rules

- A user-facing capability should have one obvious app owner.
- Podcast, Image, Audio, Video, PDF, and EPUB are separate apps.
- Shared code may exist only as subordinate helpers: source URL resolution,
  MIME/extension routing, time formatting, persisted-state helpers, transport
  controls, viewport math, and safe parsing utilities.
- Do not revive a generic `MediaFileApp` or grow `ContentViewer` into an
  everything-viewer.
- Files, launcher, prompt/conductor decisions, and public routes should converge
  on the same app identity for the same artifact.
- Primary app chrome should expose the user's task first. Provenance, source
  hashes, debug ids, and raw manifests belong in secondary details or Trace,
  not at the top of ordinary app workflows.
- No fake readers, fake transclusion panels, fake media players, fake candidate
  cards, or JSON-only success records.

## Current Proof Anchors

Computer recovery and Compute Monitor proof follows the deployed desktop
restore recovery baseline:

- baseline commit: `e61434e88708fdbc6df4c8fbe27e2f64f869d7ca`;
- local pre-landing proof must include `go test ./internal/proxy`, frontend
  build, and redaction tests for `/api/compute/status` and
  `/api/compute/recovery`;
- platform-level claims require the new commit to pass CI/deploy, staging
  `/health` to report that SHA, and staging Playwright to capture desktop and
  `390x844` mobile Compute Monitor screenshots/DOM metrics.

Recent deployed platform proof for the media split and reader sweep:

- commit: `c42108fc2b322e60b4d65c815ab3f8e2aee5dfa2`;
- staging health: proxy and sandbox reported the same commit after deploy;
- local and staging Playwright covered Files opening PDF/EPUB, content app
  prompt routing, launcher/shell smoke, VText/Trace coexistence, and Podcast
  player/mobile episode scrolling;
- screenshot/DOM artifacts were captured under
  `test-results/real-media-apps-c42108f/`.
- reader/media sweep local proof added `pnpm build`, frozen pnpm install, test
  syntax/list checks, and logged-out desktop read-shell Playwright coverage;
  staging proof for the pushed reader commit must record its SHA in the final
  mission report.

Proof caveat: before the reader commit is deployed, c42108f remains the last
fully deployed platform identity. The reader sweep adds tests that prove real
PDF rendering/search and raw EPUB archive reading, but those claims become
platform-level only after CI/deploy/staging acceptance for the new commit.

## Divergence Plan

This file is common platform state today. As source-lineage and personal
promotion mature, the same shape should become per-computer state:

```text
platform-os-app-state.md
  -> platform/default computer state
      -> user computer app/source/build state
          -> candidate computer delta and verifier state
```

Each divergent computer should eventually expose:

- app catalog and app versions;
- source/build refs for runtime and UI;
- package/adoption records;
- local app state and schema versions;
- artifact digests and content refs;
- verifier results and rollback profiles;
- public projections selected by the owner.

The platform ledger remains the official common baseline and the default fork
source for new ordinary user computers until product records supersede this
Markdown document.

## Near-Term Gaps

The highest-gradient UX gaps are:

1. PDF needs thumbnails/outline, selection, annotation, and CORS/import fallback.
2. EPUB needs richer formatting, assets, bookmarks, and VText handoff.
3. Image needs pan/touch/gallery/persisted viewer state.
4. Audio and Video need queues, metadata, Media Session, transcripts, and richer
   Files context.
5. Trace must stay readable as the evidence surface for long runs.
6. VText must remain stable when Trace/live updates coexist.
7. Shelf/Desk/Desktop Overview behavior needs richer mobile desktop proof,
   live thumbnails, and configurable Shelf placement.
8. Candidate/promotion surfaces should become contextual product surfaces.
