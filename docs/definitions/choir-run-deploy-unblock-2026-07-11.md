# Choir Run Deploy Unblock

## Subordinate Invocation Semantics

This document is the settled Deploy receipt imported by:

```text
/goal docs/definitions/choir-audited-autoputer-construction-2026-07-15.md
```

Do not invoke it as an independent mission. Its accepted evidence remains a
predecessor receipt; the active mission does not rerun it during resumption.

## Why this mission exists

Historical failure evidence recorded that `Deploy to Staging (Node B)` was
blocked because `vm-universal-wire-platform` reported `running_runs: 1`. The
run was **`running`**, not `blocked`; hot-refresh waited for sandbox `/health`
with the new commit while admission remained occupied.

The later accepted receipt restored Deploy with a progress deadline, operator
cancel path, and drained run. This failure is no longer current state. The
details remain here to define what the settled receipt proved.

## Source Authority Order

1. `docs/definitions/choir-audited-autoputer-construction-2026-07-15.md`.
2. This settled subordinate Definition as a predecessor receipt.
3. This section's historical Deploy failure evidence.
4. `docs/definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md`
   for inherited semantics only; its implementation phase is later.
5. `docs/standing-questions.md`, `AGENTS.md`, `docs/choir-doctrine.md`.
6. Observed source: `internal/runtime/runtime.go`, `internal/runtime/api.go`,
   `cmd/choir`.

## Settled Inputs (do not re-litigate)

- Single lifecycle authority default is `RunRecord.State` (R4 owns full
  projection alignment; this settled receipt only covers terminal transitions
  that release admission).
- Doctrine rejects lease as a control concept (H019). New identifiers use
  **progress deadline** / **activation budget** only.
- Continuation / parent-child / channel deletion is out of scope (og-dolt).

## Mission Purpose

1. Expose an operator cancel command (`choir run cancel <id>` or equivalent API) that transitions an active run to a terminal state and immediately releases its admission slot.
2. Ensure stuck runs transition to a terminal state on timeout (default 60 minutes) and release admission, preventing permanent resource lock-up.
3. Drain the stuck staging run, achieving `running_runs: 0` and restoring the staging deploy pipeline.

## Mission Non-Purpose

- No admission counter rewrite, retry policy, or artifact-verified completion (R4).
- No wire-store conformance (the settled Wire receipt owns it).
- No vocabulary purges or renames (R7 / og-dolt E).
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
## Settled Receipt Verification

This work is complete and must not be re-executed during resumption. Its
accepted commit, CI/deploy evidence, staging proof, independent verification,
and rollback ref are imported by the active mission capsule.

Historical focused proof:

1. `go test ./internal/runtime ./cmd/choir -run 'Deadline|Cancel|Sweep'`.
2. Landing Loop per `AGENTS.md`.
3. Product CLI/API cancellation, timeout capacity release, drained
   `running_runs`, and a successful later deploy refresh.

A reproduced regression returns to the active mission as a new code-free Define
boundary. This document cannot authorize a mutation, reopen the settled Deploy
receipt, or maintain a second evidence ledger.

## Follow-on

- R4 inherits that deadlines/cancel exist and must not re-litigate them unless
  staging reproduces a regression.

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
