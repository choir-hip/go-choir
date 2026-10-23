# stream_seq high-water mark: a lease on the tape (design, 2026-10-23)

Date: 2026-10-23. Author: an FP0 design worker. Status: **design, round 2,
not built.** It answers the gate in
`docs/problems/stream-seq-seed-scans-the-event-kind-at-boot-2026-10-23.md`
("Gate on F-c deploy 2"), option 1, the persisted high-water mark.

Mutation class of the build: **red**. It touches the event sequence
allocator, the guest boot path, the store schema epoch, a new object kind
written to the tape, and the workspace swap points. Review is tier 2: round 1
was bunch 162 (Devin, DeepSeek v4.1 flash). Its dispositions are at the end.

Tags: [V] verified in code at the cited line on `origin/main` `b1abdfb0b`;
[H] hypothesis, not verified.

Round 2 changes the trust rule, the untrusted seed, the migration gap, the
allocator's concurrency, the writer fence, the witness rule, the block size,
and drops the restore carry-forward. Section numbers keep round 1's questions.

## The decision

1. **The mark is a lease**: the highest `stream_seq` this computer may hand
   out. The allocator never gives out a number above a lease that has passed
   the head CAS. On every path except migration (section 4), the lease is
   therefore at or above every number published: stored, queued for the
   sealer, or put on the bus by a failed append.
2. **The lease lives on the tape**, as one `og_objects` object
   (`choir.stream_seq_lease`, external-key identity) written through the
   projection interceptor. There is no new table, column, file format, index
   or tape format. The tape is the single authority, and the local row is its
   projection, which replay rebuilds.
3. **It is written ahead**, in blocks of **N = 65,536**, sized against refill
   latency (section 2). A single-flight refill starts in the background when
   half the block is used. Seeding is single-flight too. No allocator ever
   holds a number above the durable lease.
4. **A trusted lease** seeds by one primary-key read plus a bounded
   cross-check: the newest 500 rows of `choir_events` by key, and the og
   windows 2,000, then 50,000. A cross-check above the lease makes the lease
   untrusted.
5. **Trust is fail-closed.** A lease is trusted only when all four hold:
   - its row is present and parses;
   - the prior store schema receipt was at least 3, with the epoch bumped to
     3;
   - no untrust marker is in the workspace (every workspace swap point and
     every low or absent prior receipt writes one, durably, before anything
     else);
   - it is within a bound of the cross-check.

   Anything else takes the **exact seed** once. That is a full paged scan of
   the event kind's `stream_seq` plus the exact maximum of `choir_events`, plus
   an unconditional gap of 2^20. Then it re-leases and clears the marker.

## 1. Where the mark lives

**What exists.**

- `choir_events` has no index led by `stream_seq`. Its indexes are `(tape_seq,
  op_index)` PK, `(run_id, seq)`, `(owner_id, stream_seq)` and
  `(trajectory_id, stream_seq)` [V `event_table.go:70-73`].
- The store-wide seed reads the newest 500 rows by tape position
  [V `event_table.go:235-239`]: newest-carries-max over tape order, not a
  maximum.
- An exact maximum over `choir_events` needs no new state: one reverse seek
  per owner on `(owner_id, stream_seq)`, with owners listed by skip-seeks on
  the same index [H: no plan test yet, the builder pins one]. For `MAX(seq)`
  over a run, Dolt plans a GroupBy over every row of the run
  [V `event_table.go:226-230`, the per-run comment]. The same is assumed, not
  checked, for `MAX(stream_seq)`, so write `ORDER BY ... DESC LIMIT 1`.
- `og_objects` events have no `stream_seq` column; it is in `metadata`
  [V `graph_store.go:1173-1186`]. An index needs a STORED generated column (the
  `og_rec_inbox_seq` pattern, `dolt_store.go:262-268`), built by the explicit
  derived-columns operation [V `derived_columns.go:14-19`]. That changes `SHOW
  COLUMNS`, which the content witness hashes
  [V `computerversion/dolt_state_extractor.go:338-375`].
- A new index on `choir_events` led by `stream_seq` would change that table's
  `SHOW COLUMNS` too, which is why its DDL is frozen
  [V `event_table.go:38-43`].

No zero-state read is exact for both halves.

**Why on the tape, as an object.**

| Home | Authority | Witness and schema | Survives |
|---|---|---|---|
| **og object through the interceptor (chosen)** | the tape | no schema change. `projectObject` is kind-agnostic [V `project.go:223-264`]. The row is witness content, made deterministic by section 7's timestamp rule | restart, crash, replay, base install, rebuild, disk loss |
| sidecar file beside the store marker | a second authority beside the tape | none | restart and restore (rematerialize renames only the marker and the workspace [V `agentcore/rematerialize.go:227-259`]); not disk loss or a new VM |
| a fold in `finalizeBatch`, into a new table or a heads column | the tape | a heads column changes a witnessed schema hash [V `dolt_state_extractor.go:338-375`]. A new table needs `IgnoredTables` at every extractor site [V `projectionbase/witness.go:25`, `agentcore/replay_completeness.go:494`], and older binaries call it unknown | restart, replay |

**The object.**

- Kind `choir.stream_seq_lease`, registered with `IdentityExternalKey`
  [V the mode exists, `objectgraph/registry.go:21`] so the canonical ID is
  stable. Owner `__system__`, one fixed identity key.
- Body and metadata: `{"reserved_through": R, "block": N, "writer_epoch": 3,
  "head_sequence": h}`, where `h` is the local projection head sequence when
  it was written (diagnostic only).
- The read is `GetObject` on the canonical ID, one PK seek under the engine
  mutex [V `objectgraph/dolt_store.go:706-711`].
- The plain upsert keeps the maximum because exactly one refill is in flight
  (section 2), each with a larger `R`. The appender serialises appends under
  `a.mu` [V `appender.go:402`, after `awaitBarrier` at `:401`], so tape order
  is value order. No projector change is needed. Replay and older binaries
  project the row as an ordinary object; the projector does not call the
  write registry [V `objectgraph/registry.go:300-306`].

## 2. Write cost, atomicity, concurrency

**Lock order.** The real order is `flushMu` → appender `a.mu` → `eventMu`
(→ the engine mutex inside the SQL).

- The sealer's flush holds `flushMu` and appends
  [V `telemetry_sealer.go:486-488`, `:587`].
- `AppendNewPayload` takes `a.mu` [V `appender.go:401-403`].
- `Prepare` and `finalizeBatch` take `eventMu` under it
  [V `computer_events.go:47`, `:140`].
- The interceptor runs outside the engine mutex (DeepSeek F6, cited at
  `dolt_store.go:646,1054,1141` [H: not re-read]).

`eventMu` is never held across an append. `AppendEvent` already releases it
before the tape write [V `store.go:2632-2635`]. Platform `writeMu`
(`platform/service.go:40`) is the platform service's lock, unrelated to the
guest store [V].

**Allocator state** (all fields on `Store`, under `eventMu`):

- `leaseState`: `unseeded`, `seeding` or `ready`;
- `reserved`, the durable `R`;
- `refillInFlight`;
- `leaseChanged`, a channel closed and replaced at every state change;
- `refillFailed`, a count, plus `lastLeaseErr`;
- the trust outcome.

`allocateEventSeqLocked` loops:

1. **`unseeded`**: set `seeding`, start the seeder goroutine, then wait.
2. **`seeding`**: wait.
3. **`ready` and `maxStreamSeq + 1 > reserved`**: if no refill is in flight,
   start one, then wait.
4. **`ready` and the number is inside `reserved`**: hand it out. If fewer than
   N/2 numbers remain and no refill is in flight, start one asynchronously.

"Wait" means:
- release `eventMu`;
- select on `leaseChanged` or the allocation deadline (30 s, the sealer's
  seal timeout);
- reacquire `eventMu`, and loop.

A deadline returns an error. The append fails, and no number is taken.

Two allocators can never both hand out unleased numbers. Only the goroutine
that flips `unseeded` to `seeding` seeds, and every path to a number checks
`reserved` under `eventMu` (DeepSeek F5).

- **The seeder** computes the seed (section 3) with `eventMu` released. It
  appends the lease `seed + N`, and only after the CAS does it take `eventMu`
  to set `maxStreamSeq = seed`, `reserved = seed + N` and `ready`, then
  broadcast.
- **On seeder failure** the state goes back to `unseeded`, the error is
  recorded, waiters are woken, and they retry or meet their deadline. No
  number is handed out.
- **The refill** reads its target under `eventMu`, `max(reserved,
  maxStreamSeq) + N`, then releases `eventMu` and appends.
  - On success it raises `reserved = max(reserved, target)`.
  - On any error it leaves `reserved` alone, including an error after the CAS,
    such as a finalize failure [V `appender.go:763-765`].
  - Either way it clears `refillInFlight`, counts `refill_failed` on error, and
    broadcasts.
  - The next allocation that needs it starts a new refill, at most one per
    second after a failure.

`reserved` never decreases.

**Atomicity under a crash.**

- A crash before the lease CAS: no number above the old `R` was handed out.
- A crash after the CAS and before the local finalize: the next boot replays
  the lease before the runtime starts [V `autoputer/run.go:747-749`;
  reconstruct fails unless the local head reaches the platform head,
  `appender.go:1013-1022`]. So no event can carry a number above a lease that
  is not on the tape.

**Fail-closed cost.** Allocation needs a durable lease. When vmctl or the
store cannot finish a refill before N/2 numbers are used, event appends fail
at their deadline. That is an event-append outage, not reuse. Today a
tape-bound append already fails when the tape append fails, so it fate-shares
with the same components (standing question 7). It shows as `refill_failed`
and `last_lease_error` in the observability block (Devin F2).

**Block size N = 65,536**, sized against refill latency (DeepSeek, open
question 2):
- A refill is one tape append. It includes `awaitBarrier`, a sealer flush of
  up to the 30 s seal timeout [V `telemetry_sealer.go:55`, `appender.go:401`],
  then pin, CAS and finalize.
- The finalize waits on the engine mutex, which the paper has seen held for
  104 s.
- The runway after the trigger is N/2 = 32,768 numbers. At the paper's
  measured 8 to 18 events/s
  [V `docs/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md`],
  that is about 30 minutes.
- Against a 120 s refill, 32,768 covers a burst of about 270 events/s, 15x
  the measured peak.
- Refills happen about every 30 minutes, each forcing one sealer flush.

Waste is at most N per process start (int64; under 2^53 for the browser).
DeepSeek's 1,024 to 2,048 gives 28 to 57 s of runway at 18 events/s. That is
shorter than one observed engine stall, so appends would fail during stalls:
rejected.

**Gaps** are allowed and already present:
- a failed batch leaves a gap [V `store.go:2547-2554`];
- the sealer gap is 512 [V `store.go:2522-2524`];
- consumers use only the maximum and `> after`
  [V `frontend/src/lib/Desktop.svelte:1031-1042`, `event_table.go:212-215`].

The per-run gap is unchanged.

## 3. The seed: trust, the exact seed, restore, rollback

### Trust (fail-closed; Devin F1, DeepSeek F1 and F2)

A lease is **trusted** only if all four hold:

- **(a) Row.** The lease row is present and parses, with `writer_epoch` of 3
  or more.
- **(b) Receipt.** The prior receipt read by `Open` was 3 or more.
  - `Open` reads `store-schema.json` before it rewrites it (today it writes at
    `store.go:964-970` and never reads it).
  - If the prior receipt is absent on a workspace that has rows, or below 3,
    `Open` first writes the untrust marker durably (write, fsync, rename,
    directory fsync: the `WriteFloor` pattern of lane `fp1-event-table-d2`).
    Only then does it write receipt 3.
  - So receipt 2, then `Open` writes 3, then a restart before the first
    append still finds the marker (DeepSeek F1).
- **(c) No marker.** `stream-seq-lease-untrusted.json` is absent from the
  workspace.
- **(d) Bound.** `R` is at most `cross_check_max + N + G`, where `G = 2^20`
  is the exact-seed gap. A legitimate lease satisfies this by construction:
  it is written at most N + G above an exact or cross-checked maximum.
  - A forged or corrupt huge `R` fails the bound. The exact seed then ignores
    any `R` above `exact + N + G`, with a log line naming it, so a forged row
    cannot burn the sequence space (Devin F2).
  - A cross-check that finds no event at all fails (d) as well.

There is no "the store is fresh" exception. With no lease row the seed is the
exact seed, and on an empty store that is cheap (Devin F1).

**The untrust marker is written by every workspace swap**, into the incoming
workspace, before it is renamed into place or opened. The swap points
[V by grep of workspace renames]:

- `projectionbase.InstallVerifiedBase` (`install.go:53`, renames `:172-187`);
- `projectionbase.RebaseRetainedStore` (`rebase.go:19`, renames `:58-76`);
- `RematerializeFromTape`'s flip (`agentcore/rematerialize.go:227-262`).

**And by the base publisher.** Bases tar every regular file in the workspace
[V `projectionbase/publisher.go:67-104`]; the bookkeeping cleanup removes only
seed metadata [V `install.go:330-337`]. So a base carries receipt 3 and a
lease row (Devin F1, DeepSeek F2). The publisher adds the marker to every
blob, so a base is untrusted by construction even if a swap site forgets.

Order at boot matters. `store.Open` runs before install or rebase
[V `autoputer/run.go:200` against `:366`], and the rebase `Reopen`s the
swapped workspace [V `projectionbase/rebase.go:19-76`,
`autoputer/projection_base.go:173-200`]. The receipt that `Reopen`'s `Open`
reads is the base's, so the marker carries the decision.

A guard test (the `event_table_guard_test.go` AST pattern) fails if any
function that renames into the live store or workspace path does not call
`markStreamSeqLeaseUntrusted`.

The marker is cleared only by the exact seed's successful lease append.
After a restore, rebase or install, the next process start pays the exact
seed once. That is priced below and logged.

### The exact seed (DeepSeek F3, Devin F3)

When the lease is absent or untrusted, the seed is the **maximum, not a
band**:

- **og half:** a full scan of the event kind's `stream_seq`. It is keyset
  paged over `idx_og_objects_kind_owner` and the PK suffix, `metadata` only,
  5,000 rows a page, releasing the engine mutex between pages. A row whose
  metadata has no `stream_seq` has its body decoded.
- **table half:** the exact `choir_events` maximum, by per-owner reverse
  seeks (section 1). If the plan test does not show seeks, a full scan of the
  `stream_seq` column instead.
- **The gap:** `seed = max(og, table, R if within bound) + G` with `G =
  2^20`, unconditionally, batching on or off (section 4).

Cost:
- It is linear in the event history, like the read that held the paper for
  104 s, and is priced at that class.
- It reads `metadata`, not `body`. Dolt keeps large blobs out of the row
  [H: so it may be cheaper; measure with `TestStreamSeqSeedScale` before the
  build ships].
- Because it pages, `/health` and other engine readers proceed between pages.
  Event appends wait for it [H: the 503 effect is unmeasured].

It runs once per computer at migration, and once after each swap or
rollback-forward. `seedStreamSeqExact` (the 500 newest by `updated_at`) is
then no longer used by the seed. The old windows remain only as the
cross-check.

### Trusted seed

- `seed = R`.
- The cross-check reads `choir_events` newest-500 by key and the og window of
  2,000, widened to 50,000 when the first window finds no event (Devin F3).
- If `cross_check_max > R`, the lease is stale: it is untrusted, the exact
  seed runs, and a log line says "lease violated" with both numbers.
- Then a fresh block is appended (`R + N`) before the first number `R + 1`.

### Replay, restore, rollback

- **Replay** projects op bodies verbatim and never calls the allocator
  [V `appender.go:879-890`, `project.go:223-264`].
- **Every boot** replays to the platform head before the runtime starts
  [V `run.go:747-767`, `appender.go:1013-1022`]. Rebase replays (W, H] after
  the swap [V `recoveryplan/recovery_plan.go:32-34`].
- **In-process restore.**
  - `RematerializeFromTape` reconstructs only to the target
    [V `rematerialize.go:164`]; later events stay on the tape
    [V `appender.go:832-834`].
  - It `Reopen`s the swapped workspace [V `rematerialize.go:262`].
  - `Reopen` does not touch `maxStreamSeq` or `streamSeqSeeded`
    [V `store.go:1188-1268`], and the lease fields sit beside them. The next
    refill writes `max(reserved, maxStreamSeq) + N`.
  - Appends are refused while the projection head is behind the platform
    head [V `appender.go:712-717`]. So the refill fails (`refill_failed`), and
    appends fail as they already do after a restore.
  - At the next boot the tail replays, and the marker the flip wrote forces
    the exact seed.
- **Binary rollback across the epoch bump.**
  - The reverted binary (epoch 2) ignores the lease row and seeds by D13's
    path. Its `Open` rewrites the receipt to 2.
  - The next epoch-3 `Open` reads 2, writes the marker, and the exact seed
    runs.
  - The guest updater's `Apply` refuses an epoch-2 release on an epoch-3
    store [V `updater/updater.go:456-458`]. The CI revert, the platform-follow
    offer and `restorePrior` are not fenced [V `storeschema/schema.go:30-37`],
    and the receipt-plus-marker rule catches them.
  - **Release rule:** the epoch-3 release keeps `min_store_schema_version` at
    2 or below, or the updater refuses every epoch-2 store and migration
    stalls [V the refusal at `updater.go:459-460`; DeepSeek F2. That an offer
    can set min = 3 is DeepSeek's cite of `platform_update.go`, H].
  - D2's floor (lane `fp1-event-table-d2`, `storeschema/floor.go`, not on
    main) is separate. The lease does not raise it: an older binary is safe
    to run.

### Restore carry-forward: dropped (DeepSeek F8)

Round 1 asked for a lease append right after `Reopen`, against a future head
rewind. It is dropped:

- The value it would carry is process memory. After a real rewind the process
  that knew it is gone.
- Today there is no rewind. The local head advances only by
  `WHERE sequence=? AND canonical_event_head=?`
  [V `computer_events.go:195`]. Appends are refused while the heads differ
  [V `appender.go:712-717`]. A boot whose local head is ahead of the platform
  head fails: the plan resumes with no tail, and reconstruct's final
  `sameHead` check returns `ErrNeedsProjectionRepair`
  [V `recoveryplan/recovery_plan.go:66-71`, `:118-123`, `appender.go:1013-1022`], which
  `runReplayPhase` turns into a fatal boot [V `run.go:781`].

That existing refusal is the head-rewind fence. The build pins it with a test
rather than adding code.

## 4. Migration (no lease yet)

- **Where:** the seeder, triggered by the first allocation of the first
  lease-aware process (today's site, `store.go:2395-2399`).
  - Optionally it can start eagerly in a goroutine right after the replay
    phase, so it overlaps boot [H: a builder choice; it does not change
    correctness].
  - It is the exact seed of section 3, with the gap `G = 2^20` applied
    unconditionally, batching on or off.
  - That gap is DeepSeek F4. `emitEvent` publishes a `stream_seq` even when
    `AppendEvent` failed [V `agentcore/runtime.go:4760-4768`], and with
    batching off today's seed adds no gap [V `store.go:2522`].
  - Migration is the one boot with no bound on numbers the previous,
    lease-unaware process published and never stored. The gap bounds them at
    2^20, and more than that is the stated residual.
- **What it holds:**
  - The engine mutex, one page at a time.
  - Not `eventMu`, which is released while seeding.
  - Event appends wait (single-flight), up to their deadline. On a large
    history the first appends can fail at the 30 s deadline while the seed
    runs [H: the paper's scan time is unmeasured]. The builder either raises
    the deadline for the `seeding` state, or starts the seed eagerly after
    replay.
- **When:** before `store.event_table` is switched on for that computer. The
  deploy-2 writer refuses to stamp, and the setting refuses to turn on, until
  a trusted lease exists on that computer.
- **Fresh store:** the exact seed is 0 (no rows), plus the gap.

## 5. D12's per-run seq, and the tape

**The tape.** `stream_seq` is on the signed tape:
- the projection batch's op bodies carry it
  [V `graph_store.go:1173-1186`, `projection_tape.go:66-77`];
- the event commits to the payload by digest [V `appender.go:412-417`];
- the receipts are signed with ed25519 [V `computerevent/receipt.go:5,26-40`].

`choir_events` projects it into a column [V `event_table.go:67`]. Replay does
not re-derive it. The lease rides the tape the same way.

**D12.**
- `AllocateEventSeq` uses `allocateEventSeqLocked` [V `store.go:2555-2569`],
  so folded lifecycle events take stream numbers under the lease. Their
  caller holds no store lock across the later batch append
  [V `store.go:1524-1528`].
- They wait like any allocation, with `eventMu` released. Test listed below.
- Per-run `seq` keeps its own seed [V `store.go:2346-2386`] and is not covered
  by the lease.
- The per-run counter re-takes a failed unbatched append's number by design
  [V `store.go:2643-2649`]. That is outside this design.

## 6. Class, surfaces, rollback, heresy, tests, proof

**Mutation class:** red, tier 2.

**Protected surfaces:**
- the stream allocator and its concurrency;
- the boot path's first allocation;
- `store.Open`'s receipt read and the marker;
- `storeschema.Version`;
- the three swap points and the base publisher;
- the objectgraph registry and the writer fence;
- tape writes of the new kind.

**Not touched:** the projector, the extractor, the replay manifest and the
tape format.

**Writer fence (Devin F2).**
- Register `choir.stream_seq_lease` with a new writer `system:store`, not
  `WriterRuntime`. A runtime kind admits every writer
  [V `objectgraph/writer.go:199-210`], and an unregistered kind is refused
  [V `registry.go:307-314`].
- The gate defaults to observe [V `writer.go:110-115`], so the registry alone
  does not refuse a desk write in production. Add a hard fence independent of
  the gate mode: `CheckWrite` refuses a non-tombstone write of the kind unless
  the context carries the store-private lease capability.
- A guard test fails if anything outside `internal/store`'s lease file sets
  that capability.
- Replay does not call `CheckWrite` [V `registry.go:300-306`], so the fence
  cannot poison a replay.
- A forged huge `R` that gets past the fence is bounded by trust rule (d).

**Rollback path.** Revert the lease commits.
- The reverted binary ignores the row and seeds by D13's path, which is
  correct while events are in `og_objects`. If deploy 2's switch is on, turn
  it off first.
- `Apply` refuses epoch-2 releases; the CI revert is the route.
- Forward again: receipt 2, then the marker, then the exact seed once.

**Heresy delta.**
- Discovered:
  1. `emitEvent` publishes a `stream_seq` for a failed append
     [V `runtime.go:4760-4768`], and with batching off a restart adds no gap
     [V `store.go:2522`]. A published number can be reissued [H: not
     observed].
  2. After a restore, appends are refused until a reboot replays past the
     target [V `appender.go:712-717`, `:1013-1022`]. Whether a restore
     survives a reboot is unverified [H].
  3. The `maxStreamSeq` comment is stale [V `store.go:248-252`].
  4. (Round 2) Bases carry `store-schema.json`, so the epoch receipt is per
     workspace copy and not per computer [V `publisher.go:67-104`].
- Introduced (by the design):
  - an allocator-written kind;
  - gaps of up to N per process start, and 2^20 per exact seed;
  - the epoch bump's `Apply` fence;
  - an event-append outage when refills fail for longer than the runway
    (fail-closed, observable);
  - one exact seed per swap.
- Repaired (once built):
  - the exact read at every boot after deploy 2;
  - the `updated_at` band. The seed no longer uses `updated_at` on any path,
    because the exact seed is a maximum. Round 1 overclaimed this while the
    untrusted path was still the band;
  - discovered item 1, after migration.

**Tests first (they fail today or on round 1's design).** Local E2E on the
store unless marked.

Trust:
1. Receipt 2, `Open` (writes 3), restart before any append, reopen: still
   untrusted, exact seed (DeepSeek F1).
2. Lease row present and receipt absent: untrusted.
3. A base built and installed (and a rebase) from a workspace with receipt 3
   and a stale lease, after an epoch-2 interlude that allocated past `R`:
   untrusted once, no reissue (Devin F1, DeepSeek F2).
4. Rematerialize, then a restart: marker present, exact seed.
5. The swap-point guard test, and the `Open` read-before-write ordering test:
   the receipt read happens before the write.

Exact seed:
6. An epoch-2 high number placed behind 500 newer events and 50,000 objects
   is not reissued after forward. This fails with round 1's band (DeepSeek
   F3).
7. Untrusted runs the full exact seed even when the first window finds an
   event.
8. With batching off, a failed append in the previous process: its published
   number is not reissued after migration (DeepSeek F4).
9. DeepSeek's bunch-156 counterexample still allocates above 12.

Concurrency:
10. Two concurrent first allocations during migration: one seeder, no
    unleased number (DeepSeek F5).
11. A refill failure clears the in-flight flag and the next allocation
    refills.
12. Two allocators trigger one refill.
13. A refill error after the CAS: `R` never decreases.
14. A migration lease-append failure: the first allocation fails, and no
    number is handed out.
15. The folded path (`AllocateEventSeq`) waits on an exhausted lease.
16. With batching on and a sealer reservation held, the first allocation
    waits for the lease: no deadlock, and the deadline fires.

Writer fence:
17. An unregistered or wrong-writer lease write is refused under
    `WriterGateEnforce`.
18. A desk-declared writer is refused even under observe (the hard fence).
19. A forged `R` above the bound is ignored with a log line.

Witness:
20. The lease op carries fixed `created_at` and `updated_at` (section 7). A
    base witness equals a replay witness after lease appends (DeepSeek F7).
21. The lease row survives `MigrateAndFenceServingVocabulary` after a replay
    with applied rows, with its key unchanged.

Rewind:
22. A boot whose local head is ahead of the platform head refuses (pins
    section 3's fence).

Kept from round 1: no `ListObjects choir.event` on a trusted reopen
(engine-mutex counters); crash between the lease CAS and finalize, then
replay, seeds above; the 2,000/50,000 window tests as the cross-check.

**Deployed proof.** FP0 runs it read-only on live state: no SQL, no SSH.

The build exposes a `stream_seq_lease` block in `/api/runtime/observability`:
`reserved_through`, `state`, `trusted`, `untrust_reason`, `seeded_from`
(exact|lease), `seed_ms`, `cross_check_max`, `refill_failed`,
`last_lease_error`, `marker_present`.

1. **Staging test computer** with history:
   - **First boot:** `seeded_from=exact`, `untrust_reason=no_lease`, and the
     `seed_ms` cost.
   - **A planned restart:** `seeded_from=lease`, `trusted=true`, `seed_ms`
     under 50, and no `ListObjects choir.event` in the engine-mutex table.
   - **A WebSocket cursor script:** record S1, restart, run, and assert every
     new `stream_seq` > S1 and that `after_seq=S1` returns them. JSON
     artifact.
2. **Same computer, a restore** (an owner checkpoint), then reboot:
   `untrust_reason=swap`, exact seed once, then `lease`.
3. **Same computer, deploy 2 on**, restart: `lease`, with no event-kind read.
4. **Optional:** a CI-revert rollback rehearsal, then forward:
   `untrust_reason=receipt`.
5. **The paper**, at its next planned refresh: `exact` and its `seed_ms`; the
   first-passivation time; the `/health` 503 window. On the refresh after
   that, `lease`. Only then is deploy 2 eligible there.

## 7. Witness determinism and alternatives

**Witness content (DeepSeek F7).**
- The witness hashes rows, not only schema
  [V `dolt_state_extractor.go:388-403`].
- `projectObject` substitutes the current time only for zero timestamps
  [V `project.go:235-241`].
- `ogPut` passes `now` into the object [V `store/graph_store.go:100-106`], so
  live and replay already agree on the op's own timestamps. The finding's
  premise that they would disagree is partly refuted.
- The rule is adopted anyway: the lease op carries fixed `created_at` and
  `updated_at` (the constant `2000-01-01T00:00:00Z`). Every lease row is then
  a pure function of `R` and its fields.
- The fixed old `updated_at` also keeps the row out of the newest-objects
  windows.

**Alternatives.**

| Option | Closes the boot read | Closes the `updated_at` band | Covers published-not-stored | Verdict |
|---|---|---|---|---|
| **Lease on the tape (chosen)** | yes | yes (the exact seed off the lease path) | yes, after migration | chosen |
| Lease in a sidecar file | yes | yes | yes | lost with the disk; a second authority |
| Fold in `finalizeBatch` (a table or column) | yes | yes | no | touches the witness boundary |
| `(object_kind, updated_at)` index (option 2) | yes | no | no | an online build on 31 GiB [H]; a read optimisation at best |
| `og_meta_stream_seq` column and index | yes | yes for og | no | the heaviest; a `SHOW COLUMNS` change |
| Exact `choir_events` max by owner seeks alone | the table half | no | no | used inside the exact seed |
| Status quo (D13) | no: 104 s per boot after deploy 2 | no | no | the reason for this record |

## Open questions (round 2)

1. **The exact seed's cost on the paper**, paged and metadata-only, is
   unmeasured. If it is far above the 30 s deadline, the builder should start
   it eagerly after replay and let `seeding` waiters wait longer.
2. **Clearing the marker in-process.** A process whose own seed was trusted
   or exact, and that later swaps in-process (restore), could clear the
   marker after its next successful refill, since its memory dominates. This
   design does not, to stay conservative: one exact seed per swap.
3. **The per-owner skip-seek plan on `choir_events`** needs a plan test. The
   fallback is a full column scan, still exact.

## Round 2: dispositions of bunch 162

Both seats said REVISE or reject-as-specified on round 1 (`ff714de1f`). Each
finding was checked in code.

| Finding | Disposition | Code |
|---|---|---|
| Devin F1, a trusted lease stale after a base install or rebase | **Accept.** Swap-point markers, the publisher marker, the receipt and marker rule, no "fresh" exception | `publisher.go:67-104`; `install.go:330-337`; `run.go:200` before `:366`; `rebase.go:19-76` |
| Devin F2, unexamined write gates, a forged R, the refill wedge | **Accept.** A dedicated writer, a hard fence independent of gate mode, bound (d), `refill_failed` | `registry.go:307-314`; `writer.go:110-115`, `199-210` |
| Devin F3, the cross-check narrower than migration | **Accept.** Widened to 2,000 then 50,000, and a violation makes the lease untrusted | `store.go:2451-2526` |
| Devin F4, citation drift | **Accept.** Fixed: `rematerialize.go:164`, `:227-259`, `:262`; `replay_completeness.go:494`; `store.go:2547-2554` | |
| DeepSeek F1, receipt written before repair | **Accept.** The durable marker is written before receipt 3 | `store.go:964-970` |
| DeepSeek F2, bases and rebuilds launder the receipt; min epoch | **Accept.** As Devin F1, plus the release rule for min epoch ≤ 2 | `updater.go:459-460` |
| DeepSeek F3, the untrusted path is the band | **Accept.** The exact seed is a full paged maximum | `store.go:2528-2545` is the band |
| DeepSeek F4, the migration gap | **Accept.** `G = 2^20` unconditionally; the residual is stated | `runtime.go:4760-4768`; `store.go:2522` |
| DeepSeek F5, migration concurrency | **Accept.** The single-flight state machine | `store.go:2328`, `2560`, `2596` |
| DeepSeek F6, the lock order is wrong | **Accept the clarification. Reject the premise.** Round 1 never stated "eventMu then the appender mutex". It said "`Prepare` and `finalizeBatch` take `eventMu` under the appender mutex", which is the correct order. The full order is now stated | `computer_events.go:47`, `:140`; `appender.go:401-403` |
| DeepSeek F7, the lease row is witness content | **Accept the rule. Partly reject the premise.** `ogPut` supplies non-zero timestamps, so live and replay already agree. Fixed constants adopted for determinism | `graph_store.go:100-106`; `project.go:235-241` |
| DeepSeek F8, the carry-forward cannot work | **Accept. Dropped**, replaced by pinning the existing rewind refusal with a test | `computer_events.go:195`; `recovery_plan.go:66-71`, `:118-123`; `appender.go:1013-1022` |
| DeepSeek F9, `MAX()` GroupBy and the appender cite | **Accept.** The GroupBy note is the per-run comment, now marked as an assumption for `stream_seq`; `a.mu` is at `:402` after `awaitBarrier` at `:401` | `event_table.go:226-230` |
| DeepSeek, N of 1,024 to 2,048 | **Reject the number. Accept the method** (size against latency). The runway would be shorter than one observed engine stall | section 2 |
| Devin and DeepSeek open-question answers (keep the exact read, take the bump, the lock audit) | **Accept** | |
