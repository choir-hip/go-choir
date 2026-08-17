# Effects CoSuper terminalized after terminal-unbound skip fix — 2026-08-17

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` reports `cec68e23afda3b2bc2554384eeb9f87e160faf5f`, deployed `2026-08-17T19:29:19Z`.
**CI:** `https://github.com/choir-hip/go-choir/actions/runs/32058303879` succeeded.

## Root cause (confirmed via guest observability)

`/api/runtime/observability` showed the guest was running the deployed commit and that boot reconcile ran but aborted with `co-super assignment invalid transition` on the first assignment in trajectory `e826402d`. That first assignment (`assignment-09bf3654`) is terminal (`cancelled`) with capsule `unbound` and an empty `BoundRunID` (its capsule spawn failed before bind). `SetCoSuperCapsuleDisposition` rejects `BoundRunID == "" && Disposition != Open`, so `ReconcileCoSuperAssignmentsForTrajectory`'s terminal-cleanup branch failed and returned, aborting the whole trajectory before the still-live bound assignment was reached.

## Fix

`cec68e23` skips `CapsuleDisposition == Unbound` in the terminal-cleanup branch (an unbound capsule was never bound, so there is nothing to revoke). Regression test `TestReconcileSkipsTerminalUnboundCapsule` reproduces a terminal-unbound assignment plus a bound assignment in one trajectory and asserts the bound assignment is reconciled.

## Verified live

Owner-scoped refresh `effects-terminal-unbound-fix-refresh-2026-08-17T19:30Z` moved epoch **289 → 290** (LifecycleReceipt `01a01133-f454-70a3-844e-4f4674b877a3`). After boot:

- `run:assignment-fa38b037-bd9d-5270-a640-e668afa4eb57` → `cancelled`, finished `2026-08-17T19:30:12.293Z`, error `restart revoked absent assignment capsule`.
- `assignment-fa38b037` → `cancelled`, capsule `revoked`, lifecycle_version 5.
- `assignment-31b710eb` (previously stuck `open`) → `cancelled`, capsule `revoked`, lifecycle_version 4.
- Terminal-unbound assignments `09bf3654`, `248209f1`, `b8a4a206` → unchanged (correctly skipped).
- Guest observability log: `runtime: boot assigned CoSuper run run:assignment-fa38b037 already terminal after reconcile`.

The constructed freeze `7122f279` is unchanged. Mode `propose_only` generation 1. No mail. Operation `selfdev-b090bcd7` still `executing`. This is not a freeze.
