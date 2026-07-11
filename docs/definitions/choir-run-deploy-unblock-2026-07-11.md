# Choir Run Deploy Unblock

## Harness Invocation Semantics

```text
/goal docs/definitions/choir-run-deploy-unblock-2026-07-11.md
```

Read this document as executable semantic authority for restoring staging
deploy hot-refresh by draining stuck `RunRunning` occupancy. It is **member 1**
of `docs/definitions/choir-run-truth-suite-2026-07-11.md` (Correctness; thin
first tangible win).

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

1. This Definition.
2. `docs/definitions/choir-run-truth-suite-2026-07-11.md` (suite sequence).
3. `docs/ACTIVE.md` Remaining Error evidence.
4. `docs/definitions/choir-run-lifecycle-and-completion-authority-2026-07-11.md`
   (inherits deadline semantics; does not execute full lifecycle here).
5. `docs/standing-questions.md` (Q5, Q9), `AGENTS.md`, `docs/choir-doctrine.md`
   (H019: no new lease vocabulary).
6. Observed source: `internal/runtime/runtime.go`, `cmd/choir`.

## Settled Inputs (do not re-litigate)

- Single lifecycle authority default is `RunRecord.State` (suite member 3 owns
  full projection alignment; this mission only needs terminal transitions that
  release admission).
- Doctrine rejects lease as a control concept (H019). New identifiers use
  **progress deadline** / **activation budget** only.
- Continuation / parent-child / channel deletion is out of scope (og-dolt).

## Mission Purpose

1. Add a processor progress deadline (default **60 minutes**) and a runtime
   sweep that transitions deadline-expired `RunRunning` → `RunFailed` with
   admission release.
2. Expose `choir run cancel <id>` with the same capacity-release semantics.
3. Drain the stuck staging run; prove `running_runs: 0` and a green
   `Deploy to Staging (Node B)` hot-refresh on the next runtime-package push.

## Mission Non-Purpose

- No admission counter rewrite, retry policy, or artifact-verified completion
  (suite member 3).
- No wire-store conformance (suite member 2).
- No vocabulary purges or renames (suite member 4 / og-dolt E).
- No VM reprovisioning; instance name `vm-universal-wire-platform` stays.

## Open Decisions (defaults govern; deviations recorded)

- **Progress deadline:** 60 minutes for processor runs (pinned).
- **Cancel path:** `choir run cancel <id>` is the operator drain; same terminal
  semantics as deadline expiry (`RunFailed` or `RunCancelled` — default
  `RunCancelled` for explicit cancel, `RunFailed` for deadline).

## Invariants

- No `RunRunning` occupies admission indefinitely without observable progress.
- New code must not introduce lease-named control identifiers.
- Acceptance uses product CLI/API only (no SSH).

## Completion Semantics

Complete when all are observed on staging:

1. Deadline sweep and/or `choir run cancel` can move a `running` run to a
   terminal state that releases admission.
2. The stuck `vm-universal-wire-platform` run is drained (`running_runs: 0`).
3. A subsequent runtime-package push gets a green `Deploy to Staging (Node B)`
   hot-refresh verifying the new commit.
4. Regression coverage exists for deadline expiry and cancel → capacity release.

## Sequencing and Gates

Single-phase red mission. Gate protocol:

1. **Consensus** before mutation (`/tmp/choir-deploy-unblock-consensus`).
2. **Local proof:** `go test ./internal/runtime ./cmd/choir -run 'Deadline|Cancel|Sweep'`.
3. **Landing loop** per `AGENTS.md` (push `origin main`, monitor CI + deploy).
4. **QA:** drain stuck run; prove Deploy green; record run IDs and Deploy URL.
5. **Halt:** failed consensus, red CI, identity mismatch, or QA failure →
   document-and-stop (or repair within mission); roll back to the pre-mutation
   SHA recorded at phase start. Do not leave staging red.
6. **Rollback ref:** `git rev-parse HEAD` immediately before the first behavior
   commit of this mission; record it in the evidence ledger.

## Execution

### Phase 0 — Consensus (green/yellow)

Consensus on this Definition; record ledger entry; no code mutation.

### Phase A — Deadline, cancel, drain, Deploy proof (red)

- Implement progress deadline + sweep in `internal/runtime`.
- Implement `choir run cancel` in `cmd/choir`.
- Land; drain staging; prove Deploy.

On success: update `docs/ACTIVE.md` Remaining Error (remove or mark resolved
the stuck-run Deploy failure); mark this Definition complete; suite advances to
wire-store.

## Follow-on

- Suite member 2: `choir-wire-store-conformance-2026-07-11.md`
- Suite member 3 inherits that deadlines/cancel exist; it must not re-litigate
  them unless staging regresses.

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
