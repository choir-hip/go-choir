# Effects Super 2210c654 broker readiness timed out — 2026-08-17

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `6fafd2f9effd677015ae3b447d4a83d0a4a9c05d` (`deployed_at` 2026-08-17T01:32:10Z)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31983808820 succeeded, including Node B.
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **284**

## Live observation

G4 preserved constructed computer `candidate-fleet-e15cb89f25d963c220319b7b` (`code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`). Freeze remains `7122f279`.

Owner-scoped refresh `effects-capsule-timeout-refresh-2026-08-17T01:33Z` moved epoch **283 → 284**. LifecycleReceipt `01a00d5a-5053-79d0-9b87-4aeb7b0fd17f`. Hung Super `8c6b660d` became terminal (`failed`: gateway 400 after refresh). Same operations POST returned **200**.

New Super `2210c654-9c09-4f15-b3d9-8c1fb426dd7f` started with:

- empty `TrajectoryID`
- `request_source=lifecycle_texture_control`
- `assignment_trajectory_id=e826402d-b666-503f-93ba-72b3bcf51e8d`
- `self_development_operation_id=selfdev-b090bcd7…`
- persistent Super agent ID `super:5bd6de97-3b58-408c-bf89-c42c81b083de`
- Texture control binding to work item `38b96770-5fb8-585a-8234-db9e4dfbd331`

Texture join remains paid. Super then **completed blocked** at 2026-08-17T01:36:36Z:

> `spawn assigned capsule after durable open: capsule broker readiness timed out`

Broker bind-mount and launcher `Start` returned. Isolated broker never became ready on the inherited Unix listener within 10s. Super also reported a later `lifecycle command digest conflict` on retry and `report_to_texture` `record not found`. No CoSuper. No bundle. Mode stayed `propose_only` generation 1. Genesis 409. No mail. Operation `selfdev-b090bcd7` stayed `executing`. This is not a freeze.

The timeout land did what it claimed: Super no longer stays `running` forever after hosts write.

## What landed in source after that observation

Capsule overlay lower is guest `/`. Guest `/proc` and `/sys` were leaking through the merged root. `prepareCapsuleRoot` now tmpfs-masks `proc` and `sys` like `run`/`tmp`/`mnt`, and bind-mounts `/dev/pts`, so the isolated broker can mount a fresh procfs instead of following guest proc. Readiness timeout now wraps the last Connect/Stat error.

## Tests

`go test ./internal/capsule -count=1 -timeout 60s`

`GOOS=linux CGO_ENABLED=0 go test -c ./internal/capsule`
