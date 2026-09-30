# coagent_result consumed without reactivating the parked run — deliverable-strand class

**Status:** HYPOTHESIZED 2026-09-30 on staging (build `37882e1d`, guest
`computer-03335285269bdba4f94377e56879f9e6`, epoch 979). Root-cause
confirmation needs guest-internal actor-log / resume-snapshot reads, which are
not host-reachable. Documented per
`docs/memo-problem-documentation-first.md` — the redrive chain is deployed and
proven live; this is the residual strand it was meant to close.

## Symptom

Researcher `research:cfa90b87-cd5b-4950-8697-93ab1ad1954e`, run
`362febb2-c58f-479c-9f20-98fb659fa97e`, trajectory
`ba6199f1-3e71-5d0e-8200-fffc93c973e2`: run remains `passivated`
(`updated_at` frozen at `2026-09-30T17:41:29Z`) across two guest boots
(`2493636`, then `2495033` after a ~22:52 platform-dolt OOM kill). The
`coagent_result` obligation (worker update `4158e48b`, work item `7be3d1de`)
is confirmed still open — the bound run is `passivated`, so
`actorWakeOutboxFromWorkerUpdate` correctly yields a wake, and the 23:16
migration logged `minted 1778 pending wakes` (re-arming this wake among them).

Yet the run never reactivates. No `disposed dead wake` for its source id, no
`wake outbox dispatch` error, no `dispatcher: deferred|poison|handle` line
naming `cfa90b87`/`research:`/`4158e48b`/`ba6199f1` across 60+ minutes of
journald.

## Why every delivered-path theory is ruled out

- **Not queue position**: the storm self-poisoned (3 rows past
  `MaxDeferrals=64`), deferrals are zero for 25+ min, disposals froze at 151 —
  the sweep is past the backlog.
- **Not a dead wake**: `control_delivered` on the tape is the non-exhausted
  bind (`lifecycle_update_delivery.go:144`), which leaves
  `Disposition = UpdatePending`. The obligation is live, not disposed.
- **Not suppression**: `actorWakeOutboxFromWorkerUpdate` suppresses only on
  non-pending disposition; `UpdatePending` + dead bound run yields the wake.
- **Not a salt/dedup failure**: `dispatchActorRedrive` mints `<base>#redrive-N`
  when the consumed identity family is fully processed; unit test
  `TestDispatchAtRedrivesConsumedCoagentResultWithSaltedGeneration` green.
- **Not a CAS-loss strand**: `MarkActorWakeProjected` only writes
  unprojected→projected; a wake that's already projected has no concurrent
  writer to lose `ExpectedContentHash` against in a serial migration loop.
- **Not an absent append**: the salted `coagent_result` appends to the
  SQLite actor mailbox (not the trajectory event tape), and
  `control_delivered` is a bind event that does not re-emit — so absence of a
  new tape event proves nothing. The dispatcher IS running (management
  occurrences received every few seconds).

## Leading candidate (unconfirmed)

`handleCoagentResult` (`internal/actorruntime/handler.go:419`) branches on
`decodeResumeState(memory)`. If the actor's durable snapshot lost
`resume.RunID` (empty), it takes the `rs.RunID == ""` arm at line 609 →
`reconcileCoagentWake` → `acknowledgeDurablyTerminalLifecycleControlActivation`
→ returns `nil` → the dispatcher **incorporates the occurrence** (line 430)
without ever calling `ReconcileParkedLifecycleCoagentWake`. The obligation
discharges; `362febb2` stays passivated forever because the run that needed
the wake was never reactivated and no further wake exists to fire.

This would make `coagent_result` delivery a silent one-way strand for any
parked run whose actor snapshot lost `resume.RunID` — the deliverable
disappears exactly once, with no durable record of the missed reactivation.

## Alternative candidate

`reconcileCoagentWake` on the `rs.RunID==""` arm may also `ErrDeferUnprocessed`
when no canonical outcome is decidable — but deferrals log, and none name the
agent, so the silent-incorporate arm is favored.

## Needed to confirm (not available to this session)

- Read the actor mailbox / resume snapshot for `research:cfa90b87` on the
  guest's embedded SQLite (requires guest-internal access; the read-only
  `/tmp/guest-data-ro` mount is a stale snapshot and `platform`@13306 is the
  host store, not the guest's).
- If `resume.RunID` is empty for a parked run that owed a wake: fix is to
  carry the bound `DeliveredToRunID` from the obligation into the reactivation
  path rather than relying solely on the snapshot — or to treat a parked
  run with an open bound obligation as resume-authoritative.

## Related

The parked-run reactivation path `recoverParkedLifecycleMailboxSnapshots`
(`adapter.go:524`) is the boot-time hedge for exactly this class; whether it
fired for `cfa90b87` is the same guest-internal question.
