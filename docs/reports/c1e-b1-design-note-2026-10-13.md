# C1e B1 Design Note: Where Management Reads An Engineering Outcome

Date: 2026-10-13. Station:
[C1e](../definitions/choir-reeng-c1e-engineering-on-records-2026-10-13.md),
before step 1. Inputs:
- [the packet-reads report](c1e-management-packet-reads-2026-10-13.md),
  whose section 3 gap list this note answers item by item;
- the [deletion inventory](c1e-deletion-inventory-2026-10-13.md).

Mutation class of the work this note shapes: red. It touches run
acceptance and management's control path. This note itself is green.

## The question

On the packet path, management reads an engineering outcome from
`buildEngineeringReturnPacket`'s producer_report, and the parked casting
run is resumed when it arrives. A records Report carries neither the
assignment identity nor a typed outcome, and a records wake starts a
fresh run rather than resuming. B1 must choose where those facts live
and how management is woken, without a new record kind or a second wake
mechanism (conjecture C1e-1).

## Decisions

Revised after convergent round 1 (claude, codex, gpt-6.1-sol,
gemini-3.8-flash and deepseek-v4.1-flash sent it back; glm-5.3-flash
accepted with two must-decide items). The panel agreed with the direction
and found five gaps: premature closure of Texture's round, the missing
request join, closure without reading, wrong outcome fields, and commit
order. Each decision below says what changed.

**D1. One conversation per assignment.** After step 3 deletes path B
(document assignments and their restart recast,
`engineering_assignment_runtime.go:398-406`, `engineering_desk.go:225-283`),
the only mint is the delegated cast, `openDelegatedCastAssignment`. It
fixes `attempt` at 1 (`:486`), derives the assignment id from the cast's
commitment record (`:490-492`), and names the agent
`engineering:<assignmentID>` (`:547`). So the conversation
`management:<owner>|engineering:<assignmentID>` belongs to exactly one
assignment, which has one round. A retry is a new Cast in a later turn:
a new commitment record, a new assignment id and a new conversation. It
is never a recast. When the binding is written, admission asserts that
the assigned inbox equals `engineering:<assignmentID>`. *(Round 1:
corrected citations; attempts above 1 do not exist on records.)*

**D2. The assignment is the outcome authority, through the chain that
exists.** No new field:
- the assignment row holds the disposition and `ReportRefs`
  (`types/engineering_assignment.go:290-308`);
- the terminal `EngineeringAssignmentReport` its refs name holds the
  result, verdict and candidate (`store/engineering_assignments.go:2212-2244`);
- the self-development operation, resolved by the binding's
  `TrajectoryID` exactly as Freeze and Verify resolve it today
  (`rlm_reduce.go:1388-1470`), holds the operation id and bundle digest.

Management's Verify walks that chain. The Report's `EvidenceRefs` repeat
the assignment, report and operation ids for the trace reader only.
*(Round 1: the row does not hold the result or the operation; the chain
does.)*

**D3. Engineering's final Report is minted by Complete, in the same
transaction as the row.**
- Today `recordAssignedEngineeringReport` writes the row and mints
  `buildEngineeringReturnPacket` in one transaction
  (`store/engineering_assignments.go:2213-2262`). B1 replaces the packet
  there with a records Report composed into the same batch
  (`planInboxRecords` composes into a caller's batch).
- The Report is sent by `engineering:<assignmentID>` to
  `management:<owner>`, with `Final: true` and `Report.StopReason` set
  to the disposition. It closes the round atomically with the terminal
  row, so a Report can never precede the outcome it reports. This
  dissolves failure mode 1.
- The cancellation path (`:2383`) mints the same final Report with
  StopReason `cancelled`, also atomic with the row.
- Reports the engineering cell stages through `choir.Report` are
  progress: non-final, and refused when the engineering conversation has
  no open round, so no round-0 late records (`store/inbox.go:214-239`).
- `validateCellIntents` already makes Complete the last intent of a cell
  (`rlm_reduce.go:122-126`), and Freeze precedes it. That is the commit
  order.

*(Round 1: the reducer commits records before intents, `rlm_reduce.go:875-918`,
so a staged final Report would precede Complete.)*

**D4. The engineering Cast names the Texture request it serves.** The
engineering Cast's `ParentID` must name the Cast record that opened
management's current round with `texture:<doc>`. The reducer validates
this: the named record is a Cast into management's inbox, and its
round is open. The binding replaces `ParentWorkItemID` with that parent
conversation and round, and `AssignedWorkItemID` with the engineering
conversation (round 1). The binding's `TrajectoryID` is derived from
the document `texture:<doc>` names. Step 2 names the lookup and refuses
the cast when the document has none. Records runs carry no trajectory
(`inbox_records.go:501-505`), so the trajectory cannot come from the
casting run. *(Round 1: the request join was undefined.)*

**D5. No resume, and Texture's round survives the casting turn.** A
Report or closure into `management:<owner>` wakes a fresh `inbox_record`
turn, so nothing parks. `managementRunCasts`, `managementParkDecision`
and `awaiting_delegated_cast` serve only `lifecycle_texture_control`
turns (`management_controller.go:2590-2593`, `:2637`) and go with path
B.
- `closeRoundsOfEndedRun` (`inbox_records.go:675-716`) closes every
  incoming round of an ended management turn. It skips a round with
  `texture:<doc>` while any assignment whose parent is that round
  (D4) is non-terminal.
- When the last such assignment's final Report wakes management, that
  turn answers Texture. If it ends without a final Report, the round
  closes `run_<state>` as today.

*(Round 1: without this skip, Texture's round closed seconds after the
cast.)*

**D6. Closing Texture's request requires having read the outcome.**
Management's final Report into round R with `texture:<doc>` is refused
unless every assignment whose parent is round R is terminal, and the
record that closed each one's engineering round has `InboxSeq` at or
below this run's shown mark for `management:<owner>` (`noteRecordsShown`,
`inbox_records.go:141`). It is checked in the reduce step that commits
the Report. Scoping to the parent round means two documents' assignments
never wait on each other. A crash loses the in-memory shown mark, but a
crash closes every round anyway (no resume). This replaces the
`ConsumedDeliveryUpdateIDs` rule (`store/lifecycle.go:3783-3800`).
*(Round 1: a closed round can arrive mid-call, unread.)*

**D7. Closures and wakes.** System records wake no one
(`store/inbox.go:191-208`), so that a boot never resumes work.
- `closeRoundsOfEndedRun` gains engineering. An engineering run that
  ends without Complete has its round closed with fate `run_<state>`.
- That closure, and a spawn failure's closure
  (`engineering_assignment_runtime.go:634-637`), are minted only by a
  live runtime, never at boot. `inboxRecordWakes` gains them as an
  exception, like support pairs and captures. The crash closure
  (`interrupted_by_restart`) stays silent.
- Cancellation reaches the engineering run through the existing
  assignment cancel, which stops its run. The `cancelled` final Report
  of D3 is engineering-sent and wakes management like any Report.
- The progress-overdue packet (`:2715`) is deleted. A capsule that
  overruns its deadline ends, and its run-end closure wakes management.

*(Round 1: system closures did not wake; cancellation did not reach
engineering; the overdue notice had no wake.)*

**D8. Integrity and sender.** Records are objectgraph objects with a
content digest. The re-check rule bounds a poison record: re-reading
happens only on a newer record. If management cannot read an
engineering Report, the outcome still sits on the assignment chain (D2).
The store's existing check (a final Report comes from the round's
recipient) is the sender check.

## What B1 does not add

There is no new table or record kind, and no field on
`CommitmentRecord`. The binding swaps two work-item ids for a parent
conversation and round and an assigned conversation. The closure gains
reasons (`cancelled`, `run_<state>`, the spawn failure); its constructor
already takes any reason. `inboxRecordWakes` gains one exception.

## Ways this could fail (written before the code)

1. A management turn reads an engineering Report before the row is
   terminal. Prevented by D3: they commit in one transaction.
2. A cast with no valid parent (no open round with Texture, as in the
   paper path's own management turns). Refused by D4's validation, which
   is tested.
3. Two documents' requests, each with an open assignment. D6 is scoped
   per parent round. Tested with interleaved Reports.
4. The engineering Report arrives while a management turn is mid-call.
   D6 refuses that turn's final Report to Texture. The turn's re-check
   wakes one more turn, which reads it.
5. A crash between Complete and management's next turn. The boot closes
   both rounds `interrupted_by_restart`, silently. Nothing resumes. The
   owner's next act reaches a fresh turn that can read the terminal
   assignment.
6. A run-end closure at boot. It cannot happen: the boot writes only the
   crash closure, which stays silent (D7).
7. The paper path. Autopaper's terminal-outcome packets do not cross
   this route (packet-reads report), so B1 does not change them. They go
   in step 7.

## Residuals

- Step 2 names the document-to-trajectory lookup D4 relies on. If a
  document can have more than one live trajectory, the cast must name
  one.
- The run acceptance report (`run_acceptance.go`) names closed rounds
  instead of consumed packet ids.
