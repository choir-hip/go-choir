# M0 Residual — Texture desk passivated by runtime restart, never re-woken

**Date:** 2026-09-30 · **Station:** M0 (debug/stabilize) · **Build:** `4c279162` · **Class:** red-class residual evidence

## What the QA repro observed

Prompt-bar submit → conductor opened texture → texture activation `654acaec`
created and **passivated** with `passivated_reason: runtime_restarted`,
zero tokens, no result, no v1 revision. The trajectory (15590d6c) never
produced a desk-authored revision. Doc `485859ad` holds only the owner's

## Root-cause (confirmed against source 2026-09-30)

`agentcore.Runtime.passivateInterruptedActivations` marks the run
`passivated_reason=runtime_restarted` + `MarkAgentMutationStale`, then the
actor-wake outbox projector (`sweepActorWakeOutbox`) is the sole
post-restart delivery path. But the wake that fired the killed activation
was already consumed, and `sweepActorWakeOutbox` hits
`store.ErrNoPendingActorOccurrence` → `MarkActorWakeProjected` → the row is
disposed as a dead wake. **No outbox row is re-minted for the still-open
`owner_revision` obligation**, so the texture desk never re-runs it. The
`textureowner.Handler.Start` reconcile *does* re-dispatch a `coagent_result`
for passivated texture authority — but it runs only at process boot, not on
the in-process `Runtime.Start` restart path that `passivateInterruptedActivations`
serves. Gap = a runtime restart that isn't a full VM boot leaves the
obligation armed-but-undelivered.

seed revision (v0).

## Defect class

A desk activated for an owner revision is parked by a runtime restart
(`runtime_restarted`) and **no wake re-arms it** after the runtime
recovers. This is a desk-restart wake gap adjacent to the signal-plane
hole M-SUB closes: the activation exists, the obligation is live, but the
post-restart continuation never fires.

Contrast with the pre-M0 hypothesis (stale build / wake-debt): on
`4c279162` the wake debt is clean (`desk_pending_mutations` absent → 0,
all three stale runs reconciled to `passivated`), yet the desk still
stalls — so the residual is a **runtime-restart → missing desk re-wake**
edge, not the pending-mutation wedge.

## Evidence

- `docs/evidence/m0-qa-baseline-timings-2026-09-30.json` — leg table
  (probe timed out after 15min; the `texture:failed` and engineering
  `cancelled` legs are pre-refresh historical runs swept by an over-broad
  run filter — probe bug, not this trajectory's events).
- Run `654acaec-c0cd-4186-91fd-ef19585b5003`: texture, state=passivated,
  `passivated_reason=runtime_restarted`, no `input_tokens`, no result,
  no trajectory binding.
- Doc revisions: only `12db5d82` (owner v0); no desk-authored head.
- Guest health post-refresh: build `4c279162`, `running_runs=0`,
  `selfdev_active_operations=8`, pending_mutations reconciled.

## Repair (landed 2026-09-30)

The upstream gate was subtler than "reconcile doesn't run on restart": a
consumed head (`texture_turn_committed` before the kill) makes
`ownerHeadPending` false, so the only remaining arm is `initialWorkWake` —
the open desk work item. `reconcileAgentWakeLocked` armed it but then
**suppressed it on any texture revision run for the doc, regardless of
state** (`texture_controller.go` ~line 797). The passivated
`runtime_restarted` run counted as live authority, zeroed `initialWorkWake`,
and reconcile early-returned before `reactivatePassivatedTextureRun`. The
interrupted run suppressed its own recovery.

Fix: the suppression now requires `runs[i].State.Active()` — a passivated or
terminal run is a dead authority, so the open-work wake stays armed and
`reactivatePassivatedTextureRun` resumes the stale_activation mutation.

Regression coverage:
`TestTextureOwnerStartReactivatesPassivatedRunOnOpenWork` (fails pre-fix,
`state=passivated`; passes post-fix) and
`TestTextureOwnerStartReactivatesRuntimeRestartedPassivatedRun` exercise the
real lifecycle seed (`ReplaceLifecycleActivation` → `UpdateRun` passivate
→ `MarkAgentMutationStale`).

Residual M-SUB follow-up (unchanged): the fix covers the open-work re-wake
case; a desk activated by a consumed owner revision with no open work item
still relies on `armedUpdates`/`ownerHeadPending` or a re-minted outbox
obligation.

## Probe fix queued

`scripts/m0_qa_probe.mjs` over-collected historical terminal runs —
scope `seenRuns` to `created_at >= submit` time so the baseline only
carries this trajectory's legs.
