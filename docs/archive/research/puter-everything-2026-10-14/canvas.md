# Canvas group review: whiteboard, vector-studio, photo-studio

Reviewer group: canvas. Repo: HeyPuter/everything shallow clone (HEAD `a7f61cdc`
"Whiteboard: integrations and interoperability review (#13)"). Nothing was
executed; all judgements come from reading source. Paths below are relative to
`repo/src/`.

All three apps share one house style, and it matters for judging them: no UI
framework (vanilla JS/TS, Canvas 2D or WebGPU), Vite, Puter.js as the only
backend (storage in `puter.fs`/`puter.kv`, AI from `puter.ai.chat`/`txt2img`,
collaboration through a Puter Worker that "sharing" deploys to the owner's
account), very long README feature lists, and long explanatory comments in a
consistent voice ("Says whether...", "as in Miro"). They were clearly written by
long-running coding agents in an optimize/verify loop. They are still not
shallow slop: each has a real domain model, many tests, and some real codecs.
The weakness is breadth over depth. Every app goes after full Miro, Illustrator
or Photoshop feature parity, so the surface is huge and much of it is checked
only by unit tests over pure functions.

---

## 1. whiteboard (Miro clone)

### What it is, and why it is 574 MB

- A Miro-style infinite-canvas whiteboard: sticky notes, shapes (70 kinds:
  flowchart, BPMN, UML, wireframe), connectors, frames, tables, mind maps,
  comments, tasks/Gantt/Kanban, facilitation (dot voting, polls, timer, agenda
  runner, private mode), templates, and live co-editing.
- **It is not an Excalidraw or tldraw fork.** It is original vanilla JS on HTML
  Canvas 2D (`whiteboard/src/render.js`, `scene.js`, `model.js`).
- **Size:** `whiteboard/tmp/` is **561 of the 574 MB**. It holds 916 entries
  (4,373 files) of committed agent scratch output from the optimization loop:
  `*-before-native/`, `*-candidate/`, `*-final-native/` copies of full
  production `dist/` builds (each 6–22 MB with fonts and the pdf.js
  wasm/JBIG2/OpenJPEG/qcms decoders), A/B entry bundles
  (`ai-summary-omission-candidate.js`, `...-before.mjs`), candidate test files
  and WCAG probe HTML (`wcag-*-native.html`). There are no `node_modules`.
  The repo's root `.gitignore` covers `dist/` and `node_modules/` but not
  `tmp/`. Real source is `src/` at 4.3 MB (199 files, ~103k lines), `tests/` is
  5.5 MB (~156k lines in 148 test files), `worker/` is 464 KB. The whole clone's
  git pack is ~397 MB, so most of the repo's weight is this one directory.
- **Stack/deps** (`whiteboard/package.json`, `THIRD_PARTY_NOTICES.md`): yjs
  (MIT), pdf-lib (MIT), pdfjs-dist (Apache-2.0), lucide (ISC), Fontsource
  fonts (OFL-1.1). Dev only: axe-core (**MPL-2.0**, tests only, fine),
  fake-indexeddb (Apache-2.0), miniflare, jsdom, @napi-rs/canvas. **No
  GPL/AGPL/non-permissive dependency.** Plausible analytics script; Puter.js
  loaded from Puter's CDN.

### Quality

- **Real engine, poor structure.** The model is a real one: `model.js`
  validates every object type, `collaboration/document.js` (1,642 lines) maps
  the board onto Yjs with rebase of local undo over remote edits,
  `collaboration/text-binding.js` binds rich text to Y.Text, and
  `connectors.js`, `geometry.js` (rotation) and `mind-map.js` (auto layout) are
  real code. But `src/main.js` is a **13,970-line god file** and
  `features.js` is 3,866 lines, so all UI wiring goes through one module.
- **Signs of generation:** every comment in the same voice, constant "as in
  Miro" parity framing, a 93 KB README, docs on WCAG 2.2 audit and fix
  commits, and the committed `tmp/` A/B builds. That is an agent loop
  optimizing measured regressions (accessibility, unicode, i18n en/fr) with no
  human pruning. The code itself is careful (surrogate-safe truncation,
  locale-independent ID ordering).
- **Tests:** 148 `node --test` files over jsdom plus a native canvas. They are
  broad but run at module and app level, not in a browser.
- **Completeness:** the core canvas, connectors, tables, mind maps and
  collaboration look complete. AI (`src/ai/ai-ui.js`, 944 lines) is thin: it
  sends board text to `puter.ai.chat` to make summaries or sticky-note grids.
  Enterprise extras (org admin, retention, Jira, Google Workspace Apps Script
  sync, Miro REST migration) are wide and mostly glue.

### Worth taking

- **Data model ideas:** the flat item list with typed objects (`model.js`
  `item()`, `board()`), connectors that attach to object outlines
  (`connectors.js`, `shape-specs.js` safe label areas), and mind-map trees
  stored as items with layout derived from them (`mind-map.js`).
- **The Miro importer** (`migration/miro-convert.js`, 2,001 lines): pure
  functions from the Miro REST v2 JSON to the board model, with injected
  downloaders. This is useful if Choir ever imports Miro boards. Take it as a
  reference, not as code.
- **Office ingestion** (`content/office.js`): DOCX/PPTX text extraction and
  XLSX opening onto the board, with a hand-rolled `unzip.js`/`zip.js`. Fine
  as a reference; Choir likely has better options.
- **Exporters:** `export-svg.js`, `export-pptx.js`, `export-pdf-vector.js`
  (board regions to slides or PDF pages). These are moderately useful as
  references.
- The **facilitation model** (`facilitation/model.js`: votes, polls, agenda
  steps, private mode) is a good idea list for multi-participant work, though
  in Choir the participants are mostly desks.
- Do not take the Yjs collaboration layer. Choir's substrate is the ledger.

### App-agent design (Choir "Board" desk)

- **State as objects** (in `internal/objectgraph`, embedded Dolt on the
  computer):
  - **`choir.board`** (new kind, `IdentityExternalKey`, `Versioned: true`):
    title, viewport defaults, and template origin. Each act moves its
    version. No existing kind fits; `choir.texture_document` is linear prose,
    not a 2D scene. This is the one new kind the app needs.
  - **Board items** are where whiteboard's model maps best and worst. Items
    are small JSON records (`model.js` `item()`: id, type, x, y, w, h, text,
    color, links), and a board holds at most 5,000
    (`MAX_OBJECTS`). Store each item as a **row of one versioned
    `choir.board` body**, or as one `choir.board_item` object per item. The
    per-item choice gives fine-grained edges but 5,000 objects per board;
    my recommendation is per-item objects only for items that **link out**
    (a sticky that cites a capture, an evidence card), with the rest kept in
    the board body.
  - **Pen strokes** (up to 20,000 points each, `PEN_POINTS`) and images go
    in **content blobs** (`choir.artifact_blob`), referenced by hash and
    never stored as Dolt rows. This follows the autopaper design's rule
    against repeating the "104 G" problem.
  - **Comments, votes and polls** stay inside the board (private
    facilitation state). Timers, cursors, presence and selection are not
    objects at all; they are ephemeral.
- **Links to other apps' objects** (existing edge kinds):
  - An evidence card `references` a `choir.web_capture` or a
    `choir.source_ref`.
  - A frame `transcludes` a pinned `choir.texture_revision` (the paper's
    pinned-revision transclusion rule applies).
  - A task note `references` a `choir.work_item`. Today's Kanban/Jira sync
    becomes this edge, so management's work items *are* the cards.
  - A diagram `derived_from` a `choir.trajectory` or run, for a ledger
    rendered as a board.
  - An image item `has_media` a `choir.media_item`.
- **What versioning gives for free:**
  - Undo/redo across sessions and across desks. Whiteboard keeps undo local
    only and hand-rebases it over Yjs; that machinery
    (`collaboration/document.js`) goes away.
  - A per-board history, replacing the IndexedDB `revisions` store in
    `storage.js`.
  - **Dolt branches as "try a variant"**: an agent reorganizes the board on
    a branch and the owner merges or discards.
  - Commit granularity must be **one semantic act per commit** (on pointer
    up, or one agent verb), never per drag frame.
- **Published vs private:**
  - Boards are **private** by default: working thinking and facilitation
    data (who voted for what is personal data).
  - Publishing to the Autopaper object graph happens through corpusd as a
    **frozen export**: a `choir.artifact_manifest` with a rendered
    SVG/PDF blob and `was_derived_from` the board version. For example, a
    story's explainer diagram, transcluded into a published Texture.
  - The live board itself is never published. A published diagram keeps its
    `cites` edges to the shared captures it used, so readers on other
    computers can follow the sources. Private items never leave.
- **Direct manipulation:** place, drag or resize items, draw connectors,
  type text, group, frame, lay out a mind map, comment.
- **Verbs for other desks and external agents:** `add_notes(texts, near?)`,
  `cluster(selection, by=theme)`, `connect(a, b, kind)`,
  `make_mindmap(from=outline|doc)`, `make_diagram(spec: flowchart/BPMN/
  architecture)`, `frame(items, title)`, `summarize(frame|board)`,
  `export(frame -> slides|pdf|svg)`, `link(item -> doc|evidence record)`,
  `read_board(query)`.
- **Desks it calls:** research (expand a note into sourced evidence and pin
  the evidence card), Texture (turn a frame into a canonical document and
  back), engineering (an architecture diagram becomes a capsule task),
  management (a Kanban column becomes a work item).
- **Why it is better as an agent:** a board is where the owner thinks
  spatially. An agent can cluster, deduplicate and connect what the desks
  produced, and it can render the commitment ledger itself (missions, work
  items, evidence) as a live, editable map. That gives Texture a spatial
  view, where a plain whiteboard holds only dead sticky notes.

### Rebuild priority: **medium**

A spatial view of the ledger and desks is attractive, but Choir already has
Texture as its document surface, and a board is not on the Autopaper path.
Rough effort the Choir way: a minimal board (notes, shapes, connectors,
frames, a ledger-backed model, and a small verb set) is about 2–3 weeks. Do
not reproduce the Miro feature list.

---

## 2. vector-studio (Illustrator clone)

### What it is

- A vector illustration editor: pen/curvature/pencil tools, live shapes,
  Pathfinder/Shape Builder, live effects, symbols, blends, envelopes, 3D,
  advanced type (threaded, area and path text, OpenType, CJK kinsoku,
  hyphenation in 12 languages, spelling), artboards, and wide import/export.
- **Stack:** plain ES modules, Canvas 2D renderer (`render/draw.js` with
  cached Path2D and culling), and a **Rust → wasm geometry engine**
  (`wasm/geom/src/{ops,model,recon}.rs`, ~1,560 lines) built on **i_overlay
  9.0.1 (MIT)** for polygon booleans and **kurbo 0.13.1 (MIT/Apache-2.0)** for
  Bézier math, called through a raw f64-buffer ABI (`wasm/geom/src/lib.rs`).
  The prebuilt `src/wasm/geom.wasm` (428 KB) is committed. JS deps:
  opentype.js 2.0.0 (MIT), lucide, Fontsource fonts (OFL). Size: 16 MB total,
  `src/` 8.8 MB, ~158k JS lines.
- **Vendored data:** hyphenation patterns (`src/model/hyphenation/*.tex`)
  under MIT, BSD-3, FSFAP and **LPPL-1.3c** (one file; permissive but with
  unusual modification terms, kept byte-for-byte with a hash check). Spelling
  dictionaries: SCOWL (MIT-like) and **French Grammalecte under MPL-2.0**
  (file-level copyleft, unmodified). `tools/check-licenses.mjs` fails the build
  on unreviewed licenses and checks the hashes of vendored data, which is good
  hygiene. **No GPL/AGPL.**

### Quality

- **The strongest engineering in this group.** The document model
  (`src/model/document.js`) is a clean **flat node store with immutable nodes
  and transactions**: an undo record is the before and after nodes, and
  `applyRemote`/`rebaseRecord` rebase local undo over remote changes. Each
  feature is a module that registers tools, panels, commands and hooks
  (`src/features/*`), so there is no god file (`editor.js` is 454 lines,
  `app.js` 759).
- **Hand-written format engines, a large and risky asset:**
  - a PDF reader/interpreter (`features/interop/pdf/`: lexer, filters,
    crypto, CFF/Type1/TrueType fonts, CMaps, shading meshes, `interpret.js`
    at 1,653 lines);
  - a PostScript/EPS interpreter (`interop/ps/interpreter.js`, 2,030 lines);
  - JBIG2 and JPEG 2000 decoders written from the ITU specs
    (`interop/codec/jbig2.js`, `jpx.js`, `mq.js`, `ccitt.js`);
  - TIFF, PSD and BMP readers and zlib;
  - DXF import/export (`interop/dxf/`).

  The headers say they were implemented from the ITU-T standards. I found no
  pdf.js-derived identifiers or attribution, but provenance cannot be proven
  from a read.
- **Tests:** 205 Vitest files, ~41k lines, driven against the model directly
  (`tests/artboards.test.js` builds a `VDocument` and checks ops). This is
  real testing of the engine layer.
- **Generation signs:** the same voice, and an implausible breadth for the
  time (3D, perspective grid, gradient mesh, 9-slice, 12-language
  hyphenation). Expect long-tail bugs in exotic features. The core (paths,
  booleans, transforms, type layout) looks solid.

### Worth taking

1. **The `VDocument` pattern** (flat immutable node map, transactions,
   before/after undo records, remote rebase). It maps almost one-to-one onto
   a ledger: one transaction is one semantic act holding node diffs. This is
   the best canvas data model in the group and is worth copying as a design.
2. **The `geom` wasm crate** (i_overlay + kurbo; booleans, offset, simplify,
   path reconstruction). It is small, permissively licensed and
   self-contained. A good seed for any Choir vector or diagram work, and it
   can run server-side in Go via wazero or natively in Rust.
3. **The scripting API** (`features/scripts/runner.js`): a sandboxed opaque
   iframe running a `vs` API (`objects`, `get`, `update`, `create`, `remove`,
   `select`, `command(id)`, `exportSVG`) by postMessage. It checks
   `e.source`, uses a timeout, and makes one run one undo step. This is
   already the shape of an app-agent verb surface.
4. **Format code as a reference:** the SVG importer (`io/svg-import.js`,
   1,834 lines), the DXF importer and exporter, and the `.vstudio` format with
   versioned migrations (`features/files/format.js`). The PDF/PS
   interpreters are impressive but carry a large attack surface; use them
   only behind a sandbox.
5. **Liang hyphenation and CJK line breaking** (`model/liang.js`,
   `model/cjk.js`, `model/text-layout.js`) could serve Texture/Autopaper
   typesetting.

### App-agent design (Choir "Vector" desk)

- **State as objects:** this is the **cleanest mapping of the three**.
  `VDocument` is already a flat map of immutable nodes keyed by id. The
  root node holds the document settings, and a transaction records the
  before and after of each node it touched (`src/model/document.js`). That
  is structurally an object-graph batch.
  - **`choir.figure`** (new kind, `IdentityExternalKey`, `Versioned: true`;
    covers both illustration and chart): the root (artboards, units,
    swatches) plus the node map as its body. Optionally there can be one
    object per top-level layer or symbol so they can be shared.
  - **Transaction → one `PutBatchConditional`** whose
    `ExpectedVersionID` / `ExpectedContentHash` condition takes the place of
    vector-studio's hand-written `applyRemote`/`rebaseRecord`. A stale writer
    gets `ErrConflict` and rebases.
  - **Images, fonts and embedded PDFs** go in content blobs
    (`choir.artifact_blob`); today they sit in `.vstudio` as data URLs, which
    should not move into Dolt rows.
  - **Symbols, swatch libraries and graphic styles** are separate
    content-hash objects (`contains` edges), so one brand palette or logo
    symbol is shared across figures, boards and Texture documents. This
    replaces vector-studio's per-browser "Libraries".
  - **Where it does not map cleanly:** live effects and blends are derived
    (`live.js` regenerates artwork from inputs). Store only the inputs; the
    rendered output is a cache, never an object.
- **Links to other apps' objects:**
  - **Data-bound graphs**: a graph node `cites` a dataset or range object (a
    spreadsheet range from the sheets group) or a `choir.retrieval_manifest`.
    When the cited object gets a new version, the figure desk re-renders on
    a branch and proposes the update.
  - Traced or placed images `derived_from` a `choir.media_item` or a
    photo-desk object.
  - Figures are `transcludes`-ed into `choir.texture_revision` pinned to a
    figure version, with the paper's newer-revision indicator.
- **What versioning gives for free:**
  - Unlimited undo across sessions. `HISTORY_LIMIT = 200` goes away.
  - Version history, replacing the app's "version history" feature.
  - **Branches for variants**, which is the core Illustrator workflow done
    properly: "try 3 color schemes" becomes three branches, each one
    diffable node by node.
  - Merge is per node, as vector-studio already assumes.
- **Published vs private:**
  - Working figures are private.
  - A figure placed in a published Texture is published to the Autopaper
    object graph as a frozen `choir.artifact_manifest` (SVG plus PNG
    fallback blobs) with `was_derived_from` the figure version and `cites`
    edges to the shared captures or datasets behind a chart. That is enough
    for another computer to **adopt** the figure and re-bind it to its own
    data.
  - Shared symbol and palette libraries could be published on their own
    (a house style for the paper).
- **Direct manipulation:** pen and anchor editing, transforms, Pathfinder,
  type on canvas, artboards, swatches.
- **Verbs:** `create(node)`, `update(id, patch)`, `boolean(op, ids)`,
  `align/distribute`, `apply_style(ids, style)`,
  `place_text(text, frame, style)`, `trace(image)`,
  `make_graph(data_ref, kind)`, `make_artboard(size)`,
  `export(artboard, svg|pdf|png)`, `import(svg|pdf|dxf)`, `recolor(palette)`.
- **Desks it calls:** research (a data sheet for a chart, or brand/source
  images), Texture (place a figure into a document and keep it linked, so the
  figure updates when the data does), engineering (icon/asset pipeline into a
  repo through a capsule).
- **Why it is better as an agent:** data-bound figures (graphs from a research
  query that re-render when the evidence changes), batch operations ("make 12
  social sizes of this artboard"), and figures for Autopaper. A plain editor
  cannot keep a figure honest to its sources.

### Rebuild priority: **medium-low**

Its most valuable parts are the doc model pattern and the geometry crate, not
the editor. Autopaper charts and figures would use them. Rough effort the
Choir way: a narrow "figure" desk (SVG model, booleans via the geom crate,
charts from data, export) is about 2–3 weeks. A real Illustrator-class editor
is months and not worth it.

---

## 3. photo-studio (Photoshop clone)

### What it is

- A raster editor: layers, masks, adjustment layers, 27 blend modes, layer
  effects, selections (GrabCut, maxflow, live-wire, saliency, quick select),
  retouching, filters, type, shapes, PSD interchange, artboards, video and
  animation timeline, scripting, an **AI Assistant that plans and runs
  commands from natural language**, and **C2PA provenance**.
- **Stack:** TypeScript (`tsc --noEmit` in the build), Vite, **WebGPU
  compositor with a WebGL2 fallback** (`src/renderer/webgpu.ts`, `webgl.ts`,
  WGSL shaders), a worker engine (`src/engine/`). Deps: **ag-psd 31.0.2
  (MIT)**, **pathkit-wasm 1.0.0 (Skia, BSD-3)**, pako (MIT/Zlib), yjs (MIT),
  lucide. Rust wasm `wasm/src/lib.rs` (31 lines: histogram, flood fill, PNG
  encode via the `png` crate). The "Rust engine" is decorative; the real
  engine is TS. A prebuilt `public/photo-engine.wasm` (166 KB) is committed.
  `public/THIRD_PARTY_NOTICES.txt` is thorough. Fonts are OFL; the SCOWL
  dictionary is MIT-like. **No GPL/AGPL.** Size: 17 MB, ~151k TS lines, 280
  test files (~58k lines) plus Playwright config.

### Quality

- **A real engine.** `model/tiles.ts` is a sparse grid of immutable 256 px
  tiles where history snapshots share tile IDs, so undo is a reference swap.
  That is the right design. There is a GPU compositor with a float pipeline
  and LUT/filter shaders, plus a large PSD layer (`model/psd*.ts`, ~20 files on
  top of ag-psd: adjustments, text, masks, patterns, high-depth).
- **Segmentation is real algorithm work:** GrabCut, maxflow, live-wire,
  saliency (`imaging/segment/`). Implementations are likely simplified;
  results unverified.
- **The C2PA writer is hand-rolled** (`imaging/provenance/`: CBOR, COSE,
  JUMBF, X.509 self-signed with ECDSA P-256, XMP). The code says plainly that
  the signatures are self-signed and vouched for by no one.
- **Generation signs:** the same agent voice and a feature list as long as
  Photoshop's. Much of it is probably shallow (video layers, 3D-ish features,
  print). The core layer/mask/tile/compositor looks credible.

### Worth taking

1. **The Assistant pattern** (`features/assistant/`, ~2,300 lines). Read this
   first. It has:
   - a **command catalog** built from the live registries (~870 commands);
   - the model gets a short common list plus `search` and `details` tools
     (`catalog.ts`);
   - typed **ops** (`ops.ts`: `op.select`, `op.crop`,
     `op.add_adjustment_layer`, `op.add_text`, ...);
   - a **plan** of steps checked against the registry before the user sees
     it (`plan.ts`: argument coercion, selection-effect checking, problems in
     plain words);
   - a **runner** that executes through the scripting host as one undo step
     named after the request (`runner.ts`);
   - spending commands (`AI_COMMANDS`) only open a prefilled dialog, and
     destructive or meta commands are blocked (`blockedCommand`).

   This is close to the app-agent shape Choir wants: NL request → plan →
   verified steps → one revertible act, with spend and authority gates. Port
   the design, not the code.
2. **The provenance model** (`model/provenance.ts`): sources hashed with
   SHA-256, edit steps grouped and "undone" kept honestly, AI steps, exports,
   all embedded in exports as XMP and C2PA. It aligns with Choir's evidence
   and Trace doctrine. The C2PA/CBOR/COSE/JUMBF code is a usable reference
   if Choir wants signed media provenance, with a real signing key in place
   of the self-signed one.
3. **The tile-grid model** (`model/tiles.ts`) for any Choir raster/image
   editing, so history is cheap.
4. **ag-psd** (MIT) directly, if PSD import or export is ever needed.
5. **Segmentation algorithms** as references only.

### App-agent design (Choir "Image" desk, extending the existing `image` app)

- **State as objects:**
  - **Source images are `choir.media_item`** (existing kind, content hash):
    the original bytes in a blob, never modified.
  - **`choir.photo_document`** (new kind, `IdentityExternalKey`,
    `Versioned: true`): the layer tree, adjustment specs (`AdjustmentSpec {
    kind, params }` is already plain data), masks, text, layer styles, and
    references to tiles. This is photo-studio's `PhotoDocument` minus the
    pixels.
  - **Pixels** use photo-studio's tile grid (`model/tiles.ts`): sparse
    immutable 256 px tiles whose IDs are shared between history snapshots.
    These map one-to-one onto **content-addressed blobs** (tile id = content
    hash). A history step then writes only the tiles it touched plus a new
    small document version. **Tiles must be blobs in host content storage,
    never Dolt rows**: a 24 MP image is about 370 tiles × 256 KB, and a brush
    session would otherwise fill Dolt history.
  - **Where it does not map:**
    - Brush strokes are hundreds of dabs per second. Commit on stroke end,
      as photo-studio's history already does.
    - The GPU compositor's caches, frame caches and video layers are
      derived or ephemeral.
    - Large intermediate history should be garbage-collectable, so the
      history bound must be declared, as the autopaper design requires for
      the global store.
  - **Provenance needs no new kind.** Photo-studio's `ProvenanceRecord`
    (sources with SHA-256, steps, AI steps, exports) maps onto the existing
    W3C-PROV kinds in the registry:
    - `choir.provenance_entity` for each source and export;
    - `choir.provenance_activity` for each edit step or AI generation;
    - `choir.provenance_agent` for the owner, the photo desk or the model;
    - edges `was_derived_from`, `was_generated_by` and
      `was_associated_with`.

    The registry already has these, so photo provenance becomes part of the
    computer's evidence graph instead of an XMP side-car.
- **Links to other apps' objects:**
  - A photo `captured_from` a `choir.web_capture` (an image pulled from an
    article).
  - An exported crop `was_derived_from` the photo version and is
    `transcludes`-ed into a Texture or Autopaper story.
  - Research's provenance check (reverse search, EXIF) attaches as
    `choir.agent_evidence` `attests_to` the media item.
  - A board or figure can `references` a photo layer.
- **What versioning gives for free:**
  - Non-destructive history across sessions. Photo-studio's "versions" and
    "restore" features (`persistence/versions.ts`, `features/restore/`) become
    Dolt history.
  - Branches for edit variants ("warm vs cool grade", "crop A vs B") share
    every untouched tile.
  - An exact, auditable chain from the source hash to the published bytes.
- **Published vs private:**
  - Source photos, working documents and generation history stay private.
    Unpublished captures and personal photos must never leak.
  - When an image ships in a published Texture, publish only the
    **rendered export** to the Autopaper object graph as a
    `choir.artifact_manifest`/blob, plus a **redacted provenance chain**:
    the source hash and source capture if it is itself shared, the activity
    labels, and AI-generated flags. Private source bytes and intermediate
    tiles are not published.
  - Optionally attach a C2PA manifest signed with the platform's key
    instead of a self-signed one. Other computers adopting the story get the
    image with honest provenance.
- **Direct manipulation:** brush, select, mask, transform, adjust, crop,
  type.
- **Verbs:** `open(file)`, `crop(rect|aspect)`, `resize`,
  `adjust(kind, params)` (non-destructive), `select_subject()`,
  `mask(selection)`, `remove_object(selection)`,
  `add_text(text, style, rect)`, `apply_style`, `export(format, size)`,
  `batch(files, recipe)`, `describe()`, `provenance()`.
- **Desks it calls:** research (source and licensed images, fact-check a
  photo's origin), Texture (place an image in a document and keep it
  linked), model/provider calls through the gateway for generative fill
  (spend gated, as the Assistant already does).
- **Why it is better as an agent:** recipes over many images (Autopaper
  thumbnails and crops), honest provenance for every published image (fits
  Choir's evidence doctrine), and NL editing ("crop to the subject, make it
  warm, 1200×630").

### Rebuild priority: **medium-high for a narrow slice**

Autopaper needs image crop, resize, thumbnails and provenance now. The
Assistant plan/verify/run pattern is the reusable app-agent template.
Rough effort: a narrow image desk (crop, resize, adjust, text overlay, export
with provenance; server or wasm image ops) is about 1–2 weeks. Porting the
Assistant pattern into a generic Choir "app desk" harness is a further 1–2
weeks. A full Photoshop is not worth it.

---

## Group summary

**Top picks**

1. **The photo-studio Assistant pattern** (`photo-studio/src/features/assistant/`):
   catalog with search/details tools, typed ops, a plan checked against the
   registry, a runner that makes one undo step per request, and spend and
   authority gates. This is the best existing template in this group for
   Choir's "app desk that operates itself from NL" and should generalize to
   every app.
2. **The vector-studio `VDocument` model** (`vector-studio/src/model/document.js`):
   flat immutable node store, transactions with before/after records, remote
   rebase. The cleanest fit for ledger-backed app state.
3. **The vector-studio `geom` crate** (`vector-studio/wasm/geom/`, i_overlay +
   kurbo, MIT): a small, reusable 2D geometry engine.
4. **The photo-studio provenance model and C2PA writer**
   (`photo-studio/src/model/provenance.ts`, `src/imaging/provenance/`) for
   signed, evidence-linked media.

**Reusable pieces across apps**

- A **command registry** (every UI action is a command by id). All three have
  one, and it is the natural verb surface for an app desk. Choir should make
  "every direct manipulation is a named command/act" a rule for every app.
- A **sandboxed script API** (vector-studio `scripts/runner.js`, photo-studio
  `scripting/host.ts`) used both by user scripts and by the AI runner. One
  surface serves user, agent and external driver alike, which is exactly the
  owner's three-mode requirement.
- **Transaction = undo step = one semantic act** in all three. This maps
  directly to ledger acts.
- **Versioned file formats with step migrations** (`.vstudio` in
  `features/files/format.js`, `.photo` in `model/serialization.ts`).
- **Object graph shape shared by all three** (see each point 4):
  - one versioned `IdentityExternalKey` document kind per app
    (`choir.board`, `choir.figure`, `choir.photo_document`);
  - heavy payloads (strokes, images, tiles) as content-hash blobs, never as
    Dolt rows;
  - shareable parts (symbols, palettes, media) as their own content-hash
    objects;
  - cross-app links using the existing edges (`cites`, `references`,
    `transcludes`, `derived_from`, `has_media`, PROV edges);
  - one commit per semantic act, using `PutBatchConditional` for
    optimistic concurrency in place of each app's Yjs or hand-rolled rebase;
  - Dolt branches for variants.

  What gets published to the Autopaper object graph is always a **frozen
  rendered artifact** (`choir.artifact_manifest` + blobs) with
  `was_derived_from` and `cites` edges to shared sources. The live editable
  document stays private on the computer. The three apps need only three new
  kinds; everything else reuses the registry (`media_item`, `artifact_*`,
  `provenance_*`, `web_capture`, `texture_revision`, `work_item`).

**Alarms**

- **Repo hygiene:** `whiteboard/tmp/` is 561 MB of committed agent A/B build
  output (full `dist/` copies, candidate bundles). This is the 587 MB cause;
  it is not vendored libraries or a fork. It also means many unreviewed
  minified bundles and wasm binaries sit in the tree.
- **Committed prebuilt binaries:** `vector-studio/src/wasm/geom.wasm`,
  `photo-studio/public/photo-engine.wasm`, and pdf.js wasm in
  `whiteboard/tmp`. None can be checked against source without rebuilding;
  rebuild from source if any is adopted.
- **Attack surface:** hand-written PDF, PostScript, JBIG2, JPX, TIFF and PSD
  parsers in vector-studio run in the main page context. Do not run them on
  untrusted files in Choir outside a sandbox or capsule.
- **Licenses:** all fine for Choir (MIT, BSD, Apache, ISC, OFL). Minor notes:
  MPL-2.0 French dictionary and axe-core (test only), and one LPPL-1.3c
  hyphenation file in vector-studio. No GPL/AGPL anywhere in the three apps.
- **Quality:** generated at scale for feature parity. `whiteboard/src/main.js`
  is a 14k-line god file. Trust the engines and models more than the feature
  lists; assume the long tail is lightly verified.
