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

## Loop 8 Audit And Refactor Design

Status: `initial_design_ready`.

Audit date: 2026-06-06.

### Size And Responsibility Evidence

```text
wc -l:
  frontend/src/lib/VTextEditor.svelte              3768
  internal/runtime/vtext.go                        5667
  internal/platform/service.go                     1302
  internal/platform/export_formats.go               848
  internal/platform/source_metadata.go              375
  internal/platform/publication_document.go         356
  internal/platform/handlers.go                     229
```

Findings:

- `frontend/src/lib/VTextEditor.svelte` owns too many responsibilities in one
  Svelte component: recent-document chooser, editor state, autosave, toolbar
  chrome, publish menu/result/download UI, published reader mode, source panel
  orchestration, source artifact forms, source-ref click handling, compare and
  merge panels, source journal flow mounting, and document rendering.
- The frontend already has useful extraction anchors:
  `vtext-source-renderer.ts`, `vtext-source-launcher.ts`,
  `vtext-source-actions.ts`, `vtext-source-flow.ts`,
  `vtext-source-flow.css`, `VTextSourcePanel.svelte`, and
  `vtext-markdown-renderer.ts`. Future extraction should strengthen these
  existing seams rather than create a parallel source UI contract.
- `internal/runtime/vtext.go` is the largest backend risk. It mixes document
  CRUD/handlers, file open/import, `.vtext` shortcut manifests, Markdown lineage
  migration, source gap repair, source artifact attachment, table preservation,
  diagnosis, compare/merge, appagent prompting, and event emission.
- The backend also already has extraction anchors:
  `internal/sourcecontract`, `internal/markdownstructure`,
  `internal/runtime/vtext_media_sources.go`,
  `internal/runtime/vtext_controller.go`,
  `internal/runtime/vtext_proposals.go`,
  `internal/runtime/vtext_workflow_verifier.go`, and the newer
  `internal/platform/publication_document.go`/`export_formats.go` spine.
- The first rich-export cleanup removed the old PDF
  `publicationDocumentPlainText`/`wrapPDFLines` shortcut path. Similar pruning
  should be searched for by behavior surface rather than by file-size pressure.

### Design Principles

- Extract around stable contracts, not around visual proximity. A component or
  package boundary is valid only if it has a named input/output contract and can
  be tested without the full editor/runtime.
- Preserve the source contract as the shared grammar. The frontend should keep
  normalizing source evidence/open-surface/reader states through
  `source-contract.ts` and source renderer helpers; backend publication/export
  should keep normalizing through `internal/sourcecontract`.
- Keep publication export profile logic in the publication/export spine. VText
  editor code should request export/download, not know how HTML/DOCX/PDF encode
  source manifests.
- Do not split files merely to make line counts smaller. Each extraction should
  remove a concrete coupling: event handling, UI chrome, source contract
  normalization, document rendering, publication actions, import projection, or
  appagent prompt construction.
- Performance constraint: extractions must not add network calls, additional
  publication bundle loads, or repeated Markdown/source parsing on the hot
  editing path. Prefer derived state passed down as props and single-pass
  render/export over recomputation inside child components.

### Frontend Extraction Order

1. `VTextToolbar.svelte`
   - Owns version controls, state label, prompt/cancel/compare/source/publish
     buttons, restore/merge actions, and stable toolbar dimensions.
   - Inputs: revision state, loading/action state, labels, source candidate
     count, booleans for published/historical/merge/compare modes.
   - Outputs: semantic events only (`prompt`, `cancel`, `compare`, `sources`,
     `restore`, `merge-preview`, `publish-toggle`, etc.).
   - Verification: existing `vtext-authoring-history.spec.js` toolbar-height
     assertions plus publish-menu click proof.

2. `VTextPublishControls.svelte`
   - Owns publish menu, publish-result panel, copy/open/download actions, and
     published-result layout.
   - Inputs: version label, publish result, public URL, pending state, available
     formats.
   - Outputs: `publish`, `cancel`, `copy-link`, `open-link`, `copy-text`,
     `download`.
   - Verification: publish policy/menu test and visual proof that controls do
     not obscure the document.

3. `VTextDocumentSurface.svelte`
   - Owns editor/published article surface, rendered Markdown injection,
     source-ref pointer/keyboard handlers, focus state, and source-flow
     mounting hooks.
   - Inputs: rendered HTML, editability/read-only mode, source entities,
     document body flags.
   - Outputs: content edits, source ref open/toggle, focus/scroll events.
   - Verification: source transclusion tests and long-doc editing tests.

4. `VTextCompareMergePanel.svelte`
   - Owns compare result, merge preview, suggestion selection, errors, and
     accept/discard controls.
   - Verification: semantic compare/merge tests; no change to model prompt or
     backend merge semantics.

5. Keep `VTextSourcePanel.svelte` as the source-work drawer, but move shared
   source selection/form state helpers out of `VTextEditor.svelte` into a small
   `vtext-source-state.ts` only after the component extractions make the
   remaining state shape obvious.

### Backend Extraction Order

1. `internal/runtime/vtext_import.go`
   - Move file-open projections, `.vtext` shortcut manifests, DOCX/PDF/text
     import projection helpers, alias/title canonicalization, and original
     content item preservation.
   - Keep API handler behavior unchanged. Tests: `vtext-markdown-lineage`,
     file-browser VText open tests, focused Go tests for import projections.

2. `internal/runtime/vtext_lineage.go`
   - Move Markdown lineage types/helpers: source gap detection, citation
     resolution application, lineage metadata, source repair resolution
     manifests, and table-shaped row normalization entry points that are
     lineage-specific.
   - Leave generic table preservation helpers in or move them to a structure
     module only if reused by restore/create paths.

3. `internal/runtime/vtext_sources.go`
   - Move VText source entity decoding/normalization, source gap repair, and
     source artifact attachment.
   - Do not fork `internal/sourcecontract`; this module should call it.

4. `internal/runtime/vtext_merge.go`
   - Move semantic compare/merge model request/normalization/prompt helpers.
   - Preserve current model policy path and no-new-role-assumption invariant.

5. `internal/runtime/vtext_api_handlers.go`
   - After helper extraction, keep thin HTTP handlers and response shaping in a
     clearly named file rather than the current monolith.

### Publication Export Package Boundary

`internal/platform/publication_document.go` and `export_formats.go` are now the
canonical rich-export spine but are still in first modular shape. The next
backend package boundary should be:

```text
internal/publicationexport
  Document AST
  Source manifest
  Export profile
  HTML renderer
  DOCX renderer
  PDF renderer
```

`internal/platform` should own storage, route resolution, access policy, and
export endpoint orchestration. The export package should own pure rendering
from an already-authorized immutable `PublicationBundle`/projection input. Do
not move until staging-rich-export behavior has remained stable through at
least one more cleanup pass; otherwise the package move will obscure renderer
bugs.

### Refactor Execution Evidence

2026-06-06: after staging proof for professional rich exports, the publication
export renderer was split without changing behavior:

```text
internal/platform/export_formats.go                 69 lines
internal/platform/export_docx.go                   253 lines
internal/platform/export_html.go                   186 lines
internal/platform/export_pdf.go                    291 lines
internal/platform/export_helpers.go                 42 lines
internal/platform/publication_document_tables.go    39 lines
```

The extraction keeps `internal/platform/export_formats.go` as route/export
orchestration and moves format-specific rendering into separate files. Shared
source-manifest JSON, script escaping, XML/PDF escaping, and clamping helpers
live in `export_helpers.go`; Markdown-table recognition used by the
`PublicationDocument` projection lives in `publication_document_tables.go`.

No new export contract or source metadata path was introduced. HTML, DOCX, and
PDF still consume the same `PublicationDocument`, source manifest, and export
profile helpers. This is intentionally a package-in-place cleanup before any
future `internal/publicationexport` move, so renderer bugs remain easy to
attribute.

focused verification:

```text
nix develop -c go test ./internal/platform ./internal/sourcecontract
result: passed.

commit:
  792691a7ab0ecd732ffe0363bd3ed0c5267e2302

CI/deploy:
  GitHub Actions CI 27073773109 passed.
  FlakeHub publish 27073773117 passed.
  Node B deploy job 79907566344 passed.
  /health reported proxy and sandbox deployed_commit
  792691a7ab0ecd732ffe0363bd3ed0c5267e2302, deployed_at
  2026-06-06T21:02:18Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/rich-export-refactor-staging.tmp.spec.js
  result: 1 passed.

  The temporary proof created a source-backed VText publication, exported HTML,
  DOCX, and PDF, and verified semantic rich output plus embedded/recoverable
  source manifests. The scratch spec was deleted after the proof.
```

2026-06-06: the VText toolbar was extracted into
`frontend/src/lib/VTextToolbar.svelte` without changing toolbar semantics.
`VTextEditor.svelte` now computes revision/publish/source state and delegates
toolbar rendering/actions through semantic Svelte events.

```text
frontend/src/lib/VTextEditor.svelte   3399 lines
frontend/src/lib/VTextToolbar.svelte   590 lines

local verification:
  npm --prefix frontend run build
  result: passed with no Svelte unused-selector warnings.

local focused Playwright:
  npm --prefix frontend run e2e -- tests/vtext-authoring-history.spec.js
  result: blocked by local auth harness, not product code.
  details: initial run failed because no local server listened on localhost:4173;
  after `nix develop -c ./start-services.sh` and a persistent Vite session,
  `/auth/register/begin` returned 500 and the auth service was no longer
  listening on 127.0.0.1:8081. Staging proof remains required after deploy.

commit:
  8633cc96f502a9354bd9d1b42673eb13e7b5537b

CI/deploy:
  GitHub Actions CI 27074176202 passed.
  FlakeHub publish 27074176189 passed.
  Node B deploy job 79908640783 passed.
  /health reported proxy and sandbox deployed_commit
  8633cc96f502a9354bd9d1b42673eb13e7b5537b, deployed_at
  2026-06-06T21:20:42Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-authoring-history.spec.js
  result: 2 passed.
```

Next extraction target: VText publication result chrome. After the toolbar
move, `VTextEditor.svelte` still owns the published-success panel,
proposal-result panel, and a second copy of download-menu styling. No new
behavior problem is confirmed; this is a bounded simplification to keep
publish-result layout/action code with publication controls and reduce duplicate
chrome CSS. The parent should retain publication state and side-effect handlers;
the child should render panels and emit semantic copy/open/download events.

Result: `frontend/src/lib/VTextPublicationResult.svelte` now owns
published-success/proposal-result rendering and the parent delegates semantic
copy/open/download events.

```text
frontend/src/lib/VTextEditor.svelte             3193 lines
frontend/src/lib/VTextToolbar.svelte             590 lines
frontend/src/lib/VTextPublicationResult.svelte   283 lines

local verification:
  npm --prefix frontend run build
  result: passed with no Svelte unused-selector warnings.

commit:
  d4de8609f350a2ed3136bf96b25e6c3400fbcd17

CI/deploy:
  GitHub Actions CI 27074365126 passed.
  FlakeHub publish 27074365139 passed.
  Node B deploy job 79909160215 passed.
  /health reported proxy and sandbox deployed_commit
  d4de8609f350a2ed3136bf96b25e6c3400fbcd17, deployed_at
  2026-06-06T21:29:33Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-authoring-history.spec.js
  result: 2 passed.
```

Next extraction target: editor DOM-to-Markdown serialization. After toolbar and
publication-result extraction, `VTextEditor.svelte` still owns low-level
serialization helpers that convert the contenteditable DOM back into canonical
Markdown, including inline mark/link/source preservation, table row
normalization, heading/list/blockquote/code handling, and horizontal rules.
This is core document-surface behavior rather than editor orchestration. No new
behavior problem is confirmed; this is a bounded behavior-preserving extraction
to reduce the monolith and give the serialization path a focused unit surface.
The child/helper module must not import Svelte state, source acquisition, or
publication contracts. It should accept a DOM root and return Markdown using
the same current projection semantics.

Result: `frontend/src/lib/vtext-markdown-serializer.ts` now owns the
contenteditable DOM-to-Markdown projection. `VTextEditor.svelte` imports the
helper and keeps only the editor event handler that calls it before autosave.

```text
frontend/src/lib/VTextEditor.svelte              3100 lines
frontend/src/lib/VTextToolbar.svelte              590 lines
frontend/src/lib/VTextPublicationResult.svelte    283 lines
frontend/src/lib/vtext-markdown-serializer.ts      95 lines

local verification:
  npm --prefix frontend run build
  result: passed with no Svelte unused-selector warnings.

commit:
  bd499fbb40000a2f7a045832376c389463e5cdad

CI/deploy:
  GitHub Actions CI 27074540510 passed.
  FlakeHub publish 27074540515 passed.
  Node B deploy job 79909616958 passed.
  /health reported proxy and sandbox deployed_commit
  bd499fbb40000a2f7a045832376c389463e5cdad, deployed_at
  2026-06-06T21:37:35Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-authoring-history.spec.js
  result: 2 passed.
```

Next extraction target: VText compare/merge panel chrome. The parent still
must own compare/merge API calls, selected suggestion state, preview adoption,
and editor content replacement. The panel itself is presentational chrome:
heading, status, retry affordance, summary, selected suggestion checkboxes,
and preview provenance chips. No new behavior problem is confirmed; this is a
bounded extraction to remove markup/CSS coupling from `VTextEditor.svelte`
without moving model/provider semantics or merge state transitions.

Result: `frontend/src/lib/VTextCompareMergePanel.svelte` now owns the
compare/merge panel rendering. `VTextEditor.svelte` keeps compare/merge API
calls, editor content replacement, selected suggestion state, and adoption
handlers, passing state into the panel and receiving semantic retry/toggle
events.

```text
frontend/src/lib/VTextEditor.svelte              2897 lines
frontend/src/lib/VTextToolbar.svelte              590 lines
frontend/src/lib/VTextPublicationResult.svelte    283 lines
frontend/src/lib/vtext-markdown-serializer.ts      95 lines
frontend/src/lib/VTextCompareMergePanel.svelte    281 lines

local verification:
  npm --prefix frontend run build
  result: passed with no Svelte unused-selector warnings.

commit:
  82a823e7fb7446c60e2d9601411e435235f93165

CI/deploy:
  GitHub Actions CI 27074699960 passed.
  FlakeHub publish 27074699942 passed.
  Node B deploy job 79910045947 passed.
  /health reported proxy and sandbox deployed_commit
  82a823e7fb7446c60e2d9601411e435235f93165, deployed_at
  2026-06-06T21:45:13Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-authoring-history.spec.js
  result: 2 passed.
```

Next backend extraction target: VText file-open projection, original-artifact
preservation metadata, and `.vtext` shortcut manifest helpers. This is the
first `internal/runtime/vtext.go` cleanup from the documented backend
extraction order. The HTTP handlers and revision mutation flow should remain
in place for now; only pure helper types/functions should move into
`internal/runtime/vtext_import.go` in the same package. No new behavior problem
is confirmed; this is a behavior-preserving module boundary around import
projection semantics, source-path canonicalization, original file preservation,
DOCX/PDF/text projection, and manifest path allocation.

Result: `internal/runtime/vtext_import.go` now owns file-open projection
types/helpers, original content-item preservation metadata, text/DOCX/PDF
projection helpers, `.vtext` shortcut file generation, manifest path
allocation, and export filename normalization. `internal/runtime/vtext.go`
retains the HTTP handlers and revision mutation flow.

```text
internal/runtime/vtext.go          4871 lines
internal/runtime/vtext_import.go    810 lines

local verification:
  nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVText(OpenFile|PlainTextImport|ImportedMarkdown|ImportMarkdownLineage|EnsureManifest|APICreateRevisionCanonicalizesAliasedImportedDocumentTitle)'
  result: passed.

  nix develop -c go test ./internal/runtime
  result: passed.

commit:
  f03bf14af91499a00fe055fb75160ca6cb44d2ba

CI/deploy:
  GitHub Actions CI 27074885597 passed.
  FlakeHub publish 27074885607 passed.
  Node B deploy job 79910525028 passed.
  /health reported proxy and sandbox deployed_commit
  f03bf14af91499a00fe055fb75160ca6cb44d2ba, deployed_at
  2026-06-06T21:53:51Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-markdown-lineage.spec.js -g
  'Markdown lineage import resolves known citation markers|Imported Markdown advances|Imported plain text advances'
  result: 3 passed.
```

Next backend extraction target: Markdown lineage and source-gap helper
contracts. After the file-open/import helper extraction, `internal/runtime/vtext.go`
still owns the Markdown lineage import request/response helper types,
lineage-revision metadata construction, unresolved citation-marker detection,
citation resolution projection, source-gap filtering, source-repair evidence
normalization, original Markdown snapshot preservation, lineage summary
building, and content-item backed lineage resolution. These helpers are
semantically one boundary around durable Markdown-to-VText revision lineage and
repairable citation gaps. No new behavior problem is confirmed; this is a
behavior-preserving same-package extraction to reduce the runtime monolith
without moving HTTP handler flow, database write ordering, sourcecontract
normalization, or source/publication policy semantics.

Acceptance for this extraction:

- `internal/runtime/vtext_lineage.go` owns lineage/source-gap helper logic;
- `internal/runtime/vtext.go` keeps route handlers and revision mutations;
- no duplicate source/evidence state contract is introduced;
- focused Markdown lineage/import/source-repair tests pass;
- runtime package tests pass or any blocker is documented precisely;
- behavior-changing staging proof is only required if the extraction changes
  product behavior or deployed handler semantics.

Result: `internal/runtime/vtext_lineage.go` now owns Markdown lineage
metadata, citation-marker detection/projection, source-gap filtering,
source-repair evidence normalization, Markdown snapshot content-item
construction, lineage summaries, and content-item backed lineage resolution.
`internal/runtime/vtext.go` retains the import/source-repair handlers and
revision mutation sequence.

```text
internal/runtime/vtext.go           4385 lines
internal/runtime/vtext_lineage.go    504 lines
internal/runtime/vtext_import.go     810 lines

local verification:
  nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVText(OpenFile|PlainTextImport|ImportedMarkdown|ImportMarkdownLineage|SourceGap|EnsureManifest|APICreateRevisionCanonicalizesAliasedImportedDocumentTitle)'
  result: passed.

  nix develop -c scripts/go-test-runtime-shards
  result: passed.

single-implementation check:
  rg confirmed the moved lineage helpers now resolve only in
  internal/runtime/vtext_lineage.go.

commit:
  3d0f1f93582e9c588a7249140b128f6ad809b1a8

CI/deploy:
  GitHub Actions CI 27075142458 passed.
  FlakeHub publish 27075142462 passed.
  Node B deploy job 79911195730 passed.
  /health reported proxy and sandbox deployed_commit
  3d0f1f93582e9c588a7249140b128f6ad809b1a8, deployed_at
  2026-06-06T22:06:01Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-markdown-lineage.spec.js -g
  'Markdown lineage import resolves known citation markers|Imported Markdown advances|Imported plain text advances'
  result: 3 passed.
```

Next backend extraction target: VText source repair and source artifact
attachment handlers. The core source-entity type and normalization helpers
already live in `internal/runtime/vtext_media_sources.go`; after the lineage
extraction, `internal/runtime/vtext.go` still owns the HTTP handlers that
repair citation gaps and attach readable content items to existing source
entities, plus the helper that applies source artifact attachments. No new
behavior problem is confirmed; this is a same-package handler extraction to
keep source-repair route behavior together while preserving the existing
source entity contract and `internal/sourcecontract` normalization path.

Acceptance for this extraction:

- source repair and source artifact attachment handlers move out of
  `internal/runtime/vtext.go`;
- source entity types and normalization remain in
  `internal/runtime/vtext_media_sources.go`;
- sourcecontract remains the only source/evidence/open-surface normalizer;
- revision write order, emitted VText document events, and HTTP responses stay
  unchanged;
- focused source repair/import tests and runtime shard tests pass;
- staging proof exercises source-gap repair through the deployed product path.

Result: `internal/runtime/vtext_source_repairs.go` now owns the source-gap
repair route, source artifact attachment route, and attachment application
helper. `internal/runtime/vtext.go` keeps the surrounding compare/merge and
restore handlers; source entity types and normalization stay in
`internal/runtime/vtext_media_sources.go`.

```text
internal/runtime/vtext.go                   4065 lines
internal/runtime/vtext_source_repairs.go     336 lines
internal/runtime/vtext_lineage.go            504 lines
internal/runtime/vtext_import.go             810 lines

local verification:
  nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVText(ImportMarkdownLineage|SourceGap|SourceArtifact|ImportedMarkdown|PlainTextImport)'
  result: passed.

  nix develop -c scripts/go-test-runtime-shards
  result: passed.

commit:
  bb1127f79e9faff9cd6662f4ee3e4fd3a571d2f5

CI/deploy:
  GitHub Actions CI 27075349478 passed.
  FlakeHub publish 27075349466 passed.
  Node B deploy job 79911740479 passed.
  /health reported proxy and sandbox deployed_commit
  bb1127f79e9faff9cd6662f4ee3e4fd3a571d2f5, deployed_at
  2026-06-06T22:15:58Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-markdown-lineage.spec.js -g
  'Migrated source gaps can be repaired|Sources panel applies source-gap repair'
  result: 2 passed.
```

### Performance Checks

- Local focused backend check:
  `nix develop -c go test ./internal/platform ./internal/sourcecontract`.
- Runtime shard check before and after backend VText extraction:
  `nix develop -c scripts/go-test-runtime-shards`.
- Frontend check after VText component extraction:
  `npm --prefix frontend run build` and focused Playwright tests for VText
  authoring/history, source entities, source-service publication, and long-doc
  editing.
- Staging acceptance for behavior-affecting extraction:
  same deployed identity gate as code fixes plus the relevant focused
  Playwright/API proof.

### Forbidden Cleanup

- Do not move source normalization into Svelte components.
- Do not make export renderers call live source acquisition or reader services.
- Do not use local proof to claim toolbar/source/publication behavior is fixed.
- Do not remove compatibility aliases until search evidence and staging proof
  show no current VText/source/publication path still emits them.
- Do not split `internal/runtime/vtext.go` by copying types into duplicate
  packages; move them with the smallest import graph that compiles.

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

### Problem L8-5: HTML Rich Export Is Semantic But Browser-Default, Not Document-Professional

Status: `fixed_staging_proven`.

problem: after the first rich export fix, HTML no longer leaks raw Markdown
syntax, but visual inspection of a staging-generated artifact shows a
browser-default document rather than a professional publication export. The
page uses default margins/fonts, unbounded line length, plain tables without
document styling, and an unpolished source appendix. That is better than copied
Markdown, but it is not the Loop 8 target: a format-native, content-forward
professional document with source provenance.

evidence:

```text
staging product-path artifact:
  /tmp/choir-rich-export-visual-proof/rich-export-visual-proof.html

visual render:
  /tmp/choir-rich-export-visual-proof/html-render.png

publication:
  https://choir.news/pub/vtext/rich-export-visual-proof-1780777604534-pubdba4af408

observed:
  semantic headings, paragraphs, table, and source appendix are present;
  no raw Markdown syntax is visible;
  document typography, width, spacing, table borders, citation styling, and
  source appendix styling are still browser defaults.
```

acceptance:

- HTML rich export includes a default-professional document profile with
  bounded readable measure, page margins, heading hierarchy, paragraph rhythm,
  table borders/cell padding, citation marker styling, and source appendix
  styling;
- the profile is represented as an explicit export-profile concept, not as
  unrelated ad hoc CSS constants;
- embedded JSON-LD and `choir-source-manifest` remain present;
- source IDs may remain in machine-readable attributes/manifests, but visible
  body citations should read as professional markers or labels.

fix/evidence:

```text
local implementation:
  internal/platform/publication_document.go defines the initial
  default-professional export profile and source ordinal helpers.
  internal/platform/export_formats.go embeds profile CSS in HTML exports and
  renders visible source references with numeric markers while preserving
  source IDs in data attributes and the embedded source manifest.

local proof:
  nix develop -c go test ./internal/platform ./internal/sourcecontract
  result: ok

visual artifact:
  /tmp/choir-rich-export-local-proof/html-render.png
  shows bounded document width, professional spacing, styled table borders,
  numeric citation marker, and styled source appendix.

staging proof:
  commit 5435e48df9840886565e6a47faf866be5265e676
  GitHub Actions CI 27073438481 passed.
  FlakeHub run 27073438490 passed.
  Node B deploy job 79906671881 passed.
  /health reported proxy and sandbox deployed_commit
  5435e48df9840886565e6a47faf866be5265e676.
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/rich-export-artifacts.tmp.spec.js
  result: 1 passed; temporary spec was deleted after the run.
  Product-path publication:
    https://choir.news/pub/vtext/rich-export-staging-proof-1780778997223-pubeceb52e4d
  Downloaded HTML artifact:
    /tmp/choir-rich-export-staging-proof/rich-export-staging-proof.html
  Visual render:
    /tmp/choir-rich-export-staging-proof/html-render.png
```

### Problem L8-6: PDF Rich Export Flattens Structure And Misrenders Document Glyphs

Status: `fixed_staging_proven`.

problem: after the first rich export fix, PDF content is generated from the
`PublicationDocument` spine, but the visual artifact still looks like a plain
text dump. It duplicates the document title, does not differentiate heading
levels, flattens tables into pipe-delimited text, and renders list bullets with
an incorrect glyph under the current PDF font encoding. This fails the
format-native PDF requirement even though source metadata and visible source
appendix text are present.

evidence:

```text
staging product-path artifact:
  /tmp/choir-rich-export-visual-proof/rich-export-visual-proof.pdf

visual render:
  /tmp/choir-rich-export-visual-proof/pdf-pages/page-1.png

observed:
  title appears twice;
  heading hierarchy is not visibly represented;
  list bullets render as bad glyphs;
  table content is flattened with pipes rather than rendered as a table;
  source appendix exists but shares the same plain text treatment as body text.
```

acceptance:

- PDF renders blocks directly as layout operations, not by flattening the whole
  document to wrapped plain text first;
- title and first H1 are not duplicated;
- headings, paragraphs, lists, tables, source markers, and source appendix have
  distinct PDF layout treatment;
- bullet/list markers render predictably under the chosen PDF font encoding;
- source manifest XMP metadata remains embedded and extraction tests still
  prove it.

fix/evidence:

```text
local implementation:
  internal/platform/export_formats.go replaces the PDF plain-text flattening
  path with block-aware PDF page rendering for headings, paragraphs, lists,
  tables, rules, and source appendix entries. The old publicationDocumentPlainText,
  wrapPDFLines, and minInt shortcut helpers were removed.

local proof:
  nix develop -c go test ./internal/platform ./internal/sourcecontract
  result: ok

visual artifact:
  /tmp/choir-rich-export-local-proof/pdf-pages/page-1.png
  shows no duplicate title, readable heading hierarchy, stable ASCII list
  markers, drawn table borders, and visible source appendix.

staging proof:
  commit 5435e48df9840886565e6a47faf866be5265e676
  GitHub Actions CI 27073438481 passed.
  Node B deploy job 79906671881 passed.
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/rich-export-artifacts.tmp.spec.js
  result: 1 passed; temporary spec was deleted after the run.
  Downloaded PDF artifact:
    /tmp/choir-rich-export-staging-proof/rich-export-staging-proof.pdf
  Visual render:
    /tmp/choir-rich-export-staging-proof/pdf-pages/page-1.png
```

### Problem L8-7: DOCX Rich Export Still Exposes Internal Source IDs And Lacks Profile Polish

Status: `fixed_staging_proven`.

problem: after the first rich export fix, DOCX is a true WordprocessingML
package with runs, tables, custom properties, and a custom XML source manifest.
Visual Quick Look inspection shows it is much closer to a real document than
the previous Markdown-in-container export. However, the visible inline source
marker exposes the internal source entity id (`src-...`), the default Word
styling is crude, and the source/citation rendering is not yet a
professional-profile choice such as numeric footnote/endnote markers with a
source appendix.

evidence:

```text
staging product-path artifact:
  /tmp/choir-rich-export-visual-proof/rich-export-visual-proof.docx

visual render:
  /tmp/choir-rich-export-visual-proof/docx-quicklook/rich-export-visual-proof.docx.png

local limitation:
  LibreOffice/soffice is not installed in this environment, so the full
  render_docx.py page-render workflow could not be used. Quick Look thumbnail
  inspection was used as the available visual check.

observed:
  WordprocessingML headings, bold runs, table borders, source marker, source
  appendix, custom properties, and custom XML manifest are present;
  visible body marker includes internal source id rather than a professional
  citation marker;
  typography, heading hierarchy, table spacing, and source appendix treatment
  need a default-professional export profile.
```

acceptance:

- DOCX visible source references use profile-selected citation markers
  instead of exposing internal source entity IDs in body text;
- internal source IDs remain recoverable from custom XML/custom properties and
  any machine-readable relationship metadata;
- DOCX styles define the default-professional profile for title/headings/body,
  list/table/source appendix treatment, and future firm-specific overrides;
- extraction tests verify the manifest, policy, and source IDs survive while
  body text no longer exposes raw `src-...` markers as the visible citation.

fix/evidence:

```text
local implementation:
  internal/platform/export_formats.go adds a DOCX styles part, package
  relationships for styles, profile-oriented title/heading/list/source appendix
  styles, and numeric visible source markers. Internal source entity IDs remain
  in custom XML/custom properties and export metadata.

local proof:
  nix develop -c go test ./internal/platform ./internal/sourcecontract
  result: ok

visual artifact:
  /tmp/choir-rich-export-local-proof/docx-quicklook/rich-export-visual-proof.docx.png
  shows styled headings, table survival, numeric visible source marker, and a
  source appendix. LibreOffice/soffice remains unavailable locally, so Quick
  Look is still the local visual renderer for DOCX.

staging proof:
  commit 5435e48df9840886565e6a47faf866be5265e676
  GitHub Actions CI 27073438481 passed.
  Node B deploy job 79906671881 passed.
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/rich-export-artifacts.tmp.spec.js
  result: 1 passed; temporary spec was deleted after the run.
  Downloaded DOCX artifact:
    /tmp/choir-rich-export-staging-proof/rich-export-staging-proof.docx
  Visual render:
    /tmp/choir-rich-export-staging-proof/docx-quicklook/rich-export-staging-proof.docx.png
```

## Loop 8 Semantic Compare/Merge Extraction Target

Status: `documented_before_code`.

Next backend simplification target: extract VText semantic compare/merge
helpers and HTTP handlers from `internal/runtime/vtext.go` into a same-package
module.

This is a behavior-preserving extraction, not a semantic merge redesign. The
model prompt, provider retry path, VText model policy resolution, evidence
records, merge-edit sanitizer, revision metadata, and API request/response
shapes must remain unchanged. The extraction is valuable because semantic
compare/merge currently mixes provider prompting, model result normalization,
HTTP handler orchestration, merge-preview evidence, and revision acceptance
inside the main VText monolith.

acceptance:

- `internal/runtime/vtext.go` loses the semantic compare/merge implementation
  without introducing a parallel model or source contract;
- existing semantic merge unit tests still cover provider-backed JSON, summary
  fallback, and provenance stripping;
- runtime shard tests pass before any deploy claim;
- staging proof after deploy uses an existing VText history/compare product
  path or records a precise limitation if provider-backed semantic merge cannot
  be safely exercised from staging.

local result:

```text
implementation:
  internal/runtime/vtext_merge.go now owns semantic compare/merge helpers,
  provider-backed prompt/model calls, model-result normalization, merge edit
  application, and semantic compare/preview/accept HTTP handlers.

line-count effect:
  internal/runtime/vtext.go        3550 lines
  internal/runtime/vtext_merge.go   533 lines

local focused proof:
  nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVTextSemanticMerge|TestApplyVTextModelMergeEdits'
  result: ok

local shard proof:
  nix develop -c scripts/go-test-runtime-shards
  result: passed

commit/deploy proof:
  commit d6f37b3c6dbe71d68f7604da611d970333c0cdfc
  GitHub Actions CI 27075642754 passed.
  FlakeHub publish 27075642763 passed.
  Node B deploy job 79912531396 passed.
  /health reported proxy and sandbox deployed_commit
  d6f37b3c6dbe71d68f7604da611d970333c0cdfc, deployed_at
  2026-06-06T22:30:22Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-semantic-merge-staging.tmp.spec.js
  result: 1 passed; temporary spec was deleted after the run.
  The proof created two VText revisions through authenticated product APIs,
  called deployed `/api/vtext/documents/{id}/compare`, `/merge-preview`, and
  `/accept-merge`, and verified accepted merge revision metadata plus absence
  of visible merge-preview provenance.
```

## Suggested Goal String

```text
/goal Run docs/mission-source-system-loop8-simplify-v0.md as a Codex-operated MissionGradient mission. Preserve docs/source-external-data-publication.md as the requirements contract and docs/missiongradient-method.md as the operating method. Treat docs/mission-source-system-simplify-secure-smart-v0.md as the parent evidence ledger, not the active checklist. First document all newly confirmed VText/source/publication UI, export, and behavior problems before behavior-changing code. Stabilize VText chrome and publish/published-result interactions on staging, including stable toolbar dimensions across version labels, correct latest/historical state labels, non-overlapping publish/download controls, and content-forward reading space. Then make rich publication export a core Loop 8 artifact: design and implement a canonical PublicationDocument/source-manifest spine that renders professional format-native HTML, DOCX, and PDF from VText/publication semantics rather than raw Markdown; preserve visible citations/source appendices and embedded source metadata in every rich format; support future firm-specific export profiles for headings, typography, citation placement, headers/footers, and metadata policy. After rich export correctness, run Loop 8 simplification in subphases: bug inventory, bounded UI stabilization, rich export spine, dead/weak path pruning, modularity/refactor design with performance constraints, incremental extraction, and adversarial hard review. Audit large core files including frontend/src/lib/VTextEditor.svelte, internal/runtime/vtext.go, and backend VText/source/publication files for dead code, duplicate contracts, shortcut paths, and refactor boundaries. Refactor only through shared contracts, focused tests, no source/publication security regressions, no measurable performance cost, CI, Node B deploy identity, staging Playwright/API proof, visual/download inspection of DOCX/PDF/HTML, rollback refs, and residual risks. Produce an updated hard mission report in docs and PDF in iCloud Drive before claiming completion.
```

## Loop 8 Appagent Revision Extraction Target

Status: `documented_before_code`.

Next backend simplification target: extract VText appagent revision handling
from `internal/runtime/vtext.go` into a same-package module. This boundary owns
the `/revise` and `/cancel` handlers, pending mutation reconciliation, VText
agent run submission, backend-owned prompt construction, worker-message
context, focused long-document edit context, and VText revision event emission.

This is a behavior-preserving extraction. It must not change the VText
appagent prompt text, provider/tool-loop path, worker handoff rules, source
entity registration, pending mutation/idempotency behavior, cancellation
semantics, or revision event payloads. Test-only worker update/research
endpoints may remain in `vtext.go` unless moving them proves cleaner without
mixing product and test seams.

acceptance:

- `internal/runtime/vtext.go` loses appagent revision orchestration without
  introducing a parallel VText agent contract;
- existing prompt/unit tests still cover backend prompt invariants, focused
  user-edit context, super/researcher routing, source-ref preservation, and
  command-evidence rules;
- existing runtime tests still cover appagent revision creation, structured
  edits, cancellation, idempotency, progress events, media/source refs, and
  document stream head-change events;
- runtime shard tests pass before any deploy claim;
- staging proof after deploy exercises a deployed VText revise path or records
  a precise limitation if a live provider-backed revise cannot be safely run
  as part of this behavior-preserving extraction.

### Problem 47: Media Source Ref Comprehensive Test Uses Loopback Without Test Policy

status: `documented_before_fix`.

problem: during the appagent revision extraction proof, the comprehensive
`TestVTextAgentRevisionRegistersMediaSourceRefs` test failed because the direct
image fixture is served from `httptest.NewServer`, which produces a loopback
URL. The source acquisition policy correctly rejects loopback/private-network
fetches by default to preserve SSRF safety, so `registerVTextMediaSourceRefs`
registered only the YouTube source ref and skipped the image ref.

evidence:

```text
nix develop -c go test -tags comprehensive ./internal/runtime -run
'^TestVTextAgentRevisionRegistersMediaSourceRefs$' -count=1 -v
result: failed
media_source_refs len = 1, want 2
only ref: kind=youtube canonical_url=https://www.youtube.com/watch?v=dQw4w9WgXcQ
```

classification: `test/proof debt` with source-policy relevance. The production
policy behavior is correct; the test must explicitly enable the existing
test-only private-network allowance while preserving the default production
SSRF checks.

acceptance:

- the test declares and restores the sourcefetch private-network test override;
- the comprehensive media-source-ref test registers both YouTube and image
  source refs;
- sourcefetch policy tests still prove localhost/private-address rejection by
  default;
- no production source acquisition policy is weakened.

fix/evidence:

```text
files:
  internal/runtime/vtext_agent_revision.go       1074 lines
  internal/runtime/vtext.go                      2493 lines

change:
  VText appagent revision handlers, pending mutation reconciliation, agent run
  submission, prompt construction, worker-message context, focused edit
  context, and revision event emission moved from vtext.go to
  vtext_agent_revision.go.

  TestVTextAgentRevisionRegistersMediaSourceRefs now enables and restores the
  existing sourcefetch.SetAllowPrivateNetworkForTests override around its
  httptest image fixture only.

local proof:
  gofmt -w internal/runtime/vtext_test.go internal/runtime/vtext.go \
    internal/runtime/vtext_agent_revision.go && git diff --check
  result: passed

  nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVTextPrompt|TestBuildAgentRevisionRequest'
  result: passed

  nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVTextAgentRevisionCreatesCanonicalRevision|TestVTextAgentRevisionAppliesStructuredEdit|TestVTextCancelAgentRevisionCancelsRunGraphAndLeavesMutationResumable|TestVTextAgentRevisionProgressEvents|TestVTextDocumentStreamEmitsHeadChangeAfterAgentRevision|TestVTextAgentRevisionNoDuplicateOnRenewalRetry|TestVTextAgentRevisionMutationCompletedOnlyOnce'
  result: passed

  nix develop -c go test -tags comprehensive ./internal/runtime -run
  '^TestVTextAgentRevisionRegistersMediaSourceRefs$' -count=1 -v
  result: passed

  nix develop -c go test ./internal/sourcefetch
  result: passed

  nix develop -c scripts/go-test-runtime-shards
  result: passed

commit/deploy proof:
  commit dd02cc4d283feaf16b4d1cf4bbd24790f4af5ffb
  GitHub Actions CI 27076046933 passed.
  FlakeHub publish 27076046941 passed.
  Node B deploy job 79913572759 passed.
  /health reported proxy and sandbox deployed_commit
  dd02cc4d283feaf16b4d1cf4bbd24790f4af5ffb, deployed_at
  2026-06-06T22:49:31Z.

staging proof:
  CHOIR_DEPLOYED_BASE_URL=https://choir.news npm --prefix frontend run e2e --
  tests/vtext-appagent-revision-staging.tmp.spec.js
  first verifier attempt: failed waiting for the already-loaded desktop DOM to
  flip to data-authenticated=true after browser-side passkey registration;
  this did not reach the VText revise endpoint.
  second verifier attempt: passed after narrowing the verifier to check
  /auth/session for the authenticated browser session before calling VText
  APIs. The temporary spec was deleted after the run.

  The proof registered a browser passkey session through the deployed product
  auth path, created a VText document and user revision through deployed
  /api/vtext product APIs, called deployed /revise and received HTTP 202 with
  a loop_id/doc_id, then called deployed /cancel and received HTTP 200 with a
  resumable cancelled/no_pending_revision state.
```

## Loop 8 Frontend VText State Helper Extraction Target

Status: `documented_before_code`.

Next frontend simplification target: extract pure VText editor state helpers
from `frontend/src/lib/VTextEditor.svelte` into a TypeScript helper module.
This boundary owns deterministic helper logic that does not require Svelte
component state or DOM access: local draft storage key construction, Markdown
table block counting for stale-draft protection, revision ordering,
version-label computation, current/next version number fallback logic, explicit
publish access/export policy construction, public URL derivation from publish
responses, text truncation, and short hash formatting.

This is a behavior-preserving extraction. It must not change editor rendering,
autosave semantics, toolbar labels, publish payloads, version navigation,
Source Viewer/Web Lens behavior, or publication/export policy. The editor
component should keep side-effect orchestration; the helper module should own
only pure calculations.

acceptance:

- `VTextEditor.svelte` imports these helpers instead of defining duplicate
  local implementations;
- helper function names make the shared VText state contract explicit enough
  for reuse by future toolbar/publication/source modules;
- frontend build passes;
- at least one focused VText browser proof still exercises version navigation,
  publish policy payload construction, or stale draft/table protection;
- no source/publication security or export behavior is changed.

local extraction evidence:

```text
files:
  frontend/src/lib/VTextEditor.svelte       2794 lines
  frontend/src/lib/vtext-editor-state.ts     140 lines

change:
  Extracted deterministic editor-state helpers for draft storage keys,
  Markdown table counting, revision ordering, version labels/current-version
  fallback, publish policy payloads, public URL derivation, text truncation,
  and short hashes. The Svelte component still owns side effects, DOM handling,
  API calls, autosave scheduling, and app dispatch.

local proof:
  git diff --check
  result: passed

  npm --prefix frontend run build
  result: passed

commit/deploy proof:
  commit 6613bf043065fe9ee8892536b3e63eb1f9f25570
  GitHub Actions CI 27076266993 passed.
  FlakeHub publish 27076266994 passed.
  Node B deploy job 79914158820 passed.
  /health reported proxy and sandbox deployed_commit
  6613bf043065fe9ee8892536b3e63eb1f9f25570, deployed_at
  2026-06-06T23:00:39Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000
  npm --prefix frontend run e2e -- tests/vtext-authoring-history.spec.js
  result: 2 passed.

  The deployed proof exercised VText version navigation, stable toolbar height
  across latest/historical labels, and publish menu policy payload construction
  against the shipped frontend bundle.
```

### Problem 48: Rich Export Source Manifest Drops URL Targets For Nested Source Entities

status: `documented_before_fix`.

problem: the rich-export source manifest only extracts source URLs from
top-level entity fields such as `url`, `source_url`, or `href`. Normalized
publication source entities for URL-backed sources can carry the URL inside the
structured `target` object instead. When the manifest drops that URL, DOCX
cannot emit a native external hyperlink relationship for the visible source
reference or Sources appendix, even though the source is authorized and
URL-backed.

evidence:

```text
nix develop -c go test ./internal/platform -run
'^TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes$' -count=1 -v
result: failed after adding DOCX hyperlink/provenance assertions

observed word/document.xml:
  visible source reference and Sources appendix contained "Export source proof"
  and "[1]", but no <w:hyperlink> elements.

observed source metadata fixture:
  source target:
    {"target_kind":"url","url":"https://example.com/export-proof"}

root cause:
  sourceEntityDisplayFields reads top-level URL fields from entity.Entity but
  does not inspect entity.Target.
```

classification: `contract drift` between publication source entity
normalization and rich export rendering.

acceptance:

- `PublicationDocument` source manifests recover URL targets from the normalized
  source entity target object when no top-level URL is present;
- DOCX export creates external hyperlink relationships for URL-backed source
  references without exposing internal source IDs in visible body text;
- HTML/PDF/source manifest behavior still preserves embedded source metadata;
- platform export tests cover the nested-target URL path.

fix/evidence:

```text
change:
  PublicationDocument source manifest extraction now reads nested source
  target URL fields and nested display reader-artifact state before renderers
  consume the manifest. DOCX export builds deterministic document
  relationships for HTTP(S) source/link URLs, renders source labels as native
  w:hyperlink runs when a URL is available, and includes reader-meaningful
  source appendix details such as evidence state and reader artifact state.

local proof:
  gofmt -w internal/platform/export_docx.go \
    internal/platform/publication_document.go internal/platform/service_test.go
  git diff --check
  result: passed

  nix develop -c go test ./internal/platform -run
  '^TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes$' -count=1 -v
  result: passed

  nix develop -c go test ./internal/platform ./internal/sourcecontract
  result: passed

commit/deploy proof:
  commit 5e9722e1fe0117bf6b1093e80667a7da273ae8e3
  GitHub Actions CI 27076516017 passed.
  FlakeHub publish 27076516030 passed.
  Node B deploy job 79914793840 passed.
  /health reported proxy and sandbox deployed_commit
  5e9722e1fe0117bf6b1093e80667a7da273ae8e3, deployed_at
  2026-06-06T23:12:52Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000
  npm --prefix frontend run e2e --
  tests/rich-export-docx-url-staging.tmp.spec.js
  result: 1 passed; temporary spec and Playwright .last-run.json were deleted
  after the run.

  The proof created a VText document with a URL-backed source entity whose URL
  lived under nested target metadata, published it through deployed product
  APIs, exported DOCX, unzipped the package, and verified
  word/document.xml native hyperlinks/provenance, external hyperlink
  relationships in word/_rels/document.xml.rels, no visible internal source ID
  or raw Markdown leakage, and customXml/item1.xml source manifest recovery.
```

## Loop 8 Export Profile Contract Target

Status: `documented_before_code`.

Next rich-export simplification target: make the default professional export
profile a real shared contract instead of an `ID`/`Name` marker plus scattered
renderer constants. This is required before broader package movement because
firm-specific export support should plug into one profile object for
typography, heading scale, table style, citation placement, source detail
level, headers/footers, and metadata embedding policy.

This is a behavior-preserving contract extraction with small metadata
improvement. The default profile should keep the current professional visual
shape, but HTML, DOCX, PDF, export metadata, and embedded manifests should all
name the same profile and policy fields. Do not introduce a user-selectable
profile UI yet; the mission target is the render/export spine that future UI
or firm configuration can safely call.

acceptance:

- one `publicationExportProfile` struct defines ID/name plus typography,
  heading, table, citation, source detail, page, header/footer, and metadata
  policy fields;
- HTML, DOCX, PDF, and export metadata consume the same profile object rather
  than separate ad hoc constants;
- default output remains compatible with current staging-proven export
  behavior;
- platform tests assert the default-professional profile and its policy fields
  survive in rich export metadata and embedded document metadata;
- no source/publication access policy, source manifest, or SSRF behavior
  changes.

local implementation/evidence:

```text
change:
  internal/platform/publication_export_profile.go defines the default
  professional profile contract: typography, heading scale, table style,
  citation placement, source detail level, page/header/footer hooks, and
  metadata embedding policy.

  buildPublicationExportBytes now computes one profile and passes it through
  export metadata plus HTML, DOCX, and PDF renderers. HTML embeds the profile
  as JSON and derives profile CSS from it. DOCX writes profile policy into
  custom properties and derives Word style sizes from it. PDF embeds the same
  profile JSON in XMP metadata. Direct HTML fallback rendering uses the same
  default profile.

local proof:
  gofmt -w internal/platform/publication_export_profile.go \
    internal/platform/publication_document.go internal/platform/export_formats.go \
    internal/platform/export_html.go internal/platform/export_docx.go \
    internal/platform/export_pdf.go internal/platform/export_helpers.go \
    internal/platform/service.go internal/platform/service_test.go
  git diff --check
  result: passed

  nix develop -c go test ./internal/platform -run
  'TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes|TestPublishVTextCreatesImmutablePublicRecords'
  -count=1 -v
  result: passed

  nix develop -c go test ./internal/platform ./internal/sourcecontract
  result: passed

commit/deploy proof:
  commit 143db3ba407d9e87297490cc03cc4468faa9a3a9
  GitHub Actions CI 27076765965 passed.
  FlakeHub publish 27076765961 passed.
  Node B deploy job 79915440789 passed.
  /health reported proxy and sandbox deployed_commit
  143db3ba407d9e87297490cc03cc4468faa9a3a9, deployed_at
  2026-06-06T23:25:37Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000
  npm --prefix frontend run e2e --
  tests/rich-export-profile-staging.tmp.spec.js
  result: 1 passed; temporary spec and Playwright .last-run.json were deleted
  after the run.

  The proof created a source-backed VText publication through deployed product
  APIs and verified the shared default-professional profile contract in HTML
  export metadata/body, DOCX custom properties/styles, PDF XMP metadata, and
  API export metadata while preserving the source manifest.
```

## Loop 8 Platform Publication Read/Export Extraction Target

Status: `documented_before_code`.

Next backend simplification target: extract publication read, export, and
published-search service methods out of `internal/platform/service.go` into a
same-package module. `service.go` currently owns publish writes, reader
proposal writes, public bundle hydration, export gating, export response
assembly, published search, blob helpers, ids, slugging, and generic SQL
helpers. Keeping the read/export path in the same monolith makes it harder to
audit source/publication visibility and export policy behavior after rich
export work.

This is a behavior-preserving extraction. It must not change route
normalization, visibility enforcement, export-policy gating, source manifest
hydration, retrieval spans, citation edge private-revision redaction, source
entity/transclusion loading, public search behavior, export media types,
filename normalization, or Markdown/table export normalization. It should only
move the publication read/export/search methods and their private helpers into
a clearer file boundary.

acceptance:

- `internal/platform/service.go` loses the public bundle/export/search helper
  surface without introducing a second export or source contract;
- extracted code remains in `internal/platform` and continues to call the same
  `PublicationBundle`, `PublicationDocument`, `publicationExportProfile`, and
  source-manifest renderers;
- platform/sourcecontract tests pass;
- deployed staging proof after push reuses a publication export path that
  checks source/profile metadata, because this refactor touches the route used
  by public rich exports.

local implementation/evidence:

```text
change:
  Extracted public publication bundle hydration, export-policy gating, export
  response assembly, published search, route/export normalization, retrieval
  span hydration, citation-edge redaction, source entity/transclusion loading,
  publication policy loading, provenance summary loading, render-block
  projection, and snippets into
  internal/platform/service_publication_read.go.

line-count effect:
  internal/platform/service.go                    684 lines
  internal/platform/service_publication_read.go   629 lines

local proof:
  gofmt -w internal/platform/service.go \
    internal/platform/service_publication_read.go
  git diff --check
  result: passed

  nix develop -c go test ./internal/platform -run
  'TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes|TestPublishVTextCreatesImmutablePublicRecords|TestPublicationPublicSurfacesEnforceVisibilityPolicy'
  -count=1 -v
  result: passed

  nix develop -c go test ./internal/platform ./internal/sourcecontract
  result: passed

commit/deploy proof:
  commit 08c079770e59135c64c248675db7523bd766c5ca
  GitHub Actions CI 27076967942 passed.
  FlakeHub publish 27076967933 passed.
  Node B deploy job 79915966087 passed.
  /health reported proxy and sandbox deployed_commit
  08c079770e59135c64c248675db7523bd766c5ca, deployed_at
  2026-06-06T23:35:53Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000
  npm --prefix frontend run e2e --
  tests/publication-read-extraction-staging.tmp.spec.js
  result: 1 passed; temporary spec and Playwright .last-run.json were deleted
  after the run.

  The proof created a public URL-backed source VText through deployed product
  APIs, published it, resolved the public publication bundle, and exported
  HTML, DOCX, and PDF through the moved publication read/export service path.
  It verified route version identity, source entity/transclusion survival,
  export-policy formats, HTML source manifest/profile scripts, source URL and
  typed evidence-state metadata, DOCX/PDF binary signatures, source manifest
  metadata, and export-profile metadata policy fields.
```

## Loop 8 VText Structure Preservation Extraction Target

Status: `documented_before_code`.

Next backend simplification target: extract VText structure-preservation
helpers out of `internal/runtime/vtext.go` into a same-package module. This
boundary owns durable metadata carry-forward, stale user-draft content rebasing,
Markdown table-block detection, collapsed-table recovery, omitted-table
restoration, comparable Markdown projections, and collapsed text boundary
mapping.

This is a behavior-preserving extraction. It must not change revision handler
flow, store write ordering, revision metadata keys, table normalization,
restore semantics, stale-save conflict behavior, source entity carry-forward,
or publication/export behavior. The extraction is valuable because this logic
protects the owner-appendix/legal-proposal table survival path and bounded
table edits, but currently lives inside the main VText API handler monolith.

acceptance:

- `internal/runtime/vtext_structure.go` owns pure structure preservation and
  stale-draft rebase helpers;
- `internal/runtime/vtext.go` keeps HTTP handlers and revision store writes;
- existing tests for collapsed table recovery, historical table restore,
  concurrent stale-save rejection/rebase, and source-entity carry-forward pass;
- runtime shard tests pass or any blocker is recorded precisely;
- no source/publication security or rich-export contract changes.

local implementation/evidence:

```text
change:
  Extracted durable metadata carry-forward, stale user-draft content rebase,
  Markdown table block detection, collapsed/omitted table recovery, comparable
  Markdown projections, and collapsed text boundary mapping into
  internal/runtime/vtext_structure.go.

line-count effect:
  internal/runtime/vtext.go             2180 lines
  internal/runtime/vtext_structure.go    324 lines

local focused proof:
  nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVText(MarkdownStructureStabilization|RestoreRevisionNormalizesMalformedTableTailRows|UserSaveAndAgentRevisePreserveSourcesAndTableShape|UserSaveRemovesDuplicateMarkdownTableSeparator|CreateRevisionRejectsStaleParent|CreateRevisionRebasesAllowedStaleUserDraft|DiagnosisIncludesStructureEvidence|DiagnosisCanOmitRevisionContentForStructureEvidence)'
  -count=1 -v
  result: passed for matched tests covering structure stabilization, bounded
  table cell edit, omitted appendix-table restoration, restore normalization,
  source/table carry-forward, duplicate separator removal, stale-save rebase,
  and diagnosis structure omission.

  nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVTextCreateRevisionRejectsStaleHead|TestVTextDiagnosisReportsCurrentRevisionVersion'
  -count=1 -v
  result: passed.

local shard proof:
  nix develop -c scripts/go-test-runtime-shards
  result: passed.

commit/deploy proof:
  commit 8c465e30210db923f4288f2261a16793f725297c
  GitHub Actions CI 27077262831 passed.
  FlakeHub publish 27077262835 passed.
  Node B deploy job 79916718143 passed.
  /health reported proxy and sandbox deployed_commit
  8c465e30210db923f4288f2261a16793f725297c, deployed_at
  2026-06-06T23:50:48Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000
  npm --prefix frontend run e2e --
  tests/vtext-structure-extraction-staging.tmp.spec.js
  result: 1 passed; temporary spec and Playwright .last-run.json were deleted
  after the run.

  The proof created a real VText document through deployed product APIs,
  created a source-backed table-bearing revision, saved an owner edit that
  omitted the appendix table and verified deployed structure stabilization
  restored the table and carried source_entities forward, created a newer head,
  verified stale parent save rejection, then allowed a stale user draft rebase
  and verified the resulting revision preserved both head/user content,
  appendix table rows, rebase metadata, and source entity metadata.
```

## Loop 8 Publication Export Formatting Boundary Target

Status: `documented_before_code`.

Next backend simplification target: move remaining text/Markdown export body
formatting out of `internal/platform/service_publication_read.go` and into the
publication export spine. The read service should own authorization, bundle
hydration, route resolution, policy gating, and response assembly. It should
not directly import Markdown table-normalization code or decide how an
authorized publication bundle becomes an export body.

This is a behavior-preserving boundary cleanup. It must not change export
formats, media types, filenames, export-policy enforcement, Markdown table
normalization, HTML rich rendering, source manifest metadata, or public route
visibility. The value is removing a remaining renderer shortcut from the
public read service after DOCX/PDF/HTML renderers were split into the export
spine.

evidence:

```text
rg evidence:
  internal/platform/service_publication_read.go imports
  internal/markdownstructure only for formatPublicationExportContent.

  internal/platform/export_formats.go already owns buildPublicationExportBytes
  and dispatches DOCX, PDF, HTML, and default text/Markdown exports.

classification:
  refactor-boundary debt / duplicate ownership risk, not a behavior bug.
```

acceptance:

- `service_publication_read.go` no longer imports `internal/markdownstructure`;
- the Markdown/text body formatting helper lives beside
  `buildPublicationExportBytes` in the export spine;
- Markdown export still normalizes malformed table-shaped rows;
- HTML export still uses `PublicationDocument` and the shared export profile;
- platform publication/export tests pass.

local implementation/evidence:

```text
change:
  Moved formatPublicationExportContent from
  internal/platform/service_publication_read.go to
  internal/platform/export_formats.go. The public read service no longer
  imports internal/markdownstructure; export_formats.go now owns the remaining
  text/Markdown body formatting next to buildPublicationExportBytes.

line-count effect:
  internal/platform/export_formats.go             89 lines
  internal/platform/service_publication_read.go  611 lines

local proof:
  gofmt -w internal/platform/export_formats.go \
    internal/platform/service_publication_read.go
  git diff --check
  result: passed

  nix develop -c go test ./internal/platform -run
  'TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes|TestPublishVTextCreatesImmutablePublicRecords|TestPublicationPublicSurfacesEnforceVisibilityPolicy'
  -count=1 -v
  result: passed

  nix develop -c go test ./internal/platform ./internal/sourcecontract
  result: passed

commit/deploy proof:
  commit 57aca0664b6f0b82895c0c98cf0600c0022121a7
  GitHub Actions CI 27077645215 passed.
  FlakeHub publish 27077645214 passed.
  Node B deploy job 79917700568 passed.
  /health reported proxy and sandbox deployed_commit
  57aca0664b6f0b82895c0c98cf0600c0022121a7, deployed_at
  2026-06-07T00:10:07Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000
  npm --prefix frontend run e2e --
  tests/publication-export-format-boundary-staging.tmp.spec.js
  result: 1 passed; temporary spec and Playwright .last-run.json were deleted
  after the run.

  The proof created a VText document through deployed product APIs, published
  it with txt/md/html/docx/pdf export policy, and verified deployed Markdown
  export still repairs a valid table whose final row lost its trailing
  delimiter. It also verified HTML export contains the source-manifest script,
  DOCX/PDF exports return binary signatures, and all rich exports expose the
  expected export metadata/source manifest through the moved export spine.
```

## Loop 8 Publication Export Unreachable HTML Fallback Prune

Status: `documented_before_code`.

Next dead-path prune: remove the unreachable `html` branch inside
`formatPublicationExportContent`. `normalizeExportFormat` can return `html`,
but `buildPublicationExportBytes` handles the `html` case before dispatching to
`formatPublicationExportContent`; the helper is reached only for text-like
formats after DOCX, PDF, and rich HTML have already been selected.

This is a behavior-preserving deletion. It must not change rich HTML rendering,
export metadata, Markdown table normalization, text export, media types, or
export policy enforcement. The value is small but direct: remove a misleading
fallback that suggests HTML may be rendered through a generic text helper
instead of the canonical `PublicationDocument`/profile path.

evidence:

```text
rg evidence:
  internal/platform/export_formats.go handles case "html" in
  buildPublicationExportBytes before defaulting to formatPublicationExportContent.

  formatPublicationExportContent is only called from that default branch.

classification:
  dead branch / shortcut residue inside the rich export spine.
```

acceptance:

- `formatPublicationExportContent` handles only `md` and default text content;
- `buildPublicationExportBytes` remains the single HTML dispatch point;
- platform publication/export tests pass.

local implementation/evidence:

```text
change:
  Removed the unreachable html case from formatPublicationExportContent.
  Rich HTML export remains handled by buildPublicationExportBytes before the
  helper is called.

line-count effect:
  internal/platform/export_formats.go  87 lines

local proof:
  gofmt -w internal/platform/export_formats.go
  git diff --check
  result: passed

  nix develop -c go test ./internal/platform -run
  'TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes|TestPublicationMarkdownExportNormalizesMalformedTableTailRows|TestPublishVTextCreatesImmutablePublicRecords|TestPublicationPublicSurfacesEnforceVisibilityPolicy'
  -count=1 -v
  result: passed

  nix develop -c go test ./internal/platform ./internal/sourcecontract
  result: passed

commit/deploy proof:
  commit 153a21d5a34834c5679309a874cd84bdcbdc539b
  GitHub Actions CI 27077825314 passed.
  FlakeHub publish 27077825284 passed.
  Node B deploy job 79918157839 passed.
  /health reported proxy and sandbox deployed_commit
  153a21d5a34834c5679309a874cd84bdcbdc539b, deployed_at
  2026-06-07T00:19:19Z.

staging proof:
  No new temporary browser proof was required for this deletion because the
  removed branch was unreachable from deployed export dispatch. The preceding
  deployed export-spine smoke already exercised the live HTML, Markdown, DOCX,
  and PDF export endpoints after the formatter move; this checkpoint adds CI,
  deploy, and health identity for the dead-branch prune.
```

### Problem L8-8: Proposal Result Panel Exposes Internal Delivery Metadata

Status: `documented_before_fix`.

problem: after a published reader submits a proposal, the VText publication
result component renders internal delivery state and a proposal revision hash
as visible panel facts. This is the same class of UI issue as the earlier
publication policy banner: technically useful metadata is escaping into a
content-forward document surface where the reader needs an outcome, not an
internal state tuple.

evidence:

```text
frontend/src/lib/VTextPublicationResult.svelte:
  {#if publishedProposal}
    <p class="eyebrow">Proposal</p>
    <h2>{publishedProposal.state || 'recorded'}</h2>
    <span>{publishedProposal.delivery_state || 'recorded_for_author'}</span>
    <span>{shortHash(publishedProposal.proposal_revision_hash || '')}</span>
  {/if}

classification:
  product UI / visible metadata leakage.
```

acceptance:

- proposal result text is reader-facing and content-forward;
- delivery state and revision hashes are not shown as visible copy;
- machine/test data attributes may preserve proposal identifiers/states where
  useful for automation, but the visible UI must not read like an internal
  publication/proposal record;
- frontend build passes and staging proof exercises the proposal-result UI.

local implementation/evidence:

```text
change:
  VTextPublicationResult now renders published-reader proposal completion as
  "Proposal sent to author" with the reader-facing explanation "Your private
  version is ready for review." It no longer shows delivery_state or proposal
  revision hash as visible copy. Proposal id/state/delivery_state remain
  available only as data attributes for automation and diagnostics.

local proof:
  npm --prefix frontend run build
  result: passed.

commit/deploy proof:
  commit d24409bc67aa285b0b54d505bedf895069b23117
  GitHub Actions CI 27077922137 passed.
  FlakeHub publish 27077922135 passed.
  Node B deploy job 79918426601 passed.
  /health reported proxy and sandbox deployed_commit
  d24409bc67aa285b0b54d505bedf895069b23117, deployed_at
  2026-06-07T00:24:32Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000
  npm --prefix frontend run e2e --
  tests/vtext-proposal-result-staging.tmp.spec.js
  result: 1 passed; temporary spec and Playwright .last-run.json were deleted
  after the run.

  The proof created a VText document/revision through deployed product APIs,
  published it, opened a desktop VText window in published-reader proposal
  mode, submitted a proposal, and verified the visible result panel contained
  "Proposal sent to author" and "Your private version is ready for review."
  It also verified that internal values such as "recorded_for_author" and
  revision-hash-looking text were absent from visible panel copy.
```

## Loop 8 VText Diagnosis Boundary Extraction Target

Status: `implemented_local`.

Next runtime simplification target: move VText diagnosis DTOs and pure
diagnosis/structure summary helpers out of `internal/runtime/vtext.go` into a
same-package module. After structure-preservation helpers were extracted,
`vtext.go` still owned diagnosis response shapes, revision-structure summaries,
table-signature summaries, diagnosis content/query parsing, run ownership
filtering, and duplicate run merging. These are diagnosis-boundary concerns,
not core document/revision handler flow.

This is a behavior-preserving extraction. It must not change document CRUD,
revision writes, diagnosis endpoint semantics, blame endpoint semantics,
source/publication contracts, table detection behavior, run/evidence loading,
or any source/publication security boundary. The value is reducing the runtime
monolith while keeping the structure-evidence surface close to the diagnosis
endpoint that consumes it.

acceptance:

- `internal/runtime/vtext_diagnosis.go` owns diagnosis DTOs and pure
  diagnosis/structure summary helpers;
- `internal/runtime/vtext.go` keeps HTTP handlers, store calls, revision
  response shaping, and write-ordering behavior;
- focused diagnosis/blame/structure tests pass;
- runtime shard tests pass;
- no frontend, source contract, publication export, or SSRF policy files
  change.

local implementation/evidence:

```text
change:
  Extracted vtextDiagnosisResponse, vtextRevisionStructureSummary,
  vtextTableStructureSummary, vtextBlameResponse,
  revisionStructureSummaryFromRecord, vtextTableStructureSummaries,
  diagnosisIncludeContent, diagnosisOwnerRunScanLimit,
  runRecordBelongsToVTextDoc, and appendUniqueRunRecords into
  internal/runtime/vtext_diagnosis.go.

line-count effect:
  internal/runtime/vtext.go             1999 lines
  internal/runtime/vtext_diagnosis.go    188 lines

local proof:
  gofmt -w internal/runtime/vtext.go internal/runtime/vtext_diagnosis.go
  git diff --check
  result: passed

  nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVTextDiagnosisReportsCurrentRevisionVersion|TestVTextDiagnosisCanOmitRevisionContentForStructureEvidence|TestVTextDiagnosisIncludesDocumentChannelRuns|TestVTextAPISnapshotDoesNotMutateHead|TestVTextAPIGetBlame'
  -count=1 -v
  result: passed.

  nix develop -c scripts/go-test-runtime-shards
  result: passed.

commit/deploy proof:
  commit b2a8d9490e92ea2171b12314bba9178686e87bc1
  GitHub Actions CI 27078169938 passed.
  FlakeHub publish 27078169931 passed.
  Node B deploy job 79919151561 passed.
  /health reported proxy and sandbox deployed_commit
  b2a8d9490e92ea2171b12314bba9178686e87bc1, deployed_at
  2026-06-07T00:37:40Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000
  npm --prefix frontend run e2e --
  tests/vtext-diagnosis-extraction-staging.tmp.spec.js
  result: first attempt failed because the temporary spec incorrectly expected
  `revisions` to be populated with `include_content=false`; this is contrary
  to the intended diagnosis contract. The corrected proof asserted that full
  revision content was omitted, that body text did not leak into the response,
  and that `revision_structures` returned the expected deployed heading/table
  counts, separator flag, and SHA-256 table signature.

  corrected result: 1 passed. The temporary spec and Playwright .last-run.json
  were deleted after the run.
```

## Loop 8 Frontend Source Diagnosis Helper Extraction Target

Status: `implemented_staging_proven`.

Next frontend simplification target: move pure source diagnosis and edit
evidence projection helpers out of `frontend/src/lib/VTextEditor.svelte` into
a focused TypeScript helper module. After the source panel component
extraction, `VTextEditor.svelte` still owns deterministic shaping of diagnosis
responses into source-panel props: diagnosis summary facts, bounded revision
structure summaries, table signatures, and VText edit-evidence metadata.

This is a behavior-preserving extraction. It must not change source diagnosis
API calls, source repair/artifact actions, source panel rendering, Source
Viewer/Web Lens launch behavior, source evidence states, or VText edit
metadata semantics. The editor should keep source-panel orchestration and
side-effect handlers; the helper module should own only pure projection from
diagnosis/revision objects to UI-ready data.

acceptance:

- `frontend/src/lib/vtext-source-diagnosis.ts` owns pure diagnosis summary,
  structure evidence, and edit-evidence projection helpers;
- `VTextEditor.svelte` imports those helpers and no longer defines the local
  metadata parsing/projection helpers;
- frontend build passes;
- focused source/VText browser proof or deployed diagnosis proof still
  exercises diagnosis-derived source panel data;
- no source/publication security, export, or backend behavior changes.

local implementation/evidence:

```text
change:
  Extracted sourceDiagnosisSummary, sourceStructureEvidence,
  revisionEditEvidence, and sourceEditEvidence from VTextEditor.svelte into
  frontend/src/lib/vtext-source-diagnosis.ts. The editor still owns source
  panel orchestration, diagnosis fetch/cancel, source repair/artifact actions,
  and source open dispatch.

line-count effect:
  frontend/src/lib/VTextEditor.svelte           2704 lines
  frontend/src/lib/vtext-source-diagnosis.ts      97 lines

local proof:
  git diff --check
  result: passed

  npm --prefix frontend run build
  result: passed
```

deployment/evidence:

```text
source commit:
  74c304d8efb923a996c548fffc0f2cd6f8934288
  frontend: extract vtext source diagnosis helpers

CI/deploy:
  GitHub Actions CI 27078359741 passed.
  FlakeHub publish 27078359731 passed.
  Node B deploy job 79919670334 passed.
  /health reported deployed_commit
  74c304d8efb923a996c548fffc0f2cd6f8934288, deployed_at
  2026-06-07T00:47:22Z.

staging proof:
  PLAYWRIGHT_BASE_URL=https://choir.news CHOIR_DESKTOP_READY_TIMEOUT_MS=180000
  npm --prefix frontend run e2e --
  tests/vtext-markdown-lineage.spec.js -g
  "structured edit evidence|bounded revision structure"

  result: 2 passed. The deployed VText Sources panel showed structured edit
  evidence without leaking raw prompts and showed bounded revision structure
  summaries, table signatures, and no body text. This exercises the data now
  projected by frontend/src/lib/vtext-source-diagnosis.ts through the deployed
  VTextEditor.svelte source panel path.
```
