# Tape volume: the forward fix, and what "tape surgery" can safely mean (2026-10-19)

Date: 2026-10-19. Analysis and design only. No runtime code changed, nothing
pushed to main, nothing run on Node B or any computer, no tape or artifact
touched. Mutation class of this doc: green. Every change it proposes is red
unless stated; the delete steps in part B are black until an off-node archive
exists.

Inputs, read in full:
[`projection-event-volume-design-2026-10-19`](../archive/reports/projection-event-volume-design-2026-10-19.md)
(the Sonnet design, "the design" below),
[`platform-artifact-small-file-growth-2026-10-19`](../problems/platform-artifact-small-file-growth-2026-10-19.md)
("the host record"),
`paper-disk-full-again-history-growth-2026-10-19`
("the guest record"), [Choir Doctrine](../choir-doctrine.md),
[standing questions](../standing-questions.md),
[computer ontology](../computer-ontology.md), and the code cited inline.

Evidence labels used throughout:

- **[V]** verified by reading the code at the cited `file:line` on
  `471689517`.
- **[M]** measured, in the cited problem record (not re-measured here).
- **[H]** hypothesis or derivation. Needs the named probe before anyone acts
  on it.

## Verdict in brief

**A. The forward fix.** Group commit is safe if it is built the way section
3 says, but it will not stop the growth on its own. Without a latency
window, the most it can give is about 1.1x to 1.3x **[H]**, and run-memory
writes cannot coalesce with each other at all: one store-wide mutex
serializes them across the whole tape round trip **[V]**
(`internal/store/run_memory.go:21-22`, `:51-88`). The fix that stops the
host growth is a different one: a per-computer **segment store** for the
four artifacts of each `projection_batch_recorded` event. It takes host
files per event from 4.02 to about zero whatever the event rate is.
Recommended order:

1. **A0** (orange, tiny). Stop the unbounded `livePayloads` map leak
   (`internal/computerevent/appender.go:391-394`; no delete anywhere
   **[V]**). Add queue-depth and batch counters, so k is measured instead
   of guessed.
2. **A1** (red, read path only, no behavior change). One tape read library,
   wired into every read site in section 4.3. It is dual-read: segments
   first, then the legacy flat file.
3. **A2** (red, flagged, default off). Write projection-batch events,
   payloads and their receipts into segments. One fsync per event, before
   the head CAS.
4. **A3** (red, flagged, default off, optional). Leader/follower group
   commit in `projectionTape.appendOps`, with no linger. Turn it on only if
   A0 measures k above about 1.3, or after the owner decides the telemetry
   durability question (section 3.5).

The guest's Dolt history is a separate leak. Its forward fix is periodic
bounded compaction (the guest record proved 48 GiB down to 7.5 GiB in
2 min 19 s), not the tape.

**B. Tape surgery.** Only one variant is safe under the standing rule as it
is written: **repacking the existing files into segments** with the same
bytes and digests. Digest verification, a census, an off-node archive and a
soak come before any legacy delete (section 6.1). Two more need an owner
ruling first:

- Deleting the whole tape of destroyed computers (section 6.3).
- Evicting old projection-batch payloads below the oldest retained base
  (section 6.4). This changes the "immutable tape" and "complete canonical
  event chain" rule. Archiving them off node, instead of deleting them,
  keeps that rule's meaning.

**Rewriting or squashing the chain is not safe**, for the concrete reasons
in section 6.5. A sweep of the orphan pins, which are not on any chain, is
safe with a grace period (section 6.2).

The "111 GiB" figure is not the tape. The tape namespaces measured 19.90 GB
in 9,074,415 files **[M]** (host record, "The tape namespaces"). Their cost
is mostly btrfs metadata: an estimated 8 to 11 GiB logical, or 16 to 22 GiB
on disk **[M, estimate]**. Most of the rest of the store is other
namespaces: `projection-base` held 47.3 GB on 10-14 **[M]**
(`artifact-store-grows-12gb-a-day-2026-10-14.md:58`), and the artifact GC
already governs it. I found no measurement of 111 GiB in the inputs; treat
it as unverified.

## 1. What a durable guest write is today

Any change has to keep this contract. These are the steps of one
`projectionTape.appendOps` call, all **[V]**:

1. Build one `ProjectionBatch` with a fresh UUIDv7 event id and the
   idempotency key `projection-batch:<event_id>`
   (`internal/store/projection_tape.go:104-129`).
2. `AppendNewPayload` takes the appender mutex. It binds the current
   platform head and dry-runs the batch against committed guest state
   inside a rolled-back serializable transaction. A refusal here happens
   **before anything is pinned** (`appender.go:338-374`,
   `internal/store/computer_events.go:96-117`).
3. It pins the payload (host: payload file plus a signed `PinReceipt` file)
   (`appender.go:375`, `internal/platform/event_artifacts.go:46-90`).
4. It pins the event (host: event file plus receipt file) (`appender.go:657`,
   `event_artifacts.go:31-44`).
5. It prepares a guest `computer_event_index` row that stores `event_json`
   (`appender.go:679`, `internal/store/computer_events.go:30-69`).
6. **Head CAS** on the platform. Under a per-computer lock it validates
   both pins by reading the receipt files and the artifacts back. It checks
   idempotency, reduces, signs an `EventHeadReceipt` over the event digest,
   previous head, sequence and both pin-receipt digests, and inserts the
   append-receipt row, all in one SQL transaction
   (`internal/platform/computer_events.go:194-295`, `:338-352`).
7. It verifies the head receipt (`appender.go:687`), then **finalizes**: one
   guest serializable transaction that advances the projection head, marks
   the index row finalized and applies every op in order
   (`computer_events.go:119-227`, `internal/store/project.go:45-62`).
8. It returns to the caller.

What this gives callers today:

- **Read-your-writes.** When the call returns, the rows are in guest SQL
  **[V]** (step 7 comes before return). `AppendRunMemoryEntry` depends on
  this. It takes the next `seq` and parent from `SELECT MAX(seq)` on the
  previous entry, under `runMemoryMu`, which is one mutex for the whole
  store **[V]** (`run_memory.go:21-22`, `:53-71`).
- **Durability, for a process crash.** The platform's SQL commit is durable
  against kill -9 (the drill is recorded in `internal/doltbatch/committer.go:1-10`).
  The guest's finalize is a committed SQL transaction. **Not durable for a
  host power loss.** `writeBlob` writes to a temp file and renames it, with
  no fsync (`internal/platform/service.go:882-898` **[V]**). A power loss
  could keep the append-receipt row while losing the event or payload
  file. That is a chain gap no replay can cross. **[H]**: it has not
  happened, and the outcome depends on the ordering of Dolt's journal
  fsync.
- **Ambiguous outcomes already exist.** A CAS that committed on the host but
  lost its response comes back to the caller as an error. A finalize
  failure after the CAS returns `ErrNeedsProjectionRepair` for a write that
  is canonical (`appender.go:682-691`). `RecoverPrepared` settles both at
  the next boot (`appender.go:699-753`).
- **The restart rule.** A crash kills blocked callers before they return,
  so nothing acknowledged is lost. A prepared row is finalized or discarded
  by `RecoverPrepared`, which is recovery of a durable fact, not resumption
  of work. The appender has no queue that outlives its callers.
- **Restore granularity.** "Intermediate heads between ops in the same batch
  are not restore targets; the whole batch is"
  (`computerevent/projection_batch.go:63-64` **[V]**).
- **Semantic events do not go through `appendOps`.** Acceptance, platform
  update, lifecycle and materialization events call `AppendNew*` directly
  (for example `agentcore/platform_update.go:203`,
  `lifecycle_receipts.go:49`, `self_development_materializer.go:595`
  **[V]**). They share the appender mutex but never share a batch.

### Who reads old tape objects

All **[V]**:

| Reader | What it reads | How far back |
|---|---|---|
| Head CAS `ValidateEventPins` (`event_artifacts.go:92-177`) | both receipts plus both artifacts | the event being appended only. **Pin receipts are read nowhere else.** |
| Replay page `EventsPage` (`internal/platform/event_replay.go:15-120`) | event file by digest; head receipt from SQL | from the start sequence the caller asks for |
| Guest payload fetch `FetchPayload` (`event_artifacts.go:194-221`) | payload by digest. It searches 3 namespaces and is not scoped to a computer. | per replayed event |
| Boot and recovery (`internal/recoveryplan/recovery_plan.go:81-140`) | a tail after a base or the local head | **at most 10,000 events** (`:15`). It never replays from genesis on an existing chain. |
| Restore (`agentcore/restore_base.go:87-122`) | an advertised or pinned base, then the tail | a target older than every base and above sequence 10,000 **is refused today** (`:113-118`) |
| State-completeness probe (`agentcore/replay_completeness.go:434-472`) | base plus tail | a genesis replay only when the chain is at most 10,000 events (`:447-456`) |
| `checkpointd` base rebuild (`cmd/checkpointd/worker.go:155-185`) | pinned seed plus tail. The indexed `EventsPage` override is at `:30-32`; payloads come from `DiskEventSource.FetchPayload`. | the tail, unless an explicit `GenesisRepair` job runs (`:180-185`) |
| Checkpoint authority (`internal/platform/checkpoints.go:214-285`) | `genesis_imported` and `verification_recorded` events plus the verifier payload | by digest, from any age |
| Platform update and self-development (`agentcore/platform_update.go:592,908,1058`, `self_development_materializer.go:396,885`) | non-projection payloads; a bundle URI that names `computer-event-payload/<digest>` | by digest, from any age |
| Ledger witness CLI (`internal/ledgerwitness/walk.go:66-110`) | walks back from a head through events and payloads, capped; **fails closed on a missing payload** (`:131-137`) | up to the cap |

Two corrections to the inputs, both **[V]**:

- "Genesis replay is the state-completeness check" is out of date for long
  chains. Boot, restore and the completeness probe all refuse a genesis
  replay of more than 10,000 events and use base plus tail. Genesis replay
  survives only as the explicit `GenesisRepair` job and for short chains.
- "The offline rebuilder walks to genesis per page" does not hold for
  `checkpointd`. It overrides `EventsPage` with the platform's indexed page
  (`worker.go:30-32`). The walk to genesis and the `ReadDir` mode of
  `DiskEventSource` (`projectionbase/source.go:54-114`) have no production
  caller that uses them. The only constructor call passes a head
  (`worker.go:156`), and `Head` and `EventsPage` are overridden.

## 2. Critique of the design

What it gets right:

- The batch format already allows many ops, and replay applies them in
  order (`project.go:45-62`). A multi-op batch needs no change to the
  format, the replay or the witness.
- Natural coalescing gives little. A real k needs a latency window or a
  non-blocking telemetry path, and the host record's 3x to 7x assumed the
  window without pricing it.
- `computer_event_index.event_json` duplicates the host event file, and it
  is inside the content witness. Shrinking it is a witness-version change.

What it misses or gets wrong:

1. **Run memory cannot coalesce with run memory.** `runMemoryMu` is one
   mutex for the store, and it is held across the whole `appendOps` round
   trip (`run_memory.go:21-22`, `:84-86` **[V]**). Run-memory ops (31% of
   batches on 10-15 **[M]**) can only join a batch led by a `choir.event`
   op. To coalesce them with each other, the mutex must first become
   per-run and `seq` allocation must move into memory, the way
   `AppendEvent` already does (`store.go:1953-1980`). That is a separate,
   prior change.
2. **Split-and-retry is safe only before the pin.** The design says "on
   failure, split and retry callers individually". That is safe only for
   the dry-run refusal (`appender.go:364-373`), where nothing is pinned or
   CAS'd. After the payload pin, a failure is either a clean refusal
   (orphan pins) or an ambiguous CAS outcome. A blind retry could append
   the same ops twice under a new event id. Object ops are upserts and
   would be harmless; run-memory inserts, guarded Texture mutations and
   `CreateOnly` ops would not be. Rule: retry only on a typed pre-pin
   refusal. Return any later error to every member unchanged, which is
   today's ambiguous-outcome semantics, one per caller.
3. **The `livePayloads` leak.** Every payload a process appends stays in a
   map forever. That includes the non-projection payloads, which nothing
   reads from it (`appender.go:127`, `:391-394`, `:1012`; no delete
   **[V]**). At about 126k events a day and about 5.5 KB a payload, that is
   roughly 0.7 GB a day of guest heap until the next restart **[H,
   derived]**. This should be fixed first, and it is orange.
4. **Event-count bounds change meaning.** The base cadence is
   `tail >= 2500` events (`internal/platform/projection_jobs.go:90-96`
   **[V]**). The recovery bound is 10,000 events
   (`recovery_plan.go:15`). With k ops per event, a base is published k
   times less often. That saves bytes: bases are 8.7 to 15.9 GB each
   **[M]**. But a tail then carries k times more ops, so tail replay time
   and the rebuilder's 2 GiB RSS guard (`projectionbase/types.go:22`) must
   be measured in ops. **[H]**: per-op replay cost falls, because the
   per-event cost is two guest transactions and one HTTP payload fetch.
5. **The fsync placement in the segment sketch.** One fsync per event, at
   the event pin, covers the payload record in the same segment file and
   comes before the CAS. That is strictly better than today's zero fsyncs.
   A per-record fsync is not needed.
6. **Scope the segments to projection-batch artifacts.** All the odd
   readers (checkpoint authority, platform update, self-development bundle
   URIs) read non-projection payloads. Segmenting only
   `projection_batch_recorded` events, their payloads (media type
   `ProjectionBatchMediaType`) and their two receipts covers about 99.6% of
   the volume **[M]** and leaves those readers on the legacy path.
7. **A rollback hazard the design does not name.** Once a segment holds
   data, reverting the read path would make the chain unreadable: boot
   replay, CAS validation and base builds would all fail closed. The read
   library must be its own commit, never reverted after A2 has written,
   and paired with an "explode" tool that turns segments back into legacy
   files before any binary rollback.
8. **Conditional OG writes.** `putBatch` evaluates conditions under
   `engineMu`, releases it, then appends (`objectgraph/dolt_store.go:839-848`
   **[V]**). That is check-then-act today. Group commit must keep these out
   of merged batches. The real fix is to carry the conditions into the
   batch, so dry-run and finalize check them under the appender mutex.
   Separate it; do not fold it into A3.

## 3. Group commit (A3), designed so it is safe

### 3.1 Shape

New file `internal/store/projection_coalesce.go`. `appendOps` dispatches to
it only when the flag is on **and** the context carries a coalesce mark.
Everything else keeps today's direct path.

- **Marking.** `Store.AppendEvent` (`store.go:1929`) and
  `AppendRunMemoryEntry` (`run_memory.go:20`) wrap the context with
  `withProjectionCoalesce(ctx)`. `interceptOG` passes the context through.
  Unmarked calls include conditional OG batches, desktop state,
  self-development ops, Texture mutations and aliases. They take the direct
  path and flush alone. That is an allowlist, not a classifier.
- **Queue.** A FIFO of `pending{ops, bytes, done chan error}` under its own
  mutex. A caller that finds no active leader becomes the leader. The
  leader takes the head of the queue, up to 64 ops or 1 MiB, builds one
  batch and calls the unchanged `AppendNewPayload`. It delivers the result
  to every member. If the queue is not empty, it hands leadership to the
  next waiter and returns. There is no timer and no background goroutine,
  so idle latency is unchanged, and a caller waits for at most one batch in
  flight plus its own.
- **Order.** Members keep FIFO order, and ops inside a member keep their
  order. Today, concurrent callers are ordered by whoever wins the mutex,
  so FIFO is at least as ordered.
- **Pre-pin split.** Add a sentinel `computerevent.ErrBatchRefused` and wrap
  it at `appender.go:368` and `:371`. On that error, the leader re-runs
  each member alone through the direct path and delivers per-member
  results.
- **Any other error.** The same error goes to every member. This is
  today's ambiguous-outcome contract.
- **Context.** A member whose context is cancelled while it is still queued
  is removed. Once the leader has taken it, it waits for the result. The
  append runs under `context.WithoutCancel(leaderCtx)` with a fixed
  timeout, so one member's cancellation cannot fail the others mid-CAS.
- **Return.** Every member returns only after finalize. Read-your-writes
  and run-memory `seq` allocation are unchanged.

### 3.2 Invariants

- The chain: event digest, previous head, request commitment, pins and
  head receipt are computed exactly as today. A merged batch is an
  ordinary event.
- Replay determinism: a merged batch replays as one ordered transaction.
  Live and replay produce the same witness, because both derive from the
  same tape. Old bases and checkpoints are untouched.
- Idempotency: one fresh event id and idempotency key per merged batch.
  No member op is ever appended twice (the rule in 3.1).
- Restart: no persistent queue. A crash loses only calls that never
  returned, and the crash rule closes their runs. A planned restart behaves
  as today, because callers are still blocked in `appendOps`.
- Fate-sharing: a poison op could fail the whole batch after the CAS. The
  dry-run under the same appender mutex (`appender.go:364-373`) makes that
  as unlikely as today. Guarded op kinds never enter a merged batch.

### 3.3 Failure modes, written before code (test each first)

1. One member's op fails the dry-run. Expect: the others still succeed
   individually and the bad member gets its own error.
2. The CAS response is lost after commit. Expect: no retry, every member
   gets the error, `RecoverPrepared` finalizes at boot, and no op appears
   twice on the tape.
3. A member is cancelled while queued, or while in flight.
4. Leader handoff under contention: no lost wakeup, and no member waits
   for more than one extra batch.
5. A cap boundary: 65 ops, or a single op over 1 MiB, which goes alone.
6. Crash with the queue full: no member returns success, and boot shows no
   prepared or finalized row for them. (E2E on staging with a planned
   kill.)
7. Interleaving with `HoldAppends` and semantic `AppendNew*`: no deadlock,
   because the leader takes only the appender mutex.
8. The flag is off: byte-identical behavior. A golden test compares tape
   events for a scripted workload.
9. The ledger witness and inbox records: a merged batch still holds each
   record exactly once (`ledgerwitness/walk.go:7-9`). Two records can now
   share a sequence. **[H]**: no consumer compares positions within one
   sequence (the CLI at `cmd/ledger-witness/main.go:31` is the only
   caller).

### 3.4 Expected effect

- k is about 1 + (mean queue depth at the appender mutex) **[H]**. That is
  about 1.1 to 1.3 at today's arrival rate (6.6 events per active second
  **[M]**, service time unmeasured).
- The reduction is in host files, receipts, platform Dolt rows, guest index
  rows and base publications, each by about k.
- `og_objects` and `run_memory_entries` inserts do not change.
- A0 decides whether A3 is worth enabling.

### 3.5 The owner decision behind a large k

A large k means a window. On the run's hot path the window is latency:
`emitEvent` is sequential within a run. Or it means a non-blocking
"telemetry" class for `choir.event`, which is acknowledged before it is on
the tape. That class breaks "nothing acknowledged is lost on a crash"
during the window, and with it the ontology's "complete canonical event
chain" (`docs/computer-ontology.md:343`).

Do not take it in a slice. If the owner wants a large k, the consistent
form is: telemetry enqueue returns at once; the queue flushes every 250 ms
to 1 s; a planned restart drains it; and a crash loses at most one window
of trace lines from runs that the crash rule closes anyway.

## 4. Segment store (A1 + A2): the host forward fix

### 4.1 Format

The paths below are relative to the artifacts root.

- **Layout.** `tape-seg/<computer_id>/seg-NNNNNNNN.seg`, append-only.
  `seg-NNNNNNNN.idx` is written at seal. Put `tape-seg` on its own btrfs
  subvolume so that it can be snapshotted cheaply. **[H]**: today
  `platform-artifacts` sits inside `/@` (host record, "Mount").
- **Record.** `magic u32 | ver u8 | ns u8 (event=1, payload=2, receipt=3) |
  sha256 [32] | len u32 | crc32c u32 | bytes`. The digest is SHA-256 of
  the bytes for all three. A receipt's digest is that of its canonical
  JSON, matching the legacy name `pin-receipts/<digest>.json`
  (`event_artifacts.go:85-86`).
- **Open.** Sealed `.idx` files are sorted digest-to-offset tables, read by
  binary search. The open segment is scanned at startup and its torn tail
  truncated at the first bad crc. A bad crc in a sealed segment is
  corruption: the read fails closed, and nothing is truncated.
- **Rotation.** At 64 MiB or 50k records: fsync the segment, write the idx
  to a temp file, fsync it, rename it, fsync the directory.

### 4.2 Write path (A2)

All changes are in `internal/platform/event_artifacts.go`.

- `pin` (`:62-90`) routes to `tapestore.Put` when the flag is `write` and
  either the namespace is `computer-event` with kind
  `projection_batch_recorded`, or the namespace is `computer-event-payload`
  with media type `ProjectionBatchMediaType`. The receipt goes to the same
  segment.
- `PinEvent` passes `sync=true`. One fsync covers the event's four records,
  and it lands before the guest can send the CAS.
- `writeImmutable` (`:223-239`) keeps its collision check across both
  stores. Same bytes: no-op. Different bytes: error.
- Writes are serialized per computer by a lock in `tapestore`. The guest
  appender already serializes pins, but the host must not trust that.

### 4.3 Read path (A1), the same library everywhere

`tapestore.Get(computerID, ns, digest)` falls back to the legacy file. It
is wired into these sites:

- `event_artifacts.go:116-120` and `:158-162` (`ValidateEventPins`).
- `event_artifacts.go:203-218` (`FetchPayload`). It stays computer-scoped
  for segments and keeps its unscoped legacy fallback.
- `event_replay.go:68-77` (`EventsPage`).
- `checkpoints.go:222`, `:247`, `:266`.
- `projectionbase/source.go:57-58` and `:180-182`. The `ReadDir`
  enumeration at `:80-114` must **refuse** once segments exist for the
  computer. It has no production caller, and if left alone it would
  silently miss segment-resident events: fail-open.
- `ledgerwitness/walk.go:119`, `:135`.

`tapestore` is read-only for offline tools. The platform process is the
only writer.

### 4.4 Failure modes (tests first)

1. A torn tail after a kill mid-append. Expect: truncated on open. The
   record was never acknowledged, because its fsync and CAS had not
   happened.
2. A power loss after fsync and before CAS. Expect: an orphan record,
   harmless. The orphan rule in 6.2 applies.
3. A crc mismatch in a sealed segment. Expect: the read fails closed and
   there is no fallback to a wrong byte. The legacy fallback applies only
   when the digest is absent.
4. A concurrent reader on the open segment. Expect: it reads only
   crc-valid records. A partial record looks absent and resolves through
   legacy or a retry.
5. An index and segment skew after a crash during seal. Expect: rebuild the
   index from the segment by scan.
6. Duplicate puts of the same digest are idempotent. A different body under
   the same digest is a collision error.
7. Flag off after writes: reads still resolve, because the read path never
   turns off.
8. A binary rollback below A1. Expect: a refused deploy, or the explode
   tool first. This is a runbook gate, not code.
9. Disk full mid-append. Expect: a pin error, no CAS, the caller gets an
   error, and nothing acknowledged is lost.

### 4.5 Proof, measurement and rollback

**E2E on staging** (CLAUDE.md prefers E2E; the artifact is a JSON report
under `tests/`):

- Drive N guest writes on a staging computer with the flag on.
- Then assert:
  - tape files created per 1,000 writes is about 0, read from an
    in-process counter on `writeBlob` and `tapestore.Put`, exposed on the
    platform diagnostics endpoint so that no SSH is needed;
  - the completeness probe passes, with the base-plus-tail witness equal
    to live;
  - a planned restart and a crash restart both boot and replay through
    segment-resident payloads;
  - `checkpointd` publishes a base whose tail came from segments;
  - after a flag flip to off, writes go to legacy and every earlier digest
    still resolves.

**Measurement on Node B, after the paper is enabled:**

- files per event in the tape namespaces drops from 4.02 to below 0.01;
- btrfs metadata growth is near zero (the existing headroom timer's log);
- bytes per event stay at about 8.8 KB.

**Expected reduction:** about 0.45M files a day to about 20 a day (segments
plus indexes) at today's rate **[H, derived]**. The bytes do not change
(about 1.1 GB a day), and sealed segments compress later.

**Rollback:** set the flag to off. The read path stays. Before any binary
rollback below A1, run the explode tool.

**Mutation class:** red. The protected surfaces are pin receipts, the event
and payload artifacts, replay, `checkpointd` and the ledger witness.

### 4.6 What A2 does not fix

- The platform Dolt row per event (`computer_event_append_receipts`). Group
  commit is the only lever there.
- The guest store. Its forward fix is periodic bounded compaction (the
  guest record's failure modes 1 to 5 and its bounded GC flags), which runs
  as a separate red mission.

### 4.7 Standing questions for A0 to A3

1. **Settled by:** nobody. The owner asked for a cause and a permanent
   solution. This plan is a proposal.
2. **Topology:** no new Dolt store and no tape bytes in Dolt
   (`internal/platform/content_store.go:16-18`, the "104 G lesson"). The segments are files under the
   existing artifact root.
3. **Deletion citers:** none in A0 to A3. Grep `computer-event`,
   `pin-receipts` and `finalizeSinceCheckpoint` before coding.
4. **Production consumers:** section 1's reader table.
5. **Single authority:** the platform head and append receipt. Segments are
   storage, not authority.
6. **Artifact:** the E2E JSON report plus the Node B files-per-event
   counter.
7. **Fate-sharing:** unchanged. Every reader already depended on the
   platform host's disk.
8. **Restart:** segment open-scan truncation, and no queue that outlives
   its callers.
9. **No-SSH:** counters on the diagnostics endpoint and the flag in runtime
   configuration.
10. **Registry:** a throughline goal file when executed.

## 5. Plan, file by file

| Slice | Class | Files and functions | Flag | Proof |
|---|---|---|---|---|
| A0 | orange | `computerevent/appender.go`: delete the `livePayloads[payloadDigest]` entry when `AppendNewPayload` returns (`:391-396`); add counters (mutex waiters on entry, service time, ops per batch, kinds per batch) to the health snapshot. `platform/service.go` `writeBlob`: per-namespace file counter. | none | heap flat across 10k writes (E2E); counters on staging health |
| A1 | red, no behavior change | new `internal/tapestore` (`Open`, `Get`, `Put`, `Seal`, `Scan`); read sites from section 4.3; `cmd/tape-explode` | read path always on | E2E reads with zero segments are identical; unit failure modes 1, 3 to 6 |
| A2 | red | `event_artifacts.go` `pin`, `PinEvent`, `writeImmutable` | `PLATFORM_TAPE_SEGMENTS=off/write` | section 4.5 |
| A3 | red | `store/projection_coalesce.go` (new), `projection_tape.go` `appendOps`, `store.go` `AppendEvent`, `run_memory.go` `AppendRunMemoryEntry` (mark only), `appender.go` sentinel `ErrBatchRefused` | `CHOIR_PROJECTION_GROUP_COMMIT=0/1` | section 3.3 plus staging k and latency p50/p99 |
| later | red | per-run `runMemoryMu` plus in-memory `seq` (prerequisite for run memory coalescing); conditions carried in the batch (fixes check-then-act) | each flagged | own missions |

## 6. Part B: can we safely do tape surgery?

The standing rule, as the owner states it: never edit the tape, never
rewind event heads. As written in
`docs/archive/problems/sa-projection-base-watermark-never-refreshed-2026-10-07.md:201`:
"retain readable prior bases and the immutable tape; never rewind event
heads". The ontology also says: "one stable ComputerID owns a complete
privacy-safe canonical event chain" (`docs/computer-ontology.md:343`).

The facts that decide which variants are safe, all **[V]**:

- The chain commits to payloads only by digest. The canonical event bytes
  include `previous_head`, `payload_commitment` and `output_artifact_refs`
  (`computerevent/event.go:83`, `:95-96`, `:174`). The signed head receipt
  binds the event digest, the previous head, the sequence and the
  pin-receipt digests (`platform/computer_events.go:338-352`). So the chain
  of headers stays verifiable without the payload bodies, but those events
  can no longer be replayed.
- A projection-batch payload belongs to exactly one event. It embeds
  `event_id` and `computer_id` (`projection_tape.go:108-114`), and replay
  checks both (`appender.go:1041-1043`). Other payload kinds (bundles,
  verifier results, platform updates) can be named from elsewhere, for
  example by the self-development bundle URI
  (`self_development_materializer.go:396`).
- Pin receipts are read only at the CAS of their own event
  (`event_artifacts.go:116`). Head-receipt verification does not read them
  (`computerevent/event_receipt.go:13-36`). For old events they are pure
  evidence.
- The artifact GC excludes the tape namespaces by design
  (`platform/artifact_gc.go:100-106`). Projection bases are retained as:
  the advertised watermark, explicit pins, and each computer's latest two
  (`:344-361`).

### 6.1 Variant 1: repack existing files into segments (same bytes, same digests)

- **Respects the rule as written.** No event, digest, receipt, head or
  sequence changes. This is a storage-layout change.
- **What it reclaims:** the 9,074,415 tape files, about 87% of them inline
  metadata, and an estimated 8 to 11 GiB of logical metadata (16 to 22
  GiB on disk) after the delete and a metadata balance **[M, estimate]**.
  The byte total stays at about 20 GB, and sealed segments compress.

**Steps:**

1. A1 has shipped, and the read path has run on staging and Node B for a
   soak period.
2. **Dry run.** For each computer, in sequence order from
   `computer_event_append_receipts`, list every referenced digest: event,
   event receipt, payload receipts, and the payload from the event's
   `payload_commitment`. Write a manifest of computer, sequence, namespace,
   digest and size. No writes. The manifest totals must match the host
   record's census within the orphan count.
3. **Backfill**, idle IO, per computer, into sealed segments, with
   `sync=true` per segment. Re-read and re-hash every record after its
   fsync. Projection-batch artifacts only, as in 4.2; the rest stay as
   files.
4. **Census.** Every manifest digest resolves through `tapestore.Get`
   **from a segment** (legacy fallback disabled for the census) with equal
   bytes. A second, independent check: a staging-style replay of each live
   computer's tail from its watermark, plus one base rebuild from segments
   only, must match the witness.
5. **Archive** the sealed segments and manifest off node (encrypted:
   payloads are owner plaintext, `privacy_class` `owner`), and verify by
   hash at the destination.
6. **Soak** for 7 days with the legacy fallback **disabled by flag** but
   the files present. Any miss shows up as an error, not as a silent
   fallback.
7. **Delete the legacy files** in bounded, idle-priority batches, logged
   against the manifest. Then run `btrfs balance -musage=...` to return
   metadata chunks.

**Backups, honestly:**

- Do **not** reflink-copy the 9M files. That creates 9M new inodes and
  doubles the scarce resource.
- A btrfs snapshot of `/@` pins all the old metadata until it is deleted,
  and deleting files under a live snapshot COWs metadata nodes. Metadata
  use can rise during step 7 **[H]**.
- So the off-node archive plus the segments themselves are the backup. A
  snapshot, if one is taken at all, should cover only a dedicated subvolume
  and be dropped before the balance.

**Rollback:**

- Before step 7: re-enable the fallback, and the segments can be dropped.
- After step 7: `tape-explode` from local segments, or from the archive.

**Class:** red for steps 1 to 6. Step 7 is black on the node and
recoverable only from the archive, so it is treated as red only once the
archive is verified.

**Owner rulings needed:** no doctrine change. The owner should approve the
red operation on Node B and name the archive target.

### 6.2 Variant 0: sweep orphan pins

About 38,000 payload and receipt files sit outside any chain (host record,
"Measured cost per event"). They exist because pins land before the CAS
(`appender.go:375`, `:657`, then `:682`).

- **Respects the rule.** They are not on any tape.
- **Safe with:**
  - a reference set built from all computers' append receipts, plus every
    event's input and output refs (non-projection payloads may be shared);
  - a grace period of at least one day, so an in-flight pin is not taken
    for an orphan;
  - a dry-run manifest and an archive before the delete.
- **Reclaims** about 0.4% of the files.
- **Owner ruling:** a signed `PinReceipt` for an artifact that never
  entered a chain is still a signed platform statement. Is it disposable?

### 6.3 Variant 2a: retention of destroyed computers' tapes

224 computers have no ownership record. Together they hold 308,221 events,
about 1.24M files and 2.7 GB (13.6%) **[M]**.

- **Does not edit any live tape**, but it deletes whole chains, and
  `computer_event_heads` and the append-receipt rows would still point at
  them.
- **Needed:**
  - per computer, a proof that it is destroyed (vmctl registry plus no
    route plus no fork lineage child);
  - an archive (segments) off node;
  - a tombstone row, so that readers fail closed with "archived" instead
    of a chain-gap error;
  - then the delete.
- **Owner ruling:** whether "never edit the tape" covers the whole tape of
  a destroyed computer, and how long to keep the evidence of a destroyed
  computer. Class: black without the archive, red with it.

### 6.4 Variant 2b: evict payloads of old projection events below the oldest retained base

**The rule as written:** this **does not** respect it. The events would
still say `payload_commitment X` with X no longer resolvable, so the tape
would no longer be "complete". It needs the owner to amend the rule.

**Keep set**, per computer:

- every non-projection payload;
- every event file and every receipt;
- projection payloads with sequence ≤ 10,000 (the genesis fallback window
  in `restore_base.go:113-118`);
- for each retained base b (advertised, pinned or latest two), the
  payloads in (b.seq, b.seq + 10,000];
- the payloads in (W, head].

**What stays working:** boot and recovery, restore to any checkpoint that
has a pinned base, the completeness probe, `checkpointd` cadence and chain
verification. Their reads are all inside the keep set (section 1 table).

**What breaks:**

- `GenesisRepair` (`checkpointd/worker.go:180-185`), the only independent
  re-derivation of state from genesis. Without it, the oldest retained
  base becomes a **trust anchor**: it can be checked against its own
  witness (`projectionbase/verify.go:68-104`), but never re-derived.
- Ledger-witness walks that cross the cut. They fail closed
  (`walk.go:135-137`).
- Any future audit that replays an old range.
- A fork or restore from a point that has no retained base. That is
  already refused today above sequence 10,000.

**What it reclaims:** for the paper, about 1.3M payload files and about 7
GB **[H, derived]**. After variant 1 it reclaims only bytes, by dropping
whole sealed segments.

**The safer form: archive, not delete.** Move the segments below the cut to
off-node storage and resolve them on demand. Then nothing is lost but
latency, and "immutable tape" keeps its meaning if the owner rules that
"retrievable from the archive" counts as retained.

**Owner rulings needed:**

- amend the immutable-tape and "complete chain" wording;
- accept bases as trust anchors, or keep the archive so that genesis
  repair stays possible;
- choose the archive target and how long to keep it.

**Class:** black if deleted, red if archived.

### 6.5 Variant 3: rewrite or squash the chain. Not safe.

Squashing events, even only an old prefix, changes the `previous_head` of
the first event after the squash. That changes its digest and then every
digest and sequence after it. Concretely, it breaks:

- **Head receipts.** Every receipt after the squash point signs the old
  `event_digest`, `previous_head` and `sequence`
  (`computer_events.go:338-352`). They would need re-signing, which turns
  history into a new platform statement, or would no longer verify
  (`event_receipt.go:13-36`). Replay verifies every receipt
  (`appender.go:810`).
- **Pin receipts and request commitments.** These are bound to the old
  event bytes (`event_artifacts.go:140-150`).
- **Bases.** Every descriptor names `Sequence` and `CanonicalHead`, and
  `VerifyTailHead` requires the first tail event to chain from that head
  (`projectionbase/verify.go:144-155`). Every published and pinned base
  would refuse.
- **Guest stores and every base blob.** Their `computer_event_index` and
  `computer_event_projection_heads` hold the old digests and sequences, and
  appends require platform head == embedded head (`appender.go:642-643`).
  Every computer would hit `ErrNeedsProjectionRepair`.
- **Checkpoints, restore and routes.** `AcceptedEventHead`, pinned-base
  references (`projectionbase.CheckpointPinReference(head)`) and
  self-development operation heads (base, desired, effective) all name old
  heads. Restore would refuse with "not on the canonical chain"
  (`restore_base.go:103-106`).
- **Identity.** `ArtifactProgramRef "event-chain:<head>"`
  (`projectionbase/witness.go:18-19`), ComputerVersion identities, and the
  heads that vmctl and its reports hold.
- **Published references.** Anything that cites a head or an event digest
  outside the computer.

It is also a direct violation of "never rewind event heads".

The only consistent version of "a compact chain" is a **fork**: a new
computer whose genesis imports a verified base. It has a new ComputerID and
lineage to the parent. Per the ontology (`computer-ontology.md:409-414`)
and CLAUDE.md, forks wait for fork re-keying and isolation to land. It
changes the paper's identity, and it reclaims nothing until variant 2a is
applied to the retired parent. Not recommended for this problem.

### 6.6 Guest history is not tape surgery

The guest's roughly 40 GiB of Dolt history is the commit graph of a
projection, not the tape. Squashing it plus a bounded full GC (the guest
record: 48 GiB to 7.5 GiB, witness-checked) respects the tape rule. It is
the first reclaim to do, through the tool change and failure modes that
record already lists.

The `computer_event_index` table (3.84 GiB live **[M]**) holds every event
header since genesis. It shrinks only with a witness-version change, as the
design said.

## 7. Heresy delta

- **discovered:**
  - `livePayloads` grows without bound in every guest process
    (`appender.go:391-394`).
  - Run-memory appends are serialized store-wide across the tape round
    trip (`run_memory.go:21-22`), which caps any coalescing gain and adds
    latency.
  - Removing the read path after segments hold data would make the chain
    unreadable (the rollback hazard above).
  - The genesis-replay framing in the inputs is out of date (section 1).
  - The "111 GiB" figure is not a tape measurement.
- **introduced:** none (docs only).
- **repaired:** none.

## 8. Not verified

- k under leader/follower, and append service time. A0 measures both.
- The cost of btrfs fsync per event on Node B.
- Whether metadata use rises while files are deleted under a snapshot.
- Whether `platform-artifacts` is on its own subvolume (the host record
  shows only `/@`).
- The paper's current watermark, its pinned bases, and so the real size
  of the keep set in 6.4.
- The pin handler's request-body limit. None was found in
  `event_handlers.go:199-236`; one may exist at the server level.
- Whether anything outside Go (runbooks, scripts) reads tape files. A grep
  of `scripts/`, `nix/` and `docs/runbooks/` found none.

## 9. A1 implementation plan and failure modes (2026-10-20)

Owner-approved in owner notes §26 ("A1 (segment reads with legacy
fallback)"). This section is written before the code, as the process
requires. Mutation class: **red**. Protected surfaces: the event evidence
store (pin receipts, event and payload artifacts), replay (`EventsPage`,
`FetchPayload`), checkpoint evidence verification, the tape sweeps, and the
offline readers (`projectionbase`, `ledgerwitness`). Rollback: revert the
commits. **No segment is written by any production code path in A1**, so a
revert strands no data. The rollback hazard in section 2, item 7 begins at
A2, not here.

### 9.1 Scope

In: the segment file format v1 (appendix A of this doc is the spec, added
with the code), a read-only reader with an index, and every read site in
section 4.3 moved onto it, segment first and the legacy per-file layout
second. Out: any production writer (A2), group commit (A3), the
`tape-explode` tool (A2, because nothing can be exploded before A2 writes),
and the destroyed-computer sweep for segment-resident tapes (A2; A1 makes it
refuse). The offline byte-identical repack tool is **deferred to a note**: it
is a thin caller of the fixture writer plus the manifest census of section
6.1 steps 2 to 4, and it has no use until a soak of A1 has run (section 6.1
step 1).

### 9.2 Package and call shape

New `internal/tapestore`, standard library only (no import of
`computerevent`, so every reader, including the offline ones, can use it
without a cycle).

- `NewReader(artifactsRoot)`: the one read entry point.
  `Read(computerID, ns, digest) ([]byte, Source, error)`. It looks in
  `tape-seg/<computer_id>/` first, then in the legacy file, and returns the
  bytes only after recomputing SHA-256 and comparing it with `digest`.
- `HasSegments(computerID)`: whether the computer has any segment file. The
  `ReadDir` enumeration and the destroyed sweep use it to refuse.
- `Scan(computerID, fn)`: full-verify iteration of every record, for tests,
  the census and the future repack tool.
- `SegmentWriter`: create, append, seal. **Test fixtures and offline tools
  only.** A guard test fails if any non-test source outside
  `internal/tapestore` references it.
- Errors: `ErrNotFound` (absent from every layout), `ErrCorrupt` (a segment
  or index failed a check; wraps the detail), `ErrDigestMismatch` (a legacy
  file does not hash to its name).

Wiring (`file:line` on `72435fcd1`):

| Site | Change |
|---|---|
| `event_artifacts.go:116-120`, `:158-162` `ValidateEventPins` | receipt and artifact via `Read`, scoped to `request.Event.ComputerID` |
| `event_artifacts.go:203-218` `FetchPayload` | tombstone gate first (unchanged), then per namespace `Read`; `projection-base` stays legacy-only |
| `event_replay.go:68-77` `EventsPage` | event via `Read` |
| `checkpoints.go:225`, `:250`, `:269` | genesis event, verifier event and verifier payload via `Read` |
| `tape_sweep.go:684` `walkTapeChain` | event via `Read`; a corrupt segment is an error, never "unreadable, skip" |
| `tape_sweep.go:1158-1200` `tapeDestroyedItems` | refuse a computer for which `HasSegments` |
| `projectionbase/source.go:57`, `:97`, `:180` | `Read`; `ReadDir` mode refuses when the computer has segments |
| `ledgerwitness/walk.go:119`, `:135` | `Read` |
| `artifact_gc.go` | no read of the tape; a test pins that `tape-seg/` is outside every swept namespace |

### 9.3 Decisions that make it fail closed

1. **A segment hit is final.** A digest found in a segment is verified and
   returned. A digest absent from the segments falls back to legacy. A
   segment error (corrupt, truncated, mismatched, unsupported version) is
   returned to the caller. It never triggers the fallback, because a
   fallback would answer from different storage than the record that was
   meant.
2. **Every returned byte is hashed.** Segment records carry a CRC32C for
   torn-write detection and the SHA-256 digest for identity. `Read` checks
   the CRC, then recomputes SHA-256 over the body and compares it with both
   the record's digest field and the requested digest. A matching CRC with a
   wrong digest is corruption.
3. **Sealed vs open.** A segment with a `.idx` is sealed; every check is
   strict and a failure is `ErrCorrupt`. A segment without one is open; a
   reader scans it read-only, and a torn tail at the end of the
   highest-numbered segment is tolerated (absent), because an unacknowledged
   append can end there. A bad record that is followed by any valid record,
   or any bad record in a non-last unsealed segment, is corruption, not a
   torn tail.
4. **The index is checked against the segment.** An index must name the same
   segment number and computer, its recorded segment size must equal the
   file size, its CRC must hold, and every entry it returns must point at a
   record header with the same namespace, digest and length. An index
   without its segment is corruption.
5. **Unknown version or magic is an error**, so an older binary does not
   silently ignore a newer segment layout.
6. **Tombstones come first.** The tombstone gate is unchanged and still runs
   before any read, so a tombstoned computer is refused before a segment is
   opened. Offline readers keep their existing behavior (no database).
7. **Sweeps read the same library.** The tape sweep builds its reference set
   from chain events. If events were read from legacy only, a
   segment-resident event would look unreadable (a refused sweep, safe) or,
   in a later careless edit, its payload references would be missing and a
   legacy-only payload would look orphaned. With the event read through
   `Read`, the payloads it names are referenced whichever layout they sit
   in. The sweeps delete only legacy files and never touch `tape-seg/`.

### 9.4 Failure modes, written before code (test each first)

Format and reader (`internal/tapestore`):

1. A bit flip in a record body of a sealed segment. Expect `ErrCorrupt`,
   even when the legacy file for the digest exists and is intact.
2. A record whose digest field disagrees with its body but whose CRC was
   recomputed to match. Expect `ErrCorrupt`.
3. A sealed segment truncated at a record boundary, mid-record and inside
   the header. Expect `ErrCorrupt` for any lookup that must probe it.
4. A torn tail on the highest-numbered unsealed segment (cut mid-header,
   mid-body, and a zero-filled tail). Expect the earlier records readable
   and the torn one absent, falling back to legacy or `ErrNotFound`.
5. A bad record in the middle of an unsealed segment with valid records
   after it, and a bad record in a non-last unsealed segment. Expect
   `ErrCorrupt`.
6. Index and segment mismatch: wrong segment size, wrong segment number,
   wrong computer, bad CRC, entry offset pointing at a different record, an
   index with no segment, a truncated index. Expect `ErrCorrupt` each time.
7. Unknown segment version, unknown index version, bad magic, unknown
   namespace byte, a record length above the limit. Expect `ErrCorrupt`.
8. A digest present in both layouts. Expect the segment copy, equal bytes.
   With a corrupt segment copy and an intact legacy copy: `ErrCorrupt`.
9. A legacy-only digest. Expect legacy bytes; a legacy file that does not
   hash to its name is `ErrDigestMismatch` (today's behavior).
10. A digest in neither layout. Expect `ErrNotFound`.
11. Concurrent readers on sealed and open segments (race detector), with
    the fixture writer appending to the open segment meanwhile. Expect only
    correct bytes, or "absent" for a record not yet whole.
12. Path safety: a computer id containing a separator or `..`, a non-hex
    digest. Expect no filesystem access outside the root.
13. Duplicate digests across segments return the same bytes.
14. No `tape-seg/` directory at all. Expect pure legacy behavior.

Wiring (`internal/platform`, `projectionbase`, `ledgerwitness`):

15. `FetchPayload`, `EventsPage`, `ValidateEventPins` and checkpoint
    evidence each succeed with the artifact in a segment only, in legacy
    only, and in both, with identical results.
16. A corrupt segment surfaces as an error naming the sequence or digest at
    each of those sites; there is no skip.
17. A tombstoned computer with segments is refused with
    `ErrComputerTapeDeleted` by `FetchPayload`, `EventsPage` and checkpoint
    evidence, and by the head CAS; the segment files are not opened.
18. Tape sweep, dry-run and apply: a chain whose event is segment-only and
    whose payload is legacy-only keeps that payload, with no refusal. A true
    orphan beside it is still listed and deleted. A corrupt segment makes
    the sweep refuse. A destroyed computer with segments is not deletable.
19. Artifact GC in active mode leaves `tape-seg/` and every segment file
    untouched.
20. `DiskEventSource` target-head replay and `FetchPayload` read segments;
    `ReadDir` mode refuses once segments exist and still works without
    them.
21. `ledgerwitness.Walk` finds a record through a segment-resident event and
    payload and fails closed on a corrupt one.
22. The no-writer guard: no non-test source outside `internal/tapestore`
    references `SegmentWriter`.
23. Zero segments: the existing test suites of the touched packages pass
    unchanged.

### 9.5 Ceremony

- **Conjecture delta.** A1 adds no storage behavior. If the design of
  section 4 is right, a reader that resolves segment-first with the checks
  above gives the same bytes and the same errors as today for every digest
  that exists today, so A2 can move writes without touching a reader.
- **Admissible evidence.** The failure-mode tests above, the unchanged
  passing suites, and the race run. Not claimable from A1: any staging
  behavior, the cost of lookups over many segments, btrfs fsync cost.
- **Heresy delta.** discovered: the ledger-witness and `projectionbase`
  readers duplicate the tape layout knowledge (`sha256/<ns>/<digest>`); A1
  collapses them onto one library. introduced: none intended. repaired: none
  claimed.
- **Standing questions.** Settled by: owner notes §26. Single state
  authority: unchanged (platform head and append receipt; segments are
  storage). Deletion citers: none, A1 deletes nothing. Fate-sharing and
  restart durability: no new process state; the reader caches only
  immutable index headers keyed by file size and mtime. No-SSH: the reader
  keeps hit counters (segment, legacy, not found, corrupt) that A2 exposes
  on the diagnostics endpoint.

### 9.6 As built, and what A2 needs

A1 landed as planned in 9.1 to 9.4, with these differences:

- The reader returns `ErrDigestMismatch` for a legacy file that does not hash
  to its name, as the old code did in substance; callers keep their error
  prefixes and gain `errors.Is` on the cause.
- The tape sweep's `walkTapeChain` treats only `ErrCorrupt` as a stop. Every
  other unreadable event (missing, legacy digest mismatch, I/O error) still
  sets `EventReadable=false`, which is the old behavior and refuses the
  payload sweep.
- The orphan dry-run now keeps the first refusal reason. Before, a chain
  that failed to read was reported as "append receipts do not match", which
  hid the cause.
- The offline `ReadDir` mode of `DiskEventSource` refuses a computer with
  segments, as section 4.3 asked.
- The reader keeps hit counters (`Reader.Stats`). They are not yet exposed
  on a diagnostics endpoint.

What A2 needs, in order:

1. The production writer, following appendix A.5: a per-computer lock in
   `tapestore`, open-segment recovery on start (truncate a torn tail, refuse
   anything else), rotation at 64 MiB or 50,000 records, one fsync per event
   before the head CAS, behind `PLATFORM_TAPE_SEGMENTS=off|write`, default
   off.
2. `pin` / `writeImmutable` routed by flag, with the collision check across
   both stores (same bytes: no-op; different bytes: error). The legacy
   path's mtime refresh on reuse (the tape sweep's grace rule) has no
   per-record analogue in a segment: a pin whose CAS never lands stays in the
   segment as dead weight until the segment is dropped. Bound it and state it
   in the A2 mission.
3. `cmd/tape-explode`, and the runbook gate: no binary rollback below A1
   once any segment holds data.
4. Counters on the diagnostics endpoint: `Reader.Stats`, plus files created
   per 1,000 writes on `writeBlob` and the segment writer, as the E2E
   assertion in section 4.5 needs.
5. The destroyed-computer sweep for segments: archive the computer's whole
   `tape-seg/<id>/` directory, tombstone, delete. Until then A1 refuses.
6. A measurement of lookup cost over many sealed segments (directory listing
   per read, one index read per segment per process) before any cache is
   added; the fanout already bounds one probe to a small bucket.
7. The byte-identical repack tool and the soak of section 6.1, which turn
   the 9M legacy files into segments. It can be written against
   `SegmentWriter`, `Reader.Scan` and `Reader.ReadLegacy` as they are.

## Appendix A. Segment format v1, as built in A1

Implementation: `internal/tapestore` (`format.go`, `reader.go`, `scan.go`,
`writer.go`). Tests: `internal/tapestore/tapestore_test.go`. This appendix is
the contract A2 writes to and every reader reads. Section 4.1 is the design
sketch; where the two differ (the segment header, the fixed-width index with a
fanout table, the crc coverage), this appendix is the built format.

### A.1 Layout

All integers are big endian. Paths are relative to the artifacts root.

```text
tape-seg/<computer_id>/seg-NNNNNNNN.seg     append-only; the segment
tape-seg/<computer_id>/seg-NNNNNNNN.idx     written at seal; sorted index
tape-seg/<computer_id>/seg-NNNNNNNN.idx.tmp writer temp file; readers ignore it
```

`NNNNNNNN` is the segment number, zero-padded to eight digits (more digits
once it exceeds 99,999,999). Only the canonical spelling names a segment.
`<computer_id>` must match `[A-Za-z0-9._:-]{1,200}`, must not start with `.`,
and is never `.` or `..`. The reader and the writer share this rule; an id
that fails it has no segments and reads legacy only. Any other file in the
directory is ignored.

A segment is **sealed** when `seg-N.idx` exists, and **open** otherwise. The
highest-numbered segment of a computer is the only one that may be open and
carry a torn tail.

### A.2 Segment file

```text
header, 32 bytes
  0   magic        "CHTS"
  4   version      u8 = 1
  5   reserved     3 bytes = 0
  8   segment no.  u64
  16  computer     first 12 bytes of SHA-256(computer_id)
  28  crc32c       u32 over bytes 0..27
records, back to back, from offset 32
  0   magic        "CHRC"
  4   version      u8 = 1
  5   namespace    u8: 1 event, 2 payload, 3 receipt
  6   digest       32 bytes, SHA-256 of the body
  38  length       u32, at most 64 MiB (67,108,864)
  42  crc32c       u32 over bytes 4..41 followed by the body
  46  body         length bytes
```

CRC32C is the Castagnoli polynomial. The CRC detects a torn or damaged
write; the digest is the identity. The body of an event is the canonical
event JSON, of a payload the payload bytes, of a receipt the canonical
`PinReceipt` JSON. For all three, `digest = SHA-256(body)`, which is the legacy
file name (`pin-receipts/<digest>.json` for receipts). The key of a record is
`(namespace, digest)`; the same bytes in two namespaces are two records. A
segment holds a key at most once; a digest may appear in more than one
segment, with the same bytes by construction.

Rotation thresholds for A2 are 64 MiB or 50,000 records (`DefaultSealBytes`,
`DefaultSealRecords`). The reader does not depend on them.

### A.3 Index file

```text
header, 40 bytes
  0   magic        "CHIX"
  4   version      u8 = 1
  5   reserved     3 bytes = 0
  8   segment no.  u64, must equal the file name's
  16  segment size u64, size in bytes of seg-N.seg at seal
  24  count        u32, number of records
  28  computer     12 bytes, as in the segment header
fanout, 1,024 bytes
  256 x u32: fanout[i] = number of entries whose digest's first byte <= i
entries, count x 45 bytes, sorted strictly by (digest, namespace)
  0   digest       32 bytes
  32  namespace    u8
  33  offset       u64, offset of the record header in the segment
  41  length       u32, record body length
trailer, 4 bytes
  crc32c over every preceding byte of the file
```

A lookup reads the bucket `[fanout[b-1], fanout[b])` for the digest's first
byte `b`, which is a few hundred entries at most, and binary-searches it. The
fixed width is what allows the index to be read in place without loading it.

### A.4 Rules every reader applies

1. **Hash what you return.** After any CRC check, recompute SHA-256 of the
   body and compare it with the record's digest field and with the digest the
   caller asked for. A mismatch is corruption.
2. **Sealed segments are strict.** Opening one checks the index size,
   trailer CRC, magic, version, segment number, computer hash, that the
   fanout is monotonic and describes the entries, that the entries are
   strictly sorted, that the segment file size equals the sealed size, and
   the segment header. Each lookup then checks that the record at the
   indexed offset has the indexed namespace, digest and length, a valid CRC,
   and a body that hashes to the digest. Any failure is `ErrCorrupt`.
3. **An index with no segment file is corruption.**
4. **Open segments are scanned read-only.** Records are accepted in order
   while each is whole and its CRC holds. At the first record that is not,
   the reader decides between a torn tail and corruption. It is a torn tail,
   and the rest of the file is treated as absent, only if the segment is the
   highest-numbered one, the unreadable region is no larger than one maximum
   record, and no whole, CRC-valid record starts anywhere after it. Anything
   else is `ErrCorrupt`. A record that is whole and CRC-valid but has an
   unknown version or namespace is `ErrCorrupt`, never a torn tail.
5. **Unknown versions and bad magics are errors.** An older binary must not
   ignore a newer layout.
6. **Search order.** Newest segment first. The first segment that holds the
   key answers. A segment that fails a check before an answer is found makes
   the lookup an error, including a lookup that would have been a miss. Only
   a clean "no segment holds this key" falls through to the legacy file.
7. **Segments are per computer; legacy files never were.** Segments are
   looked up in the requesting computer's directory only. The legacy file is
   `sha256/computer-event/<d>`, `sha256/computer-event-payload/<d>` or
   `sha256/pin-receipts/<d>.json`, found by digest alone as it always was,
   and hash-checked the same way. `projection-base` objects are never
   segmented.

### A.5 Writer rules (A2 will implement these in production)

The A1 `SegmentWriter` is test and offline only. A2's writer must:

1. Create the segment with `O_EXCL`, write the header, and append whole
   records with one `write` each.
2. `fsync` the segment before the head CAS that makes its records canonical
   (one fsync per event covers the event's records).
3. Seal by `fsync` of the segment, writing the index to `seg-N.idx.tmp`,
   `fsync` of it, `rename` to `seg-N.idx`, and `fsync` of the directory.
   A crash before the rename leaves an open segment, which the reader scans.
4. On open, scan the last segment, truncate a torn tail at the first bad
   record, and refuse to start if the damage is not a torn tail.
5. Serialize writes per computer, and never modify a sealed segment.

### A.6 What A1 does not do

- No production code writes a segment. A guard test fails if a non-test
  source outside `internal/tapestore` names `SegmentWriter` or
  `CreateSegment`.
- The byte-identical repack tool is not built (section 9.1). Its pieces are
  there: `SegmentWriter` to write, `Reader.Scan` and `Reader.ReadLegacy` for
  the census of section 6.1 step 4.
- The destroyed-computer tape sweep refuses a computer that has segments.
- Lookup cost over many segments is not measured. Each lookup lists the
  computer's directory and reads the fanout-narrowed bucket of each sealed
  segment newest first; a sealed segment's index is read and validated once
  per process.
