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
