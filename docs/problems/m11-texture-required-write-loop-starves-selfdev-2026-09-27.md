# M11 blocker: Texture required-write loop starves the engineering desk op

**Status:** documented 2026-09-27; fix pending.
**Evidence:** staging, computer `computer-9d8257559c5761cf07502d2bfb54f184`,
op `selfdev-3c27040a8900fa53ebd58831c13887d2` (probe run r5, timed out after
60 min in `executing`); Node B journal 22:45–22:56 UTC.

## Observation

The selfdev probe's engineering desk operation never reached
`awaiting_approval`. The tape (canonical events) halts at seq 80
(`projection_batch_recorded`, 22:07:21) while the guest runtime kept working.
Between 22:45 and 22:56 the autoputer log shows an unbroken ~45-second loop:

```
runtime: run <id> → failed: tool loop: required write tool did not succeed after 2 retries
texture:24ac3db1…/1c468ab9… until <+45s> cause=actor: defer unprocessed
  occurrence: actorruntime: Texture activation returned without disposing
  exact trigger
```

A `request_source=update_coagent` Texture continuation wakes, fails the
required-write check twice (`runtime.go:3294-3299` demands a
`desk_go_eval` result containing `rlm:texture_apply:`), the run terminates
failed *without disposing the occurrence*, and the dispatcher redelivers it
~45 s later — indefinitely. This is the desk receiving a worker-update
continuation it never completes: the model either isn't calling
`choir.ApplyTexture` in its cell or the receipt isn't carrying the marker.

## Structural failure (substrate, not prompt)

Two bugs compound:

1. **No disposal/backoff on required-write failure.** An occurrence whose
   activation fails the write contract is deferred and redelivered forever
   at fixed cadence. There is no max-attempts, exponential backoff, or
   terminal disposition — a broken trigger becomes a permanent spin.
2. **Starvation under a stuck trigger.** The retried Texture activation
   consumes the desk cell budget every cycle; the engineering assignment
   (separate desk, separate capsule) sits `executing` for an hour because
   the operation's next transition never schedules.

The probe's `awaiting_approval` timeout is the *symptom*; the loop is the
substrate defect. This class of bug (wake → fail → no disposition → redeliver)
is exactly what `actor_wake_outbox` + `MarkActorWakeProjected` was built to
fence — the `request_source=update_coagent` continuation path evidently
doesn't ride it.

## Likely root-cause candidates (not yet confirmed)

- The Texture desk's cell isn't staging `choir.ApplyTexture` (prompt/overlay
  doesn't tell the model that a `update_coagent` continuation requires it).
- The cell *does* stage it but the receipt marker `rlm:texture_apply:` isn't
  emitted on this path (e.g. authorizer commits but output string differs).
- The occurrence should not have been created for a desk lacking the write
  affordance (e.g. a continuation routed to Texture whose work item doesn't
  carry `lifecycle_work_item_id`, so the wrong branch of `initialTextureToolChoice`/
  required-write wiring applies).

## Fix direction (not yet decided)

- Cap required-write retries per occurrence and terminally dispose the
  occurrence with a recorded failure (dead-letter) rather than redelivering
  forever.
- Give the defer path exponential backoff keyed to occurrence identity.
- Verify `rlm:texture_apply:` is actually emitted on the desk-cell path and
  the model is instructed to stage `choir.ApplyTexture` for update
  continuations.

## Residual question for the trace drill-down

Whether the stuck engineering op is starved *by* this loop (resource
contention) or has its own defect — the next probe should run against a VM
without a live Texture continuation to isolate.
