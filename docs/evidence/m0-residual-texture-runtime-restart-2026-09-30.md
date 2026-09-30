# M0 Residual — Texture desk passivated by runtime restart, never re-woken

**Date:** 2026-09-30 · **Station:** M0 (debug/stabilize) · **Build:** `4c279162` · **Class:** red-class residual evidence

## What the QA repro observed

Prompt-bar submit → conductor opened texture → texture activation `654acaec`
created and **passivated** with `passivated_reason: runtime_restarted`,
zero tokens, no result, no v1 revision. The trajectory (15590d6c) never
produced a desk-authored revision. Doc `485859ad` holds only the owner's
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

## What M-SUB must add

M-SUB's stall terminator (`cell_fate` + armed deadline) covers the
failed-cell case; this residual is the **passivated-not-resumed** case.
M-SUB should extend `cell_fate`/wake coverage to `runtime_restarted`
passivation — a restarted desk must either resume its pending activation
or record a fate so the trajectory doesn't hang silently. Alternatively
a desk-watchdog re-arms a passivated activation whose obligation is still
open (check whether `engineeringAssignmentOpenMu`/desk reconcile kick
already covers texture on restart — the run suggests it does not for
`runtime_restarted`).

## Probe fix queued

`scripts/m0_qa_probe.mjs` over-collected historical terminal runs —
scope `seenRuns` to `created_at >= submit` time so the baseline only
carries this trajectory's legs.
