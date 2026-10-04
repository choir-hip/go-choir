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

## Mechanism (root cause — freeze intent refuses inside the cell; error never reaches the op record)

`choir.Freeze` exists on the cell surface
(`internal/yaegikernel/choir.go:507`, staging `IntentFreeze`). The
reducer commits it via `commitFreezeIntent`
(`internal/agentcore/rlm_reduce.go:1292`), which requires:

- `r.toolCtx.OperationStore != nil` (op-store authority on the run)
- `trajectoryIDForRun(r.rec)` resolves the op via
  `OperationStore.GetByTrajectory`
- `requireCapsuleMutationRole(ctx)` — the run must carry the author
  mutation role, not the verifier role.

The ops-API launch path (`POST /api/computers/{id}/self-development/
operations` → trajectory → guest run) is built for the M11 engineering
**assignment** trajectory. A raw ops launch may not bind
`OperationStore` on the run's `toolCtx`, or may not bind the op to
`trajectoryIDForRun(rec)`, so `choir.Freeze` inside the cell returns
`"freeze intent without trajectory binding"` /
`"without self-development operation authority"` as a cell result.

The model sees the refusal as an eval result and retries
`capsule_go_eval` — the cell error is never promoted to
`operation.terminal_error` because a failed cell is a normal run
outcome, not an op transition. All three runs wedge `executing`
because the model can call the tool surface but never satisfy the
binding precondition; and the refusal reason never leaves the cell.

**Run 1** (vague prompt): model never called the tool — declined.
**Runs 3/4** (directive prompts): model called `capsule_go_eval` 18+
times (`tools=1`, `text_len=0`) — the cell ran, `choir.Freeze` refused,
the model retried, the loop never converged.

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

1. **Bind the ops trajectory to the freeze authority.** The launch
   path must wire `OperationStore` into the run's `toolCtx` and bind
   the op's `trajectory_id` to `trajectoryIDForRun(rec)` so
   `commitFreezeIntent`'s `GetByTrajectory` resolves. If the ops
   launch is assignment-scoped by design, the Go-effect probe needs
   an engineering-assignment launch instead of a raw ops POST.
2. **Surface cell refusals.** When `choir.Freeze`/`choir.Verify`
   refuse inside `capsule_go_eval`, the reason must reach
   `operation.terminal_error` (or a new `last_intent_error` field) —
   a refused authority is not a normal cell error; it is the op's
   real blocker.

The state machine is correct; the ops→cell binding is not.

## Verification

Re-run the same op on a fresh disposable. Passing = op reaches
`awaiting_approval` or `failed` with a recorded reason within the
proposal timeout. Failing = `executing` persists past the last gateway
round with `verifier_refs` still empty.
