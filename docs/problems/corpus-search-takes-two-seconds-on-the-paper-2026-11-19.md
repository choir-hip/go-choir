# A corpus search takes about two seconds on the paper (2026-11-19)

## Problem

One `search_wire_corpus` call on the newspaper's computer takes a median of
2.1 seconds, and one in ten takes more than 5.4. The research desk makes
about two such calls a run, so it pays about four seconds a run today. The
corpus brief (lane `corpus-brief`, not landed) makes up to six, one after
another, under an eight second deadline before the run's first model call.
At these times the brief would answer about three threads of six, report
the rest as unanswered, and hold every research run for the full eight
seconds. Seven review rounds of the brief did not find this, because none
of them measured the search.

## Evidence

Read from Node B on 2026-11-19, 01:30Z to 02:30Z.

The guest's own line (`corpus search computer=… captures_ms=…`), 39 calls:
minimum 602 ms, quartile 1,476, median 2,096, third quartile 3,397, ninth
decile 5,423, maximum 18,399. `text_indexed=214195 computer_pages=214195`.

corpusd's journal (`corpusd retrieval op=… ms=…`), the same hour:

| operation | calls | mean ms | max ms | over 1 s |
| --- | --- | --- | --- | --- |
| `capture_search shape=terms` (titles and labels) | 35 | 1,315 | 7,208 | 20 |
| `capture_fulltext_search shape=passages` (page text) | 35 | 751 | 1,311 | 8 |
| `capture_by_url shape=url_key` | 20 | 0.4 | 1.2 | 0 |

The title-and-label half is the slower one, though it searches far less
text than the page-text half.

## Belief

The time is in corpusd, not in the guest or the network. Established by a
read-only reproduction on the build machine (2026-11-19, 214,000 synthetic
captures and 2.76 million term rows in Postgres 17; 100,000 pages and 406,463
passages in the full-text index):

- **The two journal lines nested.** `capture_search shape=terms` timed the whole
  call, the text search included (`SearchCaptures` started its timer first). Of
  the 1,315 ms mean, about 750 ms was the text half and about 560 ms the term
  table. The two lines did not add up; the first one included the second.
- **Term table.** The statement reads every posting of every query word, prefix
  expansions included, and sorts them for `COUNT(DISTINCT)`; its time is the sum
  of the words' postings (about 110 to 150 ms for one word of 82,000 postings,
  against a posting of a rare word near zero). The primary key does not hold
  `fetched_at`, so every posting was also a heap fetch (a bitmap heap scan,
  90,000 buffers for a 185,000-posting query). Not the cause: the `COUNT(*)` of
  the scope (15 to 20 ms) and fetching the 500 candidate objects.
- **Text half.** The scan budget bounds the postings read (100,000) but not the
  work after: the statement joined every matching passage to its passage and
  page rows and numbered them by page, to keep 300, over a 923 MiB file with 8
  MiB of SQLite page cache per connection. Reproduced at 100,000 pages with the
  real budget: 356 to 640 ms at the median, 3.2 s worst, the production figures;
  after the cut 106 to 163 ms at the median (ninth decile 177 to 969).
  The reference page's "about 100 ms" came from a run with the budget scaled
  down and a warm cache.

## What is being done

- Built in the lane `capture-search-latency` (orange; Ring-Expedite):
  1. `idx_pct_cover` on `proj_capture_terms` (`owner_id, computer_id, term,
     capture_id` INCLUDE `fetched_at`), built `CONCURRENTLY` outside a
     transaction at start and repaired on each periodic retrieval tick (10
     minutes) by `ensureCaptureTermsCoverIndex`; an index left
     invalid by an interrupted build is dropped and rebuilt; the search works,
     slower, until it is valid; one vacuum follows the build (without it the
     index gained nothing on a never-vacuumed table). The statement and its
     ranking are unchanged. On the build machine, 214,000 captures: 257 to 86,
     744 to 205, 292 to 108, 210 to 84 ms for queries of 124k to 428k postings.
  2. `capturefts.Search` takes the 1,500 best passages before the joins; the
     answer is the same (proof in `reference/CAPTURE-FULLTEXT.md`, test
     `TestTopPassagesAnswerLikeTheFullStatement`), and the old statement runs
     when the cut could matter and for every windowed query.
  3. The terms timer no longer contains the text search, so the journal's two
     lines add up to the call (`TestSearchCaptureTimersDoNotNest`).
- Not done, a quality decision: lower the scan budget; prune very common words
  from the term search by document frequency; `pruneTerms` still keeps the three
  rarest terms over budget.
- The corpus brief does not land as built. It will run its threads' searches at
  the same time under the one deadline, and lands only after its latency is
  measured against the numbers after this change on the paper.

## Remaining error

- The numbers above are synthetic corpora on a shared build machine, not the
  paper's titles or disk. The after-numbers on the paper (the journal's two
  lines, the guest's `captures_ms`) are the proof, and are not in yet.
- Whether six searches at once slow each other on corpusd.
- The 18.4 second maximum on the guest side has no matching corpusd line over
  7.2 seconds in the same hour; the rest of that call's time is unexplained.
- On the live database the index-only scan needs the table's visibility map;
  the build runs one `VACUUM (ANALYZE)`; autovacuum maintains the map after that.
- Accepted residuals of the review: the periodic repair builds synchronously in
  the periodic loop, so a build that waits on an old transaction delays the
  reconcile until it ends; an unlock error is ignored, so a live session that
  failed to unlock would keep the advisory lock until the pool closes it.
- One hour's numbers from one computer.

## Update, 2026-11-19T05:25Z (station L1; read-only)

Three hours of the paper's console (02:15Z to 05:22Z), 280 `corpus search`
lines: median `captures_ms` 1,706, ninth decile 4,745, maximum 15,603. The
earlier hour's median was 2,096 and its ninth decile 5,423, so the time has
not moved. The corpus brief reached the paper at about 05:24Z. Its first
three `research brief` lines: 3.4 s with four threads answered, 4.1 s with
four answered, and one `outcome=failed reason="the 8s deadline passed"` with
`unanswered=1` (one query, no thread known, nothing found). One failed in
three so far; three lines is not a rate. The failed brief is the case the
earlier "brief claimed absence wrongly" defect was about: the desk is told
the search did not answer, not that nothing is held. The clustering
assessment of this and two other corpus-search defects is in
[recurring agent failures read from traces](recurring-agent-failures-read-from-traces-2026-11-19.md).

## Update, 2026-11-19T07:55Z: the fix deployed, and a second cost found

The faster search deployed at 06:17Z (`a0e852f56`). corpusd's journal for
06:47Z to 07:47Z, 223 searches, each with one line per half:

| line | calls | mean | median | ninth decile | maximum |
| --- | --- | --- | --- | --- | --- |
| `capture_fulltext_search shape=passages` | 223 | 189 ms | 121 ms | | 1,209 ms |
| `capture_search shape=terms` | 223 | 869 ms | 37 ms | 2,046 ms | 14,178 ms |

Before, the same lines read 751 ms and 1,315 ms (the second then included the
first). The text half is four times faster and the ordinary title-and-label
search is fast: 194 of the 223 took 65 ms on average. Ledger claim C143 is
settled false all the same, because the terms mean had to be 250 ms or less.

What carries the mean is a different cost. Twenty-nine calls took over a
second. The slowest eighteen fell between 06:48Z and 06:58Z, took 8 to 14
seconds, and each returned about 1,500 rows (1,512 to 1,946); the fast calls
returned a median of two rows. Two calls ended at 4.9 s with
`err=read_capture_candidates: context canceled`, which is the brief's 8 s
deadline or the guest's giving up. So the time grows with the rows a search
returns, at roughly 7 ms a row, and it is spent after the index has answered:
in reading each candidate.

Belief: a search whose words match many captures returns every match up to a
cap near 1,500 and reads each one, and nothing a caller does needs 1,500
candidates (the guest shows ten to thirty). Not yet read from the code: where
the cap is, what is read per candidate, and which caller's wording produces
the broad matches (the corpus brief sends a thread's subject as its query,
and a subject of common words would do it). Until that is read this is a
hypothesis. No fix is built. The ranked search waiting to land (vector order
with the best three of BM25) calls the same terms half and inherits the cost.

