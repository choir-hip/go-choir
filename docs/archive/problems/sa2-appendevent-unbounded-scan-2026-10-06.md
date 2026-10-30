# AppendEvent scans the whole event objectgraph per write — OOM at owner scale

**Status:** confirmed on staging 2026-10-06 via `management-open` → `IssueLifecycleControl` → `UpdateRun` → `AppendEvent`. Blocks owner-scale mints; every event append on a large store carries the cost.

## Evidence

- `internal/store/store.go` `AppendEvent` (~line 2230): to assign `Seq` and `StreamSeq` it calls `s.og.ListObjects(ctx, {Kind: ogKindEvent, Limit: 100000})` **twice** — once for max `Seq`, once for max `StreamSeq`. Each call walks up to 100k event objects in the Dolt objectgraph.
- Guest console, owner computer `computer-03335285…` build `2921c3ca`, 21:56:11: `persist … event: list all events: objectgraph dolt: scan object: context deadline exceeded` followed at 21:58 by `Out of memory: Killed process 649 (autoputer) total-vm:5801156kB, anon-rss:3701512kB`.
- Owner store: `/var/lib/go-choir/vm-state/candidate-fleet-e15cb89f25d963c220319b7b` = **31 GiB**; guest Dolt GC deferred (`exceeds safe bounded-guest GC size (5 GiB)`), so the objectgraph grows monotonically.

## Why this is substrate, not probe

Every `AppendEvent` — control issue, run update, texture write, engineering work item — pays the scan. The mint path just happened to be the first code to hit it at owner scale (fresh disposables have empty stores, so no probe caught it). Fixing the probe doesn't fix the defect: any sustained write path on a mature computer degrades into OOM.

## Fix shape (for SA)

The defect is that `Seq`/`StreamSeq` derivation reads O(N) state to compute what should be O(1). Options, cheapest first:

1. **Maintain a persisted cursor.** Store `max_seq`, `max_stream_seq` per (owner, computer) in a small metadata object; update under `eventMu`. Append becomes `cursor+1` + write — no scan. Reconcile the cursor at boot from a bounded tail read (last N events), not a full scan.
2. **Bounded tail read.** Replace `ListObjects(100000)` with a descending-order query limited to the last few hundred events per run/stream — if the objectgraph supports ordering/limit-descending. Less invasive than a cursor but still O(page) per append rather than O(store).
3. **Denormalize on append.** Keep `StreamSeq` in memory (it's monotonic); persist the high-water mark on clean shutdown + recover on boot. Adds a recovery invariant (crash between writes) — needs a reconciliation pass.

Option 1 is the boring/durable fix and matches the existing `eventMu` critical section. Whatever lands must come with a regression gate: `AppendEvent` against a store with N events must complete in O(log N) or O(1) — measurable by seeding a synthetic store with a large event count and timing the append.

## Interaction with mint path

`UpdateRun` inside `IssueLifecycleControl` calls `AppendEvent` for the control work item + lifecycle update. Even after `management-open` is otherwise correct, owner-scale mints OOM until this is fixed. The disposable-computer SMG probes remain valid — they exercise the control path on an empty store.

## Rollback

None required — record-only. The defect is pre-existing; this doc names it so the next fix targets the scan, not the caller.
