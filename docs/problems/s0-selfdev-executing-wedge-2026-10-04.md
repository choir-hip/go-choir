# S0 finding: self-development op sits in `executing` after the model's turn ends — no-op proposals are indistinguishable from a transport wedge

**Date:** 2026-10-04
**Status:** open. Observed on staging; no guest-side root cause yet (host
journals silent — the wedge is inside the guest runtime).
**Mutation class of this record:** green. Fix is `red` (runtime selfdev
state machine).
**Station receiving it:** S0 evidence set. Re-run precondition for the
S2 builder-substrate decision (Go-effect execution is blocked on this).

## Evidence

Operation `selfdev-0280c6cf2eac90dffb5c77912c0766a9` on disposable computer
`computer-a99366facf24b872703de326d3b33832`, armed `propose_only` at
`2026-10-04T07:28:53Z`.

- Gateway log: ten inference rounds
  (`provider=opencode-go model=deepseek-v4.1-flash`, messages climbing
  5→17), last round `07:29:57` returning `text_len=93` — a terminal
  non-tool answer. No `freeze_capsule_effect_bundle` or
  `record_self_development_verification` tool call was ever issued.
- Op record: `state=executing`, `verifier_refs=[]`, `bundle_digest=null`,
  `error=null`, `updated_at` never advanced past `created_at`.
- Guest `/health`: `running_runs=0`, `running_processor_runs=0` — the
  model's turn is over; nothing is doing work.
- `replay-completeness` is `equivalent` (91 events applied, zero gaps) —
  the tape is not the blocker.

## Mechanism (corrected — model agency, not a transport defect)

`freeze_capsule_effect_bundle` is a tool call, not an automatic step.
`internal/agentcore/tools_capsule.go:367` only runs when the model
invokes it; the verifier transition `:488` runs when the model calls
`record_self_development_verification`. The gateway log shows the model
ended its turn on a terminal non-tool response (`text_len=93`) after 10
rounds — it never called either tool, so the op legitimately stays
`executing` with `verifier_refs` empty. This is the same class as S0m's
`choir.Ask` non-call (recorded as an accepted edge there): the mechanism
is intact; the model declined to use it.

The honest "proposal produced nothing committable" state has no
representation: an op whose model never calls the freeze tool sits in
`executing` forever with `error=null`, indistinguishable from a wedged
transport. That is the real defect — not that the op didn't progress,
but that a no-op proposal and a wedge look identical from outside.

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
