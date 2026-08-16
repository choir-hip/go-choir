# Effects red product-path smoke on staging 4543624b

**Boundary:** execute (route map 9 red smoke / route map 10 prep). Not live proof. Not a live send. Effects remain OFF.
**Parent:** `docs/definitions/choir-supervised-self-development-effects-2026-08-11.md`
**Deploy:** `https://choir.news/health` 2026-08-16T02:05Z `deployed_commit` `4543624b` (`deployed_at` 2026-08-16T02:02:51Z, `built_at` 20260816014626)
**CI:** https://github.com/choir-hip/go-choir/actions/runs/31920331644 (flake rerun of actorruntime SQLITE_BUSY, then deploy)
**Computer:** `computer-03335285269bdba4f94377e56879f9e6` state `active` epoch **268** mode **`off`** generation 0

## Baseline before this deploy (3141b90b)

Authenticated POST genesis, operations, and decision all returned 409 `self-development effects are disabled` at the proxy catch-all.

## After 4543624b

| Path | Status | Body | Meaning |
|---|---|---|---|
| `POST .../self-development/genesis` | 409 | `self-development effects are disabled` | Genesis remains proxy-closed |
| `POST .../self-development/operations` | 503 | `self-development mode authority unavailable` | Forwarded. Guest `selfdevControl` (`WithSelfDevelopmentControl`) is unmounted; autoputer wires only `WithOwnerRecoveryControl` and logs effects disabled |
| `POST .../operations/operation-red-rehearsal/decision` (incomplete) | 400 | `expected_pending_transition_ref is required` | Forwarded into guest decoder |
| `POST .../operations/operation-red-rehearsal/decision` (complete reject binding, no such op) | 404 | `self-development operation not found` | Forwarded into guest operations store. No operation created |
| `GET .../kernel-capabilities` | 503 | `kernel capability authority unavailable` | Guest updater/route not fully wired for capabilities |
| `GET .../self-development/mode` | 200 | `mode=off` generation 0 | Platform mode CAS unchanged |

No mail was sent. Mode was not set. Restore was not invoked. Outbox `Armed` remains false. Owner gates were not deleted.

## What this is not

This is not red rehearsal of propose → consensus → promote → restore. Start cannot reach the mode-off refuse (`current signed mode does not authorize proposal`) until the guest mounts `WithSelfDevelopmentControl`. Orange in-process rehearsal remains the only promote/outbox composition proof.

## Next

Wire guest `WithSelfDevelopmentControl` so mode `off` can refuse proposal on staging. That is an autoputer/guest-boot change and needs an active VM refresh to take effect. Do not set mode. Do not send live mail. Do not rematerialize. Do not independently green restore.
