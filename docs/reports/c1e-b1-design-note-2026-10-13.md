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

**D1. The conversation is the assignment attempt's identity.** The
engineering agent id is already per assignment and per attempt:
`engineering:<assignmentID>`, with `:attempt-<n>` from attempt 2
(`agentcore/engineering_assignment_runtime.go:398-406`, `:547`). So the
conversation `management:<owner>|engineering:<assignmentID>[:attempt-<n>]`
names exactly one assignment attempt. The store allows it at most one
open round (`store/inbox.go:220`). Step 2 already replaces
`ParentWorkItemID`/`AssignedWorkItemID` with conversation and round
authority. The binding records `conversation_id` and `round` in their
place. A Report in that conversation needs no assignment, attempt or
report-id field. The assignment id appears once, in the Cast's
`RelatedIDs`, for the trace reader only.

**D2. The assignment row is the authority for the typed outcome.** The
outcome is the result (completed, partial, failed or blocked), the
verification verdict, the terminal disposition, and the candidate and
operation the work certifies. These are facts of the assignment, written
by the engineering verbs that already write the row (Complete, Freeze).
They are not facts of the message. Management's Verify (step 4) reads the
row named by the Report's conversation and round. It binds to the
operation id and bundle digest the row holds, which the Report repeats
in `EvidenceRefs` for the trace reader. One authority per fact
(standing question: single state authority). Copying the outcome onto
the Report would make two.

**D3. The round's close is the terminal marker.**
- A final Report from engineering closes the round. Its
  `Report.StopReason` is `completed`, `failed` or `blocked`, mirroring
  the row.
- A non-final Report is progress (partial results, the overdue notice)
  and closes nothing.
- A cancellation by management closes the round with a system closure
  record, `NewRoundClosureRecord` with reason `cancelled`, the same
  constructor the crash closure uses. That gives a cancellation an
  explicit fate instead of summary text.
- An engineering run that ends without a final Report has its round
  closed with fate `run_<state>`. `closeRoundsOfEndedRun`
  (`agentcore/inbox_records.go:682-686`) gains engineering.

**D4. No resume: a Report wakes a fresh management turn.** The parked
caster exists because a producer report never mints a management run
(`management_controller.go:2581-2589`, the stranding problem). On
records, a Report into `management:<owner>` mints an `inbox_record`
wake, and `WakeRecordsInbox` starts a turn that reads it through
`Records()`. That is how Texture already reads research's Reports, so
stranding cannot occur. Management's engineering casts move to records
turns (the refusal at `inbox_records.go:353-362` goes in step 1). Those
turns do not park. `managementRunCasts`, `managementParkDecision` and
the `awaiting_delegated_cast` passivation serve only path-B
`lifecycle_texture_control` turns and go with path B in step 3.

**D5. Acceptance is redefined against the cursor and the rounds.**
- `persistentManagementBoundReport`'s rule (every delivered packet
  listed in `ConsumedDeliveryUpdateIDs`; an unseen one refuses a terminal
  `WorkDisposition`) belongs to the packet path to Texture, which L3b
  already replaced with records. It goes with path B.
- On records, the turn marks read only what `Records()` showed it (the
  cursor rule, `autopaper-arrivals-unanswered-2026-10-13`).
- The ordering guarantee the old rule gave becomes a round rule:
  management's final Report to `texture:<doc>` is refused while an
  engineering round that management cast for that request is still
  open. A conversation-head check is enough; no per-run read mark is
  needed.

**D6. Integrity, retries, late reports.**
- Records are objectgraph objects with a content digest, so they need
  no payload digest of their own.
- A record that breaks the desk is read again only when a new record
  arrives (the records re-check rule), which bounds the poison loop the
  attempt cap guarded.
- A late Report into a closed round is refused by the store, matching
  today's "late reports mint no packet". The engineering desk receives
  the refusal as a cell error.

**D7. Sender identity.** The engineering inbox is the assigned agent
(`engineering:<assignmentID>[:attempt-<n>]`). The packet's sender check
(its `AgentID` must equal the work's assignee) becomes the store's
existing check: a final Report comes from the round's recipient.

## What B1 does not add

There is no new table or record kind, and no field on
`CommitmentRecord`. The binding gains `conversation_id` and `round` in
place of the two work-item ids. The round closure gains one reason
value (`cancelled`); its constructor already takes any reason.

## Ways this could fail (written before the code)

1. A management turn reads the engineering Report but the row's outcome
   is written after the Report commits. Verify then reads a non-terminal
   row. Guard: Complete and Freeze write the row before the final Report
   is accepted, or the Report is refused (ordering test).
2. An engineering inbox is reused across assignments, so the
   conversation no longer names one attempt. Guard: when the binding is
   written, assert that the inbox equals the per-attempt agent id.
3. A retry (attempt n+1, today's restart recast) opens a new
   conversation, because its agent id is attempt-scoped. The old attempt's
   round must be closed first (closure `cancelled`, or the crash closure)
   so that no orphan round stays open.
4. A management turn that casts engineering ends, and the Report arrives
   while another management turn is active. The re-check after that turn
   wakes one more turn (`recheckRecordsInbox`). Tested by the deployed
   engineering drill.
5. The crash between Complete (row terminal) and the final Report. The
   boot closes the round `interrupted_by_restart`, and the row says
   completed. Management's next turn sees a closed round with a crash fate
   and a completed row. It reports from the row and does not re-cast.
6. The paper path. Autopaper's terminal-outcome packets do not cross
   this route (packet-reads report, first section), so B1 does not
   change them. They go in step 7.

## Residuals

- Whether the progress-overdue observation (`engineering_assignments.go:2715`)
  becomes a non-final system Report or is dropped. Proposed: a non-final
  system Report, so management can cancel.
- The old `ConsumedDeliveryUpdateIDs` evidence on run acceptance
  (`run_acceptance.go`) is replaced by the round rule. The acceptance
  report names the closed rounds instead.
