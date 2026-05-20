# MissionGradient: Desktop Overview App-Owned Spatial Previews v0

**Status:** proposed
**Date:** 2026-05-20
**Operator:** Codex or Choir-in-Choir supervisor with staging Playwright, git,
CI, deploy, product-path evidence, and design/taste review
**Predecessor:** [mission-desktop-overview-live-spatial-previews-v0.md](mission-desktop-overview-live-spatial-previews-v0.md)
**State ledger:** [platform-os-app-state.md](platform-os-app-state.md)
**Starting docs head:** `b03d2d19dab78f16373559b380157c2e918fc25d`
**Starting deployed behavior baseline:** `2f8ad7adc2697d6faff00dbc90991057c19781e9`

## One-Line Goal String

```text
/goal Run docs/mission-desktop-overview-app-owned-spatial-previews-v0.md as a Codex-operated MissionGradient mission: make Desktop Overview feel like a premium spatial control room for Choir's real overlapping desktop under ordinary and heavy real-user sessions. Starting from live DOM previews at 2f8ad7a, replace hard-coded preview heuristics with app-owned preview descriptors for privacy, heaviness, restore cost, preview fidelity, safe summary fields, and Overview actions; make safe windows animate as real DOM, while suspended, heavy, private, terminal, and candidate surfaces render honest app-owned or redacted preview cards. Polish the Overview layout so it is spatial, quiet, hospitable, and useful with many windows: active task legible, stack depth visible, controls available on intent, keyboard/touch navigation reliable, and no phone-mode simplification. Do not use WebGPU, canvas screenshots, duplicated app mounts, persisted preview captures, fake thumbnails, host/global telemetry, or local-only proof as the acceptance path. Land platform changes through git/CI/deploy and prove staging identity with desktop and 390x844 Playwright screenshots/DOM metrics under ordinary, generated heavy, and at least one returning-session-style restore profile. Stop only with deployed evidence of app-owned preview policy, bounded live previews, redaction/resource safety, Overview interaction quality, rollback refs, residual risks, and the next realism axis; otherwise report checkpoint_incomplete or blocked_incomplete with a resumable mission-doc checkpoint and continue/redirect/delegate any safe executable next probe inside current authority before stopping.
```

## Mission Frame

The previous mission proved the crucial substrate: Desktop Overview can show a
bounded set of safe live previews by transforming the real mounted window DOM,
while suspended, heavy, or unsafe windows fall back to honest cards. It avoided
the fake-thumbnail trap and preserved the heavy-session recovery budget.

The next realism axis is ownership and taste. The shell should not guess each
app's preview safety from scattered app ids. Each app should declare what
Overview may show, what must be redacted, how expensive it is to resume, which
state summary is useful, and which actions belong in the Overview. Then the
shell can arrange those previews as a calm spatial room rather than a dense
admin panel.

This is hospitality work as much as shell engineering. The target is not more
visual noise. It is anticipatory orientation: the user opens Overview and feels
"I know where I am, what is alive, what is paused, what is private, and what I
can safely do next."

## Real Artifact

The artifact is the deployed Desktop Overview v1 control surface:

```text
app registry / app-owned preview descriptors
-> preview policy decisions
-> safe live real-DOM spatial previews
-> app-owned lightweight cards for suspended/heavy windows
-> redacted cards for private/terminal/candidate surfaces
-> quiet spatial layout, keyboard/touch control, recovery actions
-> staging proof under ordinary, heavy, and returning-session-style restores
```

The artifact is not:

- a screenshot thumbnail system;
- a WebGPU or canvas demo;
- a duplicated second mount of each app;
- a generic card grid with better labels;
- a mobile phone-mode task switcher;
- a host/system telemetry dashboard;
- a local-only visual prototype.

## Invariants

- Mobile and desktop remain the same overlapping-window desktop ontology:
  movable, resizable, overlapping windows with visible stack depth.
- Desktop Overview remains a shell mode, not an ordinary app window.
- Live previews continue to use the existing real window DOM where safe. Do not
  duplicate app mounts to create previews.
- App-owned preview descriptors must be declarative, bounded, and safe by
  default. The shell may use the descriptor; it must not trust app code to
  bypass privacy or resource policy.
- Suspended, heavy, private, terminal, and candidate windows must use honest
  lightweight or redacted cards. Do not imply live content when the app is not
  mounted or safe to show.
- No preview pixels are persisted, sent to Trace, logged, shared across users,
  or used as proof artifacts unless they are ordinary Playwright screenshots
  captured for this mission's acceptance report.
- Preview fidelity is bounded by restore weight, app heaviness, privacy class,
  viewport, active/minimized/suspended state, and user intent.
- Overview actions must preserve active computer state and window identity.
  They must not silently discard foreground work to make a board look clean.
- Compute/resource labels remain scoped to the user's computer and app/window
  restore weight. Do not expose host RAM, global vmctl inventory, raw VM
  handles, or platform-wide telemetry.
- Platform behavior changes require the landing loop: commit, push, CI,
  deploy, staging identity, deployed product-path proof.

## Value Criterion

Minimize:

```text
preview policy ambiguity
+ app-specific information loss
+ unsafe preview exposure
+ duplicated app/runtime cost
+ hidden restore pressure
+ hidden stack depth
+ Overview chrome burden
+ fake thumbnail fidelity
+ focus/restore uncertainty
+ local-only UX claims
```

subject to the invariants above.

The mission moves uphill when Overview becomes a place where each app shows the
right amount of itself: live when safe, summarized when paused, redacted when
private, actionable when useful, and quiet when the user is just orienting.

## Quality Gradient

Target quality: **solid-to-excellent**.

Solid means:

- app-owned preview descriptors exist in one obvious registry or interface;
- hard-coded preview safety and heaviness rules are reduced or eliminated;
- representative apps provide distinct preview descriptors, not one generic
  media/content card;
- live preview budget remains enforced on desktop and `390x844`;
- terminal and candidate surfaces are redacted by default;
- suspended heavy apps remain unmounted until explicitly resumed;
- Overview keyboard/touch navigation can focus, close, suspend, and resume
  without breaking the desktop;
- DOM metrics prove descriptor coverage, preview state counts, mounted heavy
  body count, active preview id, and redaction/suspension counts;
- screenshots show a spatial control room, not a list-heavy admin dashboard.

Excellent means:

- the active task is visually obvious without loud chrome;
- controls appear on intent: hover, focus, tap, or keyboard selection;
- app cards feel app-specific: Podcast shows playback/listening state, media
  apps show safe file/media summary, Trace shows run/evidence state, VText
  shows document state without leaking private body text by default, Compute
  Monitor shows user-computer recovery state, terminal/candidate stay redacted;
- motion is reversible, calm, and responsive with reduced-motion fallback;
- a real returning-session-style profile proves the design under organic
  window clutter, not only synthetic generated windows;
- the mission document and platform state ledger are updated with proof and
  residual risks.

Substandard work:

- renaming cards to previews without app-owned policy;
- adding visual polish while hard-coded app id heuristics remain the truth;
- making static fake thumbnails and calling them live;
- showing private text, terminal content, candidate content, or host telemetry
  for effect;
- remounting heavy apps to make Overview prettier;
- using local screenshots as a substitute for deployed staging proof;
- calling a checkpoint complete because some polish shipped.

## Product Vocabulary

- **App-owned preview descriptor:** a declarative object for each app identity
  that names privacy class, resource class, restore cost, safe summary fields,
  allowed preview fidelity, and allowed Overview actions.
- **Preview fidelity:** `live-dom`, `summary-card`, `redacted-card`, or
  `suspended-card` in v0. Richer modes require separate proof.
- **Safe summary field:** text or metadata that can appear in Overview without
  leaking private content or implying live state.
- **Restore cost:** product-level estimate of how expensive or risky it is to
  hydrate an app body during recovery.
- **Hospitality polish:** the interface anticipates the user's next recovery or
  focus action without crowding the screen.
- **Returning-session-style restore:** a staged proof profile that resembles a
  real user's messy desktop: mixed apps, minimized windows, suspended heavy
  apps, active media/readers, VText/Trace coexistence, and stale saved windows.

## Homotopy Parameters

Increase realism continuously:

- **Descriptor coverage:** shell defaults -> core app descriptors -> all
  launcher apps -> user-installed/candidate app descriptors.
- **Preview fidelity:** current live DOM/card states -> app-specific summary
  cards -> redacted/private cards -> optional richer safe preview effects.
- **Session realism:** four-window ordinary session -> 12-window generated
  heavy restore -> returning-session-style clutter -> real opted-in user
  session profile.
- **Privacy specificity:** app-id redaction -> descriptor privacy classes ->
  per-window/per-document preview permissions -> owner-controlled policy.
- **Resource specificity:** hard-coded heavy app ids -> app-owned restore cost
  -> measured client/resource signals -> adaptive live-preview budget.
- **Interaction depth:** tap/click focus -> keyboard roving focus -> batch
  suspend/close/keep-active -> accessible command palette or conductor intents.
- **Taste polish:** visible cards -> quiet chrome -> spatial grouping -> active
  task emphasis -> reduced-motion and touch quality.

## Starting Belief State

Known:

- `2f8ad7adc2697d6faff00dbc90991057c19781e9` is the deployed live-spatial
  Overview baseline.
- The existing implementation adds explicit preview states and transforms safe
  mounted windows as real DOM.
- Heavy-session proof passed on staging with 12 visible windows, 11 heavy
  windows, 10 suspended windows, 1 mounted heavy app body, 66 overlap pairs, 2
  live previews, and 10 suspended previews.
- The current preview policy is still mostly app-id based:
  `desktop-overview-preview.js` redacts `candidate-desktop` and `terminal`, and
  heavy behavior depends on `isHeavyAppId`.
- Overview cards and map are useful but still read as a management surface.

Main uncertainties:

- What descriptor shape is simple enough to land now but rich enough for future
  user-installed apps and candidate computers.
- Which fields each current app can safely expose without privacy leakage.
- Whether a returning-session-style restore profile will reveal different
  Overview layout or interaction failures than generated heavy sessions.
- How much motion/chrome polish can be added without creating a new memory or
  interaction risk.
- Whether optional richer thumbnail-like previews are worth pursuing after
  descriptor policy exists.

Highest-impact observation:

- A staging Playwright run that opens a returning-session-style desktop with
  Files, VText, Trace, Podcast, Image/PDF/EPUB/media, Compute Monitor, terminal,
  and candidate surfaces; opens Overview; verifies app-owned descriptors,
  redaction, live preview budget, mounted heavy app count, spatial layout,
  keyboard/touch focus, and recovery actions.

## Investigation And Cognitive Reframing

If the mission stalls, do not stop at "we need a design pass" or "previews are
hard." Investigate the next failure surface and transform the route.

Cognitive transforms:

- **Ownership transform:** if the shell is guessing app semantics, move the
  semantics to app-owned descriptors.
- **Hospitality transform:** remove chrome that makes the user manage the
  system before adding features that help the user recover their task.
- **Privacy-as-design-material transform:** redaction is not a fallback failure;
  it is an honest preview state.
- **Resource-budget transform:** preview fidelity is allowed only inside an
  explicit restore/mount budget.
- **Real-session transform:** if synthetic windows pass, replay a returning
  session shape before claiming UX confidence.

Tactical blockers should trigger another bounded loop: inspect, instrument,
patch the implicated layer, verify locally, then deploy and prove on staging.

Invariant-level blockers require escalation: descriptor policy leaks private
content, Overview actions discard active state, implementation needs persisted
pixel captures, or mobile overlapping windows must be abandoned.

## Receding-Horizon Control

Operate in short intervals:

1. Inspect current Overview policy, app registry, heavy-app metadata, and
   restore state shape.
2. Add or revise the smallest app preview descriptor interface.
3. Convert one representative app group at a time while preserving current
   Preview v0 behavior.
4. Verify descriptor coverage and preview counts with local Playwright.
5. Run a visual/taste pass on desktop and `390x844`.
6. Land through git, CI, deploy, staging identity.
7. Re-run ordinary, heavy, and returning-session-style staging proof.
8. Update this mission doc and platform state only with proven changes.

Prefer app-owned metadata and quiet layout polish before optional richer visual
effects.

## Implementation Direction

### P0: App Preview Descriptor Contract

- Define a small app-owned preview descriptor shape near the existing app
  registry, not scattered through Overview.
- Suggested fields:

```text
appId
previewPrivacy: public | private-summary | redacted
resourceClass: light | medium | heavy | external | candidate
restoreCost: low | medium | high
defaultPreview: live-dom | summary-card | redacted-card | suspended-card
safeSummaryFields
overviewActions
```

- Keep defaults conservative for unknown apps.
- Add DOM/debug metrics for descriptor source and coverage.

### P1: Replace Hard-Coded Preview Heuristics

- Route `desktop-overview-preview.js` through descriptors instead of fixed
  redacted app id sets wherever practical.
- Keep terminal and candidate redaction as descriptor policy, not ad hoc
  special cases.
- Make heavy/resource classification line up with Compute Monitor and restore
  suspension policy.

### P2: App-Specific Preview Cards

- Add app-owned cards for representative groups:
  - Files: location/count or selected file summary;
  - VText: document title, dirty/saved/version state, redacted body by default;
  - Trace: trajectory/run status, evidence count, current tab, no raw payload
    dump by default;
  - Podcast/Audio/Video: playback state, progress, title, safe source summary;
  - Image/PDF/EPUB: file title, page/chapter/progress, safe source summary;
  - Compute Monitor: current computer health/recovery summary only;
  - Terminal/Candidate Desktop: redacted card with safe status and actions.
- Do not build a generic media card that erases app boundaries.

### P3: Premium Spatial Layout And Interaction

- Keep live windows spatially related to their source desktop geometry.
- Make active task prominent but not full-screen or phone-mode.
- Make controls quiet until hover, focus, tap, or keyboard selection.
- Add roving keyboard/touch selection for cards/previews.
- Improve group/stack presentation for many windows without hiding them.
- Support reduced-motion and avoid layout shifts.

### P4: Returning-Session-Style Proof Harness

- Extend Playwright with a mixed restored profile that resembles real use:
  multiple VText/Trace/media/reader windows, minimized windows, suspended heavy
  windows, terminal/candidate redaction surfaces, active media/reader state,
  and Compute Monitor handoff.
- Assert:
  - descriptor coverage;
  - live/card/redacted/suspended counts;
  - mounted heavy app body count;
  - active preview id and focus outcome;
  - no terminal/candidate live content;
  - no host/global telemetry;
  - mobile and desktop screenshots.

### P5: Optional Richer Preview Effects

- Consider View Transitions, CSS effects, or app-generated safe preview
  descriptors only after P0-P4 pass.
- Do not make WebGPU, screenshot capture, persisted preview images, or duplicate
  mounts the acceptance path.

## Dense Feedback Channels

- `npm run build`;
- focused frontend unit/type checks if available;
- local Playwright for descriptor policy and Overview interactions;
- deployed staging Playwright for ordinary, generated heavy, and
  returning-session-style restores;
- screenshots at desktop and `390x844`;
- DOM metrics for descriptor coverage, preview state counts, mounted heavy body
  count, redacted surfaces, active preview id, keyboard/touch focus, action
  outcomes, and restore pressure;
- staging `/health` identity;
- Compute Monitor/product health evidence when resource or recovery claims are
  made.

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

- app-owned preview descriptors govern Overview policy;
- hard-coded app-id preview heuristics are removed or clearly limited;
- safe windows still use transformed real DOM, not screenshots or duplicate
  mounts;
- suspended/heavy/private/terminal/candidate windows use honest app-owned or
  redacted cards;
- preview budget remains enforced on mobile and desktop;
- mounted heavy app body count remains bounded under heavy and returning-style
  restores;
- Overview focus, keyboard/touch navigation, suspend/resume, and close actions
  preserve desktop state;
- screenshots show a spatial, quiet, usable Overview rather than an admin grid;
- staging build identity matches the pushed behavior commit.

## Run Checkpoint And Resumption State

```text
status: proposed
last checkpoint: mission authored after live-spatial Overview proof at 2f8ad7a and docs checkpoint b03d2d1
current artifact state: Overview has bounded live DOM previews and card/suspended/redacted fallbacks, but preview semantics are still mostly shell-owned heuristics
what shipped: none for this mission yet
what was proven: prior live-spatial Overview proof at 2f8ad7a
unproven or partial claims: app-owned descriptors, richer app-specific cards, returning-session-style proof, premium spatial/touch/keyboard polish
belief-state changes: none yet
remaining error field: Overview is functional but still too generic and system-management flavored
highest-impact remaining uncertainty: descriptor shape and app-specific preview policy that scales to user-installed/candidate apps without privacy or memory regression
next executable probe: define the descriptor contract, wire it into preview decisions, and convert a representative set of apps while preserving deployed live-preview behavior
suggested resume goal string: use the One-Line Goal String above
evidence artifact refs: prior CI/deploy run 26133712240; prior staging health at 2f8ad7a; frontend/tests/mobile-real-desktop-overview.spec.js; frontend/tests/desktop-overview-heavy-session.spec.js
rollback refs: revert this mission's behavior commit and redeploy; fallback behavior baseline 2f8ad7adc2697d6faff00dbc90991057c19781e9
```

During or after execution, update this section with `complete`,
`checkpoint_incomplete`, `blocked_incomplete`, or `superseded`. Do not call a
useful partial checkpoint complete.

## Forbidden Shortcuts

- Do not use fake screenshots, static thumbnails, or placeholder cards as proof
  of app-owned previews.
- Do not duplicate app mounts inside Overview.
- Do not persist preview captures or expose preview pixels in Trace/logs.
- Do not show terminal, candidate, private document body text, provider
  credentials, uploads, or host/global telemetry in Overview previews.
- Do not call the mission complete if only one app gets a descriptor.
- Do not let a generic media/content card erase separate Image, Audio, Video,
  PDF, EPUB, and Podcast app identities.
- Do not collapse mobile into full-screen phone-mode cards.
- Do not replace product-path proof with local screenshots.
- Do not weaken heavy-session recovery to make Overview prettier.

## Rollback Policy

- Platform code rollback: revert this mission's behavior commit and redeploy.
- Descriptor rollback: unknown or risky app descriptors fall back to redacted or
  summary cards.
- Privacy rollback: disable live preview for an app class and use redacted
  cards.
- Performance rollback: reduce live budget to active/top-1 or cards-only under
  pressure.
- Interaction rollback: if keyboard/touch navigation breaks focus or restore,
  preserve existing Overview focus/close/suspend controls.
- State rollback: Overview actions must leave active computer state and saved
  window identity recoverable.

## Learning Side-Channel

Record tactical lessons in this mission doc. Update
[platform-os-app-state.md](platform-os-app-state.md) only when deployed behavior
changes the current shell/Overview state or durable app/shell operating rules.
Update [docs/README.md](README.md) when mission status changes.

Classify surprises:

- Tactical: descriptor fields, transform math, CSS layout, selectors, tests.
- Target-level: a better Overview layout model, grouping model, or preview
  fidelity parameterization.
- Invariant-level: privacy/memory evidence shows a preview class cannot be
  safely shown under the current shell model.

## Stopping Condition

Report `complete` only when:

- app-owned preview descriptors govern Overview preview policy for the core
  app catalog;
- descriptor coverage, preview state, redaction state, and mounted-heavy metrics
  are product-visible in DOM assertions;
- safe windows still use transformed real DOM;
- suspended/heavy/private/terminal/candidate windows use honest app-owned or
  redacted cards;
- live preview budget and mounted heavy app body count remain bounded on
  desktop and `390x844`;
- Overview supports reliable focus, close, suspend/resume, keyboard/touch
  selection, and recovery actions;
- ordinary, generated heavy, and returning-session-style staging Playwright
  sessions pass;
- screenshots show a premium spatial Overview with quiet chrome and clear
  active task orientation;
- CI/deploy, staging identity, rollback refs, residual risks, and next realism
  axis are recorded.

If useful progress lands but any required proof is missing, report
`checkpoint_incomplete` and update the Run Checkpoint section. If a blocker
remains after root-cause probes and cognitive transforms, report
`blocked_incomplete` with exact evidence and the smallest safe next probe.
