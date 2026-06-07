# Source System Loop 8 Hard Mission Review

Date: 2026-06-06

Mission: `docs/mission-source-system-loop8-simplify-v0.md`

Requirements contract: `docs/source-external-data-publication.md`

Operating method: `docs/missiongradient-method.md`

Parent evidence ledger: `docs/mission-source-system-simplify-secure-smart-v0.md`

Current behavior head: `0346495c393f50cf3f93d5f808fb874e9b1c4590`

Latest mission evidence checkpoint before this review:
`24e423738c4e93d7b21a0e27d5d7fc15a349ce6b`

Status: `complete_with_residual_risks`

## Verdict

Loop 8 achieved its mission target. The source/VText/publication system is
materially more correct, more professional, more secure against accidental
contract drift, and easier to change.

The high-risk user-facing regressions that motivated the loop were fixed and
staging-proven: the VText publish policy banner was removed from the reading
surface, toolbar dimensions and latest/historical labels are stable, published
result controls no longer collide with the document, rich HTML/DOCX/PDF exports
are format-native instead of Markdown copied into containers, source metadata
is visible and embedded in rich exports, structure preservation protects the
legal proposal/table path, and the newly observed mobile inline source
transclusion overflow is fixed on staging.

The simplification work also moved the system away from monolith pressure:
frontend VText chrome/source/publication helpers, backend VText import,
lineage, source repair, appagent revision, merge, structure, and diagnosis
logic, and platform publication/export paths now have named boundaries with
focused tests and staging evidence where behavior was touched.

Do not read this as "nothing remains." It means the requested Loop 8 artifact
is complete. The remaining items are residual risks and next-loop candidates,
not blockers to this mission's acceptance.

## Cognitive Review

Selected transforms:

1. Depth extraction: the real artifact is not a cleaner file tree. It is a
   source-rich document system whose evidence survives editing, publication,
   guest reading, and professional external formats.
2. Verifier inversion: export correctness is proven by inspecting HTML,
   WordprocessingML, PDF text/layout, embedded metadata, and public reader
   behavior, not by checking file extensions.
3. Boundary audit: Source Viewer remains the durable artifact reader and Web
   Lens remains explicit live/original inspection. Cleanup that blurs that
   boundary is regression even if it removes code.
4. Anti-Goodhart check: line-count reduction only counted when responsibility
   boundaries improved and tests proved behavior did not move.
5. Mobile realism: content-forward reading is not proven until source
   transclusions are stable on phone-width surfaces, not only desktop windows.

Changed review stance:

- accept extraction checkpoints only with named local proof and CI/deploy/
  staging proof when behavior is touched;
- treat rich export as a source-contract consumer, not a download feature;
- close Loop 8 now, but carry style-profile, PDF engine, and schema-generation
  work as explicit future axes.

## Completion Audit

| Requirement | Evidence | Status |
| --- | --- | --- |
| Preserve requirements contract and MissionGradient method | Active mission names `docs/source-external-data-publication.md` and `docs/missiongradient-method.md`; parent ledger remains separate. | Proven |
| Document newly confirmed behavior problems before fixes | Problems L8-1 through L8-7, Problems 47-49, and extraction targets were documented before behavior-changing code. | Proven |
| Stabilize VText publish/published-result chrome | Problems L8-1, L8-2, and L8-3 fixed with deployed `vtext-authoring-history.spec.js` proof. | Proven |
| Stable toolbar dimensions and latest/historical labels | Staging test asserts latest-to-historical navigation preserves toolbar height and label semantics. | Proven |
| Non-overlapping publish/download controls | Publish menu/result panel moved behind explicit controls; staging proof verified clickability and policy payload. | Proven |
| Rich export uses VText/publication semantics, not raw Markdown | Problem L8-4 fixed with shared `PublicationDocument` and source manifest spine; tests assert no raw Markdown leakage in rich formats. | Proven |
| Format-native HTML | Problem L8-5 fixed; HTML has semantic blocks, profile CSS, source appendix, JSON-LD, and embedded source manifest. | Proven |
| Format-native PDF | Problem L8-6 fixed; PDF renders blocks, lists, tables, visible markers, source appendix, and XMP manifest. | Proven with residual polish risk |
| Format-native DOCX | Problem L8-7 fixed; DOCX has styles, table rendering, visible markers, source appendix, custom XML manifest, custom properties, and native hyperlinks. | Proven with residual footnote/endnote polish risk |
| Source metadata embedded in every rich format | Staging proofs verified HTML manifest/profile scripts, DOCX custom XML/properties/hyperlinks, and PDF XMP/source manifest metadata. | Proven |
| Future firm-specific export profile spine | `publication_export_profile.go` defines typography, heading, table, citation, source detail, page/header/footer, and metadata policy consumed by renderers. | Spine proven; UI pending |
| Mobile source transclusion layout | Problem 49 fixed; deployed 390px Playwright proof asserts bounded document/rendered scroll width and aligned flow/note/body geometry. | Proven |
| Preserve Markdown/text export | Markdown export remains separate from rich renderers and is covered by publication/read/export tests. | Proven |
| Source Viewer default and Web Lens explicit live inspection | Source opening tests and publication proofs preserve Source Viewer as the durable-artifact open surface. Loop 8 did not weaken this boundary. | Proven for touched paths |
| Selector-rich transclusions and source snapshots through publication/export | Source-service, URL-backed, content-item, and publication export proofs verify source entities, transclusions, selectors, snapshots, retrieval spans, and export metadata. | Proven for tested source classes |
| Source acquisition policy and SSRF safety | Problem 47 preserved private-network rejection while enabling an explicit test override; sourcefetch tests passed. | Proven |
| Large frontend VText simplification | `VTextEditor.svelte` reduced to 2651 lines after extracting toolbar, publication result, serializer, compare/merge panel, editor state, publication context, source diagnosis, and source state helpers. | Proven |
| Large backend VText simplification | `internal/runtime/vtext.go` reduced to 1999 lines after extracting import, lineage, source repairs, merge, appagent revision, structure, and diagnosis helpers. | Proven |
| Platform publication/export simplification | `internal/platform/service.go` split publication read/export/search into `service_publication_read.go`; export renderers split by format/profile. | Proven |
| Dead/weak shortcut pruning | Unreachable HTML export fallback and remaining Markdown/text export boundary shortcut were pruned; larger stale-path hunting is a future quality axis. | Sufficient for Loop 8 |
| No source/publication security regression | Focused platform/sourcecontract/runtime tests and staging publication/export proofs passed after behavior-affecting changes. | Proven for touched paths |
| No measurable performance cost | Refactors avoid added network calls and repeated publication bundle loads; CI/staging proofs passed. Dedicated latency benchmark was not run. | Acceptable residual risk |
| CI, Node B deploy identity, staging proof | Latest behavior commit `0346495c` passed CI `27079260064`, FlakeHub `27079260070`, Node B deploy `79922158704`, and `/health` reported the same SHA. | Proven |
| Visual/download inspection of DOCX/PDF/HTML | Mission evidence includes downloaded staging artifacts and HTML/PDF/DOCX visual or structural inspection for rich export. | Proven |
| Rollback refs and residual risks | Behavior commits, deploy jobs, and prior main SHAs are in GitHub history; residual risks are named below. | Proven |
| Final hard review report and PDF in iCloud Drive | This report was updated and rendered to iCloud Drive. | Proven |

## Evidence Summary

Latest deployed behavior checkpoint:

- Commit: `0346495c393f50cf3f93d5f808fb874e9b1c4590`
- Change: `frontend: bound mobile source transclusion flow`
- CI: `27079260064`, passed
- FlakeHub: `27079260070`, passed
- Node B deploy job: `79922158704`, passed
- Staging health: proxy and sandbox `deployed_commit=0346495c393f50cf3f93d5f808fb874e9b1c4590`, `deployed_at=2026-06-07T01:33:20Z`

Latest documentation checkpoint:

- Commit: `24e423738c4e93d7b21a0e27d5d7fc15a349ce6b`
- Change: `docs: record mobile source flow staging proof`

Hard review artifacts:

- Markdown: `docs/source-system-loop8-hard-mission-review-2026-06-06.md`
- PDF: `/Users/wiz/Library/Mobile Documents/com~apple~CloudDocs/Choir Mission Reports/source-system-loop8-hard-mission-review-2026-06-06.pdf`

Key Loop 8 behavior and evidence commits:

- `9fe7a2a4956909b21c672016996d00400f7f4421`: VText toolbar/publish/result chrome stabilization.
- `e7fefc83c50e4e4d264721d02b5ce44f9b2ca6dc`: first structured rich export correctness.
- `5435e48df9840886565e6a47faf866be5265e676`: DOCX/profile/source appendix polish.
- `5e9722e1fe0117bf6b1093e80667a7da273ae8e3`: nested URL source targets preserved into DOCX hyperlinks and source manifest.
- `143db3ba407d9e87297490cc03cc4468faa9a3a9`: centralized export profile.
- `08c079770e59135c64c248675db7523bd766c5ca`: publication read/export/search extraction.
- `8c465e30210db923f4288f2261a16793f725297c`: VText structure-preservation extraction.
- `0346495c393f50cf3f93d5f808fb874e9b1c4590`: mobile source transclusion overflow fix.

Representative staging proofs:

- `tests/vtext-authoring-history.spec.js`: publish menu, toolbar stability,
  latest/historical labels, policy payload.
- `tests/rich-export-artifacts.tmp.spec.js`: downloaded HTML/DOCX/PDF
  artifacts and metadata.
- `tests/rich-export-docx-url-staging.tmp.spec.js`: DOCX URL hyperlinks and
  custom XML source manifest.
- `tests/rich-export-profile-staging.tmp.spec.js`: shared default-professional
  profile across API/HTML/DOCX/PDF.
- `tests/publication-read-extraction-staging.tmp.spec.js`: public publication
  resolve/export path after service extraction.
- `tests/vtext-structure-extraction-staging.tmp.spec.js`: deployed table
  restoration, source metadata carry-forward, stale-save rejection, allowed
  rebase.
- `tests/vtext-source-entities.spec.js`: desktop side-note source flow,
  constrained stacked source flow, and mobile bounded source-flow geometry.

Temporary staging specs and Playwright scratch outputs were deleted after proof.

## Current Shape

Current large-file evidence:

```text
frontend/src/lib/VTextEditor.svelte              2651 lines
frontend/src/lib/VTextToolbar.svelte              590 lines
frontend/src/lib/VTextPublicationResult.svelte    276 lines
frontend/src/lib/VTextCompareMergePanel.svelte    281 lines
frontend/src/lib/vtext-source-flow.css            193 lines
frontend/src/lib/vtext-editor-state.ts            140 lines
frontend/src/lib/vtext-source-diagnosis.ts         97 lines
frontend/src/lib/vtext-source-state.ts             88 lines
frontend/src/lib/vtext-publication-context.ts      70 lines

internal/runtime/vtext.go                        1999 lines
internal/runtime/vtext_agent_revision.go         1074 lines
internal/runtime/vtext_import.go                  810 lines
internal/runtime/vtext_merge.go                   533 lines
internal/runtime/vtext_lineage.go                 504 lines
internal/runtime/vtext_source_repairs.go          336 lines
internal/runtime/vtext_structure.go               324 lines
internal/runtime/vtext_diagnosis.go               188 lines

internal/platform/service.go                      684 lines
internal/platform/service_publication_read.go     611 lines
internal/platform/export_docx.go                  430 lines
internal/platform/publication_document.go         372 lines
internal/platform/export_pdf.go                   292 lines
internal/platform/export_html.go                  198 lines
internal/platform/publication_export_profile.go    93 lines
internal/platform/export_formats.go                87 lines
```

This is a real simplification, not only a shuffle: toolbar chrome, publish
result chrome, DOM serialization, compare/merge UI, editor-state helpers,
publication context helpers, source diagnosis/state helpers, runtime import,
lineage, source-repair, merge, appagent, structure, and diagnosis logic,
platform publication read/export paths, and rich export renderers now have
named boundaries.

## Hard Findings

### Finding 1: Export profile extensibility is a spine, not a product surface

Severity: medium.

The default-professional profile is real and shared by renderers, but there is
no firm-specific stored profile, template registry, or UI for choosing citation
placement and house typography. Loop 8 satisfies the requested extensible spine;
the product-facing customization path belongs in a later mission.

Next probe: add a non-user-facing second profile fixture before UI work. Prove
HTML/DOCX/PDF consume it without changing source metadata semantics.

### Finding 2: PDF layout is structurally correct but not a full publishing engine

Severity: medium.

PDF now renders from the document spine with visible citations, tables,
appendix, and XMP metadata. It is still a simple renderer. Long tables,
widows/orphans, footnotes/endnotes, headers/footers, and PDF/A associated-file
behavior remain future work.

Next probe: add a long-document export fixture with multi-page table and
source appendix extraction checks. Then decide whether to keep the custom
renderer or move PDF behind a stronger paged-layout engine.

### Finding 3: Source contract alignment is improved but still split by language

Severity: medium.

Backend `internal/sourcecontract` and frontend source-contract helpers are
aligned through tests, but the contract is not generated from one schema for
all consumers. Rich export now depends on the backend contract, making drift
more visible but not impossible.

Next probe: promote the source contract matrix/schema into generated Go and
TypeScript artifacts, or add an explicit compatibility test that fails on enum
additions without frontend/backend updates.

### Finding 4: Performance is constrained by architecture, not benchmarked

Severity: low to medium.

The refactors avoid new network calls and repeated publication bundle loads.
CI and staging product paths passed. No dedicated before/after latency
benchmark was run.

Next probe: add lightweight timings around publication export generation and
VText load/save hot paths where historical baselines exist.

### Finding 5: More cleanup is possible, but marginal value is now lower

Severity: low.

`VTextEditor.svelte` and `internal/runtime/vtext.go` are still large, but the
highest-risk coupling has already been extracted. Further line-count work is
useful only when tied to a specific user-facing bug, contract drift, or
testable boundary.

Next probe: prefer focused product bugs or a new MissionGradient target over
open-ended extraction.

## Residual Risks

- Some old compatibility aliases may still be necessary; their live-use status
  has not been exhaustively audited.
- `VTextEditor.svelte` remains large at 2651 lines; the next frontend boundary
  is likely document surface/source journal handling, but only if a real
  product change needs it.
- `internal/runtime/vtext_agent_revision.go` is over 1000 lines and will need
  a second pass around prompt construction versus mutation orchestration.
- Rich DOCX source references use visible markers and appendix entries;
  footnote/endnote output remains a profile-level future improvement.
- The PDF renderer is good enough for Loop 8 correctness but not yet a
  full professional publishing engine.

## Rollback

GitHub `origin/main` contains all landed checkpoints. The latest deployed
behavior rollback reference is the previous behavior commit before
`0346495c393f50cf3f93d5f808fb874e9b1c4590`; docs-only commit
`24e423738c4e93d7b21a0e27d5d7fc15a349ce6b` does not require platform rollback.

## Recommended Next Mission

Start a new, narrower mission rather than continuing Loop 8 indefinitely:

```text
Run a focused source-document publishing polish mission: add a second export
profile fixture, strengthen long-document PDF/DOCX pagination proof, generate
shared source-contract types, and fix only newly observed product bugs with
problem documentation first.
```

## Final Status

Status: `complete_with_residual_risks`.

Loop 8 is complete. The requested UI stabilization, rich export spine,
source-metadata preservation, simplification/refactor sequence, staging proof,
hard review, and iCloud PDF report have been produced. Remaining items are
explicit future work, not hidden acceptance blockers for this mission.
