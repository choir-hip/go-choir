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
- `.md`, DOCX, and PDF originals are preserved as source artifacts or
  ContentItems; VText owns the revisable projection.
- Import -> revise -> export works for MD, TXT, HTML, DOCX, and PDF with
  provenance metadata and policy.
- Every citation marker is an expandable source entity/transclusion target.
- Expanded transclusions can open their owning source app/window.

## Remaining Error Field

The implementation boundary is not yet final. The current code already has
source entity rendering, publication metadata preservation, VText edit tools,
ContentItems, PDF app support, and publication export for text-like formats.
The mission must determine whether the current edit tool schema is enough for
fast high-quality structured edits or whether stronger block selectors are
needed. DOCX/PDF roundtrip may require new renderer/adapter packaging and may
need to land behind explicit export format policy while preserving the
canonical artifact boundary.
