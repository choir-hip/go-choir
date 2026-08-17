# Effects Super Texture rewake paid, capsule spawn EROFS — 2026-08-16

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `d3819c3bbaeeb6bd39fecec3693948f5f7f4afc1` (`deployed_at` 2026-08-16T23:53:23Z)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31979456374 succeeded, including Node B.
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **282**

## Live observation

G4 preserved constructed computer `candidate-fleet-e15cb89f25d963c220319b7b` (`code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`). Freeze remains `7122f279`.

Owner-scoped refresh `effects-texture-rewake-refresh-2026-08-16T23:54Z` moved epoch **281 → 282**. LifecycleReceipt `01a00cff-7554-7911-b508-78a780181c27`.

Same operations POST (`effects-solitaire-start-2026-08-16T20:08Z`) returned **200**. New Super `00ebeb3d-6082-4811-91ef-8fe2204eaf76` started running with:

- empty `TrajectoryID`
- `request_source=lifecycle_texture_control`
- `assignment_trajectory_id=e826402d-b666-503f-93ba-72b3bcf51e8d`
- empty `requested_by_run_id`
- `self_development_operation_id=selfdev-b090bcd7…`
- persistent Super agent ID `super:5bd6de97-3b58-408c-bf89-c42c81b083de`
- Texture control binding to work item `38b96770-5fb8-585a-8234-db9e4dfbd331`

Texture rewake is paid. Super then **completed blocked** (not failed) at 2026-08-16T23:55:51Z:

> `spawn assigned capsule after durable open: capsule write hosts: open /run/choir/capsules/capsule-751648e9-c6ea-5a5c-adfd-ab9e783d3cb1/root/etc/hosts: read-only file system`

`assign_co_super` opened durably. Capsule spawn wrote identity files through the merged overlay path, which followed a lower `/etc/hosts` symlink into the guest Nix store. No `candidate_id`. No bundle. Mode stayed `propose_only` generation 1. Genesis 409. No mail. Operation `selfdev-b090bcd7` stayed `executing`. This is not a freeze.

A duplicate assignment submission then hit a lifecycle command digest conflict. Super also reported `report_to_texture` `record not found`. Those are secondary to the spawn EROFS.

## What landed in source after that observation

Capsule identity files (`passwd`, `group`, `hosts`, `nsswitch.conf`) are written into the overlay **upperdir** so they hide a lower `/etc/hosts` symlink instead of following it into a read-only store.

## Tests

`go test ./internal/capsule -count=1 -timeout 60s`

`GOOS=linux CGO_ENABLED=0 go test -c ./internal/capsule`
