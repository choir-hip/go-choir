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

## Cause (read in code, origin/main 54040b730)

Two parts. The approve part is found in code. The reject part was not
found by reading. The race test found it: see "Cause confirmed by the
test" below. That cause hits both decisions. Parts 1-3 are kept as read,
because part 1 is a second, independent fault on approve.

**1. Approve: the race check refuses every operation that has started to
materialize (found in code).**

- The race branch in `decideSelfDevelopment`
  (internal/agentcore/api_self_development.go:606-617) re-reads the
  operation and runs `verifyFinalizedSelfDevelopmentDecision(current,
  transition)`.
- That verifier compares the operation's heads with the heads the
  decision produced, for every state past `awaiting_approval`
  (internal/agentcore/self_development_decision_binding.go:122-127,
  "operation head projection mismatch").
- The observer does not stop at `accepted`. `reconcileSelfDevelopmentMaterialization`
  recovers the decision (self_development_materializer.go:210) and, in
  the same pass, moves the operation to `materializing`. That move
  rewrites the heads to the head after `effect_started`
  (self_development_materializer.go:389). `applied` rewrites them again
  (:853).
- So when the observer wins and has started the apply, the four checks
  are: `DecisionEvent` equal, `DecisionReceipt` equal, the state check
  passes (`materializing` and `applied` descend from `accepted`,
  self_development_decision_binding.go:136), and the verifier fails on
  the heads. The handler returns the Transition's own `ErrConflict` text.
  The owner is told 409 for a candidate that is being applied.
- The same check breaks the idempotent replay of an approve. The `found`
  branch (api_self_development.go:356-370) runs the same verifier on the
  current operation. Once the operation is `materializing` or `applied`,
  a replay with the same key answers 409 "decision idempotency key
  reused with different binding".
- The release witness already knows this. It runs the verifier as the
  decision-time operation for exactly this reason
  (selfdev_release_witness.go:592-601: "once the operation has
  materialized those legitimately differ"). The decision path did not
  get the same treatment.

**2. Reject: not found in code.**

- On a reject the observer recovers the operation to `rejected` and stops
  (self_development_materializer.go:220). Nothing else writes a rejected
  operation's decision fields or heads.
- `recoverSelfDevelopmentDecision` (self_development_materializer.go:283-290)
  writes `DecisionEvent = transition.Request.EventDigest`. The handler
  checks that this equals `eventDigest` before its own Transition
  (api_self_development.go:585). It writes `DecisionReceipt =
  transition.Receipt.ReceiptID` from the same row,
  `FinalizedDecisionForOperation`, which is deterministic
  (internal/store/computer_event_recovery.go:12). It writes the same
  actor and heads.
- The projection writes the row in one statement
  (internal/store/project.go:424). `selfDevelopmentDecisionStateDescends("rejected",
  "rejected")` is true.
- So for an observer-written `rejected` row, all four comparisons match
  in code.
- One branch is silent and unexplained: the re-read's `getErr`. If the
  re-read `Get` fails, the branch falls through to the 409 with the
  Transition's text and logs nothing (api_self_development.go:607).
- H1 and H2 above stay open as hypotheses. H3 is answered for approve
  (part 1). It is not answered for reject.
- Discriminator: the new test forces the observer to win between the
  append and the Transition, for both decisions.
  - If the reject case passes before the fix, staging's reject took a
    path the test does not model.
  - The fix's log line then names the comparison that failed: get
    error, event, receipt, state, or verifier error. The next reject 409
    on a computer becomes a trace instead of a guess.

**3. The CLI cannot learn that its first call succeeded (found in code).**

- `selfDevDecisionBinding` (cmd/choir/selfdev.go:194) refuses any state
  other than `awaiting_approval` before it posts.
- It also builds the expected heads from the *current* head
  (selfdev.go:200-211). The decision event has moved that head. The
  server's replay match is exact on the original expected heads and
  commitments (`exactSelfDevelopmentDecisionRequestMatches`,
  api_self_development.go:647-656). So a replay built from today's head
  would be refused even if the client posted it.
- A replay has to carry the decision's original bindings. The CLI has no
  read for them today. The release witness exposes only the two expected
  event heads, not the pending ref or the state commitments
  (selfdev_release_witness.go:272).

## Cause confirmed by the test (2026-10-22, before any fix)

- The race test (`TestSelfDevDecisionObserverWinsRaceAnswersDecidedOperation`)
  lets the observer win between the append and the Transition.
- It reproduces staging's exact body for both decisions:
  - reject: `self-development operation conflict: state is rejected,
    expected awaiting_approval`;
  - approve: `… state is applied, expected awaiting_approval`.
- A temporary diagnostic in the race branch, not committed, printed:
  `getErr=sql: no rows in result set`, with an empty `DecisionEvent`,
  `DecisionReceipt` and state.
- **The cause.** The handler assigns the Transition's result to its own
  `operation` variable
  (`operation, err = h.rt.selfdevOperations.Transition(...)`,
  api_self_development.go:593 on 54040b730). On a conflict, Transition
  returns `Operation{}`, so `operation.OperationID` becomes `""`. The
  race branch then re-reads `Get(ctx, computerID, operation.OperationID)`
  (:607), which is a read of the operation with id `""`. That read always
  fails, and the branch falls through to the 409.
- So the race branch has never matched. The four comparisons were never
  reached. This is the "re-read `getErr`" branch named in part 2: it is
  silent and it always fails.
- `recoverSelfDevelopmentDecision` does not have this bug. It binds the
  Transition result to a new variable (`recovered, err :=`). That is why
  only the handler fails.
- On approve, even with the id fixed, part 1's head check would still
  refuse a `materializing` or `applied` operation. Both have to be fixed.
- H1 and H2 are not needed to explain the evidence. The hypotheses
  section is kept as written at the time.

## Fix (c4fdeb3cb, red; tests 90b10b2ab, yellow)

- **The race branch reads the right operation.** `decideSelfDevelopment`
  binds Transition's result to `decided` and no longer overwrites
  `operation`. The re-read after an `ErrConflict` uses the operation id
  the handler already holds.
- **One matcher, logged.** The handler's race branch and
  `recoverSelfDevelopmentDecision`'s conflict branch share
  `selfDevelopmentDecisionNotCarried` (self_development_decision_binding.go).
  It checks event, receipt, descending state, then the verifier, in that
  order. When the match fails, both writers log the first comparison that
  failed. The values logged are the operation's state, decision_event and
  decision_receipt, and this decision's event digest, receipt id and next
  state. No secrets.
  - The handler line starts `selfdev decision: operation <id>: <decision>
    lost its transition…`.
  - The observer line starts `selfdev decision recovery: …`.
- **Heads bind only in the decision's own state.**
  `verifyFinalizedSelfDevelopmentDecision` compares the operation's heads
  with the decision's next heads only while the operation is `accepted`
  or `rejected`. From `materializing` on, the materializer owns the heads.
  The event, receipt and actor joins stay exact in every state. This fixes
  part 1 for both the race branch and the approve replay.
- **Replay from the witness.** The release witness's `decision` now also
  carries `expected_pending_transition_ref`,
  `expected_desired_state_commitment` and
  `expected_effective_state_commitment` (additive JSON). With the two
  expected heads already there, that is the whole decision-time binding.
- **CLI replay.** `choir self-dev reject` and `choir self-dev approve` post
  a replay with the caller's key when the operation is already in the
  state that decision leaves it in:
  - reject: `rejected`;
  - approve: `accepted`, `materializing` or `applied`.

  The replay carries the landed decision's binding from the witness, not
  today's head. An approve replay never re-arms accept_once. Any other
  state is refused before any write, as before. So is a witness with no
  decision of that kind. The server's exact-match replay rule is
  unchanged: a different key, or the same key with a different reason,
  decision or binding, still answers 409.

## Tests

- `TestSelfDevDecisionObserverWinsRaceAnswersDecidedOperation`
  (internal/agentcore/selfdev_decision_observer_race_test.go), for reject
  and for approve. It runs on the P4 fixture: real appender, post-commit
  observer, materializer, mode CAS with owner acts. A nil-in-production
  seam lets the observer finalize the decision between the append and the
  Transition. For approve, the observer applies the candidate in that
  window. The test asserts:
  - the observer really won;
  - the handler answers 200 with the decided operation carrying the
    tape's decision event;
  - a same-key replay rebuilt from the release witness answers 200 and
    appends no decision;
  - a same-key replay with today's head, a different key, and the same
    key with a changed reason or decision all answer 409.

  Before the fix it fails with staging's exact body.
- `TestSelfDevDecisionReplaysALandedDecisionWithItsOriginalBinding`
  (cmd/choir/selfdev_test.go) checks that:
  - the replay posts the witness binding;
  - an approve replay does not touch the mode;
  - a reject of an applied operation, an approve of a rejected one, a
    decided state with no decision on the tape, `failed`, and `executing`
    are all refused with GETs only.
- Runs:
  - `GOMAXPROCS=2 GOFLAGS=-p=2 go test -race ./internal/agentcore -run
    'SelfDev|Decision|Selfdev' -count=1`: ok (119.6s);
  - `go test ./internal/agentcore -run
    'Witness|StandingConsensus|Materializ|Rollback|Effects|Release|Freeze|Capsule'`:
    ok;
  - `go test ./cmd/choir -count=1`: ok;
  - `go test ./internal/proxy -run 'SelfDev|Decision'`: ok;
  - `go vet ./internal/agentcore ./cmd/choir`: clean.

## Rollback

`git revert c4fdeb3cb`. No stored shape changed, and the witness fields
are additive. The test commit 90b10b2ab can stay. Its seam is nil in
production, and its tests would fail again after a revert, which is the
intended signal.

## Staging proof (FP0, test computer only)

A forced observer race is not cheap on staging. The cheapest real check
is a same-key replay of the landed reject. It needs the deployed guest
(the witness fields) and a CLI built from this commit.

1. Confirm the test computer runs the deployed SHA: health/build identity
   for `computer-71afbf1461dffabcce8eeebe558d03bd`.
2. Record `choir self-dev head --computer computer-71afbf14…` (sequence S).
3. Replay: `choir self-dev reject --computer computer-71afbf14…
   --operation selfdev-bb0cebfa9edc224e29045f4921763db9
   --idempotency-key fp4ff-reject-bb0cebfa-20261022 --reason "<the exact
   08:03:09 reason>"`, with the test computer's owner key on stdin.
   - Expect exit 0 and a body with state `rejected`, decision_event
     `cdc10b03…` and decision_receipt `01a162a2…`.
   - The reason must be byte-identical, because the decision ref digests
     it.
4. Read `choir self-dev head` again. Expect no new `effect_rejected` on
   the tape. The sequence may move only by projection batches, not by a
   decision.
5. Negative checks (both must answer `http 409`, with the head
   unchanged):
   - the same command with `--reason "different"`;
   - the same command with `--idempotency-key fp4ff-reject-other`.
6. If any approve on a test computer was landed by the CLI and reached
   `applied`, replay it: `choir self-dev approve --computer … --operation
   <id>`, with the same key; the default key `cli-approve:<id>` is
   deterministic. Expect 200 with state `applied`. Before the fix this
   answered 409 "decision idempotency key reused with different binding",
   so this checks part 1 on the product path.
7. Afterwards, grep the guest log for `selfdev decision:`. Any line there
   names a race whose match failed, with the comparison that failed.

## Residuals

- The H1/H2 shapes (a different store view, or a projection without
  fields) are not needed to explain the evidence. They are also not ruled
  out on staging. The new log line is the detector.
- The `found` replay path still answers a generic 409 text when its
  verifier fails. It does not log the failed comparison.
- An approve replay is accepted by the CLI only in `accepted`,
  `materializing` or `applied`. A landed approval that later `failed` or
  rolled back is refused client-side, as before.
