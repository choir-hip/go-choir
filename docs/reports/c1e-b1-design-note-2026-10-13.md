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

Revised again after round 2 (claude, codex, gpt-6.1-sol and
deepseek-v4.1-flash sent it back; gemini-3.8-flash and glm-5.3-flash
accepted). The round-1 repairs were confirmed. Round 2 found five gaps,
each named by at least two panelists:
- G1: a partial Complete would close the round;
- G2: an automatic closure could still close Texture's round over an
  unread outcome;
- G3: an engineering run's end closed its round without settling the
  assignment;
- G4: the crash path tried a second final Report;
- G5: the trajectory lookup did not pick the assignment's operation, and
  the Cast record's order against the assignment open was undecided.

Their fixes are marked *(Round 2: …)*.

Revised a third time after round 3 (claude, codex, gemini-3.8-flash and
glm-5.3-flash sent it back on narrower points; gpt-6.1-sol hit its usage
limit). Round 3 confirmed G1, G3 and G4, and the identity half of G5. It
found four more gaps:
- H1: the trajectory lookup still serves five more callers, the cancel
  among them;
- H2: `commitRecordsRoute` commits a cell's records before any intent
  runs, so the Cast cannot reach the assignment's transaction as written;
- H3: the D5 skip and the re-check use different predicates, which can
  strand Texture's round;
- H4: a passivated run is still cancelled by the asynchronous reconcile.

Their fixes are marked *(Round 3: …)*.

**Round 4 accepted** (claude, gemini-3.8-flash and glm-5.3-flash accept,
with no blocking finding; deepseek pending; gpt-6.1-sol and codex
usage-limited). All four round-3 fixes were confirmed against the code.
Their build notes are adopted in "Build notes from round 4" below.

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
the assignment, report and operation ids.
*(Round 1: the row does not hold the result or the operation; the chain
does.)*

The operation is found by the assignment, never by the trajectory. The
freeze opener already gives each assignment one exact operation: id
`selfdev-` plus the digest of computer and assignment id, idempotency key
`freeze-open:<assignmentID>` (`freeze_candidate.go:28-48`). B1 changes
Freeze and Verify to resolve by that key (`GetByIdempotency`) instead of
`GetByTrajectory` (`tools_capsule.go:142`, `rlm_reduce.go:1402,1457`).
Management's Verify takes the assignment id from the Report it read (its
`EvidenceRefs`, rendered in the record view by D9).

**Every caller scoped to one assignment resolves by that key**, not only
Freeze and Verify:
- the cancel's operation close, `failBoundSelfdevOperation`
  (`engineering_assignment_fate.go:241,349`);
- the freeze-tool gating (`tool_profiles.go:239,490`);
- the verification spawn (`engineering_assignment_runtime.go:667`);
- desk reconcile (`engineering_desk.go:321`);
- run acceptance (`run_acceptance.go:427`).

A caller that is truly trajectory-wide lists every operation on the
trajectory and picks explicitly, never through a `QueryRow` with no
ORDER BY. Step 1's inventory lists every `GetByTrajectory` caller, and B1
deletes `GetByTrajectory` once the list is empty.

The opener's trajectory pre-check (`freeze_candidate.go:30-33`) is
deleted, not relaxed. `Start` already refuses any open operation across
the computer (`selfdev/operations.go:224-228`), and it does so atomically
with creation. So a retry after a failed assignment opens its own
operation, and two candidates can never be open at once. B1 keeps that
lock. The station's later removal of it must first put an equally
atomic guard in its place. *(Round 3, H1: the pre-check refused every
retry, and five callers would have read whichever row the query
returned, including the cancel, which could have failed the retry's live
operation.)* *(Round 2, G5: `GetByTrajectory`
takes whichever row the query returns, and Freeze returned an earlier
frozen bundle without reading the new capsule.)*

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
- **A final Report is minted only when the row turns terminal in that
  batch.** A partial Complete leaves the row open
  (`store/engineering_assignments.go:1686-1687`), so its Report is
  non-final progress into the open round. *(Round 2, G1: a final on a
  partial Complete closed the round, the later terminal Complete was
  refused, and the row stranded.)*
- The cancellation path (`:2383`) mints the same final Report with
  StopReason `cancelled`, atomic with the row, but only while the
  engineering round is open. If the round is already closed (only the
  crash closure does that, D7), the row turns terminal alone. That makes
  cancellation idempotent against every closer.
- Reports the engineering cell stages through `choir.Report` are
  progress. The reducer strips finality from any Report an
  `engineering:*` sender stages, so only Complete's or the cancel's
  transaction mints a final. *(Round 2: reusing `reportBodyFromPacket`,
  `inbox_records.go:330-345`, would derive Final from a "completed"
  body.)*
- A non-final record into a conversation with no open round commits at
  round 0 today (`store/inbox.go:214-239` refuses only finals). B1 adds
  that refusal for `engineering:*` senders only, since other desks send
  round-0 notes. *(Round 2: the note had claimed this was existing
  behavior.)*
- `validateCellIntents` already makes Complete the last intent of a cell
  (`rlm_reduce.go:122-126`), and Freeze precedes it. That is the commit
  order.

*(Round 1: the reducer commits records before intents, `rlm_reduce.go:875-918`,
so a staged final Report would precede Complete.)*

**D4. The engineering Cast names the Texture request it serves, and the
Cast and the assignment commit together.** The
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

Order, so that no round exists without its assignment. The records route
is where it happens, because `commitRecordsRoute` commits a cell's
records and cursor in one `CommitInboxRecords` before the intent loop
runs (`inbox_records.go:415-430`, `rlm_reduce.go:880-884`):
- The assignment id is derived before anything commits, from the Cast's
  pre-commit `RecordID` (`cellID:Cast:localID`, `rlm_reduce.go:1143`),
  the owner, the computer and the document's trajectory. So the Cast can
  be addressed to `engineering:<assignmentID>` when it is minted.
- D4's parent check, the trajectory lookup and spawn preflight all run
  before commit. A failure refuses the cell's Cast, and nothing is
  written.
- When a cell holds a Cast to `engineering:*`, `commitRecordsRoute` does
  not call `CommitInboxRecords`. It plans the cell's whole records batch
  (every record, the cursor advance and the verdicts) with
  `planInboxRecords` and hands the plan to `openDelegatedCastAssignment`.
  That function composes it into `OpenEngineeringAssignment`'s
  transaction. So the cell's records, the cursor, the assignment row and
  the binding commit together, one transaction per cell, as before.
- `openDelegatedCastAssignment` is the one mint, and the checks live in
  it. Its two current gates go, replaced by D4's checks: "requires a
  trajectory-bound caster run" and "caster has no open work item"
  (`delegatedCastParentWork`). Records management runs have neither.
- `rlm_reduce.go:1042` is not used by records turns. `:1990`
  (`commitLifecycleActIntent`, the packet path) loses its engineering
  branch in B1: engineering casts from packet-path control turns are cut,
  and the step-1 commit names that as a behavior change.
- The assignment id comes from the pre-commit `RecordID` everywhere. The
  lifecycle path's committed canonical id (`rlm_reduce.go:1994`) goes
  with `:1990`'s engineering branch, so the two are never mixed.

*(Round 3, H2: the Cast would have committed in the records route, and
the cited call sites were never reached.)*

*(Round 2, G5: the Cast committed in `commitRecordsRoute` before
admission ran, so a failed admission left a round with no assignment and
no run, which nothing closes.)*

**D5. No resume, and Texture's round survives the casting turn.** A
Report or closure into `management:<owner>` wakes a fresh `inbox_record`
turn, so nothing parks. `managementRunCasts`, `managementParkDecision`
and `awaiting_delegated_cast` serve only `lifecycle_texture_control`
turns (`management_controller.go:2590-2593`, `:2637`). They are deleted
with management's packet activation in step 7, not with path B. Because
D4 cuts engineering casts from those turns in B1, the park has nothing
to wait on from B1 onward. *(Round 2: the deletion was mis-scoped.)*
- `closeRoundsOfEndedRun` (`inbox_records.go:675-716`) closes every
  incoming round of an ended management turn. It skips a round with
  `texture:<doc>` while any assignment whose parent is that round (D4)
  is non-terminal, **or** is terminal but the record that closed its
  engineering round has an `InboxSeq` above management's durable reader
  cursor **and** was committed after the ending turn began. That cursor
  advances only to what a turn was shown, so "above the cursor" means
  "not yet read". The second clause is the re-check's own wake predicate
  (`inbox_records.go:610-617`), so a skipped round always has a reader
  coming. Otherwise the round closes `run_<state>`, which wakes Texture,
  as today. *(Round 3, H3: a turn that failed before reading left the
  outcome unread but older than the turn, so the round was skipped and
  nobody was woken.)* *(Round 2, G2: an outcome that
  arrived during the last call made the child terminal, so the old skip
  let the round close before the re-check woke a reader.)*
- The skip and D6 share one helper: assignments by parent conversation
  and round, a scan of the owner's assignment objects filtered on the
  binding's parent fields. There is no new table.
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
`inbox_records.go:141`). That record is found by its id: D3's final
Report gets a record id derived from the assignment report's `ReportID`,
so the row's `ReportRefs` lead to it (`listInboxRecordsBy`,
`store/inbox.go:676`). *(Round 2: the link from row to record was
unnamed.)* A row terminalized at a crash boot has no such record, but
then the crash also closed Texture's round, so D6 never runs for it. It is checked in the reduce step that commits
the Report. Scoping to the parent round means two documents' assignments
never wait on each other. A crash loses the in-memory shown mark, but a
crash closes every round anyway (no resume). This replaces the
`ConsumedDeliveryUpdateIDs` rule (`store/lifecycle.go:3783-3800`).
*(Round 1: a closed round can arrive mid-call, unread.)*

**D7. One terminal transition per assignment, and who wakes.** System
records wake no one (`store/inbox.go:191-208`), so that a boot never
resumes work.
- **A live engineering run that ends without Complete settles its
  assignment; it does not write a bare closure.** `closeRoundsOfEndedRun`
  gains an engineering branch that runs only on terminal run states, not
  on passivation, so a parked capsule run keeps its round.
- Passivation is consistent across all three places: the run-end branch,
  the reconcile kick (`runtime.go:1780`) and reconcile eligibility
  (`assignedEngineeringCapsuleUsable`, `engineering_assignment_fate.go:98`).
  The kick fires only on terminal states. A passivated run on a live
  runtime counts as usable, so its capsule is not revoked. A crash boot
  still terminalizes it. *(Round 3, H4: the kick fired on passivation and
  eligibility accepted only pending or running, so reconcile cancelled the
  parked run D7 meant to keep.)*
- The branch revokes the capsule first, because the store cancel requires
  `CapsuleRevoked`. If the revoke fails, the round stays open until the
  asynchronous reconcile or the next restart settles it. The branch
  calls the assignment's cancel synchronously, with reason
  `run_<state>`. The cancel turns the row terminal and mints the final
  Report (D3) in one transaction. That Report closes the round and wakes
  management like any agent Report. The asynchronous fate reconcile
  (`runtime.go:1780-1799`) then finds a terminal row and does nothing. A
  bare `run_<state>` closure is written only when no non-terminal
  assignment matches the conversation. *(Round 2, G3: the synchronous
  closure ran before the asynchronous cancel, whose final Report was then
  refused, so the row never turned terminal.)*
- A spawn failure already calls `CancelEngineeringAssignment`
  (`engineering_assignment_runtime.go:613-631`), whose final Report under
  D3 is the closure. No separate spawn-failure closure is minted.
  *(Round 2: it would have been a duplicate.)*
- Live run-end closures of management's own rounds already wake the
  caster by an explicit `DispatchActor` call (`inbox_records.go:711-713`).
  `inboxRecordWakes` gains no exception. *(Round 2: the note described
  the wake path wrongly; both together would wake twice.)*
- **Crash path.** The boot closes every open round
  `interrupted_by_restart`, silently, as today.
  "Mint no Report" is gated on the boot kind (crash) as well as on the
  round state, so one failed crash closure (`runtime.go:651`, `continue`)
  cannot let the boot cancel mint a Report that wakes management. On a
  crash boot the reconcile also skips its pending-proposal resume branch
  (`engineering_assignment_fate.go:524`). Boot reconciliation
  (`runtime.go:756-765`) then turns each open assignment terminal with
  disposition `interrupted_by_restart`. Its round is already closed, so
  D3's cancel mints no Report and wakes no one. Nothing resumes, and no
  assignment row stays open across a crash.
- **Planned restart.** Rounds stay open, and a passivated engineering run
  takes the planned restart's one resume. A cancel during a planned boot
  mints its final Report and wakes management, which a planned restart
  allows.
- A late Complete, or a report on a row that is already terminal, mints
  no record. Its evidence is kept on the late-evidence path
  (`engineering_assignment_fate.go:613-616`), as today. *(Round 2, G4: boot
  reconciliation tried a second final Report into a crash-closed
  round.)*
- Cancellation authority moves from the exact parent run
  (`cancelAssignedEngineering` requires "the exact recorded parent",
  `engineering_assignment_fate.go:166-180`) to the management desk that
  is the engineering conversation's caster. Under D5 the canceller is a
  fresh turn, never the casting run. The `cancelled` final Report of D3
  is sent as engineering and wakes management like any Report.
- Whether the store cancel (`CancelEngineeringAssignment`,
  `store/engineering_assignments.go:2300`) stops the engineering run is
  not settled by reading. Step 1 checks it first, with a test. If it does
  not, the cancel also cancels the run through the runtime's run cancel.
  The cancelled run then ends with its row already terminal, so the
  engineering run-end branch above finds nothing to settle and writes
  nothing.
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

**D9. Management can see what it must name.** A fresh management turn
reads records through `inboxRecordView` (`inbox_records.go:225-245`),
which renders neither `ParentID`/`RelatedIDs` nor `EvidenceRefs`, and the
Texture request sits below the cursor when the turn starts. For
engineering Reports the view gains both: the parent conversation and
round from the binding, so the turn can answer `texture:<doc>`, and the
evidence refs, so Verify can name the assignment (D2). It is a view
change; nothing new is stored. The desk-side struct
`yaegikernel.InboxRecord` (`internal/yaegikernel/intent.go:100-108`)
gains the same three fields. *(Round 2: the woken turn could not name
the request D6 checks or the assignment Verify needs.)*

## Build notes from round 4

- **Compose the whole plan, not only `planInboxRecords`.**
  `CommitInboxRecords` also plans support verdicts
  (`planSupportVerdicts`), story openings (`planStoryOpenings`) and the
  `choir.inbox_command` object that makes cell replay idempotent
  (`store/inbox.go:567-597`). Move its body after the lock into one plan
  function that both paths call. Plan inside `OpenEngineeringAssignment`'s
  `trajectoryMu`: pass the request, not a stale plan. On replay, the
  receipt sequence numbers come from the stored inbox command.
- **One engineering Cast per cell.** `validateCellIntents` refuses a
  second one.
- **Trajectory-wide callers.** `textureRunSelfDevelopmentOperation`
  (`tool_profiles.go:490`, Texture has no assignment) and
  `run_acceptance.go:427` list every operation on the trajectory and pick
  explicitly. So does `engineering_desk.go:321`. No list-by-trajectory
  query exists today; step 1 adds one.
- **D5's skip uses `rec.CreatedAt` exactly**, the re-check's own field,
  so the two predicates cannot drift.
- **A passivated run counts as usable on a planned boot too.** Boot
  reconcile runs after `passivateInterruptedActivations`, and excluding
  the run there would cost it the planned restart's one resume.
- **Re-freeze after the mode is switched off.** With the pre-check gone,
  the opener calls `GetByIdempotency` first and returns an existing
  operation before `authorizeProposal`.
- **Store cancel and capsule revoke.** The store cancel branches on
  `CapsuleDisposition == Revoked` for parent authority rather than
  strictly requiring it. Step 1's test settles whether revoke-first is
  needed.
- **Citations.**
  - `yaegikernel.InboxRecord` is at `intent.go:153-185`.
  - The crash closure that ends in `continue` is in `inbox_records.go`,
    not `runtime.go:651`.
  - The Verify lookup is at `rlm_reduce.go:1456`.
  - In `engineering_assignment_fate.go`, `failBoundSelfdevOperation` is
    called at `:241` and does its lookup at `:349`.
- **Before B1's paper run.** Confirm that records-spawned engineering
  runs carry the binding's trajectory, which
  `kickEngineeringReconcileAfterPersist` needs.

## What B1 does not add

There is no new table or record kind, and no field on
`CommitmentRecord`. The binding swaps two work-item ids for a parent
conversation and round and an assigned conversation. Cancellation gains
reasons (`run_<state>`, `interrupted_by_restart`); the closure constructor
already takes any reason. `inboxRecordWakes` is unchanged. The record
view renders two existing fields.

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
7. A partial Complete, then a terminal one. The first mints a progress
   Report, the second the final (D3). Tested.
8. An engineering run dies with its assignment open. The run-end branch
   cancels it in one transaction; the later asynchronous reconcile does
   nothing (D7). Tested in both orders.
9. Engineering finishes while management's last call is in flight. The
   outcome is above the cursor, so Texture's round survives the turn's
   end, and the re-check wakes a reader (D5). Tested.
10. A retry on the same trajectory after a failed assignment. It opens
    its own operation; Freeze and Verify resolve by assignment and never
    see the earlier bundle (D2). Tested.
11. A cast whose admission fails. Nothing commits: no Cast, no round, no
    row (D4). Tested.
12. The paper path. Autopaper's terminal-outcome packets do not cross
   this route (packet-reads report), so B1 does not change them. They go
   in step 7.

## Residuals

- Step 2 names the document-to-trajectory lookup D4 relies on. If a
  document can have more than one live trajectory, the cast must name
  one.
- The run acceptance report (`run_acceptance.go`) names closed rounds
  instead of consumed packet ids. It must tell a successful answer (a
  final Report by management) from a system closure (`run_<state>`,
  `interrupted_by_restart`); a closed round alone is not acceptance.
- The records drill needs no verification-kind assignment: management's
  Verify intent is the verification. Path B was the only
  `Kind=Verification` mint, and it is deleted.
- Store cancel versus run cancel (D7) is still checked by a test first.
