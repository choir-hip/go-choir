# Effects Super Texture control join — 2026-08-16

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Parent HEAD:** `17c1fea3` (docs stamp of Texture-join refusal)
**Mutation class:** red (self-development Super start goes through Texture lifecycle control)

## Why

Live Super `c003412a` passed `requirePersistentSuperExecution` then `assign_co_super` refused missing `assignment_trajectory_id`. HTTP Super start must not forge Texture sender authorization. Texture still has no generic `update_coagent`. CoSuper still cannot open Super.

## What landed

Self-development Super start now:

1. Ensures the persistent Super agent (`LifecycleVersion==0`).
2. Starts a Texture document lifecycle and applies a `Direction=control` `execution_request` that opens Super-targeted work.
3. Cites the bound operation only as `packet.sources[].target.uri=operation:<id>` (`kind=capsule_bundle`) plus compact revision metadata `self_development_operation_id`. No operation id in revision prose. No `source_entities`. No `CoagentSourcePacketPayload` schema change.
4. Wakes persistent Super through `reconcilePersistentSuperActor` with `request_source=lifecycle_texture_control` and `assignment_trajectory_id`. Super stays non-lifecycle (`TrajectoryID=""`).
5. Does not stamp Texture's caller run onto Super `requested_by_run_id`, so `ListRunsBySelfDevelopmentOperation` still treats Super as the root actor.

Agentcore now parses the same joinable URI keys Texture already did (`operation`, `receipt`, `event_head`).

This is not a freeze. This is not qualified-consensus CAS. OwnerRecovery `663540be` remains inadmissible for promotion.

## Tests

`go test ./internal/agentcore -count=1 -timeout 240s -run 'TestSelfDevelopment|TestConcurrentExactRetries|TestSurvivorContract|TestTextureProductionRegistryOmitsGenericUpdateCoagent|TestLifecycleControl|TestReconcilePersistentSuper|TestUpdateCoagentCutover|TestAssignCoSuper'`

## Unchanged

Do not treat Texture join as freeze. Do not add Texture `update_coagent`. Do not let CoSuper open Super. Do not forge Texture sender auth from HTTP Super start. Genesis 409. Armed=false. No live send. Constructed freeze remains `7122f279` until proven on the next deploy.
