# VText Fluid Editing, Document Roundtrip, And Transclusion Problem Record - 2026-06-05

Status: problem documentation checkpoint before behavior-changing fixes

## Problem

VText still behaves too much like a Markdown editor wrapped around a large
prompt. Long-document revisions are slow, direct instruction edits can leak
into the final document or append beside stale text, rendered Markdown can
roundtrip back into corrupted source, and non-VText documents such as `.md`,
DOCX, and PDF do not yet have a complete import -> revise -> export lifecycle
that preserves original artifacts and source provenance.

The source/citation layer also needs to become product-complete. A visible
citation marker must be an interactive source entity and transclusion target,
not a decorative marker. Existing versioned Markdown documents, especially the
legal-cloud proposal class of documents, should migrate into real VTexts with
preserved version lineage, source entities, expandable citation transclusions,
and openable source windows.

## Evidence

### Rendered Table Roundtrip Corrupts Markdown

The VText renderer wraps Markdown tables in:

```text
div.table-scroll > table
```

The editor input path serializes the rendered DOM back to Markdown through
`serializeEditorMarkdown` / `serializeBlockMarkdown`. The serializer has a
`table` branch, but the top-level rendered node for a table is the wrapper
`div.table-scroll`, not the table itself. When a user focuses, edits, or saves
after the rendered table exists, the wrapper falls through to generic text
serialization and can flatten table cells into plain text.

Observed product symptom: the appendix/glossary table in
`choir_private_legal_cloud_proposal.md` repeatedly reverted from a pipe table
into flattened text after later user/appagent revisions.

### Long-Document Revision Context Is Too Large

Recent appagent revisions for yusef's legal-cloud proposal took roughly one to
two minutes. The runtime currently builds VText revision prompts with the
current revision, previous revision, diff summary, grounded-history state,
recent worker messages, and up to 200 user revision diff summaries. This
preloads too much context for ordinary edits and pushes the model toward
large-context generation.

The desired inverted behavior is current head plus the user's direct edit diff
by default, with prior versions, source metadata, worker messages, import
manifests, and publication records retrieved by VText tools only when needed.

### Instruction Edits Are Not Yet A Clean Document Delta

The intended UX is seamless: the user edits the document directly. That edit
can be literal replacement text, scratch instruction, deletion, annotation, or
a mix. VText should interpret the diff as intent. Current behavior has shown
two failure classes:

- instruction-like text can remain in the final document when it should be
  consumed;
- replacement content can be appended while the stale target text remains.

Prompt/content classifiers or required markers such as `//<edit>` would be the
wrong fix. The document delta itself is the control surface.

### Non-VText Documents Are Still Special Cases

Markdown files can open through VText-like UI, and some paths still write
through to source paths. That is unsafe once the user begins VText revision,
compare, merge, citation, or publication work. The correct behavior is to
preserve the original `.md` as a source artifact and create a VText working
projection with migration/import lineage.

DOCX and PDF imports are inherently lossy. They require original preservation,
import manifests, selector/confidence data, style/export profiles, and
server-side export artifacts. They cannot be treated as frontend-only format
conversions.

### Versioned Markdown Lineage Has No Product Migration Path

After the import/export checkpoints, a single `.md` file can be opened as a
VText projection with an original ContentItem and migration manifest. That is
not enough for the existing legal-cloud proposal class of documents. Those
documents are valuable precisely because they have historical versions: older
versions can have better appendix/table formatting, better glossary structure,
or source material worth merging into the current draft.

Current runtime APIs only support:

- opening one file path as one initial VText revision;
- creating later revisions one at a time after the document already exists;
- manually attaching arbitrary metadata to those revisions.

There is no owner-authenticated product API that accepts a Markdown lineage,
preserves each source version as migration evidence, creates a single VText
document, imports each historical Markdown snapshot as ordered VText revisions,
records the original source path/hash for each migrated version, and aliases
the canonical VText document back to its source path. Without that path, bulk
migration is either a one-off database/script operation or a lossy import of
only the latest file content. Both would violate the mission invariant that
existing versioned Markdown documents become real VTexts with preserved version
lineage, historical publish/compare/merge, and later citation/source repair.

### Markdown Lineage Import Cannot Yet Attach Known Source Evidence

The first lineage import endpoint creates durable VText revisions and records
unresolved numeric/footnote citations as repairable `source_gaps`. That is the
right behavior when evidence is missing, but it is incomplete when the migration
input already has source evidence. Existing versioned Markdown documents may
include footnotes, source sections, appendix references, quoted excerpts, URLs,
or manually known citation mappings. The product path needs to carry that
evidence into revision-scoped `source_entities` and convert resolved citation
markers such as `[1]` into live `source:` transclusion targets.

Without that capability, migrating the legal-cloud proposal class of documents
would require a follow-up script or manual database edit to attach sources.
That would be a workaround, not the intended product path. The importer should
distinguish between unresolved markers, which remain repairable source gaps,
and resolved markers, which become clickable citation/transclusion points in
the migrated VText content.

### Resolved Markdown Lineage Citations Used Non-Renderable Source Syntax

Staging proof after commit `5745c6f3` showed that the source-aware lineage
import API accepted known `source_entities`, preserved unresolved marker `[2]`
as a source gap, and rewrote resolved marker `[1]` into VText content as:

```text
[[1]](source:ENTITY_ID)
```

That syntax preserved the visual marker text but did not render as a clickable
VText source reference in the app. The VText renderer intentionally recognizes
canonical inline source syntax shaped as:

```text
[label](source:ENTITY_ID)
```

where `label` cannot contain `]`. Therefore a migrated citation marker cannot
use the full bracketed marker as its Markdown link label. The migration
projection must preserve the original bracketed marker in metadata while
rewriting the VText working projection into renderer-compatible source syntax
such as `[1](source:ENTITY_ID)`.

### Binary Original Preservation Is Still Projection-Only

After checkpoint `0a5a31de`, the backend creates separate original
`ContentItem` rows for `.md`, DOCX, and PDF file opens, and the first VText
revision records an import manifest. That repaired the identity boundary, but
the DOCX/PDF path still depends on the caller sending `initial_content` text.
The runtime does not yet read the original bytes from the owner filesystem,
does not hash the real binary bytes, and does not run a real import adapter to
create the VText projection.

The frontend also only calls `/api/vtext/files/open` from
`handleOpenTextFile`. Files whose names route to media apps, including PDF,
open in media surfaces instead of offering an import-to-VText path. DOCX is not
handled as a first-class VText import target in the file browser either.

This means the current deployed state can prove "VText projection plus
preserved original record," but not the required DOCX/PDF
`original bytes -> ContentItem -> import manifest -> VText projection`
roundtrip.

### Publish Download UI Exposes DOCX/PDF Before Backend Export Exists

After the first mission checkpoint, staging served commit
`19f41da9d649395bb010480a45a7c278ff890fa4`. The public export endpoint
successfully returned Markdown, Text, and HTML for an existing published VText
route, but:

```text
GET /api/platform/publications/export?route=/pub/vtext/staging-long-compare-merge-proof-1780614390072-pub32bd3c150&format=docx
-> 502 {"error":"failed to export publication"}
```

The proxy was forwarding to platformd correctly; platformd rejected the format
because `normalizeExportFormat` only accepts text-like exports. The durable fix
belongs in the platform publication export service, with canonical
publication-bundle bytes and compact provenance metadata, not in a frontend
download shim.

### Citation Markers Are Not Complete Without Transclusion And Source Windows

Existing docs define `source_entities`, display policies, and every citation
as a transclusion point. The product requirement is stronger than storage:
visible citation markers must expand inline, quoted excerpts should default to
embedded transclusion, and expanded transclusions must be able to open their
owning source surface in a separate app/window when a source artifact exists.

Current frontend code in `frontend/src/lib/VTextEditor.svelte` already renders
canonical `[label](source:ENTITY_ID)` Markdown into clickable
`data-vtext-source-ref` markers, and it can render a source rail containing
`data-vtext-source-inline` details blocks. However, clicking a citation marker
currently toggles an absolutely positioned popover and separately opens or
scrolls a source rail. That proves that source entities are present, but it is
not the same product behavior as "the citation itself expands into an inline
transclusion at the citation point." This matters because users read the
source in context, not in a detached top rail or hover popover.

The default display policy is also inconsistent: frontend-derived media refs
set `display.inline_mode: "chip"`, while `sourceEntityDisplayPolicy` only
recognizes `embedded_excerpt`, `embedded_preview`, `expanded`, and
`collapsed_citation`. That mismatch turns media refs into collapsed citations
by fallback and makes default embedding depend on side effects instead of the
source entity contract.

Staging QA after commit `d755a58f` found a sharper version of the same issue:
the citation marker expanded in-flow and showed the source title, but the
inline transclusion did not include the YouTube iframe. The only media embed
still lived in the separate source rail. That leaves multimedia transclusion
half-shipped: the citation marker becomes expandable, but users must look
elsewhere to inspect the media source.

### Diagnosis Bundles Can Omit The Relevant VText Run

Live staging proof for the long-document fluid-edit path created a VText
document, direct user-edit revision, and appagent revision successfully. The
appagent revision metadata recorded the VText context mode and structured edit
operation, but `/api/vtext/documents/{id}/diagnosis?limit=100` did not include
the `loop_id` returned by the document's own `/revise` call in `runs`.

The endpoint currently appends recent owner runs through `ListRunsByOwner`.
That is useful for broad retrospective context, but it is not sufficient for a
document diagnosis bundle: active or recent runs on the document channel can be
older than the last N owner runs, especially in QA accounts with multiple
windows and tests. For VText debuggability, a document-scoped diagnosis must
include runs whose channel/doc metadata points at the document before falling
back to owner-level recent runs.

## Desired State

- Ordinary VText revisions use current head plus user edit diff by default.
- VText retrieves extra context through tools, not runtime classifiers or
  prompt preloading.
- Direct document edits are interpreted as intent without required meta syntax.
- Instruction text is consumed when not intended as prose.
- Replacement edits remove stale target text.
- Tables, lists, citations, transclusions, and hidden metadata survive
  focus/edit/save/revise/reload.
- Existing versioned Markdown docs migrate into real VTexts with durable
  version lineage.
- Markdown lineage migration is exposed through an authenticated product API,
  not a raw database script, and records each imported Markdown snapshot as
  migration evidence linked to the resulting VText revision.
- Markdown lineage migration accepts known source entities and citation-marker
  resolutions, stores them as revision metadata, and rewrites resolved markers
  into live VText source links while leaving unresolved markers as repairable
  source gaps.
- `.md`, DOCX, and PDF originals are preserved as source artifacts or
  ContentItems; VText owns the revisable projection.
- Import -> revise -> export works for MD, TXT, HTML, DOCX, and PDF with
  provenance metadata and policy.
- DOCX/PDF import reads the original owner-file bytes on the server, stores a
  real binary content hash, creates a best-effort VText projection through an
  explicit adapter, and records adapter warnings/lossiness rather than relying
  on caller-supplied projection text.
- Every citation marker is an expandable source entity/transclusion target.
- Expanded transclusions can open their owning source app/window.
- Document diagnosis includes the document's own VText/appagent runs even when
  they are not among the latest owner-level runs.

## Remaining Error Field

The implementation boundary is not yet final. The current code already has
source entity rendering, publication metadata preservation, VText edit tools,
ContentItems, PDF app support, and publication export for text-like formats.
The mission must determine whether the current edit tool schema is enough for
fast high-quality structured edits or whether stronger block selectors are
needed. DOCX/PDF roundtrip may require new renderer/adapter packaging and may
need to land behind explicit export format policy while preserving the
canonical artifact boundary.
