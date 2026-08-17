# Effects CoSuper still running after boot-repair deploy — 2026-08-17

**Boundary:** execute. Problem documentation first. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` reports `beb32aeb265b39a9e0e089450b5239219beedd6d`, deployed `2026-08-17T04:42:10Z`.
**CI:** `https://github.com/choir-hip/go-choir/actions/runs/31993817931` succeeded.
**Computer:** `computer-03335285269bdba4f94377e56879f9e6`

## Live observation

Owner-scoped refresh `effects-cosuper-reconcile-refresh-2026-08-17T04:45Z` completed at `2026-08-17T04:46:22.033385Z` with LifecycleReceipt `01a00e0a-c551-75fb-8bc1-7999022df864`, advancing the retained computer from realization epoch **286 → 287**. Lifecycle status remained `active`.

After the refresh and an 8-second boot-settle interval, GET `/api/runs/run:assignment-fa38b037-bd9d-5270-a640-e668afa4eb57` returned HTTP 200 with:

- state `running`
- `created_at` `2026-08-17T02:05:08.767708199Z`
- `updated_at` still `2026-08-17T02:05:08.829230827Z`
- `finished_at` null
- error null
- empty result and no token counts

The deployed `beb32aeb` boot-reconcile repair therefore did not make this historical CoSuper terminal. Super `f8ee744f` remains `completed`. Operation `selfdev-b090bcd72d300fed17cb3f5a142f8595` remains `executing` with null bundle/checkpoint. Mode remains `propose_only`, generation 1. Genesis remains 409. No mail. This is not a freeze.

The constructed G4 freeze remains recorded as `7122f279`; the refresh is owner-scoped and does not promote or rewrite that constructed identity.

## Boundary

Do not retry the same operations POST while this CoSuper run is live. Do not cancel it solely for silence. Do not self-promote, CAS `qualified_consensus`, send mail, or use OwnerRecovery `663540be` for promotion.

## Next

Inspect the deployed boot path with this exact durable state: determine whether assigned-CoSuper rewarm is skipped before `ReconcileCoSuperAssignmentsForTrajectory`, reconciliation returns without changing the bound run, or cancellation fails to project terminal state. Document the next finding before a runtime fix.
