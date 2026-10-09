# Every replay page scans the computer's whole chain; app-layer apply rolls back on the owner computer

Date: 2026-10-09. Found by the failed staging deploy of `c72c38c4` (CI run
37869452392). Mutation class of the fix: red (canonical event replay read
path; projection resume and app-layer apply depend on it). Related:
[`app-layer-push-health-gate-not-commit-bound-2026-10-07`](app-layer-push-health-gate-not-commit-bound-2026-10-07.md)
(same rollback, gate half fixed `68397ae7`, probe half mitigated `19d7913e`).

## Evidence

**Deploy.** `c72c38c4` changed only guest code, so the deploy took the
app-layer push path (the three earlier deploys today rebuilt guest images;
their app-layer phase was 0 s). Result: `app-layer push: 2/3 applied`. The
owner computer `candidate-fleet-e15cb89f…` (computer-03335285…) rolled back
to `eedd7b22`.

Owner console (Node B `vm-state/candidate-fleet-e15cb89f…/console.log`):

- 01:32:08 old runtime receives SIGTERM; shutdown deadline exceeded,
  stopped 01:32:18.
- 01:32:18 new release starts; `projection recovery resume (local=576514
  W=574653 H=576514 tail=0)`; server listening 01:32:20.
- 01:32:38 the updater kills it: `health probe failed: health status=503
  replaying committed_sequence=0 progress=0`. The prior release is
  restored. Its first replay page fetch (`after=576514 count=0`) took
  **17.8 s**.

The updater's probe allows 30 polls without progress. About 10 of them went
on the old process's shutdown (connection refused) and the rest on a
`/health` that reports `committed_sequence=0 progress=0` while the first,
empty page is in flight. The budget ran out about 18 s into the new process.

The empty first page fetch is slow on every boot of this computer:
19:43 27.7 s, 21:33 27.1 s, 23:08 18.7 s, 00:09 16.9 s, 00:10 16.2 s,
01:02 28.1 s, 01:03 17.0 s, 01:32 17.8 s.

**Store A (read-only probes, 2026-10-09 ~02:10Z).** The page query in
`internal/platform/event_replay.go`:

```sql
SELECT … FROM computer_event_append_receipts
WHERE computer_id=? AND sequence>? ORDER BY sequence LIMIT ?
```

- Takes **3.6 s with no other load, even when it returns zero rows**.
- `EXPLAIN PLAN`: Dolt chooses the primary key `(computer_id,
  idempotency_key)` with only the `computer_id` prefix, reads all 576,855
  rows of this computer, then filters on `sequence` and sorts (TopN). The
  unique index `(computer_id, sequence)` is ignored. `FORCE INDEX` and a
  bounded `BETWEEN` do not change the plan.
- The same page as `sequence IN (a+1, …, a+128)` uses the `(computer_id,
  sequence)` index: **0.02 s** for a 128-row probe (the production page size
  is 1024).
- The chain is gapless: `COUNT(*) = MAX(sequence) = 576855` for this
  computer, and appends are `sequence = head + 1`.

## Problem

1. **Substrate: each replay page costs O(chain length).** On this computer
   one page is at least 3.6 s of server time, and 17–28 s from the guest at
   boot, when several computers read Store A at once. A full replay of 577k
   events at the 1024-event page size takes about 564 pages × ≥3.6 s, so at
   least 34 min of query time alone.
   This is plausibly the cost behind past "large-history replay outlasts the
   apply fence" findings. Hypothesis, not verified against old traces.
2. **Liveness signal missing during a projection resume.** `/health` reports
   `committed_sequence=0` although the projection resumed at 576514. A slow
   first page therefore looks like a stalled replay to the updater, and
   `19d7913e` (an advancing replay counts as liveness) cannot help.
3. **Stall budget counts the old process's shutdown.** About 10 of the 30
   polls go on connection refusals before the new release is listening.
4. **Unexplained (hypothesis):** on every boot, `replay complete` is logged
   9.5–11.5 min after the runtime starts. `/health` leaves the replay gate
   about 20 s in, so this lag is outside the gate. Not investigated here.
5. **Residual:** checkpoint lookups `WHERE computer_id=? AND
   event_head_receipt_id=?` (`internal/platform/checkpoints.go`) have no
   index on `event_head_receipt_id`. By the same planner behaviour they
   likely scan the computer's chain too. Not measured.

## Fix direction

Fix the substrate first: issue the replay page as point lookups on the
`(computer_id, sequence)` index (`sequence IN (after+1 … after+n)`). This
keeps the same rows and order, and the contiguity checks already in the
reader stay. With the page at about 20 ms, problems 2 and 3 stop causing
rollbacks. They remain correctness gaps in the liveness signal and are
recorded for SL/O7 (boot-cost budget), not patched here.

## Side effect during this investigation

Running `nix shell nixpkgs#mariadb.client` on Node B triggered Nix's
automatic min-free GC (the root filesystem is 81% full). It deleted only
unrooted store paths (for example, old `choir-builder` builds). Rooted
deploy closures are unaffected.

## Status

Fix: replay pages are point lookups bounded by the head, and a gap below
the head is a typed error (`internal/platform/event_replay.go`, test
`TestEventsPageWalksGaplessChainToHead`). Staging acceptance: pending
deploy.
