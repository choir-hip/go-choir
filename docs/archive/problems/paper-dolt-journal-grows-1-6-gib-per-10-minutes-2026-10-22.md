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

From [`paper-unreachable-minutes-after-every-boot-2026-10-22`](../../problems/paper-unreachable-minutes-after-every-boot-2026-10-22.md),
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

## Root cause, second investigation (2026-10-22)

Date: 2026-10-22. Station: FP1 (lane `lane/fp1-journal-root-cause`). Mutation
class of this section: green. No runtime code changed. Nothing was run on any
computer, no SQL against any production store. One local probe test was run
and deleted (section 2).

Evidence labels as above. Checkout: `85a8115b1`. Paper readings **[M]** are FP0's
two observability snapshots of computer-1355b83b, 21:30:27 and 21:38:45 UTC
(498 s apart, build `674460d15`): `telemetry_batching`, `engine_mutex`,
`recent_log`, `guest_checkpoint`. All rates below are differences of those two.

### Answer in brief

1. The journal is written at about **1.8 MiB/s** (model; FP0 saw 1.3 to
   2.0) because the tape projects **14.0 ops/s in 10.8 tape events/s**
   **[M]**, and every op costs a full root-to-leaf path in every tree of its
   table: **about 160 KiB per `choir.event` tape event and about 90 KiB per
   run-memory tape event** on a 100,000-row store (probe, section 2). The
   cost is **per op, not per byte**: a 300-byte `desk.cell` row costs 159 KiB,
   an 8 KB progress row 169 KiB.
2. There is no unexplained writer. The writers are the run machinery itself.
   Of the 14.0 ops/s, **6.0/s are `choir.event` trace lines**, 3.7/s are
   run-memory entries, and 4.0/s are other objects, mostly in about 250 multi-op
   commits per 498 s. The product's real output (Texture revisions) is
   about 0.06/s.
3. A large part of the trace lines and of the run-memory entries is
   **redundant or has no durable reader**: a second `loop.progress` per model
   call that nothing reads (0.9/s), a Texture copy of every progress line that
   only the live bus reads (0.6/s), `tool.invoked` (0.84/s, live ticker only),
   `desk.cell` and `desk.egress` (1.6/s, diagnostic tooling only), an
   `inbox_cursor` run-memory entry written after every cell even when the
   cursor did not move, and a duplicate index on `run_memory_entries`.
4. Source cuts (R1 to R5) remove about **33%** of the journal rate (about
   600 KiB/s). Moving the trace kinds out of the tape and out of Dolt (R7)
   removes about **53%** on its own. Everything together (R1 to R7) removes
   about **68%** and leaves about 0.57 MiB/s: one 1 GiB GC pass per 30
   minutes instead of per 10 to 15.
5. Yes, this is a re-architecture question: ephemeral, loss-tolerant trace
   lines (owner ruling: "a crash may lose the open window", section 27 item 18
   of the owner notes) should not be rows in a versioned store behind a signed
   tape with four host files per event. Section 4, step R7.

### 1. The 14 ops/s, writer by writer

Window arithmetic **[M]**: 5,388 tape events (10.82/s), 6,979 ops (14.01/s).
Standalone `Store.AppendEvent` calls: 2,998 (6.02/s), each one tape event with
one op (batching off, `batching.enabled=false`). Run-memory ops: 1,823
(3.66/s). The remaining 567 tape events (1.14/s) carry 2,158 ops (4.33/s),
of which 1,996 are `object_recorded` (4.01/s), 88 edges, 74 Texture mutations.
Ops-per-event histogram of the window: 315 events with 1 op, 252 with 2 or
more (36 with 9 to 16, 17 with 17 to 32, 6 with 33 to 64, 1 with 65 or more).
So the non-event objects are about 250 batches per 498 s averaging 7 ops,
with a tail of 24 batches of 17 or more ops. `putBatch` engine-mutex calls:
440 (0.88/s) **[M]**.

| Stream | Rate | Writer (code) | Necessary at this rate? |
|---|---|---|---|
| `loop.progress`, phase `provider_call_started` | about 0.9/s | `toolloop.go:493-516`: emitted before every model call, with `system_preview` (2,000 chars), `last_user_text` (4,000 chars), two SHA-256s | **No.** No reader of the phase in Go, frontend, scripts or cmd **[V]** (grep of `provider_call_started`, `provider_call`). The cost reader takes only phase `tool_loop` (`llmcost/aggregator.go:72-83`) |
| `loop.progress`, phase `tool_loop` | about 0.9/s | `toolloop.go:660-678`, one per model call, token counts and 2,000 chars of response text | Yes for cost (`llmcost`). One per model call is the minimum |
| `texture.agent_revision.progress` | 0.59/s | `runtime.go:3549-3562`: for a Texture run, every `loop.progress` is copied as `{doc_id, loop_id, phase}` | **No durable reader.** Its only consumer is the live stream (`textureowner/texture.go:1705-1790` reads a bus channel) **[V]** |
| `tool.invoked` | 0.84/s | `batch_executor.go:73` | **No durable reader** in Go. The frontend ticker reads it live (`PromptSurface.svelte:139,159`). `cmd/choir/trace.go` lists every kind, so it is in trace bundles |
| `tool.result` | 0.83/s | `batch_executor.go:119` | Yes. It is evidence for run acceptance and the Texture workflow verifier (`run_acceptance.go:284`, `texture_workflow_verifier.go:345,378,397`) |
| `desk.cell` | 0.84/s | `tools_desk.go:644`, one timing row per cell (no source) | Diagnostic only: `cmd/choir/trace.go:363`, `scripts/management_latency_probe.py:295` |
| `desk.egress` | 0.80/s | `tools_desk.go:521,535,1215`, one row per host `choir.*` call | Diagnostic only: `cmd/choir/trace.go:364`. About 1 per cell |
| other `choir.event` (started, submitted, completed, revision started/completed/created) | 0.33/s | `emitEvent`, `runtime.go:4703` | Yes: run state and Texture canonical events |
| run-memory `message` entries | about 1.7/s | `run_memory.go:286-296` via `afterAppendMessage`: assistant turn and tool-result turn of each model call | Yes: the run's context is rebuilt from these each call (`run_memory.go:298-304`) |
| run-memory `inbox_cursor` | up to 0.84/s | `rlm_reduce.go:536-546`, called by `commitTray` at `:1040` after every successful cell | **Mostly redundant [H].** It is written even when `highWater == scope.Cursor`, and the reader takes the maximum (`rlm_reduce.go:516-531`), so an equal value changes nothing **[V]**. How often the cursor moves is not measured |
| run-memory `cell_fate` | up to 0.84/s | `rlm_reduce.go:574-607`, `recordFate` at `:858` | Needed so a late timeout cannot override a success (first fate wins). It does not need a row in the versioned store **[H]** |
| other objects, 4.0/s | in about 250 commits | Cell commits (`CommitInboxRecords`, `inbox.go:441-760`: records, inbox head, conversation head, wake outbox row, cursor, and the `inbox_command` replay row), Texture revisions with their citation checks, lifecycle transitions with folded events (`lifecycle.go:3331`), wake outbox state changes | **Unknown by kind [H].** The tape records the kind only in each op's `canonical_id`, and no counter reads it. Section 5, reading 1 settles it |

The mix in the log agrees: in the last 6 minutes the paper authored 31 Texture
turns, 23 of them revisions and 7 `no_semantic_change`, and published 3 stories;
20 revisions were held for citing fewer than 2 outlets. That is one revision per 16 s
and one published story per 2 minutes on top of 14 ops/s.

No heartbeat-like or periodic writer was found **[V]**: grep for heartbeat,
lease renewal and touch-run writers in `internal/agentcore` and `internal/store`
returns nothing. The tool-loop log rate (258 lines per 6 minutes, 0.8/s)
matches `tool.invoked` (0.84/s) and half of `loop.progress` (1.8/s), so the
progress rate is two lines per model call, not a loop.

Redundancy inside one cell (a `desk_go_eval` call), counted from the code:
the cell source is stored in `tool.invoked.arguments`, in the assistant
run-memory message, and up to 2,000 chars in `loop.progress.response_text`.
The cell output is stored in `tool.result.output` and in the tool-result
run-memory message. One cell costs about 8 to 10 tape ops: 2 `loop.progress`,
`tool.invoked`, `tool.result`, `desk.cell`, about 1 `desk.egress`, 2 messages,
`inbox_cursor`, `cell_fate`, and sometimes a records commit.

### 2. What it costs on the paper's store

Local probe, run and deleted (the lane's machine, not the paper): the real
tape harness (`telemetry_harness_test.go`), a store seeded to **100,000 rows
in `og_objects` (random ids, all 13 trees) and 100,000 in
`run_memory_entries`**, then 150 appends per kind through `Store.AppendEvent`
and `Store.AppendRunMemoryEntry`, journal bytes read with `doltJournalBytes`
around a `DOLT_COMMIT`. Index counts from `information_schema.statistics` on
the same schema: `og_objects` 13 trees (primary plus 12), `run_memory_entries`
**6 trees, two of them identical (`UNIQUE(loop_id, seq)` is `loop_id`, and
`idx_run_memory_entries_loop_seq` repeats it)**, `computer_event_index` 4.

| Tape event (one op each) | Journal per event |
|---|---|
| `choir.event`, 0.3 KB payload (a `desk.cell` row) | **159 KiB** |
| `choir.event`, 8 KB payload (the `provider_call_started` line) | **169 KiB** |
| run-memory entry, details only (an `inbox_cursor` row) | **90 KiB** |
| run-memory entry, 3 KB message | **96 KiB** |

Payload size is a 6% term (8 KB costs 10 KiB more). The path is the cost.
The record's 141 KiB for `loop.delta` was on 3,000 rows; 100,000 rows add a
level and about 18 KiB. The paper has 1.0M `og_objects` rows and a
deeper tree still, so these are floors **[H]**.

Model of the paper, per second, with those constants (`og` op about 113 KiB,
run-memory op about 45, tape-event overhead about 47) **[H]**:

| Group | Rate | Cost | KiB/s |
|---|---|---|---|
| trace lines (standalone `choir.event`) | 6.02 events/s | 160 | 963 |
| other objects, edges, Texture (567 events, 2,158 ops per 498 s) | 4.33 ops/s in 1.14 events/s | 113 per op + 47 per event | 507 |
| run-memory entries | 3.66 events/s | 93 | 340 |
| **Total** | | | **about 1,810 KiB/s = 1.77 MiB/s = 6.2 GiB/h** |

FP0's 1.3 to 2.0 MiB/s is bracketed. One GiB arrives in 9 to 13 minutes at
the model rate, which is the GC cadence seen (a pass every 10 to 15 minutes).
There is no residual that needs another writer. The split by group is
trace 53%, other objects 28%, run memory 19%. The per-op constants on the
paper's own store are not measured (reading 2).

### 3. Why it is a design problem and not one bad writer

- Every durable write, however small and however disposable, takes the same
  path: one tape event (four host files, three guest transactions) and one
  full prolly path per tree. The cost model has no term for payload size or
  for importance. A timing row and a Texture revision cost the same journal.
- 6.0 of the 14.0 ops/s are trace lines the owner has already ruled
  loss-tolerant. They sit in `og_objects` with 13 trees (most of them index
  nothing a trace reader asks for [H]), in the content witness, in every
  100-event checkpoint, and on the host as four files each (24 files/s).
- Permanent growth follows tape events, not journal bytes. The live store is
  28 to 29 GiB for 2.68M events: about **10 KB of retained history per tape
  event** **[H]**, derived from the live figure in the first section and today's head
  (2,677,384). At 10.8 events/s that is about 110 KiB/s, **about 9.5 GiB a
  day** if today's rate held. GC does not touch it. Cutting tape events is
  the only lever on it, so the step that removes 6 of 10.8 events/s matters
  more for the disk than for the journal.
- Telemetry batching (F-a) was the wrong lever for this: it removes the
  per-event 47 KiB, not the per-op path, which is what trace lines pay. That
  is why staging measured 4 to 10%.

### 4. Resolution, ranked by saving per unit of risk

Base rate used (KiB/s, section 2): trace lines 963, other objects 507,
run-memory 340, total about 1,810. "Saved" is derived from the paper's
rates above and the section 2 per-op constants **[H]**; the real constants
come from reading 2.

| # | Step | Saved | Class | Protected surface | Rollback | Build |
|---|---|---|---|---|---|---|
| R1 | Stop emitting the `provider_call_started` progress line (`toolloop.go:493-516`) and stop copying progress to `texture.agent_revision.progress` durably (publish on the bus only, `runtime.go:3549-3562`) | about 240 KiB/s (13%) | orange | none read it durably **[V]**; touches the trace layer | revert | hours; test: a stub provider run emits one `loop.progress` per model call; `cost` totals unchanged |
| R2 | Skip the `inbox_cursor` run-memory entry when `highWater <= scope.Cursor` (`rlm_reduce.go:1040`) | up to 75 KiB/s (4%) | orange | run memory (red-adjacent); the reader takes a max, so no read changes **[V]** | revert | hours; fraction unmeasured, reading 1 |
| R3 | Merge `tool.invoked` into `tool.result` (arguments ride the result; `batch_executor.go:73,119`), or make it live-only | about 135 KiB/s (7%) | orange, trace content changes | Trace/evidence: `tool.result` stays, `tool.invoked` disappears from bundles | revert | a day; frontend ticker reads the live bus, keep publishing there |
| R4 | Fold a cell's `desk.egress` rows into its `desk.cell` row (one event per cell with an `egress[]` array; `tools_desk.go:521,535,644`) | about 130 KiB/s (7%) | orange | Trace bundles and `management_latency_probe.py` must read the new shape | revert | a day |
| R5 | Drop `idx_run_memory_entries_loop_seq`, an exact duplicate of `UNIQUE(loop_id, seq)` (`store.go:407,485`) | about 30 KiB/s (2%) | orange, schema | duplicate confirmed locally. An index is not state in the witness extraction (`projectionbase/witness.go:19-38` compares table content) **[H]** | recreate the index | run as an explicit migration, never at boot (`boot-alters-og-objects-when-older-columns-missing-2026-10-21`) |
| R6 | Multi-row upserts in `projectBatch` for the 7-op average batch (F-e) | about 155 KiB/s (9%) | red | tape projection, replay, witness | flag per computer | days; local probe: 100 to 63 KiB per row |
| R7 | **Trace lines leave the tape and the store.** The sealer already classifies the ten kinds (`event_durability.go:35-46`) and queues them; its sink becomes an append-only segment log in the workspace (outside the Dolt directory, rotated hourly, group fsync, retention by age). List APIs read the log first and `og_objects` second for history written before the cutover | the whole trace share, 963 KiB/s (53%), of which R1, R3, R4 are 500, so R7 adds about 460 after them. Also 24 host files/s, and the retained-history growth of 6 of 10.8 events/s | red | the tape, Trace/evidence (`tool.result` is run-acceptance evidence), replay, content witness, every `ListEvents*` reader | a per-computer setting; off writes to the tape as today, and reads see both | 1 to 2 weeks plus a panel; design first |

Totals, derived **[H]**: R1 to R5 about 605 KiB/s (33%). R1 to R6 about
760 KiB/s (42%). R1 to R7 about 1,220 KiB/s (68%), leaving about
590 KiB/s (0.57 MiB/s, 2.0 GiB/h): one 1 GiB GC pass per 30 minutes
instead of per 10 to 15, and the stall 3x rarer. R1 to R5 are small,
independent, orange, and each can be proven on a staging test computer by the
existing counters (`events_appended_by_kind`, `tape_ops_by_kind`, section 5):
do them first, in one slice. R7 is the answer to "re-architect": do the
design in parallel, not after.

If the owner will not leave the Dolt store for trace lines, the next best is
F-c plus telemetry batching: a `(run_id, seq)` table with a sequential key and
one index costs about 20 KiB per op, plus 47 KiB per tape event divided by k.
At k = 5 that is about 30 KiB per event instead of 160, saving about
780 KiB/s (43%) **[H]**. It keeps the tape, the host files and the witness
cost, needs a new op kind and a replay change, and is red as well.
It is worse than R7 on every axis except staying inside one store.

Not first: dropping `og_objects` indexes (about 2% per tree; which exist on
the paper is unread, reading 3) and moving the transient coordination rows
(`inbox_command`, wake outbox) until reading 1 sizes them.

A GC that does not hold the write connection saves no bytes. It removes the
503s and is worth doing, but the stall shrinks by itself as the rate falls:
pass time is about 16.6 s per GiB, and after R1 to R7 passes are 3x rarer.

### 5. Readings FP0 can take

Each is read-only. FP0 does the host and paper reads.

1. **Tape sample (settles the 4.0 objects/s, the cursor and the fate rows).**
   On the host, take the newest 3,000 events of computer-1355b83b from the
   platform's append receipts and read each event's payload artifact
   (`sha256/computer-event-payload/<digest>`). Decode `ops[]`. Tally by
   `op.kind`; for objects, by the kind in `canonical_id`
   (`obj:<kind>:<owner>:<suffix>`); count ops and sum `len(body)` per group.
   For run-memory ops tally the body's `kind` (message, inbox_cursor,
   cell_fate, compaction). For `inbox_cursor` ops, count how many carry the
   same `cursor` as the previous one for the same agent and channel.
   Also list the batches of 17 or more ops: their kinds.
2. **Real bytes per op.** Two samples 600 s apart inside one GC interval (after
   a pass ends, before the next tick): `GET /api/runtime/observability?memory=detail`
   (`store` tree `bytes`, `truncated` empty, plus `telemetry_batching`) and
   `/health` `dolt_gc`. Then bytes per op is the journal delta over the
   `tape_ops_by_kind` delta, and with reading 1's mix it solves the og-row,
   run-memory and per-event constants.
3. **Tree count of `og_objects`.** The boot console line
   `objectgraph dolt: derived access paths ... (live: [...])` or
   `... missing on a table with rows ... (live: [...])`
   (`dolt_store.go:343,346`). It says which generated columns and indexes
   exist on the paper. The engine-mutex labels show `index` for
   `choir.event $.run_id`, `$.channel_id`, and `json` for
   `choir.channel_message $.to_agent_id`, so at least some exist and some do not.
4. **After R1 to R5 on a test computer:** the same `events_appended_by_kind`
   and `tape_ops_by_kind` over a 498 s window of a comparable load. The
   acceptance is the rate of each cut kind going to 0 (or halving for R3,
   R4) with `tool.result` and `loop.progress` phase `tool_loop` unchanged.

### 6. Not settled

- The kinds in the 4.0 objects/s. Candidates from the code are cell commits
  (inbox records, wake outbox, `inbox_command`), Texture revisions with
  citation checks, and lifecycle commits. No counter reads the kind. A
  one-line counter in `recordTapeEvent` (`telemetry_stats.go:114`) that
  tallies `canonical_id` kinds and body bytes would make reading 1 permanent;
  it is orange, small, and not built here.
- The paper's real per-op constants (reading 2). The model reproduces the
  measured rate (1.6 MiB/s against 1.3 to 2.0 MiB/s) but cannot tell the
  og-row constant apart from the per-event constant.
- How often a cell's cursor does not move (R2) and what share of
  `cell_fate` rows could leave the store.
- Whether anything reads `tool.invoked` or `desk.*` outside this repo's
  tools (owner dashboards, panels). The grep covers the repo only.
- The 10 KB per retained event is a division of two totals, not a
  measurement of what a checkpoint pins.

### Heresy delta (second investigation)

- discovered: trace lines are the largest single journal writer (6.0 of
  14.0 ops/s) and are paid at full tape price; a second per-call progress
  line, a durable copy of progress that only the bus reads, and a cursor
  entry per cell are written with no durable reader or no change; a duplicate
  index on `run_memory_entries`.
- introduced: none.
- repaired: none.

## Build: journal cuts R1 to R4 (2026-10-22, lane `lane/fp1-journal-cuts`)

Date: 2026-10-22. Builder for FP0. Based on `73718ec65`. Mutation class: R1,
R3 and R4 are **red** (they change what the Trace records); R2 is orange; this
section is green.

Owner, 2026-10-22: "we can't continue with the paper writing a gigabyte every
10 to 20 minutes... resolve it." And: "Anything which improves resource
utilization on a forward and ongoing basis should be prioritized over
stability, availability, or uptime in the short term." Owner ruling that trace
lines are loss-tolerant: notes section 27 item 18
(`docs/reports/owner-notes-2026-10-14.md`, "a crash may lose up to the batch
window of telemetry"), carried in `internal/types/event_durability.go` and
`docs/reports/telemetry-batching-design-2026-10-20.md`. The cuts below lean on
it: a crash loses at most what a crash already loses, and a bus-only line is
lost the same way.

Not built: R5 (store schema change, its own lane), R6, R7.

### The grep, kind by kind

Searched the whole checkout: Go (`internal`, `cmd`), `frontend/` (`*.svelte`,
`*.ts`, `*.js`, Playwright specs, helpers), `scripts/`, `skills/`, `nix/`,
`load/`, and `docs/` for a reader contract. Evidence JSON under
`docs/evidence/` and `docs/archive/` are dated captures, not readers.

**Correction to the record's R1.** The second investigation found no reader of
`provider_call_started` "in Go, frontend, scripts or cmd". There is one in
`scripts/`: `management_latency_probe.py` `model_calls` pairs the request
`loop.progress` (the one with `message_roles`/`messages` and no `input_tokens`)
with the next response `loop.progress` of the same iteration to get per-call
latency. The record's grep was for the phase string; the probe tests for the
payload field. So R1 does not simply delete the durable line: the reader moves
to the replacement (below).

| Kind | Durable readers found | Live readers | Decision |
|---|---|---|---|
| `loop.progress` phase `provider_call_started` | `scripts/management_latency_probe.py:216-235` (latency pairing). `docs/archive/reports/reeng-cut0-measurements-2026-10-12.md` section 2 defines "a model turn" by the pairing (a dated method, not code). `docs/problems/prompt-cache-breaks-across-turns-2026-10-14.md:234` used `system_preview` once to verify a fix. Go: none (`llmcost/aggregator.go:72-83` takes phase `tool_loop` only; `toolregistry/toolloop_test.go:242-281` tests the emit, not a stored row) | none in the frontend; Texture's `synth_progress` stream sees its `texture.agent_revision.progress` copy on the bus | **Bus-only.** The response line (`tool_loop`) gains `provider_call_ms`, `messages`, `tools`, `system_sha256`, `last_user_sha256`. The probe reads `provider_call_ms` and keeps the old pairing for old tapes |
| `texture.agent_revision.progress` | none. `agentcore/internal_run_event_api_test.go:111` reads it from the store (test, moves to the bus). `docs/archive/design/choir-event-table-f-c-2026-10-22.md:231` names it in a kind list | `textureowner/texture.go:1698,1790` (`Bus.SubscribeWithBuffer`, `textureStreamEventFromRecord` maps it to `synth_progress` / `synth_completed`); `agentcore/live_ws.go:110` (bus) | **Bus-only.** One cost: the live WebSocket catch-up (`live_ws.go:80`, `ListEventsByOwnerAfter`) can no longer replay a progress tick after a reconnect. Ticks are advisory; the revision events that matter stay acknowledged |
| `tool.invoked` | `scripts/reeng_tracer_checks.mjs:85` (`goTestCells` takes the cell source from the invoked row, matches it to the result by `call_id`); `scripts/retrieval_eval_extract.py:75` (cell sources); `scripts/management_latency_probe.py:297` and `scripts/l2a_window_pairs.py:158` (regex over the text of invoked and result rows); `scripts/reeng_tracer.mjs:442` (`cellInFlight`: invoked minus results = a cell is running, for the crash drill); `cmd/choir/trace.go` (lists every run event, no per-kind logic); tests (`toolloopvalidation_test.go`, `toolloop_test.go`, `batch_executor_test.go`, `reeng_*_test.mjs`) | `frontend/src/lib/PromptSurface.svelte:139,159` (the ticker, "agent called tool"), fed by `live_ws.go` from the bus. Same WebSocket catch-up caveat as above | **Merged into `tool.result`.** The result carries `arguments`, `started_at`, `duration_ms`. `tool.invoked` is still published on the bus for the ticker and is no longer appended. Readers move to the result with a fallback to the old invoked row. `cellInFlight` has no durable signal left and reports "not served" (its existing 8 s fallback) |
| `desk.egress` | `cmd/choir/trace.go:364` (the bundle's `egress` layer), which `scripts/reeng_invariants.mjs` (I5, `deskItems`) consumes; `scripts/reeng_tracer.mjs:1025-1026` (search start/done counts from raw events); `scripts/reeng_ttl_session.mjs:212`; `docs/reports/reeng-trace-bundle-format-2026-10-12.md` (the contract: "one item per `desk.egress` run event"); `docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md` (a refusal is a `desk.egress` event with `refused: true`) | none (desk events were never published on the bus: `recordDeskEvent`) | **Folded** into the `egress[]` array of the cell's `desk.cell` event: the synchronous per-call rows, same field names. Stays standalone: `refused` rows (SEC-02 evidence, the fp2 contract), the async `search_done` row (it outlives the cell), and any row that arrives after its cell event closed. The `egress` layer of the trace bundle is built from both shapes, so the bundle contract and I5 are unchanged |
| `desk.cell` | `cmd/choir/trace.go:363`, `scripts/management_latency_probe.py:295`, `reeng_invariants.mjs`, `reeng_ttl_session.mjs`, `reeng_tracer.mjs` | none | **Kept**, gains `egress[]`. Readers of the old fields are unaffected |
| `inbox_cursor` run-memory entry (R2) | `LoadInboxCursorForDesk` (`rlm_reduce.go:516`, takes the maximum), `recoverPartialActCommit` (`:888`, reads the same maximum), `LoadInboxCursor` (runID keyed, tests only) | n/a | **Skipped when the cursor did not move** (`highWater <= scope.Cursor`). The reader takes a maximum, so an equal value changes nothing; recovery compares `cursor < highWater`, which a skipped write never affects |

Replay and restore: every old tape still holds `tool.invoked`,
`desk.egress`, standalone progress lines. No reader is changed to require the
new shape. Readers accept both. No stored event is edited.

What the Trace loses, plainly:

- The prompt text of each model call (`system_preview` 2,000 chars,
  `last_user_text` 4,000 chars), `message_roles`, `tool_names`, `max_tokens`,
  `tool_choice`. The run-memory messages still hold what the model was sent;
  the hashes on the response line show whether the prefix changed.
- A tool call that started and never produced a result (a crash mid-tool) no
  longer leaves a `tool.invoked` row. The assistant run-memory message that
  holds the call is written before the tool runs and stays.
- The live ticker and the Texture progress stream lose their WebSocket
  catch-up replay for these three bus-only lines after a reconnect.

### Plan and evidence

Order: this section (green); tests that fail on `73718ec65` (yellow); one fix
commit per cut: R1 (red), R2 (orange), R3 (red), R4 (red); a closing record
update. Each red commit carries conjecture delta, protected surfaces,
admissible evidence, rollback (revert that commit) and heresy delta.

Admissible evidence for each red cut, taken by FP0 on a test computer under
comparable load over a window of about 500 s, as in reading 4 above:
`events_appended_by_kind` and `tape_ops_by_kind` per second, before and after.
Expected: `loop.progress` from about 1.8/s to about 0.9/s;
`texture.agent_revision.progress` to 0; `tool.invoked` to 0; `desk.egress` to
the refusals and `search_done` rows only; `tool.result`, `desk.cell` and
`loop.progress` phase `tool_loop` unchanged in count; `llmcost` totals
unchanged. The tests in the lane assert the same counts on a canned run
against the real store counters.

### Build result (lane `lane/fp1-journal-cuts`)

What each cut became, in commit order after the plan and the tests:

| Cut | Commit | Class | What it is now |
|---|---|---|---|
| R1 | `9804f0d57` | red | `types.LiveOnly` names `loop.progress` under emit phase `provider_call` and `texture.agent_revision.progress`. `runEventEmitter` publishes them on the bus and does not append. The tool loop still emits both. The response line (phase `tool_loop`) gains `provider_call_ms`, `messages`, `tools`, `system_sha256`, `last_user_sha256`. The latency probe reads `provider_call_ms` and keeps the pairing for old tapes |
| R2 | `920c57c5f` | orange | `commitTray` writes the `inbox_cursor` run-memory entry only when `highWater > scope.Cursor` |
| R3 | `39da1adec` | red | `tool.invoked` is in `LiveOnly`. `tool.result` carries `arguments`, `started_at`, `duration_ms`. Readers (tracer checks, retrieval extractor, the in-flight gauge) take the source from the result, with the old invoked row as fallback |
| R4 | `36187fa3b` | red | a cell's synchronous egress rows are written in its `desk.cell` event (`egress[]`, same fields). Refusals, `search_done` and a row that arrives after its cell event stay standalone. The bundle's `egress` layer reads both shapes |

Local counters (the real store counters, a canned run of 2 model calls and 1
tool call, `TestRunAppendsOnlyWhatReadersReadDurably`):

| `events_appended_by_kind` | before (`55bf2abf2`) | after |
|---|---|---|
| `loop.progress` | 4 | 2 |
| `tool.invoked` | 1 | 0 |
| `tool.result` | 1 | 1 |

A Texture run additionally stopped appending one `texture.agent_revision.progress`
per progress line (`TestTextureProgressIsPublishedNotAppended`). A desk cell
with three `choir.SearchStart` calls appends one `desk.cell` plus three
`search_done` events instead of one plus six (`TestDeskCellAndEgressRecordRunEvents`).

Saved, from the record's section 4 constants and not measured: R1 about
240 KiB/s, R2 up to 75, R3 about 135, R4 about 130: **about 580 KiB/s, 32% of
the 1,810 KiB/s model**. The share of R2 depends on how often the cursor does
not move, which is still unmeasured. The measured version is reading 4 above,
taken by FP0 on a test computer.

Tests run: `go test ./internal/types ./internal/toolregistry ./internal/llmcost
./cmd/choir`; `go test ./internal/agentcore -run 'Event|Trace|Live|Acceptance|ToolLoop|Progress|Cursor|Desk|Reduc|Texture'`;
`go test ./internal/textureowner -run 'Workflow|Progress|Stream|Verifier|Texture'`;
`node --test scripts/reeng_invariants_test.mjs scripts/reeng_tracer_*_test.mjs`;
`python3 -I scripts/journal_cuts_readers_test.py`. The frontend is unchanged
(the ticker reads the bus), so no frontend test was run.

Declined or left:

- R5 (duplicate index): its own lane, a store schema change.
- The prompt text of each model call (`system_preview`, `last_user_text`) is
  no longer durable. The hashes on the response line replace "did the prefix
  change". Putting the preview back on the response line would bring back most
  of the bytes, so it was not.
- The WebSocket catch-up cannot replay the three bus-only lines after a
  reconnect. A catch-up would need a bounded ring on the bus; not built.
- `docs/archive/reports/reeng-cut0-measurements-2026-10-12.md` still defines a model
  turn by the two-line pairing. It is a dated measurement; not edited.
- No frontend or Playwright reader of any cut kind exists besides the ticker
  (`tool.invoked`, bus), so nothing there moved.
- Not run: anything on a test computer or staging. The before/after rates per
  kind (reading 4) are FP0's to take.

Heresy delta (build): discovered: the record's R1 reader grep missed a script
that tests a payload field instead of the phase string; `tool.invoked` had five
script readers. Introduced: the crash-mid-tool and crash-mid-cell evidence
losses and the lost reconnect replay named under each cut, all inside the
owner's loss-tolerant ruling (notes section 27 item 18). Repaired: three
disposable lines, one redundant cursor entry and one row per cell no longer pay
a tape event each.

## Review round 1 of the journal cuts (2026-10-22, tip `27c6de2d0`)

Three seats reviewed the delta (DeepSeek v4.1 flash, Gemini 3.8, Devin;
Devin's transcript is incomplete). R1, R2 and R4 were accepted. Findings,
before any fix:

**F-1 (required, R3): malformed tool-call arguments erase the only durable
tool record.** `executeOneTool` (`internal/toolregistry/batch_executor.go`)
now puts the call's `arguments` into the `tool.result` payload as a
`json.RawMessage`. `canonicalJSON` passes invalid JSON through unchanged
(`internal/provider/provider.go`), so a provider that emits malformed
arguments reaches `executeOneTool` with bytes that are not JSON. Then
`json.Marshal` of the result map fails, the error is ignored
(`resultPayload, _ :=`), the payload is nil, and `AppendEvent` stores `{}`
for an empty payload. The `tool.result` row loses `tool`, `call_id`,
`is_error` and `output`. Before R3 the result never embedded the arguments,
so its row survived; and `tool.invoked` is now bus-only, so the attempt
leaves no durable trace at all. Run acceptance reads `tool.result` as
evidence. Verified empirically by two seats (Devin's last step, DeepSeek's
run): marshaling a map holding an invalid RawMessage fails.

Non-blocking notes, recorded:

- F-2 (R4): the unit of crash loss for egress rows is now the open cell, not
  the batching window. Rows wait in `cellEgressLog` until the `desk.cell`
  event is written. The bound is 300 s for management and unbounded for
  research and Texture (`deskCellHostDeadline` returns 0). Owner ruling
  (notes section 27 item 18) is "a crash may lose up to the batch window of
  telemetry"; a cell-length loss is wider than that wording. Mitigations:
  the cell event is written under `context.WithoutCancel`, and every
  non-crash exit (eval error, worker death, timeout) still writes it with
  the rows; a crash now loses `search_start` and `search_done` together
  instead of leaving a `search_done` without its start.
- F-3 (R1/R3): bus-only lines carry no `Seq`, so the live-WS catch-up after a
  reconnect cannot replay them. Already declined above.
- F-4: `scripts/reeng_tracer.mjs` `cellInFlight` returns `served:false` on new
  tapes (no `tool.invoked`) and falls back to `running >= 8s`, recording
  `observed_by: run_state_running_8s`. No acceptance depends on it.
- F-5: `serializeRunEventLedgerForCheckpoint` serializes every kind except
  `run.delta`, so the model-visible continuation checkpoint loses the
  truncated `system_preview`/`last_user_text` and the invoked row and gains
  `arguments` on results. Durable and model-visible, not structural.
- F-6: `scripts/reeng_invariants_plant.mjs` `plantI5` mutates
  `desk.egress` events in `runEvents` only; on a bundle whose egress is
  folded into `desk.cell` it mutates the egress layer but not the folded
  rows. Test scaffold only.
- F-7 (Devin): `scripts/l2a_window_pairs.py` and
  `scripts/management_latency_probe.py` (line 307) filter on
  `tool.result` and `tool.invoked` and scan payload text for receipts.
- Nits: a stale "one item per ... run event" comment in
  `scripts/reeng_invariants.mjs`; the trace bundle carries the egress bytes
  twice (`cells` embeds `egress[]`, plus the `egress` layer).

### Review round 1: what each finding became

- **F-1 fixed** (`internal/toolregistry/batch_executor.go`). Valid arguments
  are embedded as `arguments` as before. Invalid bytes ride as the string
  `arguments_raw` with `arguments_invalid: true`, on both the bus-only
  `tool.invoked` and the durable `tool.result`. A marshal failure of the
  result still falls back to a payload with `tool`, `call_id`, `is_error`,
  `output`, `output_len`, timing and `marshal_error`, and the error is no
  longer ignored. `TestToolResultSurvivesMalformedArguments` fails on
  `27c6de2d0` (empty payload) and passes. The readers of `arguments`
  (`retrieval_eval_extract.py`, `reeng_tracer_checks.mjs`) only look at a dict
  `arguments`, so a malformed call yields no source there, as before.
- **F-2 recorded, not changed.** The cell-length crash window for egress rows
  is wider than the batching-window wording of notes section 27 item 18. It
  sits inside the owner's loss tolerance for telemetry (crash restarts close
  the cell's work anyway), and the fold removes the orphaned `search_done`.
  If the owner wants the window bounded, the hardening is to seal and flush
  the log at N rows or T seconds into a standalone `desk.egress` part; that is
  an owner call, not a design branch, so it is left as a named residual.
- **F-3 declined** (above): a replay ring on the bus is not built.
- **F-4 recorded.** `cellInFlight` degrades to the 8 s running fallback on new
  tapes; no acceptance depends on it.
- **F-5 recorded.** The model-visible continuation checkpoint changes content
  on new tapes (no invoked row, no prompt previews, `arguments` on results).
- **F-6 fixed** (`scripts/reeng_invariants_plant.mjs`): `plantI5` also crosses
  the response id in the rows folded into a `desk.cell` event. The stale
  comment in `scripts/reeng_invariants.mjs` is corrected.
- **F-7 checked, no change needed.** Both scripts accept `tool.invoked` and
  `tool.result` and scan the payload text for receipt and story ids. The
  ids they look for appear in the call's output, and the arguments that used to
  ride `tool.invoked` now ride `tool.result`, so a new tape is scanned as
  fully as an old one. No test was added: they are live-computer probes that
  call the host API, which this lane does not run.
- The double carriage of egress bytes (`cells` embeds `egress[]`, plus the
  `egress` layer) is left; it is bundle size, not journal bytes.

Heresy delta (round 1): discovered: R3 introduced a silent loss path (an
ignored marshal error on a payload that embedded provider-controlled bytes).
Introduced: none. Repaired: that path; the I5 planter now covers folded rows.

Tests run: `go test ./internal/toolregistry`; `go test ./internal/agentcore -run
'Cut|ToolResult|DeskCell|Egress|ToolLoop'`; `node --test
scripts/reeng_invariants_test.mjs scripts/reeng_tracer_checks_test.mjs`;
`python3 -I scripts/journal_cuts_readers_test.py`.

### FP0 at landing (D7): a refused tool batch leaves no durable row

Bunch 138 (DeepSeek, on 65e471113, ACCEPT) noted, out of this delta's scope:
a refused batch (`internal/toolregistry/batch_executor.go:44-56`) returns
without emitting any `tool.result`. Before R1, its `tool.invoked` rows were
durable, so the refused attempt stayed on the tape; with `tool.invoked`
bus-only it leaves nothing durable. The gap is older than this lane (the
refusal never had a `tool.result`) but R1 widens it. Residual R-J1: emit a
durable `tool.result` with `is_error` and the refusal reason for a refused
batch, in a later lane; no acceptance or replay reader depends on refused
attempts today (bunch 138 checked `run_acceptance.go`). Also from bunch 138:
the F-6 planter change is inert (no test covers it; LOW), and
`arguments_raw` is not byte-exact for non-UTF-8 provider bytes (INFO).
