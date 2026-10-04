# S0 finding: self-development operation wedges in `executing` after the proposal loop ends

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

- Guest ran ten gateway inference rounds
  (`provider=opencode-go model=deepseek-v4.1-flash`, messages climbing
  5→17), last round `07:29:57` returning `text_len=93` — a terminal
  (non-tool) response.
- The operation record stayed `state: executing` for ≥16 minutes past
  that round with `verifier_refs: []`, `bundle_digest: null`,
  `error: null`, `updated_at` never advancing.
- No guest-side `running_runs`/`running_processor_runs` — the loop is not
  doing work; it is wedged between proposal-end and bundle-commit.
- `replay-completeness` is `equivalent` (91 events applied, zero gaps) —
  the tape is not the blocker; the op-state transition is.

## Mechanism (source-traced, not runtime-verified)

`internal/selfdev/operations.go` state machine:

```text
executing -> frozen (tools_capsule.go:367 FreezeGrantedWorktree commit)
frozen    -> verified (tools_capsule.go:604)
verified  -> awaiting_approval (tools_capsule.go:488)
```

The op is stuck at `executing` with `capsule_id` unset and `bundle_digest`
null — the freeze never committed. The last gateway round at `07:29:57`
returned a terminal non-tool response (`text_len=93`), which is where the
model's turn ended; the executor-side `Freeze` + op-record transition that
should have followed produced no journal entry and no error. Either the
freeze call never returned, or it returned before the
`Transition(executing, frozen)` write — both paths are silent from the
host's view.

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

Either (a) transition `executing → failed` with a recorded reason when
the proposal loop ends without a bundle, or (b) commit a
`proposal_uncommitable` marker so the op state is honest. (a) is simpler
and matches the existing failure surface. Guest-side instrumentation is
the real gap: the wedge is invisible because the loop's exit path is not
logged.

## Verification

Re-run the same op on a fresh disposable. Passing = op reaches
`awaiting_approval` or `failed` with a recorded reason within the
proposal timeout. Failing = `executing` persists past the last gateway
round with `verifier_refs` still empty.
