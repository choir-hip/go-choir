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
