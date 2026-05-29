# MissionGradient: Choir Redesign Hard Cutover on Node A - v0

Status: draft for owner review  
Target branch: `codex/redesign-hard-cutover-node-a`  
Target host: `node-a`  
Target public URL: `https://choir-ip.com`  
Protected production host: `node-b` / `https://choir.news`  
Primary asset source: `docs/choir-redesign-hard-cutover-assets/`

## Goal String

```text
/goal Run docs/mission-redesign-hard-cutover-node-a-v0.md as a Codex-operated MissionGradient mission: create a disposable Node A design lab for a hard-cut Choir frontend redesign without touching choir.news or main. Work on branch codex/redesign-hard-cutover-node-a. Wipe the stale choiros-rs runtime from ssh alias node-a, disable old services, and deploy go-choir from this branch to https://choir-ip.com with fresh local state. Do not clone choir.news auth, passkeys, mail routing, Resend mail behavior, or production secrets unless already required for ordinary go-choir operation on Node A. Configure Node A for choir-ip.com fresh auth if needed, but do not make auth the proof bottleneck. Implement the liberal logged-out preview rule: the shell and major apps are visible and interactive with frontend mock/demo data while logged out, and auth is requested only for durable/shared/private mutation, provider spend, account data, publish/send/import/activate, or other owner-scoped actions. Then perform the Svelte + TypeScript hard cutover using docs/choir-redesign-hard-cutover-assets as design direction: delete BottomBar compatibility, introduce PromptSurface, DeskSheet, TetraMark, schema-v2 themes with exactly futuristic-noir, carbon-fiber-kintsugi, and london-salmon, convert new/redesigned components to <script lang="ts"> and directly touched view-model helpers to .ts where useful, retokenize the shell/windows/apps, redesign Auth, Desktop Overview, Compute Monitor, Trace, VText preview/version progression, Files, Podcast, PDF, EPUB, Image, Video, Audio, Email preview, and tiny browser details such as favicon/page titles/copy/typography. Use frontend-only demo fixtures for logged-out private surfaces, including Trace trajectories/swimlanes, VText docs/version animation, files, podcasts, media libraries, compute samples, and a cleaned bounded Chyron ticker. Demo fixtures must never be used as backend proof or written to authenticated user state. Use Computer Use as the primary visual verifier across desktop and mobile-sized viewports; use build/typecheck/Playwright only as regression support. Optimize for quality, deletion, taste, and coherence over speed. If Node A reaches a 90%+ reviewable visual/product cut by Computer Use and owner QA, prepare the branch for later merge-back to main; do not merge or deploy to Node B in this mission. Stop with Node A deployed evidence, screenshots/Computer Use observations, branch/commit/CI identity, deleted-code diffstat, residual visual issues, typecheck/build status, and notes for morning review.
```

## Mission Identity

This mission is not a theme swap and not a local mockup. The real artifact is a live alternate Choir product surface on `https://choir-ip.com`, running `go-choir` from a redesign branch, with enough real shell behavior and frontend demo fixtures that visual review can happen without relying on Node B or passkey-authenticated private state.

Node A is disposable. It should stop being an old `choiros-rs` host and become a `go-choir` design lab. Node B remains the production source of truth. The branch is the bridge back to production after human review.

## Current Belief State

Known:

- `node-a` currently reports hostname `choiros-a`.
- `node-a` currently runs `caddy.service` and `hypervisor.service`.
- `node-a` has old code/state under `/opt/choiros` and `/var/lib/choiros`.
- `node-a` Caddy currently serves `choir-ip.com` by reverse proxying `127.0.0.1:9090`.
- `node-b` currently runs `go-choir` services for `choir.news`.
- `go-choir` Node B auth is configured for WebAuthn RP ID `choir.news`, so passkey/auth DB cloning to `choir-ip.com` is not a valid equivalence target.
- The redesign asset bundle includes a hard-cutover brief, Svelte snippets, theme tokens, three theme presets, TetraMark assets, selector mapping, and desktop/trace/theme reference mockups.

Uncertain:

- Whether GitHub Actions already has the secrets needed to deploy to Node A.
- Whether the existing Node B deploy workflow can be parameterized cleanly for Node A, or should be copied into a separate branch-only Node A workflow.
- Whether Node A has enough disk/build capacity for the same host/frontend/guest artifacts as Node B.
- Whether fresh `choir-ip.com` passkey registration can be completed unattended. This should not block logged-out visual proof.
- How much of the existing frontend can be cleanly converted to TypeScript during the cutover without creating a repo-wide migration tarpit.

Highest-impact uncertainty:

Can `https://choir-ip.com` become a branch-deployed `go-choir` frontend lab through CI without touching `choir.news` or `main`?

Next high-information probe:

Create the branch, inspect GitHub deploy secret feasibility for Node A, inspect Node A disk/services/Caddy, and establish the smallest CI or branch deploy route that can serve `go-choir` on `choir-ip.com`.

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
- Open VText, type locally, try protected save/revise/publish and verify auth prompt.
- Open Trace and inspect graph/timeline/swimlane.
- Open Desktop Overview.
- Open Compute Monitor.
- Open Podcast.
- Open Files and media apps.
- Switch all three themes.
- Test mobile-sized viewport visually.
- Watch Chyron and VText version animation complete without metadata line noise or stuck streaming.

## Stop Conditions

Complete:

- Node A serves branch `go-choir` at `https://choir-ip.com`.
- Redesign hard cut is implemented enough for owner QA.
- Logged-out preview rule works across major surfaces.
- Three themes are coherent.
- PromptSurface/DeskSheet/TetraMark are real production code.
- Auth UI is redesigned.
- Fixture-backed app surfaces are visually reviewable.
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

last checkpoint: Node A pre-wipe and deploy-secret inventory recorded before code or host mutation.

current artifact state: branch exists locally; Node A still runs old `choiros-rs`; frontend hard cutover not started.

what shipped: nothing yet.

what was proven: Node A stale runtime facts; GitHub secret/key deploy-path mismatch; `choir.news` and `main` untouched.

unproven or partial claims: Node A go-choir serving, branch CI, hard-cutover UI, logged-out preview, Computer Use observations.

belief-state changes: Node A has capacity; first deploy likely needs tracked Node A config plus local SSH bootstrap before branch CI can own subsequent deploys.

remaining error field: old Node A services/state, missing branch CI host secret, frontend redesign scope, DNS uncertainty.

highest-impact remaining uncertainty: whether `https://choir-ip.com` can be made to serve the branch from Node A without owner DNS action.

next executable probe: create tracked Node A deploy/config path and Svelte hard-cutover surface, then bootstrap Node A with local SSH and verify direct/public health.

suggested resume goal string: Continue `docs/mission-redesign-hard-cutover-node-a-v0.md` from the 2026-05-29 Node A deploy-path checkpoint; implement tracked Node A go-choir config, hard-cutover frontend, deploy to Node A, and verify with Computer Use.

evidence artifact refs: this mission doc.

rollback refs: Node A disposable; no Node B/main mutation performed.

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
