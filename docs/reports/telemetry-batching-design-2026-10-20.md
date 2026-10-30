# Batching run telemetry: classification, window form, chain shape (2026-10-20)

Date: 2026-10-20. Design only. No runtime code changed, nothing run on Node B or
any computer, no tape touched. Mutation class of this doc: green. Mutation
class of every slice it proposes: **red** (tape write path and evidence), except
phase 0, which is orange.

Owner ruling this design serves
([`owner-notes-2026-10-14.md`](owner-notes-2026-10-14.md) §27 item 18):
"**Batch run telemetry.** Telemetry events may be sealed in small batches
instead of one acknowledged write each; a crash may lose up to the batch window
of telemetry. The window form goes through a panel."

Inputs, read in full: CLAUDE.md,
[`tape-volume-forward-fix-and-surgery-2026-10-19`](tape-volume-forward-fix-and-surgery-2026-10-19.md)
(the "forward fix"; §3.5, §6, A0 to A3),
[`projection-event-volume-design-2026-10-19`](../archive/reports/projection-event-volume-design-2026-10-19.md)
(the "volume design"; stage 2),
[`platform-artifact-small-file-growth-2026-10-19`](../problems/platform-artifact-small-file-growth-2026-10-19.md)
(the "host record"), and the code cited inline.

Evidence labels: **[V]** verified by reading the code at the cited `file:line`
on this checkout (`0a541483c`); **[M]** measured, in a cited record, not
re-measured here; **[H]** hypothesis or derivation, with the probe that would
settle it.

## Answer in brief

1. **What batches.** Only a closed allowlist of ten high-volume, low-authority
   runtime event kinds (`loop.delta`, `loop.progress`, `loop.retry`,
   `loop.compaction.started`, `loop.compaction.completed`, `tool.invoked`,
   `tool.result`, `desk.cell`, `desk.egress`,
   `texture.agent_revision.progress`), written by the standalone
   `Store.AppendEvent` path. Every other `choir.event` kind, every other op
   kind, every non-batch tape event kind, and every run-memory entry stays one
   acknowledged write. Anything unclassified is acknowledged. Section 3.
2. **Shape on the tape.** One sealed batch is **one ordinary
   `projection_batch_recorded` event holding N `choir.event` object ops**.
   Not N chained events. No payload format, reducer, replay or witness change;
   only the event's `authority_ref` differs, so the tape itself says which
   batches are telemetry. Section 5.
3. **Window form.** A hybrid, with the time bound measured **from the first
   unsealed record, not sliding**: seal at the first of `W` = 1 s (default,
   runtime-tunable 250 ms to 2 s), 64 ops, 256 KiB, or a barrier. Barriers are:
   any acknowledged write except run memory (so the tape never shows a semantic
   event ahead of earlier telemetry), any evidence or stream-catch-up read,
   shutdown, and the planned-restart marker. Section 4.
4. **Honest reduction.** About 1.5x to 2x fewer tape events (not 6x), because
   run memory, which is not telemetry, and every semantic write interleave with
   the telemetry, and the ordering rule seals at each of them. On the host
   today's 4.02 files per event goes to roughly 2.0 to 2.7 files per event
   before segments, and A2 makes it near zero regardless. Section 7.
5. **Two hazards the owner ruling did not name.** Stream and per-run sequence
   numbers can be **issued twice** (after a crash, after a second crash, and
   in-process when the sequence cache is dropped under a live queue; section 6.1,
   blocking), and the **evidence readers** (run acceptance, the Texture workflow
   verifier, run-memory compaction) read `tool.result` events (section 3.3).

## 1. Current state

Two event vocabularies exist, and the ruling concerns only the inner one.

- **Tape events** (`computerevent.EventKind`, 26 kinds,
  `internal/computerevent/event.go:33-60` **[V]**): `genesis_imported`,
  `effect_accepted`, `verification_recorded`, `materialization_*`,
  `checkpoint_published`, `projection_batch_recorded`, and so on. Of these,
  `projection_batch_recorded` is 99.2% to 99.6% of the volume **[M]** (host
  record). The rest are rare semantic events that call `AppendNew*` directly
  (forward fix §1, last bullet).
- **Runtime events** (`types.EventKind`, 42 kinds,
  `internal/types/task.go:157-327` **[V]**): the `loop.*`, `tool.*`, `texture.*`
  and product kinds. Each is stored as one `choir.event` object, which is one op
  inside one `projection_batch_recorded` tape event. On 10-15 these were 207,658
  of 358,503 batch events, 58% **[M]** (host record, "What
  `projection_batch_recorded` is"). Run-memory entries were 111,636, 31%.

The funnel, all **[V]**:

| Step | Where |
|---|---|
| `Store.AppendEvent` allocates the per-run `seq` and global `stream_seq` in memory under `eventMu`, releases `eventMu` when the tape is bound, then writes | `internal/store/store.go:1929-2028` (tape release at `:2009-2011`) |
| `AppendEventOG` is one `ogPut` of a `choir.event` object | `internal/store/graph_store.go:1168-1198` |
| The OG interceptor turns the object into one `ProjectionOpObject` and calls `appendOps` | `internal/store/projection_tape.go:56-84` |
| `appendOps` builds one `ProjectionBatch`, fresh UUIDv7 event id, key `projection-batch:<id>`, authority `authority:vm-local-projection`, and calls `AppendNewPayload` | `projection_tape.go:97-135` (authority at `:126`) |
| `AppendNewPayload` takes the appender mutex, dry-runs the batch, pins the payload, appends, and only then returns | `internal/computerevent/appender.go:338-406` |
| The caller (`emitEvent`) logs an error and publishes to the SSE bus after `AppendEvent` returns | `internal/agentcore/runtime.go:4555-4580` |

So a runtime event is acknowledged only after the tape CAS and the guest
finalize, four host round trips and two guest SQL transactions each
(forward fix §1). Two facts matter for batching:

- **Live streams do not read the store.** `emitEvent` publishes to
  `rt.bus` in memory. The WebSocket sends a one-time store catch-up for
  `afterSeq`, then follows the bus (`internal/agentcore/live_ws.go:75-90`,
  `:98-`). So the live UI is independent of durability latency **[V]**.
- **Some readers do use the store as evidence** (section 3.3).

Already landed since the forward fix: A0's `livePayloads` leak fix
(`appender.go:395-403`, commit `b36519198`). A1, A2 and A3 have not landed
(`internal/tapestore` does not exist). **[V]**

## 2. Design principles

1. **Allowlist, not classifier.** A kind batches only if it is named in one
   table, with a reason. Default is acknowledged. This is the forward fix's
   rule for A3 marking (§3.1) applied to kinds.
2. **Class belongs to the kind, never to the caller.** A call site cannot ask
   to be batched or exempted. The caller's only input is a record; the kind
   decides.
3. **A write path can also disqualify.** A `choir.event` folded into a
   conditional batch with a run-state projection
   (`UpdateRunWithEvent` -> `persistLifecycleRunWithEvent`,
   `internal/store/lifecycle.go:3249-3257`) is part of that atomic acknowledged
   write whatever its kind. So the rule is: batched iff (standalone
   `AppendEvent`) and (kind in the allowlist) and (flag on).
4. **Loss is a tail, never a hole.** Whatever the window loses on a crash is
   the newest telemetry of the run, never telemetry in the middle with later
   events present. This is what the ordering barrier buys (section 4.3).
5. **Telemetry is lost; nothing else is.** The ruling allows telemetry loss
   up to the window. It does not allow a semantic act, a canonical revision, an
   owner act, evidence or an acceptance to be late or lost.

## 3. Classification

### 3.1 Runtime event kinds (`types.EventKind`)

"Batch" means standalone `AppendEvent` may seal it in a window. Everything
else is acknowledged exactly as today.

| Kind | Class | Reason |
|---|---|---|
| `loop.delta` | **batch** | streaming text; the run-memory ledger already drops it (`agentcore/run_memory.go:1163`) |
| `loop.progress` | **batch** | per-iteration progress and per-call cost lines. Cost endpoint is `Estimate: true` (`apihandler/costs.go:95-102`); a crash undercounts by at most a window |
| `loop.retry` | **batch** | trace line |
| `loop.compaction.started`, `loop.compaction.completed` | **batch** | trace notices. The compaction itself is the run-memory entry, which is acknowledged |
| `tool.invoked`, `tool.result` | **batch, with the cite barrier (3.3)** | per tool step; the bulk of run telemetry. Read as evidence by acceptance and the verifier, so those readers barrier |
| `desk.cell`, `desk.egress` | **batch** | trace-bundle records (`types/task.go:215-226`) |
| `texture.agent_revision.progress` | **batch** | progress ticks for an open document |
| `loop.submitted`, `loop.started`, `loop.completed`, `loop.failed`, `loop.blocked`, `loop.cancelled`, `activation.passivated` | **never** | run state transitions. Low volume (a few per run). Terminal events are also what a boundary reads |
| `runtime.health`, `runtime.degraded` | **never** | operational incident signal, ownerless, rare |
| `channel.message` | **never** | inter-agent coordination record (`types/task.go:228-230`) |
| `email.draft.approval_recorded`, `email.draft.blocked`, `email.draft.sent` | **never** | the owner's approval and the send are acts and evidence. `HandleInternalRunEvents` admits only the first and third (`agentcore/api.go:437-444`) |
| `browser.*` (6 kinds) | **never** | owner-visible session acts |
| `texture.agent_revision.started`, `.completed`, `.failed`, `texture.document_revision.created`, `texture.decision.recorded` | **never** | the Texture desk's canonical-revision and decision events. Texture is the sole writer of canonical revisions; the revision object is its own acknowledged write, and these events are what editors follow |
| `desktop.*` (3), `content.item.created`, `media.*` (2), `theme.updated`, `file.changed` | **never** | product state events (owner acts); low volume |
| any kind not in this table | **never** | default |

Ten kinds batch; thirty-two never do. The ten were chosen by authority first and
volume second. The per-kind volume is **not measured**: the 58% is all
`choir.event` kinds together. Phase 0 adds a per-kind counter so the table is
priced before any batching is enabled. **[H]**: the ten kinds are most of the
58%, because the thirty-two never-batch kinds are per-run or per-owner-act
events, not per-step events.

### 3.2 Everything that is not a runtime event, and never batches

| Surface | Why it never batches | Evidence |
|---|---|---|
| **Run-memory entries** (`AppendRunMemoryEntry`) | Not telemetry. They are the run's durable context and the desks' state: `rlm_inbox_cursor` entries are the unread watermark a respawned desk resumes from (`agentcore/rlm_reduce.go:33-35`, `:511-525`, `:538-547`); `cell_fate`, `compaction` and `message` entries rebuild provider context. They also cannot be queued: `seq` and parent are read from SQL under one store-wide mutex held across the whole append (`store/run_memory.go:20-21`, `:53-71`, `:84-86`) | forward fix §2 item 1 |
| Commitments, wakes, work items, source refs, lifecycle objects | Semantic acts, authority records | forward fix §1 |
| Texture canonical document revisions, aliases, agent mutations | Texture's sole-writer canonical state | `store/lifecycle.go`, `ProjectionOpTextureDocumentAlias*` (`computerevent/projection_batch.go:55-60`) |
| Owner acts, desktop state | Owner intent | `projection_tape.go:86-95` |
| Run acceptance records, verifier receipts | Acceptance is evidence | `agentcore/run_acceptance.go` |
| Self-development ops and start intents | Authority and idempotency bindings | `projection_batch.go:55-60` |
| Conditional OG batches (`PutBatchConditional`) | Check-then-act; forward fix §2 item 8 | `objectgraph/dolt_store.go:832-848` |
| An event folded into a run projection | Part of an atomic acknowledged write (principle 3) | `store/lifecycle.go:3249-3257` |
| All 25 tape event kinds other than `projection_batch_recorded` | Called via `AppendNew*` directly; structurally unreachable by the sealer | forward fix §1 |
| Any `ProjectionBatch` with a guarded op (`ExpectedStates`, `RequireRevision`, `CreateOnly`) | A3 rule: guarded ops never merge | volume design (a) |

### 3.3 Telemetry that evidence readers read

Five readers treat `choir.event` records as evidence or as input to a decision,
all **[V]**:

- `synthesizeRunAcceptance` reads `ListEventsByTrajectory(...)`, then cites
  events by id and `StreamSeq` in the acceptance record
  (`agentcore/run_acceptance.go:91`, `:140-147`, `:283`).
- The Texture workflow verifier reads `tool.result` events to prove "worker run
  creation is backed by parent tool results" and artifact-write results
  (`textureowner/texture_workflow_verifier.go:96`, `:315`, `:348`, `:367`).
- Run-memory compaction serializes the run's event ledger into a summary
  (`agentcore/run_memory.go:437`).
- WebSocket catch-up after a reconnect (`agentcore/live_ws.go:79`).
- The internal run-events GET used by desk tooling (`agentcore/api.go:432`).

Rule, the **cite barrier**: a reader that decides from events, or a record that
cites an event by id, reads through the `Consistent` list API, which first seals
the pending batch and waits. In-process, an acceptance can therefore never cite
a telemetry event that is not on the tape. After a crash a lost event is simply
not there: an acceptance is blocked rather than falsely accepted. That is the
safe direction for acceptance, but the verifier is a different case: a
crash-lost `tool.result` tail would make `verifyWorkerRunToolCausality` or
`RequireArtifactWriteEvent` fail a legitimately backed run, and a failure
receipt would then be produced by ruled telemetry loss. So a trajectory that
carries the boot fate "interrupted by a restart" is reported by the verifier as
**inconclusive, not failed** when the only missing evidence is batched-kind
events (panel finding; phase 1 item).

### 3.4 How the classification is enforced in code

Five layers, so that a mistake needs several independent failures.

1. **One closed table with a forcing test.** A new `types.EventKind` method
   `Durability()` returns `DurabilityBatched` or `DurabilityAcknowledged` from an
   explicit map. A test parses `internal/types/task.go` with `go/parser`,
   collects every `EventKind` constant, and fails if one has no map entry. A new
   kind cannot ship unclassified, and the default for an unknown string is
   acknowledged.
2. **The kind decides, in one place.** `Store.AppendEvent` tests
   `rec.Kind.Durability()` and the flag. Callers have no parameter to opt in. The
   email evidence handler (`agentcore/api.go:437-444`) is already a two-kind
   allowlist.
3. **An unforgeable input type.** The sealer takes only a `telemetryRecord`, an
   unexported-field type built by `newTelemetryRecord(rec)`, which returns an
   error unless the kind is batched. No code outside `internal/store` can put a
   non-telemetry op in the queue.
4. **Re-check at seal time.** Before building the batch the sealer re-decodes
   every op and asserts: op kind is `object`, object kind is `choir.event`, the
   event kind is batched. A violation refuses the whole batch and falls back to
   acknowledged singles; it increments a counter named
   `telemetry_class_violation`.
5. **The tape says what it is.** A sealed batch carries
   `authority_ref: "authority:vm-local-projection-telemetry"` (today every
   projection batch uses `authority:vm-local-projection`,
   `projection_tape.go:126`). `AuthorityRef` is free-form and only required to
   be non-empty (`computerevent/event.go:167`), and nothing in the reducer keys
   on it **[V]**. A tape audit (section 9) then asserts, over the real staging
   chain, that every batch with the telemetry authority contains only batched
   kinds, and that no batch with the ordinary authority was produced by the
   sealer. The marker also lets later retention work (forward fix §6.4) treat
   telemetry batches separately.

An exemption from the ordering barrier (section 4.3) uses the same discipline:
it is a named context marker, exactly two call sites may set it (the sealer's own
flush and `AppendRunMemoryEntry`), and a source-scan test fails if a third
appears.

## 4. Window form

### 4.1 What the measurements say

Measured arrival windows, last 12 h on 10-19 **[M]** (host record):

| Window | Batches | Events per batch |
|---|---:|---:|
| 0.25 s | 32,479 | 2.95 |
| 1 s | 14,411 | 6.64 |
| 2 s | 9,038 | 10.6 |
| 5 s | 4,322 | 22.2 |

These count all events and only active windows. The telemetry-only rate is the
telemetry share of each. They also assume no early seals. Interleaved
acknowledged writes seal early under the ordering rule (4.3), so the realised
`k` is lower. **[H]**, settled by phase 0 and phase 2 counters.

### 4.2 The options

| Option | Loss window | Realised k | Verdict |
|---|---|---|---|
| Count-based only (every N records) | unbounded when quiet: the last N-1 records sit forever | high at peak, 1 when quiet | **Reject.** A quiet run's tail would never reach the tape |
| Size-based only | same | same | **Reject as sole trigger.** Keep as a cap |
| Fixed time 250 ms | 250 ms | about 3 | Safe but gives little: the telemetry class goes from 100% to about 33% of itself |
| Fixed time 1 s | 1 s | about 6.6 | Good k. 1 s of trace lines at risk |
| Fixed time 2 s | 2 s | about 10.6 | Marginal gain over 1 s, doubles the loss |
| Sliding idle timer ("seal after X ms of quiet") | **unbounded under steady load** | high | **Reject.** A busy run never goes quiet, so the loss window has no bound. The owner's ruling needs a bound |
| Seal at run boundaries only | the whole run | very high | **Reject.** A crash loses a run's whole trace, and it breaks tails |
| **Hybrid: deadline from first record, plus caps, plus barriers** | at most `W` | 3 to 6 **[H]** | **Recommended** |

### 4.3 Recommended: hybrid, deadline measured from the first unsealed record

A batch seals at the first of these:

1. **Deadline.** `W` after the first record entered the empty queue. Not reset
   by later records. Default **1 s**. `W` is a runtime-configuration value
   clamped to 250 ms to 2 s, so the owner can move it without a deploy.
2. **Count.** 64 ops.
3. **Bytes.** 256 KiB of marshalled ops. One op larger than the cap seals alone.
4. **Barrier (ordering).** Any acknowledged write that reaches the appender,
   except the one exemption below, first seals and waits for the pending batch.
   This is the rule that makes loss a tail: the tape never holds a semantic event
   from a run ahead of an earlier telemetry record of that run. Implemented as
   one hook in the appender, before it takes its mutex, so it covers
   `appendOps` and the direct `AppendNew*` callers (lifecycle receipts,
   platform update, materialization) alike. It must sit on **every** entry
   point: `Append`, `AppendNew`, `AppendNewPayload`, `AppendNewPrivatePayload`
   and `AppendNewPayloadSet` (`appender.go:307-476`). A source-scan test lists the
   exported methods that take `a.mu` and fails if one lacks the hook. A missed
   entry point puts a semantic event ahead of earlier telemetry.
5. **Barrier (read).** `Consistent` event reads (3.3).
6. **Shutdown and planned restart.** Section 4.5.

**The one exemption.** `AppendRunMemoryEntry` does not barrier. Run memory is 31%
of events **[M]** and sits inside the same hot loop as the telemetry. If it
barriered, the telemetry would seal at nearly every step and `k` would collapse
to about 1.4 **[H]**. The cost of the exemption: a crash can leave a run-memory
entry on the tape whose neighbouring trace lines were lost. That is a lost
telemetry tail, which the ruling allows; run memory itself is never lost. The
default for any new acknowledged writer is "barrier", so a forgotten exemption
lowers `k` and breaks nothing.

**Why 1 s rather than 250 ms.** The ruling accepts a window of loss in exchange
for volume; at 250 ms the class only drops to about a third of itself and the
total only by about 1.6x (section 7). At 1 s it drops to about a sixth before
barriers. The owner can lower `W` at runtime if 1 s of lost trace proves painful.

**Backpressure, never silent drop.** Pending plus in-flight ops are capped at
256 (a hard invariant, since the reseed gap in 6.1 depends on it). At the cap,
`AppendEvent` stops returning at once and flushes synchronously, which is
today's behaviour and latency. A platform outage therefore degrades to the
present blocking write; it does not grow memory and does not drop records.

**Lock discipline.** The record is enqueued **inside** the `eventMu` critical
section, after `seq` and `stream_seq` are allocated (`store.go:1937-2012`), so
enqueue order equals allocation order. Any synchronous flush (the cap, a
barrier) runs **after** `eventMu` is released, as today's tape path already
releases it before the write (`store.go:2009-2011`). A flush under `eventMu`
would stall all sequence allocation computer-wide and create the lock order
`eventMu -> appender mutex` (panel finding).

### 4.4 Reads

Two classes, default safe.

- **`Consistent`** (default for every `ListEvents*` call): seal and wait if the
  queue is non-empty, then read. Cost: one flush, about one append.
- **`Relaxed`** (explicit, only for the polled UI endpoints: run events page
  `agentcore/api_run_events.go:72`, owner events `textureowner/texture.go:1965`,
  costs `apihandler/costs.go:73`): read the tape-derived state without sealing.
  It lags by at most `W`. Its guarantee is exactly this: **no acknowledged
  write lands on the tape before telemetry that was enqueued earlier**, and the
  sealer enqueues in `seq` order under `eventMu`, so a poller does not see a
  later acknowledged event while missing an earlier batched one. It is not a
  stronger prefix claim: acknowledged-versus-acknowledged `seq` races exist
  today because `eventMu` is released before the write (`store.go:2009-2011`).

Phase 1 ships everything as `Consistent` (simple, correct). Phase 3 moves the
three polled endpoints to `Relaxed` only if the phase 2 counters show reads
costing `k`.

### 4.5 Shutdown, planned restart, crash

- **Graceful shutdown.** The sealer drains on the runtime's shutdown path
  (`internal/server/server.go:236-246` handles SIGTERM and calls
  `Shutdown`), with a deadline.
- **Planned restart.** `markPlannedRestart` (`agentcore/planned_restart.go:294-314`)
  first puts the sealer in **synchronous mode** (new telemetry takes the
  acknowledged path), then drains with a deadline equal to the shutdown
  deadline, then writes the marker. Marker present therefore normally implies the
  queue was empty at the moment of restart, so the one resume a planned restart
  allows (`agentcore/runtime.go:849-852`) resumes from a tape with no telemetry
  gap. It adds no boot-time resumer and no new marker. If the drain fails or
  times out, the marker is **still written** (the planned-restart contract is
  about work, not telemetry), the failure is logged and counted, and the resume
  record carries the same "last window of this run's trace may be missing"
  sentence as a crash boot, because a markerful boot would otherwise skip it
  (`runtime.go:849-858`; panel finding).
- **Crash (no marker).** The queue is lost: at most `W` of telemetry, plus at
  most the one batch in flight. The boot rule already closes every pre-boot run
  with the fate "interrupted by a restart" (`runtime.go:849-858`). Nothing is
  resumed, nothing is re-dispatched, so the queue needs no persistence and
  nothing replays it.
- **VM stop by the host without a signal.** Treated as a crash.

## 5. What "lost on crash" means: batch shape and chain integrity

### 5.1 Decision: one tape event holding N ops

Two shapes were possible.

| | One event, N `choir.event` ops | N events sealed together |
|---|---|---|
| Files per telemetry record | 4/N | 4 (each event still pins a payload, an event and two receipts) |
| Chain | one link; digest, previous head, request commitment, two pin receipts and head receipt computed exactly as today | N links, so nothing is saved, and "sealed together" has no object that holds the grouping |
| Replay | the existing ordered multi-op replay (`store/project.go:45-62`; forward fix §2) | N replays |
| Format change | none; `ProjectionBatch.Ops` is already a list with no cap (`projection_batch.go:159-215`) | none, but pointless |
| Restore granularity | whole batch is the restore target (`projection_batch.go:63-64`) | per event |

**Choose one event with N ops.** It is an ordinary
`projection_batch_recorded` event; the request commitment binds one payload
receipt, as today.

**Chain integrity.** Nothing in the chain's hash structure depends on how many
ops the payload holds. The payload digest commits to the bytes; the event
commits to the payload digest; the head receipt commits to the event. A batch
that loses a race with a crash is simply an event that was never appended; the
chain before it is intact and no head is rewound.

**Replay and witness.** Replay applies the ops in order inside one SQL
transaction, the same as any multi-op `PutBatch`. The content witness is a
function of table contents, not of how events partition the ops, so live and
replay still agree **[H]**: verify by the forward fix's staging completeness
probe with the flag on. The op order inside a batch is `seq` order.

**Restore.** Intermediate heads inside a batch are not restore targets. With
batches of up to 64 telemetry ops, a restore target granularity is coarser by
up to `W` of telemetry only. No restore goes to a mid-batch point today either.

### 5.2 What a reader sees after a crash

- **Tape and replay readers.** The tape is a valid chain ending at the last
  sealed event. Replay needs no knowledge that telemetry was lost.
- **Trace and run readers.** For each run: its trace ends at the last sealed
  telemetry record, followed (after boot) by the run's "interrupted by a
  restart" terminal event. By the ordering barrier, no acknowledged event of
  the run precedes a lost telemetry record, so the run's event list is a
  prefix of what the process emitted (run memory aside, 4.3).
- **The loss is visible.** Per-run `seq` and global `stream_seq` have a gap
  where the lost records were. Phase 1 also adds a sentence to the boot
  interruption record, so a reader is told the last window of a run's trace may be
  missing, rather than inferring it from a gap.

### 5.3 What a lost record may never mean

Telemetry is lost; its effects are not. The effect of any tool step (a file
written, a document revision, a commitment) is on the tape through its own
acknowledged write. A lost `tool.result` loses the trace line only.

## 6. Failure modes (write the tests before the code)

### 6.1 Sequence numbers can be reused (blocking; three cases, one rule)

`stream_seq` is seeded from the newest 500 stored events
(`store/store.go:1990-2005`) and `seq` from the run's stored events
(`:1958-1978`). Today every number handed out is durable before it is published
to the bus (`emitEvent` publishes after `AppendEvent`, `runtime.go:4570-4578`),
so a client can only have seen numbers that are stored. With the sealer a client
may see numbers that never reached the tape. Three ways a number is then issued
twice, the third found by the panel:

1. **Crash reseed.** Events 101 to 105 are published and lost; the next process
   seeds at the stored 100 and issues 101 to 105 again. A client with a cursor at
   105 misses the new events, and per-run cursors (`api_run_events.go:72`) see
   numbers reused.
2. **Crash then crash again** (panel finding). Boot 1 finds nothing to close, so
   nothing durable moves the stored maximum; it publishes events that stay
   queued; a second crash loses them; boot 2 seeds from the same stored maximum.
   A fixed gap added at each boot would hand out the same numbers twice.
3. **In-process reseed under a live queue** (panel finding, verified).
   `eventSeqs` is dropped wholesale at 100,000 keys (`store.go:1977-1979`), and
   the `AppendEventOG` error path deletes a run's mark when it still equals the
   failed number (`store.go:2019-2025`). Both reseed from **stored** events
   only. With a queue holding higher, not yet stored numbers for that run, the
   next append re-issues one of them: a duplicate `(run, seq)` on the tape, and a
   cursor consumer (`api_run_events.go` `NextAfter`, `live_ws.go:79`) treats
   `seq <= cursor` as seen, so the later event becomes invisible.

**One rule, three parts.**

- **Floor from the queue.** With the sealer enabled, every reseed takes
  `max(storedMax, highest number allocated and not yet stored for that run or for
  the stream)`. The sealer exposes that floor to the allocator. The cache drop and
  the error path may no longer lower a mark below a number the queue holds. An
  asynchronous seal failure never forgets a mark: it leaves a gap, never a
  duplicate.
- **Crash reseed gap.** On the first batched append for a run whose stored events
  predate this process, start `seq` at `storedMax + G`. `G` is the pending-ops cap
  plus the largest batch (256 + 64 = 320, rounded to **512**), so a gap always
  exceeds anything the previous process could have lost.
- **Boot watermark.** Before the first batched enqueue after a boot, write one
  acknowledged record at the reseeded `stream_seq` (an `runtime.health`-kind
  event with a boot payload, which is an acknowledged kind and ownerless). That
  makes the new floor durable, so a second crash seeds above it (case 2). It is
  one acknowledged write per boot that carries telemetry. Per-run `seq` needs no
  second record: a crash boot closes every pre-boot run with an acknowledged event
  (`runtime.go:854`, `closePreBootRunsAfterCrash`), which stores a number above
  the lost range, and a closed run takes no further events.

Gaps are already legal ("a gap, never a duplicate", `store.go:2019-2024`). The
alternative, a durable `stream_seq` lease written every 1,024 numbers, costs a
steady stream of extra acknowledged writes; the watermark costs one per boot.

Tests, each first: kill -9 with a full queue, reboot, assert no duplicate
`stream_seq` and no duplicate per-run `seq`; kill, reboot, publish, kill, reboot
(case 2); fill the queue, force the 100,000-key cache drop and an async seal
failure, append, assert no duplicate (case 3).

### 6.2 The table

1. **Crash with pending telemetry.** Expect: loss of at most `W` plus one
   in-flight batch; the chain is valid; no event is acknowledged-then-lost
   except telemetry; the boot rule closes the runs (4.5); sequence rule 6.1 holds.
2. **Seal failure before pin** (dry-run refusal at `appender.go:364-373`).
   Expect: split to individual acknowledged writes via the unchanged direct path
   (nothing is pinned yet, so this is safe; forward fix §2 item 2); a poison
   record is dropped with a counter and one warning, not retried forever.
3. **Seal failure after pin** (CAS outcome unknown). Expect: **no retry**. The
   `choir.event` ids are content-hashed, so a retry would be an upsert and not a
   duplicate object, but it would still be a second tape event; the unchanged
   `RecoverPrepared` settles the first at boot (`appender.go:699-753`). Count
   and log the loss.
4. **Platform outage.** Expect: at the cap, backpressure (4.3), identical to
   today; memory bounded.
5. **Barrier deadlock.** The barrier waits for the flush, which needs the
   appender mutex. `HoldAppends` holds that mutex while running its function
   (`appender.go:164-172`), so nothing inside `HoldAppends` may call **any
   sealer-aware path**: not an append, and not a `Consistent` `ListEvents*`
   read either. Today neither deadlocks because neither exists inside it; both
   `HoldAppends` callers (`agentcore/replay_completeness.go:274`,
   `checkpoint_restore_bindings.go:84`) touch only the head, the state extractor
   and the platform publish. The new hazard is the `Consistent` read, which did
   not wait on anything before. Test: a lock-order test over `eventMu`,
   `trajectoryMu`, `runMemoryMu`, the sealer mutex and the appender mutex; the
   barrier runs before the appender mutex is taken; the sealer never takes
   `trajectoryMu`, `runMemoryMu` or `eventMu`; a synchronous flush never runs
   under `eventMu` (4.3). A lifecycle write that barriers while holding
   `trajectoryMu` pays one flush round trip and no more, since the flush never
   takes that lock.
6. **Timer and barrier race.** One flusher mutex; a flush snapshots the whole
   queue atomically; a barrier waits for "flush generation >= generation at
   call". No lost wakeup, no double seal.
7. **Cancelled context.** The caller of `AppendEvent` returns at enqueue, so its
   context cannot cancel the flush. The flush runs under
   `context.WithoutCancel` with a fixed timeout, as the forward fix specifies
   for A3.
8. **Class violation.** Section 3.4 layer 4. Expect: refused batch, singles,
   counter.
9. **Flag off.** Expect: byte-identical tape events for a scripted workload
   (golden test), as for A3.
10. **Flag turned off with pending records.** Expect: drain, then the
    acknowledged path.
11. **Large payloads.** 64 ops of up to 5.5 KB average is about 350 KB; the cap
    is 256 KiB, so sealed payloads stay far below the replay page cap (8 MiB,
    `EventReplayMaxResponseBytes`) and the payload cap (64 MiB).
12. **Bus publishes before durability.** A client may see a record that a
    crash then loses. That is the ruled trade; only the sequence reuse (6.1) is a
    correctness hazard.

## 7. Expected reduction

Let `t` be the share of tape events that are batched telemetry and `k` the
realised ops per sealed batch. The remaining event fraction is
`1 - t + t/k`. `t` is **not measured** (58% is an upper bound, all `choir.event`
kinds); `k` is bounded above by the table in 4.1 and below by the barrier rate.
**[H]** throughout; phase 0 and phase 2 counters settle both.

| `t` \ `k` | 3 | 5 | 8 |
|---|---:|---:|---:|
| 0.40 | 0.73 (1.4x) | 0.68 (1.5x) | 0.65 (1.5x) |
| 0.50 | 0.67 (1.5x) | 0.60 (1.7x) | 0.56 (1.8x) |
| 0.58 | 0.61 (1.6x) | 0.54 (1.9x) | 0.49 (2.0x) |

Planning figure: **0.55 to 0.65 of today's events**, or 1.5x to 1.8x fewer.

At 112k events a day (the rate since 10-17 **[M]**) and 4.02 host files per
event:

| | Events/day | Host files/day (no segments) |
|---|---:|---:|
| Today | 112k | 0.45M |
| Batched, planning range | 62k to 73k | 0.25M to 0.29M |
| Same at the 10-15 peak (358k events) | 197k to 233k | 0.79M to 0.94M |

Other effects, per sealed batch instead of per record: one platform
`computer_event_append_receipts` row and head update; one `computer_event_index`
insert (about 3 KB of `event_json` plus head-receipt JSON per row **[M]**: 542 MB
plus 523 MB for 359,911 rows, volume design §3); one prepare and one finalize
guest transaction; fewer base publications, since the cadence counts events
(`platform/projection_jobs.go:90-96`, forward fix §2 item 4).

Not reduced: `og_objects` insert count (one per record), payload bytes (about 5.5
KB per record), and the run-memory share (31%, untouched). Guest history
hypothesis: roughly a fifth less, because a batched telemetry record saves one of
its two hash-keyed inserts (the index row) for all but one record per batch.
**[H]**, needs the forward fix's stage-0 offline measurement.

**This is a volume trim, not the host fix.** Segments (A2) take host files per
event to near zero whatever the event rate is; batching halves what segments
would otherwise have to carry and halves the per-event round trips and database
rows. It does not by itself stop the 111 GiB class of growth.

## 8. Interaction with the segment store (A1, A2) and group commit (A3)

- **A1 (read library).** Independent. A sealed batch is an ordinary event and
  payload, so every A1 read site works unchanged. Land A1 before enabling
  batching in production if both are queued, so that rollback of either never
  has to reason about both.
- **A2 (write into segments).** Composes. One sealed batch is one event, four
  records and one fsync at the event pin (forward fix §4.2). Fewer batches means
  fewer fsyncs, and a larger payload per record (up to 256 KiB, against a 64 MiB
  segment), which segments handle without change. The telemetry authority marker
  lets a later segment or retention policy treat telemetry batches separately.
- **A3 (group commit).** Batching subsumes A3's "windowed telemetry" form
  (volume design (a2)) and the forward fix §3.5's "non-blocking telemetry class",
  which is the form it told the owner to decide. A3 then stays the lingerless
  leader/follower queue for acknowledged writes. The sealer's flush is a single
  caller of the same direct `appendOps` path; it does not wait on A3. Its
  `ErrBatchRefused` sentinel (A3, `appender.go:368`, `:371`) is reused by
  failure mode 2.
- **Prerequisite for run-memory batching, deliberately not included.** Per-run
  `runMemoryMu` and in-memory `seq` allocation (forward fix §5, "later"). Run
  memory is not telemetry (3.2), so this is a separate decision.

## 9. Proof

CLAUDE.md prefers E2E and requires an artifact. Tests for the failure modes in
section 6 are written first, per CLAUDE.md "If you must test a system in
isolation, first write down all the ways it could fail, then write the code".

E2E on a staging computer with the flag on, driven through the product path:

1. Run a real research or Texture run. Assert the tape gained batches with the
   telemetry authority, each with more than one op, and that every op is a
   batched kind (the tape audit; read through the existing replay page endpoint,
   no ssh).
2. Assert no batch with the ordinary authority was produced by the sealer, and no
   telemetry-authority batch contains an op that is not a batched `choir.event`.
3. Ordering: for each run, no acknowledged event's tape position precedes an
   earlier-`seq` telemetry record. The run-memory exemption is the one exception,
   and the audit says so: tape position between a run-memory event and the
   telemetry around it is not ordered.
4. **Kill drill.** Kill -9 the guest with a full queue. Assert: boot closes the run
   "interrupted by a restart"; no duplicate `stream_seq` or `seq`; the loss is no
   larger than `W` plus one batch; the state-completeness probe passes.
5. **Planned-restart drill.** With the marker written and pending records,
   assert the queue drains first and the resumed run has no telemetry gap.
6. Acceptance synthesis on a trajectory with pending telemetry returns the same
   verdict as with the flag off (cite barrier). The verifier on a
   restart-interrupted trajectory reports inconclusive, not failed.
7. The three sequence tests of 6.1, and the entry-point and exemption source-scan
   tests of 3.4 and 4.3.
8. Counters on the diagnostics endpoint: records enqueued, batches sealed, ops per
   batch, seal reason (deadline, count, bytes, barrier, read, shutdown), barrier
   waits, `telemetry_class_violation`, dropped records, queue depth, seal latency.

The artifact is a JSON report under `tests/` plus the Node B files-per-event and
events-per-day counters before and after.

## 10. Mutation class, rollback, flag

**Red.** Protected surfaces: the Texture and evidence readers (3.3), the tape
write path, replay, sequence allocation, run acceptance, restart handling. Phase 0
is orange (classification table, counters; no behaviour change). Full ceremony
applies to the later phases.

**Flag.** `CHOIR_TELEMETRY_BATCH=off|on`, default **off**, read through runtime
configuration, with `W` as a second value. Off means byte-identical behaviour to
today (golden test). No read path depends on the flag.

**Rollback.** Set the flag to off; the sealer drains and the acknowledged path
resumes. Tape batches already written are ordinary events and stay valid and
readable by an older binary, because no payload, reducer or witness format
changed. Unlike the segment store, a binary rollback needs no explode tool.
Git revert of the slice is also safe. The one residue is sequence gaps, which are
legal.

**Heresy delta.** Discovered: the `rlm_inbox_cursor` and similar entries make run
memory functional state, not telemetry (3.2); `tool.result` is read as evidence
(3.3); sequence reuse after a crash would be a new hazard of any queued
publisher (6.1). Introduced: none yet (design). Repaired: none yet.

## 11. Standing questions

1. **Settled by:** owner ruling 2026-10-14 §27 item 18 settles that telemetry may
   batch and may be lost up to a window. The window form is this doc plus the
   panel. The classification and the reseed gap are proposals.
2. **Topology:** no new store; the sealer is an in-memory queue in the guest.
3. **Deletion citers:** none; nothing is deleted.
4. **Production consumers:** the readers in 3.3 and the polled endpoints in 4.4.
5. **Single authority:** the platform head and the append receipt; the queue is
   a latency buffer, not authority.
6. **Artifact-verified success:** the section 9 report.
7. **Fate-sharing:** unchanged; the queue lives and dies with the guest process.
8. **Restart durability:** drain on planned restart; loss on crash is the ruled
   trade; no resumer added.
9. **No-SSH operability:** counters on the diagnostics endpoint; flag and `W` in
   runtime configuration.
10. **Registry hygiene:** one throughline goal file when executed; this doc is the
    design record.

## 12. Phased plan

| Phase | Class | Content | Gate |
|---|---|---|---|
| 0 | orange | `Durability()` table plus the parser-based forcing test; per-kind event counters and the sealer counters on the A0 diagnostics; `Consistent`/`Relaxed` list API with `Relaxed` equal to `Consistent` | per-kind volume known on staging; `t` measured; no behaviour change |
| 1 | red, flag off | sealer in `internal/store`, appender barrier hook on all five entry points, sequence floor plus reseed gap plus boot watermark (6.1), drain on shutdown and planned restart, boot and resume interruption sentence, verifier "inconclusive" rule; the section 6 tests first | tests green; flag off golden test; staging E2E steps 1 to 6 with the flag on for one computer |
| 2 | red | enable on staging computers; measure `k`, latency, loss drill | realised `k` and reduction match section 7 or the window is revisited |
| 3 | red | `Relaxed` reads for the three polled endpoints if reads cost `k`; revisit `W` | counters |
| 4 | red | default on after a soak; A2 segments compose | soak receipt |

Order against the forward fix: phase 0 can run now (A0 landed). Phase 1 is
independent of A1 to A3, but see section 8 on A1 first.

## 13. Not verified

- The per-kind split of the 58%. Only the all-`choir.event` share is measured.
- Realised `k` with the ordering barrier. Section 4.1 measures arrivals, not
  seal reasons.
- That the content witness is independent of event partitioning. It follows from
  the forward fix's reading of replay, but no probe has been run with a
  multi-op telemetry batch on a real chain.
- That nothing outside the cited readers reads `choir.event` as evidence.
  Phase 0 should add a source-scan test listing every `ListEvents*` call site so
  a new one must be classified `Consistent` or `Relaxed`.
- That run-memory entries other than `rlm_inbox_cursor` are all functional. The
  conclusion (never batch) does not depend on it.
- Whether `loop.progress` cost lines feed anything other than the estimate
  endpoint. `llmcost.ExtractCostEntries` has one caller
  (`apihandler/costs.go:95`).

## Panel review

Panel, per the owner's rule (Devin plus omp-gemini38 only): `devin` (account
default model) and `omp-gemini38` (`google-antigravity/gemini-3.8-flash`, thinking
high). Convergent mode, `--keep-going`, both ran to exit 0 (325 s and 527 s).
Prompt: this document as it stood before this section, plus the question "pick
the window form and confirm the never-batch set; name blocking issues". Raw
outputs: `.agentic-consensus/telemetry-batching-2026-10-20/run1/`
(gitignored, not durable evidence). Both panelists read the code; I re-checked
the load-bearing claim (the sequence cache drop, `store.go:1977-1979`, and the
error-path mark deletion, `:2019-2025`) myself.

**Verdict.** Window form endorsed by both, unchanged. Never-batch set confirmed
by both: all 42 runtime kinds accounted for (10 batch, 32 never, default never),
nothing to move in or out. One **blocking** issue from Devin, which I confirmed
and fixed in the design; Gemini found no blocking issue but raised a related
sequence case that the first draft's fix did not cover.

**Agreements.**

- Hybrid window: deadline from the first unsealed record (not sliding), W = 1 s
  default with a 250 ms to 2 s clamp, 64 ops / 256 KiB caps, ordering barrier
  with the run-memory exemption, consistent reads, drain on planned restart. Both
  independently rejected the sliding idle timer (unbounded loss under steady
  load), count-only and size-only (a quiet tail never drains), and run-boundary
  sealing.
- `tool.invoked` and `tool.result` stay in the batch set; the cite barrier is the
  right mitigation. `loop.progress` stays (the cost endpoint is an estimate, one
  caller). The 32 never-batch kinds are correct. The run-memory exclusion is
  correct and load-bearing.
- One tape event holding N ops is correct: multi-op `projection_batch_recorded`
  events already exist through conditional batches, and `AuthorityRef` is only
  required to be non-empty (`event.go:167`).
- The 256 KiB cap clears the 8 MiB replay cap: replay pages carry event
  envelopes, not payload bytes.

**Dissent.** None on the window or the set. Severity differed on sequence reuse:
Devin called the in-process case blocking, Gemini called the crash-crash case
should-fix. I took the stricter reading; they are one root cause.

**Changes I made.**

1. **Blocking, Devin, confirmed locally.** In-process sequence reuse: the
   100,000-key cache drop and the error-path mark deletion reseed from stored
   events only, so a live queue can hold numbers the reseed re-issues. 6.1 now has
   a rule with a queue floor, and an asynchronous seal failure never forgets a mark.
2. Gemini, crash then crash again: a fixed reseed gap is not idempotent across two
   crashes with no durable write between. 6.1 adds the boot watermark (one
   acknowledged record per boot that carries telemetry) and sets `G` to 512.
3. Both: the flush must not run under `eventMu`. 4.3 now states the lock
   discipline (enqueue inside `eventMu`, every synchronous flush outside) and 6.2
   item 5 adds it to the lock-order test.
4. Devin: the `HoldAppends` prohibition now covers `Consistent` reads, not only
   appends (6.2 item 5).
5. Devin: the barrier hook is required on all five appender entry points, with a
   source-scan test (4.3).
6. Devin: the verifier reports a restart-interrupted trajectory as inconclusive,
   not failed (3.3); added `agentcore/api.go:432` to the reader list.
7. Devin: planned-restart drain failure is no longer silent: synchronous mode,
   bounded drain, then the marker, and a failed drain is recorded on the resume
   record (4.5).
8. Devin: "prefix-closed" overstated; 4.4 now claims only that no acknowledged
   write lands before earlier-enqueued telemetry.
9. Gemini: the audit states that tape position between run-memory events and
   telemetry is not ordered (9.3).

**Not changed.** Gemini's suggestion to publish to the bus only after seal was not
taken: it would delay the live view by `W` for no benefit the ruling asks for. The
residual client-side risk, a client that saw events the tape never got, is
recorded in 6.2 item 12 and must be tested in the kill drill. Gemini's own main
residual risk (the UI must tolerate sequence discontinuities on reconnect to an
interrupted run) is accepted and goes into phase 1's E2E.

**Confidence.** Panel: high on the verdict. Mine: high on the classification and
shape; medium on realised `k` until phase 0 counters exist.

## Implementation plan and failure modes (2026-10-20, written before any code)

Scope of this implementation: phase 0 and phase 1 of section 12, flag
`CHOIR_TELEMETRY_BATCH` default off. Phases 2 to 4 (enabling anywhere,
`Relaxed` reads, default on) are not part of it. Mutation class: phase 0
orange; phase 1 **red** (protected surfaces: tape write path, evidence readers,
sequence allocation, restart handling; evidence class: unit and in-process
tape tests with a real appender over the real projection store, not staging;
rollback: flag off drains and resumes the acknowledged path, or revert the
commit; no payload, reducer or witness format changes). Heresy delta:
discovered, see "Found while reading the code" below; introduced, none
intended (the failure-mode tests are what would find one); repaired, none.

### Found while reading the code (feeds the plan)

1. **The projection store's `Prepare`, `Finalize` and `DiscardPrepared` take
   `Store.eventMu`** (`internal/store/computer_events.go:46,132,261`), and the
   appender calls them while holding its own mutex. So the lock order is already
   appender mutex then `eventMu`. A flush under `eventMu` would deadlock, not
   merely stall. The design's "flush outside `eventMu`" is therefore a hard
   requirement, and the backpressure test (P1-F11) is also the deadlock test.
2. **A crash-boot close event is the first append of its run in the new
   process**, and it is an acknowledged kind. Design 6.1 says the close event
   "stores a number above the lost range"; it does not unless the reseed gap
   applies to it too (stored 100, lost 101 to 105, close event gets 101). So the
   gap is applied to **every** first seed of a run's `seq` while the sealer
   exists, whatever the kind of the appending event, not only batched ones.
3. **No diagnostics counters from forward-fix A0 exist yet** (only the
   `livePayloads` leak fix landed). The counters are added to the existing
   guest `GET /api/runtime/observability` response, next to `EngineMutex`
   (`store.EngineMutexStats()` is the precedent), not to the replay health gate.
4. **`ErrBatchRefused` (A3) does not exist.** The appender wraps the decode and
   dry-run refusals with `%w` on a new sentinel so the sealer can tell a safe
   pre-pin refusal from an unknown-outcome failure. Messages are unchanged.
5. **`server.Shutdown` has no hook and `Runtime.Stop` is not called on SIGTERM**
   in `autoputer.Run`. Drain on shutdown therefore needs a small
   `AddShutdownHook` on `internal/server` (runs after the HTTP server stops
   accepting, with the shutdown deadline).
6. **Flag source.** A computer-owned boolean setting already exists
   (`KnownComputerSettings`, written through the settings API,
   `internal/agentcore/computer_settings.go`). The flag is on when the env value
   `CHOIR_TELEMETRY_BATCH=on` or the setting `telemetry.batch` is true, so a test
   computer is switched on through the product path without patching the host.
   `W` is `CHOIR_TELEMETRY_BATCH_WINDOW_MS`, clamped to 250 to 2000, env only for
   now (settings are boolean). Both are residual items, listed in the report.

### Decisions where the design left a choice

- Reads: a context marker `store.WithReadConsistency(ctx, ReadRelaxed)`; absent
  means `Consistent`; in this implementation `Relaxed` behaves as `Consistent`
  (phase 0 says so). Every `ListEvents*` method seals first.
- Seal failure classes: an error wrapping `ErrBatchRefused` (decode or dry-run,
  nothing pinned) splits to individual writes through the same direct path; a
  single record that is still refused is dropped and counted as poison; **any
  other error is an unknown outcome and is not retried** (design 6.2 item 3),
  counted and logged. The unknown-outcome rule is also applied to a pin failure
  (outage), which keeps an outage from multiplying work.
- Capacity: pending plus in-flight plus reserved is capped at 256. A slot is
  reserved before `eventMu`; at the cap the caller flushes (outside `eventMu`)
  and retries. Records that fail to seal free their slots (dropped and counted).
- Boot watermark: the first batched append after boot adds `G`=512 to
  `maxStreamSeq` under `eventMu`, then writes one acknowledged `runtime.health`
  event (payload `{"telemetry_boot_watermark":true}`) before anything is
  queued. If that write fails, batched appends keep taking the acknowledged path
  until it succeeds.
- The sealer exists only after the first time batching is configured on in this
  process. A process that never turned it on runs the old code, plus counters.
  Known residual: a crash with a non-empty queue and a reboot with batching off
  can reuse numbers; the flag is written by owner acts that drain first, so only
  an env change coinciding with a crash exposes it.

### Phase 0 failure modes (orange)

| # | Failure | Test, written first |
|---|---|---|
| P0-1 | A new `types.EventKind` ships unclassified | `TestEveryEventKindIsClassified` parses `internal/types` with `go/parser`; every constant of type `EventKind` needs a map entry and the map has no entry without a constant |
| P0-2 | An unknown or empty kind batches | `TestDurabilityUnknownKindIsAcknowledged` |
| P0-3 | The table drifts from the design's ten kinds | `TestDurabilityTableIsTheDesignedTen` |
| P0-4 | A new `ListEvents*` call site is added without a read class | `TestListEventsCallSitesAreClassified` scans every non-test `.go` file for `ListEvents*` calls outside `internal/store` against a table of (file, method, class) |
| P0-5 | Counters are wrong or racy | `TestTelemetryStatsCountEventsAndBatchSizes`: per-kind counts, batchable share, ops-per-tape-event histogram, under `-race` with concurrent appends |
| P0-6 | Counters change behaviour | covered by the flag-off golden test (P1-F8) |

### Phase 1 failure modes (red), each with its test

| # | Failure | Test |
|---|---|---|
| P1-F1 | A semantic kind is batched | type level: `TestTelemetryRecordIsUnforgeable` (go/parser: `telemetryRecord` has no exported field, and a composite literal of it appears only in `newTelemetryRecord`); runtime: `TestSemanticKindsNeverEnterTheQueue` (every `Acknowledged` kind: constructor refuses it, and with the flag on `AppendEvent` leaves queue depth 0 and writes an ordinary-authority event) |
| P1-F2 | A forged record reaches the sealer | `TestSealTimeRecheckRefusesViolation`: a test-forged record with an acknowledged kind makes the whole batch fall back to singles, `telemetry_class_violation` increments, no telemetry-authority batch holds it |
| P1-F3 | A missed appender entry point puts a semantic event ahead of earlier telemetry | `TestEveryAppenderEntryPointRunsTheBarrier` (go/parser: every exported method that locks `a.mu` is in a named non-append allowlist or calls `awaitBarrier` before locking) and a behavioural test that the entry points call the hook once, before the lock |
| P1-F4 | A third caller exempts itself from the barrier | `TestBarrierExemptionHasExactlyTwoCallSites` (source scan: the sealer flush and `AppendRunMemoryEntry`) |
| P1-F5 | Ordering: a semantic event lands ahead of earlier telemetry | `TestOrderingIsPreservedAcrossBarrier`: t1, t2, semantic, t3 gives tape order [t1,t2] then semantic then [t3], read from the recorded CAS stream |
| P1-F6 | A crash loses a semantic event, or leaves a hole | `TestCrashLosesOnlyTheOpenWindow`: abandon the store with a non-empty queue, reopen the same files; every acknowledged event is present, the lost records are the newest telemetry of the run, none in the middle |
| P1-F7 | Sequence reuse (design 6.1, four cases) | `TestNoSeqReuseCrashReseed`, `TestNoSeqReuseCrashThenCrash`, `TestNoSeqReuseInProcessReseedUnderQueue` (cache drop, mark deletion, async seal failure), `TestNoSeqReuseCloseEventAfterCrash` (finding 2), `TestBootWatermarkWrittenOncePerBoot` |
| P1-F8 | Flag off changes the tape | `TestFlagOffTapeIsByteIdentical`: a scripted workload (batched kinds, semantic kinds, run memory, a folded lifecycle event) with fixed ids and timestamps, normalised only for tape event ids and times, compared to a golden file **generated from the unmodified baseline commit before any production code changed** |
| P1-F9 | The window is sliding, or a quiet tail never seals | `TestDeadlineIsFromFirstRecordNotSliding`, `TestQuietTailSealsAtDeadline`, count cap 64, byte cap 256 KiB, single oversized op seals alone |
| P1-F10 | Reads see a tape missing telemetry that is in the queue | `TestConsistentReadSealsPending` for every `ListEvents*` method; acceptance synthesis returns the same verdict as flag off |
| P1-F11 | Barrier deadlock, lock inversion | `TestBackpressureAtCapacityDoesNotDeadlock` (cap path with the real appender, whose `Prepare` takes `eventMu`; a flush under `eventMu` hangs it), `TestSealerNeverTakesStoreLocks` (source scan of the sealer file), `TestConsistentReadUnderTrajectoryLock` |
| P1-F12 | Timer/barrier/count race: lost wakeup, double seal, reordering | `TestConcurrentTelemetryAndBarriers` under `-race`: every record sealed exactly once, per-run order preserved, no duplicate `(run, seq)` |
| P1-F13 | Seal failure handling | `TestSealFailureBeforePinSplitsToSingles`, `TestPoisonRecordDroppedAndCounted`, `TestSealFailureAfterPinIsNotRetried`, `TestPlatformOutageIsBounded` (queue never exceeds 256, nothing grows) |
| P1-F14 | Flag turned off with pending records, or planned restart with a queue | `TestFlagOffDrainsThenAcknowledges`; agentcore: `TestPlannedRestartDrainsBeforeMarker`, `TestDrainFailureStillWritesMarkerAndFlagsResume` |
| P1-F15 | A restart-interrupted trajectory fails verification because trace lines were lost | `TestVerifierReportsInconclusiveForInterruptedTrajectory` |
| P1-F16 | The loss is silent | `TestCrashCloseRecordNamesTheMissingWindow` (sentence only when batching is configured on; flag-off payload unchanged) |
| P1-F17 | Counters not readable without ssh | `TestObservabilityReportsTelemetryBatching` |

### Order of work and commits

1. This section (green, docs only).
2. Golden test and golden file generated on the unmodified baseline (yellow).
3. Phase 0 (orange): `Durability()` plus parser test, counters, read-class API,
   call-site scan.
4. Phase 1 (red): appender barrier hook and sentinel, sealer, `AppendEvent` and
   sequence rule, reads, drain and restart hooks, verifier rule, flag wiring,
   observability. Tests listed above are written before the code they cover.

## As built (phase 0 and phase 1, flag default off; 2026-10-20)

Where the code departs from, or settles a question left open by, the sections
above.

- **Files.** `internal/types/event_durability.go` (the closed table);
  `internal/store/telemetry_sealer.go` (queue, barrier, seal, boot watermark),
  `telemetry_stats.go`, `telemetry_reads.go`; `internal/computerevent/appender.go`
  (`SetAppendBarrier`, `WithBarrierExemption`, `ErrBatchRefused`);
  `internal/agentcore/telemetry_batching.go` (flag, restart rules, records);
  `internal/server` (`AddShutdownHook`); `internal/textureowner` verifier rule.
- **Per-run `seq` after two crashes** relies on the crash boot closing every
  pre-boot run with an acknowledged event (design 6.1), exactly as written. The
  test for case 2 therefore puts the second-boot telemetry on a run created
  after the first crash, not on the run the first crash boot closed. A run with
  no stored acknowledged event at all (never the case in production: every run
  begins with `loop.submitted`) would not get a gap.
- **Stream gap and per-run gap apply at the moment each counter is seeded**,
  whatever the kind of the appending event (finding 2 of the plan), and only
  when the sealer exists then. `stream_seq` also gets the boot watermark.
- **`W` is env only** (`CHOIR_TELEMETRY_BATCH_WINDOW_MS`, clamped 250 ms to
  2 s); the on/off flag is the env value or the computer setting
  `telemetry.batch`. The setting is read at `Runtime.Start`, before the first
  event of the process, using `CHOIR_OWNER_ID`, which the host passes to every
  guest.
- **`Relaxed` reads behave as `Consistent`.** The three polled endpoints are
  marked `Relaxed` (a source-scan test pins every external `ListEvents*` call
  site), so phase 3 is a one-line change in `beforeEventRead`.
- **The Texture verifier rule is a wrapper** (`toolEvidenceError`,
  `ErrVerificationInconclusive`) on the three checks that read `tool.result`.
  `VerifyTextureWorkflow` has no non-test caller in the repo today.
- **Not done, deliberately:** `Relaxed` reads that skip the seal, a sealer
  integration with group commit (A3), any retention policy keyed on the
  telemetry authority, relaxing generic object-graph reads of `choir.event`
  made outside `Store.ListEvents*` (none found in `internal/`).

### Reading the counters (no ssh)

`GET /api/runtime/observability` on the computer (the product path, owner
authenticated) returns `telemetry_batching`:

- Phase 0, with the flag off: `events_appended_by_kind`,
  `events_appended_batchable` and `_acknowledged`, `batchable_share`;
  `tape_events`, `tape_ops`, `ops_per_tape_event` (the baseline batch size,
  1 for every standalone write), `tape_events_by_authority`, `tape_ops_by_kind`.
  **t of section 7** is `events_appended_batchable / tape_events` read over a
  flag-off interval.
- Phase 1, with the flag on: `batching.enabled`, `window_ms`, `queue_depth`,
  `records_enqueued`, `batches_sealed`, `sealed_ops_per_batch` and
  `mean_ops_per_batch` (the realised **k**), `seal_reasons` (deadline, count,
  bytes, barrier, read, shutdown, capacity), `barrier_waits`,
  `telemetry_class_violation`, `records_dropped`, `poison_dropped`,
  `seal_failures`, `unknown_outcome_records_lost`, `split_to_singles`,
  `capacity_flushes`, `boot_watermark_writes`, seal latency mean and max.

### Turning it on for one test computer first

Do not set it fleet-wide. On the test computer, as its owner:
`POST /api/computer-settings {"key":"telemetry.batch","value":true}`. It takes
effect at once and persists across restarts (read at boot); the same call with
`false` drains the queue and returns to acknowledged writes. Alternatively
`CHOIR_TELEMETRY_BATCH=on` in the guest environment turns it on whatever the
setting says. Verify with the observability counters above, then run the
section 9 drills before phase 2.

### Phase 1 staging gate (2026-10-22)

Run by FP0's lane `lane/fp0-batching-e2e` on one fresh test computer,
`computer-71afbf1461dffabcce8eeebe558d03bd` (class `test`), on staging builds
`840e1a701` and `d991907e6` (the batching files are the same in both). The
paper and every other computer were not touched. Evidence, machine-readable:
[`telemetry-batching-phase1-staging-2026-10-22.json`](../evidence/telemetry-batching-phase1-staging-2026-10-22.json).

The tape audit read the chain through corpusd's replay page and payload
routes, read-only, from Node B (no guest access). Runs were driven through the
product API: Texture create, revise and research on four documents.

- **Flag.** `POST /api/computer-settings` took effect at once, without a
  restart (`batching.enabled` true, `window_ms` 1000). Both crash boots came up
  with it on, so the stored setting is read at boot. Setting it back to false
  with two records queued drained them, lost none, and later batched kinds took
  the acknowledged path.
- **Steps 1 and 2: pass.** 211 telemetry batches held 1,089 ops, mean **k =
  5.16** (5.04, 5.36 and 3.38 in the three processes). Every op in them is a
  batched `choir.event`. No ordinary-authority batch in the flag-on window holds
  only batched kinds. 27 batches (13%) hold one op; they are a lone record
  sealed at the deadline or by a barrier, which is legitimate, so step 1's "each
  with more than one op" is read as "batches form".
- **Step 3: pass.** No acknowledged event sits on the tape before a telemetry
  record of the same run with a lower `seq`, and none before one with a lower
  `stream_seq`. Run-memory entries (763 ops in the window) are not ordered
  against telemetry, as section 4.3 says.
- **Step 4: pass.** The kill went through the owner's product route
  `POST /api/computers/<id>/lifecycle/refresh`, the same vmctl refresh as T7
  (a SIGKILL). It was sent when observability showed three records queued, and
  the kill landed 6 s later, mid-turn. The live WebSocket was recorded
  throughout: of 643 published events, 4 never reached the tape. All 4 are
  batched kinds, the newest of their two runs, spanning 0.27 s: inside `W` plus
  one batch. The boot closed 8 runs "interrupted by a restart" with the
  sentence about the missing window. The whole chain (seq 1 to 3532) has no
  duplicate `seq` or `stream_seq`; the only holes are the two 512 reseed gaps.
  The replay completeness probe reported `equivalent` at head 2694, with 30
  multi-op telemetry batches in its replayed tail, which settles the section 13
  question about the witness and event partitioning. An unplanned pressure
  hibernate earlier gave a second crash boot with the same result.
- **Step 5: not run.** The only marker-backed restart of one computer is the
  host's deploy refresh, outside this gate's authority, and a test computer
  refuses the marker while a run is running. Reading that path found a defect:
  the host-requested marker skips the drain and the refresh is a SIGKILL, so a
  busy protected computer loses the window silently on a planned boot
  ([problem record](../problems/host-planned-restart-skips-telemetry-drain-2026-10-22.md)).
- **Step 6: partial.** The cite barrier works: four acceptance syntheses made
  with records queued each left the queue empty, three with seal reason
  `read`. The verdict comparison and the restart-interrupted trajectory were not
  reached: synthesis of any Texture-with-research trajectory is refused by the
  vocabulary fence, a defect unrelated to batching
  ([problem record](../problems/run-acceptance-refused-by-vocab-fence-on-multi-desk-trajectory-2026-10-22.md)).
  The verifier's inconclusive rule has no product route.
- **Step 8: recorded.** Process 2 before the kill: 618 records enqueued, 118
  batches, seal reasons deadline 75, barrier 40, read 3; 53 barrier waits; no
  class violation, drop, seal failure or capacity flush; seal latency mean 141
  ms, max 512 ms. Process 1's counters were lost to the pressure kill; its k
  comes from the tape.
- **Events.** The flag-on window has 0.60 of the tape events it would have had
  without batching (1.67x fewer), inside the section 7 planning range. `t` over
  the flag-off interval was 0.57.
- **Journal.** Small. Per tape op, store bytes fell 3.7% to 5.6%; per runtime
  event, 1.3%. A fit over 15 snapshot segments gives about 86 KB of store
  growth per op and 12.5 KB per tape event, so a telemetry record costs about
  10% less at k = 5, not the 25% to 35% measured locally. This was a fresh,
  small store; the paper's may weigh the per-event share differently.

Residuals: the planned-restart drain defect blocks enabling on
`critical_protected` and `public_platform` computers, including the paper;
step 6's comparison and the verifier rule are unproven on staging; the journal
effect on the paper is unmeasured. The flag was left **off** on the test
computer, because steps 5 and 6 did not fully pass.
