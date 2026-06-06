# Source System Loop 8: Simplify, Stabilize, Modularize

Status: `draft_ready_to_run`.

Parent mission: `docs/mission-source-system-simplify-secure-smart-v0.md`.

Requirements contract: `docs/source-external-data-publication.md`.

Operating method: `docs/missiongradient-method.md`.

## Mission Thesis

The source-system mission reached useful correctness and staging proof across
VText, publication metadata, public source access, and visibility enforcement,
but the implementation, UI surface, and export path have accumulated
complexity. Loop 8 is a quality mission: stabilize the newly exposed
VText/source UI bugs, make published VText exports professional
format-native documents with embedded source provenance, remove dead or weak
paths, and design/refactor toward smaller reusable contracts without changing
source/publication security semantics or degrading performance.

Do not treat this as a cosmetic pass. The real artifact is a simpler,
staging-proven source/VText/publication system whose visible UI and downloaded
documents are professional, whose source provenance survives across formats,
whose behavior is easier to verify, and whose core files are easier to change
safely.

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
- Rich exports must be rendered from structured VText/publication semantics,
  not by copying Markdown into DOCX/HTML/PDF containers.
- Exported source provenance must be visible enough for a reader and embedded
  enough for machine recovery.

## Current Belief State

- Public publication policy/storage/enforcement is deployed through
  `aa5902c42f65e834590e54a3b2617ce2819c8bd5`.
- VText publish chrome stabilization is deployed through
  `9fe7a2a4956909b21c672016996d00400f7f4421`; focused staging Playwright
  passed after the active-window test helper fix in
  `2769fea8177433bba634b75ae354a2e5f8eb5136`.
- VText chrome has newly observed layout regressions:
  - version-label width and publish-button wrapping can change toolbar height;
  - the left draft-state chip says `Primary draft` for both latest and older
    revisions;
  - changing version labels must not alter toolbar dimensions;
  - the recent-document panel can remain in the pointer-event layer and
    intercept toolbar clicks after opening/creating a document.
- Published-result chrome still risks stealing reading space after publish and
  should be treated as part of the same content-forward UI stabilization pass.
- Rich export first correctness is deployed through
  `e7fefc83c50e4e4d264721d02b5ce44f9b2ca6dc`: HTML, DOCX, and PDF now render
  from a shared `PublicationDocument`/source-manifest spine with semantic
  rich-format output and embedded manifest proof. Remaining export work is
  higher-fidelity PDF layout, DOCX footnote/hyperlink polish, visual artifact
  inspection, and future profile customization.
- Core files such as `frontend/src/lib/VTextEditor.svelte` and VText/backend
  source files are too large for confident future changes.

Highest-impact uncertainty: what canonical document/source representation can
drive VText rendering, publication export, source metadata preservation, and
future firm-specific document styles without creating another parallel system.

## Cognitive Transform Pass

Current uncertainty or obstacle: Loop 8 could collapse into local cleanup
patches if it optimizes file size or UI nits directly. The export failure shows
the deeper object is not "clean up VText"; it is "make VText a durable document
system whose source-rich documents survive authoring, publication, and external
professional formats."

Selected transforms:

1. Audience-level translation: a lawyer/client judges the artifact by the
   downloaded memo and visible provenance, not by the platform metadata API.
2. Depth extraction: export is not format conversion; it is projection of the
   canonical VText/source graph into another professional document grammar.
3. Artifact-biology: the system should grow around one document/source spine,
   not around separate UI, export, and publication organisms that drift apart.
4. Verifier inversion: acceptance must inspect Word/PDF/browser artifacts and
   embedded manifests, not merely assert that the API returned a `.docx` ZIP or
   a `%PDF` header.

Route-changing insights:

- The first major refactor target should be a canonical
  `PublicationDocument`/source-manifest representation, not only splitting
  `VTextEditor.svelte`.
- Rich export is a design driver for modularity: HTML, DOCX, PDF, VText
  renderer, Source Viewer, and publication metadata should consume the same
  source contract.
- UI stabilization remains urgent, but it is bounded pre-work; the larger
  simplification payoff comes from removing Markdown-as-interchange shortcuts.
- Performance is not the limiting factor. The limiting factor is semantic
  drift: source metadata, selectors, and citation markers must not fork across
  renderers.

Changed plan:

- implementation: stabilize VText chrome first, then build the structured
  export/document spine before large extraction;
- verifier/evidence: combine API checks with visual/download inspection and
  embedded metadata extraction for DOCX/HTML/PDF;
- scope: include rich export and source embedding as Loop 8 core work, not a
  follow-up;
- stopping condition: do not call Loop 8 complete until VText UI, rich exports,
  source metadata embedding, and simplification/refactor evidence all pass on
  staging.

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
- published-result header/actions do not permanently obscure the document or
  collide with download controls.

Acceptance:

- focused Playwright proof passes on staging;
- screenshots or DOM metrics show toolbar height unchanged across latest and
  historical versions with different version-number widths;
- publish payload still includes explicit access/export policy.

### 8C. Rich Export Spine

Design and implement structured rich exports before broad refactoring.

Targets:

- `PublicationDocument` AST from the immutable publication bundle;
- shared source manifest used by HTML, DOCX, PDF, Markdown, VText rendering,
  Source Viewer, and publication metadata;
- HTML renderer with semantic headings, paragraphs, lists, tables, links,
  source markers, source appendix, JSON-LD, and embedded manifest;
- DOCX renderer with WordprocessingML styles/runs/tables, footnotes/endnotes
  or source appendix, hyperlinks where allowed, custom XML source manifest, and
  custom properties;
- PDF renderer with formatted layout, visible citations/source appendix, XMP
  metadata, and optional associated-file JSON manifest for archival profiles;
- export profiles for future firm-specific style, headings, citation placement,
  headers/footers, and metadata policy.

Acceptance:

- no raw Markdown markers leak into DOCX/HTML/PDF output body;
- source references are visible in each rich format;
- a normalized source manifest is embedded in every rich format;
- downloaded DOCX/PDF/HTML are visually inspected and have extraction tests;
- publication export metadata, access policy, source snapshots, selectors, and
  retrieval spans are preserved.

### 8D. Dead/Weak Path Prune

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

### 8E. Modularity And Contract Design

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

### 8F. Incremental Refactor Execution

Extract modules only after 8E has a documented design.

Acceptance:

- each extraction commit preserves behavior and has focused tests;
- public/staging source publication behavior is unchanged;
- file size and responsibility boundaries improve measurably;
- no performance regression is observed in build output or product-path checks.

### 8G. Final Hard Review

Run adversarial/cognitive review after first correctness and refactor passes.

Acceptance:

- review names remaining weak spots and rejects superficial cleanup;
- update the hard mission report and PDF in iCloud Drive;
- final report includes commit SHAs, CI, deploy identity, staging proof,
  rollback refs, residual risks, and next realism axis.

## Initial Problems

### Problem L8-1: VText Publish Menu Still Has Hit-Test Debt

Status: `fixed_staging_proven`.

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

user screenshots:
  /var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/TemporaryItems/NSIRD_screencaptureui_2qmOee/Screenshot 2026-06-06 at 15.34.18.png
  /var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/TemporaryItems/NSIRD_screencaptureui_YPcG5o/Screenshot 2026-06-06 at 15.36.06.png
  show that the publish menu is visually transparent, overlaps the document
  title, and exposes route/source/download policy metadata that is not useful
  at the owner decision point. After publishing, the published-result header
  opens while the stale publish menu can remain visually present, colliding
  with the published header/download controls.
```

acceptance:

- publish menu and confirmation are clickable on staging;
- recent panel cannot intercept toolbar actions once a document surface is
  active;
- publish menu is opaque and compact;
- publish menu presents user-relevant consequence text and commands, not raw
  route/source/download metadata;
- successful publish closes the menu before showing the published-result
  header;
- the focused staging Playwright proof passes without force-clicking.

fix/evidence:

```text
commits:
  308cdddab186e25834b473dacf3ea69992309711
    simplified the publish confirmation menu.
  9fe7a2a4956909b21c672016996d00400f7f4421
    stabilized toolbar/publish/result chrome.
  2769fea8177433bba634b75ae354a2e5f8eb5136
    scoped the VText history test to the active window.

CI/deploy:
  GitHub Actions CI 27072453680 passed.
  Node B deploy job 79904081697 passed.
  /health reported proxy and sandbox deployed_commit
  9fe7a2a4956909b21c672016996d00400f7f4421.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-authoring-history.spec.js
  result: 2 passed.
```

### Problem L8-2: VText Toolbar Layout Changes Across Version Labels

Status: `fixed_staging_proven`.

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

fix/evidence:

```text
commit:
  9fe7a2a4956909b21c672016996d00400f7f4421

test coverage:
  tests/vtext-authoring-history.spec.js now asserts latest -> historical
  navigation changes the label while preserving toolbar height.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-authoring-history.spec.js
  result: 2 passed.
```

### Problem L8-3: VText Draft-State Label Does Not Reflect Revision State

Status: `fixed_staging_proven`.

problem: the left state chip always says `Primary draft`, including when the
editor is viewing an older revision. Latest and historical states should be
semantically distinct, but changing labels must not change toolbar layout.

acceptance:

- latest editable revision shows `Latest` or an equivalent current-state label;
- historical revision shows `Historical` or equivalent;
- label changes do not alter toolbar dimensions.

fix/evidence:

```text
commit:
  9fe7a2a4956909b21c672016996d00400f7f4421

behavior:
  latest revision label renders as "Latest"; historical revision label renders
  as "Historical"; both use reserved toolbar dimensions.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-authoring-history.spec.js
  result: 2 passed.
```

### Problem L8-4: Rich Publication Exports Leak Markdown Into DOCX/HTML/PDF

Status: `first_correctness_staging_proven`.

problem: published VText downloads for DOCX, HTML, and PDF are valid file
containers but not correct formatted documents for their formats. HTML and PDF
show raw Markdown headings, front matter delimiters, bold markers, and source
link syntax. DOCX uses some heading paragraph styles, but still leaves inline
Markdown bold/link syntax as literal text. These exports should be
format-native documents, not Markdown copied into different containers.

evidence:

```text
user files:
  /Users/wiz/Downloads/choir-private-legal-cloud-proposal-vtext-pubc66d4bdf0.docx
  /Users/wiz/Downloads/choir-private-legal-cloud-proposal-vtext-pubc66d4bdf0.html
  /Users/wiz/Downloads/choir-private-legal-cloud-proposal-vtext-pubc66d4bdf0.pdf

user screenshots:
  /var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/TemporaryItems/NSIRD_screencaptureui_xaiJ55/Screenshot 2026-06-06 at 15.38.14.png
  /var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/TemporaryItems/NSIRD_screencaptureui_VBXF0u/Screenshot 2026-06-06 at 15.38.25.png
  /var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/TemporaryItems/NSIRD_screencaptureui_pN38V3/Screenshot 2026-06-06 at 15.38.35.png

local inspection:
  DOCX word/document.xml contains literal "**private legal cloud**" and
  "(source:src_aba_formal_op_512)".
  HTML contains escaped "# Proposal", "## The Problem...", raw
  "**private legal cloud**", and markdown source-link syntax inside a single
  paragraph with <br>.
```

acceptance:

- HTML export renders headings, paragraphs, emphasis, lists, tables, and source
  references as HTML elements rather than escaped Markdown text;
- DOCX export converts inline emphasis and source links to Word runs/hyperlinks
  or acceptable styled text, without literal Markdown markers;
- PDF export renders a publication-quality document from formatted blocks, not
  raw Markdown lines;
- existing metadata, source snapshot, access/export policy, and retrieval
  envelopes remain present;
- add tests that fail on literal Markdown markers in DOCX/HTML/PDF exports.

fix/evidence:

```text
commit:
  e7fefc83c50e4e4d264721d02b5ce44f9b2ca6dc

implementation:
  internal/platform/publication_document.go adds a shared
  PublicationDocument/source-manifest spine.
  internal/platform/export_formats.go renders HTML/DOCX/PDF from that spine
  instead of per-format raw Markdown copying.
  legacy markdownBlocks parser path was removed.

local proof:
  nix develop -c go test ./internal/platform
  result: ok
  nix develop -c go test ./internal/platform ./internal/sourcecontract
  result: ok

CI/deploy:
  GitHub Actions CI 27072850941 passed.
  FlakeHub run 27072850942 passed.
  Node B deploy job 79905108418 passed.
  /health reported proxy and sandbox deployed_commit
  e7fefc83c50e4e4d264721d02b5ce44f9b2ca6dc.

staging product-path proof:
  temporary Playwright proof used authenticated product APIs to create a VText
  document with source metadata, publish it publicly, and export html/docx/pdf.
  Assertions verified semantic HTML, DOCX WordprocessingML runs/table/customXml
  source manifest, PDF visible text/source appendix/XMP manifest, and no raw
  Markdown source/bold syntax in the rendered rich outputs.
  Command:
    PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
    tests/rich-export-proof.tmp.spec.js
  Result:
    1 passed
  The temporary proof spec was deleted after the run to preserve worktree
  hygiene.
```

remaining risks:

- PDF is still a simple generated PDF renderer rather than a typographically
  complete professional layout engine.
- DOCX uses source appendix/custom XML and styled source markers; footnotes,
  endnotes, and hyperlink relationships remain profile-level polish.
- Firm-specific export profiles are designed but not implemented.
- Visual/manual inspection of downloaded real proposal artifacts still remains
  for final Loop 8 acceptance.

## Suggested Goal String

```text
/goal Run docs/mission-source-system-loop8-simplify-v0.md as a Codex-operated MissionGradient mission. Preserve docs/source-external-data-publication.md as the requirements contract and docs/missiongradient-method.md as the operating method. Treat docs/mission-source-system-simplify-secure-smart-v0.md as the parent evidence ledger, not the active checklist. First document all newly confirmed VText/source/publication UI, export, and behavior problems before behavior-changing code. Stabilize VText chrome and publish/published-result interactions on staging, including stable toolbar dimensions across version labels, correct latest/historical state labels, non-overlapping publish/download controls, and content-forward reading space. Then make rich publication export a core Loop 8 artifact: design and implement a canonical PublicationDocument/source-manifest spine that renders professional format-native HTML, DOCX, and PDF from VText/publication semantics rather than raw Markdown; preserve visible citations/source appendices and embedded source metadata in every rich format; support future firm-specific export profiles for headings, typography, citation placement, headers/footers, and metadata policy. After rich export correctness, run Loop 8 simplification in subphases: bug inventory, bounded UI stabilization, rich export spine, dead/weak path pruning, modularity/refactor design with performance constraints, incremental extraction, and adversarial hard review. Audit large core files including frontend/src/lib/VTextEditor.svelte, internal/runtime/vtext.go, and backend VText/source/publication files for dead code, duplicate contracts, shortcut paths, and refactor boundaries. Refactor only through shared contracts, focused tests, no source/publication security regressions, no measurable performance cost, CI, Node B deploy identity, staging Playwright/API proof, visual/download inspection of DOCX/PDF/HTML, rollback refs, and residual risks. Produce an updated hard mission report in docs and PDF in iCloud Drive before claiming completion.
```
