# Office group: spreadsheet, documents, presentation, pdf, notes

Reviewer: office. Source: `repo/src/<app>` (HeyPuter/everything, MIT, shallow
clone at commit a7f61cdc). Nothing in the clone was executed. Line counts
come from `wc -l`; test counts are `it(`/`test(` occurrences.

## How this code was made

All five apps share a house style. They are vanilla JS on Vite, with no
framework, and Puter.js is the only backend (`src/lib/puter.js` stores
files in the user's Puter cloud). Comments are long and written in
consistent prose, and the tests are very large: thousands of cases checked
against fixtures probed from native Office. The spreadsheet's built-in
assistant pins `AI_MODEL = 'claude-opus-5-5'`
(`spreadsheet/src/core/aiagent.js:18`). This is agent-generated code built
over many iterations against oracles, not hand-written code. It is not
"slop" in the sense of broken: the engines are real and heavily tested.
But it has the agent-fidelity-chase pathology: hundreds of tiny modules
that each chase one more Excel or Word edge case
(`spreadsheet/src/core/native-pivot-*.js` is about 45 files, and
`documents/src/engine` has 310 files, including `legacy-kern-overflow.js`
and `drawing-3d-lighting.js`), plus 10-15k-line monolithic UI files. The
reusable value is in the **engines, the file-format code, and the oracle
fixture corpora**. The UI shells have little value.

Choir mapping used below (see `internal/objectgraph/object.go` and
`registry.go`). An object is `{canonical_id obj:<kind>:<owner>:<suffix>,
kind, owner, computer, version_id, content_hash, body, metadata}`. Edges
are typed (`cites`, `derived_from`, `contains`, `references`,
`has_version`, `supersedes`, ...). Kinds are registered with an identity
mode (`content_hash` or `external_key`) and a `Versioned` flag. Per the
Autopaper design doc (§2), I reuse existing kinds where they fit
(`choir.texture_document`/`texture_revision`, `choir.web_capture`,
`choir.source_ref`, `choir.media_item`, `choir.artifact_blob`,
`choir.commitment_record`) and propose new kinds only where none fits. The
per-computer embedded Dolt holds private working state. The global corpus
Dolt (the Autopaper object graph, through corpusd) holds only what is
published.

---

## 1. Spreadsheet (`src/spreadsheet`)

### What it is and its stack
An Excel clone: vanilla JS with no framework, Vite, and vitest+jsdom.

**Size:** `src/core` is 68k lines in 268 files; `src/ui` and `src/views`
add 37.5k; tests are 190k lines (2,360 files, about 9,450 cases).

**Runtime dependencies:**
- `@noble/hashes` (MIT), `@xmldom/xmldom` (MIT) and `lucide` (ISC).
- Vendored and **locally modified** SheetJS Community Edition 0.20.3
  (`src/vendor/xlsx.mjs`, 29k lines, **Apache-2.0**; the app prepends its
  own imports to it).
- Apache POI data (preset geometry and pivot styles; Apache-2.0), .NET
  data (MIT), and Unicode data (Unicode License v3).

All of these are permissive. The Apache-2.0 items are not MIT, but they are
compatible. They need NOTICE retention if reused.

### Quality
**This is a real engine.** The formula pipeline is in
`src/core/formula/`:
- `tokenizer.js` (271 lines) and a recursive-descent `parser.js` (246) that
  produce a clean typed AST. Node types include `ref`, `tref` (structured
  table refs), `call`, `apply` (lambda calls), `array`, `union`, `range`
  and `intersection`.
- `evaluator.js` (883), a tree-walking interpreter. It handles array
  broadcasting, spill, implicit intersection (`@`), lazy IF/CHOOSE
  evaluation, LET/LAMBDA (`lambda.js`), and an array budget of 4M cells.
- `functions.js` (8.5k lines, about 554 entries in `FUNCTIONS`). It covers
  statistics, distributions, finance, bonds, engineering, complex numbers,
  dynamic arrays, GROUPBY/PIVOTBY, Google's QUERY (`query.js`), and regex
  with a ReDoS guard (`regex-safety.js`).

The formula core is DOM-free. Numerics are taken seriously: `numeric.js`
reproduces Excel's subnormal flushing and final-cancellation rounding.
Correctness is pinned by **oracle fixtures recorded from Microsoft Excel
16.113 for Mac** (`tests/fixtures/reference-*.json`, 83 files; for example
`reference-power-arithmetic.json` says "Microsoft Excel 16.113 for Mac,
2026-09-22"). These include Hebrew, Hijri, Persian and lunisolar calendar
tables.

**The architectural weakness is recalculation.** `workbook.js` (12k lines;
header at line 38) says "lazy formula evaluation with a cache that is
dropped whole on every change". There is no dependency graph:
- `_changed()` (line 11952) calls `_invalidate()`.
- `_compute()` (line 10862) re-evaluates on demand and memoizes per cell.
- Cycles are detected through an `_evaluating` stack and become
  `#CYCLE!`. Iterative calculation lives in `iteration.js`.

Dependents are computed by scanning (`dependents()`, line 5325, limit 5000).

This is fine for one user editing a browser grid. It is wrong for an agent
that needs "what changed downstream of B7?" and incremental recalculation
over large or remote sheets.

Undo is per-method closures with whole-workbook snapshots for structural
edits. There is no CRDT and no collaboration (the grep hits for "collab"
are UI strings only). Persistence is a whole `.xlsx` saved to Puter plus a
local draft. App-specific state rides in a very-hidden `_puter_meta` sheet
(`core/io.js`).

**File formats:**
- Very deep .xlsx read and write via SheetJS plus about 60 `xlsx-*.js`
  modules: styles, conditional formats, sparklines, pivots, slicers,
  charts, threaded comments, data validation and external links.
- ODS settings, filters and validation.
- CSV, TSV and HTML.

There is also a full Power Pivot/DAX data model (`data-model-*.js`,
`dax.js`). That is scope creep with low value for Choir.

**It already has an in-app agent.** `core/aiagent.js` and `core/aitools.js`
(1,290 lines) expose 22 tools:
- Read and compute: `get_workbook_info`, `read_range`, `calculate`
  (evaluates a formula without writing), `find`.
- Write: `write_cells`, `fill_formula`, `clear_range`, `format_cells`,
  insert/delete rows and columns, `sort_range`.
- Build: `create_chart`, `create_table`, `add_conditional_format`,
  `add_dropdown`.
- Sheet and view: `manage_sheet`, `freeze_panes`, `set_column_width`,
  `add_note`, `select_range`.

It has approval modes (`always`, `destructive`, `never`), a "cell contents
are data, not instructions" prompt-injection rule, transcript budgeting,
and the whole loop is testable with a scripted model. There is also an
`AI(prompt, range)` formula with async per-prompt caching
(`functions.js:7677`, through `ctx.asyncValue`). That is a working
precedent for "a cell whose value comes from an agent".

### Worth taking for Choir
1. **The formula engine** (`src/core/formula/*` plus its imports from
   `refs.js`, `cell.js`, `format.js`, `numfmt.js` and `numeric.js`). It
   parses, evaluates and covers about 550 functions with Excel-oracle
   tests, and it is DOM-free, so it can run in the desk's runtime
   (Node/Bun, or Go via goja; or as a reference to port). Lift the engine
   and the fixtures, not `workbook.js`.
2. **The oracle fixture corpus** (`tests/fixtures/reference-*.json`). It is
   language-neutral and would let a Go or TS engine of our own prove Excel
   parity. It may be the most durable asset in the whole group.
3. **The agent tool schema** (`aitools.js` `TOOLS`) as a starting verb set.
   Also take the approval-mode idea and the `calculate`-don't-do-arithmetic
   prompt rule.
4. **SheetJS CE** (Apache-2.0) for xlsx I/O, taken upstream rather than as
   the modified vendored copy.

Do not take `workbook.js`. It is a 12k-line god object mixing model, undo,
pivots and drawing.

### App-agent design
**State as objects** (embedded Dolt on the computer):

- **Workbook**: new kind `choir.sheet_workbook`, external key, versioned.
  Its metadata holds the name, sheet order, named ranges and calculation
  mode.
- **Sheet**: new kind `choir.sheet`, versioned, linked to the workbook by
  `contains`. Its body is the sparse cell map in a compact canonical JSON
  form: `{key: {v, f, s}}`, chunked into row blocks of about 1k rows for
  large sheets (`choir.sheet_block`). Do not use one object per cell:
  Dolt rows and edge counts would explode, and the commit diff of a block
  is already cell-granular when the body is canonical JSON.
- **Range**: new kind `choir.sheet_range`. This is the **linkable** unit: a
  named range or table with an A1 address and a stable id that survives
  row insertion. It carries the ref-adjust logic from `refs.js`
  (`adjustRefForInsert`). Other apps and desks link to ranges, not to
  cells.
- **Cross-app edges:**
  - `sheet_range --derived_from--> choir.web_capture`: a table filled from
    a research capture.
  - `sheet_range --cites--> choir.source_ref`.
  - `choir.texture_revision --references--> sheet_range`: a Texture
    article embeds a live table or chart, and the edge records the range
    version it quoted.
  - `chart --derived_from--> sheet_range`.
  - `sheet_range --derived_from--> choir.commitment_record`: a research
    query result delivered as a record.
- **Computed values** are not stored as truth. They are a derived cache
  per sheet version (they could live in a `choir.sheet_block` metadata
  `values_hash`). This is the xlsx "cached value" concept. An
  `externalResult`-style cache holds agent- or web-derived values
  (`AI()`, IMPORT*) keyed by prompt hash, so a recompute does not re-spend
  model calls.

**What versioning gives for free:**
- Every agent transaction is a Dolt commit, which gives history and
  multi-step undo across sessions. This replaces the closure undo.
- A Dolt branch is "what-if". Excel's Scenario Manager (`scenarios.js`)
  and Goal Seek become "branch the workbook, change inputs, compare
  ranges, merge or discard". This is the single best agent-native feature.
- Diff between versions gives "what did the agent change", rendered by
  Texture.

**Published versus private:**
- Private by default.
- Publishing a range or chart to the Autopaper object graph puts a frozen
  `choir.publication_version` that `contains` a snapshot
  (`choir.artifact_blob`: values plus formulas plus the provenance edges to
  captures) into the global store. Other computers adopt the range by id
  and version and can re-link it (a "published dataset").
- Live formulas referencing private ranges never leave the computer.

**Direct manipulation:** edit a cell, fill, select, sort, filter, format,
resize, insert or delete rows and columns, and charts. Each is one
transaction and one commit.

**Desk verbs** (exposed to other desks and external agents over the
ledger):
- `read_range(range, values|formulas)`
- `calculate(formula)`: evaluate without writing
- `set_cells(start, rows)` and `fill_formula(range, f)`
- `insert_rows` and `delete_rows`
- `define_range(name, ref)`
- `recompute(scope)`
- `explain(cell)`: precedents and dependents, plus the citations behind
  inputs
- `branch(name)`, `compare(branch, range)` and `merge(branch)`
- `link_range_to_query(range, research_query, schedule)`: research fills
  it, each refresh is a commit, and stale inputs are flagged
- `publish_range(range)`
- `import_file(xlsx|csv)` and `export(xlsx)`

**Desks it calls:**
- research: fill a range from sources, refresh linked ranges, verify a
  figure against evidence
- Texture: write the narrative or report around a range, render diffs
- engineering: build a custom function or connector in a capsule
- management: schedule refreshes, gate destructive edits

**Better as an agent because** the sheet stays connected to its evidence:
every input range knows which capture it came from and can be re-pulled,
and "what if" is a branch, not a copy-paste. External agents get
semantic verbs (`calculate`, `explain`) instead of screen scraping. The
existing `calculate` tool proves the pattern: the model computes through
the engine, never in its head.

**Gaps for the agent design:** no dependency graph, so
`explain`/`recompute` need one. Build a precedent/dependent index when
building the Choir version. The AST already makes this easy:
`caller-context.js`/`callerDependencies` walk refs.

### Rebuild priority: HIGH
It is the owner's lead example, and the hard part (formula semantics plus
Excel parity tests) exists under a permissive license.

**Effort the Choir way:** about 3-5 weeks for a v1. That covers the object
model with range ids and Dolt branches, the lifted formula engine with a
new dependency index, the desk verbs, and a Svelte grid (virtualized, about
2k lines; do not port `ui/grid.js`, which is 5.6k lines), with xlsx via
upstream SheetJS. Pivots, slicers and DAX are out of scope.

---

## 2. Documents (`src/documents`)

### What it is and its stack
A Word clone with an **original canvas-rendered layout engine** (no
contenteditable; a hidden `<textarea>` handles input and IME,
`engine/canvas-editor.js:53-80`).

**Size:** `src` is 59k lines, of which `src/engine` is 33k lines in 310
files. Tests are 103k lines (about 5,200 cases plus 750 Playwright
browser tests). `tools/` holds 1,610 Python fixture generators and probes,
and 792 fixture directories contain "native" EMF page renders from Word.

**Dependencies:**
- MIT: `fflate`, `bidi-js`, `opentype.js` (also vendored), `rtf.js`,
  `utif`, `nspell` (with the SCOWL en_US dictionary, permissive), and the
  mdast/micromark/hast family.
- `@zip.js/zip.js` (BSD-3).
- Fonts under OFL or Apache.

`tools/check-licenses.mjs` enforces an allowlist, and `AGENTS.md` bans
AGPL code.

### Quality and data model
The data model (`engine/model.js`, class `TextDocument`) is:
- A **flat array of paragraphs**, each `{id, runs:[{text, ...RUN_FORMAT,
  properties, overrides}], format, cellId?, blockControls?, bookmarks?,
  fieldMarkers?}`.
- Tables kept separately (`doc.tables`), with paragraphs pointing into
  cells via `cellId` and nested tables via `parentCellId`.
- Styles and numbering kept as OOXML-shaped dictionaries.
- Positions as **UTF-16 offsets into the paragraphs joined by `\n`**
  (comment at line 53).

**There is no CRDT and no operational model.** Mutations are methods on
`TextDocument` (83 of them: `setParagraphFormat`, `mergeCells`,
`insertTable`, `setHyperlink`, `addComment`, ...). Undo is
`checkpoint()`, which pushes a `structuredClone` of the whole document onto
a 100-deep history (lines 975-990, 2042). Track changes exists
(`engine/revisions.js`) as an OOXML feature, not as a collaboration
substrate.

The engine is a genuine typesetter: line builder, justification, bidi,
OpenType kerning, pagination, floating images, footnotes, columns and
tables across pages. It chases Word pixel fidelity against EMF renders.
That is impressive and enormous, and far beyond what Choir needs.
Coverage gaps are honestly documented in `docs/*-coverage.md`, and
unsupported files open read-only.

**Other formats:** an HWP/HWPX (Korean Hanword) importer (`lib/hwp/`)
exists but is not wired to the current editor (`docs/hwp.md`). Markdown,
HTML, RTF and TIFF importers live in `lib/`. The AI layer (`lib/ai.js`) is
just 12 rewrite prompts over the selection.

### Worth taking for Choir
Mostly **docx I/O ideas, not the editor**. Choir already has Texture,
which is the sole writer of canonical document revisions. A second
document model would violate the desk ontology.

The useful pieces are:
- `engine/docx.js` (2k lines) plus `docx-*.js` as a reference for a
  **docx-to-Texture importer and Texture-to-docx exporter**. It is in the
  browser and DOM-dependent, so treat it as a reference to port, not a
  library.
- `lib/import-markdown-*.js` and the mdast pipeline for Markdown
  normalization.
- The HWP importer, only if Korean users matter.
- The OOXML coverage docs as a checklist.

Skip the canvas typesetter.

### App-agent design
The documents app should **be Texture with docx import and export**, not a
new app:
- **State:** the existing `choir.texture_document`, plus a
  `choir.texture_revision` (content hash, versioned) per commit and
  `document_revision`/`revision_parent` edges. These already exist.
- **The imported .docx** is a `choir.artifact_blob` (content hash), linked
  `texture_revision --derived_from--> artifact_blob`, so the original
  survives untouched. This matches this app's own "preserve untouched
  package parts" rule.
- **Citations:** `cites`/`source_ref` to `choir.web_capture`.
- **Embedded tables:** `references` to `choir.sheet_range` with the
  version pinned.
- **Comments:** comments and suggestions map to Texture decisions or
  commitment records rather than OOXML comment parts. OOXML comments are
  imported as such on the way in and exported on the way out.

**Versioning gives:** history, diff ("show me what the agent changed"),
and branches for alternative drafts. These replace the 100-snapshot undo.

**Published:** the existing `choir.publication` /
`publication_version` path. **Private:** drafts and the imported blobs.

**Where it does not map:** Word's layout-level state (section properties,
exact page geometry, fields, content controls) has no Texture equivalent.
Keep it as opaque metadata on the blob for export round-trip, not as
first-class objects.

**Direct manipulation:** Texture's existing editor.

**Desk verbs** (Texture already owns most):
- `import_docx(blob)`: produces a revision plus a fidelity report
- `export_docx(doc, version)`
- `apply_suggestion`, `rewrite(selection, instruction)`, `cite(range,
  capture)`

**Desks:** research for citations, the spreadsheet for embedded figures,
and management for review gates.

### Rebuild priority: LOW as an app, MEDIUM as a docx bridge for Texture
About 1-2 weeks for a docx importer and exporter covering paragraphs,
runs, headings, lists, tables, images, links and comments. Use this repo
only as a reference.

---

## 3. Presentation (`src/presentation`)

### What it is and its stack
A PowerPoint clone.

- **App code:** 29k lines of JS. It is dominated by `src/main.js` at 14.7k
  lines, a monolith.
- **Engine:** 78k lines of TypeScript in `lib/pptx`
  (reader/editor/writer: OPC, DrawingML, PresentationML, ChartML, SmartArt
  diagrams, fonts) and `lib/pptx-preview` (SVG renderer and text layout).
  It is absorbed from **office-kit/pptx 0.12.0 and pptx-preview 0.9.1, MIT,
  © 2026 Yuichiro Yamashita** (`lib/README.md`, LICENSE files retained),
  with no upstream sync after that.
- **Dependencies:** `fflate` (MIT, carrying a **local security patch**
  that bounds decompressed size, `patches/fflate+0.8.3.patch`),
  `unicode-segmenter` (MIT) and `lucide` (ISC). Fonts are OFL.
- **Tests:** about 2,670 cases (unit plus e2e). Disclosure is good
  (`DEPENDENCY_REVIEW.md`).

### Quality
The engine is real and well layered. It is a functional API over an
in-memory OPC package (`lib/pptx/src/api/fn/*.ts`, 99 modules: slides,
shapes, text, tables, charts, diagrams, themes, layouts, placeholder
inheritance). The deck state *is* the package, so unknown parts are
preserved.

The app layer is weak: a 14.7k-line `main.js`, with undo kept as
serialized package revisions.

`src/assistant-tools.js` has 23 agent tools: `get_presentation`,
`read_slide`, `find_text`, `add_slide`, `duplicate_slide`, `move_slide`,
`delete_slides`, `set_slide_properties`, `set_notes`, `add_text`,
`add_shape`, `add_line`, `add_table`, `set_text`, `set_table_cells`,
`format_text`, `style_objects`, `set_bounds`, `reorder_objects`,
`delete_objects`, `replace_text`, `apply_theme`. Each runs as one undo
step via `edit(fn)`.

### Worth taking
**`lib/pptx` plus `lib/pptx-preview`** as a TS library: PPTX read, edit
and write plus a PPTX-to-SVG render, under MIT. It is better to take it
from upstream office-kit if that is maintained, and to evaluate this fork's
fixes. Choir's existing `SlidesApp.svelte` only *plays* PPTX via JSZip, and
this would let an agent author and edit decks. Also take the assistant
tool list as the verb set.

### App-agent design
- **Deck:** new kind `choir.deck`, external key, versioned. Its body is a
  canonical slide-level JSON model (slides, shapes, text, theme
  references), not raw OPC XML. Each slide is `choir.deck_slide`, linked
  to the deck by `contains`, so that slides are linkable, reusable and
  diffable.
- **The original .pptx** is a `choir.artifact_blob` (`derived_from`),
  kept for round-trip of unknown parts.
- **Cross-app edges:**
  - `deck_slide --references--> choir.sheet_range` or a chart (a live
    figure)
  - `deck_slide --derived_from--> choir.texture_revision` ("make a deck
    from this article")
  - `--cites--> choir.source_ref`
  - images and video as `choir.media_item`
- **Mapping:** this maps moderately well. Slides and shapes are natural
  objects. DrawingML styling and inheritance (master, layout, slide) do
  not map cleanly; keep them as opaque package data resolved by the
  library.
- **Versioning:** branch a deck for variants ("a 5-minute version"), and
  diff slides.
- **Publishing:** to Autopaper as a rendered `artifact_manifest` (SVG or
  PDF per slide) plus the source deck version. Speaker notes stay private
  unless chosen.
- **Desk verbs:** the 23 above, plus `deck_from(texture_doc |
  sheet_range)`, `refresh_linked_figures`, `render(slide, svg|png)`,
  `export(pptx|pdf)`.
- **Desks:** Texture for the outline and narrative, the spreadsheet for
  charts, research for images and sources.
- **Better as an agent because** a deck generated from a Texture article
  plus linked sheet ranges stays live: update the data, and the slides
  refresh.

### Rebuild priority: MEDIUM
The library is valuable and permissive, but decks come after
sheets and Texture. About 2-3 weeks to wrap `lib/pptx` behind desk verbs
and a Svelte editor, reusing the SVG renderer in `SlidesApp`.

---

## 4. PDF (`src/pdf`)

### What it is and its stack
An Acrobat clone.

- **Size:** `src` is 60k lines, with `src/editor.js` at 11.6k (a
  monolith). Tests are 66k lines (about 1,560 cases).
- **Rendering:** `pdfjs-dist` 6.3.289 (Apache-2.0).
- **Editing:** `pdf-lib` 1.17.1 (MIT; last release 2021, which the app's
  own docs flag).
- **Fonts:** `@pdf-lib/fontkit` (MIT declared, with no LICENSE file in the
  package; flagged in `docs/ENGINE_AND_LICENSES.md`).
- **OCR:** `tesseract.js`/`-core` 7 (Apache-2.0), on-device.
- **Other:** `@xmldom/xmldom` (MIT).
- **License flag:** PDF.js `standard_fonts` include **Liberation Sans
  1.07.4 under GPL-2.0 with a font exception**. The app ships the font
  source tarball (`vendor/liberation-fonts/`) to comply. It is not AGPL,
  but it is a GPL redistribution obligation if copied; the Foxit fonts
  alone avoid it.

### Quality
The features are broad and real:
- redaction that removes content (`core/redact.js`, `image-redact.js`)
- in-place text editing matched to content-stream commands
- forms, including field detection
- XFDF comments
- a compare view
- PDF/A (`core/pdfa.js`)
- autotagging and accessibility checks
- AES-256 protection
- incremental-update digital signatures

**Security alarm:** the app hand-rolls its PKI and legacy-crypto code:
- `core/asn1.js`, `x509.js`, `cms.js` and `pkcs12.js` (about 1.8k lines),
  which parse untrusted DER and validate certificate chains.
- AES-CBC, RC4, MD5, DES and RC2 (`security.js`, `legacy-ciphers.js`,
  `sha.js`).

RSA and ECDSA verification and signing do go through WebCrypto
(`pki-crypto.js`). AES-CBC in pure JS for PDF encryption and
non-constant-time code are acceptable for PDF compatibility. **Chain and
signature-status validation written by an agent should not be trusted as a
security verdict** without independent review.

### Worth taking
- **Ideas and some pure modules:** XFDF import/export
  (`annotation-xfdf.js`), the redaction algorithm (true content removal,
  not overlays), `content-filter.js`/`stream-filters.js`, and the
  PDF-to-docx/xlsx/pptx layout export (`export-layout.js`).
- **The engine choice is sound and reusable:** pdf.js plus pdf-lib, with
  MuPDF avoided because it is AGPL. Choir already uses pdfjs-dist 5.x in
  `PdfApp.svelte`.
- On-device OCR via tesseract.js is a good pattern for a capture pipeline,
  though server-side OCR may fit Choir better.

### App-agent design
- **The PDF** is a `choir.artifact_blob` or a `choir.web_capture` (when
  research fetched it), content-hash identity. It is immutable.
- **Annotations** (highlights, notes, redaction marks, form values): a new
  kind `choir.pdf_annotation` (or reuse `choir.source_ref` with
  page/quad selectors for highlights). Each links `--references-->` the
  PDF by hash, with page, quads and text quote. This is the W3C Web
  Annotation shape, so the same highlight is a citation that Texture can
  cite (`cites`).
- **An edited PDF** is a new blob linked `derived_from` the original, with
  the edit list as metadata. Signatures and redactions are explicit
  derivations, which gives an audit trail for free.
- **Mapping:** this maps cleanly because PDFs are immutable, which suits
  content-hash objects. In-place text editing does not map well, and is
  rarely needed.
- **Published:** captures of public PDFs are already in the Autopaper
  graph. Annotations and highlights are private unless cited in a
  published Texture, in which case the `source_ref` travels with it.
- **Desk verbs:**
  - `extract_text(pdf, pages)`, `ocr(pdf)`, `find(pdf, q)`
  - `highlight(pdf, quote)` → a `source_ref`
  - `fill_form(pdf, values)`, `redact(pdf, patterns)` → a derived PDF
  - `merge` and `split`
  - `export(pdf, docx|xlsx)`
  - `table_to_sheet(pdf, page, region)` → a `choir.sheet_range`
- **Desks:** research (fetch, OCR, extract claims), the spreadsheet
  (tables), Texture (citations).
- **Better as an agent because** "pull the table on page 12 into a sheet
  and cite it" and "redact all SSNs in these 40 files" are agent verbs.
  The direct-manipulation UI is secondary.

### Rebuild priority: MEDIUM-LOW
Choir has a PDF reader, so add annotation-as-citation and
`table_to_sheet` first (about 1-2 weeks). Full Acrobat editing is low
priority.

---

## 5. Notes (`src/notes`)

### What it is and its stack
A OneNote clone: notebooks, sections, pages, free-positioned note
containers, ink, tags and to-dos.

- **Size:** 16k lines in total, the smallest and most conventional app in
  the group. There are 131 tests.
- **Dependencies:** `pdfjs-dist` (Apache-2.0), `perfect-freehand` (MIT),
  `temml` (MIT; LaTeX to MathML) and `lucide` (ISC).
- **Editing:** contenteditable plus `document.execCommand`
  (`editor/format.js:46`), a deprecated API. Content is sanitized HTML via
  an allowlist sanitizer (`lib/sanitize.js`, which looks reasonable).
- **Protected sections:** PBKDF2 to AES-GCM via WebCrypto
  (`data/crypto.js`). This is done correctly.

### Data model
`data/store.js` lays out plain JSON files:
- `library.json`
- `notebooks/<id>.json`: the manifest of sections and page metadata
- `pages/<id>.json`: `{id, title, created, modified, outlines:[{x, y,
  width, html}], ink:[strokes]}`
- `versions/<id>.json`: up to 20 per page
- `assets/<id>`
- `trash.json`, with 60-day retention

Storage goes through a tiny backend interface (`data/backend.js`:
readJSON, writeJSON, readBlob, writeBlob, list).

### Worth taking
Only ideas:
- the page model of positioned outlines plus ink strokes
- tags and to-dos extracted from content (`data/tags.js`), with a tag
  summary
- shape recognition on ink
- perfect-freehand for ink rendering (a library)

The code quality is ordinary.

### App-agent design
Notes should be **Texture documents of a "notebook page" kind**, not a
separate store:
- **A page** is a `choir.texture_document`. Its revisions carry the
  outlines (Markdown or Texture blocks rather than raw HTML) and the ink
  as a structured block. Positioned containers are layout metadata.
- **Notebooks and sections** are a container object (reuse `contains`
  edges; a light `choir.notebook` kind if needed).
- **Tags and to-dos:** the to-dos should become **work items or
  commitment records**. A note-level "to-do" that management can see and
  schedule is the agent-native win.
- **Attachments:** `choir.media_item` or `artifact_blob`.
- **Clips:** `choir.web_capture` (`captured_from`).
- **Versioning:** Dolt history replaces the 20-version files.
  **Published:** almost nothing; notes are private by default, and a page
  can be promoted to a published Texture.
- **Mapping:** this maps cleanly, except for freeform x/y layout and ink,
  which Texture does not model.
- **Desk verbs:**
  - `append_note(page, text)` and `clip(url)` (via research)
  - `extract_todos(page)` → work items
  - `summarize(section)`
  - `find(q)`
  - `promote_to_texture(page)`
- **Desks:** management (to-dos), research (clipping), Texture.

### Rebuild priority: LOW
It is mostly subsumed by Texture plus captures. A freeform ink canvas is a
later nicety.

---

## Group summary

**Top picks:**
1. **Spreadsheet formula engine** (`spreadsheet/src/core/formula/*`). It
   is DOM-free, covers about 550 functions with LET/LAMBDA/dynamic arrays,
   and is MIT. Pair it with its **Excel-oracle fixture corpus**
   (`tests/fixtures/reference-*.json`). This is the clear high-priority
   rebuild seed. Add a dependency graph, which it lacks: its cache "is
   dropped whole on every change".
2. **`presentation/lib/pptx` plus `lib/pptx-preview`.** These are a TS
   PPTX read/edit/write library and an SVG renderer (MIT, office-kit
   origin).
3. **The agent tool schemas** already written for spreadsheet
   (`core/aitools.js`, 22 tools, approval modes, a `calculate` tool,
   injection rule) and presentation (`src/assistant-tools.js`, 23 tools).
   These are ready-made starting verb sets for app desks.

**Cross-app reusable pieces:**
- SheetJS CE (Apache-2.0) for xlsx
- the docx reader as a reference for a Texture docx bridge
- pdf.js plus pdf-lib as the PDF stack
- XFDF and the redaction approach
- fflate with the decompression-bound patch, as zip-bomb hygiene for every
  OOXML importer
- the shared idea that **unknown package parts are preserved in an opaque
  original blob**. In Choir terms: the imported file is an `artifact_blob`
  and the editable object is `derived_from` it.

**Shared object-graph pattern across the office group:**
- Imported file → content-hash blob.
- Editable state → versioned object.
- Linkable sub-units → their own objects: `sheet_range`, `deck_slide`,
  `pdf_annotation`/`source_ref`, Texture blocks.
- Cross-app `references`/`cites`/`derived_from` edges, pinned to versions.
- Dolt branches for variants.
- Publication via the existing `choir.publication_version` path.

Documents and Notes should collapse into Texture rather than become new
apps. New kinds needed: `choir.sheet_workbook`, `choir.sheet`,
`choir.sheet_block`, `choir.sheet_range`, `choir.deck`, `choir.deck_slide`,
and possibly `choir.pdf_annotation`.

**Alarms:**
- **Quality:** this is agent-generated at enormous scale. Expect
  monoliths (`workbook.js` 12k lines, `presentation/src/main.js` 14.7k,
  `pdf/src/editor.js` 11.6k) and heavy fidelity-chasing sprawl. Lift
  engines, not apps.
- **Security:** PDF's hand-rolled ASN.1, X.509, CMS and PKCS#12 parsing
  and its chain validation. Do not trust its signature verdicts without an
  audit.
- **Licenses:** no AGPL anywhere in the group.
  - Apache-2.0: SheetJS, Apache POI data, pdf.js, tesseract.js.
  - GPL-2.0 with font exception: Liberation Sans 1.07.4 in the PDF build.
  - `@pdf-lib/fontkit` is missing its license file.
