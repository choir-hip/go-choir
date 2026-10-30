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

## Landing reconciliation (rebase onto main 3a5084fea)

The branch was rebased as `land/store-b-retrieval` (not pushed). The
rebase had no textual conflicts. Each of main's changes was then checked
against the branch:

- **Cross-owner scoping (96dd9fc62, auth-boundary-gaps F5).** Main now
  requires a guest's object list to name exactly the owner and computer
  vmctl binds it to. The new routes already use that rule
  (`requireInternalOrBoundGuest(r, owner, computer, false)`, which routes to
  `requireInternalOrBoundGuestWrite`), and their SQL is scoped by
  `owner_id AND computer_id`. Added
  `TestCaptureIndexRoutesFollowTheGuestListRule`:
  - a bound guest naming another owner, another computer, or both is
    refused on both routes;
  - an unbound tap address is refused;
  - a loopback caller with the marker but no host credential is refused
    under hostauth enforce;
  - a host search for another owner on the same computer id returns
    nothing.
- **Host credential (44656ef90).** The routes inherit `trustedInternalCaller`,
  which now includes `hostauth.Authorize`. The guest client reaches corpusd
  over its tap and is bound by vmctl, so it needs no credential.
- **Object-graph write gate and boot-ordered record columns (85d2295b7,
  f1005b16e, 8f6708541).** Those are the guest store
  (`internal/objectgraph`, `internal/store`). This branch changes Store B
  only (`internal/platform`); no shared index names or DDL.
- **Guest Dolt GC (4bf25d82a).** Guest store only. Store B's two
  projection tables are `dolt_ignore`d (working set only, never committed);
  `og_lookup_keys` is versioned with `og_objects` (small: five
  publication-side kinds).
- **Dolt password auth (9ab0c6b7b).** Node B is at stage `users`: corpusd
  still connects as root. At stage `clients`, the app user has `ALL` on its
  one database, which covers `CREATE INDEX`, `CREATE TABLE` and writes to
  `dolt_ignore`. This has not been run against an app-user connection.
  Check it when the stage flips (residual).

### Migration safety (the 2026-10-14 index lesson)

The lesson ([guest-cpu-is-unindexed-store-scans-2026-10-14.md](guest-cpu-is-unindexed-store-scans-2026-10-14.md),
"Deploy evidence"): a guest built its record indexes before its first
healthy answer, took 1m55s, passed vmctl's 120 s write timeout, and failed
the deploy. Store B's migration does not have that shape:

- It runs in corpusd (a host service), not in a guest, so vmctl's
  deploy-refresh timeout does not apply. CI restarts corpusd and curls
  `127.0.0.1:8086/health` with five retries.
- `StartRetrievalMigrations` is a goroutine started before the listener
  and never awaited. `Bootstrap` runs none of the new DDL. `/health`
  answers at once. Each new query shape switches on only when its step
  reports live (`retrieval.live` on `/health`). A failed step leaves the
  old shape in use and retries with backoff (30 s, doubling to 10 min).
- The steps:
  1. two `CREATE INDEX` statements: the `og_objects` composite and
     `og_edges (to_id, kind)`;
  2. create and backfill `og_lookup_keys`;
  3. create the two projection tables;
  4. the throttled projection backfill (250-capture pages, 250 ms pause).
- Estimated backfill time: Store B was reset on 10-09 and takes ~4,500
  captures a day, so the projection holds ~25-30 k captures. At the
  measured throttled rate of ~180 captures/s, that is about 3 minutes.
  Search and URL lookup refuse until it completes, so guests use the
  window until then.
- Measured locally with a capture writer every 20 ms running beside the
  whole migration, at 50 k captures: no write errors, p95 225 ms, max
  3.4 s.
- Not measured: `CREATE INDEX` on Node B's platform Dolt (MemoryHigh 3G;
  the 2026-10-01 OOM cluster) while Store A shares the server. Store B
  still shares the `platform` database unless `corpus-dsn.env` is present.
  Watch `go-choir-platform-dolt` memory during the first boot. Each index
  is committed once (`markCorpusDirty`), so its chunks enter platform Dolt
  history once.

### Second rebase and Dolt 2.4.2 check (2026-10-17, after the Dolt soak)

Rebased again onto main dc3d9c38a (138 commits later). There were no textual conflicts.
Two of main's changes overlap the branch in meaning:
- **575cb3ef5** made the check overlay read checks by ID. The
  `publication_check` lookup key is now unused on that path, which is harmless.
- **9eb2c037f**, the event layer, lives in `corpus_events.go`, which this branch does not touch. The
  capture half of corpus search is the only part this branch changes.

`go build ./...` and `go vet` pass. `go test ./internal/platform ./internal/researchtools` passes.

Node B's corpus and platform servers now run **Dolt 2.4.2**, not 2.1.9.
`scripts/ograph-bench platform` ran against a local `dolt sql-server` 2.4.2,
using the production DSN flags (`multiStatements=true` is required:
Bootstrap is multi-statement). It seeded 20,000 captures over 10 days and 3 computers:
- the indexes built in 118 ms and 13 ms;
- `og_lookup_keys` backfilled 1,400 keys in 108 ms;
- the throttled capture backfill took 69 s;
- all three steps reported live;
- concurrent capture writes: 1,163 with 0 errors, p95 103 ms, max 420 ms.

After the migration (ms p50):
- window ladder 23;
- corpus search 3.6–54;
- capture by URL 1.1;
- `public_route` by lookup key 0.97;
- capture write 20 (it was 7.4 before; the projection costs extra, as recorded above).

The upsert breakage recorded for 2.1.9 does not arise, because the branch uses the side table.

### Rollback

- Host switch without a deploy: `nix/node-b.nix` now gives corpusd an
  optional `EnvironmentFile`, `/var/lib/go-choir/corpusd-retrieval.env`.
  Write `CORPUSD_CAPTURE_SEARCH=off` there and restart `go-choir-corpusd`.
  Both routes then answer 503, and guests fall back to the window.
- Guest switch: with `CHOIR_CORPUS_SEARCH=window` in its runtime
  environment, a guest never calls the routes. That setting reaches a
  computer only through its runtime environment, so the host switch is
  the one to use in operation.
- Schema: revert the commits. The indexes and tables can stay; without
  the code they do nothing.

### Staging proof (after the deploy reports the landed SHA)

Run on Node B (`ssh node-b`). The host credential header comes from
the root-only header file, so the secret never appears on a command line.
The owner is the paper computer's owner (`CORPUSD_AUTOPAPER_OWNERS`).

```sh
H=/var/lib/go-choir/secrets/internal-service-token.header
C=http://127.0.0.1:8086
OWNER=e5962310-8767-4624-87e5-a9b93a4e1db1
# 1. identity, and the migration live (wait for capture_index_complete)
curl -fsS $C/health | jq '{commit:.build.commit, live:.retrieval.live}'
journalctl -u go-choir-corpusd --since -2h | grep -E 'corpusd retrieval migrate (index|lookup_keys|done)'
# 2. a capture older than the old 1,000-row window, found by URL over all history
OLD=$(curl -fsS -H @$H -H 'X-Internal-Caller: true' "$C/internal/platform/objects?kind=choir.web_capture&owner=$OWNER&limit=3000" | jq -c '.[-1] | {id:.canonical_id, computer:.computer_id, url:.metadata.url}')
COMP=$(echo "$OLD" | jq -r .computer); URL=$(echo "$OLD" | jq -r .url)
curl -fsS -G -H @$H -H 'X-Internal-Caller: true' "$C/internal/platform/captures/by-url" \
  --data-urlencode owner=$OWNER --data-urlencode computer=$COMP --data-urlencode "url=$URL" | jq .
#    expect found=true and capture_id equal to $(echo "$OLD" | jq -r .id)
# 3. search over all history; searched equals the computer's projected captures
curl -fsS -H @$H -H 'X-Internal-Caller: true' -X POST $C/internal/platform/captures/search \
  -d "{\"owner_id\":\"$OWNER\",\"computer_id\":\"$COMP\",\"words\":[\"ai\"],\"need\":1,\"candidates\":5}" \
  | jq '{searched, matched, n:(.objects|length)}'
# 4. owner scope: another owner on the same computer id finds nothing
curl -fsS -H @$H -H 'X-Internal-Caller: true' -X POST $C/internal/platform/captures/search \
  -d "{\"owner_id\":\"not-$OWNER\",\"computer_id\":\"$COMP\",\"words\":[\"ai\"],\"need\":1}" | jq '.objects|length'   # expect 0
# 5. the guest uses it: corpus-search lines from the paper computer after its next research run
#    (capture_ids no longer bounded to the newest 1,000), and corpusd timing per call
journalctl -u go-choir-corpusd --since -1h | grep -E 'corpusd retrieval op=capture_(search|by_url)' | tail
curl -fsS $C/health | jq '.retrieval.ops | with_entries(select(.key|test("capture")))'
```

Step 2 needs a computer with more than 1,000 captures. The paper
computer has about 4,500 a day.

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

## Landed and proved on staging (2026-10-17)

Pushed as main 4ed852b67, CI run 39047149193 green, staging deployed it at
16:04:16. corpusd on Node B (corpus Dolt 2.4.2) ran the migration:
- the two indexes built in 1,524 ms and 1,138 ms;
- 42,145 lookup keys backfilled in 3,487 ms;
- the throttled capture projection backfilled 13,887 captures in 44 s;
- the migration finished at 16:02:55 (51 s);
- `/health` reports `capture_index`, `capture_index_complete` and
  `lookup_keys` live.

Corpus Dolt memory afterwards was 2.2 GB, against its 5.4 GB MemoryHigh, with no unit restarts.

Proof (steps 1-4 above, run on Node B):
- capture by URL: the oldest of the paper's newest 3,000 captures
  (2026-10-16 03:57) was found, with the same capture id;
- search over all history: `searched` 12,132, `matched` 1,590;
- owner scope: another owner on the same computer id gets 0 objects.

Inline projection writes for new captures took about 7 ms, with 0 errors. The paper took a
clean planned restart at 16:02:22. Step 5 (the paper's own research
calls through the new routes) is checked after its next research run.

Step 5 (16:19): the paper's own calls go through the new routes. Since
16:06, after the proof queries, corpusd logged 4 `capture_by_url` calls
(mean 3.8 ms, 0 errors) and 2 more `capture_search` calls (70 ms with 251
rows; 340 ms with 1,598 rows for a broad query), all with 0 errors.
