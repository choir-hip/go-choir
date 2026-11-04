# A frozen candidate dies with its build's deadline (2026-11-04)

Status: open. Mutation class of the fix: red (self-development operation
fate, the assignment cancellation choke point, management's Verify gate).
This doc is green.

Owner ruling, 2026-11-04: waits are to be eradicated, and work is never to
be lost to a wait.

## Evidence

Luna test computer computer-2bf3cab1e3ebf3eb24a515c4603cf954 (VM
vm-774a03f85439cd8ab1b726c01f248b21), trajectory
176a4ac9-34f8-5c04-84a1-7c32f7fd7e75, assignment
assignment-37621a331407861f3707e875, capsule
capsule-c522d7a1-a70b-5424-b809-33b3458e61b0, operation
selfdev-27a4897e82a5411739247ab7962de7e1. Sources: the guest console log
on Node B (`vm-state/<vm>/console.log`) and the Node B journal (gateway,
vmctl). No owner credentials were used; the computer was only read.

| time (Z) | event | source |
|---|---|---|
| 02:34:19 | cast preflight, admission, carried patch | console `capsule ... saga phase=` |
| 02:34:52 | capsule spawned (29.8 s), bound, run activated | console `saga phase=activate` |
| 03:22:53 | gpt-6-luna call returns the Freeze cell | gateway |
| 03:22:56 | Freeze commits: operation evidence embedded on the Texture doc, precommits tray-1..3 `settled yes at freeze`, tool loop iteration 79 | console |
| 03:22:56 | the engineer's next model call (gpt-6-luna, xhigh, 171 messages) | gateway |
| 03:32:56 | that call fails `chatgpt: read stream: context deadline exceeded`: the gateway's own 10-minute inference bound (`internal/gateway/handlers.go` `inferenceTimeout`) | gateway |
| 03:32:57 to 03:33:51 | the retry succeeds (54 s); iteration 80 | gateway, console |
| 03:34:18, 03:34:45 | iterations 81 and 82, one tool each; no terminal fate committed | console |
| 03:34:52 | the next model call is `context canceled`: 02:34:52 plus the 60-minute activation budget | gateway |
| 03:34:53 | capsule teardown | console |
| 03:34:54 | precommits tray-4, tray-5 `settled no at terminal cancelled` | console |
| 03:34:58 | operation failed, `context deadline exceeded` | operation record (mission brief) |

Answers, from the trace:

- **Nothing of the decision ran between the Freeze and the kill.** No
  judge, screenshot judge, Verify or owner gate appears in the console or
  the journal. The only work was the engineer's own tool loop after the
  Freeze, and 10 of its 12 minutes were one model call that hung until the
  gateway's 10-minute per-call bound.
- **Why the decision never started.** management's Verify refuses any
  assignment that is not `completed` (`commitVerifyIntent`,
  internal/agentcore/rlm_reduce.go: "only a completed assignment's
  candidate is verified"). The assignment completes only when the engineer
  commits `choir.Complete` after the Freeze. So the decision phase waits on
  the build's run, and the build's run carries the build's wall clock.
- What the engineer did in iterations 80 to 82 is not in the console (the
  cell contents are on the trace, which needs the computer owner's key).
  HYPOTHESIS: it was preparing or attempting Complete. FINDING: no terminal
  fate committed before the budget.

## Root cause, in code

1. `ExecuteActivationSync` (internal/agentcore/runtime.go) runs every
   activation under `context.WithTimeout(ctx, rt.activationBudgetForRun(rec))`;
   the budget is `provideriface.DefaultActivationBudget`, 60 minutes. The
   engineer's run keeps that context across its Freeze.
2. At expiry the loop returns `context deadline exceeded`.
   `handleExecutionError` sends an assigned engineering run to
   `terminalizeRun(..., err.Error())`, so the reason is the raw
   `context deadline exceeded`.
3. `terminalizeRun` -> `cancelBoundEngineeringRun` ->
   `persistSystemEngineeringCancellation` -> `failBoundSelfdevOperation`,
   which fails the operation from any pre-terminal state, `frozen`
   included. The same happens at the six-hour assignment deadline
   (`HandleAssignedEngineeringFateDeadline` -> `cancelAssignedEngineering`).

So a candidate that froze inside the build's budget still dies with that
budget. Freeze does not end the build budget, and the decision has no
budget of its own: it is not even allowed to start until the build's run
reports.

## Substrate vs symptom

Substrate: the operation's fate is joined to the run's wall clock at the
cancellation choke point. The slow model call is a symptom (any 12 minutes
of post-Freeze work would do it). No existing mechanism separates them:
the carry (carried_patch.go) offers a failed candidate's patch to a later
round, but only if a later round is cast, and the decision is still lost.

## Fix shape

- Freeze ends the build budget. When the build's run ends by its
  activation budget or the assignment deadline and its operation is
  `frozen`, the operation is retained (not failed); the assignment is
  cancelled with a reason that names the retained candidate and its
  bundle, and that reason is the final Report management receives.
- management's Verify admits a retained candidate (a cancelled
  assignment whose operation is still `frozen`).
- The wait for the decision is a typed wait (`candidate_decision`) with
  its own budget, in management's pack and as a Note to the Texture the
  round serves. When the budget runs out the candidate stays frozen and
  retained; the wait says so ("at its bound").
- A crash restart closes a retained candidate (the restart rule).

## Residuals

- A provider call can still hang for the gateway's full 10 minutes;
  that is a separate wait (the gateway bound), not addressed here.
- Other ends of the build's run after Freeze (a provider failure, a run
  that ends without Complete) still fail the candidate.
