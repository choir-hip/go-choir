# VText Publish Export UX And DOCX/PDF Research

Date: 2026-06-04

## Problem

The VText publish result panel overcorrected toward explicit copy controls. The public URL should still become the browser URL after publish, because that is the natural confirmation and sharing affordance for experienced users. The in-app publish menu is still needed for users who do not understand or trust the URL bar.

The current publish panel also makes the URL text too large and exposes low-level publication metadata. The public link should be readable as a compact link, and hashes/version ids should stay hidden in DOM data attributes or diagnostic exports rather than visible product chrome.

Download support is currently too shallow. The platform export path supports `txt`, `md`, and `html` as JSON string exports. VText needs DOCX and PDF downloads, but those should be real server-side publication exports with embedded provenance metadata, not frontend-only conversions.

## Current Code State

- `frontend/src/lib/VTextEditor.svelte` renders the publish panel, copy/open/copy-text/download controls, visible link text, and visible `publication-facts`.
- `frontend/src/lib/vtext.js` calls `/api/platform/publications/export?route=...&format=...`.
- `internal/platform/service.go` normalizes export formats to `txt`, `md`, or `html`; unknown formats are rejected.
- `internal/platform/service.go` currently returns export content as a string in `PublicationExport`, which is not the right transport for binary DOCX/PDF files.
- `internal/platform/source_metadata.go` defaults export policy formats to `txt`, `md`, and `html`.

## Research Summary

Pandoc is the best fit for the first production-grade DOCX/PDF renderer because it already converts Markdown-like structured text to DOCX and PDF and supports reference documents for DOCX styling. The Pandoc manual says `--reference-doc` uses the reference DOCX styles and document properties, including margins, page size, headers, and footers, while ignoring the body content. That matches Choir's need for configurable legal/professional document styles without hand-writing WordprocessingML.

DOCX metadata should be stored in OpenXML core properties plus custom document properties. Microsoft documents that custom properties live in the `docProps/custom.xml` part, with typed values and stable property names. Choir should use this for compact provenance fields such as `ChoirPublicationID`, `ChoirPublicationVersionID`, `ChoirRoutePath`, `ChoirContentHash`, `ChoirSourceManifestHash`, and `ChoirExportedAt`.

PDF metadata should use XMP metadata, synchronized with the legacy DocumentInfo dictionary for compatibility. The pikepdf documentation notes that PDFs have XMP and DocumentInfo metadata, that DocumentInfo is older/deprecated but still relevant, and that pikepdf's metadata interface can synchronize both. This suggests a pipeline where Pandoc renders PDF, then a post-process step writes XMP/DocumentInfo metadata.

## Recommended Architecture

1. Add a structured export service boundary in platformd:
   - Input: publication route, requested format, caller policy, optional export profile.
   - Output: bytes, media type, filename, content hash, export metadata.
   - Formats: `txt`, `md`, `html`, `docx`, `pdf`.

2. Keep text exports in-process, but move DOCX/PDF through a renderer adapter:
   - `pandoc` for Markdown/VText to DOCX.
   - `pandoc` plus a controlled PDF engine for PDF.
   - Post-process DOCX package metadata with an OpenXML library or zip/XML writer.
   - Post-process PDF metadata with an XMP-aware library.

3. Embed the right amount of metadata:
   - Visible document body: title, content, visible citations/transclusions as normal document content.
   - File metadata: compact identifiers and hashes needed to resolve provenance.
   - Optional embedded manifest: for DOCX, a custom XML part or attached JSON part; for PDF, XMP extension schema or PDF/A-3 attachment later.
   - Avoid embedding private owner ids, access tokens, unpublished revision text, or hidden agent traces in exported files.

4. Introduce export profiles:
   - `clean`: public content plus compact provenance metadata.
   - `provenance`: public content plus citation/source manifest payload.
   - `legal`: conservative layout, citations preserved, metadata compact.
   - Future policy can disable copy/download or restrict formats per publication role.

5. Update APIs:
   - JSON export response remains acceptable for text formats.
   - Binary formats should either return bytes directly with `Content-Disposition` or return a signed/ephemeral download URL.
   - Frontend should show a download menu only for formats allowed by the resolved publication policy.

## Sources

- Pandoc User's Guide, `--reference-doc` behavior for DOCX styling and document properties: https://pandoc.org/MANUAL.pdf
- Microsoft Learn, OpenXML custom properties in DOCX: https://learn.microsoft.com/en-us/office/open-xml/word/how-to-set-a-custom-property-in-a-word-processing-document
- pikepdf metadata documentation, XMP and DocumentInfo synchronization: https://pikepdf.readthedocs.io/en/latest/topics/metadata.html

## Immediate UI Fix

- Restore auto-opening the published URL after publish.
- Keep `Copy link`, `Open link`, `Copy text`, and `Download` in the publish panel.
- Make visible link text compact.
- Hide publication hashes/version ids from the visible panel while keeping them in data attributes for automation and diagnostics.

## Follow-Up Implementation

DOCX/PDF exports should be a separate backend mission because they require binary response design, renderer dependency packaging, metadata policy, and deployed proof with real downloaded files inspected for metadata.
