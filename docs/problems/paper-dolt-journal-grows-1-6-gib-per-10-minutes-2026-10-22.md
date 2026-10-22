# The paper writes about 1.6 GiB of Dolt journal every 10 minutes (2026-10-22)

Date: 2026-10-22. Station: FP1 follow-up (lane `lane/fp1-journal-growth`).
Mutation class of this record: green. No runtime code changed, nothing run
on Node B or any computer, no SQL against any production store.

Evidence labels: **[V]** verified by reading the code at the cited
`file:line` on this checkout (`ab8df8fb6`) or the vendored Dolt module
(`github.com/dolthub/dolt/go v0.40.5-0.20260716231223-6bea93f2f665`);
**[M]** measured, in a cited record, not re-measured here; **[H]**
hypothesis or derivation, with the observation that would settle it.

## Evidence

From [`paper-unreachable-minutes-after-every-boot-2026-10-22`](paper-unreachable-minutes-after-every-boot-2026-10-22.md),
"A second, separate finding" **[M]**: the online GC on the paper
(vm-1669020a) ran at

| Pass | Journal at start | Duration |
|---|---|---|
| 03:02:35 | 1034 MiB | 19 s |
| 03:17:35 | 1772 MiB | 31 s |
| 03:27:35 | 1613 MiB | 28 s |
| 03:37:35 | 1634 MiB | 30 s |

Store: used 37 GiB, live (non-journal) 28 to 29 GiB. `/health` failed at
03:37:53, inside the 03:37:35 pass.

Derived **[H]**: the online GC ticks every 5 minutes
(`internal/autoputer/run.go:734`) and runs when the journal is at least
1 GiB (`internal/store/dolt_maintenance.go:25`, `:183`). The 03:22 and 03:32
ticks were no-ops, so the journal grows by 1.6 to 1.8 GiB in 10 to 15
minutes: **about 2.0 to 2.7 MiB/s**. GC time is close to linear in the
journal: about 16.6 s per GiB plus about 2 s.

## Is it a defect?

Two defects, one cause.

1. **Write amplification.** A few KB of new content per durable write
   produces hundreds of KB of journal (section "Mechanism"). The journal is
   garbage that only GC reclaims, and the part reachable from the
   100-event checkpoints becomes permanent history (the 28 to 29 GiB live).
2. **Every GC pass stops the store's writes for 20 to 30 s.** The online
   GC runs `CALL DOLT_GC()` on the store's only write connection
   (`dolt_maintenance.go:326-336`; `configureEmbeddedDoltDB` sets
   `MaxOpenConns(1)`, `internal/store/texture.go:352-355`) while holding
   `doltCommitMu` **[V]**. Every write behind it waits for the whole pass.
   Dolt also parks `addChunk` callers while a GC is in its keeper phase
   (`store/nbs/store.go:1030-1075`, `waitForGC`) **[V]**. `/health` that
   touches the store fails inside the pass.

## Mechanism: what one durable write costs in journal bytes

### Dolt facts the estimate rests on

- Every SQL statement that writes ends with the table writer's `Close`,
  which flushes the edited prolly maps (`libraries/doltcore/sqle/writer/prolly_table_writer.go:320-326`)
  **[V]**: new leaf chunks and every ancestor up to the root, for the
  primary index and for each secondary index whose key changed, plus new
  table and root-value chunks.
- The chunk store has **one store-wide memtable** (`store/nbs/store.go:1041-1062`,
  `nbs.memtable`) **[V]**. Chunks flushed by a statement are in it whether
  or not the transaction later commits; the next commit by any session
  persists the memtable. So a **rolled-back transaction still writes its
  chunks to the journal**. Measured locally (section "Local measurement"):
  a rolled-back insert costs the same journal bytes as a committed one,
  once the next commit lands. (But when a later commit writes the same
  row into the same tree, its chunks are the same and cost nothing again:
  see the correction.)
- The journal deduplicates by content: `ChunkJournal.Persist` skips chunks
  the store already has (`store/nbs/journal.go:293-321`) **[V]**. Rewriting
  an identical row costs nothing; any changed byte (an `updated_at`)
  rewrites the leaf and its path.
- `LONGTEXT`/`LONGBLOB` values are content-chunked blobs, so changing one
  field of a large JSON value rewrites the affected blob chunk(s) and the
  blob's path, not the whole value **[H]** (Dolt's blob chunker; not
  re-read here). Large-row rewrites are therefore not the main term.

### What one tape event writes on the guest

Almost every durable guest write is one `projection_batch_recorded` tape
event with one op (1.07 ops per event,
[`projection-event-volume-design-2026-10-19`](../reports/projection-event-volume-design-2026-10-19.md) §1)
**[M]**. Each such event runs **three** guest SQL transactions **[V]**:

| Transaction | Where | Trees touched |
|---|---|---|
| Dry run, rolled back | `DryRunProjectionBatch`, `internal/store/computer_events.go:96-117`, called from `internal/computerevent/appender.go:433` | the op's projection: for an `og_objects` row, the primary key and all 12 secondary indexes |
| Prepare | `Prepare`, `computer_events.go:33-70` (INSERT at `:63`) | `computer_event_index`: primary key (`event_digest`, a SHA-256, random) and 3 secondary (`store.go:446-476`) |
| Finalize | `finalizeBatch`, `computer_events.go:119-248` | `computer_event_projection_heads` (1 row), `computer_event_index` (row update and the status index), and the op's projection again |

`og_objects` carries **13 prolly trees** **[V]**: the primary key
(`canonical_id`; for `choir.event` a content hash, so random,
`internal/store/graph_store.go:1163-1166`) and 12 secondary indexes:
`kind_owner`, `updated` (`internal/objectgraph/dolt_store.go:35-36`),
`owner_computer_kind`, `kind_owner_computer` (`:147`, `:154`), the six
generated-column indexes (`:254-261`, built at `:469`), `rec_inbox_seq` and
`rec_claim` (`:266-273`, `:358-365`). A row of any kind is in every index;
the generated columns are NULL for other kinds, but the rows are still
indexed. `run_memory_entries` has a random UUID key and 5 secondary indexes
(`store.go:362-365` plus the UNIQUE) **[V]**.

### Local measurement (2026-10-22, this lane)

An embedded Dolt store from this module, on the lane's machine (a probe
test, run and deleted; not a proof about the paper). One table shaped like
`og_objects`: a random `CHAR(64)` key, 12 secondary indexes, 100,000 seed
rows, then 200 operations of each kind, journal size read with
`doltJournalBytes` before and after:

| Operation (one transaction each) | Journal per operation |
|---|---|
| Autocommit insert, random key | **87 KiB** |
| Insert, then ROLLBACK (bytes appear at the next commit) | **90 KiB** |
| Autocommit UPDATE of the timestamp column only | 63 KiB |

The same probe with 6 secondary indexes and 60,000 rows: 53, 56 and
60 KiB. So about 6.7 to 7.6 KiB per touched tree at this size; a table of
about 10^6 rows is one level deeper, about 8 to 9 KiB per tree **[H]**.

Estimate per `choir.event` tape event on the paper **[H]**, from those
constants:

| Part | Trees | Journal |
|---|---|---|
| Dry run (`og_objects` insert, rolled back) | 13 | about 90 to 120 KiB |
| Prepare (`computer_event_index` insert) | 4 | about 30 to 40 KiB |
| Finalize (heads, index row and status index, `og_objects` insert) | about 16 | about 115 to 150 KiB |
| **Total** | | **about 240 to 310 KiB** |

The dry-run row of this table is **wrong**; see the correction below.

Cross-check from an earlier measurement **[M]**: on 10-15 the GC stopped at
10-14 13:00 and the journal reached 27.2 GB at 10-15 00:20
([`guest-dolt-storage-growth-2026-10-15`](guest-dolt-storage-growth-2026-10-15.md)),
at about 3 tape events per second. That is 0.67 MB/s, or about **220 KB per
event**, and the volume design states "240 KB per event". The estimate
above is the same order and slightly larger, as it should be on today's
store: `og_objects` gained `rec_inbox_seq`, `rec_claim` and the derived-key
side table since 10-15, and the trees are deeper. The two agree.

At 220 to 310 KiB per event, 2.0 to 2.7 MiB/s means **about 7 to 12 tape
events per second** on the paper now, against about 3 on 10-15 and 1.0 to
1.5 per day-average since 10-17. The paper is busier than either:
after deploy C it ran research 8/8 and sources 4/4 slots with 67 due agents
(boot record). Which of the two (more events, or more bytes per event)
carries the growth is the main open question; FP0's first measurement
below splits it.

## Correction (2026-10-22, second pass): the dry run costs about 5%, not 35 to 40%

The estimate above counted the dry run's chunks as extra bytes. They are
mostly not extra. A Dolt prolly tree's shape depends only on its contents,
not on the order of writes, and the journal stores a chunk once
(`journal.go:293-321`). The dry run inserts the same `og_objects` row into
the same committed tree that finalize inserts it into a moment later, under
the same appender mutex. So finalize produces the same chunks and writes
none of them again. Only the dry run's table and root chunks, and any value
that differs between the two passes (a `time.Now()` default for a missing
timestamp, `project.go:107`, `:230`, `:272`, `:598`), are new.

Measured locally on the real path (the store's tape harness,
`internal/store/telemetry_harness_test.go`: a real `Store`, the real
appender, recording pin and CAS, 3,000 seed events, then 150 `loop.delta`
events per sample through `Store.AppendEvent`; probe tests run and deleted):

| Sample | Journal per event |
|---|---|
| Without the dry run (switched off for the probe) | 133 KiB |
| With the dry run | 141 KiB |
| Without the dry run, again | 136 KiB |

The same holds on the raw-table probe: prepare plus finalize 119 KiB per
event, with a rolled-back dry run of the same row first 126 KiB. **The dry run
is about 5 to 8 KiB per event, about 4 to 6%.** The 90 KiB of a rolled-back
insert measured earlier is real only when nothing later writes the same row.
F-b is withdrawn: it would be a red change to tape validation for about 5%.

What the remaining bytes are, measured on the same harness with telemetry
batching on and one batch of k events forced by `DrainTelemetry`:

| Ops per tape event (k) | Journal per event |
|---|---|
| 1 (batching off) | 141 KiB |
| 3 | 105 KiB |
| 6 | 97 KiB |
| 12 | 93 KiB |
| 64 | 91 KiB |

Batching removes the per-event prepare and finalize overhead (about
45 KiB). The floor of about 90 KiB is the `og_objects` insert itself. That
insert stays per op even inside one transaction, because each op is its own
statement, and each statement flushes its own path in all 13 trees
(`prolly_table_writer.go:320-326`). The raw-table probe confirms this: 6
rows as 6 statements in one transaction cost 100 KiB per row; the same 6
rows as one multi-row INSERT cost 63 KiB per row.

### The ranking after the correction

1. **The `og_objects` row: 13 trees, one statement per op** (about 90 of the
   141 KiB). Levers: F-c (`choir.event` out of `og_objects`, fewer trees,
   sequential key), and a new smaller one, F-e: in `projectBatch`, write
   consecutive unguarded `og_objects` upserts of a batch as one multi-row
   statement (about -37% per row at k = 6 on the raw probe; only useful with
   batching on).
2. **Prepare and finalize per event** (about 45 KiB). Lever: F-a, telemetry
   batching, built and off: -31% per event at k = 6, measured.
3. The dry run: about 5%. No change.

Recommendation, revised: F-a on a test computer first (built; measured
-25% to -35% for telemetry events locally), then F-e (small, red, inside
`projectBatch`), then F-c as the substrate design.

## Staging measurement of F-a (FP0, 2026-10-22 05:50)

Telemetry batching's phase 1 staging gate ran on test computer
computer-71afbf14 (evidence
docs/evidence/telemetry-batching-phase1-staging-2026-10-22.json):

- Batches form: k = 5.16 (211 batches, 1,089 ops). Tape events fall to
  0.60x.
- Journal bytes fall much less than the local -25 to -35%: 3.7 to 5.6% per
  tape op (5.6% is the same-build comparison), 1.3% per runtime event, about
  10% per telemetry record.
- A fit over the run gives about 86 KB of store growth per op and about
  12.5 KB per tape event. That agrees with the local 90 KiB per og_objects
  row (13 trees): on staging the journal is paid per op, by the row and its
  trees. Prepare and finalize per tape event cost little.

So F-a is not the journal lever. It also cannot go on the paper until the
host's planned restart drains the queue
(docs/problems/host-planned-restart-skips-telemetry-drain-2026-10-22.md).
F-e (multi-row upserts in `projectBatch`) is only useful with batching on.
The ranking becomes F-c first: move `choir.event` out of `og_objects` into a
table with a sequential key and the one or two indexes its readers use. It
needs its own design and a panel. Caveat: the gate's store was fresh and
small; the paper's per-op cost is not measured, and deeper trees cost more
per op, not less.

## Causes, ranked (first pass; see the correction above)

1. **Per-event transaction amplification (top).** Three transactions per
   durable write, each rewriting a root-to-leaf path in every touched
   tree, 13 of them for any `og_objects` row, and the dry run's chunks
   reach the journal although it rolls back. The journal rate is
   (tape events per second) x (about 240 to 310 KiB). **[V]** mechanism
   and the local constants, **[H]** the paper's constants.
2. **Event rate.** `choir.event` telemetry was 58% and run-memory entries
   31% of the paper's tape events on 10-15 **[M]**. Telemetry batching is
   built and off: the `telemetry.batch` computer setting
   (`internal/store/computer_setting.go:38-59`, sealer
   `internal/store/telemetry_sealer.go`) **[V]**. Whether it is on for the
   paper is not known here.
3. **Random keys.** `computer_event_index.event_digest`, `og_objects`
   content-hash ids and `run_memory_entries.entry_id` are random, so
   consecutive events never share a leaf. This matters little for the
   journal (one path per transaction either way) and a lot for history
   (each 100-event checkpoint pins about 100 distinct paths per table,
   15 to 31 KB per event, volume design §3) **[M]**. It explains the live
   28 to 29 GiB, not the 1.6 GiB.
4. **Not the cause, checked:**
   - Dolt commits. `DOLT_COMMIT` runs once per 100 finalized events
     (`store.go:101-104`, `computer_events.go:232-248`), on discard, on a
     replay boundary and before a Texture history read (`texture.go:1259`)
     **[V]**. A commit adds a commit chunk; the bytes come from the
     working-set transactions in between. `doltCommitMu` callers are only
     `commitDoltCheckpoint`, `markDoltHistoryDirty` and the online GC
     (`store.go:223`, `:244`; `dolt_maintenance.go:330`) **[V]**.
   - Large JSON rows rewritten on small updates: blob chunking bounds the
     cost of an unchanged-but-rewritten value to its changed chunks
     **[H]**; content-identical rewrites are free **[V]**.
   - Direct (non-tape) writers. Nearly every guest write goes through the
     tape projection (`internal/store/project.go`) or `computer_events.go`.
     The others (`desktop_live.go`, `media.go`, the derived-key `Maintain`
     and validator, `derived_keys.go:285-335`, `:800`) are per user action
     or per page of a validation pass **[V]**; the paper has no user at its
     desktop. The derived-key `Maintain` adds one `og_key_index` path to
     each keyed write (`project.go:254`) **[V]**, small.
   - Source polling and the outbox write through the same tape path, so
     they are part of the event rate in cause 2, not a separate writer.

## Proposed fixes, first pass (none implemented here; superseded by the correction's ranking)

Smallest first. Each needs FP0's measurement first: if bytes per event are
far above about 400 KiB, this model is wrong and none of these is the fix.

| # | Fix | Effect (derived) | Class | Rollback |
|---|---|---|---|---|
| F-a | Turn on telemetry batching for the paper (`telemetry.batch`, already built) | One dry run, prepare and finalize per sealed batch of up to 64 ops. `og_objects` inserts stay one per op but share the batch's upper-level chunks; the three-transaction overhead falls by k. Expect the `choir.event` share of the journal to fall 2x to 4x (volume design §4a) | red (tape write path; the design's own class) | setting off |
| F-b (withdrawn, see the correction) | Stop the dry run writing chunks: validate with a read-only check where the op cannot fail at the engine (unguarded `choir.event` and run-memory inserts), keep the full dry run for guarded ops | Removes about 35 to 40% of the journal for those ops (local measurement) | red (tape append validation; a projection that fails after the CAS wedges the computer) | revert |
| F-c | Move `choir.event` (append-only telemetry) out of `og_objects` into its own table with a sequential key and the 1 or 2 indexes its readers use | About 33 trees to about 10 per telemetry event (13 to about 2 in both dry run and finalize): the largest single lever, about two thirds of a telemetry event's journal | red (store schema, content witness, replay, every `choir.event` reader) | dual-read; a design of its own |
| F-d | Sequential key for `computer_event_index` (volume design b2) | History, not journal | red (schema, witness) | migration |

Recommendation: F-a first, measured on its own test computer (it is built,
and it acts on the dominant event kind); F-c is the substrate fix and
needs a design and a panel; F-b only if F-a's measurement shows the dry run
is a large share.

## Stopgaps for the stall

- **Smaller, more frequent GC passes.** GC time is about 16.6 s/GiB + 2 s,
  so a 256 MiB threshold gives about 6 s passes every 2 minutes at today's
  rate: the same total stall, but each one shorter than a health probe
  timeout. Not reachable by configuration today:
  `RUNTIME_DOLT_GC_JOURNAL_GIB` is whole GiB with a floor of 1
  (`dolt_maintenance.go:136-146`), and the tick is a fixed 5 minutes
  (`run.go:734`). It needs a MiB knob and a 1-minute tick (the check is a
  `stat`). Orange; rollback is the env default. **[H]** that the intercept
  stays about 2 s.
- **Raising the threshold** makes fewer, longer stalls. Worse for `/health`.
- **Non-blocking GC.** Run `DOLT_GC` on a session other than the single
  write connection, so writers keep going until Dolt's own safepoint and
  only the final swap parks them. **[H]**: needs a local probe first
  (write latency on a populated store during an online GC, both forms).
  Orange (store maintenance; no schema). Separately, the paper-boot
  record's F1 (bounded `/health` reads, fast 503) removes the health
  symptom whatever GC does.

## What FP0 can read without a guest shell

All are existing routes or logs **[V]**; none changes the computer.

1. **Events per second.** `GET /health` twice, 600 s apart:
   `effective_release.event_head.sequence` (`internal/agentcore/effective_release.go:59-72`).
   Δsequence / 600 is the paper's tape event rate.
2. **Journal growth between GCs.** `GET /api/runtime/observability?memory=detail`
   at the same two times: the `store` tree's `bytes`
   (`internal/agentcore/api.go:839-845`; check `truncated` is empty). Between two GC passes the old
   generation does not change, so Δbytes is the journal growth. Read both
   samples inside one GC interval (after a pass ends, before the next tick
   that runs).
3. **Bytes per event** = (2) / (1). Prediction, revised: the real path
   measured 141 KiB per `loop.delta` event on a small local store; the
   paper's deeper trees and larger rows put it at about 150 to 250 KiB,
   which makes 2.0 to 2.7 MiB/s about 8 to 18 events/s. Above about 400 KiB
   means a writer or a blob rewrite this record missed.
4. **Kind mix and batching state.** The same observability response,
   `telemetry_batching`: `enabled`, `events_appended_by_kind`,
   `tape_events`, `tape_ops`, `ops_per_tape_event`,
   `tape_events_by_authority`, `tape_ops_by_kind`
   (`internal/store/telemetry_stats.go:147-194`). Diff the two samples.
5. **Write paths by count.** `engine_mutex` (calls, hold) in the same
   response: diff by op name.
6. **Each GC pass.** `/health` `dolt_gc`: `journal_mib`,
   `after_journal_mib`, `avail_mib`, `after_avail_mib`, `duration_ms`
   (`dolt_maintenance.go:64-80`; only the last decision is kept, so read it
   after a pass). The console log lines `store: dolt gc (online) starting:`
   and `complete in ...: journal A MiB -> B MiB, avail C MiB -> D MiB`
   give every pass. The share of each journal retained as history is
   about `journal_mib - after_journal_mib - (after_avail_mib - avail_mib)`.

## Next observation

FP0 takes readings 1 to 6 on the paper (two samples, 600 s apart, inside
one GC interval). That splits "more events" from "more bytes per event"
and gives the kind mix, which decides between F-a and F-c.

## Heresy delta

- discovered: the online GC holds the only write connection for its whole
  pass; each projected op is its own statement, so batching cannot share
  the 13-tree path of an `og_objects` row. (The first pass also listed the
  dry run's journal cost as a defect; the correction measures it at about
  5%, so it is not one.)
- introduced: none.
- repaired: none.
