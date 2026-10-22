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

## Dispositions (lane/fp4-r4)

Tests b79cab192 (each failed on main 6c60685cc), fix 4e6f96833.

- **P2: repaired at the store.** `RecordEngineeringAssignmentReport`, on a
  committed command id with another digest, replays the committed result
  when that command committed this same terminal proposition (the slot's
  match has the same report id and command id). A partial, or a different
  proposition, keeps the conflict. Corrected in round 4b: this made the
  store answer success to both writers, not the cells. A loser that found
  the winner had already destroyed the capsule failed earlier, resolving
  the frozen capsule's capability or binding receipts, before it reached
  the store, so its cell still heard an error for a report that landed
  (review round 4b, DeepSeek 3). Round 4b fixes the race at its source;
  see below. Not proven locally through the runtime (the race needs a
  frozen capsule); the store test reproduces the second writer's request.
- **P3: repaired.** The guard resets its refusal when a later tool result
  carries `freeze_result.operation_id`; the key is top-level in the cell
  result, so text the model prints cannot set it. Round 3b's residual in
  docs/problems/fp4-r3-partial-advice-ends-in-system-cancel-2026-10-22.md
  is closed.
- **P4: repaired.** The dispatcher's poison lines carry the handler error
  on the last attempt and at the deferral limit, and say "earlier
  deliveries did not complete (crash, kill or a failed commit)" when the
  budget was spent before the handler ran (wording corrected in round 4b:
  a handler that returned but whose commit failed, an epoch conflict
  included, or a restart that lowered MaxAttempts, reaches it too). Which branch dead-lettered 58a5b36c stays
  unknown; the next one will say.

## Round 4b (review of 8727d5f41: DeepSeek v4.1 flash, Sonnet; both accept with edits)

Tests fec168943, fix 3efb1312d.

- **P2 at its source (DeepSeek 3, Sonnet F6): repaired.** A per-assignment
  saga mutex in the host process (`assignmentSagaMutex`). The worker's
  Complete takes it; the fate wake's resume of the staged proposal only
  tries it, and defers when the live saga holds it. [Corrected in round
  4c: as built in 3efb1312d the deferral returned nil and consumed the
  wake, the watchdog arm deduped into the consumed G9 wake, and several
  failure exits armed nothing, so the claimed "arms the fate watchdog,
  which re-drives" did not hold; see Round 4c.] A worker that waits behind a resume finds the report
  stored and replays it before any capsule step. Both drivers run in the
  host process (capsule cells reduce there; the fate wake is handled
  there); a driver in another process would not be covered, and none
  exists today. Not minting the immediate fate wake was the other option;
  it would leave a crash between the freeze intent and the commit without
  its re-drive, so it was not taken.
- **Replay checks (Sonnet F2): repaired.** The same-command replay first
  runs the fresh path's checks: a stated proposition digest must be the
  one the report computes, and the report id the one that digest derives.
- **Replay evidence (Sonnet F7): repaired.** When the replayed writer's
  evidence (executor receipts, candidate, mutations, attestation count)
  differs from the committed report's, one line says so; the committed
  report stands.
- **Guard reset (DeepSeek, Sonnet): repaired.** Any top-level
  `freeze_result` (a committed freeze, or a rejected bundle, which also
  passed the open-candidate check) resets the refusal, and wins over the
  refusal pattern in the same cell. A cell whose freeze committed before a
  later intent failed returns an error that carries the committed
  `freeze_result`, which resets too. [Corrected in round 4c: the marker
  check covered every tool result, so a successful cell's printed stdout
  could set it, not only a reduce error; it now counts only in a tool
  error. The wrap and the worker's lock were not pinned by a test.]
- **Poison lines (Sonnet F5): tests added** for the pre-handler and
  deferral-limit lines. [Corrected in round 4c: the deferral-limit test
  matched the earlier "deferred … cause=" line too, so it did not pin the
  poison line; it now judges the poison line.]

## Round 4c (review of f848bb74e: DeepSeek v4.1 flash sends back, Sonnet accepts with edits)

Tests ce11b6dd0, fix a32e7652f. Both reviewers found the same High.

- **A deferred fate wake was lost (DeepSeek 1, Sonnet E1): repaired.**
  Three faults, each confirmed (Sonnet by probe, then in code):
  - the resume's deferral returned nil, so the dispatcher consumed the
    wake;
  - `armAssignedEngineeringFateWatchdog` scheduled the same content as the
    G9 wake through the plain hook, and the actor log's
    `ON CONFLICT(update_id) DO NOTHING` dropped it once G9 was consumed;
  - the live saga's exits at the frozen capability, its digest, the
    receipt binding (the P1 incident's exit), the candidate export and the
    revoke path armed nothing.

  So a live saga that failed after a deferral was re-driven only by the
  six-hour bound deadline, the guard's cancel, or a planned restart.
  Now: the resume returns `ErrAssignmentSagaHeld`, which actorruntime maps
  to `actor.ErrDeferUnprocessed`, so the dispatcher re-delivers the same
  wake with backoff (0.5 s doubling to 30 s; after 64 deferrals, about 30
  minutes, it is dead-lettered with its cause). Every failure of the
  terminal report saga on an assignment that still owes its fate arms a
  re-drive, and the arm goes through the redrive hook, which mints a new
  generation when the family's wakes are all consumed and nothing while
  one is pending.
- **The worker's lock (DeepSeek 2, Sonnet E3 M2): pinned** by
  `TestWorkerCompleteWaitsForTheAssignmentSaga`, and released by defer
  (DeepSeek 5, Sonnet E5).
- **The freeze wrap (DeepSeek 3, Sonnet E3 M6): moved into
  `reduction.commit`** and pinned there; every caller carries it.
- **The P1 test (DeepSeek 4, Sonnet E2): skips off Linux**; receipts open
  only on the Linux guest kernel. It runs and pins on the Linux VM.
- **The marker (DeepSeek 6, Sonnet E8): counts only in a tool error.**
  Text inside a reduce error can still set it; the effect is only the
  default guard reason and instruction, never a refusal.
- **The saga map (DeepSeek 7, Sonnet E7): trimmed.** A free entry is
  dropped once the assignment no longer owes its report.
- **Store mutants (Sonnet E4): killed** by a case with a derived report id
  and a wrong stated digest, and one row per evidence leg.
- **Comment order (Sonnet E9): fixed.**
- **Residual (Sonnet E6):** the worker waits on the saga mutex without its
  context. A resume that holds it longer than the cell's three-minute fate
  context leaves the worker's store calls to fail after it acquires, so its
  cell can hear an error for a report the resume landed. Narrow (a resume
  holds the mutex only across one saga attempt).
