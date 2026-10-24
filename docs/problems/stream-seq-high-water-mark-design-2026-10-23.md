# stream_seq high-water mark: a lease on the tape (design, 2026-10-23)

Date: 2026-10-23. Author: an FP0 design worker. Status: **design, round 3,
not built; ready to build** (bunch 164, Devin: "READY TO BUILD", no
silent-reuse or fail-open path). It answers the gate in
`docs/problems/stream-seq-seed-scans-the-event-kind-at-boot-2026-10-23.md`
("Gate on F-c deploy 2"), option 1, the persisted high-water mark.

**Mutation class of the build: red.** It touches:
- the event sequence allocator;
- the guest boot path;
- the store schema epoch;
- a new object kind written to the tape;
- the workspace swap points;
- the objectgraph writer fence.

**Review: tier 2.** Round 1 was bunch 162 (Devin, DeepSeek v4.1 flash).
Round 2 was bunch 164 (Devin). The dispositions are at the end.

Tags: [V] verified in code at the cited line on `origin/main` `b1abdfb0b`;
[H] hypothesis, not verified. Section numbers keep round 1's seven
questions. Section 8 is the build plan, and section 6's test list is the
tests-first list.

## The decision

1. **The mark is a lease**: the highest `stream_seq` this computer may hand
   out. The allocator never gives out a number above a lease that has passed
   the head CAS. On every path except migration (section 4), the lease is
   therefore at or above every number published: stored, queued for the
   sealer, or put on the bus by a failed append.
2. **The lease lives on the tape.** It is one `og_objects` object
   (`choir.stream_seq_lease`, external-key identity) written through the
   projection interceptor. There is no new table, column, index or tape
   format. The tape is the single authority, and the local row is its
   projection, which replay rebuilds.
3. **It is written ahead, in blocks of N = 65,536**, sized against refill
   latency (section 2). A single-flight refill starts in the background when
   half the block is used. Seeding is single-flight. No allocator ever holds
   a number above the durable lease.
4. **A trusted lease** seeds by one primary-key read plus a bounded
   cross-check. A cross-check above the lease makes the lease untrusted.
5. **Trust is fail-closed.** A lease is trusted only if all of these hold:
   - its row is present and self-consistent;
   - the prior store schema receipt was at least 3 (the epoch is bumped to
     3);
   - no untrust marker is in the workspace. Every workspace swap point, every
     base blob, and every low or absent prior receipt writes the marker,
     durably, before anything else;
   - `R` is under a fixed ceiling above the cross-check.

   Anything else takes the **exact seed**: a full paged scan of the event
   kind's `stream_seq`, plus the exact `choir_events` maximum, plus an
   unconditional gap `G = 2^20`. It then re-leases and clears the marker. On
   migration it starts eagerly after replay, and event appends wait for it
   rather than fail.

## 1. Where the mark lives

**What exists.**

- `choir_events` has no index led by `stream_seq`. Its indexes are the
  `(tape_seq, op_index)` PK, `(run_id, seq)`, `(owner_id, stream_seq)` and
  `(trajectory_id, stream_seq)` [V `event_table.go:70-73`].
- The store-wide seed reads the newest 500 rows by tape position
  [V `event_table.go:235-239`]: newest-carries-max over tape order, not a
  maximum.
- An exact maximum over `choir_events` needs no new state: one reverse seek
  per owner on `(owner_id, stream_seq)`, with owners listed by skip-seeks on
  the same index [H: no plan test yet; the builder pins one]. The owner-newest
  read is the existing precedent [V `event_table.go:217-220`].
- For `MAX(seq)` over a run, Dolt plans a GroupBy over the run's rows
  [V `event_table.go:226-230`, the per-run comment]. The same is assumed, not
  checked, for `MAX(stream_seq)`, so write `ORDER BY ... DESC LIMIT 1`.
- `og_objects` events have no `stream_seq` column; it is in `metadata`
  [V `graph_store.go:1173-1186`].
  - An index would need a STORED generated column (the `og_rec_inbox_seq`
    pattern, `dolt_store.go:262-268`), built by the derived-columns operation
    [V `derived_columns.go:14-19`].
  - That changes `SHOW COLUMNS`, which the witness hashes
    [V `computerversion/dolt_state_extractor.go:338-375`].
- A new `choir_events` index led by `stream_seq` would change that table's
  `SHOW COLUMNS` too, which is why its DDL is frozen
  [V `event_table.go:38-43`].

No zero-state read is exact for both halves.

**Why on the tape, as an object.**

| Home | Authority | Witness and schema | Survives |
|---|---|---|---|
| **og object through the interceptor (chosen)** | the tape | no schema change. `projectObject` is kind-agnostic [V `project.go:223-264`]. The row is witness content, made deterministic by section 7 | restart, crash, replay, base install, rebuild, disk loss |
| sidecar file beside the store marker | a second authority | none | restart and restore [V `agentcore/rematerialize.go:238-259` renames only the marker and the workspace]; not disk loss |
| a fold in `finalizeBatch` (a table or heads column) | the tape | a heads column changes a witnessed schema hash [V `dolt_state_extractor.go:338-375`]. A table needs `IgnoredTables` at every site [V `projectionbase/witness.go:25`, `agentcore/replay_completeness.go:494`], and older binaries call it unknown | restart, replay |

**The object.**

- Kind `choir.stream_seq_lease`; owner `__system__`; identity key
  `stream_seq_lease`; `IdentityExternalKey` [V `objectgraph/registry.go:21`],
  so the canonical ID is stable.
- Body and metadata (identical JSON):

  ```json
  {"reserved_through": R, "allocated_through": A, "block": N,
   "writer_epoch": 3, "basis": "exact" | "lease", "head_sequence": h}
  ```

  - `A` is the writer's `maxStreamSeq` when it wrote the lease.
  - `basis` says how the writing process seeded.
  - `h` is the local projection head sequence at write (diagnostic).
- Fixed `created_at` and `updated_at` (section 7).
- The read is `GetObject` on the canonical ID, one PK seek under the engine
  mutex [V `objectgraph/dolt_store.go:706-715`].
- **The plain upsert keeps the maximum.** One refill is in flight at a time,
  and each writes a larger `R`. The appender serialises appends under `a.mu`
  [V `appender.go:402`, after `awaitBarrier` at `:401`], so tape order is value
  order. No projector change is needed.
- **Older binaries.** Replay and older binaries project the row as an
  ordinary object; the projector does not call the write registry
  [V `objectgraph/registry.go:300-306`].

## 2. Write cost, atomicity, concurrency

**Lock order.** `flushMu` → appender `a.mu` → `eventMu` (→ the engine mutex
inside SQL).

- The sealer's flush holds `flushMu` and appends
  [V `telemetry_sealer.go:486-488`, `:587`].
- `AppendNewPayload` takes `a.mu` [V `appender.go:401-403`].
- `Prepare` and `finalizeBatch` take `eventMu` under it
  [V `computer_events.go:47`, `:140`].

`eventMu` is never held across an append. `AppendEvent` already releases it
before the tape write [V `store.go:2632-2635`]. Platform `writeMu`
(`platform/service.go:40`) is unrelated.

**Allocator state.** All fields are on `Store`, under `eventMu`, and `Reopen`
leaves them alone:

| Field | Meaning |
|---|---|
| `leaseState` | `unseeded`, `seeding` or `ready` |
| `reserved` | the durable `R` |
| `refillInFlight` | a refill is running |
| `leaseChanged` | a channel closed and replaced on every change |
| `refillFailed` | a count |
| `lastLeaseErr` | the last error |
| `seedBasis` | `exact` or `lease` |
| `untrustReason` | why the lease was not trusted |
| `seedMS` | the seed's duration |

**`allocateEventSeqLocked`** replaces today's `!s.streamSeqSeeded` block
(`store.go:2395-2399`) with a loop:

1. **`unseeded`**: set `seeding`, start the seeder goroutine, then wait
   (seeding deadline).
2. **`seeding`**: wait (seeding deadline).
3. **`ready` and `maxStreamSeq + 1 > reserved`**: if no refill is in flight,
   start one. Wait (ready deadline).
4. **`ready` and the number is inside `reserved`**: take
   `maxStreamSeq + 1`. If `reserved - maxStreamSeq < N/2` and no refill is in
   flight, start one asynchronously.

"Wait" means:
- release `eventMu`;
- select on `leaseChanged`, the deadline, and `ctx.Done()`;
- reacquire `eventMu`, and loop.

A deadline or a cancelled `ctx` returns an error. The append fails, and no
number is taken.

**Deadlines.**
- **Ready deadline: 30 s**, the sealer's seal timeout
  [V `telemetry_sealer.go:55`].
- **Seeding deadline: `max(10 min, 3 × the exact seed's measured time at
  paper scale)`.** The builder measures that time before the release, with
  `TestStreamSeqSeedScale` extended to the exact seed at the paper's event
  count, and records it in the record (Devin bunch 164 F3).

**Exactness during the scan.**
- While `seeding`, every allocation waits, so no `stream_seq` is minted or
  appended mid-scan. Only the allocator gives event rows a `stream_seq`.
- So releasing the engine mutex between scan pages is safe: the single-flight
  wait is the real barrier (Devin bunch 164).
- **The seeder's precondition** is that the local projection head equals the
  platform head when the scan starts. Then replay cannot add events during
  the scan, and the lease append, which requires equal heads
  [V `appender.go:712-717`], lands on the same state.
  - At boot this holds after replay [V `appender.go:1013-1022`].
  - If it does not hold, the seeder returns an error and the state goes back
    to `unseeded`.
  - The builder adds a read of the platform head to the appender if none is
    exported [H: the API name].

**The seeder.**
1. Compute the seed (section 3) with `eventMu` released.
2. Append the lease `R = seed + N`, with `A = seed` and the basis.
3. After the CAS succeeds, take `eventMu`, then:
   - `maxStreamSeq = seed`, `reserved = R`, state `ready`;
   - clear the marker on an exact seed (section 3);
   - broadcast.

On failure the state goes back to `unseeded`, `lastLeaseErr` is set, and the
waiters wake. They retry, or fail at their deadline.

**The refill.**
1. Under `eventMu`, take `target = max(reserved, maxStreamSeq) + N` and
   `A = maxStreamSeq`.
2. Release `eventMu` and append.
3. On success, `reserved = max(reserved, target)`.
4. On any error, including one after the CAS such as a finalize failure
   [V `appender.go:763-765`], leave `reserved` alone and count
   `refillFailed`.
5. Always clear `refillInFlight` and broadcast.

A refill after a failure waits at least 1 s. `reserved` never decreases.

**Atomicity under a crash.**
- A crash before the lease CAS: no number above the old `R` was handed out.
- A crash after the CAS and before the local finalize: the next boot replays
  the lease before the runtime starts [V `autoputer/run.go:747-749`;
  `appender.go:1013-1022`].

**Fail-closed cost.**
- A ready-state refill that cannot finish within the runway fails appends at
  30 s. That is an outage, not reuse, and it fate-shares with the tape append
  every bound append already needs.
- It shows as `refill_failed` and `last_lease_error`.

**Block size N = 65,536**, sized against refill latency:
- A refill is one tape append: `awaitBarrier`, up to the 30 s seal timeout
  [V `telemetry_sealer.go:55`, `appender.go:401`], then pin, CAS and finalize.
  The finalize can wait behind an engine-mutex hold; the paper has shown
  104 s.
- The runway is N/2 = 32,768 numbers. At the paper's 8 to 18 events/s
  [V `docs/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md`]
  that is about 30 minutes.
- Against a 134 s worst-case refill it covers about 240 events/s, a 13x
  margin over the measured peak.
- Refills come about every 30 minutes, each forcing one sealer flush.

Waste is at most N per process start.

**Gaps** are allowed and already present:
- a failed batch leaves a gap [V `store.go:2547-2554`];
- the sealer gap is 512 [V `store.go:2522-2524`];
- consumers use only the maximum and `> after`
  [V `frontend/src/lib/Desktop.svelte:1031-1042`, `event_table.go:212-215`].

**A fresh computer's first `stream_seq` is `2^20 + 1`**: the exact seed of
an empty store is 0, plus `G`. That is harmless, and expected in fixtures and
proofs.

## 3. The seed: trust, the exact seed, restore, rollback

### Trust (fail-closed)

A lease is **trusted** only if all of these hold.

**(a) Row.** The lease row is present and parses. It is self-consistent:
`writer_epoch >= 3` and `A <= R <= A + 3N/2`. A refill writes `R =
max(reserved, A) + N` with `reserved - A < N/2` at the trigger, so `R - A <
3N/2`. A seeder writes `R = A + N`.

**(b) Receipt.** The prior receipt read by `Open` was 3 or more.
- `Open` reads `store-schema.json` **before** it rewrites it. Today it only
  writes, at `store.go:964-970`.
- If the prior receipt is absent on a workspace that holds rows, or below 3,
  `Open` writes the untrust marker durably (write, fsync, rename, directory
  fsync) **before** `storeschema.Write`.
- **A marker-write failure aborts `Open`** before the receipt write. The store
  does not open, so there is no state in which receipt 3 exists without the
  marker the old receipt called for (Devin bunch 164 F4).
- So receipt 2, then `Open` writes 3, then a restart still finds the marker.
- `storeschema.Write` has no fsync [V `storeschema/schema.go:61-75`]. A lost
  receipt reads as absent, which writes the marker: still closed (Devin bunch
  164).

**(c) No marker.** `stream-seq-lease-untrusted.json` is absent from the
workspace root (beside `store-schema.json`).

**(d) Ceiling.** `R <= cross_check_max + C`, with the ceiling `C = 2^40`.

**Why (d) is a ceiling and not round 2's `N + G` margin** (Devin bunch 164
F2). Round 2's margin failed a legitimate lease whenever the cross-check
underestimated the stored maximum. That made the exact seed recur on every
boot of a computer whose newest windows held no event: the recurring cost this
design exists to remove.

- Stale detection is the cross-check exceeding `R`, not the margin.
- Same-lineage consistency is (a): the row checks against its own `A`.
- Forgery is the hard writer fence (section 6).
- The ceiling only caps what a forged, self-consistent row behind the fence
  could burn: at most 2^40 numbers, once, which leaves the browser's 2^53 far
  away.
- No legitimate lineage reaches 2^40 above its stored maximum. That would
  take about a trillion failed-append allocations.

So the exact seed runs once per migration, swap or rollback-forward, never
per boot.

There is no "the store is fresh" exception. With no lease row the seed is the
exact seed, which on an empty store is cheap.

### The untrust marker: every writer, every swap point

The marker is a file written with fsync by `storeschema.WriteLeaseUntrusted`,
a new leaf helper with the `WriteFloor` pattern of lane
`fp1-event-table-d2`. It is written into the incoming workspace before that
workspace is renamed into place or opened. The swap points, enumerated by a
grep of renames into the live store and workspace paths:

| Swap point | Where the marker is written |
|---|---|
| `projectionbase.installDescriptor` (`install.go:110`, staged renames `:186-190`) | in the staged workspace. This covers both entry points: `InstallVerifiedBase` (`:53`, calls it at `:84`) and `InstallPinnedBase` (`:90`, calls it at `:107`) [V] |
| `projectionbase.RebaseRetainedStore` (`rebase.go:19`, flip `:58-78`) | in the staged workspace, before the flip [V] |
| `RematerializeFromTape` (`agentcore/rematerialize.go:227-262`, flip `:238-259`, `Reopen` `:262`) | in the staged workspace after `staged.Close()`, before the flip [V] |
| `Store.ReplaceWorkspace` (`store.go:822-861`) | in the fresh workspace `OpenFresh` created, before it returns [V]. Benign today: the fresh workspace has no lease row, so (a) already fails, and the runtime restarts afterwards (`agentcore/workspace_replace.go:57-77`). The marker makes the guard test uniform |
| `projectionbase` base publisher (`publisher.go:67-104` tars every file of the workspace subtree; cleanup removes only seed metadata, `install.go:330-337`) | a marker entry in every blob, so a base is untrusted by construction [V] |

At boot, `store.Open` runs before install or rebase [V `autoputer/run.go:200`
against `:366`]. Rebase `Reopen`s the swapped workspace
[V `autoputer/projection_base.go:173-200`]. The receipt that `Reopen`'s `Open`
reads is the base's, so the marker carries the decision.

**Guard test.** In the `event_table_guard_test.go` AST pattern, it fails if
any function in `internal/store`, `internal/projectionbase` or
`internal/agentcore` that renames into a store or workspace path, or calls
`OpenFresh` for a live path, does not call `storeschema.WriteLeaseUntrusted`.

**Clearing.** The marker is cleared, after a successful lease CAS, only by a
seeder whose basis is `exact`.

**VM hibernate and snapshot resume: a stated residual.**
`ResumeVMFromSnapshot` (`vmmanager/snapshot_lifecycle.go:440`) restores guest
memory and disk. A snapshot resumed after the computer's tape advanced would
bring back an old workspace and an old in-memory allocator (`ready`, a stale
`reserved`) that no guest-side marker reaches. Its appends are refused while
the local head is behind the platform head [V `appender.go:712-717`]. But
`emitEvent` publishes the number it was allocated even when the append fails
[V `agentcore/runtime.go:4760-4768`], so such a resume can put reused numbers
on the bus until the next cold boot. See open question 4.

### The exact seed

When the lease is absent or untrusted, the seed is the **maximum, not a
band**:

- **og half.** A full scan of the event kind:
  - keyset paged over `idx_og_objects_kind_owner` and the PK suffix
    (`ListObjectsPage`, `dolt_store.go:1322`, is the keyset precedent
    [V]);
  - `metadata` only, 5,000 rows a page, releasing the engine mutex between
    pages;
  - a row whose metadata has no `stream_seq` has its body decoded.
- **table half.** The exact `choir_events` maximum by per-owner reverse seeks.
  If the plan test shows no seeks, a full scan of the column.
- **The result.** `seed = max(og, table, R') + G`, where `R'` is the existing
  lease's `R` when it is self-consistent (a) and `R <= exact + C`, and is
  otherwise ignored with a log line naming it. `G = 2^20` is applied
  unconditionally, batching on or off.

Taking a legitimate `R` covers the numbers that lineage published and never
stored. The exact maximum covers anything an older binary stored.

**Cost.** Linear in the event history, in the class of the 104 s read. It
reads metadata, not body [H: Dolt keeps large blobs out of the row, so it may
be cheaper; the builder measures it]. `/health` and other engine readers
proceed between pages. Event appends wait (seeding deadline).
`seedStreamSeqExact` (the 500 newest by `updated_at`) is no longer used by the
seed.

### Trusted seed

1. `seed = R`.
2. The cross-check reads `choir_events` newest-500 by key
   (`eventTableNewestStreamSeq`) and the og window of 2,000, widened to 50,000
   when the first window holds no event (`seedStreamSeq`'s windows, kept).
3. If `cross_check_max > R`, the lease is stale: `untrustReason=violated`, the
   exact seed runs, and a log line gives both numbers.
4. Otherwise a fresh block is appended (`R + N`, basis `lease`) before the
   first number `R + 1`.

### Replay, restore, rollback

- **Replay** projects op bodies verbatim and never calls the allocator
  [V `appender.go:879-890`, `project.go:223-264`].
- **Every boot** replays to the platform head before the runtime starts
  [V `run.go:747-767`; `appender.go:1019-1021`, the `sameHead` check that
  returns `ErrNeedsProjectionRepair`; fatal at `run.go:781`]. Rebase replays
  (W, H] after the swap [V `recoveryplan/recovery_plan.go:32-34`].
- **In-process restore.**
  - `RematerializeFromTape` reconstructs only to the target
    [V `rematerialize.go:164`]; later events stay on the tape
    [V `appender.go:832-834`].
  - It `Reopen`s [V `:262`]. `Reopen` preserves `maxStreamSeq` and
    `streamSeqSeeded` [V `store.go:1188-1268`], and it must preserve the lease
    fields. The state stays `ready`, so no seeder runs in-process.
  - Refills fail while the heads differ [V `appender.go:712-717`], so appends
    fail as they already do after a restore.
  - At the next boot the marker forces the exact seed.
- **Binary rollback across the epoch bump.**
  - The reverted epoch-2 binary ignores the row, seeds by D13's path, and
    rewrites the receipt to 2.
  - The next epoch-3 `Open` reads 2, writes the marker, and the exact seed
    runs.
  - `Apply` refuses an epoch-2 release on an epoch-3 store
    [V `updater/updater.go:456-458`]. The CI revert, the platform-follow offer
    and `restorePrior` are unfenced [V `storeschema/schema.go:30-37`], and the
    receipt-plus-marker rule catches them.
  - **Release rule:** the epoch-3 release keeps `min_store_schema_version` at
    2 or below [V the refusal at `updater.go:459-460`].
  - D2's floor (lane `fp1-event-table-d2`, not on main) is separate. The lease
    does not raise it.
- **Head rewind.** No restore carry-forward append: it would carry process
  memory that a real rewind loses. The existing fence holds:
  - the local head advances only by a CAS on its current sequence
    [V `computer_events.go:195`];
  - appends are refused while the heads differ [V `appender.go:712-717`];
  - a boot whose local head is ahead of the platform head resumes with no
    tail (`recovery_plan.go:66-71` only bounds the tail), then fails
    reconstruct's `sameHead` check [V `appender.go:1019-1021`], and the boot is
    fatal [V `run.go:781`].

  A test pins this fence.

## 4. Migration (no lease yet)

- **Eager.** The seeder starts in a goroutine in `runReplayPhase`, right after
  the vocabulary fence (`bootMark("vocab_fenced")`, `autoputer/run.go:772`),
  before the runtime starts serving. Replay has finished there, so the head
  precondition holds.
- **First allocations.** Any that arrive first (the boot passivation phase)
  wait in `seeding` up to the seeding deadline, which is at least 3x the
  measured seed time.
- **No planned outage.** Migration delays event appends; it does not fail
  them. An append fails only if the seed overruns 3x its measured time, or
  errors. Both are logged, and both mean the seed itself is broken.
- **What it holds.** The engine mutex one page at a time. Not `eventMu`.
  Event appenders wait.
- **The gap.** `G = 2^20`, unconditionally (DeepSeek bunch 162 F4).
  - `emitEvent` publishes a failed append's number
    [V `agentcore/runtime.go:4760-4768`], and today's seed adds no gap with
    batching off [V `store.go:2522`].
  - Migration is the one boot with no bound on numbers the last
    lease-unaware process published and never stored. More than 2^20 of them
    is the stated residual.
- **When.** Before `store.event_table` is switched on for that computer. The
  deploy-2 writer refuses to stamp, and the setting refuses to turn on, until
  the lease state is `ready` with a lease on the tape [H: the setting's code is
  on lane `fp1-event-table-d2`].
- **Fresh store.** The seed is 0 + G (section 2).

## 5. D12's per-run seq, and the tape

**The tape.** `stream_seq` is on the signed tape:
- the projection batch's op bodies carry it
  [V `graph_store.go:1173-1186`, `projection_tape.go:66-77`];
- the event commits to the payload by digest [V `appender.go:412-417`];
- the receipts are signed with ed25519 [V `computerevent/receipt.go:5,26-40`].

`choir_events` projects it into a column [V `event_table.go:67`]. Replay does
not re-derive it, and the lease rides the tape the same way.

**D12.**
- `AllocateEventSeq` shares `allocateEventSeqLocked`
  [V `store.go:2555-2569`]. Its caller holds no store lock across the later
  batch append [V `store.go:1524-1528`], so folded events wait like any
  allocation.
- Per-run `seq` keeps its own seed [V `store.go:2346-2386`] and is not covered
  by the lease.
- The per-run counter re-takes a failed unbatched append's number by design
  [V `store.go:2643-2649`]. That is outside this design.

## 6. Class, surfaces, fence, rollback, heresy, tests, proof

**Mutation class:** red, tier 2.

**Protected surfaces:**
- the stream allocator and its concurrency;
- the boot path (replay phase, first allocation);
- `store.Open` (the receipt read and the marker);
- `storeschema.Version`;
- the five swap points of section 3;
- the objectgraph registry and the writer fence;
- tape writes of the new kind.

**Not touched:** the projector, the extractor, the replay manifest and the
tape format.

**Writer fence** (Devin bunch 162 F2 and bunch 164 F4):
- **The writer.** Add `WriterStore Writer = "system:store"` to `writer.go` and
  to `BuiltinWriters` [V `writer.go:62-65`], or
  `TestEveryRegisteredKindNamesExactlyOneKnownWriter` fails. Register
  `choir.stream_seq_lease` in `DefaultRegistry` with `Store: StoreTypeDolt`,
  `IdentityMode: IdentityExternalKey`, `Owner: OwnerVMStore` and `Writer:
  WriterStore`.
  - Not `WriterRuntime`: a runtime kind admits every declared writer
    [V `writer.go:199-202`].
  - An unregistered kind is refused [V `registry.go:307-314`].
- **The hard fence goes in `CheckWriteAs`** (`writer.go:217`). Put it
  immediately after its `CheckWrite` call (`:218`) and before the gate-mode
  check, so it holds in off, observe and enforce alike.
  - It refuses any non-tombstone object of kind `choir.stream_seq_lease`
    unless `ctx` carries the store-private lease capability (an unexported
    context key with a `WithStreamSeqLeaseCapability(ctx)` constructor).
  - `CheckWrite` takes no `ctx` [V `registry.go:307`], which is why the fence
    sits in `CheckWriteAs`.
  - `CheckWriteAs` runs on the VM store's object write paths
    [V `dolt_store.go:630`, `:643`, `:1138`] and on corpusd's
    [V `platform/objectgraph_store.go:104`, `:154`, `:611`].
  - The gate defaults to observe [V `writer.go:110-115`], so the registry alone
    would not refuse a desk write.
- **Guard test.** The only non-test caller of
  `WithStreamSeqLeaseCapability` is `internal/store/stream_seq_lease.go`.
- **Replay.** It does not call `CheckWrite` [V `registry.go:300-306`], so the
  fence cannot poison it.

**Rollback path.** Revert the lease commits.
- The reverted binary ignores the row and seeds by D13's path. That is correct
  while events are in `og_objects`; if deploy 2's switch is on, turn it off
  first.
- `Apply` refuses epoch-2 releases, so the CI revert is the route.
- Forward again: receipt 2, the marker, the exact seed once.

**Heresy delta.**
- Discovered:
  1. `emitEvent` publishes a failed append's `stream_seq`, and no gap is added
     with batching off [V `runtime.go:4760-4768`, `store.go:2522`].
  2. After a restore, appends are refused until a reboot replays past the
     target [V `appender.go:712-717`, `:1019-1021`]. Whether a restore
     survives a reboot is unverified [H].
  3. The `maxStreamSeq` comment is stale [V `store.go:248-252`].
  4. Bases carry `store-schema.json`, so the receipt is per workspace copy
     [V `publisher.go:67-104`].
  5. A snapshot resume can rewind the workspace and the allocator with no
     guest-side signal (section 3, residual).
- Introduced (by the design):
  - an allocator-written kind;
  - gaps of up to N per process start, and 2^20 per exact seed;
  - fresh computers start at `2^20 + 1`;
  - the epoch bump's `Apply` fence;
  - fail-closed append outages when a refill fails past the runway;
  - one exact seed per swap.
- Repaired (once built):
  - the exact read at every boot after deploy 2;
  - the `updated_at` band, since no seed path uses `updated_at`;
  - discovered item 1, after migration.

**Tests first.** The builder writes these failing, then the code. Local E2E
on the store unless marked.

Trust:
1. Receipt 2, `Open` (writes 3), restart before any append, reopen: the
   marker is present, `untrustReason=receipt`, exact seed.
2. Lease row present and receipt absent: untrusted.
3. A base built from, and installed (plus a rebase) onto, a workspace with
   receipt 3 and a stale lease, after an epoch-2 interlude that allocated past
   `R`: untrusted once, no reissue.
4. Rematerialize, then a restart: marker present, exact seed.
5. `InstallPinnedBase`, as well as `InstallVerifiedBase`, leaves the marker in
   the installed workspace.
6. `ReplaceWorkspace` leaves the marker in the fresh workspace.
7. The swap-point AST guard test.
8. `Open` reads the receipt before it writes it (ordering test).
9. A marker-write failure (injected) aborts `Open`, and the receipt is
   unchanged.
10. A self-inconsistent row (`R > A + 3N/2`, or `writer_epoch < 3`):
    untrusted.
11. A legitimate lease whose cross-check underestimates the maximum, because
    the windows hold no event: trusted, with no exact seed on the second boot
    (bunch 164 F2).

Exact seed:
12. An epoch-2 high number behind 500 newer events and 50,000 objects is not
    reissued after forward.
13. Untrusted runs the full exact seed even when the first window holds an
    event.
14. With batching off, a failed append in the previous process: its
    published number is not reissued after migration.
15. DeepSeek's bunch-156 counterexample still allocates above 12.
16. A fresh store's first `stream_seq` is `2^20 + 1`.
17. A forged self-consistent `R` above `exact + C` is ignored with a log line.

Concurrency:
18. Two concurrent first allocations during migration: one seeder, no
    unleased number.
19. A refill failure clears the in-flight flag, and the next allocation
    refills after 1 s.
20. Two allocators trigger one refill.
21. A refill error after the CAS: `R` never decreases.
22. A seeder lease-append failure: no number is handed out, and the state
    returns to `unseeded`.
23. The folded path (`AllocateEventSeq`) waits on an exhausted lease.
24. With batching on and a sealer reservation held, the first allocation
    waits for the lease: no deadlock, and the ready deadline fires.
25. The seeder refuses when the local head is not the platform head.
26. An eager seed started after replay, with an allocation arriving
    mid-seed: it waits (seeding deadline) and gets `seed + 1`, not an error.

Writer fence:
27. A lease write without the capability is refused under all three gate
    modes, including a desk-declared writer.
28. With the capability, a write passes under enforce.
29. The capability-caller guard test.
30. `TestEveryRegisteredKindNamesExactlyOneKnownWriter` passes with
    `system:store`.

Witness and replay:
31. The lease op carries the fixed timestamps. A base witness equals a replay
    witness after lease appends.
32. The lease row survives `MigrateAndFenceServingVocabulary` after a replay
    with applied rows, with its key unchanged.
33. A boot whose local head is ahead of the platform head refuses (the
    rewind fence).

Kept from round 1:
34. No `ListObjects choir.event` on a trusted reopen (engine-mutex counters).
35. A crash between the lease CAS and finalize, then replay, seeds above.
36. The 2,000/50,000 window tests as the cross-check.

Measurement (env-gated, before the release):
37. `TestStreamSeqSeedScale` extended to the exact seed at 20,000, 60,000 and
    the paper's event count, if feasible locally. Its time sets the seeding
    deadline.

**Deployed proof.** FP0 runs it read-only on live state: no SQL, no SSH.

The build adds a `stream_seq_lease` block to `HandleGuestObservability`
(`agentcore/api.go:956`, `GET /api/runtime/observability`):
`state`, `reserved_through`, `allocated_through`, `trusted`, `untrust_reason`
(`no_lease`|`receipt`|`marker`|`inconsistent`|`ceiling`|`violated`),
`seeded_from` (`exact`|`lease`), `seed_ms`, `cross_check_max`,
`refill_failed`, `last_lease_error`, `marker_present`.

1. **Staging test computer** with history:
   - **First boot:** `seeded_from=exact`, `untrust_reason=no_lease`,
     `seed_ms`, and no append errors during the seed.
   - **A planned restart:** `seeded_from=lease`, `seed_ms` under 50, and no
     `ListObjects choir.event` on the engine mutex.
   - **A WebSocket cursor script:** record S1, restart, run a prompt, and
     assert every new `stream_seq` > S1 and that `after_seq=S1` returns them.
     JSON artifact.
2. **A restore** (an owner checkpoint), then reboot: `untrust_reason=marker`,
   exact once, then `lease`.
3. **Deploy 2 on**, then restart: `lease`, with no event-kind read.
4. **Optional:** a CI-revert rollback rehearsal, then forward:
   `untrust_reason=receipt`.
5. **The paper**, at its next planned refresh: `exact` and its `seed_ms`; no
   event-append errors; the first-passivation time; the `/health` 503 window.
   On the refresh after that, `lease`. Only then is deploy 2 eligible there.

## 7. Witness determinism and alternatives

**Witness content.**
- The witness hashes rows [V `dolt_state_extractor.go:388-403`].
- `projectObject` substitutes the current time only for zero timestamps
  [V `project.go:235-241`].
- `ogPut` passes `now` [V `store/graph_store.go:100-106`], so live and replay
  already agree.
- The lease op still carries fixed `created_at` and `updated_at`
  (`2000-01-01T00:00:00Z`), for three reasons:
  - every lease row is a pure function of its fields;
  - a base and a replay agree;
  - the old `updated_at` keeps the row out of the newest-objects windows.

The lease write therefore goes through a store method that builds the
`objectgraph.Object` with those timestamps, not through `ogPut`'s `now`.

**Alternatives.**

| Option | Closes the boot read | Closes the `updated_at` band | Covers published-not-stored | Verdict |
|---|---|---|---|---|
| **Lease on the tape (chosen)** | yes | yes | yes, after migration | chosen |
| Lease in a sidecar file | yes | yes | yes | lost with the disk; a second authority |
| Fold in `finalizeBatch` | yes | yes | no | touches the witness boundary |
| `(object_kind, updated_at)` index (option 2) | yes | no | no | an online build on 31 GiB [H] |
| `og_meta_stream_seq` column and index | yes | yes for og | no | the heaviest |
| `choir_events` owner seeks alone | the table half | no | no | used inside the exact seed |
| Status quo (D13) | no: 104 s per boot after deploy 2 | no | no | the reason for this record |

## 8. Build plan (for the builder)

**Order.**
1. Commit the tests of section 6 (yellow): they fail on main.
2. Commit the code (red).
3. Commit the measurement (37) result into this record (green) before the
   release sets the seeding deadline.

**Files.**
- `internal/storeschema/schema.go`:
  - `Version = 3`, with the epoch 3 line in the doc comment;
  - `LeaseUntrustedFile = "stream-seq-lease-untrusted.json"`;
  - `WriteLeaseUntrusted(workspacePath, reason)` (fsync, rename, directory
    fsync);
  - `LeaseUntrusted(workspacePath) (bool, error)`;
  - `ClearLeaseUntrusted(workspacePath)`.

  It is a leaf package, so `projectionbase` and the publisher can call it
  without the engine.
- `internal/objectgraph/writer.go`: `WriterStore`, `BuiltinWriters`, the
  capability key and constructor, and the fence in `CheckWriteAs`.
- `internal/objectgraph/registry.go`: the kind registration.
- `internal/store/stream_seq_lease.go` (new):
  - the constants: `N`, `G`, `C`, the deadlines, the 1 s backoff, the page
    size of 5,000, the fixed timestamp, the identity key;
  - the lease read and write (with the capability);
  - the state machine, the seeder, the refill, the exact seed, the trust
    evaluation, and the observability snapshot.
- `internal/store/store.go`:
  - the lease fields on `Store`;
  - `allocateEventSeqLocked`'s loop;
  - in `Open`, the prior-receipt read and the marker before
    `storeschema.Write`, aborting on error;
  - `ReplaceWorkspace`'s marker;
  - a corrected `maxStreamSeq` comment.

  `seedStreamSeq`'s windows are kept as the cross-check, and
  `seedStreamSeqExact` is replaced by the exact seed.
- `internal/projectionbase/install.go` (`installDescriptor`), `rebase.go` and
  `publisher.go`: the marker.
- `internal/agentcore/rematerialize.go`: the marker in the staged workspace
  before the flip.
- `internal/autoputer/run.go`: start the eager seed after the vocabulary
  fence.
- `internal/agentcore/api.go`: the observability block.
- Lane `fp1-event-table-d2` (when it lands): the `store.event_table` setting
  and the writer refuse until the lease is `ready`.

**Constants.**

| Name | Value |
|---|---|
| N (block) | 65,536 |
| Refill trigger | N/2 |
| G (exact-seed gap) | 2^20 |
| C (ceiling) | 2^40 |
| Ready deadline | 30 s |
| Seeding deadline | `max(10 min, 3 × measured exact seed)` |
| Refill backoff | 1 s |
| Page size | 5,000 |
| Lease timestamps | `2000-01-01T00:00:00Z` |
| Owner / identity key | `__system__` / `stream_seq_lease` |

## Open questions (round 3)

1. **The exact seed's real time on the paper** is unmeasured. Test 37 sets it
   before the release, and the seeding deadline follows from it.
2. **Clearing the marker in-process** after a restore: does a process whose
   memory dominates need the exact seed? This design keeps the one exact seed
   per swap, to stay conservative.
3. **The per-owner skip-seek plan on `choir_events`** needs a plan test. The
   fallback is a full column scan, still exact.
4. **Residual: VM snapshot resume** (`vmmanager/snapshot_lifecycle.go:440`).
   A stale snapshot resumed after the tape advanced rewinds the workspace and
   the in-memory allocator with no guest-side marker. Appends are refused
   until the heads agree, but a failed append still publishes its number.
   - A real close needs a host signal: a vmctl resume hint that sets
     `unseeded` and the marker on resume, or the planned "a computer with a
     pending update boots fresh" rule.
   - Not in this build.

## Round 2: dispositions of bunch 162

| Finding | Disposition | Code |
|---|---|---|
| Devin F1, a trusted lease stale after a base install or rebase | **Accept.** Markers at every swap point and in every base | `publisher.go:67-104`; `install.go:330-337`; `run.go:200` before `:366` |
| Devin F2, write gates, a forged R, the refill wedge | **Accept.** `system:store`, the hard fence, the ceiling, `refill_failed` | `registry.go:307-314`; `writer.go:110-115`, `199-202` |
| Devin F3, the cross-check narrower than migration | **Accept.** Widened; a violation makes the lease untrusted | `store.go:2451-2526` |
| Devin F4, citation drift | **Accept.** Fixed | |
| DeepSeek F1, receipt written before repair | **Accept.** The marker comes before receipt 3 | `store.go:964-970` |
| DeepSeek F2, bases launder the receipt; min epoch | **Accept** | `updater.go:459-460` |
| DeepSeek F3, untrusted is the band | **Accept.** The exact seed is a full maximum | `store.go:2528-2545` |
| DeepSeek F4, the migration gap | **Accept.** `G` unconditional | `runtime.go:4760-4768`; `store.go:2522` |
| DeepSeek F5, migration concurrency | **Accept.** Single-flight | `store.go:2328`, `2560`, `2596` |
| DeepSeek F6, lock order | **Accept the clarification. Reject the premise** (round 1 said `eventMu` under the appender mutex) | `computer_events.go:47`, `:140` |
| DeepSeek F7, witness content | **Accept the rule. Partly reject the premise** (`ogPut` supplies non-zero timestamps) | `graph_store.go:100-106` |
| DeepSeek F8, carry-forward | **Accept. Dropped**; the rewind fence is pinned | `computer_events.go:195`; `appender.go:1019-1021` |
| DeepSeek F9, the GroupBy and appender cites | **Accept** | `event_table.go:226-230`; `appender.go:401-402` |
| DeepSeek, N of 1,024 to 2,048 | **Reject the number. Accept the method** | section 2 |

## Round 3: dispositions of bunch 164 (Devin: READY TO BUILD)

| Finding | Disposition | Code |
|---|---|---|
| F1, swap-point inventory | **Accept.** `ReplaceWorkspace` named; the marker sits in `installDescriptor`, so `InstallPinnedBase` is covered; VM snapshot resume is a stated residual (open question 4) | `store.go:822-861`; `install.go:84`, `:90`, `:107`, `:110`; `vmmanager/snapshot_lifecycle.go:440` |
| F2, bound (d) makes the exact seed recurring | **Accept. Provenance chosen**, not an accepted recurrence: `A` and `basis` in the body, so (a) checks the row against its own lineage, and (d) becomes a 2^40 ceiling. Why: a recurring scan of the event kind on every boot is the defect this design removes, and forgery is the fence's job, not the margin's | section 3 |
| F3, the 30 s deadline against a 104 s-class seed | **Accept.** An eager seed after replay, and a seeding deadline of at least 3x the measured time. No planned outage | `run.go:772`; section 4 |
| F4, fence mechanics | **Accept.** The fence goes in `CheckWriteAs` before the mode check; `system:store` joins `BuiltinWriters` (it is at `writer.go:62-65` on main, not `:52-56`, which hold the writer constants); a marker-write failure aborts `Open` | `writer.go:62-65`, `:217-218`; `registry.go:307` |
| F5, cite drift | **Accept 3. Reject 2.** Fixed: `dolt_store.go:706-715`; the rebase flip `:58-78`; the rewind fence cited at `appender.go:1019-1021`, with `recovery_plan.go:66-71` named only as the tail bound. Rejected: `queryTableContentHash` does start at `dolt_state_extractor.go:388` (`:382` is the schema hash's return), and `DisallowUnknownFields` is at `storeschema/schema.go:88`, not `:84`. Added the `2^20 + 1` first-number line | the cited lines |

## Build record (2026-10-23, lane `lane/fp0-stream-seq-lease`)

Status: **built, not deployed.** Tests first (`6c7a435a8`, yellow), then the
code (red), then this section (green). The deployed proof of section 6 is
FP0's; nothing here touched Node B.

### Test status

All 37 exist and pass on the lane. Test 3 is split: the store half (an
epoch-2 interlude) and the base half (a published blob carries the marker, an
installed base opens untrusted and seeds exact).

| Tests | Where |
|---|---|
| 1, 2, 3 (store half), 6, 8, 9, 10, 11 | `internal/store/stream_seq_lease_test.go` |
| 3 (base half), 5 | `internal/projectionbase/lease_marker_test.go` |
| 4, 31 (replay equivalence), observability | `internal/agentcore/stream_seq_lease_test.go` |
| 7, 29, 8 (AST order) | `internal/store/stream_seq_lease_guard_test.go` |
| 12 to 17 | `internal/store/stream_seq_lease_test.go` |
| 18 to 26 | `internal/store/stream_seq_lease_test.go` |
| 27, 28, 30 | `internal/objectgraph/stream_seq_lease_fence_test.go` |
| 31 (fixed timestamps), 32, 34, 36 | `internal/store/stream_seq_lease_test.go` |
| 33 | `internal/computerevent/heads_aligned_test.go` |
| 35 | `internal/store/stream_seq_lease_crash_test.go` |
| 37 | `internal/store/stream_seq_exact_seed_scale_test.go` (env gated) |
| epoch 3, release rule | `internal/storeschema`, `internal/updater/state_compat_lease_test.go` |
| eager seed placement | `internal/autoputer/stream_seq_seed_boot_test.go` |

### Test 37: the exact seed's cost, measured

Machine: Apple M1, 8 CPUs, 16 GiB, `darwin/arm64`, Go 1.26.9, APFS on the
internal SSD (`/dev/disk3s1s1`, local). Warm (best of 3, same process, page
cache hot). Events carry 1.5 KB payloads over 50 runs; one other-kind object
per three events is interleaved; the maximum `stream_seq` sits on the oldest
event; `choir_events` holds n/10 rows over 20 owners.

| events | og half, record's keyset page | og half, primary-key id range (built) | table half |
|---|---|---|---|
| 20,000 | 474 ms | 53 ms | 11 ms |
| 60,000 | 6,554 ms | 148 ms | 11 ms |
| 150,000 | 24,291 ms | 653 ms | 10 ms |

**[V] refuted by the code.** Section 3 said the exact seed's og half is a
keyset page over `idx_og_objects_kind_owner` and the PK suffix, with
`ListObjectsPage` as the precedent. The plan (`EXPLAIN PLAN`, in the test log)
is an `IndexedTableAccess` on a *different* composite index
(`object_kind, og_rec_conversation_id, canonical_id`) under a
`TopN(Limit 5000; canonical_id ASC)`: every page re-reads and sorts the kind,
so the walk is quadratic in pages and was 25 s at 150,000 events. Fail-closed
choice: the walk is the primary-key range of the kind's canonical ids
(`obj:<kind>:` to `obj:<kind>;`, the `derivedKeyRange` precedent), a plan with
no sort, 4.4 us an event, linear. A row of the kind outside that range makes
the walk fall back to the kind-predicate pages (slow, exact), so the answer is
exact whatever ids rows carry (`TestExactSeedFindsAnEventOutsideTheKindIDRange`).

The seeding deadline: `max(10 min, 3 x 665 ms)` is the 10-minute floor.
`leaseExactSeedMeasured` is 665 ms. The paper's own figure is unmeasured (its
event count is not on this machine, and its first read is cold: the 104 s of
the seed record was cold chunk reads). The first paper refresh reports
`seed_ms`; if it exceeds about 200 s the floor must be raised before the
refresh after. Open question 1 stays open for the paper, closed for the local
shape.

**Open question 3 closed.** The per-owner skip-seek on `choir_events` plans as
a reverse `IndexedTableAccess` on `(owner_id, stream_seq)` with no sort
(`TestExactSeedTablePlanSeeksPerOwner`); 10 ms at 15,000 rows.

### Deviations and additions

Deviations from the record:

1. The og half of the exact seed walks by primary-key id range, not by the
   kind-predicate keyset (above).
2. Stream numbers are taken **before** the per-run seq in
   `allocateEventSeqLocked`. A wait releases `eventMu`; taking the run's seq
   first would let two appends to one run take seq in one order and stream_seq
   in the other.
3. `Close` cancels lease work without a lock and drains it after the
   commit-lock fence, before the pools are pinned. The first build took
   `eventMu` at the start of `Close`; an inline commit holds `eventMu` across
   the fence, and three checkpoint tests (`TestCloseFailedAtTheFenceRestoresCadence`
   and two others) caught it: `Close` waited 21 s and then succeeded where it
   must fail closed. A closing store refuses new lease work and fails an
   allocation fast until `Reopen` (or a failed `Close`) resumes.
4. `AllocateEventSeq` callers sharing one record: the "already numbered"
   check is repeated inside the wait loop (`TestAllocateEventSeqSharedRecordTakesOneNumber`
   caught four callers taking four numbers after a shared wait).
5. Untrust reason precedence is `no_lease`, then `receipt` or `marker`, then
   `inconsistent`, `ceiling`, `violated`. A store with no lease row reports
   `no_lease` even with a marker, so the proof's first boot on an epoch-2
   store reads `no_lease`, and a rollback reads `receipt`.
6. `Open` writes the marker for an absent receipt only on a workspace that
   already holds data (a fresh workspace is not a swap). An unreadable
   receipt writes it.
7. An outage before a process's first allocation makes first allocations wait
   for the seed, up to the seeding deadline (10 minutes), and then fail; they
   do not fail at once as a store with a lease does. `TestPlatformOutageIsBounded`
   now seeds first.
8. The eager seed is skipped in the replay-only host drive (`RUNTIME_RECOVERY_REPLAY_ONLY`):
   a lease append is an append, and that drive must not append.
9. `last_lease_error` is sticky: the last error, not the current one.
10. The telemetry test harness records lease-only batches apart
    (`leaseSnapshot`) so the workload tape keeps its shape. The flag-off golden
    (`telemetry_flag_off_tape.golden.json`) is regenerated: event bodies carry
    `stream_seq`, which now starts at 2^20 + 1. Only hashes and the
    `stream_seq` field changed. `TestTelemetryStats...` count the lease as one
    tape event.
11. Existing tests that pinned `stream_seq` values (a fresh store began at 1)
    pin the lease's numbers instead: 2^20 + 1 fresh, `max + 2^20 + 1` for an
    exact seed, above the previous lease on a trusted restart. The old window
    tests became cross-check tests (test 36).
12. Unbound stores (tests, no tape) write the lease by direct SQL and have no
    head precondition.

Additions from bunch 169 (Devin, round 3: nothing blocking):

1. **Medium-Low, the seeder's retry.** Chosen: a retry backoff (1 s,
   doubling per consecutive failure, capped at 30 s) and a counter, surfaced
   in `last_lease_error` ("seed attempt N failed: ...") and the new
   `seed_failed` field (`TestSeederRetriesWithABackoffAndACounter`,
   `TestSeederFailureNamesTheAttemptInLastLeaseError`). Not built: the
   head-delta retry. It needs a classification of every op kind between the
   scan's head and the retry's, and a mistake in it silently reuses a stale
   scan; backoff is the fail-closed choice that can be tested. Reading the
   code: an intervening desk write does not fail the lease CAS in-process
   (`appendLocked` binds the head under the appender mutex), and no event can
   be minted while seeding, so a rescan is needed only when an append really
   failed; the premise is recorded as unverified on a live always-busy
   computer.
2. **Low, a tombstoned lease row.** Logged and counted
   (`lease_tombstoned`); the row is untrusted and the exact seed runs
   (`TestTombstonedLeaseRowIsUntrustedLoggedAndCounted`).
3. **Low, the capability.** `WithStreamSeqLeaseCapability` inherits the
   `WithWriter` trust boundary: both are context values set by trusted
   in-process code, never from a cell, a model or a request body, and the
   source guard (test 29) is what keeps the capability's caller list to
   `internal/store/stream_seq_lease.go`.
4. **Nit, test 37** records the machine and disk context (above, and in the
   test log).

### The observability block

`GET /api/runtime/observability` carries `stream_seq_lease`, served from a
snapshot published on each state change (reading it never waits on the
allocator; only `marker_present` is read live, one `stat`):

```json
{"state":"ready","reserved_through":0,"allocated_through":0,"trusted":false,
 "untrust_reason":"no_lease","seeded_from":"exact","seed_ms":0,
 "cross_check_max":0,"refill_failed":0,"last_lease_error":"",
 "marker_present":false,"seed_failed":0,"lease_tombstoned":0,
 "seed_undecodable_rows":0,"on_tape":false}
```

`state` is `unseeded`, `seeding` or `ready`; `untrust_reason` is
`no_lease`, `receipt`, `marker`, `inconsistent`, `ceiling`, `violated`, or
empty when trusted; `seeded_from` is `exact` or `lease`. The last two fields
are additions (bunch 169); `on_tape` is an addition (bunch 178): the last lease
row this process wrote went through a bound tape.

### Not done, and residual

- The deployed proof (section 6) and the paper's measured `seed_ms`.
- Lane `fp1-event-table-d2`: its `store.event_table` setting must refuse to
  turn on until `state` is `ready` with a lease on the tape (not on this lane).
  Built on `lane/fp1-event-table-d2-r3` (2026-10-23): `Store.EventTableLeaseReady`
  reads this lease's snapshot and one atomic (`lease.onTape`, set where the
  seed and the refill write the row: the tape was bound); the writer's stamp
  and the setting's POST (409) both refuse before it; deploy 2's proof step 5
  is this section's step 3. Round 2 of bunch 178 (both seats approved
  `f66b6f217`): `Reopen` re-reads the adopted workspace before the lease
  resumes, clears `onTape` and writes a fresh lease row onto the tape (the
  gate reopens when it lands); `StreamSeqLeaseStats.on_tape` reports the atomic
  and deploy 2's proof step 2 asserts it. Recorded as information, not fixed
  (bunch 178, Devin 2, 4 and 5):
  - a `store.event_table=true` retry POST during a not-ready window gets 409
    instead of re-applying the same value; the setting stays stored and the
    switch armed, and it converges once the lease lands (cosmetic);
  - proof step 5's engine-mutex regex keys on `seed_owner_max`, which the exact
    seed always runs; it goes blind if that op label is renamed, and
    `seeded_from == "lease"` is the primary signal;
  - `RestagePinnedRelease` passes `allowUndeclared=true` so genesis baselines
    restage on a floored store; safe because an undeclared manifest means the
    booted floor-aware image, and no pre-contract pinned release should exist.
- The VM snapshot resume residual (open question 4) is unchanged.
- The full `internal/agentcore` suite was not run; the focused run of the
  instruction (`Event|Seq|Lease|Rematerial|Restore|Replay`) passes.
- A desk-forged `choir.event` row (a high `stream_seq` written by a desk
  through the ordinary event path, which no fence guards) is inherited from
  the pre-lease design and recorded here: the cross-check and the exact seed
  read it like any event, so it can raise the next seed (a burn of the number
  space, bounded by the ceiling for the lease and by nothing for the exact
  seed's maximum), never lower it. Round 2 of bunch 175 (Devin) left it open
  on purpose.

### Round 2 (bunch 175: both seats accepted `642efb512`, no blocking finding)

Fixed: a `Close` wakes every parked waiter at once (a closing channel closed
without `eventMu`); the id-range walk has its own engine-mutex label
(`ListKindIDRangePage`) and tests assert which path ran; the outside-range
probe has a plan pin; the marker read is one call; `s.texturePath` is
snapshotted under `eventMu` (Open, Reopen) instead of read in the seeder;
`marker_present` is published inside the snapshot; undecodable event rows
are counted and logged (`seed_undecodable_rows`, a fourteenth field); the
stale comments and loose `> lastStored` assertions in the seed tests are
replaced by the lease's exact `R + 1`; the base-install test asserts
`no_lease`.

### Round 3 (PR #91 CI: `TestSeederRefusesWhenTheHeadsDiffer`)

The failure was test-only: the test shrank the seeding deadline to 200 ms,
and after the heads agreed the seeder's next attempt (the doubling backoff
had reached 160 to 320 ms, an attempt itself took 141 ms on the loaded
runner) fell outside a second 200 ms deadline. Two of its neighbours
(`TestSeederAppendFailureHandsOutNoNumber`, the ready phase of
`TestBatchingReservationDoesNotDeadlockTheLeaseWait`) had the same shape.
The tests now nudge the seeder (`StartStreamSeqSeed`) until the lease is
ready, with a one-minute bound, instead of racing a shortened deadline.

`TestDeadlineIsFromFirstRecordNotSliding` (a 400 ms window, run in the same
package and shard) measured from before the process's first allocation,
which since the lease includes the exact seed and a tape append; it now seeds
the lease before the clock starts. Its 700 ms bound is otherwise unchanged
and is still a wall-clock test, so it can still flake on a starved runner.

Product question recorded, no code change: a seeder in its retry backoff
does not notice that the cause cleared (heads agreeing is not an event the
store is signalled about) until the backoff ends, so the first append after
a transient failure can wait up to one backoff step (capped at 30 s, against
the 10-minute seeding deadline). A wake would need a head-change signal from
the appender; the bound is the same order as the 30 s ready deadline and an
append never fails because of it. Left as is; revisit if a deployed proof
shows `seed_failed` above 0 on a healthy boot.
