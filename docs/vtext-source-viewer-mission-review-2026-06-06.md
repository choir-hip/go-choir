# VText Source Viewer Mission Review - 2026-06-06

Status: review checkpoint after deployed guest proof

Mission under review:
[mission-vtext-source-viewer-reader-mode-hardening-v0.md](mission-vtext-source-viewer-reader-mode-hardening-v0.md)

Baseline: `eef70b6a900d7994d22292192730444086898ada`

Latest pushed commit reviewed: `a9facaff` with behavior deployed at
`9be193eaa2e2a155653f5e8e1c30ae760221e155`.

## Review Stance

This is a hard review of the whole mission and current system state around
VText source readers, citation transclusions, publication snapshots, Markdown
table preservation, and source UI simplification. It uses three lenses:

- Cognitive transform: the real object is the VText source graph, not a source
  card UI. The load-bearing variable is whether a reader can inspect evidence
  without losing source identity, publication policy, or reading flow.
- GStack review stance: findings first, risk-weighted, with line/file evidence.
- Design-review stance: source UI is judged as a reading instrument. Card
  chrome, metadata-first hierarchy, and text collision are treated as product
  defects, not cosmetic preference.

## Findings

### P1 - Published source readers still rely on a boolean mode split, not an explicit source authority type

The latest fix correctly prevents public source windows from replacing the
publication reader snapshot with a mutable content item, and it avoids the
guest auth overlay. The implementation is still keyed on
`publishedRoutePath || publishedGuest` in
[ContentViewer.svelte](/Users/wiz/go-choir/frontend/src/lib/ContentViewer.svelte:25).

That is acceptable for the current mission, but the deeper owner should be a
typed launch contract such as `sourceReaderAuthority: "publication_snapshot" |
"content_item" | "source_service_item"`. Today the mode split is implicit:

- published readers prefer `sourceEntityReaderSnapshot` before `item.text_content`;
- live/editable readers prefer `item.text_content` first;
- public readers skip the content-item fetch only if a snapshot exists.

Relevant code:
[ContentViewer.svelte](/Users/wiz/go-choir/frontend/src/lib/ContentViewer.svelte:30),
[ContentViewer.svelte](/Users/wiz/go-choir/frontend/src/lib/ContentViewer.svelte:43).

Risk: the next source type can accidentally bypass the intended authority
ordering or reintroduce a private fetch in public mode.

Recommendation: promote source-reader authority into
`sourceEntityLaunchPayload` and app context, then make `ContentViewer` switch
on that explicit value. Keep the existing fallback order as behavior, but stop
encoding authority as route presence.

### P2 - Markdown table repair is general enough for the observed bug, but still a Markdown-shaped stabilizer rather than canonical table structure

The table regression was repaired without hardcoding glossary terms. The
runtime now normalizes table-shaped tail rows after table extraction and
revision stabilization:
[vtext.go](/Users/wiz/go-choir/internal/runtime/vtext.go:2335),
[vtext.go](/Users/wiz/go-choir/internal/runtime/vtext.go:2365).

This is the right near-term repair. It protects malformed rows such as a final
line missing the trailing pipe, and it handles escaped pipes in tests. It is
still a Markdown syntax stabilizer. It does not yet make table blocks a
structured VText document object.

Risk: future table corruption can move to cases that are not "starts with pipe,
missing final pipe", such as inserted blank lines inside cells, column-count
drift, or rendered-editor DOM mutations.

Recommendation: treat this as a bridge. The next table axis should add an
internal table-block representation or at least a normalized table AST boundary
shared by import, render, edit/save, revise, and export.

### P2 - Source reader UI is much better, but still shares one component with generic media/content previews

`ContentViewer.svelte` now renders source readers as content-first pages, hides
the generic eyebrow, uses `Open original`, moves source details below the
reader, and keeps evidence in normal flow:
[ContentViewer.svelte](/Users/wiz/go-choir/frontend/src/lib/ContentViewer.svelte:99),
[ContentViewer.svelte](/Users/wiz/go-choir/frontend/src/lib/ContentViewer.svelte:145).

The component still owns unrelated media preview behavior for images, audio,
video, PDFs, imported URLs, publication snapshots, source entities, source
evidence, and provenance. That is a lot of responsibility for the component
that just caused a layout regression.

Risk: future media-preview changes can regress source-reader layout, and future
source-reader changes can regress media previews.

Recommendation: extract a `SourceReader.svelte` or source-reader branch module
once the current deployed mission settles. Keep generic media previews in
`ContentViewer`; move source-reader authority, reader snapshot selection, and
source apparatus into the source-reader module.

### P2 - Source note excerpting is useful now, but not selector-aware enough for legal citation precision

Inline transclusion notes now draw from richer reader snapshots and bound the
excerpt length:
[vtext-source-renderer.ts](/Users/wiz/go-choir/frontend/src/lib/vtext-source-renderer.ts:74),
[vtext-source-renderer.ts](/Users/wiz/go-choir/frontend/src/lib/vtext-source-renderer.ts:89).

This solves the "one-sentence stub despite available space" problem. It is not
yet selector-aware in the way legal/source work ultimately needs. The helper
chooses bounded sentences from the reader snapshot; it does not preferentially
center the selected quote with surrounding context.

Risk: the inline note can show broadly relevant source content but omit the
exact sentence that substantiates the marker, especially when a long source
snapshot starts with general context.

Recommendation: make inline excerpts quote-centered when a selector quote is
present: selected quote plus one bounded neighboring sentence before/after,
falling back to the current snapshot excerpt when no selector is available.

### P3 - Duplicate-window behavior is improved, but source comparison mode is undefined

The mission changed desktop opening behavior so source windows can dedupe by
`singletonKey`:
[desktop.js](/Users/wiz/go-choir/frontend/src/lib/stores/desktop.js:283).

That is correct for normal reading. The product has not defined how a user
intentionally opens two copies for comparison.

Risk: future users may need side-by-side source comparison and work around the
dedupe mechanism in ways that recreate window clutter.

Recommendation: keep dedupe as the default, and later add an explicit "Open
another copy" or "Compare source" affordance if source comparison becomes a
real workflow.

## What Is Proven

- Source viewer text-on-text regression was reproduced from owner screenshots
  across ABA and OVH sources, then fixed.
- Source evidence now renders below reader content in local geometry tests and
  deployed owner/guest proof.
- Article-side source notes show richer bounded source substance instead of a
  one-sentence stub.
- Appendix glossary final-row table loss was reproduced and repaired with a
  general malformed table-row stabilizer, not a glossary special case.
- The owner legal-cloud publication opens readable source windows on staging.
- A fresh unauthenticated guest context opens the same owner publication and
  reads public source windows for ABA Formal Opinion 512 and non-ABA OVHcloud.
- Public source windows now use publication-carried reader snapshots and avoid
  private content-item auth renewal.
- Source-window duplicate behavior is fenced by `singletonKey`.
- CI and Node B deploy passed for the final behavior commit.

## Verification Evidence

- Latest behavior commit: `9be193eaa2e2a155653f5e8e1c30ae760221e155`.
- CI: `27053665973`, passed non-runtime Go tests, integration smoke, runtime
  shards 0-3, frontend build, Go vet/build, and Node B deploy.
- FlakeHub: `27053665978`, passed.
- Staging health: proxy and sandbox deployed at
  `9be193eaa2e2a155653f5e8e1c30ae760221e155`, deployed at
  `2026-06-06T05:23:54Z`.
- Local focused verification:
  - `pnpm --dir frontend exec playwright test tests/vtext-source-entities.spec.js`
    passed all 8 tests.
  - `pnpm --dir frontend build` passed.
- Deployed guest proof:
  - route:
    `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`;
  - fresh unauthenticated browser context;
  - `Guest published VText` visible;
  - ABA source reader bottom `655.828125`, evidence top `680.828125`, overlap
    `false`;
  - OVH source reader bottom `632.203125`, evidence top `657.203125`, overlap
    `false`;
  - no `[data-auth-overlay]` appeared.

## Export And Canonicality Evidence

The mission preserved the VText/import/export direction, but the final deployed
guest proof did not rerun a live owner Markdown export. Current test coverage
does cover the relevant contracts:

- `vtext-source-service-publication.spec.js` checks publication export policy,
  Markdown export, text export, and omission of UI labels like `Open source`
  from text export.
- `vtext-markdown-lineage.spec.js` checks imported Markdown advances from v0
  source artifact to canonical `.vtext`, and that Markdown export returns the
  v1 VText content with `.md` filename.

Residual proof gap: no final deployed owner-account export download was captured
after `9be193ea`. This is a proof-strength issue, not evidence of a current
regression.

## Simplification Review

Good simplifications completed:

- Removed the `requestAnimationFrame` measurement shim from the source reader
  layout.
- Moved source evidence into normal document flow.
- Reused shared source-renderer helpers for reader snapshot selection.
- Added source-window singleton identity instead of letting repeated clicks
  create window clutter.
- Avoided source-specific fixes for ABA, OVH, Appendix A, glossary, or
  `Work product`.

Still worth simplifying:

- Split source reader authority from route/guest booleans.
- Extract source-reader UI from generic content/media preview UI.
- Make table preservation a structured VText concern instead of Markdown
  syntax repair.
- Make source excerpting selector-centered, not just first-sentences bounded.
- Decide how Web Lens cleaned Markdown and source reader snapshots share one
  reader-mode pipeline.

## Current-State Risk Register

- Source acquisition quality remains uneven. Some source snapshots are concise
  summaries, and some scraped content still carries cookie/banner noise.
- Long PDFs need page/selector-aware readers; current reader snapshots are
  source summaries or cleaned Markdown, not legal-grade page citation surfaces.
- Public source policy is now safer for carried snapshots, but licensed/private
  source omission UX still needs a dedicated proof path.
- Local publication-source test hit `502 {"error":"failed to publish vtext"}`
  during this run before reaching its UI assertions. Deployed publication
  proof succeeded on the owner artifact, but the local harness failure should
  be investigated separately.
- The final report PDF is a review artifact, not a product-path verifier.

## Completion Judgment

The core mission objective is substantially satisfied for the source viewer,
inline transclusion, table-row regression, owner proof, guest/public proof,
review, and simplification axes.

Do not treat this as the end of the broader VText/source program. The next
realism axis is source acquisition and reader-mode quality: clean web/PDF
sources into richer Markdown snapshots, make inline excerpts selector-centered,
and make source-reader authority explicit.
