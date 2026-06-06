# Source System Loop 8: Simplify, Stabilize, Modularize

Status: `draft_ready_to_run`.

Parent mission: `docs/mission-source-system-simplify-secure-smart-v0.md`.

Requirements contract: `docs/source-external-data-publication.md`.

Operating method: `docs/missiongradient-method.md`.

## Mission Thesis

The source-system mission reached useful correctness and staging proof across
VText, publication metadata, public source access, and visibility enforcement,
but the implementation and UI surface have accumulated complexity. Loop 8 is a
quality mission: stabilize the newly exposed VText/source UI bugs, remove dead
or weak paths, and design/refactor toward smaller reusable contracts without
changing source/publication security semantics or degrading performance.

Do not treat this as a cosmetic pass. The real artifact is a simpler,
staging-proven source/VText/publication system whose behavior is easier to
verify and whose core files are easier to change safely.

## Hard Invariants

- Preserve `docs/source-external-data-publication.md` as the requirements
  contract.
- Preserve the parent mission as the evidence ledger for Problems 1-38.
- Document every newly confirmed behavior problem before behavior-changing
  code.
- Keep Source Viewer as the default durable-artifact open surface and Web Lens
  as explicit original/live inspection.
- Preserve selector-rich transclusions, source snapshots, publication export
  metadata, and public visibility enforcement.
- Do not weaken SSRF/source acquisition policy checks.
- Do not introduce parallel source, VText, or publication contracts while
  refactoring. Extract shared contracts; delete duplicate shortcuts.
- Do not accept a refactor that adds measurable product-path latency without a
  named reason and benchmark evidence.
- Staging remains the acceptance environment for behavior changes.

## Current Belief State

- Public publication policy/storage/enforcement is deployed through
  `aa5902c42f65e834590e54a3b2617ce2819c8bd5`.
- The persistent publication-policy banner was removed, but the replacement
  publish menu still needs a passing deployed interaction proof.
- VText chrome has newly observed layout regressions:
  - version-label width and publish-button wrapping can change toolbar height;
  - the left draft-state chip says `Primary draft` for both latest and older
    revisions;
  - changing version labels must not alter toolbar dimensions;
  - the recent-document panel can remain in the pointer-event layer and
    intercept toolbar clicks after opening/creating a document.
- Core files such as `frontend/src/lib/VTextEditor.svelte` and VText/backend
  source files are too large for confident future changes.

Highest-impact uncertainty: which large-file paths are true shared contracts
versus incidental mission accretion that can be safely removed or extracted.

## Loop 8 Subphases

### 8A. Bug Inventory And Problem Ledger

Audit VText/source/publication UI and product paths before more fixes.

Deliverables:

- document confirmed bugs as numbered problems in this mission;
- classify each as product behavior, layout stability, contract drift, dead
  path, duplicate path, or test/proof debt;
- preserve screenshots/traces as evidence refs;
- identify whether each bug blocks existing acceptance proof.

Acceptance:

- every confirmed behavior bug has an evidence paragraph and acceptance
  criteria before code changes;
- no behavior-changing commit precedes the relevant problem documentation.

### 8B. Bounded UI Stabilization

Fix VText chrome/layout bugs with stable dimensions and deployed proof.

Known initial targets:

- publish menu opens from `Publish vN`, stays above document/recent surfaces,
  and its final command is clickable;
- no persistent publication-policy banner;
- version chip, draft-state chip, nav buttons, and publish command reserve
  stable width/height across v9/v10/v97/v100 and latest/historical states;
- latest state label reads `Latest` or equivalent;
- historical state label reads `Historical` or equivalent;
- label changes do not resize the toolbar;
- no fake pill controls for noninteractive policy facts.

Acceptance:

- focused Playwright proof passes on staging;
- screenshots or DOM metrics show toolbar height unchanged across latest and
  historical versions with different version-number widths;
- publish payload still includes explicit access/export policy.

### 8C. Dead/Weak Path Prune

Search for unused, duplicated, or shortcut paths introduced during the source
mission.

Targets:

- unused Svelte state, handlers, data attributes, CSS classes, and stale tests;
- duplicate source entity/reader artifact/evidence normalization paths;
- publication export/source metadata branches that bypass shared contracts;
- temporary proof hooks, debug-only selectors, and stale mission shims;
- backend code paths that encode source states outside shared packages.

Acceptance:

- deleted code has tests or search evidence proving no live product path uses
  it;
- no new abstraction is added merely to move code around;
- local and CI checks pass;
- behavior-changing deletions get staging proof when they affect product paths.

### 8D. Modularity And Contract Design

Design the refactor before large extraction.

Candidate modules:

- VText toolbar/chrome component;
- VText publication menu/policy review component;
- VText source transclusion renderer and source journal components;
- shared source contract normalization for VText, Source Viewer, Web Lens,
  publication, and export;
- backend VText/publication service boundaries around source metadata and
  reader artifact states.

Acceptance:

- produce a design note in this mission naming module boundaries, ownership,
  inputs/outputs, and migration order;
- define performance constraints and any benchmark/profiling checks;
- choose extraction order that keeps diffs reviewable and staging-verifiable.

### 8E. Incremental Refactor Execution

Extract modules only after 8D has a documented design.

Acceptance:

- each extraction commit preserves behavior and has focused tests;
- public/staging source publication behavior is unchanged;
- file size and responsibility boundaries improve measurably;
- no performance regression is observed in build output or product-path checks.

### 8F. Final Hard Review

Run adversarial/cognitive review after first correctness and refactor passes.

Acceptance:

- review names remaining weak spots and rejects superficial cleanup;
- update the hard mission report and PDF in iCloud Drive;
- final report includes commit SHAs, CI, deploy identity, staging proof,
  rollback refs, residual risks, and next realism axis.

## Initial Problems

### Problem L8-1: VText Publish Menu Still Has Hit-Test Debt

Status: `documented_pending_fix`.

problem: after replacing the persistent publication policy banner with a
publish menu, staging Playwright showed two hit-test failures: first the editor
surface intercepted the menu confirmation button; after adding toolbar stacking,
the recent-document panel could still intercept the toolbar publish button.

evidence:

```text
commit 7f576d1a9e316ded3741af39d0d1e019bf085ee9:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-authoring-history.spec.js -g "publish keeps policy"
  failed because data-vtext-editor-area intercepted data-vtext-publish-confirm.

commit aa5902c42f65e834590e54a3b2617ce2819c8bd5:
  the same staging proof failed because data-vtext-recent intercepted
  data-vtext-publish.
```

acceptance:

- publish menu and confirmation are clickable on staging;
- recent panel cannot intercept toolbar actions once a document surface is
  active;
- the focused staging Playwright proof passes without force-clicking.

### Problem L8-2: VText Toolbar Layout Changes Across Version Labels

Status: `documented_pending_fix`.

problem: changing between versions with different label widths can change the
toolbar height. The screenshots show latest `v97` fitting in one toolbar height
while historical `v96` wraps `Publish v96` across two lines and makes the top
bar taller.

evidence:

```text
user screenshots:
  /var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/TemporaryItems/NSIRD_screencaptureui_o1uNy8/Screenshot 2026-06-06 at 15.25.02.png
  /var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/TemporaryItems/NSIRD_screencaptureui_6dyNfa/Screenshot 2026-06-06 at 15.25.11.png

observed:
  latest shows Publish v97 on one line;
  historical shows Publish v96 wrapped onto two lines;
  toolbar vertical space changes.
```

acceptance:

- toolbar height is stable across latest/historical navigation;
- version chip, draft-state chip, and publish command reserve fixed responsive
  dimensions;
- text does not overflow or wrap inside toolbar buttons on supported widths.

### Problem L8-3: VText Draft-State Label Does Not Reflect Revision State

Status: `documented_pending_fix`.

problem: the left state chip always says `Primary draft`, including when the
editor is viewing an older revision. Latest and historical states should be
semantically distinct, but changing labels must not change toolbar layout.

acceptance:

- latest editable revision shows `Latest` or an equivalent current-state label;
- historical revision shows `Historical` or equivalent;
- label changes do not alter toolbar dimensions.

## Suggested Goal String

```text
/goal Run docs/mission-source-system-loop8-simplify-v0.md as a Codex-operated MissionGradient mission. Preserve docs/source-external-data-publication.md as the requirements contract and docs/missiongradient-method.md as the operating method. Treat docs/mission-source-system-simplify-secure-smart-v0.md as the parent evidence ledger, not the active checklist. First document all newly confirmed VText/source/publication UI and behavior problems before behavior-changing code. Stabilize VText chrome and publish-menu interactions on staging, including stable toolbar dimensions across version labels and correct latest/historical state labels. Then run Loop 8 simplification in subphases: bug inventory, bounded UI stabilization, dead/weak path pruning, modularity/refactor design with performance constraints, incremental extraction, and adversarial hard review. Audit large core files including frontend/src/lib/VTextEditor.svelte and backend VText/source/publication files for dead code, duplicate contracts, shortcut paths, and refactor seams. Refactor only with shared contracts, focused tests, no source/publication security regressions, no measurable performance cost, CI, Node B deploy identity, staging Playwright/API proof, rollback refs, and residual risks. Produce an updated hard mission report in docs and PDF in iCloud Drive before claiming completion.
```
