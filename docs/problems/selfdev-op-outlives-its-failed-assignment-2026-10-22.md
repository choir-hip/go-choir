# A failed engineering assignment leaves its self-development operation "executing"; three times in one day on the delegate's computer (2026-10-22)

Found 04:20 UTC by the self-development track monitor (read-only). This
record adds trace evidence and a cause to
docs/problems/selfdev-freeze-failure-leaves-operation-executing-2026-10-22.md.
It does not replace it. Mutation class of this record: green. The fix is red
(the self-development operation lifecycle).

## Evidence

All reads were host-internal GETs from Node B to the guest of
computer-ad1b46d4… (vm-4a9afa23), with vmctl's read headers. Only states,
ids and times were read. No prompt, report or document content was read.

1. `GET /api/computers/<id>/self-development/operations`, 04:10 UTC. Three
   operations. Each was created during an engineering assignment's bound
   window, and each stayed `executing` after that assignment failed:

   | operation | created | last updated | state now | last_intent_error starts |
   | --- | --- | --- | --- | --- |
   | selfdev-bd394956… | 10-21 06:01:30 | 06:46:31 | failed, "interrupted by a restart" | "freeze: a cited execution receipt records an execution that did not succeed" |
   | selfdev-f6171703… | 10-21 07:29:07 | 10:20:17 | failed, "interrupted by a restart" | the same |
   | selfdev-20d9baa5… | 10-21 10:23:37 | 10:23:39 | **executing** (18 h on 10-22 04:20) | "freeze: capsule release must contain executable bin/autoputer" |

2. `GET /api/trajectories/<trajectory_id>` for each operation's trajectory.
   The assignment rows on the tape:

   | operation | assignment that brackets its creation | opened | terminal | disposition |
   | --- | --- | --- | --- | --- |
   | selfdev-bd394956… | assignment-9a542b7a… | 06:00:08 | 06:01:42 | failed (reducer-derived from report), capsule revoked |
   | selfdev-f6171703… | assignment-962dc239… | 07:27:26 | 07:29:20 | failed (reducer-derived from report), capsule revoked |
   | selfdev-20d9baa5… | assignment-f138ade6… | 10:22:14 | 10:23:49 | failed (reducer-derived from report), capsule revoked |

   Each trajectory is still `live`. Its Texture activation is cancelled or
   passivated.

3. Trajectory dbadbdfd… events (selfdev-20d9baa5…'s trajectory):
   `co_super_assignment_opened` 10:22:14, `co_super_assignment_bound` 10:22:33,
   four `co_super_capsule_disposition_set` 10:23:48-49,
   `co_super_assignment_reported` 10:23:49. Then a second assignment
   (assignment-415c4263…) opened 10:24:34 and completed 10:25:30, and a
   Texture turn committed 10:25:55. Nothing after that. The operation's
   freeze refusal (10:23:39) is 10 s before the failed report.

4. The two earlier operations were closed only by crash boots. The console
   log shows `selfdev: crash boot: selfdev-bd394956… closed from executing:
   interrupted by a restart` at 06:46:31, and the same for selfdev-f6171703…
   at 10:20:18. The third has had no boot since.

5. vmctl, Node B: `deploy refresh caller=retry-sweep … vm=vm-4a9afa23…
   target=3c16f7c33… outcome=deferred … selfdev_active_operations 1,
   live_capsules 0`, once a minute (20 of 20 in the 20 minutes before
   04:10:58).

Binding caveat: the operations API does not expose the idempotency key
(`json:"-"`). The freeze key is `freeze-open:<assignment id>`
(freeze_candidate.go:31). The operation-to-assignment binding above is
inferred from times: one operation per trajectory, created inside exactly
one assignment's bound window.

## Cause (code read; consistent with the trace, not yet shown by a test)

When an assignment ends by its own failed report, nothing closes the
operation that the assignment bound.

- `commitAssignedEngineeringReport` (internal/agentcore/engineering_assignment_fate.go,
  about line 1305) records the report. On a terminal disposition it calls only
  `settleEngineeringPrecommitsAtTerminal`. That function reads the operation
  but does not move it.
- `failBoundSelfdevOperation` closes the bound operation. Its callers are the
  cancel paths only: management's Cancel (:215), the system cancellation
  choke point `persistSystemEngineeringCancellation` (:281, its comment
  says "Every assignment-level cancellation funnels through this choke
  point"), and the restart reconcile (:595, :638). A reported failure is
  not a cancellation, so it goes through none of them.
- `failTrajectorySelfdevOperations` runs only when the trajectory is not
  live (:647). These trajectories stay live.
- The store's report transaction (`RecordEngineeringAssignmentReport`,
  internal/store/engineering_assignments.go:1763) does not touch
  `self_development_operations`.

So the assignment row (the tape) says the work is over. The operation row
says it is still executing. Nothing reconciles the two except a crash boot.
Two state authorities disagree about one fact (standing question 5).

## Why this matters for the fix in flight

The first record's repair 2 is "a freeze or intent error closes the
operation … in the same transaction that records the error". That conflicts
with the field's own contract. `LastIntentError` is documented as "the most
recent non-terminal authority refusal … informational, never a state"
(internal/selfdev/operations.go:98-103). `RecordIntentError` says a refused
freeze "leaves the operation executing while the cell retries"
(operations.go:510-517). A worker can fix its receipts and freeze again in
the same assignment. Closing on the first refusal would end that retry.

The trace points at a different join. **The operation should end when its
assignment ends without a frozen candidate.** On a terminal report whose
operation is still pre-frozen (requested or executing), fail the operation
with the report's disposition as its error, in the same transaction or at
the same choke point as the report. A successful report keeps the operation
for management's Verify, as today (`assignmentOutcomeRefs`, D9).

This is the cluster rule's third receipt for "an operation stays executing
on a computer that never restarts":

- docs/problems/s0-selfdev-executing-wedge-2026-10-04.md (live wedge);
- docs/problems/selfdev-zombie-operations-pin-owner-computer-2026-10-10.md,
  which named the gap: "They do not close the live wedge on a computer that
  never restarts. That case still needs the desk run's terminal fate to
  close its bound operation.";
- this record (three operations on 10-21, one computer).

A substrate-level fix closes all three. A freeze-error close fixes only the
freeze-refusal case. An assignment that fails for any other reason after
the operation opened would still strand it.

## Hypotheses to confirm by test before the fix

- H1: a focused test that opens an assignment, starts its freeze operation
  (executing), records a refused freeze, then commits a failed report,
  leaves the operation executing on current main. That test fails first.
- H2: the same test with a completed report and a frozen operation leaves
  the operation frozen (unchanged behaviour).
- H3: the guest's `activeSelfdevOperations` then reads 0 and vmctl's busy
  check answers not busy.

## Not done by the monitor

The monitor did not restart, stop, or change the computer. The owner
restarts it (owner-authorized repair 1 in the first record). The expected
result after that boot is in the first record.

## Update, 04:25 UTC

lane/fp4-freeze-failure-closes reached the same cause on its own
(b005f4bfe, the first record's cause section rewritten to the report path).
The test 72347ed78 fails first, and the fix is 4851efbb8:
`closeEndedAssignmentSelfdevOperation`, called after a terminal report and
on replays. The warning above about repair 2 is resolved. The trace table
here is that fix's staging evidence: after it deploys, a failed assignment's
operation should log `selfdev: <op> closed from executing: assignment failed
before its candidate froze`, and no operation should stay executing past its
assignment's terminal_at.

The fix acts when a report commits. It does not close selfdev-20d9baa5…,
whose report committed on 10-21. That operation still needs the owner's
restart (repair 1 of the first record).
