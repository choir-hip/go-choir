# Platform OS And App State

**Status:** canonical platform-level state ledger
**Last updated:** 2026-05-22
**Baseline checked:** Live computer sync driver-lease completion
`8c0b941c36ce620d3f6cc5ed0b5fbcdb471cac65`

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
signed-in users. Desktop state is now session-aware: app instances and semantic
order are shared user-computer state, while focus and window placement are
session/viewport presentation state. Mobile is intended to use the same
overlapping desktop model as desktop, with tighter geometry and better overview
controls rather than a separate phone-mode navigation stack.

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
- desktop state persists shared app instances, semantic order, icon positions,
  and per-session window placements for authenticated sessions;
- real local input renews a driver lease; only the driving session should save
  visible focus, foreground window, and local geometry;
- passive sessions receive shared app roster/order updates without stealing
  active focus or moving local windows;
- Desktop Overview uses shared semantic app/window order for its cards and
  spatial map identity, not session-local CSS z-index;
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
- Trace-specific content live sync and long-lived real-account multi-device
  sessions still need broader product-path proof; deployed live-sync proof now
  covers media recents/progress, Files changes, VText recent updates, shared
  app roster/order, session-local focus/geometry, and `/api/ws?after_seq=`
  catch-up on top of the driver-lease state model;
- the Shelf/prompt bar must keep shrinking when input is empty and only grow
  for real prompt content;
- Desk menu, Shelf placement, and desktop icon surfaces are not yet a unified,
  tasteful, configurable desktop environment;
- Desktop Overview now has bounded live spatial previews for safe mounted
  windows by transforming the real window DOM into Overview positions, with
  honest card/suspended/redacted fallbacks for heavy, private, or unsafe
  windows. It is proven for four-window mobile/desktop multitasking and
  generated 12-window restored heavy sessions. Real long-lived user accounts
  still need taste/visual QA and richer app-owned preview descriptors.

## App Catalog

| App | Current state | Known gaps / next realism axis |
| --- | --- | --- |
| **Files** | First-class file browser with navigation, upload, text-to-VText open, known media routing to Image/Audio/Video/PDF/EPUB apps, and live file-change notifications for the current directory. Unknown binaries still download. | Keep proving that PDF/EPUB/media open in apps instead of downloading. Add richer previews only through app boundaries and broaden live file events into richer change history. |
| **VText** | Primary appagent and versioned document editor. Owns canonical document versions and prompt-created writing surfaces. | Continue flicker/focus/coexistence hardening, especially while Trace or live updates are open. Improve document/read surfaces without mixing worker patches directly into canonical text. |
| **Trace** | Evidence app for trajectories, runs, summaries, timelines, inspectors, search/provider stats, and run-acceptance surfaces. | Eliminate confusing multi-scroll layouts on mobile except bounded code/payload blocks. Make run/candidate/promotion evidence easier to drill into on desktop and mobile. |
| **Web Lens / Browser** | Browser-style URL input with backend Web Lens snapshots when authenticated/configured and iframe fallback for guest/external pages. | Backend control/screenshot support remains a distinct substrate frontier. Browser should remain an app, not a bypass around product APIs. |
| **Terminal** | Floating terminal backed by `ghostty-web` and `/api/terminal/ws`, with independent PTY sessions per window. | Keep guarded as a signed-in/mutation surface. Do not treat terminal proof as product proof for app/VM/promotion behavior. |
| **Settings** | Account, runtime health, server-backed theme presets/editing, and low-level promotion/adoption evidence. Promotion queue refresh UI has been removed in favor of live product events. | Theme system needs taste/design hardening. Settings should not be the main owner-facing install surface; Apps & Changes owns ordinary change discovery and adoption. Runtime health still needs a true push source rather than opportunistic event refreshes. |
| **Compute Monitor** | First-class app for user-computer health and recovery. It uses authenticated product APIs to show only the current user's current computer, background candidate computers, warmness/protection, current runtime health, app/window restore weight, safe desktop-state recovery actions, and disabled unsafe controls. Manual refresh UI has been removed. | Add true event-backed computer status updates, trend history, app-owned process/resource accounting, candidate discard/hibernate actions, conductor recovery intents, and stronger long-session regression proof. |
| **Apps & Changes** | Launcher-facing change store replacing Candidate Desktop. It presents reviewable changes by name, hides package/candidate refs inside technical details, can pull an AppChangePackage, create a candidate adoption for the current computer, preview that candidate through an internal frame, verify recipient builds, install/promote, rollback through product APIs, and open/create a mission VText dashboard plus owner-readable per-change VText reports. All four alternate-computer experiments now have product-openable VText reports with screenshot/video/benchmark artifact links; Liquid and Python benchmark artifacts are linked from those reports. The selected Change now exposes an honest removal/recovery model: Chiron is rollback-only, Uninstall is disabled without a verified inverse source patch, Disable is disabled without a declared feature flag/capability toggle, and empty rollback-profile JSON is not accepted as evidence. Chiron has accepted promotion-level run acceptance from product adoption evidence, and the owner-facing Chiron detail can surface that Trace/run-acceptance evidence and open Trace focused to the relevant trajectory. The portfolio review panel aggregates all four experiment Changes with report/benchmark coverage, shows Chiron's accepted promotion-level row, and run-acceptance synthesis now carries adoption rollback refs into the accepted record. | Needs actual source-level uninstall and feature-disable semantics beyond rollback-only labeling, inline media embedding in VText, continuation-level evidence, a loaded accepted-record path for non-Chiron source experiments inside the recipient computer, and owner-review visual polish. |
| **Podcast** | Working app-grade v0. It has library/search/recommendations, hidden advanced RSS import, feed detail, scrollable episode list, full player controls, speed/seek, and server-backed playback-position sync. | Treat as a regression/reference app, not the center of the next media mission. Continue improving subscription durability, played/unplayed state, conductor actions, and VText radio continuity later. |
| **Image** | First-class app with source resolution, title, fit/original, zoom controls, rotate left/right, reset, and image rendering. | Add pan/drag, touch/pinch behavior, folder gallery navigation, richer metadata, and persisted viewer state. |
| **Audio** | First-class app with play/pause, 15s back, 30s forward, scrubber, speed, current/duration, native audio fallback, server-backed recents, and server-backed playback-position sync. | Add queue/playlist from Files, metadata, Media Session integration, transcript/VText hook, and keyboard controls. |
| **Video** | First-class app for native video and YouTube embeds. Native video has custom/native controls, speed/seek, server-backed recents, and server-backed playback-position sync. | Add fullscreen/theater fit, captions/subtitles, transcript/VText hook, playlist/folder navigation, and consistent YouTube/native control surfaces. |
| **PDF** | Real reader path using PDF.js: browser-fetchable PDFs render to canvas pages with actual page count, page navigation, zoom/fit width/fit page, text search, and server-backed recents. Files/prompt routes can open the PDF app. | Add thumbnails/outline, annotations, richer text selection, and server-side/import fallback for CORS-blocked remote PDFs. |
| **EPUB** | Real reader path using EPUB archive parsing: browser-fetchable EPUBs parse container/package/spine, render chapters as safe text blocks, expose chapter selection, font/width/progress controls, search, server-backed recents, and server-backed reading-position sync. Extracted text still renders as a reader source. | Add richer XHTML formatting, EPUB nav/TOC semantics, bookmarks, image assets, server-side extraction, and VText/transclusion handoff. |
| **ContentViewer** | Legacy generic content surface still exists in code but is not the place to add media behavior. | Do not put new app work here. Retain only as fallback/dispatcher/inspector until it can be safely retired or narrowed. |

## App Boundary Rules

- A user-facing capability should have one obvious app owner.
- Podcast, Image, Audio, Video, PDF, and EPUB are separate apps.
- Shared code may exist only as subordinate helpers: source URL resolution,
  MIME/extension routing, time formatting, product-backed persisted-state helpers, transport
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

## Live State Rules

- User-computer state that must follow a user across devices belongs in
  product APIs backed by embedded Dolt, not browser storage.
- `/api/ws` is the owner/computer-scoped notification and catch-up fabric for
  product events. It is not canonical storage.
- Desktop live sync is canonicalized through Dolt-backed product APIs:
  `desktop_sessions`, `desktop_app_instances`, and
  `desktop_window_placements` split presence/driver state, shared app identity,
  semantic order, and session-local placement/focus.
- WebSocket desktop events are typed notifications that tell clients to
  refetch or merge scoped durable state. They must not become reload-the-whole
  desktop commands.
- Browser localStorage/sessionStorage must not be used for synced media
  progress, media recents, desktop/window state, theme state, Files listings,
  or candidate/promotion queues.
- Product apps should not expose manual Refresh/Reload controls to repair
  stale data. Browser page reload remains a browser app navigation command, not
  Choir state synchronization.
- Existing VText and Trace SSE streams remain valid scoped transports where
  they preserve stronger revision/trajectory catch-up semantics.

## Current Proof Anchors

Recent deployed platform proof for live multi-device computer sync:

- behavior commits:
  `484d2d6c8ee11a333b48a9a33b132c4ecb54a3b8`,
  `664df41f93f6c39a6e1409573b28e1606137b41c`,
  `93b530c3e68b29211142fe63be50bce8cb686a80`,
  `68eb6849db40e7edc703a550164271ed78bbd1a1`,
  `615fbd74d518cd7c857d407326d06231ddc68228`,
  `1b1fabde480dbb8d9bb643fc4589abb2d816fb52`, and
  `8c0b941c36ce620d3f6cc5ed0b5fbcdb471cac65`;
- CI/deploy run:
  `https://github.com/yusefmosiah/go-choir/actions/runs/26304651141`;
- staging health reported proxy and sandbox commit
  `8c0b941c36ce620d3f6cc5ed0b5fbcdb471cac65`, built at
  `20260522181828`, deployed at `2026-05-22T18:20:17Z`;
- deployed Playwright:
  `LIVE_SYNC_EVIDENCE_DIR=/Users/wiz/go-choir/test-results/live-sync-driver-lease-staging-20260522T182540Z CHOIR_AUTH_STATE=/Users/wiz/go-choir/test-results/live-sync-driver-lease-auth-20260522T182540Z/storage.json PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com BASE_URL=https://draft.choir-ip.com npx playwright test tests/live-sync-driver-lease-deployed.tmp.spec.js --project=chromium --workers=1 --timeout=420000 --reporter=list`;
- result: `1 passed`;
- proof used one desktop context at `1440x920` and one mobile context at
  `390x844` for the same fresh authenticated user computer;
- proof covered shared app instance convergence for Files, Audio, and VText;
  desktop focus stayed on Files while mobile focus stayed on VText; desktop
  Files geometry remained stable while mobile drove Audio/VText;
- proof covered media recents/progress without localStorage or manual refresh:
  the mobile Audio app showed the proof audio and `0:42 / 6:00`, while the
  mobile product API returned `current_time: 42`;
- proof covered Files and VText content updates: mobile Files showed
  `live-sync-proof-1779474356092.txt`, and mobile VText recent showed
  `Live sync VText proof 1779474356092`;
- proof covered websocket catch-up from `/api/ws?after_seq=6`, returning missed
  `media.recent.updated`, `media.progress.updated`,
  `desktop.driver_lease.updated`, `desktop.app_instances.updated`,
  `desktop.window_placement.updated`, `file.changed`, and
  `vtext.document_revision.created` events;
- proof covered Desktop Overview convergence: desktop and mobile card/map app
  ids all matched `files`, `audio`, `vtext` while local focus/z-index and
  placement remained session-specific;
- artifacts:
  `test-results/live-sync-driver-lease-staging-20260522T182540Z/metrics.json`,
  `desktop-driver-files.png`, `mobile-passive-files-synced.png`,
  `desktop-overview-order.png`, `mobile-overview-order.png`,
  `desktop-after-app-content-sync.png`, and
  `mobile-driver-vtext-content-sync.png`.

Recent deployed platform proof for Apps & Changes, VText reports, and benchmark
evidence:

- behavior commits:
  `e0a8f76954cb01a983c6d980b3e558fae45e06a0`,
  `75c80cd4b17e5403bf5f20ef835b4d42a0aea859`, and
  `a73affbc5c58121ceead49b8a8580b4247627fe6`,
  `efeb5d8fc926099ddbebf731d916f6dd83b54245`, and
  `2ea3deefa0108b9cc7307f2c7e64dbe58c3c295e`, and
  `a6767cf1436d18d2f144faad4ccb300ec8707b21`, and
  `9bb9446b55588beabb63a750f8d25e93a692e074`, and
  `22410dafff91cdc4edcddfa65ffa609c2973e928`;
- CI/deploy runs:
  `https://github.com/yusefmosiah/go-choir/actions/runs/26198364649`,
  `https://github.com/yusefmosiah/go-choir/actions/runs/26199378174`,
  `https://github.com/yusefmosiah/go-choir/actions/runs/26199796372`, and
  `https://github.com/yusefmosiah/go-choir/actions/runs/26200571636`, and
  `https://github.com/yusefmosiah/go-choir/actions/runs/26202379885`, and
  `https://github.com/yusefmosiah/go-choir/actions/runs/26204257440`;
- staging health reported proxy and sandbox commit
  `22410dafff91cdc4edcddfa65ffa609c2973e928`;
- deployed product proof artifacts:
  `test-results/apps-changes-vtext-report-staging-2026-05-21T00-50-49-966Z/apps-changes-vtext-report-proof.json`;
  `test-results/apps-changes-benchmark-reports-staging-2026-05-21T01-33-57-228Z/apps-changes-benchmark-reports-proof.json`;
  `test-results/apps-changes-removal-model-staging-2026-05-21T02-17-21-563Z/apps-changes-removal-model-proof.json`;
  `test-results/apps-changes-removal-model-staging-2026-05-21T02-17-21-563Z/apps-changes-removal-run-acceptance-proof.json`;
  `test-results/apps-changes-trace-surfacing-staging-2026-05-21T02-58-41-000Z/apps-changes-trace-surfacing-proof.json`;
  `test-results/apps-changes-portfolio-aggregation-staging-2026-05-21T03-55-47-000Z/apps-changes-portfolio-aggregation-proof.json`;
- proof covered Apps & Changes opening from the Desk on desktop and `390x844`
  mobile, four ordinary Change cards without package ids, collapsed Technical
  refs, mission VText dashboard creation/opening, and Chiron per-change VText
  report creation/opening;
- follow-up proof covered all four per-change VText reports on desktop and
  `390x844` mobile, with package refs, source/recipient acceptance refs,
  pulled manifest hashes, benchmark status, and artifact links inside the
  reports;
- Liquid benchmark artifact:
  `test-results/apps-changes-benchmarks-2026-05-21T01-00-45-3NZ/liquid-material-benchmark.json`
  measured WebGL rendering in Chromium and WebKit at desktop and `390x844`
  viewports, avg frame time 16.66-16.67ms and p95 <= 18.1ms;
- Python benchmark artifact:
  `test-results/apps-changes-benchmarks-2026-05-21T01-00-45-3NZ/python-code-mode-ab-benchmark.json`
  measured five matched repo tasks: bash 807.19ms average wall time vs Python
  129.28ms, estimated input payload tokens bash 128 vs Python 221;
- the first all-four deployed proof exposed and then fixed a compact/mobile
  Apps & Changes bug where the selected detail pane intercepted catalog card
  clicks; the regression is now covered by
  `Apps & Changes compact catalog remains clickable beside the detail pane`;
- earlier Chiron proof through the same app covered Try, recipient build
  verification, Install, and Rollback with rollback profile
  `refs/computers/primary/active` plus `route:primary`.
- latest Chiron removal proof covered the deployed rollback-only model on
  desktop and `390x844` mobile after a real pull -> Try -> recipient build
  Verify -> Install flow. It recorded adoption
  `adoption-chiron-shelf-b8d4c4d9-c787-4fe0-9f2b-c26bacb57efb`, candidate
  `candidate-chiron-shelf-c732c878-35fc-4026-acde-a379bf6f4794`, runtime digest
  `sha256:194a0b412998a1a373e9c489717181578012ed3edd7a9a1f71cd9f4e68a8879f`,
  UI digest
  `sha256:b2367c43c9e0b2d31eb51894237b3bdfef3fe9bfae040bb8e6f2e27972209024`,
  and rollback profile `refs/computers/primary/active` plus `route:primary`.
- run acceptance for the same Chiron evidence returned accepted
  `promotion-level` record `runacc-e89094a0f29869807b09` for trajectory
  `apps-changes-chiron-shelf`.
- latest trace-surfacing proof covered the selected Chiron Change detail on
  desktop and `390x844` mobile: Apps & Changes displayed the accepted
  `promotion-level` run acceptance, then opened Trace focused to trajectory
  `apps-changes-chiron-shelf-trace-surfacing-mpexedqq` and acceptance
  `runacc-2ec3b0a57b8ac4f0bc05`.
- that proof recorded a real recipient build with adoption
  `adoption-chiron-trace-trace-surfacing-mpexedqq`, candidate
  `candidate-chiron-trace-trace-surfacing-mpexedqq`, runtime digest
  `sha256:d764e5a1f56f1f781d0d453619d55370228e9ecc2463241f242dc7072fca0c84`,
  UI digest
  `sha256:b2367c43c9e0b2d31eb51894237b3bdfef3fe9bfae040bb8e6f2e27972209024`,
  base source SHA `575ff3014a85524da4233e60ce44345804d46807`,
  and head source SHA `5f46838346e861a2e3f0265f380f5f8a60ff8437`.
- latest portfolio aggregation proof covered Apps & Changes on desktop and
  `390x844` mobile after a fresh product-path Chiron pull -> adoption ->
  recipient build -> promote flow. It recorded account
  `apps-changes-portfolio-portfolio-aggregation-mpezvpo8@example.com`,
  adoption `adoption-chiron-portfolio-portfolio-aggregation-mpezvpo8`,
  candidate `candidate-chiron-portfolio-portfolio-aggregation-mpezvpo8`,
  trajectory `apps-changes-chiron-shelf-portfolio-aggregation-mpezvpo8`,
  and accepted `promotion-level` run acceptance
  `runacc-fa74b7932d330ba7f04d` with rollback ref
  `refs/computers/primary/active`.
- that proof recorded a real recipient build with runtime digest
  `sha256:28353f90b25e4c9092180f24e080aa8d55dc87beae8740811a5ecf944284300d`,
  UI digest
  `sha256:b2367c43c9e0b2d31eb51894237b3bdfef3fe9bfae040bb8e6f2e27972209024`,
  base source SHA `575ff3014a85524da4233e60ce44345804d46807`,
  head source SHA `90db78e095f9487c4ebb1efe73580a3e8b4c5edc`,
  runtime build duration `5m22.71091975s`, and UI build duration
  `7.927733702s`.
- that proof captured `desktop-apps-changes-portfolio.png`,
  `desktop-portfolio-vtext.png`, `desktop-trace-from-portfolio.png`, and
  `mobile-390x844-apps-changes-portfolio.png`, with DOM metrics showing four
  portfolio Changes, four reports, four benchmark/media links, one accepted
  record, and no visible Chiron package id in ordinary portfolio UI.

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

Recent deployed platform proof for mobile real desktop and Desktop Overview:

- behavior commit: `79b14e2cf6057ee33154dd1d2700ae8cf26ce355`;
- proof-harness follow-up commit: `5820a88`;
- CI/deploy run for behavior commit:
  `https://github.com/yusefmosiah/go-choir/actions/runs/26125883507`;
- test-only CI run:
  `https://github.com/yusefmosiah/go-choir/actions/runs/26126390728`;
- staging health reported proxy and upstream commit
  `79b14e2cf6057ee33154dd1d2700ae8cf26ce355`;
- deployed Playwright:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com GO_CHOIR_DESKTOP_BOOT_TIMEOUT_MS=300000 npx playwright test tests/mobile-real-desktop-overview.spec.js --project=chromium --workers=1 --timeout=360000 --reporter=line`;
- result: `2 passed`;
- proof covered Files, VText, Trace, and Podcast as overlapping non-fullscreen
  windows on `390x844` and desktop, with drag, resize, minimize, restore,
  Desktop Overview focus, and background suspension controls.

Recent deployed platform proof for heavy-session Desktop Overview:

- behavior commit: `b148461dafc6125fa321de9b10814cdc6af285b6`;
- CI/deploy run:
  `https://github.com/yusefmosiah/go-choir/actions/runs/26131606449`;
- staging health reported proxy and sandbox commit
  `b148461dafc6125fa321de9b10814cdc6af285b6`;
- deployed heavy-session Playwright:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com GO_CHOIR_DESKTOP_BOOT_TIMEOUT_MS=300000 npx playwright test tests/desktop-overview-heavy-session.spec.js --project=chromium --workers=1 --timeout=360000 --reporter=line`;
- result: `2 passed`;
- proof opened 12 real app windows through the Desk, persisted and reloaded the
  session, exercised restore recovery, and verified on both `390x844` and
  `1280x900`;
- DOM metrics: 12 visible windows, 11 heavy windows, 10 suspended windows, 1
  mounted heavy app body, 66 overlap pairs, 12 Overview cards, 12 Overview map
  windows, pressure `elevated`;
- proof covered Overview focus/resume, background suspension, Compute Monitor
  handoff, and keep-active-only recovery without fake thumbnails, host/global
  telemetry, broad kill controls, or phone-mode simplification.

Recent deployed platform proof for live-spatial Desktop Overview previews:

- behavior commit: `2f8ad7adc2697d6faff00dbc90991057c19781e9`;
- CI/deploy run:
  `https://github.com/yusefmosiah/go-choir/actions/runs/26133712240`;
- staging health reported proxy and sandbox commit
  `2f8ad7adc2697d6faff00dbc90991057c19781e9`, built at
  `20260520002859`, deployed at `2026-05-20T00:31:14Z`;
- deployed ordinary-session Playwright:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com GO_CHOIR_DESKTOP_BOOT_TIMEOUT_MS=300000 npx playwright test tests/mobile-real-desktop-overview.spec.js --project=chromium --workers=1 --timeout=360000 --reporter=line`;
- result: `2 passed`;
- ordinary proof covered Files, VText, Trace, and Podcast as overlapping
  windows on `390x844` and desktop, with bounded live Overview previews and
  fallback cards;
- deployed heavy-session Playwright:
  `PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com GO_CHOIR_DESKTOP_BOOT_TIMEOUT_MS=300000 npx playwright test tests/desktop-overview-heavy-session.spec.js --project=chromium --workers=1 --timeout=360000 --reporter=line`;
- result: `2 passed`;
- mobile and desktop heavy DOM metrics: 12 visible windows, 11 heavy windows,
  10 suspended windows, 1 mounted heavy app body, 66 overlap pairs, 2 live
  previews, 10 suspended previews, 12 Overview cards, 12 map windows, pressure
  `elevated`;
- proof kept live previews as transformed real DOM, not WebGPU/canvas
  screenshots, duplicated app mounts, persisted preview captures, fake
  thumbnails, host/global telemetry, or phone-mode simplification.

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
9. Apps & Changes needs honest source-level uninstall/disable capability
   records and non-Chiron accepted-record loading across source computers.
