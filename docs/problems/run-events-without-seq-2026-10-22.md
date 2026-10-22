# Run-state events folded into the lifecycle batch are stored with seq 0 and stream_seq 0

**Status:** fixed on lane `lane/fp1-run-events-seq` (see "Fix landed" at the end), pending
staging proof. Confirmed by a store test on `origin/main` 85a8115b1, 2026-10-22. Found by the
reader survey for the trace segment log design
(`docs/design/trace-segment-log-2026-10-22.md`, heresy delta, branch
`lane/fp1-trace-segment-design`). The regression tests are in
`internal/store/run_events_seq_test.go` and `internal/agentcore/run_state_event_seq_test.go`.

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

## Fix landed (lane `lane/fp1-run-events-seq`)

**What changed** (`internal/store/store.go` only):

- The allocation block of `AppendEvent` (seed, `rec.Seq`, `rec.StreamSeq`, the system-owner
  default) moved, unchanged, into `allocateEventSeqLocked`. `AppendEvent` calls it under
  `eventMu` exactly where the block was.
- `Store.AllocateEventSeq(ctx, rec)` takes `eventMu`, calls the same helper, and returns at
  once when `rec.Seq != 0`.
- `persistLifecycleRunWithEvent` calls `AllocateEventSeq` on a non-nil event after it knows
  the run is lifecycle-bound and before the request, and so the command digest, is built.
  A run that is not lifecycle-bound still goes to `AppendEvent`, as before.

**Retry.** The numbers are on the caller's `*EventRecord`. A second `UpdateRunWithEvent`
with the same event takes none, and the digest is stable across the CAS retries inside
`projectLifecycleRunWithRetry`. A batch that fails after allocation leaves a gap, never a
duplicate. `replaceLifecycleActivationWithDeadlineRetry` (`runtime_persistence.go`) was
checked and needs no change: none of its requests carries an `Event`. The run-end retry
(`run_end_retry.go`) builds a new event with a new `EventID` on each attempt, so a failed
attempt costs one number (a gap) and a committed-but-timed-out attempt could still store two
events **[H]**. That second case was already true (two `EventID`s) and is unchanged; it is a named
residual, not repaired here.

**The live bus** now carries the numbers: `persistActivationStateAndEmit` publishes the same
`*EventRecord` it passed down, and the helper mutates it.

**History is not repaired.** Existing rows keep `seq = 0` and `stream_seq = 0`. The bug
began with `acb85f449` on **2026-09-24**; every run-state event of a lifecycle-bound run
written from then until this fix deploys has zeros. The readers are not changed to include
zero rows. A reader that wants them has the unfiltered `ListEventsOG`.

**Tests (yellow commit `d23ab1d08`, failing before the fix):**

- `TestRunEventFoldedIntoLifecycleBatchGetsSeq` (store): stored numbers, run events API
  and trace export, WebSocket catch-up, trajectory order, acceptance evidence URL.
- `TestFoldedRunEventRetryTakesOneNumber` (store): a second fold of the same event keeps
  its numbers, the next append takes the next number, the event is stored once.
- `TestRunStateEventPublishCarriesStoredSeq` (agentcore): the bus publish carries the numbers
  that are stored.

**Fix ceremony (red).**

- Mutation class: red. Protected surfaces: run event sequence allocation, the run event read
  path, and the read path of acceptance evidence (`runEventEvidenceURL` is now correct for
  new events without a change to it).
- Conjecture delta: before, "a folded event needs no number because it is committed with the
  run"; after, "every run event gets a number from the one allocator, whichever path writes
  it". Refuted by the old behavior; supported by the tests above.
- Admissible evidence: the three tests; on staging, a finished test-computer run's
  `GET /api/runs/{id}/events?after=0` lists `run.started` and `run.completed` with
  `seq > 0`, and its acceptance evidence URL returns the cited event. Not yet run (no staging
  in this pass).
- Rollback: revert the fix commit. Numbered rows stay valid; new run-state events go back
  to `seq 0`.
- Heresy delta: discovered 0 further (the run-end retry duplicate is a residual that already
  existed); introduced 0; repaired 1 (new run-state events). Rows written 2026-09-24 to the
  deploy stay unrepaired.

## Review round 1 (bunch 130: flash, DeepSeek, Devin all accept)

Findings FP0 acts on, before the fix:

- **Doc, the digest (DeepSeek 1, flash 2).** Plan item 3 above says the
  command digest covers the event. It does not:
  `ReplaceLifecycleActivationRequest.Event` is `json:"-"`
  (`internal/types/lifecycle.go:476`) and `lifecycleDigest` is a plain
  `json.Marshal` (`internal/store/lifecycle.go:149-155`) **[V]**. So
  allocation order is no digest hazard.
- **Doc, the run-end residual (DeepSeek 2).** "A committed-but-timed-out
  attempt could still store two events" cannot happen.
  `persistActivationStateWithEvent` re-reads the run and returns
  `(false, nil)` when it is terminal (`internal/agentcore/runtime.go:1917-1920`).
  A non-terminal retry with a fresh event hits the content-hash replay
  (`lifecycle.go:3311-3313`) **[V]**. It costs a gap only. The residual is
  withdrawn.
- **The guard is checked outside the lock (flash 1).** Two goroutines
  holding the same `*EventRecord` could both pass `rec.Seq == 0` and take
  two numbers. No caller shares a record today. Fix: re-check under
  `eventMu`.
- **The skip is Seq-only (DeepSeek 5).** An event with a Seq but no
  StreamSeq would fold with a zero stream cursor. Fix: skip only when both
  are set.

Noted, not fixed:

- The `eventSeqs` eviction window at 100,000 keys, which `AppendEvent`
  already has (DeepSeek 3).
- Replay publishing an event it did not store; this is pre-existing
  (DeepSeek 4).
- `AllocateEventSeq` being exported (DeepSeek 6).
- A seed-read failure now fails the projection closed, which Devin judged
  acceptable.

Round 1 fixed: `AllocateEventSeq` takes `eventMu` first and returns only
when both `Seq` and `StreamSeq` are set (`allocateEventSeqLocked` always
sets `StreamSeq`, so a retry still takes no second number). Two tests
fail on 31280c3ef and pass now. The two doc errors are corrected in the
review section above; plan item 3's premise (the digest covers the event)
is withdrawn, and so is the run-end two-events residual.
