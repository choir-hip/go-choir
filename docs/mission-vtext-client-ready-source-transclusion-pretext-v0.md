# MissionGradient v0: Client-Ready VText Source Transclusion And Proposal Cleanup

Status: draft for owner review
Date: 2026-06-05

Requirements contracts:
[source-external-data-publication.md](source-external-data-publication.md),
[vtext-version-compare-merge-debuggability-spec.md](vtext-version-compare-merge-debuggability-spec.md),
[vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md](vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md)

Related mission:
[mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md](mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md)

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
