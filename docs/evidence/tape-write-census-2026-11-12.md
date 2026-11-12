# Tape write census and group-commit sketch (paper, 2026-11-12)

Read-only investigation. Nothing was edited, committed or pushed; nothing was
written on Node B; the paper's guest was not called. Host reads were
`BEGIN READ ONLY` selects on the receipts table (sequence, kind, created_at,
`pg_column_size`, counts). No digests, signatures, bodies or payloads were
selected.

Tags: **[O]** observed, with source. **[I]** inferred, basis stated.
"The dump" is `paper-stall-dump.json` in this directory; its guest counters
start at the 2026-11-11 21:35Z deploy and end at the stall (about 15:28Z on
11-12, about 18 h) [I from the file's deployed_at and the stall record; the log
tail in the file reaches 17:42Z]. Host windows are UTC.

## 0. Findings that change the question

1. **Conjecture B is already built, and it is off.** `internal/store/tape_group_commit.go`
   (leader and followers, no timer, caps 256 ops / 1 MiB, telemetry rides the
   leader's event, refused groups split to singles, unknown outcome never
   retried) landed in `abfab8754` (2026-10-31) behind the computer setting
   `tape.group_commit`, default false (`computer_setting.go:233`, constant at
   `:183`). The paper's counters show it unused: `group_commit.enabled=false`,
   `groups=0` [O dump telemetry_batching.group_commit]. The design is
   `docs/design/tape-group-commit-2026-10-31.md`.
2. **The owner ruled the switch and its code deleted on 2026-11-03**: "i dont
   like the sound of losing events just before a crash" [O
   `docs/problems/default-off-switches-audit-2026-11-03.md:106`,
   `docs/evidence/switch-audit-2026-11-03.md:769`]. The code is still in the tree
   [O]. The objection is to the part that adds a crash-lossy window (telemetry
   window stretched to 10 s, and run-memory `message` entries joining it,
   `run_memory_batch.go`). The leader/follower coalescing of acknowledged
   writes has no such window: every member returns only after the shared event's
   CAS and local finalize (design F1, `tape_group_commit.go:169-214`). So B
   splits in two, and only the lossless half is needed (section 4).
3. **The 1 CAS per event premise is off: it is four sequential corpusd HTTP
   calls per event** plus three SQLite write transactions in the guest:
   pin payload (`appender.go:~571`), `cas.Head` GET (`:786`), pin event (`:815`),
   CAS (`:845`); locally `DryRunProjectionBatch` (a rolled-back write-pool
   transaction that projects the ops, `store/computer_events.go:164`, called
   from `projection_tape.go:200` before the appender mutex), `Prepare`
   (`computer_events.go:98`) and `finalizeBatch` (`:147`) [O code]. This is
   why one event costs about 142 ms and why the per-event cost, not the byte
   count, is what saturates [I].
4. **Events drive snapshots.** A snapshot is due when the tail since the newest
   reaches 540,000 events (`snapshot_producer.go:~88`, 3/4 of
   `MaxRecoveryTailEvents` 720,000, `recoveryplan/recovery_plan.go:22`). Host
   receipts show the paper's last 8 snapshots 1.1 to 1.4 days apart at
   about 550k events each [O receipts, event_kind snapshot_recorded]. The code
   comment says each writes about 25 GB of new chunks [O
   `snapshot_producer.go:~83`]. So the 53 GiB of snapshot chunks scales with
   events/day, not with bytes written [I]. Cutting events 4x cuts the snapshot
   rate to about one per 4 days (the weekly backstop is the floor).
5. **Run memory is the biggest writer and it doubled.** See section 2.

## 1. The write path

### 1.1 Chain from a store write to a tape event [O code]

```
caller (agentcore, textureowner, store method)
  -> Store.ogPut / PutBatch / AppendRunMemoryEntry / AppendEvent / PutX
  -> objectgraph.SQLStore.PutObject|putBatchChecked      sql_store.go:421,1002
       validate; (conditional batches: evaluateConditions on committed
       SQLite state, a read-only tx, sql_store.go:1022-1026)
       then s.intercept(...)  -- NO SQL write here; the SQL write happens later
  -> projectionTape.interceptOG  (projection_tape.go:85)   one op per object/edge
     other writers: Store.AppendProjectionOps (:98), texture.go:349,1340,
     desktop_live.go:73, media_projection.go:229, run_memory.go:264,
     event_table.go:695, selfdev/operations.go:742,757
  -> appendOps -> appendOpsAs (:158)
       group commit off (default): appendDirect (:171)
  -> appendDirect: marshal ProjectionBatch (V2), seal under the computer key,
       DryRunProjectionBatch (write-pool tx, rolled back)
  -> ComputerEventAppender.AppendNewPrivatePayload       appender.go:527
       awaitBarrier (telemetry flush)  ->  a.mu.Lock()   (one mutex, whole append)
       PreparePrivatePayload, PinPrivatePayload  (RPC 1)
       appendLocked: cas.Head (RPC 2), Reduce, PinEvent (RPC 3),
       projection.Prepare (SQLite insert), cas.CompareAndSwap (RPC 4; corpusd:
       per-computer lock, SERIALIZABLE tx, one receipt row INSERT),
       VerifyEventHeadReceipt, finalizeProjection -> store.FinalizeBatch
       (apply the ops to SQLite: this is the store write), a.mu.Unlock()
  -> the caller returns
```

**Unit of a batch today.** One call to `appendOps`, which is one tape event.
Callers pick the unit: `ogPut` is one object (one op); `PutBatch*` is a caller's
object set (lifecycle transitions are 3 to 10 ops); the telemetry sealer packs
queued records (mean 2.1 ops, `batching.mean_ops_per_batch`); run memory is
always one entry per event, because `AppendRunMemoryEntries` is only used by the
`tape.group_commit` path (`agentcore/run_memory.go:~454-474`, `batchedAppends()`
= `TapeGroupCommitOn()`) [O code]. It is not one SQL transaction: with the tape
bound, the SQL transaction is the projector's `finalizeBatch`, after the CAS.

**What serializes it.**
- `ComputerEventAppender.mu` (appender.go:162): one append at a time per
  computer, held across all four RPCs and the finalize.
- The telemetry append barrier: every acknowledged write first flushes pending
  telemetry as its own event (`awaitBarrier`, appender.go:240; 33,703 barrier
  seals, 68,458 barrier waits in the dump [O]). So an acknowledged write can
  wait for two appends.
- The guest write pool `max_open 1`: dry-run, Prepare and finalize all use it,
  and so does every other direct-SQL writer. Since boot: `wait_count` 699,568,
  `wait_duration_ms` 8,405,291, mean 12 ms a wait [O dump store_pools]. The
  dry-run runs before `a.mu` (projection_tape.go:200), so concurrent callers
  queue on the pool and then on the mutex.
- corpusd: a per-computer lock in `CompareAndSwap` plus a SERIALIZABLE
  transaction (`platform/computer_events.go:~215-250`).

**Where the caller waits.** In `appendOps`, synchronously, until the event is
durable and finalized. The SQL write of its own change has not happened when the
object write call is made; it happens inside the append. A caller therefore
waits for: the pool, the barrier, the mutex queue, four RPCs, and two SQLite
transactions.

### 1.2 Host measurements, 2026-11-12 09:00-15:00Z [O receipts]

| | |
| --- | --- |
| events | 129,867 gaps (129,868 events), 21.6k an hour, 519k a day |
| gap percentiles (ms) | p10 120, p25 128, p50 142, p75 162, p90 187, p99 544 |
| gaps under 300 ms | 100-125 ms: 24.4k; 125-150: 54.7k; 150-175: 31.1k; 175-200: 10.6k; 200-225: 2.9k; 225-250: 1.3k; 250-275: 0.7k; 275-300: 0.5k |
| busy fraction per hour (sum of gaps under 300 ms / hour) | 89.3, 83.6, 86.4, 81.7, 88.5, 86.3 percent |
| mean gap under 300 ms per hour | 148, 140, 138, 144, 159, 156 ms |

Reading [I]: the gap distribution is a narrow band with a hard floor near
100 ms and no tail of think time. That is the shape of an always-nonempty queue
served at a fixed service time, not of a serial chain with variable caller
delay. The writers are concurrent and blocked behind the appender, so there is
something to coalesce; the unknown is how much (section 4.5).

Daily counts for the paper [O receipts, 11-09 to 11-12]: projection_batch_recorded
77,976 (partial day), 474,140, 544,608, 340,303 (to about 15:30Z); other kinds
per day: artifact_produced 591 to 3,785, file_root_committed 17 to 110,
key_revoked 2 to 32, snapshot_recorded 0 to 1.

## 2. Census of writers

### 2.1 What is measured

The dump's counters (about 18 h since boot) split by *op kind*, by *telemetry
event kind* and by *batch size*, but not by object kind or run-memory kind
[O `telemetry_batching`, `recordTapeEvent` at `telemetry_stats.go:157` counts
`op.Kind` and `op.Table` only]. The payloads are sealed, so the host cannot
split further. The 2026-10-29 offline census
(`docs/evidence/paper-tape-composition-2026-10-30.md`, read from plaintext
segments that no longer exist as plaintext) gives a per-object-kind split for a
day that is 3 weeks old [O doc].

Guest counters, 11-11 21:35Z to the stall [O dump]:

| | count | share |
| --- | ---: | ---: |
| tape events | 360,812 | |
| tape ops | 535,868 | 1.49 ops an event |
| events with 1 op | 316,564 | 87.7% |
| telemetry-authority events (sealer batches) | 42,986 | 11.9% |
| telemetry records sealed | 90,573 (mean 2.1 an event) | |
| acknowledged-authority events | 317,826 | 88.1% |
| ops by kind | object_recorded 298,772; run_memory_entry_recorded 225,061; object_edge_recorded 6,451; texture_agent_mutation_recorded 5,575; object_retire 8; run_memory_retire 1 | |
| batchable telemetry kinds (records) | loop.progress 33,053; tool.result 26,168; desk.cell 26,166; desk.egress 4,604 (plus retries, compaction marks) | |
| payload bytes of the choir.event records | tool.result 121.7 MB (80% of the 152 MB total); desk.cell 23.7 MB; loop.progress 21.9 MB | |
| acknowledged choir.event records | 25,658 (model.served 6,962, search.served 4,389, loop.submitted 3,996, texture revision events about 7,100, system1.judgment 1,178, loop.started/completed about 1,580, story.entities 342, ...) | |

### 2.2 The decomposition of the 360,812 events [I, arithmetic from the counters and code]

Run memory ops can only be single-op events with the switch off (the batch
entry point is not called), so:

| Writer family | Events | Share | Basis |
| --- | ---: | ---: | --- |
| run memory (one entry per event) | 225,061 | 62.4% | op count = event count by code (section 1.1) |
| other acknowledged single-op object events | 73,439 | 20.4% | 316,564 singles - 18,064 telemetry singles - 225,061 |
| telemetry sealer batches | 42,986 | 11.9% | measured |
| acknowledged multi-op events (lifecycle, inbox, Texture commits, run steps) | 19,326 | 5.4% | 317,826 - 298,500; they carry 146,795 ops, 7.6 an event |

Scaled to the host's 519k events a day: run memory about 324k, other single
objects about 106k, telemetry batches about 62k, multi-op about 28k [I].

**Run memory doubled since 10-29.** On 10-29 the paper wrote 125,926 run-memory
entries in a day (5.2k an hour) with 33,475 desk cells and 33,380 tool results
[O census]. In the dump: 225,061 in about 18 h (12.5k an hour) with 26,166 desk
cells and 26,168 tool results, so entries per desk cell went from 3.8 to 8.6,
while the telemetry kinds and the non-event object rate stayed flat (non-event,
non-run-memory object ops: about 8.2k an hour on 10-29, about 10k an hour now)
[I, different windows, same order]. Growth in event rate since 10-29 (339k to
about 519k a day) is mostly run memory [I]. A hypothesis for the cause, not
confirmed from traces: `carryActorConversation` re-writes the carried tail of a
previous run's conversation as one event per message on every rewarm
(`agentcore/run_memory.go:~189-240`, loop at `:229` and `:238`), so entries scale
with the length of the carried tail, not with new work. **Check this in the run
ledger before acting** (CLAUDE.md "Read the traces first").

### 2.3 Writers, triggers, classes

Rates are the paper's. "m" = measured, "i" = inferred. Class: **C** canonical,
**R** reconstructible, **E** ephemeral (restart rules: a crash restart never
resumes work; open obligations are closed at boot; standing intents survive).

| Writer (kind) | File:line | Trigger | Rate on the paper | Class |
| --- | --- | --- | --- | --- |
| run memory `message` (assistant and tool-result messages) | `store/run_memory.go:31` (AppendRunMemoryEntry), `agentcore/run_memory.go:437` | per model turn and per tool result | 12.5k ops/h (m, all kinds) | C for the run's own context while it lives and for the next run of the actor if it completed or passivated; R after the run closes, in the sense that nothing reads it again except `LatestActorRunMemoryEntries` |
| run memory, carried tail (copies of earlier entries) | `agentcore/run_memory.go:~189-240` | per actor rewarm, one event per carried message | unknown; hypothesis: a large part of the doubling (i) | **R**: each carried entry is a copy of an entry already on the tape in the source run (`source_loop_id` is in the checkpoint's details) |
| run memory `rlm_inbox_cursor`, `cell_fate`, `compaction`, `texture_cell_author_receipt` | `agentcore/rlm_reduce.go:599,662`, `agentcore/run_memory.go:558,640`, `textureowner/texture_cell_author.go:126` | per desk cell / compaction / authoring | a few per cell (i) | C (the audit in design section 9: loss duplicates work) |
| `choir.event` telemetry kinds (loop.progress, tool.result, desk.cell, desk.egress, retries, compaction marks) | `store/graph_store.go:1238` (AppendEvent), `types/event_durability.go:36-45` | per step, via the sealer | 90.6k records, 43k events, in 18 h (m) | E for loop.progress, desk.cell, desk.egress, retries (trace lines: the owner already accepts a crash-lossy window, ruling 2026-10-14). **tool.result is C-as-evidence** (run acceptance and the Texture verifier read it) **and R in content**: it duplicates the tool message in run memory (121.7 MB of the 152 MB) |
| `choir.event` acknowledged kinds | `types/event_durability.go:47-110` | per run transition / provenance | 25.7k in 18 h (m) | C |
| `choir.run` (4 writes a run, mean 9.1 KB, 11% of op bytes on 10-29) | `store/graph_store.go:597` | submit, start, progress marks, finish | 18.7k a day on 10-29 (m, old); today (i) similar | the final row is C; the intermediate rows are **R** from the `loop.submitted/started/completed` events, which are on the tape separately (unverified in code that a reducer can rebuild them; check) |
| `choir.source_schedule` | `agentcore/source_schedule.go:157,162,227,344,433,470,509`; `store/source_schedule.go:163` | per poll tick: an in-flight mark, then the result; plus rearm, repair | 66 keys x 83 writes a day, 6.6 KB each (m, 10-29) | the schedule as a standing intent is C; the **in-flight marks (`PollInFlightSince`, line 227) are E** (only read by crash boot to close the poll and count the crash) |
| `choir.actor_wake_outbox` | `store/lifecycle.go:894`, `store/actor_wake_signal.go` | enqueue, then done, per wake; one per poll tick | 24k ops a day (m, 10-29), 2 writes a key | **E/R**: a crash boot closes undelivered work; a planned restart resumes once from a marker. Delivered/done state is R from the delivery it records |
| `choir.platform_poll` | `agentcore/platform_poll.go:260,281`; `store/platform_poll.go:97` | oped_intake every 1 min, notice_intake every 5 min, an in-flight mark and a result each (`:31-32`) | about 3.5k events a day, 0.7% (i, from the code) | in-flight mark **E**; the cursor and gate queue C |
| `choir.lease`, `stream_seq_lease` | `store/lease.go:44` | per lease renewal | 3 ops a day (m, 10-29) | E, negligible |
| `choir.agent`, `trajectory`, `management_task`, `capture_disposition`, `conversation_head`, `inbox_head`, `inbox_cursor` | `store/engineering_assignments.go:1341,1647,...`, `store/inbox.go:641`, `store/management_tasks.go` | per run step / message | 15.6k, 6.5k, 3.9k, 6.8k, 7.1k, 10.7k, 5.3k a day (m, 10-29) | working state (restore needs it): C |
| `choir.story_library_head` | `store/management_tasks.go:44` | per story commit | 997 writes of 21 KB a key on 10-29 (m) | state, but a counter whose body carries a list: churn, not canonical history (needs only its last value); R-by-latest-value |
| lifecycle command/event, inbox command | `store/engineering_assignments.go:1034,1165`, `store/inbox.go` | per transition | 4.7k, 4.7k, 11.4k a day (m) | C (idempotency receipts) |
| acts: texture_revision/document, source_ref/entity, support_verdict, citation_check, commitment_record, capture_label, world_event | store/ texture.go, support.go, commitment.go, capture_label.go, world_event.go | owner and desk acts, publications, evidence | 20.5% of bytes (m, 10-29) | C |
| edges, Texture agent mutations | `projection_tape.go:99-110`, `texture.go:349,1340` | per link / per authoring turn | 6.5k and 5.6k in 18 h (m) | C |
| desktop/media/preferences | `desktop_live.go:73`, `media_projection.go:229` | owner UI | about 0 on the paper | C (owner) |

**Heaviest reconstructible writers** (by events): (1) the carried-tail run
memory copies (size unknown, suspected large); (2) `choir.run` intermediate rows
(about 4 a run); (3) tool.result bodies as telemetry (80% of telemetry bytes,
duplicated in run memory).
**Heaviest ephemeral writers** (by events): (1) the telemetry trace lines, 43k
events in 18 h even though already batched (they vanish as events if they ride,
section 4); (2) the source-schedule and platform-poll in-flight marks plus
actor-wake outbox pairs, about 2% to 4% of events (i: 66 schedules x about 40
ticks x about 4 events each, plus polls), 3% of bytes on 10-29.

**Restart-rule check on "ephemeral".** The in-flight marks are not useless: the
crash-loop bound ("a schedule whose tick crashes the computer twice is closed",
CLAUDE.md "Restarts") reads them at boot. They need to survive a guest crash
and a planned restart, not a loss of the guest disk. A fsynced boot-local
journal on the guest disk, excluded from the replay witness, satisfies both
[I]. If the disk is lost the restore closes every obligation anyway (doctrine).
This is a restart-semantics change and should be red.

### 2.4 The smallest counter that settles the split

Today it cannot be measured. Add to `telemetryCounters` (`telemetry_stats.go`,
exposed in the observability `telemetry_batching` block):

1. `tape_ops_by_object_kind` and `tape_op_bytes_by_object_kind`: in
   `recordTapeEvent` (`:157`), for `object_recorded` ops take the kind from the
   canonical id (`obj:<kind>:<owner>:<suffix>`, `objectgraph/object.go:76`);
   count `len(op.Body)`.
2. `run_memory_ops_by_kind_and_reason`: one increment per entry in
   `appendRunMemoryTapeLocked` (`store/run_memory.go:~215`) keyed by `Kind` and
   by whether `Reason` is `actor_rewarm` or the entry came through the carry
   loop.
3. `appender_waiters_at_enter` histogram (0, 1, 2-3, 4-7, 8+): sampled on entry
   to `AppendNew*` before `a.mu.Lock()`, plus the mutex wait time. This is the
   number that predicts the group size (section 4.5).

Class: **yellow** (detectors; no behaviour change), though it edits the tape
write path files, so name the files in the commit. Rollback: revert. These are
cheap enough to land first and would resolve section 2.2's inferences in a
day.

## 3. Classification summary

| Class | Writers | Share of paper events now [I] |
| --- | --- | --- |
| Canonical (must be on the tape) | acts, Texture, evidence, owner acts, run transitions, idempotency receipts, edges, working state | about 25% |
| Canonical for the life of a run, reconstructible afterwards | run memory messages | up to 62% (the biggest) |
| Reconstructible | carried-tail copies, tool.result duplicate bodies, `choir.run` intermediate rows, story_library_head intermediate values | unknown, suspected 10% to 25% |
| Ephemeral (trace, in-flight marks, wake pairs, polls) | telemetry trace lines, platform_poll, source-schedule in-flight, actor_wake_outbox pairs | 12% (telemetry, measured) + 2% to 4% (i) |

Conjecture A is therefore **partly true but not the main lever**: the
ephemeral share is about 15% to 17% of events, and the telemetry part of it is
already batched into 12%. The big share is acknowledged run memory, which is
the agent record (the frame calls it canonical: `choir-data-system-frame` section
5, 33.7% of paper bytes), so moving it off the tape needs an owner act, and the
owner has already refused a loss window on it. What does not need to be on the
tape is the **number of events** it occupies, not the data.

## 4. Group commit design sketch (lossless half only)

### 4.1 Scope

Ship the leader/follower coalescing that exists, minus its two crash-lossy
pieces, with no setting. Delete:
- `telemetryGroupWindow` 10 s and the clamp change (`tape_group_commit.go:35`,
  `telemetry_sealer.go:54`): telemetry keeps today's 1 s window;
- `run_memory_batch.go` (the message window, damaged-run logic, tail cache) and
  `runMemoryKindBatched`.
Keep: pending, `takeGroup`, `leadGroup`, `sealRiding` (telemetry riding the
leader's event does not lengthen its loss window; it shortens it, because the
queue is sealed at the next acknowledged write), the refusal split, the caps.
This is also what the contract wants ("no switches", "no limits on our own
model calls": coalescing what is queued is neither).

### 4.2 Coalescing point

`projectionTape.appendOpsAs` (`projection_tape.go:158`), where it already is.
The test `groupableOps` (`tape_group_commit.go:~113`) accepts any batch of
`object_recorded`, `object_edge_recorded`, `run_memory_entry_recorded`. Add the
rest of the "unconditional upsert" op kinds that carry no guard
(`desktop_state_recorded`, media/preferences) when measured; keep guarded kinds
(`self_development_*`, `texture_agent_mutation_recorded` with
`ExpectedStates`/`RequireRevision`, retire ops) direct.

One gap in the existing F4: a `PutBatchConditional` write reaches `interceptOG`
as plain `object_recorded` ops, because its conditions were checked earlier on
committed state (`sql_store.go:1022-1026`) and are not carried into the op. So
conditional lifecycle batches are grouped today if the switch is on. That is
the same check-then-append race the direct path has (the check runs before the
queue wait), and a shorter queue narrows it; but F4's statement that guarded
writes go direct is not true for object conditions. Either carry a flag from
`putBatchChecked` to `appendOps` (`conditional` -> direct), or re-check
conditions at the leader. Take the first (smaller).

### 4.3 What the event contains and the guarantees

One `projection_batch_recorded` event with ops in this order: riding telemetry
(enqueue order), then members' ops (arrival order under `g.mu`). Authority
`authority:vm-local-projection`. Idempotency key `projection-batch:<event id>`
(minted per group). Same format every release reads; replay applies the ops in
order and positions (`pos`, op index) come from the tape, so live finalize and
replay agree (design F8). Atomicity: all members' ops are one SQLite
transaction in `finalizeBatch` and one tape event; ordering: a write issued after
another's acknowledgement lands in a later event (F6); members of one group have
no order among themselves beyond arrival, which no caller can observe.
Intermediate heads between ops of one batch are not restore targets (comment at
`projection_batch.go:~99`), so restore granularity becomes one group.

### 4.4 The caller's wait

Before: dry-run on the write pool, barrier flush (an append), queue on `a.mu`
behind all earlier waiters, then its own append: roughly (queue length + 2) x
142 ms. After: wait for the group in flight (at most one cycle), then for its own
group's cycle. A lone writer commits at once (no timer). Latency is bounded by
about two group cycles whatever the queue depth, so p99 stops scaling with
concurrency [I]. Costs per group: one dry-run, one Prepare, four RPCs, one
finalize, one receipt row, shared by every member. This also removes most of the
write-pool waits (699k waits since boot).

### 4.5 Expected reduction (the estimate and how it was made)

Anchors [O]: 129,868 events in 6 h; 3,725 runs of events under 300 ms apart,
mean 33.9 events a run (about 5 s); 273 gaps over 2 s; busy 82% to 89%.

- **Floor** (not reachable): one event per run = 3,725 per 6 h = 14.9k a day.
  It would require every run's writers to be queued together at the start.
- **Chain bound** [I]: the events in a run are served from m concurrent writer
  chains; a chain's own writes cannot merge (each waits for its previous ack),
  so a run of 34 events from m chains takes about max chain length rounds. With
  m = 3 to 8 (the paper runs 4 management slots plus Texture, research and
  engineering desks, 8 running runs at 10-29 health, the poll and schedule
  loops, the sealer) and a skew of 1.5, a run of 34 events becomes 6 to 17
  groups: 22k to 63k events per 6 h, 90k to 250k a day on the 519k base.
- **Decomposition** (section 2.2) with a merge factor G on the 430k single-op
  acknowledged events: G = 2: 215k + 14k + about 5k = 234k a day (-55%); G = 4:
  108k + 7k + 5k = 120k (-77%); G = 6: 72k + 5k + 5k = 82k (-84%). Telemetry
  batches (62k a day) go to near zero by riding.
- **Realistic: 100k to 200k a day, central 130k (-75%).** The factor that
  decides it is G, the number of writers blocked when a commit ends. The
  `appender_waiters_at_enter` histogram (2.4) measures it on the paper without
  changing behaviour, and it is the single number to read before building.
- **Closed-loop caveat** [I]: if the paper is throttled by the appender today,
  faster commits let callers do more work and ops a day rise; the events
  figure falls less than the table says. The sealer's `seal_latency_mean_ms`
  694 ms and max 27.6 s, and 67 queued management tasks at the stall, hint
  that some throttling exists, but the stall data are not representative.
  Re-measure after.
- Plus: call-site batching (R2 below) lowers the base G is applied to.

### 4.6 Interaction with prepared-row and recovery logic

(`lane/appender-wedge` is changing appender.go for re-send on unclear CAS and
in-place catch-up. None of the following duplicates it.)

- Layering: group commit sits above the appender and makes one
  `AppendNewPrivatePayload` call per group, so one prepared row per group.
  Recovery (`recoverPreparedInPlaceLocked`, `RecoverPrepared`) sees an ordinary
  event.
- Contract needed from the lane: `AppendNewPrivatePayload` returns nil only
  when the event is CAS-verified and finalized, and returns an error only when
  the outcome is known failed or unresolvable. If the lane makes the appender
  settle an unclear CAS by re-sending the same request (same idempotency key
  and request commitment, so `existingComputerEventReceipt`,
  `platform/computer_events.go:333`, returns the stored receipt), the group sees
  success and every member returns nil. The group layer's "unknown outcome:
  every member gets the error, never retried" (F3) then only fires on a
  genuinely unresolvable failure. Keep that rule; do not add a second retry in
  the group layer.
- Blast radius: one failed group fails every member, where before it failed one
  caller. The wedge of 11-12 came from a lost CAS reply on one of more than 500k
  CAS a day [O `docs/problems/...` commit 6fa8668ee]; with 4x fewer events the
  exposure falls about 4x [I].
- `ErrBatchRefused` (nothing pinned) splits to singles in arrival order after
  sealing the riding telemetry (F2). A refused group costs N appends: acceptable
  as it is rare.
- `HoldAppends` (`appender.go:280`, used by replay completeness and checkpoint
  restore) holds `a.mu`, so the leader waits on it like any append.

### 4.7 Replay and snapshot implications

- Replay reads one payload per event (`replay_prefetch.go`, one request per
  event) and verifies one receipt per event (`appender.go:1065`), so replay time
  falls with events, not ops [I].
- Snapshot due-ness is by tail events (540,000). At 130k events a day the
  producer fires about every 4 days instead of every 1.1 days, so snapshot chunk
  writes fall about 4x (to the 7-day backstop at worst). Recovery cost per
  snapshot interval is bounded by the replay of 720k events, which now covers
  about 4x as many ops: fine for correctness, but recovery time per event grows
  with a larger payload [I]; measure replay seconds before and after.
- `snapshot_recorded` boundaries are event boundaries; a group is one boundary.
- Witness/equivalence probe unchanged: same ops, same order.

## 5. Receipts

### 5.1 What a row holds [O schema `platform/computer_events.go:40-56`, sizes `pg_column_size`, paper, newest 20k rows]

| column | type | bytes |
| --- | --- | ---: |
| computer_id | VARCHAR(128) | 42 |
| idempotency_key | VARCHAR(255) | 54 |
| request_commitment | CHAR(64) | 65 |
| sequence | BIGINT | 8 |
| event_kind | VARCHAR(64) | 26 |
| event_digest | CHAR(64) | 65 |
| event_pin_receipt_digest | CHAR(64) | 65 |
| event_head_receipt_id | VARCHAR(255) | 37 |
| event_head_receipt_json | TEXT | 473 compact (`c1:` form), 1,511 for older rows |
| created_at | TIMESTAMPTZ | 8 |
| whole tuple | | 880 |

Table: 11.5M rows (reltuples), heap 17 GB, indexes 9.88 GB, total 27 GB, so
2.34 KB a row [O]. Five btrees, bytes a row: pkey (computer_id, idempotency_key)
188; (computer_id, sequence) unique 149; (computer_id, event_digest) unique 202;
(computer_id, event_head_receipt_id) 201; (computer_id, event_kind, sequence)
159; total 859 [O pg_relation_size / reltuples]. So the 2.2 KB is: about 0.9 to
1.5 KB heap (the compact JSON is 473 B; hex columns are stored as text, not 32
bytes) plus 0.86 KB of indexes, 69% of which sit on three random hex or uuid
keys (pkey on the idempotency key, digest, receipt id).

**Two measured side facts.** (a) Rows below about sequence 5M still carry the
full 1,511-byte JSON (deciles 0 to 4 of the paper's sequence are 1,511 B; 5 to 10
are 473 to 637 B), although `computer_event_receipt_compaction` records the
paper as `active`, `through_sequence` 10,939,797, 518,320 rows rewritten,
10,421,477 kept, bytes 10.97e9 to 10.46e9 [O]. About 5M rows x about 1.04 KB is
about 5 GB of heap still to recover; why `kept` is large is not determined
here. The table has 0 dead tuples and was autovacuumed 11-10 [O pg_stat], so
the 17 GB heap is genuine row bytes, not bloat. (b) The paper is about 90% of
the table by sample [O].

### 5.2 Readers [O grep]

Per-event rows are read by: guest replay (`platform/event_replay.go:49` pages;
`computerevent/appender.go:1065` verifies every receipt's signature);
idempotency/resend (`platform/computer_events.go:333`); tape sweep verification
(`tape_sweep.go:667-745`); bundle export and import, one row per event
(`bundle/store.go:38`, `bundle/import.go:473`); checkpoint verification and list
(`checkpoints.go:62,83,120,137,224,252,354`, `checkpoint_list.go:98`, by receipt
id and event digest); clean rebase (`clean_rebase.go:181,293`); verifier key
pin (`verifier_key_pin.go:67,174`); purge (`tape_purge.go:78`); compaction
(`receipt_compaction.go`).

### 5.3 Per-segment signed root instead of per-event receipts

Possible, and it is the frame's destination (`choir-data-system-frame` section 3
and 4: "one signed receipt for the batch", "the control database holds only the
segment index", per-event receipts "go away: the segment is the pin"). Notes
[I]:
- Events already chain (each binds `previous_head`; `Reduce` recomputes the
  head, `appender.go:1054`), so a signature on the last head of a segment
  attests the whole segment. The root would sign (computer, first and last
  sequence, last head, digest of the ordered event and pin digests).
- Past the newest verified snapshot, history is only for export and audit
  (`MaxRecoveryTailEvents` bounds restore to snapshot plus tail), so rows with
  sequence at or below that snapshot are the first to convert: keep full rows
  for the tail (idempotency, resend, checkpoints) and for the rare kinds
  (artifact_produced, snapshot_recorded, file_root_committed, key_revoked,
  trajectory_started: under 1% of rows) that checkpoint and key-pin lookups need.
- Work: replay verification at `appender.go:1065` becomes per-segment;
  `bundle` format is per-row lines; `tape_sweep` and `checkpoints` lookups by
  digest and receipt id must keep working for the retained kinds. Class red
  (receipt and head projection, deploy routing). Prove on d@e with the
  existing chaos drills and a bundle round trip.
- Smaller, earlier step with most of the host-side win: after group commit the
  rows fall 4x; drop the unique digest and the receipt-id indexes for
  `projection_batch_recorded` rows (partial indexes `WHERE event_kind <>
  'projection_batch_recorded'`; the lookups above are all by non-batch kinds),
  saving about 400 B a row of 2.34 KB (17%), and finish the old-row compaction
  (about 5 GB once). Also red (host schema).

## 6. Ranked changes

Base: paper 519k events a day, tape 6.4 KB an event (given; of which about
3.5 KB is fixed envelope plus two pin receipts, from the 10-29 census, so about
2.9 KB variable [I]), receipts 2.34 KB an event [O], about 25 GB of chunks per
snapshot [O comment] at one snapshot per 1.1 day.

| # | Change | Events/day | Tape GiB/day | Receipts GB/day | Append latency | Class | Rollback | Measure first |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 0 | Counters of section 2.4 (object kind and bytes, run-memory kind and carry, appender queue depth histogram) | 0 | 0 | 0 | 0 | yellow | revert | this is the measurement |
| 1 | Run memory writes as one acknowledged event per call: `appendMessages` and the carry loop call `AppendRunMemoryEntries` unconditionally (the batch path exists and is durable with the switch off: `store/run_memory.go:233-264`); batch parallel tool results; no loss window | -0 to -30% (needs counter 0; hypothesis: the carried tail is a large part of the doubling) | proportional on the fixed 3.5 KB | proportional | the run's opening burst drops from N x 142 ms to about 142 ms | orange | revert | carry vs fresh share of run memory ops |
| 2 | Lossless group commit: keep the leader/follower and the telemetry ride-along, delete the 10 s window, the run-memory window and the setting; conditional batches go direct | 519k to 100k-200k, central 130k (-75%) | 3.3 to about 2.0 (-40%; fixed part only) | 1.2 to about 0.3 (-75%) | caller wait bounded by about 2 group cycles; mean falls with queue depth; busy fraction 88% to about 30-40% [I] | red (tape write path) | revert; the refusal split stays as correctness | `appender_waiters_at_enter` histogram on the paper: G >= 3 on at least 60% of commits; d@e chaos s1-s3 and the replay equivalence probe |
| 3 | Cut RPCs per event: drop the `cas.Head` GET when the appender holds the head from its last verified receipt and nothing else wrote (sole writer under `a.mu`); fold the event pin into the CAS call (corpusd validates the pins at CAS already) | 0 | 0 | 0 | 142 to about 70-90 ms an event (i: 4 RPCs to 2) | red; touches appender.go: serialize behind `lane/appender-wedge` | revert | per-RPC timings (guest side spans for the four calls) |
| 4 | Snapshot cadence follows automatically from 1 and 2 (no change) | | chunk writes about 24 to about 6 GB/day (-75%) (i) | | | n/a | n/a | check `snapshot_recorded` interval after 2 |
| 5 | In-flight marks and wake/outbox pairs to a boot-local journal off the tape (platform_poll `:260`, source_schedule `:227`, wake outbox pairs) | -2% to -4% (i), after 2 these fold into groups so smaller | -3% bytes (source_schedule) | small | small | red (restart semantics) | revert | counter 0 per kind |
| 6 | Receipts schema: partial digest and receipt-id indexes for batch rows; finish old-row compaction (about 5 GB once) | 0 | 0 | -17% per row, plus 5 GB once | 0 | red (host schema) | recreate indexes | confirm all lookups listed in 5.2 are by non-batch kinds |
| 7 | Per-segment signed root replacing per-event receipts past the newest verified snapshot (frame section 4) | 0 | -pin receipts (about 1.9 KB of the 6.4) | -90% or more | 0 | red | keep rows during a trial window | needs replay verification, bundle format and sweep redesigned |
| 8 | Telemetry off the tape (frame section 5, class 4) | after 2 only telemetry ops remain, no events | -5% bytes | 0 | 0 | red | revert | not needed once 2 lands; owner decision on tool.result (evidence) |

Recommended order: 0, then 1 (cheap, deterministic, independent of
concurrency), then 2, then 3 (after the wedge lane lands), then 6, then 5 and
7. Do not build 2 before the histogram: if G is 1 to 2 the gain is 40% not 75%
and 3 is the better lever.

**Calibration entries to enter before landing 2** (CLAUDE.md "Proof"): P(median
waiters at enter >= 3 on the paper) = 0.75, settle on counter 0; P(events a day
under 200k after 1 and 2) = 0.7, settle on the host receipts over three full
days; P(snapshot interval >= 3 days) = 0.6, settle on `snapshot_recorded`
spacing.

**Owner-facing note.** Shipping 2 with no setting follows "no switches"; the
lossless half is not the thing the owner objected to, but the code and the
ruling are both in the repo under the same name, so the change should say it
supersedes the 11-03 delete ruling for the leader/follower part and keeps the
ruling for the windows.

## 7. Not determined here

- The per-object-kind and per-run-memory-kind split of today's events (sealed;
  counters needed).
- Whether `carryActorConversation` is the cause of run memory doubling.
- The true waiter depth G (needs counter 0).
- Why `receipt_compaction` kept 10.4M rows full for rows below sequence 5M.
- Per-RPC times of the four calls (the 142 ms is the whole event; the vsock
  probe in `vsock_rtt.out`, 33 to 47 ms p50 to first byte, is the only related
  number and is a different probe).
- Whether `choir.run` intermediate rows are rebuildable from the loop.* events
  by a reducer today.
