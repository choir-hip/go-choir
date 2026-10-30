# One event volume feeds two leaks: design options for projection_batch_recorded (2026-10-19)

Date: 2026-10-19. Design only: no runtime code changed, nothing pushed,
nothing run on production. Everything measured comes from the three problem
records cited below; everything derived is labeled as derived; anything that
needs an experiment is labeled a hypothesis.

Mutation class of this doc: green. Mutation class of every option below:
red unless stated (the tape write path, evidence, and the content witness are
protected surfaces).

Inputs (read first):

- [`platform-artifact-small-file-growth-2026-10-19`](../problems/platform-artifact-small-file-growth-2026-10-19.md)
  (host: 111 GiB of small files, 4.02 files per tape event).
- [`paper-disk-full-again-history-growth-2026-10-19`](../problems/paper-disk-full-again-history-growth-2026-10-19.md)
  (guest: 48 GiB Dolt store, about 6.8 GiB live, the rest history; the
  offline full GC was aborted for memory).
- [`guest-dolt-storage-growth-2026-10-15`](../problems/guest-dolt-storage-growth-2026-10-15.md)
  (about 240 KB of journal and about 1.5 MB of pinned history per 100-event
  commit) and
  [`guest-store-history-bloat-and-memory-shape-2026-10-09`](../problems/guest-store-history-bloat-and-memory-shape-2026-10-09.md).

## Answer in brief

1. The volume is real and it is one stream: every durable guest write is
   exactly one `projection_batch_recorded` event, and almost every event
   carries exactly one op because each caller writes alone. About 1.07 ops
   per event (derived from the paper's row counts).
2. The same stream feeds both leaks, but the **amplifiers differ**, so no
   single lever fixes both:
   - Host: the cost is **per file** (4 files per event, metadata inline).
     It scales with event count and with nothing else.
   - Guest: the cost is **per randomly keyed row** in a prolly tree. Every
     event inserts a hash-keyed row into `computer_event_index`, and most
     events also insert a hash-keyed row into `og_objects` or
     `run_memory_entries`. Each insert rewrites a leaf and its two parent
     chunks. That is about 10 KB of new chunks per table per insert, and
     history keeps all of it.
3. Group commit (fewer events) is the only lever that acts on both leaks. It
   is also the smallest. But it does **not** cut the guest's `og_objects` and
   `run_memory_entries` inserts, which are one per op. So it cuts host files
   by about k and guest history by much less (hypothesis: 1.3x to 2x).
4. Ranked recommendation: (a) group commit first, as the shared lever; (d)
   segment files next for the host, because it makes host cost independent of
   event rate; (b) plus a sequential index key and (c) time-batched commits
   for the guest; (e) retention last, because it touches the "genesis replay"
   invariants. Stage 0 is an offline measurement that settles the guest-side
   numbers before any red change.

## 1. Where projection_batch_recorded comes from

### The single funnel

All durable writes go through one function:
`projectionTape.appendOps` (`internal/store/projection_tape.go:97-135`). It
builds one `ProjectionBatch` (`ProjectionBatchV2`, one fresh UUIDv7 event id,
`Ops: ops`) and calls `AppendNewPayload` with idempotency key
`"projection-batch:" + eventID` (`:104-130`). There is no queue, no merge, no
timer. A call is one event.

The three entry points into the funnel:

- OG mutations: `interceptOG` (`projection_tape.go:56-84`) wraps whatever one
  `PutObject`, `PutEdge` or `PutBatch` call carries. The interception points
  are `internal/objectgraph/dolt_store.go:497-507` (object),
  `:745-748` (edge), `:832-848` (batch, after the conditions check) and
  `:1541-1556`. A `PutBatch` carries many ops in one event already; most
  callers do not use it for more than one object.
- Run memory: `AppendRunMemoryEntry` (`internal/store/run_memory.go:20`)
  appends exactly one op per entry (`:84`).
- Desktop and self-development: `appendDesktop` (`projection_tape.go:86-95`,
  from `desktop_live.go:73`) and `AppendProjectionOps`
  (`projection_tape.go:49-54`, used at `internal/selfdev/operations.go:547`
  and `:585`).

### What triggers each event

Per write, not per run and not per Texture turn. The largest producer is the
runtime event log: `Store.AppendEvent` (`internal/store/store.go:1929`) calls
`AppendEventOG` (`graph_store.go:1168`), which is one `choir.event` object,
which is one batch event. `AppendEvent` is called from about twenty sites
(for example `internal/agentcore/runtime.go:2355` and `:4570` `emitEvent`,
`runtime_persistence.go:161`, `product_events.go:49`,
`textureowner/texture_agent_revision.go:795-902`, `agentcore/tools_desk.go:1124`).
Each run state change, tool step and product event is one event. Run memory
adds one event per entry (`agentcore/run_memory.go:157, 287, 378, 460`,
`rlm_reduce.go:537, 600`). Lifecycle, commitment, wake and source-ref writes
add the remainder.

When the tape is bound, `AppendEvent` releases `eventMu` before the append
(`store.go:2009-2011`), so concurrent callers do reach the appender
together. They then serialize on the appender mutex (`appender.go:338-339`).
The caller blocks until the event is durable. Callers on the agent hot path
(`emitEvent`) therefore wait for the whole append.

### What the payload holds

A `ProjectionBatch` JSON with `ops[]`. Each op is `{kind, canonical_id,
table, body}` with the full row snapshot as `body`
(`computerevent/projection_batch.go:65-70`). A `choir.event` op is the
complete event record (payload and metadata) as a JSON-escaped body; a run
memory op is the complete row (`RunMemoryEntryProjection`, `:72-90`). So the
payload **is** the write and also the replay source. Measured average: 5.5 KB
per payload file (host record).

### Measured and derived rate

From the host record: 337k to 365k events per day on 10-14 to 10-16, then
90k to 126k per day since 10-17 (cause of the drop not traced). That is about
4.0 per second at the peak and about 1.0 to 1.5 per second now. Bursts reach
27 in one second. On the paper, 358,503 batch events on 10-15 were 207,658
`choir.event` objects (58%), 111,636 run-memory entries (31%) and 5% other;
those shares are the paper's 10-15 counts, not today's.

Derived from the paper's current live rows: 1.36M events, `og_objects` 1.02M
rows, `run_memory_entries` 0.425M rows. That is 1.07 ops per event, which
confirms that nearly every batch has one op.

Serialized service time: the busiest second held 27 events, so one append
takes at most about 35 ms in a burst. Each append is four sequential host
round trips (`cas.Head`, `pinPayload`, `PinEvent`, `CompareAndSwap`;
`appender.go:338-397, 610-690`) plus two guest SQL transactions (see
section 3). Inference, not measured: with the arrival rate below the service
rate most of the time, a queue-while-busy scheme alone (no timer) will find
few waiting ops, so k from "natural" coalescing is probably 1.1 to 1.5. The
measured windows (2.95 events per 250 ms, 6.6 per 1 s, 10.6 per 2 s) are the
k that a time window would capture. This is the central tension of option
(a): real k needs a window, and a window adds caller latency (section 4a).

## 2. What the host writes per event

Code: `appender.go:338-397` (`AppendNewPayload`) and `:610-690`
(`appendLocked`) call the pin service. Host side:
`internal/platform/event_artifacts.go:46-60` (`pinPayload`), `:31-44`
(`PinEvent`), `:62-90` (`pin`), `:223-239` (`writeImmutable`), and
`internal/platform/service.go:882-898` (`writeBlob`).

Per event, in order:

1. Payload file: `sha256/computer-event-payload/<digest>`.
2. Payload pin receipt: `sha256/pin-receipts/<digest>.json`.
3. Event file: `sha256/computer-event/<digest>` (the canonical event bytes).
4. Event pin receipt: `sha256/pin-receipts/<digest>.json`.
5. One row in platform Dolt `computer_event_append_receipts`, with the signed
   head receipt (`platform/computer_events.go:275`), plus one
   `computer_event_heads` update. The platform Dolt commit is debounced by
   `doltbatch` (`platform/store.go:454-459`, `internal/doltbatch/committer.go`),
   so it is not a per-event commit.

That is 4 files; the measured 4.02 is the same number plus about 0.8% orphan
pins from refused appends. Why exactly two receipts: the event commitment
binds one payload pin receipt per artifact ref (`AppendNewPayload` passes one
payload receipt digest into `ComputeRequestCommitment`), and the event itself
is pinned separately. Both receipts carry random UUIDv7 ids and issue times,
so they are real signed evidence, not re-derivable. Each file goes through
open-for-read (miss), `MkdirAll`, temp write, rename, and none is fsynced.
Receipts are read exactly once, by `ValidateEventPins` at CAS time.

## 3. What the guest stores per event

### computer_event_index

Schema: `internal/store/store.go:433-463`. Primary key is `event_digest`
(a SHA-256, so random). There are also unique keys on `(computer_id,
sequence)` and `(computer_id, idempotency_key)`, and an index on `(computer_id,
status, sequence)`. The first two are monotonic for batch events (sequence;
UUIDv7 in the key), so they are cheap. The primary key is the random one.

Writes per event:

- `Prepare` (`internal/store/computer_events.go:33-70`): INSERT with
  `event_json` (`event.CanonicalBytes()`, line 40) and
  `payload_pin_receipt_digests_json`. Marks Dolt history dirty.
- `DryRunProjectionBatch` (`:96-117`): applies the batch in a transaction and
  rolls it back (a second projection pass, journal cost only).
- `finalizeBatch` (`:119-242`): updates the head row, then UPDATE of the same
  index row with `event_head_receipt_json`, `event_head_receipt_digest` and
  `finalized_at`, and applies the ops to `og_objects` / `run_memory_entries`
  in the same transaction.

So each event is 2 SQL transactions plus a rolled-back one.

### Does event_json duplicate the tape?

Yes, exactly. `Prepare` requires `EventArtifactDigest == EventDigest`
(`computer_events.go:39`), and the event file on the host is the same
canonical bytes. The row also stores the head receipt JSON, which the
platform keeps verbatim in `computer_event_append_receipts.event_head_receipt_json`.
Measured on 10-15: `event_json` 542 MB plus receipt JSON 523 MB for 359,911
rows (about 1.5 KB each). Today the table is 1.36M rows and 3.84 GiB (about
2.8 KB a row), consistent.

Readers of `event_json` (`EventByDigest`, `EventByIdempotency`,
`LatestComputerEventByIdempotencyPrefix`, `computer_events.go:306-370`, and
`computer_event_recovery.go:16`) are used by `agentcore/platform_update.go`
and `rematerialize.go:385, 496` for lifecycle, platform-update and
accepted-effect events. `rematerialize.go:496` reads the **head** event by
digest, and the head can be a batch event. So batch rows cannot simply lose
their body; they need a fetch-by-digest fallback.

The content witness covers this table. `projectionbase/witness.go:24` and
`agentcore/replay_completeness.go:493` hash `computer_event_index` with only
`prepared_at` and `finalized_at` ignored. Any change to the stored columns
changes every witness. This is the main invariant for option (b).

### Dolt commits in the projection path

The commit is count-based and inside `finalizeBatch`:
`defaultFinalizeCheckpointEvery = 100` (`store.go:101-104`), counted in
**events** (`computer_events.go:232-241`), via `commitDoltCheckpoint`
(`store.go:197-225`, `CALL DOLT_COMMIT('-Am', ...)`). Replay mode
commits every 500 events or 60 s (`appender.go:149, 265`). `GetHistory`
forces a commit before a Texture history read (`texture.go:1326`).
Nothing else in the live path commits. (Contrast: the platform store uses a
time-debounced committer, `internal/doltbatch`; the guest does not.)

### History bytes per event

Measured, from the records: about 1.5 MB of pinned superseded chunks per
100-event commit (10-15 record), so about 15 KB per event. The paper's whole
lifetime gives about 31 KB per event (about 40 GiB of history over 1.36M
events), and the 6 GB/day on 10-15 over 360k events gives about 17 KB.
Journal garbage is another 240 KB per event but is collected by GC.

Why the history is that big (derived, hypothesis until measured): the
three big tables have random primary keys (`event_digest`, `canonical_id`
ending in a content hash, random UUID `entry_id`), so 100 inserts into a table
of about a million leaf chunks touch about 100 distinct leaves, 100 distinct
level-1 parents and most of about 280 level-2 chunks. Rewritten chunk bytes
per insert: leaf about 3 KB, level-1 about 4 KB, level-2 about 3 KB, about
10 KB per table. Two to three random tables per event gives 20 to 30 KB,
matching the 15 to 31 KB measured. Two consequences:

- Commit cadence helps only the upper levels. At 10,000 inserts per commit,
  level-1 sharing rises and level-2 almost vanishes, so about 6 KB per table
  insert remains: roughly 1.5x to 2x less, not 100x.
- Sequential keys help much more. A monotonic key touches the rightmost leaf
  and path only: about 3 KB per insert in total.

## 4. Options

Common invariants for every option that touches the tape:

- Event chain hashes: the event digest, `previous_head`, `request_commitment`
  and the head receipt must be computed exactly as today for whatever events
  exist. No option rewrites or re-signs a past event.
- Restart rule: a crash closes open work and never resumes it; only a planned
  restart with a marker resumes. In-memory queues must not become a resume
  mechanism. Ops queued at a crash are lost together with their callers'
  unreturned calls; nothing acknowledged is lost.
- Replay determinism: applying the tape from genesis (or a base) to a head
  must reproduce the live content witness, including op order inside and
  across batches.
- Content witness: per-table digests must remain equal between live and replay
  within one build, and the recorded witnesses of past bases and checkpoints
  must remain verifiable (versioned, not silently redefined).
- Evidence integrity: bytes and digests of event, payload and receipt are
  never altered; reads stay digest-verified.

### (a) Group commit of projection batches

What: merge ops from several callers into one `ProjectionBatch`. The format
already allows it (`Ops []ProjectionOp`, `projection_batch.go:159-215`); the
replay and finalize code applies a list in order. No format or replay change.

Two forms, which compose:

- (a1) Leader/follower queue in `appendOps`. A caller on an idle queue sends at
  once. Callers arriving while an append is in flight join the next batch.
  No timer. Added latency zero. Expected k is small (1.1 to 1.5, inference
  from section 1).
- (a2) Windowed coalescing for **unguarded telemetry ops** only (`choir.event`
  objects and run-memory entries, about 89% of events). A window of W ms
  before flushing. Measured k for W = 250 ms / 1 s / 2 s is 3 / 6.6 / 10.6
  (host record; a lower bound, since arrivals were already serialized). The
  cost is caller latency up to W on `emitEvent`, which is on the run hot path
  and called sequentially within a run. A 250 ms window on a run that emits
  100 events adds up to 25 s. That is not acceptable unless callers do not
  block. Making telemetry non-blocking (enqueue and return) changes the
  durability contract and needs a read-your-writes barrier for event
  replay-from-store reads. Owner decision; do not take it in the first slice.
- (a3) Explicit batch scopes at call sites that already emit adjacent ops
  (for example an event plus a run-memory entry for the same step). Zero
  added latency, deterministic k. The gain is unknown until the adjacency is
  measured (stage 0).

Mutation class: red (tape write path). Protected surfaces: computer events,
pin receipts, projection, replay. Not touched: checkpoint authority, route
projection, auth.

Specific invariants and failure modes:

- A bad op refuses every caller's write at the dry-run
  (`appender.go:364`). On failure, split the batch and retry callers
  individually, and return per-caller errors.
- Guarded ops (`ExpectedStates`, `RequireRevision`, `CreateOnly`; `project.go`
  `:481-504`) and conditional OG batches (`dolt_store.go:832-848`, conditions
  evaluated before the intercept) must not be merged across callers. They flush
  alone or in their own batch.
- Op order inside a merged batch is arrival order. Within a caller the order is
  unchanged.
- Batch cap by ops and bytes (replay page response cap is 8 MiB,
  `event.go` `EventReplayMaxResponseBytes`; payload cap 64 MiB).
- The finalize counter counts events, so with k ops per event the 100-event
  Dolt commit becomes a 100k-op commit. That is an accidental form of (c) and
  should be made explicit (an ops-based or time-based bound) so a quiet
  system still commits.

Expected reduction (derived):

- Host files, receipts and platform Dolt rows: 1/k for the merged class.
  k = 3 gives about 2.5x overall, k = 6.6 about 5x (the telemetry share is
  89%; the rest stays at 1).
- Guest `computer_event_index` inserts and prepare/finalize transactions:
  1/k. `og_objects` and `run_memory_entries` inserts: unchanged, but they
  commit k times less often per op, which helps their upper levels.
  Hypothesis: guest history 1.3x to 2x less; journal 2x to 4x less.

Rollback: flag off (default). Merged events already on the tape are normal
events and replay as before.

### (b) Store the event once: reference by digest

What: do not write `event_json` and `event_head_receipt_json` into
`computer_event_index` for `projection_batch_recorded` rows. Keep the digests
(`event_artifact_digest`, `event_head_receipt_digest`, pin digests) and fetch
the body from the host by digest when `EventByDigest` and friends need it
(`FetchPayload`-style, digest-verified). Lifecycle and platform-update rows
keep their bodies (under 0.4% of rows).

Mutation class: red. Protected surfaces: the content witness (the table is
in it), replay and rematerialize, Trace/evidence reads.

Invariants:

- The witness. `event_digest` already binds `event_json`, so dropping the
  body from the witness loses no strength, but the witness value changes for
  every computer. This needs a versioned witness (add `event_json` and
  `event_head_receipt_json` to `IgnoredContentColumns` under a new witness
  version), verification of old bases under the old version, and a build in
  which live and replay agree. A mixed fleet is the hazard.
- `rematerialize.go:496` needs the head event body: the host fetch must be
  available at boot.
- Fate-sharing: a guest read that used to be local now needs the platform.
  Acceptable only where the caller already talks to the platform.

Expected reduction: the index table drops from about 2.8 KB to about 0.5 KB a
row (about 3 GiB of the paper's 3.84 GiB) in the live set, and the per-insert
leaf cost of that table falls with it. Fewer history bytes, but the table
still has a random key, so the upper-level cost stays. Estimate: live raw
-2.5 to -3 GiB on the paper; history -10% to -20% (hypothesis).

Rollback: stop omitting the body for new rows. Old rows are valid either way.
Combine with the sequential-key change below only if the experiment in stage
0 shows the key matters.

### (b2) Sequential primary key for the index (companion to b)

Not on your list, but the analysis above makes it the strongest guest-only
lever: re-key `computer_event_index` to `(computer_id, sequence)` so the
insert path is a monotonic append. It changes a table schema on a 1.36M-row
table, so it needs a migration (copy, swap, witness equal) and shares (b)'s
witness-version dependency. Expected: that table's history cost falls from
about 10 KB to about 3 KB per event (derived), i.e. about 7 KB of the
15 to 31 KB. Class red (guest store schema, witness). Only attempt after
stage 0 confirms the model.

### (c) Commit the guest store less often

What: replace "every 100 finalized events" (`store.go:101-104`,
`computer_events.go:232-241`) with a time-debounced commit like the platform's
`doltbatch` committer, for example one commit per 5 to 15 minutes plus the
existing forced commits (`GetHistory`, planned shutdown, replay boundaries).

Mutation class: orange (guest store behavior; not the tape, not a witness
input; `HASHOF('HEAD')` is audit-only per the 10-15 record). Protected
surfaces: none directly; the commit boundary matters for crash recovery only
in replay mode, which keeps its own cadence.

Invariants: nothing reads Dolt history in the live path (10-09 record).
Durability does not depend on the commit (the SQL transaction is durable in
the journal, and the tape is the authority). Crash restart replays from the
tape or resumes the local store as today.

Expected reduction: only the upper-level share, so 1.5x to 2x less history
(hypothesis, section 3). It does not touch the number of leaves rewritten.
Stand-alone it will not stop the disk filling; it buys time.

Rollback: restore the count-based cadence (one constant).

### (d) Per-computer segment files on the host

What: the plan in the host record, "Option (b)" there: append-only
per-computer segments with a sealed index, dual-read, backfill, soak, then
delete legacy files; one library for put/get by digest, with fsync grouped
per event.

Mutation class: red (evidence store write path). Protected surfaces: pin
receipts, event and payload artifacts, replay readers including
`cmd/checkpointd`, `cmd/ledger-witness`, `cmd/choir-rebuild-base`.

Invariants: bytes and digests unchanged; reads digest-verified; the chain
stays addressable by digest; the legacy files stay until the census proves
every digest resolves from a segment with equal bytes; fsync before ack
(today `writeBlob` never fsyncs).

Expected reduction: host file growth to about zero and the 9.07M legacy files
gone after backfill (about 8 to 11 GiB logical metadata, 16 to 22 GiB on disk,
estimated in the host record). It does **not** reduce the guest at all and
does not depend on event rate. It composes with every other option.

Rollback: dual-read and unchanged legacy files until the census; flag for the
write path.

### (e) Retention of old projection events after a compact base

What: with a verified base at sequence S (the projection-base machinery,
`internal/projectionbase`), archive or delete the events and payloads before S
and replay only the tail. On the guest side, the equivalent is rebuilding the
store from a base plus the tail instead of keeping history.

Mutation class: red, and the heaviest. Protected surfaces: the whole chain.
It contradicts today's "genesis replay is the state-completeness check, the
chain links to genesis, the witness walks to a record" (host record,
Retention section; `projectionbase/source.go:51-107` walks to genesis). It
needs a doctrine decision: a signed base becomes a new trust anchor.

Expected reduction: the host's old-event files (and the guest's history) up to
the retained window, but only once. It does not slow growth. Never truncate a
live chain; archive first. 13.6% of tape events belong to computers with no
ownership record, which is the safe first target and is a separate policy
(owner decision), not this option.

Rollback: archives are kept; nothing is deleted until an owner decision.

### Which options compose

- (a) and (d): fully independent. (a) cuts event count; (d) removes the
  per-file cost. After both, host cost is bytes only (about 1 GB/day logical
  before compression at today's rate).
- (a) and (c): (a) already increases ops per commit; a time or ops bound makes
  that explicit. Do together.
- (b) and (b2): same migration window and same witness version. Do together.
- (a) and (b): independent; (a) reduces how many rows (b) has to shrink.
- (e) depends on a trustworthy base; do after (d) and after the guest
  rebuild-from-base path exists. It is also the only option that shrinks the
  existing 48 GiB; none of (a) to (d) reclaims what already exists. That is
  the job of the compaction work already open in the paper record (row copy
  into a fresh database, then witness compare).

## 5. Staged plan

### Stage 0: measure, no behavior change (green, orange counters)

Goal: settle the guest-side numbers before touching red surfaces.

1. Offline Dolt experiment on a scratch store (a Mac or a scratch dir; not the
   live image): insert the same synthetic rows with (i) random vs sequential
   primary key, (ii) 100, 1,000 and 10,000 rows per commit, (iii) with and
   without `event_json`, with a default GC every simulated 35 minutes.
   Record store bytes per inserted row. Confirms or refutes the 10 KB-per-
   table model, the 1.5x to 2x cadence gain and the value of a sequential key.
2. Counters in the appender (orange, tiny): queue depth on entering
   `a.mu`, ops per batch, op kinds per batch, append latency, adjacency of
   events within the same caller goroutine. Exposed in health. This answers
   how large k is under (a1) and whether (a3) has adjacent ops to merge.
3. Read the run ledgers for the 10-17 rate drop (trace-before-attribute).
4. Decode today's op mix for counts only (kind counts, no content).

Evidence needed: a table of bytes per row per configuration; k distribution;
a dated problem-doc update. Problem Documentation First applies: update the
two problem records with these results before any fix commit.

### Stage 1: smallest safe first slice (red, flagged, default off)

(a1) Leader/follower coalescing in `projectionTape.appendOps`, no timer, merging
only unguarded `choir.event` and `run_memory_entry` ops, with a batch cap, split
on failure, and an ops-or-time bound on the Dolt checkpoint (the (c) piece).
Flag off by default; turn on for staging, then the paper.

Why this is the smallest: no format change, no replay change, no schema change,
no witness change, no host change, one function plus the checkpoint counter.
It cannot lose acknowledged data, and rollback is the flag.

Evidence needed:

- Failing tests first: bad-op split, guarded-op isolation, ordering, cap, crash
  with queued callers (no resume), idle-latency unchanged.
- Staging: files per 1,000 guest writes before and after; k distribution;
  p50/p99 append latency; genesis-replay content-witness equality
  (`projectionbase/witness.go`) on a staging computer that ran with the flag.
- Landing loop per CLAUDE.md: commit, push, CI, deploy identity, deployed
  acceptance proof.

Expected: small k (1.1 to 1.5) from this slice alone; its purpose is to prove
the mechanism and measure. If stage 0 shows adjacent ops (a3), batch scopes
come next and give the real k with no latency cost.

### Stage 2: choose the windowed or non-blocking telemetry form (owner decision)

Only if stage 1 shows k too small. It changes caller latency or the durability
contract for telemetry. Take it through the agentic-consensus panel, per the
No Blocking Asks rule, and record the choice.

### Stage 3: guest store shape (red, with witness version)

(b) plus (b2) after stage 0 confirms them, in one migration window with a
versioned witness, plus the compaction rebuild path for the existing 48 GiB.

### Stage 4: host segment store (red, large)

(d) with dual-read, backfill, soak and census. Then dead-computer retention as
a segment policy.

### Stage 5: retention after a compact base (red, doctrine)

(e), only after the owner decides what a base anchors.

## 6. Standing questions, for the first slice (stage 1)

1. Who settled this? The owner asked for a cause and a permanent solution
   (host record) and for history compaction (paper record). The host record's
   "group commit first" is an agent recommendation, not an owner settlement;
   this slice is a proposal until ratified. No settled decision names a tape
   write shape.
2. Conformance with settled decisions: no new store, no Dolt for tape bytes,
   no change to the two-store taxonomy. The tape stays the sole durable write
   path ("Production autoputer must bind").
3. What references what is deleted? Nothing is deleted. The only removal is the
   one-op-per-event assumption; grep callers of `appendOps`,
   `AppendProjectionOps`, and anything that counts events as ops (the
   `finalizeSinceCheckpoint` counter and any metric or test that assumes one
   op per event) before coding.
4. Who consumes it in production? The guest finalizer, boot replay, the
   offline rebuilder (`projectionbase/source.go`), the ledger witness
   (`ledgerwitness/walk.go:93`), and the checkpoint authority. All read the
   batch as a list.
5. Single authority: the platform event head is the authority; the guest
   projection follows it. The queue is process-local and never authoritative.
6. Artifact-verified success: files-per-1,000-writes measured on the host,
   plus a genesis-replay witness equal to the live witness. Narrated success
   is not enough.
7. Fate-sharing: unchanged read path. The write path gains no new dependency;
   a failed merged append fails all merged callers, which then retry
   individually (so no new coupling between unrelated callers).
8. Restart: the queue is lost on a crash and its callers never returned, so
   nothing acknowledged is lost. A crash restart closes work as
   "interrupted by a restart"; a planned restart drains the queue first. No
   boot-time resumer is added.
9. No-SSH operability: counters (queue depth, k, latency) in the health
   surface and the flag as runtime configuration, so staging proof does not
   need a shell.
10. Registry hygiene: no definition created; when a mission goal file is
    authored, update `docs/ACTIVE.md`, `docs/mission-graph.yaml` and
    `docs/doc-authority-manifest.yaml` together.

Ceremony for stage 1 (red): conjecture "most unguarded telemetry events arrive
close enough that coalescing cuts event count without added latency";
protected surfaces as above; admissible evidence as in stage 1; rollback the
flag; heresy delta below.

## 7. Not verified

- The amplification model for guest history (random-key path rewrite). It
  matches the 15 to 31 KB per event measured, but no one has measured bytes per
  insert under controlled key order and commit cadence. Stage 0.
- k under (a1). Inferred from burst service time; no queue-depth data exists.
- The share of unguarded ops today (89% is the paper's 10-15 count).
- That `event_json` and receipt JSON are the only large index columns
  (`SUM(LENGTH)` on 10-15 says yes).
- That nothing else consumes `computer_event_index.event_json` outside the
  readers listed in section 3 (callers were grepped in Go code only, not in
  scripts or the runbooks).
- Whether default GC promotes intermediate commit versions into the old
  generation exactly as assumed (the 10-15 record says it promotes everything
  reachable from commits).

## Heresy delta

- discovered: the guest commits every 100 **events**, so any change in ops
  per event silently changes commit size; and the platform already has a
  time-debounced committer the guest does not use.
- discovered: `computer_event_index` stores the event body and head receipt
  twice (also on the host and in the platform Dolt), and is inside the content
  witness, so shrinking it is a witness-version change.
- discovered: natural coalescing probably gives little; a real k needs either
  a latency window or a non-blocking telemetry path, and the host record's
  3x to 7x assumed the window without pricing it.
- introduced: none (docs only).
- repaired: none.
