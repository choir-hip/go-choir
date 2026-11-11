# Golden documents for the Texture page format

Autoradio pass-1 acceptance: "run the golden documents through both renderers;
proves web and native draw the same page format". These files are the page
format's shared test. They are chosen from the format as `schema.go` and
`embed.go` define it, not from what the native renderer happens to draw: a
feature the schema allows is listed below as covered or as not covered.

Each file is a publication bundle, the shape of
`GET /api/platform/publications/resolve`, plus a `golden` header
(`name`, `covers`, `checks`, `note`). Both renderers read the bundle as the
server sends it.

## How they are checked

| Piece | File |
| --- | --- |
| Every `body_doc` validates against the schema | `../golden_test.go` (`go test ./internal/texturedoc -run Golden`) |
| Web: the real Texture reader draws the bundle, the DOM is projected | `frontend/tests/golden-documents.spec.js` |
| Native: `TextureDocParser.parseBundle` builds the phone's `ReaderDoc`, projected | `mobile/AutoradioKit/Tests/AutoradioKitTests/GoldenDocumentsTests.swift` |
| Diff of the two, known differences, exit code | `scripts/golden-docs`, `scripts/golden-docs-compare.mjs`, `known-differences.json` |

Run all of it with `scripts/golden-docs`. The native half needs only
`swift test` in `mobile/AutoradioKit` (no Exact app build, no simulator); on
the shared Mac it runs through `shared-proof.sh <branch> proof-golden.sh`, and
`GOLDEN_NATIVE_DIR=<proof dir> scripts/golden-docs` then compares.

### The projection (one definition, two implementations)

An ordered list of blocks. Each block has `kind` and `text` and, where they
apply:

- `heading` (`level`, and `depth` when inside a blockquote), `paragraph`,
  `list_item` (`depth` of list nesting from 1, `marker` = `bullet` or `N.`),
  `quote` (`depth`), `code` (raw text, trailing newlines trimmed), `rule`,
  `table` (`rows` of cell text), `embed` (`embed` = the embed kind, `text` =
  its caption or title).
- `text`: whitespace-collapsed; a citation is the token `[n]`; a hard break is
  a space.
- `spans`: the styled text (`strong`, `emphasis`, `code`, joined with `+`), merged
  when adjacent.
- `cites`: `n`, `title`, `quote` (and `check` for documents with `checks: true`,
  where the bundle carries a manifest and live checks).

### What is compared, and what is not

Web layer: the DOM the Texture reader produces (`renderStructuredTextureDocHTML`
or the Markdown renderer, in the published reader at phone width). Native
layer: the `ReaderDoc` model that `TextureReaderSource` hands to the Exact
Contract (`reader.contract`); the Contract is not run. Not compared: pixel
look, fonts, colours, spacing, motion, the page chrome (headline, dek,
byline, AI disclosure, Sources sheet, version list, "More on this"), what a
tap or hover does, and the Exact `reader.contract` drawing itself. A native
renderer whose model is right and whose Contract draws it wrongly would pass
this check; the simulator screenshots in `mobile/scripts/proof-lane3.sh` are
the only look at that.

## Documents and what they cover

| File | Covers |
| --- | --- |
| `headings-and-marks.json` | paragraph; heading levels 1 to 6; text; strong, emphasis, strong+emphasis, code marks; hard_break; text with runs of spaces |
| `lists.json` | bullet_list; ordered_list; ordered_list `start`; nested lists; a list item with two blocks; marks in an item |
| `quotes-code-rule.json` | blockquote; two paragraphs in a quote; nested blockquote; a heading inside a quote; code_block with indentation and blank lines; code_block with two text nodes; horizontal_rule |
| `citations.json` | source_ref numbered_ref and expanded_ref; `ref.quote`; one source cited again; adjacent cites of one source; a cite in a heading; entity list order different from reading order |
| `citations-published.json` | source_ref with manifest anchors, resolved blocks and live checks (exact, failed, tolerant, pending); adjacent cites of one source |
| `transclusion.json` | texture_span entity (a Texture cited as a source); expanded_ref with a quote; web_url beside it |
| `embeds.json` | embed image, audio, video, file, capsule_evidence |
| `non-latin.json` | Cyrillic, Japanese (no spaces), Arabic (right to left), Devanagari, Korean, combining marks, a ZWJ emoji sequence, a flag; a cite after non-Latin text; non-Latin inside a quote |
| `long-text.json` | a paragraph over 20 000 characters (past the native sentence-split limit); sixty short paragraphs |
| `markdown-fallback.json` | a bundle with no `body_doc`, only Markdown `content`: heading, marks, nested list, ordered list, quote, table, fenced code, rule |
| `edge-shapes.json` | empty paragraph; whitespace-only paragraph; hard_break at both ends of a paragraph; first heading equal to the title; literal `**` in stored text |

## Format features not covered, and why

- **A reader's fork and comments.** The Texture page format defines neither
  (no fork, comment or annotation node in `schema.go`, in the web reader or in
  `TextureDoc.swift`). They are a product idea, not a format feature, so no
  golden document can cover them yet. When the format gains them, add a
  document here first.
- **The audio timeline over block ids** (handoff section 0.1 says the page
  format includes it). `internal/texturedoc` holds no timeline type yet; the
  `audio` embed (an audio file the desk cites) is covered, the narration
  timeline is not.
- **`link` marks.** The schema allows them only in a world document
  (`choir.world_doc.v1`, a capture's blocks), never in a Texture page, so a
  Texture golden document cannot carry one. Native draws link marks and web
  drops them; that is not in the Texture page format and is not compared.
- **Table nodes in a `body_doc`.** The native parser reads them; the schema
  refuses them. Tables are covered through the Markdown fallback only.
- **Source entity kinds beyond `web_url` and `texture_span`** (pdf, audio,
  video, transcript, diff_hunk and the rest of `validSourceTargetKind`), and
  the selector kinds. The renderers show a title and a host or a Texture
  address; they do not branch on the other kinds.
- **Unknown block, inline and embed kinds from a newer schema.** The writers
  refuse them, so a valid document cannot hold one; both renderers tolerate
  them by design. Not covered because the validator would refuse the file.
- **Embeds in a published page.** Publication refuses a document holding an
  embed today. `embeds.json` is served as a bundle because both renderers draw
  embeds from a `body_doc`; the compared path is the body renderer, not
  publication.
- **Private revisions.** The phone reads a private revision through
  `parseRevision`; the golden bundles use `parseBundle`. The body path
  (`build`) is shared, so block and inline drawing is covered; revision
  versions and authorship are not.

## Differences the first run found

`known-differences.json` lists each one with the renderer that departs and a
reason. The comparison fails on any difference not listed and on any listed
difference that stops appearing, so the list is a worklist, not a tolerance.
Fixing a renderer means deleting its entries. The first run (24 entries in 9 of
11 documents) found: web ignores `ordered_list` `start`, concatenates the
paragraphs of a list item, flattens nested Markdown lists and draws `---` after
a code fence as text; native numbers unpublished citations by first use where
web uses the entity list order, ignores a citation's own `ref.quote`, shows the
check outcome word ("exact") as the quote of a checked citation, draws an
embed's alt instead of its caption, raises a heading in a quote to level 3 and
joins a code block's text nodes with nothing. Identical on both sides:
`headings-and-marks`, `long-text`, `non-latin`.
