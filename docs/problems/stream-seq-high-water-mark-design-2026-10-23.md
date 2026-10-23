# stream_seq high-water mark: a lease on the tape (design, 2026-10-23)

Date: 2026-10-23. Author: an FP0 design worker. Status: **design, not
built.** It answers the gate in
`docs/problems/stream-seq-seed-scans-the-event-kind-at-boot-2026-10-23.md`
("Gate on F-c deploy 2"), option 1, the persisted high-water mark. Mutation
class of the build: **red** (the event sequence allocator, the guest boot
path, the store schema epoch, a new tape-written object kind). Review: as the
change it designs, tier 2 (boot path, new concurrency, store epoch).

Tags: [V] verified in code at the cited line on `origin/main` `b1abdfb0b`;
[H] hypothesis, not verified.

## The decision

1. The mark is a **lease**: the highest `stream_seq` this computer may hand
   out. The allocator never gives out a number above the lease, so the lease
   is at or above every number the computer has published (stored, queued
   for the sealer, or put on the bus by a failed append).
2. The lease lives **on the tape**, as one singleton `og_objects` object
   (`choir.stream_seq_lease`), written through the projection interceptor
   like any other object. No new table, column, file, index or tape format.
   The tape stays the single authority. The local row is its projection, and
   replay rebuilds it.
3. It is written **ahead**, one block of N numbers (proposed N = 8,192) at a
   time. A refill runs in the background when half the block is used. An
   allocation waits only on a process's first number, or when a whole
   half-block runs out before the refill lands.
4. The seed at boot is **one primary-key read** of the lease, plus the two
   bounded reads that exist today, used only as a cross-check (taken by
   `max`, logged if they exceed the lease). The exact read of the event kind
   never runs while the lease is trusted.
5. A **binary rollback** below the lease is detected by the store schema
   receipt, which every binary's `Open` rewrites: bump `storeschema.Version`
   to 3 and read the prior receipt before `Open` rewrites it. A store last
   opened by an epoch-below-3 binary has an untrusted lease. Its seed takes
   today's path once (windows, then the exact read) and re-leases above it.

## 1. Where the mark lives

### What exists today

- `choir_events` has no index led by `stream_seq`. Its indexes are the
  primary key `(tape_seq, op_index)`, `(run_id, seq)`, `(owner_id,
  stream_seq)` and `(trajectory_id, stream_seq)` [V `event_table.go:70-73`].
  The store-wide seed reads the newest 500 rows by tape position
  [V `event_table.go:235-239`]. That is the newest-carries-max heuristic over
  tape order, not a maximum.
- An exact store-wide maximum over `choir_events` is possible with no new
  state: one seek per owner on `(owner_id, stream_seq)`, written as `ORDER BY
  owner_id DESC, stream_seq DESC LIMIT 1` and not as `MAX()`, which Dolt plans
  as a GroupBy [V the comment at `event_table.go:228-230`]. Owners can be
  listed by skip-seeks on the same index [H: plan unmeasured]. This covers
  only the table half.
- `og_objects` events have no `stream_seq` column. `stream_seq` is in
  `metadata` JSON [V `graph_store.go:1173-1186`]. An index needs a STORED
  generated column (the `og_rec_inbox_seq` pattern, `dolt_store.go:263-266`)
  built by the explicit derived-columns operation
  [V `derived_columns.go:14-19`]. A new column changes `og_objects`' `SHOW
  COLUMNS`, which the content witness hashes
  [V `computerversion/dolt_state_extractor.go:338-375`]. Not cheap (section
  7).
- A new index on `choir_events` led by `stream_seq` would change that table's
  `SHOW COLUMNS` too. Its DDL is frozen for this reason
  [V `event_table.go:38-43`].

So no zero-state read is exact for both halves. The table half can be. The
og half cannot without a schema change on the largest table.

### Why on the tape, as an object

Three candidate homes, judged against the standing questions (5, single
authority; 8, restart):

| Home | Authority | Witness and schema | Survives |
|---|---|---|---|
| **og object through the interceptor (chosen)** | the tape: the lease is a projection op on the signed tape, and the local row is its projection | none: `og_objects` already exists, `projectObject` is kind-agnostic [V `project.go:223-264`], and live and replay both hold the row | restart, crash, replay, base install, rebuild from tape, disk loss |
| sidecar file beside the store marker | a second authority beside the tape | none (outside Dolt) | restart, restore (rematerialize renames only the marker and the workspace, [V `rematerialize.go:224-252`]); **not** disk loss or a new VM, where it falls back to the slow seed |
| new derived table, or a column on `computer_event_projection_heads` | the tape, if kept as a fold in `finalizeBatch` | a column changes a witnessed table's schema hash, so every older checkpoint stops matching [V `dolt_state_extractor.go:338-375`, `projectionbase/witness.go:21-34`]. A new table must be in `IgnoredTables` at every extractor site [V `witness.go:25`, `agentcore/replay_completeness.go:492`], and older binaries call it unknown, so checkpoint publication is refused after a rollback (the og_key_index precedent, F-c design rollback matrix) | restart, replay; restore re-derives it |

The object needs no new structure and keeps one authority. The object is:
kind `choir.stream_seq_lease`, owner `__system__`, one fixed identity key,
body and metadata `{"reserved_through": R, "block": N, "writer_epoch": 3}`.
It carries no role fields, so the vocabulary write guard passes it
[V `vocab_migrate_og.go:921-938` checks role fields only, with no kind
allowlist]. The read is `GetObject` on the canonical ID, one primary-key seek
under the engine mutex [V `dolt_store.go:706-711`].

Why the plain upsert holds the maximum: only one refill is in flight at a
time, and each writes a larger `R`. The appender serialises appends
[V `appender.go:400-403`], so tape order is value order and the last upsert
is the largest. No projector change is needed. This matters: a projector
change would make replay depend on the binary.

## 2. Write cost and atomicity

**Cost.** One tape append (a batch with one object op of about 200 bytes)
per N/2 to N numbers. On every process start there is one more: the first
allocation reserves a fresh block. The steady-state seed cost is one PK seek
plus the two bounded cross-check reads that already exist (the 2,000-object
window, measured 6.6 to 9.3 ms warm in the record, and `choir_events`'
newest 500 by key). There is no per-event or per-batch write.

**Order of operations (the invariant).** A number above `R` is never handed
out before a lease of at least that number has been accepted by the head CAS.
In detail:

- The lease append cannot run under `eventMu`. `Prepare` and `finalizeBatch`
  take `eventMu` under the appender mutex [V `computer_events.go:47`,
  `:140`], and the sealer rule is the same: no flush under `eventMu`
  [V `telemetry_sealer.go:29-37`]. So the refill computes its target under
  `eventMu`, then releases it and appends. A waiting allocator waits with
  `eventMu` released (a condition or channel wait that reacquires it) and has
  a deadline, so an outage fails the append instead of hanging it.
- The sealer's append barrier waits only for enqueued records, not reserved
  slots [V `telemetry_sealer.go:460-472`]. A caller holding a reservation
  while it waits for the lease does not block the lease's barrier [H: needs a
  test with the sealer on].
- `AllocateEventSeq` (D12's folded events) is called before the lifecycle
  batch is built, outside the appender [V `store.go:1524-1528`]. The builder
  must check every caller of `AppendEvent` and `AllocateEventSeq` for a lock
  the lease append also takes [H].

**Crash between the event write and the mark write.** That window does not
exist, because the mark comes first:

- A crash before the lease's CAS: no number above the old `R` was handed out.
- A crash after the CAS but before the local finalize: the lease is on the
  tape, and the next boot replays it before the runtime starts
  [V `autoputer/run.go:747-749`; reconstruct fails unless the local head
  reaches the platform head, `appender.go:1013-1022`].
- If `AppendNewPayload` returns an error after the CAS (a finalize failure,
  `appender.go:763-765`), the refill treats the lease as not written and does
  not raise `R`. A later refill writes a higher `R`. The cost is waste, never
  reuse.

**Gaps.** Allowed, and already present:

- A failed batch leaves a gap [V `store.go:2548-2553`].
- The sealer gap of 512 is applied at every seed with batching on
  [V `store.go:2522-2524`, `telemetry_sealer.go:46-50`].
- Consumers use only the maximum and `> after`: the browser cursor
  [V `frontend/src/lib/Desktop.svelte:1031-1042`] and the table's after-read
  [V `event_table.go:212-215`]. Grep finds no contiguity assumption in Go
  [V: no `StreamSeq ± 1` outside the allocator].
- Waste per process start: at most N. That is irrelevant in an int64, and
  below 2^53 for the browser for any plausible lifetime.
- Under a trusted lease the 512 gap for `stream_seq` is redundant: a queued
  number is at most `R`. Keep it anyway (conservative, costs nothing). The
  per-run gap is unchanged.

## 3. Restore, rollback, replay, rematerialize

**Replay** projects op bodies verbatim and never calls the allocator
[V `appender.go:879-890` Prepare and finalize; `project.go:223-264`]. The
lease row is rebuilt exactly as written.

**Boot after anything** (crash, planned restart, RecoveryResume, Install,
Rebase): the runtime starts only after the tape has been replayed to the
platform head [V `run.go:747-767`, `appender.go:1013-1022`]. Rebase
reinstalls an older base and then replays (W, H]
[V `recoveryplan/recovery_plan.go:32-34`, `autoputer/projection_base.go:173-200`].
So the first seed of a process reads a projection that holds every lease on
the tape, and the tape is never rewound.

**Checkpoint restore and rematerialize, in process.** `RematerializeFromTape`
reconstructs a staged store only through the target head. Later events stay
on the tape [V `appender.go:832-834`, `rematerialize.go:161-163`]. The
staged store is then swapped in and `Reopen`ed [V `rematerialize.go:254`].
The restored row holds an older lease. `Reopen` does not touch
`maxStreamSeq` or `streamSeqSeeded` [V `store.go:1188-1268`], and the design
puts the lease state (`reserved`, the refill flag) on `Store` beside them,
also untouched by `Reopen`. The next refill writes `max(R, maxStreamSeq) + N`,
never a lower value. Builder: add the new fields to the Open/Reopen field
audit the selfdev-operations replay record started.

**Restore, after a restart.** Today the restored projection head is the
target while the platform head is later
[V test `rematerialize_test.go:504-510`]. `appendLocked` refuses appends
while the two differ [V `appender.go:712-717`], and the next boot replays
the events after the target [V `appender.go:1013-1022`]. Those include every
later lease, so the seed is safe. Whether that replay is restore's intended
meaning is not this design's question [H, recorded in the heresy delta]. If a
future restore really rewinds the platform head, it must carry the lease
forward in the new timeline before it serves (append `max(R, maxStreamSeq) +
N` right after `Reopen`). The design asks the builder to add that append now,
so the rule does not depend on restore's semantics.

**Never below a published number.** Every number on the bus, in a cursor, or
held by an `after_seq` client was handed out by an allocator. Every allocator
was bounded by a lease that had passed the CAS. Every such lease is on the
tape, and the projection a seed reads holds it. One case falls outside this:
an older binary allocated (below).

**Binary rollback across the introduction.**

- An older binary ignores the lease row: the projector is kind-agnostic, and
  no older reader asks for the kind. It seeds by D13's path and can allocate
  above `R`.
- After a forward deploy, a stale lease would reissue those numbers. The
  detector is the store schema receipt. Every binary's `Open` rewrites
  `store-schema.json` with its own epoch [V `store.go:964-970`,
  `storeschema/schema.go:27-41`].
- Bump `Version` to 3, and have `Open` read the prior receipt before it
  writes. A lease is trusted only if the prior receipt says 3 or more, or the
  store is fresh.
- An untrusted lease seeds as `max(R, today's seed with its exact
  fallback)`. That is slow at most once after a rollback, and it is logged.
  Then the seed re-leases.
- A new field in the receipt would not work: `storeschema.Read` disallows
  unknown fields, so older updaters would fail to read it
  [V `schema.go:86-89`].
- Side effect of the bump: the updater's `Apply` refuses an epoch-2 release
  on the store. The CI revert, the platform-follow offer and `restorePrior`
  are not fenced [V `schema.go:30-37`], and the receipt catches those.
- D2's store floor (lane `fp1-event-table-d2`, `storeschema/floor.go`) is
  separate: the lease does not raise the floor, because an older binary is
  safe to *run* on a lease store.

## 4. Migration (a store with no lease)

- **Where:** at today's site, the first allocation of the first lease-aware
  process on each computer, under `eventMu` [V `store.go:2395-2399`]. No
  lease row means the seed runs today's `seedStreamSeq` unchanged: the
  bounded windows, and the exact read only if they miss
  [V `store.go:2451-2526`]. Then `R0 = seed + gap`, a synchronous lease
  append of `R0 + N`, and the first number is `R0 + 1`.
- **What it holds:**
  - `eventMu`: every event append waits, as today.
  - The engine mutex, for the 2,000-object window (about 9 ms warm).
  - One tape append, with `eventMu` released.
  - On the paper after D13 this site cost a 748 ms first passivation (ledger
    `35e5c82f2`) [V commit message; the cold share of it is not split out].
- **Why it is not boot-critical:** the lease must ship, and must have
  migrated on a computer, **before** `store.event_table` is switched on there.
  While every event is still in `og_objects`, the windows find one and the
  exact read does not run. The record: "correct for a store whose events are
  in `og_objects`, which is every store today".
- **Builder rule:** the deploy-2 writer refuses to stamp, and the setting
  refuses to turn on, until the lease row exists. Then the exact read can run
  only on a computer whose newest 50,000 objects hold no event before deploy
  2. That case is not expected and is logged.
- **Fresh store:** no lease and no events. The seed is 0 and the lease is N.

## 5. D12's per-run seq, and the tape

- **The tape.** `stream_seq` is on the signed tape, in the projection batch
  payload:
  - op bodies carry the object's metadata and the `EventRecord` with
    `stream_seq` [V `graph_store.go:1173-1186`, `projection_tape.go:66-77`];
  - the event commits to the payload by digest [V `appender.go:412-417`];
  - head receipts are ed25519-signed [V `computerevent/receipt.go:5,26-40`].
  - `choir_events` projects it into a column [V `event_table.go:67`].
  - Replay does not re-derive it (above). The lease is on the tape the same
    way, as an ordinary object op.
- **D12.**
  - `AllocateEventSeq` shares `allocateEventSeqLocked`
    [V `store.go:2555-2569`], so lifecycle-folded events take their stream
    numbers under the lease with no change.
  - The per-run `seq` is a separate counter with its own seed: the run's
    metadata index, `eventTableRunMaxSeq`, and with the sealer the gap and
    `seqFloor` [V `store.go:2346-2386`]. That seed is bounded by the run, and
    the lease does not cover it. It stays as is (the record's follow-up 2
    still applies).
  - Note for the per-run counter: on a failed unbatched append, the per-run
    mark is dropped so the next append takes the same `seq` again
    [V `store.go:2643-2649`]. That is deliberate and outside this design.

## 6. Class, surfaces, rollback, heresy, proof

**Mutation class:** red.

**Protected surfaces:**
- the stream allocator (`allocateEventSeqLocked`, `seedStreamSeq`);
- the boot path's first event append;
- `store.Open` (the receipt read) and `storeschema.Version`;
- tape writes of a new object kind;
- `Reopen`'s field set.

**Not touched:** the projector, the witness extractor, the replay manifest,
the tape format, and every table and index.

**Rollback path.** Revert the lease commits.
- The reverted binary ignores the lease row (an inert og row) and seeds by
  D13's path. That path is correct while events are in `og_objects`. If
  deploy 2's switch is on, turn it off first; otherwise the exact read returns
  at that binary's boots.
- Guest-updater `Apply` will refuse an epoch-2 release. The CI revert path is
  the rollback route.
- A later forward deploy sees receipt 2, distrusts the lease, and re-seeds
  once.

**Heresy delta.**
- Discovered:
  1. `emitEvent` publishes the allocated `stream_seq` on the bus even when
     `AppendEvent` failed [V `agentcore/runtime.go:4760-4768`]. With batching
     off the seed adds no gap [V `store.go:2522`], so a restart after a failed
     append can reissue a number a client already holds. This is a reuse path
     independent of the seed's read [H: no observed occurrence].
  2. After a restore, the projection head stays behind the platform head:
     appends are refused [V `appender.go:712-717`], and the next boot replays
     past the target [V `appender.go:1013-1022`]. Whether a restore survives
     a reboot is unverified [H].
  3. The comment on `maxStreamSeq` [V `store.go:248-252`] still describes "a
     lazy seed from the newest page".
- Introduced (by the design):
  - an allocator-written object kind outside run semantics;
  - gaps of up to N per process start;
  - the epoch bump's `Apply` fence on epoch-2 releases.
- Repaired (once built):
  - the exact read at boot after deploy 2;
  - the `updated_at` reuse band (Round 2 F1), since the seed no longer
    depends on `updated_at`;
  - discovered item 1.

**Conjecture delta.** "The newest events carry the maximum" stops being
load-bearing. It survives only in the cross-check and in the
migration/untrusted path.

**Tests first (the builder writes them failing).** Local E2E on the store:
- After a lease exists, a reopen's first append runs no `ListObjects
  choir.event` and no exact read (engine-mutex counters, as
  `TestStreamSeqSeedDoesNotListTheEventKind`).
- A failed append's published number is not reissued after a reopen
  (fails today with batching off).
- DeepSeek's counterexample (bunch 156) still allocates 13.
- A crash between the lease CAS and finalize, then a replay, seeds above the
  lease.
- Restore, then reopen, then append: above the pre-restore maximum.
- The receipt says 2 with a lease present: untrusted, exact path, logged.
- With the sealer on, the first allocation waits for the lease while a
  reservation is held: no deadlock, with a deadline.
- The 2,000/50,000 window tests still pass on the migration path.

**Deployed proof (FP0, read-only on live state, no SQL, no SSH).** The build
must expose a `stream_seq_lease` block (`reserved_through`, `trusted`,
`seeded_from` lease|migration|untrusted, `seed_ms`, `cross_check_max`) in
`/api/runtime/observability` (standing question 9).
1. Staging, after CI and the deploy identity check, on a **test computer**
   with history:
   - **First boot:** `seeded_from=migration` once.
   - **A planned restart:** `seeded_from=lease`, `trusted=true`, `seed_ms`
     under 50, and the engine-mutex table shows no `ListObjects choir.event`.
   - **A WebSocket cursor script:** record the highest `stream_seq` S1, then
     restart, run a prompt, and assert every new `stream_seq` > S1 and that
     `after_seq=S1` returns them. JSON artifact.
2. Same test computer, **deploy 2 on**, then restart: the same three facts,
   and no "out of reach" line.
3. Optional rollback rehearsal on the test computer via the CI revert path,
   then forward: `seeded_from=untrusted` once, then `lease`.
4. **The paper**, at its next planned refresh:
   - `seeded_from=migration`, its `seed_ms`, the first passivation time and
     the `/health` 503 window.
   - On the refresh after that: `seeded_from=lease`, with no event-kind read
     on the engine mutex.
   - Only then is deploy 2's switch eligible on the paper.

## 7. Alternatives considered

| Option | Closes the boot read | Closes the `updated_at` band | Covers published-not-stored | Cost | Verdict |
|---|---|---|---|---|---|
| **Lease on the tape (chosen)** | yes, one PK seek | yes | yes | one tape append per N/2 to N numbers; an async refill and a wait path; an epoch bump | chosen: no schema, witness or tape-format change, and one authority |
| Lease in a sidecar file | yes | yes | yes | fsync per block, simple | lost with the disk or VM (slow seed then); a second authority beside the tape |
| Projector fold (max in `finalizeBatch`'s transaction) into a new table or a heads column | yes | yes | **no** (only stored numbers) | one row write per batch; witness schema change, or an `IgnoredTables` entry at every site plus older manifests calling it unknown | rejected: misses failed-append numbers, and touches the witness boundary |
| Option 2, the `(object_kind, updated_at)` index on `og_objects` | yes, one reverse seek | **no** | no | an online build on a 31 GiB table, write amplification on every og write [H: unmeasured]; witness-neutral, because `object_kind` already leads an index, the kind-index trick [H] | rejected as the close; acceptable later as a read optimisation |
| Generated column `og_meta_stream_seq` and an index | yes | yes for og | no | the derived-columns operation on og, and a `SHOW COLUMNS` change | rejected: heaviest; witness churn |
| Exact `choir_events` max by owner seeks, og by the windows | the table half only | no for og | no | none | rejected alone; a possible tighter cross-check |
| Status quo (D13, with the exact fallback) | no: 104 s at every boot after deploy 2 | no | no | none | the reason for this design |

## Open questions (for review)

1. **The epoch bump versus accepting the band.** Without the bump, the
   rollback-forward residual is the old band, limited to "an older binary ran
   and its newest event is behind 50,000 objects". The bump closes it, at the
   cost of the `Apply` fence. This design takes the bump as the conservative
   option.
2. **Block size N.** 8,192 is proposed. It needs the paper's event rate (not
   measured here) to confirm that a refill lands well within a half-block.
3. **Lock audit.** Can any caller of `AppendEvent` or `AllocateEventSeq` hold
   a lock the lease append also takes (the appender mutex, sealer `flushMu`,
   lifecycle locks)? [H]
4. **Base installs.** Does a ProjectionBase blob carry `store-schema.json`?
   If a base published by an epoch-2 binary is installed, the first seed
   distrusts the lease and is slow once [H].
5. **Restore semantics** (heresy 2): does a restored computer replay past its
   target at the next boot? The design is safe either way, with the
   carry-forward append after `Reopen`.
6. **Retiring the exact fallback.** Should `seedStreamSeqExact` be deleted
   once every computer holds a trusted lease? It is then reachable only on
   the migration and untrusted paths.
