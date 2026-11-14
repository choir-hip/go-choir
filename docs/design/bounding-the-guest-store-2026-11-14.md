# Bounding the guest store (2026-11-14)

Status: revised 2026-11-14 from named-tree readings of the paper's 47.5 GB
file (section 0 and its ranking, 0.5). Sections 1 to 4 are the earlier
design, written before those numbers existed, and are marked superseded where
section 0 replaces them. The earlier note said: nothing in it is
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

## 0. Measured 2026-11-14 (this section replaces the ranking below)

Named-tree `dbstat` readings on the paper (the busiest computer; guest store
`state.db`, 47.5 GB), 05:30Z to 05:58Z, by `store_tables=detail&store_tables_only=<names>`.
A tree marked "cut" hit the 95 s budget; its number is a floor.

| Tree | Bytes | Note |
| --- | --- | --- |
| `og_bodies` (table) | at least 12.43 GB | cut after 5.15 M rows; payload 10.0 GB; 932 k overflow pages |
| `run_memory_entries` (table) | at least 11.17 GB | cut after 4.63 M rows; payload 9.87 GB; 1.24 M overflow pages |
| `run_memory_entries` indexes (5) | 2.51 GB | complete |
| `og_objects` (table) | 5.42 GB | complete; 5.98 M rows; payload 4.43 GB |
| `og_objects` indexes (19) | at least 7.73 GB | four cut: `idx_og_objects_owner_computer_kind`, `sqlite_autoindex_og_objects_1`, `idx_og_objects_kind_id`, `idx_og_objects_kind_owner_computer` |
| `og_key_index` and its indexes | 1.38 GB | 1.44 M rows |
| `og_edges`, `computer_event_index`, `texture_agent_mutations` | 0.29 GB | |

At most 29.9 GB is left for the two cut tables together (floors 12.4 and
11.2). The object graph (`og_*`) is at least 27 GB. Run memory with its
indexes is 13.7 to 20 GB. The store grows about 2 GiB a day. On the small
test computer (d@e, 1.11 GB): `run_memory_entries` 38%, `og_bodies` 30%,
`og_objects` 25% (two thirds of that is indexes).

Tape-side facts for context, first hour after group commit deployed:
run-memory ops by kind: message 15,563; cell_fate/tray committed 1,979;
compaction/actor_rewarm 59; compaction/threshold 15. Per-op tape bytes:
`choir.event` 20.3%, `choir.run` 19.4% (23.5 KB per op, 1,586 ops, a mutable
object rewritten whole).

What this settles about the estimates in sections 1 to 3 below:

- Rank 1 (run memory) is confirmed as the largest named component and larger
  than the 12 to 18 GiB estimate's low end: 13.7 GB at the floor.
- Rank 3 (the `og_objects` index share) was too small a claim: the indexes are
  larger than the table (7.73 GB or more against 5.42 GB). Claim C81 settled
  false on the same readings (the two big tables hold 55% to 68%, not 70%).
- Rank 2 (unretired event kinds) is unmeasured; the readings do not split
  `og_bodies` or `og_objects` by kind.
- Rank 4 (free pages) is not supported: the readings show a freelist near 0
  on an `auto_vacuum=2` file. The file is not holding returned-but-unreclaimed
  space; the deletes are being reused as fast as they arrive, or they are not
  happening.

### 0.1 What bounds `run_memory_entries` today (read from code)

The only delete is `ProjectionOpRunMemoryRetire`
(`run_memory_retire.go:58-82`): `DELETE FROM run_memory_entries WHERE loop_id
IN (...) AND kind NOT IN ('rlm_inbox_cursor')`. The daily job that appends it
is `StartTelemetryRetirement` (`telemetry_retire.go:275-331`).

Which rows are eligible (`selectRunMemoryPlan`, `run_memory_retire.go`; the
rule after R-1, docs/problems/run-memory-retirement-keeps-nearly-everything-
2026-11-14.md). A run's rows are retired only when all hold:

1. The run has at least one entry that is not `rlm_inbox_cursor`.
2. Its newest entry is older than 7 days (`runMemoryRetireWindow`).
3. It is not a carry source: for each owner's document custodian agent
   (`deskmanifest.IsDocumentInbox`), the newest `runMemoryKeepPerAgent = 2`
   runs whose record is `completed` or `passivated`, ordered by `FinishedAt`
   else `UpdatedAt` as `LatestActorRunMemoryEntries` orders them, keep their
   entries whatever their age. A failed, cancelled or record-less run takes no
   slot; a record that does not decode is protected. No other agent's run is
   protected: nothing reads it.
4. The run record is read through the read pool by its owner-scoped id and by
   its `run_id` metadata (lifecycle runs live under a computer-scoped id the
   owner-scoped read cannot build). A run with a record is judged by rules 5
   and 6. A run with no record retires when its newest entry is older than 30
   days (`runMemoryRetireNoRecordWindow`; the design asked two windows, this
   is the longer hold until a census on the paper shows what remains). A
   record that does not decode keeps the run.
5. The run's state is `completed`, `failed` or `cancelled`. A `passivated`,
   `blocked`, `pending` or `running` run is never retired, however old.
6. The run's finish time (`FinishedAt`, else `UpdatedAt`) is older than 7 days.

Rows of kind `rlm_inbox_cursor` are never deleted (`:53`).

Pacing and size of one pass.

- Per pass at most `runMemoryRetireMaxOps * RunMemoryRetireMaxRuns` = 50 x
  100 = 5,000 runs (`run_memory_retire.go:51,173`;
  `computerevent/projection_batch.go:321`).
- Ten runs to an op (`runMemoryRetireRunsPerOp`, `:214`), so at most 500 ops
  in a pass. One op is one transaction on the one writer.
- After each op the job rests `max(250 ms, 4 x the op's own time)`
  (`:246`, `telemetry_retire.go:33,36`). The writer is held about a fifth of
  the time. At 2 s an op, 500 ops take about 83 minutes.
- The number of rows per pass is not bounded, only the number of runs. A run
  that carried a conversation holds hundreds of rows
  (`carryActorConversation`, `agentcore/run_memory.go:165-217`: the previous
  run's whole tail is copied, "history is kept, never trimmed").
- The selection reads the covering index `idx_run_memory_entries_loop_retire`
  whole (`GROUP BY loop_id` over every row, `:118`) on each pass.

When the pass runs.

- `StartTelemetryRetirement` waits 10 minutes after boot, then runs every 24 h
  (`telemetry_retire.go:28,30,283-298`). A restart resets the timer to 10
  minutes; the paper restarts on every deploy.
- Run-memory retirement is the second step of the pass, after
  `RetireTelemetry` returns without error (`:298-310`). An error in the
  telemetry step logs and skips run memory for that process until the next
  24 h tick (`:299-302`; `wait` was set to 24 h before the run, `:296`). A
  restart during the telemetry step cancels the context and the pass ends
  there.
- The pass is gated on file-tree hydration being settled and on the
  `store.telemetry_retire` setting, default on (`autoputer/run.go:793-794,
  822-823`; `computer_setting.go:159`; `graph_home.go:33-49`, which answers
  false with no owner id, no projection tape, or a read error).
- An interrupted run-memory pass keeps what it did: each op is committed and
  the selection reads the table, not a watermark, so the next pass takes the
  remaining runs. So a pass that is cut during run memory still makes
  progress; a pass that is cut during the telemetry step makes none for run
  memory. The telemetry step restarts from the oldest candidate after every
  boot (`retireCleared` is in memory, `:116,148-156`) and does at most 2,000
  hour slices per group (`:39,162`).
- Whether run-memory retirement has ever completed on the paper is not
  known from code. No retirement counter is served by
  `GET /api/runtime/observability`: the only evidence is the guest log lines
  `store: run memory retirement: N runs in M ops`, `store: telemetry
  retirement: ...`, `store: run memory retirement: after N ops: <err>` and
  `store: incremental vacuum: N pages returned` (`telemetry_retire.go:300-321`;
  `run_memory_retire.go:254`). Served and relevant: `store_tables.header`
  (`freelist_count`, `auto_vacuum`, `file_bytes`) and, per tree, `rowid_min`
  and `rowid_max` (`internal/storetables/storetables.go:111-147`;
  `agentcore/api.go:958-966`).

Do retired rows free pages. A delete puts pages on the freelist. On a file
with `auto_vacuum=2`, `reclaimFreePages` runs `PRAGMA incremental_vacuum`
after the daily retirement (`telemetry_retire.go:317`; `compact.go:978-1012`):
2,000 pages a chunk, 100 ms apart, at most 250,000 pages a run (`compact.go:
102-105`; about 1 GiB at 4 KiB pages, page size not read here). It returns 0
on a file whose `auto_vacuum` is not 2 (`:983-985`). It is the last step of
the pass, so it is skipped whenever the telemetry or run-memory step returns
an error (`:300-310` `continue`). New inserts reuse free pages first, so a
store that inserts 2 GiB a day shows a freelist near 0 whether it deleted
little or a lot.

Most likely reason the table is this large (a hypothesis until the read).
Arithmetic: 15,563 message ops in the first hour is about 374,000 rows a day;
seven days of them is about 2.6 M rows, about 7.6 GB with indexes at the
measured 2.41 KB a row in the table plus about 0.5 KB of index. The table
holds at least 4.63 M rows and at least 13.7 GB with indexes. So even if the
retirement runs, at least about 6 GB sits outside a 7-day window, and the
real surplus is larger because the table reading was cut. The code names three
places rows can sit past the window:

1. Runs that are `passivated` or `blocked` and never resumed, which rule 5
   excludes. A planned restart passivates every running run
   (`agentcore/runtime.go:3095-3115`), so every deploy adds to them, and the
   Texture reader itself takes the newest `completed` or `passivated` run
   (`store/run_memory.go:329`), so passivated is a normal end state.
2. The newest 2 runs of every agent, forever, however old (rule 3). Texture
   documents each have their own agent, and each wake copies the whole
   conversation tail into a new run, so a dormant document keeps two full
   conversations. The reader that needs this is the document custodian only
   (`run_memory.go:136-146`; `run_memory_retire.go:10-16`).
3. Runs the store cannot find (rule 4).

The single read that would confirm which: the group query the job itself
runs, joined to the run state, on a restored copy of the store (not the live
file): rows and runs by (state, in the newest 2 of its agent or not, older
than 7 days or not). It is the same `SelectRunMemoryRetirements` logic as a
count. On the live computer the counter-only version is two
`store_tables=detail&store_tables_only=run_memory_entries` readings 24 hours
apart: if `rowid_min` does not rise between them, old runs are not being
retired at all; if it rises while `rows` does not fall, the retirement keeps
pace with arrivals but never gets behind them.

### 0.2 What is in `og_bodies`, and what is ever deleted (read from code)

`og_bodies` (`objectgraph/sql_store.go:252-263`) holds one row per
`og_objects` row of any kind, keyed by `obj_id`; a trigger deletes the body
when its header is deleted (`:261-262`), so a retired object takes its body
with it. The only code that deletes `og_objects` rows besides the one-object
`DeleteObject` (`sql_store.go:1886`, one id) and the vocabulary rewrite
(`store/vocab_migrate_og.go:773`) is `ProjectionOpObjectRetire`
(`store/object_retire.go:191`), and the op is validated to name `choir.event`
only (`computerevent/projection_batch.go:206-209,267`).

Retired event kinds today (`telemetry_retire.go:88-94`): `tool.invoked` and
`texture_agent_revision_progress` at once; phaseless `loop.progress` at once;
`desk.cell` and `desk.egress` after 14 days; `tool.result` after 7 days except
rows a run acceptance cites; phased `loop.progress` after 14 days. Every other
event kind (for example `model.served`, `search.served`, the run lifecycle
events) is kept forever, and so is every object of every other kind (Texture
documents and revisions, lifecycle commands, commitment records, run records,
source refs, work items). The tape holds all of them.

I did not find a kind that is written once and never read after N days with
a deletion rule; that is the candidate class for bound R-5 and it needs the
per-kind counts, which no served counter gives. Objects written whole and
rewritten in place (`choir.run`, 23.5 KB a write on the tape) do not grow the
store: the overwrite reuses the row. What the readings cannot say is which kinds
account for the 10.0 GB of body payload; `store-once-2026-11-14.md` names the
suspects (lifecycle commands that embed the revision, revisions with both
the structured body and the plain text).

### 0.3 The 19 indexes of `og_objects` (read from code)

Defined in `objectgraph/sql_store.go:194-214` (`objectIndexes`), created by
`CREATE INDEX IF NOT EXISTS` in `SchemaDDL` at every open
(`sql_store.go:276-292,385-392`). The test fixture
`store/testdata/presplit/schema.sql` is the pre-split layout and is stale for
the current set. Nineteen are the unique key plus 18 named ones.

| # | Index | Columns, predicate | Queries that need it |
| --- | --- | --- | --- |
| 1 | `sqlite_autoindex_og_objects_1` | `canonical_id` unique, all rows | every `GetObject` and write conflict (`sql_store.go:574,1843,1886`); `KindIDRangePageSQL` (`newest_headers.go:91`); delete by id (`object_retire.go:191`) |
| 2 | `idx_og_objects_updated` | `updated_at`, all rows | `NewestObjectHeadersSQL` (`newest_headers.go:19`, the stream-seq seed); `ListObjects` with no kind (`sql_store.go:611`) |
| 3 | `idx_og_objects_kind_id` | `object_kind, canonical_id`, all rows | `ListObjectsPage` (`:1180`); `KindMetadataPageSQL` and `KindOutsideRangeSQL` (`newest_headers.go:92-93`); `ObjectKindExistsSQL` (`:22`) |
| 4 | `idx_og_objects_owner_computer_kind` | `owner_id, computer_id, object_kind, canonical_id`, all rows | `ReadObjectSnapshot` and the filtered snapshot (`sql_store.go:651,681,696`); `ListObjects` with owner and computer |
| 5 | `idx_og_objects_kind_owner_computer` | `object_kind, owner_id, computer_id, canonical_id`, all rows | `ListObjectsByKindOwnerComputerPage` (`:1218`) |
| 6 | `idx_og_objects_kind_owner_updated` | `object_kind, owner_id, updated_at`, all rows | `ListObjectsByKindOwnerWhere` (`:1556`); `ListObjectsByOwnerAndBody` (`:1418,1438`); `ListJSONBodyFieldsByKindOwner` (`:1693`); `KindUsage` (`:1366`) |
| 7, 8 | `..._og_meta_run_id`, `..._recent` | `object_kind, og_meta_run_id, canonical_id` and `..., updated_at`; partial `og_meta_run_id IS NOT NULL` | `ListObjectsByMetadataPage` (keyset, `:1139`) and `ListObjectsByMetadata` (newest first, `:1115`); callers `store.go:1329,1518,2516`, `graph_store.go:661` |
| 9, 10 | `..._og_meta_trajectory_id`, `..._recent` | same shape on `og_meta_trajectory_id` | the same two reads; `trajectoryRefsSQL` (`:766`); callers `store.go:794,1697,1815,2073`, `engineering_assignments.go:1478,2511,2676`, `lifecycle.go:1602,2630` |
| 11, 12 | `..._og_meta_channel_id`, `..._recent` | same shape on `og_meta_channel_id` | the same two reads; callers `store.go:744,1652,1746,2317` |
| 13 | `..._og_rec_inbox` | `object_kind, og_rec_inbox, canonical_id`; partial, commitment records only | `listInboxRecordsBy` field `inbox` (`inbox.go:990,1045,1177,1443`) |
| 14 | `..._og_rec_sender_inbox` | same, `sender_inbox` | `inbox.go:1424` |
| 15 | `..._og_rec_conversation_id` | same, `conversation_id` | `inbox.go:1141,1146` |
| 16 | `idx_og_objects_rec_inbox_seq` | `object_kind, og_rec_inbox, og_rec_inbox_seq`; partial | `ListRecordsByInboxSeq` (`:1448-1454`; `inbox.go:1038`) |
| 17 | `idx_og_objects_rec_claim` | `object_kind, owner_id, og_rec_claim, updated_at, canonical_id`; partial `og_rec_claim IS NOT NULL` | `ListCommitmentClaimObjects` (`:1481`; `commitment.go:179`) |
| 18 | `idx_og_objects_choir_event_updated` | `updated_at`; partial `object_kind = 'choir.event'` | `retireOldest` (`object_retire.go:278`, pinned `INDEXED BY`) |
| 19 | `idx_og_objects_choir_event_retire` | `event_kind, updated_at, event_phase`; partial `object_kind = 'choir.event'` | every telemetry retire slice (`object_retire.go:57`, pinned) |

Findings.

- No index of the 19 is unused: each has a named reader above. (In
  `run_memory_entries` one is: see 0.4.)
- Redundant on a single-tenant guest. The guest store holds one owner and one
  computer (`CHOIR_OWNER_ID`), so `owner_id` and `computer_id` are constant
  and index 4 orders rows the same way as `(object_kind, canonical_id)`,
  index 5 the same as index 3, and index 6 is `(object_kind, updated_at)`.
  Indexes 3, 4 and 5 then carry one ordering three times. Whether the guest
  can hold more than one owner or computer id is not read here; the read that
  settles it is `SELECT count(*) FROM (SELECT DISTINCT owner_id, computer_id
  FROM og_objects)` on a restored copy. If it is 1, indexes 4 and 5 can go,
  with `ReadObjectSnapshot` and `ListObjectsByKindOwnerComputerPage` taking
  indexes 3 and 1 and filtering owner and computer on the fetched row (the
  header fetch is needed anyway for a full object read). The hazard is
  the planner, as in the 2026-10-14 incident
  (`docs/archive/problems/guest-cpu-is-unindexed-store-scans-2026-10-14.md`,
  and `trajectoryRefsSQL`'s own comment, `sql_store.go:760-764`).
- Overlap for events: while `store.event_table` is off and events live in
  `og_objects`, every event row is in indexes 1 to 6, 18, 19, and in 7 to 12
  where its metadata names a run, trajectory or channel: up to about 13 entries
  an event (the count `computer_setting.go:54-63` and the earlier version of
  this note give for the event table). Index 18 is a subset of index 2 by
  construction. Making index 2 partial (`WHERE object_kind <> 'choir.event'`)
  would stop the planner using it for `NewestObjectHeadersSQL` and for
  `ListObjects` with no kind, so it is not a free change. Whether the paper's
  `og_objects` rows are mostly events is not measured: `choir_events` is not in
  the readings and the setting default is off (`computer_setting.go:209`).
- Could be partial: 7 to 17 already are. Nothing else qualifies without a
  query change.
- The paired indexes (7/8, 9/10, 11/12) are both read: the `_recent` one for
  the newest-first list, the other for the keyset page. Merging them is a
  query change, not a drop.
- Sizes. Not measured per index: the four full indexes the readings cut are
  the biggest, and the lower bound on all 19 is 7.73 GB (1.29 KB a row over
  5.98 M rows, before the cut ones are completed). Estimate by key width, not
  measurement: indexes 4 and 5 are about 0.4 to 0.8 GB each (5.98 M entries of
  owner, computer, kind and canonical id, 70 to 130 bytes with the rowid and
  cell overhead); 3 about 0.3 to 0.5 GB; 1 about 0.3 to 0.4 GB. The read that
  replaces these: `store_tables_only=idx_og_objects_owner_computer_kind,
  idx_og_objects_kind_owner_computer,idx_og_objects_kind_id,
  sqlite_autoindex_og_objects_1` with a long `store_tables_seconds`.

Cost of changing an index on a 6 M-row table in the guest.

- Create: no index is created by a migration step today. `SchemaDDL` runs
  `CREATE INDEX IF NOT EXISTS` at every open on the one write handle
  (`sql_store.go:385-392`), so a new index is built synchronously at boot,
  reading the 5.4 GB header table and sorting 6 M entries before the process
  serves. How long is not measured; the 2026-10-31 index catch-up stalled the
  paper's store at 97% (`telemetry_retire.go:187-189`), which is why the
  split migration builds indexes one at a time (`header_body_split.go`).
- Drop: nothing drops an `og_objects` index today (`DROP INDEX` appears only in
  the partition-file migration, `store/partition_split.go:70`, inside its own
  transaction). A drop is one write transaction on the single writer that
  frees every page of the index; its time scales with the index's pages
  (0.4 to 0.8 GB is 100 k to 200 k pages) and is not measured: measure it on
  d@e first. The freed pages go to the freelist and leave the file only
  through `reclaimFreePages`, at most 250,000 pages a day.
- Removing the index from `objectIndexes()` is also required in the same
  change, or the next open recreates it. The reverse is the rollback cost: an
  older binary that opens the store rebuilds every missing index at boot,
  synchronously.
- How schema changes are applied: the DDL above at every open; data
  migrations (header and body split, partition split, compaction rebase) are
  named steps with a store schema floor written first
  (`storeschema.WriteFloor`; `header_body_split.go:598`, `event_table.go:554`)
  so an older binary refuses the file.

### 0.4 The indexes of `run_memory_entries` (read from code)

Five trees, 2.51 GB complete: the primary key autoindex on `entry_id`, the
`UNIQUE(loop_id, seq)` autoindex, `idx_run_memory_entries_parent`
(`parent_entry_id`), `idx_run_memory_entries_kind` (`owner_id, agent_id, kind,
created_at`) and `idx_run_memory_entries_loop_retire` (`loop_id, kind,
created_at, owner_id, agent_id`) (`schema.go:94-113,188,356-357`).

- `idx_run_memory_entries_parent` has no reader: no statement in the
  repository filters on `parent_entry_id` (`grep` of non-test code finds it
  only in column lists and the entry's own fields; its only mention in
  prose is the id comment, `run_memory_ids.go:7-9`). Every row pays for it
  (a 36-byte id and the rowid).
- `idx_run_memory_entries_kind` serves one reader: `ListRunMemoryEntriesForAgent`
  with `kind IN ('rlm_inbox_cursor')` (`run_memory.go:416-436`;
  `agentcore/rlm_reduce.go:582`), yet it indexes every row. It could be
  partial on that kind only if the query names the kind as a literal; with a
  bound `IN (?)` the planner cannot prove the partial predicate.
- `idx_run_memory_entries_loop_retire` is wide (it also carries `owner_id`
  and `agent_id`) so the retire group query is index only
  (`run_memory_retire.go:108-119`); its leading `loop_id` duplicates the
  `UNIQUE(loop_id, seq)` autoindex.
- Share by key width (estimate, not measured): with entry ids at 36 bytes the
  five trees are about 36 + 36 + 45 + 130 + 150 bytes an entry, so the parent
  index is about 9% (0.2 to 0.3 GB), the kind index about 33% (0.6 to
  0.8 GB), the loop_retire index about 38% (0.7 to 1.0 GB). The per-tree bytes
  are in the 05:30Z reading's tree list; read them there before using these.
- Building a replacement index reads the whole table including overflow
  chains, because `created_at` sits after `message_json`
  (`run_memory_retire.go:110-111`): an 11 GB read.

### 0.5 Ranking from the measurements (replaces section 3)

Order is GB per unit of risk. Every item that deletes rows or changes a
query plan on the guest is red by the project's classes (the store is a
projection that replay and the content witness read): two seats, a
conjecture delta, a rollback and a heresy delta. "Read first" names what the
lead can run before building.

### R-1. Make run-memory retirement catch every finished run (selection only; the op exists)

- Removes: rows of runs that rules 3 to 5 of 0.1 keep forever. Three changes
  to the selection, each independent: retire `passivated` and `blocked` runs
  whose last entry and last update are older than the window and which are
  not the newest `completed` or `passivated` run of their agent (the one the
  Texture reader takes, `run_memory.go:329`) (not built); protect only the
  document custodian's newest completed or passivated runs, the only desk that
  inherits a prior run (`run_memory.go:136-146`) (built, R-1); retire runs
  whose record is not found after a longer hold by entry age alone (built, R-1:
  the lookup was the defect, and the hold is 30 days).
- GB now: about 6 to 12 GB. Lower bound: 13.7 GB (table floor plus indexes)
  minus the 7-day steady state of about 7.6 GB, if the hour's rate holds.
  Upper bound: the 29.9 GB left for the two cut tables, less the 12.4 GB floor
  of `og_bodies`, leaves at most about 17.5 GB for the table, 20 GB with
  indexes, minus 7.6 GB. Which of the three changes holds the GB is unknown
  until the read.
- GB a day stopped: this makes run memory follow its window, so it stops the
  part of the 2 GiB a day that is surplus over the window. Run memory is
  about 374,000 rows a day, about 1.1 GB a day at 2.9 KB a row with indexes
  (inferred from one hour). If retirement removes nothing today, the whole
  1.1 GB a day is surplus; the two readings 24 h apart tell how much is.
- Class: red (a retire policy), two seats.
- Could go wrong: a Texture wake finds no carried conversation and starts
  from its prompt: the model's context changes and the provider's warm prompt
  cache is lost for that wake; `get_run_memory_entry` answers not found for a
  retired entry (`agentcore/tools_evidence.go:57`). Retiring a passivated run
  that is later re-warmed breaks resume: re-warm reads its entries
  (`texture_lifecycle_api.go:243-247`), so keep the newest completed or
  passivated run per agent whatever its age.
- Proof: a dry-run count of the new selection on a restored copy, by rule;
  then on staging the log line `run memory retirement: N runs in M ops` and
  the `run_memory_entries` tree's `rows` falling across two readings;
  a test that renders a Texture continuation from a store before and after
  and compares bytes.
- Rollback: revert the selection. The deleted rows are on the tape and return
  by replay or by restoring a snapshot taken before the op.
- Read first (needed): the run-state, keep-rule and age breakdown on a
  restored copy (0.1); the two `rowid_min` readings; the guest log lines for
  retirement since the last boot. None of these is a served counter.

### R-2. Shorten the run-memory window from 7 days to 3

- Removes: rows of finished runs between 3 and 7 days old.
- GB now: about 4.3 GB (7.6 GB at 7 days minus 3.3 GB at 3 days of
  arrivals, same inference). GB a day stopped: none (it lowers the level,
  not the rate).
- Class: red (policy constant, `run_memory_retire.go:46`); an owner decision,
  since the 7 days came from the lean-system step (`telemetry_retire.go:68`).
- Could go wrong: a person reading an older run's trace gets not found; a
  Texture wake after 3 days of the document being quiet loses nothing, as
  the newest run is kept by R-1.
- Proof, rollback: as R-1. Do it after R-1, because it only helps once the
  job reaches every run.
- Read first: none beyond R-1's.

### R-3. Drop or narrow the `run_memory_entries` indexes

- Removes: `idx_run_memory_entries_parent` (no reader), then the kind index
  made partial by naming the kind as a literal in the one query, then the
  `owner_id` and `agent_id` columns out of the retire index.
- GB now: about 1.1 to 1.6 GB of the 2.51 GB by key-width estimate (0.2 to
  0.3, 0.6 to 0.8 and about 0.3). GB a day stopped: about 0.1 GB (0.25 to
  0.3 KB an entry at 374,000 entries a day).
- Class: orange for dropping the unreferenced parent index (it changes no
  plan, because no statement uses it); red for the other two (query plans on a
  hot read and on the retire selection).
- Could go wrong: a statement I did not find (a dynamic query, a tool that
  builds SQL) uses the parent index; the kind query loses its index and scans
  an 11 GB table. The plan test for `runMemoryAgentListQuery` is the guard
  (`run_memory.go:412-415`); extend it.
- Proof: `EXPLAIN QUERY PLAN` of every statement in 0.4 on a copy; the tree
  list of a later reading; CPU and read bytes for a day.
- Rollback: the older binary recreates the dropped index at open,
  synchronously; scan time for 4.6 M or more rows is not measured.
- Read first: the three per-index byte figures, already in the 05:30Z tree
  list; no new read needed.

### R-4. Drop the redundant full indexes of `og_objects` (indexes 4 and 5)

- Removes: two of the five full-width indexes, on the premise of 0.3 that the
  guest holds one owner and computer.
- GB now: about 0.8 to 1.6 GB by estimate (0.4 to 0.8 each). GB a day: 130 to
  260 bytes an object written; objects a day is not measured.
- Class: red (plans on the hot path, a schema change), two seats.
- Could go wrong: the planner prefers a full scan for the snapshot read or
  the page read (the 2026-10-14 incident). The retire and event paths use
  `INDEXED BY` pins and are not touched.
- Proof: `EXPLAIN QUERY PLAN` of each reader in the table of 0.3 on a copy of
  the paper's store; drop time on d@e; CPU after.
- Rollback: an older binary rebuilds both at open (minutes, unmeasured).
- Read first: `SELECT DISTINCT owner_id, computer_id` count (restored copy),
  and the four cut indexes' bytes through `store_tables_only` as in 0.3.

### R-5. Retire more event kinds, and any object kind that is write-once and unread

- Removes: `choir.event` rows of kinds no group names (`model.served`,
  `search.served`, `loop.delta`, lifecycle events of finished runs) after a
  window, each only after its reader is named; for other kinds, a new
  retire class, which `ObjectRetireKindEvent` forbids today
  (`projection_batch.go:206-209`) and needs its own design.
- GB now: not estimable. The readings do not split `og_bodies` (at least
  12.4 GB) or `og_objects` (at least 13.2 GB with indexes) by kind. Whatever
  they hold of the kinds named is the ceiling.
- Class: red (`telemetryRetireGroups` is a projection rule), two seats.
- Could go wrong: a reader of the kind exists that the search missed.
  `model.served` is read by publication provenance
  (`store-once-2026-11-14.md`, R3), so it is not on the list without a
  design.
- Proof, rollback: as R-1, plus per-kind counts before and after. Rows return
  by replay from the tape.
- Read first (needed, not served): row count and body bytes by `object_kind`
  and, for events, `event_kind`. A yellow addition to `internal/storetables`
  (a covering walk of `idx_og_objects_kind_id` for counts; body bytes need
  `og_bodies` joined by `obj_id`) or a query on a restored copy. Build this
  before R-5.

### R-6. Carry run memory by reference (store-once R1)

- Removes: the copy of the previous run's whole tail into every new
  Texture run (`run_memory.go:165-217`).
- GB now: none directly; it lowers the steady state that R-1 and R-2
  leave. GB a day: at most the run-memory share, about 1.1 GB; the carried
  share of it is not measured (store-once R0 names the counter, carried
  versus new rows).
- Class: red, the largest change here; the conversation the model sees must
  be byte-equal, proved by rendering both ways.
- Read first: the carried-versus-new run-memory counter. Do not build before
  it.

### R-7. Return freed pages sooner

- Removes: nothing; it returns pages to the filesystem.
- GB now: none; the freelist is near 0. After R-1 and R-2 free 6 to 12 GB,
  new inserts reuse them (the store grows about 2 GiB a day, so 3 to 6 days to
  refill), and `reclaimFreePages` returns at most about 1 GiB a day
  (`compact.go:105`). Raise the cap in proportion to the freelist only if
  the file's size, not its content, is what matters (snapshot size follows
  the file).
- Class: orange, one seat; proof: `header.freelist_count` and `file_bytes`
  before and after a daily pass; rollback: revert, the file stays larger.

### Order and what to read first

1. Read, no build: two `store_tables_only=run_memory_entries` readings 24 h
   apart (`rowid_min`, `rows`, `freelist_count`); the four cut `og_objects`
   index trees; the guest's retirement log lines.
2. Restored-copy counts: run state and keep-rule breakdown of the rows (R-1);
   distinct owner and computer (R-4).
3. Build the per-kind counter (R-5's read, yellow).
4. R-1, then R-2 and R-3 as separate changes, then R-7 if file size matters.
   R-4 and R-5 when their reads are in. R-6 after the carried-share counter.

Sections 1 to 4 below are the earlier note, kept as history. Their
estimates are superseded where 0 says so.

## Read on the paper, 2026-11-14T06:08Z (console log, after the 02:37Z boot)

The daily job ran to its end on that boot: telemetry retirement 18 ops in
6 m (02:48Z to 02:54Z), then `store: run memory retirement: 4 runs in 1 ops`
(36 s), then `store: incremental vacuum: 26262 pages returned`. So the job
is not being cut short and no step errored. It found four eligible runs in
a table of at least 4.6 million rows. The selection rule, not the schedule,
is what keeps run memory: R-1 stands, and what remains unknown is which of
its three exclusions (passivated or blocked runs, the newest two runs of
each agent, runs whose record is not found) holds the rows. The next step
is a census taken by the pass itself, at no extra read: for each exclusion,
the runs, rows and bytes it kept, logged and served with the store counters.

## 1. What the 44 GiB is made of (ranked) (superseded 2026-11-14 by section 0; kept as history)

Superseded: the estimates in the table below were made before the readings.
Rank 3 and rank 4 in particular are wrong or unsupported (see 0).

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
| `run_memory_entries` + `idx_run_memory_entries_parent`, `_kind`, `_loop_retire` (`schema.go:94-357`) | one message of a run's conversation: `message_json` (2.2 KB average), role, kind, details_json | 2.45 KB [M] | `ProjectionOpRunMemoryEntry` | `ProjectionOpRunMemoryRetire` (`run_memory_retire.go:216`): whole runs, terminal and last written over 7 days ago, not a document custodian's newest 2 completed or passivated runs, 5,000 runs a pass and up to 32 passes a day (`run_memory_retire.go`) |
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

## 3. Candidate bounds, by bytes saved per unit of risk (superseded 2026-11-14 by 0.5; kept as history)

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
