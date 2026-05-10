# Choir Origin/Main Change Report

Date: 2026-05-10

Repository: `go-choir`

Local branch: `main`

Base: `origin/main@5bd7da58a80f69761a6de75ac5d329d84b132f5b`

Local HEAD before the cleanup commit: `c92ff3c192311f7933a68a1b41b7d681f09320a4`

Status after Obscura cleanup: `main` was `3` commits ahead of `origin/main` with a smaller dirty worktree containing Choir auth/report/checklist changes. Those dirty changes are intended to become the fourth stacked commit before pushing. Obscura-specific docs, patch fragments, scripts, and sanitized audit summaries were migrated to `/Users/wiz/obscura` on the fork branch `choir/playwright-parity-audit-2026-05-10`.

## Executive Summary

The committed delta is focused and product-relevant: it improves mobile VText usability, removes the read/edit split in favor of one rendered-editable document surface, makes the prompt bar grow as a textarea, reduces mobile viewport zoom issues, and adds a planning checklist for the next VText/browser/VM work.

The dirty delta is now Choir-specific: this report, the reusable `auth:setup` helper, host-keyed Playwright auth-state reuse, and a checklist update pointing Obscura-specific audit evidence at the Obscura fork.

Push readiness is better after cleanup. The three committed UX/planning commits look coherent as a staging candidate. The remaining dirty auth/report/checklist changes should still be reviewed and committed separately if accepted.

## Ahead Commits

```text
c92ff3c Plan server-side browser with Obscura
5c5323c Document vtext next-step checklist
f08b77a Improve mobile vtext editing UX
pending fourth commit: preserve Choir auth setup and Obscura migration cleanup
```

Committed diff size:

```text
17 files changed, 496 insertions(+), 141 deletions(-)
```

Committed file inventory:

```text
docs/vtext-next-planning-checklist-2026-05-09.md
frontend/index.html
frontend/src/App.svelte
frontend/src/lib/AuthEntry.svelte
frontend/src/lib/BottomBar.svelte
frontend/src/lib/Desktop.svelte
frontend/src/lib/FloatingWindow.svelte
frontend/src/lib/VTextEditor.svelte
frontend/tests/desktop-shell-core.spec.js
frontend/tests/file-browser.spec.js
frontend/tests/gateway-e2e-deployed.spec.js
frontend/tests/vtext-agent-revision.spec.js
frontend/tests/vtext-authoring-history.spec.js
frontend/tests/vtext-deployed-live-search.spec.js
frontend/tests/vtext-document-stream.spec.js
frontend/tests/vtext-dry-run-plumbing-demo.spec.js
frontend/tests/vtext-real-workflow-demo.spec.js
```

## Committed Change Inventory

### Mobile VText Editor

Purpose: make VText usable on mobile without controls overlapping the document.

Changes:

- Replaced the old `textarea` read/edit split with one `contenteditable` rendered Markdown surface.
- Added Markdown-to-HTML rendering that remains directly editable.
- Added DOM-to-Markdown serialization for common inline/block structures.
- Made the VText toolbar a compact single control band and fade while scrolling.
- Updated tests from `toHaveValue` to content assertions because the editor is no longer a `textarea`.
- Added recent-document/new-document handling in tests so opening VText is deterministic.

Risk:

- `contenteditable` serialization is a stopgap, not a long-term document model. It handles basic Markdown structures but can drift on complex nested markup.
- Selection/caret behavior is not deeply tested yet.
- This is a good usability improvement but not a replacement for a future Pretext-backed editor.

### Mobile Desktop Viewport And Prompt Bar

Purpose: make the desktop behave more like a fixed app viewport on mobile.

Changes:

- Updated viewport metadata to use `interactive-widget=resizes-content`, `viewport-fit=cover`, and `maximum-scale=1`.
- Set `html`, `body`, and `#app` to fixed full-height app containers using `100dvh`.
- Prevented page overscroll and iOS-style input zoom by ensuring editable controls are at least `16px`.
- Replaced the prompt input with a growing `textarea`.
- Published bottom-bar height into `--choir-bottom-bar-height` so windows and the desktop can clear the prompt bar.
- Adjusted desktop and floating-window max-height calculations to respect dynamic bottom-bar height.

Risk:

- Mobile Safari keyboard behavior is browser-version-sensitive and still needs manual verification on real devices.
- `maximum-scale=1` improves app stability but can reduce browser-level zoom accessibility.
- Dynamic viewport units and `interactive-widget` behavior vary across mobile browsers.

### VText Planning Checklist

Purpose: document the next sequence before mixing UX, deployment, browser, VM, extraction, and citation scopes.

Changes:

- Added `docs/vtext-next-planning-checklist-2026-05-09.md`.
- Captures staging verification gates after pushing.
- Separates mobile/editor UX, staging verification, VM/background-work architecture, server-side browser/Obscura, coding benchmarks, extraction, citations/publication, and Pretext.
- Adds medium-difficulty coding benchmark ideas that should run through the product path rather than manual worker orchestration.

Risk:

- The checklist's Obscura section now points to the Obscura fork branch rather than carrying detailed Obscura evidence in the Choir repo.

## Dirty Worktree Inventory

Tracked dirty files:

```text
docs/vtext-next-planning-checklist-2026-05-09.md
frontend/package.json
frontend/tests/helpers/auth-state.js
```

Tracked dirty diff size:

```text
3 files changed, reduced from the pre-cleanup Obscura script/docs diff. Current tracked dirty content is the checklist pointer, `auth:setup` script entry, and auth-state helper reuse logic.
```

Untracked files:

```text
docs/choir-origin-main-change-report-2026-05-10.md
frontend/scripts/setup-auth-state.mjs
```

## Dirty Change Inventory

### Obscura Audit Migration

Purpose: remove Obscura-specific research artifacts from the Choir product repo.

Changes:

- Moved Obscura audit docs and upstream patch fragments into `/Users/wiz/obscura/docs/choir/`.
- Moved Obscura audit harness scripts into `/Users/wiz/obscura/scripts/choir/`.
- Moved sanitized JSON summaries from `/tmp/obscura-test/full-audit-pdf-complete-1778402200` into `/Users/wiz/obscura/docs/choir/artifacts/full-audit-pdf-complete-1778402200/`.
- Updated the VText planning checklist to point at the Obscura fork branch instead of local Choir docs.

Risk:

- The Obscura fork now owns that evidence. Choir keeps only a pointer and the generic auth-state setup utility.

### Auth-State Reuse

Purpose: set up authentication once and reuse it across Playwright/Obscura probes.

Changes:

- `frontend/tests/helpers/auth-state.js` now stores auth state under `frontend/playwright/.auth/<host>.storage.json`.
- Revalidates stored state against `/auth/session` before reuse.
- Writes metadata next to the storage state.
- Supports `CHOIR_AUTH_STATE` and `CHOIR_AUTH_META`.
- Uses lock files to avoid concurrent auth-state creation.

Risk:

- The storage directory is ignored by `frontend/.gitignore`, which is correct.
- Cross-host isolation is better than the old single `test-results` path.
- The helper now launches a browser context for validation, so failures may be more sensitive to staging availability.

### Package Scripts

Purpose: make reusable auth setup discoverable through npm.

Scripts added:

```text
auth:setup
```

Risk:

- Useful for staging QA and future browser automation.
- Does not add Obscura-specific npm commands to Choir.

## Verification Status

Commands used to prepare this report:

```sh
git fetch origin main
git status --short --branch
git log --oneline origin/main..HEAD
git diff --stat origin/main..HEAD
git diff --stat
git ls-files --others --exclude-standard
```

No build, Go tests, or Playwright tests were rerun as part of this report generation.

The committed checklist claims local frontend build and focused local Playwright checks passed when the UX commit was created. That evidence should be refreshed before pushing or before using this report as a release gate.

## Push Readiness Assessment

### Reasonable To Push After Review

- The three committed changes form a coherent staging candidate for mobile VText UX and planning.
- The committed code does not touch backend runtime/auth/vmctl behavior.
- The main product risk is frontend/mobile behavior, which can be checked with staging Playwright plus manual mobile Safari QA.

### Do Not Push Blindly

- Review and commit the remaining dirty auth/report/checklist changes separately from the existing UX/planning commits if they are kept.
- Refresh build and focused Playwright before pushing.

## Recommended Next Steps

1. Review this report and the remaining dirty Choir auth/checklist files.
2. If keeping them, run `cd frontend && npm run build`.
3. Run focused local Playwright on the VText/mobile prompt path.
4. Commit accepted dirty work separately from the existing three commits.
5. Push to `origin/main`, wait for GitHub Actions and Node B deployment, then run staging Playwright against `https://draft.choir-ip.com`.
6. Manually QA mobile Safari for viewport, keyboard, VText toolbar, prompt bar growth, and rendered-editable document behavior.
