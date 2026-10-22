# FP4 freeze close, failure branch: deployed proof on a test computer (2026-10-22)

Run by the self-development monitor (lane/fp4-selfdev-monitor) for FP0, after
deploy I (4c501bae6, which carries f68100cf6). It follows the earlier drill,
docs/evidence/fp4-freeze-close-deployed-proof-2026-10-22.md, which exercised
only the completed-assignment branch. Target: the test computer
`computer-71afbf1461dffabcce8eeebe558d03bd` (vm-c5c801f8…, warmness `test`),
and no other computer. No apply is approved.

Under proof: `closeEndedAssignmentSelfdevOperation`
(internal/agentcore/engineering_assignment_fate.go). An assignment whose
own report ends it `failed` while its operation is still `executing`
(after a refused freeze) closes the operation `failed`, with the terminal
error "assignment failed before its candidate froze; last refusal: freeze:
…".

## Plan (written before any act)

1. Wait for deploy I's `deploy refresh caller=deploy` round, and confirm
   the computer serves 4c501bae6 or a descendant of f68100cf6.
2. Clear the open candidate `selfdev-bb0cebfa9edc224e29045f4921763db9`
   (`awaiting_approval`). Reject it, as the test computer's owner, through
   the product path (`choir self-dev reject --reason …`, which is
   `POST …/self-development/operations/<id>/decision` with
   `decision: reject`). Never approve it. With it open, a new freeze is
   refused `ErrOperationOpen` (FP4's R3 case), and that would confound
   the drill.
3. Cast the request below through the prompt bar (`choir run start`).
4. Expected:
   - the assignment reports `failed`;
   - its operation closes `failed` with "assignment failed before its
     candidate froze; last refusal: freeze: capsule freeze: computer-surface
     frontend artifacts are underivable";
   - obligations selfdev reads busy 0, stalled 0;
   - /health reads no `selfdev_active_operations`, the field vmctl's busy
     probe reads (internal/vmctl/ownership.go:3459-3476);
   - the next vmctl refresh of this computer is not deferred by the
     operation.
5. Budget: about 40 minutes of wall time from the cast. If the worker does
   not give up, or management re-casts, the drill stops: I record it here
   and cancel nothing on my own.

## Mechanism and reasoning

The operation must open first. Only a refusal after that point leaves the
operation `executing` with a stamp for the close to carry. Read in code
(origin/main 97de4db0b; deploy I changes only the cancellation close in engineering_assignment_fate.go):

- `choir.Freeze` stages without checking its references
  (internal/yaegikernel/intent.go:437).
- The reducer's freeze body, `freezeCapsuleEffectBundle`
  (internal/agentcore/tools_capsule.go:156), refuses only two things
  before it opens the operation:
  - an empty build, test or toolchain slot;
  - a job still running.
- It then opens the assignment's operation (`OpenFreezeCandidate`, tools_capsule.go:189).
- Every later refusal leaves the operation `executing`, and the reducer
  stamps `last_intent_error` as "freeze: <error>"
  (internal/agentcore/rlm_reduce.go:1465). Those refusals are: receipt
  resolution, the distinct slots, the receipt outcomes, the classifier,
  staging the release and the source-change check.

The refusal chosen is the one the first drill met naturally:

- "capsule freeze: computer-surface frontend artifacts are underivable"
  (internal/capsule/executor.go:1866).
- A release with no `frontend/` file can never freeze.
- The capsule is fresh, so it has no frontend build unless the worker runs
  one (in the drill, `vite build` under `choir.Start`).

The request therefore forbids every build and names three cheap
receipts:

- toolchain: `go version`, `node --version` and `npm --version`;
- check: `gofmt -l` on the one file;
- build stand-in: a `test -s` on the file.

It then asks for one Freeze and, on refusal, a failed report with no
retry. Why this mechanism:

- **Harmless.** The change is one comment, inside a capsule on a test
  computer. Nothing can freeze, so nothing reaches a decision or an
  apply.
- **Cheap.** No go vet and no go build, which avoids the cold-cache
  module download that took 9 min 43 s last time. There is no npm, and
  only a handful of cells.
- **Test-only.** The prompt names itself a drill. It asks management not
  to re-cast, so no second assignment and no second operation.
- **Robust to how the worker reads it.** It does not depend on the worker
  citing receipts correctly. If it cites a slot wrongly, the
  distinct-slot or outcome refusal still comes after the operation opens,
  so the stamp is still there. Only the refusal text would differ.

Risks:

- Management may paraphrase the constraint away when it casts (it
  paraphrased the first drill's request). If the worker then builds the
  frontend, the freeze succeeds. The drill records that and stops
  without approving.
- The worker may give up with `blocked` or `partial` instead of `failed`.
  Those are recorded as they come.

## The request (sent verbatim through the prompt bar)

> Engineering freeze-failure drill FP4FF-20261022 (test computer only; it
> exercises the failed-before-freeze path, so nothing should freeze, be
> approved or be applied). Through the engineering desk, in one assignment:
> 1. In internal/selfdev/operations.go add the comment line "// Operation
> is one self-development change, from freeze to its decision." directly
> above "type Operation struct".
> 2. Record three cheap receipts, each in its own cell: the toolchain
> (`go version`, `node --version`, `npm --version`); the check
> `gofmt -l internal/selfdev/operations.go` (no output expected); and, as
> the build stand-in, `sh -c 'test -s internal/selfdev/operations.go'`.
> 3. Do not run go vet, go build, npm or vite. This drill keeps the capsule
> free of frontend and binary build artifacts on purpose.
> 4. Call Freeze exactly once, citing those three receipts. It is expected
> to be refused, because the release has no frontend artifacts.
> 5. When Freeze is refused, do not retry and do not work around it:
> Complete the assignment as failed, quoting the refusal text.
> Management: cast this once and do not re-cast or retry; report the
> failure back to me as the result.

## Acts and results

(Filled in as the drill runs.)
