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

## Follow-up 2026-10-05 ~11:40: outcome split between pre- and post-journal gates

The agent-side discharge fix above handled daemon refusals that return an
empty outcome. A later schema-window leg proved a second shape: the daemon
returned a non-empty outcome (set, no recovery receipt) for a refusal the
agent-side then treated as mutated-then-unrestored and kept the wedge. Fix in
flight: the daemon journals every pre-mutation refusal as a terminal
`refused` outcome (base/shape/commitment/idempotency fence, base join,
state-compat, source trust, stage, closure replay), gated BEFORE any
mutation; the agent-side discharges on `refused` exactly like the empty
case. Regression: daemon unit test asserts `Outcome == "refused"` plus
replay-identical refusal on the same idempotency key.

## Root cause (original)

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

## Second occurrence (2026-10-05 ~19:47Z) — fix deployed, wedge still happened

The `neg absent-entrypoint` leg on computer-6450a253 refused
`updater refused apply: updater refused request` and the pending
transition wedged at `86b98028e712` — *with* `fae12950` present in the
deployed daemon (verified `fae12950` ⊂ deployed `1e2e2895`). So the
in-band discharge path (`platform_update.go:273-282` →
`recordPlatformUpdateFailed` → `materialization_failed` append) exists
but can itself fail: the only un-journaled failure left is the
`AppendNewPayload` for the failed event racing guest replay/CAS state.
Manual `s2_discharge_pending_transition.go` cleared it again (head
`c0b84209` → `0661e341`, `discharged_update: s2neg-absent-entrypoint
-20261005T194732Z`). Per the "watch for a second occurrence" instruction:
it recurred — the discharge condition needs widening so a failed
`materialization_failed` append retries rather than leaving the pending
bound permanently.
