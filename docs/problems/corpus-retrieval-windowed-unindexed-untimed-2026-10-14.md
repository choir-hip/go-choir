# Corpus retrieval reads a window, scans JSON, and is never timed (2026-10-14)

Store B (corpusd's Dolt, `internal/platform`) answers every corpus, citation
and story lookup by loading a window of rows and matching in Go, four hot
lookups are JSON-path scans no index can serve, and no retrieval call is
timed or counted. This record names those problems, the failure modes any
fix must not have, and the plan, before any code changes (problem
documentation first). Mutation class of the fix: **orange** (database
queries, a new corpusd route, a guest client switch). It touches no red
surface: no Texture canonical write, no auth, no gateway/provider call, no
vmctl, no deploy routing.

Evidence source: [object-graph-retrieval-research-2026-10-14](../reports/object-graph-retrieval-research-2026-10-14.md)
(inventory §1, measurements §4 on local Dolt 2.1.9, the version Store B
runs). Companions:
[search-is-duplicated-and-discarded-2026-10-14](search-is-duplicated-and-discarded-2026-10-14.md)
(corpus-first misses: "the corpus is titles of the newest 1,000 captures"),
[search-efficiency-and-reuse-2026-10-14](../reports/search-efficiency-and-reuse-2026-10-14.md)
§3. Nothing on Node B was queried for this record.

## Evidence (code at `origin/main` 980ffaf4e)

1. **Window, not search.** `researchtools.searchCaptures` (the capture half
   of `choir.SearchWireCorpus`, called by 122 of 136 paper research runs)
   asks corpusd for the computer's newest 1,000 captures
   (`GET /internal/platform/objects?kind=choir.web_capture&owner&computer&limit=1000`)
   and matches query words in Go. At ~4,500 captures a day that is about
   five hours of history. `TextureCaptures.newestCaptureOf` resolves a URL
   citation handle by the same 1,000-row list, so a URL captured more than
   ~5 hours ago is "not captured" for a `{{cite}}`.
2. **The window query is itself slow at scale.**
   `ObjectGraphStore.ListObjects` runs `object_kind=? AND owner_id=? AND
   computer_id=? ORDER BY updated_at DESC LIMIT ?`; Store B has only
   `(object_kind, owner_id)` and `(updated_at)`, so it ranges over every
   capture the owner has on every computer and sorts it. Measured at 1M
   captures: 5.3 s p50, 14 s p95. A composite index alone made it slower
   (14.7 s) because Dolt 2.1.9 sorts the whole equality range for
   `ORDER BY … DESC LIMIT`; composite plus a time bound took 28 ms.
3. **JSON-path lookups are scans.** `GetObjectByMetadata` /
   `ListObjectsByMetadata` run `JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS
   JSON), ?)) = ?` with the path as a bind parameter, which no index can
   match. Store B callers: `choir.public_route` `$.route_path` (every public
   story load), `choir.publication_check` `$.publication_version_id` (every
   publication read), `choir.publication_proposal` `$.proposal_id` (proposal
   delivery), `choir.publication` `$.publication_id` and
   `choir.publication_version` `$.publication_version_id` (publish). Measured
   cost of this shape at 1M rows: 5.0 s p50, 18 s p95.
   (`choir.capture_label` `$.capture_id` is the fifth hot path named in the
   research note, but capture labels live in the **guest** store
   `internal/store/capture_label.go`, not Store B; see Overlap.)
4. **Edges to an object by kind** have only a single-column `(to_id)` index.
5. **No timing.** No retrieval call in corpusd logs a latency or a count;
   `/health` reports only store reachability. Every claim about retrieval
   cost today is synthetic.
6. **No yardstick.** No frozen labelled set exists to say whether a change
   to retrieval improved recall.

## Failure modes any fix must not have

Written before the code. Each names the test or check that guards it.

1. **A changed answer.** The time-bounded ladder must return exactly the
   rows the unbounded query returns, in the same order, for every limit
   (including rows at a window boundary, rows with a future `updated_at`,
   and a computer whose newest row is older than every window). Guard:
   equivalence test against the unbounded query on a fixture.
2. **An index the planner ignores, or one that flips another plan** (§4 G3
   of the research note: one added index turned a 1.5 ms edge lookup into
   12 s). Guard: `EXPLAIN PLAN` tests for every rewritten query, asserting
   the index name and no full-table access, including the queries whose
   text did not change (edges from an id, edges to an id, PK reads).
3. **A generated column that rejects a write.** A column computed from
   `metadata` must never make an `INSERT` fail: invalid JSON, a non-string
   value, an over-long value, or an unregistered kind must yield NULL or a
   truncated key, never an error. Guard: write test with each case.
4. **A lookup that misses because the key was truncated or the value is
   not a string.** The indexed key narrows; the original JSON comparison
   still decides. Guard: lookup test with a value longer than the key.
5. **A migration that blocks boot or writes.** Index builds on a table of
   millions of rows take minutes. They must not hold corpusd's start (it
   serves the front page and the capture route), must be idempotent across
   restarts and partial runs, and a failed build must leave the old query
   working. Guard: migrations run after the listener starts, each step is
   `IF NOT EXISTS` or an `information_schema` check, the query path uses a
   new shape only once its index is reported live; local measurement of
   concurrent writes during a build.
6. **History growth.** Rebuildable projections (the typed capture table and
   the term table) must not enter Dolt history or change Store B's commit
   cadence. Guard: the tables are in `dolt_ignore` before they are created;
   the debounced committer is unchanged; test that `dolt_status` after
   `DOLT_ADD -A` does not list them.
7. **A projection that diverges from its authority.** A capture written
   while the projection write fails, a projection table lost to a reset, or
   a backfill interrupted by a restart must be repaired without operator
   action. Guard: projection writes never fail the capture write; a
   reconciler compares counts at boot and periodically and backfills by
   cursor with `INSERT IGNORE`; the search route refuses (so the guest
   falls back) until the backfill is complete.
8. **Corpus search loses matches the window path found.** Within the
   window, every capture the old path returned must still be returned, with
   the same ranking (label-only matches included). Guard: equivalence test
   over fixtures run through both paths; deviations are named, not
   accidental.
9. **Tokenizer gaps.** Two-character tokens (AI, EU, US, 5G), CJK without
   spaces, Arabic and Hebrew with attached prefixes, mixed case and
   diacritics must be findable at least as well as by the old Go matcher.
   Guard: recall tests per class.
10. **A search that crosses computers or owners.** The SQL search is scoped
    to the caller's owner and computer, as the window was, and the route
    requires the bound guest for that computer. Guard: scope test.
11. **No rollback.** Each new path has a switch that restores today's
    behaviour without a redeploy of the other side: corpusd
    `CORPUSD_CAPTURE_SEARCH=off` makes the search route refuse (the guest
    falls back to the window), and the guest's `CHOIR_CORPUS_SEARCH=window`
    never calls it. Dropping the indexes and reverting the query text is the
    schema rollback.
12. **Timing that costs more than it measures, or floods the journal.**
    Counters are lock-cheap; the log line is one per retrieval call at the
    route level, PK and edge reads log only when slow.

## Plan (this branch; not pushed, not deployed)

Rungs 0-3 of the research note's ladder, on Store B only:

1. Time and count every retrieval call (one structured log line per call,
   counters and latency buckets on corpusd `/health` under `retrieval`);
   define `retrieval-eval-v1` (queries and judged ids in the repo; any
   article text outside it).
2. Store B: composite `(object_kind, owner_id, computer_id, updated_at)`; one
   virtual generated lookup key over the registered `(kind, path)` pairs
   with an index `(object_kind, lookup key, updated_at)` and the queries
   rewritten to it; `(to_id, kind)` on edges; a time-bounded ladder on every
   newest-N query (`ListObjects`, the front list); plan tests.
3. A typed capture projection and a term table (any token length, CJK and
   other unsegmented scripts by character unigrams and bigrams), written
   with each capture and backfilled, both `dolt_ignore`d; a corpusd search
   route and URL lookup over all history; the guest's corpus search and URL
   citation lookup use them, with the window path kept behind the switches
   above.
4. Migrations online and idempotent; Store B commit batching unchanged.

## Overlap with the guest-store indexing work

Another agent is indexing the **guest** store (`internal/store`, commitment
records and inbox) at the same time. This work stays on Store B. Shared
surfaces:

- `internal/store/capture_label.go` (`LatestCaptureLabel`, `CaptureLabels`,
  `RecentCaptureLabels`): the `capture_label.capture_id` JSON scan is a
  guest-store problem (G5 in the research note) and is left to that work.
  The corpus search still reads `RecentCaptureLabels` as hints, unchanged.
- `internal/objectgraph/dolt_store.go` `ListObjectsByMetadataPage` (guest)
  never uses the guest's generated columns: guest-side, not touched here.
- `store.SearchDocuments` (the story half of `SearchWireCorpus`) scans
  every Texture document in the guest: guest-side, not touched here.

## Heresy delta

- discovered: as listed under Evidence (no new ones beyond the research
  note, which first recorded them).
- introduced: none (documentation).
- repaired: none yet.
