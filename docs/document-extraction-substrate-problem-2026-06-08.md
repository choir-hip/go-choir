# Document Extraction Substrate Problem

**Date:** 2026-06-08  
**Status:** active problem record  
**Mission:** `docs/mission-natural-compaction-pdf-recall-eval-v0.md`

## Problem

Choir cannot honestly run a frozen-corpus natural compaction recall eval yet
because researcher and VText document acquisition are not backed by a shared,
format-aware document extraction substrate.

Current evidence from the codebase:

- user/candidate computer image config includes `python3` and `nodejs`, but not
  declared document extraction tools such as Poppler, Pandoc, LibreOffice, or
  Python extraction libraries;
- `internal/runtime/content.go` imports ordinary URLs into `ContentItem`, but
  non-HTML/non-text responses do not get real extracted document text;
- `internal/runtime/tools_research.go` exposes `import_url_content` and
  `read_content_item`, but not selector-aware document tools for pages, slides,
  headings, or chunks;
- `internal/runtime/vtext_import.go` has VText-private DOCX/PDF projection
  logic instead of shared ContentItem extraction;
- the PDF path is `pdf_literal_text_projection`, a best-effort regex over PDF
  literal strings, which is not adequate for research, VText import, Global Wire
  source grounding, or compaction recall evaluation;
- PPTX/HTML slide decks are not represented as source documents with per-slide
  selectors, even though they matter for research and future Slides app work.

## Why It Matters

The compaction matrix should measure model/provider memory behavior after
automatic compaction. If document extraction is weak, the eval instead measures
whether the input corpus was unreadable or truncated before the model ever saw
it. That would burn model budget and produce misleading evidence.

Global Wire also needs the same capability: sources must be real full articles,
documents, filings, reports, decks, and social/media artifacts with provenance,
not headline stubs or lossy snippets.

## Desired Direction

Build one shared document-source substrate:

```text
URL or file bytes
  -> ContentItem raw hash/provenance
  -> cleaned text
  -> extraction adapter metadata and warnings
  -> addressable selectors
  -> researcher/VText/source-app reads
```

VText import should consume this substrate instead of maintaining separate PDF
and DOCX hacks. Researcher tools should read full documents through bounded,
selector-aware access rather than relying on `fetch_url` snippets.

## Non-Goals

- Do not build the Slides app in this mission.
- Do not prioritize native Keynote or native Google Slides formats.
- Do not spend live search API quota during the scored compaction eval.
- Do not use local macOS tools or Node B host PATH as proof of user/candidate
  computer capability.

## First Implementation Target

1. Declare real document tools in the user/candidate computer NixOS image.
2. Add shared runtime extraction helpers for PDF, DOCX, EPUB, PPTX, and HTML.
3. Store selector metadata in `ContentItem.Metadata`.
4. Expose selector-aware researcher reads.
5. Route VText file import through the shared extraction helper.
6. Only then run frozen-corpus compaction recall across model arms.
