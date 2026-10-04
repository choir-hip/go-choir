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

## Mechanism (confirmed — ops launch carries no freeze authority)

`choir.Freeze` exists (`internal/yaegikernel/choir.go:507`) and the
reducer commits it via `commitFreezeIntent`
(`internal/agentcore/rlm_reduce.go:1292`), which calls
`freezeCapsuleEffectBundle` against `r.toolCtx.OperationStore`.

`OperationStore` is populated on exactly one path:
`assignedEngineeringCapsuleToolCtx`
(`internal/agentcore/engineering_assignment_tools_overlay.go:79`), and
that builder only runs when `assignedEngineeringToolOverlay` binds —
which requires `rec.Metadata["assignment_id"]` +
`["assignment_attempt"]` to name a **bound Engineering assignment**
(`:24-31`). A `POST /api/computers/{id}/self-development/operations`
creates a document lifecycle and a desk run with `trajectory_id` set
(`selfdev_texture_join.go:53-88`) but NO engineering assignment — the
metadata keys are never stamped. The run either fails the overlay
(`"unassigned Engineering cannot execute"`) or starts with
`toolCtx.OperationStore = nil`; either way `choir.Freeze` inside the
cell returns `"freeze intent without assignment authority"` and the
op can never leave `executing`.

The cell refusal surfaces only as a `capsule_go_eval` result value —
not a tool-call error, not an op transition — so the model retries the
same eval and the loop never converges. The refusal reason never
leaves the cell.

**Run 1** (vague prompt): model never called `capsule_go_eval` —
declined.
**Runs 3/4** (directive prompts): 18+ `capsule_go_eval` calls —
the cell ran, `choir.Freeze` refused on nil OperationStore, the model
retried, the loop never converged.

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

1. **Launch Go-effect probes through an engineering assignment, not a
   raw ops POST.** The ops path creates a document lifecycle but no
   `assignment_id` metadata, so `assignedEngineeringToolOverlay` never
   binds `OperationStore`. A self-dev Go effect requires the M11
   assignment trajectory: management casts an engineering assignment →
   the assigned run spawns with `assignment_id` + `assignment_attempt`
   stamped → `choir.Freeze` resolves the op via `trajectoryIDForRun`.
   The ops POST is the wrong surface for an effect probe; it can
   propose a document but can never freeze.
2. **Surface cell refusals on the op record.** When `choir.Freeze` or
   `choir.Verify` refuse inside `capsule_go_eval`, promote the refusal
   reason to `operation.terminal_error` (or a new `last_intent_error`
   field). A refused authority is the op's real blocker, not a normal
   cell error.

The state machine is correct; the ops→cell binding is not.

## Verification

Re-run the Go effect through an engineering-assignment launch (management
desk casts `kind=engineering` → bound capsule run). Passing = op reaches
`awaiting_approval` with `bundle_digest` set and a committed
`freeze_capsule_effect_bundle` event. Failing = op persists `executing`
with `verifier_refs` still empty or the refusal reason absent.
