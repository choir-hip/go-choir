# Effects Super f8ee744f spawned bound CoSuper — 2026-08-17

**Boundary:** execute. Not freeze. Not promote. No live send.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Host:** `https://choir.news/health` `deployed_commit` `dddcd80da0547ba476f4aee7d431ec70f84f44d5` (`deployed_at` 2026-08-17T02:04:07Z)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31985817733 succeeded after rerunning the timed-out non-runtime shard, including Node B.
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` epoch **285**

## Live observation

G4 preserved constructed computer `candidate-fleet-e15cb89f25d963c220319b7b` (`code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380`). Freeze remains `7122f279`.

Owner-scoped refresh `effects-proc-sys-mask-refresh-2026-08-17T02:05Z` moved epoch **284 → 285**. LifecycleReceipt `01a00d76-c153-764b-9f3b-9d88a927c105`. Super `2210c654` was already terminal. Same operations POST returned **200**.

New Super `f8ee744f-dc92-4079-9746-47a759d82331` started with Texture `Direction=control` join (`assignment_trajectory_id=e826402d…`, `request_source=lifecycle_texture_control`) and **completed** at 2026-08-17T02:05:46Z after opening a bound CoSuper implementation assignment:

- `assignment_id`: `assignment-fa38b037-bd9d-5270-a640-e668afa4eb57`
- `assigned_work_item_id`: `work:assignment-fa38b037-bd9d-5270-a640-e668afa4eb57`
- CoSuper run: `run:assignment-fa38b037-bd9d-5270-a640-e668afa4eb57`
- `capsule_id`: `capsule-83337f60-080b-5b45-9df5-bc116d823867`
- parent work: `38b96770-5fb8-585a-8234-db9e4dfbd331`

Capsule spawn no longer failed on `/etc/hosts` EROFS or broker readiness. Super reported a duplicate `lifecycle command digest conflict` on a concurrent assignment-open attempt and reported the open assignment to Texture.

CoSuper stayed **running** with unchanged `updated_at` `2026-08-17T02:05:08.829Z`, empty result, and no token counts through at least 2026-08-17T02:16:42Z. Operation `selfdev-b090bcd7` stayed `executing` with no bundle. Mode stayed `propose_only` generation 1. Genesis 409. No mail. This is not a freeze.

Do **not** retry the same operations POST while this CoSuper is live. Super `f8ee744f` is terminal; a retry would unbind it and start a new Super.

## Tests

`go test ./internal/capsule -count=1 -timeout 60s` on `dddcd80d`

`GOOS=linux CGO_ENABLED=0 go test -c ./internal/capsule`
