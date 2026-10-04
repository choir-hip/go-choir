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

## Mechanism (sharpened — tool-call loop never converges)

`freeze_capsule_effect_bundle` is a tool call, not an automatic step
(`tools_capsule.go:367` runs only when the model invokes it). Two runs
on two disposables show the loop's two failure shapes:

**Run 1** (op `selfdev-0280c6cf…`, vague prompt): 10 rounds, ended on a
terminal non-tool answer — model declined to call the tool. Op stays
`executing`.

**Run 3** (op `selfdev-772a70c9…`, directive prompt): 18+ rounds with
`tools=1`/`text_len=0` — the model IS calling the tool repeatedly, but
the call returns an error it retries instead of escalating. Op stays
`executing` past 15 minutes. Likely cause: `freeze_capsule_effect_bundle`
requires a worktree handle + build recipe ref the model cannot produce
because nothing in the loop granted a writable worktree or authored a
change — the model has no prior `capsule_write`/`edit` to freeze.

The state machine is honest: `executing` is correct for both — a
declined proposal and an error-retry loop are both genuinely still
executing. The defect is that **neither shape is distinguishable from
outside**: `verifier_refs=[]`, `error=null`, `updated_at` frozen. An
operator cannot tell "model declined" from "model is retrying a
doomed tool call" without reading gateway logs.

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

This is not a transport bug to fix but a visibility gap to close: the op
state machine honestly records that the model's turn ended, and it stays
`executing` because no terminal transition is warranted (the model may
still call the freeze tool on a future turn — there is no op-level
timeout that makes "model declined" a terminal verdict). The repair is
instrumentation: log the proposal loop's terminal tool-name (or
"end_of_turn_no_tool") so a no-op proposal is distinguishable from a
wedge without reading gateway logs, and surface `model_turn_ended` on
the op record. The state machine itself is correct.

## Verification

Re-run the same op on a fresh disposable. Passing = op reaches
`awaiting_approval` or `failed` with a recorded reason within the
proposal timeout. Failing = `executing` persists past the last gateway
round with `verifier_refs` still empty.
