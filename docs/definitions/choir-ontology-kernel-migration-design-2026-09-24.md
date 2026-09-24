# K Migration Design — Tape Topology and the Store→Actor Projection

**Date:** 2026-09-24
**Status:** design for the K execution slices; inherits
`docs/definitions/choir-ontology-kernel-2026-09-24.md` (the charter) and the
ratified ontology (`docs/archive/choir-event-driven-rlm-ontology-minimal-2026-09-15.md`).
**Mutation class:** green (design doc; the work it plans is red).

## The problem this resolves

The kernel substrate landed (slices 1–2): `actor_updates` gained
`not_before`/`epoch`, `actor_heads` holds the durable per-actor epoch/head,
`SQLiteLog.Commit` is the fenced atomic commit, `PendingAgents`/`NextDue` are
the projection and due-index, and `Dispatcher` consumes the projection
serially per actor. `WithKernelMode()` wires the adapter.

But the substrate owns only the **actor mailbox tape** (`actor_updates`).
The wrong-path cluster's continuations are mostly **store→actor** wakes: a
sweep enumerates store state (`ListRunsByState`, `ListLifecycleSubjects`,
`ListOpenAssignedLifecycleWorkItems`, `ListCoagentMailboxBacklogAll`) and
mints an actor wake. Deleting those sweeps requires the wake to be derivable
— a store lifecycle event must mint the actor wake event on the tape, and
the dispatcher delivers it. Without that projection, deleting a sweep strands
the continuation it was causing.

## Tape topology decision

There are two durable logs, and the design must name which is "the tape":

1. **`actor_updates`** (SQLite, `internal/actor`) — the actor-to-actor
   delivery tape. `processed_at` is the state head; the pending projection is
   due unprocessed rows. The dispatcher owns this. **This is the delivery
   tape** — the thing the dispatcher consumes to fire activations.
2. **`computerevent` + object-graph lifecycle events** (Dolt,
   `internal/computerevent`, `internal/store/lifecycle.go`) — the canonical
   event log and the `QueueLifecycle*` reducer commands (the keepers). This
   is the **authority tape** — where lifecycle/work-item/run state lives.

The ontology's "the tape" is the authority tape; the delivery tape is its
projection surface for actor delivery. The kernel's job is to make the
delivery tape a **pure projection** of the authority tape: every actor wake
is minted by folding a canonical event, never by scanning store state.

## The store→actor projection (the missing substrate)

Replace "scan store → wake actor" with "canonical event → actor wake event →
dispatcher". The two tapes live in **different stores** (Dolt for the
canonical event log, SQLite for `actor_updates`), so the wake mint cannot be
atomic with the event append. The correct model is a **derivable
projector**, not a cross-store transaction:

- A `QueueLifecycle*` command appends its lifecycle event (already does —
  the keepers). A **projector** folds new canonical events and mints the
  addressed actor wake onto `actor_updates`, tracking a durable projection
  cursor (its position in the canonical log).
- The mint is **idempotent**: the actor wake's `update_id` is derived
  deterministically from the canonical event id (same scheme as
  `actorDispatchUpdateID`), so a replayed fold re-mints the same wake and
  `Append` dedupes it. A crash between event-append and wake-mint is
  recovered by re-projection from the cursor — the wake is derivable, never
  lost.
- The projector is itself a tape consumer like the dispatcher: pending =
  canonical events minus the projection cursor. It is the dispatcher's
  projection extended one level up to the authority tape — not a wrong-path
  scan, because it folds the tape rather than enumerating mutable state.
- The dispatcher's pending projection (`actor_updates` due unprocessed) is
  then the delivery authority; the projector is the derivation that feeds
  it. Together they are the one continuation path.
- Recovery: on restart, the projector resumes from its cursor and the
  dispatcher re-reads its projection; any wake minted-but-unincorporated is
  re-delivered. No boot sweep enumerating store state.

## What the projector must reuse

`wakeUpdatedCoagent` is not a bare wake mint — it binds lifecycle controls
to resident runs (`bindLifecycleControlsToRun`) before dispatching. The
projector must call `wakeUpdatedCoagent` per backlog row, not re-implement
the wake, so the resident-binding path is preserved. The projector is a
resumable fold over the coagent mailbox backlog (`ListCoagentMailboxBacklogAll`)
with a durable cursor, run continuously — not only at boot — so the crash
window between coagent-write and wake-mint is covered by re-projection, not
by a boot scan. `sweepPendingUpdateActors` is exactly this fold run once at
boot; the projector is the same fold made resumable and continuous.

## Per-sub-class migration contract

Each wrong-path instance migrates by the same rule: **the continuation it
caused becomes an event on the authority tape, folded to mint the actor wake
on the delivery tape; the process-local mechanism is deleted.**

### (b) Process-local continuations — `time.AfterFunc`, `context.AfterFunc`, detached `go`, coalescer timers

Replacement: an unaddressed scheduled event carrying `not_before` on the
delivery tape; the dispatcher's due-index mints the addressed wake when due.
For store-plane deadlines (activation budget, assignment fate watchdog,
persistent-Management watchdog), the deadline is a lifecycle event with a
due time; the fold mints the `not_before` actor wake. The detached
`rlm_reduce` fate commit becomes an executor-actor continuation driven from
the work stream (ontology P5) — the terminal proposal is staged before
freeze, the executor mints the terminal event after the revoke ack.

### (d) Sweep/enumerate-state recovery — boot/recovery scans

Replacement: the dispatcher's pending projection is the recovery rule. Boot
scans that rewarm passivated runs / reactivate actors are deleted; the wakes
they would have minted are already on the delivery tape (minted by the fold
when the lifecycle event landed). `StartKernel` replaces `Sweep`.

### (e) Non-event mutations — direct `UpdateRun`/`UpdateWorkItem`/`UpdateTrajectory`/`PutBatchConditional`

Replacement: an event-backed reducer command (`QueueLifecycle*` or a new
typed command) that appends the event and folds it into state. The direct
`Update*` callers die; the reducer commands are keepers.

### (c) Dual paths — legacy JSON capsule ops, `DispatchWorkerUpdate`, `report_to_texture`

Replacement: the single canonical path (staged cell intent + reducer event).
`DispatchWorkerUpdate` callers migrate to `QueueLifecycleUpdate`; the legacy
queue is drained inside the write fence. R3 completes the desk crossings.

## The write fence

The cutover (flip `WithKernelMode` + migrate pending rows) runs inside a
write fence: quiesce the old writers, mint a delivery-tape wake for every
pending row in the old tables (deterministic mint identity, `not_before`
seeding for live deadlines), then cut over. No mixed-authority interval —
the charter's hard requirement. The migration must be reversible or the
rollback is a restore, not a revert.

## What is already true vs what remains

**True now:** the delivery tape carries `not_before`/`epoch`; `Commit` is
the fenced atomic commit including the folded memory snapshot (moved inside
the transaction after the 2026-09-24 panel); `PendingAgents`/`NextDue` are
the projection and due-index; `Dispatcher` consumes serially per actor with
a bounded `MaxConcurrent` fan-out and a `Stop` that waits for in-flight
activations; `WithKernelMode` wires the adapter; emissions buffer into the
fenced commit (per-event drain; a failed handler's emissions are discarded
and the batch stops at the first failure, preserving tape order); poison
events route `delivery_failed` to the error sink after `MaxAttempts` with
durable retry accounting.

**Consensus review 2026-09-24 (convergent, 4/8 panelists returned —
codex, gpt6-sol, claude, gemini38): unanimous SEND BACK.** Blockers closed
in `5d1360f8`/`f733106c`: atomic snapshot, `not_before` round-trip, tape
order, per-event buffer, `Stop` WaitGroup, `MaxConcurrent`, channel-cast
emission escape. **Open design findings the panel confirmed (not yet
fixed):** the projector is a 500ms state scan over `ListCoagentMailboxBacklogAll`
that mutates resident runs via `bindLifecycleControlsToRun` — not the
derivable event fold this design specifies; the epoch is a commit-time
conflict check, not a preemption lease (no claim/revoke); `frame_lock`
edges (`sourcecycled`, `vmctl`, `frontend-current`) remain boundary
exceptions. These are the remaining-work items below, now panel-confirmed.

**Remaining:** the store→actor projection (fold mints actor wakes in the
event-append transaction — replaces the 500ms scan); the per-instance
migration of the 51 classified instances; the write-fence cutover; the
cluster recount to zero; staging verification; a second consensus review
after the projector redesign.

## Boundary exceptions (unchanged)

`sourcecycled` ticker, `vmctl` sweeper, `frontend-current` pointer — deferred
external control planes, not in-scope (see the charter).
