# S1 Deploy Unblock Dispatch

- Suite: `choir-autoputer-completion-2026-07-11-01`
- Slice: `S1-deploy-unblock-01`
- Dispatch nonce: `s1-deploy-unblock-01-nonce-01`
- Canonical parent: `063d42aef8df4e59101a2ed2eed20f8185d9fb31`
- Mutation class: red

## Problem Evidence

Staging public health still reports deployed commit `6e893d90d8df0655398177e839e5270547472cd7`. The authenticated CLI can list old live trajectories, but no product CLI exposes current runs or cancellation. Source already contains owner-scoped `Runtime.CancelRun`, `HandleCancel`, and `HandleRunList`; the two public handlers are not registered, and `cmd/choir` exposes only `run start|status`. This is an existing replacement/connection opportunity and must be wired instead of inventing a second lifecycle path. Active execution has no default activation budget, so an in-process provider/tool loop can remain `running` indefinitely.

## Red-Class Ceremony

- Conjecture delta: the existing `RunRecord.State`/`Runtime.CancelRun` authority is sufficient if the existing list/cancel handlers are wired, cancellation persists terminal state immediately, and every activation receives a bounded 60-minute activation budget. No second lifecycle state machine is needed.
- Protected surfaces: run acceptance, admission occupancy, owner-scoped cancellation, `choir run` CLI, staging hot refresh/deploy.
- Admissible evidence: focused lifecycle and CLI tests; S0 ratchet pass; pushed CI; staging product CLI/API cancellation and `running_runs: 0` or authoritative equivalent; deployed commit identity and subsequent green hot refresh. SSH is not acceptance evidence.
- Rollback: revert the smallest S1 landing to `063d42aef8df4e59101a2ed2eed20f8185d9fb31`; do not reconnect an alternate lifecycle path.
- Heresy delta: `discovered` — existing cancellation/list handlers are unwired and active execution has no bounded budget; `introduced` — none authorized; `repaired` — pending product-path connection, budget, drain, and deploy proof.

## Mutation Lock

Allowed implementation targets:

- `internal/provideriface/provider.go`
- `internal/runtime/config.go`
- `internal/runtime/config_test.go`
- `internal/runtime/runtime.go`
- `internal/runtime/runtime_test.go`
- `internal/runtime/api.go`
- `internal/runtime/api_test.go`
- `cmd/choir/main.go`
- `cmd/choir/main_test.go`
- `docs/runtime-dissolution-inventory.yaml`

Durable suite/evidence updates remain orchestrator-owned. No other production files, lifecycle state machine, admission counter rewrite, retry policy, VM reprovisioning, Wire work, or deployment configuration is authorized.

## Required Behavior

1. Wire the existing authenticated owner-scoped run list and cancellation handlers.
2. Add `choir run list` and `choir run cancel <id>` using those routes; preserve JSON output/error conventions.
3. Cancellation must persist `RunCancelled`, `FinishedAt`, and admission release before the API returns, including a currently resident run; late execution must not overwrite the terminal cancellation.
4. Apply a configurable activation budget with a 60-minute production default and a test override. Deadline expiry must persist a terminal state and release admission. Use progress-deadline/activation-budget vocabulary only.
5. Reuse boot passivation and existing lifecycle authority. Do not add a supervisor state machine.
6. Add deterministic tests for owner scope, immediate resident cancellation, late completion resistance, deadline terminalization/capacity release, route wiring, and CLI list/cancel requests.
7. Regenerate the S0 inventory only for exact added identities and supply explicit dispositions; `go run ./cmd/runtime-ratchet` must pass.

## Required Focused Proof

- `go test ./internal/runtime ./cmd/choir -run 'Deadline|ActivationBudget|Cancel|RunList'`
- `go test ./cmd/runtime-ratchet`
- `go run ./cmd/runtime-ratchet`

The implementer returns an isolated commit and exact outputs. It does not push, deploy, drain staging, or edit suite/evidence documents.

## Independent Verification Receipt

At canonical `e649ee28`, `S1DeployVerifier` reported **BLOCKING**. Focused runtime/CLI tests and `go test ./cmd/runtime-ratchet` passed. The required default ratchet invocation failed because the regenerated baseline still cited two prospective suite entries for `internal/runtime/runtime_test.go` and `internal/runtime/api_test.go`; the implementation did not modify those files, and the orchestrator correctly removed the prospective entries before verification. The baseline therefore contained two nonexistent citer identities and reported 165 citers versus the current 163.

**Classification:** verification-order drift, not a lifecycle behavior failure. Route registration, owner scope, CLI shapes, immediate terminal cancellation, deadline terminalization, admission release, and late-completion resistance passed independent review.

**Required repair:** regenerate the inventory against the final canonical suite state, preserving all S1 code identities and explicit dispositions while removing only the two nonexistent citations; then rerun focused and default ratchet proof and independent verification.

## Deployed Acceptance Receipt

GitHub Actions run `29178010201`, attempt 3, completed successfully after the
first retry had passivated the stale pre-S1 runs. The deploy activation receipt
records target `26d7aa2accda63e20daa19c42381d13aec14baed` with `ordinary_guest`,
`sandbox`, `active_computers`, and `gateway` active. The full rerun passed all
selected build, test, race, ratchet, health, and deploy gates.

The deployed product CLI then proved the owner-scoped surfaces:

- `choir run list` returned current lifecycle records and exposed an active
  Texture child run.
- A dedicated API/CLI-equivalent probe submitted prompt-bar activation
  `77da11b4-d4ed-488e-846d-9f060d5a9b07`, observed child run
  `8d203e02-29b7-4f6b-a7e2-bfb95434cf9d` in `running`, cancelled it through
  `POST /api/agent/cancel`, received HTTP 200 with `state: cancelled`, and read
  the same durable terminal state back with
  `finished_at: 2026-07-12T03:57:52.141Z`.

This is staging product-path evidence for list routing, active-run
cancellation, immediate durable terminalization, and admission release. The
60-minute production activation budget and late-completion overwrite guard
remain covered by focused runtime tests and independent verification; waiting
60 minutes is not required for the staging cancellation transition.

## Final Consensus Finding

Codex reproduced a blocking uncovered lifecycle race:
`passivateIdleToolLoopRun` writes its activation snapshot directly through
`Store.UpdateRun`. If owner cancellation or deadline terminalization wins after
the tool loop returns its passivation result but before that direct write, the
stale passivation snapshot can overwrite `cancelled`/`failed`, clear
`FinishedAt`, and make the run nonterminal again. The shared
`persistActivationState` guard protects completion and failure writes but is
not used by this passivation path.

**S1-CONS-001 — confirmed blocking.** Passivation must use the same serialized
stored-terminal-wins authority as all other post-provider lifecycle writes, and
a deterministic cancellation/deadline-versus-passivation regression must fail
before the repair and pass afterward. The deployed acceptance remains valid
for the route it exercised but does not disprove this narrower race.

## S1-CONS-001 Repair Receipt

Commit `4973ee40570382c25398ea50e15148569cf351ab` routes idle passivation through
`persistActivationState`, so the shared lifecycle lock reloads durable state and
stored terminal authority wins. The reproduced regression
`TestIdlePassivationCannotOverwriteCancelledRun` failed before the repair with
`state "passivated" finished_at <nil>` and passes after it. The false persistence
result returns before emitting a passivation event or reconciling Texture.

Focused lifecycle tests passed across `internal/runtime` and `cmd/choir`.
`go test ./cmd/runtime-ratchet && go run ./cmd/runtime-ratchet` passed with 150
Go files, 48,892 production LOC, 55,707 test LOC, 1,215 exports, 462 exact store
calls, four interface candidates, and 164 citers. `S1DeployVerifier`
independently returned PASS on the repair and exact regression.

GitHub Actions run `29179656372` passed all selected build, test, race, ratchet,
SBOM, and deploy gates. Its activation receipt records `sandbox` and `gateway`
active at `4973ee40570382c25398ea50e15148569cf351ab` at
`2026-07-12T04:37:20Z`.

Post-deploy, the actual product CLI command
`choir run cancel 2d37e688-7fa5-4034-859e-b98b3a445e01` returned
`state: cancelled`; the second active probe
`7b0cb532-b4d1-4c85-9a9d-062beab82197` was also cancelled through the CLI.
This closes the earlier non-blocking evidence gap between the authenticated API
and the CLI wrapper.

## Post-Repair Consensus Adjudication

The post-repair panel produced four explicit S1 PASS verdicts: Codex
(`0.98`), OMP GPT-5.5 (`0.94`), OMP Gemini 3.5 (`1.0`), and Cursor. Opencode
independently ran the exact focused regressions successfully but did not finish
the requested verdict after its attempt to create a temporary worktree was
denied. OMP GLM produced no output and was terminated as stalled; neither
incomplete member contributes a vote or a finding.

No completed reviewer produced a blocking finding. The only new observation was
the already-classified non-blocking boot-only direct passivation write in
`passivateInterruptedActivations`; it operates before resident activation
admission and cannot race owner cancellation or a resident progress deadline.
The orchestrator therefore adjudicates S1-CONS-001 repaired and the S1
checkpoint **PASS**.

After the repair receipt introduced one new runtime-package citer,
the inventory classified it as `historical_evidence`; canonical
`go test ./cmd/runtime-ratchet && go run ./cmd/runtime-ratchet` passed at
`9dff3690` with 165 citers. This is the final S1 ratchet state.
