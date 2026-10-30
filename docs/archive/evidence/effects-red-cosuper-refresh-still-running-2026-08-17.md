# Effects CoSuper still running after ed3ffa3f refresh — 2026-08-17

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `ed3ffa3f7eb290e13a510af5a1382d86d57329c2` (`deployed_at` 2026-08-17T03:39:12Z)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31990768871 succeeded after rerunning flaked `TestSelfDevelopmentTextureJoinRewakesTerminalPersistentSuper`.
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **286**

## Live observation

G4 preserved constructed computer `candidate-fleet-e15cb89f25d963c220319b7b` (`code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`). Freeze remains `7122f279`. Host deploy therefore did not recycle that guest; CoSuper `run:assignment-fa38b037` stayed `running` across the Node B deploy.

Owner-scoped refresh `effects-forcedestroy-wait-refresh-2026-08-17T03:40Z` moved epoch **285 → 286**. LifecycleReceipt `01a00dce-84a0-7486-a38d-7f38331c92b0`. Texture `3b18a6d7` became `passivated`. CoSuper `run:assignment-fa38b037` remained **running** with frozen `updated_at` `2026-08-17T02:05:08.829Z`, empty result, no error, no tokens.

Boot `ReconcileCoSuperAssignmentsForTrajectory` is supposed to revoke an absent in-process capsule and cancel the assignment. The public run projection did not become terminal. Super `f8ee744f` is still completed. Operation `selfdev-b090bcd7` is still `executing`. Mode `propose_only` generation 1. No mail. This is not a freeze.

Do **not** retry the operations POST while this CoSuper run is live.

## Already landed

`ed3ffa3f` bounds `ForceDestroy` process wait to 10s independently of caller context (`waitCapsuleProcess`). That cannot act on a G4-preserved guest until that guest actually restarts and reconcile joins assignment fate to the run projection.

## Next

Inspect why epoch-286 boot did not terminalize `run:assignment-fa38b037` (reconcile not reached vs assignment cancelled without run projection). Do not cancel solely for silence. Do not retry Super while the CoSuper run is live.
