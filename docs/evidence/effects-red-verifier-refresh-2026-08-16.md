# Effects red guest refresh after verifier wiring

**Boundary:** execute (route map 9 red smoke). Not live proof. Not a live send. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Deploy:** `https://choir.news/health` 2026-08-16T05:32Z `deployed_commit` `5557840c` (`deployed_at` 2026-08-16T05:32:15Z, `built_at` 20260816051712)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31928656492 (includes Deploy to Staging (Node B))
**Computer:** `computer-03335285269bdba4f94377e56879f9e6`

## Baseline before refresh (host already `5557840c`, epoch 270)

| Call | Result |
|---|---|
| `POST .../self-development/genesis` | 409 `self-development effects are disabled` |
| `POST .../self-development/operations` (`effects-red-verifier-pre-2026-08-16T05:32Z`) | 409 `current signed mode does not authorize proposal` |
| `GET .../kernel-capabilities` | 200 signed `KernelCapabilityReceipt` (boot_id `867f5480-9438-4b6b-8aed-9e34fef7584e`, generation 270) |
| `GET .../self-development/mode` | 200 `mode=off` generation 0 |
| `choir computer status` | active, epoch **270** |

## Actuation

`choir computer refresh --computer computer-03335285269bdba4f94377e56879f9e6 --idempotency-key effects-red-verifier-refresh-2026-08-16T05:32Z --host https://choir.news --timeout 8m`

| Field | Value |
|---|---|
| receipt_id | `01a0090f-191b-72f2-8d36-7bf1b15134f2` |
| action | `refresh` |
| prior | active / epoch **270** |
| resulting | active / epoch **271** |
| rematerialize | not invoked |
| restart | not invoked |
| mode set | no |

Frozen ComputerVersion `code_commit` remains **`7122f2799be4458f4b925be11990321c7e70ffc4`**. Refresh does not rewrite that join.

## After refresh (2026-08-16T05:33Z)

| Call | Result |
|---|---|
| `POST .../self-development/genesis` | 409 `self-development effects are disabled` |
| `POST .../self-development/operations` (`effects-red-verifier-post-2026-08-16T05:33Z`) | **409** `current signed mode does not authorize proposal` |
| `GET .../operations/operation-red-rehearsal` | 404 `self-development operation not found` |
| `GET .../kernel-capabilities` | **200** `KernelCapabilityReceipt` `01a0090f-4cb0-7b00-849c-effeca21d526` issuer `choir-updater` lifecycle_generation **271** realization `candidate-fleet-e15cb89f25d963c220319b7b-epoch-271` |
| `GET .../self-development/mode` | 200 `mode=off` generation 0 |
| `choir computer status` | active, epoch **271** |

No mail was sent. Mode was not set. Restore was not invoked. Outbox `Armed` remains false. Owner gates were not deleted.

Verifier mount is not independently GET-able: genesis stays proxy-disabled, and `selfdevVerifier.PublicKey` is only used by genesis reconstruction and materialize. The same owner-scoped refresh path previously picked up mode-authority (503→409) and kernel-route (503→200). This refresh advanced the guest to epoch 271 on host `5557840c`. Guest init still prefers `/mnt/persistent/choir-updater/current/bin/autoputer` when that file is executable.

## What this is not

This is not red rehearsal of propose → consensus → promote → restore. Mode remains `off`. Orange in-process rehearsal remains the only promote/outbox composition proof. Live proof (route map 10) remains unpaid. Kernel GET 200 is not permission to set mode.

## Next

Do not set mode yet until the red promote+restore slice names the exact mode and refuse matrix. Next unpaid product slice is red promote+restore without a live send. Do not rematerialize. Do not invent choir computer create. Do not independently green restore. Do not send live mail.
