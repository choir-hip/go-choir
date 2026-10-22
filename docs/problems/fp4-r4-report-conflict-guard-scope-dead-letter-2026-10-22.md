# FP4 round 4: a landed report answered "digest conflict", the guard keeps a stale refusal, and a dead letter has no cause (2026-10-22)

Found in the FP4 round 3 drill and its re-run on the test computer
`computer-71afbf1461dffabcce8eeebe558d03bd` (deploys J and Q), reported by
FP0. Companion to
docs/problems/frozen-candidate-lost-when-complete-cites-a-duplicate-execution-ref-2026-10-22.md
(P1 of round 4). Problem documentation only; there is no fix in this
commit. Causes are read from code at 6c60685cc; where the trace does not
confirm them they are marked as hypotheses.

## P2: "lifecycle command digest conflict" for a report that landed

Evidence:

- First drill, A: guest log 10:11:20 "assignment A terminal report attempt
  failed: disposition=bound capsule=revoked version=6 err=lifecycle
  command digest conflict"; a retry recorded the report at 10:11:20.297.
  Four `assigned_engineering_fate_deadline` wakes for management were
  handled at 10:11:15 to 10:11:20, just before.
- Re-run, B: B's reporting cell got "lifecycle command digest conflict",
  though B's report landed.

Code read:

- A terminal report's command id is `engineering-report:<assignment>:<report
  id>`, and its report id is derived from the terminal proposition
  (`store.TerminalReportID`). Two writers of the same proposition use the
  same command id.
- Two writers exist. The worker's Complete drives the terminal saga
  (`recordAssignedEngineeringReport`). When that saga commits
  freeze_requested with a pending proposal, the store's outbox mints an
  immediate `assigned_engineering_fate_deadline` wake for management
  (internal/store/lifecycle.go, "G9"), whose handler re-drives the same
  saga from the staged proposal (`resumeStrandedFrozenAssignmentCommit`).
  Nothing serializes the two.
- Their requests differ in evidence (the resume commits the staged
  proposal's report; the worker binds receipts itself, and the outcome
  refs are read at commit time), so their command digests differ.
- `RecordEngineeringAssignmentReport` checks the command id first
  (`replayEngineeringAssignmentCommand`): same id, different digest
  answers `ErrLifecycleCommandConflict`, before the terminal slot gate that
  would replay a same-proposition report.
- `recordAssignedEngineeringReport` does not retry a conflict, so the
  loser's cell gets the error although its report is committed.

Cause: confirmed in code; that the resume was the other writer in each
case is a hypothesis supported by the timing of the fate wakes.

Harm: the model is told its report failed when it landed; a frozen
capsule refuses its next cell, so it cannot check, and the guard may
retry or cancel on a run whose fate is already terminal.

## P3: the completion guard keeps a refusal from before a successful freeze

Round 3b recorded as a residual that the guard's reason and instruction
read the run's last freeze refusal even after a later successful freeze,
on the belief that "a successful freeze leaves no mark in the cell output
the guard reads". That belief is false: a cell whose freeze committed
returns its tool result with a top-level `freeze_result` holding the
operation id (internal/capsule/types.go `FreezeResult`, set at
internal/agentcore/tools_capsule.go:671). The scan can be scoped to
refusals after the last successful freeze.

## P4: a dead-lettered wake has no logged cause

Evidence (FP0): a management `assigned_engineering_fate_deadline` wake for
A (update 58a5b36c…) was dead-lettered three times with no logged error.

Code read (internal/actor/dispatcher.go):

- When a handler error uses the last allowed attempt, the dispatcher
  emits `delivery_failed` and logs "poison … after N attempts" without
  the error, and skips the usual "handle … : err" line.
- When a deferral outlives MaxDeferrals the poison line omits the
  deferral's cause.
- When the attempt budget was already spent before the handler runs
  (earlier deliveries crashed or were killed), the poison line does not
  say so.
- The sink's line (`describeDeadLetter`) names only the update id.

Which branch dead-lettered 58a5b36c is not known (hypothesis: the
final-attempt branch, the one that drops the error).
