# Capture full text: BM25 over the passages of every captured page

Reference page (convention: [README.md](README.md)); written 2026-11-18 in the
lane `capture-fulltext`. Built and tested in the lane, **not deployed**. Every
figure is marked **measured** (a command we ran, named), **code** (read in the
source, file named) or **est** (our arithmetic from marked inputs). Text found
in files was treated as data.

The owner, 2026-11-18, after the audit found 207,000 captured pages
searchable by their titles and labels only
([problem](../problems/research-does-not-start-from-the-corpus-2026-11-18.md)):
"we should be thinking resource aware, but I think we should be able to do
hybrid vectors and BM25 and graph traversal across entities and events. And
time windowing... we should be doing full page indexing for both BM25 and for
vectors... But yes, we should be doing full page indexing for everything with
BM25 immediately as well." This page is the BM25 half, with time windowing,
built so the vector half sits beside it on the same passages (section 5).

## 1. Facts, the measurement and the choice

### 1.1 What text exists per capture

**code.** A capture is a `choir.web_capture` object in Store B (`og_objects`)
with three blobs on corpusd's host disk, `capture-bodies/<2 hex>/<sha256>`
(`internal/platform/captures.go`, `internal/sourcegraph/captures.go:134`
`WriteCaptures`): the raw bytes as fetched (`ContentBlobID`; a page or a PDF,
at most 16 MiB), the block JSON (`BlocksBlobID`) and the **text blob**
(`ExtractedTextBlobID`), which is `sourcefetch.Projection(blocks)`: the
blocks' clean text joined by a blank line. Extraction is not in corpusd. It
runs in the guest before the post: `researchtools/read.go` (`ExtractBlocks`
for a page, `content/pdf_blocks.go` for a PDF) and `sources` for a feed
item's body (`TextBlocks`); corpusd stores what it is sent. So clean text
exists for every capture, in one blob, and the index reads that blob; no HTML
or PDF is parsed by the index. A capture written before "Cut 2" has one
flat-text blob named by both fields; `ExtractedTextBlobID` is the text in both
shapes (`HandleCaptureRead` says so), so the index handles both. A capture
with no text blob, or an unreadable one, is skipped and counted.

**Size, assumed.** Neither the code, the docs nor the tests hold a
distribution of text size, and production data was not read. Known: a feed
item's body is cut at 1 MiB before extraction (`maxCaptureBodyBytes`), raw
bytes at 16 MiB; the host holds 154,649 body files in 9.37 GB, 60 KB a file
on average, across raw HTML, block JSON and text (object-storage design,
2026-10-29), which says raw HTML dominates and nothing about the text.
**Assumed** (and built into the benchmark corpus): 30% of pages are short feed
items (median 80 words), 69% articles (median 1,250 words), 1% long documents
(median 15,000 words); mean 1,300 tokens, 7.8 KB at 6 bytes a token. The
query that gives the real distribution, read-only, on Node B:

```sh
psql graph -Atc "select substr(metadata::json->>'extracted_text_blob_id',8)
  from og_objects where object_kind='choir.web_capture'" |
 while read h; do stat -c %s /var/lib/go-choir/platform-artifacts/capture-bodies/${h:0:2}/$h; done |
 sort -n | awk '{a[NR]=$1; s+=$1} END {print NR, "files; mean", s/NR; split("50 90 99",q," ");
   for (i=1;i<=3;i++) print "p" q[i], a[int(NR*q[i]/100)]}'
```

If the mean is far from 8 KB, every size and time below scales with it
(index bytes and indexing time are linear in tokens).

### 1.2 The four engines, measured

**Method.** `internal/capturefts/ftsbench` (a test, rerunnable: `CHOIR_FTS_BENCH_DOCS=20000 go test ./internal/capturefts/ftsbench -run TestBench -v`)
builds a synthetic corpus of invented words (Zipf, 150,000-word vocabulary,
exponent 1.07; titles of 6 to 11 words) shaped as in 1.1, and runs 200
queries of 4 words and 200 of 12 words, each drawn from a random page's body,
with the 50 most frequent words removed (the stop words a query drops), and
100 12-word queries that keep them ("raw"). Every number is a p50 / p95 of
those. **The build VM is shared and was busy** (load average 6 to 36 on 8
vCPU, 3.9 GB, other builds; two runs were killed by the memory manager), so
timings are good to a factor of two and the Postgres rows are from 2,000
pages (a 20,000-page run was killed). Test Postgres runs with fsync off, which
flatters its inserts. 20,000 pages = 26.2 million tokens.

| Engine | Disk (20k pages) | Memory | Build | Query 4 words | Query 12 words | 12 words with common words | Insert / page | After a restart |
|---|---|---|---|---|---|---|---|---|
| **(a) SQLite FTS5, passages** (82k to 99k rows) | 118 to 124 MiB | page cache, set (below) | 56 s (30 s idle) | 67 / 157 ms | 196 / 271 ms; 31 / 61 ms with common terms pruned | 207 / 292 ms (pruned: 19 / 45) | 4.2 ms (6.6 loaded) | reopen file; first query 0.5 s |
| (a') SQLite FTS5, whole pages (20k rows) | 98 MiB | same | 23 s | 24 / 47 ms | 61 / 93 ms | 321 / 484 ms | 2 ms | reopen; first query 40 ms |
| (b) PostgreSQL `tsvector` + GIN, pages (**2k pages**) | 28 MiB per 2k = 14 KB a page | server (shared buffers 512 MB) | 3.2 s per 2k | 5.8 / 9.5 ms | 9.7 / 13.6 ms (rises with matching rows) | 12 / 15 ms | 1.4 ms | nothing to do; but `ts_rank` is not BM25 |
| (c) term table with frequencies, pages (**2k pages**) | 93 MiB per 2k = 46 KB a page | server | 5.7 s per 2k | 10 / 24 ms | 26 / 46 ms | 74 / 104 ms | 5 ms | nothing to do |
| (d) in-memory inverted index, pages | 0 | **133 MiB heap** (5.3 bytes a token) | 4.8 s, plus reading all text | 3.4 / 6.1 ms | 5.8 / 8.6 ms | 9 / 12 ms | 0.33 ms | **rebuild from all text** |

With the production code (`TestProductionPath`: the real `Put`, tokenizer and
`Search`) the passage index had 82,321 passages for 20,000 pages (4.1 a page),
a size of 124 MiB and 4.2 ms to index a page. Windowed to the newest tenth,
a 12-word query ran 75 / 122 ms; in recency order 609 / 847 ms (that run ordered every matching passage by time; the repaired search takes candidates by score, see section 2). The scan budget (100,000 postings) is 8% of the 1.25 million passages of a
250,000-page index; to see what it does there, the same corpus was run with a
budget of 8,000 (`CHOIR_FTS_BENCH_BUDGET=8000`), 9.7% of its 82,321 passages (a little above the 8%, so a little slower than the larger index would be): a
12-word query ran **80 / 112 ms**, 12 words with the common ones kept 66 /
132 ms, windowed to the newest tenth 35 / 53 ms, in recency order 97 / 135 ms,
4 words 57 / 204 ms (indexing was 10 ms a page in that run, the VM being busy
again). This is the evidence for the claim that a search stays near 100 ms and
under half a second at 250,000 pages; at a million pages it is an
**extrapolation**, not a measurement: the budget bounds the postings read,
but the index no longer fits the machine's page cache, and nothing was run
at that size.

What decided it:

- **(d) is the fastest and cannot be the answer.** It holds 133 MiB for 20,000
  pages: 1.66 GiB at 250,000 pages and 6.6 GiB at a million, in corpusd's
  heap, growing with the corpus ("nothing unbounded in memory"), and a restart
  re-reads and re-tokenizes every page's text from disk (about 2 GB at 250,000
  pages) before the first search is whole. The published-pieces index can do
  this because 8,000 stories are small.
- **(c) is out on size and on shape:** 46 KB a page is 11.6 GB at 250,000
  pages and 46 GB at a million, in the 4 GiB-capped PostgreSQL beside an
  18 GB control database, and it needs the BM25 arithmetic written in SQL.
- **(b) fits the database the data is in but is not BM25.** `ts_rank` has no
  inverse document frequency and a crude length normalisation (rank is
  computed per matching row from the stored `tsvector`, so cost rises with the
  matches); a title/body weight exists (`setweight`), saturation does not.
  What that costs: the ranking the owner asked for ("BM25") is not what is
  delivered, a rare word does not outrank a common one, and a long page that
  repeats a word outranks the page that answers. The `tsvector` also stores
  every position: 14 KB a page, 3.4 GB at 250,000 pages and 13.7 GB at a
  million, written to the WAL and the cap's page cache on every capture, and a
  `tsvector` is limited to 1 MiB (a 2 MB page raises an error and needs a cut
  anyway). Its latency is the best of the on-disk engines.
- **(a) is BM25 as the owner asked, on disk, in a file corpusd owns,** with
  memory a configuration (below) rather than a function of the corpus, restart
  free of rebuilding, and Postgres untouched. Its weakness is measured: FTS5
  scores every row a query matches, so a 12-word question that keeps common
  words scans most of the index (1 second at 99,000 passages). That is
  handled in the query, not in the engine: stop words are dropped, and a
  scan budget keeps the rarest terms whose postings fit 100,000 and drops the
  rest (section 2). The lead's leaning was (a); the numbers agree with it.

**Memory of (a), configured.** Each open file keeps at most 3 connections of
8 MiB of SQLite page cache (`cache_size`, a limit on the cache and not on
SQLite's other allocations: sorter, statements); the code enforces at most 4
open files, evicting on release as well as on open. That bounds the page cache
at 96 MiB, about 24 MiB while only the paper's computer is searched; the
process's total is that plus working memory, which no code here limits but
which does not grow with the corpus. The document-
frequency cache is at most 50,000 small entries per file.

### 1.3 The unit: passages, not pages

Passages of about 300 tokens (1,800 runes; section 2), each a row with the
capture id, ordinal and byte offsets, the page's title carried by the FTS row
as a second column. The measured costs of passages against whole pages for
the same engine and 20,000 pages: **disk +20 to 27%** (118 to 124 against 98
MiB: the postings carry positions in both, the extra is rows), **query 2 to
4 times slower** (82k to 99k rows against 20k; 196 against 61 ms for 12
words), **insert 2 times** (4.2 against 2 ms). Why that is paid:

- BM25's length normalisation works on the unit. A fact in one paragraph of a
  30,000-word document is a short passage that matches; as a page it is a
  rare term in a long document and loses to a short page that mentions the
  word in passing. Finding the page that holds a fact is the job.
- The snippet is the passage that matched, found, not guessed afterwards by
  reading the page and re-scoring it.
- The vector index needs a passage table anyway. Whole-page BM25 would make
  that table a second structure that nothing keeps in step; here the same row
  is the BM25 posting's key and the embedding's key (section 5).

### 1.4 Choice

SQLite FTS5, passages, one file per computer, owned by corpusd; no change to
PostgreSQL, `nix/` or the 4 GiB cap.

## 2. What is built

**Scope and files.** One SQLite file per computer (an owner's computer, the
pair `owner_id`, `computer_id`), in `<artifacts root>/capture-fts/<sha256 of
the pair, 32 hex>.db`, owned by corpusd. The collector sweeps named
namespaces only, so the directory is outside it; no `nix/` change is needed
(the directory is under corpusd's `StateDirectory`). A search names its
scope and opens only that file, so one computer's pages cannot appear in
another's search by construction; the capture search route's guard
(`requireInternalOrBoundGuest`: a bound guest names exactly its own owner and
computer) is unchanged and still the first check, and every hit's capture
object is re-read from Store B under the request's owner and computer.

**Tables** (`internal/capturefts/index.go`):

| Table | Columns | Purpose |
|---|---|---|
| `pages` | `capture_id` PK, `title`, `text_hash`, `fetched_at`, `published_at`, `eff`, `passages`, `text_bytes`, `truncated`, `indexed_at` | one row per indexed capture; `text_hash` names the text blob the offsets refer to; `eff` is `published_at` when known, else `fetched_at` |
| `passages` | `id` INTEGER PK AUTOINCREMENT, `capture_id`, `ordinal`, `start_byte`, `end_byte`, `eff` | one row per passage; `id` is never reused |
| `passages_fts` | FTS5 over `title`, `body`; `content=''`, `contentless_delete=1`, `detail=full`, tokenizer `unicode61 remove_diacritics 2` | the postings; rowid = `passages.id`; stores no text |

No page text is stored. A passage is a byte range of the text blob its page
names (`CaptureBodyStore.Range`), so the snippet, the embedding input and the
text the index saw are the same bytes.

**Tokens** (`capturefts.Terms`): the capture term table's tokenizer
(`captureterms.Tokens`: lowercase runs of letters and digits; Han, kana, Thai,
Lao, Khmer, Myanmar, Arabic and Hebrew as character unigrams and bigrams),
the published-pieces index's plural fold (`captureterms.PluralFold`, moved
there from `publication_pieces_index.go`), NFC, and accents folded on Latin
words. The same function tokenizes pages, titles and queries, and the tokens
are handed to FTS5 already made, so the engine's tokenizer only splits on
spaces. Query stop words (about 90 function words: `a`, `the`, `of`, `was`, `latest`, `says` and so on) are dropped from the query and stay in the index.

**Passages** (`capturefts.Passages`): the text's blank-line-separated blocks
(the capture projection joins blocks with a blank line) packed to about 1,800
runes (about 300 English tokens), a block over 2,400 runes split at its last
sentence end or space in the final quarter of an 1,800-rune window. At most
the first 256 KiB (`MaxIndexedBytes`) of a page's text is indexed, about 140
passages: the capture route already cuts a feed item at 1 MiB, a document that
long is a few percent of pages, and its subject is in its first 256 KiB; the
page is marked `truncated`. A page with no text still gets one empty passage,
so its title is searched.

**Ranking.** BM25 as FTS5 computes it (k1 1.2, b 0.75: the published-pieces
index's constants), over passages, with column weights title 3.0 and body 1.0
(`TitleWeight`; the pieces index counts a title word three times). A page
stands for its best passage; its score is that passage's score. Order is
`relevance` (score, then newest, then capture id) or `recent` (time, then
score, then capture id): both total orders, so repeatable. `since` is inclusive
and `until` exclusive, on the page's published time when known, else its
fetched time.

**What a query may say** (`capturefts.PlanQuery`). Words are searched, any of
them, stop words dropped. A "quoted phrase" is searched as a phrase (its tokens
adjacent, in order); when no passage holds it, its words are searched instead
and the result says so (`phrase_fallbacks`). `-word` and `-"a phrase"` are
exclusions: never searched for, and a passage that holds one is not returned
(at the passage, not the page: another passage of the same page may still be).
Everything else is punctuation, and nothing of the text reaches FTS5 as syntax:
`C++` is searched as `c`, `AT&T` as `t` (`at` is a stop word), `NEAR/3` as
`near` and `3`, `title:` as the word `title`. The tokenizer is the shared one
(it also serves the published-stories index) and was left alone; write the
distinctive word beside such a term. A word with nothing after the colon
(`order:`) is the word. A query with no searchable word answers with a note,
not as a search with no hits.

**Pruning and the cap.** The cost of an OR query is a scan of every passage
that holds any of its terms, and FTS5 has no early exit for `ORDER BY bm25`.
So the terms are ranked by rarity (document frequency, counted once, up to the
scan budget, and cached for an hour: only its size matters). Terms in no
passage are pruned; the rarest are kept while their postings sum to at most
100,000 (`DefaultScanBudget`), always keeping the three rarest, and the rest
are pruned; then, of those kept, the 32 rarest are searched and the most
common are capped away. A question keeps the words that name its subject and
loses "week" and "decided" on a large index. A small index prunes nothing.
The result names the pruned, capped, excluded and phrase-fallback words
(`pruned`, `term_capped`, `excluded`, `phrase_fallbacks`), and the tool's note
and `capture_words_dropped` carry them to the model.

**Candidates.** Passages are taken by score, whatever the order: at most 300
candidate passages, at most 3 (`MaxPassagesPerPage`) from any one page, so a
long page cannot take the slots of the pages that answer the question. The
order is then applied to the candidates: `relevance` by score; `recent` by
time among the pages at least half as relevant as the best one (so the newest
page that says one common word does not lead).

**Write path.** `Store.indexCaptures` (called by `PutObject`, `PutBatch` and
`InsertObjectIfAbsent`) writes the text after the projection: read the text
blob (at most 256 KiB), tokenize, one transaction per page. A tombstoned or
superseded capture is removed, `DeleteObject` removes its page. A failed write
is counted, and logged at most once in 30 seconds with the count, the number
suppressed and the first capture id (`corpusd capture fulltext write failed
count= suppressed= capture=`); it sets a dirty flag, marks the index not
complete, and asks for a catch-up at most once in 5 minutes; the capture is
stored regardless. A capture whose record carries an owner-private label is
not indexed, is removed by the catch-up if it was, and is never returned (the
capture read gate would refuse its body to a non-custodian); the research
tool's result, which carries a passage, is labelled `owner_private:capture`
beside `world_untrusted` (`agentcore/run_labels.go`), the floor publish-auto
admission keys on.

**Search path.** `POST /internal/platform/captures/search` takes `query`,
`since`, `until`, `order`, `limit` beside the old `words`, `need`, `ids`. The
answer adds `hits` (capture id, title, score, passage id and ordinal, byte
range, text hash, snippet of at most 420 runes, times, whether the title and
the passage matched), `hit_objects` (the capture objects, no body) and
`fulltext` (used or why not, pages indexed, captures the computer has,
terms). Deadline 8 s; a failure is reported in `fulltext` and the old half
still answers.

**Guest merge** (`researchtools.searchCapturesFor`): the text half and the old
title-and-label half are merged by reciprocal rank (k 60), or by time for
`recent`; a capture both found rises and carries both halves' evidence
(`matched`: `title`, `body`, `topic`, `outlet`, `language`) and the passage.
Each result has what research needs: `capture_id` (citable, `ReadCapture` it),
`url`, `title`, times, and `passage` with its ordinal and the page's passage
count. The tool takes `since`, `until`, `order`, or the same as words in the
query (`since:2026-11-10 order:recent`), which is how a cell that passes only
a query reaches them. The window also filters the old half. The note says how
much of the computer's text is searchable so far. The journal line adds
`text_index`, `text_indexed`, `computer_pages`, `by_body`, `by_title`,
`by_both`, `by_labels_only`, and still only capture ids.

## 3. Resources at 250,000 and a million pages

Inputs: 20,000 synthetic pages of 1,308 tokens each (1.1), the production
code, the busy build VM. **Disk** is measured per page and scaled; **memory**
is a configured bound; **CPU** is measured per page and per search.

| | 20,000 pages (measured) | 207,000 pages (the paper today, est) | 250,000 pages (est) | 1,000,000 pages (est) |
|---|---|---|---|---|
| Passages | 82,321 | 0.85 M | 1.03 M | 4.1 M |
| Index on disk | 124 MiB (6.35 KB a page, WAL included) | 1.28 GiB | 1.55 GiB | 6.2 GiB |
| For scale: the pages' text blobs | 112 MiB | 1.6 GB | 1.9 GB | 7.8 GB |
| Page cache of the index | at most 96 MiB (a limit on the cache, section 1.2) | same | same | same |
| Indexing one captured page (inline, in the capture write) | 4 to 10 ms | same | same | same |
| Catching up every existing page | 84 s to 3.4 min of CPU | 14 to 35 CPU-minutes; about 43 to 105 minutes wall at a third of a core | 17 to 42 CPU-minutes; 52 to 125 min wall | 1.2 to 2.8 CPU-hours; 3.5 to 8.4 h wall |
| A 12-word search | 196 / 271 ms with no pruning (the whole index is under the budget) | 80 / 112 ms with the budget | same order, bounded by the budget | same order |

Disk is the number to watch: Node B's headroom is the recurring risk
(`scripts/status-storage`), and the index is 1.3 GiB for the paper's
207,000 pages today, 6.2 GiB at a million. It is derived: it can be deleted
and rebuilt, and it is not in the restore set. If the real mean text per page
is not 7.8 KB (1.1 gives the query), disk and catch-up time scale with it.
Search CPU is small: research runs about 190 corpus searches an hour
(audit), under 100 ms of one core each. Nothing here calls a model.

## 4. Catch-up and restarts

`Store.StartCaptureFullTextCatchUp` (called from corpusd's `main`) runs
`CatchUpCaptureFullText` 20 s after boot, again when an inline write failed,
and once a day. It pages `og_objects` by primary key, 100 captures a batch,
asks the index which of them it does not hold from that text
(`Index.Missing`), indexes those, and after the forward pass removes index
pages whose capture is gone. It is idempotent (a second pass writes nothing) and safe to kill, and,
like `ReconcileCaptureIndex`, it skips the walk when nothing failed since the
last one and the captures minus the indexed pages are still the gap that walk
measured (captures that cannot be indexed: no text blob, owner-private).

**Why this is not "resuming work" (CLAUDE.md, Restarts).** It opens no run, no
work item and no obligation, closes none, reads no marker, and restores no
in-flight occurrence. It re-derives a projection from its sources of truth,
as `ReconcileCaptureIndex` does at every boot for `proj_capture_index`. Doing
it twice, or never, changes no fact: only what a search can find. After a
crash it simply starts the same walk, which skips what is indexed. A host
that loses the `capture-fts` directory loses nothing but time.

**Yielding.** Between batches it rests for twice the time the batch took (a
third of one core at most), waits up to 2 s for searches in flight to finish
(`captureFullText.searching`), and writes through the same per-file writer
lock as the inline path, so a capture is never queued behind more than one
batch's page write. Searches read in WAL mode and are not blocked by it. It
logs `corpusd capture fulltext catch-up scanned= indexed= skipped= failed=
removed= total= remaining= rate= elapsed=` every 30 s and at the end. These
are not model calls; nothing here limits any.

**Until a capture is caught up** it is found the old way, by title and
labels, because the guest runs both halves; nothing gets worse the day this
deploys.

## 5. What the vector index takes from this

The `passages` table is the key. A vector is stored against `(scope, passage
id)`; the passage id is unique within the scope and never reused (a replaced
page gets new ids), so a stale vector cannot point at the wrong text. To
embed a passage: read `pages.text_hash`, `Range(hash, start_byte, end_byte)`
from the capture body store, prepend `pages.title` (the model takes a title
field). The plan in [EMBEDDINGGEMMA.md](EMBEDDINGGEMMA.md) section 0.8 embeds
"the lead of each captured page, the first 512 tokens": that is passage
ordinal 0 (about 300 tokens; two passages for 512). A hybrid ranker takes two
lists of `(capture_id, passage_id, score)`: this index's top passages
(`Search` returns the passage id of each page's best) and the vector index's,
and fuses by reciprocal rank at passage level, then groups by capture. The
catch-up's shape (idempotent walk by primary key, skip by lookup, yield) is
the one to copy.

## 6. Follow-ups and residuals

- **Remove the term table's search role** (`proj_capture_terms` and
  `SearchCaptures`' `Words` path) once the catch-up has reported `remaining=0` on the paper's computer and a
  week of `by_labels_only` counts show the full-text half finds what the
  title half does. It is not deleted here. `proj_capture_index` stays: the
  URL lookup and the title half read it.
- Orphan removal after a failed inline delete is done by the catch-up's
  sweep, which runs only in scopes whose index holds more pages than the
  forward pass found indexable.
- A scope with no captures anymore keeps its (empty) file.
- The index is not in the restore set: it is derived, rebuilt by the
  catch-up.
- A query of one very common word on the paper's index scans every passage
  that holds it; the 8 s deadline bounds it and the guest falls back to the
  title half with a note. Measured latencies are in section 1.2.
- Two settings exist that this change did not add and reads no further than
  before: `CHOIR_CORPUS_SEARCH=window` (the guest never asks corpusd's capture
  route) and `CORPUSD_CAPTURE_SEARCH=off` (corpusd's route refuses). The text
  search rides that route, so both now also turn the text half off, with the
  title half falling back to the window. They are a residual to remove, not to
  extend; nothing new reads them.
- Punctuation inside a word splits it (`C++` is `c`, `AT&T` is `t`); the shared
  tokenizer was left alone because it also serves the published-stories index.
  The tool text says so and what to write instead.
- The tool and surface text edit moves the guest floor digest
  (`internal/yaegikernel` is a floor path), so a guest image built from this
  change cold-boots hibernated computers.
- The tests were committed with the code, not before it. What was seen: the
  first round's tests failed against a stubbed index (10 of 11 engine tests,
  7 of 8 corpusd tests, 4 of 6 research tests); the repair round's tests each
  failed against a copy with its repair reverted, then passed.
- Not measured: the real size distribution of the paper's text blobs (1.1 has
  the query); the index on the paper's 207,000 pages (the claim to enter in
  `calibration/claude-ledger.md` before landing: the catch-up reaches
  `remaining=0` and a body-only fact is found on staging); the Postgres rows at
  20,000 pages; query latency on an idle machine.
- Disk headroom on Node B is the recurring risk (`scripts/status-storage`):
  section 3 gives the index's size.

## 7. Surprises (read before changing the engine)

- **FTS5 `bm25()` returns 0 for a contentless table with `detail=column` or
  `detail=none`** (seen, SQLite in `modernc.org/sqlite` v1.49.0; it works with
  `detail=full`, and with a content-storing table in any detail). Ranking by
  `rank` silently orders arbitrarily. `detail=full` costs the positions.
- FTS5 gives a term found in more than half the documents a weight of about
  zero (idf is floored at 1e-6): a test corpus of two or three documents ranks
  nothing; use at least a score-able corpus.
- `ORDER BY bm25()` scores every matching row; a query of common words is a
  scan. Stop words and pruning exist for that.
- The build VM is shared: a bench run was killed by the memory manager twice
  (3.9 GB, other builds). The Postgres rows were measured on a smaller corpus
  for that reason.
