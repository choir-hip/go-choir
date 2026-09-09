# Settlement Gate Item 6 — Code-Free Define Receipt (2026-09-09)

Mutation class: green (docs only). No source changed in this receipt.
Mission: `docs/definitions/choir-rlm-settlement-gate-2026-09-09.md`, acceptance item 6.
Prior: item-5 repair `0921c542` (resumable fate saga with atomic final boundary after revoke acknowledgement).
Worktree: 4 untracked unrelated-WIP paths preserved (mission-0 completion report,
report generator script, pycache, tmp/).

## Problem (observed, not inferred)

1. **Dual authors of terminal truth.**
   In `internal/agentcore/researcher_checkpoint_fallback.go:124-167`, `ensurePersistedTerminalRunOutcome`
   synthesizes worker updates (`terminalOutcomeReferenceUpdatePrefix`), binds them via
   `rt.store.BindWorkerUpdateTerminalOutcome` or `rt.store.DispatchWorkerUpdate`, emits
   channel events via `rt.emitChannelMessageEvent`, and triggers parent wakes via `rt.wakeUpdatedCoagent`
   (`:41-43`).
   This creates a competing author of terminal truth outside the reducer.
2. **Metadata-conditional exemption leak.**
   The fallback's exemption check (`:59-67`) checks `hasLifecycleMarker` via metadata
   `lifecycle_work_item_id` or `work_item_ids`. If a CoSuper run lacks these metadata keys
   or if computer ID is missing, the fallback activates and authors a canonical write,
   bypassing the assignment's reducer-owned terminal settlement.
3. **Orphan vs. pending collision.**
   If a delegated child run terminates authoritatively (or is interrupted) while an assignment
   carries a pending proposal (Item 5), the fallback could attempt to synthesize an outcome
   rather than reconciling through the assignment's resumable fate saga.

## Authorized repair boundary (item 6 only; red, next commit)

1. **Strip canonical write and wake authority from the fallback:**
   In `researcher_checkpoint_fallback.go`:
   - Remove `DispatchWorkerUpdate`, `BindWorkerUpdateTerminalOutcome`, and `emitChannelMessageEvent`.
   - Remove inline `rt.wakeUpdatedCoagent` calls.
   - The fallback becomes an observation producer only: producing an immutable `CoSuperOrphanObservation`.
2. **Single reducer-owned orphan port (`internal/store/cosuper_assignments.go`):**
   - Add `RecordCoSuperOrphanObservation`: accepts an authenticated immutable orphan observation
     (scope, child/run identity, terminal observation, reason enum, timestamps, evidence).
   - Reducer validation:
     - **Obligation check:** if the child run is a `BoundRunID` on an assignment, refuse orphan disposition
       and route through the Item 5 pending saga (the delegated-run slot exists only where no assignment obligation exists).
     - **Slot protection:** an unreserved slot may reserve an orphan proposition; a pending slot reconciles
       through its saga; an accepted or rejected reservation retains its proposition with the same replay and conflict rules as the primary path.
     - **Proposition derivation:** reducer derives terminal proposition in the `TerminalPropositionV1` domain,
       mints `ReportID` in the shared slot family, commits report, advances disposition, and emits parent update via outbox.
3. **Liveness timeout restriction:**
   Liveness timeout alone never opens this path and never expires a pending freeze/revoke reservation.
4. **Caller-map proof:**
   Add unit and caller-map tests proving:
   - No non-reducer path can bind a terminal outcome, emit a canonical update, or wake.
   - Interrupted runs with a pending proposal reconcile through the saga, never through the orphan path.
   - Replay/conflict rules apply uniformly to orphan observations.

## Explicitly not in this boundary

Admission grammar (item 4), fate saga (item 5), staging deployment proof (item 7).

## Evidence floor / rollback

Floor: local tests for orphan observation submission, obligation routing, fallback authority removal,
and caller-map proof.
Rollback: revert the repair commit; this Define receipt retained.

## Standing-questions answers for this boundary

1. Settled by: owner-chartered definition item 6 (single author of terminal truth, owner direction 2026-09-09).
2. Topology: conforms to charter entrypoints (`researcher_checkpoint_fallback.go`, `cosuper_assignments.go`).
3. Deletion citers: direct `DispatchWorkerUpdate` and `BindWorkerUpdateTerminalOutcome` calls in `researcher_checkpoint_fallback.go` deleted.
4. Consumers: lifecycle reconciliation, child run termination monitors.
5. Single authority: reducer alone derives and commits terminal truth; fallback explains and observes only.
6. Artifact: local_test orphan observation contracts and caller-map proof.
7. Fate-sharing: orphan observations are immutable and idempotent.
8. Restart: orphan closure is durable in Dolt; duplicate observations replay.
9. No SSH: local unit and integration tests.
