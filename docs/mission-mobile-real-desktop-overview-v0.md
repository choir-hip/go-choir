# MissionGradient: Mobile Real Desktop And Overview v0

**Status:** complete
**Date:** 2026-05-19
**Operator:** Codex supervising staging, product-path Playwright, git, CI, deploy, and owner review
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)
**Starting deployed baseline:** `cdf23b10823007cf54157d42087247ad1c121221`
**Completed platform behavior commit:** `79b14e2cf6057ee33154dd1d2700ae8cf26ce355`
**Proof harness follow-up commit:** `5820a88`

## One-Line Goal String

```text
/goal Run docs/mission-mobile-real-desktop-overview-v0.md as a Codex-operated MissionGradient mission: make mobile Choir the same real overlapping web desktop as desktop Choir, not a phone-mode adaptation. Remove mobile fullscreen-by-default behavior and snapping assumptions, preserve movable resizable overlapping windows with visible stack depth, make window raise/minimize/restore reliable, rename the bottom-bar concept toward a configurable Shelf, and build a Desktop Overview shell mode reachable from the Desk menu so users can see, focus, suspend, minimize, or close all open apps at once. Keep app content usable inside normal floating windows, preserve logged-out read/explore and auth-on-mutation, avoid fake overview thumbnails or local-only proof, and land platform changes through git/CI/deploy with staging identity, desktop and 390x844 Playwright screenshots/DOM metrics proving multi-window mobile multitasking, rollback refs, residual risks, and the next realism axis.
```

## Mission Frame

Choir should be a real web desktop on mobile, not a phone app with a desktop
skin. The power comes from true multitasking: overlapping movable windows,
visible stack depth, app switching, and the ability to keep VText, Trace,
Podcast, Files, and media apps open together.

The current shell is conceptually split. `FloatingWindow.svelte` supports
dragging and resizing on compact screens, but `stores/desktop.js` gives many
apps `compact.fullBleed` defaults and opens them nearly fullscreen. That makes
mobile feel like one app at a time, hides how many apps are open, encourages
app accumulation, and makes recovery problems harder to understand.

This mission corrects the product ontology: mobile is the same desktop model
with denser touch affordances and an overview mode. It does not introduce a
separate phone layout or a snapping window manager.

## Real Artifact

The artifact is the deployed Choir desktop shell geometry and navigation model:

```text
desktop surface
-> movable resizable overlapping windows
-> Shelf with Desk menu, prompt, status, and open-window strip
-> Desktop Overview shell mode for all open windows
-> app content remains usable inside normal floating windows
-> deployed Playwright evidence on desktop and 390x844 mobile
```

The artifact is not:

- a phone-mode rewrite;
- automatic fullscreen windows on mobile;
- window snapping as the primary mobile organization model;
- a decorative app launcher detached from window state;
- a fake overview that lists app labels without usable focus/close/minimize
  actions;
- a local-only screenshot claim.

## Invariants

- Mobile and desktop use one windowing ontology: overlapping windows with
  titlebars, focus, z-index, drag, resize, minimize, maximize, restore, and
  close.
- Mobile may tune defaults, target sizes, and density, but it must not replace
  the desktop with a one-window phone flow.
- New mobile windows should not open fullscreen by default. They should open
  large enough to be useful while leaving visible stack/desktop context.
- Maximize remains an explicit user action. It is not the default mobile state.
- Do not introduce automatic snap-to-edge behavior as the core organization
  primitive. Optional tiling commands can be future work, but not the default.
- Users must be able to see that multiple apps are open before opening more.
- The overview is a shell mode, not an app window. It represents the current
  desktop state and returns to the chosen focused window.
- App content must remain usable inside ordinary floating windows. Do not
  trade the shell fix for unusable PDF/EPUB/Trace/VText windows.
- Recovery and suspension are bounded: suspending background app bodies is
  allowed; silently discarding active state is not.
- Logged-out users keep read/explore usability, with auth only required at
  mutation or private-computer actions.
- Platform behavior changes require git, CI, deploy, staging identity, and
  deployed product-path proof.

## Value Criterion

Minimize:

```text
mobile fullscreen defaults
+ hidden open-window count
+ accidental app hoarding
+ unreliable window raising/restoring
+ app content clipped by shell chrome
+ separate mobile/desktop mental models
+ fake overview/list-only state
+ snapping-induced loss of freeform desktop control
+ local-only visual claims
+ recovery burden from invisible restored windows
```

subject to the invariants above.

The mission moves uphill when a user on a 390x844 phone can open several apps,
see that they are all part of one desktop, move and resize them, bring any one
forward, enter Desktop Overview, choose a window, and continue working without
the shell forcing a phone-style single-app flow.

## Quality Gradient

Target quality: **solid**, with excellent interaction taste for the core shell.

Solid means:

- the window geometry model is simple and shared across desktop/mobile;
- compact-window defaults are intentional and documented in code;
- visible window count and stack depth are obvious on mobile;
- Desktop Overview is keyboard/touch accessible and testable;
- opening, focusing, minimizing, restoring, and closing windows remain
  persistent across reload;
- app content remains usable inside floating windows;
- tests cover normal and compact viewports;
- docs and platform state ledger are updated when the shell state changes.

Substandard work:

- hiding the problem with fullscreen-by-default app windows;
- adding a second mobile-only shell;
- building an overview that cannot focus, close, minimize, or identify windows;
- relying on only one app in screenshots;
- making app content technically reachable but visually cramped or clipped;
- skipping deployed staging proof.

## Product Vocabulary

Use this vocabulary unless owner review changes it:

- **Shelf:** the configurable system bar currently implemented as
  `BottomBar.svelte`. It starts at the bottom, but should not remain
  conceptually bottom-only forever.
- **Desk:** the launcher/system button and menu. Do not call it "Start" in
  product copy. Keep legacy test selectors only if needed during migration.
- **Desk Menu:** the menu opened from Desk. It contains Apps, Windows,
  Computer, and account actions.
- **Desktop Overview:** the shell mode that shows all open windows at once.
- **Show Desktop:** secondary command that hides windows to reveal icons. It is
  not the main way to switch work.

## Homotopy Parameters

Increase realism continuously without changing the artifact:

- **Window density:** one window -> three overlapping windows -> six mixed
  heavy/light windows -> restored persisted desktop.
- **Viewport realism:** desktop `1280x900` -> compact `390x844` -> compact with
  browser chrome/safe-area pressure.
- **Interaction realism:** open/focus -> move/resize -> minimize/restore ->
  overview focus/close/suspend -> reload persistence.
- **App variety:** Files/VText/Trace -> media apps -> Compute Monitor and
  candidate surfaces.
- **Overview fidelity:** static cards -> DOM-derived thumbnails/cards ->
  lightweight live snapshots where safe.
- **Proof realism:** local Playwright -> deployed staging Playwright with
  screenshots and DOM metrics.

## Starting Belief State

Known from code and recent missions:

- `FloatingWindow.svelte` already supports pointer drag and bottom-right resize
  on compact screens.
- `stores/desktop.js` currently has compact-window behavior and many app
  preferences with `compact.fullBleed`, causing near-fullscreen mobile opens.
- `BottomBar.svelte` currently owns Desk/start button, prompt bar, window
  switcher, live status, and app launcher list.
- `Desktop.svelte` has recovery logic for heavy restored windows and app body
  suspension, but no first-class overview mode.
- Recent media missions improved app code paths and content immersion, but the
  shell still biases compact screens toward oversized windows.
- Compute Monitor now exposes user-computer scoped recovery data and can be a
  destination from overview/recovery surfaces.

Main uncertainties:

- Which app windows become unusable when no longer opened full-bleed on mobile.
- Whether thumbnail snapshots can be implemented cheaply and safely in V0, or
  whether the overview should start as spatial cards with title/app/state/action
  evidence.
- How much of `BottomBar.svelte` can be renamed to `Shelf` without excessive
  churn in one pass.
- Which existing tests assume fullscreen compact windows or "start" naming.

Highest-impact observation:

- A deployed 390x844 Playwright run showing four or more open windows with
  visible overlap, successful drag/resize/focus/minimize/restore, Desktop
  Overview focus selection, and no app content becoming unusable.

## Investigation And Cognitive Reframing

If the mission stalls, do not stop at "mobile is too small." Apply these route
changes before accepting a blocker:

- **Same-object transform:** ask how desktop behavior would work on mobile with
  better handles, rather than creating a separate mobile behavior.
- **Stack-visibility transform:** if a window feels too small, first tune
  initial geometry and overview affordances before returning to fullscreen.
- **Overview-before-snap transform:** if organization feels hard, improve the
  overview and window strip before adding snapping.
- **Content-pressure transform:** if an app becomes cramped, identify whether
  the shell, titlebar, Shelf, or app chrome is stealing space, then fix the
  specific layer.
- **Recovery-transform:** if many windows overload mobile, prefer lazy
  hydration, suspension, overview-level cleanup, and Compute Monitor entry over
  hiding windows by default.

A blocker is tactical when a failing app, selector, geometry rule, or test can
be isolated and patched. Run the next safe probe instead of ending.

A blocker is invariant-level when the requested behavior would expose private
state, discard active user state, require unapproved platform authority, or
force a separate mobile ontology. Escalate with evidence.

## Receding-Horizon Control

Operate in short intervals:

1. Select the next shell behavior that most reduces fullscreen/mobile drift.
2. Predict the observable change in DOM geometry and screenshots.
3. Patch only the implicated shell/app boundary.
4. Run focused local checks.
5. Inspect screenshots and DOM metrics.
6. Update belief state.
7. Continue, narrow, or land when the deployed proof is strong.

Prefer shell primitives before app-specific patches. Patch an app only when the
shell now preserves the desktop ontology and the app fails inside that ontology.

## Implementation Direction

### P0: Remove Fullscreen Defaults

- Replace `compact.fullBleed` default openings with mobile desktop geometry:
  large but visibly windowed.
- Preserve explicit maximize and restore.
- Keep window constraints inside viewport and above the Shelf.
- Ensure new compact windows cascade with visible offsets and titlebars.
- Keep app-specific minimums where content requires it, but do not let minimums
  silently force fullscreen.

### P1: Make Mobile Window Controls Reliable

- Make titlebar drag reliable on touch.
- Make resize handle large enough to use but visually quiet.
- Verify focus/raise works by tapping any visible window region.
- Verify minimized indicators always restore and raise the selected window.
- Add a window action affordance if small controls are hard to hit on mobile.

### P2: Shelf And Desk Menu

- Rename the product concept from bottom bar/start menu toward Shelf/Desk.
- Keep compatibility selectors only where needed for tests during the cutover.
- Make the open-window strip show enough window state to prevent app hoarding.
- Add a first-class Desktop Overview command in the Desk Menu.
- Keep prompt growth bounded and shrinking when empty.

### P3: Desktop Overview Shell Mode

Build a shell mode, not an app:

- show all open non-closed windows;
- preserve enough spatial position/stack information to communicate desktop
  state;
- normalize cards/thumbnails so every window is tappable on 390x844;
- tap/click a window to focus it and exit overview;
- provide close/minimize/suspend actions where safe;
- show suspended/heavy/restored status clearly;
- include contextual actions: Suspend background apps, Clear saved windows,
  Open Compute Monitor when relevant;
- escape/click backdrop exits without mutating window state.

V0 can use DOM-derived cards with app icon, title, mode, and scaled geometry if
live thumbnails are too costly. Do not fake thumbnails that imply content was
captured when it was not.

### P4: App Usability Regression Pass

Run the shell against representative apps:

- VText and Trace open together;
- Files opens media/readers without losing window context;
- PDF/EPUB remain readable in non-fullscreen compact windows;
- Podcast remains usable as a reference app;
- Compute Monitor can recover/suspend heavy windows from the shell.

Only patch app layouts where the shell invariants expose real app bugs.

## Dense Feedback Channels

Use feedback that measures the desktop model directly:

- DOM metrics:
  - window count;
  - visible window count;
  - active window id;
  - z-index order;
  - bounding boxes;
  - visible overlap between windows;
  - max window area divided by desktop area;
  - Shelf height;
  - overview card count and action visibility.
- Screenshots:
  - desktop viewport with 4+ overlapping windows;
  - 390x844 with 4+ overlapping windows;
  - Desktop Overview on both viewports;
  - focus selection after overview;
  - minimized/restore behavior.
- Tests:
  - existing floating-window and desktop-state persistence coverage;
  - new mobile real-desktop overview proof;
  - app smoke coverage for VText, Trace, Files, PDF/EPUB, Podcast, Compute
    Monitor where feasible.
- Staging checks:
  - `/health` reports pushed SHA;
  - deployed Playwright proof runs against `https://draft.choir-ip.com`.

## Evidence Ledger

For every nontrivial claim, record:

```text
claim
evidence source
command or observation
artifact path
result
uncertainty/caveat
promotion relevance
```

Required final evidence:

- pushed commit SHA;
- GitHub Actions run URL and conclusion;
- staging deploy identity;
- desktop and 390x844 screenshots;
- DOM metric table for multi-window geometry and overview;
- tests run locally and on staging;
- rollback refs;
- residual risks;
- next realism axis.

## Forbidden Shortcuts

- Do not make mobile a separate phone app shell.
- Do not hide mobile complexity by opening every app fullscreen.
- Do not introduce automatic snapping as the default answer.
- Do not remove overlapping windows or resize controls on compact screens.
- Do not claim overview success from a static list that cannot focus windows.
- Do not use fake screenshots/thumbnails as evidence.
- Do not make Desktop Overview an ordinary app window.
- Do not silently discard saved windows to reduce clutter.
- Do not rely on internal/test-only routes for product proof.
- Do not claim platform behavior from local-only checks.
- Do not degrade logged-out read/explore usability.

## Rollback Policy

- Git rollback: revert the shell mission commits.
- Deploy rollback: previous staging SHA is `cdf23b10823007cf54157d42087247ad1c121221`.
- State rollback: desktop window persistence must remain compatible enough that
  old geometry can be loaded or safely constrained. If state migration is
  required, document exact rollback behavior.
- UX rollback: if Desktop Overview breaks focus or restore, keep a path to
  open Compute Monitor and clear saved windows.
- Test rollback: preserve or update old selectors intentionally; do not leave
  ambiguous parallel test paths.

## Learning Side-Channel

Classify surprises:

- Tactical: app minimum dimensions, touch targets, selector changes, geometry
  thresholds. Patch directly.
- Target-level: naming such as Shelf/Desk, overview thumbnail fidelity, or
  whether overview needs grouped spaces. Update this mission doc or propose a
  reparameterization.
- Invariant-level: any pressure to make mobile a separate ontology, drop
  overlapping windows, expose private app state in overview, or discard active
  state silently. Stop and escalate.

Update [platform-os-app-state.md](platform-os-app-state.md) if the shell,
launcher, Shelf, overview, or mobile desktop state changes.

## Stopping Condition

Stop only when one of these is true:

- Success: staging proves the same real desktop model on mobile and desktop,
  with overlapping movable/resizable windows, visible open-window state,
  working Desktop Overview, app usability smoke proof, rollback refs, and
  residual risks.
- Hard blocker: after root-cause probes and cognitive reframing, a named
  invariant-level or external blocker remains and no safe executable probe
  exists inside current authority.

Do not stop because the first compact geometry feels awkward. Tune it, measure
it, and keep the artifact identity intact.

## Next Realism Axis

The next realism axis is:

- make Desktop Overview more spatial and useful under heavy real-user sessions,
  including better app suspension policy, window memory/recovery controls, and
  optional live preview thumbnails once the memory cost is bounded.

Successor mission: [mission-desktop-overview-heavy-session-v0.md](mission-desktop-overview-heavy-session-v0.md).

## Run Checkpoint & Resumption State

**Status:** `complete`

**Last checkpoint:** mobile is now the same real overlapping web desktop model as
desktop, with non-fullscreen default compact windows and Desktop Overview
available from the Desk menu.

**Current artifact state:**

- deployed staging identity for product behavior:
  `79b14e2cf6057ee33154dd1d2700ae8cf26ce355`;
- follow-up committed test selector correction: `5820a88`;
- compact windows open large but visibly windowed, cascade, overlap, move,
  resize, minimize, restore, and focus;
- Shelf/Desk product vocabulary is introduced in UI copy while legacy selectors
  remain for compatibility;
- Desktop Overview exists as a shell mode with DOM-derived spatial cards and
  actions to focus, minimize, suspend, close, suspend background apps, open
  Compute Monitor, and clear saved windows where allowed.

**What shipped:**

- compact `fullBleed` mobile defaults were removed from desktop app registry
  behavior;
- compact window geometry and cascade defaults were tuned for visible stack
  depth on `390x844`;
- `FloatingWindow.svelte` exposes window mode/active state and has improved
  compact resize affordance;
- `BottomBar.svelte` now presents Desk/Shelf vocabulary and a Desktop Overview
  command;
- `DesktopOverview.svelte` was added as a shell overlay;
- deployed Playwright proof was added for mobile and desktop real-desktop
  behavior.

**What was proven:**

- GitHub Actions run `26125883507` completed successfully and deployed staging.
- Staging `/health` reported proxy and upstream commit
  `79b14e2cf6057ee33154dd1d2700ae8cf26ce355`.
- Deployed Playwright command:

```bash
PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com GO_CHOIR_DESKTOP_BOOT_TIMEOUT_MS=300000 npx playwright test tests/mobile-real-desktop-overview.spec.js --project=chromium --workers=1 --timeout=360000 --reporter=line
```

- Result: `2 passed`.
- Mobile `390x844` proof opened Files, VText, Trace, and Podcast as four
  overlapping non-fullscreen windows, verified drag, resize, minimize, restore,
  Desktop Overview focus, and background suspension controls.
- Mobile DOM metrics included max window area ratio `0.624`, `6` overlap pairs,
  and `4` visible windows.
- Desktop `1280x900` proof included max window area ratio `0.614`, `6` overlap
  pairs, and `4` visible windows.

**Unproven or partial claims:**

- Desktop Overview is not yet proven under heavy real-user restore sessions
  with many saved windows, mixed app weights, and long-lived browser memory
  pressure.
- Overview uses DOM-derived spatial cards, not live thumbnails.
- App suspension policy is still coarse: it knows heavy app bodies, but does
  not yet use richer app-owned process/memory accounting.
- Shelf placement and desktop style families remain future work.

**Belief-state changes:**

- The mobile/desktop ontology can remain unified on a phone-sized viewport.
- The immediate risk is no longer "mobile forces phone mode"; the next risk is
  whether real users with many restored windows can understand, triage, suspend,
  and recover their desktop without invisible memory pressure or app hoarding.

**Remaining error field:**

- overview spatial fidelity under many windows;
- bounded preview fidelity without fake thumbnails or unbounded memory cost;
- richer suspend/unload/recovery policy;
- long-session mobile restore proof;
- better integration between Desktop Overview and Compute Monitor.

**Highest-impact remaining uncertainty:**

Whether Desktop Overview can become the user's primary heavy-session recovery
and multitasking surface without hydrating too many expensive app bodies or
exposing private content in unsafe previews.

**Next executable probe:**

Run the successor mission against a staged heavy-session desktop: create or
restore 10-20 mixed windows on desktop and `390x844`, measure DOM/app weight,
prove overview focus/suspend/close/recover flows, then refine overview layout,
suspension policy, and recovery controls.

**Suggested resume goal string:**

```text
/goal Run docs/mission-desktop-overview-heavy-session-v0.md as a Codex-operated MissionGradient mission: make Desktop Overview the heavy-session control surface for Choir's real web desktop. Preserve overlapping movable windows on mobile and desktop, then make Overview spatially useful with many real windows, bounded suspension/recovery actions, user-computer-scoped memory/restore evidence, and optional live previews only when their privacy and memory cost are proven. Do not fake thumbnails, discard active state silently, expose host/global system information, use phone-mode simplification, or claim local-only proof. Land platform changes through git/CI/deploy and prove on staging with desktop and 390x844 Playwright screenshots/DOM metrics under a heavy restored session, rollback refs, residual risks, and the next realism axis. If the stopping condition is not reached, do not call the mission complete; report checkpoint_incomplete or blocked_incomplete with a resumable mission-doc checkpoint and continue/redirect/delegate any safe executable next probe inside current authority before stopping.
```

**Evidence artifact refs:**

- `/Users/wiz/go-choir/frontend/test-results/mobile-real-desktop-overvi-a8af8-th-Desktop-Overview-actions-chromium/mobile-overlapping-windows.png`
- `/Users/wiz/go-choir/frontend/test-results/mobile-real-desktop-overvi-a8af8-th-Desktop-Overview-actions-chromium/mobile-desktop-overview.png`
- `/Users/wiz/go-choir/frontend/test-results/mobile-real-desktop-overvi-7c5c2-l-model-on-desktop-viewport-chromium/desktop-overlapping-windows.png`
- `/Users/wiz/go-choir/frontend/test-results/mobile-real-desktop-overvi-7c5c2-l-model-on-desktop-viewport-chromium/desktop-overview.png`

**Rollback refs:**

```bash
git revert 79b14e2 5820a88
```
