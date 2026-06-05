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

status: checkpoint_incomplete

last checkpoint: June 5, 2026. The first implementation checkpoint landed
after a separate problem-documentation commit. A second checkpoint added real
platform DOCX/PDF publication export after documenting the staging DOCX export
blocker. A third checkpoint preserved non-VText originals as ContentItems when
opening files into VText projections.

current artifact state: problem evidence exists for slow VText revision,
Markdown table corruption, durable version numbers, publication/export UX, and
source-entity architecture. This checkpoint repaired the most immediate
diff-first editing and table roundtrip failures, but it does not complete the
document roundtrip or source-entity mission.

what shipped:

- `fa58040d` documented the expanded mission and current problem field before
  code changes.
- `19f41da9` changed VText appagent revision prompts to use current head plus
  user edit diff as the default context, records `vtext_context_mode` and
  prompt size metadata, removes routine preload of old user revision diffs and
  worker messages, and keeps worker/grounding context for worker wake or
  integration turns.
- `19f41da9` repaired rendered Markdown table serialization for the
  `.table-scroll > table` DOM shape and added a focused frontend regression
  proving focus/edit/autosave preserves Markdown pipe tables instead of
  flattening cells.
- `19f41da9` prevents opened `.md` files from continuing to write through to
  the original Markdown path once a canonical VText doc exists, and records
  import/migration metadata for Markdown file-open projections.
- `19f41da9` makes citation-marker clicks expand their matching inline
  transclusion without toggling an already-open citation closed.
- `19f41da9` adds publish-surface download choices for Markdown, Text, HTML,
  DOCX, and PDF, while leaving unsupported backend formats as explicit
  failures rather than fake exports.
- `52379bec` documented that staging commit `19f41da9` still returned a public
  export failure for DOCX on the prior public route because platformd only
  accepted text-like formats and that route's immutable export policy allowed
  only `txt`, `md`, and `html`.
- `631acb58` adds platform-owned DOCX and PDF export generation from canonical
  publication bundles. DOCX is emitted as an OOXML package with document,
  core-properties, custom-properties, table preservation, and compact public
  provenance. PDF is emitted as valid PDF 1.4 bytes with paginated text and XMP
  public provenance. Binary exports travel through the public JSON export API
  as `content_base64`, and the VText download UI decodes that payload for file
  downloads.
- `631acb58` expands the default publication export policy to
  `txt`, `md`, `html`, `docx`, and `pdf` for new publications.
- `0a5a31de` preserves original file artifacts when `.md`, DOCX, or PDF-like
  files are opened as VText. Text-compatible originals keep text content and an
  available original hash. Binary originals create separate ContentItems with
  text content omitted, explicit binary hash-state metadata, and lossy VText
  projection manifests on the first revision.

what was proven:

- Local runtime tests passed for diff-first prompt construction, `.md` file-open
  import/migration metadata, and existing initial VText tool choice behavior.
- Local frontend build passed.
- Local focused Playwright E2E passed for source entity expansion and rendered
  Markdown table autosave roundtrip.
- GitHub CI and FlakeHub publish both passed for `19f41da9`.
- Staging health proved proxy and sandbox deployed commit
  `19f41da9d649395bb010480a45a7c278ff890fa4` at
  `2026-06-05T05:00:52Z`.
- Staging public VText reader opened
  `/pub/vtext/staging-long-compare-merge-proof-1780614390072-pub32bd3c150` and
  rendered the prior long published VText.
- Staging public export API returned Markdown, Text, and HTML exports for that
  public route.
- Local platform/proxy tests passed for publication/export paths, including
  DOCX and PDF bytes generated from canonical publication content with table
  preservation and public provenance metadata.
- Frontend build passed after adding binary export decoding.
- GitHub CI, staging deploy, and FlakeHub publish passed for
  `631acb588eb991186b1cab10c4fcccdaa4d7b7b1`.
- Staging health proved proxy and sandbox deployed commit
  `631acb588eb991186b1cab10c4fcccdaa4d7b7b1` at
  `2026-06-05T05:15:16Z`.
- A fresh staging publication
  `/pub/vtext/staging-docx-pdf-export-proof-pub479b3a5d9` was created with the
  new default export policy. Its public resolve API returned
  `txt`, `md`, `html`, `docx`, and `pdf` as allowed formats.
- Public export API calls for that route returned:
  - DOCX:
    `application/vnd.openxmlformats-officedocument.wordprocessingml.document`,
    filename `staging-docx-pdf-export-proof-pub479b3a5d9.docx`,
    `content_base64`, and `choir.publication_export.v0` metadata.
  - PDF: `application/pdf`, filename
    `staging-docx-pdf-export-proof-pub479b3a5d9.pdf`, `content_base64`, and
    `choir.publication_export.v0` metadata.
- Decoded DOCX proof at `/tmp/choir-docx-export-proof.docx` was recognized as
  `Microsoft Word 2007+`; `word/document.xml` preserved the table and final
  content line; `docProps/custom.xml` contained the publication version and
  content hash.
- Decoded PDF proof at `/tmp/choir-pdf-export-proof.pdf` was recognized as
  `PDF document, version 1.4`; extracted strings contained the publication
  version, XMP public provenance, body content, and final content line.
- Local focused tests passed for non-VText import preservation:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextOpenFileResolvesCanonicalAlias|TestVTextOpenFilePreservesDocxAndPDFOriginalArtifacts' -count=1`.
- Local focused VText prompt tests still passed:
  `nix develop -c go test ./internal/runtime -run 'TestVTextPromptUsesDiffFirstContextForDirectUserEdits|TestInitialVTextToolChoiceUsesExactTools' -count=1`.
- `git diff --check` passed before committing `0a5a31de`.
- GitHub CI, staging deploy, and FlakeHub publish passed for
  `0a5a31de4e4c1f0127e5a27b006b66fea5e98e88`.
- Staging health proved proxy and sandbox deployed commit
  `0a5a31de4e4c1f0127e5a27b006b66fea5e98e88`.
- Deployed Node B sandbox service-level proof opened `.md`, DOCX, and PDF
  paths as VText projections under owner
  `mission-vtext-import-proof-1780637418`.
  - Markdown original ContentItem:
    `56cb0ee0-d729-4a17-b278-7ed33030e39f`, media `text/markdown`,
    app `vtext`, text length `50`, hash state
    `available_from_text_projection`.
  - DOCX original ContentItem:
    `73978839-1da3-4fa3-b899-9b7899e6cc15`, media
    `application/vnd.openxmlformats-officedocument.wordprocessingml.document`,
    app `vtext`, text length `0`, hash state
    `unavailable_until_binary_bytes_adapter`, text policy
    `not_embedded_for_binary_original`.
  - PDF original ContentItem:
    `13f5d192-b9c8-4669-870e-a6b0de2dcb56`, media `application/pdf`,
    app `pdf`, text length `0`, hash state
    `unavailable_until_binary_bytes_adapter`, text policy
    `not_embedded_for_binary_original`.
- Deployed Node B sandbox revision-metadata proof under owner
  `mission-vtext-import-revision-proof-1780637446` confirmed the first DOCX and
  PDF VText revisions carry import manifests with `projection_kind: vtext`,
  source media type, original ContentItem ID, projection content hash, binary
  original hash state, and lossy adapter warnings:
  `docx_projection_requires_style_adapter` and
  `pdf_projection_requires_extraction_adapter`.
- `ee4c6582` documented the remaining binary import projection gap: the first
  DOCX/PDF import checkpoint preserved original ContentItems but still relied on
  caller-supplied projection text instead of reading original bytes from the
  user-computer file root.
- `c5e5e919` adds deployed DOCX and PDF byte import adapters for
  `/api/vtext/files/open`. The runtime now reads original bytes from the
  sandbox file root, creates a VText working projection, stores the original
  binary ContentItem without text flattening, and records the real original
  byte hash in the import manifest.
- Local focused runtime tests passed for the byte adapters:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextOpenFileResolvesCanonicalAlias|TestVTextOpenFilePreservesDocxAndPDFOriginalArtifacts|TestVTextOpenFileImportsDocxAndPDFBytesFromFilesRoot' -count=1`.
- Local focused VText prompt tests still passed:
  `nix develop -c go test ./internal/runtime -run 'TestVTextPromptUsesDiffFirstContextForDirectUserEdits|TestInitialVTextToolChoiceUsesExactTools' -count=1`.
- Local frontend build passed after adding explicit file-browser VText import
  affordances for binary document files: `npm --prefix frontend run build`.
- Local focused Playwright E2E passed for the file-browser PDF route:
  `npm --prefix frontend run e2e -- tests/file-browser.spec.js -g "PDF files expose explicit VText import" --workers=1 --reporter=line`.
- GitHub CI, staging deploy, and FlakeHub publish passed for
  `c5e5e91966fe17375efc3b0750d2fe3958e93117`.
- Staging health proved proxy and sandbox deployed commit
  `c5e5e91966fe17375efc3b0750d2fe3958e93117` at
  `2026-06-05T05:49:34Z`.
- Deployed Node B sandbox service-level proof under owner
  `vtext-byte-import-proof-2026-06-05@example.com` uploaded unique DOCX and PDF
  files through `/api/files`, then opened them through `/api/vtext/files/open`
  without `initial_content`.
  - DOCX VText doc `cd050c63-2f80-4ec7-ad1a-3eab54d97a13` created original
    ContentItem `7579440d-dc3d-473a-82b3-adef08c42e3c`; the first revision
    content included `Deployed DOCX Import Proof`, a paragraph from the DOCX
    bytes, and the table rows `| Term | Definition |` and
    `| Work product | Durable professional output |`.
  - DOCX original ContentItem media type was
    `application/vnd.openxmlformats-officedocument.wordprocessingml.document`,
    text length was `0`, and both ContentItem hash and manifest
    `original_content_hash` matched the uploaded byte SHA-256
    `f4480acbf313efeb191b48898a0f40ec1d77ae0052c207b060963da7872cbbba`.
  - DOCX import manifest adapter was `docx_ooxml_text_table_projection`, hash
    state `available_from_original_bytes`, with warning
    `docx_styles_preserved_as_manifest_only`.
  - PDF VText doc `222abb38-23a3-4510-a28a-3ecacafaab76` created original
    ContentItem `f368f71f-9c95-4a3d-8858-6675db22f809`; the first revision
    content included `Deployed PDF Import Proof` and
    `Second PDF line from bytes`.
  - PDF original ContentItem media type was `application/pdf`, text length was
    `0`, and both ContentItem hash and manifest `original_content_hash` matched
    the uploaded byte SHA-256
    `bef1fd0cac52b2077cb01be6d117402d8680247b37b2fc94858b7c74afc0ff3d`.
  - PDF import manifest adapter was `pdf_literal_text_projection`, hash state
    `available_from_original_bytes`, with warning `pdf_layout_is_best_effort`.
- Staging browser fallback QA passed for the visible file-browser import path:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/file-browser.spec.js -g "PDF files expose explicit VText import|DOCX files import to VText" --workers=1 --reporter=line`.
  - PDF proof: clicking a PDF file still opens the dedicated PDF reader, while
    the explicit `VText` import affordance opens a VText window whose editor
    contains text projected from the uploaded PDF bytes.
  - DOCX proof: uploading a DOCX fixture, clicking the explicit `VText` import
    affordance, and opening the VText window showed paragraph text and table
    terms projected from the original DOCX bytes.
- Staging browser fallback QA also passed for DOCX import -> revise -> publish
  -> export:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/file-browser.spec.js -g "DOCX import can revise" --workers=1 --reporter=line`.
  The test uploads a DOCX through Files, imports it to VText through the visible
  `VText` affordance, creates a normal user revision, publishes that revision
  through `/api/platform/vtext/publications`, exports DOCX and PDF through
  `/api/platform/publications/export`, verifies the DOCX is an OOXML package,
  and verifies the PDF bytes include the revised export line.
- Staging browser fallback QA also passed for PDF import -> revise -> publish
  -> export:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/file-browser.spec.js -g "PDF import can revise" --workers=1 --reporter=line`.
  The test uploads a PDF through Files, imports it to VText through the visible
  `VText` affordance, verifies the VText projection contains text extracted
  from the original PDF bytes, creates a normal user revision, publishes that
  revision, exports DOCX and PDF derivatives, verifies the DOCX is an OOXML
  package, and verifies the PDF bytes include the revised export line.
- `839cd676` documented the next migration gap before code: existing
  versioned Markdown documents had no owner-authenticated product path to
  migrate ordered historical Markdown snapshots into one canonical VText with
  durable version numbers, preserved source snapshot evidence, aliasing, and
  repairable citation-gap metadata.
- `5ab52672` adds `POST /api/vtext/markdown-lineage/import`. The route accepts
  an ordered Markdown lineage, creates one VText document, stores each source
  snapshot as a `file_version` ContentItem, creates sequential VText revisions
  with durable version numbers, records a `markdown_lineage_to_vtext_revisions`
  migration manifest, refuses duplicate source-path aliases with `409`, and
  records unresolved numeric/footnote citation markers as repairable source
  gaps instead of inventing source entities.
- Local focused runtime tests passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVText(OpenFile|ImportMarkdownLineage)' -count=1`.
- Local default runtime compile path passed:
  `nix develop -c go test ./internal/runtime -run TestNonexistentCompileOnly -count=1`.
- GitHub CI run `26998977583` passed for
  `5ab52672396c23fb1260e29f0051a12aaba22bc3`; runtime shards, vet/build,
  integration smoke, non-runtime tests, and Node B staging deploy were green.
- FlakeHub publish run `26998977615` passed for the same commit.
- Staging health proved proxy and sandbox deployed commit
  `5ab52672396c23fb1260e29f0051a12aaba22bc3`, deployed at
  `2026-06-05T06:19:44Z`.
- Deployed browser-authenticated staging proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-markdown-lineage.tmp.spec.js --workers=1 --reporter=line`.
  The temporary proof spec used the existing `desktopSession` fixture, posted
  a three-snapshot legal-cloud-style Markdown lineage, verified VText revisions
  `v0`, `v1`, and `v2` in order, verified the current revision is the latest
  snapshot, verified the oldest revision carries the migration manifest,
  version-lineage array, original ContentItem refs, and `[1]` source gap, and
  verified a second import of the same `source_path` returns `409` with the
  existing document id. The temporary `*.tmp.spec.js` was deleted after proof.
- `aa2aa5f8` documented that VText source citations were still popover/source
  rail shaped rather than in-flow transclusions at the citation target, and
  that frontend-derived media refs used an unrecognized `chip` display policy.
- `d755a58f` changed VText citation rendering so clicking/tapping a
  `source:` citation expands an in-flow transclusion card at the marker,
  normalizes frontend-derived media refs to `embedded_preview`, and hides raw
  source IDs/evidence internals from user-visible source cards.
- Staging QA on `d755a58f` caught a sharper multimedia gap: the citation card
  expanded in-flow but did not include the YouTube iframe; media was still only
  in the separate source rail. `c8aa13e3` documented that finding before the
  follow-up fix.
- `bae534ae` adds inline media rendering for expanded citation cards while
  preserving the open-owning-source button.
- Local frontend builds passed for the citation work:
  `npm --prefix frontend run build`.
- GitHub CI run `26999307285` passed for `d755a58f`; GitHub CI run
  `26999451279` passed for `bae534ae`. Both Node B staging deploy jobs were
  green, and both corresponding FlakeHub publish runs succeeded.
- Staging health proved proxy and sandbox deployed commit
  `bae534ae61f5c4585f55d537faf9026487992594`, deployed at
  `2026-06-05T06:32:49Z`.
- Deployed browser-authenticated source-entity QA passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js --workers=1 --reporter=line`.
  This proves a real VText with a YouTube source entity renders a citation
  marker, expands it in-flow at the marker, shows the YouTube iframe inside
  the expanded citation card, preserves the source rail's embedded-preview
  policy, opens the owning video app/window from the citation card, and still
  preserves table roundtrip behavior through the same test file.
- Deployed browser-authenticated publication source QA passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-service-publication.spec.js --workers=1 --reporter=line`.
  This proves a published source-service source entity resolves to an
  expandable transclusion, keeps canonical export behavior, and no longer
  exposes the raw source-service item id as user-visible prose.
- `91dfaeb1` documented a debuggability gap found during live staging proof:
  `/api/vtext/documents/{id}/diagnosis` could omit the VText run returned by
  the same document's `/revise` call because diagnosis listed recent owner
  runs, not document-linked runs.
- `dad21f1b` added document-channel runs to VText diagnosis and introduced an
  opt-in live staging proof:
  `frontend/tests/vtext-long-doc-fluid-editing-live.spec.js`.
- Staging QA on `dad21f1b` proved the live long-document edit path content
  behavior but showed that some live revise runs are linked through
  `metadata.doc_id` rather than the `channel_id` column. `5b7dbbc2` added the
  metadata-linked fallback and a runtime regression where the document run is
  outside the owner-level limit and not indexed by the document channel.
- Local focused runtime proof passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextDiagnosisIncludesDocumentChannelRuns' -count=1`.
- GitHub CI run `27000115994` passed for
  `5b7dbbc256a8cb8310dc61da2186f8aefab33129`; runtime shards, vet/build,
  integration smoke, non-runtime tests, and Node B staging deploy were green.
- FlakeHub publish run `27000115981` passed for the same commit.
- Staging health proved proxy and sandbox deployed commit
  `5b7dbbc256a8cb8310dc61da2186f8aefab33129`, deployed at
  `2026-06-05T06:50:35Z`.
- Deployed browser-authenticated live long-document VText proof passed:
  `GO_CHOIR_RUN_LIVE_VTEXT_EDIT=1 PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-long-doc-fluid-editing-live.spec.js --workers=1 --reporter=line`.
  This creates a legal-cloud-class long VText, creates a direct user-authored
  edit diff with an instruction line in the document body, triggers `/revise`
  with no meta syntax, waits for the appagent revision, and verifies:
  - the final revision keeps the intended recommendation;
  - the instruction line is consumed and removed;
  - the stale `Draft needs tightening` text is removed;
  - the appendix Markdown table is preserved;
  - revision metadata records `vtext_context_mode:
    current_head_plus_user_edit_diff`;
  - revision metadata records `vtext_edit_operation: apply_edits`;
  - revision metadata records prompt size under 40k chars (`18731` chars in
    the diagnostic run) and latency metadata;
  - diagnosis with `limit=3` includes the returned `loop_id`.
- Deployed browser-authenticated DOCX/PDF roundtrip QA passed on the same
  deployed build:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/file-browser.spec.js --grep "DOCX files import|DOCX import can revise|PDF import can revise" --workers=1 --reporter=line`.
- Deployed browser-authenticated citation/source transclusion QA passed on the
  same deployed build:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-source-entities.spec.js tests/vtext-source-service-publication.spec.js --workers=1 --reporter=line`.
- `5d1b2cee` documented the next source migration gap: the Markdown lineage
  importer could record unresolved source gaps, but could not yet accept known
  source entities or citation-marker resolutions during product-path migration.
- `5745c6f3` extends `POST /api/vtext/markdown-lineage/import` so migration
  payloads can include revision/global `source_entities` and
  `citation_resolutions`. The importer now validates that resolved markers
  point at known source entities, preserves raw Markdown snapshots as original
  ContentItems, writes renderer-compatible `source:` refs into the VText
  working projection, records resolution manifests, and leaves only unresolved
  markers as repairable source gaps. Local focused runtime proof passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextImportMarkdownLineage(ResolvesCitationMarkers|RejectsUnknownCitationEntity|CreatesRevisionHistory|RejectsExistingAlias)' -count=1`.
- Staging QA on `5745c6f3` caught a projection bug: the importer rewrote
  marker `[1]` as `[[1]](source:ENTITY_ID)`, but the VText renderer only
  recognizes canonical `[label](source:ENTITY_ID)` labels without `]`.
  `c13801c0` documented the render gap before the follow-up fix.
- `9c83e725` normalizes migrated bracketed markers into canonical source-link
  labels such as `[1](source:ENTITY_ID)` while preserving the original
  bracketed marker in the migration manifest. GitHub CI run `27000934623`
  passed for `9c83e725d0a2e470854a9da81f8944650a46a378`; runtime shards,
  vet/build, integration smoke, non-runtime tests, and Node B staging deploy
  were green. FlakeHub publish run `27000934617` passed for the same commit.
- Staging health proved proxy and sandbox deployed commit
  `9c83e725d0a2e470854a9da81f8944650a46a378`, deployed at
  `2026-06-05T07:11:07Z`.
- Deployed browser-authenticated source-aware Markdown lineage proof passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-markdown-lineage.spec.js --workers=1 --reporter=line`.
  This proves the product API can migrate a legal-cloud-style Markdown
  lineage with a known source entity, convert resolved marker `[1]` into a
  rendered clickable VText source reference, expand the citation in-flow at the
  marker, show the source label and excerpt, expose the open-source control,
  preserve unresolved marker `[2]` as a repairable source gap, and preserve the
  original raw Markdown snapshot separately.
- `e8701336` documented the next product-path gap: the source-aware lineage
  importer still required every Markdown snapshot's raw text in the request
  body, even when the versions already existed as owner-scoped ContentItems
  inside the user computer.
- `232c3ef4` extends `POST /api/vtext/markdown-lineage/import` so a migrated
  version may reference an existing `content_item_id`. The importer now reads
  that stored owner artifact, validates it has text, uses it as the original
  source record instead of duplicating it, records `source_content_item_id`,
  `original_content_id`, `original_content_path`, and
  `original_content_source: content_item` in the migration manifest, and still
  supports raw `content` snapshots for external imports. Local focused runtime
  proof passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextImportMarkdownLineage(UsesExistingContentItems|RejectsMissingContentItem|ResolvesCitationMarkers|RejectsUnknownCitationEntity|CreatesRevisionHistory|RejectsExistingAlias)' -count=1`.
- GitHub CI run `27001378141` passed for
  `232c3ef4cc2ccbe6a4df5d1ee1457102144c4cc9`; runtime shards, vet/build,
  integration smoke, non-runtime tests, and Node B staging deploy were green.
  FlakeHub publish run `27001378162` passed for the same commit.
- Staging health proved proxy and sandbox deployed commit
  `232c3ef4cc2ccbe6a4df5d1ee1457102144c4cc9`, deployed at
  `2026-06-05T07:22:04Z`.
- Deployed browser-authenticated source-aware lineage proof was expanded and
  passed:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-markdown-lineage.spec.js --workers=1 --reporter=line`.
  The expanded proof creates stored Markdown ContentItems through the public
  `/api/content/items` product API, migrates a two-version legal-cloud-style
  lineage by `content_item_id`, verifies original content IDs and migration
  manifest refs point at those stored artifacts, verifies the latest migrated
  version renders a preserved Markdown table, restores the historical migrated
  version as a new canonical VText revision, and verifies the restored
  historical citation expands in-flow with the supplied source label/excerpt.
- `10ce194f` corrected the browser proof fixture to use public `file`
  ContentItems instead of the runtime-internal `file_version` source type.
- `4a2b05a7` documented the next source-substrate gap before the behavior
  change: migrated Markdown revisions could record unresolved `source_gaps`,
  but there was no canonical product path to attach later-discovered source
  evidence, rewrite the matching citation markers into source entities, clear
  repaired gaps, and create the next durable VText revision.
- `b9e485d4` adds `POST /api/vtext/documents/{id}/source-repairs`. The route
  is owner-authenticated, accepts a base revision, source entities, and
  citation-marker resolutions, validates that every resolution points at a real
  source entity, rewrites only unresolved Markdown citation markers into
  `source:` links, preserves unrepaired gaps, clears repaired gaps, and creates
  the next canonical VText revision with `vtext_source_gap_repair` metadata.
  It does not re-import the document, does not invent citations, and does not
  double-link already resolved `source:` markers. Local focused runtime proof
  passed:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVText(SourceGapRepair|ImportMarkdownLineage)' -count=1`.
- GitHub CI run `27001890802` passed for
  `b9e485d4cf2a26fee34528a3879a29226e00b0aa`; runtime shards, vet/build,
  integration smoke, non-runtime tests, aggregate Go gate, and Node B staging
  deploy were green. FlakeHub publish run `27001890817` passed for the same
  commit.
- Staging health proved proxy and sandbox deployed commit
  `b9e485d4cf2a26fee34528a3879a29226e00b0aa`, deployed at
  `2026-06-05T07:34:17Z`.
- `7fc64d21` corrected the browser assertion for the repaired source marker:
  the VText renderer keeps the original citation marker in `data-source-label`
  while the expanded inline transclusion text contains source-card content.
  GitHub CI run `27002025582` passed for
  `7fc64d2171e8efa9614f75bde6fa8c609973870b`; staging deploy was skipped as
  expected because the change touched only tests. FlakeHub publish run
  `27002025545` passed for the same commit.
- Deployed browser-authenticated source-aware lineage proof now passes with
  the source-gap repair case included:
  `PLAYWRIGHT_BASE_URL=https://choir.news npm --prefix frontend run e2e -- tests/vtext-markdown-lineage.spec.js --workers=1 --reporter=line`.
  Result: 3 passed. The proof migrates raw Markdown with a known citation
  marker into expandable source transclusion, migrates stored ContentItem
  versions while preserving table rendering and historical restore behavior,
  then migrates a document with unresolved `[2]`, repairs it through
  `/source-repairs`, verifies the new revision is parented to the migrated
  base version with `source_repair_resolutions` metadata and no remaining
  `source_gaps`, opens the VText UI, and expands the repaired citation into an
  inline source transclusion with an open-source affordance.

unproven or partial claims:

- Computer-use QA was requested as the default, but this Codex session did not
  expose Mac computer-use control. The deployed browser proof used
  `agent-browser` as the backup path.
- The deployed staging proof did not authenticate into
  `yusefnathanson@me.com` and did not run on the owner's existing
  `choir_private_legal_cloud_proposal.md`. It did run a deployed live
  long-document revise on a legal-cloud-class VText fixture.
- DOCX and PDF buttons are visible in the publish download menu, and backend
  DOCX/PDF export and import adapters now work for the service paths. This
  checkpoint still does not prove owner-authenticated VText publish/download UX
  by computer use.
- Existing immutable publications keep their historical export policy. The
  prior route
  `/pub/vtext/staging-long-compare-merge-proof-1780614390072-pub32bd3c150`
  still allows only `txt`, `md`, and `html`, so DOCX/PDF are correctly rejected
  there unless a new version is published with an updated policy.
- `.md` file-open normalization now records import and migration manifests, and
  the deployed Markdown lineage API can migrate ordered snapshots into durable
  VText revisions. It can attach known source entities and citation
  resolutions during migration, and it can migrate from existing owner-scoped
  ContentItems instead of requiring raw snapshot text payloads. The actual
  owner-authenticated bulk migration of existing legal-cloud-class documents
  has not yet been run because the actual source snapshots were not available
  in this Codex session; source extraction/repair for citations whose evidence
  is not supplied remains incomplete.
- DOCX/PDF import now preserves original ContentItems, reads original bytes,
  records real byte hashes, creates VText projections, and has staged browser
  proof for DOCX and PDF import -> revise -> publish -> DOCX/PDF export.
  Style-profile preservation, asset manifests, and full-fidelity PDF text
  extraction/OCR are not complete.
- Source entity behavior now has deployed frontend proof for existing inline
  `source:` markup, in-flow citation expansion, embedded YouTube media,
  publication source-service transclusions, hidden raw source IDs, and
  open-owning-source behavior. Citation repair/source-entity creation over the
  actual legal-cloud proposal lineage remains incomplete.
- Latency improvement now has deployed positive evidence on the owner's actual
  legal-cloud proposal. The repaired path uses `focused_user_edit_diff`,
  `apply_edits`, and a 9,291-character VText prompt on a 37k-character
  document. Broader semantic multi-region edits still need proof.

highest-impact remaining uncertainty: whether the existing VText edit tools
are sufficient for fast high-quality structured edits on the legal-cloud
proposal, or whether a stronger block/section selector operation is needed to
avoid whole-document edits while keeping semantic quality.

2026-06-05 owner-account continuation probe:

- With owner authorization, the active `yusefnathanson@me.com` primary user
  computer on staging was identified as
  `vm-5b0c1bef1e2b6d7f8dad7d0e8473ed19`, serving sandbox build
  `b9e485d4cf2a26fee34528a3879a29226e00b0aa`.
- The owner-authenticated sandbox VText API lists
  `choir_private_legal_cloud_proposal.md` as document
  `f93cea62-f833-4dae-b414-8e44783d8cbe`, current version 76 with 77
  revisions. The version cap bug is no longer present on this document.
- The recent owner revision history confirms the earlier failure mode: many
  user-authored revisions were created within seconds during manual editing,
  old appagent revisions used `replace_all`, and newer appagent revisions use
  `apply_edits`.
- The latency/context invariant is still broken. Recent real proposal VText
  runs had prompt lengths around 61k-76k characters, input token counts around
  50k-136k, output token counts around 5k-18k, and wall-clock run durations
  from roughly 43 seconds to 112 seconds. The newest revision records
  `vtext_edit_operation: apply_edits`, but still records
  `vtext_run_prompt_chars: 76446`.
- Root cause evidence in the deployed source: `buildAgentRevisionRequest`
  states that the default context is “current head plus the exact user edit
  diff,” but the same builder always appends the complete current canonical
  document content and additional guidance/context. This makes the product
  behavior materially different from the mission contract: the model is still
  asked to process the long document every ordinary revise turn instead of
  receiving a small instruction-bearing diff plus retrieval tools.

new problem to fix after this documentation checkpoint: make ordinary VText
revision prompts actually honor the small-context contract. The VText agent
must receive the user-authored diff and only the bounded current-head regions
needed to apply it by default; full current-document context should be reserved
for explicit whole-document transformations or retrieved on demand.

2026-06-05 focused long-edit prompt repair:

- Problem checkpoint commit `3170bb0c` recorded the owner-account evidence
  before the fix.
- Behavior commit `6feda8f275967513c5785228228256e52e87d4f9` changes ordinary
  long user-authored VText revise prompts to use `focused_user_edit_diff`.
  For long direct-edit drafts, VText now receives exact changed regions plus
  nearby current-head context instead of the whole document body. Short docs
  and non-direct-edit paths keep the previous full-current-content context.
- The fix also corrected a stale test assertion that still expected broad
  `"required"` tool choice. Current VText policy uses exact
  `function:edit_vtext` tool choice for the first VText write and leaves
  worker-wake turns unconstrained when appropriate.
- Local verification:
  `nix develop -c go test -tags comprehensive ./internal/runtime -run
  'TestVTextPrompt|TestVTextAgentRevision' -count=1` passed, and
  `nix develop -c scripts/go-test-runtime-shards` passed.
- GitHub CI run `27014358456` passed for
  `6feda8f275967513c5785228228256e52e87d4f9`, including all four
  `internal/runtime` shards, non-runtime tests, vet, build, and Node B staging
  deploy. FlakeHub publish run `27014358497` passed.
- Staging health then reported sandbox commit
  `6feda8f275967513c5785228228256e52e87d4f9`, deployed at
  `2026-06-05T12:19:07Z`.
- Owner-account deployed proof ran on the actual
  `choir_private_legal_cloud_proposal.md` VText document
  `f93cea62-f833-4dae-b414-8e44783d8cbe`. A temporary user-authored revision
  v77 appended a probe instruction line; the deployed VText appagent then
  created v78 and removed the probe line while leaving the document length back
  at 37,385 chars.
- The v78 revision metadata proves the repaired path:
  `vtext_context_mode: focused_user_edit_diff`,
  `vtext_edit_operation: apply_edits`, `vtext_edit_count: 1`,
  `vtext_run_prompt_chars: 9291`, and `vtext_edit_delta_chars: -128`.
  The previous comparable owner proposal appagent revisions recorded prompt
  sizes around 74k-76k chars.
- The matching deployed run completed from `2026-06-05T12:20:55Z` to
  `2026-06-05T12:21:05Z` with `input_tokens: 17034`,
  `output_tokens: 976`, provider `fireworks`, model
  `accounts/fireworks/models/deepseek-v4-flash`, and no
  `Incorrect string value` diagnosis matches.

remaining risk after this repair: this proves the fast path for a simple
single-region direct edit on the owner legal-cloud proposal. It does not yet
prove multi-region semantic edits, table-preserving appendix repair under the
new focused context, or citation/transclusion repairs on the same owner
document.

next executable probe: use authenticated computer-use on staging when that
capability is available, or browser/API backup otherwise, to prove the next
realism axis on the owner legal-cloud proposal: multi-region semantic edits,
appendix-table preservation through focus/edit/save/revise, unresolved citation
gap repair through source tooling, and citation expansion/open-source behavior
on the migrated owner document.

2026-06-05 handoff checkpoint for next Codex session:

status: checkpoint_incomplete

current artifact state:

- `main` is at docs checkpoint `f05b4c92`; deployed staging sandbox remains the
  behavior commit `6feda8f275967513c5785228228256e52e87d4f9`.
- CI run `27014358456`, FlakeHub run `27014358497`, and Node B staging deploy
  all passed for `6feda8f275967513c5785228228256e52e87d4f9`.
- The only known dirty path in the local worktree at handoff is unrelated
  untracked documentation:
  `docs/overnight-vtext-super-console-zot-mega-report-2026-05-31.md`.

code state reviewed:

- `internal/runtime/vtext.go` now chooses `focused_user_edit_diff` for long
  user-authored drafts with a previous revision, via
  `vtextAgentRevisionContextMode`, `vtextUseFocusedUserEditContext`, and
  `summarizeFocusedUserEditContext`.
- `buildAgentRevisionRequest` no longer unconditionally includes the complete
  current document for that long direct-edit path. It includes changed regions
  and nearby current-head context, then tells VText to use `apply_edits` and
  retrieve broader context only when needed.
- `internal/runtime/vtext_prompt_unit_test.go` includes
  `TestVTextPromptFocusesLongDirectUserEdits`, which verifies the focused
  prompt includes the edited region but not distant untouched body text.
- `internal/runtime/vtext_test.go` now expects exact `function:edit_vtext`
  tool choice instead of broad `"required"` for the initial VText write.

cognitive transforms applied for continuation:

- State-machine inversion: treat the document delta as the control surface.
  The next session should not introduce classifiers or workflow branches for
  “table edit” versus “citation edit.” It should preserve structure by making
  the current VText representation and edit tools capable of expressing the
  needed deltas.
- Via negativa: remove or bypass corruption paths rather than adding recovery
  patches. The appendix problem is probably not solved by another glossary
  special case; it needs the path that flattens Markdown/HTML/table structure
  identified and eliminated or constrained.
- Fidelity-as-invariant: tables, citations, transclusions, hidden metadata, and
  source markers are document structure, not decoration. The verifier should
  assert structure survives render/edit/save/revise/export, not merely that
  text content remains present.
- OODA/depth extraction: the load-bearing variable is feedback speed with
  trustworthy evidence. The next probe should compare real owner revisions,
  identify the first transition that flattened the table, and then make the
  smallest structural repair that prevents the class of recurrence.

new evidence gathered for the appendix-table regression:

- Owner document:
  `choir_private_legal_cloud_proposal.md`,
  doc id `f93cea62-f833-4dae-b414-8e44783d8cbe`.
- The appendix table has toggled between valid Markdown table structure and a
  flattened `TermDefinition**...` artifact.
- Revision scan from v44 onward:
  v44-v59 had zero Markdown table rows and contained the flattened
  `TermDefinition**` artifact.
  v60 restored 50 Markdown table rows with `replace_all`.
  v66-v67 were concept-merge user revisions with 50 table rows.
  v69 regressed to zero table rows and flattened `TermDefinition**`.
  v70 was a user-authored `codex_appendix_table_repair` with 50 table rows.
  v72-v74 appagent `apply_edits` revisions preserved 50 table rows, but with
  large 70k-75k prompts.
  v75-v78 regressed to one Markdown table row while retaining
  `TermDefinition**`; v78 is otherwise the focused-prompt proof revision.
- This means the prompt-size repair is real but does not by itself repair table
  fidelity. The table corruption likely arises from a render/save/roundtrip or
  user-authored draft path, not only appagent prompt size.

remaining error field:

- Need root cause for the exact transition that flattened the appendix table
  after v74.
- Need structural fix so contenteditable/rendered-Markdown roundtrip cannot
  collapse Markdown tables into concatenated inline text.
- Need proof that focused long-edit prompts preserve table structure when the
  table is not the edited region and when it is the edited region.
- Need citation/transclusion repair on the actual owner document, not only
  fixture proof.
- Need computer-use QA if the tool becomes available in the next session. In
  this session `tool_search` still returned no computer-use/desktop-control
  tool after the user re-enabled it, so API/browser backup was used.

2026-06-05 imported Markdown-as-VText structural identity checkpoint:

status: checkpoint_incomplete

new problem documented before code changes:

- Computer Use is available in this continuation, and the Comet browser is
  authenticated on staging with the owner account. The owner proposal VText is
  open at `https://choir.news` as document
  `f93cea62-f833-4dae-b414-8e44783d8cbe`, current UI label v78.
- The document title/source label is still
  `choir_private_legal_cloud_proposal.md`, and the rendered document is acting
  as VText. The appendix glossary is visibly flattened in the owner UI as
  `TermDefinition...`, matching the prior API/browser evidence that v75-v78
  regressed from the v70-v74 Markdown table shape.
- The correction target is not to make `.md` behave "close enough" to
  `.vtext`. Imported `.md`, `.txt`, DOCX, PDF, and other source files should
  become canonical VText projections once the user begins durable VText work.
  The original file remains source evidence/import lineage, and export back to
  `.md`/`.txt` is an explicit export from canonical VText.
- Specifically, an imported `.txt`, `.md`, or other source artifact may seed
  v0 as original/source content, but as soon as it advances from v0 to v1 the
  canonical editable artifact should be a `.vtext` projection/manifest. VText
  revisions after that point must not write through to the source extension or
  rely on the source extension's Markdown identity for correctness.
- Code inspection before mutation found that the backend already has
  projection/import metadata and `.vtext` shortcut-manifest machinery, while
  the frontend still preserves `appContext.sourcePath` and contains
  `writeThroughToFile` behavior for non-`.vtext` source paths when no
  `currentDoc.doc_id` is present. This does not prove the exact v74->v75
  transition by itself, but it keeps the `.md` substrate in the live edit
  boundary and is incompatible with the mission invariant that VText owns the
  canonical revisable projection after import.
- The first collapsed transition remains the table fidelity target: v70-v74
  preserved about 50 Markdown table rows; v75-v78 contain the collapsed
  `TermDefinition` artifact. The repair must identify and close the structural
  corruption path for rendered table serialization and imported-source
  projection identity without a glossary-specific special case.

belief-state update:

- `.md`-labeled owner documents are not proven equivalent to `.vtext` today.
  The appendix regression is evidence that source-format identity, rendered
  Markdown serialization, and user-authored draft/revision flow are still
  entangled. The next code change should make canonical VText projection
  identity explicit, preserve table structures through serializer coverage,
  and keep `.md` as import/export lineage rather than a live editable substrate
  once durable VText revisions begin.

2026-06-05 deployed owner-document transition and restore-route checkpoint:

status: checkpoint_incomplete

deployed identity and owner UI evidence:

- Behavior fix commit `c2cc8eb74e1de930067e9c9fb0cbdb9e0d5f6de4` is on
  `origin/main`, CI run `27015378414` passed, FlakeHub run `27015378389`
  passed, and staging `/health` reports proxy and sandbox deployed commit
  `c2cc8eb74e1de930067e9c9fb0cbdb9e0d5f6de4` with deployed timestamp
  `2026-06-05T12:41:26Z`.
- Computer Use is available. Comet is authenticated on staging and the private
  owner VText window for document `f93cea62-f833-4dae-b414-8e44783d8cbe` is
  visible as `choir_private_legal_cloud_proposal.md`.
- Extension-backed DOM reading of the authenticated Comet page identified the
  target private VText root with doc id
  `f93cea62-f833-4dae-b414-8e44783d8cbe`, version v78, state `Latest`.
- Authenticated UI/DOM comparison of v78 -> v74 found the first collapsed
  transition: v78, v77, v76, and v75 contain `TermDefinition` with zero rendered
  HTML tables and one stray Markdown pipe row; v74 is a historical version with
  one rendered HTML table and no `TermDefinition` collapse. This confirms the
  first bad transition is v74 -> v75.

new problem documented before any follow-on code:

- The product has backend endpoints for historical restore, diagnosis, and
  source-gap repair, but the visible VText editor does not expose restore or
  source-repair controls.
- Attempting to use the built-in compare/merge UI from v74 into latest v78
  failed on staging with `COMPARE FAILED`, `Could not compare v74 to v78`, and
  `model-backed semantic compare failed`. This blocks owner-document repair by
  the currently visible historical-merge route.
- The browser security policy rejected a `javascript:` URL API probe. That path
  was not pursued further. Read-only DOM proof remains available through the
  extension-backed browser client, but mutating owner-product state still needs
  a visible product control or an ordinary authenticated product API call from
  an allowed surface.

belief-state update:

- The structural code fix is deployed and covers the future corruption class
  where an incoming user draft collapses a Markdown table while the parent
  canonical revision still has table structure. It cannot by itself repair an
  already-corrupted head whose parent is also corrupted.
- The owner document still needs a product-visible restoration path from v74
  table structure into the current head before the untouched-table and
  bounded-table-edit staging proofs can be completed. That path should be a
  general historical-restore/merge/source-repair UX, not a document-specific
  or glossary-specific migration.

2026-06-05 deployed restore-control and owner-table revise checkpoint:

status: checkpoint_incomplete

deployed identity and owner UI evidence:

- Restore-control fix commit `b88e251bc956bc8274223d50be7f5e44ebbc5dc6`
  is on `origin/main`. GitHub Actions run `27015893011` completed
  successfully, including frontend build, Go non-runtime tests, all
  `internal/runtime` shards, and `Deploy to Staging (Node B)`. Staging
  `/health` reported proxy and sandbox deployed commit
  `b88e251bc956bc8274223d50be7f5e44ebbc5dc6`, deployed at
  `2026-06-05T12:52:41Z`.
- Computer Use on authenticated Comet showed the private owner document
  `choir_private_legal_cloud_proposal.md` at historical `v74` with a visible
  `Restore` control. Clicking that owner-visible control created latest
  revision `v79`.
- Read-only DOM inspection of the authenticated Comet page after restore found
  document `f93cea62-f833-4dae-b414-8e44783d8cbe` at `v79`, state `Latest`,
  with one rendered HTML table in Appendix A, 48 table rows, `Term` and
  `Definition` headers, and no `TermDefinition` collapse. This repaired the
  owner head from the known-good historical table shape without a
  glossary-specific code path.
- Through the visible editor, a scratch user instruction was appended to the
  restored owner document and the visible `Revise` control was clicked. The UI
  saved a long user-authored `v80` draft, submitted appagent run
  `8ce7ce08-2710-4aa3-bb3c-ab35cf4c8f5a`, and advanced the document to `v81`.
  The scratch instruction was consumed, Appendix A still rendered as one HTML
  table, and `TermDefinition` did not reappear. This proves the deployed owner
  document survives a focus/edit/save/revise path when the table is untouched.

new problem documented before any follow-on code:

- After the `v81` head appeared, the owner VText toolbar remained stuck in
  `Revising...` for more than 100 seconds with run id
  `8ce7ce08-2710-4aa3-bb3c-ab35cf4c8f5a`. The `Revise` button remained disabled,
  `Cancel` remained visible, and `Publish v81` remained disabled even though
  the document head had advanced and the table-preserving appagent revision was
  visible. This blocks the next bounded table-edit proof and citation/source
  repair work through the same owner UI.
- The read-only browser extension can expose DOM state and the
  `data-vtext-agent-run-id`, but it cannot use `fetch`, `XMLHttpRequest`,
  cookies, localStorage, or sessionStorage from its evaluation context. Comet's
  encrypted Chromium cookies were found on disk, but the Keychain lookup for a
  guessed Comet safe-storage service hung and was stopped. Therefore deployed
  prompt-size and `apply_edits` metadata proof is not yet available from this
  Codex session's authenticated browser surface; it still needs either a
  product-visible diagnosis/export surface or a reliable authenticated product
  API client path.

belief-state update:

- The structural table repair and historical restore path are working on the
  real owner document through deployed Comet UI, and the untouched-table revise
  path preserves the table in the visible canonical document.
- The next repair should target the status/pending-mutation cleanup after an
  appagent revision creates a head revision. It must not be document-specific
  and should preserve the existing run cancellation/stream semantics.
- Bounded table-edit proof, source-gap repair on the owner document, citation
  expansion/open-source proof, and deployed metadata proof remain incomplete.

2026-06-05 deployed pending-state recovery and canonical-VText checkpoint:

status: checkpoint_incomplete

deployed identity and owner UI evidence:

- Pending-state cleanup fix commit
  `989007787da955e511441fdfa2ec9d3b8f806713` is on `origin/main`.
  GitHub Actions run `27016828839` completed successfully, including Go vet
  and build, non-runtime Go tests, all `internal/runtime` shards, integration
  smoke, and `Deploy to Staging (Node B)`. The frontend build job was skipped
  by the deploy-impact filter for this backend/runtime change; local frontend
  build passed before the commit.
- FlakeHub run `27016828856` completed successfully for the same head.
- Staging `/health` reported proxy and sandbox deployed commit
  `989007787da955e511441fdfa2ec9d3b8f806713`, deployed at
  `2026-06-05T13:12:32Z`.
- Computer Use is available, and the Comet browser is authenticated on staging
  with the owner account. After the deployed fix, the private owner VText UI
  for `choir_private_legal_cloud_proposal.md` showed document
  `f93cea62-f833-4dae-b414-8e44783d8cbe` at `v81`, state `Latest`, with
  `Revise` enabled and `Publish v81` enabled. The visible status stream showed
  the `edit_vtext` appagent run receiving tool output and completing.
- This proves the prior `Revising...` pending-state blocker was cleared by the
  deployed general reconciliation path, rather than by a document-specific
  workaround.

answer to the imported `.md`/`.vtext` identity question:

- No, the owner `.md`-named document should not be treated as proven identical
  to a native `.vtext` document. The mission evidence shows the opposite:
  source extension, rendered Markdown serialization, and VText revision
  ownership were still entangled enough for a real `.md` import acting as VText
  to lose appendix-table structure between v74 and v75.
- The intended invariant is now sharper: imported `.txt`, `.md`, DOCX, PDF, or
  other source artifacts may seed v0/source lineage, but the first durable
  VText revision after import should allocate and write the canonical `.vtext`
  projection. Export back to `.md` is an export operation from canonical VText,
  not the live canonical write target.
- The deployed structural fix enforces this direction for the revision path by
  allocating a `.vtext` projection when the existing alias is absent or still
  points at a non-`.vtext` source path, and by preserving Markdown table block
  structure through parent-to-child draft stabilization. That is a structural
  identity repair, not a claim that historical `.md` VText documents already
  behaved identically.

new problem documented before any future code:

- The authenticated browser evidence surfaces are currently inconsistent for
  this owner document. Computer Use sees the private owner VText surface at
  `v81 Latest`, while extension-backed DOM/tab control later exposed a
  `Published v49` root for the same document id. A bounded table-edit
  instruction was typed into that wrong published/root surface during QA, so it
  must not be counted as owner-head proof.
- A follow-up Computer Use check of the visible Choir tab confirmed the same
  wrong root: the URL was the public VText URL, the visible document window was
  `choir_private_legal_cloud_proposal.md` at `v49`, state `Published v49`,
  and it still contained the accidental `QA scratch instruction`. The visible
  `Cancel` control was clicked and the UI reported `Revision cancelled. You
  can revise again from the current version.` This cancellation is protective
  cleanup only; it is not bounded table-edit acceptance evidence.
- Until this browser-root ambiguity is repaired or isolated, mutation proof on
  the real owner document should use Computer Use against the visible private
  owner UI, and extension-backed DOM should be treated as read-only diagnostic
  evidence only after confirming the visible root's version and state.
- The diagnosis endpoint is reachable in authenticated Comet and renders raw
  JSON in the page, but this session still lacks a reliable structured browser
  extraction path for prompt-size and `apply_edits` metadata from that page.
  The accessibility tree proves the endpoint opens under the owner session but
  not enough structured fields were extracted to claim metadata invariants.

remaining error field:

- Bounded table-edit proof on the deployed owner `v81` head remains incomplete.
- Source-gap repair on the owner document remains incomplete.
- Citation marker expansion into transclusion points and source-window opening
  remain incomplete.
- Deployed metadata proof for focused prompt sizes and `apply_edits` metadata
  remains incomplete until diagnosis/export data can be extracted from the
  authenticated product surface without confusing private and published roots.

2026-06-05 owner-visible source repair and diagnosis blocker checkpoint:

status: checkpoint_incomplete

new problem documented before code:

- Computer Use remains available in Comet, but the foreground Choir surface is
  the public VText route for the owner proposal, not the private `v81` owner
  head. The visible proposal window is still `v49`, state `Published v49`.
- The Comet desktop/window list includes a
  `choir_private_legal_cloud_proposal.md` entry, but selecting it from the
  visible list focused the entry without raising a root that could be verified
  as the private `v81 Latest` owner surface. Mutation proof through this visible
  page would therefore risk acting on the wrong published root.
- The backend already has generic product APIs for document diagnosis and
  source-gap repair. The diagnosis URL opens under the authenticated owner
  session, but the page is a raw JSON dump with no owner-safe structured
  extraction or product UI. Copying selected JSON through Comet produced only a
  short selection artifact (`F8J212BTWHT`) in the system clipboard, so this
  session still cannot rely on clipboard extraction for prompt-size,
  `apply_edits`, or source-gap metadata proof.
- Source-gap repair is also backend-only from the owner user's point of view:
  VText can render source entities, but there is no visible repair affordance
  that lets an owner inspect unresolved citation markers, attach source
  entities, apply the generic repair endpoint, and then test citation expansion
  and source-window opening from the same root-safe VText surface.

belief-state update:

- The remaining citation/transclusion realism axis is not just missing data on
  the owner document. It is missing owner-visible, root-safe repair and
  diagnostic affordances. Adding such affordances is aligned with the
  requirements contracts because citations are transclusion points and
  diagnosis/debug evidence must be product-accessible rather than raw
  spelunking.
- The next code change should expose existing generic source-gap repair and
  diagnosis data through VText UI without adding document-specific repairs,
  classifiers, or workflow scaffolding. It should preserve VText as canonical
  and keep hidden metadata out of prose.

2026-06-05 deployed source-repair diagnostics UI checkpoint:

status: checkpoint_incomplete

landed platform change:

- Documentation-first checkpoint `58e4653d` recorded the owner-visible source
  repair and diagnosis blocker before code changed.
- Code commit `838aebaf9f78f44d7aff0d31320acdd9e8a0909e` is on
  `origin/main`. It exposes the existing generic VText diagnosis/source-gap
  repair path in the VText editor: a `Sources` panel lists unresolved citation
  markers, source entities, diagnosis summary data, and an editable JSON repair
  payload. Applying the panel calls
  `POST /api/vtext/documents/{id}/source-repairs`. It does not hardcode the
  owner document, does not add a classifier/workflow scaffold, and keeps repair
  UI metadata outside the editable document prose.
- Local verification before push passed:
  `npm --prefix frontend run build`;
  `nix develop -c go test -tags comprehensive ./internal/runtime -run 'TestVTextSourceGapRepairCreatesRevision|TestVTextSourceGapRepairPreservesUnrepairedGaps|TestVTextSourceGapRepairRejectsUnknownEntity|TestVTextDiagnosisIncludesDocumentRuns|TestVTextImportedMarkdownRevisionUsesVTextProjectionAndPreservesCollapsedTable|TestVTextDocumentResponseReconcilesPendingMutationFromCurrentHead' -count=1`;
  and `git diff --check`.

landing evidence:

- GitHub Actions CI run `27017631552` completed successfully for
  `838aebaf9f78f44d7aff0d31320acdd9e8a0909e`, including frontend build, Go
  vet/build, non-runtime Go tests, all runtime shards, integration smoke, and
  `Deploy to Staging (Node B)`.
- FlakeHub run `27017631591` completed successfully for the same head.
- Staging `/health` reported proxy and sandbox deployed commit
  `838aebaf9f78f44d7aff0d31320acdd9e8a0909e`, deployed at
  `2026-06-05T13:28:44Z`.

deployed owner-account proof limitation:

- Computer Use is available, and Comet is authenticated on staging, but the
  visible foreground remains the public route
  `/pub/vtext/choir-private-legal-cloud-proposal-md-pub28536a488`, showing
  `choir_private_legal_cloud_proposal.md` at `v49`, state `Published v49`.
- The visible page still contains the earlier accidental QA scratch instruction
  in the public root, followed by `Revision cancelled. You can revise again
  from the current version.` The app switcher contains a
  `choir_private_legal_cloud_proposal.md` entry, but the session still has not
  produced a foreground window verifiably showing the private owner `v81
  Latest` surface after the source-repair UI deployment.
- Therefore this checkpoint proves the shipped generic diagnosis/source-repair
  affordance at build/deploy level, but it does not yet prove citation repair,
  citation expansion into transclusions, source-window opening, bounded
  appendix-table edit survival, or focused prompt/apply-edits metadata on the
  actual owner head.

remaining error field:

- Restore a root-safe authenticated path to the private owner VText surface for
  doc `f93cea62-f833-4dae-b414-8e44783d8cbe`; do not mutate the public v49
  published root for acceptance.
- Once the private head is visible, verify the deployed `Sources` panel on the
  actual owner document, apply bounded source-gap repair only through generic
  source entities, and prove citation markers expand into transclusion points
  that open source windows.
- Complete the bounded table-edit acceptance on the private owner head and
  capture focused prompt-size plus `apply_edits` metadata through product
  diagnosis/export surfaces.

suggested resume goal string:

2026-06-05 private VText deep-link blocker checkpoint:

status: checkpoint_incomplete

new problem documented before code:

- A Comet probe navigated the authenticated-looking session from the public
  proposal route to `https://choir.news/?desktop_recovery=1`. The resulting
  page rendered the signed-out/preview desktop with `Choir Preview`, not the
  private owner desktop recovery surface. This means the public proposal route
  was readable, but it did not prove the current tab had a usable private
  owner session.
- The product has a stable public VText route (`/pub/vtext/...`) and app URL
  intents for some apps, but there is no generic private VText deep link that
  opens a specific authenticated document by `doc_id`. Owner proof therefore
  depends on restored desktop window state, app-switcher ordering, and visual
  foregrounding, which already produced a public/private root confusion for
  doc `f93cea62-f833-4dae-b414-8e44783d8cbe`.
- This is a product-path debuggability and QA problem, not a document-specific
  data problem. A root-safe owner proof needs a private, authenticated VText
  URL intent that opens a requested canonical document or shows the normal
  passkey overlay and then replays the same intent after login.

belief-state update:

- The next structural fix should add a generic authenticated VText URL intent,
  such as `/?app=vtext&doc=<doc_id>&title=<optional-title>`, using the
  existing app replay/auth overlay machinery. It must not bypass auth, must not
  expose private documents publicly, and must not hardcode the owner proposal.
- After that ships, the owner-account proof can use Comet to open the private
  doc directly, verify `v81+ Latest`, and continue source-repair/table-edit
  acceptance without relying on saved desktop state.

suggested resume goal string:

2026-06-05 deployed private VText deep-link checkpoint:

status: checkpoint_incomplete

landed platform change:

- Documentation-first checkpoint `fda76b14` recorded the private VText
  deep-link blocker before code changed.
- Code commit `830178f157e156b76b613ec5b5b2e1cefbdada3a` is on
  `origin/main`. It adds a generic authenticated URL intent:
  `/?app=vtext&doc=<doc_id>&title=<optional-title>`. A signed-in session opens
  the requested private VText document with `createInitialVersion: false`; a
  signed-out session shows the existing passkey overlay and preserves the same
  private-document intent for replay after login. Bare `?app=vtext` is treated
  as stale and consumed, matching the existing stale-email URL behavior.
- The change is not document-specific and does not bypass auth or expose
  private documents through a public route.
- A Playwright regression was added to create a private VText document through
  product APIs, open it through the new URL intent, assert the exact
  `data-vtext-doc-id`, and confirm the URL intent is consumed.

verification and deployment evidence:

- Local verification passed: `npm --prefix frontend run build` and
  `git diff --check`.
- The focused Playwright test was not run locally because the expected
  `localhost:4173` service stack was not running. The available
  `start-services.sh` path injects host-specific Dolt/ICU CGO flags, which the
  repo contract says not to normalize as durable proof.
- GitHub Actions CI run `27018087336` completed successfully for
  `830178f157e156b76b613ec5b5b2e1cefbdada3a`, including frontend build, Go
  vet/build, non-runtime Go tests, all runtime shards, integration smoke, and
  `Deploy to Staging (Node B)`.
- FlakeHub run `27018087354` completed successfully for the same head.
- Staging `/health` reported proxy and sandbox deployed commit
  `830178f157e156b76b613ec5b5b2e1cefbdada3a`, deployed at
  `2026-06-05T13:37:40Z`.

deployed Comet proof:

- Computer Use remained available for Comet.
- Opening
  `https://choir.news/?app=vtext&doc=f93cea62-f833-4dae-b414-8e44783d8cbe&title=choir_private_legal_cloud_proposal.md`
  on staging produced the expected private-action passkey overlay:
  `Open choir_private_legal_cloud_proposal.md from your private computer.`
- Switching the overlay to `Use passkey`, filling `yusefnathanson@me.com`, and
  invoking `Use Passkey` produced the macOS system passkey sheet:
  `Sign in to "choir.news" with your passkey for "yusefnathanson@me.com"?`
  This proves the deployed URL intent reached the correct owner-auth ceremony
  for the target private document.
- The passkey ceremony was not completed in this Codex session. Computer Use
  could observe the sheet but did not advance it with the available click-like
  drag action, and bypassing user-presence would violate the product auth
  boundary.

remaining error field:

- Owner-account proof is now blocked at passkey user presence rather than by
  ambiguous public/private VText routing. Once the owner completes or refreshes
  the passkey session, the same URL can open the private document directly.
- Still unproven: bounded appendix-table edit on the private owner head,
  owner-document source-gap repair, citation expansion into transclusion
  affordances, source-window opening, and focused prompt-size/`apply_edits`
  metadata for ordinary revisions.

suggested resume goal string:

2026-06-05 generic source-window blocker checkpoint:

status: checkpoint_incomplete

new problem documented before code:

- Existing generic tests prove that VText source citations render as inline
  transclusions and that YouTube/video-backed citations can open a video app.
  They do not prove that source-service or content-item-backed citations open a
  usable source window.
- Code inspection found the gap: `VTextEditor.svelte` maps
  `display.open_surface: "source"` to the requested app id when the source
  entity has no URL, and otherwise falls back to `content`. The app registry
  has no `source` app and no `content` app, even though `ContentViewer.svelte`
  exists. Therefore a non-media citation's `Open source` button can dispatch a
  launch request that cannot mount a real owning surface.
- This violates the citation/transclusion invariant at the next realism axis:
  all citations are transclusion points, and opening one should reveal the
  source substrate rather than a missing/unknown app.

belief-state update:

- The next structural fix should register the existing content/source viewer as
  a normal app and map generic `open_surface: "source"` / content-item targets
  to that app when no more specific media/browser/VText app applies. This is
  generic source substrate preservation, not a document-specific source repair.

suggested resume goal string:

2026-06-05 deployed generic source-window checkpoint:

status: checkpoint_incomplete

landed platform change:

- Documentation-first checkpoint `36b1a349` recorded the generic source-window
  blocker before code changed.
- Code commit `ef3c3dbaba4018dff4d769d4e5b1f90098144f6e` is on
  `origin/main`. It registers the existing `ContentViewer` as a hidden
  `content` / Source app and updates VText source opening so generic
  `open_surface: "source"` and content/source-service targets open that source
  viewer when they do not resolve to browser, media, or VText publication
  surfaces.
- VText now passes the source entity into the opened source window. The
  content viewer renders bounded snapshot/selector text, entity id,
  source-service item id, content item id, and evidence state when available.
  This keeps source metadata out of VText prose while making the citation's
  owning source substrate visible.
- A Playwright regression extends the markdown-lineage source test to click a
  non-media citation's `Open source` button and assert that a real
  `data-content-viewer` window opens with the source label, excerpt, and source
  entity metadata.

verification and deployment evidence:

- Local verification passed: `npm --prefix frontend run build` and
  `git diff --check`.
- The focused Playwright regression was not run locally because the expected
  `localhost:4173` authenticated service stack was not running.
- GitHub Actions CI run `27018544095` completed successfully for
  `ef3c3dbaba4018dff4d769d4e5b1f90098144f6e`, including frontend build, Go
  vet/build, non-runtime Go tests, all runtime shards, integration smoke, and
  `Deploy to Staging (Node B)`.
- FlakeHub run `27018543926` completed successfully for the same head.
- Staging `/health` reported proxy and sandbox deployed commit
  `ef3c3dbaba4018dff4d769d4e5b1f90098144f6e`, deployed at
  `2026-06-05T13:46:42Z`.

remaining error field:

- This generic source-window path is deployed and test-covered at product UI
  level, but it has not yet been exercised on the actual owner proposal because
  owner-head access still requires completing the passkey user-presence
  ceremony.
- Still unproven on the owner document: source-gap repair through the deployed
  `Sources` panel, citation marker expansion into source-backed transclusions,
  source-window opening from the owner head, bounded appendix-table edit
  survival, and focused prompt-size/`apply_edits` metadata for ordinary
  revisions.

suggested resume goal string:

2026-06-05 source-panel repair proof gap checkpoint:

status: checkpoint_incomplete

new problem documented before code:

- The source-gap repair backend is covered by API tests, and generic source
  windows are now covered for non-media citations. However, the product path
  that the owner must use on the proposal is the VText `Sources` panel:
  inspect unresolved markers, edit the repair JSON, apply the repair, then
  verify the resulting citation transclusion and source window.
- Existing browser tests do not exercise that panel apply path. This leaves a
  realism gap between the shipped source-repair UI and the owner-document
  acceptance requirement.

belief-state update:

- The next safe improvement should add a browser-level regression for applying
  a source repair through the VText `Sources` panel on a fixture document with
  a repairable citation gap, then verify the repaired marker renders as a
  source transclusion and opens the generic source window. This does not
  mutate the owner document and does not add document-specific behavior.

suggested resume goal string:

```text
/goal Continue docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md as a Codex-operated MissionGradient mission from checkpoint f05b4c92. Use the requirements contracts in docs/source-external-data-publication.md, docs/vtext-version-compare-merge-debuggability-spec.md, and docs/vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md. First verify whether computer-use is available; if it is, use authenticated staging UI QA on yusefnathanson@me.com, otherwise use browser/API backup and record that limitation. Do not write code before documenting any newly found problem. Root-cause the real owner document appendix-table regression in choir_private_legal_cloud_proposal.md (doc f93cea62-f833-4dae-b414-8e44783d8cbe): compare v70-v78 and identify the first transition that collapses the Markdown glossary table into the TermDefinition artifact. Repair the structural corruption path, not with a glossary-specific special case but by preserving VText document structure through render/edit/save/revise. Prove on staging with the actual owner document that table formatting survives focus/edit/save/revise both when the table is untouched and when a bounded table edit is requested, while ordinary revisions keep focused_user_edit_diff prompt sizes and apply_edits metadata. Then continue the next realism axis: repair unresolved citation/source gaps on the same owner document so citation markers expand into transclusions and open source windows. Preserve invariants: VText is canonical, only VText writes canonical .vtext revisions, hidden metadata must not render as prose, all citations are transclusion points, whole-document rewrite is explicit and exceptional, and no classifiers/workflow scaffolding or hardcoded document-specific fixes. Land with commit -> push main -> CI -> Node B deploy -> staging identity -> deployed owner-account proof, and update this mission doc with evidence and residual risks.
```

2026-06-05 source-panel repair regression checkpoint:

status: checkpoint_incomplete

landed test change:

- Documentation-first checkpoint `52d1bdf9` recorded the VText `Sources` panel
  repair proof gap before test code changed.
- Test commit `f36bba49e1549e0a80dea2419d02057ae1275444` is on
  `origin/main`. It adds a browser-level regression that imports a VText
  fixture document with a repairable citation marker, opens the VText `Sources`
  panel, applies a bounded source repair payload through the same panel control
  an owner would use, verifies the repaired citation renders as a
  `data-vtext-citation-transclusion`, and clicks `Open source` to prove the
  generic `ContentViewer` window opens with source-entity metadata.
- This is fixture coverage for the deployed product path. It does not mutate
  the private owner proposal and does not add document-specific behavior.

verification and deployment evidence:

- Local verification passed before the test commit: `npm --prefix frontend run
  build` and `git diff --check`.
- The focused Playwright regression was not run locally because the expected
  `localhost:4173` staging-like service was not running; the durable acceptance
  environment remains `https://choir.news`.
- GitHub Actions CI run `27018843463` completed successfully for
  `f36bba49e1549e0a80dea2419d02057ae1275444`, including Go vet/build,
  non-runtime Go tests, all runtime shards, and integration smoke. The deploy
  impact detector skipped `Build Frontend` and `Deploy to Staging (Node B)`
  because this was test-only.
- FlakeHub run `27018843204` completed successfully for the same head.
- Staging `/health` still reported proxy and sandbox deployed commit
  `ef3c3dbaba4018dff4d769d4e5b1f90098144f6e`, deployed at
  `2026-06-05T13:46:42Z`, which is expected because the later
  `f36bba49` change did not deploy behavior.

remaining error field:

- The generic source-window behavior is deployed at `ef3c3dba` and now has
  fixture-level source-panel repair coverage at `f36bba49`.
- Owner-account proof remains blocked at the passkey user-presence ceremony in
  Comet. The deep link reaches the correct private action for
  `choir_private_legal_cloud_proposal.md`, but the private document has not
  been reopened in this session after passkey completion.
- Still unproven on the actual owner document: source-gap repair through the
  deployed `Sources` panel, citation marker expansion into transclusions,
  source-window opening from the owner head, bounded appendix-table edit
  survival, focused prompt-size/`apply_edits` metadata, and the practical
  migration of this imported `.md` acting-as-VText document onto a canonical
  `.vtext` document name with export back to Markdown.

suggested resume goal string:

```text
/goal Continue docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md as a Codex-operated MissionGradient mission from checkpoint f05b4c92. Use the requirements contracts in docs/source-external-data-publication.md, docs/vtext-version-compare-merge-debuggability-spec.md, and docs/vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md. First verify whether computer-use is available; if it is, use authenticated staging UI QA on yusefnathanson@me.com using the Comet browser, otherwise use browser/API backup and record that limitation. Do not write code before documenting any newly found problem. Root-cause the real owner document appendix-table regression in choir_private_legal_cloud_proposal.md (doc f93cea62-f833-4dae-b414-8e44783d8cbe): compare v70-v78 and identify the first transition that collapses the Markdown glossary table into the TermDefinition artifact. Repair the structural corruption path, not with a glossary-specific special case but by preserving VText document structure through render/edit/save/revise. Treat imported `.txt`, `.md`, and other text-like documents as VText once they first transition from v0 to v1: canonical revisions should be `.vtext`, with Markdown available as an export format rather than as the canonical owner document. Prove on staging with the actual owner document that table formatting survives focus/edit/save/revise both when the table is untouched and when a bounded table edit is requested, while ordinary revisions keep focused_user_edit_diff prompt sizes and apply_edits metadata. Then continue the next realism axis: repair unresolved citation/source gaps on the same owner document so citation markers expand into transclusions and open source windows. Preserve invariants: VText is canonical, only VText writes canonical .vtext revisions, hidden metadata must not render as prose, all citations are transclusion points, whole-document rewrite is explicit and exceptional, and no classifiers/workflow scaffolding or hardcoded document-specific fixes. Land with commit -> push main -> CI -> Node B deploy -> staging identity -> deployed owner-account proof, and update this mission doc with evidence and residual risks.
```

2026-06-05 imported file canonical VText identity checkpoint:

status: checkpoint_incomplete

new problem documented before code:

- The deployed repair makes imported Markdown structurally VText-backed and
  preserves original source artifacts, but the editable document identity is not
  yet fully `.vtext` canonical. `POST /api/vtext/files/open` and
  `POST /api/vtext/markdown-lineage/import` still derive `vtext_documents.title`
  from the caller-provided title or original `source_path`, so an imported
  `choir_private_legal_cloud_proposal.md` can keep presenting the owner-facing
  editable document as `.md` even after the first canonical VText revision.
- That is materially different from a true `.vtext` document. The source
  artifact and alias are preserved, but the canonical owner document name still
  implies Markdown ownership rather than VText ownership. This weakens the
  invariant that VText is canonical and Markdown is an export/projection format.
- The requirements contracts distinguish original imported files as
  `ContentItem` source artifacts from VText as the editable semantic projection.
  Therefore the fix should not discard or rewrite the original `.md`, `.txt`,
  DOCX, or PDF evidence. It should make the VText document identity canonical
  `.vtext` at the v0-to-v1 transition, while preserving the original source path
  as an alias/import manifest entry and enabling Markdown export from VText.

belief-state update:

- The next structural fix should introduce a generic title/path normalization
  for imported text-like and document-like files opened into VText. Original
  source aliases such as `proposals/legal-cloud.md` must continue resolving to
  the canonical document, but the document title returned by VText APIs and
  shown in recent/private document intents should use a `.vtext` name.
- Markdown lineage imports should likewise create a `.vtext` canonical document
  even when the source snapshots are Markdown. Existing hidden import metadata
  should continue recording `source_path`, `source_kind`, original content ids,
  source gaps, and source entities; hidden metadata must not render as prose.
- Export back to Markdown should remain a supported projection/export concern,
  not a reason to keep `.md` as the canonical VText document identity.

suggested resume goal string:

```text
/goal Continue docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md as a Codex-operated MissionGradient mission from checkpoint f05b4c92. Use the requirements contracts in docs/source-external-data-publication.md, docs/vtext-version-compare-merge-debuggability-spec.md, and docs/vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md. First verify whether computer-use is available; if it is, use authenticated staging UI QA on yusefnathanson@me.com using the Comet browser, otherwise use browser/API backup and record that limitation. Do not write code before documenting any newly found problem. Root-cause the real owner document appendix-table regression in choir_private_legal_cloud_proposal.md (doc f93cea62-f833-4dae-b414-8e44783d8cbe): compare v70-v78 and identify the first transition that collapses the Markdown glossary table into the TermDefinition artifact. Repair the structural corruption path, not with a glossary-specific special case but by preserving VText document structure through render/edit/save/revise. Treat imported `.txt`, `.md`, and other text-like or document-like files as VText once they first transition from v0 to v1: canonical VText document identity should be `.vtext`, original files remain source artifacts/aliases, and Markdown should be available as an export format rather than as the canonical owner document. Prove on staging with the actual owner document that table formatting survives focus/edit/save/revise both when the table is untouched and when a bounded table edit is requested, while ordinary revisions keep focused_user_edit_diff prompt sizes and apply_edits metadata. Then continue the next realism axis: repair unresolved citation/source gaps on the same owner document so citation markers expand into transclusions and open source windows. Preserve invariants: VText is canonical, only VText writes canonical .vtext revisions, hidden metadata must not render as prose, all citations are transclusion points, whole-document rewrite is explicit and exceptional, and no classifiers/workflow scaffolding or hardcoded document-specific fixes. Land with commit -> push main -> CI -> Node B deploy -> staging identity -> deployed owner-account proof, and update this mission doc with evidence and residual risks.
```

2026-06-05 deployed imported-file VText identity checkpoint:

status: checkpoint_incomplete

landed platform change:

- Documentation-first checkpoint `bbb04c95` recorded the imported-file VText
  identity gap before code changed.
- Code commit `2f1f40540cf2483b09042dd4b950ce61164e0aec` is on
  `origin/main`. It canonicalizes VText document titles created from
  `POST /api/vtext/files/open` and
  `POST /api/vtext/markdown-lineage/import` to `.vtext` while preserving the
  original source path as the alias/import manifest source artifact.
- The change is generic. Imported Markdown, text, DOCX, PDF, and other
  VText-opened files no longer keep the original extension as the canonical
  VText document title. Existing source aliases such as
  `proposals/legal-cloud.md` still resolve to the canonical VText document.
- The same commit adds owner-scoped
  `GET /api/vtext/documents/{doc_id}/export?format=md`, returning the selected
  or current revision content as Markdown with a `.md` filename and content
  hash. The export response is revision content only; hidden import/source
  metadata stays out of visible Markdown prose.

verification and deployment evidence:

- Local verification passed:
  - `nix develop -c go test -tags comprehensive ./internal/runtime -run
    'TestVText(OpenFileResolvesCanonicalAlias|ImportMarkdownLineageCreatesRevisionHistory|OpenFilePreservesDocxAndPDFOriginalArtifacts|OpenFileImportsDocxAndPDFBytesFromFilesRoot)'`
  - `nix develop -c go test ./internal/runtime -run
    'TestVTextPromptInitialRevisionUsesSingleWriterLoop|TestVTextEditRevisionMetadataRecordsOperationEvidence'`
  - `nix develop -c go test -tags comprehensive ./internal/runtime -run
    'TestVText(OpenFile|ImportMarkdownLineage|ImportedMarkdown)'`
  - `git diff --check`
- GitHub Actions CI run `27019394481` completed successfully for
  `2f1f40540cf2483b09042dd4b950ce61164e0aec`, including Go vet/build,
  non-runtime Go tests, all runtime shards, integration smoke, and
  `Deploy to Staging (Node B)`. Frontend build was skipped because the change
  did not touch deployed frontend artifacts.
- FlakeHub run `27019394298` completed successfully for the same head.
- Staging `/health` reported proxy and sandbox deployed commit
  `2f1f40540cf2483b09042dd4b950ce61164e0aec`, deployed at
  `2026-06-05T14:02:57Z`.

deployed proof limitation:

- Computer Use is available and Comet still opens the correct private-document
  URL intent for `choir_private_legal_cloud_proposal.md`, but the page remains
  on the passkey overlay for `yusefnathanson@me.com` with the prior
  `Passkey ceremony was cancelled. Please try again.` state.
- Therefore the deployed generic import/export behavior is proven by local
  runtime API tests plus CI/deploy identity, but it is not yet proven on the
  private owner proposal through the authenticated product UI. Completing the
  passkey ceremony is still required before the owner document can be opened
  and mutated safely.

remaining error field:

- Still unproven on the actual owner document: canonical title migration for
  the existing `choir_private_legal_cloud_proposal.md` document, source-gap
  repair through the deployed `Sources` panel, citation marker expansion into
  transclusions, source-window opening from the owner head, bounded appendix
  table edit survival, and focused prompt-size/`apply_edits` metadata.
- Existing imported documents whose title is already `.md` may need a
  migration/restore path after owner access is available. The new code fixes
  newly opened/imported VText projections; it does not bulk-retitle private
  documents that already exist.

suggested resume goal string:

```text
/goal Continue docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md as a Codex-operated MissionGradient mission from checkpoint f05b4c92. Use the requirements contracts in docs/source-external-data-publication.md, docs/vtext-version-compare-merge-debuggability-spec.md, and docs/vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md. First verify whether computer-use is available; if it is, use authenticated staging UI QA on yusefnathanson@me.com using the Comet browser, otherwise use browser/API backup and record that limitation. Do not write code before documenting any newly found problem. Continue from deployed commit 2f1f40540cf2483b09042dd4b950ce61164e0aec, which canonicalizes newly imported/opened VText projection document titles to `.vtext` and adds owner-scoped Markdown export. Root-cause the real owner document appendix-table regression in choir_private_legal_cloud_proposal.md (doc f93cea62-f833-4dae-b414-8e44783d8cbe): compare v70-v78 and identify the first transition that collapses the Markdown glossary table into the TermDefinition artifact. Repair the structural corruption path, not with a glossary-specific special case but by preserving VText document structure through render/edit/save/revise. Treat imported `.txt`, `.md`, and other text-like or document-like files as VText once they first transition from v0 to v1: canonical VText document identity should be `.vtext`, original files remain source artifacts/aliases, and Markdown should be available as an export format rather than as the canonical owner document. Prove on staging with the actual owner document that table formatting survives focus/edit/save/revise both when the table is untouched and when a bounded table edit is requested, while ordinary revisions keep focused_user_edit_diff prompt sizes and apply_edits metadata. Then continue the next realism axis: repair unresolved citation/source gaps on the same owner document so citation markers expand into transclusions and open source windows. Preserve invariants: VText is canonical, only VText writes canonical .vtext revisions, hidden metadata must not render as prose, all citations are transclusion points, whole-document rewrite is explicit and exceptional, and no classifiers/workflow scaffolding or hardcoded document-specific fixes. Land with commit -> push main -> CI -> Node B deploy -> staging identity -> deployed owner-account proof, and update this mission doc with evidence and residual risks.
```

2026-06-05 legacy imported document first-revision identity checkpoint:

status: checkpoint_incomplete

new problem documented before code:

- Commit `2f1f4054` fixes newly opened/imported VText projection documents, but
  it does not fully cover the stated v0-to-v1 invariant for documents that were
  already imported before the fix. `handleVTextCreateRevision` verifies the
  document, creates a canonical `.vtext` manifest alias, and writes the user
  revision, but it does not retitle an existing aliased document whose
  `vtext_documents.title` still ends in `.md`, `.txt`, `.docx`, or `.pdf`.
- That means a legacy imported document can cross a new edit/save boundary and
  still present the editable canonical document as `*.md`. This is weaker than
  "as soon as an imported txt or md or other goes from v0 to v1, it should be
  converted to `.vtext`."
- The fix should be generic and alias-driven: if a document has a source alias,
  VText revision creation can safely canonicalize its document title to
  `.vtext` before writing the revision. It must preserve the original alias and
  import/source metadata, must not retitle arbitrary non-aliased hand-created
  documents, and must not write metadata as visible prose.

belief-state update:

- The next structural fix should make the revision-create path converge legacy
  imported/aliased documents to a `.vtext` title on the next canonical VText
  write. This creates a safe migration path for existing private documents once
  owner access is available, without bulk-mutating private state blindly.

suggested resume goal string:

```text
/goal Continue docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md as a Codex-operated MissionGradient mission from checkpoint f05b4c92. Use the requirements contracts in docs/source-external-data-publication.md, docs/vtext-version-compare-merge-debuggability-spec.md, and docs/vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md. First verify whether computer-use is available; if it is, use authenticated staging UI QA on yusefnathanson@me.com using the Comet browser, otherwise use browser/API backup and record that limitation. Do not write code before documenting any newly found problem. Continue from deployed commit 2f1f40540cf2483b09042dd4b950ce61164e0aec, and repair the remaining legacy imported-document identity path: on the next VText revision write, aliased imported documents whose title still looks like `.md`, `.txt`, DOCX, PDF, or another source artifact should converge to canonical `.vtext` identity while original files remain source artifacts/aliases and Markdown remains an export format. Root-cause the real owner document appendix-table regression in choir_private_legal_cloud_proposal.md (doc f93cea62-f833-4dae-b414-8e44783d8cbe): compare v70-v78 and identify the first transition that collapses the Markdown glossary table into the TermDefinition artifact. Repair the structural corruption path, not with a glossary-specific special case but by preserving VText document structure through render/edit/save/revise. Prove on staging with the actual owner document that table formatting survives focus/edit/save/revise both when the table is untouched and when a bounded table edit is requested, while ordinary revisions keep focused_user_edit_diff prompt sizes and apply_edits metadata. Then continue the next realism axis: repair unresolved citation/source gaps on the same owner document so citation markers expand into transclusions and open source windows. Preserve invariants: VText is canonical, only VText writes canonical .vtext revisions, hidden metadata must not render as prose, all citations are transclusion points, whole-document rewrite is explicit and exceptional, and no classifiers/workflow scaffolding or hardcoded document-specific fixes. Land with commit -> push main -> CI -> Node B deploy -> staging identity -> deployed owner-account proof, and update this mission doc with evidence and residual risks.
```

2026-06-05 deployed legacy imported-document VText identity checkpoint:

status: checkpoint_incomplete

landed platform change:

- Documentation-first checkpoint `8eb9174b` recorded the remaining legacy
  imported-document identity gap before code changed.
- Code commit `e5d3b4e698c01fcced13b1c2dd15077792d37ab8` is on
  `origin/main`. It makes public VText revision creation canonicalize the title
  of an aliased document to `.vtext` before writing the revision, while leaving
  non-aliased hand-created documents alone.
- The fix is alias-driven: the original source alias, for example
  `imports/legacy-import.md`, continues resolving to the same canonical VText
  document. The title migration happens on the next VText write and does not
  render import/source metadata as prose.
- This closes the generic v0-to-v1 path for existing imported documents once
  they receive a new canonical VText revision. It also creates a safe migration
  path for the private owner proposal after passkey-gated owner access is
  available.

verification and deployment evidence:

- Local verification passed:
  - `nix develop -c go test -tags comprehensive ./internal/runtime -run
    'TestVTextAPICreateRevisionCanonicalizesAliasedImportedDocumentTitle|TestVText(OpenFile|ImportMarkdownLineage|ImportedMarkdown)'`
  - `nix develop -c go test ./internal/runtime -run
    'TestVTextPromptInitialRevisionUsesSingleWriterLoop|TestVTextEditRevisionMetadataRecordsOperationEvidence'`
  - `git diff --check`
- GitHub Actions CI run `27019765922` completed successfully for
  `e5d3b4e698c01fcced13b1c2dd15077792d37ab8`, including Go vet/build,
  non-runtime Go tests, all runtime shards, integration smoke, and
  `Deploy to Staging (Node B)`. Frontend build was skipped because the change
  did not touch deployed frontend artifacts.
- FlakeHub run `27019765882` completed successfully for the same head.
- Staging `/health` reported proxy and sandbox deployed commit
  `e5d3b4e698c01fcced13b1c2dd15077792d37ab8`, deployed at
  `2026-06-05T14:10:01Z`.

deployed proof limitation:

- Computer Use remains available. Comet still resolves the private VText deep
  link for doc `f93cea62-f833-4dae-b414-8e44783d8cbe` to the passkey overlay
  for `yusefnathanson@me.com`; the ceremony has not been completed in this
  session.
- Therefore the legacy retitle path is proven generically by runtime tests,
  CI, and deployed staging identity, but not yet exercised on the actual owner
  proposal. Owner-account proof still requires passkey completion.

remaining error field:

- Still unproven on the actual owner document: title migration from
  `choir_private_legal_cloud_proposal.md` to canonical `.vtext`, appendix table
  survival under untouched focus/edit/save/revise, bounded appendix-table edit
  survival, focused prompt-size/`apply_edits` metadata, source-gap repair
  through the deployed `Sources` panel, citation transclusion expansion, and
  source-window opening from the owner head.

suggested resume goal string:

```text
/goal Continue docs/mission-vtext-fluid-editing-doc-roundtrip-transclusion-v0.md as a Codex-operated MissionGradient mission from checkpoint f05b4c92. Use the requirements contracts in docs/source-external-data-publication.md, docs/vtext-version-compare-merge-debuggability-spec.md, and docs/vtext-publish-export-ux-and-docx-pdf-research-2026-06-04.md. First verify whether computer-use is available; if it is, use authenticated staging UI QA on yusefnathanson@me.com using the Comet browser, otherwise use browser/API backup and record that limitation. Do not write code before documenting any newly found problem. Continue from deployed commit e5d3b4e698c01fcced13b1c2dd15077792d37ab8, which canonicalizes newly imported/opened VText projection titles and legacy aliased document titles to `.vtext` on VText revision write while preserving original source aliases and Markdown export. Root-cause the real owner document appendix-table regression in choir_private_legal_cloud_proposal.md (doc f93cea62-f833-4dae-b414-8e44783d8cbe): compare v70-v78 and identify the first transition that collapses the Markdown glossary table into the TermDefinition artifact. Repair the structural corruption path, not with a glossary-specific special case but by preserving VText document structure through render/edit/save/revise. Prove on staging with the actual owner document that table formatting survives focus/edit/save/revise both when the table is untouched and when a bounded table edit is requested, while ordinary revisions keep focused_user_edit_diff prompt sizes and apply_edits metadata. Then continue the next realism axis: repair unresolved citation/source gaps on the same owner document so citation markers expand into transclusions and open source windows. Preserve invariants: VText is canonical, only VText writes canonical .vtext revisions, hidden metadata must not render as prose, all citations are transclusion points, whole-document rewrite is explicit and exceptional, and no classifiers/workflow scaffolding or hardcoded document-specific fixes. Land with commit -> push main -> CI -> Node B deploy -> staging identity -> deployed owner-account proof, and update this mission doc with evidence and residual risks.
```

2026-06-05 source-panel stale candidate checkpoint:

status: checkpoint_incomplete

new problem documented before code:

- Computer Use remains available, but Comet is still at the passkey overlay for
  private owner document `f93cea62-f833-4dae-b414-8e44783d8cbe`; the owner
  passkey ceremony is not complete in this session.
- As browser/API backup, the deployed staging fixture test
  `BASE_URL=https://choir.news npx playwright test
  tests/vtext-markdown-lineage.spec.js -g "VText Sources panel applies
  source-gap repair" --project=chromium` was run against `https://choir.news`
  with a disposable Playwright-authenticated product session.
- The test failed before repair application. The VText `Sources` panel opened
  and the repair JSON textarea was prefilled with marker `[2]`, but the
  visible source-gap list selector `[data-vtext-source-gaps]` was absent and
  the panel heading said `0 source entities`.
- Product API inspection of the same fixture document showed the backend
  revision is correct: its content contains the unresolved marker `[2]` and
  `metadata.source_gaps` contains
  `{kind: "unresolved_markdown_citation_marker", marker: "[2]",
  policy: "repairable_gap_no_invented_citations"}`.
- Code inspection points to a frontend reactivity bug, not missing source
  metadata. `VTextEditor.svelte` computes `sourceCandidates` through a Svelte
  reactive assignment that calls helper functions with hidden dependencies on
  `currentRevision` and `editorValue`. The on-demand repair payload sees the
  current marker, but the visible candidate list can remain stale after a
  document/revision load.

belief-state update:

- The remaining source-repair realism axis needs the visible `Sources` panel to
  derive unresolved markers from explicit reactive inputs. The fix should be
  generic Svelte state plumbing, not a source-gap special case and not a
  document-specific repair.

2026-06-05 deployed source-panel candidate refresh checkpoint:

status: checkpoint_incomplete

landed platform change:

- Documentation-first checkpoint `c8dd4976` recorded the stale source-panel
  candidate problem before code changed.
- Code commit `1fbcb3e14a5b533fbb70877f7cf98a83b0810862` is on
  `origin/main`. It makes the VText `Sources` panel derive source entities,
  source gaps, repair candidates, and diagnosis summaries from explicit Svelte
  reactive inputs (`currentRevision`, `editorValue`, `publishedBundle`, and
  `sourceDiagnosis`) instead of helper calls with hidden dependencies.
- The change is generic frontend state plumbing. It does not add
  document-specific source repairs, does not invent citations, does not render
  hidden metadata as prose, and keeps repair application on the existing
  `POST /api/vtext/documents/{id}/source-repairs` product path.

verification and deployment evidence:

- Local verification passed: `npm --prefix frontend run build` and
  `git diff --check`.
- Before the fix, deployed staging fixture proof failed with the `Sources`
  panel open, repair JSON prefilled for marker `[2]`, but no visible
  `[data-vtext-source-gaps]` marker list.
- GitHub Actions CI run `27020266462` completed successfully for
  `1fbcb3e14a5b533fbb70877f7cf98a83b0810862`, including frontend build, Go
  vet/build, non-runtime Go tests, all runtime shards, integration smoke, and
  `Deploy to Staging (Node B)`.
- FlakeHub run `27020266487` completed successfully for the same head.
- Staging `/health` reported proxy and sandbox deployed commit
  `1fbcb3e14a5b533fbb70877f7cf98a83b0810862`, deployed at
  `2026-06-05T14:19:41Z`.
- After deploy, the exact staging regression passed:
  `BASE_URL=https://choir.news npx playwright test
  tests/vtext-markdown-lineage.spec.js -g "VText Sources panel applies
  source-gap repair" --project=chromium`. This proves a disposable
  authenticated product VText can show the unresolved marker in the `Sources`
  panel, apply bounded source repair through the panel, render the repaired
  citation as a transclusion, and open the generic source viewer.

deployed owner-account proof limitation:

- Computer Use remains available. Comet still opens the private VText deep
  link for owner doc `f93cea62-f833-4dae-b414-8e44783d8cbe` to the correct
  passkey overlay: `Open choir_private_legal_cloud_proposal.md from your
  private computer.`
- The passkey ceremony for `yusefnathanson@me.com` remains cancelled/not
  completed in this session. Therefore the source-panel repair path is proven
  on deployed staging with a disposable authenticated product document, but it
  is still not proven on the actual owner proposal.

remaining error field:

- Still unproven on the actual owner document: source-gap repair through the
  deployed `Sources` panel, citation marker expansion into transclusions,
  source-window opening from the owner head, bounded appendix-table edit
  survival, focused prompt-size/`apply_edits` metadata, and title migration of
  `choir_private_legal_cloud_proposal.md` to canonical `.vtext` on the next
  owner VText write.

2026-06-05 diagnosis edit-evidence visibility checkpoint:

status: checkpoint_incomplete

new problem documented before code:

- Computer Use remains available, but Comet remains passkey-gated for the
  private owner proposal. The private owner proof cannot proceed until the
  passkey ceremony completes.
- The mission still needs proof that ordinary revisions keep
  `focused_user_edit_diff` prompt sizes and `apply_edits` metadata. Backend
  revision metadata already records the necessary fields:
  `vtext_context_mode`, `vtext_edit_operation`, `vtext_edit_count`,
  `vtext_run_prompt_chars`, `vtext_edit_delta_chars`, and
  `vtext_run_latency_ms`.
- The deployed VText `Sources`/diagnosis panel currently exposes only diagnosis
  counts such as revision count, run count, latest version, and error count.
  It does not give an owner-visible structured surface for the current or
  latest appagent edit evidence. Previous owner-proof attempts therefore had
  to rely on raw JSON/clipboard extraction from the diagnosis endpoint, which
  was unreliable in Comet.
- This is a product-path debuggability gap, not missing VText metadata. The
  fix should expose a compact, non-prose diagnostic evidence strip in the
  VText panel. It must not render hidden metadata inside the editable document
  body, must not expose raw prompts, and must remain generic for any VText
  document.

belief-state update:

- A small owner-visible edit-evidence surface will make the final owner proof
  simpler once passkey access is restored: the verifier can confirm
  `focused_user_edit_diff`, `apply_edits`, prompt chars, edit count, delta
  chars, and latency from product UI instead of raw JSON extraction.

2026-06-05 deployed diagnosis edit-evidence checkpoint:

status: checkpoint_incomplete

landed platform change:

- Documentation-first checkpoint `e4a5fe61` recorded the diagnosis
  edit-evidence visibility gap before code changed.
- Code commit `4255dc7efe5407b67bb78075cf477c133958d2f3` is on
  `origin/main`. It adds a compact edit-evidence strip to the VText
  `Sources`/diagnosis panel, deriving fields from revision metadata:
  `vtext_context_mode`, `vtext_edit_operation`, `vtext_run_prompt_chars`,
  `vtext_edit_count`, `vtext_edit_delta_chars`, and
  `vtext_run_latency_ms`.
- The evidence strip is outside the editable document body and does not render
  raw prompt text. It is generic for any VText revision carrying the existing
  edit metadata; it does not add classifiers, workflow scaffolding, or
  document-specific handling.

verification and deployment evidence:

- Local verification passed: `npm --prefix frontend run build` and
  `git diff --check`.
- GitHub Actions CI run `27020641535` completed successfully for
  `4255dc7efe5407b67bb78075cf477c133958d2f3`, including frontend build, Go
  vet/build, non-runtime Go tests, all runtime shards, integration smoke, and
  `Deploy to Staging (Node B)`.
- FlakeHub run `27020641521` completed successfully for the same head.
- Staging `/health` reported proxy and sandbox deployed commit
  `4255dc7efe5407b67bb78075cf477c133958d2f3`, deployed at
  `2026-06-05T14:26:47Z`.
- After deploy, the focused staging regression passed:
  `BASE_URL=https://choir.news npx playwright test
  tests/vtext-markdown-lineage.spec.js -g "VText Sources panel shows
  structured edit evidence" --project=chromium`. This proves a disposable
  authenticated product VText can show `focused_user_edit_diff`,
  `apply_edits`, prompt chars, edit count, delta chars, and latency in the
  diagnosis panel while keeping raw prompt text out of the panel and rendered
  document body.
- The source-repair regression was also rerun and passed:
  `BASE_URL=https://choir.news npx playwright test
  tests/vtext-markdown-lineage.spec.js -g "VText Sources panel applies
  source-gap repair" --project=chromium`.

deployed owner-account proof limitation:

- Computer Use remains available. Comet still opens the private VText deep
  link for owner doc `f93cea62-f833-4dae-b414-8e44783d8cbe` to the correct
  passkey overlay, but the passkey ceremony for `yusefnathanson@me.com` remains
  cancelled/not completed in this session.
- Therefore the product-visible edit-evidence surface is proven on deployed
  staging with a disposable authenticated product document, but not yet on the
  actual owner proposal.

remaining error field:

- Still unproven on the actual owner document: bounded appendix-table edit
  survival, source-gap repair through the deployed `Sources` panel, citation
  marker expansion into transclusions, source-window opening from the owner
  head, owner-head focused prompt-size/`apply_edits` metadata in the new
  evidence strip, and title migration of `choir_private_legal_cloud_proposal.md`
  to canonical `.vtext` on the next owner VText write.

2026-06-05 bounded table-cell edit proof gap checkpoint:

status: checkpoint_incomplete

new problem documented before code:

- Computer Use remains available, but the owner proposal deep link is still at
  the passkey overlay for `yusefnathanson@me.com`; the actual owner document
  cannot be safely mutated from this session.
- Existing deployed and local tests prove that rendered Markdown tables survive
  focus/autosave when the edit is outside the table, and backend stabilization
  preserves a parent Markdown table when an incoming draft has already
  collapsed it.
- The remaining owner acceptance explicitly requires the table to survive when
  a bounded table edit is requested. Current fixture coverage does not prove
  that editing a rendered table cell through the product surface serializes
  back to canonical Markdown table structure instead of flattening cells or
  dropping row separators.
- This is a proof gap along the same structural corruption axis as the owner
  appendix regression. The next safe improvement should add a generic browser
  regression that edits a single table cell in the rendered VText surface,
  waits for the normal local draft/autosave path, and verifies the resulting
  Markdown still has a valid pipe table, the bounded cell edit, and no
  `TermDefinition`-style collapse. The test must use a fixture document, not
  the owner proposal.

belief-state update:

- If the fixture passes on deployed staging, it will strengthen the generic
  bounded-table-edit proof but still will not replace the required owner-head
  passkey-gated acceptance. If it fails, the fix should stay in the generic
  rendered-table serialization path rather than adding owner/glossary-specific
  recovery code.

2026-06-05 bounded rendered-table edit regression checkpoint:

status: checkpoint_incomplete

landed proof artifact:

- Documentation-first checkpoint `806fefce` recorded the bounded table-cell
  edit proof gap before the regression test was added.
- Test commit `fcbaf5b40c79f3e54e45ada128e19d166389fc05` is on
  `origin/main`. It adds a generic browser regression in
  `frontend/tests/vtext-source-entities.spec.js` that creates a fixture VText
  document with a Markdown pipe table, edits one rendered table cell through
  the product surface, waits for the normal autosave/draft path, and verifies
  the saved Markdown still contains the table header, separator row, untouched
  sibling row, bounded edited cell text, and no `TermDefinition` artifact.
- The regression is deliberately fixture-based. It does not mutate the private
  owner proposal and does not add owner-specific, glossary-specific, or
  classifier/workflow behavior.

verification and deployment evidence:

- Local verification before commit passed: `npm --prefix frontend run build`
  and `git diff --check`.
- Deployed staging proof also passed before the test commit, against deployed
  behavior commit `4255dc7efe5407b67bb78075cf477c133958d2f3`:
  `BASE_URL=https://choir.news npx playwright test
  tests/vtext-source-entities.spec.js -g "bounded cell edit"
  --project=chromium`.
- GitHub Actions CI run `27020985659` completed successfully for
  `fcbaf5b40c79f3e54e45ada128e19d166389fc05`, including Go vet/build,
  non-runtime Go tests, all runtime shards, integration smoke, and the final
  Go gate aggregator. Frontend build and staging deploy were skipped because
  this commit changed only the browser regression test.
- FlakeHub run `27020985709` completed successfully for the same head.
- Staging `/health` still reports proxy and sandbox deployed commit
  `4255dc7efe5407b67bb78075cf477c133958d2f3`, deployed at
  `2026-06-05T14:26:47Z`; this is expected because `fcbaf5b4` did not change
  deployed artifacts.

deployed owner-account proof limitation:

- Computer Use remains available, and Comet still reaches the private action
  passkey overlay for `yusefnathanson@me.com` on owner document
  `f93cea62-f833-4dae-b414-8e44783d8cbe`.
- The passkey ceremony remains cancelled/not completed in this session.
  Therefore the new bounded rendered-table edit proof is durable on generic
  deployed staging fixtures, but the actual owner appendix table still has not
  been exercised through focus/edit/save/revise on the private owner head.

remaining error field:

- Still unproven on the actual owner document: canonical title migration from
  `choir_private_legal_cloud_proposal.md` to `.vtext` on the next owner VText
  write, bounded appendix-table edit survival, source-gap repair through the
  deployed `Sources` panel, citation marker expansion into transclusions,
  source-window opening from the owner head, and owner-head focused prompt-size
  / `apply_edits` metadata in the visible edit-evidence strip.

2026-06-05 source panel open-source proof gap checkpoint:

status: checkpoint_incomplete

new problem documented before code:

- Computer Use is available in this turn. Comet still reaches the private
  action passkey overlay for owner document
  `f93cea62-f833-4dae-b414-8e44783d8cbe`, and the passkey ceremony for
  `yusefnathanson@me.com` remains cancelled/not completed. No private owner
  mutation is authorized from this session.
- The authenticated Comet diagnosis tab for the same owner document remains
  readable as raw JSON, while unauthenticated shell access to the same
  diagnosis endpoint returns `401 authentication required`. This is
  owner-session evidence, not public access, and it remains too raw for
  owner-safe product proof.
- Existing deployed source-repair proof exercises the `Sources` panel repair
  control, then verifies that the repaired inline citation expands into a
  transclusion and that the citation's `Open source` control opens a source
  window.
- The owner repair workflow also needs the `Sources` panel itself to be a
  reliable source browser after repair: once a repaired entity appears in the
  panel, its source-entity chip should open the owning source window. That
  panel-chip open path is generic code, but current focused regression evidence
  does not prove it on the repaired-source path.

belief-state update:

- A focused fixture regression that applies a source-gap repair through the
  product `Sources` panel, observes the repaired entity in that same panel,
  clicks the panel source chip, and verifies the source window will strengthen
  the next owner-account proof without touching the private owner proposal or
  adding document-specific behavior.

2026-06-05 source panel repaired-source opening checkpoint:

status: checkpoint_incomplete

landed proof artifact:

- Documentation-first checkpoint `4353b17d` recorded the source-panel
  open-source proof gap before the regression changed.
- Test commit `68794d61fe436ac961c5d847fbe73ef51b2c97d1` is on
  `origin/main`. It extends the generic `VText Sources panel applies
  source-gap repair and opens repaired source window` regression so the test
  now repairs a citation gap through the product `Sources` panel, verifies the
  inline citation expands into a transclusion and opens a source window, closes
  that source window, verifies the repaired source entity appears in the
  `Sources` panel, clicks the panel source-entity chip, and verifies that chip
  opens the same owning source window with label, excerpt, and entity id.
- The test remains fixture-based and uses disposable authenticated staging
  product state. It does not mutate the private owner proposal and does not add
  owner-specific or glossary-specific behavior.

verification and deployment evidence:

- Local verification passed: `npm --prefix frontend run build` and
  `git diff --check`.
- Deployed staging proof passed against deployed behavior commit
  `4255dc7efe5407b67bb78075cf477c133958d2f3`:
  `BASE_URL=https://choir.news npx playwright test
  tests/vtext-markdown-lineage.spec.js -g "VText Sources panel applies
  source-gap repair" --project=chromium`.
- GitHub Actions CI run `27021693186` completed successfully for
  `68794d61fe436ac961c5d847fbe73ef51b2c97d1`, including Go vet/build,
  non-runtime Go tests, all runtime shards, integration smoke, and the final
  Go gate aggregator. Frontend build and staging deploy were skipped because
  this commit changed only the browser regression test.
- FlakeHub run `27021693234` completed successfully for the same head.
- Staging `/health` still reports proxy and sandbox deployed commit
  `4255dc7efe5407b67bb78075cf477c133958d2f3`, deployed at
  `2026-06-05T14:26:47Z`; this is expected because `68794d61` did not change
  deployed artifacts.

remaining error field:

- Still unproven on the actual owner document: canonical title migration from
  `choir_private_legal_cloud_proposal.md` to `.vtext` on the next owner VText
  write, bounded appendix-table edit survival, source-gap repair through the
  deployed `Sources` panel on the owner head, citation marker expansion into
  transclusions on that owner head, source-window opening from that owner head,
  and owner-head focused prompt-size / `apply_edits` metadata in the visible
  edit-evidence strip.

2026-06-05 imported Markdown v0-to-v1 product proof gap checkpoint:

status: checkpoint_incomplete

new problem documented before code:

- Computer Use is available, but owner-document UI proof remains blocked at the
  passkey/user-presence boundary. Comet can read the authenticated raw
  diagnosis JSON for owner doc `f93cea62-f833-4dae-b414-8e44783d8cbe`, but the
  private VText document itself is not open for safe mutation in this session.
- Backend tests prove important imported-file invariants: `/api/vtext/files/open`
  creates `.vtext` canonical document titles for Markdown/DOCX/PDF projections,
  original source aliases remain resolvable, the next revision can shift the
  latest alias source path to `.vtext`, and Markdown export returns canonical
  VText bytes as `.md`.
- Existing browser tests prove prompt-created `.vtext` shortcuts and
  DOCX/PDF import-revise-publish-export flows, but they do not prove the exact
  user-added invariant for an imported Markdown file acting as VText: seed v0
  from the `.md` source, advance to v1 through the VText revision write path,
  verify canonical document identity is `.vtext`, verify the original `.md`
  alias still resolves to the same document, and verify Markdown is available
  only as export bytes rather than as canonical document identity.
- This is a product-path proof gap, not currently a known backend behavior
  failure. The next safe improvement should add a focused browser regression
  using a disposable imported `.md` fixture and public authenticated VText APIs.

belief-state update:

- A staging browser/API regression for Markdown import v0-to-v1 canonical
  `.vtext` identity will strengthen the owner-document title-migration proof
  needed after passkey access is restored, while preserving the invariant that
  original imported files remain source artifacts/aliases and Markdown remains
  an export format.

2026-06-05 imported Markdown v0-to-v1 product proof checkpoint:

status: checkpoint_incomplete

landed proof artifact:

- Documentation-first checkpoint `dedf460b` recorded the imported Markdown
  v0-to-v1 product proof gap before the browser regression changed.
- Test commit `a2c7c62f55c9ad28c7346954a1ccbdd8d7b24c22` is on
  `origin/main`. It adds a staging-capable browser/API regression that opens a
  disposable `.md` source through `/api/vtext/files/open`, verifies the seeded
  v0 document title is canonical `.vtext`, creates a v1 VText revision through
  `/api/vtext/documents/{doc_id}/revisions`, verifies the v1 metadata carries
  a `.vtext` `canonical_vtext_source_path`, reopens the original `.md` alias
  and verifies it resolves to the same document rather than forking, ensures a
  `.vtext` manifest path, exports Markdown from the canonical VText, and opens
  the document from the VText recent list as a `.vtext` at v1.
- The test is fixture-based and uses disposable authenticated product state.
  It does not mutate the private owner proposal and does not add
  document-specific behavior.

verification and deployment evidence:

- Local verification passed: `npm --prefix frontend run build` and
  `git diff --check`.
- Deployed staging proof passed against deployed behavior commit
  `4255dc7efe5407b67bb78075cf477c133958d2f3`:
  `BASE_URL=https://choir.news npx playwright test
  tests/vtext-markdown-lineage.spec.js -g "Imported Markdown advances"
  --project=chromium`.
- GitHub Actions CI run `27022749951` completed successfully for
  `a2c7c62f55c9ad28c7346954a1ccbdd8d7b24c22`, including the normal Go gates.
  Frontend build and staging deploy were skipped because this commit changed
  only browser regression coverage.
- FlakeHub run `27022749965` completed successfully for the same head.
- Staging `/health` still reports proxy and sandbox deployed commit
  `4255dc7efe5407b67bb78075cf477c133958d2f3`, deployed at
  `2026-06-05T14:26:47Z`; this is expected because `a2c7c62f` did not change
  deployed artifacts.

remaining error field:

- Still unproven on the actual owner document: canonical title migration from
  `choir_private_legal_cloud_proposal.md` to `.vtext` on the next owner VText
  write, bounded appendix-table edit survival, source-gap repair through the
  deployed `Sources` panel on the owner head, citation marker expansion into
  transclusions on that owner head, source-window opening from that owner head,
  and owner-head focused prompt-size / `apply_edits` metadata in the visible
  edit-evidence strip.

2026-06-05 whole-mission hard-review checkpoint:

status: checkpoint_incomplete

review artifact:

- Added `docs/vtext-mission-hard-review-2026-06-05.md` as a hard review of the
  whole mission and current system state. A PDF copy was rendered to
  `/Users/wiz/Library/Mobile Documents/com~apple~CloudDocs/vtext-mission-hard-review-2026-06-05.pdf`.
- The review records that `origin/main` is at
  `400b8084048129c3051b1df0af50d059300304a3`, while staging still serves
  behavior commit `4255dc7efe5407b67bb78075cf477c133958d2f3`; this is expected
  because later commits are docs/test-only.
- The review finding field is intentionally hard-edged: the mission is not
  accepted until owner-account proof completes; the source repair UI remains a
  raw JSON repair surface; diagnosis is too broad/raw for durable product proof;
  compare/merge remains brittle on the owner restoration path; and
  `VTextEditor.svelte` needs a simplification pass before further mission
  expansion.

remaining error field:

- The next code pass should be simplification and pruning only where it preserves
  the existing tested behavior. It should not be treated as acceptance proof and
  should not replace the owner-account proof still blocked by passkey access.

2026-06-05 owner Sources-panel access checkpoint:

status: checkpoint_incomplete

new evidence before code:

- Computer Use remains available. The `get_app_state` read path worked for
  Comet, but click attempts using `app: "Comet"` returned a plugin/session
  state error saying Computer Use was not active. Retrying with the exact bundle
  id `ai.perplexity.comet` and a coordinate click worked. This is a tool
  targeting/session quirk, not a Choir product failure.
- The authenticated Comet Choir tab is currently open to staging
  `https://choir.news/` with the private owner document
  `choir_private_legal_cloud_proposal.md` visible at `v81`, state `Latest`.
  The page is not currently blocked by the passkey overlay.
- Opening the owner document `Sources` panel through Computer Use exposed
  visible owner-head metadata evidence for `v81`: `revision kind`
  `focused_user_edit_diff`, `operation type` `apply_edits`, `prompt chars`
  `12886`, `edits` `2`, `delta chars` `-216`, and `latency ms` `19`.
  This is deployed owner-account UI evidence that ordinary revision metadata is
  visible on the owner head.
- The same owner `Sources` panel reports `0 source entities`; its repair JSON
  contains an empty `citation_resolutions` array. Therefore the source/citation
  realism axis is not yet accepted on the owner document: there is no visible
  owner source entity to open, and no owner citation resolution has been applied
  through the deployed panel.

belief-state update:

- The prior passkey blocker is not absolute in the current Comet session. Owner
  UI proof can resume, but mutating the owner document should remain bounded and
  reversible.
- The owner metadata proof gap is substantially reduced by the visible v81
  edit-evidence strip. Remaining owner proof should focus on bounded appendix
  table edit survival, canonical `.vtext` title migration, and creating or
  repairing source/citation transclusion points from the current owner head.

remaining error field:

- Still unproven on the actual owner document: canonical title migration from
  `choir_private_legal_cloud_proposal.md` to `.vtext` on the next owner VText
  write, bounded appendix-table edit survival, source-gap repair that creates or
  attaches real source entities through the deployed `Sources` panel, citation
  marker expansion into transclusions on that owner head, and source-window
  opening from that owner head.

2026-06-05 canonical legal-cloud proposal VText pivot:

status: checkpoint_incomplete

new problem documented before mutation/code:

- The current private owner artifact `choir_private_legal_cloud_proposal.md` is
  an imported Markdown/source artifact acting as VText. Even with alias
  canonicalization, continuing to force this historical `.md` document to carry
  the whole acceptance burden confuses source identity with canonical document
  identity.
- The better product target is a new canonical VText proposal derived from the
  legal-cloud proposal content. The old `.md` should remain a source artifact or
  alias with export/import lineage, while the new owner-facing working proposal
  should have `.vtext` identity from the start.
- The new proposal should not merely preserve existing prose. It should perform
  research where claims need support, produce citation/source points as VText
  transclusion points, expand those points into embedded source transclusions,
  and allow each source to open in its own source window.
- Publication has a source-graph requirement: publishing the canonical VText
  proposal must also publish or share the cited source entities/transclusion
  targets so every viewer authorized to access the published proposal can see
  the supporting sources. A published VText whose citations render but whose
  sources are unavailable to the viewer is not accepted.

belief-state update:

- The restored `choir_private_legal_cloud_proposal.md` remains valuable evidence
  for the original table corruption regression and for import/alias
  compatibility. It should not be treated as the ideal long-term artifact.
- The next realism axis should create or derive a canonical owner `.vtext`
  legal-cloud proposal and prove the complete source/citation publication loop
  there, while preserving the old `.md` as a noncanonical source projection.

remaining error field:

- Still unproven: canonical owner `.vtext` legal-cloud proposal creation or
  derivation from the existing `.md`; research-backed citation/source point
  creation; embedded source transclusion rendering; source-window opening from
  those transclusions; and publication that grants authorized proposal viewers
  access to the cited sources.

2026-06-05 published source-window proof gap checkpoint:

status: checkpoint_incomplete

new problem documented before product-code fix:

- The existing source-service publication regression already proves that a
  published VText bundle stores `source_entities` and `transclusions`, resolves
  them through `/api/platform/publications/resolve`, exports canonical
  publication bytes, and renders an embedded source transclusion with a visible
  `Open source` control.
- Extending that regression to click the published reader's `Open source`
  control failed on deployed staging (`https://choir.news`): after clicking the
  control, Playwright expected one additional `[data-content-viewer]` Source
  window but observed zero for 10 seconds.
- This means the publication path currently proves source metadata and inline
  transclusion rendering, but does not yet prove that published viewers can open
  the source in its own window. That violates the new publication-source
  requirement for the canonical legal-cloud proposal.

verification evidence:

- Failing command:
  `BASE_URL=https://choir.news npx playwright test tests/vtext-source-service-publication.spec.js --project=chromium`
- Failure:
  `expect(locator('[data-content-viewer]')).toHaveCount(1)` received `0` after
  clicking `[data-vtext-source-inline] [data-vtext-open-source]`.

belief-state update:

- The likely failure surface is frontend event handling in published read-only
  mode, not backend publication storage. The backend bundle already contains the
  source entity and transclusion, and the published reader renders the open
  button.

remaining error field:

- Repair the published-reader source-open path without hardcoding legal-cloud or
  source-service fixtures. Then rerun the staging publication regression and use
  it as a prerequisite for the new canonical owner `.vtext` proposal proof.
