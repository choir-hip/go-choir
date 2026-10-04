# Registration-computer reaches active with no canonical genesis — first write 500s

**Status:** confirmed on staging, reproducible. Fix owner: S0b (disposable-computer probe suite) or S1a hardening — decide at the S1a boundary.

**Found:** 2026-10-04, running the S0m boundary-close stranded-bound probe on a fresh disposable computer (`computer-ca3a2cf9b7d921460ba990cefca57c97`, user `2587b196`, registered via the real product path — passkey registration → ownership created → VM `active` in ~14s).

**Mutation class of this record:** green. The fix is red (provisioning / event-chain authority).

## Symptom

A brand-new registered computer reports `state: active` on vmctl ownership, accepts authenticated `/api/trajectories` (empty), but the first **write** — `POST /api/prompt-bar` — returns `500 {"error":"failed to submit prompt"}`. The autoputer log shows:

```
texture prompt bar: submit: persist agent: store: append projection batch:
invalid computer event transition: invalid genesis
```

Every write fails identically until `POST /api/computers/{id}/lifecycle/bootstrap-chain` is called manually — it minted `genesis_imported` (sequence 1) in one call, after which prompt-bar accepts (`202`).

## Why this is a defect, not intended laziness

- `runtime.go` run-admission already refuses pre-genesis writes *by design* ("computer is pre-genesis: run admission refused (bootstrap-chain required)"), but that gate only guards `StartRun` — the prompt-bar submit path reaches `persist agent` **before** admission and surfaces a raw 500.
- Nothing in the registration → active path mints or waits for `genesis_imported`. The UI's first prompt for a new owner will hit this wall.
- `bootstrap-chain` exists as the repair route, but nothing calls it automatically: not registration, not first `/api/shell/bootstrap`, not prompt-bar. It requires `computer:lifecycle` scope — reachable by the owner's own key, but the owner is never told to call it.
- The S0a probe's earlier note ("api-key create needs a session") plus this finding means every disposable-computer acceptance run must carry a registration→key→bootstrap-chain preamble today.

## Evidence

- Live run, 2026-10-04T03:00–03:03Z: `prompt_bar` 500 ×2 (journal `go-choir-run-autoputer-runtime`), bootstrap-chain `201` minted head `3329a00a…`, then `prompt_bar` `202`.
- Code: `internal/agentcore/runtime.go:843-854` (pre-genesis admission gate), `internal/agentcore/chain_bootstrap.go` (the repair route), `internal/autoputer/projection_base.go:24` ("empty store + no canonical chain: explicit new-computer genesis" — the *intended* contract, apparently not wired to mint on first boot).

## Suspected root cause

`materializeProjectionBaseIfNeeded` says "empty store + no canonical chain: explicit new-computer genesis" — but the mint is either not invoked on the warm/fast provisioning path, or the append that would mint it races the ownership flip to `active`. A 14-second activation strongly suggests a pre-warmed pool VM was bound to the new owner without running the genesis path.

## Fix shape (for the receiving station)

1. Provisioning should mint `genesis_imported` (or refuse to report `active`) before ownership becomes usable — the "explicit new-computer genesis" contract already named in `projection_base.go`.
2. If lazy genesis is kept, prompt-bar/run-admission paths must return a clean "computer initializing" 503 and the frontend must trigger `bootstrap-chain` once, not a 500.
3. S0b's disposable-computer probe suite should add "fresh registration computer accepts first prompt without manual bootstrap-chain" as a regression gate.

## Rollback

None — record only. The deployed repair path (manual `bootstrap-chain` POST) is available to any holder of `computer:lifecycle` scoped to the computer.
