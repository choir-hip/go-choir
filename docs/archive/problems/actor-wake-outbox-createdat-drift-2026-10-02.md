# Actor wake outbox body drift on CreatedAt: pending wakes falsely CAS-fail as concurrent state changes

Date: 2026-10-02
Status: **problem documented; fix landed in the same worktree session** (kept as a
separate commit per problem-documentation-first).
Mutation class: **orange → red-adjacent** (lifecycle commit-time CAS in
`commitLifecycleTransition`; protected surface: lifecycle delivery path).
Heresy delta: **discovered** (pre-existing, introduced by `a51be4b4` wake-drift
guard + `a2b87d69`/`a7e31232` wake re-mint); **repaired** by this mission's fix.

## The failure

`commitLifecycleTransition` (internal/store/lifecycle.go) derives a durable
actor-wake outbox for every committed object and, when an unprojected wake
already exists at the same canonical key, enforces a byte-compare on the
wake body as the concurrent-state-change signal:

```go
} else if !bytes.Equal(existing.Body, outbox.Body) {
    return types.LifecycleResult{}, ErrConcurrentStateChange
}
```

The wake body is `ActorWakeOutbox`, whose `created_at` is copied through from
the source object's *object-level* `CreatedAt` (`actorWakeOutbox`:
`wake.CreatedAt = source.CreatedAt.UTC()`). That field is bookkeeping, not
obligation content — but it is not stable across a store round-trip:

- At mint time the source object is in-memory: `created_at` carries
  nanoseconds (e.g. `2026-10-02T04:38:09.647627Z`).
- The Dolt object-graph `DATETIME` column rounds sub-second precision
  (`.5+` rounds up). On the next reconcile, `GetObject` returns the source
  with `CreatedAt = 2026-10-02T04:38:10Z`.
- The re-minted wake body differs by exactly the rounding delta (6 bytes
  observed) → `ErrConcurrentStateChange`.

## Receipts

Observed 2026-10-02 while verifying the S0m RN2 stranded-control repair:
`ReconcileUpdateDelivery` (unbind of a control packet bound to a passivated
run) committed an updated packet object; the re-derived wake outbox body
differed from the still-pending wake only in `created_at`, and the CAS
rejected the batch. Debug-dump diff (only differing field):

```text
existing: {"created_at":"2026-10-02T04:38:09.647627Z", ...}
new:      {"created_at":"2026-10-02T04:38:10Z",        ...}
```

Failing tests (7, all stranded/lifecycle-control reconcile paths):
`TestPersistentManagementLifecycleControlsStayTrajectoryIsolatedThenReconcile`,
`TestPersistentManagementReconcilesOtherTrajectoryAfterTerminalRun`,
`TestPersistentManagementRewakeReceivesPendingEngineeringCancellationReports`,
`TestPersistentManagementEngineeringCancellationDoesNotMintManagementReportAndSettles`,
`TestPersistentManagementProducerReportSettlementCASOnReplayIdempotent`,
`TestSchedulingReadiness_Criterion4_ProducerReportStoreSettlement`,
`TestPersistentManagementResearchOpenWorkNeedsExactControlAndNotThinSuccessor`
(internal/agentcore).

## Why it matters

Any lifecycle transition on a source object whose wake is still pending and
whose object timestamp was rounded on storage CAS-fails forever — the wake
never clears because its canonical key re-mints with a bookkept timestamp
that no longer matches. Stranded-control unbind, assignment progress
deadlines, and cancel intents all route through this code path. The desk
live-locks: unbind required for redelivery is itself the transition that the
false drift rejects.

## The fix (same commit chain, separate commit)

In the drift branch, decode both wakes, zero `CreatedAt` on both, and compare
the decoded structs. `CreatedAt`/`CanonicalID` are bookkeeping (`CanonicalID`
is `json:"-"`); every remaining field — `SourceUpdateID`, `UpdateID`,
`OwnerID`, `ComputerID`, `TargetAgentID`, `TrajectoryID`, `AgentID`, `Kind`,
`Content`, `NotBefore` — is obligation identity and still guards real drift.
The projected-wake re-arm path is untouched.

Belief update: timestamp-valued fields that pass through a Dolt `DATETIME`
column cannot serve as byte-compare inputs on second reads; either normalize
at mint or compare decoded bodies with volatile fields zeroed. The pattern to
audit for siblings: any `bytes.Equal(existing.Body, x.Body)` where `x.Body`
embeds a store-round-tripped timestamp.
