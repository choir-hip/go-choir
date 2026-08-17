# Effects CoSuper assignment-fa38b037 outlived activation budget — 2026-08-17

**Boundary:** execute. Problem documentation first. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `dddcd80da0547ba476f4aee7d431ec70f84f44d5` (`deployed_at` 2026-08-17T02:04:07Z)
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **285**

## Live observation

Super `f8ee744f` remains `completed` after opening bound CoSuper `assignment-fa38b037` (`capsule-83337f60`).

CoSuper `run:assignment-fa38b037-bd9d-5270-a640-e668afa4eb57` stayed **running** with:

- `created_at` `2026-08-17T02:05:08.767Z`
- `updated_at` frozen at `2026-08-17T02:05:08.829Z`
- empty result, no error, no token counts
- last live GET at `2026-08-17T03:14:42Z` (69 minutes after create)

Default `ActivationBudget` is 60 minutes (`internal/provideriface/config.go`). Staging does not override `RUNTIME_ACTIVATION_BUDGET`. The run outlived that budget and did not terminalize.

Operation `selfdev-b090bcd7` stayed `executing` with no bundle. Mode stayed `propose_only` generation 1. No mail. This is not a freeze.

Do **not** retry the operations POST while this CoSuper is live. Do **not** cancel it solely for silence.

## Substrate

`ExecuteActivationSync` registers `context.AfterFunc` on the activation budget, then `terminalizeRun` for an assigned CoSuper **must** revoke/destroy the capsule before it may persist a generic run cancellation (`cancelBoundCoSuperRun` → `revokeAssignedCapsule` → `ForceDestroy`).

`ForceDestroy` waits on `caps.wait()` with no independent timeout. The progress-deadline path calls `terminalizeRun(context.Background(), ...)`, so `ctx.Done()` never unblocks that wait. `runCtx` cancel runs only after capsule fate returns. A hung broker `Wait` therefore leaves the CoSuper `running` past the budget.

## Next

Bound capsule process wait in `ForceDestroy` independently of caller context, so the activation budget can complete assigned-CoSuper fate. After that deploys, confirm G4 freeze `7122f279` and whether restart reconciliation terminalizes this assignment. Do not retry Super until the CoSuper is terminal.
