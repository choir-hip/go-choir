# C1e B1: Engineering Assignments End "Cancelled" Seconds After Management's Cast

Date: 2026-10-14, 05:20 UTC. Found by the B1 engineering drill
(`node scripts/reeng_tracer.mjs --scenario engineering`, authored tonight
as C1e step 4, commit 6ae126b9 on branch worktree-agent-ab80b7882f2ad21aa,
staged for the next push). Deploy: staging e9d727a7 (C1e B1 + the
homepage change). Status: documented; cause not yet established from the
trace.

## What the drill saw

Test computer `computer-d8ad5936bca79ed2668330c68419c5db` (build
e9d727a7, self-development mode set to propose_only), label
`c1e-b1-engineering-0444`, receipt in
`~/.config/choir-qa/traces/c1e-b1-engineering-0444/`.

| time (UTC) | phase |
|---|---|
| 04:46:35 | the owner request is submitted through the prompt bar |
| 04:46:45 | Texture Casts to management (round 1) |
| 04:47:12 | management Casts engineering on records: assignment-703ad944..., parent is Texture's Cast |
| 04:47:22 | engineering's final Report: `stop_reason: cancelled`, record id `assignment-report:cancel-report:...`, no operation, no bundle digest |
| 04:58:02 / 04:58:13 | a second Cast (assignment-afc3c5a7...), cancelled 11 s later |
| 05:00:05 / 05:00:16 | a third Cast (assignment-979023d5...), cancelled 11 s later |
| 05:16:35 | the drill times out: no Freeze, no Verify, no decision |

So B1's first half works on staging: management casts engineering on
records, with the right parent. The second half doesn't: each assignment
ends with a cancel report about 10 seconds later, and engineering never
freezes.

## Candidates (hypotheses until the ledger says)

- B1 named these open points (station now card, 01:50 UTC):
  `stopCancelledAssignmentRun` is new; "the reconcile and the run-end
  settle can each mint the final Report". A reconcile that sees no
  engineering run yet, or a run that ended at once, may mint the cancel
  report.
- The engineering run may fail to start or end at once (capsule, mode,
  or module surface), and the run-end settle reports it as cancelled.
- Management may cancel its own Cast on its next turn (B1 gave management
  a cancel).

The next step is to read the trace: the assignment rows, the engineering
runs and their events, and who wrote each cancel report.

## Trace findings (2026-10-14, 05:18 UTC)

Read from the test computer's ledger (started 05:17:55, stopped 05:18:26;
the start was a crash boot, which closed the three Texture runs with
`interrupted_by_restart` and did not touch the engineering evidence).

**The engineering run starts every time; every capsule cell is refused.**
All three runs (`run:assignment-703ad944...`, `run:assignment-afc3c5a7...`,
`run:assignment-979023d5...`) have the same 21 events:

| seq | event |
|---|---|
| 2 | the model calls `capsule_go_eval` (first cell: `choir.Records()`) |
| 4 | `tool.result` is_error: `assigned capsule obligation is no longer executable: assigned work obligation is not open: record not found` |
| 6 | end_turn: "the capsule runtime reports that assignment ... is no longer open" |
| 7 | `loop.retry` reason `engineering_overlay_terminal_fate_required` |
| 11, 18 | the same refusal on the next two cells (`choir.Help("Complete")`, `choir.Records()`) |
| 14 | second `loop.retry` |
| 21 | `loop.cancelled`: completion guard not satisfied after 2 retries |

Run 703ad944: created 04:47:10.291, finished 04:47:19.946 (cancelled). The
run metadata carries `assigned_conversation_id`, `parent_round` and the
capsule id, and no `assigned_work_item_id`, as a records run should.
`/api/runtime/obligations` holds one work item (Texture's,
`to: texture:9383da12...`); no engineering work item exists.

**Who minted the cancel Report.** The final Report in management's inbox
(seq 2, 04:47:19.946, `assignment-report:cancel-report:sha256:be8686...`)
has the claim `tool loop: completion guard "engineering_overlay_terminal_fate_required"
was not satisfied after 2 retries; assignment cancelled; late results are
evidence-only and cannot reopen work`. That reason text is the run's own
error, so the path is the run's failure path, not the reconcile, the
restart, or management: `handleExecutionError` (runtime.go:4221) ->
`terminalizeRun(reason = err)` -> `cancelBoundEngineeringRun`
(engineering_assignment_fate.go:347) -> `persistSystemEngineeringCancellation`
-> store `CancelEngineeringAssignment`, which mints the cancel-report
(engineering_assignments.go:2020, 2065). Seqs 4 and 6 are the same for
the other two assignments. Management's runs call `choir.Cast`,
`choir.Records`, `choir.Judge`, `choir.Note`, `choir.ReportPacket`, and
never `choir.Cancel(`. Hypothesis 3 (management cancels its own Cast) is
ruled out; hypothesis 1 (the reconcile cancels a fresh assignment) is
ruled out; hypothesis 2 is right in a specific form: the run starts, and
the capsule gate refuses it.

**Cause.** `validateAssignedEngineeringExecution`
(internal/agentcore/engineering_assignment_tools_overlay.go:100-148) is the
`ValidateCurrentObligation` that `requireCurrentAssignedCapsule`
(tools_capsule.go:93) runs before every capsule cell. B1 (a06191f9) left
it on the pre-records shape: it loads
`GetLifecycleWorkItem(assignment.Binding.AssignedWorkItemID)` and requires
an open work item assigned to the run's agent. A records binding carries
no work item by construction (`EngineeringAssignmentBinding.Validate`
refuses one; `buildAssignedEngineeringRun` sets none), so the lookup is
`GetLifecycleWorkItem("")` -> `ErrNotFound` on every cell. The engineering
desk can never run a cell, so it can never Freeze or Complete, and the
completion guard ends the run.

**Why the B1 tests missed it.** Every test that builds an engineering
`CapsuleToolCtx` stubs the validator (`ValidateCurrentObligation: func(...)
error { return nil }` in rlm_reduce_test.go:53 and
selfdev_freeze_base_head_test.go:172); no test runs the real validator
against a records assignment. The store and run-end paths were ported
(store `projectEngineeringTerminalRun` branches on `Binding.Records()`);
this one runtime gate was not.

**Substrate vs symptom.** Symptom of an incomplete port, not a substrate
fault: the records model is right and its authority (the assignment row,
the engineering conversation's round 1) exists; one gate still reads the
deleted work-item authority. The records equivalent of "the assigned work
obligation is open" is: the assignment's engineering conversation
(`management:<owner>|engineering:<assignment>`) has an open round whose
recipient is the run's agent. That is what `planEngineeringReportRecord`
already reads before minting a Report.

Other residual pre-records reads of `AssignedWorkItemID` (not on this
drill's path; to be checked once engineering reaches them):
`texture_lifecycle_api.go:569` (producer-report authority compares
`ProducerWorkItemID` to it; both "" on records, so equal),
`engineering_evidence.go:361/485/556` (report events matched by
`WorkItemID`, "" on records both sides), and
`resumeStrandedFrozenAssignmentCommit` metadata.
