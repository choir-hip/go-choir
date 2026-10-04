# S0 finding: self-development op sits in `executing` after the model's turn ends — no-op proposals are indistinguishable from a transport wedge

**Date:** 2026-10-04
**Status:** open. Observed on staging; no guest-side root cause yet (host
journals silent — the wedge is inside the guest runtime).
**Mutation class of this record:** green. Fix is `red` (runtime selfdev
state machine).
**Station receiving it:** S0 evidence set. Re-run precondition for the
S2 builder-substrate decision (Go-effect execution is blocked on this).

## Evidence

Operation `selfdev-0280c6cf2eac90dffb5c77912c0766a9` (run 1, vague
prompt) on `computer-a99366facf24b872703de326d3b33832` and
`selfdev-772a70c9bc63da3197c39a9ba8ae798e` (run 3, directive prompt) on
`computer-efef241fa2a8652758e4105e89412a76`, both armed `propose_only`.

- Gateway log, run 1: ten inference rounds
  (`provider=opencode-go model=deepseek-v4.1-flash`, messages climbing
  5→17), last round `07:29:57` returning `text_len=93` — a terminal
  non-tool answer. No `freeze_capsule_effect_bundle` or
  `record_self_development_verification` call was ever issued.
- Gateway log, run 3: 18+ rounds (messages climbing 11→37), `tools=1`,
  `text_len=0` every response — the model IS issuing tool calls but the
  call never satisfies; the loop is a retry, not a converge.
- Op record: `state=executing`, `verifier_refs=[]`, `bundle_digest=null`,
  `error=null`, `updated_at` never advanced past `created_at`.
- Guest `/health`: `running_runs=0`, `running_processor_runs=0` — the
  model's turn is over; nothing is doing work.
- `replay-completeness` is `equivalent` (91 events applied, zero gaps) —
  the tape is not the blocker.

## Mechanism (ledger-confirmed — the freeze intent is unreachable for an eval-only probe)

Guest tape (`state-actor.db` on the dead VM's `data.img`, mounted
read-only) shows the full chain **works as designed**:
ops POST → `trajectory_started` → `lifecycle_work_assigned` to the
engineering desk → desk casts `assignment-e69fd5d6` → capsule worker
`run:assignment-…` spawned (`initial_dispatch` → `attempt-2` after the
platform restart). The assignment machinery is intact, and the worker
run does carry the bound capsule context (the earlier
"OperationStore unbound" hypothesis is hereby corrected).

The wedge sits at the end of the chain: the capsule worker ran ~30+
`capsule_go_eval` cells over two attempts (each hitting
`cell_terminal_deadline` ≈5 min) and never staged `choir.Freeze`.
Two compounding causes:

1. **Freeze is semantically unreachable for an eval-only probe.**
   `Tray.Freeze(buildRecipeRef, testReceipts, dependencyToolchainRefs)`
   (`internal/yaegikernel/intent.go:229`) only *stages* the intent; the
   reducer rejects it unless all three are non-empty
   (`rlm_reduce.go:123`: "freeze requires build recipe, test receipts,
   and dependency/toolchain refs"). A Go-eval health probe produces
   none of these — no recipe, no test run, no toolchain. The
   acceptance contract "a safe capsule Go effect runs" cannot be met
   by any prompt, because the effect path *requires* a build context
   a marker/eval probe structurally does not have.
2. **Cell refusals never reach the op record.** A rejected or
   unstageable `Freeze` is a cell-level result; the worker retries
   evals until `cell_terminal_deadline`, parks, and the op stays
   `executing` — `verifier_refs=[]`, `error=null`, `updated_at`
   frozen. "Worker declined", "freeze rejected in reduce", and a
   wedged transport are indistinguishable on the op record; only the
   guest actor DB and gateway logs distinguish them.

**Runs:** `selfdev-0280c6cf` (vague prompt, worker declined),
`selfdev-772a70c9` (directive, parked at platform restart →
attempt-2), `selfdev-39ce659c` (directive naming `choir.Freeze` with
empty refs — unfulfillable by design).

## Why it matters

- **S2 builder-substrate decision is blocked on a real Go effect.** The
  `s0b-builder-substrate-decision` receipt names this op as the Go-effect
  re-run; its wedge means the acceptance contract ("the runtime executes
  the intended payload," S0 finish:339-340) still has zero runtime
  observation. `send_back` panelists named exactly this.
- **The wedge is silent.** No error, no failed transition, no guest log —
  an operator sees `executing` forever. That is a worse failure than a
  noisy error: the honest "proposal produced nothing committable" state
  has no representation.

## Fix direction

1. **Make the probe fulfill the contract or split the contract.** Either
   (a) the Go-effect probe must produce a freezeable artifact —
   buildRecipeRef + real test receipts + toolchain refs — i.e. become a
   real capsule change with build context, not a bare eval; or (b) the
   selfdev pipeline needs a lightweight effect path for eval-type
   probes whose output is a receipt, not a bundle. Option (a) is the
   faithful test for S2: "the runtime executes the intended payload"
   means a real change with a real recipe.
2. **Surface cell refusals on the op record.** When `choir.Freeze` or
   `choir.Verify` fail validation or authority inside
   `capsule_go_eval`, promote the reason to
   `operation.terminal_error`/`last_intent_error`. A refused authority
   is the op's real blocker, not a normal cell error.
3. **Op-level proposal timeout.** An op whose worker runs N cells
   without staging a freeze should transition `executing → failed`
   (`proposal_timeout`) instead of parking forever.

The state machine and the assignment machinery are correct; the probe
contract and the error surface are not.

## Verification

Re-run the Go effect through an engineering-assignment launch (management
desk casts `kind=engineering` → bound capsule run). Passing = op reaches
`awaiting_approval` with `bundle_digest` set and a committed
`freeze_capsule_effect_bundle` event. Failing = op persists `executing`
with `verifier_refs` still empty or the refusal reason absent.
