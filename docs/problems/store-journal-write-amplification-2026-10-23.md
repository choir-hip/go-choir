# Where the paper's journal bytes come from, and what would cut them (2026-10-23)

Date: 2026-10-23. Found by: FP0 worker, lane `fp0-journal-amplification`
(read-only investigation plus local measurement; nothing is fixed here).
Status: open. Mutation class of this record: green (docs) plus one yellow test
(`TestJournalAmplificationCensus`, off by default). Mutation class of the fixes
named below: orange (an index drop, a batched statement) and red (anything
that changes the tape projection or the store schema).

Parents: [paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md](paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md)
(the write model and the R1 to R7 cuts), [paper-remaining-store-stall-holders-2026-10-23.md](paper-remaining-store-stall-holders-2026-10-23.md)
(the online GC holds the write connection about 18 s per GiB of journal; section 1a
rules out moving it), [run-memory-entries-hold-the-engine-2026-10-23.md](run-memory-entries-hold-the-engine-2026-10-23.md)
(the 41 to 45 KiB per append and the R5 duplicate index; R5 is on
`lane/fp0-run-memory-r5`). Claims are marked **[V]** verified (a local measurement
or the code or Dolt source, named beside it), **[M]** measured elsewhere and cited,
**[H]** hypothesis.

## The question

The online GC is 13 of the 16 recent `/health` 503s. Its length is the journal's
length, so the substrate fix is to write less journal (owner, 2026-10-22: resource
use ahead of short-term uptime). Where do the roughly 50 KiB of journal per
run-memory append come from, which writers dominate the journal, what cuts the
bytes per logical write most, and what does that do to the GC?

## Answer

1. **A write costs one root-to-leaf path in every tree it touches, rewritten on
   every statement, and nothing else is large.** **[V]** A run-memory append is
   37 chunks and 67 KiB of journal on a 100,000-row table, 77 KiB at 425,000 rows
   (local, direct path). 61 KiB of it is the paths of seven prolly trees (the
   primary key, the `UNIQUE(loop_id, seq)` key and five indexes), 8.8 KiB a tree
   on average; 6 KiB is fixed per transaction (root value, table, index-set
   address maps, working set). The row's size is not a term (0 bytes and 3 KB cost
   the same), a Dolt commit is not a term (1.0 KiB per commit, one in 100 events),
   and the duplicate `idx_run_memory_entries_loop_seq` costs under 1.5 KiB because
   its chunks are byte-identical to the unique key's and the journal stores a chunk
   once. Through the tape, the path the paper uses, the same append is 103 KiB: a
   fixed 36 KiB per tape event (dry run, prepare, finalize) on top.
2. **Batching helps only at the statement, not the transaction.** **[V]** 200
   appends as 200 statements in one transaction: 67.5 KiB each against 69.9 for 200
   transactions (3%). The same 200 rows as one multi-row `INSERT`: 14.0 KiB each
   (80% less). Rows of one run, four to a statement: 33 KiB each (52% less),
   against 70 for one by one. Through the tape, ops per event amortise the 36 KiB
   (k = 4: 77, k = 64: 67.5 KiB per op) but each op is still its own statement
   (`project.go:63`).
3. **No single run-memory fix moves the GC much, because run memory is about a
   quarter of the journal.** **[H]** (the shares are a model; the per-op costs are
   measured). Run memory is 3.4 of about 10.7 ops a second and about 27% of the
   bytes; the trace lines still in `og_objects` are about 35% and the other
   `og_objects` ops (cell commits, wake outbox, Texture, lifecycle) about 38%. The
   best run-memory pair measured (three indexes dropped, four rows of one run to a
   statement and one tape event) is 103 to 30 KiB per append, 71% of run memory, 19%
   of the whole journal. F-c deploy 2 (trace lines into `choir_events`) is 55 to 77%
   of the trace share measured, also about 19% of the whole. All three together
   (those two plus multi-row `og_objects` upserts) is about 50%.
4. **The GC's stall share is proportional to journal bytes written and does not
   depend on the threshold.** **[V]** arithmetic of the paper's 18 s per GiB
   (stall seconds per second of wall time = 18 s/GiB x journal rate, 1.76% at 1.0 MiB/s).
   Halving the journal rate halves the stall share (1.76% to 0.85%) and the passes
   (3.1 to 1.6 an hour) but leaves each pass about 19 s long, so each pass is still
   a seven-sample run of 503s. Writing less is the only lever that shrinks total GC
   seconds and disk growth; ending the 503s needs the pass to be shorter than the
   health budget or off the write connection, which section 1a of the stall record
   shows is unsafe.

## What was measured, and how

`internal/store/journal_amplification_census_test.go` (`TestJournalAmplificationCensus`,
off by default). It does not infer bytes from a total: after each window it opens
the store's noms journal file, walks every chunk record written in the window with
Dolt's own reader (`nbs.VisitJournalReaderChunks`, `store/nbs/journal_record.go:643`),
decompresses each chunk (snappy, `store/nbs/table_reader.go:98`) and classifies it by
its serial file id (`TUPM` prolly tree node with its level, `ADRM` address map,
`BLOB`, `RTVL` root value, `DTBL` table, `WRST` working set, `STRT` store root).
Journal bytes are the record sizes (compressed payload plus 32 bytes of framing;
a root-hash record is 40 bytes).

```text
CHOIR_JOURNAL_AMP=depth,index,rowsize,floor,batch,cell,drops,commit,tape,events,og \
  go test ./internal/store -run TestJournalAmplificationCensus -v -count=1 -timeout 60m
# CHOIR_JOURNAL_AMP_SEED (default 100000), _ROWS (depth sizes), _N (appends per window, 200)
```

Conditions that bear on the numbers: embedded Dolt on the lane's machine; entry ids
are UUIDs (random keys, which snappy cannot compress), message text is word salad
(compresses about 2 to 3 times, as prose and JSON do; a string of one repeated byte
would have hidden the row's cost); eight runs append in rotation, so consecutive
appends are from different runs. Repeating the full-production-set window five times
gave 66.9, 67.2, 68.5, 69.9 and 70.0 KiB, so any two lines below differ meaningfully
only above about 4 KiB. The paper's trees are deeper (the paper has 425,000+
run-memory rows and 1.0M `og_objects` rows), so every per-tree number is a floor.

## 1. Where the 50 KiB come from (run-memory append)

### The tree path, by depth and by tree

One autocommit `INSERT` into the production table (`store.go:504-519`, indexes
`:599-602` plus the kind index), a 1.5 KB `message_json`, 200 appends per cell. **[V]**

| rows in the table | KiB per append | chunks | what changed |
|---|---|---|---|
| 1,000 | 39.9 | 27 | 2 levels in most trees |
| 10,000 | 55.8 | 32 | a third level in 6 trees |
| 100,000 | 67.2 (66.9 to 70.0 over 5 runs) | 37 | a fourth level in the primary key tree |
| 425,000 | 76.7 | 38 | a fourth level in all 6 secondary trees too |

Each added level costs 9 to 16 KiB over the 7 trees. The journal record's
90 to 96 KiB for a tape append at 100,000 rows (before the kind index existed) is
this record's 103 below less the kind index's 9. The existing census's 41 to 45 KiB
per append (`run-memory-entries-hold-the-engine`) is lower mainly because that
census's ids and text were synthetic and compress better (sequential `e-%08d` ids,
repeated-byte payloads) and its table had one tree fewer; not re-measured **[H]**. At 100,000 rows the 67.2 KiB is, by chunk class: **[V]**

| chunk class | chunks per append | KiB | per chunk (compressed, raw) |
|---|---|---|---|
| tree leaf (`TUPM` L0) | 7.2 | 20.1 | 2.8, 4.3 KiB |
| tree level 1 | 6.2 | 22.2 | 3.6, 4.1 KiB |
| tree level 2 | 6.0 | 16.7 | 2.8, 3.1 KiB |
| tree level 3 (the primary key's root and its parents) | 5.0 | 2.2 | 0.4, 0.4 KiB |
| index-set address maps (`ADRM`) | 8.0 | 3.2 | 0.4 KiB |
| table (`DTBL`), root value (`RTVL`), working set (`WRST`), store root (`STRT`) | 4.0 | 2.7 | |
| root-hash record | 1.0 | 0.04 | |

- A chunk is 2.8 to 3.6 KiB compressed because Dolt's chunker targets 4 KiB:
  `targetSize = 4096`, minimum 512, maximum 16,384 bytes, a Weibull split on the
  key hash (`store/prolly/tree/node_splitter.go:35-36, 217, 245`;
  `defaultSplitterFactory = newKeySplitter`, `:60`). There is no setting for it:
  the constants are compile-time, and changing them changes every chunk hash, so
  every content witness and every checkpoint. **[V]** Interior nodes are as large as
  leaves, because each child entry is a 36-byte UUID key plus a 20-byte address
  and a UUID does not compress.
- The statement rewrites the whole path because a prolly tree is immutable: the
  new leaf has a new hash, so its parent changes, up to the root. `Close` flushes
  the writer after every statement (`libraries/doltcore/sqle/writer/prolly_table_writer.go:320`),
  `FlushTable` puts the new table into the working root
  (`.../prolly_write_session.go:169-190`), and `table()` calls `idxSet.PutIndex` once
  per secondary index (`prolly_table_writer.go:399-411`), which rewrites the
  index-set address map each time: that is the 8 `ADRM` chunks, 0.4 KiB per index.
  All of these chunks go into the store-wide memtable and `ChunkJournal.Persist`
  writes every one the journal does not already have (`store/nbs/journal.go:293-321`),
  intermediate versions included. **[V]** (code and the chunk census above)

Per tree, by adding indexes one at a time to the primary key (100,000 rows, direct
path), then removing each from the full set. **[V]**

| table | KiB per append | marginal tree |
|---|---|---|
| primary key + `UNIQUE(loop_id, seq)` only | 26.3 | 2 trees plus 3.9 fixed: 11 KiB each |
| + `idx_..._loop_seq` (duplicate of the unique key) | 25.8 | **0.5 (none: no new tree chunks)** |
| + `idx_..._owner_loop_seq` (owner_id, loop_id, seq) | 35.4 | 9.6 |
| + `idx_..._owner_agent_created` (owner_id, agent_id, created_at) | 45.5 | 10.1 |
| + `idx_..._parent` (parent_entry_id) | 58.4 | 12.9 (random keys, incompressible) |
| + `idx_..._owner_agent_kind_created` (the cursor kind index) | 67.5 | 9.1 |
| the full set minus loop_seq / owner_loop_seq / owner_agent_created / parent / kind | 67.0 / 57.6 / 57.1 / 54.3 / 58.8 | 0.5 / 9.9 / 10.4 / 13.2 / 8.7 |

The duplicate index is free because the journal is content-addressed: the chunk
list of a window with and without it differs by one 0.4 KiB address map and no tree
chunk, so the duplicate tree's nodes are byte-identical to the unique key's.
That is why R5 measured 1.1 to 2.8 KiB. **[V]** (R5's number is explained, not
contradicted). The kind index costs 8.7 to 9.1 KiB here, 12.5 to 13 KiB in the
earlier census; both are inside the noise of one UUID run to the next plus a depth
difference.

### The row itself

`message_json` of 0, 500, 1,500, 3,000, 8,000 and 64,000 bytes (the table seeded
with rows of the same size, 50,000 rows; 20,000 for 8 KB and 64 KB). **[V]**

| message_json | KiB per append | the row's own chunks |
|---|---|---|
| 0 | 62.8 | none |
| 500 | 61.1 | none |
| 1,500 | 62.4 | none (inline) |
| 3,000 | 63.7 | 1 `BLOB` chunk, 1.4 KiB |
| 8,000 | 57.3 (shallower tree at 20,000 rows) | 4 `BLOB` chunks, 3.6 KiB |
| 64,000 | 79.8 | 18 `BLOB` chunks, 26.7 KiB |

The answer to "inline or out-of-line": Dolt's text columns are adaptive
(`typeinfo.UseAdaptiveEncoding = true`, `libraries/doltcore/schema/typeinfo/typeinfo.go:32`).
A tuple is kept inline while it is at most 2 KiB (`val.DefaultTupleLengthTarget = 1<<11`,
`store/val/tuple_builder.go:64`, applied at `:116`); above that the largest values
are moved out into content-addressed blobs, chunked like any other value. Each
append's text is new, so its blob chunks are new bytes, written once and never
rewritten; they cost their own compressed size and nothing more. Row size only
touches the primary key's tree (a 1.7 KB inline row makes a leaf hold two rows and
a fourth level at 100,000 rows); the six other trees hold keys only. So
**moving large payloads out of the row saves nothing: they are already out of the
row when they are large, and when they are small they do not matter.** **[V]**

### Fixed cost per transaction, and the Dolt commit

- A transaction with one `BIGINT` row costs 4.8 KiB (8 chunks: two address maps,
  root value, table, a 1-row leaf, store root, working set), in a store of 27
  tables. DOLT_COMMIT adds 1.0 KiB (commit, closure, working set, store root, an
  address map). **[V]**
- The paper commits once per 100 finalized events (`computer_events.go:259-270`,
  `defaultFinalizeCheckpointEvery = 100`, `store.go:159`): 0.01 KiB per append.
  Measured, a commit after every append, every 10 and every 100: 66.7, 64.9 and
  64.2 KiB per append against 65.3 with none. **A Dolt commit is not a term.** **[V]**
- The per-transaction fixed part is 6 KiB of the 67 (9%): a quarter of it is
  address maps that scale with the number of indexes.

### Through the tape: the 36 KiB per event

`AppendRunMemoryEntry` on a tape-bound store (`run_memory.go:52-93`) goes through
`appendOps` (`projection_tape.go:110-153`) and three transactions: a rolled-back
dry run (`computer_events.go:96-117`), `Prepare` (`:33-70`, an `INSERT` into
`computer_event_index`, four trees, one with a large `event_json` row) and
`finalizeBatch` (`:119-248`: the projection head, the index row and its status
index, then the op through `projectRunMemoryEntry`, `project.go:297-342`, a
`SELECT ... FOR UPDATE` and one `INSERT`). Measured with the harness's
`recordingTape`, 100,000 rows seeded, 200 appends. **[V]**

| run-memory ops per tape event (one INSERT each) | KiB per op | chunks per op |
|---|---|---|
| 1 (today) | 102.7 | 71.8 |
| 4 | 77.3 | 45.9 |
| 16 | 69.3 | 39.3 |
| 64 | 67.5 | 37.6 |
| the direct path, no tape | 67.2 to 70.0 | 37 |

That fits `F/k + D` with **F = 35.8 KiB per tape event and D = 66.9 KiB per
statement** (predicted 75.9 at k = 4 against 77.3, 69.1 at k = 16 against 69.3).
The 36 KiB: 14.7 leaf chunks against 7.2 (the event index and head trees, twice),
21 address maps against 8, 4 table and 2 root-value chunks against 1, one `BLOB`
chunk (the event row). It is almost all fixed per event; the record's 5 to 8 KiB for
the dry run is inside it. **[V]** (fit), **[M]** (the dry-run share).

## 2. The write paths, and what is batched

Is every append its own transaction or Dolt commit? **Its own SQL transaction (three
through the tape, one rolled back), never its own Dolt commit** (one commit per 100
finalized events, 1.0 KiB, `computer_events.go:259`). The direct path
(`run_memory.go:95-139`, used only when no tape is bound) is one autocommit transaction.
**[V]**

Rates are FP0's two observability snapshots of 10-22 21:30 and 21:38 (498 s, build
`674460d15`, before R1 to R4) **[M]**, with the cuts the journal record built applied
by arithmetic **[H]**; per-op costs are this census **[V]** at 20,000 to 100,000 rows
(the paper's trees are deeper). The paper's mean journal growth since the cuts is
1.0 MiB/s (0.66 to 1.36 over 18 passes) **[M]**, against 1.38 MiB/s for the model
below, so the shares are usable and the absolute bytes are about 35% high.

| stream | ops/s (after R1 to R4) | tape events/s | KiB per op, measured | modelled KiB/s | share | batched today? |
|---|---|---|---|---|---|---|
| trace lines still durable (`choir.event`: `tool.result`, `loop.progress` tool_loop, `desk.cell` with its egress, run state) in `og_objects`, 13 trees | about 3.0 [H] (6.0 before) | 3.0 | 135 (20,000 rows) to 160 (100,000, [M]) | 480 | 35% | **No.** `telemetry.batch` exists (`telemetry_sealer.go`, ten classified kinds `types/event_durability.go:35-46`) and was off on the paper at 10-22 (`batching.enabled=false` [M]); each line is its own tape event |
| run-memory entries (`message`, `rlm_inbox_cursor`, `cell_fate`) | 3.4 [H] (3.66 before R2) | 3.4 | 103 at 100,000 rows, about 110 at 425,000 | 374 | 27% | **No.** One op per tape event, exempt from the telemetry barrier on purpose (`run_memory.go:84-87`); no multi-op path exists |
| other objects: cell commits (`CommitInboxRecords`: records, inbox head, conversation head, wake outbox row, cursor, `inbox_command`), Texture revisions, lifecycle, wake-outbox state changes | 4.3 [M] | 1.14 | 113 per op + 36 per event | 530 | 38% | **Batched per event, not per statement.** About 250 events per 498 s of 7 ops on average, 24 of 17 or more [M]; each op is a separate `INSERT` (`project.go:63`, `:243`) and pays its own 13 trees. A wake's projected flag is flipped later as its own event (`MarkActorWakeProjected`, `lifecycle.go:897-930`): a second full `og_objects` write |
| self-development, desktop state, media, texture documents | under 0.1 [H] | | | negligible | | per user action or per operation; the paper has no user at its desktop [M] |

The kinds inside the 4.3 ops/s are still unmeasured (the journal record's reading 1;
no counter tallies `canonical_id` kinds). The wake outbox's share of it, and whether
`MarkActorWakeProjected` is a large part, is the open question here.

## 3. What would cut journal per logical write

Savings are measured per append unless marked; "of the journal" multiplies by the
stream's modelled share **[H]**. Risk is the mutation class and what protects it.

| rank by whole journal | option | measured saving | of the journal | risk |
|---|---|---|---|---|
| 1 | **F-c deploy 2: trace lines into `choir_events`** (4 trees, sequential key; "fewer or merged tables") | one `desk.cell` line per tape event: `og_objects` 133.8 KiB, `choir_events` 60.6 (**-55%**); six lines per event: 106.3 against 29.6 (**-72%**); 24: 102.6 against 24.1 (**-77%**) | -19% (one line per event) to -27% (24 per event) | red; already designed and built through deploy 1; needs its writer, the setting, and the replay fence |
| 2 | **Batch rows into one statement** (a multi-row `INSERT` for consecutive ops of one kind, and a group append for the end of a cell) | 4 rows of one run per statement: 69.9 to 33.2 KiB (**-52%**); 16: 22.6 (-68%); 200 interleaved rows: 14.0 (-80%); through the tape with k = 4: 102.7 to 42.2 (**-59%**) | run memory -41% (a cell writes its assistant message before the tool runs and three rows after, so two events: 1 + 3) to -59% (four at once) = **-11% to -16%** | red: `projectBatch` order and replay equivalence (the rows are identical, the statement count is not), and run memory is read back by the run's next call, so a delay of the group is not free |
| 3 | **Multi-row upserts for `og_objects` ops of one batch** (R6 / F-e) | -37% per row at k = 6, on the raw probe (journal record) | other objects -37% = **about -14%** | red, projection |
| 4 | **Drop indexes no reader needs** on `run_memory_entries` | duplicate `loop_seq` -0.7 to -1.5 KiB (1 to 2%); + `owner_loop_seq` -9.8 (to 57.1, **-15%**); + `parent` -12.6 (to 44.5, **-33% total**); also `owner_agent_created` (-8.8, to 35.7, **-47% total**) | run memory 102.7 to 80.3 KiB (**-22%**) for the three safe ones = **-6%** | orange to red: see below |
| 5 | **Narrow `og_objects` indexes** (12 secondary trees) | each costs about 5.7 KiB of 133.8 at 20,000 rows (68 KiB of the 134 are the 12 secondary trees; the primary key and the event cost 66); individual steps 4 to 11 KiB, noise +/- 5 | about -3% of the journal per tree dropped (4.3% of an `og_objects` op times its 73% of the journal) [H]; needs a reader audit this record did not do | red, schema, each reader audited (the kind-index record's witness analysis applies) |
| 6 | Ordered `entry_id` (time-ordered, not UUID) | one row per transaction 69.9 to 64.0 (-8%); 16-row statements 33.2 to 22.3 (**-33%**) | a lever only together with 2; changes the id format | red: `entry_id` is the identity on the tape |
| - | Move large payloads out of the row | **0**: they already are (above) | 0 | |
| - | Batch N statements in one transaction | 69.9 to 67.5 (**-3%**) for 200 | negligible | |
| - | Commit less often | 0.01 KiB per append | 0 | |
| - | A different chunking setting | none exists; compile-time constants (`node_splitter.go:35-36, 217`); changing them changes every hash | | not available |

The top two measured locally, as asked, are 2 and 4 (F-c deploy 2 is ranked first
by whole-journal effect but belongs to its own design, and was measured here only
as a table-against-table comparison: the stamped op is put on by hand, deploy 2's
writer is not built).

**Batching, the literal comparison.** 200 appends, direct path, 100,000 rows: **one
per transaction 69.9 KiB each; 200 statements in one transaction 67.5; a transaction
of 20 ten-row statements 41.6; one multi-row statement of 200: 14.0.** At 2, 4, 8,
16 and 64 rows to a statement and eight interleaved runs: 57.9, 52.0, 47.6, 33.2 and
20.2. The knee at 16 is the eight runs: with two rows of each run in a statement,
the rows of one run are adjacent in `(loop_id, seq)` and the parent chain and share
a leaf; the `created_at` trees take every row at their tail whatever the run. Rows
of a single run (the shape of a cell's end): 1, 2, 4, 8, 16 rows to a statement:
69.9, 45.4, 33.2, 27.2, 22.6. What is left at 200 rows (14.0 KiB, 4.6 chunks) is the
leaf of the random-key trees: each UUID lands in its own leaf, which a statement
cannot share.

**Index drops, the literal comparison.** 100,000 rows, direct path, cumulative:
full set (7 trees) 66.9 (70.0 on the repeat), minus `loop_seq` 66.2 (68.5), minus
`owner_loop_seq` 57.1 (60.9), minus `parent` 44.5 (47.6), minus
`owner_agent_created` 35.7 (37.6). The two levers together (four trees, four rows
of one run to a statement): **20.7 KiB per row against 70** (-70%); sixteen rows:
13.1 (-81%). Through the tape that is `35.8/4 + 20.7 = 29.7` against 102.7 (-71%) on
the F/k + D fit.

**Risk of the index drops. [V] code, [H] the rest.**

- The readers of `run_memory_entries` are `run_memory.go:55, 64, 108, 119, 247, 257,
  290, 390, 461, 481, 491` and `residue_import.go:409`; none filters on
  `parent_entry_id`. `owner_loop_seq` serves `WHERE owner_id = ? AND loop_id = ?
  ORDER BY seq` (`run_memory.go:257`) that `UNIQUE(loop_id, seq)` serves with a
  filter on `owner_id`, which is selective already; R5 verified the same plan
  equality for the duplicate. Each drop needs its own plan test, as R5 and the kind
  index have.
- A drop is a store-schema change, so it goes through the explicit
  access-path operation (`POST /internal/store/derived-columns?action=migrate`),
  never `schemaDDL` at boot (`boot-alters-og-objects-when-older-columns-missing`).
  The witness hashes `SHOW COLUMNS`, whose `Key` for a column that starts a
  non-unique index is `MUL` (`computerversion/dolt_state_extractor.go:338-382`):
  dropping `parent` turns `parent_entry_id` from `MUL` to empty, a replay drift
  until the live store and a fresh one agree, so it needs the kind index's ordering
  (operation first, then the DDL change) and its schema-observation test. Dropping
  `owner_loop_seq` or `loop_seq` does not change a key (`owner_id` is `MUL` through
  `owner_agent_created`; `loop_id` through the unique key). Rollback of each is a
  one-second `CREATE INDEX` (R5: 1.0 to 1.1 s at 425,000 rows) that holds the write
  connection that long.

## 4. What the best options do to the GC

The pass is `CALL DOLT_GC()` on the one write connection, started when the journal is
at least 1 GiB at a five-minute tick (`dolt_maintenance.go:340`, `run.go:727`), and it
takes about 18 s per GiB on the paper (17 to 24 s at 1,025 to 1,342 MiB; mean
1,148 MiB, 19.6 s) **[M]**. With journal growth g (MiB/s) the journal at the start of
a pass is about `1024 + 150 g` MiB (the tick lags the threshold by 2.5 minutes on
average), the interval is that over g, and the stall share is the pass over the
interval. At g = 1.0 this gives 19.6 minutes and 1.76% against the observed median
19.7 minutes and 1.56%, so the arithmetic is calibrated on the paper's own passes.
**[V]** arithmetic, **[M]** inputs. Note what it says: stall share =
`18 s/GiB x g`, independent of the threshold. A lower threshold gives shorter, more
frequent passes with the same total, which is why the stall record dismissed it.

| scenario (cut of the whole journal) | g (MiB/s) | pass | interval | passes an hour | stall share |
|---|---|---|---|---|---|
| today | 1.00 | 20.6 s | 19.6 min | 3.07 | 1.76% |
| run-memory indexes dropped (3), -6% | 0.94 | 20.5 s | 20.7 min | 2.90 | 1.65% |
| run-memory cell batching (4 to a statement), -16% | 0.84 | 20.2 s | 22.8 min | 2.63 | 1.48% |
| run-memory batching and the three drops, -19% | 0.81 | 20.1 s | 23.6 min | 2.55 | 1.42% |
| F-c deploy 2 alone, -19% | 0.81 | 20.1 s | 23.6 min | 2.55 | 1.42% |
| F-c deploy 2 and the run-memory pair, -38% | 0.62 | 19.6 s | 30.0 min | 2.00 | 1.09% |
| and R6 multi-row `og_objects` upserts, -52% | 0.48 | 19.3 s | 38.1 min | 1.58 | 0.84% |

- **Pass frequency and the stall share fall in proportion to the journal.** The best
  one or two run-memory options remove 19% of the journal and take the stall share
  from 1.76% to 1.42%, three passes an hour to 2.55. All three families, about half
  the journal, 0.84%. **[H]** on the shares of the model, **[V]** on the mapping
  from share to stall.
- **The length of a pass does not fall**: it stays 19 to 21 s, because the trigger
  is a size, not a time. A 503 run is still about seven probe samples per pass
  (the stall record), so the count of 503 samples falls with the passes, 3.1 to 1.6
  an hour, and no pass gets under the 1 s budget. Eliminating the 503s needs the
  pass short (under about 55 MiB of journal at 18 s/GiB, if there is no intercept; the
  03:xx fit had 2 s, the local intercept 0.2 s) or off the write connection (unsafe).
  Both are named in the stall record; this record adds that writing less is the only
  thing that reduces total GC seconds, disk growth and the cold-read cost together.
- **Run memory is the wrong first lever for the GC and the right first lever for the
  write model**: it is the single stream whose cost the Dolt facts predict exactly
  (F/k + D) and whose batching and index set are within one table. F-c deploy 2 is
  the larger and the more certain lever for the paper: it removes 55 to 77% of a
  stream that is 35% of the journal, and it is already designed. **[H]**

## Ranking by risk-adjusted bytes saved

1. F-c deploy 2 (trace lines to `choir_events`): -19% to -27% of the journal; red; built through deploy 1.
2. Drop `idx_run_memory_entries_loop_seq` and `idx_..._owner_loop_seq`: 10.5 KiB of a tape append's 103 (10% of run memory, about -2.7% of the journal); orange to red; two plan tests and the access-path operation, the same mechanism as R5 and the kind index. The cheapest real byte saving, and the only one with no change to the tape.
3. Group append for the end of a cell (three rows, one statement, one event): -11% of the journal; red; needs a new projection op or a coalescing rule in `projectBatch`, and a replay proof.
4. R6 multi-row `og_objects` upserts: -14%; red.
5. Drop `idx_..._parent` (no Go reader): another 12.6 KiB (-12% of run memory, -3.3% of the journal); red because the witness sees `MUL` go.
6. An `og_objects` index audit: up to -5.7 KiB per op per unneeded tree on 73% of the journal; the largest unmeasured lever, **[H]**, read-only to size.

## Not settled

- The kind mix of the 4.3 other ops/s, and the cost of wake-outbox projected flips (the one-line counter in `recordTapeEvent`, journal record section 6, is still unbuilt).
- The paper's per-op constants on its own store (425,000 run-memory rows, 1.0M `og_objects` rows, cold reads): every number here is a warm local floor. A paper reading of `journal_mib` growth against `tape_ops_by_kind` over a GC interval settles it (the journal record's reading 2).
- Whether a delayed group of run-memory rows is acceptable to the run's own next read (it rebuilds its context from them each call, `run_memory.go:298-304`).
- That F-c deploy 2's ratio at the paper's depth is the same as at 20,000 rows; measured here at 20,000 events per table only.
- Noise: a UUID run varies +/- 4% per window; the `og_objects` index drops are +/- 5 KiB per step (three steps are non-monotonic).
- The F fit is from four points; it assumes a constant tape overhead.

## Heresy delta

- Discovered: the journal cost of a write is per statement and per tree, so batching at the transaction (the natural reading of "batch N appends") saves 3%; a duplicate index is nearly free in bytes (content-addressed chunks) though it still costs an address map and a tree to maintain; a Dolt commit and the row size are not terms; the tape adds a fixed 36 KiB per event that only ops per event amortise; every added tree level costs 9 to 16 KiB per append on 7 trees.
- Introduced: none (a yellow test, a green record).
- Repaired: none.

## Rollback

Revert the commit that adds `internal/store/journal_amplification_census_test.go` and
this record. No runtime, schema, data or index change.
