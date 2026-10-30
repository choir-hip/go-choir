# The stream_seq seed reads the whole event kind at boot and holds the engine for 104 s (2026-10-23)

Date: 2026-10-23, 06:35 UTC. Found by: FP0, reading the paper's first boot
on D9 (`663023143`), whose `ListObjects` engine-mutex op now names its kind
(lane fp4-outbox-scan). Status: open, problem record before any fix.
Mutation class of a fix: red (the event sequence allocator, on the guest
boot path).

## Evidence

The paper (computer-1355b83b…, vm-1669020a…) took a planned refresh to D9
at 06:25:01. From `/api/runtime/observability` at 06:28 and 06:32:

- `engine_mutex`: `(*DoltStore).ListObjects choir.event`, **calls 1,
  max_hold 104,024 ms**. Every other op's max hold is under 4 s.
- Boot log:
  - 06:25:11 `boot phase begin passivate_interrupted_activations`, 21 candidates.
  - 06:25:24 and 06:26:24 `store_stalled holders: ... unanswered=desk_pending_mutations,selfdev_active_operations,effective_release engine_goroutines=9 shown=1`, and both times the one holder is `choir=internal/objectgraph.(*DoltStore).ListObjects`, with engine frames `nbs.(*fileReaderAt).ReadAtWithStats < nbs.tableReader.get` and then `nbs.(*archiveReader).readByteSpan`. It is reading cold table and archive chunks from disk.
  - 06:26:59 `passivated run 9b298265… (was running) after restart profile=texture dur=1m44.378s`. The first passivation waited the whole hold. The next twenty took 0.2 to 0.8 s each.
  - 06:27:05 `boot phase end passivate_interrupted_activations dur=1m53.608s`.
- So for about 105 s after boot the paper's store answered nothing that needs the engine. `/health` returned 503 (`store_stalled`), and every event append waited.

## The caller

The only `ListObjects` over `choir.event` on the boot path is the
`stream_seq` seed. On main it is in `(*Store).AppendEvent`
(`internal/store/store.go`, "seed stream_seq"). On the run-events seq lane
it is in `allocateEventSeqLocked`. It runs once per process, on the first
event append, under `eventMu`:

```go
newest, err := s.og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindEvent, Limit: 500})
```

The comment above it says that seeding from the newest page
(`updated_at DESC`, indexed) is O(1). `DoltStore.ListObjects` issues:

```sql
SELECT canonical_id, object_kind, ..., body, metadata, ... FROM og_objects
WHERE 1=1 AND object_kind = ? ORDER BY updated_at DESC LIMIT ?
```

`og_objects` has `idx_og_objects_kind_owner (object_kind, owner_id)` and
`idx_og_objects_updated (updated_at)`. There is no `(object_kind,
updated_at)` index.

## Hypothesis (not yet confirmed from a query plan)

The planner takes the kind index, reads every `choir.event` row with its
`body`, and sorts by `updated_at` to keep 500. That is linear in the
paper's whole event history, and cold after a refresh. The comment's O(1)
holds only if the plan walks `idx_og_objects_updated` backwards. The
1m44s, the archive-chunk reads, and one call with no wait all fit the
linear plan.

To confirm it: `EXPLAIN` the statement on a copy of a large store (never
on the paper), and time the seed on a local store with N events at two
sizes.

## Why it matters

- It is the first-boot window of the paper's `/health` 503s, measured at
  about 105 s after this refresh. It is window B of
  `paper-post-refresh-health-windows-2026-10-22.md`, now attributed. It
  grows with the event history, so every refresh of a long-lived
  computer gets slower.
- It runs on the first event append of every process, on every computer,
  and the engine mutex it holds stops all object-graph reads and writes.
- D10 (F-c deploy 1) adds a second seed that reads `choir_events`
  (`eventTableNewestStreamSeq`) beside it. That one is bounded by its
  table's key order.

## Options (for the fix lane; not chosen here)

1. Seed from a bounded read. The newest event by `updated_at` through the
   `updated_at` index alone (no kind predicate in the index's way), or
   through the derived key index's `idx_og_key_index_recent (key_name,
   key_value, owner_id, updated_at, canonical_id)` with a key family for
   events. Read only the metadata or `stream_seq`, never `body`.
2. Persist a high-water mark: a durable `stream_seq` floor written with
   each batch commit, or with the guest checkpoint. Seed from it, plus a
   bounded catch-up read.
3. Add an `(object_kind, updated_at)` index. This is a schema change on a
   very large table: an online build only, never a boot-path step that
   scans `og_objects` (FP0 rule). It is likely the most expensive option.

The fix must keep the invariant the seed exists for: no `stream_seq` is
ever reused after a restart, including across the telemetry sealer's gap
(`telemetrySeqGap`) and the event table.

## Heresy delta

- Discovered: a comment states an O(1) bound that the plan does not give.
  A boot-path read of a whole object kind runs under the engine mutex.
- Introduced: none.
- Repaired: none yet.

## Phase 1 findings: hypothesis confirmed (2026-10-23, lane fp0-stream-seq-seed)

Method: a local store (embedded Dolt, the store test helpers) with N
`choir.event` objects (1.5 KB payload each, `updated_at` one second apart)
and N/3 `choir.actor_wake_outbox` objects interleaved at the same times.
`EXPLAIN PLAN` and a timed run of the exact statement
`DoltStore.ListObjects` builds for the seed
(`internal/objectgraph/dolt_store.go:719`). Warm: all chunks in the
process cache; the paper's 104 s was cold chunk reads, so these are lower
bounds on the work, and the shape is what matters.

### The plan (n = 20,000; identical at 2,000 and 60,000)

```text
Limit(500)
 └─ Project
     ├─ columns: [og_objects.canonical_id, ..., og_objects.body, og_objects.metadata, ...]
     └─ TopN(Limit: [500]; og_objects.updated_at DESC)
         └─ IndexedTableAccess(og_objects)
             ├─ index: [og_objects.object_kind,og_objects.owner_id]
             ├─ filters: [{[choir.event, choir.event], [NULL, ∞)}]
             └─ columns: [canonical_id object_kind owner_id computer_id ... body metadata ...]
```

The planner takes `idx_og_objects_kind_owner` for the kind predicate, feeds
every `choir.event` row (with `body`) into a TopN keyed on `updated_at`, and
keeps 500. It never walks `idx_og_objects_updated`. The comment above the
seed ("O(1) because the newest page is `updated_at DESC`, indexed") is wrong
for this statement.

### Timings (same statement, three consecutive calls, warm)

| events | other rows | call 1 | call 2 | call 3 |
|---|---|---|---|---|
| 2,000 | 667 | 6.8 ms | 7.1 ms | 7.1 ms |
| 20,000 | 6,667 | 532 ms | 115 ms | 92 ms |
| 60,000 | 20,000 | 1,207 ms | 148 ms | 147 ms |

(An earlier run of the 20,000 case read 78, 50, 53 ms.) Rows returned are 500
every time; the time is the rows the engine reads and sorts. It grows with
the event count, not with the 500, in every column: a 30x larger kind costs
about 170x on the first (coldest) call and about 20x warm. The paper's
history is orders of magnitude larger and was cold, which is how 500 returned
rows took 104 s.

### What bounded reads the engine gives (measured on the same store)

- `FORCE INDEX (idx_og_objects_updated)` with the kind predicate is ignored.
  The plan is the same TopN over the kind index. A hint does not fix it.
- A window subquery does walk the time index, with no sort:

  ```sql
  SELECT canonical_id FROM (
      SELECT canonical_id, object_kind FROM og_objects ORDER BY updated_at DESC LIMIT ?
  ) w WHERE object_kind = ?
  ```

  ```text
  Project [w.canonical_id]
   └─ SubqueryAlias w
       └─ Filter (og_objects.object_kind = 'choir.event')
           └─ Limit(5000)
               └─ Project [og_objects.canonical_id, og_objects.object_kind]
                   └─ IndexedTableAccess(og_objects)
                       ├─ index: [og_objects.updated_at]
                       ├─ columns: [canonical_id object_kind updated_at]
                       └─ reverse: true
  ```

  5,000 rows examined in 12 to 18 ms at n = 20,000; the cost is the window,
  not the kind.

### Other boot-path and first-call reads with the same shape

Same shape: `object_kind = ?` (plus owner) with `ORDER BY updated_at DESC`,
served by `idx_og_objects_kind_owner` and a sort, not by an
`(object_kind, updated_at)` index, under the engine mutex. Not fixed here.

Verified by reading the call chain:

1. **The per-run seq seed**, the same function, one block above, also under
   `eventMu`, on each run's first append in a process:
   `ogListByMetadata(ogKindEvent, "run_id", runID, 10000)`
   (`store.go` allocateEventSeqLocked). With the `og_meta_run_id` column and
   `idx_og_objects_og_meta_run_id (object_kind, og_meta_run_id, updated_at)`
   live it is an index read bounded by the run, but it still reads and
   decodes up to 10,000 bodies of one run to find a maximum. When the
   column is not live (a store the online migration has not finished) it is
   the `JSON_EXTRACT` scan of the whole event kind. A bounded
   `ORDER BY seq DESC LIMIT 1` on a metadata column is the likely fix.
2. **`ListEventsByOwnerAfter`** (`store.go`, called by the live WebSocket
   catch-up, `agentcore/live_ws.go:79`): `ListObjects(Kind: event, OwnerID,
   Limit: 100000)`, then filters `stream_seq > after` in Go. Every first
   client catch-up after a boot reads and decodes the owner's whole event
   history with bodies under the engine mutex. It is the same read as the
   seed with a 200x larger limit, and it runs when the first browser
   connects. The most likely next stall.
3. **`ListEventsByOwner`** (`apihandler/costs.go:74`,
   `textureowner/texture.go:1969`): the same TopN over the owner's events,
   limit 200 by default.
4. **Boot run headers**: `forEachPreBootRun` (`agentcore/runtime.go:2579`,
   the crash-close and the planned-restart passivation phases) calls
   `ListRunsByOwnerStates(... ListAllRunHeaders)`, which is
   `ListAllObjectRefsByKindOwner(choir.run, owner)`: every run header of the
   owner, `ORDER BY updated_at DESC`, no `LIMIT`, metadata only (no body). It
   is linear in run history under the engine mutex, once per boot. It was
   the next phase after the seed on the paper (21 candidates).
   `ListObjectRefsByKindOwner(choir.run, ..., limit)`, used by the recent-run
   and latest-run-by-agent reads, is the same TopN with a limit.
5. **Event reads by run, trajectory and channel** through `ogListByMetadata`
   (`store.go:2513`, `2633`, `2671`, limits 100,000, 10,000 and the
   trajectory window): indexed by the generated columns when live, and the
   full-kind `JSON_EXTRACT` scan when not.
6. `ListRunAcceptances` (limit 100,000, `agentcore/api.go:329`) and the
   texture source entity list (limit 100,000, `graph_store.go:3104`): same
   shape, API-called, not boot.

Read and judged small (a kind with few rows), not a risk now:
`recoverSourceSchedulesAtBoot` (`ListSourceSchedules(ctx, "")`, kind
`source_schedule`, limit 1,000), `OpenConversationHeads` at the crash boot
(`ListAllObjectRefsByKindOwner(conversation_head)`), the capture-label and
world-event lists.

Not traced (named so they are looked at, not claimed): the worker-update
readers that extract body JSON server-side
(`ListJSONBodyFieldsByKindOwner(choir.worker_update, owner)`, seven call
sites in `lifecycle.go` and `lifecycle_control_delivery.go`) scan every body
of an owner's worker updates, and the management rewarm at boot
(`management_controller.go:418`) reaches this family. They are the
`JSON_EXTRACT`-over-bodies shape and deserve their own record.

### Consequence for the fix

The invariant the seed keeps is "no `stream_seq` is reused after a restart".
The old seed took it from the newest 500 events by `updated_at`; its
assumption was that the newest event carries the maximum. A bounded read can
keep exactly that assumption: take the maximum over the events that are
among the newest K objects of any kind by `updated_at`, through the
`idx_og_objects_updated` reverse walk, with the kind check applied to K rows
and only `metadata` read. It differs from the old seed in one case, an event
history whose newest event is older than the newest K objects of every kind;
see the fix section.

## The fix (option 1, no schema change; red)

Mutation class: red (the event sequence allocator, on the guest boot path).
Protected surfaces: `allocateEventSeqLocked` / the stream_seq allocator, the
boot path's first event append. Rollback: revert the commit; the seed returns
to the newest-500 `ListObjects`. No stored data, schema or index changes.

What changed:

- `objectgraph.DoltStore.ListNewestObjectHeaders(limit)` reads
  `SELECT object_kind, metadata FROM og_objects ORDER BY updated_at DESC
  LIMIT ?`. The plan is a reverse walk of `idx_og_objects_updated`, no sort,
  no kind index, no body (pinned by
  `TestNewestObjectHeadersPlanWalksTheTimeIndex`). It is named on the engine
  mutex. `HasObjectOfKind` is one seek of the kind index.
- `Store.seedStreamSeq` reads the newest 2,000 objects of any kind, takes the
  maximum `stream_seq` from the metadata of the events among them, and widens
  to 50,000 only if the first window holds no event. If neither does, one
  `HasObjectOfKind` seek tells a store with no events (seed 0) from events out
  of reach. Then the event table's seed and the sealer's gap, unchanged.

### Does a bounded read keep the no-reuse invariant?

Not outright. It keeps the old seed's assumption (the newest events by
`updated_at` carry the maximum `stream_seq`) over a different read, and is
weaker than the old seed in one band (Round 2 F1, restated in Round 3). When
no window holds a usable event it runs the old exact read, which rests on the
same assumption: `seedStreamSeqExact` is `LIMIT 500` by `updated_at`, not an
exact `MAX`. "Exact" names the read, not a proof of the maximum.

- The old seed assumed the newest events by `updated_at` carry the maximum
  `stream_seq`. The new one assumes the same of the newest objects of every
  kind. They differ only when the last event is older than the newest 2,000
  (then 50,000) objects of other kinds. In that case the seed falls back to the
  exact old read (below). Events are the most frequent write of
  the store, run-state events are folded into the same batches as the
  projection writes, and a boot's own run closures emit events, so such a
  stretch is not expected (a hypothesis from the write paths, not a measured
  rate); the tests exercise it (2,600 later
  objects of another kind; the widening).
- The exception, now fail-safe (coordinator change request after the first
  push; Round 2's narrowing to an empty event table was reverted in Round 3): events exist and every one is older than the newest 50,000 objects of
  all kinds. The first version logged and seeded from the event table alone,
  which could reuse a number; reuse is a correctness defect in red code. The
  seed now falls back to the exact read it used before
  (`seedStreamSeqExact`: the kind index, the 500 newest events by
  `updated_at`, with bodies) and takes its maximum. That read is the 104 s
  scan on a large history, so the case is slow, and it logs `stream_seq seed:
  choir.event objects exist but none is among the newest 50000 objects, out of
  reach of the bounded seed; took the exact read of the kind, max stream_seq N,
  took D`. It is rare by the argument above; slow is the accepted price of
  never reusing a number. No cheaper exact form exists without a structure
  (an `(object_kind, updated_at)` index or a persisted high-water mark, option
  2), neither built here. Test: `TestStreamSeqSeedSaysWhenEventsAreOutOfReach`
  failed before the change (seeded 1 above a stored 5) and now seeds 6.
- Unchanged: the telemetry gap (`telemetrySeqGap`, applied once, after the
  maximum of both tables), the event-table half, and the per-run seed.

### Results

Plan and timing, `TestStreamSeqSeedScale` (env-gated, repeatable:
`CHOIR_STREAM_SEQ_SEED_SCALE=2000,20000,60000 go test ./internal/store -run
TestStreamSeqSeedScale -v -count=1`), best of three, warm, same store shape as
Phase 1:

| events | old seed statement | new seed (`seedStreamSeq`, whole) |
|---|---|---|
| 2,000 | 5.7 ms | 6.6 ms |
| 20,000 | 47 ms | 8.5 ms |
| 60,000 | 148 ms | 9.3 ms |

The old statement grows with the kind (TopN over the kind index); the new
read's time is the window's. The slight growth of the new column is the index
depth, not the kind: it reads 2,000 entries at every size. The cold case on the
paper cannot be reproduced locally; the plan, not the warm time, is the claim:
no read of any row of the event kind, no body.

Tests: the new `TestStreamSeqSeed*` (store), `TestNewestObjectHeaders*`,
`TestHasObjectOfKind` (objectgraph); `TestEventTableReadersAreUnionAware` lists
`seedStreamSeq`. The test that failed before the fix is
`TestStreamSeqSeedDoesNotListTheEventKind` (the engine-mutex counters showed
one `ListObjects choir.event` on the first append).

### Heresy delta

- Discovered: a comment stating an O(1) bound the plan does not give (the
  seed); the same shape in the per-run seed, `ListEventsByOwnerAfter`, the
  boot run-header list, and the worker-update body readers (Phase 1 list).
- Introduced: the out-of-reach case runs the old slow exact read (logged with
  its duration); no reuse.
- Repaired: the seed's read of the event kind, its comment (now states the
  assumption and the exception).

### Follow-ups named, not done here

1. `ListEventsByOwnerAfter` (limit 100,000 with bodies, first live catch-up
   after every boot) is the next stall; it needs a bounded `stream_seq` read.
2. The per-run seed reads up to 10,000 bodies for a maximum.
3. Boot `ListAllObjectRefsByKindOwner(choir.run)` is linear in run history.
4. Option 2, the persisted high-water mark, to make the out-of-reach case
   fast too, if its log line is ever seen on a computer.

## Round 2 (Devin's review, accepted)

### F2: the exact read must not be the common case after the event-table stamp (REVERTED in Round 3)

Round 2's table shortcut below was rejected by review bunch 156 and removed in
Round 3; see "Round 3". Kept for the record of what was tried.

About 94% of event kinds go to `choir_events` once deploy 2's writer stamps.
"No og event in the newest 50,000 objects" then stops being rare, and every
boot would have paid the exact scan with a misleading log line. Now, when the
og windows hold no usable event: a whole-store read that held none seeds 0; a
non-empty `choir_events` seeds its maximum (an og event older than every
window is older than the table's newest under the same newest-carries-max
assumption); only an empty table with events still in the kind, or an
unreadable window event (below), takes the exact read. Tests:
`TestStreamSeqSeedUsesTheTableWhenTheWindowsHoldNoEvent` (og events 1 to 3, a
table holding 4 to 9, 400 newer objects of another kind: seeds above 9, the
exact read's engine-mutex op never ran, no "out of reach" line) failed before
the change; `TestStreamSeqSeedSaysWhenEventsAreOutOfReach` (empty table) still
takes the exact read.

### F1: the reuse band, stated plainly

The seed keeps the old seed's assumption, "the newest events carry the
maximum stream_seq", and is weaker than the old seed in one band. It can seed
below a stored number when all of these hold:

- An event holds the maximum `stream_seq` but sits behind the newest 50,000
  objects of all kinds in `updated_at` order, and another event is inside a
  window (so the seed counts as found and reads no further). `updated_at` of
  an event object is the caller's `Timestamp` (`AppendEventOG`, the sealer),
  `run.UpdatedAt` (`lifecycle.go` fold) or `CreatedAt`
  (`runtime_persistence.go`), not the allocation time. Three ways to land an
  event behind: a backdated timestamp; a wall-clock regress (an NTP step, a
  snapshot resume) that sorts every later write behind earlier objects; or
  `updated_at` ties bisected at a window edge.
- The old seed caught the case "an event inside the newest 500 events but
  outside the newest 50,000 objects" (a backdated event plus a large burst of
  non-event objects on a store with under 500 newer events). This one misses
  it silently.

(Round 3 correction: the second suppressor Round 2 added, a non-empty
`choir_events`, is gone. Only a found event suppresses the exact read now.)

It is inherited in kind and narrower in expectation than the 104 s boot stall
it removes, but it is not zero, and the earlier sentence "the bound never
trades away the invariant" is true only under the shared assumption. The exact
close is option 2, a persisted high-water mark; the log lines cannot see this
band, because a found event suppresses the fallback.

The cheap part is fixed: a window event whose metadata has no parseable
`stream_seq` used to count as found and contribute 0. It now counts as
unusable and sends the seed to the exact read, which decodes the body (Round 3 keeps this). Test:
`TestStreamSeqSeedDoesNotTrustAnEventWithoutMetadataSeq` (an event whose
metadata lacks `stream_seq` and whose body says 7 seeds 8; it seeded 1 before).

### F3: no body, pinned

`TestNewestObjectHeadersPlanWalksTheTimeIndex` now also requires the plan's
projections to be `[og_objects.object_kind, og_objects.metadata]` and the
table access columns `[object_kind metadata updated_at]`, and no `body`.

### Forward note

Once the stamp ships the og half of the seed reads mostly transition and
incident events; the windows still bound the work. The cold hold of a
50,000-row widening is unmeasured (warm, a 2,000-row window is 7 to 9 ms);
the exact read is the only unbounded path (Round 3: it runs whenever the
windows hold no event, whatever the table holds).

## Round 3 (review bunch 156; FP0's conservative decision)

DeepSeek REJECTed Round 2's table shortcut with an executed proof: table
stream max 9, an og event with stream 12 that is newer, 400 newer non-event
objects, windows 20 and 200, reopen, append. Round 1 allocated 13; Round 2
allocated 10 with 12 stored and logged nothing. A window miss says nothing
about the table's newest row: stamped table events and unstamped og events
coexist by design (a stamp rollback, a mixed fleet, a non-event backfill).
Devin accepted with notes.

What changed, test first:

- Round 1's fail-closed rule is back: when no window holds a usable event and
  the whole store was not read, the seed probes the kind (one seek) and, if
  events exist, runs `seedStreamSeqExact`, whatever `choir_events` holds. The
  table's maximum joins through `max()`. The `unusable` branch (an event in a
  window with no parseable `stream_seq`) stays; both seats found it sound.
- `TestStreamSeqSeedTakesTheExactReadWhenTheWindowsHoldNoEventWhateverTheTableHolds`
  is DeepSeek's counterexample and failed on the Round 2 code (seeded 10, want
  13). It replaces Round 2's `TestStreamSeqSeedUsesTheTableWhenTheWindowsHoldNoEvent`,
  which pinned the shortcut and so pinned the defect; the flip is deliberate.
- Devin F2 (nit): `streamSeedWindows[len-1]` panicked on an empty slice. It is
  guarded (an empty list names width 0 and takes the exact read), with
  `TestStreamSeqSeedWithNoWindowsTakesTheExactRead`.
- Invariant wording corrected above (same assumption, over a different read).
- The reuse band is restated above for the restored code: the only suppressor
  is a found event. The band is unchanged otherwise: an event with the maximum
  `stream_seq` behind the newest 50,000 objects (a backdated timestamp, a
  wall-clock regress, ties at a window edge) while another event sits inside a
  window.
- Devin F3 (nit): the cold cost of the 50,000 widening is unmeasured; it is
  recorded, not measured here.

### Gate on F-c deploy 2

This seed is correct for a store whose events are in `og_objects`, which is
every store today (no writer sets the table stamp yet). Once deploy 2 routes
about 94% of event kinds to `choir_events`, the og windows will often hold no
event, and the restored rule makes the exact read run at boot: the 104 s scan
on a large history, at every boot, with a misleading "out of reach" line. So
deploy 2 needs a real close first. Two options; neither is built here.

1. **A persisted high-water mark** (option 2): a durable stream_seq floor
   written with each batch commit or with the guest checkpoint, the seed reads
   it and does a bounded catch-up over the table and the newest og window. Cost:
   a write on the hot path of every batch (or a coupling to the checkpoint
   cadence), a second state authority beside the tape that must itself survive
   the restore, rollback and replay paths (a restored computer must not seed
   from a floor ahead of or behind its tape), and a rollout order (an older
   binary does not write it). It closes the reuse band as well, because the
   floor does not depend on `updated_at`.
2. **An `(object_kind, updated_at)` index** on `og_objects`: the engine can
   then answer "newest events of the kind" by one reverse seek, so the old
   seed's statement becomes bounded and the windows are unnecessary. Cost: a
   schema change on the largest table. It must be built online, never as a
   boot-path scan of `og_objects` (lane rule), and its build time and write
   amplification on the paper's 31 GiB store are unmeasured. It does not close
   the `updated_at`-versus-allocation-order band (a backdated event), only the
   cost.

Recommendation, not a decision: option 1 first, because it removes the
dependency on `updated_at` that the whole family of bands comes from; option 2
if the high-water mark's restore coupling proves worse than the build.
