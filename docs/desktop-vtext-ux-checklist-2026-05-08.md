# Desktop and VText UX Checklist

Date: 2026-05-08

This checklist captures the current UX/debugging pass for the deployed desktop,
especially VText, Trace, Settings, browser cache invalidation, and future
theme/app extensibility. Do not treat stale deployed UI screenshots as product
truth until the cache/deploy identity checks pass.

## 1. Browser Cache and Deploy Identity

- [x] Ensure the deployed SPA shell is never browser-cached.
- [x] Serve `/assets/*` with long immutable caching because Vite fingerprints built JS/CSS assets.
- [x] Serve `index.html` and SPA fallback responses with `Cache-Control: no-store`.
- [x] Verify `curl -I https://draft.choir-ip.com/` returns `Cache-Control: no-store` after deploy.
- [x] Verify `curl -I https://draft.choir-ip.com/assets/<current-bundle>.js` returns `Cache-Control: public, max-age=31536000, immutable`.
- [x] Add a visible or hidden build identity surface: frontend git SHA, sandbox git SHA, proxy git SHA, and deploy timestamp.
- [x] Add a deployed Playwright assertion that the loaded JS bundle commit matches the server-reported commit.
- [x] Add a deployed Playwright assertion that the browser does not call removed/stale routes such as `/api/agent/topology`, `/api/prompts`, or `/api/events`.

## 2. VText Layout and Interaction

- [x] Redesign VText so the document body owns the main surface and controls never overlay text.
- [x] Move the version indicator and previous/next buttons into reserved chrome, either titlebar-adjacent or a compact document toolbar.
- [x] Move the `Revise` action out of the text area overlay, likely into a reserved footer/action rail or titlebar action.
- [x] Reserve explicit top/bottom padding only for readable document margins, not for floating controls.
- [x] Preserve minimalism: no comments sidebar, no Google Docs clone, no heavy collaboration chrome.
- [x] Make historical versions clearly read-only.
- [x] Make dirty/latest/historical state legible without adding chat-like status clutter.
- [x] Keep Trace internals out of the default VText writing UI; a hidden deep link to the relevant trajectory can come later.

## 3. Markdown and Pretext Editing

- [x] Keep canonical VText storage as Markdown text for now.
- [x] Render Markdown in read mode instead of showing raw Markdown markers.
- [x] Support editing the same Markdown content without losing source fidelity.
- [x] Start with a simple split between rendered read mode and focused edit mode if full WYSIWYG is too much for the first pass.
- [x] Spike Pretext as the layout/rendering layer before committing to it as the full editor engine.
- [x] Decide whether Pretext should own only document rendering/measurement or also interactive editing.
- [x] Add tests that Markdown headings, emphasis, lists, and links render correctly and remain editable.
- [x] Do not block the immediate overlap/layout fix on full Pretext integration.

Pretext decision for this pass: defer integration. Pretext is a text
measurement/layout library (`@chenglou/pretext`), useful later for fast
measurement, virtualization, shaped text flow, and precise rendered document
geometry. It should not be treated as the first VText editor engine. The
current pass keeps Markdown source fidelity with textarea editing and rendered
read mode. Sources: https://github.com/chenglou/pretext and
https://pretextjs.dev/pretext-library.

## 4. VText App Opening Behavior

- [x] When VText opens without a `docId`, show a recent VTexts landing view instead of a blank document.
- [x] Use existing `GET /api/vtext/documents` for the first recent-documents implementation.
- [x] Include document title, latest revision number, latest editor, and updated time.
- [x] Provide a minimal “new document” action.
- [x] Preserve prompt-bar-created VTexts as the primary creation path for agentic documents.
- [x] Add Playwright coverage for opening VText from the desktop icon and selecting an existing document.

## 5. Window Geometry and Responsive Behavior

- [x] Define app-specific default geometry in the app registry or window store, not scattered component logic.
- [x] Make VText default launch size near full available width/height on mobile, with small borders and prompt-bar clearance.
- [x] Make VText substantially larger by default on tablet and desktop.
- [x] Persist window size/position across reloads.
- [x] Repair persisted geometry on load when viewport size changes or old geometry is too small.
- [x] Prevent windows from opening underneath the bottom prompt bar.
- [x] Prevent window controls and resize handles from being unreachable on mobile.
- [x] Add visual regression checks for mobile, tablet, and desktop VText launch sizes.

## 6. Bottom Bar, Account Menu, and Sign Out

- [x] Remove the permanently prominent `Sign Out` button from the bottom bar.
- [x] Put account/session actions behind the “show desktop” or account/desktop menu.
- [x] Keep live connection status visible but visually quiet.
- [x] Preserve fast access to the prompt bar.
- [x] Add a Playwright test for opening the desktop/account menu and signing out from there.

## 7. Trace Rewrite/Fix

- [x] Confirm deployed Trace is running the current `/api/trace/*` frontend, not stale topology code.
- [x] Keep Trace read-only and auth-gated.
- [x] Do not reintroduce public `/api/agent/*` or raw `/api/events` for Trace.
- [x] Make the empty state say what the user/operator can do next.
- [x] Show trajectory list, agent graph, moment strip, message/tool details, and search-provider stats.
- [x] Make search endpoint success, rate limits, errors, and latency visible per trajectory.
- [x] Add Playwright coverage: after a prompt-bar VText run, opening Trace shows the trajectory with no 404s.
- [x] Add a regression test that Trace never calls stale removed routes.

## 8. Settings Rewrite/Fix

- [x] Remove the stale prompt-manager UI from product Settings.
- [x] Do not publicly expose prompt mutation APIs as normal product Settings.
- [x] Replace Settings v0 with safe product settings: account/session, theme selection, desktop reset, and read-only provider/search status.
- [x] Keep any prompt-policy editor dev/admin-only behind explicit gating.
- [x] Add Playwright coverage that Settings opens without 404s and does not call `/api/prompts` in normal product mode.

## 9. Themeability and App Creation Surface

- [x] Centralize design tokens as CSS variables: colors, radii, shadows, spacing, typography, and motion.
- [x] Move app metadata into one registry: id, title, icon, component, singleton/multi-instance behavior, default geometry, mobile geometry, and persistence behavior.
- [x] Reduce the number of files required to add a new app.
- [x] Make app shell components consume shared theme tokens instead of hard-coded colors.
- [x] Treat “redesign desktop with one prompt” as generating a validated theme/app-layout config, not arbitrary source edits.
- [x] Add a theme preview/test route or internal harness before exposing theme mutation to users.

## 10. Verification Plan

- [x] Run `git diff --check`.
- [x] Run frontend build after UI changes.
- [x] Run local Playwright for VText layout, recent-documents, Trace, Settings, and bottom-bar account menu.
- [x] Run deployed Playwright against `https://draft.choir-ip.com` after push/deploy.
- [x] Capture screenshots for mobile, tablet, and desktop sizes.
- [x] Verify the deployed response headers after deploy with `curl -I`.
- [x] Verify stale browser sessions update without manual cache clearing.

Deployment evidence, 2026-05-09:

- GitHub Actions run `25606669466` succeeded for commit `6d16df1204e46524b524fcfcdaeada63d553e6fd`; deploy job `Deploy to Staging (Node B)` completed successfully.
- `curl -I https://draft.choir-ip.com/` returned `Cache-Control: no-store`.
- The deployed JS bundle `/assets/index-BVn6_J4O.js` returned `Cache-Control: public, max-age=31536000, immutable`.
- `/health` reported proxy and sandbox build commit `6d16df1204e46524b524fcfcdaeada63d553e6fd` and deployed timestamp `2026-05-09T17:02:39Z`.
- `npx playwright test tests/deployed-origin-auth-shell.spec.js --project=chromium` passed: 12/12.
- `GO_CHOIR_RUN_DEPLOYED_LIVE_SEARCH=1 CHOIR_DEPLOYED_BASE_URL=https://draft.choir-ip.com npx playwright test tests/vtext-deployed-live-search.spec.js --project=chromium` passed: 1/1.
- Responsive screenshots captured:
  - `frontend/test-results/deployed-responsive-screenshots/mobile-vtext.png`
  - `frontend/test-results/deployed-responsive-screenshots/tablet-vtext.png`
  - `frontend/test-results/deployed-responsive-screenshots/desktop-vtext.png`
- Stale-browser update verification is covered by the deployed no-store shell header, immutable fingerprinted assets, frontend/server build identity match, absence of `/src/` scripts, and the successful post-deploy reload in the deployed Playwright suite. No service worker cache path is present.
