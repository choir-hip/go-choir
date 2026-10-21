# Store engine concurrency: what the one connection protects, and the next step

Date: 2026-10-21. Status: design, not implemented. Station FP1
([choir-launch-fp1-store-throughput-2026-10-21.md](../definitions/choir-launch-fp1-store-throughput-2026-10-21.md),
item 3). Mutation class of the work it describes: red (store substrate).
Evidence classes used below: **code** (read in this worktree), **measured**
(run in this worktree, test named), **paper** (the orchestrator's read-only
counters of 2026-10-21), **hypothesis** (not shown).

## The question

Desk slots are 8 (`MaxConcurrent`, `internal/actor/dispatcher.go`). A desk
turn holds its slot while it waits for the store, and the store serves every
object-graph read through one engine mutex. The owner wants more desks per
computer. Can the slots rise, and what must be true of the store first?

## What exists (code)

| Handle | Pool | Users | Takes `engineMu`? |
|---|---|---|---|
| `Store.db` | 1 connection | `ogStore` (every `og_*` write and read through `DoltStore`), the projector (`finalizeBatch`, under `eventMu`), events, desktop, Texture tables, GC, checkpoints | `ogStore`: yes. Projector and the rest: **no** |
| `Store.readDB` | 1 connection | `ogReadStore`: 46 call sites in `internal/store` | yes, the same mutex as `ogStore` |

Both pools are opened on one embedded connector, so the engine already sees
two sessions. `engineMu` serialises every `DoltStore` call across both. The
projector, which is the only production writer of `og_objects`, is excluded
from the mutex and excluded from the read connection only by its own
connection being a different one: a projector transaction on `db` and an
`ogReadStore` read on `readDB` can run at the same time today.

Two guards therefore exist, and they are not the same guard:

1. **The pools' one connection** gives a serial history on `db`: one
   transaction at a time. `PutBatchConditional` (the lifecycle reducers'
   compare-and-write) depends on it: its compare is a `SELECT ... FOR UPDATE`
   in a serializable transaction, and nothing in this tree shows that the
   embedded engine locks a row for it. (Hypothesis: it does not.)
2. **`engineMu`** serialises `ogStore` with `ogReadStore`. It was added in
   488e8cd0 for a data race in the engine, recorded in
   [embedded-dolt-single-connection-is-a-fixed-upstream-race-2026-10-11.md](../problems/embedded-dolt-single-connection-is-a-fixed-upstream-race-2026-10-11.md):
   `unescapeHTMLCodepoints` wrote into a JSON buffer that aliased a shared,
   cached prolly node.

## What changed since the mutex was added

- `go.mod` pins `github.com/dolthub/dolt/go v0.40.5-0.20260716231223-6bea93f2f665`.
  The suffix is upstream commit 6bea93f2, the fix that record names (July
  16). **code.** The record's own follow-up, "remove the engine mutex in
  measured steps", has not been done.
- The online GC already depends on Dolt's session-aware safepoint across the
  write and read sessions (`MaybeRunOnlineDoltGC`, `internal/store/dolt_maintenance.go`).
  **code.** More read sessions mean a safepoint that waits for more sessions.
- The projector already runs concurrently with `ogReadStore` reads, without
  the mutex. **code.** So a writer beside a reader is the production shape
  today; what is new in this design is a second reader beside a first.

## What the paper says the engine is doing (paper, 122 s from 17:58:57)

Engine mutex held 70% of the time (85 of 122 s), 247.6 s of waiting. Of the
85 s, every operation in the top list is a read: three JSON scans 40.1 s
(this batch's side table removes them), `GetObject` 15.5 s held and 150.5 s
waited over 10,701 calls (1.4 ms each), `ListObjectsPage` 14.7 s over 417
calls, `ListObjectRefsByKindOwner` 2.8 s, `ListObjects` 2.5 s. **About 89%
of the held time is reads.** The writes are not in the list, because the
projector does not take the mutex; their cost is invisible to these counters.

The paper has 2 vCPUs. The mutex serialises reads onto one thread at a time
(70% busy is at most 0.7 of a core), so read concurrency can raise engine
throughput by at most about 2x before the daemon, the desks' own Go work and
the guest kernel contend for the same two cores. Reducing the work is worth
more than overlapping it:

| Lever | Removes | Risk | Verdict |
|---|---|---|---|
| Side table for the three scans (this batch) | 40.1 s of 85 s held | rollback staleness | landed here |
| Name `GetObject` / `ListObjectsPage` by kind (this batch) | nothing; says what to remove next | none | landed here |
| In-process cache of immutable objects | most of `GetObject`'s 10.7k calls, if they repeat | invalidation | next batch |
| A second read connection for pure reads | overlap of what is left, at most 2x | below | later, with evidence |
| More dispatcher slots | nothing: adds waiters | starves the store further | after the above |

A larger `MaxConcurrent` before the store has headroom adds contention for
the same mutex, not throughput
([research-more-slow-and-hidden-2026-10-21.md](../problems/research-more-slow-and-hidden-2026-10-21.md), E6).

## Measured here

Two probes in `internal/objectgraph/engine_concurrency_probe_test.go`, on the
pinned engine, over one connector with a writer pool of one connection and a
reader pool of four:

1. `TestSecondConnectionSeesEveryCommittedWrite`: after the writer connection
   commits, a read on the other connection sees that write and that content
   hash, 300 of 300 times, with and without `-race`. This is the freshness a read pool needs, and it
   contradicts the comment in `latestLifecycleRunByAgent` ("the read pool's
   transaction snapshot can predate a just-committed mint") for the case of
   an autocommit read after a returned commit. If that comment recorded a real
   stale read, it came from something else (a read inside a long-lived
   transaction, or a write not yet returned); that cause is **unknown**, and
   authority reads stay on the primary until it is found.
2. `TestSeveralReadersWhileOneWriterReturnWholeRows`: four connections read
   JSON with HTML-escapable content while a writer rewrites the same rows for
   six seconds; every count and every field is whole (4,539 reader
   iterations without `-race`). Run with `go test -race` (Go 1.26.9, darwin,
   845 iterations in the same six seconds) it reports no data race: the
   engine's shared buffers were under the detector.

What these do not show: behaviour under the guest's real mix (Texture
revisions, large bodies), GC with four read sessions open, memory per
session (the guest runs at 8.3 GB RSS), or the planner under concurrent
sessions. They are the first two of the tests the 2026-10-11 record asked
for, not a licence to remove the mutex.

## Design (next batch, B2b)

Keep `engineMu` for `ogStore` and every write. Add read concurrency only for
reads that are safe to serve from a snapshot, in steps, each measured on the
paper before the next.

1. **Measure after this batch lands.** Read the engine-mutex counters over
   120 s (now with `GetObject <kind>` and `ListObjectsPage <kind>`), the
   `dispatcher: activation wait` lines, and the validator's pages
   (`ValidateDerivedKeys page`, at most 25% duty). Decide from the numbers.
2. **Cache what cannot change.** An object whose identity is its content hash
   (`IdentityContentHash` kinds: captures, source entities and refs, events,
   messages) is immutable under its canonical id; a bounded in-process cache
   in `GetObject` removes repeat reads without a second connection. Keyed
   kinds are not cached (their id is stable and their content changes).
   Test: the differential read of cached against uncached over every writer,
   including a projector transaction that rolls back.
3. **Classify the 46 `ogReadStore` call sites and the `ogStore` reads**
   into `snapshot-safe` (a stale answer is a slower or older answer: lists,
   counts, public reads, Texture documents) and `authority` (a stale answer
   is a wrong decision: lifecycle singleton checks, reducer compares,
   obligation reads). Authority reads and all writes stay on `ogStore`. The
   classification is a table in code with a test that fails when a call site
   is unclassified, as the writer guard in this batch does for `og_objects`.
4. **Give `snapshot-safe` reads a pool of two connections and their own
   mutex-free path**, `MaxOpenConns(2)` on `readDB`, `engineMu` no longer
   taken by `ogReadStore`. Stop condition: any `-race` report, any read that
   observes a state older than its own session's last commit, any GC
   safepoint wait over 10 s, guest RSS up more than 5% at the same load.
5. **Only then raise `MaxConcurrent`**, one step at a time (8, 10, 12), each
   judged on `GetObject` wait and `slots=n/n` frequency.

Rollback is `git revert` of each step; steps 2 to 4 are independent of the
side table and of each other.

## What this batch lands for it

- The side table (docs/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md).
- Kind labels on `GetObject` and `ListObjectsPage`.
- The two probes above and this design.
- Nothing that removes or narrows `engineMu`.

## Why not implement it now

The change is on the store substrate, a protected surface that must land in
a deploy of its own. Its acceptance is a measurement under the paper's real
load, which cannot be had before the side table is on the paper (the scans
are 47% of what the new connection would overlap). The two probes give the
direction, not the proof. The brief for this batch allowed implementing it
only if it was clear and testable inside the batch; the part that is clear
(what to measure) is in, and the part that needs the paper's numbers is the
next batch.

## Heresy delta

- discovered: the comment in `latestLifecycleRunByAgent` about a stale read
  pool is not reproduced; the projector already runs beside readers with no
  mutex; the engine mutex outlived its stated reason.
- introduced: none.
- repaired: none.
