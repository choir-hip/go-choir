# What Management Reads From A Terminal-Outcome Packet (C1e Step 7 Uncertainty)

Date: 2026-10-13. Station:
[C1e](../definitions/choir-reeng-c1e-engineering-on-records-2026-10-13.md),
batch B0 (the optional written answer to the main uncertainty).
Source: a read of `main` at 86e7ea0f. Line citations are to that tree;
three load-bearing ones were re-checked by hand
(`store/engineering_assignments.go:1719-1725`,
`agentcore/inbox_records.go:353-362` and `:682-686`,
`store/store.go:2400`).

This is the code half of the answer. The evidence half the station
requires before step 7 (records-side delivery, cursor and wake evidence
for succeeded, failed and cancelled outcomes) can only be gathered once
engineering converses on records (B1).

## The packet that reaches management

The synthetic run-outcome packet (`terminal-coagent-outcome-*`) does not
reach management in practice:

- Assignment and lifecycle runs skip it
  (`agentcore/research_checkpoint_fallback.go:94`, `:97-105`).
- `DispatchWorkerUpdate` refuses it inside a lifecycle trajectory unless
  it is an engineering producer report to management
  (`store/store.go:2677-2690`).
- Outside a trajectory, management settles it as
  `settled_non_executable` (`agentcore/management_controller.go:1027-1043`,
  `:1703-1711`).

The packet that does reach management is engineering's assignment report:
`buildEngineeringReturnPacket`, id `assignment-report:<ReportID>`,
direction `producer_report` (`store/engineering_assignments.go:1719-1763`).
It is minted in three places: a report (`:2251`), a cancellation
(`:2383`) and the progress-overdue observation (`:2715`). Genuinely late
reports mint none (`:2246`).

So step 7's "management's packet activation" is, concretely, the
engineering-to-management return path. It is the same path B1 moves onto
records. The Autopaper residue doesn't depend on it.

## What management reads, and what a records Report carries

| Packet fact | Used for | Records equivalent |
|---|---|---|
| `UpdateID` | activation, dedupe, `ConsumedDeliveryUpdateIDs` | `RecordID`, `InboxSeq` |
| `Direction`, `Role` | sorting into control, directive or report; admissibility | `Kind`=report and the `SenderInbox` prefix (no carrier class) |
| `AgentID` (assigned engineering agent) | must equal the producer work's assignee | desk inbox only. **Gap** |
| `TrajectoryID`, producer/target work item ids, `ControlBindingID`, `SourceRunID` | the work and trajectory join; delivered-page validation (`store/lifecycle_control_delivery.go:749-829`) | `ParentID`/`RelatedIDs` can name the Cast. The run, work and trajectory join is missing. **Gap** |
| `PayloadDigest`, `DeliveryAttempts` | integrity check; poison-packet cap | none. **Gap** |
| `Disposition`, `DeliveredToRunID`, `DeliveredAt` | per-run claim; outbox wake suppressed while the bound run is active; stranded unbind | per-desk cursor. The per-run start lives only in memory (`recordsRunStart`, `inbox_records.go:116`). **Gap** |
| `Packet.Kind` | admissibility (`execution_result`, `blocker`) | report, reply or closure only. **Gap**: no outcome class |
| `Summary`/`Content`, `Claims`, `Sources` | what the model reads through `choir.Updates()` | `Body`, `Report.Claims` (`ClaimID`, `Stance` lost), `EvidenceRefs` (not rendered by `inboxRecordView`) |
| `Packet.Notes` (`assignment_id`, `attempt`, `report_id`, `late`) | what the model reads | none. **Gap** |
| `WorkDisposition` | end of round | `Final` + `Report.StopReason` |

Acceptance depends on packets even though it reads none of these fields.
`persistentManagementBoundReport`
(`agentcore/tools_engineering_assignment.go:114-288`) and
`QueueLifecycleUpdate` (`store/lifecycle.go:3757-3821`) require every
packet delivered to the run and seen in its memory to be listed in
`ConsumedDeliveryUpdateIDs`. They refuse a terminal `WorkDisposition`
while a delivered packet is unseen. Records have no counterpart. The
station must either redefine that rule against the cursor or carry a
per-run read mark.

## Wake paths

**Packet path.**
- An outbox wake (`coagent_result`) is suppressed while the bound run is
  active (`store/lifecycle.go:536-584`).
- The handler resumes the exact parked casting run (`handler.go:619-628`,
  `:818-826`).
- A report never mints a new management run (`store/store.go:2400`).

**Records path.**
- `inbox_record` wakes call `WakeRecordsInbox`, which starts a fresh run
  when head > cursor and nothing is pending or running
  (`inbox_records.go:506-563`).

For management, the records path is the wrong shape in four ways:
1. A parked casting run (passivated, awaiting the delegated cast) is not
   active, so a record starts a new run instead of resuming it.
2. Engineering casts from records turns are refused
   (`inbox_records.go:353-362`).
3. Engineering has no records route (`recordsDesk`/`recordsRoute`,
   `:31-86`).
4. `closeRoundsOfEndedRun` closes rounds only for research and management
   (`:682-686`), so an engineering run that ends without a final Report
   leaves management's round open.

## Gap list before packet activation can be deleted

On the record:
1. Assignment identity: `assignment_id`, `attempt`, `report_id`, the late
   flag and the source run.
2. A typed outcome. Today `execution_result` covers both completed and
   partial, and `blocker` covers failed, blocked, cancelled and overdue
   (`engineering_assignments.go:1702-1706`). The record needs the result
   (completed, partial, failed or blocked), the verification verdict,
   the assignment's terminal disposition, and the candidate it certifies.
   Otherwise management keeps reading the assignment row.
3. A terminal marker. Partial and overdue notices are not `Final`.
   `StopReason` must distinguish completed, failed and cancelled; a
   cancellation exists today only in summary text and Notes
   (`:2348-2362`).
4. The trajectory, work and control join that acceptance validates.
5. A per-run delivery or read mark, or the acceptance rule redefined
   against the cursor.
6. An integrity digest and an attempt cap.
7. The sender's assigned agent, not just the desk.

On the wake path:

8. Resume the exact parked casting run.
9. Allow engineering casts from records turns.
10. Close the round when an engineering run ends without a final Report.

## What this changes in the station

- Conjecture C1e-1 ("records already carry everything the work-item and
  packet tables carry for engineering … without a new mechanism") is
  **contradicted on the code read** for items 1-3, 5 and 8. Records do
  not carry the assignment identity, the typed outcome or a cancelled
  fate, and the wake path cannot resume a parked caster. B1 must add
  these to the Report body and the records wake, or name where management
  reads them instead (the assignment row is the candidate: items 1-2 are
  facts of the assignment, not the message). Whether that counts as a
  "new mechanism" is a decision for B1's review. It is recorded here as
  a residual, not settled.
- Items 9, 10 and the records route (3 in the wake list) are B1's own
  scope already (steps 1-4).
- Step 7 is no longer the place this uncertainty bites. It bites in B1.
  Step 7 inherits whatever B1 lands.
