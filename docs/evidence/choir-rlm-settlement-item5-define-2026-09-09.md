# Settlement Gate Item 5 — Code-Free Define Receipt (2026-09-09)

Mutation class: green (docs only). No source changed in this receipt.
Mission: `docs/definitions/choir-rlm-settlement-gate-2026-09-09.md`, acceptance item 5.
Prior: item-4 repair `18498447` (narrow assigned-CoSuper admission grammar, sequential execution, and toolloop decoupling).
Worktree: 4 untracked unrelated-WIP paths preserved (mission-0 completion report,
report generator script, pycache, tmp/).

## Problem (observed, not inferred)

1. **Premature terminal finality before capsule revocation.**
   In `internal/agentcore/cosuper_assignment_fate.go:762-780`:
   `commitAssignedCoSuperReport` is called at line 770 immediately upon freeze acknowledgement
   (`assignment.CapsuleDisposition == types.CoSuperCapsuleFrozen`).
   `commitAssignedCoSuperReport` writes the final report to the store (`store.RecordCoSuperAssignmentReport`),
   advances assignment disposition to `Completed` or `Failed`, commits terminal events, and updates the parent coagent.
   Only *after* that terminal commit does line 777 call `rt.revokeAssignedCapsule`.
   If a crash, network partition, process termination, or executor error occurs between lines 770 and 777,
   terminal truth is permanently committed and visible in the database while the capsule remains
   unrevoked (`CapsuleDisposition == CoSuperCapsuleFrozen`).
2. **Replay bypasses incomplete revocation.**
   In `cosuper_assignment_fate.go:644-650`:
   ```go
   if lateFate && reportExists {
       result, replayErr := rt.store.ReplayRecordedCoSuperAssignmentReport(...)
       if replayErr == nil {
           return result, nil
       }
   }
   ```
   If a retry or late submission arrives after a crash between freeze-commit and revoke,
   `reportExists` is true, so `recordAssignedCoSuperReportOnce` returns the replay result immediately,
   bypassing revocation entirely and permanently stranding the capsule in a frozen unrevoked state.
3. **Absence of durable pending proposal.**
   The attempt's terminal proposal currently exists only in transient executor/agentcore memory
   between freeze request and report commit. Without computer-durable pending proposal state
   recorded in Dolt before physical actions, crash recovery cannot distinguish an ongoing
   settlement saga from an abandoned run, risking orphan close or premature timeout.

## Authorized repair boundary (item 5 only; red, next commit)

1. **Durable pending proposal schema (`internal/types/cosuper_assignment.go`):**
   Add `PendingProposal *CoSuperPendingProposal` to `types.CoSuperAssignment`:
   ```go
   type CoSuperPendingProposal struct {
       PropositionDigest string                     `json:"proposition_digest"`
       Report            CoSuperAssignmentReport    `json:"report"`
       FreezeIntentRef   string                     `json:"freeze_intent_ref"`
       RevokeIntentRef   string                     `json:"revoke_intent_ref,omitempty"`
       CreatedAt         time.Time                  `json:"created_at"`
   }
   ```
   A pending proposal occupies the Item 3 slot: competing terminal submissions with the same digest replay,
   while different digests conflict immediately.
   Assignment disposition remains `Bound` (strictly non-terminal). No `TerminalAt`, no candidate publication,
   and no parent wake are emitted while pending.
2. **Store methods for pending proposal and atomic finalization (`internal/store/cosuper_assignments.go`):**
   - Add `SetCoSuperPendingProposal`: records the pending proposal on the assignment within a store transaction.
   - Add `FinalizeCoSuperAssignmentSettlement`: executed only when `CapsuleDisposition == CoSuperCapsuleRevoked`.
     In a single atomic transaction:
     - Clears `PendingProposal`.
     - Records the terminal `types.CoSuperAssignmentReport`.
     - Advances assignment disposition to `Completed` or `Failed`.
     - Publishes candidate artifact (if passed).
     - Emits parent update, receipt, outbox, and wake.
3. **Saga reordering in `cosuper_assignment_fate.go`:**
   Refactor `recordAssignedCoSuperReportOnce`:
   - Step 1: Durably commit `PendingProposal` in the store before issuing physical freeze.
   - Step 2: Issue fenced freeze $\to$ record `CoSuperCapsuleFrozen` with freeze ack.
   - Step 3: Issue fenced revoke $\to$ record `CoSuperCapsuleRevoked` with revoke ack.
   - Step 4: Call `FinalizeCoSuperAssignmentSettlement` to publish finality atomically.
   - Step 5: If an interrupted run resumes with `PendingProposal != nil`:
     - If capsule not frozen $\to$ resume freeze.
     - If capsule frozen but not revoked $\to$ resume revoke.
     - If capsule revoked $\to$ finalize settlement.
     Never orphan a reserved pending slot!
4. **Tests:**
   Add tests in `internal/agentcore` and `internal/store` verifying:
   - No terminal state is visible in store while capsule is frozen-unrevoked.
   - Crash after freeze-ack resumes through revoke to final settlement.
   - Crash after revoke-ack resumes to final settlement without re-issuing physical actions.
   - Pending slot blocks competing different-digest proposals.

## Explicitly not in this boundary

Admission grammar (item 4, completed in `18498447`), fallback orphan close port (item 6),
physical staging proof (item 7).

## Evidence floor / rollback

Floor: local tests for pending proposal commit, freeze $\to$ revoke $\to$ finalize sequence,
crash resumption, and no-terminal-before-revoke invariant.
Rollback: revert the repair commit; this Define receipt retained.

## Standing-questions answers for this boundary

1. Settled by: owner-chartered definition item 5 (fenced fate saga with atomic final boundary).
2. Topology: conforms to charter entrypoints (`cosuper_assignment_fate.go`, `cosuper_assignments.go`).
3. Deletion citers: premature commit in `cosuper_assignment_fate.go:770` replaced by atomic finalize after revoke.
4. Consumers: agentcore capsule fate supervisor, store assignment lifecycle.
5. Single authority: reducer-owned Dolt state; pending proposal is computer-durable.
6. Artifact: local_test saga state transition and resumption tests.
7. Fate-sharing: physical operations are idempotent fenced operations of the pending proposal.
8. Restart: pending proposal survives crash/restart and resumes through saga.
9. No SSH: local unit and integration tests.
