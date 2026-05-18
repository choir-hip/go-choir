# MissionGradient: UX Full Bag Sweep v0

Status: ready for execution
Date: 2026-05-18
Operator: Codex supervising staging, Choir-in-Choir workers where viable, git, CI, deploy, Playwright, Trace, VText, and owner review

## One-Line Goal String

```text
/goal Run docs/mission-ux-full-bag-sweep-v0.md as a Codex-operated MissionGradient mission: make the automatic computer feel coherent and usable across its web desktop, app boundaries, evidence surfaces, and media apps. Start from deployed baseline 3b62a31, where Podcast has just been split out of ContentViewer, then continue the full UX bag sweep: split image/audio/video/PDF/EPUB into real apps with standard controls, harden the floating window shell on mobile and desktop, make Trace a readable evidence app, stabilize VText editing/coexistence, repair prompt/bottom-bar/app-switching behavior, improve logged-out read/explore with auth only at mutation, and make candidate/promotion surfaces appear contextually instead of requiring manual IDs. Use product-path Playwright, screenshots, DOM metrics, Trace/VText/run-acceptance evidence, and staging identity for proof. Prefer Choir-in-Choir worker/candidate dispatch for mutable app/platform work when the substrate is healthy; if the substrate blocks the sweep, root-cause and repair it directly through git/CI/deploy. Do not shrink mobile into a gimped phone mode, stuff more apps into ContentViewer, use fake placeholders, hide failures behind debug controls, rely on local-only proof, or claim success without deployed evidence, rollback refs, residual risks, and the next realism axis.
```

## Mission Frame

This is a full-bag UX sweep for the automatic computer, not a single-app polish sprint.

The user-facing problem is that several independent failures have the same root: Choir does not yet have a strong desktop/app UX substrate. Podcast exposed this because it was trapped in `ContentViewer`; Trace exposes it because evidence is unreadable on mobile; VText exposes it because editing and Trace coexistence can flicker or fight focus; the window shell exposes it because minimized apps, focus, prompt-bar growth, bottom-bar layout, and logged-out affordances are not yet stable enough to support serious work.

The mission should make this substrate materially better while preserving the product identity: a powerful web desktop on mobile and desktop, not a simplified mobile app.

## Real Artifact

The artifact is Choir's deployed automatic-computer UX substrate:

```text
staging web desktop
-> floating window shell and app switcher
-> prompt bar and conductor action routing
-> logged-out read/explore boundary
-> standalone media/content apps
-> Podcast/Audio/Video/Image/PDF/EPUB app-grade controls
-> VText editing and reading surface
-> Trace evidence and run-acceptance inspection surface
-> candidate/promotion/contextual review surfaces
-> product-path evidence proving the whole system works on desktop and mobile
```

The artifact is not `ContentViewer`, not a set of disconnected Svelte patches, and not a checklist of visible bug fixes. The artifact is the user computer becoming stable enough that future effort can focus on automatic newspaper intelligence instead of fighting the substrate.

## Starting Belief State

Baseline:

- Latest deployed commit before this mission: `3b62a31eb4c06dbec3611930e91084bf94a60eff`.
- Podcast was split into `PodcastApp.svelte` at that baseline and no longer renders inside `ContentViewer`.
- `ContentViewer` remains a generic surface for image/audio/video/PDF/EPUB/reference content and should not accumulate more app behavior.
- Mobile is intentionally a web desktop. Floating/overlapping windows should remain possible on touch viewports.
- Known UX failures include weak window raise/focus/restore behavior, bottom/prompt bar growth oddities, Trace mobile unreadability, VText flicker/coexistence problems, candidate/promotion surfaces requiring manual IDs, weak media app controls, settings/theme quality issues, and over-aggressive auth walls.

Highest-impact uncertainty:

- Whether the window shell or individual app layouts are the dominant cause of mobile unusability. Probe both, but fix the shell first when it explains multiple app failures.

Next observations that reduce uncertainty:

- Playwright mobile and desktop screenshots/DOM metrics with several windows open together.
- Trace/VText co-open editing proof.
- Long-content app scroll measurements.
- Logged-out app-launch and mutation-gated auth proof.
- Product-path prompt actions such as opening Podcast, Trace, VText, and candidate/promotion context without manual IDs.

## Invariants

- Mobile remains a desktop. Do not replace floating windows with single-app phone navigation as the acceptance path.
- App boundaries must get cleaner. Podcast, Image, Audio, Video, PDF, and EPUB should become separate app surfaces or explicit components with app-grade controls, not branches inside a swollen `ContentViewer`.
- Logged-out public/read/explore actions should work. Mutation, persistence, uploads, private state, model/search/worker actions, subscription/adoption, and promotion require auth.
- Window shell behavior must be predictable: focus, raise, minimize, restore, fit, snap, drag, resize, and app switching should have visible, touch-usable affordances.
- Evidence surfaces must remain truthful. Trace and VText should expose uncertainty and failed probes instead of laundering missing evidence into success.
- Product-path proof is required for platform UX claims. Local proof may shape fixes, but staging proof decides.
- Platform behavior changes land through:

```text
commit -> push origin main -> monitor CI -> monitor staging deploy
-> verify staging commit identity -> run deployed acceptance proof
```

- Choir-in-Choir is the preferred self-development path for mutable UX work when healthy. If worker/candidate/export evidence blocks the sweep, direct Codex platform repair is allowed only after the blocker is named and root-caused.
- Do not mutate active user computers directly for risky candidate work. Candidate/promotion flows need lineage, rollback, and evidence.

## Value Criterion

Minimize:

```text
UX substrate friction
+ app-boundary confusion
+ mobile desktop uncertainty
+ evidence inspection friction
+ editor/focus instability
+ auth overblocking
+ media-control incompleteness
+ hidden state
+ verifier Goodharting
+ future cleanup debt
```

subject to the invariants above.

The mission moves uphill when a normal user can open multiple real apps, move between them, inspect evidence, read/write VText, play media, and understand candidate/promotion state on both desktop and mobile without relying on debug knowledge.

The mission moves downhill when it makes a local visual improvement by creating new hidden state, fake app islands, generic-content hacks, permissive tests, manual-ID-only paths, or mobile behavior that is no longer a desktop.

## Quality Gradient

Expected quality: `solid-to-excellent`.

Solid means:

- app boundaries are named and locally understandable;
- tests cover the behavior that broke;
- Playwright proof includes mobile and desktop;
- staging identity and rollback refs are recorded;
- residual risks are explicit.

Excellent means:

- the shell patterns generalize across apps;
- app code is simpler after the split;
- the user can infer controls without explanatory copy;
- future app additions have an obvious path;
- evidence review is easier for a human after the mission than before it.

Substandard work:

- another large component that branches on app type;
- controls that exist but are too small, hidden, or unreachable on mobile;
- assertions that only check code existence;
- screenshots without DOM metrics;
- local-only claims for deployed behavior;
- UI summaries that hide failed proof.

## Homotopy Parameters

Increase realism continuously while preserving product topology:

- app separation: Podcast split -> shared app-loading helpers -> Image/Audio/Video/PDF/EPUB standalone apps;
- shell robustness: basic focus/raise -> mobile fit/snap/restore -> configurable dock/top/bottom bar;
- Trace: readable summary -> drill-in inspector -> artifacts/run-acceptance/promotion evidence;
- VText: flicker reproduction -> editor-state fix -> Trace/VText coexistence proof;
- auth: logged-out app launch/read -> action-level auth prompts -> persisted-private-state gates;
- conductor: app launch -> app-specific command intents -> cross-app action such as "play latest Lenny's podcast";
- candidate/promotion UX: manual IDs -> contextual cards from current Trace/run/promotion evidence -> owner-review actions;
- verification: unit/build -> local Playwright -> staging Playwright -> product-path Trace/run acceptance.

Avoid fake ladders. A simplified app or shell must use the same production route family, state ownership model, and evidence semantics as the fuller version.

## Execution Model

Use receding-horizon loops. Each loop should pick the highest-value next UX substrate improvement, make a bounded mutation, verify, update belief state, and then continue.

Preferred loop:

1. Baseline the relevant UX with Playwright screenshots, DOM metrics, and product API evidence.
2. Decide whether the failure is app-local, shell-level, auth-boundary, evidence-surface, or substrate/deploy.
3. Dispatch through Choir-in-Choir if the worker/candidate path is healthy and the task is mutable app/platform work.
4. If Choir-in-Choir blocks progress, root-cause the blocker and repair direct platform code only when needed to restore the self-development loop or complete the UX sweep safely.
5. Land platform changes through git/CI/deploy.
6. Verify staging identity and rerun deployed acceptance.
7. Record evidence, rollback, residual risk, and next realism axis.

Mutation radius:

- Prefer one app boundary, shell behavior, or evidence surface per commit.
- Larger commits are allowed when a shared app substrate must be introduced to prevent duplicated code.
- Do not rewrite unrelated architecture while chasing cosmetic wins.

## Priority Sweep Surface

### 1. Shell First When It Explains Multiple Bugs

Fix or precisely isolate:

- tapping minimized apps not reliably raising/restoring them;
- show-desktop interactions needed before restore works;
- bottom bar growing empty space and not shrinking when prompt content shrinks;
- prompt bar height and focus management;
- app switcher/taskbar affordances for all open windows, not only minimized windows;
- raise/focus for overlapped windows on mobile;
- fit-to-screen and snap/restore controls with touch-safe targets;
- optional top/bottom/side placement as a substrate, not one-off CSS.

Acceptance:

- On `390x844` and desktop viewport, open VText, Trace, Podcast, and one media app.
- Prove focus/raise/minimize/restore/fit/snap with Playwright.
- DOM metrics show no incoherent horizontal overflow or unreachable controls.

### 2. Split Remaining Content Apps

Create standalone app surfaces for:

- Image: pan/zoom/fit/original, metadata secondary, touch drag/zoom where feasible.
- Audio: persistent player, seek, speed, progress, queue/title metadata.
- Video: native/YouTube playback controls, fit/full-window, source fallback.
- PDF: page scroll, page number, zoom, fit width/page, search or explicit follow-up if search is too large.
- EPUB: table of contents or chapter navigation, reader scroll, font/size, progress.

`ContentViewer` should become a small generic fallback/dispatcher or disappear from normal media app routing.

Acceptance:

- Bare content references still route to the correct app through the prompt bar.
- Each app has a focused Playwright proof and one mobile screenshot/metric set.
- No new app relies on UUID/hash/provenance as primary UI.

### 3. Podcast Continuation

Podcast is already split out; continue from that baseline.

Improve:

- subscribed podcast list and empty/recommended state;
- search/import hierarchy;
- episode list scroll and selection;
- playback position persistence beyond local-only where appropriate;
- played/unplayed state;
- commandability through conductor, starting with a narrow "play latest <podcast>" path if product APIs support it;
- VText radio-brief continuity without making VText the primary podcast UI.

Acceptance:

- Staging mobile proof with a long feed.
- Desktop proof with library/search/detail/player/back.
- Mutation/persistence gates are correctly auth-protected.

### 4. Trace Evidence App

Make Trace usable as a real evidence app, especially on mobile desktop.

Improve:

- trajectory list readability;
- selected trajectory summary;
- agent graph and child-agent drill-in;
- moment/timeline readability;
- inspector pane reachability;
- tool-call/result wrapping;
- run acceptance, export, candidate, promotion, rollback, manifest, patch links;
- mobile panes/tabs without hiding causal detail.

Acceptance:

- Use a real recent trajectory or create one through the visible prompt bar.
- Show a user can inspect child agents, channel messages, worker/export/promotion evidence, and run acceptance on `390x844`.
- No manual route or raw internal endpoint is required for normal inspection.

### 5. VText Stability And Coexistence

Fix or precisely isolate:

- text flicker while editing;
- selection/caret loss during reactive updates;
- Trace and VText open together causing focus or layout problems;
- public/read mode vs authenticated mutation clarity;
- version/revision feedback that does not interrupt editing.

Acceptance:

- Playwright typing test with Trace open beside/overlapping VText.
- Cursor/selection remains stable enough for normal editing.
- No unexpected text reset/flicker under a simulated or real refresh/update.

### 6. Candidate, Promotion, And VM UX

Improve normal-user candidate/promotion surfaces:

- candidate VM evidence should appear when context exists; users should not need to paste hashes for normal flows;
- manual IDs should move to advanced/developer mode;
- candidate cards should show source trajectory, worker/candidate refs, verifier status, rollback, and owner action;
- promotion/adoption should be inspectable in Trace and product APIs.

Acceptance:

- From a Trace/run/promotion context, candidate review UI can be opened without manual ID entry.
- Evidence is product-visible and rollback refs are named.

### 7. Settings And Theme Quality

Do not let settings become the mission, but remove obvious theme/system quality failures:

- themes should not feel like random palettes;
- typography and contrast should support app work;
- any top/bottom/side dock setting must be coherent with shell layout persistence.

Acceptance:

- One focused settings proof if modified.
- No new theme creates unreadable Trace/VText/Podcast surfaces.

## Dense Feedback Channels

Use a layered proof stack:

- `npm --prefix frontend run build`
- focused frontend Playwright tests
- mobile Playwright at `390x844`
- desktop sanity viewport
- screenshot artifacts in `test-results/`
- DOM metrics:
  - scroll region height vs scroll height;
  - horizontal overflow;
  - visible control counts;
  - app/window bounds;
  - active/focused/minimized window state;
  - prompt/bottom bar height before/after text changes;
- Trace API/product UI evidence for real runs;
- VText evidence for edit stability and readable reports;
- CI and staging deploy status;
- staging `/health` commit identity;
- run acceptance where self-development/candidate/promotion work is involved.

## Evidence Ledger Format

For every nontrivial claim record:

```text
claim:
evidence source:
command or product observation:
artifact path or URL:
result:
uncertainty/caveat:
rollback relevance:
promotion relevance:
```

Completion reports should group evidence by user-facing capability, not by how the implementation happened.

## Investigation And Cognitive Reframing

Before stopping on a blocker:

1. Classify it as app-local, shell-level, auth-boundary, evidence-surface, substrate/deploy, invariant-level, or external.
2. Run at least one root-cause probe at the implicated layer.
3. Apply 2-5 cognitive transforms that change the next probe, not just wording:
   - **Boundary transform:** is this app-local or shell-level?
   - **User-action transform:** what exact user action is impossible or unreliable?
   - **Evidence-surface transform:** can the product explain its own state?
   - **Homotopy transform:** is this a smaller real version or a fake island?
   - **Control-surface transform:** should conductor drive this through app APIs instead of manual UI?
4. If the transformed probe is inside current authority and safe, execute it instead of ending.

Only stop on a blocker when continuing would cross an authority boundary, violate an invariant, become unsafe/destructive, or repeat falsified probes without new evidence.

## Forbidden Shortcuts

- Do not make mobile a single-app or reduced phone mode.
- Do not add more media-specific behavior to a giant generic `ContentViewer`.
- Do not use source/provenance/hash panels as primary app UI.
- Do not use fake transclusion panels, fake island placeholders, fake screenshots, or mocked success data.
- Do not hide normal actions behind manual IDs.
- Do not weaken auth for mutation/persistence/private state.
- Do not claim logged-out support if only the shell opens but app read/explore is blocked.
- Do not claim Trace readability from desktop screenshots only.
- Do not claim VText stability without typing/editing proof.
- Do not accept worker self-report without verifier/product evidence when candidate/promotion work is in scope.
- Do not skip deploy proof for platform UX behavior.
- Do not produce a long chat-log report instead of a reviewable evidence ledger.

## Rollback Policy

Baseline rollback reference:

```text
3b62a31eb4c06dbec3611930e91084bf94a60eff
```

For each landed commit:

- preserve git SHA and previous deployed SHA;
- name CI run and deploy result;
- verify staging health identity;
- keep screenshots/test artifacts;
- state whether rollback is a simple git revert, config restore, state rollback, VM/candidate discard, route/profile rollback, or promotion rollback.

If user state is touched, name the affected state and rollback/cleanup path. Prefer test users and seeded artifacts for proof.

## Learning Side-Channel

Write durable learnings into one of:

- this mission document under `Operator Log`;
- a follow-up architecture doc when the learning changes app/shell architecture;
- focused tests that encode the regression;
- final evidence report.

Classify learnings:

- Tactical: fix during mission.
- Target-level: update mission surface/priority and continue.
- Invariant-level: stop and escalate before changing the product ontology or authority boundary.

## Stopping Condition

Successful stop requires deployed evidence that the automatic computer UX substrate is materially better across the full bag, not merely one fixed screenshot:

- app boundary progress: Podcast remains standalone and at least two more media surfaces are standalone or app-grade with tests, or a precise blocker explains why the split cannot continue yet;
- shell progress: focus/raise/minimize/restore/fit/snap or prompt/bar behavior is measurably improved on mobile and desktop;
- Trace progress: real trajectory evidence is readable and drillable on mobile;
- VText progress: edit stability/coexistence is proven or a precise root-cause blocker is recorded;
- logged-out progress: read/explore boundary is improved without weakening mutation auth;
- candidate/promotion UX progress: contextual evidence surfaces improve or blocker is named;
- CI/deploy/staging identity proof exists for platform behavior changes;
- screenshots, DOM metrics, acceptance commands, rollback refs, residual risks, and next realism axis are named.

If the mission timebox ends before all surfaces are complete, the final report must still include:

- what shipped and what was proven;
- what remains, ranked by impact on automatic-computer stability;
- why remaining work is not safe or realistic to continue in the current run;
- the next executable probe.

Do not call the mission complete if only local tests passed, if mobile was reduced to a non-desktop mode, or if the app split regressed content routing.

## Operator Log

Append dated entries here during execution. Include belief-state updates, dispatched prompts, trajectory/run IDs, commits, CI/deploy refs, screenshots, DOM metrics, rollback refs, and residual risks.
