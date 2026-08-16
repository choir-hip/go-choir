# Effects red live kernel capability receipt after 7eee9f10 refresh

**Boundary:** execute (route map 9 red smoke). Not live proof. Not a live send. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Deploy:** `https://choir.news/health` 2026-08-16T05:08Z `deployed_commit` `7eee9f10` (`deployed_at` 2026-08-16T05:04:03Z, `built_at` 20260816043836)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31927114217
**Computer:** `computer-03335285269bdba4f94377e56879f9e6`

## Baseline before refresh (host already `7eee9f10`, epoch 269)

| Call | Result |
|---|---|
| `POST .../self-development/genesis` | 409 `self-development effects are disabled` |
| `POST .../self-development/operations` (`effects-red-kernel-pre-2026-08-16T05:05Z`) | 409 `current signed mode does not authorize proposal` |
| `GET .../kernel-capabilities` | 503 `kernel capability authority unavailable` |
| `GET .../self-development/mode` | 200 `mode=off` generation 0 |
| `choir computer status` | active, epoch **269** |

The constructed guest was still on the pre-`7eee9f10` binary. Global deploy preserved `snapshot_kind=constructed-computer-version`.

## Actuation

`choir computer refresh --computer computer-03335285269bdba4f94377e56879f9e6 --idempotency-key effects-red-kernel-route-refresh-2026-08-16T05:05Z --host https://choir.news --timeout 8m`

| Field | Value |
|---|---|
| receipt_id | `01a008f8-6612-7bcd-9033-1b32d4551aea` |
| action | `refresh` |
| prior | active / epoch **269** |
| resulting | active / epoch **270** |
| rematerialize | not invoked |
| restart | not invoked |
| mode set | no |

This is `RefreshVM` / `POST /internal/vmctl/refresh`, not tape rematerialize and not `choir computer create`. Frozen ComputerVersion `code_commit` remains **`7122f2799be4458f4b925be11990321c7e70ffc4`**; refresh does not rewrite that join. `code_ref` / `artifact_program_ref` stay `code:sha256:499bee7bf2a486941c5a717a8b25b4030bc869929f96a0ac625f08e9eac9f380` / `artifact-program:sha256:9d90c8666a1d9a69f46daca644bb9470505831bb9926e21d2a577d0bd9aa5a6f`.

## After refresh (2026-08-16T05:08Z)

| Call | Result |
|---|---|
| `POST .../self-development/genesis` | 409 `self-development effects are disabled` |
| `POST .../self-development/operations` (`effects-red-kernel-post-2026-08-16T05:08Z`) | **409** `current signed mode does not authorize proposal` |
| `GET .../operations/operation-red-rehearsal` | 404 `self-development operation not found` |
| `GET .../kernel-capabilities` | **200** signed `KernelCapabilityReceipt` |
| `GET .../self-development/mode` | 200 `mode=off` generation 0 |
| `choir computer status` | active, epoch **270** |

Kernel receipt (read, not a promotion):

| Field | Value |
|---|---|
| receipt_id | `01a008f9-0bb2-7d09-9cb9-dda525530dcd` |
| receipt_kind | `KernelCapabilityReceipt` |
| issuer | `choir-updater` |
| boot_id | `867f5480-9438-4b6b-8aed-9e34fef7584e` |
| lifecycle_generation | **270** |
| realization_id | `candidate-fleet-e15cb89f25d963c220319b7b-epoch-270` |
| observed_at | 2026-08-16T05:08:10.724511827Z |
| issued_at | 2026-08-16T05:08:54.322167316Z |
| canonical_payload_sha256 | `738c60290a779a89834f61ca6a47866bfeaabcd367bdfd88978eef4b8d63dd21` |
| kernel_release | `6.18.21` |
| signer | `guest-core` / `guest-core-8fe17612df9c9de8` |
| computer_version | same constructed CodeRef / ArtifactProgramRef as the freeze |

All ten `observed_capabilities` are `supported=true` and `enforced=true` (cgroup_v2, ipc/mount/network/pid/user/uts namespaces, landlock, overlayfs, seccomp).

This is past `kernel capability authority unavailable`. It is also past the acceptable later 503s (`computer route identity unavailable` / `kernel capability receipt unavailable`): updater, route, and updater-root probe succeeded on the refreshed guest.

No mail was sent. Mode was not set. Restore was not invoked. Outbox `Armed` remains false. Owner gates were not deleted.

## What this is not

This is not red rehearsal of propose → consensus → promote → restore. `WithSelfDevelopmentVerifier` remains unmounted, so `reconcileSelfDevelopmentMaterialization` still no-ops. Orange in-process rehearsal remains the only promote/outbox composition proof. Live proof (route map 10) remains unpaid. Kernel GET 200 is a signed capability read, not permission to set mode.

## Next

Do not set mode yet. Next unpaid product slice is guest verifier wiring so the materializer can fail closed instead of remaining inert, then red promote+restore without a live send. Do not rematerialize. Do not invent choir computer create. Do not independently green restore. Do not send live mail.
