# MissionGradient: Desktop Overview Live Spatial Previews v0

**Status:** proposed
**Date:** 2026-05-19
**Operator:** Codex or Choir-in-Choir supervisor with staging Playwright, git,
CI, deploy, and product-path evidence
**Predecessor:** [mission-desktop-overview-heavy-session-v0.md](mission-desktop-overview-heavy-session-v0.md)
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)
**Starting platform doc head:** `824c793d95bd0d3863b0761aed8d155f22ad684f`
**Starting deployed behavior baseline:** `b148461dafc6125fa321de9b10814cdc6af285b6`

## One-Line Goal String

```text
/goal Run docs/mission-desktop-overview-live-spatial-previews-v0.md as a Codex-operated MissionGradient mission: make Desktop Overview feel like a premium live spatial control surface for Choir's real overlapping web desktop. Build bounded live previews by animating real window DOM into Overview positions where safe, using app-owned lightweight/redacted preview cards for suspended, heavy, private, or unsafe windows. Do not make WebGPU, canvas screenshots, fake thumbnails, duplicated app mounts, persisted preview captures, phone-mode simplification, host/global telemetry, or local-only proof the acceptance path. Preserve heavy-session recovery, app suspension, privacy, and active computer state. Land platform changes through git/CI/deploy and prove on staging with desktop and 390x844 Playwright screenshots/DOM metrics under ordinary and heavy restored sessions, including memory/restore-weight evidence, rollback refs, residual risks, and the next realism axis. If the stopping condition is not reached, report checkpoint_incomplete or blocked_incomplete, update this mission doc with a resumable checkpoint, and continue/redirect/delegate any safe executable next probe inside current authority before stopping.
```

## Mission Frame

The previous Desktop Overview missions established the right ontology: Choir on
mobile is still a real floating-window desktop, and Overview can manage a
12-window restored heavy session with bounded suspension/recovery controls.

The next realism axis is quality of perception. A crowded desktop should not
feel like a list of app records. It should feel like the current room has been
arranged for the user: windows retain spatial truth, the active task is clear,
background work is legible, expensive apps are honestly suspended, and recovery
actions are close without turning the interface into an admin dashboard.

The desired effect is live, tasteful, and hospitable, but not reckless. The
mission should earn live previews by preserving the crash and privacy lessons
from the heavy-session mission.

## Design Research Summary

The default implementation should not be WebGPU. WebGPU is a good future tool
for shader-heavy visual effects, custom 3D scenes, or GPU compute, but the core
Overview problem is DOM state, memory, privacy, recovery, and spatial shell
control. MDN describes WebGPU as access to GPU adapters/devices and render or
compute pipelines, which is a different layer than arranging existing DOM
windows.

The preferred v0 path is:

```text
real window DOM
-> measured current rectangles
-> bounded Overview target rectangles
-> CSS transform/opacity animation
-> focus or close reverses/resolves the transform
```

This gives live previews without screenshot capture, without duplicating app
mounts, and without storing thumbnail pixels.

View Transitions may be used as progressive enhancement for specific transitions
if they help, but they should not be the core dependency. The API is designed
around snapshots between DOM states, which is useful for polish but not enough
to govern many live overlapping windows with suspension policy.

Canvas/DOM screenshot approaches such as html2canvas should not be the primary
path. They can fail on CSS coverage, canvas size, cross-origin media, and
privacy constraints. Cross-origin image/video data drawn into canvas can taint
the canvas and block reading/exporting pixel data, which is the opposite of a
durable product proof surface.

Reference sources:

- [MDN WebGPU API](https://developer.mozilla.org/en-US/docs/Web/API/WebGPU_API)
- [MDN Document.startViewTransition](https://developer.mozilla.org/en-US/docs/Web/API/Document/startViewTransition)
- [html2canvas FAQ](https://html2canvas.hertzen.com/faq.html)
- [MDN cross-origin images and canvas security](https://developer.mozilla.org/en-US/docs/Web/HTML/How_to/CORS_enabled_image)

## Real Artifact

The artifact is the deployed Desktop Overview shell mode as a live spatial
control surface:

```text
persistent desktop/window state
-> active real overlapping windows
-> bounded live preview policy
-> transformed live DOM previews where safe
-> app-owned suspended/redacted preview cards where needed
-> focus/suspend/minimize/close/recover actions
-> staging proof under ordinary and heavy restored sessions
```

The artifact is not:

- a WebGPU demo;
- a generic screenshot grid;
- fake thumbnails;
- a duplicated second mount of each app inside Overview;
- a mobile phone-mode app switcher;
- a host/system monitor;
- local-only proof of a platform shell behavior.

## Invariants

- Mobile and desktop remain one overlapping-window ontology: move, resize,
  focus, z-index, minimize, maximize, restore, close.
- Desktop Overview remains a shell mode, not an app window and not a separate
  launcher island.
- Live previews must be derived from real window state. They must not duplicate
  expensive app mounts, manually seed fake content, or imply captured pixels
  when none exist.
- Suspended/heavy/private windows may use app-owned lightweight cards or
  redacted previews. Honesty beats fake fidelity.
- No preview pixels are persisted, sent to Trace, written into logs, or exposed
  across users.
- Preview policy is bounded by user-computer restore weight, app heaviness,
  privacy class, active/minimized/suspended state, and viewport size.
- Overview actions preserve active computer state and window identity. They
  must not silently discard foreground work to make the board look clean.
- Compute/resource labels shown to users remain scoped to their computer,
  candidate computers, app/window restore weight, and product health. Do not
  expose host-wide RAM, raw VM handles, or global platform telemetry.
- Logged-out read/explore remains possible. Private-computer recovery and
  mutation actions require auth.
- Platform behavior changes require git, CI, deploy, staging identity, and
  deployed product-path proof.

## Value Criterion

Minimize:

```text
overview abstraction gap
+ hidden window stack depth
+ cognitive load in crowded sessions
+ accidental app hoarding
+ memory cost of preview fidelity
+ privacy exposure from previews
+ fake thumbnail fidelity
+ duplicated app mount cost
+ local-only shell claims
```

subject to the invariants above.

The mission moves uphill when a user can open Overview on a crowded desktop and
immediately understand "what is open, where it is, what is active, what is
expensive, what is suspended, and what can be safely resumed or closed" without
losing the sense that they are operating one real desktop.

## Quality Gradient

Target quality: **solid**, with excellent care for interaction taste, privacy,
and memory control.

Solid means:

- the Overview opening/closing animation uses compositor-friendly transforms
  and avoids layout thrash;
- ordinary sessions get live spatial previews where safe;
- heavy sessions degrade gracefully to a bounded live set plus honest cards;
- private/secure windows can be redacted without breaking focus/recovery;
- mobile `390x844` keeps the same real desktop model;
- DOM metrics prove preview state, live count, suspended/redacted count, and
  mounted heavy body count;
- screenshots prove the result is spatial and readable;
- tests distinguish full completion from checkpoint progress.

Substandard work:

- adding screenshot thumbnails that are stale, fake, or privacy-sensitive;
- remounting apps inside Overview to get previews;
- making WebGPU a blocker for shell behavior;
- hiding suspended windows because they cannot be previewed live;
- collapsing mobile into one full-screen card at a time;
- claiming "live thumbnails" when only titles and icons changed;
- claiming completion without staging proof.

## Product Vocabulary

- **Live spatial preview:** a real mounted window visually transformed into an
  Overview target rectangle.
- **Preview card:** app-owned lightweight representation used when a live
  preview is unsafe or too expensive.
- **Redacted preview:** privacy-preserving card with app identity/state but no
  content.
- **Preview budget:** per-viewport and per-session cap on live preview count and
  mounted heavy app count.
- **Spatial truth:** Overview layout preserves enough x/y/size/z-order
  relationship that users recognize their desktop.
- **Hospitality polish:** calm, precise, reversible motion and controls that
  make a crowded session feel cared for, not administrated.

## Homotopy Parameters

Increase realism continuously:

- **Window count:** 4 windows -> 8 windows -> 12+ restored windows.
- **Preview fidelity:** icon card -> app-owned card -> live transformed DOM for
  safe visible windows -> optional richer effects.
- **Preview budget:** active window only -> top 3 live on mobile -> top 6 live
  on desktop -> adaptive policy based on restore pressure.
- **Privacy class:** ordinary app -> media app -> VText/Trace -> candidate or
  sensitive/private windows requiring redaction.
- **App heaviness:** lightweight apps -> PDF/EPUB/Image -> Trace/VText/media ->
  candidate desktop/terminal surfaces.
- **Motion realism:** instant layout -> FLIP transform -> reversible spring-like
  timing -> reduced-motion fallback.
- **Proof realism:** local component proof -> staging ordinary session ->
  staging heavy restored session -> returning-user session.

## Starting Belief State

Known:

- `b148461dafc6125fa321de9b10814cdc6af285b6` is the deployed heavy-session
  Desktop Overview baseline.
- Heavy-session proof passed on staging with 12 visible windows, 11 heavy
  windows, 10 suspended windows, 1 mounted heavy app body, 66 overlap pairs, 12
  Overview cards, and Compute Monitor handoff.
- The current Overview is spatial and actionable, but it is card/map based, not
  live-preview based.
- The recent crash class came from restoring too much app/window weight, so any
  live preview system must not remount or eagerly hydrate more apps.

Main uncertainties:

- How many live transformed windows can be shown safely on `390x844` mobile and
  desktop before interaction or memory degrades.
- Which apps should default to redacted or app-owned cards.
- Whether true DOM transforms are enough visually, or whether a later effect
  layer is needed after the behavior is correct.
- How Overview should represent minimized and suspended windows spatially.
- Whether browser support for View Transitions adds useful polish without
  becoming a dependency.

Highest-impact observation:

- A staging Playwright run that opens/restores a mixed heavy session, opens
  Overview, verifies bounded live previews and honest suspended/redacted cards,
  focuses a live preview, resumes a suspended card, closes Overview, and proves
  mounted heavy app count and restore pressure stay bounded.

## Investigation And Cognitive Reframing

If the mission stalls, do not stop at "live thumbnails are hard" or "we need
WebGPU." Run root-cause probes and apply route-changing transforms.

Cognitive transforms:

- **Actual-object transform:** animate the real window object before inventing a
  representation of it.
- **Hospitality-before-spectacle transform:** the Overview should make the user
  feel oriented and cared for; effects that obscure task recovery are negative.
- **Budget-before-fidelity transform:** define live preview budget first, then
  choose fidelity inside that budget.
- **Redaction-is-honesty transform:** for private/heavy/suspended windows, a
  truthful card is better than a fake screenshot.
- **Spatial-memory transform:** preserve position and stack relationships so
  users recognize their desktop by memory, not by reading labels.

Tactical blockers should trigger another probe or patch: transform math bugs,
bad z-order, clipped windows, poor touch targets, overly high live count,
wrong heaviness classification, animation jank, or selector/test gaps.

Invariant-level blockers require escalation: preview leaks private content,
state is discarded without consent, implementation needs cross-user pixel
capture, or mobile overlapping windows must be abandoned to make the design
work.

## Receding-Horizon Control

Operate in short intervals:

1. Measure the current Overview geometry and restore pressure.
2. Implement the smallest live-preview policy or transform change.
3. Verify locally with focused Playwright and DOM metrics.
4. Check that mounted heavy app count did not increase unexpectedly.
5. Deploy behavior changes through the landing loop.
6. Re-run staging ordinary and heavy-session proof.
7. Update this mission doc with belief-state changes, checkpoint status, and
   evidence refs.

Prefer proving live transformed DOM for a small bounded set before adding
effect polish.

## Implementation Direction

### P0: Preview Policy Model

- Add explicit preview states: `live`, `card`, `redacted`, `suspended`.
- Derive state from app id, privacy/sensitivity, minimized/suspended mode,
  active/top-N z-order, viewport, and restore pressure.
- Expose data attributes for Playwright: preview state, live count, redacted
  count, suspended count, heavy count, mounted heavy count, active preview id.

### P1: Real DOM Transform Overview

- Animate safe mounted windows into Overview target rectangles using measured
  source and target geometry.
- Keep the same app instance mounted; do not create a second app render tree.
- Preserve pointer/focus semantics: clicking/tapping a preview focuses that
  window and exits or resolves Overview predictably.
- Support reduced-motion fallback.

### P2: App-Owned Preview Cards

- For suspended/heavy/private/minimized windows, render honest lightweight cards
  with app icon, title, state, and safe actions.
- Give apps a small optional preview descriptor later if needed, but do not
  require every app to build a miniature UI before Overview works.
- Redact sensitive surfaces by default until privacy policy is explicit.

### P3: Premium Interaction Polish

- Use motion that feels spatial and reversible: short, clear, no noisy bounce.
- Highlight the active window as the obvious primary task.
- Keep controls quiet until hover/tap/long-press or keyboard focus.
- Provide tactile mobile hit targets without turning mobile into a phone mode.
- Optional effect layer may use CSS, View Transitions, WebGL, or WebGPU only
  after the core behavior and budgets pass.

### P4: Deployed Proof

- Extend existing Overview tests to cover ordinary and heavy restored sessions.
- Capture desktop and `390x844` screenshots before Overview, during Overview,
  after focusing a live preview, and after resuming a suspended/card preview.
- Assert live preview budget and mounted heavy app body count.
- Verify staging identity and record CI/deploy/run artifacts.

## Dense Feedback Channels

- `npm run build`
- focused frontend unit/type checks if available;
- local Playwright for Overview transform behavior;
- deployed staging Playwright for ordinary session and 12-window heavy restored
  session;
- screenshots at desktop and `390x844`;
- DOM metrics for preview state counts, mounted heavy body count, geometry
  ratios, overlap pairs, active window id, and action outcomes;
- staging `/health` identity;
- Compute Monitor or product health evidence where resource/restore claims are
  involved.

## Evidence Ledger

For each nontrivial claim, record:

```text
claim:
evidence source:
command or observation:
artifact path:
result:
uncertainty/caveat:
promotion relevance:
```

Required claims:

- live previews are actual transformed window DOM, not screenshots or duplicate
  app mounts;
- preview budget is enforced on mobile and desktop;
- suspended/heavy/private windows use honest cards or redaction;
- Overview focus/resume/close actions preserve state;
- mounted heavy app count remains bounded under heavy restored session;
- staging build identity matches the pushed behavior commit.

## Run Checkpoint And Resumption State

```text
status: proposed
last checkpoint: mission authored from heavy-session completion and live-preview design review
current artifact state: Desktop Overview is spatial/card based with heavy-session recovery proof, not live-preview based
what shipped: none for this mission yet
what was proven: prior heavy-session Overview proof at b148461
unproven or partial claims: live transformed previews, preview privacy policy, preview budget, polished motion
belief-state changes: none yet
remaining error field: Overview still feels like a management grid rather than a live spatial room
highest-impact remaining uncertainty: can live DOM transforms produce premium previews without increasing memory/crash risk?
next executable probe: implement preview policy data model and a bounded live-transform prototype for safe visible windows
suggested resume goal string: use the One-Line Goal String above
evidence artifact refs: prior heavy-session Playwright artifacts under frontend/test-results/desktop-overview-heavy-session-*
rollback refs: revert behavior commit from this mission; prior deployed baseline b148461
```

During or after execution, update this section with `complete`,
`checkpoint_incomplete`, `blocked_incomplete`, or `superseded`. Do not call a
useful partial checkpoint complete.

## Forbidden Shortcuts

- Do not build a WebGPU demo and call it Desktop Overview.
- Do not use canvas/html screenshots as the primary acceptance path.
- Do not persist preview images or expose preview pixels in Trace/logs.
- Do not duplicate app mounts inside Overview.
- Do not fake thumbnails with static cards while claiming live preview proof.
- Do not hide suspended windows because they cannot be previewed.
- Do not turn mobile into a one-window task carousel.
- Do not expose host/global memory, raw VM handles, or platform-wide telemetry.
- Do not silently discard active computer state to reduce restore weight.
- Do not use local-only proof for deployed platform shell claims.
- Do not claim completion without staging identity, screenshots, DOM metrics,
  rollback refs, and residual risks.

## Rollback Policy

- Platform code rollback: revert this mission's behavior commit and redeploy.
- UI state rollback: Desktop Overview must remain closable with Escape/backdrop
  and return to normal desktop state.
- Recovery rollback: any new suspend/close/keep-active action must preserve the
  existing restore recovery and Compute Monitor paths.
- Privacy rollback: disable live preview for an app class and fall back to
  redacted cards if preview risk is found.
- Performance rollback: reduce preview budget to active/top-1 live preview or
  cards-only under pressure.

## Learning Side-Channel

Record tactical lessons in this mission doc. Update
[platform-os-app-state.md](platform-os-app-state.md) only if deployed behavior
changes the current shell/Overview state, app catalog, or operating rules.
Update [docs/README.md](README.md) when mission status changes.

Classify surprises:

- Tactical: transform math, layout, selectors, or preview budget tuning.
- Target-level: a better Overview model such as hybrid map plus live board.
- Invariant-level: privacy or memory evidence shows live previews cannot be
  safe under the current shell model.

## Stopping Condition

Report `complete` only when:

- live previews for safe windows are real transformed window DOM, or the mission
  explicitly proves that a lower-fidelity representation is the correct
  topology-preserving target;
- preview states are explicit and product-visible in DOM metrics;
- live preview budget is enforced on desktop and `390x844`;
- suspended/heavy/private windows use app-owned or redacted cards honestly;
- focus/resume/close actions work from Overview;
- ordinary and 12-window heavy restored staging sessions pass Playwright proof;
- mounted heavy app body count remains bounded;
- screenshots show a premium spatial Overview rather than a list/grid-only
  management panel;
- staging identity, CI/deploy, rollback refs, residual risks, and next realism
  axis are recorded.

If useful progress lands but any required proof is missing, report
`checkpoint_incomplete` and update the Run Checkpoint section. If a blocker
remains after root-cause probes and cognitive transforms, report
`blocked_incomplete` with exact evidence and the smallest safe next probe.
