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

## Found while building: Dolt 2.1.9 breaks upserts on indexed expressions

The research note's §3.1 measured functional indexes and generated
columns as working for **reads**. They break Store B's **writes**.
Reproduced on the `dolt 2.1.9` CLI (the Node B binary) and on the
embedded engine the platform tests use, with Store B's own upsert shape
(`INSERT … ON DUPLICATE KEY UPDATE col = VALUES(col)`, used by
`PutObject` and `PutBatch`):

- **VIRTUAL generated column** on `og_objects`: the second upsert of a
  row fails with `'{"a":"2"}' is not a valid value for 'bigint'` — the
  update writes `metadata` into `body_size`. The `AS new … new.col` alias
  form fails the same way. Caught by the platform tests
  (`TestPublishTextureCreatesImmutablePublicRecords`,
  `TestPublicationPublicSurfacesEnforceVisibilityPolicy`) before any
  commit.
- **Functional index**: every upsert fails with `column "!hidden!fk!0!0"
  could not be found in any table in scope`; a composite functional index
  `(object_kind, (expr))` is refused ("multiple expressions are not
  supported").
- **STORED generated column**: upserts work, but adding one rewrites every
  `og_objects` row, so the whole table re-enters Dolt history (the 218 G
  class of growth).

Consequence for failure mode 3: the four JSON lookups cannot be indexed
on `og_objects` itself on this Dolt. They get a narrow side table
`og_lookup_keys (object_kind, lookup_key, canonical_id)` written in the
same transaction as the object, backfilled by the migration, read by
primary key, with the JSON comparison still deciding.

## Plan (this branch; not pushed, not deployed)

Rungs 0-3 of the research note's ladder, on Store B only:

1. Time and count every retrieval call (one structured log line per call,
   counters and latency buckets on corpusd `/health` under `retrieval`);
   define `retrieval-eval-v1` (queries and judged ids in the repo; any
   article text outside it).
2. Store B: composite `(object_kind, owner_id, computer_id, updated_at)`; a
   lookup key per registered `(kind, path)` pair (the side table above,
   not a generated column) and the lookups rewritten to seek it;
   `(to_id, kind)` on edges; a time-bounded ladder on every newest-N query
   (`ListObjects`, the front list); plan tests.
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

## Found while measuring: an unthrottled backfill starves capture writes

`scripts/ograph-bench platform` (local `dolt sql-server` 2.1.9, one owner,
three computers, 120 days, a capture writer every 20 ms beside the
migration). The first projection writer issued three statements per
capture: 20,000 captures backfilled in 613 s. Batched into multi-row
statements: 60 s. At 200,000 captures the batched, unthrottled backfill
took 69 min and **stalled one capture write for 45.8 s** (inline
projection writes up to 21 s; p95 569 ms; no errors) — long enough to
time out a guest's 100-item poll (60 s client). With 250-capture pages
and a 250 ms pause after each written page, 50,000 captures backfill in
278 s and the concurrent writer saw p50 38 ms, p95 225 ms, max 3.4 s, no
errors. (Recorded in the commit message of 4326473df and here; the fix
landed in the same sitting.)

## Fix on the branch `retrieval-store-b-rungs-0-3` (not pushed, not deployed)

| Commit | What |
|---|---|
| e87fdf56e | rung 0: per-operation retrieval counters and latency buckets on corpusd `/health` (`retrieval`), one journal line per route-level call, guest corpus-search line with query and result ids; `retrieval-eval-v1` definition and 568 frozen queries |
| 278d6b200 | this record: the Dolt 2.1.9 upsert breakage |
| cc3dc5250 | rung 1: composite and edge indexes, `og_lookup_keys`, time-bounded newest-N ladder, plan tests |
| 0643f7fa2 | rungs 2-3: capture projection and term table (`dolt_ignore`d), search and URL routes, guest switch with window fallback, E2E and recall tests |
| c49bd6b62 | multi-row projection and key backfill writes |
| a023151c4 | `ograph-bench platform`: before/after through the shipped code |
| 4326473df | backfill throttle |

Local measurements, ms p50 / p95, warm, one owner over three computers
(the computer searched holds a third of the captures):

| Query | 50k before | 50k after | 200k before | 200k after |
|---|---|---|---|---|
| corpus window, newest 1,000 of a computer | 125 / 137 | 34 / 106 (ladder) | 771 / 944 | 170 / 206 (ladder) |
| origin/main window text, new composite index | — | 80 / 106 | — | 539 / 1,270 |
| corpus search, `[harbor storm]` (window: 1,000 newest; after: all history) | 125 (window) | 6.0 / 6.2 | 771 (window) | 24 / 26 |
| corpus search, `[ai]` (very common; 500 candidates) | 125 (window) | 72 / 80 | 771 (window) | 193 / 211 |
| corpus search, `[人工智能]` | 125 (window) | 36 / 40 | 771 (window) | 75 / 105 |
| URL citation lookup | 127 / 134 (window, misses older) | 4.0 / 5.1 | 602 / 734 | 5.1 / 5.4 |
| `public_route` by `route_path` | 2.2 / 3.2 | 1.2 / 4.9 | 8.1 / 8.5 | 4.9 / 5.3 |
| `publication_check` by version | 10 / 11 | 5.0 / 14 | 44 / 54 | 5.2 / 8.5 |
| capture write (`InsertObjectIfAbsent`) | 10 / 11 | 20 / 31 | 9.3 / 14 | 30 / 41 |
| migration, all steps | | 278 s (throttled) | | 69 min (unthrottled) |

The time ladder does not reach the research note's 28 ms because a
computer here holds ~550 captures a day, so the newest 1,000 span two
windows, and the call returns 1,000 full rows; the unbounded text on the
same index stays 3-6× slower and grows with the range. Each capture write
pays ~10-20 ms more for its projection (a second transaction).

## Residuals

- **Backfill of a large Store B is slow** (~180 captures/s throttled, and
  the rate falls as the term table grows). At today's Store B size (reset
  10-09; ~4,500 captures a day) it is minutes; at 1M captures it would be
  hours in the background, with search refusing (guest on the window)
  until it completes. A bulk offline build is the next step if needed.
- **A capture written a moment before its projection row** is missed by
  the index; the URL lookup falls back to the window on a miss, the
  corpus search does not (it is ranked over all history and finds it on
  the next call).
- **`og_lookup_keys` backfill race**: an object whose keyed field changes
  while the boot backfill reads it can keep a stale key until the next
  boot. The keyed fields are immutable ids.
- **Node B absolute numbers are unmeasured** (6-core Xeon vs the 8-core
  M1 here); `/health` `retrieval` gives them after deploy.
- **retrieval-eval-v1 labels are silver** (the run read it); pooled blind
  judging and the frozen corpus snapshot come with the first scored rung.
- Guest-side shapes (`capture_label.capture_id`, `SearchDocuments`,
  `ListStoryDocuments`) are the guest-store work's (Overlap above).

## Heresy delta

- discovered: as listed under Evidence; Dolt 2.1.9 upserts break on
  virtual generated columns and functional indexes; an unthrottled
  projection backfill starves Store B writers.
- introduced: two rebuildable projections outside Dolt history
  (`proj_capture_index`, `proj_capture_terms`) whose authority stays
  `og_objects`; a ~2× capture write cost.
- repaired (on the branch, not deployed): untimed retrieval; the
  newest-1,000 corpus window and URL lookup (now all history); JSON-path
  scans for the five Store B lookups; unbounded newest-N sorts; edges to an
  id by kind.
