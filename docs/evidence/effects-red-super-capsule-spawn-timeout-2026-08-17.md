# Effects Super 8c6b660d hung after capsule hosts write — 2026-08-17

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `34575a8a1d8d894ecf4373f2334f1ccb78116672` (`deployed_at` 2026-08-17T00:21:36Z)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31980781341 succeeded, including Node B.
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **283**

## Live observation

G4 preserved constructed computer `candidate-fleet-e15cb89f25d963c220319b7b` (`code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`). Freeze remains `7122f279`.

Owner-scoped refresh `effects-capsule-hosts-refresh-2026-08-17T00:38Z` moved epoch **282 → 283**. LifecycleReceipt `01a00d28-884d-76ca-a117-9f1d0b773e5c`.

Same operations POST (`effects-solitaire-start-2026-08-16T20:08Z`) returned **200**. New Super `8c6b660d-9763-4b10-b160-51c2af90134e` started running with:

- empty `TrajectoryID`
- `request_source=lifecycle_texture_control`
- `assignment_trajectory_id=e826402d-b666-503f-93ba-72b3bcf51e8d`
- empty `requested_by_run_id`
- `self_development_operation_id=selfdev-b090bcd7…`
- persistent Super agent ID `super:5bd6de97-3b58-408c-bf89-c42c81b083de`
- Texture control binding to work item `38b96770-5fb8-585a-8234-db9e4dfbd331`

Texture join remains paid. Super stayed **running** with unchanged `updated_at` `2026-08-17T00:39:31Z` for more than 20 minutes. No CoSuper run appeared. Texture supervision stayed at revision 1. No bundle. Mode stayed `propose_only` generation 1. Genesis 409. No mail. Operation `selfdev-b090bcd7` stayed `executing`. This is not a freeze.

The previous Super on `d3819c3b` completed in 78 seconds by failing at merged `/etc/hosts` EROFS after source copy and overlay mount. This Super is the first to pass that write. Broker bind-mount and `cmd.Start()` after that write had no timeout; a hang there leaves the Super `running`, and an operations POST retry is a no-op until the Super is terminal.

## What landed in source after that observation

Broker bind-mount and launcher `Start()` wait at most 10s. Assigned CoSuper `Spawn` is bounded to 90s. A retry after refresh still requires the hung Super to be terminal.

## Tests

`go test ./internal/capsule -count=1 -timeout 60s`

`GOOS=linux CGO_ENABLED=0 go test -c ./internal/capsule`
