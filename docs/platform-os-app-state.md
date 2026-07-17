# Platform OS And App State

**Status:** canonical platform-level state ledger
**Last updated:** 2026-07-10
**Changelog:** Reconciled the App Catalog and Apps & Changes/Features entries
with the shipped `features` app (frontend/src/lib/FeaturesApp.svelte) after
the 2026-05-28/31 frontend redesign cutover, the 2026-06-11 owner-approval
gate (commit `77f65651`), and the freshness CAS guard; moved unshipped
design intentions (Uninstall/Disable/portfolio review/trace evidence/Try-preview)
to a clearly labeled "Design intent, not shipped" section. The superseded
portfolio and promotion design sources remain available in Git history.
**Baseline checked:** `choir.news` primary-domain cutover, WebAuthn hard reset,
VM retention/pruning policy hardening, deploy-speed, and disk-pressure work.

This document records the current common state of the Choir automatic computer:
the platform substrate, desktop shell, app catalog, app boundaries, known proof,
and known gaps. Keep it updated when platform-level OS, shell, routing, app, VM
lifecycle, promotion, or public/default computer behavior changes.

## Executable App Inventory

`frontend/src/lib/apps/registry.ts` is the executable inventory and wins over
this narrative if they drift. At this revision it registers 20 surfaces:

```text
Files, Web Lens, Email, Compute Monitor, Pulse, Texture, Universal Wire,
Podcast, Image, Audio, Video, PDF, EPUB, Slides, Calendar, Features,
Candidate Review, Source, Super Console, Settings
```

Registration means code-live surface, not feature completeness or semantic
endorsement of its visible name. In particular, `Universal Wire` is retained
implementation vocabulary pending the World Wire rename; Candidate Review is a
read-only, non-deployed review surface; Source is the hidden reader/viewer path;
and Features activation is an adoption/lineage protocol rather than a served
runtime/UI cutover. Detailed rows below describe selected high-impact surfaces;
absence of a row does not mean absence from the registry.

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
- candidate, promotion, package, run-acceptance, Trace, or Texture evidence
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
  currently projected by `https://choir.news` and used as the conceptual base
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

- **Acceptance environment:** `https://choir.news`.
- **Source of truth:** GitHub `origin/main` for tracked platform files.
- **Deploy model:** GitHub Actions deploys tracked platform changes to Node B.
  Documentation-only commits intentionally skip CI/CD.
- **Legacy domains:** old pre-`choir.news` hostnames are not acceptance targets.
  `choir-ip.com` is legacy DNS and still needs Cloudflare authority/credentials
  before it can redirect reliably to the current primary domain.
- **Runtime shape:** Go proxy/runtime services, Svelte frontend, `vmctl`, NixOS
  host services, Firecracker-backed user computers, and embedded per-user Dolt
  for runtime/control product state.
- **Public platform state:** host/platform services own accounts, routing,
  lifecycle, publication/public artifact records, and aggregate health. Browsers
  do not talk to Dolt directly.
- **Computer lifecycle and disk retention:** active primary computers are
  protected ahead of candidate/worker computers. Ordinary primaries stay warm
  while capacity allows; configured always-on primaries have a protected lane.
  Candidate and worker computers hibernate first. Disposable worker/candidate
  VM disks are reclaimable after evidence has moved into product records.
  Staging test/proof primary computers are reclaimable only when they are
  explicitly classified by ephemeral account policy, currently `example.com`
  auth emails, and only after they are stopped, hibernated, or failed past the
  diagnostic TTL. Real primary computers are retained. Platform rollback keeps
  Git refs plus a small NixOS generation tail, not every historical guest image.

## Runtime Model Policy State

Choir's current model assignments are defaults, not architecture. The platform
catalog owns provider/model capability facts: provider family, model id, context
and output limits, text/image/tool support, reasoning controls, request schema,
deadline class, and any provider-specific carry-forward fields such as
reasoning content. A user computer's model policy owns the effective routing
preference for its roles and tasks.

The target state is:

- any configured compatible model can serve conductor, Texture, researcher,
  super, vsuper, co-super, verifier, or future roles;
- ChatGPT, Fireworks DeepSeek V4 Flash/Pro, Fireworks Kimi K2.6, and later
  catalog models are selectable by policy wherever the current turn's
  modality, tool, context, latency, and cost requirements match;
- Fireworks DeepSeek V4 Flash/Pro are text-only but valid for orchestration,
  writing, research, coding, and verifier work that does not require media
  input;
- Kimi K2.6 and ChatGPT multimodal paths are required only when a turn actually
  carries screenshots, images, video frames, or other media inputs;
- per-computer model policy is durable computer-owned state, editable through
  product paths and eventually by `super` in response to an owner prompt;
- platform deploys or Node B environment edits are not required merely to
  change which configured model a computer uses for a role;
- Trace and run evidence record the resolved provider/model/reasoning for each
  run without exposing provider secrets or turning ordinary UI into a provider
  dashboard.

This policy model deliberately separates "recommended defaults" from
"compatible execution." A strong coding model may be the default for `super` or
`vsuper`, and a fast writing model may be the default for Texture, but those are
computer policy choices. They must remain changeable without creating
role-specific provider assumptions in the runtime. The operational test is
per-turn compatibility: if the next turn is text-only, a text-only model such as
Fireworks DeepSeek V4 Flash/Pro remains eligible for any role, including
verification; if the next turn carries a screenshot or other media input, policy
must select a declared multimodal path such as Kimi K2.6 or ChatGPT multimodal,
or record a precise capability blocker.

Current generated policy declares separate verifier lanes: `verifier` defaults
to Fireworks DeepSeek V4 Pro for text-only evidence checks, while
`verifier_multimodal` defaults to Fireworks Kimi K2.6 for image/media-bearing
checks. These names are policy roles, not privileged agent castes.

## Desktop Shell State

The shell is a web desktop with floating windows, freely placed desktop icons,
Shelf/Desk menu, prompt bar, live status, and persisted desktop state for
signed-in users. Desktop state is now session-aware: app instances and semantic
order are shared user-computer state, while focus and window placement are
session/viewport presentation state. Mobile is intended to use the same
overlapping desktop model as desktop, with tighter geometry and better overview
controls rather than a separate phone-mode navigation stack.

A native macOS app (`cmd/desktop/`) wraps the same Svelte frontend in a Wails v3
window with `ASWebAuthenticationSession` for passkey auth via Safari. It launches
in cloud mode by default (connecting to `choir.news`). See
[cmd/desktop/README.md](../cmd/desktop/README.md).

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
- real local input renews a driver retired lease; only the driving session should save
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
- unified log/evidence live sync and long-lived real-account multi-device
  sessions still need broader product-path proof; deployed live-sync proof now
  covers media recents/progress, Files changes, Texture recent updates, shared
  app roster/order, session-local focus/geometry, and `/api/ws?after_seq=`
  catch-up on top of the driver-retired-lease state model;
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
| **Files** | First-class file browser with navigation, upload, text-to-Texture open, known media routing to Image/Audio/Video/PDF/EPUB apps, and live file-change notifications for the current directory. Unknown binaries still download. | Keep proving that PDF/EPUB/media open in apps instead of downloading. Add richer previews only through app boundaries and broaden live file events into richer change history. |
| **Texture** | Primary appagent and versioned document editor. Owns canonical document versions and prompt-created writing surfaces. Source citation is tri-state: every source entity is cited (`source_ref` in the body), toolbar-only (a Style.texture style source), or marked-unused (`mark_source_unused` with rationale). The former `source_embed` block node is removed; all citations are `source_ref` with `display_mode` (`numbered_ref` \| `expanded_ref`). There is no `WireTexture` prompt control-flow branch; article-format guidance is unconditional, driven by the default Style.texture. Target direction is a multimedia computational-essay surface with typed snippets for sources, media, evidence, candidate demo videos, interactive graphics, and nested Textures. | Continue version-advancement stability hardening. Add durable snippet/embed records, Pretext-powered responsive reading/layout, expansion into owning app windows, and video-first candidate approval reports without mixing worker patches directly into canonical text. |
| **Trace Evidence** | Trace remains as structured evidence, unified logs, run bundles, acceptance records, and diagnosis artifacts. The visual Trace app is no longer a product direction and should be unshipped rather than redesigned. | Preserve machine-readable evidence for zot, Texture reports, run acceptance, and operator diagnosis. Do not keep an emergency human Trace UI. |
| **Web Lens** | Explicit live/original web inspection surface. It still carries legacy `browser` implementation IDs, data attributes, session tables, and iframe behavior, but the product object is Web Lens, not a general manual Browser app. Durable web-derived sources should default to Source Viewer/reader artifacts before live/original inspection. | Rename or quarantine browser-session implementation residue over time. Backend control/screenshot support remains a distinct substrate frontier for Web Lens, source acquisition, and candidate-computer inspection; it must not become a bypass around product APIs or the primary source-gathering workflow. |
| **Super Console** | Target replacement for retired Terminal: singleton repair app inside each user computer, backed by out-of-process `zot` running separately from the runtime MAS. It reads unified logs/source/files/process state, can run command-actuation such as `!` commands, patches/rebuilds/restarts locally, verifies, and writes markdown diagnosis reports that Texture can open. | Do not expose retired raw Terminal as a normal app. Do not let Super Console become the main scripting/product surface or spawn multiple retired chat-agent sessions. It is repair mode when Texture/MAS malfunctions. |
| **Settings** | Account, runtime health, server-backed theme presets/editing, and low-level promotion/adoption evidence. Promotion queue refresh UI has been removed in favor of live product events. | Theme system needs taste/design hardening. Settings should not be the main owner-facing install surface; Features owns ordinary change discovery and adoption. Runtime health still needs a true push source rather than opportunistic event refreshes. |
| **Compute Monitor** | First-class app for user-computer health and recovery. It uses authenticated product APIs to show only the current user's current computer, background candidate computers, warmness/protection, current runtime health, app/window restore weight, safe desktop-state recovery actions, and disabled unsafe controls. Manual refresh UI has been removed. | Add true event-backed computer status updates, trend history, app-owned process/resource accounting, candidate discard/hibernate actions, conductor recovery intents, and stronger long-session regression proof. |
| **Features** (`frontend/src/lib/FeaturesApp.svelte`, app id `features`) | Launcher-facing AppChangePackage catalog. **Import** creates an adoption for hard-coded target `primary` and starts recipient build/verification. **Activate** records owner approval then calls promote; Roll back/Roll forward call the corresponding protocol APIs. | "Activate" updates `ComputerSourceLineageRecord` (`ActiveSourceRef`, digests, `RouteProfile`) with approval and freshness guards. Nothing in the ordinary personal-computer path consumes `RouteProfile` to switch routes, restart a process, or swap runtime/UI binaries. Treat active/rolled-back labels and current API promotion-level records as adoption/lineage evidence, not completed ComputerVersion promotion. Preview exists server-side but has no Features UI. |
| **Podcast** | Working app-grade v0. It has library/search/recommendations, hidden advanced RSS import, feed detail, scrollable episode list, full player controls, speed/seek, and server-backed playback-position sync. | Treat as a regression/reference app, not the center of the next media mission. Continue improving subscription durability, played/unplayed state, conductor actions, and Texture radio continuity later. |
| **Image** | First-class app with source resolution, title, fit/original, zoom controls, rotate left/right, reset, and image rendering. | Add pan/drag, touch/pinch behavior, folder gallery navigation, richer metadata, and persisted viewer state. |
| **Audio** | First-class app with play/pause, 15s back, 30s forward, scrubber, speed, current/duration, native audio fallback, server-backed recents, and server-backed playback-position sync. | Add queue/playlist from Files, metadata, Media Session integration, transcript/Texture hook, and keyboard controls. |
| **Video** | First-class app for native video and YouTube embeds. Native video has custom/native controls, speed/seek, server-backed recents, and server-backed playback-position sync. | Add fullscreen/theater fit, captions/subtitles, transcript/Texture hook, playlist/folder navigation, and consistent YouTube/native control surfaces. |
| **PDF** | Real reader path using PDF.js: browser-fetchable PDFs render to canvas pages with actual page count, page navigation, zoom/fit width/fit page, text search, and server-backed recents. Files/prompt routes can open the PDF app. | Add thumbnails/outline, annotations, richer text selection, and server-side/import fallback for CORS-blocked remote PDFs. |
| **EPUB** | Real reader path using EPUB archive parsing: browser-fetchable EPUBs parse container/package/spine, render chapters as safe text blocks, expose chapter selection, font/width/progress controls, search, server-backed recents, and server-backed reading-position sync. Extracted text still renders as a reader source. | Add richer XHTML formatting, EPUB nav/TOC semantics, bookmarks, image assets, server-side extraction, and Texture/transclusion handoff. |
| **ContentViewer** | Legacy generic content surface still exists in code but is not the place to add media behavior. | Do not put new app work here. Retain only as fallback/dispatcher/inspector until it can be safely retired or narrowed. |

## Features: design intent, not shipped

The predecessor "Apps & Changes" app described capabilities and a removal
model that were never carried into the shipped `features` app
(`frontend/src/lib/FeaturesApp.svelte`) after the 2026-05-28/31 frontend
redesign cutover. These remain real design intentions worth preserving, but
none of the following ships today. Do not describe them as current product
behavior.

- **Uninstall / Disable removal model.** The pre-cutover design called for an
  honest removal/recovery model: rollback-only labeling for changes without a
  verified inverse source patch, Uninstall disabled without a verified
  inverse source patch, Disable disabled without a declared feature
  flag/capability toggle, and empty rollback-profile JSON not accepted as
  evidence. Features today only ships **Roll back** / **Roll forward** against
  a recorded rollback ref; there is no Uninstall or Disable action, declared
  or stubbed, in the current UI or adoption API. Source-level
  uninstall/disable semantics remain a real gap (see Near-Term Gaps item 9).
- **Portfolio review panel.** The pre-cutover design aggregated multiple
  experiment Changes into a portfolio view with report/benchmark coverage and
  an accepted-promotion-level row per experiment. Features has no portfolio
  aggregation view; it is a flat catalog list with a single detail pane.
  Portfolio-style review (headline, plan view, check badges gating Approve,
  restore-point timeline) remains target behavior, not shipped behavior.
- **Trace integration.** The pre-cutover design intended the selected Change
  to surface run-acceptance/evidence refs and expose evidence without a separate
  visual retired Trace app. In the shipped Features app, this path is still a
  compatibility stub and may return `"Trace UI is unshipped"` (retired) when a trace ref
  exists. That behavior is a known bug-shaped gap: the visual retired Trace app is not a
  product direction, and product-ready behavior should replace this stub with a
  trace-evidence/provenance action and run-acceptance linkage.
- **Try/preview flow.** The pre-cutover design described an internal-frame
  preview of a candidate before installing. A preview endpoint already EXISTS
  server-side at `/api/adoptions/{id}/preview/*` (requires a verified
  recipient build), but the Features UI never calls it — there is no
  Try-it-now button or preview frame. Wiring it remains unowned product work.
- **Promotion as a real route flip.** The pre-cutover design implied that
  Install/Activate changes what the computer actually serves. Today
  "Activate" only updates the `ComputerSourceLineageRecord`
  (`ActiveSourceRef`, digests, `RouteProfile`) — a durable pointer flip in
  product state, with no route switch, process restart, or binary swap
  consuming it. The completed route-over-ComputerVersion work does not authorize
  this separate app-adoption cutover; a future promoted Definition is required.
  Deleted portfolio/design chains are not executable successors.

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
- Texture may embed snippets from other apps, but the full-control surface remains
  the owning app. Embedded snippets are durable artifact references and layout
  intent; they are not a reason to collapse Image, Audio, Video, Podcast, PDF,
  EPUB, Trace Evidence, Features, or Web Lens back into a generic viewer.
- Each Texture snippet should expose an expansion target that opens the relevant
  app/window while preserving the reader's Texture position. Multi-window reading
  is a core affordance for sources, demos, media, nested Textures, and evidence.
- Candidate coding work intended for human approval should be video-first when
  visual or temporal behavior matters. The Texture approval packet should embed
  the demo video if available, then link package/diff refs, verifier evidence,
  Trace/run-acceptance refs, rollback path, risks, and follow-up requests.
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
  stale data. Web Lens page reload remains live-web navigation, not Choir state
  synchronization.
- Existing Texture and Trace SSE streams remain valid scoped transports where
  they preserve stronger revision/trajectory catch-up semantics.
- Texture source entities are tri-state: every source entity is cited
  (`source_ref` in the body with `display_mode` `numbered_ref` or
  `expanded_ref`), toolbar-only (a Style.texture style source that shapes
  writing but is not cited in the body), or marked-unused
  (`mark_source_unused` with a rationale in revision metadata). No source is
  silently ignored. The former `source_embed` block node is removed. There is
  no `WireTexture` prompt control-flow branch; article-format and citation
  guidance is unconditional, driven by the default Style.texture registered as
  a source entity.

## Current Proof Anchors

The current primary staging origin is `https://choir.news`. Older proof commands
below old staging hostnames are historical evidence from before
the 2026-05-26 domain cutover, not current instructions for new acceptance runs.

Recent deployed platform proof for the primary-domain cutover:

- behavior/deploy commits:
  `84ad8d0d0c87e36cd329bca226b17432c43b57d7`,
  `7fd49beee9b1ed517e73bc0885a6f3f8a2d1e6a5`, and
  `2b2243394ce86cc8a79d62e615fc6039c8c658a9`;
- final evidence checkpoint:
  `a077efa` from the pruned primary-domain cutover mission;
- CI/deploy run:
  `https://github.com/choir-hip/go-choir/actions/runs/26441752735`;
- staging health reported proxy and sandbox commit/deployed_commit
  `2b2243394ce86cc8a79d62e615fc6039c8c658a9`, deployed at
  `2026-05-26T08:40:19Z`;
- deployed browser proof used real origin `https://choir.news`, registered a
  fresh account after the WebAuthn RP-ID hard reset, logged out, logged back in,
  and observed a virtual authenticator credential for RP ID `choir.news`;
- auth database backup before the hard reset:
  `/var/lib/go-choir/auth/auth.db.pre-choir-news-20260526T084548Z`;
- residual blocker: `choir-ip.com` remains pointed at legacy DNS until
  Cloudflare credentials/record authority are available.

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
  historical live-sync proof command against the old staging hostname;
- result: `1 passed`;
- proof used one desktop context at `1440x920` and one mobile context at
  `390x844` for the same fresh authenticated user computer;
- proof covered shared app instance convergence for Files, Audio, and Texture;
  desktop focus stayed on Files while mobile focus stayed on Texture; desktop
  Files geometry remained stable while mobile drove Audio/Texture;
- proof covered media recents/progress without localStorage or manual refresh:
  the mobile Audio app showed the proof audio and `0:42 / 6:00`, while the
  mobile product API returned `current_time: 42`;
- proof covered Files and Texture content updates: mobile Files showed
  `live-sync-proof-1779474356092.txt`, and mobile Texture recent showed
  `Live sync Texture proof 1779474356092`;
- proof covered websocket catch-up from `/api/ws?after_seq=6`, returning missed
  `media.recent.updated`, `media.progress.updated`,
  `desktop.driver_lease.updated` (retired), `desktop.app_instances.updated`,
  `desktop.window_placement.updated`, `file.changed`, and
  `texture.document_revision.created` events;
- proof covered Desktop Overview convergence: desktop and mobile card/map app
  ids all matched `files`, `audio`, `texture` while local focus/z-index and
  placement remained session-specific;
- artifacts:
  `test-results/live-sync-driver-lease-staging-20260522T182540Z/metrics.json` (retired),
  `desktop-driver-files.png`, `mobile-passive-files-synced.png`,
  `desktop-overview-order.png`, `mobile-overview-order.png`,
  `desktop-after-app-content-sync.png`, and
  `mobile-driver-texture-content-sync.png`.

Recent deployed platform proof for Apps & Changes, Texture reports, and benchmark
evidence (historical: this proof predates the 2026-05-28/31 frontend redesign
cutover that replaced "Apps & Changes" with the `features` app described
above; the underlying adoption/promotion product APIs and evidence model are
still in active use by Features, but the "Apps & Changes" UI and its
portfolio review panel no longer ship):

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
  `test-results/apps-changes-texture-report-staging-2026-05-21T00-50-49-966Z/apps-changes-texture-report-proof.json`;
  `test-results/apps-changes-benchmark-reports-staging-2026-05-21T01-33-57-228Z/apps-changes-benchmark-reports-proof.json`;
  `test-results/apps-changes-removal-model-staging-2026-05-21T02-17-21-563Z/apps-changes-removal-model-proof.json`;
  `test-results/apps-changes-removal-model-staging-2026-05-21T02-17-21-563Z/apps-changes-removal-run-acceptance-proof.json`;
  `test-results/apps-changes-trace-surfacing-staging-2026-05-21T02-58-41-000Z/apps-changes-trace-surfacing-proof.json`;
  `test-results/apps-changes-portfolio-aggregation-staging-2026-05-21T03-55-47-000Z/apps-changes-portfolio-aggregation-proof.json`;
- proof covered Apps & Changes opening from the Desk on desktop and `390x844`
  mobile, four ordinary Change cards without package ids, collapsed Technical
  refs, mission Texture dashboard creation/opening, and Chiron per-change Texture
  report creation/opening;
- follow-up proof covered all four per-change Texture reports on desktop and
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
- latest evidence-surfacing proof covered the selected Chiron Change detail on
  desktop and `390x844` mobile: Apps & Changes displayed the accepted
  `promotion-level` run acceptance and linked trajectory
  `apps-changes-chiron-shelf-trace-surfacing-mpexedqq` plus acceptance
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
  `desktop-portfolio-texture.png`, `desktop-trace-from-portfolio.png`, and
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
  prompt routing, launcher/shell smoke, Texture/Trace coexistence, and Podcast
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
  historical mobile overview proof command against the old staging hostname;
- result: `2 passed`;
- proof covered Files, Texture, Trace, and Podcast as overlapping non-fullscreen
  windows on `390x844` and desktop, with drag, resize, minimize, restore,
  Desktop Overview focus, and background suspension controls.

Recent deployed platform proof for heavy-session Desktop Overview:

- behavior commit: `b148461dafc6125fa321de9b10814cdc6af285b6`;
- CI/deploy run:
  `https://github.com/yusefmosiah/go-choir/actions/runs/26131606449`;
- staging health reported proxy and sandbox commit
  `b148461dafc6125fa321de9b10814cdc6af285b6`;
- deployed heavy-session Playwright:
  historical desktop heavy-session proof command against the old staging hostname;
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
  historical mobile overview proof command against the old staging hostname;
- result: `2 passed`;
- ordinary proof covered Files, Texture, Trace, and Podcast as overlapping
  windows on `390x844` and desktop, with bounded live Overview previews and
  fallback cards;
- deployed heavy-session Playwright:
  historical desktop heavy-session proof command against the old staging hostname;
- result: `2 passed`;
- mobile and desktop heavy DOM metrics: 12 visible windows, 11 heavy windows,
  10 suspended windows, 1 mounted heavy app body, 66 overlap pairs, 2 live
  previews, 10 suspended previews, 12 Overview cards, 12 map windows, pressure
  `elevated`;
- proof kept live previews as transformed real DOM, not WebGPU/canvas
  screenshots, duplicated app mounts, persisted preview captures, fake
  thumbnails, host/global telemetry, or phone-mode simplification.

Recent deployed runtime proof for async supervision and Texture worker-update
dashboards:

- behavior commits:
  `490f70aafed53802a01e5e763f5afa3ccab554fd` and
  `846cfbbf2eb47206c6262d0ab032845c013ff8eb`;
- CI/deploy run for `846cfbb`:
  `https://github.com/yusefmosiah/go-choir/actions/runs/26327200933`;
- deploy job:
  `https://github.com/yusefmosiah/go-choir/actions/runs/26327200933/job/77507019637`;
- staging health reported proxy and upstream commit
  `846cfbbf2eb47206c6262d0ab032845c013ff8eb`, built at
  `20260523074113`, deployed at `2026-05-23T07:43:07Z`;
- deployed product proof:
  `GO_CHOIR_RUN_ASYNC_SUPERVISION_PROOF=1 ... pnpm exec playwright test
  tests/async-supervision-runtime-proof.tmp.spec.js --project=chromium`;
- evidence directories:
  `test-results/async-supervision-runtime-proof-846cfbb-20260523T075607Z`
  and
  `test-results/async-supervision-runtime-proof-846cfbb-texturewait-20260523T080251Z`;
- Playwright trace/video:
  `frontend/test-results/async-supervision-runtime--aad53-evidence-or-precise-blocker-chromium/trace.zip`
  and
  `frontend/test-results/async-supervision-runtime--aad53-evidence-or-precise-blocker-chromium/video.webm`;
- result: `1 passed` in the Texture-wait proof;
- trajectory/submission:
  `2d45d210-cce7-4276-9ec8-b68d62cafb68`;
- accepted run acceptance:
  `runacc-0addeeafd0abe7c9154d` at `staging-smoke-level`;
- Texture dashboard document:
  `b7663242-616b-4a23-a80f-bc7065f059fb`, final head revision
  `192cfee2-2601-4664-b945-db4eeb94e95f`;
- worker proof:
  request/start/observe/finish converged on worker run
  `6e9eaaf3-5119-4318-8dde-a74e91a65a7b`, worker VM
  `vm-2e6c63b2b834b6441c324cb32f82d24f`, worker
  `worker-c38f1d6d33760bd2`;
- proof covered successful `submit_worker_update` mirroring into the active
  Texture channel (`worker_submit_update_mirrored`,
  `mirrored_worker_update_count=1`), Texture synthesis into an owner-readable
  request/start/observe/finish dashboard, Trace-visible worker events, and
  runtime-supervision run acceptance without AppChangePackage requirements;
- first Chiron sequential rerun after that proof did not produce a package:
  evidence directory `test-results/chiron-sequential-20260523T081544Z`,
  trajectory `d850d92a-b90d-48f3-842a-f9fa5d5d3a37`, Texture dashboard
  `bcb8329e-ce45-426c-9bc5-5552fca3208f`, run acceptance
  `runacc-86cb5ab95084483a9084` at `staging-smoke-level`, and outcome
  `no_matching_package`. That probe isolated the next runtime gap:
  active `finish_worker_delegation` needed to return and checkpoint
  actionable worker/child-run evidence before terminal completion;
- caveat: screenshot/video evidence was captured by the outer Playwright proof
  harness. Product-requested `worker-playwright` evidence capture remains the
  next proof requirement for UI experiment reruns.

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
2. EPUB needs richer formatting, assets, bookmarks, and Texture handoff.
3. Image needs pan/touch/gallery/persisted viewer state.
4. Audio and Video need queues, metadata, Media Session, transcripts, and richer
   Files context.
5. Unified logs/evidence must stay machine-readable for long runs.
6. Texture must remain stable while live updates and Super Console repair reports
   exist alongside it.
7. Shelf/Desk/Desktop Overview behavior needs richer mobile desktop proof,
   live thumbnails, and configurable Shelf placement.
8. Candidate/promotion surfaces should become contextual product surfaces.
9. Features needs honest source-level uninstall/disable capability records,
   non-Chiron accepted-record loading across source computers, a real
   route-flip consumer for promotion (M6), and a Try-it-now flow wired to the
   existing preview endpoint (M7). See "Design intent, not shipped" below.
