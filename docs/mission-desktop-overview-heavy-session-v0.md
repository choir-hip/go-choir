# MissionGradient: Desktop Overview Heavy Session v0

**Status:** complete
**Date:** 2026-05-19
**Operator:** Codex supervising staging, product-path Playwright, git, CI, deploy, and owner review
**Predecessor:** [mission-mobile-real-desktop-overview-v0.md](mission-mobile-real-desktop-overview-v0.md)
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)
**Starting deployed behavior baseline:** `79b14e2cf6057ee33154dd1d2700ae8cf26ce355`
**Latest main test-harness commit:** `5820a88`
**Completed platform behavior commit:** `b148461dafc6125fa321de9b10814cdc6af285b6`

## One-Line Goal String

```text
/goal Run docs/mission-desktop-overview-heavy-session-v0.md as a Codex-operated MissionGradient mission: make Desktop Overview the heavy-session control surface for Choir's real web desktop. Preserve overlapping movable windows on mobile and desktop, then make Overview spatially useful with many real windows, bounded suspension/recovery actions, user-computer-scoped memory/restore evidence, and optional live previews only when their privacy and memory cost are proven. Do not fake thumbnails, discard active state silently, expose host/global system information, use phone-mode simplification, or claim local-only proof. Land platform changes through git/CI/deploy and prove on staging with desktop and 390x844 Playwright screenshots/DOM metrics under a heavy restored session, rollback refs, residual risks, and the next realism axis. If the stopping condition is not reached, do not call the mission complete; report checkpoint_incomplete or blocked_incomplete with a resumable mission-doc checkpoint and continue/redirect/delegate any safe executable next probe inside current authority before stopping.
```

## Mission Frame

The previous mission proved that mobile Choir can keep the same overlapping web
desktop ontology as desktop Choir. That is the right foundation, but a real user
session will not always contain four tidy windows. It may contain many restored
apps, mixed heavy/light surfaces, VText and Trace live at once, media readers,
Compute Monitor, candidate/promotion surfaces, and stale windows from prior
work.

Desktop Overview should become the user's heavy-session control surface: the
place to understand the current desktop, select the next window, suspend or
close expensive background work, recover from a bad restore, and avoid opening
more apps just because the existing stack is hidden.

This mission does not replace the real desktop with a phone mode or a task
switcher. It refines the same desktop object under more realistic session
pressure.

## Real Artifact

The artifact is the deployed shell control system for many-window sessions:

```text
persistent desktop state
-> many overlapping floating windows
-> app body hydration/suspension policy
-> Desktop Overview spatial management surface
-> Compute Monitor / recovery affordance handoff
-> staged heavy-session product proof on desktop and 390x844 mobile
```

The artifact is not:

- a phone-mode recast of mobile;
- a decorative app switcher detached from real window state;
- fake overview thumbnails;
- a broad kill switch;
- host/global system telemetry in user-facing UI;
- a local-only proof of staging behavior.

## Invariants

- Mobile and desktop remain one overlapping-window ontology: move, resize,
  focus, z-index, minimize, maximize, restore, close.
- Desktop Overview remains a shell mode, not an app window.
- Overview actions operate on real window state and app body state. They must
  not mutate hidden state through fake records.
- Suspending an app body may unload expensive UI/runtime work, but must preserve
  active computer state, app identity, window geometry, and user intent.
- Closing a window must be an explicit user action or a clearly scoped recovery
  action; do not silently discard active state to make memory metrics look good.
- Compute/resource information shown to users must be scoped to their current
  computer, candidate computers, app/window restore weight, and product health.
  Do not expose host-wide memory, global vmctl inventory, raw VM handles, or
  platform internals.
- Live previews are optional and must be bounded by privacy and memory policy.
  A card may be spatial and useful without implying a screenshot was captured.
- Logged-out users keep read/explore usability; private-computer recovery,
  suspension, and mutation actions require auth.
- Platform behavior changes require git, CI, deploy, staging identity, and
  deployed product-path proof.

## Value Criterion

Minimize:

```text
hidden many-window state
+ browser memory pressure from eager restored app hydration
+ user confusion about which apps are open or expensive
+ accidental app hoarding
+ recovery flows that require memorized query params
+ Overview cards that are non-spatial or non-actionable
+ fake preview fidelity
+ unsafe discard/kill controls
+ host/global system info leakage
+ local-only claims about staging shell behavior
```

subject to the invariants above.

The mission moves uphill when a real user can return to a crowded desktop,
open Desktop Overview, understand the spatial stack, focus the right window,
suspend or close safe background windows, reach Compute Monitor when deeper
recovery is needed, and keep working on mobile or desktop without losing state.

## Quality Gradient

Target quality: **solid**, with excellent care around safety and interaction
semantics.

Solid means:

- heavy-session state is measured before and after changes;
- Overview scales from 4 windows to at least 12 mixed app windows;
- cards communicate position, stack, app identity, active/minimized/suspended
  state, and safe actions;
- suspend/recover actions are bounded and reversible where possible;
- mobile `390x844` remains a real desktop, not a full-screen task carousel;
- screenshots and DOM metrics prove behavior on staging;
- tests distinguish `complete` from `checkpoint_incomplete`.

Substandard work:

- adding pretty cards that cannot focus or recover windows;
- using fake thumbnails that imply captured content;
- measuring host RAM or global VM inventory in a user-facing monitor;
- deleting/restoring desktop state without user intent;
- making mobile a one-window or snap-only flow;
- calling a useful partial checkpoint "complete."

## Product Vocabulary

- **Shelf:** configurable shell bar currently implemented by `BottomBar.svelte`.
- **Desk:** launcher/system menu entrypoint.
- **Desktop Overview:** shell mode for seeing and managing open windows.
- **Suspended app body:** a window that keeps identity/geometry/state but does
  not mount its expensive app UI until focused or explicitly resumed.
- **Restore weight:** product-level measure of how much desktop/app state will
  hydrate on boot or reload. This is not host RAM.
- **Recovery action:** explicit bounded action that helps the user regain a
  usable desktop without broad destructive control.

## Homotopy Parameters

Increase realism continuously:

- **Window count:** 4 windows -> 8 windows -> 12+ windows -> restored persisted
  heavy session.
- **App mix:** Files/VText/Trace/Podcast -> media apps -> Compute Monitor ->
  candidate/promotion surfaces.
- **Hydration pressure:** all bodies mounted -> heavy bodies suspended when
  background -> restore only active/top bodies -> user-configurable policy.
- **Overview fidelity:** DOM-derived cards -> spatial mini-map -> bounded
  content previews where safe -> live previews only after memory/privacy proof.
- **Recovery strength:** focus/minimize/close -> suspend background -> restore
  top window only -> open Compute Monitor -> candidate discard/hibernate where
  product APIs support it.
- **Viewport realism:** desktop `1280x900` -> mobile `390x844` -> mobile with
  browser chrome/safe-area pressure and many windows.
- **Proof realism:** local targeted proof -> deployed staging heavy-session
  proof -> returning-user restored-session proof.

## Starting Belief State

Known from the predecessor mission:

- Mobile can sustain the same overlapping desktop model as desktop.
- `390x844` deployed proof passed with four overlapping windows.
- Desktop Overview exists and can focus, minimize, close, suspend a window, and
  suspend background apps.
- Overview v0 uses DOM-derived spatial cards, not live thumbnails.
- Heavy restored app bodies can be suspended in some recovery paths, and
  Compute Monitor is the product surface for scoped user-computer recovery.

Main uncertainties:

- How Overview behaves with 10-20 windows on mobile and desktop.
- Whether current suspension policy prevents expensive background hydration in
  restored sessions, not just manually opened sessions.
- Which apps are genuinely heavy in product terms: Terminal, Trace, media,
  VText, PDF/EPUB, candidate desktop, or others.
- Whether live thumbnails are worth their memory/privacy cost.
- How much recovery should happen in Overview versus Compute Monitor.

Highest-impact observation:

- A deployed staging run with a heavy restored desktop on `390x844` showing
  Overview remains readable and actionable, app bodies can be suspended or
  resumed without losing window state, and the desktop avoids crash/reload loops
  from eager hydration.

## Investigation And Cognitive Reframing

If the mission stalls, do not stop at "many windows are hard on mobile" or
"thumbnails are expensive." Apply route-changing transforms:

- **Control-surface transform:** Overview is not decoration; it is the control
  surface for a high-dimensional desktop state.
- **Spatial-before-preview transform:** first make position, stack, state, and
  action clear. Add live previews only if the cheaper spatial model fails a
  real task.
- **Policy-before-kill transform:** prefer hydration policy, suspension, and
  explicit recovery over broad kill/clear controls.
- **User-scope transform:** translate resource questions into user-computer and
  app/window restore facts, not host/platform internals.
- **Return-session transform:** a clean newly registered test is not enough;
  prove a returning or restored heavy session.

Tactical blockers should trigger another probe or patch: selector failures,
layout overflow, bad card density, inaccurate restore weight, app misclassified
as heavy, or missing product API fields.

Invariant-level blockers require escalation: private content leaks through
preview capture, user state discard without consent, host/global telemetry
exposure, or a requirement to abandon overlapping windows on mobile.

## Receding-Horizon Control

Operate in bounded intervals:

1. Create or restore a heavier desktop session.
2. Measure window/app state and current failure surface.
3. Patch the smallest shell/recovery/policy layer that reduces the error.
4. Re-run local focused checks.
5. Deploy when platform behavior changes.
6. Run staging proof with screenshots and DOM metrics.
7. Update belief state and either continue, narrow, rollback, or checkpoint.

Prefer improving Overview and hydration policy before app-specific layout work,
unless a single app is clearly breaking the shared shell invariant.

## Implementation Direction

### P0: Heavy-Session Measurement

- Add or extend a Playwright proof that opens/restores at least 12 windows across
  mixed app types on desktop and `390x844`.
- Capture DOM metrics: window count, visible count, active id, z-order, area
  ratios, overlap pairs, minimized count, suspended count, heavy app count,
  mounted app body count, and Overview card/action count.
- If feasible, include a returning/reload step so the proof exercises persisted
  desktop restore, not only newly opened windows.

### P1: Spatial Overview Density

- Make Overview useful with many windows:
  - preserve spatial position and stack depth;
  - avoid cards becoming unreadable on `390x844`;
  - show active, minimized, suspended, and heavy states;
  - offer fast focus with no extra confirm;
  - keep destructive actions visually secondary;
  - support keyboard Escape and touch-friendly controls.
- Consider grouped sections only if they preserve spatial truth: active stack,
  minimized, suspended, heavy/background, or app family.

### P2: Suspension And Hydration Policy

- Define app body lifecycle states clearly: mounted, suspended, minimized,
  restored-suspended, and failed/needs-reload if applicable.
- Ensure background heavy apps can stay suspended across restore until focused.
- Make focus/resume explicit and reliable.
- Add product-visible explanations for why a window is suspended.
- Do not discard app state merely to reduce restore weight.

### P3: Window Memory And Recovery Controls

- Integrate Overview with Compute Monitor without exposing platform internals.
- Add Overview-level controls for bounded recovery:
  - suspend background heavy apps;
  - keep active/top windows mounted;
  - close selected windows;
  - clear saved desktop state only with explicit authenticated intent;
  - open Compute Monitor for deeper computer/candidate recovery.
- Ensure the recovery path is reachable when a restored session is heavy, not
  only after a successful full desktop hydration.

### P4: Bounded Preview Experiment

- Start with spatial DOM-derived cards.
- If live previews are attempted, first define:
  - privacy policy for preview capture;
  - memory budget;
  - invalidation/update cadence;
  - fallback when capture fails;
  - proof that previews are real captures, not fake placeholders.
- It is acceptable to complete this mission without live previews if spatial
  cards plus state/action evidence satisfy the user task under heavy load.

### P5: Proof And Documentation

- Update [platform-os-app-state.md](platform-os-app-state.md) when platform
  shell behavior changes.
- Run local frontend build and focused tests.
- Push to `origin/main`, monitor CI/deploy, verify staging identity.
- Run deployed Playwright proof on desktop and `390x844`.
- Record complete/checkpoint status honestly in this file.

## Dense Feedback Channels

Use feedback that exposes the heavy-session error field:

- DOM metrics:
  - total/open/visible/minimized/suspended windows;
  - mounted app bodies;
  - heavy app count and restore weight;
  - z-index order and overlap pairs;
  - Overview card count, card density, and action visibility;
  - focus result after selecting cards;
  - suspend/resume effect on mounted bodies.
- Screenshots:
  - desktop with 12+ windows;
  - `390x844` with 12+ windows;
  - Desktop Overview under load;
  - Overview after background suspension;
  - focused window after Overview selection;
  - Compute Monitor handoff if used.
- Product checks:
  - staging `/health` reports pushed SHA;
  - authenticated product API evidence for scoped compute/recovery state where
    relevant;
  - no browser-public internal/test-only routes.

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
- desktop and `390x844` screenshots under heavy session;
- DOM metric table before and after Overview/recovery actions;
- local and deployed acceptance commands;
- rollback refs;
- residual risks;
- next realism axis.

## Run Checkpoint & Resumption State

Use this section during execution. A checkpoint is not completion.

**Status:** `complete`

**Last checkpoint:** platform behavior commit
`b148461dafc6125fa321de9b10814cdc6af285b6` landed and deployed to staging with
heavy-session Desktop Overview proof on desktop and `390x844` mobile.

**Current artifact state:** Desktop Overview now reports heavy-session pressure
from real window/app-body state, remains spatial with dense 12-window maps and
cards, exposes mounted/heavy/suspended/minimized/active state, and offers
bounded recovery actions: suspend background apps, focus/resume a suspended
window, open Compute Monitor, keep active window only, and clear saved windows
behind auth.

**What shipped:**

- `DesktopOverview.svelte` gained mounted-heavy, minimized, pressure, active
  window, dense map, state badges, and safe recovery action metrics;
- `Desktop.svelte` wires Overview's authenticated "Keep active only" action to
  the existing state-preserving saved-window reducer;
- `desktop-overview-heavy-session.spec.js` opens 12 real app windows through the
  Desk, persists/reloads them, handles restore recovery, asserts suspension and
  Overview metrics, checks Compute Monitor handoff, and verifies keep-active
  recovery;
- MissionGradient checkpoint semantics were updated in the repo skill copy.

**What was proven:**

- local checks:
  - `npm run build`;
  - `npx playwright test tests/desktop-overview-heavy-session.spec.js --list`;
  - local full product run was blocked by a missing local Dolt runtime store
    under `/tmp/go-choir-m3/...`, so local proof was not used for staging
    claims;
- GitHub Actions run
  `https://github.com/yusefmosiah/go-choir/actions/runs/26131606449` completed
  successfully;
- staging `/health` reported proxy and sandbox commit
  `b148461dafc6125fa321de9b10814cdc6af285b6`;
- deployed heavy-session command:

```bash
PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com GO_CHOIR_DESKTOP_BOOT_TIMEOUT_MS=300000 npx playwright test tests/desktop-overview-heavy-session.spec.js --project=chromium --workers=1 --timeout=360000 --reporter=line
```

- result: `2 passed`;
- deployed regression command:

```bash
PLAYWRIGHT_BASE_URL=https://draft.choir-ip.com GO_CHOIR_DESKTOP_BOOT_TIMEOUT_MS=300000 npx playwright test tests/mobile-real-desktop-overview.spec.js --project=chromium --workers=1 --timeout=360000 --reporter=line
```

- result: `2 passed`;
- mobile `390x844` heavy proof restored 12 visible overlapping windows, 11
  heavy windows, 10 suspended windows, 1 mounted heavy body, 66 overlap pairs,
  12 Overview cards, 12 Overview map windows, and pressure `elevated`;
- desktop `1280x900` heavy proof restored the same counts and verified the same
  Overview action path;
- Compute Monitor handoff opened a user-scoped recovery app, and the
  keep-active action reduced saved/open windows to one without broad kill or
  hidden state discard.

**Unproven or partial claims:**

- no live thumbnails were attempted. Spatial/state Overview was sufficient for
  this proof and avoids privacy/memory risk;
- suspension remains app-body level, not app-owned process/resource accounting;
- this proof uses generated test users, not a long-lived account with days of
  accumulated real windows;
- Overview has not yet gained keyboard navigation beyond Escape or richer
  accessibility semantics.

**Belief-state changes:**

- The heavy-session problem is now mostly a policy and information-design
  problem, not a mobile-ontology problem. The same overlapping desktop can carry
  12 restored windows on mobile when heavy background app bodies remain
  suspended.
- Live previews are not required for the next step; state density and bounded
  recovery actions are already useful and cheaper.

**Remaining error field:** app-owned resource accounting, long-lived real-user
session replay, keyboard/accessibility depth, and future bounded preview
feasibility.

**Highest-impact remaining uncertainty:** whether real user accounts with
heterogeneous app-owned state, active media, browser snapshots, Trace runs, and
candidate windows produce the same bounded restore behavior as the generated
12-window proof.

**Next executable probe:** build app-owned resource/restore metadata and replay
a real returning-account desktop with heterogeneous app state, then decide
whether Overview needs grouping, keyboard navigation, or bounded live previews.

**Suggested resume goal string:**

```text
/goal Run docs/mission-overview-real-user-resource-accounting-v0.md as a Codex-operated MissionGradient mission: make Desktop Overview and Compute Monitor understand app-owned restore/resource weight under real returning-account sessions. Preserve the overlapping desktop ontology, use product-path staging proof, add app-owned resource metadata where safe, improve keyboard/accessibility navigation, and only explore live previews after privacy and memory budgets are explicit. Stop only with deployed proof or an honest checkpoint_incomplete/blocker state.
```

**Evidence artifact refs:**

- `/Users/wiz/go-choir/frontend/test-results/desktop-overview-heavy-ses-99cd8-red-12-window-heavy-session-chromium/mobile-heavy-session-restored-heavy-windows.png`
- `/Users/wiz/go-choir/frontend/test-results/desktop-overview-heavy-ses-99cd8-red-12-window-heavy-session-chromium/restore-recovery-gate.png`
- `/Users/wiz/go-choir/frontend/test-results/desktop-overview-heavy-ses-99cd8-red-12-window-heavy-session-chromium/mobile-heavy-session-heavy-overview.png`
- `/Users/wiz/go-choir/frontend/test-results/desktop-overview-heavy-ses-99cd8-red-12-window-heavy-session-chromium/mobile-heavy-session-compute-monitor-handoff.png`
- `/Users/wiz/go-choir/frontend/test-results/desktop-overview-heavy-ses-ea3d1-red-12-window-heavy-session-chromium/desktop-heavy-session-restored-heavy-windows.png`
- `/Users/wiz/go-choir/frontend/test-results/desktop-overview-heavy-ses-ea3d1-red-12-window-heavy-session-chromium/desktop-heavy-session-heavy-overview.png`
- `/Users/wiz/go-choir/frontend/test-results/desktop-overview-heavy-ses-ea3d1-red-12-window-heavy-session-chromium/desktop-heavy-session-compute-monitor-handoff.png`

**Rollback refs:**

```bash
git revert b148461
```

## Forbidden Shortcuts

- Do not make mobile a phone-mode task carousel.
- Do not use fake thumbnails or static placeholders as preview proof.
- Do not expose host/global memory, global VM inventory, raw VM handles, or
  platform internals in user-facing UI.
- Do not add a broad kill switch.
- Do not silently clear, close, or discard active user state.
- Do not make Overview an app window.
- Do not use internal/test-only routes for acceptance proof.
- Do not claim local-only proof for staging behavior.
- Do not call a useful checkpoint complete.

## Rollback Policy

- Git rollback: revert behavior commits from this mission.
- Deploy rollback: previous deployed behavior baseline is
  `79b14e2cf6057ee33154dd1d2700ae8cf26ce355`.
- State rollback: desktop persistence changes must remain compatible or include
  explicit migration/rollback behavior.
- Recovery rollback: if Overview actions regress restore, Compute Monitor and
  existing recovery paths must remain reachable.
- Preview rollback: any live preview experiment must be removable without
  changing window identity, geometry, or app state.

## Learning Side-Channel

Write tactical learnings into this mission document, focused tests, or Trace.

Promote learnings to [platform-os-app-state.md](platform-os-app-state.md),
[computer-ontology.md](computer-ontology.md), or
[runtime-invariants.md](runtime-invariants.md) only when they change current
platform state, product ontology, authority boundaries, or recovery invariants.

Classify learnings:

- Tactical: patch and continue.
- Target-level: update this mission and continue under a better
  parameterization.
- Invariant-level: stop and escalate before changing the product ontology or
  recovery authority model.

## Stopping Condition

`complete` requires deployed staging proof that:

- desktop and mobile both preserve overlapping windows under a heavy session;
- Desktop Overview remains spatially readable and actionable with at least 12
  mixed app windows;
- focus, minimize, close, suspend, and resume/recover paths work from Overview;
- mounted/suspended/heavy window metrics show that recovery actions reduce
  restore pressure without silently discarding active state;
- Compute Monitor handoff works where deeper recovery is needed;
- no fake thumbnails, host telemetry leaks, phone-mode simplification, or
  internal/test-only proof paths were used;
- rollback refs, residual risks, and next realism axis are named.

`checkpoint_incomplete` is allowed only when useful progress landed but the
above stopping condition is not satisfied, and continuing would exceed an
authorized boundary, require human/operator authority, become unsafe, wait on
external systems with no parallel work, or repeat already-falsified probes
without new evidence. The mission document must be updated with the real
frontier and the next executable probe.

`blocked_incomplete` requires named root-cause probes, cognitive reframing, exact
evidence, rollback state, and the smallest safe next probe or external authority
needed.

Do not call the mission complete merely because Overview looks better in one
fresh four-window session.

## Next Realism Axis

Likely successor axes after completion:

- user-configurable Shelf placement and desktop style families;
- Overview keyboard navigation and accessibility depth;
- app-owned process/resource accounting;
- candidate/promotion cards appearing contextually inside Overview/Trace;
- bounded live preview thumbnails if not completed here.
