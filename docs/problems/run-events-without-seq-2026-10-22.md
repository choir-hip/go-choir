# Run-state events folded into the lifecycle batch are stored with seq 0 and stream_seq 0

**Status:** confirmed by a store test on `origin/main` 85a8115b1, 2026-10-22. Found by the
reader survey for the trace segment log design
(`docs/design/trace-segment-log-2026-10-22.md`, heresy delta, branch
`lane/fp1-trace-segment-design`). No fix in this record's commit; the failing test is in
`internal/store/run_events_seq_test.go`.

Evidence labels: **[V]** code read at file:line, **[M]** measured by the test,
**[H]** hypothesis.

Mutation class of the fix: **red** (run event read path and sequence allocation, which are
Trace/evidence surfaces). Heresy delta: discovered 1 (this one), introduced 0, repaired 0.

## Finding

Every run-state event of a lifecycle-bound run (`run.started`, `run.completed`,
`run.failed`/cancelled, `run.passivated`) is stored with `seq = 0` and `stream_seq = 0`.
The hypothesis is confirmed.

- `AppendEvent` is the only place that allocates `seq` (per run) and `stream_seq` (store-wide)
  **[V]** `internal/store/store.go:2181-2216` (`rec.Seq = maxSeq + 1`) and `:2243-2244`
  (`s.maxStreamSeq++; rec.StreamSeq = ...`). Both run under `eventMu`.
- `Store.UpdateRunWithEvent` hands the event to `persistLifecycleRunWithEvent` first
  **[V]** `store.go:1396-1399`. For a lifecycle-bound run that function returns
  `handled = true`, so the `AppendEvent` calls at `store.go:1407` and `:1431` never run
  **[V]** `store.go:1321-1360`.
- The event is put in the request (`Event: event`, `store.go:1338-1341`) and written by
  `projectLifecycleRun` through `buildEventObject` into the same `PutBatchConditional` as the
  run projection **[V]** `internal/store/lifecycle.go:3328-3336`, `:3349`;
  `internal/store/graph_store.go:1205-1239`. `buildEventObject` copies `rec.Seq` and
  `rec.StreamSeq` into the object metadata and body as they are. Nothing assigns them first
  **[V]**.
- The callers pass events without numbers **[V]**: `persistActivationStateAndEmit`
  builds `types.EventRecord{EventID, RunID, ..., Kind, Payload}` with no `Seq`
  (`internal/agentcore/runtime.go:1968-1979`), and calls `persistActivationStateWithEvent`
  (`:1909`), which reaches `UpdateRunWithEvent` through
  `updateRunAndMarkSuccessfulCoagentActivationDeliveredWithEvent`
  (`internal/agentcore/management_controller.go:896-913`). Its callers are the run start
  (`runtime.go:3504`), completion (`:3842`), the two passivations (`:3912`, `:3978`) and the
  failure/cancel end (`:4585`, and the retry path `run_end_retry.go:52`).
- Introduced by `acb85f449` "K: atomic projection+event fold for activation state writes"
  (2026-09-24) **[V]** `git log -S`.
- The event is still published on the live bus with the same zero numbers
  (`runtime.go:1984-1988`), so a live subscriber gets `seq: 0, stream_seq: 0` too **[V]**.
- Only runs with a lifecycle trajectory take the fold. Runs with no lifecycle trajectory
  fall through to `AppendEvent` and are numbered **[V]** `store.go:1321-1331`. Most desk
  runs are lifecycle-bound **[H]**, so the share of affected runs on a live computer is
  large, not an edge case.
- The existing test reads through `ListEventsOG`, which does not filter on `seq`, so it
  passes while the numbers are zero **[V]** `internal/store/lifecycle_test.go:315-326`.

## Measurement

`go test ./internal/store -run TestKnownFailingRunEventFolded -v` (GOMAXPROCS=2
GOFLAGS=-p=1). The test appends an ordinary event (seq 1), folds `run.completed` through
`UpdateRunWithEvent`, then appends another ordinary event (seq 2). Output:

```
[M] AppendEvent assigned: before seq=1 stream_seq=1; after seq=2 stream_seq=2
[M] stored folded event: seq=0 stream_seq=0
[M] ListEventsAfter(run, 0)            = [ev-before ev-after]
[M] ListEventsByOwnerAfter(owner, 0)   = [ev-before ev-after]
[M] ListEventsByOwnerAfter(owner, 1)   = [ev-after]
[M] ListEventsByTrajectory             = [ev-folded ev-before ev-after]
[M] evidence URL after=0&limit=1 returns [ev-before] (cited ev-folded)
```

The event is stored (the unfiltered `ListEventsOG` control finds it). Four subtests fail,
one per reader family below.

## Readers affected

What each reader does with a row whose numbers are zero:

| Reader | Behavior | Evidence |
|---|---|---|
| `GET /api/runs/{id}/events?after=N` (`handleRunEvents`, `internal/agentcore/api_run_events.go:73`), through `ListEventsAfter` (`store.go:2294`) | **Vanishes** for every cursor, including `after=0`: the filter is `rec.Seq <= afterSeq` (`:2311`), and `0 <= 0` | [V] [M] |
| `choir trace` `run_events` layer (`cmd/choir/trace.go:326-361`), which pages that route from `after=0` | **Vanishes**. The bundle has no `run.started`/`run.completed` lines, and nothing marks the gap | [V] [M] |
| Legacy `GET /api/internal/runs/{id}/events` (`internal/agentcore/api.go:437`) and run memory (`run_memory.go:437`), through `ListEvents` (`ListEventsAfter(0)`) | **Vanishes** | [V] |
| WebSocket catch-up `GET /api/ws?after_seq=` (`live_ws.go:79`), through `ListEventsByOwnerAfter` (`store.go:2357-2384`, filter `StreamSeq <= afterSeq`) | **Vanishes**. A reconnecting client misses every run-state event written while it was away. Catch-up only runs when `after_seq > 0` (`:78`), and the cursor is the highest `stream_seq` seen (`Desktop.svelte:987`, `:1031`) | [V] [M] |
| WebSocket live stream | **Delivered, with `seq: 0, stream_seq: 0`**. The frontend cursor is a `max`, so the zeros never move it | [V] |
| `ListEventsByTrajectory` (`store.go:2421-2447`), used by run acceptance (`run_acceptance.go:92`) and the Texture workflow verifier (`texture_workflow_verifier.go:126`) | **Sorts first**: ordered by `StreamSeq` ascending, so zero precedes everything. It is inside the 3000/2000 newest-window cut but out of order | [V] [M] |
| `ListEventsByTrajectoryAfter` (`store.go:2451-2480`) | **Vanishes** (`StreamSeq <= afterSeq`) | [V] |
| Run acceptance causality (`run_acceptance.go:592`) | **Skipped**: a checkpoint is considered only when `StreamSeq > 0`, so the "causal where trace events provide stream_seq" check (`:567`) passes without the run-state events. Acceptance checkpoints that cite a folded event carry `StreamSeq 0` | [V] |
| Acceptance evidence URL (`runEventEvidenceURL`, `run_acceptance.go:345-356`) | **Cites the wrong event**: `after = max(seq-1, 0) = 0`, `limit=1`, which returns the run's first numbered event | [V] [M] |
| `choir lifecycle events` / `/api/trajectories/{id}/events` | **Not affected**: these are lifecycle reducer events, a different record kind | [H] (not traced to the handler) |

Nothing breaks a cursor: a zero row never becomes `next_after` (the page is built from rows
that passed the filter), so paging does not loop. The failure is silent omission.

## What the owner would see

- A run's trace in the Trace pane or a `choir trace` bundle that has tool calls, cells and
  progress, but no `run.started`, no `run.completed`, no `run.failed`/`run.passivated`. A
  finished run looks as if it never ended.
- A `choir trace` export that cannot show when or why a run ended, even though the run
  record says so.
- After a reconnect, a desktop that shows a stale run state until the next numbered event,
  or a state it only gets from a full refetch.
- Run acceptance records whose terminal checkpoint has `stream_seq` 0, and whose evidence
  link opens the wrong event.
- Debugging that follows the CLAUDE.md rule ("read the run's actual ledger first") reads a
  ledger with the lifecycle edges missing. That imitates a run that never started or never
  ended.

## Not known

- How many events on a live computer already carry `seq = 0`. Counting them needs a read of
  the event kind on a running store, which this pass did not do (no staging). **[H]** every
  run-state event of a lifecycle-bound run since 2026-09-24.
- Whether any consumer relies on the zeros (for example treating `seq == 0` as "not yet
  persisted"). A grep of `Seq == 0` was not done beyond the readers above.

## Likely fix

Allocate the numbers before the fold, under `eventMu`, with the same counters
`AppendEvent` uses.

1. Factor the allocation block of `AppendEvent` (`store.go:2174-2244`: seed, `rec.Seq`,
   `rec.StreamSeq`) into one helper, `allocateEventSeq(ctx, rec)`. `AppendEvent` calls it;
   `persistLifecycleRunWithEvent` calls it on a non-nil `event` before it computes the
   command digest and builds the request. The helper mutates the caller's `*EventRecord`,
   so the live bus publish (`runtime.go:1984`) carries the numbers too.
2. If the batch then fails, the numbers are a gap, never a duplicate. `AppendEvent`
   already accepts that on its tape-bound path (`store.go:2267-2284`).
3. Replay: the digest covers the event (`ComputeReplaceLifecycleActivationDigest`
   marshals the request). Allocating before the digest keeps one request one digest, but a
   retry that re-enters `persistLifecycleRunWithEvent` with the same pointer must not
   allocate twice. Allocate only when `event.Seq == 0`. The deadline-retry path
   (`runtime_persistence.go:111-130`) should be checked for this.
4. The telemetry batching seeds (`telemetrySeqGap`) must see the fold's numbers: the helper
   is the single allocator, so the trace-log design's seeds (section 4.4) stay correct.
5. History: old rows stay at 0. Options for the owner to rank: (a) leave them and record
   the window; (b) a one-time read-side tolerance that places `seq 0` rows by timestamp;
   (c) a backfill that numbers them in timestamp order, which is a protected write over
   history. Prefer (a) with a dated marker, then (b) only if the trace gap matters.

Do not "fix" the readers to include `seq 0` rows: an unnumbered row cannot be ordered or
cursored, and the evidence URL would still be wrong.

The test to land with the fix is the one already here, renamed without `KnownFailing`;
add a second assertion that a retried fold does not take two numbers.

## Rollback

Revert the fix commit. Rows written with numbers stay valid: the change is additive (the
fields already exist and readers already handle non-zero values), so a rollback returns new
run-state events to `seq 0` without corrupting anything written in between. A mixed store
holds numbered and zero rows, as it does today. The trace log design's pre-rollback step
(F-c section 4.5) is not needed.

## Relation to other records

- The trace segment log design lists this as a discovered heresy and says it needs its own
  record before any fix. This is that record. The design does not change the fold.
- `docs/problems/sa2-appendevent-unbounded-scan-2026-10-06.md` is the earlier work on the
  same allocator. The fold bypasses the allocator that record made cheap.
- Acceptance evidence as a URL, not a copy (design heresy delta) is a separate weakness;
  this record only shows that, for a folded event, the URL points at the wrong event today.
