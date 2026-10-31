# The append-clustered guest store (2026-10-31)

Status: part 1 built (the WAL bound), step S1 of part 2 built (time-ordered
run-memory entry ids), steps S2 to S5 designed and measured on a synthetic
store, not built. Mutation class: part 1 orange (guest store pragmas and a
checkpoint, no data change); S1 red (run-memory identity of new rows, on the
tape); S2 to S5 red (store schema, witness, snapshot layout, boot path).
This note is rule B of the
[small-object storage design](small-object-storage-2026-10-31.md) taken to a
build, and it arrives through the compaction of the
[one-tape design](one-tape-durability-2026-10-31.md) section 6b.

Labels: **[P]** measured on the read-only clone of the paper's disk
(2026-10-31, the numbers the mission brief carried); **[C]** read from code
at the cited line; **[S]** measured on the synthetic store of section 6;
**[E]** estimated, arithmetic shown.

## 0. The answer

The paper's state.db is 20.96 GB of 4 KiB pages [P] and in 50 minutes 16% of
its blocks and 67% of its 1 MiB regions were rewritten [P], so every
snapshot chunk and every backup chunk changes every day. The brief's
hypothesis was where inserts land: the 136-byte canonical id
`obj:<kind>:<owner b64>:sha256-<hex>` is the text primary key of
`og_objects` and the last column of 13 of its 16 secondary indexes, and
`run_memory_entries` is keyed by a UUIDv4, so inserts land on random leaves.
The synthetic store (section 6) confirms that, and shows a larger cause next
to it: **in a file grown by inserts every B-tree's pages are interleaved
through the whole file, and the `auto_vacuum=INCREMENTAL` pointer map (one
page per 820) is rewritten wherever a page moves**, so an hour of the
paper's writes changes every 4 MiB chunk in either key layout (1 of 3,736
held with the new keys, 0 of 4,380 with today's). A compact copy, which
writes each B-tree contiguously, holds 88% of the chunks across that hour
with today's keys and 93% with the new ones, and 83% and 88% after 24 hours.
So the order of value is: (a) get the file compacted, and keep it so; (b)
the new keys, which halve pages per insert and index bytes and add about
5 points of reuse; (c) compression.

1. **WAL bound (built).** `journal_size_limit` 64 MiB on every writer
   connection, and a PASSIVE checkpoint as soon as a snapshot capture or a
   compact copy ends. The 584 MB WAL [P] shrinks back to at most 64 MiB at the
   first commit after the long read; no reader or writer waits.
   `synchronous` stays FULL (section 3.2). No index is dropped: the reader
   census found a reader for all 17 (section 3.3).
2. **Time-ordered run-memory ids (built, S1).** New entry ids are UUIDv7, so
   the `entry_id` autoindex and `idx_run_memory_entries_parent` append at the
   right edge. No schema change, no migration; old rows keep their ids.
3. **The og_id layout (designed, S2 to S5).** `og_objects` gets
   `og_id INTEGER PRIMARY KEY` (insert order, stable under VACUUM),
   `canonical_id` becomes `UNIQUE`, and no secondary index carries
   `canonical_id`: each ends in the 1 to 4 byte `og_id` instead, so an insert
   appends to every `(kind, ...)` range. It arrives only through a relayout
   copy staged like the compact copy and swapped in at a planned restart,
   with the witness made layout-independent first.
4. **Compression of bodies over 1 KiB (designed, S3).** Measured price
   below; it waits for the witness to hash decoded values.
5. **Compaction that reaches the paper (open, S0).** The compact copy exists
   (`store/compact.go`) but is staged only when the freelist is large, and
   the planned rebase drops a copy whose tape tail exceeds 50,000 events,
   about 4 hours of the paper's events at ~12,000 an hour [E from the
   small-object note's receipt rate]. Until compaction reaches the paper on
   a cadence, its file stays interleaved and no key layout restores chunk
   reuse. This is the next build, before S2.

## 1. What the paper shows [P]

- state.db 20.96 GB, page size 4096, WAL mode, `auto_vacuum=INCREMENTAL`,
  `synchronous=FULL`, no `journal_size_limit`, WAL file 584 MB.
- `og_objects` 10.54 GB, 3.34 M rows, plus 17 indexes 5.51 GB (1.65 KB of
  index per row). `run_memory_entries` with its indexes 4.46 GB, 1.14 M rows.
- 1,582 GB written to the guest filesystem since 2026-10-13, about 110 GB a
  day; about 11 GB of uncompressed JSON in the 21 GB.

## 2. Failure modes first

A layout that writes fewer pages is wrong if any of these happen. Each step
in section 4 names the ones it must test.

- **A1 Witness divergence.** The content witness v3
  (`computerversion/sqlite_state_extractor.go`) hashes every stored column in
  primary-key order and the `sqlite_master` text of every table and index. A
  new primary key (`og_id`) changes both the row order and the row encoding;
  a new index set changes the schema hash. A live store relaid out by a copy
  assigns `og_id` in the copy's order, a replay into a fresh store assigns it
  in tape order: if the witness covers `og_id`, live and replay disagree on
  every computer, and the replay-eligibility gate refuses every candidate.
- **A2 Mixed layouts.** A fresh store (a replay probe, a restore from
  genesis, a new computer) takes the new DDL at once, while an existing store
  keeps its own (`CREATE ... IF NOT EXISTS` never alters). A probe that
  replays an old-layout computer into a new-layout store fails the schema
  hash even when every row matches.
- **A3 Reader regression.** Keyset pages read `WHERE object_kind = ? AND
  canonical_id > ? ORDER BY canonical_id` through `(object_kind,
  canonical_id)` with no sort. Without `canonical_id` in that index the
  planner sorts the whole kind (1.75 M `choir.event` rows) for every page.
- **A4 Rollback.** A binary below the layout cannot read a relaid store:
  `og_id` is harmless to old SQL, but the old index definitions are gone and
  old keyset pages sort the kind (A3). The store floor must be raised before
  the first relaid file exists (`storeschema/floor.go`).
- **A5 Live migration.** Rewriting the table of a running computer holds the
  single writer for minutes and doubles the file. Never: the relayout is a
  copy from one read transaction, swapped at a planned restart.
- **A6 Id determinism.** `og_id` must never leave the guest or appear on the
  tape, in a receipt, a cursor or a witness; it is a page-placement key, not
  an identity.
- **A7 CPU and memory spikes.** The owner's bound: storage efficiency must
  not cost CPU or memory spikes. A relayout copy is one sequential read and
  write; compression must be priced per MB.
- **A8 Non-deterministic compression.** A zstd frame depends on the library
  version and level; a witness over compressed bytes diverges across
  releases.
- **A9 Lost acknowledged write.** `synchronous=NORMAL` loses the newest
  commits on a guest kill; safe only for state the boot replay re-derives.
- **A10 Lane conflict.** `lane/tape-group-commit` rewrites
  `store/run_memory.go` (batching, striped locks); it and the other lanes
  raised the security epoch to 25 on main before this lane landed.

## 3. Part 1: the WAL, durability and the index set

### 3.1 The WAL bound (built)

Cause [C]. The snapshot capture (`snapshot/snapshot.go`, `capture`) reads
the event head, every page through `sqlite_dbpage` and the witness in one
read transaction on the read pool. While it is open a checkpoint cannot
backfill past its read mark, so every commit made during the capture stays
in the WAL and the file grows by all of them. The compact copy
(`store/compact.go`, `VACUUM INTO`, budget 90 minutes) does the same. When
the read ends, the next autocheckpoint (PASSIVE, at 1000 pages) backfills
the WAL and the next writer restarts it from the top, but without
`journal_size_limit` SQLite never truncates the file, so the high-water mark
stays on disk.

Can the read be shorter? No. The head, the pages and the witness must be one
state, so they share one transaction. The SQLite backup API in steps
releases the read between steps but restarts from page 1 whenever another
connection writes the source; at the paper's write rate it would never
finish. `VACUUM INTO` is also one read transaction.

Built:

- `dbx.SQLiteDSN` gives every writer connection
  `journal_size_limit(67108864)` (`dbx.SQLiteJournalSizeLimit`): the first
  commit after a WAL restart truncates the file to 64 MiB, 16x the
  1000-page autocheckpoint, so ordinary traffic never truncates and regrows
  it. Read-only connections do not take it (they never restart the WAL).
- `Store.CheckpointAfterLongRead` (`store/wal_bound.go`): one
  `wal_checkpoint(PASSIVE)` on the write connection, run after every
  snapshot capture (`agentcore/snapshot_producer.go`, deferred in `Take`)
  and every compact copy. PASSIVE never calls the busy handler: no reader or
  writer waits beyond the checkpoint itself, which the single write
  connection serializes like a commit.
- `/api/observability` reports `wal`: the limit, the WAL file size, the
  checkpoints run and the last one's busy/log/checkpointed frames and
  before/after sizes.

There is no switch. Both acts are SQLite's own maintenance with no
durability or ordering effect, and the release-switch cap (64,
`switches/registry.yaml`) is full: adding `store.wal_bound` failed
`check-switch-registry`, and the one zero-reader setting is already being
retired by `lane/tape-group-commit` for its own switch. Rollback: revert;
the WAL returns to growing unbounded, nothing on disk changes format.

Test (`store/wal_bound_test.go`, failure modes W1 to W4): 80 MiB committed
under an open read transaction with no writer waiting; with the limit off
the WAL keeps 80+ MiB after the read ends, with it on the WAL is at most
64 MiB after one commit; the post-read checkpoint backfills every frame;
a reopened store's writer carries the limit.

### 3.2 synchronous FULL or NORMAL: FULL stays

NORMAL in WAL mode never corrupts the file; a guest kill rolls back the
newest commits. Every tape-projected commit is re-derived by the boot replay
(`autoputer/run.go` runReplayPhase replays from the store's projection head
to the canonical head before the runtime serves; `store/replay_durability.go`
already runs replays at NORMAL on that argument). Most families that once
wrote SQL directly are projections when the store is tape-bound (desktop
state, media state, Texture aliases and agent mutations each append an op
first). Two still write direct SQL on a bound store, with their graph
switches at their default (off) [C]:

- `co_super_slots`, the engineering slot claim
  (`store/store.go` claimEngineeringSlotTable; home moves with
  `engineering.slots_graph`);
- `agent_evidence` (`store/texture.go` createEvidenceTable; moves with
  `evidence.graph`).

An acknowledged slot claim or evidence row would be lost by a guest kill at
NORMAL (A9), and the census behind this list is a grep of the store
package's direct `INSERT`/`UPDATE`/`DELETE`, not a proof. FULL stays until
both homes move to the graph and a write census proves every guest write is
tape-derived; then NORMAL on the writer is the one-line change, and the
replay argument covers it. The fsync count is
not the churn: NORMAL changes when the WAL is synced, not which pages are
written.

### 3.3 The og_objects index census

Method: every non-test query on `og_objects` in `internal/objectgraph` and
`internal/store` (none in `internal/agentcore` or `cmd`), run under
`EXPLAIN QUERY PLAN` on a seeded store with the production DDL and the
pinned modernc SQLite v1.49.0, with and without ANALYZE (production runs
no ANALYZE). Readers are function names; line numbers approximate.

| Index | Columns | Readers | Plan |
|---|---|---|---|
| `sqlite_autoindex_og_objects_1` | canonical_id (PK) | GetObject, ReadObjectSnapshotFiltered phase 2, ogHeadPageSQL, ListObjectGraphFingerprints, snapshotResidueObjects, upsert ON CONFLICT | seek; ordered full scans |
| `idx_og_objects_updated` | updated_at | NewestObjectHeadersSQL, ListObjects without kind | ordered scan, no sort; the only kind-less newest-first read |
| `idx_og_objects_kind_id` | kind, canonical_id | ListObjectsPage, KindMetadataPageSQL, deriveSQL, derived-key IN and COUNT, ObjectKindExistsSQL, KindOutsideRangeSQL, GetObjectByMetadata, metadataPageJSONSQL, ListObjectsByEdgeTo, keep-cited scans (object_retire, telemetry_partition) | keyset seek, no sort; covering for COUNT/EXISTS |
| `idx_og_objects_owner_computer_kind` | owner, computer, kind, canonical_id | ReadObjectSnapshot, snapshot phase 1 without kinds, snapshotResidueObjects, ListObjects owner+computer | seek, temp sort on canonical_id |
| `idx_og_objects_kind_owner_computer` | kind, owner, computer, canonical_id | ObjectsByKindOwnerComputerPageSQL, phase 1 with kinds, ListObjectRefsByKindOwner, ListAllObjectRefsByKindOwner, ListJSONBodyFieldsByKindOwner, ListObjectsByOwnerAndBody, ListObjects kind+owner | keyset seek; temp sort for updated_at |
| `..._og_meta_run_id` / `_recent` | kind, run_id, canonical_id / updated_at | metadataPageSQL / metadataListSQL | seek, no sort |
| `..._og_meta_trajectory_id` / `_recent` | same | metadataPageSQL / metadataListSQL, trajectoryRefsSQL | seek, no sort |
| `..._og_meta_channel_id` / `_recent` | same | metadataPageSQL / metadataListSQL | seek, no sort |
| `..._og_rec_inbox`, `_sender_inbox`, `_conversation_id` | kind, column, canonical_id | metadataPageSQL (seek), metadataListSQL (seek + temp sort on updated_at) | |
| `idx_og_objects_rec_inbox_seq` | kind, inbox, inbox_seq | ListRecordsByInboxSeq | range seek |
| `idx_og_objects_rec_claim` | kind, owner, claim, updated_at, canonical_id | ListCommitmentClaimObjects | reverse walk |
| `idx_og_objects_choir_event_updated` | updated_at WHERE kind = choir.event | the retire statements, INDEXED BY | range |

Findings: **no index has zero readers, and no pair is redundant**: each
`_recent` twin serves `ORDER BY updated_at DESC LIMIT`, its equality twin
the canonical-id keyset page, and each fails the other's reader;
`owner_computer_kind` is the only owner-without-kind path; `kind_id` the
only cross-owner keyset page. Nothing is dropped. Two notes for later: the
three `og_rec_*` columns have no `_recent` index, so their newest-first list
sorts every record of the key (adding one would be more churn, so only if a
reader shows the sort); and without ANALYZE the planner picks
`owner_computer_kind` for some kind+owner+computer reads, so the plan is
the planner's guess, not a design.

What the census says about churn: the four indexes that make an insert land
at random are exactly the four whose last column is `canonical_id` over a
low-cardinality prefix (the PK, `kind_id`, `owner_computer_kind`,
`kind_owner_computer`): one owner and one computer hold almost every row, so
their prefixes do not localize the insert. The metadata indexes also end in
`canonical_id` but behind `run_id`/`trajectory_id`/`channel_id`, which a run
writes in a burst, so they land on a few hot leaves. Dropping no index, the
churn is cut only by changing what the indexes end in (section 4).

## 4. Part 2: the layout, step by step

### S1. Run-memory entry ids are UUIDv7 (built)

`run_memory_entries` has two random structures [C]: the `entry_id TEXT
PRIMARY KEY` autoindex and `idx_run_memory_entries_parent(parent_entry_id)`;
`UNIQUE(loop_id, seq)` already lands at each live loop's tail and
`idx_run_memory_entries_kind(owner, agent, kind, created_at)` at each
agent's tail. Keying the table by `(loop_id, seq)` (the small-object rule
B.4) does not remove the entry-id index: `GetRunMemoryEntry` by id is a
product read (`choir.GetRunMemoryEntry`, broker action
`get_run_memory_entry`), so an index on `entry_id` must stay, and a
`WITHOUT ROWID` table holds at most ~1 KB of a row in the leaf (index B-tree
local payload) so the 2.2 KB messages would spill to overflow pages. What
makes both indexes append is a time-ordered id. `newRunMemoryEntryID`
(`store/run_memory_ids.go`) returns a UUIDv7 (48-bit milliseconds then a
monotonic counter), so the id autoindex appends, and a parent is the loop's
previous entry, which is recent, so the parent index appends too.

- No schema change and no migration: rows written before keep their v4 ids.
- The id is opaque everywhere [C]: it is recorded in the projection op on
  the tape and replay writes the recorded value, so replay and witness are
  unchanged (A1 holds trivially).
- `cell-fate:<hash>` ids (`agentcore/rlm_reduce.go:594`) stay as they are; a
  small share.
- Test: 20,000 ids sort in creation order and are UUIDv7 strings
  (`store/run_memory_ids_test.go`).
- Rollback: revert; ids go back to v4 for new rows, both kinds coexist.
- Conflict (A10), resolved: tape group commit landed first and moved the
  `entry.EntryID = uuid.NewString()` line into `normalizeRunMemoryEntry`;
  the rebase took main's file and replaced that one call with
  `newRunMemoryEntryID()`, so the batched and the acknowledged paths both
  mint v7 ids.

### S2. The og_id layout (designed)

```sql
CREATE TABLE og_objects (
	og_id         INTEGER PRIMARY KEY,   -- rowid alias, insert order
	canonical_id  TEXT NOT NULL UNIQUE,
	... the same columns and generated columns ...
);
-- every secondary index as today minus its trailing canonical_id:
-- (object_kind) (owner_id, computer_id, object_kind)
-- (object_kind, owner_id, computer_id) (object_kind, og_meta_run_id) ...
```

This is the intern table of rule B.2 without the extra table: the rowid
already is an insert-ordered integer, the `UNIQUE` autoindex on
`canonical_id` is the one random structure left (the text-to-integer
lookup, one leaf per insert instead of four), and every secondary index
implicitly ends in the rowid. `INTEGER PRIMARY KEY` matters: `VACUUM` (and
so the compact copy) may renumber an implicit rowid but never an explicit
one, and the snapshot copies pages verbatim.

Reader changes (A3), each keyset page moves from canonical-id order to
og_id order with the cursor still a canonical id: a page resolves its
cursor to `og_id` through the unique index (one seek) and reads
`og_id > ? ORDER BY og_id`. The callers are the keyset readers of 3.3
(ListObjectsPage, KindMetadataPageSQL, deriveSQL, ObjectsByKindOwnerComputerPageSQL,
metadataPageSQL, metadataPageJSONSQL, the derived-key range checks). A cursor
row deleted between pages (a retire) needs the cursor to carry the og_id as
well; the page API grows an opaque token. Readers that order by
`updated_at` do not change. The derived-key range checks
(`canonical_id > ? AND < ?`) keep using the unique index.

### S3. Compression of large values (designed)

Bodies over 1 KiB in `og_objects.body` and `run_memory_entries.message_json`
in a zstd envelope (magic `28 b5 2f fd`), readers expanding transparently in
`scanObject` and `scanRunMemoryEntry`. `og_rec_claim` reads the
commitment-record body inside a generated column, so that kind stays raw,
and so does any kind a generated column or `json_extract` reads. The
witness must hash the decoded value, not the frame (A8). Priced in 6.3.

### S4. Delivery: a relayout copy at a planned restart (designed)

Never an in-place migration (A5). The compact copy is the vehicle
(`store/compact.go`): `PrepareCompactCopy` writes the copy from one read
transaction while the computer runs and `RebaseOntoCompactCopy` swaps it in
at the next planned restart, after which the ordinary boot replay brings it
from the copy's head to the canonical head. A relayout copy differs only in
how it writes: instead of `VACUUM INTO`, a fresh file with the new DDL and
`INSERT INTO new.og_objects SELECT ... FROM og_objects ORDER BY created_at,
canonical_id` (so og_id is time order), every other table copied as is. The
store floor is raised before the first relaid copy is staged (A4), and the
sidecar names the layout. A crash boot never swaps (the existing rule).

### S5. The witness covers rows, not layout (designed, first)

Before any relaid file exists: the extractor leaves `og_id` out of the row
encoding (`IgnoredContentColumns`), orders `og_objects` by `canonical_id`
(an order override, read through the unique index), and the schema hash
covers a layout-normalized DDL, or the probe opens its fresh store in the
live store's layout (A1, A2). Check: a store built in the old layout and the
same rows relaid give the same `content_root`, and a replay into a fresh
store of either layout matches the live one.

Order of build: S5, then S2's readers (they work on both layouts), then the
DDL for fresh stores with the probe layout rule, then S4 for existing
computers on d@e, then the paper by its ring.

## 5. Cost

- S1: none measurable; a v7 id is a clock read and 10 random bytes, like v4.
- S2: one more probe per write (the unique index to the integer) and none
  saved on reads by canonical id; memory none extra (page cache as today).
  Index bytes fall by about 128 B times 13 entries per row before slack
  (the small-object estimate, 4 to 5 GB on the paper [E]; 6.2 measures it).
- S4: one sequential copy, the cost of a compact copy (the paper's
  `VACUUM INTO` budget is 90 minutes, idle IO); peak memory the copy's page
  cache, bounded by `cache_size`.
- S3: see 6.3.

## 6. Measurement on a synthetic store sized like the paper [S]

`store/churn_bench_test.go` (env-gated `TestChurnBench`): 3.34 M og_objects
rows in time order with the paper's kind mix and body sizes (choir.event 52%
at 1.37 KB, Zipf-distributed words), metadata with `run_id` (24 runs live at
a time), `trajectory_id`, `channel_id`, the record columns; 1.14 M
run-memory rows of ~2.2 KB messages over 24 live loops. Each layout built
fresh, then measured with WAL and autocheckpoint off:

- **pages dirtied per 1,000 writes**: distinct page numbers in the WAL after
  the writes, from an empty WAL: the pages the next checkpoint writes into
  the file, so the pages a snapshot or backup re-sends;
- **an hour of writes**: 10,640 og writes (70% inserts, 30% in-place run
  updates) and 5,320 run-memory entries in transactions of 10 to 11, the
  paper's hourly volume; the file's 4 MiB chunks (snapshot v1's cut, 1024
  pages) hashed before and after, checkpointed.

### 6.1 Results (3.34 M og rows, 1.14 M run-memory rows, 4 KiB pages)

| | current | clustered | change |
|---|---:|---:|---:|
| file | 18.37 GB | 15.67 GB | -15% |
| og_objects table | 9.00 GB | 9.01 GB | |
| og_objects 16 indexes + PK | 5.32 GB | 2.63 GB | -51% |
| run_memory_entries + 4 indexes | 3.93 GB | 3.93 GB | (S1 changes id order, not size) |
| build, time-ordered inserts | 10,634 s | 4,993 s | -53% |
| pages dirtied / 1,000 og inserts, 1 per tx | 14,237 | 6,699 | -53% |
| pages dirtied / 1,000 og inserts, 10 per tx | 14,330 | 6,941 | -52% |
| pages dirtied / 1,000 og writes, 30% run updates | 10,647 | 5,293 | -50% |
| pages dirtied / 1,000 run-memory inserts | 3,800 | 1,196 | -69% |
| ms / 1,000 og inserts (1 per tx, sync off) | 1,550 | 862 | -44% |
| hour of writes, distinct pages | 106,177 | 41,361 | -61% |
| hour, grown file: chunks unchanged | 0 of 4,380 | 1 of 3,736 | |
| hour, compact copy: chunks unchanged | 3,716 of 4,205 (88.4%) | 3,388 of 3,662 (92.5%) | |
| 12 h, compact copy: base chunks unchanged | 3,617 (86.0%) | 3,310 (90.4%) | |
| 24 h, compact copy: base chunks unchanged | 3,498 (83.2%) | 3,212 (87.7%) | |
| 24 h, compact copy: chunks a daily snapshot re-sends | ~1,030 (4.3 GB) | ~728 (3.0 GB) | -29% |

The synthetic current layout matches the paper where it should: og index
bytes 5.32 GB against 5.51 GB [P], file 18.4 GB against 21 GB [P].

Where the pages go (one batch of 1,000 og inserts on the grown clustered
store, `TestChurnAttribute`, pages named by `dbstat`): 4,556 distinct pages
in 2,236 of the file's 3,740 chunks; **1,287 pointer-map pages touching
1,211 chunks**, the per-(kind, run) and per-trajectory/channel tails of the
six metadata indexes 2,072 pages over 185 to 404 chunks each, the
table's own tail 580 leaf and 61 overflow pages in 4 chunks, the canonical-id
unique index 110 pages in 49 chunks. On the compact copy the pointer map stops mattering:
auto_vacuum NONE and INCREMENTAL hold 3,395 and 3,388 chunks across the hour
(`TestChurnHourCompacted`), so auto_vacuum stays as it is.

### 6.2 What the hour model leaves out

The model's hour dirties 106,177 distinct pages (0.43 GB) with today's keys;
the paper measured at least 4.6 GB of distinct blocks an hour [P], about 11x
more. The model writes only the daily row growth (10,640 og writes and 5,320
run-memory entries an hour). It has no retirement deletes (7- and 14-day
telemetry, which delete rows in old regions and every index entry they
own), no event-index inserts and trims, no projection-head and lease
updates, and no derived-key rows. So the model shows the mechanism and the
relative gain; it does not reproduce the paper's absolute rate. Retirement
deletes favor the clustered keys further [E]: a retired time slice is a
contiguous og_id range inside each kind's index range, against random leaves
today. That is a hypothesis for the next measurement, not a result.

### 6.3 Compression price

`BenchmarkChurnZstd`, klauspost zstd v1.18.7, one encoder reused, record at
a time, bodies over 1 KiB of the paper's sizes, on an idle M-series core:
SpeedDefault 129 MB/s compress (7.7 ms CPU per MB), 259 MB/s decompress
(3.9 ms per MB), ratio 2.07 on the synthetic Zipf text; SpeedFastest
156 MB/s and 270 MB/s at 2.03. The paper writes about 710 MB a day of
og bodies and messages [P: 430 + 280 MB], 5.5 s of CPU a day to compress;
memory is the one encoder's window, a few MB. The 30,000-row trial of the
clustered layout with compression gave a 33% smaller file (151 against
226 MB) and 7% fewer pages per insert (2,996 against 3,245, 10 per tx). The
synthetic ratio is a floor: real JSON repeats keys, so record-at-a-time
lands nearer the small-object note's 1.5-2x measured, and block compression
(5.4x) needs packs, not rows.

## 7. Open risks

- Compaction does not reach the paper on a cadence (item 5 of section 0):
  the freelist gate and the 50,000-event tail bound. The snapshot-based
  compaction of one-tape 6b (a compact snapshot, a planned rebase onto it)
  is the vehicle that fits the paper's event rate; the local copy fits
  quiet computers.
- A compacted file decays as splits allocate new pages at the end and
  scatter parents: 88% to 83% of chunks in a day with today's keys. A weekly
  compaction keeps it above about 80% if the decay stays linear [E]; to be
  measured past 24 hours.

- The synthetic store is not the paper: one owner and one computer (true of
  the paper), uniform run concurrency, Zipf text. The page counts depend on
  key placement, which the generator reproduces; the compression ratio
  depends on real JSON, which it does not.
- S2 changes keyset page order (canonical id to insert order). Any reader
  that relied on canonical order for correctness, not just completeness,
  must be found before S2's readers land; the census lists the keyset
  readers but not their callers' assumptions.
- The biggest single lever needs no new code: moving `choir.event` (52% of
  og rows) to the F-c `choir_events` table and the telemetry partitions
  (rule B.1, a ring widening) takes half the inserts off the 17 indexes.
- Security epoch: this lane raises it 25 -> 26 (agentcore is a surface);
  main reached 25 through the group-commit, tool-layers and email lanes
  while this one was built.
