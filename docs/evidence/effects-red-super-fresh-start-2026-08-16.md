# Effects Super revive replayed blocked memory — 2026-08-16

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `82d87bc0a675fae30c33985666cdebbe4d63b241` (`deployed_at` 2026-08-16T20:45:29Z)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31970219783 succeeded, including Node B.
**Computer:** `computer-03335285269bdba4f94377e56879f9e6`

## Live observation

G4 preserved constructed freeze `7122f279` on deploy. Owner-scoped refresh `effects-persistent-super-refresh-2026-08-16T20:47Z` moved epoch **276 → 277**. LifecycleReceipt `01a00c54-184e-7a2e-a2ea-09dce952f96e`.

Same operations POST (`effects-solitaire-start-2026-08-16T20:08Z`) returned 200. Operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` stayed `executing` with empty bundle. Mode stayed `propose_only` generation 1. Genesis 409. No mail.

Revived Super `cdf0af4c-fc24-47e8-881c-c6e9d1e6fa0b` ran with empty `TrajectoryID`, persistent Super agent ID, and `actor_reactivate_existing_memory=true`. It completed blocked with the same `assign_co_super` authority error as the first attempt. Persisted run identity after completion still had empty trajectory, so the second completion is the blocked conversation replayed from run memory, not a fresh gate test.

This is not a freeze. Texture control/work join remains unpaid.

## What landed

An executing operation whose unique Super run is terminal and has no bundle now unbinds that run (`self_development_operation_id` removed) and starts a **new** non-lifecycle persistent Super. Same-run revive is gone so blocked memory cannot restated as a new attempt.

`requirePersistentSuperExecution` is asserted on that fresh Super. Passing the identity gate is not permission to assign CoSuper without Texture-authorized control/work, and is not freeze.

## Tests

`go test ./internal/agentcore -run 'TestConcurrentExactRetriesRepairOneRequestedOperationRun|TestSelfDevelopmentStartRevivesTerminalPersistentSuper|TestSelfDevelopmentPersistentSuperPassesAssignCoSuperGate'`
