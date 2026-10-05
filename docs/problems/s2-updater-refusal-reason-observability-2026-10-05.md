# Updater daemon discards its refusal reason — layered-apply refusals are un-diagnosable
Date: 2026-10-05 · Station: S2 · Surface: `cmd/choir-updater/main_linux.go` `/v1/apply`

## Symptom
`POST /v1/apply` returns `{"error":"updater refused request"}` for **every** pre-mutation refusal gate. The daemon has at least eight distinct refusal paths that each produce their own message, and exactly one of them reaches the caller:

| Gate | Message the daemon computes | What the caller sees |
|---|---|---|
| journal/read | `updater: invalid operation journal phase %q` | `updater refused request` |
| commitment | `updater: request commitment mismatch` | `updater refused request` |
| realization fence | `updater: incomplete or mismatched apply request` | `updater refused request` |
| base digest join | `updater: release base %s does not match booted base %s` | `updater refused request` |
| state compat | `updater: guest store schema %d is newer…` / base_commit join | `updater refused request` |
| source trust | `updater: incoming source directory must be private` | `updater refused request` |
| manifest verify | `updater: invalid release manifest` | `updater refused request` |
| closure replay | `updater: replay app-layer closure: %w` | `updater refused request` |

Only the recovered path is informative (`:174`, `materialization failed and prior release was restored: %v`).

## Cost
Five distinct refusal attempts on the disposable (2026-10-05 05:54 → 07:05) could not be told apart from outside the guest. The guest's `console.log` sink stops ~16 s after boot, and the daemon writes nothing to its journal for a pre-mutation refusal, so the message exists only in the HTTP response that the daemon throws away. Root cause required source-tracing all eight gates and then bisecting by elimination — roughly five hours of the station's budget.

## Fix
Return the computed error as an additional field alongside the stable `error` string. The stable string stays the machine-facing discriminator (existing callers and the CI push classifier `grep -qiE '"error"|refused|rejected'` keep working); `reason` carries the diagnosis.

## Why not more
The daemon cannot log it: it is a separate systemd unit whose journal lives inside the guest and is not host-readable for disposables, which is the same observability hole recorded for the layered exec in `docs/problems/s2-runtime-exec-still-baseline-2026-10-04.md`. The HTTP response is the only channel that already exists and already crosses the host boundary.