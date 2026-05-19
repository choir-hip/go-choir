# MissionGradient: Real Media Apps And UX Sweep v0

Status: ready for overnight execution
Date: 2026-05-19
Operator: Codex supervising staging, product-path Playwright, Choir-in-Choir workers where healthy, git, CI, deploy, Trace, VText, and owner review

## One-Line Goal String

```text
/goal Run docs/mission-real-media-apps-ux-sweep-v0.md as a Codex-operated MissionGradient mission: make Choir's web desktop feel coherent by replacing generic media/content surfaces with real apps and hardening the shell/evidence UX around them. Build first-class Image, Audio, Video, PDF, EPUB, and Podcast apps with app-specific state, controls, routing, Files integration, launcher entries, mobile desktop behavior, and product-path proof; keep shared helpers small and subordinate, not a generic MediaFileApp/ContentViewer island. Also continue the broader UX sweep across Trace readability, VText coexistence/flicker, prompt/bottom-bar/app switching, contextual candidate/promotion surfaces, logged-out read/explore with auth only at mutation, and VM priority/status UX. Preserve mobile as a powerful floating-window desktop, not a reduced phone mode. Prefer Choir-in-Choir worker/candidate dispatch when healthy; if substrate blocks progress, root-cause and repair it directly through git/CI/deploy. Finish with staging identity, screenshots, DOM metrics, VText/Trace/run-acceptance evidence, rollback refs, residual risks, and the next realism axis.
```

## Mission Frame

This mission is not "add a few media controls." It is a product-boundary repair.

The current system has the right direction but the wrong center of gravity: Podcast has been split out, but Image, Audio, Video, PDF, and EPUB are still mostly thin wrappers over one generic media surface. That repeats the mistake that made Podcast feel like a content artifact instead of an app. Files also still treats important media files as downloads in the user path, and the launcher does not make the media apps visible as first-class desktop capabilities.

The overnight objective is to make the first media-app family feel real, while using that work to harden the automatic computer shell and evidence surfaces. The product should feel like a powerful web desktop that happens to run well on mobile, not a website full of generic viewers.

## Real Artifact

The artifact is the deployed Choir automatic-computer UX substrate:

```text
Files and content references
-> typed app routing
-> first-class media apps
-> floating desktop shell
-> prompt/conductor app actions
-> Trace/VText/candidate evidence surfaces
-> deployed mobile and desktop proof
```

The artifact is not:

- a single `MediaFileApp` with branches for every type;
- a swollen `ContentViewer`;
- a set of separate filenames that still share one generic product behavior;
- a local-only demo;
- a checklist that passes while the apps remain unpleasant or unreachable.

Shared code is allowed only when it is a subordinate primitive, such as transport controls, source URL helpers, zoom viewport, metadata drawer, or an app chrome helper. The user-facing app identity, state machine, routing, controls, empty/loading/error states, and tests must be app-specific.

## Starting Belief State

Known state:

- Staging has the recent vmctl liveness fix at `6836249`.
- `PodcastApp.svelte` exists and is no longer trapped inside `ContentViewer`.
- `ImageApp.svelte`, `AudioApp.svelte`, `VideoApp.svelte`, `PdfApp.svelte`, and `EpubApp.svelte` exist, but are thin wrappers around `MediaFileApp.svelte`.
- `ContentViewer.svelte` still exists as a generic content surface and can still exert old design pressure.
- The Files app currently opens text files in VText but downloads non-text files, including PDF and EPUB, instead of opening the matching apps.
- The launcher currently emphasizes core apps and does not fully present media apps as first-class capabilities.
- Trace has mobile drill-in work, but the user still observed multiple vertical scroll zones and unreadable overlap on mobile.
- VText has reported flicker/coexistence issues with Trace.
- VM priority policy is documented in `docs/vm-priority-policy.md`, but the product UX still needs clearer warm/waking/degraded state.

Highest-impact uncertainty:

- Whether the worst remaining UX failures are caused primarily by app-local layout, shell geometry/focus behavior, or routing/authority boundaries. The mission should probe all three early and fix the shared substrate when one root cause explains multiple symptoms.

Next observations:

- Product-path Playwright on staging for Files -> PDF/EPUB/Image/Audio/Video opens.
- Mobile screenshots and DOM metrics for each media app at `390x844`.
- Trace mobile scroll-zone metrics with a real trajectory.
- VText typing/focus test while Trace is open.
- Logged-out launcher/app-open proof for read/explore surfaces.

## Invariants

- Mobile remains a desktop. Preserve floating windows, overlap, task switching, and power-user density. Add touch affordances; do not collapse into a single-app phone navigation model.
- Active user computers must not be mutated directly for risky development. Use candidate/worker paths when healthy; direct Codex platform fixes are allowed only for platform substrate repair and must land through git/CI/deploy.
- App boundaries must become clearer. Podcast, Image, Audio, Video, PDF, and EPUB are real apps. `ContentViewer` may remain only as a fallback/dispatcher/inspector, not as a primary media app.
- A generic media component may not be the product architecture. Shared primitives are fine; a single app-shaped branch tree is not.
- Files and prompt/conductor routes should open the same app family for the same content type.
- The app launcher should expose real capabilities. Users should not need prompt magic or manual IDs to discover obvious apps.
- Logged-out read/explore should work where privacy permits. Mutations, persistence, uploads, private state, worker/candidate actions, provider calls, subscriptions, adoption, and promotion require auth.
- Trace must stay truthful. Do not hide failed evidence or replace missing artifacts with fake summaries.
- No fake placeholders: no fake transclusion panels, fake readers, fake media renderers, or JSON-only success labels.
- Platform behavior changes require:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> deployed Playwright/API acceptance
```

## Value Criterion

Minimize:

```text
generic-viewer leakage
+ app-boundary confusion
+ media-control incompleteness
+ Files/download routing surprises
+ launcher discoverability failure
+ mobile shell friction
+ evidence inspection friction
+ VText focus instability
+ auth overblocking
+ manual-ID candidate/promotion UX
+ hidden state and verifier Goodharting
+ future cleanup debt
```

subject to the invariants above.

The mission moves uphill when a normal user can open the launcher, open Files, click a PDF or EPUB, view it in the appropriate app, play audio/video/podcasts with standard controls, inspect Trace, edit VText, and switch windows on mobile without fighting the shell.

## Quality Gradient

Expected quality: `solid`.

Solid means:

- each media app has its own product state and tests;
- shared helpers are named as helpers, not as the app identity;
- Files and prompt routes agree on media type routing;
- launcher entries exist and fit on mobile;
- app controls are reachable in floating windows on mobile and desktop;
- failures are precise and honest;
- staging proof exists after deploy.

Substandard:

- renaming `MediaFileApp` without changing behavior;
- leaving PDF/EPUB clicks as browser downloads;
- making EPUB pretend to render when extraction/reader support is absent;
- adding a "view" that is only metadata/provenance/source;
- hiding media apps behind prompt-only access;
- fixing mobile by disabling desktop window behavior;
- claiming success from local screenshots only.

## Homotopy Axes

Increase realism continuously:

- media app boundary: generic viewer -> app-specific route/state/control -> shared helper primitives only where justified;
- file routing: text-only VText open -> PDF/EPUB/media app open -> unknown binary download fallback;
- launcher: core apps only -> all first-class apps -> grouped/adaptive launcher;
- mobile shell: basic fit -> reliable focus/raise/restore -> snap/overview/top-or-bottom panel options;
- Trace: readable summary -> timeline/inspector drill-in -> run acceptance/candidate/promotion artifacts;
- VText: basic edit -> no flicker under live updates -> stable Trace coexistence;
- auth: launch wall -> read/explore guest mode -> mutation-specific auth prompts;
- proof: local build -> focused local Playwright -> staging Playwright -> Trace/VText/run-acceptance evidence.

Avoid discontinuous fake ladders. A simplified PDF app can lack full search, but it must still be the real PDF app path. A simplified EPUB app may precisely block if extraction is missing, but it must not fake a reader.

## Priority Surfaces

### P0: Real Media App Boundaries

Create or refactor the media apps so each has app-specific behavior:

- Image app: pan/zoom, fit/original, metadata/inspect secondary, touch drag/zoom if feasible.
- Audio app: play/pause, seek, speed, duration/progress, source/title metadata, persistence plan.
- Video app: native video controls or embedded provider controls, fit/full-window, source fallback, speed where browser supports it.
- PDF app: inline open from Files and prompt routes, page navigation, fit width/page, zoom, scroll, precise blocker for search if deferred.
- EPUB app: either extracted-text reader with table/chapter/progress/font controls, or a precise no-fake-reader blocker with the extraction path as next probe.
- Podcast app: subscriptions/library/search/detail/player/progress with RSS import hidden under advanced.

Do not accept a single `MediaFileApp` as the app layer. If shared code remains, split it into primitives with app-owned state machines.

### P0: Files And Launcher Routing

Files:

- Text/Markdown/VText files open in VText.
- PDF files open PDF app.
- EPUB files open EPUB app.
- Image/audio/video files open their apps.
- Unknown binary files still download.
- The user sees no unexpected browser download for known media types.

Launcher:

- Image, Audio, Video, PDF, EPUB, and Podcast are visible as first-class apps.
- The launcher remains usable on mobile; scrolling/grouping is acceptable.
- Guest mode can launch read/explore apps without auth where no private state is touched.

### P0: Mobile Desktop Shell

Preserve the desktop model while making it usable:

- tapping a task/app indicator reliably raises and focuses the window;
- minimize/restore does not require toggling show desktop first;
- prompt/bottom panel grows only for content and shrinks back;
- media apps, Trace, VText, Files, and Podcast fit within the mobile workspace;
- snap/fit/overview affordances are added or precisely blocked with the next implementation path.

### P0: Trace Evidence App

Trace must be a usable evidence app:

- exactly one intentional app-level vertical scroll surface on mobile, or a documented reason for a nested scroll where it is a code/payload block;
- Runs/Summary/Timeline/Inspector reachable on `390x844`;
- run acceptance, rollback, worker/export/candidate/promotion evidence readable;
- long payloads wrap or scroll inside bounded blocks without taking over the page.

### P1: VText Stability

Create or repair a focused test:

- VText and Trace open together on mobile;
- user types while live/refresh/head events occur;
- selection/focus is not lost;
- visible flicker is minimized or precisely isolated.

### P1: Candidate/Promotion Context

Candidate Desktop and promotion views should not require manual IDs in ordinary flow:

- show contextual candidate/export/promotion cards from current Trace/run/promotion evidence;
- keep manual ID entry behind an advanced/debug drawer;
- link rollback and verifier evidence from product-visible surfaces.

### P1: VM Priority And Wake UX

Carry forward the vmctl priority policy:

- real user primary computers should stay warm under capacity;
- candidate/worker VMs can hibernate aggressively;
- the UI should say warm/waking/recovering/degraded rather than hanging indefinitely;
- do not expose private VM/user IDs in public health.

## Investigation And Cognitive Reframing

Before stopping on a blocker:

1. Classify it as app-local, shell-level, auth-boundary, routing/substrate, deploy, external provider/browser, or invariant-level.
2. Run the smallest root-cause probe that distinguishes those classes.
3. If the next probe/fix is inside current authority, execute it rather than ending the mission.
4. Apply 2-5 route-changing transforms before declaring a hard blocker:
   - Boundary transform: is this bug caused by the wrong app owning the state?
   - Evidence transform: what would convince a skeptical reviewer this behavior exists on staging?
   - Homotopy transform: is the simplification continuously deformable into the real app?
   - Authority transform: is this failing because read/explore and mutation share one auth gate?
   - Shell-vs-app transform: would fixing the window shell solve this class across several apps?

Do not stop with "needs follow-up" while an executable safe probe remains.

## Receding-Horizon Control

Run in bounded loops:

1. Baseline one user-visible failure with Playwright/screenshot/DOM metric.
2. Identify ownership: app, Files, launcher, shell, auth, Trace, VText, vmctl, or deploy.
3. Make a small mutation with tests.
4. Run focused local verification.
5. Commit and push when behavior changes.
6. Monitor CI/deploy.
7. Verify staging identity.
8. Run deployed proof.
9. Update evidence ledger and choose next highest-gradient failure.

Prefer one coherent commit per substrate slice. Larger commits are acceptable only when separating app boundaries requires shared helper extraction.

## Dense Feedback Channels

Use:

- `npm run build` in `frontend`;
- focused Playwright specs for Files, media apps, Podcast, Trace, VText, and shell;
- `go test` for any runtime/sandbox/proxy route changes;
- staging `/health` build identity;
- screenshots at `390x844` and desktop viewport;
- DOM metrics for overflow, scroll ownership, visible controls, and window bounds;
- Trace trajectory and run-acceptance records for self-development/proof runs;
- VText report/certificate for final human review.

## Evidence Ledger

For each claim record:

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

- Files opens PDF/EPUB/media apps without downloads.
- Launcher exposes media apps and remains usable on mobile.
- Each media app has reachable controls on mobile.
- Trace has usable mobile drill-in and bounded scroll.
- VText remains stable beside Trace.
- Logged-out read/explore works without mutating private state.
- Staging is on the pushed commit.

## Forbidden Shortcuts

- Keeping one generic `MediaFileApp` as the product app and calling wrappers "separate apps."
- Moving debug/provenance/source controls into primary app chrome.
- Fake EPUB/PDF/media renderers.
- Browser-public internal/test routes as product proof.
- Manual success seeding.
- Local-only proof for deployed claims.
- Disabling mobile floating windows to make tests easier.
- Mutating active computers directly for risky candidate work.
- Claiming a platform deploy proves user-computer promotion or fleet adoption.

## Rollback Policy

- Every platform patch has a git rollback SHA.
- UI changes must be revertible without data migration when possible.
- File-serving changes must preserve download fallback for unknown binaries.
- App state migrations, if any, must be additive and tolerate missing old state.
- VM priority/config changes must preserve candidate/worker reclaim and not leak private IDs.

## Learning Side-Channel

Update this mission doc or a follow-up proof doc with:

- app-boundary decisions;
- shell-vs-app root causes;
- media app shared-helper extraction choices;
- staging screenshots/metrics;
- blockers and next probes.

Do not bury architecture learnings only in chat.

## Stopping Condition

Stop only when either:

1. Full-chain UX proof exists on staging:
   - Files opens PDF/EPUB/image/audio/video into real apps;
   - launcher exposes media apps;
   - Podcast remains usable with player/library/search/progress path;
   - Trace and VText are usable together on mobile;
   - prompt/bottom bar/window switching are materially improved or precisely bounded;
   - logged-out read/explore and auth-on-mutation are verified;
   - staging identity, screenshots, DOM metrics, tests, rollback refs, residual risks, and next realism axis are recorded.

2. A hard blocker remains after root-cause probes and cognitive transforms:
   - exact failing layer named;
   - evidence captured;
   - no safe executable next probe remains inside authority;
   - rollback/no-mutation status stated;
   - next safe probe is concrete.

Do not stop merely because one app slice passed. The objective is coherent automatic-computer UX substrate, with real media apps as the pressure test.
