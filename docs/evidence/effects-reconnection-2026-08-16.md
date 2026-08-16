# Effects reconnection — assigned CoSuper `update_coagent` with sender-authorized Super executability

**Boundary:** execute (route map 4). Not freeze/propose. Not decision-policy reducer. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `dfcb8ad8` (reversible-selfdev-v1 policy-bytes ACCEPT)
**Mutation class:** red (actor isolation / Super privilege / CoSuper assigned registry)

## What landed

Assigned CoSuper regained `update_coagent` as the Super report channel. Persistent Super executability moved from model-written `packet.kind` to sender authorization. The survivor contract was replaced by a stronger assertion, not deleted.

Concretely:

1. Assigned CoSuper closed registry is capsule-local tools plus `update_coagent`. Static CoSuper registry still has no `update_coagent`. Freeze/propose tools remain absent (route map 5).
2. CoSuper message policy allows Super only. Unassigned CoSuper still cannot use the legacy Super path.
3. Assigned CoSuper → persistent Super reports are `Direction=producer_report`, routed through the worker mailbox even when a lifecycle trajectory exists. Packet kind describes content and does not open Super execution.
4. Persistent Super executes only Texture `Direction=control` `execution_request` packets. CoSuper `execution_request`, Texture packets without control direction, and other roles do not open Super.
5. Assigned CoSuper producer reports (`evidence_update`, `execution_result`, `blocker`, `question`, `proposal`, `decision_request`) remain in the mailbox for a mailbox-activated Super to inject. Unsigned CoSuper packets without `Direction=producer_report` are still settled as non-executable.

## What did not change

- `external-owner:` remains the only accepted decision authority (`internal/agentcore/self_development_decision_binding.go`).
- `accept_once` remains (`internal/platform/self_development_modes.go`).
- Effects remain OFF. No actuator was armed.
- Freeze/propose still have no assigned-CoSuper production call site.
- Tape-recovery restore substrate was not rematerialized or independently greened.
- OwnerRecovery checkpoints remain inadmissible for promotion.

## Ceremony

- **Conjecture delta:** CoSuper can report upward as an actor without being able to open privileged Super execution, because executability is sender authorization rather than packet kind.
- **Protected surfaces:** none of the Definition protected surfaces were mutated (decision binding, event chain, materializer, checkpoint/route, Texture canonical writes, updater, vmctl, auth/session, gateway, deploy). Super privilege and the CoSuper assigned registry are the reconnection surface named by `actor-isolation-stopgap-2026-08-11`.
- **Admissible evidence:** targeted Go tests in `internal/agentcore`, `internal/agentprofile`, and `internal/store` (sender-authorization table, CoSuper execution_request does not open Super, producer report does not open Super and is retained, assigned overlay includes `update_coagent`, freeze/propose remain absent, mailbox exception allows assigned CoSuper Super reports when a lifecycle trajectory exists, unsigned CoSuper packets still refuse).
- **Rollback:** revert this commit. Owner gates and effects-OFF are unchanged, so revert restores the stopgap (no CoSuper `update_coagent`) without leaving an armed effect path.
- **Heresy delta:** `repaired` `actor-isolation-stopgap-2026-08-11` (privilege relocated to sender, not deleted). `introduced` worker-mailbox exception for assigned CoSuper → persistent Super producer reports so a lifecycle trajectory cannot silence the restored channel.

## Residual

Lifecycle-control Super (`request_source=lifecycle_texture_control`) still consumes only lifecycle packets bound to that run. Mailbox CoSuper reports inject into mailbox-activated Super (`request_source=update_coagent`). The RLM rebase is allowed to rewrite this layer without weakening sender authorization.

## Next

Route map 5: wire freeze/propose onto assigned CoSuper under its capsule binding. Do not delete `external-owner:` first.
