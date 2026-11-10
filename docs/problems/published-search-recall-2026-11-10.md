# The paper cannot find its own earlier stories (2026-11-10)

Mutation class orange (runtime behaviour of a research tool and one overlay
sentence). Rollback: revert the commit. Wave 3 item 1 of
[the overnight definition](../definitions/choir-autopaper-overnight-2026-11-09.md);
design source
[autopaper-learning-materiality-2026-11-04.md](../design/autopaper-learning-materiality-2026-11-04.md)
sections 8.2 and 8.6. Same defect as lane Q's duplicate detection in
[unsupported-cites-duplicates-and-churn-2026-11-10.md](unsupported-cites-duplicates-and-churn-2026-11-10.md)
(Q2, cause 1: a window of the newest stories, not a search of the archive).

Owner, 2026-11-09: "we should be doing BM25 ... We should make it work
better. No embeddings."

## What is wrong (read from `internal/platform/publication_pieces.go` at 313e5bae3)

`SearchPublishedPieces`, the search research reaches through
`choir.SearchWireCorpus`, does this:

1. It scans the newest 2,000 publications (`publishedPieceScanMax`) of about
   5,700. Older stories are never seen, and every day the blind share grows.
2. It zeroes any story that lacks even one query word (`score = 0; break`).
   A natural query of four words finds only a story holding all four.
3. It scores by raw substring counts, so one repeated common word beats a
   story that holds the query's rare words, and `harbor` finds `harborside`.
4. Every query reads the object graph and the blob of up to 2,000 stories:
   several store reads and a blob read per story, per query.
5. Research's overlay never tells it to look for the paper's own earlier
   stories, so the verb is used for captures and rarely for stories.

## Failure list (written before the tests and the code)

- S1. One query word the story lacks zeroes the story (all-words rule).
- S2. A story older than the newest scan window is invisible.
- S3. Raw counts let one repeated word beat the query's rare words.
- S4. A republished story is found by its old text, or not by its new text,
  once something is cached.
- S5. A story withdrawn after it was indexed is still returned.
- S6. A non-English title, or an accented word, is not found (this one the
  substring match handled; the index must not lose it).
- S7. A restarted corpusd finds nothing until something is published.
- S8. A search that arrives while the index is still being built reports
  nothing wrong and reads as "no earlier story".
- S9. The index leaks content the route does not publish (a hand-published
  owner-mode Texture, a withdrawn story, a non-public access policy).
- S10. Research is never told to look.

Tests: `internal/platform/publication_pieces_bm25_test.go` (S1 to S7, with
the earlier `publication_pieces_test.go` covering S9),
`internal/runtimeprompts/research_own_archive_test.go` (S10). S8 is
`Capped` on the answer, set while the index is incomplete.

## Physics

| Quantity | Number | Source |
|---|---|---|
| Archive | about 5,700 stories now; 150 to 250 new per day, plus about 33 updates an hour | the definition, the keep-set accounting |
| Archive text | about 350 words a story (78% of stories are under 300 words); about 2 MB of text | audit, 2026-11-09 |
| Index memory | 65 MB heap for 5,700 synthetic stories of 350 words on a 60k-term Zipf vocabulary (about 300 distinct terms a story, a high estimate); real stories repeat more, so 30 to 65 MB | measured with a throwaway test, not committed |
| Growth | about 12 KB a story; a year at 200 stories a day is 73,000 stories and about 0.9 GB, so past about a quarter of a million stories the design needs a second look (segments, or a term table in the store) | arithmetic |
| Build time | 0.2 s for 600 published stories through the fixture store (about 0.33 ms a story), so about 2 s for 5,700 on the test store. Production reads sqlite and a blob store, with five store reads a story, so the expectation is 10 to 60 s. Bounded at 10 minutes at boot and 20 s per search | measured fixture; production is an estimate to confirm on d@e |
| Per query | 1.1 ms to rank 5,700 stories for a four-term query; plus about 2 ms of store reads per verified hit (limit times three at most) | measured |
| Currency | one windowed query of the newest 100 publication rows per search (17 ms idle in the fixture, which holds 600 rows in one second; one row re-read in practice) | measured |

Design that follows from the numbers:

- An in-memory inverted index, derived from Store B, never written back. No
  embeddings and no new durable store: the boot cost is seconds, far inside
  every recovery bound.
- BM25 with k1 = 1.2 and b = 0.75; a title word counts as three body words
  (the old title bonus); partial match, the sum over the query words a story
  holds.
- Current without a publish hook. The index remembers the newest publication
  update it has seen; each search reads the rows newer than that and
  re-indexes them. A republish moves the row, so the republished text replaces
  the old (S4).
- The hit list is checked against the store before it is returned: the
  publication must still be published, public and readable (S5, S9). A stale
  index can cost recall, never expose a withdrawn story.
- A restart rebuilds from the store in the background (`WarmPublishedPieceIndex`,
  started at boot), newest first, and a search before the build is whole
  answers from what is built and says so (S7, S8).
- Tokens: the capture term table's tokenizer (letters and digits; CJK, Thai
  and other unspaced scripts as character unigrams and bigrams), so a Han
  title is found by its characters. Accents are kept (a French query with the
  accent finds the accented word; an unaccented query does not fold). A plain
  ASCII plural is folded (`harbors` and `harbor`). No stemming beyond that.
- Query bound: 32 distinct terms.

## What this does not change

The unfiltered `SearchPublished` (the retrieval search for the owner's own
read, `service_publication_read.go`) still scans the newest 2,000. It is not
the research path and is left for its own change. The lane Q candidate
shortlists (`intake_labels.go`, `event_layer.go`) are a separate fix and are
not touched here; this change gives them a correct archive search to call.

## Proof still needed (d@e)

A research run on d@e where a story's subject has an earlier story older than
the newest 2,000: the run's ledger shows a `SearchWireCorpus` call returning
that story as a published piece, a `choir.Read` of its `read_url`, and the
capture in the research packet. Also the corpusd boot log line
`published piece index warm: pieces=N complete=true ms=T` for the real build
time. Hypothesis AL-H4 in
[the pre-registration](../precommitments/autopaper-materiality-hypotheses-2026-11-10.md)
settles the recall question.

## Part 2: the reliance link (wave 3, item 2)

Mutation class red-adjacent (it adds an edge to the publish batch of every
story; additive, one-act rollback by revert). The edge is data only.

`relies_on` (`objectgraph.EdgeReliesOn`, owner corpusd) runs from a published
publication to an earlier published publication of the same owner, written in
the citing version's batch beside `cites_capture`, where the cited captures
are already visited (`publication_reliance.go`). A cited capture names a story
when its URL is on the paper's own host (the public base URL) and is either
the plain-text export (`/api/platform/publications/export?route=...`) or a
public page (`/pub/texture/<slug>[/v<n>]`); the route resolves through the
`choir.public_route` object to the publication, which must be active and
published and not the citing publication. The edge metadata is the fixed
`{"relation":"relies_on"}`, so its id is deterministic: a republish rewrites
the same edge (idempotent), and a republish that drops the citation keeps it
(the link records that the story relied on the other at some version; its
edge time is the first write).

Failure list, tests in `internal/platform/publication_reliance_test.go` and
`internal/store/vocab_write_guard_reliance_test.go`:

- R1. A story that cites a capture of an earlier own story gets no link.
- R2. A capture of another host's page whose path names a route gets a link.
- R3. An unknown route or a withdrawn story gets a link, or fails the publish.
- R4. A republish writes a second link or drops the first.
- R5. A story gets a link to itself.
- R6. Something reads the link: no source outside its writer and its
  registration may name it, and no prompt mentions it (a source scan test).
- R7. The kind is unregistered, so the write gate refuses every publish that
  cites an earlier story.
- R8. The metadata carries a key the vocabulary write guard refuses on a
  computer (the Autopaper graph is adopted onto computers' stores). The metadata
  has the one key `relation`; the store test commits it with the cutover on.

Not built: the independent-sources comparison (pre-registration rule 4 computes
it at settlement from the two stories' cited captures), a correction path that
marks dependents of a corrected story, and any read of the link.
