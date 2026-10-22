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
  chunks to the journal**. **[V]** for the shared memtable; **[H]** that no
  path discards a rolled-back session's chunks (none found).
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

Estimate per `choir.event` tape event **[H]** (depth about 4 for a table of
about 10^6 rows, about 3 KiB per compressed chunk, so about 12 KiB per
touched tree):

| Part | Trees | Journal |
|---|---|---|
| Dry run (`og_objects` insert) | 13 | about 156 KiB |
| Prepare (`computer_event_index` insert) | 4 | about 48 KiB |
| Finalize (heads, index update, `og_objects` insert) | about 16 | about 190 KiB |
| Table, root-value and working-set chunks, about 8 statements | | about 30 to 60 KiB |
| Content (event JSON, receipt, body) | | about 5 KiB |
| **Total** | | **about 250 to 450 KiB** |

Cross-check from an earlier measurement **[M]**: on 10-15 the GC stopped at
10-14 13:00 and the journal reached 27.2 GB at 10-15 00:20
([`guest-dolt-storage-growth-2026-10-15`](guest-dolt-storage-growth-2026-10-15.md)),
at about 3 tape events per second. That is 0.67 MB/s, or about **220 KB per
event**, and the volume design states "240 KB per event". The estimate
above is the same order and slightly larger, as it should be on today's
store: `og_objects` gained `rec_inbox_seq`, `rec_claim` and the derived-key
side table since 10-15, and the trees are deeper.

At 220 to 450 KiB per event, 2.0 to 2.7 MiB/s means **about 5 to 12 tape
events per second** on the paper now, against about 3 on 10-15 and 1.0 to
1.5 per day-average since 10-17. The paper is busier than either:
after deploy C it ran research 8/8 and sources 4/4 slots with 67 due agents
(boot record). Which of the two (more events, or more bytes per event)
carries the growth is the main open question; FP0's first measurement
below splits it.

## Causes, ranked

1. **Per-event transaction amplification (top).** Three transactions per
   durable write, each rewriting a root-to-leaf path in every touched
   tree, 13 of them for any `og_objects` row, and the dry run's chunks
   reach the journal although it rolls back. The journal rate is
   (tape events per second) x (about 250 to 450 KiB). **[V]** mechanism,
   **[H]** the constants.
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

## Proposed fixes for the top cause (none implemented here)

Smallest first. Each needs FP0's measurement first: if bytes per event are
far above 450 KiB, this model is wrong and none of these is the fix.

| # | Fix | Effect (derived) | Class | Rollback |
|---|---|---|---|---|
| F-a | Turn on telemetry batching for the paper (`telemetry.batch`, already built) | One dry run, prepare and finalize per sealed batch of up to 64 ops. `og_objects` inserts stay one per op but share the batch's upper-level chunks; the three-transaction overhead falls by k. Expect the `choir.event` share of the journal to fall 2x to 4x (volume design §4a) | red (tape write path; the design's own class) | setting off |
| F-b | Stop the dry run writing chunks: validate with a read-only check where the op cannot fail at the engine (unguarded `choir.event` and run-memory inserts), keep the full dry run for guarded ops | Removes about a third of the journal for those ops | red (tape append validation; a projection that fails after the CAS wedges the computer) | revert |
| F-c | Move `choir.event` (append-only telemetry) out of `og_objects` into its own table with a sequential key and the 1 or 2 indexes its readers use | About 13 trees to about 2 per telemetry event in both dry run and finalize: the largest single lever, roughly half the journal | red (store schema, content witness, replay, every `choir.event` reader) | dual-read; a design of its own |
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
3. **Bytes per event** = (2) / (1). Prediction: 220 to 450 KiB. Above about
   1 MiB means a writer or a blob rewrite this record missed.
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

- discovered: a rolled-back dry-run transaction per tape event writes its
  chunks to the journal (the shared memtable); the online GC holds the only
  write connection for its whole pass.
- introduced: none.
- repaired: none.
