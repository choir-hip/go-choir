# Bounding the guest store (2026-11-14)

Status: design from code and earlier measurements. Nothing in this note is
measured on the 43.76 GiB file; the measurement that settles it is
`GET /api/runtime/observability?store_tables=detail` (built in this lane,
`internal/storetables`). Run it on the paper before building any bound below
past the first. Problem record:
[the paper's store doubles in eleven days](../problems/the-papers-store-doubles-in-eleven-days-2026-11-13.md).

Labels: **[C]** read from code at the cited place, **[M]** measured earlier
(`docs/design/small-object-storage-2026-10-31.md` section 1, on the paper's
20.9 GB file, `dbstat` on a clone), **[E]** estimated, arithmetic shown, **[?]**
unknown.

Mutation classes: every bound below that deletes or stops writing rows of
`state.db` is **red** (Texture canonical writes, tape and evidence, checkpoint
projection: the store is a projection that replay and the content witness
read), two seats, a conjecture delta, a rollback and a heresy delta. The
measurement is **orange** and built.

## 1. What the 44 GiB is made of (ranked)

The file is a projection of the tape (`docs/computer-ontology.md`, Store
Taxonomy: "a projection of the computer's tape, rebuilt by replay"). The
table anatomy at 20.9 GB [M]: `og_objects` 10.5 GB table + 5.5 GB in 17
indexes (3.34 M rows), `run_memory_entries` 4.46 GB with 3 indexes (1.82 M
rows), `og_key_index` 0.55, `og_edges` 0.12, everything else 0.13. At 3.6 KB
per row overall and 2 GiB a day, the file grows by about 600,000 rows a day,
which is the tape's rate (25,000 events an hour). So the file is growing at
about the rate rows arrive, which means **retirement removes little or
nothing of what arrives now**, or removes it into free pages that are not yet
returned.

| Rank | Component | Estimate of the 44 GiB | Estimate of the 2 GiB/day | Confidence | Settled by |
|---|---|---|---|---|---|
| 1 | `run_memory_entries` rows of runs that never reach "terminal and older than 7 days", and the 2 newest runs per agent | 12 to 18 GiB | 0.7 to 0.9 GB | medium | `tables[run_memory_entries].bytes` and `rows`; `rowid_max - rowid_min` against `rows`; then `SELECT count(DISTINCT loop_id)` and the state of the runs that hold most rows |
| 2 | `og_objects` rows of semantic and unlisted event kinds: nothing retires them (section 2, Q1) | 10 to 15 GiB | 0.5 to 0.7 GB | medium | per-kind row count from `idx_og_objects_kind_id` (a covering index walk), on request |
| 3 | The `og_objects` index share: 17 indexes, 1.65 KB per row against 2.2 KB of logical row [M]; every row kept pays it | 6 to 9 GiB (about a third of the `og_objects` rows' bytes) | 0.25 to 0.4 GB | high that it is large; it is a multiple of rank 2, not an independent cause | `tables[og_objects].index_bytes` vs `table_bytes` |
| 4 | Free pages SQLite holds: retirement deletes pages inside the file, and `reclaimFreePages` returns at most 250,000 pages (about 1 GiB) a day, and only on an `auto_vacuum=INCREMENTAL` file | 0 to 8 GiB | 0 (it is reuse, not growth) | low; the paper's file was rewritten with `auto_vacuum=2` on 11-01 and 11-02 [C], and nothing since has rewritten it | `header.freelist_count * page_size`, `header.auto_vacuum` |
| 5 | Page slack inside og/body overflow chains (2.55 GB of 20.9 [M]) | 3 to 5 GiB | about 0.1 GB | high | `unused_bytes` per tree |
| 6 | Retirement that is failing: a run of `RetireTelemetry` or `RetireRunMemory` that errors logs and stops for the day (`telemetry_retire.go:300-312`), gated on file-tree hydration (`autoputer/run.go:793,822`) and a restart resets the cleared watermark | up to the whole 14-day telemetry window (about 4 GiB) | 0.3 GB | low, no log read | `store: telemetry retirement:` and `run memory retirement:` lines in the guest log; `og_objects` row count by `event_kind` and age |
| 7 | `og_key_index`, `og_edges`, `computer_event_index` (windowed), actor-side rows | under 2 GiB | under 0.1 GB | high | `tables` |

Ranks 1 to 3 are the components that can account for 40 GiB. Rank 4 decides
whether a bound shrinks the file or only stops its growth.

The doubling date matters: on 11-02 the file already held the full 14-day
telemetry window and was freshly rewritten, so the growth since is rows no
group in `telemetryRetireGroups` names (rank 2), run memory the selection
does not take (rank 1), and the indexes those rows pay for (rank 3).
Telemetry the job does retire is bounded: it is the window, not the growth.

## 2. The answers behind the ranks

### Q1. Tables, what grows, what deletes

`schema.go` and `objectgraph/sql_store.go` create these; the rest of the
tables of `state.db` are small or windowed.

| Table (with indexes) | Row holds | Per row, with indexes [E from M] | Writer | Delete path |
|---|---|---|---|---|
| `og_objects` header (`sql_store.go:230`) + `og_bodies` (`:257`), 17 indexes (`:195-215`) | one object: id (about 140 B, in 13 of 17 indexes), kind, owner, metadata JSON, times, body BLOB | 2.2 KB logical, 4.8 KB physical [M: 16.05 GB / 3.34 M] | the tape projector (`ProjectionOpObject`), every kind including `choir.event` | `ProjectionOpObjectRetire` only: `choir.event` rows of named kinds (`computerevent/projection_batch.go:206-209`, validated at `:267`); other kinds are never deleted except by a tombstone-and-supersede on the same row |
| `run_memory_entries` + `idx_run_memory_entries_parent`, `_kind`, `_loop_retire` (`schema.go:94-357`) | one message of a run's conversation: `message_json` (2.2 KB average), role, kind, details_json | 2.45 KB [M] | `ProjectionOpRunMemoryEntry` | `ProjectionOpRunMemoryRetire` (`run_memory_retire.go:216`): whole runs, terminal and last written over 7 days ago, not one of the newest 2 per agent, at most 5,000 runs a day (`:44-52`, `:155`) |
| `og_key_index` + 2 indexes (`derived_keys.go:72`) | derived key per object | 0.9 KB [M] | the same projector | dropped with its object (`retireDropBatch`) |
| `og_edges` + 3 indexes | edge | 0.9 KB | projector | none that I found |
| `choir_events` + `choir_event_bodies` (`schema.go:33`) | the event table | not in use: `store.event_table` is off and `choir_events` holds 0 rows (`store.split.summary`) | none | n/a |
| `computer_event_index` | one tape event's index row | windowed to about 68 k rows | the appender | `event_index_trim.go` (a pure function of the head) |
| `texture_*`, `events`, `desktop_*`, `media_*`, `self_development_*` | small; Texture lives in `og_objects` | small [M: one page each] | | |

Telemetry partitions (`telemetry_partition.go`) are separate files and are
not part of the 43.76 GiB; `store.telemetry_partitions` is off by default
(`computer_setting.go:233`). The 11-12 snapshot trace counts about 5,170
partition chunks in a root, so the paper has them on; that is a separate
byte count under `telemetry/` and is not in `state.db`.

### Q2. Is deleted space returned?

Not by delete. `retireOGTable` deletes rows and pages go to the freelist.
Three things return space:

1. `auto_vacuum=INCREMENTAL` (value 2) and `PRAGMA incremental_vacuum`:
   `reclaimFreePages` (`compact.go:974-1012`) runs after each daily
   retirement (`telemetry_retire.go:314`), at most 250,000 pages (1 GiB)
   a run, 2,000 pages a chunk with a 100 ms pause. It does nothing on a file
   whose `auto_vacuum` is not 2. The paper's file is 2: the 10-31 compact
   copy set it (`compact.go:407`) and the 11-02 header/body split's new file
   sets it again (`header_body_split.go:816-817`).
2. The compact copy (`VACUUM INTO`, `compact.go:332`), swapped in at the next
   planned restart; it needs 1.2 times the live size free in the guest
   (`compact.go:314-322`), and runs only when the freelist is over 10% or
   2 GiB, or the file is over 1 GiB and not rewritten in 7 days.
3. Reuse: a free page is the next insert's page, so a file that deletes as
   fast as it inserts does not grow.

The `wal` reading: `wal_bytes_before 2,613,274,832` is the WAL file's size
when the checkpoint after the snapshot ran (`wal_bound.go:96`). The snapshot
capture holds one read transaction for 37 minutes, so no checkpoint can pass
its mark and every commit made meanwhile stays in the WAL (the paper commits
all the time). `journal_size_limit` (64 MiB, `dbx/dbx.go:69-96`) is applied
only when the WAL restarts from the top after a checkpoint completes, so it
trims the file after the read ends, not during it. The 2.6 GB is the
37-minute snapshot's pinned writes; it is the same cause as the churn of
every 4 MiB chunk changing, not a leak. It is not in the 43.76 GiB.

### Q3. Projection or canonical

All of `state.db` is a projection of the tape by the doctrine, with the
exception of the rows the tape-bound store still writes as direct SQL while
their graph switches are off (`co_super_slots`, `agent_evidence`, per
`wal_bound.go:40-44`; both small). So any row the tape recorded can leave
the working store if (a) the tape still holds it and (b) nothing reads it.
Existing retire ops already rely on that (`telemetry_retire.go:1-8`:
"the row stays on the sealed tape"). Replay agrees because the op names what
it removes.

Rebuildable with no loss, by replay, a snapshot, or a read of the tape:
`choir.event` rows (tool calls, model calls, `model.served`, `desk.cell`,
`loop.progress`), the `og_key_index`, every index, finished runs' run memory.
Canonical in the guest only, in the sense that the guest's replay result
must equal them: the rows of the semantic kinds (Texture revisions,
commitment records, lifecycle commands, run records, source refs) which
products read; these may only be bounded by tiering, not by deletion.

### Q4. Duplication

- A tool's output exists in the tool's `tool.result` event twice and a third
  time in run memory: `toolregistry/batch_executor.go:112-135` writes
  `output` (at most 100 KB, `:210`), `full_output` (at most 512 KB, `:277`)
  and the call arguments into one event payload; the same text is the
  tool-result message in `run_memory_entries.message_json`. [C for the three
  copies; the share of the file is unmeasured.] `tool.result` retires at 7
  days except the cited rows; run memory retires at 7 days after the run is
  terminal, so the duplication is bounded only if both retire.
- `model.served` and the per-call `loop.progress phase tool_loop` carry the
  same facts; the comment on `types/model_served.go:5-14` says the second
  is retired at 14 days and the first is "small and kept". It is not on the
  retire list, so it stays forever. [C]
- Large bodies are inline: `og_bodies.body` BLOB, overflow chains for bodies
  over about 4 KB (600 k of 2.57 M table pages are overflow [M]). Web
  captures keep blobs by id (`objectgraph/web_capture.go:19-37`:
  `ContentBlobID`, `ExtractedTextBlobID`), not inline. Model-call
  transcripts are the run memory rows. Fetched pages in a tool result are
  inline in the tool result and run-memory rows (up to 100 KB each).

### Q5. Mechanisms built, off or unwired

| Mechanism | State | Evidence |
|---|---|---|
| `store.event_table` (`choir_events`: 4 b-trees an insert instead of 13) | built, off; `choir_events` 0 rows | `computer_setting.go:63,222`, `agentcore/event_table.go:68`; needs a host grant too |
| `store.telemetry_partitions` (tool.result per-day files, retired by dropping the file, no row deletes) | built, default off; the paper has it on in part [I from the 11-12 trace] | `computer_setting.go:233`, `agentcore/telemetry_partitions.go:25` |
| `store.telemetry_retire` | on by default | `computer_setting.go:232` |
| `store.compact_force`, layout-age compaction | on by the layout rule (7 days); needs 1.2x live free and a planned restart | `compact.go`, `docs/design/paper-compaction-2026-10-31.md` |
| `store.tape_group_commit` | off | `computer_setting.go:234` |
| `RetireRunMemory` | wired, daily, limited to terminal runs | `telemetry_retire.go:305` |
| `vm-state/store-compact-attempts.json` | dead record, the host routine was deleted 10-30 (`docs/problems/node-b-storage-clustering-assessment-2026-11-11.md`) | |

No retire op exists for: `model.served`, `search.served`, `loop.delta`,
`loop.completed` and the other run lifecycle events, `channel.message`,
`calibration.read`, `disclosure.decision`, `trust.breaker` and the rest of
`types.EventKind` not named in `telemetryRetireGroups`
(`telemetry_retire.go:88-93`); and none for an object of any other kind.

### Q6. What a per-table reading costs

`dbstat` is compiled into the driver (`modernc.org/sqlite v1.49.0`; it is
read by existing tests, `churn_attr_test.go:55`, `derived_key_marks_test.go:182`).
It has no cheap form: it visits every page of the b-tree it names. A reading
of all trees reads the whole file once, the same read as a snapshot capture
(which already reads all 44 GiB every attempt, and ran 37 minutes). What is
O(1) or O(log n): `page_count`, `freelist_count`, `page_size`, `auto_vacuum`
(header), and `min(rowid)`, `max(rowid)` of a rowid table (b-tree edge). So
the reading is built bounded, on request only, and says partial when cut.

## 3. Candidate bounds, by bytes saved per unit of risk

Do the measurement first. Bounds 1 to 3 are wiring and extending what exists;
none adds a switch (CLAUDE.md "no switches").

### Bound 1. Retire run memory of every finished run, whatever its state name, and by size, not only by age

- **What it deletes.** `run_memory_entries` rows of runs that are not live:
  today only a run in `completed`, `failed` or `cancelled` whose last entry is
  over 7 days old and which is not one of the newest 2 of its agent
  (`run_memory_retire.go:167-196`). Candidates are the runs that never reach
  those states: `passivated` and `blocked` runs nobody resumed (the
  selection skips them, `:181-187`), runs the store cannot see
  (`ErrNotFound`, skipped, `:178-180`), and the newest-2-per-agent keep on a
  paper whose agents number in the thousands. The first change is to read
  which of these hold the rows (the reading, then a group query on a copy),
  then give each class a bound: a passivated run is retired after N days
  without a wake.
- **Why safe.** Every entry reached the working store from a
  `ProjectionOpRunMemoryEntry` on the sealed tape (`run_memory_retire.go:3-6`),
  and the op names the runs it removes, so replay agrees. A retired run's
  entries answer `get_run_memory_entry` not found, a visible refusal.
- **Class.** Red (run memory is evidence-adjacent and the retire op is a
  projection rule). Two seats.
- **Bytes.** Rank 1: 12 to 18 GiB if confirmed, 0.7 to 0.9 GB a day.
- **Proof.** On a copy of the paper's store (a restored snapshot), run the
  selection and count the rows it would take; then on staging the daily
  retire log line `run memory retirement: N runs`, and `store_tables`
  `run_memory_entries.rows` falling at the next daily reading; then the
  compact copy's size.
- **Rollback.** Revert; deleted rows are on the tape and come back by replay
  or restore of a snapshot before the op.

### Bound 2. Retire the event kinds nothing reads: add `model.served`, `search.served`, `loop.delta`, and the lifecycle events of finished runs to `telemetryRetireGroups`

- **What it deletes.** `choir.event` rows of the kinds named, after a window
  (14 days like the trace kinds), keeping cited rows (`keep_cited`).
- **Why safe.** The mechanism and the op are built and replay-safe;
  `ObjectRetireKindEvent` already restricts the op to `choir.event`
  (`computerevent/projection_batch.go:206-209`); the row stays on the tape.
  Each added kind first needs its reader checked in code (the comment block
  `telemetry_retire.go:60-80` shows the standard: name each reader). The cost
  page reads `model.served`? [?] check before listing it.
- **Class.** Red (the retire op's policy list is a projection rule), two
  seats; the list is data in one table, so the change is small.
- **Bytes.** Rank 2: up to 10 to 15 GiB if the unretired event kinds are the
  bulk of it, which the per-kind count settles.
- **Proof.** Same as bound 1; per-kind row counts before and after.
- **Rollback.** Revert; rows return by replay from the tape.

### Bound 3. Make the retired space leave the file: raise the incremental vacuum bound and run the compact copy when the freelist is large

- **What it deletes.** Nothing; it returns pages. `reclaimFreePages`
  returns at most 250,000 pages (about 1 GiB) a day (`compact.go:105`);
  after bounds 1 and 2 free a large share the daily return must exceed the
  daily delete or the file stays at its high-water mark. Raise
  `incrementalVacuumMaxPages` as a function of the freelist (the pacing is
  already there: 2,000 pages, 100 ms), not a fixed 250,000. And the compact
  copy needs 1.2x the live size free in the guest
  (`compact.go:314-322`): at 44 GiB live on a 125.7 GiB volume with 69 GiB
  available that is 52.6 GiB, which fits barely; at any growth it does
  not. Compaction should not be the way back from 44 GiB; `VACUUM INTO`
  of a file that big is a 37-minute read at best.
- **Why safe.** `incremental_vacuum` moves pages and updates the pointer
  map; no row changes; the witness covers rows, not layout
  (`docs/design/small-object-storage-2026-10-31.md` F2). Bounded by chunks.
- **Class.** Orange (store maintenance with no row effect), one seat.
- **Bytes.** Only after bounds 1 and 2, up to the freelist's size.
- **Proof.** `header.freelist_count` and `file_bytes` in `store_tables`
  before and after a daily run.
- **Rollback.** Revert; the file just stays larger.

### Bound 4. Fewer indexes on `og_objects` (rank 3)

- **What it deletes.** Indexes, not rows: 17 indexes at 1.65 KB a row is
  about a third of the table. The ones that serve one kind's scan (the
  record-scoped `og_rec_*`, the `*_recent` indexes) are partial, so they
  cost only the rows with the column; the five unconditional ones
  (`sql_store.go:195-201`) cost every row. Candidate: drop
  `idx_og_objects_owner_computer_kind` or `idx_og_objects_kind_owner_computer`
  (one is the other with the leading columns swapped) after reading which the
  planner uses. Rebuilding an index reads the table: do it in the
  split-migration pattern, one at a time.
- **Why safe.** An index is derived; dropping it changes plans, not data.
  Risk is a hot query losing its index (the 2026-10-14 CPU incident,
  `docs/archive/problems/guest-cpu-is-unindexed-store-scans-2026-10-14.md`).
- **Class.** Red (query plans on the guest's hot path; a store schema
  epoch), two seats.
- **Bytes.** 1 to 2 GiB per dropped unconditional index at the paper's row
  count [E: 0.6 to 0.9 GB each at 3.3 M rows, now doubled].
- **Proof.** `EXPLAIN QUERY PLAN` of every query that named the index on a
  copy; `store_tables` index bytes after; CPU and read-bytes over a day.

### Bound 5. Store the second and third copy of a tool's output once

- **What it deletes.** `full_output` when equal to `output`, and, once, the
  run-memory copy of a tool result already on the tape (a reference to the
  event id and digest).
- **Why safe.** It is a write-shape change: a different stored form for new
  rows, with readers taught to follow the reference. It touches the evidence
  record (run acceptance cites tool results), so it needs the longest
  review.
- **Class.** Red; do it last, after the reading says tool results are a
  large share.
- **Bytes.** Unknown until measured; the three copies suggest up to two
  thirds of tool-result bytes.

### Not recommended now

- Enabling `store.event_table`: 4 b-trees an insert instead of 13 for ten
  kinds saves the index share of new events, but it moves rows between
  tables and the host grant makes deploy 1 a rollback floor
  (`computer_setting.go:54-63`); it does not delete the 44 GiB.
- A new archive tier: a second store (CLAUDE.md "one home per kind"). The
  tape is the archive.
- Deleting semantic kinds. Their reader is the product.

## 4. Order and proof

1. Land `store_tables` (this lane; orange, one seat). Read it on the paper:
   `?store_tables=detail` at a quiet hour, one reading, 60 s; repeat with
   `store_tables_seconds=100`. Record: header, the top ten trees, the roll-up.
2. Run the per-kind row count and the run-memory selection on a restored
   copy, not the paper (the parent task's rules).
3. Build bounds 1 and 2 as one change (one policy table, one run-memory
   rule), then bound 3.
4. After each: the reading again, the compact copy's staged size, the next
   snapshot's size and duration, the data image's allocated bytes on the
   host.

Success is a file whose size follows the retention windows, not the age of
the computer: the same reading a week apart shows the same bytes.
