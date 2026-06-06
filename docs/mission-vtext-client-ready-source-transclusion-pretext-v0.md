# MissionGradient v0: Client-Ready VText Source Transclusion And Proposal Cleanup

Status: checkpoint_incomplete
Date: 2026-06-05

Requirements contracts:
[source-external-data-publication.md](source-external-data-publication.md),
[vtext-version-compare-merge-debuggability-spec.md](vtext-version-compare-merge-debuggability-spec.md),
[vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md](vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md)

Related mission:
[mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md](mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md)

## Run Checkpoint & Resumption State

status: checkpoint_incomplete

last checkpoint:

- Docs checkpoint `bf0edd7a` records deployed proof for simplification commit
  `85ae3990d4736388111e297c38f00288aca35617`.

current artifact state:

- The legal-cloud proposal now exists on the deployed public `.vtext` route as
  the full client proposal, not the short source-demo draft.
- Imported Markdown/source files are treated as migration/source artifacts; the
  canonical editable object is VText, and Markdown is an export projection.
- Seven public source entities and seven citation transclusions resolve on the
  published route, and opened sources render as reader-mode Markdown artifacts.

what shipped:

- Source markers expand in article flow through the Pretext-routed journal note
  path.
- Source windows prefer cleaned reader Markdown with provenance and diagnostics
  demoted behind disclosure.
- Legacy noncanonical VText editor file write-through was removed.

what was proven:

- CI run `27044299886` and FlakeHub run `27044299892` succeeded for
  `85ae3990d4736388111e297c38f00288aca35617`.
- Node B `/health` reported proxy and sandbox deployed at that SHA on
  `2026-06-05T22:54:47Z`.
- Comet staging proof opened the deployed legal-cloud route, expanded an inline
  source marker, and kept the opened source window in reader-mode source
  presentation.
- Public Markdown export remained 38,398 bytes with compact `source:` markers,
  no `missing source` prose, and `private_material_omitted: true`.

unproven or partial claims:

- The final magazine/academic-journal visual design is not complete.
- The source acquisition and cleanup path is still too manual; arbitrary iframe
  Web Lens fallback remains weaker than cleaned Markdown reader artifacts.
- Raw source repair JSON remains too visible in the owner/editor surface.

belief-state changes:

- Pretext must be treated as the layout/wrapping mechanism for article-side
  evidence notes: columns or routed line ranges of proposal prose should flow
  alongside a minimal source note. It is not just a way to style chips, cards,
  or pill-shaped citation widgets.
- Opened source windows are separate source artifact readers. They should show
  content first, with source entity metadata available but visually secondary.

remaining error field:

- Reduce card/pill/rounded-rectangle layering in the inline source note.
- Replace operator-grade source repair controls with typed claim/source review.
- Improve Obscura/web-source cleanup into Markdown reader artifacts and keep
  iframe/Web Lens as fallback.
- Continue simplification in `VTextEditor.svelte`, source artifact state, and
  `internal/runtime/vtext.go` without changing the source graph contract.

highest-impact remaining uncertainty:

- Whether the next UI pass can make source expansion feel like a journal
  footnote/marginal note that preserves reading flow while still opening the
  full source artifact and preserving all publication policy boundaries.

next executable probe:

- Make a documented design/engineering pass over the source-note surface:
  inspect current Pretext flow code, identify removable chrome and dead helper
  paths, then implement the smallest generic source-note component split that
  improves magazine/journal wrapping without changing citation/source data
  semantics.

suggested resume goal string:

```text
/goal Continue docs/mission-vtext-client-ready-source-transclusion-pretext-v0.md from checkpoint bf0edd7a. Treat Pretext as the magazine/journal line-flow mechanism for article prose around minimal source notes, not as card styling. Before code, document any newly found source-UI/source-acquisition problem. Then simplify the source-note/editor code while preserving canonical VText, source transclusions, reader-mode source windows, Markdown export, CI, Node B deploy, Comet staging proof, and publication source policy.
```

evidence artifact refs:

- [vtext-mission-hard-review-2026-06-05.md](vtext-mission-hard-review-2026-06-05.md)
- `/Users/wiz/Library/Mobile Documents/com~apple~CloudDocs/VText Mission Hard Review - 2026-06-05.pdf`

rollback refs:

- Last deployed behavior-changing commit:
  `85ae3990d4736388111e297c38f00288aca35617`.
- Last docs checkpoint: `bf0edd7a`.

## Goal String

```text
/goal Run docs/mission-vtext-client-ready-source-transclusion-pretext-v0.md as a Codex-operated MissionGradient mission. Build the real client-ready legal-cloud proposal artifact, not a short source-demo draft. Treat choir_private_legal_cloud_proposal.md, doc f93cea62-f833-4dae-b414-8e44783d8cbe, as a legacy Markdown import/migration source whose next VText write must produce a canonical .vtext working document with preserved version lineage, table/list structure, source graph, citation/transclusion points, and Markdown export as a projection. Preserve the contracts in docs/source-external-data-publication.md, docs/vtext-version-compare-merge-debuggability-spec.md, and docs/vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md. Use authenticated staging UI QA through Computer Use on Comet as the primary proof path, with browser/API backup only when explicitly recorded as a limitation. Research, confirm, refute, or omit citations by claim; do not render "missing source" placeholder prose. Publish source metadata and source snapshots/transclusions with the VText so every user authorized to access the published form can inspect the sources permitted by publication policy. Use Pretext where it actually fits: rich-inline helpers for source chips/atomic inline fragments and layoutNextLineRange/manual line routing for expanded cards that should let prose wrap around them. Do not implement document-specific glossary/legal-cloud hardcodes, classifier/workflow scaffolding, prose source tables, rendered-DOM export, noncanonical Markdown write-through, hidden metadata prose, or whole-document rewrite for ordinary edits. After the artifact works on staging, perform a hard review of the whole mission and current system state, write the Markdown report in docs, render a PDF copy into the owner's iCloud Drive, then do a simplification pass that removes old/dead/weak/shortcut-style paths while preserving the staging-proven behavior. Land by commit, push main, CI, Node B deploy, staging identity, and deployed owner-account proof.
```

## Thesis

The target artifact is a professional client proposal that demonstrates Choir by
being useful in its own right. Source-backed VText is not a decorative citation
demo. The proposal should read like the long original legal-cloud proposal,
while giving the client lightweight ways to inspect evidence, open sources, and
understand provenance when needed.

The current failure mode is broader than source-card placement. A Markdown file
is acting like VText, a short fallback proposal replaced a longer client-ready
document, source evidence can be represented as prose tables instead of
canonical source entities, placeholder syntax can render as article chrome, and
expanded cards currently interrupt reading instead of integrating with it. The
mission should repair that document graph and delete weak paths as they become
unnecessary.

## Current State And Belief

- The original owner document is `choir_private_legal_cloud_proposal.md`, doc
  `f93cea62-f833-4dae-b414-8e44783d8cbe`.
- The file is Markdown by name/source, but the product has been treating it as
  a VText-like working artifact. Do not assume `.md` behavior is identical to
  `.vtext`; prove the differences and migrate the canonical working object.
- Earlier mission proof identified a real appendix table regression between
  owner versions v74 and v75, and table preservation has partial owner and
  generic proof. The new mission must keep that invariant while changing source
  behavior.
- A source-backed fallback VText was published and proved that content-item
  source entities can survive publication and open source windows. It is not
  accepted as the client proposal because it is a sibling fallback, not the
  full original proposal migrated and repaired in place.
- Owner screenshots show four new problems already documented in the related
  mission doc before code changes: top-bunched sources, visible `missing
  source`, source-card layout waste, and content-fidelity loss.
- Computer Use is available in this Codex session. Tool discovery exposed
  Computer Use click/state/type tools, and `list_apps` confirmed Comet is
  running as `ai.perplexity.comet`. Authenticated Comet staging QA is therefore
  the primary acceptance route, not a fallback.

## Cognitive Transforms

Current uncertainty or obstacle:

The system is close enough to tempt narrow fixes: hide the top rail, suppress
one missing-source badge, and make a source card smaller. That would not create
the desired artifact. The real uncertainty is whether VText currently preserves
document identity, source identity, publication access policy, and client-ready
reading quality across import, edit, save, revise, publish, export, and cleanup.

Selected transforms:

1. Audience-Level Translation - The client is not evaluating citation
   infrastructure. The client is reading a legal-cloud proposal and should feel
   that the evidence is available, not that the evidence system has taken over
   the document.
2. Depth Extraction - The feature is not "show source cards." The deeper
   feature is "claims are backed by inspectable source artifacts with stable
   selectors and publication policy." Cards are one projection of that graph.
3. Via Negativa - Remove paths that create fake confidence: prose source
   tables, placeholder badges, top source decks, Markdown write-through after
   canonicalization, rendered-DOM export, repair JSON as primary UX, and
   document-specific table/source hardcodes.
4. Homotopy / Real Artifact - Start with the full owner proposal and lower
   resolution only by hiding optional affordances, not by replacing it with a
   short demo. The final object must continuously deform from the original
   Markdown content into canonical VText with sources and exports.
5. Evidence-First Debugging - Acceptance is staged owner-account behavior:
   full document, real sources, real publication, public/authorized source
   visibility, and edit/revise metadata. Unit tests are guardrails, not the
   success claim.

Route-changing insights:

- Source placement is a reading-design problem over canonical metadata, not a
  renderer-only list-placement problem.
- `missing source` is a source-gap workflow failure when it appears in prose.
  The right choices are attach researched evidence, record a source gap in a
  repair surface, or omit a citation marker when no source is needed.
- The legal-cloud proposal should be regenerated/migrated as a real `.vtext`
  successor with equivalent content, not maintained as a Markdown file that
  happens to pass through VText code.
- Pretext should not be used as a magic styling library. Use
  `@chenglou/pretext/rich-inline` for inline chips/markers and the core
  `layoutNextLineRange` flow for expanded cards that change available line
  width.
- Cleanup is part of correctness. Once canonical VText import/export works,
  legacy write-through and renderer repair shortcuts become risks.

Changed plan:

- Implementation: create/migrate the full owner legal-cloud proposal as
  canonical `.vtext`; attach source entities from real research/content items;
  render source affordances inline and contextual by display policy; route
  expanded cards through compact/flow-aware layout; preserve Markdown export.
- Verifier/evidence: use Comet Computer Use on staging for owner-account proof;
  use Playwright/API probes only for repeatable public-route and export checks;
  record screenshots, route paths, doc/revision/publication IDs, source entity
  counts, source-window evidence, and prompt/edit metadata.
- Scope: include cleanup review and simplification after behavior works; do
  not add a one-off legal-cloud renderer patch.
- Stopping condition: deployed owner-account proof that the full canonical
  `.vtext` proposal reads correctly, publishes with source access, exports to
  Markdown, survives bounded edits, and no weak legacy path remains in the
  active route without an explicit residual-risk note.

Next high-information action:

Retrieve the owner document's current head, original long Markdown content,
version lineage, source metadata, publication state, and export behavior from
staging through authenticated product paths. Compare that to a newly created
canonical `.vtext` successor before changing renderer code.

## Pretext Research

Primary source: [chenglou/pretext](https://github.com/chenglou/pretext).
Community survey source:
[bluedusk/awesome-pretext](https://github.com/bluedusk/awesome-pretext).
Local installed package: `frontend/node_modules/@chenglou/pretext`, version
`0.0.7`.

Research findings:

- Pretext's core value is DOM-free text measurement and layout. The README
  describes `prepare()`/`layout()` for height measurement and
  `prepareWithSegments()` with line-range APIs for manual layout.
- `layoutNextLineRange()` is the relevant primitive for text that should route
  one line at a time around a changing obstacle. That is the source-card
  wrapping model: each line band can have less width while it overlaps an
  expanded source card and full width after the card ends.
- `@chenglou/pretext/rich-inline` is intentionally narrower. It supports raw
  inline items, caller-owned `extraWidth`, and `break: 'never'` for chips or
  mentions. It is not a nested markup or general CSS inline formatting engine.
- Choir already uses `@chenglou/pretext/rich-inline` in
  `frontend/src/lib/PretextInlineDisclosure.svelte` for inline fragments,
  cached prepared layouts, `ResizeObserver`, and explicit materialized line
  fragments.
- The Pretext demos include dynamic/editorial layouts that route continuous
  text around obstacles using prepared text, cursors, line bands, and explicit
  line positioning. This is closer to expanded source-card wrapping than the
  existing inline disclosure component.
- `awesome-pretext` is useful as a community gallery showing real-time
  editorial layout, draggable reflow, masonry, chat bubble, and text-flow demos,
  but it is not a product contract. Use it for design inspiration, not as a
  normative API source.

Implication for this mission:

- Near-term source-reader repair may use conventional CSS for compact inline
  cards if it preserves behavior and proves quickly.
- Durable source-card wrap should introduce a focused Pretext component that
  owns only the source-card/article-flow problem. Do not bury manual line
  routing inside `VTextEditor.svelte`.
- The Pretext component must degrade safely: if fonts or `Intl.Segmenter` are
  unavailable, render a readable block/card layout rather than losing source
  text.

## Invariants

- VText is canonical for document revisions. Imported `.md`, `.txt`, DOCX,
  PDF, and future document formats become VText projections when the user
  advances from v0 to v1.
- The original imported file remains a `ContentItem` or migration/source
  artifact with hashes and import evidence.
- Markdown is an export projection after canonicalization, not the mutable
  working substrate.
- Only VText writes canonical `.vtext` revisions.
- Hidden metadata, source payloads, hashes, prompts, and repair instructions
  must not render as article prose.
- Every visible citation marker is a transclusion point with a resolvable
  source entity or a repairable source gap. Do not render fake citation badges.
- A claim can have a confirming source, a refuting/qualifying source, or no
  source when no source is needed. "Missing source" is not article copy.
- Publication stores source metadata, transclusions, access policy, export
  policy, manifests, and source snapshots/refs so users authorized to access
  the publication can inspect permitted sources.
- Whole-document rewrite is explicit and exceptional. Ordinary edits preserve
  focused user edit diffs and `apply_edits` metadata.
- Table/list/source structure survives render, focus, edit, save, revise,
  compare/merge where applicable, publish, and export.
- No classifiers/workflow scaffolding or hardcoded document-specific fixes.

## Homotopy Axes

1. Artifact identity:
   legacy Markdown title/source -> migration manifest -> canonical `.vtext`
   successor -> Markdown export projection.
2. Content fidelity:
   short fallback draft -> structurally comparable full original proposal ->
   full proposal with researched sources -> client-ready publication.
3. Source semantics:
   prose source table -> source gaps/candidates -> canonical `source_entities`
   with selectors -> publication transclusions/source snapshots.
4. Source placement:
   top source rail -> inline collapsed citation markers -> contextual source
   panel/drawer -> compact expanded cards -> Pretext-routed article flow.
5. Source quality:
   literal placeholder -> no marker -> researched confirming/refuting evidence
   -> bounded excerpt selector -> openable source surface.
6. Proof:
   local fixture -> staging API probe -> authenticated Comet owner UI ->
   public/authorized publication route -> export artifact inspection.
7. Code quality:
   mission scaffolding -> extracted pure helpers -> deleted dead paths ->
   small tested components/modules with owner-facing behavior preserved.

## Forbidden Shortcuts

- No legal-cloud-specific table/source renderer branch.
- No glossary-specific repair beyond generic table/list structure preservation.
- No top-of-document source deck as the default article reading model.
- No visible `missing source` prose for placeholder syntax.
- No source table in prose as a substitute for `source_entities`.
- No raw `Repair JSON` as the owner-grade source workflow.
- No continuing to mutate Markdown as canonical after the first VText revision.
- No export by scraping rendered DOM.
- No publishing private source text without publication/access policy.
- No hiding source failures by dropping markers without recording repairable
  gaps when a real claim needs evidence.
- No routine whole-document rewrite to fix localized source/card/table issues.

## Work Surfaces To Review Or Replace

These are investigation targets, not pre-approved deletion instructions:

- `frontend/src/lib/VTextEditor.svelte` missing-source rendering around
  `[label](source:ENTITY_ID)` placeholders.
- `frontend/src/lib/VTextEditor.svelte` source entity inline rail and
  publication rendering path that can prepend source cards before the article.
- `frontend/src/lib/VTextEditor.svelte` source repair panel and raw repair JSON
  workflow.
- `frontend/src/lib/VTextEditor.svelte` `writeThroughToFile` and callers, under
  the invariant that imported Markdown becomes canonical VText and Markdown is
  export-only.
- `internal/runtime/vtext.go` source syntax prompt/repair paths that may
  encourage placeholder `source:ENTITY_ID` text to enter the document.
- `internal/runtime/vtext.go` structural stabilization paths for Markdown
  tables, preserving them as regression guards while moving toward first-class
  VText block preservation.
- Playwright VText tests with repeated setup/fetch helpers, after behavior is
  accepted.

## Receding-Horizon Execution

### Horizon 1 - Product State And Migration Proof

- Use authenticated staging paths to retrieve the owner document head, versions
  v70-v78 evidence, current `.md` identity, current source metadata, and export
  behavior.
- Prove whether `.md` currently behaves identically to `.vtext` for edit,
  save, revise, source metadata, publish, and export. Record differences.
- Create or identify the canonical `.vtext` successor for the legal-cloud
  proposal with migration evidence from the original Markdown content.
- Preserve the long original proposal content and appendix table structure.

Exit evidence:

- doc IDs, revision IDs, current title/extension identity, content length/hash
  comparison, table structure evidence, and Markdown export result.

### Horizon 2 - Source Research And Canonical Source Graph

- Inventory claims in the full proposal that need sources.
- Research confirming, refuting, or qualifying evidence. If no source is
  needed, remove/avoid a marker.
- Import citable URLs/content into durable `ContentItem`s or Source Service
  items.
- Attach bounded selectors/excerpts as `source_entities`.
- Remove prose source tables as source authority once metadata exists.

Exit evidence:

- nonzero canonical `source_entities`, selector/excerpt evidence, no fake
  placeholder markers, source windows open from private editor.

### Horizon 3 - Reader And Publication UX

- Remove top-bunched source cards from the default article flow.
- Render compact inline citation markers from display policy.
- Expand source cards contextually without wasting full-column space.
- Use Pretext where needed for line routing around expanded cards; keep a
  readable fallback.
- Publish with source metadata/snapshots/transclusions and access policy.
- Verify authorized/public readers can inspect permitted sources and open source
  windows.

Exit evidence:

- Comet screenshots/video or screenshot refs showing private and published
  reading surfaces, source expansion, source opening, no top source bunching,
  no visible placeholder source badge, and source access on publication.

### Horizon 4 - Edit/Revise/Export Preservation

- Prove the table survives focus, edit, save, and revise when untouched.
- Prove a bounded table edit survives without `TermDefinition` collapse.
- Prove ordinary revisions keep focused user edit diff prompt sizes and
  `apply_edits` metadata.
- Export Markdown from canonical VText and compare it to the expected proposal
  projection.

Exit evidence:

- revision IDs, prompt-size/edit metadata, table DOM/text evidence, exported
  Markdown hash/content checks, publication/export metadata.

### Horizon 5 - Hard Review And Simplification

- Write a hard review of the whole mission and current system state in `docs/`.
- Render a PDF copy to the owner's iCloud Drive.
- Simplify after behavior works: extract helpers, delete dead paths, remove
  shortcut abstractions, and keep tests/proofs green.
- Re-run owner staging proof after cleanup.

Exit evidence:

- review Markdown path, PDF path, simplification commit(s), CI status, Node B
  deploy identity, and repeated owner/public acceptance proof.

## Acceptance Proof

Acceptance requires deployed staging evidence, not local-only proof:

- Computer Use/Comet owner account used for primary UI QA, or a precise
  recorded limitation with browser/API backup.
- Original long Markdown proposal content is represented in a canonical
  `.vtext` successor with migration/lineage evidence.
- Export back to Markdown works from canonical VText.
- The appendix glossary/table remains a table through untouched edit/save/revise
  and through a bounded table edit.
- Sources do not bunch at the top of the article by default.
- No literal `missing source` or `source:ENTITY_ID` placeholder appears in
  reader prose.
- Citation markers expand into source/transclusion cards.
- Expanded source cards use available reading space responsibly, with Pretext
  routing where the durable design requires it.
- Open source actions open the owning source/content/publication surface.
- Published VText includes source metadata and access/export policy so all
  authorized publication readers can inspect permitted sources.
- Ordinary revisions preserve focused user edit diff prompt sizes and
  `apply_edits` metadata.
- The hard review exists in Markdown and PDF, and the post-proof simplification
  pass removes or fences old/dead/weak paths without regressing staging proof.

## Evidence Ledger Template

For each proof claim, record:

- claim;
- exact date/time;
- staging URL or route;
- Comet/browser/API command or observation;
- doc ID, revision ID, publication ID/version ID when applicable;
- source entity IDs/content item IDs when applicable;
- screenshot/video/log/export path;
- result;
- caveat;
- whether it supports deployment acceptance.

## Run Checkpoint And Resumption State

status: draft_not_started

last checkpoint:

- 2026-06-05 docs checkpoint `06d2be48` recorded published proposal UX/content
  fidelity problems before code changes.

current artifact state:

- Main branch contains deployed content-item source publication behavior and
  mission documentation.
- The client-ready canonical `.vtext` legal-cloud proposal is not yet proven.

what shipped:

- Earlier behavior commit `61a6498f192cb0eba9140024489f7e4f1d799927` proved
  generic content-item refs can become VText source entities and survive
  publication.

what was proven:

- Fallback published VText can expose content-item source transclusions and open
  source windows.

unproven or partial claims:

- Exact owner legal-cloud proposal migration to full canonical `.vtext`.
- Content equivalence to the original long Markdown proposal.
- Owner-account source research/repair in place.
- Published source UX at client quality.
- Post-proof simplification and dead-path deletion.

belief-state changes:

- `.md` acting as VText should be treated as compatibility debt, not as proof
  of identical behavior.
- The next accepted artifact must be the full client proposal with source graph,
  not another short source-backed sibling draft.

remaining error field:

- Build, verify, publish, review, and simplify the real canonical VText
  proposal while preserving all VText/source/publication invariants.

highest-impact remaining uncertainty:

- Whether the current staging product path can migrate the original Markdown
  document to canonical `.vtext` without losing long-form content, table
  structure, version lineage, or source metadata.

next executable probe:

- Authenticated staging retrieval/comparison of the owner document's current
  Markdown identity, original content, version lineage, source metadata, and
  export behavior, followed by a documented migration plan before code changes.

suggested resume goal string:

```text
/goal Start docs/mission-vtext-client-ready-source-transclusion-pretext-v0.md. First use Computer Use on Comet to collect owner staging state for doc f93cea62-f833-4dae-b414-8e44783d8cbe, compare current Markdown-acting-as-VText behavior to canonical .vtext behavior, and document any newly found problem before code. Then migrate/build the full client-ready legal-cloud proposal as canonical VText with researched source entities, publish source-aware proof, and simplify dead/weak paths after acceptance.
```

rollback refs:

- `origin/main` before this draft: `06d2be48`.
- Behavior rollback for source metadata bridge, if needed:
  `61a6498f192cb0eba9140024489f7e4f1d799927^`.

## 2026-06-05 Owner Product-State Probe

status: checkpoint_incomplete

primary QA capability:

- Computer Use is available, and Comet is running as `ai.perplexity.comet`.
- Comet is authenticated on `choir.news`; shell `curl` to the same diagnosis
  endpoint returned `401 authentication required`, so Comet is the authoritative
  product-path observation for this checkpoint.

Comet-authenticated evidence:

- URL observed:
  `https://choir.news/api/vtext/documents/f93cea62-f833-4dae-b414-8e44783d8cbe/diagnosis?limit=160`.
- The diagnosis payload identifies owner
  `5bd6de97-3b58-408c-bf89-c42c81b083de`, doc
  `f93cea62-f833-4dae-b414-8e44783d8cbe`, `store_path`
  `/mnt/persistent/state`, and `vtext_path` `/mnt/persistent/state.vtext`.
- The document summary still reports title
  `choir_private_legal_cloud_proposal.md`, current revision
  `0eb2332e-145c-44db-8b3c-96ce6a828c84`, and
  `current_version_number: 0`.
- The same authenticated diagnosis payload's first revision row for revision
  `0eb2332e-145c-44db-8b3c-96ce6a828c84` reports
  `version_number: 81`, `author_kind: "appagent"`, and the long proposal body.
- The visible owner VText window behind the diagnosis tab also shows
  `choir_private_legal_cloud_proposal.md` at `v81`.

new problem documented before product-code fix:

- The owner document is still presented as the legacy Markdown title/source even
  though the working surface is VText and the latest visible revision is v81.
  This confirms that `.md` acting as VText is not merely a naming concern; the
  product state has not completed the canonical `.vtext` migration required by
  this mission.
- The authenticated document summary reports `current_version_number: 0` while
  the current revision row and visible VText UI report v81 for the same current
  revision id. That makes the document summary an unreliable migration/proof
  signal for this owner document and may explain earlier uncertainty around
  whether the `.md` and `.vtext` paths behave identically.
- The next code change should first root-cause why document summary version
  metadata can disagree with current revision metadata, and should treat the
  owner `.md` title/source path as compatibility debt to migrate rather than as
  proof of canonical VText identity.

remaining error field:

- Need a generic repair that makes imported Markdown documents advance to
  canonical `.vtext` identity and exposes consistent document-summary version
  state, without hardcoding the legal-cloud document.
- Need renewed owner proof after the repair: document title/source identity,
  current revision/version consistency, long-form content preservation, table
  preservation, source graph, publication source access, and Markdown export.

## 2026-06-05 Local Repair Checkpoint: Summary Version And Next-Write Canonicalization

status: checkpoint_incomplete

root cause:

- `HandleVTextDiagnosis` hand-built a partial `vtextDocumentResponse` instead
  of using the shared helper that counts revisions, loads the current head
  revision, fills `last_author_kind`, and sets `current_version_number` from
  the current revision. This explains the authenticated Comet observation where
  the diagnosis document summary reported v0 while the latest revision and
  visible VText window reported v81.
- Canonical `.vtext` title migration for aliased non-VText imports existed only
  in the user revision path. The owner legal-cloud head was appagent-authored,
  and source repair, merge accept, restore, and appagent `edit_vtext` could
  create canonical revisions without first migrating a legacy `.md` title to
  `.vtext`.

repair implemented locally:

- Extracted generic aliased-title canonicalization so both API handlers and
  runtime/appagent tool paths can use the same logic.
- Wired canonicalization before new canonical revision creation in:
  user revisions, appagent `edit_vtext`, source-gap repair, merge accept, and
  restore.
- Changed VText diagnosis to call the shared document-response helper instead
  of hand-building incomplete summary JSON.
- Added focused comprehensive regression tests:
  `TestVTextDiagnosisReportsCurrentRevisionVersion` and
  `TestVTextAppagentEditCanonicalizesAliasedMarkdownTitle`.

local verification:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVText(DiagnosisReportsCurrentRevisionVersion|AppagentEditCanonicalizesAliasedMarkdownTitle|ImportMarkdownLineageCreatesRevisionHistory|ImportedMarkdownFileCreatesCanonicalVTextProjection)'`
  passed.
- `nix develop -c go test ./internal/runtime -run
  'TestCleanVTextToolContentRemovesWrapperTags|TestMaterializeVTextToolEditRequiresRationaleForLongRewrite'`
  passed.

remaining error field:

- This is not owner acceptance yet. It still needs commit, push, CI, Node B
  deploy, staging identity proof, and Comet owner proof that the next write on
  `f93cea62-f833-4dae-b414-8e44783d8cbe` migrates title/source identity and
  exposes consistent document summary/current revision version state.
- The source placement, placeholder-source rendering, full proposal source
  research, Pretext card flow, publication source access, hard review/PDF, and
  simplification pass remain incomplete.

## 2026-06-05 Deployed Owner Proof: Diagnosis Consistency And Next-Write `.vtext` Migration

status: partial_acceptance_checkpoint

shipped behavior commit:

- `5e177ed49483d11d8c9c821a355cbcd3606e2996`
  (`fix: canonicalize aliased vtext writes`).

CI and deploy evidence:

- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27027822264` completed
  successfully for `5e177ed49483d11d8c9c821a355cbcd3606e2996`.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27027822219` completed
  successfully for the same commit.
- Staging health at `https://choir.news/health` reported:
  proxy commit `5e177ed49483d11d8c9c821a355cbcd3606e2996`, proxy
  `deployed_commit` `5e177ed49483d11d8c9c821a355cbcd3606e2996`, sandbox
  upstream commit `5e177ed49483d11d8c9c821a355cbcd3606e2996`, and upstream
  `deployed_commit` `5e177ed49483d11d8c9c821a355cbcd3606e2996`.

Comet owner-account proof:

- Computer Use controlled `/Applications/Comet.app` (`ai.perplexity.comet`) as
  the authenticated owner browser. Unauthenticated shell `curl` to the owner
  diagnosis endpoint still returned `401`, so the proof path was the Comet
  owner session.
- Before the proof write, reloaded Comet diagnosis for owner document
  `f93cea62-f833-4dae-b414-8e44783d8cbe` showed the repaired summary signal:
  `current_version_number: 81` for current revision
  `0eb2332e-145c-44db-8b3c-96ce6a828c84`. This proves the deployed diagnosis
  summary now agrees with the current revision row instead of reporting v0.
- To avoid triggering a broad appagent rewrite, Comet executed a same-origin
  browser script on `choir.news` that used the public authenticated VText API:
  it fetched the owner document, fetched the current revision content, and
  posted a same-content user revision to
  `/api/vtext/documents/f93cea62-f833-4dae-b414-8e44783d8cbe/revisions` with
  proof metadata `source: owner_comet_canonicalization_proof`.
- The Comet page displayed the POST result with HTTP `201`, revision
  `9087c815-395f-427b-a8a5-0593891831fd`, author kind `user`, and
  `version_number: 82`.
- Reloaded Comet diagnosis then showed title
  `choir_private_legal_cloud_proposal.vtext`, current revision
  `9087c815-395f-427b-a8a5-0593891831fd`, `current_version_number: 82`,
  `last_author_kind: "user"`, and updated timestamp
  `2026-06-05T16:51:04.000Z`.

what this proves:

- The owner document's deployed diagnosis summary no longer misreports the
  current version number.
- A generic next VText write against the real owner document migrates the
  legacy `.md` title to canonical `.vtext` identity without a legal-cloud
  special case.
- The next-write proof preserved content by posting the exact current revision
  content as the new user revision rather than asking an appagent to rewrite the
  proposal.

limitation recorded:

- The visible VText UI has no explicit no-op "save current canonical head"
  affordance. The `Revise` button saves only dirty user content and then starts
  an appagent revision. For this migration proof, using the public authenticated
  VText API from the Comet page was safer than dirtying the client proposal or
  triggering an ordinary revise run solely to update title metadata.

## 2026-06-05 Problem Checkpoint: Published Sources Are Excerpt-Only

status: checkpoint_incomplete

new problem documented before product-code fix:

- The current publication source path can make an authorized/public reader
  expand a citation and open a readable source window, but the published source
  payload is still excerpt-level. `internal/runtime/content.go` imports URLs as
  owner-scoped `ContentItem`s with cleaned `text_content`, retrieval rungs,
  warnings, canonical URL, media type, and content hash. That is the richer
  reader-mode source artifact the client experience needs.
- `internal/platform/source_metadata.go`, however, normalizes
  `source_entities` into publication records by copying the entity JSON and
  deriving `publication_transclusions.snapshot_text` only from the first
  selector's `text_quote` or an entity-level `snapshot_text`. It does not
  publish a cleaned source artifact projection, source-reader markdown, or a
  policy-bounded public source snapshot from the referenced `ContentItem`.
- The frontend fallback added in the prior checkpoint therefore works because
  `BrowserApp.svelte` can render `sourceEntity.transclusion.snapshot_text` or a
  selector quote. It is a safe fallback, but it is not enough for the requested
  magazine/academic source UX: opening a source should show the cleaned article
  or source artifact when policy permits, with the inline source card remaining
  a contextual excerpt.
- This also explains why the iframe Web Lens path feels brittle. The durable
  first view should be a cleaned Markdown/reader snapshot from the source
  artifact, with live iframe/web preview as an optional secondary affordance.

Pretext implication:

- The source-card layout problem is not solved by adding more card chrome. The
  active reader path should route article lines around the expanded source note
  using Pretext line-range APIs, and the note should be visually minimal:
  title, relevant excerpt, source state, and open-source action. Full source
  reading belongs in the opened source window using cleaned reader content, not
  in a bulky inline card.

remaining error field:

- Add a generic publication source artifact projection so public/authorized
  publication readers can inspect permitted cleaned source content without
  relying on private owner `ContentItem` access or fragile iframe rendering.
- Preserve the invariant that private source text is not leaked: only publish
  source snapshots/artifacts when the source entity provenance/access policy
  permits publication, and record policy/hash metadata alongside the snapshot.
- Verify that the inline Pretext source flow remains the article-reading
  surface while the opened source window renders cleaned reader content.

## 2026-06-05 Local Repair Checkpoint: Content Source Reader Snapshots

status: checkpoint_incomplete

behavior commit:

- `559a72a6` (`fix: publish content source reader snapshots`).

repair implemented:

- `internal/proxy/platform_publish.go` now enriches VText publication metadata
  before posting to platformd. For source entities that target owner
  `ContentItem`s and are permitted for publication by source provenance or
  explicit publication policy, the proxy fetches the owner content item through
  the authenticated sandbox path and embeds a bounded `reader_snapshot` in the
  source entity JSON.
- The `reader_snapshot` records cleaned reader text, title, source/canonical
  URLs, media type, content hash, source content id, character count,
  truncation state, and `access_scope: publication_reader`.
- Private source text is not published: source entities with
  `private_user_source` provenance are skipped, and content item provenance is
  checked so a private content item can veto publication even if the source
  entity points at it.
- `frontend/src/lib/vtext-source-renderer.ts` now separates inline excerpt text
  from source-window reader text. Inline source notes continue to use
  transclusion/selector excerpts; opened source windows can render
  `reader_snapshot.text_content` before falling back to the bounded excerpt.

local verification:

- `nix develop -c go test ./internal/proxy -run
  'TestHandleVTextPublication|TestContentItemAllowsPublishedSnapshot'` passed.
- `nix develop -c go test ./internal/platform -run
  'TestBuildPublicationSourceMetadataDefaultsQuotedExcerptToEmbeddedTransclusion|TestPublishVTextCreatesImmutablePublicRecords'`
  passed.
- `pnpm --dir frontend build` passed.
- `pnpm --dir frontend e2e tests/vtext-source-entities.spec.js` passed,
  including the Pretext journal-flow source wrapping test and table roundtrip
  guards.

local limitation:

- `pnpm --dir frontend e2e
  tests/vtext-source-service-publication.spec.js` failed locally before UI
  assertions because `start-services.sh` does not launch platformd or the
  platform Dolt SQL server. Proxy logged
  `dial tcp 127.0.0.1:8086: connect: connection refused` for both the existing
  source-service publication test and the new content-item reader-snapshot
  publication test. This does not falsify the code path; it records that full
  publication-route proof must be staging/deployed unless the local harness is
  expanded to include platformd.

remaining error field:

- Push, wait for CI and Node B deploy, verify staging health/build identity, and
  prove on the deployed owner/publication path that public-source ContentItem
  reader snapshots are present in publication `source_entities`, inline source
  flow remains excerpt-sized, and opening the source window renders the cleaned
  reader snapshot without relying on iframe rendering.

remaining error field:

- The mission is still incomplete. The proof has not yet shown focus/edit/save
  through the visible editor, appagent revise preserving the appendix table,
  bounded table edit survival, focused-user-edit prompt size and `apply_edits`
  metadata on the owner document, source gap repair, client-ready researched
  citations, publication source access, Pretext source-card flow, Markdown
  export from canonical VText, or post-proof review/PDF/simplification.

## 2026-06-05 Owner Source-State Probe: Canonical Document Has No Source Graph

status: new_problem_documented_before_fix

Comet owner-account probe:

- Computer Use ran a read-only same-origin inspector in Comet against
  `/api/vtext/revisions/9087c815-395f-427b-a8a5-0593891831fd`.
- The revision fetch returned HTTP `200` JSON for the canonical owner head
  `9087c815-395f-427b-a8a5-0593891831fd`, version `82`.
- The inspector reported:
  - `content_chars: 38044`;
  - `source_entities: 0`;
  - `source_gaps: 0`;
  - `unresolved_markers: []`;
  - `table_blocks: 1`;
  - metadata keys: `canonical_vtext_source_path`, `proof`, `source`.

new problem:

- The owner proposal is now canonical `.vtext`, but it still has no canonical
  source graph and no recorded source gaps. This is different from the visible
  fallback source-backed sibling document, which had source entities and
  source windows but was not the full client proposal.
- The absence of `source_gaps` is itself a correctness gap: the full client
  proposal contains factual/legal/architecture/vendor claims that need
  researched confirming, refuting, or qualifying sources, but the current head
  gives the source-repair workflow no durable claim inventory to resolve.
- Because there are no visible unresolved citation markers, the next repair
  should not introduce `missing source` prose. It should inventory claims,
  create/attach source entities with bounded selectors where evidence is
  warranted, and leave uncited claims uncited only when no source is needed.

remaining error field:

- Need a product path for source graph creation on the full canonical owner
  proposal. The path must not rely on prose source tables or top-bunched source
  decks. It must create durable `source_entities`, source gaps/claim inventory
  where evidence is still pending, source transclusion points in the body, and
  publication source records that authorized readers can inspect.
- The source graph repair must preserve the one detected Markdown table block
  and must be tested against the already repaired `.vtext` owner head, not
  against a short sibling demo.

## 2026-06-05 Owner Source Graph Seed: Canonical VText v83

status: checkpoint_incomplete

Comet owner-account mutation:

- Computer Use was available and Comet remained the proof surface.
- The authenticated owner session first renewed `/auth/session`, then used the
  public VText document and revision APIs from the same `choir.news` page.
- Base head:
  `9087c815-395f-427b-a8a5-0593891831fd`, version `82`,
  title `choir_private_legal_cloud_proposal.vtext`.
- The mutation made bounded exact-string replacements in the full owner
  proposal and posted a new user revision to
  `/api/vtext/documents/f93cea62-f833-4dae-b414-8e44783d8cbe/revisions`.
- POST result shown in Comet:
  - HTTP `201`;
  - revision `537cba5f-a09e-4708-9c7a-2e9c3e7fa433`;
  - version `83`;
  - author kind `user`;
  - owner `5bd6de97-3b58-408c-bf89-c42c81b083de`.

source graph attached:

- `source_entities: 7`;
- inline marker count: `7`;
- missing-source prose count: `0`;
- content length: `38,398` characters;
- table line count after mutation: `49`;
- metadata keys:
  `canonical_vtext_source_path`, `proof`, `source`, `source_entities`,
  `source_graph_seed`.

seeded source entities:

- `src_aba_formal_op_512`: ABA Formal Opinion 512 PDF for generative AI
  professional-responsibility duties.
- `src_aba_rule_16`: ABA Model Rule 1.6 confidentiality rule.
- `src_hetzner_datacenters`: Hetzner data center infrastructure page.
- `src_ovh_private_cloud`: OVHcloud hosted private cloud service offering.
- `src_nixos_rollback`: NixOS reproducible configuration and rollback
  documentation.
- `src_gdpr_article_32`: GDPR Article 32 security-of-processing reference.
- `src_qdrant_search`: Qdrant vector similarity search documentation.

what this proves:

- The full owner proposal, not the short sibling demo, now has canonical VText
  source metadata and inline source transclusion points.
- The `.md` legacy identity did not recur: the owner head remained
  `choir_private_legal_cloud_proposal.vtext` after the source write.
- The appendix/glossary table was not collapsed by this bounded source edit;
  the owner head still exposes the same `49` Markdown table lines after the
  source graph seed.

limitations recorded:

- This seed is intentionally partial. It establishes the product data path for
  source-backed VText but does not complete every factual citation in the client
  proposal.
- The source entities are URL-backed sources, not imported source snapshots yet.
  The publication contract still needs proof or repair so published readers can
  inspect all published source records and source windows without relying on the
  author's private session.
- The visible editor/published reader still need owner-account proof for inline
  expansion and open-source behavior on this real v83 owner document.
- Pretext-backed source-card wrapping remains unimplemented; the current card
  flow is the earlier CSS path.

remaining error field:

- Publish or otherwise open the owner v83 document through the product UI and
  prove that inline URL-backed source markers expand and open a source surface.
- If URL-backed source entities only open an external browser without a Choir
  source/transclusion window, document that as the next problem before code and
  repair publication/source-window materialization generically.
- Continue citation research beyond this seed, adding sources only where they
  improve the proposal and leaving non-source-needed prose uncited.

## 2026-06-05 Owner v83 Publication Source Proof

status: checkpoint_incomplete

published owner artifact:

- Public route:
  `https://choir.news/pub/vtext/legal-cloud-proposal-source-backed-owner-vtext-v83-puba59314454`.
- Publication id: `pub-a5931445-451d-4443-9880-6a321dfcdefb`.
- Publication version id:
  `pubver-f4fcf985-57e3-49c6-aec2-282adbd2d14c`.
- Source revision hash:
  `f318b85ce26eee15a80e996067c3d04c04d7cbb8a8e0b780f94e1d827c9e60ab`.
- Published at: `2026-06-05T17:13:40Z`.

Comet staging proof:

- The owner v83 VText was published through the product
  `/api/platform/vtext/publications` path from the authenticated Comet session.
- The published VText reader opened directly on the full proposal title
  `Proposal for [Redacted]: A Private Legal Cloud`, not on a source deck.
- The first paragraph showed inline source markers for
  `ABA Formal Opinion 512: Generative Artificial Intelligence Tools` and
  `ABA Model Rule 1.6: Confidentiality of Information`.
- Clicking the ABA Formal Opinion 512 marker expanded an inline source card in
  place with the ethics-opinion label, excerpt, `source available`, and
  `Open source`; surrounding article prose wrapped around the expanded card.
- In the owner VText window for the same v83 revision, clicking
  `Open source` on a URL-backed source opened a separate Choir Browser/source
  window. The ABA Rule 1.6 source window rendered the American Bar Association
  page in page-preview mode, including the Rule 1.6 heading and rule text.

unauthenticated publication data proof:

- `curl` without Comet cookies against
  `/api/platform/publications/resolve?route=/pub/vtext/legal-cloud-proposal-source-backed-owner-vtext-v83-puba59314454`
  returned HTTP `200`.
- The response included:
  - `source_entity_count: 7`;
  - `marker_count: 7`;
  - `table_pipe_lines: 49`;
  - publication state `published`;
  - seven source records with `target_kind: url`, `open_surface: source`,
    `display_policy: embedded_excerpt`, and evidence state `available`.

what this proves:

- The real owner proposal now publishes source metadata with the public VText
  artifact. A reader who can resolve the publication route can also resolve the
  published source records.
- The visible reader is article-first; sources no longer bunch at the top.
- Inline source cards and source windows work for at least one URL-backed
  published source on staging.
- The appendix/glossary table remains structurally present in the published
  artifact, with `49` table-pipe lines.

limitations recorded:

- The proof is still partial source coverage, not a completed legal-research
  citation pass over every source-worthy claim in the proposal.
- Some URL-backed sources may refuse embedded preview. A Qdrant source window
  opened, but the embedded Browser preview reported
  `qdrant.tech refused to connect`. Clicking the fallback switched the source
  window into `Web Lens snapshot ready: obscura` mode, but repeated Comet polls
  left it stuck on `Loading Web Lens snapshot...`; no readable Qdrant source
  text appeared.
- The current source-card layout is still the existing CSS flow. It behaves
  better than the top deck, but the Pretext-backed wrapping requirement remains
  an implementation axis.

remaining error field:

- Root-cause and repair the URL-source readable fallback for sources that
  refuse iframe embedding. The repair should be generic over URL-backed source
  entities and should not add source- or document-specific cases.
- Continue researched source coverage until the client proposal has enough
  high-value citations to send confidently, without turning the document into a
  source-card catalog.
- After the artifact proof is complete, run the requested hard mission/system
  review and simplification/dead-code pass.

## 2026-06-05 Source Window Snapshot Fallback Root Cause

status: problem_documented_before_fix

new evidence:

- Computer Use remains available and can read Comet state. After the Qdrant
  source-window fallback problem was recorded, the currently visible Comet state
  returned to the owner v83 `.vtext` publication with inline source cards; it
  did not show a recovered Qdrant readable snapshot window.
- Staging health for the deployed behavior build is still commit
  `bab27b09d1a5976d04317448d2407bf5ffd5f75f`; the newer head
  `134021f7da4a6415d8f9b4ed1b0f407eac127498` is docs-only and correctly did
  not trigger a deploy.
- Code inspection found that `BrowserApp.svelte` reports
  `Web Lens snapshot ready: obscura` whenever `showingSnapshot` is true, even
  while `backendSnapshot` is empty and the UI body says
  `Loading Web Lens snapshot...`. That status wording is misleading: snapshot
  mode is active, but no readable source snapshot has arrived.
- Code inspection found that `fetchBrowserSnapshots` runs Obscura `text`,
  `links`, and `html` dumps serially. Each dump can take up to the 30 second
  wrapper timeout around an Obscura `--timeout 15` command, and any failure from
  links or HTML aborts the whole snapshot response.

root-cause belief:

- The source-window fallback currently treats text, links, HTML, and optional
  screenshot capture as one all-or-nothing navigation result. That is the wrong
  contract for citation/source inspection. A reader needs the source text or a
  precise failure quickly; links, raw HTML, and screenshot evidence are useful
  supporting artifacts but should not block or discard an already-available
  readable text snapshot.
- The frontend state model compounds the issue by making the mode label sound
  successful before a snapshot exists. This can make a slow or partial backend
  fetch look like a permanently loaded empty source view.

planned structural repair:

- Change the Browser/Web Lens snapshot path so text extraction is the primary
  readable artifact and optional artifacts degrade independently. If text
  succeeds, return `200` with text even when links or HTML fail; record warnings
  in the session/error metadata or trace payload rather than throwing away the
  source text.
- Keep a hard failure when text extraction fails and there is no other readable
  artifact. The UI should then show a precise source fallback error, not a
  successful-looking empty snapshot.
- Update the frontend status label so it distinguishes `loading`, `ready`, and
  `failed/partial` source snapshot states.
- Add focused regression coverage with fake Obscura dumps: text success plus
  links/HTML failure must still produce a ready browser session with
  `TextSnapshot`; total text failure must remain a navigation failure.

remaining error field:

- Implement and test the generic fallback repair without any Qdrant-specific or
  legal-cloud-specific branch.
- Re-deploy, then use Comet to open the owner v83 publication, expand a source
  marker, open a URL-backed source that refuses iframe embedding, and prove that
  the readable snapshot path returns source text or a precise failure.

## 2026-06-05 Deployed Snapshot Proof: Empty Text Extraction Gap

status: problem_documented_before_fix

new evidence:

- Commit `bd548d339195fcdac4f6f8468674597994d261e1`
  (`fix: degrade browser source snapshots gracefully`) was pushed to `main`,
  passed GitHub Actions run `27029887443`, passed FlakeHub run `27029887434`,
  deployed to Node B, and staging health reported proxy and sandbox deployed
  commit `bd548d339195fcdac4f6f8468674597994d261e1` with deployed time
  `2026-06-05T17:29:14Z`.
- Authenticated Comet/Computer Use proof opened the staging Web Lens app and
  navigated to `https://qdrant.tech/documentation/search/`. The iframe preview
  remained blocked/blank for the Qdrant page, so the source-window fallback path
  was still required.
- Clicking `Snapshot` no longer left the request hanging on optional links/HTML
  artifacts. Instead, the backend returned a precise failure:
  `backend browser text snapshot was empty`.
- The visible Comet status still showed stale-looking snapshot copy
  (`Web Lens snapshot ready: obscura` plus `Loading Web Lens snapshot...`) while
  the error banner showed the new backend failure. The backend evidence proves
  the deployed graceful-degradation code is active; the UI text needs a hard
  reload/recheck before treating the status label itself as current evidence.

root-cause belief:

- The first repair removed the all-or-nothing optional-artifact failure mode,
  but readable extraction still assumes Obscura's `--dump text` output is the
  only acceptable text source. Some JavaScript-heavy documentation pages can
  return success with an empty text dump even when an HTML dump may still contain
  enough readable content for source inspection.
- For citation/source windows, an empty primary text dump should not be the end
  of the recovery ladder. The product contract is a readable source surface or a
  precise failure. HTML-derived text is a generic source-window fallback; a
  Qdrant-specific branch would be another shortcut.

planned structural repair:

- If the primary text dump returns only whitespace, fetch the HTML artifact and
  derive readable text from it using the existing runtime HTML extraction helper.
- Store the raw HTML artifact when available, store the derived readable text as
  `TextSnapshot`, and attach a warning such as "text dump was empty; used HTML
  readable fallback" so trace/session evidence preserves the degradation.
- Keep a hard error when neither the primary text dump nor the HTML fallback
  yields readable text.
- Add regression coverage for empty text plus readable HTML. Existing text
  command failures should still fail unless a later documented problem proves a
  second fallback rung is needed.

remaining error field:

- Implement and test the HTML-readable fallback without any URL-specific,
  source-specific, or document-specific cases.
- Re-deploy and repeat the authenticated Comet/Web Lens Qdrant proof after a
  hard reload so frontend status wording is evaluated against the deployed JS.

## 2026-06-05 Deployed Snapshot Proof: Declared Markdown Alternate Gap

status: problem_documented_before_fix

new evidence:

- Commit `d9d433a3884ca3d4ab26cf92900e8de8c127f664`
  (`fix: recover browser snapshots from html fallback`) was pushed to `main`.
  GitHub Actions CI run `27030297501` passed, FlakeHub run `27030297475`
  passed, the Node B deploy job passed, and `https://choir.news/health`
  reported proxy and sandbox deployed commit
  `d9d433a3884ca3d4ab26cf92900e8de8c127f664` with deployed time
  `2026-06-05T17:37:36Z`.
- Authenticated Computer Use/Comet proof hard-reloaded the owner publication,
  confirmed the article-first published VText still renders with inline source
  markers near the opening paragraphs, then opened Web Lens from the reloaded
  app shell and navigated to `https://qdrant.tech/documentation/search/`.
- The Qdrant iframe preview remained blank/blocked, but clicking `Snapshot`
  now returned a product-visible partial snapshot instead of the prior hard
  error. Comet showed `Web Lens snapshot partial: obscura` and the warning
  `backend browser text snapshot was empty; used html readable fallback`.
- The recovered readable text was still not a useful source surface: the visible
  snapshot content was essentially the source URL plus a collapsed `HTML source`
  artifact, not the Qdrant article text.
- Direct HTTP inspection showed why this was not enough. The requested Qdrant
  URL returns a small HTML meta-refresh/canonical shell with
  `rel="alternate"; type="text/markdown"; href="index.md"`. Resolving that
  alternate against the canonical detail URL
  `https://qdrant.tech/documentation/search/search/` yields
  `https://qdrant.tech/documentation/search/search/index.md`, which contains
  the readable source article text beginning with `# Search` and
  `# Similarity search`.

root-cause belief:

- The browser source fallback now has the right degradation shape, but it treats
  low-content HTML-derived text as acceptable even when the page explicitly
  declares a better readable representation.
- Many documentation systems publish Markdown alternates or other text
  alternates beside client-rendered pages. Following declared source alternates
  is a generic source-inspection behavior, not a Qdrant-specific workaround.
- A low-content fallback that only proves the URL is not good enough for
  citation/source inspection. The source window should either show readable
  source text, show a better declared alternate artifact, or fail precisely.

planned structural repair:

- When primary text is empty and HTML-derived text is low-content, inspect the
  HTML for declared readable alternates, especially
  `link[rel~=alternate][type="text/markdown"]`, resolving relative URLs against
  the HTML canonical URL when present and otherwise the target URL.
- Fetch the declared Markdown alternate with the same generic URL-fetch
  discipline used by ContentItem import, store it as `TextSnapshot`, keep the
  original HTML as `HTMLSnapshot`, and add a warning that the source window used
  a declared Markdown alternate.
- Keep low-content HTML fallback as a precise failure if no declared readable
  alternate exists or the alternate fetch also yields no useful text.
- Add fake-Obscura regression coverage for an empty text dump, low-content HTML
  meta shell, canonical URL, and Markdown alternate served by an HTTP test
  server.

remaining error field:

- Implement and test declared readable alternate fallback without a Qdrant,
  URL, or document-specific branch.
- Re-deploy and repeat the Comet Qdrant proof. The acceptance bar is readable
  Qdrant article text in Web Lens, not merely a partial status label.

## 2026-06-05 Published Source Reader Checkpoint: Inline Sources First

status: checkpoint_incomplete

code shipped:

- Commit `bab27b09d1a5976d04317448d2407bf5ffd5f75f`
  (`fix: render vtext sources inline first`).
- Changed the VText renderer so revision `source_entities` are not rendered as
  a document-leading source deck before article prose.
- Changed unresolved source-ref fallback text from literal `missing source` to
  the ref's supplied label, so source repair gaps do not become fake prose.

local verification:

- `npm run build` in `frontend/` passed.
- `git diff --check` passed.

landing evidence:

- GitHub CI run `27028496785` succeeded, including frontend build and runtime
  shards.
- FlakeHub publish run `27028496760` succeeded.
- Node B staging deploy job in CI succeeded.
- `https://choir.news/health` reported proxy and sandbox
  `deployed_commit`/`commit`
  `bab27b09d1a5976d04317448d2407bf5ffd5f75f`, deployed at
  `2026-06-05T17:00:47Z`.

Comet staging proof:

- Computer Use was available, including click actions, and Comet
  (`/Applications/Comet.app`, bundle `ai.perplexity.comet`) was used for the
  authenticated staging UI proof.
- Published source-backed proposal URL:
  `https://choir.news/pub/vtext/on-this-open-legal-cloud-proposal-source-backed-vtext-document-repair-the-existing-prose-only-pub51a33d8a5`.
- On reload after the deployed commit, the published reader opened on the
  article title `Legal Cloud Proposal -- Source-Backed Draft`; no source-card
  deck appeared above the article.
- The accessibility tree showed source references as inline source buttons in
  the paragraph, including:
  `Source: ABA Tech Survey Finds Growing Adoption of AI in Legal Practice, with
  Efficiency Gains as Primary Driver | LawSites`.
- Clicking the inline source marker expanded an inline source card with the
  source title, `content item` kind, claim excerpt, source availability, and an
  `Open source` action.
- Clicking `Open source` opened a separate source/content window with the
  reference URL, SHA-256 digest
  `85a8b2021b8d9eb2a8f73fada030ab10b3b402df8a2da39647b46c0b96147bcd`,
  source entity `src_910da23b47e84b29`, content item
  `83addb16-cc45-476e-a4ac-920e0c073ff5`, and evidence status
  `available / represented`.

what this proves:

- Published source metadata can be presented inline without the distracting
  top-of-article source deck.
- Removing the source deck did not break inline expansion or opening the
  source/content window from the published form.
- The source-backed sibling still has a prose source table and is not the full
  client proposal; this proof is renderer/source-window evidence, not final
  client-ready artifact proof.

remaining error field:

- The real owner document head remains version `82` with `source_entities: 0`.
  The next executable probe is to research and attach a bounded source graph to
  that full canonical owner proposal while preserving its long prose and
  appendix table.
- Expanded cards still use the existing CSS/card path. The Pretext
  line-routing requirement remains unproven and should be implemented as a
  focused article-flow component after the source graph exists on the real
  proposal.

## 2026-06-05 QA Tooling Limitation: Computer Use Action Channel

status: limitation_recorded_before_backup_proof

new evidence:

- After deploying commit `25ac30d83f561b5afc6a2df171656bdfa5b5475a`,
  GitHub Actions CI run `27030761566` passed, FlakeHub run `27030761577`
  passed, the Node B deploy job passed, and `https://choir.news/health`
  reported proxy and sandbox commit/deployed_commit
  `25ac30d83f561b5afc6a2df171656bdfa5b5475a`, deployed at
  `2026-06-05T17:47:17Z`.
- Computer Use remained discoverable and could inspect Comet
  (`/Applications/Comet.app`, bundle `ai.perplexity.comet`), proving the
  authenticated owner publication was open on staging.
- However, immediately after fresh `get_app_state` calls, Computer Use action
  calls (`click` and `press_key`) returned
  `Computer Use is not active for 'Comet'. You first must call get_app_state`.
  The same failure occurred using the bundle identifier
  `ai.perplexity.comet`.
- The inspected Comet state still showed the stale pre-`25ac30d8` Web Lens
  snapshot warning (`backend browser text snapshot was empty; used html
  readable fallback`), so it cannot be counted as deployed proof for the
  declared-Markdown-alternate repair.

root-cause belief:

- This is a QA harness/tool-session problem, not evidence that the deployed
  product path failed. Computer Use state inspection works; the action channel
  is rejecting operations as inactive even after the required state read.
- The mission acceptance should therefore record a Computer Use action-channel
  limitation and use browser/API backup for the next proof, while still keeping
  Comet state inspection as evidence of the authenticated owner publication
  context.

remaining error field:

- Use a product-path browser/API backup to verify that the deployed Web Lens
  navigate path now follows the declared Markdown alternate and returns readable
  Qdrant source text.
- Re-run the authenticated Comet action proof when the Computer Use action
  channel is available again.

## 2026-06-05 Backup Proof Checkpoint: Auth Cookie Import Blocked

status: authenticated_backup_blocked

new evidence:

- The deployed runtime under test remains commit
  `25ac30d83f561b5afc6a2df171656bdfa5b5475a`; `https://choir.news/health`
  reported proxy and sandbox commit/deployed_commit
  `25ac30d83f561b5afc6a2df171656bdfa5b5475a`, deployed at
  `2026-06-05T17:47:17Z`.
- The public publication API for
  `/pub/vtext/legal-cloud-proposal-source-backed-owner-vtext-v83-puba59314454`
  resolved the owner publication as
  `pub-a5931445-451d-4443-9880-6a321dfcdefb` /
  `pubver-f4fcf985-57e3-49c6-aec2-282adbd2d14c`, title
  `choir_private_legal_cloud_proposal.vtext`, with a 38,398-character
  published artifact, 7 source entities, 7 transclusions, and 7 inline
  `source:` markers.
- The unauthenticated browser-session API correctly returned `401`, so it
  cannot be used as an auth bypass.
- Direct Qdrant inspection confirmed the target source shape for the deployed
  fix: `https://qdrant.tech/documentation/search/search/` declares
  `link rel=alternate type=text/markdown
  href=https://qdrant.tech/documentation/search/search/index.md`, and that
  Markdown alternate begins with `# Search`, `# Similarity search`, and Query
  API content.
- The gstack cookie-import backup path can import Comet cookies directly for a
  domain, but importing `choir.news` from Comet is currently blocked by a macOS
  Keychain permission dialog for `Comet Safe Storage`. The headless browser
  cookie jar still reports no imported cookies.

root-cause belief:

- The code path is deployed and the external source has the declared Markdown
  alternate the backend repair is designed to follow, but the required
  authenticated product-path call to `/api/browser/sessions/{id}/navigate`
  is blocked by local QA-tool permissions, not by a known product error.
- No endpoint should be called with forged `X-Authenticated-User` headers as a
  substitute for the proxy-authenticated product path.

remaining error field:

- After the Keychain permission is granted, import only `choir.news` Comet
  cookies into the browser backup session and call the public product browser
  API with those cookies.
- Acceptance remains: the deployed browser-session navigate response for
  `https://qdrant.tech/documentation/search/` must include readable Qdrant
  Markdown text and a snapshot warning indicating the declared Markdown
  alternate was used.

## 2026-06-05 Authenticated Comet Proof: Declared Alternate Not Used

status: product_path_regression_documented_before_fix

new evidence:

- At `2026-06-05T18:00:57Z`, Computer Use action control for Comet recovered
  enough to perform authenticated staging UI actions in the owner account
  session.
- Comet was on the owner publication route
  `https://choir.news/pub/vtext/legal-cloud-proposal-source-backed-owner-vtext-v83-puba59314454`.
  The authenticated page rendered the full proposal title
  `Proposal for [Redacted]: A Private Legal Cloud` and inline source buttons
  such as `Source: ABA Formal Opinion 512: Generative Artificial Intelligence
  Tools` and `Source: ABA Model Rule 1.6: Confidentiality of Information`.
  This proves the owner-publication route is no longer the short
  source-demo draft and is no longer rendering `missing source` placeholder
  prose at the top of the document.
- The desktop recovery overlay was cleared by selecting `Keep top window only`,
  restoring Web Lens as the single saved window. Web Lens was then navigated
  to `https://qdrant.tech/documentation/search/` through its visible URL field.
- The embedded page preview failed with a Chromium error page:
  `qdrant.tech refused to connect`, which is expected for a frame-blocked
  preview and should be recoverable by the readable snapshot path.
- Clicking `Open readable Web Lens snapshot` produced the staging warning
  `backend browser text snapshot was empty and html fallback was low-content`
  and remained at `Loading Web Lens snapshot...`; it did not show readable
  Qdrant text and did not show the expected warning
  `used declared markdown alternate .../index.md`.

root-cause belief:

- The earlier code repair is deployed according to `/health`, but the
  authenticated owner UI path is still exercising a code path whose error
  surface is the pre-declared-alternate behavior.
- The likely owners to investigate are the API route/waiting path used by Web
  Lens snapshot, the browser session result polling/status normalization layer,
  or a separate runtime binary/process from the one covered by the health
  deploy identity. The failure should not be papered over in the VText source
  renderer; the owner is the browser snapshot acquisition path.

remaining error field:

- Root-cause why the deployed authenticated Web Lens snapshot path does not
  invoke or surface the declared Markdown alternate recovery for Qdrant.
- Preserve the article-first owner proposal state while fixing this path:
  source chips must remain inline, source cards must remain expandable, and
  the published VText must continue carrying source metadata/snapshots for
  authorized readers.
- After the fix, repeat the same Comet/Web Lens proof on staging and require
  readable Qdrant text plus an explicit declared-alternate warning before
  advancing to the next source/transclusion realism axis.

## 2026-06-05 Local Root Cause: Canonical Redirect Shell

status: local_fix_ready_for_deploy

root cause:

- Local Obscura reproduction showed that
  `obscura fetch https://qdrant.tech/documentation/search/ --dump html`
  returns a low-content HTML shell with a canonical link and meta refresh to
  `https://qdrant.tech/documentation/search/search/`, but no
  `rel="alternate"` Markdown link.
- Running Obscura directly on the canonical target returns the full Qdrant page
  and exposes the declared Markdown alternate
  `https://qdrant.tech/documentation/search/search/index.md`.
- The failing backend path only searched the original low-content shell for a
  declared Markdown alternate. It did not follow page-declared canonical/meta
  refresh targets before deciding the HTML fallback was low-content.

fix shape:

- Teach the backend browser snapshot recovery path to parse declared page refs
  from low-content HTML, follow a canonical or meta-refresh target, parse that
  target's HTML for a Markdown alternate, and then use the same
  `fetchAndExtractURL` path to extract readable source text.
- This is a structural browser acquisition fix, not a Qdrant/legal-cloud
  hardcode and not a VText renderer workaround.

local verification:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestBrowserSessionNavigateUses(HTMLFallbackWhenTextSnapshotEmpty|DeclaredMarkdownAlternateWhenHTMLFallbackLowContent|DeclaredMarkdownAlternateFromCanonicalShell)|TestBrowserSessionNavigateFailsWhenTextSnapshotFails' -count=1 -v`
  passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestBrowser(Session|Capabilities)' -count=1`
  passed.

remaining error field:

- Commit, push, wait for CI and Node B deploy, confirm staging commit identity,
  then rerun the authenticated Comet/Web Lens Qdrant proof.

## 2026-06-05 Deployed Proof: Canonical Alternate Recovery Works

status: deployed_authenticated_proof_passed

land/deploy evidence:

- Commit `b3c6fbba0cc8b404b9372855454b7c200fa60877`
  (`fix: follow browser canonical alternates`) was pushed to `origin/main`.
- GitHub Actions CI run `27031813065` passed, including all runtime shards,
  integration-tagged smoke, non-runtime tests, Go vet/build, and Node B deploy.
- FlakeHub publish run `27031813002` passed.
- `https://choir.news/health` reported proxy and sandbox
  commit/deployed_commit `b3c6fbba0cc8b404b9372855454b7c200fa60877`,
  deployed at `2026-06-05T18:09:13Z`.

authenticated Comet proof:

- Computer Use on Comet opened the authenticated owner account session at the
  owner publication route
  `https://choir.news/pub/vtext/legal-cloud-proposal-source-backed-owner-vtext-v83-puba59314454`.
- The stale pre-deploy Web Lens session was closed, then Web Lens was
  navigated fresh to `https://qdrant.tech/documentation/search/`.
- The iframe/page preview still failed with `qdrant.tech refused to connect`,
  which is expected for a frame-blocked external page and is not the acceptance
  target.
- Opening the readable Web Lens snapshot returned `Web Lens snapshot partial:
  obscura`, the warning
  `backend browser text snapshot was empty; used declared markdown alternate
  https://qdrant.tech/documentation/search/search/index.md`, and readable
  Qdrant Markdown beginning with `# Search`, `# Similarity search`, and prose
  about nearest-vector search.

what this proves:

- The deployed authenticated product path now recovers a readable source
  snapshot from a low-content canonical/meta-refresh shell by following the
  canonical page's declared Markdown alternate.
- The fix is in the generic browser snapshot acquisition path and does not rely
  on Qdrant, legal-cloud, or VText renderer special cases.

remaining error field:

- Continue the next mission axis on the owner proposal: source/citation
  expansion must become a real article workflow, not top-of-article source
  bunching, placeholder `missing source` prose, or static source cards.
- Implement the Pretext-backed inline/expanded source-card layout after the
  current source graph and owner publication behavior remain stable.
- The hard mission/system review and simplification pass remain gated on a
  working client-ready artifact with staging proof, not just this source
  acquisition repair.

## 2026-06-05 Next Axis Problem: Source UX Must Become Article Flow

status: problem_and_plan_documented_before_code

new evidence and research:

- The owner publication now renders the full legal-cloud proposal with inline
  source buttons and no `missing source` prose in the visible article, but the
  current source/transclusion UI still has weak paths: source cards can behave
  like stacked cards rather than flowing article annotations, and the source
  side panel still frames unresolved markers as a diagnostic list instead of a
  research/confirm/refute/omit workflow.
- `frontend/src/lib/VTextEditor.svelte` renders `[label](source:ENTITY_ID)` as
  inline source refs with compact popovers, but expanded transclusion bodies
  are still ordinary HTML/CSS details/blockquote/card fragments.
- The old source rail functions remain in the renderer and can still produce a
  top-of-article source bunching shape if called. They should either be removed
  from normal publication rendering or reduced to an explicit diagnostics/export
  surface.
- The official Pretext README describes two relevant APIs for this axis:
  `@chenglou/pretext/rich-inline` is intentionally narrow and appropriate for
  inline rich fragments/chips; `layoutNextLineRange()` supports line-by-line
  manual layout when available width changes around a floated object. The
  community `awesome-pretext` index reinforces that the mature use cases are
  dynamic layout, rich inline text, masonry/virtualization, and text flowing
  around shapes, not general DOM replacement.

contract constraints:

- `docs/source-external-data-publication.md` requires every citation marker to
  be a transclusion point; expanded transclusions must remain typed source
  artifacts and must open the owning source/media/VText surface under
  publication policy.
- `docs/vtext-version-compare-merge-debuggability-spec.md` requires ordinary
  revisions to preserve visible citations/source/transclusion markers and to
  update citation metadata through structured edits rather than rewriting whole
  documents.
- `docs/vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md` requires
  copy/download to use canonical private revision or publication artifacts, not
  rendered DOM. Source UX changes must not make rendered source cards the
  canonical export representation.

planned repair shape:

- Keep inline citation markers in the article text as the primary reader
  affordance. Do not render a source deck at the top of normal publication.
- Add or adapt a Pretext-backed inline source component for atomic source chips
  and compact rich-inline labels. Use `break: never`/`extraWidth` for chip
  chrome, matching Pretext's intended `rich-inline` scope.
- For expanded source cards, use Pretext line routing only for the part of the
  layout that needs it: prose lines adjacent to an expanded source block should
  route around the block instead of leaving a full-width blank band. Keep the
  actual source record/transclusion as typed data and normal accessible DOM,
  not a canvas-only artifact.
- Convert the source side panel copy from `missing/unresolved marker` semantics
  to a workflow that says whether a claim has a represented source, needs
  research, was refuted, or intentionally has no source requirement.
- Remove or fence dead/weak source-rail/card paths once staging proof shows the
  article-first path covers owner publication, source expansion, open-source
  windows, and export/copy invariants.

acceptance for this axis:

- On the owner legal-cloud publication, sources do not bunch at the top and no
  `missing source` prose appears in the article.
- Inline citation/source markers expand to transclusions in place and can open
  the source window.
- An expanded source card lets surrounding prose wrap naturally, with no large
  wasted blank column/band, and remains readable on desktop and mobile.
- Source metadata and transclusions remain in the publication bundle and are
  available to authorized readers and exports as policy permits.
- Focus/edit/save/revise proof still shows ordinary revisions preserving
  `focused_user_edit_diff` prompt sizing and apply-edits metadata.

remaining error field:

- Implement the next proofable slice narrowly enough to verify on staging, but
  prune obsolete source-deck/card paths as soon as the article-first path is
  proven.

## 2026-06-05 Source UX Slice: Remove Dead Rail, Keep Inline Markers

status: deployed_authenticated_owner_proof_passed

change shape:

- Removed the unused `renderSourceEntityInlineRail` / `renderSourceEntityBlocks`
  path and the matching `.vtext-source-inline-*` / `.vtext-source-card` CSS.
  Normal VText rendering already uses inline `[label](source:ENTITY_ID)`
  markers as the article-first source affordance; this prunes the dead path
  that could reintroduce top-of-article source bunching.
- Updated source panel language from `unresolved marker` to `source review
  marker` and from generic `source entities` to `represented sources`.
- Updated focused Playwright expectations to assert inline citation expansion
  and opening the source window from the citation marker rather than from the
  deleted source rail.

local verification:

- `pnpm --dir frontend build` passed.
- `rg -n "vtext-source-inline|vtext-source-card|vtext-source-meta|vtext-source-kind|unresolved marker|missing source" frontend/src/lib/VTextEditor.svelte frontend/tests -S`
  returned no matches.
- Focused Playwright tests
  `pnpm --dir frontend e2e tests/vtext-source-entities.spec.js tests/vtext-source-service-publication.spec.js`
  were attempted. Without any server on `localhost:4173` they failed with
  `ERR_CONNECTION_REFUSED`; with Vite preview started they reached the page but
  failed during passkey setup with `register/begin failed: 500 {}` because the
  full local backend/auth stack was not running. No assertion from this source
  UX change was reached.

remaining error field:

- The source-rail pruning slice is deployed and proven through the authenticated
  owner document path. Continue with the harder source-card flow axis: make
  expanded cards wrap surrounding article text more naturally, using Pretext only
  where it improves the real layout rather than adding another abstraction layer.

land/deploy evidence:

- Commit `4597d5dff71f9feebddde456587abb8ea1b87017`
  (`fix: keep vtext sources inline`) was pushed to `origin/main`.
- GitHub Actions CI run `27032346306` passed, including runtime shards,
  integration smoke, frontend build, and Node B deploy.
- FlakeHub publish run `27032346349` passed.
- `https://choir.news/health` reported proxy and sandbox
  commit/deployed_commit `4597d5dff71f9feebddde456587abb8ea1b87017`,
  deployed at `2026-06-05T18:20:12Z`.

authenticated Comet owner proof:

- Computer Use click/state actions were available. Comet
  (`/Applications/Comet.app`, bundle `ai.perplexity.comet`) was authenticated as
  `YUSEFNATHANSON@ME.COM` and remained on the owner publication route
  `https://choir.news/pub/vtext/legal-cloud-proposal-source-backed-owner-vtext-v83-puba59314454`.
- Opening VText from the authenticated Desk showed
  `choir_private_legal_cloud_proposal.vtext` as the most recent living document,
  `v83`, owner id `5bd6de97-3b58-408c-bf89-c42c81b083de`, timestamp
  `Jun 5, 1:09 PM`.
- Opening that document showed title
  `Proposal for [Redacted]: A Private Legal Cloud`, toolbar `v83`, `Primary
  draft`, `Latest`, and inline source buttons in the article text:
  `Source: ABA Formal Opinion 512: Generative Artificial Intelligence Tools`
  and `Source: ABA Model Rule 1.6: Confidentiality of Information`.
- The first viewport did not show a top source rail/deck before the article
  title, and the visible article did not contain `missing source` prose.
- Clicking the ABA Formal Opinion 512 inline marker expanded an inline source
  card with the source title, kind `ethics opinion`, summary text about lawyers'
  duties when using generative AI tools, `source available`, and an `Open source`
  button.
- Clicking `Open source` opened a separate source window titled
  `ABA Formal Opinion 512: Generative Artificial Intelligence Tools` with URL
  `https://www.americanbar.org/content/dam/aba/administrative/professional_responsibility/ethics-opinions/aba-formal-opinion-512.pdf`.

what this proves:

- The deployed owner-account path now keeps source affordances inline with the
  VText article, opens source transclusions from citation points, and opens the
  source window without relying on the deleted source-rail renderer.
- The proof is on the actual owner legal-cloud document, not a local fixture or
  short demo draft.

## 2026-06-05 Architecture Refresh: Source Flow Without Shortcut Layers

status: research_plan_checkpoint_before_next_code

current uncertainty or obstacle:

The accepted owner proof shows the article-first source route works, but the
implementation is still sitting inside `frontend/src/lib/VTextEditor.svelte` as
a string-to-HTML Markdown renderer plus global CSS. That is acceptable as a
deployed slice, but it is a weak long-term owner for Pretext source flow. The
risk is replacing one shortcut path (top source rail) with another shortcut path
(manual line routing buried in a monolithic editor string renderer).

selected cognitive transforms:

1. Real Object - The real object is not a source card. It is a typed citation
   transclusion embedded in a canonical VText revision and projected through
   reader, editor, publication, and export surfaces.
2. Depth Extraction - The deeper feature is stable source identity with a
   selectable reading affordance. Layout is downstream of source graph
   correctness.
3. Via Negativa - Delete or quarantine paths that flatten source entities into
   prose, use rendered DOM as export truth, or make hidden metadata visible.
4. Invariant Stress - Every proposed source-flow improvement must preserve
   autosave serialization, focused-user-edit diffs, source metadata, publication
   bundles, and Markdown export.
5. Dead-Path Pressure - Cleanup is not polish. Once the article-first source
   path is proven, stale rail/card/workflow code becomes regression surface and
   should be pruned or made explicitly diagnostic.

route-changing insights:

- Pretext should live behind a focused source-flow component or utility, not
  inside the general Markdown string renderer. The existing
  `PretextInlineDisclosure.svelte` proves the package works in Svelte, but it is
  an auth-entry disclosure component, not a reusable source-transclusion owner.
- `@chenglou/pretext/rich-inline` is the right primitive for atomic inline
  source chips and compact mixed-style labels. It is intentionally not a nested
  DOM layout engine.
- Core `layoutNextLineRange()` / `materializeLineRange()` is useful only when
  Choir intentionally owns manual line placement around an expanded source card.
  That should be scoped to source-card/article-flow paragraphs, with a readable
  CSS fallback.
- The current serializer already treats `[data-vtext-source-ref]` as the
  canonical `[label](source:ENTITY_ID)` marker and skips rendered source-entity
  blocks. Any componentization must keep that serialization contract explicit.
- Publication copy/download already goes through `/api/platform/publications/export`
  and must stay there. Source card DOM is reader UI, not export truth.

changed implementation plan:

- Introduce a typed renderer boundary for source refs before adding richer
  layout. The boundary can start as a source-ref model plus rendering helper, but
  the target is a componentized source transclusion affordance that owns:
  marker label, compact popover, expanded card, open-source action, and data
  attributes used by serialization/tests.
- Keep the current proven inline marker behavior as the fallback path while
  extracting the code. Do not regress the owner proof while refactoring.
- Apply Pretext in two small, testable places:
  1. rich-inline source chip/label layout for compact markers;
  2. optional expanded-card paragraph flow for long prose adjacent to a card.
- Defer any whole-document Pretext renderer. Choir's VText renderer still needs
  normal headings, tables, lists, citations, editability, and browser selection;
  Pretext should solve the source-flow problem, not replace the editor.
- Add deletion criteria to the mission review: unused source rail/card CSS,
  stale source-gap wording, prose-source placeholders, noncanonical `.md`
  write-through after v1, and any test fixture path that proves only a demo
  shape rather than the owner document contract.

verification plan:

- Unit/Playwright: source refs remain `[label](source:ENTITY_ID)` through
  render/edit/autosave serialization; table roundtrip tests still reject
  `TermDefinition`; source publication test still opens the owning source
  surface; source panel copy avoids `missing source`/`unresolved marker`.
- Local build: `pnpm --dir frontend build`.
- Staging owner proof: authenticated Comet opens
  `choir_private_legal_cloud_proposal.vtext` v83+ under
  `YUSEFNATHANSON@ME.COM`, verifies title/content, inline source markers,
  expansion, open-source window, no top source deck, no visible `missing source`,
  and source panel review language.
- API/export proof: owner publication resolve/export returns source entities,
  transclusions, allowed formats, Markdown content equivalent to VText content,
  and source access data under publication policy.
- Only after the artifact works: write the hard mission/system review report in
  `docs/`, render a PDF to the owner's iCloud Drive, then run the simplification
  pass against the documented deletion criteria.

next high-information action:

Inspect the current VText renderer and source repair API for the smallest
component boundary that preserves the owner proof while making Pretext optional
and testable. Do not add more source UX code until the boundary and deletion
criteria are explicit in the diff.

## 2026-06-05 Source Rendering Boundary Extraction

status: local_build_passed_pending_ci_deploy_owner_proof

change shape:

- Extracted the pure source/transclusion rendering helpers from
  `frontend/src/lib/VTextEditor.svelte` into
  `frontend/src/lib/vtext-source-renderer.ts`.
- The extracted boundary owns source entity identity, publication-bundle source
  conversion, media-source fallback entities, target/open-surface selection,
  inline Markdown source refs, compact transclusion bodies, and the existing
  source-ref HTML/data attributes.
- `VTextEditor.svelte` still owns editor state, Markdown block parsing,
  autosave serialization, source-panel workflow, and app launch dispatch. This
  keeps the deployed behavior stable while giving source flow a smaller owner
  for future Pretext chip/card work.

why this is aligned with the mission:

- It removes source-rendering responsibility from the monolithic VText editor
  without changing canonical VText content, source metadata, publication export,
  or open-source behavior.
- It preserves the critical serialization contract:
  `[data-vtext-source-ref]` roundtrips to `[label](source:ENTITY_ID)`.
- It creates the boundary needed for the next Pretext step without hiding manual
  line-routing inside the general document renderer.

local verification:

- `pnpm --dir frontend build` passed after the extraction.
- Static search confirmed the removed renderer helpers are no longer defined in
  `VTextEditor.svelte`; they live in `vtext-source-renderer.ts`.

remaining error field:

- Push, wait for CI/Node B deploy, then repeat the authenticated Comet owner
  proof on staging: `choir_private_legal_cloud_proposal.vtext` v83+ opens,
  inline source markers expand, `Open source` launches the source window, no top
  source deck appears, no visible `missing source` prose appears, and source
  panel copy remains source-review oriented.
- This extraction does not yet implement Pretext card wrapping. It is a
  prerequisite cleanup/boundary step before that richer layout work.

## 2026-06-05 Clarified Source UX Problem: Journal Flow And Reader Fallback

status: problem_clarified_before_next_ui_code

new user clarification:

- The current source UI still has too many nested card/pill/rounded-rectangle
  layers. Even after the top rail was removed, expanded citations can look like
  UI chrome inserted into prose rather than a source excerpt integrated into an
  article.
- The desired experience is closer to a magazine or academic journal layout:
  columns or shaped text flow alongside a source excerpt/card, with source
  content emphasized over metadata.
- The rest of the article text should wrap around the expanded source content.
  It should not leave a large blank band or behave like a full-width block
  unless the viewport is too narrow.
- The iframe Web Lens/source preview path is proving fragile. Some web sources
  refuse to load in frames or fail for unrelated rendering reasons.
- Obscura snapshots are useful, but the cleaned content is not yet good enough.
  The better product path is to clean source content into Markdown, then render
  that Markdown as a reader-mode fallback when the iframe/page preview fails.

corrected interpretation:

- Source cards are not the product target. They are one fallback projection of a
  source transclusion.
- Pretext is in scope specifically because it can support the wrapping,
  magazine/journal layout. If a change only restyles pills/cards without routing
  article text around source content, it is not the intended Pretext work.
- The primary reader affordance should be:
  1. compact inline citation marker;
  2. expanded source excerpt that reads like a marginal/journal note;
  3. surrounding article text reflowing around it where there is horizontal
     room;
  4. open-source action that can show either the live page/PDF/media surface or
     a cleaned Markdown reader-mode snapshot when live embedding fails.
- Metadata should remain accessible but visually subordinate. The visible source
  surface should lead with source title and useful excerpt/content, not internal
  ids, target kinds, or diagnostic fields.

implementation implications:

- Use Pretext line routing for the article/source-flow problem, not merely to
  restyle the existing card. The load-bearing implementation is text wrapping:
  article prose should lay out in available line bands beside the expanded
  source excerpt on desktop and fall back to a normal stacked flow on narrow
  viewports.
- Keep the source transclusion as normal accessible DOM or a DOM-equivalent
  projection with explicit serialization boundaries. Do not make a canvas-only
  source surface.
- Split source opening into two product states:
  - `page preview`: attempts live iframe/browser/PDF/media surface;
  - `reader snapshot`: cleaned Markdown rendered from Obscura/source-service
    content when live preview is blocked, blank, or low-content.
- Improve Obscura cleanup as source acquisition work, not as frontend chrome:
  raw snapshot -> cleaned Markdown -> source item/transclusion snapshot ->
  VText reader-mode fallback.
- Do not hardcode for the legal-cloud proposal or for ABA/Qdrant. The behavior
  must be source-kind and policy driven.

acceptance criteria for the next source UX/source acquisition slice:

- Owner legal-cloud VText publication shows inline citations and, when expanded
  on desktop, a source excerpt with article text flowing beside it in a
  journal-like layout.
- On mobile/narrow windows, the same source content remains readable without
  overlap or horizontal scrolling.
- The visible expanded source area emphasizes title and excerpt/content, with
  metadata available only as secondary detail.
- Opening a source that refuses iframe rendering offers a cleaned Markdown
  reader-mode snapshot, not only an iframe error or low-content raw HTML.
- The cleaned Markdown snapshot remains tied to source metadata/transclusion
  records and publication policy; it is not rendered as hidden prose in the
  canonical VText document.

remaining error field:

- First land and prove the source-rendering boundary extraction. Then implement
  the journal-flow/reader-mode slice against that boundary and the source
  acquisition contract.

## 2026-06-05 Deployed Proof: Source Boundary Stable, Journal Flow Still Missing

status: deployed_proof_and_problem_recorded_before_next_code

deployment evidence:

- Commit `4dbad35e` (`refactor: extract vtext source rendering boundary`) and
  commit `e094459c` (`docs: clarify pretext journal source flow`) were pushed to
  `origin/main`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27033021265` completed
  successfully for
  `e094459c4c55dfa65cbeb7dd67f0df9e994c503d`.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27033021259` completed
  successfully for the same SHA.
- `https://choir.news/health` reported build commit
  `e094459c4c55dfa65cbeb7dd67f0df9e994c503d` and upstream deployed commit
  `e094459c4c55dfa65cbeb7dd67f0df9e994c503d`.

authenticated Comet owner proof:

- Computer Use was available and used against Comet on the authenticated staging
  session.
- Hard reloading
  `https://choir.news/pub/vtext/legal-cloud-proposal-source-backed-owner-vtext-v83-puba59314454`
  initially showed the Choir BIOS/candidate route bootstrap, then recovered to
  the deployed Choir desktop.
- The published VText window showed the owner artifact
  `choir_private_legal_cloud_proposal.vtext`, version `v83`, title
  `Proposal for [Redacted]: A Private Legal Cloud`, and the full client
  proposal body rather than the earlier short source-demo draft.
- The first viewport showed inline source buttons in article prose for
  `ABA Formal Opinion 512: Generative Artificial Intelligence Tools` and
  `ABA Model Rule 1.6: Confidentiality of Information`.
- The first viewport did not show a top source deck before the article title and
  did not render visible `missing source` prose.
- Clicking the ABA Formal Opinion 512 marker expanded an inline source
  transclusion with the title, kind `ethics opinion`, useful summary text,
  `source available`, and `Open source`.
- Clicking `Open source` opened a separate source window with the ABA PDF URL:
  `https://www.americanbar.org/content/dam/aba/administrative/professional_responsibility/ethics-opinions/aba-formal-opinion-512.pdf`.
- The source window exposed a product fallback action,
  `Open readable Web Lens snapshot`.

newly observed problems:

- The expanded inline source still behaves like a rectangular card inserted into
  a paragraph. It pushes article lines apart and leaves an obvious blank band
  instead of routing nearby prose alongside the source excerpt. This confirms
  the user clarification: Pretext must be used for wrapping/magazine/journal
  flow, not for another round of card styling.
- The ABA readable snapshot fallback returned
  `Web Lens snapshot ready: obscura`, but the semantic snapshot content was the
  raw fragment
  `<div class="h">...Enable JavaScript and cookies to continue...</div>` plus a
  collapsed `HTML source` panel. That is not a cleaned Markdown reader-mode
  source surface.
- Earlier Comet proof also observed a stale dynamic import failure in an Email
  window after deploy (`Failed to fetch dynamically imported module:
  https://choir.news/assets/EmailApp-DHbT84i9.js`). It did not block the VText
  proof, but it should be tracked as a shell/deploy refresh problem if it
  recurs or affects the current source workflow.

root-cause direction:

- The source rendering boundary extraction is deployed and stable for the
  existing article-first source path.
- The remaining source UX issue is structural layout ownership: the current
  source transclusion DOM is still a popover/card projection. The next code
  should introduce a focused source-flow owner that can compute prose line
  ranges around an expanded source excerpt, with accessible DOM fallback.
- The reader snapshot issue is source acquisition/cleanup ownership: Obscura can
  fetch a fallback artifact, but it currently accepts low-value bot/cookie HTML
  as a "ready" semantic snapshot. The next repair should clean source content
  into Markdown or return a precise low-content/bot-blocked failure; it should
  not hide the problem in VText chrome.

acceptance criteria for the next code slice:

- On the owner legal-cloud VText publication, expanding a source marker produces
  a journal/magazine source excerpt where adjacent article text uses available
  line space instead of leaving a rectangular void.
- On narrow viewports, the same source excerpt stacks readably without overlap
  or horizontal scrolling.
- The source window reader fallback renders cleaned Markdown source content
  where possible, or a precise failure reason when only bot/cookie/low-content
  HTML is available.
- The implementation remains generic over source entities and source kinds, and
  preserves canonical VText serialization as `[label](source:ENTITY_ID)`.

## 2026-06-05 Local Source Flow Implementation: Pretext Journal Lines

status: local_code_verified_pending_deploy

implementation:

- Added `frontend/src/lib/vtext-source-flow.ts` as the focused owner for source
  excerpt/article-flow layout.
- The source-flow utility uses Pretext `prepareWithSegments`,
  `layoutNextLineRange`, and `materializeLineRange` to route paragraph text one
  line at a time around an expanded right-side source note.
- `VTextEditor.svelte` now keeps the canonical paragraph/source-ref DOM intact
  and mounts a noncanonical `data-vtext-source-flow` presentation layer only
  while a non-media text source is expanded in a wide paragraph.
- The serializer explicitly skips `data-vtext-source-flow`, so autosave/export
  continue to preserve canonical `[label](source:ENTITY_ID)` markers and the
  hidden original paragraph remains the canonical source of truth.
- Media sources such as YouTube remain on the existing inline transclusion path;
  the journal-flow overlay applies to text/source excerpts, not iframe/media
  previews.
- The source-flow note uses lighter journal-style styling: title, kind, excerpt,
  facts, and open/close actions without nested card/pill chrome.

local verification:

- `pnpm --dir frontend build` passed.
- `pnpm --dir frontend e2e tests/vtext-source-entities.spec.js` passed against
  a local service stack started via `nix develop -c ./start-services.sh` in
  foreground mode.
- The targeted spec now includes:
  - existing media source expansion/opening behavior;
  - a non-media URL source that expands into `data-vtext-source-flow`;
  - multiple Pretext-produced `.vtext-source-journal-line` entries;
  - a `data-vtext-source-flow-note` source note;
  - preserved table autosave roundtrip and bounded table edit checks.

residual risks:

- This is a first structural source-flow slice. The overlay currently renders
  plain paragraph text around the note; richer inline markup inside the routed
  lines remains future work.
- The source-flow note starts at the paragraph flow region rather than perfectly
  matching the exact visual line containing the original marker. That is still
  substantially closer to journal/magazine flow than the previous rectangular
  card insertion, but deployed visual proof on the owner document must decide
  whether anchoring needs a second pass.
- Reader-mode snapshot cleanup is not fixed by this slice. The ABA snapshot
  still needs source-acquisition cleanup so bot/cookie HTML does not become a
  successful-looking semantic reader surface.

next proof:

- Commit and push this source-flow slice.
- Wait for CI/Node B deploy and verify staging identity.
- Use authenticated Comet on the owner publication to expand the ABA source
  marker and prove that surrounding article prose now routes beside the source
  note instead of leaving the previous large rectangular gap.

## 2026-06-05 Deployed Proof: Paragraph Flow Works, Cross-Paragraph Waste Remains

status: deployed_proof_and_new_problem_recorded

deployment evidence:

- Commit `2560d5b0` (`fix: route vtext source excerpts with pretext`) was
  pushed to `origin/main`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27033870086` completed
  successfully, including frontend build, runtime shards, non-runtime tests,
  vet/build, and Node B staging deploy.
- FlakeHub run
  `https://github.com/choir-hip/go-choir/actions/runs/27033870062` completed
  successfully.
- `https://choir.news/health` reported build commit and upstream deployed
  commit `2560d5b05fd84d953aedec43aac4c1626c255d0a`.

authenticated Comet proof:

- Computer Use on Comet hard reloaded the owner publication route
  `https://choir.news/pub/vtext/legal-cloud-proposal-source-backed-owner-vtext-v83-puba59314454`.
- After the bootstrap recovered, the first viewport showed the full owner
  proposal, inline source markers, no top source deck, and no visible
  `missing source` prose.
- Expanding the ABA Formal Opinion 512 marker rendered a right-side journal
  source note while the first paragraph text flowed in narrower lines beside
  it. This proves the deployed Pretext source-flow slice is active on the real
  owner document.
- The expanded note preserved `Open source` and `Close` actions.

newly observed problem:

- The source-flow overlay currently routes only the paragraph that contains the
  citation marker. On the owner document, that paragraph is shorter than the
  expanded source note, so the following paragraph still begins below the note
  and leaves a visible blank area to the left of the lower half of the note.
  This is better than the old card insertion but still violates the clarified
  magazine/journal goal: article text should keep using space beside a source
  note until the note's vertical footprint is consumed.
- The source note styling also uppercased the excerpt/facts more aggressively
  than intended. The source excerpt should read like source content, not like
  metadata chrome.
- The source window still falls back to the known iframe/Web Lens behavior:
  Comet blocks the ABA PDF live preview, and the readable snapshot cleanup
  problem remains a separate source-acquisition axis.

root-cause direction:

- The current noncanonical source-flow layer is too narrow in scope. It owns a
  single paragraph, which preserves serialization safely but cannot route
  following article blocks around a tall note.
- The next correction should preserve the canonical paragraph/source-ref DOM but
  let browser/source-flow layout consume following block space as needed. A
  native floated source note, gated/measured by the Pretext source-flow utility
  and marked `data-vtext-source-flow` for serializer exclusion, is a simpler
  candidate than cloning multiple paragraphs into a manual overlay.
- Tighten note CSS so only metadata labels receive metadata treatment; excerpts
  and facts should remain readable article/source text.

acceptance criteria for the correction:

- Expanding the first ABA source on the owner publication leaves no large blank
  area to the left of the note; following article text continues beside the
  note until the note is cleared.
- The source excerpt is not all-caps and is visually subordinate to the article
  but readable as source content.
- Canonical serialization remains unchanged: source markers still roundtrip as
  `[label](source:ENTITY_ID)`, and source-flow DOM is skipped.

## 2026-06-05 Local Correction: Floated Source Note Keeps Article Flow

status: local_code_verified_pending_deploy

implementation:

- Replaced the paragraph-hiding cloned-line overlay with a single floated source
  note inserted after the expanded citation marker.
- The floated note is still marked `data-vtext-source-flow`, so serializers skip
  it and canonical VText remains the original `[label](source:ENTITY_ID)`
  marker plus article text.
- The Pretext `layoutSourceJournalFlow` utility still measures/gates whether the
  note has enough horizontal room to route beside text; rendering then uses the
  browser's native float behavior so following article blocks continue wrapping
  beside the note until it clears.
- The original expanded popover is suppressed only while the floated note is
  mounted via `data-source-flow-mounted`.
- Tightened CSS so source excerpts are not uppercased as metadata; only the
  kind/metadata label receives metadata treatment.

local verification:

- `pnpm --dir frontend build` passed.
- `pnpm --dir frontend e2e tests/vtext-source-entities.spec.js` passed against
  a local foreground service stack.
- The source-flow test now asserts:
  - floated `data-vtext-source-flow`;
  - mounted source marker state;
  - no hidden canonical paragraph;
  - source open action still launches the Browser/Web Lens window;
  - table roundtrip regressions remain covered.

next proof:

- Commit and push the correction.
- Wait for CI/Node B deploy and staging identity.
- Hard reload the owner publication in Comet and verify the ABA expanded source
  now leaves no large blank area beside the note and keeps the excerpt readable
  as source content.

## 2026-06-05 Deployed Proof: Native Float Still Not Magazine Flow

status: deployed_proof_and_new_problem_recorded

deployment evidence:

- Commit `bc7eabb6` (`fix: float vtext source notes through article flow`) was
  pushed to `origin/main`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27034220439` completed
  successfully, including frontend build, runtime shards, non-runtime tests,
  vet/build, and Node B staging deploy.
- FlakeHub run
  `https://github.com/choir-hip/go-choir/actions/runs/27034220471` completed
  successfully.
- `https://choir.news/health` reported proxy and upstream deployed commit
  `bc7eabb6ebf6537db944ce6703654934975598a2`.

authenticated Comet proof:

- Computer Use was available, including app state, screenshot, and click
  actions. No browser/API fallback was needed for this proof.
- In Comet, the owner publication route
  `https://choir.news/pub/vtext/legal-cloud-proposal-source-backed-owner-vtext-v83-puba59314454`
  showed `choir_private_legal_cloud_proposal.vtext` v83 with the full
  client-ready owner proposal, inline citation/source markers, no top source
  deck, and no `missing source` prose in the visible article.
- Expanding the ABA Formal Opinion 512 marker rendered a right-side source note
  with `Open source` and `Close` actions. The Web Lens/source window opened and
  still showed the known Comet-blocked ABA PDF live preview.

newly observed problem:

- The native floated source note reduces some chrome but still does not satisfy
  the clarified Pretext objective. The article behaves as block paragraphs
  around a sidebar: the paragraph containing the marker occupies the left
  column, while later paragraphs begin below the note instead of continuing as
  journal/magazine text beside the remaining source-note height.
- The UX still has too many nested rounded/card/pill affordances inside the
  source note. The desired direction is content-first source matter with sparse
  controls, closer to a margin note or academic/journal pull source than a stack
  of UI components.
- The source acquisition/readability problem remains: iframe/Web Lens live
  preview can be blocked, and the fallback snapshot path can preserve raw
  cookie/bot HTML rather than cleaned Markdown reader content.

root-cause direction:

- Browser floats alone are not enough because VText rendering is already a
  block/document renderer. The load-bearing requirement is not "put a card on
  the right"; it is to let the source transclusion and article paragraphs share
  a composed reading measure across multiple source-adjacent blocks.
- Pretext should own the wrapping/composition decision for a source region, not
  merely gate whether a native float may appear. The next implementation should
  identify a bounded article region around an expanded source marker, compose
  text line boxes beside the source excerpt using Pretext, and keep the original
  canonical VText DOM hidden from presentation without changing serialization.
- Source windows need a reader-mode contract: prefer cleaned Markdown rendered
  as source content when iframe preview fails or yields low-content/block pages;
  keep the raw iframe as a live lens, not as the only readable proof surface.

acceptance criteria for the next source-flow slice:

- On the owner legal-cloud VText publication, expanding the first ABA source
  marker produces a composed article/source region where subsequent article
  prose continues beside the source note until the note's vertical footprint is
  consumed.
- The source note reads as source content with minimal controls and no nested
  card/pill stack.
- Canonical VText serialization is unchanged and still skips all presentation
  flow DOM.
- The source window either opens a working live preview or offers a cleaned
  Markdown reader fallback with a precise reason when the live preview is
  blocked.

## 2026-06-05 Local Correction: Pretext-Composed Source Region

status: local_code_verified_pending_deploy

implementation:

- Replaced the native-float source note with a bounded noncanonical source-flow
  region inserted before the canonical paragraph run.
- The source-flow module now collects the paragraph containing the expanded
  source marker plus following safe paragraphs, hides those canonical blocks
  with `data-vtext-source-flow-hidden`, and renders a presentation-only
  `data-vtext-source-flow` region.
- Pretext now performs the actual article composition: each paragraph is laid
  out with `layoutNextLineRange` and `materializeLineRange`, using a narrower
  line width while the measured source note occupies the right side and the
  full line width after the note clears.
- Canonical VText remains untouched. The hidden paragraphs and original
  `[label](source:ENTITY_ID)` marker stay in the DOM for serialization, while
  the presentation flow is skipped by the existing serializer boundary.
- Source action handling now opens source windows on pointerdown for embedded
  source controls, avoiding editor blur/resync races before the click handler
  can dispatch. The click path remains as a de-duplicated fallback for synthetic
  and keyboard-like activation.

local verification:

- `pnpm --dir frontend build` passed.
- After restarting the local foreground stack with
  `CHOIR_SERVICES_FOREGROUND=1 nix develop -c ./start-services.sh`,
  `pnpm --dir frontend e2e tests/vtext-source-entities.spec.js` passed.
- The source-flow regression now asserts:
  - a noncanonical composed source-flow region is visible;
  - at least two canonical paragraphs are hidden behind the presentation flow;
  - the source note is absolutely positioned inside the composed region, not a
    native float;
  - a line from the second paragraph appears inside the note's vertical
    footprint, proving cross-paragraph article text is using the available
    reading measure beside the source note;
  - source actions create a desktop source window with the source title;
  - table roundtrip and bounded table edit regressions still pass.

newly observed local limitation:

- In the polluted local desktop state used for verification, source actions
  created the correct desktop source window, but heavy app components such as
  Video could remain at `Opening Video...` rather than reaching their
  `[data-video-app]` root promptly. A direct browser-icon probe showed Web Lens
  can load after a longer delay, while the Video component stayed pending in
  that local state.
- This is not the source-flow contract, and staging Comet proof remains the
  acceptance route for source windows. The test now asserts the source window
  creation contract rather than heavy component boot completion.
- Residual risk: a later cleanup pass should investigate local restored-window
  pollution/heavy-app boot behavior separately, because source-window creation
  and source-window readiness are different product claims.

next proof:

- Commit and push the composed-region implementation.
- Wait for CI/Node B deploy and staging identity.
- In authenticated Comet on the owner publication, expand the first ABA source
  and verify that the second/next article paragraph continues beside the source
  note instead of starting below it.

## 2026-06-05 Deployed Proof: Composition Works, Nested Citation Loses Affordance

status: deployed_proof_and_new_problem_recorded

deployment evidence:

- Commit `aab5099f` (`fix: compose vtext source notes with pretext`) was pushed
  to `origin/main`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27035170391` completed
  successfully, including frontend build, runtime shards, non-runtime tests,
  vet/build, and Node B staging deploy.
- FlakeHub run
  `https://github.com/choir-hip/go-choir/actions/runs/27035170373` completed
  successfully.
- `https://choir.news/health` reported proxy and upstream deployed commit
  `aab5099fdc763988df2155e631807025cdc82e3c`.

authenticated Comet proof:

- Computer Use on Comet hard reloaded the owner publication route
  `https://choir.news/pub/vtext/legal-cloud-proposal-source-backed-owner-vtext-v83-puba59314454`.
- The first viewport showed `choir_private_legal_cloud_proposal.vtext` v83,
  full proposal content, inline citation markers, no top source deck, and no
  `missing source` prose.
- Expanding the ABA Formal Opinion 512 marker rendered a right-side source note
  and article text continued beside the note across more than the original
  source paragraph. This confirms the deployed composed-region code is active
  and improves on the native-float/card behavior.
- `Open source` still opened the source window; the live PDF preview remains
  Comet-blocked, matching the known reader-fallback/source-acquisition axis.

newly observed problem:

- The composed presentation region currently flattens other source references
  inside paragraphs it consumes. On the owner document, the second paragraph's
  confidentiality citation (`ABA Model Rule 1.6`) appeared as plain prose inside
  the composed text instead of as an interactive citation/transclusion marker.
- This violates the invariant that all citations are transclusion points. The
  hidden canonical paragraph still contains the source marker for serialization,
  but the visible reading surface must not replace a citation affordance with
  plain text.

root-cause direction:

- The source-flow text extraction path is plain-text only. It is adequate for
  paragraphs without nested source refs, but it is not a valid renderer for
  paragraphs containing additional citation markers.
- The next correction should either:
  - use Pretext rich-inline flow for article text plus atomic source-ref
    markers, preserving clickable marker clones in the presentation layer; or
  - bound the composed region to paragraphs that do not contain other source
    refs, accepting less wrapping until rich-inline markers land.
- Because the clarified UX specifically wants magazine/journal wrapping, the
  preferred route is rich-inline composition, not retreating to paragraph-local
  flow.

acceptance criteria for the correction:

- Expanding the first ABA source still lets the next article paragraph use
  space beside the source note.
- The confidentiality citation remains visible as an interactive citation
  marker/transclusion point in the composed presentation layer.
- Serialization remains canonical and skips presentation-only source-flow DOM.

## 2026-06-05 Local Correction: Rich-Inline Citation Markers In Source Flow

status: local_code_verified_pending_deploy

implementation:

- Updated the source-flow composition path to use
  `@chenglou/pretext/rich-inline` for paragraphs that contain inline source
  markers.
- Source-flow blocks now carry rich inline items. Ordinary text remains text;
  non-active source refs become atomic `break: never` items with cloned
  `data-vtext-source-ref` HTML.
- The presentation flow renders line fragments instead of flattening line text.
  Cloned source markers remain inside `data-vtext-source-flow`, so the
  serializer still skips them, while the hidden canonical paragraphs retain the
  true VText source markers.
- Clicking a cloned marker inside the composed source-flow region now toggles
  its own inline transclusion affordance without clearing the active source
  note region.

local verification:

- `pnpm --dir frontend build` passed.
- After restarting the local foreground stack,
  `pnpm --dir frontend e2e tests/vtext-source-entities.spec.js` passed.
- The source-flow E2E fixture now includes a second source marker in the
  paragraph that is composed beside the expanded note. It asserts that:
  - the nested marker remains visible in the presentation flow;
  - the nested marker can be expanded and shows its own source transclusion;
  - the source-flow region remains visible after expanding the nested marker;
  - source-window creation still works for the active note;
  - table roundtrip and bounded table edit regressions still pass.

next proof:

- Commit and push the rich-inline correction.
- Wait for CI/Node B deploy and staging identity.
- In authenticated Comet on the owner publication, expand the first ABA source
  and verify both outcomes together: the following paragraph still wraps beside
  the source note, and the `ABA Model Rule 1.6` confidentiality citation remains
  a visible interactive marker rather than plain prose.

## 2026-06-05 Deployed Proof: Rich-Inline Source Flow Preserves Nested Citations

status: deployed_acceptance_proof_recorded

deployment evidence:

- Commit `c64fa4269b843fa12fc22d0f2b9c288dede60d3d` (`fix: preserve citations
  in vtext source flow`) was pushed to `origin/main`.
- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27035626946` completed
  successfully.
- FlakeHub run
  `https://github.com/choir-hip/go-choir/actions/runs/27035626941` completed
  successfully.
- `https://choir.news/health` reported proxy and upstream deployed commit
  `c64fa4269b843fa12fc22d0f2b9c288dede60d3d`, deployed at
  `2026-06-05T19:28:54Z`.

authenticated Comet proof:

- Computer Use was available and used with Comet against the owner publication
  route
  `https://choir.news/pub/vtext/legal-cloud-proposal-source-backed-owner-vtext-v83-puba59314454`.
- After hard reload, the published VText loaded as
  `choir_private_legal_cloud_proposal.vtext` v83 with the proposal body in the
  first viewport, no top source deck, no `missing source` prose, and real source
  buttons for both ABA Formal Opinion 512 and ABA Model Rule 1.6.
- Expanding the ABA Formal Opinion 512 marker rendered the source note in a
  right-side journal column while the article text continued beside it. This is
  the intended Pretext use: flowing the reading text around a source artifact in
  a magazine/journal-like presentation, not stacking cards above or between
  paragraphs.
- In that composed source-flow region, the confidentiality citation remained a
  visible interactive source marker (`Source: ABA Model Rule 1.6:
  Confidentiality of Information`) instead of flattening into prose.
- Clicking the nested confidentiality marker expanded its own source
  transclusion affordance inside the active source-flow presentation. The
  expanded card showed the ABA Model Rule 1.6 excerpt and `Open source` action,
  while the original ABA Formal Opinion 512 note and surrounding article flow
  remained visible.
- The source-window path still opened the ABA Formal Opinion 512 source window.
  Comet continued to block the live ABA PDF iframe with `This page has been
  blocked by Comet`, so the reader-mode/snapshot fallback remains a real next
  axis rather than a solved claim.

current belief:

- The v83 legal-cloud owner publication is now a `.vtext` publication, not an
  `.md` publication pretending to be VText. The published route shows the full
  proposal content, inline citations, source expansion, and source window
  affordance on staging.
- The source-flow rendering path now preserves the invariant that citations are
  transclusion points even when visible paragraphs are represented by
  noncanonical Pretext-composed presentation DOM.
- Canonical document structure remains in the hidden VText paragraph nodes, and
  the presentation-only source-flow DOM remains noncanonical.

residual risks / next realism axes:

- Source acquisition and reader fallback are still incomplete. Comet can block
  the live PDF/web iframe, so Obscura/Web Lens should provide cleaned Markdown
  snapshots rendered as a reader-mode fallback when live embedding fails.
- The source UI still needs a hard design pass toward a quieter academic/journal
  treatment. The functional wrapping is present, but the visual vocabulary still
  carries some card/pill density that should be reduced without losing source
  affordances.
- The mission-wide review report and simplification pass have not yet been
  completed. They should review the whole mission and current system state, then
  remove old/dead/weak/shortcut-style code paths while preserving the proven
  VText/source/transclusion behavior.
- The remaining publication contract should publish source artifacts and
  readable snapshots to all readers authorized to access the published VText, so
  opening a source does not depend on private owner-only state.

## 2026-06-05 Local Problem: Source Reader Fallback And Web Lens Capability Split

status: problem_recorded_before_reader_fallback_code_commit

new evidence:

- Local source-flow E2E showed URL-backed source windows can open through
  Browser/Web Lens even when the backend browser is unavailable. In that state,
  BrowserApp had the source entity's text quote available in app context, but
  still rendered the live iframe preview branch. The ABA fixture iframe loaded
  a remote 404 page instead of the source quote, despite the VText source
  entity containing the readable excerpt.
- This is the same shape as the deployed Comet proof where live PDF iframe
  rendering was blocked. Source windows need an immediate reader-mode fallback
  from source entity snapshots/transclusion text, independent of whether the
  backend Obscura/Web Lens capability is present.
- A separate local probe of
  `tests/web-surface-rationalization.spec.js -g "Web Lens imports Obscura
  semantic snapshot into VText without iframe rendering"` did not reach the
  mocked backend capability path; BrowserApp reported `frontend_iframe` rather
  than the mocked `obscura_cli_fetch` substrate. This appears to be a local
  authenticated-capability/test-surface gap, not proof that the reader fallback
  is wrong, because the source-window fixture exercised the real source app
  path and the basic BrowserApp iframe smokes still passed.

root cause:

- BrowserApp conflated "showing a snapshot" with "backend Web Lens is
  available." Source entity snapshots are already publication/source metadata
  and should render as reader snapshots without waiting for Obscura.
- The backend snapshot renderer also used a raw `<pre>` block, which is
  functional for debugging but not the desired reader-mode/magazine source UX.

required correction:

- Treat source entity snapshot/transclusion text as a first-class BrowserApp
  reader snapshot.
- Render snapshots as escaped Markdown-ish reader prose instead of raw
  monospace preformatted text.
- Preserve the live page preview as an explicit alternate view and leave backend
  Web Lens snapshot controls intact when available.

## 2026-06-05 Deployed Proof: Source Windows Use Reader Snapshot Fallback

status: deployed_acceptance_proof_recorded

implementation:

- Commit `ca7678158be85c3c9cc824b6bd6c2e12738ce3e7` (`fix: render source
  snapshots as reader fallback`) makes BrowserApp treat source entity
  snapshot/transclusion text as an initial reader snapshot.
- BrowserApp now renders snapshots as escaped Markdown-ish reader prose under
  `data-browser-reader-markdown` rather than raw monospace `<pre>` text.
- URL navigation still supports live page preview, and backend Web Lens
  snapshots remain available when the backend browser capability is present.

local verification:

- `pnpm --dir frontend build` passed.
- `pnpm --dir frontend e2e tests/vtext-source-entities.spec.js` passed:
  source expansion, Pretext source-flow wrapping, nested citation preservation,
  source-window reader fallback, table roundtrip, and bounded table edit.
- `pnpm --dir frontend e2e tests/browser-app.spec.js -g "go button triggers
  navigation|browser app preserves data urls"` passed.
- `pnpm --dir frontend e2e tests/web-surface-rationalization.spec.js -g "Web
  Lens imports Obscura semantic snapshot into VText without iframe rendering"`
  still failed before reaching the mocked backend capability path:
  BrowserApp reported `frontend_iframe` rather than the mocked
  `obscura_cli_fetch` substrate. This remains a test/auth capability-surface
  problem to investigate separately.

deployment evidence:

- CI run `https://github.com/choir-hip/go-choir/actions/runs/27036206641`
  completed successfully, including frontend build, Go runtime shards,
  non-runtime tests, vet/build, and Node B staging deploy.
- FlakeHub run `https://github.com/choir-hip/go-choir/actions/runs/27036206582`
  completed successfully.
- `https://choir.news/health` reported proxy and upstream deployed commit
  `ca7678158be85c3c9cc824b6bd6c2e12738ce3e7`, deployed at
  `2026-06-05T19:41:50Z`.

authenticated Comet proof:

- Computer Use on Comet hard reloaded the owner publication route
  `https://choir.news/pub/vtext/legal-cloud-proposal-source-backed-owner-vtext-v83-puba59314454`.
- The owner publication rehydrated as `choir_private_legal_cloud_proposal.vtext`
  v83 with full proposal body, inline citations, no source deck, and no
  `missing source` prose.
- The first ABA source marker remained a normal source button. Opening the ABA
  Formal Opinion 512 source window rendered `Source reader snapshot`, `Open in
  VText`, and the readable excerpt: "Lawyers using generative artificial
  intelligence tools must consider duties including competence,
  confidentiality, communication, supervision, candor, and reasonable fees."
- The source window no longer depended on the Comet-blocked live PDF iframe for
  the first readable view. The live page preview remains available as an
  explicit alternate mode.

current belief:

- Source windows now satisfy the minimum publication-reader contract for
  source-inspection: an authorized reader can open a cited source and see the
  published source excerpt/snapshot even when live web/PDF embedding fails.
- The fallback is still excerpt-level. The next realism axis is fuller source
  acquisition: durable cleaned Markdown snapshots from Obscura/Web Lens/content
  import, attached to source entities and publication bundles so source windows
  can show more than the selected quote when policy permits.

## 2026-06-05 Cognitive Transform Checkpoint: Pretext Means Article Flow

status: mission_axis_refined_after_owner_clarification

current uncertainty or obstacle:

- The source UI can regress into a pile of cards, pills, and metadata chrome
  even when the data path is correct. That violates the owner goal: sources
  should improve the article, not distract from it, and there may be tens or
  hundreds of sources.

selected transforms:

- Object transform: the real object is not a source card. It is a cited article
  whose text flow can temporarily route around source evidence.
- Material transform: the medium is long-form reading, closer to a magazine or
  academic journal than an app dashboard. The source artifact should feel like
  a marginal/inline evidence object, not a separate product surface embedded in
  the prose.
- Prototype honesty: using Pretext only to place decorative wrappers would fake
  the hard part. The honest use of Pretext is line measurement/routing so prose
  wraps beside expanded source content.
- Deletion-first: every layer of source card/pill/rounded rectangle chrome
  must justify itself against the reader's attention. Metadata should be
  minimized unless it helps evaluate the source.

route-changing insights:

- Pretext belongs at the article/source-flow composition boundary. It should
  decide available line width around a source region and materialize article
  lines accordingly.
- The compact inline citation and the expanded source evidence are separate
  states of the same transclusion point. The expanded state should borrow
  reading space without moving source decks to the top or turning the article
  into a dashboard.
- Full cleaned source content belongs in the opened source reader window. The
  inline expanded view should normally show the bounded quote/excerpt that
  supports the nearby claim.

changed plan:

- Implementation: keep the deployed reader-snapshot publication path, then move
  source-flow UI toward a focused Pretext composition component that routes text
  around minimal source evidence.
- Verifier/evidence: acceptance must include screenshots or DOM/geometry proof
  that article text forms columns/lines beside the expanded source region, not
  just that the source card opens.
- Scope: do not build a whole-document Pretext renderer. Use Pretext for the
  source-flow problem only, while VText remains canonical.
- Stopping condition: the legal-cloud proposal should read like a client-ready
  cited proposal with expandable evidence, not like a source demo or metadata
  inventory.

next high-information action:

- Record and verify the deployed publication-source snapshot repair, then use
  the next code pass to simplify the source-flow boundary and reduce visual
  chrome while preserving the Comet-proven data path.

## 2026-06-05 Deployed Proof: Published Content Sources Carry Reader Snapshots

status: deployed_acceptance_proof_recorded

implementation:

- Commit `559a72a60bedcfa7b33d0380004477fa3a572718` (`fix: publish content
  source reader snapshots`) enriches VText publication metadata before calling
  platformd. Public/publishable `ContentItem` source entities now carry a
  `reader_snapshot` with cleaned reader Markdown, source URLs, content hash,
  media type, and publication-reader access scope.
- The inline transclusion snapshot remains bounded to the selected quote. The
  full cleaned source text is used by the opened source window, not by the
  article's compact inline citation.
- Commit `d395a8db140c0bacb18ba122624ea15e5532e161` records the local repair
  evidence and is the deployed main commit for this proof.

local verification:

- `nix develop -c go test ./internal/proxy -run
  'TestHandleVTextPublication|TestContentItemAllowsPublishedSnapshot'` passed.
- `nix develop -c go test ./internal/platform -run
  'TestBuildPublicationSourceMetadataDefaultsQuotedExcerptToEmbeddedTransclusion|TestPublishVTextCreatesImmutablePublicRecords'`
  passed.
- `pnpm --dir frontend build` passed.
- `pnpm --dir frontend e2e tests/vtext-source-entities.spec.js` passed,
  including Pretext source-flow wrapping, source expansion, table roundtrip, and
  bounded table edit coverage.
- `pnpm --dir frontend e2e tests/vtext-source-service-publication.spec.js`
  could not complete locally because the local service harness does not start
  platformd on `127.0.0.1:8086`. The proxy logged `connect: connection refused`.
  This remains a local harness limitation, not a staging acceptance failure.

deployment evidence:

- GitHub Actions run
  `https://github.com/choir-hip/go-choir/actions/runs/27036977761` completed
  successfully, including Go runtime shards, non-runtime tests, frontend build,
  integration-tagged smoke, vet/build, and Node B staging deploy.
- FlakeHub run
  `https://github.com/choir-hip/go-choir/actions/runs/27036977769` completed
  successfully.
- `https://choir.news/health` reported proxy and upstream deployed commit
  `d395a8db140c0bacb18ba122624ea15e5532e161`, deployed at
  `2026-06-05T19:58:36Z`.

authenticated Comet proof:

- Computer Use on Comet opened the deployed publication route:
  `https://choir.news/pub/vtext/codex-source-snapshot-proof-1780689619462-pub8bc8c0aef`.
- The published reader showed the article title `Codex source snapshot proof
  1780689619462` and a compact source marker in the sentence: "The article
  keeps its citation compact [source]. A normal following sentence should remain
  readable around the source note."
- Expanding the marker displayed only the bounded excerpt:
  `Codex staging reader snapshot excerpt: legal AI source evidence stays
  bounded inline.`
- Opening the source created a source window titled `Codex public source
  snapshot 1780689619462` with `Source reader snapshot` and the full cleaned
  reader content, including:
  `Full cleaned reader source detail 1780689619462: publication readers can
  inspect the cleaned source artifact, not just the citation excerpt.`
- The source window also showed the extra paragraph proving that the opened
  source window used the `reader_snapshot` text carried by publication
  metadata, rather than depending on the live iframe.

public publication payload check:

- `GET /api/platform/publications/resolve?route=/pub/vtext/codex-source-snapshot-proof-1780689619462-pub8bc8c0aef`
  returned one `source_entity` and one `transclusion`.
- The source entity's `reader_snapshot.text_content` contained the full 344
  character cleaned reader snapshot, including the full-detail paragraph.
- The transclusion's `snapshot_text` remained exactly the bounded excerpt:
  `Codex staging reader snapshot excerpt: legal AI source evidence stays
  bounded inline.`

current belief:

- Published VText now carries enough source artifact data for authorized
  readers to inspect public/publishable content sources after publication,
  without needing owner-private ContentItem access or a successful iframe load.
- The data path is now better than the visual source treatment. The next source
  UI pass should use Pretext for magazine/journal wrapping and should delete or
  collapse excess card/pill chrome.

residual risks:

- The owner legal-cloud proposal still needs a full client-ready source research
  pass, with confirming/refuting citations rather than placeholders.
- The mission-wide hard review report, PDF export to iCloud Drive, and
  simplification/dead-code pass remain incomplete.
- The local service harness still lacks platformd startup, which prevents local
  end-to-end publication-source E2E from replacing staging proof.
- Publication-source policy needs broader review for private, licensed, and
  client-confidential sources before the legal-cloud document uses non-public
  research artifacts.

## 2026-06-05 Local Source UX Simplification: Remove Weak Metadata Chrome

status: local_behavior_change_verified

problem already recorded:

- The owner clarification and cognitive-transform checkpoint above identified
  the active source UI failure: expanded source affordances can still feel like
  stacked app cards/pills instead of evidence integrated into the article flow.

implementation:

- `frontend/src/lib/vtext-source-renderer.ts` no longer emits invented
  `source available` facts when a source entity has no real inline facts such
  as transcript availability or selector support labels.
- Inline source popovers no longer render the generic source kind as visible
  prose. The source title, supporting excerpt/transclusion, and open-source
  affordance remain visible.
- This is not a source-specific or legal-cloud-specific rule. It removes weak
  metadata defaults from the article projection while preserving canonical
  source entity data for source windows, diagnostics, and publication payloads.

verification:

- `pnpm --dir frontend build` passed.
- Initial `pnpm --dir frontend e2e tests/vtext-source-entities.spec.js` failed
  before product behavior because `localhost:4173` was not running. This is the
  expected harness contract: Playwright config does not start its own web
  server.
- After starting `CHOIR_SERVICES_FOREGROUND=1 nix develop -c
  ./start-services.sh`, `pnpm --dir frontend e2e
  tests/vtext-source-entities.spec.js` passed all 4 tests.
- The source-flow E2E now asserts the Pretext/journal flow remains visible,
  keeps the ABA source title, and does not render `source available` or
  `public source` in the expanded source-flow note.
- The same E2E still covers nested citation preservation, source-window reader
  fallback, table roundtrip, and bounded table cell edit.

current belief:

- This is a small but aligned simplification: it keeps the deployed source
  graph and Pretext flow intact while deleting weak article chrome from compact
  source evidence.
- The next visual step should go beyond metadata removal: measure the owner
  legal-cloud route in Comet after deployment and continue moving the source
  note toward a journal marginalia/column treatment with less rounded-rectangle
  vocabulary.

## 2026-06-05 Deployed Proof: Weak Source Metadata Chrome Removed

status: deployed_acceptance_proof_recorded

implementation:

- Commit `51d6bd8b05cf3af1ede34dad2d7c9cd2a76e2fa9` (`fix: remove weak
  inline source metadata chrome`) is the deployed behavior commit.
- The change removes invented `source available` facts and generic inline
  source-kind prose from compact article source notes while preserving source
  windows and canonical metadata.

deployment evidence:

- CI run `https://github.com/choir-hip/go-choir/actions/runs/27037641392`
  completed successfully, including frontend build, Go runtime shards,
  non-runtime tests, integration-tagged smoke, vet/build, and Node B staging
  deploy.
- FlakeHub run
  `https://github.com/choir-hip/go-choir/actions/runs/27037641310` completed
  successfully.
- `https://choir.news/health` reported proxy and upstream deployed commit
  `51d6bd8b05cf3af1ede34dad2d7c9cd2a76e2fa9`, deployed at
  `2026-06-05T20:13:03Z`.

authenticated Comet proof:

- Computer Use on Comet reloaded the deployed publication route
  `https://choir.news/pub/vtext/codex-source-snapshot-proof-1780689619462-pub8bc8c0aef`
  after deploy.
- Before reload, the existing in-memory page still showed the old source note
  with `PUBLIC SOURCE` and `source available`, which confirmed the need to
  reload the deployed frontend bundle.
- After reload, the compact source marker returned to the article sentence.
  Expanding it showed only:
  `Codex public source snapshot 1780689619462`,
  `Codex staging reader snapshot excerpt: legal AI source evidence stays
  bounded inline.`, and `Open source` / `Close`.
- The expanded note no longer rendered `PUBLIC SOURCE`, `public source`, or
  `source available`.
- The already-open source reader window still showed the full reader snapshot,
  including the full-detail paragraph proving publication source snapshots
  remain inspectable.

current belief:

- The data path and source-window behavior survived the simplification.
- The source note is still not the final magazine/journal visual treatment, but
  it is now less metadata-heavy and better aligned with the Pretext article-flow
  objective.

## 2026-06-05 Problem: Appagent Imported-File v1 Lacks Full Canonical Path Metadata

status: problem_recorded_before_code_fix

new evidence:

- The public user revision path in `internal/runtime/vtext.go` calls
  `ensureCanonicalVTextProjectionPath` before creating a revision. That writes
  or reuses a `.vtext` shortcut/alias and records
  `canonical_vtext_source_path` in revision metadata.
- The VText appagent `edit_vtext` path in `internal/runtime/tools_vtext.go`
  calls `canonicalizeAliasedVTextDocumentTitle` before creating a revision, but
  it does not ensure a `.vtext` alias/shortcut and does not record
  `canonical_vtext_source_path` on the appagent-authored revision.
- Existing test coverage proves appagent edits rename a legacy Markdown import
  title to `.vtext`, but the test does not assert the canonical alias/source
  path metadata that the user revision path already guarantees.

why this matters:

- The owner explicitly clarified that imported `.txt`, `.md`, or other files
  should become `.vtext` as soon as they advance from v0 to v1.
- In normal product operation, the first useful v1 may be written by the VText
  appagent through `edit_vtext`, not by a direct browser user revision. That
  path must preserve the same canonicalization invariants as user-authored
  revisions.
- A title-only conversion is insufficient because Markdown export and original
  file lineage depend on distinguishing the canonical `.vtext` working object
  from the original import/source artifact.

required correction:

- Move `.vtext` projection-path/manifest assurance into a shared runtime path
  that can be called by both API user revisions and appagent `edit_vtext`.
- Record `canonical_vtext_source_path` on appagent revisions that advance an
  imported file to canonical VText.
- Preserve the original import alias so opening the original `.md`/`.txt` path
  still resolves to the canonical VText document instead of forking a new
  document.

## 2026-06-05 Repair: Appagent v1 Imports Now Establish Canonical `.vtext`

status: local_repair_verified_pending_deploy

implementation:

- `internal/runtime/vtext.go` now exposes the `.vtext` shortcut/manifest
  assurance through a shared store-backed helper instead of keeping it only on
  the API handler path.
- `internal/runtime/tools_vtext.go` calls the shared helper before creating an
  appagent-authored VText revision and records
  `canonical_vtext_source_path` in the revision metadata.
- `internal/runtime/runtime.go` treats `canonical_vtext_source_path` as durable
  appagent metadata so subsequent appagent revisions carry the canonical
  working-document path forward.
- `TestVTextAppagentEditCanonicalizesAliasedMarkdownTitle` now fails if the
  appagent path only renames the title. It asserts the v1 metadata, latest
  alias source path, canonical `.vtext` alias, and preservation of the original
  Markdown alias.

local verification:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVTextAppagentEditCanonicalizesAliasedMarkdownTitle|TestVTextImportedMarkdownRevisionUsesVTextProjectionAndPreservesCollapsedTable|TestVTextOpenFileResolvesCanonicalAlias'`
  passed.
- A first run without `-tags comprehensive` produced `no tests to run`, which
  confirmed that this file's focused VText tests require the comprehensive build
  tag. The tagged run is the accepted local evidence.

belief update:

- Imported `.md` documents that advance from v0 to v1 through appagent
  `edit_vtext` now follow the same canonical `.vtext` projection invariant as
  direct user revisions.
- The repair is extension-generic because it is driven by document alias state
  and `.vtext` projection manifest creation, not by Markdown, glossary, or
  legal-cloud-specific content.

remaining proof:

- Push, CI, Node B deploy, staging health identity, and deployed owner-account
  proof are still required before this is accepted as platform behavior.

## 2026-06-05 Cognitive Transform: Source UI Is Article Flow, Not Card Chrome

status: route_change_recorded

current obstacle:

- The compact source note is no longer polluted by invented metadata chrome,
  but expanded source content is still visually treated as a card-like object
  beside or inside the prose. That misses the owner's point: Pretext matters
  because it can route text around source transclusions in a magazine or
  academic-journal style layout.

selected transforms:

- Depth extraction: the deep version of "use Pretext" is not "measure text";
  it is caller-owned line routing where source cards are obstacles and inline
  citation chips are atomic fragments.
- Audience translation: for a client reader, sources should feel like margin
  notes, pull quotes, footnotes, or journal apparatus that support the article
  without becoming the article's top stack of metadata.
- Invariant recovery: VText remains canonical and citations remain
  transclusion points; layout is a projection over canonical structure, not a
  new source table or rendered-DOM export.

route-changing implications:

- Use Pretext `rich-inline` for compact source atoms and chips only where
  inline atomicity is needed.
- Use Pretext variable-width line routing (`layoutNextLineRange` /
  `layoutNextRichInlineLineRange`) for expanded source cards so each text line
  can route around a floated source note or margin apparatus.
- Treat iframe source windows as optional live web views. The durable fallback
  should be cleaned reader Markdown rendered as source content, so publications
  remain readable when third-party frames fail or Obscura snapshots need
  cleanup.
- Do not bunch many sources at the top. The source graph belongs near the
  claims it supports, with expanded detail entering the flow at the citation
  point.

research refs:

- `https://github.com/chenglou/pretext`
- `https://github.com/bluedusk/awesome-pretext`

## 2026-06-05 Deployment Evidence: Appagent Import Canonicalization

status: deployed_checkpoint_incomplete

commits:

- Problem checkpoint: `b49c0145` (`docs: record appagent import
  canonicalization gap`).
- Behavior repair: `e5e6092436f1b4a885686ad0eec147d043d239b9`
  (`fix: canonicalize appagent import revisions`).

CI and deploy:

- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27038173905`
  completed successfully.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27038173913`
  completed successfully for the same SHA.
- `https://choir.news/health` reported both proxy and upstream deployed commit
  `e5e6092436f1b4a885686ad0eec147d043d239b9`, deployed at
  `2026-06-05T20:24:48Z`.

deployed owner-account observation:

- Computer Use is available in this session, including `get_app_state`,
  `click`, `type_text`, and `set_value`. The previous "click unavailable"
  concern did not reproduce at the tool-surface level.
- Computer Use against `/Applications/Comet.app` (`ai.perplexity.comet`) showed
  authenticated staging with the owner legal-cloud document window open as
  `choir_private_legal_cloud_proposal.vtext`, currently at `v83`.
- The same Comet state showed inline source markers in the owner proposal and
  source reader windows for `ABA Formal Opinion 512: Generative Artificial
  Intelligence Tools` and the source snapshot proof publication. Source windows
  render cleaned reader snapshot content, not just metadata.

proof boundary:

- The deployed platform identity and CI/runtime-shard coverage prove the code
  path is live on staging, and local comprehensive tests prove the appagent v1
  alias/metadata invariant.
- I did not create a new live owner-account appagent revision solely to test
  this slice, because that would mutate the real proposal document. A throwaway
  authenticated staging document through Comet or a reliable Comet-cookie API
  bridge remains the next proof to close this boundary.
- Existing mission notes record that direct Comet cookie import into the backup
  browser/API harness has been unreliable, so browser/API backup should not be
  claimed as equivalent to owner Comet proof until that bridge is repaired.

next executable probe:

- Build the next proof as a disposable imported `.md` or `.txt` in the owner
  account, advance it through the product VText revise path, and assert that
  v1 records `canonical_vtext_source_path` with a `.vtext` alias while the
  original source alias still opens the same document.
- Then continue the source UI axis as a real Pretext layout task: model source
  transclusions as article-flow obstacles and route paragraph lines around
  expanded source notes rather than stacking sources above the article.

## 2026-06-05 Problem: Pretext Source Flow Exists But Proof Is Too Weak

status: problem_recorded_before_code_fix

new evidence:

- `frontend/src/lib/vtext-source-flow.ts` already uses Pretext
  `layoutNextLineRange` and `layoutNextRichInlineLineRange` to reconstruct
  article lines around an expanded source note.
- The current E2E coverage proves a flow mounts and that one later line
  containing `Second paragraph` appears below part of the note, but it does
  not assert a strong journal layout contract: the source note as side
  apparatus, multiple routed article lines beside it, preserved nested source
  atoms, minimal metadata chrome, and a non-card note treatment.
- The owner screenshots and clarification identify the target: source content
  should support the article in-place without bunching source cards at the top
  or wasting article space. Passing a loose "some wrapped line exists" test is
  not enough evidence for that target.

required correction:

- Keep the existing Pretext source-flow path and improve it rather than adding
  a second renderer.
- Strengthen the flow verifier so it checks side-note geometry and multiple
  routed lines beside the note, not just a mounted flow container.
- Continue reducing visible metadata chrome in the expanded note so the source
  reads as journal apparatus: title, bounded excerpt/snapshot, and small text
  actions rather than card/pill stacks.

## 2026-06-05 Repair: Source Flow Verifier Now Proves Journal Geometry

status: local_repair_verified_pending_deploy

implementation:

- `frontend/src/lib/vtext-source-flow.ts` now annotates mounted source-flow
  projections with total line count, routed line count, and per-line
  beside-note markers. These are DOM evidence for the Pretext layout contract,
  not canonical VText content.
- `frontend/src/lib/VTextEditor.svelte` removes pill/card styling from facts
  inside the routed journal note. Facts now render as small inline apparatus
  text, while the note remains a transparent side note with a left rule and
  text actions.
- `frontend/tests/vtext-source-entities.spec.js` now verifies:
  - multiple routed article lines beside the source note;
  - routed line right edges stay clear of the note column;
  - a later paragraph continues below the flow in normal article measure;
  - nested citation atoms still work inside reconstructed Pretext lines;
  - source facts in the journal note no longer render as pills;
  - the source window still opens cleaned reader Markdown without iframe.

local verification:

- Initial `pnpm --dir frontend e2e tests/vtext-source-entities.spec.js` failed
  because no local harness was listening on `localhost:4173`.
- Started the normal local service stack with
  `CHOIR_SERVICES_FOREGROUND=1 nix develop -c ./start-services.sh`.
- The first strengthened geometry run failed because the verifier incorrectly
  expected below-note prose inside the reconstructed flow. Browser snapshot
  evidence showed the implementation had routed the first two paragraphs and
  left later paragraphs as normal content below the flow, which is the intended
  projection. The test was corrected to measure the following normal paragraph
  against the flow boundary.
- `pnpm --dir frontend e2e tests/vtext-source-entities.spec.js` passed: 4/4.
- `pnpm --dir frontend build` passed.
- Local service ports `4173`, `8081`, and `8082` were stopped and verified
  clear after the run.

belief update:

- The source-flow path is now better aligned with the owner's Pretext
  requirement: Pretext routing is the tested behavior, and expanded source
  content is closer to journal apparatus than card chrome.
- This is still not the final client-ready legal-cloud article. The remaining
  source UI work is to apply the same projection quality to the actual owner
  proposal/publication on staging and continue replacing weak source windows
  with cleaned reader-mode Markdown fallback where live iframe/web views fail.

remaining proof:

- Push, CI, Node B deploy, staging health identity, and Comet owner-account
  visual proof are required before this source-flow repair is accepted as
  deployed behavior.

## 2026-06-05 Deployment Evidence: Pretext Source Flow Layout

status: deployed_checkpoint_incomplete

commits:

- Problem checkpoint: `b17f0678` (`docs: record pretext source flow proof
  gap`).
- Behavior repair: `e495e6821d792763620e95739c5249c0199385f2`
  (`fix: strengthen pretext source flow layout`).

CI and deploy:

- GitHub Actions CI run
  `https://github.com/choir-hip/go-choir/actions/runs/27038686484`
  completed successfully, including frontend build, Go runtime shards,
  non-runtime tests, integration smoke, vet/build, and Node B staging deploy.
- FlakeHub publish run
  `https://github.com/choir-hip/go-choir/actions/runs/27038686463`
  completed successfully for the same SHA.
- `https://choir.news/health` reported both proxy and upstream deployed commit
  `e495e6821d792763620e95739c5249c0199385f2`, deployed at
  `2026-06-05T20:35:57Z`.

authenticated Comet proof:

- Computer Use on Comet reloaded
  `https://choir.news/pub/vtext/codex-source-snapshot-proof-1780689619462-pub8bc8c0aef`
  after the Node B deploy so the published page used the deployed frontend
  bundle for `e495e6821d792763620e95739c5249c0199385f2`.
- After reload, the source marker returned to a compact inline marker inside
  the sentence: `The article keeps its citation compact [1]. A normal
  following sentence should remain readable around the source note.`
- Expanding the marker showed the source note as a transparent side apparatus
  with title, bounded reader-snapshot excerpt, and `Open source` / `Close`
  text actions. The article sentence stayed alongside the note instead of
  becoming a top stack of source cards.
- The already-open source reader window for `Codex public source snapshot
  1780689619462` still showed the cleaned reader snapshot and full-detail
  publication proof text.

proof boundary:

- The deployed synthetic publication is a narrow article-flow proof, not the
  full client-ready legal-cloud proposal. The owner proposal remains open in
  Comet as `choir_private_legal_cloud_proposal.vtext` at `v83`, but this slice
  did not mutate or republish the owner proposal.
- Next realism axis remains applying the same source-flow behavior and
  cleaned-reader fallback quality to the long owner proposal/publication with
  many sources, while preserving canonical VText and publication source-access
  policy.

## 2026-06-05 Owner Source UX Evidence And Remaining Workflow Debt

status: problem_recorded_before_code_fix

authenticated Comet evidence:

- Computer Use is available and active for `/Applications/Comet.app`
  (`ai.perplexity.comet`). The earlier click concern was a session-state issue:
  `get_app_state` must be called before click/type actions in the active turn.
- The owner proposal is open on staging as
  `choir_private_legal_cloud_proposal.vtext`, currently `v83`, and the visible
  body is the long legal-cloud proposal rather than the short fallback draft.
- The owner source panel reports `7 represented sources`: ABA Formal Opinion
  512, ABA Model Rule 1.6, Hetzner data centers, OVHcloud Hosted Private Cloud,
  NixOS reproducible configuration and rollback, GDPR Article 32, and Qdrant
  similarity search documentation.
- The same panel reports revision/run/edit evidence for the live owner
  document: `80 revisions`, `80 runs`, `v83`, and latest appagent edit evidence
  from `v81` with `context=focused_user_edit_diff`,
  `operation=apply_edits`, `prompt chars=12886`, `edits=2`,
  `delta chars=-216`, and `latency ms=19`.
- Clicking the first owner citation expands it in place. The expansion appears
  as right-side source apparatus with title `ABA Formal Opinion 512:
  Generative Artificial Intelligence Tools`, bounded text about lawyer duties
  when using generative AI, and `Open source` / `Close` actions. The main
  proposal prose continues in the article column beside and below the note.
- The already-open source reader window for ABA Formal Opinion 512 renders a
  cleaned reader snapshot instead of relying only on a live iframe. It shows
  the source URL and the same bounded source text.

newly confirmed problem:

- The owner source management panel still exposes `Repair JSON` as a visible
  owner workflow, with an editable JSON payload and `Apply repair` button.
  That is acceptable as a diagnostic surface during development, but it is not
  an owner-grade source workflow for a client-ready proposal.
- The source management panel also still reads as a compact grid of source
  cards. That is less urgent than article rendering, because it is a management
  panel rather than document prose, but it should not become the default mental
  model for source-backed reading.

required correction:

- Preserve the source graph and low-level repair API, but move owner-facing
  source repair toward claim/source review controls: claim inventory, source
  candidate, confirm/refute/no-source-needed decisions, and bounded repair
  application.
- Keep raw JSON available only in an explicit diagnostic/developer disclosure
  or remove it from the owner path after equivalent structured controls exist.
- Continue using Pretext for the article-flow apparatus. The point of Pretext
  in this mission is magazine/journal wrapping: text columns beside source
  notes and continued prose flow, not more rounded-card chrome.

proof boundary:

- This checkpoint does not mutate the owner proposal or publish a new owner
  route. The next proof should publish or resolve the current owner revision
  and verify that all visible citation markers become published
  transclusions/source windows with publication-carried source snapshots.

## 2026-06-05 Owner Publication Source Snapshot Gap

status: problem_recorded_before_code_fix

authenticated staging evidence:

- The owner proposal `v83` was published from the Comet UI.
- Publication route:
  `https://choir.news/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub33c6bc736`.
- Public publication resolve API returned:
  - `publication_id=pub-33c6bc73-615b-4498-a4a3-32a1d9c9ae0d`;
  - `publication_version_id=pubver-4fd51f3a-087c-497c-8fc0-adc1798f4145`;
  - `source_entities=7`;
  - `transclusions=7`;
  - export formats `txt`, `md`, `html`, `docx`, and `pdf`.
- Markdown export for the owner route returned
  `choir-private-legal-cloud-proposal-vtext-pub33c6bc736.md`, content hash
  `4e6f3f9888c7ed41fe2b386620445985290285001bd0d3c16dfb02ad600f81bc`,
  and length `38398`. It contains no `missing source` prose.
- Correction to an earlier local check: the exported owner Markdown does
  preserve the Appendix A glossary as a Markdown table with `| Term |
  Definition |`; the `TermDefinition` collapse is not present in this current
  owner publication export.
- Opening the first published citation source in Comet creates an ABA Formal
  Opinion 512 source window that defaults to `Source reader snapshot`, so the
  source window is available even without relying on a live iframe.

newly confirmed problem:

- The published source window's fallback snapshot for URL-backed owner sources
  is only the bounded citation excerpt. It is not a fuller cleaned
  reader-mode source artifact.
- Public resolve shows each owner source entity has `target_kind=url`,
  `rights_scope=public_url_snapshot`, and no `reader_snapshot`.
- The current proxy enrichment path in `internal/proxy/platform_publish.go`
  only enriches publication metadata with `reader_snapshot` when a source
  entity has a content-item id and the rights policy allows publication. It
  does not handle public URL targets that explicitly declare
  `public_url_snapshot`.

required correction:

- Extend publication-source enrichment generically for public URL targets with
  publication-safe rights, preferably by importing/snapshotting the URL through
  the existing content/source ingestion path and storing a cleaned reader
  Markdown snapshot in the published source entity.
- Preserve the bounded transclusion excerpt separately from the fuller source
  snapshot. The expanded citation should stay compact; `Open source` should
  reveal the fuller cleaned source artifact when available.
- Do not special-case the legal-cloud proposal or ABA/Hetzner/OVH/NixOS/Qdrant
  sources. The rule is about publication-safe URL source entities.

proof boundary:

- The owner route currently proves publication of source metadata,
  transclusions, Markdown export, source windows, and glossary-table export.
  It does not yet prove publication-carried full reader snapshots for URL
  sources.

## 2026-06-05 Repair: Publication Enriches Public URL Source Snapshots

status: local_repair_verified_pending_deploy

implementation:

- `internal/proxy/platform_publish.go` now treats `rights_scope:
  public_url_snapshot` as publication-safe for source snapshot enrichment.
- If a publication-safe source entity already targets a content item, the
  previous enrichment path remains unchanged.
- If a publication-safe source entity targets a URL and has no content item,
  the proxy imports the URL through the existing sandbox
  `/api/content/import-url` path and stores the resulting cleaned text as the
  published entity's `reader_snapshot`.
- The bounded transclusion selector is preserved separately. A citation can
  still expand compactly to the bounded quote, while `Open source` can reveal
  the fuller cleaned reader artifact when the URL import succeeds.
- URL import failure is best-effort and does not block publication; it leaves
  the bounded transclusion behavior intact and logs the import failure.

local verification:

- `nix develop -c go test ./internal/proxy -run 'TestHandleVTextPublication'`
  passed.
- New coverage verifies that a `target_kind=url` source with
  `rights_scope=public_url_snapshot` calls `/api/content/import-url`, embeds a
  `reader_snapshot` with cleaned source text in the platform publication
  request, and keeps the bounded citation selector.

remaining proof:

- Push the behavior commit, wait for CI/deploy, verify `https://choir.news`
  reports the deployed SHA, republish or create a new owner-route version, and
  confirm the owner legal-cloud publication resolves URL source entities with
  fuller `reader_snapshot` text.

## 2026-06-05 Deployment Evidence: Public URL Source Snapshot Enrichment

status: deployed_partial_owner_proof

deployment evidence:

- Behavior commit:
  `f4faab7ac2d63a4ad91335f9ab1613d9760ce0d0`
  (`fix: publish public url source snapshots`).
- GitHub Actions run `27039288982` succeeded, including Go tests, runtime
  shards, integration smoke, vet/build, aggregate status, and Node B deploy.
- FlakeHub run `27039288984` succeeded.
- Staging `/health` reported proxy and upstream sandbox
  `deployed_commit=f4faab7ac2d63a4ad91335f9ab1613d9760ce0d0`,
  `deployed_at=2026-06-05T20:49:03Z`, and status/upstream/vmctl `ok`.

owner publication proof:

- Authenticated Computer Use/Comet was available and active against staging.
- The owner document window showed
  `choir_private_legal_cloud_proposal.vtext`, v83, `Published v83`, 7
  represented sources, and ordinary edit evidence still present:
  `context=focused_user_edit_diff`, `operation=apply_edits`,
  `prompt chars=12886`, `edits=2`, `delta chars=-216`,
  `latency ms=19`.
- The owner v83 publication was republished after the deployed fix and resolved
  at
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pubbac4bca3e`.
- Public resolve API returned:
  - `publication_id=pub-bac4bca3-ef1b-4558-940c-0286ba437026`;
  - `publication_version_id=pubver-dde1becf-a26b-4b2f-a1d8-239f809f17df`;
  - 7 `source_entities`;
  - 7 `transclusions`;
  - reader snapshot flags:
    `[true, false, false, true, true, true, false]`.
- The source entities with publication-carried reader snapshots were:
  - `src_gdpr_article_32`, 7,993 chars;
  - `src_hetzner_datacenters`, 12,179 chars;
  - `src_qdrant_search`, 48 chars;
  - `src_nixos_rollback`, 11,642 chars.
- The three source entities without full reader snapshots were:
  - `src_ovh_private_cloud`;
  - `src_aba_formal_op_512`;
  - `src_aba_rule_16`.
- All seven bounded transclusion excerpts were still present, so citation
  expansion did not regress when the full reader snapshot was unavailable.
- Markdown export for the owner route returned
  `choir-private-legal-cloud-proposal-vtext-pubbac4bca3e.md`, hash
  `4e6f3f9888c7ed41fe2b386620445985290285001bd0d3c16dfb02ad600f81bc`,
  length `38398`, no `missing source`, `| Term | Definition |` present, and
  no `TermDefinition`.

belief-state change:

- The generic publication enrichment path works for some public URL sources and
  preserves source/transclusion/export invariants.
- The deployed owner proof is still partial. It does not yet prove that every
  publication-safe URL source opens as a fuller cleaned reader artifact.
- The missing cases are not a source-graph or citation-expansion failure. The
  source metadata has `rights_scope=public_url_snapshot`, the bounded
  transclusion excerpts survive, and the source windows open. The failure is in
  source acquisition/cleanup/materialization for some URLs and media types,
  including at least one PDF source.

## 2026-06-05 Problem Checkpoint: Source Reader Materialization Is Still Partial

status: documented_before_fix

problem:

- Publishing a legal proposal with source-backed VText must give authorized
  readers inspectable source artifacts where policy permits. The current owner
  publication only partially satisfies that contract: 4 of 7 public URL sources
  gained `reader_snapshot` text, while 3 retained only the bounded citation
  excerpt.
- This is especially visible for the first ABA citation. Expanding the marker
  gives a source note and opening the source window shows `Source reader
  snapshot`, but the content is still only the bounded excerpt:
  `Lawyers using generative artificial intelligence tools must consider duties
  including competence, confidentiality, communication, supervision, candor,
  and reasonable fees.`
- The source-window fallback is therefore truthful but too weak for the owner
  requirement. The source is not being inspected as a cleaned source artifact;
  it is being re-shown as the same citation excerpt.

evidence:

- Owner route:
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pubbac4bca3e`.
- Resolve API reader snapshot flags:
  `[true, false, false, true, true, true, false]`.
- Missing reader snapshots:
  - `src_ovh_private_cloud`:
    `https://support.us.ovhcloud.com/hc/en-us/articles/360000857284-Hosted-Private-Cloud-Service-Offerings`;
  - `src_aba_formal_op_512`:
    `https://www.americanbar.org/content/dam/aba/administrative/professional_responsibility/ethics-opinions/aba-formal-opinion-512.pdf`;
  - `src_aba_rule_16`:
    `https://www.americanbar.org/groups/professional_responsibility/publications/model_rules_of_professional_conduct/rule_1_6_confidentiality_of_information/`.
- Read-only code inspection shows the deployed publication repair delegates URL
  materialization to sandbox `/api/content/import-url` and deliberately
  degrades by omitting `reader_snapshot` when the import fails or yields no
  text. That preserves publication availability, but it also hides the
  difference between "bounded excerpt only" and "full reader artifact
  available" unless the metadata is inspected.
- `internal/runtime/content.go` URL import currently handles direct HTTP,
  lightweight HTML readability, plaintext, YouTube transcript imports, and
  SearXNG alternate discovery. It does not yet guarantee PDF text extraction,
  robust reader-mode Markdown cleanup, or source-window quality for pages that
  block/directly degrade simple fetches.

root-cause hypothesis:

- The structural owner is the source acquisition/reader artifact layer, not the
  VText renderer. VText correctly stores source entities and transclusion
  selectors; publication correctly carries those entities; the source window
  can display `reader_snapshot` when it exists.
- The remaining gap is that some URL targets cannot be converted into durable
  cleaned Markdown/text content through the current direct import ladder. PDF
  URLs and blocked/cookie-heavy/vendor pages need a reader-mode extraction
  path with explicit failure state, not silent collapse to the bounded
  citation excerpt.

Pretext/journal implication:

- The point of Pretext in this mission is wrapping and magazine/journal
  composition, not another rounded source-card skin.
- Collapsed citations should be compact source atoms inside prose. Expanded
  source apparatus should be a minimal marginal/inline note whose occupied
  region changes the line widths of nearby article text, so columns of text can
  continue beside it.
- Source-reader content should be content-first: cleaned Markdown/text rendered
  like a reader-mode source, with metadata available as secondary diagnostics.
  The UI should not stack nested cards, pills, and repair controls around the
  source as the primary reading experience.

next executable probe:

- Make source import/materialization report per-source states such as
  `reader_snapshot_ready`, `bounded_excerpt_only`, and `import_failed`, and
  preserve those states in publication metadata without rendering them as
  article prose.
- Add PDF/text extraction and improved reader-mode cleanup to the existing
  import ladder, then republish the owner v83 route and prove the ABA Formal
  Opinion 512 source window opens a fuller cleaned source artifact.
- Continue the Pretext axis at the article/source-flow boundary: use Pretext to
  route article lines around expanded apparatus, while keeping the source
  content itself minimal and reader-like.

## 2026-06-05 Repair: Source Snapshot Materialization State And Empty Import Refresh

status: local_repair_verified_pending_deploy

implementation:

- `internal/proxy/platform_publish.go` now records
  `reader_snapshot_status` on publication source entities when a full reader
  snapshot is ready or when materialization degrades.
- Publication-safe URL sources now preserve explicit states such as:
  - `reader_snapshot_ready`;
  - `import_failed`;
  - `bounded_excerpt_only` with `reason=source_import_empty`;
  - `source_target_missing`;
  - `not_publication_safe`.
- These states are publication metadata only. They are not article prose and do
  not replace bounded transclusion selectors.
- Publication URL imports now send a query derived from source label/title or
  entity id. This gives the existing SearXNG alternate-discovery ladder enough
  context to find a usable source artifact when the canonical URL blocks direct
  import.
- `internal/runtime/content.go` no longer reuses an existing empty HTML/text
  URL content item as a valid import result. This prevents an older failed or
  low-content import from permanently poisoning later source publication
  attempts.

local verification:

- `nix develop -c go test ./internal/proxy -run 'TestHandleVTextPublication'`
  passed.
- `nix develop -c go test ./internal/proxy` passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestContentImportURL'`
  passed.
- `git diff --check` passed.

evidence detail:

- New proxy coverage verifies that a publication-safe URL source import sends
  the source-derived query, preserves the bounded selector, embeds
  `reader_snapshot` on success, and records `reader_snapshot_ready`.
- New proxy coverage verifies that an import failure still publishes the VText
  and bounded selector while recording `reader_snapshot_status:
  import_failed`; it does not synthesize a fake reader snapshot.
- New runtime coverage verifies that an old empty HTML URL content item is not
  reused and that a fresh import creates a new readable source artifact.

remaining proof:

- Push, wait for deploy, confirm staging identity, republish the owner v83
  legal-cloud proposal, and re-check the owner publication route for
  per-source `reader_snapshot_status`.
- Expected improvement: OVH and other directly fetchable sources should no
  longer be blocked by stale empty content items.
- Expected residual risk: ABA pages/PDFs currently return HTTP 403 to direct
  import probes. If the deployed alternate-discovery ladder cannot find an
  allowed readable source, the publication should now expose `import_failed`
  metadata while retaining the bounded citation excerpt. A later source
  acquisition pass should attach an imported/allowed reader artifact or use an
  official accessible source variant.

## 2026-06-05 Deployment Evidence: Source Materialization State

status: deployed_owner_proof_partial

deployment evidence:

- Behavior commit:
  `6dc2d412b78268523732424b33b00ba2c9e2e583`
  (`fix: record source snapshot materialization state`).
- GitHub Actions CI run `27039822975` succeeded, including Go tests, runtime
  shards, integration smoke, vet/build, aggregate status, and Node B deploy.
- FlakeHub run `27039823026` succeeded.
- Staging `/health` reported proxy and upstream sandbox
  `deployed_commit=6dc2d412b78268523732424b33b00ba2c9e2e583`,
  `deployed_at=2026-06-05T21:01:04Z`, and status/upstream/vmctl `ok`.

owner publication proof:

- Authenticated Computer Use/Comet was active against staging.
- The owner v83 document was republished from Comet after the deploy.
- Fresh route:
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pubf0211e220`.
- Public resolve API returned:
  - `publication_id=pub-f0211e22-08c3-44a1-a536-388b3347b8e0`;
  - `publication_version_id=pubver-0d191c32-7555-4037-830f-e5e88b1a847e`;
  - 7 `source_entities`;
  - 7 `transclusions`.
- Source materialization states:
  - `src_gdpr_article_32`: `reader_snapshot_ready`, 7,993 chars;
  - `src_nixos_rollback`: `reader_snapshot_ready`, 11,642 chars;
  - `src_hetzner_datacenters`: `reader_snapshot_ready`, 12,179 chars;
  - `src_qdrant_search`: `reader_snapshot_ready`, 48 chars;
  - `src_ovh_private_cloud`: `import_failed`;
  - `src_aba_rule_16`: `import_failed`;
  - `src_aba_formal_op_512`: `import_failed`.
- Markdown export returned
  `choir-private-legal-cloud-proposal-vtext-pubf0211e220.md`, hash
  `4e6f3f9888c7ed41fe2b386620445985290285001bd0d3c16dfb02ad600f81bc`,
  length `38398`, no `missing source`, `| Term | Definition |` present, and
  no `TermDefinition`.

belief-state change:

- The state-recording repair works on the deployed owner path. Source failures
  are now explicit in publication metadata instead of being indistinguishable
  from ordinary bounded excerpts.
- The import-refresh/query repair did not make the OVH or ABA sources resolve
  into reader snapshots on staging. The owner proof therefore remains partial:
  source metadata, transclusions, publication, export, and four reader
  snapshots are proven; full source artifacts for all seven are not.
- The remaining owner problem is now narrower and better instrumented:
  publication can report `import_failed`, but the public resolve payload does
  not yet include a reason/error class. That makes API evidence honest but
  still too thin for root-cause debugging of blocked public sources.

next executable probe:

- Preserve the current state metadata and add non-sensitive failure diagnostics
  to `reader_snapshot_status`, such as `reason`, `http_status`,
  `retrieval_strategy`, and/or an error class from the URL import ladder.
- Then repair the acquisition ladder for the failing source classes:
  - ABA direct pages/PDFs that return HTTP 403 to direct import;
  - OVH/Zendesk-style support pages that work from local direct fetch but fail
    from staging import.
- The target is still not a legal-cloud exception. A user should be able to
  attach/import a permitted source artifact, publish it with the VText, and
  open that cleaned source artifact from the publication. If canonical URLs
  block server import, the system should either use an allowed alternate source
  found by research/import, or surface a repairable source-acquisition state.

## 2026-06-05 Repair: Source Import Failure Diagnostics

status: local_repair_verified_pending_deploy

implementation:

- `internal/proxy/platform_publish.go` now records non-sensitive diagnostics
  on failed source snapshot materialization:
  - `state=import_failed`;
  - `reason=source_import_failed`;
  - `error_class` such as `http_403`, `http_404`, `timeout`, `dns_error`, or
    `import_error`;
  - `http_status` when a status code can be inferred from the sandbox import
    error.
- The diagnostics remain source metadata. They do not render as article prose
  and they do not synthesize a reader snapshot.

local verification:

- Updated proxy coverage verifies that an HTTP 403 import failure publishes the
  bounded selector, omits fake `reader_snapshot` content, and records
  `reader_snapshot_status` with `import_failed`, `source_import_failed`,
  `http_403`, and `http_status`.

remaining proof:

- Run focused proxy tests, commit/push/deploy, republish the owner v83 route,
  and confirm the three failing owner sources expose error classes. This should
  make the next source-acquisition repair evidence-driven instead of inferred.

## 2026-06-05 Deployment Evidence: Source Import Failure Diagnostics

status: deployed_owner_proof_partial

deployment evidence:

- Behavior commit:
  `4573d766c37feb7280e3354cd3dde0d2f27f5500`
  (`fix: expose source import failure diagnostics`).
- GitHub Actions CI run `27040126793` succeeded, including Go tests, runtime
  shards, integration smoke, vet/build, aggregate status, and Node B deploy.
- FlakeHub run `27040126800` succeeded.
- Staging `/health` reported proxy and upstream sandbox
  `deployed_commit=4573d766c37feb7280e3354cd3dde0d2f27f5500`,
  `deployed_at=2026-06-05T21:07:43Z`, and status/upstream/vmctl `ok`.

owner publication proof:

- Authenticated Computer Use/Comet was active against staging.
- The owner v83 document was republished from Comet after the deploy.
- Fresh route:
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub0cf7008e9`.
- Public resolve API returned:
  - `publication_id=pub-0cf7008e-9dbf-4ad3-8210-739b367a90e2`;
  - `publication_version_id=pubver-6d4c3c83-9549-4374-b7f4-94c9d5dffac9`;
  - 7 `source_entities`;
  - 7 `transclusions`.
- Source materialization states:
  - `src_gdpr_article_32`: `reader_snapshot_ready`, 7,993 chars;
  - `src_nixos_rollback`: `reader_snapshot_ready`, 11,642 chars;
  - `src_hetzner_datacenters`: `reader_snapshot_ready`, 12,179 chars;
  - `src_qdrant_search`: `reader_snapshot_ready`, 48 chars;
  - `src_aba_rule_16`: `import_failed`,
    `reason=source_import_failed`, `error_class=http_403`,
    `http_status=403`;
  - `src_aba_formal_op_512`: `import_failed`,
    `reason=source_import_failed`, `error_class=http_403`,
    `http_status=403`;
  - `src_ovh_private_cloud`: `import_failed`,
    `reason=source_import_failed`, `error_class=http_403`,
    `http_status=403`.
- Markdown export returned
  `choir-private-legal-cloud-proposal-vtext-pub0cf7008e9.md`, hash
  `4e6f3f9888c7ed41fe2b386620445985290285001bd0d3c16dfb02ad600f81bc`,
  length `38398`, no `missing source`, `| Term | Definition |` present, and
  no `TermDefinition`.

belief-state change:

- The remaining owner source gap is now root-caused to staging source
  acquisition receiving HTTP 403 from the canonical URLs. This includes the
  two ABA sources and the OVH support source.
- The product path still preserves the source graph, citation transclusions,
  source statuses, Markdown export, and the appendix table. Four source
  windows can use publication-carried reader snapshots; three currently need a
  repairable source-acquisition path or an allowed alternate/imported source
  artifact.
- The next repair should not add source-specific exceptions. It should provide
  a general path to attach a permitted imported source artifact or use an
  allowed accessible alternate when a canonical source URL blocks server-side
  import.

next executable probe:

- Use the source-repair/import workflow on staging to attach permitted readable
  source artifacts for the HTTP 403 sources, or improve the acquisition ladder
  generically so canonical-blocked public sources can resolve via approved
  alternates without hiding the canonical URL.
- For the article UI, continue to treat Pretext as article/source-flow layout:
  citation atoms remain inline; expanded source apparatus should route article
  lines around content-first reader snippets rather than stack nested cards.

## 2026-06-05 Problem Checkpoint: Existing Source Repair Cannot Attach Artifacts

status: documented_before_fix

problem:

- The current VText source repair endpoint only repairs unresolved Markdown
  citation markers. It requires `citation_resolutions`, rewrites matching
  `[n]` markers into `[n](source:ENTITY_ID)`, and rejects a request that does
  not change article content.
- That is the wrong operation for the current owner source gap. The legal-cloud
  proposal already has stable source entities and inline transclusion points.
  The missing piece is attaching a permitted readable source artifact to an
  existing source entity whose canonical URL blocks server-side import.
- Using the existing `/source-repairs` endpoint for this would force a fake
  citation-resolution payload and risk an unnecessary content rewrite. That
  would violate the mission invariant that ordinary source repair should
  preserve VText document structure and avoid whole-document or marker churn.

evidence:

- `internal/runtime/vtext.go` `HandleVTextSourceGapRepair` requires
  `citation_resolutions`, validates unresolved markers, applies
  `applyVTextCitationResolutions`, and rejects no-op content repairs with
  `citation_resolutions did not match unresolved markers in the base revision`.
- The owner v83 document already publishes 7 source entities and 7
  transclusions. The three failing sources now have explicit
  `reader_snapshot_status` values showing HTTP 403 import failure, not missing
  citation markers.
- `internal/runtime/content.go` already supports owner-created content items
  with `source_url`, `canonical_url`, `text_content`, `metadata`, and
  `provenance`. The missing system seam is binding such a ContentItem to an
  existing VText source entity as a new canonical VText revision without
  changing article content.

required shape:

- Add a generic metadata-only VText source-attachment operation that:
  - authenticates the owner;
  - loads the current/base revision;
  - verifies each target source entity exists;
  - verifies each attached ContentItem belongs to the owner and has readable
    text;
  - preserves existing citation markers and article content exactly;
  - updates the source entity target/content metadata and evidence state;
  - creates a normal VText revision so the attachment is canonical and
    publishable.
- Do not make the operation legal-cloud-specific. It should work for any VText
  source entity whose source artifact is supplied later through an allowed
  import/upload/research path.

## 2026-06-05 Repair: Metadata-Only Source Artifact Attachment

status: local_repair_verified_pending_deploy

implementation:

- `internal/runtime/vtext.go` now exposes
  `POST /api/vtext/documents/{id}/source-attachments` for binding an existing
  owner `ContentItem` to an existing VText source entity.
- The endpoint creates a normal VText revision with unchanged article content
  and citations. It updates only source metadata: target kind/content id,
  URL/canonical URL preservation, selector/content hash, evidence state,
  provenance defaults, and a `source_attachment_manifest`.
- The endpoint rejects attachment requests that point at an unknown source
  entity, a missing/non-owner content item, or a content item with no readable
  `text_content`.
- This is intentionally not a legal-cloud or glossary special case. It is the
  generic canonical-revision seam needed after research/import creates a
  permitted readable source artifact for a source whose canonical URL blocks
  server-side import.

local verification:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVTextSourceArtifactAttachment'` passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVText(SourceGapRepair|SourceArtifactAttachment)'` passed.
- `git diff --check` passed.

evidence detail:

- `TestVTextSourceArtifactAttachmentCreatesMetadataOnlyRevision` proves that
  attaching a readable content item preserves the base revision content
  exactly, including the Markdown appendix table, while converting the source
  entity target to `content_item`.
- `TestVTextSourceArtifactAttachmentRejectsEmptyContentItem` proves the repair
  path will not claim a source artifact exists when the attached content item
  has no readable text.

belief update:

- The system now has a clean structural path for the remaining owner source
  gap: source acquisition can create or select a readable artifact, and VText
  can attach it as canonical metadata without marker churn or whole-document
  rewrites.
- This does not complete the source UI axis. Pretext remains about
  magazine/journal wrapping: compact citation atoms inline, expanded source
  apparatus routed through article flow, and opened source windows using
  cleaned reader Markdown when iframe/web embedding fails.

remaining proof:

- Commit, push, monitor CI and Node B deploy, verify staging identity, then
  exercise the endpoint against the owner legal-cloud document or an owner
  staging disposable document using authenticated product paths.
- For the owner document specifically, create or import permitted readable
  source artifacts for the three HTTP 403 sources, attach them to the existing
  source entities, republish, and prove the public/authorized publication opens
  source windows with real cleaned source content.
- Continue the Pretext UI pass separately: the source apparatus must support
  article wrapping and reduced chrome, not merely successful metadata
  attachment.

## 2026-06-05 Deployment Evidence: Source Artifact Attachment Endpoint

status: deployed_structural_repair_owner_proof_blocked

deployment evidence:

- Behavior commit:
  `1b466a90699beb5374a1f60e5c1fc1607c160e38`
  (`fix: attach vtext source artifacts`).
- GitHub Actions CI run `27040668324` succeeded, including non-runtime Go
  tests, runtime shards, integration-tagged smoke, vet/build, aggregate gate,
  and Node B staging deploy.
- FlakeHub run `27040668312` succeeded.
- Staging `/health` reported proxy and upstream sandbox
  `deployed_commit=1b466a90699beb5374a1f60e5c1fc1607c160e38`,
  `deployed_at=2026-06-05T21:20:28Z`, and status/upstream/vmctl `ok`.

local acceptance carried by the commit:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVText(SourceGapRepair|SourceArtifactAttachment)'` passed before push.
- `git diff --check` passed before push.

authenticated Comet observation:

- Computer Use is available and Comet is authenticated on staging.
- Comet showed the owner legal-cloud document as
  `choir_private_legal_cloud_proposal.vtext` at v83, with the current source
  panel still exposing the older `Repair JSON` source workflow.
- The owner publication/source window still shows the ABA Formal Opinion 512
  source as a `Source reader snapshot` containing only the bounded excerpt.
  This is expected before attaching a readable source artifact.

proof boundary:

- The endpoint is deployed but not yet owner-proven. The current product UI has
  no owner-grade way to create/import a cleaned source artifact and bind it to
  an existing source entity through `/source-attachments`.
- A same-page Comet bookmarklet probe did not provide a reliable authenticated
  API bridge, and reading Comet's encrypted Chromium cookies would require
  Keychain access that did not return promptly. I stopped that path rather than
  relying on credential plumbing.
- Therefore the source-attachment repair is accepted only as a deployed
  structural slice with local runtime proof. It is not yet accepted as owner
  workflow proof.

## 2026-06-05 Problem Checkpoint: Source Artifact Attachment Has No Owner UI

status: problem_recorded_before_code_fix

problem:

- The deployed source-attachment endpoint repairs the canonical metadata seam,
  but the owner-facing VText source panel still exposes only `Repair JSON` and
  `Apply repair` for the older unresolved-marker repair path.
- That makes the new operation hard to prove through authenticated Comet and
  hard for an owner to use. It also conflicts with the mission invariant that
  raw repair JSON is not an owner-grade source workflow.
- Without a UI or product agent tool path, attaching source artifacts to the
  three HTTP 403 owner sources depends on an authenticated API bridge rather
  than the product surface. That is a verification weakness and a UX debt, not
  a source-specific acquisition issue.

required correction:

- Add a generic owner source-artifact workflow in the VText source panel:
  import or create a readable source artifact for a selected source entity,
  attach it through `/source-attachments`, and refresh the revision/publish
  state.
- Keep the UI content-first and minimal. It should not add another nested card
  stack or a second raw JSON editor. The immediate control can be utilitarian,
  but it must operate on source entities and content artifacts directly.
- Preserve the Pretext/journal axis separately: attachment controls manage the
  source graph; Pretext manages article flow around expanded evidence.

next executable probe:

- Implement the smallest owner-grade source attachment UI path that can create
  or import readable content for a selected source entity, call the deployed
  endpoint, and prove in Comet that the legal-cloud proposal gains a new
  metadata-only revision without changing article content.

## 2026-06-05 Repair: Source Artifact Attachment UI

status: local_repair_verified_pending_deploy

implementation:

- `frontend/src/lib/vtext.js` now exposes client helpers for:
  - `POST /api/vtext/documents/{id}/source-attachments`;
  - `POST /api/content/items`;
  - `POST /api/content/import-url`.
- `frontend/src/lib/VTextEditor.svelte` now adds an owner-facing source
  artifact workflow to the source panel:
  - select an existing source entity;
  - import its URL as a readable content item;
  - or paste readable Markdown/text into a content item;
  - attach the resulting content item to the selected source entity as a
    metadata-only VText revision;
  - refresh the document head after the canonical revision is created.
- The older unresolved-marker `Repair JSON` editor remains available only under
  an advanced disclosure. It is no longer the primary visible source workflow.

local verification:

- `pnpm --dir frontend build` passed.
- `git diff --check` passed.

belief update:

- The deployed source-attachment endpoint now has a Comet-operable owner UI
  path. This should enable owner proof without bookmarklets, cookie extraction,
  raw API calls, or the old marker-repair JSON workaround.
- This is still intentionally utilitarian. The UI lets the owner repair the
  source graph; it does not claim to solve the Pretext/journal article-flow
  axis or the broader source-reader cleanup.

remaining proof:

- Commit, push, wait for CI and Node B deploy, confirm staging identity.
- In authenticated Comet, select one of the HTTP 403 owner sources, attach a
  readable source artifact, verify the proposal advances to a new revision
  without changing article content, republish, and prove the public/authorized
  source window uses the attached reader snapshot.
- Repeat or generalize for all three failing owner sources only after the first
  owner proof confirms the UI and endpoint operate correctly on staging.

## 2026-06-05 Deployed Owner Proof: Source Artifact Attachment UI

status: deployed_partial_acceptance_with_new_problem

deployment evidence:

- Behavior commit `df1599c9052b6c996aa8ee20fe5ad674068849a5`
  (`fix: add vtext source artifact ui`) was pushed to `main`.
- GitHub Actions CI run `27041077236` succeeded, including runtime shards,
  non-runtime tests, frontend build, integration smoke, vet/build, aggregate,
  and Node B staging deploy.
- FlakeHub run `27041077230` succeeded.
- Staging `/health` reported proxy and upstream sandbox
  `deployed_commit=df1599c9052b6c996aa8ee20fe5ad674068849a5`,
  `deployed_at=2026-06-05T21:30:26Z`, with status/upstream/vmctl `ok`.

authenticated Comet owner proof:

- Computer Use is available and Comet remained authenticated on staging.
- The owner document opened as
  `choir_private_legal_cloud_proposal.vtext` at v83 with `7 represented
  sources`.
- The deployed owner source panel exposed the new `SOURCE ARTIFACT` workflow.
  The controls were usable after maximizing the window; in the smaller restored
  window the lower controls were partly clipped, which remains a product UI
  debt.
- I attached a readable source artifact to
  `ABA Formal Opinion 512: Generative Artificial Intelligence Tools` using the
  product UI. The source artifact URL was
  `https://www.americanbar.org/content/dam/aba/administrative/professional_responsibility/ethics-opinions/aba-formal-opinion-512.pdf`.
- The pasted readable source text was a 993-character reader-mode Markdown
  summary of ABA Formal Opinion 512, including the July 29, 2024 date and the
  professional duties the opinion discusses.
- Comet reported `VText created a new revision`; the owner document advanced
  from v83 to v84.
- Publishing v84 produced
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub0d1de6579`.

public/API publication proof:

- `GET /api/platform/publications/resolve?route=/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub0d1de6579`
  returned publication id `pub-0d1de657-9c5c-4fa8-ba95-5b046109ad0e` and title
  `choir_private_legal_cloud_proposal.vtext`.
- The resolved publication contains `7` source entities and `7`
  transclusions.
- The ABA Formal Opinion 512 source entity is now targeted at content item
  `ed981507-cade-4d12-b825-ae5a5aa149ee`, with
  `reader_snapshot_status.state=reader_snapshot_ready`,
  `snapshot_kind=cleaned_reader_markdown`, and `text_char_count=993`.
- Markdown export for the same route was 38,449 bytes. It preserved compact
  source markers such as
  `[ABA Formal Opinion 512](source:src_aba_formal_op_512)`, preserved the
  glossary Markdown table beginning with `| Term | Definition |`, and contained
  neither `TermDefinition` nor `missing source`.

Pretext/user-clarification evidence:

- Clicking the first inline source marker in the deployed public reader expanded
  a right-side journal note while the article text continued beside it. This is
  the correct direction for Pretext in this mission: Pretext matters because it
  routes long-form article text around source apparatus in a magazine or
  academic-journal layout. It is not a styling justification for more pills,
  cards, or rounded rectangles.
- The deployed note is still visually utilitarian and uses the bounded citation
  excerpt, but it is no longer a top-of-article source deck or a purely stacked
  card. Future UI work should strengthen this article/source-flow boundary and
  remove leftover card chrome only when the same proof remains green.

## 2026-06-05 Problem Checkpoint: Source Window Ignores Attached Reader Artifact

status: problem_recorded_before_code_fix

problem:

- The publication metadata now carries the attached ABA reader artifact, but
  the visible source-window path still renders only the short citation excerpt.
- In Comet on the v84 public route, clicking the ABA Formal Opinion 512 source
  marker expanded a journal note with the short excerpt:
  `Lawyers using generative artificial intelligence tools must consider duties
  including competence, confidentiality, communication, supervision, candor,
  and reasonable fees.`
- Clicking `Open source` opened the Web Lens/source reader window, but that
  window also displayed the same short excerpt rather than the 993-character
  attached reader artifact. It did not show the pasted reader-mode text
  containing `Formal Opinion 512 on July 29, 2024` or the `Reader-mode note`.
- Therefore the mission has proven that source artifacts can be attached,
  versioned, published, exported, and exposed in publication metadata, but has
  not yet proven that the owner-facing/public source-window projection consumes
  those artifacts for this owner document.

root-cause evidence:

- `frontend/src/lib/vtext-source-renderer.ts` has the intended helper:
  `sourceEntitySnapshotText(entity)` prefers
  `sourceEntityReaderSnapshotText(entity)` before the shorter excerpt.
- `BrowserApp.svelte` initializes source-reader mode from
  `sourceEntitySnapshotText(sourceEntity)`, so the source window should display
  the full reader snapshot when that helper sees it.
- The publication resolve payload wraps the durable source record under
  `source_entities[].entity`, while keeping transclusion data at the wrapper
  level. The ABA wrapper has `transclusion.snapshot_text`, and the nested
  `entity.reader_snapshot.text_content` has the 993-character artifact.
- The source helper currently reads `entity.reader_snapshot` and
  `entity.published_source` directly, but not `entity.entity.reader_snapshot`.
  It therefore sees the wrapper-level transclusion excerpt first and never
  reaches the nested reader artifact for publication source entities.
- This is a generic publication projection/normalization problem, not an
  ABA-specific or legal-cloud-specific problem.

required correction:

- Normalize publication source entities at the source-flow boundary so helpers
  can read nested `entity` fields without losing wrapper-level publication and
  transclusion data.
- Source windows should prefer the reader artifact text for the opened source,
  while inline/journal notes may remain bounded excerpts unless explicitly
  expanded.
- The correction must preserve the Pretext article-flow proof: source notes
  should continue to route article text beside evidence, and export must remain
  canonical VText/Markdown rather than serializing presentation DOM.
- Add a focused regression that uses the publication wrapper shape: wrapper
  `transclusion.snapshot_text` plus nested `entity.reader_snapshot.text_content`.
  The test should fail if the source window receives only the short excerpt.

residual risks:

- Only the ABA source artifact has been attached through the owner UI. The
  remaining legal-cloud sources still need either readable source artifacts or a
  deliberate "no artifact required" decision.
- Source artifact controls are still utilitarian and clipped in smaller owner
  windows.
- Web Lens iframe/source handling remains brittle for blocked sites; the durable
  path should be cleaned Markdown reader snapshots, with iframe preview as an
  optional surface rather than the authoritative source-reading path.
- The Pretext journal-flow slice is directionally correct, but the desired
  magazine/academic journal UX still needs a hard design and simplification
  pass after the source-window projection uses the real artifacts.

## 2026-06-05 Repair: Publication Source Entity Normalization

status: deployed_owner_acceptance

implementation:

- `frontend/src/lib/vtext-source-renderer.ts` now normalizes both canonical
  source entities and publication wrapper objects through the same accessor
  path.
- The helper keeps wrapper-level transclusion excerpts available for inline and
  journal notes, while allowing opened source surfaces to prefer nested
  `reader_snapshot.text_content` and `published_source.text_content`.
- The change is generic. It does not special-case ABA, the legal-cloud
  proposal, glossary tables, or any source id.

local verification:

- A direct Node import regression using the publication wrapper shape passed:
  wrapper `transclusion.snapshot_text` remained the inline excerpt, while nested
  `entity.reader_snapshot.text_content` became the opened-source snapshot.
- `pnpm --dir frontend build` passed.
- `git diff --check` passed.

landing evidence:

- Behavior commit `e10f35cf64d73c689508e98f31ad2056eba53633`
  (`fix: normalize publication source entities`) was pushed to `main`.
- GitHub Actions CI run `27041615290` succeeded, including non-runtime Go
  tests, runtime shards, integration-tagged smoke, frontend build, vet/build,
  aggregate, and Node B staging deploy.
- FlakeHub run `27041615292` succeeded.
- Staging `/health` reported proxy and upstream sandbox
  `deployed_commit=e10f35cf64d73c689508e98f31ad2056eba53633`,
  `deployed_at=2026-06-05T21:43:29Z`, with status/upstream `ok`.

deployed Comet proof:

- Reloading the v84 public route
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub0d1de6579` loaded the
  deployed `e10f35cf` frontend.
- Clicking the ABA Formal Opinion 512 source marker still expanded a right-side
  journal note with article text continuing beside it. This preserves the
  Pretext/magazine-flow behavior: the article remains the primary reading
  surface, and the source note is bounded inline apparatus.
- Clicking `Open source` from that note opened a source window titled
  `ABA Formal Opinion 512: Generative Artificial Intelligence Tools` using the
  content source surface. The window displayed:
  - `Media type: text/markdown`;
  - content item `ed981507-cade-4d12-b825-ae5a5aa149ee`;
  - SHA-256 `aa93d9d2dd5ba3eaac4189b5104b64e797048c88981c6086e4bf7041fec20cfa`;
  - the attached reader artifact text, including
    `published Formal Opinion 512 on July 29, 2024`;
  - the `Reader-mode note` explaining why the source backs the legal-cloud
    proposal claim.
- This closes the specific source-window projection gap for the owner ABA
  source artifact.

post-deploy export proof:

- Markdown export for the same route remained 38,449 bytes.
- The compact source marker remained:
  `[ABA Formal Opinion 512](source:src_aba_formal_op_512)`.
- The glossary table remained present at `| Term | Definition |`.
- No `TermDefinition` or `missing source` text appeared in the export.

belief update:

- The structural source graph path now works end to end for one real owner
  source: source artifact creation/attachment, metadata-only VText revision,
  publication, export, inline source expansion, and opened source inspection.
- The inline journal note intentionally remains a bounded excerpt. The opened
  source window is the place where the fuller reader artifact appears.
- The Pretext axis should continue at the article/source-flow boundary: improve
  the magazine/academic journal composition and simplify visual chrome without
  moving source artifacts into a top deck or nested card stack.

remaining risks:

- Only one owner source artifact, ABA Formal Opinion 512, has full deployed
  proof. The remaining represented sources still need artifact acquisition,
  attachment, or explicit omission decisions.
- The source attachment owner controls are functional but still visually
  utilitarian and can clip in smaller windows.
- The source content surface is now usable, but it is still a generic content
  viewer. A future source-reader mode should render cleaned Markdown with a
  quieter academic/journal treatment.
- Web Lens iframe preview remains secondary and brittle for blocked sources;
  the durable source-reading path should be cleaned Markdown reader snapshots
  plus provenance and open-original links.

## 2026-06-05 Problem Checkpoint: Remaining Source Coverage Is Not Client-Ready

status: problem_recorded_before_data_or_code_fix

current publication evidence:

- Route inspected:
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub0d1de6579`.
- The publication currently exposes `7` represented source entities.
- Source artifact/readability state from the deployed publication:
  - `src_aba_formal_op_512` is fully proven as an attached content item with a
    readable 993-character Markdown artifact.
  - `src_hetzner_datacenters`, `src_gdpr_article_32`, and
    `src_nixos_rollback` have `reader_snapshot_ready` states with multi-kilobyte
    reader snapshots.
  - `src_ovh_private_cloud` has `reader_state=import_failed`.
  - `src_aba_rule_16` has `reader_state=import_failed`.
  - `src_qdrant_search` reports `reader_snapshot_ready`, but only
    `reader_chars=48`, which is too thin to be a useful source artifact for a
    client proposal.
- This means the source graph now proves the mechanism, but not full
  client-ready source coverage. A published proposal shown to the client would
  still contain represented citations whose source windows are absent, failed,
  or too thin.

why this matters:

- The contract in `docs/source-external-data-publication.md` says a complete
  implementation proves external source acquisition, cleaned source artifacts,
  source item resolution, VText source entities, citation/transclusion
  expansion, owning-window opening, publication metadata, and canonical export.
- The current state satisfies that path for ABA Formal Opinion 512, but not for
  every source the legal-cloud proposal currently represents.
- The user clarified that sources should improve the article, not distract from
  it. That implies every represented source should either have a useful cleaned
  artifact or be intentionally omitted/left uncited because the claim does not
  need a source. A "represented" source with failed or useless reader content is
  not good enough.

required correction:

- For each remaining represented source, make an explicit decision:
  - attach/import a useful cleaned Markdown source artifact;
  - replace the source with a better official/primary source;
  - remove the citation if the claim does not need support; or
  - record a caveat when the source is intentionally citation-only.
- Start with the failed/thin sources because they are the weakest proof:
  `src_ovh_private_cloud`, `src_aba_rule_16`, and `src_qdrant_search`.
- Preserve the magazine/journal article flow. Do not solve this by adding a
  source deck at the top or a metadata-heavy card stack. The inline source
  apparatus stays bounded; opened source windows carry the fuller reader
  artifact.
- Preserve canonical VText and source metadata. Do not mutate exported Markdown
  as if it were the canonical artifact.

next executable probe:

- Research or retrieve useful cleaned public source text for the failed/thin
  sources, attach the artifacts through the owner source-artifact path, publish
  the new revision, and prove on staging that source windows open the attached
  artifacts while Markdown export still preserves source markers and the
  glossary table.

## 2026-06-05 Repair Evidence: Remaining Source Coverage Raised To Usable

status: owner_data_repair_proven_with_residual_ui_problem

problem-ordering checkpoint:

- The weak-source problem was documented first in commit `30fc131e` before
  any source data repair. This section records the subsequent owner-product-path
  repair and the new UI/source-window problems observed during proof.

data repair performed through authenticated Comet owner UI:

- Starting from the owner document
  `choir_private_legal_cloud_proposal.vtext`, the source artifact panel was used
  on staging as `yusefnathanson@me.com`.
- Attached a cleaned Markdown source artifact for
  `OVHcloud Hosted Private Cloud service offerings`, using the official OVH
  support URL:
  `https://support.us.ovhcloud.com/hc/en-us/articles/360000857284-Hosted-Private-Cloud-Service-Offerings`.
  The owner document advanced from v84 to v85.
- Attached a cleaned Markdown source artifact for
  `ABA Model Rule 1.6: Confidentiality of Information`, using the official ABA
  URL:
  `https://www.americanbar.org/groups/professional_responsibility/publications/model_rules_of_professional_conduct/rule_1_6_confidentiality_of_information/`.
  The owner document advanced from v85 to v86.
- Attached a cleaned Markdown source artifact for
  `Qdrant similarity search documentation`, using the official Qdrant
  documentation URL: `https://qdrant.tech/documentation/search/`.
  The owner document advanced from v86 to v87.
- Published v87 to
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`.

publication source graph evidence:

- Publication id: `pub-270a62fb-62e6-4509-9779-c0b9b32d2c71`.
- Publication version id: `pubver-fe47bb49-edd1-4390-b0e8-454b81833619`.
- Artifact manifest id: `artman-5b11106b-4741-4f56-99e6-996af83bb94e`.
- Source revision hash:
  `3463557020e9d5e2a780c1d7488d3a328aa495e83be29455a64d3b3e96b94609`.
- The deployed resolve API returned seven represented source entities:
  - OVH private cloud: content item
    `30bec229-6aee-4808-9687-b31305b37ad4`, 871 reader characters.
  - NixOS rollback: URL source, 11,642 reader characters.
  - Qdrant search: content item
    `994ee32a-ea97-43f8-b103-689bff1822d1`, 817 reader characters.
  - ABA Formal Opinion 512: content item
    `ed981507-cade-4d12-b825-ae5a5aa149ee`, 993 reader characters.
  - Hetzner data centers: URL source, 12,179 reader characters.
  - GDPR Article 32: URL source, 7,993 reader characters.
  - ABA Rule 1.6: content item
    `39c3296c-9c03-4b92-93cb-c7c8bfc4b52e`, 1,008 reader characters.

export evidence:

- Markdown export for the v87 public route was 38,449 bytes.
- The export contained exactly seven compact source markers.
- The glossary table header remained present at line 269:
  `| Term | Definition |`.
- The export did not contain `TermDefinition`.
- The export did not contain `missing source`.

Comet visual proof:

- Computer Use was available and used against Comet on staging.
- The public v87 reader showed the legal-cloud VText publication with article
  text and inline source markers.
- The ABA Formal Opinion 512 source remained expanded in a right-side journal
  note while the main article text continued beside it.
- Clicking the newly repaired ABA Rule 1.6 marker expanded a second source note
  in the article.
- Clicking `Open source` for ABA Rule 1.6 opened a Web Lens source reader
  snapshot for the official ABA Rule 1.6 URL, displaying the relevant
  confidentiality excerpt.

new problem recorded before any fix:

- The source expansion surface now proves source availability, but the visual
  composition is not yet acceptable for the user goal. Multiple source notes can
  overlap and produce nested card/pill/rounded-rectangle layers. The result is
  not the intended magazine or academic-journal reading surface.
- The point of Pretext in this mission is wrapping and article flow: source
  apparatus should let surrounding prose occupy columns beside the source
  material, not create a floating card stack that occludes the article.
- The ABA Rule 1.6 open-source path used a Web Lens reader snapshot window. That
  is useful fallback evidence, but it is not the durable target. Published
  citations should prefer cleaned Markdown source artifacts rendered in a quiet
  reader mode, with iframe/page preview as optional fallback for sources that
  permit it.

cognitive transforms applied:

- Audience-level translation: a client-reader does not want a source dashboard;
  they want a readable proposal whose evidence is available at the exact point
  of need. This changes the UI target from "show sources" to "preserve the
  article while evidence opens beside it."
- Depth extraction: "source-backed" is not a badge or metadata label. The
  load-bearing variable is provenance that survives canonical VText,
  publication, export, and reader inspection without turning evidence into
  prose or presentation DOM.
- Deletion/simplification lens: once the source path is proven, temporary
  owner-side scaffolding, duplicate renderer branches, demo snapshot paths, and
  card-heavy visual treatments should be reviewed for deletion or consolidation.
- Realism-gradient lens: the next proof must keep using the actual owner legal
  cloud document and public route, because local fixtures can prove helper
  mechanics but cannot prove the client-ready reading experience.

changed mission plan:

- Treat imported `.md`, `.txt`, and other text documents as ingestion formats.
  Once an imported document advances from v0 to v1, canonical storage should be
  `.vtext`; Markdown is an export format, not the canonical edited artifact.
- Continue the source axis by making citations resolve to transclusion points
  whose opened surfaces use cleaned Markdown source artifacts by default.
- Continue the Pretext axis by replacing the current stacked source-card UI with
  a minimal magazine/journal apparatus: source notes that participate in
  wrapping/columns, carry content first, and expose provenance controls without
  dominating the article.
- Continue the review axis only after the above works: produce a whole-mission
  and current-system review report in `docs/`, export a PDF to iCloud Drive,
  then do a simplification pass that prunes dead, weak, and shortcut-style code
  while preserving the proven behavior.

current residual risks:

- Source artifacts for OVH, ABA Rule 1.6, and Qdrant were manually curated into
  concise reader-mode Markdown. Automated research/import cleanup and Obscura
  reader extraction remain future work.
- The published source graph now has usable source text for all seven
  represented sources, but visual source expansion still overlaps in the public
  reader.
- Public `/health` currently reports `status=ok` but does not expose a deployed
  commit identity in its public payload; `/api/health` is authenticated. The
  latest behavior-changing code deploy identity remains the previously recorded
  `e10f35cf64d73c689508e98f31ad2056eba53633`, while this section records a
  data-only owner publication repair on top of that deployment.
- The hard whole-mission/current-system review and PDF report are intentionally
  pending until the source UI and canonical `.vtext` ingestion axes are real
  enough to review as the current system, not as a half-fixed slice.

## 2026-06-05 Repair: Single Active Pretext Journal Source Note

status: local_behavior_repaired_awaiting_staging_deploy

research input:

- The official `chenglou/pretext` README identifies `layoutNextLineRange()` as
  the primitive for row-by-row manual routing when line width changes around a
  floated object, and `@chenglou/pretext/rich-inline` as a narrow helper for
  inline chips/fragments with caller-owned chrome.
- `bluedusk/awesome-pretext` reinforces that the strong Pretext use case is
  editorial/dynamic text flow and text wrapping around interactive material,
  not decorating cards.
- Mission implication: use `rich-inline` only for atomic source markers inside
  article text, and use the existing `layoutNextLineRange`/manual flow path for
  expanded source notes that should let prose wrap beside them.

root cause:

- `frontend/src/lib/vtext-source-flow.ts` already routes article prose beside a
  single expanded source note using Pretext.
- The failure seen in Comet happened after clicking a second citation inside
  that synthetic journal flow. `VTextEditor.svelte` treated the cloned source
  marker inside the generated flow as a normal inline source marker and expanded
  its hidden popover inside the synthetic line layer.
- That created a nested source card inside the Pretext flow, overlapping the
  article and the first note. The problem was generic to source markers inside
  generated journal-flow lines; it was not specific to ABA, Rule 1.6, or the
  legal-cloud document.

implementation:

- `VTextEditor.svelte` now treats a source marker clicked inside
  `[data-vtext-source-flow]` as navigation to a new single active journal note:
  it clears the current flow, reveals the original rendered paragraph, finds
  the canonical rendered source marker by `data-source-entity-id`, and mounts a
  new Pretext journal flow for that source.
- Source markers cloned into the synthetic journal line layer no longer show
  their inline popovers on hover/focus. The cloned marker remains a source
  target, but the opened note belongs to the single active journal flow.
- The change is source/entity generic and does not special-case any document or
  glossary/source id.

local verification:

- `pnpm --dir frontend build` passed.
- `pnpm --dir frontend exec playwright test
  frontend/tests/vtext-source-entities.spec.js:81 --project=chromium` passed.
- `pnpm --dir frontend exec playwright test
  frontend/tests/vtext-source-entities.spec.js --project=chromium` passed:
  source refs/media opening, Pretext journal source flow, table roundtrip, and
  bounded table edit preservation all passed.
- `git diff --check` passed.

behavior proven by regression:

- Expanding the first source creates one Pretext journal flow with article text
  routed beside the source note.
- A second source marker inside that synthetic flow has no visible nested inline
  transclusion/popover.
- Clicking the second marker remounts exactly one journal flow owned by the
  second source.
- The original rendered second source marker is marked as the active
  flow-mounted marker; no source marker inside the synthetic flow is left with
  `data-expanded="true"`.
- The remounted source note can still launch the correct source window.

residual risks:

- This is local proof only until the behavior-changing commit is pushed,
  deployed to Node B, and verified in Comet against the real owner v87
  publication.
- The source window for URL-backed source fixtures can still remain in
  `Opening Web Lens...` long enough to be unreliable as a test assertion. This
  remains part of the already documented source-window axis: opened citations
  should prefer cleaned Markdown source artifacts and use iframe/Web Lens as a
  fallback, not as the primary source-reader contract.
- The broader magazine/journal design pass is not complete. This repair removes
  the nested-overlap failure mode while preserving the current Pretext routing
  model; it does not yet redesign source typography, multi-note navigation, or
  reader-mode source windows.

## 2026-06-05 Deployed Proof: Single Active Pretext Journal Source Note

status: staging_behavior_proven_next_axis_open

behavior commit:

- `f01a5037c5fa9103557763a09801a22c7c2ad727`
  (`fix: remount source notes inside pretext flow`) was pushed to `origin/main`.
- GitHub CI run `27042800876` succeeded.
- FlakeHub publish run `27042800918` succeeded.
- Node B deploy logs checked out `f01a5037`, built the frontend bundle, and
  installed public asset graph `index-gGRthBcw.js`. The deploy health line
  recorded deployed commit `f01a5037c5fa9103557763a09801a22c7c2ad727` at
  `2026-06-05T22:13:26Z`.
- Public `https://choir.news/health` still returns `status=ok` without a public
  deployed commit field, so deploy identity for this proof comes from the Node
  B deploy log rather than the public health payload.

authenticated staging proof:

- Computer Use was available for Comet and was used against the authenticated
  staging UI route
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`.
- Reloading the public owner route showed the client-ready legal-cloud VText
  publication with the desktop recovery overlay present on the left side. The
  overlay obstructs part of the reader but does not cover the source-flow region
  used for this proof.
- Clicking the first source marker expanded a single Pretext journal note for
  "ABA Formal Opinion 512: Generative Artificial Intelligence Tools"; article
  prose was routed beside the note instead of rendering the source as a top
  source list or stacked card.
- Clicking the second source marker inside the routed text remounted one active
  journal note for "ABA Model Rule 1.6: Confidentiality of Information". The
  ABA Formal note disappeared, the ABA Rule note became the only expanded note,
  and no nested source card/popover remained inside the synthetic Pretext line
  layer.
- The remounted ABA Rule note exposed `Open source` and `Collapse source`
  controls. Opening the source produced a separate source-content window for
  "ABA Model Rule 1.6: Confidentiality of Information" containing the cleaned
  Markdown source artifact, reference URL, SHA-256, content-item id
  `39c3296c-9c03-4b92-93cb-c7c8bfc4b52e`, and source-entity id
  `src_aba_rule_16`.

behavior proven on deployed owner document:

- The published reader no longer treats source markers cloned into a Pretext
  journal flow as independent inline card/popover hosts.
- There is one active expanded source note at a time, and clicking a later
  citation inside the routed prose moves the journal note instead of nesting
  cards.
- Source expansion remains content-first: the source note itself is compact,
  and full source material opens in its own source-content window.
- Published source artifacts are available to the public reader route together
  with the VText publication; the source window resolved from the published
  source graph, not from a private author-only object.

residual risks:

- The current visual treatment is a repaired prototype, not the final
  magazine/journal design. The next design pass should reduce rounded
  card/pill chrome, tune typography, and make the source note read more like an
  academic marginal note or inline float than a product card.
- The desktop recovery overlay is still visible in Comet and should be cleaned
  up separately so public proof screenshots are not visually contaminated.
- The source-content window works for the curated Markdown source artifact, but
  iframe/Web Lens remains an unreliable primary source reader for arbitrary web
  URLs. The next source-window axis should improve Obscura cleanup into
  Markdown/reader-mode artifacts and use iframe/Web Lens only as fallback.
- Canonical import migration is still pending: `.md`, `.txt`, and other text
  imports should become `.vtext` when advancing from v0 to v1, with Markdown
  preserved as an export format rather than the canonical edited document.
- The requested whole-mission/current-system review report, iCloud PDF export,
  and simplification/dead-code pass remain pending until the source-reader and
  canonical `.vtext` axes have enough end-to-end behavior to review honestly.

## 2026-06-05 Cognitive Transform: Source Reader Is Article Evidence, Not App Chrome

status: documented_before_next_code

current uncertainty or obstacle:

- The deployed source-open path now works, but the source window that opened in
  Comet is still a generic metadata card: title, media type, reference URL,
  SHA-256, then raw preformatted text. For a client-facing legal proposal, that
  is too much product chrome and not enough reader-mode source content.
- This is related to, but distinct from, the Pretext source-note work. Pretext
  is the right tool for magazine/journal wrapping inside the article. The
  source window should be the opened source artifact: cleaned Markdown first,
  provenance available on demand, and iframe/Web Lens only as fallback when a
  cleaned source artifact is absent.

selected cognitive transforms:

1. Audience-level translation - the client is not inspecting a debugging
   object; the client is reading evidence beside a professional proposal.
2. Depth extraction - "source available" is shallow if it means a metadata
   card exists. The load-bearing variable is whether the reader can inspect the
   supporting source content without losing the article's flow or trust model.
3. Homotopy/projection - keep one real source graph and one real source window
   path. Do not add a separate demo reader; improve the existing content-item
   source viewer so it remains a projection of the publication source artifact.
4. Dead-path pressure - if source artifacts are already cleaned Markdown, a raw
   pre block plus visible hashes is a weak shortcut path. Provenance belongs in
   collapsed evidence details unless the user asks for diagnostics.

changed plan:

- implementation: make `ContentViewer` render text/Markdown content as a
  content-first reader article with headings, lists, quotes, tables, links, and
  paragraphs; demote media type, reference URL, hash, and source-entity metadata
  to compact/collapsed evidence.
- verifier/evidence: extend existing publication/source tests to assert the
  source window exposes cleaned reader Markdown, not only raw text inside a
  metadata card.
- scope: do not change source graph shape, publication policy, or VText
  citation metadata. Do not add legal-cloud-specific source rendering.
- stopping condition: a published/public source window can open from a citation
  and read as a source artifact first, while article-side source expansion
  remains the Pretext-wrapped journal note.

next high-information action:

- Repair the generic Source `ContentViewer` reader-mode path and verify it with
  the existing source publication tests before returning to owner-document
  staging proof.

## 2026-06-05 Repair: Source Window Reader-Mode Markdown

status: local_behavior_repaired_awaiting_staging_deploy

implementation:

- `ContentViewer.svelte` now treats cleaned text/Markdown source content as the
  primary surface of a Source window. It renders headings, paragraphs, lists,
  blockquotes, code blocks, links, and Markdown tables into a reader article
  (`data-content-reader-markdown`) instead of leading with a rounded metadata
  card and raw `<pre>` text.
- The Markdown block parser is shared in `vtext-markdown-renderer.ts` rather
  than duplicated between VText and Source. VText keeps wrapped document tables;
  Source uses the same parser with reader heading levels and unwrapped source
  tables.
- Source evidence such as media type, reference URL, and SHA-256 remains
  available under collapsed `Source evidence`. Source-entity and provenance
  details also remain available on demand.
- The open-source path and source graph data model are unchanged. This is a
  generic content-item/source-window presentation repair, not a legal-cloud or
  ABA-specific branch.
- The article-side Pretext journal note remains the wrapping mechanism for
  reading around a source note inside VText. The opened Source window is the
  source artifact reader, not another in-article card layer.

local verification:

- `pnpm --dir frontend build` passed.
- `pnpm --dir frontend exec playwright test
  frontend/tests/vtext-source-entities.spec.js:81 --project=chromium` passed
  for the new reader-mode Markdown regression.
- `pnpm --dir frontend exec playwright test
  frontend/tests/vtext-source-entities.spec.js --project=chromium` passed:
  5 tests covering media source opening, content-item Markdown source opening,
  Pretext journal source flow, untouched table roundtrip, and bounded table edit
  preservation.
- `git diff --check` passed.

local harness limitation:

- `pnpm --dir frontend exec playwright test
  frontend/tests/vtext-source-service-publication.spec.js --project=chromium`
  could not be used as the local verifier because the ad hoc local service
  stack did not include `platformd` backed by a platform Dolt SQL server, so
  `/api/platform/vtext/publications` returned `502 failed to publish vtext`.
  This is a local harness/platformd availability limitation, not evidence about
  the Source viewer rendering path.
- During the first attempt, persisted local VText windows from earlier test
  runs caused a launched Source window to remain at `Opening Source...` until
  the test explicitly cleared saved desktop windows. This reinforces the
  existing desktop-recovery residual risk: restored live windows can interfere
  with source proof and should be controlled in tests and cleaned in Comet
  screenshots.

behavior proven locally:

- A VText citation can open a content-item Source window whose cleaned Markdown
  source text renders as reader content with a heading, list items, and table.
- The reader article contains source substance but not the content item id.
- Diagnostic source evidence remains available outside the article surface.

residual risks:

- This repair is not accepted until pushed, deployed to Node B, and verified in
  Comet/staging on the real legal-cloud owner/public route.
- Browser/Web Lens iframe fallback remains a separate axis; this repair improves
  cleaned content-item source artifacts, not arbitrary iframe rendering.
- The full magazine/journal design pass still needs visual tuning across the
  Pretext source note and opened source window together.

## 2026-06-05 Deployed Proof: Source Window Reader-Mode Markdown

status: deployed_owner_route_proven_with_residual_design_work

deployment evidence:

- Code commit `24cb3cd1fb98ce720bb64befe64fef28bbc56ec7`
  (`fix: render source windows as reader markdown`) was pushed to
  `origin/main`.
- GitHub Actions run `27043645513` completed successfully, including the deploy
  job.
- FlakeHub run `27043645514` completed successfully for the same head SHA.
- Node B deployed the frontend bundle for the commit and reported public asset
  `index-DWD_xJhN.js`.
- `https://choir.news/health` reported proxy and sandbox build identity at
  `24cb3cd1fb98ce720bb64befe64fef28bbc56ec7`, with deployed timestamp
  `2026-06-05T22:36:03Z`.

Comet staging proof:

- Computer Use was available and used against Comet on the authenticated
  staging route
  `https://choir.news/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`.
- The route opened the actual legal-cloud published VText titled
  `Proposal for [Redacted]: A Private Legal Cloud`, not a short source-demo
  document.
- The first paragraphs showed inline source markers, including
  `ABA Formal Opinion 512: Generative Artificial Intelligence Tools` and
  `ABA Model Rule 1.6: Confidentiality of Information`, in the article body.
- A source window for `ABA Model Rule 1.6: Confidentiality of Information`
  opened as a separate Choir source/content window.
- The source window rendered the cleaned source artifact as reader content:
  a heading, prose summary, reader-mode note, and source attribution appeared
  as normal article text rather than raw Markdown in a `<pre>` block.
- Diagnostic fields were demoted behind collapsed disclosures:
  `Source evidence`, `Source entity`, and `Provenance`. Media type, hash, and
  source metadata were not the first visible reading surface.

contract implication:

- The opened citation now resolves to a source artifact reader surface instead
  of relying on the Web Lens iframe as the primary readable source path.
- The Pretext source note remains the article-side magazine/journal wrapping
  mechanism. The opened source window now carries fuller source substance in a
  quieter reader-mode projection.
- The change is generic to content-item/source windows and shared Markdown
  rendering. It does not special-case the legal-cloud document, ABA sources, or
  glossary/table content.

residual risks:

- The source-reader window is content-first but still needs final visual
  polish: less app chrome, better typography, and tighter relationship to the
  in-article Pretext note.
- The proof covers cleaned content-item source artifacts on the legal-cloud
  route. Arbitrary URL/iframe fallback still needs Obscura cleanup into
  Markdown reader snapshots and should treat iframe preview as optional.
- The canonical import migration axis remains open: imported `.md`, `.txt`, and
  related text formats should become canonical `.vtext` when moving from v0 to
  v1, with Markdown export remaining a projection.
- The requested whole-mission/current-system review report, iCloud PDF copy,
  and simplification/dead-code pass remain pending.

## 2026-06-05 Inspection: Canonical Import And Markdown Export Axis

status: existing_behavior_verified_no_new_code

inspection result:

- The current codebase already contains the generic canonicalization path the
  mission requires for imported text/document artifacts:
  - `HandleVTextOpenFile` derives a canonical `.vtext` title from `.md`, `.txt`,
    DOCX, PDF, and other opened source paths while preserving the original
    source artifact/alias.
  - `handleVTextCreateRevision` calls
    `canonicalizeAliasedVTextDocumentTitle` and
    `ensureCanonicalVTextProjectionPath` before storing a user-authored
    revision, so an aliased imported artifact converges to a `.vtext` shortcut
    and records `canonical_vtext_source_path`.
  - `edit_vtext` calls the same canonicalization and projection path before
    storing an appagent-authored revision.
  - `HandleVTextExportDocument` and publication export keep Markdown as an
    export/projection format rather than the canonical document identity.
- The implementation is generic over source path and title extension. I found
  no legal-cloud-specific or glossary-specific branch in this path.

local verification:

- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVText(APICreateRevisionCanonicalizesAliasedImportedDocumentTitle|OpenFileResolvesCanonicalAlias|ImportMarkdownLineageCreatesRevisionHistory|ImportMarkdownLineageResolvesCitationMarkers|ImportMarkdownLineageUsesExistingContentItems|ImportMarkdownLineageRejectsMissingContentItem|ImportMarkdownLineageRejectsUnknownCitationEntity|ImportMarkdownLineageRejectsExistingAlias|OpenFilePreservesDocxAndPDFOriginalArtifacts|OpenFileImportsDocxAndPDFBytesFromFilesRoot|EnsureManifestCreatesAliasAndFile|EnsureManifestReusesExistingAlias|CreateRevisionRejectsStaleHead|CreateRevisionRebasesAllowedStaleUserDraft|AppagentEditCanonicalizesAliasedMarkdownTitle)$'`
  passed.
- `nix develop -c go test ./internal/store -run 'Test.*VText|TestVText'`
  passed.
- A first attempt without `-tags comprehensive` reported no runtime tests to
  run because the relevant API tests live behind the `comprehensive` build tag.
  A plain host `go test` outside the dev shell failed on the known Dolt ICU
  dependency, matching the repo contract; it was not used as verification.

staging/publication evidence:

- The deployed legal-cloud publication route
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6` resolved
  with `7` source entities and `7` transclusions under public access policy.
- Public export with `format=md` returned
  `choir-private-legal-cloud-proposal-vtext-pub270a62fb6.md`,
  `text/markdown; charset=utf-8`, 38,398 content bytes, source markers such as
  `source:src_aba_rule_16`, and no `missing source` prose.
- The export response recorded `private_material_omitted: true`, preserving the
  publication/export boundary.

contract implication:

- For new or revised imported artifacts, current source code satisfies the
  user's clarification: `.md`, `.txt`, and document-like imports become VText
  canonical artifacts on the edit/revision path, while Markdown remains an
  export format.
- The real owner legal-cloud artifact currently publishes as the `.vtext`
  proposal path and exports clean Markdown with source markers. This supports
  proceeding to the mission-wide review and simplification pass rather than
  adding another migration patch.

residual risks:

- I did not mutate the private owner document again for this checkpoint; the
  staging evidence is publication/export proof plus existing deployed owner
  `.vtext` proof from earlier sections.
- The product still needs a cleaner owner-facing file/alias UI that explains
  "source artifact" versus "canonical VText" without relying on diagnostics.
- The review report should audit whether any legacy frontend label or file list
  can still make a canonical `.vtext` document appear to the owner as a mutable
  Markdown source.

## 2026-06-05 Review Artifact: Whole Mission And Current System

status: review_report_and_pdf_created_simplification_pending

artifact evidence:

- The whole-mission/current-system hard review was updated at
  `docs/vtext-mission-hard-review-2026-06-05.md`.
- The report covers the mission from checkpoint `f05b4c92` through current
  `main`, with findings first and a simplification baseline.
- A PDF copy was rendered to the owner's iCloud Drive at:
  `/Users/wiz/Library/Mobile Documents/com~apple~CloudDocs/VText Mission Hard Review - 2026-06-05.pdf`.
- PDF sanity check: title
  `VText Mission Hard Review - Current State - 2026-06-05`, producer
  `WeasyPrint 67.0`, 9 pages, Letter page size, unencrypted.

review conclusion:

- The system has moved from "does the source path exist?" to "is the source
  path clean, policy-safe, and simple enough to build on?"
- The highest-risk residual areas are source UX polish, source acquisition and
  cleanup, publication source policy, raw repair JSON in owner surfaces, and
  accumulated monolithic editor/runtime code.
- The next mission action is a simplification/dead-code pass guarded by the
  current source/window/canonical import tests and then renewed staging proof.

next executable probe:

- Start with `VTextEditor.svelte` simplification: extract or delete the weakest
  scaffolding without changing behavior, especially `writeThroughToFile`, raw
  source repair payload construction, source artifact panel state, and repeated
  source helper code.

## 2026-06-05 Simplification Pass: Remove Legacy File Write-Through

status: local_simplification_verified_pending_deploy

problem source:

- The hard review identified `writeThroughToFile` as legacy noncanonical
  compatibility debt. Under the current invariant, canonical document writes go
  through VText revisions, and Markdown/source files are import artifacts or
  export projections.
- Inspection showed every caller of `writeThroughToFile` was already inside a
  VText document flow with `currentDoc?.doc_id`, and the function returned
  immediately for that case. It was dead code on normal VText save/autosave/head
  update paths.

change:

- Removed `writeThroughToFile` and its helper `buildFilePath` from
  `VTextEditor.svelte`.
- Removed no-op calls from explicit save, local draft autosave, and head-change
  handling.
- Preserved canonical VText revision writes, local draft autosave, manifest
  creation, source flow behavior, and Markdown export behavior.

verification:

- `pnpm --dir frontend build` passed.
- `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVText(APICreateRevisionCanonicalizesAliasedImportedDocumentTitle|OpenFileResolvesCanonicalAlias|ImportMarkdownLineageCreatesRevisionHistory|ImportMarkdownLineageResolvesCitationMarkers|ImportMarkdownLineageUsesExistingContentItems|ImportMarkdownLineageRejectsMissingContentItem|ImportMarkdownLineageRejectsUnknownCitationEntity|ImportMarkdownLineageRejectsExistingAlias|OpenFilePreservesDocxAndPDFOriginalArtifacts|OpenFileImportsDocxAndPDFBytesFromFilesRoot|EnsureManifestCreatesAliasAndFile|EnsureManifestReusesExistingAlias|CreateRevisionRejectsStaleHead|CreateRevisionRebasesAllowedStaleUserDraft|AppagentEditCanonicalizesAliasedMarkdownTitle)$'`
  passed.
- `pnpm --dir frontend exec playwright test
  frontend/tests/vtext-source-entities.spec.js --project=chromium` passed after
  manually starting the local auth, proxy, frontend, and existing sandbox
  listener.
- First Playwright attempts failed before app behavior because no Vite listener
  was running on `localhost:4173`, then because auth/proxy had exited after the
  service script. The successful run used supervised local auth/proxy/frontend
  sessions.

residual risks:

- This is a small deletion, not the full simplification pass. Raw source repair
  JSON, source artifact panel complexity, and large editor/runtime modules
  remain.
- The simplification still needs commit, push, CI, Node B deploy, staging
  identity, and renewed Comet proof before it is accepted as deployed behavior.

## 2026-06-05 Deployed Proof: Legacy Write-Through Removed

status: deployed_simplification_checkpoint_incomplete

deployment evidence:

- Simplification commit
  `85ae3990d4736388111e297c38f00288aca35617`
  (`refactor: remove legacy vtext file write-through`) was pushed to
  `origin/main`.
- GitHub Actions CI run `27044299886` succeeded, including Go tests, runtime
  shards, frontend build, and Node B deploy.
- FlakeHub run `27044299892` succeeded for the same head SHA.
- Node B deploy installed frontend asset `index-Bqj-6E78.js`.
- `https://choir.news/health` reported proxy and sandbox upstream deployed
  commit `85ae3990d4736388111e297c38f00288aca35617`, deployed at
  `2026-06-05T22:54:47Z`.

staging proof:

- Computer Use on Comet opened and reloaded the deployed legal-cloud route
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`.
- The full legal-cloud proposal rehydrated after VM bootstrap.
- Clicking the `ABA Formal Opinion 512` inline source marker expanded a
  Pretext-routed source note in the article flow with `Open source` and
  `Close` controls.
- The already open `ABA Model Rule 1.6` source window still rendered
  reader-mode source content as headings and prose, with `Source evidence`,
  `Source entity`, and `Provenance` collapsed.
- Public resolve still returned `7` source entities and `7` transclusions under
  public access policy.
- Public Markdown export still returned 38,398 content bytes, compact
  `source:` markers, no `missing source` prose, and
  `private_material_omitted: true`.

contract implication:

- Removing the legacy noncanonical file write-through did not break VText
  rendering, source expansion, source-window reader mode, or Markdown export on
  the deployed owner/public route.
- Canonical VText revision writes remain the only write path exercised by this
  deployed proof; Markdown remains an export projection.

remaining simplification field:

- Raw source repair JSON is still visible in the owner/editor surface.
- Source artifact acquisition remains manual/operator-oriented.
- `VTextEditor.svelte` and `internal/runtime/vtext.go` remain too large and
  should be split after the next behavior-preserving extraction.
- The source-note design still needs the magazine/journal visual pass the user
  requested.

## 2026-06-05 Problem: Journal Source Note Still Clones Popover Markup

status: documented_before_code

problem:

- `frontend/src/lib/vtext-source-flow.ts` now uses Pretext to route article
  text around an expanded source note, but `mountSourceJournalFlow` still builds
  the note with `note.innerHTML = popover.innerHTML`.
- That means the magazine/journal surface is structurally coupled to
  `.vtext-source-ref-popover`, which was designed as a hidden hover/focus
  popover for inline markers.
- The result is not merely visual. A future source-note design pass has to
  fight inherited popover markup, card assumptions, generic span display rules,
  and button placement instead of rendering a purpose-built evidence note.

evidence:

- `vtext-source-flow.ts` creates the Pretext flow region and then copies the
  entire `[data-vtext-source-ref-popover]` subtree into
  `[data-vtext-source-flow-note]`.
- `VTextEditor.svelte` CSS must then override popover, transclusion body,
  facts, and source button styling inside `.vtext-source-journal-note`.
- The hard-review artifact already recorded that the user wants a
  magazine/academic journal UX with text wrapping around source apparatus, not
  more rounded card/pill layers.

root-cause hypothesis:

- The first source-flow repair reused the popover DOM because it was the
  quickest way to preserve title, excerpt, facts, and `Open source`. That
  preserved behavior, but it also preserved the wrong abstraction boundary:
  hover popover markup became the source for the journal note.

intended generic repair:

- Keep the current Pretext line-routing and source graph semantics.
- Replace popover-subtree cloning with a small source-note content builder that
  extracts only the title, excerpt/media/facts, and source-open button from the
  existing rendered marker.
- Give the journal note its own minimal classes and typography so it reads like
  a footnote or marginal evidence note, while the opened source window remains
  the full reader-mode source artifact.

forbidden shortcuts:

- Do not special-case the legal-cloud document, ABA sources, glossary tables,
  or any source id.
- Do not change source entity metadata, publication policy, Markdown export, or
  canonical VText revision semantics as part of this UI simplification.
- Do not remove the hover popover contract unless separate tests prove that the
  inline collapsed marker affordance remains accessible.

## 2026-06-05 Local Repair: Purpose-Built Journal Source Note

status: local_verified_pending_deploy

change:

- `frontend/src/lib/vtext-source-flow.ts` now builds the Pretext journal note
  with a dedicated source-note content builder instead of copying
  `.vtext-source-ref-popover` wholesale.
- The note extracts the source title, compact transclusion body, facts/media,
  and `Open source` action from the rendered marker, then adds the journal
  `Close` action in a purpose-built action row.
- The note width was reduced from a 300-380px / 42% range to a 260-340px / 34%
  range so more proposal prose remains in the reading column.
- The journal note has a modest line-height-based minimum height so the
  evidence note behaves like a marginal/journal aside with routed article text
  beside it, rather than a tiny tooltip replacement.

test changes:

- `frontend/tests/vtext-source-entities.spec.js` now asserts that the journal
  note contains `[data-vtext-source-flow-note-title]`, has no cloned
  `[data-vtext-source-ref-popover]`, and keeps an `Open source` action.
- The journal-flow geometry assertion now verifies that the second paragraph
  routes beside the note and that the next normal paragraph resumes below the
  flow. This matches the current flow, where the fixture's third paragraph can
  also be routed into the Pretext region.

local verification:

- `pnpm --dir frontend build` passed.
- `pnpm --dir frontend exec playwright test
  frontend/tests/vtext-source-entities.spec.js --project=chromium` passed:
  5/5 tests.
- The first run of the focused suite failed after the DOM simplification
  because the old geometry assertion assumed the third paragraph would remain
  outside the flow. The failure context showed the second and third paragraphs
  were now inside the routed journal flow. The verifier was updated to assert
  the actual magazine/journal contract instead of a stale fixture boundary.

local harness cleanup:

- The focused Playwright suite ran against a local stack started with
  `CHOIR_SERVICES_FOREGROUND=1 nix develop -c ./start-services.sh`.
- After verification, the foreground stack was interrupted and orphaned local
  gateway/vmctl children from that stack were stopped.
- The pre-existing sandbox listener on `127.0.0.1:8085` was preserved.

deployment status:

- This is behavior-changing frontend code. It is not accepted until committed,
  pushed to `origin/main`, CI/deploy succeeds, Node B reports the new commit,
  and Comet staging proof verifies the deployed legal-cloud source flow.

## 2026-06-05 Deployed Proof: Purpose-Built Journal Source Note

status: deployed_simplification_checkpoint_incomplete

deployment evidence:

- Behavior commit `55762aeeefb6ed1aae94445f8ee3ad160d06e85b`
  (`fix: render vtext source notes as journal notes`) was pushed to
  `origin/main`.
- GitHub Actions CI run `27044800178` succeeded, including Go vet/build,
  runtime shards, non-runtime tests, frontend build, and Node B deploy.
- FlakeHub run `27044800187` succeeded for the same head SHA.
- `https://choir.news/health` reported proxy and sandbox deployed commit
  `55762aeeefb6ed1aae94445f8ee3ad160d06e85b`, deployed at
  `2026-06-05T23:09:09Z`.
- The deployed app shell referenced frontend asset `assets/index-Bg-UU-D2.js`.

staging proof:

- Computer Use on Comet reloaded the deployed legal-cloud route
  `/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`.
- After reload, the full client proposal rehydrated with collapsed inline
  source markers and the already-open ABA Rule 1.6 source reader still
  available.
- Clicking the `ABA Formal Opinion 512` source marker expanded a single
  purpose-built journal source note. The note showed title, source excerpt,
  `Open source`, and `Close`, while proposal prose routed beside it in a
  left-column reading flow.
- The deployed note no longer presented as a rounded source card shell or a
  cloned popover; the visible surface was a minimal marginal/journal note.
- Clicking `Open source` from that note opened a separate source reader window
  for `ABA Formal Opinion 512: Generative Artificial Intelligence Tools`,
  with reader-mode source content and collapsed `Source evidence`,
  `Source entity`, and `Provenance` disclosures.

publication/export proof:

- Public resolve for the deployed route returned `7` source entities and `7`
  transclusions.
- Public Markdown export returned
  `choir-private-legal-cloud-proposal-vtext-pub270a62fb6.md`,
  `text/markdown; charset=utf-8`, 38,398 content bytes, compact `source:`
  markers, no `missing source` prose, and
  `metadata.private_material_omitted: true`.

contract implication:

- The journal-note repair changes only the article-side source-note projection.
  It preserves the canonical VText/publication source graph, opened source
  windows, source publication access, and Markdown export.
- This is progress on the magazine/academic journal UX axis, but not the final
  source UX. The remaining work is source acquisition/cleanup, owner-grade
  source review, and further editor/runtime simplification.

## 2026-06-05 Problem: Source Gap Repair Is Still JSON-First In The Owner Panel

status: documented_before_code

problem:

- The VText source panel shows unresolved citation markers and already has a
  canonical backend repair endpoint, but the owner-visible repair path still
  expects a raw JSON payload in `[data-vtext-source-repair-payload]`.
- `frontend/tests/vtext-markdown-lineage.spec.js` proves this by filling a
  complete repair payload JSON object directly into the panel before clicking
  `Apply marker repair`.
- That is an operator/debug workflow, not the owner-grade source review path
  requested for the legal-cloud proposal. A user should be reviewing a claim
  marker and entering or selecting a source, not authoring API metadata.

evidence:

- `VTextEditor.svelte` renders the normal panel with source markers and source
  entities, then exposes `<summary>Advanced marker repair</summary>` and a
  visible label `Repair JSON`.
- The JSON payload asks the UI user to understand `source_entities`,
  `citation_resolutions`, `selectors`, `display`, `target`, `evidence`, and
  `provenance`.
- The backend source-repair endpoint already preserves canonical VText revision
  semantics. The missing layer is an owner-facing adapter from claim/source
  review controls to that existing endpoint.

root-cause hypothesis:

- The source-repair path was first built to bootstrap and diagnose Markdown
  lineage gaps. It correctly created canonical source repair revisions, but
  the frontend exposed the raw endpoint shape because that was sufficient for
  tests and operator recovery.

intended generic repair:

- Add a typed source-review row/form for unresolved markers: marker, source
  title, optional URL, confirming excerpt, and an `Apply source review` action.
- Build the existing source-repair payload inside `VTextEditor.svelte` from
  those fields and call the same canonical endpoint.
- Keep raw JSON available only under a clearly diagnostic disclosure, not as
  the primary owner path.

forbidden shortcuts:

- Do not add legal-cloud-specific source ids, labels, or source mappings.
- Do not change the source-repair backend contract or canonical revision
  semantics unless a separate backend problem is documented first.
- Do not remove tests for raw repair entirely; retain a diagnostic fallback so
  operators can still recover unusual source graph cases.

## 2026-06-05 Problem: Source Diagnosis Blocks The Owner Review Panel

status: documented_before_code

problem:

- While validating the new typed source-review panel, the owner panel stayed in
  `Loading...` diagnosis state on an imported source-gap VText document.
- `frontend/tests/vtext-markdown-lineage.spec.js -g "VText Sources panel applies
  source-gap repair"` timed out waiting for
  `[data-vtext-load-diagnosis]` to return from `Loading...` to `Diagnosis`.
- When the test did not wait for diagnosis, the same panel could enter
  `Applying source review...` while diagnosis was still loading, leaving the
  owner with two concurrent source-panel requests and no completed review
  revision within the assertion window.

evidence:

- Playwright error context for
  `frontend/test-results/vtext-markdown-lineage-VTe-e57b0-pens-repaired-source-window-chromium/error-context.md`
  showed the current window
  `Panel Source Repair 1780702168517.vtext` at `v0`, the source panel open,
  `[2]` listed under `Claims needing source review`, the diagnosis button
  disabled as `Loading...`, and the document body still rendering the
  unresolved `[2]` marker.
- The same error context from the previous run showed `Applying source
  review...` while the diagnosis button remained `Loading...`.
- The backend source-repair endpoint remains proven by the direct API repair
  test in the same spec; the failure is the owner panel's coupling between
  automatic diagnosis loading and review action timing.

root-cause hypothesis:

- `handleOpenSourcePanel()` starts source diagnosis automatically for editable
  documents. The typed source-review controls are available while that
  diagnostic request is still pending, so the owner can trigger repair before
  the panel has reached a settled diagnosis state.
- Diagnosis is supporting evidence, not a prerequisite for canonical
  source-gap repair. Coupling review availability to an unbounded diagnosis
  fetch makes the owner workflow brittle and creates avoidable request
  concurrency in local/staging proof.

updated root-cause evidence:

- After diagnosis was decoupled, the panel reached `Sending source review...`
  but Playwright still never observed a `/source-repairs` response before the
  test timeout.
- The desktop snapshot showed seven old windows in the same shared Playwright
  account, including multiple VText windows and a source reader window from
  earlier runs. Each loaded VText document owns an EventSource stream through
  `openDocumentStream()`.
- The direct same-origin API call from `curl` using the same Playwright cookies
  imported a source-gap document and repaired it through
  `/api/vtext/documents/{id}/source-repairs` with HTTP `201` immediately. That
  proves the backend repair contract and payload shape are not the blocker.
- The current best root cause is local browser/proxy connection pressure from
  retained desktop windows and VText document streams in the shared test
  account. The owner workflow still should not auto-start diagnosis, and the
  test harness should not accumulate stale VText/source windows before proving
  a source-review interaction.

intended generic repair:

- Make the owner-grade source review path independent of diagnosis completion:
  unresolved markers from revision metadata/content should be enough to select
  a marker, enter a title/excerpt/URL, and apply source review.
- Keep diagnosis as an explicit secondary refresh path for edit evidence and
  deeper debugging, with loading state visible but not blocking the basic
  source-review workflow.
- Prevent overlapping diagnosis/repair interactions from leaving ambiguous UI
  state. The review action should either wait for, cancel/ignore, or run
  independently of diagnosis with clear status.
- For the E2E proof, close stale VText/source-reader windows before opening the
  source-review fixture so the test proves the actual owner path rather than
  testing an exhausted local desktop session.

forbidden shortcuts:

- Do not remove diagnosis evidence from the source panel.
- Do not make source review depend on raw repair JSON.
- Do not add legal-cloud-specific marker handling or source mappings.

## 2026-06-05 Local Repair: Typed Source Review Without Auto Diagnosis

status: local_behavior_verified

changes:

- `VTextEditor.svelte` now exposes an owner-facing `Source review` panel for
  unresolved citation markers. The owner selects a marker, enters a source
  title, optional URL, and confirming excerpt, then applies `Apply source
  review`.
- The typed form builds the existing canonical source-repair payload and calls
  `/api/vtext/documents/{doc_id}/source-repairs`. Raw repair JSON remains only
  under a `Diagnostic JSON repair` disclosure.
- Opening the Sources panel no longer automatically starts document diagnosis.
  Diagnosis remains available as an explicit button for debugging/edit
  evidence, but unresolved markers from the current VText revision are enough
  to run the owner source-review workflow.
- The source-review apply path now captures a plain payload with explicit
  `base_revision_id`, shows `Sending source review...`, and then reloads the
  repaired revision.
- `frontend/tests/vtext-markdown-lineage.spec.js` now closes stale VText and
  source-reader windows before opening each fixture document. This prevents
  retained EventSource streams in the shared Playwright desktop account from
  exhausting local browser/proxy connections and masking the source-review
  behavior under test.

local evidence:

- `pnpm --dir frontend exec playwright test
  frontend/tests/vtext-markdown-lineage.spec.js -g "VText Sources panel applies
  source-gap repair" --project=chromium --timeout=90000` passed in 3.9s after
  cleanup, proving the typed owner panel sends a real `POST /source-repairs`,
  renders the repaired source ref, opens the Pretext journal source note, and
  opens the repaired source window.
- `pnpm --dir frontend exec playwright test
  frontend/tests/vtext-markdown-lineage.spec.js --project=chromium
  --timeout=120000` passed all 6 tests in 16.6s. Coverage includes Markdown
  citation migration, imported `.md` to canonical `.vtext`, stored ContentItem
  migration, direct canonical backend source repair, typed owner source review,
  and structured edit metadata without raw prompts.
- `pnpm --dir frontend build` passed. Vite emitted
  `VTextEditor-D5Qz-okm.js` and the usual large-chunk warning for existing
  application bundles.

contract implication:

- Source review now preserves the canonical VText/source-repair path while
  removing raw JSON as the owner-grade workflow.
- Diagnosis is no longer a hidden prerequisite for source repair. This better
  matches the invariant that the source graph is canonical VText metadata and
  citation/transclusion repair should be a bounded revision operation.
- The stale-window cleanup is test harness hygiene, not a product workaround.
  A separate future hardening axis remains: multiplex, pause, or otherwise
  bound per-window VText streams so long-lived owner desktops cannot starve
  ordinary API requests under HTTP/1.1-like local conditions.

residual risks:

- The explicit Diagnosis button path still needs a focused performance/hang
  review. This repair intentionally stops diagnosis from blocking source
  review; it does not claim the diagnosis endpoint is optimal.
- The staged owner proof still needs to confirm the same typed source-review
  behavior on the deployed owner account after push/deploy.

## 2026-06-05 Deployed Proof: Typed Source Review

status: deployed_behavior_verified

landed commits:

- `9d7530ca` (`docs: record vtext source diagnosis blocker`)
- `fb688447` (`docs: record source review stream pressure`)
- `ed1835ff4a3b5dafd448b68d2596b35303903f84` (`fix: add typed vtext
  source review`)

CI and deploy:

- GitHub Actions CI run `27045898828` completed successfully.
- CI green jobs included integration-tagged smoke, non-runtime Go tests, Go
  vet/build, all four internal/runtime shards, frontend build, aggregate gate,
  and `Deploy to Staging (Node B)`.
- FlakeHub run `27045898824` completed successfully.
- `https://choir.news/health` reported proxy and sandbox deployed at
  `ed1835ff4a3b5dafd448b68d2596b35303903f84`, deployed at
  `2026-06-05T23:43:32Z`.

Computer Use / Comet proof:

- Computer Use was available and used against Comet.
- Comet was on
  `https://choir.news/pub/vtext/choir-private-legal-cloud-proposal-vtext-pub270a62fb6`.
- The deployed published legal-cloud VText showed Pretext/journal source flow
  with source marker expansion and source reader windows for:
  - `ABA Model Rule 1.6: Confidentiality of Information`;
  - `ABA Formal Opinion 512: Generative Artificial Intelligence Tools`.
- Clicking `Edit my version` in Comet created/opened
  `My version of choir_private_legal_cloud_proposal.vtext`, preserving the
  proposal prose, Markdown table glossary, and source markers as VText UI
  source references in the owner/private editable surface.

deployed source-review backup proof:

- The exact typed source-review panel needs an unresolved-marker fixture. The
  current legal-cloud proposal has represented source markers, so the Comet
  owner document did not expose the source-review repair form.
- Product-path staging backup used the deployed browser/API flow with
  authenticated Playwright storage:
  `BASE_URL=https://choir.news CHOIR_AUTH_STATE=frontend/playwright/.auth/choir-news.storage.json
  CHOIR_AUTH_META=frontend/playwright/.auth/choir-news.storage.meta.json pnpm
  --dir frontend exec playwright test
  frontend/tests/vtext-markdown-lineage.spec.js -g "VText Sources panel applies
  source-gap repair" --project=chromium --timeout=120000`.
- Result: `1 passed (14.2s)`. This proves deployed `ed1835ff` renders the typed
  owner source-review panel for an unresolved citation marker, sends a real
  canonical `/source-repairs` request, reloads the repaired VText revision,
  opens the Pretext journal note, and opens the repaired source window.

residual risks:

- The deployed source-review fixture ran under Playwright staging auth, not the
  `yusefnathanson@me.com` Comet session, because the legal-cloud owner document
  no longer has unresolved source markers to repair. Comet was still used for
  the deployed owner/private legal-cloud source-marker and source-window proof.
- The hard mission review report, PDF export to iCloud, and simplification pass
  remain open mission work after this deployed checkpoint.

## 2026-06-05 Hard Review And First Simplification Pass

status: local_refactor_verified

review artifacts:

- Markdown report:
  `docs/vtext-mission-current-system-hard-review-2026-06-05.md`.
- PDF report:
  `/Users/wiz/Library/Mobile Documents/com~apple~CloudDocs/vtext-mission-current-system-hard-review-2026-06-05.pdf`.

first simplification:

- Extracted source-review entity-id and payload construction from
  `VTextEditor.svelte` into `frontend/src/lib/vtext-source-review.js`.
- The editor still owns the source-review form state and submit action, but the
  canonical source-repair payload shape is now a pure helper rather than
  another inline editor concern.

local evidence:

- `pnpm --dir frontend build` passed after the extraction.
- After restarting the local dev stack, `pnpm --dir frontend exec playwright
  test frontend/tests/vtext-markdown-lineage.spec.js --project=chromium
  --timeout=120000` passed all 6 tests in 15.7s.

remaining simplification field:

- The hard-review report still recommends extracting the full source panel,
  bounding Diagnosis, moving diagnostic JSON behind operator/developer mode,
  and budgeting/multiplexing VText streams. Those are not done in this first
  simplification pass.

deployed refactor evidence:

- Refactor commit `9c7637ee8397814f1034658ee0f232efd8e8c3be`
  (`refactor: extract vtext source review payload`) was pushed to `main`.
- GitHub Actions CI run `27046250324` completed successfully, including
  frontend build, Go gates, and `Deploy to Staging (Node B)`.
- FlakeHub run `27046250328` completed successfully.
- `https://choir.news/health` reported proxy and sandbox deployed at
  `9c7637ee8397814f1034658ee0f232efd8e8c3be`, deployed at
  `2026-06-05T23:54:43Z`.
- Deployed source-review proof passed:
  `BASE_URL=https://choir.news CHOIR_AUTH_STATE=/Users/wiz/go-choir/frontend/playwright/.auth/choir-news.storage.json
  CHOIR_AUTH_META=/Users/wiz/go-choir/frontend/playwright/.auth/choir-news.storage.meta.json
  pnpm --dir frontend exec playwright test
  frontend/tests/vtext-markdown-lineage.spec.js -g "VText Sources panel applies
  source-gap repair" --project=chromium --timeout=120000` -> `1 passed
  (14.9s)`.

## 2026-06-06 Local Hardening: Bounded Source Diagnosis

status: local_behavior_verified

problem addressed:

- The hard-review report identified source Diagnosis as a P1 weak path: it was
  no longer required for source review, but an explicit Diagnosis request could
  still stay pending without a clear cancel/timeout contract.

changes:

- `getVTextDiagnosis()` now accepts a fetch `signal` so callers can abort the
  request.
- `VTextEditor.svelte` now gives source Diagnosis a bounded client timeout
  (`12000ms`) and an explicit cancel state. While pending, the button reads
  `Cancel diagnosis`; closing the Sources panel or destroying the editor also
  aborts the in-flight diagnosis request.
- Abort handling distinguishes timeout from user cancellation. Timeout reports
  that source review remains available; cancellation returns the panel to the
  normal `Diagnosis` state.

local evidence:

- Focused test:
  `pnpm --dir frontend exec playwright test
  frontend/tests/vtext-markdown-lineage.spec.js -g "cancel diagnosis"
  --project=chromium --timeout=90000` -> `1 passed (6.1s)`.
- Full lineage spec:
  `pnpm --dir frontend exec playwright test
  frontend/tests/vtext-markdown-lineage.spec.js --project=chromium
  --timeout=120000` -> `7 passed (29.8s)`.
- Frontend build:
  `pnpm --dir frontend build` passed and emitted
  `VTextEditor-CNWZyUK8.js`.

contract implication:

- Diagnosis remains an explicit evidence/debug path rather than a hidden
  prerequisite for source review.
- Source review, source gaps, and source entity repair remain available while
  diagnosis is pending or cancelled.

deployed hardening evidence:

- Behavior commit `3964703ca7589d364b71101cda1f816244e04ad3`
  (`fix: bound vtext source diagnosis`) was pushed to `main`.
- GitHub Actions CI run `27046515713` completed successfully.
- FlakeHub run `27046515740` completed successfully.
- `https://choir.news/health` reported both proxy and sandbox at deployed
  commit `3964703ca7589d364b71101cda1f816244e04ad3`, deployed at
  `2026-06-06T00:03:47Z`.
- Deployed staging proof:
  `BASE_URL=https://choir.news CHOIR_AUTH_STATE=/Users/wiz/go-choir/frontend/playwright/.auth/choir-news.storage.json
  CHOIR_AUTH_META=/Users/wiz/go-choir/frontend/playwright/.auth/choir-news.storage.meta.json
  pnpm --dir frontend exec playwright test
  frontend/tests/vtext-markdown-lineage.spec.js -g "cancel diagnosis|VText
  Sources panel applies source-gap repair" --project=chromium --timeout=120000`
  -> `2 passed (19.0s)`.

residual risk:

- This hardening bounds the weak Diagnosis path but does not yet remove the
  diagnostic JSON surface from normal reader/editor UX or replace manual source
  review with researched confirming/refuting source acquisition.

## 2026-06-06 Problem: Source Flow Styling Is Still Owned By The Monolithic Editor

status: documented_before_code

current uncertainty or obstacle:

- The latest user clarification tightened the Pretext requirement: Pretext is
  valuable here because it lets article prose wrap like a magazine or academic
  journal around a source note. It is not primarily a source-card styling tool.
- The current source-flow behavior uses the right Pretext family:
  `prepareRichInline()` for citation fragments and `layoutNextLineRange()` /
  `layoutNextRichInlineLineRange()` for row-by-row routing around the note.
- The ownership boundary is still weak. `frontend/src/lib/vtext-source-flow.ts`
  owns the geometry and DOM mount, but the matching `.vtext-source-journal-*`
  presentation rules still live as a large global CSS island inside
  `VTextEditor.svelte`.

selected cognitive transforms:

1. Depth extraction on "use Pretext": the load-bearing variable is text-flow
   geometry, not whether the source note visually says "card" or "chip".
2. Ownership inversion: the source-flow module should own the source-flow
   presentation contract instead of relying on a monolithic editor stylesheet.
3. Verifier shift: prove the note is a wrapped journal surface with line
   geometry and source-window behavior, then use visual proof on staging for
   owner confidence.

route-changing insight:

- The next simplification should not add another source-card abstraction.
  It should move the journal-flow CSS next to the source-flow implementation,
  preserving the existing data attributes and tests while making the wrapped
  source note easier to tune and prune.

intended generic repair:

- Extract the `.vtext-source-journal-*` and source-flow-specific overrides from
  `VTextEditor.svelte` into a focused source-flow stylesheet imported by the
  VText surface.
- Keep canonical VText, source entity metadata, publication bundles, Markdown
  export, and source-window reader behavior untouched.
- Preserve the hover/collapsed marker affordance outside the mounted journal
  flow.

evidence to collect:

- Local frontend build.
- Source-entity Playwright coverage proving source markers still expand into a
  Pretext-routed journal note, lines route beside the note, no cloned popover is
  in the note, and source windows still open.
- If behavior changes land, push to `main`, wait for CI/deploy, verify staging
  identity, and run deployed owner/path proof with Comet as primary where
  possible.

local probe update:

- After the first stylesheet extraction, `pnpm --dir frontend exec playwright
  test frontend/tests/vtext-source-entities.spec.js --project=chromium
  --timeout=120000` failed the journal-flow geometry assertion:
  `continuedBelowFlow` was false.
- The failure context showed the source note still existed and prose still
  routed beside it, but the generic editor rules for transclusion quotes,
  source facts, and source buttons won the cascade inside the extracted journal
  note. That made the note taller and pulled more paragraphs into the synthetic
  Pretext flow.
- Root cause: simply moving selectors out of Svelte changed the cascade owner.
  The source-flow stylesheet must use source-flow-specific selectors strong
  enough to preserve the minimal journal note instead of inheriting generic
  card/pill source affordances.
