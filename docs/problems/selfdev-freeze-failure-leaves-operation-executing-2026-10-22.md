# A failed self-development freeze leaves the operation "executing" forever, and the computer never updates (2026-10-22)

Found 04:05 UTC. The owner reported that their computer (computer-ad1b46d4…,
vm-4a9afa23, the Claude delegate's computer) had deferred every deploy
refresh tonight and was "not actually doing any work".

## Evidence

- vmctl's retry sweep, every minute since at least 02:54: `deploy refresh
  … outcome=deferred planned_restart=false busy_allowed=false reason="busy:
  … running_runs 0, selfdev_active_operations 1, live_capsules 0"`.
  This was not a capsule: the console shows every capsule torn down by
  10-21 11:14.
- `GET /api/computers/<id>/self-development/operations` (host-internal,
  read-only) lists one active operation:
  - `selfdev-20d9baa5eb36704d52009b6876611f98`, state `executing`;
  - created 2026-10-21T10:23:37.961Z, updated 10:23:39.739Z;
  - `last_intent_error`: "freeze: capsule release must contain executable
    bin/autoputer: the running binary was built from 219535ef… but the
    capsule source…".
- The computer booted at 10:20:18 on 10-21 (crash boot; it closed the
  earlier `selfdev-f6171703…` from executing). Firecracker has run since
  then: 17 h 40 min, RSS 3.9 GiB of host memory.

## Cause (read in code, to be confirmed by the fix's test)

The freeze step failed and recorded `last_intent_error`, but nothing moved
the operation to a terminal state (rejected, failed or closed). The active
set `{executing, accepted, materializing, rollback_pending}` keeps
counting it (`activeSelfdevOperations`, internal/agentcore). So:
- `/health` reports `selfdev_active_operations: 1`;
- vmctl defers every refresh indefinitely ("would not survive a restart");
- an owner decision cannot close it either. Approve and reject both
  require a frozen operation (`decideSelfDevelopment`,
  internal/agentcore/api_self_development.go:310-330), and this one never
  froze.

The only exit is the crash-boot close ("interrupted by a restart"), which
needs a markerless restart that nothing schedules.

## Harm

- The computer never takes a deploy. It has run a build from before
  tonight's deploys A, B and C.
- It holds 3.9 GiB on a full host, where admission refused test
  computers for 20 minutes tonight.
- The owner sees nothing: no fate, no notice.

## Repair

1. Now (owner-authorized, 2026-10-22: "we should get it updated and not
   stuck"): a plain stop and start of this computer through vmctl's
   lifecycle, with no planned-restart marker. Boot then closes the
   operation as "interrupted by a restart" (the designed crash fate), and
   the computer boots on the current deploy. Expected afterwards: no
   active selfdev operation, and the current build.
2. Code (red: self-development lifecycle). A freeze or intent error
   closes the operation with a visible terminal fate (failed: the error),
   in the same transaction that records the error. Tests first: a freeze
   failure leaves no active operation; vmctl's busy check then answers
   not busy.
3. Detector: an active self-development operation with no transition for
   more than N minutes appears on the obligations surface as stalled.

## Cause, read in code and reproduced by test (FP4 follow-up, 2026-10-22)

The operation belongs to its engineering assignment (C1e D2, keyed
`EngineeringFreezeIdempotencyKey(assignment)`). A freeze refusal only
stamps `last_intent_error` and leaves the operation executing, on
purpose: while the assignment lives, the desk may fix its receipts and
freeze again (`TestFreezeRefusalStampsOperationIntentError` pins
"intent refusal must not move state"). So the refusal itself is not
the moment the operation becomes dead.

The operation becomes dead when its assignment ends. Every
cancellation path closes it (`failBoundSelfdevOperation` from
`persistSystemEngineeringCancellation` and the management cancel; the
run-death settle goes through the same cancel). The **report** path
does not: `commitAssignedEngineeringReport` turns the assignment
`completed` or `failed` and never touches the operation. An assignment
whose freeze was refused and which then reported (completed or
failed) leaves its operation executing with nothing able to advance
it. `TestAssignmentEndClosesItsUnfrozenOperation` reproduces this for
both results before the fix.

Hypothesis, not yet confirmed from the ledger: selfdev-20d9baa5… is
this case (its assignment reported after the freeze refusal at
10:23:39). FP0 can confirm from the assignment row for that operation's
freeze key (disposition and report id); a still-open assignment with a
live run would be a different cause.

## Repair shape (deviation from item 2 above, stated)

Item 2 asked to close the operation in the transaction that records
the error. That would kill the in-assignment retry the design keeps.
Instead the operation shares its assignment's fate at the report:

- assignment ends `completed` with the operation `requested` or
  `executing` -> operation `failed`, terminal error "assignment
  completed before its candidate froze; last refusal: <the stamp>";
- assignment ends `failed` -> the same, and a `frozen` or `verified`
  candidate also closes (management verifies only a completed
  assignment's candidate);
- a completed assignment's frozen candidate stays for Verify.

The close is an ordinary `Transition` (on the tape like every other)
and runs on report replays too, so a crash between the report and the
close converges. A re-freeze of the same assignment finds the failed
operation and is refused; a new assignment opens its own. Crash-boot
close is unchanged. Item 3 lands as a read-only `selfdev` section on
`GET /api/runtime/obligations`: open operations, the busy count
`/health` reports, and `stalled` for any open operation idle longer
than 30 minutes, with its last refusal.

## Now (FP4 lanes: lane/fp4-freeze-failure-closes, then lane/fp4-freeze-followup)

- state: lane/fp4-freeze-failure-closes (950adaed0) accepted in bunch
  19 and going to deploy G. Its reviewers' low findings are on
  lane/fp4-freeze-followup (from 950adaed0); FP0 lands and proves.
- review bunch 15 (Devin, 2026-10-22): F1 the close now also runs at
  the report entry point on every success, so the early replay returns
  converge after a crash before the close; F2 every give-up logs; F3
  the close ignores a cancelled request; F4 a test drives the outer
  replay path and failed before F1; F5 the obligations list says when
  it is truncated.
- DeepSeek review of bunch 15: its replay finding is the same gap as
  F1, closed by the entry-point close. Added tests: the close on the
  projection tape, a late freeze racing the close (one store), decided
  states never closed, and a tape conflict between two stores. The
  last failed 3 of 3: the close gave up on `ErrProjectionMismatch`;
  it now re-reads and retries it like `ErrConflict`.
- bunch 19 follow-up (DeepSeek, Devin, flash): `failSelfdevOperation`
  (the cancellation close, assignment and trajectory) now reads the
  current state, closes from it, retries a lost race (`ErrConflict` or
  `ErrProjectionMismatch`) up to three times and logs every give-up
  (`TestCancellationCloseRetriesATapeConflict` failed 3 of 3 before).
  The assignment-end close keeps a 15 s deadline instead of none
  (`TestAssignmentEndCloseIsBounded` hung before). The "never closes a
  decided operation" invariant is scoped to the assignment-end close:
  cancellation closes `awaiting_approval` on purpose.
- bunch 24 follow-up (DeepSeek, Devin), on lane/fp4-freeze-followup
  after 5f8fe18d7: the cancellation close carries the same 15 s
  deadline as the assignment-end close, at its one seam
  (`failSelfdevOperation`, through `selfdevCloseContext`), so a wedged
  tape append no longer holds the Store's write mutex, or
  `engineeringAssignmentOpenMu` on the reconcile paths, forever
  (`TestCancellationCloseIsBounded` hung before). A failed read in
  `failBoundSelfdevOperation` is logged. Both bounded-close tests
  assert a timed-out close leaves the operation whole in its prior
  state. The slot replay now has its test (below). The two-Store tape
  races run 8 iterations instead of 30: under `-race` each iteration
  hit the conflict 28 to 30 times in 30.
- deployed proof (FP0, TEST computer only): see the lane report.
- residual: the stamp write (`RecordIntentError`) is direct SQL, off
  the tape; the terminal error now carries the stamp onto the tape at
  the close. A stall that is not an ended assignment (an open
  assignment whose run is alive but idle) is only detected, not
  closed. `idle_seconds` reads 0 if `updated_at` fails to parse (it
  round-trips in practice). Closed on the follow-up lane: the
  cancellation close stopping on `ErrProjectionMismatch`. The earlier
  rationale here ("not reachable in one process, shared write mutex")
  was wrong: the write mutex is per `selfdev.Store`, and rematerialize
  swaps `rt.selfdevOperations` for a new Store
  (`internal/agentcore/rematerialize.go:268`) while the old one may
  still be writing, so two writers without a shared mutex can race in
  one process. Closed in bunch 24: the same-digest slot replay return
  (`recordAssignedEngineeringReportOnce`, the `SlotTerminalReport`
  match) now has a test, `TestSlotReplayClosesTheOperation`. The
  bunch 19 note here said no public API could build its row; that was
  wrong. The store's report_id check runs only when the request carries
  a proposition digest (`internal/store/engineering_assignments.go`,
  `RecordEngineeringAssignmentReport`), and the store keeps the
  caller's report id, so a report recorded with an empty digest and
  another id builds it. The test pins existing behaviour: it passes
  with the entry-point close and fails without it.


## Merged from docs/problems/selfdev-op-outlives-its-failed-assignment-2026-10-22.md (2026-10-22)

**A failed engineering assignment leaves its self-development operation "executing"; three times in one day on the delegate's computer (2026-10-22).**
Found 04:20 UTC by the self-development track monitor (read-only). This record adds trace evidence and a cause to docs/problems/selfdev-freeze-failure-leaves-operation-executing-2026-10-22.md. It does not replace it. Mutation class of this record: green. The fix is red (the self-development operation lifecycle).
- result after that boot is in the first record.

(Full text in git history at the path above.)
