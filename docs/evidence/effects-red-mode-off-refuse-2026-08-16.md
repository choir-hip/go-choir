# Effects red mode-off refuse after owner-scoped refresh

**Boundary:** execute (route map 9 red smoke). Not live proof. Not a live send. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Deploy:** `https://choir.news/health` 2026-08-16T04:30Z `deployed_commit` `21b79872` (`deployed_at` 2026-08-16T04:27:50Z, `built_at` 20260816040241)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31925691337
**Computer:** `computer-03335285269bdba4f94377e56879f9e6`

## Actuation

`choir computer refresh --computer computer-03335285269bdba4f94377e56879f9e6 --idempotency-key effects-red-guest-boot-refresh-2026-08-16T04:29Z`

| Field | Value |
|---|---|
| receipt_id | `01a008d4-f9d3-7130-9346-786212018260` |
| action | `refresh` |
| prior | active / epoch **268** |
| resulting | active / epoch **269** |
| rematerialize | not invoked |
| restart | not invoked |
| mode set | no |

This is `RefreshVM` / `POST /internal/vmctl/refresh`, not tape rematerialize and not `choir computer create`. Frozen ComputerVersion `code_commit` remains **`7122f279`**; refresh does not rewrite that join.

## After refresh (2026-08-16T04:30Z)

| Call | Result |
|---|---|
| `POST .../self-development/genesis` | 409 `self-development effects are disabled` |
| `POST .../self-development/operations` (`effects-red-mode-off-2026-08-16T04:30Z`) | **409** `current signed mode does not authorize proposal` |
| `GET .../operations/operation-red-rehearsal` | 404 `self-development operation not found` |
| `GET .../kernel-capabilities` | 503 `kernel capability authority unavailable` |
| `GET .../self-development/mode` | 200 `mode=off` generation 0 |
| `choir computer status` | active, epoch **269** |
| runtime | reachable, `service=autoputer`, `runtime_health=ready` |

No mail was sent. Mode was not set. Restore was not invoked. Outbox `Armed` remains false. Owner gates were not deleted.

## What this is not

This is not red rehearsal of propose → consensus → promote → restore. Kernel/updater/route remain unmounted (`kernel capability authority unavailable`), so materialize cannot run. Orange in-process rehearsal remains the only promote/outbox composition proof. Live proof (route map 10) remains unpaid.

## Next

Do not set mode yet. Next unpaid product slice is guest kernel/route/updater wiring so promote can fail closed, then red promote+restore without a live send. Do not rematerialize. Do not invent choir computer create. Do not independently green restore. Do not send live mail.
