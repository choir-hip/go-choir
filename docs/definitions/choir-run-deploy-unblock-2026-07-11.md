# Choir Run Deploy Unblock

## Subordinate Invocation Semantics

This document is the bounded S1 subgoal specification of:

```text
/goal docs/definitions/choir-autoputer-completion-suite-2026-07-11.md
```

Do not invoke it as an independent mission. The grand-suite orchestrator
delegates its execution and micro-verification, runs the phase-checkpoint
consensus, and records durable evidence and resumption state in the suite.

## Why this mission exists

`docs/ACTIVE.md` Remaining Error: `Deploy to Staging (Node B)` fails because
`vm-universal-wire-platform` reports `running_runs: 1`. The run is **`running`**,
not `blocked`. The post-mortem's original three freeze modes did not cover a
run that stays `running` forever. Hot-refresh waits on sandbox `/health` with
the new commit and cannot complete while admission is occupied.

The `sourcecycled` `blocked`→terminal fix (`f1ceba5`) does not drain this run.
CI may already ignore `skills/*` for sandbox deploy classify (`d8fe4336`); that
is not this mission's scope. This mission only restores Deploy by giving
`RunRunning` a progress deadline and an operator cancel path.

## Source Authority Order

1. `docs/definitions/choir-autoputer-completion-suite-2026-07-11.md`.
2. This subordinate Definition within S1 scope.
3. `docs/ACTIVE.md` Remaining Error evidence.
4. `docs/definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md`
   for inherited semantics only; its implementation phase is later.
5. `docs/standing-questions.md`, `AGENTS.md`, `docs/choir-doctrine.md`.
6. Observed source: `internal/runtime/runtime.go`, `internal/runtime/api.go`,
   `cmd/choir`.

## Settled Inputs (do not re-litigate)

- Single lifecycle authority default is `RunRecord.State` (suite member 3 owns
  full projection alignment; this mission only needs terminal transitions that
  release admission).
- Doctrine rejects lease as a control concept (H019). New identifiers use
  **progress deadline** / **activation budget** only.
- Continuation / parent-child / channel deletion is out of scope (og-dolt).

## Mission Purpose

1. Expose an operator cancel command (`choir run cancel <id>` or equivalent API) that transitions an active run to a terminal state and immediately releases its admission slot.
2. Ensure stuck runs transition to a terminal state on timeout (default 60 minutes) and release admission, preventing permanent resource lock-up.
3. Drain the stuck staging run, achieving `running_runs: 0` and restoring the staging deploy pipeline.

## Mission Non-Purpose

- No admission counter rewrite, retry policy, or artifact-verified completion (suite member 3).
- No wire-store conformance (suite member 2).
- No vocabulary purges or renames (suite member 4 / og-dolt E).
- No VM reprovisioning; instance name `vm-universal-wire-platform` stays.

## Open Decisions (agent discretion)

- **Progress clock implementation:** Left to agent discretion (e.g. simple timer or updatedAt check) provided the outcome of releasing admission on timeout is met.
- **Cancel path UX:** Exposing `choir run cancel <id>` as the primary operator CLI verb.

## Invariants

- No run occupies admission indefinitely without observable progress or active execution context.
- New code must not introduce lease-named control identifiers (conforming to H019).
- Acceptance uses product CLI/API only (no platform-operator SSH).

## Completion Semantics

Complete when all are observed on staging:

1. Operator cancellation via API/CLI transitions a running run to a terminal state and decrements `running_runs`.
2. Stuck runs are cleaned up after the timeout expires, releasing admission.
3. The stuck platform VM run is drained (`running_runs: 0`).
4. Staging deploy hot-refresh successfully accepts and verifies subsequent commits.
5. Unit/integration tests cover cancellation and timeout capacity release.
## Sequencing and Gates

This subordinate inherits the grand-suite behavior-phase checkpoint protocol.
An optional pre-mutation review may be recorded as planning evidence, but it is
advisory and cannot replace the mandatory post-acceptance consensus and
orchestrator adjudication. All prompts, outputs, receipts, and decisions are
durable grand-suite ledger refs, never only `/tmp`.

Required focused proof:

1. `go test ./internal/runtime ./cmd/choir -run 'Deadline|Cancel|Sweep'`.
2. Landing loop per `AGENTS.md` (push `origin/main`, monitor CI + deploy).
3. Drain the stuck run; prove Deploy green; record run IDs and Deploy URL.
4. On red CI, identity mismatch, or QA failure, document and repair or revert
   the smallest atomic landing to its recorded pre-mutation SHA.

## Execution

### Phase 0 — Optional Planning Review (green/yellow)

If run, persist it as advisory suite evidence; no code mutation and no phase
status advancement.

### Phase A — Deadline, cancel, drain, Deploy proof (red)

- Implement progress deadline + sweep in `internal/runtime`.
  Reuse `Runtime.CancelRun` and the existing authenticated cancellation API;
  add no second lifecycle state machine.
- Implement `choir run cancel` in `cmd/choir`.
- Before landing, record every added `internal/runtime` file, symbol, test,
  route, configuration field, and production caller in the grand suite's
  `s1_runtime_exception_disposition`; the orchestrator enforces this gate.
- Land; drain staging; prove Deploy.

On success, propose the `docs/ACTIVE.md` Remaining Error update and return all
receipts to the grand-suite orchestrator. This subordinate document may record
S1 evidence but cannot advance suite state. Only the grand checkpoint marks S1
complete after deployed proof, independent verification, consensus
adjudication, and durable state.

## Follow-on

- Grand S2: `choir-wire-store-conformance-2026-07-11.md`.
- Grand S6 inherits that deadlines/cancel exist; it must not re-litigate them
  unless staging regresses.

## Supersession Record

- Extracted from run-lifecycle Phase C (2026-07-11 suite foliation) so Deploy
  unblock is not gated on full lifecycle or naming work.
- Does not supersede the full run-lifecycle Definition.

## Red-Class Ceremony

- **Mutation class:** green for this doc; code is red (run acceptance, CLI).
- **Autonomous execution:** defaults govern; consensus deviations recorded;
  unrepairable halt → documented failure report; no owner pause required.
- **Protected surfaces:** run acceptance, admission occupancy, `choir run` CLI,
  staging deploy refresh path.
- **Admissible evidence:** staging drain + Deploy run URL; unit tests; no SSH.
- **Rollback:** revert to pre-mutation SHA; re-disable cancel/deadline if needed.
- **Heresy delta:** `discovered` — fourth freeze mode (`running` forever);
  `repaired` — progress deadline + operator cancel restore Deploy.
