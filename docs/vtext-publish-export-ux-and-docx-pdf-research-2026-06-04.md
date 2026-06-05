# VText Publish Export UX And DOCX/PDF Research

Date: 2026-06-04

## Problem

The VText publish result panel overcorrected toward explicit copy controls. The public URL should still become the browser URL after publish, because that is the natural confirmation and sharing affordance for experienced users. The in-app publish menu is still needed for users who do not understand or trust the URL bar.

The current publish panel also makes the URL text too large and exposes low-level publication metadata. The public link should be readable as a compact link, and hashes/version ids should stay hidden in DOM data attributes or diagnostic exports rather than visible product chrome.

Download support is currently too shallow. The platform export path supports `txt`, `md`, and `html` as JSON string exports. VText needs DOCX and PDF downloads, but those should be real server-side publication exports with embedded provenance metadata, not frontend-only conversions.

The inverse direction matters too. Users will bring DOCX and PDF files into
Choir and expect VText to help revise them. Importing those formats into VText
is inherently lossy: DOCX is a package of styled document XML, while PDF is a
fixed-layout presentation format that often lacks semantic reading order. Choir
must never treat an imported VText projection as the original file. The original
DOCX/PDF remains a first-class source artifact, and VText owns a revisable
semantic projection with explicit import evidence and an export profile that
tries to preserve style when producing a new DOCX/PDF.

## Current Code State

- `frontend/src/lib/VTextEditor.svelte` renders the publish panel, copy/open/copy-text/download controls, visible link text, and visible `publication-facts`.
- `frontend/src/lib/vtext.js` calls `/api/platform/publications/export?route=...&format=...`.
- `internal/platform/service.go` normalizes export formats to `txt`, `md`, or `html`; unknown formats are rejected.
- `internal/platform/service.go` currently returns export content as a string in `PublicationExport`, which is not the right transport for binary DOCX/PDF files.
- `internal/platform/source_metadata.go` defaults export policy formats to `txt`, `md`, and `html`.

## Research Summary

Pandoc is the best fit for the first production-grade DOCX/PDF renderer because it already converts Markdown-like structured text to DOCX and PDF and supports reference documents for DOCX styling. The Pandoc manual says `--reference-doc` uses the reference DOCX styles and document properties, including margins, page size, headers, and footers, while ignoring the body content. That matches Choir's need for configurable legal/professional document styles without hand-writing WordprocessingML.

Pandoc is also useful for DOCX import, especially with the `docx+styles` reader
extension. The manual describes that the DOCX reader normally reads styles it
can convert into Pandoc elements, and that enabling styles can preserve custom
style information as structured spans/divs. This gives Choir a practical path:
parse DOCX into a semantic intermediate representation, retain style names and
document properties as import metadata, and create a reference DOCX/profile that
can be reused when exporting a revised version.

Mammoth is a second useful DOCX importer when the target is clean HTML rather
than broad Pandoc AST compatibility. Its style maps can translate Word paragraph
and run styles into semantic HTML. It should be considered as an adapter option
for high-fidelity legal/client documents where preserving named styles,
blockquote conventions, captions, and tables is more important than supporting
every Pandoc input/output feature.

PDF import is materially less reliable than DOCX import. PDF text is commonly
stored as positioned glyphs rather than semantic paragraphs, headings, and
tables. Libraries such as pdfplumber can extract characters, words, page
geometry, and tables using positional analysis, but the import result is still a
best-effort reconstruction. OCR may be required for scanned PDFs, and even
digitally generated PDFs may need table/column heuristics. Therefore PDF import
must record confidence, page selectors, extracted text blocks, table extraction
method, OCR status, and unresolved layout warnings.

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

## Import Architecture

Import should mirror export without pretending to be reversible.

1. Create a `ContentItem` for the original uploaded or synced file:
   - Store original bytes, media type, filename, size, content hash, owner,
     source path, and acquisition method.
   - Keep the original DOCX/PDF available for download, comparison, audit, and
     future higher-fidelity re-import.
   - Treat the original file as the source artifact, not as a transient upload.

2. Create a VText projection:
   - DOCX input: convert through Pandoc `docx+styles` and/or Mammoth style maps
     into VText sections, citations, tables, images, footnotes, comments, and
     style markers.
   - PDF input: extract text blocks, page selectors, figures, tables, and
     citations with page geometry and confidence metadata; OCR scanned pages
     when available.
   - Preserve each imported block's source selector: DOCX XML path/style/run
     where practical; PDF page number, bounding box, extraction method, and text
     hash.

3. Store an import manifest:
   - `original_content_item_id`
   - `original_content_hash`
   - `import_adapter`
   - `import_adapter_version`
   - `semantic_blocks`
   - `style_profile`
   - `asset_manifest`
   - `warnings`
   - `lossiness_score`
   - `roundtrip_profile_id`

4. Let VText revise the projection:
   - The user edits or prompts against VText, not the binary file.
   - VText revisions carry import lineage and block-level source selectors.
   - Comments, footnotes, citations, and tables should survive as structured
     entities whenever the importer can identify them.

5. Export with style preservation:
   - DOCX-imported documents should default to a derived reference DOCX/profile
     based on the original's styles, page size, margins, headings, captions,
     and table styles.
   - PDF-imported documents should usually export through a document profile
     inferred from layout, but Choir should be explicit that exact PDF visual
     roundtripping is not guaranteed.
   - Exports should embed provenance linking the new file to the original import
     artifact and the VText revision lineage.

The product flow should be:

```text
original DOCX/PDF
  -> ContentItem original artifact
  -> import manifest + VText projection
  -> VText revise/compare/merge
  -> export profile
  -> DOCX/PDF/MD/TXT/HTML output with provenance
```

This gives the user an editable semantic document while preserving the legal
and practical truth that the imported VText is a projection, not the original.

## Choir Base Research

`Choir Base` should be the user-facing product name for owner-scoped files,
synced folders, imported artifacts, source manifests, and durable content
available to VText and other apps. There are two separable layers:

- Native device clients that make Choir Base feel like a local/cloud file
  provider on Mac, iPhone/iPad, Windows, and Android.
- A platform service that stores, indexes, versions, authorizes, syncs, and
  serves the files and manifests. This is the "based" Choir platform service.

### Native Device Requirements

Mac:

- Deep Finder integration requires a signed macOS app with a File Provider
  extension. Apple's File Provider framework is available on macOS 11+; the
  replicated extension model handles local copies while syncing with remote
  storage.
- The app/extension needs the right Apple Developer entitlements, App Group
  style shared-container design, notarization for non-App-Store distribution,
  and a user consent/onboarding flow.
- A simpler first Mac client can be a regular signed app with a local Choir
  Base folder, upload/download, and open-in-place behavior, but it will not feel
  like iCloud Drive/Dropbox in Finder until File Provider is implemented.

iPhone/iPad:

- Deep Files app integration requires an iOS/iPadOS app with a File Provider
  extension. Apple's File Provider extension is available to iOS document
  workflows and newer replicated File Provider capabilities are available on
  iOS 16+.
- A lighter app can expose its local Documents directory to Files with
  `UISupportsDocumentBrowser`, `UIFileSharingEnabled`, and
  `LSSupportsOpeningDocumentsInPlace`, but that is local app document sharing,
  not a full cloud sync provider.
- Background sync, large downloads, offline availability, and conflict handling
  are product constraints, not just API calls.

Windows:

- Explorer-grade integration requires a Windows sync provider using the Cloud
  Files API / Cloud Filter API. Windows 10 version 1709+ supports placeholder
  files, hydration, dehydration, sync roots, and Explorer integration.
- The sync provider must register a sync root and satisfy platform access
  requirements; Microsoft documents that registration can fail without write or
  security descriptor access to the sync root.
- A simpler first Windows client can be a signed desktop app that syncs a normal
  folder, but on-demand placeholder behavior needs the Cloud Files API path.

Android:

- System file-picker integration requires an Android app implementing
  `DocumentsProvider` for the Storage Access Framework. Android documents
  providers expose durable local or cloud files to other apps and must be
  declared with provider metadata such as `android:exported="true"`,
  `android:grantUriPermissions="true"`, and
  `android.permission.MANAGE_DOCUMENTS`, which only the system obtains.
- Apps can also consume files through SAF with user-granted URI permissions and
  persistable grants, but that is different from being a provider.
- Android storage is strongly user-mediated; broad filesystem access is not the
  right default product architecture.

### Does Choir Base Require Local Apps?

For upload/download in a browser: no. A web app can upload files, save exported
files, and open web-based VText/source surfaces.

For OS-native cloud-drive behavior: yes. Finder, Files, Explorer, and Android's
document picker each require a local installed client or app/extension/provider.
Those clients must be signed, permissioned, and designed around each OS's file
provider contract. There is no single web-only implementation that gives true
local placeholder sync across all four platforms.

### "Based" Platform Service Requirements

The platform service should not just be generic object storage. It needs to be
the owner-scoped substrate for source artifacts and document roundtrips:

- Object storage: immutable blobs keyed by hash, with encrypted-at-rest storage
  and owner/org tenancy.
- File records: filename, MIME type, source path, size, hash, created/imported
  timestamps, device/source origin, and retention policy.
- Version records: file version lineage, VText projection lineage, export
  lineage, and roundtrip relationships.
- Manifests: import manifests, export manifests, source manifests, asset
  manifests, and lossiness/confidence records.
- Sync protocol: device registration, cursor-based delta feed, resumable
  upload/download, conflict records, tombstones, move/rename semantics, offline
  availability, and hydration/dehydration policy.
- Indexing: full-text, metadata, extracted entities, citations, tables,
  embeddings, and source selectors.
- Authorization: owner/org roles, client sharing, capability-scoped links,
  publication/export policy, revocation, and audit logs.
- App integration: ContentItem resolution for VText/researcher agents, source
  entity creation, transclusion selectors, and publication export inputs.
- Security: malware scanning, file type sniffing, macro handling, sandboxed
  conversion workers, DLP/redaction hooks, and provenance-preserving audit.
- Operations: backfills, retention, backups, restore, per-user quota,
  observability, and admin/owner retrospective query.

The important architecture point: Choir Base should be a platform source/content
service consumed by VText and source/research workflows, not a separate document
editor and not a local-only sync script. Native clients make it feel present on
devices; the platform service makes it trustworthy, queryable, and usable by
Choir's agents.

## Sources

- Pandoc User's Guide, `--reference-doc` behavior for DOCX styling and document properties: https://pandoc.org/MANUAL.pdf
- Pandoc User's Guide, DOCX reader and `docx+styles` behavior: https://www.pandoc.org/demo/example2.html
- Mammoth style maps for DOCX-to-HTML conversion: https://tessl.io/registry/tessl/npm-mammoth/files/docs/style-maps.md
- pdfplumber project documentation and table/text extraction capabilities: https://pypi.org/project/pdfplumber/
- Microsoft Learn, OpenXML custom properties in DOCX: https://learn.microsoft.com/en-us/office/open-xml/word/how-to-set-a-custom-property-in-a-word-processing-document
- pikepdf metadata documentation, XMP and DocumentInfo synchronization: https://pikepdf.readthedocs.io/en/latest/topics/metadata.html
- Apple Developer Documentation, File Provider framework: https://developer.apple.com/documentation/fileprovider
- Android Developers, Storage Access Framework and DocumentsProvider: https://developer.android.com/guide/topics/providers/document-provider
- Android Developers, DocumentsProvider API reference: https://developer.android.com/reference/android/provider/DocumentsProvider
- Microsoft Learn, Cloud Files API portal: https://learn.microsoft.com/en-us/windows/win32/cfapi/cloud-files-api-portal
- Microsoft Learn, `CfRegisterSyncRoot`: https://learn.microsoft.com/en-us/windows/win32/api/cfapi/nf-cfapi-cfregistersyncroot

## Immediate UI Fix

- Restore auto-opening the published URL after publish.
- Keep `Copy link`, `Open link`, `Copy text`, and `Download` in the publish panel.
- Make visible link text compact.
- Hide publication hashes/version ids from the visible panel while keeping them in data attributes for automation and diagnostics.

## Follow-Up Implementation

DOCX/PDF exports should be a separate backend mission because they require binary response design, renderer dependency packaging, metadata policy, and deployed proof with real downloaded files inspected for metadata.

DOCX/PDF import should be part of the same document-roundtrip campaign rather
than a separate one-off importer. The acceptance proof should use at least one
real DOCX and one real PDF, preserve the originals as ContentItems, create
VText projections with import manifests, revise the VTexts, export DOCX/PDF
again, and inspect the exported files for style preservation and provenance
metadata.
