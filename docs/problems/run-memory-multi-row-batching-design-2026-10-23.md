# Run-memory rows as multi-row statements, with nothing any reader sees changed (design, 2026-10-23)

Date: 2026-10-23. Author: FP0 design worker, lane `fp0-run-memory-batching`
(design only; no product code). Status: proposed. Mutation class of this
record: green. Mutation class of the build it designs: **red** (tape
projection, the store's write path, new concurrency), review tier 2.

Parent: [store-journal-write-amplification-2026-10-23.md](store-journal-write-amplification-2026-10-23.md)
(the measurements this design builds on: 67 KiB of journal per run-memory
append direct, 103 KiB through the tape; a multi-row `INSERT` of one run's
rows is -52% at 4 rows, -68% at 16, -80% at 200; through the tape at 4 rows
an event about -59%; statements batched in one transaction save 3%).
Sibling: [run-memory-entries-hold-the-engine-2026-10-23.md](run-memory-entries-hold-the-engine-2026-10-23.md)
(the readers of the table and their cost). Claims are **[V]** (checked in the
code at the file:line given) or **[H]** (hypothesis).

## The answer in brief

Two halves, both with no delay of any row past the point where today's code
already waits, so no reader can see a state it cannot see today.

1. **The projector writes consecutive run-memory ops of one batch as one
   multi-row statement.** `projectBatch` runs each op as its own `INSERT`
   (`internal/store/project.go:63-67`, `:330-337`). A coalescer takes each
   maximal run of consecutive `run_memory_entry_recorded` ops, reads which of
   their ids exist in one `SELECT ... WHERE entry_id IN (...) FOR UPDATE`,
   checks existing rows for equality as today, and inserts the absent rows in
   op order in one `INSERT ... VALUES (...), (...)`. The tape format does not
   change: a batch of several run-memory ops is already valid and already on
   tapes (residue import writes them, `residue_import.go:428`). Older binaries
   replay it op by op into the same rows.
2. **The writer puts rows that already exist together into one tape event.**
   (a) A new `AppendRunMemoryEntries(ctx, []RunMemoryEntry)` for one run
   allocates the seqs and the parent chain once and appends one event; the
   tool loop and the run-memory manager call it at the points where several
   rows exist before the first is written. (b) A group commit inside
   `AppendRunMemoryEntry`: appends already wait in line on `runMemoryMu`
   (`run_memory.go:21-22`) for the whole tape round trip, so whoever takes the
   lock writes every unit queued behind it as one event. Each caller still
   returns only after its rows are finalized.

A time-delay buffer is rejected: eleven readers derive state from the run's
durable rows (section Q2). A buffer would need an overlay in each of them,
which would make a second state authority.

## Q1. Where rows of one run are produced together, and grouping with no delay

**What one tool-loop iteration writes.** **[V]** `internal/toolregistry/toolloop.go`:

| step | line | row | read of the run's memory before the next row? |
|---|---|---|---|
| assistant message with the tool calls | 733 | 1 message row | **tools run next** (738): side effects |
| tools run (`ExecuteToolBatch`, calls in parallel, `batch_executor.go:51`) | 738 | cursor row from a desk `commit` (`rlm_reduce.go:1062`), cell fate on abort (`:862`) | tools read cursor and fate rows |
| tool result message (all results of the batch in one message) | 743 | 1 message row | the injector runs next (749) |
| injected turns | 753 | 0..n `runtime_injection` rows | none |
| required-next-tool or initial-tool reminder | 764, 804, 821 | 0..1 row | none |
| `BeforeProviderCall`: compaction check and context rebuild | 449-455 | reads every row (`run_memory.go:299, 307`) | **this is the read** |

The boundaries that must stay are the two that sit before side effects or
before the run's own read. **[V]** for where they are; the reasons are below.

- **The assistant tool_use row stays durable before the tools run.** If it
  waited for the tool results, a planned restart during a tool (the paper
  takes marker-backed refreshes while busy, CLAUDE.md "Restarts") would resume
  a run that has no record of a call whose effects already landed. The
  resumed model would mint a new call id, and the cell identity
  (`cellIDForExecution`, `rlm_reduce.go:872-874`) keys on the call id, so the
  act could repeat. Rejected.
- **The tool result stays durable before the next provider call.** That call
  is the longest wait in the loop and the likeliest place for a planned stop.
  A read overlay could serve the read itself, but not durability.

**Groups that need no delay at all. [V]** (every row is computed before the
first append and nothing reads in between):

| site | rows | line |
|---|---|---|
| activation start | actor snapshot (compaction row) + every initial message | `agentcore/run_memory.go:93-100` (the snapshot is computed from *other* runs, `:136`) |
| end turn, ignored initial tool | assistant text + reminder | `toolloop.go:866, 869` |
| end turn, required tool not called | assistant text + reminder | `:890, 893` |
| end turn, final inbox checkpoint | assistant text + injected turns (the injector ran first, `:907`) | `:912, 915` |
| end turn, completion guard | assistant text + guard reminder | `:942, 945` |
| after tools | injected turns + the reminder that follows | `:753` with `:764` / `:804` / `:821` |

**One group crosses a call, and why it is still invisible to readers. [V]**
After the tools run, the tool result (743) and the injected turns (753) are
separated by `injectUserTurns`, and the parked path has the same shape
(989, 992, 999). Every injector that reads run memory reads only
`runtime_injection` rows. The co-agent injector does, through
`lifecycleInjectionIDsFromRunMemory` (`management_controller.go:2500`, filter
`:2362`), and so does the Texture frame injector (`texture_frame.go:58`,
filter `:36`). The Texture-owner injector does not read run memory
(`textureowner/coagent_injection.go`). So writing the tool result after the
injector instead of before it changes nothing any injector reads. Design:
the after-tools group is {tool result, injected turns, reminder}, written
once, before the next `BeforeProviderCall`. **On an injector error the tool
result is written alone before the error returns**, so the error path leaves
exactly today's rows.

The durability effect is named, not hidden. **[H]** A stop during the
injector (milliseconds, or as long as a stalled read of the write connection
lasts) now loses the tool result. The state that loss leaves (a tool_use row
with no result) is already reachable today, because a stop during the tools
leaves it, and the tools run far longer than the injector. No resume repair
for it exists today (no match in `toolregistry`/`agentcore` for a dangling
tool_use repair). The widening is the injector's duration added to a window
that is already open for the tools' duration. Recorded as open question O3,
with the strict alternative.

**How the groups reach the store.** `ToolLoopMemoryHooks`
(`toolloop.go:28-32`) gains an optional `AfterAppendMessages(ctx, []RoleMessage) error`.
At each site above the loop collects the rows and calls it once, falling back
to the per-message hook when it is nil. `runMemoryManager` implements it with
one `AppendRunMemoryEntries` call. It keeps the per-row rules of
`afterAppendMessage` (`agentcore/run_memory.go:227-252`): on failure of a
group that holds an injection row it returns `ErrRuntimeInjectionAppendFailed`,
and `appendCoagentUpdateIDsForRun` runs only after the group is durable.

**The rows the tools write** (the cursor row, `rlm_reduce.go:537`; the cell
fate, `:600`, `continuation_schedule.go:159`; the Texture author receipt,
`texture_cell_author.go:104`) are written alone, at the moment they are made.
They join a group only through the group commit (Q1b below), never by delay.
A version that holds the cursor row until the tool result is written is
named and deferred (O8), because it is a delay.

**The group commit (Q1b). [V]** for the queue it uses. `AppendRunMemoryEntry`
holds `runMemoryMu` across seq allocation and the whole tape append
(`run_memory.go:21-22`, `:52-92`), and the tape append takes the appender's
`a.mu` (`computerevent/appender.go:401-403`), which every projection event of
the computer shares. Concurrent run-memory writers (eight activation slots,
parallel tool calls) therefore already wait in line, for the appender behind
other events and for the write connection behind its holders: the online GC
holds it about 20 s a pass, and the cold cursor read held it up to 16.6 s.
Design:

- A caller adds its unit (one run's rows, `[]RunMemoryEntry`) to a FIFO
  under a short queue mutex. `entry_id` and `created_at` are stamped there,
  so `created_at` stays in queue order, which is seq order per run.
- Whoever holds `runMemoryMu` drains every queued unit (caps: 64 rows,
  1 MiB of bodies; a unit is never split; a second op with an `entry_id`
  already in the drain waits for the next drain) and writes them as one
  event. Units drained by an earlier holder are found done, and their callers
  return.
- With one writer at a time this is exactly today: one unit, one event, the
  same rows. **No caller waits for a timer, and no caller returns before its
  rows are finalized.** A caller's wait is no longer than today's, because
  the units ahead of it go as one event instead of several.

## Q2. If there were a delay: who would see the gap

None of the shapes above delays a row, so no reader sees a gap. Read-your-writes
holds by construction:

- the tape append is synchronous to finalize (`appender.go:438`, then
  `appendLocked` through Prepare, CAS and Finalize, `:752-769`);
- `AppendRunMemoryEntries` and the group commit return after it;
- the run's next read is `BeforeProviderCall` (`toolloop.go:449`), after the
  flush.

**[V]**

For the record, these readers would see the gap a time-delay buffer opens,
and they are why it is rejected:

| reader | line | what a delayed row breaks |
|---|---|---|
| seq allocation `MAX(seq)+1` and the parent lookup | `run_memory.go:54-73` | a duplicate `(loop_id, seq)` or a wrong parent unless the buffer joins the read |
| the run's context rebuild and compaction check | `agentcore/run_memory.go:299, 307` | the model call misses its own last turn |
| the injection seen-set (co-agent updates, owner revisions, emits) | `management_controller.go:2500, 2649, 2718`; `store/lifecycle.go:3845`; `tools_engineering_assignment.go:183` | **the same update is injected twice** |
| the Texture frame mark | `texture_frame.go:58` | the frame is re-injected |
| the desk inbox cursor | `rlm_reduce.go:497, 520` | messages are reduced again |
| the first cell fate wins | `rlm_reduce.go:552, 579` | a second fate wins |
| `LatestRunMemoryEntry` | `run_memory.go:456` (no production caller; a test, `run_memory_test.go:60`) | stale tail |
| Texture recovery tail | `textureowner/texture_controller.go:1455` | wrong tail id |
| evidence by entry id | `tools_evidence.go:57` | not found |
| actor rewarm of a later activation | `run_memory.go:167-229` | missing tail |
| replay, the witness, residue import, fingerprints | `computer_events.go:226-237`; `computerversion/dolt_state_extractor.go:388`; `residue_import.go:406`; `run_memory.go:385` | a buffered row is on no tape and in no checkpoint |

Keeping read-your-writes under a delay would mean an overlay in every one of
these. That is a second state authority (standing question: single state
authority). Rejected.

## Q3. Seq allocation and parent links inside a batch

**[V]** for today's rule, design for the batch.

- Today, under `runMemoryMu`, seq is `COALESCE(MAX(seq),0)+1` for the loop
  (`run_memory.go:54-59`), and the parent is the entry with the highest seq
  (`:61-73`). Both are read on `s.db` outside a transaction. They are
  consistent only because the previous append finalized before the lock was
  released.
- In a group (one unit or a drain of several), for each run in drain order:
  one read of `SELECT entry_id, seq ... WHERE loop_id = ? ORDER BY seq DESC LIMIT 1`.
  Under `UNIQUE(loop_id, seq)` (`store.go:521`) that row gives both the
  maximum seq and the parent. Rows then take `max+1 .. max+k` in unit order,
  and row i's parent is row i-1, with row 0's parent the durable tail. The
  result is byte-for-byte what k sequential appends in queue order would
  assign.
- An entry that arrives with `Seq > 0` or a `ParentEntryID` goes alone (k = 1).
  **[V]** No production caller sets either (`agentcore/run_memory.go:157, 287, 378, 460`;
  `rlm_reduce.go:537, 588`; `texture_cell_author.go:104`).
- The projector allocates nothing. Seq and parent are in the op body, and it
  inserts them as given (`project.go:330-337`). Coalescing cannot change a seq
  or a link. A `(loop_id, seq)` collision inside a statement fails the
  statement, and with it the event's transaction, exactly as the per-op path
  fails the batch at that op.

## Q4. Replay, tape compatibility, and what the witness sees

- **No new op kind, no new batch version, no projector-version bump.**
  **[V]** A V2 batch may hold any number of `run_memory_entry_recorded` ops
  (`projection_batch.go:173-207` checks kinds, not counts). The tape already
  holds such batches: residue import puts every imported run-memory row of
  many runs into one batch (`residue_import.go:415-430`). Live finalize, the
  dry run and replay all go through `projectOp` (`project.go:44-50, 63-67`;
  dry run `computer_events.go:121`; replay `:231`).
- **Older binaries replay a grouped tape.** **[V]** for the path: Validate
  passes; the per-op loop inserts row by row; inside the one transaction a
  later op's `SELECT ... FOR UPDATE` (`project.go:309-313`) finds no row for
  its own new id, so it inserts. Seqs within a run are distinct, so the
  unique key holds. Same rows. Nothing needs refusing, because nothing an old
  binary cannot read is written.
- **New binaries replay old tapes.** One op per batch, so the coalescer
  degenerates to the per-op path.
- **The witness.** **[V]** The table content hash is a hash of the rows from
  `SELECT *` (`dolt_state_extractor.go:388-418`), and the schema observation
  is `SHOW COLUMNS` (`:338-382`). Identical rows and no DDL give an identical
  witness, and `run_memory_entries` stays `ReplayEventProjection`
  (`agentcore/replay_eligibility.go:60`). **[H]** Dolt's own
  `dolt_hashof_table` is the same too, because prolly trees are
  history-independent: chunk boundaries come from key hashes
  (`newKeySplitter`, `node_splitter.go:60`, cited in the parent record), so the
  same row set gives the same tree however many statements wrote it. Test T3
  asserts it.
- **The event chain differs, as it should.** A grouped computer has fewer
  `projection_batch_recorded` events than an ungrouped one would have. Replay
  reconstructs whatever tape exists, and the rows of one event are one
  restore unit ("intermediate heads between ops in the same batch are not
  restore targets", `projection_batch.go:63-64`). **[V]**
- **Upcast, aliases, derived keys** do not touch run-memory ops
  (`upcastProjectionDeposit` is the vocabulary deposit; derived keys are
  `og_objects` only). **[H]**, to check in review.

## Q5. Failure partway

- **In the projector: all or nothing per event.** **[V]** One serializable
  transaction per finalize (`computer_events.go:143`, commit `:242`, rollback
  deferred `:148`). A failed multi-row statement fails the event, as a failed
  single op does today. The error names the op range; on a failure the
  coalescer may re-run the range op by op to name the exact op (O5).
- **In the writer: all or nothing per unit, and a unit is one run's group.**
  - *Refused before pin* (`ErrBatchRefused`, the dry run, `appender.go:427-435`,
    sentinel `:168-183`). Nothing is on the tape. A drain of several units is
    retried one unit at a time, so each caller gets its own verdict and one
    bad unit cannot fail its neighbours. A unit of one run's group fails
    whole, and its caller sees one error.
  - *Failed after pin* (CAS lost or finalize failed,
    `ErrNeedsProjectionRepair`, `appender.go:757-764`). The event may be
    canonical. **No retry**, because a retry would write the rows a second time
    under new seqs. Every caller in the event gets the error, exactly what
    each single append gets today, and repair projects the rows.
  - *A caller's context.* It is cancelled before its unit is drained: the unit
    leaves the queue, and no row is written. It is cancelled after the drain:
    the caller waits for the result (as `RecordCellFate` already detaches,
    `rlm_reduce.go:862`), so it is never told "failed" about rows that landed.
    The holder writes under its own context with `context.WithoutCancel`, so
    one caller's cancel cannot fail the others.
- **In the tool loop.** A group failure returns the error the first failing
  per-row append returns today (`toolloop.go:743-755` wrapping), after the
  injector-error rule in Q1. The in-memory `messages` slice is discarded with
  the error, as today.
- **Duplicate deterministic ids** (cell fate, `rlm_reduce.go:589`). They are
  never in one drain; across drains the per-op path's rule holds: an
  identical row is a no-op, a different one is `ErrProjectionMismatch`
  (`project.go:318-325`), and `RecordCellFate` already treats that as "first
  fate wins" (`:600-604`).

## Q6. Journal and GC effect, and the staging measurement

The parent record's model gives KiB per row = **F/k + D(k)**, with F = 35.8 KiB
per tape event and D(k) the statement cost per row at k rows to a statement
(measured, 100,000 rows, warm, local). **[V]** for the measured points, **[H]**
for the mixes.

| rows per event k, and their shape | KiB per row | vs today | of the journal (run memory is ~27%) |
|---|---|---|---|
| 1 (today) | 35.8 + 66.9 = 102.7 | - | - |
| 2, one run | 17.9 + 45.4 = 63.3 | -38% | -10% |
| 2, two runs (group commit) | 17.9 + 57.9 = 75.8 | -26% | -7% |
| 4, one run | 9.0 + 33.2 = 42.2 | -59% | -16% |
| 4, interleaved runs | 9.0 + 52.0 = 61.0 | -41% | -11% |
| 16, interleaved | 2.2 + 33.2 = 35.4 | -66% | -18% |
| 64 (a GC-pass backlog: 3.4 rows/s x 20 s) | 0.6 + 20.2 = 20.8 | -80% | n/a: stalls only |

**What the real k will be is not known, and it decides the result. [H]**

- Producer groups: the common desk iteration writes the assistant row alone
  and the tool result alone, unless an injection or a reminder follows. Only
  activation starts, end-turn paths and turns with injections give k > 1. If a
  fifth of rows ride in pairs, run memory falls about 8% and the journal 2%.
- Group commit: k > 1 whenever a second writer arrives during a run-memory
  append. The append costs three transactions, two host pins and the wait for
  `a.mu`. Steady-state service time on the paper is unmeasured. At 3.4
  arrivals a second and 50 ms per append, about one append in six would find
  company. During the 1 to 16 s holds of the write connection, and the 20 s
  GC passes, the whole backlog goes as one event.
- A plausible range is k̄ = 1.2 to 2, so run memory -10% to -30% and the
  journal **-3% to -8%**. That is below the record's -16% ceiling (four rows of
  one run in every event), which no producer site reaches in the common loop.

**GC. [V]** arithmetic of the record: the stall share is 18 s/GiB x the
journal rate, so the journal cut is the stall cut: 1.76% becomes 1.71% to
1.62%, and 3.07 passes an hour become 2.98 to 2.83. The pass is still about
20 s long. The batching also shortens the 20 s backlog itself: the rows queued
behind a pass go as one event when it ends, which is about 70 events fewer per
pass. Run-memory batching is worth building for the write model and for
residue import, which coalesces at once. It is not a GC fix. F-c deploy 2
remains the larger lever (record, ranking 1).

**Measurement before building the producer half.** The first deploy carries
the counters, so the build is sized on paper numbers:

1. **Rows per run-memory event.** A histogram of `k`, and drains with more
   than one unit, added beside `recordTapeEvent`
   (`store/telemetry_stats.go:155-172`, which already counts
   `tapeOpsByKind` and an all-kinds op histogram).
2. **The row mix per site.** A counter by (kind, role, call site): assistant
   tool_use, tool result, injected, reminder, cursor, fate, snapshot. It
   gives the fraction of rows the producer groups can reach.
3. **Queue wait and depth** at `runMemoryMu`: the group commit's k before it
   exists.
4. **Journal per row.** The record's reading 2: `journal_mib` growth per GC
   interval against `tape_ops_by_kind`, before and after, at least two GC
   intervals each.
5. **Local acceptance.** A `group` arm of `TestJournalAmplificationCensus`
   through the tape with the coalescing projector, k = 1, 2, 4, 16, one run and
   interleaved. Each cell must be within 4 KiB (the census noise) of the table
   above.

## Q7. Tests first, then the staged proof

Per CLAUDE.md "Testing", the ways it can fail come first; each gets a test
written and run red before the code.

| # | failure | test |
|---|---|---|
| T1 | a seq gap or a duplicate in a group | k rows of one run get `max+1..max+k`; two units of one run in one drain stay contiguous in queue order |
| T2 | a wrong parent chain | row i's parent is row i-1, and row 0's parent is the prior tail |
| T3 | grouped rows differ from sequential ones | the same entries (fixed ids and times) appended singly into store A and grouped into store B: equal `ListRunMemoryEntryFingerprints`, equal witness content hash, equal `dolt_hashof_table('run_memory_entries')` |
| T4 | replay differs | a grouped tape replayed by the coalescer and by the per-op path (the old binary's path) gives identical tables; an old one-op-per-event tape and a residue-import batch are identical under both |
| T5 | duplicate id inside one batch | equal: no-op; different: `ErrProjectionMismatch` naming the id, as per-op |
| T6 | re-finalize of an already projected batch | no-op; a mismatch still errors |
| T7 | one bad unit fails its neighbours | a drain with one unit refused at dry run: the others land, one at a time, and the bad one's caller alone errors; no row twice |
| T8 | a retry after pin duplicates rows | a finalize failure injected after the CAS: no retry, every caller gets `ErrNeedsProjectionRepair`, and repair yields exactly k rows |
| T9 | a cancelled caller is told the wrong thing | cancel before the drain: no row; cancel after: the caller waits and gets success |
| T10 | `created_at` out of seq order within a run | monotonic per run across drains |
| T11 | an oversized event | the 64-row and 1 MiB caps split between units, never inside one |
| T12 | the barrier exemption spreads | `TestBarrierExemptionHasExactlyTwoCallSites` (`telemetry_sealer_test.go:191-209`) unchanged: one call site in `run_memory.go` |
| T13 | the tool loop changes rows or their order | for each site in Q1, the rows, roles and order written equal today's; an injector error leaves the tool result written |
| T14 | the injection failure is lost | a group with an injection row that fails returns `ErrRuntimeInjectionAppendFailed`; no co-agent ids are recorded |
| T15 | a reader sees half a group | a hook between the drain and the finalize: a concurrent `ListRunMemoryEntries` sees none or all of it |

**Staged proof.** Each step runs on staging (`choir.news`). One setting
(`run_memory_group_commit`, default off) gates the writer half; the
projector's coalescing is gated by the same setting at store open.

1. **Deploy 1** (counters, and the coalescer off): read the k histogram, the
   row mix and the queue depth on a staging test computer and on the paper
   for a day. Decide from them whether the producer groups are worth
   building (O1).
2. **Deploy 2** (coalescer, `AppendRunMemoryEntries`, group commit; setting
   off). Run T1 to T15 in CI. On a staging test computer, not the paper, turn
   the setting on and drive a desk workload for an hour, then:
   - the replay-completeness probe gives an equal `run_memory_entries`
     content hash between live and a fresh replay;
   - `ListRunMemoryEntryFingerprints` is equal live and replayed;
   - the k histogram is above 1;
   - no new `ErrProjectionMismatch` or `ErrNeedsProjectionRepair`.
3. **The paper.** Turn the setting on. Over at least two GC intervals before
   and after, compare journal MiB/s, run-memory rows a second, k, GC passes an
   hour and pass length, and `/health` 503 samples. The artifact is the
   before-and-after table with the counter snapshots.
4. **Deploy 3**, the producer groups in the tool loop, only if deploy 1's row
   mix shows enough rows at those sites.

## Q8. Class, review tier, rollback

- **Class: red.** The tape projection (`project.go`), the store's run-memory
  write path, and new concurrency (the drain). A defect would be silent:
  rows written wrong replay wrong. **Tier 2**: Devin and DeepSeek v4.1 flash.
  The counters of deploy 1 are orange (tier 1).
- **Protected surfaces:** the tape projection and replay, the run-memory
  evidence rows, and `runMemoryMu` with the appender lock order. Not touched:
  the event format, the batch version, the projector version, the store
  schema and its epoch, `schemaDDL`, and indexes.
- **Rollback:** turn the setting off. Writers go back to one row per event,
  which the coalescer handles as the per-op path. That takes one act, and it
  shows within minutes on the k histogram. Full rollback: revert the commits.
  Every grouped event already on a tape replays on the old binary into the
  same rows (Q4). There is no schema, data or index change to undo.
- **Heresy delta:** *discovered*:
  - the common tool-loop iteration has no two rows that can share an event
    without moving a durability boundary;
  - the post-tools group crosses the injector, which is safe for readers
    only because every injector reads `runtime_injection` rows alone;
  - `LatestRunMemoryEntry` has no production caller;
  - multi-op run-memory batches are already on tapes, from residue import.

  *introduced*: none (design only). *repaired*: none.

## og_objects multi-row upserts (the record's option 3, R6)

**Same mechanism on the projector side; not needed on the writer side.**
`og_objects` ops already arrive several to an event: cell commits average
about 7 ops, and some carry 17 or more (record, **[M]**). Each is still its own
upsert (`project.go:243-259`). The coalescer generalizes as follows. A run of
consecutive `object_recorded` ops with distinct canonical ids and no
`choir_events` stamp (`project.go:79-81`) becomes one
`INSERT ... VALUES (...), (...) ON DUPLICATE KEY UPDATE`. The run splits at a
repeated id, at an edge op and at a stamped op, so last-writer-wins never
depends on how Dolt orders duplicates inside one statement **[H]**. The
differences that make it a separate deploy:

- **Derived keys.** `derived.Maintain` writes `og_key_index` per object, with
  a per-row error fallback (`objectgraph/derived_keys.go:311-339`). Kept per
  row, after the statement and in op order, it keeps its semantics. Those
  tree paths stay per row, so the saving is below the raw probe's -37%
  **[H]**. Making them one statement too loses the per-row fallback and needs
  its own design.
- **Per-op normalization.** `created := time.Now()` when zero
  (`project.go:237-238`) must stay per op.
- **Equivalence.** The same tests T3 to T6, over `og_objects`, `og_edges` and
  `og_key_index`, plus a replay with derived keys withdrawn (the replay path,
  `computer_events.go:230-234`).

Order: run memory first. It is one table with a pure insert and no upsert,
and it proves the coalescer and its replay equivalence. R6 then reuses it.

## Open questions

- **O1.** The row mix by site and the queue depth on the paper. They decide
  whether the producer half (deploy 3) is worth its code; deploy 1's
  counters settle it.
- **O2.** The run-memory append's service time on the paper. It sets k for
  the group commit, which may be about 1 in the steady state and large only
  during holds and GC passes.
- **O3.** The post-tools group widens by the injector's duration the window
  in which a planned stop loses the tool result. That state is already
  reachable today. Chosen: grouped, with the tool result written alone on an
  injector error. The strict alternative writes the tool result before the
  injector and groups only {injected, reminder}. A panel may prefer it if
  deploy 1 shows injections are rare, in which case the group gains nothing
  anyway.
- **O4.** `dolt_hashof_table` equality, grouped against sequential. T3
  settles it. Only the SQL-level witness is required to match, and it does
  by construction.
- **O5.** Whether Dolt rolls back a failed multi-row statement at the
  statement level inside a transaction. This matters only for naming the
  exact failing op on a re-run, because the event's transaction is rolled
  back either way.
- **O6.** The payload and pin limits per event. The 1 MiB cap is a guess, to
  check against the host pin path.
- **O7.** No projector-version bump. A bump would make old binaries refuse
  new tapes for no change in rows. Reviewers may disagree.
- **O8.** Holding the cursor row (written during a desk `commit`) until the
  tool result is written would put it in the post-tools group. That is a
  delay inside the tool. Its readers come in later cells, but parallel tool
  calls in one batch could read it. Deferred.

## Build size

About 500 lines of product code and 1,000 of tests, over three deploys:

| deploy | contents | code | tests |
|---|---|---|---|
| 1 | counters | about 60 | about 80 |
| 2 | the coalescer in `project.go` (about 120); `AppendRunMemoryEntries` and the group commit in `run_memory.go` (about 200); the setting | about 330 | T1 to T12, T15: about 650 |
| 3 | the tool-loop hook and its sites (about 70); the manager's group method (about 50) | about 120 | T13, T14: about 250 |

Add the census `group` arm, about 80 lines of yellow test. Estimate: two to
three worker-days and two tier-2 review rounds.

## Rollback of this record

Revert the commit that adds this file. There is no runtime change.
