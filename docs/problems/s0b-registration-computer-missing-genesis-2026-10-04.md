# Registration-computer reaches active with no canonical genesis — first write 500s

**Status:** confirmed on staging, reproducible; ROOT CAUSE CONFIRMED 2026-10-06 — fix owned by SA slice 0 (v5.1 ordering).

**Found:** 2026-10-04, running the S0m boundary-close stranded-bound probe on a fresh disposable computer (`computer-ca3a2cf9b7d921460ba990cefca57c97`, user `2587b196`, registered via the real product path — passkey registration → ownership created → VM `active` in ~14s).

**Confirmed root cause (2026-10-06):** `materializeProjectionBaseIfNeeded` (`internal/autoputer/projection_base.go:86-93`) plans `RecoveryGenesis` (empty store + no platform chain) and returns without minting — per its own comment, "explicit new-computer genesis" was left to the manual `bootstrap-chain` route, and nothing in the boot sequence calls it. `recovery_plan.go:88` names the same intent ("explicit new-computer bootstrap"). The mint code in `Runtime.BootstrapChain` (`chain_bootstrap.go`) exists only behind the owner-scoped HTTP route. Meanwhile vmctl reports `active` as soon as `BootVM`'s guest `/health` returns 200 and the replay gate opens serving — a fresh computer is `active`-usable with zero canonical events.

**Fix shape adopted (SA slice 0, red):**

1. **Provisioned genesis mint, in-guest, pre-replay.** After `BindProjectionTape` succeeds and before `runReplayPhase` starts, when the recovery plan was `RecoveryGenesis` (empty store + no platform chain — provably fresh: a wiped store leaves a platform chain and plans `RecoveryInstall`/refuse instead), the guest mints the same `genesis_imported` event `BootstrapChain` produces (extracted shared mint helper), with `AuthorityRef "provisioned-genesis:"+CHOIR_OWNER_ID` and its own idempotency key. `RecoveryGenesis` is the only plan that mints. Failure is logged, not fatal: the computer stays pre-genesis under the PF-3 admission gate, repairable by the owner-scoped `bootstrap-chain` route exactly as today — a boot that cannot mint must not crash-loop.
2. **Ordering guarantee.** The mint precedes the replay phase; `replayHealthGate` holds `/health` at 503 `ReplayInProgress` until replay completes, and vmctl's `BootVM` waits on `/health` 200 before the ownership flips to `active`. So `active` ⇒ canonical head ≥ 1: no window where a computer reports active pre-genesis.
3. **Clean 503 pre-genesis.** `HandlePromptBar` maps the pre-genesis admission error to a typed sentinel → `503 {"error":"computer initializing"}` instead of the raw 500 (covers both the residual repair window and any mint-failed boot).
4. **Regression gate.** Fresh registration → first `POST /api/prompt-bar` succeeds (202) with no manual `bootstrap-chain`; the S0b keydriver preamble becomes unnecessary.

**Deployed acceptance (2026-10-06, commit 9f6f369c):** fresh registration on post-fix staging (`computer-07b582d5`, user `2d595c59`): `bootstrap-chain` reports `already_bootstrapped` immediately (genesis minted in-guest at boot; the mint only logs on append, and the pre-mint head check returns silently, so absence of a mint log line is expected when the head is already present). First-ever write (`sa0-accept-1`) still returned 500 — **residual**: `texture prompt bar: submit: start initial Texture agent revision: replace durable activation: lifecycle invalid transition` — a *transient* race between the first activation commit and the just-booted outbox/replay phase (the trajectory + work item committed and the run exists as `passivated`). The second identical submit (13:07:15) hit the same failure; the third (`sa0-accept-3`, 13:08:51) succeeded `202` — by then the replay/outbox burst had drained. So: genesis class fixed (no more pre-genesis 500/503 repair dance), but a separate *first-activation-races-boot-replay* defect is exposed behind it. That defect is a symptom-level CAS race on `projectLifecycleRun` (bare `ErrLifecycleInvalidTransition` under concurrent activation attempts on a brand-new trajectory) — belongs with the SA slice 1 occurrence-storm investigation, not the genesis contract.

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
