# Effects trusted-outbox dispatch — irreversible-email-v1 actuator (no live send)

**Boundary:** execute (route map 7). Not rehearsal. Not a live send. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `ede00140` (irreversible-email-v1 policy-bytes ACCEPT)
**Mutation class:** red (new trusted-outbox actuator + email policy embed; owner gates unchanged)

## What landed

`irreversible-email-v1` and `human-required-v1` are now embedded next to `reversible-selfdev-v1`. A trusted-outbox actuator records a dispatch-intent receipt **before** any provider call, uses an exact-subject idempotency key, checks revocation immediately before dispatch, and writes a consequence receipt. Live send is not armed. RecordingProvider is the only test path.

Concretely:

1. Frozen bytes keyed at `d83c2154…` and `33f5dc44…`. Reducer required-present tracking is domain-general; author recusal covers `verification` and `external_effects`.
2. `reversible-selfdev-v1` refuses an irreversible email subject (`ErrReversiblePolicyIrreversibleSubject`).
3. `irreversible-email-v1` produces a no-human-seat QualifiedConsensusReceipt at 3 counting accepts.
4. `human-required-v1` refuses when the owner-human seat is absent, and accepts when it is present (4 counting accepts).
5. Same signer in authoring and `external_effects` is refuse.
6. Outbox refuses mode `off`, unarmed live providers, reversible receipts, revoked policies. Unknown provider outcome does not green. Crash after provider-accept / before persist is recovered by `Reconcile`. Retry of the same subject does not double-send.

## What did not change

- `external-owner:` and `accept_once` remain.
- `awaiting_approval` remains.
- Mode `off` remains the default. `Outbox.Armed` defaults false.
- No mail was sent. No maild/Resend production wiring was added.
- Tape-recovery restore was not rematerialized or independently greened.
- OwnerRecovery remains inadmissible.

## Ceremony

- **Conjecture delta:** An irreversible exact email can be authorized by a stronger policy without a human seat, while a human-required policy on the same subject fails closed when that seat is absent, and dispatch is receipted rather than fire-and-forget.
- **Protected surfaces:** gateway/provider calls are not newly opened to production mail. Decision binding still requires owner or consensus. Event-chain Reduce() is unchanged.
- **Admissible evidence:** `go test ./internal/decisionpolicy ./internal/trustedoutbox` plus targeted agentcore/platform names in this receipt.
- **Rollback:** revert this commit. Owner gates and effects-OFF are unchanged; revert removes the actuator without leaving a live send path.
- **Heresy delta:** `introduced` `internal/trustedoutbox` with Armed=false. `repaired` reducer required-present so the third independence domain is not silently skipped.

## Residual

Rehearsal still unpaid. Supervision wiring still unpaid. Live send remains forbidden until rehearsal passes. Do not delete owner gates.

## Next

Route map 8 (supervision wiring), then rehearsal (route map 9). Do not send live mail. Do not rematerialize.
