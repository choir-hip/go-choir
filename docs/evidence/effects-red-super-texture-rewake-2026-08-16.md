# Effects Super Texture rewake 409 after inject restore — 2026-08-16

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `efdc131a899d4f445fa6666ea892743cd3b3d312` (`deployed_at` 2026-08-16T23:24:38Z)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31977938737 succeeded, including Node B.
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **281**

## Live observation

G4 preserved constructed computer `candidate-fleet-e15cb89f25d963c220319b7b` (`code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`). Freeze remains `7122f279`.

Owner-scoped refresh `effects-inject-restore-refresh-2026-08-16T23:25Z` moved epoch **280 → 281**. LifecycleReceipt `01a00ce5-3292-721e-8144-4bba092db070`.

Same operations POST (`effects-solitaire-start-2026-08-16T20:08Z`) returned **409**:

> `start self-development run: Texture control did not wake persistent Super`

Operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` stayed `executing` with empty bundle. Mode stayed `propose_only` generation 1. Genesis 409. No mail.

Terminal Super `dc66c265-d1c9-4162-9113-f21095c42be6` remains failed and unbound. Texture document run `3b18a6d7-5fd4-5de4-86d2-c27954698548` stayed running. No new Super started. This is not a freeze.

The inject-after-tools restore on `efdc131a` deployed. Texture then replayed the original opener/continue command ID, so ApplyTextureTurn was a no-op and reconcile found no pending Super-executable control.

## What landed in source after that observation

Texture-authorized `ApplyTextureTurn` issues a new `turn:selfdev-texture-rewake:` command when the latest persistent Super is terminal. HTTP Super start still does not mint Texture sender authorization. An executing operation with no bound Super starts Super instead of returning 200 with an empty actor.

## Tests

`go test ./internal/agentcore -count=1 -timeout 180s -run 'TestSelfDevelopment|TestSurvivorContract_CoSuperExecutionRequestDoesNotOpenPersistentSuper|TestPersistentSuperReportToTexture|TestPersistentSuperReportRequiresComplete'`

`go test ./internal/store -count=1 -timeout 180s -run 'TestBindLifecycle|TestListLifecycleControls|TestPersistentSuper|TestGetLatest'`
