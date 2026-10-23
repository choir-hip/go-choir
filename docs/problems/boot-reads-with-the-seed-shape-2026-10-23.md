# Which reads with the stream_seq seed's shape will hold the engine next (2026-10-23)

Date: 2026-10-23, 08:30 UTC. Found by: FP0 worker, lane `fp0-boot-read-census`
(investigation only; nothing is fixed here). Status: open, problem record
before any fix. Mutation class of this record: green (docs) plus one yellow
test (`TestBootReadCensus`). Mutation class of the fixes named below: red
(the event sequence allocator, a boot-path read) and orange (a read path).

Parent record:
[stream-seq-seed-scans-the-event-kind-at-boot-2026-10-23.md](stream-seq-seed-scans-the-event-kind-at-boot-2026-10-23.md)
(on `origin/lane/fp0-stream-seq-seed`; its fix is not on `main` or on the
paper). Claims are marked **[V]** verified (a plan, a measurement, a log line
or the code, named beside it) or **[H]** hypothesis.

## The question

The paper stalled at every planned refresh for 1m45s to 1m49s. One cause is
found, the stream_seq seed (`ListObjects choir.event`, limit 500). Will the
next three reads with the same shape (no fitting index, walks a whole kind or
a whole owner, runs under the store's engine mutex at boot or on first use)
hold the engine for long, and how long?

## Answer

No one of the three is a second 100-second boot stall at the sizes we can
measure. Ranked by measured hold:

| rank | read | when | engine hold, warm, paper-like size | scales with | verdict |
|---|---|---|---|---|---|
| 1 | `ListEventsByOwnerAfter` | first live-WS catch-up after every browser reconnect (not boot) | 125 ms (31k events), 318 ms (71k); whole call 0.39, 0.92, 1.66 s (31k, 71k, 131k) and up to 1.1 GiB allocated per call | the owner's whole event kind (capped at 100,000 rows returned, not at rows read) | the next stall: the seed's shape and read, with a larger limit, and it recurs |
| 2 | boot run-header list | once per boot, before any run is passivated | 132 ms (50k headers); whole `ListRunsByOwnerStates` 256 ms | the owner's run headers, linear (2.6 us per header held) | small; the paper's own log bounds it at 2 to 3 s cold, four times |
| 3 | per-run seq seed | first append of each run in a process (so every passivated run at boot) | 5 ms at 1,000 events, 42 to 45 ms at the 10,000 cap, per run | the run's events, capped at 10,000 rows; not the total event count | bounded; 130 ms per run whole block at the cap; paper passivations of runs 2 to N took 0.13 to 0.5 s each, total |

Cold cost on the paper cannot be reproduced locally, so none of these is a
prediction of seconds on the paper; the plans and the scaling are the
claims. What the paper does tell us (console, below): rank 2 and rank 3 are
already inside every refresh and cost seconds, not minutes. Rank 1 never
shows in a boot log, because nothing logs it, and by plan it does the same
kind of read as the seed, owner-wide, with bodies.

Two findings the question did not ask for:

- `ListEventsByOwner` (limit 200), called from `apihandler/costs.go:74` and
  `textureowner/texture.go:1969`, has the seed's exact plan and costs the
  same as the seed statement on every call (78, 186, 575 ms at 31k, 71k, 131k
  events). It is API-called, not boot. **[V]** (measured, below).
- Over the five console logs on node-b (03:24 to 08:17 UTC), the most frequent
  stall holder is not any of the three: `internal/store.(*Store).ListRunMemoryEntriesForAgent`
  is named in **49 of 99** `store_stalled holder` lines. The seed's `ListObjects`
  is 23. See "Not asked, seen".

## 1. The per-run seq seed

**The query and its call sites. [V]** `allocateEventSeqLocked`
(`internal/store/store.go:2295`), on a run's first append in a process:

```go
existing, err := s.ogListByMetadata(ctx, ogKindEvent, "run_id", rec.RunID, 10000)
// then ogDecode of every body, for the maximum ev.Seq
```

which is `DoltStore.ListObjectsByMetadata` (`dolt_store.go:1295`) over
`metadataListSQL`:

```sql
SELECT <all 12 columns incl. body, metadata> FROM og_objects
WHERE object_kind = ? AND og_meta_run_id = ? ORDER BY updated_at DESC LIMIT ?
```

Callers of the allocator: `AppendEvent` (every event) and, since D12
(`74260d09d`, `f96245f9b`), `AllocateEventSeq`, which
`persistLifecycleRunWithEvent` (`store.go:1489`) calls for the run-state event
folded into every lifecycle batch. So D12 added the seed to the first
lifecycle transition of any run first seen in this process, including each
run boot passivation touches. Boot passivation already appended an event per
run before D12 (`passivateInterruptedActivations` calls `rt.emitEvent`, which
is `AppendEvent`, `runtime.go:2689`, `:4757`), so D12 did not add a first call
at boot, only more routes to it.

**Locks. [V]** `eventMu` (the store-wide event append mutex: every append of
every run waits behind it) for the whole block, and the engine mutex for the
query and its row scan (named `(*DoltStore).ListObjectsByMetadata choir.event
$.run_id index`; the op name says `index` when the generated column is live and
`json` when not). The body decode runs under `eventMu` only. The stream seed is
in the same function, above and below, under the same `eventMu`.

**Plan. [V]** (`EXPLAIN PLAN`, local Dolt store, the test below). With
`og_meta_run_id` live:

```text
Limit(10000)
 └─ Project [... body, metadata ...]
     └─ TopN(Limit: [10000]; og_objects.updated_at DESC)
         └─ IndexedTableAccess(og_objects)
             ├─ index: [og_objects.object_kind,og_objects.og_meta_run_id,og_objects.updated_at]
             └─ filters: [{[choir.event, choir.event], [run-k1000, run-k1000], [NULL, ∞)}]
```

It seeks the run's range of the index; it reads every event of the run (a
TopN, not a limit pushed into the index), with `body`. Cost is the run's event
count up to 10,000, not the kind's. With the column not live the plan is
`Filter(json_extract(...)) over IndexedTableAccess [object_kind,owner_id]`
under the same TopN: the full event kind, bodies included.

The column is live at every one of the four refresh boots. **[V]** The console
logs `derived access paths complete (live: [... og_meta_run_id ...])` at 06:25:06,
06:53:18, 07:28:46 and 08:05:32, each about 5 seconds before
`boot phase begin passivate_interrupted_activations`.

**Cost as it scales. [V]** `TestBootReadCensus`, local Dolt, 1.5 KB event
payloads, best of three warm; three store sizes (31,100 / 71,100 / 131,100
events, of which three runs of 100, 1,000 and 10,000 events):

| run size | whole block (query + decode) | engine hold (query only) | allocated |
|---|---|---|---|
| 100 events | 2.5 ms | n/a | 4 MiB |
| 1,000 | 14.6 ms | 5.1 to 5.5 ms | 17 to 21 MiB |
| 10,000 (cap) | 125 / 129 / 130 ms | 42 to 45 ms | 117 to 137 MiB |
| run never seen | 1.1 ms | | 1.8 MiB |

The block does not grow with the store: 125, 129 and 130 ms for the
10,000-event run at 31k, 71k and 131k total events. The "first call" of a
run was 166 to 175 ms in one pass and 1.15 to 1.37 s in another at the larger
sizes (Dolt compacting after the bulk seed, a partly cold chunk cache): so
the cold cost of the cap is of the order of a second per run **[H]**. With
the column not live (an unmigrated store) the same block is the whole-kind
scan: for the 1,000-event run 170, 389 and 968 ms warm at 31k, 71k, 131k
events (first calls 0.7, 4.3 and 5.9 s), and 265, 481, 1,045 ms for the
10,000-event run; it grows with the kind. **[V]**

**Console. [V] (bound), [H] (the rest).** After the first passivation (which
carries the stream seed), runs 2 to N passivate in 0.13 to 0.51 s each,
including `UpdateRun`, the event append with this seed, and the commit:

| boot (version) | first passivation | the next | boot phase |
|---|---|---|---|
| 06:25 (D9, `66302314`) | 1m44.378s | 16 runs, 0.17 to 0.40 s, 06:27:01 to :05 | 1m53.6s |
| 06:53 (`a49ae398`) | 1m48.801s | 18 runs, 0.16 to 0.51 s, 06:55:16 to :20 | 1m57.8s |
| 07:28 (`6d71f5de`) | 1m49.513s | 16 runs, 0.13 to 0.23 s, 07:30:44 to :49 | 1m56.8s |
| 08:05 (D12, `7ca537e0`) | 1m45.792s | 9 runs, 0.13 to 0.21 s, 08:07:27 to :28 | 1m50.4s |

So the per-run seed at the paper's runs, cold after a reboot, is below half a
second per run and a few seconds in total per boot, and D12's extra routes to
it left no visible mark (0.13 to 0.21 s at 08:05 against 0.17 to 0.51 s
before). Caveat **[H]**: the 104-second seed read walked every event row of the
kind, which may have left the event pages of these runs in the chunk cache.
After the seed fix lands that warmth is gone and these per-run costs could
read higher at the next refresh. Read the next refresh's runs-2-to-N
durations before calling rank 3 closed.

**Fix shape (not built). [H]** (a) Read `metadata` only: `seq` is in event
metadata (`buildEventObject`), so the seed needs no body and no decode; that
is about a tenth of the bytes (the 10,000-event block allocates 117 MiB for a
number). (b) F-c deploy 2 (the writer stamping events into `choir_events`)
turns the seed into one reverse seek on `idx_choir_events_run_seq`
(`eventTableRunMaxSeqSQL` already does it for the table half) and leaves the
og half to acknowledged kinds only; this is the structural fix and it is
already designed. (c) Make the seed refuse the `JSON_EXTRACT` path (error or
defer) so an unmigrated store never scans the event kind under `eventMu`.

## 2. ListEventsByOwnerAfter

**The query and its call sites. [V]** `Store.ListEventsByOwnerAfter`
(`store.go:2584`):

```go
objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{
    Kind: ogKindEvent, OwnerID: ownerID, Limit: 100000})
// decode every body; keep stream_seq > afterSeq in Go; union with choir_events
```

Sole caller: `HandleLiveWS` (`agentcore/live_ws.go:79`), when the request
carries `after_seq` > 0, with `limit 500`. The browser sends `after_seq`
whenever it reconnects with a `lastLiveStreamSeq` it already holds
(`frontend/src/lib/Desktop.svelte:987`), which is what a tab open across a
planned refresh does. So it runs on the first browser reconnect after a boot,
and on every reconnect after, not at boot proper. **[V]** (code).

**Locks. [V]** The engine mutex for the whole `ListObjects` (op `(*DoltStore).ListObjects
choir.event`: the same op name as the seed's, so the engine-mutex counters
cannot tell the two apart; only the stall-holder stack's caller frames or a
request log could). `beforeEventRead` takes the telemetry sealer's read
barrier first. It does not take `eventMu`. The decode and filter run after the
engine mutex is released; they are not a hold, but they are two thirds of the
call's time and a third of its allocation.

**Plan. [V]**

```text
Limit(100000)
 └─ Project [... body, metadata ...]
     └─ TopN(Limit: [100000]; og_objects.updated_at DESC)
         └─ IndexedTableAccess(og_objects)
             ├─ index: [og_objects.object_kind,og_objects.owner_id]
             └─ filters: [{[choir.event, choir.event], [owner-census, owner-census]}]
```

The seed's plan with the owner as a second key and a limit 200 times larger.
There is no `stream_seq` index on `og_objects`, and the statement orders by
`updated_at`, so the engine reads every event of the owner with its body, keeps
100,000 and the caller filters on `stream_seq` in Go. The `choir_events` half is
indexed (`idx_choir_events_owner_stream`, `eventTableOwnerAfterSQL`) and is
not the problem; the og half is, and the og half is all events on the paper:
nothing stamps events into `choir_events` until deploy 2's writer (header of
`event_table.go`).

**Cost as it scales. [V]** (`TestBootReadCensus`, warm, best of three; one
catch-up asking for the 50 events behind the head):

| events (owner) | ListObjects alone (engine hold) | whole call | allocated per call | seed statement for scale | `ListEventsByOwner` limit 200 |
|---|---|---|---|---|---|
| 31,100 | 125 ms | 390 ms | 203 MiB | 79 ms | 78 ms |
| 71,100 | 318 ms | 924 ms | 467 MiB | 193 ms | 186 ms |
| 131,100 | not split out | 1,663 ms | 1,104 MiB | 583 ms | 575 ms |

It is linear in the owner's event count. The 100,000 cap bounds the rows
returned and decoded (the heap of one call), not the rows the engine reads. Per call it holds the engine for 1.6 to
1.7 times the seed statement's time at the same size (125 against 79 ms at 31k,
318 against 193 ms at 71k), the same read plus returning every row rather than 500.

**Console. [H]** There is no log line for it: the catch-up logs only on
failure. A stall sample at 06:57:09, 1m48s after the 06:55:21 boot phase
ended (4.4 minutes after the 06:53 boot), names `ListObjects` as a holder,
runnable in `archiveReader.readByteSpan` (a cold read), on goroutine 227803,
long after the boot seed's goroutine (164173 at 06:53:35 and 06:54:35). A
browser reconnect would produce that. So would the actor-wake outbox list
(`ListObjects choir.actor_wake_outbox`, which logged `dur=2.382s` at 06:57:20
and `dur=21.204s` at 06:35:28, the fp4 outbox-scan lane's finding) and nothing in
the console separates them, because the stall holder line names the function,
not the kind. Therefore: the console neither confirms nor rules out a
`ListEventsByOwnerAfter` hold. The cold cost on the paper is a guess: by plan
it reads the same pages the seed reads, so at least the seed's order if the
chunk cache has been dropped, and seconds if the seed (or a previous
catch-up) just warmed it. **[H]**

**Fix shape (not built). [H]** Read the window the seed fix reads: the newest
K object headers by `updated_at` through `idx_og_objects_updated` (a reverse
walk, no sort, no body: `ListNewestObjectHeaders` on the seed lane), keep the
events of this owner with `stream_seq > after` from the metadata, then `GetObject`
the at most 500 hits. A client whose cursor is older than the window gets an
explicit "resync" frame and refetches, instead of an unbounded replay (the
current read returns at most 100,000 rows anyway, so it is already lossy past
that). After deploy 2 the og half is a short list and the table half is
already indexed. Coalesce simultaneous reconnects (singleflight per owner and
cursor) so a refresh with several tabs reads once. `ListEventsByOwner` (limit
200) needs the same window read.

## 3. The boot run-header list

**The query and its call sites. [V]** `forEachPreBootRun`
(`agentcore/runtime.go:2571`) for both `closePreBootRunsAfterCrash` and
`passivateInterruptedActivations`, once per boot, before any run is
passivated, on the production computer path (`ownerID` set):

```go
runs, err := rt.store.ListRunsByOwnerStates(ctx, ownerID, computerID, states, store.ListAllRunHeaders)
```

which is `ListAllObjectRefsByKindOwner(choir.run, owner)`:

```sql
SELECT canonical_id, computer_id, metadata, updated_at, tombstone FROM og_objects
WHERE object_kind = ? AND owner_id = ? ORDER BY updated_at DESC     -- no LIMIT, no body
```

then, in Go, a JSON decode of every header's metadata to test `state`, then
`GetObject` (a separate engine-mutex acquisition each) for the matches. The
other callers pass a limit (`RunningCountByProfile`, 1024; the obligations
surface, a scan cap) and are not boot reads. **[V]** (code).

**Locks. [V]** The engine mutex, anonymous op `(*DoltStore).ListAllObjectRefsByKindOwner`
(no kind in the name, as the seed's was before the kind-naming change), for
the whole query and row scan. The metadata decode and the `GetObject` calls are
outside the one hold.

**Plan. [V]**

```text
Project [canonical_id, computer_id, metadata, updated_at, tombstone]
 └─ Sort(og_objects.updated_at DESC)
     └─ IndexedTableAccess(og_objects)
         ├─ index: [og_objects.object_kind,og_objects.owner_id]
         └─ filters: [{[choir.run, choir.run], [owner-census, owner-census]}]
```

It seeks the owner's run range of `idx_og_objects_kind_owner` (the run kind,
not the whole table) and sorts all of it in memory by `updated_at`. No body.
It misses an index on `state`, so the 21 live runs are found by decoding every
header.

**Cost as it scales. [V]** (`TestBootReadCensus`, warm; 21 of the headers are
pending or running, 2 KB bodies):

| run headers | `ListAllObjectRefsByKindOwner` (engine hold) | `ListRunsByOwnerStates` whole | allocated whole |
|---|---|---|---|
| 5,000 | 11.8 ms | 31 ms | 26 MiB |
| 20,000 | 52 ms | 108 ms | 75 MiB |
| 50,000 | 132 ms | 256 ms | 188 MiB |

Linear: 2.6 microseconds held and about 5 microseconds in total per header,
independent of the rest of the store (the three sizes carried 31k to 131k
events and 6.7k to 40k outbox rows).

**Console. [V]** `boot phase begin passivate_interrupted_activations` to the
`boot passivation owner-scoped ... candidates=N` line, which is logged right
after this list (and its `GetObject` calls) returns:

| boot | begin | candidates line | elapsed | candidates |
|---|---|---|---|---|
| 06:25 | 06:25:11 | 06:25:14 | 3 s | 21 |
| 06:53 | 06:53:23 | 06:53:25 | 2 s | 21 |
| 07:28 | 07:28:52 | 07:28:55 | 3 s | 21 |
| 08:05 | 08:05:38 | 08:05:41 | 3 s | 10 |

So the cold list on the paper costs at most 3 seconds including the matching
`GetObject`s, at whatever number of headers the paper holds (not known from
the log; the object count is behind the observability API, which this
investigation did not call). Locally 3 s at 5 microseconds a header is about
600,000 headers warm. The paper is far under that **[H]**: it is a small
fraction of what the seed took, and the list finishes before the seed starts.

**Fix shape (not built). [H]** Index `state` for the run kind: add `state` to
`metadataIndexColumns` with `Kind: choir.run` and the
`(object_kind, og_meta_state, updated_at)` index, built online by the existing
access-path migration (never a boot-path step), and read
`state IN (pending, running[, passivated, blocked])`. That turns a linear walk
into a 21-row seek. Not urgent: it is the cheapest of the three.

## Not asked, seen

- **`ListRunMemoryEntriesForAgent`** is the top stall holder: 49 of 99 holder lines
  across the five logs, 03:24 to 08:17 UTC, in steady state (cursor reads,
  `LoadInboxCursorForDesk`, `rlm_reduce.go:520`, `:665`, `:732`, `:891`), not
  at boot, and in the `run_memory_entries` table (outside the object graph, on
  the same engine). Its query reads every run-memory row of the desk agent
  across all its runs, with `message_json`, ordered by `created_at, seq`, and
  applies the `kind IN (...)` filter beside the index
  (`idx_run_memory_entries_owner_agent_created (owner_id, agent_id, created_at)`
  serves the equality and the order, not `kind`). **[V]** (stall samples and the
  statement) **[H]** (that this is the cost: unplanned, unmeasured). It is a
  fourth read of the family and has the most evidence. It should get its own
  record before anyone tunes the three above.
- The seed's `ListObjects` holder is seen at 23 stall samples; 22 are within
  417 s of a guest boot by dmesg offset, one (02:55:16) at 2,714 s.
- `ListObjectRefsByKindOwner` appears once (06:35:29), the limit-8192 scan of
  `latestLifecycleRunByAgent`; `derivedStrayIDs` twice (06:59:37, 07:02:44), the
  derived-key validation of the 69,000-row outbox at 138 rows per second
  (about 8 minutes per pass); `queryTableContentHash` three times.

## How this was measured

- Worktree `lane/fp0-boot-read-census` off `origin/main` (`7ca537e01`, D12).
  Local embedded Dolt only; no SQL against production Dolt; no API call to
  any computer; no `/api/runtime/observability` read. The paper's console
  was read with `ssh node-b 'sudo grep -a ...'` on
  `vm-1669020adcece51c2f5ff9b6df399ef5/console.log{,.1,.2,.3,.4}`.
- `TestBootReadCensus` (`internal/store/boot_read_census_test.go`, env-gated,
  repeatable):

  ```text
  CHOIR_BOOT_READ_CENSUS=20000:5000,60000:20000,120000:50000 \
    go test ./internal/store -run TestBootReadCensus -v -count=1 -timeout 60m
  ```

  Each size is EVENTS:RUNS; the events are 1.5 KB payloads for one owner
  over EVENTS/200 runs plus three runs of 100, 1,000 and 10,000 events, EVENTS/3
  actor-wake-outbox rows and RUNS run headers (21 pending or running). The
  totals are the events plus 11,100: 31,100, 71,100 and 131,100. The first
  pass ran all three sizes (704 s), the second the first two with the
  engine-hold lines added (268 s); the tables use the second pass where it
  has the line and the first for the 131,100 row. The runs repeat within
  about 5% except the "first call" of a freshly bulk-written store, which
  varied 8 times between passes and is reported as a range.
- Warm: all chunks in the process cache. Allocation is `TotalAlloc` over the
  call, not peak heap. A local run has no archive reads, so the paper's cold
  cost (the seed read took 104 s) is not reproduced; the plan and the scaling
  are the claims. The paper's event, run-header and outbox counts are
  unknown to this record except the outbox (about 69,000 rows, from the
  derived-key validation lines).
- The local store has all the generated-column access paths live; the
  paper's four boots log the same set.

## What this does not settle

1. The cold cost of ranks 1 and 3 on the paper. Rank 3 can be read off the
   first refresh after the seed fix (runs 2 to N durations).
2. Which of the 06:57:09 `ListObjects` holders was which. A request-scoped
   log on the live-WS catch-up (rows read, elapsed) would settle rank 1 in
   one refresh.
3. `ListRunMemoryEntriesForAgent`'s plan and cost at the paper's size.

## Heresy delta

- Discovered: `ListEventsByOwnerAfter` and `ListEventsByOwner` have the
  seed's plan (whole owner's event kind sorted under the engine mutex),
  `ListEventsByOwner` at limit 200 and every call; the per-run seed reads
  bodies of up to 10,000 events for a number, and has a whole-kind scan
  behind it when the column is not live; the engine-mutex op name for
  `ListAllObjectRefsByKindOwner` and `ListObjectRefsByKindOwner` carries no
  kind, and `ListObjects choir.event` cannot tell the seed from a catch-up;
  the top stall holder in the console is a fourth, unmeasured read.
- Introduced: none (a yellow test, a green record).
- Repaired: none.

## Rollback

Revert the commit that adds `internal/store/boot_read_census_test.go` and this
record. No runtime, schema, data or index change.
