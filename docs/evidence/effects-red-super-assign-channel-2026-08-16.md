# Effects Super Texture join live, CoSuper assignment refused Super channel — 2026-08-16

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `7fed24fe4a394a3ea0acf469e630a5422589a06a` (`deployed_at` 2026-08-16T22:09:41Z)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31973948466 succeeded, including Node B (failed-job rerun after unrelated `SQLITE_BUSY` flake in `TestAdapterSQLitePreBindResearcherRecoveryBindsAndExecutesWithoutSnapshot`).
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **279**

## Live observation

G4 preserved constructed computers (`code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`). Freeze remains `7122f279`.

Owner-scoped refresh `effects-texture-join-refresh-2026-08-16T22:12Z` moved epoch **278 → 279**. LifecycleReceipt `01a00ca2-1958-7e45-8306-6b4058bb0376`.

Same operations POST (`effects-solitaire-start-2026-08-16T20:08Z`) returned 200. Operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` stayed `executing` with empty bundle. Mode stayed `propose_only` generation 1. Genesis 409. No mail.

New Super `d45d4eb1-69cd-4237-bcb9-13aea6d81d14` started with:

- empty `TrajectoryID`
- `request_source=lifecycle_texture_control`
- `assignment_trajectory_id=e826402d-b666-503f-93ba-72b3bcf51e8d`
- empty `requested_by_run_id`
- `self_development_operation_id=selfdev-b090bcd7…`

Texture join is paid. Super then completed blocked:

> `co-super assignment: exact non-lifecycle persistent Super unavailable: co-super assignment invalid transition`

`requireCoSuperParentAuthority` requires Super **agent** `ChannelID == AgentID`. Texture-woken Super create copied the Texture document onto Super run channel and `UpsertAgent` overwrote the persistent Super agent channel. `report_to_texture` still needs the run channel; the agent channel must stay `super:<owner>`.

Terminal Super `c003412a` remains unbound. This is not a freeze.

## What landed in source after that observation

After Texture control wakes Super, `EnsurePersistentSuperAgent` restores the persistent Super agent (`ChannelID=AgentID`, `LifecycleVersion==0`) before binding the operation. The Super run may keep the Texture document channel for reports.

## Tests

`go test ./internal/agentcore -count=1 -timeout 180s -run 'TestSelfDevelopment|TestConcurrentExactRetries|TestSurvivorContract_CoSuperExecutionRequestDoesNotOpenPersistentSuper|TestPersistentSuperReportToTexture|TestPersistentSuperReportRequiresComplete'`
