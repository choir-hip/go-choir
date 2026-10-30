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

## Mechanism (substrate confirmed — cells that succeed are durably fated `timeout`)

Two independent tapes now converge on one substrate bug plus one
contributing contract gap:

**Tape 1 (earlier disposables, `efef241f`):** the ops POST → doc → cast →
worker chain works as designed; the worker ran ~30+ evals and never
staged a contract-complete `choir.Freeze` because the probe prompts
forbade the writes and Execs that produce the receipt refs
`freezeCapsuleEffectBundle` requires.

**Tape 2 (probe 5, `computer-48344a6711dcb149ef0a4c7b680844d0`, prompt
with exact contract):** the worker's `capsule_go_eval` cells succeeded —
**33 execution receipts persisted under `capsule-artifacts/receipts/
execution/` on the guest disk, every one `exit 0`** — yet the guest
actor DB shows **33 `cell_terminal_deadline` wakes, one per cell**, and
the op never left `executing`. Successful cells were durably fated
`timeout` before their commit could land.

**Substrate (fixed):** `armCellTerminalDeadline` was armed with
`evalCtx` — the context **already wrapped by the model's `timeout_ms`**
(`tools_capsule.go:686`, and the same pattern in `tools_desk.go:187`) —
not the cell's activation-bounded context. Any eval that outlived its
own `timeout_ms` was fated `timeout` and its commit/receipt result
masked, so the model retried until the assignment attempt parked. The
durable fate wake must come from the activation deadline; the
`timeout_ms` is only the eval's own watchdog.

**Contract (prompt, not code):** `choir.Freeze` requires ≥3 distinct
receipt refs that bind (run, handle, capsule, source snapshot) and whose
**chronologically latest** worktree digest equals the frozen tree —
all receipt-carrying Execs must run *after* the change is written
(`ResolveGrantedExecutionReceipts`, `capsule/executor.go:1037-42`).
Earlier probe prompts named `choir.Freeze` with empty args; that is a
prompt defect, not a platform refusal.
**Runs:** `selfdev-0280c6cf` (vague prompt, worker declined),
`selfdev-772a70c9` (directive, parked at platform restart →
attempt-2), `selfdev-39ce659c` (directive naming `choir.Freeze` with
empty refs — unfulfillable), `selfdev-b02e316c` (contract-complete
prompt on a fresh disposable — 33 `exit 0` evals, all fated `timeout`
by the deadline bug, op still `executing`).

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

   Contract details confirmed from source while authoring probe 5:
   every `choir.Exec`/`capsule_go_eval` call returns a `receipt_ref`
   (`capsule-exec:sha256:*` / `capsule-go-eval:sha256:*`,
   `capsule/executor.go:752,813`); `choir.Freeze` requires ≥3 distinct
   refs that bind (run, handle, capsule, source snapshot)
   (`grantedExecutionBindReason`, `capsule/granted_receipt.go:12`) and
   the chronologically **latest** cited receipt's `WorktreeDigest` must
   equal the frozen tree — so every receipt-carrying Exec must run
   **after** the change is written, with no writes between them
   (`ResolveGrantedExecutionReceipts`, `capsule/executor.go:1037-42`).
2. **Surface cell refusals on the op record** *(landed this session).*
   `selfdev.Operation` gained `last_intent_error`;
   `commitFreezeIntent`/`commitVerifyIntent` stamp the refusal (and a
   success clears it) via `Store.RecordIntentError`. A refused authority
   is the op's real blocker and is now owner-visible.
3. **Cell terminal deadline must arm from the activation ctx**
   *(landed this session).* `armCellTerminalDeadline(evalCtx, …)` →
   `armCellTerminalDeadline(ctx, …)` in `tools_capsule.go` and
   `tools_desk.go`. The model's `timeout_ms` bounds the eval, never the
   cell's durable fate.
4. **Op-level proposal timeout.** An op whose worker runs N cells
   without staging a freeze should transition `executing → failed`
   (`proposal_timeout`) instead of parking forever. The existing
   `engineeringMaxRecastAttempts` cap only bounds restart recasts; a
   worker that never freezes still loops within an attempt until the
   deadline refactor below makes its cells honest.

The state machine and the assignment machinery are correct; the
substrate deadline bug and the error surface were not.


## Resolution (2026-10-04, deployed fix `f61de45b`)

The wedge is gone and the Go-effect plane is proven end-to-end on staging.

- **Substrate fix deployed:** `armCellTerminalDeadline` arms from the
  activation ctx (commit `f61de45b`, CI + staging verified);
  `last_intent_error` surfaces every refusal.
- **Probe 6** (`computer-59addaef`, op `selfdev-a4baaa4262e266cab3d8064950e71f6c`):
  `freeze: distinct build, test, and dependency/toolchain execution receipts
  are required` — wedge gone, contract gap exposed.
- **Probe 7** (`computer-9dcde912`, op `selfdev-861f7fd312e2f588f8943816852539a7`):
  `freeze rejected: unknown paths` — root-path write rejected.
- **Probe 8** (`computer-d03eb970`, op `selfdev-c82335d383ae310d4d2a24eb31a99eab`):
  `freeze: no frozen runtime artifacts` — release needs
  `/var/lib/artifact/release/` layout.
- **Probe 9** (`computer-70a23d69`, op `selfdev-b72a48565061c35c0a22246cb6fc06c3`):
  **FROZEN, `bundle_digest=9d2be524fa6657b6`, `capsule_id` bound** — the
  Go effect executed, staged a valid release, and froze on the deployed
  fixed SHA.

### Full contract for a successful freeze (confirmed against deployed state)

1. Change lands under a classifying ledger prefix (`/workspace` = LedgerSource).
2. Runtime artifacts staged under `/var/lib/artifact/release/`:
   executable `bin/autoputer` + `frontend/` content.
3. All writes complete BEFORE the receipt-carrying calls; no writes between them.
4. Three DISTINCT `Exec`/`go_eval` `receipt_refs` to `Freeze` — the latest
   receipt's `WorktreeDigest` must equal the frozen tree.

Remaining edge (next station): `frozen -> verified` needs an independent
`EngineeringAssignmentVerification` run + owner `awaiting_approval`; the
wedge that blocked the effect plane is closed.
## Verification

Re-run the Go effect through an engineering-assignment launch (management
desk casts `kind=engineering` → bound capsule run). Passing = op reaches
`awaiting_approval` with `bundle_digest` set and a committed
`freeze_capsule_effect_bundle` event. Failing = op persists `executing`
with `verifier_refs` still empty or the refusal reason absent.
