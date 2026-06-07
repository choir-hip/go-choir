# Source System Loop 8 Hard Mission Review

Date: 2026-06-06

Mission: `docs/mission-source-system-loop8-simplify-v0.md`

Requirements contract: `docs/source-external-data-publication.md`

Operating method: `docs/missiongradient-method.md`

Parent evidence ledger: `docs/mission-source-system-simplify-secure-smart-v0.md`

Current review head: `66b29d7ee7542fd90b58fe18289cddbb247b3248`

Status: `checkpoint_incomplete`

## Verdict

Loop 8 made the source/VText/publication system materially more stable and
more maintainable, but the mission should not be called complete.

The main correctness claims now have staging evidence: VText publish chrome no
longer exposes the policy banner or unstable toolbar labels, rich publication
exports are format-native documents rather than Markdown copied into containers,
source provenance is visible and embedded across HTML, DOCX, and PDF, and
several large VText/runtime/platform files have been reduced through
behavior-preserving extractions.

The remaining reason to keep the mission open is not a known blocking bug. It
is proof depth and final pruning. Loop 8 still needs a final adversarial pass
that rejects weak abstractions, identifies any remaining dead paths, and decides
which residual risks belong in the next mission rather than hiding them under a
"simplified" label.

## Cognitive Review

Selected transforms:

1. Depth extraction: the real artifact is not a cleaner file tree. It is a
   source-rich document system whose evidence survives editing, publication,
   guest reading, and external professional formats.
2. Verifier inversion: a green export API is insufficient. The artifacts must
   be inspected as HTML, WordprocessingML, PDF text/layout, embedded metadata,
   and public reader behavior.
3. Boundary audit: Source Viewer remains the durable artifact reader; Web Lens
   remains explicit live/original inspection. Cleanup that blurs this boundary
   is regression, even if it removes code.
4. Anti-Goodhart check: line-count reduction is only useful when responsibility
   boundaries improve and tests prove behavior did not move.

Changed review stance:

- accept focused extraction checkpoints only when CI, deploy identity, and
  staging product proof exist;
- treat rich export as a core source contract consumer, not a download feature;
- treat current Loop 8 as a strong checkpoint, not completion, until final
  pruning and residual-risk review are done.

## Completion Audit

| Requirement | Evidence | Status |
| --- | --- | --- |
| Preserve requirements contract and MissionGradient method | Active mission names `docs/source-external-data-publication.md` and `docs/missiongradient-method.md`; parent ledger remains separate. | Proven |
| Document newly confirmed problems before behavior-changing code | Problems L8-1 through L8-7 and Problems 47-48 were documented before fixes; later pure refactors were documented as extraction targets. | Proven for known issues |
| Stabilize VText publish/published-result chrome | Problems L8-1, L8-2, L8-3 fixed with staging `vtext-authoring-history.spec.js` proof. | Proven |
| Stable toolbar dimensions and latest/historical labels | Staging test asserts latest to historical navigation preserves toolbar height and label semantics. | Proven |
| Non-overlapping publish/download controls | Publish menu and result panel were moved behind explicit controls; staging proof verified clickability and policy payload. | Proven for tested surface |
| Rich export uses structured VText/publication semantics, not raw Markdown | Problem L8-4 fixed with shared `PublicationDocument` and source manifest spine; tests assert no raw Markdown leakage in rich formats. | Proven |
| Format-native HTML | Problem L8-5 fixed; HTML has semantic blocks, profile CSS, source appendix, JSON-LD, and embedded source manifest. | Proven |
| Format-native PDF | Problem L8-6 fixed; PDF renders blocks, lists, tables, visible markers, source appendix, and XMP manifest. Typography is still simple. | Proven with residual polish risk |
| Format-native DOCX | Problem L8-7 fixed; DOCX has styles, table rendering, visible source markers, source appendix, custom XML manifest, custom properties, and native hyperlinks. | Proven with residual footnote/endnote polish risk |
| Source metadata embedded in every rich format | Staging proofs verified HTML manifest/profile scripts, DOCX custom XML/properties/hyperlinks, and PDF XMP/source manifest metadata. | Proven |
| Future firm-specific export profile spine | `publication_export_profile.go` defines typography, heading, table, citation, source detail, page/header/footer, and metadata policy fields consumed by renderers. No user/firm configuration UI yet. | Spine proven, product customization pending |
| Preserve Markdown export | Markdown export remains separate from rich renderers and is covered by publication/read/export tests. | Proven for tested paths |
| Source Viewer default and Web Lens explicit live inspection | Earlier source-system proofs and frontend tests cover durable source opening. Loop 8 did not weaken this path. | Proven by inherited evidence, not re-proven in every Loop 8 slice |
| Selector-rich transclusions and source snapshots through publication/export | Source-service and URL-backed publication proofs verify source entities, transclusions, selectors, snapshots, retrieval spans, and export metadata. | Proven for tested source classes |
| Source acquisition policy and SSRF safety | Loop 8 did not change source fetch policy. Problem 47 explicitly preserved private-network rejection while enabling a test-only override. | Proven by sourcefetch tests and no-regression evidence |
| Large frontend VText simplification | `VTextEditor.svelte` reduced to 2794 lines after extracting toolbar, publication result, DOM-to-Markdown serializer, compare/merge panel, and state helpers. | Proven |
| Large backend VText simplification | `internal/runtime/vtext.go` reduced to 2178 lines after extracting import, lineage, source repairs, merge, appagent revision, and structure helpers. | Proven |
| Platform publication/export simplification | `internal/platform/service.go` split publication read/export/search into `service_publication_read.go`; export renderers split by format. | Proven |
| No source/publication security regression | Focused platform/sourcecontract/runtime tests and staging publication/export proofs passed after behavior-affecting changes. | Proven for touched paths |
| No measurable performance cost | Design constraints avoid added network calls or repeated bundle loads; CI/staging proofs show no functional regression. No dedicated latency benchmark was run. | Partial |
| CI, Node B deploy identity, staging proof | Latest behavior commit `8c465e30` passed CI `27077262831`, FlakeHub `27077262835`, Node B deploy `79916718143`, and `/health` reported the same SHA. | Proven |
| Visual/download inspection of DOCX/PDF/HTML | Mission evidence includes downloaded staging artifacts and Quick Look/PDF/HTML visual inspection for rich export. | Proven |
| Rollback refs and residual risks | Commit SHAs and deploy identities are recorded; product rollback refs are covered by parent mission evidence. Residual risks are named below. | Partial |
| Final hard review report and PDF in iCloud Drive | This Markdown report is complete; PDF generated at `/Users/wiz/Library/Mobile Documents/com~apple~CloudDocs/Choir Mission Reports/source-system-loop8-hard-mission-review-2026-06-06.pdf`. | Proven |

## Evidence Summary

Latest deployed behavior checkpoint:

- Commit: `8c465e30210db923f4288f2261a16793f725297c`
- Change: `runtime: extract vtext structure preservation helpers`
- CI: `27077262831`, passed
- FlakeHub: `27077262835`, passed
- Node B deploy job: `79916718143`, passed
- Staging health: proxy and sandbox `deployed_commit=8c465e30210db923f4288f2261a16793f725297c`, `deployed_at=2026-06-06T23:50:48Z`

Latest docs checkpoint:

- Commit: `66b29d7ee7542fd90b58fe18289cddbb247b3248`
- Change: `docs: record vtext structure extraction proof`

Hard review artifact:

- Markdown: `docs/source-system-loop8-hard-mission-review-2026-06-06.md`
- PDF: `/Users/wiz/Library/Mobile Documents/com~apple~CloudDocs/Choir Mission Reports/source-system-loop8-hard-mission-review-2026-06-06.pdf`
- PDF sanity check: `pdfinfo` reports 12 letter-size pages, unencrypted, tagged, 280604 bytes.
- Text sanity check: `pdftotext` starts with the expected cover, contents, mission, contract, method, review head, and `checkpoint_incomplete` status.

Key Loop 8 behavior and evidence commits:

- `9fe7a2a4956909b21c672016996d00400f7f4421`: VText toolbar/publish/result chrome stabilization.
- `e7fefc83c50e4e4d264721d02b5ce44f9b2ca6dc`: first structured rich export correctness.
- `5435e48df9840886565e6a47faf866be5265e676`: DOCX profile/source appendix polish.
- `5e9722e1fe0117bf6b1093e80667a7da273ae8e3`: nested URL source targets preserved into DOCX hyperlinks and source manifest.
- `143db3ba407d9e87297490cc03cc4468faa9a3a9`: centralized export profile.
- `08c079770e59135c64c248675db7523bd766c5ca`: publication read/export/search extraction.
- `8c465e30210db923f4288f2261a16793f725297c`: VText structure-preservation extraction.

Representative staging proofs:

- `tests/vtext-authoring-history.spec.js`: publish menu, toolbar stability, latest/historical labels, policy payload.
- `tests/rich-export-artifacts.tmp.spec.js`: downloaded HTML/DOCX/PDF artifacts and metadata.
- `tests/rich-export-docx-url-staging.tmp.spec.js`: DOCX URL hyperlinks and custom XML source manifest.
- `tests/rich-export-profile-staging.tmp.spec.js`: shared default-professional profile across API/HTML/DOCX/PDF.
- `tests/publication-read-extraction-staging.tmp.spec.js`: public publication resolve/export path after service extraction.
- `tests/vtext-structure-extraction-staging.tmp.spec.js`: deployed table restoration, source metadata carry-forward, stale-save rejection, allowed rebase.

All temporary staging specs were deleted after proof.

## Current Shape

Current large-file evidence:

```text
frontend/src/lib/VTextEditor.svelte               2794 lines
frontend/src/lib/VTextToolbar.svelte               590 lines
frontend/src/lib/VTextPublicationResult.svelte     283 lines
frontend/src/lib/VTextCompareMergePanel.svelte     281 lines
frontend/src/lib/vtext-editor-state.ts             140 lines
frontend/src/lib/vtext-markdown-serializer.ts       95 lines

internal/runtime/vtext.go                         2178 lines
internal/runtime/vtext_import.go                   810 lines
internal/runtime/vtext_lineage.go                  504 lines
internal/runtime/vtext_source_repairs.go           336 lines
internal/runtime/vtext_merge.go                    533 lines
internal/runtime/vtext_agent_revision.go          1074 lines
internal/runtime/vtext_structure.go                324 lines

internal/platform/service.go                       684 lines
internal/platform/service_publication_read.go      629 lines
internal/platform/export_formats.go                 71 lines
internal/platform/export_docx.go                   430 lines
internal/platform/export_html.go                   198 lines
internal/platform/export_pdf.go                    292 lines
internal/platform/publication_document.go          372 lines
```

This is a real simplification, not just a shuffle: toolbar chrome, publish
result chrome, DOM serialization, compare/merge UI, editor-state helpers,
runtime import/lineage/source-repair/merge/appagent/structure logic, platform
publication read/export paths, and format renderers now have named boundaries.

## Hard Findings

### Finding 1: Loop 8 is not complete until final pruning rejects weak paths

Severity: high.

The mission has many successful extraction checkpoints, but extraction can hide
dead paths as easily as remove them. The final review still needs a deliberate
search for stale selectors, duplicate CSS, unused helper branches, old export
fallbacks, compatibility aliases, and test-only shims that survived because
they were not in the current slice.

Next probe:

- Run a dead-path audit over VText frontend modules, runtime VText helpers,
  platform export renderers, and source contract adapters.
- Delete only paths with search/test evidence and product-path proof where
  behavior is touched.

### Finding 2: Export profiles are a contract spine, not yet a firm-style product surface

Severity: medium.

The default professional profile is now real and shared by renderers, but there
is no firm-specific profile selection, no stored firm template, and no product
path for users to choose citation placement or house typography. This is
acceptable for Loop 8 if recorded as residual risk, but it should not be
represented as complete customization.

Next probe:

- Add a non-user-facing profile registry test or fixture profile before adding
  UI. Prove the renderers can consume a second profile without changing source
  metadata semantics.

### Finding 3: PDF layout is structurally correct but not a full professional layout engine

Severity: medium.

PDF now renders headings, paragraphs, lists, tables, source markers, appendix,
and XMP metadata from the `PublicationDocument` spine. It is no longer raw
Markdown in a PDF container. It is still a simple renderer. Long tables,
pagination, widows/orphans, footnotes/endnotes, headers/footers, and PDF/A
associated-file behavior remain future work.

Next probe:

- Add a long-document export fixture with multi-page table and source appendix
  extraction checks. Decide whether to keep the custom renderer or move PDF
  rendering behind a stronger paged-layout engine.

### Finding 4: Source contract alignment is improved but still split by language

Severity: medium.

Backend `internal/sourcecontract` and frontend source-contract helpers are
aligned through tests, but the contract is not generated from one schema for
all consumers. Rich export now depends on the backend contract, which makes
drift more visible but not impossible.

Next probe:

- Promote the source contract matrix/schema into a generated Go/TypeScript
  artifact or add an explicit compatibility test that fails on new enum/state
  additions without frontend/backend updates.

### Finding 5: Performance is constrained by design, not benchmarked

Severity: low to medium.

The extractions intentionally avoid new network calls and repeated publication
bundle loads. CI and staging behavior passed. But the mission objective asks
for no measurable performance cost, and the evidence is design-based rather
than benchmark-based.

Next probe:

- Add lightweight timing assertions or logged timings around publication export
  generation and VText load/save hot paths, comparing before/after only where
  historical baselines exist.

## Residual Risks

- Some old compatibility aliases may still be necessary, but their current
  live-use status has not been fully audited.
- `VTextEditor.svelte` remains large at 2794 lines; the next valuable frontend
  boundary is likely document surface/source journal handling.
- `internal/runtime/vtext_agent_revision.go` is over 1000 lines and will need
  a second pass around prompt construction versus mutation orchestration.
- Rich DOCX source references use visible source markers and appendix entries;
  footnote/endnote output remains a profile-level future improvement.
- The final hard-review PDF itself proves reporting, not product completion.

## Recommended Next Loop

1. Run a dead/weak path audit with search evidence over VText frontend,
   runtime VText, platform export, and source contracts.
2. Delete one small stale path at a time, with tests and staging proof when the
   product path changes.
3. Add a second non-user-facing export profile fixture to prove profile
   extensibility without UI.
4. Add a long-export fixture for PDF/DOCX pagination and appendix durability.
5. Update the mission status to `checkpoint_incomplete` with this review and
   carry remaining risks into the next MissionGradient target.

## Checkpoint Status

Status: `checkpoint_incomplete`.

Loop 8 is safe to continue from the current deployed state. The rollback
reference for deployed behavior is the previous main commit before
`8c465e30`, and GitHub `origin/main` contains all landed checkpoints. The next
realism axis is adversarial pruning: remove weak paths only where the evidence
proves they are not live contracts.
