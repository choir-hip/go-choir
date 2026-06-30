# Publication Rich Export Design

Status: `draft_design_before_implementation`.

Related mission problem:
`docs/mission-source-system-loop8-simplify-v0.md`, Problem L8-4.

## Problem

Published VText exports currently produce valid containers for DOCX, HTML, and
PDF, but the formatted outputs still expose Markdown syntax. That is not an
acceptable export contract. A user who downloads a `.docx`, `.html`, or `.pdf`
should receive a document native to that format, with source provenance carried
as both visible citations and embedded metadata.

## Format Research Summary

### Shared Principle

Export should not be `Markdown string -> container`. It should be:

```text
VText/publication bundle
  -> canonical PublicationDocument AST
  -> format renderer: HTML | DOCX | PDF | Markdown | Text
  -> metadata/provenance embedding layer
```

The AST should preserve:

- block structure: headings, paragraphs, lists, tables, blockquotes, code,
  horizontal rules;
- inline structure: emphasis, strong, code, links, source references;
- source references: source entity id, marker, label/title, URL/open surface,
  evidence state, selector, snapshot text/hash, reader artifact state;
- document metadata: publication id/version, route, content hash,
  source revision hash, export policy, access policy, generation time.

### HTML

HTML should use semantic document markup and structured metadata:

- headings as `<h1>` through `<h6>`;
- paragraphs/lists/tables as native HTML;
- inline emphasis as `<em>`/`<strong>`, code as `<code>`;
- source references as real anchors, for example
  `<a class="vtext-source-ref" href="#source-src_..." data-source-id="...">`;
- a generated `Sources` section with one entry per source entity/snapshot;
- embedded JSON-LD in `<script type="application/ld+json">` containing a
  `CreativeWork`/`ScholarlyArticle`-style graph for the publication and cited
  sources;
- optional `data-choir-*` attributes for exact round-trip provenance.

This gives browsers a normal readable document, gives machines structured
metadata, and keeps source details inspectable without polluting body text.

### DOCX

DOCX should be real WordprocessingML:

- document body in `word/document.xml`;
- built-in or profile-defined paragraph styles for Title, Heading1-Heading6,
  Normal, ListParagraph, Quote, table header/cell styles;
- inline runs for bold/italic/code-like text instead of literal Markdown;
- source references rendered as footnote or endnote references by default for
  legal/professional documents;
- external source URLs represented as OOXML hyperlink relationships when a URL
  is allowed;
- a `Sources` appendix/endnotes section for richer source metadata when
  footnotes would be too verbose;
- full source/provenance manifest embedded in a custom XML part, with package
  relationships and custom document properties pointing to the manifest.

Comments should not be the default citation mechanism because they read as
review markup and are often hidden, removed, or treated as unresolved review
state. Footnotes/endnotes are more document-native. Comments can be an export
profile option for internal review exports.

### PDF

PDF should be rendered from the same PublicationDocument AST, not from raw
Markdown lines:

- page layout with title, headings, paragraphs, lists, tables, page breaks, and
  running pagination;
- source references as visible superscript markers and link annotations where
  URLs/routes are allowed;
- source appendix/endnotes in the PDF body;
- document-level XMP metadata with a Choir namespace containing publication and
  source manifest identifiers;
- optionally embed the full JSON source manifest as an associated file for
  PDF/A-3 or PDF 2.0-style exports when archival/provenance mode is enabled.

PDF viewers vary widely in how much custom metadata they expose. Therefore,
PDF should carry visible source references plus embedded metadata, not embedded
metadata alone.

## Source Embedding Contract

For each export, produce one normalized source manifest:

```json
{
  "schema": "choir.publication_sources.v1",
  "publication_id": "...",
  "publication_version_id": "...",
  "route_path": "...",
  "content_hash": "...",
  "source_revision_hash": "...",
  "generated_at": "...",
  "sources": [
    {
      "source_entity_id": "...",
      "title": "...",
      "url": "...",
      "open_surface": "source_viewer",
      "evidence_state": {"state": "confirms"},
      "reader_artifact_state": "available",
      "selector": {...},
      "snapshot_text": "...",
      "snapshot_hash": "sha256:..."
    }
  ],
  "transclusions": [...]
}
```

Use that same manifest in every format:

- HTML: JSON-LD plus `<script type="application/json" id="choir-source-manifest">`.
- DOCX: `/customXml/item1.xml` or JSON-in-XML manifest part plus
  `docProps/custom.xml` fields referencing the schema/version.
- PDF: XMP metadata summary plus optional embedded JSON associated file.

Visible citation rendering should use the same source ids and labels in every
format. The user experience should be:

- body text has small source markers or footnotes;
- exported document has a readable `Sources` appendix;
- machines can recover the full source graph from embedded metadata.

## Customization And Extensibility

Add export profiles rather than hard-coding a single house style:

```text
PublicationExportProfile
  id
  name
  typography
  heading numbering
  paragraph spacing
  table styling
  citation placement: inline | footnote | endnote | appendix | comments
  source detail level: markers | labels | excerpts | full snapshot appendix
  logo/header/footer
  jurisdiction/firm template hooks
  metadata embedding policy
```

Initial profile:

- `default-professional`: conservative legal/business document styling;
- footnote source markers for DOCX/PDF;
- source appendix for all rich formats;
- JSON manifest embedded in all formats.

Future profiles:

- law firm house style;
- court filing style;
- client memo style;
- public web article style;
- archival PDF/A-3 profile.

## Implementation Plan

1. Add `PublicationDocument` AST in `internal/platform` or a narrower
   `internal/publicationexport` package.
2. Convert publication artifact Markdown plus source/transclusion metadata into
   the AST.
3. Implement HTML renderer from AST with semantic markup, source appendix, and
   embedded source manifest.
4. Upgrade DOCX renderer to:
   - emit styles;
   - render inline runs;
   - emit footnotes/endnotes or source appendix;
   - include custom XML source manifest.
5. Upgrade PDF renderer to:
   - render from AST blocks;
   - add visible citations/source appendix;
   - include XMP/source manifest metadata.
6. Add tests that assert absence of raw Markdown markers in HTML/DOCX/PDF and
   presence of embedded source manifest in every rich export.
7. Run local focused tests, CI, deploy, staging export proof, and inspect
   downloaded artifacts visually.

## Performance Notes

The expensive work is not CPU; these exports are string/XML/PDF generation over
one publication version. The performance constraint is to keep renderers
single-pass over the AST and avoid network calls during export. Source snapshots
and metadata must come from the immutable publication bundle already loaded by
the export endpoint.

## References

- ECMA-376 Office Open XML standard:
  https://ecma-international.org/publications-and-standards/standards/ecma-376/
- Microsoft custom XML parts overview:
  https://learn.microsoft.com/en-us/visualstudio/vsto/custom-xml-parts-overview
- Microsoft Office custom XML parts notes:
  https://learn.microsoft.com/en-us/openspecs/office_standards/ms-oe376/52769434-bde1-4e81-a128-7001873acb2b
- OOXML package/custom properties reference:
  https://ooxml.info/docs/15/15.2/15.2.12/
- W3C JSON-LD 1.1:
  https://www.w3.org/TR/json-ld11/
- Schema.org ScholarlyArticle / citation patterns:
  https://schema.org/ScholarlyArticle
- Adobe XMP specifications:
  https://developer.adobe.com/xmp/docs/xmp-specifications/
- PDF Association associated files overview:
  https://pdfa.org/files-inside-pdf/
- PDF/A-3 custom metadata note:
  https://pdfa.org/resource/iso-19005-3-pdf-a-3/
