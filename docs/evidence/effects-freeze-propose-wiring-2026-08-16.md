# Effects freeze/propose wiring — assigned CoSuper capsule-bound call sites

**Boundary:** execute (route map 5). Not decision-policy reducer. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `b7b9f0b1` (reconnection)
**Mutation class:** red (assigned CoSuper freeze/inspect/verify call sites; updater incoming is written only through existing capsule-bound freeze)

## What landed

`commit_transaction`, `inspect_self_development_bundle`, and `record_self_development_verification` now have a production call site: the assigned CoSuper closed registry, under the authenticated capsule binding.

Concretely:

1. Assigned CoSuper registry composes capsule-local tools, `update_coagent`, and the capsule-bound freeze/inspect/verify installer. Static CoSuper still has none of these.
2. Assigned CoSuper `CapsuleToolCtx` now carries freeze authority (`TransactionBuilder`, `EventAppender`, `OperationStore`, `EventProjection`, `UpdaterRoot`) plus the existing obligation validator. Host file, spawn, materialize, checkpoint, route, VM, and owner-decision tools remain absent.
3. Tool-internal gates are unchanged: freeze still requires `requireCapsuleMutationRole`; inspect/record still require the verifier slot. `propose_effect` / `finalize_effect` remain unwired.
4. Overlay prompt names the freeze tools and no longer claims freeze/propose are absent.

## What did not change

- `external-owner:` and `accept_once` remain the decision gates.
- Effects remain OFF. No actuator was armed. No QualifiedConsensusReceipt reducer landed.
- Reconnection privilege property is unchanged: CoSuper packets still cannot open Super.
- Tape-recovery restore was not rematerialized or independently greened.

## Ceremony

- **Conjecture delta:** A live assigned CoSuper can freeze and verify its own capsule candidate because freeze/inspect/verify are worker tools on the capsule binding, not a supervisory or owner gate.
- **Protected surfaces:** updater incoming is written only by the existing `commit_transaction` implementation, now reachable from assigned CoSuper. Decision binding, event-chain append authority beyond that tool, materialize, checkpoint/route, Texture writes, vmctl, auth, gateway, and deploy were not newly opened.
- **Admissible evidence:** targeted Go tests (`TestAssignedCoSuperBuilderIsExactClosedSet`, overlay exact-set, prompt names freeze tools, `TestAssignedCoSuperCapsuleToolCtxCarriesFreezeAuthority`, disjoint installer test, static CoSuper still forbids freeze tools).
- **Rollback:** revert this commit. Owner gates and effects-OFF are unchanged.
- **Heresy delta:** `repaired` the "tools exist but have no production call site" gap named in this Definition start.observed_artifact. `introduced` freeze-authority fields on assigned CoSuper capsule context; they fail closed when store/updater root/operations are absent.

## Next

Route map 6: decision-policy reducer atomically with `QualifiedConsensusReceipt`. Never by deleting `external-owner:` first.
