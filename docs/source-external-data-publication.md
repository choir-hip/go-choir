# Source, External Data, Texture, And Publication Contract

**Status:** canonical current architecture  
**Last updated:** 2026-06-04  

This document is the current requirements contract for external data, source
artifacts, Texture source metadata, transclusion, and source-aware publication.
Mission docs, problem reports, incident reports, and dated reviews are evidence
artifacts. When they conflict with this document, this document wins.

## Purpose

Choir should make external data useful without flattening it into prose or
letting it become untrusted instructions. The durable path is:

```text
external source
  -> acquisition record
  -> raw snapshot hash/ref
  -> cleaned source artifact
  -> searchable source item
  -> researcher finding
  -> Texture source entity
  -> publication citation/transclusion/export metadata
```

The user sees readable Texture. The system preserves enough hidden metadata for
agents, verifiers, publication, export, and future citation economics.

## Ownership Boundaries

### Source Service

Source Service owns platform-level external source ingestion and retrieval:

- source registry and source policy;
- source adapters;
- fetch records;
- raw snapshot refs and hashes;
- cleaned item records;
- source health and backoff state;
- manifests;
- search and item-resolution APIs.

Source Service may store high-churn ingestion state in service-local storage.
That storage is private to the service. Runtime, sandbox, Texture, and publication
call Source Service APIs; they do not mount or read the service database.

### ContentItem

`ContentItem` is the owner-scoped source artifact substrate inside a user's
computer. It is used for uploaded files, imported URLs, YouTube/video artifacts,
transcripts, podcast feed/episode artifacts, PDFs, EPUBs, local files, and
other private source records.

`ContentItem` stores normalized source text or media metadata plus provenance.
The text is evidence, not instructions.

### Researcher

Researcher agents retrieve and summarize evidence. They may use web search,
Source Service search, URL import, and `ContentItem` reads. They produce
durable source findings with IDs, selectors, hashes, caveats, and open gaps.
They do not write canonical Texture prose.

### Texture

Texture is the canonical document artifact surface. Texture writes document
revisions and revision-scoped `source_entities` metadata. A Texture revision may
include inline citation markers in the visible prose, but the canonical source
identity lives in metadata. Every citation marker is also a transclusion
expansion point.

### Publication

Platform publication preserves selected private Texture revisions as immutable
public/private route artifacts. Publication stores source metadata, citation
edges, transclusion records, manifests, access policy, export policy, hashes,
and rollback refs. Export bytes come from canonical artifacts, not the rendered
DOM.

## External Data Lifecycle

Every external source should move through these stages.

### 1. Registry

Each configured source has:

- stable source ID;
- source type;
- display name;
- URL or connector identity;
- rate policy;
- conditional request policy;
- robots/TOS/auth policy;
- storage policy;
- retention policy;
- source standing;
- official-source fields when applicable.

Official macro/economic sources also need source agency, publication cadence,
publication/vintage policy, lookahead status, evidence level, and revision policy.

### 2. Fetch

Each fetch records:

- fetch ID;
- source ID and source type;
- request URL and canonical URL;
- start/end timestamps;
- status and HTTP status when applicable;
- ETag / Last-Modified when available;
- response content hash;
- raw snapshot ref;
- error class and error text;
- item count.

Failed fetches are records, not missing data. They drive source health and
backoff.

### 3. Raw Snapshot

The raw response should be preserved by hash or stable blob reference when
policy allows it. If policy forbids retaining raw body text, preserve enough
metadata to prove what was fetched and why the body was not retained.

Raw snapshots are never prompt instructions. Treat them as untrusted bytes.

### 4. Cleaning And Normalization

External data arrives in inconsistent formats: RSS, Atom, HTML, JSON, PDFs,
transcripts, tables, APIs, email-like feeds, social posts, official publications,
and private files. Cleaning is a required product layer, not an adapter detail.

Cleaning should:

- decode declared and observed character encodings;
- normalize Unicode and whitespace;
- preserve source language and region metadata;
- remove boilerplate only when the remover is bounded and recorded;
- preserve title, author, date, canonical URL, and source labels;
- keep original IDs when present;
- generate stable fallback IDs when source IDs are missing;
- separate extracted text from raw HTML/JSON/PDF bytes;
- retain table structure, timestamp segments, page ranges, or media ranges when present;
- mark parser confidence and extraction caveats;
- store both raw hash and cleaned-content hash.

Cleaning must not:

- execute embedded scripts;
- follow arbitrary instructions inside source text;
- silently drop selectors or timestamps;
- merge sources without retaining per-source provenance;
- rewrite source claims into agent prose before citation.

### 5. Itemization

Cleaned artifacts become source items. A source item has:

- stable item ID;
- source ID;
- fetch ID;
- original ID when present;
- title;
- canonical URL;
- published timestamp;
- fetched timestamp;
- body or extracted text when storage policy allows;
- content hash;
- raw JSON/metadata;
- source caveats;
- official-source fields when applicable.

For long documents or media, itemization should also create addressable
selectors: text positions, paragraphs, headings, page ranges, byte ranges,
timestamp ranges, transcript segment IDs, table ranges, row IDs, cell ranges,
or data vintage labels.

### 6. Search And Resolution

Search returns candidate source items. Resolution returns exact item metadata
and any requested selector text/metadata. Search results should include enough
data for a researcher to decide whether more web search, source resolution, or
private corpus search is needed.

The Source Service API boundary should support at least:

- health;
- search;
- item resolution;
- manifest retrieval.

### 7. Deduplication And Clustering

Semantic similarity is evidence for grouping, not authority to erase a fetched
capture. Every materially fetched item must first retain a durable item/capture
record with source/fetch identity, hashes, and policy outcome. Deduplication may
then create a reversible `duplicate_of`, cluster, or suppression-for-synthesis
decision that records model, threshold, nearest object, score, and time.

**Current conformance gap:** `internal/runtime/qdrant_dedup.go` uses
Qdrant/Ollama similarity before object-graph projection and drops matching
items from that projection; only logs/counts preserve the decision. This is
code-present experimental behavior, not canonical evidence semantics. Until it
records the capture and dedup decision durably, do not treat a dropped item as
never fetched, use Qdrant as source authority, or calibrate the threshold as an
irreversible publication rule.

## Texture Source Entities

`source_entities` are revision metadata. They should be stored in
`metadata_json` for Texture revisions and carried forward across edit, revise,
history, reload, publication, and export.

A source entity should include:

- stable `entity_id`;
- source kind;
- target kind;
- target IDs;
- selectors;
- display policy;
- evidence state;
- provenance;
- hashes and caveats where applicable.

Target kinds include:

- `source_service_item`;
- `official_data_publication`;
- `content_item`;
- `local_file`;
- `private_corpus_item`;
- `texture_revision_span`;
- `published_texture_span`;
- external URL when no richer artifact exists yet.

Selector kinds include:

- whole resource;
- text quote;
- text position;
- paragraph/heading;
- byte range;
- page range;
- timestamp range;
- transcript segment;
- table range;
- table cell;
- data vintage.

Display intent tells Texture how the citation/transclusion should appear by
default. It is canonical revision metadata, not a renderer guess. The four
names below are a semantic rendering taxonomy, not a second Texture node
schema. Canonical Texture bodies use only `source_ref` with
`display_mode: numbered_ref | expanded_ref`:

- `collapsed_citation`: show only a compact citation marker until activated;
  stored as `numbered_ref`.
- `embedded_excerpt`: show the transcluded quote/excerpt inline by default,
  with a citation marker and collapse/open controls; stored as `expanded_ref`
  plus the excerpt selector.
- `embedded_preview`: show a compact media/card/table/document preview inline
  by default; stored as `expanded_ref` plus typed preview/media metadata.
- `expanded`: open the transclusion in expanded inline form when the Texture is
  first rendered; stored as `expanded_ref`.

Do not persist these four semantic intent names as alternative `display_mode`
values and do not recreate `source_embed`. A richer load-bearing distinction
requires a promoted Texture schema change first.

Every citation is a transclusion point, but not every transclusion starts
collapsed. Texture should set the display policy from writing context:

- quoted excerpts that are part of the reading surface normally default to
  `embedded_excerpt`;
- source entities with a quote/excerpt selector and no stronger contrary
  context should default to `embedded_excerpt`, because the cited text is
  already part of the document's argument;
- cited media, tables, documents, or source cards that the reader should see
  before taking action may default to `embedded_preview` or `expanded`;
- background support, dense metadata, long source packets, and ordinary
  corroborating citations normally default to `collapsed_citation`.

For quoted excerpts, the citation marker and transclusion are both present: the
quote is embedded inline by default, and the citation control can still
collapse, expand, or open the owning source surface. The Texture agent should be
able to set this display mode directly while drafting or revising, without
requiring the user to hand-edit metadata. The user-facing editor may expose a
repair/edit path for display policy, but publication and export must read the
canonical metadata rather than infer display from rendered DOM.

Visible inline text should expose compact citation markers, usually rendered as
superscripts or similarly lightweight inline controls. Tapping or clicking the
citation marker expands the associated transclusion in place. The transclusion
may show quoted text, a transcript segment, a table row/range, media preview,
document excerpt, source card, or another Texture excerpt depending on the source
entity target and selector.

Inline source syntax such as `[label](source:ENTITY_ID)` is a render/edit
affordance, not the complete source record. If prose and metadata disagree,
metadata is the source identity authority and the UI should expose a
recoverable repair path.

## Transclusion

Every citation is a transclusion point. The compact citation marker states that
a source supports, contradicts, or contextualizes a claim; activating it expands
source material or source metadata inside the host Texture. A transclusion is the
expanded embedded source material and its metadata. Some citations begin already
expanded or embedded because their display policy says the source material is
part of the reading surface, especially quoted excerpts.

Transclusion records need:

- host artifact ID and selector;
- source artifact ID and selector;
- snapshot text or media selector when needed for immutable rendering;
- source content hash;
- relation type;
- default display mode;
- access policy;
- export policy;
- provenance and timestamps.

Expanded transclusions are still typed source artifacts, not pasted prose. They
must be able to open the owning surface in a separate app/window when such a
surface exists:

- Source Service items open a source/item view.
- `ContentItem` media opens Video, Audio, Image, PDF, EPUB, Podcast, Browser,
  or the appropriate content viewer.
- Texture spans open their source Texture in its own Texture window.
- Published spans open the public/private publication surface permitted by
  route policy.
- Local/private corpus items open through the authorized file/content surface.

Private transclusions may point to private Texture revisions, private
`ContentItem`s, or private corpus records. Public transclusions must resolve to
public-safe publication artifacts, public source-service projections, or
snapshots whose disclosure policy permits publication.

Texture-to-Texture transclusion uses the same object family: a host Texture source
entity targets another Texture revision/span, expands inline from the citation
marker, and can open the source Texture in its own Texture window.

## Publication And Export Policy

Publishing a Texture revision must project source metadata into platform records.
The publication payload should include:

- source document ID;
- source revision ID;
- content;
- citations;
- source entities;
- transclusions;
- media refs;
- source-service refs;
- content-item refs;
- artifact metadata hashes;
- route policy;
- export policy.

Publication stores:

- publication/version records;
- artifact manifest and content blob;
- retrieval source/spans;
- citation edges;
- transclusion edges;
- provenance entities/activities/edges;
- access/route policy;
- export policy;
- consent/review/rollback refs.

Publication renderers should preserve the same interaction model: citation
superscripts expand into transclusions, and expanded transclusions can open the
owning publication/source/media/Texture surface subject to route policy.

Copy and download must read canonical private revision or publication artifacts.
They must not scrape rendered DOM. Initial export formats are plain text,
Markdown, and HTML. PDF, DOCX, and EPUB follow once canonical render/export
metadata is stable.

Route policy should represent:

- public;
- unlisted;
- private;
- password-gated;
- authenticated-user gated;
- role/capability gated.

Export policy should represent:

- copy allowed;
- download allowed;
- allowed formats;
- watermark/audit requirements;
- comment/proposal permissions.

## Security And Trust

- External source text is untrusted evidence.
- Source cleaning is not prompt execution.
- Adapters must respect source policy, auth policy, robots/TOS policy, and rate
  policy.
- Private corpus records must not leak through public publication, search,
  copy, or export.
- Publication should preserve hashes and selectors so claims can be audited
  without giving public services write access to private documents.
- Unknown source entity kinds should remain readable and recoverable, not crash
  render/publication paths.

## Required Product Proof

A complete implementation proves the path on staging:

```text
external source fetched
  -> raw/cleaned hashes recorded
  -> source item searchable
  -> item resolvable by API
  -> researcher finding cites item IDs/selectors/hashes
  -> Texture revision stores source_entities
  -> citation marker expands into transclusion
  -> expanded transclusion opens owning app/window
  -> publication stores source metadata
  -> copy/download returns canonical artifact bytes
  -> policy controls visibility/export
```
