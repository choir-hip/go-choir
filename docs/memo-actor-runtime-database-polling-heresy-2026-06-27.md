# Heresy: Actor Runtime Database Polling (3rd occurrence)

**Date:** 2026-06-27
**Heresy ID:** H030
**Status:** discovered and repaired
**Mutation class:** orange (runtime behavior change)

## The Heresy

The actor runtime (`internal/actor/actor.go`) was designed to use Go-channel
mailboxes for message delivery, with a durable log only for crash recovery.
The design document states explicitly:

> An agent is a long-lived actor: a goroutine with a Go-channel mailbox while
> resident, an event log + compacted memory while passivated.
>
> **The database remembers. Go delivers.**

Despite this, the implementation polled the SQLite database every loop
iteration as the delivery mechanism. There were zero Go channels in the actor
runtime. The database was both the memory AND the delivery mechanism.

This is the **third time** the actor runtime ended up database-backed instead
of using Go concurrency primitives:

1. **Old `channels.go`** — database-polling message bus (deleted in Phase 2a)
2. **Actor runtime design** — explicitly said "Go-channel mailbox, database
   remembers" (docs/choir-rearchitecture-durable-actors-2026-06-11.md)
3. **Actor runtime implementation** — polled the database every loop iteration,
   no Go channel in sight

## Evidence

The original `loop` function (before fix):

```go
func (rt *Runtime) loop(ctx context.Context, r *residentActor) {
    for {
        backlog, err := rt.log.Unprocessed(ctx, r.agentID) // DB query every iteration
        // ... process backlog ...
        r.pending = r.pending[:0] // vestigial slice, ignored
    }
}
```

- `residentActor.pending` was `[]Update` (a slice), not `chan Update`
- `Send` appended to the slice, but the loop ignored it and re-queried the DB
- The comment said "steers are already in the log; re-query" — admitting the
  database was the delivery mechanism
- Zero `chan` declarations in `internal/actor/actor.go`
- The `pending` slice was cleared at the end of each iteration, making it
  purely vestigial

## Why This Keeps Happening

The pattern recurs because:

1. The durable log is the obvious place to look for "what messages exist" —
   it's a query-able store, so polling it feels natural
2. Go channels require thinking about buffer sizes, overflow, non-blocking
   sends, and the relationship between channel delivery and log persistence
3. The "database remembers" half of the design is easy to implement; the "Go
   delivers" half requires more care
4. When under mission pressure, polling the database is the path of least
   resistance — it works, it's simple, and the performance cost isn't
   immediately visible

## The Fix

Applied 2026-06-27. The fix replaces database polling with Go-channel delivery:

1. `residentActor.pending []Update` → `residentActor.mailbox chan Update`
   (buffered Go channel, default capacity 256)
2. `Send` does a non-blocking channel send when warm (select with default for
   overflow)
3. `loop` restructured:
   - Cold start: replay log backlog once (only log-as-delivery query)
   - Drain channel of messages already processed during cold-start replay
   - Warm loop: `select` on channel + idle timer (no log polling)
   - Post-drain: one backlog query for overflow + handler-error retries
   - Passivation: idle timer fires with empty mailbox → save snapshot, exit
4. Skip set prevents double processing (messages can be in both channel and log)
5. Snapshot saved under lock during passivation

The database is now exclusively the recovery substrate. Go channels are the
delivery mechanism. As the design intended.

## Verification

- All 8 existing actor tests pass (`go test ./internal/actor/ -v -count=1`)
- Actorruntime adapter tests pass (`go test ./internal/actorruntime/ -v`)
- Full build compiles (`go build ./...`)
- Independent review verified 8 correctness properties (PASS WITH NOTES)
  - One concern: skip-set race window (mitigated by handler idempotency)
  - Fix applied: skip set persists for activation lifetime (no clear between
    iterations)
- Second review found `processOne` missing skip check — fixed
- Third review verified SHIP

### Comprehensive test repair (2026-06-28)

Deep review found three broken tests behind the `//go:build comprehensive` tag
that referenced symbols deleted during the H030 repair and Phase 2 deletions:

1. `TestScheduleTextureWorkerWakeLeadingCoalesce` — referenced deleted
   `textureWakeKey`, `rt.textureWakeMu`, `rt.textureWakePending` (old timer-based
   debounce system replaced by actor dispatch). Test deleted.
2. `TestHandleTopologyReportsOrchestrationShape` — referenced deleted
   `rt.ChannelManager().Channel()`. Updated to expect `ChannelCount: 0`.
3. `TestConcurrentWorkers_IndependentChannels` — referenced deleted
   `rt.ChannelManager().Channel(id)` for existence check. Removed; kept
   store-backed `ChannelPost`/`ChannelRead` verification.

All comprehensive-tagged tests now compile and pass. See the mission ledger
for full details.

## Lesson

When a design says "Go delivers, database remembers," the implementation must
use Go concurrency primitives for delivery. The database is the recovery
substrate, not the delivery mechanism. If the implementation finds itself
polling the database in a hot loop, it has regressed to the old model under a
new name.

The test for this: **are there any `chan` declarations in the actor runtime?**
If not, it's database-polling regardless of what the comments say.

## Heresy Delta

- `discovered`: 2026-06-27, during worktree cleanup audit
- `introduced`: original actor runtime implementation (mission 3c_2)
- `repaired`: 2026-06-27, Go-channel mailbox fix

## See Also

- [docs/choir-rearchitecture-durable-actors-2026-06-11.md](choir-rearchitecture-durable-actors-2026-06-11.md)
  — original design doc, section 2.2 "Messaging: Go delivers, the database
  remembers"
- [docs/mission-3c_2-actor-runtime-migration-real-v0.md](mission-3c_2-actor-runtime-migration-real-v0.md)
  — migration mission that produced the database-polling implementation
- [docs/choir-doctrine.md](choir-doctrine.md) — H001-H029, prior heresies
