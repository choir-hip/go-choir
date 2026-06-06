# MissionGradient v0: Source Viewer Reader-Mode Hardening And Source UX Simplification

Status: checkpoint_incomplete
Date: 2026-06-06

Requirements contracts:
[source-external-data-publication.md](source-external-data-publication.md),
[vtext-version-compare-merge-debuggability-spec.md](vtext-version-compare-merge-debuggability-spec.md),
[vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md](vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md)

Supersedes for the next run:
[mission-vtext-client-ready-source-transclusion-pretext-v0.md](mission-vtext-client-ready-source-transclusion-pretext-v0.md)

Related current-state review:
[vtext-mission-hard-review-2026-06-05.md](vtext-mission-hard-review-2026-06-05.md)

## Run Checkpoint & Resumption State

status: checkpoint_incomplete

last checkpoint:

- Behavior-changing commit `eef70b6a900d7994d22292192730444086898ada`
  persists Web Lens snapshots as `ContentItem` artifacts instead of opening a
  prose-only VText wrapper with a fake source content ID.
- Staging `/health` reports proxy and sandbox deployed at `eef70b6a`, deployed
  at `2026-06-06T03:48:56Z`.
- This document is a documentation-first checkpoint for newly observed source
  viewer failures. Do not write source-viewer code before this problem record.

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
  space without becoming a full source window.

suggested resume goal string:

```text
/goal Run docs/mission-vtext-source-viewer-reader-mode-hardening-v0.md as a Codex-operated MissionGradient mission. Start from deployed commit eef70b6a. Preserve canonical VText, source entities, citation transclusions, source publication policy, Markdown export, and staging proof. First reproduce and fix the generic source viewer text-on-text regression across multiple source windows with geometry/visual proof. Then make article-side transclusion notes show more useful source substance for readers who inspect citations inline without opening separate source windows. Run cognitive transforms and gstack review/design-review for adversarial perspective before and after the first working fix, then simplify source viewer/source flow code paths and remove weak/dead abstractions while keeping the legal-cloud proposal source graph and opened source windows working for owner and guest readers.
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
- The verifier must include readability geometry and source-note content
  density, not only text presence.
- Source-window dedupe is part of client-readiness because review sessions can
  involve tens or hundreds of source opens.
- Adversarial review is part of the mission control loop, not an optional
  after-action report.

Changed plan:

- implementation: fix `ContentViewer` source-reader layout and metadata
  hierarchy, enrich article-side transclusion notes from bounded source
  snapshots, then decide whether to extract a dedicated `SourceReader`
  component rather than adding more modes to the generic content app.
- verifier/evidence: use Comet staging proof for the actual legal-cloud
  proposal when possible, and add Playwright geometry checks for collision-free
  source windows plus inline-note density checks.
- scope: include source-window lifecycle, inline transclusion substance, and
  multiple source artifacts, not just one ABA source screenshot.
- stopping condition: the owner and guest legal-cloud publication can open
  source windows that read cleanly, expose permitted source snapshots, avoid
  duplicate-window clutter, and preserve article-side Pretext journal flow with
  more useful inline source content.

Next high-information action:

- Reproduce the source-window overlap with at least two source payloads and
  reproduce the one-sentence inline-note underuse with a source snapshot that
  contains more relevant text. Add failing geometry/content assertions before
  changing layout code.

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
3. Source acquisition:
   manual artifact -> Web Lens content artifact -> cleaned Markdown pipeline
   -> policy-aware Source Service itemization.
4. Source UI density:
   card stack -> quiet journal note -> content-first source window -> academic
   reader with compact footnote/provenance affordances.
5. Lifecycle:
   unlimited duplicate windows -> source-identity reuse -> explicit comparison
   mode for duplicate opens.
6. Verification:
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
