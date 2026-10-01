# run start 502 — computer realization flaps unavailable during OOM windows

**Status:** OBSERVED 2026-10-01 on staging (build `4a718af4`, guest
`computer-03335285269bdba4f94377e56879f9e6`). Surfaced during the overnight QA
drive — broad `run start` submissions intermittently fail even while the guest
is internally alive and computing.

## Symptom

`choir run start` returns `502 {"error":"failed to resolve user autoputer"}`.
The proxy log shows `api key computer realization unavailable`
(`internal/proxy/api_key_computer_authority.go:183`), which fires when the
resolved computer target has `State != "active"` or an empty `ComputerURL`.

Meanwhile the guest is *alive* — journald shows the autoputer-runtime iterating
tool loops and gateway search/inference succeeding at the same timestamps the
502s occur. So the proxy's resolved routing record lags or flaps relative to
actual VM liveness during an OOM/realization transition window.

## Why it matters for QA

This is a routing-plane availability gap, not a compute failure. The research
desks work (the DOGE/fed-reserve trajectories produced research work items and
search calls), but a human `run start` during an OOM realization flap is
refused. For an owner-facing "submit a task" surface, a flap that outlives the
client timeout reads as "the computer is down" even when it isn't.

## Leading candidate (unconfirmed)

`resolveComputerURLForComputerTarget` gates on `target.State == "active"` +
non-empty `ComputerURL`. During a realization transition (OOM kill → vmctl
recreate → re-register), the target row's state/URL can be momentarily stale or
re-pointed, so a concurrently arriving `run start` resolves to the
not-yet-active record. The flap window equals the realization gap — on an
OOM-cycling guest that's a large fraction of uptime.

## Needed to confirm

- Read the resolved target row (`State`, `ComputerURL`, transition timestamps)
  for the owner computer during a 502, vs. the VM's actual liveness.
- Whether the resolve should retry/back off through a realization transition
  rather than fail fast, or surface a transient "realizing" state the CLI can
  distinguish from a hard failure.

## Related

- `docs/problems/coagent-result-parked-run-not-reactivated-2026-09-30.md` —
  parked-run strand (separate; the wake path, not the submit path).
- Goal `docs/definitions/choir-parked-run-reactivation-2026-09-30.md` names the
  guest OOM/serial-drain as an excluded substrate issue; this flap is a
  *symptom of that same environment* surfacing on the submit path.
