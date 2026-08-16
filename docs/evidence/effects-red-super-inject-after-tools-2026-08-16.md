# Effects Super Texture join live, then inject-after-tools failed — 2026-08-16

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `134b60dd3103d53c954600ed1cd36c18e73cc4ea` (`deployed_at` 2026-08-16T22:42:59Z)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31976060235 succeeded, including Node B.
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **280**

## Live observation

G4 preserved constructed computers (`code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`). Freeze remains `7122f279`.

Owner-scoped refresh `effects-super-agent-channel-refresh-2026-08-16T22:48Z` moved epoch **279 → 280**. LifecycleReceipt `01a00cc3-b325-754e-847c-44a7b5f832ae`.

Same operations POST (`effects-solitaire-start-2026-08-16T20:08Z`) returned 200. Operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` stayed `executing` with empty bundle. Mode stayed `propose_only` generation 1. Genesis 409. No mail.

New Super `dc66c265-d1c9-4162-9113-f21095c42be6` started running with:

- empty `TrajectoryID`
- `request_source=lifecycle_texture_control`
- `assignment_trajectory_id=e826402d-b666-503f-93ba-72b3bcf51e8d`
- empty `requested_by_run_id`
- `self_development_operation_id=selfdev-b090bcd7…`
- persistent Super agent ID `super:5bd6de97-3b58-408c-bf89-c42c81b083de`
- run channel Texture document `0744fc0c-eaa6-5e22-acd5-dcb9cb68fb93`

Texture join remains paid. Super then **failed** (not blocked) at 2026-08-16T22:49:45Z:

> `tool loop inject turns after tools: list pending update_coagent turns: record not found`

Initial mailbox inject succeeded (the model ran and called tools). Re-listing delivered Texture controls after the tool turn used `GetAgentByScope` / `GetRunByOwner` and 404'd, which killed the Super run. Terminal Super `d45d4eb1` remains unbound. Texture supervision document was not revised after this failure. This is not a freeze.

## What landed in source after that observation

Texture-control persistent Super restores its non-lifecycle agent before re-listing delivered controls. `ListLifecycleControlsDeliveredToRunPage` falls back to owner-wide `GetRun` when the owner-scoped canonical lookup is `ErrNotFound`. A later inject `ErrNotFound` returns empty pending packets instead of failing the Super run.

## Tests

`go test ./internal/agentcore -count=1 -timeout 180s -run 'TestSelfDevelopment|TestSurvivorContract_CoSuperExecutionRequestDoesNotOpenPersistentSuper|TestPersistentSuperReportToTexture|TestPersistentSuperReportRequiresComplete'`

`go test ./internal/store -count=1 -timeout 180s -run 'TestBindLifecycle|TestListLifecycleControls|TestPersistentSuper'`
