# MissionGradient: Choir Redesign Hard Cutover on Node A - v0

Status: checkpoint_incomplete; Node A is deployed, but owner QA found the logged-out app/theme cut incomplete
Target branch: `codex/redesign-hard-cutover-node-a`  
Target host: `node-a`  
Target public URL: `https://choir-ip.com`  
Protected production host: `node-b` / `https://choir.news`  
Primary asset source: `docs/choir-redesign-hard-cutover-assets/`

## Goal String

```text
/goal Continue docs/mission-redesign-hard-cutover-node-a-v0.md as a Codex-operated MissionGradient mission from the 2026-05-29 owner QA redirect checkpoint. Stay on branch codex/redesign-hard-cutover-node-a and keep Node A as the disposable live design lab at https://choir-ip.com; do not touch main, Node B, choir.news, production auth, production mail routing, Resend behavior, or production secrets. The mission is incomplete until every app in the product app registry is visible and meaningfully interactive while logged out using frontend-only demo fixtures, and all three schema-v2 themes, exactly futuristic-noir, carbon-fiber-kintsugi, and london-salmon, can be switched and visually QA'd while logged out. Settings must open logged out so theme switching is always available. Email, Files, VText, Trace, Desktop Overview, Compute Monitor, Podcast, PDF, EPUB, Image, Video, Audio, Features if present, Terminal/Web Lens shells, Auth, PromptSurface, DeskSheet, desktop icons, window chrome, mobile app switching, empty/loading/error states, favicon/page titles/copy/typography, and tiny browser details must share one coherent tokenized style. Trace must include the reference-mockup swimlane/timeline model: horizontal lanes per agent/tool, duration bars, moment dots, failure ticks, and linked graph/timeline/inspector selection. Auth is requested only for durable/shared/private mutation, provider spend, account data, real prompt submission, save/revise/publish, send, import, activate, upload, rollback/roll-forward, or other owner-scoped actions; app opening, theme switching, fixture browsing, local VText typing, and preview inspection must not be auth gated. Remove hard outlines and mismatched old styles across all apps, including VText buttons and app controls; use coordinated radii, shadows, blur, depth, and theme tokens instead. Fix all overlap, navigation, and viewport safety issues: DeskSheet/tray content must never overlap, text must fit its containers, the Desk menu must be dense enough to show fully without scrolling in normal desktop and compact QA viewports, desktop icons must never go under the PromptSurface or off screen and must reflow under small or magnified viewports, and mobile must support open-app switching from the PromptSurface by tapping TetraMark to temporarily replace the prompt field with icons for currently open apps. Keep iterating after first correctness: use Computer Use as the primary visual verifier across desktop and mobile-sized viewports for every app and all three themes; use build/typecheck/Playwright only as regression support. Toward the end of the trajectory, perform a deliberate theme-convergence pass for each theme against docs/choir-redesign-hard-cutover-assets/reference-mockups: Futuristic Noir should recover the Trace/prompt reference's dark navy glass and luminous cyan/blue depth, Carbon Fiber Kintsugi should recover the dark industrial carbon texture and precise gold repair/glow language, and London Salmon should recover the salmon paper, oxblood/ink, serif, bespoke broadsheet character without becoming cute pastel. Deploy each reviewable checkpoint to Node A through the branch workflow, verify health/deployed commit, and stop only when Node A has a 90%+ owner-reviewable logged-out product cut with all apps/all themes proven, or when an invariant-level/external blocker is documented with the smallest safe next probe. Final report must include Node A deployed evidence, Computer Use observations, screenshot refs, app/theme QA matrix, branch/commit/CI identity, deleted-code diffstat, residual visual issues, build/typecheck/test status, and notes for morning review.
```

## Mission Identity

This mission is not a theme swap and not a local mockup. The real artifact is a live alternate Choir product surface on `https://choir-ip.com`, running `go-choir` from a redesign branch, with enough real shell behavior and frontend demo fixtures that visual review can happen without relying on Node B or passkey-authenticated private state.

Node A is disposable. It should stop being an old `choiros-rs` host and become a `go-choir` design lab. Node B remains the production source of truth. The branch is the bridge back to production after human review.

## Owner QA Redirect - 2026-05-29

The previous run reached a deployed Node A checkpoint but stopped too early. Prompt polish, Node A health, and a couple of shell screenshots are not mission completion.

Owner corrections now define the active loss function:

- Do not let any desktop app icon go underneath the PromptSurface or off screen; under small or magnified viewports, icons must reflow and remain visible.
- DeskSheet must be dense enough to show the whole menu without scrolling in normal desktop and compact QA viewports; a scrollable Desk menu is a design failure, not a fallback.
- Nothing in DeskSheet, the prompt tray, app cards, buttons, labels, tabs, or app bodies may overlap.
- All app surfaces must use the same visual system. VText still showed button outlines; similar old controls across other apps must be found and retokenized.
- Settings must open while logged out because theme switching is required for QA.
- Email, Files, and every other app shell must open while logged out with frontend-only fixture/demo state. Auth belongs at protected action time, not app-open time.
- All three themes must be visible, switchable, and QA'd while logged out.
- Mobile app switching must be solved. TetraMark must still open DeskSheet and show the app tray; on mobile-sized viewports while the sheet is open, the prompt field should temporarily become icons for the currently open apps, so the old mobile app tray behavior is preserved inside the new PromptSurface model.
- Trace must include swimlanes, not only run cards or summary panels. The reference mockup shows horizontal lanes per agent/tool, bars for duration, dots for moments, failure ticks, and selection linked across graph, lane chart, and inspector.
- The final third of the trajectory must include a deliberate per-theme convergence pass against the reference mockups, not only generic token cleanup.

This redirect supersedes any earlier interpretation of "major apps" as "some apps." The acceptance surface is every app in the app registry, plus the shell and auth boundary.

## Current Belief State

Known:

- Node A now serves `go-choir` at `https://choir-ip.com` from branch `codex/redesign-hard-cutover-node-a`.
- The latest deployed health evidence before this redirect reported commit `00a3000d6105b8415b56be1e68ce853a230a7860`.
- The old `choiros-rs` runtime path was removed/disabled during the earlier Node A cutover.
- `node-b` currently runs `go-choir` services for `choir.news`.
- `go-choir` Node B auth is configured for WebAuthn RP ID `choir.news`, so passkey/auth DB cloning to `choir-ip.com` is not a valid equivalence target.
- The redesign asset bundle includes a hard-cutover brief, Svelte snippets, theme tokens, three theme presets, TetraMark assets, selector mapping, and desktop/trace/theme reference mockups.
- Owner QA has found that the current Node A cut is not mission-complete: some app opens are still auth gated, Settings is not reliably available logged out for theme switching, VText and likely other app controls still carry old outline styling, DeskSheet can overlap in compressed viewports, and desktop icons need prompt-safe reflow.

Uncertain:

- Which app-opening paths still call `requestAuth` before showing a logged-out fixture shell.
- Which apps still use old borders, old button treatment, or app-local visual systems instead of the theme tokens.
- Whether all app surfaces remain readable and non-overlapping under all three themes and compact/magnified viewports.
- Whether any fixture/demo state accidentally crosses into authenticated persistence paths.
- Whether the current uncommitted icon-safe patch should be retained as-is after full Computer Use QA or adjusted with the DeskSheet overlap fix.

Highest-impact uncertainty:

Can the deployed Node A branch become a coherent logged-out product review surface where every app and every theme is visible without auth, while protected actions still request auth at the moment of durable/private mutation?

Next high-information probe:

Audit app launch/auth gates, theme switching, and app-local styling; then fix the smallest set of shell/app boundaries that makes Settings and every app registry entry open logged out with fixtures before doing another Computer Use full-surface QA pass.

## Cognitive Transforms

### Via Negativa

The redesign should become real by deleting misleading structures:

- Delete old Node A runtime traces from the running host.
- Delete or hard-rename `BottomBar` production implementation.
- Delete bottom-only selectors and CSS variables.
- Delete old theme presets from the product theme list.
- Delete old auth-wall UX that prevents seeing the product.
- Delete dashboard noise where the product should feel like an operating surface.
- Delete Chyron metadata line noise and unbounded streaming behavior.

Expected signature: meaningful deleted-code diffstat, not only additive panels.

### Real Object

The artifact is not "pretty Svelte components." It is a real browser product at `https://choir-ip.com` that a human can use with Computer Use and manual QA tomorrow morning.

Local screenshots help iteration, but do not define success. Node A deployed behavior defines success for this mission.

### Clone vs Fork

Clone Node B's platform shape, not Node B's identity state.

Do clone or reproduce:

- `go-choir` service topology where practical.
- Svelte frontend deployment model.
- Caddy edge shape.
- health endpoint expectations.
- app shell and routing behavior.

Do not clone:

- `choir.news` passkeys/auth DB as a correctness target.
- production mail routing or Resend inbound/outbound behavior.
- private production user data.
- Node B deploy authority.

Node A is a fresh-auth fork and a frontend design lab.

### Taste as Verifier

Quality is primary. Playwright can catch broken selectors, but it cannot judge whether the interface feels coherent. Computer Use, screenshots, and human-visible interaction are first-class evidence.

The verifier should ask:

- Does this feel like an operating surface rather than a SaaS dashboard?
- Are the important things calm, clear, and touchable?
- Does mobile feel designed, not squeezed?
- Does the logged-out preview make Choir legible before authentication?
- Do animations clarify state instead of adding noise?

### Hard Cutover as Naming Truth

If the product primitive is `PromptSurface`, the code should say `PromptSurface`. Keeping `BottomBar` with new styling preserves the wrong ontology. This mission should make names match the new product model.

## Invariants

Production isolation:

- Do not deploy to Node B.
- Do not push redesign code to `main`.
- Do not mutate `https://choir.news`.
- Do not change `choir.news` DNS, Gandi records, Resend domain state, or mail routing.
- Do not use manual Node B deploy shortcuts.

Node A disposability:

- Node A rollback safety is not required.
- It is acceptable to remove old `choiros-rs` runtime state from Node A.
- It is acceptable to fresh-install `go-choir` state on Node A.
- Still record enough pre-wipe facts to understand what was removed.

Frontend scope:

- Prefer Svelte-only changes.
- Backend changes are allowed only when current logic prevents logged-out app shells from rendering or when Node A configuration requires it.
- Do not expand backend product features to satisfy mockups.
- Demo data is frontend-owned fixture state, not backend proof.

Auth and logged-out preview:

- Logged-out users should see the desktop shell and major apps.
- Auth should be requested at protected action time, not as a front-door wall.
- Protected actions include durable/shared/private mutation, provider/model spend, account/private data access, publish, send, import, activate, rollback/roll-forward, account settings, and owner-scoped state.
- Logged-out editing can be local/ephemeral. For example, VText can allow typing, formatting, and previewing, but saving/revising/publishing requires auth.

TypeScript:

- New/redesigned Svelte components use `<script lang="ts">`.
- New/redesigned view-model helpers use `.ts`.
- Directly touched JS modules may convert to TS when it clarifies the cutover.
- Do not perform an unfocused repo-wide TypeScript migration.

Fixture honesty:

- Mock/demo data must never be written to authenticated user state.
- Mock/demo data must never be sent to backend APIs as proof.
- The final report must distinguish preview fixture evidence from live API evidence.
- Do not claim live agent behavior from fixture animations.

Design:

- Use `docs/choir-redesign-hard-cutover-assets/` as direction and constraints, not as a backend feature spec.
- Keep visual density appropriate for an operating surface.
- No marketing landing page. The app itself is the first screen.
- No nested-card slop, decorative orb backgrounds, or one-note palettes.
- Text must fit on mobile and desktop.
- Typography, page titles, favicon, empty states, loading states, and tiny details count.

## Hard Cutover Requirements

### Node A Runtime

Target:

- `https://choir-ip.com` serves `go-choir` from branch `codex/redesign-hard-cutover-node-a`.
- Old `choiros-rs` runtime service is disabled/removed from active service path.
- Old `choir-ip.com` reverse proxy to `127.0.0.1:9090` is replaced by the `go-choir` edge route.
- Public health confirms the deployed branch commit.

Expected Node A differences from Node B:

- Fresh auth state is acceptable.
- No production mail routing required.
- No production Resend inbound/outbound proof required.
- Demo fixtures support logged-out visual review.

### Branch CI

Target:

- Pushes to `codex/redesign-hard-cutover-node-a` deploy to Node A.
- Pushes to this branch must not deploy Node B.
- Existing `main` workflow continues to deploy only Node B.
- Node A deploy identity is recorded in health or deploy logs.

If GitHub secrets are missing:

- Stop the CI part with exact missing secret names or alternatives.
- Do not replace branch CI with untracked manual Node A mutation as the final mission path.
- A one-time exploratory SSH inventory is acceptable; production behavior should come from branch deploy once configured.

### Prompt Surface

Delete or replace:

- `frontend/src/lib/BottomBar.svelte`
- `BottomBar`
- `bottomBarEl`
- `bottomBarHeight`
- `BOTTOM_BAR_HEIGHT`
- `.bottom-bar`
- `[data-bottom-bar]`
- `--choir-bottom-bar-height`
- old start-menu selectors as production selectors.

Introduce:

- `frontend/src/lib/PromptSurface.svelte`
- `frontend/src/lib/DeskSheet.svelte`
- `frontend/src/lib/TetraMark.svelte`
- `[data-prompt-surface]`
- `[data-desk-menu-button]`
- `[data-desk-sheet]`
- `[data-desk-sheet-app]`
- `[data-window-tray-item]`
- `[data-online-indicator]`
- `--choir-prompt-surface-size`
- `--choir-prompt-surface-top-offset`
- `--choir-prompt-surface-bottom-offset`

Behavior:

- PromptSurface supports top and bottom placement.
- DeskSheet opens away from the surface: upward in bottom mode, downward in top mode.
- Window geometry respects top/bottom prompt surface offsets.
- Prompt input remains usable and visually central.
- Window tray is useful but not visually noisy.
- On mobile-sized viewports, TetraMark still opens DeskSheet and shows the app tray. While the sheet is open, the prompt input area becomes a compact open-app switcher until an app is chosen, the sheet is dismissed, or the user returns to prompt entry.
- TetraMark replaces the old four-square desk glyph.

### Theme System

Replace the current schema-v1 theme set with schema v2.

Exactly three product themes:

- `futuristic-noir`
- `carbon-fiber-kintsugi`
- `london-salmon`

Remove product exposure of old presets:

- `system-noir`
- `next-workstation`
- `classic-mac`
- `aqua-glass`
- `frutiger-aero`
- `gtk-slate`
- `y3k-console`

Legacy stored themes normalize to `futuristic-noir`.

Theme tokens should cover shell, windows, sheets, Auth, Overview, Compute Monitor, Trace, Podcast, VText, Files, Email preview, and media apps.

Theme convergence pass:

- Run this toward the end of the trajectory, after all apps open logged out and the overlap/navigation problems are fixed.
- Compare against `docs/choir-redesign-hard-cutover-assets/reference-mockups/trace-reference.png`, `carbon-fiber-kintsugi-suite.png`, `london-salmon-suite.png`, `desktop-overview-reference.png`, and `prompt-surface-reference.png`.
- Futuristic Noir: recover the reference Trace/PromptSurface language: dark navy glass, restrained luminous cyan/blue depth, sparse panels, soft shadows, and no hard outline grid.
- Carbon Fiber Kintsugi: recover dark industrial carbon material, precise expensive controls, gold repair seams/glow as emphasis, and mechanical density without orange/brown drift.
- London Salmon: recover salmon paper, oxblood/ink accents, serif heading character, broadsheet/Savile Row restraint, and avoid cute pastel or beige SaaS.
- For each theme, inspect PromptSurface, DeskSheet, Settings, VText, Trace, Files, Email, Compute Monitor, Podcast, and at least one media viewer before considering the pass complete.

### Auth UX

Auth is part of the redesign, not a separate wall.

Requirements:

- Redesign `AuthEntry.svelte` to match the new visual system.
- Improve auth copy, typography, spacing, and tiny details.
- Explain auth at protected action time with concrete copy such as saving, running, sending, importing, or publishing.
- Avoid implementation-heavy phrases.
- Make signed-out first impression feel like a usable preview, not a lockout.
- Make page title and favicon feel intentional.

### Liberal Logged-Out Preview

Logged-out mode should be a high-fidelity preview desktop.

Visible logged out:

- Desktop shell.
- PromptSurface and DeskSheet.
- App launcher.
- Auth UI.
- VText editor shell with local draft typing.
- Trace with fixture trajectories and swimlanes.
- Files with fixture filesystem.
- Podcast with fixture subscriptions/episodes.
- PDF/EPUB/Image/Video/Audio fixture libraries.
- Compute Monitor with fixture telemetry.
- Email shell/preview fixture mailbox.
- Features/catalog preview if still present.

Require auth:

- Real prompt submission to agents.
- Model/provider-spending actions.
- Saving VText revisions.
- Publishing VText.
- Accessing private traces.
- Accessing real mailbox/drafts/send.
- Importing or activating Features.
- Mutating account/computer state.
- Uploading private files.
- Persisting media/library state to an account.

### Demo Fixtures

Add frontend fixture modules for logged-out preview surfaces.

Fixture targets:

- Trace:
  - several trajectories;
  - agents, tool calls, failures, findings, searches;
  - graph nodes;
  - timeline/swimlane events;
  - inspector details.
- VText:
  - several docs;
  - version history;
  - v1 -> v2 -> v3 progression;
  - animated "new revision" or diff highlight.
- Files:
  - folder tree;
  - mixed file types;
  - timestamps/sizes;
  - preview metadata.
- Podcast:
  - subscriptions;
  - episodes;
  - playback progress;
  - now playing.
- PDF, EPUB, Image, Video, Audio:
  - demo library items;
  - thumbnails or generated/static previews where practical;
  - realistic metadata.
- Compute Monitor:
  - pressure samples;
  - restore-weight samples;
  - window state samples;
  - recent events.
- Chyron:
  - synthetic live event stream;
  - bounded lifecycle;
  - no raw IDs or metadata line noise;
  - settles/stops on completion.
- Email:
  - preview mailbox/draft states only;
  - no fake send proof.

Fixtures may use web-sourced or generated content where licensing is safe and local asset size stays reasonable. Avoid remote-loading assets at runtime unless intentionally part of the product behavior.

### App Redesign Targets

Desktop Overview:

- Switcher-first, not dashboard-first.
- Active window hero.
- Background/paused/hibernated cards.
- Mobile cards primary; live screenshots are accents.
- Remove user-facing "layer 1/layer 2" language.

Compute Monitor:

- Diagnostic and temporal.
- Charts/gauges/samples.
- Top contributors and recent recovery events.
- Do not duplicate Desktop Overview window management as the primary UI.

Trace:

- Visual-first.
- Run graph plus timeline/swimlanes.
- Agent/tool nodes are visually distinct.
- Delegation and data-flow edges differ.
- Failure states visible at a glance.
- Inspector is useful but not dominant.
- Mobile uses tabs/panels: Runs, Graph, Timeline, Inspector.
- The swimlane/timeline chart is required. It must use horizontal lanes per agent/tool, duration bars, moment dots, failure ticks, and a visible now/selection marker where useful.
- Selecting a graph node, lane item, or timeline moment should update the selected inspector detail and visually link the related graph/lane state.
- Logged-out Trace fixtures must include enough agent/tool diversity and failures to exercise the swimlane design honestly.

VText:

- Logged-out editable local preview.
- Clear save/revise/publish auth boundaries.
- Version progression animation.
- Better version pills and revision-state copy.
- Avoid dense technical metadata in the primary reading surface.

Files and media:

- Fixture library visible logged out.
- Calm file browser hierarchy.
- File/media viewers should feel native to the shell.
- Empty/loading/error states get polished copy.

Podcast:

- Sparse and calm.
- Search/import secondary.
- Now playing clean.
- Mobile single-column with sticky mini-player.

Email:

- Preview shell is visible logged out.
- Real mailbox/send/drafts require auth.
- Keep email preview copy clean and human.
- No raw hashes or metadata line noise in user-facing messages.

Features:

- If touched, keep video-first/import language from the previous product direction.
- Do not reintroduce raw AppChangePackage/adoption/promotion language in happy-path UI.

## Homotopy Axes

Work should increase realism along these axes without changing the artifact identity:

1. Environment realism:
   - local visual iteration -> Node A live shell -> Node A branch CI deploy.
2. Logged-out preview realism:
   - static fixtures -> animated fixture state -> protected-action auth prompts.
3. Redesign completeness:
   - PromptSurface/theme/auth -> core app redesigns -> media and tiny details.
4. Verification realism:
   - build/typecheck -> local browser -> Computer Use on Node A desktop/mobile -> owner QA.
5. Merge-back readiness:
   - isolated branch -> clean deleted-code diffstat -> documented residuals -> reviewed branch ready for main PR.

Do not jump to Node B rollout inside this mission.

## Receding-Horizon Control

Operate in bounded loops:

1. Observe current state.
2. Predict the next evidence change.
3. Make the smallest coherent mutation.
4. Run focused verification.
5. Use Computer Use to inspect the visible surface when UI changed.
6. Update belief state in the mission doc when evidence changes materially.
7. Continue, narrow, or stop honestly.

Suggested control intervals:

- Node A inventory and branch setup.
- Node A go-choir serving proof.
- PromptSurface/theme hard cut.
- Logged-out preview rule.
- Fixture data and Chyron/VText animation.
- Core app redesign pass.
- Mobile pass.
- Quality/deletion pass.
- Final evidence package.

## Evidence Ledger Requirements

For each major claim, record:

- claim;
- evidence source;
- command or Computer Use observation;
- branch/commit;
- URL;
- result;
- caveat.

Required evidence:

- Node A pre-wipe runtime facts.
- Node A `go-choir` health and deployed commit.
- Branch CI run identity or precise blocker.
- `https://choir-ip.com` visual proof on desktop viewport.
- `https://choir-ip.com` visual proof on mobile-sized viewport.
- PromptSurface top and bottom behavior.
- DeskSheet behavior.
- Auth UI screenshot/observation.
- Logged-out VText local editing observation.
- Logged-out protected-action auth prompt observation.
- Trace fixture graph/swimlane observation.
- Trace swimlane/timeline observation with bars, dots, failure ticks, and linked inspector selection.
- VText version progression observation.
- Chyron bounded ticker observation.
- Three-theme observations.
- Frontend build/typecheck result.
- Focused Playwright result where selector behavior changed.
- Deleted-code diffstat.

## Anti-Goodhart Rules

- Do not count "component exists" as proof that the UI works.
- Do not count local-only screenshots as Node A proof.
- Do not count fixture animation as live agent behavior.
- Do not preserve old selectors just to keep old tests passing.
- Do not add panels to satisfy a mockup if deletion/simplification would improve the product.
- Do not spend the mission on backend feature expansion.
- Do not add email/password auth in this mission.
- Do not wire Node A mail to Resend as a side quest.
- Do not make a landing page instead of the app surface.
- Do not optimize for pixel-perfect mockup recreation over coherent product behavior.

## Verification Plan

Automated:

- `cd frontend && pnpm install --frozen-lockfile` if dependencies are not present.
- `cd frontend && pnpm build`.
- TypeScript/Svelte checks if the repo has an existing check command.
- Focused Playwright tests for:
  - new PromptSurface selectors;
  - top/bottom placement;
  - DeskSheet opening direction;
  - logged-out app shell rendering;
  - old selector absence where practical.

Manual/Computer Use:

- Open `https://choir-ip.com`.
- Inspect first paint, favicon, page title, loading state.
- Inspect Auth UI.
- Use logged-out desktop.
- Open DeskSheet.
- Verify DeskSheet shows the complete menu without internal scrolling or overlap in desktop and compact viewports.
- Verify mobile app switching: with multiple windows open, tap TetraMark and confirm DeskSheet opens, the app tray is visible, the prompt field is replaced by open-app icons, switching works, and prompt entry can return without overlap.
- Open VText, type locally, try protected save/revise/publish and verify auth prompt.
- Open Trace and inspect graph/timeline/swimlane.
- In Trace, verify swimlanes are visible and interactive enough to link graph/timeline/inspector state.
- Open Desktop Overview.
- Open Compute Monitor.
- Open Podcast.
- Open Files and media apps.
- Switch all three themes.
- Near the end, run one Computer Use pass per theme against the reference mockups and record theme-specific residual issues.
- Test mobile-sized viewport visually.
- Watch Chyron and VText version animation complete without metadata line noise or stuck streaming.

## Stop Conditions

Complete:

- Node A serves branch `go-choir` at `https://choir-ip.com`.
- Redesign hard cut is implemented enough for owner QA across every app.
- Logged-out preview rule works across every app registry entry, not only major surfaces.
- Three themes are coherent and can be switched while logged out through Settings.
- Each theme has received a late convergence pass against the reference mockups and has app-by-app Computer Use observations.
- PromptSurface/DeskSheet/TetraMark are real production code.
- Auth UI is redesigned.
- Fixture-backed app surfaces are visually reviewable.
- Trace includes a visible swimlane/timeline chart with bars, dots, failure markers, and linked selection.
- DeskSheet is dense and fully visible without internal scrolling in normal desktop and compact QA viewports.
- No visible overlap remains in app trays, DeskSheet, window chrome, app bodies, or controls.
- Desktop icons stay inside the prompt-safe viewport and reflow under small or magnified viewports.
- Mobile app switching works from TetraMark without reintroducing the old BottomBar or hiding open apps behind the prompt.
- Computer Use desktop/mobile observations are recorded.
- Build/typecheck/focused tests are reported.
- `choir.news` and `main` remain untouched.

Checkpoint incomplete:

- Node A go-choir serving works, but redesign is partial.
- Redesign works locally but Node A CI/deploy is blocked by missing GitHub secrets.
- Logged-out preview works for core shell but not all media apps.
- Visual quality is below owner-review threshold but the next probe is clear.

Blocked incomplete:

- GitHub/Node A deploy requires unavailable secrets or owner action.
- Node A cannot build/run `go-choir` due host capacity or OS mismatch after root-cause probes.
- Passkey user-presence is required for deeper private-state proof and logged-out preview cannot substitute for the claim being tested.

Superseded:

- Owner decides to skip Node A and perform the redesign directly on Node B/main.
- The branch reveals that the frontend architecture requires a broader product/app runtime redesign before visual cutover makes sense.

## Morning Review Package

The final report should include:

- branch name;
- commit SHAs;
- Node A URL;
- Node A deployed commit and health;
- CI run URL/status or exact deploy blocker;
- Computer Use observations;
- screenshots or screenshot paths if captured;
- theme status;
- logged-out preview status;
- auth UX status;
- old-code deletion summary;
- old selectors/names remaining, if any;
- build/typecheck/test status;
- what is ready for owner QA;
- what should wait for merge-back/main;
- explicit statement that Node B/`choir.news` was not touched.

## Suggested Merge-Back Policy

If Node A reaches 90%+ visual/product quality by Computer Use and owner manual QA, prepare the branch for merge-back to `main` in a separate landing mission.

Merge-back should:

- preserve the hard-cut names;
- carry fixture preview behavior only where it improves logged-out product UX;
- reverify production auth/mail/private paths on Node B;
- remove Node A-only deployment workflow if it should not persist;
- update canonical frontend docs only after owner approval.

Do not merge back during this Node A mission unless the owner explicitly changes the mission.

## Run Evidence Ledger

### 2026-05-29T06:47Z - Node A Pre-Wipe Inventory

Claim: Node A is still the stale `choiros-rs` host and is safe to treat as the disposable design lab target.

Evidence:

- Branch at inventory time: `codex/redesign-hard-cutover-node-a`.
- `ssh node-a 'hostname'` returned `choiros-a`.
- Running services included `caddy.service`, `hypervisor.service`, `cloud-hypervisor@u-b59f2836.service`, and `socat-sandbox@u-b59f2836.service`.
- `systemctl is-active` reported `caddy.service=active`, `hypervisor.service=active`, and all checked `go-choir-*` services inactive.
- `/opt/choiros` and `/var/lib/choiros` existed; `/opt/go-choir` and `/var/lib/go-choir` did not.
- `/etc/caddy/caddy_config` served `choir-ip.com` by `reverse_proxy 127.0.0.1:9090`.
- Root disk had about `888G` available, enough for a fresh go-choir design lab build/deploy.

Caveat: public DNS for `choir-ip.com` still resolved to `147.135.24.51` at probe time, while `node-a` reported public address `51.81.93.94`; direct `--resolve choir-ip.com:443:51.81.93.94` reached the old Caddy-served site.

### 2026-05-29T06:48Z - Branch CI Deploy Problem

Claim: branch CI cannot yet deploy to Node A through the existing workflow shape without a deploy-path adjustment.

Evidence:

- `gh secret list --repo choir-hip/go-choir` listed only `OVH_DEPLOY_SSH_KEY` and `OVH_NODE_B_HOST`.
- No `OVH_NODE_A_HOST` or equivalent Node A host secret existed.
- Node A `/etc/nixos/configuration.nix` authorized `github-actions-deploy@choiros`, while tracked go-choir Node B config authorizes `github-actions-deploy@go-choir`.

Belief-state update:

- The initial branch deploy should either use a public Node A host constant plus the existing `OVH_DEPLOY_SSH_KEY`, after Node A is switched to a go-choir config that authorizes that deploy key, or stop with the exact missing secret/key blocker.
- A one-time local SSH cutover remains inside the mission authority because Node A is disposable, but the final recurring path should be branch CI once the host accepts the repo deploy key.

Remaining error field:

- Need implement a tracked `go-choir-a` NixOS/deploy path that uses `choir-ip.com`, fresh Node A state, the go-choir deploy public key, and no Node B auth/mail identity.
- Need verify whether DNS/proxying for `choir-ip.com` reaches Node A normally or requires owner-side DNS after direct Node A proof.

## Run Checkpoint & Resumption State

status: checkpoint_incomplete

last checkpoint: 2026-05-29T08:12Z local owner-QA redirect pass after the all-app/all-theme logged-out fixes, before the next Node A deploy.

current artifact state: local branch code now opens every app registry entry while logged out with frontend-only demo/preview state; Settings opens logged out and switches exactly the three schema-v2 themes; Trace has visible swimlanes; DeskSheet is dense and non-scroll in normal and mobile-sized support checks; TetraMark still opens DeskSheet and, on mobile while the sheet is open, replaces the prompt field with open-app icons. Node A still serves the prior deployed commit until the next branch workflow completes.

what shipped: prior Node A branch deploy path, hard-cut shell, schema-v2 theme set, PromptSurface/DeskSheet/TetraMark, initial logged-out VText/Trace preview, and prompt geometry polish.

what was proven: locally, `pnpm build` passed; the focused hard-cutover Playwright suite passed 5/5; Computer Use observed desktop/compact DeskSheet and Trace swimlanes; support screenshots covered all three themes and mobile Tetra switching. Node A deployment proof for this checkpoint is still pending.

unproven or partial claims: live Node A proof for this checkpoint, deeper owner visual approval across every app body in all three themes, full interactive mobile Computer Use proof, and merge-back readiness.

belief-state changes: the live deployment route works; the owner QA redirect exposed product completeness gaps rather than infrastructure gaps; local evidence now shows the major logged-out visibility, theme switching, mobile switching, and swimlane requirements are implemented.

remaining error field: live Node A deploy/health identity for the latest commit, deployed Computer Use screenshots/observations, residual theme taste gaps, and possible fixture honesty gaps.

highest-impact remaining uncertainty: whether the deployed Node A build matches the local 90%+ reviewable cut under real browser/network conditions.

next executable probe: commit and push only the intended branch changes, let the Node A redesign workflow deploy them, then verify `https://choir-ip.com` health/build identity and repeat deployed desktop/mobile visual QA.

suggested resume goal string: use the `Goal String` section above.

evidence artifact refs: this mission doc; local screenshots `/tmp/choir-local-futuristic-noir.png`, `/tmp/choir-local-carbon-fiber-kintsugi.png`, `/tmp/choir-local-london-salmon-v2.png`, and `/tmp/choir-local-mobile-tetra.png`; previous deployed screenshot refs `/tmp/choir-prompt-polish-deployed-desktop.png` and `/tmp/choir-prompt-polish-deployed-mobile.png`.

rollback refs: Node A disposable; branch previous deployed commit `00a3000d6105b8415b56be1e68ce853a230a7860`; no Node B/main mutation performed.

### 2026-05-29T07:16Z - Node A Frontend Nix Dependency Blocker

Claim: the first branch CI deploy reached Node A and built the host closure, but failed before activation because the frontend Nix derivation did not have a package-lock cache entry for the newly added TypeScript dependency.

Evidence:

- GitHub Actions run `26623653598` on branch `codex/redesign-hard-cutover-node-a` reached job `Deploy Node A Design Lab`, step `Deploy branch to Node A`.
- The remote deploy cloned `/opt/go-choir`, reset it to commit `870d4d4`, disabled stale `choiros-rs` services, removed stale `/opt/choiros` and `/var/lib/choiros`, and built `.#nixosConfigurations.go-choir-a.config.system.build.toplevel`.
- The deploy then failed during `nix build .#frontend`.
- The Nix log reported `npm error code ENOTCACHED` and `request to https://registry.npmjs.org/typescript failed: cache mode is 'only-if-cached' but no cached response is available`.
- `flake.nix` documents that local frontend development uses `pnpm-lock.yaml`, while the Nix build uses `frontend/package-lock.json` and `npmDepsHash`.

Belief-state update:

- The TypeScript cutover changed `package.json` and `pnpm-lock.yaml` but did not update `frontend/package-lock.json` and the flake `npmDepsHash`.
- This is a reproducibility/deploy packaging blocker, not a product-path frontend runtime blocker.
- The correct fix is to update the npm lockfile and Nix npm dependency hash in a follow-up code commit, then rerun the same branch-scoped Node A deploy workflow.

Remaining error field:

- Need update `frontend/package-lock.json` for `typescript`.
- Need update `flake.nix` `npmDepsHash` from the Nix build error after the lockfile changes.
- Need rerun branch CI and then verify Node A health and visual behavior.

### 2026-05-29T07:21Z - Node A Deployed Review Cut

Claim: Node A is now a disposable `go-choir` design lab serving the redesign branch at `https://choir-ip.com`; Node B and `main` were not touched.

Evidence:

- Branch: `codex/redesign-hard-cutover-node-a`.
- Deployed commit: `cd1955c9ff2b24a6cd3100a81ba81a4ccaa4cb46`.
- GitHub Actions run: `https://github.com/choir-hip/go-choir/actions/runs/26623900052`, status `completed`, conclusion `success`, head SHA `cd1955c9ff2b24a6cd3100a81ba81a4ccaa4cb46`.
- Node A deploy env: `CHOIR_DEPLOYED_AT=2026-05-29T07:19:19Z`, `CHOIR_DEPLOYED_BRANCH=codex/redesign-hard-cutover-node-a`, `CHOIR_DEPLOYED_COMMIT=cd1955c9ff2b24a6cd3100a81ba81a4ccaa4cb46`.
- Stale state deletion: `/opt/choiros` absent and `/var/lib/choiros` absent on Node A after deploy.
- Stale services: `hypervisor.service` not found/inactive; old `cloud-hypervisor@u-b59f2836.service` and `socat-sandbox@u-b59f2836.service` inactive.
- Active services: `caddy`, `go-choir-auth`, `go-choir-platformd`, `go-choir-platform-dolt`, `go-choir-gateway`, `go-choir-sandbox`, `go-choir-vmctl`, `go-choir-proxy`, and `go-choir-maild` all reported active.
- Public health: `curl -fsS https://choir-ip.com/health` returned proxy and sandbox health with deployed commit `cd1955c9ff2b24a6cd3100a81ba81a4ccaa4cb46`.
- Public HTML: `curl -fsS https://choir-ip.com/` returned `title>Choir Design Lab</title>` and the new inline TetraMark favicon.
- Direct Node A health with `--resolve choir-ip.com:443:51.81.93.94` also returned the same deployed commit.
- DNS caveat: `dig +short choir-ip.com A` returned `147.135.24.51` while `node-a` still reports `51.81.93.94`; nevertheless public `https://choir-ip.com` served the deployed build and health at evidence time.

Visual observations:

- Computer Use desktop observation on deployed `choir-ip.com`: PromptSurface visible at the bottom, VText Preview and Trace Preview open while logged out, fixture-backed VText and Trace content visible, and protected actions such as Publish/Sign in presented at action boundaries.
- Computer Use Desk observation on deployed `choir-ip.com`: DeskSheet opened from PromptSurface, showed Desktop Overview, all major app entries, `PUBLIC PREVIEW`, and `Sign in`.
- Screenshot refs: `/tmp/choir-node-a-desktop.png` and `/tmp/choir-node-a-mobile.png`.
- Mobile-sized Playwright screenshot support pass: the 390px viewport rendered the shell, VText/Trace preview windows, PromptSurface, and app icons without a blank screen.

Local regression evidence:

- `pnpm build` passed in `frontend/`; only existing chunk-size warnings remained.
- Focused Playwright cutover smoke passed: `PLAYWRIGHT_BASE_URL=http://localhost:5173 pnpm exec playwright test tests/prompt-surface-hard-cutover.spec.js --project=chromium --reporter=line` with `2 passed`.
- Nix config probes passed for `go-choir-a` hostname, `choir-ip.com` auth RP environment, and Caddy virtual host.

Residual visual/product issues:

- PromptSurface/Chyron ticker is bounded, but the desktop accessibility tree and screenshots still show repeated ticker copy over time; this should be cleaned before merge-back.
- Mobile review used Playwright screenshot support because Comet could not be reliably resized by Computer Use; mobile is reviewable but not yet as thoroughly interactively verified as desktop.
- The app icon set still uses emoji-style desktop icons in several places; this is acceptable for the Node A review cut but should be revisited for a polished merge-back.
- Node A hostname still reports `choiros-a` immediately after switch even though the active NixOS config is `go-choir-a`; verify after reboot or hostname service reload before treating the machine identity as clean.

Remaining error field:

- Owner QA should review `https://choir-ip.com` directly and decide whether this is the right visual direction.
- Before merge-back to `main`, clean the ticker repetition, do deeper mobile Computer Use or device proof, decide whether to keep Node A workflow/assets in the branch, and reverify production auth/mail/private paths on Node B in a separate landing mission.

### 2026-05-29T08:12Z - Owner QA Redirect Local Cut

Claim: the local branch now satisfies the owner redirect well enough to redeploy Node A for another review cut.

Evidence:

- Build: `pnpm build` passed in `frontend/`; only the existing Vite chunk-size warnings remained.
- Focused regression: `PLAYWRIGHT_BASE_URL=http://localhost:5173 pnpm exec playwright test tests/prompt-surface-hard-cutover.spec.js --project=chromium --reporter=line` passed 5/5.
- No typecheck script exists in `frontend/package.json`; `pnpm check` returned "Command check not found", so the available Svelte/TypeScript regression proof is Vite build plus Playwright.
- Computer Use desktop observation on local Comet: TetraMark opened DeskSheet, DeskSheet showed the complete app tray without internal overlap or scrolling, and the prompt field was removed in compact switcher state while open-app icons remained visible.
- Computer Use Trace observation: Trace defaults to the Timeline panel in compact windows and shows the Agent graph plus Swimlanes; hard button outlines around graph nodes were removed by the app-content retokenization layer.
- Local Playwright support metrics: all app registry entries opened while logged out; Settings exposed 3 theme presets; Trace rendered 4 swimlanes and 7 swimlane moments; Terminal showed logged-out preview mode; desktop icons stayed above the PromptSurface in the 1440x900 fixture.
- Mobile support metrics at 390x844: DeskSheet visible, `sheetOverflow=0`, mobile app switcher visible, prompt input absent while switcher open, 3 switcher buttons all inside viewport.
- Theme support screenshots: `/tmp/choir-local-futuristic-noir.png`, `/tmp/choir-local-carbon-fiber-kintsugi.png`, `/tmp/choir-local-london-salmon-v2.png`, and `/tmp/choir-local-mobile-tetra.png`.

Changes made in response to owner QA:

- App-open auth gates were removed for logged-out app shells; protected actions still request auth at mutation/spend/account boundaries.
- Settings, Features, Terminal, VText-open-from-content, and Trace-open-from-content now have logged-out preview/demo behavior instead of app-open auth prompts.
- Trace now includes real swimlane rows with lane labels, duration bars, moment dots, failure ticks, and selected moment linkage to inspector state.
- PromptSurface keeps TetraMark as the DeskSheet opener; on mobile while the sheet is open, the command field becomes the open-app switcher.
- DeskSheet was made denser and non-scroll in the normal/compact support viewports.
- Desktop icons reflow within the PromptSurface-safe viewport under small/magnified layouts.
- App window controls and cards received a no-hard-outline retokenization layer; London Salmon Settings contrast was corrected after screenshot review.

Remaining before stopping:

- Commit and push this cut.
- Let the branch-scoped Node A workflow deploy it.
- Verify deployed health/build identity, deployed Computer Use observations, and deployed screenshots before claiming the live Node A review checkpoint.
