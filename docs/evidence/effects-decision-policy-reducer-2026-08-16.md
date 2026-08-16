# Effects decision-policy reducer — QualifiedConsensusReceipt lands with owner gates still present

**Boundary:** execute (route map 6). Not irreversible email. Not rehearsal. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `fb89c245` (freeze/propose wiring)
**Mutation class:** red (decision binding, mode CAS, new consensus reducer)

## What landed

Schema + typed receipt + reducer landed together. `external-owner:` is no longer the *only* accepted decision authority. It is still an accepted authority. `accept_once` and `awaiting_approval` remain.

Concretely:

1. `internal/decisionpolicy` is the consensus-reduction stage (non-event). It consumes a frozen `DecisionPolicy`, `SeatManifest`, `EffectSubject`, `PolicySelectionReceipt`, and `BallotAttestation`s and produces a `QualifiedConsensusReceipt` whose `receipt_digest` is SHA-256 of canonical bytes excluding itself.
2. Frozen `reversible-selfdev-v1` bytes are embedded and keyed at digest `c34ddf073aecaacc307f375d6f2e398798350d7a48c8d3c2e7c6d10248b394d7`.
3. New mode `qualified_consensus` binds the same exact operation/bundle/heads/pending-transition/commitments `accept_once` binds, plus `policy_digest` and `consensus_receipt_digest`, with a future canonical UTC expiry. Extra consensus fields are stored in the ModeReceipt (no ALTER).
4. `verifyFinalizedSelfDevelopmentDecision` accepts either `external-owner:` (exactly one mode-receipt artifact) or `qualified-consensus:<receipt_digest>` joined to a second input artifact whose digest equals the receipt digest. A decision with neither is refused.
5. The decide API, when the current mode is `qualified_consensus`, re-runs consensus reduction, consumes the mode to `propose_only`, and appends `effect_accepted` with consensus authority and the receipt preimage as an event payload.

## Refuse matrix covered in tests

No policy / revoked; missing selection; ballot not joined; missing required seat; below quorum (abstention); unresolved policy-blocking dissent; silently recused seat; independence fabricated (author signer in verification); reversible policy + irreversible subject; OwnerRecovery subject; ballot without attestation; selection sequence 0; `human-required` + human absent; `accept_once` refuses consensus bindings; neither-owner-nor-consensus authority refused.

## What did not change

- `external-owner:` prefix check remains.
- `accept_once` remains with its exact bindings.
- `awaiting_approval` remains the legal pre-decision state.
- Mode `off` remains the default. This does not arm effects.
- Irreversible email is unpaid (`irreversible-email-v1` bytes not frozen).
- Tape-recovery restore was not rematerialized or independently greened.
- OwnerRecovery checkpoints remain inadmissible.

## Ceremony

- **Conjecture delta:** A reversible self-development effect can be authorized by a policy-bound QualifiedConsensusReceipt without a per-candidate human, while the fail-closed owner gate still works for the historical path.
- **Protected surfaces:** decision binding now has a second accepted authority. Event-chain Reduce() is unchanged. Owner mode CAS still defaults to off. Materializer, checkpoint/route, Texture, updater, vmctl, auth, gateway, deploy were not newly opened.
- **Admissible evidence:** `go test ./internal/decisionpolicy ./internal/platform ./internal/agentcore` with the Reduce/mode/binding names in this receipt.
- **Rollback:** revert this commit. Owner gates are still in the same commit, so revert removes the consensus path without leaving an ungoverned accept.
- **Heresy delta:** `repaired` "external-owner is the only accepted authority" by adding the consensus path rather than deleting the owner check. `introduced` `qualified_consensus` mode and a process-local DecisionPolicy store. Forbidden intermediate (accept with neither owner nor consensus) is tested.

## Residual

Deployed acceptance of the consensus path has not happened. Until it does, do not delete `external-owner:` / `accept_once` / `awaiting_approval`. Rehearsal and irreversible email remain unpaid. Effects remain OFF.

## Next

Irreversible-email-v1 policy bytes (define) then trusted-outbox dispatch (route map 7), or supervision wiring (route map 8), then rehearsal. Never by deleting the owner gates first.
