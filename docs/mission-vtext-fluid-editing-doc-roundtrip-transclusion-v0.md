# MissionGradient v0: VText Fluid Editing, Document Roundtrip, And Source Transclusion

Status: draft for owner review
Date: 2026-06-05
Requirements contracts:
[source-external-data-publication.md](source-external-data-publication.md),
[vtext-version-compare-merge-debuggability-spec.md](vtext-version-compare-merge-debuggability-spec.md),
[vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md](vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md)

## Goal String

```text
/goal Run docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md as a Codex-operated MissionGradient mission. Preserve the requirements contracts in docs/source-external-data-publication.md, docs/vtext-version-compare-merge-debuggability-spec.md, and docs/vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md, and treat the Required Behavior Spec inside this mission doc as binding. Apply state-machine inversion and via negativa to VText: the document delta is the control surface, not a classifier/workflow/prompt scaffold; the user's direct document edits are the instruction-bearing diff; the VText agent receives only the current head revision plus that user edit diff and minimal document identity by default; prior versions, source metadata, worker messages, import manifests, and publication records are available through retrieval tools instead of preloaded context. Remove prompt/content classifiers, required meta syntax such as //<edit>, routine whole-document rewrites, rendered-DOM corruption paths, Markdown-shaped special cases, visible hidden metadata, and citation decorations that do not resolve to source entities. Repair VText so long-document editing is fluid, semantic, and fast; fix the rendered-Markdown roundtrip bugs that corrupt tables and other structure; normalize .md/DOCX/PDF imports into VText working projections while preserving original files as ContentItems with import manifests; implement revise/export roundtrip paths for MD, DOCX, PDF, TXT, and HTML with provenance metadata and style profiles where possible; and make copy/download/open UX easy from the VText publish surface. Convert existing versioned Markdown documents, including the legal-cloud proposal class of docs, into real VTexts with preserved version lineage, source entities, citation markers, expandable transclusions, and openable source windows. Complete the VText source-entity interaction model: every citation marker is a tap/click transclusion target; quoted excerpts and source entities with quote/excerpt selectors default to embedded transclusion; collapsed citations expand inline; expanded transclusions can open the owning source, media, document, or VText in its own app/window. QA by computer use on staging as the default, with Playwright/agent-browser only as backup. Do not claim success without deployed staging evidence on a real long VText showing table preservation through focus/edit/save/revise, direct instruction edits consumed and applied without stale replacement text, materially lower ordinary revision prompt size and latency, non-VText import normalization, migrated versioned Markdown lineage, DOCX/PDF export/import proof with original preservation, and user-visible citation expansion/open-source behavior.
```

## Thesis

VText should feel like editing a living professional document, not steering a
chatbot that rewrites a file. The user edits the document directly. Those edits
are the instruction surface. VText interprets the change, consumes scratch
instructions when they are not meant as prose, applies the intended semantic
edit, preserves structure and sources, and creates one meaningful new revision.

The same mission must close the document-format loop. Users will start from
Markdown, DOCX, PDF, pasted web content, source citations, and old VTexts. Choir
should preserve originals as source artifacts, create VText projections for
revision, and export back into useful document formats with provenance and as
much style preservation as the format allows.

Existing versioned Markdown documents are a first-class migration target, not
edge cases. The legal-cloud proposal class of documents should become real
VTexts with preserved version lineage, VText revision IDs, source entities,
visible citation markers, expandable transclusions, and openable source
windows. A `.md` filename may remain a compatibility alias or source label, but
the working object should be VText once the user is revising, comparing,
publishing, or citing it inside Choir.

The source interaction model is part of the same artifact. A citation in VText
is not a footnote-shaped decoration; it is a live transclusion point. The user
should be able to tap a superscript citation, expand the evidence inline, and
open the owning source surface when needed.

## Cognitive Transforms

Current uncertainty or obstacle:

VText is carrying too many accidental pathways: rendered DOM is serialized back
into Markdown and corrupts structure, long-document revisions preload too much
history into model context, non-VText files are edited as if lossy roundtrip is
free, and citations are sometimes treated as visible marks rather than source
entities with transclusion behavior.

Selected transforms:

1. Depth extraction: the real feature is not "apply a prompt to a document";
   it is an intent-bearing diff over a canonical artifact.
2. State-machine inversion: the document delta drives VText behavior; the
   runtime does not classify the user's words into a workflow before VText can
   act.
3. Via negativa: remove prompt classifiers, required edit syntax, routine full
   rewrites, rendered-DOM corruption paths, and hidden metadata leakage.
4. Substrate normalization: imported files are source artifacts plus VText
   projections, not mutable originals pretending to be VText.
5. Artifact identity transform: citations, transclusions, source entities,
   import manifests, revisions, and exports are one document graph, not
   separate UI flourishes.
6. Evidence-first QA: the acceptance surface is the deployed product operated
   by computer use, because the failures are interactive and perception-level.

Route-changing insights:

- There is no separate prompt box concept for normal revision. The user's
  direct edit diff is the instruction.
- The fast path should not be chosen by a classifier; it should be the only
  default path. Extra context is retrieved by tools when needed.
- Table corruption is a symptom of a broader serialization boundary problem.
- DOCX/PDF import is inherently lossy, so original preservation and import
  manifests are correctness requirements, not polish.
- Citation expansion and open-source behavior must be proven in the same VText
  artifact used for editing/export proof.

## Required Behavior Spec

This section is the behavioral contract for the mission. If code, prompts,
tools, or UI conflict with this section, this section wins unless the mission
doc is explicitly revised.

### 1. Inverted VText Edit Loop

The ordinary VText revision loop is:

```text
current canonical VText revision
  -> user edits document directly
  -> system computes user edit diff
  -> VText receives current head + user edit diff + minimal identity
  -> VText interprets the diff as intent
  -> VText calls structured edit tools or retrieves more context if needed
  -> VText writes one canonical revision with metadata
```

Required behavior:

- The user does not need a separate prompt field for ordinary revision.
- The user does not need to mark instructions with `//<edit>`, XML, comments,
  or any other meta syntax.
- The user's edits may be prose, replacement text, scratch instructions,
  deletions, annotations, or a mix. VText decides what belongs in the final
  artifact by interpreting the diff.
- If inserted text is instruction-like, VText should consume it and apply the
  requested change rather than preserving it as prose.
- If inserted text is meant to replace existing text, VText removes the stale
  target text rather than appending a second competing version.
- If the user's edit is literal final prose, VText preserves it as prose.
- Multiple local edits may be grouped into one canonical revision.

### 2. Via Negativa: Removed Behavior

The following paths are not acceptable fixes:

- no prompt/content classifier that decides whether a revision is simple,
  complex, research, formatting, or rewrite before VText acts;
- no required edit syntax, sentinel comments, or hidden prompt markers;
- no routine whole-document rewrite for ordinary long-document edits;
- no conductor-authored first draft or conductor-authored semantic revision;
- no required tool choreography where the runtime pre-decides that VText must
  call a specific sequence of tools;
- no preloading large prior-version summaries, worker messages, or metadata
  history into ordinary revision context;
- no Markdown write-through that mutates the original non-VText artifact after
  the user has begun VText work;
- no visible hidden metadata comments, provenance payloads, hashes, or merge
  instructions in the document body;
- no citation marker that is merely visual and cannot resolve to a source
  entity or repairable source gap;
- no frontend-only import/export conversion that bypasses canonical artifacts,
  import manifests, export policy, or provenance metadata.

### 3. Context And Tool Behavior

Default context must be small and invariant:

- current head revision content;
- exact user edit diff;
- document ID, revision ID, draft-line ID, owner context, and source/import
  identifiers needed to resolve tools;
- concise tool descriptions.

Additional context must come through explicit VText tool calls:

- fetch prior revision;
- list/search versions;
- fetch source entities;
- resolve ContentItem or Source Service item;
- fetch import or migration manifest;
- fetch publication/export policy;
- fetch researcher/worker evidence;
- search local/user corpus and Source Service.

The runtime should make retrieval available; it should not decide the semantic
workflow by inspecting the user's words.

### 4. Structured Edit Behavior

Ordinary revisions should use structured edit operations:

- replace paragraph, heading, list item, table row/range, or section;
- insert before/after a stable selector;
- delete a selected range;
- move a section or block;
- update citation/source-entity metadata;
- update display policy;
- preserve unrelated blocks.

Whole-document rewrite is allowed only for explicit whole-artifact
transformations such as style rewrite, summary, expansion from outline, or
full reorganization. It must record rationale, prompt size, token use, and why
structured edits were insufficient.

### 5. Existing Versioned Markdown Migration

Existing versioned Markdown documents are migration inputs, not permanent
special cases.

Required behavior:

- Migrate each existing version into a durable VText revision with version
  number, parent lineage, author/source, timestamp when known, and content
  hash.
- Preserve the original Markdown file and version records as source artifacts
  or migration evidence.
- Keep historical versions publishable, comparable, mergeable, and restorable
  after migration.
- Resolve existing citations, links, footnotes, source sections, appendix
  references, and quoted excerpts into `source_entities` when evidence exists.
- Missing source evidence becomes a repairable source gap. Do not invent
  citations.
- The migrated document should no longer rely on Markdown write-through for
  canonical VText edits.
- The legal-cloud proposal class of docs is an acceptance target for this path.

### 6. Document Format Normalization

For `.md`, `.txt`, DOCX, PDF, and future imported document types:

- the original file remains a `ContentItem` or source artifact;
- VText owns the revisable projection;
- the projection carries an import manifest or migration manifest;
- import manifests record source hash, adapter, adapter version, warnings,
  lossiness, selectors, style profile, and asset manifest;
- exports are generated from canonical VText plus export policy, not rendered
  DOM;
- DOCX/PDF export embeds compact provenance metadata without leaking private
  owner IDs, secrets, raw prompts, traces, or unpublished revisions.

### 7. Citation And Transclusion Behavior

Every visible citation marker is an interactive transclusion target.

Required behavior:

- Tap/click on a citation marker expands the source material inline.
- Quoted excerpts and quote/excerpt selectors default to embedded
  transclusion.
- Background support citations default to collapsed markers unless VText sets a
  stronger display policy.
- Expanded transclusions can collapse again.
- Expanded transclusions can open the owning source surface in a new
  app/window when a source artifact exists.
- Supported owning surfaces include ContentItem, media/video, transcript,
  Source Service item, local file source, another VText, and publication span.
- If a citation target is missing, the UI exposes a repairable source gap
  instead of pretending the citation is valid.
- Publication and export preserve citation/source identity and display policy.

### 8. Publish And Export UX

Required behavior:

- Publishing does not reload or navigate away from the VText app.
- The browser URL may become the public route when that is the intended
  product confirmation, but explicit `Copy link`, `Open link`, `Copy text`,
  and `Download` controls remain available.
- Visible link text is compact.
- Low-level hashes, IDs, and metadata are hidden from visible chrome.
- Download supports policy-allowed MD, TXT, HTML, DOCX, and PDF formats.
- Copy/download/open actions read canonical publication/export artifacts.

### 9. QA Behavior

Computer use is the default QA mode because these failures are product-path
and interaction-level.

Required QA scenarios:

- edit a real long VText on staging without meta syntax;
- verify instructions are consumed and stale text is removed;
- verify table/list/citation structure survives focus, edit, autosave, revise,
  reload, compare, merge, and publish;
- migrate a versioned Markdown legal-cloud proposal class doc into VText;
- expand migrated citation markers into transclusions;
- open at least one transclusion source in a new window/app;
- import DOCX and PDF, revise as VText, export DOCX and PDF, and inspect
  lineage/metadata;
- record revision latency, prompt size, token use, context mode, edit
  operation, retrieval calls, and delta evidence.

## Real Artifact

The artifact is a deployed VText document system over real user-computer state:

```text
original file/source/content
  -> ContentItem or source artifact
  -> import or migration manifest when lossy conversion is needed
  -> VText working projection
  -> preserved version lineage for existing versioned docs
  -> direct user edits as instruction-bearing diffs
  -> VText agent revision using current head + diff by default
  -> structured edit operations and preserved source_entities
  -> semantic versions, compare, merge, and publication
  -> citation markers as transclusion controls
  -> export artifacts with policy, metadata, and style profile
```

The artifact is not:

- a contenteditable Markdown demo;
- a chat prompt wrapped around a document;
- a classifier-driven workflow;
- a one-off table repair;
- a frontend-only DOCX/PDF converter;
- a raw Markdown citation convention without source metadata;
- a source browser detached from VText.

## Hard Invariants

- VText remains the canonical artifact-level writing surface.
- Only the VText agent writes canonical `.vtext` files.
- User edits inside the document are the ordinary instruction surface.
- No required meta syntax such as `//<edit>` is needed for the agent to
  understand an instruction-bearing edit.
- No prompt/content classifier chooses whether a revision is "simple" or
  "complex." The default context contract is always current head plus user edit
  diff; the VText agent retrieves more context through tools when needed.
- Ordinary long-document revisions use structured edits. Whole-document rewrite
  is explicit, exceptional, and recorded with rationale.
- Draft persistence does not flood canonical version history.
- Non-VText originals are preserved. VText projections do not overwrite the
  source artifact's identity.
- Existing versioned Markdown documents migrate into VText without flattening
  their version lineage or losing the ability to publish, compare, and merge
  historical versions.
- Import/export metadata is hidden from the reading surface unless deliberately
  rendered as user-facing citation/source content.
- Every visible citation marker is a transclusion point.
- Display policy is revision metadata set by VText, not a renderer guess.
- Quoted excerpts and quote/excerpt source selectors default to embedded
  transclusion unless VText deliberately sets a stronger contrary policy.
- Expanded transclusions can open the owning source, media, document, or VText
  in its own app/window.
- Export bytes come from canonical artifacts and export policy, not rendered
  DOM scraping.
- Staging `https://choir.news` is the acceptance environment.
- Computer use is the primary QA path; Playwright or agent-browser is backup.

## Value Criterion

Minimize latency, semantic loss, formatting loss, and provenance loss across
VText edit, import, source citation, publication, and export while preserving
the user's direct-document workflow.

The system moves uphill when:

- a long VText revision consumes only current head plus the user's edit diff by
  default;
- prompt size, token use, and wall-clock latency drop for ordinary revisions;
- user instruction text is consumed instead of leaking into the document;
- replacement edits remove stale text instead of appending competing prose;
- tables, lists, citations, and metadata survive focus/edit/save/revise;
- imported DOCX/PDF/MD files become revisable VTexts without losing the
  original file;
- existing versioned Markdown docs become VTexts with usable historical
  versions rather than remaining Markdown-shaped special cases;
- exports preserve useful style, visible citations, and compact provenance;
- citation tap targets expand evidence inline and can open owning sources.

## Current Belief State

Evidence already gathered:

- On yusef's `choir_private_legal_cloud_proposal.md`, the appendix table
  repeatedly reverted because rendered table HTML was wrapped in
  `div.table-scroll`, then editor input serialization missed the table and
  flattened nested cell text back into Markdown.
- The same document's recent appagent revisions took roughly one to two
  minutes, with prompts around tens of thousands of characters and input-token
  counts often far above what an ordinary local edit should require.
- Tool commit latency metadata was low, so the visible slowness is dominated by
  model/context path rather than final revision persistence.
- The VText backend currently preloads current content plus previous revision,
  history summaries, worker messages, and other context. This pushes ordinary
  edits toward large-context generation.
- Some revision flows have let user instruction text remain in the output or
  added replacement content without deleting the content being replaced.
- `.md` source paths still matter: write-through and rendered Markdown
  roundtrip can mutate the original file path in ways that are wrong once the
  user is doing VText work.
- Existing source architecture docs already define source entities, display
  policy, and every citation as a transclusion point, but product-path proof is
  incomplete.
- Existing export/import research identifies Pandoc, Mammoth, pdfplumber, DOCX
  custom properties, and PDF XMP metadata as likely adapter choices.

Highest-impact uncertainty:

Whether the current VText edit tool schema can express reliable section/table
edits quickly enough, or whether it needs a stronger structured block selector
contract before long-document editing becomes consistently fluid.

## Homotopy Axes

Increase realism along these axes without changing topology:

- Edit context: full preload -> current head plus edit diff -> tool-retrieved
  prior context only when needed.
- Edit operation: replace all -> paragraph/section patch -> block/table/list
  patch -> source-entity-aware patch.
- Serialization fidelity: rendered DOM Markdown -> wrapper-aware serializer ->
  structured VText block model -> editor operations over structured blocks.
- Format normalization: `.md` aliases -> `.md` original plus VText projection
  -> DOCX/PDF originals plus import manifests -> roundtrip style profiles.
- Export depth: TXT/MD/HTML -> DOCX/PDF bytes -> embedded compact provenance ->
  policy-controlled exports.
- Source interaction: collapsed superscript -> inline transclusion -> embedded
  excerpt defaults -> open owning app/window -> merge/source-aware compare.
- QA realism: local tests -> staging browser automation -> computer-use
  product QA on real long documents and real imported files.

## Required Work

### 1. Problem Documentation Checkpoint

Before behavior-changing fixes, update or create problem records for:

- rendered Markdown/table roundtrip corruption;
- slow long-document revision context path;
- instruction text leaking into canonical output;
- replacement edits appending instead of replacing;
- non-VText write-through and normalization gap;
- DOCX/PDF import/export roundtrip risk;
- citation/transclusion UI proof gap.

### 2. Reproduce With Computer Use

Use computer use on staging as the default QA harness.

Reproduce and record:

- appendix/glossary table survives initial render but flattens after focus or
  edit;
- ordinary instruction-based revision on a long VText takes too long;
- direct instruction text can remain in the final document;
- replacement instructions can leave stale content beside new content;
- citation markers do or do not expand/open as required;
- published/downloaded exports lack DOCX/PDF paths or needed metadata.

Use Playwright or agent-browser only when computer use cannot capture the
needed state or when a repeatable regression harness is needed after a manual
repro.

### 3. Fix VText Render/Edit Roundtrip

Repair the editor boundary so rendered Markdown structures serialize back to
canonical Markdown/VText without corruption.

Required coverage:

- table wrappers such as `.table-scroll > table`;
- `thead`, `tbody`, `tr`, `th`, and `td`;
- lists, nested lists, blockquotes, code blocks, horizontal rules, footnotes,
  citation markers, source chips, and embedded transclusion blocks;
- no visible metadata comments or provenance payloads in the reading surface;
- focus/click/type/autosave/reload does not corrupt structure.

This may start by fixing the serializer, but the mission should keep moving
toward structured editor operations where feasible.

### 4. Implement Diff-First VText Revision Context

Make the VText agent's default revision context:

- current head revision content;
- user edit diff from head to user-authored draft;
- minimal document identity and source/draft-line IDs;
- available retrieval tools.

Remove routine preloading of:

- large prior-version summaries;
- long worker-message context;
- full metadata history;
- unrelated recent revision diffs.

Expose retrieval tools instead, such as:

- list VText versions;
- fetch a specific revision;
- search prior revisions;
- fetch document metadata and source entities;
- fetch import manifest and export profile;
- fetch worker/researcher messages;
- resolve source entities and ContentItems;
- search source service and local corpus.

The VText prompt/tool contract should say: inspect additional context only when
the edit actually requires it.

### 5. Tighten The VText Editing Contract

Update prompts, tools, tests, and metadata so ordinary revisions:

- use structured edits first;
- consume instruction text when it is not intended as prose;
- replace stale target content instead of appending alternatives;
- preserve unrelated sections and source entities;
- preserve tables/lists/citations unless the edit asks to change them;
- record edit operation, target selectors, prompt chars, tokens, model,
  retrieval calls, latency, delta size, and rewrite rationale if any.

Whole-document rewrite remains valid for transformations such as style rewrite,
summary, expansion from outline, or full reorganization, but it must be
explicit and auditable.

### 6. Normalize Non-VText Documents

When a user opens or imports `.md`, `.txt`, DOCX, PDF, or similar artifacts and
begins VText work, create a VText working projection and preserve the original
artifact.

Required model:

```text
original file
  -> ContentItem original artifact
  -> import manifest
  -> VText projection
  -> VText revisions
  -> export artifacts linked back to original and revision lineage
```

For `.md` and `.txt`:

- preserve the original file/source path and hash;
- create a `.vtext` working projection or equivalent VText document identity;
- avoid destructive write-through once VText revision work begins.

For existing versioned Markdown documents:

- migrate the existing version history into VText revisions with durable
  version numbers and parent lineage;
- preserve the original Markdown file/version records as source artifacts or
  migration evidence;
- keep historical versions publishable, comparable, and mergeable after
  migration;
- resolve existing inline links, footnotes, citations, source lists, appendix
  references, and quoted excerpts into `source_entities` where evidence exists;
- when source evidence is missing, create a repairable citation/source gap
  rather than inventing a source;
- render every migrated citation marker as an expandable transclusion point;
- ensure each transclusion can open the owning source window when a source
  artifact, URL, ContentItem, publication, media item, or VText target exists.

For DOCX:

- preserve original DOCX bytes;
- import through a structured adapter such as Pandoc `docx+styles` and/or
  Mammoth style maps;
- preserve named styles, headings, lists, tables, footnotes, comments, images,
  captions, and document properties where possible;
- derive a style/export profile for later DOCX export.

For PDF:

- preserve original PDF bytes;
- extract text, tables, page selectors, images, OCR status, geometry, and
  confidence where possible;
- record lossiness and unresolved layout warnings;
- never imply exact visual roundtrip when the PDF lacks semantic structure.

### 7. Implement Import -> Revise -> Export UX

Build a simple user path:

- import/open file;
- see that it is now revisable as VText;
- revise using direct document edits;
- export as MD, TXT, HTML, DOCX, or PDF where policy allows;
- copy full text;
- copy/open public link after publication without reloading the VText app;
- download through a menu with clear filetype options;
- keep link text compact and hide low-level hashes/metadata from visible UI.

DOCX/PDF exports should be server-side publication/document exports with bytes,
media type, filename, content hash, export metadata, and policy checks. Use
embedded metadata for compact provenance:

- DOCX core/custom properties or a custom XML part when appropriate;
- PDF XMP/DocumentInfo synchronization;
- no private owner IDs, access tokens, unpublished revision text, raw logs, or
  hidden agent traces in exported files.

### 8. Complete Citation-To-Transclusion UX

Implement the VText source interaction contract:

- every visible citation marker is a tap/click target;
- activation expands the associated transclusion inline;
- quoted excerpts and quote/excerpt source selectors default to embedded
  transclusion;
- collapsed support citations default to compact superscript markers;
- expanded transclusions show excerpt/media/table/document/source details
  appropriate to target and selector;
- expanded transclusions have controls such as open source, show context, use
  in merge, pin, and open in window where appropriate;
- opening in window launches the owning app/surface for the target type:
  ContentItem, media app, transcript view, source-service item, another VText,
  publication route, or local file source;
- private/public publication projection preserves the same source identity and
  display policy.

The UI should follow existing VText visual language. The provided mockups are
behavioral inspiration, not a requirement to change the design system.

### 9. Preserve Version Compare/Merge Integration

Keep the earlier version/merge mission coherent with this work:

- historical versions remain publishable;
- `Primary draft` stays the default product label;
- compare identifies changes in sections, concepts, citations, metadata, and
  transclusions;
- merge previews preserve provenance without leaking metadata into the body;
- accepting a preview creates the next canonical revision;
- a v44-like historical source can contribute a well-formatted table or
  section to the latest draft without reintroducing table corruption.

### 10. Observability And Debuggability

For every VText appagent revision, persist:

- end-to-end latency;
- provider/model latency;
- prompt chars and input/output tokens;
- model/provider;
- context mode, expected `diff_first`;
- retrieval calls and retrieved object IDs;
- edit operation and target selectors;
- rewrite rationale when used;
- delta size and structure-change summary;
- source entity changes;
- import/export lineage changes;
- errors and persistence failures.

Owner/admin diagnosis export should be able to answer, for a document:

- what changed;
- why it was slow;
- which context was sent;
- which retrieval tools were used;
- whether a rewrite occurred;
- whether tables/source entities changed;
- which import/export/source artifacts are linked;
- whether hidden metadata leaked into visible content.

## Evidence Plan

Required staging evidence:

- deployed commit identity and health for `https://choir.news`;
- computer-use QA notes or screenshots for the real long VText editing path;
- table preservation proof across focus, direct edit, autosave, revise, reload;
- instruction-consumption proof with no `//<edit>` or other meta syntax;
- replacement proof where stale text is removed;
- latency/token evidence showing ordinary revisions use `diff_first` context
  and materially smaller prompts than the current full preload path;
- revision metadata showing structured edits rather than whole-document rewrite
  for ordinary edits;
- `.md` normalization proof showing original preservation and VText projection;
- migration proof for an existing versioned Markdown document such as the
  legal-cloud proposal, showing preserved version lineage, migrated source
  entities, historical publish/compare/merge, expandable citations, and
  openable source windows;
- DOCX import proof with original ContentItem, import manifest, VText
  projection, revision, DOCX export, and inspected metadata;
- PDF import proof with original ContentItem, extraction/lossiness manifest,
  VText projection, revision, PDF export, and inspected metadata;
- publish UX proof for copy link, open link, copy text, and download without
  page reload;
- citation marker expansion proof on mobile and desktop;
- embedded excerpt default proof;
- open-source/open-in-window proof for at least one source entity;
- private-to-public publication proof that source entities/transclusions
  survive into the published artifact;
- regression tests for serializer, revision context, import manifests,
  export metadata, source entity rendering, and publication projection.

## Forbidden Shortcuts

- No prompt/content classifier to decide whether the edit is simple.
- No required `//<edit>` or other user-visible meta syntax.
- No hardcoded compare/merge suggestions.
- No replacing computer-use QA with local-only screenshots.
- No table-only patch that leaves the rendered-DOM serializer generally
  fragile.
- No frontend-only DOCX/PDF export.
- No import that discards the original DOCX/PDF.
- No exported file with private owner IDs, access tokens, raw trace, hidden
  agent prompts, or unpublished revision text.
- No visible provenance comments or metadata payloads in the VText body.
- No citation marker without a resolvable source entity or repair state.
- No publication/export path that scrapes rendered DOM instead of canonical
  artifact state.
- No declaring success from API records alone without deployed user-visible
  behavior.

## Rollback Policy

- Keep original imported files immutable and recoverable by content hash.
- Preserve pre-mission VText revisions and publish routes.
- Gate new export formats behind policy/format availability until verified.
- If DOCX/PDF renderer deployment fails, leave TXT/MD/HTML exports intact and
  record the renderer blocker without weakening the canonical export boundary.
- If diff-first revision degrades quality, retain old revisions and use
  diagnosis metadata to compare context, edit operation, and output deltas
  before rollback or prompt/tool adjustment.
- Do not migrate existing `.md` documents destructively; create VText
  projection aliases with lineage.

## Learning Side-Channel

Update the mission document as the run learns:

- root cause records for new failures;
- import/export adapter decisions;
- source entity schema adjustments;
- latency/token before-after evidence;
- computer-use QA observations;
- residual risks and next homotopy axes.

Update canonical docs only when the implementation changes current operating
rules or durable architecture. Tactical evidence belongs in this mission doc or
dated problem/evidence artifacts.

## Stopping Condition

The mission may be marked complete only when staging proves, through product
paths and computer-use QA, that:

- long-document VText revision is diff-first by default and materially faster;
- instruction-bearing user edits are interpreted without meta syntax;
- stale replaced content is removed;
- table/list/source structure survives focus/edit/save/revise/reload;
- non-VText imports become VText projections while originals remain preserved;
- existing versioned Markdown documents migrate into VTexts with preserved
  version lineage and citation/source transclusions;
- DOCX and PDF can complete import -> revise -> export with lineage and
  inspected metadata;
- publish/copy/download/open UX works without reloading the VText app;
- every citation marker tested expands to an inline transclusion;
- embedded excerpt defaults and open-owning-source behavior work;
- version compare/merge remains coherent with source entities and hidden
  provenance;
- diagnosis export can explain latency, context, edit operation, and
  import/source/export lineage for the tested revisions.

If any item is not feasible in this run, stop as `checkpoint_incomplete` with
the exact blocker, evidence, rollback state, and next executable probe. Do not
call a partial implementation complete.

## Run Checkpoint And Resumption State

status: draft

last checkpoint: mission expanded before execution to include fluid editing,
document normalization, import/export roundtrip, and citation transclusion UX.

current artifact state: problem evidence exists for slow VText revision,
Markdown table corruption, durable version numbers, publication/export UX, and
source-entity architecture; implementation for this expanded mission has not
yet begun under this mission doc.

what shipped: nothing from this mission yet.

what was proven: prior investigation proved a table serializer failure mode and
large-context revision path on yusef's long legal-cloud proposal; prior docs
define source entities, transclusion display policy, and DOCX/PDF import/export
research.

unproven or partial claims: fluid diff-first revision, document normalization,
DOCX/PDF roundtrip, and citation expansion/open-source behavior still require
staging product proof.

highest-impact remaining uncertainty: whether current VText edit tools are
sufficient for fast high-quality structured edits, or whether a stronger block
selector operation must be added first.

next executable probe: reproduce the table corruption and slow direct-edit
revision on staging with computer use, document the problem checkpoint, then
repair the serializer and diff-first revision context in separate code commits.

suggested resume goal string: use the Goal String in this document.
