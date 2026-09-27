# M11 blocker: guest restart cancels the bound desk run terminally; the selfdev operation strands `executing` forever

**Status:** open, blocking M11 episode leg `primary_started -> awaiting_approval`.
**First observed:** 2026-09-27, staging `b0a21b18`, probe run
(op `selfdev-75b0c0e8ab796cecaeae01444222fe3d`, computer
`computer-f53b4421fed0a62d2dd6ebc982e4b8a2`, VM `vm-a597952dab44a90b9db0de622fe30865`).

## Evidence

The desk run (`run:assignment-0ac62c54-2e48-5daf-8c8a-0e1d077ea98f`) was
healthy on the RLM carrier — 54 iterations, `choir.ReadFile`/`ListDir`/
`WriteFile`/`Exec` all executing on real platform source, a self-built
verification checker iterated to a failing test — when the guest autoputer
restarted (deploy re-serve of active VMs; the VM's tap re-bound
10.200.12.2 -> 10.200.14.2). On restart:

```
activation.passivated {"recovery":"passivated_on_restart"}
run cancelled: "restart revoked absent assignment capsule"
```

`reconcileEngineeringAssignmentCapsulesAfterRestart` (fate.go:522) revoked
the capsule (tmpfs, absent post-restart) and issued
`CancelEngineeringAssignment` — disposition `cancelled`, which is
**terminal** (`EngineeringAssignmentDisposition.Terminal()` includes
cancelled).

The selfdev operation remains `state: executing` with no further authority
path: `reconcileEngineeringCast` (engineering_desk.go:153) short-circuits
on `existing.Disposition.Terminal()`, so re-delivery of the cast
occurrence returns the cancelled row and never re-executes. The operation
waits on a desk that can never run again.

## Diagnosed cause

Two coupled gaps:

1. **No re-cast path after restart.** The deterministic assignment
   identity is `(owner, computer, trajectory, revision, kind, attempt=1)`.
   A restart-cancelled implementation assignment leaves no route for the
   same admitting revision to spawn `attempt=2`. The occurrence
   (DocumentRevisionOccurrence / coagent wake) can refire via dispatcher
   backoff, but the reconciler treats the terminal row as done.
2. **Op never observes desk death.** The selfdev operation's `executing`
   state has no transition on bound-assignment `cancelled`; it is not
   `failed`, not retried, not parked. Operator-visible strand with no
   terminal_error on the op record.

M7's derivable continuations cover materialize->apply *after* a decision
commits; they do not cover desk-run death *before* freeze.

## Blast radius

Any guest restart (deploy re-serve, hibernate/resume crash, pressure
reclaim during a desk run) permanently strands the bound selfdev op at
`executing`. On staging this fires on every deploy that touches active
computers — the probe's own dependency chain (push commit -> deploy ->
guest restart -> dead desk) self-churns the episode under test.

## Fix options (not yet decided)

- **A.** Attempt-bump re-cast: on restart reconciliation, a
  terminal-cancelled implementation assignment whose bound op is still
  `executing` re-opens as `attempt=N+1` (new capsule, fresh desk run).
  Preserves the event chain; the cancelled attempt stays on the tape as
  evidence of the interruption.
- **B.** Op-level terminalization: bound-assignment `cancelled` transitions
  the op to `failed` with `terminal_error`, letting the caller re-open a
  new operation. Simpler, but loses in-place recovery.
- **C.** Probe-only mitigation: pin deploys so no push lands mid-episode.
  Does not fix the product surface — deploys *must* be safe mid-episode.

A is the doctrine-conformant direction (the occurrence refires; the desk
is restart-durable by design — "the cell may have partially executed"
already assumes re-delivery), but the assignment-attempt arithmetic needs
review against `deterministicDocumentAssignmentIdentity`'s replay
guarantees.

## What proves closure

`m11_selfdev_episode_probe.mjs` reaches `awaiting_approval` across a guest
restart mid-desk-run: the op either re-casts (attempt 2) or fails with a
typed terminal state, never strands `executing` with a terminal-cancelled
assignment.

## Recovery posture

- Stranded ops are disposable (fresh-owner computers); no data risk.
- Ops stranded `executing` hold the trajectory's pending work item; the
  host shows the operation mid-flight indefinitely.
