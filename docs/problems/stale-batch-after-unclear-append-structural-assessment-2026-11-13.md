# Structural assessment: a stale batch after an unclear append

Written 2026-11-13T23:10Z, after three review rounds on one mechanism did
not converge (contract, "Before you change anything", rule 7).

## The defect

On main today. An append lands on the host but its reply is lost. The next
run-memory write for that run numbers itself from a local table that lacks
the landed entry. A duplicate `(loop_id, seq)` reaches the canonical tape
and every later finalize fails on `UNIQUE(loop_id, seq)`: the tape is
wedged by an event the host already holds. Found while hardening write
coalescing (`lane/s5-coalesce`); coalescing did not introduce it.

## The three rounds

| Round | Mechanism | Verdicts | What broke it |
| --- | --- | --- | --- |
| 1 (`1afc773ab`) | store-level latch, cleared with `Swap(false)` before the settle | REPAIR, ACCEPT | a second run reads the latch as clear while the first is still settling; reproduced as a wedge |
| 2 (`652238d2c`) | latch cleared after the settle, under a settle mutex | REPAIR, REPAIR | the settle returns nil when the prepared row was discarded and catch-up failed; the clear erases a concurrent set |
| 3 (`6c767dc0d`) | latch deleted; the appender re-runs the dry run when this append's own recovery did work | ACCEPT, REPAIR | another writer's recovery between this writer's dry run and its turn under the appender mutex is not seen; shown at the appender level, not reproduced in the store harness |

## What the rounds have in common

Each mechanism tried to detect the event that makes a batch stale (a failed
append, a settle, a recovery inside this append). Each time a reviewer found
another event of the same kind that the detector did not watch. The frame
was wrong: the question is not "did something happen" but "is the state the
batch was validated against still the state".

## The changed approach

Optimistic concurrency on the projection head. The store's dry run already
reads the projection head; the head and the projected rows commit in one
transaction, so an unchanged head means unchanged state. The batch carries
the head it was validated against. Under the appender mutex, before anything
is pinned or sent, the appender compares that head with the current local
head. Equal: proceed, at no extra cost. Different, for any reason (this
append's recovery, another writer's recovery, another writer's ordinary
finalize): re-run the dry run against current state and refuse the batch if
it no longer fits. One comparison replaces every event detector.

The cost to watch: under load other writers finalize between a writer's dry
run and its turn, so the head may differ often and the dry run would run
twice per append. The group commit's leader token should make that rare on
the coalesced path. The build measures revalidations per append under
concurrent writers; if it is not near zero the dry run moves under the
mutex instead of being repeated.

## If this does not converge

The lane splits. Coalescing and its accepted hardening land without any fix
for this defect, which leaves main no worse than today, and the defect
becomes its own lane with its own goal. That decision is taken after one
review of the new approach, not after another three.
