# S2: a refused apply wedges the pending transition — every later offer permanently stale

**Date observed**: 2026-10-05 (S2 station acceptance probe, negative legs).
**Computer**: `computer-6450a253b8b6ebc0866471973694f5be` (staging disposable).
**Mutation class**: red (canonical event discharge path).
**Status**: fix landed (`internal/agentcore/platform_update.go` + regression
test); live wedge on the disposable discharged manually.

## Symptom

One base-mismatched signed offer reached the guest, committed
`effect_accepted` + `materialization_started`, then the updater refused
before mutation (`updater refused request`). Result:

- `pending_transition_ref` stayed pinned to that accepted event;
- every subsequent signed offer (any content) refused
  `platform update: base event head is stale`;
- re-pushing the *same* offer resumes the pending branch and refuses again
  identically — the wedge is self-healing-proof;
- `materialization_failed` was never committed: it only ran when
  `result.RecoveryReceipt` verified — i.e. only for post-mutation
  health-failure restores, never for pre-mutation refusals.

Live receipt: `s2-acceptance-20261005T024234Z.jsonl` on Node B —
`neg-base-digest` refused `updater refused request`, head advanced
`f3fa6f1b→7eadac15` (accepted + started committed, no failed), and every
later push answered `base event head is stale`.

## Root cause

`ApplyPlatformUpdate` (`internal/agentcore/platform_update.go`) commits the
accepted event **before** calling the updater — correct canonical binding —
but the apply-error branch committed `materialization_failed` only when a
`RecoveryReceipt` verified. `updater.Apply` returns `ApplyResult{}, err`
(empty result, `Outcome == ""`) for every pre-mutation refusal
(base-digest mismatch, base_commit mismatch, schema window, provenance,
realization fence, request-commitment mismatch, missing closure.nar). No
recovery receipt exists because there is nothing to recover — yet the reducer
requires `materialization_failed{DecisionRef: pending, RestoredPriorEffective:
true}` to clear `PendingTransitionRef` (`computerevent/reducer.go:103`).

So every deterministic refusal left the transition open forever: the chain
moves to a head whose desired state names a release that was refused, and the
resume machinery can only ever re-drive the same refused offer.

## Fix

`platform_update.go`: on `applyErr` with `result.Outcome == ""` (no
mutation happened — the prior effective state is trivially preserved),
commit `materialization_failed` exactly as the restored path does. A mutated
then *unrestored* failure keeps the wedge — refusing new updates while the
effective state is unknown is fail-closed, not a defect.

Regression: `TestPlatformUpdateRefusalDischargesPendingTransition`
(`platform_update_test.go`) — a validly-signed offer carrying a
`closure_digest` that cannot resolve is refused, the failed event commits,
pending clears, and the next offer applies.

## Staging repair (manual discharge)

The live wedge cannot clear itself: the resumed apply hits the same refusal,
and `recordPlatformUpdateFailed` on the deployed binary lacks the new branch.
Repair = append the missing `materialization_failed` event via the corpusd
event CAS under a capability minted by the platform signing key, then cold
boot the VM so the embedded projection replays to platform head
(`ErrNeedsProjectionRepair` is expected until replay). Receipt:
`docs/evidence/s2-wedge-discharge-2026-10-05.json`.

## Residual

- Errors between the accepted commit and `updater.Apply` (payload staging,
  ref fetch, commitment compute) still leave the transition pending — kept
  resumable deliberately (re-push of the same offer retries). A transient
  failure there should be retryable; a deterministic one needs the same
  discharge treatment if it recurs. Watch for a second occurrence before
  widening the discharge condition.
- The `neg-*` legs before this fix were contaminated: early runs mint-refused
  on probe defects (malformed request; `short canonical expiry` — a probe-
  side expiry bug since corrected), and the five pushed after the wedge all
  answered `base event head is stale` — the wedge itself, not the intended
  fence. Only `neg-base-digest` produced a genuine pre-mutation refusal.
  Re-run all six after repair.
