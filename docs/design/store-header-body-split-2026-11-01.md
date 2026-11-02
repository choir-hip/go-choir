# Choir store header/body split: design (S1)

Status: design, read-only. Source: worktree `agent-a68a1d96eb5bf63ea` at `2ec066cdb`. Inputs:
- the goal, `docs/definitions/choir-store-header-body-split-2026-11-02.md`
- the S0 evidence, `docs/evidence/store-layout-reads-2026-11-01.md`
- `CLAUDE.md` Operating Tempo
- the 46 files below

Four things I found while reading change the plan:
1. **The witness reads every stored column in primary-key order.** The witness is `computerversion/sqlite_state_extractor.go`. A new `INTEGER PRIMARY KEY` would change the og_objects and choir_events digests. The rowid can also differ between the live store and a replay, so live-vs-replay equivalence would break.
2. **The host gives a booting guest 20 seconds to report progress, or it fails the boot.** The value is `vmmanager` `BootReadyTimeout`. The store opens before the listener, so a 20-40 minute migration with no health surface would be killed and redone forever.
3. **VACUUM / VACUUM INTO may renumber implicit rowids.** Compaction uses VACUUM INTO, so the body key must be an explicit `INTEGER PRIMARY KEY`.
4. **The tape stamp string `"choir_events"` is the same constant as the table name** (`choirEventsTable`). The tape format is excluded, so the header table keeps that name.

---

## 0. Decisions in one place

| Decision | Choice | Reason |
|---|---|---|
| Header table names | Keep `og_objects` and `choir_events` | About 30 header readers stay unchanged. The `"choir_events"` tape stamp must not change. Partitions keep the same table name. |
| Body table names | `og_bodies`, `choir_event_bodies` (in each partition file too) | |
| Body key | The header's explicit rowid alias: `obj_id INTEGER PRIMARY KEY` / `ev_id INTEGER PRIMARY KEY`. The body table is a plain rowid table with the same `INTEGER PRIMARY KEY`. | S0 measured this layout. WITHOUT ROWID is forbidden. An explicit alias survives VACUUM and VACUUM INTO. An implicit rowid does not. |
| Is the rowid state? | **No.** It is a physical address inside one file. It is never in the witness, the fingerprints or the tape, and never compared between stores. | Live, replayed and migrated stores assign rowids differently. |
| Delete cascade | `AFTER DELETE` trigger on each header deletes the body row | Independent of the per-connection `foreign_keys` pragma (migration connection, test openers). Covers every delete path (retire, vocab drop, `DeleteObject`, partition above-head delete) with no call-site change. |
| Header-only derived values | Stay `VIRTUAL` generated columns (`og_meta_*`, `og_rec_inbox*`, `og_rec_inbox_seq`, new `event_kind` on og_objects) | Their inputs are all in the header. Nothing is stored, nothing to migrate, no witness impact. |
| Body-derived values | Become plain typed header columns that every writer fills in the same statement, using the same SQL expressions as today (`og_rec_claim`, `event_phase`) | A generated column cannot read another table. One SQL expression in every path (live, replay, vocab rewrite, migration) means equal values by construction. |
| Migration shape | New file beside the old one: copy everything, split the two tables, swap with renames; the old file is kept as `state.db.presplit` | Peak disk is 2x live and falls to 1x when presplit is removed (an in-place split stays at 2x with a freelist). The new file is compact. The whole old file is a clean rollback (old binary + tape tail). |
| Version marker | `store_meta.schema_version` 1 -> 2 inside the file. `storeschema.Version` 5 -> 6 and floor 6 in the sidecar. Detection is structural: `og_objects` has a `body` column, so the store is old layout. | An old binary refuses through `store_meta`, and the updater fences on the floor. |

---

## 1. Inventory (46 files)

Role key: **W** writer, **H** header reader, **B** body reader, **M** migration, **F** fingerprint/witness, **R** replay, **I** index/schema, **C** comment only.

### internal/objectgraph (slice A)

| File | Role | Change |
|---|---|---|
| `sql_store.go` | I, W, H, B | New DDL (section 2). `upsertObjectSQL` becomes exported `ObjectHeaderUpsertSQL` (with `RETURNING obj_id`) plus `ObjectBodyUpsertSQL`, and a new `WriteObjectTx(ctx, x, obj) (objID int64, err)` used by `PutObject`, `putBatchTx` and the projector. `DeleteObject` stays a header delete (the trigger removes the body). Every full-row reader joins the body: `GetObject`, `ListObjects`, `ReadObjectSnapshot`, `snapshotFetchByID`, `GetObjectByMetadata`, `metadataListSQL`/`metadataPageSQL` (+ JSON variants), `ListObjectsPage`, `ObjectsByKindOwnerComputerPageSQL`, `ListObjectsByEdgeTo`, `ListObjectsByOwnerAndBody`, `recordsByInboxSeqSQL`, `commitmentClaimSQL`, `ListJSONBodyFieldsByKindOwner`, `DeleteObject`'s intercept read. `ReadObjectSnapshotFiltered` and `ReadTrajectorySnapshot`: phase 1 reads full headers plus `obj_id`; phase 2 reads `og_bodies` only, by `obj_id`, in the same read transaction. **Delete:** `ObjectRef`, `ListObjectRefsByKindOwner`, `ListAllObjectRefsByKindOwner`, `refsByKindOwnerSQL`, `ChoirEventKindUpdatedIndex(+DDL)`, `ChoirEventPhaseUpdatedIndex(+DDL)`, the `ChoirEventPhaseSQL` var, and the three `EnsureSchema` boot steps for those indexes. **Add:** `HeaderRow`, `ListObjectsByKindOwnerWhere`, `BodyDerivedSQL`, `ChoirEventRetireIndex`. `ChoirEventUpdatedIndex` moves into `objectIndexes()`. |
| `derived_keys.go` | I, W, B | `deriveSQL` reads `FROM og_objects JOIN og_bodies USING (obj_id)`, so `fam.expr` text is **unchanged** (`body` resolves to `og_bodies.body`). Add `derivedFamily.needsBody` (addressee family only), **not** included in `markIdentity()`. `ogHeadPageSQL`/`headColumns`/`readValidationIDs`/count/outside-range stay header-only; only the "precedes body" comments change. |
| `derived_key_marks.go` | C | No code change. A golden test pins `markIdentity()` for all four families, so marks stay trusted across the migration. |
| `newest_headers.go` | H | No SQL change (`NewestObjectHeadersSQL`, `ObjectKindExistsSQL`, `KindIDRangePageSQL`, `KindMetadataPageSQL`, `KindOutsideRangeSQL` are header-only by construction now). Add header-only plan tests. |

### internal/store, event side (slice B)

| File | Role | Change |
|---|---|---|
| `schema.go` | I | Replace the `choir_events` DDL and indexes with the shared `choirEventsDDL` (section 2). `storeSchemaVersion = 2` (owned by slice 0). |
| `event_table.go` | W, H, B, R | `insertChoirEventRow` becomes a header upsert `... RETURNING ev_id` plus a body upsert, with `event_phase` computed in the header statement. For `keepExisting` (DO NOTHING), no returned row means skip the body. The four union halves (`eventTableRunAfterSQL`, `OwnerAfter`, `OwnerNewest`, `TrajectoryNewest`) select `h.canonical_id, h.updated_at, b.body FROM choir_events h LEFT JOIN choir_event_bodies b ON b.ev_id = h.ev_id`, ordered by `h.` columns. Seeds (`RunMaxSeq`, `NewestStream`, `NextOwner`, `OwnerMaxStream`) and `eventTableAnyRowSQL` stay header-only. Split the constant into `choirEventsStamp` (tape, `"choir_events"`) and `choirEventsTable` (SQL). |
| `telemetry_partition.go` | I, W, H, B, M | `telemetryPartitionDDL` becomes the same `choirEventsDDL`. `openFile` calls `migratePartitionSplit(db)` (from slice D) **before** the DDL. The above-head delete and `flushOne` deletes are unchanged (the trigger cascades). `readStoredRows` and `citedEventIDs` join bodies. `partitionRetireWhere` uses the typed `event_phase`. |
| `object_retire.go` | W(delete), H | `retireOGTable` becomes `og_objects INDEXED BY idx_og_objects_choir_event_retire`; `retireOGTableFor` collapses into it. `retireOGWhere`: `object_kind = 'choir.event' AND event_kind IN (…) AND updated_at >= ? AND updated_at < ?` plus `event_phase = ''` or `<> ''`. `retireTableWhere` does the same on choir_events. `retirePhasePredicate` loses its body/meta arguments. The `retireKeepCitedPredicate` fallback becomes `FROM og_objects ra JOIN og_bodies rb ON rb.obj_id = ra.obj_id, json_each(…rb.body…)`. `retireOldest` keeps `ChoirEventUpdatedIndex`. |
| `telemetry_sealer.go` | R | `choirEventsStamp` rename only. |
| `stream_seq_lease.go` | H | Constant rename only. The exact seed is already header-only. |

### internal/store, object side (slice C)

| File | Role | Change |
|---|---|---|
| `project.go` | W, R | Delete the duplicate `ogObjectUpsertSQL`. `projectObject` calls `objectgraph.WriteObjectTx` through two prepared statements (`projectStmts.objectHeaderStmt()`, `objectBodyStmt()`). `derived.Maintain` is unchanged. |
| `graph_store.go` | H then B | `ListRecentRunsByOwner`, `ListPassivatedPersistentManagementControlRunsByOwner` and `ListRunsByOwnerStates` currently do refs, filter, then `GetObject` per row. They become `graph.ListObjectsByKindOwnerWhere(ctx, kind, owner, limit, keep)`. |
| `store.go` | H then B | The latest-run-by-agent read (line ~1185) uses the same helper. Union-reader comments are touched up. |
| `inbox.go` (not in grep) | H then B | `openConversationHeads` uses the helper with `limit = -1`. |
| `vocab_migrate_og.go` | M, W, B | `planOGMigration` scan, the convergence check (l.662), the live/old self-join (l.730) and the revert reads (l.998) all join bodies. `UPDATE og_objects SET canonical_id … body …` (l.778, l.1045) becomes a header UPDATE that recomputes `og_rec_claim`/`event_phase` via `BodyDerivedSQL` with `RETURNING obj_id`, then `UPDATE og_bodies SET body = ? WHERE obj_id = ?`. The DELETE (l.770) is unchanged. `objectRowTables` becomes `[]objectRowTable{{Header, Body, Key}}`, and `servingRoleFieldsOf` joins per entry. |
| `vocab_replay_fence.go` | R, B | The by-id read (l.230) and the by-tape-position read (l.250) join bodies; uses the new `objectRowTables`. |
| `residue_import.go` | M, B | The full-row read (l.265) joins. The edges `EXISTS` (l.295) is header-only; unchanged. |
| `object_graph_fingerprints.go` | F | The og half selects `h.… , COALESCE(b.body,'') … FROM og_objects h LEFT JOIN og_bodies b … ORDER BY h.canonical_id`. The choir_events half does the same, by tape position. The partition half uses `readStoredRows`. **Output byte-identical.** |
| `vocab_migrate.go` | C | JSON tag `"og_objects"` in the report format: no change. |
| `telemetry_stats.go`, `telemetry_retire.go`, `derived_keys.go` (store), `run_acceptance.go`, `computer_setting.go` | C | No change. |
| `open.go` | M (glue) | `openStore` calls `migrateHeaderBodySplitIfNeeded(dbPath, nil)` after `CheckFloor` and before `dbx.OpenSQLite` (slice D). This covers Open, Reopen and restore. |
| `testdata/fc-deploy1-rollback/fc_deploy1_rollback.go` | C | Dolt-era recipe: no change. |

### Other packages

| File | Role | Change |
|---|---|---|
| `internal/storeschema/schema.go` | I | `Version = 6`, plus an epoch-6 doc entry (slice D). |
| `internal/storeschema/floor.go` | C | No change. |
| `internal/agentcore/replay_eligibility.go` | R | No entries needed when the body tables are logical members (section 5). Slice E adds a test that a split store classifies eligible. |
| `internal/agentcore/replay_object_graph.go`, `runtime.go`, `event_table.go` | C | No change. |
| `internal/types/event_storage.go`, `internal/eventtablegrant/grant.go`, `internal/vmmanager/event_table_grant.go`, `internal/vmmanager/manager.go`, `internal/vocabmigrate/families.go` | C | No change. |
| `internal/platform/` (10 files): `publication_opinion.go`, `store.go`, `capture_index.go`, `objectgraph_store.go`, `corpus_schema.go`, `publication_front.go`, `publication_related_backfill.go`, `retrieval_schema.go`, `artifact_gc.go`, `publication_related.go` | n/a | **Postgres corpus (Store B): excluded, no change.** It shares no SQL with `objectgraph.SQLStore`. |

### Files outside the grep that must change

- `internal/computerversion/sqlite_state_extractor.go`: add `LogicalTables` (slice E).
- `internal/agentcore/restore_snapshot.go`: `replayStateExtractor` gets the logical tables (slice E).
- `internal/store/compact.go`: discard an old-layout staged copy instead of swapping it. Do not prepare a compact copy while `.presplit` exists (slice D).
- `internal/autoputer/run.go`: the early split health gate (slice D).
- New files:
  - `internal/store/header_body_split.go`
  - `internal/dbx/dbxplan/plan.go`
  - `cmd/store-split-diff/main.go`
  - `scripts/rollback/header-body-split-rollback.sh`
  - `internal/store/testdata/presplit/schema.sql` (the frozen old DDL)
- `scripts/cutover/keepset-snapshot.sh` reads header columns only and runs unchanged. Run it before and after.

---

## 2. Schema

### og_objects (header)

```sql
CREATE TABLE IF NOT EXISTS og_objects (
	obj_id        INTEGER PRIMARY KEY,
	canonical_id  TEXT NOT NULL UNIQUE,
	object_kind   TEXT NOT NULL,
	owner_id      TEXT NOT NULL,
	computer_id   TEXT NOT NULL DEFAULT '',
	version_id    TEXT NOT NULL DEFAULT '',
	content_hash  TEXT NOT NULL,
	metadata      TEXT NOT NULL,
	created_at    DATETIME NOT NULL,
	updated_at    DATETIME NOT NULL,
	tombstone     INTEGER NOT NULL DEFAULT 0,
	superseded_by TEXT NOT NULL DEFAULT '',
	og_rec_claim  INTEGER,   -- body-derived; writer fills; NULL unless choir.commitment_record
	event_phase   TEXT,      -- body-derived; writer fills; NULL unless choir.event, else '' or the phase
	-- unchanged VIRTUAL columns from metadataIndexColumns (generatedColumn() text unchanged):
	og_meta_run_id, og_meta_trajectory_id, og_meta_channel_id,
	og_rec_inbox, og_rec_sender_inbox, og_rec_conversation_id,   -- TEXT … GENERATED ALWAYS AS (…) VIRTUAL
	og_rec_inbox_seq INTEGER GENERATED ALWAYS AS (…unchanged…) VIRTUAL,
	event_kind GENERATED ALWAYS AS (CASE WHEN object_kind = 'choir.event' THEN
	           CASE WHEN json_valid(metadata) THEN json_extract(metadata, '$.kind') END END) VIRTUAL
	           -- deliberately NO declared type: no affinity, so `IN (?)` compares exactly as the old expression index did
);
```

`canonical_id TEXT NOT NULL UNIQUE` replaces `TEXT PRIMARY KEY`. In a rowid table these are physically the same: an autoindex plus a rowid lookup. `ON CONFLICT (canonical_id)` works against the UNIQUE constraint.

### og_bodies (body)

```sql
CREATE TABLE IF NOT EXISTS og_bodies (
	obj_id INTEGER PRIMARY KEY,
	body   BLOB
);
CREATE TRIGGER IF NOT EXISTS og_objects_drop_body AFTER DELETE ON og_objects
BEGIN DELETE FROM og_bodies WHERE obj_id = OLD.obj_id; END;
```

Invariant: every header has exactly one body row (a NULL body is stored as a NULL `body`).

### choir_events (header and body; the same text in the main store and in every partition file)

`choirEventsDDL` in `internal/store/schema.go` is shared by `schemaDDL` and `telemetryPartitionDDL`.

```sql
CREATE TABLE IF NOT EXISTS choir_events (
	ev_id INTEGER PRIMARY KEY,
	tape_seq INTEGER NOT NULL,
	op_index INTEGER NOT NULL,
	canonical_id TEXT NOT NULL,
	object_kind TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	computer_id TEXT NOT NULL DEFAULT '',
	version_id TEXT NOT NULL DEFAULT '',
	content_hash TEXT NOT NULL,
	metadata TEXT NOT NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	tombstone INTEGER NOT NULL DEFAULT 0,
	superseded_by TEXT NOT NULL DEFAULT '',
	run_id TEXT NOT NULL DEFAULT '',
	seq INTEGER NOT NULL DEFAULT 0,
	stream_seq INTEGER NOT NULL DEFAULT 0,
	trajectory_id TEXT NOT NULL DEFAULT '',
	event_kind TEXT NOT NULL DEFAULT '',
	event_phase TEXT NOT NULL DEFAULT '',
	UNIQUE (tape_seq, op_index)
);
CREATE TABLE IF NOT EXISTS choir_event_bodies (
	ev_id INTEGER PRIMARY KEY,
	body  BLOB NOT NULL
);
CREATE TRIGGER IF NOT EXISTS choir_events_drop_body AFTER DELETE ON choir_events
BEGIN DELETE FROM choir_event_bodies WHERE ev_id = OLD.ev_id; END;
CREATE INDEX IF NOT EXISTS idx_choir_events_run_seq ON choir_events(run_id, seq);
CREATE INDEX IF NOT EXISTS idx_choir_events_owner_stream ON choir_events(owner_id, stream_seq);
CREATE INDEX IF NOT EXISTS idx_choir_events_trajectory_stream ON choir_events(trajectory_id, stream_seq);
CREATE INDEX IF NOT EXISTS idx_choir_events_updated ON choir_events(updated_at);
CREATE INDEX IF NOT EXISTS idx_choir_events_kind_updated_phase ON choir_events(event_kind, updated_at, event_phase);
```

`ev_id` cannot be made a function of the tape position (for example `tape_seq<<16 | op_index`): projection batches have no op-count bound (`computerevent/projection_batch.go`), so a historical tape could collide. Hence an assigned `ev_id` plus `UNIQUE(tape_seq, op_index)`.

### Body-derived expressions

`objectgraph.BodyDerivedSQL(kind, body, meta string) (claim, phase string)` returns the two SQL expressions over the given references.

- **claim** is today's `recordColumns[1].Expr` with `body` replaced by the argument:

  `CASE WHEN <kind> = 'choir.commitment_record' THEN CASE WHEN <body> IS NULL OR json_valid(CAST(<body> AS TEXT)) THEN <recordClaimPredicate over CAST(<body> AS TEXT)> END END`

- **phase** is `CASE WHEN <kind> = 'choir.event' THEN ChoirEventPhaseSQLFor(<body>, <meta>) END`. The function is kept and is now the only phase definition.

Callers pass `?N` numbered parameters in the writers (the body is referenced several times), and column references in the migration (`o.object_kind`, `o.body`, `o.metadata`). For choir_events, phase is `ChoirEventPhaseSQLFor(?N, ?M)` with no kind gate, because the table holds only `choir.event` rows.

### og_objects indexes

| Index | Fate |
|---|---|
| `idx_og_objects_updated(updated_at)` | keep (newest headers) |
| `idx_og_objects_kind_id(object_kind, canonical_id)` | keep (`ListObjectsPage`) |
| `idx_og_objects_owner_computer_kind(…, canonical_id)` | keep (snapshot) |
| `idx_og_objects_kind_owner_computer(…, canonical_id)` | keep (scope pages) |
| `idx_og_objects_kind_owner_updated(object_kind, owner_id, updated_at)` | keep (header newest, now truly header-only) |
| 6 `idx_og_objects_og_meta_*` / `og_rec_*` equality, 3 `_recent` | keep (equality seeks; not body dodges) |
| `idx_og_objects_rec_inbox_seq` | keep |
| `idx_og_objects_rec_claim(object_kind, owner_id, og_rec_claim, updated_at, canonical_id) WHERE og_rec_claim IS NOT NULL` | keep, now on the typed column |
| `idx_og_objects_choir_event_updated(updated_at) WHERE object_kind='choir.event'` | keep (for `retireOldest`), moved into `objectIndexes()` |
| **new** `idx_og_objects_choir_event_retire(event_kind, updated_at, event_phase) WHERE object_kind='choir.event'` | replaces the two below. The phase is tested on the index entry before any header lookup. |
| `idx_og_objects_choir_event_kind_updated` (expression) | **delete** |
| `idx_og_objects_choir_event_phase_updated` (expression) | **delete** |

On "json generated columns used only to dodge body reads": the only body-dodging keys are the two expression-index keys, and they are deleted. `og_rec_claim` stays, because it answers a body predicate and the body is now in another table, but it becomes a typed column. The metadata VIRTUAL columns serve equality seeks and stay as they are.

ANALYZE must never run, in the migration or anywhere else. The plans and `INDEXED BY` assume no statistics.

---

## 3. Write path

**Objects.** `objectgraph.WriteObjectTx(ctx, x, obj)` runs inside the caller's transaction:
1. `ObjectHeaderUpsertSQL`: `INSERT INTO og_objects (canonical_id, …, superseded_by, og_rec_claim, event_phase) VALUES (?1…?11, <claim(?2,?12,?6)>, <phase(?2,?12,?6)>) ON CONFLICT (canonical_id) DO UPDATE SET …, og_rec_claim = excluded.og_rec_claim, event_phase = excluded.event_phase RETURNING obj_id`. Here `?12` is the body: bound for evaluation only, never stored in the header.
2. `ObjectBodyUpsertSQL`: `INSERT INTO og_bodies (obj_id, body) VALUES (?, ?) ON CONFLICT (obj_id) DO UPDATE SET body = excluded.body`.

The function is used by:
- `SQLStore.PutObject`, which now always opens a transaction (two statements need one)
- `putBatchTx`
- `store.projectObject`, through two prepared statements in `projectStmts`

`derived.Maintain` stays after both statements in the same transaction.

**Rowid stability.**
- `ON CONFLICT … DO UPDATE` keeps the row and its `obj_id`. A vocab rename (`UPDATE … SET canonical_id`) keeps `obj_id` too.
- REPLACE would delete and reinsert, giving a new rowid and orphaning or misattaching the body. It is **forbidden** on the four tables. A static guard test (`TestNoReplaceOnSplitTables`, in A and B) greps `internal/objectgraph` and `internal/store` for `REPLACE INTO` / `OR REPLACE` / `ON CONFLICT REPLACE` near these table names.
- An explicit `INTEGER PRIMARY KEY` survives VACUUM INTO, which compaction uses.
- Rowid reuse after deleting the max row is harmless inside one transaction. Two-phase reads therefore run both phases in **one read transaction** (section 4).

**Events.** `insertChoirEventRow(ctx, x, pos, idx, row, keepExisting)` does the header upsert on `(tape_seq, op_index)` with `event_phase = ChoirEventPhaseSQLFor(?body, ?meta)`, `RETURNING ev_id`, then the body upsert. With `keepExisting` (`DO NOTHING`), no returned row means the body is left untouched. The same function serves the main store, partitions (`flushOne`) and the cited-row move.

**Deletes and retirement.** Every `DELETE FROM og_objects|choir_events` (retire slices, keyed-family chunk delete, `DeleteObject`, vocab drop, partition above-head delete, partial partition retire) cascades through the trigger. No call site deletes bodies itself.

**Telemetry partitions** are separate files, not ATTACHed. Each gets the same DDL and trigger. The in-place migration runs in `openFile` (section 6).

---

## 4. Read path

**Join shape for every body reader:**

`FROM og_objects h LEFT JOIN og_bodies b ON b.obj_id = h.obj_id`, selecting `b.obj_id` alongside `b.body`. The choir_events equivalent is `choir_events h LEFT JOIN choir_event_bodies b ON b.ev_id = h.ev_id`.

- LEFT JOIN pins the header as the outer loop, so the planner never scans bodies first.
- `scanObject` returns an error when `b.obj_id IS NULL` (missing body), so a missing body fails visibly instead of reading as a NULL body.
- Only plain column references go in the select list. The driver reads the declared type through the join, so DATETIME columns still arrive as `time.Time`.

**Two-phase reads, redesigned.** Today's pattern is refs, filter in Go, then `GetObject` per id (a second header lookup plus the body). The replacement:

`ListObjectsByKindOwnerWhere(ctx, kind, owner string, limit int, keep func(HeaderRow) bool) ([]Object, error)`. One read-only transaction:
1. Phase 1: `SELECT obj_id, <header cols> FROM og_objects WHERE object_kind=? AND owner_id=? ORDER BY updated_at DESC [LIMIT ?]` (no LIMIT when `limit < 0`).
2. `keep` runs in Go.
3. Phase 2: `SELECT obj_id, body FROM og_bodies WHERE obj_id IN (…256…)`, then attach each body to its header.

`ReadObjectSnapshotFiltered` and `ReadTrajectorySnapshot` phase 2 use the same body-only fetch. `snapshotFetchByID` (follow rounds and derived-key reads) is a join by `canonical_id`. `HeaderRow` is `{ObjID int64; Object}` with `Body` nil. It is never marshalled; `Object` itself is unchanged because it is the tape payload.

**Plan tests.** New helper package `internal/dbx/dbxplan`, imported only by tests:
- `HeaderOnly(t, db, query, args...)`: no plan line names `og_bodies` / `choir_event_bodies`, and the expected index is named (reuses `assertIndexedPlan` logic).
- `BodyByRowid(t, db, bodyAlias, query, args...)`: every line about the body alias is `SEARCH <alias> USING INTEGER PRIMARY KEY (rowid=?)`; there is no `SCAN <alias>`; the header line comes first.
- `NoIndexWalkCensus(t, db, query)`: for whole-table header answers, the header is `SCAN og_objects` (or a covering-index scan), never `SCAN … USING INDEX` with a lookup per row.

Table-driven tests:
- **A** (`objectgraph/split_plan_test.go`):
  - HeaderOnly: `NewestObjectHeadersSQL`, `ObjectKindExistsSQL`, the three kind pages, `trajectoryRefsSQL`, snapshot phase 1, `ListObjectsByKindOwnerWhere` phase 1, `checkConditions`, `ogHeadPageSQL` (both modes), the `readValidationIDs` query, the derived count, `CountObjectsByEdgeTo`.
  - BodyByRowid: every `objectColumnsSQL` reader, phase 2, `ListJSONBodyFieldsByKindOwner`, `ListObjectsByOwnerAndBody`, `deriveSQL` (addressee).
- **B** (`store/split_plan_test.go`):
  - HeaderOnly: retire selection (all three phase modes), `retireMatchExists` on both tables, `retireOldest`, `partitionRetireWhere`, the four seeds, `eventTableAnyRowSQL`, the above-head selection.
  - BodyByRowid: the four union halves, `readStoredRows`, `citedEventIDs`, the cited fallback.
  - The existing `TestObjectRetireSlicePlanUsesThePartialIndex` and `TestObjectRetireEventTablePlanUsesTheUpdatedIndex` are updated to the new index names.
- **Allowlisted** index walks for whole-table answers, each with its reason in the test table: `ListObjectGraphFingerprints` (diagnostic, needs `canonical_id` order), the witness logical query, `ReadObjectSnapshot` (`engineering_evidence.go`, rare).

---

## 5. Replay, fingerprints, derived keys: what stays byte-identical

| Must stay | How |
|---|---|
| Tape (ops, the `"choir_events"`/`"telemetry_partition"` stamps, object JSON) | `Object` struct unchanged. Stamp constant split from the table constant. No ProjectionOp change. |
| **Witness per-table content digests** for every table, including `og_objects` and `choir_events` | `SQLiteStateExtractor` gets `LogicalTables map[string]LogicalTable{Query string; Members []string}`. When all members exist, the logical table's digest runs `Query` instead of the generic select. Members are skipped from `Tables`/`TableSchemas` (their schema entries still go into the overall schema hash and into the logical table's schema digest). Old-layout files (no member table) take the generic path, so `VerifySnapshotFile` on pre-split snapshots still works. |
| | og_objects query: `SELECT h.canonical_id, h.object_kind, h.owner_id, h.computer_id, h.version_id, h.content_hash, b.body, h.metadata, h.created_at, h.updated_at, h.tombstone, h.superseded_by FROM og_objects h LEFT JOIN og_bodies b ON b.obj_id = h.obj_id ORDER BY h.canonical_id`. These are exactly the old stored columns, in declaration order and PK order. `obj_id`, `og_rec_claim`, `event_phase` and the VIRTUAL columns are excluded. |
| | choir_events query: the old 19 columns (`event_kind` included, `ev_id`/`event_phase` excluded), with the body in position 9, `ORDER BY h.tape_seq, h.op_index`. |
| | Definitions are exported as `objectgraph.WitnessLogicalTables()` and `store.WitnessLogicalTables()`, merged in `agentcore.replayStateExtractor`. Result: the schema hash and content root change once at the cut, and **every table digest is equal before and after**. |
| `ListObjectGraphFingerprints` output | Same values and order; joins only. |
| Derived keys (`og_key_index` rows, marks) | The table is copied verbatim by the migration. The projection head is unchanged. `fam.expr` text is unchanged, so `markIdentity()` is unchanged and `TrustDerivedKeyMarks` trusts the marks at the first split boot (no validation pass). Golden test on `markIdentity()`. |
| Retire selections (which rows an op deletes, live and on replay) | Typed columns are computed by the same SQL expressions that the old predicates evaluated. `event_kind` has no affinity. |
| `vocab_replay_fence` decisions | Same rows read; joins only. |
| Replay from tape | Replay into `OpenFresh` uses the new DDL directly (no migration). Equivalence is "replay witness == live witness", and that now ignores rowids. |

**The differential proves it with three layers:**
1. Witness digests: each old-file table digest equals the matching new-file digest.
2. Derived-column recomputation: `event_phase`/`og_rec_claim` equal `BodyDerivedSQL` over the joined body for every row.
3. A tape replay into a fresh store at the migrated head has the same witness as the migrated store.

Details in section 8.

---

## 6. Migration (at boot, marker-backed, in `internal/store/header_body_split.go`)

**Entry points.**
- `migrateHeaderBodySplitIfNeeded(dbPath string, progress func(uint64)) error` is called from `openStore`. That covers Open, Reopen (restore or base install of an old snapshot) and every boot.
- The exported `MigrateHeaderBodySplit(dbPath, progress)` is called earlier by `autoputer/run.go`, under the early health gate. It runs right after `compact-rebase` and before `runtime-store-open`.
- Detection: the state file exists, `dbx.ColumnExists(og_objects, body)` is true, and no completed staging file exists.
- The migration runs on crash and planned boots alike. It is boot infrastructure: this binary cannot serve the old layout.

**Early health gate (required).** `waitForGuestReady` fails a guest that has not reported progress within `BootReadyTimeout` (20 s), and a replaying guest after `ReplayStallTimeout` (120 s) with no advance. The in-guest updater probe parses the same body.
- Before migrating, `autoputer` binds the guest listen address with a minimal handler. It answers `503 {"status":"replaying","sequence":0,"committed_sequence":0,"progress":N}`, where N is rows copied plus one tick per 5 s while a single long statement (an index build) runs.
- It closes the handler before the main server binds.
- A preflight refusal or an exhausted attempt counter is served as the typed `503 {"status":"refused","kind":"store_header_body_split",…}`, so the host fails fast with a witness.

**Steps.** Each step is a `BootPhase("store: open", "header-body-split/<step>")`.
1. `preflight`:
   - Discard `.compact`, `.compact.json` and `.compact-staging`: they are old layout, and the new file is compact anyway.
   - Read `page_count`, `freelist_count` and free bytes (`compactFreeBytes`).
   - Require free ≥ live bytes × 1.05 + 1 GiB (sort temp) + `compactFreeFloorBytes` (2 GiB). Otherwise refuse **before any mutation**. No floor or receipt has been written yet, so the updater's `restorePrior` can still return the old binary cleanly.
   - Log `live=… free=… need=…`.
   - Read the attempts file `state.db.split-attempts`. At 3 or more, refuse.
2. `checkpoint-old`: open the old file with dbx, run `PRAGMA wal_checkpoint(TRUNCATE)`, close.
3. `stage`:
   - Create `state.db.split-staging` with its own `sql.Open` (not dbx), like `vacuumIntoCopy`: `page_size=4096`, `auto_vacuum=INCREMENTAL`, `journal_mode=OFF`, `synchronous=OFF`, `cache_size=-65536`, `temp_store=FILE`. Set `SQLITE_TMPDIR` to the state directory: the guest `/tmp` may be RAM.
   - `ATTACH` the old file as `old`; it is never written.
4. `copy-schema`: for every `old.sqlite_master` entry (table, index, trigger, view) not on `og_objects`/`choir_events`, execute its `sql` verbatim. Tables first, rows next, indexes last. Copy `sqlite_sequence` rows as well.
5. `copy-tables/<name>`: `INSERT INTO main.T SELECT * FROM old.T` for each such table. This includes `og_key_index`, `computer_event_projection_heads` and `run_memory_entries`. Those tables are small next to the bodies.
6. `copy-og_objects`:
   - Create the header (without secondary indexes), `og_bodies` and the trigger from the new DDL.
   - In rowid ranges of 256 rows, two statements per batch:
     - `INSERT INTO main.og_objects(obj_id, …, og_rec_claim, event_phase) SELECT rowid, …, <claim(o.object_kind,o.body)>, <phase(o.object_kind,o.body,o.metadata)> FROM old.og_objects o WHERE rowid > ? AND rowid <= ?`
     - `INSERT INTO main.og_bodies SELECT rowid, body FROM old.og_objects WHERE rowid > ? AND rowid <= ?`. The second read of the batch is served from cache.
   - `obj_id` = old rowid: deterministic, and keeps the physical order.
   - Tally rows read and inserted. Log progress every 10 s: `rows=1,200,000/3,340,000 cursor/maxrowid elapsed rate`. `MAX(rowid)` is cheap: the rightmost leaf.
7. `copy-choir_events`: the same, with `ev_id` = old rowid and `event_phase` computed.
8. `build-indexes/<name>`: every secondary index from the new DDL, one at a time (header-only scans). The progress ticker keeps the gate alive. Each step has a 20-minute context deadline.
9. `verify`:
   - Tallies equal: old rows equal header rows equal body rows per table.
   - A 1,000-row sample (random rowids) compares the old row byte-for-byte with the joined new row.
   - Set `store_meta.schema_version='2'`.
   - Switch to `journal_mode=WAL`, close.
   - Write `state.db.split-staging.json` durably, recording `{source_head_sequence, source_head, sizes, counts, durations}`.
   - No `integrity_check`: on the paper it took 58 minutes for 20.66 GB. The full check is the offline differential.
10. `swap`:
    - Re-read the old projection head. If it differs from the sidecar's, an old binary ran in between: discard and redo.
    - Rename `state.db` to `state.db.presplit` (`moveWAL` first), rename the staging file to `state.db`, `syncDir`.
    - Then write the floor (6) and the `.layout.json` marker, so compaction is not due immediately.
    - `WriteLeaseUntrusted` is not needed: the lease row is copied and the head is unchanged.
11. `partitions`: done later, in `openTelemetryPartitions` → `openFile` → `migratePartitionSplit`, one transaction per file:
    - Rename to `choir_events_presplit` and drop its five indexes.
    - Create the new DDL and copy (with phase).
    - Drop the presplit table, commit, then `VACUUM` the partition file.
    - Partitions hold one day of `tool.result`; peak is 2x one file. Retired files are not opened and not migrated.

**Crash safety** (every case resumes or redoes safely):
- Crash before the staging sidecar exists: the next boot deletes the staging files, increments attempts and redoes. With journal OFF, a partial staging file is untrusted.
- Staging sidecar present, `state.db` old layout, head equal to the sidecar's: do the swap.
- `state.db` missing, `.presplit` present: finish the swap if the staging file and its sidecar are intact, else undo (rename presplit back). This mirrors `RebaseOntoCompactCopy`.
- `state.db` new layout: done.
- Between swap and floor: an old binary refuses through `store_meta` version 2 > 1.

**`.presplit` lifecycle.**
- Kept, per the goal's rollback rule ("keeps the old tables until the differential is green").
- Removed by a host-peer route `POST /internal/store/split-finalize` (pattern of `/internal/diag/drill`), which the ring tooling calls after the differential is green.
- Removed automatically, with a loud log and an observability flag, if free space falls below `compactFreeFloorBytes`.
- `PrepareCompactCopy` refuses while it exists.
- Rollback: `scripts/rollback/header-body-split-rollback.sh`:
  1. Stop the runtime.
  2. Move `state.db` aside and `state.db.presplit` to `state.db`.
  3. Rewrite `store-floor.json` to 5 (a manual recipe: floors never lower in code).
  4. Install the prior release.

  The old binary boots at the presplit head and replays the tape tail. The tape format is identical.

**Peak disk.** Old file plus new file plus temp sort space. Paper: about 21.45 + ~20.7 + ≤1 ≈ **43 GB peak**, falling to ~20.7 GB after finalize.

**Memory bound.** 64 MiB SQLite cache plus 64 MiB sorter before it spills to files. Bodies never pass through Go: the copy is INSERT…SELECT. The gate and ticker are trivial. Peak RSS stays under ~200 MiB above baseline.

**Time estimate (paper).**
- About 21 GB read plus ~20 GB written with journal OFF at ~45-75 MB/s: 9-15 min.
- Phase JSON evaluation over ~2.5M event bodies: 1-3 min CPU.
- Around 17 header index builds over a ~0.6 GB header table: 2-4 min.
- Other tables: about 1 min.
- **Expected 15-25 min; budget 45 min.** S3 measures it on a paper copy, and that number gates S4. d@e is proportionally smaller.

**Boot log summary.** One line `store: header-body-split: done rows og=… ev=… bytes=… duration=… presplit=…`. The same fields go into `state.db.presplit.json` and `/api/runtime/observability` as `store.split`.

---

## 7. Build split (parallel slices by package)

**Slice 0 (Opus, contract, first).**
- Freeze today's DDL into `internal/store/testdata/presplit/schema.sql` (`objectgraph.SchemaDDL` plus the three choir-event index DDLs, store `choir_events` plus indexes, partition DDL) for the old-layout fixtures.
- Land the new DDL constants and exported signatures with stubs: `BodyDerivedSQL`, `ObjectHeaderUpsertSQL`, `ObjectBodyUpsertSQL`, `WriteObjectTx`, `HeaderRow`, `ListObjectsByKindOwnerWhere`, `choirEventsDDL`, `WitnessLogicalTables`, `computerversion.LogicalTable`, `migratePartitionSplit`, the `dbxplan` helpers.
- Set `storeSchemaVersion = 2`.

**Slice A: objectgraph** (`sql_store.go`, `derived_keys.go`, new `witness_tables.go`, `internal/dbx/dbxplan/plan.go`). Tests:
- `split_plan_test.go` (section 4)
- `TestWriteKeepsObjIDAcrossUpsertAndRename`
- `TestDeleteDropsBody` (`DeleteObject`, plain DELETE)
- `TestBodyDerivedMatchesOldExpressions`: corpus of non-JSON, JSONB-looking, empty, NULL, numeric-phase and numeric-kind bodies. Compares the new typed and generated values with the old generated and expression values on the frozen DDL.
- `TestMarkIdentityGolden`
- `TestNoReplaceOnSplitTables`
- `TestListObjectsByKindOwnerWhereOneTx` (concurrent delete does not misattach a body)
- existing generated-column, scope-page, newest-headers and derived-key suites updated

**Slice B: store events** (`schema.go` `choir_events` part, `event_table.go`, `telemetry_partition.go`, `object_retire.go`, `telemetry_sealer.go`, `stream_seq_lease.go`, `store/witness_tables.go`). Tests:
- `split_plan_test.go` (section 4)
- `TestRetireSelectsSameRowsAsOldPredicate`: old-layout fixture with the old SQL against migrated rows with the new SQL, across all phase modes, KeepCited on and off, and partial and whole partitions
- `TestPartitionSplitMigration`: old partition fixture, migrate, rows and fingerprints equal, crash mid-transaction leaves it old
- `TestKeepExistingSkipsBody`
- updated object-retire and event-table plan tests

**Slice C: store objects** (`project.go`, `graph_store.go`, `store.go`, `inbox.go`, `vocab_migrate_og.go`, `vocab_replay_fence.go`, `residue_import.go`, `object_graph_fingerprints.go`). Tests:
- `TestFingerprintsByteIdenticalAcrossSplit`
- vocab migrate plus revert round trip on the split layout, including the phase/claim recompute
- run-list and conversation-head readers return the same records as before (fixture)
- replay projector writes both tables (`projectObject`)

**Slice D: migration and boot** (`header_body_split.go`, `open.go`, `compact.go`, `storeschema/schema.go`, `autoputer/run.go`, the rollback script). Tests:
- `TestSplitMigratesFixtureStore`, checking witness table digests equal, counts, the sample, and the `og_key_index` copy
- crash injection hooks after every step, then the next open completes
- head-moved staging discarded
- interrupted swap finished and undone
- preflight refusal leaves the store untouched and floor-free
- attempts cap gives a typed refusal
- Reopen of an old-layout snapshot migrates
- old-layout staged compact copy discarded
- early gate answers `replaying` with advancing progress (autoputer test)

**Slice E: witness, replay, differential** (`computerversion/sqlite_state_extractor.go`, `agentcore/restore_snapshot.go`, `agentcore/replay_eligibility.go` test, `cmd/store-split-diff/main.go`, `scripts/proofs/split-differential.sh`). Tests:
- `TestLogicalTableDigestEqualsOldGenericDigest` (old fixture against its migrated copy)
- `TestOldLayoutFileDigestsGenerically`
- `TestReplayEligibilityOnSplitStore`
- replay-into-fresh witness equals migrated witness on a fixture tape

**Integration order.** Slice 0 → A → (B and C in parallel, both on A's helpers) → E (can start from the contract) → D last (it needs the final DDL). Then the full store and objectgraph suites, `scripts/go-test-runtime-shards`, the CI mirror on `choir-agents` (stagger compiles), then S3.

---

## 8. Risks and the differential check

**Differential** (`cmd/store-split-diff --old state.db.presplit --new state.db`, read-only; run on a paper copy in S3, then on d@e, then on each fleet ring before finalize):
1. **Rows:** counts by `(object_kind, tombstone)` for og_objects, and choir_events by `event_kind`. Header count equals body count in the new file, with no orphan bodies (body `LEFT JOIN` header is NULL → 0) and no missing bodies.
2. **Bodies by hash, and every other column:** witness per-table digests are equal for **all** tables (via the logical tables). On a mismatch, fall back to a per-row fingerprint diff that names the canonical ids.
3. **Derived:**
   - `event_phase`/`og_rec_claim` equal `BodyDerivedSQL` recomputed over the joined body, for every row.
   - The old `event_kind` expression on the old file equals the new column.
   - Retire equivalence: for the last 30 days × each event kind × the three phase modes, the old predicate count on old equals the new predicate count on new.
   - `og_key_index` is byte-equal. The first boot logs the marks as trusted. A `ValidateDerivedKeys` pass on the copy finds 0 differences.
4. **Replay head:** `computer_event_projection_heads` is equal. A tape replay into `OpenFresh` up to that head gives the same witness content root as the migrated store.
5. **Keep set:** `scripts/cutover/keepset-snapshot.sh` before and after (it reads header columns only).
6. **Reads:** the S0 statements on the migrated copy with `/proc/self/io`, plus the plan suite. This feeds the S6 meter.

**Risks:**

| Risk | Mitigation |
|---|---|
| Host or updater kills a migrating guest (20 s / 120 s windows), giving an endless redo | Early `replaying` gate with progress and a ticker; attempts cap with a typed refusal. |
| Disk short | Preflight before any mutation; ring tooling checks free space against live size first; `.presplit` auto-dropped under the floor (rollback is then the pre-migration snapshot, which the ring runbook must take). |
| Sorter temp files on tmpfs eat memory | `SQLITE_TMPDIR` set to the state directory and `temp_store=FILE`. Verify modernc honours it in slice D's test. |
| Planner puts the body table outer, or walks an index for a census | LEFT JOIN, `dbxplan` assertions, no ANALYZE. |
| Rowid reuse between statements | Both phases in one read transaction. Bodies are never fetched by a remembered `obj_id` outside its transaction. |
| Affinity drift on `event_kind` | Untyped generated column, plus the corpus test. |
| Mark identity drift forces a full validation pass at boot | `expr` text unchanged; golden test. |
| modernc lacks `RETURNING` or `?NNN` | SQLite in modernc v1.49 supports both. A slice 0 smoke test pins it. |
| Restoring a pre-split snapshot pays the migration inside Reopen (no gate) | Accepted for S2. Snapshots taken after the cut are new layout. Named as a residual. |
| Write amplification: two B-tree writes per object | Measure the replay rate on d@e against 206-350 events/s. Stop if it drops more than 20%. |
| Witness in `canonical_id` order = index walk plus two lookups | No regression against today's walk plus whole-row lookup. Residual: a future witness version could digest in `obj_id` order. |
| Partition snapshot chunks change once | Accepted. |
| Old binary after the swap | Refused by `store_meta` and the floor; the updater's restorePrior is fenced by the floor. Recovery is forward deploy or the rollback recipe. |

### Critical Files for Implementation
- /Users/wiz/Dev/go-choir/.claude/worktrees/agent-a68a1d96eb5bf63ea/internal/objectgraph/sql_store.go
- /Users/wiz/Dev/go-choir/.claude/worktrees/agent-a68a1d96eb5bf63ea/internal/store/event_table.go
- /Users/wiz/Dev/go-choir/.claude/worktrees/agent-a68a1d96eb5bf63ea/internal/store/object_retire.go
- /Users/wiz/Dev/go-choir/.claude/worktrees/agent-a68a1d96eb5bf63ea/internal/computerversion/sqlite_state_extractor.go
- /Users/wiz/Dev/go-choir/.claude/worktrees/agent-a68a1d96eb5bf63ea/internal/store/open.go (plus the new internal/store/header_body_split.go and internal/autoputer/run.go for the boot gate)
---

## Deviations (S2 slice 0 and slice A, as built)

1. **`ListObjectsByKindOwnerWhere`'s `keep` returns `(bool, error)`**, not `bool`. Every caller decoded metadata and returned the decode error; a keep error ends the read with it.
2. **Header upsert numbering.** `ObjectHeaderUpsertArgs` binds `?1..?11` in header declaration order (canonical_id, kind, owner, computer, version, hash, metadata, created, updated, tombstone, superseded_by) and `?12` = body. The derived expressions are `BodyDerivedSQL("?2", "?12", "?7")`. `ObjectHeaderUpsertSQL` is a `var` (built from `BodyDerivedSQL`).
3. **Extra exports** beyond section 7: `ObjectHeaderUpsertArgs`, `ObjectBodyUpsertArgs`, the `ObjectWriteTx` interface, `WriteObjectStmts` (the projector's prepared-statement form), `ObjectHeaderTableDDL()`, `ObjectBodyTableDDL`, `ObjectIndexDDL()` (for the migration's create-then-index order), `ErrMissingBody`, `ObjectsWitnessQuery`.
4. **`ListObjectsByEdgeTo` uses `CROSS JOIN`.** With `canonical_id` no longer the primary key, the planner read the whole kind through `idx_og_objects_kind_id` and probed edges per row. `og_edges e CROSS JOIN og_objects h` keeps the edge seek outer; a test pins it.
5. **`object_retire.go` og side landed in slice A** (forced by deleting the expression indexes): `retireOGTable` is `INDEXED BY idx_og_objects_choir_event_retire`, `retireOGWhere` tests `event_kind IN (…)` and the new `retireOGPhasePredicate(p)` on `event_phase`, the keep-cited fallback joins `og_bodies`. `retirePhasePredicate(p, body, meta)` is left for the choir_events side (slice B), as is `telemetry_partition.go` (its DDL is still the old text; `migratePartitionSplit(ctx, *sql.DB)` is a stub nobody calls yet).
6. **Store callers switched in slice A** (mechanical, to keep the tree compiling): `graph_store.go` (three run lists), `store.go` `latestLifecycleRunByAgent`, `inbox.go` `openConversationHeads`. `latestLifecycleRunByAgent` now fetches the bodies of every agent-id match within its scan cap instead of stopping at the first.
7. **`ChoirEventKindSQL` is kept**: it is the `event_kind` expression. `ChoirEventUpdatedIndexDDL` is deleted (the index is in `objectIndexes()`).
8. **Frozen DDL** is the whole `sqlite_master` of a fresh store at 95ea10801 (`testdata/presplit/schema.sql`), plus `partition.sql` (old `telemetryPartitionDDL`).
9. **Non-JSON metadata is refused by both layouts** (the indexed `og_meta_*` generated columns error on insert), so the derived-value corpus uses JSON metadata only; bodies cover NULL, empty, non-JSON, JSONB, text-typed, numeric and null fields.
11. **Slice E (witness, differential), as built.**
    - The logical table's column names come from the query's result (`rows.Columns()`), so `IgnoredContentColumns` applies to a logical table by result-column name. `queryDigest` is the one streaming digest for generic and logical tables.
    - `agentcore.replayStateExtractor` sets `LogicalTables: choirstore.WitnessLogicalTables()`; that function already merges the objectgraph map, so no second merge is needed.
    - New test-fixture package `internal/store/splitfixture` (raw SQL: the frozen old DDL with rows, and a split copy built the way the migration copies) is shared by the computerversion, `store-split-diff` and the other slices' tests; it uses no store writer.
    - `store-split-diff` checks, beyond section 8: a `layouts` check (old has the body column and no bodies table, new the reverse), and the witness run ignores only `og_key_index` (checked separately, row-digest equal) and applies no ignored content columns, so it is stricter than the production witness (`updated_at`/`prepared_at` columns must also be equal after a verbatim copy). The `ValidateDerivedKeys` pass and the replay-into-OpenFresh comparison (section 8, items 3 and 4) are not in the tool: they need the store packages and a tape, and stay S3 steps.
    - Retire equivalence groups by event kind inside one query per (day, phase mode, table) instead of one query per kind; the counts compared are the same. Windows are UTC days ending at `--as-of` (default now).
    - `scripts/proofs/split-differential.sh` runs the keep-set snapshot only when `KEEPSET_BEFORE` names a pre-migration snapshot (the script reads the platform graph database, Node B only).
10. **`dbxplan.HeaderOnly(t, db, wantIndex, query, args...)`** takes the expected index ("" for none); `AssertHeaderOnly`/`AssertBodyByRowid` take a plan already read. A LEFT JOIN body seek reads `SEARCH b USING INTEGER PRIMARY KEY (rowid=?) LEFT-JOIN`.

## Deviations (S2 slice C, as built)

1. **`projectObject` normalises, then writes through the primitive.** It defaults empty metadata to `{}` and zero timestamps as before, then calls `objectgraph.WriteObjectStmts(tx.StmtContext(header), tx.StmtContext(body))`; with no prepared set it falls back to `WriteObjectTx(tx)`. `projectStmts` holds `objectHeader` and `objectBody`.
2. **`objectRowTables` is `[]objectRowTable{Header, Body, Key}`** (`og_objects/og_bodies/obj_id`, `choir_events/choir_event_bodies/ev_id`). The fence reads (`servingRoleFieldsOf`, the by-id and by-tape-position replay-fence reads) build their LEFT JOIN from it.
3. **`updateObjectRow` (vocab_migrate_og.go)** is the one UPDATE for the migration and its revert: header UPDATE with `BodyDerivedSQL("object_kind", "?6", "?3")` and `RETURNING obj_id`, then `UPDATE og_bodies`. In an UPDATE, `object_kind` and `metadata` in the SET expressions read the old row; the kind never changes, and the new metadata is passed as `?3`.
4. **The maintained-writer guard** lists `WriteObjectTx`/`WriteObjectStmts` and `updateObjectRow` as primitives whose callers run Maintain/Drop; `revertOGMigration` no longer holds an `og_objects` literal, so the known-writer list names `updateObjectRow` instead.
5. **Test seeds** for the split layout go through `drillSeed`/`drillSeedErr` (split_testhelpers_test.go), which call `WriteObjectTx`.
6. **Not covered by a new test:** the choir_events half of the fingerprints (needs slice B's event writer); the run-list and conversation-head readers are covered by the existing graph_store and inbox_open_rounds tests plus `TestRunListReadersMatchAFullRead`.
## Deviations (S2 slice B, as built)

11. **`migratePartitionSplit(ctx, db)` lives in `internal/store/partition_split.go`**, not `header_body_split.go` (slice B owns it; the slice 0 stub is deleted from `header_body_split.go`, which is now a comment-only file for slice D to fill). It takes the partition's write `*sql.DB`, runs rename, drop of the five old indexes, `choirEventsDDL`, header copy (`ev_id` = old rowid, `event_phase` by `ChoirEventPhaseSQLFor("body","metadata")`), body copy, a three-way row tally and `DROP TABLE` in one transaction, then a best-effort `VACUUM`. `partitionSplitHook` is a test-only failure injection point.
12. **`telemetryPartitionDDL` is the constant `choirEventsDDL`** (no copy of the text). The partition kind index is now `idx_choir_events_kind_updated_phase`; the old `idx_choir_events_kind_updated` is dropped by the migration.
13. **`choirEventsStamp` is a separate constant** from `choirEventsTable` (both `"choir_events"`). `eventObjectOp`, `eventStamp` and the sealer use the stamp; `project.go` and `vocab_replay_fence.go` (slice C) still compare `op.Table` to `choirEventsTable`, which is equal and compiles, and should switch to the stamp when slice C touches them.
14. **`insertChoirEventRow` takes a `choirEventWriter`** (`ExecContext` + `QueryRowContext`; `*sql.Tx` and `*sql.DB` satisfy it). The header insert binds the body as `?9` for the phase expression only. With `keepExisting`, `DO NOTHING ... RETURNING` returns no row and the body statement is skipped.
15. **Plan-test SQL is exposed as `readStoredRowsSQL(where)` and `citedEventIDsSQL`** so the plan tests read the production text. DELETE statements are not plan-asserted header-only (the drop trigger's body seek appears in a DELETE's plan); the tests assert the equivalent SELECTs.
16. **Test fixtures that inserted `body` into `choir_events` directly** (`event_table_vocab_test.go`, `stream_seq_exact_seed_scale_test.go`) use `rawEventRow` (in `split_events_plan_test.go`, the production writer).
