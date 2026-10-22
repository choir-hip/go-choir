# An owner's reject lands, but the decision API answers 409 "conflict" (2026-10-22)

Found 08:03 UTC by the self-development monitor, while clearing a drill
candidate on the test computer `computer-71afbf1461dffabcce8eeebe558d03bd`
(vm-c5c801f8…, build 43e318712). Problem documentation only. There is no
fix in this commit, and the cause is not yet known: the hypotheses below
are labelled as such.

## Evidence

- 08:03:09, `choir self-dev reject` (CLI built from a077df91e; the test
  computer's owner key, sent on stdin):
  - flags: `--computer computer-71afbf14… --operation
    selfdev-bb0cebfa9edc224e29045f4921763db9 --idempotency-key
    fp4ff-reject-bb0cebfa-20261022 --reason "FP4 drill: …"`;
  - the operation was `awaiting_approval`, read with `choir self-dev show`
    at 08:01:38;
  - result: exit 1, `http 409: {"error":"self-development operation
    conflict: state is rejected, expected awaiting_approval"}`.
- 08:03:2x, `GET …/self-development/operations` (host-internal, read-only)
  shows the decision landed:
  - state `rejected`, updated_at `2026-10-22T08:03:10.635103Z`;
  - decision_actor `c7f51f0f-…` (the test computer's owner);
  - decision_event `cdc10b03fc9d0a10487995e1b0e028f5e1b4de565bc31fcbefd919d79024d5bf`;
  - decision_receipt `01a162a2-ac78-7894-8a0b-913e8dc04f63`;
  - mode_receipt `ecf41698…`;
  - desired and effective head `eadaf178…`, equal to the computer's head
    (`choir self-dev head`, sequence 4815, no pending transition);
  - terminal_error null.
- The CLI sends one POST and has no retry (`selfDevDecide`,
  cmd/choir/commands.go). So a single request both landed the decision
  and answered 409.
- A replay with the same idempotency key was refused on the client:
  `operation … is rejected, not awaiting_approval`.
  `selfDevDecisionBinding` checks the state before it posts. A caller
  cannot use the API's idempotent replay path to learn that the first
  call succeeded.
- The guest's recent log (`GET /api/runtime/observability`) holds only
  boot lines. The console log stopped carrying runtime lines after its
  07:36 rotation, so I could not find a log line for this request.

## Code read (origin/main 4c501bae6)

`decideSelfDevelopment` (internal/agentcore/api_self_development.go) does
this, in order:

1. Appends the `effect_rejected` event.
2. Reads the finalized decision transition.
3. Moves the operation `awaiting_approval → rejected`
   (`selfdevOperations.Transition`).

If that Transition fails with `ErrConflict`, the handler re-reads the
operation. It returns 200 when the operation carries this exact decision:
- `DecisionEvent == eventDigest`;
- `DecisionReceipt == transition.Receipt.ReceiptID`;
- the state descends from the decision;
- `verifyFinalizedSelfDevelopmentDecision(current, transition)` passes.

The comment there names the expected racer, the post-commit observer's
`recoverSelfDevelopmentDecision`
(internal/agentcore/self_development_materializer.go:274). That function
writes the same fields from the same durable transition.

The 409 body is the Transition's own `ErrConflict` text
(internal/selfdev/operations.go:456 for the projection store). So the
Transition lost a race, and then the race check did not match. Which
comparison failed is not observable: nothing logs it.

## Hypotheses (not confirmed by trace)

- **H1.** Another writer reached `rejected` before the observer's fields
  were all visible. For example, a projection of the operation was
  derived from the tape, and the handler's re-read saw `DecisionReceipt`
  (or `DecisionEvent`) empty or different, which were filled in later.
  The values all match now.
- **H2.** The racer was `recoverSelfDevelopmentDecision`, but the handler's
  re-read went to a different `selfdev.Store` view than the one that
  wrote. Rematerialize swaps `rt.selfdevOperations`, as recorded in
  selfdev-freeze-failure-leaves-operation-executing-2026-10-22.md.
- **H3.** `verifyFinalizedSelfDevelopmentDecision(current, transition)`
  fails on the head or actor projection for a rejected operation in a way
  the observer's own conflict check does not reach. Not found in my read;
  listed for completeness.

## Harm

- The owner's decision lands, but the owner is told it conflicted. In the
  CLI the owner then cannot replay to confirm.
- This test computer was rejecting, which is harmless. On **approve** the
  same race would answer 409 for an accepted candidate that then
  materializes. An owner who believes the approval failed may approve
  again: that is refused, because the operation is no longer
  `awaiting_approval`. Or the owner may act on a computer they think is
  unchanged.
- Decision acceptance is a red surface (run acceptance, apply). A wrong
  status here misreports a protected transition.

## Next probes (for whoever takes the fix)

1. Log which comparison failed in the race branch of
   `decideSelfDevelopment` (and in `recoverSelfDevelopmentDecision`), with
   the operation and transition values. That turns H1-H3 into a trace.
2. Find who else moves an `awaiting_approval` operation to `rejected` on
   the event append path, and check each writer's field set against the
   handler's check.
3. Let the CLI's reject and approve replay the same idempotency key when
   the operation already carries a decision, so the API's `found` path can
   answer, instead of refusing on the client.
4. A test that has the observer win the Transition and asserts 200. The
   existing comment claims this case, so a test may already exist. If it
   does, it does not cover the shape staging produced.
