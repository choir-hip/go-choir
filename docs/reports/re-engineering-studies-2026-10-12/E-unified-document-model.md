# E. One document model for captures and Texture

Read-only design research, 2026-10-12. Repo: go-choir at `a401c878`;
reference: `~/mike` (TypeScript).

Evidence labels: **[V]** means I read it in the code at the cited line. **[I]**
means inference or design proposal. **[D]** means an owner decision is still
needed.

---

## 0. Summary

The owner's model: one block model, two kinds of author, the world (read-only)
and Texture. A story transcludes source blocks the same way one Texture
transcludes another. Choir already has most of the parts for this, but six
facts in the code shape the design.

1. **Texture block ids are not stable today.** **[V]** There are three
   id-minting paths and none of them is content-addressed:
   - Edit ops mint UUIDs (`internal/textureowner/tools_texture.go:1164,1177`;
     `source_ref` at `:1086`).
   - Whole-document `content` writes mint ids scoped to the revision,
     `p-<doc>-<rev>-<seq>`, so every block id changes on every content-mode
     revision (`tools_texture.go:941-945,993-1001`;
     `texture_lineage.go:148-151`).
   - The owner's editor mints positional ids, `<docid>-<kind>-<index>`, which
     shift whenever a paragraph is added or removed
     (`frontend/src/lib/texture-structured-editor-doc.ts:30-34`).

   Today a Texture block can only be addressed safely inside one revision.
   Content addressing fixes cross-revision transclusion for free.
2. **Captures do not keep the bytes that structured extraction needs.** **[V]**
   - `WriteCaptures` stores one body ref as both the content blob and the
     extracted-text blob (`internal/sourcegraph/captures.go:77-84`).
   - That body is already flat text.
   - Poll captures of RSS items are only the feed *description*, whitespace
     collapsed (`internal/sources/rss.go:101,132-160`).
   - `ExtractReadableHTML` returns `(title, flatText)`
     (`internal/sourcefetch/readability.go:14-32`).
3. **A re-capture of the same item is dropped.** **[V]** Capture identity is
   `autopaper:capture:<item.ID>` (`captures.go:66-75`), and `item.ID` is
   derived from source plus GUID or URL (`internal/sources/types.go:194-205`).
   A later body for the same URL counts as "Repeated" and writes nothing. The
   app design's claim that "a later capture of the same URL with a different
   body is a different capture" (`docs/autopaper-app-design-2026-10-11.md`
   §4.4) is not true of W1 polls today. This matters for D (re-capture
   alignment), which is deferred.
4. **A source entity carries one anchor.** **[V]**
   - The citation anchor (quote) lives on the *source entity's* selectors, not
     on the `source_ref` node.
   - The publication transclusion is built from the entity's **first**
     selector only (`internal/platform/source_metadata.go:153-158,200-209`).
   - So one capture cited at three paragraphs either needs three entities or
     loses two anchors.

   There are also two Go selector shapes:
   - `texturedoc.SourceSelector{Kind, Data map}` (`internal/texturedoc/schema.go:51-54`)
   - `types.SourceEntitySelector{selector_kind, start/end_seconds, text_quote, content_hash}` (`internal/types/texture_provenance.go:112-118`)

   The contract vocabulary has 13 selector kinds
   (`internal/sourcecontract/selector.go:5-19`), and the app design's JSON
   uses `exact`, not `text_quote` (the renderer accepts both:
   `texture-source-renderer.ts:68`).
5. **Every publish mints a new publication and route.** **[V]**
   - `PublishTexture` mints a new publication and route each time
     (`internal/platform/service.go:202,217`).
   - It hard-codes `edition_label: "v1"` and `supersedes_version_id: ""`
     (`internal/platform/publication_graph.go:129,138,522`).
   - Resolve takes the last `has_version` edge
     (`service_publication_read.go:96`).
   - The tables already have `publications.latest_version_id` and
     `publication_versions.supersedes_version_id`
     (`internal/platform/store.go:78,95`), so per-version permalinks fit the
     existing schema.
6. **Mike's slow step is the models, not the cascade.** **[V]** The
   exact → whitespace → punctuation cascade is pure string matching
   (`~/mike/backend/src/modules/chat/engine/verifyCitations.ts:41-58,137-154`).
   The slow part of Mike's checker is **two model calls per citation**: one
   extracts the citations and one judges support
   (`~/mike/backend/src/modules/citations/citations.tasks.ts:10-16`). In Choir,
   citations are already structured nodes, so the extract call disappears.
   With block addressing, quote matching runs against one block, which takes
   microseconds. **[I]** The work that stays slow is (a) capturing a source
   that is cited but not yet captured, and (b) judging support with a model.
   The iterative design should place its asynchrony there.

The design in one paragraph **[I]**:
- **Block model:** `choir.texture_doc` grows a few block types and becomes
  the one block model. World documents are captures rendered as texture_doc
  trees, with `author_kind: "world"`.
- **Block ids:**
  - Every block gets a computed content address, `cid = b_<sha256(kind‖normtext)[:12]>`,
    with an ordinal suffix for duplicates.
  - World blocks use `cid` as their id.
  - Texture blocks keep a lineage `id` as the edit handle and also get a
    computed `cid`.
  - Transclusion and citation always address `(doc, pinned version, cid)`.
- **Document identity:** a Merkle hash over the block hashes.
- **Verification:** runs in tiers.
  - Structural and quote checks are deterministic, memoized by
    `(quote/host hash, cited-block hash, matcher version)`, and run at commit
    and at publish.
  - The model support judgment is asynchronous and memoized the same way.
  - Passes become a monotone overlay on an immutable version.
  - Failures go to Texture as Updates. Only Texture writes the correcting
    revision, which becomes the next version.
- **Publishing:** adds a version to the document's one publication.
  - `/pub/texture/<slug>-<id>/v7` is immutable.
  - The bare route follows the latest version.
  - The bundle carries prev, next and latest for the chevrons and the "new
    version available" control.

---

## 1. Unified block schema

### 1.1 What exists **[V]**

`choir.texture_doc.v1` (`internal/texturedoc/schema.go:13-31`) is a
ProseMirror-shaped JSON tree, `Node{type, attrs, content, text, marks}`.

| Kind | Accepted types | Ids |
|---|---|---|
| Blocks (`:187-263`) | `paragraph`, `heading` (level 1..6), `bullet_list`, `ordered_list`, `list_item`, `blockquote`, `code_block`, `horizontal_rule` | every block needs `attrs.id` |
| Inline (`:265-305`) | `text`, `hard_break`, `source_ref` (atom: `source_entity_id`, `display_mode`) | — |

- Marks are only `strong`, `emphasis` and `code` (`:307-316`). There is **no
  `link` mark**.
- The validator ignores unknown attrs. `requireNodeID` checks only `id`
  (`:338-347`). So adding attrs is backward-compatible. **[V]**
- Text payloads are rejected if they contain "legacy citation syntax",
  including `\[\d+\]` (`:318-336`). **[V]** A captured Wikipedia or news page
  with `[12]` footnote markers would therefore **fail validation**. The check
  is a guard against model-authored citations and must apply only to Texture
  authors. **[I]**

Mike's reading model (`~/mike/backend/src/lib/documentModel/types.ts:9-41`) is
a flat `DocBlock[]`.
- Kinds: `heading|paragraph|list_item|table|code|quote|note|figure|page_break`.
- Fields: `level`, `label`, `rows: string[][]`, `page`, `source` ("lines
  12-14"), `ocr`, `inferred`, `language`.
- Ids are ordinal per format: `m<line>` for Markdown (`fromMarkdown.ts:31,141`),
  `p<page>.<n>` for PDF (`fromPdf.ts:202`).
- Mike keeps ids across document versions by **alignment**, not by hashing
  (`~/mike/backend/src/lib/docx/blockIds.ts:139-223`).

### 1.2 Proposed schema: `choir.texture_doc` v1 plus additive extensions **[I]**

I recommend keeping the tree, not flattening to Mike's list. Texture,
`texturedoc.Project`, `Outline`, and the frontend editor all walk the tree
(`internal/texturedoc/projection.go`, `outline.go`,
`texture-structured-editor-doc.ts`). Every change below is additive. Bump the
schema string to `.v2` only if the owner wants old readers to refuse new node
types. Old readers would otherwise fail anyway on unknown block types
(`schema.go:259-260`).

Root `doc.attrs`:

```jsonc
{
  "id": "<doc lineage id>",            // Texture doc id, or capture id for world docs
  "author_kind": "texture" | "world",  // new; default "texture"
  "doc_hash": "sha256:…",              // computed (§1.4), never authored
  "origin": {                           // world docs only
    "capture_id": "obj:choir.web_capture:…",
    "url": "…", "canonical_url": "…",
    "fetched_at": "…",
    "raw_sha256": "sha256:…",          // the bytes fetched
    "extractor": "choir.html_blocks/1"  // extractor name + version
  }
}
```

Every block's `attrs` gains:

| attr | who sets it | meaning |
|---|---|---|
| `id` | Texture: lineage handle (existing). World: equals `cid`. | What edit ops address. |
| `cid` | computed at commit or extraction | Content address (§1.3). What transclusion and citation address. |
| `h` | computed | Full node hash: kind, attrs without `id`/`cid`/`prov`, marks, and children (§1.4). |
| `prov` | world extractor | `{page, char_start, char_end, dom_path, ocr, inferred}`. Excluded from every hash. |

New block types, needed for captures and later for PDFs:

| type | content | attrs | why |
|---|---|---|---|
| `figure` | optional inline caption | `src` (absolute URL), `alt`, `media_ref` (blob sha, later) | `<figure>/<img>/<figcaption>`; Mike `figure` |
| `table` → `table_row` → `table_cell` | cell content: paragraphs | `table_cell.header: bool`, `colspan`, `rowspan` | `<table>`; Mike `rows`. Cells are addressed relative to the table (§1.3). |
| `page_break` | leaf | `page` | PDF page boundaries (Mike `page_break`) |
| `note` | blocks | `label`, `note_kind` (footnote, endnote, aside) | footnotes, PDF notes (Mike `note`) |
| `transclusion` | leaf atom | `ref` (§3) | block-level transclusion, for both authors |

Already present and reused: `heading` (with level), `blockquote` (Mike calls
it `quote`), `list_item`, `code_block` (add attr `language`).

New inline marks and nodes:
- `link` mark (`href`). Needed for world text and harmless for Texture.
- Optionally `sup`/`sub`. Low priority.
- `source_ref` gains an optional `anchor` (§3.2).

The validator becomes author-aware **[I]**:
- `author_kind: "world"` skips `validateTextPayload` (the legacy syntax guard).
- World docs also skip the "every source entity must be referenced" rule
  (`schema.go:125-135`). World docs carry no source entities.
- World docs reject `source_ref` nodes. The world does not cite in Choir's
  vocabulary; their links are `link` marks.

### 1.3 Content-addressed block ids

**Normalization** **[I]**. Use one function, shared with the verifier (§4):

```
normtext(block) = NFC(concat of descendant text nodes, hard_break → "\n")
                  → collapse each run of whitespace to one space → trim
```

Marks are excluded, so bolding a phrase does not move a citation. Case and
punctuation are kept. Mike's tolerant matching belongs to quote *matching*,
not to *identity*.

**Id formula**:

```
leaf textual block:   cid = "b_" + hex(sha256(kindclass + "\x00" + normtext))[:12]
figure:               cid = "b_" + hex(sha256("fig\x00" + src + "\x00" + alt + "\x00" + caption_norm))[:12]
container (list,
  blockquote, note):  cid = "b_" + hex(sha256(kind + "\x00" + join(child cids, ",")))[:12]
table:                cid = "b_" + hex(sha256("table\x00" + rows_norm))[:12]
table cell / row:     "<table cid>/r<R>c<C>"   (positional inside a content-addressed table)
horizontal_rule,
page_break:           "b_hr" / "b_pb" + ordinal (structural, never cited)
```

- `kindclass` is `p` (paragraph), `h<level>`, `li`, or `code`. A heading and a
  paragraph with the same text are different blocks.
- **Hash length.** 12 hex characters is 48 bits. Inside one document of n
  blocks, a birthday collision has probability about n²/2⁴⁹, roughly 2×10⁻⁷
  at n = 10⁴. Collisions across documents do not matter, because an address
  is always `(doc, version, cid)`. A true collision inside one document is
  handled exactly like a duplicate (below), so correctness never depends on
  the hash being unique.
- **Duplicates.** Repeated paragraphs are common in captures: "Advertisement",
  "Read more", "—". The first occurrence in document order keeps the bare
  `cid`. The k-th repeat becomes `cid.k` (`b_3f2a….2`).
  - The ordinal is fully deterministic for a given document, which is all a
    pinned reference needs.
  - Ordinals shift only if an earlier copy of the *same text* is inserted or
    removed. That affects alignment across versions (D), not resolution
    inside one pinned version.
- **An alternative I considered and rejected:** disambiguating by the hash of
  the previous block (context hash). It survives some edits better, but it
  makes every id depend on its neighbour, so an unrelated edit renumbers a
  duplicate. The simple ordinal is easier to explain and to test.
- **Empty blocks.** Blocks with no text after normalization are dropped by
  the world extractor. A Texture block can be empty while the owner is
  editing; it gets `b_empty.k`.

### 1.4 Document identity

```
h(node)   = sha256(canonical JSON {type, attrs − {id,cid,h,prov}, marks, text, [h(child)…]})
doc_hash  = sha256(schema + "\n" + join(h(top-level block), "\n"))
```

This is a Merkle construction. Two captures whose extracted blocks are
identical have the same `doc_hash`, even if the raw bytes differ (ads, nonces).
The `raw_sha256` in `origin` still pins the exact bytes. **[I]**

A world document's version is its `doc_hash`. The capture object
(`choir.web_capture`, content-hash identity mode,
`internal/objectgraph/registry.go:53`) then records `raw_sha256`,
`blocks_ref` (the body-store sha of the document JSON) and `doc_hash`.

If the extractor version changes, the same raw bytes produce a new
`doc_hash`. Old citations stay pinned to the old document JSON, which is still
in the body store by its own sha. That is correct: a citation pins what the
writer saw. **[I]**

### 1.5 Texture blocks: content-addressed too, or lineage only?

| Option | For | Against |
|---|---|---|
| **(a) `id` becomes the content address in every revision** (Texture ids churn as text changes) | One id concept. A transclusion of a Texture block equals a transclusion of a source block. | Edit ops break mid-batch: `update_block_text` then `insert_source_ref` on the same block, and the id changed in between. The outline the model read goes stale after every edit. All stored revisions use the old ids, so this needs a migration. "Same block, revised" is no longer expressible, and the chevron diff loses it. |
| **(b) lineage `id` only, stays as today** | No change. | Lineage ids are *already* unstable in two of three write paths (§0.1). A transclusion of a Texture block would break on the next content-mode revision. And it does not satisfy "content-addressed block ids". |
| **(c) Recommended: both.** `id` is the lineage and edit handle. `cid` is computed by the reducer at commit, for both authors. | Edit ops keep working unchanged. Transclusion and citation address `cid`, so they survive content-mode rewrites and owner-editor renumbering whenever the text is unchanged. Memoized verification keys on `cid`/`h` directly. Additive: old revisions get `cid` computed on read. | Two ids to explain. Mitigation: the outline shows both, and the prompt rule is "edit by `id`, cite by `cid`". |

Under (c), lineage `id` stability should then be repaired, so that "same
block, revised" survives content-mode writes and owner-editor saves. Mike
already has the algorithm: carry ids from the previous revision by alignment
(`~/mike/backend/src/lib/docx/blockIds.ts:59-223`). It matches common prefix
and suffix, anchors on signatures unique to both sides, uses LCS on small
gaps, and pairs the rest by Dice similarity of at least 0.5. Port it to Go in
the reducer: when a write arrives as whole content or from the editor, carry
the previous revision's `id`s onto the aligned blocks. **[I]**

The same alignment function later becomes D (aligning blocks across
re-captures). Write it once.

This is red (Texture canonical writes) and can follow C. Transclusion does
not depend on it, because transclusion uses `cid`.

---

## 2. Ingestion: structured blocks instead of flat readability text

### 2.1 What exists **[V]**

- `golang.org/x/net v0.55.0` is in `go.mod`, and `x/net/html` is already the
  parser in `internal/sourcefetch/readability.go:10,15`. There is no goquery
  and no readability port.
- `readability.go` already does the hard part:
  - candidate selection: `<article>`, `<main>`, else `<body>`, scored by long
    lines (`:78-107,232-244`);
  - noise skipping: tags, ARIA roles, and class/id tokens such as cookie,
    consent, nav and sidebar (`:136-185`);
  - noise lines (`:196-230`);
  - a list of block tags (`:187-194`).
- It then throws the structure away by writing `\n` around block tags and
  concatenating text (`:109-134`).
- Callers:
  - `internal/content/content.go:1910`
  - `internal/sources/article_import.go:59`
  - `internal/browsercontrol/handler.go:710,806`
- PDF extraction shells out to poppler `pdftotext -layout` and `pdfinfo`
  (`internal/content/extract.go:104-120`). Pages are split on `\f` into coarse
  `page-N` selectors (`:396-418`).
- There is an existing proto-block model: `content.Selector{id, kind, label,
  text, start_char, end_char}` (`internal/content/extract.go:24-31`). Its ids
  are ordinal (`page-3`, `section-2`, `chunk-4`), at page or section or 12k
  chunk size, which is too coarse for citation.

  Following the repo's rule to check for an existing replacement before
  building: this is a coarse precursor, not a replacement. The block model
  should *subsume* it, so that `ListContentSelectors` becomes a view over
  blocks. **[I]**

### 2.2 Proposed: `sourcefetch.ExtractBlocks` **[I]**

```go
// package sourcefetch (or a new internal/docmodel shared by texturedoc + sourcefetch)
type Extracted struct {
    Title    string
    Doc      texturedoc.StructuredTextureDoc // author_kind "world", cids and hashes filled
    Text     string                          // texturedoc.Project(Doc): the flat projection
    Warnings []string
}
func ExtractBlocks(raw []byte, baseURL string) (Extracted, error)
```

Steps:
1. Parse with `x/net/html`. Choose the candidate exactly as today, but score
   candidates by the block text they produce.
2. Walk the DOM and emit blocks. Skip with the existing `skipReaderHTMLNode`.

   | HTML | Block |
   |---|---|
   | `h1–h6` | `heading{level}` |
   | `p` | `paragraph` |
   | `ul/ol>li` | `bullet_list`/`ordered_list` > `list_item` > `paragraph`; `ol` keeps `start` |
   | `blockquote` | `blockquote` |
   | `pre` | `code_block` (with `language` from `class="language-x"`) |
   | `table` | `table`/`table_row`/`table_cell{header}` |
   | `figure`, `img` | `figure{src (absolutized against baseURL), alt}`; `figcaption` becomes its inline caption |
   | `hr` | `horizontal_rule` |
   | `br` | `hard_break` |

   Inline mapping: `strong/b` → strong, `em/i` → emphasis, `code` → code,
   `a[href]` → link mark (absolutized).

   "Anonymous" text, meaning text directly inside a `div`, `section` or
   `article` among block siblings, is gathered into a `paragraph`. This is
   the usual readability rule and the step most likely to need tuning.
3. Drop blocks whose `normtext` is a noise line (reuse `readerNoiseLine`).
   Drop empty blocks. Merge adjacent runs of only whitespace and text.
4. Compute `cid`, then duplicate ordinals, then `h`, then `doc_hash` (§1.3,
   §1.4).
5. Keep `ExtractReadableHTML(data) (title, text)` as a thin wrapper:
   `ExtractBlocks`, then `Project`. All four callers keep working, and the
   flat text equals the block projection. Verification depends on that: the
   text a quote is matched against is exactly the joined block texts, so
   block offsets are well defined.

**Capture path changes** **[I]** (`internal/sourcegraph/captures.go:50-111`,
corpusd `internal/platform/captures.go`):
- Store the **raw bytes** as the content blob, by sha256. Today the raw HTML
  is discarded and `ContentBlobID == ExtractedTextBlobID`.
- Store the structured doc JSON as a derived blob by its sha (`blocks_ref`).
  Keep the projection as `ExtractedTextBlobID` for existing readers.
- Extract eagerly at capture time. It is deterministic, needs no model, and
  costs about a millisecond per page. Memoize by
  `(raw_sha256, extractor_version)`, so a recapture of identical bytes reuses
  the document.
- For old captures that have only flat text, build blocks by splitting on
  blank lines or newlines into paragraphs (Mike's `textToModel` equivalent).
  Mark them `extractor: "choir.text_blocks/1"`. Citations still work, only
  with coarser blocks.
- **Poll captures are feed descriptions, not articles.** **[V]**
  `rss.go:101`, `cleanFeedDescriptionText`. A story that cites an article
  needs that article's *page* to be captured: `FetchURL`/`ImportURL` must
  write a capture, which slice C already requires (app design §4.7 stage 2).
  RSS `content:encoded` HTML, where present, should go through
  `ExtractBlocks` rather than `cleanFeedDescriptionText`. **[I]**

**Later.**
- **PDF:** use `pdftotext -bbox-layout`. It emits XHTML with
  page/flow/block/line/word boxes, the same poppler binary already used, so
  it gives blocks with `prov.page`. Or port Mike's heuristics for
  `fromPdf.ts`: body size by font, headings by size ratio, and removal of
  repeated headers and footers (`~/mike/backend/src/lib/documentModel/fromPdf.ts:180-231`).
  **[I]** I have not verified that the staging guest image has
  `pdftotext -bbox-layout` beyond the `-layout` call it uses today.
- **Transcripts:** one block per segment, with `prov.start_seconds` and
  `prov.end_seconds`. Then the `timestamp_range` selector becomes block
  provenance, not a selector.

---

## 3. Transclusion

### 3.1 What exists **[V]**

- `source_ref` is an inline atom that names `source_entity_id`. The entity is
  a separate per-revision record with `target`, `selectors`, `display`,
  `evidence` and `provenance` (`schema.go:33-80`).
- Publish normalizes each entity into exactly one transclusion row:
  - `host_selector` is `{type: "citation_marker", source_entity_id}`;
  - `source_selector` is the selectors, with the first one used for
    `snapshot_text`/`content_hash` (`source_metadata.go:135-213`).
  - It is written as an object plus a `transcludes` edge
    (`publication_graph.go:511`) and read by `graphTransclusions`
    (`service_publication_read.go:401-437`).
  - Table: `publication_transclusions(transclusion_id, publication_version_id,
    source_entity_id, host_selector_json, source_selector_json, relation_type,
    default_display_mode, snapshot_text, content_hash, …)` (`store.go:260-274`).
- The renderer already supports:
  - numbered and expanded source refs with a quote popover
    (`texture-source-renderer.ts:360-405`);
  - Texture-to-Texture references `texture:<doc>@<rev>` with a "Version pin
    vN" and a "Newer version available" indicator (`:411-500`).

  These references are *markdown-link* based, not structured nodes.

### 3.2 Proposed reference shape **[I]**

A single reference value is used by both the block node and the inline anchor:

```jsonc
"ref": {
  "doc":   "obj:choir.web_capture:…"  |  "texture:<doc_id>",
  "ver":   "sha256:<doc_hash>"        |  "rev:<revision_id>"  |  "pub:<publication_version_id>",
  "block": "b_3f2a9c01d4e7",           // cid (with .k ordinal if duplicate)
  "h":     "sha256:…",                 // expected h(block): integrity, and the verification memo key
  "quote": {                           // optional sub-block anchor
    "exact": "Pricing is $0.10 input…",
    "start": 0, "end": 58             // char offsets in normtext(block), filled by the runtime
  }
}
```

- **Block-level `transclusion` node** (new block type). Its attrs are `id`,
  `cid`, `ref`, and `display` (`"block"`, `"pullquote"` or `"card"`). It is
  an atom: the revision stores only the reference, never a copy of the text.
- **Inline citation.** `source_ref` gains an optional `anchor: ref`. The
  `source_entity_id` stays and names the **bibliography entry**, the
  document. The anchor names the **block and quote** this occurrence rests
  on. One capture cited at three places then means one entity and three
  anchors, which removes the one-anchor-per-entity limit (§0.4).
- `source_entities` stay as the per-revision bibliography: the toolbar,
  numbering, display metadata, and the `mark_source_unused` tri-state.
  Entity `target` gains `{kind: "web_capture", capture_id, doc_hash}` and
  `{kind: "texture", doc_id, ver}`. Entity `selectors` become legacy-only: a
  `whole_resource` entity with anchors on its refs replaces quote selectors.

**Which selectors remain needed** **[I]**:

| selector (sourcecontract) | fate |
|---|---|
| `text_quote` | kept, as `ref.quote.exact`, scoped to one block |
| `text_position` | kept, as `ref.quote.start/end` (runtime-derived, never model-authored) |
| `whole_resource` | kept, for document-level citation with no anchor (legacy, or "see generally") |
| `paragraph_heading`, `byte_range`, `selector_set` | obsolete; the block address subsumes them |
| `page_range` | becomes block `prov.page`. Kept only for citing a page *range* of a PDF with no block. |
| `timestamp_range`, `transcript_segment` | become transcript block provenance. Kept until transcripts are block docs. |
| `table_range`, `table_cell` | kept as `<table cid>/rRcC` addressing |
| `image_region` | kept (future: region of a `figure`) |
| `data_vintage` | orthogonal; kept |

This also fixes the selector-shape split: a single `Ref` struct in Go,
shared by `texturedoc` and `types`. `types.SourceEntitySelector` (the one with
`text_quote`, `content_hash` and `seconds`) becomes a compatibility reader.

### 3.3 Resolution and rendering **[I]**

**In the owner's live editor (guest):**
- The Texture handler resolves refs through corpusd:
  `GET /internal/platform/docs/<doc>@<ver>/blocks?ids=…`, a new route next to
  `/internal/platform/captures/…`. For `texture:` refs it resolves against
  local revisions.
- The returned blocks are cached by `(ver, cid)` forever, because they are
  immutable.

**At publish (corpusd):**
- Materialize every ref into the publication version as
  `resolved_blocks: {"<doc>@<ver>#<cid>": {node, h, doc_title, origin…}}`.
- The public reader then renders a pinned version without touching the
  paper's computer, which the app design requires (§4.6).
- Optionally add one neighbouring block on each side for "in context". With
  copyright set aside for now, whole blocks are fine. Bounding the context
  window remains the app design's open question 4.

**Renderer:**
- For `transclusion` and anchored `source_ref`: look up `resolved_blocks`
  and render the node with the *same* block renderer used for the host.
  - Highlight `quote.start..end`.
  - Wrap it in attribution chrome: outlet, captured time, verification badge
    (§4), "open source", which opens the Source viewer at `#cid`, and
    "original ↗".
- Recursion limit: depth 2, so a Texture can transclude a Texture that
  transcludes a capture. Guard against cycles with the set of
  `(doc, ver, cid)` already being rendered.
- Missing block: an "unresolved" chip. Hash mismatch (`h` differs from the
  resolved node): a "source changed" chip. Under content addressing a
  mismatch means corruption or a bug, never drift.

**`publication_transclusions`: keep the table, re-key the rows.** One row per
**anchor occurrence**, not per entity:
- `host_selector_json = {host_block_cid, host_block_id, node_id}`
- `source_selector_json = ref`
- `snapshot_text = normtext(resolved block or quote span)`
- `content_hash = h(cited block)`

No column changes. The existing `transcludes` edges and `graphTransclusions`
reader keep working. `normalizePublicationSourceEntity` stops being the only
producer of transclusions. A new walker over `body_doc` produces them.

---

## 4. Iterative, memoized citation verification

### 4.1 Where the line between code and model goes

The owner's principle is that intelligence belongs in models, not in the
runtime. Under that principle **[I]**:

- **Deterministic code** handles anything that is a function of bytes:
  - does the address resolve, and does the hash match;
  - does the quoted string occur in the cited block under the exact →
    whitespace → punctuation cascade, including ellipsis segments (Mike's
    `verifyQuoteAgainstSource`);
  - which source span the quote corresponds to.

  This is provenance checking, the same kind of operation as hashing. Making
  it a model call would be slower and *less* trustworthy. It also costs no
  model call, as the app design already argued (§4.4, "checked for free").
- **A model** handles anything that is a judgment:
  - does the cited block *support* the claim in the host block (Mike's
    `judgeSupport`, `citations.judge.ts`);
  - and, when a check fails, *what to do about it*: re-quote, cite a
    different block, soften the claim, or drop it. That is Texture's job,
    because it changes the canonical text.
- **Not the producer.** Mike enforces that a checker never grades its own
  output (`migrations/20261010_03_citation_checks.sql`, constraint
  `verification_tasks_not_self_graded`). Choir should keep this: the support
  judge is not the Texture desk that wrote the revision. **[I]**

### 4.2 Data model **[I]**

One object kind, `choir.citation_check`, in Store B (corpusd), with
content-hash identity. Its key is derived, so a lookup is a get by id, and two
computers that check the same pair share the result.

```jsonc
{
  "tier": "structural" | "quote" | "support",
  "key": {
    "cited_h": "sha256:<h(cited block)>",
    "quote_h": "sha256:<normquote>",      // tier quote; empty when there is no quote
    "host_h":  "sha256:<h(host block)>",  // tier support only (the claim)
    "checker": "quote_match/1" | "judge:<model>@<prompt_version>"
  },
  "verdict": "exists-and-matches" | "matches-normalized" | "not-found" |
             "unverifiable" | "supports" | "partial" | "does-not-support" | "contradicts" | "unclear",
  "span": {"start": 0, "end": 58},        // tier quote: offsets in normtext(cited block)
  "excerpt": "…",                         // the source's own words for the span
  "evidence": "…",                         // tier support: a verbatim excerpt; checked with evidenceInSource
  "reason": "…",
  "checked_at": "…", "checker_run": "…"
}
```

- The memo key never includes the revision, the document or the version.
  So the **same claim citing the same block across v7, v8 and v9 is checked
  once**. That is the owner's "not redone when the citing block and the cited
  source content are unchanged".
- The quote tier does not depend on `host_h`: a quote's presence in a block
  does not depend on the surrounding sentence. Only the support tier keys on
  the host block.
- A `choir.citation_check_link` edge (or a row in the publication bundle)
  joins `(publication_version, anchor occurrence)` to the check ids. That is
  the per-version overlay.

### 4.3 When it runs and who runs it **[I]**

| Tier | When | Who | Latency | On failure |
|---|---|---|---|---|
| 0 structural: ref resolves, `h` matches | **inside the ApplyTexture reducer**, synchronously, before commit | runtime (guest), using corpusd block reads | ms | **reject the turn** with the nearest blocks' text. The model fixes it in the same cell, as today's rejection path does (`tools_texture.go:1011,1027,1067`). |
| 1 quote: cascade inside the cited block | reducer pre-check at commit (fast feedback), **authoritative at publish in corpusd** | runtime: one Go `quotematch` package, ported from Mike, shared by guest and corpusd | µs per anchor | not found: reject the turn (the model copied badly). Normalized match: accept, and store the source's own span, so the rendered quote is always the source's words. |
| 2 support judgment | **after publish**, asynchronously; also after commit for the owner lens | a model, not the producing Texture desk: either a runtime-owned verifier job (cf. `docs/definitions/choir-research-job-2026-10-11.md`) or a research Cast | seconds per anchor; memoized | `does-not-support`/`contradicts`: deliver an **Update to the Texture desk** (`choir.Updates()`, surface `internal/yaegikernel/surface.go:35`). Texture decides and writes the next revision, which becomes the next version. |
| pending capture: cited URL not yet captured | after commit | research `FetchURL` → capture (slice C) | seconds | anchor stays `pending` until the capture lands, then tiers 0 and 1 run |

**Why tier 1 belongs both at commit and at publish.**
- At commit, the model gets the cheapest possible correction loop. A bad
  copy is rejected before it becomes canonical.
- At publish, corpusd re-runs it against the body it trusts. The app design
  says publish must resolve captures only from trusted writers (§4.4).
- Both runs hit the same memo, so the second costs a lookup.

### 4.4 "Initial versions immediately, verification produces later versions" [D]

There are three readings:

- **(A) Overlay.** v7 is published at once with "checking…" badges. Check
  results attach to v7 as they land. v7's *text* never changes. The overlay
  can only gain facts, and each fact is keyed by hashes of v7's own content,
  so it cannot contradict v7.
- **(B) Re-publish.** Each completed verification pass mints a new version
  (v7 → v8) with the results baked in. The chevrons fill with versions that
  have no text change. It also breaks "a version is a revision".
- **(C) Texture-authored.** Failures are delivered to Texture. Texture
  writes a corrected revision, which publishes as v8.

**Recommendation: A for passes and pending, C for failures.** A new version
appears exactly when the text changes, and Texture stays the sole writer of
canonical revisions. This is the doctrine, and it is red. The reader still
sees versions get better over time: v7 ✓9/10 · 1 pending, then v8 with the
fix.

If the owner meant B literally, it can be built as "verification epochs":
`v7.1` and `v7.2` labels on the same text. I do not recommend it.

### 4.5 How readers see results

- Per anchor:

  | Badge | Meaning |
  |---|---|
  | ✓ | quote matches exactly |
  | ≈ | matches after normalization; the source's own words are shown |
  | … | pending |
  | ✗ | not found; this should be rare, since commit rejects it |
  | ? | source unavailable |

  The support verdict appears as a second line in the popover, with the
  judge's verbatim evidence excerpt and the judge's name. It is never
  aggregated into a score, consistent with the app design's
  "grouped by scorer, never a verdict".
- Per version: "10 sources · 10 quotes checked · support: 7 checked, 3
  pending".
- Owner lens: the failure list with reasons, and the Update each one became.
- Bundle: `PublicationBundle` gains
  `checks: {"<anchor occurrence id>": [check…]}`. This overlay is served live
  by resolve. It is not frozen into the manifest, so the manifest hash and the
  signatures stay stable (`version_history.go:50-63`).

---

## 5. Per-version permalinks

### 5.1 Data model **[I]**, mostly existing columns **[V]**

| object | change |
|---|---|
| `publications` (`store.go:71-81`) | **One per `(owner_id, source_doc_id)`.** Add or derive `source_doc_id` (versions carry it today, `store.go:88`). `latest_version_id` (exists) is the only mutable pointer. Add `version_count`. |
| `publication_versions` (`store.go:83-96`) | Immutable. `edition_label` = `"v<n>"`, from a real ordinal (today it is hard-coded `"v1"`, `publication_graph.go:129`). `supersedes_version_id` = previous version (today hard-coded `""`, `:138`). Add `ordinal int` and `doc_hash`. |
| `public_routes` (`store.go:98-110`) | One row per publication: `/pub/texture/<slug>-<shortid>`, `target_kind=publication`. `target_version_id` follows latest, or is dropped in favour of `publications.latest_version_id`. Version routes are **parsed, not stored**: `/pub/texture/<slug>-<shortid>/v<n>` resolves `(publication_id, ordinal n)`. A row per version is not needed. |
| graph | `has_version` edges get `ordinal`. A `supersedes` edge links version n to n-1 (the edge kind is already registered, `objectgraph/registry.go`). |

**Ordinal vs revision number [D].** Not every revision is published. I
recommend **contiguous published ordinals** (v1..vN, the public's numbering)
and showing the source revision number as metadata. The alternative, labels
equal to Texture version numbers with gaps, confuses readers when they
navigate with chevrons.

### 5.2 Changes to `PublishTexture` (`internal/platform/service.go:153-…`) **[I]**

1. Under `writeMu` (`:157`), look up the publication by
   `(owner_id, source_doc_id)`.
   - If there is none, mint as today: new publication id, slug and route.
   - If there is one, **append**: new version id, `ordinal = count+1`,
     `supersedes_version_id = latest`, keep the route, and update
     `latest_version_id`.
2. Idempotence: if `source_revision_id` equals the latest version's, return
   the existing version. A republish of the same revision is a no-op.
3. Decide which per-publish objects (proposal, consent, review, activity,
   attestation; `service.go:202-212`) a version append still needs. This is
   W3a's open "design §6 R2".
4. Stop embedding the **full history bodies** in every version's manifest
   (`service.go:247-255`). Storage grows as O(n²) once versions accumulate per
   publication. Version n's manifest can reference version n-1's manifest
   hash, which keeps the tamper-evident chain without copying bodies.
5. Materialize `resolved_blocks` and the transclusion rows (§3.3), and run
   tier 0 and tier 1 checks through the memo (§4.3).

### 5.3 Resolve and the UI **[I]**

- `GET /api/platform/publications/resolve?route=…/v7` returns the immutable
  bundle for v7.
- `?route=…` (bare) returns the latest bundle.
  `GetPublicationBundleByRoute` stops taking "the last `has_version` edge"
  (`service_publication_read.go:96`). It parses `/v<n>` and selects by
  ordinal.
- Both responses carry:

  ```jsonc
  "nav": {"ordinal": 7, "prev": {"ordinal": 6, "route": "…/v6"},
          "next": {"ordinal": 8, "route": "…/v8"} | null,
          "latest": {"ordinal": 9, "route": "…/v9", "published_at": "…"}}
  ```

- **Chevrons** use `nav.prev` and `nav.next`.
- **"New version available"** shows when `nav.latest.ordinal > nav.ordinal`.
  A pinned page polls a tiny `GET /api/platform/publications/latest?route=`
  (returns `{ordinal, route}`) every few minutes, or on focus. This reuses the
  existing "Newer version available" styling
  (`texture-source-renderer.ts:460-470,495`).
- Visiting the bare route shows latest with the URL bar unchanged, and
  "permalink to this version" copies `/v<n>`. **[D]** The alternative is to
  redirect bare → `/v<n>`. Both work. Not redirecting keeps shared links to
  the story "living".

### 5.4 Pinning transcluded blocks per version **[I]**

- A capture block is pinned by `(capture, doc_hash, cid)`. Content addressing
  means v7 renders the same forever. `resolved_blocks` in v7's bundle makes
  that independent of the capture store.
- A Texture-to-Texture transclusion pins `pub:<publication_version_id>` (or
  `rev:` for unpublished targets) plus `cid`. The popover shows "v4 of
  ‹story›". If the target publication's latest is newer, it shows "newer
  version of this story" with the same indicator style. Whether the *block*
  still exists in the newer version can be computed exactly, because a
  `cid` lookup in the latest `doc_hash` either hits or misses. That needs no
  alignment.
- For captures, "newer capture of this page" needs re-captures to exist
  (§0.3) and, for block-level "this paragraph changed", needs D. Both are
  deferred.

---

## 6. Texture tools and prompts

Current surface **[V]**:
- The Texture desk has `ReadDoc`, `ApplyTexture`, Cast, Ask, Note, Precommit
  and the others (`internal/yaegikernel/choir.go:268-271`).
- `ReadCapture` is research-only (`:257-260`, surface `surface.go:61`).
- `ApplyTexture` edits: `update_block_text`, `insert_block`, `append_block`,
  `delete_node`, `insert_source_ref` (with `source_entity_id` or an inline
  `{target:{kind:"web_url",uri}}`), and `mark_source_unused`
  (`surface.go:39`).
- The error text actively steers the model toward inline `web_url` sources
  (`tools_texture.go:1227`).

That steer is why the staging demo's ten sources are all bare
`web_url`/`whole_resource` with empty snapshots (app design §2.2). **[V, from
the doc]**

Proposed surface **[I]**:

| verb / op | shape | notes |
|---|---|---|
| `choir.ReadSource(ref, opts)` | `ref` = capture id, or `texture:<doc>[@ver]`. Returns `{doc, ver(doc_hash), title, origin, outline: [{cid, type, level, text≤N}]}`. `opts.blocks` gives full text for named cids; `opts.around` gives a cid ±k. | Read-only; replaces `ReadCapture` for Texture and management. The world doc's outline has the same shape as `ReadDoc().Outline`, which is the "same model" made visible to the model. |
| `choir.FindInSource(ref, query)` | returns the matching blocks (cid + text) by deterministic term or substring search | Lets the model find the supporting block without paging a whole capture. Deterministic, so no intelligence sits in the runtime. |
| `ReadDoc()` outline | each line gains `cid=` next to `id=` | The prompt rule: **edit by `id`, cite and transclude by `cid`.** |
| op `cite` (replaces `insert_source_ref`) | `{op:"cite", block_id, offset?, source:<ref.doc>, block:<cid>, quote?:"exact words", display:"numbered_ref"\|"expanded_ref"}` | The runtime pins `ver` to the current `doc_hash`, finds or creates the bibliography entity, fills `quote.start/end`, and runs tiers 0 and 1. A quote it cannot find rejects the turn. |
| op `transclude` | `{op:"transclude", after_block_id, source, block:<cid>, quote?, display:"block"\|"pullquote"}` | Inserts a block-level `transclusion` node. Used for Texture-to-Texture and for pull-quotes from captures. |
| op `cite` with `source:{url}` (uncaptured) | allowed, but becomes **pending** | The runtime enqueues a capture (FetchURL → corpusd capture). The anchor resolves later, which is where the iterative design pays off. Retires the bare `web_url` path. |
| `mark_source_unused` | unchanged | — |
| `choir.Updates()` | gains `citation_check` updates (tier 2 failures, pending captures resolved) | Texture decides; a revision follows. |

**Prompt alignment** **[I]**:
- Quote only through `cite.quote` or `transclude`. Copy the exact words from
  a `ReadSource` block.
- Never put quotation marks around words that are not in a cited block.
  Paraphrase without quotation marks and `cite` the block.
- Each factual sentence about the world cites the block it rests on.
- Remove the guidance that recommends inline `web_url` sources.
- Keep the existing distribution rule (refs next to the claims they support,
  `tools_texture.go:1011`).

**Mike parallels to port** **[I]**:
- the `cachedModel` LRU of documents by version id
  (`~/mike/backend/src/lib/documentModel/index.ts:11-33`);
- `blockAt(offsets, pos)` for mapping legacy whole-text quotes onto blocks
  during migration (`citations.verifier.ts:44-51`);
- `evidenceInSource` for checking the judge's excerpts (`:68-72`).

---

## 7. Staged plan

**[I] Letter mapping.** I do not have the earlier study's text. I mapped the
letters by content:
- **B** = per-version permalinks;
- **A** = the unified block model and structured capture ingestion;
- **C** = transclusion and anchored citation of capture blocks, plus
  iterative verification and the Texture tool surface;
- **D** = alignment across re-captures, deferred.

If the earlier study used the letters differently, the stage contents below
still stand; only the labels change.

### Stage B: one publication per document, immutable `/v<n>` permalinks

- **Mutation class: red** (publication and route projection). Protected
  surfaces: `internal/platform/service.go`, `publication_graph.go`,
  `service_publication_read.go`, `internal/proxy/platform_publish.go`.
- **Overlap:** this subsumes W3a step 1
  (`docs/definitions/choir-autopaper-w3a-reader-2026-10-11.md:67-70`) and
  extends it with `/v<n>`, `nav`, and the latest endpoint. Land it *as* W3a
  step 1 rather than as a separate station.
- **Rollback:** `git revert`. Appended versions remain readable as separate
  version objects. The old code ignores ordinals and mints new routes again
  (W3a `:85-87`).
- **Staging acceptance (choir.news):**
  - Publish the same test-computer document at two revisions. There is one
    bare route and `/v1`, `/v2` both resolve.
  - `/v1`'s content hash is unchanged after v2. The bare route serves v2.
  - `nav` is correct in both bundles.
  - A logged-out Playwright spec opens `/v1`, sees "new version available",
    follows it to v2, and the chevrons go back to v1.
  - Artifact: both bundle JSONs, plus the spec trace.

### Stage A: one block model; captures become world documents

- **A1, orange.** `texturedoc` extensions (new block types, `link` mark,
  author-aware validator, `cid`/`h`/`doc_hash` functions), `ExtractBlocks`,
  raw-bytes and `blocks_ref` on captures, and the text→blocks fallback for
  old captures. Touches the capture runtime and Store B. It does **not**
  change Texture canonical writes yet.
  - Rollback: revert. Captures written in the meantime keep their extra
    blobs, which old code ignores.
- **A2, red.** The reducer computes `cid`/`h` on Texture revisions at commit,
  and `ReadDoc().Outline` shows `cid`.
  - Additive attrs: old revisions get them computed on read.
  - Optionally includes the Go port of Mike's lineage carry for content-mode
    and editor writes. It can also follow C.
- **Staging acceptance:**
  - Capture a fixed page under our control (a static fixture served from
    choir.news, so the bytes are stable) twice.
  - Both captures produce identical `doc_hash` and cids. Headings, lists, a
    table and a figure come out as typed blocks.
  - A page with `[12]`-style footnotes validates.
  - `ExtractReadableHTML`'s flat output equals `Project(blocks)` for the
    fixture.
  - A2: a content-mode Texture rewrite that leaves a paragraph's text
    unchanged leaves that paragraph's `cid` unchanged (its `id` still
    changes until the lineage port lands).
  - Artifact: the block JSON and hashes.

### Stage C: cite and transclude blocks; iterative, memoized verification

- **Mutation class: red.** It touches:
  - Texture canonical writes: new ops, the `transclusion` node, `source_ref`
    anchors;
  - Trace/evidence: `choir.citation_check`;
  - publication projection: `resolved_blocks`, per-anchor transclusion rows,
    the checks overlay.

  The tier-2 judge adds provider/model routing, which is also red. Split it
  as **C2**, after C1 = tiers 0 and 1, pending captures, and the tool
  surface.
- **Dependencies:** A (blocks exist), B (versions to overlay), and app-design
  slice C's "FetchURL writes a capture".
- **Rollback:** revert the ops and renderer. Revisions that contain anchors
  stay valid, because anchors are additive attrs on `source_ref`. The old
  renderer falls back to the entity. `transclusion` block nodes would fail
  old validation, so gate the `transclude` op behind a flag until C has
  soaked.
- **Staging acceptance:**
  - A Texture cell on a staging test computer reads a capture with
    `ReadSource`, cites block X with a quote, and transcludes block Y. The
    revision commits.
  - The published v1 shows the quote highlighted inside the source's own
    block, with ✓.
  - A deliberately miscopied quote is rejected in-cell with the nearest
    block text.
  - Edit an unrelated paragraph and publish v2. The verification log shows
    0 recomputations for unchanged anchors (memo hits = anchor count − 1) and
    1 for the changed one.
  - C2: a planted unsupported claim yields `does-not-support`, the Texture
    desk receives an Update, and v3 corrects it.
  - Artifact: per-version check tables with memo-hit counts, plus Playwright
    screenshots of the badges.

### Stage D (deferred): alignment across re-captures

Prerequisites:
- re-captures must exist: change capture identity from `item.ID` to
  `(item.ID, raw_sha256)` for a body change, which is a red-adjacent
  capture-identity change;
- then port Mike's `carryBlockIds` alignment (shared with A2's lineage
  carry), to say "paragraph X changed in the newer capture" and offer
  "re-pin".

Until then, a citation is pinned to the capture it was written against,
which is correct.

### Standing-question notes **[I]**

- **Single state authority:** the check memo, resolved blocks and versions
  live in Store B (corpusd). The guest only pre-checks, using the same
  `quotematch` package.
- **Restart durability:** checks are content-keyed objects. A restart
  re-derives pending work from "anchors without a check" and does not resume
  a job. This fits the crash rule in CLAUDE.md: nothing is re-dispatched
  from a stale queue.
- **Artifact-verified success:** every acceptance above names a JSON or
  Playwright artifact.

---

## 8. Open decisions for the owner [D]

1. §4.4: is verification an overlay on an immutable version (recommended)
   plus Texture-authored correction versions? Or does each verification pass
   mint a version?
2. §5.1: contiguous published ordinals (recommended), or Texture revision
   numbers with gaps?
3. §5.3: should the bare route redirect to `/v<n>`, or serve latest in place
   (recommended)?
4. §1.5: confirm dual identity for Texture blocks: lineage `id` for editing,
   content `cid` for addressing.
5. Tier-2 judge: who runs it (a runtime-owned verifier job or a research
   Cast), and on which model? It must not be the producing desk.
6. Context window around transcluded blocks in public pages (copyright is
   deferred, but the default matters for page size).

## 9. Residual risks

- **The DOM-to-block heuristics** (anonymous text in divs, nested
  layout tables) will need tuning across outlets. Mitigation: fixture pages
  per outlet in the acceptance set; keep `extractor` versioned so tuning
  never silently re-addresses old citations.
- **Duplicate ordinals** are stable only inside a pinned version. That is
  enough for C and is a known limit for D.
- **Tier 0 and tier 1 at commit require the guest to read blocks from
  corpusd** during the reducer. That adds a network dependency to a red
  path. A corpusd outage must reject the turn cleanly, never hang it.
  Fate-sharing: give the read a short timeout, and on failure accept the
  anchor as `pending` instead of rejecting.
- **The O(n²) history embedding** in manifests (§5.2.4) becomes acute once B
  lands. Fix it in B.
