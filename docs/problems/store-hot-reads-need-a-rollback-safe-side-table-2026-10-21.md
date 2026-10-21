# The three hot store reads need a rollback-safe side table, not columns on og_objects

Date: 2026-10-21. Status: **open**; design below, code to follow in lane
lane/b2-store (FP1). Mutation class of the fix: red (object store schema,
every writer of `og_objects`, boot path).
Found by: reading the code for the fix of
[stored-column-add-blocks-paper-boot-2026-10-21.md](stored-column-add-blocks-paper-boot-2026-10-21.md)
after the revert (f2c4a3ec3). Problem documentation first: no fix in this
commit.

## What is still slow (measured, not inferred)

The paper at c893a956a, 122 s from 17:58:57 UTC (orchestrator, read-only):
engine mutex held 70% of the time, 247.6 s of waiting.

| Read | Calls | Held | Per call |
|---|---|---|---|
| `ListObjectsByOwnerAndBody choir.commitment_record $.addressee` | 10 | 19.5 s | 1.95 s |
| `ListObjectsByMetadataPage choir.run $.agent_id` | 98 | 18.5 s | 188 ms |
| `GetObject` (label has no kind) | 10,701 | 15.5 s held, 150.5 s waited | 1.4 ms |
| `ListObjectsPage` (label has no kind) | 417 | 14.7 s | 35 ms |
| `ListObjectRefsByKindOwner` | 23 | 2.8 s | |
| `ListObjects` | 4 | 2.5 s | 628 ms |
| `ListObjectsByMetadataPage choir.source_ref $.doc_id` | 4 | 2.1 s | 517 ms |

The first, second and last rows are this batch's three JSON scans (E6 in
[research-more-slow-and-hidden-2026-10-21.md](research-more-slow-and-hidden-2026-10-21.md)).
`GetObject` rose from about 50 to 88 calls a second between 14:24 and now
and nobody can say which kind of object is being read: the op label carries
no kind. Labelling `GetObject` and `ListObjectsPage` by kind is part of this
batch so the next measurement can attribute them.

## Why not columns on og_objects again

The reverted attempt (51667a3ce) ran `ALTER TABLE ... ADD COLUMN ... STORED`
on the boot path. Its panel found the plain-column replacement unsafe for
rollback: an older binary's `INSERT ... ON DUPLICATE KEY UPDATE` leaves a
derived column untouched, so after rollback-then-forward a row can carry a
derived value that no longer matches its body, and nothing marks it. A side
table carries the `content_hash` the derivation was made from, so staleness
is detectable.

## Findings from reading the code (new; each one shapes the design)

1. **Six SQL sites write og_objects in the guest** (the earlier doc said
   four): `PutObject` and `putBatchTx` and `DeleteObject`
   (`internal/objectgraph/dolt_store.go`), `projectObject`
   (`internal/store/project.go`, the production writer: every object on the
   tape reaches og_objects through it), and a DELETE and two UPDATEs in
   `internal/store/vocab_migrate_og.go` (the UPDATE renames `canonical_id`).
   The platform store (corpusd, `internal/platform`) is a different database
   and is out of scope.
2. **The production writer does not hold the engine mutex.** `finalizeBatch`
   takes `eventMu` and runs its transaction on `s.db` directly. The engine
   mutex serialises only `ogStore` and `ogReadStore`; the projector is
   excluded by the pool's single connection. A backfill that holds the mutex
   per batch therefore still interleaves with the projector at transaction
   granularity (one connection), which is what we need, but it means the
   mutex counters do not show the projector's time.
3. **A new table in the workspace changes the replay verdict.** The replay
   eligibility gate (`internal/agentcore/replay_eligibility.go`) is
   default-deny: a table that is not in `replayAirworthinessEntries` is an
   "unknown table", and the live and replay content hashes of every table
   enter `content_root` (`computerversion.DoltStateExtractor`, `SHOW TABLES`
   then `SELECT *`). A side table that the backfill has not finished would
   make live and replay differ, and the unknown name alone makes the
   computer replay-ineligible. The same holds for the projection-base
   content witness (`internal/projectionbase/witness.go`). The new binary
   must therefore leave the derived table out of both extractions. **An old
   binary after a rollback cannot**: it will list `og_key_index` as unknown
   and report replay-ineligible until the table is dropped. That is the
   rollback cost of any new table, and of any new column too (`SHOW COLUMNS`
   is hashed into the schema observation). It does not stop the computer; it
   withdraws the reversibility claim on the rolled-back binary. Mitigation
   in the landing order: ship the extractor change one deploy before the
   table exists, or accept the claim being withdrawn until the next forward
   deploy. (Hypothesis, not confirmed from a trace: the four STORED columns
   the failed release may have left on the paper's og_objects already put
   the paper's live schema hash out of step with a fresh replay workspace.
   The next replay probe on the paper will say.)
4. **`content_hash` is a convention, not an invariant.** Every writer found
   recomputes `ContentHash(kind, body, metadata)` when it changes body or
   metadata (`newObject`, `lifecycleObject`, the vocabulary upcast and
   migration, the actor wake outbox), but nothing enforces it. A validation
   that trusts the hash catches an older binary's writes only if that holds.
   The design therefore also checks at read time (see Tripwire).
5. **A failing derivation must not fail a write, and the old read already
   fails on a malformed body.** `JSON_EXTRACT(CAST(CAST(body AS CHAR) AS
   JSON), ...)` errors on a body that is not JSON (measured: Error 1105
   "Invalid JSON text"), and the error fails the whole query. A failed
   statement inside a transaction does not abort the transaction in this
   engine (measured), so a maintenance statement can fail alone.
6. **The database collation is `utf8mb4_0900_bin`** (measured): equality on a
   default VARCHAR column is case- and trailing-space-sensitive, as
   `JSON_UNQUOTE(...) = ?` is. JSON null extracts as the string `null`, and a
   number as its text; a faithful derivation stores whatever the engine's own
   expression yields.

## Design

**Table** (created empty by `EnsureSchema`; `CREATE TABLE IF NOT EXISTS`,
indexes inline, so nothing scales with og_objects at boot):

```
og_key_index(
  canonical_id VARCHAR(255) NOT NULL,   -- the og_objects row
  key_name     VARCHAR(64)  NOT NULL,   -- one of three derived families
  key_value    VARCHAR(255) NULL,       -- LEFT(<the old predicate's expression>, 255); NULL = no key
  owner_id     VARCHAR(255) NOT NULL,   -- copied from the row, for the addressee read
  updated_at   DATETIME     NOT NULL,   -- copied from the row, for newest-first
  content_hash VARCHAR(128) NOT NULL,   -- the row's hash when this was derived
  derive_ok    BOOLEAN      NOT NULL,   -- false: the expression errored (malformed body)
  PRIMARY KEY (canonical_id, key_name),
  INDEX (key_name, key_value, canonical_id),
  INDEX (key_name, key_value, owner_id, updated_at, canonical_id))
```

The names `og_meta_agent_id`, `og_meta_capture_id`, `og_meta_doc_id` and
`og_rec_addressee` (what the failed release may have left) are not used and
never named: nothing here depends on their presence or absence, and nothing
rewrites or drops them.

**Families** (the exact old predicates, evaluated by the engine over the
stored row, never re-implemented in Go):

| key_name | kind | expression |
|---|---|---|
| `run.agent_id` | `choir.run` | `LEFT(JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.agent_id')), 255)` |
| `commitment_record.addressee` | `choir.commitment_record` | `LEFT(JSON_UNQUOTE(JSON_EXTRACT(CAST(CAST(body AS CHAR) AS JSON), '$.addressee')), 255)` |
| `source_ref.doc_id` | `choir.source_ref` | `LEFT(JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), '$.doc_id')), 255)` |

A lookup value of 255 characters or more keeps the JSON path (below that
length `LEFT(x, 255) = v` exactly when `x = v`).

**Writers.** After each upsert, in the same transaction, an exported helper
derives the row's keys with `SELECT <expression> FROM og_objects WHERE
canonical_id = ?` and writes the side row with `REPLACE`. A delete or a rename
drops the old id's rows. The helper does nothing for kinds with no family
(no statement). A derivation error stores `derive_ok = false` with a NULL
key, does not fail the write, and sets the family's `bad` flag; a side-table
error of any other kind leaves the family `unvalidated` and wakes the
validator. A test fails if any `INSERT INTO`, `UPDATE`, `REPLACE INTO` or
`DELETE FROM og_objects` appears in a guest non-test file outside the list of
maintained writers. The replay path (`projectBatchForReplay`) skips
maintenance: a replayed workspace is rebuilt by the validator after boot, off
the boot path.

**State, in memory, per family**: `unvalidated` at every open of the store
(never persisted: a persisted flag is exactly what an older binary would
leave lying). Reads use the side table only when the family is `validated`
and not `bad` and the lookup value is shorter than 255 characters;
otherwise the existing JSON path runs, unchanged.

**Validator** (`Store.StartDerivedKeyValidation`, called after the listener is
up and the runtime has started, never on the boot path): per family, pages
`og_objects` of the kind by canonical id with narrow columns
(`canonical_id, owner_id, updated_at, content_hash`; no body, no metadata)
and merges them with the family's side rows over the same id range. A missing
row, an orphan, or a row whose hash, owner or updated_at differs is
re-derived or dropped. Each page is one transaction holding the engine mutex
for that page only, then the validator sleeps in proportion to the page's
hold time (duty cycle at most 25%), logs progress (rows done, rows left, rate)
about every five seconds, resumes after any failure, and sets `validated`
only after the last page with no maintenance failure during the pass. Run
under a writer storm it must end consistent: a writer and a validator page
are never interleaved inside a transaction (one connection).

**Tripwire.** A side-table read compares the `content_hash`, `owner_id` and
`updated_at` of the rows it fetches by primary key with the side rows it
read. A difference marks the family `unvalidated`, wakes the validator, and
answers this call from the JSON path. This catches a missed writer, and a
`content_hash` that was not recomputed, at the cost of one comparison.

**Reads.** `ListObjectsByMetadataPage` (kind `choir.run` `$.agent_id`, kind
`choir.source_ref` `$.doc_id`): ids from `(key_name, key_value, canonical_id)`
after the cursor, in order, limit; rows by primary key in batches of 256.
`ListObjectsByOwnerAndBody` (kind `choir.commitment_record`, exactly one
non-"missing matches empty" match on `$.addressee`): ids from
`(key_name, key_value, owner_id, updated_at, canonical_id)` newest first,
limit; rows by primary key, reordered to the id order. Order among equal
`updated_at` is now `canonical_id` descending; the scan left it to the engine.

**Replay and witness.** `computerversion.DoltStateExtractor` skips the derived
table (`IgnoredTables`); the replay extractor and the projection-base witness
set it.

**Operational labels.** `GetObject` and `ListObjectsPage` take the object's
kind in their engine-mutex op names.

## Ways it can fail (written before the code; each has a test)

1. Boot scans or rewrites og_objects: opening a store with many rows must
   not change og_objects' columns, fill the side table, or take time that
   scales with the rows (test with 20,000 rows).
2. The side read returns other rows or another order than the JSON read:
   randomized differential test over case, trailing space, unicode, JSON
   null, numbers, a missing field, 254/255/256-character values, another
   owner, tombstones, duplicates, paging past 256, limits.
3. A writer misses the table: one behavioural test per maintained site, and
   the source guard.
4. Stale after rollback-then-forward: an old binary's raw INSERT, UPDATE and
   DELETE (no helper) followed by a reopen: reads stay correct before
   validation (JSON path), the validator repairs, reads then match.
5. The validator and writers interleave wrongly: writer storm during a pass.
6. The validator starves the engine: per-page hold and the wait of a
   concurrent `GetObject` are bounded.
7. A malformed body fails a write, or the read answers differently from the
   JSON path.
8. The stray STORED columns break something: all the above with the four
   columns present.
9. Replay / witness: the extractor skips the table; live and replay agree
   with a half-filled side table.
10. The op labels carry a kind and stay low-cardinality.

## Rollback

`git revert` of the batch. The side table stays on the paper's database
(additive; an older binary never names it). An older binary lists it as an
unknown table (finding 3). Dropping it is `DROP TABLE og_key_index` by hand,
not a boot step. Forward again: the new binary treats whatever the table
holds as unvalidated and repairs it online.

## Heresy delta

- discovered: the earlier fix design's "four writers" was six; the projector
  does not take the engine mutex; a new table makes an old binary
  replay-ineligible; `content_hash` freshness is only a convention.
- introduced: none yet.
- repaired: none yet.
