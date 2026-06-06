# MissionGradient v0: Source Viewer Reader-Mode Hardening And Source UX Simplification

Status: deployed_review_checkpoint_complete
Date: 2026-06-06

Requirements contracts:
[source-external-data-publication.md](source-external-data-publication.md),
[vtext-version-compare-merge-debuggability-spec.md](vtext-version-compare-merge-debuggability-spec.md),
[vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md](vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md)

Supersedes for the next run:
[mission-vtext-client-ready-source-transclusion-pretext-v0.md](mission-vtext-client-ready-source-transclusion-pretext-v0.md)

Related current-state review:
[vtext-mission-hard-review-2026-06-05.md](vtext-mission-hard-review-2026-06-05.md)

Final mission review:
[vtext-source-viewer-mission-review-2026-06-06.md](vtext-source-viewer-mission-review-2026-06-06.md)

## Run Checkpoint & Resumption State

status: deployed_review_checkpoint_complete

last checkpoint:

- Behavior-changing commit `eef70b6a900d7994d22292192730444086898ada`
  persists Web Lens snapshots as `ContentItem` artifacts instead of opening a
  prose-only VText wrapper with a fake source content ID.
- Staging `/health` reports proxy and sandbox deployed at `eef70b6a`, deployed
  at `2026-06-06T03:48:56Z`.
- This document is a documentation-first checkpoint for newly observed source
  viewer failures. Do not write source-viewer code before this problem record.

latest deployed repair checkpoint:

- Table repair landed in behavior commit `7c5dec1c7ed52e7fc6bc352907e86b191dee36f0`.
  The generic fix preserves malformed Markdown table tail rows through
  renderer parsing and VText revision stabilization without naming glossary,
  Appendix A, or `Work product`.
- Source-reader repair landed in behavior commit
  `518c5fccc211e557a927f46c76c83790c30d104e`. CI run `27052988898` passed,
  FlakeHub publish workflow `27052988902` passed, and staging `/health`
  reported deployed commit `518c5fccc211e557a927f46c76c83790c30d104e` at
  `2026-06-06T04:50:11Z`.
- Local verification before deploy: `pnpm --dir frontend exec playwright test
  tests/vtext-source-entities.spec.js` passed all 7 source-entity tests, and
  `pnpm --dir frontend build` passed.
- Authenticated Comet proof on the owner publication
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6` after
  reload showed the deployed source-reader mode: no `FILES CONTENT` source
  chrome, a quieter `Open original` source header, reader text first, and
  `Source details` after the reader body.
- Comet proof on the same owner publication showed the Appendix glossary
  final `Work product` row inside the rendered table, followed separately by
  `---` and `End of Proposal`.
- Comet proof showed the ABA Formal Opinion 512 source marker expands into a
  right-side journal note with multiple lines of source content while proposal
  text wraps around it. Opening it produced a readable source window, and
  expanding `Source evidence` displayed metadata below the reader body instead
  of overlapping source prose.
- Comet proof showed a second owner source, ABA Model Rule 1.6, also expands
  inline as a side note and opens a readable source window with reader text,
  reader-mode note, source citation, `Open original`, and collapsed source
  details.
- Source-window singleton behavior is repaired for windows opened under the
  deployed keyed path: a repeated open of the same source did not increase the
  window count. One stale pre-reload ABA source window remained after the app
  reload because it had been opened before singleton keys existed; this is a
  residual desktop-migration nuance, not proof that the new source-opening path
  still duplicates.
- Remaining limitation: the owner source snapshots are still concise
  summaries. The renderer now uses more available inline space when richer
  snapshot text exists, but improving real source depth requires the next
  source-acquisition/cleaned-Markdown reader-mode slice.

final review artifact checkpoint:

- Hard review report written to
  [vtext-source-viewer-mission-review-2026-06-06.md](vtext-source-viewer-mission-review-2026-06-06.md).
- PDF rendered to iCloud Drive:
  `/Users/wiz/Library/Mobile Documents/com~apple~CloudDocs/Choir Reports/vtext-source-viewer-mission-review-2026-06-06.pdf`.
- PDF verification: `pdfinfo` reported title `VText Source Viewer Mission
  Review`, producer `WeasyPrint 67.0`, 5 pages, PDF 1.7, size 40301 bytes.
- Review conclusion: the core mission objective is substantially satisfied for
  source viewer collision repair, richer inline transclusions, generic table
  stabilization, owner proof, guest/public proof, adversarial review, and code
  simplification. Residual risks remain for source acquisition quality,
  selector-centered excerpts, explicit source-reader authority typing, and
  structured VText table blocks.

export completion-audit problem checkpoint:

- Before marking the goal complete, a deployed owner-publication Markdown export
  check was run against
  `/api/platform/publications/export?route=/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6&format=md`.
- The export returned `format: md`, filename
  `choir-private-legal-cloud-proposal-vtext-pub270a62fb6.md`, a content hash,
  the proposal title, source marker/source text, and no source-window UI labels
  such as `Open source` or `Close`.
- The same export still rendered the final Appendix glossary row as malformed
  Markdown: `| **Work product** | Durable, reviewable output ... chat responses.`
  followed by a blank line, without the trailing `|`. The rendered reader table
  is repaired, but the publication Markdown export path is not yet preserving
  the final row delimiter for the actual owner document.
- This contradicts the mission invariant that Markdown export remains a faithful
  projection of canonical VText/table structure. Do not mark this mission
  complete until the export path is repaired and the deployed owner export
  check passes.

post-fix adversarial review checkpoint:

- Cognitive transform - Depth Extraction: the source viewer is not a generic
  media preview with metadata bolted on; it is a source-inspection instrument
  for the VText source graph. Any code path that treats it as "just content"
  risks reintroducing card chrome, metadata-first layout, or iframe-only
  fallback behavior.
- Cognitive transform - Load-bearing Variable: the real load-bearing variable
  is not whether a source window opens. It is whether a reader can inspect the
  cited source without losing reading flow, provenance, source identity, or
  publication policy.
- Cognitive transform - High-information Probe: the best next probe is not a
  new source fixture. It is whether the deployed owner and guest publication
  can both open multiple source windows while `ContentViewer` sheds the
  measurement/metadata workaround and still passes geometry tests.
- GStack review finding: `ContentViewer.svelte` now contains source-reader
  semantics, generic media preview behavior, source apparatus rendering, and a
  `requestAnimationFrame` descendant-measurement shim. That shim was useful to
  get the first deployed overlap repair, but it is a weak abstraction: it hides
  layout ownership in imperative DOM measurement instead of making reader text
  and source apparatus normal document flow.
- Design-review finding: the deployed source reader is materially better than
  the screenshot failure, but its title/source header, reader body, and source
  details should remain an academic-reader hierarchy. The next design
  simplification should remove measurement scaffolding and keep the quiet
  content-first source-reader mode, not add new cards or pills.
- Problem to fix next, after this documentation checkpoint: simplify
  `ContentViewer` so source reader content and apparatus participate in normal
  block flow without the `readerShellMinHeight` measurement shim. Preserve the
  same geometry tests, source-window singleton behavior, and media preview
  behavior.

simplification implementation checkpoint:

- Investigation during the simplification found that removing the measurement
  shim made the focused source-reader geometry test fail: the reader article
  bottom was below the reader-shell bottom, so expanded `Source evidence`
  could begin before the visible table ended.
- Root cause: `ContentViewer` is a flex column and the source reader shell was
  allowed to shrink. The deployed measurement shim had compensated by forcing
  a measured `min-height`; the structural fix is to make the reader shell and
  source apparatus non-shrinking flex items so normal document flow determines
  their vertical position.
- Code simplification removed the `afterUpdate` import, `readerShellEl`,
  `readerShellMinHeight`, `measureQueued`, `measureReader`, and
  `requestAnimationFrame` measurement path from `ContentViewer.svelte`.
- Source-reader tables now keep the `.table-scroll` wrapper as the horizontal
  overflow owner while the table itself participates in normal table layout.
- Verification:
  - `pnpm --dir frontend exec playwright test
    tests/vtext-source-entities.spec.js -g "content-item text sources"` passed.
  - `pnpm --dir frontend exec playwright test
    tests/vtext-source-entities.spec.js -g "roundtrips rendered markdown
    tables"` passed after a prior full-suite desktop-readiness timeout.
  - `pnpm --dir frontend exec playwright test
    tests/vtext-source-entities.spec.js` passed all 7 tests.
  - `pnpm --dir frontend build` passed.
- Behavior-changing simplification landed in commit
  `96fe4b6d04c71b883cbb439937e61aae764d2843`.
- CI run `27053309226` passed all required jobs: non-runtime Go tests,
  integration smoke, runtime shards 0-3, frontend build, and Go vet/build.
  FlakeHub publish run `27053309229` also passed.
- Node B staging `/health` reported deployed commit
  `96fe4b6d04c71b883cbb439937e61aae764d2843`, deployed at
  `2026-06-06T05:05:43Z`.
- Authenticated Comet proof on the owner publication
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6` after
  the deploy showed Computer Use click support available again, the published
  reader loading under the deployed bundle, and the ABA Model Rule 1.6 inline
  source note expanded as a right-side journal note while surrounding proposal
  prose continued in the main reading column.
- The same Comet proof foregrounded the ABA Model Rule 1.6 source window under
  the deployed bundle. The source window showed the content-first reader shape:
  title and `Open original`, source text, reader-mode note, source citation,
  then collapsed `Source evidence` below the reader body. This preserved the
  previous text-on-text fix after removing the imperative measurement shim.
- Limitation: the Comet coordinate path was unreliable for expanding the
  already-open source window's `Source evidence` disclosure in this final
  deployed proof pass; earlier deployed proof at `518c5fcc` did expand source
  evidence without overlap, and local geometry tests cover the expanded
  apparatus after the simplification. Guest/public source-window proof is
  still outstanding and must not be claimed complete.

guest/public proof problem checkpoint:

- A fresh unauthenticated Playwright context opened the owner publication
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6` on
  staging and saw `Guest published VText`, proving the probe was not using the
  authenticated Comet owner session.
- The guest publication exposes public source markers for ABA Formal Opinion
  512, ABA Model Rule 1.6, Hetzner data centers, OVHcloud Hosted Private
  Cloud, NixOS rollback, GDPR Article 32, and Qdrant search.
- The public resolve API for the same route carries fuller
  `reader_snapshot.text_content` for the ABA and OVH source entities. Example:
  ABA Formal Opinion 512 includes the fuller reader summary beginning "The ABA
  Standing Committee on Ethics and Professional Responsibility published
  Formal Opinion 512..."; OVH includes the fuller reader summary beginning
  "# OVHcloud Hosted Private Cloud Service Offerings".
- The guest inline ABA source note used the fuller cleaned reader text, but
  opening the ABA source window showed only the shorter selector quote:
  "Lawyers using generative artificial intelligence tools must consider duties
  including competence, confidentiality, communication, supervision, candor,
  and reasonable fees."
- Root-cause hypothesis before code: `ContentViewer` loads the target content
  item by `content_id` and prefers `item.text_content` over the publication
  source entity's `reader_snapshot.text_content`. For public source windows,
  that can replace the publication-carried reader snapshot with an older,
  shorter, or selector-like content body. The general repair should make source
  readers prefer the source entity's own reader snapshot, then fall back to
  transclusion/selector text and loaded content items. This is not
  source-specific and preserves VText publication snapshots as the public
  source contract.
- Fix applied locally after the documentation checkpoint: `ContentViewer`
  now distinguishes published source readers from live/editable content source
  readers. Published source readers prefer
  `sourceEntity.reader_snapshot.text_content`, then loaded content-item text,
  then transclusion/selector fallback. Live content-item source windows keep
  the previous behavior: loaded content-item text first, then source snapshots
  as fallback.
- Regression coverage added:
  `VText published source readers prefer publication snapshots over loaded
  content items` launches a source window with both a target content item and a
  publication reader snapshot, waits for source evidence to prove the content
  item loaded, and asserts the rendered reader still shows the publication
  snapshot rather than the mutable content-item body or selector quote.
- Local verification:
  - `pnpm --dir frontend exec playwright test
    tests/vtext-source-entities.spec.js -g "content-item text
    sources|published source readers"` passed 2 tests.
  - `pnpm --dir frontend exec playwright test
    tests/vtext-source-entities.spec.js` passed all 8 tests.
  - `pnpm --dir frontend build` passed.
- Attempted local publication regression
  `pnpm --dir frontend exec playwright test
  tests/vtext-source-service-publication.spec.js -g "publishes public
  content-item sources"` did not reach the source-window assertion because the
  local publish endpoint returned `502 {"error":"failed to publish vtext"}`.
  That is recorded as a local harness/server limitation for this slice, not as
  deployed guest proof.
- Deployed guest proof after `26d94b46` exposed a second public-source issue:
  after the first guest source-window interaction, the unauthenticated page
  displayed the auth overlay and intercepted clicks on later public source
  markers such as OVH. Root-cause hypothesis before code: even when a published
  source entity carries a reader snapshot, `ContentViewer` still calls
  `/api/content/items/{content_id}` through `fetchWithRenewal`. Public
  publication source windows should not need an authenticated content-item fetch
  when the publication already carries the source reader snapshot. The general
  repair is to skip private content-item loading for published source readers
  that have a publication reader snapshot, while preserving authenticated/live
  content-item loading when no publication snapshot is available.
- Follow-up local fix after this documentation checkpoint: `ContentViewer`
  skips target content-item loading when the window is a published source
  reader and the source entity already carries a reader snapshot. The
  publication snapshot is therefore self-contained for guest readers and cannot
  trigger auth renewal while a public source is opened.
- Regression coverage updated so the published-source fixture asserts the
  source window shows the publication snapshot and reference metadata, does not
  render the mutable target content-item body, does not fall back to selector
  text, and does not require content-item SHA metadata from a private fetch.
- Local verification after the follow-up fix:
  - `pnpm --dir frontend exec playwright test
    tests/vtext-source-entities.spec.js` passed all 8 tests.
  - `pnpm --dir frontend build` passed.
- Behavior follow-up landed in commit
  `9be193eaa2e2a155653f5e8e1c30ae760221e155`. CI run
  `27053665973` passed, including non-runtime Go tests, integration smoke,
  runtime shards 0-3, frontend build, Go vet/build, and Node B deploy.
  FlakeHub publish run `27053665978` passed. Staging `/health` reported proxy
  and sandbox deployed at `9be193eaa2e2a155653f5e8e1c30ae760221e155`,
  deployed at `2026-06-06T05:23:54Z`.
- Deployed guest proof on the actual owner publication used a fresh
  unauthenticated Playwright context with empty storage state and confirmed
  `Guest published VText` was visible.
- The guest proof opened two source markers in
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`:
  ABA Formal Opinion 512 and non-ABA OVHcloud Hosted Private Cloud. Both opened
  readable source windows from publication-carried source snapshots. The ABA
  window contained "The ABA Standing Committee on Ethics and Professional
  Responsibility..." and did not render the shorter selector quote as the body.
  The OVH window contained "OVHcloud describes Hosted Private Cloud as a
  VMware-based private cloud service family..." and did not render the shorter
  selector quote as the body.
- Geometry proof from the deployed guest run:
  - ABA source reader bottom `655.828125`, source evidence top `680.828125`,
    overlap `false`.
  - OVH source reader bottom `632.203125`, source evidence top `657.203125`,
    overlap `false`.
  No `[data-auth-overlay]` appeared during the guest source-window proof.

current artifact state:

- The legal-cloud proposal is a canonical `.vtext` publication with source
  entities, inline citation transclusions, cleaned source snapshots, Markdown
  export, and opened source windows.
- Article-side text source expansion has moved from nested cards toward a
  Pretext-routed journal flow.
- Opened source windows use `ContentViewer.svelte`, which is separate from the
  article-side Pretext flow and still has utility-app chrome, oversized
  typography, disclosure boxes, duplicate-window behavior, and weak visual
  regression coverage.

what was proven before this checkpoint:

- CI and Node B deploy passed for the recent source-flow, reader-snapshot, and
  Web Lens content-artifact behavior changes.
- Deployed Playwright proof passed for Pretext side-note and stacked journal
  flow, content-item source publication, and Web Lens snapshot import into
  durable content artifacts.
- Comet staging proof showed source windows can open and display reader-mode
  source content.

new evidence:

- Computer Use was re-checked on 2026-06-06. The plugin exposes callable
  `get_app_state`, `click`, `type_text`, `scroll`, and related Mac UI actions,
  and `/Applications/Comet.app/` is running. The next authenticated staging UI
  proof should therefore use Comet first, not the browser/API fallback, unless
  a specific action fails and that failure is recorded.
- Owner screenshot
  `/var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/TemporaryItems/NSIRD_screencaptureui_FgmWYm/Screenshot 2026-06-05 at 23.49.38.png`
  shows an ABA Formal Opinion 512 source window with `Source evidence`,
  `Source entity`, and `Provenance` disclosure rows overlapping reader text.
- Owner screenshot
  `/var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/TemporaryItems/NSIRD_screencaptureui_SCTK33/Screenshot 2026-06-05 at 23.50.02.png`
  shows the expanded `Source evidence` disclosure covering multiple lines of
  source prose and metadata.
- Owner screenshot
  `/var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/TemporaryItems/NSIRD_screencaptureui_Laklqc/Screenshot 2026-06-05 at 23.58.32.png`
  shows the same source-window overlap pattern on an OVHCloud source, proving
  this is not specific to ABA Formal Opinion 512.
- Owner screenshot
  `/var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/TemporaryItems/NSIRD_screencaptureui_kjek9s/Screenshot 2026-06-05 at 23.59.22.png`
  shows the article-side Pretext note has enough vertical space for more
  source content, but the transclusion body is still a one-sentence stub.
- Owner screenshot
  `/var/folders/28/gwvkv0wn6lq64jvqvmny5xnw0000gn/T/TemporaryItems/NSIRD_screencaptureui_tX1uyi/Screenshot 2026-06-06 at 00.03.37.png`
  shows the Appendix glossary table rendering correctly until the final
  `Work product` entry, which falls out of table formatting and renders as
  prose beginning with a pipe character.
- `frontend/src/lib/ContentViewer.svelte` renders the opened source as a
  flex-column utility page: hero header, `Open source` header link, reader
  article, then card-like `details.provenance` blocks.
- Existing tests assert source windows open and contain text, but do not assert
  no text overlap, disclosure geometry, duplicate-window behavior, or
  content-first visual hierarchy.

remaining error field:

- Source viewer text-on-text overlap must be fixed and covered by visual or
  geometry proof across several source windows, not only the ABA Formal Opinion
  512 example.
- Source viewer UI must become a content-first source reader, not a dashboard
  of app chrome and metadata cards.
- Article-side transclusion notes should show enough source substance for the
  common no-window path: users often click source points to inspect support
  inline without opening a separate source window and disrupting reading flow.
- Appendix glossary/table formatting consistency still has a narrower
  regression: a final row can fall out of the rendered table even after the
  earlier full `TermDefinition` collapse was repaired.
- Source window lifecycle should not accumulate duplicate windows for the same
  source during owner/client review.
- Source acquisition still needs a first-class cleaned Markdown reader pipeline
  with iframe/live web lens as fallback, not the primary proof.
- Publication source policy still needs explicit proof that authorized readers
  see permitted source snapshots while private/licensed metadata remains
  bounded.
- The next coding pass must simplify weak/dead paths while preserving the
  staging-proven source graph, VText canonicality, and export behavior.

highest-impact remaining uncertainty:

- Whether `ContentViewer` should stay as a generic media/content app with a
  source-reader mode, or whether source windows deserve a dedicated
  source-reader component that shares source graph contracts but owns a
  magazine/academic reader layout.

next executable probe:

- Build minimal reproductions for the ABA Formal Opinion source window and at
  least one non-ABA source window, first against staging with Comet if
  practical, otherwise with local fixtures that preserve the same
  `ContentViewer` payload shape. Measure collision rectangles for reader text,
  `details` summaries, and expanded evidence rows. Also measure article-side
  transclusion note height and content density so the note uses available
  space without becoming a full source window. Reproduce the appendix glossary
  final-row fall-out and inspect whether the canonical content lost the row's
  trailing table delimiter or whether render normalization is too strict.

suggested resume goal string:

```text
/goal Run docs/mission-vtext-source-viewer-reader-mode-hardening-v0.md as a Codex-operated MissionGradient mission. Start from deployed commit eef70b6a. Preserve canonical VText, source entities, citation transclusions, source publication policy, Markdown export, and staging proof. First reproduce and fix the generic source viewer text-on-text regression across multiple source windows with geometry/visual proof. Then make article-side transclusion notes show more useful source substance for readers who inspect citations inline without opening separate source windows. Also reproduce the appendix glossary final-row table-formatting loss and repair formatting consistency through general VText/Markdown structure preservation, not a glossary hardcode. Run cognitive transforms and gstack review/design-review for adversarial perspective before and after the first working fix, then simplify source viewer/source flow code paths and remove weak/dead abstractions while keeping the legal-cloud proposal source graph and opened source windows working for owner and guest readers.
```

rollback refs:

- Last deployed behavior-changing commit before this doc: `eef70b6a`.
- Prior source-flow rollback refs remain in
  [mission-vtext-client-ready-source-transclusion-pretext-v0.md](mission-vtext-client-ready-source-transclusion-pretext-v0.md).

## Problem Documentation First

This mission begins because the opened source viewer now has a newly observed
visual corruption:

> Source evidence and source metadata disclosure boxes overlap source prose in
> opened source windows. The problem appears across multiple source artifacts,
> including ABA Formal Opinion 512 and OVHCloud Hosted Private Cloud service
> offerings.

This is not just a CSS polish bug. The visible failure means an authorized
reader can open a source from a published VText and be unable to read the
evidence. That violates the publication contract that source markers are
inspectable transclusion points and weakens the client-ready legal-cloud demo.

No source-viewer code should be changed until this problem is recorded in a
docs commit.

## Adversarial Findings

### Cognitive Transform And GStack Review Pass - 2026-06-06

Skills applied:

- `cognitive-transform-portfolio`: selected transforms that changed the next
  route rather than only explaining the bug.
- `gstack-autoplan`: used the CEO/design/engineering/DX review structure for a
  whole-mission/current-state review rather than a narrow PR diff review.
- `gstack-design-review`: used the live visual-audit criteria against the
  source reader and article-side citation surfaces.
- `gstack-review`: used the scope-drift and missing-requirements checks; since
  `main` has no active feature-branch diff after the table slice landed, the
  useful output is mission/current-state review rather than "nothing to
  review."

Route-changing cognitive transforms:

- Real object: this is not a `ContentViewer` styling bug. The real object is a
  source-inspection instrument attached to a canonical VText source graph.
- Load-bearing variable: the time and disruption between clicking a citation
  and understanding the evidence. Metadata visibility matters, but only after
  the reader can understand the source.
- Deep version of "source cards": citation markers are transclusion points, not
  decorative badges. Expanded inline notes should carry enough bounded source
  substance to support the claim in place; opened windows should be full source
  readers with provenance as apparatus.
- Verification transform: source proof cannot stop at "text is visible."
  Geometry and overlap assertions must cover opened source windows the same way
  the existing Pretext tests cover article-side note wrapping.
- Simplification transform: the right simplification is shared source excerpt
  and source reader ownership. The wrong simplification is deleting metadata or
  hiding reader failures with CSS.

GStack CEO review findings:

- The legal-cloud proposal is client-facing. If source windows overlap text,
  inline source notes show skeletal excerpts, or duplicate source windows pile
  up, the artifact reads as a demo rather than a professional deliverable.
- The whole mission should keep the long proposal's content and source graph
  equivalent while improving source inspection, not chase a narrow ABA/OVH
  screenshot fix.
- The table repair is valuable but only a slice. The next acceptance proof must
  return to the owner publication and prove the citation/source behavior that a
  client will actually inspect.

GStack design review findings:

- Opened source windows have a false hierarchy: large app title, eyebrow,
  duplicate heading, and rounded disclosure cards compete with the source text.
- The user-visible failure is not merely "overlap"; it is reader-mode
  authorship confusion. The source viewer looks like a file metadata app, while
  the desired UX is closer to an academic/journal source apparatus.
- Article-side Pretext flow is directionally right because it wraps prose around
  a note, but the note content is underfilled. The available space should be
  used for a bounded multi-sentence excerpt and source context.
- Metadata should become compact apparatus: source URL/action, evidence state,
  and provenance should remain inspectable without being the first visual layer
  readers have to parse.

GStack engineering review findings:

- `ContentViewer.svelte` is currently both generic media/file viewer and source
  reader. Its generic metadata blocks are the direct owner of the source-window
  overlap failure and should be split or given a dedicated source-reader mode.
- `vtext-source-flow.ts` builds article-side notes from exactly one compact
  quote extracted from the hidden popover. There is no shared excerpt-selection
  layer that can use reader snapshot text, selector quote, available note space,
  and publication policy together.
- `vtext-source-launcher.ts` sets `allowMultiple: true`, causing duplicate
  source windows by default. That may be useful for explicit comparison, but it
  is bad default behavior for client review.
- Existing source tests already contain useful geometry assertions for Pretext
  side-note routing, but opened source-window tests only assert text presence
  and SHA visibility. The next code pass should add source-window geometry
  tests before/with the fix.
- Search hygiene: generated `frontend/dist` output is not tracked, but it is
  present locally and can drown source searches unless excluded. Keep review
  commands scoped to `frontend/src`, `frontend/tests`, and runtime packages.

GStack DX/current-state findings:

- The mission has strong docs, but there is no single command/report yet that
  tells a future agent "source window source-reader QA status." Add or reuse
  focused tests with names that encode the source-reader contract.
- Current proof relies on Comet plus API/Playwright fragments. That is
  acceptable for staging, but every acceptance note must distinguish local,
  deployed, owner-authenticated, and guest/public proof.

Changed plan:

- Implement the next source UX repair as a source-reader/source-excerpt
  architecture pass, not a narrow `details` margin patch.
- Add opened source-window geometry proof across at least ABA and OVH-like text
  sources before claiming the visual failure fixed.
- Make inline source excerpts bounded but richer by reusing source snapshot
  text where policy allows, with the opened source window remaining the full
  reader.
- Dedupe source windows by source identity by default, leaving any explicit
  comparison behavior for a later/visible affordance.
- After the source-reader fix works, run a simplification pass that removes or
  fences old popover/card paths and stale duplicated source rendering logic.

### P0 - Generic Source Viewer Text-On-Text Regression

The screenshot evidence shows source body text, disclosure summaries, and
expanded metadata rendered in the same visual region. This is a reader failure,
not merely an aesthetic problem.

Likely implicated surface:

- `frontend/src/lib/ContentViewer.svelte` lines 90-171 render reader content
  followed by three `details.provenance` blocks.
- Lines 176-185 make the app a flex column with global gap, while lines 318-324
  style every provenance disclosure as a card-like block.
- The tests that open source windows only assert visibility and text content;
  they do not assert geometry or collision-free rendering.

Acceptance implication: source-window proof must include bounding-box or
screenshot checks for no overlap with all disclosures closed and with `Source
evidence` expanded, across at least two materially different source windows.

### P0 - Article-Side Transclusion Notes Underuse Their Reading Surface

The article-side Pretext note is meant to let users inspect source support
without leaving the reading flow. The current note often shows a one-sentence
stub even when the side-note allocation has room for more useful source
content. That pushes users toward opening separate source windows, which is
more disruptive and does not match the expected reading pattern.

Evidence:

- The OVHCloud article screenshot shows a tall right-side source note with only
  title, one sentence, and actions, while the surrounding proposal text keeps
  flowing beside it.
- The user explicitly clarified that many readers will click source points but
  not open source windows.

Direction: inline transclusion notes should include a richer bounded excerpt:
enough source substance to evaluate the claim in context, while preserving the
opened source window as the place for the full reader artifact, provenance, and
source URL.

### P1 - Appendix Glossary Final Row Falls Out Of Table Formatting

The legal-cloud proposal still has a table consistency problem even though the
earlier whole-table `TermDefinition` collapse was repaired. In the appendix
glossary, the final `Work product` entry renders as prose with visible pipe
characters instead of remaining in the table.

Likely mechanism:

- `frontend/src/lib/vtext-markdown-renderer.ts` only treats a line as a table
  row when the trimmed line starts with `|` and ends with `|`.
- The screenshot shows the row beginning with `| Work product |`, but it does
  not visibly keep the final trailing delimiter. If the canonical row is
  missing the final `|`, the renderer flushes the table and renders the line as
  a paragraph.
- Existing tests cover full table preservation and collapse recovery, but do
  not appear to cover a malformed or delimiter-damaged final row after a long
  table.

Empirical renderer probe on 2026-06-06, using the real
`renderMarkdownBlocks` module through esbuild:

- A strict table renders as one `table` with all rows.
- If only the final row is missing its trailing `|`, the prior rows remain in
  the table and that final row renders as a paragraph beginning with `|`.
- If a middle row is missing its trailing `|`, the table terminates early and
  later valid-looking rows also render as paragraphs.
- Trailing spaces after a valid trailing `|` are fine because the renderer
  trims the line first.
- Rows without side pipes, even if GitHub-flavored Markdown would accept them,
  render as ordinary paragraph text.
- A row with an escaped pipe currently splits into an extra cell, so pipe
  escaping is another table-structure fidelity gap.
- A row with an empty final cell renders as prose because the table parser
  accepts it, but the fallback table validity path drops empty cells when
  deciding whether the block is a valid table.

Authenticated staging diagnosis on 2026-06-06, using Comet and
`/api/vtext/documents/f93cea62-f833-4dae-b414-8e44783d8cbe/diagnosis?limit=160`,
shows this is not only a strict-renderer problem:

- The live owner document is `choir_private_legal_cloud_proposal.vtext`, current
  version `v87`, current revision
  `4d2a9034-0cd3-4af2-b160-01c9f265eb19`.
- `v70` had 50 strict pipe-table rows and the final `Work product` row ended
  with ` |`.
- `v71` collapsed the glossary into the `TermDefinition**...` artifact.
- `v72` through `v74` restored 49 strict rows plus one table-shaped final row,
  but the final `Work product` row no longer had the closing pipe.
- `v75` through `v78` collapsed again into `TermDefinition`, preserving only
  the malformed table-shaped `Work product` line.
- `v79` through `v87` restored 49 strict rows plus that same malformed final
  line; therefore the screenshot at `v87` is a persistent canonical/projection
  corruption, not just a visual renderer decision.

Cognitive-transform route change: treat this as structure preservation across
canonical VText, editable DOM, Markdown import/export, and revision repair.
Renderer tolerance is useful as a reader safety net, but it is insufficient if
save/revise/export continue to persist delimiter-damaged table rows.

Direction: repair this as a general formatting-consistency problem. The system
should normalize table-shaped rows adjacent to known tables, preserve trailing
cell delimiters through edit/save/revise/export, and prefer structured VText
table blocks over renderer-only recovery. Do not special-case `Work product`,
Appendix A, or glossary rows.

Implementation checkpoint on 2026-06-06:

- Commit `7c5dec1c7ed52e7fc6bc352907e86b191dee36f0` changed the Markdown
  renderer and VText revision stabilization to tolerate and normalize
  table-shaped rows adjacent to existing Markdown tables, including rows that
  start with `|` but are missing the final delimiter. The implementation also
  parses escaped pipe characters as cell content rather than as extra column
  separators. It does not name the legal-cloud document, glossary, appendix,
  or `Work product`.
- Focused local regression proof passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVTextMarkdownStructureStabilizationRepairsMalformedTableTailRow|TestVTextMarkdownTableRowParserHandlesEscapedPipes|TestVTextImportedMarkdownRevisionUsesVTextProjectionAndPreservesCollapsedTable'`.
- Frontend build proof passed: `pnpm --dir frontend build`.
- Partial-Markdown renderer probe against the real
  `frontend/src/lib/vtext-markdown-renderer.ts` showed:
  final-row-missing-trailing-pipe and middle-row-missing-trailing-pipe both
  render inside a `table`; escaped pipe renders as literal cell text; pipe-led
  prose with fewer than two cells remains paragraph text.
- CI run `27052504250` passed for commit
  `7c5dec1c7ed52e7fc6bc352907e86b191dee36f0`.
- FlakeHub publish run `27052504263` passed for the same commit.
- Staging health at `https://choir.news/health` reports deployed commit
  `7c5dec1c7ed52e7fc6bc352907e86b191dee36f0`, deployed at
  `2026-06-06T04:26:37Z`.
- Authenticated Comet proof on the owner publication
  `https://choir.news/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`
  showed the deployed `Published VText` reader rendering `Work product` as the
  final row inside the Appendix A glossary table, followed by `---` and
  `End of Proposal` outside the table. This proves the deployed renderer
  safety net covers the current owner artifact. It does not by itself prove
  future focus/edit/save/revise structural preservation under all table edits;
  that remains a required next-axis acceptance item.

### P1 - Source Windows Still Lead With App Chrome Instead Of Evidence

The current source viewer opens with an eyebrow, a huge duplicate title, and a
right-floating `Open source` link before the source content. In a source
artifact reader, this makes the evidence feel like a desktop app status page
rather than a source.

Evidence:

- `ContentViewer.svelte` lines 91-99 render `FILES CONTENT`, a large `h2`, and
  header link before the reader article.
- The screenshot shows the formal opinion title three times: window title,
  page title, and reader title.

Direction: use a quieter source-reader header, keep the canonical source title,
and move URL/source actions into a compact reader toolbar or metadata drawer.

### P1 - Metadata Is Still Visually Too Primary

Source entity and provenance metadata are useful for auditors and agents, but
the opened reader should prioritize the source substance. Today metadata
appears as multiple bordered rounded rectangles immediately after the first
source paragraph and can collide with content when expanded.

Direction: treat source metadata as an inspectable appendix, drawer, or compact
footnote-like apparatus. Keep provenance available without competing with the
reader.

### P1 - Source Window Lifecycle Accumulates Duplicate Windows

Opening the same source repeatedly can create many separate source windows
because `sourceEntityLaunchPayload` sets `allowMultiple: true`. That may help
some side-by-side inspections, but the legal-cloud client review path is now
showing taskbar/source-window accumulation.

Evidence:

- `frontend/src/lib/vtext-source-launcher.ts` line 31 sets `allowMultiple:
  true`.
- `frontend/src/lib/stores/desktop.js` lines 280-335 focus an existing window
  only when multiple windows are not allowed.

Direction: source windows should dedupe by source identity by default, with an
explicit "open another copy" affordance only if comparison is useful.

### P1 - Source Acquisition Is Still Not One Clean Reader Pipeline

The Web Lens path now stores snapshots as content artifacts, which is a real
repair. But the durable target is still one path:

```text
source URL or file -> raw snapshot/hash -> cleaned Markdown reader artifact
-> ContentItem or Source Service item -> VText source entity -> publication
source snapshot -> opened source reader
```

Iframe/live web views should be optional fallback, not the canonical source
reader proof. Obscura/Web Lens cleanup should remove boilerplate, preserve
selectors, record warnings, and render Markdown reader mode when iframe fails.

### P1 - Publication Source Access Needs A Focused Policy Proof

The product direction is correct: a published VText should publish the sources
that authorized readers are allowed to inspect. The next mission must verify
the actual source-window access policy:

- owner-authenticated reader can inspect private and public permitted sources;
- guest/public reader can inspect public publication snapshots;
- omitted/private/licensed source material is clearly omitted, not leaked;
- source metadata required for verification does not render as ordinary prose.

### P2 - Source Review UX Is Better But Still Not Fully Owner-Grade

Raw repair JSON has been removed from the visible source panel, and no-source
needed is supported. The remaining source artifact controls still feel closer
to an operator/debug surface than a client-grade claim/source review flow.

Direction: continue toward typed rows: claim, marker, source candidate,
confirm/refute/omit/no-source-needed, selector, caveat, and source-reader
preview.

### P2 - Source Rendering Paths Need A Simplification Pass

Recent work moved many pieces in the right direction, but the system now has
several adjacent concepts:

- article-side Pretext journal flow;
- old source-ref popover DOM hidden by source-flow CSS;
- media preview card expansion;
- `ContentViewer` source-reader mode;
- `BrowserApp` Web Lens reader snapshot mode;
- VText source artifact attach/import panel;
- publication source snapshot materialization.

The next pass should remove or fence dead/weak paths, not add another local
patch. Keep the source graph contract stable and simplify projections around
it.

### P2 - Large Owners Still Make Regression Review Hard

Current file sizes are still high:

- `frontend/src/lib/VTextEditor.svelte`: 3,586 lines.
- `internal/runtime/vtext.go`: 5,277 lines.
- `frontend/src/lib/BrowserApp.svelte`: 1,303 lines.

These files are now carrying multiple domains. The next simplification should
extract by contract boundary and delete old compatibility paths only after
staging-proofed behavior is covered.

### P2 - Visual Test Coverage Is Too Functional

The tests prove source windows open and contain evidence, but not that readers
can actually read them. Add geometry/pixel checks for:

- closed metadata disclosures do not overlap reader text;
- expanded `Source evidence` does not overlap reader text;
- long URLs and hashes wrap without escaping their container;
- 720px and wide desktop windows remain readable;
- repeated opening focuses or reuses the existing source window.
- article-side source notes use available side-note/stacked-note space for
  more than a one-sentence stub when the source snapshot has more relevant
  text.
- long tables preserve every row through render/edit/save/revise/export,
  including a final row whose Markdown delimiter shape is damaged or
  normalized late.

### P2 - The Next Run Needs Explicit Adversarial Review Loops

This mission should not proceed as a single bug fix. Before implementation,
run cognitive transforms and gstack review/design-review to reframe the
problem, identify adjacent defects, and find simplification opportunities.
After the first working fix and proof, run the same adversarial pass again
against the diff and the visual result.

Required review posture:

- Use the Cognitive Transform Portfolio to change the next probe, verifier, or
  scope; do not use it as decorative analysis.
- Use gstack review posture for code-surface risks, dead paths, ownership
  confusion, trust-boundary mistakes, and missing tests.
- Use gstack design-review posture for visual hierarchy, text overlap,
  source-note density, interaction flow, and client-readiness.
- Convert any newly found behavior problem into a docs checkpoint before
  fixing it.

## Cognitive Transforms

Current uncertainty or obstacle:

After 11 hours of mission work, the system is close enough to encourage narrow
patches: adjust a margin, hide a disclosure, or shrink one card. That would
repair one screenshot while leaving the deeper source-reader and inline
transclusion contracts weak.

Selected transforms:

1. Audience-Level Translation - A client opening a citation does not want to
   see app scaffolding. A client clicking a citation inline also may not want
   a new window at all. They want enough evidence in the note to keep reading,
   plus a clean path to the full source when needed.
2. Depth Extraction - The deep feature is not "cards expand." The feature is
   inspectable source-backed claims with stable source identity, selectors,
   publication policy, and readable evidence surfaces.
3. Via Negativa - Delete or demote UI that creates fake confidence: duplicate
   titles, metadata chrome, stale popover layers, iframe-only source proof,
   source-window duplicates, and ad hoc source wrappers.
4. Homotopy / Real Artifact - Keep one real source graph and improve its
   projections. Do not create a separate demo source reader that bypasses VText
   metadata, publication snapshots, or source policy.

Route-changing insights:

- The first implementation target is not "make the disclosure box prettier";
  it is "make the opened source reader a reliable projection of the source
  artifact."
- The inline source note target is not "show a source card"; it is "give the
  reader enough bounded evidence to decide whether to keep reading or open the
  full source."
- Formatting consistency is not "make this one row blue." It is "preserve
  document structure until the last row, last cell, and export projection."
- The verifier must include readability geometry and source-note content
  density, not only text presence.
- Source-window dedupe is part of client-readiness because review sessions can
  involve tens or hundreds of source opens.
- Adversarial review is part of the mission control loop, not an optional
  after-action report.

Changed plan:

- implementation: fix `ContentViewer` source-reader layout and metadata
  hierarchy, enrich article-side transclusion notes from bounded source
  snapshots, repair table-row consistency through shared structure
  preservation, then decide whether to extract a dedicated `SourceReader`
  component rather than adding more modes to the generic content app.
- verifier/evidence: use Comet staging proof for the actual legal-cloud
  proposal when possible, and add Playwright geometry checks for collision-free
  source windows plus inline-note density checks and table-row preservation
  checks.
- scope: include source-window lifecycle, inline transclusion substance, and
  multiple source artifacts, plus appendix table consistency, not just one ABA
  source screenshot.
- stopping condition: the owner and guest legal-cloud publication can open
  source windows that read cleanly, expose permitted source snapshots, avoid
  duplicate-window clutter, and preserve article-side Pretext journal flow with
  more useful inline source content.

Next high-information action:

- Reproduce the source-window overlap with at least two source payloads and
  reproduce the one-sentence inline-note underuse with a source snapshot that
  contains more relevant text. Reproduce the appendix glossary final-row
  fall-out from canonical content or a faithful fixture. Add failing
  geometry/content/table assertions before changing layout code.

## Real Artifact

The real artifact is the legal-cloud proposal as a canonical VText publication
whose citations expand into transclusions and open readable, policy-correct
source artifacts. The source reader is not a separate demo. It is a projection
of the same source graph used by VText revisions, publication, export, and
future research.

## Invariants

- VText is canonical. Markdown is import/export projection after v0 -> v1.
- Citation markers are transclusion points backed by `source_entities`
  metadata.
- Tables are VText document structure, not decorative Markdown. Import/export
  may use Markdown table syntax, but render/edit/save/revise should preserve
  table rows and cells consistently.
- Hidden metadata must not render as prose.
- Source text is untrusted evidence, never prompt instructions.
- Publication exposes only permitted source snapshots and explains omissions.
- Article-side Pretext flow remains a reading aid, not a card stack.
- Whole-document rewrite remains explicit and exceptional.
- No hardcoded legal-cloud, ABA, or glossary special cases.
- No classifier/workflow scaffolding as a substitute for source graph repair.

## Value Criterion

Minimize the distance between "open a citation" and "understand the source"
while preserving canonical VText, source identity, publication policy, export
truthfulness, and code simplicity. A change is uphill only if it improves the
real legal-cloud artifact and reduces weak paths instead of adding another
projection layer.

## Homotopy Axes

1. Source reader fidelity:
   text presence -> collision-free reader -> metadata appendix -> source
   selectors/highlights -> multi-page/source-specific readers.
2. Inline transclusion usefulness:
   one-sentence stub -> bounded multi-sentence excerpt -> selector-aware
   quote/context -> richer note tuned to available Pretext space.
3. Table/format structure consistency:
   markdown-string table -> normalized table-shaped rows -> structured VText
   table blocks -> consistent render/edit/save/revise/export projection.
4. Source acquisition:
   manual artifact -> Web Lens content artifact -> cleaned Markdown pipeline
   -> policy-aware Source Service itemization.
5. Source UI density:
   card stack -> quiet journal note -> content-first source window -> academic
   reader with compact footnote/provenance affordances.
6. Lifecycle:
   unlimited duplicate windows -> source-identity reuse -> explicit comparison
   mode for duplicate opens.
7. Verification:
   text assertions -> geometry assertions -> Comet owner proof -> owner and
   guest publication proof -> export/source policy proof.

## Dense Feedback

- Playwright fixtures for at least two source readers with closed and expanded
  metadata disclosures, including a non-ABA source such as OVHCloud.
- Screenshot or bounding-box proof that source text and metadata do not
  overlap.
- Article-side Pretext proof that source notes render more than a one-sentence
  stub when the source snapshot has relevant additional text and the note has
  available space.
- Appendix glossary proof that the final `Work product` row remains in the
  table on staging, and a generic regression fixture proving final-row table
  preservation without relying on glossary-specific terms.
- Deployed `choir.news` proof on the owner legal-cloud publication using
  authenticated Comet where possible.
- Guest/public proof that source windows open publication-carried source
  snapshots.
- Regression tests for Pretext side-note and stacked source flow.
- Markdown export proof with compact source markers and omitted-private-source
  metadata.
- `pnpm --dir frontend run build`.
- Focused Playwright source tests before any CI/deploy landing loop.

## Forbidden Shortcuts

- Do not hide the failing metadata blocks without preserving provenance access.
- Do not patch only ABA Formal Opinion 512 or any other named source.
- Do not force users to open a new source window just to get useful evidence
  for a normal inline citation click.
- Do not patch `Work product`, Appendix A, or glossary rows by name.
- Do not treat renderer leniency as a substitute for preserving canonical
  table structure through VText revisions.
- Do not replace source windows with iframe-only previews.
- Do not add another source-card layer to solve a card-layer problem.
- Do not use rendered DOM as export source.
- Do not use Markdown write-through as canonical VText mutation.
- Do not claim proof from local screenshots when staging owner proof is needed.
- Do not leave stale duplicate source windows as accepted client-review UX.

## Simplification Targets

- Decide whether `ContentViewer` should extract a dedicated source-reader
  component.
- Decide whether source-note excerpt selection belongs in source rendering,
  source materialization, or a shared selector/excerpt helper. Avoid another
  one-off truncation path.
- Decide whether Markdown table normalization belongs at import, revision
  stabilization, renderer parse, export, or a shared VText structure layer.
  Prefer one structural owner over scattered table heuristics.
- Remove or fence old source-ref popover/expanded-card paths that are no
  longer used for text sources.
- Reuse one Markdown reader renderer between content source windows and Web
  Lens reader snapshots where contract-compatible.
- Centralize source-window launch identity and dedupe behavior.
- Move source artifact panel state out of `VTextEditor.svelte` when it reduces
  ownership confusion.
- Keep media preview behavior separate from text-source reader behavior.
- Consolidate source test helpers instead of adding more one-off fixtures.

## Stopping Condition

Mission is complete only when:

- the new text-on-text source viewer regression is reproduced, fixed, and
  covered by geometry or screenshot proof across multiple source windows;
- article-side transclusion notes show useful bounded source substance for
  readers who do not open source windows, without becoming full source dumps;
- the appendix glossary final row remains inside the table on staging, and the
  regression fix is generic to table-shaped document structure;
- the owner legal-cloud publication opens multiple source windows, including a
  non-ABA source, as readable source windows on staging;
- the same publication exposes permitted sources to guest/public readers;
- Pretext article-side source flow still passes deployed proof;
- source-window duplicate behavior is resolved or explicitly fenced;
- cognitive-transform and gstack review/design-review passes are recorded with
  issues found, issues fixed, and residual risks;
- simplification removes or fences weak/dead source UI paths without regressing
  source graph behavior;
- commit -> push main -> CI -> Node B deploy -> staging identity -> deployed
  owner-account proof is complete.

## Residual Risks To Track

- Some source URLs may forbid raw snapshot publication or live embedding. The
  reader snapshot policy must distinguish public, private, licensed, and
  omitted materials.
- Long legal PDFs need page/selector-aware readers, not only Markdown summaries.
- A generic content viewer may remain appropriate for files/media but still be
  the wrong owner for citation source readers.
- Visual proof can miss dynamic text scaling issues unless tested across
  constrained and wide windows.

## 2026-06-06 Export Completion-Audit Problem Checkpoint

Status: `documented_before_fix`.

The deployed owner publication at
`/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6` rendered the
appendix glossary table correctly after the table-structure repair, but the
public Markdown export still emitted the final glossary row as a malformed
partial table row:

```text
| **Vector search** | ... |

| **Work product** | Durable, reviewable output ...
```

The export proof was a direct call to
`/api/platform/publications/export?route=/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6&format=md`.
It confirmed title, source markers, and absence of source UI labels, but failed
the final-row table assertion because the exported `Work product` row lacked
the closing `|`.

Root-cause belief before code change: the browser renderer and VText editor
path were now lenient enough to render a table-shaped row, but the Markdown
export path still returned immutable publication artifact bytes verbatim. This
left old/publication-carried malformed rows visible in exported Markdown even
when the reader UI appeared fixed. The fix must therefore repair the Markdown
export projection generally while leaving canonical publication records
immutable.

## 2026-06-06 Export Fix Evidence

Status: `local_fix_ready_for_landing`.

Partial Markdown renderer probe before fixing:

```text
strict row: hasTable=true, hasWorkProductCell=true
missingTailPipe row: hasTable=true, hasWorkProductCell=true
missingTailPipeThenRule: hasTable=true, hasWorkProductCell=true
```

This confirmed that UI rendering can pass while exported Markdown remains
structurally invalid for downstream Markdown consumers.

Code path repaired:

- Added `internal/markdownstructure.NormalizeTableShapedRows` as the shared Go
  structure normalizer for rows that remain inside a real Markdown table but
  lost the final delimiter.
- Replaced the runtime-local normalization copy in VText revision
  stabilization with the shared helper.
- Applied the same helper to `md` publication exports only, so legacy
  malformed publication bytes export as valid Markdown without mutating the
  immutable stored publication artifact.

Regression coverage:

- `TestNormalizeTableShapedRowsRepairsFinalRowMissingDelimiter`
- `TestNormalizeTableShapedRowsIgnoresOrdinaryPipedParagraph`
- `TestTableRowCellsHandlesEscapedPipes`
- `TestPublicationMarkdownExportNormalizesMalformedTableTailRows`

Local verification:

```text
nix develop -c go test ./internal/markdownstructure
nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextUserRevisionRepairsMalformedFinalMarkdownTableRow|TestVTextMarkdownTableRowParserHandlesEscapedPipes'
nix develop -c go test ./internal/platform -run 'TestPublicationMarkdownExportNormalizesMalformedTableTailRows|TestPublicationExportDocxAndPDFUseCanonicalPublicationBytes'
nix develop -c go test ./internal/markdownstructure ./internal/platform
pnpm --dir frontend exec playwright test tests/vtext-markdown-lineage.spec.js -g "Imported Markdown advances from v0 source artifact to canonical .vtext with Markdown export"
```

Residual risk before staging: this proves the export projection and import
lineage locally, but the actual owner publication export still needs deployed
Node B proof after commit, push, CI, and staging deploy.

## 2026-06-06 Node B Deploy Packaging Problem Checkpoint

Status: `documented_before_fix`.

Commit `01137cede6576579956ff1cbd431e77acb034639` passed local focused tests,
GitHub CI Go tests, runtime shards, and Go build, but the CI `Deploy to Staging
(Node B)` job failed before staging could adopt the commit.

Evidence:

```text
CI run: 27054007982
Deploy job: 79854748232
Failure phase: Nix build for Node B guest/sandbox image
Primary error:
internal/runtime/vtext.go:48:2: cannot find module providing package
github.com/yusefmosiah/go-choir/internal/markdownstructure:
import lookup disabled by -mod=vendor
Secondary existing-package symptom:
internal/runtime/browser.go:26:2: cannot find module providing package
golang.org/x/net/html: import lookup disabled by -mod=vendor
```

Root-cause belief before code/packaging change: the repository CI build sees the
new local package, but the Node B Nix derivation builds from a filtered source
tree or vendored module surface that does not include the new
`internal/markdownstructure` directory. This is a packaging/source inclusion
problem, not a runtime behavior failure in the export code. The repair should
make the deploy source closure include the shared structure package generally,
or place the shared logic in an already-included package, without reverting to a
glossary-specific export patch.

Stopping impact: mission cannot be complete until a follow-up commit repairs the
Node B build, deploys, confirms staging identity for the fixed SHA, and reruns
the actual owner-publication Markdown export proof.

## 2026-06-06 Node B Deploy Packaging Fix Evidence

Status: `local_packaging_fix_ready_for_ci`.

Repair made after the documentation checkpoint:

- Added `internal/markdownstructure` to the flake `internalDirs` source
  closures for `proxy`, `gateway`, `platformd`, `sourcecycled`, and `sandbox`,
  matching the service graphs that compile `internal/platform` or
  `internal/runtime`.
- Added a deploy-impact classifier rule for `internal/markdownstructure/*` so
  future changes select gateway, platformd, proxy, sandbox, and the appropriate
  host/guest deployment work instead of silently skipping the shared package.

Local verification:

```text
printf '%s\n' internal/markdownstructure/tables.go internal/platform/service.go internal/runtime/vtext.go .github/scripts/deploy-impact-classify flake.nix | .github/scripts/deploy-impact-classify /tmp/choir-impact.out
# deploy_needed=true, host_services=gateway,platformd,proxy,sandbox,
# ordinary/playwright guest image refresh selected through flake.nix

nix develop -c go test ./internal/markdownstructure ./internal/platform
```

Local limitation:

```text
nix build .#packages.x86_64-linux.sandbox .#packages.x86_64-linux.platformd .#packages.x86_64-linux.proxy --no-link
```

could not run on the local `aarch64-darwin` machine because no
`x86_64-linux` builder was available. The packaging fix therefore requires CI
and Node B deploy proof before acceptance.

## 2026-06-06 Runtime Nix Vendor Closure Problem Checkpoint

Status: `documented_before_fix`.

Commit `44de82dcebb05cd1c86c2cc8ddd1d8bf73e7788f` repaired the missing local
`internal/markdownstructure` source closure problem. CI Go tests, runtime
shards, frontend build, and Go build passed, and Node B then progressed to the
host NixOS closure build. That build failed in the sandbox package with a
different missing import:

```text
CI run: 27054120147
Deploy job: 79855083318
Failure phase: Host NixOS closure build
Primary error:
internal/runtime/browser.go:26:2: cannot find module providing package
golang.org/x/net/html: import lookup disabled by -mod=vendor
```

Root-cause belief before fix: `golang.org/x/net v0.52.0` is present in
`go.mod` and `go.sum`, and ordinary CI Go builds can compile it. The failing
path is specific to Nix `buildGoModule` package closures. The runtime browser
Markdown-alternate work introduced a real runtime dependency on
`golang.org/x/net/html`, but at least the sandbox Nix `vendorHash` still points
to a vendored dependency closure that predates that dependency. Because Nix
uses `-mod=vendor`, the build cannot fall back to `go.mod` resolution.

Stopping impact: mission cannot be complete until the runtime-bearing Nix
service package vendor closures are updated, Node B deploy succeeds, staging
reports the accepted SHA, and the owner-publication Markdown export proof passes
against the deployed service.

## 2026-06-06 Runtime Nix Vendor Closure Fix Evidence

Status: `node_b_temp_build_passed`.

Repair made after the documentation checkpoint:

- Updated the Nix `vendorHash` for `gateway` and `sandbox` from the stale
  runtime dependency closure to
  `sha256-2uExDYKXWdF4NyIMX6NVVXcuXRoTm+/S/CxuwPExXiI=`, matching the
  sourcecycled closure that already included the runtime browser dependency
  graph.

Node B temporary-clone verification:

```text
git checkout 44de82dcebb05cd1c86c2cc8ddd1d8bf73e7788f
replace gateway/sandbox stale vendor hashes with sha256-2uExDYKXWdF4NyIMX6NVVXcuXRoTm+/S/CxuwPExXiI=
nix build .#packages.x86_64-linux.sandbox .#packages.x86_64-linux.gateway --no-link
```

Result: both temporary package builds completed successfully on Node B. The
remaining proof is the normal push-triggered CI deploy and staging acceptance
checks.

## 2026-06-06 Deployed Markdown Export Table-Tail Proof Failure

Status: `documented_before_fix`.

Commit `18c5bf505b1e16efd779fc46e57d4dffc9720304` passed CI and deployed to
Node B. Staging health reported both proxy and sandbox at that SHA:

```text
CI run: 27054280586
Deploy job: 79855568648
Health deployed_commit: 18c5bf505b1e16efd779fc46e57d4dffc9720304
Health deployed_at: 2026-06-06T05:54:39Z
```

The owner publication Markdown export proof still failed against staging:

```text
GET https://choir.news/api/platform/publications/export?route=/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6&format=md
format md
filename choir-private-legal-cloud-proposal-vtext-pub270a62fb6.md
content_hash_present True
proposal title retained: True
work product row present: True
work product row has trailing delimiter: False
no open source ui label: True
no close ui label: True
source evidence retained: True
```

Observed tail:

```markdown
| **Vector database** | A database optimized for storing and searching numerical vectors (embeddings), enabling similarity search. |
| **Vector search** | A search technique that finds items similar to a query by comparing their vector representations in a high-dimensional space. |

| **Work product** | Durable, reviewable output of professional work—drafts, memos, briefs, letters, tables, cited research—as opposed to ephemeral chat responses.

---
```

Root-cause belief before code change: the shared Markdown table normalizer fixed
malformed table-shaped rows only while it was already inside a contiguous table
block. The real owner document contains a blank line before the malformed final
row. The frontend renderer's table recovery is permissive enough that this row
appears visually attached to the glossary table, but the export normalizer
treated the blank line as the end of table context and therefore did not repair
the final delimiter. The next repair should generalize table-structure recovery
for table-shaped continuation rows near a preceding confirmed table, while still
avoiding ordinary pipe-containing prose.

## 2026-06-06 Deployed Markdown Export Table-Tail Fix Evidence

Status: `accepted_on_staging`.

Repair made after the documentation checkpoint:

- Updated the shared Markdown table normalizer to preserve table context across
  a small blank gap only when the following row is table-shaped and has the same
  column count as the preceding confirmed table.
- Removed the blank gap while repairing the missing trailing delimiter, so the
  exported Markdown is a valid contiguous table block rather than merely a row
  that happens to start and end with pipes.
- Added regression coverage for the actual blank-separated table-tail shape and
  a mismatched-width guardrail that leaves non-continuation pipe rows unchanged.

Local verification:

```text
nix develop -c go test ./internal/markdownstructure
nix develop -c go test ./internal/platform -run 'TestPublicationMarkdownExportNormalizesMalformedTableTailRows'
nix develop -c go test ./internal/markdownstructure ./internal/platform -run 'TestPublicationMarkdownExportNormalizesMalformedTableTailRows|TestNormalizeTableShapedRows'
```

Landing evidence:

```text
Fix commit: 532798571532074befdd2984f5ff5dc127a0578f
CI run: 27054515347
Node B deploy job: 79856294400
FlakeHub publish run: 27054515344
Health proxy deployed_commit: 532798571532074befdd2984f5ff5dc127a0578f
Health sandbox deployed_commit: 532798571532074befdd2984f5ff5dc127a0578f
Health deployed_at: 2026-06-06T06:06:03Z
```

Deployed owner-publication export proof:

```text
GET https://choir.news/api/platform/publications/export?route=/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6&format=md
format md
filename choir-private-legal-cloud-proposal-vtext-pub270a62fb6.md
content_hash_present True
length 38399
format is md: True
filename is markdown: True
content hash present: True
proposal title retained: True
work product row present: True
work product row has trailing delimiter: True
work product row rejoined to glossary: True
no open source ui label: True
no close ui label: True
source evidence retained: True
```

Accepted tail:

```markdown
| **Vector database** | A database optimized for storing and searching numerical vectors (embeddings), enabling similarity search. |
| **Vector search** | A search technique that finds items similar to a query by comparing their vector representations in a high-dimensional space. |
| **Work product** | Durable, reviewable output of professional work—drafts, memos, briefs, letters, tables, cited research—as opposed to ephemeral chat responses. |

---

*End of Proposal*
```

Residual risk: the repair remains a Markdown-table structural recovery heuristic,
not a full Markdown parser. It is intentionally constrained by confirmed table
context, a short blank gap, and matching column count. A later VText-native table
node would be a stronger canonical representation and would reduce dependence on
Markdown renderer recovery behavior.

## 2026-06-06 Source URL Routing Problem Checkpoint

Status: `documented_before_fix`.

New routing problem found during post-mission review: `sourceEntityOpenAppID`
currently routes a source entity with `display.open_surface: "source"` to the
Browser/Web Lens app whenever the entity also has a URL. That means ordinary
citation source opens can land in Web Lens solely because a URL exists, even
when the source entity carries a cleaned reader snapshot or content-item backing
that Source Viewer can render more reliably.

Observed code path:

```text
frontend/src/lib/vtext-source-renderer.ts
if (requested === 'source' && sourceEntityTargetURL(entity)) return 'browser';
```

This is backwards for the current UX contract. A source marker should default to
the source reader surface. Web Lens is appropriate when the source entity or
caller explicitly requests browser/Web Lens, or when a later capability-aware
flow has proven that a live iframe/full page can load and the user chooses that
surface. It should not be selected merely because an iframe might exist, and it
should definitely not be the fallback when iframe loading is unavailable or
blocked.

Root-cause belief before code change: the earlier rule conflated "owning source
surface" with "URL browser surface." That made URL presence override the
semantic `open_surface: "source"` display policy. The repair should preserve
explicit `open_surface: "browser"` behavior while mapping generic
`open_surface: "source"` to Source Viewer regardless of URL presence.

## 2026-06-06 Source Viewer Snapshot-Clobber Problem Checkpoint

Status: `documented_before_fix`.

The first source-routing regression test confirmed that the new route opens the
Source Viewer, but exposed a second issue: a URL-only source entity with a
reader snapshot still caused `ContentViewer` to attempt `/api/content/import-url`.
When the URL import returned `404 Not Found`, the Source Viewer showed the import
error instead of the already-available reader snapshot:

```text
Source URL routing fixture
available
URL import failed: direct_http returned status 404 Not Found
```

This violates the intended source UX. A source reader snapshot is durable source
evidence and should be the default Source Viewer content. A failed live URL,
iframe, or Web Lens attempt must not hide a usable source snapshot. The original
URL can remain available as an "Open original" action, but Source Viewer should
not block on live web import when the source entity already carries reader text.

Root-cause belief before code change: `ContentViewer.loadContentItem` only skips
URL import for published source readers with snapshots. Owner/private source
windows with `sourceEntity.reader_snapshot.text_content` still attempt live URL
import, and the error branch wins over the snapshot renderer. The repair should
skip import whenever a source entity provides reader snapshot text, not only in
published reader mode.
