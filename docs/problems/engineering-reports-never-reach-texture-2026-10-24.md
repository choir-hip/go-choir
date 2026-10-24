# Engineering's Reports never reach the owner's Texture document unless a management model chooses to relay them (2026-10-24)

Status: fix built on lane/fp0-reports-texture (switch off by default, not
deployed or drilled). Mutation class of the fix: `red`, additive and behind a per-computer switch that is off by
default (it adds records to Texture's inbox and so wakes Texture; Texture
stays the only writer of the document).

Gap C of docs/design/app-packages-replan-2026-10-24.md section 5.

## Evidence

The apprewrite run log, /Users/wiz/Dev/lanes/_runs/apprewrite/log.md
(read, not a ledger):

- v6, trajectory 9da63b10, Texture doc d4fe7dda: engineering sent four
  Reports (10:58:49, 10:59:26, 11:03:15 and one more); at 11:14 the doc
  was still at 2 revisions: "none of engineering's 4 Reports reached it".
- v7, doc a922c601: 2 revisions when the candidate reached
  awaiting_approval.
- v5, doc 679b943b: the Reports of 09:44:26 and 10:24:26 did reach it
  (Texture rev 3 at 09:45:13, rev 5 at 10:24:59: "Management relayed
  Engineering's interim four-app progress"). So the path works when a
  management turn relays, and not otherwise.

What the evidence does not contain: the guest ledger of v6 (management's
inbox, its cursor, its runs, the wake rows). The staging computer was not
read for this record. So the trace below is from code at e2d8fca61 (this
branch's base), and the cause of the v6 silence is stated as the code allows
it, with the reads that would settle it listed under Belief state.

## The trace of one engineering Report, to where it stops

The replan (section 5, gap C) read this as a producer-report packet to
management that waits behind a wake gate at
`management_controller.go:3233-3246` (`wakeUpdatedCoagent`). The code does
not support that reading for an engineering `choir.Report`: that gate is
the lifecycle-packet path (`rlm_reduce.go:2120`,
`commitLifecycleReportActIntent`), and engineering's Report never reaches
it. The trace:

1. Engineering's cell stages `choir.Report("management", packet)`. The
   manifest lets engineering address management only
   (`internal/deskmanifest/manifest.go:262`, `Addresses`).
2. `commitTray` sends every research, management and engineering cell
   through the records route first (`internal/agentcore/rlm_reduce.go:970-975`
   calling `commitRecordsRoute`, `inbox_records.go:549`).
3. `recordsRoute` maps engineering to `management:<owner>`
   (`inbox_records.go:94-97`), so the Report is a records-route act, and
   `commitRecordsRoute` returns it consumed: it never falls to
   `commitActIntent`, so `wakeUpdatedCoagent` is not on the path.
4. `inboxRecordForIntent` builds the record (`inbox_records.go:390-428`):
   an engineering Report is forced to progress (`final = false`; only
   Complete or cancel mints the final Report, C1e D3).
5. `CommitInboxRecords` commits it with a wake row for `management:<owner>`
   (`internal/store/inbox.go:546`; `RecordWakesReader`,
   `internal/store/management_tasks.go:64`: an agent-emitted record wakes
   its reader).
6. The wake starts a management records turn (`WakeRecordsInbox`,
   `inbox_records.go:870`), or, if a management turn is live, returns nil
   (`inbox_records.go:904`) and the turn-end re-check
   (`recheckRecordsInbox`, `inbox_records.go:1070-1126`) wakes one more.
7. The management turn is shown the record through `choir.Records()`. What
   it must do with an interim Report is not specified. The prompt
   (`internal/promptstore/defaults/management.yaml:16-22`) says engineering
   "answers you, not the Texture", to read its *final* Report in a later
   turn and to "end the turn" after reporting; the overlay says "Your next
   turn starts when engineering's final Report arrives"
   (`internal/runtimeprompts/overlays/rlm_management_runtime.yaml:45`). No
   sentence tells management to pass progress to Texture.
8. When the turn completes, `markTurnRecordsRead` marks what the turn was
   shown as read (`inbox_records.go:972`). A turn that read the Report and
   staged nothing for Texture has consumed it.
9. Texture reads only its own inbox (`loadCellRecords`,
   `inbox_records.go:144`; frame in `agentcore/texture_frame.go:134`). Its
   inbox is written by management (`recordsRoute` case management,
   `inbox_records.go:82-87`), research, and system records. Nothing writes
   an engineering Report there.

The Report stops at step 8 or 9: durable in management's inbox, read, and
never in Texture's. The only route to the owner's document is a management
model deciding to author a `ReportPacket` to `texture:<doc>`. That is the
v5 relay; v6 and v7 show it is not a guarantee.

## Root cause

The owner-facing surface (the Texture document) is fed by the commitment
ledger through a desk's discretion, not by the platform. Engineering
addresses management only (correct: I1, I2a, I26), management is a model
turn, and the one step that turns engineering progress into something
Texture can render is a free choice of that turn. The contract is silent,
the platform adds nothing, and a turn that ends without a relay consumes
the record. Substrate class: message delivery between desks (a symptom of
no deterministic projection from the ledger to Texture), not a lost wake.

Not the cause, from the code (and corrected here because the replan names
it): the `wakeUpdatedCoagent` gate. That gate exists, and is still real for
lifecycle-bound producer reports (Autopaper terminal-outcome packets and
the like), but an engineering `choir.Report` does not use it.

## Failure modes of the fix (written before the code)

The fix mints, in the same transaction as an engineering progress Report,
one platform-authored progress record into the Texture inbox of the request
the assignment serves. Ways it could fail, with the test that pins each:

1. The Report still stops at management: a progress record is not minted
   or not addressed to the right Texture. Test: engineering Report with
   the switch on leaves exactly one progress record in
   `texture:<doc>` of the assignment's parent conversation.
2. The switch is off and behavior changes anyway. Test: with the switch
   unset, the same Report mints nothing in Texture's inbox and the
   engineering and management side is byte-identical.
3. Authority widened: the progress record is shaped as engineering
   speaking to Texture, opening a conversation or a round, or closing one.
   Test: sender is `progress:<assignment>`, emitter `system`, not final,
   round 0; the Texture conversation head with management is unchanged
   (no open or close).
4. Texture does not wake, so the doc stays stale. Test: the record wakes
   its reader (`RecordWakesReader`) and mints a wake row for the Texture
   inbox; a clean management-task Report and other system records still do
   not.
5. A crash boot resumes work by waking Texture. Test: the relay is
   minted only by the live commit of an agent's Report, never by the boot
   closure; `closeOpenRoundsAfterCrash` mints none.
6. Replay or a retried cell mints a second record or fails the commit.
   Test: the relay record id derives from the Report's id and rides the
   same command, so a replay commits once (same digest) and a second cell
   produces its own.
7. A Report whose assignment has no parent Texture (a legacy, non-records
   binding) breaks the commit. Test: no relay, no error, the Report
   commits as before.
8. The relay carries engineering text the owner should not see, or loses
   it. The body is the Report's own packet, the same bytes management reads;
   the frame renders it as it renders any packet (`texturewrite.RecordsFrom`).
   It is a Note directive, not a Report, so no open-claim projection
   (`ResolveCommitments` skips directives) counts it as a second standing
   claim that goes overdue. Test: the directive body equals the source
   packet and the Texture frame shows its claims.
9. Texture sees it as a "closure" (a system record with no other body
   reads as one in `inboxRecordKind`). Test: the view's kind is
   `progress`.

## Belief state and remaining error field

Before: engineering's Reports during a run reach the owner when management
relays. After the fix, with the switch on, they reach Texture on every
Report without depending on management.

Remaining error field, to settle from the v6 computer's ledger (read only,
not done here):

- whether management's inbox held the four Reports and its cursor passed
  them (the consumed-without-relay branch above), or whether management
  was never woken for them (a lost or deferred wake);
- management's runs between 10:58 and 11:14 and the records each was shown;
- whether any `ReportPacket` from management to `texture:d4fe7dda` exists
  after rev 1.

Either outcome ends at a Texture that was not told, and the fix covers
both; the read decides whether a second, management-side defect exists.

Not covered: progress that Complete mints with a partial result
(`recordAssignedEngineeringReport`) is an assignment-row transaction and is
left for a follow-up. Texture's prompt is unchanged: a progress record
renders in its frame as any Report does; whether Texture revises on every
one is its judgement, and a prompt line for the new kind waits for a trace.

Heresy delta: discovered 1 (the replan's trace names the wrong path), 0
introduced, 0 repaired in this commit.

## The fix (red, additive, switched off by default)

`engineering.progress_to_texture`, a per-computer boolean in
`KnownComputerSettings` (`internal/store/computer_setting.go`), set by the
owner through `POST /api/computer-settings` and read at each commit, so it
changes at runtime with no restart. Default off. Off is the rollback: it
mints nothing, and the records already minted stay in the ledger as
ordinary evidence Texture has read or can reject.

With it on, `commitRecordsRoute` (`inbox_records.go`) appends, to the
transaction that commits an engineering cell's progress Report to
management, one system Note per Report into the inbox of the Texture whose
request the assignment serves (`engineering_progress_relay.go`; the Texture
is the counterpart of the assignment's `ParentConversationID`). The record
is sent from `progress:<assignment>`, so it is nobody's speech and opens or
closes no round; `RecordWakesReader` admits it as a fifth system record that
wakes (it is minted only by a live commit, never at a crash boot); Texture's
view labels it `progress`. Engineering still addresses management only,
management still reads the Report, Texture still decides whether and how
the document changes, and the final Report still goes through management's
D6 gate.

Class: red (it wakes Texture, a protected surface's writer, and adds to the
ledger). Tier 1 review: additive, behind a data switch, one-act rollback
that shows within a minute on the next Report. Rollback: set the switch
false; revert the commit for removal.
