# C8 quote census: do research's quotes land in the pages they cite?

Written 2026-10-13 for the Cut 2 pre-flight, item C8 in
`docs/archive/reports/reeng-plan-review-2026-10-13.md` ("quote-in-block census now;
report missing quotes, raw bytes and block structure separately"). Cut 2's
conjecture C2-1 has a `delta_o` asking what share of research-reported quotes
land inside one extracted block of the page they cite
(`docs/archive/definitions/choir-reeng-cut2-evidence-to-publication-2026-10-12.md`).
This census is offline and read-only. It cannot measure that rate, because
the future extractor does not exist and no bundle holds a page's HTML. It
measures what today's data can show, and says where the gaps are.

Mutation class: green (report only). No source code changed. Analysis
scripts were kept outside the repo, and no bundle content was copied in.

## Data

The data is the tracer trace bundles under `~/.config/choir-qa/traces/` (the
owner's disposable test computers, outside the repo). There are 8 bundle
files: `bundle-a1.json` for `c0-accept-1`, `c0-fast-1`, `c0-tracer-dev-8`,
`c1-l1-1`, `c1-l1-crash-2`, `c1-l3a-story-1` and `c1-l3b-mgmt-1`, plus
`bundle.json` for `c0-ttl-20261012-2330`, where every layer is unserved.

Only four bundles hold research packets with sources: `c0-accept-1` (6
packets), `c0-fast-1` (2), `c0-tracer-dev-8` (7) and `c1-l3b-mgmt-1` (1).
The Cut 1 story run `c1-l3a-story-1` has three research reports, but none
has a `sources` array. `c1-l1-1` and `c1-l1-crash-2` have no sourced
packets.

Where things live:

- **Quotes.** These are `packet.sources[].selectors[]` entries with
  `kind: text_quote`. Cut 0 bundles carry them in `trajectory.updates[].packet`;
  `deliveries` is a copy of the same updates. Cut 1 bundles carry them in
  `records[kind=report].prediction.hypothesis`, which holds the whole packet
  as a JSON string. Packets were deduplicated by their sources.
- **Page content.** There is no fetched page body in any structured layer.
  `egress` gives verbs and timings only (`web_search`, `import_url_content`,
  `read_content_item`, `read_content_item_selector`), and `search_arrivals`
  is unserved. Page text appears only where a desk cell printed it to stdout
  (`run_events` `tool.result` output), and the agent's own code often cut it
  to between 900 and 6,000 characters or filtered it line by line. Text was
  tied to a URL in three ways: the platform's JSON (`canonical_url` /
  `content_id` plus `text_content` or chunk text); content ids named in the
  cell's source and mapped back to their import; and tag-to-URL tables in the
  cell's source.

## Method

There are 210 `text_quote` selectors in total. Five cite `urn:choir:...`
pseudo-sources (search batches, not pages) and are left out, which leaves
**205 quotes that cite a URL**. All 210 have a quote: none are missing or
empty.

Each quote was matched against the page text visible for its URL. Before
comparing, the scheme, `www.` and any trailing slash were dropped from URLs.
Matching uses a port of mike's `locateQuote` cascade
(`~/mike/backend/src/modules/chat/engine/verifyCitations.ts`), tried in this
order:

1. exact substring;
2. whitespace collapsed and lowercased;
3. tier 2 plus punctuation removed (anything that is not a letter, digit or
   space).

As in mike, a quote containing `...` or `…` is split into segments, and every
segment must match. To keep a quote from matching its own echo, stdout from
cells that echo packets (`Updates`, `Inbox`, `Records`, `ReportPacket`) was
excluded. For pages with no text, two further checks look for the quote: in
search-cell output within about 1.5k characters of the URL, and anywhere in
content-cell output.

A "unit" is one line of the matched text, and a "block" is a run of lines
between blank lines.

## Results

### Can each quote be judged? (205 URL quotes)

| class | n | share |
|---|---|---|
| (a) quote missing or empty | 0 | 0% |
| (b) no page text in the bundle for that URL | 94 | 46% |
| — never imported or read; quote found in search output near the URL | 48 | 23% |
| — never imported or read; quote found elsewhere in content output | 3 | 1% |
| — never imported or read; quote not visible anywhere | 26 | 13% |
| — imported, but no text for it was printed | 17 | 8% |
| (c) page text present | 111 | 54% |

### Match tier when page text is present (111)

| tier | n | share of (c) |
|---|---|---|
| exact | 52 | 47% |
| whitespace and case | 12 | 11% |
| punctuation-tolerant | 15 | 14% |
| no match, and only part of the page text is visible | 24 | 22% |
| no match, and the whole page text is visible | 8 | 7% |

The judgeable subset is every match plus the misses where the whole page is
visible. Of those 87 quotes, 79 matched (91%) and 52 matched exactly (60%).
Restricted to page text in the platform's own JSON (`text_content` and chunk
text, 46 matches), the split is 35 exact, 5 whitespace and 6 punctuation.

Why the tolerant tiers fired:

- **Whitespace tier (all 12):** the quote crosses a line break in the
  flattened text, or the page has doubled spaces.
- **Punctuation tier (15):** 4 are curly versus straight quotes or dashes,
  3 are ellipsis quotes, and 8 are other punctuation drift, such as
  markdown link brackets.

29 of the 111 quotes in (c) contain an ellipsis.

The 8 true misses, and the partial-text misses that could be read, fall into
recognisable kinds:

- **Paraphrase presented as a quote.** The model wrote its own sentence (for
  example "Meta announced its ... models on October 12, 2026, describing")
  and labelled it a quote.
- **Rows the model built from a table.** All six `en.wikipedia.org` quotes are
  rows of the form `City | Country | population | date`, joined with pipes
  by the model. The page holds no such string.
- **Markup copied from a search snippet.** Some quotes start with `<strong>`
  or `## Most viewed`, which comes from the search provider's snippet, not
  from the reader text.
- **Ellipsis joins** whose segments do not all appear on the page.

### Unit structure of the 79 matches

| text source | matches | inside one line |
|---|---|---|
| platform `text_content` (reader markdown made from HTML) | 44 | 39 (89%) |
| platform chunk text (`metadata.selectors[].text`) | 2 | 0 |
| text the desk printed itself (filtered or joined by agent code) | 33 | 19 |
| all | 79 | 58 (73%) |

The platform's reader text (`readability_lite`, `text/markdown` with
`original_media_type: text/html`) is flattened: one line per element and
**no blank lines anywhere**. Splitting on blank lines therefore yields one
block per page, and the "inside one block" test passes for every match, which
tells us nothing. The only structure left to measure is per line. The 89% is
an upper-bound proxy for "inside one block": it was measured on the old
flattener's lines, not on extracted DOM blocks.

### Raw bytes (separately, as C8 asks)

There are 118 distinct (bundle, URL) pairs cited. 41 have import metadata in
some printed output: 30 report an original `text/html`, 1 a PDF, and 10 do
not say. 31 record a `direct_http` byte count (median about 300 KB) and a
`raw_content_hash`. **No bundle contains a page's HTML**: there are no
`<html`, `<body`, `<p` or `<div` strings, raw or JSON-escaped. The only tags
present are `<strong>` highlights inside search-provider snippets. This fits the Cut 2 premise that
captures keep one flat-text body and drop the raw bytes
(`internal/sourcegraph/captures.go:77-84`).

### By bundle (URL quotes)

| bundle | quotes | (b) no page text | exact | whitespace and case | punctuation | no match |
|---|---|---|---|---|---|---|
| c0-accept-1 | 118 | 62 | 17 | 5 | 11 | 23 |
| c0-fast-1 | 20 | 15 | 2 | 2 | 1 | 0 |
| c0-tracer-dev-8 | 47 | 15 | 22 | 4 | 3 | 3 |
| c1-l3b-mgmt-1 | 20 | 2 | 11 | 1 | 0 | 6 |

### By domain (top 13 of 84 cited domains; 19 domains have any match)

| domain | quotes | (b) no page text | matched | no match |
|---|---|---|---|---|
| techcrunch.com | 17 | 4 | 12 | 1 |
| about.fb.com | 13 | 0 | 11 | 2 |
| artificialanalysis.ai | 12 | 3 | 9 | 0 |
| vals.ai | 11 | 4 | 6 | 1 |
| research.meta.ai | 9 | 3 | 0 | 6 |
| reflection.ai | 7 | 3 | 3 | 1 |
| un.org (PDF) | 7 | 0 | 7 | 0 |
| blog.google | 6 | 0 | 4 | 2 |
| en.wikipedia.org | 6 | 0 | 0 | 6 |
| the-decoder.com | 5 | 2 | 3 | 0 |
| aleph-alpha.com | 4 | 0 | 4 | 0 |
| dev.to | 4 | 4 | 0 | 0 |
| mistral.ai | 4 | 2 | 0 | 2 |

The long tail is mostly single quotes from pages research only saw as search
results.

## Limitations

- **Small and skewed sample.** Four runs, mostly one Cut 0 story prompt about
  AI news, plus one city-population run. 205 quote instances, 195 of them
  unique. Instances are not independent.
- **Visible text is not the capture.** Page text is whatever the desk
  printed, often cut short or filtered by the agent's own code. "No page text"
  means none was visible in the bundle, not that none was captured. "No
  match, only part of the text visible" cannot be judged.
- **Loose attribution of printed text.** When a cell read several content ids
  and printed plain text, the whole stdout is credited to each id. This can
  over-credit a match, and it is why the desk-printed rows are separated
  from the platform-text rows.
- **Search-proximity heuristic.** "Found in search output near the URL" uses
  a character window, not a parsed result. It can credit a neighbouring
  result.
- **No DOM.** Nothing here measures the future extractor. The per-line rate
  comes from the old flattener, so it is a proxy only.

## What this implies for Cut 2

1. **`delta_o` stays unmeasured.** It needs staging captures that keep raw
   bytes, which 2a provides. The trace bundle should gain a captures layer
   (content id, URL, raw hash, extracted blocks) so this census can be rerun
   unchanged once the extractor lands.
2. **About half the quotes cite pages research never captured.** 23% appear
   to come from search snippets. The matcher needs a verdict for "not
   captured" that is separate from "not found". Otherwise roughly half of all
   citations would read as mismatches. Prompting research to capture before
   quoting would raise the judgeable share.
3. **The cascade earns its keep.** Exact matching finds 52 of the 79
   matches. The tolerant tiers add 27 (34%), and the whitespace tier is
   almost entirely line-break crossings. The port must keep mike's ellipsis
   splitting, since 29 of 111 quotes use an ellipsis.
4. **"Inside one block" needs a rule for quotes that cross blocks.** On
   today's lines, 11% of platform-text matches span lines. Some of these will
   span DOM blocks too. Ellipsis segments may sit in different blocks.
5. **Tables need a decision.** Quotes the model builds from table rows
   (pipe-joined cells) cannot match any linear text. The extractor needs
   table-row blocks with a canonical cell join, or the matcher needs to match
   cell by cell, or research must quote one cell at a time.
6. **Paraphrase and snippet markup are true failures.** These are the cases
   the support judgement should catch. They are a small share of the
   judgeable quotes (8 of 87), but they are real.
